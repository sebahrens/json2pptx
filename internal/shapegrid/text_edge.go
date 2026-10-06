package shapegrid

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// One left edge for title, body and exhibit text (go-slide-creator-svrpx).
//
// Every grid cell writes the uniform 0.5 cm text inset, while the title's
// text starts at its placeholder edge plus the template's own lIns (7.2pt by
// default, 0 on p-style). The grid's left edge is the body placeholder edge,
// so unfilled first-column text — exec-summary lead-ins, metric labels,
// next-steps numerals, agenda numerals — started 7-14pt right of the title
// text. Filled cards keep their inner padding (their edge is the visible
// line); text standing on the slide background has no edge of its own, so its
// left inset is reduced until it starts on the title's text line.

// alignFirstColumnText reduces the left inset of unfilled, unlined,
// left-aligned text cells in the grid's first column so their text starts at
// textLeft (absolute EMU). Cells with an explicit inset_left or an icon
// reservation are left alone, and the inset is never increased.
func alignFirstColumnText(cells []ResolvedCell, gridX, textLeft int64) {
	if textLeft <= 0 {
		return
	}
	def := pptx.ShapeTextInsets()[0]
	for i := range cells {
		c := &cells[i]
		if c.Layer || c.Kind != CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		if d := c.CellBounds.X - gridX; d < -1 || d > 1 || c.TextInsets[0] != 0 {
			continue
		}
		if !unpaintedJSON(c.ShapeSpec.Fill) || !unpaintedJSON(c.ShapeSpec.Line) || hasExplicitInsetLeft(c.ShapeSpec.Text) {
			continue
		}
		tb, err := ResolveTextInput(c.ShapeSpec.Text)
		if err != nil || tb.Vert != "" || !allParagraphsLeft(tb) {
			continue
		}
		want := max(textLeft-c.Bounds.X, 0)
		if want >= def {
			continue
		}
		c.TextInsets[0] = want - def
	}
}

// unpaintedJSON reports whether a fill / line value draws nothing: absent,
// "none", or an object whose colour is none or whose alpha is 0.
func unpaintedJSON(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		return s == "" || strings.EqualFold(s, "none")
	}
	var obj struct {
		Color string   `json:"color"`
		Alpha *float64 `json:"alpha"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	if c := strings.TrimSpace(obj.Color); c == "" || strings.EqualFold(c, "none") {
		return true
	}
	return obj.Alpha != nil && *obj.Alpha == 0
}

func hasExplicitInsetLeft(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	_, ok := obj["inset_left"]
	return ok
}

func allParagraphsLeft(tb *pptx.TextBody) bool {
	if len(tb.Paragraphs) == 0 {
		return false
	}
	for _, p := range tb.Paragraphs {
		if p.Align != "l" && p.Align != "just" {
			return false
		}
	}
	return true
}
