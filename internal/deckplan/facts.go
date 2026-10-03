package deckplan

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A brief fact is a short clause lifted verbatim from the brief that carries
// either a quantity ("+23% revenue", "churn 4%", "adds 40 engineers") or a
// named entity ("EU expansion is on track"). The planner routes facts into the
// content seeds of the slides best suited to show them (quantities to KPI /
// stat / chart patterns first), and reports any fact it could not place in
// Result.UnplacedFacts so no brief fact silently disappears.

// maxFactLen caps a single fact's length; a longer clause is split — at a
// joining word, then at a word boundary outside brackets — into several
// facts so a run-on sentence doesn't swamp a content seed. Nothing is cut off:
// truncating a clause dropped its tail, and with it the second option of a
// comparison and its amounts, while unplaced_facts stayed empty
// (go-slide-creator-ze5u7). 120 runes keeps an option list with its criteria
// ("three options were evaluated (build, partner, acquire) against cost,
// time-to-market, and risk") whole (go-slide-creator-gvbw8).
const maxFactLen = 120

// maxComparisonFactLen is the cap for a clause that weighs alternatives
// ("Compare building an internal team (€0.8m per year, launch in 6 months)
// with outsourcing (€0.5m per year, launch in 2 months)"). Splitting it would
// put the two options on different slides, so it stays whole up to twice the
// normal cap (go-slide-creator-ze5u7).
const maxComparisonFactLen = 2 * maxFactLen

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

// briefClause is one clause of the brief with its place in a list.
type briefClause struct {
	text string
	// group names the list the clause was itemised under ("three options to
	// fix margin: a, b, c" puts a, b and c in group "option"); "" otherwise.
	group string
	// header marks the clause that introduced a list.
	header bool
}

// splitBriefClauses splits a brief into clauses with factClauseSplit, with
// these list-preserving exceptions (go-slide-creator-gvbw8,
// go-slide-creator-hf8tf):
//
//   - a comma or colon inside brackets is not a boundary, so "(build, partner
//     with Globex, acquire Initech)" and "(Q2: 63.0%)" stay inside their
//     clause instead of being routed as fragments;
//   - a short lower-case comma item (two words or fewer, e.g. "time-to-market"
//     or "and risk") rejoins the clause before it, so "against cost,
//     time-to-market, and risk" stays one list;
//   - a bare number after a comma rejoins the clause before it, so "revenue
//     last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2" stays one series instead
//     of five facts on four slides;
//   - a short label keeps the clause it labels ("recommendation: renegotiate
//     now", "source: management accounts"): the label alone says nothing and
//     the clause alone lost what it was.
//
// Every other boundary — sentence ends, semicolons, newlines, spaced dashes,
// and a colon after the topic or a list header — still splits wherever it
// appears (go-slide-creator-vmiy).
func splitBriefClauses(brief string) []string {
	clauses := splitClauses(brief)
	out := make([]string, len(clauses))
	for i, c := range clauses {
		out[i] = c.text
	}
	return out
}

// Separator kinds between two parts of a brief.
const (
	sepHard  = byte('x')
	sepComma = byte(',')
	sepColon = byte(':')
)

