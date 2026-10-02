package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// ---------------------------------------------------------------------------
// agenda pattern — numbered section list for table-of-contents slides
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&agenda{})
}

type agenda struct{}

func (a *agenda) Name() string { return "agenda" }
func (a *agenda) Description() string {
	return "Numbered section list for agenda / table-of-contents slides"
}
func (a *agenda) UseWhen() string {
	return "Numbered agenda or table of contents listing deck sections; prefer icon-row when items are visual categories, card-grid when items need body text"
}
func (a *agenda) NotWhen() string {
	return "Items are visual categories with icons (use icon-row), items need multi-line descriptions (use card-grid), or content is a sequential process (use process-flow)"
}
func (a *agenda) Version() int      { return 1 }
func (a *agenda) CellsHint() string { return "2-10" }
func (a *agenda) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"open", "frame"},
		PairsWith:     []string{"kpi-3up", "card-grid", "stat-hero"},
		DensityClass:  "low",
		AccentWeight:  "subtle",
	}
}

func (a *agenda) ExemplarValues() any {
	v := AgendaValues{
		Items: []string{"Introduction", "Market Analysis", "Strategy", "Next Steps"},
	}
	return &v
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// AgendaValues holds the section titles for the agenda pattern.
type AgendaValues struct {
	Items []string `json:"items"` // Section titles in order
	// Subtitles is an optional one-line description per section, parallel to
	// Items, set in a smaller muted line under the title. It lets a described
	// agenda keep the numbered list and its current-section highlight instead
	// of the agenda-with-images tiles (go-slide-creator-rv9fe).
	Subtitles []string `json:"subtitles,omitempty"`
}

// agendaSubtitleMax is the per-subtitle character budget.
const agendaSubtitleMax = 120

// subtitle returns the subtitle for item i, or "".
func (v *AgendaValues) subtitle(i int) string {
	if i < len(v.Subtitles) {
		return strings.TrimSpace(v.Subtitles[i])
	}
	return ""
}

// AgendaOverrides contains pattern-level overrides for the agenda.
type AgendaOverrides struct {
	Accent         string  `json:"accent,omitempty"`          // Accent scheme color for number badges
	SemanticAccent string  `json:"semantic_accent,omitempty"` // Semantic accent role
	Highlight      int     `json:"highlight,omitempty"`       // 1-based index of item to highlight (0 = none)
	NumberSize     float64 `json:"number_size,omitempty"`     // Font size for number in points
	TitleSize      float64 `json:"title_size,omitempty"`      // Font size for title in points
}

// AgendaCellOverride is an alias for the shared CellOverride struct.
type AgendaCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (a *agenda) NewValues() any       { return &AgendaValues{} }
func (a *agenda) NewOverrides() any    { return &AgendaOverrides{} }
func (a *agenda) NewCellOverride() any { return &AgendaCellOverride{} }

func (a *agenda) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*AgendaValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	if limit := agendaUnbrokenBudget(len(v.Items)); limit > 0 {
		for i, title := range v.Items {
			longest := 0
			for _, word := range strings.Fields(title) {
				longest = max(longest, runeLen(word))
			}
			if longest > limit {
				warnings = append(warnings, fmt.Sprintf("%s: agenda items[%d] contains a %d-character unbroken word; %d rows hold about %d wide characters per title — add a word break, shorten the title, or split the agenda", ErrCodeBodyTooLong, i, longest, len(v.Items), limit))
			}
		}
	}
	if len(warnings) > 0 || len(v.Items) == 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return warnings
	}
	// With the template's own content area the rows are measured against it
	// (go-slide-creator-n1muf).
	ovr, _ := overrides.(*AgendaOverrides)
	if ovr == nil {
		ovr = &AgendaOverrides{}
	}
	v, lay, fits, dropped := agendaFitValues(ctx, v, ovr)
	if dropped {
		warnings = append(warnings, fmt.Sprintf("%s: agenda subtitles do not fit the content area at readable sizes and are left off — shorten or drop the subtitles, or use fewer sections", ErrCodeBodyTooLong))
	}
	if !fits && agendaFlexShrinks(ctx, v, lay, ovr.Highlight) {
		_, areaH := sizingAreaPt(ctx)
		warnings = append(warnings, fmt.Sprintf("%s: agenda rows need %s at readable sizes but the content area holds about %.0fpt — shorten the section titles or subtitles, or use fewer sections", ErrCodeBodyTooLong, readableNeedPhrase(lay.natural()), areaH))
	}
	return warnings
}

