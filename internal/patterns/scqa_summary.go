package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
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

func (s *scqaSummary) Schema() *Schema {
	stringOrArray := OneOfSchema(
		StringSchema(240).WithDescription("Single-paragraph form"),
		ArraySchema(StringSchema(240), 1, 4).WithDescription("Array form (1-4 bullet points)"),
	).WithDescription("String (single paragraph) or array of strings (1-4 bullets)")

	bulletArray := ArraySchema(StringSchema(240), 1, 4).
		WithDescription("1-4 bullet points")

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
			} else if len(it) > 240 {
				errs = append(errs, errMaxLength(name, itemPath, 240, len(it)))
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
	headerSize := ResolveSize(ovr.HeaderSize, 20.0)
	bodySize := ResolveSize(ovr.BodySize, 12.0)

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

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[1, 4]`),
		Gap:     8,
		RowGap:  6,
		Rows:    rows,
	}

	return grid, nil
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
// renders as one paragraph (no bullet prefix); multi-item lists render as
// bulleted paragraphs prefixed with "• ".
func buildSCQAContentText(items []string, size float64) json.RawMessage {
	paras := make([]scqaParagraph, 0, len(items))
	singleItem := len(items) == 1
	for _, item := range items {
		content := pptx.ConvertMarkdownEmphasis(item)
		if !singleItem {
			content = "• " + content
		}
		paras = append(paras, scqaParagraph{
			Content: content,
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
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
