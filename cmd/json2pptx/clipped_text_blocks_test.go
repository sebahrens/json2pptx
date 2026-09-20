package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// chevronDeck builds a numbered-step-strip whose labels are too long for the
// step count — the case the round-2 render showed breaking as
// "Internationalisatio / n programme".
func chevronDeck(labels ...string) *PresentationInput {
	steps := make([]any, len(labels))
	for i, l := range labels {
		steps[i] = map[string]any{"label": l}
	}
	vals, _ := json.Marshal(map[string]any{"style": "chevron", "steps": steps})
	return &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			SlideType: "content",
			Content:   []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Steps")}},
			Pattern:   &PatternInput{Name: "numbered-step-strip", Values: vals},
		}},
	}
}

// gateFor scores a deck the way score_deck does.
func gateFor(t *testing.T, in *PresentationInput) (*deterministic.DeckScore, *deterministic.QualityGate, []patterns.FitFinding) {
	t.Helper()
	layouts, theme, w, h := fitReportGeometry(in.Template, filepath.Join("..", "..", "templates"))
	findings := collectFitFindings(in, layouts, w, h, theme)
	ds := deterministic.ScoreFromFindings(findings, len(in.Slides))
	return ds, deterministic.EvaluateQualityGate(ds, findings, deterministic.DefaultQualityGateCriteria()), findings
}

// TestMeasuredMidWordBreakFailsTheGate is go-slide-creator-rxkt: the deck
// rendered chevron labels broken mid-word while score_deck said ship it.
func TestMeasuredMidWordBreakFailsTheGate(t *testing.T) {
	long := chevronDeck(
		"Organisational restructuring", "Internationalisation programme",
		"Decommissioning infrastructure", "Institutionalisation",
		"Operationalisation readiness", "Commercialisation")
	_, gate, findings := gateFor(t, long)

	var blocking *patterns.FitFinding
	for i := range findings {
		if findings[i].Code == patterns.ErrCodeTextExceedsShape && findings[i].Action == "shrink_or_split" {
			blocking = &findings[i]
		}
	}
	if blocking == nil {
		t.Fatal("a chevron strip whose labels cannot fit at the readable floor must raise a blocking TEXT_EXCEEDS_SHAPE")
	}
	if !strings.Contains(blocking.Message, "mid-word") {
		t.Errorf("message %q should say what the reader will see", blocking.Message)
	}
	if gate.Passed {
		t.Errorf("the gate passed a deck with a measured mid-word break: %+v", gate.Reasons)
	}
	if len(gate.Reasons) == 0 || !strings.Contains(strings.Join(gate.Reasons, " "), "P1") {
		t.Errorf("gate reasons %v should name the P1 finding", gate.Reasons)
	}
}

// TestShortLabelsPassTheGateAgain: the escalation must be about the defect, not
// about the pattern.
func TestShortLabelsPassTheGateAgain(t *testing.T) {
	short := chevronDeck("Restructure", "Go global", "Decommission", "Embed", "Operate", "Commercial")
	_, gate, findings := gateFor(t, short)
	for _, f := range findings {
		if f.Code == patterns.ErrCodeTextExceedsShape && f.Action == "shrink_or_split" {
			t.Fatalf("short labels raised a blocking finding: %s", f.Message)
		}
	}
	if !gate.Passed {
		t.Errorf("a strip whose labels fit should pass: %v", gate.Reasons)
	}
}

// TestPredictedTextExceedsShapeStaysAdvisory keeps the distinction that makes
// the escalation safe: the geometry detector ESTIMATES the box from authored
// sizes, and a 1.15 estimate can still render on one line (examples/
// process-grid-2row.json does). Only a pattern that MEASURED the failure blocks.
func TestPredictedTextExceedsShapeStaysAdvisory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "process-grid-2row.json"))
	if err != nil {
		t.Skipf("example unavailable: %v", err)
	}
	var in PresentationInput
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	_, gate, findings := gateFor(t, &in)
	sawPredicted := false
	for _, f := range findings {
		if f.Code != patterns.ErrCodeTextExceedsShape {
			continue
		}
		sawPredicted = true
		if f.Action == "shrink_or_split" {
			t.Errorf("a predicted overflow blocked the gate: %s", f.Message)
		}
	}
	if !sawPredicted {
		t.Skip("this example no longer trips the predicted check")
	}
	if !gate.Passed {
		t.Errorf("a bundled example that renders correctly must not fail the gate: %v", gate.Reasons)
	}
}

// TestPatternWarningActions pins which post-expand warnings block.
func TestPatternWarningActions(t *testing.T) {
	if got := patternWarningAction(patterns.ErrCodeTextExceedsShape); got != "shrink_or_split" {
		t.Errorf("TEXT_EXCEEDS_SHAPE from a pattern = %q, want shrink_or_split", got)
	}
	for _, code := range []string{patterns.ErrCodeChartPlaceholderEmpty, patterns.ErrCodeBodyTooLong, "ANYTHING_ELSE"} {
		if got := patternWarningAction(code); got != "review" {
			t.Errorf("%s = %q, want review", code, got)
		}
	}
}
