package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// risk-heatmap pattern — named risks placed on a likelihood × impact grid
// (3 × 3, or 5 × 5), every cell filled by its exposure band
// (go-slide-creator-ec74l).
//
//   I      High   [▒▒ Model risk ▒▒][██ Third-party ██][██ Cyber   ██]
//   m      Medium [░░ Climate    ░░][▒▒ Conduct     ▒▒][██         ██]
//   p      Low    [░░            ░░][░░ Fraud       ░░][▒▒         ▒▒]
//   a               Low               Medium             High
//   c                              Likelihood
//   t      ■ Low risk  ■ Medium risk  ■ High risk               ← legend
//
// A 2 × 2 cannot place "medium": the risk lands on an axis line. Here every
// level has a row and a column of its own, all nine (or twenty-five) cells are
// drawn whether or not a risk sits in them — the grid is the heat map — and a
// cell's fill is its likelihood × impact band, not an author's choice. The
// band colours are a ramp of the template's own "negative" accent (neutral,
// tint, solid), so darker always means more exposure and no status colour is
// hard-coded.
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&riskHeatmap{})
}

type riskHeatmap struct{}

const (
	rhmName = "risk-heatmap"

	rhmDefaultSize = 3
	rhmLargeSize   = 5
	rhmMinItems    = 1
	rhmMaxItems    = 20
	rhmTierCount   = 3

	rhmNameMax      = 40
	rhmLevelMax     = 14
	rhmAxisLabelMax = 24
	rhmTierLabelMax = 24

	rhmGapPt        = 3.0
	rhmYTitleWPt    = 30.0 // rotated impact title strip
	rhmMinYLevelWPt = 44.0
	rhmMaxYLevelWPt = 110.0
	rhmItemGapPt    = 3.0 // space between two risks in one cell
	// rhmOneLineFactor is the line height of a one-line axis label's box, the
	// one the writer's margin clamp leaves (pptx writtenOneLineFactor).
	rhmOneLineFactor = 1.3
	rhmWrapSlackFrac = 0.1
	rhmLegendRowPt   = 26.0 // legend row: one line of labels under a little air
	rhmLegendSwatchW = 14.0
	rhmMinItemPt     = 12.0
	rhmMaxItemPt     = 20.0
)

// rhmDefaultLevels are the level labels low → high when the author gives none.
var rhmDefaultLevels = map[int][]string{
	rhmDefaultSize: {"Low", "Medium", "High"},
	rhmLargeSize:   {"Very low", "Low", "Medium", "High", "Very high"},
}

// rhmLevelWords are the words an item's likelihood / impact may be written
// in, besides a 1-based number and the grid's own level labels.
var rhmLevelWords = map[int]map[string]int{
	rhmDefaultSize: {"low": 1, "medium": 2, "moderate": 2, "med": 2, "high": 3},
	rhmLargeSize:   {"very low": 1, "low": 2, "medium": 3, "moderate": 3, "med": 3, "high": 4, "very high": 5},
}

// rhmDefaultTierLabels are the legend labels low → high.
var rhmDefaultTierLabels = []string{"Low risk", "Medium risk", "High risk"}

func (p *riskHeatmap) Name() string { return rhmName }
func (p *riskHeatmap) Description() string {
	return "Risk heat map: named risks placed on a likelihood × impact grid (3 × 3, or 5 × 5), every cell filled by its likelihood × impact band (low / medium / high, darkest = highest), with level labels along both axes and a band legend"
}
func (p *riskHeatmap) UseWhen() string {
	return "Placing 1-20 named risks (or issues, opportunities) by likelihood and impact on a low / medium / high scale, where a \"medium\" must have a cell of its own and the colour is the exposure band; prefer matrix-2x2 for four described quadrants, capability-heatmap for activities rated per function, table-highlight or a table for a risk register with mitigations and owners"
}
func (p *riskHeatmap) NotWhen() string {
	return "Four quadrants each with a headline and a description (use matrix-2x2), items rated on one scale under function columns (use capability-heatmap), a register that needs mitigation, owner or date per risk (use a table or table-highlight), or a numeric intensity matrix (use a heatmap chart)"
}
func (p *riskHeatmap) Version() int      { return 1 }
func (p *riskHeatmap) CellsHint() string { return "3 × 3 or 5 × 5 + axes + legend" }
func (p *riskHeatmap) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:           "data-display",
		NarrativeRole:      []string{"evidence", "frame"},
		PairsWith:          []string{"table-highlight", "next-steps", "exec-summary"},
		DensityClass:       "medium",
		AccentWeight:       "strong",
		SparseThresholdPct: 15,
		// A judgement call placed on two axes, like matrix-2x2: structural,
		// not a quantitative claim that needs a "so what" lint.
	}
}

