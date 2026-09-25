package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func labeledRowsPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("labeled-rows")
	if !ok {
		t.Fatal("labeled-rows not registered")
	}
	return p
}

func labeledRowItems(n int) []LabeledRow {
	rows := make([]LabeledRow, n)
	for i := range rows {
		rows[i] = LabeledRow{Label: "WHY", Sublabel: "now is the time", Body: "Short supporting body text for the row."}
	}
	return rows
}

// contentRows returns the label/body rows of an expanded grid (rules excluded).
func contentRows(grid *jsonschema.ShapeGridInput) []jsonschema.GridRowInput {
	var out []jsonschema.GridRowInput
	for _, r := range grid.Rows {
		if len(r.Cells) == 2 {
			out = append(out, r)
		}
	}
	return out
}

func TestLabeledRows_Metadata(t *testing.T) {
	p := labeledRowsPattern(t)
	if p.Version() != 1 || p.UseWhen() == "" || p.NotWhen() == "" || p.CellsHint() == "" {
		t.Fatalf("metadata incomplete: v=%d", p.Version())
	}
	for _, sibling := range []string{"exec-summary", "metric-list", "comparison-2col", "scqa-summary", "kpi-Nup", "stat-hero"} {
		if !strings.Contains(p.UseWhen()+p.NotWhen(), sibling) {
			t.Errorf("UseWhen/NotWhen should contrast with %s", sibling)
		}
	}
	tax := p.Taxonomy()
	if tax.Category != "narrative" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 || len(tax.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tax)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) {
		t.Fatalf("schema invalid: %v", err)
	}
	for _, want := range []string{"2020-12", "label_style", "label_width_pct", "label_size", "body_size", "cell_accent_mode", "sublabel"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema missing %q", want)
		}
	}
}

