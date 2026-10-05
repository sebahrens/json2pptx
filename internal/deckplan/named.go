package deckplan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Kind-vocabulary routing (go-slide-creator-xbwlt).
//
// Four product-only agent journeys discarded plan_deck's deckspec draft: a
// brief that said "margin bridge", "site photo", "pricing schedule", "heat
// map" or "team" was routed through the narrative-role defaults, so the bridge
// became pillars, the photo case a KPI card and the schedule a timeline, while
// the facts those sentences carried — the bridge columns, the options, the
// phases — landed in unplaced_facts.
//
// A brief sentence that names a slide in the kind catalogue's own vocabulary is
// now drafted as that kind BEFORE the defaults run: the sentence is the unit
// (its label before the colon, its list items, its trailing notes), the kind
// comes from a small cue table, the kind's obvious fields are filled from the
// items (bridge columns from "A + x − y = B", KPIs from "metric value" items,
// options from "(A) … (B) … (C)", phases from "Name (Mon–Mon: detail)"), and
// the sentence's facts are claimed so the defaults only draft from what is
// left. An "Ask:" / next-steps sentence is the closer, never a second one.

// namedSlide is one slide the brief names by kind.
type namedSlide struct {
	kind, slot, family, guidance string
	// header is the sentence's label before its first colon; body the rest.
	header, body, sentence string
	start, end             int
	// items are the body's list items; notes the trailing segments that are
	// not items; subs the labelled secondary lists ("parallel tracks": …).
	items []string
	notes []string
	subs  map[string][]string
	// fields are the kind's own fields drafted from the items.
	fields map[string]any
	// facts are the brief facts of the unit, verbatim, with their indices.
	facts   []string
	factIdx []int
	// closer marks an "Ask:" / next-steps sentence, which merges into the
	// closing slot instead of drafting a slide.
	closer bool
	// weak marks a kind inferred from the shape of the items alone (a series
	// with no chart word, three numbers with no KPI word): it ranks below a
	// slide the brief asks for outright.
	weak bool
	// spans are the brief byte ranges the unit was read from: the segments
	// of a sentence it owns, so a fact is claimed only when it lies in one.
	spans [][2]int
}

// owns reports whether a brief offset lies in one of the unit's spans.
func (n *namedSlide) owns(pos int) bool {
	for _, s := range n.spans {
		if pos >= s[0] && pos < s[1] {
			return true
		}
	}
	return false
}

// --- Sentence reading ---

var (
	namedSentenceEnd = regexp.MustCompile(`[.!?]+(?:\s+|$)|\n+`)
	namedAbbrev      = map[string]bool{"vs": true, "e.g": true, "i.e": true, "no": true, "approx": true, "incl": true, "etc": true, "cf": true}
	namedEnumMarker  = regexp.MustCompile(`(?i)^\(?(?:[a-z]|\d{1,2}|option\s+\d|option\s+[a-z])[.):]\s*`)
	namedRecommend   = regexp.MustCompile(`(?i)\brecommend(?:s|ed|ation)?\b`)
	namedTrailingRec = regexp.MustCompile(`(?i)\s+[—–-]\s+((?:we\s+|our\s+)?recommend.*)$`)
	namedLabelDash   = regexp.MustCompile(`\s+[—–]\s+`)
	namedWordChars   = regexp.MustCompile(`[\p{L}\p{N}]+`)
)

// sentenceSpans cuts the brief into sentences at sentence punctuation
// followed by whitespace, and at newlines, outside brackets. An abbreviation
// ("vs.", "e.g.") does not end a sentence.
func sentenceSpans(brief string) [][2]int {
	depths := bracketDepths(brief)
	var out [][2]int
	start := 0
	for _, loc := range namedSentenceEnd.FindAllStringIndex(brief, -1) {
		if loc[0] > 0 && depths[loc[0]-1] > 0 {
			continue
		}
		if strings.HasPrefix(brief[loc[0]:loc[1]], ".") {
			words := strings.Fields(brief[start:loc[0]])
			if len(words) > 0 && namedAbbrev[strings.ToLower(strings.TrimLeft(words[len(words)-1], "("))] {
				continue
			}
		}
		if strings.TrimSpace(brief[start:loc[0]]) != "" {
			out = append(out, [2]int{start, loc[0]})
		}
		start = loc[1]
	}
	if start < len(brief) && strings.TrimSpace(brief[start:]) != "" {
		out = append(out, [2]int{start, len(brief)})
	}
	return out
}

// maxNamedHeaderWords bounds a sentence label: longer is a clause, not a label.
const maxNamedHeaderWords = 14

// splitSentenceLabel splits a sentence at its first top-level colon into the
// label that introduces the list and the body. A filler label ("Facts:") is
// skipped; a label that holds a semicolon or too many words is not a label.
func splitSentenceLabel(sentence string) (header, body string) {
	body = strings.TrimSpace(sentence)
	for range 2 {
		depths := bracketDepths(body)
		cut := -1
		for i := 0; i+1 < len(body); i++ {
			if body[i] == ':' && depths[i] == 0 && body[i+1] == ' ' {
				cut = i
				break
			}
		}
		if cut < 0 {
			return header, body
		}
		label := strings.TrimSpace(body[:cut])
		if strings.ContainsAny(label, ";") || len(strings.Fields(label)) > maxNamedHeaderWords {
			return header, body
		}
		rest := strings.TrimSpace(body[cut+1:])
		if isFillerLabel(label) {
			body = rest
			continue
		}
		// "Results by domain — access management: 14 controls …": the label
		// is the part before the dash; the rest labels the first item.
		if m := namedLabelDash.FindStringIndex(label); m != nil {
			header = strings.TrimSpace(label[:m[0]])
			return header, strings.TrimSpace(label[m[1]:]) + ": " + rest
		}
		return label, rest
	}
	return header, body
}

// topLevelSplit splits s at every top-level occurrence of sep.
func topLevelSplit(s, sep string) []string {
	depths := bracketDepths(s)
	var out []string
	last := 0
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep && depths[i] == 0 {
			out = append(out, s[last:i])
			last = i + len(sep)
		}
	}
	return append(out, s[last:])
}

// splitListItems splits a comma list at its top-level commas, rejoining a
// bare number to the item before it, and splits a final "X (a) and Y (b)"
// pair when both halves carry a parenthesis or a percentage.
func splitListItems(s string) []string {
	var out []string
	for _, raw := range topLevelSplit(s, ", ") {
		item := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), ".;,"))
		if item == "" {
			continue
		}
		if len(out) > 0 && isBareNumber(item) {
			out[len(out)-1] += ", " + item
			continue
		}
		out = append(out, item)
	}
	if n := len(out); n > 0 {
		last := out[n-1]
		depths := bracketDepths(last)
		if i := strings.LastIndex(last, " and "); i > 0 && depths[i] == 0 {
			a, b := strings.TrimSpace(last[:i]), strings.TrimSpace(last[i+5:])
			if (strings.Contains(a, "(") && strings.Contains(b, "(")) || (strings.Contains(a, "%") && strings.Contains(b, "%")) {
				out = append(out[:n-1], a, b)
			}
		}
	}
	for i := range out {
		out[i] = strings.TrimSpace(strings.TrimPrefix(out[i], "and "))
	}
	return out
}

// readSentence builds the named-slide candidate for one sentence: label,
// body, items, notes and labelled sub-lists.
func readSentence(sentence string) *namedSlide {
	n := &namedSlide{sentence: strings.TrimSpace(sentence), subs: map[string][]string{}}
	n.header, n.body = splitSentenceLabel(n.sentence)
	var segs []string
	enumerated, labelled := 0, 0
	for _, raw := range topLevelSplit(n.body, ";") {
		s := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), ".;,"))
		if s == "" {
			continue
		}
		segs = append(segs, s)
		if namedEnumMarker.MatchString(s) {
			enumerated++
		}
		if _, _, ok := labelledList(s); ok {
			labelled++
		}
	}
	switch {
	case enumerated >= 2:
		readEnumerated(n, segs)
	case labelled >= 2:
		// "access management: 14 controls, 3 exceptions, red; change
		// management: …": two or more labelled segments are rows.
		n.items = segs
	default:
		readListSentence(n, segs)
	}
	return n
}

// readEnumerated reads "(A) …; (B) …; (C) … — we recommend B": the marked
// segments are the items, the rest notes.
func readEnumerated(n *namedSlide, segs []string) {
	for _, s := range segs {
		if m := namedTrailingRec.FindStringSubmatchIndex(s); m != nil {
			n.notes = append(n.notes, strings.TrimSpace(s[m[2]:m[3]]))
			s = strings.TrimSpace(s[:m[0]])
		}
		if namedEnumMarker.MatchString(s) {
			n.items = append(n.items, s)
		} else {
			n.notes = append(n.notes, s)
		}
	}
}

// readListSentence reads a comma list in the first segment with the later
// segments as notes or labelled sub-lists; three or more segments with no
// list in the first are themselves the items.
func readListSentence(n *namedSlide, segs []string) {
	if len(segs) == 0 {
		return
	}
	first := segs[0]
	for _, s := range segs[1:] {
		if label, list, ok := labelledList(s); ok {
			n.subs[strings.ToLower(label)] = list
			continue
		}
		n.notes = append(n.notes, s)
	}
	if m := namedTrailingRec.FindStringSubmatchIndex(first); m != nil {
		n.notes = append(n.notes, strings.TrimSpace(first[m[2]:m[3]]))
		first = strings.TrimSpace(first[:m[0]])
	}
	n.items = splitListItems(first)
	if len(n.items) < 2 && len(segs) >= 3 {
		n.items, n.notes, n.subs = segs, nil, map[string][]string{}
	}
}

// labelledList reads "parallel tracks: a, b" as a short label and its items.
func labelledList(seg string) (label string, items []string, ok bool) {
	depths := bracketDepths(seg)
	for i := 0; i+1 < len(seg); i++ {
		if seg[i] == ':' && depths[i] == 0 && seg[i+1] == ' ' {
			label = strings.TrimSpace(seg[:i])
			if !isShortLabel(label) {
				return "", nil, false
			}
			return label, splitListItems(seg[i+1:]), true
		}
	}
	return "", nil, false
}

