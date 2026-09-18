package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// fullThemeCtx returns an expand context with a midnight-blue-like theme so
// contrast-aware colour choices resolve deterministically.
func fullThemeCtx() ExpandContext {
	return ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
		Theme: types.ThemeInfo{Colors: []types.ThemeColor{
			{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
			{Name: "dk2", RGB: "#1B2A4A"}, {Name: "lt2", RGB: "#E8ECF1"},
			{Name: "accent1", RGB: "#2E5090"}, {Name: "accent2", RGB: "#D4463A"},
			{Name: "accent3", RGB: "#E8A838"}, {Name: "accent4", RGB: "#43A047"},
			{Name: "accent5", RGB: "#5C6BC0"}, {Name: "accent6", RGB: "#26A69A"},
		}},
	}
}

func execSummaryPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("exec-summary")
	if !ok {
		t.Fatal("exec-summary not registered")
	}
	return p
}

func execPoints(n int) []ExecSummaryPoint {
	pts := make([]ExecSummaryPoint, n)
	for i := range pts {
		pts[i] = ExecSummaryPoint{Lead: "Lead statement number " + string(rune('A'+i)), Support: "Short supporting evidence sentence."}
	}
	return pts
}

// cellText returns the concatenated paragraph content of a shape cell.
func cellText(t *testing.T, raw json.RawMessage) chartInsightsText {
	t.Helper()
	var txt chartInsightsText
	if err := json.Unmarshal(raw, &txt); err != nil {
		t.Fatalf("unmarshal cell text: %v", err)
	}
	return txt
}

func TestExecSummary_Metadata(t *testing.T) {
	p := execSummaryPattern(t)
	if p.Version() != 1 || p.UseWhen() == "" || p.NotWhen() == "" || p.CellsHint() == "" {
		t.Fatalf("metadata incomplete: v=%d", p.Version())
	}
	if !strings.Contains(p.UseWhen(), "scqa-summary") {
		t.Errorf("UseWhen should contrast with scqa-summary: %q", p.UseWhen())
	}
	tax := p.Taxonomy()
	if tax.Category != "narrative" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tax)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) {
		t.Fatalf("schema invalid: %v", err)
	}
	if !strings.Contains(string(data), "2020-12") {
		t.Error("schema missing draft 2020-12 marker")
	}
}

