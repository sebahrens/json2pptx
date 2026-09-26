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
)

// ---------------------------------------------------------------------------
// capability-heatmap pattern — 3-8 function columns, each a pointed header
// (bold title + optional sublabel) over 1-6 activity cells filled by tier,
// with a tier legend under the grid.
//
//   [ Front office ▷ ][ Middle office ▷ ][ Operations ▷ ]
//   [██ Research   ██][░░ Reconcile  ░░][▒▒ Settle     ▒▒]
//   [░░ Pitchbooks ░░][██ Risk calc  ██]
//   [▒▒ CRM upkeep ▒▒]
//   ■ High  ■ Medium  ■ Low                                    ← legend
//
// The fill of every cell is its tier, not its position, so the pattern does
// not expose cell_accent_mode: the colour carries the data.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&capabilityHeatmap{})
}

type capabilityHeatmap struct{}

const (
	chmName = "capability-heatmap"

	chmMinColumns = 3
	chmMaxColumns = 8
	chmMinCells   = 1
	chmMaxCells   = 6
	chmMinTiers   = 2
	chmMaxTiers   = 4

	chmHeaderMax      = 40
	chmSublabelMax    = 40
	chmCellMax        = 60
	chmTierLabelMax   = 30
	chmTierDescMax    = 80
	chmColGapPt       = 6.0
	chmRowGapPt       = 4.0
	chmHeaderPointPt  = 10.0 // depth of the homePlate point, in points
	chmHeaderPadPt    = 3.0  // breathing room beyond the point
	chmFitSafetyFrac  = 0.90 // measure against 90% of the computed width
	chmMinHeaderPt    = 40.0
	chmMinCellPt      = 30.0
	chmMaxCellPt      = 66.0 // stretched cells stop growing here
	chmCellPadPt      = 8.0
	chmFillFrac       = 0.95 // share of the content height the block aims for
	chmLegendGapPt    = 8.0
	chmLegendSwatchPt = 14.0
)

// Header shapes.
const (
	chmHeaderHomePlate = "homePlate"
	chmHeaderRect      = "rect"
)

func (p *capabilityHeatmap) Name() string { return chmName }
func (p *capabilityHeatmap) Description() string {
	return "Capability / automation heatmap: 3-8 function columns with a pointed header (bold title + optional sublabel) over 1-6 activity cells each, every cell filled by its tier (darkest = highest), with a tier legend underneath"
}
func (p *capabilityHeatmap) UseWhen() string {
	return "Rating many activities grouped under 3-8 functions on one 2-4 level scale (automation or AI potential, capability strength, maturity by function) where colour carries the rating and columns may hold different numbers of activities; prefer table-highlight for options scored against shared criteria, value-chain for a described step sequence, card-grid for unrated tiles"
}
func (p *capabilityHeatmap) NotWhen() string {
	return "Options scored against the same criteria (use table-highlight), a sequence of described steps (use value-chain), items without a rating tier (use card-grid or stylish-panels), items placed on two axes (use matrix-2x2), a two-track process (use process-grid-2row), or a numeric intensity matrix with shared rows and columns (use a heatmap chart)"
}
func (p *capabilityHeatmap) Version() int      { return 1 }
func (p *capabilityHeatmap) CellsHint() string { return "(3-8 columns) × (1-6 cells) + legend" }
func (p *capabilityHeatmap) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "data-display",
		NarrativeRole:      []string{"evidence", "frame"},
		PairsWith:          []string{"chart-insights-split", "exec-summary", "phase-roadmap"},
		DensityClass:       "high",
		AccentWeight:       "strong",
		SparseThresholdPct: 15,
		DataVisual:         true,
	}
}
func (p *capabilityHeatmap) SupportsInlineMarkdown() bool { return true }

func (p *capabilityHeatmap) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 4, Rows: 5},
		{Columns: 6, Rows: 5},
		{Columns: 8, Rows: 7},
	}
}

