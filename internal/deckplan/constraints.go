package deckplan

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Deck instructions (go-slide-creator-hf8tf).
//
// A brief mixes what the deck must say with how the deck must be built:
// "Board deck on our Q3 results, 9-10 slides", "use the warm-coral template",
// "audience: investors". The fact extractor used to read the second kind as
// content, so "9-10 slides" landed on a KPI card as a fact. Instructions about
// the deck itself are now lifted out of the brief before any fact is read and
// reported as constraints; the planner acts on the ones it can (the slide
// count sets the budget when the caller passed none, the audience fills
// meta.audience, a request for an agenda or dividers permits them in a short
// deck).

// Constraint is one instruction about the deck itself that the brief states.
type Constraint struct {
	// Kind is slide_count, template, audience, duration or structure.
	Kind string `json:"kind"`
	// Text is the brief's own wording, verbatim.
	Text string `json:"text"`
	// Value is the parsed value: "9-10" / "8" for slide_count, the template
	// name, the audience, minutes for duration, and "agenda" / "dividers" /
	// "no agenda" / "no dividers" for structure.
	Value string `json:"value,omitempty"`
}

// Constraint kinds.
const (
	ConstraintSlideCount = "slide_count"
	ConstraintTemplate   = "template"
	ConstraintAudience   = "audience"
	ConstraintDuration   = "duration"
	ConstraintStructure  = "structure"
)

// constraintNumberWord spells the slide counts a brief writes out.
var constraintNumberWord = map[string]int{
	"three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"eleven": 11, "twelve": 12, "fifteen": 15, "twenty": 20,
}

const constraintCount = `(\d{1,2}|three|four|five|six|seven|eight|nine|ten|eleven|twelve|fifteen|twenty)`

var (
	// cueSlideCount matches "8 slides", "9-10 slides", "7-slide", "in ten
	// slides", "max 12 pages", with the qualifiers that belong to the count.
	cueSlideCount = regexp.MustCompile(`(?i)\b(?:(?:in|within|across|over|max(?:imum)?(?: of)?|at most|no more than|up to|about|around|roughly|approximately|exactly|keep (?:it|this) to|limit(?:ed)? to)\s+)?` +
		constraintCount + `(?:\s*(?:-|–|—|to|or)\s*` + constraintCount + `)?[- ]?(?:slides?|pages?)\b(?:\s+(?:max(?:imum)?|or (?:less|fewer)|total|in total|at most))?`)

	// cueTemplateNamed matches "use the warm-coral template", "template:
	// midnight-blue" and "the forest-green template".
	cueTemplateNamed = regexp.MustCompile(`(?i)\b(?:(?:please\s+)?(?:use|using|with|in|on|apply|applying)\s+)?(?:the\s+|our\s+)?([a-z0-9]+(?:-[a-z0-9]+)+)\s+template\b` +
		`|\b(?:please\s+)?(?:use|using|apply|applying)\s+(?:the\s+|our\s+)?([a-z0-9]+)\s+template\b` +
		`|\b(?:(?:please\s+)?(?:use|using|with)\s+)?(?:the\s+)?template\s*[:=]\s*["']?([a-z0-9]+(?:-[a-z0-9]+)*)["']?`)

	// cueAudienceStated matches an audience the brief states as an
	// instruction: up to six words, none a number. "for the board" inside the
	// topic is left alone: it is part of what the deck is.
	cueAudienceStated = regexp.MustCompile(`(?i)\b(?:the\s+)?(?:target\s+)?audience\s*(?:is|are|will be|:|=)\s*((?:[\p{L}'’&/-]+\s?){1,6})|\b(?:for an audience of|aimed at|to be (?:presented|shown|pitched) to)\s+((?:[\p{L}'’&/-]+\s?){1,6})`)

	// cueAudienceEnd matches where a stated audience stops and the brief's
	// content resumes ("aimed at investors and covering three options").
	cueAudienceEnd = regexp.MustCompile(`(?i)\s+(?:and|with|who|which|that|covering|about|on|in|to|so|but)\s`)

	// cueDuration matches the length of the talk the deck supports.
	cueDuration = regexp.MustCompile(`(?i)\b(?:for\s+)?(?:an?\s+)?(\d{1,3})[- ]min(?:ute)?s?\s+(?:talk|presentation|pitch|slot|session|meeting|read)\b`)

	// cueStructureAsk matches a request for (or against) an agenda or section
	// dividers.
	cueStructureAsk = regexp.MustCompile(`(?i)\b(with|include|including|includes|add|plus|needs?|open(?:s|ing)? with|start(?:s|ing)? with|no|without|skip(?:ping)?|omit|drop)\s+(?:an?\s+|the\s+)?(agenda|table of contents|contents page|section dividers?|dividers?|section breaks?|chapters?)(?:\s+(?:slide|page)s?)?(?:\s+and\s+(?:an?\s+|the\s+)?(agenda|table of contents|contents page|section dividers?|dividers?|section breaks?|chapters?)(?:\s+(?:slide|page)s?)?)?`)

	constraintTidyPunct   = regexp.MustCompile(`\s*[,;]\s*([.:;!?])`)
	constraintTidyColon   = regexp.MustCompile(`([:;(])\s*[,;]\s*`)
	constraintTidyEmpty   = regexp.MustCompile(`\(\s*\)`)
	constraintTidySpaces  = regexp.MustCompile(`[ \t]{2,}`)
	constraintTidyLead    = regexp.MustCompile(`^[\s,;:.]+`)
	constraintTidyDouble  = regexp.MustCompile(`,\s*,`)
	constraintTidyStops   = regexp.MustCompile(`([.!?])\s*[.:]\s*`)
	constraintTidyLineEnd = regexp.MustCompile(`[ \t]*[,;:][ \t]*\n`)
)

