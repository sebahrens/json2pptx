package patterns

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
)

// ApplyReadableInk keeps a pattern on the template's own accents. Patterns
// write light (lt1) text on accent fills; on templates whose accents are light
// (p-style's oranges) that text is unreadable, and the old answer was to swap
// the FILL for dk2 / black, painting every card black. Instead, keep the
// accent fill and swap the pattern-authored light text for the first theme ink
// that is readable on it. Only light inks written by the pattern are touched,
// so author-chosen colours and dark text are never rewritten.
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
	minContrast := svggen.WCAGAANormal
	size, _ := text["size"].(float64)
	bold, _ := text["bold"].(bool)
	if size >= 24 || (bold && size >= 18) {
		minContrast = 3.0 // WCAG large text
	}
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		return
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
