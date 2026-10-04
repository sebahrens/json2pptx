package patterns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/svggen"
)

// ---------------------------------------------------------------------------
// Shared KPI types and helpers used by kpi-3up, kpi-4up, etc.
// ---------------------------------------------------------------------------

// KPICell is a single KPI cell: a big number, a short caption, and an optional
// sub annotation (a delta/trend value such as "+5%").
// Supports string shorthand: "Big | Small" unmarshals to {big:"Big", small:"Small"}.
//
// Sub accepts the canonical key "sub" or the intuitive aliases delta/trend/change,
// so a KPI delta is rendered instead of being silently dropped.
//
// Icon accepts either a bundled-name string shorthand or a full IconRef object
// (path / url / svg_data / fill / alt / position). See IconRef and IconRefSchema.
//
// Comparator is the reference the number is read against ("vs plan +4 pts",
// "vs PY -2%"): a partner reads a KPI without one as unanchored
// (go-slide-creator-lsfbi). It renders as its own line under the caption and
// delta, in the delta's size and ink, so every card in a row shares one style.
type KPICell struct {
	Big        string   `json:"big"`
	Small      string   `json:"small"`
	Sub        string   `json:"sub,omitempty"`
	Comparator string   `json:"comparator,omitempty"`
	Icon       *IconRef `json:"icon,omitempty"`
}

const (
	// kpiComparatorMaxChars bounds the comparator line ("vs plan +4 pts").
	kpiComparatorMaxChars = 24
	kpiSubMaxChars        = 12
	kpiNupBigMaxChars     = 12
	kpiInlineBigMaxChars  = 8
)

