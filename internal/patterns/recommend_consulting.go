package patterns

import (
	"math"
	"strconv"
)

// Consulting-intent routing for recommend_visual (go-slide-creator-biw2e).
//
// Keyword scoring alone misranked the most common consulting slide intents:
// "compare revenue across 5 regions" tied stat-hero with bar (stat-hero's
// keyword list includes "revenue"), "org chart of the leadership team" put the
// team-bios people grid above the org_chart diagram, "executive summary"
// ranked the plain bullets layout above the exec-summary pattern, "appendix
// of detailed financials" returned hero-detail, and "risk matrix" / "key
// risks and mitigations" surfaced a two-column comparison and a topic card
// grid. These rules encode what the intent means, applied after keyword
// scoring and before ranking.

// intentHasAny reports whether any phrase occurs in the intent as whole words.
func intentHasAny(words []string, phrases ...string) bool {
	for _, p := range phrases {
		if intentContainsPhrase(words, intentWords(p)) {
			return true
		}
	}
	return false
}

// intentMultiItem reports whether the intent describes several data points —
// a comparison across categories or a trend over periods — rather than one
// number. A numeral ≥ 2 counts ("5 regions", "last 8 quarters"), as do the
// words that only make sense with several values.
func intentMultiItem(words []string, hints *VisualHints) bool {
	if hints != nil {
		if hints.ItemCount == 1 {
			return false
		}
		if hints.ItemCount >= 2 || hints.DataPoints >= 2 || hints.SeriesCount >= 2 {
			return true
		}
	}
	if intentHasAny(words, "single", "one number", "big number", "hero", "headline number") {
		return false
	}
	for _, w := range words {
		if n, err := strconv.Atoi(w); err == nil && n >= 2 && n < 1000 {
			return true
		}
	}
	return intentHasAny(words, "across", "compare", "comparison", "versus", "vs", "trend", "over time",
		"by region", "by segment", "by product", "by quarter", "by month", "by year", "per region",
		"quarters", "months", "years", "regions", "segments", "products", "countries", "markets")
}

func intentIsOrgChart(words []string) bool {
	return intentHasAny(words, "org chart", "organization chart", "organisation chart", "organigram",
		"reporting structure", "reporting line", "reporting lines", "org structure",
		"organizational structure", "organisational structure", "who reports")
}

func intentIsExecSummary(words []string) bool {
	return intentHasAny(words, "executive summary", "exec summary", "summary of key findings", "key findings summary")
}

func intentIsAppendix(words []string) bool {
	return intentHasAny(words, "appendix", "appendices", "backup", "back up", "annex")
}

func intentIsRisk(words []string) bool {
	return intentHasAny(words, "risk", "risks", "risk register", "risk matrix", "risk heat map", "risk heatmap")
}

func intentIsRiskMatrix(words []string) bool {
	return intentIsRisk(words) && intentHasAny(words, "matrix", "heat map", "heatmap", "likelihood", "probability", "impact", "severity")
}

// intentIsOptionMatrix reports an options-against-criteria evaluation: "three
// vendors on five criteria", "score the options against cost and risk".
func intentIsOptionMatrix(words []string) bool {
	if !intentHasAny(words, "criteria", "criterion", "dimensions") {
		return false
	}
	return intentHasAny(words, "option", "vendor", "supplier", "provider", "alternative", "candidate", "solution",
		"platform", "tool", "bidder", "partner", "compare", "comparison", "evaluate", "evaluation", "assess",
		"assessment", "score", "scoring", "shortlist", "recommendation")
}

// intentIsTwoWayComparison reports two things set against each other ("SMB
// 2.1% vs enterprise 0.6%"): a contrast word plus a count of two.
func intentIsTwoWayComparison(words []string, hints *VisualHints) bool {
	if !intentHasAny(words, "vs", "versus", "contrast", "against", "compared with", "compared to") {
		return false
	}
	if hints != nil && hints.ItemCount > 0 {
		return hints.ItemCount == 2
	}
	return intentHasAny(words, "two", "2", "both", "pair")
}

