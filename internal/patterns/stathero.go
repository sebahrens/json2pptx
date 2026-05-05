package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// stat-hero pattern — single oversized statistic with label and optional context
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&statHero{})
}

type statHero struct{}

func (sh *statHero) Name() string        { return "stat-hero" }
func (sh *statHero) Description() string { return "Single oversized statistic with label and optional context" }
func (sh *statHero) UseWhen() string     { return "One big number dominates the slide" }
func (sh *statHero) Version() int        { return 1 }
func (sh *statHero) CellsHint() string   { return "1" }

func (sh *statHero) SupportsCallout() bool { return false }

func (sh *statHero) ExemplarValues() any {
	v := StatHeroValues{
		Value: "$2.4B",
		Label: "Addressable AI consulting market by FY27",
	}
	return &v
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// StatHeroValues holds the data for a stat-hero pattern.
type StatHeroValues struct {
	Value   string `json:"value"`             // The big number (e.g. "$2.4B", "99.9%")
	Unit    string `json:"unit,omitempty"`     // Optional unit suffix (e.g. "TAM", "MRR")
	Label   string `json:"label"`             // One-line context (e.g. "addressable market")
	Context string `json:"context,omitempty"` // Optional subtext line
	Source  string `json:"source,omitempty"`  // Optional source/footnote
}

// StatHeroOverrides contains pattern-level overrides for stat-hero.
type StatHeroOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	ValueSize      float64 `json:"value_size,omitempty"`
	LabelSize      float64 `json:"label_size,omitempty"`
	ContextSize    float64 `json:"context_size,omitempty"`
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (sh *statHero) NewValues() any      { return &StatHeroValues{} }
func (sh *statHero) NewOverrides() any   { return &StatHeroOverrides{} }
func (sh *statHero) NewCellOverride() any { return nil }

func (sh *statHero) Schema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"values": ObjectSchema(
				map[string]*Schema{
					"value":   StringSchema(20).WithDescription("The big number (e.g. \"$2.4B\", \"99.9%\")"),
					"unit":    StringSchema(10).WithDescription("Optional unit suffix (e.g. \"TAM\", \"MRR\")"),
					"label":   StringSchema(80).WithDescription("One-line label beneath the number"),
					"context": StringSchema(120).WithDescription("Optional subtext line"),
					"source":  StringSchema(80).WithDescription("Optional source/footnote text"),
				},
				[]string{"value", "label"},
			).WithAdditionalProperties(false),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"value_size":      NumberSchema(40, 200).WithDescription("Font size for the big number in points (default 120)"),
					"label_size":      NumberSchema(6, 60).WithDescription("Font size for the label in points (default 18)"),
					"context_size":    NumberSchema(6, 40).WithDescription("Font size for the context line in points (default 14)"),
				},
				nil,
			).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Single oversized statistic with label and optional context")
}

func (sh *statHero) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*StatHeroValues)
	if !ok || v == nil {
		return fmt.Errorf("stat-hero: values must be *StatHeroValues, got %T", values)
	}

	const name = "stat-hero"
	var errs []error

	if v.Value == "" {
		errs = append(errs, errRequired(name, "values.value"))
	} else if len(v.Value) > 20 {
		errs = append(errs, errMaxLength(name, "values.value", 20, len(v.Value)))
	}

	if v.Unit != "" && len(v.Unit) > 10 {
		errs = append(errs, errMaxLength(name, "values.unit", 10, len(v.Unit)))
	}

	if v.Label == "" {
		errs = append(errs, errRequired(name, "values.label"))
	} else if len(v.Label) > 80 {
		errs = append(errs, errMaxLength(name, "values.label", 80, len(v.Label)))
	}

	if v.Context != "" && len(v.Context) > 120 {
		errs = append(errs, errMaxLength(name, "values.context", 120, len(v.Context)))
	}

	if v.Source != "" && len(v.Source) > 80 {
		errs = append(errs, errMaxLength(name, "values.source", 80, len(v.Source)))
	}

	return errors.Join(errs...)
}

func (sh *statHero) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*StatHeroValues)
	if !ok {
		return nil, fmt.Errorf("stat-hero: values must be *StatHeroValues, got %T", values)
	}
	ovr := &StatHeroOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*StatHeroOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("stat-hero: overrides must be *StatHeroOverrides, got %T", overrides)
		}
	}

	accent := ResolveAccent(ovr.Accent, ovr.SemanticAccent, ctx.Metadata)
	valueSize := ResolveSize(ovr.ValueSize, 120.0)
	labelSize := ResolveSize(ovr.LabelSize, 18.0)
	contextSize := ResolveSize(ovr.ContextSize, 14.0)

	// Build the main value display string
	displayValue := v.Value
	if v.Unit != "" {
		displayValue += " " + v.Unit
	}

	// Build paragraphs
	paragraphs := []statHeroParagraph{
		{Content: displayValue, Size: valueSize, Bold: true, Color: accent, Align: "ctr"},
		{Content: v.Label, Size: labelSize, Color: "dk1", Align: "ctr"},
	}

	if v.Context != "" {
		paragraphs = append(paragraphs, statHeroParagraph{
			Content: v.Context, Size: contextSize, Color: "dk1", Align: "ctr",
		})
	}

	if v.Source != "" {
		paragraphs = append(paragraphs, statHeroParagraph{
			Content: v.Source, Size: 10, Color: "dk1", Align: "ctr", Italic: true,
		})
	}

	textObj := statHeroText{
		Paragraphs:    paragraphs,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	textJSON, _ := json.Marshal(textObj)

	cell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Text:     textJSON,
		},
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []jsonschema.GridRowInput{
			{Cells: []*jsonschema.GridCellInput{cell}},
		},
	}

	return grid, nil
}

// statHeroParagraph is a text paragraph for JSON marshalling.
type statHeroParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Italic  bool    `json:"italic,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

// statHeroText is the text object for JSON marshalling.
type statHeroText struct {
	Paragraphs    []statHeroParagraph `json:"paragraphs"`
	Align         string              `json:"align"`
	VerticalAlign string              `json:"vertical_align"`
}
