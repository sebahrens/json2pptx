package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func metricListPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("metric-list")
	if !ok {
		t.Fatal("metric-list not registered")
	}
	return p
}

func metricItems(n int) []MetricListItem {
	items := make([]MetricListItem, n)
	for i := range items {
		items[i] = MetricListItem{Value: "2-5x", Label: "Research coverage", Detail: "Per analyst, versus 2023"}
	}
	return items
}

// metricRow is one expanded item row: the value and text cells.
type metricRow struct {
	value, text chartInsightsText
	valueFill   string
	bar         bool
}

// expandMetricRows expands the list and returns its item rows (rules and the
// callout excluded).
func expandMetricRows(t *testing.T, p Pattern, ctx ExpandContext, vals *MetricListValues, ovr any, co map[int]any) []metricRow {
	t.Helper()
	grid, err := p.Expand(ctx, vals, ovr, co)
	if err != nil {
		t.Fatal(err)
	}
	var out []metricRow
	for _, r := range grid.Rows {
		if len(r.Cells) != 2 {
			continue
		}
		out = append(out, metricRow{
			value:     cellText(t, r.Cells[0].Shape.Text),
			text:      cellText(t, r.Cells[1].Shape.Text),
			valueFill: string(r.Cells[0].Shape.Fill),
			bar:       r.Cells[0].AccentBar != nil,
		})
	}
	return out
}

func TestMetricList_Metadata(t *testing.T) {
	p := metricListPattern(t)
	if p.Version() != 1 || p.UseWhen() == "" || p.NotWhen() == "" || p.CellsHint() == "" {
		t.Fatalf("metadata incomplete: v=%d", p.Version())
	}
	for _, sibling := range []string{"kpi-3up", "stat-hero", "hero-detail", "exec-summary", "labeled-rows"} {
		if !strings.Contains(p.UseWhen()+p.NotWhen(), sibling) {
			t.Errorf("UseWhen/NotWhen should contrast with %s", sibling)
		}
	}
	tax := p.Taxonomy()
	if tax.Category != "data-display" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 || len(tax.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tax)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) {
		t.Fatalf("schema invalid: %v", err)
	}
	for _, want := range []string{"2020-12", "value_width_pct", "value_size", "label_size", "cell_accent_mode", "highlight", "callout"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema missing %q", want)
		}
	}
}

