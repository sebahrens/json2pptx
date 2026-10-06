package main

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// Hierarchical unknown-key check for the full PresentationInput tree.
//
// Walks raw JSON at every object level and reports unknown fields as
// ValidationError items. Callers choose the severity: MCP surfaces these as
// warnings by default (advisory, generation proceeds) and promotes to errors
// only when strict_unknown_keys=true. CLI treats them as warnings.
// ---------------------------------------------------------------------------

// checkInputUnknownKeys runs unknown-key detection on the full
// PresentationInput JSON tree. Returns a ValidationError for every unknown
// field found. Callers decide the severity (warning vs error).
//
// The paths are pointers into the JSON as written (a split_slide's base is
// /slides/1/base), so each finding is marked Authored.
func checkInputUnknownKeys(raw json.RawMessage) []*patterns.ValidationError {
	out := inputUnknownKeys(raw)
	for _, ve := range out {
		ve.Authored = true
	}
	return out
}

func inputUnknownKeys(raw json.RawMessage) []*patterns.ValidationError {
	// A patch envelope ({"base": {...}, "operations": [...]}, docs/INPUT_FORMAT_ADVANCED.md)
	// is not a PresentationInput itself: scan its base deck, or "base" and
	// "operations" are reported as unknown keys and --strict-unknown-keys
	// rejects the documented form (go-slide-creator-csclk.67).
	var envelope struct {
		Base       json.RawMessage   `json:"base"`
		Operations []json.RawMessage `json:"operations"`
	}
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Operations) > 0 && len(envelope.Base) > 0 {
		return inputUnknownKeys(envelope.Base)
	}

	var warnings []*patterns.ValidationError

	// Top level: PresentationInput.
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(PresentationInput{}), "")...)

	// Parse the top-level object to walk children.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return warnings
	}

	// footer
	if v, ok := top["footer"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(JSONFooter{}), "/footer")...)
	}

	// chrome
	if v, ok := top["chrome"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(ChromeInput{}), "/chrome")...)
		// Check nested page_numbers
		var chromeObj map[string]json.RawMessage
		if json.Unmarshal(v, &chromeObj) == nil {
			if pn, pnOK := chromeObj["page_numbers"]; pnOK {
				warnings = append(warnings, checkUnknownKeysForType(pn, reflect.TypeOf(PageNumbersInput{}), "/chrome/page_numbers")...)
			}
		}
	}

	// theme_override
	if v, ok := top["theme_override"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(ThemeInput{}), "/theme_override")...)
	}

	// defaults
	if v, ok := top["defaults"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(DefaultsInput{}), "/defaults")...)
		var defaultsObj map[string]json.RawMessage
		if json.Unmarshal(v, &defaultsObj) == nil {
			if ts, tsOK := defaultsObj["table_style"]; tsOK {
				warnings = append(warnings, checkUnknownKeysForType(ts, reflect.TypeOf(jsonschema.TableStyleInput{}), "/defaults/table_style")...)
			}
			if cs, csOK := defaultsObj["cell_style"]; csOK {
				warnings = append(warnings, checkShapeUnknownKeys(cs, "/defaults/cell_style")...)
			}
		}
	}

	// grid
	if v, ok := top["grid"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(GridConfig{}), "/grid")...)
	}

	// slides[]
	if slidesRaw, ok := top["slides"]; ok {
		var slides []json.RawMessage
		if json.Unmarshal(slidesRaw, &slides) == nil {
			for i, slideRaw := range slides {
				prefix := slidepath.Slide(i)
				warnings = append(warnings, checkSlideUnknownKeys(slideRaw, prefix)...)
			}
		}
	}

	// structure: the block itself, its cover and closing, each section and
	// each section's slides (go-slide-creator-3znh9).
	if v, ok := top["structure"]; ok {
		warnings = append(warnings, checkStructureUnknownKeys(v, "/structure")...)
	}

	return warnings
}