// splitClauses is splitBriefClauses with each clause's list membership.
func splitClauses(brief string) []briefClause {
	depths := bracketDepths(brief)
	type part struct {
		text          string
		before, after byte
	}
	var parts []part
	last := 0
	before := sepHard
	for _, loc := range factClauseSplit.FindAllStringIndex(brief, -1) {
		sep := sepHard
		switch {
		case strings.HasPrefix(brief[loc[0]:loc[1]], ","):
			sep = sepComma
		case strings.HasPrefix(brief[loc[0]:loc[1]], ":"):
			sep = sepColon
		}
		if sep != sepHard && loc[0] > 0 && depths[loc[0]-1] > 0 {
			continue // a comma or colon inside brackets is not a clause boundary
		}
		parts = append(parts, part{brief[last:loc[0]], before, sep})
		before = sep
		last = loc[1]
	}
	parts = append(parts, part{brief[last:], before, sepHard})

	var out []briefClause
	group, label := "", ""
	for i, p := range parts {
		trimmed := strings.TrimSpace(p.text)
		if p.before == sepHard {
			group = ""
		}
		// Under a list header every comma item is an item of the list, however
		// short; a bare number always continues the item before it.
		if p.before == sepComma && len(out) > 0 && label == "" && ((group == "" && isShortListItem(trimmed)) || isBareNumber(trimmed)) {
			out[len(out)-1].text = strings.TrimRight(out[len(out)-1].text, " \t") + ", " + trimmed
			continue
		}
		text := p.text
		if label != "" {
			text, label = label+": "+trimmed, ""
		}
		if p.after == sepColon && i+1 < len(parts) {
			name := strings.TrimSpace(factListMarker.ReplaceAllString(strings.TrimSpace(text), ""))
			g := listHeaderGroup(name)
			switch {
			case len(out) == 0:
				// The topic: what follows the colon is the brief itself, not
				// a list the topic heads.
				group = ""
			case isFillerLabel(name):
				group = ""
				continue // "Facts:" introduces the brief's facts and is not one
			case g != "":
				out = append(out, briefClause{text: text, group: g, header: true})
				group = g
				continue
			case isShortLabel(name):
				label, group = name, ""
				continue
			default:
				group = ""
			}
			out = append(out, briefClause{text: text})
			continue
		}
		out = append(out, briefClause{text: text, group: group})
	}
	return out
}

// factBareNumber matches a list item that is only a number: "42.3", "€4m",
// "12%", "and 48.2".
var factBareNumber = regexp.MustCompile(`^(?i)(?:(?:and|or)\s+)?[+\-−±]?[$€£¥]?\d[\d.,]*\s*(?:%|[a-z]{1,3})?$`)

// isBareNumber reports whether a comma-separated piece is a bare number, which
// continues the list of numbers before it rather than starting a fact.
func isBareNumber(s string) bool {
	return factBareNumber.MatchString(s)
}

// factFillerLabel matches a label that only announces the brief's content.
var factFillerLabel = regexp.MustCompile(`^(?i)(?:the\s+)?(?:key\s+|main\s+|hard\s+)?(?:facts?|data|data points?|details?|context|background|notes?|inputs?|figures|numbers|content|contents|brief|information|info)$`)

// isFillerLabel reports whether a colon label only announces what follows
// ("Facts:", "Key data:").
func isFillerLabel(s string) bool {
	return factFillerLabel.MatchString(strings.TrimSpace(s))
}

// isShortLabel reports whether a colon label names the clause after it
// ("recommendation", "source", "main risk"): at most four words.
func isShortLabel(s string) bool {
	n := len(strings.Fields(s))
	return n > 0 && n <= 4
}

// factListHeader matches the plural noun a list header counts, with the group
// its items belong to.
var factListHeader = regexp.MustCompile(`(?i)\b(?:(options|alternatives|scenarios|choices)|(milestones|phases|stages|deadlines)|(next steps|steps|actions|priorities|initiatives|workstreams|to-?dos)|(risks|threats|concerns)|(features|pillars|themes|drivers|reasons|goals|objectives|findings|recommendations|benefits|issues|problems|learnings|lessons|highlights|results|metrics|kpis))\b`)

// List groups: what the items under a list header are.
const (
	groupOption    = "option"
	groupMilestone = "milestone"
	groupAction    = "action"
	groupRisk      = "risk"
	groupList      = "list"
)