// --- Kind vocabulary ---

var (
	cueNamedCloser    = regexp.MustCompile(`(?i)^(?:the\s+)?(?:asks?|next[- ]steps?|decisions?\s+(?:requested|needed|sought)|call to action|closer)$`)
	cueNamedAskStart  = regexp.MustCompile(`(?i)^(?:the\s+)?ask\b`)
	cueNamedPricing   = regexp.MustCompile(`(?i)\b(?:pricing|price|fees?|rate card|line items?)\b`)
	cueNamedBridge    = regexp.MustCompile(`(?i)\b(?:bridge|waterfall|walk)\b`)
	cueNamedMatrix    = regexp.MustCompile(`(?i)\b(?:likelihood|probability)\s*(?:[/×x]|and|vs\.?|by|against|versus)\s*impact\b|\bimpact\s*(?:[/×x]|and|vs\.?|by|against|versus)\s*(?:likelihood|probability|effort)\b|\bheat\s?maps?\b|\b2\s?[x×]\s?2\b`)
	cueNamedRiskMap   = regexp.MustCompile(`(?i)\b(?:risks?|likelihood|probability)\b`)
	cueNamedOtherAxis = regexp.MustCompile(`(?i)\b(?:effort|cost|value|urgency|importance|reach|feasibility)\b|\b2\s?[x×]\s?2\b`)
	cueNamedImage     = regexp.MustCompile(`(?i)\b(?:photos?|photographs?|screenshots?|site photos?|case stud(?:y|ies)|customer stor(?:y|ies)|comparables?|reference cases?)\b`)
	cueNamedArch      = regexp.MustCompile(`(?i)\b(?:architecture|tech(?:nology)? stack|platform stack|tiered|tiers)\b`)
	cueNamedTeam      = regexp.MustCompile(`(?i)\b(?:team|our people|who we are|consultants?|engagement team|bios?|headshots?)\b`)
	cueNamedTeamHead  = regexp.MustCompile(`(?i)^(?:the\s+|our\s+)?(?:(?:engagement|project|core|proposed)\s+)?team\b`)
	cueNamedOptions   = regexp.MustCompile(`(?i)\b(?:options?|alternatives?|scenarios?|vendors?)\b`)
	cueNamedOptCount  = regexp.MustCompile(`(?i)\b(?:two|three|four|five|six|\d)[- ](?:\w+[- ])?(?:options?|alternatives?|scenarios?|vendors?)\b`)
	cueNamedCriteria  = regexp.MustCompile(`(?i)\b(?:criteria|criterion|scored|scoring|scores?|evaluat\w+|assess\w+|rated|weighed|matrix)\b`)
	cueNamedRoadmap   = regexp.MustCompile(`(?i)\b(?:roadmap|work ?plan|phases?|phased|parallel tracks?|workstreams?|waves?)\b`)
	cueNamedPlanHead  = regexp.MustCompile(`(?i)\bplan\b`)
	cueNamedProcess   = regexp.MustCompile(`(?i)\b(?:approach|methodology|method|process|steps?|workflow|stages|how it works|journey)\b`)
	cueNamedTable     = regexp.MustCompile(`(?i)\b(?:schedules?|registers?|price lists?|pricing|line items?|rate cards?|fees?|tables?|results by|status board|results? board|scoreboard)\b`)
	// cueNamedTableStrong names a table outright, ahead of a "phase" in its
	// column list reading as a roadmap.
	cueNamedTableStrong = regexp.MustCompile(`(?i)\b(?:pricing|price lists?|line items?|rate cards?|registers?|fee schedules?|fees?)\b`)
	cueNamedRegister  = regexp.MustCompile(`(?i)\b(?:owners?|due dates?|severity|register)\b`)
	cueNamedFindings  = regexp.MustCompile(`(?i)\bfindings?\b`)
	cueNamedRAG       = regexp.MustCompile(`(?i)\b(?:red|amber|green|rag)\b`)
	cueNamedCompare   = regexp.MustCompile(`(?i)\b(?:two columns?|side[- ]by[- ]side|before (?:and|vs\.?|/|→|versus) after|(?:current|today|as-is|now) (?:vs\.?|versus|and|against|/|→|to) (?:the\s+)?(?:target|to-be|future|tomorrow|after|fy\d{2,4})|ladder)\b`)
	cueNamedVsHead    = regexp.MustCompile(`(?i)\b([\p{L}\d-]+)\s+(?:vs\.?|versus)\s+([\p{L}\d-]+)\b`)
	cueNamedArrow     = regexp.MustCompile(`\d[^,;]*?\s(?:→|->|⇒|to)\s(?:under\s+|over\s+|about\s+|below\s+|above\s+)?[+\-−]?\d`)
	cueNamedMaturity  = regexp.MustCompile(`(?i)\bmaturity\b`)
	cueNamedTarget    = regexp.MustCompile(`(?i)\btarget\b`)
	cueNamedKPI       = regexp.MustCompile(`(?i)\b(?:kpis?|metrics|dashboard|scorecard|at a glance|headline (?:numbers|figures)|key (?:numbers|figures)|status board)\b`)
	cueNamedPillars   = regexp.MustCompile(`(?i)\b(?:pillars?|themes)\b`)
	cueNamedSeries    = regexp.MustCompile(`(?i)\byear[- ]on[- ]year\b|\byear[- ]by[- ]year\b|\btrends?\b|\bover time\b|\bchart\b`)
	cueNamedAppendix  = regexp.MustCompile(`(?i)\b(?:appendix|appendices|back-?up)\b`)
)

// Guidance per named kind.
var namedGuidance = map[string]string{
	"bridge":            "The walk the brief gives, as waterfall columns: the opening total, each delta with its sign, the closing total; the title states what moved the number.",
	"matrix_2x2":        "The heat map: the brief's axes and every named item placed in its quadrant (a medium rating sits with high — move it if the brief means otherwise); the title names the quadrant that needs action.",
	"risk_heatmap":      "The heat map: every named risk with its own likelihood and impact (low / medium / high), placed on the grid; the title names the risk that needs action.",
	"image_case":        "The picture beside its story: set image.path (or keep image_label until the file exists), the body, up to 3 result metrics and the callouts the brief asks for; the title states the result.",
	"architecture":      "The stack top to bottom: one tier per layer with its components, the cross-cutting concerns as rails, never as another tier.",
	"team":              "Who delivers: one card per person with name, role and the one line that makes them credible; a headshot via members[].photo.",
	"decision":          "The options the brief enumerates, the recommended one marked, and the recommendation as the band beneath; the title names the choice.",
	"option_matrix":     "The options scored against the brief's criteria (fill the scores), the recommended option highlighted; the title names the winner and why.",
	"roadmap":           "The phases with their dates and what each delivers; parallel tracks are listed in the facts — show them as a last phase row or in the takeaway, the roadmap kind has no track lanes.",
	"process":           "The steps in order, each with its detail where the brief gives one; the title says what the sequence produces.",
	"table":             "One row per item, the brief's columns as headers; rows the brief leaves to you stay __FILL__. Over 7 rows the density rules ask for a split — do not drop rows.",
	"comparison":        "Two columns aligned row by row: the same item on the same row in each column; the title says what changes.",
	"kpi_snapshot":      "The brief's metric: value items as KPI cards (value, label, delta / comparator); the title states the headline as a sentence carrying its number.",
	"chart_insight":     "One chart from the series the brief gives — categories and values as written, never invented — with the insights beside it; the title states the claim with its number.",
	"pillars":           "The named pillars, one panel each with what it covers; the title says what they add up to.",
	"regions":           regionsGuidance,
	"executive_summary": guideAnswer,
	"next_steps":        guideClosing,
}

// namedSlot is the slot name a named kind takes in the storyline.
var namedSlot = map[string]string{
	"bridge": "bridge", "matrix_2x2": "risks", "risk_heatmap": "risks", "image_case": "case", "architecture": "architecture",
	"team": "team", "decision": "options", "option_matrix": "options", "roadmap": "roadmap",
	"process": "plan", "table": "table", "comparison": "comparison", "kpi_snapshot": "context",
	"chart_insight": "evidence", "pillars": "framework", "regions": "regions",
	"executive_summary": "answer", "next_steps": "closing",
}

// namedChapter is the chapter a named kind sits in on a chaptered draft.
var namedChapter = map[string]string{
	"bridge": chapterSituation, "matrix_2x2": chapterSituation, "risk_heatmap": chapterSituation, "image_case": chapterSituation,
	"architecture": chapterSituation, "table": chapterSituation, "comparison": chapterSituation,
	"kpi_snapshot": chapterSituation, "chart_insight": chapterSituation, "pillars": chapterSituation,
	"regions": chapterSituation, "team": chapterPlan, "decision": chapterPlan,
	"option_matrix": chapterOptions, "roadmap": chapterPlan, "process": chapterPlan,
}

// numericItems counts the items that carry a quantity.
func numericItems(items []string) int {
	n := 0
	for _, it := range items {
		if factQuantity.MatchString(it) {
			n++
		}
	}
	return n
}

// firstWords returns the first k words of s.
func firstWords(s string, k int) string {
	words := strings.Fields(s)
	if len(words) > k {
		words = words[:k]
	}
	return strings.Join(words, " ")
}

// namedRule is one entry of the kind vocabulary, tried in order.
type namedRule struct {
	kind  string
	match func(n *namedSlide, head, all string) bool
	// after refines the match: the closer flag, a table family, a weak
	// (inferred) kind, options with criteria.
	after func(n *namedSlide, all string)
}

// A KPI word names the slide only as its label (or its opening words): "we
// shipped a new analytics dashboard" is not a KPI slide.
func kpiCueOf(head, all string) bool {
	return cueNamedKPI.MatchString(head) || (head == "" && cueNamedKPI.MatchString(firstWords(all, 5)))
}

func hasList(n *namedSlide, all string) bool { return len(n.items) >= 2 || countWord(all) > 0 }

