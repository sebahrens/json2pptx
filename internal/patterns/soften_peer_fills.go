package patterns

import (
	"github.com/sebahrens/json2pptx/internal/jsonschema"
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
	// Its two column headers are peers too (go-slide-creator-8xsj3).
	"comparison-2col": true,
}

// SharedRotationKey returns the rotation key every pattern of a sibling family
// shares, and ok=false for patterns that rotate on their own content. KPI
// patterns are one family: under accent_strategy "rotate" a kpi-3up and a
// kpi-4up (or two composed on one slide) used to hash to different accents,
// so sibling metric cards changed colour for no reason — midnight-blue's
// KPI 4-up turned pink with a red rule beside a navy 3-up
// (go-slide-creator-8xsj3). The family key keeps them on one accent.
func SharedRotationKey(patternName string) (string, bool) {
	switch patternName {
	case "kpi-2up", "kpi-3up", "kpi-4up", "kpi-5up", "kpi-6up", "kpi-inline":
		return "kpi-family", true
	}
	return "", false
}

// peerRuleWidthPt is the thickness of the accent rule a softened card gets.
const peerRuleWidthPt = 3

// SoftenPeerFills replaces solid accent fills on peer cards with a neutral
// surface (dk1 at 4%) plus an accent rule along the top
// (go-slide-creator-pymy7, go-slide-creator-8xsj3).
//
// A slide of 100%-accent peer cards is a wall of colour: nothing reads as
// emphasis because everything is. accent1 at 100% belongs on at most one
// emphasised element per slide, so peer cards take the neutral surface on
// every template — light accents (p-style's oranges) and dark ones
// (midnight-blue's navy) alike — and the accent survives as the 3pt rule.
// The rule only fires when:
//
//   - the pattern is a peer-card pattern (peerCardPatterns);
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
		if len(cells) < 2 {
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

func softenCell(ctx ExpandContext, cell *jsonschema.GridCellInput, accent string) {
	oldFill := cell.Shape.Fill
	tint := neutralFillJSON(NeutralTint4)
	cell.Shape.Fill = tint
	cell.AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: peerRuleWidthPt}
	if icon := cell.Shape.Icon; icon != nil && (icon.Fill == "" || icon.Fill == iconFillOn(ctx, oldFill, accent)) {
		icon.Fill = iconFillOn(ctx, tint, accent)
	}
}
