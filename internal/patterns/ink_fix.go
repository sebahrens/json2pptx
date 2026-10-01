package patterns

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
)

// ApplyReadableInk keeps a pattern on the template's own accents. Patterns
// write light (lt1) text on accent fills; on templates whose accents are
// mid-tone (p-style's orange, abstract's taupe, warm-coral's E64A19) that text
// misses WCAG AA. The old answers were to swap the FILL for dk2 / black
// (every card black) or the text for dk1 (bold black type on a saturated
// box). Instead the accent fill is first deepened by the smallest a:shade at
// which the light text reads — same hue, white type (go-slide-creator-v9tup) —
// and only a fill no shade can rescue (a pale accent, a tint) keeps its
// surface and has the pattern-authored light text swapped for the first
// readable theme ink. Only light inks written by the pattern are touched, so
// author-chosen colours and dark text are never rewritten.
func ApplyReadableInk(ctx ExpandContext, grid *jsonschema.ShapeGridInput) {
	if grid == nil || len(ctx.Theme.Colors) == 0 {
		return
	}
	for ri := range grid.Rows {
		for ci := range grid.Rows[ri].Cells {
			cell := grid.Rows[ri].Cells[ci]
			if cell == nil {
				continue
			}
			if cell.Grid != nil {
				ApplyReadableInk(ctx, cell.Grid)
			}
			if cell.Shape != nil {
				fixShapeInk(ctx, cell.Shape)
			}
		}
	}
}

// WCAG "large text" in points: 18pt, or 14pt bold (the 24px / 18.66px of the
// CSS wording). Shared with the render-time contrast pass so the ink written
// at expansion is never re-judged against a different bar
// (go-slide-creator-z668n).
const (
	LargeTextPt     = 18.0
	LargeBoldTextPt = 14.0
)

// TextContrastThreshold returns the WCAG AA ratio text of the given size must
// meet: 3:1 for large text, 4.5:1 otherwise. A size <= 0 is treated as normal.
func TextContrastThreshold(pt float64, bold bool) float64 {
	if pt >= LargeTextPt || (bold && pt >= LargeBoldTextPt) {
		return svggen.WCAGAALarge
	}
	return svggen.WCAGAANormal
}

func fixShapeInk(ctx ExpandContext, shape *jsonschema.ShapeSpecInput) {
	if len(shape.Fill) == 0 || len(shape.Text) == 0 {
		return
	}
	tone, ok := opaqueFillTone(shape.Fill)
	if !ok {
		return
	}
	var text map[string]any
	if err := json.Unmarshal(shape.Text, &text); err != nil {
		return // plain-string text carries no colour
	}
	size, _ := text["size"].(float64)
	bold, _ := text["bold"].(bool)
	minContrast := TextContrastThreshold(size, bold)
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		return
	}
	if hasFailingLightInk(ctx, text, fill, minContrast) && !hasDarkInk(text) {
		if shaded, sok := shadeForLightInk(ctx, tone, minContrast); sok {
			shape.Fill = shaded.fillJSON()
			return
		}
	}
	if !recolorLightInk(ctx, text, fill, tone, minContrast) {
		return
	}
	if data, err := json.Marshal(text); err == nil {
		shape.Text = data
	}
}

// recolorLightInk rewrites every light-ink "color" in the text object (top
// level, paragraphs, runs) that fails minContrast on the fill.
func recolorLightInk(ctx ExpandContext, node any, fill svggen.Color, tone fillTone, minContrast float64) bool {
	changed := false
	switch v := node.(type) {
	case map[string]any:
		if c, ok := v["color"].(string); ok && isLightInk(c) {
			if ink, iok := resolveThemeColor(ctx, c); iok && ink.ContrastWith(fill) < minContrast {
				if repl := readableInkOn(ctx, tone, c, minContrast); repl != c {
					v["color"] = repl
					changed = true
				}
			}
		}
		for k, child := range v {
			if k == "color" {
				continue
			}
			if recolorLightInk(ctx, child, fill, tone, minContrast) {
				changed = true
			}
		}
	case []any:
		for _, child := range v {
			if recolorLightInk(ctx, child, fill, tone, minContrast) {
				changed = true
			}
		}
	}
	return changed
}

// hasFailingLightInk reports whether any light-ink "color" in the text object
// misses minContrast on fill.
func hasFailingLightInk(ctx ExpandContext, node any, fill svggen.Color, minContrast float64) bool {
	switch v := node.(type) {
	case map[string]any:
		if c, ok := v["color"].(string); ok && isLightInk(c) {
			if ink, iok := resolveThemeColor(ctx, c); iok && ink.ContrastWith(fill) < minContrast {
				return true
			}
		}
		for k, child := range v {
			if k != "color" && hasFailingLightInk(ctx, child, fill, minContrast) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasFailingLightInk(ctx, child, fill, minContrast) {
				return true
			}
		}
	}
	return false
}

// hasDarkInk reports whether the text object sets any non-light colour; a
// shaded fill would cost that text contrast, so such shapes keep their fill.
func hasDarkInk(node any) bool {
	switch v := node.(type) {
	case map[string]any:
		if c, ok := v["color"].(string); ok && c != "" && !isLightInk(c) {
			return true
		}
		for k, child := range v {
			if k != "color" && hasDarkInk(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasDarkInk(child) {
				return true
			}
		}
	}
	return false
}

func isLightInk(c string) bool {
	switch strings.ToLower(strings.TrimPrefix(c, "#")) {
	case "lt1", "bg1", "ffffff":
		return true
	}
	return false
}

// opaqueFillTone parses a shape-grid fill, rejecting translucent ones: an
// explicit low alpha (e.g. alpha 0 on label cells) means the text sits on
// whatever is behind the shape, not on this colour.
func opaqueFillTone(raw json.RawMessage) (fillTone, bool) {
	tone, ok := parseFillTone(raw)
	if !ok {
		return fillTone{}, false
	}
	var probe struct {
		Alpha *float64 `json:"alpha"`
	}
	if json.Unmarshal(raw, &probe) == nil && probe.Alpha != nil && *probe.Alpha < 50 {
		return fillTone{}, false
	}
	return tone, true
}