var namedRules = []namedRule{
	{kind: "next_steps", match: func(n *namedSlide, head, all string) bool {
		return cueNamedCloser.MatchString(head) || (head == "" && cueNamedAskStart.MatchString(all))
	}, after: func(n *namedSlide, _ string) { n.closer = true }},
	{kind: "bridge", match: func(_ *namedSlide, _, all string) bool { return cueNamedBridge.MatchString(all) }},
	{kind: "matrix_2x2", match: func(_ *namedSlide, _, all string) bool { return cueNamedMatrix.MatchString(all) },
		after: func(n *namedSlide, all string) {
			// Rated risks on likelihood × impact are the risk heat map: a
			// 2x2 has no place for "medium" (go-slide-creator-ec74l).
			if (riskHeatmapFields(n) != nil || cueNamedRiskMap.MatchString(all)) && !cueNamedOtherAxis.MatchString(all) {
				n.kind = "risk_heatmap"
			}
		}},
	{kind: "image_case", match: func(_ *namedSlide, _, all string) bool { return cueNamedImage.MatchString(all) }},
	{kind: "architecture", match: func(_ *namedSlide, _, all string) bool { return cueNamedArch.MatchString(all) }},
	{kind: "team", match: func(n *namedSlide, head, all string) bool {
		return cueNamedTeamHead.MatchString(head) || (head == "" && cueNamedTeamHead.MatchString(all)) || (cueNamedTeam.MatchString(head) && len(n.items) > 0)
	}},
	{kind: "decision", match: func(n *namedSlide, _, all string) bool {
		enumerated := len(n.items) >= 2 && namedEnumMarker.MatchString(n.items[0])
		return cueNamedOptions.MatchString(all) && (enumerated || cueNamedOptCount.MatchString(all))
	}, after: func(n *namedSlide, all string) {
		if cueNamedCriteria.MatchString(all) {
			n.kind = "option_matrix"
		}
	}},
	{kind: "process", match: func(n *namedSlide, head, _ string) bool {
		return cueNamedProcess.MatchString(head) && len(n.items) >= 3 && len(n.items) <= 8
	}},
	{kind: "table", match: func(n *namedSlide, _, all string) bool {
		return cueNamedTableStrong.MatchString(all) && hasList(n, all)
	}, after: func(n *namedSlide, _ string) { n.family = "pricing" }},
	{kind: "roadmap", match: func(n *namedSlide, head, all string) bool {
		return cueNamedRoadmap.MatchString(all) || (cueNamedPlanHead.MatchString(head) && len(n.items) >= 2 && datedItems(n.items) >= 2)
	}},
	{kind: "comparison", match: func(_ *namedSlide, head, all string) bool {
		arrows := len(cueNamedArrow.FindAllString(all, -1)) >= 2 && !factSeries.MatchString(all)
		return arrows || cueNamedCompare.MatchString(all) || (cueNamedMaturity.MatchString(head) && cueNamedTarget.MatchString(all)) || cueNamedVsHead.MatchString(head)
	}},
	{kind: "table", match: func(n *namedSlide, _, all string) bool {
		return (cueNamedTable.MatchString(all) && hasList(n, all)) ||
			(cueNamedFindings.MatchString(all) && cueNamedRegister.MatchString(all)) ||
			len(cueNamedRAG.FindAllString(all, -1)) >= 2
	}, after: func(n *namedSlide, all string) {
		if cueNamedPricing.MatchString(all) {
			n.family = "pricing"
		}
	}},
	{kind: "pillars", match: func(n *namedSlide, _, all string) bool { return cueNamedPillars.MatchString(all) && len(n.items) >= 2 }},
	{kind: "chart_insight", match: func(n *namedSlide, _, _ string) bool { return parseSeries(n) != nil },
		after: func(n *namedSlide, all string) { n.weak = !cueNamedSeries.MatchString(all) }},
	{kind: "kpi_snapshot", match: func(n *namedSlide, head, all string) bool { return kpiCueOf(head, all) && len(n.items) >= 2 }},
	{kind: "kpi_snapshot", match: func(n *namedSlide, _, _ string) bool { return len(n.items) >= 3 && numericItems(n.items) >= 3 },
		after: func(n *namedSlide, _ string) { n.weak = true }},
}

// namedKindFor decides the kind a sentence names, from its label first and
// its whole text second. It reports false when the sentence names none.
func namedKindFor(n *namedSlide) bool {
	head, all := n.header, n.sentence
	n.weak, n.closer, n.family = false, false, ""
	for _, r := range namedRules {
		if !r.match(n, head, all) {
			continue
		}
		n.kind = r.kind
		if r.after != nil {
			r.after(n, all)
		}
		n.slot = namedSlot[n.kind]
		n.guidance = namedGuidance[n.kind]
		return true
	}
	return false
}

// datedItems counts items that carry a date.
func datedItems(items []string) int {
	n := 0
	for _, it := range items {
		if factDated.MatchString(it) || regexp.MustCompile(`(?i)\b(?:week|wave|phase|sprint)\s+\d`).MatchString(it) {
			n++
		}
	}
	return n
}

