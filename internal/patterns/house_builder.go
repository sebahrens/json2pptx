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

// ---------------------------------------------------------------------------
// House builder — the one layout the strategy-house pattern and the native
// house_diagram both draw (go-slide-creator-vlef3, -bjxb9, -x25dq)
// ---------------------------------------------------------------------------
//
// A house is a gabled roof over a stack of levels:
//
//	        /\
//	   ____/  \____      roof: one gable pentagon in the slide's accent,
//	  |  objective |      the objective (and any badges) in its eaves band
//	  |____________|
//	  [ beam        ]    band level (optional)
//	  [P1][P2][P3][P4]   pillar level
//	  [ enablers    ]    band level
//	  [ A ][ B ][ C ]    band level split into cells
//
// The pattern and the native diagram used to be two builders: the pattern
// stacked five rectangles under a flat grey strip, the diagram drew a shallow
// triangle over pastel pillars in four hues. BuildHouse is the single source
// of the roof geometry, the level heights, the fills and the text styling;
// each caller only maps its own input onto a HouseModel and places the
// returned grid.

// HouseCell is one box of a house level.
type HouseCell struct {
	Title string
	Body  []string
}

// HouseLevelKind says how a level's cells are drawn.
type HouseLevelKind int

const (
	// HouseBand is a structural band: one full-width cell, or a row of
	// equal boxes when the level carries several cells.
	HouseBand HouseLevelKind = iota
	// HousePillars is a row of pillar columns: accent title over bullets on
	// a neutral surface, under a thin accent rule.
	HousePillars
)

// HouseLevel is one horizontal level under the roof.
type HouseLevel struct {
	Kind  HouseLevelKind
	Cells []HouseCell
	// OverrideIndex is the cell_overrides index of the level's first cell;
	// the following cells take the following indices. Negative: no overrides.
	OverrideIndex int
}

// HouseModel is the content of a house, top to bottom.
type HouseModel struct {
	Roof   string
	Badges []string
	Levels []HouseLevel
	// RoofOverrideIndex and BadgeOverrideIndex are the cell_overrides
	// indices of the objective and of the badge line (negative: none).
	RoofOverrideIndex  int
	BadgeOverrideIndex int
}

// HouseStyle carries what a house takes from its surroundings.
type HouseStyle struct {
	// Fonts are the theme typefaces the text is measured in.
	Fonts pptx.ThemeFonts
	// Accent is the house's one accent: the roof fill, and each pillar's
	// rule and title unless PillarAccent says otherwise.
	Accent string
	// PillarAccent, when set, picks pillar i's accent (cell_accent_mode).
	PillarAccent func(i int) string
	// PillarSurface is the pillar fill; empty takes the neutral 4% step.
	PillarSurface json.RawMessage
	// BandFill is the fill of a band level (beam, foundation); empty takes
	// the accent's Lighter 80% swatch, the tonal system's content tone.
	BandFill json.RawMessage
	// HeaderPt and BodyPt are the title and bullet sizes; BandPt is the band
	// label size (default: two points under the header).
	HeaderPt, BodyPt, BandPt float64
	// ColGapPt and RowGapPt separate cells and levels.
	ColGapPt, RowGapPt float64
	// Override, when set, applies a per-cell override to the cell with the
	// given cell_overrides index before it is measured.
	Override func(index int, cell *jsonschema.GridCellInput, accent string)
}

// HouseLayout is a laid-out house.
type HouseLayout struct {
	// Grid is the house as a shape grid: one pinned row per level.
	Grid *jsonschema.ShapeGridInput
	// HeightPt is the height the house takes: its rows plus the row gaps.
	HeightPt float64
	// NeedPt is the least height at which every text keeps its written size
	// under the flattest gable drawn.
	NeedPt float64
	// InkPt is the height of the text alone, summed over the levels.
	InkPt float64
	// RoofRisePt is the height of the gable above the eaves band.
	RoofRisePt float64
	// RoofFlattened reports that the gable was flattened below the minimum
	// pitch to keep the levels' text at its written size.
	RoofFlattened bool
	// Tight reports that NeedPt exceeds the height available: the levels then
	// share the height in proportion to their text, which is written smaller.
	Tight bool
}

