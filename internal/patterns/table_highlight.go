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
)

// ---------------------------------------------------------------------------
// table-highlight pattern — options × criteria evaluation matrix with Harvey
// balls, RAG dots or short text, a highlighted (recommended) option row and an
// optional highlighted criterion column.
// ---------------------------------------------------------------------------
//
// Layout:
//
//   | Option      | Cost | Speed | Risk |      ← dk2 header (highlight column: accent)
//   | Option A    |  ◕   |  ●    |  ◑   |      ← alternating light banding
//   ▌Option B ★   |  ●   |  ◕    |  ●   |      ← highlighted row: accent tint + bar
//   | Option C    |  ◔   |  ◑    |  ○   |
//     ● Fully meets  ◑ Partially meets  ○ Does not meet   ← legend
//
// Harvey balls and RAG dots are drawn as inline SVG icons (resolved theme hex
// for Harvey balls, conventional status colours for RAG) centred on the cell
// shape, so the row fill stays behind them. Rows are content-sized from the
// measured option / criterion text and pinned in points (min_height =
// max_height), so the table never stretches to full height and is centred by
// the pattern vertical_align default.

func init() {
	Default().Register(&tableHighlight{})
}

type tableHighlight struct{}

// Pattern budgets.
const (
	thMinOptions     = 2
	thMaxOptions     = 6
	thMinCriteria    = 2
	thMaxCriteria    = 6
	thNameMax        = 40
	thDetailMax      = 80
	thDenseDetailMax = 60
	thCriterionMax   = 30
	thTextCellMax    = 24
	thLabelMax       = 24
	thLegendMax      = 20

	thColGapPt    = 3.0
	thRowGapPt    = 3.0
	thMinRowPt    = 38.0
	thMinHeaderPt = 34.0
	// thLegendPt is the height reserved per legend row. It must be generous:
	// the table's rows are scaled down proportionally when they over-fill, and
	// at 32pt the legend cell resolved to ~18pt, leaving a 10pt nested row that
	// shrank its labels to ~2pt (go-slide-creator-z0up).
	thLegendPt = 64.0
	// thLegendSwatchPct / thLegendLabelPct are the swatch and label column
	// widths of a legend row, as a percentage of the table. Three entries per
	// row (one scale) leaves room for readable words; six squeezed them to ~3pt.
	thLegendSwatchPct = 2.5
	thLegendLabelPct  = 30.0
	// thLegendRowPt is the height of the nested legend row inside the legend
	// cell, which otherwise collapses and shrinks its labels to illegibility.
	thLegendRowPt = 24.0
	thSymbolPt    = 22.0 // rendered Harvey ball / RAG dot diameter
	thMinFillPct  = 55.0 // pad body rows until the table covers this share of the content height
	thMaxPad      = 1.5
)

// Score scales.
const (
	thScaleHarvey = "harvey"
	thScaleRAG    = "rag"
	thScaleText   = "text"
)

// Conventional RAG status colours. Status semantics must read the same on
// every template, so these are the one non-theme palette the pattern uses;
// overrides.rag_colors swaps any of them for scheme names or other hex values.
var thDefaultRAG = map[string]string{
	"red":   "#C62828",
	"amber": "#F2A900",
	"green": "#2E7D32",
}

func (p *tableHighlight) Name() string { return "table-highlight" }
func (p *tableHighlight) Description() string {
	return "Options × criteria evaluation matrix (2-6 × 2-6) scored with Harvey balls, RAG dots or short text, with a highlighted recommended row and optional highlighted criterion column"
}
func (p *tableHighlight) UseWhen() string {
	return "Evaluating 2–6 options against 2–6 criteria where each cell is a rating (Harvey ball 0–4, RAG status, or a short value) and one option is recommended; prefer comparison-2col for two options described in prose, horizontal-bar-with-callouts for a single ranked score per item, and a plain table content block for raw numeric data"
}
func (p *tableHighlight) NotWhen() string {
	return "Only two options described in sentences (use comparison-2col), one score per item (use horizontal-bar-with-callouts), dense numeric data (use a table content block or chart), or more than 6 options / criteria (split the matrix)"
}
func (p *tableHighlight) Version() int      { return 1 }
func (p *tableHighlight) CellsHint() string { return "(2-6 options + header) × (2-6 criteria + name)" }
func (p *tableHighlight) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "data-display",
		NarrativeRole: []string{"compare", "evidence", "conclude"},
		PairsWith:     []string{"exec-summary", "comparison-2col", "phase-roadmap"},
		DensityClass:  "high",
		AccentWeight:  "normal",
		DataVisual:    true,
	}
}

func (p *tableHighlight) SupportsInlineMarkdown() bool { return true }