var namedCountWords = map[string]int{"two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12}

var cueNamedCount = regexp.MustCompile(`(?i)\b(two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|\d{1,2})[- ](?:line items?|rows?|lines?|findings|items|entries|actions|risks)\b`)

// countWord reads "eight line items" / "seven findings" / "8-row".
func countWord(s string) int {
	m := cueNamedCount.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	w := strings.ToLower(m[1])
	if n, ok := namedCountWords[w]; ok {
		return n
	}
	n, _ := strconv.Atoi(w)
	return n
}

// --- Detection ---

// detectNamedSlides reads every sentence after the topic into units — a
// sentence, or the segment groups of a long semicolon-joined one — drafts a
// named slide for each unit that names a kind, merges units of one family
// (fees and the pricing schedule are one table), and claims each unit's
// facts.
func detectNamedSlides(brief string, facts []briefFact) []namedSlide {
	spans := sentenceSpans(brief)
	positions := factPositions(brief, facts)
	var out []namedSlide
	consumed := map[int]bool{}
	claim := func(n *namedSlide) {
		for fi, f := range facts {
			if positions[fi] >= 0 && !f.source && n.owns(positions[fi]) {
				n.facts = append(n.facts, f.text)
				n.factIdx = append(n.factIdx, fi)
			}
		}
	}
	for si, span := range spans {
		if si == 0 || consumed[si] {
			continue // the topic, or a sentence folded into the one before
		}
		units := sentenceUnits(brief, span)
		for ui := range units {
			n := &units[ui]
			// "Recommend option 2." as a sentence of its own belongs to the
			// options before it.
			if (n.kind == "decision" || n.kind == "option_matrix") && ui == len(units)-1 && si+1 < len(spans) {
				next := strings.TrimSpace(brief[spans[si+1][0]:spans[si+1][1]])
				if namedRecommend.MatchString(next) && len(strings.Fields(next)) <= 12 {
					// Its text shapes the options' fields; its fact stays in
					// the pool for the ask slot.
					text := n.sentence + "; " + strings.TrimRight(next, ". ")
					refined := finalizeUnit(text, n.spans)
					if refined != nil {
						*n = *refined
					}
					consumed[si+1] = true
				}
			}
			claim(n)
			n.fields = namedFields(n)
			out = appendNamed(out, *n)
		}
	}
	return out
}

// appendNamed adds a unit to the list, merging it into an earlier unit of the
// same kind and family.
func appendNamed(out []namedSlide, n namedSlide) []namedSlide {
	if n.family != "" {
		for i := range out {
			if out[i].kind == n.kind && out[i].family == n.family {
				mergeNamed(&out[i], &n)
				return out
			}
		}
	}
	return append(out, n)
}

// maxFoldWords is the longest segment that folds into the named unit before
// it as a note ("peer median is about 0.02% of assets", "we recommend B").
const maxFoldWords = 30

// foldable reports whether a segment that names no kind belongs with the
// named unit beside it rather than with the rest of the brief: short, and not
// the next steps, an ask or the source — those have slots of their own.
func foldable(seg string) bool {
	if seg == "" || len(strings.Fields(seg)) > maxFoldWords {
		return false
	}
	if cueNextSteps.MatchString(seg) || factSource.MatchString(seg) || factAsk.MatchString(seg) {
		return false
	}
	return true
}

// sentenceUnits reads one sentence into named units. A sentence that
// enumerates options, or lists rows as labelled segments, is one unit. Any
// other sentence is read segment by segment: a segment that names a kind
// anchors a unit, a short segment after it folds in as a note (a segment of
// the same kind joins it), a leading short segment under the sentence's label
// folds forward, and everything else is left for the storyline to route. A
// sentence with no anchoring segment is tried whole.
func sentenceUnits(brief string, span [2]int) []namedSlide {
	text := brief[span[0]:span[1]]
	whole := func() []namedSlide {
		if n := finalizeUnit(text, [][2]int{span}); n != nil {
			return []namedSlide{*n}
		}
		return nil
	}
	sentence := readSegments(brief, span)
	if sentence == nil || sentence.enumerated >= 2 {
		return whole()
	}
	anchor, strong := sentence.anchors()
	if (strong < 2 && sentence.rows >= 2) || len(anchor) == 0 {
		return whole() // rows: "access management: 14 controls, …; change management: …"
	}
	var out []namedSlide
	for _, u := range sentence.group(anchor) {
		if n := finalizeUnit(strings.Join(u.texts, "; "), u.spans); n != nil {
			out = append(out, *n)
		}
	}
	return out
}

// sentenceSegments is a sentence cut into its top-level ";" segments, with
// the label, each segment's span, and the counts the unit rules read.
type sentenceSegments struct {
	header     string
	segs       []string
	spans      [][2]int
	enumerated int // segments with an enumeration marker
	rows       int // segments shaped "short label: cell, cell"
}

// readSegments cuts the sentence at span into segments, or returns nil when
// it has fewer than two.
func readSegments(brief string, span [2]int) *sentenceSegments {
	text := brief[span[0]:span[1]]
	header, body := splitSentenceLabel(text)
	bodyAt := strings.LastIndex(text, body)
	if bodyAt < 0 || body == "" {
		return nil
	}
	s := &sentenceSegments{header: header, spans: segmentSpans(body, span[0]+bodyAt)}
	if len(s.spans) < 2 {
		return nil
	}
	s.segs = make([]string, len(s.spans))
	for i, sp := range s.spans {
		s.segs[i] = strings.TrimSpace(strings.Trim(strings.TrimSpace(brief[sp[0]:sp[1]]), ".;,"))
		if namedEnumMarker.MatchString(s.segs[i]) {
			s.enumerated++
		}
		// A row: a short label over two or more cells ("change management:
		// 11 controls, 1 exception, amber"); "recommendation: renegotiate
		// now" is a labelled note, not a row.
		if label, cells, ok := labelledList(s.segs[i]); ok && len(strings.Fields(label)) <= 3 && len(cells) >= 2 {
			s.rows++
		}
	}
	return s
}

// withLabel is segment i with the sentence's label on the first.
func (s *sentenceSegments) withLabel(i int) string {
	if i == 0 && s.header != "" {
		return s.header + ": " + s.segs[0]
	}
	return s.segs[i]
}

// anchors finds the segments that name a kind on their own, keyed by index,
// and counts the distinct strong (not inferred) kinds among them.
func (s *sentenceSegments) anchors() (map[int]*namedSlide, int) {
	anchor := map[int]*namedSlide{}
	strong := map[string]bool{}
	for i := range s.segs {
		n := readSentence(s.withLabel(i))
		if namedKindFor(n) {
			anchor[i] = n
			if !n.weak {
				strong[n.kind] = true
			}
		}
	}
	return anchor, len(strong)
}

// unitGroup is the text and spans of one named unit of a sentence.
type unitGroup struct {
	texts []string
	spans [][2]int
	kind  string
}

// group builds the units: an anchor starts one (a following anchor of the
// same kind joins it), a short later segment folds in as a note, a short
// leading segment under the label folds forward into the first anchor.
func (s *sentenceSegments) group(anchor map[int]*namedSlide) []*unitGroup {
	var units []*unitGroup
	var pending []int // leading segments waiting for the first anchor
	for i := range s.segs {
		a := anchor[i]
		switch {
		case a != nil:
			if n := len(units); n > 0 && units[n-1].kind == a.kind && i > 0 && anchor[i-1] != nil {
				units[n-1].texts = append(units[n-1].texts, s.segs[i])
				units[n-1].spans = append(units[n-1].spans, s.spans[i])
				continue
			}
			u := &unitGroup{kind: a.kind}
			for _, p := range pending {
				u.texts = append(u.texts, s.segs[p])
				u.spans = append(u.spans, s.spans[p])
			}
			pending = nil
			u.texts = append(u.texts, s.withLabel(i))
			u.spans = append(u.spans, s.spans[i])
			units = append(units, u)
		case len(units) > 0 && foldable(s.segs[i]):
			// A recommendation lends the options its text (the recommended
			// option, the recommendation band) but stays a fact of its own:
			// the storyline routes it to the ask.
			u := units[len(units)-1]
			u.texts = append(u.texts, s.segs[i])
			if !namedRecommend.MatchString(s.segs[i]) {
				u.spans = append(u.spans, s.spans[i])
			}
		case len(units) == 0 && s.header != "" && foldable(s.segs[i]):
			pending = append(pending, i)
		}
	}
	return units
}

// segmentSpans returns the absolute spans of a body's top-level ";" segments.
func segmentSpans(body string, offset int) [][2]int {
	depths := bracketDepths(body)
	var out [][2]int
	last := 0
	for i := 0; i < len(body); i++ {
		if body[i] == ';' && depths[i] == 0 {
			out = append(out, [2]int{offset + last, offset + i})
			last = i + 1
		}
	}
	return append(out, [2]int{offset + last, offset + len(body)})
}

// finalizeUnit reads a unit's text as a named slide with the given spans, or
// returns nil when it names no kind.
func finalizeUnit(text string, spans [][2]int) *namedSlide {
	n := readSentence(text)
	n.spans = spans
	if len(spans) > 0 {
		n.start, n.end = spans[0][0], spans[len(spans)-1][1]
	}
	if !namedKindFor(n) {
		return nil
	}
	return n
}

// mergeNamed folds b into a: facts, items and spans join, fields come from
// whichever has the richer ones.
func mergeNamed(a, b *namedSlide) {
	a.facts = append(a.facts, b.facts...)
	a.factIdx = append(a.factIdx, b.factIdx...)
	a.items = append(a.items, b.items...)
	a.spans = append(a.spans, b.spans...)
	a.sentence += " " + b.sentence
	a.end = b.end
	if len(b.fields) > len(a.fields) {
		a.fields = b.fields
	}
}

// factPositions finds each fact's byte offset in the brief by searching for
// its text (or, for a label-joined fact, its clause), in brief order. A fact
// the search cannot place maps to -1.
func factPositions(brief string, facts []briefFact) []int {
	owner := make([]int, len(facts))
	cursor := 0
	for i, f := range facts {
		owner[i] = -1
		needles := []string{f.text}
		if k := strings.Index(f.text, ": "); k > 0 {
			needles = append(needles, f.text[k+2:])
		}
		if r := []rune(f.text); len(r) > 24 {
			needles = append(needles, string(r[:24]))
		}
		pos := -1
		for _, needle := range needles {
			if p := strings.Index(brief[cursor:], needle); p >= 0 {
				pos = cursor + p
				break
			}
			if p := strings.Index(brief, needle); p >= 0 {
				pos = p
				break
			}
		}
		if pos < 0 {
			continue
		}
		owner[i] = pos
		if pos >= cursor {
			cursor = pos
		}
	}
	return owner
}

// --- Field drafting ---

// namedFields drafts the kind's own fields from the sentence.
func namedFields(n *namedSlide) map[string]any {
	switch n.kind {
	case "bridge":
		return bridgeFields(n)
	case "kpi_snapshot":
		return kpiFields(n)
	case "decision":
		return decisionFields(n)
	case "option_matrix":
		return optionMatrixFields(n)
	case "roadmap":
		return roadmapFields(n)
	case "team":
		return teamFields(n)
	case "matrix_2x2":
		return matrixFields(n)
	case "risk_heatmap":
		return riskHeatmapFields(n)
	case "architecture":
		return architectureFields(n)
	case "process":
		return processFields(n)
	case "table":
		return tableFields(n)
	case "comparison":
		return comparisonFields(n)
	case "chart_insight":
		return chartFields(n)
	case "image_case":
		return imageCaseFields(n)
	case "pillars":
		return pillarsFields(n)
	case "next_steps":
		if n.body != "" {
			return map[string]any{"decisions": []any{sentenceCase(strings.TrimSpace(n.body))}}
		}
	}
	return nil
}

var (
	namedSignedNumber = regexp.MustCompile(`([+\-−]?\d[\d,]*(?:\.\d+)?)\s*$`)
	namedYear         = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
	namedParen        = regexp.MustCompile(`^(.*?)\s*\(([^()]*)\)\s*$`)
)

// parseNumber reads a signed number written with commas and the unicode minus.
func parseNumber(s string) (float64, bool) {
	s = strings.NewReplacer("−", "-", ",", "", "+", "", " ", "").Replace(strings.TrimSpace(s))
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// bridgeFields reads "2023 EBITDA 24.0, price +6.5, volume +3.2, raw material
// −4.1, opex +1.4 = 31.0" as waterfall columns.
func bridgeFields(n *namedSlide) map[string]any {
	type col struct {
		label string
		value float64
	}
	var cols []col
	end, endOK := 0.0, false
	for _, item := range n.items {
		if k := strings.Index(item, "="); k >= 0 {
			if v, ok := parseNumber(item[k+1:]); ok {
				end, endOK = v, true
			}
			item = strings.TrimSpace(item[:k])
		}
		m := namedSignedNumber.FindStringSubmatchIndex(item)
		if m == nil {
			continue
		}
		v, ok := parseNumber(item[m[2]:m[3]])
		if !ok {
			continue
		}
		cols = append(cols, col{strings.TrimSpace(item[:m[2]]), v})
	}
	if len(cols) < 2 && !endOK {
		return nil
	}
	out := make([]any, 0, len(cols)+1)
	for i, c := range cols {
		typ := "delta"
		if i == 0 {
			typ = "total"
		}
		out = append(out, map[string]any{"label": c.label, "type": typ, "value": c.value})
	}
	if endOK {
		label := patterns.FillPlaceholder
		years := namedYear.FindAllString(n.header, -1)
		if len(years) >= 2 && len(cols) > 0 {
			label = strings.Join(strings.Fields(years[len(years)-1]+" "+namedYear.ReplaceAllString(cols[0].label, "")), " ")
		}
		out = append(out, map[string]any{"label": label, "type": "total", "value": end})
	}
	fields := map[string]any{"columns": out}
	if n.header != "" {
		fields["caption"] = n.header
	}
	return fields
}

var (
	namedMetric = regexp.MustCompile(`(?i)(?:(?:EUR|USD|GBP|CHF)\s?|[$€£¥]\s?)?[+\-−±]?\d[\d,]*(?:\.\d+)?(?:[\s-]?(?:bn|mm|m|k|pts?|points?|bps|x|FTE|h|hours?|days?|weeks?|months?|years?)\b|\s?%)?`)
	namedOrdinal = regexp.MustCompile(`^\d+(?:st|nd|rd|th)\b`)
	namedStatus  = regexp.MustCompile(`(?i)\b(within(?: appetite| limit| tolerance)?|amber|breached|red|green|on track|off track|at risk)\b`)
	namedLinkLead  = regexp.MustCompile(`(?i)^(?:is|was|are|were|at|of|to|takes|took|has|have|with|and|about|the|a|an)\s+`)
	namedLinkTrail = regexp.MustCompile(`(?i)\s+(?:is|was|are|were|at|of|to|takes|took|has|have|with|and|about|the|a|an)$`)
)

// firstMetric finds the first metric token in s that is not an ordinal, a
// clock time or a year standing alone.
func firstMetric(s string) []int {
	for _, m := range namedMetric.FindAllStringIndex(s, -1) {
		tok := s[m[0]:m[1]]
		if namedOrdinal.MatchString(s[m[0]:]) {
			continue
		}
		rest := strings.TrimSpace(s[m[1]:])
		if strings.HasPrefix(strings.ToLower(rest), "am") || strings.HasPrefix(strings.ToLower(rest), "pm") {
			continue
		}
		if namedYear.MatchString(tok) && len(strings.TrimSpace(tok)) == 4 {
			continue
		}
		return m
	}
	return nil
}

// trimLabel strips linking words and dashes from both ends of a label.
func trimLabel(s string) string {
	for {
		next := strings.Trim(s, " \t—–:-,.")
		next = namedLinkLead.ReplaceAllString(next, "")
		next = namedLinkTrail.ReplaceAllString(next, "")
		next = strings.Trim(next, " \t—–:-,.")
		if next == s {
			return s
		}
		s = next
	}
}

// parseKPI reads one "metric value" item as a KPI card.
func parseKPI(item string) map[string]any {
	fill := patterns.FillPlaceholder
	paren := ""
	core := item
	if m := namedParen.FindStringSubmatch(item); m != nil {
		core, paren = strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
	}
	if sm := namedStatus.FindStringSubmatchIndex(core); sm != nil {
		kpi := map[string]any{"label": trimLabel(core[:sm[0]]), "value": core[sm[2]:sm[3]]}
		if paren != "" {
			kpi["comparator"] = paren
		}
		if kpi["label"] == "" {
			kpi["label"] = fill
		}
		return kpi
	}
	loc := firstMetric(core)
	if loc == nil {
		if loc = firstMetric(paren); loc == nil {
			return map[string]any{"value": fill, "label": item}
		}
		label := trimLabel(core)
		if rest := trimLabel(paren[:loc[0]] + " " + paren[loc[1]:]); rest != "" {
			label += ", " + rest
		}
		return map[string]any{"value": strings.TrimSpace(paren[loc[0]:loc[1]]), "label": label}
	}
	label := trimLabel(strings.TrimSpace(trimLabel(core[:loc[0]]) + " " + trimLabel(core[loc[1]:])))
	if label == "" {
		label = fill
	}
	kpi := map[string]any{"value": strings.TrimSpace(core[loc[0]:loc[1]]), "label": label}
	if paren != "" {
		kpi["comparator"] = paren
	}
	return kpi
}

func kpiFields(n *namedSlide) map[string]any {
	kpis := make([]any, 0, len(n.items))
	for _, it := range n.items {
		kpis = append(kpis, parseKPI(it))
	}
	if len(kpis) == 0 {
		return nil
	}
	return map[string]any{"kpis": kpis}
}

// recommendedToken reads the option a recommendation names ("we recommend
// B", "Recommend option 2", "recommend Databricks").
func recommendedToken(notes []string) (token, note string) {
	re := regexp.MustCompile(`(?i)\brecommend(?:s|ed|ation)?\s+(?:is\s+)?(?:option\s+|scope\s+|scenario\s+|alternative\s+)?([\p{L}\d][\p{L}\d-]*)`)
	for _, s := range notes {
		if m := re.FindStringSubmatch(s); m != nil {
			return strings.ToLower(m[1]), s
		}
	}
	return "", ""
}

// enumeratedOption splits "(B) market model plus 25 customer interviews, EUR
// 320k, 4 weeks" into marker, label and detail.
func enumeratedOption(item string) (marker, label, detail string) {
	m := namedEnumMarker.FindStringIndex(item)
	if m != nil {
		marker = strings.ToLower(strings.Trim(item[:m[1]], "() .:"))
		marker = strings.TrimPrefix(marker, "option ")
		item = strings.TrimSpace(item[m[1]:])
	}
	parts := topLevelSplit(item, ", ")
	label = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		detail = strings.TrimSpace(strings.Join(parts[1:], ", "))
	}
	return marker, label, detail
}

func decisionFields(n *namedSlide) map[string]any {
	token, note := recommendedToken(n.notes)
	options := make([]any, 0, len(n.items))
	for _, it := range n.items {
		marker, label, detail := enumeratedOption(it)
		opt := map[string]any{"label": label}
		if marker != "" {
			opt["label"] = strings.ToUpper(marker) + ": " + label
		}
		if detail != "" {
			opt["detail"] = detail
		}
		if token != "" && (token == marker || strings.Contains(strings.ToLower(label), token)) {
			opt["recommended"] = true
		}
		options = append(options, opt)
	}
	if len(options) == 0 {
		return nil
	}
	fields := map[string]any{"options": options}
	if note != "" {
		fields["recommendation"] = sentenceCase(note)
	}
	return fields
}

var namedCriteria = regexp.MustCompile(`(?i)\b(?:scored|assessed|evaluated|rated|compared|weighed|measured)\s+(?:on|against|by)\s+(.+)$|\bcriteria:?\s+(.+)$|\bagainst\s+(.+)$`)

// splitCriteria reads "cost, migration risk, skills availability, lock-in"
// (also "cost, time-to-market, and risk") as criteria.
func splitCriteria(s string) []any {
	var out []any
	for _, piece := range regexp.MustCompile(`\s*,\s*|\s+and\s+`).Split(strings.TrimSpace(s), -1) {
		piece = strings.TrimSpace(strings.Trim(piece, ".;:()"))
		if piece != "" {
			out = append(out, piece)
		}
	}
	return out
}

func optionMatrixFields(n *namedSlide) map[string]any {
	fields := map[string]any{}
	for _, src := range []string{n.header, n.sentence} {
		if m := namedCriteria.FindStringSubmatch(src); m != nil {
			if c := splitCriteria(m[1] + m[2] + m[3]); len(c) >= 2 {
				fields["criteria"] = c
				break
			}
		}
	}
	token, note := recommendedToken(n.notes)
	options := make([]any, 0, len(n.items))
	for _, it := range n.items {
		marker, label, detail := enumeratedOption(it)
		opt := map[string]any{"name": label}
		if detail != "" {
			opt["detail"] = detail
		}
		if token != "" && (token == marker || strings.Contains(strings.ToLower(label), token)) {
			fields["recommended"] = label
		}
		options = append(options, opt)
	}
	if len(options) > 0 {
		fields["options"] = options
	}
	if note != "" {
		fields["takeaway"] = sentenceCase(note)
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

var (
	namedPhaseParen = regexp.MustCompile(`^(.+?)\s*\(([^():]+?)(?::\s*([^()]*))?\)$`)
	namedDatePrefix = regexp.MustCompile(`(?i)^((?:week|wave|phase|sprint|month)\s+\d+|q[1-4](?:\s*[–-]\s*q[1-4])?\s+\d{4}|h[12]\s+\d{4}|fy\s?\d{2,4}|(?:19|20)\d{2})\b[:\s]*`)
)

func roadmapFields(n *namedSlide) map[string]any {
	phases := make([]any, 0, len(n.items))
	for _, it := range n.items {
		phase := map[string]any{}
		switch {
		case namedPhaseParen.MatchString(it):
			m := namedPhaseParen.FindStringSubmatch(it)
			phase["name"] = strings.TrimSpace(m[1])
			phase["date_label"] = strings.TrimSpace(m[2])
			if d := strings.TrimSpace(m[3]); d != "" {
				phase["description"] = d
			}
		case namedDatePrefix.MatchString(it):
			m := namedDatePrefix.FindStringSubmatchIndex(it)
			phase["date_label"] = strings.TrimSpace(it[m[2]:m[3]])
			rest := strings.TrimSpace(it[m[1]:])
			if pm := namedParen.FindStringSubmatch(rest); pm != nil && strings.TrimSpace(pm[1]) != "" {
				rest, phase["description"] = strings.TrimSpace(pm[1]), strings.TrimSpace(pm[2])
			}
			phase["name"] = rest
		default:
			phase["name"] = it
		}
		phases = append(phases, phase)
	}
	if len(phases) == 0 {
		return nil
	}
	fields := map[string]any{"phases": phases}
	for label, list := range n.subs {
		if strings.Contains(label, "track") || strings.Contains(label, "workstream") {
			n.guidance += fmt.Sprintf(" Parallel tracks named by the brief: %s.", strings.Join(list, "; "))
		}
	}
	return fields
}

var (
	namedPerson     = regexp.MustCompile(`^([A-Z][\p{L}'’.-]+(?:\s+[A-Z][\p{L}'’.-]+)+)\s*\(([^,)]+)(?:,\s*([^)]*))?\)$`)
	namedRoleCount  = regexp.MustCompile(`(?i)^(two|three|four|five|six|\d+)\s+([a-z][a-z -]*?)s?$`)
	namedPersonBare = regexp.MustCompile(`^[A-Z][\p{L}'’.-]+(?:\s+[A-Z][\p{L}'’.-]+)+$`)
)

func teamFields(n *namedSlide) map[string]any {
	fill := patterns.FillPlaceholder
	var members []any
	for _, it := range n.items {
		switch {
		case namedPerson.MatchString(it):
			m := namedPerson.FindStringSubmatch(it)
			member := map[string]any{"name": m[1], "role": strings.TrimSpace(m[2])}
			if bio := strings.TrimSpace(m[3]); bio != "" {
				member["bio"] = bio
			}
			members = append(members, member)
		case namedRoleCount.MatchString(it):
			m := namedRoleCount.FindStringSubmatch(it)
			count := namedCountWords[strings.ToLower(m[1])]
			if count == 0 {
				count, _ = strconv.Atoi(m[1])
			}
			for i := 0; i < count && i < 8; i++ {
				members = append(members, map[string]any{"name": fill, "role": strings.TrimSpace(m[2])})
			}
		case namedPersonBare.MatchString(it):
			members = append(members, map[string]any{"name": it, "role": fill})
		default:
			members = append(members, map[string]any{"name": fill, "role": it})
		}
	}
	if len(members) == 0 {
		return nil
	}
	return map[string]any{"members": members}
}

var (
	namedAxes   = regexp.MustCompile(`(?i)\b(likelihood|probability|impact|effort|cost|value|urgency|importance|reach|feasibility)\s*(?:[/×x]|and|vs\.?|by|against|versus)\s*(likelihood|probability|impact|effort|cost|value|urgency|importance|reach|feasibility)\b`)
	namedRating = regexp.MustCompile(`(?i)^(.+?)\s*\((low|medium|med|high)\s*/\s*(low|medium|med|high)\)$`)
)

// riskHeatmapFields drafts a risk_heatmap's items from "name (rating/rating)"
// entries. The ratings are read in the order the sentence names its axes —
// likelihood first unless it says "impact / likelihood".
func riskHeatmapFields(n *namedSlide) map[string]any {
	impactFirst := false
	if m := namedAxes.FindStringSubmatch(n.sentence); m != nil {
		impactFirst = strings.EqualFold(m[1], "impact")
	}
	level := func(s string) string {
		if s = strings.ToLower(s); s == "med" {
			return "medium"
		}
		return s
	}
	var items []any
	for _, it := range n.items {
		m := namedRating.FindStringSubmatch(it)
		if m == nil {
			continue
		}
		likelihood, impact := level(m[2]), level(m[3])
		if impactFirst {
			likelihood, impact = impact, likelihood
		}
		items = append(items, map[string]any{"name": strings.TrimSpace(m[1]), "likelihood": likelihood, "impact": impact})
	}
	if len(items) == 0 {
		return nil
	}
	return map[string]any{"items": items}
}

func matrixFields(n *namedSlide) map[string]any {
	x, y := "Likelihood", "Impact"
	if m := namedAxes.FindStringSubmatch(n.sentence); m != nil {
		x, y = sentenceCase(strings.ToLower(m[1])), sentenceCase(strings.ToLower(m[2]))
	}
	bodies := map[string][]string{}
	for _, it := range n.items {
		m := namedRating.FindStringSubmatch(it)
		if m == nil {
			continue
		}
		right := !strings.EqualFold(m[2], "low")
		top := !strings.EqualFold(m[3], "low")
		pos := "bottom_left"
		switch {
		case top && right:
			pos = "top_right"
		case top:
			pos = "top_left"
		case right:
			pos = "bottom_right"
		}
		bodies[pos] = append(bodies[pos], it)
	}
	if len(bodies) == 0 {
		return nil
	}
	headers := map[string]string{
		"top_left": "High " + strings.ToLower(y) + ", low " + strings.ToLower(x), "top_right": "High " + strings.ToLower(y) + ", high " + strings.ToLower(x),
		"bottom_right": "Low " + strings.ToLower(y) + ", high " + strings.ToLower(x), "bottom_left": "Low " + strings.ToLower(y) + ", low " + strings.ToLower(x),
	}
	quads := make([]any, 0, 4)
	for _, pos := range []string{"top_left", "top_right", "bottom_right", "bottom_left"} {
		body := strings.Join(bodies[pos], ", ")
		if body == "" {
			body = "—"
		}
		quads = append(quads, map[string]any{"header": headers[pos], "body": body})
	}
	return map[string]any{
		"x_axis": x, "y_axis": y, "x_low": "Low", "x_high": "High", "y_low": "Low", "y_high": "High",
		"quadrants": quads,
	}
}

var namedRails = regexp.MustCompile(`(?i)\bwith\s+(.+?)\s+as\s+(?:the\s+)?(?:two\s+)?cross-cutting\b`)

func architectureFields(n *namedSlide) map[string]any {
	var tiers, rails []any
	for _, it := range append(append([]string{}, n.items...), n.notes...) {
		if m := namedRails.FindStringSubmatch(it); m != nil {
			for _, r := range regexp.MustCompile(`\s*,\s*|\s+and\s+`).Split(m[1], -1) {
				if r = strings.TrimSpace(r); r != "" {
					rails = append(rails, r)
				}
			}
			continue
		}
		if m := namedParen.FindStringSubmatch(it); m != nil && strings.TrimSpace(m[1]) != "" {
			items := make([]any, 0)
			for _, c := range splitListItems(m[2]) {
				items = append(items, c)
			}
			tiers = append(tiers, map[string]any{"label": strings.TrimSpace(m[1]), "items": items})
			continue
		}
		if strings.Contains(strings.ToLower(it), "cross-cutting") {
			continue
		}
		tiers = append(tiers, map[string]any{"label": it})
	}
	if len(tiers) == 0 {
		return nil
	}
	fields := map[string]any{"tiers": tiers}
	if len(rails) > 0 {
		fields["rails"] = rails
	}
	return fields
}

func processFields(n *namedSlide) map[string]any {
	steps := make([]any, 0, len(n.items))
	for _, it := range n.items {
		if m := namedParen.FindStringSubmatch(it); m != nil && strings.TrimSpace(m[1]) != "" {
			steps = append(steps, map[string]any{"label": strings.TrimSpace(m[1]), "description": strings.TrimSpace(m[2])})
			continue
		}
		steps = append(steps, map[string]any{"label": it})
	}
	if len(steps) == 0 {
		return nil
	}
	return map[string]any{"steps": steps}
}

func pillarsFields(n *namedSlide) map[string]any {
	if len(n.items) < 2 {
		return nil
	}
	pillars := make([]any, 0, len(n.items))
	for _, it := range n.items {
		pillars = append(pillars, map[string]any{"title": it})
	}
	return map[string]any{"pillars": pillars}
}

var (
	namedHeaderParen = regexp.MustCompile(`\(([a-z][a-z -]*(?:,\s*[a-z][a-z -]*){2,})\)`)
	namedEachWith    = regexp.MustCompile(`(?i)\beach with\s+(.+?)(?:;|$)`)
	namedRowNoun     = regexp.MustCompile(`(?i)\b(?:two|three|four|five|six|seven|eight|nine|ten|\d{1,2})[- ](\w+?)s?\b`)
	namedCellCount   = regexp.MustCompile(`^(\d[\d.,]*)\s+([\p{L}][\p{L} -]*)$`)
	namedCellRAG     = regexp.MustCompile(`(?i)^(?:(\w+)\s+)?(red|amber|green)$`)
	namedQuoted      = regexp.MustCompile(`["“']([^"”']{10,})["”']`)
)

// tableFields drafts headers and rows: the brief's column list, the counted
// placeholder rows it leaves to the author, or the "label: cells" items it
// gives (results by domain).
func tableFields(n *namedSlide) map[string]any {
	headers := tableHeaders(n.sentence)
	if rows := tableRowsFromItems(n); len(rows) >= 2 {
		if headers == nil {
			headers = tableHeadersFromRow(n.header, rows[0])
		}
		out := make([]any, 0, len(rows))
		for _, r := range rows {
			cells := []any{r.label}
			for _, c := range r.cells {
				cells = append(cells, tableCellValue(c))
			}
			out = append(out, cells)
		}
		return map[string]any{"headers": headers, "rows": out}
	}
	count := countWord(n.sentence)
	if count == 0 && headers == nil {
		return nil
	}
	if headers == nil {
		headers = []any{patterns.FillPlaceholder, patterns.FillPlaceholder}
	}
	return map[string]any{"headers": headers, "rows": tablePlaceholderRows(n.sentence, headers, max(count, 1))}
}

var (
	namedParenAny   = regexp.MustCompile(`\([^)]*\)`)
	namedBetween    = regexp.MustCompile(`(?i)\s+between\b.*$`)
	namedListSep    = regexp.MustCompile(`\s*,\s*|\s+and\s+`)
	namedArticle    = regexp.MustCompile(`(?i)^(?:a|an|the)\s+`)
	namedByNoun     = regexp.MustCompile(`(?i)\bby\s+(\w+)`)
	namedHighFinder = "high findings"
)

// tableHeaders reads the brief's column list: "(phase, deliverable, days,
// fee)" or "each with a severity, an owner and a due date".
func tableHeaders(sentence string) []any {
	if m := namedHeaderParen.FindStringSubmatch(sentence); m != nil {
		var headers []any
		for _, h := range splitListItems(m[1]) {
			headers = append(headers, h)
		}
		return headers
	}
	m := namedEachWith.FindStringSubmatch(sentence)
	if m == nil {
		return nil
	}
	cols := namedBetween.ReplaceAllString(namedParenAny.ReplaceAllString(m[1], ""), "")
	first := "item"
	if rm := namedRowNoun.FindStringSubmatch(sentence); rm != nil {
		first = strings.ToLower(rm[1])
	}
	headers := []any{first}
	for _, c := range namedListSep.Split(cols, -1) {
		if c = strings.TrimSpace(namedArticle.ReplaceAllString(strings.TrimSpace(c), "")); c != "" {
			headers = append(headers, c)
		}
	}
	return headers
}

// tableRow is one "label: cell, cell" item.
type tableRow struct {
	label string
	cells []string
}

// tableRowsFromItems reads the items shaped "label: cell, cell" (or "label
// 14 controls, 3 exceptions, red").
func tableRowsFromItems(n *namedSlide) []tableRow {
	var rows []tableRow
	for _, it := range n.items {
		label, cells := "", it
		if k := strings.Index(it, ": "); k > 0 {
			label, cells = it[:k], it[k+2:]
		}
		parts := splitListItems(cells)
		if label == "" && len(parts) > 0 {
			if m := firstMetric(parts[0]); m != nil && m[0] > 0 {
				label, parts[0] = trimLabel(parts[0][:m[0]]), strings.TrimSpace(parts[0][m[0]:])
			}
		}
		if label != "" && len(parts) >= 2 {
			rows = append(rows, tableRow{label, parts})
		}
	}
	return rows
}

// tableHeadersFromRow derives headers from the first row's cell shapes: "14
// controls" → Controls, "rating red" → Rating; the label column is named
// after "by <noun>" in the sentence label.
func tableHeadersFromRow(header string, first tableRow) []any {
	label := patterns.FillPlaceholder
	if m := namedByNoun.FindStringSubmatch(header); m != nil {
		label = sentenceCase(strings.ToLower(m[1]))
	}
	headers := []any{label}
	for _, c := range first.cells {
		switch {
		case namedCellCount.MatchString(c):
			headers = append(headers, sentenceCase(namedCellCount.FindStringSubmatch(c)[2]))
		case namedCellRAG.MatchString(c):
			h := namedCellRAG.FindStringSubmatch(c)[1]
			if h == "" {
				h = "rating"
			}
			headers = append(headers, sentenceCase(strings.ToLower(h)))
		default:
			headers = append(headers, patterns.FillPlaceholder)
		}
	}
	return headers
}

// tableCellValue is the value a cell carries: the number of "14 controls",
// the colour of "rating red", else the cell as written.
func tableCellValue(c string) any {
	switch {
	case namedCellCount.MatchString(c):
		return namedCellCount.FindStringSubmatch(c)[1]
	case namedCellRAG.MatchString(c):
		return strings.ToLower(namedCellRAG.FindStringSubmatch(c)[2])
	}
	return c
}

// tablePlaceholderRows drafts the counted rows the brief leaves to the
// author ("eight line items", "seven findings"), the named ones (quoted
// finding titles) first.
func tablePlaceholderRows(sentence string, headers []any, count int) []any {
	fill := patterns.FillPlaceholder
	named := namedQuoted.FindAllStringSubmatch(sentence, -1)
	highNamed := strings.Contains(strings.ToLower(sentence), namedHighFinder)
	out := make([]any, 0, count)
	for i := 0; i < count && i < 12; i++ {
		cells := make([]any, len(headers))
		for j := range cells {
			cells[j] = fill
		}
		if i < len(named) {
			cells[0] = named[i][1]
			for j, h := range headers {
				if highNamed && strings.EqualFold(fmt.Sprint(h), "severity") {
					cells[j] = "high"
				}
			}
		}
		out = append(out, cells)
	}
	return out
}

var (
	// namedArrowSplit captures the arrow between a before and an after value;
	// the split point is the end of group 1 and the start of group 2.
	namedArrowSplit = regexp.MustCompile(`(\s(?:→|->|⇒|to)\s)((?:under\s+|over\s+|about\s+|below\s+|above\s+)?[+\-−]?\d)`)
	namedTargetEvery = regexp.MustCompile(`(?i)\btarget\s+(\d+(?:\.\d+)?)\s+(?:everywhere|across the board|for all|in every domain)(?:\s+by\s+(\w+))?`)
)

func comparisonFields(n *namedSlide) map[string]any {
	left, right := "Today", "Target"
	lower := strings.ToLower(n.sentence)
	switch {
	case strings.Contains(lower, "after"):
		left, right = "Before", "After"
	case strings.Contains(lower, "current"):
		left = "Current"
	}
	var a, b []any
	arrows := 0
	for _, it := range n.items {
		loc := namedArrowSplit.FindStringSubmatchIndex(it)
		if loc == nil {
			// "reconciliation effort −70%": a change with no stated target.
			a = append(a, it)
			b = append(b, patterns.FillPlaceholder)
			continue
		}
		arrows++
		a = append(a, strings.TrimSpace(it[:loc[2]]))
		b = append(b, strings.TrimSpace(it[loc[4]:]))
	}
	if arrows == 0 {
		a, b = nil, nil
	}
	if len(a) == 0 {
		for _, note := range n.notes {
			m := namedTargetEvery.FindStringSubmatch(note)
			if m == nil {
				continue
			}
			if m[2] != "" {
				right = "Target " + strings.ToUpper(m[2])
			}
			for _, it := range n.items {
				a = append(a, it)
				label := it
				if k := namedSignedNumber.FindStringIndex(it); k != nil {
					label = strings.TrimSpace(it[:k[0]])
				}
				b = append(b, strings.TrimSpace(label+" "+m[1]))
			}
		}
	}
	if len(a) == 0 {
		if m := cueNamedVsHead.FindStringSubmatch(n.header); m != nil {
			return map[string]any{"columns": []any{map[string]any{"header": m[1], "items": []any{}}, map[string]any{"header": m[2], "items": []any{}}}}
		}
		return nil
	}
	return map[string]any{"columns": []any{map[string]any{"header": left, "items": a}, map[string]any{"header": right, "items": b}}}
}

// seriesData is a chart series read from the brief.
type seriesData struct {
	typ        string
	name       string
	unit       string
	categories []string
	series     []chartSeries
}

type chartSeries struct {
	name   string
	values []float64
}

var (
	namedSlashSeries = regexp.MustCompile(`(?i)([\p{L}][\p{L} -]*?)\s+(\d+(?:\.\d+)?(?:/\d+(?:\.\d+)?){2,})`)
	namedAmount      = `(?:(?:EUR|USD|GBP|CHF)\s?|[$€£¥]\s?)?[+\-−]?\d[\d,]*(?:\.\d+)?\s?(?:%|bn|mm|m|k)?`
	namedYearValue   = regexp.MustCompile(`\b((?:19|20)\d{2}):?\s+(` + namedAmount + `)`)
	namedValueYear   = regexp.MustCompile(`(` + namedAmount + `)\s*(?:\(((?:19|20)\d{2})\)|\bby\s+((?:19|20)\d{2})\b|\bin\s+((?:19|20)\d{2})\b)`)
	namedPercentPair = regexp.MustCompile(`([\p{L}][\p{L}\- /]*?)\s+(\d+(?:\.\d+)?)\s*%`)
	namedChangeVerb  = regexp.MustCompile(`(?i)\s+(?:grew|rose|fell|declined|increased|decreased|went|moved|climbed|dropped|is|was)\b.*$`)
	namedUnitOf      = regexp.MustCompile(`(?i)(EUR|USD|GBP|CHF|[$€£¥])\s?[\d.,]+\s?(bn|mm|m|k|%)?|[\d.,]+\s?(bn|mm|m|k|%)`)
)

func unitOf(amount string) string {
	m := namedUnitOf.FindStringSubmatch(amount)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(strings.ToUpper(m[1]) + " " + strings.ToUpper(m[2]+m[3]))
}

func amountValue(s string) float64 {
	v, _ := parseNumber(regexp.MustCompile(`[^\d.,+\-−]`).ReplaceAllString(s, ""))
	return v
}

// parseSeries reads the chart series a sentence carries: two or more series
// written with slashes, values labelled by year, values dated in brackets, a
// plain list of values, or a percentage split (a pie).
func parseSeries(n *namedSlide) *seriesData {
	for _, read := range []func(*namedSlide) *seriesData{slashSeries, yearLabelledSeries, datedSeries, numberListSeries, percentSplitSeries} {
		if sd := read(n); sd != nil {
			return sd
		}
	}
	return nil
}

// orFill returns the first non-empty name, else the placeholder.
func orFill(names ...string) string {
	for _, s := range names {
		if s != "" {
			return s
		}
	}
	return patterns.FillPlaceholder
}

// slashSeries reads "current 3.6/3.7/3.7 vs target 4.8/3.1/1.9".
func slashSeries(n *namedSlide) *seriesData {
	m := namedSlashSeries.FindAllStringSubmatch(n.sentence, -1)
	if len(m) == 0 {
		return nil
	}
	sd := &seriesData{typ: "line", name: n.header}
	for _, s := range m {
		var vals []float64
		for _, v := range strings.Split(s[2], "/") {
			f, _ := strconv.ParseFloat(v, 64)
			vals = append(vals, f)
		}
		name := strings.TrimSpace(s[1])
		if words := strings.Fields(name); len(words) > 0 {
			name = words[len(words)-1]
		}
		sd.series = append(sd.series, chartSeries{name: sentenceCase(name), values: vals})
	}
	for i := range sd.series[0].values {
		sd.categories = append(sd.categories, fmt.Sprintf("Year %d", i+1))
	}
	return sd
}

// yearLabelledSeries reads "2023 EUR 6.1M, 2024 EUR 8.4M; …", one series per
// segment.
func yearLabelledSeries(n *namedSlide) *seriesData {
	sd := &seriesData{typ: "bar", name: n.header}
	for _, seg := range topLevelSplit(n.body, ";") {
		pairs := namedYearValue.FindAllStringSubmatchIndex(seg, -1)
		if len(pairs) < 2 {
			continue
		}
		cs := chartSeries{name: orFill(strings.TrimSpace(strings.Trim(seg[:pairs[0][0]], " :,")), n.header)}
		for _, p := range pairs {
			year, val := seg[p[2]:p[3]], seg[p[4]:p[5]]
			if len(sd.series) == 0 {
				sd.categories = append(sd.categories, year)
			}
			cs.values = append(cs.values, amountValue(val))
			if sd.unit == "" {
				sd.unit = unitOf(val)
			}
		}
		sd.series = append(sd.series, cs)
	}
	if len(sd.series) == 0 || len(sd.categories) < 2 {
		return nil
	}
	if len(sd.series) > 1 {
		sd.typ = "line"
	}
	return sd
}

// datedSeries reads "EUR 8.1bn (2021) to EUR 9.4bn (2025), forecast EUR
// 10.6bn by 2028".
func datedSeries(n *namedSlide) *seriesData {
	pairs := namedValueYear.FindAllStringSubmatch(n.sentence, -1)
	if len(pairs) < 2 {
		return nil
	}
	sd := &seriesData{typ: "bar", name: n.header}
	cs := chartSeries{}
	for _, p := range pairs {
		sd.categories = append(sd.categories, p[2]+p[3]+p[4])
		cs.values = append(cs.values, amountValue(p[1]))
		if sd.unit == "" {
			sd.unit = unitOf(p[1])
		}
	}
	if len(n.items) > 0 {
		if subject := namedChangeVerb.ReplaceAllString(n.items[0], ""); subject != n.items[0] {
			cs.name = sentenceCase(strings.TrimSpace(subject))
		}
	}
	cs.name = orFill(cs.name, n.header)
	sd.series = []chartSeries{cs}
	return sd
}

var (
	namedNumberSep   = regexp.MustCompile(`\s*,\s*(?:and\s+)?|\s+and\s+`)
	namedNonNumeric  = regexp.MustCompile(`[^\d.,+\-−]`)
	namedNumberLabel = regexp.MustCompile(`^[\s:,]+|[\s:,]+$`)
)

// numberListSeries reads "quarterly revenue last 5 quarters 41.0, 42.3,
// 44.1, 45.9, 48.2": the periods are not named, so the categories are
// numbered.
func numberListSeries(n *namedSlide) *seriesData {
	m := factNumberList.FindStringIndex(n.body)
	if m == nil || len(n.items) > 1 {
		return nil
	}
	var vals []float64
	for _, piece := range namedNumberSep.Split(n.body[m[0]:m[1]], -1) {
		if v, ok := parseNumber(namedNonNumeric.ReplaceAllString(piece, "")); ok {
			vals = append(vals, v)
		}
	}
	if len(vals) < 3 {
		return nil
	}
	name := orFill(namedNumberLabel.ReplaceAllString(n.body[:m[0]], ""), n.header)
	sd := &seriesData{typ: "bar", name: n.header, series: []chartSeries{{name: name, values: vals}}}
	for i := range vals {
		sd.categories = append(sd.categories, strconv.Itoa(i+1))
	}
	return sd
}

// percentSplitSeries reads "compute 48%, storage 12%, licences 22%, people
// 18%" as a pie.
func percentSplitSeries(n *namedSlide) *seriesData {
	if len(n.items) < 3 {
		return nil
	}
	sd := &seriesData{typ: "pie", name: n.header, unit: "%"}
	cs := chartSeries{name: orFill(n.header)}
	for _, it := range n.items {
		m := namedPercentPair.FindStringSubmatch(it)
		if m == nil {
			return nil
		}
		label := strings.TrimSpace(m[1])
		if words := strings.Fields(label); len(words) > 3 {
			label = words[len(words)-1]
		}
		sd.categories = append(sd.categories, label)
		f, _ := strconv.ParseFloat(m[2], 64)
		cs.values = append(cs.values, f)
	}
	sd.series = []chartSeries{cs}
	return sd
}

func (sd *seriesData) chart() map[string]any {
	cats := make([]any, len(sd.categories))
	for i, c := range sd.categories {
		cats[i] = c
	}
	series := make([]any, 0, len(sd.series))
	for _, s := range sd.series {
		vals := make([]any, len(s.values))
		for i, v := range s.values {
			vals[i] = v
		}
		name := s.name
		if name == "" {
			name = patterns.FillPlaceholder
		}
		series = append(series, map[string]any{"name": name, "values": vals})
	}
	chart := map[string]any{"type": sd.typ, "data": map[string]any{"categories": cats, "series": series}}
	title := sd.name
	if sd.unit != "" {
		if title == "" {
			title = sd.unit
		} else {
			title += " (" + sd.unit + ")"
		}
	}
	if title != "" {
		chart["title"] = title
	}
	return chart
}

func chartFields(n *namedSlide) map[string]any {
	sd := parseSeries(n)
	if sd == nil {
		return nil
	}
	fields := map[string]any{"chart": sd.chart()}
	var insights []any
	for _, note := range n.notes {
		insights = append(insights, sentenceCase(note))
	}
	if len(insights) > 0 {
		fields["insights"] = insights
	}
	return fields
}

var (
	namedImagePath = regexp.MustCompile("`([^`]+\\.(?:png|jpe?g|svg|gif|webp))`|([^\\s`\"']+\\.(?:png|jpe?g|svg|gif|webp))\\b")
	namedCallOut   = regexp.MustCompile(`(?i)\bcall(?:s|ed|ing)?\s+out\s+(.+)$`)
	namedCalloutAt = regexp.MustCompile(`(?i)^(.+?)\s*\((?:roughly\s+|about\s+|approx\.?\s+|near\s+)?(?:the\s+)?([a-z]+(?:[- ][a-z]+)?)\)$`)
	calloutPoints  = map[string][2]float64{
		"top-left": {0.15, 0.15}, "top left": {0.15, 0.15}, "upper left": {0.15, 0.15}, "upper-left": {0.15, 0.15},
		"top-right": {0.85, 0.15}, "top right": {0.85, 0.15}, "upper right": {0.85, 0.15}, "upper-right": {0.85, 0.15},
		"bottom-left": {0.15, 0.85}, "bottom left": {0.15, 0.85}, "lower left": {0.15, 0.85}, "lower-left": {0.15, 0.85},
		"bottom-right": {0.85, 0.85}, "bottom right": {0.85, 0.85}, "lower right": {0.85, 0.85}, "lower-right": {0.85, 0.85},
		"centre": {0.5, 0.5}, "center": {0.5, 0.5}, "middle": {0.5, 0.5}, "top": {0.5, 0.15}, "bottom": {0.5, 0.85},
		"left": {0.15, 0.5}, "right": {0.85, 0.5},
	}
)

func imageCaseFields(n *namedSlide) map[string]any {
	fill := patterns.FillPlaceholder
	fields := map[string]any{"body": n.sentence}
	lower := strings.ToLower(n.sentence)
	switch {
	case strings.Contains(lower, "screenshot"):
		fields["eyebrow"] = "Screenshot"
	case strings.Contains(lower, "comparable") || strings.Contains(lower, "case"):
		fields["eyebrow"] = "Case study"
	}
	if m := namedImagePath.FindStringSubmatch(n.sentence); m != nil {
		fields["image"] = map[string]any{"path": m[1] + m[2]}
	} else {
		fields["image_label"] = fill
	}
	var metrics []any
	for _, piece := range append(append([]string{}, n.items...), n.notes...) {
		for _, m := range namedMetric.FindAllStringIndex(piece, -1) {
			if len(metrics) >= 3 {
				break
			}
			tok := strings.TrimSpace(piece[m[0]:m[1]])
			if namedOrdinal.MatchString(piece[m[0]:]) || (namedYear.MatchString(tok) && len(tok) == 4) {
				continue
			}
			after := strings.Fields(regexp.MustCompile(`(?i)^\s*(?:that|which|of|the|a|an)\s+`).ReplaceAllString(piece[m[1]:], ""))
			before := strings.Fields(piece[:m[0]])
			label := ""
			if len(after) > 0 {
				label = strings.Join(after[:min(3, len(after))], " ")
			} else if len(before) > 0 {
				label = strings.Join(before[max(0, len(before)-4):], " ")
			}
			metrics = append(metrics, map[string]any{"value": tok, "label": strings.Trim(label, " ,.;")})
		}
	}
	if len(metrics) > 0 {
		fields["metrics"] = metrics
	}
	for _, piece := range append(append([]string{}, n.items...), n.notes...) {
		m := namedCallOut.FindStringSubmatch(piece)
		if m == nil {
			continue
		}
		var callouts []any
		for _, c := range regexp.MustCompile(`\s*,\s*|\s+and\s+`).Split(m[1], -1) {
			cm := namedCalloutAt.FindStringSubmatch(strings.TrimSpace(c))
			if cm == nil {
				continue
			}
			label := strings.TrimSpace(regexp.MustCompile(`(?i)^(?:the|a|an)\s+`).ReplaceAllString(cm[1], ""))
			callout := map[string]any{"label": label}
			if p, ok := calloutPoints[strings.ToLower(cm[2])]; ok {
				callout["x"], callout["y"] = p[0], p[1]
			}
			callouts = append(callouts, callout)
		}
		if len(callouts) > 0 {
			fields["callouts"] = callouts
		}
	}
	return fields
}

// --- Titles ---

var (
	deckNameQuoted = regexp.MustCompile(`["“]([^"”]{2,60})["”]`)
	deckNameFor    = regexp.MustCompile(`\bfor\s+((?:[A-Z][\p{L}&'’.-]*\s?){1,4})`)
)

// deckName is the name the brief gives its deck in the topic clause: a quoted
// name ("Project Falcon") or the proper noun after "for" ("for Harbour Bank").
// A lower-case object ("for the board", "for a startup") is not a name.
func deckName(topic string) string {
	if m := deckNameQuoted.FindStringSubmatch(topic); m != nil {
		return strings.TrimSpace(m[1])
	}
	for _, m := range deckNameFor.FindAllStringSubmatch(topic, -1) {
		name := strings.TrimSpace(m[1])
		proper := false
		for _, w := range strings.Fields(name) {
			if r, _ := utf8.DecodeRuneInString(w); unicode.IsUpper(r) && strings.IndexFunc(w, unicode.IsLower) > 0 {
				proper = true
			}
		}
		if proper {
			return name
		}
	}
	return ""
}

// deckTitle is the draft's title: the deck name the brief gives, else the
// topic when it is short enough to be a title, else __FILL__ — never a
// truncated fragment of the brief (go-slide-creator-xbwlt).
func deckTitle(topicBrief string) string {
	first := strings.TrimSpace(topicBrief)
	if parts := splitBriefClauses(first); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		first = strings.TrimSpace(parts[0])
	}
	first = strings.TrimRight(first, " ,;:")
	if name := deckName(first); name != "" {
		return name
	}
	if utf8.RuneCountInString(first) <= 80 {
		return sentenceCase(first)
	}
	return patterns.FillPlaceholder
}

// wordSet is the significant words of a text, lower-cased, trailing "s"
// trimmed, for matching a named sentence to an outline item.
func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range namedWordChars.FindAllString(strings.ToLower(s), -1) {
		w = strings.TrimSuffix(w, "s")
		if len(w) >= 4 && !namedStopWords[w] {
			out[w] = true
		}
	}
	return out
}

var namedStopWords = map[string]bool{"with": true, "from": true, "that": true, "this": true, "into": true, "their": true, "them": true, "than": true, "then": true, "each": true, "slide": true, "beside": true, "left": true, "right": true, "about": true, "over": true, "under": true, "three": true, "four": true, "five": true, "seven": true, "eight": true, "today": true, "current": true, "view": true}

// sharedWords counts the significant words two texts share.
func sharedWords(a, b string) int {
	wa, wb := wordSet(a), wordSet(b)
	n := 0
	for w := range wa {
		if wb[w] {
			n++
		}
	}
	return n
}