func TestLabeledRows_Validate(t *testing.T) {
	p := labeledRowsPattern(t)
	for _, n := range []int{2, 4, 6} {
		if err := p.Validate(&LabeledRowsValues{Rows: labeledRowItems(n)}, nil, nil); err != nil {
			t.Errorf("n=%d: unexpected error: %v", n, err)
		}
	}
	// Budgets count characters: 24 umlauts are 48 bytes.
	multi := labeledRowItems(2)
	multi[0].Label = strings.Repeat("ü", labeledRowsLabelMax)
	multi[1].Body = strings.Repeat("€", labeledRowsBodyMax)
	if err := p.Validate(&LabeledRowsValues{Rows: multi}, nil, nil); err != nil {
		t.Errorf("multi-byte values inside the budget rejected: %v", err)
	}

	cases := []struct {
		name    string
		vals    *LabeledRowsValues
		ovr     *LabeledRowsOverrides
		cellOvr map[int]any
		want    string
	}{
		{name: "too few", vals: &LabeledRowsValues{Rows: labeledRowItems(1)}, want: "pull-quote"},
		{name: "too many", vals: &LabeledRowsValues{Rows: labeledRowItems(7)}, want: "rows"},
		{name: "blank label", vals: &LabeledRowsValues{Rows: append(labeledRowItems(1), LabeledRow{Body: "b"})}, want: "rows[1].label"},
		{name: "blank body", vals: &LabeledRowsValues{Rows: append(labeledRowItems(1), LabeledRow{Label: "HOW"})}, want: "rows[1].body"},
		{name: "label too long", vals: &LabeledRowsValues{Rows: append(labeledRowItems(1), LabeledRow{Label: strings.Repeat("x", labeledRowsLabelMax+1), Body: "b"})}, want: "rows[1].label"},
		{name: "sublabel too long", vals: &LabeledRowsValues{Rows: append(labeledRowItems(1), LabeledRow{Label: "x", Sublabel: strings.Repeat("s", labeledRowsSublabelMax+1), Body: "b"})}, want: "rows[1].sublabel"},
		{name: "body too long", vals: &LabeledRowsValues{Rows: append(labeledRowItems(1), LabeledRow{Label: "x", Body: strings.Repeat("b", labeledRowsBodyMax+1)})}, want: "rows[1].body"},
		{name: "bad label style", vals: &LabeledRowsValues{Rows: labeledRowItems(2)}, ovr: &LabeledRowsOverrides{LabelStyle: "outline"}, want: "label_style"},
		{name: "bad accent mode", vals: &LabeledRowsValues{Rows: labeledRowItems(2)}, ovr: &LabeledRowsOverrides{CellAccentMode: "rainbow"}, want: "cell_accent_mode"},
		{name: "label width out of range", vals: &LabeledRowsValues{Rows: labeledRowItems(2)}, ovr: &LabeledRowsOverrides{LabelWidthPct: 60}, want: "label_width_pct"},
		{name: "cell override out of range", vals: &LabeledRowsValues{Rows: labeledRowItems(2)}, cellOvr: map[int]any{2: &LabeledRowsCellOverride{AccentBar: true}}, want: "cell_overrides"},
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

func TestLabeledRows_ExpandFilled(t *testing.T) {
	p := labeledRowsPattern(t)
	vals := p.(Exemplar).ExemplarValues().(*LabeledRowsValues)
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 3 rows + 2 rules between them.
	if got := len(grid.Rows); got != 5 {
		t.Fatalf("rows = %d, want 5", got)
	}
	if grid.Rows[1].Cells[0].ColSpan != 2 {
		t.Error("rule row should span both columns")
	}
	for i, r := range contentRows(grid) {
		label := r.Cells[0].Shape
		if string(label.Fill) != `"accent1"` {
			t.Errorf("row %d label fill = %s, want accent1 block", i, label.Fill)
		}
		txt := cellText(t, label.Text)
		if txt.Paragraphs[0].Content != vals.Rows[i].Label || !txt.Paragraphs[0].Bold {
			t.Errorf("row %d keyword = %+v", i, txt.Paragraphs[0])
		}
		if txt.Paragraphs[0].Color != "lt1" {
			t.Errorf("row %d keyword ink on dark accent1 = %q, want lt1 (measured)", i, txt.Paragraphs[0].Color)
		}
		if len(txt.Paragraphs) != 2 || txt.Paragraphs[1].Size >= txt.Paragraphs[0].Size || txt.Paragraphs[1].Size < 12 {
			t.Errorf("row %d sublabel should be smaller than the keyword and >= 12pt: %+v", i, txt.Paragraphs)
		}
		body := cellText(t, r.Cells[1].Shape.Text).Paragraphs[0]
		if body.Size < 12 || body.Color != "dk1" {
			t.Errorf("row %d body = %+v", i, body)
		}
		if r.MinHeight != r.MaxHeight || r.MaxHeight <= 0 {
			t.Errorf("row %d should be point-capped: %+v", i, r)
		}
	}
	// **Bold** in the body becomes inline markup.
	if body := cellText(t, contentRows(grid)[0].Cells[1].Shape.Text).Paragraphs[0].Content; !strings.Contains(body, "<b>") {
		t.Errorf("markdown emphasis not converted: %q", body)
	}
}

func TestLabeledRows_TextStyleAndOverrides(t *testing.T) {
	p := labeledRowsPattern(t)
	grid, err := p.Expand(fullThemeCtx(), &LabeledRowsValues{Rows: labeledRowItems(3)},
		&LabeledRowsOverrides{LabelStyle: "text", LabelWidthPct: 30, LabelSize: 24, BodySize: 16},
		map[int]any{1: &LabeledRowsCellOverride{AccentBar: true}})
	if err != nil {
		t.Fatal(err)
	}
	if string(grid.Columns) != "[30,70]" {
		t.Errorf("label_width_pct not applied: %s", grid.Columns)
	}
	for i, r := range contentRows(grid) {
		label := r.Cells[0].Shape
		if string(label.Fill) != `"none"` {
			t.Errorf("text style row %d should have no fill, got %s", i, label.Fill)
		}
		txt := cellText(t, label.Text)
		if txt.Paragraphs[0].Color != "accent1" || txt.Paragraphs[0].Size != 24 {
			t.Errorf("text style keyword = %+v, want accent1 at 24pt", txt.Paragraphs[0])
		}
		if body := cellText(t, r.Cells[1].Shape.Text).Paragraphs[0]; body.Size != 16 {
			t.Errorf("body_size not applied: %v", body.Size)
		}
		if (r.Cells[1].AccentBar != nil) != (i == 1) {
			t.Errorf("row %d accent bar = %v", i, r.Cells[1].AccentBar)
		}
	}
}

func TestLabeledRows_CellAccentModes(t *testing.T) {
	p := labeledRowsPattern(t)
	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{"uniform", "alternate", "progressive"} {
			grid, err := p.Expand(ExpandContext{}, &LabeledRowsValues{Rows: labeledRowItems(4)}, &LabeledRowsOverrides{Accent: base, CellAccentMode: mode}, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i, r := range contentRows(grid) {
				want := `"` + ResolveCellAccent(base, i, mode) + `"`
				if got := string(r.Cells[0].Shape.Fill); got != want {
					t.Errorf("%s/%s row %d fill = %s, want %s", base, mode, i, got, want)
				}
			}
		}
	}
}

func TestLabeledRows_ContentSized(t *testing.T) {
	p := labeledRowsPattern(t)
	short := &LabeledRowsValues{Rows: []LabeledRow{{Label: "A", Body: "One."}, {Label: "B", Body: "Two."}}}
	grid, err := p.Expand(fullThemeCtx(), short, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, areaH := sizingAreaPt(fullThemeCtx())
	sum := 0.0
	for _, r := range grid.Rows {
		sum += r.MaxHeight
	}
	if sum >= areaH*0.9 {
		t.Errorf("short rows should not fill the area: %.0f of %.0f", sum, areaH)
	}
	if sum < areaH*0.5 {
		t.Errorf("sparse rows should still read as the slide's content: %.0f of %.0f", sum, areaH)
	}

	long := &LabeledRowsValues{Rows: labeledRowItems(3)}
	long.Rows[1].Body = strings.Repeat("Much longer body text that wraps. ", 8)
	grid, err = p.Expand(fullThemeCtx(), long, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows := contentRows(grid)
	if rows[1].MaxHeight <= rows[0].MaxHeight {
		t.Errorf("the long row should be taller: %.1f vs %.1f", rows[1].MaxHeight, rows[0].MaxHeight)
	}
}

func TestLabeledRows_PostExpandWarnings(t *testing.T) {
	p := labeledRowsPattern(t).(PostExpandWarner)
	if got := p.PostExpandWarnings(fullThemeCtx(), &LabeledRowsValues{Rows: labeledRowItems(4)}, nil); len(got) != 0 {
		t.Fatalf("ordinary rows warned: %v", got)
	}
	wide := &LabeledRowsValues{Rows: labeledRowItems(2)}
	wide.Rows[0].Label = "Internationalization"
	got := p.PostExpandWarnings(fullThemeCtx(), wide, &LabeledRowsOverrides{LabelWidthPct: 12})
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeTextExceedsShape+":") || !strings.Contains(got[0], "rows[0].label") {
		t.Fatalf("unfit keyword warning = %v", got)
	}
	dense := &LabeledRowsValues{Rows: labeledRowItems(6)}
	for i := range dense.Rows {
		dense.Rows[i].Body = strings.Repeat("Body words that keep going. ", 11)[:labeledRowsBodyMax]
	}
	got = p.PostExpandWarnings(fullThemeCtx(), dense, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") {
		t.Fatalf("overfull rows should warn BODY_TOO_LONG, got %v", got)
	}
	if p.PostExpandWarnings(fullThemeCtx(), nil, nil) != nil {
		t.Error("nil values should not warn")
	}
}