const (
	// houseColGapPt and houseRowGapPt are the default gutters.
	houseColGapPt = 8.0
	houseRowGapPt = 6.0
	// houseBandPadPt is the breathing room a band gets around its text.
	houseBandPadPt = 10.0
	// houseRoofPadPt is the breathing room of the roof's eaves band when it
	// holds the objective alone. With the gable above it the roof already has
	// weight, and a band holding the badge line too takes none: every point
	// of solid accent that carries no text makes the roof a larger empty
	// block (SPARSE_FILL measures the band).
	houseRoofPadPt = 8.0
	// houseRoofPitch is the gable's rise as a share of the house width: one
	// in six of the half-span, the pitch at which a wide shallow pentagon
	// still reads as a roof rather than as a banner with a crease.
	houseRoofPitch = 1.0 / 12
	// houseRoofMinPitch is the flattest gable that still reads as a roof; a
	// house short of height keeps it and gives up its breathing room first.
	houseRoofMinPitch = 1.0 / 20
	// houseRoofFloorPitch and houseRoofFloorPt are the gable left when the
	// text itself needs the height: readable text outranks the roof.
	houseRoofFloorPitch = 1.0 / 40
	houseRoofFloorPt    = 8.0
	// houseRoofMaxShare caps the gable at a share of the height available.
	houseRoofMaxShare = 0.22
	// houseRoofSteepPitch and houseRoofSteepMaxShare are the gable a house
	// with height to spare rises to once its text, its breathing room and its
	// pillars have theirs: one in four of the half-span, at most 30% of the
	// height available.
	houseRoofSteepPitch    = 1.0 / 8
	houseRoofSteepMaxShare = 0.30
	// housePillarGrow is how far a pillar row may grow past its text to use
	// spare height; beyond it the columns read as empty boxes.
	housePillarGrow = 1.2
	// HouseMaxLevelCells is the most cells a split band level holds.
	HouseMaxLevelCells = 5
)

// houseColumns returns the column count every level's cells divide evenly:
// the least common multiple of the level cell counts.
func houseColumns(levels []HouseLevel) int {
	cols := 1
	for _, l := range levels {
		n := max(len(l.Cells), 1)
		cols = cols / gcdInt(cols, n) * n
	}
	return cols
}

func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// HouseColumnsFit reports whether levels with these cell counts share one
// column grid.
func HouseColumnsFit(counts ...int) bool {
	cols := 1
	for _, n := range counts {
		n = max(n, 1)
		cols = cols / gcdInt(cols, n) * n
		if cols > shapegrid.MaxColumns {
			return false
		}
	}
	return true
}