func TestMetricList_Validate(t *testing.T) {
	p := metricListPattern(t)
	for _, n := range []int{3, 5, 7} {
		if err := p.Validate(&MetricListValues{Items: metricItems(n)}, nil, nil); err != nil {
			t.Errorf("n=%d: unexpected error: %v", n, err)
		}
	}
	// Budgets count characters, not bytes: "€186.4M →" is 9 characters.
	euro := metricItems(3)
	euro[0].Value = "€186.4M → €2B"[:len("€186.4M → ")]
	euro[1].Label = strings.Repeat("ü", metricListLabelMax)
	if err := p.Validate(&MetricListValues{Items: euro}, nil, nil); err != nil {
		t.Errorf("multi-byte values inside the budget rejected: %v", err)
	}

	twoHighlights := metricItems(3)
	twoHighlights[0].Highlight = true
	twoHighlights[2].Highlight = true
	cases := []struct {
		name    string
		vals    *MetricListValues
		ovr     *MetricListOverrides
		cellOvr map[int]any
		want    string
	}{
		{name: "too few", vals: &MetricListValues{Items: metricItems(2)}, want: "stat-hero"},
		{name: "too many", vals: &MetricListValues{Items: metricItems(8)}, want: "items"},
		{name: "blank value", vals: &MetricListValues{Items: append(metricItems(2), MetricListItem{Label: "x"})}, want: "items[2].value"},
		{name: "blank label", vals: &MetricListValues{Items: append(metricItems(2), MetricListItem{Value: "1"})}, want: "items[2].label"},
		{name: "value too long", vals: &MetricListValues{Items: append(metricItems(2), MetricListItem{Value: strings.Repeat("9", metricListValueMax+1), Label: "x"})}, want: "items[2].value"},
		{name: "detail too long", vals: &MetricListValues{Items: append(metricItems(2), MetricListItem{Value: "1", Label: "x", Detail: strings.Repeat("d", metricListDetailMax+1)})}, want: "items[2].detail"},
		{name: "callout too long", vals: &MetricListValues{Items: metricItems(3), Callout: strings.Repeat("c", metricListCalloutMax+1)}, want: "callout"},
		{name: "two highlights", vals: &MetricListValues{Items: twoHighlights}, want: "items[2].highlight"},
		{name: "bad accent mode", vals: &MetricListValues{Items: metricItems(3)}, ovr: &MetricListOverrides{CellAccentMode: "rainbow"}, want: "cell_accent_mode"},
		{name: "value width out of range", vals: &MetricListValues{Items: metricItems(3)}, ovr: &MetricListOverrides{ValueWidthPct: 80}, want: "value_width_pct"},
		{name: "cell override out of range", vals: &MetricListValues{Items: metricItems(3)}, cellOvr: map[int]any{3: &MetricListCellOverride{AccentBar: true}}, want: "cell_overrides"},
		{name: "cell override unknown key", vals: &MetricListValues{Items: metricItems(3)}, cellOvr: map[int]any{0: map[string]any{"geometry": "ellipse"}}, want: "geometry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ovr any
			if tc.ovr != nil {
				ovr = tc.ovr
			}
			err := p.Validate(tc.vals, ovr, tc.cellOvr)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestMetricList_ExpandStructure(t *testing.T) {
	p := metricListPattern(t)
	vals := p.(Exemplar).ExemplarValues().(*MetricListValues)
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 4 item rows + 3 rules + the spacer and the callout band.
	if got := len(grid.Rows); got != 9 {
		t.Fatalf("rows = %d, want 9", got)
	}
	if grid.ColGap <= 0 || grid.ColGap >= 1 {
		t.Errorf("col_gap = %v: a highlighted row tints both cells, so the gap must be a hair above zero (0 resolves to 8pt)", grid.ColGap)
	}
	rows := expandMetricRows(t, p, fullThemeCtx(), vals, nil, nil)
	sizes := map[float64]bool{}
	for i, r := range rows {
		v := r.value.Paragraphs[0]
		if !v.Bold || v.Align != "r" || v.Content != vals.Items[i].Value {
			t.Errorf("row %d value = %+v, want bold right-aligned %q", i, v, vals.Items[i].Value)
		}
		sizes[v.Size] = true
		if r.text.Paragraphs[0].Content != vals.Items[i].Label || !r.text.Paragraphs[0].Bold {
			t.Errorf("row %d label = %+v", i, r.text.Paragraphs[0])
		}
		if len(r.text.Paragraphs) != 2 || r.text.Paragraphs[1].Size < 12 {
			t.Errorf("row %d detail missing or below 12pt: %+v", i, r.text.Paragraphs)
		}
	}
	if len(sizes) != 1 {
		t.Errorf("values should share one size, got %v", sizes)
	}
	// Uniform item row heights, point-capped.
	var h float64
	for i := 0; i < len(grid.Rows)-2; i += 2 {
		r := grid.Rows[i]
		if r.MinHeight != r.MaxHeight || r.MaxHeight <= 0 {
			t.Fatalf("item row %d not point-capped: %+v", i, r)
		}
		if h != 0 && r.MaxHeight != h {
			t.Errorf("item row %d height %.1f differs from %.1f", i, r.MaxHeight, h)
		}
		h = r.MaxHeight
	}
	// The callout is the shared takeaway band (go-slide-creator-3a1rm): a dark
	// neutral band the width of the list, never a solid accent banner.
	callout := grid.Rows[len(grid.Rows)-1].Cells[0]
	if callout.ColSpan != 2 {
		t.Fatalf("callout should span both columns, got %+v", callout)
	}
	assertTakeawayFilledBand(t, fullThemeCtx(), callout, vals.Callout)
}

func TestMetricList_Highlight(t *testing.T) {
	p := metricListPattern(t)
	vals := &MetricListValues{Items: metricItems(4)}
	vals.Items[1].Highlight = true
	rows := expandMetricRows(t, p, fullThemeCtx(), vals, nil, nil)
	for i, r := range rows {
		highlighted := i == 1
		if r.bar != highlighted {
			t.Errorf("row %d accent bar = %v, want %v", i, r.bar, highlighted)
		}
		tinted := strings.Contains(r.valueFill, "lumOff")
		if tinted != highlighted {
			t.Errorf("row %d fill = %s, tinted=%v want %v", i, r.valueFill, tinted, highlighted)
		}
	}

	// A light accent is not readable on white: the value ink is measured and
	// falls back to a theme ink instead of painting an invisible number.
	light := fullThemeCtx()
	for i := range light.Theme.Colors {
		if light.Theme.Colors[i].Name == "accent3" {
			light.Theme.Colors[i].RGB = "#FFE680"
		}
	}
	rows = expandMetricRows(t, p, light, vals, &MetricListOverrides{Accent: "accent3"}, nil)
	for i, r := range rows {
		if got := r.value.Paragraphs[0].Color; got == "accent3" {
			t.Errorf("row %d: pale accent3 value ink kept on a light surface", i)
		}
	}
}

func TestMetricList_AccentModesAndOverrides(t *testing.T) {
	p := metricListPattern(t)
	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{"uniform", "alternate", "progressive"} {
			rows := expandMetricRows(t, p, ExpandContext{}, &MetricListValues{Items: metricItems(4)}, &MetricListOverrides{Accent: base, CellAccentMode: mode}, nil)
			for i, r := range rows {
				if want := ResolveCellAccent(base, i, mode); r.value.Paragraphs[0].Color != want {
					t.Errorf("%s/%s row %d value colour = %q, want %q", base, mode, i, r.value.Paragraphs[0].Color, want)
				}
			}
		}
	}

	rows := expandMetricRows(t, p, fullThemeCtx(), &MetricListValues{Items: metricItems(3)},
		&MetricListOverrides{ValueSize: 20, LabelSize: 22}, map[int]any{2: &MetricListCellOverride{AccentBar: true, Color: "accent2"}})
	if rows[0].value.Paragraphs[0].Size != 20 || rows[0].text.Paragraphs[0].Size != 22 {
		t.Errorf("size overrides not applied: value=%v label=%v", rows[0].value.Paragraphs[0].Size, rows[0].text.Paragraphs[0].Size)
	}
	if !rows[2].bar || rows[2].value.Paragraphs[0].Color != "accent2" {
		t.Errorf("cell override not applied: bar=%v colour=%q", rows[2].bar, rows[2].value.Paragraphs[0].Color)
	}

	narrow, err := p.Expand(fullThemeCtx(), &MetricListValues{Items: metricItems(3)}, &MetricListOverrides{ValueWidthPct: 20}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(narrow.Columns) != "[20,80]" {
		t.Errorf("value_width_pct not applied: columns=%s", narrow.Columns)
	}
}

func TestMetricList_ValuesShrinkToOneLine(t *testing.T) {
	p := metricListPattern(t)
	vals := &MetricListValues{Items: metricItems(3)}
	vals.Items[0].Value = "$186.4M→$2B"
	rows := expandMetricRows(t, p, fullThemeCtx(), vals, &MetricListOverrides{ValueWidthPct: 15}, nil)
	if rows[0].value.Paragraphs[0].Size >= 40 {
		t.Errorf("a long value in a narrow column should shrink the shared size, got %v", rows[0].value.Paragraphs[0].Size)
	}
	if rows[0].value.Paragraphs[0].Size != rows[1].value.Paragraphs[0].Size {
		t.Error("values must share one size")
	}
}

func TestMetricList_PostExpandWarnings(t *testing.T) {
	p := metricListPattern(t).(PostExpandWarner)
	if got := p.PostExpandWarnings(fullThemeCtx(), &MetricListValues{Items: metricItems(4)}, nil); len(got) != 0 {
		t.Fatalf("ordinary list warned: %v", got)
	}
	wide := &MetricListValues{Items: metricItems(3)}
	wide.Items[1].Value = "WWWWWWWWWWWW"
	got := p.PostExpandWarnings(fullThemeCtx(), wide, &MetricListOverrides{ValueWidthPct: 15})
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeTextExceedsShape+":") || !strings.Contains(got[0], "items[1].value") {
		t.Fatalf("unfit value warning = %v", got)
	}
	dense := &MetricListValues{Items: metricItems(7), Callout: strings.Repeat("Callout words ", 10)}
	for i := range dense.Items {
		dense.Items[i].Label = strings.Repeat("Label words ", 5)
		dense.Items[i].Detail = strings.Repeat("Detail words here ", 6)
	}
	got = p.PostExpandWarnings(fullThemeCtx(), dense, nil)
	if len(got) == 0 || !strings.HasPrefix(got[len(got)-1], ErrCodeBodyTooLong+":") {
		t.Fatalf("overfull list should warn BODY_TOO_LONG, got %v", got)
	}
	if p.PostExpandWarnings(fullThemeCtx(), nil, nil) != nil {
		t.Error("nil values should not warn")
	}
}

// metricListLadder is the documented default value ladder.
var metricListLadder = map[float64]bool{40: true, 36: true, 32: true, 28: true, 24: true}

// go-slide-creator-1vmsk: default values stay on the 40/36/32/28/24pt ladder
// whatever the area and however long the longest value is. The exemplar under
// a title used to draw 18pt values beside 12pt labels.
func TestMetricList_ValuesStayOnTheLadder(t *testing.T) {
	p := metricListPattern(t)
	exemplar := (&metricList{}).ExemplarValues().(*MetricListValues)
	seven := &MetricListValues{Items: metricItems(7)}
	for i := range seven.Items {
		seven.Items[i].Detail = ""
	}
	for _, vals := range []*MetricListValues{exemplar, {Items: metricItems(3)}, {Items: metricItems(5)}, seven} {
		for h := 180.0; h <= 420; h += 20 {
			ctx := fullThemeCtx()
			ctx.LayoutBounds = LayoutBounds{Width: 660 * 12700, Height: int64(h * 12700)}
			rows := expandMetricRows(t, p, ctx, vals, nil, nil)
			size := rows[0].value.Paragraphs[0].Size
			if !metricListLadder[size] {
				t.Errorf("%d rows in %.0fpt: value size %vpt is off the 40/36/32/28/24 ladder", len(vals.Items), h, size)
			}
			for _, r := range rows {
				if r.value.Paragraphs[0].Size != size {
					t.Errorf("%d rows in %.0fpt: values do not share one size", len(vals.Items), h)
				}
				if r.text.Paragraphs[0].Size >= size {
					t.Errorf("%d rows in %.0fpt: %vpt label is not led by its %vpt value", len(vals.Items), h, r.text.Paragraphs[0].Size, size)
				}
			}
		}
	}

	// The exemplar on a titled slide (about 660 x 300pt of content).
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: 660 * 12700, Height: 300 * 12700}
	if size := expandMetricRows(t, p, ctx, exemplar, nil, nil)[0].value.Paragraphs[0].Size; size < 24 {
		t.Errorf("exemplar values render at %vpt, want 24pt or larger", size)
	}
	if got := p.(PostExpandWarner).PostExpandWarnings(ctx, exemplar, nil); len(got) != 0 {
		t.Errorf("exemplar warns: %v", got)
	}
}

// One long value widens the value column instead of shrinking every value.
func TestMetricList_LongValueWidensItsColumn(t *testing.T) {
	p := metricListPattern(t)
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: 660 * 12700, Height: 400 * 12700}
	short := &MetricListValues{Items: metricItems(3)}
	long := &MetricListValues{Items: metricItems(3)}
	long.Items[1].Value = "$186.4M→$2B"
	cols := func(v *MetricListValues, ovr any) []float64 {
		grid, err := p.Expand(ctx, v, ovr, nil)
		if err != nil {
			t.Fatal(err)
		}
		var c []float64
		if err := json.Unmarshal(grid.Columns, &c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := cols(short, nil); c[0] != metricListDefaultValuePct {
		t.Errorf("short values: value column %v%%, want the default %v%%", c[0], metricListDefaultValuePct)
	}
	c := cols(long, nil)
	if c[0] <= metricListDefaultValuePct || c[0] > metricListAutoMaxValuePct {
		t.Errorf("long value: value column %v%%, want it widened within %v-%v%%", c[0], metricListDefaultValuePct, metricListAutoMaxValuePct)
	}
	rows := expandMetricRows(t, p, ctx, long, nil, nil)
	size := rows[1].value.Paragraphs[0].Size
	if !metricListLadder[size] {
		t.Errorf("long value: size %vpt is off the ladder", size)
	}
	valueTextW := (660-metricListColGapPt)*c[0]/100 - 2*defaultShapeInsetLRPt
	if lines := measuredLines(long.Items[1].Value, ctx.Theme.BodyFont, true, size, valueTextW); lines != 1 {
		t.Errorf("long value wraps to %d lines at %vpt in its %v%% column", lines, size, c[0])
	}
	// An authored column width is kept.
	if c := cols(long, &MetricListOverrides{ValueWidthPct: 20}); c[0] != 20 {
		t.Errorf("authored value_width_pct 20 became %v", c[0])
	}
}

// A list too tall for 24pt values at the uniform row margin drops the margin
// first; one that still does not fit keeps its 24pt values and is reported
// with its row count.
func TestMetricList_TallListDropsSpacingThenReports(t *testing.T) {
	p := metricListPattern(t)
	seven := &MetricListValues{Items: metricItems(7)}
	for i := range seven.Items {
		seven.Items[i].Detail = ""
	}
	tightRows := func(ctx ExpandContext) (size float64, tight bool) {
		grid, err := p.Expand(ctx, seven, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range grid.Rows {
			if len(r.Cells) != 2 {
				continue
			}
			var txt struct {
				Paragraphs []struct {
					Size float64 `json:"size"`
				} `json:"paragraphs"`
				InsetTop *float64 `json:"inset_top"`
			}
			if err := json.Unmarshal(r.Cells[0].Shape.Text, &txt); err != nil {
				t.Fatal(err)
			}
			return txt.Paragraphs[0].Size, txt.InsetTop != nil
		}
		t.Fatal("no item row")
		return 0, false
	}
	warner := p.(PostExpandWarner)

	roomy := fullThemeCtx()
	roomy.LayoutBounds = LayoutBounds{Width: 660 * 12700, Height: 460 * 12700}
	if _, tight := tightRows(roomy); tight {
		t.Error("a list that fits keeps the uniform row margin")
	}

	snug := fullThemeCtx()
	snug.LayoutBounds = LayoutBounds{Width: 660 * 12700, Height: 300 * 12700}
	size, tight := tightRows(snug)
	if size != 24 || !tight {
		t.Errorf("7 rows in 300pt: value %vpt, tight margin %v; want 24pt values on tight rows", size, tight)
	}
	if got := warner.PostExpandWarnings(snug, seven, nil); len(got) != 0 {
		t.Errorf("7 rows that fit on tight rows warn: %v", got)
	}

	short := fullThemeCtx()
	short.LayoutBounds = LayoutBounds{Width: 660 * 12700, Height: 200 * 12700}
	if size, _ := tightRows(short); size != 24 {
		t.Errorf("7 rows in 200pt: value %vpt, want the 24pt floor kept", size)
	}
	got := warner.PostExpandWarnings(short, seven, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") || !strings.Contains(got[0], "7 rows need") || !strings.Contains(got[0], "24pt values") {
		t.Errorf("7 rows in 200pt: want one BODY_TOO_LONG naming the row count and the 24pt values, got %v", got)
	}
}

// overrides.value_size is still honoured, and still shrinks for a long value.
func TestMetricList_AuthoredValueSizeHonoured(t *testing.T) {
	p := metricListPattern(t)
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: 660 * 12700, Height: 400 * 12700}
	for _, size := range []float64{20, 30, 48} {
		rows := expandMetricRows(t, p, ctx, &MetricListValues{Items: metricItems(3)}, &MetricListOverrides{ValueSize: size}, nil)
		if got := rows[0].value.Paragraphs[0].Size; got != size {
			t.Errorf("value_size %v rendered at %v", size, got)
		}
	}
}
