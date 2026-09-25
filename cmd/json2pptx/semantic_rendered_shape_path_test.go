package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

func TestRenderedMatrixAxisEndFindingMapsToAuthoredField(t *testing.T) {
	ir := &semantic.DeckIR{Slides: make([]semantic.SlideIR, 8)}
	ir.Slides[7] = semantic.SlideIR{
		Kind:       semantic.KindMatrix2x2,
		SourcePath: "slides[7]",
		Visual:     semantic.VisualPlan{Pattern: "matrix-2x2"},
		Body: map[string]any{
			"x_high": "Slow, crowded", "y_high": "Fast, under-served",
		},
	}
	f := patterns.FitFinding{ValidationError: patterns.ValidationError{
		Code: patterns.ErrCodeTextBelowReadableMin, Path: "/slides/7/rendered_shapes/19",
		Fix: &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{
			"rendered_shape_text": "Fast, under-served",
		}},
	}, Action: "review"}
	d := semanticDiagFromFitWithIR(nil, ir, f)
	if d.RawPath != f.Path || d.SemanticPath != "slides[7].y_high" || d.RecommendedEdit == nil || d.RecommendedEdit.Kind != semantic.EditShortenText {
		t.Fatalf("rendered matrix end locator = %+v", d)
	}
	ir.Slides[7].Body["x_high"] = "Fast, under-served"
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "" {
		t.Errorf("duplicate axis-end text should not guess a semantic field, got %q", got)
	}
	ir.Slides[7].Body["x_high"] = "Slow, crowded"
	ir.Slides[7].Body["top_left"] = map[string]any{"header": "Fast, under-served"}
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "" {
		t.Errorf("quadrant text collision should not guess an axis-end field, got %q", got)
	}
	delete(ir.Slides[7].Body, "top_left")
	ir.Slides[7].Title = "Fast, under-served"
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "" {
		t.Errorf("title text collision should not guess an axis-end field, got %q", got)
	}
	ir.Slides[7].Title = ""
	ir.Slides[7].Body["y_axis"] = "Fast, under-served"
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "" {
		t.Errorf("axis-title text collision should not guess an axis-end field, got %q", got)
	}
}