func (p *tableHighlight) ExemplarValues() any {
	one := 1
	return &TableHighlightValues{
		Criteria: []TableHighlightCriterion{{Label: "Cost"}, {Label: "Time to value"}, {Label: "Scalability"}, {Label: "Delivery risk"}},
		Options: []TableHighlightOption{
			{Name: "Build in-house", Detail: "Custom platform", Scores: []TableHighlightScore{"1", "1", "4", "2"}},
			{Name: "Buy SaaS suite", Detail: "Configure and integrate", Scores: []TableHighlightScore{"3", "4", "3", "3"}},
			{Name: "Partner / JV", Detail: "Shared investment", Scores: []TableHighlightScore{"2", "2", "3", "1"}},
		},
		HighlightRow:   &one,
		HighlightLabel: "Recommended",
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// TableHighlightScore is one cell value. JSON numbers and strings are both
// accepted (Harvey levels are usually written as numbers).
type TableHighlightScore string

// UnmarshalJSON accepts a JSON string or number.
func (s *TableHighlightScore) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		*s = TableHighlightScore(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(b, &num); err != nil {
		return fmt.Errorf("table-highlight: score must be a string or number, got %s", string(b))
	}
	*s = TableHighlightScore(num.String())
	return nil
}

// TableHighlightCriterion is a column: a label plus an optional per-column
// scale overriding values.scale.
type TableHighlightCriterion struct {
	Label string `json:"label"`
	Scale string `json:"scale,omitempty"`
}

// UnmarshalJSON accepts a bare string label or the object form.
func (c *TableHighlightCriterion) UnmarshalJSON(b []byte) error {
	var label string
	if err := json.Unmarshal(b, &label); err == nil {
		*c = TableHighlightCriterion{Label: label}
		return nil
	}
	type plain TableHighlightCriterion
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("table-highlight: criteria entries must be a string or {label, scale}: %w", err)
	}
	*c = TableHighlightCriterion(p)
	return nil
}

// TableHighlightOption is a row: option name, optional one-line detail, and
// one score per criterion.
type TableHighlightOption struct {
	Name   string                `json:"name"`
	Detail string                `json:"detail,omitempty"`
	Scores []TableHighlightScore `json:"scores"`
}

// TableHighlightValues holds the evaluation matrix.
type TableHighlightValues struct {
	Criteria       []TableHighlightCriterion `json:"criteria"`
	Options        []TableHighlightOption    `json:"options"`
	Scale          string                    `json:"scale,omitempty"` // harvey (default) | rag | text
	HighlightRow   *int                      `json:"highlight_row,omitempty"`
	HighlightCol   *int                      `json:"highlight_col,omitempty"`
	HighlightLabel string                    `json:"highlight_label,omitempty"`
	CornerLabel    string                    `json:"corner_label,omitempty"`
	ShowLegend     *bool                     `json:"show_legend,omitempty"`
	LegendLabels   []string                  `json:"legend_labels,omitempty"` // [high, mid, low]
	// LegendLabelsRAG is the wording for the RAG swatches, also [high, mid, low]
	// — i.e. [green, amber, red]. A deck mixing harvey and RAG columns used to
	// reuse LegendLabels for BOTH sets, so a green dot carried the harvey
	// "does not meet" label: a legend that says the opposite of the chart
	// (go-slide-creator-z0up).
	LegendLabelsRAG []string `json:"legend_labels_rag,omitempty"` // [green, amber, red]
}

// TableHighlightOverrides are the pattern-level overrides.
type TableHighlightOverrides struct {
	Accent         string            `json:"accent,omitempty"`
	SemanticAccent string            `json:"semantic_accent,omitempty"`
	HeaderSize     float64           `json:"header_size,omitempty"`
	BodySize       float64           `json:"body_size,omitempty"`
	RAGColors      map[string]string `json:"rag_colors,omitempty"`
}

// TableHighlightCellOverride is the shared per-cell override, indexed by
// option row (applied to the option-name cell).
type TableHighlightCellOverride = CellOverride

func (p *tableHighlight) NewValues() any       { return &TableHighlightValues{} }
func (p *tableHighlight) NewOverrides() any    { return &TableHighlightOverrides{} }
func (p *tableHighlight) NewCellOverride() any { return &TableHighlightCellOverride{} }

// The paired name/detail target was measured with the fit collector on all
// four bundled templates. It is guidance for a dense matrix, not a per-field
// validation limit: a sparse row can use its full schema maxima.
func tableHighlightPairedCopyBudget(options, criteria int) int {
	if options <= 4 {
		return 40
	}
	if options == 5 {
		if criteria >= 5 {
			return 37
		}
		return 40
	}
	if criteria >= 5 {
		return 30
	}
	if criteria == 4 {
		return 33
	}
	return 35
}

// TableHighlightDetailLimit permits a longer descriptor only when the matrix
// has enough space to widen the option column without crowding score columns.
func TableHighlightDetailLimit(options, criteria int) int {
	if options <= 4 && criteria <= 4 {
		return thDetailMax
	}
	return thDenseDetailMax
}

func (p *tableHighlight) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*TableHighlightValues)
	if !ok || v == nil {
		return nil
	}
	budget := tableHighlightPairedCopyBudget(len(v.Options), len(v.Criteria))
	if budget >= thNameMax {
		return nil
	}
	var warnings []string
	for i, option := range v.Options {
		// The two paragraphs share a row. A highlighted tag can make one
		// otherwise tall row fit because the renderer gives it its own height.
		if i == highlightRow(v) && v.HighlightLabel != "" &&
			(len(v.Options) == 5 || len(v.Criteria) <= 4) {
			continue
		}
		if runeLen(option.Name)+runeLen(option.Detail) > 2*budget {
			warnings = append(warnings, fmt.Sprintf("%s: table-highlight options[%d].name/detail use %d/%d characters; a %d-option x %d-criterion matrix holds about %d characters each when both are populated — shorten the option copy, hide the legend, or split the table", ErrCodeBodyTooLong, i, runeLen(option.Name), runeLen(option.Detail), len(v.Options), len(v.Criteria), budget))
		}
	}
	return warnings
}

func highlightRow(v *TableHighlightValues) int {
	if v.HighlightRow != nil {
		return *v.HighlightRow
	}
	return -1
}