func (p *riskHeatmap) ExemplarValues() any {
	return &RiskHeatmapValues{Items: []RiskHeatmapItem{
		{Name: "Cyber attack", Likelihood: "high", Impact: "high"},
		{Name: "Third-party outage", Likelihood: "medium", Impact: "high"},
		{Name: "Model risk", Likelihood: "low", Impact: "high"},
		{Name: "Conduct", Likelihood: "medium", Impact: "medium"},
		{Name: "Climate transition", Likelihood: "low", Impact: "medium"},
		{Name: "Payment fraud", Likelihood: "medium", Impact: "low"},
	}}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// RiskLevel is a likelihood or impact as the author wrote it: a 1-based
// number (1 = lowest) or a level word ("medium", or one of the grid's own
// level labels). It is resolved against the grid in Validate / Expand.
type RiskLevel string

// UnmarshalJSON accepts a JSON string or a whole number.
func (l *RiskLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*l = RiskLevel(strings.TrimSpace(s))
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err != nil || n != math.Trunc(n) {
		return fmt.Errorf("a likelihood / impact level must be a whole number or a level word, got %s", string(data))
	}
	*l = RiskLevel(strconv.Itoa(int(n)))
	return nil
}

// MarshalJSON writes a numeric level as a number and a word as a string.
func (l RiskLevel) MarshalJSON() ([]byte, error) {
	if n, err := strconv.Atoi(string(l)); err == nil {
		return json.Marshal(n)
	}
	return json.Marshal(string(l))
}

// RiskHeatmapItem is one named risk and where it sits.
type RiskHeatmapItem struct {
	Name       string    `json:"name"`
	Likelihood RiskLevel `json:"likelihood"`
	Impact     RiskLevel `json:"impact"`
}

// RiskHeatmapValues holds the risks and the grid they are placed on.
type RiskHeatmapValues struct {
	Items            []RiskHeatmapItem `json:"items"`
	Size             int               `json:"size,omitempty"` // 3 (default) or 5
	LikelihoodLabel  string            `json:"likelihood_label,omitempty"`
	ImpactLabel      string            `json:"impact_label,omitempty"`
	LikelihoodLevels []string          `json:"likelihood_levels,omitempty"`
	ImpactLevels     []string          `json:"impact_levels,omitempty"`
	TierLabels       []string          `json:"tier_labels,omitempty"` // [low, medium, high]
}

// RiskHeatmapOverrides are the pattern-level overrides.
type RiskHeatmapOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	ItemSize       float64 `json:"item_size,omitempty"`
	ShowLegend     *bool   `json:"show_legend,omitempty"`
}

func (p *riskHeatmap) NewValues() any       { return &RiskHeatmapValues{} }
func (p *riskHeatmap) NewOverrides() any    { return &RiskHeatmapOverrides{} }
func (p *riskHeatmap) NewCellOverride() any { return nil }