// UnmarshalJSON supports string shorthand "Big | Small" or an object. The object
// canonical keys are {big, small}, but the intuitive aliases {value|number} and
// {label|caption} are also accepted so agents that reach for the natural field
// names ("$127M" as value, "Revenue" as label) succeed instead of silently
// producing empty cells.
func (c *KPICell) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parts := strings.SplitN(s, " | ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("KPICell string must be \"Big | Small\", got %q", s)
		}
		c.Big = parts[0]
		c.Small = parts[1]
		return nil
	}
	var obj struct {
		Big     string `json:"big"`
		Small   string `json:"small"`
		Value   string `json:"value"`
		Number  string `json:"number"`
		Label   string `json:"label"`
		Caption string `json:"caption"`
		Sub     string `json:"sub"`
		Delta   string `json:"delta"`
		Trend   string `json:"trend"`
		Change  string `json:"change"`
		// Comparator and its alias vs name the reference the value is read
		// against ("vs plan +4 pts").
		Comparator string   `json:"comparator"`
		Vs         string   `json:"vs"`
		Icon       *IconRef `json:"icon"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("KPICell must be a string \"Big | Small\" or an object {big, small} (aliases value/number, label/caption, sub/delta/trend/change): %w", err)
	}
	c.Big = firstNonEmpty(obj.Big, obj.Value, obj.Number)
	c.Small = firstNonEmpty(obj.Small, obj.Label, obj.Caption)
	c.Sub = firstNonEmpty(obj.Sub, obj.Delta, obj.Trend, obj.Change)
	c.Comparator = firstNonEmpty(obj.Comparator, obj.Vs)
	c.Icon = obj.Icon
	return nil
}

// firstNonEmpty returns the first non-empty string in vs, or "".
func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

// KPINupValues is the values type for every kpi-Nup / kpi-inline variant: an
// ordered list of KPI cells.
type KPINupValues []KPICell

// UnmarshalJSON accepts the canonical JSON array of cells and, as a tolerance
// affordance, a single bare cell (object or string). A lone object/string is
// wrapped into a one-element slice so that passing one KPI surfaces a clear
// count validation error ("exactly N cells") rather than the cryptic
// "cannot unmarshal object into Go value of type []patterns.KPICell".
func (v *KPINupValues) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var cells []KPICell
		if err := json.Unmarshal(data, &cells); err != nil {
			return err
		}
		*v = cells
		return nil
	}
	// Single object or string shorthand — wrap into a one-element slice.
	var cell KPICell
	if err := json.Unmarshal(data, &cell); err != nil {
		return fmt.Errorf("values must be a JSON array of KPI cells (or a single cell): %w", err)
	}
	*v = KPINupValues{cell}
	return nil
}

// KPIOverrides contains pattern-level overrides common to KPI patterns.
type KPIOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	BigSize        float64 `json:"big_size,omitempty"`
	SmallSize      float64 `json:"small_size,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"` // uniform | alternate | progressive
	// Style picks the container: "open" sets the numbers on the canvas between
	// hairline dividers, "tiles" keeps a tinted card under an accent rule
	// (kpi-inline calls that "tinted" and also takes "solid"). Unset, plain
	// value + caption cells are open, and cells with icons or a semantic /
	// per-cell accent keep their container (go-slide-creator-8zles).
	Style string `json:"style,omitempty"`
}

// KPI container styles (overrides.style).
const (
	kpiStyleOpen   = "open"
	kpiStyleTiles  = "tiles"
	kpiStyleTinted = "tinted" // kpi-inline's name for tiles
	kpiStyleSolid  = "solid"  // kpi-inline only
)

// kpiNupStyles are the overrides.style values kpi-Nup accepts.
var kpiNupStyles = []string{kpiStyleOpen, kpiStyleTiles}

// kpiDefaultOpen reports whether a KPI row with no authored style is drawn
// open: plain value + caption cells in one accent. An icon, a semantic accent
// and a per-cell accent mode all carry meaning the container holds, so those
// rows keep it.
func kpiDefaultOpen(cells []KPICell, ovr *KPIOverrides) bool {
	if ovr != nil && (ovr.SemanticAccent != "" || (ovr.CellAccentMode != "" && ovr.CellAccentMode != "uniform")) {
		return false
	}
	for _, c := range cells {
		if c.Icon != nil && !c.Icon.IsEmpty() {
			return false
		}
	}
	return true
}

// KPICellOverride is an alias for the shared CellOverride struct.
type KPICellOverride = CellOverride

// resolveKPIAccent returns the accent color, honoring the deck-level accent strategy.
func resolveKPIAccent(ovr *KPIOverrides, ctx ExpandContext) string {
	if ovr == nil {
		return ctx.ResolveAccent("", "")
	}
	return ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
}

// resolveKPIBigSize returns the big-number font size, defaulting to the
// 40pt KPI display step (measured down to fit by kpiFitBigSize).
func resolveKPIBigSize(ovr *KPIOverrides) float64 {
	if ovr == nil {
		return scaleKPIPt
	}
	return ResolveSize(ovr.BigSize, scaleKPIPt)
}

// resolveKPISmallSize returns the caption font size, defaulting to 14pt.
func resolveKPISmallSize(ovr *KPIOverrides) float64 {
	if ovr == nil {
		return scaleSubheadPt
	}
	return ResolveSize(ovr.SmallSize, scaleSubheadPt)
}

// validateKPICells validates a slice of KPI cells for a pattern with the given
// name and expected count. siblingHint is the name of the sibling pattern to
// suggest when the count is off by one (e.g. "kpi-4up" when validating kpi-3up).
func validateKPICells(patternName string, cells []KPICell, expectedCount int, siblingHint string, cellOverrides map[int]any) error {
	var errs []error

	// D4: exact count with swap suggestion
	if len(cells) != expectedCount {
		errs = append(errs, newValidationError(patternName, "values", ErrCodeCountMismatch,
			fmt.Sprintf("%s: values must contain exactly %d cells, got %d", patternName, expectedCount, len(cells)),
			ResizeListFix("values", expectedCount)))

		// Reverse-recommend: suggest alternative patterns that accept the actual cell count.
		if swaps := SuggestSwap(Default(), patternName, len(cells), true); len(swaps) > 0 {
			errs = append(errs, ErrWrongPatternFor(patternName, len(cells), swaps))
		}
	}

	// Per-cell validation
	for i, cell := range cells {
		bigPath := fmt.Sprintf("values[%d].big", i)
		if cell.Big == "" {
			errs = append(errs, errRequired(patternName, bigPath))
		} else if runeLen(cell.Big) > kpiNupBigMaxChars {
			errs = append(errs, errMaxLength(patternName, bigPath, kpiNupBigMaxChars, runeLen(cell.Big)))
		}
		smallPath := fmt.Sprintf("values[%d].small", i)
		if cell.Small == "" {
			errs = append(errs, errRequired(patternName, smallPath))
		} else if runeLen(cell.Small) > 40 {
			errs = append(errs, errMaxLength(patternName, smallPath, 40, runeLen(cell.Small)))
		}
		if subLength := runeLen(cell.Sub); subLength > kpiSubMaxChars {
			errs = append(errs, errMaxLength(patternName, fmt.Sprintf("values[%d].sub", i), kpiSubMaxChars, subLength))
		}
		if n := runeLen(cell.Comparator); n > kpiComparatorMaxChars {
			errs = append(errs, errMaxLength(patternName, fmt.Sprintf("values[%d].comparator", i), kpiComparatorMaxChars, n))
		}
		if cell.Icon != nil {
			iconPath := fmt.Sprintf("values[%d].icon", i)
			errs = append(errs, validateIconRef(patternName, iconPath, *cell.Icon)...)
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	if coErr := validateCellOverrideKeys(patternName, cellOverrides, expectedCount, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// kpiCellSchema returns the JSON Schema for a single KPI cell.
// Accepts either the object form {big, small, sub?, icon?} or the shorthand string "Big | Small".
//
// The icon field is polymorphic: it accepts a bundled-name string shorthand
// (e.g. "rocket") or a full IconRef object (path / url / svg_data / fill / alt
// / position).
func kpiCellSchema(bigMaxChars, comparatorMaxChars int) *Schema {
	props := map[string]*Schema{
		"big":   StringSchema(bigMaxChars).WithDescription("The big number (e.g. \"$4.2M\"); the hard character maximum is not a fit guarantee for every card width"),
		"small": StringSchema(40).WithDescription("Short caption (e.g. \"ARR\")"),
		"sub":   StringSchema(kpiSubMaxChars).WithDescription("Optional delta/trend annotation rendered below the number (e.g. \"+5%\"); aliases delta/trend/change"),
		"icon":  IconRefSchema("Optional icon: bundled name string or {name|path|url|svg_data, fill?, alt?, position?} object"),
	}
	shape := "{big, small, sub?, icon?}"
	// comparatorMaxChars 0 means the pattern takes no comparator line (the
	// height-capped kpi-inline bar).
	if comparatorMaxChars > 0 {
		props["comparator"] = StringSchema(comparatorMaxChars).WithDescription("Optional comparator line: the reference the number is read against (e.g. \"vs plan +4 pts\", \"vs PY -2%\"); rendered under the caption in the delta's style, with the line reserved on every card when any card has one; alias vs")
		shape = "{big, small, sub?, comparator?, icon?}"
	}
	return OneOfSchema(
		StringSchema(0).WithDescription("Shorthand: \"Big | Small\" (e.g. \"$4.2M | ARR\")"),
		ObjectSchema(props, []string{"big", "small"}).WithAdditionalProperties(false),
	).WithDescription("KPI cell: string \"Big | Small\" or " + shape)
}

// kpiOverridesSchema returns the JSON Schema for KPI pattern-level overrides.
// style is the pattern's own container-style enum.
func kpiOverridesSchema(style *Schema) *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"style":            style,
			"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"big_size":         NumberSchema(6, 120).WithDescription("Font size for big number in points (kpi-Nup default 40, stepping to 28 then 24 in an area too short for it)"),
			"small_size":       NumberSchema(6, 120).WithDescription("Font size for small caption in points (kpi-Nup default 14, 12 on the smaller value steps and whenever a long value is fitted below 18pt, so the value stays the larger line)"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation: uniform (default, all cells same accent), alternate (base/base+1), progressive (walks accent1-6)").WithDefault("uniform"),
		},
		nil,
	).WithAdditionalProperties(false)
}

// kpiText is one KPI cell's text: the value, its caption, and the optional
// delta and comparator lines ("vs plan +4 pts") in one shared style.
type kpiText struct {
	big       string
	bigSize   float64
	small     string
	smallSize float64
	// padLines are blank caption lines under a caption shorter than the row's
	// longest, so the delta and comparator lines of every cell sit on one
	// baseline.
	padLines   int
	sub        string
	comparator string
	// reserveDelta / reserveComparator keep the line on every cell when any
	// cell of the row carries one.
	reserveDelta, reserveComparator bool
	// valueInk / ink colour the value and the lines under it. Cards write
	// "lt1" for both: the peer-fill and readable-ink passes recolour them for
	// the surface the card ends up on.
	valueInk, ink string
	// tight replaces the uniform 0.5 cm top / bottom text margin with
	// kpiTightInsetPt: the last thing a row gives up before it is refused.
	tight bool
}

// kpiBlankLine is a visually blank line. An empty <a:t> may be collapsed by
// PowerPoint / LibreOffice; a non-breaking space reserves exactly one line.
const kpiBlankLine = " "

// json renders the cell text. It is anchored to the TOP of its box: every
// cell of a row has the same box, so the values share one baseline and the
// captions start on one line whatever their line counts. A centred block rode
// up by half a line for every extra caption line (go-slide-creator-0cy3p).
func (t kpiText) json() json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
		// Figure keeps the value at the size it was fitted to (shapegrid's
		// paragraph "figure").
		Figure bool `json:"figure,omitempty"`
	}
	line := func(content string, size float64) paragraph {
		return paragraph{Content: content, Size: size, Color: t.ink, Align: "ctr"}
	}

	paragraphs := []paragraph{
		{Content: t.big, Size: t.bigSize, Bold: true, Color: t.valueInk, Align: "ctr", Figure: true},
		line(t.small, t.smallSize),
	}
	for i := 0; i < t.padLines; i++ {
		paragraphs = append(paragraphs, line(kpiBlankLine, t.smallSize))
	}
	sub := t.sub
	if sub == "" && t.reserveDelta {
		sub = kpiBlankLine
	}
	if sub != "" {
		paragraphs = append(paragraphs, line(sub, kpiSubSize(t.smallSize)))
	}
	comparator := t.comparator
	if comparator == "" && t.reserveComparator {
		comparator = kpiBlankLine
	}
	if comparator != "" {
		paragraphs = append(paragraphs, line(comparator, kpiSubSize(t.smallSize)))
	}

	var inset *float64
	if t.tight {
		tightPt := kpiTightInsetPt
		inset = &tightPt
	}
	data, _ := json.Marshal(struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
		InsetTop      *float64    `json:"inset_top,omitempty"`
		InsetBottom   *float64    `json:"inset_bottom,omitempty"`
	}{paragraphs, "ctr", "t", inset, inset})
	return data
}

// kpiCaptionPadLines returns, per cell, the blank caption lines that bring its
// caption up to the row's longest (measured at the caption size in each cell's
// own text width). It is all zeros when no cell carries a delta or a
// comparator: nothing sits under the caption to align.
func kpiCaptionPadLines(font string, cells []KPICell, smallSize float64, widthOf func(i int) float64) []int {
	pads := make([]int, len(cells))
	if delta, comparator := kpiReservedSlots(cells); !delta && !comparator {
		return pads
	}
	size := shapegrid.EffectiveTextSizePt(smallSize)
	lines := make([]int, len(cells))
	most := 0
	for i, c := range cells {
		lines[i] = max(1, measuredLines(c.Small, font, false, size, widthOf(i)))
		most = max(most, lines[i])
	}
	for i := range pads {
		pads[i] = most - lines[i]
	}
	return pads
}

// kpiReservedSlots reports whether any card in the row carries a delta and a
// comparator line, so every card reserves the same lines and keeps one
// baseline.
func kpiReservedSlots(cells []KPICell) (delta, comparator bool) {
	for _, c := range cells {
		delta = delta || c.Sub != ""
		comparator = comparator || c.Comparator != ""
	}
	return delta, comparator
}

// kpiSubSize returns the font size for the sub/delta annotation: ~85% of the
// caption size, floored at 8pt so it stays legible.
func kpiSubSize(smallSize float64) float64 {
	s := smallSize * 0.85
	if s < 8 {
		return 8
	}
	return s
}

// ---------------------------------------------------------------------------
// Open strip (go-slide-creator-8zles)
// ---------------------------------------------------------------------------

const (
	// kpiDividerPt is the open strip's vertical hairline.
	kpiDividerPt = 0.75
	// kpiDividerInkPct is the divider's dk1 ink coverage: the tone of the
	// pattern row rules.
	kpiDividerInkPct = 30
	// kpiOpenGapPt is the column gap of an open strip. Its cells are
	// unpainted, so the gap only has to hold the divider: a left accent bar
	// sits 2pt outside its cell, and this gap centres it between two cells.
	kpiOpenGapPt = 2*2 + kpiDividerPt
)

// kpiOpenInks returns the value and caption inks of an open KPI cell, which
// sits on the slide background: the accent when it clears the large-text 3:1
// there (else its minimal in-hue darken), over dk1 captions.
func kpiOpenInks(ctx ExpandContext, accent string) (valueInk, ink string) {
	valueInk = accent
	if v, ok := accentInkOn(ctx, accent, fillTone{Color: "lt1"}, svggen.WCAGAALarge); ok {
		valueInk = v
	}
	return valueInk, "dk1"
}

// kpiDivider is the hairline between two open KPI cells: a neutral step of the
// template's own ink, resolved to a colour because an accent bar takes no
// tint. Without a theme it falls back to the ink itself.
func kpiDivider(ctx ExpandContext) *jsonschema.AccentBarInput {
	color := "dk1"
	if c, ok := effectiveFillColor(ctx, neutralTone(kpiDividerInkPct)); ok {
		color = c.Hex()
	}
	return &jsonschema.AccentBarInput{Position: "left", Color: color, Width: kpiDividerPt}
}

// kpiOpenCell is an unpainted KPI cell; every cell after the first carries the
// divider on its left.
func kpiOpenCell(ctx ExpandContext, index int, text json.RawMessage) *jsonschema.GridCellInput {
	gc := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Line:     noLine,
		Text:     text,
	}}
	if index > 0 {
		gc.AccentBar = kpiDivider(ctx)
	}
	return gc
}

// ---------------------------------------------------------------------------
// KPI row layout (go-slide-creator-5lbo, go-slide-creator-uj9zq)
// ---------------------------------------------------------------------------

const (
	// kpiCardGapPt is the gap between KPI cards.
	kpiCardGapPt = 12.0
	// kpiMinBigSize is the floor the big number shrinks to before it may wrap.
	kpiMinBigSize = 16.0
	// Mirrors shapegrid's overlay defaults: 3pt gap, and the left-icon cap of
	// 25% of the card width.
	kpiIconGapPt        = 3.0
	kpiLeftIconMaxWFrac = 0.25
	// Default KPI icon footprint (the icon is an accent, not the headline): a
	// top icon is as tall as the value it sits over and at most 45% of the
	// card width; a left icon at most 40% of the card height / 20% of its
	// width.
	kpiTopIconValueRatio = 1.1
	kpiTopIconHFrac      = 0.28
	kpiTopIconWFrac      = 0.45
	kpiLeftIconHFrac     = 0.4
	kpiLeftIconWFrac     = 0.2
	// kpiBaseCardHeightFrac is the share of the area height used as the
	// card-height ESTIMATE a left icon's footprint is taken from while the
	// value is fitted to its width. It is neither a floor nor a cap on the
	// rendered card: the row is sized to its content.
	kpiBaseCardHeightFrac = 0.70
	// kpiRowSlackPt is the air a row keeps beyond what the writer needs to
	// store its text unshrunk; the text is top-anchored, so it falls under
	// the last line.
	kpiRowSlackPt = 4.0
	// kpiTightInsetPt is the top / bottom text margin of a row that only fits
	// its area without the uniform 0.5 cm one; kpiTightSlackPt its slack.
	kpiTightInsetPt = 6.0
	kpiTightSlackPt = 2.0
	// kpiFitTolerancePt absorbs the rounding between the row's measured
	// height and the area the caller resolved for it.
	kpiFitTolerancePt = 1.0
	// kpiValueLineFrac is the share of a card's text width a value is fitted
	// to. Fitted edge to edge, "EUR 48.2M" kept 3pt of a 168pt line: a
	// renderer that rounds a glyph differently broke it at the space
	// (go-slide-creator-kjrxx).
	kpiValueLineFrac = 0.96
	// kpiValueGrowthRoom is the growth a value must have room for before its
	// row takes part in the grid's type step: the step grows a display figure
	// by 1.2 (shapegrid's composeFigureScale) and a 16pt value to 18pt.
	kpiValueGrowthRoom = 1.2
)

// kpiValueFits reports whether value is one line at sizePt in a card whose
// text is widthPt wide (less for a substituted face, whose metrics are a
// guess). The row's size is chosen against kpiValueLineFrac of that width;
// the finding that a value does not fit at all measures the whole width.
func kpiValueFits(value, font string, sizePt, widthPt float64) bool {
	return measuredLines(value, font, true, sizePt, textfit.AtomicTokenWidthPt(font, widthPt)) <= 1
}

// KPIValueLineBudget is how many characters of a figure one kpi-Nup card of an
// n-KPI row holds on one line on ctx's template: digits at the smallest size
// a value is fitted to, in the default open strip, capped at the hard
// maximum. It is the measure the BODY_TOO_LONG finding applies to a value
// (kpiValueFits over the card's text width), taken with digits, so a value of
// this many digits is never reported and one more is. Capitals and "M" run
// wider than digits, punctuation narrower: the finding quotes the count for
// the value's own glyphs. The fixed 12-character maximum holds for two to
// four KPIs on the shipped templates; five and six hold fewer on the
// wide-faced ones (go-slide-creator-6xgxm).
func KPIValueLineBudget(ctx ExpandContext, n int) int {
	if n < 1 {
		n = 1
	}
	width := kpiCardGeometryWithGap(ctx, n, ctx.Gap(kpiOpenGapPt)).valueWidthPt(nil, "")
	for k := kpiNupBigMaxChars; k > 1; k-- {
		if kpiValueFits(strings.Repeat("0", k), ctx.Theme.BodyFont, kpiMinBigSize, width) {
			return k
		}
	}
	return 1
}

// kpiNupTightFromCount is the KPI count from which a card of a shipped
// template holds fewer digits than the hard maximum; kpiNupNarrowestBudget is
// KPIValueLineBudget on the narrowest shipped template for those counts, the
// numbers the pattern schema states (TestKPIValueBudgetStatedPerCount holds
// them to the measure).
const kpiNupTightFromCount = 5

var kpiNupNarrowestBudget = map[int]int{5: KPIValueFiveMax, 6: KPIValueSixMax}

// kpiNupSteps is the type ladder a KPI row walks when its area is too short
// for the 40pt figure: value and caption, largest first. The last step is the
// minimum — below a 24pt figure over a 12pt caption the row is refused rather
// than drawn with its lines touching (go-slide-creator-uj9zq).
var kpiNupSteps = [][2]float64{{scaleKPIPt, scaleSubheadPt}, {scaleDisplayPt, scaleBodyPt}, {sizeFigurePt, scaleBodyPt}}

// kpiCardGeometry is the size (points) of one KPI card.
type kpiCardGeometry struct {
	wPt, hPt float64
}

// kpiCardGeometryFor estimates the card size for n cards spread across the
// content area with the tile gap. The height is the kpiBaseCardHeightFrac
// estimate; layoutKPIRow sizes the rendered row to its content.
func kpiCardGeometryFor(ctx ExpandContext, n int) kpiCardGeometry {
	return kpiCardGeometryWithGap(ctx, n, ctx.Gap(kpiCardGapPt))
}

func kpiCardGeometryWithGap(ctx ExpandContext, n int, gapPt float64) kpiCardGeometry {
	w, h := contentAreaPt(ctx)
	return kpiCardGeometry{wPt: equalColumnWidthPt(w, n, gapPt), hPt: h * kpiBaseCardHeightFrac}
}

// kpiRowLayout is a KPI row measured at one type step.
type kpiRowLayout struct {
	geo                kpiCardGeometry // card width and the row's final height
	iconPos            string          // default icon position
	bigSize, smallSize float64
	rowPt              float64 // row max_height: the tallest cell's content
	needPt             float64 // rowPt before it was held to the area
	availPt            float64 // height of the area the row sits in
	fits               bool    // needPt fits availPt
	authored           bool    // the sizes are overrides, not a ladder step
	// pinned says a value has no room to grow on its line: the row keeps the
	// sizes it was measured at (see Expand).
	pinned bool
	// valueSize is the size the values are written at: bigSize, the size
	// every value fits one line at — or, when a value does not fit one line
	// even at the kpiMinBigSize floor, the subhead step below it, where the
	// long value has the better chance of staying whole
	// (go-slide-creator-a5ogo). The BODY_TOO_LONG finding quotes it.
	valueSize  float64
	texts      []kpiText
	iconScales []float64 // overlay scale per cell (0 = no icon)
}

// layoutKPIRow sizes a KPI row to its content: every value on one line at one
// shared size, every cell tall enough that the writer stores its text
// unshrunk, and the row as tall as its tallest cell. In an area too short for
// the default sizes it steps down kpiNupSteps; authored sizes are measured as
// given. A row that still does not fit gives up its vertical text margin
// before anything else; fits is false when it is taller than the area even so.
func layoutKPIRow(ctx ExpandContext, cells []KPICell, ovr *KPIOverrides, gapPt float64) kpiRowLayout {
	steps := kpiNupSteps
	authored := ovr != nil && (ovr.BigSize > 0 || ovr.SmallSize > 0)
	if authored {
		steps = [][2]float64{{resolveKPIBigSize(ovr), resolveKPISmallSize(ovr)}}
	}
	var lay kpiRowLayout
	for _, st := range steps {
		lay = measureKPIRow(ctx, cells, gapPt, st[0], st[1], false)
		if lay.fits {
			break
		}
	}
	if !lay.fits {
		last := steps[len(steps)-1]
		lay = measureKPIRow(ctx, cells, gapPt, last[0], last[1], true)
	}
	lay.authored = authored
	return lay
}

// measureKPIRow measures the row at one value / caption size.
func measureKPIRow(ctx ExpandContext, cells []KPICell, gapPt, bigSize, smallSize float64, tight bool) kpiRowLayout {
	insetPt, slackPt := defaultShapeInsetTBPt, kpiRowSlackPt
	if tight {
		insetPt, slackPt = kpiTightInsetPt, kpiTightSlackPt
	}
	_, availPt := contentAreaPt(ctx)
	est := kpiCardGeometryWithGap(ctx, len(cells), gapPt)
	lay := kpiRowLayout{
		iconPos:    est.iconPosition(),
		smallSize:  smallSize,
		availPt:    availPt,
		texts:      make([]kpiText, len(cells)),
		iconScales: make([]float64, len(cells)),
	}
	lay.bigSize = kpiFitBigSize(ctx, cells, bigSize, est, lay.iconPos)
	// A value fitted down below the lead step sits two points or less over a
	// 14pt caption: six long values read at the size of their captions
	// (go-slide-creator-6xgxm). The caption then takes the body step, so the
	// value — written at the size it was fitted to, 16pt at the least
	// (paragraph "figure", go-slide-creator-a5ogo) — stays the larger line.
	if lay.bigSize < scaleLeadPt && smallSize > scaleBodyPt {
		smallSize = scaleBodyPt
		lay.smallSize = smallSize
	}

	font := ctx.Theme.BodyFont
	widthOf := func(i int) float64 { return est.valueWidthPt(cells[i].Icon, lay.iconPos) }
	// A value that does not fit one line at the floor is reported
	// (BODY_TOO_LONG) and would wrap at 16pt; the row is written at the
	// subhead step instead, which the finding states.
	lay.valueSize = lay.bigSize
	if lay.bigSize <= kpiMinBigSize {
		for i, c := range cells {
			if c.Big != "" && !kpiValueFits(c.Big, font, lay.bigSize, widthOf(i)) {
				lay.valueSize = math.Min(lay.bigSize, scaleSubheadPt)
				break
			}
		}
	}
	for i, c := range cells {
		if c.Big != "" && !kpiValueFits(c.Big, font, math.Ceil(lay.bigSize*kpiValueGrowthRoom), widthOf(i)*kpiValueLineFrac) {
			lay.pinned = true
			break
		}
	}
	pads := kpiCaptionPadLines(font, cells, smallSize, widthOf)
	reserveDelta, reserveComparator := kpiReservedSlots(cells)
	textPt := make([]float64, len(cells))
	for i, c := range cells {
		t := kpiText{
			big: c.Big, bigSize: lay.valueSize, small: c.Small, smallSize: smallSize, padLines: pads[i],
			sub: c.Sub, comparator: c.Comparator, reserveDelta: reserveDelta, reserveComparator: reserveComparator,
			valueInk: "lt1", ink: "lt1", tight: tight,
		}
		lay.texts[i] = t
		// The height the writer needs to store this text unshrunk in the
		// cell's text width (a left icon narrows it).
		textPt[i] = writtenFitHeightPt(ctx.themeFonts(), t.json(), widthOf(i)+2*defaultShapeInsetLRPt, 0)
		if textPt[i] <= 0 {
			textPt[i] = 2*insetPt + textBlockHeightPt(font, widthOf(i),
				textParagraph{text: c.Big, size: lay.valueSize, bold: true},
				textParagraph{text: c.Small, size: smallSize})
		}
	}

	// A top icon takes its zone above the text. An authored scale is a share
	// of the card's shorter side, which depends on the row height the icon
	// itself adds to, so the row is settled over a few passes.
	icons := make([]float64, len(cells))
	row := 0.0
	for pass := 0; pass < 4; pass++ {
		next := 0.0
		for i, c := range cells {
			icons[i] = est.topIconPt(c.Icon, lay.iconPos, lay.bigSize, row)
			h := textPt[i]
			if icons[i] > 0 {
				h += icons[i] + 2*kpiIconGapPt
			}
			next = math.Max(next, h)
		}
		if next == row {
			break
		}
		row = next
	}
	lay.needPt = math.Ceil(row + slackPt)
	lay.fits = lay.needPt <= availPt+kpiFitTolerancePt
	lay.rowPt = clampPt(lay.needPt, 0, availPt)
	lay.geo = kpiCardGeometry{wPt: est.wPt, hPt: lay.rowPt}
	for i, c := range cells {
		if c.Icon == nil || c.Icon.IsEmpty() {
			continue
		}
		if icons[i] > 0 {
			if minDim := math.Min(lay.geo.wPt, lay.geo.hPt); minDim > 0 {
				lay.iconScales[i] = math.Min(1, icons[i]/minDim)
			}
			continue
		}
		lay.iconScales[i] = lay.geo.iconScale(c.Icon, effectiveIconPos(c.Icon, lay.iconPos))
	}
	return lay
}

// errKPIRowTooTall refuses a KPI row whose area cannot hold a value over its
// caption: the lines would be written shrunk into each other.
func errKPIRowTooTall(patternName string, n int, lay kpiRowLayout) *ValidationError {
	sizes := fmt.Sprintf("the minimum %.0fpt value over a %.0fpt caption", lay.bigSize, shapegrid.EffectiveTextSizePt(lay.smallSize))
	advice := "give the region or segment more height (a larger size_pct), shorten the captions, drop the delta / comparator lines, or use kpi-inline"
	if lay.authored {
		sizes = fmt.Sprintf("the %.0fpt value over a %.0fpt caption set in overrides", lay.bigSize, shapegrid.EffectiveTextSizePt(lay.smallSize))
		advice = "lower or remove overrides.big_size / small_size, or give the row more height"
	}
	return newValidationError(patternName, "values", ErrCodeFitOverflow,
		fmt.Sprintf("%s: %d KPIs need %.0fpt of height for %s, but their area is %.0fpt tall — %s",
			patternName, n, lay.needPt, sizes, lay.availPt, advice), nil)
}

// iconPosition is the default overlay icon position: "top" for every
// kpi-Nup card, so the icon and value stack on one centred axis and the whole
// family shares one anatomy. Landscape kpi-2up / kpi-3up cards used to pin
// the icon at the left inset and centre the value in the remaining width, so
// the number sat right of centre and the anatomy flipped per template
// (go-slide-creator-ux1le). An authored icon.position still wins.
func (g kpiCardGeometry) iconPosition() string {
	return "top"
}

// topIconPt returns the edge (points) of a cell's top icon, or 0 when the
// cell has no icon or it sits elsewhere. By default it follows the value it
// sits over; an authored scale is taken of the card's shorter side, with
// rowPt the row height measured so far.
func (g kpiCardGeometry) topIconPt(icon *IconRef, pos string, bigSize, rowPt float64) float64 {
	if icon == nil || icon.IsEmpty() || effectiveIconPos(icon, pos) != "top" {
		return 0
	}
	if icon.Scale > 0 && icon.Scale <= 1 {
		side := g.wPt
		if rowPt > 0 {
			side = math.Min(side, rowPt)
		}
		return icon.Scale * side
	}
	return math.Min(bigSize*kpiTopIconValueRatio, g.wPt*kpiTopIconWFrac)
}

// iconSizePt returns the default icon edge (points) for a card at pos.
func (g kpiCardGeometry) iconSizePt(pos string) float64 {
	if pos == "left" {
		return math.Min(g.hPt*kpiLeftIconHFrac, g.wPt*kpiLeftIconWFrac)
	}
	return math.Min(g.hPt*kpiTopIconHFrac, g.wPt*kpiTopIconWFrac)
}

// iconScale converts the default icon size into the overlay scale factor
// shapegrid applies (icon edge = scale x min(card w, card h)); a
// user-authored scale wins.
func (g kpiCardGeometry) iconScale(icon *IconRef, pos string) float64 {
	if icon != nil && icon.Scale > 0 && icon.Scale <= 1 {
		return icon.Scale
	}
	minDim := math.Min(g.wPt, g.hPt)
	if minDim <= 0 {
		return 0
	}
	return math.Min(1, math.Round(g.iconSizePt(pos)/minDim*100)/100)
}

// effectiveIconPos returns the icon's authored position or the default.
func effectiveIconPos(icon *IconRef, pos string) string {
	if icon != nil && icon.Position != "" {
		return icon.Position
	}
	return pos
}

// valueWidthPt returns the text width available to the big value in a card
// whose icon (if any) sits at pos, mirroring shapegrid's overlay layout.
func (g kpiCardGeometry) valueWidthPt(icon *IconRef, pos string) float64 {
	w := g.wPt - 2*defaultShapeInsetLRPt
	if icon == nil || icon.IsEmpty() {
		return w
	}
	pos = effectiveIconPos(icon, pos)
	if pos != "left" {
		return w
	}
	scale := g.iconScale(icon, pos)
	iconPt := math.Min(g.hPt*scale, math.Min(g.wPt, g.hPt)*scale)
	iconPt = math.Min(iconPt, g.wPt*kpiLeftIconMaxWFrac)
	return w - iconPt - 2*kpiIconGapPt
}

// kpiFitBigSize shrinks the big-number size so every card's value renders on
// a single line (no "$4 / .2 / M" or "12 / days" breaks). The smallest fitting
// size is applied to all cards so the row stays visually uniform.
func kpiFitBigSize(ctx ExpandContext, cells []KPICell, bigSize float64, geo kpiCardGeometry, iconPos string) float64 {
	font := ctx.Theme.BodyFont
	size := bigSize
	for _, c := range cells {
		if c.Big == "" {
			continue
		}
		s := fitSingleLineSize(c.Big, font, true, bigSize, kpiMinBigSize, geo.valueWidthPt(c.Icon, iconPos)*kpiValueLineFrac)
		size = math.Min(size, s)
	}
	return size
}
