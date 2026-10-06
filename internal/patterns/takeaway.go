package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// ---------------------------------------------------------------------------
// The takeaway component (go-slide-creator-7b5o6).
//
// One editorial device carries every "so what" in the engine: the slide
// takeaway band (internal/generator/takeaway_note.go), chart-insights-split's
// so_what, exec-summary's bottom_line, metric-list's callout and the pattern /
// compose envelope callout. Four surfaces used to draw four different boxes —
// a peach band with a saturated outline, a chevron "BOTTOM LINE" flag, a solid
// accent banner and a grey panel — for the same job.
//
// The spec is the consultant + designer convergence of 2026-09-28:
//
//   - no stroke, and no fill by default;
//   - a flush 3pt accent bar on the left, the full band height;
//   - 14pt bold text in the theme's dk1 ink, 12pt from the bar, top-anchored,
//     budgeted to two lines;
//   - the content width, never a full bleed;
//   - at least 16pt of air above it (and 12pt to the footer / source line,
//     which the chrome frame owns for the slide band).
//
// emphasis "subtle" adds a 5% neutral tint behind the text; "strong" makes the
// band a solid accent with measured-contrast ink. Those are the only variants.
// ---------------------------------------------------------------------------

// Takeaway geometry and type, in points. The generator's slide band reads the
// same constants so the two renderers cannot drift.
const (
	TakeawayBarPt        = 3.0                       // flush accent bar width
	TakeawayTextInsetPt  = 12.0                      // bar → text
	TakeawaySizePt       = tokens.TypeScaleSubheadPt // text size
	TakeawayInk          = "dk1"
	TakeawayMaxLines     = 2
	TakeawayGapAbovePt   = 16.0 // air between the content above and the band
	TakeawayGapBelowPt   = 12.0 // air between the band and the footer / source line
	TakeawayPadPt        = 2.0  // top / bottom text inset on an unfilled band
	TakeawayFilledPadPt  = 6.0  // top / bottom / right inset when the band is filled
	TakeawaySubtleTint   = 5.0  // % opacity of the neutral tint behind "subtle"
	TakeawayBandPadPt    = 8.0  // top / bottom text inset of the default band
	takeawaySafetyPt     = 2.0  // slack so rounding never pushes text into autofit
	takeawayMinBandWidth = 40.0 // below this the bar column cannot be expressed
)

// Takeaway emphasis values. The empty string is the default: the band.
const (
	TakeawayEmphasisNone   = ""
	TakeawayEmphasisBar    = "bar"
	TakeawayEmphasisSubtle = "subtle"
	TakeawayEmphasisStrong = "strong"
)

// TakeawaySpec describes one takeaway band.
type TakeawaySpec struct {
	// Text is the band's content. Inline <b>/<i> markup is passed through.
	Text string
	// Accent is the bar colour (a scheme name); empty means accent1.
	Accent string
	// Emphasis is "", "subtle" or "strong".
	Emphasis string
	// SizePt overrides TakeawaySizePt (a narrow column may step down a point).
	SizePt float64
	// Italic sets the text italic; the band is always bold.
	Italic bool
}

func (s TakeawaySpec) size() float64 {
	if s.SizePt > 0 {
		return s.SizePt
	}
	return TakeawaySizePt
}

func (s TakeawaySpec) accent() string {
	if s.Accent != "" {
		return s.Accent
	}
	return "accent1"
}

// band reports the default look: one filled band, no bar column.
func (s TakeawaySpec) band() bool { return s.Emphasis == TakeawayEmphasisNone }

func (s TakeawaySpec) pad() float64 {
	switch s.Emphasis {
	case TakeawayEmphasisNone:
		return TakeawayBandPadPt
	case TakeawayEmphasisSubtle, TakeawayEmphasisStrong:
		return TakeawayFilledPadPt
	}
	return TakeawayPadPt
}

// textWidthPt is the width the band's text wraps at: the band minus its side
// insets, or — for the bar variants — minus the bar, the bar-to-text inset
// and (when filled) the right inset.
func (s TakeawaySpec) textWidthPt(bandWidthPt float64) float64 {
	if s.band() {
		return math.Max(bandWidthPt-2*TakeawayTextInsetPt, 1)
	}
	right := 0.0
	if s.Emphasis == TakeawayEmphasisSubtle || s.Emphasis == TakeawayEmphasisStrong {
		right = TakeawayFilledPadPt
	}
	return math.Max(bandWidthPt-TakeawayBarPt-TakeawayTextInsetPt-right, 1)
}