func (p *tableHighlight) Schema() *Schema {
	scaleEnum := func() *Schema { return EnumSchema(thScaleHarvey, thScaleRAG, thScaleText) }
	criterion := OneOfSchema(
		StringSchema(thCriterionMax),
		ObjectSchema(map[string]*Schema{
			"label": StringSchema(thCriterionMax),
			"scale": scaleEnum().WithDescription("Per-column scale overriding values.scale"),
		}, []string{"label"}).WithAdditionalProperties(false),
	).WithDescription("Criterion column: label string or {label, scale}")

	score := OneOfSchema(NumberSchema(0, 4), StringSchema(thTextCellMax)).
		WithDescription("harvey: 0-4 (or none/quarter/half/three-quarter/full); rag: red/amber/green (r/a/g); text: ≤24 chars; any scale: \"-\" or \"n/a\" for not applicable").WithDefault(0)

	option := ObjectSchema(map[string]*Schema{
		"name":   StringSchema(thNameMax).WithDescription("Option name (≤40 chars); dense paired name/detail targets depend on matrix shape"),
		"detail": StringSchema(thDetailMax).WithDescription("Optional descriptor under the name (≤80 chars for up to 4 options × 4 criteria; ≤60 in denser matrices)"),
		"scores": ArraySchema(score, thMinCriteria, thMaxCriteria).WithDescription("One score per criterion, in criteria order"),
	}, []string{"name", "scores"}).WithAdditionalProperties(false).
		WithDescription("Paired name/detail readable characters by option rows x criteria: 2-4 rows about 40 each; 5 rows with 5-6 criteria about 37; 6 rows with 2-3/4/5-6 criteria about 35/33/30. Sparse rows can use field maxima; fit reports flag copy beyond dense paired targets")

	valuesSchema := ObjectSchema(map[string]*Schema{
		"criteria":          ArraySchema(criterion, thMinCriteria, thMaxCriteria).WithDescription("2-6 criteria (columns)"),
		"options":           ArraySchema(option, thMinOptions, thMaxOptions).WithDescription("2-6 options (rows)"),
		"scale":             scaleEnum().WithDescription("Default cell scale (default harvey)").WithDefault(thScaleHarvey),
		"highlight_row":     IntegerSchema(0, thMaxOptions-1).WithDescription("0-based option row to highlight (recommended option)"),
		"highlight_col":     IntegerSchema(0, thMaxCriteria-1).WithDescription("0-based criterion column to highlight (decisive criterion)"),
		"highlight_label":   StringSchema(thLabelMax).WithDescription("Tag shown under the highlighted option name, e.g. \"Recommended\""),
		"corner_label":      StringSchema(thLabelMax).WithDescription("Header of the option column (default \"Option\")"),
		"show_legend":       BooleanSchema().WithDescription("Legend row under the table for harvey / rag columns (default true)"),
		"legend_labels":     ArraySchema(StringSchema(thLegendMax), 3, 3).WithDescription("Legend wording for the Harvey-ball scale, in the order [HIGH, MID, LOW] — the full ball first, the empty ball last. Default: [\"Fully meets\", \"Partially meets\", \"Does not meet\"]."),
		"legend_labels_rag": ArraySchema(StringSchema(thLegendMax), 3, 3).WithDescription("Legend wording for the RAG scale, in the same [HIGH, MID, LOW] order — i.e. [green, amber, red]. Default: [\"Green\", \"Amber\", \"Red\"]. Set this when the deck mixes harvey and rag columns: the two scales get their own legend row and their own words."),
	}, []string{"criteria", "options"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":          StringSchema(0).WithDescription("Accent for the highlighted row / column (default accent1)").WithDefault("accent1"),
		"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"header_size":     NumberSchema(12, 28).WithDescription("Header font size in points (default 13)"),
		"body_size":       NumberSchema(12, 28).WithDescription("Option name / text cell font size in points (default 14)"),
		"rag_colors": ObjectSchema(map[string]*Schema{
			"red": StringSchema(0), "amber": StringSchema(0), "green": StringSchema(0),
		}, nil).WithAdditionalProperties(false).WithDescription("Replace the conventional RAG status colours (scheme name or hex)"),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":         valuesSchema,
		"overrides":      overridesSchema,
		"cell_overrides": CellOverridesSchema("cellOverride"),
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Options × criteria evaluation matrix with Harvey balls / RAG / text cells and a highlighted recommended row")
}

// thNormalizedScore is a parsed cell: kind is the resolved scale or "na".
type thNormalizedScore struct {
	kind  string // harvey | rag | text | na
	level int    // harvey 0-4
	rag   string // red | amber | green
	text  string
}

var thHarveyWords = map[string]int{
	"none": 0, "empty": 0, "quarter": 1, "half": 2,
	"three-quarter": 3, "three-quarters": 3, "three_quarters": 3, "threequarter": 3, "full": 4,
}

var thRAGWords = map[string]string{
	"red": "red", "r": "red", "amber": "amber", "a": "amber", "yellow": "amber",
	"green": "green", "g": "green",
}

// parseTableHighlightScore normalises a raw score for a scale.
func parseTableHighlightScore(raw TableHighlightScore, scale string) (thNormalizedScore, bool) {
	s := strings.TrimSpace(string(raw))
	switch strings.ToLower(s) {
	case "-", "–", "—", "n/a", "na":
		return thNormalizedScore{kind: "na"}, true
	}
	switch scale {
	case thScaleRAG:
		if c, ok := thRAGWords[strings.ToLower(s)]; ok {
			return thNormalizedScore{kind: thScaleRAG, rag: c}, true
		}
		return thNormalizedScore{}, false
	case thScaleText:
		if s == "" || runeLen(s) > thTextCellMax {
			return thNormalizedScore{}, false
		}
		return thNormalizedScore{kind: thScaleText, text: s}, true
	default:
		if lvl, ok := thHarveyWords[strings.ToLower(s)]; ok {
			return thNormalizedScore{kind: thScaleHarvey, level: lvl}, true
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f != math.Trunc(f) || f < 0 || f > 4 {
			return thNormalizedScore{}, false
		}
		return thNormalizedScore{kind: thScaleHarvey, level: int(f)}, true
	}
}

func (v *TableHighlightValues) scaleFor(col int) string {
	if col < len(v.Criteria) && v.Criteria[col].Scale != "" {
		return v.Criteria[col].Scale
	}
	if v.Scale != "" {
		return v.Scale
	}
	return thScaleHarvey
}

func validScale(s string) bool {
	return s == "" || s == thScaleHarvey || s == thScaleRAG || s == thScaleText
}

func (p *tableHighlight) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*TableHighlightValues)
	if !ok || v == nil {
		return fmt.Errorf("table-highlight: values must be *TableHighlightValues, got %T", values)
	}
	var errs []error
	if overrides != nil {
		ovr, ok := overrides.(*TableHighlightOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("table-highlight: overrides must be *TableHighlightOverrides, got %T", overrides))
		} else {
			errs = append(errs, thValidateRAGColors(ovr.RAGColors)...)
		}
	}
	errs = append(errs, thValidateCriteria(v)...)
	errs = append(errs, thValidateOptions(v)...)
	errs = append(errs, thValidateExtras(v)...)
	if coErr := validateCellOverrideKeys(thName, cellOverrides, len(v.Options), "(index = option row)"); coErr != nil {
		errs = append(errs, coErr)
	}
	return errors.Join(errs...)
}

