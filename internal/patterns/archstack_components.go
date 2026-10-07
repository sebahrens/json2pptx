package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// Component layout of arch-stack (go-slide-creator-6h1fy, redrawn for
// go-slide-creator-tfxns).
//
// A tier that lists its components is a lane: its name on a pentagon tab
// pointing at one block per component, the blocks standing directly on the
// page:
//
//	[ Channels        > [ Web ] [ Mobile app ] [ Partner API ]     ▮ ▮
//	[ Domain services > [ Orders ] [ Pricing ] [ Billing ] [ … ]   ▮ ▮
//	[ Data platform   > [ Operational DB ] [ Lakehouse ]           ▮ ▮
//
// The stack takes this layout as soon as one tier has components; a tier with
// a description keeps it as a line of text on one pale block where the
// component blocks would be, and a tier with neither is its tab before an
// empty pale block. A stack with no components anywhere is drawn as before
// (label over description, centred).
//
// There is no band behind a tier's blocks: fourteen blocks inside four bands
// read as cards inside cards. The tab (the deeper accent swatch, bold name)
// says which lane the blocks belong to, the blocks (the accent's content
// swatch) are the content, and the rails — which cross every lane — are the
// stack's only dark bars. Each tier is one grid row [tab | blocks]; the blocks
// are a nested grid, which the renderer insets by archStackSubGridInsetPt, so
// the tab and the rails are pulled in by the same amount and every lane's
// edges line up. Seven to twelve components wrap to two rows of blocks.

const (
	// archStackMaxComponents is the most blocks one tier holds, and
	// archStackComponentsPerRow the most on one line before they wrap.
	archStackMaxComponents    = 12
	archStackComponentsPerRow = 6
	// archStackComponentMaxLen is the longest component name.
	archStackComponentMaxLen = 40
	// archStackTierGapPt separates two tiers and archStackBlockGapPt two
	// blocks: lanes sit a little further apart than the blocks of one lane.
	archStackTierGapPt  = 10.0
	archStackBlockGapPt = 6.0
	// archStackSubGridInsetPt is the inset the renderer gives a nested grid.
	archStackSubGridInsetPt = 4.0
	// archStackSeamPt is a grid gap that is no gap: 0 would select the grid's
	// default.
	archStackSeamPt = 0.01
	// archStackRailGapPt separates the tiers from a rail and two rails.
	archStackRailGapPt = 4.0
	// Label column, as a share of the tier width: as wide as the longest tier
	// name needs, within these bounds.
	archStackLabelMinFrac = 0.14
	archStackLabelMaxFrac = 0.26
	// archStackTabPointPt is the depth of a tier tab's point.
	archStackTabPointPt = labeledRowsTabPointPt
	// archStackFillFrac is the share of the content height a stack grows its
	// lanes towards, and archStackMaxStretch how far past their written need:
	// the 1.6x a filled box takes before it reads as an empty panel.
	archStackFillFrac   = 0.80
	archStackMaxStretch = contentStretchMax
	// archStackLeadFillFrac is the share of the content height the stack may
	// need at the larger type step.
	archStackLeadFillFrac = 0.85
)

// archStackLeadScale is the larger type step of the component layout, tier
// name then component: every block of the stack is pinned at one size
// (peerTextTypeScale), so the layout picks it — 18 / 14pt when no name wraps
// further for it and the stack keeps its air, else the standard 14 / 12pt.
var archStackLeadScale = [2]float64{scaleLeadPt, scaleSubheadPt}

// archStackHasComponents reports whether any tier lists components.
func archStackHasComponents(v *ArchStackValues) bool {
	for _, t := range v.Tiers {
		if len(t.Components) > 0 {
			return true
		}
	}
	return false
}

// archStackBlockRows is how many rows of blocks n components take, and how
// many blocks sit on the fuller row.
func archStackBlockRows(n int) (rows, perRow int) {
	if n <= archStackComponentsPerRow {
		return 1, n
	}
	return 2, (n + 1) / 2
}

// archStackText builds a one-paragraph text object in the given ink.
func archStackText(content string, size float64, bold bool, align, ink string, insetRight float64) json.RawMessage {
	para := map[string]any{"content": content, "size": size, "color": ink, "align": align}
	if bold {
		para["bold"] = true
	}
	obj := map[string]any{"paragraphs": []any{para}, "align": align, "vertical_align": "ctr"}
	if insetRight != 0 {
		obj["inset_right"] = insetRight
	}
	data, _ := json.Marshal(obj)
	return data
}