// TakeawayLines returns how many lines the band's text wraps to in a band
// bandWidthPt wide.
func TakeawayLines(ctx ExpandContext, s TakeawaySpec, bandWidthPt float64) int {
	// paragraphLines subtracts the uniform side insets from the width it is
	// handed; add them back so it wraps at the band's real text width.
	return paragraphLines(ctx, sizedPara{text: s.Text, sizePt: s.size(), bold: true}, s.textWidthPt(bandWidthPt)+2*sizingInsetLRPt)
}

// TakeawayBandHeightPt is the band's height (without the air above it) for
// text wrapped in a band bandWidthPt wide.
func TakeawayBandHeightPt(ctx ExpandContext, s TakeawaySpec, bandWidthPt float64) float64 {
	lines := max(TakeawayLines(ctx, s, bandWidthPt), 1)
	return math.Ceil(float64(lines)*s.size()*sizingLineSpacing + 2*s.pad() + takeawaySafetyPt)
}

// SubGridInsetPt is the inset the renderer applies on every side of a nested
// sub-grid (cmd/json2pptx subGridInsetEMU). A band rendered as a sub-grid
// cell loses it on all four sides, so its row claims it back.
const SubGridInsetPt = 4.0

// TakeawaySpacerPt is the extra air a host grid must add above the band so
// the gap from the content above reaches TakeawayGapAbovePt, given the gap
// the host already leaves between rows and the sub-grid's own top inset.
func TakeawaySpacerPt(hostRowGapPt float64) float64 {
	return math.Max(TakeawayGapAbovePt-hostRowGapPt-SubGridInsetPt, 0)
}

// TakeawayRow builds the grid row that carries the band at the bottom of a
// host grid: a nested grid of [spacer] over [bar | text], spanning colSpan
// host columns. cellWidthPt is the width of the host cell the row spans and
// hostRowGapPt the host grid's row gap, so the row can top the air above up
// to 16pt.
//
// The row is an auto row floored at its measured height, not a min = max
// pin: a max on any row switches the host grid from stretching its rows to
// content-sized rows, which shrank auto-height hosts (numbered-step-strip)
// to their estimates.
func TakeawayRow(ctx ExpandContext, s TakeawaySpec, colSpan int, cellWidthPt, hostRowGapPt float64) jsonschema.GridRowInput {
	bandW := cellWidthPt - 2*SubGridInsetPt
	bandPt := TakeawayBandHeightPt(ctx, s, bandW)
	spacerPt := TakeawaySpacerPt(hostRowGapPt)
	return jsonschema.GridRowInput{
		AutoHeight: true, MinHeight: TakeawayRowHeightPt(ctx, s, cellWidthPt, hostRowGapPt),
		Cells: []*jsonschema.GridCellInput{{
			ColSpan: max(colSpan, 1),
			Grid:    TakeawayGrid(ctx, s, bandW, spacerPt, bandPt),
		}},
	}
}

// TakeawayRowHeightPt is the height the takeaway will claim in its host
// (TakeawayRows, or TakeawayRow for a bar variant), for hosts that budget
// their rows before building them. For the band it counts the spacer row and
// the host gap between the spacer and the band.
func TakeawayRowHeightPt(ctx ExpandContext, s TakeawaySpec, cellWidthPt, hostRowGapPt float64) float64 {
	if s.band() {
		h := TakeawayBandHeightPt(ctx, s, cellWidthPt)
		if spacer := takeawayBandSpacerPt(hostRowGapPt); spacer > 0 {
			h += spacer + hostRowGapPt
		}
		return h
	}
	return TakeawayBandHeightPt(ctx, s, cellWidthPt-2*SubGridInsetPt) + TakeawaySpacerPt(hostRowGapPt) + 2*SubGridInsetPt
}

// TakeawaySqueezePt is the part of TakeawayRowHeightPt a host grid can give
// up before any text is written shrunk: the bar variants' spacer, sub-grid
// insets and rounding slack. The band's rows are what they measure.
func TakeawaySqueezePt(s TakeawaySpec, hostRowGapPt float64) float64 {
	if s.band() {
		return takeawaySafetyPt
	}
	return TakeawaySpacerPt(hostRowGapPt) + 2*SubGridInsetPt + takeawaySafetyPt
}

