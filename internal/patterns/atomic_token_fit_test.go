package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

// substitutedFontCtx is a 16:9 content area whose theme body font is not
// embedded ("Segoe UI", as on modern-yellow), so every atomic token is
// measured against textfit.AtomicTokenWidthPct of its width wherever the host
// lacks the face.
func substitutedFontCtx() ExpandContext {
	ctx := kpiTestCtx()
	ctx.Theme.BodyFont = "Segoe UI"
	return ctx
}

// go-slide-creator-b7qqg.14: the catalog kpi-2up..6up values must keep their
// magnitude and unit on one line ("$4.2M", not "$4.2" / "M") at a readable
// size, measured against the width a stand-in face may really need.
func TestKPINupCatalogValuesFitAsAtomicTokens(t *testing.T) {
	for _, ctx := range []ExpandContext{kpiTestCtx(), substitutedFontCtx()} {
		font := ctx.Theme.BodyFont
		for _, name := range []string{"kpi-2up", "kpi-3up", "kpi-4up", "kpi-5up", "kpi-6up"} {
			pat, ok := Default().Get(name)
			if !ok {
				t.Fatalf("%s not registered", name)
			}
			k := pat.(*kpiNup)
			cells := KPINupValues(k.cfg.Exemplars)
			if len(cells) == 0 {
				t.Fatalf("%s has no exemplar values", name)
			}
			geo := kpiCardGeometryFor(ctx, k.cfg.Count)
			pos := geo.iconPosition()
			size := kpiFitBigSize(ctx, cells, resolveKPIBigSize(nil), geo, pos)
			if size < 24 {
				t.Errorf("%s (font %q): values shrink to %.0fpt, below a readable KPI size", name, font, size)
			}
			for i, c := range cells {
				w := textfit.AtomicTokenWidthPt(font, geo.valueWidthPt(c.Icon, pos))
				if lines := measuredLines(c.Big, font, true, size, w); lines != 1 {
					t.Errorf("%s (font %q) values[%d].big %q wraps to %d lines at %.0fpt", name, font, i, c.Big, lines, size)
				}
			}
			if w := k.PostExpandWarnings(ctx, &cells, nil); len(w) != 0 {
				t.Errorf("%s (font %q): catalog values warn: %v", name, font, w)
			}
		}
	}
}

// A value too long for its card at the floor is reported, not left to wrap.
func TestKPINupTooLongValueWarns(t *testing.T) {
	pat, _ := Default().Get("kpi-6up")
	cells := KPINupValues{
		{Big: "$4,218,500,000", Small: "ARR"}, {Big: "127%", Small: "NRR"}, {Big: "12d", Small: "Cycle"},
		{Big: "98%", Small: "CSAT"}, {Big: "42", Small: "NPS"}, {Big: "3.2x", Small: "LTV/CAC"},
	}
	got := pat.(PostExpandWarner).PostExpandWarnings(substitutedFontCtx(), &cells, nil)
	if len(got) == 0 || !strings.Contains(got[0], "values[0].big") {
		t.Fatalf("over-long KPI value not reported: %v", got)
	}
}

// go-slide-creator-b7qqg.15: the exemplar row labels ("DESIGN PROCESS",
// "PRODUCTION") must stay whole words — the label column widens before the
// label shrinks, and the label stays at a readable size.
func TestProcessGrid2RowLabelsNeverBreakMidWord(t *testing.T) {
	p := &processGrid2Row{}
	vals := &ProcessGrid2RowValues{
		Row1Label: "DESIGN PROCESS", Row1Phases: []string{"DESIGN", "EDIT", "ASSETS", "UX / UI"},
		Row2Label: "PRODUCTION", Row2Phases: []string{"PROTOTYPE", "DEVELOP", "USER TESTING", "RELEASE"},
	}
	tinted := &ProcessGrid2RowOverrides{Style: "tinted"}
	for _, ctx := range []ExpandContext{kpiTestCtx(), substitutedFontCtx()} {
		// The default lanes style keeps the words whole in its pentagons too.
		for _, v := range []*ProcessGrid2RowValues{vals, p.ExemplarValues().(*ProcessGrid2RowValues)} {
			if lanes := fitProcessGrid2RowLanes(ctx, v, &ProcessGrid2RowOverrides{}); len(lanes.unfit) != 0 || lanes.labelPt < 12 {
				t.Errorf("font %q lanes: labels %v unfit at %.0fpt", ctx.Theme.BodyFont, lanes.unfit, lanes.labelPt)
			}
			if w := p.PostExpandWarnings(ctx, v, nil); len(w) != 0 {
				t.Errorf("font %q lanes: warns: %v", ctx.Theme.BodyFont, w)
			}
		}
		fit := fitProcessGrid2RowLabels(ctx, vals, 14)
		if len(fit.unfit) != 0 {
			t.Errorf("font %q: exemplar label words do not fit: %v", ctx.Theme.BodyFont, fit.unfit)
		}
		if fit.sizePt < 12 {
			t.Errorf("font %q: label shrank to %.0fpt", ctx.Theme.BodyFont, fit.sizePt)
		}
		grid, err := p.Expand(ctx, vals, tinted, nil)
		if err != nil {
			t.Fatal(err)
		}
		var cols []float64
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatal(err)
		}
		if cols[0] != fit.colPct {
			t.Errorf("label column %.0f%%, fit chose %.0f%%", cols[0], fit.colPct)
		}
		if w := p.PostExpandWarnings(ctx, vals, tinted); len(w) != 0 {
			t.Errorf("font %q: exemplar warns: %v", ctx.Theme.BodyFont, w)
		}
	}
	// Short labels keep the default 12% column.
	short := &ProcessGrid2RowValues{Row1Label: "PLAN", Row1Phases: []string{"A", "B", "C"}, Row2Label: "RUN", Row2Phases: []string{"D", "E", "F"}}
	if fit := fitProcessGrid2RowLabels(kpiTestCtx(), short, 14); fit.colPct != processGrid2RowLabelColPct || fit.sizePt != 14 {
		t.Errorf("short labels changed the column: %+v", fit)
	}
}

// A label word that cannot fit even the widest column at the floor gets a
// measured TEXT_EXCEEDS_SHAPE warning naming the word.
func TestProcessGrid2RowUnfittableLabelWarns(t *testing.T) {
	p := &processGrid2Row{}
	vals := &ProcessGrid2RowValues{
		Row1Label: "INTERNATIONALISATION", Row1Phases: []string{"A", "B", "C"},
		Row2Label: "RUN", Row2Phases: []string{"D", "E", "F"},
	}
	ctx := kpiTestCtx()
	ctx.LayoutBounds.Width /= 2 // a half-width content area
	got := p.PostExpandWarnings(ctx, vals, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeTextExceedsShape) || !strings.Contains(got[0], "INTERNATIONALISATION") {
		t.Fatalf("unfittable label word not reported: %v", got)
	}
}