func TestExecSummary_Validate(t *testing.T) {
	p := execSummaryPattern(t)
	for _, n := range []int{3, 4, 5} {
		if err := p.Validate(&ExecSummaryValues{Points: execPoints(n)}, nil, nil); err != nil {
			t.Errorf("n=%d: unexpected error: %v", n, err)
		}
	}

	cases := []struct {
		name    string
		vals    *ExecSummaryValues
		ovr     *ExecSummaryOverrides
		cellOvr map[int]any
		want    string
	}{
		{name: "too few", vals: &ExecSummaryValues{Points: execPoints(2)}, want: "pull-quote"},
		{name: "too many", vals: &ExecSummaryValues{Points: execPoints(6)}, want: "points"},
		{name: "blank lead", vals: &ExecSummaryValues{Points: append(execPoints(2), ExecSummaryPoint{Lead: "  "})}, want: "points[2].lead"},
		{name: "lead too long", vals: &ExecSummaryValues{Points: append(execPoints(2), ExecSummaryPoint{Lead: strings.Repeat("x", execSummaryLeadMax+1)})}, want: "points[2].lead"},
		{name: "support too long", vals: &ExecSummaryValues{Points: append(execPoints(2), ExecSummaryPoint{Lead: "ok", Support: strings.Repeat("x", execSummarySupportMax+1)})}, want: "points[2].support"},
		{name: "bottom line too long", vals: &ExecSummaryValues{Points: execPoints(3), BottomLine: strings.Repeat("x", execSummaryBottomLineMax+1)}, want: "bottom_line"},
		{name: "bad accent mode", vals: &ExecSummaryValues{Points: execPoints(3)}, ovr: &ExecSummaryOverrides{TextOverrides: TextOverrides{CellAccentMode: "rainbow"}}, want: "cell_accent_mode"},
		{name: "cell override out of range", vals: &ExecSummaryValues{Points: execPoints(3)}, cellOvr: map[int]any{3: &ExecSummaryCellOverride{AccentBar: true}}, want: "cell_overrides"},
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

func TestExecSummary_ExpandStructure(t *testing.T) {
	p := execSummaryPattern(t)
	vals := p.(Exemplar).ExemplarValues().(*ExecSummaryValues)
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 4 point rows + 3 rules + bottom line.
	if got := len(grid.Rows); got != 8 {
		t.Fatalf("rows = %d, want 8", got)
	}
	if got := len(grid.Rows[0].Cells); got != 3 {
		t.Errorf("point row cells = %d, want 3 (number, lead, support)", got)
	}
	if grid.Rows[1].Cells[0].ColSpan != 3 {
		t.Errorf("rule row should span all columns, got col_span %d", grid.Rows[1].Cells[0].ColSpan)
	}
	num := cellText(t, grid.Rows[0].Cells[0].Shape.Text)
	if num.Paragraphs[0].Content != "1" || !num.Paragraphs[0].Bold {
		t.Errorf("number cell = %+v, want bold \"1\"", num.Paragraphs[0])
	}
	lead := cellText(t, grid.Rows[0].Cells[1].Shape.Text)
	if !lead.Paragraphs[0].Bold || lead.Paragraphs[0].Size < 12 {
		t.Errorf("lead paragraph must be bold and >= 12pt: %+v", lead.Paragraphs[0])
	}
	support := cellText(t, grid.Rows[0].Cells[2].Shape.Text)
	if support.Paragraphs[0].Size < 12 {
		t.Errorf("support text below 12pt: %v", support.Paragraphs[0].Size)
	}
	bottom := grid.Rows[7].Cells[0]
	if bottom.AccentBar == nil || !strings.Contains(string(bottom.Shape.Fill), "lumMod") {
		t.Errorf("bottom line should be a tinted bar with an accent bar: fill=%s bar=%v", bottom.Shape.Fill, bottom.AccentBar)
	}
	if !strings.Contains(cellText(t, bottom.Shape.Text).Paragraphs[0].Content, "Bottom line:") {
		t.Error("bottom line text missing its label")
	}
}

func TestExecSummary_ContentSizedHeight(t *testing.T) {
	p := execSummaryPattern(t)
	short := &ExecSummaryValues{Points: []ExecSummaryPoint{
		{Lead: "Demand is recovering", Support: "Orders up 12%."},
		{Lead: "Supply is the constraint", Support: "Plants above 95%."},
		{Lead: "Add a third shift", Support: "Costs $3M."},
	}}
	grid, err := p.Expand(fullThemeCtx(), short, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Every row is capped in points (min == max) so the resolver keeps the
	// content-sized block and centres it instead of stretching the rows.
	_, areaH := sizingAreaPt(fullThemeCtx())
	sum := 0.0
	for i, r := range grid.Rows {
		if r.MaxHeight <= 0 || r.MinHeight != r.MaxHeight || r.Height != 0 {
			t.Fatalf("row %d should be point-capped, got %+v", i, r)
		}
		sum += r.MaxHeight
	}
	if sum >= areaH*0.9 {
		t.Errorf("short summary should not fill the content area: rows sum to %.0fpt of %.0fpt", sum, areaH)
	}
	if grid.Bounds != nil {
		t.Errorf("content-sized rows need no bounds cap, got %+v", grid.Bounds)
	}
}

func TestExecSummary_DenseContentStepsDownTypeScale(t *testing.T) {
	p := execSummaryPattern(t)
	long := strings.Repeat("Evidence sentence with several words. ", 6)[:195]
	dense := &ExecSummaryValues{Points: []ExecSummaryPoint{}}
	for i := 0; i < 4; i++ {
		dense.Points = append(dense.Points, ExecSummaryPoint{Lead: strings.Repeat("Lead words ", 8)[:85], Support: long})
	}
	grid, err := p.Expand(fullThemeCtx(), dense, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	lead := cellText(t, grid.Rows[0].Cells[1].Shape.Text).Paragraphs[0]
	if lead.Size >= 17 {
		t.Errorf("dense content should step the lead size below the 17pt default, got %v", lead.Size)
	}
	support := cellText(t, grid.Rows[0].Cells[2].Shape.Text).Paragraphs[0]
	if support.Size < 12 {
		t.Errorf("step-down must not go below 12pt, got %v", support.Size)
	}

	// Explicit sizes pin the scale.
	pinned, _ := p.Expand(fullThemeCtx(), dense, &ExecSummaryOverrides{TextOverrides: TextOverrides{HeaderSize: 20, BodySize: 15}}, nil)
	if got := cellText(t, pinned.Rows[0].Cells[1].Shape.Text).Paragraphs[0].Size; got != 20 {
		t.Errorf("header_size override should pin lead size, got %v", got)
	}
}

func TestExecSummary_OverridesAndAccentModes(t *testing.T) {
	p := execSummaryPattern(t)
	vals := &ExecSummaryValues{Points: execPoints(3)}
	no := false
	grid, err := p.Expand(fullThemeCtx(), vals, &ExecSummaryOverrides{Numbered: &no}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(grid.Rows[0].Cells); got != 2 {
		t.Errorf("numbered=false should drop the number column, got %d cells", got)
	}

	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{"uniform", "alternate", "progressive"} {
			ovr := &ExecSummaryOverrides{TextOverrides: TextOverrides{Accent: base, CellAccentMode: mode}}
			g, err := p.Expand(ExpandContext{}, vals, ovr, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				row := g.Rows[i*2] // rules sit between point rows
				got := cellText(t, row.Cells[0].Shape.Text).Paragraphs[0].Color
				if want := ResolveCellAccent(base, i, mode); got != want {
					t.Errorf("%s/%s point %d number colour = %s, want %s", base, mode, i, got, want)
				}
			}
		}
	}

	g, _ := p.Expand(fullThemeCtx(), vals, nil, map[int]any{1: &ExecSummaryCellOverride{AccentBar: true}})
	if g.Rows[2].Cells[1].AccentBar == nil {
		t.Error("cell_overrides accent_bar should decorate the lead cell of point 1")
	}
}

func TestExecSummary_LowContrastAccentFallsBackToDark(t *testing.T) {
	ctx := fullThemeCtx()
	for i := range ctx.Theme.Colors {
		if ctx.Theme.Colors[i].Name == "accent6" {
			ctx.Theme.Colors[i].RGB = "#F5F5F0"
		}
	}
	p := execSummaryPattern(t)
	g, err := p.Expand(ctx, &ExecSummaryValues{Points: execPoints(3)}, &ExecSummaryOverrides{TextOverrides: TextOverrides{Accent: "accent6"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := cellText(t, g.Rows[0].Cells[0].Shape.Text).Paragraphs[0].Color; got != "dk1" {
		t.Errorf("near-white accent numbers should fall back to dk1, got %s", got)
	}
}

func TestExecSummary_Recommend(t *testing.T) {
	res := Recommend(Default(), "key messages for the board", &ContentHints{ItemCount: 4}, 3)
	if len(res.Candidates) == 0 || res.Candidates[0].PatternName != "exec-summary" {
		t.Errorf("key messages intent should rank exec-summary first, got %+v", res.Candidates)
	}
	res = Recommend(Default(), "SCQA executive summary", nil, 3)
	if len(res.Candidates) == 0 || res.Candidates[0].PatternName != "scqa-summary" {
		t.Errorf("SCQA intent must stay on scqa-summary, got %+v", res.Candidates)
	}
}
