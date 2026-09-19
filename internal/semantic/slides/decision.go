package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Decision slides (go-slide-creator-4ndv).
//
// The ask is the slide a board deck exists for, and it compiled to the plainest
// page in the deck: a bold paragraph and a column of dashes. The options now
// take a numbered visual and the recommendation takes the callout band beneath
// it, which is where the ask belongs.

const (
	// decisionMinSteps / decisionMaxSteps mirror numbered-step-strip's bounds.
	decisionMinSteps = 3
	decisionMaxSteps = 6
	// decisionLabelMax / decisionDetailMax mirror its text budgets.
	decisionLabelMax  = 60
	decisionDetailMax = 180
	// decisionPairCardBodyMax is card-grid's body budget, used for the
	// two-option treatment.
	decisionPairCardBodyMax = 300
	// decisionPairHeaderMax is card-grid's header budget.
	decisionPairHeaderMax = 80
)

// decisionOption is one resolved option: what it is, and what it means.
type decisionOption struct {
	Label  string
	Detail string
}

// decisionStep is one numbered-step-strip step.
type decisionStep struct {
	Label string `json:"label"`
	Body  string `json:"body,omitempty"`
}

// decisionStripValues is the numbered-step-strip pattern's values object.
type decisionStripValues struct {
	Style string         `json:"style,omitempty"`
	Steps []decisionStep `json:"steps"`
}

// decisionCard is one card-grid cell.
type decisionCard struct {
	Header string `json:"header"`
	Body   string `json:"body"`
}

// decisionCardValues is the card-grid pattern's values object.
type decisionCardValues struct {
	Columns int            `json:"columns"`
	Rows    int            `json:"rows"`
	Cells   []decisionCard `json:"cells"`
}

// decisionCardOverrides carries the numbered badge style.
type decisionCardOverrides struct {
	Style string `json:"style,omitempty"`
}

// CompileDecision compiles a decision slide. Three or more options become a
// numbered step strip; exactly two, each with a detail, become a pair of cards;
// anything else keeps the content slide this kind has always produced.
// Whichever visual it reaches, the recommendation takes the callout band
// beneath it.
func CompileDecision(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	options := DecisionOptions(in.Body)
	switch DecisionPattern(in.Body) {
	case "numbered-step-strip":
		return compileDecisionStrip(in, options)
	case "card-grid":
		return compileDecisionCards(in, options)
	default:
		return compileDecisionContent(in)
	}
}

// compileDecisionStrip emits the numbered options with the ask beneath them.
func compileDecisionStrip(in Input, options []decisionOption) (*deckinput.SlideInput, []SourceLink, error) {
	steps := make([]decisionStep, 0, len(options))
	for _, o := range options {
		steps = append(steps, decisionStep{Label: o.Label, Body: o.Detail})
	}
	encoded, err := json.Marshal(decisionStripValues{Style: "stacked-box", Steps: steps})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal numbered-step-strip values: %w", err)
	}
	return decisionPatternSlide(in, "numbered-step-strip", encoded, nil)
}

// compileDecisionCards emits two options side by side. The soft-card surface
// beats the numbered badge here: with two cards the badge style renders bare
// numbers over unfilled text and the slide reads as half-empty, while the
// surface makes them two panels to weigh against each other.
func compileDecisionCards(in Input, options []decisionOption) (*deckinput.SlideInput, []SourceLink, error) {
	cells := make([]decisionCard, 0, len(options))
	for _, o := range options {
		cells = append(cells, decisionCard{Header: o.Label, Body: o.Detail})
	}
	encoded, err := json.Marshal(decisionCardValues{Columns: len(cells), Rows: 1, Cells: cells})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal card-grid values: %w", err)
	}
	overrides, err := json.Marshal(decisionCardOverrides{Style: "soft-card"})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal card-grid overrides: %w", err)
	}
	return decisionPatternSlide(in, "card-grid", encoded, overrides)
}