// BuildHouse lays a house out widthPt wide in availPt of height. The roof's
// eaves band and every band level are pinned to their text; pillar rows are
// pinned to their tallest column and grow a little into spare height; the
// gable takes its pitch from the width and gives height back when the levels
// need it.
func BuildHouse(m HouseModel, st HouseStyle, widthPt, availPt float64) (*HouseLayout, error) { //nolint:gocognit,gocyclo // one pass per level kind
	if st.ColGapPt <= 0 {
		st.ColGapPt = houseColGapPt
	}
	if st.RowGapPt <= 0 {
		st.RowGapPt = houseRowGapPt
	}
	if st.HeaderPt <= 0 {
		st.HeaderPt = sizeHeaderPt
	}
	if st.BodyPt <= 0 {
		st.BodyPt = scaleBodyPt
	}
	if st.BandPt <= 0 {
		st.BandPt = st.HeaderPt - 2
	}
	if st.Accent == "" {
		st.Accent = "accent1"
	}
	if len(st.PillarSurface) == 0 {
		st.PillarSurface = neutralFillJSON(NeutralTint4)
	}
	if len(st.BandFill) == 0 {
		st.BandFill = tonalLighter(st.Accent, TonalLighterContent).fillJSON()
	}
	cols := houseColumns(m.Levels)
	if cols > shapegrid.MaxColumns {
		return nil, fmt.Errorf("house: levels with %s cells cannot share one column grid; use cell counts that divide a common number of at most %d", houseCountList(m.Levels), shapegrid.MaxColumns)
	}
	unitW := (widthPt - float64(cols-1)*st.ColGapPt) / float64(cols)
	cellWidth := func(n int) float64 {
		span := cols / max(n, 1)
		return math.Max(float64(span)*unitW+float64(span-1)*st.ColGapPt, 1)
	}
	override := func(idx int, cell *jsonschema.GridCellInput, accent string) {
		if st.Override != nil && idx >= 0 {
			st.Override(idx, cell, accent)
		}
	}

	type levelRow struct {
		cells  []*jsonschema.GridCellInput
		need   float64 // text height at its written size
		pad    float64 // breathing room when the height allows it
		margin float64 // top + bottom text margin the cells are written with
		pillar bool
	}
	var (
		roofText                 json.RawMessage
		roofBar                  *jsonschema.AccentBarInput
		roofBand, roofMargin     float64
		rows                     []levelRow
		lean, pads, ink, roofPad float64
	)
	gaps := 0.0
	// measure builds the roof text and every level at a band text margin of
	// padPt (0: the uniform margin) and sums what they need.
	measure := func(padPt float64) {
		bandMargin := 2*defaultShapeInsetTBPt - rowPadTrimPt(padPt)
		// Roof: the objective, under the badge line when there is one.
		// A roof with nothing to say is the bare gable.
		roofText, roofBar, roofBand, roofMargin, roofPad = nil, nil, 0, bandMargin, houseRoofPadPt
		if len(m.Badges) > 0 {
			roofPad = 0
		}
		if strings.TrimSpace(m.Roof) != "" || len(m.Badges) > 0 {
			roofText, roofBar = buildHouseRoofText(m, st, override)
			roofText = houseTextPad(roofText, padPt)
			roofBand = houseNeedPt(st.Fonts, roofText, widthPt)
		}

		rows = make([]levelRow, 0, len(m.Levels))
		for _, l := range m.Levels {
			n := len(l.Cells)
			if n == 0 {
				continue
			}
			w := cellWidth(n)
			span := cols / n
			lr := levelRow{pillar: l.Kind == HousePillars, pad: houseBandPadPt, margin: bandMargin}
			if lr.pillar {
				// A pillar keeps the uniform margin: it is a column of the
				// house, a plain panel under the beam or the roof, and takes
				// an accent rule along its top edge only where cell_overrides
				// asks for one (go-slide-creator-mot7a).
				lr.pad, lr.margin = cardPadPt, 2*defaultShapeInsetTBPt
			}
			anyBody := false
			for _, c := range l.Cells {
				anyBody = anyBody || len(c.Body) > 0
			}
			for i, c := range l.Cells {
				cell := &jsonschema.GridCellInput{ColSpan: span}
				accent := st.Accent
				if lr.pillar {
					if st.PillarAccent != nil {
						accent = st.PillarAccent(i)
					}
					cell.Shape = &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     st.PillarSurface,
						Text:     buildHousePillarText(c, st.HeaderPt, st.BodyPt, accent, anyBody),
					}
				} else {
					cell.Shape = &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     st.BandFill,
						Text:     buildHouseBandText(c, st.BandPt, st.BodyPt),
					}
				}
				if l.OverrideIndex >= 0 {
					override(l.OverrideIndex+i, cell, accent)
				}
				if !lr.pillar {
					cell.Shape.Text = houseTextPad(cell.Shape.Text, padPt)
				}
				lr.need = math.Max(lr.need, houseNeedPt(st.Fonts, cell.Shape.Text, w))
				lr.cells = append(lr.cells, cell)
			}
			rows = append(rows, lr)
		}

		gaps = float64(len(rows)) * st.RowGapPt
		lean, pads = roofBand+gaps, 0.0
		if roofBand > 0 {
			pads = roofPad
		}
		ink = math.Max(roofBand-roofMargin, 0)
		for _, lr := range rows {
			lean += lr.need
			pads += lr.pad
			ink += math.Max(lr.need-lr.margin, 0)
		}
	}

	// Heights. Every level needs its text; what is left over is spent, in
	// order, on a gable at the minimum pitch, on breathing room around the
	// text, on the designed pitch, on slightly taller pillars and on a steeper
	// gable.
	// A house short of height gives them back in the reverse order. Before the
	// gable goes below the minimum pitch, the roof's eaves band and the band
	// levels give up text margin (rowPadStepsPt): a house on a short content
	// area is drawn with slimmer bands under a roof that still reads as one
	// (go-slide-creator-vg73u). Only then is the gable flattened, and last the
	// pillars squeezed.
	floorRise := math.Max(math.Round(widthPt*houseRoofFloorPitch), houseRoofFloorPt)
	minRise := math.Max(math.Round(widthPt*houseRoofMinPitch), floorRise)
	rise := math.Max(math.Round(widthPt*houseRoofPitch), minRise)
	for _, padPt := range append([]float64{0}, rowPadStepsPt...) {
		measure(padPt)
		if availPt <= 0 || availPt-lean >= minRise {
			break
		}
	}
	layout := &HouseLayout{NeedPt: lean + floorRise, InkPt: ink}
	padShare, spare := 1.0, 0.0
	if availPt > 0 {
		rise = math.Max(math.Min(rise, math.Round(availPt*houseRoofMaxShare)), minRise)
		switch slack := availPt - lean; {
		case slack >= pads+rise:
			spare = slack - pads - rise
		case slack >= pads+minRise:
			rise = math.Floor(slack - pads)
		case slack >= minRise && pads > 0:
			rise, padShare = minRise, (slack-minRise)/pads
		case slack >= floorRise:
			rise, padShare = math.Floor(slack), 0
			layout.RoofFlattened = true
		default:
			rise, padShare = floorRise, 0
			layout.RoofFlattened, layout.Tight = true, true
		}
	}
	if roofBand > 0 {
		roofBand += math.Floor(roofPad * padShare)
	}
	for i := range rows {
		rows[i].need += math.Floor(rows[i].pad * padShare)
	}
	if spare > 0 {
		pillars := 0
		for _, lr := range rows {
			if lr.pillar {
				pillars++
			}
		}
		for i := range rows {
			if !rows[i].pillar {
				continue
			}
			grow := math.Min(math.Floor(spare/float64(pillars)), math.Round(rows[i].need*(housePillarGrow-1)))
			rows[i].need += grow
			spare -= grow
			pillars--
		}
		// Height still spare goes into the roof: a steeper gable, up to the
		// steep pitch, before the house is left short in its area
		// (go-slide-creator-yhzxt).
		if steep := math.Min(math.Round(widthPt*houseRoofSteepPitch), math.Round(availPt*houseRoofSteepMaxShare)); steep > rise {
			rise += math.Min(math.Floor(spare), steep-rise)
		}
	}
	layout.RoofRisePt = rise

	// A house that does not fit keeps every row's text margin and shares what
	// is left in proportion to each row's text, so the roof, the bands and
	// the pillars are all written a little smaller instead of the pillars
	// alone being crushed between full-size bands.
	if layout.Tight && ink > 0 {
		squeeze := math.Max(availPt-rise-(lean-ink), 0) / ink
		shrink := func(h, margin float64) float64 {
			margin = math.Min(margin, h)
			return math.Max(math.Floor(margin+(h-margin)*squeeze), 1)
		}
		if roofBand > 0 {
			roofBand = shrink(roofBand, roofMargin)
		}
		for i := range rows {
			rows[i].need = shrink(rows[i].need, rows[i].margin)
		}
	}

	roofH := rise + roofBand
	roofCell := &jsonschema.GridCellInput{
		ColSpan:   cols,
		AccentBar: roofBar,
		Shape: &jsonschema.ShapeSpecInput{
			// An up arrow whose shaft is the full width is a gable
			// pentagon — one native shape, no seam between gable and eaves —
			// and its text rectangle is the eaves band under the slope.
			Geometry:    "upArrow",
			Adjustments: map[string]int64{"adj1": 100000, "adj2": houseRoofAdj(rise, widthPt, roofH)},
			Fill:        json.RawMessage(fmt.Sprintf("%q", st.Accent)),
			Text:        roofText,
			// The band is sized to the text; growing the text into the gable
			// height (type_scale) would push it out of the band.
			TypeScale: "compact",
		},
	}
	// Every row is pinned: the grid adds nothing and takes nothing.
	gridRows := make([]jsonschema.GridRowInput, 0, len(rows)+1)
	gridRows = append(gridRows, jsonschema.GridRowInput{MinHeight: roofH, MaxHeight: roofH, Cells: []*jsonschema.GridCellInput{roofCell}})
	layout.HeightPt = roofH + gaps
	for _, lr := range rows {
		layout.HeightPt += lr.need
		gridRows = append(gridRows, jsonschema.GridRowInput{MinHeight: lr.need, MaxHeight: lr.need, Cells: lr.cells})
	}

	layout.Grid = &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf("%d", cols)),
		Gap:           st.ColGapPt,
		RowGap:        st.RowGapPt,
		Rows:          gridRows,
		VerticalAlign: GridVerticalAlignDefault,
	}
	return layout, nil
}

