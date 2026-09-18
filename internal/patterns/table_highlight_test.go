package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func tableHighlightPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("table-highlight")
	if !ok {
		t.Fatal("table-highlight not registered")
	}
	return p
}

func decodeTableHighlight(t *testing.T, raw string) *TableHighlightValues {
	t.Helper()
	v := &TableHighlightValues{}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		t.Fatalf("unmarshal values: %v", err)
	}
	return v
}

func TestTableHighlight_Metadata(t *testing.T) {
	p := tableHighlightPattern(t)
	if p.Version() != 1 || p.UseWhen() == "" || p.NotWhen() == "" || p.CellsHint() == "" {
		t.Fatal("metadata incomplete")
	}
	tax := p.Taxonomy()
	if tax.Category == "" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 || len(tax.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tax)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), "2020-12") {
		t.Fatalf("schema invalid: %v", err)
	}
}

func TestTableHighlight_DecodeFlexibleForms(t *testing.T) {
	v := decodeTableHighlight(t, `{
		"criteria": ["Cost", {"label": "Status", "scale": "rag"}, {"label": "Go-live", "scale": "text"}],
		"options": [
			{"name": "A", "scores": [4, "green", "Q1"]},
			{"name": "B", "scores": ["half", "a", "-"]}
		]
	}`)
	if v.Criteria[0].Label != "Cost" || v.Criteria[1].Scale != "rag" {
		t.Errorf("criteria decode: %+v", v.Criteria)
	}
	if v.Options[0].Scores[0] != "4" || v.Options[1].Scores[0] != "half" {
		t.Errorf("scores decode: %+v", v.Options)
	}
	if err := tableHighlightPattern(t).Validate(v, nil, nil); err != nil {
		t.Errorf("mixed-scale matrix should validate: %v", err)
	}
}

func TestTableHighlight_ParseScore(t *testing.T) {
	cases := []struct {
		raw   string
		scale string
		kind  string
		level int
		rag   string
		ok    bool
	}{
		{"0", thScaleHarvey, thScaleHarvey, 0, "", true},
		{"4", thScaleHarvey, thScaleHarvey, 4, "", true},
		{"three-quarter", thScaleHarvey, thScaleHarvey, 3, "", true},
		{"5", thScaleHarvey, "", 0, "", false},
		{"2.5", thScaleHarvey, "", 0, "", false},
		{"G", thScaleRAG, thScaleRAG, 0, "green", true},
		{"amber", thScaleRAG, thScaleRAG, 0, "amber", true},
		{"blue", thScaleRAG, "", 0, "", false},
		{"n/a", thScaleRAG, "na", 0, "", true},
		{"-", thScaleHarvey, "na", 0, "", true},
		{"$2.1M", thScaleText, thScaleText, 0, "", true},
		{strings.Repeat("x", thTextCellMax+1), thScaleText, "", 0, "", false},
	}
	for _, tc := range cases {
		got, ok := parseTableHighlightScore(TableHighlightScore(tc.raw), tc.scale)
		if ok != tc.ok || (ok && (got.kind != tc.kind || got.level != tc.level || got.rag != tc.rag)) {
			t.Errorf("parse(%q, %s) = %+v ok=%v", tc.raw, tc.scale, got, ok)
		}
	}
}