func (p *capabilityHeatmap) ExemplarValues() any {
	return &CapabilityHeatmapValues{
		Tiers: []CapabilityHeatmapTier{
			{Label: "High", Description: ">50% of time automatable"},
			{Label: "Medium", Description: "20-50% of time"},
			{Label: "Low", Description: "<20% of time"},
		},
		Columns: []CapabilityHeatmapColumn{
			{Header: "Front office", Sublabel: "Up to 30% capacity unlock", Cells: []CapabilityHeatmapCell{
				{Text: "Research summaries", Tier: 0}, {Text: "Pitch preparation", Tier: 1}, {Text: "Client meetings", Tier: 2},
			}},
			{Header: "Middle office", Sublabel: "Up to 45% capacity unlock", Cells: []CapabilityHeatmapCell{
				{Text: "Reconciliations", Tier: 0}, {Text: "Risk reporting", Tier: 0}, {Text: "Limit monitoring", Tier: 1}, {Text: "Exception handling", Tier: 2},
			}},
			{Header: "Operations", Sublabel: "Up to 40% capacity unlock", Cells: []CapabilityHeatmapCell{
				{Text: "Trade settlement", Tier: 0}, {Text: "Corporate actions", Tier: 1},
			}},
			{Header: "Corporate", Sublabel: "Up to 20% capacity unlock", Cells: []CapabilityHeatmapCell{
				{Text: "Invoice processing", Tier: 0}, {Text: "HR queries", Tier: 1}, {Text: "Strategy work", Tier: 2},
			}},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CapabilityHeatmapTier is one level of the rating scale; tier 0 is the
// highest (darkest) level.
type CapabilityHeatmapTier struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// CapabilityHeatmapCell is one activity and the tier it is rated at.
type CapabilityHeatmapCell struct {
	Text string `json:"text"`
	Tier int    `json:"tier"`
}

// CapabilityHeatmapColumn is one function: a header, an optional sublabel,
// and its activity cells top to bottom.
type CapabilityHeatmapColumn struct {
	Header   string                  `json:"header"`
	Sublabel string                  `json:"sublabel,omitempty"`
	Cells    []CapabilityHeatmapCell `json:"cells"`
}

// CapabilityHeatmapValues holds the rating scale and the function columns.
type CapabilityHeatmapValues struct {
	Tiers   []CapabilityHeatmapTier   `json:"tiers"`
	Columns []CapabilityHeatmapColumn `json:"columns"`
}

// CapabilityHeatmapOverrides are the pattern-level overrides.
type CapabilityHeatmapOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeaderSize     float64 `json:"header_size,omitempty"`
	CellSize       float64 `json:"cell_size,omitempty"`
	ShowLegend     *bool   `json:"show_legend,omitempty"`
	HeaderShape    string  `json:"header_shape,omitempty"` // homePlate (default) | rect
}

// CapabilityHeatmapCellOverride is the shared per-cell override. Indices are
// the headers first (0..N-1), then each column's cells top to bottom, column
// by column.
type CapabilityHeatmapCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (p *capabilityHeatmap) NewValues() any       { return &CapabilityHeatmapValues{} }
func (p *capabilityHeatmap) NewOverrides() any    { return &CapabilityHeatmapOverrides{} }
func (p *capabilityHeatmap) NewCellOverride() any { return &CapabilityHeatmapCellOverride{} }

func (p *capabilityHeatmap) Schema() *Schema {
	tierSchema := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(chmTierLabelMax).WithDescription("Legend label for this tier (e.g. High)"),
		"description": StringSchema(chmTierDescMax).WithDescription("Optional legend explanation (e.g. >50% of time automatable)"),
	}, []string{"label"}).WithAdditionalProperties(false)

	cellSchema := ObjectSchema(map[string]*Schema{
		"text": StringSchema(chmCellMax).WithDescription("Activity name; keep it to 1-3 short words at 7-8 columns"),
		"tier": IntegerSchema(0, chmMaxTiers-1).WithDescription("Index into tiers: 0 = highest (darkest accent fill), 1 = light accent tint, 2 = neutral grey, 3 = lightest grey"),
	}, []string{"text", "tier"}).WithAdditionalProperties(false)

	columnSchema := ObjectSchema(map[string]*Schema{
		"header":   StringSchema(chmHeaderMax).WithDescription("Function name, bold in the header"),
		"sublabel": StringSchema(chmSublabelMax).WithDescription("Optional small line under the header (e.g. Up to 45% capacity unlock)"),
		"cells":    ArraySchema(cellSchema, chmMinCells, chmMaxCells).WithDescription("Activities top to bottom; shorter columns leave the bottom empty"),
	}, []string{"header", "cells"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"tiers":   ArraySchema(tierSchema, chmMinTiers, chmMaxTiers).WithDescription("Rating scale, highest first (2-4); every cell's tier indexes this list"),
		"columns": ArraySchema(columnSchema, chmMinColumns, chmMaxColumns).WithDescription("Function columns left to right (3-8)"),
	}, []string{"tiers", "columns"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":          StringSchema(0).WithDescription("Accent scheme color for tier 0 and the headers (default accent1)").WithDefault("accent1"),
		"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"header_size":     NumberSchema(6, 120).WithDescription("Header title size in points (default 14; shrinks to 12 so no word breaks)"),
		"cell_size":       NumberSchema(6, 120).WithDescription("Activity cell text size in points (default 12)"),
		"show_legend":     BooleanSchema().WithDescription("Tier legend under the grid (default true)"),
		"header_shape":    EnumSchema(chmHeaderHomePlate, chmHeaderRect).WithDescription("Header geometry: homePlate (pointed, default) or rect").WithDefault(chmHeaderHomePlate),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":         valuesSchema,
		"overrides":      overridesSchema,
		"cell_overrides": CellOverridesSchema("cellOverride"),
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Capability heatmap: 3-8 function columns of tier-coloured activity cells under pointed headers, with a tier legend")
}

func (p *capabilityHeatmap) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*CapabilityHeatmapValues)
	if !ok || vals == nil {
		return fmt.Errorf("%s: values must be *CapabilityHeatmapValues, got %T", chmName, values)
	}
	var errs []error

	if ovr, ok := overrides.(*CapabilityHeatmapOverrides); ok && ovr != nil {
		switch ovr.HeaderShape {
		case "", chmHeaderHomePlate, chmHeaderRect:
		default:
			errs = append(errs, newValidationError(chmName, "overrides.header_shape", "invalid_enum",
				fmt.Sprintf("%s: overrides.header_shape must be one of homePlate, rect; got %q", chmName, ovr.HeaderShape),
				UseOneOfFix("overrides.header_shape", []string{chmHeaderHomePlate, chmHeaderRect})))
		}
	}

	if len(vals.Tiers) < chmMinTiers {
		errs = append(errs, errMinItems(chmName, "tiers", chmMinTiers, len(vals.Tiers), "(hint: a single tier carries no rating — use card-grid)"))
	}
	if len(vals.Tiers) > chmMaxTiers {
		errs = append(errs, errMaxItems(chmName, "tiers", chmMaxTiers, len(vals.Tiers), "(hint: merge adjacent levels; more than four fills stop reading as a scale)"))
	}
	for i, tier := range vals.Tiers {
		errs = appendRequiredMax(errs, chmName, fmt.Sprintf("tiers[%d].label", i), tier.Label, chmTierLabelMax, true)
		errs = appendRequiredMax(errs, chmName, fmt.Sprintf("tiers[%d].description", i), tier.Description, chmTierDescMax, false)
	}

	if len(vals.Columns) < chmMinColumns {
		errs = append(errs, errMinItems(chmName, "columns", chmMinColumns, len(vals.Columns), "(hint: use table-highlight or card-grid for fewer groups)"))
	}
	if len(vals.Columns) > chmMaxColumns {
		errs = append(errs, errMaxItems(chmName, "columns", chmMaxColumns, len(vals.Columns), "(hint: split the functions across two slides)"))
	}
	totalCells := len(vals.Columns)
	for i, col := range vals.Columns {
		errs = appendRequiredMax(errs, chmName, fmt.Sprintf("columns[%d].header", i), col.Header, chmHeaderMax, true)
		errs = appendRequiredMax(errs, chmName, fmt.Sprintf("columns[%d].sublabel", i), col.Sublabel, chmSublabelMax, false)
		cellsPath := fmt.Sprintf("columns[%d].cells", i)
		if len(col.Cells) < chmMinCells {
			errs = append(errs, errMinItems(chmName, cellsPath, chmMinCells, len(col.Cells), ""))
		}
		if len(col.Cells) > chmMaxCells {
			errs = append(errs, errMaxItems(chmName, cellsPath, chmMaxCells, len(col.Cells), "(hint: group minor activities or split the slide)"))
		}
		totalCells += len(col.Cells)
		for j, cell := range col.Cells {
			errs = appendRequiredMax(errs, chmName, fmt.Sprintf("%s[%d].text", cellsPath, j), cell.Text, chmCellMax, true)
			if len(vals.Tiers) > 0 && (cell.Tier < 0 || cell.Tier >= len(vals.Tiers)) {
				errs = append(errs, errOutOfRange(chmName, fmt.Sprintf("%s[%d].tier", cellsPath, j), 0, len(vals.Tiers)-1, cell.Tier))
			}
		}
	}

	if coErr := validateCellOverrideKeys(chmName, cellOverrides, totalCells, "(indices: headers 0..N-1, then each column's cells top to bottom)"); coErr != nil {
		errs = append(errs, coErr)
	}
	return errors.Join(errs...)
}

