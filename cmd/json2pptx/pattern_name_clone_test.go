package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestPatternNameRejectsMalformedDiscriminator(t *testing.T) {
	for _, raw := range []string{``, `null`, `[]`, `{"name":4}`, `{"values":[]}`, `{"name":" "}`, `{"name":`} {
		if _, err := patternName(json.RawMessage(raw)); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if got, err := patternName(json.RawMessage(`{"name":"stat-hero","values":{"value":"7"}}`)); err != nil || got != "stat-hero" {
		t.Fatalf("valid pattern: %q %v", got, err)
	}
}

func TestComposeNativeCloneRetainsMalformedPatternAndFontMetadata(t *testing.T) {
	raw := json.RawMessage(`{"name":`)
	source := &ShapeGridInput{KeepTextSizes: true, BoundsRelativeToContentArea: true, VerifiedMatrixPattern: "comparison", Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Pattern: raw}, {Shape: &jsonschema.ShapeSpecInput{MeasureFonts: jsonschema.MeasureFonts{Minor: "Lora"}}}}}}}
	clone := composeNativeLeaf(SegmentInput{ShapeGrid: source})
	if !clone.KeepTextSizes || !clone.BoundsRelativeToContentArea || clone.VerifiedMatrixPattern != "comparison" || clone.Rows[0].Cells[1].Shape.MeasureFonts.Minor != "Lora" || string(clone.Columns) != "2" || len(source.Columns) != 0 {
		t.Fatal("compose dropped provenance or changed input")
	}
	out := nestedPatternCells{}
	err := collectNestedPatternCells(clone, "/slides/0/shape_grid", out, 0)
	if err == nil || !strings.Contains(err.Error(), "/slides/0/shape_grid/rows/0/cells/0/pattern") {
		t.Fatalf("malformed nested pattern lost its error path: %v", err)
	}
	if len(out) != 0 {
		t.Fatal("malformed pattern treated as unnamed pattern")
	}
	if _, err := regionVisualMinPct(clone.Rows[0].Cells[0]); err == nil {
		t.Fatal("malformed region accepted as nonpattern")
	}
}

func TestPatternAccentContextRejectsMalformedOverrides(t *testing.T) {
	for _, raw := range []string{`{"accent":`, `{"accent":4}`} {
		if _, err := patternAccentContext(patterns.ExpandContext{}, &PatternInput{Name: "stat-hero", Overrides: json.RawMessage(raw)}); err == nil {
			t.Fatalf("invalid override accepted %s", raw)
		}
	}
}

func TestGenerationRefusesMalformedComposedPattern(t *testing.T) {
	grid := &ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Pattern: json.RawMessage(`{"name":`)}}}}}
	slide := SlideInput{LayoutID: "content", Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{{ShapeGrid: grid}, composeTextLeaf("Keep the review record.")}}}
	_, _, _, err := convertPresentationSlides([]SlideInput{slide}, []types.LayoutMetadata{{ID: "content", Tags: []string{"content"}}}, 12192000, 6858000, nil, nil, "", nil, false)
	if err == nil || !strings.Contains(err.Error(), "invalid pattern") {
		t.Fatalf("generation accepted malformed compose payload: %v", err)
	}
	if string(grid.Rows[0].Cells[0].Pattern) != `{"name":` {
		t.Fatal("generation erased malformed author input")
	}
}
