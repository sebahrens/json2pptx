package template

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log/slog"
	"sort"

	"github.com/sebahrens/json2pptx/internal/types"
)

// NormalizationResult records all changes made by placeholder normalization.
type NormalizationResult struct {
	Renames    []PlaceholderRename // Shape name renames
	TypeFixes  []TypeInjection     // Placeholders that had type="body" injected
	Warnings   []string            // Structural warnings (e.g., duplicate canonical names)
}

// HasChanges returns true if any renames or type fixes were applied.
func (r NormalizationResult) HasChanges() bool {
	return len(r.Renames) > 0 || len(r.TypeFixes) > 0
}

// PlaceholderRename records a shape name change.
type PlaceholderRename struct {
	OldName string
	NewName string
}

// TypeInjection records a type attribute injection for an implicit body placeholder.
type TypeInjection struct {
	ShapeName string // Shape name (after rename)
	PhIndex   int    // Placeholder index attribute value
}

// shapeRef tracks a shape's index and visual position for sorting.
type shapeRef struct {
	shapeIdx int
	x        int64
	y        int64
}

// NormalizePlaceholderNames computes and applies canonical names to placeholder
// shapes in-place. It also injects type="body" for implicit body placeholders
// (those with no explicit type attribute).
//
// Canonical naming scheme:
//
//	title       - type="title" or type="ctrTitle"
//	subtitle    - type="subTitle"
//	body        - first body/implicit body placeholder (by visual order)
//	body_2      - second body/implicit body placeholder
//	image       - first pic placeholder
//	image_2     - second pic placeholder
//	dt/ftr/sldNum/hdr - utility placeholders (preserved as-is)
//
// Visual order: sort by X offset (left-to-right), then Y offset (top-to-bottom).
func NormalizePlaceholderNames(shapes []shapeXML) NormalizationResult { //nolint:gocognit,gocyclo
	var result NormalizationResult

	// Phase 1: Inject type="body" for implicit body placeholders.
	// In OOXML, a placeholder with no explicit type attribute (but with an idx)
	// is a generic content placeholder that defaults to body behavior.
	for i := range shapes {
		ph := shapes[i].NonVisualProperties.Placeholder
		if ph == nil {
			continue
		}
		if ph.Type == "" {
			ph.Type = "body"
			idx := 0
			if ph.Index != nil {
				idx = *ph.Index
			}
			result.TypeFixes = append(result.TypeFixes, TypeInjection{
				ShapeName: shapes[i].NonVisualProperties.ConnectionNonVisual.Name,
				PhIndex:   idx,
			})
		}
	}

	// Phase 2: Collect placeholders by semantic role.
	var titles []shapeRef
	var subtitles []shapeRef
	var bodies []shapeRef
	var images []shapeRef

	for i := range shapes {
		ph := shapes[i].NonVisualProperties.Placeholder
		if ph == nil {
			continue
		}

		var x, y int64
		if t := shapes[i].ShapeProperties.Transform; t != nil {
			if t.Offset != nil {
				x = t.Offset.X
				y = t.Offset.Y
			}
		}

		ref := shapeRef{shapeIdx: i, x: x, y: y}

		switch ph.Type {
		case "title", "ctrTitle":
			titles = append(titles, ref)
		case "subTitle":
			subtitles = append(subtitles, ref)
		case "body":
			bodies = append(bodies, ref)
		case "pic":
			images = append(images, ref)
		case "dt", "ftr", "sldNum", "hdr":
			// Utility placeholders — preserve names as-is
		}
	}

	// Phase 3: Sort by visual position (X primary, Y tiebreaker).
	sortByPosition := func(refs []shapeRef) {
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].x != refs[j].x {
				return refs[i].x < refs[j].x
			}
			return refs[i].y < refs[j].y
		})
	}

	sortByPosition(bodies)
	sortByPosition(images)

	// Phase 4: Assign canonical names.
	rename := func(ref shapeRef, newName string) {
		old := shapes[ref.shapeIdx].NonVisualProperties.ConnectionNonVisual.Name
		if old != newName {
			shapes[ref.shapeIdx].NonVisualProperties.ConnectionNonVisual.Name = newName
			result.Renames = append(result.Renames, PlaceholderRename{
				OldName: old,
				NewName: newName,
			})
		}
	}

	for _, ref := range titles {
		rename(ref, "title")
	}
	for _, ref := range subtitles {
		rename(ref, "subtitle")
	}
	for i, ref := range bodies {
		if i == 0 {
			rename(ref, "body")
		} else {
			rename(ref, fmt.Sprintf("body_%d", i+1))
		}
	}
	for i, ref := range images {
		if i == 0 {
			rename(ref, "image")
		} else {
			rename(ref, fmt.Sprintf("image_%d", i+1))
		}
	}

	// Phase 5: Validate — check for duplicate canonical names.
	seen := make(map[string]bool)
	for i := range shapes {
		if shapes[i].NonVisualProperties.Placeholder == nil {
			continue
		}
		name := shapes[i].NonVisualProperties.ConnectionNonVisual.Name
		if name == "" {
			continue
		}
		if seen[name] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("duplicate canonical name %q in layout", name))
		}
		seen[name] = true
	}

	return result
}