// listHeaderGroup returns the group a colon label introduces ("three options
// to fix margin" → option, "next steps" → action), or "" when the label is not
// a list header.
func listHeaderGroup(label string) string {
	m := factListHeader.FindStringSubmatch(label)
	if m == nil {
		return ""
	}
	for i, g := range []string{groupOption, groupMilestone, groupAction, groupRisk, groupList} {
		if m[i+1] != "" {
			return g
		}
	}
	return ""
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
// A joining word inside brackets is not a split point: "(€0.8m and 6 months)"
// is one parenthetical (go-slide-creator-ze5u7).
var factLongClauseSplit = regexp.MustCompile(`(?i)\s+(?:and|while|whereas|but)\s+`)

// splitTopLevel splits s at every match of re that starts outside brackets.
func splitTopLevel(s string, re *regexp.Regexp) []string {
	depths := bracketDepths(s)
	var out []string
	last := 0
	for _, loc := range re.FindAllStringIndex(s, -1) {
		if depths[loc[0]] > 0 {
			continue
		}
		out = append(out, s[last:loc[0]])
		last = loc[1]
	}
	return append(out, s[last:])
}

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
// at that point. Every byte of a multi-byte rune carries the rune's depth: a
// continuation byte left at zero made "(€0.5m" look closed inside the "€", so
// a cut there split the rune and left the parenthesis open, and a comma after
// a CJK character inside brackets read as a top-level clause boundary
// (go-slide-creator-ze5u7).
func bracketDepths(s string) []int {
	depths := make([]int, len(s))
	var stack []rune
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		if closer, ok := factBracketPairs[r]; ok {
			stack = append(stack, closer)
		} else if len(stack) > 0 && r == stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
		}
		for j := i; j < i+w; j++ {
			depths[j] = len(stack)
		}
		i += w
	}
	return depths
}

// isFactCloser reports whether r closes a tracked bracket.
func isFactCloser(r rune) bool {
	return r == ')' || r == ']' || r == '}'
}