// constraintSpan is a match to lift out of the brief.
type constraintSpan struct {
	start, end int
	c          Constraint
}

// parseConstraints lifts the deck instructions out of the brief. It returns
// the brief without them — punctuation closed up, so the remaining clauses
// read as written — and the instructions in brief order.
func parseConstraints(brief string) (string, []Constraint) {
	var spans []constraintSpan
	add := func(loc []int, kind, value string) {
		for _, s := range spans {
			if loc[0] < s.end && s.start < loc[1] {
				return // overlaps an instruction already taken
			}
		}
		spans = append(spans, constraintSpan{loc[0], loc[1], Constraint{
			Kind: kind, Text: strings.TrimSpace(brief[loc[0]:loc[1]]), Value: strings.TrimSpace(value),
		}})
	}

	// The first slide count is the deck's; a later one ("two slides on
	// pricing") is about content.
	if m := cueSlideCount.FindStringSubmatchIndex(brief); m != nil {
		value := strings.ToLower(brief[m[2]:m[3]])
		if m[4] >= 0 {
			value += "-" + strings.ToLower(brief[m[4]:m[5]])
		}
		add(m[:2], ConstraintSlideCount, value)
	}
	for _, m := range cueTemplateNamed.FindAllStringSubmatchIndex(brief, -1) {
		name := submatch(brief, m, 1) + submatch(brief, m, 2) + submatch(brief, m, 3)
		add(m[:2], ConstraintTemplate, strings.ToLower(name))
	}
	for _, m := range cueAudienceStated.FindAllStringSubmatchIndex(brief, -1) {
		g := 1
		if m[2] < 0 {
			g = 2
		}
		who, end := submatch(brief, m, g), m[1]
		if cut := cueAudienceEnd.FindStringIndex(who); cut != nil {
			who, end = who[:cut[0]], m[2*g]+cut[0]
		}
		end -= len(who) - len(strings.TrimRight(who, " \t\n"))
		add([]int{m[0], end}, ConstraintAudience, who)
	}
	for _, m := range cueDuration.FindAllStringSubmatchIndex(brief, -1) {
		add(m[:2], ConstraintDuration, submatch(brief, m, 1))
	}
	for _, m := range cueStructureAsk.FindAllStringSubmatchIndex(brief, -1) {
		negated := false
		switch strings.ToLower(strings.Fields(submatch(brief, m, 1))[0]) {
		case "no", "without", "skip", "skipping", "omit", "drop":
			negated = true
		}
		var values []string
		for _, g := range []int{2, 3} {
			what := strings.ToLower(submatch(brief, m, g))
			if what == "" {
				continue
			}
			v := "dividers"
			if strings.Contains(what, "agenda") || strings.Contains(what, "contents") {
				v = "agenda"
			}
			if negated {
				v = "no " + v
			}
			values = append(values, v)
		}
		add(m[:2], ConstraintStructure, strings.Join(values, ", "))
	}
	if len(spans) == 0 {
		return brief, nil
	}

	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var b strings.Builder
	out := make([]Constraint, 0, len(spans))
	last := 0
	for _, s := range spans {
		b.WriteString(brief[last:s.start])
		// A hyphenated count ("7-slide investor pitch") leaves the noun it
		// qualified; any other instruction leaves a gap the tidy pass closes.
		b.WriteString(" ")
		last = s.end
		out = append(out, s.c)
	}
	b.WriteString(brief[last:])
	return tidyAfterConstraints(b.String()), out
}

