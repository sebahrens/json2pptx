package main

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/templatesettings"
)

// resolveNamedSettings resolves named style references in the presentation
// against the template's settings file. A table style_id that is not a GUID
// and not @template-default is treated as a named table style lookup. Similarly,
// shape cells with no fill/border/text can inherit from a named cell style.
//
// This runs AFTER applyDefaults (deck-level defaults win) and fills in named
// lookups as priority 3 in the resolution chain.
func resolveNamedSettings(input *PresentationInput, settings *templatesettings.File) {
	if input == nil || settings == nil {
		return
	}
	for i := range input.Slides {
		s := &input.Slides[i]
		// Content-level tables.
		for j := range s.Content {
			c := &s.Content[j]
			if c.Type == "table" && c.TableValue != nil {
				resolveTableNamedStyle(c.TableValue, settings)
			}
		}
		// Shape grid: tables and cell shapes.
		if s.ShapeGrid != nil {
			resolveShapeGridNamedSettings(s.ShapeGrid, settings)
		}
	}
}

// resolveTableNamedStyle checks if the table's style_id is a named reference
// (not a GUID, not @template-default) and expands it from the settings file.
func resolveTableNamedStyle(t *jsonschema.TableInput, settings *templatesettings.File) {
	if t.Style == nil || t.Style.StyleID == "" {
		return
	}
	sid := t.Style.StyleID

	// GUIDs start with '{', @template-default is a sentinel — skip those.
	if strings.HasPrefix(sid, "{") || sid == template.TemplateDefaultSentinel {
		return
	}

	def, ok := settings.TableStyles[sid]
	if !ok {
		return // Unknown name; downstream validation will report it.
	}

	// Apply the named definition, preserving any inline overrides (swap-only).
	if t.Style.StyleID == sid {
		// Replace the name with the resolved style_id from the definition.
		t.Style.StyleID = def.StyleID
	}
	if !t.Style.UseTableStyle && def.UseTableStyle {
		t.Style.UseTableStyle = def.UseTableStyle
	}
	if t.Style.Striped == nil && def.BandedRows != nil {
		t.Style.Striped = def.BandedRows
	}
}

// resolveShapeGridNamedSettings walks a shape grid and resolves named references
// for tables and cell shapes.
func resolveShapeGridNamedSettings(sg *jsonschema.ShapeGridInput, settings *templatesettings.File) {
	for ri := range sg.Rows {
		for ci := range sg.Rows[ri].Cells {
			cell := sg.Rows[ri].Cells[ci]
			if cell == nil {
				continue
			}
			if cell.Table != nil {
				resolveTableNamedStyle(cell.Table, settings)
			}
			if cell.Shape != nil && cell.NamedStyle != "" {
				resolveShapeNamedStyle(cell, settings)
			}
		}
	}
}

// resolveShapeNamedStyle expands a named cell style reference on a grid cell.
func resolveShapeNamedStyle(cell *jsonschema.GridCellInput, settings *templatesettings.File) {
	def, ok := settings.CellStyles[cell.NamedStyle]
	if !ok {
		return // Unknown name; downstream may warn.
	}

	shape := cell.Shape

	// Apply fill if not already set.
	if len(shape.Fill) == 0 && def.Fill != nil {
		fill, err := json.Marshal(def.Fill)
		if err == nil {
			shape.Fill = fill
		}
	}

	// Apply border as line if not already set.
	if len(shape.Line) == 0 && def.Border != nil {
		line, err := json.Marshal(def.Border)
		if err == nil {
			shape.Line = line
		}
	}

	// Apply text_align if shape text is a string (not already an object with align).
	// This is best-effort — text alignment is inside the text object.
	// We leave it for now since the shape text structure is complex.
}
