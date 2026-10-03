package shapegrid

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// composeTestGrid is a slide's content-sized block: a heading row and a body
// row capped at their text, in a 300pt-tall area.
func composeTestGrid(rowPt float64, rows int) *Grid {
	text := func(s string, size float64) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"content": s, "size": size})
		return raw
	}
	g := &Grid{
		Bounds:  pptx.RectEmu{X: 0, Y: 1000000, CX: PtToEMU(800), CY: PtToEMU(300)},
		Columns: []float64{50, 50},
		VAlign:  VAlignAuto,
		Compose: true,
		RowGap:  8,
	}
	for r := 0; r < rows; r++ {
		size, s := 12.0, "Body line"
		if r == 0 {
			size, s = 14, "Heading"
		}
		g.Rows = append(g.Rows, Row{MaxHeight: rowPt, Cells: []Cell{
			{Shape: &ShapeSpec{Geometry: "rect", Text: text(s, size)}},
			{Shape: &ShapeSpec{Geometry: "rect", Text: text(s, size)}},
		}})
	}
	return g
}

func resolvedBlock(t *testing.T, g *Grid) (res *ResolveResult, top, bottom int64) {
	t.Helper()
	res, err := Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil || res == nil {
		t.Fatalf("resolve: %v", err)
	}
	top, bottom = 1<<62, 0
	for _, c := range res.Cells {
		top = min(top, c.CellBounds.Y)
		bottom = max(bottom, c.CellBounds.Y+c.CellBounds.CY)
	}
	return res, top, bottom
}

func sizesOf(res *ResolveResult) string {
	var out []string
	for _, c := range res.Cells {
		var obj struct {
			Size float64 `json:"size"`
		}
		_ = json.Unmarshal(c.ShapeSpec.Text, &obj)
		out = append(out, strconv.FormatFloat(obj.Size, 'f', -1, 64))
	}
	return strings.Join(out, ",")
}

// A sparse block takes one type step, grows its rows with it and sits at the
// optical centre of its area.
func TestComposeSparseBlock(t *testing.T) {
	g := composeTestGrid(50, 2) // 108pt of 300pt
	res, top, bottom := resolvedBlock(t, g)
	if !res.Composed {
		t.Fatal("sparse auto block was not composed")
	}
	if got := sizesOf(res); got != "18,18,14,14" {
		t.Errorf("sizes = %s, want the 14pt heading at 18 and the 12pt body at 14", got)
	}
	if h := float64(bottom-top) / 12700; h < 108*1.14 || h > 108*1.36 {
		t.Errorf("block height %.1fpt, want the 108pt block grown by 1.15–1.35", h)
	}
	above, below := top-g.Bounds.Y, g.Bounds.Y+g.Bounds.CY-bottom
	if share := float64(above) / float64(above+below); share < ComposeOpticalTop-0.01 || share > ComposeOpticalTop+0.01 {
		t.Errorf("%.0f%% of the spare height is above the block, want %.0f%%", share*100, ComposeOpticalTop*100)
	}
	// The authored grid is untouched: the step is written to resolved copies.
	if strings.Contains(string(g.Rows[0].Cells[0].Shape.Text), "18") || g.Rows[0].MaxHeight != 50 {
		t.Error("composing mutated the input grid")
	}
}

// A dense block, a grid that is not a slide's own, an explicit alignment and
// a stretch grid are resolved exactly as before.
func TestComposeLeavesOtherBlocksAlone(t *testing.T) {
	plain := func(g *Grid) *Grid { c := *g; c.Compose = false; return &c }
	cases := map[string]*Grid{
		"dense":    composeTestGrid(50, 5), // 282pt of 300pt
		"nested":   plain(composeTestGrid(50, 2)),
		"explicit": func() *Grid { g := composeTestGrid(50, 2); g.VAlign = VAlignTop; return g }(),
		"stretch":  func() *Grid { g := composeTestGrid(50, 2); g.VAlign = VAlignStretch; return g }(),
	}
	for name, g := range cases {
		t.Run(name, func(t *testing.T) {
			res, top, _ := resolvedBlock(t, g)
			want, wantTop, _ := resolvedBlock(t, plain(g))
			if res.Composed {
				t.Error("block was composed")
			}
			if top != wantTop || sizesOf(res) != sizesOf(want) {
				t.Errorf("top %d sizes %s, want top %d sizes %s", top, sizesOf(res), wantTop, sizesOf(want))
			}
		})
	}
}

// The body line bounds the placement from above, and a step that would break
// a word, overflow a cell or touch pinned text is not taken.
func TestComposeGuards(t *testing.T) {
	t.Run("never above the body line", func(t *testing.T) {
		g := composeTestGrid(100, 2) // 208pt: optical top would be ~41pt
		g.AnchorY = g.Bounds.Y + PtToEMU(60)
		_, top, _ := resolvedBlock(t, g)
		if top < g.AnchorY {
			t.Errorf("block starts %.1fpt above the body line", float64(g.AnchorY-top)/12700)
		}
	})
	t.Run("compact text keeps its size", func(t *testing.T) {
		g := composeTestGrid(50, 2)
		g.TypeScale = "compact"
		res, _, _ := resolvedBlock(t, g)
		if got := sizesOf(res); got != "14,14,12,12" {
			t.Errorf("sizes = %s, want the authored 14,14,12,12", got)
		}
		if !res.Composed {
			t.Error("a compact sparse block is still placed by the policy")
		}
	})
	t.Run("a word is not broken", func(t *testing.T) {
		g := composeTestGrid(50, 2)
		g.Columns = []float64{9, 91} // "Heading" fits 72pt at 14pt, not at 18pt
		res, _, _ := resolvedBlock(t, g)
		if got := sizesOf(res); got != "14,14,12,12" {
			t.Errorf("sizes = %s, want the step refused", got)
		}
	})
	t.Run("adjacent levels up to the lead step stay", func(t *testing.T) {
		g := composeTestGrid(50, 2)
		lead, _ := json.Marshal(map[string]any{"content": "Lead", "size": 18})
		g.Rows[1].Cells[0].Shape = &ShapeSpec{Geometry: "rect", Text: lead}
		res, _, _ := resolvedBlock(t, g)
		if got := sizesOf(res); got != "14,14,18,12" {
			t.Errorf("sizes = %s, want 12/14/18pt levels kept (no step closes on the level above)", got)
		}
	})
	t.Run("hairline rows keep their thickness", func(t *testing.T) {
		g := composeTestGrid(50, 2)
		g.Rows = append(g.Rows, Row{MinHeight: 0.75, MaxHeight: 0.75, Cells: []Cell{{ColSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}}}})
		res, _, _ := resolvedBlock(t, g)
		rule := res.Cells[len(res.Cells)-1]
		if h := float64(rule.CellBounds.CY) / 12700; h > 1 {
			t.Errorf("rule row is %.2fpt tall after composing", h)
		}
	})
}

func TestComposedTopOffset(t *testing.T) {
	if got := ComposedTopOffset(400, 1000, 0); got != 270 {
		t.Errorf("optical offset = %d, want 270", got)
	}
	if got := ComposedTopOffset(400, 1000, 500); got != 500 {
		t.Errorf("offset below the body line = %d, want 500", got)
	}
	if got := ComposedTopOffset(400, 1000, 900); got != 600 {
		t.Errorf("offset is bounded by the slack: %d, want 600", got)
	}
	if !IsSparseBlock(700, 1000) || IsSparseBlock(750, 1000) {
		t.Error("a block is sparse under 75% of its area")
	}
}