// ApplyNormalizationToBytes applies name renames and type injections to raw
// layout XML bytes. Returns modified bytes (or the original if no changes).
func ApplyNormalizationToBytes(rawXML []byte, result NormalizationResult) []byte {
	if !result.HasChanges() {
		return rawXML
	}

	modified := rawXML

	// Apply name renames: replace name="OldName" with name="NewName".
	// Shape names are unique within a layout, so this is safe.
	for _, rename := range result.Renames {
		old := []byte(fmt.Sprintf(`name="%s"`, rename.OldName))
		new := []byte(fmt.Sprintf(`name="%s"`, rename.NewName))
		modified = bytes.Replace(modified, old, new, 1)
	}

	// Apply type injections: add type="body" to <p:ph> elements without a type attribute.
	for _, injection := range result.TypeFixes {
		modified = injectPhTypeAttr(modified, injection.PhIndex, "body")
	}

	return modified
}

// injectPhTypeAttr adds a type attribute to a <p:ph> element identified by its idx value.
// Only injects if the element doesn't already have a type attribute.
func injectPhTypeAttr(xml []byte, idx int, phType string) []byte {
	// Find the idx="N" pattern
	idxPattern := []byte(fmt.Sprintf(`idx="%d"`, idx))
	pos := bytes.Index(xml, idxPattern)
	if pos < 0 {
		return xml
	}

	// Find the opening <p:ph before this position
	prefix := xml[:pos]
	phStart := bytes.LastIndex(prefix, []byte("<p:ph"))
	if phStart < 0 {
		return xml
	}

	// Find the end of this element (/>)
	phEnd := bytes.Index(xml[phStart:], []byte("/>"))
	if phEnd < 0 {
		return xml
	}
	phEnd += phStart

	// Extract the full element to check for existing type=
	element := xml[phStart : phEnd+2]
	if bytes.Contains(element, []byte("type=")) {
		return xml
	}

	// Inject type="body" after "<p:ph "
	insertPos := phStart + len("<p:ph ")
	injection := []byte(fmt.Sprintf(`type="%s" `, phType))

	result := make([]byte, 0, len(xml)+len(injection))
	result = append(result, xml[:insertPos]...)
	result = append(result, injection...)
	result = append(result, xml[insertPos:]...)

	return result
}

