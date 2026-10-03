package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// kpiCellText is a KPI cell's text object as the pattern writes it.
type kpiCellText struct {
	Paragraphs []struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color"`
	} `json:"paragraphs"`
	VerticalAlign string   `json:"vertical_align"`
	InsetTop      *float64 `json:"inset_top"`
}

func kpiCellTextOf(t *testing.T, cell *jsonschema.GridCellInput) kpiCellText {
	t.Helper()
	var txt kpiCellText
	if err := json.Unmarshal(cell.Shape.Text, &txt); err != nil {
		t.Fatalf("cell text: %v", err)
	}
	return txt
}

func plainKPIs(n int) *KPINupValues {
	all := KPINupValues{
		{Big: "$4.2M", Small: "ARR"}, {Big: "127%", Small: "NRR"}, {Big: "12d", Small: "Sales cycle"},
		{Big: "98%", Small: "CSAT"}, {Big: "42", Small: "NPS"}, {Big: "3.2x", Small: "LTV/CAC"},
	}
	v := all[:n]
	return &v
}

func kpiThemeCtx() ExpandContext {
	ctx := fullThemeCtx()
	ctx.LayoutBounds = kpiTestCtx().LayoutBounds
	return ctx
}

func isOpenKPICell(c *jsonschema.GridCellInput) bool {
	return string(c.Shape.Fill) == `"none"` && string(c.Shape.Line) == `"none"`
}

// go-slide-creator-8zles: plain value + caption cells are set on the canvas
// between neutral hairline dividers, on every kpi-Nup and on kpi-inline.
func TestKPIOpenStripIsDefaultForPlainCells(t *testing.T) {
	ctx := kpiThemeCtx()
	names := []string{"kpi-2up", "kpi-3up", "kpi-4up", "kpi-5up", "kpi-6up", "kpi-inline"}
	for i, name := range names {
		n := min(i+2, 6)
		grid := expandFor(t, name, ctx, plainKPIs(n), nil)
		if len(grid.Rows) != 1 || len(grid.Rows[0].Cells) != n {
			t.Fatalf("%s: want one row of %d cells, got %d rows", name, n, len(grid.Rows))
		}
		for ci, c := range grid.Rows[0].Cells {
			if !isOpenKPICell(c) {
				t.Errorf("%s cell %d: fill %s line %s, want an unpainted cell", name, ci, c.Shape.Fill, c.Shape.Line)
			}
			switch {
			case ci == 0 && c.AccentBar != nil:
				t.Errorf("%s: the first cell has no divider, got %+v", name, c.AccentBar)
			case ci > 0 && (c.AccentBar == nil || c.AccentBar.Position != "left" || c.AccentBar.Width != kpiDividerPt):
				t.Errorf("%s cell %d: divider = %+v, want a %.2fpt left hairline", name, ci, c.AccentBar, kpiDividerPt)
			case ci > 0 && (isAccentSlot(c.AccentBar.Color) || c.AccentBar.Color != "#B3B3B3"):
				t.Errorf("%s cell %d: divider colour %q, want the neutral dk1 30%% step", name, ci, c.AccentBar.Color)
			}
			txt := kpiCellTextOf(t, c)
			if txt.Paragraphs[0].Color != "accent1" || txt.Paragraphs[1].Color != "dk1" {
				t.Errorf("%s cell %d: value/caption ink = %s/%s, want accent1/dk1", name, ci, txt.Paragraphs[0].Color, txt.Paragraphs[1].Color)
			}
		}
	}
}

// Cells with an icon or a semantic / per-cell accent keep their container,
// and overrides.style picks either look explicitly.
func TestKPIContainerKeptForIconsAndSemanticAccents(t *testing.T) {
	ctx := kpiThemeCtx()
	icons := plainKPIs(3)
	(*icons)[1].Icon = &IconRef{Name: "clock"}

	for _, tc := range []struct {
		name     string
		vals     *KPINupValues
		ovr      *KPIOverrides
		wantOpen bool
	}{
		{"plain", plainKPIs(3), nil, true},
		{"icon", icons, nil, false},
		{"semantic accent", plainKPIs(3), &KPIOverrides{SemanticAccent: "positive"}, false},
		{"progressive accents", plainKPIs(3), &KPIOverrides{CellAccentMode: "progressive"}, false},
		{"uniform accent mode", plainKPIs(3), &KPIOverrides{CellAccentMode: "uniform"}, true},
		{"explicit accent", plainKPIs(3), &KPIOverrides{Accent: "accent3"}, true},
		{"style tiles", plainKPIs(3), &KPIOverrides{Style: "tiles"}, false},
		{"style open with an icon", icons, &KPIOverrides{Style: "open"}, true},
	} {
		var ovr any
		if tc.ovr != nil {
			ovr = tc.ovr
		}
		grid := expandFor(t, "kpi-3up", ctx, tc.vals, ovr)
		for ci, c := range grid.Rows[0].Cells {
			if got := isOpenKPICell(c); got != tc.wantOpen {
				t.Errorf("%s: cell %d open = %v, want %v (fill %s)", tc.name, ci, got, tc.wantOpen, c.Shape.Fill)
			}
		}
	}

	// kpi-inline: an icon keeps the tinted cell under its accent rule.
	grid := expandFor(t, "kpi-inline", ctx, icons, nil)
	for ci, c := range grid.Rows[0].Cells {
		if isOpenKPICell(c) || c.AccentBar == nil || c.AccentBar.Position != "top" {
			t.Errorf("kpi-inline with an icon: cell %d is not a tinted cell under a top rule (fill %s, bar %+v)", ci, c.Shape.Fill, c.AccentBar)
		}
	}
}

