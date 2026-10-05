package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// lowerBandFindings returns, per slide index, the lower-third SLIDE_UNDERUSED
// finding of deck on tpl (the one carrying fix.params.empty_band_pct).
func lowerBandFindings(t *testing.T, tpl string, deck PresentationInput) map[string]patterns.FitFinding {
	t.Helper()
	a := loadTemplateAnalysis(t, tpl)
	out := map[string]patterns.FitFinding{}
	for _, f := range collectFitFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
		if f.Code != patterns.ErrCodeSlideUnderused || f.Fix == nil {
			continue
		}
		if _, ok := f.Fix.Params["empty_band_pct"]; ok {
			out[f.Path] = f
		}
	}
	return out
}

func codesAt(t *testing.T, tpl string, deck PresentationInput, path string) map[string]int {
	t.Helper()
	a := loadTemplateAnalysis(t, tpl)
	out := map[string]int{}
	for _, f := range collectFitFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
		if f.Path == path || strings.HasPrefix(f.Path, path+"/") {
			out[f.Code]++
		}
	}
	return out
}

// lowerBandTemplates are the templates the rule is asserted on: the shortest
// content area, the standard one and the local template when present.
func lowerBandTemplates(t *testing.T) []string {
	t.Helper()
	tpls := []string{"modern-template", "midnight-blue"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		tpls = append(tpls, "p-style")
	}
	return tpls
}

// The owner's rule is general (go-slide-creator-kgfs1): a slide whose lower
// third or more is an empty band must not score 100 silently, whatever the
// pattern. go-slide-creator-i7yju applied it to KPI rows and tables; a thin
// before-after-compact, comparison or card row sat centred, cleared its
// coverage threshold and reported nothing.
func TestAThinBlockOverAnEmptyLowerThirdIsReported(t *testing.T) {
	cases := []struct {
		name, pattern, values string
		// wantHint is a phrase the remedy must carry.
		wantHint string
	}{
		{"before-after-compact of one item a side", "before-after-compact",
			`{"before":{"header":"Current","items":["Manual"]},"after":{"header":"Future","items":["Automated"]}}`,
			"before-after-compact is a supporting band"},
		{"card-grid of one row of short cards", "card-grid",
			`{"columns":3,"cells":[{"header":"Price","body":"One corridor per segment"},{"header":"Cost","body":"300 strategic suppliers"},{"header":"Cash","body":"45 inventory days"}]}`,
			"add what completes the exhibit"},
		{"kpi-inline alone", "kpi-inline",
			`[{"big":"42%","small":"Share"},{"big":"8.1","small":"NPS"},{"big":"3x","small":"Growth"}]`,
			"kpi-inline is a supporting band"},
	}
	for _, tpl := range lowerBandTemplates(t) {
		for _, tc := range cases {
			deck := PresentationInput{Slides: []SlideInput{titledPatternSlide(tc.pattern, json.RawMessage(tc.values))}}
			got := codesAt(t, tpl, deck, "/slides/0")
			if got[patterns.ErrCodeSlideUnderused]+got[patterns.ErrCodeVerticalImbalance] == 0 {
				t.Errorf("%s: %s: no SLIDE_UNDERUSED or VERTICAL_IMBALANCE: the slide scores 100 over an empty lower third (findings %v)", tpl, tc.name, got)
				continue
			}
			if got[patterns.ErrCodeSlideUnderused] > 1 {
				t.Errorf("%s: %s: SLIDE_UNDERUSED reported %d times", tpl, tc.name, got[patterns.ErrCodeSlideUnderused])
			}
			f, ok := lowerBandFindings(t, tpl, deck)["/slides/0"]
			if !ok {
				continue // reported by coverage or as lopsided, which is the same fact
			}
			if f.Action != "review" || f.Fix.Kind != "add_detail_or_resize" {
				t.Errorf("%s: %s: action %q / fix %q, want review / add_detail_or_resize", tpl, tc.name, f.Action, f.Fix.Kind)
			}
			if pct, _ := f.Fix.Params["empty_band_pct"].(float64); pct < 33 {
				t.Errorf("%s: %s: empty_band_pct = %v, want 33 or more", tpl, tc.name, f.Fix.Params["empty_band_pct"])
			}
			if f.Fix.Params["empty_band_side"] != "below" || f.Fix.Params["band_capped_by"] != "pattern" {
				t.Errorf("%s: %s: params %v", tpl, tc.name, f.Fix.Params)
			}
			if hint, _ := f.Fix.Params["hint"].(string); !strings.Contains(hint, tc.wantHint) {
				t.Errorf("%s: %s: hint %q does not name the remedy %q", tpl, tc.name, hint, tc.wantHint)
			}
		}
	}
}

