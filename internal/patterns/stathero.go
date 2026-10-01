package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// stat-hero pattern — single oversized statistic with label and optional context
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&statHero{})
}

type statHero struct{}

func (sh *statHero) Name() string { return "stat-hero" }
func (sh *statHero) Description() string {
	return "Single oversized statistic with label and optional context"
}
func (sh *statHero) UseWhen() string {
	return "One big number dominates the slide; prefer kpi-3up when showing 3+ metrics side-by-side, pull-quote when the focal content is words not a number"
}
func (sh *statHero) NotWhen() string {
	return "Multiple KPIs need equal weight (use kpi-3up or kpi-4up), or the focal content is a quote (use pull-quote)"
}
func (sh *statHero) Version() int      { return 1 }
func (sh *statHero) CellsHint() string { return "1" }
func (sh *statHero) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "hero",
		NarrativeRole: []string{"open", "evidence"},
		PairsWith:     []string{"kpi-3up", "card-grid", "process-flow"},
		DensityClass:  "low",
		AccentWeight:  "strong",
	}
}

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
	Unit    string `json:"unit,omitempty"`    // Optional unit suffix (e.g. "TAM", "MRR")
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

func (sh *statHero) NewValues() any       { return &StatHeroValues{} }
func (sh *statHero) NewOverrides() any    { return &StatHeroOverrides{} }
func (sh *statHero) NewCellOverride() any { return nil }

// statHeroStackBudget is the combined label + context + source length that
// writes at or above the role floors on every shipped template, measured
// against the written size (go-slide-creator-n1muf): any one field at its
// maximum (context 120) fits, as does every worded field at its maximum
// beside a short number, or every field as one unbroken run at 29%,
// but every field at its maximum does not, with or without word breaks.
const statHeroStackBudget = 120

func (sh *statHero) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*StatHeroValues)
	if !ok || v == nil {
		return nil
	}
	total := runeLen(v.Label) + runeLen(v.Context) + runeLen(v.Source)
	if total <= statHeroStackBudget {
		return nil
	}
	return []string{fmt.Sprintf("%s: stat-hero values.value/unit/label/context/source exceed the measured combined text stack; label, context and source use %d characters but the stack beneath the number holds about %d before text shrinks below readable size — shorten the copy or move context/source elsewhere", ErrCodeBodyTooLong, total, statHeroStackBudget)}
}

func (sh *statHero) Schema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"values": ObjectSchema(
				map[string]*Schema{
					"value":   StringSchema(20).WithDescription("The big number (e.g. \"$2.4B\", \"99.9%\"); shorten long unbroken text when all context fields are populated"),
					"unit":    StringSchema(10).WithDescription("Optional unit suffix (e.g. \"TAM\", \"MRR\")"),
					"label":   StringSchema(80).WithDescription("One-line label beneath the number; label, context and source together hold about 120 readable characters"),
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
	} else if runeLen(v.Value) > 20 {
		errs = append(errs, errMaxLength(name, "values.value", 20, runeLen(v.Value)))
	}

	if v.Unit != "" && runeLen(v.Unit) > 10 {
		errs = append(errs, errMaxLength(name, "values.unit", 10, runeLen(v.Unit)))
	}

	if v.Label == "" {
		errs = append(errs, errRequired(name, "values.label"))
	} else if runeLen(v.Label) > 80 {
		errs = append(errs, errMaxLength(name, "values.label", 80, runeLen(v.Label)))
	}

	if v.Context != "" && runeLen(v.Context) > 120 {
		errs = append(errs, errMaxLength(name, "values.context", 120, runeLen(v.Context)))
	}

	if v.Source != "" && runeLen(v.Source) > 80 {
		errs = append(errs, errMaxLength(name, "values.source", 80, runeLen(v.Source)))
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

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	valueSize := ResolveSize(ovr.ValueSize, 120.0)
	labelSize := ResolveSize(ovr.LabelSize, 18.0)
	contextSize := ResolveSize(ovr.ContextSize, 14.0)

	// The unit is a trailing run in the value's paragraph — same baseline,
	// same accent — at statHeroUnitRatio of the value size. At the value's own
	// size "$2.4B TAM" read as a slogan rather than a figure; consulting
	// big-number slides set the unit at about a third of the number
	// (go-slide-creator-yn2pw).
	valuePara := statHeroParagraph{Content: v.Value, Size: valueSize, Bold: true, Color: accent, Align: "ctr"}
	if v.Unit != "" {
		valuePara.Suffix = " " + v.Unit
		valuePara.SuffixSize = statHeroUnitSize(valueSize)
	}

	// Build paragraphs
	paragraphs := []statHeroParagraph{
		valuePara,
		{Content: v.Label, Size: labelSize, Color: "dk1", Align: "ctr"},
	}

	if v.Context != "" {
		paragraphs = append(paragraphs, statHeroParagraph{
			Content: v.Context, Size: contextSize, Color: "dk1", Align: "ctr",
		})
	}

	if v.Source != "" {
		// One source convention: the same prefix, size and colour the slide
		// band and every other pattern use. The alignment follows the hero's
		// own centred stack rather than breaking it (go-slide-creator-7eib).
		paragraphs = append(paragraphs, statHeroParagraph{
			Content: SourceNoteText(v.Source), Size: SourceNoteSizePt, Color: SourceNoteScheme, Align: "ctr", Italic: true,
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

// statHeroUnitRatio is the unit's size as a share of the value's.
const statHeroUnitRatio = 0.4

// statHeroUnitSize is the unit run's size beside a value of valueSize points,
// never under the 12pt floor.
func statHeroUnitSize(valueSize float64) float64 {
	return math.Max(12, math.Round(valueSize*statHeroUnitRatio))
}

// statHeroParagraph is a text paragraph for JSON marshalling. Suffix is a
// trailing run at SuffixSize in the same paragraph (the unit).
type statHeroParagraph struct {
	Content    string  `json:"content"`
	Size       float64 `json:"size"`
	Bold       bool    `json:"bold,omitempty"`
	Italic     bool    `json:"italic,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	Suffix     string  `json:"suffix,omitempty"`
	SuffixSize float64 `json:"suffix_size,omitempty"`
}

// statHeroText is the text object for JSON marshalling.
type statHeroText struct {
	Paragraphs    []statHeroParagraph `json:"paragraphs"`
	Align         string              `json:"align"`
	VerticalAlign string              `json:"vertical_align"`
}