func (p *riskHeatmap) Schema() *Schema {
	level := func(axis string) *Schema {
		return OneOfSchema(IntegerSchema(1, rhmLargeSize), StringSchema(rhmLevelMax)).
			WithDescription(axis + ": 1 (lowest) to size, or a level word — low / medium / high (very low … very high at size 5) or one of the grid's own level labels")
	}
	itemSchema := ObjectSchema(map[string]*Schema{
		"name":       StringSchema(rhmNameMax).WithDescription("Risk name as it is printed in its cell; keep it to 1-3 words"),
		"likelihood": level("Likelihood (column)"),
		"impact":     level("Impact (row)"),
	}, []string{"name", "likelihood", "impact"}).WithAdditionalProperties(false)

	levels := func(axis string) *Schema {
		return ArraySchema(StringSchema(rhmLevelMax), rhmDefaultSize, rhmLargeSize).
			WithDescription(axis + " level labels, lowest first; exactly size entries (default Low / Medium / High)")
	}
	valuesSchema := ObjectSchema(map[string]*Schema{
		"items":             ArraySchema(itemSchema, rhmMinItems, rhmMaxItems).WithDescription("The named risks (1-20); several may share a cell, each on its own line"),
		"size":              IntegerSchema(rhmDefaultSize, rhmLargeSize).WithDescription("Grid size: 3 (default, low / medium / high) or 5").WithDefault(rhmDefaultSize),
		"likelihood_label":  StringSchema(rhmAxisLabelMax).WithDescription("Horizontal axis title").WithDefault("Likelihood"),
		"impact_label":      StringSchema(rhmAxisLabelMax).WithDescription("Vertical axis title").WithDefault("Impact"),
		"likelihood_levels": levels("Likelihood"),
		"impact_levels":     levels("Impact"),
		"tier_labels":       ArraySchema(StringSchema(rhmTierLabelMax), rhmTierCount, rhmTierCount).WithDescription("Legend wording for the three bands, [low, medium, high] (default Low risk / Medium risk / High risk)"),
	}, []string{"items"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":          StringSchema(0).WithDescription("Scheme color the bands are a ramp of (default: the template's negative accent)"),
		"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set").WithDefault("negative"),
		"item_size":       NumberSchema(rhmMinItemPt, rhmMaxItemPt).WithDescription("Risk name size in points (default 14, stepping to 12 when a cell is crowded)"),
		"show_legend":     BooleanSchema().WithDescription("Band legend under the grid (default true)"),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":    valuesSchema,
		"overrides": overridesSchema,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Risk heat map: named risks on a likelihood × impact grid (3 × 3 or 5 × 5) of band-coloured cells")
}

// rhmSize is the grid size the values ask for.
func rhmSize(v *RiskHeatmapValues) int {
	if v.Size == rhmLargeSize {
		return rhmLargeSize
	}
	return rhmDefaultSize
}

// rhmLevels returns an axis's level labels, lowest first.
func rhmLevels(custom []string, size int) []string {
	if len(custom) == size {
		return custom
	}
	return rhmDefaultLevels[size]
}

// rhmResolveLevel returns the 1-based level a likelihood / impact names.
func rhmResolveLevel(level RiskLevel, labels []string, size int) (int, bool) {
	s := strings.TrimSpace(string(level))
	if n, err := strconv.Atoi(s); err == nil {
		return n, n >= 1 && n <= size
	}
	for i, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), s) {
			return i + 1, true
		}
	}
	n, ok := rhmLevelWords[size][strings.ToLower(strings.Join(strings.Fields(s), " "))]
	return n, ok
}

// rhmTier is the band of a cell: 0 low, 1 medium, 2 high, from the product of
// its 1-based likelihood and impact. The cuts are the conventional ones: on a
// 3 × 3, 1-2 low / 3-4 medium / 6-9 high; on a 5 × 5, 1-4 / 5-12 / 15-25.
func rhmTier(likelihood, impact, size int) int {
	score := likelihood * impact
	lowMax, mediumMax := 2, 4
	if size == rhmLargeSize {
		lowMax, mediumMax = 4, 12
	}
	switch {
	case score <= lowMax:
		return 0
	case score <= mediumMax:
		return 1
	default:
		return 2
	}
}

