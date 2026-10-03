package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-bzh34: content inside its published budget must fit the
// content area it is expanded into, and content that cannot must be reported
// before generation rather than written shrunk below the 12pt floor.

const bzh34TableHighlight = `{"criteria":[{"label":"Speed"},{"label":"Cost"},{"label":"Control"},{"label":"Risk"}],
	"options":[{"name":"Build in-house","detail":"Own models and platform","scores":["1","1","4","2"]},
		{"name":"Platform plus partner","detail":"Shared platform, partner apps","scores":["3","3","3","3"]},
		{"name":"Buy point tools","detail":"Vendor copilots per team","scores":["4","2","1","2"]}],
	"highlight_row":1,"highlight_label":"Recommended"}`

func TestTableHighlight_FitsShortContentArea(t *testing.T) {
	v := &TableHighlightValues{}
	if err := json.Unmarshal([]byte(bzh34TableHighlight), v); err != nil {
		t.Fatal(err)
	}
	// midnight-blue's blank-title content area under a takeaway bar: the
	// estimated table (320pt) over-filled it and the grid shrank the names.
	fits := ExpandContext{LayoutBounds: LayoutBounds{Width: 10515600, Height: 3826700}}
	l := newTHLayout(fits, v, &TableHighlightOverrides{})
	l.fit()
	if !l.tight || l.total() > l.areaH {
		t.Fatalf("table should fit %.1fpt with writer-measured rows: tight=%v total=%.1f", l.areaH, l.tight, l.total())
	}
	for _, w := range (&tableHighlight{}).PostExpandWarnings(fits, v, nil) {
		t.Errorf("unexpected warning: %s", w)
	}

	// modern's shorter area holds three two-line options, the tag line and a
	// legend at 12pt once the rows give up padding (go-slide-creator-vg73u):
	// the text keeps its size and the rows are tighter.
	modern := ExpandContext{LayoutBounds: LayoutBounds{Width: 10831550, Height: 3340690}}
	lm := newTHLayout(modern, v, &TableHighlightOverrides{})
	lm.fit()
	if lm.padPt <= 0 || lm.total() > lm.areaH || lm.bodySize < scaleBodyPt {
		t.Fatalf("table should fit %.1fpt with tighter rows at 12pt: pad=%v total=%.1f body=%v", lm.areaH, lm.padPt, lm.total(), lm.bodySize)
	}
	for _, w := range (&tableHighlight{}).PostExpandWarnings(modern, v, nil) {
		t.Errorf("unexpected warning: %s", w)
	}

	// An area that cannot hold them even at the tightest rows is reported
	// before generation.
	short := ExpandContext{LayoutBounds: LayoutBounds{Width: 10831550, Height: 2413000}}
	warnings := (&tableHighlight{}).PostExpandWarnings(short, v, nil)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], ErrCodeBodyTooLong) || !strings.Contains(warnings[0], "content area") {
		t.Fatalf("want one template-aware BODY_TOO_LONG, got %v", warnings)
	}
}

func TestChartInsightsSplit_StackedColumnFitsContentArea(t *testing.T) {
	v := &ChartInsightsSplitValues{}
	if err := json.Unmarshal([]byte(`{
		"chart": {"type": "bar_chart", "data": {"categories": ["2023", "2024", "2025", "2026"], "series": [{"name": "Share", "values": [22, 35, 58, 78]}]}},
		"insights_title": "What drove it",
		"insights": ["Packaged copilots removed the build barrier for common tasks.", "Evaluation tooling made quality measurable before launch.", "Falling inference costs turned marginal cases positive."],
		"headline": {"value": "+56 pts", "label": "production adoption 2023 to 2026"},
		"so_what": "The question is no longer whether to adopt, but how fast to scale.",
		"source": "Illustrative estimates for discussion"}`), v); err != nil {
		t.Fatal(err)
	}
	// midnight-blue's blank-title content area.
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 10515600, Height: 4437062}}
	ovr := &ChartInsightsSplitOverrides{}
	panel := buildInsightsPanel(v.InsightsTitle, v.Insights, "accent1", 12, 12)
	cell, pct, short := buildInsightsColumn(ctx, v, ovr, panel, "accent1", chartInsightsDefaultPct)
	if short > 0 {
		t.Fatalf("stacked column is %.1fpt short at chart %.0f%%", short, pct)
	}
	if pct > chartInsightsDefaultPct || pct < cisMinChartPct {
		t.Errorf("chart pct = %.0f, want within [%.0f, %.0f]", pct, cisMinChartPct, chartInsightsDefaultPct)
	}
	if r := cell.Grid.Rows[0]; r.MinHeight <= 0 || r.MinHeight != r.MaxHeight {
		t.Errorf("headline row should be pinned in points: %+v", r)
	}
	if r := cell.Grid.Rows[2]; !r.AutoHeight || r.MinHeight <= 0 {
		t.Errorf("so-what band row should be floored in points: %+v", r)
	}
	for _, w := range (&chartInsightsSplit{}).PostExpandWarnings(ctx, v, nil) {
		t.Errorf("unexpected warning: %s", w)
	}

	// A pinned chart width cannot narrow; the column may then not fit.
	pinned := &ChartInsightsSplitOverrides{ChartWidthPct: 75}
	if _, pct, _ := buildInsightsColumn(ctx, v, pinned, panel, "accent1", 75); pct != 75 {
		t.Errorf("chart_width_pct must pin the chart, got %.0f", pct)
	}
	if w := cisColumnAreaWarning(ctx, v, pinned); !strings.HasPrefix(w, ErrCodeBodyTooLong) {
		t.Errorf("a column that cannot fit must warn, got %q", w)
	}
}