// archStackComponentLayout is the measured geometry of a component stack.
type archStackComponentLayout struct {
	headerSize, bodySize float64
	tierW, labelW, areaW float64
	railW                float64
	labelText            []json.RawMessage   // by tier: the name, measured in the label column
	blockText            [][]json.RawMessage // by tier, by component
	blockW               []float64           // by tier: width of one block
	blockH               []float64           // by tier: height of one block as laid out
	tierH                []float64           // by tier: lane height as laid out
	needPt               float64             // height the stack needs before it grows
	wrapped              int                 // names set on more than one line
}

// layoutArchStackComponents measures the stack at the context's content area
// and picks its type sizes: the larger step when no name wraps further for it
// and the stack keeps its air, else the standard sizes. An authored
// header_size or body_size is kept.
func layoutArchStackComponents(ctx ExpandContext, vals *ArchStackValues, ovr *ArchStackOverrides) archStackComponentLayout {
	std := measureArchStackComponents(ctx, vals, ResolveSize(ovr.HeaderSize, scaleSubheadPt), ResolveSize(ovr.BodySize, scaleDenseBodyPt))
	if ovr.HeaderSize != 0 || ovr.BodySize != 0 {
		return std
	}
	_, contentH := contentAreaPt(ctx)
	if lead := measureArchStackComponents(ctx, vals, archStackLeadScale[0], archStackLeadScale[1]); contentH > 0 && lead.wrapped <= std.wrapped && lead.needPt <= contentH*archStackLeadFillFrac {
		return lead
	}
	return std
}

func measureArchStackComponents(ctx ExpandContext, vals *ArchStackValues, headerSize, bodySize float64) archStackComponentLayout {
	fonts := ctx.themeFonts()
	contentW, contentH := contentAreaPt(ctx)
	n := len(vals.Tiers)

	lay := archStackComponentLayout{
		headerSize: headerSize,
		bodySize:   bodySize,
		railW:      contentW * archStackRailWidthPct / 100,
		labelText:  make([]json.RawMessage, n),
		blockText:  make([][]json.RawMessage, n),
		blockW:     make([]float64, n),
		blockH:     make([]float64, n),
		tierH:      make([]float64, n),
	}
	lay.tierW = contentW - float64(len(vals.SideRails))*(lay.railW+archStackRailGapPt)

	minLabel, maxLabel := lay.tierW*archStackLabelMinFrac, lay.tierW*archStackLabelMaxFrac
	lay.labelW = minLabel
	for i, tier := range vals.Tiers {
		lay.labelText[i] = archStackTabText(pptx.ConvertMarkdownEmphasis(tier.Label), headerSize, "dk1")
		lay.labelW = math.Max(lay.labelW, labelFitWidthPt(fonts, lay.labelText[i], minLabel, maxLabel))
	}
	lay.labelW = math.Ceil(lay.labelW)
	lay.areaW = lay.tierW - lay.labelW

	// Blocks are one height across the stack: the tallest block's need.
	oneLine := math.Ceil(bodySize*contentLineHeight + 2*defaultShapeInsetTBPt)
	blockNeed := 0.0
	for i, tier := range vals.Tiers {
		if len(tier.Components) == 0 {
			continue
		}
		_, perRow := archStackBlockRows(len(tier.Components))
		lay.blockW[i] = math.Max((lay.areaW-2*archStackSubGridInsetPt-float64(perRow-1)*ctx.Gap(archStackBlockGapPt))/float64(perRow), 1)
		for _, c := range tier.Components {
			text := archStackText(pptx.ConvertMarkdownEmphasis(c), bodySize, false, "ctr", "dk1", 0)
			lay.blockText[i] = append(lay.blockText[i], text)
			need := driverTreeNeedPt(fonts, text, lay.blockW[i], bodySize)
			if need > oneLine+1 {
				lay.wrapped++
			}
			blockNeed = math.Max(blockNeed, need)
		}
	}

	// One-row tiers share a height; a tier whose blocks wrap is taller.
	single, sum := 0.0, 0.0
	for i, tier := range vals.Tiers {
		need := driverTreeNeedPt(fonts, lay.labelText[i], lay.labelW, headerSize)
		switch {
		case len(tier.Components) > 0:
			rows, _ := archStackBlockRows(len(tier.Components))
			need = math.Max(need, float64(rows)*blockNeed+float64(rows-1)*ctx.Gap(archStackBlockGapPt))
			if rows > 1 {
				lay.tierH[i] = math.Ceil(need) + 2*archStackSubGridInsetPt
				continue
			}
		case tier.Description != "":
			desc := archStackText(pptx.ConvertMarkdownEmphasis(tier.Description), bodySize, false, "l", "dk1", 0)
			need = math.Max(need, driverTreeNeedPt(fonts, desc, lay.areaW-2*archStackSubGridInsetPt, bodySize))
		}
		single = math.Max(single, math.Ceil(need)+2*archStackSubGridInsetPt)
	}
	for i := range lay.tierH {
		if lay.tierH[i] == 0 {
			lay.tierH[i] = single
		}
		sum += lay.tierH[i]
	}
	gaps := float64(n-1)*archStackTierRowGapPt() + 1
	lay.needPt = sum + gaps
	if avail := contentH - gaps; contentH > 0 && avail > 0 {
		_, sizingH := sizingAreaPt(ctx)
		k := avail / sum
		if sum <= avail {
			// The lanes grow towards the fill target, so a stack alone on a
			// slide reaches the lower third (go-slide-creator-tfxns).
			k = clampPt((sizingH*archStackFillFrac-gaps)/sum, 1, math.Min(archStackMaxStretch, k))
		}
		// A stack taller than the content area keeps its proportions.
		for i := range lay.tierH {
			lay.tierH[i] = math.Floor(lay.tierH[i] * k)
		}
	}
	for i, tier := range vals.Tiers {
		if rows, _ := archStackBlockRows(len(tier.Components)); len(tier.Components) > 0 {
			lay.blockH[i] = (lay.tierH[i] - 2*archStackSubGridInsetPt - float64(rows-1)*ctx.Gap(archStackBlockGapPt)) / float64(rows)
		}
	}
	return lay
}

