package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// mismatchDeck wraps one pattern slide in a presentation.
func mismatchDeck(t *testing.T, pattern, values string) *PresentationInput {
	t.Helper()
	return &PresentationInput{
		Slides: []SlideInput{{
			SlideType: "content",
			Pattern:   &PatternInput{Name: pattern, Values: json.RawMessage(values)},
		}},
	}
}

// mismatchCodes returns the mismatch findings a deck produces.
func mismatchCodes(t *testing.T, in *PresentationInput) []patterns.FitFinding {
	t.Helper()
	return collectPatternMismatchFindings(in)
}

// TestPatternMismatchFires pins the three shapes the check exists to catch
// (go-slide-creator-h339i).
func TestPatternMismatchFires(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		values  string
		wantTo  string
	}{
		{
			name:    "metrics drawn as a timeline",
			pattern: "timeline-horizontal",
			values:  `[{"label":"$48M","date":"Revenue"},{"label":"118%","date":"NRR"},{"label":"41d","date":"Sales cycle"}]`,
			wantTo:  "kpi-3up",
		},
		{
			name:    "a plan drawn as a 2x2",
			pattern: "matrix-2x2",
			values:  `{"top_left":{"header":"January"},"top_right":{"header":"April"},"bottom_left":{"header":"August"},"bottom_right":{"header":"December"}}`,
			wantTo:  "phase-roadmap",
		},
		{
			name:    "figures drawn as a process",
			pattern: "process-flow",
			values:  `{"steps":[{"label":"EMEA $12M"},{"label":"APAC $9M"},{"label":"Americas $27M"}]}`,
			wantTo:  "kpi-3up",
		},
		{
			name:    "figures drawn as a pyramid",
			pattern: "pyramid",
			values:  `{"tiers":["EMEA $12M","APAC $9M","Americas $27M"]}`,
			wantTo:  "kpi-3up",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mismatchCodes(t, mismatchDeck(t, tc.pattern, tc.values))
			if len(got) != 1 {
				t.Fatalf("expected one mismatch finding, got %d: %+v", len(got), got)
			}
			f := got[0]
			if f.Code != patterns.ErrCodePatternContentMismatch {
				t.Errorf("code = %q, want %q", f.Code, patterns.ErrCodePatternContentMismatch)
			}
			if f.Action != "review" {
				t.Errorf("action = %q, want review — a mismatch is a judgement call, not a refusal", f.Action)
			}
			if f.Fix == nil || f.Fix.Kind != "swap_pattern" {
				t.Fatalf("fix = %+v, want a swap_pattern", f.Fix)
			}
			if f.Fix.Params["from"] != tc.pattern {
				t.Errorf("fix.from = %v, want %q", f.Fix.Params["from"], tc.pattern)
			}
			if f.Fix.Params["to"] != tc.wantTo {
				t.Errorf("fix.to = %v, want %q", f.Fix.Params["to"], tc.wantTo)
			}
			if f.Path != "/slides/0/pattern" {
				t.Errorf("path = %q, want /slides/0/pattern", f.Path)
			}
		})
	}
}