// submatch returns capture group g of a FindSubmatchIndex result, or "".
func submatch(s string, m []int, g int) string {
	if 2*g+1 >= len(m) || m[2*g] < 0 {
		return ""
	}
	return s[m[2*g]:m[2*g+1]]
}

// tidyAfterConstraints closes up the punctuation an instruction left behind:
// "results, . Facts" → "results. Facts", "company, : headline" → "company:
// headline".
func tidyAfterConstraints(s string) string {
	s = constraintTidySpaces.ReplaceAllString(s, " ")
	s = constraintTidyEmpty.ReplaceAllString(s, "")
	for range 3 {
		s = constraintTidyDouble.ReplaceAllString(s, ",")
		s = constraintTidyPunct.ReplaceAllString(s, "$1")
		s = constraintTidyColon.ReplaceAllString(s, "$1 ")
		s = constraintTidyStops.ReplaceAllString(s, "$1 ")
	}
	s = constraintTidyLineEnd.ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, " .", ".")
	s = strings.ReplaceAll(s, " :", ":")
	s = strings.ReplaceAll(s, "( ", "(")
	s = constraintTidySpaces.ReplaceAllString(s, " ")
	s = constraintTidyLead.ReplaceAllString(s, "")
	return strings.TrimRight(strings.TrimSpace(s), ",;: ")
}

// constraintValue returns the value of the first constraint of a kind, or "".
func constraintValue(cs []Constraint, kind string) string {
	for _, c := range cs {
		if c.Kind == kind {
			return c.Value
		}
	}
	return ""
}

// statedSlideCount is the slide count the brief states — the upper end of a
// range — or 0 when it states none.
func statedSlideCount(cs []Constraint) int {
	v := constraintValue(cs, ConstraintSlideCount)
	if v == "" {
		return 0
	}
	n := 0
	for _, part := range strings.Split(v, "-") {
		k, err := strconv.Atoi(part)
		if err != nil {
			k = constraintNumberWord[part]
		}
		n = max(n, k)
	}
	return n
}

// structureAsked reports whether the brief asks for the named structural
// slide ("agenda" or "dividers"), and whether it rules it out.
func structureAsked(cs []Constraint, what string) (asked, refused bool) {
	for _, c := range cs {
		if c.Kind != ConstraintStructure {
			continue
		}
		for _, v := range strings.Split(c.Value, ", ") {
			switch v {
			case what:
				asked = true
			case "no " + what:
				refused = true
			}
		}
	}
	return asked, refused
}

// Slide-budget bounds, the same range plan_deck's slide_budget argument takes.
const (
	minSlideBudget = 3
	maxSlideBudget = 30
)

// effectiveBudget is the budget the plan is drafted to: the caller's
// slide_budget when it passed one, else the count the brief states, else the
// default the caller supplied.
func effectiveBudget(p Params, cs []Constraint) int {
	budget := p.SlideBudget
	if !p.BudgetExplicit {
		if n := statedSlideCount(cs); n > 0 {
			budget = n
		}
	}
	return min(max(budget, minSlideBudget), maxSlideBudget)
}