// checkStructureUnknownKeys checks a structure block. Its slides are plain
// slides: a split_slide envelope is not read there.
func checkStructureUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	warnings := checkUnknownKeysForType(raw, reflect.TypeOf(StructureInput{}), path)
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	for _, key := range []string{"cover", "closing"} {
		if v, ok := obj[key]; ok && string(v) != "null" {
			warnings = append(warnings, checkPlainSlideUnknownKeys(v, path+"/"+key)...)
		}
	}
	var sections []json.RawMessage
	if json.Unmarshal(obj["sections"], &sections) != nil {
		return warnings
	}
	for i, sectionRaw := range sections {
		sectionPath := fmt.Sprintf("%s/sections/%d", path, i)
		warnings = append(warnings, checkUnknownKeysForType(sectionRaw, reflect.TypeOf(SectionInput{}), sectionPath)...)
		var section struct {
			Slides []json.RawMessage `json:"slides"`
		}
		if json.Unmarshal(sectionRaw, &section) != nil {
			continue
		}
		for j, slideRaw := range section.Slides {
			warnings = append(warnings, checkPlainSlideUnknownKeys(slideRaw, fmt.Sprintf("%s/slides/%d", sectionPath, j))...)
		}
	}
	return warnings
}

// checkSlideUnknownKeys checks a single slide (or split_slide) for unknown keys.
func checkSlideUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	// Detect split_slide vs regular slide.
	var probe struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &probe) == nil && probe.Type == "split_slide" {
		return checkSplitSlideUnknownKeys(raw, path)
	}
	return checkPlainSlideUnknownKeys(raw, path)
}

// checkPlainSlideUnknownKeys checks a slide object.
func checkPlainSlideUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(SlideInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}

	// background
	if v, ok := obj["background"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(BackgroundInput{}), path+"/background")...)
		var bgObj map[string]json.RawMessage
		if json.Unmarshal(v, &bgObj) == nil {
			if ov, ovOK := bgObj["overlay"]; ovOK {
				warnings = append(warnings, checkUnknownKeysForType(ov, reflect.TypeOf(BackgroundOverlayInput{}), path+"/background/overlay")...)
			}
		}
	}

	// pattern
	if v, ok := obj["pattern"]; ok {
		warnings = append(warnings, checkPatternBlockUnknownKeys(v, path+"/pattern")...)
	}

	// compose
	if v, ok := obj["compose"]; ok {
		warnings = append(warnings, checkComposeUnknownKeys(v, path+"/compose")...)
	}

	// shape_grid
	if v, ok := obj["shape_grid"]; ok {
		warnings = append(warnings, checkShapeGridUnknownKeys(v, path+"/shape_grid")...)
	}

	// overlays[]
	if v, ok := obj["overlays"]; ok {
		var overlays []json.RawMessage
		if json.Unmarshal(v, &overlays) == nil {
			for i, overlayRaw := range overlays {
				warnings = append(warnings, checkOverlayUnknownKeys(overlayRaw, fmt.Sprintf("%s/overlays/%d", path, i))...)
			}
		}
	}

	// source_link
	if v, ok := obj["source_link"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.LinkInput{}), path+"/source_link")...)
	}

	// content[]
	if contentRaw, ok := obj["content"]; ok {
		var items []json.RawMessage
		if json.Unmarshal(contentRaw, &items) == nil {
			for j, itemRaw := range items {
				p := fmt.Sprintf("%s/content/%d", path, j)
				warnings = append(warnings, checkContentUnknownKeys(itemRaw, p)...)
			}
		}
	}

	return warnings
}