// agendaUnbrokenBudget is the widest unbroken run a title holds at an item
// count, measured by TestAgendaBudgetProbe against the written size on every
// shipped template with the uniform shape text margin; 0 means the schema
// maximum holds.
func agendaUnbrokenBudget(items int) int {
	switch {
	case items >= 6:
		return 59
	case items == 5:
		return 61
	}
	return 0
}

func (a *agenda) Schema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"values": ObjectSchema(
				map[string]*Schema{
					"items": ArraySchema(StringSchema(100).WithDescription("Section title; with 5-10 items, keep wide unbroken runs near 59 characters or add word breaks"), 2, 10).
						WithDescription("Section titles in order"),
					"subtitles": ArraySchema(StringSchema(agendaSubtitleMax).WithDescription("One-line section description, set smaller and muted under the title; left off, with BODY_TOO_LONG, when the rows cannot hold them"), 0, 10).
						WithDescription("Optional per-section descriptions, parallel to items (no more entries than items)"),
				},
				[]string{"items"},
			).WithAdditionalProperties(false),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":          StringSchema(0).WithDescription("Accent scheme color for number badges (default accent1)").WithDefault("accent1"),
					"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role"),
					"highlight":       NumberSchema(0, 10).WithDescription("1-based index of item to highlight (0 = none)"),
					"number_size":     NumberSchema(6, 120).WithDescription("Font size for number in points"),
					"title_size":      NumberSchema(6, 120).WithDescription("Font size for title in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Numbered section list for agenda / table-of-contents slides")
}

