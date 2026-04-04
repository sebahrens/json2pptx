// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/sebahrens/json2pptx/internal/types"
)

// FooterConfig controls slide footer injection.
// When Enabled, three footer zones are injected into every generated slide:
//   - Left: configurable text (company name, date, etc.)
//   - Center: auto-populated with each slide's title text
//   - Right: auto-incremented slide number
type FooterConfig struct {
	Enabled  bool   // Master switch — when false, no footers are injected
	LeftText string // Left footer text (e.g., "Acme Corp | Confidential")
}

// GenerationRequest specifies what to generate.
type GenerationRequest struct {
	TemplatePath          string              // Path to template PPTX file
	OutputPath            string              // Where to write generated PPTX
	Slides                []SlideSpec         // Slide specifications
	AllowedImagePaths     []string            // Allowed base paths for image loading (security)
	SVGStrategy           string              // SVG conversion strategy: "png" (default), "emf", or "native"
	SVGScale              float64             // Scale factor for SVG to PNG conversion (default: 2.0)
	SVGNativeCompat       string              // Native SVG compatibility mode: "warn" (default), "fallback", "strict", "ignore"
	MaxPNGWidth           int                 // Maximum pixel width for PNG fallback images (0 = no cap, default: 2500)
	ExcludeTemplateSlides bool                // When true, exclude template's example slides from output (should always be true)
	ThemeOverride         *types.ThemeOverride // Per-deck theme color/font overrides from frontmatter (nil = no override)
	SyntheticFiles        map[string][]byte   // Synthetic layout files from SynthesisManifest (nil = no synthetic layouts)
	Footer                *FooterConfig       // Footer configuration (nil = disabled)
}

// BackgroundImage specifies a slide background image.
type BackgroundImage struct {
	Path string // File path to background image
	Fit  string // "cover" (default), "stretch", "tile"
}

// SlideSpec defines a single slide to create.
type SlideSpec struct {
	LayoutID        string           // Layout to use (e.g., "slideLayout1")
	Content         []ContentItem    // Content items to populate
	Background      *BackgroundImage // Slide background image (nil = no background image)
	SpeakerNotes    string           // Speaker notes text (written to notesSlide XML)
	SourceNote      string           // Source attribution text (rendered as small text at slide bottom)
	Transition      string           // Slide transition type: "fade", "push", "wipe", "cover", "uncover", "cut", "dissolve"
	TransitionSpeed string           // Transition speed: "slow", "med", "fast" (default: "med")
	Build           string           // Build animation: "bullets" for one-by-one bullet reveal
	RawShapeXML     [][]byte         // Pre-generated <p:sp> XML fragments to inject into spTree
	IconInserts     []IconInsert     // SVG icon images from shape_grid (require media registration)
	ImageInserts    []ImageInsert    // Image files from shape_grid (require media registration)
}

// IconInsert describes an SVG icon to embed in a slide as a p:pic element.
// The generator registers the SVG + PNG fallback as media files and creates
// the necessary relationships.
type IconInsert struct {
	SVGData  []byte // Raw SVG content
	OffsetX  int64  // X position in EMU
	OffsetY  int64  // Y position in EMU
	ExtentCX int64  // Width in EMU
	ExtentCY int64  // Height in EMU
}

// ImageInsert describes an image file to embed in a slide as a p:pic element.
// Used by shape_grid cells with CellKindImage.
type ImageInsert struct {
	Path     string // File path to the image
	Alt      string // Alt text for accessibility
	OffsetX  int64  // X position in EMU
	OffsetY  int64  // Y position in EMU
	ExtentCX int64  // Width in EMU
	ExtentCY int64  // Height in EMU
}

// ContentItem represents content to place in a placeholder.
type ContentItem struct {
	PlaceholderID string      // Target placeholder ID
	Type          ContentType // Type of content
	Value         any         // Type-specific value
	FontSize      int         // Font size override in hundredths of a point (e.g., 7200 = 72pt). 0 means no override.
}

// ContentType indicates the kind of content.
type ContentType string

const (
	ContentText            ContentType = "text"
	ContentSectionTitle    ContentType = "section_title"       // Section divider title (large, prominent text)
	ContentTitleSlideTitle ContentType = "title_slide_title"   // Title slide ctrTitle (preserves template font/alignment)
	ContentBullets         ContentType = "bullets"
	ContentBodyAndBullets  ContentType = "body_and_bullets"    // Body text followed by bullets
	ContentBulletGroups    ContentType = "bullet_groups"       // Grouped bullets with section headers
	ContentImage           ContentType = "image"
	ContentDiagram         ContentType = "diagram"             // Unified diagram type (charts, infographics)
)

