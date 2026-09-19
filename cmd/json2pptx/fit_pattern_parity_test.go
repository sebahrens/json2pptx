package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// go-slide-creator-adur: the text-capacity and readability detectors walked
// slide.ShapeGrid, which a slide-level pattern only gets at generation time. The
// same content authored as a pattern reported nothing while the expanded grid
// reported four error-severity findings — so agents authoring through the
// recommended surface got "VALID, no issues" on decks whose body text renders at
// 4-8pt.
//
// The two forms must now produce the same finding codes, with the pattern form's
// paths rooted at /slides/N/pattern.
func TestPatternAndShapeGridFormsAgree(t *testing.T) {
	values := scqaOverstuffedValues()
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}

	patternDeck := &PresentationInput{
		Template: "midnight-blue",
		Slides:   []SlideInput{{LayoutID: "slideLayout2", Pattern: &PatternInput{Name: "scqa-summary", Values: raw}}},
	}
	// The same slide with the pattern already expanded, as design_mode free would author it.
	expanded, _ := expandPatternsForFit(patternDeck, 12192000, 6858000, nil)
	gridDeck := &PresentationInput{
		Template: "midnight-blue",
		Slides:   []SlideInput{{LayoutID: "slideLayout2", ShapeGrid: expanded.Slides[0].ShapeGrid}},
	}

	patternCodes := fitCodeSet(t, patternDeck)
	gridCodes := fitCodeSet(t, gridDeck)
	// sparse_layout is the one deliberate difference: it measures authored grid
	// bounds against estimated text height, and a pattern's bounds are the
	// PATTERN's choice, not the author's — on a clean three-card KPI slide it
	// reported "6% filled" and blocked a good deck (go-slide-creator-q7ar).
	delete(patternCodes, patterns.ErrCodeSparseLayout)
	delete(gridCodes, patterns.ErrCodeSparseLayout)
	if len(patternCodes) == 0 {
		t.Fatal("pattern slide produced no findings at all — the detectors are still skipping it")
	}
	for code, n := range gridCodes {
		if patternCodes[code] != n {
			t.Errorf("code %s: pattern form has %d, shape_grid form has %d — the two surfaces must agree", code, patternCodes[code], n)
		}
	}
	for code := range patternCodes {
		if _, ok := gridCodes[code]; !ok {
			t.Errorf("code %s fires only on the pattern form", code)
		}
	}
}

// Findings on an expanded pattern must not name a shape_grid the deck does not
// have, and must not carry a fix that edits one.
func TestPatternFindingPathsAndFixes(t *testing.T) {
	values := scqaOverstuffedValues()
	raw, _ := json.Marshal(values)
	deck := &PresentationInput{
		Template: "midnight-blue",
		Slides:   []SlideInput{{LayoutID: "slideLayout2", Pattern: &PatternInput{Name: "scqa-summary", Values: raw}}},
	}

	findings := collectFitFindings(deck, nil, 12192000, 6858000, nil)
	if len(findings) == 0 {
		t.Fatal("no findings on an overstuffed pattern slide")
	}
	sawPatternPath := false
	for _, f := range findings {
		if strings.HasPrefix(f.Path, slidepath.ShapeGrid(0)) {
			t.Errorf("finding %s points at /slides/0/shape_grid, which this deck does not have: %s", f.Code, f.Path)
		}
		if strings.HasPrefix(f.Path, "/slides/0/pattern/") {
			sawPatternPath = true
		}
		if f.Fix != nil && f.Fix.Kind == "reduce_cell_text" {
			t.Errorf("finding %s suggests reduce_cell_text on a pattern slide, which has no cell to edit", f.Code)
		}
	}
	if !sawPatternPath {
		t.Error("no finding was rooted at /slides/0/pattern/")
	}
}

// The advisory substitution keeps the measured budget so the agent knows how
// much to cut, and names the pattern.
func TestPatternCellFixCarriesTheBudget(t *testing.T) {
	fix := &patterns.FixSuggestion{Kind: "reduce_cell_text", Params: map[string]any{
		"cell_path": "/slides/0/shape_grid/rows/1/cells/0",
		"max_chars": 90,
	}}
	got := patternCellFix(fix, "scqa-summary")
	if got.Kind != "rewrite_field" {
		t.Errorf("kind = %q, want rewrite_field (advisory: the agent shortens its own values)", got.Kind)
	}
	if got.Params["max_chars"] != 90 {
		t.Errorf("the character budget must survive: %+v", got.Params)
	}
	if got.Params["cell_path"] != nil {
		t.Error("a cell_path an agent cannot act on must not be carried over")
	}
	if got.Params["pattern"] != "scqa-summary" {
		t.Errorf("pattern name missing: %+v", got.Params)
	}
	if !patterns.FixKindIsAdvisory(got.Kind) {
		t.Errorf("%q must be a registered advisory kind", got.Kind)
	}
}

// fitCodeSet returns the finding-code histogram for a deck.
func fitCodeSet(t *testing.T, deck *PresentationInput) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, f := range collectFitFindings(deck, nil, 12192000, 6858000, nil) {
		out[f.Code]++
	}
	return out
}

// scqaOverstuffedValues is an scqa-summary payload whose four rows each carry
// four long lines — every value inside the pattern's own maxLength budget, yet
// far more text than the rows can show at a readable size.
func scqaOverstuffedValues() map[string]any {
	line := func(seed string) string {
		return strings.Repeat(seed+" ", 3)
	}
	four := func(seed string) []string {
		return []string{line(seed), line(seed + " again"), line(seed + " still"), line(seed + " finally")}
	}
	return map[string]any{
		"situation":    four("The operating model has drifted across regions and product lines"),
		"complication": four("Funding is committed but the delivery plan has no single owner"),
		"questions":    four("Which operating model gets us to one accountable owner per market"),
		"answer":       four("Consolidate into three regional units with a shared platform team"),
	}
}
