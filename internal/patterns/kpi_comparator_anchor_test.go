package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// kpiParagraphsOf returns the paragraph contents of one expanded KPI cell.
func kpiParagraphsOf(t *testing.T, text json.RawMessage) []string {
	t.Helper()
	var body struct {
		Paragraphs []struct {
			Content string `json:"content"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(text, &body); err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(body.Paragraphs))
	for i, p := range body.Paragraphs {
		out[i] = p.Content
	}
	return out
}

// A comparator floated under its card: six KPIs on modern-template wrap their
// captions to two to four lines, every card was padded to the longest caption
// so the annotation lines shared a baseline, and the one comparator in the row
// ("SLA is 6 am") was written two blank lines below its two-line caption. In a
// four-up row a comparator sat one blank line below its label because another
// card carried a delta (go-slide-creator-18dqh, journey h-A11).
func TestKPIComparatorSitsUnderItsLabel(t *testing.T) {
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 824 * 12700, Height: 239 * 12700}}
	ctx.Theme.BodyFont = "Calibri"

	six := KPINupValues{
		{Big: "14", Small: "source systems feeding a 2009 warehouse"},
		{Big: "1,900", Small: "hand-written ETL jobs"},
		{Big: "9.5 h", Small: "nightly load window", Comparator: "SLA is 6 am"},
		{Big: "61 / 90", Small: "nights the SLA was missed"},
		{Big: "23%", Small: "product records with a bad attribute"},
		{Big: "60%", Small: "of 4 FTE's time spent on reconciliations"},
	}
	p, _ := Default().Get("kpi-6up")
	grid, err := p.Expand(ctx, &six, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := kpiParagraphsOf(t, grid.Rows[0].Cells[2].Shape.Text)
	if want := []string{"9.5 h", "nightly load window", "SLA is 6 am"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the only comparator of the row must follow its caption directly: got %q, want %q", got, want)
	}

	four := KPINupValues{
		{Big: "14", Small: "Findings raised", Comparator: "3 high / 6 med / 5 low"},
		{Big: "3", Small: "High findings"},
		{Big: "EUR 11.2M", Small: "Op-risk losses 2025", Sub: "+33% YoY"},
		{Big: "30 Jun 2027", Small: "Remediation deadline", Comparator: "9 months from today"},
	}
	p, _ = Default().Get("kpi-4up")
	grid, err = p.Expand(ctx, &four, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range [][]string{
		{"14", "Findings raised", "3 high / 6 med / 5 low"},
		{"3", "High findings", kpiBlankLine},
		{"EUR 11.2M", "Op-risk losses 2025", "+33% YoY"},
		{"30 Jun 2027", "Remediation deadline", "9 months from today"},
	} {
		if got := kpiParagraphsOf(t, grid.Rows[0].Cells[i].Shape.Text); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("cell %d: no card carries both a delta and a comparator, so each annotation takes the line under its caption: got %q, want %q", i, got, want)
		}
	}
}

// Captions that differ by one line still share the annotation baseline; a
// card without an annotation never sets it; a wider spread gives the
// baseline up rather than floating a line under its card.
func TestKPICaptionPadLines(t *testing.T) {
	const font, size = "Arial", 14.0
	width := func(int) float64 { return 100 }
	long := "a caption that certainly wraps to several lines in a narrow card"
	two := "a caption on two lines"
	if n := measuredLines(two, font, false, size, 100); n != 2 {
		t.Fatalf("fixture: %q wraps to %d lines, want 2", two, n)
	}
	if n := measuredLines(long, font, false, size, 100); n < 4 {
		t.Fatalf("fixture: %q wraps to %d lines, want at least 4", long, n)
	}
	for _, tc := range []struct {
		name  string
		cells []KPICell
		want  []int
	}{
		{"one line of difference aligns", []KPICell{{Small: "one", Sub: "+1"}, {Small: two, Comparator: "vs plan"}}, []int{1, 0}},
		{"a bare long caption sets nothing", []KPICell{{Small: long}, {Small: "one", Comparator: "vs plan"}}, []int{0, 0}},
		{"a wide spread gives the baseline up", []KPICell{{Small: long, Sub: "+1"}, {Small: "one", Comparator: "vs plan"}}, []int{0, 0}},
		{"no annotation, no padding", []KPICell{{Small: long}, {Small: "one"}}, []int{0, 0}},
	} {
		got := kpiCaptionPadLines(font, tc.cells, size, width)
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s: pads = %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}