// houseUnfittedPt stands in for the height of text the fit search gives up
// on (it stops 400pt past one line): far more than any region holds, so the
// house is laid out as short of height rather than as if the text needed
// nothing.
const houseUnfittedPt = 480.0

// houseNeedPt is the height at which text is written at its size in a shape
// widthPt wide.
func houseNeedPt(fonts pptx.ThemeFonts, text json.RawMessage, widthPt float64) float64 {
	if need := writtenFitHeightPt(fonts, text, widthPt, 0); need > 0 {
		return need
	}
	return houseUnfittedPt
}

// houseRoofAdj is the upArrow head-length adjustment for a gable risePt high
// on a roof widthPt × heightPt: the preset measures it against the shorter
// side.
func houseRoofAdj(risePt, widthPt, heightPt float64) int64 {
	ss := math.Min(widthPt, heightPt)
	if ss <= 0 {
		return 0
	}
	return int64(math.Round(math.Min(risePt/ss, heightPt/ss) * 100000))
}

func houseCountList(levels []HouseLevel) string {
	parts := make([]string, 0, len(levels))
	for _, l := range levels {
		if len(l.Cells) > 1 {
			parts = append(parts, fmt.Sprintf("%d", len(l.Cells)))
		}
	}
	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type houseParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
	Bullet  bool    `json:"bullet,omitempty"`
}

