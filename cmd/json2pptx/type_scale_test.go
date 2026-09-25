package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

const beforeAfterTypeScaleValues = `{"before":{"header":"Today","items":["Manual handoffs","Five-day close"]},"after":{"header":"Target","items":["Automated routing","Two-day close"]}}`

func TestPatternTypeScaleOverrideWinsAndIsDiscoverable(t *testing.T) {
	pat, ok := patterns.Default().Get("before-after")
	if !ok {
		t.Fatal("before-after pattern missing")
	}
	if !strings.Contains(string(patterns.SchemaJSON(pat)), `"type_scale"`) {
		t.Fatal("pattern discovery schema omits generic type_scale override")
	}
	for _, tc := range []struct {
		name, override, inherited, want string
	}{
		{"inherited", `{}`, "comfortable", "comfortable"},
		{"explicit presentation", `{"type_scale":"presentation"}`, "comfortable", "presentation"},
		{"explicit compact", `{"type_scale":"compact"}`, "presentation", "compact"},
		{"alongside authored override", `{"accent":"accent2","type_scale":"presentation"}`, "comfortable", "presentation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pi := &PatternInput{Name: "before-after", Values: json.RawMessage(beforeAfterTypeScaleValues), Overrides: json.RawMessage(tc.override), DefaultTypeScale: tc.inherited}
			if findings := patterns.InspectPatternInput(pat, pi.Values, pi.Overrides, nil); len(findings) != 0 {
				t.Fatalf("valid generic override was rejected: %+v", findings)
			}
			grid, _, err := expandPattern(pi, patterns.ExpandContext{}, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			if grid.TypeScale != tc.want || grid.Rows[1].Cells[0].Shape.TypeScale != tc.want {
				t.Errorf("grid/shape type scale = %q/%q, want %q", grid.TypeScale, grid.Rows[1].Cells[0].Shape.TypeScale, tc.want)
			}
		})
	}
}

func TestPatternTypeScaleRejectsInvalidOverride(t *testing.T) {
	pi := &PatternInput{Name: "before-after", Values: json.RawMessage(beforeAfterTypeScaleValues), Overrides: json.RawMessage(`{"type_scale":"giant"}`)}
	_, _, err := expandPattern(pi, patterns.ExpandContext{}, patterns.Default())
	if err == nil || !strings.Contains(err.Error(), "overrides.type_scale") {
		t.Fatalf("invalid override needs a path-scoped finding, got %v", err)
	}
}

func TestDeckTypeScaleMatchesRenderedAndPreflightGrid(t *testing.T) {
	text := json.RawMessage(`{"content":"• Manual handoffs\n• Five-day close\n• Fragmented reporting\n• Reconciliation delays","size":12}`)
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows:    []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Text: text}}}}},
	}
	input := &PresentationInput{TypeScale: "presentation", Slides: []SlideInput{{ShapeGrid: grid}}}
	applyDefaults(input)
	if grid.TypeScale != "presentation" {
		t.Fatalf("deck type_scale was not inherited by a raw grid: %q", grid.TypeScale)
	}
	structural := resolveGridForStructural(grid, nil, nil, 12192000, 6858000)
	if structural == nil || len(structural.Cells) != 1 {
		t.Fatal("preflight did not resolve grid")
	}
	preflightText, err := shapegrid.ResolveTextInput(structural.Cells[0].ShapeSpec.Text)
	if err != nil {
		t.Fatal(err)
	}
	if got := preflightText.Paragraphs[0].Runs[0].FontSize; got != 1800 {
		t.Errorf("preflight font = %d, want 1800", got)
	}
	rendered, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), nil, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered.Shapes) != 1 || !strings.Contains(string(rendered.Shapes[0]), `sz="1800"`) {
		t.Fatal("rendered OOXML does not use the same grown size as preflight")
	}
	if string(grid.Rows[0].Cells[0].Shape.Text) != string(text) {
		t.Fatal("rendering mutated authored text")
	}
}

func TestTypeScaleEnumValidation(t *testing.T) {
	input := &PresentationInput{TypeScale: "giant", Slides: []SlideInput{{ShapeGrid: &ShapeGridInput{TypeScale: "tiny"}}}}
	errs := checkInputEnumValues(input)
	if len(errs) != 2 || errs[0].Path != "type_scale" || errs[1].Path != "/slides/0/shape_grid/type_scale" {
		t.Fatalf("invalid type scales not reported at their paths: %+v", errs)
	}
}

func TestTypeScaleSurvivesComposeWithPerSegmentOverrides(t *testing.T) {
	compose := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
		{Pattern: PatternInput{Name: "before-after", Values: json.RawMessage(beforeAfterTypeScaleValues), Overrides: json.RawMessage(`{"type_scale":"compact"}`)}},
		{Pattern: PatternInput{Name: "before-after", Values: json.RawMessage(beforeAfterTypeScaleValues), Overrides: json.RawMessage(`{"type_scale":"presentation"}`)}},
	}}
	input := &PresentationInput{TypeScale: "comfortable", Slides: []SlideInput{{Compose: compose}}}
	applyDefaults(input)
	grid, _, err := expandCompose(compose, patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: patterns.LayoutBounds{Width: 11000000, Height: 5000000}}, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	var countShapes func(*ShapeGridInput)
	countShapes = func(g *ShapeGridInput) {
		if g == nil {
			return
		}
		for _, row := range g.Rows {
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				if cell.Shape != nil {
					counts[cell.Shape.TypeScale]++
				}
				countShapes(cell.Grid)
			}
		}
	}
	countShapes(grid)
	if counts["compact"] == 0 || counts["presentation"] == 0 || counts["comfortable"] != 0 {
		t.Fatalf("compose discarded segment type-scale overrides: %v", counts)
	}
}

func TestNestedPatternInheritsDeckTypeScale(t *testing.T) {
	pattern, err := json.Marshal(PatternInput{Name: "before-after", Values: json.RawMessage(beforeAfterTypeScaleValues)})
	if err != nil {
		t.Fatal(err)
	}
	grid := &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Pattern: pattern}}}}}
	input := &PresentationInput{TypeScale: "presentation", Slides: []SlideInput{{ShapeGrid: grid}}}
	applyDefaults(input)
	if err := expandNestedCellPatterns(grid, patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, patterns.Default()); err != nil {
		t.Fatal(err)
	}
	child := grid.Rows[0].Cells[0].Grid
	if child == nil || child.TypeScale != "presentation" || child.Rows[1].Cells[0].Shape.TypeScale != "presentation" {
		t.Fatalf("nested pattern lost deck type_scale: %+v", child)
	}
}