// TestPatternMismatchStaysQuiet is the more important half: a conforming deck
// must gain nothing. The previous attempt at this signal fired on every
// conforming timeline and 2x2 alike (go-slide-creator-wrsb).
func TestPatternMismatchStaysQuiet(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		values  string
	}{
		{"a real timeline", "timeline-horizontal", `[{"label":"Kick-off","date":"Q1 2026"},{"label":"Pilot","date":"Q2 2026"},{"label":"Rollout","date":"Q3 2026"}]`},
		{"a timeline of metrics against real periods", "timeline-horizontal", `[{"label":"$12M","date":"Q1"},{"label":"$18M","date":"Q2"},{"label":"$27M","date":"Q3"}]`},
		{"a timeline with one named stop", "timeline-horizontal", `[{"label":"$48M","date":"Revenue"},{"label":"118%","date":"NRR"},{"label":"Go-live","date":"Sales cycle"}]`},
		{"two stops is too few to judge", "timeline-horizontal", `[{"label":"$48M","date":"Revenue"},{"label":"118%","date":"NRR"}]`},
		{"a real 2x2", "matrix-2x2", `{"top_left":{"header":"High impact, low effort"},"top_right":{"header":"High impact, high effort"},"bottom_left":{"header":"Low impact, low effort"},"bottom_right":{"header":"Low impact, high effort"}}`},
		{"a 2x2 with three dates and one dimension", "matrix-2x2", `{"top_left":{"header":"January"},"top_right":{"header":"April"},"bottom_left":{"header":"August"},"bottom_right":{"header":"Quick wins"}}`},
		{"a real process flow", "process-flow", `{"steps":[{"label":"Collect the deposits"},{"label":"Reconcile the ledger"},{"label":"Settle with the clearer"}]}`},
		{"a flow whose steps mention a figure in prose", "process-flow", `{"steps":[{"label":"Collect the first $1M in deposits"},{"label":"Reconcile the ledger daily"},{"label":"Settle T+1 with the clearer"}]}`},
		{"a real pyramid", "pyramid", `{"tiers":["Vision","Strategy","Execution"]}`},
		{"a pattern with no rule", "card-grid", `{"cards":[{"title":"$12M"},{"title":"$9M"},{"title":"$27M"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mismatchCodes(t, mismatchDeck(t, tc.pattern, tc.values)); len(got) != 0 {
				t.Errorf("conforming content produced %d finding(s): %+v", len(got), got)
			}
		})
	}
}

// TestPatternMismatchQuietOnBundledDecks sweeps every bundled example and every
// calibration deck: only the deck built to carry this defect may report it.
func TestPatternMismatchQuietOnBundledDecks(t *testing.T) {
	dirs := []string{filepath.Join("..", "..", "examples"), filepath.Join("testdata", "calibration")}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var input PresentationInput
			if json.Unmarshal(data, &input) != nil || len(input.Slides) == 0 {
				continue
			}
			findings := collectPatternMismatchFindings(&input)
			if len(findings) == 0 {
				continue
			}
			if strings.HasPrefix(e.Name(), "B07_wrong_pattern") {
				continue // the deck built to carry exactly this defect
			}
			t.Errorf("%s/%s gained %d false mismatch finding(s): %+v", dir, e.Name(), len(findings), findings)
		}
	}
}

// TestPeriodAndMeasureLabels pins the two classifiers the rules rest on. A
// bare number is deliberately neither: a timeline stop reads "2026".
func TestPeriodAndMeasureLabels(t *testing.T) {
	periods := []string{"January", "Apr", "Q3", "H1", "2026", "FY26", "April 2026", "Q3 FY26", "Week 4", "Phase 2"}
	for _, p := range periods {
		if !isPeriodLabel(p) {
			t.Errorf("isPeriodLabel(%q) = false, want true", p)
		}
		if isQuantityLabel(p) {
			t.Errorf("isQuantityLabel(%q) = true; a period is not a measure", p)
		}
	}
	measures := []string{"$48M", "118%", "41d", "3.4x", "€1,200", "+23%", "14 months", "-6 pts"}
	for _, m := range measures {
		if !isQuantityLabel(m) {
			t.Errorf("isQuantityLabel(%q) = false, want true", m)
		}
		if isPeriodLabel(m) {
			t.Errorf("isPeriodLabel(%q) = true; a measure is not a period", m)
		}
	}
	for _, neither := range []string{"Revenue", "NRR", "Sales cycle", "Kick-off", ""} {
		if isPeriodLabel(neither) && neither != "" {
			t.Errorf("isPeriodLabel(%q) = true", neither)
		}
		if isQuantityLabel(neither) {
			t.Errorf("isQuantityLabel(%q) = true", neither)
		}
	}
	// A bare integer is a count, not a measure — flagging it would fire on
	// conforming timelines.
	if isQuantityLabel("3") || isQuantityLabel("2026") {
		t.Error("a bare number must not read as a measure")
	}
}