// appendRequiredMax appends a required / max-length error for one string
// field, counting characters rather than bytes.
func appendRequiredMax(errs []error, pattern, path, value string, maxLen int, required bool) []error {
	if strings.TrimSpace(value) == "" {
		if required {
			errs = append(errs, errRequired(pattern, path))
		}
		return errs
	}
	if runeLen(value) > maxLen {
		errs = append(errs, errMaxLength(pattern, path, maxLen, runeLen(value)))
	}
	return errs
}

// ---------------------------------------------------------------------------
// Geometry
// ---------------------------------------------------------------------------

// chmLayout is the measured geometry one heatmap expands to.
type chmLayout struct {
	colWPt       float64
	headerTextW  float64
	cellTextW    float64
	headerPt     float64 // header title size after the shared shrink
	sublabelPt   float64
	cellPt       float64
	headerHPt    float64
	cellContentH float64 // tallest cell's content-sized height
	cellRowHPt   float64 // cell row height after the fill
	legendHPt    float64
	maxCells     int
	unfitHeaders []string
	neededHPt    float64 // height the block needs at content size
	areaHPt      float64
}

func chmResolveOverrides(overrides any) *CapabilityHeatmapOverrides {
	if ovr, ok := overrides.(*CapabilityHeatmapOverrides); ok && ovr != nil {
		return ovr
	}
	return &CapabilityHeatmapOverrides{}
}