func (a *agenda) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*AgendaValues)
	if !ok || v == nil {
		return fmt.Errorf("agenda: values must be *AgendaValues, got %T", values)
	}

	const name = "agenda"
	var errs []error

	if len(v.Items) < 2 {
		errs = append(errs, errMinItems(name, "items", 2, len(v.Items), ""))
	}
	if len(v.Items) > 10 {
		errs = append(errs, errMaxItems(name, "items", 10, len(v.Items), ""))
	}

	for i, item := range v.Items {
		path := fmt.Sprintf("items[%d]", i)
		if item == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(item) > 100 {
			errs = append(errs, errMaxLength(name, path, 100, runeLen(item)))
		}
	}
	if len(v.Subtitles) > len(v.Items) {
		errs = append(errs, fmt.Errorf("agenda: subtitles has %d entries for %d items; give at most one subtitle per item", len(v.Subtitles), len(v.Items)))
	}
	for i, sub := range v.Subtitles {
		if runeLen(sub) > agendaSubtitleMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("subtitles[%d]", i), agendaSubtitleMax, runeLen(sub)))
		}
	}

	if overrides != nil {
		ovr, ovrOk := overrides.(*AgendaOverrides)
		if !ovrOk {
			errs = append(errs, fmt.Errorf("agenda: overrides must be *AgendaOverrides, got %T", overrides))
		} else if ovr.Highlight > len(v.Items) {
			errs = append(errs, fmt.Errorf("agenda: overrides.highlight (%d) exceeds item count (%d)", ovr.Highlight, len(v.Items)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(v.Items), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// Agenda typography and rules (go-slide-creator-r3gsw, design review C5): a
// serif numeral in the accent beside the item, 0.5pt rules between rows, no
// filled tiles, content-height rows middle-anchored on the slide.
const (
	agendaNumberSize   = tokens.TypeScaleDisplayPt
	agendaTitleSize    = scaleSubheadPt
	agendaRulePt       = 0.5
	agendaListRowGapPt = 2.0
	// agendaDimAlpha is the opacity of the sections the deck is not at when
	// the agenda repeats with a highlight: dk1 (and the numeral) at 50%.
	agendaDimAlpha = 50.0
	// agendaMinFillFrac gives a short agenda enough presence to read as the
	// slide's content rather than a list floating in the middle.
	agendaMinFillFrac = 0.62
	// agendaNumberFont is the theme's major (heading) font — the serif on a
	// serif-headed template.
	agendaNumberFont = "+mj-lt"
)

// agendaScales steps the numeral / item sizes down only when the default rows
// no longer fit the content area (8–10 item agendas). Numerals step from the
// 28pt display step to the 18pt lead step; items stay on the 14pt subhead
// step (go-slide-creator-vmdfm).
var agendaScales = [][2]float64{{agendaNumberSize, agendaTitleSize}, {scaleLeadPt, agendaTitleSize}}

// agendaParagraph is one agenda text run: numerals carry the heading font and
// dimmed rows an opacity.
type agendaParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
	Font    string  `json:"font,omitempty"`
	Alpha   float64 `json:"alpha,omitempty"`
}

// agendaText is the left-aligned, middle-anchored text object every agenda
// cell uses: one paragraph, or a title over its subtitle.
func agendaText(ps ...agendaParagraph) json.RawMessage {
	for i := range ps {
		ps[i].Align = "l"
	}
	data, _ := json.Marshal(struct {
		Paragraphs    []agendaParagraph `json:"paragraphs"`
		Align         string            `json:"align"`
		VerticalAlign string            `json:"vertical_align"`
	}{ps, "l", "ctr"})
	return data
}

// agendaSubtitleSize is the subtitle size under a title of titleSize: four
// points smaller, never under the 12pt floor.
func agendaSubtitleSize(titleSize float64) float64 {
	return math.Max(12, titleSize-4)
}

// agendaSubtitleAlpha is the muted (secondary) opacity of a subtitle on a row
// that is not dimmed.
const agendaSubtitleAlpha = 70.0

// agendaLayout is the measured geometry at one type scale.
type agendaLayout struct {
	numberSize, titleSize float64
	numberPct             float64
	rowPt                 []float64
	rowGapPt              float64 // agendaListRowGapPt on the template grid
}

func (l agendaLayout) natural() float64 {
	n := len(l.rowPt)
	h := float64(n-1) * agendaRulePt
	for _, r := range l.rowPt {
		h += r
	}
	return h + float64(2*n-2)*l.rowGapPt
}

func measureAgenda(ctx ExpandContext, v *AgendaValues, numberSize, titleSize float64, highlight int) agendaLayout {
	areaW, _ := sizingAreaPt(ctx)
	// The numeral column holds "10" at the numeral size plus the text margin.
	numberColPt := numberSize*1.3 + 2*defaultShapeInsetLRPt
	pct := math.Min(20, math.Max(6, math.Ceil(numberColPt/areaW*100)))
	lay := agendaLayout{numberSize: numberSize, titleSize: titleSize, numberPct: pct, rowGapPt: ctx.Gap(agendaListRowGapPt)}
	numberW, titleW := lay.columnWidths(areaW)
	numberRow := numberSize*sizingLineSpacing + 2*sizingInsetTBPt
	for i, title := range v.Items {
		paras := []sizedPara{{text: title, sizePt: titleSize}}
		if sub := v.subtitle(i); sub != "" {
			paras = append(paras, sizedPara{text: sub, sizePt: agendaSubtitleSize(titleSize)})
		}
		h := sizedBlockHeightPt(ctx, paras, titleW)
		// The row is never below the written fit of its title (bold when it
		// is the highlighted section) or numeral at the real column widths
		// (go-slide-creator-n1muf).
		h = math.Max(h, rowTextNeedPt(agendaTitleText(title, v.subtitle(i), titleSize, highlight == i+1), titleW))
		h = math.Max(h, rowTextNeedPt(agendaNumberText(i, numberSize), numberW))
		lay.rowPt = append(lay.rowPt, math.Ceil(math.Max(numberRow, h)))
	}
	return lay
}

// columnWidths are the numeral and title column widths in an areaW-wide grid.
func (l agendaLayout) columnWidths(areaW float64) (numberW, titleW float64) {
	inner := areaW - agendaColGapPt
	return inner * l.numberPct / 100, inner * (100 - l.numberPct) / 100
}

// agendaColGapPt is the gap between the numeral and title columns.
const agendaColGapPt = 0.1

// agendaTitleText and agendaNumberText are the plain (undimmed) cell texts
// the rows are measured on; dimming changes only the opacity.
func agendaTitleText(title, subtitle string, size float64, bold bool) json.RawMessage {
	return agendaItemText(title, subtitle, size, bold, 0, 0)
}

// agendaItemText is a section's title paragraph and, when it has one, its
// subtitle paragraph — smaller and muted — at the given opacities.
func agendaItemText(title, subtitle string, size float64, bold bool, titleAlpha, subAlpha float64) json.RawMessage {
	paras := []agendaParagraph{{Content: title, Size: size, Bold: bold, Color: "dk1", Alpha: titleAlpha}}
	if subtitle != "" {
		paras = append(paras, agendaParagraph{Content: subtitle, Size: agendaSubtitleSize(size), Color: "dk1", Alpha: subAlpha})
	}
	return agendaText(paras...)
}

func agendaNumberText(i int, size float64) json.RawMessage {
	return agendaText(agendaParagraph{Content: fmt.Sprintf("%02d", i+1), Size: size, Font: agendaNumberFont})
}

// agendaFloorScale is the last step: items at the 12pt floor, taken only when
// it makes the rows fit so an overflowing agenda keeps the larger step.
var agendaFloorScale = [2]float64{scaleLeadPt, scaleBodyPt}

// layoutAgenda picks the largest type scale whose rows fit the content area;
// an authored numeral or title size is kept. The second result reports
// whether the rows fit at all.
func layoutAgenda(ctx ExpandContext, v *AgendaValues, ovr *AgendaOverrides) (agendaLayout, bool) {
	_, areaH := sizingAreaPt(ctx)
	var lay agendaLayout
	for _, sc := range agendaScales {
		lay = measureAgenda(ctx, v, ResolveSize(ovr.NumberSize, sc[0]), ResolveSize(ovr.TitleSize, sc[1]), ovr.Highlight)
		if lay.natural() <= areaH {
			return lay, true
		}
	}
	if ovr.TitleSize == 0 {
		floor := measureAgenda(ctx, v, ResolveSize(ovr.NumberSize, agendaFloorScale[0]), agendaFloorScale[1], ovr.Highlight)
		if floor.natural() <= areaH {
			return floor, true
		}
	}
	return lay, false
}

// agendaFitValues lays the agenda out, leaving the subtitles off when the rows
// cannot hold them at a readable size: the section titles are the agenda, and
// squeezing every row to share the area shrank them below the floor. The last
// result reports that subtitles were dropped (PostExpandWarnings says so).
func agendaFitValues(ctx ExpandContext, v *AgendaValues, ovr *AgendaOverrides) (*AgendaValues, agendaLayout, bool, bool) {
	lay, fits := layoutAgenda(ctx, v, ovr)
	if fits || len(v.Subtitles) == 0 {
		return v, lay, fits, false
	}
	bare := &AgendaValues{Items: v.Items}
	lay, fits = layoutAgenda(ctx, bare, ovr)
	return bare, lay, fits, true
}

// agendaFlexShrinks reports whether an agenda whose rows do not fit would be
// written shrunk: the rows share the area as equal flex rows, where a
// one-line title can still be written whole (the writer clamps a short
// shape's margin).
func agendaFlexShrinks(ctx ExpandContext, v *AgendaValues, lay agendaLayout, highlight int) bool {
	areaW, areaH := sizingAreaPt(ctx)
	n := float64(len(v.Items))
	rowH := (areaH - (n-1)*agendaRulePt - (2*n-2)*ctx.Gap(agendaListRowGapPt)) / n
	numberW, titleW := lay.columnWidths(areaW)
	for i, title := range v.Items {
		for _, cell := range []struct {
			text json.RawMessage
			w    float64
		}{{agendaTitleText(title, v.subtitle(i), lay.titleSize, highlight == i+1), titleW}, {agendaNumberText(i, lay.numberSize), numberW}} {
			tb, err := shapegrid.ResolveTextInput(cell.text)
			if err == nil && tb != nil && !pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: int64(cell.w * sizingEMUPerPt), CY: int64(rowH * sizingEMUPerPt)}) {
				return true
			}
		}
	}
	return false
}

func (a *agenda) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*AgendaValues)
	if !ok {
		return nil, fmt.Errorf("agenda: values must be *AgendaValues, got %T", values)
	}

	ovr := &AgendaOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*AgendaOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("agenda: overrides must be *AgendaOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	v, lay, fits, _ := agendaFitValues(ctx, v, ovr)

	// Numerals are large text (3:1); items are body text (4.5:1). A pale
	// accent falls back to dk2 / dk1, and the 50% dim steps up just enough to
	// stay readable, so the WCAG pass never has to swap the ink wholesale.
	numberInk := accentInkOnLight(ctx, accent, 3.0)
	numberDim := readableDimAlpha(ctx, numberInk, agendaDimAlpha, 3.0)
	titleDim := readableDimAlpha(ctx, "dk1", agendaDimAlpha, 4.5)
	subMuted := readableDimAlpha(ctx, "dk1", agendaSubtitleAlpha, 4.5)

	ruleFill := fillTone{Color: "dk1", Alpha: 30}.fillJSON()
	rows := make([]jsonschema.GridRowInput, 0, 2*len(v.Items))
	var itemRow []bool
	for i, title := range v.Items {
		if i > 0 {
			rows = append(rows, jsonschema.GridRowInput{
				MinHeight: agendaRulePt, MaxHeight: agendaRulePt,
				Cells: []*jsonschema.GridCellInput{{
					ColSpan: 2,
					Shape:   &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: ruleFill, Line: json.RawMessage(`"none"`)},
				}},
			})
			itemRow = append(itemRow, false)
		}

		// A repeated agenda marks where the deck is: the current section in
		// bold dk1, the others at 50%. Without a highlight every row is plain.
		highlighted := ovr.Highlight > 0 && ovr.Highlight == i+1
		dimNumber, dimTitle, dimSub := 0.0, 0.0, subMuted
		if ovr.Highlight > 0 && !highlighted {
			dimNumber, dimTitle, dimSub = numberDim, titleDim, titleDim
		}

		numberCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Line:     json.RawMessage(`"none"`),
				Text: agendaText(agendaParagraph{
					Content: fmt.Sprintf("%02d", i+1), Size: lay.numberSize,
					Color: numberInk, Font: agendaNumberFont, Alpha: dimNumber,
				}),
			},
		}
		titleCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Line:     json.RawMessage(`"none"`),
				Text:     agendaItemText(title, v.subtitle(i), lay.titleSize, highlighted, dimTitle, dimSub),
			},
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			if cellOvr, coOk := co.(*AgendaCellOverride); coOk {
				applyCellTextOverride(titleCell, cellOvr)
				if cellOvr.AccentBar {
					titleCell.AccentBar = &jsonschema.AccentBarInput{
						Position: "left",
						Color:    accent,
						Width:    4,
					}
				}
			}
		}

		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: lay.rowPt[i],
			MaxHeight: lay.rowPt[i],
			Cells:     []*jsonschema.GridCellInput{numberCell, titleCell},
		})
		itemRow = append(itemRow, true)
	}
	// Past the smallest scale the rows cannot all hold their content height:
	// hand them to the grid as flex rows so it shares the area rather than
	// overflowing it.
	if !fits {
		for i := range rows {
			if itemRow[i] {
				rows[i].MinHeight, rows[i].MaxHeight = 0, 0
			}
		}
	} else {
		fillCappedRows(ctx, rows, ctx.Gap(agendaListRowGapPt), agendaMinFillFrac, func(i int) bool { return itemRow[i] })
	}

	colsJSON, _ := json.Marshal([]float64{lay.numberPct, 100 - lay.numberPct})
	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		ColGap:        agendaColGapPt,
		RowGap:        ctx.Gap(agendaListRowGapPt),
		Rows:          rows,
		VerticalAlign: "center", // an agenda is a balanced contents page, not a body block
	}

	return grid, nil
}
