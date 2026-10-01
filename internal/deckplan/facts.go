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
// ellipsis so a run-on sentence doesn't swamp a content seed. 120 runes keeps
// an option list with its criteria ("three options were evaluated (build,
// partner, acquire) against cost, time-to-market, and risk") whole
// (go-slide-creator-gvbw8).
const maxFactLen = 120

// factClauseSplit splits a brief into clauses: sentence punctuation followed by
// whitespace (so decimals like "1.5M" survive), semicolons, newlines, commas
// followed by whitespace (so "1,000" survives), colons, and spaced dashes.
// Every one of these is a clause boundary wherever it appears — a fact that
// kept a semicolon would read as two facts spliced together, which is the
// shape this file exists to avoid (go-slide-creator-vmiy).
//
// CJK sentence and clause punctuation (。！？；，、：) is a boundary without
// trailing whitespace, since CJK text has none (go-slide-creator-csclk.48).
var factClauseSplit = regexp.MustCompile(`[.!?]+(?:\s+|$)|[;\n]+|,\s+|:\s+|\s+[-–—]\s+|[。！？；，、：]+`)

// splitBriefClauses splits a brief into clauses with factClauseSplit, with two
// list-preserving exceptions (go-slide-creator-gvbw8):
//
//   - a comma inside brackets is not a boundary, so "(build, partner with
//     Globex, acquire Initech)" stays one clause instead of being routed as the
//     fragments "partner with Globex" and "acquire Initech) against cost";
//   - a short lower-case comma item (two words or fewer, e.g. "time-to-market"
//     or "and risk") rejoins the clause before it, so "against cost,
//     time-to-market, and risk" stays one list.
//
// Every other boundary — sentence ends, semicolons, newlines, colons, spaced
// dashes — still splits wherever it appears (go-slide-creator-vmiy).
func splitBriefClauses(brief string) []string {
	depths := bracketDepths(brief)
	var parts []string
	var commaJoined []bool // part i was cut from part i-1 by a top-level comma
	last := 0
	joinedByComma := false
	for _, loc := range factClauseSplit.FindAllStringIndex(brief, -1) {
		isComma := strings.HasPrefix(brief[loc[0]:loc[1]], ",")
		if isComma && loc[0] > 0 && depths[loc[0]-1] > 0 {
			continue // a comma inside brackets is not a clause boundary
		}
		parts = append(parts, brief[last:loc[0]])
		commaJoined = append(commaJoined, joinedByComma)
		joinedByComma = isComma
		last = loc[1]
	}
	parts = append(parts, brief[last:])
	commaJoined = append(commaJoined, joinedByComma)

	var out []string
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if commaJoined[i] && len(out) > 0 && isShortListItem(trimmed) {
			out[len(out)-1] = strings.TrimRight(out[len(out)-1], " \t") + ", " + trimmed
			continue
		}
		out = append(out, part)
	}
	return out
}

// isShortListItem reports whether a comma-separated piece is a bare list item
// ("time-to-market", "and risk") rather than a clause of its own: at most two
// lower-case words after an optional leading and/or. A digit or a capital marks
// a quantity or a named entity ("EU launch"), which is a fact in its own right.
func isShortListItem(s string) bool {
	if s == "" || strings.ContainsAny(s, "0123456789") || strings.ToLower(s) != s {
		return false
	}
	words := strings.Fields(s)
	if len(words) > 0 && (strings.EqualFold(words[0], "and") || strings.EqualFold(words[0], "or")) {
		words = words[1:]
	}
	return len(words) > 0 && len(words) <= 2
}

// factLongClauseSplit splits a clause longer than maxFactLen at a joining word,
// so the facts after "... from 5% to 3% and headcount reached 1200 ..." are
// kept as their own facts instead of being cut off by the length cap
// (go-slide-creator-csclk.48).
var factLongClauseSplit = regexp.MustCompile(`(?i)\s+(?:and|while|whereas|but)\s+`)

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
// A digit straight after a CJK character still counts: CJK text has no spaces,
// so "增长23%" is a quantity (go-slide-creator-csclk.48).
var factQuantity = regexp.MustCompile(`(?:^|[^\p{L}\d]|[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}])[+\-−±]?[$€£¥]?\d`)

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
//
// numeric means the fact is a metric a KPI card or stat can carry: a percent,
// a currency amount, a magnitude ("12.5m", "4.2B"), points, or a count that is
// neither an action nor a date. "hiring 12 AEs" and "SOC 2 Type II in
// December" carry digits but are a to-do and a milestone, and an agent that
// copied them onto KPI cards built a dashboard out of a task list
// (go-slide-creator-gvbw8).
type briefFact struct {
	text    string
	numeric bool
	// series marks a metric that describes change over time (YoY, "from X to
	// Y", a trend): the only kind of fact a chart can be drawn from without
	// inventing data.
	series bool
	// ask marks a request for a decision or approval, often with a deadline.
	ask bool
	// action marks a to-do: something the team will do (hire, launch, fix).
	action bool
	// dated marks a milestone tied to a month, quarter or year.
	dated bool
	// option marks a clause that weighs alternatives.
	option bool
	// quote marks a quoted voice ("…" said the COO).
	quote bool
}

