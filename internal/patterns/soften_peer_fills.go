package patterns

import (
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
)

// peerCardPatterns are the patterns whose solid accent cards are peers — every
// card carries the same weight, so none of them is a deliberate emphasis.
// Patterns whose fills encode data (heatmap tiers, waterfall bars, scores) or
// a single highlighted step are deliberately absent.
var peerCardPatterns = map[string]bool{
	"kpi-2up": true, "kpi-3up": true, "kpi-4up": true, "kpi-5up": true, "kpi-6up": true,
	"card-grid":            true,
	"icon-row":             true,
	"scqa-summary":         true,
	"before-after":         true,
	"before-after-compact": true,
}

// peerRuleWidthPt is the thickness of the accent rule a softened card gets.
const peerRuleWidthPt = 3

// SoftenPeerFills replaces solid light-accent fills on peer cards with a light
// tint of the same accent plus an accent rule along the top
// (go-slide-creator-pymy7).
//
// On templates whose accents are light (p-style's oranges), a slide of
// 100%-accent peer cards is a wall of colour: nothing reads as emphasis
// because everything is. The rule only fires when:
//
//   - the pattern is a peer-card pattern (peerCardPatterns);
//   - the accent is light, i.e. the pattern's lt1 text fails WCAG AA on it —
//     the same trigger ApplyReadableInk uses; dark-accent templates keep their
//     solid cards unchanged;
//   - at least two cells share the identical, unmodified accent fill. A cell
//     that already stands apart (its own accent bar, a different fill) is the
//     pattern's emphasis and stays solid, as do lone solid cells.
//
// Run it BEFORE ApplyReadableInk, which then swaps the cards' lt1 text for a
// readable theme ink on the new tint. Icons drawn in a light ink on the old
// fill are re-picked for the tint.
func SoftenPeerFills(ctx ExpandContext, patternName string, grid *jsonschema.ShapeGridInput) {
	if grid == nil || !peerCardPatterns[patternName] || len(ctx.Theme.Colors) == 0 {
		return
	}
	groups := map[string][]*jsonschema.GridCellInput{}
	var order []string
	collectPeerCells(grid, groups, &order)
	for _, accent := range order {
		cells := groups[accent]
		if len(cells) < 2 || !lightAccent(ctx, accent) {
			continue
		}
		for _, cell := range cells {
			softenCell(ctx, cell, accent)
		}
	}
}

// collectPeerCells groups text-bearing cells by their bare accent fill.
func collectPeerCells(grid *jsonschema.ShapeGridInput, groups map[string][]*jsonschema.GridCellInput, order *[]string) {
	for ri := range grid.Rows {
		for _, cell := range grid.Rows[ri].Cells {
			if cell == nil {
				continue
			}
			if cell.Grid != nil {
				collectPeerCells(cell.Grid, groups, order)
			}
			if cell.Shape == nil || cell.AccentBar != nil || len(cell.Shape.Text) == 0 {
				continue
			}
			tone, ok := opaqueFillTone(cell.Shape.Fill)
			if !ok || !isAccentSlot(tone.Color) || tone != (fillTone{Color: tone.Color}) {
				continue
			}
			if _, seen := groups[tone.Color]; !seen {
				*order = append(*order, tone.Color)
			}
			groups[tone.Color] = append(groups[tone.Color], cell)
		}
	}
}

func isAccentSlot(c string) bool {
	switch c {
	case "accent1", "accent2", "accent3", "accent4", "accent5", "accent6":
		return true
	}
	return false
}

// lightAccent reports whether lt1 text fails WCAG AA on the accent.
func lightAccent(ctx ExpandContext, accent string) bool {
	fill, ok := resolveThemeColor(ctx, accent)
	if !ok {
		return false
	}
	light, ok := resolveThemeColor(ctx, "lt1")
	return ok && light.ContrastWith(fill) < svggen.WCAGAANormal
}

func softenCell(ctx ExpandContext, cell *jsonschema.GridCellInput, accent string) {
	oldFill := cell.Shape.Fill
	tint := inactiveTintTone(accent).fillJSON()
	cell.Shape.Fill = tint
	cell.AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: peerRuleWidthPt}
	if icon := cell.Shape.Icon; icon != nil && (icon.Fill == "" || icon.Fill == iconFillOn(ctx, oldFill, accent)) {
		icon.Fill = iconFillOn(ctx, tint, accent)
	}
}