// dropStrayClosers removes closing brackets that close nothing — the tail of a
// parenthetical the clause split cut at its opening half.
func dropStrayClosers(s string) string {
	var b strings.Builder
	var stack []rune
	for _, r := range s {
		if closer, ok := factBracketPairs[r]; ok {
			stack = append(stack, closer)
		} else if len(stack) > 0 && r == stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
		} else if len(stack) == 0 && isFactCloser(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// splitOpenBrackets splits a fact at every bracket a clause cut left open:
// "ARR reached €12.5m (up 31%" becomes "ARR reached €12.5m" and "up 31%".
// Half a parenthesis reads as a typo in a content seed, but dropping the
// bracketed tail lost its numbers (go-slide-creator-ze5u7), so the tail is
// kept as a piece of its own. Stray closing brackets are removed.
func splitOpenBrackets(s string) []string {
	s = strings.TrimSpace(dropStrayClosers(s))
	depths := bracketDepths(s)
	if len(depths) == 0 || depths[len(depths)-1] == 0 {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	// The unmatched opener is the byte after the last point where every
	// bracket was closed. Openers are ASCII, so it is one byte wide.
	cut := 0
	for i := len(depths) - 1; i >= 0; i-- {
		if depths[i] == 0 {
			cut = i + 1
			break
		}
	}
	var out []string
	if head := strings.TrimRight(strings.TrimSpace(s[:cut]), " \t,;:-–—"); head != "" {
		out = append(out, head)
	}
	return append(out, splitOpenBrackets(s[cut+1:])...)
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

// factMonthWord lists month names, which bind to the number on either side:
// "January 2027", "30 November".
var factMonthWord = map[string]bool{
	"january": true, "february": true, "march": true, "april": true, "may": true, "june": true,
	"july": true, "august": true, "september": true, "october": true, "november": true, "december": true,
	"jan": true, "feb": true, "mar": true, "apr": true, "jun": true, "jul": true, "aug": true,
	"sep": true, "sept": true, "oct": true, "nov": true, "dec": true,
}

// factCurrencyCode lists the currency codes that bind to the number on either
// side: "EUR 12m", "12m EUR".
var factCurrencyCode = map[string]bool{"usd": true, "eur": true, "gbp": true, "chf": true, "jpy": true}

// bindsNumber reports whether the space between prev and next sits inside a
// numeric, currency or date token ("€ 12m", "EUR 12m", "January 2027"), or
// right after a number, which binds the word it counts or measures ("12 %",
// "6 months", "14 depots"): a fact is never split there.
func bindsNumber(prev, next string) bool {
	pf, nf := strings.Fields(prev), strings.Fields(next)
	if len(pf) == 0 || len(nf) == 0 {
		return false
	}
	pw := strings.ToLower(strings.Trim(pf[len(pf)-1], "([{"))
	nw := strings.ToLower(strings.TrimRight(nf[0], ",;:.)]}"))
	pr, _ := utf8.DecodeLastRuneInString(pw)
	nr, _ := utf8.DecodeRuneInString(nw)
	switch {
	case strings.ContainsRune("$€£¥", pr):
		return true
	case (factCurrencyCode[pw] || factMonthWord[pw]) && unicode.IsDigit(nr):
		return true
	case strings.IndexFunc(pw, unicode.IsDigit) >= 0:
		return true
	}
	return false
}

// chunkFact splits a fact longer than limit runes into pieces of at most
// limit runes where it can, of roughly even length so the last piece is not a
// stray word. It cuts only at a space outside brackets and never inside a
// number, an amount or a date; a fact with no such space stays whole rather
// than being truncated (go-slide-creator-ze5u7).
func chunkFact(s string, limit int) []string {
	var out []string
	for n := utf8.RuneCountInString(s); n > limit; n = utf8.RuneCountInString(s) {
		pieces := (n + limit - 1) / limit
		cut := factBreak(s, limit, (n+pieces-1)/pieces)
		if cut <= 0 {
			break
		}
		if head := strings.TrimRight(strings.TrimSpace(s[:cut]), " \t,;:-–—"); head != "" {
			out = append(out, head)
		}
		s = strings.TrimSpace(s[cut:])
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

// factBreak returns the byte offset of the allowed break within limit runes
// nearest to target runes, else the first allowed break after limit, else -1.
func factBreak(s string, limit, target int) int {
	depths := bracketDepths(s)
	best, bestDist, after := -1, 0, -1
	runeIdx := 0
	for i, r := range s {
		if i > 0 && unicode.IsSpace(r) && depths[i] == 0 && !bindsNumber(s[:i], s[i:]) {
			if runeIdx <= limit {
				if d := abs(runeIdx - target); best < 0 || d < bestDist {
					best, bestDist = i, d
				}
			} else if after < 0 {
				after = i
			}
		}
		runeIdx++
	}
	if best > 0 {
		return best
	}
	return after
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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
	// recommend marks the recommendation: the alternative backed and why.
	recommend bool
	// risk marks a clause that names a risk.
	risk bool
	// source marks the provenance of the brief's figures ("source: …").
	source bool
	// list marks a metric given as a list of three or more values — the one
	// fact a chart can be drawn from as written.
	list bool
	// group is the list the fact was itemised under (groupOption, …), and
	// header marks the clause that introduced the list.
	group  string
	header bool
	// plain marks a clause with no quantity, name, option, quote or
	// recommendation: a statement the brief makes that no slide kind is built
	// for. It is still routed or reported, never dropped.
	plain bool
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

// factCompareVerb matches "compare A with B" (also "to", "against", "versus"),
// and factComparedWith matches "A compared with B". Either weighs two
// alternatives unless B is a benchmark (go-slide-creator-fu6uy).
var (
	factCompareVerb   = regexp.MustCompile(`(?i)\bcompar(?:e|es|ing)\s+\S.*?\s+(?:with|to|against|versus|vs\.?)\s+(\S.*)`)
	factComparedWith  = regexp.MustCompile(`(?i)\bcompared\s+(?:with|to|against)\s+(\S.*)`)
	factLeadingFiller = regexp.MustCompile(`(?i)^(?:(?:the|our|its|their|this|that)\s+)+`)
)

// factBenchmarkWord lists the words that open a benchmark rather than an
// alternative: "compared with plan", "compare revenue to last year",
// "compared with a year ago".
var factBenchmarkWord = map[string]bool{
	"plan": true, "target": true, "targets": true, "budget": true, "forecast": true, "guidance": true,
	"consensus": true, "last": true, "prior": true, "previous": true, "py": true, "ly": true,
	"ytd": true, "benchmark": true, "benchmarks": true, "a": true, "same": true, "year": true,
	"quarter": true, "month": true, "peers": true, "industry": true, "expectations": true,
}

// factPeriodLabel matches a reporting period used as a benchmark: q3, h1, fy24.
var factPeriodLabel = regexp.MustCompile(`^(?:q[1-4]|h[12]|fy\d*)$`)

// comparesAlternatives reports whether a clause asks to compare two
// alternatives ("Compare internal support with outsourcing", "outsourcing
// compared to an internal team"). A comparison against a benchmark, a period
// or a number ("revenue compared with plan", "compared to 2024", "compared to
// $4m") is a metric, not a choice.
func comparesAlternatives(text string) bool {
	for _, re := range []*regexp.Regexp{factCompareVerb, factComparedWith} {
		m := re.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		words := strings.Fields(factLeadingFiller.ReplaceAllString(strings.TrimSpace(m[1]), ""))
		if len(words) == 0 {
			continue
		}
		first := strings.ToLower(strings.Trim(words[0], ",;:.()"))
		r, _ := utf8.DecodeRuneInString(first)
		if unicode.IsDigit(r) || strings.ContainsRune("$€£¥+-−", r) || factBenchmarkWord[first] || factPeriodLabel.MatchString(first) {
			continue
		}
		return true
	}
	return false
}

// isOptionText reports whether a clause weighs alternatives: an option cue
// that is not a benchmark ("vs plan"), or a "compare A with B" request.
func isOptionText(text string) bool {
	return factOption.MatchString(factBenchmark.ReplaceAllString(text, "")) || comparesAlternatives(text)
}

// factRecommend matches a recommendation: the alternative the brief backs
// and, usually, why. It is a fact even without a number or a name, so the
// decision slot carries it (go-slide-creator-fu6uy).
var factRecommend = regexp.MustCompile(`(?i)\brecommend(?:s|ed|ing|ation)?\b`)

// factBenchmark matches a comparison against a benchmark rather than an
// alternative: "vs plan", "versus last year", "vs. target".
var factBenchmark = regexp.MustCompile(`(?i)\b(?:vs\.?|versus|against)\s+(?:the\s+)?(?:plan|target|budget|forecast|guidance|consensus|last|prior|previous|py|ly|ytd|benchmark)\b`)

// factMetricValue matches one unit-bearing value (a percent or a currency
// amount), for counting a clause's data points: three of them in one clause
// ("North 41%, South 33%, West 26%") are a series a chart can show; "$8M over
// 2 years versus $25M" is a comparison of two.
var factMetricValue = regexp.MustCompile(`[$€£¥]\s*\d+(?:[.,]\d+)*|\d+(?:[.,]\d+)*\s*%`)

// factRisk matches a clause that names a risk.
var factRisk = regexp.MustCompile(`(?i)\b(?:risks?|threats?|blockers?|headwinds?)\b`)

// factSource matches the provenance of the brief's figures.
var factSource = regexp.MustCompile(`(?i)^sources?\s*:`)

// factNumberList matches three or more comma-separated values: a series.
var factNumberList = regexp.MustCompile(`\d(?:\.\d+)?\s*%?(?:,\s+(?:and\s+)?[+\-−]?[$€£¥]?\d+(?:\.\d+)?\s*%?){2,}`)

// classifyFact sets a fact's routing flags from its text. quantity reports
// whether the clause carries a standalone number at all.
func classifyFact(text string, quantity bool) briefFact {
	f := briefFact{text: text}
	f.ask = factAsk.MatchString(text) || factDeadline.MatchString(text)
	f.action = f.ask || factAction.MatchString(text)
	f.dated = factDated.MatchString(text)
	// "vs plan" / "vs last year" is a benchmark, not a choice between options.
	f.option = isOptionText(text)
	f.recommend = factRecommend.MatchString(text)
	switch {
	case factMetricUnit.MatchString(factYear.ReplaceAllString(text, "")):
		f.numeric = true
	case quantity && !f.action && !f.dated:
		// A bare count ("14 depots", "1200 people") is a metric unless the
		// clause is a to-do or a milestone.
		f.numeric = true
	}
	// "risk" as a criterion the options are weighed against is not a risk.
	f.risk = !f.option && factRisk.MatchString(text)
	f.source = factSource.MatchString(text)
	if factNumberList.MatchString(text) {
		// "41.0, 42.3, 44.1, 45.9, 48.2" is a series whatever else the clause
		// says.
		f.numeric, f.list = true, true
	}
	f.series = f.numeric && (f.list || factSeries.MatchString(text) || len(factMetricValue.FindAllString(text, -1)) >= 3)
	if f.numeric && !f.ask {
		// A metric that happens to contain a verb ("pipeline will grow 20%")
		// is still a metric, not a to-do.
		f.action = false
	}
	return f
}

// extractBriefFacts turns the brief into facts, in brief order, de-duplicated:
// every clause after the topic is a fact (go-slide-creator-hf8tf). A clause
// used to qualify only when it carried a quantity, a name, an option or a
// recommendation, so "main risk is permitting delay" and "ask is introductions
// to two strategic partners" reached neither a slot nor unplaced_facts. A
// clause with none of those is now kept and marked plain. The brief's first
// clause is the deck topic (it already feeds the opening slide), so it only
// counts as a fact when it carries a quantity.
//
// No clause is truncated: a long clause is split into several facts, and a
// clause that weighs alternatives keeps both of them, so every amount and
// date reaches a slot or unplaced_facts (go-slide-creator-ze5u7).
func extractBriefFacts(brief string) []briefFact {
	var out []briefFact
	seen := make(map[string]bool)
	first := true
	for _, clause := range splitLongClauses(splitClauses(brief)) {
		c := strings.TrimSpace(strings.Trim(clause.text, " \t\"'"))
		c = factListMarker.ReplaceAllString(c, "")
		c = factLeadingConjunction.ReplaceAllString(c, "")
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		isFirst := first
		first = false
		quoted := cueQuote.MatchString(clause.text)

		// A clause cut inside a bracket splits at the open bracket; the
		// bracketed tail is a piece of its own, never dropped.
		for _, piece := range splitOpenBrackets(c) {
			if isFirst && !factQuantity.MatchString(piece) {
				continue // the deck topic
			}
			for _, f := range pieceFacts(piece, quoted) {
				key := strings.ToLower(f.text)
				if seen[key] {
					continue
				}
				seen[key] = true
				f.group, f.header = clause.group, clause.header
				if f.group == groupOption {
					f.option = true
				}
				out = append(out, f)
			}
		}
	}
	return out
}

// splitLongClauses splits a clause longer than maxFactLen at its joining
// words, keeping each piece in the clause's list. A clause that weighs
// alternatives stays whole.
func splitLongClauses(clauses []briefClause) []briefClause {
	var out []briefClause
	for _, c := range clauses {
		if len([]rune(strings.TrimSpace(c.text))) > maxFactLen && !isOptionText(c.text) {
			for _, piece := range splitTopLevel(c.text, factLongClauseSplit) {
				out = append(out, briefClause{text: piece, group: c.group, header: c.header})
			}
			continue
		}
		out = append(out, c)
	}
	return out
}

// pieceFacts classifies one piece of a clause, chunking it when it is longer
// than a fact may be. quoted says the clause quotes someone.
func pieceFacts(piece string, quoted bool) []briefFact {
	if !hasFactContent(piece) {
		return nil
	}
	entity := factAcronym.MatchString(piece) || factProperNoun.MatchString(piece)
	option := factOption.MatchString(piece) || comparesAlternatives(piece)
	plain := !factQuantity.MatchString(piece) && !entity && !quoted && !option && !factRecommend.MatchString(piece)
	limit := maxFactLen
	if isOptionText(piece) {
		limit = maxComparisonFactLen
	}
	var out []briefFact
	for _, text := range chunkFact(piece, limit) {
		if quoted && strings.Count(text, "\"")%2 == 1 {
			// The clause trim took one quote mark of the pair.
			text = strings.ReplaceAll(text, "\"", "")
		}
		f := classifyFact(text, factQuantity.MatchString(text))
		f.quote = quoted
		f.plain = plain
		out = append(out, f)
	}
	return out
}

// hasFactContent reports whether a clause says anything: it holds a letter or
// a digit.
func hasFactContent(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
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
	// The source the brief names belongs to the opening slide, with the
	// topic (see openingSource), not to a content slide.
	var facts []briefFact
	for _, f := range extractBriefFacts(brief) {
		if !f.source {
			facts = append(facts, f)
		}
	}
	unplaced := make([]string, 0)
	if len(facts) == 0 {
		return unplaced
	}

	placedGroup := placePercentSplitFacts(slides, facts, percentSplitFacts(brief))
	placeListGroups(slides, facts, placedGroup)
	placeSeriesLists(slides, facts, placedGroup)

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

// openingSource adds the source the brief names to the opening slide's content
// seed, next to the topic it already carries. Title and closing slides take no
// facts in the raw plan; a source routed as a fact became a content slide
// titled "Source: management accounts" (go-slide-creator-hf8tf).
func openingSource(slides []Slide, brief string) {
	if len(slides) == 0 || slides[0].NarrativeRole != "opening" {
		return
	}
	for _, f := range extractBriefFacts(brief) {
		if f.source {
			slides[0].ContentSeed = strings.TrimRight(slides[0].ContentSeed, ". ") + ". " + sentenceCase(f.text)
		}
	}
}

// roadmapPatterns are the patterns that show a dated sequence.
var roadmapPatterns = map[string]bool{"phase-roadmap": true, "roadmap-phased": true, "timeline-horizontal": true}

// placeListGroups keeps a list the brief itemises under one header together
// on the slide built for it, whatever that slide's capacity: the options
// ("three options to fix margin: a, b, c") on the first comparison slide, the
// phases or milestones on the first roadmap slide. Two of three options on
// another slide, or in unplaced_facts, is no comparison
// (go-slide-creator-hf8tf).
func placeListGroups(slides []Slide, facts []briefFact, placed map[int]bool) {
	homes := map[string]int{}
	for i := range slides {
		if !factEligible(slides[i]) {
			continue
		}
		if _, ok := homes[groupOption]; !ok && slides[i].NarrativeRole == "comparison" {
			homes[groupOption] = i
		}
		if _, ok := homes[groupMilestone]; !ok && roadmapPatterns[slides[i].RecommendedPattern] {
			homes[groupMilestone] = i
		}
	}
	for fi, f := range facts {
		if home, ok := homes[f.group]; ok && !placed[fi] {
			slides[home].Facts = append(slides[home].Facts, f.text)
			placed[fi] = true
		}
	}
}

// placeSeriesLists puts a metric written out as a list of values on the chart
// slide, the one pattern that can draw it as written.
func placeSeriesLists(slides []Slide, facts []briefFact, placed map[int]bool) {
	for fi, f := range facts {
		if !f.list || placed[fi] {
			continue
		}
		for i := range slides {
			if factEligible(slides[i]) && slides[i].RecommendedPattern == "chart-insights-split" && factRoom(slides[i]) > 0 {
				slides[i].Facts = append(slides[i].Facts, f.text)
				placed[fi] = true
				break
			}
		}
	}
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