// archStackTierRowGapPt is the grid's row gap: the nested grids' own inset
// already holds two lanes apart, and the row gap adds the rest of
// archStackTierGapPt.
func archStackTierRowGapPt() float64 {
	return math.Max(archStackTierGapPt-2*archStackSubGridInsetPt, archStackSeamPt)
}

// archStackTabText is a tier tab's text: the name, bold, left-aligned, its
// right inset giving back the width the point takes.
func archStackTabText(label string, size float64, ink string) json.RawMessage {
	return archStackText(label, size, true, "l", ink, labeledRowsTabInsetRight)
}

// archStackComponentWarnings names the components that are written shrunk in
// their block.
func archStackComponentWarnings(ctx ExpandContext, vals *ArchStackValues, ovr *ArchStackOverrides) []string {
	lay := layoutArchStackComponents(ctx, vals, ovr)
	fonts := ctx.themeFonts()
	var warnings []string
	for i, tier := range vals.Tiers {
		for j, text := range lay.blockText[i] {
			tb, err := shapegrid.ResolveTextInput(text)
			if err != nil || tb == nil {
				continue
			}
			tb.ThemeFonts = fonts
			box := pptx.RectEmu{CX: int64(lay.blockW[i] * sizingEMUPerPt), CY: int64(lay.blockH[i] * sizingEMUPerPt)}
			if !pptx.AutofitFitsFor(tb, box) {
				warnings = append(warnings, fmt.Sprintf("%s: arch-stack tiers[%d].components[%d] is %d characters; its block, one of %d in a stack of %d tiers, does not hold it at a readable size — shorten the name, or use fewer components in this tier or fewer tiers", ErrCodeBodyTooLong, i, j, runeLen(tier.Components[j]), len(tier.Components), len(vals.Tiers)))
			}
		}
	}
	return warnings
}