func TestTableHighlight_Validate(t *testing.T) {
	p := tableHighlightPattern(t)
	if err := p.Validate(p.(Exemplar).ExemplarValues(), nil, nil); err != nil {
		t.Fatalf("exemplar must validate: %v", err)
	}
	five, nine := 5, 9
	cases := []struct {
		name string
		raw  string
		ovr  *TableHighlightOverrides
		co   map[int]any
		want string
	}{
		{name: "one criterion", raw: `{"criteria":["A"],"options":[{"name":"x","scores":[1]},{"name":"y","scores":[2]}]}`, want: "horizontal-bar-with-callouts"},
		{name: "one option", raw: `{"criteria":["A","B"],"options":[{"name":"x","scores":[1,2]}]}`, want: "options"},
		{name: "seven criteria", raw: `{"criteria":["a","b","c","d","e","f","g"],"options":[{"name":"x","scores":[1,1,1,1,1,1,1]},{"name":"y","scores":[1,1,1,1,1,1,1]}]}`, want: "criteria"},
		{name: "score count mismatch", raw: `{"criteria":["A","B"],"options":[{"name":"x","scores":[1]},{"name":"y","scores":[2,3]}]}`, want: "options[0].scores"},
		{name: "bad harvey", raw: `{"criteria":["A","B"],"options":[{"name":"x","scores":[1,7]},{"name":"y","scores":[2,3]}]}`, want: "options[0].scores[1]"},
		{name: "bad rag", raw: `{"scale":"rag","criteria":["A","B"],"options":[{"name":"x","scores":["g","blue"]},{"name":"y","scores":["r","a"]}]}`, want: "options[0].scores[1]"},
		{name: "blank name", raw: `{"criteria":["A","B"],"options":[{"name":" ","scores":[1,2]},{"name":"y","scores":[2,3]}]}`, want: "options[0].name"},
		{name: "unknown scale", raw: `{"scale":"stars","criteria":["A","B"],"options":[{"name":"x","scores":[1,2]},{"name":"y","scores":[2,3]}]}`, want: "scale"},
		{name: "legend labels count", raw: `{"legend_labels":["a"],"criteria":["A","B"],"options":[{"name":"x","scores":[1,2]},{"name":"y","scores":[2,3]}]}`, want: "legend_labels"},
		{name: "bad rag colour key", raw: `{"criteria":["A","B"],"options":[{"name":"x","scores":[1,2]},{"name":"y","scores":[2,3]}]}`, ovr: &TableHighlightOverrides{RAGColors: map[string]string{"blue": "#0000FF"}}, want: "rag_colors"},
		{name: "bad rag colour value", raw: `{"criteria":["A","B"],"options":[{"name":"x","scores":[1,2]},{"name":"y","scores":[2,3]}]}`, ovr: &TableHighlightOverrides{RAGColors: map[string]string{"red": "crimson-ish"}}, want: "rag_colors.red"},
		{name: "cell override out of range", raw: `{"criteria":["A","B"],"options":[{"name":"x","scores":[1,2]},{"name":"y","scores":[2,3]}]}`, co: map[int]any{4: &TableHighlightCellOverride{AccentBar: true}}, want: "cell_overrides"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ovr any
			if tc.ovr != nil {
				ovr = tc.ovr
			}
			err := p.Validate(decodeTableHighlight(t, tc.raw), ovr, tc.co)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want mention of %q", err, tc.want)
			}
		})
	}

	v := decodeTableHighlight(t, `{"criteria":["A","B"],"options":[{"name":"x","scores":[1,2]},{"name":"y","scores":[2,3]}]}`)
	v.HighlightRow = &five
	v.HighlightCol = &nine
	err := p.Validate(v, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "highlight_row") || !strings.Contains(err.Error(), "highlight_col") {
		t.Errorf("out-of-range highlights should be rejected, got %v", err)
	}
}