const thName = "table-highlight"

func thScaleError(path, got string) error {
	return newValidationError(thName, path, ErrCodeUnknownEnum,
		fmt.Sprintf("%s: %s must be one of harvey, rag, text; got %q", thName, path, got),
		UseOneOfFix(path, []string{thScaleHarvey, thScaleRAG, thScaleText}))
}

func thValidateRAGColors(colors map[string]string) []error {
	var errs []error
	for k, c := range colors {
		path := "overrides.rag_colors." + k
		if _, known := thDefaultRAG[k]; !known {
			errs = append(errs, errUnknownKey(thName, "overrides.rag_colors", k, "amber, green, red"))
		} else if !isCardGridColor(c) {
			errs = append(errs, newValidationError(thName, path, ErrCodeOutOfRange,
				fmt.Sprintf("%s: %s must be a scheme colour name or hex, got %q", thName, path, c), ProvideValueFix(path)))
		}
	}
	return errs
}

func thValidateCriteria(v *TableHighlightValues) []error {
	var errs []error
	if !validScale(v.Scale) {
		errs = append(errs, thScaleError("scale", v.Scale))
	}
	if len(v.Criteria) < thMinCriteria {
		errs = append(errs, errMinItems(thName, "criteria", thMinCriteria, len(v.Criteria), "(hint: one criterion is a ranking — use horizontal-bar-with-callouts)"))
	}
	if len(v.Criteria) > thMaxCriteria {
		errs = append(errs, errMaxItems(thName, "criteria", thMaxCriteria, len(v.Criteria), "(hint: group criteria or split the matrix across two slides)"))
	}
	for i, c := range v.Criteria {
		path := fmt.Sprintf("criteria[%d]", i)
		if strings.TrimSpace(c.Label) == "" {
			errs = append(errs, errRequired(thName, path+".label"))
		} else if runeLen(c.Label) > thCriterionMax {
			errs = append(errs, errMaxLength(thName, path+".label", thCriterionMax, runeLen(c.Label)))
		}
		if !validScale(c.Scale) {
			errs = append(errs, thScaleError(path+".scale", c.Scale))
		}
	}
	return errs
}

func thValidateOptions(v *TableHighlightValues) []error {
	var errs []error
	if len(v.Options) < thMinOptions {
		errs = append(errs, errMinItems(thName, "options", thMinOptions, len(v.Options), "(hint: describe a single option with card-grid or stat-hero)"))
	}
	if len(v.Options) > thMaxOptions {
		errs = append(errs, errMaxItems(thName, "options", thMaxOptions, len(v.Options), "(hint: shortlist to 6 options or split across slides)"))
	}
	for i, o := range v.Options {
		path := fmt.Sprintf("options[%d]", i)
		if strings.TrimSpace(o.Name) == "" {
			errs = append(errs, errRequired(thName, path+".name"))
		} else if runeLen(o.Name) > thNameMax {
			errs = append(errs, errMaxLength(thName, path+".name", thNameMax, runeLen(o.Name)))
		}
		if limit := TableHighlightDetailLimit(len(v.Options), len(v.Criteria)); runeLen(o.Detail) > limit {
			errs = append(errs, errMaxLength(thName, path+".detail", limit, runeLen(o.Detail)))
		}
		if len(o.Scores) != len(v.Criteria) {
			errs = append(errs, errCountMismatch(thName, path+".scores", len(v.Criteria), len(o.Scores), "(one score per criterion)"))
			continue
		}
		errs = append(errs, thValidateScores(v, path, o.Scores)...)
	}
	return errs
}