func chmShowLegend(ovr *CapabilityHeatmapOverrides) bool {
	return ovr.ShowLegend == nil || *ovr.ShowLegend
}

func chmPointed(ovr *CapabilityHeatmapOverrides) bool {
	return ovr.HeaderShape != chmHeaderRect
}

// chmMeasure lays the heatmap out against the content area. The header text
// width is what is left once the homePlate point is cleared: the preset's own
// text rectangle stops half-way into the point, and the right inset the
// pattern emits stacks on top of that, so both are subtracted.
func chmMeasure(ctx ExpandContext, v *CapabilityHeatmapValues, ovr *CapabilityHeatmapOverrides) chmLayout {
	font := ctx.Theme.BodyFont
	contentW, contentH := contentAreaPt(ctx)
	n := max(len(v.Columns), 1)
	l := chmLayout{areaHPt: contentH}
	l.colWPt = equalColumnWidthPt(contentW, n, chmColGapPt)
	l.cellTextW = math.Max(l.colWPt-2*defaultShapeInsetLRPt, 1)
	if chmPointed(ovr) {
		l.headerTextW = (l.colWPt - defaultShapeInsetLRPt - (chmHeaderPointPt + chmHeaderPadPt) - chmHeaderPointPt/2) * chmFitSafetyFrac
	} else {
		l.headerTextW = (l.colWPt - 2*defaultShapeInsetLRPt) * chmFitSafetyFrac
	}
	l.headerTextW = math.Max(l.headerTextW, 1)

	// One shared title size: shrink until no header word breaks mid-word,
	// never below the renderer's floor.
	size := shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, 14))
	for _, col := range v.Columns {
		for _, word := range strings.Fields(col.Header) {
			if s := fitSingleLineSize(word, font, true, size, shapegrid.MinTextSizePt, l.headerTextW); s < size {
				size = s
			}
		}
	}
	l.headerPt = size
	l.sublabelPt = shapegrid.EffectiveTextSizePt(math.Max(size-2, shapegrid.MinTextSizePt))
	for _, col := range v.Columns {
		for _, word := range strings.Fields(col.Header) {
			if measuredLines(word, font, true, size, l.headerTextW) > 1 {
				l.unfitHeaders = append(l.unfitHeaders, col.Header)
				break
			}
		}
	}

	headerH := 0.0
	for _, col := range v.Columns {
		h := textBlockHeightPt(font, l.headerTextW,
			textParagraph{text: col.Header, size: l.headerPt, bold: true},
			textParagraph{text: col.Sublabel, size: l.sublabelPt})
		headerH = math.Max(headerH, h)
	}
	l.headerHPt = math.Round(math.Max(headerH+2*defaultShapeInsetTBPt+headerBandPadPt, chmMinHeaderPt))

	l.cellPt = shapegrid.EffectiveTextSizePt(ResolveSize(ovr.CellSize, 12))
	cellH := 0.0
	for _, col := range v.Columns {
		l.maxCells = max(l.maxCells, len(col.Cells))
		for _, cell := range col.Cells {
			cellH = math.Max(cellH, textBlockHeightPt(font, l.cellTextW, textParagraph{text: inlineMarkupRe.ReplaceAllString(cell.Text, ""), size: l.cellPt}))
		}
	}
	l.cellContentH = math.Round(math.Max(cellH+2*defaultShapeInsetTBPt+chmCellPadPt, chmMinCellPt))

	if chmShowLegend(ovr) && len(v.Tiers) > 0 {
		l.legendHPt = chmLegendRowPt(ctx, v, contentW)
	}

	fixed := l.headerHPt + l.legendHPt + chmRowGapPt*float64(l.maxCells)
	if l.legendHPt > 0 {
		fixed += chmRowGapPt
	}
	l.neededHPt = fixed + float64(l.maxCells)*l.cellContentH

	// Grow the cells toward the fill target so the heatmap reads as the
	// slide's main content, but stop before a sparse column turns into a
	// tall empty tile.
	_, sizingH := sizingAreaPt(ctx)
	l.cellRowHPt = l.cellContentH
	if l.maxCells > 0 {
		per := (sizingH*chmFillFrac - fixed) / float64(l.maxCells)
		l.cellRowHPt = math.Round(clampPt(per, l.cellContentH, math.Max(l.cellContentH, chmMaxCellPt)))
	}
	return l
}