// factMetricUnit matches a quantity that is a metric on its face: a percent,
// a currency amount, a magnitude suffix, points/basis points, a multiple, or a
// thousands-grouped / four-plus-digit count. Years are stripped before it runs.
var factMetricUnit = regexp.MustCompile(`(?i)\d\s*%|[$€£¥]\s*\d|\d(?:\.\d+)?\s*(?:k|m|mm|bn|b|tn)\b|\b(?:usd|eur|gbp|chf|jpy)\s*\d|\d\s*(?:usd|eur|gbp|chf|jpy)\b|\d\s*(?:pts?|points|bps|basis points|x)\b|\d{1,3}(?:,\d{3})+|\d{4,}`)

// factYear matches a calendar year, which is a date rather than a quantity.
var factYear = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)

// factSeries matches the language of change over time.
var factSeries = regexp.MustCompile(`(?i)\b(?:yoy|qoq|mom|year[- ]over[- ]year|quarter[- ]over[- ]quarter|month[- ]over[- ]month|trend\w*|cagr|over the (?:last|past)|since (?:19|20)\d{2}|from [^,;]*\d[^,;]* to [^,;]*\d)`)

// factAsk matches a request for a decision.
var factAsk = regexp.MustCompile(`(?i)\b(?:approve|approval|approving|decide|decision|asks?|asking|request\w*|sign[- ]off|green[- ]light|go/no-go|funding)\b`)

// factAction matches a to-do in present or future form. Past forms ("we
// launched", "shipped") are results, not actions.
var factAction = regexp.MustCompile(`(?i)\b(?:hire|hires|hiring|launch|launches|launching|fix|fixes|fixing|build|building|deliver|delivering|roll(?:ing)? out|migrate|migrating|recruit|recruiting|implement|implementing|deploy|deploying|propose|proposing|priorities|prioriti[sz]e|need to|needs to|will|plan to|plans to|next steps?)\b`)

// factDeadline matches "by <date>".
var factDeadline = regexp.MustCompile(`(?i)\bby\s+(?:the\s+)?(?:end of\s+)?(?:\d{1,2}(?:st|nd|rd|th)?\s+)?(?:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec|q[1-4]|h[12]|fy|eo[qy]|(?:19|20)\d{2})`)

// factDated matches a month, quarter, half or fiscal-year reference.
var factDated = regexp.MustCompile(`(?i)\b(?:january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|sept|oct|nov|dec)\b|\b[QH][1-4]\b|\bFY\s?\d{2,4}\b|\b(?:19|20)\d{2}\b`)

// factOption matches a clause that weighs alternatives.
var factOption = regexp.MustCompile(`(?i)\b(?:options?|alternatives?|versus|vs\.?|scenarios?|criteria|trade-?offs?)\b`)

// factBenchmark matches a comparison against a benchmark rather than an
// alternative: "vs plan", "versus last year", "vs. target".
var factBenchmark = regexp.MustCompile(`(?i)\b(?:vs\.?|versus|against)\s+(?:the\s+)?(?:plan|target|budget|forecast|guidance|consensus|last|prior|previous|py|ly|ytd|benchmark)\b`)

// factMetricValue matches one unit-bearing value (a percent or a currency
// amount), for counting a clause's data points: three of them in one clause
// ("North 41%, South 33%, West 26%") are a series a chart can show; "$8M over
// 2 years versus $25M" is a comparison of two.
var factMetricValue = regexp.MustCompile(`[$€£¥]\s*\d+(?:[.,]\d+)*|\d+(?:[.,]\d+)*\s*%`)

// classifyFact sets a fact's routing flags from its text. quantity reports
// whether the clause carries a standalone number at all.
func classifyFact(text string, quantity bool) briefFact {
	f := briefFact{text: text}
	f.ask = factAsk.MatchString(text) || factDeadline.MatchString(text)
	f.action = f.ask || factAction.MatchString(text)
	f.dated = factDated.MatchString(text)
	// "vs plan" / "vs last year" is a benchmark, not a choice between options.
	f.option = factOption.MatchString(factBenchmark.ReplaceAllString(text, ""))
	switch {
	case factMetricUnit.MatchString(factYear.ReplaceAllString(text, "")):
		f.numeric = true
	case quantity && !f.action && !f.dated:
		// A bare count ("14 depots", "1200 people") is a metric unless the
		// clause is a to-do or a milestone.
		f.numeric = true
	}
	f.series = f.numeric && (factSeries.MatchString(text) || len(factMetricValue.FindAllString(text, -1)) >= 3)
	if f.numeric && !f.ask {
		// A metric that happens to contain a verb ("pipeline will grow 20%")
		// is still a metric, not a to-do.
		f.action = false
	}
	return f
}