type houseTextObj struct {
	Paragraphs    []houseParagraph `json:"paragraphs"`
	Align         string           `json:"align"`
	VerticalAlign string           `json:"vertical_align"`
}

// houseTextPad sets a band's top and bottom text margin to padPt
// (rowPadStepsPt); 0 leaves the uniform margin.
func houseTextPad(text json.RawMessage, padPt float64) json.RawMessage {
	top, bottom := rowPadInsets(padPt, 0)
	if top == nil || len(text) == 0 {
		return text
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(text, &obj); err != nil || obj == nil {
		return text
	}
	obj["inset_top"], obj["inset_bottom"] = marshalRaw(*top), marshalRaw(*bottom)
	out, err := json.Marshal(obj)
	if err != nil {
		return text
	}
	return out
}

func houseText(paras []houseParagraph, align, vAlign string) json.RawMessage {
	data, _ := json.Marshal(houseTextObj{Paragraphs: paras, Align: align, VerticalAlign: vAlign})
	return data
}

// houseBadgeSeparator joins roof badges on one line.
const houseBadgeSeparator = "   ·   "

// buildHouseRoofText is the roof's text: the badge line, when there is one,
// over the objective. The two carry separate cell_overrides indices, so each
// is overridden on its own before they are joined.
func buildHouseRoofText(m HouseModel, st HouseStyle, override func(int, *jsonschema.GridCellInput, string)) (json.RawMessage, *jsonschema.AccentBarInput) {
	type textObj struct {
		Paragraphs    []json.RawMessage `json:"paragraphs"`
		Align         string            `json:"align"`
		VerticalAlign string            `json:"vertical_align"`
	}
	part := func(idx int, p houseParagraph) (textObj, *jsonschema.AccentBarInput) {
		cell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Text: houseText([]houseParagraph{p}, "ctr", "ctr")}}
		override(idx, cell, st.Accent)
		var obj textObj
		_ = json.Unmarshal(cell.Shape.Text, &obj)
		return obj, cell.AccentBar
	}
	var paras []json.RawMessage
	if len(m.Badges) > 0 {
		badges, _ := part(m.BadgeOverrideIndex, houseParagraph{
			Content: strings.Join(m.Badges, houseBadgeSeparator), Size: scaleBodyPt, Bold: true, Color: "lt1", Align: "ctr",
		})
		paras = append(paras, badges.Paragraphs...)
	}
	// The objective's override sets the roof's alignment and anchor; its
	// accent bar becomes a thin rule under the eaves (a bar across the top of
	// a gable would float above the slope).
	objective, bar := part(m.RoofOverrideIndex, houseParagraph{
		Content: pptx.ConvertMarkdownEmphasis(m.Roof), Size: st.HeaderPt, Bold: true, Color: "lt1", Align: "ctr",
	})
	if bar != nil {
		bar = &jsonschema.AccentBarInput{Position: "bottom", Color: bar.Color, Width: 2}
	}
	objective.Paragraphs = append(paras, objective.Paragraphs...)
	data, _ := json.Marshal(objective)
	return data, bar
}

