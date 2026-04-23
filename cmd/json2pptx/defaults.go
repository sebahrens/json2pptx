package main

import "github.com/sebahrens/json2pptx/internal/jsonschema"

// applyDefaults shallow-applies deck-level defaults onto every matching block
// in the presentation. Must be called after JSON unmarshal but before struct
// validation or conversion. Swap-only semantics: an inline field always wins;
// a missing field is filled from the default.
func applyDefaults(input *PresentationInput) {
	if input == nil || input.Defaults == nil {
		return
	}
	d := input.Defaults
	for i := range input.Slides {
		s := &input.Slides[i]
		// Content-level tables (e.g., type:"table" with table_value).
		for j := range s.Content {
			c := &s.Content[j]
			if c.Type == "table" && c.TableValue != nil && d.TableStyle != nil {
				applyTableStyleDefaults(c.TableValue, d.TableStyle)
			}
		}
		// Shape grid: tables embedded in cells, and shape defaults.
		if s.ShapeGrid != nil {
			applyShapeGridDefaults(s.ShapeGrid, d)
		}
	}
}

// applyTableStyleDefaults shallow-merges default table style fields onto a table.
func applyTableStyleDefaults(t *jsonschema.TableInput, def *TableStyleInput) {
	if t.Style == nil {
		// No inline style at all — adopt the full default.
		t.Style = &jsonschema.TableStyleInput{
			HeaderBackground: def.HeaderBackground,
			Borders:          def.Borders,
			Striped:          def.Striped,
			UseTableStyle:    def.UseTableStyle,
			StyleID:          def.StyleID,
		}
		return
	}
	// Swap-only: each zero-value field is filled from the default.
	if t.Style.HeaderBackground == nil && def.HeaderBackground != nil {
		t.Style.HeaderBackground = def.HeaderBackground
	}
	if t.Style.Borders == "" && def.Borders != "" {
		t.Style.Borders = def.Borders
	}
	if t.Style.Striped == nil && def.Striped != nil {
		t.Style.Striped = def.Striped
	}
	if !t.Style.UseTableStyle && def.UseTableStyle {
		t.Style.UseTableStyle = def.UseTableStyle
	}
	if t.Style.StyleID == "" && def.StyleID != "" {
		t.Style.StyleID = def.StyleID
	}
}

// applyShapeGridDefaults walks a shape grid and applies table_style and
// cell_style defaults to embedded tables and shape cells respectively.
func applyShapeGridDefaults(sg *jsonschema.ShapeGridInput, d *DefaultsInput) {
	for ri := range sg.Rows {
		for ci := range sg.Rows[ri].Cells {
			cell := sg.Rows[ri].Cells[ci]
			if cell == nil {
				continue
			}
			// Tables embedded in grid cells.
			if cell.Table != nil && d.TableStyle != nil {
				applyTableStyleDefaults(cell.Table, d.TableStyle)
			}
			// Shape defaults for grid cells.
			if cell.Shape != nil && d.CellStyle != nil {
				applyCellStyleDefaults(cell.Shape, d.CellStyle)
			}
		}
	}
}

// applyCellStyleDefaults shallow-merges default shape properties onto a cell's shape.
func applyCellStyleDefaults(shape *jsonschema.ShapeSpecInput, def *jsonschema.ShapeSpecInput) {
	if shape.Geometry == "" && def.Geometry != "" {
		shape.Geometry = def.Geometry
	}
	if len(shape.Fill) == 0 && len(def.Fill) > 0 {
		shape.Fill = def.Fill
	}
	if len(shape.Line) == 0 && len(def.Line) > 0 {
		shape.Line = def.Line
	}
	if len(shape.Text) == 0 && len(def.Text) > 0 {
		shape.Text = def.Text
	}
	if shape.Rotation == 0 && def.Rotation != 0 {
		shape.Rotation = def.Rotation
	}
	if shape.Adjustments == nil && def.Adjustments != nil {
		shape.Adjustments = def.Adjustments
	}
	if shape.Icon == nil && def.Icon != nil {
		shape.Icon = def.Icon
	}
}
