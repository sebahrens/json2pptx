package main

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// Hierarchical unknown-key check for the full PresentationInput tree.
//
// Walks raw JSON at every object level and reports unknown fields as
// ValidationError warnings. Integrated into validate and MCP
// validate/generate paths as warnings (not errors) for the initial release.
// ---------------------------------------------------------------------------

// checkInputUnknownKeys runs unknown-key detection on the full
// PresentationInput JSON tree. Returns warnings (not errors) for every
// unknown field found.
func checkInputUnknownKeys(raw json.RawMessage) []*patterns.ValidationError {
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
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(JSONFooter{}), "footer")...)
	}

	// theme_override
	if v, ok := top["theme_override"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(ThemeInput{}), "theme_override")...)
	}

	// defaults
	if v, ok := top["defaults"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(DefaultsInput{}), "defaults")...)
	}

	// slides[]
	if slidesRaw, ok := top["slides"]; ok {
		var slides []json.RawMessage
		if json.Unmarshal(slidesRaw, &slides) == nil {
			for i, slideRaw := range slides {
				prefix := fmt.Sprintf("slides[%d]", i)
				warnings = append(warnings, checkSlideUnknownKeys(slideRaw, prefix)...)
			}
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

	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(SlideInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}

	// background
	if v, ok := obj["background"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(BackgroundInput{}), path+".background")...)
	}

	// pattern
	if v, ok := obj["pattern"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(PatternInput{}), path+".pattern")...)
	}

	// shape_grid
	if v, ok := obj["shape_grid"]; ok {
		warnings = append(warnings, checkShapeGridUnknownKeys(v, path+".shape_grid")...)
	}

	// content[]
	if contentRaw, ok := obj["content"]; ok {
		var items []json.RawMessage
		if json.Unmarshal(contentRaw, &items) == nil {
			for j, itemRaw := range items {
				p := fmt.Sprintf("%s.content[%d]", path, j)
				warnings = append(warnings, checkContentUnknownKeys(itemRaw, p)...)
			}
		}
	}

	return warnings
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
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(SplitConfig{}), path+".split")...)
	}
	if v, ok := obj["base"]; ok {
		warnings = append(warnings, checkSlideUnknownKeys(v, path+".base")...)
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

	if v, ok := obj["body_and_bullets_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(BodyAndBulletsInput{}), path+".body_and_bullets_value")...)
	}
	if v, ok := obj["bullet_groups_value"]; ok {
		warnings = append(warnings, checkBulletGroupsUnknownKeys(v, path+".bullet_groups_value")...)
	}
	if v, ok := obj["table_value"]; ok {
		warnings = append(warnings, checkTableUnknownKeys(v, path+".table_value")...)
	}
	if v, ok := obj["image_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(ImageInput{}), path+".image_value")...)
	}
	if v, ok := obj["chart_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(types.ChartSpec{}), path+".chart_value")...) //nolint:staticcheck // ChartSpec is deprecated but still used for backward compat
	}
	if v, ok := obj["diagram_value"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(types.DiagramSpec{}), path+".diagram_value")...)
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
				p := fmt.Sprintf("%s.groups[%d]", path, i)
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
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.TableStyleInput{}), path+".style")...)
	}
	if rowsRaw, ok := obj["rows"]; ok {
		var rows []json.RawMessage
		if json.Unmarshal(rowsRaw, &rows) == nil {
			for i, rowRaw := range rows {
				var cells []json.RawMessage
				if json.Unmarshal(rowRaw, &cells) == nil {
					for j, cellRaw := range cells {
						p := fmt.Sprintf("%s.rows[%d][%d]", path, i, j)
						warnings = append(warnings, checkUnknownKeysForType(cellRaw, reflect.TypeOf(jsonschema.TableCellInput{}), p)...)
					}
				}
			}
		}
	}
	return warnings
}

// checkShapeGridUnknownKeys checks shape_grid and its nested structures.
func checkShapeGridUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.ShapeGridInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["bounds"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.GridBoundsInput{}), path+".bounds")...)
	}
	if rowsRaw, ok := obj["rows"]; ok {
		var rows []json.RawMessage
		if json.Unmarshal(rowsRaw, &rows) == nil {
			for i, rowRaw := range rows {
				p := fmt.Sprintf("%s.rows[%d]", path, i)
				warnings = append(warnings, checkGridRowUnknownKeys(rowRaw, p)...)
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
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.ConnectorSpecInput{}), path+".connector")...)
	}
	if cellsRaw, ok := obj["cells"]; ok {
		var cells []json.RawMessage
		if json.Unmarshal(cellsRaw, &cells) == nil {
			for i, cellRaw := range cells {
				p := fmt.Sprintf("%s.cells[%d]", path, i)
				warnings = append(warnings, checkGridCellUnknownKeys(cellRaw, p)...)
			}
		}
	}
	return warnings
}

// checkGridCellUnknownKeys checks a shape_grid cell.
func checkGridCellUnknownKeys(raw json.RawMessage, path string) []*patterns.ValidationError {
	var warnings []*patterns.ValidationError
	warnings = append(warnings, checkUnknownKeysForType(raw, reflect.TypeOf(jsonschema.GridCellInput{}), path)...)

	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return warnings
	}
	if v, ok := obj["shape"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.ShapeSpecInput{}), path+".shape")...)
	}
	if v, ok := obj["table"]; ok {
		warnings = append(warnings, checkTableUnknownKeys(v, path+".table")...)
	}
	if v, ok := obj["icon"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.IconInput{}), path+".icon")...)
	}
	if v, ok := obj["image"]; ok {
		warnings = append(warnings, checkGridImageUnknownKeys(v, path+".image")...)
	}
	if v, ok := obj["accent_bar"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.AccentBarInput{}), path+".accent_bar")...)
	}
	if v, ok := obj["diagram"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(types.DiagramSpec{}), path+".diagram")...)
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
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.GridOverlayInput{}), path+".overlay")...)
	}
	if v, ok := obj["text"]; ok {
		warnings = append(warnings, checkUnknownKeysForType(v, reflect.TypeOf(jsonschema.GridImageTextInput{}), path+".text")...)
	}
	return warnings
}