func thValidateScores(v *TableHighlightValues, path string, scores []TableHighlightScore) []error {
	var errs []error
	for j, sc := range scores {
		scale := v.scaleFor(j)
		if !validScale(scale) {
			continue // already reported on the criterion
		}
		if _, ok := parseTableHighlightScore(sc, scale); !ok {
			spath := fmt.Sprintf("%s.scores[%d]", path, j)
			errs = append(errs, newValidationError(thName, spath, ErrCodeOutOfRange,
				fmt.Sprintf("%s: %s = %q is not a valid %s score (%s)", thName, spath, string(sc), scale, thScaleHelp(scale)), ProvideValueFix(spath)))
		}
	}
	return errs
}

func thValidateExtras(v *TableHighlightValues) []error {
	var errs []error
	if v.HighlightRow != nil && (*v.HighlightRow < 0 || *v.HighlightRow >= len(v.Options)) {
		errs = append(errs, errOutOfRange(thName, "highlight_row", 0, len(v.Options)-1, *v.HighlightRow))
	}
	if v.HighlightCol != nil && (*v.HighlightCol < 0 || *v.HighlightCol >= len(v.Criteria)) {
		errs = append(errs, errOutOfRange(thName, "highlight_col", 0, len(v.Criteria)-1, *v.HighlightCol))
	}
	if runeLen(v.HighlightLabel) > thLabelMax {
		errs = append(errs, errMaxLength(thName, "highlight_label", thLabelMax, runeLen(v.HighlightLabel)))
	}
	if runeLen(v.CornerLabel) > thLabelMax {
		errs = append(errs, errMaxLength(thName, "corner_label", thLabelMax, runeLen(v.CornerLabel)))
	}
	if len(v.LegendLabels) != 0 && len(v.LegendLabels) != 3 {
		errs = append(errs, errCountMismatch(thName, "legend_labels", 3, len(v.LegendLabels), "([high, mid, low])"))
	}
	for i, l := range v.LegendLabels {
		if runeLen(l) > thLegendMax {
			errs = append(errs, errMaxLength(thName, fmt.Sprintf("legend_labels[%d]", i), thLegendMax, runeLen(l)))
		}
	}
	if len(v.LegendLabelsRAG) != 0 && len(v.LegendLabelsRAG) != 3 {
		errs = append(errs, errCountMismatch(thName, "legend_labels_rag", 3, len(v.LegendLabelsRAG), "([green, amber, red])"))
	}
	for i, l := range v.LegendLabelsRAG {
		if runeLen(l) > thLegendMax {
			errs = append(errs, errMaxLength(thName, fmt.Sprintf("legend_labels_rag[%d]", i), thLegendMax, runeLen(l)))
		}
	}
	return errs
}

func thScaleHelp(scale string) string {
	switch scale {
	case thScaleRAG:
		return `red / amber / green, r / a / g, or "-"`
	case thScaleText:
		return fmt.Sprintf("non-empty, ≤%d chars", thTextCellMax)
	default:
		return `integer 0-4, none / quarter / half / three-quarter / full, or "-"`
	}
}

// thLayout is the resolved geometry and styling of one table expansion.
type thLayout struct {
	ctx        ExpandContext
	v          *TableHighlightValues
	ovr        *TableHighlightOverrides
	accent     string
	hlRow      int
	hlCol      int
	cols       []float64
	areaW      float64
	areaH      float64
	corner     string
	symbolInk  string
	legend     []string // symbol scales shown in the legend (empty = no legend)
	headerSize float64
	bodySize   float64
	detailSize float64
	headerPt   float64
	rowPt      []float64
	gapsPt     float64
	fixedPt    float64
}

func (l *thLayout) colW(i int) float64 {
	return (l.areaW - thColGapPt*float64(len(l.cols)-1)) * l.cols[i] / 100
}

func (l *thLayout) bodyTotal() float64 {
	t := 0.0
	for _, h := range l.rowPt {
		t += h
	}
	return t
}

func (l *thLayout) total() float64 { return l.fixedPt + l.bodyTotal() }

// newTHLayout resolves column geometry, highlight indices and the legend.
func newTHLayout(ctx ExpandContext, v *TableHighlightValues, ovr *TableHighlightOverrides) *thLayout {
	l := &thLayout{ctx: ctx, v: v, ovr: ovr, accent: ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent), hlRow: -1, hlCol: -1}
	if v.HighlightRow != nil {
		l.hlRow = *v.HighlightRow
	}
	if v.HighlightCol != nil {
		l.hlCol = *v.HighlightCol
	}
	// The name column narrows as criteria are added but never below ~26% so
	// option names stay on one or two lines.
	nCrit := len(v.Criteria)
	nameColPct := 30.0
	switch {
	case nCrit >= 5:
		nameColPct = 26
	case nCrit == 4:
		nameColPct = 28
	}
	if len(v.Options) <= 4 {
		for _, option := range v.Options {
			if runeLen(option.Detail) <= thDenseDetailMax {
				continue
			}
			switch {
			case nCrit <= 3:
				nameColPct = 36
			case nCrit == 4:
				nameColPct = 32
			}
			break
		}
	}
	l.cols = append(l.cols, nameColPct)
	for i := 0; i < nCrit; i++ {
		l.cols = append(l.cols, (100-nameColPct)/float64(nCrit))
	}
	l.areaW, l.areaH = sizingAreaPt(ctx)
	l.symbolInk = inkOnLight(ctx, "dk2", 4.5)
	l.corner = v.CornerLabel
	if strings.TrimSpace(l.corner) == "" {
		l.corner = "Option"
	}
	if kinds := thLegendKinds(v); len(kinds) > 0 && (v.ShowLegend == nil || *v.ShowLegend) {
		l.legend = kinds
	}
	// One row per legend scale (go-slide-creator-z0up).
	rowCount := 1 + len(v.Options) + len(l.legend)
	l.gapsPt = thRowGapPt * float64(rowCount-1)
	return l
}