// BodyAndBulletsContent represents body text followed by bullet points,
// with optional trailing body text after the bullets.
type BodyAndBulletsContent struct {
	Body         string   // Body text paragraph before bullets (no bullet marker)
	Bullets      []string // Bullet points (with bullet markers)
	TrailingBody string   // Body text paragraph after bullets (no bullet marker)
}

// BulletGroupsContent represents grouped bullets with section headers.
// Each group has an optional header and a list of bullets beneath it.
// Body is always rendered before all groups (it is the intro text preceding
// the first section header/bullets in the markdown source).
// If TrailingBody is non-empty, it is rendered as a paragraph after the last group.
type BulletGroupsContent struct {
	Body         string        // Optional intro paragraph before the first group
	Groups       []BulletGroup // Ordered list of bullet groups
	TrailingBody string        // Optional concluding paragraph after the last group
}

// BulletGroup is an alias for types.BulletGroup.
// It represents a section with an optional header and bullet items.
type BulletGroup = types.BulletGroup

// GenerationResult contains the result of generation.
type GenerationResult struct {
	OutputPath    string         // Path to generated file
	FileSize      int64          // Size of generated file in bytes
	SlideCount    int            // Number of slides created
	Warnings      []string       // Non-fatal warnings
	Duration      time.Duration  // Time taken to generate
	MediaFailures []MediaFailure // Structured per-slide media errors (diagrams, images, tables)
}

// MediaFailure describes a media content item that failed to render on a slide.
// This enables callers to distinguish between informational warnings and actual
// content failures, allowing decisions about retry, fallback, or reporting.
type MediaFailure struct {
	SlideNum      int    // 1-based slide number
	PlaceholderID string // Target placeholder that failed
	ContentType   string // "diagram", "image", "table"
	DiagramType   string // e.g., "pie_chart", "timeline" (only for diagrams)
	Reason        string // Human-readable failure reason
	Fallback      string // What was done instead: "placeholder_image", "skipped"
}

// ImageContent represents image content to embed.
type ImageContent struct {
	Path   string             // File path to image
	Alt    string             // Alt text for accessibility
	Bounds *types.BoundingBox // Optional bounds override
}

// Generator defines the interface for PPTX generation.
// This interface enables dependency injection and mocking in tests.
type Generator interface {
	// Generate creates a PPTX file from the request.
	Generate(ctx context.Context, req GenerationRequest) (*GenerationResult, error)
}

// DefaultGenerator is the production implementation of Generator.
// It uses the package-level Generate function internally.
type DefaultGenerator struct{}

// NewGenerator creates a new DefaultGenerator instance.
func NewGenerator() *DefaultGenerator {
	return &DefaultGenerator{}
}

// Generate implements the Generator interface using the package-level Generate function.
func (g *DefaultGenerator) Generate(ctx context.Context, req GenerationRequest) (*GenerationResult, error) {
	return Generate(ctx, req)
}

// Generate creates a PPTX file from the request.
// AC1: Valid PPTX output
// AC2: Correct slide count
// AC9: Layout accuracy
//
// Performance optimization (C1 fix):
// This function uses single-pass ZIP generation, performing only ONE ZIP read
// and ONE ZIP write operation instead of the previous 3x read/write cycles.
func Generate(ctx context.Context, req GenerationRequest) (*GenerationResult, error) {
	startTime := time.Now()

	// Validate request
	if err := validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Use single-pass generation for optimal performance
	result, warnings, err := generateSinglePass(ctx, req)
	if err != nil {
		// Clean up partial output if it exists
		_ = os.Remove(req.OutputPath)
		return nil, err
	}

	// Set duration (single-pass returns without duration set)
	result.Duration = time.Since(startTime)
	result.Warnings = warnings

	return result, nil
}

// validateRequest checks that the request is valid.
func validateRequest(req GenerationRequest) error {
	if req.TemplatePath == "" {
		return fmt.Errorf("template path is required")
	}
	if req.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}
	if len(req.Slides) == 0 {
		return fmt.Errorf("at least one slide is required")
	}

	// Check template exists
	if _, err := os.Stat(req.TemplatePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template file not found: %s", req.TemplatePath)
		}
		return fmt.Errorf("cannot access template: %w", err)
	}

	// Check output directory exists
	outputDir := filepath.Dir(req.OutputPath)
	if _, err := os.Stat(outputDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory does not exist: %s", outputDir)
		}
		return fmt.Errorf("cannot access output directory: %w", err)
	}

	return nil
}

