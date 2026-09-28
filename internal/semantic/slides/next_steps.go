package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Next-steps slides (go-slide-creator-7lzdh).
//
// A consulting deck closes on what happens next and what the room is asked to
// decide, not on "Thank you". The payload is what an author writes — actions
// with an owner and a date, and the decisions requested — and the compiler maps
// it onto the next-steps pattern. Outside the pattern's bounds the actions and
// decisions become bullets, so nothing is lost.

// nextStepsAction is one resolved action row, in the pattern's own shape.
type nextStepsAction struct {
	Action string `json:"action"`
	Owner  string `json:"owner,omitempty"`
	Date   string `json:"date,omitempty"`
}

// nextStepsValues is the next-steps pattern's values object.
type nextStepsValues struct {
	Actions        []nextStepsAction `json:"actions"`
	Decisions      []string          `json:"decisions,omitempty"`
	DecisionsLabel string            `json:"decisions_label,omitempty"`
}

// nextStepsActionFields / nextStepsDecisionFields are the payload fields the
// actions and decisions are read from, canonical first.
var (
	nextStepsActionFields   = []string{"actions", "next_steps", "steps"}
	nextStepsDecisionFields = []string{"decisions", "decisions_requested", "asks"}
)

// CompileNextSteps compiles a next-steps slide onto the next-steps pattern,
// falling back to bullets when it does not fit.
func CompileNextSteps(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	values := nextStepsPayload(in.Body)
	if NextStepsPattern(in.Body) == "" {
		return compileNextStepsFallback(in, values)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal next-steps values: %w", err)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "next-steps", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.actions",
		SemanticPath: in.semSlide() + "." + NextStepsActionsField(in.Body),
	})
	if len(values.Decisions) > 0 {
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.decisions",
			SemanticPath: in.semSlide() + "." + nextStepsDecisionsField(in.Body),
		})
	}
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileNextStepsFallback lists the actions (with owner and date) and the
// decisions as bullets on a content slide.
func compileNextStepsFallback(in Input, v nextStepsValues) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := make([]string, 0, len(v.Actions)+len(v.Decisions))
	for _, a := range v.Actions {
		line := a.Action
		meta := strings.Join(nonBlank(a.Owner, a.Date), ", ")
		if meta != "" {
			line += " — " + meta
		}
		bullets = append(bullets, line)
	}
	label := firstNonEmpty(v.DecisionsLabel, "Decision requested")
	for _, d := range v.Decisions {
		bullets = append(bullets, label+": "+d)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	return contentFallback(in, NextStepsActionsField(in.Body), bullets)
}

func nonBlank(vals ...string) []string {
	var out []string
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// nextStepsPayload resolves the payload into the pattern's values. An action
// is a string or an object {action|title|label|step|text, owner|who,
// date|due|when}; entries with no action text are dropped.
func nextStepsPayload(body map[string]any) nextStepsValues {
	var v nextStepsValues
	if raw, ok := firstList(body, nextStepsActionFields...); ok {
		for _, e := range raw {
			switch t := e.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					v.Actions = append(v.Actions, nextStepsAction{Action: s})
				}
			case map[string]any:
				action := strings.TrimSpace(firstNonEmpty(strField(t, "action"), strField(t, "title"), strField(t, "label"), strField(t, "step"), strField(t, "text")))
				if action == "" {
					continue
				}
				v.Actions = append(v.Actions, nextStepsAction{
					Action: action,
					Owner:  strings.TrimSpace(firstNonEmpty(strField(t, "owner"), strField(t, "who"))),
					Date:   strings.TrimSpace(firstNonEmpty(strField(t, "date"), strField(t, "due"), strField(t, "when"))),
				})
			}
		}
	}
	for _, key := range nextStepsDecisionFields {
		if list, ok := stringList(body, key); ok {
			v.Decisions = list
			break
		}
	}
	v.DecisionsLabel = strField(body, "decisions_label")
	return v
}

// NextStepsPattern returns "next-steps" when the payload compiles to the
// pattern and "" when it degrades to bullets; explain and validation read it
// so neither promises a visual compile will not emit.
func NextStepsPattern(body map[string]any) string {
	if NextStepsDegradeReason(body) != "" {
		return ""
	}
	return "next-steps"
}

// NextStepsDegradeReason explains why a payload will not render as the
// next-steps pattern, or "" when it will.
func NextStepsDegradeReason(body map[string]any) string {
	v := nextStepsPayload(body)
	if len(v.Actions) == 0 {
		return "no usable actions"
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return "the actions cannot be encoded as pattern values"
	}
	if err := deckinput.ValidatePattern(&deckinput.PatternInput{Name: "next-steps", Values: encoded}, patterns.Default()); err != nil {
		first := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
		return strings.TrimSpace(strings.TrimPrefix(first, "next-steps:"))
	}
	return ""
}

// UsableNextStepsActionCount returns how many actions survive extraction.
func UsableNextStepsActionCount(body map[string]any) int { return len(nextStepsPayload(body).Actions) }

// NextStepsActionsField names the payload field the actions came from.
func NextStepsActionsField(body map[string]any) string {
	for _, key := range nextStepsActionFields {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "actions"
}

func nextStepsDecisionsField(body map[string]any) string {
	for _, key := range nextStepsDecisionFields {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "decisions"
}