// chmLegendText is the legend wording for one tier.
func chmLegendText(t CapabilityHeatmapTier) string {
	label := "<b>" + pptx.ConvertMarkdownEmphasis(strings.TrimSpace(t.Label)) + "</b>"
	if d := strings.TrimSpace(t.Description); d != "" {
		return label + " " + pptx.ConvertMarkdownEmphasis(d)
	}
	return label
}

// chmLegendColumns returns the nested legend grid's swatch width and the
// width of each entry's text column (percent of the legend), plus the text
// width in points each entry is measured at. Entries share the width in
// proportion to their wording, so a short "Retain" does not hold a quarter of
// the slide while a long description beside it wraps.
func chmLegendColumns(tiers []CapabilityHeatmapTier, contentW float64) (swatchPct float64, textPct, textWPt []float64) {
	n := max(len(tiers), 1)
	swatchPct = math.Round(chmLegendSwatchPt/contentW*1000) / 10
	avail := 100 - swatchPct*float64(n)
	weights := make([]float64, len(tiers))
	total := 0.0
	for i, t := range tiers {
		weights[i] = 1.1*float64(runeLen(t.Label)) + float64(runeLen(t.Description)) + 8
		total += weights[i]
	}
	textPct = make([]float64, len(tiers))
	textWPt = make([]float64, len(tiers))
	for i := range tiers {
		textPct[i] = math.Floor(avail*weights[i]/total*10) / 10
		textWPt[i] = math.Max(contentW*textPct[i]/100-2*defaultShapeInsetLRPt, 1)
	}
	return swatchPct, textPct, textWPt
}