func (p *riskHeatmap) Validate(values, overrides any, _ map[int]any) error {
	v, ok := values.(*RiskHeatmapValues)
	if !ok || v == nil {
		return fmt.Errorf("%s: values must be *RiskHeatmapValues, got %T", rhmName, values)
	}
	var errs []error
	if ovr, ok := overrides.(*RiskHeatmapOverrides); ok && ovr != nil {
		if ovr.ItemSize != 0 && (ovr.ItemSize < rhmMinItemPt || ovr.ItemSize > rhmMaxItemPt) {
			errs = append(errs, errOutOfRange(rhmName, "overrides.item_size", rhmMinItemPt, rhmMaxItemPt, int(ovr.ItemSize)))
		}
	}

	if v.Size != 0 && v.Size != rhmDefaultSize && v.Size != rhmLargeSize {
		errs = append(errs, newValidationError(rhmName, "size", "invalid_enum",
			fmt.Sprintf("%s: size must be 3 or 5; got %d", rhmName, v.Size),
			UseOneOfFix("size", []string{"3", "5"})))
	}
	size := rhmSize(v)
	for _, axis := range []struct {
		path   string
		labels []string
	}{{"likelihood_levels", v.LikelihoodLevels}, {"impact_levels", v.ImpactLevels}} {
		if len(axis.labels) == 0 {
			continue
		}
		if len(axis.labels) != size {
			errs = append(errs, errCountMismatch(rhmName, axis.path, size, len(axis.labels), "(one label per level, lowest first)"))
		}
		for i, label := range axis.labels {
			errs = appendRequiredMax(errs, rhmName, fmt.Sprintf("%s[%d]", axis.path, i), label, rhmLevelMax, true)
		}
	}
	errs = appendRequiredMax(errs, rhmName, "likelihood_label", v.LikelihoodLabel, rhmAxisLabelMax, false)
	errs = appendRequiredMax(errs, rhmName, "impact_label", v.ImpactLabel, rhmAxisLabelMax, false)
	if len(v.TierLabels) != 0 && len(v.TierLabels) != rhmTierCount {
		errs = append(errs, errCountMismatch(rhmName, "tier_labels", rhmTierCount, len(v.TierLabels), "([low, medium, high])"))
	}
	for i, label := range v.TierLabels {
		errs = appendRequiredMax(errs, rhmName, fmt.Sprintf("tier_labels[%d]", i), label, rhmTierLabelMax, true)
	}

	if len(v.Items) < rhmMinItems {
		errs = append(errs, errMinItems(rhmName, "items", rhmMinItems, len(v.Items), "(hint: each item is {name, likelihood, impact})"))
	}
	if len(v.Items) > rhmMaxItems {
		errs = append(errs, errMaxItems(rhmName, "items", rhmMaxItems, len(v.Items), "(hint: keep the top risks on the map and the full register in a table)"))
	}
	for i, it := range v.Items {
		errs = appendRequiredMax(errs, rhmName, fmt.Sprintf("items[%d].name", i), it.Name, rhmNameMax, true)
		for _, axis := range []struct {
			field  string
			level  RiskLevel
			labels []string
		}{
			{"likelihood", it.Likelihood, rhmLevels(v.LikelihoodLevels, size)},
			{"impact", it.Impact, rhmLevels(v.ImpactLevels, size)},
		} {
			if _, ok := rhmResolveLevel(axis.level, axis.labels, size); ok {
				continue
			}
			path := fmt.Sprintf("items[%d].%s", i, axis.field)
			allowed := make([]string, len(axis.labels))
			for j, label := range axis.labels {
				allowed[j] = strings.ToLower(label)
			}
			errs = append(errs, newValidationError(rhmName, path, "invalid_enum",
				fmt.Sprintf("%s: %s must be a number 1-%d (1 = lowest) or one of %s; got %q", rhmName, path, size, strings.Join(allowed, ", "), string(axis.level)),
				UseOneOfFix(path, allowed)))
		}
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Geometry
// ---------------------------------------------------------------------------

// rhmLayout is the measured geometry one heat map expands to.
type rhmLayout struct {
	size       int
	cells      [][][]string // [display row, top = highest impact][column] risk names
	firstItem  [][]int      // index into values.items of each cell's first risk
	xLevels    []string
	yLevels    []string // lowest first
	xTitle     string
	yTitle     string
	tierLabels []string // low, medium, high; nil when the legend is hidden
	colsPct    []float64
	cellWPt    float64
	itemPt     float64
	padPt      float64 // cell top / bottom text margin; 0 = the uniform margin
	rowNeeds   []float64
	xLevelHPt  float64
	xTitleHPt  float64
	legendHPt  float64
	availPt    float64 // height left for the cell rows
	unfitNames []string
}

func rhmResolveOverrides(overrides any) *RiskHeatmapOverrides {
	if ovr, ok := overrides.(*RiskHeatmapOverrides); ok && ovr != nil {
		return ovr
	}
	return &RiskHeatmapOverrides{}
}

// rhmTextObj is a paragraphs text object whose top / bottom margins may be
// tightened (rowPadStepsPt); nil keeps the uniform margin.
type rhmTextObj struct {
	Paragraphs    []chartInsightsParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
	InsetTop      *float64                 `json:"inset_top,omitempty"`
	InsetBottom   *float64                 `json:"inset_bottom,omitempty"`
}

// rhmCellText is the text object of one cell: each risk a centred paragraph.
func rhmCellText(names []string, size, padPt float64, ink string) json.RawMessage {
	paras := make([]chartInsightsParagraph, len(names))
	for i, name := range names {
		paras[i] = chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(name), Size: size, Color: ink, Align: "ctr"}
		if i < len(names)-1 {
			paras[i].SpaceAfter = rhmItemGapPt
		}
	}
	obj := rhmTextObj{Paragraphs: paras, Align: "ctr", VerticalAlign: "ctr"}
	obj.InsetTop, obj.InsetBottom = rowPadInsets(padPt, 0)
	data, _ := json.Marshal(obj)
	return data
}

// rhmLabelText is an axis level label, an axis title or a legend label.
func rhmLabelText(content string, size float64, bold bool, align, vAlign string) json.RawMessage {
	return patternTextObj{
		Paragraphs:    []chartInsightsParagraph{{Content: content, Size: size, Bold: bold, Color: "dk1", Align: align}},
		Align:         align,
		VerticalAlign: vAlign,
	}.json()
}

// rhmLabelNeedPt is the height an axis label needs in a box widthPt wide. A
// label that stays on one line sits in a box one line high — the writer
// clamps the margin of such a box so the line always fits
// (pptx.EffectiveTextInsets) — and one that wraps takes its written fit.
func rhmLabelNeedPt(ctx ExpandContext, content string, size float64, bold bool, widthPt float64) float64 {
	text := rhmLabelText(content, size, bold, "ctr", "ctr")
	if oneLine := rhmOneLinePt(size); writtenFitsAt(ctx.themeFonts(), text, widthPt, oneLine) {
		return oneLine
	}
	return rowTextNeedPt(ctx.themeFonts(), text, widthPt)
}

// rhmOneLinePt is the height of a box that holds one line at size.
func rhmOneLinePt(size float64) float64 {
	return math.Ceil(size*rhmOneLineFactor) + 2
}

// rhmOneLineWidthPt is the width, between minPt and maxPt, of a box that holds
// a 12pt label on one line inside the uniform side margins. The text width is
// measured in the theme face with a tenth of slack, since the writer and the
// fit findings each measure a little differently.
func rhmOneLineWidthPt(ctx ExpandContext, label string, minPt, maxPt float64) float64 {
	textW := 12.0
	for textW < maxPt && measuredLines(label, ctx.Theme.BodyFont, false, scaleBodyPt, textW) > 1 {
		textW += 3
	}
	return clampPt(math.Ceil(textW*1.1)+2*defaultShapeInsetLRPt, minPt, maxPt)
}

// place puts each risk in the cell its likelihood and impact name.
func (l *rhmLayout) place(items []RiskHeatmapItem) {
	size := l.size
	l.cells = make([][][]string, size)
	l.firstItem = make([][]int, size)
	for r := range l.cells {
		l.cells[r] = make([][]string, size)
		l.firstItem[r] = make([]int, size)
	}
	for i, it := range items {
		likelihood, okL := rhmResolveLevel(it.Likelihood, l.xLevels, size)
		impact, okI := rhmResolveLevel(it.Impact, l.yLevels, size)
		if !okL || !okI {
			continue
		}
		row := size - impact
		if len(l.cells[row][likelihood-1]) == 0 {
			l.firstItem[row][likelihood-1] = i
		}
		l.cells[row][likelihood-1] = append(l.cells[row][likelihood-1], strings.TrimSpace(it.Name))
	}
}

// rhmStep is one rung of the fit ladder: a name size and a cell top / bottom
// margin (0 = the uniform margin).
type rhmStep struct{ pt, pad float64 }

// rhmSteps is the fit ladder. Risk names start at the subhead size with the
// uniform cell margin; a grid that does not fit gives up cell padding first
// and type second, the way a ruled list does (rowPadStepsPt): 14pt at 10 and
// 7pt of padding, then the 12pt floor down to 5pt. An authored size is kept.
func rhmSteps(itemSize float64) []rhmStep {
	sizes := []float64{scaleSubheadPt, shapegrid.MinTextSizePt}
	if itemSize > 0 {
		sizes = []float64{shapegrid.EffectiveTextSizePt(itemSize)}
	}
	var steps []rhmStep
	for i, pt := range sizes {
		steps = append(steps, rhmStep{pt, 0})
		pads := rowPadStepsPt
		if i < len(sizes)-1 {
			pads = rowPadStepsPt[:len(rowPadStepsPt)-1]
		}
		for _, pad := range pads {
			steps = append(steps, rhmStep{pt, pad})
		}
	}
	return steps
}

// sizeRows takes the first step of the ladder at which the rows fit availPt,
// and leaves the last one's needs when none does.
func (l *rhmLayout) sizeRows(fonts pptx.ThemeFonts, labelNeeds []float64, itemSize float64) {
	// A name that only just fits one line by the writer's measure wraps by
	// another renderer's: rows are sized at a slightly narrower text width so
	// either reading has its height.
	needW := l.cellWPt - rhmWrapSlackFrac*(l.cellWPt-2*defaultShapeInsetLRPt)
	for _, st := range rhmSteps(itemSize) {
		l.itemPt, l.padPt = st.pt, st.pad
		l.rowNeeds = make([]float64, l.size)
		total := 0.0
		// Every row is at least one risk name high, filled or not: an empty
		// band is still a band of the grid.
		oneLine := rowTextNeedPt(fonts, rhmCellText([]string{"Risk"}, st.pt, st.pad, "dk1"), l.cellWPt)
		for r, row := range l.cells {
			l.rowNeeds[r] = math.Max(labelNeeds[r], oneLine)
			for _, names := range row {
				if len(names) > 0 {
					l.rowNeeds[r] = math.Max(l.rowNeeds[r], rowTextNeedPt(fonts, rhmCellText(names, st.pt, st.pad, "dk1"), needW))
				}
			}
			total += l.rowNeeds[r]
		}
		if total <= l.availPt {
			return
		}
	}
}

// namesWiderThanCell lists the names with a word too wide for a cell's text
// width at the chosen size.
func (l *rhmLayout) namesWiderThanCell(font string) []string {
	textW := l.cellWPt - 2*defaultShapeInsetLRPt
	var out []string
	for _, row := range l.cells {
		for _, names := range row {
			for _, name := range names {
				for _, word := range strings.Fields(name) {
					if measuredLines(word, font, false, l.itemPt, textW) > 1 {
						out = append(out, name)
						break
					}
				}
			}
		}
	}
	return out
}

// rhmMeasure places the risks and sizes the grid against the content area.
func rhmMeasure(ctx ExpandContext, v *RiskHeatmapValues, ovr *RiskHeatmapOverrides) rhmLayout {
	size := rhmSize(v)
	l := rhmLayout{
		size:    size,
		xLevels: rhmLevels(v.LikelihoodLevels, size),
		yLevels: rhmLevels(v.ImpactLevels, size),
		xTitle:  firstNonBlank(v.LikelihoodLabel, "Likelihood"),
		yTitle:  firstNonBlank(v.ImpactLabel, "Impact"),
	}
	l.place(v.Items)

	font := ctx.Theme.BodyFont
	fonts := ctx.themeFonts()
	areaW, areaH := contentAreaPt(ctx)
	gap := ctx.Gap(rhmGapPt)

	// The impact level column is as wide as its widest label needs.
	yLevelW := rhmMinYLevelWPt
	for _, label := range l.yLevels {
		yLevelW = math.Max(yLevelW, rhmOneLineWidthPt(ctx, label, rhmMinYLevelWPt, rhmMaxYLevelWPt))
	}
	usableW := areaW - float64(size+1)*gap
	l.cellWPt = math.Max((usableW-rhmYTitleWPt-yLevelW)/float64(size), 1)
	l.colsPct = []float64{pctOf(rhmYTitleWPt, usableW), pctOf(yLevelW, usableW)}
	for i := 0; i < size; i++ {
		l.colsPct = append(l.colsPct, pctOf(l.cellWPt, usableW))
	}

	for _, label := range l.xLevels {
		l.xLevelHPt = math.Max(l.xLevelHPt, rhmLabelNeedPt(ctx, label, scaleBodyPt, false, l.cellWPt))
	}
	l.xTitleHPt = rhmLabelNeedPt(ctx, l.xTitle, scaleSubheadPt, true, l.cellWPt*float64(size))
	fixed := l.xLevelHPt + l.xTitleHPt + float64(size+1)*gap
	if ovr.ShowLegend == nil || *ovr.ShowLegend {
		l.tierLabels = rhmDefaultTierLabels
		if len(v.TierLabels) == rhmTierCount {
			l.tierLabels = v.TierLabels
		}
		l.legendHPt = rhmLegendRowPt
		fixed += l.legendHPt + gap
	}
	l.availPt = areaH - fixed

	// An empty row still holds its level label.
	labelNeeds := make([]float64, size)
	for r := range labelNeeds {
		labelNeeds[r] = rhmLabelNeedPt(ctx, l.yLevels[size-1-r], scaleBodyPt, false, yLevelW)
	}
	l.sizeRows(fonts, labelNeeds, ovr.ItemSize)
	l.unfitNames = l.namesWiderThanCell(font)
	return l
}

// rhmRowHeights shares availPt among the rows: equal shares, except that a
// row whose need exceeds the share takes its need and the rest re-share what
// is left. Needs that do not fit at all are returned as they are.
func rhmRowHeights(needs []float64, availPt float64) []float64 {
	out := make([]float64, len(needs))
	taken := make([]bool, len(needs))
	remaining, free := availPt, len(needs)
	for free > 0 {
		share, changed := remaining/float64(free), false
		for i, need := range needs {
			if !taken[i] && need > share {
				out[i], taken[i], changed = need, true, true
				remaining -= need
				free--
			}
		}
		if !changed {
			for i := range needs {
				if !taken[i] {
					out[i] = math.Floor(share)
				}
			}
			break
		}
	}
	return out
}

// firstNonBlank returns the first value with visible text.
func firstNonBlank(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Fills
// ---------------------------------------------------------------------------

// rhmTierTone is the fill of a band: a neutral step for low, a light tint of
// the accent for medium and the accent itself for high, so the ramp reads
// light-to-dark on every template.
func rhmTierTone(accent string, tier int) fillTone {
	switch tier {
	case 0:
		return neutralTone(NeutralTint8)
	case 1:
		return chmTierTone(accent, 1)
	default:
		return fillTone{Color: accent}
	}
}

// rhmAccent is the colour the bands are a ramp of: an authored accent, else
// the template's negative accent — exposure is the "bad" direction whatever
// the palette — else the deck's default accent.
func rhmAccent(ctx ExpandContext, ovr *RiskHeatmapOverrides) string {
	semantic := ovr.SemanticAccent
	if ovr.Accent == "" && semantic == "" {
		semantic = "negative"
	}
	return ctx.ResolveAccent(ovr.Accent, semantic)
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (p *riskHeatmap) Expand(ctx ExpandContext, values, overrides any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*RiskHeatmapValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("%s: values must be *RiskHeatmapValues, got %T", rhmName, values)
	}
	if overrides != nil {
		if _, ok := overrides.(*RiskHeatmapOverrides); !ok {
			return nil, fmt.Errorf("%s: overrides must be *RiskHeatmapOverrides, got %T", rhmName, overrides)
		}
	}
	ovr := rhmResolveOverrides(overrides)
	accent := rhmAccent(ctx, ovr)
	l := rhmMeasure(ctx, v, ovr)
	n := l.size

	open := func(text json.RawMessage) *jsonschema.GridCellInput {
		return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine, Text: text}}
	}

	rows := make([]jsonschema.GridRowInput, 0, n+3)
	for r := 0; r < n; r++ {
		impact := n - r
		var cells []*jsonschema.GridCellInput
		if r == 0 {
			// The impact title reads bottom-to-top beside the rows it names;
			// only the text is turned, never the shape (J2P-MATRIX-005).
			title := open(buildMatrix2x2LabelContent(l.yTitle, scaleSubheadPt, "dk1", "ctr", "vert270"))
			title.RowSpan = n
			cells = append(cells, title)
		}
		cells = append(cells, open(rhmLabelText(l.yLevels[impact-1], scaleBodyPt, false, "r", "ctr")))
		for c := 0; c < n; c++ {
			tier := rhmTier(c+1, impact, n)
			tone := rhmTierTone(accent, tier)
			fallback := "dk1"
			if tier == rhmTierCount-1 {
				fallback = "lt1"
			}
			shape := &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: tone.fillJSON(), Line: noLine}
			if names := l.cells[r][c]; len(names) > 0 {
				shape.Text = rhmCellText(names, l.itemPt, l.padPt, readableTextOn(ctx, tone, fallback))
			}
			cells = append(cells, &jsonschema.GridCellInput{Shape: shape})
		}
		rows = append(rows, jsonschema.GridRowInput{Cells: cells})
	}
	// The rows share the height equally; a crowded one takes what it needs
	// and the others share the rest equally, so sparse rows stay rows of the
	// same grid instead of collapsing to their label.
	for r, h := range rhmRowHeights(l.rowNeeds, l.availPt) {
		rows[r].MinHeight = math.Ceil(l.rowNeeds[r])
		rows[r].Flex = h
	}

	levelCells := []*jsonschema.GridCellInput{{}, {}}
	for _, label := range l.xLevels {
		levelCells = append(levelCells, open(rhmLabelText(label, scaleBodyPt, false, "ctr", "t")))
	}
	rows = append(rows, jsonschema.GridRowInput{MinHeight: l.xLevelHPt, MaxHeight: l.xLevelHPt, Cells: levelCells})

	xTitle := open(rhmLabelText(l.xTitle, scaleSubheadPt, true, "ctr", "ctr"))
	xTitle.ColSpan = n
	rows = append(rows, jsonschema.GridRowInput{MinHeight: l.xTitleHPt, MaxHeight: l.xTitleHPt, Cells: []*jsonschema.GridCellInput{{}, {}, xTitle}})

	if l.legendHPt > 0 {
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: l.legendHPt,
			MaxHeight: l.legendHPt,
			Cells:     []*jsonschema.GridCellInput{rhmLegendCell(ctx, l, accent)},
		})
	}

	colsJSON, _ := json.Marshal(l.colsPct)
	gap := ctx.Gap(rhmGapPt)
	return &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  gap,
		RowGap:  gap,
		Rows:    rows,
	}, nil
}

