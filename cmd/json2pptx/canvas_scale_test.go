package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
// report SLIDE_UNDERUSED by design (docs/PATTERNS.md). kpi-inline joined them
// with go-slide-creator-i7yju: its bar covers its coverage threshold but
// leaves the lower 41% of the content area empty, which the lower-third rule
// for KPI rows now reports.
var supportingBandPatterns = map[string]bool{"process-flow-compact": true, "kpi-inline": true}

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

// underusedSweepTemplates are the templates the exemplar sweep measures:
// -short takes the two whose exemplars sat nearest their thresholds
// (business-template's larger slide, modern-template's light Poppins).
var underusedSweepTemplates = []string{"business-template", "modern-template", "midnight-blue", "warm-coral", "forest-green", "modern", "modern-yellow", "blue-corporate", "abstract", "p-style"}

// exemplarUnderusedMargin is how far over its SLIDE_UNDERUSED threshold every
// full-slide exemplar must sit, as a share of the content area.
const exemplarUnderusedMargin = 0.015

// Every full-slide exemplar clears its SLIDE_UNDERUSED threshold by
// exemplarUnderusedMargin on every template. Coverage is measured from glyph
// widths, so it moves by a point or so with the template's face and with any
// change to a metric: exemplars that sat 0.3–1.4 points over their threshold
// (timeline-horizontal, image-text-split, driver-tree, value-chain) were one
// rounding from being reported, and value-chain was 0.1 under on a local
// template (go-slide-creator-am8kr). A pattern that falls inside the margin
// here needs sizing, or its ink counted as the eye sees it — not a lower
// threshold.
func TestExemplarsClearTheUnderusedThreshold(t *testing.T) {
	templates := underusedSweepTemplates
	if testing.Short() {
		templates = templates[:2]
	}
	for _, tpl := range templates {
		if _, err := os.Stat(filepath.Join("..", "..", "templates", tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		a := loadTemplateAnalysis(t, tpl)
		deck := exemplarDeck(t)
		measured := 0
		thinnest := slideUsage{frac: 1}
		collectGeometry(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme, func(u slideUsage) {
			if supportingBandPatterns[u.pattern] {
				return
			}
			measured++
			if u.frac-u.threshold < thinnest.frac-thinnest.threshold {
				thinnest = u
			}
			if u.frac < u.threshold+exemplarUnderusedMargin {
				t.Errorf("%s: %s exemplar covers %.1f%% of the content area against a %.0f%% threshold: under the %.1f-point margin",
					tpl, u.pattern, 100*u.frac, 100*u.threshold, 100*exemplarUnderusedMargin)
			}
		})
		if measured < 40 {
			t.Errorf("%s: only %d exemplars were measured", tpl, measured)
		}
		// Coverage is not the whole rule for a KPI row: a row that clears
		// its 20% threshold may still be a strip over an empty lower third
		// (slideLowerBandMaxFrac). The kpi-Nup exemplars clear that too —
		// they are grown into the free height (go-slide-creator-i7yju).
		rows := 0
		for _, f := range collectGeometryFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
			if f.Code == patterns.ErrCodeSlideUnderused && isKPINupPattern(f.Pattern) {
				t.Errorf("%s: %s exemplar: %s", tpl, f.Pattern, f.Message)
			}
		}
		for _, s := range deck.Slides {
			if s.Pattern != nil && isKPINupPattern(s.Pattern.Name) {
				rows++
			}
		}
		if rows != 5 {
			t.Errorf("%s: %d kpi-Nup exemplars were checked against the lower-third rule, want 5", tpl, rows)
		}
		t.Logf("%s: thinnest exemplar %s at %.1f%% against %.0f%%", tpl, thinnest.pattern, 100*thinnest.frac, 100*thinnest.threshold)
	}
}

// Counting a pattern's open columns by slot must not hide a sparse slide: the
// bare forms of the patterns counted that way since go-slide-creator-am8kr
// still report.
func TestBareOpenColumnPatternsStillReportUnderused(t *testing.T) {
	a := loadTemplateAnalysis(t, "midnight-blue")
	for name, values := range bareOpenColumnFixtures {
		deck := PresentationInput{Slides: []SlideInput{titledPatternSlide(name, json.RawMessage(values))}}
		reported := false
		for _, f := range collectGeometryFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
			reported = reported || f.Code == patterns.ErrCodeSlideUnderused
		}
		if !reported {
			t.Errorf("a bare %s must still report SLIDE_UNDERUSED", name)
		}
	}
}

var bareOpenColumnFixtures = map[string]string{
	"value-chain":         `{"steps":[{"label":"Extraction"},{"label":"Processing"},{"label":"Manufacturing"},{"label":"Distribution"},{"label":"Retail"}]}`,
	"timeline-horizontal": `[{"label":"Discovery"},{"label":"Pilot"},{"label":"Scale-up"},{"label":"Run"}]`,
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

	// A regions slide is a grid the semantic compiler builds by hand: it and
	// the grids it nests (a region under its heading) carry the compiler's
	// stamp and follow the canvas too (go-slide-creator-o9n8u).
	spec := map[string]any{
		"meta": map[string]any{"title": "Deck", "template": "business-template"},
		"slides": []any{map[string]any{
			"kind": "regions", "title": "Two regions", "arrangement": "columns",
			"regions": []any{
				map[string]any{"kind": "text", "heading": "Left", "body": "One paragraph."},
				map[string]any{"kind": "text", "heading": "Right", "body": "Another paragraph."},
			},
		}},
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	parsed, diags := semantic.ParseJSON(raw)
	if parsed == nil {
		t.Fatalf("parse regions spec: %+v", diags)
	}
	compiled, _, err := semantic.Compile(parsed, semantic.CompileOptions{})
	if err != nil || compiled == nil {
		t.Fatalf("compile regions spec: %v", err)
	}
	var check func(g *ShapeGridInput, depth int) int
	check = func(g *ShapeGridInput, depth int) int {
		n := 1
		if got := gridCanvasScale(g, a.SlideWidth, a.SlideHeight); got < 1.09 || got > 1.11 {
			t.Errorf("regions grid at depth %d (source %q): canvas scale %v, want ~1.10", depth, g.Source, got)
		}
		for _, row := range g.Rows {
			for _, cell := range row.Cells {
				if cell != nil && cell.Grid != nil {
					n += check(cell.Grid, depth+1)
				}
			}
		}
		return n
	}
	for _, slide := range compiled.Slides {
		if slide.ShapeGrid != nil {
			if n := check(slide.ShapeGrid, 0); n < 3 {
				t.Errorf("regions slide compiled to %d grid(s), want the outer grid and one per headed region", n)
			}
			return
		}
	}
	t.Fatal("the regions slide compiled to no shape_grid")
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