func TestKPIStyleIsValidated(t *testing.T) {
	nup, _ := Default().Get("kpi-3up")
	for style, ok := range map[string]bool{"": true, "open": true, "tiles": true, "tinted": false, "solid": false, "cards": false} {
		err := nup.Validate(plainKPIs(3), &KPIOverrides{Style: style}, nil)
		if (err == nil) != ok {
			t.Errorf("kpi-3up style %q: err = %v, want ok=%v", style, err, ok)
		}
	}
	inline, _ := Default().Get("kpi-inline")
	for style, ok := range map[string]bool{"": true, "open": true, "tinted": true, "solid": true, "tiles": false} {
		err := inline.Validate(plainKPIs(3), &KPIInlineOverrides{Style: style}, nil)
		if (err == nil) != ok {
			t.Errorf("kpi-inline style %q: err = %v, want ok=%v", style, err, ok)
		}
	}
	for _, name := range []string{"kpi-3up", "kpi-inline"} {
		p, _ := Default().Get(name)
		style := p.Schema().raw.Properties["overrides"].raw.Properties["style"]
		if style == nil || len(style.raw.Enum) == 0 {
			t.Errorf("%s: overrides.style is missing from the schema", name)
		}
	}
}

// go-slide-creator-0cy3p: every cell of a KPI row anchors its text to the top
// of the same box at one value size, so the values share a baseline whatever
// the caption line counts; cells with a shorter caption carry blank caption
// lines so the delta and comparator lines share theirs too.
func TestKPIValuesShareOneBaseline(t *testing.T) {
	ctx := kpiThemeCtx()
	mixed := &KPINupValues{
		{Big: "$4.2M", Small: "ARR"},
		{Big: "127%", Small: "Net revenue retention across enterprise"},
		{Big: "12d", Small: "Sales cycle"},
		{Big: "98%", Small: "CSAT"},
	}
	withLines := &KPINupValues{
		{Big: "$4.2M", Small: "ARR", Sub: "+12%", Comparator: "vs plan +4 pts"},
		{Big: "127%", Small: "Net revenue retention across enterprise", Comparator: "vs PY +2 pts"},
		{Big: "12d", Small: "Sales cycle", Sub: "-3d"},
		{Big: "98%", Small: "CSAT"},
	}
	for _, tc := range []struct {
		name, pattern string
		vals          *KPINupValues
		ovr           any
	}{
		{"open", "kpi-4up", mixed, nil},
		{"tiles", "kpi-4up", mixed, &KPIOverrides{Style: "tiles"}},
		{"open with delta and comparator", "kpi-4up", withLines, nil},
		{"tiles with delta and comparator", "kpi-4up", withLines, &KPIOverrides{Style: "tiles"}},
		{"inline", "kpi-inline", mixed, nil},
		{"inline tinted", "kpi-inline", mixed, &KPIInlineOverrides{Style: "tinted"}},
	} {
		grid := expandFor(t, tc.pattern, ctx, tc.vals, tc.ovr)
		cells := grid.Rows[0].Cells
		first := kpiCellTextOf(t, cells[0])
		for ci, c := range cells {
			txt := kpiCellTextOf(t, c)
			if txt.VerticalAlign != "t" {
				t.Errorf("%s: cell %d anchors its text %q, want the top", tc.name, ci, txt.VerticalAlign)
			}
			if txt.Paragraphs[0].Size != first.Paragraphs[0].Size || txt.Paragraphs[1].Size != first.Paragraphs[1].Size {
				t.Errorf("%s: cell %d sizes %v/%v differ from cell 0's %v/%v", tc.name, ci,
					txt.Paragraphs[0].Size, txt.Paragraphs[1].Size, first.Paragraphs[0].Size, first.Paragraphs[1].Size)
			}
			if (txt.InsetTop == nil) != (first.InsetTop == nil) {
				t.Errorf("%s: cell %d text margin differs from cell 0's", tc.name, ci)
			}
		}
		if tc.vals != withLines {
			continue
		}
		// Value, caption (2 lines in cell 1, 1 line + 1 blank elsewhere),
		// delta, comparator: the comparator is the same line in every cell.
		font := ctx.Theme.BodyFont
		for ci, c := range cells {
			txt := kpiCellTextOf(t, c)
			lines := 0
			for _, p := range txt.Paragraphs[1 : len(txt.Paragraphs)-2] {
				if strings.TrimSpace(strings.ReplaceAll(p.Content, kpiBlankLine, "")) == "" {
					lines++
					continue
				}
				w := kpiCardGeometryWithGap(ctx, 4, grid.Gap).valueWidthPt(nil, "")
				lines += measuredLines(p.Content, font, false, p.Size, w)
			}
			if lines != 2 {
				t.Errorf("%s: cell %d has %d caption lines above its delta, want 2 in every cell", tc.name, ci, lines)
			}
			if got := txt.Paragraphs[len(txt.Paragraphs)-1].Content; (*tc.vals)[ci].Comparator == "" && got != kpiBlankLine {
				t.Errorf("%s: cell %d does not reserve the comparator line (last line %q)", tc.name, ci, got)
			}
		}
	}
}