// checkComposeUnknownKeys checks a compose envelope and its segments.
func checkComposeUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(ComposeInput{}), path)...)

	var composeObj map[string]json.RawMessage
	if json.Unmarshal(raw, &composeObj) != nil {
		return warnings
	}
	if v, ok := composeObj["banner"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(patterns.BannerSpec{}), path+"/banner")...)
	}
	if v, ok := composeObj["callout"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(patterns.PatternCallout{}), path+"/callout")...)
	}
	segRaw, ok := composeObj["segments"]
	if !ok {
		return warnings
	}
	var segs []json.RawMessage
	if json.Unmarshal(segRaw, &segs) != nil {
		return warnings
	}
	for j, segBytes := range segs {
		p := fmt.Sprintf("%s/segments/%d", path, j)
		warnings = append(warnings, checkUnknownKeysForType(segBytes, reflect.TypeOf(SegmentInput{}), p)...)
		var segObj map[string]json.RawMessage
		if json.Unmarshal(segBytes, &segObj) == nil {
			if patRaw, ok := segObj["pattern"]; ok {
				warnings = append(warnings, checkPatternBlockUnknownKeys(patRaw, p+"/pattern")...)
			}
			if v, ok := segObj["diagram"]; ok {
				warnings = append(warnings, checkDiagramUnknownKeys(v, p+"/diagram")...)
			}
			// Recurse into a nested compose envelope so unknown keys deep in
			// the tree still surface with the correct JSON path.
			if subRaw, ok := segObj["compose"]; ok {
				warnings = append(warnings, checkComposeUnknownKeys(subRaw, p+"/compose")...)
			}
		}
	}
	return warnings
}

// checkPatternBlockUnknownKeys checks a pattern block wherever one is written:
// on a slide, in a compose segment or in a shape_grid cell. The keys inside
// values, overrides and cell_overrides belong to the named pattern, whose own
// decoder reports them when the block is expanded.
func checkPatternBlockUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	warnings := checkUnknownKeysForType(raw, reflect.TypeOf(PatternInput{}), path)
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["callout"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(patterns.PatternCallout{}), path+"/callout")...)
	}
	if v, ok := obj["bounds"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.GridBoundsInput{}), path+"/bounds")...)
	}
	return warnings
}

// checkDiagramUnknownKeys checks a diagram spec and its style blocks: a cell's
// diagram, a composite's sub_diagram or a compose segment's diagram.
func checkDiagramUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	warnings := checkUnknownKeysForType(raw, reflect.TypeOf(types.DiagramSpec{}), path)
	return append(warnings, checkChartStyleUnknownKeys(raw, path)...)
}

// checkSplitSlideUnknownKeys checks a split_slide entry.
func checkSplitSlideUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(SplitSlideInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["split"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(SplitConfig{}), path+"/split")...)
	}
	if v, ok := obj["base"]; ok {
		warnings = append(warnings, checkSlideUnknownKeys(v, path+"/base")...)
	}
	return warnings
}

// checkContentUnknownKeys checks a content item for unknown keys.
func checkContentUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(ContentInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}

	// Check for redundant typed + legacy value fields.
	warnings = append(warnings, checkRedundantValue(obj, path)...)

	if v, ok := obj["body_and_bullets_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(BodyAndBulletsInput{}), path+"/body_and_bullets_value")...)
	}
	if v, ok := obj["body_and_lead_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(BodyAndLeadInput{}), path+"/body_and_lead_value")...)
	}
	if v, ok := obj["bullet_groups_value"]; ok {
		warnings = append(warnings, checkBulletGroupsUnknownKeys(v, path+"/bullet_groups_value")...)
	}
	if v, ok := obj["table_value"]; ok {
		warnings = append(warnings, checkTableUnknownKeys(v, path+"/table_value")...)
	}
	if v, ok := obj["image_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(ImageInput{}), path+"/image_value")...)
	}
	if v, ok := obj["chart_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(types.ChartSpec{}), path+"/chart_value")...) //nolint:staticcheck // ChartSpec is deprecated but still used for backward compat
		warnings = append(warnings, checkChartStyleUnknownKeys(v, path+"/chart_value")...)
	}
	if v, ok := obj["diagram_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(types.DiagramSpec{}), path+"/diagram_value")...)
		warnings = append(warnings, checkChartStyleUnknownKeys(v, path+"/diagram_value")...)
	}
	return warnings
}

