package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-kspkr: the rotate fallback is a template property, so a
// deck reports it once, at the deck field, however many slides rotate.
func TestCollapseRotatedAccentFindingsToOneDeckLevelInfo(t *testing.T) {
	reason := "rotation cycles through this template's safe accents accent1, accent3; excluded: accent2 (unreadable)"
	var findings []patterns.FitFinding
	for _, si := range []int{0, 2, 5} {
		f := patternWarningAsFinding(si, "card-grid", patterns.ErrCodeRotatedAccentUnreadable+": "+reason)
		if f == nil {
			t.Fatal("warning did not convert to a finding")
		}
		findings = append(findings, *f)
	}
	other := patterns.FitFinding{ValidationError: patterns.ValidationError{Code: "title_wraps", Path: "/slides/1/title"}, Action: "info"}
	findings = append(findings, other)

	got := BudgetFitFindings(findings, DefaultFindingBudget, false)
	var rotated []patterns.FitFinding
	for _, f := range got {
		if f.Code == patterns.ErrCodeRotatedAccentUnreadable {
			rotated = append(rotated, f)
		}
	}
	if len(rotated) != 1 {
		t.Fatalf("want one deck-level rotate finding, got %d: %+v", len(rotated), rotated)
	}
	r := rotated[0]
	if r.Path != rotatedAccentDeckPath {
		t.Errorf("path = %q, want %q", r.Path, rotatedAccentDeckPath)
	}
	if !strings.Contains(r.Message, "slides 1, 3, 6") || !strings.Contains(r.Message, reason) {
		t.Errorf("message should list the rotating slides and the reason once: %q", r.Message)
	}
	if strings.Count(r.Message, "safe accents") != 1 {
		t.Errorf("reason repeated: %q", r.Message)
	}
	var sawOther bool
	for _, f := range got {
		sawOther = sawOther || f.Code == "title_wraps"
	}
	if !sawOther {
		t.Error("collapsing dropped an unrelated finding")
	}
}
