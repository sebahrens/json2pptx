// Package template provides PPTX template analysis functions.
package template

import (
	"encoding/xml"
	"fmt"
	"log/slog"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// MasterPositionResolver resolves placeholder positions from slide masters.
// This is used when a layout's placeholder has no transform (inherits from master).
type MasterPositionResolver struct {
	reader *Reader
	cache  map[string]map[string]*MasterTransform // masterPath -> (key -> transform)
}

// MasterTransform represents a placeholder transform from a slide master.
// This is the position and size information that layouts inherit when they
// don't specify their own transform.
type MasterTransform struct {
	OffsetX  int64
	OffsetY  int64
	ExtentCX int64
	ExtentCY int64
}

// ToBoundingBox converts a MasterTransform to a BoundingBox.
func (t *MasterTransform) ToBoundingBox() types.BoundingBox {
	return types.BoundingBox{
		X:      t.OffsetX,
		Y:      t.OffsetY,
		Width:  t.ExtentCX,
		Height: t.ExtentCY,
	}
}

// NewMasterPositionResolver creates a resolver for the given template.
func NewMasterPositionResolver(reader *Reader) *MasterPositionResolver {
	return &MasterPositionResolver{
		reader: reader,
		cache:  make(map[string]map[string]*MasterTransform),
	}
}

// GetMasterPositionsForLayout retrieves placeholder positions from the slide master
// associated with the given layout. Results are cached to avoid re-parsing.
func (r *MasterPositionResolver) GetMasterPositionsForLayout(layoutID string) map[string]*MasterTransform {
	// Find the master for this layout by reading the layout's relationships
	layoutRelsPath := fmt.Sprintf("ppt/slideLayouts/_rels/%s.xml.rels", layoutID)
	relsData, err := r.reader.ReadFile(layoutRelsPath)
	if err != nil {
		slog.Debug("master position resolution failed: layout rels file not found",
			slog.String("layout_id", layoutID),
			slog.String("rels_path", layoutRelsPath),
			slog.String("error", err.Error()),
		)
		return nil // No rels file, can't determine master
	}

	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		slog.Debug("master position resolution failed: layout rels XML parse error",
			slog.String("layout_id", layoutID),
			slog.String("rels_path", layoutRelsPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	// Find the slideMaster relationship
	var masterPath string
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			// Target is relative (e.g., "../slideMasters/slideMaster1.xml")
			// Convert to absolute path within the ZIP
			masterPath = ResolveRelativePath("ppt/slideLayouts", rel.Target)
			break
		}
	}

	if masterPath == "" {
		slog.Debug("master position resolution failed: no slideMaster relationship",
			slog.String("layout_id", layoutID),
			slog.String("rels_path", layoutRelsPath),
			slog.Int("relationship_count", len(rels.Relationships)),
		)
		return nil // No master relationship found
	}

	// Check cache
	if positions, ok := r.cache[masterPath]; ok {
		slog.Debug("master positions resolved from cache",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.Int("position_count", len(positions)),
		)
		return positions
	}

	// Load and parse the master
	masterData, err := r.reader.ReadFile(masterPath)
	if err != nil {
		slog.Debug("master position resolution failed: master file not found",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	positions, err := ParseSlideMasterPositions(masterData)
	if err != nil {
		slog.Debug("master position resolution failed: master XML parse error",
			slog.String("layout_id", layoutID),
			slog.String("master_path", masterPath),
			slog.String("error", err.Error()),
		)
		return nil
	}

	// Cache for future layouts using the same master
	r.cache[masterPath] = positions
	slog.Debug("master positions resolved successfully",
		slog.String("layout_id", layoutID),
		slog.String("master_path", masterPath),
		slog.Int("position_count", len(positions)),
	)
	return positions
}

// ResolveRelativePath converts a relative path to an absolute path within the ZIP.
// Exported for use by generator package to avoid code duplication.
func ResolveRelativePath(basePath, relativePath string) string {
	// Handle "../" prefixes
	for len(relativePath) > 3 && relativePath[:3] == "../" {
		relativePath = relativePath[3:]
		// Go up one directory level
		lastSlash := len(basePath) - 1
		for lastSlash > 0 && basePath[lastSlash] != '/' {
			lastSlash--
		}
		if lastSlash > 0 {
			basePath = basePath[:lastSlash]
		}
	}
	return basePath + "/" + relativePath
}

// ParseSlideMasterPositions extracts placeholder positions from a slide master.
// Returns a map of placeholder key -> MasterTransform with multiple keys per placeholder.
// Keys include: "type:<type>", "type:<type>:idx:<idx>", and "idx:<idx>"
func ParseSlideMasterPositions(masterData []byte) (map[string]*MasterTransform, error) {
	var master slideMasterXML
	if err := xml.Unmarshal(masterData, &master); err != nil {
		return nil, fmt.Errorf("failed to parse slide master: %w", err)
	}

	positions := make(map[string]*MasterTransform)
	for i := range master.CommonSlideData.ShapeTree.Shapes {
		shape := &master.CommonSlideData.ShapeTree.Shapes[i]
		if shape.ShapeProperties.Transform == nil {
			continue
		}
		// Register all possible keys for this placeholder
		keys := getMasterPlaceholderKeys(shape)
		transform := &MasterTransform{
			OffsetX:  shape.ShapeProperties.Transform.Offset.X,
			OffsetY:  shape.ShapeProperties.Transform.Offset.Y,
			ExtentCX: shape.ShapeProperties.Transform.Extents.CX,
			ExtentCY: shape.ShapeProperties.Transform.Extents.CY,
		}
		for _, key := range keys {
			positions[key] = transform
		}
	}
	return positions, nil
}

// getMasterPlaceholderKeys generates all possible lookup keys for a shape's placeholder.
// Returns keys in priority order: type, type+idx, idx
// This handles the OOXML inheritance where layouts may have only idx but masters have type+idx.
func getMasterPlaceholderKeys(shape *slideMasterShapeXML) []string {
	ph := shape.NonVisualProperties.Placeholder
	if ph == nil {
		return nil
	}

	var keys []string

	// Type-based key (highest priority)
	if ph.Type != "" {
		keys = append(keys, "type:"+ph.Type)
	}

	// Combined type+index key (for exact matching)
	if ph.Type != "" && ph.Index != nil {
		keys = append(keys, fmt.Sprintf("type:%s:idx:%d", ph.Type, *ph.Index))
	}

	// Index-based key (lowest priority)
	if ph.Index != nil {
		keys = append(keys, fmt.Sprintf("idx:%d", *ph.Index))
	}

	return keys
}

// getLayoutPlaceholderKeys generates lookup keys for a layout placeholder.
// This mirrors the generator's getPlaceholderKeys function.
func getLayoutPlaceholderKeys(ph *placeholderXML) []string {
	if ph == nil {
		return nil
	}

	var keys []string

	// Type-based key (highest priority)
	if ph.Type != "" {
		keys = append(keys, "type:"+ph.Type)
	}

	// Combined type+index key (for exact matching)
	if ph.Type != "" && ph.Index != nil {
		keys = append(keys, fmt.Sprintf("type:%s:idx:%d", ph.Type, *ph.Index))
	}

	// Index-based key (lowest priority)
	if ph.Index != nil {
		keys = append(keys, fmt.Sprintf("idx:%d", *ph.Index))
	}

	return keys
}

// LookupMasterPosition finds a master position for a layout placeholder.
// Tries multiple keys in priority order: type, type+idx, idx.
// Per OOXML spec, content placeholders without a type attribute (idx only)
// inherit from the master's body-type placeholder when no exact match exists.
func LookupMasterPosition(masterPositions map[string]*MasterTransform, ph *placeholderXML) *MasterTransform {
	if masterPositions == nil || ph == nil {
		return nil
	}

	keys := getLayoutPlaceholderKeys(ph)
	for _, key := range keys {
		if transform, ok := masterPositions[key]; ok {
			slog.Debug("resolved placeholder position from master",
				slog.String("key", key),
				slog.Int64("offset_x", transform.OffsetX),
				slog.Int64("offset_y", transform.OffsetY),
				slog.Int64("extent_cx", transform.ExtentCX),
				slog.Int64("extent_cy", transform.ExtentCY),
			)
			return transform
		}
	}

	// OOXML fallback: untypified content placeholders (no type attr, idx only)
	// inherit from the master's body placeholder. This handles templates like
	// templates where layout content placeholders use high idx values (e.g., idx=12)
	// that don't exist in the master, but should inherit the master's body bounds.
	//
	// Guard: if the master's body bounds are too small (height < 2 inches),
	// return nil instead of propagating tiny bounds. This causes the caller
	// to fall back to zero bounds, which downstream guards in layout selection
	// (isSuitableForSlideType) and generation (processDiagramContent) treat
	// as "unknown size" and handle appropriately. Without this guard, a master
	// with a small body placeholder (e.g., designed for subtitle text) would
	// propagate its tiny bounds to ALL untypified content placeholders, causing
	// charts and diagrams to render as thumbnails.
	if ph.Type == "" && ph.Index != nil {
		if transform, ok := masterPositions["type:body"]; ok {
			if transform.ExtentCY < minMasterBodyHeight {
				slog.Debug("master body fallback rejected: height below minimum",
					slog.Int("idx", int(*ph.Index)),
					slog.Int64("extent_cy", transform.ExtentCY),
					slog.Int64("min_height", minMasterBodyHeight),
				)
				return nil
			}
			slog.Debug("resolved untypified placeholder from master body fallback",
				slog.Int("idx", int(*ph.Index)),
				slog.Int64("offset_x", transform.OffsetX),
				slog.Int64("offset_y", transform.OffsetY),
				slog.Int64("extent_cx", transform.ExtentCX),
				slog.Int64("extent_cy", transform.ExtentCY),
			)
			return transform
		}
	}

	return nil
}

// minMasterBodyHeight is the minimum height (in EMUs) for a master body
// placeholder to be used as a fallback for untypified content placeholders.
// 1828800 EMU ≈ 2 inches. Masters with body placeholders shorter than this
// are designed for subtitle/caption text, not full-area content.
const minMasterBodyHeight int64 = 1828800

// XML structure definitions for parsing slide masters

// slideMasterXML represents a slide master file for parsing placeholder positions.
type slideMasterXML struct {
	XMLName         xml.Name               `xml:"sldMaster"`
	CommonSlideData slideMasterCommonSlide `xml:"cSld"`
}

type slideMasterCommonSlide struct {
	Name      string               `xml:"name,attr"`
	ShapeTree slideMasterShapeTree `xml:"spTree"`
}

type slideMasterShapeTree struct {
	Shapes []slideMasterShapeXML `xml:"sp"`
}

type slideMasterShapeXML struct {
	NonVisualProperties slideMasterNvSpPr        `xml:"nvSpPr"`
	ShapeProperties     slideMasterShapePropsXML `xml:"spPr"`
}

type slideMasterNvSpPr struct {
	Placeholder *placeholderXML `xml:"nvPr>ph"`
}

type slideMasterShapePropsXML struct {
	Transform *slideMasterTransformXML `xml:"xfrm"`
}

type slideMasterTransformXML struct {
	Offset  slideMasterOffsetXML  `xml:"off"`
	Extents slideMasterExtentsXML `xml:"ext"`
}

type slideMasterOffsetXML struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

type slideMasterExtentsXML struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

// ResolvePlaceholderBounds attempts to resolve bounds for a placeholder,
// first checking the shape's own transform, then falling back to master positions.
func ResolvePlaceholderBounds(
	shapeTransform *transformXML,
	placeholder *placeholderXML,
	masterPositions map[string]*MasterTransform,
	layoutName string,
	shapeIndex int,
) types.BoundingBox {
	// If the shape has its own transform, use it directly
	if shapeTransform != nil {
		bounds := types.BoundingBox{}
		if shapeTransform.Offset != nil {
			bounds.X = shapeTransform.Offset.X
			bounds.Y = shapeTransform.Offset.Y
		}
		if shapeTransform.Extents != nil {
			bounds.Width = shapeTransform.Extents.CX
			bounds.Height = shapeTransform.Extents.CY
		}
		return bounds
	}

	// Try to resolve from master positions
	if masterTransform := LookupMasterPosition(masterPositions, placeholder); masterTransform != nil {
		return masterTransform.ToBoundingBox()
	}

	// Unable to resolve - log warning and return empty bounds
	var keys []string
	if placeholder != nil {
		keys = getLayoutPlaceholderKeys(placeholder)
	}
	slog.Warn("placeholder missing transform and master resolution failed",
		slog.String("layout", layoutName),
		slog.Int("shape_index", shapeIndex),
		slog.Any("keys_tried", keys),
	)
	return types.BoundingBox{}
}
