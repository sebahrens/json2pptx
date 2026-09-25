package patterns

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func textSidebarPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("text-sidebar")
	if !ok {
		t.Fatal("text-sidebar not registered")
	}
	return p
}

func TestTextSidebar_Metadata(t *testing.T) {
	p := textSidebarPattern(t)
	if p.Version() != 1 || p.UseWhen() == "" || p.NotWhen() == "" || p.CellsHint() == "" || p.Description() == "" {
		t.Fatal("metadata incomplete")
	}
	for _, sibling := range []string{"pull-quote", "image-text-split", "exec-summary"} {
		if !strings.Contains(p.UseWhen(), sibling) || !strings.Contains(p.NotWhen(), sibling) {
			t.Errorf("use_when / not_when must contrast with %s", sibling)
		}
	}
	tax := p.Taxonomy()
	if tax.Category != "narrative" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 || len(tax.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tax)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), "2020-12") {
		t.Fatalf("schema invalid: %v", err)
	}
}

func TestTextSidebar_Validate(t *testing.T) {
	p := textSidebarPattern(t)
	if err := p.Validate(p.(Exemplar).ExemplarValues(), nil, nil); err != nil {
		t.Fatalf("exemplar must validate: %v", err)
	}
	// Budgets count characters, not bytes.
	multi := &TextSidebarValues{Paragraphs: []string{strings.Repeat("ü", tsParagraphMax)}, Sidebar: strings.Repeat("€", tsSidebarMax)}
	if err := p.Validate(multi, nil, nil); err != nil {
		t.Errorf("multi-byte values inside the budget must validate: %v", err)
	}
	long := func(n int) string { return strings.Repeat("x", n) }
	ok := func() *TextSidebarValues { return &TextSidebarValues{Paragraphs: []string{"p"}, Sidebar: "s"} }
	cases := []struct {
		name string
		vals *TextSidebarValues
		ovr  *TextSidebarOverrides
		co   map[int]any
		want string
	}{
		{name: "no paragraphs", vals: &TextSidebarValues{Sidebar: "s"}, want: "paragraphs"},
		{name: "too many paragraphs", vals: &TextSidebarValues{Paragraphs: []string{"a", "b", "c", "d", "e"}, Sidebar: "s"}, want: "paragraphs"},
		{name: "blank paragraph", vals: &TextSidebarValues{Paragraphs: []string{"a", " "}, Sidebar: "s"}, want: "paragraphs[1]"},
		{name: "paragraph too long", vals: &TextSidebarValues{Paragraphs: []string{long(tsParagraphMax + 1)}, Sidebar: "s"}, want: "paragraphs[0]"},
		{name: "heading too long", vals: &TextSidebarValues{Heading: long(tsHeadingMax + 1), Paragraphs: []string{"p"}, Sidebar: "s"}, want: "heading"},
		{name: "too many bullets", vals: &TextSidebarValues{Paragraphs: []string{"p"}, Bullets: []string{"1", "2", "3", "4", "5", "6", "7"}, Sidebar: "s"}, want: "bullets"},
		{name: "bullet too long", vals: &TextSidebarValues{Paragraphs: []string{"p"}, Bullets: []string{long(tsBulletMax + 1)}, Sidebar: "s"}, want: "bullets[0]"},
		{name: "missing sidebar", vals: &TextSidebarValues{Paragraphs: []string{"p"}}, want: "sidebar"},
		{name: "sidebar too long", vals: &TextSidebarValues{Paragraphs: []string{"p"}, Sidebar: long(tsSidebarMax + 1)}, want: "sidebar"},
		{name: "bad side", vals: ok(), ovr: &TextSidebarOverrides{SidebarSide: "top"}, want: "sidebar_side"},
		{name: "bad style", vals: ok(), ovr: &TextSidebarOverrides{SidebarStyle: "outline"}, want: "sidebar_style"},
		{name: "bad width", vals: ok(), ovr: &TextSidebarOverrides{SidebarWidthPct: 60}, want: "sidebar_width_pct"},
		{name: "bad body size", vals: ok(), ovr: &TextSidebarOverrides{BodySize: 9}, want: "body_size"},
		{name: "bad sidebar size", vals: ok(), ovr: &TextSidebarOverrides{SidebarSize: 60}, want: "sidebar_size"},
		{name: "cell overrides rejected", vals: ok(), co: map[int]any{0: &CellOverride{}}, want: "cell_overrides"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ovr any
			if tc.ovr != nil {
				ovr = tc.ovr
			}
			err := p.Validate(tc.vals, ovr, tc.co)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want mention of %q", err, tc.want)
			}
		})
	}
}