// extractBriefFacts pulls quantity and named-entity clauses out of the brief,
// in brief order, de-duplicated. The brief's first clause is treated as the
// deck topic (it already feeds the opening slide's seed), so it only counts as
// a fact when it carries a quantity.
func extractBriefFacts(brief string) []briefFact {
	var clauses []string
	for _, c := range splitBriefClauses(brief) {
		if len([]rune(strings.TrimSpace(c))) > maxFactLen {
			clauses = append(clauses, factLongClauseSplit.Split(c, -1)...)
			continue
		}
		clauses = append(clauses, c)
	}
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
		quoted := cueQuote.MatchString(raw)
		option := factOption.MatchString(c)
		if !numeric && (!(entity || quoted || option) || isFirst) {
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
		if quoted && strings.Count(c, "\"")%2 == 1 {
			// The clause trim took one quote mark of the pair.
			c = strings.ReplaceAll(c, "\"", "")
		}
		f := classifyFact(c, numeric)
		f.quote = quoted
		out = append(out, f)
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

	placedGroup := placePercentSplitFacts(slides, facts, percentSplitFacts(brief))

	// Phase 1: quantities to numeric-friendly patterns, quotes to pull-quote
	// slides and options to comparison slides, in slide order.
	var leftover []briefFact
	for fi, f := range facts {
		if placedGroup[fi] {
			continue
		}
		if !placeFactOnHome(slides, f) {
			leftover = append(leftover, f)
		}
	}
	seedComparisonSlot(slides, facts, brief)

	// Phase 2: everything else, round-robin so facts spread across the deck
	// instead of piling onto the first evidence slide.
	var order []int
	for _, tier := range [][]string{{"evidence", "comparison"}, {"framework", "emphasis"}} {
		for i := range slides {
			if factEligible(slides[i]) && containsStr(tier, slides[i].NarrativeRole) {
				order = append(order, i)
			}
		}
	}
	next := 0
	for _, f := range leftover {
		placed := false
		for tries := 0; tries < len(order); tries++ {
			i := order[(next+tries)%len(order)]
			if factRoom(slides[i]) > 0 && slotAcceptsFact(slides[i], f) {
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

// factRoom is how many more facts a slide can take.
func factRoom(s Slide) int {
	return factCapacity(s.RecommendedPattern) - len(s.Facts)
}

// factEligible reports whether a slide takes brief facts at all: pattern
// slides only, never the title or closing.
func factEligible(s Slide) bool {
	return s.RecommendedPattern != "" && s.NarrativeRole != "opening" && s.NarrativeRole != "closing"
}

// placeFactOnHome puts a fact on the first slide built for its kind — a metric
// on a KPI / stat / chart pattern, a quote on a pull-quote, an option on a
// comparison slot — and reports whether it found one.
func placeFactOnHome(slides []Slide, f briefFact) bool {
	for i := range slides {
		if !factEligible(slides[i]) || factRoom(slides[i]) <= 0 {
			continue
		}
		pat, role := slides[i].RecommendedPattern, slides[i].NarrativeRole
		if (f.numeric && numericFriendlyPatterns[pat]) ||
			(f.quote && pat == "pull-quote") ||
			(f.option && role == "comparison") {
			slides[i].Facts = append(slides[i].Facts, f.text)
			return true
		}
	}
	return false
}

// seedComparisonSlot gives an empty comparison slot the comparison the brief
// makes even when that clause is not otherwise a fact ("compare build vs
// buy"). A clause that is already a fact went where its kind belongs.
func seedComparisonSlot(slides []Slide, facts []briefFact, brief string) {
	clause := comparisonClause(brief)
	if clause == "" || factTextIn(facts, clause) {
		return
	}
	for i := range slides {
		if factEligible(slides[i]) && slides[i].NarrativeRole == "comparison" && len(slides[i].Facts) == 0 {
			slides[i].Facts = append(slides[i].Facts, clause)
			return
		}
	}
}

// slotAcceptsFact reports what a slot can show: a KPI card or stat needs a
// metric, a pull-quote a quote, and a comparison an option — a to-do on a KPI
// card builds a dashboard out of a task list (go-slide-creator-gvbw8).
func slotAcceptsFact(s Slide, f briefFact) bool {
	pat := s.RecommendedPattern
	switch {
	case pat == "stat-hero" || pat == "kpi-inline" || strings.HasPrefix(pat, "kpi-"):
		return f.numeric
	case pat == "pull-quote":
		return f.quote
	case s.NarrativeRole == "comparison":
		return f.option
	}
	return true
}

// factTextIn reports whether text is one of the facts, ignoring case.
func factTextIn(facts []briefFact, text string) bool {
	for _, f := range facts {
		if strings.EqualFold(f.text, text) {
			return true
		}
	}
	return false
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