// A flow SPARSE_SINGLE_ROW_FLOW names is not charged a second time for the
// same empty band, and the hero statements keep their white space.
func TestLowerThirdRuleDoesNotDoubleReport(t *testing.T) {
	for _, tpl := range lowerBandTemplates(t) {
		flow := PresentationInput{Slides: []SlideInput{titledPatternSlide("process-flow",
			json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Run"}]}`))}}
		got := codesAt(t, tpl, flow, "/slides/0")
		if got[patterns.ErrCodeSparseSingleRowFlow] != 1 {
			t.Errorf("%s: a three-word process-flow: SPARSE_SINGLE_ROW_FLOW x%d, want 1 (findings %v)", tpl, got[patterns.ErrCodeSparseSingleRowFlow], got)
		}
		if len(lowerBandFindings(t, tpl, flow)) != 0 {
			t.Errorf("%s: a three-word process-flow is reported for its empty band twice", tpl)
		}
		hero := PresentationInput{Slides: []SlideInput{titledPatternSlide("stat-hero",
			json.RawMessage(`{"value":"48%","label":"of revenue from new products"}`))}}
		if fs := lowerBandFindings(t, tpl, hero); len(fs) != 0 {
			t.Errorf("%s: stat-hero is held to the lower-third rule: %v", tpl, fs)
		}
	}
}

// An author's bounds box that ends above the lower third is reported with the
// remedy an author has: the cap.
func TestLowerThirdRuleNamesAnAuthorCap(t *testing.T) {
	for _, tpl := range lowerBandTemplates(t) {
		slide := titledPatternSlide("card-grid", json.RawMessage(
			`{"columns":3,"cells":[{"header":"Pricing discipline","body":"Retire ad-hoc discounts and enforce one price corridor per segment across all regions"},{"header":"Procurement","body":"Consolidate 1,400 suppliers into 300 strategic partners by the end of 2026"},{"header":"Network footprint","body":"Close four sub-scale depots and serve their regions from two hubs"},{"header":"Digital channel","body":"Move repeat orders to self-service and free up sales capacity for new accounts"},{"header":"Working capital","body":"Cut inventory days from 62 to 45 through demand-driven planning"},{"header":"Organisation","body":"Merge regional back offices into one shared-service centre"}]}`))
		slide.Pattern.Bounds = &GridBoundsInput{X: 5, Y: 22, Width: 90, Height: 38}
		deck := PresentationInput{Slides: []SlideInput{slide}}
		got := codesAt(t, tpl, deck, "/slides/0")
		if got[patterns.ErrCodeSlideUnderused]+got[patterns.ErrCodeVerticalImbalance] == 0 {
			t.Errorf("%s: six cards capped to the upper half report nothing (findings %v)", tpl, got)
		}
		if f, ok := lowerBandFindings(t, tpl, deck)["/slides/0"]; ok {
			if hint, _ := f.Fix.Params["hint"].(string); f.Fix.Params["band_capped_by"] != "author" || !strings.Contains(hint, "raise or remove the cap") {
				t.Errorf("%s: params %v, want band_capped_by author and the cap as the remedy", tpl, f.Fix.Params)
			}
		}
	}
}

// What the growth is for: a value-chain and a process-flow whose steps are
// sentences are full-slide exhibits and clear the rule on every template; the
// flow's boxes grow with the type step, never past a square.
func TestBandScaledFlowsClearTheLowerThird(t *testing.T) {
	for _, tpl := range lowerBandTemplates(t) {
		a := loadTemplateAnalysis(t, tpl)
		for _, name := range []string{"value-chain", "process-flow"} {
			p, _ := patterns.Default().Get(name)
			values, err := json.Marshal(p.(patterns.Exemplar).ExemplarValues())
			if err != nil {
				t.Fatal(err)
			}
			deck := PresentationInput{Slides: []SlideInput{titledPatternSlide(name, values)}}
			if !scalesToBand(expandSlidePatternGrid(&deck.Slides[0], 0, a.SlideWidth, a.SlideHeight, &a.Theme)) {
				t.Errorf("%s: the %s exemplar is not band-scaled", tpl, name)
			}
			collectGeometry(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme, func(u slideUsage) {
				if u.lowerBand < 0.26 || u.lowerBand > 0.30 {
					t.Errorf("%s: %s exemplar leaves %.1f%% under it, want the grown block's 28%%", tpl, name, 100*u.lowerBand)
				}
			})
		}
		// Short labels are not grown: a taller box around one word is a slab.
		short := titledPatternSlide("process-flow", json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Run"}]}`))
		if scalesToBand(expandSlidePatternGrid(&short, 0, a.SlideWidth, a.SlideHeight, &a.Theme)) {
			t.Errorf("%s: a three-word process-flow is band-scaled", tpl)
		}
	}
}