func intentIsTabular(words []string) bool {
	return intentHasAny(words, "table", "tabular", "p&l", "profit and loss", "income statement", "balance sheet",
		"financials", "financial statement", "price list", "rate card", "segment split", "line items",
		"risk register")
}

// applyConsultingIntentRouting adds the candidates a consulting intent needs
// but keyword scoring cannot produce, then re-weights the ranking. Shortlist
// mode only re-weights (adjustConsultingIntentScores): it must not invent
// names the caller did not ask about.
func applyConsultingIntentRouting(all []VisualCandidate, intentLower string, hints *VisualHints) []VisualCandidate {
	words := intentWords(intentLower)
	ensure := func(cat VisualCategory, name string, score float64, rationale string) {
		for i := range all {
			if all[i].Category == cat && all[i].Name == name {
				if all[i].Score < score {
					all[i].Score = score
					all[i].Rationale = rationale
					all[i].ConfidenceBand = confidenceBand(score)
				}
				return
			}
		}
		all = append(all, VisualCandidate{Category: cat, Name: name, Score: score, Rationale: rationale, ConfidenceBand: confidenceBand(score)})
	}
	tableRationale := tablePlaceholderRationale()

	if intentIsTabular(words) && !intentIsRiskMatrix(words) {
		ensure(VisualCategoryPlaceholder, "table", 0.92, tableRationale)
		if intentHasAny(words, "p&l", "profit and loss", "income statement", "balance sheet", "financials", "financial", "pricing", "segment") {
			ensure(VisualCategoryPattern, "table-highlight", 0.85,
				"table-highlight when the rows are options or segments scored against criteria (Harvey balls, RAG) with one recommended row; plain figures stay a native table")
		}
	}
	if intentIsAppendix(words) {
		ensure(VisualCategoryPlaceholder, "section", 0.90,
			"Appendix / backup divider: a section slide titled \"Appendix\" (or section_number: false / DeckSpec structure.sections[].appendix: true) is unnumbered and left out of the agenda")
		ensure(VisualCategoryPlaceholder, "content", 0.86,
			"Backup content slide: dense supporting detail reads best as a plain titled content slide")
		if intentHasAny(words, "financials", "financial", "figures", "data", "detail", "detailed", "numbers") {
			ensure(VisualCategoryPlaceholder, "table", 0.88, tableRationale)
		}
	}
	if intentIsRiskMatrix(words) {
		ensure(VisualCategoryPattern, "matrix-2x2", 0.92,
			"Risk heat map: matrix-2x2 (DeckSpec kind matrix_2x2) with Likelihood × Impact axes, each quadrant listing the risks that sit in it")
		ensure(VisualCategoryPattern, "table-highlight", 0.88,
			"Risk register scored with RAG dots per likelihood / impact, highest-exposure risk highlighted")
	} else if intentIsRisk(words) && intentHasAny(words, "mitigation", "mitigations", "mitigate", "register", "owner", "owners") {
		ensure(VisualCategoryPlaceholder, "table", 0.94,
			"Risk register as a native table: Risk | Likelihood | Impact | Mitigation | Owner — one row per risk, not a card per topic")
		ensure(VisualCategoryPattern, "table-highlight", 0.86,
			"Risk register scored with RAG dots per likelihood / impact, highest-exposure risk highlighted")
	}
	if intentIsOrgChart(words) {
		ensure(VisualCategoryDiagram, "org_chart", 0.96,
			"Organizational chart for reporting structures")
	}
	if intentIsOptionMatrix(words) && !intentIsRiskMatrix(words) {
		ensure(VisualCategoryPattern, "table-highlight", 0.93,
			"Options scored against shared criteria: one row per option, one column per criterion (Harvey balls, RAG or short text), the recommended row highlighted")
	} else if intentIsTwoWayComparison(words, hints) {
		ensure(VisualCategoryPattern, "comparison-2col", 0.90,
			"Two things set against each other: one column each, the figures in the headers")
	}
	if intentIsExecSummary(words) && !intentHasAny(words, "scqa", "situation", "complication") {
		ensure(VisualCategoryPattern, "exec-summary", 0.90,
			"Executive summary of 3-5 bold lead-in statements, each with one supporting sentence")
	}
	all = ensureNamedOutright(all, words)
	return adjustConsultingIntentScores(all, intentLower, hints)
}