// expandComponents renders the stack as lanes: a tab and one block per
// component.
func (a *archStack) expandComponents(ctx ExpandContext, vals *ArchStackValues, ovr *ArchStackOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	lay := layoutArchStackComponents(ctx, vals, ovr)
	n := len(vals.Tiers)

	// Columns: the tier's tab, its blocks, then a gap and a bar per rail.
	cols := []float64{lay.labelW, lay.areaW}
	for range vals.SideRails {
		cols = append(cols, archStackRailGapPt, lay.railW)
	}
	colsJSON, _ := json.Marshal(cols)

	// Roles (go-slide-creator-x5m8f): a tab heads its lane (the deeper rung),
	// the component blocks are the content, a lane without components is a
	// pale block, and a rail is an anchor that ties every lane together.
	tabTone := tonalRung(ctx, baseAccent, TonalLighterDeep)
	tabInk := tonalInk(ctx, tabTone)
	blockTone := tonalContent(ctx, baseAccent)
	blockInk := tonalInk(ctx, blockTone)
	paleTone := tonalRung(ctx, baseAccent, TonalLighterPale)
	paleInk := tonalInk(ctx, paleTone)
	// in pulls a shape in to the lanes' edges (the nested grids' inset).
	in := func(cell *jsonschema.GridCellInput) *jsonschema.GridCellInput {
		cell.InsetTop, cell.InsetBottom = archStackSubGridInsetPt, archStackSubGridInsetPt
		return cell
	}

	var rows []jsonschema.GridRowInput
	for i, tier := range vals.Tiers {
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		tierH := lay.tierH[i]

		tab := in(&jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry:    "homePlate",
			Fill:        tabTone.fillJSON(),
			Line:        noLine,
			Adjustments: map[string]int64{"adj": swimlanePointAdj(archStackTabPointPt, lay.labelW, tierH-2*archStackSubGridInsetPt)},
			Text:        archStackTabText(pptx.ConvertMarkdownEmphasis(tier.Label), lay.headerSize, tabInk),
		}})
		if co, ok := cellOverrides[i].(*ArchStackCellOverride); ok {
			applyCellTextOverride(tab, co)
			if co.AccentBar {
				tab.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
			}
		}

		sub := &jsonschema.ShapeGridInput{Columns: json.RawMessage("1"), Gap: ctx.Gap(archStackBlockGapPt)}
		switch {
		case len(tier.Components) > 0:
			blockRows, perRow := archStackBlockRows(len(tier.Components))
			sub.Columns = json.RawMessage(fmt.Sprintf("%d", perRow))
			for r := 0; r < blockRows; r++ {
				var blocks []*jsonschema.GridCellInput
				for k := r * perRow; k < min((r+1)*perRow, len(tier.Components)); k++ {
					blocks = append(blocks, &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						// The blocks of every tier are peers, and each tier is a sub-grid
						// of its own, which the peer-growth resolver cannot see across:
						// they keep the size the layout measured (go-slide-creator-riyh7).
						TypeScale: peerTextTypeScale,
						Fill:      blockTone.fillJSON(),
						Line:      noLine,
						Text:      recolorTextInk(lay.blockText[i][k], "dk1", blockInk),
					}})
				}
				sub.Rows = append(sub.Rows, jsonschema.GridRowInput{Cells: blocks})
			}
		default:
			// A lane without components: one pale block, holding the tier's
			// description when it has one.
			block := &jsonschema.ShapeSpecInput{Geometry: "rect", TypeScale: peerTextTypeScale, Fill: paleTone.fillJSON(), Line: noLine}
			if desc := strings.TrimSpace(tier.Description); desc != "" {
				block.Text = archStackText(pptx.ConvertMarkdownEmphasis(desc), lay.bodySize, false, "l", paleInk, 0)
			}
			sub.Rows = []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Shape: block}}}}
		}
		cells := []*jsonschema.GridCellInput{tab, {Grid: sub}}
		if i == 0 {
			for j, rail := range vals.SideRails {
				railCell := in(archStackRailCell(ctx, rail, lay.bodySize))
				railCell.RowSpan = n
				if co, ok := cellOverrides[n+j].(*ArchStackCellOverride); ok {
					applyCellTextOverride(railCell, co)
				}
				cells = append(cells, &jsonschema.GridCellInput{RowSpan: n}, railCell)
			}
		}
		rows = append(rows, jsonschema.GridRowInput{Cells: cells, MinHeight: tierH, MaxHeight: tierH})
	}

	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		ColGap:        archStackSeamPt,
		RowGap:        archStackTierRowGapPt(),
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// archStackRailCell is a cross-cutting rail: one dark bar down the whole
// stack with its name in bold, reading bottom to top. A rail crosses every
// lane, so it takes the anchor tone (tonalBadge) — neither one more lane in
// the accent's tint nor a second accent.
func archStackRailCell(ctx ExpandContext, label string, size float64) *jsonschema.GridCellInput {
	tone, ink := tonalBadge(ctx)
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry:  "rect",
		TypeScale: peerTextTypeScale,
		Fill:      tone.fillJSON(),
		Line:      noLine,
		Text:      buildArchStackRailContent(label, size, ink),
	}}
}