func TestTableHighlight_ExpandStructure(t *testing.T) {
	p := tableHighlightPattern(t)
	v := p.(Exemplar).ExemplarValues().(*TableHighlightValues)
	grid, err := p.Expand(fullThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// header + 3 options + legend.
	if len(grid.Rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(grid.Rows))
	}
	if got := len(grid.Rows[0].Cells); got != 5 {
		t.Errorf("header cells = %d, want 5", got)
	}
	if grid.Bounds == nil || grid.Bounds.Height >= 99.5 {
		t.Errorf("table should be content-sized, bounds = %+v", grid.Bounds)
	}

	hdr := cellText(t, grid.Rows[0].Cells[0].Shape.Text).Paragraphs[0]
	if hdr.Content != "Option" || hdr.Color != "lt1" || hdr.Size < 12 {
		t.Errorf("corner header = %+v", hdr)
	}

	// Highlighted row (option 1): tinted fill, accent bar, tag paragraph.
	hl := grid.Rows[2].Cells[0]
	if hl.AccentBar == nil || !strings.Contains(string(hl.Shape.Fill), "lumMod") {
		t.Errorf("highlight row name cell should be tinted with an accent bar: %s %+v", hl.Shape.Fill, hl.AccentBar)
	}
	paras := cellText(t, hl.Shape.Text).Paragraphs
	if len(paras) != 3 || paras[2].Content != "Recommended" {
		t.Errorf("highlight paragraphs = %+v", paras)
	}
	for _, c := range grid.Rows[2].Cells[1:] {
		if string(c.Shape.Fill) != string(hl.Shape.Fill) {
			t.Errorf("whole highlighted row should share the tint, got %s vs %s", c.Shape.Fill, hl.Shape.Fill)
		}
	}

	// Score cells carry a centred SVG Harvey ball with alt text.
	sc := grid.Rows[1].Cells[1]
	if sc.Icon == nil || !strings.HasPrefix(sc.Icon.SVGData, "<svg") || !strings.Contains(sc.Icon.Alt, "Harvey ball 1 of 4") {
		t.Fatalf("score cell icon = %+v", sc.Icon)
	}
	if !strings.Contains(sc.Icon.SVGData, "#1B2A4A") {
		t.Errorf("Harvey ball should use the resolved dk2 ink: %s", sc.Icon.SVGData)
	}
	if sc.Icon.Position != "center" || sc.Icon.Scale <= 0 || sc.Icon.Scale > 1 {
		t.Errorf("icon placement = %+v", sc.Icon)
	}

	// Legend spans the table and nests a grid of symbol/label pairs.
	leg := grid.Rows[4].Cells[0]
	if leg.ColSpan != 5 || leg.Grid == nil || len(leg.Grid.Rows[0].Cells) < 6 {
		t.Errorf("legend cell = %+v", leg)
	}
}

func TestTableHighlight_HarveySVGLevels(t *testing.T) {
	ctx := fullThemeCtx()
	for lvl := 0; lvl <= 4; lvl++ {
		svg, alt := thSymbolSVG(ctx, thNormalizedScore{kind: thScaleHarvey, level: lvl}, "dk2", nil)
		paths := strings.Count(svg, "<path")
		wantPaths := 0
		if lvl >= 1 && lvl <= 3 {
			wantPaths = 1
		}
		if paths != wantPaths {
			t.Errorf("level %d: %d wedge paths, want %d", lvl, paths, wantPaths)
		}
		if lvl == 4 && strings.Count(svg, "<circle") != 2 {
			t.Errorf("level 4 should add a filled disc: %s", svg)
		}
		if !strings.Contains(alt, "of 4") {
			t.Errorf("alt = %q", alt)
		}
	}
	svg, alt := thSymbolSVG(ctx, thNormalizedScore{kind: thScaleRAG, rag: "red"}, "dk2", map[string]string{"red": "accent2"})
	if !strings.Contains(svg, "#D4463A") || alt != "RAG status: red" {
		t.Errorf("RAG override should resolve accent2: %s %s", svg, alt)
	}
	svg, _ = thSymbolSVG(ExpandContext{}, thNormalizedScore{kind: thScaleRAG, rag: "green"}, "dk2", nil)
	if !strings.Contains(svg, thDefaultRAG["green"]) {
		t.Errorf("default RAG green missing: %s", svg)
	}
}