// measure sets the header height and body row heights at a type scale:
// uniform ordinary rows (the tallest sets them, so it reads as a table) and
// the highlighted row at its own height (it may carry an extra tag line).
func (l *thLayout) measure(headerSize, bodySize, detailSize float64) {
	ctx, v := l.ctx, l.v
	l.headerSize, l.bodySize, l.detailSize = headerSize, bodySize, detailSize
	l.headerPt = math.Max(thMinHeaderPt, sizedBlockHeightPt(ctx, []sizedPara{{text: l.corner, sizePt: headerSize, bold: true}}, l.colW(0)))
	for j, c := range v.Criteria {
		l.headerPt = math.Max(l.headerPt, sizedBlockHeightPt(ctx, []sizedPara{{text: c.Label, sizePt: headerSize, bold: true}}, l.colW(j+1)))
	}
	rowPt, hlPt := thMinRowPt, 0.0
	for i, o := range v.Options {
		h := l.optionHeight(o, bodySize, detailSize)
		if i == l.hlRow && v.HighlightLabel != "" {
			hlPt = h + detailSize*sizingLineSpacing
			continue
		}
		rowPt = math.Max(rowPt, h)
	}
	hlPt = math.Max(hlPt, rowPt)
	l.rowPt = make([]float64, len(v.Options))
	for i := range l.rowPt {
		l.rowPt[i] = rowPt
		if i == l.hlRow {
			l.rowPt[i] = hlPt
		}
	}
	l.fixedPt = l.headerPt + l.gapsPt
	l.fixedPt += float64(len(l.legend)) * thLegendPt
}

// optionHeight is the natural height of one option row's name / detail /
// text cells.
func (l *thLayout) optionHeight(o TableHighlightOption, bodySize, detailSize float64) float64 {
	paras := []sizedPara{{text: o.Name, sizePt: bodySize, bold: true}}
	if o.Detail != "" {
		paras = append(paras, sizedPara{text: o.Detail, sizePt: detailSize})
	}
	h := sizedBlockHeightPt(l.ctx, paras, l.colW(0)-6)
	for j := range l.v.Criteria {
		if sc, ok := parseTableHighlightScore(safeScore(o.Scores, j), l.v.scaleFor(j)); ok && sc.kind == thScaleText {
			h = math.Max(h, sizedBlockHeightPt(l.ctx, []sizedPara{{text: sc.text, sizePt: bodySize}}, l.colW(j+1)))
		}
	}
	return h
}

// fit picks the largest type-scale step whose table fits the content area
// (explicit header_size / body_size pin it; nothing steps below 12pt), then
// pads short tables so they do not read as a sliver.
func (l *thLayout) fit() {
	steps := [][3]float64{{13, 14, 12}, {12, 13, 12}, {12, 12, 12}}
	if l.ovr.HeaderSize > 0 || l.ovr.BodySize > 0 {
		steps = [][3]float64{{ResolveSize(l.ovr.HeaderSize, 13), ResolveSize(l.ovr.BodySize, 14), 12}}
	}
	for _, st := range steps {
		l.measure(st[0], st[1], st[2])
		if l.total() <= l.areaH {
			break
		}
	}
	if target := l.areaH * thMinFillPct / 100; l.total() < target {
		scale := math.Min((target-l.fixedPt)/l.bodyTotal(), thMaxPad)
		for i := range l.rowPt {
			l.rowPt[i] *= scale
		}
	}
}

func (p *tableHighlight) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*TableHighlightValues)
	if !ok {
		return nil, fmt.Errorf("table-highlight: values must be *TableHighlightValues, got %T", values)
	}
	ovr := &TableHighlightOverrides{}
	if overrides != nil {
		if ovr, ok = overrides.(*TableHighlightOverrides); !ok {
			return nil, fmt.Errorf("table-highlight: overrides must be *TableHighlightOverrides, got %T", overrides)
		}
	}

	l := newTHLayout(ctx, v, ovr)
	l.fit()

	rows := make([]jsonschema.GridRowInput, 0, len(v.Options)+2)
	rows = append(rows, jsonschema.GridRowInput{MinHeight: l.headerPt, MaxHeight: l.headerPt, Cells: l.headerCells()})
	for i := range v.Options {
		rows = append(rows, jsonschema.GridRowInput{MinHeight: l.rowPt[i], MaxHeight: l.rowPt[i], Cells: l.optionCells(i, cellOverrides)})
	}
	// One row per scale. Both scales on one row gave each label a twelfth of the
	// table and squeezed the text to ~3pt; a row of its own gives each label a
	// third (go-slide-creator-z0up).
	for _, kind := range l.legend {
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: thLegendPt, MaxHeight: thLegendPt,
			Cells: []*jsonschema.GridCellInput{thLegendCell(ctx, v, kind, l.symbolInk, ovr.RAGColors, len(l.cols))},
		})
	}

	colsJSON, _ := json.Marshal(l.cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  thColGapPt,
		RowGap:  thRowGapPt,
		Rows:    rows,
	}
	return grid, nil
}

// thTones are the table fills: dk2 header (accent for the highlighted
// column), light banding on alternate rows, an accent "Lighter 60%" tint on
// the highlighted row and a "Lighter 80%" tint down the highlighted column.
type thTones struct {
	header, hlHeader, band, hlRow, hlCol fillTone
}