// decisionPatternSlide assembles a pattern slide with the recommendation in the
// callout band. The band is the ask's place: it is the one line the room has to
// leave with, and both patterns draw it below their own content.
func decisionPatternSlide(in Input, pattern string, values, overrides json.RawMessage) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: pattern, Values: values, Overrides: overrides}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values",
		SemanticPath: in.semSlide() + ".options",
	})
	if rec := strField(in.Body, "recommendation"); rec != "" {
		slide.Pattern.Callout = &patterns.PatternCallout{Text: rec, Emphasis: "bold"}
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.callout.text",
			SemanticPath: in.semSlide() + ".recommendation",
		})
	}
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileDecisionContent is the content slide this kind has always produced: a
// title, the recommendation as a lead-in body paragraph, and the options as
// supporting bullets. It is what a payload outside the visuals' bounds gets,
// and it always renders.
func compileDecisionContent(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink

	if in.Title != "" {
		idx := appendContent(slide, textContent("title", in.Title))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".title",
		})
	}

	recommendation := strField(in.Body, "recommendation")
	options := decisionOptionLines(DecisionOptions(in.Body))

	switch {
	case recommendation != "" && len(options) > 0:
		idx := appendContent(slide, bodyAndBulletsContent("body", recommendation, options))
		links = append(links,
			SourceLink{
				RawPath:      fmt.Sprintf("%s.content[%d].body_and_bullets_value.body", in.rawSlide(), idx),
				SemanticPath: in.semSlide() + ".recommendation",
			},
			SourceLink{
				RawPath:      fmt.Sprintf("%s.content[%d].body_and_bullets_value.bullets", in.rawSlide(), idx),
				SemanticPath: in.semSlide() + ".options",
			},
		)
	case len(options) > 0:
		idx := appendContent(slide, bulletsContent("body", options))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".options",
		})
	case recommendation != "":
		idx := appendContent(slide, textContent("body", recommendation))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".recommendation",
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// decisionOptionLines renders the options as bullet text, keeping each detail
// with the option it belongs to.
func decisionOptionLines(options []decisionOption) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		line := o.Label
		if o.Detail != "" {
			line += " — " + o.Detail
		}
		out = append(out, line)
	}
	return out
}

// DecisionOptions resolves a decision payload's options. An option is a string
// ("Label", "Label | detail" or "Label — detail") or an object carrying a label
// and optionally what it means. Entries with no label are dropped.
func DecisionOptions(body map[string]any) []decisionOption {
	raw, ok := firstList(body, "options", "choices", "alternatives")
	if !ok {
		return nil
	}
	var out []decisionOption
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if o, valid := decisionOptionFromString(t); valid {
				out = append(out, o)
			}
		case map[string]any:
			label := firstNonEmpty(strField(t, "label"), strField(t, "title"), strField(t, "name"), strField(t, "option"))
			if label == "" {
				continue
			}
			out = append(out, decisionOption{
				Label:  label,
				Detail: firstNonEmpty(strField(t, "detail"), strField(t, "description"), strField(t, "body"), strField(t, "summary")),
			})
		}
	}
	return out
}

// decisionOptionFromString splits an option written as one line.
func decisionOptionFromString(s string) (decisionOption, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decisionOption{}, false
	}
	for _, sep := range []string{" | ", " — ", " - "} {
		if label, detail, found := strings.Cut(s, sep); found {
			return decisionOption{Label: strings.TrimSpace(label), Detail: strings.TrimSpace(detail)}, true
		}
	}
	return decisionOption{Label: s}, true
}

// decisionStripFits reports whether the options fit the numbered step strip.
func decisionStripFits(options []decisionOption) bool {
	if len(options) < decisionMinSteps || len(options) > decisionMaxSteps {
		return false
	}
	for _, o := range options {
		if runeLen(o.Label) > decisionLabelMax || runeLen(o.Detail) > decisionDetailMax {
			return false
		}
	}
	return true
}

// decisionCardsFit reports whether the options fit the two-card treatment. Both
// cards need a detail: card-grid requires a body, and a card with a heading and
// a blank space beneath it reads as missing data.
func decisionCardsFit(options []decisionOption) bool {
	if len(options) != 2 {
		return false
	}
	for _, o := range options {
		if o.Detail == "" || runeLen(o.Label) > decisionPairHeaderMax || runeLen(o.Detail) > decisionPairCardBodyMax {
			return false
		}
	}
	return true
}

// DecisionPattern returns the pattern a decision payload compiles to, or ""
// when it keeps the content slide.
func DecisionPattern(body map[string]any) string {
	options := DecisionOptions(body)
	switch {
	case decisionStripFits(options):
		return "numbered-step-strip"
	case decisionCardsFit(options):
		return "card-grid"
	default:
		return ""
	}
}

// DecisionOverBudget explains why the options cannot take a visual, or "" when
// they can (or when there are none to place).
func DecisionOverBudget(body map[string]any) string {
	options := DecisionOptions(body)
	if len(options) == 0 || DecisionPattern(body) != "" {
		return ""
	}
	switch {
	case len(options) == 1:
		return "has one option; a decision slide needs at least two to be a choice"
	case len(options) == 2:
		return "has two options and one of them says only what it is called; give both a detail for the two-card treatment"
	case len(options) > decisionMaxSteps:
		return fmt.Sprintf("has %d options; the numbered strip holds %d", len(options), decisionMaxSteps)
	}
	for i, o := range options {
		switch {
		case runeLen(o.Label) > decisionLabelMax:
			return fmt.Sprintf("option %d's label is %d characters; a step holds %d", i+1, runeLen(o.Label), decisionLabelMax)
		case runeLen(o.Detail) > decisionDetailMax:
			return fmt.Sprintf("option %d's detail is %d characters; a step holds %d", i+1, runeLen(o.Detail), decisionDetailMax)
		}
	}
	return ""
}

// UsableDecisionOptionCount returns how many options survive extraction.
func UsableDecisionOptionCount(body map[string]any) int { return len(DecisionOptions(body)) }