// NormalizeLayoutBytes normalizes a single layout XML byte slice in one shot.
// It parses the XML, computes canonical names, and applies changes to the raw
// bytes. Returns the (possibly modified) bytes and true if any changes were made.
// This is the preferred entry point for callers that have raw bytes rather than
// a Reader + LayoutMetadata.
func NormalizeLayoutBytes(rawXML []byte) ([]byte, bool) {
	var xmlLayout slideLayoutXML
	if err := xml.Unmarshal(rawXML, &xmlLayout); err != nil {
		return rawXML, false
	}

	normResult := NormalizePlaceholderNames(xmlLayout.CommonSlideData.ShapeTree.Shapes)
	if !normResult.HasChanges() {
		return rawXML, false
	}

	return ApplyNormalizationToBytes(rawXML, normResult), true
}

// NormalizeLayoutFiles reads each layout's XML from the template, normalizes
// placeholder names, updates layout metadata IDs in-place, and returns a map
// of layout path → normalized XML bytes suitable for SyntheticFiles injection.
//
// Layouts that require no normalization (no renames or type injections) are
// skipped — the generator will read them directly from the template ZIP.
func NormalizeLayoutFiles(reader *Reader, layouts []types.LayoutMetadata) (map[string][]byte, error) {
	normalizedFiles := make(map[string][]byte)

	for i := range layouts {
		layout := &layouts[i]
		filename := fmt.Sprintf("ppt/slideLayouts/%s.xml", layout.ID)

		data, err := reader.ReadFile(filename)
		if err != nil {
			// Layout not in template ZIP (e.g., synthetic layout) — skip
			continue
		}

		var xmlLayout slideLayoutXML
		if err := xml.Unmarshal(data, &xmlLayout); err != nil {
			slog.Debug("normalization: failed to parse layout",
				slog.String("file", filename),
				slog.String("error", err.Error()),
			)
			continue
		}

		normResult := NormalizePlaceholderNames(xmlLayout.CommonSlideData.ShapeTree.Shapes)
		if !normResult.HasChanges() {
			continue
		}

		// Update layout metadata placeholder IDs to canonical names
		updateLayoutPlaceholderIDs(layout, normResult)

		// Apply changes to raw bytes for SyntheticFiles
		normalizedData := ApplyNormalizationToBytes(data, normResult)
		normalizedFiles[filename] = normalizedData

		slog.Debug("normalized layout placeholders",
			slog.String("layout", layout.Name),
			slog.String("id", layout.ID),
			slog.Int("renames", len(normResult.Renames)),
			slog.Int("type_fixes", len(normResult.TypeFixes)),
			slog.Any("warnings", normResult.Warnings),
		)
		for _, r := range normResult.Renames {
			slog.Debug("  rename",
				slog.String("old", r.OldName),
				slog.String("new", r.NewName),
			)
		}
		for _, t := range normResult.TypeFixes {
			slog.Debug("  type_inject",
				slog.String("shape", t.ShapeName),
				slog.Int("idx", t.PhIndex),
			)
		}
	}

	return normalizedFiles, nil
}

// updateLayoutPlaceholderIDs updates PlaceholderInfo.ID fields to reflect
// canonical names from the normalization result.
func updateLayoutPlaceholderIDs(layout *types.LayoutMetadata, result NormalizationResult) {
	// Apply renames sequentially: for each rename, find the first placeholder
	// that still has the old name and rename it. This handles duplicate names
	// correctly (e.g., three "Content Placeholder 2" → "body", "body_2", "body_3")
	// because each rename consumes exactly one matching placeholder.
	for _, r := range result.Renames {
		for i := range layout.Placeholders {
			if layout.Placeholders[i].ID == r.OldName {
				layout.Placeholders[i].ID = r.NewName
				break
			}
		}
	}

	// Also update placeholder type for type injections:
	// Placeholders that had type="" (mapped to PlaceholderContent) should
	// now be PlaceholderBody since we injected type="body".
	for i := range layout.Placeholders {
		if layout.Placeholders[i].Type == types.PlaceholderContent {
			layout.Placeholders[i].Type = types.PlaceholderBody
		}
	}
}