// checkBulletGroupsUnknownKeys checks bullet_groups_value and its nested groups.
func checkBulletGroupsUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(BulletGroupsInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if groupsRaw, ok := obj["groups"]; ok {
		var groups []json.RawMessage
		if json.Unmarshal(groupsRaw, &groups) == nil {
			for i, g := range groups {
				p := fmt.Sprintf("%s/groups/%d", path, i)
				warnings = append(warnings, checkUnknownKeysForType(g, reflect.TypeOf(BulletGroupInput{}), p)...)
			}
		}
	}
	return warnings
}

// checkTableUnknownKeys checks table_value and its nested cells/style.
func checkTableUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.TableInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["style"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.TableStyleInput{}), path+"/style")...)
	}
	if rowsRaw, ok := obj["rows"]; ok {
		var rows []json.RawMessage
		if json.Unmarshal(rowsRaw, &rows) == nil {
			for i, rowRaw := range rows {
				var cells []json.RawMessage
				if json.Unmarshal(rowRaw, &cells) == nil {
					for j, cellRaw := range cells {
						p := fmt.Sprintf("%s/rows/%d/%d", path, i, j)
						warnings = append(warnings, checkUnknownKeysForType(cellRaw, reflect.TypeOf(jsonschema.TableCellInput{}), p)...)
						var cell map[string]json.RawMessage
						if json.Unmarshal(cellRaw, &cell) == nil {
							if v, ok := cell["conditional"]; ok {
								warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.ConditionalFormatInput{}), p+"/conditional")...)
							}
						}
					}
				}
			}
		}
	}
	return warnings
}

// checkShapeGridUnknownKeys checks a shape grid and its nested structures: the
// slide's shape_grid, and through checkGridCellUnknownKeys the grid of any cell
// at any depth, so a finding inside one is named by the full pointer to it
// (.../cells/0/grid/rows/0/cells/1/shape/fil).
func checkShapeGridUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.ShapeGridInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["bounds"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.GridBoundsInput{}), path+"/bounds")...)
	}
	if rowsRaw, ok := obj["rows"]; ok {
		var rows []json.RawMessage
		if json.Unmarshal(rowsRaw, &rows) == nil {
			for i, rowRaw := range rows {
				p := fmt.Sprintf("%s/rows/%d", path, i)
				warnings = append(warnings, checkGridRowUnknownKeys(rowRaw, p)...)
			}
		}
	}
	if linksRaw, ok := obj["links"]; ok {
		var links []json.RawMessage
		if json.Unmarshal(linksRaw, &links) == nil {
			for i, linkRaw := range links {
				p := fmt.Sprintf("%s/links/%d", path, i)
				warnings = append(warnings, checkUnknownKeysForType(linkRaw, reflect.TypeOf(jsonschema.GridLinkInput{}), p)...)
				var link map[string]json.RawMessage
				if json.Unmarshal(linkRaw, &link) == nil {
					if v, ok := link["connector"]; ok {
						warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.ConnectorSpecInput{}), p+"/connector")...)
					}
				}
			}
		}
	}
	return warnings
}

// checkGridRowUnknownKeys checks a shape_grid row.
func checkGridRowUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.GridRowInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["connector"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.ConnectorSpecInput{}), path+"/connector")...)
	}
	if cellsRaw, ok := obj["cells"]; ok {
		var cells []json.RawMessage
		if json.Unmarshal(cellsRaw, &cells) == nil {
			for i, cellRaw := range cells {
				p := fmt.Sprintf("%s/cells/%d", path, i)
				warnings = append(warnings, checkGridCellUnknownKeys(cellRaw, p)...)
			}
		}
	}
	return warnings
}