// go-slide-creator-uj9zq: in an area too short for the 40pt figure the row
// steps down its type ladder, then gives up its vertical text margin, and is
// refused when a 24pt value over a 12pt caption still does not fit.
func TestKPIRowStepsDownThenRefusesInShortArea(t *testing.T) {
	p, _ := Default().Get("kpi-4up")
	vals := plainKPIs(4)
	for _, tc := range []struct {
		heightPt    float64
		big, small  float64
		tight, fail bool
	}{
		{300, 40, 14, false, false},
		{85, 28, 12, false, false},
		{78, 24, 12, false, false},
		{62, 24, 12, true, false},
		{50, 0, 0, false, true},
		{27, 0, 0, false, true},
	} {
		ctx := kpiThemeCtx()
		ctx.LayoutBounds.Height = int64(tc.heightPt * 12700)
		grid, err := p.Expand(ctx, vals, nil, nil)
		if tc.fail {
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Code != ErrCodeFitOverflow {
				t.Errorf("%.0fpt: err = %v, want a fit_overflow refusal", tc.heightPt, err)
				continue
			}
			if !strings.Contains(ve.Message, "4 KPIs need") || !strings.Contains(ve.Message, fmt.Sprintf("%.0fpt tall", tc.heightPt)) {
				t.Errorf("%.0fpt: refusal does not name the need and the area: %s", tc.heightPt, ve.Message)
			}
			continue
		}
		if err != nil {
			t.Errorf("%.0fpt: Expand: %v", tc.heightPt, err)
			continue
		}
		if got := grid.Rows[0].MaxHeight; got > tc.heightPt+kpiFitTolerancePt {
			t.Errorf("%.0fpt: row max_height %.0fpt is taller than its area", tc.heightPt, got)
		}
		for ci, c := range grid.Rows[0].Cells {
			txt := kpiCellTextOf(t, c)
			if txt.Paragraphs[0].Size != tc.big || txt.Paragraphs[1].Size != tc.small {
				t.Errorf("%.0fpt cell %d: sizes %v/%v, want %v/%v", tc.heightPt, ci, txt.Paragraphs[0].Size, txt.Paragraphs[1].Size, tc.big, tc.small)
			}
			if tight := txt.InsetTop != nil; tight != tc.tight {
				t.Errorf("%.0fpt cell %d: tight text margin = %v, want %v", tc.heightPt, ci, tight, tc.tight)
			}
			// The cell is tall enough that the writer stores its text unshrunk.
			if need := writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, 150, 0); need > grid.Rows[0].MaxHeight {
				t.Errorf("%.0fpt cell %d: text needs %.0fpt in a %.0fpt row", tc.heightPt, ci, need, grid.Rows[0].MaxHeight)
			}
		}
	}

	// Authored sizes are measured as given, never stepped down.
	ctx := kpiThemeCtx()
	ctx.LayoutBounds.Height = 70 * 12700
	_, err := p.Expand(ctx, vals, &KPIOverrides{BigSize: 40}, nil)
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Message, "overrides.big_size") {
		t.Errorf("authored 40pt in a 70pt area: err = %v, want a refusal naming overrides.big_size", err)
	}
}
