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

// Component layout of arch-stack (go-slide-creator-6h1fy).
//
// A tier that lists its components is a band with its name on the left and one
// block per component beside it:
//
//	┃ Channels        [ Web ] [ Mobile app ] [ Partner API ]     ▒ ▒
//	┃ Domain services [ Orders ] [ Pricing ] [ Billing ] [ … ]   ▒ ▒
//	┃ Data platform   [ Operational DB ] [ Lakehouse ]           ▒ ▒
//
// The stack takes this layout as soon as one tier has components; a tier with
// a description keeps it as a line of text where the blocks would be, and a
// tier with neither is its name alone. A stack with no components anywhere is
// drawn as before (label over description, centred).
//
// Each tier is two grid rows. The first holds the blocks as a nested grid,
// which the renderer draws after every shape of the outer grid. The second is
// a thin spacer (the gap to the next tier) whose band shape bleeds up over the
// first, so the band lies under the blocks; the band's text is the tier name,
// held to the label column by its right text inset. Seven to twelve components wrap to two rows of blocks. The rails
// take a tint of the accent, so a cross-cutting concern does not read as one
// more neutral tier.

const (
	// archStackMaxComponents is the most blocks one tier holds, and
	// archStackComponentsPerRow the most on one line before they wrap.
	archStackMaxComponents    = 12
	archStackComponentsPerRow = 6
	// archStackComponentMaxLen is the longest component name.
	archStackComponentMaxLen = 40
	// archStackTierGapPt separates two tiers, archStackBlockGapPt two blocks,
	// and archStackBlockPadPt a block from the edge of its band.
	archStackTierGapPt  = 4.0
	archStackBlockGapPt = 6.0
	archStackBlockPadPt = 6.0
	// archStackSubGridInsetPt is the inset the renderer gives a nested grid.
	archStackSubGridInsetPt = 4.0
	// archStackSeamPt is the grid gap: the spacer rows carry the spacing, so
	// rows and columns abut. 0 would select the grid's default gap.
	archStackSeamPt = 0.01
	// archStackRailGapPt separates the tiers from a rail and two rails.
	archStackRailGapPt = 4.0
	// Label column, as a share of the tier width: as wide as the longest tier
	// name needs, within these bounds.
	archStackLabelMinFrac = 0.14
	archStackLabelMaxFrac = 0.26
)

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

// archStackText builds a one-paragraph text object.
func archStackText(content string, size float64, bold bool, align string, insetRight float64) json.RawMessage {
	para := map[string]any{"content": content, "size": size, "color": "dk1", "align": align}
	if bold {
		para["bold"] = true
	}
	obj := map[string]any{"paragraphs": []any{para}, "align": align, "vertical_align": "ctr"}
	if insetRight > 0 {
		obj["inset_right"] = insetRight
	}
	data, _ := json.Marshal(obj)
	return data
}

// archStackComponentLayout is the measured geometry of a component stack.
type archStackComponentLayout struct {
	tierW, labelW, areaW float64
	railW                float64
	labelText            []json.RawMessage   // by tier: the name, measured in the label column
	blockText            [][]json.RawMessage // by tier, by component
	blockW               []float64           // by tier: width of one block
	blockH               []float64           // by tier: height of one block as laid out
	tierH                []float64           // by tier: band height as laid out
}