// checkGridCellUnknownKeys checks a shape_grid cell and every container it can
// hold: shape, table, icon, image, accent_bar, diagram, composite, a nested
// pattern block, a nested grid (recursively) and layers.
func checkGridCellUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.GridCellInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["shape"]; ok {
		warnings = append(warnings, checkShapeUnknownKeys(v, path+"/shape")...)
	}
	if v, ok := obj["table"]; ok {
		warnings = append(warnings, checkTableUnknownKeys(v, path+"/table")...)
	}
	if v, ok := obj["icon"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.IconInput{}), path+"/icon")...)
	}
	if v, ok := obj["image"]; ok {
		warnings = append(warnings, checkGridImageUnknownKeys(v, path+"/image")...)
	}
	if v, ok := obj["accent_bar"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.AccentBarInput{}), path+"/accent_bar")...)
	}
	if v, ok := obj["diagram"]; ok {
		warnings = append(warnings, checkDiagramUnknownKeys(v, path+"/diagram")...)
	}
	if v, ok := obj["composite"]; ok {
		warnings = append(warnings, checkCompositeUnknownKeys(v, path+"/composite")...)
	}
	if v, ok := obj["pattern"]; ok {
		warnings = append(warnings, checkPatternBlockUnknownKeys(v, path+"/pattern")...)
	}
	if v, ok := obj["grid"]; ok {
		warnings = append(warnings, checkShapeGridUnknownKeys(v, path+"/grid")...)
	}
	if v, ok := obj["layers"]; ok {
		var layers []json.RawMessage
		if json.Unmarshal(v, &layers) == nil {
			for i, layerRaw := range layers {
				warnings = append(warnings, checkGridLayerUnknownKeys(layerRaw, fmt.Sprintf("%s/layers/%d", path, i))...)
			}
		}
	}
	return warnings
}

// checkGridLayerUnknownKeys checks one layer of a shape_grid cell.
func checkGridLayerUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	warnings := checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.LayerInput{}), path)
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["frame"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.LayerFrameInput{}), path+"/frame")...)
	}
	if v, ok := obj["shape"]; ok {
		warnings = append(warnings, checkShapeUnknownKeys(v, path+"/shape")...)
	}
	return warnings
}

// checkShapeUnknownKeys checks a shape spec with its icon overlay, link and the
// object forms of its fill, line and text: a cell's shape, a layer's shape or
// a composite's text. The object keys are read from the types the writer
// decodes them into (shapegrid.ResolveFillInput and its siblings); a string
// shorthand has no keys and reports nothing.
func checkShapeUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	warnings := checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.ShapeSpecInput{}), path)
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["icon"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.IconInput{}), path+"/icon")...)
	}
	if v, ok := obj["link"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.LinkInput{}), path+"/link")...)
	}
	if v, ok := obj["fill"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(shapegrid.FillObjectInput{}), path+"/fill")...)
	}
	if v, ok := obj["line"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(shapegrid.LineObjectInput{}), path+"/line")...)
	}
	if v, ok := obj["text"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(shapegrid.TextObjectInput{}), path+"/text")...)
		var text struct {
			Paragraphs []json.RawMessage `json:"paragraphs"`
		}
		if json.Unmarshal(v, &text) == nil {
			for i, para := range text.Paragraphs {
				warnings = append(warnings, checkUnknownKeysForType(para, reflect.TypeOf(shapegrid.ParagraphInput{}), fmt.Sprintf("%s/text/paragraphs/%d", path, i))...)
			}
		}
	}
	return warnings
}

// checkOverlayUnknownKeys checks one slide overlay: the shape, its from / to
// points with their cell and image anchors, and its link.
func checkOverlayUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	warnings := checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.OverlayShapeInput{}), path)
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	for _, end := range []string{"from", "to"} {
		v, ok := obj[end]
		if !ok {
			continue
		}
		p := path + "/" + end
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.OverlayPointInput{}), p)...)
		var point map[string]json.RawMessage
		if json.Unmarshal(v, &point) != nil {
			continue
		}
		if a, ok := point["anchor_cell"]; ok {
			warnings = append(warnings, checkUnknownKeysForType(a, reflect.TypeOf(jsonschema.OverlayAnchorCellInput{}), p+"/anchor_cell")...)
		}
		if a, ok := point["anchor_image"]; ok {
			warnings = append(warnings, checkUnknownKeysForType(a, reflect.TypeOf(jsonschema.OverlayAnchorImageInput{}), p+"/anchor_image")...)
		}
	}
	if v, ok := obj["link"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.LinkInput{}), path+"/link")...)
	}
	return warnings
}

