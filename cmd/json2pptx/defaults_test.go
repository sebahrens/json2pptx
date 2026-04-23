package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func TestApplyDefaults_NilInputNoPanic(t *testing.T) {
	applyDefaults(nil)
	applyDefaults(&PresentationInput{})
	applyDefaults(&PresentationInput{Defaults: &DefaultsInput{}})
}

func TestApplyDefaults_TableStyleOnContentTable(t *testing.T) {
	input := &PresentationInput{
		Defaults: &DefaultsInput{
			TableStyle: &jsonschema.TableStyleInput{
				UseTableStyle: true,
				StyleID:       "@template-default",
				Borders:       "all",
			},
		},
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type: "table",
						TableValue: &jsonschema.TableInput{
							Headers: []string{"A", "B"},
							Rows:    [][]jsonschema.TableCellInput{{{Content: "1"}, {Content: "2"}}},
							// No inline style — should get full defaults.
						},
					},
				},
			},
		},
	}
	applyDefaults(input)

	style := input.Slides[0].Content[0].TableValue.Style
	if style == nil {
		t.Fatal("expected style to be set from defaults")
	}
	if !style.UseTableStyle {
		t.Error("expected UseTableStyle=true from defaults")
	}
	if style.StyleID != "@template-default" {
		t.Errorf("expected StyleID=@template-default, got %q", style.StyleID)
	}
	if style.Borders != "all" {
		t.Errorf("expected Borders=all, got %q", style.Borders)
	}
}

func TestApplyDefaults_TableStyleSwapSemantics(t *testing.T) {
	input := &PresentationInput{
		Defaults: &DefaultsInput{
			TableStyle: &jsonschema.TableStyleInput{
				UseTableStyle: true,
				StyleID:       "@template-default",
				Borders:       "all",
				Striped:       boolPtr(true),
			},
		},
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type: "table",
						TableValue: &jsonschema.TableInput{
							Headers: []string{"A"},
							Rows:    [][]jsonschema.TableCellInput{{{Content: "1"}}},
							Style: &jsonschema.TableStyleInput{
								// Inline sets borders — this should WIN over default.
								Borders: "none",
								// StyleID not set — should get default.
							},
						},
					},
				},
			},
		},
	}
	applyDefaults(input)

	style := input.Slides[0].Content[0].TableValue.Style
	if style.Borders != "none" {
		t.Errorf("expected inline Borders=none to win, got %q", style.Borders)
	}
	if style.StyleID != "@template-default" {
		t.Errorf("expected StyleID=@template-default from defaults, got %q", style.StyleID)
	}
	if !style.UseTableStyle {
		t.Error("expected UseTableStyle=true from defaults")
	}
	if style.Striped == nil || !*style.Striped {
		t.Error("expected Striped=true from defaults")
	}
}

func TestApplyDefaults_ShapeGridTableAndCellStyle(t *testing.T) {
	defaultFill := json.RawMessage(`"accent2"`)

	input := &PresentationInput{
		Defaults: &DefaultsInput{
			TableStyle: &jsonschema.TableStyleInput{
				UseTableStyle: true,
				StyleID:       "@template-default",
			},
			CellStyle: &jsonschema.ShapeSpecInput{
				Geometry: "roundRect",
				Fill:     defaultFill,
			},
		},
		Slides: []SlideInput{
			{
				ShapeGrid: &jsonschema.ShapeGridInput{
					Rows: []jsonschema.GridRowInput{
						{
							Cells: []*jsonschema.GridCellInput{
								{
									// Table in grid — should get table_style defaults.
									Table: &jsonschema.TableInput{
										Headers: []string{"X"},
										Rows:    [][]jsonschema.TableCellInput{{{Content: "v"}}},
									},
								},
								{
									// Shape with no fill — should get cell_style defaults.
									Shape: &jsonschema.ShapeSpecInput{
										Text: json.RawMessage(`{"content":"hello"}`),
									},
								},
								{
									// Shape with inline fill — should keep its fill.
									Shape: &jsonschema.ShapeSpecInput{
										Geometry: "rect",
										Fill:     json.RawMessage(`"accent5"`),
									},
								},
								nil, // nil cell — should not panic.
							},
						},
					},
				},
			},
		},
	}
	applyDefaults(input)

	// Check table in grid got defaults.
	tbl := input.Slides[0].ShapeGrid.Rows[0].Cells[0].Table
	if tbl.Style == nil || !tbl.Style.UseTableStyle {
		t.Error("expected table in grid to get UseTableStyle=true from defaults")
	}

	// Check shape with no fill got defaults.
	shape1 := input.Slides[0].ShapeGrid.Rows[0].Cells[1].Shape
	if shape1.Geometry != "roundRect" {
		t.Errorf("expected geometry=roundRect from defaults, got %q", shape1.Geometry)
	}
	if string(shape1.Fill) != `"accent2"` {
		t.Errorf("expected fill=accent2 from defaults, got %s", shape1.Fill)
	}

	// Check shape with inline fill kept its own.
	shape2 := input.Slides[0].ShapeGrid.Rows[0].Cells[2].Shape
	if shape2.Geometry != "rect" {
		t.Errorf("expected geometry=rect (inline), got %q", shape2.Geometry)
	}
	if string(shape2.Fill) != `"accent5"` {
		t.Errorf("expected fill=accent5 (inline), got %s", shape2.Fill)
	}
}

