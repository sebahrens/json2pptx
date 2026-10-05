package patterns

import (
	"math"
	"regexp"
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

// Status boards, single findings, negated bullets, share splits and tiered
// structures (go-slide-creator-ux1fl): the consulting intents the 2026-10
// agent journeys found misrouted.

// statusBoardKeywords name a board of rows rated on a status scale: a risk
// appetite dashboard (metric, limit, within / amber / breached), RAG results
// by domain, controls tested with exceptions.
var statusBoardKeywords = []string{
	"status board", "status dashboard", "status table", "status by workstream", "status by domain", "status per",
	"risk appetite", "appetite dashboard", "appetite statement", "within appetite", "outside appetite",
	"rag status", "rag rating", "rag ratings", "rag dashboard", "traffic light", "traffic lights", "red amber green",
	"breached", "breached rows", "against limits", "against thresholds", "metric and limit", "metric and the limit",
	"results by domain", "exceptions by domain", "rating by domain", "controls tested", "control results",
	"compliance status", "exception report",
}

const statusBoardRationale = "Status board: one row per risk type / domain, columns for the metric, the limit and a status scored red / amber / green — the option_matrix kind with scale \"rag\" (table-highlight), the breached rows named in recommended so they are highlighted"

// intentIsStatusBoard reports a board of rows with a status: a status-board
// phrase, or a status word beside the thing it is measured against.
func intentIsStatusBoard(words []string) bool {
	if intentHasAny(words, statusBoardKeywords...) {
		return true
	}
	return intentHasAny(words, "status", "rag", "amber", "rating", "ratings", "breach", "breaches") &&
		intentHasAny(words, "limit", "limits", "threshold", "thresholds", "exceptions", "metric", "metrics",
			"domain", "domains", "workstream", "workstreams", "control", "controls", "appetite")
}

// singleFindingKeywords name one audit / review finding laid out as its parts.
var singleFindingKeywords = []string{
	"audit finding", "one finding", "single finding", "finding per slide", "one finding per slide", "finding slide",
	"each finding", "per finding", "what we found", "why it matters", "observation and recommendation",
	"finding and recommendation", "recommendation per finding", "finding and action", "root cause and action",
	"issue per slide", "one issue per slide", "one issue", "single issue", "structured finding",
}

const singleFindingRationale = "One finding / issue / observation on its own slide as labelled rows — WHAT WE FOUND / WHY IT MATTERS / ACTION / OWNER / DUE DATE — each a keyword label beside one to four lines of body (not bullets)"

func intentIsSingleFinding(words []string) bool {
	return intentHasAny(words, singleFindingKeywords...)
}

// intentNegatesBullets reports "not bullets" / "instead of bullets": the
// bullets layout matched the word, not the request.
func intentNegatesBullets(words []string) bool {
	return intentHasAny(words, "not bullets", "no bullets", "not a bullet list", "not bullet points", "no bullet points",
		"instead of bullets", "rather than bullets", "without bullets", "not as bullets", "beyond bullets")
}

// intentIsTieredStructure reports a stack, tier or layer structure — a
// "target architecture" is a structure, not a measure against a target, so
// a gauge is never its chart.
func intentIsTieredStructure(words []string) bool {
	return intentHasAny(words, "stack", "stacks", "tier", "tiers", "tiered", "layer", "layers", "layered",
		"architecture", "target state", "target operating model", "target architecture", "target model", "pyramid")
}

var percentRE = regexp.MustCompile(`(\d{1,3}(?:\.\d+)?)\s*(?:%|percent\b|pct\b)`)

// intentShareSplit reports whether the intent lists three or more
// percentages that sum to a whole (95–105): a part-to-whole split, which is
// a pie / donut / stacked bar, not a driver tree or a bridge. n is the
// number of parts.
func intentShareSplit(intentLower string) (n int, ok bool) {
	sum := 0.0
	for _, m := range percentRE.FindAllStringSubmatch(intentLower, -1) {
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		sum += f
		n++
	}
	return n, n >= 3 && sum >= 95 && sum <= 105
}

// shareSplitLead is the chart that leads a share split of n parts: a pie up
// to pieMaxRecommendedSlices, a stacked bar past it.
func shareSplitLead(n int, hints *VisualHints) string {
	if n <= pieMaxRecommendedSlices && (hints == nil || !chartOverloaded("pie", hints, "")) {
		return "pie"
	}
	return "stacked_bar"
}

// applyConsultingIntentRouting adds the candidates a consulting intent needs
// but keyword scoring cannot produce, then re-weights the ranking.
func applyConsultingIntentRouting(all []VisualCandidate, intentLower string, hints *VisualHints) []VisualCandidate {
	return consultingIntentRouting(all, intentLower, hints, true)
}

// applyConsultingShortlistRouting is the shortlist form of the same model:
// the scores the routing gives a name in the open ranking apply to that name
// in the shortlist, but no name the caller did not ask about is added
// (go-slide-creator-ux1fl).
func applyConsultingShortlistRouting(all []VisualCandidate, intentLower string, hints *VisualHints) []VisualCandidate {
	return consultingIntentRouting(all, intentLower, hints, false)
}

func consultingIntentRouting(all []VisualCandidate, intentLower string, hints *VisualHints, addMissing bool) []VisualCandidate { //nolint:gocognit,gocyclo
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
		if !addMissing {
			return
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
	if intentIsStatusBoard(words) && !intentIsRiskMatrix(words) {
		ensure(VisualCategoryPattern, "table-highlight", 0.94, statusBoardRationale)
		ensure(VisualCategoryPlaceholder, "table", 0.86,
			"Native table for a status board whose status is a word rather than a colour: Risk type | Metric | Limit | Status, one row per risk type")
	}
	if intentIsSingleFinding(words) {
		ensure(VisualCategoryPattern, "labeled-rows", 0.94, singleFindingRationale)
	}
	if n, ok := intentShareSplit(intentLower); ok {
		parts := strconv.Itoa(n) + " parts summing to 100%"
		if n <= pieMaxRecommendedSlices {
			ensure(VisualCategoryChart, "pie", 0.92, "Pie chart: "+parts+" are a part-to-whole split, each slice labelled with its share")
			ensure(VisualCategoryChart, "donut", 0.88, "Donut chart: "+parts+" around a centre annotation (the total)")
			ensure(VisualCategoryChart, "stacked_bar", 0.86, "One stacked bar: "+parts+" as segments of a single 100% bar")
		} else {
			ensure(VisualCategoryChart, "stacked_bar", 0.92, "One stacked bar: "+parts+" as segments of a single 100% bar — past five slices a pie's labels collide")
			ensure(VisualCategoryChart, "bar", 0.88, "Sorted bar chart: "+parts+" as bars largest first when the segments are too many for one stacked bar")
		}
	}
	if addMissing {
		all = ensureNamedOutright(all, words)
	}
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
	cs.adjustWaveIntents(words, intentLower, hints)
	switch {
	case intentIsStatusBoard(words) && !intentIsRiskMatrix(words):
		cs.makeTop(cs.find(VisualCategoryPattern, "table-highlight"), "a status beside a metric and a limit per row is a rated table, not a row of KPI tiles")
	case intentIsOptionMatrix(words) && !intentIsRiskMatrix(words) && !intentIsTabular(words):
		cs.makeTop(cs.find(VisualCategoryPattern, "table-highlight"), "options against criteria is an evaluation matrix, not a chart of one measure")
	case intentIsTwoWayComparison(words, hints):
		cs.preferTwoWayComparison(words)
	}
	return all
}

// adjustWaveIntents re-weights for the go-slide-creator-ux1fl intents: a
// negated bullets request, one finding as labelled rows, and a share split
// that is a part-to-whole chart.
func (cs candidateSet) adjustWaveIntents(words []string, intentLower string, hints *VisualHints) {
	if intentNegatesBullets(words) {
		if c := cs.find(VisualCategoryPlaceholder, "content"); c != nil {
			setCandidateScore(c, c.Score-0.30, "the intent asks for a structured layout, not bullets")
		}
	}
	if intentIsSingleFinding(words) {
		cs.makeTop(cs.find(VisualCategoryPattern, "labeled-rows"), "one finding reads as labelled parts, not as a list or a summary of several")
	}
	n, ok := intentShareSplit(intentLower)
	if !ok {
		return
	}
	lead := shareSplitLead(n, hints)
	if lead != "pie" {
		for _, name := range []string{"pie", "donut"} {
			if c := cs.find(VisualCategoryChart, name); c != nil && c.Score >= 0.5 {
				setCandidateScore(c, 0.49, "more than five slices: a pie's labels collide")
			}
		}
	}
	cs.makeTop(cs.find(VisualCategoryChart, lead), "shares that sum to 100% are a part-to-whole split, not a driver tree or a bridge")
}

// preferTwoWayComparison: two values are a comparison, not a time series
// ("monthly churn" names the measure; a trend chart needs the trend asked
// for), and comparison-2col leads.
func (cs candidateSet) preferTwoWayComparison(words []string) {
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