// chmLegendTextPt is the height of the tallest legend entry's text.
func chmLegendTextPt(ctx ExpandContext, v *CapabilityHeatmapValues, contentW float64) float64 {
	_, _, textW := chmLegendColumns(v.Tiers, contentW)
	h := 0.0
	for i, t := range v.Tiers {
		h = math.Max(h, textBlockHeightPt(ctx.Theme.BodyFont, textW[i], textParagraph{text: inlineMarkupRe.ReplaceAllString(chmLegendText(t), ""), size: shapegrid.MinTextSizePt}))
	}
	return math.Round(math.Max(h, chmLegendSwatchPt) + 2*defaultShapeInsetTBPt)
}

func chmLegendRowPt(ctx ExpandContext, v *CapabilityHeatmapValues, contentW float64) float64 {
	return chmLegendTextPt(ctx, v, contentW) + chmLegendGapPt
}

// ---------------------------------------------------------------------------
// Fills
// ---------------------------------------------------------------------------

// chmTierTone is the fill for a tier: the accent itself for the top tier, a
// light tint of it for the second, then neutral greys derived from the page
// colour so the scale reads dark-to-light on every template.
func chmTierTone(accent string, tier int) fillTone {
	switch tier {
	case 0:
		return fillTone{Color: accent}
	case 1:
		if isHexColor(accent) {
			return paleAccentTone(accent)
		}
		return fillTone{Color: accent, LumMod: 40000, LumOff: 60000}
	case 2:
		return fillTone{Color: "lt1", LumMod: 85000}
	default:
		return fillTone{Color: "lt1", LumMod: 95000}
	}
}

// chmTierLine is the cell border: none, except for the lightest tier, which
// is barely off the page colour and needs a hairline to read as a cell.
func chmTierLine(tier int) json.RawMessage {
	if tier >= 3 {
		return json.RawMessage(paperSurfaceHairline)
	}
	return json.RawMessage(`"none"`)
}

// chmTierFallbackInk is the text colour without a theme to measure against.
func chmTierFallbackInk(tier int) string {
	if tier == 0 {
		return "lt1"
	}
	return "dk1"
}