// layoutArchStackComponents measures the stack at the context's content area.
func layoutArchStackComponents(ctx ExpandContext, vals *ArchStackValues, ovr *ArchStackOverrides) archStackComponentLayout {
	headerSize := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	bodySize := ResolveSize(ovr.BodySize, scaleDenseBodyPt)
	fonts := ctx.themeFonts()
	contentW, contentH := contentAreaPt(ctx)
	n := len(vals.Tiers)

	lay := archStackComponentLayout{
		railW:     contentW * archStackRailWidthPct / 100,
		labelText: make([]json.RawMessage, n),
		blockText: make([][]json.RawMessage, n),
		blockW:    make([]float64, n),
		blockH:    make([]float64, n),
		tierH:     make([]float64, n),
	}
	lay.tierW = contentW - float64(len(vals.SideRails))*(lay.railW+archStackRailGapPt)

	minLabel, maxLabel := lay.tierW*archStackLabelMinFrac, lay.tierW*archStackLabelMaxFrac
	lay.labelW = minLabel
	for i, tier := range vals.Tiers {
		lay.labelText[i] = archStackText(pptx.ConvertMarkdownEmphasis(tier.Label), headerSize, true, "l", 0)
		lay.labelW = math.Max(lay.labelW, labelFitWidthPt(fonts, lay.labelText[i], minLabel, maxLabel))
	}
	lay.labelW = math.Ceil(lay.labelW)
	lay.areaW = lay.tierW - archStackTierBarPt - lay.labelW - (archStackBlockPadPt - archStackSubGridInsetPt)

	// Blocks are one height across the stack: the tallest block's need.
	blockNeed := 0.0
	for i, tier := range vals.Tiers {
		if len(tier.Components) == 0 {
			continue
		}
		_, perRow := archStackBlockRows(len(tier.Components))
		lay.blockW[i] = math.Max((lay.areaW-2*archStackSubGridInsetPt-float64(perRow-1)*ctx.Gap(archStackBlockGapPt))/float64(perRow), 1)
		for _, c := range tier.Components {
			text := archStackText(pptx.ConvertMarkdownEmphasis(c), bodySize, false, "ctr", 0)
			lay.blockText[i] = append(lay.blockText[i], text)
			blockNeed = math.Max(blockNeed, driverTreeNeedPt(fonts, text, lay.blockW[i], bodySize))
		}
	}

	// One-row tiers share a height; a tier whose blocks wrap is taller.
	single, sum := 0.0, 0.0
	for i, tier := range vals.Tiers {
		need := driverTreeNeedPt(fonts, lay.labelText[i], lay.labelW, headerSize)
		switch {
		case len(tier.Components) > 0:
			rows, _ := archStackBlockRows(len(tier.Components))
			need = math.Max(need, 2*archStackBlockPadPt+float64(rows)*blockNeed+float64(rows-1)*ctx.Gap(archStackBlockGapPt))
			if rows > 1 {
				lay.tierH[i] = math.Ceil(need)
				continue
			}
		case tier.Description != "":
			desc := archStackText(pptx.ConvertMarkdownEmphasis(tier.Description), bodySize, false, "l", 0)
			need = math.Max(need, driverTreeNeedPt(fonts, desc, lay.areaW, bodySize))
		}
		single = math.Max(single, math.Ceil(need))
	}
	for i := range lay.tierH {
		if lay.tierH[i] == 0 {
			lay.tierH[i] = single
		}
		sum += lay.tierH[i]
	}
	// A stack taller than the content area keeps its proportions.
	gaps := float64(n-1)*archStackTierGapPt + float64(2*n)*archStackSeamPt + 1
	if avail := contentH - gaps; contentH > 0 && sum > avail && avail > 0 {
		for i := range lay.tierH {
			lay.tierH[i] = math.Floor(lay.tierH[i] * avail / sum)
		}
	}
	for i, tier := range vals.Tiers {
		if rows, _ := archStackBlockRows(len(tier.Components)); len(tier.Components) > 0 {
			lay.blockH[i] = (lay.tierH[i] - 2*archStackBlockPadPt - float64(rows-1)*ctx.Gap(archStackBlockGapPt)) / float64(rows)
		}
	}
	return lay
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

// expandComponents renders the stack as bands holding one block per component.
func (a *archStack) expandComponents(ctx ExpandContext, vals *ArchStackValues, ovr *ArchStackOverrides, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	bodySize := ResolveSize(ovr.BodySize, scaleDenseBodyPt)
	lay := layoutArchStackComponents(ctx, vals, ovr)
	n := len(vals.Tiers)

	// Columns: the tier's accent bar, its label, its blocks, the band's right
	// padding, then a gap and a band per rail.
	padW := archStackBlockPadPt - archStackSubGridInsetPt
	cols := []float64{archStackTierBarPt, lay.labelW, lay.areaW, padW}
	for range vals.SideRails {
		cols = append(cols, archStackRailGapPt, lay.railW)
	}
	colsJSON, _ := json.Marshal(cols)

	railTone := inactiveTintTone(baseAccent)
	var rows []jsonschema.GridRowInput
	for i, tier := range vals.Tiers {
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		tierH := lay.tierH[i]

		// Content row: what sits on the band. A nested grid is drawn after
		// every shape of this grid, so it lands on top of the band below.
		content := []*jsonschema.GridCellInput{{}, {}} // accent bar and label columns: the band below draws both
		switch {
		case len(tier.Components) > 0:
			blockRows, perRow := archStackBlockRows(len(tier.Components))
			sub := &jsonschema.ShapeGridInput{
				Columns: json.RawMessage(fmt.Sprintf("%d", perRow)),
				Gap:     ctx.Gap(archStackBlockGapPt),
			}
			for r := 0; r < blockRows; r++ {
				var blocks []*jsonschema.GridCellInput
				for k := r * perRow; k < min((r+1)*perRow, len(tier.Components)); k++ {
					blocks = append(blocks, &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"bg1"`),
						Line:     noLine,
						Text:     lay.blockText[i][k],
					}})
				}
				sub.Rows = append(sub.Rows, jsonschema.GridRowInput{Cells: blocks})
			}
			content = append(content, &jsonschema.GridCellInput{
				Grid:      sub,
				MaxHeight: tierH - 2*(archStackBlockPadPt-archStackSubGridInsetPt),
			})
		case strings.TrimSpace(tier.Description) != "":
			content = append(content, &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
				Columns: json.RawMessage("1"),
				Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(`"none"`),
					Text:     archStackText(pptx.ConvertMarkdownEmphasis(tier.Description), bodySize, false, "l", 0),
				}}}}},
			}})
		}
		if i == 0 {
			if len(content) == 2 {
				content = append(content, &jsonschema.GridCellInput{})
			}
			content = append(content, &jsonschema.GridCellInput{}) // band padding column
			for j, rail := range vals.SideRails {
				railCell := &jsonschema.GridCellInput{
					RowSpan: 2*n - 1,
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     railTone.fillJSON(),
						Line:     noLine,
						Text:     buildArchStackRailContent(rail),
					},
				}
				if co, ok := cellOverrides[n+j].(*ArchStackCellOverride); ok {
					applyCellTextOverride(railCell, co)
				}
				content = append(content, &jsonschema.GridCellInput{RowSpan: 2*n - 1}, railCell)
			}
		}
		rows = append(rows, jsonschema.GridRowInput{Cells: content, MinHeight: tierH, MaxHeight: tierH})

		// Band row: a thin spacer (the gap to the next tier) whose shapes
		// bleed up over the content row. The band's text is the tier name,
		// held to the label column by its right inset.
		spacerH := archStackTierGapPt
		if i == n-1 {
			spacerH = archStackSeamPt
		}
		over := func(cell *jsonschema.GridCellInput) *jsonschema.GridCellInput {
			cell.BleedTop = tierH + archStackSeamPt
			cell.InsetBottom = spacerH
			return cell
		}
		band := over(&jsonschema.GridCellInput{
			ColSpan: 3,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     neutralFillJSON(NeutralTint8),
				Line:     noLine,
				Text:     archStackText(pptx.ConvertMarkdownEmphasis(tier.Label), headerSize, true, "l", lay.areaW+padW+defaultShapeInsetLRPt),
			},
		})
		bar := over(&jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"` + accent + `"`),
			Line:     noLine,
		}})
		if co, ok := cellOverrides[i].(*ArchStackCellOverride); ok {
			applyCellTextOverride(band, co)
			if co.AccentBar {
				// The emphasised tier's bar is a point wider, as in the
				// description layout (3pt -> 4pt).
				bar.BleedLeft = 1
			}
		}
		rows = append(rows, jsonschema.GridRowInput{Cells: []*jsonschema.GridCellInput{bar, band}, MinHeight: spacerH, MaxHeight: spacerH})
	}

	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		ColGap:        archStackSeamPt,
		RowGap:        archStackSeamPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}
