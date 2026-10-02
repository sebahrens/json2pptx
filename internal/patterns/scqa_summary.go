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
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// ---------------------------------------------------------------------------
// scqa-summary pattern — 4-row Situation/Complication/Questions/Answer
// executive summary layout (McKinsey/BCG-style problem framing).
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&scqaSummary{})
}

type scqaSummary struct{}

func (s *scqaSummary) Name() string { return "scqa-summary" }
func (s *scqaSummary) Description() string {
	return "4-row SCQA (Situation / Complication / Questions / Answer) executive summary layout for consulting problem framing"
}
func (s *scqaSummary) UseWhen() string {
	return "Executive summary or problem-framing slide following the SCQA narrative arc (situation → complication → questions → answer); prefer pull-quote for a single takeaway, agenda for a deck section list, comparison-2col when comparing two options side-by-side"
}
func (s *scqaSummary) NotWhen() string {
	return "Content is a deck section list (use agenda), a single attributed takeaway (use pull-quote), a two-option side-by-side comparison (use comparison-2col), or a temporal before/after transformation (use before-after)"
}
func (s *scqaSummary) Version() int      { return 1 }
func (s *scqaSummary) CellsHint() string { return "8 (4 labels + 4 content)" }

func (s *scqaSummary) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"open", "frame", "conclude"},
		PairsWith:     []string{"agenda", "kpi-3up", "stat-hero", "pull-quote"},
		DensityClass:  "medium",
		AccentWeight:  "strong",
	}
}

func (s *scqaSummary) SupportsCallout() bool        { return true }
func (s *scqaSummary) SupportsInlineMarkdown() bool { return true }