// chmHeaderTone is the header fill: the same accent one step darker than tier
// 0, so the header band never merges with a top-tier cell under it.
func chmHeaderTone(accent string) fillTone {
	if isHexColor(accent) {
		return fillTone{Color: accent}
	}
	return peerTone(accent)
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

// patternTextObj is a paragraphs text object with optional side insets.
type patternTextObj struct {
	Paragraphs    []chartInsightsParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
	InsetLeft     float64                  `json:"inset_left,omitempty"`
	InsetRight    float64                  `json:"inset_right,omitempty"`
	InsetTop      float64                  `json:"inset_top,omitempty"`
}

func (t patternTextObj) json() json.RawMessage {
	data, _ := json.Marshal(t)
	return data
}

func (p *capabilityHeatmap) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*CapabilityHeatmapValues)
	if !ok || vals == nil {
		return nil, fmt.Errorf("%s: values must be *CapabilityHeatmapValues, got %T", chmName, values)
	}
	if overrides != nil {
		if _, ok := overrides.(*CapabilityHeatmapOverrides); !ok {
			return nil, fmt.Errorf("%s: overrides must be *CapabilityHeatmapOverrides, got %T", chmName, overrides)
		}
	}
	ovr := chmResolveOverrides(overrides)
	n := len(vals.Columns)
	if n == 0 {
		return nil, fmt.Errorf("%s: at least one column is required", chmName)
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	l := chmMeasure(ctx, vals, ovr)
	pointed := chmPointed(ovr)

	headerTone := chmHeaderTone(accent)
	headerInk := readableTextOn(ctx, headerTone, "lt1")
	headerCells := make([]*jsonschema.GridCellInput, n)
	for i, col := range vals.Columns {
		paras := []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(col.Header), Size: l.headerPt, Bold: true, Color: headerInk, Align: "ctr"}}
		if s := strings.TrimSpace(col.Sublabel); s != "" {
			paras = append(paras, chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(s), Size: l.sublabelPt, Color: headerInk, Align: "ctr"})
		}
		text := patternTextObj{Paragraphs: paras, Align: "ctr", VerticalAlign: "ctr"}
		shape := &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: headerTone.fillJSON(), Text: nil}
		if pointed {
			shape.Geometry = chmHeaderHomePlate
			// adj is a fraction of the SHORTER side; derive it from this
			// header's own geometry so the point is chmHeaderPointPt deep.
			shape.Adjustments = map[string]int64{"adj": int64(math.Round(chmHeaderPointPt / math.Min(l.headerHPt, l.colWPt) * 100000))}
			text.InsetLeft = defaultShapeInsetLRPt
			text.InsetRight = chmHeaderPointPt + chmHeaderPadPt
		}
		shape.Text = text.json()
		headerCells[i] = &jsonschema.GridCellInput{Shape: shape}
		chmApplyCellOverride(headerCells[i], cellOverrides, i, accent)
	}

	rows := []jsonschema.GridRowInput{{MinHeight: l.headerHPt, MaxHeight: l.headerHPt, Cells: headerCells}}

	// Cell overrides index the cells column by column after the headers.
	colStart := make([]int, n)
	next := n
	for i, col := range vals.Columns {
		colStart[i] = next
		next += len(col.Cells)
	}
	for r := 0; r < l.maxCells; r++ {
		cells := make([]*jsonschema.GridCellInput, n)
		for i, col := range vals.Columns {
			if r >= len(col.Cells) {
				cells[i] = &jsonschema.GridCellInput{}
				continue
			}
			cell := col.Cells[r]
			tier := min(max(cell.Tier, 0), chmMaxTiers-1)
			tone := chmTierTone(accent, tier)
			ink := readableTextOn(ctx, tone, chmTierFallbackInk(tier))
			text := patternTextObj{
				Paragraphs:    []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(cell.Text), Size: l.cellPt, Color: ink, Align: "ctr"}},
				Align:         "ctr",
				VerticalAlign: "ctr",
			}
			cells[i] = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     tone.fillJSON(),
				Line:     chmTierLine(tier),
				Text:     text.json(),
			}}
			chmApplyCellOverride(cells[i], cellOverrides, colStart[i]+r, accent)
		}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: l.cellRowHPt, MaxHeight: l.cellRowHPt, Cells: cells})
	}

	if l.legendHPt > 0 {
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: l.legendHPt,
			MaxHeight: l.legendHPt,
			Cells:     []*jsonschema.GridCellInput{chmLegendCell(ctx, vals, accent, n)},
		})
	}

	colsJSON, _ := json.Marshal(n)
	return &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(colsJSON),
		ColGap:        chmColGapPt,
		RowGap:        chmRowGapPt,
		Rows:          rows,
		VerticalAlign: GridVerticalAlignDefault,
	}, nil
}