// createSlideFromLayout creates a slide XML struct from a layout XML template.
// masterPositions maps placeholder type/index to transforms from the slide master.
func createSlideFromLayout(layoutData []byte, slideNum int, masterPositions map[string]*transformXML) (*slideXML, error) {
	// Parse layout XML
	var layout slideLayoutXML
	if err := xml.Unmarshal(layoutData, &layout); err != nil {
		return nil, fmt.Errorf("failed to parse layout: %w", err)
	}

	// Resolve empty spPr transforms from slide master
	if masterPositions != nil {
		resolveEmptyTransforms(&layout.CommonSlideData.ShapeTree, masterPositions)
	}

	// Create slide XML structure from layout
	slide := &slideXML{
		XMLName: xml.Name{Space: "http://schemas.openxmlformats.org/presentationml/2006/main", Local: "sld"},
		CommonSlideData: commonSlideDataXML{
			Name:      fmt.Sprintf("Slide %d", slideNum),
			ShapeTree: layout.CommonSlideData.ShapeTree,
		},
		ColorMapOverride: colorMapOverrideXML{
			MasterColorMapping: masterColorMappingXML{},
		},
	}

	return slide, nil
}

// resolveEmptyTransforms fills in missing transforms from the slide master.
// In OOXML, when a layout's <p:spPr/> is empty, it inherits position from the master.
// We must explicitly copy the transform since we're creating a standalone slide.
func resolveEmptyTransforms(tree *shapeTreeXML, masterPositions map[string]*transformXML) {
	var emptyCount, resolvedCount, unresolvedCount int
	for i := range tree.Shapes {
		shape := &tree.Shapes[i]
		// Check if shape has empty spPr (no transform)
		if shape.ShapeProperties.Transform == nil {
			emptyCount++
			// Try multiple keys to find a match in the master
			keys := getPlaceholderKeys(shape)
			resolved := false
			for _, key := range keys {
				if masterXfrm, ok := masterPositions[key]; ok {
					// Copy the transform from the master
					shape.ShapeProperties.Transform = &transformXML{
						Offset: offsetXML{X: masterXfrm.Offset.X, Y: masterXfrm.Offset.Y},
						Extent: extentXML{CX: masterXfrm.Extent.CX, CY: masterXfrm.Extent.CY},
					}
					resolved = true
					resolvedCount++
					slog.Debug("resolved empty transform from master",
						slog.String("key_used", key),
						slog.Int64("offset_x", masterXfrm.Offset.X),
						slog.Int64("offset_y", masterXfrm.Offset.Y),
						slog.Int64("extent_cx", masterXfrm.Extent.CX),
						slog.Int64("extent_cy", masterXfrm.Extent.CY),
					)
					break
				}
			}
			if !resolved {
				unresolvedCount++
				slog.Debug("failed to resolve empty transform, shape will have zero bounds",
					slog.Any("keys_tried", keys),
					slog.Int("available_master_positions", len(masterPositions)),
				)
			}
		}
	}
	if emptyCount > 0 {
		slog.Debug("empty transform resolution summary",
			slog.Int("total_shapes", len(tree.Shapes)),
			slog.Int("empty_transforms", emptyCount),
			slog.Int("resolved", resolvedCount),
			slog.Int("unresolved", unresolvedCount),
		)
	}
}

// getPlaceholderKeys generates all possible lookup keys for a shape's placeholder.
// Returns keys in priority order: type, type+idx, idx
// This handles the OOXML inheritance where layouts may have only idx but masters have type+idx.
//
// Per OOXML spec ECMA-376 §19.3.1.36, a placeholder with no explicit type attribute
// defaults to type="body". We use this effective type for key generation so that
// implicit body placeholders (e.g., <p:ph idx="12"/>) match master entries keyed
// by "type:body".
func getPlaceholderKeys(shape *shapeXML) []string {
	ph := shape.NonVisualProperties.NvPr.Placeholder
	if ph == nil {
		return nil
	}

	var keys []string

	// Determine effective type: empty means implicit body per OOXML spec
	effectiveType := ph.Type
	if effectiveType == "" {
		effectiveType = "body"
	}

	// Type-based key (highest priority)
	keys = append(keys, "type:"+effectiveType)

	// Combined type+index key (for exact matching)
	if ph.Index != nil {
		keys = append(keys, fmt.Sprintf("type:%s:idx:%d", effectiveType, *ph.Index))
	}

	// Index-based key (lowest priority)
	if ph.Index != nil {
		keys = append(keys, fmt.Sprintf("idx:%d", *ph.Index))
	}

	return keys
}

