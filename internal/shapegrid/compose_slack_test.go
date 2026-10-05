package shapegrid

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// A stepped label is not left on one line of a box one line tall when it is
// within RenderFaceSlack of the box's width (go-slide-creator-bhoo3). The type
// step set a comparison cell at 18pt across 95% of its box: the engine's face
// keeps it on one line, a renderer whose face runs wider wraps it, the box
// has no second line and the renderer shrinks that cell alone. The step now
// takes a row growth that leaves the second line, or the block keeps its
// type. The sweep grows the label a letter at a time across the whole window
// and holds every resolved cell to the rule.
func TestComposeStepLeavesRoomForARendererWrap(t *testing.T) {
	text := func(s string) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"content": s, "size": 14})
		return raw
	}
	grid := func(long string) *Grid {
		g := &Grid{
			Bounds:  pptx.RectEmu{X: 0, Y: 1000000, CX: PtToEMU(800), CY: PtToEMU(360)},
			Columns: []float64{50, 50},
			VAlign:  VAlignAuto,
			Compose: true,
			RowGap:  8,
		}
		for r := 0; r < 3; r++ {
			right := "Short label"
			if r == 0 {
				right = long
			}
			g.Rows = append(g.Rows, Row{MaxHeight: 56, Cells: []Cell{
				{Shape: &ShapeSpec{Geometry: "rect", Text: text("Quarterly reporting packs")}},
				{Shape: &ShapeSpec{Geometry: "rect", Text: text(right)}},
			}})
		}
		return g
	}
	stepped, tall := 0, 0
	for n := 0; n <= 60; n++ {
		long := "Always-on client portal with live " + strings.Repeat("i", n)
		res, _, _ := resolvedBlock(t, grid(long))
		for _, c := range res.Cells {
			tb, err := ResolveTextInput(c.ShapeSpec.Text)
			if err != nil || tb == nil {
				t.Fatal(err)
			}
			paras := scaleParagraphs(tb, c.ShapeSpec.ThemeFonts)
			if len(paras) != 1 {
				t.Fatalf("cell has %d paragraphs", len(paras))
			}
			p := paras[0]
			if p.fontPt > 14 && strings.HasPrefix(p.text, "Always-on") {
				stepped++
			}
			width, height := scaledTextRect(c.Bounds, tb, c.TextInsets)
			at := func(w int64) int {
				m, err := textfit.MeasureRun(p.text, p.face, p.fontPt, w, 0)
				if err != nil {
					t.Fatal(err)
				}
				return m.Lines
			}
			if at(width) != 1 || at(int64(float64(width)/RenderFaceSlack)) == 1 {
				continue
			}
			// On one line here, wrapped in a renderer's wider face: the box
			// holds the second line.
			if twoLines := 2 * p.fontPt * textLineHeightFactor; float64(height)/12700 < twoLines {
				t.Errorf("n=%d: %q is set at %.0fpt on one line within %.0f%% of a box %.1fpt tall: a renderer that wraps it has no second line (%.1fpt) and shrinks this cell alone",
					n, p.text, p.fontPt, (RenderFaceSlack-1)*100, float64(height)/12700, twoLines)
			} else if p.fontPt > 14 {
				tall++
			}
		}
	}
	if stepped == 0 {
		t.Error("no label length took the type step: the sweep no longer exercises it")
	}
	if tall == 0 {
		t.Error("no stepped label within the slack got a row with room for its second line")
	}
}