// takeawayBandSpacerPt is the spacer row a host adds above the band so the
// air above it reaches TakeawayGapAbovePt: the host leaves one row gap above
// the spacer and one below it.
func takeawayBandSpacerPt(hostRowGapPt float64) float64 {
	return math.Max(TakeawayGapAbovePt-2*hostRowGapPt, 0)
}

// TakeawayRows builds the rows that carry the takeaway at the foot of a host
// grid, spanning colSpan host columns cellWidthPt wide. The default band is a
// cell of the host grid itself — a spacer row, then the band — so it runs
// the full width of the block, flush with the rules and tiles above it; a
// nested grid would stand 4pt in on each side (go-slide-creator-3a1rm). The
// bar variants are the single nested-grid row of TakeawayRow.
//
// The band row is an auto row floored at its measured height, like
// TakeawayRow's.
func TakeawayRows(ctx ExpandContext, s TakeawaySpec, colSpan int, cellWidthPt, hostRowGapPt float64) []jsonschema.GridRowInput {
	if !s.band() {
		return []jsonschema.GridRowInput{TakeawayRow(ctx, s, colSpan, cellWidthPt, hostRowGapPt)}
	}
	span := max(colSpan, 1)
	bandPt := TakeawayBandHeightPt(ctx, s, cellWidthPt)
	band := takeawayBandGrid(ctx, s, 0, bandPt).Rows[0].Cells[0]
	band.ColSpan = span
	rows := []jsonschema.GridRowInput{{AutoHeight: true, MinHeight: bandPt, Cells: []*jsonschema.GridCellInput{band}}}
	if spacer := takeawayBandSpacerPt(hostRowGapPt); spacer > 0 {
		rows = append([]jsonschema.GridRowInput{{MinHeight: spacer, MaxHeight: spacer, Cells: []*jsonschema.GridCellInput{{ColSpan: span}}}}, rows...)
	}
	return rows
}

