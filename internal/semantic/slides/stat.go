package slides

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Stat slides (go-slide-creator-2hkc).
//
// One number, made the whole slide — the "the market is $2.4B" moment every
// pitch and board deck has. The DeckSpec had no kind for it, so an agent
// reaching for it dropped to raw_json2pptx or wrote the number into a title.
// The payload here is what an author writes and the compiler maps it onto the
// stat-hero pattern.

const (
	// statValueMax, statUnitMax, statLabelMax, statContextMax and statSourceMax
	// mirror the stat-hero pattern's own string budgets: past them the pattern
	// refuses the slide, so the compiler degrades instead of handing it a
	// payload it will reject.
	statValueMax   = 20
	statUnitMax    = 10
	statLabelMax   = 80
	statContextMax = 120
	statSourceMax  = 80
)

// statHeroValues is the stat-hero pattern's values object.
type statHeroValues struct {
	Value   string `json:"value"`
	Unit    string `json:"unit,omitempty"`
	Label   string `json:"label"`
	Context string `json:"context,omitempty"`
	Source  string `json:"source,omitempty"`
}

// CompileStat compiles a stat payload onto the stat-hero pattern, falling back
// to a content slide when the payload does not fit the pattern's budgets.
func CompileStat(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	values := statValues(in.Body)
	if StatOverBudget(in.Body) != "" {
		return compileStatFallback(in, values)
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal stat-hero values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "stat-hero", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.value",
		SemanticPath: in.semSlide() + "." + statValueField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileStatFallback keeps the number and everything around it on a content
// slide, so a value the display size cannot hold is not squeezed into it.
func compileStatFallback(in Input, v statHeroValues) (*deckinput.SlideInput, []SourceLink, error) {
	headline := v.Value
	if v.Unit != "" {
		headline += " " + v.Unit
	}
	if v.Label != "" {
		if headline != "" {
			headline += " — "
		}
		headline += v.Label
	}
	if headline == "" {
		return CompileFallback(in)
	}

	bullets := []string{headline}
	if v.Context != "" {
		bullets = append(bullets, v.Context)
	}
	// The source is deliberately not a bullet: the compiler promotes a payload's
	// source to the slide's own attribution band, and a bullet as well printed it
	// twice (the double-print go-slide-creator-xg48 fixed for chart_insight).

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{
		RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + "." + statValueField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// statValues resolves a stat payload into the pattern's fields. The label falls
// back to the slide title, because an author who wrote only a title meant it as
// the words beneath the number: a bare number with no words is not a slide.
func statValues(body map[string]any) statHeroValues {
	v := statHeroValues{
		Value:   firstNonEmpty(strField(body, "value"), strField(body, "stat"), strField(body, "number"), strField(body, "metric")),
		Unit:    firstNonEmpty(strField(body, "unit"), strField(body, "suffix")),
		Label:   firstNonEmpty(strField(body, "label"), strField(body, "caption"), strField(body, "subtitle")),
		Context: firstNonEmpty(strField(body, "context"), strField(body, "detail"), strField(body, "description")),
		Source:  strField(body, "source"),
	}
	if v.Label == "" {
		v.Label = strField(body, "title")
	}
	return v
}

// statValueField names the payload field the number came from, so a finding
// addresses the key the author actually wrote.
func statValueField(body map[string]any) string {
	for _, key := range []string{"value", "stat", "number", "metric"} {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "value"
}

// StatOverBudget explains why a stat payload cannot take the hero treatment, or
// "" when it fits. An empty value is the required-field gate's business, not the
// budget's, so it reports nothing.
func StatOverBudget(body map[string]any) string {
	issues := StatBudgetIssues(body)
	if len(issues) > 0 {
		return issues[0].Message
	}
	return ""
}

// StatBudgetIssue locates one reason the stat-hero pattern would reject the
// payload. Field is the authored alias selected by statValues.
type StatBudgetIssue struct {
	Field   string
	Message string
}

// StatBudgetIssues returns every independent stat-hero budget violation so an
// author can repair the slide in one validation round trip.
func StatBudgetIssues(body map[string]any) []StatBudgetIssue {
	v := statValues(body)
	if v.Value == "" {
		return nil
	}
	var issues []StatBudgetIssue
	if v.Label == "" {
		issues = append(issues, StatBudgetIssue{"label", "has no label; a number with no words beneath it is not a slide"})
	}
	for _, field := range []struct {
		value, name string
		max         int
		message     string
		aliases     []string
	}{
		{v.Value, "value", statValueMax, "the display number", []string{"value", "stat", "number", "metric"}},
		{v.Unit, "unit", statUnitMax, "the suffix", []string{"unit", "suffix"}},
		{v.Label, "label", statLabelMax, "the line beneath the number", []string{"label", "caption", "subtitle", "title"}},
		{v.Context, "context", statContextMax, "the subtext line", []string{"context", "detail", "description"}},
		{v.Source, "source", statSourceMax, "the footnote", []string{"source"}},
	} {
		if n := runeLen(field.value); n > field.max {
			issues = append(issues, StatBudgetIssue{
				Field:   firstStatPopulatedField(body, field.aliases...),
				Message: fmt.Sprintf("%s is %d characters; %s holds %d", field.name, n, field.message, field.max),
			})
		}
	}
	return issues
}

func firstStatPopulatedField(body map[string]any, fields ...string) string {
	for _, field := range fields {
		if strField(body, field) != "" {
			return field
		}
	}
	return fields[0]
}

// StatPattern returns the pattern a stat payload compiles to, or "" when it
// degrades to a content slide. The explain planner and validation both read it,
// so neither promises a visual compile will not emit.
func StatPattern(body map[string]any) string {
	if statValues(body).Value != "" && StatOverBudget(body) == "" {
		return "stat-hero"
	}
	return ""
}

// UsableStatValue reports whether the payload carries a number to show, so
// validation fails fast on a stat slide with nothing on it.
func UsableStatValue(body map[string]any) int {
	if statValues(body).Value == "" {
		return 0
	}
	return 1
}
