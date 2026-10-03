package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
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
	// An ambiguous shape is not attributed to a field; the finding still names
	// the slide to act on, never nothing (go-slide-creator-y81vn).
	ir.Slides[7].Body["x_high"] = "Fast, under-served"
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "slides[7]" {
		t.Errorf("duplicate axis-end text should not guess a semantic field, got %q", got)
	}
	ir.Slides[7].Body["x_high"] = "Slow, crowded"
	ir.Slides[7].Body["top_left"] = map[string]any{"header": "Fast, under-served"}
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "slides[7]" {
		t.Errorf("quadrant text collision should not guess an axis-end field, got %q", got)
	}
	delete(ir.Slides[7].Body, "top_left")
	ir.Slides[7].Title = "Fast, under-served"
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "slides[7]" {
		t.Errorf("title text collision should not guess an axis-end field, got %q", got)
	}
	ir.Slides[7].Title = ""
	ir.Slides[7].Body["y_axis"] = "Fast, under-served"
	if got := semanticDiagFromFitWithIR(nil, ir, f).SemanticPath; got != "slides[7]" {
		t.Errorf("axis-title text collision should not guess an axis-end field, got %q", got)
	}
}

// MISSING_TITLE addresses the whole raw slide, which has no source link, so it
// came back with a raw_path and no semantic_path (go-slide-creator-y81vn). A
// render finding always names where in the DeckSpec to act: here the title
// field to write, and for any other slide-level finding the slide itself.
func TestSlideLevelFindingCarriesSemanticPath(t *testing.T) {
	sm := semantic.NewSourceMap()
	sm.SetSlidePath(3, "structure.sections[0].slides[1]")
	missing := patterns.FitFinding{ValidationError: patterns.ValidationError{
		Code: patterns.ErrCodeMissingTitle, Path: "/slides/2", Message: "slide 3 has no title",
	}, Action: "review"}

	d := semanticDiagFromFitWithIR(sm, nil, missing)
	if d.SemanticPath != "slides[2].title" || d.RawPath != "/slides/2" {
		t.Errorf("MISSING_TITLE locator = %q (raw %q), want slides[2].title", d.SemanticPath, d.RawPath)
	}
	// Structure-mode decks: the slide's own DeckSpec locator, not slides[N].
	missing.Path = "/slides/3"
	if got := semanticDiagFromFitWithIR(sm, nil, missing).SemanticPath; got != "structure.sections[0].slides[1].title" {
		t.Errorf("structure-mode MISSING_TITLE locator = %q", got)
	}
	other := patterns.FitFinding{ValidationError: patterns.ValidationError{
		Code: patterns.ErrCodeSlideNearlyEmpty, Path: "/slides/2",
	}, Action: "review"}
	if got := semanticDiagFromFitWithIR(sm, nil, other).SemanticPath; got != "slides[2]" {
		t.Errorf("slide-level finding locator = %q, want slides[2]", got)
	}
	// A deck-level finding has no slide to name.
	deck := patterns.FitFinding{ValidationError: patterns.ValidationError{Code: "ACCENT_ROTATION", Path: "/accent_strategy"}}
	if got := semanticDiagFromFitWithIR(sm, nil, deck).SemanticPath; got != "" {
		t.Errorf("deck-level finding locator = %q, want none", got)
	}
}

// An agenda's title is optional. Left out, the slide renders the default
// title and the fit collectors raise no MISSING_TITLE; before, the render
// reported "slide 3 has no title" for a field the kind documents as optional
// (go-slide-creator-y81vn).
func TestUntitledAgendaRendersDefaultTitleWithoutFinding(t *testing.T) {
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "State of AI", Template: "midnight-blue"},
		Slides: []semantic.SlideSpec{
			{Kind: semantic.KindTitle, Body: map[string]any{"title": "State of AI", "subtitle": "2026"}},
			{Kind: semantic.KindAgenda, Body: map[string]any{"sections": []any{"Where we are", "What we found", "What we recommend"}}},
		},
	}
	input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatal(err)
	}
	applyDefaults(input)
	if _, title := extractTitleText(input.Slides[1]); title != "Agenda" {
		t.Errorf("untitled agenda title = %q, want the default \"Agenda\"", title)
	}
	for _, f := range collectFitFindings(input, reserveTestLayouts(), shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU, nil) {
		if f.Code == patterns.ErrCodeMissingTitle {
			t.Errorf("an untitled agenda drew %s at %s: %s", f.Code, f.Path, f.Message)
		}
	}
}
