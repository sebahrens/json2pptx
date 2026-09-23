package deckplan

import (
	"regexp"
	"strings"
)

// A brief fact is a short clause lifted verbatim from the brief that carries
// either a quantity ("+23% revenue", "churn 4%", "adds 40 engineers") or a
// named entity ("EU expansion is on track"). The planner routes facts into the
// content seeds of the slides best suited to show them (quantities to KPI /
// stat / chart patterns first), and reports any fact it could not place in
// Result.UnplacedFacts so no brief fact silently disappears.

// maxFactLen caps a single fact's length; longer clauses are truncated with an
// ellipsis so a run-on sentence doesn't swamp a content seed.
const maxFactLen = 100

// factClauseSplit splits a brief into clauses: sentence punctuation followed by
// whitespace (so decimals like "1.5M" survive), semicolons, newlines, commas
// followed by whitespace (so "1,000" survives), colons, and spaced dashes.
// Every one of these is a clause boundary wherever it appears — a fact that
// kept a semicolon would read as two facts spliced together, which is the
// shape this file exists to avoid (go-slide-creator-vmiy).
var factClauseSplit = regexp.MustCompile(`[.!?]+(?:\s+|$)|[;\n]+|,\s+|:\s+|\s+[-–—]\s+`)

// factListMarker strips a list marker a brief's bullet leaves at the head of a
// clause: "- ", "* ", "• ", "1. ", "2) ". The marker is the author's list
// formatting, not part of the fact, and a seed that keeps it ships "- ARR
// reached EUR 12.5m" into a slide (go-slide-creator-vmiy).
var factListMarker = regexp.MustCompile(`^(?:[-*•▪·–—]|\(?\d{1,2}[.)])\s+`)

// factBracketPairs are the bracket kinds balanceFactBrackets tracks. A clause
// cut — by the splitter or by the length cap — can land inside one, and half a
// parenthesis reads as a typo in a content seed.
var factBracketPairs = map[rune]rune{'(': ')', '[': ']', '{': '}'}

// bracketDepths returns, for every byte offset in s, how many brackets are open
// at that point.
func bracketDepths(s string) []int {
	depths := make([]int, len(s))
	var stack []rune
	for i, r := range s {
		if closer, ok := factBracketPairs[r]; ok {
			stack = append(stack, closer)
		} else if len(stack) > 0 && r == stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
		}
		depths[i] = len(stack)
	}
	return depths
}

// balanceFactBrackets drops an unterminated bracket clause from the tail of a
// fact. Truncation at maxFactLen can cut inside a parenthetical even when the
// split did not, and half a parenthesis reads as a typo in a content seed.
func balanceFactBrackets(s string) string {
	depths := bracketDepths(s)
	if len(depths) == 0 || depths[len(depths)-1] == 0 {
		return s
	}
	// Cut back to the last point where every bracket was closed.
	for i := len(depths) - 1; i >= 0; i-- {
		if depths[i] == 0 {
			return strings.TrimRight(strings.TrimSpace(s[:i+1]), " \t,;:-–—")
		}
	}
	return ""
}

// factQuantity matches a standalone number (not glued to a preceding letter,
// so period labels like "Q3", "FY24", "H1" do not count as quantities),
// optionally signed and currency-prefixed.
var factQuantity = regexp.MustCompile(`(?:^|[^\p{L}\d])[+\-−±]?[$€£¥]?\d`)

// namedPercentSplit matches a contiguous three-category percentage breakdown
// ("North 41%, South 33%, West 26%"). Keeping this as one chart signal and
// one fact group prevents the planner from scattering its slices across slides.
var namedPercentSplit = regexp.MustCompile(`(?i)[\p{L}][\p{L}\-]*\s+\d+(?:\.\d+)?\s*%\s*,\s*[\p{L}][\p{L}\-]*\s+\d+(?:\.\d+)?\s*%\s*,\s*[\p{L}][\p{L}\-]*\s+\d+(?:\.\d+)?\s*%`)