func TestApplyDefaults_NoDefaultsNoChange(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type: "table",
						TableValue: &jsonschema.TableInput{
							Headers: []string{"H"},
							Rows:    [][]jsonschema.TableCellInput{{{Content: "c"}}},
						},
					},
				},
			},
		},
	}
	applyDefaults(input)

	// No style should be set when no defaults.
	if input.Slides[0].Content[0].TableValue.Style != nil {
		t.Error("expected no style when defaults is nil")
	}
}

func TestApplyDefaults_RoundTripJSON(t *testing.T) {
	// Verify defaults survive JSON round-trip.
	src := `{
		"template": "midnight-blue",
		"defaults": {
			"table_style": {
				"use_table_style": true,
				"style_id": "@template-default"
			},
			"cell_style": {
				"geometry": "roundRect",
				"fill": "accent1"
			}
		},
		"slides": [
			{
				"slide_type": "content",
				"content": [
					{
						"placeholder_id": "body",
						"type": "table",
						"table_value": {
							"headers": ["Col"],
							"rows": [["val"]]
						}
					}
				]
			}
		]
	}`

	var input PresentationInput
	if err := json.Unmarshal([]byte(src), &input); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if input.Defaults == nil {
		t.Fatal("expected Defaults to be parsed")
	}
	if input.Defaults.TableStyle == nil {
		t.Fatal("expected TableStyle default")
	}
	if input.Defaults.TableStyle.StyleID != "@template-default" {
		t.Errorf("expected StyleID=@template-default, got %q", input.Defaults.TableStyle.StyleID)
	}
	if input.Defaults.CellStyle == nil {
		t.Fatal("expected CellStyle default")
	}
	if input.Defaults.CellStyle.Geometry != "roundRect" {
		t.Errorf("expected geometry=roundRect, got %q", input.Defaults.CellStyle.Geometry)
	}

	applyDefaults(&input)

	// Table should now have the defaults applied.
	style := input.Slides[0].Content[0].TableValue.Style
	if style == nil || !style.UseTableStyle {
		t.Error("expected table style defaults to be applied after round-trip")
	}
}

func TestApplyDefaults_HeaderBackgroundSwap(t *testing.T) {
	input := &PresentationInput{
		Defaults: &DefaultsInput{
			TableStyle: &jsonschema.TableStyleInput{
				HeaderBackground: strPtr("accent1"),
			},
		},
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type: "table",
						TableValue: &jsonschema.TableInput{
							Headers: []string{"H"},
							Rows:    [][]jsonschema.TableCellInput{{{Content: "c"}}},
							Style: &jsonschema.TableStyleInput{
								HeaderBackground: strPtr("accent3"),
							},
						},
					},
				},
			},
		},
	}
	applyDefaults(input)

	// Inline header_background should win.
	bg := input.Slides[0].Content[0].TableValue.Style.HeaderBackground
	if bg == nil || *bg != "accent3" {
		t.Errorf("expected inline HeaderBackground=accent3, got %v", bg)
	}
}