// XML structures for parsing and generating PPTX content
// Presentation and slide ID types are from internal/pptx package.

type slideXML struct {
	XMLName          xml.Name            `xml:"sld"`
	CommonSlideData  commonSlideDataXML  `xml:"cSld"`
	ColorMapOverride colorMapOverrideXML `xml:"clrMapOvr"`
}

type slideLayoutXML struct {
	XMLName         xml.Name           `xml:"sldLayout"`
	CommonSlideData commonSlideDataXML `xml:"cSld"`
}


type commonSlideDataXML struct {
	Name      string       `xml:"name,attr"`
	ShapeTree shapeTreeXML `xml:"spTree"`
}

type shapeTreeXML struct {
	NvGrpSpPr nvGrpSpPrXML `xml:"nvGrpSpPr"`
	GrpSpPr   grpSpPrXML   `xml:"grpSpPr"`
	Shapes    []shapeXML   `xml:"sp"`
}

// nvGrpSpPrXML is the non-visual group shape properties (required by OOXML).
type nvGrpSpPrXML struct {
	CNvPr      grpCNvPrXML      `xml:"cNvPr"`
	CNvGrpSpPr grpCNvGrpSpPrXML `xml:"cNvGrpSpPr"`
	NvPr       grpNvPrXML       `xml:"nvPr"`
}

