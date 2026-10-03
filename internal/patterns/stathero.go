package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/textfit"
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

func (sh *statHero) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*StatHeroValues)
	if !ok || v == nil {
		return nil
	}
	total := runeLen(v.Label) + runeLen(v.Context) + runeLen(v.Source)
	if total <= statHeroStackBudget {
		// The budget assumes a whole slide; in a segment or cell the stack
		// is measured against its own area (go-slide-creator-hidji).
		if ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
			return nil
		}
		ovr, _ := overrides.(*StatHeroOverrides)
		if ovr == nil {
			ovr = &StatHeroOverrides{}
		}
		sz := statHeroFit(ctx, v, ovr, ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent))
		if sz.fits {
			return nil
		}
		return []string{fmt.Sprintf("%s: stat-hero figure, label and context do not fit their %.0f×%.0fpt area even at the %.0fpt figure / %.0fpt label minimum — give the region or segment more room, shorten the label or context, or use kpi-inline", ErrCodeBodyTooLong, sz.widthPt, sz.heightPt, sz.value, sz.label)}
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
					"value_size":      NumberSchema(40, 200).WithDescription("Font size for the big number in points (default: 120 on a whole slide; in a compose segment, region or grid cell the largest size, down to 28, at which the stack fits unshrunk)"),
					"label_size":      NumberSchema(6, 60).WithDescription("Font size for the label in points (default 18; 14 when the cell is too short for an 18pt label beside a 40pt figure)"),
					"context_size":    NumberSchema(6, 40).WithDescription("Font size for the context line in points (default 14; 12 in a short cell)"),
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
	sz := statHeroFit(ctx, v, ovr, accent)
	textJSON := statHeroTextJSON(v, accent, sz)

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
	InsetTop      *float64            `json:"inset_top,omitempty"`
	InsetBottom   *float64            `json:"inset_bottom,omitempty"`
}

// ---------------------------------------------------------------------------
// Sizing to the cell (go-slide-creator-hidji)
// ---------------------------------------------------------------------------

// The 120pt figure is drawn for a whole slide. In a compose segment, a
// regions cell or a grid cell it was written shrunk, and LibreOffice drew the
// shrunk number over its own label. The pattern sizes the figure to the
// rectangle it is expanded into instead: the largest whole-point size at
// which the stack writes without a shrink and no word of the figure wraps.
// The figure steps down to the KPI step with the label and context at their
// defaults; then the label and context drop to their floors and the figure
// continues down to statHeroMinValuePt — twice the label floor, so the number
// still dominates. Last, the unfilled stack gives up its top and bottom text
// margin, which a short cell spends on nothing visible. Authored sizes are
// kept.
const (
	statHeroKPIValuePt   = scaleKPIPt     // 40pt: the figure reaches the KPI step before the label gives way
	statHeroMinValuePt   = scaleDisplayPt // 28pt: the smallest figure that still dominates a 14pt label
	statHeroMinLabelPt   = scaleSubheadPt // 14pt label floor
	statHeroMinContextPt = scaleBodyPt    // 12pt context floor
)

// statHeroSizes is one stat-hero's resolved type sizes, whether its stack
// writes without a shrink at them, and the area it was measured in.
type statHeroSizes struct {
	value, label, context float64
	// tight drops the top and bottom text inset.
	tight             bool
	fits              bool
	widthPt, heightPt float64
}