func TestTableHighlight_HighlightColumnAndTextCells(t *testing.T) {
	p := tableHighlightPattern(t)
	v := decodeTableHighlight(t, `{
		"criteria": ["Fit", {"label": "Go-live", "scale": "text"}, {"label": "Risk", "scale": "rag"}],
		"options": [{"name": "A", "scores": [4, "Q1 FY26", "g"]}, {"name": "B", "scores": [2, "-", "r"]}],
		"highlight_col": 0, "show_legend": false
	}`)
	grid, err := p.Expand(fullThemeCtx(), v, &TableHighlightOverrides{Accent: "accent2"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 3 {
		t.Fatalf("show_legend=false should drop the legend row, got %d rows", len(grid.Rows))
	}
	if got := string(grid.Rows[0].Cells[1].Shape.Fill); got != `"accent2"` {
		t.Errorf("highlighted column header fill = %s, want accent2", got)
	}
	if got := string(grid.Rows[1].Cells[1].Shape.Fill); !strings.Contains(got, "accent2") {
		t.Errorf("highlighted column body cell should be accent-tinted, got %s", got)
	}
	txt := cellText(t, grid.Rows[1].Cells[2].Shape.Text).Paragraphs[0]
	if txt.Content != "Q1 FY26" || txt.Size < 12 {
		t.Errorf("text cell = %+v", txt)
	}
	if na := cellText(t, grid.Rows[2].Cells[2].Shape.Text).Paragraphs[0].Content; na != "–" {
		t.Errorf("n/a cell should render an en dash, got %q", na)
	}
	if grid.Rows[1].Cells[3].Icon == nil || !strings.Contains(grid.Rows[1].Cells[3].Icon.SVGData, thDefaultRAG["green"]) {
		t.Error("rag column should render a green dot")
	}
}

func TestTableHighlight_DenseStepsDownButStaysReadable(t *testing.T) {
	p := tableHighlightPattern(t)
	opts := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		opts = append(opts, `{"name":"Vendor option with a longer name","detail":"A one-line descriptor for the vendor","scores":[1,2,3,4,0,2]}`)
	}
	v := decodeTableHighlight(t, `{"criteria":["Functional fit","Total cost of ownership","Security posture","Integration effort","Vendor viability","Support"],"options":[`+strings.Join(opts, ",")+`]}`)
	grid, err := p.Expand(fullThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	name := cellText(t, grid.Rows[1].Cells[0].Shape.Text).Paragraphs[0]
	if name.Size >= 14 || name.Size < 12 {
		t.Errorf("dense table should step the body size down but not below 12pt, got %v", name.Size)
	}
	pinned, _ := p.Expand(fullThemeCtx(), v, &TableHighlightOverrides{BodySize: 16}, nil)
	if got := cellText(t, pinned.Rows[1].Cells[0].Shape.Text).Paragraphs[0].Size; got != 16 {
		t.Errorf("body_size override should pin the size, got %v", got)
	}
}

func TestTableHighlight_CellOverrideAccentBar(t *testing.T) {
	p := tableHighlightPattern(t)
	v := decodeTableHighlight(t, `{"criteria":["A","B"],"options":[{"name":"x","scores":[1,2]},{"name":"y","scores":[2,3]}]}`)
	grid, err := p.Expand(fullThemeCtx(), v, nil, map[int]any{0: &TableHighlightCellOverride{AccentBar: true}})
	if err != nil {
		t.Fatal(err)
	}
	if grid.Rows[1].Cells[0].AccentBar == nil {
		t.Error("cell_overrides[0].accent_bar should decorate option 0")
	}
	if grid.Rows[2].Cells[0].AccentBar != nil {
		t.Error("option 1 should stay undecorated")
	}
}

func TestTableHighlight_Recommend(t *testing.T) {
	res := Recommend(Default(), "evaluate vendor options against criteria with harvey balls", &ContentHints{ItemCount: 4}, 3)
	if len(res.Candidates) == 0 || res.Candidates[0].PatternName != "table-highlight" {
		t.Errorf("evaluation-matrix intent should rank table-highlight first, got %+v", res.Candidates)
	}
	res = Recommend(Default(), "compare two options", &ContentHints{Columns: 2}, 3)
	if len(res.Candidates) > 0 && res.Candidates[0].PatternName == "table-highlight" {
		t.Error("two-option prose comparison should not route to table-highlight")
	}
}