// rhmLegendCell is the band legend: [swatch, label] pairs low → high in one
// line on the left under the grid, spanning it.
func rhmLegendCell(ctx ExpandContext, l rhmLayout, accent string) *jsonschema.GridCellInput {
	areaW, _ := contentAreaPt(ctx)
	// The legend starts where the cells do, under the first likelihood column.
	lead := l.colsPct[0] + l.colsPct[1]
	cells := []*jsonschema.GridCellInput{{}}
	cols := []float64{lead}
	used := lead
	for i, label := range l.tierLabels {
		// The nested grid is inset inside its cell, so a column is a little
		// narrower than its share of the content width: leave that room.
		labelW := rhmOneLineWidthPt(ctx, label, 60, 260) + 2*SubGridInsetPt
		cells = append(cells,
			&jsonschema.GridCellInput{MaxHeight: rhmLegendSwatchW, Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: rhmTierTone(accent, i).fillJSON(), Line: noLine}},
			&jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine,
				Text: rhmLabelText(label, scaleBodyPt, false, "l", "ctr")}})
		cols = append(cols, pctOf(rhmLegendSwatchW, areaW), pctOf(labelW, areaW))
		used += cols[len(cols)-2] + cols[len(cols)-1]
	}
	// The rest of the row stays empty so the legend keeps its natural width.
	cells = append(cells, &jsonschema.GridCellInput{})
	cols = append(cols, math.Max(100-used, 1))
	colsJSON, _ := json.Marshal(cols)
	return &jsonschema.GridCellInput{
		ColSpan: l.size + 2,
		Grid: &jsonschema.ShapeGridInput{
			Columns:       colsJSON,
			ColGap:        0,
			Rows:          []jsonschema.GridRowInput{{MinHeight: rhmOneLinePt(scaleBodyPt), MaxHeight: rhmOneLinePt(scaleBodyPt), Cells: cells}},
			VerticalAlign: "bottom",
		},
	}
}