func (l *thLayout) tones() thTones {
	t := thTones{
		header:   fillTone{Color: "dk2"},
		hlHeader: fillTone{Color: l.accent},
		band:     fillTone{Color: "lt2", Alpha: 55},
		hlRow:    fillTone{Color: l.accent, LumMod: 40000, LumOff: 60000},
		hlCol:    fillTone{Color: l.accent, LumMod: tintLumMod, LumOff: tintLumOff},
	}
	if isHexColor(l.accent) {
		// lumMod / lumOff only apply to scheme colours; fall back to neutral.
		t.hlRow, t.hlCol = fillTone{Color: "lt2"}, fillTone{Color: "lt2", Alpha: 70}
	}
	return t
}

func (l *thLayout) headerCells() []*jsonschema.GridCellInput {
	tones := l.tones()
	cells := make([]*jsonschema.GridCellInput, 0, len(l.cols))
	cells = append(cells, thTextCell(tones.header, readableTextOn(l.ctx, tones.header, "lt1"), "l",
		[]chartInsightsParagraph{{Content: l.corner, Size: l.headerSize, Bold: true}}))
	for j, c := range l.v.Criteria {
		tone := tones.header
		if j == l.hlCol {
			tone = tones.hlHeader
		}
		cells = append(cells, thTextCell(tone, readableTextOn(l.ctx, tone, "lt1"), "ctr",
			[]chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(c.Label), Size: l.headerSize, Bold: true}}))
	}
	return cells
}

func (l *thLayout) optionCells(i int, cellOverrides map[int]any) []*jsonschema.GridCellInput {
	tones := l.tones()
	o := l.v.Options[i]
	rowTone := fillTone{Color: "none"}
	if i%2 == 1 {
		rowTone = tones.band
	}
	if i == l.hlRow {
		rowTone = tones.hlRow
	}
	nameInk := readableTextOn(l.ctx, rowTone, "dk1")
	paras := []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(o.Name), Size: l.bodySize, Bold: true, Color: nameInk}}
	if o.Detail != "" {
		paras = append(paras, chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(o.Detail), Size: l.detailSize, Color: nameInk})
	}
	if i == l.hlRow && l.v.HighlightLabel != "" {
		paras = append(paras, chartInsightsParagraph{Content: l.v.HighlightLabel, Size: l.detailSize, Bold: true, Color: thTagInk(l.ctx, l.accent, rowTone)})
	}
	nameCell := thTextCell(rowTone, nameInk, "l", paras)
	co, hasCO := cellOverrides[i].(*TableHighlightCellOverride)
	if i == l.hlRow || (hasCO && co.AccentBar) {
		nameCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: l.accent, Width: 4}
	}
	cells := []*jsonschema.GridCellInput{nameCell}
	for j := range l.v.Criteria {
		tone := rowTone
		if j == l.hlCol && i != l.hlRow {
			tone = tones.hlCol
		}
		sc, _ := parseTableHighlightScore(safeScore(o.Scores, j), l.v.scaleFor(j))
		cells = append(cells, thScoreCell(l.ctx, sc, tone, l.symbolInk, l.ovr.RAGColors, l.bodySize, l.rowPt[i]))
	}
	return cells
}

func safeScore(scores []TableHighlightScore, j int) TableHighlightScore {
	if j < len(scores) {
		return scores[j]
	}
	return ""
}

// thTagInk colours the highlight tag: the accent when it reads on the row
// fill, else the row's readable text colour.
func thTagInk(ctx ExpandContext, accent string, tone fillTone) string {
	fill, ok := effectiveFillColor(ctx, tone)
	ac, aok := resolveThemeColor(ctx, accent)
	if ok && aok && ac.ContrastWith(fill) >= 4.5 {
		return accent
	}
	return readableTextOn(ctx, tone, "dk1")
}

// thTextCell builds a filled (or transparent) text cell.
func thTextCell(tone fillTone, ink, align string, paras []chartInsightsParagraph) *jsonschema.GridCellInput {
	for i := range paras {
		if paras[i].Color == "" {
			paras[i].Color = ink
		}
		paras[i].Align = align
	}
	textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: align, VerticalAlign: "ctr"})
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: thFill(tone), Text: textJSON},
	}
}

func thFill(tone fillTone) json.RawMessage {
	if tone.Color == "none" {
		return json.RawMessage(`"none"`)
	}
	return tone.fillJSON()
}

// thScoreCell renders one score: a Harvey ball / RAG dot icon centred on the
// cell shape, short text, or an en dash for not-applicable.
func thScoreCell(ctx ExpandContext, sc thNormalizedScore, tone fillTone, symbolInk string, ragColors map[string]string, bodySize, rowPt float64) *jsonschema.GridCellInput {
	switch sc.kind {
	case thScaleHarvey, thScaleRAG:
		svg, alt := thSymbolSVG(ctx, sc, symbolInk, ragColors)
		scale := math.Max(0.3, math.Min(0.8, thSymbolPt/math.Max(rowPt, 1)))
		return &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: thFill(tone)},
			Icon:  &jsonschema.IconInput{SVGData: svg, Alt: alt, Position: "center", Scale: math.Round(scale*100) / 100},
		}
	case thScaleText:
		return thTextCell(tone, readableTextOn(ctx, tone, "dk1"), "ctr", []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(sc.text), Size: bodySize}})
	default:
		return thTextCell(tone, readableTextOn(ctx, tone, "dk1"), "ctr", []chartInsightsParagraph{{Content: "–", Size: bodySize}})
	}
}