// TakeawayGrid is the band as a self-contained sub-grid: an optional spacer
// row spacerPt tall, then the band bandPt tall — a flush accent bar column
// TakeawayBarPt wide and the text column beside it.
func TakeawayGrid(ctx ExpandContext, s TakeawaySpec, bandWidthPt, spacerPt, bandPt float64) *jsonschema.ShapeGridInput {
	bandWidthPt = math.Max(bandWidthPt, takeawayMinBandWidth)
	if s.band() {
		return takeawayBandGrid(ctx, s, spacerPt, bandPt)
	}
	barPct := TakeawayBarPt / bandWidthPt * 100
	colsJSON, _ := json.Marshal([]float64{barPct, 100 - barPct})

	accent := s.accent()
	if s.Accent == "" {
		accent = ctx.DefaultAccent()
	}
	textFill := json.RawMessage(`"none"`)
	ink := inkOnLight(ctx, TakeawayInk, 4.5)
	right := 0.0
	switch s.Emphasis {
	case TakeawayEmphasisSubtle:
		tone := fillTone{Color: TakeawayInk, Alpha: TakeawaySubtleTint}
		textFill = tone.fillJSON()
		ink = readableTextOn(ctx, tone, TakeawayInk)
		right = TakeawayFilledPadPt
	case TakeawayEmphasisStrong:
		tone := fillTone{Color: accent}
		textFill = tone.fillJSON()
		ink = readableTextOn(ctx, tone, "lt1")
		right = TakeawayFilledPadPt
	}
	pad := s.pad()
	left := TakeawayTextInsetPt
	text, _ := json.Marshal(takeawayText{
		Paragraphs: []chartInsightsParagraph{{
			Content: strings.TrimSpace(s.Text), Size: s.size(), Bold: true, Italic: s.Italic, Color: ink, Align: "l",
		}},
		Align: "l", VerticalAlign: "t",
		InsetLeft: &left, InsetRight: &right, InsetTop: &pad, InsetBottom: &pad,
	})

	band := jsonschema.GridRowInput{
		MinHeight: bandPt, MaxHeight: bandPt,
		Cells: []*jsonschema.GridCellInput{
			{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"` + accent + `"`),
				Line:     json.RawMessage(`"none"`),
			}},
			{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     textFill,
				Line:     json.RawMessage(`"none"`),
				Text:     text,
			}},
		},
	}
	rows := []jsonschema.GridRowInput{band}
	if spacerPt > 0 {
		rows = []jsonschema.GridRowInput{
			{MinHeight: spacerPt, MaxHeight: spacerPt, Cells: []*jsonschema.GridCellInput{{ColSpan: 2}}},
			band,
		}
	}
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  0.01, // zero means "default gap"; the bar must sit flush
		RowGap:  0.01,
		Rows:    rows,
		// Any slack the host row carries goes ABOVE the band: the band keeps
		// its measured height and its 16pt of air only grows.
		VerticalAlign: "bottom",
	}
}

// takeawayBandInkPct is the ink coverage of the band where dk2 is black: a
// charcoal (dk1 at 85%), dark enough to close the slide and a step off the
// black of the type above it. The 60% structural grey read as a disabled
// control on p-style.
const takeawayBandInkPct = 85

// TakeawayBandTone is the fill of the default band and the ink measured
// against it: dk2 where it carries the brand (a navy, a forest green), dk1
// at 85% where dk2 is black. It is never an accent, so the band can close a
// slide that already spends its one solid accent block.
func TakeawayBandTone(ctx ExpandContext) (fill json.RawMessage, ink string) {
	tone := fillTone{Color: "dk2"}
	if isNearBlack(ctx, "dk2") {
		tone = neutralTone(takeawayBandInkPct)
	}
	return tone.fillJSON(), readableTextOn(ctx, tone, "lt1")
}

// takeawayBandGrid is the default look (go-slide-creator-3a1rm): one band the
// full width of its host, filled with the dark structural neutral, the
// statement bold in the ink measured against it, centred on the band's
// height. No bar, no outline.
func takeawayBandGrid(ctx ExpandContext, s TakeawaySpec, spacerPt, bandPt float64) *jsonschema.ShapeGridInput {
	fill, ink := TakeawayBandTone(ctx)
	pad, side := s.pad(), TakeawayTextInsetPt
	text, _ := json.Marshal(takeawayText{
		Paragraphs: []chartInsightsParagraph{{
			Content: strings.TrimSpace(s.Text), Size: s.size(), Bold: true, Italic: s.Italic, Color: ink, Align: "l",
		}},
		Align: "l", VerticalAlign: "ctr",
		InsetLeft: &side, InsetRight: &side, InsetTop: &pad, InsetBottom: &pad,
	})
	rows := []jsonschema.GridRowInput{{
		MinHeight: bandPt, MaxHeight: bandPt,
		Cells: []*jsonschema.GridCellInput{{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect", Fill: fill, Line: json.RawMessage(`"none"`), Text: text,
			// The band is sized to its measured lines; it is not a sparse
			// cell for grow-to-fill.
			TypeScale: peerTextTypeScale,
		}}},
	}}
	if spacerPt > 0 {
		rows = append([]jsonschema.GridRowInput{{MinHeight: spacerPt, MaxHeight: spacerPt, Cells: []*jsonschema.GridCellInput{{}}}}, rows...)
	}
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		RowGap:  0.01,
		Rows:    rows,
		// Any slack the host row carries goes ABOVE the band.
		VerticalAlign: "bottom",
	}
}

// takeawayText is the band's text object with explicit per-side insets.
type takeawayText struct {
	Paragraphs    []chartInsightsParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
	InsetLeft     *float64                 `json:"inset_left,omitempty"`
	InsetRight    *float64                 `json:"inset_right,omitempty"`
	InsetTop      *float64                 `json:"inset_top,omitempty"`
	InsetBottom   *float64                 `json:"inset_bottom,omitempty"`
}

// TakeawayEmphasisSchema is the overrides schema for a pattern's takeaway band.
func TakeawayEmphasisSchema() *Schema {
	return EnumSchema(TakeawayEmphasisBar, TakeawayEmphasisSubtle, TakeawayEmphasisStrong).WithDescription(
		"Takeaway style: omit for the default (dark neutral band, bold measured-ink text); bar is a flush 3pt accent bar, no fill; subtle adds a 5% neutral tint to it; strong is a solid accent band")
}

func validateTakeawayEmphasis(patternName, e string) error {
	if ValidTakeawayEmphasis(e) {
		return nil
	}
	return &ValidationError{
		Pattern: patternName,
		Path:    "overrides.takeaway_emphasis",
		Code:    "invalid_enum",
		Message: fmt.Sprintf("%s: overrides.takeaway_emphasis must be bar, subtle or strong (omit for the default band); got %q", patternName, e),
	}
}

// ValidTakeawayEmphasis reports whether e is a takeaway emphasis value.
func ValidTakeawayEmphasis(e string) bool {
	switch e {
	case TakeawayEmphasisNone, TakeawayEmphasisBar, TakeawayEmphasisSubtle, TakeawayEmphasisStrong:
		return true
	}
	return false
}