func percentSplitFacts(brief string) []string {
	match := namedPercentSplit.FindString(brief)
	if match == "" {
		return nil
	}
	parts := strings.Split(match, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func briefHasPhaseSequence(brief string) bool {
	lower := strings.ToLower(brief)
	for _, cue := range []string{"phases", "phase 1", "phase one", "roadmap", "milestones", "workstreams", "implementation plan"} {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

func briefHasComparisonMatrix(brief string) bool {
	lower := strings.ToLower(brief)
	if !strings.Contains(lower, "criteria") {
		return false
	}
	for _, cue := range []string{"vendor", "option", "alternative"} {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

// factAcronym matches an all-caps token such as "EU", "APAC", "ARR".
var factAcronym = regexp.MustCompile(`\b[A-Z]{2,6}s?\b`)

// factProperNoun matches a capitalized word that is not the first word of the
// clause (the first word is capitalized by sentence convention, not because it
// names something).
var factProperNoun = regexp.MustCompile(`\s[A-Z][a-z]{2,}`)

// factLeadingConjunction strips a joining word left at the start of a clause
// by the comma split ("…, and we serve 1,200 customers" -> "we serve …").
var factLeadingConjunction = regexp.MustCompile(`(?i)^(?:and|but|while|plus|also|so)\s+`)

// briefFact is one extracted fact.
type briefFact struct {
	text    string
	numeric bool
}

// extractBriefFacts pulls quantity and named-entity clauses out of the brief,
// in brief order, de-duplicated. The brief's first clause is treated as the
// deck topic (it already feeds the opening slide's seed), so it only counts as
// a fact when it carries a quantity.
func extractBriefFacts(brief string) []briefFact {
	clauses := factClauseSplit.Split(brief, -1)
	var out []briefFact
	seen := make(map[string]bool)
	first := true
	for _, raw := range clauses {
		c := strings.TrimSpace(strings.Trim(raw, " \t\"'"))
		c = factListMarker.ReplaceAllString(c, "")
		c = factLeadingConjunction.ReplaceAllString(c, "")
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		isFirst := first
		first = false

		numeric := factQuantity.MatchString(c)
		entity := factAcronym.MatchString(c) || factProperNoun.MatchString(c)
		if !numeric && (!entity || isFirst) {
			continue
		}
		c = balanceFactBrackets(TruncateBrief(c, maxFactLen))
		if c == "" {
			continue
		}
		key := strings.ToLower(c)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, briefFact{text: c, numeric: numeric})
	}
	return out
}

// numericFriendlyPatterns are patterns whose primary content is a number or a
// quantitative comparison — the first home for quantity facts.
var numericFriendlyPatterns = map[string]bool{
	"stat-hero":                    true,
	"kpi-inline":                   true,
	"kpi-2up":                      true,
	"kpi-3up":                      true,
	"kpi-4up":                      true,
	"kpi-5up":                      true,
	"kpi-6up":                      true,
	"chart-insights-split":         true,
	"horizontal-bar-with-callouts": true,
	"waterfall-bridge":             true,
	"driver-tree":                  true,
}

// factCapacity is how many brief facts a slide's content seed can carry for
// the given pattern: one per KPI card, one for a single-number/quote pattern,
// two otherwise.
func factCapacity(pattern string) int {
	switch pattern {
	case "chart-insights-split":
		return 3 // one named three-category split stays together
	case "kpi-2up":
		return 2
	case "kpi-3up", "kpi-inline":
		return 3
	case "kpi-4up":
		return 4
	case "kpi-5up":
		return 5
	case "kpi-6up":
		return 6
	case "stat-hero", "pull-quote":
		return 1
	default:
		return 2
	}
}

// assignBriefFacts distributes the brief's facts into the content seeds of
// pattern-bearing content slides and returns the facts that found no slot.
// Quantities go to numeric-friendly patterns (KPI / stat / chart) first; every
// remaining fact is spread round-robin across evidence and comparison slides,
// then framework and emphasis slides. Title and closing slides never receive
// facts. The returned slice is never nil so unplaced_facts always serializes as
// an array.
func assignBriefFacts(slides []Slide, brief string) []string {
	facts := extractBriefFacts(brief)
	unplaced := make([]string, 0)
	if len(facts) == 0 {
		return unplaced
	}

	remaining := func(i int) int {
		return factCapacity(slides[i].RecommendedPattern) - len(slides[i].Facts)
	}
	eligible := func(i int) bool {
		s := slides[i]
		return s.RecommendedPattern != "" && s.NarrativeRole != "opening" && s.NarrativeRole != "closing"
	}
	placedGroup := placePercentSplitFacts(slides, facts, percentSplitFacts(brief))

	// Phase 1: quantities to numeric-friendly patterns, in slide order.
	var leftover []briefFact
	for fi, f := range facts {
		if placedGroup[fi] {
			continue
		}
		placed := false
		if f.numeric {
			for i := range slides {
				if eligible(i) && numericFriendlyPatterns[slides[i].RecommendedPattern] && remaining(i) > 0 {
					slides[i].Facts = append(slides[i].Facts, f.text)
					placed = true
					break
				}
			}
		}
		if !placed {
			leftover = append(leftover, f)
		}
	}

	// Phase 2: everything else, round-robin so facts spread across the deck
	// instead of piling onto the first evidence slide.
	var order []int
	for _, tier := range [][]string{{"evidence", "comparison"}, {"framework", "emphasis"}} {
		for i := range slides {
			if eligible(i) && containsStr(tier, slides[i].NarrativeRole) {
				order = append(order, i)
			}
		}
	}
	next := 0
	for _, f := range leftover {
		placed := false
		for tries := 0; tries < len(order); tries++ {
			i := order[(next+tries)%len(order)]
			if remaining(i) > 0 {
				slides[i].Facts = append(slides[i].Facts, f.text)
				next = (next + tries + 1) % len(order)
				placed = true
				break
			}
		}
		if !placed {
			unplaced = append(unplaced, f.text)
		}
	}

	// Fold the placed facts into each slide's prose seed so agents (and
	// make_deck's derived titles) see the brief's actual numbers. Facts are
	// separate sentences: joining them with "; " read as one malformed clause,
	// and the semicolon was exactly the character the clause splitter had just
	// cut on (go-slide-creator-vmiy).
	for i := range slides {
		if len(slides[i].Facts) > 0 {
			slides[i].ContentSeed = strings.Join(slides[i].Facts, ". ") + " — " + slides[i].ContentSeed
		}
	}
	return unplaced
}

func placePercentSplitFacts(slides []Slide, facts []briefFact, group []string) map[int]bool {
	placed := make(map[int]bool)
	if len(group) != 3 {
		return placed
	}
	for si := range slides {
		if slides[si].NarrativeRole == "opening" || slides[si].NarrativeRole == "closing" || slides[si].RecommendedPattern != "chart-insights-split" || factCapacity(slides[si].RecommendedPattern)-len(slides[si].Facts) < len(group) {
			continue
		}
		indices := make([]int, 0, len(group))
		for _, text := range group {
			for fi, fact := range facts {
				if !containsInt(indices, fi) && strings.EqualFold(fact.text, text) {
					indices = append(indices, fi)
					break
				}
			}
		}
		if len(indices) != len(group) {
			return placed
		}
		for _, fi := range indices {
			slides[si].Facts = append(slides[si].Facts, facts[fi].text)
			placed[fi] = true
		}
		break
	}
	return placed
}

func containsInt(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