// checkCompositeUnknownKeys checks a cell's composite: the envelope, its text
// shape and its sub_diagram.
func checkCompositeUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	warnings := checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.CompositeInput{}), path)
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["text"]; ok {
		warnings = append(warnings, checkShapeUnknownKeys(v, path+"/text")...)
	}
	if v, ok := obj["sub_diagram"]; ok {
		warnings = append(warnings, checkDiagramUnknownKeys(v, path+"/sub_diagram")...)
	}
	return warnings
}

// checkGridImageUnknownKeys checks image in a shape_grid cell.
func checkGridImageUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.GridImageInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["overlay"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.GridOverlayInput{}), path+"/overlay")...)
	}
	if v, ok := obj["text"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.GridImageTextInput{}), path+"/text")...)
	}
	return warnings
}

// typedFieldForType maps content type to the corresponding typed value JSON key.
var typedFieldForType = map[string]string{
	"text":             "text_value",
	"bullets":          "bullets_value",
	"body_and_bullets": "body_and_bullets_value",
	"body_and_lead":    "body_and_lead_value",
	"bullet_groups":    "bullet_groups_value",
	"table":            "table_value",
	"chart":            "chart_value",
	"diagram":          "diagram_value",
	"image":            "image_value",
}

// checkRedundantValue warns when a content item has both a typed value field
// and the legacy "value" field set. The typed field wins (per ResolveValue),
// but the agent should know that "value" is being ignored.
func checkRedundantValue(obj map[string]json.RawMessage, path string) []*patterns.ValidationError {
	valueRaw, hasValue := obj["value"]
	if !hasValue || len(valueRaw) == 0 {
		return nil
	}

	typeRaw, ok := obj["type"]
	if !ok {
		return nil
	}
	var contentType string
	if json.Unmarshal(typeRaw, &contentType) != nil {
		return nil
	}

	typedField, ok := typedFieldForType[contentType]
	if !ok {
		return nil
	}

	if _, hasTyped := obj[typedField]; !hasTyped {
		return nil
	}

	return []*patterns.ValidationError{{
		Path:    path,
		Code:    "redundant_field",
		Message: fmt.Sprintf("%s: both %s and value set; using %s (value is ignored)", path, typedField, typedField),
		Fix: &patterns.FixSuggestion{
			Kind:   "remove_field",
			Params: map[string]any{"field": "value"},
		},
	}}
}

// checkChartStyleUnknownKeys walks the style / chart_style sub-objects of a
// chart or diagram value. Their keys were never checked, so a deck that set
// style.palette (svggen's name — the engine's is colors) or style.show_grid got
// silence even under --strict-unknown-keys, and the agent had no way to learn
// that the knob it reached for does not exist (go-slide-creator-z72f).
func checkChartStyleUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	var warnings []*patterns.ValidationError
	if v, ok := obj["style"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(types.ChartStyle{}), path+"/style")...)
		warnings = append(warnings, checkValueFormatUnknownKeys(v, path+"/style")...)
	}
	if v, ok := obj["chart_style"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(types.ChartStyleOverrides{}), path+"/chart_style")...)
	}
	return warnings
}

// checkValueFormatUnknownKeys walks style.value_format. A misspelled key there
// is silently dropped by the decoder and the chart renders with the default
// format, which looks like the argument was ignored — because it was
// (go-slide-creator-e2ck9).
func checkValueFormatUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	v, ok := obj["value_format"]
	if !ok {
		return nil
	}
	return checkUnknownKeysForType(v, reflect.TypeOf(types.ValueFormatSpec{}), path+"/value_format")
}