// statHeroFit sizes the stack to the pattern's area. On the whole content
// area the default sizes fit, so a full-slide stat-hero is unchanged.
func statHeroFit(ctx ExpandContext, v *StatHeroValues, ovr *StatHeroOverrides, accent string) statHeroSizes {
	w, h := contentAreaPt(ctx)
	fonts := ctx.themeFonts()
	fits := func(s statHeroSizes) bool {
		return statHeroValueUnbroken(ctx.Theme.BodyFont, v.Value, s.value, w-2*defaultShapeInsetLRPt) &&
			writtenFitsAt(fonts, statHeroTextJSON(v, accent, s), w, h)
	}
	type stage struct {
		label, context, lo, hi float64
		tight                  bool
	}
	label := ResolveSize(ovr.LabelSize, scaleLeadPt)
	context := ResolveSize(ovr.ContextSize, scaleSubheadPt)
	top := ResolveSize(ovr.ValueSize, sizeHeroFigurePt)
	stages := []stage{{label: label, context: context, lo: math.Min(top, statHeroKPIValuePt), hi: top}}
	if ovr.ValueSize > 0 {
		stages[0].lo = top
	}
	if ovr.LabelSize == 0 || ovr.ContextSize == 0 {
		st := stage{label: label, context: context, lo: math.Min(top, statHeroMinValuePt), hi: math.Min(top, statHeroKPIValuePt)}
		if ovr.ValueSize > 0 {
			st.lo, st.hi = top, top
		}
		if ovr.LabelSize == 0 {
			st.label = statHeroMinLabelPt
		}
		if ovr.ContextSize == 0 {
			st.context = statHeroMinContextPt
		}
		stages = append(stages, st)
	}
	tight := stages[len(stages)-1]
	tight.tight = true
	stages = append(stages, tight)
	var last statHeroSizes
	for _, st := range stages {
		at := func(value float64) statHeroSizes {
			return statHeroSizes{value: value, label: st.label, context: st.context, tight: st.tight, widthPt: w, heightPt: h}
		}
		if s := at(st.hi); fits(s) {
			s.fits = true
			return s
		}
		last = at(st.lo)
		if !fits(last) {
			continue
		}
		// The largest whole-point figure that fits; the fit is monotonic in
		// the figure size.
		lo, hi := st.lo, st.hi
		for hi-lo > 1 {
			mid := math.Floor((lo + hi) / 2)
			if fits(at(mid)) {
				lo = mid
			} else {
				hi = mid
			}
		}
		s := at(lo)
		s.fits = true
		return s
	}
	return last
}

// statHeroValueUnbroken reports whether every word of the figure sits on one
// line at sizePt in a box widthPt wide: "$2.4B" shrinks rather than wrapping
// to "$2.4 / B".
func statHeroValueUnbroken(font, value string, sizePt, widthPt float64) bool {
	widthPt = textfit.AtomicTokenWidthPt(font, widthPt)
	for _, word := range strings.Fields(value) {
		if measuredLines(word, font, true, sizePt, widthPt) > 1 {
			return false
		}
	}
	return true
}

// statHeroTextJSON builds the stack's text at the given sizes: the figure
// with its unit as a trailing run, the label, then the optional context and
// source lines.
func statHeroTextJSON(v *StatHeroValues, accent string, sz statHeroSizes) json.RawMessage {
	// The unit is a trailing run in the value's paragraph — same baseline,
	// same accent — at statHeroUnitRatio of the value size. At the value's own
	// size "$2.4B TAM" read as a slogan rather than a figure; consulting
	// big-number slides set the unit at about a third of the number
	// (go-slide-creator-yn2pw).
	valuePara := statHeroParagraph{Content: v.Value, Size: sz.value, Bold: true, Color: accent, Align: "ctr"}
	if v.Unit != "" {
		valuePara.Suffix = " " + v.Unit
		valuePara.SuffixSize = statHeroUnitSize(sz.value)
	}
	paragraphs := []statHeroParagraph{
		valuePara,
		{Content: v.Label, Size: sz.label, Color: "dk1", Align: "ctr"},
	}
	if v.Context != "" {
		paragraphs = append(paragraphs, statHeroParagraph{
			Content: v.Context, Size: sz.context, Color: "dk1", Align: "ctr",
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
	obj := statHeroText{Paragraphs: paragraphs, Align: "ctr", VerticalAlign: "ctr"}
	if sz.tight {
		zero := 0.0
		obj.InsetTop, obj.InsetBottom = &zero, &zero
	}
	text, _ := json.Marshal(obj)
	return text
}
