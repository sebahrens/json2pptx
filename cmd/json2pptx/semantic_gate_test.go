package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-05wn: the recommended new-deck path was the blindest tool in
// the server. render_deck_spec reported only what generation happened to emit —
// one executive_summary slide with a 100-word body came back with
// diagnostics=null and quality_summary 100, while validate_input on the SAME
// compiled deck reported FIT.BODY_TOO_LONG. There was also no gate on the
// semantic surface: quality_summary returned 100 for 15 of 16 calibration decks,
// including one whose own diagnostics carried five SEMANTIC_WEAK_CONTENT
// findings.
func TestRenderDeckSpecReportsWhatValidateInputWould(t *testing.T) {
	spec := longBodySpec()
	input, result, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v (%+v)", err, result.Diagnostics)
	}
	applyDefaults(input)

	// What validate_input / generate_presentation see on the compiled deck.
	rawCodes := map[string]bool{}
	for _, f := range collectFitFindings(input, nil, 12192000, 6858000, nil) {
		rawCodes[f.Code] = true
	}
	if !rawCodes[patterns.ErrCodeBodyTooLong] {
		t.Fatalf("fixture no longer produces BODY_TOO_LONG on the compiled deck: %v", rawCodes)
	}

	// What the semantic render path reports for the same deck: the collectors
	// run there too, so its codes are a superset.
	fit := collectFitFindings(input, nil, 12192000, 6858000, nil)
	q := semanticQualityScorePtr(input, fit, nil, nil)
	if q.QualityGate == nil {
		t.Fatal("render_deck_spec quality_summary carries no quality_gate")
	}
	if q.StructuralScore == 0 {
		t.Error("structural_score missing from the semantic quality summary")
	}
	if q.Score > float64(q.StructuralScore) {
		t.Errorf("headline score %.0f exceeds the structural verdict %d", q.Score, q.StructuralScore)
	}
}

// A lorem DeckSpec must not come back at 100 with a passing gate.
func TestSemanticQualityGateFailsPlaceholderDeck(t *testing.T) {
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "Lorem", Template: "midnight-blue"},
		Slides: []semantic.SlideSpec{
			{Kind: semantic.KindTitle, Body: map[string]any{"title": "Presentation Title", "subtitle": "Subtitle goes here"}},
			{Kind: semantic.KindExecutiveSummary, Body: map[string]any{
				"title":    "Lorem ipsum dolor sit amet",
				"points":   []any{"Lorem ipsum dolor sit amet, consectetur", "Sed do eiusmod tempor incididunt", "TODO: add content"},
				"takeaway": "Lorem ipsum.",
			}},
		},
	}
	input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	applyDefaults(input)
	fit := collectFitFindings(input, nil, 12192000, 6858000, nil)
	q := semanticQualityScorePtr(input, fit, nil, nil)
	if q.QualityGate == nil || q.QualityGate.Passed {
		t.Errorf("a lorem deck passed the semantic gate: %+v", q.QualityGate)
	}
	if q.Score >= 100 {
		t.Errorf("headline score = %.0f on a lorem deck", q.Score)
	}
}

// The compiler's source map must reach findings addressed by placeholder, or
// every collected finding comes back without a semantic_path to edit.
func TestCompiledFindingsCarrySemanticPaths(t *testing.T) {
	spec := longBodySpec()
	input, result, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatal(err)
	}
	applyDefaults(input)
	for _, f := range collectFitFindings(input, nil, 12192000, 6858000, nil) {
		if f.Code != patterns.ErrCodeBodyTooLong {
			continue
		}
		semPath, _, mapped := result.SourceMap.ResolveSemantic(f.Path)
		if !mapped {
			t.Fatalf("BODY_TOO_LONG at %q has no semantic mapping", f.Path)
		}
		if !strings.HasSuffix(semPath, ".points") {
			t.Errorf("semantic_path = %q, want the points field the author wrote", semPath)
		}
		return
	}
	t.Fatal("no BODY_TOO_LONG finding on the fixture")
}

func longBodySpec() *semantic.DeckSpec {
	long := strings.TrimSpace(strings.Repeat("operational ", 20))
	points := make([]any, 0, 5)
	for i := 0; i < 5; i++ {
		points = append(points, long)
	}
	return &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "Repro", Template: "midnight-blue"},
		Slides: []semantic.SlideSpec{{Kind: semantic.KindExecutiveSummary, Body: map[string]any{
			"title":    "Where we stand",
			"points":   points,
			"takeaway": "On track.",
		}}},
	}
}