type grpCNvPrXML struct {
	ID   uint32 `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

type grpCNvGrpSpPrXML struct {
	// Empty - just needs to exist
}

type grpNvPrXML struct {
	// Empty - just needs to exist
}

// grpSpPrXML is the group shape properties with transform (required by OOXML).
// Note: XML tags use unprefixed names for parsing; namespace prefixes are added
// by fixOOXMLNamespaces during output.
type grpSpPrXML struct {
	Xfrm *grpXfrmXML `xml:"xfrm,omitempty"`
}

type grpXfrmXML struct {
	Off   grpOffXML `xml:"off"`
	Ext   grpExtXML `xml:"ext"`
	ChOff grpOffXML `xml:"chOff"`
	ChExt grpExtXML `xml:"chExt"`
}

// grpOffXML is for offset elements (off, chOff) which use x and y attributes.
type grpOffXML struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// grpExtXML is for extent elements (ext, chExt) which use cx and cy attributes.
type grpExtXML struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

type shapeXML struct {
	NonVisualProperties nonVisualPropertiesXML `xml:"nvSpPr"`
	ShapeProperties     shapePropertiesXML     `xml:"spPr"`
	TextBody            *textBodyXML           `xml:"txBody,omitempty"`
}

type nonVisualPropertiesXML struct {
	ConnectionNonVisual connectionNonVisualXML `xml:"cNvPr"`
	NonVisualShape      nonVisualShapeXML      `xml:"cNvSpPr"`
	NvPr                nvPrXML                `xml:"nvPr"`
}

type nvPrXML struct {
	Placeholder *placeholderXML `xml:"http://schemas.openxmlformats.org/presentationml/2006/main ph,omitempty"`
}

type connectionNonVisualXML struct {
	ID   uint32 `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

type nonVisualShapeXML struct {
	TxBox bool `xml:"txBox,attr,omitempty"`
}

type placeholderXML struct {
	Type            string `xml:"type,attr,omitempty"`
	Index           *int   `xml:"idx,attr,omitempty"`
	HasCustomPrompt string `xml:"hasCustomPrompt,attr,omitempty"`
}

// shapePropertiesXML represents shape properties including transform.
// Note: XML tags use unprefixed names for parsing; namespace prefixes are added
// by fixOOXMLNamespaces during output.
type shapePropertiesXML struct {
	Transform *transformXML `xml:"xfrm,omitempty"`
}

type transformXML struct {
	Offset offsetXML `xml:"off"`
	Extent extentXML `xml:"ext"`
}

// offsetXML represents position offset using x and y attributes.
type offsetXML struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// extentXML represents extent/size using cx and cy attributes.
type extentXML struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

type textBodyXML struct {
	BodyProperties *bodyPropertiesXML `xml:"bodyPr,omitempty"`
	ListStyle      *listStyleXML      `xml:"lstStyle,omitempty"`
	Paragraphs     []paragraphXML     `xml:"p"`
}

// bodyPropertiesXML preserves the raw XML content of body properties.
// Attributes are explicitly declared so they survive XML round-tripping;
// innerxml captures child elements (normAutofit, noAutofit, etc.).
type bodyPropertiesXML struct {
	Wrap    string `xml:"wrap,attr,omitempty"`    // Text wrapping: "square" (word wrap) or "none"
	LIns    *int64 `xml:"lIns,attr,omitempty"`    // Left inset in EMU
	RIns    *int64 `xml:"rIns,attr,omitempty"`    // Right inset in EMU
	TIns    *int64 `xml:"tIns,attr,omitempty"`    // Top inset in EMU
	BIns    *int64 `xml:"bIns,attr,omitempty"`    // Bottom inset in EMU
	Anchor  string `xml:"anchor,attr,omitempty"`  // Vertical anchor: "t", "ctr", "b"
	AnchorCtr string `xml:"anchorCtr,attr,omitempty"` // Center anchor: "0" or "1"
	RtlCol  string `xml:"rtlCol,attr,omitempty"`  // RTL columns: "0" or "1"
	Vert    string `xml:"vert,attr,omitempty"`     // Text direction: "horz", "vert", etc.
	Inner   string `xml:",innerxml"`
}

// listStyleXML preserves the raw XML content of list styles.
// We use innerxml to preserve styling from the layout without parsing every detail.
type listStyleXML struct {
	Inner string `xml:",innerxml"`
}

type paragraphXML struct {
	Properties *paragraphPropertiesXML `xml:"a:pPr,omitempty"`
	Runs       []runXML                `xml:"a:r"`
	EndParaRPr *endParaRPrXML          `xml:"a:endParaRPr,omitempty"`
}

// paragraphPropertiesXML preserves paragraph properties including the level attribute.
// The Level attribute (lvl) is critical for bullet inheritance from slide master styles.
// Inner captures any child elements (buChar, buFont, etc.) using innerxml.
type paragraphPropertiesXML struct {
	Level  *int   `xml:"lvl,attr,omitempty"`    // Bullet level (0-8), nil if not set
	MarL   *int   `xml:"marL,attr,omitempty"`   // Left margin in EMU, nil inherits from bodyStyle
	Indent *int   `xml:"indent,attr,omitempty"` // First-line indent in EMU, nil inherits from bodyStyle
	Algn   string `xml:"algn,attr,omitempty"`   // Paragraph alignment: "l", "ctr", "r", "just"
	Inner  string `xml:",innerxml"`             // Child elements (preserved verbatim)
}

type runXML struct {
	RunProperties *runPropertiesXML `xml:"rPr,omitempty"`
	Text          string            `xml:"t"`
}

// runPropertiesXML preserves run properties including the language attribute.
// The Lang attribute is needed for proper text rendering. Inner captures any
// child elements (solidFill, latin font, etc.) using innerxml.
// Bold and Italic are used for inline <b>bold</b> and <i>italic</i> formatting.
type runPropertiesXML struct {
	Lang     string `xml:"lang,attr,omitempty"` // Language code (e.g., "en-US")
	FontSize string `xml:"sz,attr,omitempty"`   // Font size in hundredths of a point (e.g., "7200" = 72pt)
	Bold     string `xml:"b,attr,omitempty"`    // Bold flag: "1" for bold, omit otherwise
	Italic   string `xml:"i,attr,omitempty"`    // Italic flag: "1" for italic, omit otherwise
	Inner    string `xml:",innerxml"`           // Child elements (preserved verbatim)
}

// endParaRPrXML preserves end paragraph run properties (used by OOXML for cursor styling).
// The Lang attribute is required by ECMA-376 §19.3.1.33 for every <a:p> element.
type endParaRPrXML struct {
	Lang  string `xml:"lang,attr,omitempty"` // Language code (e.g., "en-US")
	Inner string `xml:",innerxml"`
}

// emptyParagraph returns a spec-compliant empty paragraph with the required
// <a:endParaRPr lang="en-US"/> element (ECMA-376 §19.3.1.33).
// Use this instead of bare paragraphXML{} to avoid triggering PowerPoint repair.
func emptyParagraph() paragraphXML {
	return paragraphXML{
		EndParaRPr: &endParaRPrXML{Lang: "en-US"},
	}
}

type colorMapOverrideXML struct {
	MasterColorMapping masterColorMappingXML `xml:"masterClrMapping"`
}

type masterColorMappingXML struct {
}

// Picture XML types for image embedding are defined in internal/pptx package.
