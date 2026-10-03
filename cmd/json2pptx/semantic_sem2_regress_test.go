package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

func substantiveReadabilityFinding(path string) patterns.FitFinding {
	return patterns.FitFinding{ValidationError: patterns.ValidationError{
		Code: patterns.ErrCodeTextBelowReadableMin, Path: path,
		Fix: &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{
			"measurement_source": "generated", "rendered_shape_text": "FY26, USD millions",
		}},
	}, Action: "review"}
}

// slide_scores disagreed with the overall score on a one-slide deck
// (go-slide-creator-csclk.54).
func TestSemanticSlideScoresTakeStructuralDeductions(t *testing.T) {
	text := "Plenty of body text so the input heuristic scores this slide cleanly at one hundred."
	input := &PresentationInput{Slides: []SlideInput{{SlideType: "content", Content: []ContentInput{{PlaceholderID: "body", Type: "text", TextValue: &text}}}}}
	q := semanticQualityScorePtr(input, []patterns.FitFinding{substantiveReadabilityFinding("/slides/0/rendered_shapes/200")}, nil, nil)
	if len(q.SlideScores) != 1 || q.SlideScores[0].Score != q.Score {
		t.Fatalf("slide score %+v disagrees with overall %v", q.SlideScores, q.Score)
	}
}

// A gate-blocking review finding read as "info" with no semantic_path
// (go-slide-creator-csclk.53). What blocks the gate is an error
// (go-slide-creator-x9rhq).
func TestGateBlockingFindingIsErrorWithSemanticPath(t *testing.T) {
	ir := &semantic.DeckIR{Slides: []semantic.SlideIR{{
		Kind: semantic.KindBridge, SourcePath: "slides[0]",
		Body: map[string]any{"caption": "FY26, USD millions", "unit": "$m"},
	}}}
	d := semanticDiagFromFitWithIR(nil, ir, substantiveReadabilityFinding("/slides/0/rendered_shapes/200"))
	if d.Severity != "error" || !d.Blocking || d.SemanticPath != "slides[0].caption" {
		t.Fatalf("got severity %q path %q", d.Severity, d.SemanticPath)
	}
}

// explanation_summary kept the planned visual after a degrade
// (go-slide-creator-csclk.55).
func TestExplanationReportsCompiledLayout(t *testing.T) {
	exp := &semantic.DeckExplanation{Slides: []semantic.SlideExplanation{
		{Layout: "blank-title", VisualFamily: semantic.FamilyKPI, Pattern: ""},
		{Layout: "blank-title", VisualFamily: semantic.FamilyKPI, Pattern: "kpi-3up"},
	}}
	input := &PresentationInput{Slides: []SlideInput{
		{SlideType: "content"},
		{SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "kpi-3up"}},
	}}
	reconcileExplanationWithCompiled(exp, input)
	if exp.Slides[0].Layout != "content" || exp.Slides[0].VisualFamily != semantic.FamilyText {
		t.Errorf("degraded slide still reports %+v", exp.Slides[0])
	}
	if exp.Slides[1].Layout != "blank-title" || exp.Slides[1].VisualFamily != semantic.FamilyKPI || exp.Slides[1].Pattern != "kpi-3up" {
		t.Errorf("visual slide changed: %+v", exp.Slides[1])
	}
}
