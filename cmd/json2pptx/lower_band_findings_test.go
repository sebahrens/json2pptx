package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// tableSlide is the slide the DeckSpec table kind compiles to: a title over a
// one-cell grid holding a native table of n data rows.
func tableSlide(n int) SlideInput {
	title := "Four levers deliver EUR 9.0M of the EUR 9.6M target"
	rows := make([][]jsonschema.TableCellInput, n)
	for i := range rows {
		rows[i] = []jsonschema.TableCellInput{{Content: "Rightsize compute"}, {Content: "EUR 2.9M"}, {Content: "Medium"}, {Content: "Platform"}}
	}
	return SlideInput{
		SlideType: "content",
		LayoutID:  "blank-title",
		Content:   []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		ShapeGrid: &ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{
			Table: &jsonschema.TableInput{Headers: []string{"Lever", "Annual saving", "Effort", "Owner"}, Rows: rows},
		}}}}},
	}
}

func codesOn(findings []patterns.FitFinding, codes ...string) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range findings {
		for _, c := range codes {
			if f.Code == c {
				out = append(out, f)
			}
		}
	}
	return out
}

// A table is written at its row heights from the top of its cell. Counted as
// the whole cell, four rows over an empty lower half read as a full slide and
// scored 100 (journey h-A15). Its planned rows are the ink now: a short table
// reports VERTICAL_IMBALANCE with the table's own remedies, and one whose
// rows reach into the lower third does not (go-slide-creator-i7yju).
func TestShortTableOverAnEmptyBandIsReported(t *testing.T) {
	for _, tpl := range loneRowTemplates {
		if _, err := os.Stat(filepath.Join("..", "..", "templates", tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		a := loadTemplateAnalysis(t, tpl)
		for _, tc := range []struct {
			rows int
			want bool
		}{{2, true}, {4, true}, {10, false}} {
			deck := PresentationInput{Slides: []SlideInput{tableSlide(tc.rows)}}
			got := codesOn(collectGeometryFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme), patterns.ErrCodeVerticalImbalance)
			if (len(got) > 0) != tc.want {
				t.Errorf("%s: %d-row table: VERTICAL_IMBALANCE reported = %v, want %v (%+v)", tpl, tc.rows, len(got) > 0, tc.want, got)
				continue
			}
			if !tc.want {
				continue
			}
			f := got[0]
			if f.Action != "review" || f.Fix == nil {
				t.Fatalf("%s: finding = %+v", tpl, f)
			}
			if side, _ := f.Fix.Params["empty_band_side"].(string); side != "below" {
				t.Errorf("%s: %d-row table: empty_band_side = %q, want below", tpl, tc.rows, side)
			}
			if pct, _ := f.Fix.Params["empty_band_pct"].(float64); pct < 100*slideLowerBandMaxFrac {
				t.Errorf("%s: %d-row table: empty_band_pct = %v, want a third or more", tpl, tc.rows, pct)
			}
			if hint, _ := f.Fix.Params["hint"].(string); !strings.Contains(hint, "rows") || strings.Contains(hint, "vertical_align") {
				t.Errorf("%s: hint must name the table's remedies, not vertical_align: %q", tpl, hint)
			}
		}
	}
}

// The band under a table is measured to the third: a table that leaves
// between a third and 40% of the area empty hangs from the body line like any
// block, but has no other placement, so it is reported where a pattern block
// is not.
func TestTableBandIsHeldToAThird(t *testing.T) {
	a := loadTemplateAnalysis(t, "midnight-blue")
	reported := func(rows int) (bool, float64) {
		deck := PresentationInput{Slides: []SlideInput{tableSlide(rows)}}
		got := codesOn(collectGeometryFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme), patterns.ErrCodeVerticalImbalance)
		if len(got) == 0 {
			return false, 0
		}
		pct, _ := got[0].Fix.Params["empty_band_pct"].(float64)
		return true, pct
	}
	seenBetween := false
	last := true
	for rows := 2; rows <= 10; rows++ {
		ok, pct := reported(rows)
		if ok && !last {
			t.Errorf("%d rows reported after a shorter table was not", rows)
		}
		if ok && pct < 40 {
			seenBetween = true
		}
		if ok && pct < 100*slideLowerBandMaxFrac {
			t.Errorf("%d rows: reported at %v%%, under a third", rows, pct)
		}
		last = ok
	}
	if last {
		t.Error("a ten-row table must not be reported")
	}
	if !seenBetween {
		t.Log("no row count on midnight-blue leaves between a third and 40% empty; the third is exercised by the other templates")
	}
}

// A KPI row that still leaves the lower third of the content area empty says
// so: tiles keep their content height and kpi-inline is a supporting band.
// The open kpi-Nup row, grown into the free height, does not report. Each
// finding is one review advisory with the remedy for its pattern.
func TestKPIRowOverAnEmptyLowerThirdIsReported(t *testing.T) {
	for _, tpl := range loneRowTemplates {
		if _, err := os.Stat(filepath.Join("..", "..", "templates", tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		a := loadTemplateAnalysis(t, tpl)
		balance := []string{patterns.ErrCodeSlideUnderused, patterns.ErrCodeVerticalImbalance}
		find := func(slide SlideInput) []patterns.FitFinding {
			deck := PresentationInput{Slides: []SlideInput{slide}}
			return codesOn(collectGeometryFindings(&deck, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme), balance...)
		}

		for n := 2; n <= 6; n++ {
			name := "kpi-" + string(rune('0'+n)) + "up"
			if got := find(titledPatternSlide(name, kpiValuesN(t, n))); len(got) != 0 {
				t.Errorf("%s: an open %s row alone on a slide reported %+v", tpl, name, got)
			}
		}

		tiles := titledPatternSlide("kpi-3up", kpiValuesN(t, 3))
		tiles.Pattern.Overrides = json.RawMessage(`{"style":"tiles"}`)
		inline := titledPatternSlide("kpi-inline", kpiValuesN(t, 4))
		for _, tc := range []struct {
			name, hint string
			slide      SlideInput
		}{
			{"kpi-3up tiles", "delta or comparator", tiles},
			{"kpi-inline", "supporting band", inline},
		} {
			got := find(tc.slide)
			if len(got) != 1 || got[0].Code != patterns.ErrCodeSlideUnderused || got[0].Action != "review" {
				t.Errorf("%s: %s: findings = %+v, want one SLIDE_UNDERUSED review", tpl, tc.name, got)
				continue
			}
			p := got[0].Fix.Params
			if p["empty_band_side"] != "below" || p["threshold_pct"] != float64(33) {
				t.Errorf("%s: %s: params = %+v", tpl, tc.name, p)
			}
			if pct, _ := p["empty_band_pct"].(float64); pct < 33 {
				t.Errorf("%s: %s: empty_band_pct = %v", tpl, tc.name, pct)
			}
			if hint, _ := p["hint"].(string); !strings.Contains(hint, tc.hint) {
				t.Errorf("%s: %s: hint %q does not name %q", tpl, tc.name, hint, tc.hint)
			}
			if !strings.Contains(got[0].Message, "lower") {
				t.Errorf("%s: %s: message = %q", tpl, tc.name, got[0].Message)
			}
		}

		// A row pinned to the top is already a lopsided slide: one finding.
		top := titledPatternSlide("kpi-4up", kpiValuesN(t, 4))
		top.Pattern.VerticalAlign = "top"
		got := find(top)
		if len(got) != 1 || got[0].Code != patterns.ErrCodeVerticalImbalance {
			t.Errorf("%s: top-pinned row: findings = %+v, want VERTICAL_IMBALANCE alone", tpl, got)
		}
	}
}
