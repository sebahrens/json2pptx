package deckplan

import (
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Cycle slides in a brief (go-slide-creator-53v5u).
//
// "Our operating rhythm is a six-phase continuous improvement loop fed by a
// two-step onboarding intake" names a picture: a loop, with steps that feed
// it. The storyline planner had no word for it, so the sentence became a fact
// under the executive summary and the agent was left to find the circular
// patterns by itself. A sentence that names a loop, a hub or nested rings is
// now drafted as a cycle slide, with the style its words point at, the phases
// it lists (or a placeholder per phase when it only counts them) and the
// intake steps when it has them.

var (
	// cueNamedCycle names a loop, a hub or nested rings outright. Words that
	// also read as prose ("sales cycle", "recurring revenue", "in the loop")
	// are left out: they name the picture only beside a count or a list
	// (cueCycleWeak).
	cueNamedCycle = regexp.MustCompile(`(?i)\b(?:life[- ]?cycles?|flywheels?|pdca|plan[-–, ]+do[-–, ]+check[-–, ]+act|operating (?:rhythm|cadence)|continuous[- ]improvement|closed[- ]loop|feedback loops?|virtuous (?:cycle|circle)|(?:devops|infinity) loop|figure[- ](?:eight|8)|hub[- ]and[- ]spokes?|onion (?:model|diagram|layers?)|concentric)\b` +
		`|\b(?:two|three|four|five|six|seven|eight|\d)[- ](?:phase|step|stage)s?\s+(?:[\p{L}-]+\s+){0,3}(?:cycle|loop)\b` +
		`|\b(?:recurring|iterative|repeating)\s+(?:[\p{L}-]+\s+){0,2}(?:cycle|loop)\b` +
		`|\bonboarding\b[^.;]*\bthen\b[^.;]*\b(?:recurring|loop|cycle)\b`)
	// cueCycleWeak is a loop word that needs a list of 3–8 items beside it.
	cueCycleWeak = regexp.MustCompile(`(?i)\b(?:cycle|loop|ecosystem|nested|rings?|spokes?)\b`)
	// cueCycleNot are the phrases in which a loop word is not a picture.
	cueCycleNot = regexp.MustCompile(`(?i)\b(?:sales|billing|budget|business|economic|credit|release|hype|planning|audit|cash conversion) cycles?\b|\bcycle times?\b|\bin the loop\b|\bout of the loop\b`)

	cueCycleFigureEight = regexp.MustCompile(`(?i)\b(?:devops|infinity|figure[- ](?:eight|8)|two (?:coupled |linked |connected )?loops|dual[- ]loop)\b`)
	cueCycleRadial      = regexp.MustCompile(`(?i)\b(?:hub[- ]and[- ]spokes?|spokes?|ecosystem|around (?:a|the|one) (?:central|shared|common|single))\b`)
	cueCycleConcentric  = regexp.MustCompile(`(?i)\b(?:onion|concentric|nested)\b`)
	cueCycleIntake      = regexp.MustCompile(`(?i)\b(?:intake|fed by|feeds? (?:into )?(?:the|a|one) (?:[\p{L}-]+ ){0,2}(?:loop|cycle))\b|\bonboarding\b[^.;]*\bthen\b`)

	// cueCyclePhaseCount reads "six-phase loop" / "a loop of 6 phases";
	// cueCycleIntakeCount "two-step onboarding intake" / "intake of 2 steps".
	cueCyclePhaseCount  = regexp.MustCompile(`(?i)\b(two|three|four|five|six|seven|eight|\d)[- ](?:phase|stage)s?\b`)
	cueCycleIntakeCount = regexp.MustCompile(`(?i)\b(one|two|three|\d)[- ]step\b[^.;]{0,24}\b(?:intake|onboarding)\b|\b(?:intake|onboarding)\b[^.;]{0,16}\bof\s+(one|two|three|\d)\s+steps?\b`)
)

// cycleGuidance is the slot guidance of a cycle slide.
const cycleGuidance = "The loop as a picture: 3-8 phases in order (labels up to 24 characters), one highlighted, and the style its shape has — ring or nodes for one loop, intake when one-off steps feed it, figure_eight for two coupled loops, radial for peers around a center, concentric for layers that contain one another; the title says what the loop produces."

// namesCycle reports whether a sentence names a cycle slide: a loop word that
// is unambiguous, or an ambiguous one beside a list the picture can hold.
func namesCycle(n *namedSlide, all string) bool {
	all = cueCycleNot.ReplaceAllString(all, "")
	if cueNamedCycle.MatchString(all) {
		return true
	}
	return cueCycleWeak.MatchString(all) && len(n.items) >= 3 && len(n.items) <= 8
}

// cycleStyleFor is the style a sentence's words point at, "" for the default.
func cycleStyleFor(all string) string {
	switch {
	case cueCycleFigureEight.MatchString(all):
		return "figure_eight"
	case cueCycleRadial.MatchString(all):
		return "radial"
	case cueCycleConcentric.MatchString(all):
		return "concentric"
	case cueCycleIntake.MatchString(all):
		return "intake"
	}
	return ""
}

// countOf reads the number a count cue captured.
func countOf(m []string) int {
	for _, w := range m[1:] {
		if w == "" {
			continue
		}
		w = strings.ToLower(w)
		if w == "one" {
			return 1
		}
		if n, ok := namedCountWords[w]; ok {
			return n
		}
		if len(w) == 1 && w[0] >= '0' && w[0] <= '9' {
			return int(w[0] - '0')
		}
	}
	return 0
}

// fillList is n placeholders for a list the brief counts but does not name.
func fillList(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = patterns.FillPlaceholder
	}
	return out
}