// tablePlaceholderRationale is the "table" placeholder rule's rationale.
func tablePlaceholderRationale() string {
	for _, r := range placeholderRules {
		if r.slideType == "table" {
			return r.rationale
		}
	}
	return "Native PowerPoint table"
}

// candidateSet is the ranking being re-weighted.
type candidateSet []VisualCandidate

func (cs candidateSet) find(cat VisualCategory, name string) *VisualCandidate {
	for i := range cs {
		if cs[i].Category == cat && cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

// bestScore is the top score among candidates matching keep.
func (cs candidateSet) bestScore(keep func(VisualCandidate) bool) float64 {
	best := 0.0
	for _, c := range cs {
		if keep(c) && c.Score > best {
			best = c.Score
		}
	}
	return best
}

func setCandidateScore(c *VisualCandidate, score float64, note string) {
	if c == nil {
		return
	}
	c.Score = roundScore(math.Max(0, math.Min(1, score)))
	if note != "" {
		c.Rationale += "; " + note
	}
	c.ConfidenceBand = confidenceBand(c.Score)
}

// adjustConsultingIntentScores re-weights existing candidates for the
// consulting intents above. It never adds candidates.
func adjustConsultingIntentScores(all []VisualCandidate, intentLower string, hints *VisualHints) []VisualCandidate {
	words := intentWords(intentLower)
	cs := candidateSet(all)
	if intentMultiItem(words, hints) {
		cs.demoteStatHero()
	}
	if intentIsOrgChart(words) {
		cs.preferOrgChart()
	}
	if intentIsExecSummary(words) && !intentHasAny(words, "scqa", "situation", "complication") {
		cs.preferExecSummary()
	}
	if intentIsAppendix(words) {
		cs.plainAppendix()
	}
	if intentIsRisk(words) {
		for _, name := range []string{"comparison-2col", "card-grid", "stylish-panels"} {
			if c := cs.find(VisualCategoryPattern, name); c != nil {
				setCandidateScore(c, c.Score-0.40, "")
			}
		}
	}
	if intentIsTabular(words) {
		cs.keepTableOnTop()
	}
	if intentIsOptionMatrix(words) && !intentIsRiskMatrix(words) && !intentIsTabular(words) {
		cs.makeTop(cs.find(VisualCategoryPattern, "table-highlight"), "options against criteria is an evaluation matrix, not a chart of one measure")
	} else if intentIsTwoWayComparison(words, hints) {
		// Two values are a comparison, not a time series: "monthly churn" names
		// the measure. A trend chart needs the trend asked for.
		if !intentHasAny(words, "trend", "over time", "time series", "trajectory") {
			cmp := cs.find(VisualCategoryPattern, "comparison-2col")
			for _, name := range []string{"line", "area", "stacked_area", "small_multiples"} {
				if c := cs.find(VisualCategoryChart, name); c != nil && cmp != nil && c.Score >= cmp.Score-0.10 {
					setCandidateScore(c, cmp.Score-0.15, "two values compared are not a time series")
				}
			}
		}
		cs.makeTop(cs.find(VisualCategoryPattern, "comparison-2col"), "")
	}
	return all
}

// demoteStatHero: several values compared or trended cannot be shown by one
// oversized number, so the chart outranks stat-hero.
func (cs candidateSet) demoteStatHero() {
	c := cs.find(VisualCategoryPattern, "stat-hero")
	if c == nil {
		return
	}
	best := cs.bestScore(func(o VisualCandidate) bool { return o.Category == VisualCategoryChart })
	if best > 0 && c.Score >= best-0.05 {
		setCandidateScore(c, math.Min(c.Score-0.20, best-0.10), "the intent compares or trends several values; one hero number cannot show them")
	}
}

// preferOrgChart: a reporting structure is a tree, not a people grid.
func (cs candidateSet) preferOrgChart() {
	org := cs.find(VisualCategoryDiagram, "org_chart")
	if org == nil {
		return
	}
	for _, name := range []string{"team-bios", "card-grid", "contact-directory"} {
		if c := cs.find(VisualCategoryPattern, name); c != nil && c.Score >= org.Score-0.05 {
			setCandidateScore(c, org.Score-0.10, "a reporting structure is a tree (org_chart); use team-bios for who is on the team")
		}
	}
}

// preferExecSummary: "executive summary" names the exec-summary pattern; the
// plain bullets layout matched only on the word "summary".
func (cs candidateSet) preferExecSummary() {
	exec := cs.find(VisualCategoryPattern, "exec-summary")
	if exec == nil {
		return
	}
	top := cs.bestScore(func(o VisualCandidate) bool {
		return o.Category != exec.Category || o.Name != exec.Name
	})
	if exec.Score <= top {
		setCandidateScore(exec, top+0.01, "the executive summary is 3–5 bold conclusions with their evidence")
	}
	if c := cs.find(VisualCategoryPlaceholder, "content"); c != nil && c.Score >= exec.Score-0.10 {
		setCandidateScore(c, exec.Score-0.15, "")
	}
}

// plainAppendix: back matter is dividers, content and tables; a hero or
// other emphasis pattern is the wrong register.
func (cs candidateSet) plainAppendix() {
	for i := range cs {
		if cs[i].Category == VisualCategoryPattern || cs[i].Category == VisualCategoryCompose {
			setCandidateScore(&cs[i], cs[i].Score-0.30, "")
		}
	}
}

// keepTableOnTop: tabular data stays tabular; a card grid or pyramid must
// not outrank the native table.
func (cs candidateSet) keepTableOnTop() {
	table := cs.find(VisualCategoryPlaceholder, "table")
	if table == nil {
		return
	}
	for i := range cs {
		c := &cs[i]
		if c == table || c.Name == "table-highlight" || c.Category == VisualCategoryChart {
			continue
		}
		if c.Score >= table.Score {
			setCandidateScore(c, table.Score-0.05, "")
		}
	}
}

// makeTop lifts c clear of every other candidate by more than a near tie;
// when the scale's top stops it, the others are capped below instead.
func (cs candidateSet) makeTop(c *VisualCandidate, note string) {
	if c == nil {
		return
	}
	const gap = differsByMargin + 0.01
	top := cs.bestScore(func(o VisualCandidate) bool {
		return o.Category != c.Category || o.Name != c.Name
	})
	if c.Score >= top+gap {
		return
	}
	setCandidateScore(c, math.Min(1, top+gap), note)
	for i := range cs {
		o := &cs[i]
		if o == c || o.Score <= c.Score-gap {
			continue
		}
		setCandidateScore(o, c.Score-gap, "")
	}
}

// namedOutrightScore is the score a visual the intent names outright enters
// the ranking with when no keyword rule produced it.
const namedOutrightScore = 0.90

// ensureNamedOutright adds the chart / diagram the intent names outright
// when keyword scoring produced no candidate for it: "a line chart of weekly
// active users" carries no trend keyword, yet names its chart
// (go-slide-creator-3ujfq). Patterns are left to their rules, which cover
// every pattern name.
func ensureNamedOutright(all []VisualCandidate, words []string) []VisualCandidate {
	cs := candidateSet(all)
	readyCharts, readyDiagrams := readyChartTypes(), readyDiagramTypes()
	for _, r := range chartRules {
		c := VisualCandidate{Category: VisualCategoryChart, Name: r.chartType}
		if !readyCharts[r.chartType] || cs.find(c.Category, c.Name) != nil || !namedOutright(c, words) {
			continue
		}
		c.Score, c.Rationale, c.ConfidenceBand = namedOutrightScore, r.rationale, confidenceBand(namedOutrightScore)
		all = append(all, c)
	}
	for _, r := range diagramRules {
		c := VisualCandidate{Category: VisualCategoryDiagram, Name: r.diagramType}
		if !readyDiagrams[r.diagramType] || cs.find(c.Category, c.Name) != nil || !namedOutright(c, words) {
			continue
		}
		c.Score, c.Rationale, c.ConfidenceBand = namedOutrightScore, r.rationale, confidenceBand(namedOutrightScore)
		all = append(all, c)
	}
	return all
}