// ---------------------------------------------------------------------------
// Post-expand warnings
// ---------------------------------------------------------------------------

// PostExpandWarnings reports what the measurement found that the renderer
// cannot rescue: cells whose risks need more height than the grid has even at
// the 12pt floor, and a name with a word wider than its cell.
func (p *riskHeatmap) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*RiskHeatmapValues)
	if !ok || v == nil || len(v.Items) == 0 {
		return nil
	}
	l := rhmMeasure(ctx, v, rhmResolveOverrides(overrides))
	var out []string
	if len(l.unfitNames) > 0 {
		out = append(out, fmt.Sprintf(
			"%s: risk-heatmap name %s has a word too wide for its cell on a %d × %d grid even at %.0fpt — the renderer breaks it mid-word; shorten or hyphenate the name",
			ErrCodeTextExceedsShape, listFirstN(l.unfitNames, 3), l.size, l.size, l.itemPt))
	}
	total := 0.0
	for _, need := range l.rowNeeds {
		total += need
	}
	if l.availPt > 0 && total > l.availPt+1 {
		row, col, most := 0, 0, 0
		for r, cells := range l.cells {
			for c, names := range cells {
				if len(names) > most {
					row, col, most = r, c, len(names)
				}
			}
		}
		out = append(out, fmt.Sprintf(
			"%s: risk-heatmap items[%d].name shares the %s likelihood / %s impact cell with %d more risks; the %d rows need about %.0fpt at %.0fpt in their tightest rows but the grid holds %.0fpt, so the names shrink below the floor — shorten the names, keep the top risks on the map and the rest in a risk register table, or free height (drop the takeaway, hide the legend)",
			ErrCodeBodyTooLong, l.firstItem[row][col], strings.ToLower(l.xLevels[col]), strings.ToLower(l.yLevels[l.size-1-row]), most-1, l.size, total, l.itemPt, l.availPt))
	}
	return out
}