func TestTextSidebar_ExpandDefault(t *testing.T) {
	p := textSidebarPattern(t)
	vals := p.(Exemplar).ExemplarValues().(*TextSidebarValues)
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 2 || cols[1] != tsDefaultSidePct {
		t.Fatalf("sidebar should sit right at %v%%, got %s", tsDefaultSidePct, grid.Columns)
	}
	row := grid.Rows[0]
	_, areaH := sizingAreaPt(fullThemeCtx())
	if row.MaxHeight <= 0 || row.MinHeight != row.MaxHeight || row.MaxHeight > areaH || row.MaxHeight < areaH*tsMinFillPct/100-0.1 {
		t.Errorf("row should be point-capped between %v%% and 100%% of the area, got %v (area %v)", tsMinFillPct, row.MaxHeight, areaH)
	}
	main := cellText(t, row.Cells[0].Shape.Text).Paragraphs
	if main[0].Content != vals.Heading || !main[0].Bold {
		t.Errorf("first paragraph should be the bold heading, got %+v", main[0])
	}
	// The exemplar's second paragraph ends with a colon, so the bullets follow it.
	if !strings.HasPrefix(main[3].Content, "• ") || strings.HasPrefix(main[2].Content, "• ") {
		t.Errorf("bullets should follow the paragraph that introduces them, got %q / %q", main[2].Content, main[3].Content)
	}
	if !strings.Contains(main[1].Content, "<b>") {
		t.Errorf("markdown emphasis should convert, got %q", main[1].Content)
	}
	for _, para := range main {
		if para.Size < 12 {
			t.Errorf("paragraph below 12pt: %+v", para)
		}
	}
	side := row.Cells[1]
	if side.AccentBar == nil || side.AccentBar.Position != "top" {
		t.Errorf("tinted sidebar carries a top accent bar: %+v", side.AccentBar)
	}
	st := cellText(t, side.Shape.Text).Paragraphs[0]
	if !st.Bold || st.Size < 16 || st.Content != vals.Sidebar {
		t.Errorf("sidebar should be one large bold message, got %+v", st)
	}
	if st.Color == "lt1" {
		t.Errorf("text on the pale tinted panel must be dark, got %q", st.Color)
	}
}

func TestTextSidebar_FilledLeft(t *testing.T) {
	p := textSidebarPattern(t)
	vals := &TextSidebarValues{Paragraphs: []string{"Short paragraph."}, Sidebar: "One key message"}
	grid, err := p.Expand(fullThemeCtx(), vals, &TextSidebarOverrides{SidebarSide: "left", SidebarStyle: "filled", SidebarWidthPct: 36, Accent: "accent1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || cols[0] != 36 {
		t.Fatalf("sidebar should be the left 36%% column, got %s", grid.Columns)
	}
	side := grid.Rows[0].Cells[0]
	if string(side.Shape.Fill) != `"accent1"` || side.AccentBar != nil {
		t.Errorf("filled sidebar is the solid accent with no bar, got fill=%s bar=%+v", side.Shape.Fill, side.AccentBar)
	}
	if ink := cellText(t, side.Shape.Text).Paragraphs[0].Color; ink != "lt1" {
		t.Errorf("light text is measured to read on the dark accent1 fill, got %q", ink)
	}
	main := cellText(t, grid.Rows[0].Cells[1].Shape.Text)
	if main.VerticalAlign != "ctr" || main.Paragraphs[0].Size != tsSparseBodySize {
		t.Errorf("short copy is promoted and centred, got %v / %+v", main.VerticalAlign, main.Paragraphs[0])
	}
}

func TestTextSidebar_WarnsWhenTooLong(t *testing.T) {
	p := textSidebarPattern(t).(PostExpandWarner)
	long := strings.Repeat("Word after word of dense operating detail. ", 12)[:tsParagraphMax]
	vals := &TextSidebarValues{
		Heading:    "Introduction",
		Paragraphs: []string{long, long, long, long},
		Bullets:    []string{strings.Repeat("b", tsBulletMax), strings.Repeat("b", tsBulletMax), strings.Repeat("b", tsBulletMax)},
		Sidebar:    "Key message",
	}
	warnings := p.PostExpandWarnings(fullThemeCtx(), vals, nil)
	if len(warnings) == 0 || !strings.HasPrefix(warnings[0], ErrCodeBodyTooLong+": text-sidebar paragraphs") {
		t.Fatalf("an over-long main column should report BODY_TOO_LONG, got %v", warnings)
	}
	if got := p.PostExpandWarnings(fullThemeCtx(), textSidebarPattern(t).(Exemplar).ExemplarValues(), nil); len(got) != 0 {
		t.Errorf("the exemplar should fit without warnings, got %v", got)
	}
}

func TestTextSidebar_Golden(t *testing.T) {
	p := textSidebarPattern(t)
	grid, err := p.Expand(fullThemeCtx(), p.(Exemplar).ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "text-sidebar", "default.golden.json"))
}

func TestTextSidebar_Recommend(t *testing.T) {
	for _, intent := range []string{"introduction", "intro page", "foreword", "key message sidebar"} {
		res := Recommend(Default(), intent, nil, 3)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != "text-sidebar" {
			t.Errorf("%q should rank text-sidebar first, got %+v", intent, res.Candidates)
		}
	}
}
