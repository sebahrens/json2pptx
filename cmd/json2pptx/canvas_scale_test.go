package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// exemplarDeck is one titled slide per registered pattern, at its exemplar
// values.
func exemplarDeck(t *testing.T) PresentationInput {
	t.Helper()
	var deck PresentationInput
	for _, p := range patterns.Default().List() {
		ex, ok := p.(patterns.Exemplar)
		if !ok {
			continue
		}
		values, err := json.Marshal(ex.ExemplarValues())
		if err != nil {
			t.Fatalf("marshal exemplar %s: %v", p.Name(), err)
		}
		deck.Slides = append(deck.Slides, titledPatternSlide(p.Name(), values))
	}
	return deck
}

// supportingBandPatterns are not full-slide exhibits: alone on a slide they
// report SLIDE_UNDERUSED by design (docs/PATTERNS.md).
var supportingBandPatterns = map[string]bool{"process-flow-compact": true}

// The patterns' points are designed on the 13.33 x 7.5in slide. On
// business-template's 14.7 x 8.3in slide the same points left eleven
// exemplars underused and scqa-summary lopsided; the grids now follow the
// canvas (shapegrid.Grid.CanvasScale), so every full-slide exhibit uses the
// larger content area as it uses the standard one (go-slide-creator-ttpae).
// midnight-blue holds the standard slide to the same bar.
func TestExemplarDeckUsesTheContentArea(t *testing.T) {
	balance := map[string]bool{
		patterns.ErrCodeSlideUnderused:      true,
		patterns.ErrCodeHorizontalImbalance: true,
		patterns.ErrCodeVerticalImbalance:   true,
	}
	for _, tpl := range []string{"business-template", "midnight-blue"} {
		a := loadTemplateAnalysis(t, tpl)
		deck := exemplarDeck(t)
		for _, f := range collectFitFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
			if balance[f.Code] && !supportingBandPatterns[f.Pattern] {
				t.Errorf("%s: %s exemplar: %s: %s", tpl, f.Pattern, f.Code, f.Message)
			}
		}
	}
}

// A pattern grid, and the sub-grids the pattern nested in its cells, resolve
// at the slide's canvas scale; an authored grid keeps its points.
func TestGridCanvasScale(t *testing.T) {
	a := loadTemplateAnalysis(t, "business-template")
	p, _ := patterns.Default().Get("image-text-split")
	values, err := json.Marshal(p.(patterns.Exemplar).ExemplarValues())
	if err != nil {
		t.Fatal(err)
	}
	ctx := patterns.ExpandContext{Theme: a.Theme, SlideWidth: a.SlideWidth, SlideHeight: a.SlideHeight}
	grid, _, err := expandPattern(&PatternInput{Name: "image-text-split", Values: values}, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	nested := 0
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil && cell.Grid != nil {
				nested++
				if cell.Grid.Source != grid.Source {
					t.Errorf("nested grid source = %q, want the pattern's %q", cell.Grid.Source, grid.Source)
				}
			}
		}
	}
	if nested == 0 {
		t.Fatal("image-text-split no longer nests a grid: pick another pattern for this test")
	}
	if got := gridCanvasScale(grid, a.SlideWidth, a.SlideHeight); got < 1.09 || got > 1.11 {
		t.Errorf("pattern grid on business-template: canvas scale %v, want ~1.10", got)
	}
	if got := gridCanvasScale(grid, 12192000, 6858000); got != 1 {
		t.Errorf("pattern grid on the standard slide: canvas scale %v, want 1", got)
	}
	if got := gridCanvasScale(&ShapeGridInput{}, a.SlideWidth, a.SlideHeight); got != 1 {
		t.Errorf("authored grid: canvas scale %v, want 1", got)
	}
}

// validate_deck_spec gives a sparse-slide advisory the advice render gives as
// its recommended_edit (go-slide-creator-le9d0): it used to report the finding
// with nothing to do.
func TestValidateDeckSpecCarriesSparseSlideAdvice(t *testing.T) {
	spec := map[string]any{
		"meta": map[string]any{"title": "Deck", "template": "midnight-blue"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Pilot review", "subtitle": "Steering committee"},
			map[string]any{"kind": "stat", "title": "The pilot pays back within the first week", "value": "7", "label": "days", "source": "Illustrative"},
		},
	}
	res, err := testValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec}))
	if err != nil || res.IsError {
		t.Fatalf("validate_deck_spec failed: %v %v", err, res)
	}
	var env diagnostics.FindingEnvelope
	structuredInto(t, res.StructuredContent, &env)
	for _, f := range env.Findings {
		if !strings.HasSuffix(f.Code, patterns.ErrCodeSlideUnderused) {
			continue
		}
		if f.Remediation == nil || f.Remediation.Primary == nil || f.Remediation.Primary.Action != semantic.EditAddDetailOrMerge {
			t.Fatalf("SLIDE_UNDERUSED remediation = %+v, want the %s advice", f.Remediation, semantic.EditAddDetailOrMerge)
		}
		if hint, _ := f.Remediation.Primary.Params["hint"].(string); !strings.HasPrefix(hint, "Add useful detail") {
			t.Errorf("hint = %q, want render's recommended_edit hint", hint)
		}
		return
	}
	t.Fatalf("a one-digit stat slide should report SLIDE_UNDERUSED; findings: %+v", env.Findings)
}