// thHex resolves a scheme name / hex to "#RRGGBB", falling back when the
// theme is unavailable.
func thHex(ctx ExpandContext, color, fallback string) string {
	if c, ok := resolveThemeColor(ctx, color); ok {
		return c.Hex()
	}
	if isHexColor(color) {
		return "#" + strings.TrimPrefix(color, "#")
	}
	return fallback
}

// thSymbolSVG returns inline SVG markup and alt text for a Harvey ball or RAG
// dot. Harvey balls use the resolved theme ink on an lt1 disc; RAG dots use
// the (overridable) status colours.
func thSymbolSVG(ctx ExpandContext, sc thNormalizedScore, symbolInk string, ragColors map[string]string) (svg, alt string) {
	const head = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" width="100" height="100">`
	if sc.kind == thScaleRAG {
		color := thDefaultRAG[sc.rag]
		if c, ok := ragColors[sc.rag]; ok && c != "" {
			color = c
		}
		hex := thHex(ctx, color, thDefaultRAG[sc.rag])
		return head + fmt.Sprintf(`<circle cx="50" cy="50" r="44" fill="%s"/></svg>`, hex), "RAG status: " + sc.rag
	}
	ink := thHex(ctx, symbolInk, "#1F2A44")
	paper := thHex(ctx, "lt1", "#FFFFFF")
	var b strings.Builder
	b.WriteString(head)
	fmt.Fprintf(&b, `<circle cx="50" cy="50" r="45" fill="%s" stroke="%s" stroke-width="7"/>`, paper, ink)
	switch sc.level {
	case 1:
		fmt.Fprintf(&b, `<path d="M50,50 L50,5 A45,45 0 0,1 95,50 Z" fill="%s"/>`, ink)
	case 2:
		fmt.Fprintf(&b, `<path d="M50,5 A45,45 0 0,1 50,95 Z" fill="%s"/>`, ink)
	case 3:
		fmt.Fprintf(&b, `<path d="M50,50 L50,5 A45,45 0 1,1 5,50 Z" fill="%s"/>`, ink)
	case 4:
		fmt.Fprintf(&b, `<circle cx="50" cy="50" r="45" fill="%s"/>`, ink)
	}
	b.WriteString(`</svg>`)
	return b.String(), fmt.Sprintf("Harvey ball %d of 4", sc.level)
}

// thLegendKinds lists the symbol scales used in the matrix (harvey before rag).
func thLegendKinds(v *TableHighlightValues) []string {
	seen := map[string]bool{}
	for j := range v.Criteria {
		seen[v.scaleFor(j)] = true
	}
	var kinds []string
	for _, k := range []string{thScaleHarvey, thScaleRAG} {
		if seen[k] {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

// thLegendCell builds the legend row: a nested grid of [symbol, label] pairs
// for high / mid / low, spanning the whole table width.
func thLegendCell(ctx ExpandContext, v *TableHighlightValues, kind string, symbolInk string, ragColors map[string]string, span int) *jsonschema.GridCellInput {
	labels := thLegendLabelsFor(v, kind)
	samples := []thNormalizedScore{{kind: thScaleHarvey, level: 4}, {kind: thScaleHarvey, level: 2}, {kind: thScaleHarvey, level: 0}}
	if kind == thScaleRAG {
		samples = []thNormalizedScore{{kind: thScaleRAG, rag: "green"}, {kind: thScaleRAG, rag: "amber"}, {kind: thScaleRAG, rag: "red"}}
	}

	var cells []*jsonschema.GridCellInput
	var cols []float64
	for i, sc := range samples {
		svg, alt := thSymbolSVG(ctx, sc, symbolInk, ragColors)
		cells = append(cells, &jsonschema.GridCellInput{Fit: "contain", Icon: &jsonschema.IconInput{SVGData: svg, Alt: alt}})
		textJSON, _ := json.Marshal(chartInsightsText{
			Paragraphs:    []chartInsightsParagraph{{Content: labels[i], Size: 12, Color: "dk1", Align: "l"}},
			Align:         "l",
			VerticalAlign: "ctr",
		})
		cells = append(cells, &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: textJSON}})
		cols = append(cols, thLegendSwatchPct, thLegendLabelPct)
	}
	used := 0.0
	for _, c := range cols {
		used += c
	}
	if used < 100 {
		cells = append(cells, &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}})
		cols = append(cols, 100-used)
	}
	colsJSON, _ := json.Marshal(cols)
	return &jsonschema.GridCellInput{
		ColSpan: span,
		Grid: &jsonschema.ShapeGridInput{
			Columns: json.RawMessage(colsJSON),
			ColGap:  2,
			// Without an explicit height the nested row collapsed to ~10pt
			// inside its 32pt cell and the labels autofit down to ~2pt
			// (go-slide-creator-z0up).
			Rows: []jsonschema.GridRowInput{{MinHeight: thLegendRowPt, MaxHeight: thLegendRowPt, Cells: cells}},
		},
	}
}

// thLegendLabelsFor returns the three legend words for one scale, in
// [high, mid, low] order. The RAG scale has its own set: reusing the Harvey
// words put "does not meet" beside a green dot (go-slide-creator-z0up).
func thLegendLabelsFor(v *TableHighlightValues, kind string) []string {
	if kind == thScaleRAG {
		if len(v.LegendLabelsRAG) == 3 {
			return v.LegendLabelsRAG
		}
		return []string{"Green", "Amber", "Red"}
	}
	if len(v.LegendLabels) == 3 {
		return v.LegendLabels
	}
	return []string{"Fully meets", "Partially meets", "Does not meet"}
}