// chmLegendCell builds the legend: a nested row of [swatch, label] pairs, one
// per tier, spanning the whole grid.
func chmLegendCell(ctx ExpandContext, v *CapabilityHeatmapValues, accent string, span int) *jsonschema.GridCellInput {
	contentW, _ := contentAreaPt(ctx)
	swatchPct, textPct, _ := chmLegendColumns(v.Tiers, contentW)
	var cells []*jsonschema.GridCellInput
	var cols []float64
	for i, t := range v.Tiers {
		tone := chmTierTone(accent, i)
		cells = append(cells, &jsonschema.GridCellInput{
			MaxHeight: chmLegendSwatchPt,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     tone.fillJSON(),
				Line:     json.RawMessage(paperSurfaceHairline),
			},
		})
		text := patternTextObj{
			Paragraphs:    []chartInsightsParagraph{{Content: chmLegendText(t), Size: shapegrid.MinTextSizePt, Color: "dk1", Align: "l"}},
			Align:         "l",
			VerticalAlign: "t",
		}
		cells = append(cells, &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     text.json(),
		}})
		cols = append(cols, swatchPct, textPct[i])
	}
	colsJSON, _ := json.Marshal(cols)
	// The nested row needs an explicit height: without one it collapses inside
	// its cell and the labels autofit to nothing (go-slide-creator-z0up).
	rowPt := chmLegendTextPt(ctx, v, contentW)
	return &jsonschema.GridCellInput{
		ColSpan: span,
		Grid: &jsonschema.ShapeGridInput{
			Columns: json.RawMessage(colsJSON),
			ColGap:  0,
			Rows:    []jsonschema.GridRowInput{{MinHeight: rowPt, MaxHeight: rowPt, Cells: cells}},
			// The gap above the legend is the top of this cell; the legend
			// itself sits on the bottom edge.
			VerticalAlign: "bottom",
		},
	}
}

func chmApplyCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, ok := co.(*CapabilityHeatmapCellOverride)
	if !ok {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
	}
}

// ---------------------------------------------------------------------------
// Post-expand warnings
// ---------------------------------------------------------------------------

// PostExpandWarnings reports what the measurement found that the renderer
// cannot rescue: a header word too wide for its column even at the 12pt floor
// (it breaks mid-word), and a heatmap whose content-sized rows need more
// height than the content area has (every cell autofits smaller).
func (p *capabilityHeatmap) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*CapabilityHeatmapValues)
	if !ok || v == nil || len(v.Columns) == 0 {
		return nil
	}
	ovr := chmResolveOverrides(overrides)
	l := chmMeasure(ctx, v, ovr)
	var out []string
	if len(l.unfitHeaders) > 0 {
		out = append(out, fmt.Sprintf(
			"%s: capability-heatmap header %s has a word too wide for its column at %d columns even at %.0fpt — the renderer breaks it mid-word; shorten or hyphenate the header, or use fewer columns",
			ErrCodeTextExceedsShape, listFirstN(l.unfitHeaders, 3), len(v.Columns), l.headerPt))
	}
	if l.areaHPt > 0 && l.neededHPt > l.areaHPt {
		ci, cj, longest := 0, 0, -1
		for i, col := range v.Columns {
			for j, cell := range col.Cells {
				if n := runeLen(cell.Text); n > longest {
					ci, cj, longest = i, j, n
				}
			}
		}
		out = append(out, fmt.Sprintf(
			"%s: capability-heatmap columns[%d].cells[%d].text sets a %.0fpt cell height; %d cell rows plus the header and legend need about %.0fpt but the content area holds %.0fpt, so every cell shrinks — shorten the longest activities, drop a row, or hide the legend",
			ErrCodeBodyTooLong, ci, cj, l.cellContentH, l.maxCells, l.neededHPt, l.areaHPt))
	}
	return out
}