// buildHousePillarText is a pillar column: an accent title over dk1 bullets.
// topAligned is set when any pillar of the row carries bullets, so a pillar
// without them keeps its title on the same line as its neighbours instead of
// floating in the middle of the column.
func buildHousePillarText(c HouseCell, titlePt, bodyPt float64, accent string, topAligned bool) json.RawMessage {
	paras := []houseParagraph{{Content: pptx.ConvertMarkdownEmphasis(c.Title), Size: titlePt, Bold: true, Color: accent, Align: "ctr"}}
	for _, b := range c.Body {
		paras = append(paras, houseParagraph{Bullet: true, Content: pptx.ConvertMarkdownEmphasis(b), Size: bodyPt, Color: "dk1", Align: "l"})
	}
	if !topAligned {
		return houseText(paras, "ctr", "ctr")
	}
	if len(c.Body) == 0 {
		paras[0].Align = "l"
	}
	return houseText(paras, "l", "t")
}

// buildHouseBandText is a band cell: a bold dk1 label, with any items on one
// line under it.
func buildHouseBandText(c HouseCell, titlePt, bodyPt float64) json.RawMessage {
	paras := []houseParagraph{{Content: pptx.ConvertMarkdownEmphasis(c.Title), Size: titlePt, Bold: true, Color: "dk1", Align: "ctr"}}
	if len(c.Body) > 0 {
		items := make([]string, len(c.Body))
		for i, b := range c.Body {
			items[i] = pptx.ConvertMarkdownEmphasis(b)
		}
		paras = append(paras, houseParagraph{Content: strings.Join(items, " · "), Size: bodyPt, Color: "dk1", Align: "ctr"})
	}
	return houseText(paras, "ctr", "ctr")
}