// cycleFields drafts a cycle slide from its sentence: the style, the phases
// (the listed items, else one placeholder per counted phase) and, for an
// intake, a placeholder per counted intake step.
func cycleFields(n *namedSlide) map[string]any {
	all := n.sentence
	fields := map[string]any{}
	style := cycleStyleFor(all)
	if style != "" {
		fields["style"] = style
	}
	switch {
	case len(n.items) >= 3 && len(n.items) <= 8:
		phases := make([]any, 0, len(n.items))
		for _, it := range n.items {
			if m := namedParen.FindStringSubmatch(it); m != nil && strings.TrimSpace(m[1]) != "" {
				phases = append(phases, map[string]any{"label": strings.TrimSpace(m[1]), "description": strings.TrimSpace(m[2])})
				continue
			}
			phases = append(phases, it)
		}
		fields["phases"] = phases
	default:
		if m := cueCyclePhaseCount.FindStringSubmatch(all); m != nil {
			if c := countOf(m); c >= 3 && c <= 8 {
				fields["phases"] = fillList(c)
			}
		}
	}
	switch style {
	case "intake":
		steps := 1
		if m := cueCycleIntakeCount.FindStringSubmatch(all); m != nil {
			if c := countOf(m); c >= 1 && c <= 3 {
				steps = c
			}
		}
		fields["intake"] = fillList(steps)
	case "radial":
		fields["center"] = patterns.FillPlaceholder
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// topicCycle drafts a cycle slide from the brief's topic sentence. The named
// vocabulary reads the sentences after the topic; a brief that is one
// sentence ("our operating rhythm is a six-phase loop …") would otherwise
// carry its picture nowhere. It returns nil when the topic names no loop or a
// later sentence already does.
func topicCycle(brief string, named []namedSlide) *namedSlide {
	for _, n := range named {
		if n.kind == "cycle" {
			return nil
		}
	}
	spans := sentenceSpans(brief)
	if len(spans) == 0 {
		return nil
	}
	n := readSentence(brief[spans[0][0]:spans[0][1]])
	if !namesCycle(n, n.sentence) {
		return nil
	}
	n.kind, n.slot, n.guidance = "cycle", namedSlot["cycle"], cycleGuidance
	n.fields = cycleFields(n)
	n.facts = []string{n.sentence}
	return n
}