func (s *scqaSummary) ExemplarValues() any {
	return &SCQASummaryValues{
		Situation: SCQAText{
			"Cloud spend grew 38% YoY across the portfolio in FY25.",
		},
		Complication: SCQAText{
			"Unit economics flattened as workloads scaled, eroding gross margin.",
			"Finance projects an additional 22% increase if usage trends hold.",
		},
		Questions: []string{
			"Which workloads are driving disproportionate spend?",
			"Where can we reclaim margin without slowing the roadmap?",
			"What governance keeps the savings durable?",
		},
		Answer: []string{
			"Right-size the top 10 workloads to recover 14% of FY26 spend.",
			"Stand up a FinOps council with monthly accountability reviews.",
			"Reinvest 30% of the savings into platform reliability.",
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SCQAText is a polymorphic field that accepts either a string (treated as a
// single-item list) or an array of strings. It normalises both forms to a
// []string for downstream rendering.
type SCQAText []string

// UnmarshalJSON accepts either a JSON string or a JSON array of strings.
func (t *SCQAText) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*t = SCQAText{s}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*t = SCQAText(arr)
		return nil
	}
	return &ValidationError{
		Pattern: "scqa-summary",
		Path:    "",
		Code:    ErrCodeInvalidShape,
		Message: "scqa-summary: situation/complication must be a string or an array of strings",
		Fix:     ReshapeValueFix("", `string or array of strings`, `"single value" or ["item 1", "item 2"]`),
	}
}

// SCQASummaryValues holds the four SCQA regions.
type SCQASummaryValues struct {
	Situation    SCQAText `json:"situation"`
	Complication SCQAText `json:"complication"`
	Questions    []string `json:"questions"`
	Answer       []string `json:"answer"`
}

// SCQASummaryOverrides is the standard text overrides (without cell_accent_mode;
// SCQA is a content-structured layout per docs/PATTERNS.md).
type SCQASummaryOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeaderSize     float64 `json:"header_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
}

// SCQASummaryCellOverride is the shared per-cell override.
type SCQASummaryCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (s *scqaSummary) NewValues() any       { return &SCQASummaryValues{} }
func (s *scqaSummary) NewOverrides() any    { return &SCQASummaryOverrides{} }
func (s *scqaSummary) NewCellOverride() any { return &SCQASummaryCellOverride{} }

// scqaItemBudgets returns the longest single bullet (beside short ones) and the
// average bullet a row holds, by bullet count, measured against the written
// size (no run stored below its role floor) on every shipped template with
// every shape keeping the uniform 0.5 cm text margin (go-slide-creator-n1muf).
// Four-bullet rows hold no readable copy: zero budgets.
func scqaItemBudgets(items int) (single, average int) {
	switch items {
	case 2:
		return 210, 105
	case 3:
		return 105, 105
	default:
		return 0, 0
	}
}

func (s *scqaSummary) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*SCQASummaryValues)
	if !ok || v == nil {
		return nil
	}
	warnings := scqaBudgetWarnings(v)
	if len(warnings) > 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return warnings
	}
	// The budgets assume a typical content area; with the template's own
	// area the rows are measured against it (go-slide-creator-k3eb3).
	ovr, _ := overrides.(*SCQASummaryOverrides)
	if ovr == nil {
		ovr = &SCQASummaryOverrides{}
	}
	specs := []struct {
		label string
		body  []string
	}{
		{"Situation", []string(v.Situation)}, {"Complication", []string(v.Complication)},
		{"Questions", v.Questions}, {"Answer", v.Answer},
	}
	needs, avail := scqaRowNeeds(ctx, specs, fitSCQALabels(ctx, ResolveSize(ovr.HeaderSize, sizeLeadPlusPt)), ResolveSize(ovr.BodySize, scaleBodyPt))
	total := 0.0
	for _, n := range needs {
		total += n
	}
	if total > avail+1 {
		warnings = append(warnings, fmt.Sprintf("%s: scqa-summary rows need %.0fpt at readable sizes but the content area holds about %.0fpt — shorten or merge bullets, or split the summary", ErrCodeBodyTooLong, total, avail))
	}
	return warnings
}

// scqaBudgetWarnings reports rows whose bullets exceed the measured budgets.
func scqaBudgetWarnings(v *SCQASummaryValues) []string {
	rows := []struct {
		name  string
		items []string
	}{
		{"situation", []string(v.Situation)}, {"complication", []string(v.Complication)},
		{"questions", v.Questions}, {"answer", v.Answer},
	}
	var warnings []string
	for _, row := range rows {
		n := len(row.items)
		if n < 2 || n > 4 {
			continue
		}
		single, average := scqaItemBudgets(n)
		if single == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: scqa-summary %s has %d bullets; a row holds at most 3 readable bullets — merge bullets or split the summary", ErrCodeBodyTooLong, row.name, n))
			continue
		}
		total := 0
		for i, item := range row.items {
			length := runeLen(item)
			total += length
			if length > single {
				warnings = append(warnings, fmt.Sprintf("%s: scqa-summary %s[%d] is %d characters; with %d bullets a row holds about %d characters in one bullet beside short ones — shorten the bullet or split the summary", ErrCodeBodyTooLong, row.name, i, length, n, single))
			}
		}
		if total > average*n {
			warnings = append(warnings, fmt.Sprintf("%s: scqa-summary %s uses %d characters across %d bullets; a row holds about %d characters per bullet on average — shorten bullets or split the summary", ErrCodeBodyTooLong, row.name, total, n, average))
		}
	}
	return warnings
}

func (s *scqaSummary) Schema() *Schema {
	stringOrArray := OneOfSchema(
		StringSchema(240).WithDescription("Single-paragraph form"),
		ArraySchema(StringSchema(240), 1, 4).WithDescription("Array form (1-4 bullet points)"),
	).WithDescription("String (single paragraph) or array of strings (1-4 bullets); about 240 characters as one bullet, 105 each with 2-3 (one bullet beside a short one: 210 with 2); rows hold at most 3 readable bullets")

	bulletArray := ArraySchema(StringSchema(240), 1, 4).
		WithDescription("1-4 bullet points; about 240 characters as one bullet, 105 each with 2-3 (one bullet beside a short one: 210 with 2); rows hold at most 3 readable bullets")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"situation":    stringOrArray,
			"complication": stringOrArray,
			"questions":    bulletArray,
			"answer":       bulletArray,
		},
		[]string{"situation", "complication", "questions", "answer"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color for label cells (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":     NumberSchema(6, 120).WithDescription("Font size for row labels in points (default 20)"),
			"body_size":       NumberSchema(6, 120).WithDescription("Font size for body text in points (default 12)"),
		},
		nil,
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      overridesSchema,
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("4-row SCQA (Situation/Complication/Questions/Answer) executive summary")
}

func (s *scqaSummary) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*SCQASummaryValues)
	if !ok || vals == nil {
		return fmt.Errorf("scqa-summary: values must be *SCQASummaryValues, got %T", values)
	}

	const name = "scqa-summary"
	var errs []error

	validateBullets := func(label string, items []string, minItems, maxItems int) {
		if len(items) < minItems {
			errs = append(errs, errMinItems(name, label, minItems, len(items), ""))
		}
		if len(items) > maxItems {
			errs = append(errs, errMaxItems(name, label, maxItems, len(items), ""))
		}
		for i, it := range items {
			itemPath := fmt.Sprintf("%s[%d]", label, i)
			if strings.TrimSpace(it) == "" {
				errs = append(errs, errRequired(name, itemPath))
			} else if runeLen(it) > 240 {
				errs = append(errs, errMaxLength(name, itemPath, 240, runeLen(it)))
			}
		}
	}

	validateBullets("situation", []string(vals.Situation), 1, 4)
	validateBullets("complication", []string(vals.Complication), 1, 4)
	validateBullets("questions", vals.Questions, 1, 4)
	validateBullets("answer", vals.Answer, 1, 4)

	if overrides != nil {
		if _, ok := overrides.(*SCQASummaryOverrides); !ok {
			errs = append(errs, fmt.Errorf("scqa-summary: overrides must be *SCQASummaryOverrides, got %T", overrides))
		}
	}

	// 4 rows × 2 cells = 8 addressable cells
	if coErr := validateCellOverrideKeys(name, cellOverrides, 8,
		"(0=Situation label, 1=Situation content, 2=Complication label, 3=Complication content, 4=Questions label, 5=Questions content, 6=Answer label, 7=Answer content)"); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (s *scqaSummary) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*SCQASummaryValues)
	if !ok {
		return nil, fmt.Errorf("scqa-summary: values must be *SCQASummaryValues, got %T", values)
	}
	ovr := &SCQASummaryOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*SCQASummaryOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("scqa-summary: overrides must be *SCQASummaryOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	labelFit := fitSCQALabels(ctx, ResolveSize(ovr.HeaderSize, sizeLeadPlusPt))
	headerSize := labelFit.size
	bodySize := ResolveSize(ovr.BodySize, scaleBodyPt)

	rowSpecs := []struct {
		label string
		body  []string
	}{
		{"Situation", []string(vals.Situation)},
		{"Complication", []string(vals.Complication)},
		{"Questions", vals.Questions},
		{"Answer", vals.Answer},
	}

	rows := make([]jsonschema.GridRowInput, len(rowSpecs))
	cellIdx := 0
	for i, spec := range rowSpecs {
		labelCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     buildSCQALabelText(spec.label, headerSize),
			},
		}
		applySCQACellOverride(labelCell, cellOverrides, cellIdx, accent)
		cellIdx++

		contentCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Text:     buildSCQAContentText(spec.body, bodySize),
			},
		}
		applySCQACellOverride(contentCell, cellOverrides, cellIdx, accent)
		cellIdx++

		rows[i] = jsonschema.GridRowInput{
			Cells: []*jsonschema.GridCellInput{labelCell, contentCell},
		}
	}

	needs, avail := scqaRowNeeds(ctx, rowSpecs, labelFit, bodySize)
	floorFlexRowsAtNeeds(rows, needs, avail)

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`[%g, 4]`, labelFit.weight)),
		Gap:     ctx.Gap(scqaColGapPt),
		RowGap:  ctx.Gap(scqaRowGapPt),
		Rows:    rows,
	}

	return grid, nil
}

const (
	scqaColGapPt = 8.0
	scqaRowGapPt = 6.0
)

// scqaLabels are the four row labels, each a single word.
var scqaLabels = []string{"Situation", "Complication", "Questions", "Answer"}

// scqaLabelFit is the label column weight (the content column is 4) and the
// label size.
type scqaLabelFit struct {
	weight, size float64
}

// scqaLabelWeights are the label column weights tried, narrowest first.
var scqaLabelWeights = []float64{1, 1.1, 1.2, 1.3}

// scqaColumnWidthsPt returns the label and content column widths.
func scqaColumnWidthsPt(ctx ExpandContext, areaW, weight float64) (label, content float64) {
	unit := (areaW - ctx.Gap(scqaColGapPt)) / (weight + 4)
	return weight * unit, 4 * unit
}

// fitSCQALabels returns the label size and column weight at which every row
// label stays on one line. A label is one word, so a wrap is a mid-word break
// ("Complicatio / n"): each word is measured bold against the atomic-token
// width, which leaves a substituted template face (abstract's Tenorite) the
// margin it draws wider than its stand-in. Geometry gives way before type: at
// each size, from sizePt down 1pt to the 12pt floor, the label column widens
// up to 1.3 : 4 before the size steps down (go-slide-creator-n1muf).
func fitSCQALabels(ctx ExpandContext, sizePt float64) scqaLabelFit {
	areaW, _ := sizingAreaPt(ctx)
	font := ctx.Theme.BodyFont
	fits := func(size, weight float64) bool {
		labelW, _ := scqaColumnWidthsPt(ctx, areaW, weight)
		w := textfit.AtomicTokenWidthPt(font, math.Max(labelW-2*defaultShapeInsetLRPt, 1))
		for _, label := range scqaLabels {
			if measuredLines(label, font, true, size, w) > 1 {
				return false
			}
		}
		return true
	}
	floor := math.Min(sizePt, shapegrid.MinTextSizePt)
	for size := sizePt; size > floor; size-- {
		for _, weight := range scqaLabelWeights {
			if fits(size, weight) {
				return scqaLabelFit{weight: weight, size: size}
			}
		}
	}
	for _, weight := range scqaLabelWeights {
		if fits(floor, weight) {
			return scqaLabelFit{weight: weight, size: floor}
		}
	}
	return scqaLabelFit{weight: 1, size: floor}
}

// scqaRowNeeds returns each row's written-fit height (label and content at
// their real column widths) and the height the four rows share.
func scqaRowNeeds(ctx ExpandContext, specs []struct {
	label string
	body  []string
}, label scqaLabelFit, bodySize float64) ([]float64, float64) {
	areaW, areaH := sizingAreaPt(ctx)
	labelW, contentW := scqaColumnWidthsPt(ctx, areaW, label.weight)
	needs := make([]float64, len(specs))
	for i, s := range specs {
		needs[i] = math.Max(writtenFitHeightPt(buildSCQALabelText(s.label, label.size), labelW, 0),
			writtenFitHeightPt(buildSCQAContentText(s.body, bodySize), contentW, 0))
	}
	return needs, areaH - float64(len(specs)-1)*ctx.Gap(scqaRowGapPt)
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type scqaParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
	Bullet  bool    `json:"bullet,omitempty"`
}

type scqaTextObj struct {
	Paragraphs    []scqaParagraph `json:"paragraphs"`
	Align         string          `json:"align"`
	VerticalAlign string          `json:"vertical_align"`
}

// buildSCQALabelText builds the centered, white-on-accent row label.
func buildSCQALabelText(label string, size float64) json.RawMessage {
	textObj := scqaTextObj{
		Paragraphs: []scqaParagraph{
			{Content: label, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// buildSCQAContentText builds the right-column body text. A single-item list
// renders as one paragraph (no bullet); multi-item lists render as real
// bulleted paragraphs with a hanging indent.
func buildSCQAContentText(items []string, size float64) json.RawMessage {
	paras := make([]scqaParagraph, 0, len(items))
	singleItem := len(items) == 1
	for _, item := range items {
		paras = append(paras, scqaParagraph{
			Bullet:  !singleItem,
			Content: pptx.ConvertMarkdownEmphasis(item),
			Size:    size,
			Color:   "dk1",
			Align:   "l",
		})
	}
	textObj := scqaTextObj{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func applySCQACellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	if cell == nil {
		return
	}
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*SCQASummaryCellOverride)
	if !coOk {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
