package deckplan

import (
	"fmt"
	"regexp"
	"strings"
)

// (go-slide-creator-xbwlt) outline items are resolved through the kind
// vocabulary in named.go before the rules below.

// Enumerated outlines (go-slide-creator-hf8tf).
//
// "7-slide investor pitch: problem, solution, market size chart, traction
// KPIs, business model, team of 4, the ask" is not a brief to route into a
// consulting storyline: it is the deck's table of contents. The storyline
// planner returned five slides for it, without the chart, the KPI, the team or
// the business-model slide. A brief that enumerates its slides now gets one
// slot per listed item, in the brief's order, each with the kind (and raw
// pattern) the item names.

// outlineItem is one listed slide of an enumerated outline.
type outlineItem struct {
	text string
	def  outlineKind
	// facts are the brief facts routed to the item: the item itself first.
	facts []string
	// fields are the kind's own fields, drafted from the item ("(cost,
	// duration)" criteria) and from the brief sentence merged into it.
	fields map[string]any
	// merged marks an item a named brief sentence has been folded into.
	merged bool
	// appendix marks an item the brief puts in the appendix.
	appendix bool
}

// briefOutline is the enumerated outline read from a brief.
type briefOutline struct {
	items []outlineItem
	// rest is the brief without the list: the topic and any other sentence.
	rest string
}

// outlineKind is the slide an outline item names.
type outlineKind struct {
	slot     string // deckspec slot name
	kind     string // DeckSpec kind
	pattern  string // raw pattern
	role     string // raw narrative role
	guidance string
	capacity int // facts from the rest of the brief the slide can also carry
}

// outlineRule maps the words of an outline item to the slide it names.
type outlineRule struct {
	cue *regexp.Regexp
	def outlineKind
}

// Slot guidance shared with the storyline.
const (
	guideAnswer   = "The whole answer up front: 3-5 points, each a conclusion a later slide proves; the title is the single-sentence answer."
	guideClosing  = "Close on next steps, not \"Thank you\": 2-6 actions, each with an owner and a date, and the decisions requested. A plain closing kind stays available for a Q&A page."
	guideDecision = "The decision needed now: the options, the recommended one, and the ask with owner and date."
	guideOptions  = "The options scored against the same criteria, with the recommended option highlighted; the title names the winner and why."
	guideEvidence = "One chart that proves one claim: the title states the claim with its number; set takeaway and source; never invent data."
	guideRisks    = "What could go wrong: one row per risk with its impact and its mitigation; the title names the risk that matters most."
)

// Outline slide definitions, by what the item names.
var (
	outlineCover    = outlineKind{"cover", "title", "", "opening", "Deck title and a subtitle naming audience and date.", 0}
	outlineAgenda   = outlineKind{"agenda", "agenda", "agenda", "framework", "The agenda the brief asks for: one line per section, in deck order.", 0}
	outlineAnswer   = outlineKind{"answer", "executive_summary", "exec-summary", "framework", guideAnswer, 3}
	outlineClosing  = outlineKind{"closing", "next_steps", "next-steps", "framework", guideClosing, 4}
	outlineAsk      = outlineKind{"ask", "decision", "next-steps", "framework", guideDecision, 2}
	outlineOptions  = outlineKind{"options", "option_matrix", "table-highlight", "comparison", guideOptions, 4}
	outlineRisks    = outlineKind{"risks", "table", "labeled-rows", "framework", guideRisks, 3}
	outlineTeam     = outlineKind{"topic", "team", "team-bios", "framework", "Who is behind it: one card per person with name, role and the one line that makes them credible here.", 2}
	outlineRoadmap  = outlineKind{"roadmap", "roadmap", "phase-roadmap", "evidence", "When it happens: 3-6 phases with dates and the milestone that ends each.", 4}
	outlineTimeline = outlineKind{"roadmap", "timeline", "timeline-horizontal", "evidence", "When it happens: the dated milestones from the brief, in order; the title says what the sequence delivers.", 4}
	outlineChart    = outlineKind{"evidence", "chart_insight", "chart-insights-split", "evidence", guideEvidence, 1}
	outlineKPIs     = outlineKind{"context", "kpi_snapshot", "kpi-3up", "evidence", "2-6 KPIs in the brief's own numbers; the title states the headline as a sentence carrying its number.", 6}
	outlineStat     = outlineKind{"context", "stat", "stat-hero", "emphasis", "One number on its own slide; the title says what the number means.", 0}
	outlineProcess  = outlineKind{"plan", "process", "numbered-step-strip", "framework", "How it works: 3-6 steps in order, each an action with its outcome.", 3}
	outlineCompare  = outlineKind{"topic", "comparison", "comparison-2col", "comparison", "Two sides, row for row; the title says which side wins and on what.", 2}
	outlineQuote    = outlineKind{"topic", "quote", "pull-quote", "emphasis", "The voice the brief quotes, with who said it; never an invented quote.", 1}
	outlineProblem  = outlineKind{"problem", "pillars", "card-grid", "framework", "What is wrong or at stake: 3-5 points, each with its evidence; the title states the problem as a sentence.", 3}
	outlineArch     = outlineKind{"topic", "architecture", "arch-stack", "framework", "How it is built: the layers and what each does.", 2}
	outlineTable    = outlineKind{"topic", "table", "labeled-rows", "framework", "One row per item; the title says what the table shows.", 4}
	outlineTopic    = outlineKind{"topic", "pillars", "card-grid", "framework", "3-5 points, each a conclusion with its evidence; the title is the slide's message as a sentence.", 3}
	outlineBridge   = outlineKind{"bridge", "bridge", "waterfall-bridge", "evidence", namedGuidance["bridge"], 2}
	outlineCase     = outlineKind{"case", "image_case", "image-text-split", "evidence", namedGuidance["image_case"], 3}
	outlineMatrix   = outlineKind{"risks", "matrix_2x2", "matrix-2x2", "framework", namedGuidance["matrix_2x2"], 3}
	outlineHeatmap  = outlineKind{"risks", "risk_heatmap", "risk-heatmap", "framework", namedGuidance["risk_heatmap"], 3}
	outlineCycle    = outlineKind{"cycle", "cycle", "cycle-ring", "framework", cycleGuidance, 2}
	outlineRegions  = outlineKind{"regions", "regions", "", "evidence", regionsGuidance, 4}
	outlinePillars  = outlineKind{"framework", "pillars", "stylish-panels", "framework", namedGuidance["pillars"], 3}
	// A list of points or findings an outline item asks for, however it words
	// the form ("five key points as bullets"): a structured slide that fills
	// the page, never a bullet list (go-slide-creator-s49nz).
	outlineFindings = outlineKind{"topic", "pillars", "exec-summary", "framework", "3-5 findings, each a bold conclusion with the one line of evidence behind it: a structured slide, not a bullet list; the title is the slide's message as a sentence.", 3}
)

// outlineKindByName maps a kind the vocabulary names to its outline slide.
var outlineKindByName = map[string]outlineKind{
	"bridge": outlineBridge, "image_case": outlineCase, "matrix_2x2": outlineMatrix, "risk_heatmap": outlineHeatmap, "regions": outlineRegions,
	"cycle": outlineCycle, "architecture": outlineArch, "team": outlineTeam, "table": outlineTable, "comparison": outlineCompare,
	"option_matrix": outlineOptions, "decision": outlineAsk, "roadmap": outlineRoadmap, "process": outlineProcess,
	"kpi_snapshot": outlineKPIs, "chart_insight": outlineChart, "pillars": outlinePillars,
	"next_steps": outlineClosing, "executive_summary": outlineAnswer,
}

var outlineRules = []outlineRule{
	{regexp.MustCompile(`(?i)^(?:a\s+|the\s+)?(?:title|cover)(?:\s+(?:slide|page))?$`), outlineCover},
	{regexp.MustCompile(`(?i)\b(?:agenda|table of contents|contents page)\b`), outlineAgenda},
	{regexp.MustCompile(`(?i)\b(?:executive summary|exec summary|summary|overview|headlines?|key messages?|tl;?dr)\b`), outlineAnswer},
	{regexp.MustCompile(`(?i)\b(?:next[- ]steps?|action plan|actions|call to action|way forward|wrap[- ]up|q&a|closer)\b`), outlineClosing},
	{regexp.MustCompile(`(?i)\b(?:asks?|funding|fundraising|use of funds|decision|approval|recommendation)\b`), outlineAsk},
	{regexp.MustCompile(`(?i)\b(?:options?|alternatives|scenarios|trade-?offs?)\b`), outlineOptions},
	{regexp.MustCompile(`(?i)\b(?:risks?|mitigations?|threats?)\b`), outlineRisks},
	{regexp.MustCompile(`(?i)\b(?:team|founders?|leadership|management|who we are)\b`), outlineTeam},
	{regexp.MustCompile(`(?i)\b(?:roadmap|phases)\b`), outlineRoadmap},
	{regexp.MustCompile(`(?i)\b(?:milestones?|timeline|schedule)\b`), outlineTimeline},
	{regexp.MustCompile(`(?i)\b(?:charts?|graphs?|trends?|forecasts?|projections?|over time|by (?:quarter|year|month))\b`), outlineChart},
	{regexp.MustCompile(`(?i)\b(?:kpis?|metrics|traction|results|performance|numbers|financials|scorecard|dashboard|unit economics)\b`), outlineKPIs},
	{regexp.MustCompile(`(?i)\b(?:process|steps|how it works|workflow|journey)\b`), outlineProcess},
	{regexp.MustCompile(`(?i)\b(?:competition|competitors?|competitive|versus|vs\.?|comparison|before and after)\b`), outlineCompare},
	{regexp.MustCompile(`(?i)\b(?:quotes?|testimonials?|customer voices?)\b`), outlineQuote},
	{regexp.MustCompile(`(?i)\b(?:problems?|pains?|pain points?|challenges?|issues?|why now)\b`), outlineProblem},
	{regexp.MustCompile(`(?i)\b(?:architecture|tech stack|technology stack)\b`), outlineArch},
	{regexp.MustCompile(`(?i)\b(?:pricing|price list|budget|cost breakdown)\b`), outlineTable},
	// Last: a count of points ("five key points", "three findings") is a
	// points slide, not the one number the count would otherwise read as.
	{regexp.MustCompile(`(?i)\b(?:findings|takeaways?|conclusions|lessons(?:\s+learned)?|learnings|insights|observations)\b`), outlineFindings},
	{regexp.MustCompile(`(?i)\b(?:(?:key|main|talking)\s+points|points|bullets?|bullet\s+(?:points|list)|reasons|highlights|principles|priorities|themes)\b`), outlineTopic},
}

// outlineKindFor picks the slide an outline item names: a cue word in the item
// first, then what the item is as a fact (a series is a chart, a single number
// a stat), else a points slide.
func outlineKindFor(text string) outlineKind {
	f := classifyFact(text, factQuantity.MatchString(text))
	if f.list {
		return outlineChart
	}
	// The kind catalogue's own vocabulary first (go-slide-creator-xbwlt): a
	// "margin bridge" item is a bridge, a "heat map" a matrix, a "next-steps
	// closer" the close, a "chart beside a number" a regions slide.
	if regionsOutlineItem(text) != nil {
		return outlineRegions
	}
	if cueNamedAppendix.MatchString(text) {
		return outlineTable
	}
	n := readSentence(text)
	n.header = ""
	if namedKindFor(n) {
		if def, ok := outlineKindByName[n.kind]; ok {
			return def
		}
	}
	for _, r := range outlineRules {
		if r.cue.MatchString(text) {
			return r.def
		}
	}
	switch {
	case f.series:
		return outlineChart
	case f.numeric:
		return outlineStat
	}
	return outlineTopic
}

var (
	// outlineHeader marks a label that introduces the deck's slides.
	outlineHeader = regexp.MustCompile(`(?i)\b(?:slides?|outline|agenda|sections?|structure|storyline|story ?line|flow|chapters|topics|covering|covers?|order|required|must (?:include|have|cover)|deliverables?)\b`)

	// outlineDeckHeader marks a label that names the deck's slides outright:
	// its list is the outline whatever the items are.
	outlineDeckHeader = regexp.MustCompile(`(?i)\b(?:slides|outline|agenda|sections|chapters|storyline|story ?line)\b`)

	// outlineOneSlide matches a label about one slide — "A regions slide shows
	// a bar chart of the market", "The bridge slide walks EBITDA", "One KPI
	// slide" — whose list is that slide's data (go-slide-creator-hxcum). "The
	// slide order" still names the deck's.
	outlineOneSlide = regexp.MustCompile(`(?i)\b(?:a|an|one|this|that)\s+(?:[\w-]+\s+){0,3}slide\b|\S\s+slide$|\bslide\s+(?:shows?|has|carries|contains|walks|gives|lists|with|is|covers?|compares?|presents?|plots?|charts?|breaks)\b`)

	// outlineLineItem matches a list line: a bullet, a number, or "Slide 3:".
	outlineLineItem = regexp.MustCompile(`(?i)^\s*(?:[-*•▪·–—]|\(?\d{1,2}[.)]|slide\s+\d{1,2}\s*[:.)–—-])\s*`)

	// outlineSlideLine matches a line that numbers its slide outright.
	outlineSlideLine = regexp.MustCompile(`(?i)^\s*slide\s+\d{1,2}\b`)

	// outlineSentenceEnd matches the end of a sentence inside running text.
	outlineSentenceEnd = regexp.MustCompile(`[.!?](?:\s+|$)|\n`)

	// outlineInlineColon matches a colon that opens an inline list.
	outlineInlineColon = regexp.MustCompile(`:\s+`)

	outlineLeadingAnd = regexp.MustCompile(`(?i)^(?:and|then|finally)\s+`)

	// outlineInlineEnumerator matches the number an inline list gives its
	// item: "2) five key points", "3. revenue chart", "slide 4: risks". The
	// space after it keeps "1.5x growth" whole.
	outlineInlineEnumerator = regexp.MustCompile(`(?i)^(?:\(?\d{1,2}[.)]\s+|slide\s+\d{1,2}\s*[:.)–—-]\s*)`)
)

// labelsOutline reports whether a list's label introduces the deck's slides.
// A label that names the slides outright ("Slides", "Outline", "Agenda") does.
// A label about one slide ("A chart slide shows the market") does not, and any
// other header word ("covering", "must include", "order") does only when the
// list is not mostly figures: the data of a chart, a bridge or a KPI row is
// content, not slides.
func labelsOutline(label string, items []string) bool {
	switch {
	case !outlineHeader.MatchString(label):
		return false
	case outlineDeckHeader.MatchString(label):
		return true
	case outlineOneSlide.MatchString(label):
		return false
	}
	return !mostlyFigures(items)
}

// mostlyFigures reports whether more than half the items are figures.
func mostlyFigures(items []string) bool {
	figures := 0
	for _, it := range items {
		if classifyFact(it, factQuantity.MatchString(it)).numeric {
			figures++
		}
	}
	return 2*figures > len(items)
}

// minOutlineItems is the fewest listed items that read as an outline.
const minOutlineItems = 3

// parseOutline reads an enumerated outline from the brief, or returns nil. A
// list of three or more items is an outline when
//
//   - its label says so ("Slides: …", "Outline: …", "covering: …", or lines
//     that start "Slide 1: …"), or
//   - it follows the topic and its length matches the stated slide count (the
//     count, or the count less a cover and a close), and at most half the items
//     are figures, or
//   - it follows the topic and every item is a short topic rather than a
//     figure ("problem, solution, market, team, ask").
//
// A list the brief labels as facts ("Facts: …"), or one a list header counts
// ("three options: a, b, c"), is content, not an outline. stated is the slide
// count the caller or the brief gave, 0 for none.
func parseOutline(brief string, stated int) *briefOutline {
	if o := parseLineOutline(brief, stated); o != nil {
		return o
	}
	return parseInlineOutline(brief, stated)
}

// parseLineOutline reads an outline written as bulleted or numbered lines.
func parseLineOutline(brief string, stated int) *briefOutline {
	lines := strings.Split(brief, "\n")
	start, end := -1, -1
	for i, line := range lines {
		if outlineLineItem.MatchString(line) && strings.TrimSpace(outlineLineItem.ReplaceAllString(line, "")) != "" {
			if start < 0 {
				start = i
			}
			end = i
			continue
		}
		if start >= 0 && strings.TrimSpace(line) != "" {
			break
		}
	}
	if start < 0 || end-start+1 < minOutlineItems {
		return nil
	}
	var texts []string
	numbered := true
	for _, line := range lines[start : end+1] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		numbered = numbered && outlineSlideLine.MatchString(line)
		texts = append(texts, cleanOutlineItem(outlineLineItem.ReplaceAllString(line, "")))
	}
	label := ""
	if start > 0 {
		label = strings.TrimSpace(lines[start-1])
	}
	labelled := strings.HasSuffix(label, ":") && labelsOutline(strings.TrimSuffix(label, ":"), texts) && !isFillerLabel(strings.TrimSuffix(label, ":"))
	if !numbered && !labelled && !outlineCountMatches(texts, stated) {
		return nil
	}
	rest := lines[:start]
	if labelled {
		rest = lines[:start-1]
	}
	rest = append(append([]string{}, rest...), lines[end+1:]...)
	return newOutline(texts, strings.TrimSpace(strings.Join(rest, "\n")))
}

// parseInlineOutline reads an outline written as one comma- or
// semicolon-separated list after a colon.
func parseInlineOutline(brief string, stated int) *briefOutline {
	depths := bracketDepths(brief)
	for _, loc := range outlineInlineColon.FindAllStringIndex(brief, -1) {
		if depths[loc[0]] > 0 {
			continue
		}
		// The label runs back to the start of its sentence.
		labelStart := 0
		for _, e := range outlineSentenceEnd.FindAllStringIndex(brief[:loc[0]], -1) {
			labelStart = e[1]
		}
		label := strings.TrimSpace(brief[labelStart:loc[0]])
		if isFillerLabel(label) {
			continue
		}
		// The list runs to the end of its sentence.
		listEnd := len(brief)
		for _, e := range outlineSentenceEnd.FindAllStringIndex(brief[loc[1]:], -1) {
			if at := loc[1] + e[0]; depths[at] == 0 {
				listEnd = at
				break
			}
		}
		texts := splitOutlineItems(brief[loc[1]:listEnd])
		if len(texts) < minOutlineItems {
			continue
		}
		isTopic := labelStart == 0
		labelled := !isTopic && labelsOutline(label, texts)
		if isTopic && labelsOutline(label, texts) && len(strings.Fields(label)) <= 4 {
			labelled = true // the brief opens with "Slides: …"
		}
		switch {
		case labelled:
		case !isTopic || listHeaderGroup(label) != "":
			continue // "three options: a, b, c" lists content, not slides
		case outlineCountMatches(texts, stated), outlineItemsAreTopics(texts):
		default:
			continue
		}
		rest := brief[:labelStart]
		if isTopic {
			rest = label + "."
		}
		tail := strings.TrimLeft(brief[listEnd:], ".!? \n\t")
		return newOutline(texts, strings.TrimSpace(rest+" "+tail))
	}
	return nil
}

// splitOutlineItems splits an inline list at its top-level semicolons, or at
// its top-level commas when it has none. A bare number continues the item
// before it.
func splitOutlineItems(list string) []string {
	depths := bracketDepths(list)
	cut := func(sep byte) []string {
		var out []string
		last := 0
		for i := 0; i < len(list); i++ {
			if list[i] == sep && depths[i] == 0 && (sep == ';' || (i+1 < len(list) && list[i+1] == ' ')) {
				out = append(out, list[last:i])
				last = i + 1
			}
		}
		return append(out, list[last:])
	}
	raw := cut(';')
	if len(raw) < 2 {
		raw = cut(',')
	}
	var out []string
	for _, r := range raw {
		item := cleanOutlineItem(r)
		if item == "" {
			continue
		}
		if len(out) > 0 && isBareNumber(item) {
			out[len(out)-1] += ", " + item
			continue
		}
		out = append(out, item)
	}
	return out
}

// cleanOutlineItem trims an item to its own words.
func cleanOutlineItem(s string) string {
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), ".;,"))
	// The item's own number is not content: left in, it made every item of
	// "1) title; 2) five key points; …" read as a figure and plan a stat.
	s = outlineInlineEnumerator.ReplaceAllString(s, "")
	return strings.TrimSpace(outlineLeadingAnd.ReplaceAllString(s, ""))
}

// outlineCountMatches reports whether the list is as long as the stated slide
// count allows — the count itself, or the count less a cover and a close — and
// is not a list of figures.
func outlineCountMatches(items []string, stated int) bool {
	if stated <= 0 || len(items) > stated || len(items) < stated-2 {
		return false
	}
	return !mostlyFigures(items)
}

// outlineItemsAreTopics reports whether every item of a list of four or more
// is a short topic ("problem", "market size chart") and at most one a figure.
func outlineItemsAreTopics(items []string) bool {
	if len(items) < 4 {
		return false
	}
	figures := 0
	for _, it := range items {
		if len(strings.Fields(it)) > 5 {
			return false
		}
		if factQuantity.MatchString(it) {
			figures++
		}
	}
	return figures <= 1
}

// newOutline builds the outline: each item with the slide it names, and the
// facts of the rest of the brief routed to the items built to show them.
func newOutline(texts []string, rest string) *briefOutline {
	o := &briefOutline{rest: rest}
	for i, t := range texts {
		def := outlineKindFor(t)
		if def.slot == "cover" {
			continue // every plan opens with a cover
		}
		if def == outlineAsk && i == len(texts)-1 {
			// The ask that ends the deck is its close: next steps carry the
			// decisions requested.
			def = outlineClosing
		}
		def.guidance = fmt.Sprintf("Outline item %d of %d, in the brief's words: %q. %s", i+1, len(texts), t, def.guidance)
		item := outlineItem{text: t, def: def, facts: []string{t}, appendix: cueNamedAppendix.MatchString(t)}
		item.fields = outlineItemFields(t, def.kind)
		o.items = append(o.items, item)
	}
	if len(o.items) < minOutlineItems {
		return nil
	}
	return o
}

// outlineItemFields drafts the fields an outline item carries on its own:
// the criteria in an option matrix's brackets, the headers of a table, the
// regions of a "chart beside a number" slide, the headers of an "A vs B"
// comparison.
func outlineItemFields(text, kind string) map[string]any {
	switch kind {
	case "regions":
		if req := regionsOutlineItem(text); req != nil {
			body := req.draftBody()
			delete(body, "kind")
			delete(body, "title")
			return body
		}
		return sidesFields(text)
	case "option_matrix":
		if m := namedHeaderParen.FindStringSubmatch(text); m != nil {
			return map[string]any{"criteria": splitCriteria(m[1])}
		}
	case "table":
		n := readSentence(text)
		n.kind = "table"
		return tableFields(n)
	case "comparison":
		if m := cueNamedVsHead.FindStringSubmatch(text); m != nil {
			return map[string]any{"columns": []any{map[string]any{"header": m[1], "items": []any{}}, map[string]any{"header": m[2], "items": []any{}}}}
		}
	}
	return nil
}

// endsOnClose reports whether the outline's last body item is the deck's
// close.
func (o *briefOutline) endsOnClose() bool {
	body, _ := o.splitAppendix()
	if len(body) == 0 {
		return false
	}
	last := body[len(body)-1].def
	return last.kind == "next_steps" || last.kind == "decision"
}

// splitAppendix separates the outline's body items from the ones the brief
// puts in the appendix.
func (o *briefOutline) splitAppendix() (body, appendix []outlineItem) {
	for _, it := range o.items {
		if it.appendix {
			appendix = append(appendix, it)
		} else {
			body = append(body, it)
		}
	}
	return body, appendix
}

// outlineMergeKinds says which named kinds an outline item of each kind
// takes: the item's own kind, and the kinds whose facts it is built to show.
var outlineMergeKinds = map[string][]string{
	"regions":           {"chart_insight", "kpi_snapshot", "table", "comparison"},
	"option_matrix":     {"decision", "option_matrix"},
	"decision":          {"decision", "option_matrix"},
	"comparison":        {"comparison", "decision", "kpi_snapshot"},
	"kpi_snapshot":      {"kpi_snapshot", "comparison"},
	"chart_insight":     {"chart_insight"},
	"executive_summary": {},
	"next_steps":        {"next_steps"},
}

// mergeNamed folds the brief sentences that name a kind into the outline
// items built for them — by shared words first, then by kind — and returns
// the named slides no item takes, plus the closer sentence.
func (o *briefOutline) mergeNamed(named []namedSlide) (extras []namedSlide, closer *namedSlide) {
	for i := range named {
		n := named[i]
		if n.closer {
			if closer == nil {
				closer = &named[i]
			}
			o.mergeCloser(n)
			continue
		}
		best := o.bestItemFor(n)
		if best < 0 {
			extras = append(extras, n)
			continue
		}
		mergeIntoItem(&o.items[best], n)
	}
	o.fillComparisons(named)
	return extras, closer
}

// mergeCloser gives the outline's next-steps item the "Ask:" sentence.
func (o *briefOutline) mergeCloser(n namedSlide) {
	for j := range o.items {
		if o.items[j].def.kind != "next_steps" {
			continue
		}
		o.items[j].facts = append(o.items[j].facts, n.facts...)
		if o.items[j].fields == nil {
			o.items[j].fields = map[string]any{}
		}
		for k, v := range n.fields {
			o.items[j].fields[k] = v
		}
		return
	}
}

// itemAccepts reports whether an outline item can take a named sentence:
// not merged yet, and of a kind built to show the sentence's kind.
func itemAccepts(item outlineItem, n namedSlide) bool {
	if item.merged {
		return false
	}
	kinds, ok := outlineMergeKinds[item.def.kind]
	if !ok {
		return item.def.kind == n.kind
	}
	for _, k := range kinds {
		if k == n.kind {
			return true
		}
	}
	return false
}

// bestItemFor picks the outline item a named sentence belongs to: the
// accepting item sharing the most significant words, else the first that
// accepts it; -1 when none does.
func (o *briefOutline) bestItemFor(n namedSlide) int {
	best, bestShared := -1, -1
	for j := range o.items {
		if !itemAccepts(o.items[j], n) {
			continue
		}
		if s := sharedWords(o.items[j].text, n.header+" "+n.sentence); s > bestShared {
			best, bestShared = j, s
		}
	}
	return best
}

// fillComparisons gives a "B vs C" comparison with no sentence of its own its
// columns from the options the brief enumerates.
func (o *briefOutline) fillComparisons(named []namedSlide) {
	for j := range o.items {
		if o.items[j].def.kind != "comparison" || o.items[j].merged {
			continue
		}
		for _, n := range named {
			if n.kind == "decision" || n.kind == "option_matrix" {
				fillComparisonFromOptions(&o.items[j], n)
			}
		}
	}
}

// mergeIntoItem gives the item the named sentence's facts and fields; a
// regions item takes the chart or stat into its regions, an option matrix
// takes enumerated options as its rows.
func mergeIntoItem(item *outlineItem, n namedSlide) {
	item.merged = true
	item.facts = append(item.facts, n.facts...)
	if item.fields == nil {
		item.fields = map[string]any{}
	}
	switch {
	case item.def.kind == "regions":
		mergeIntoRegions(item.fields, n)
	case item.def.kind == n.kind || len(item.fields) == 0:
		for k, v := range n.fields {
			if _, keep := item.fields[k]; !keep {
				item.fields[k] = v
			}
		}
	case item.def.kind == "option_matrix" && n.kind == "decision":
		mergeDecisionIntoMatrix(item.fields, n)
	}
}

// mergeIntoRegions fills the regions of an outline item from the named
// sentence: the chart data into the chart region, the first KPI into the
// stat region, the table into the table region.
func mergeIntoRegions(fields map[string]any, n namedSlide) {
	regions, _ := fields["regions"].([]any)
	for _, r := range regions {
		region, _ := r.(map[string]any)
		switch {
		case region["kind"] == "chart" && n.kind == "chart_insight":
			if chart, ok := n.fields["chart"].(map[string]any); ok {
				setRegionChart(region, chart)
			}
		case region["kind"] == "stat" && n.kind == "kpi_snapshot":
			if kpis, ok := n.fields["kpis"].([]any); ok && len(kpis) > 0 {
				if first, ok := kpis[0].(map[string]any); ok {
					region["value"], region["label"] = first["value"], first["label"]
				}
			}
		case region["kind"] == "table" && n.kind == "table":
			region["headers"], region["rows"] = n.fields["headers"], n.fields["rows"]
		}
	}
}

// setRegionChart puts a drafted chart into a chart region. The chart's title
// becomes the region's heading and is not repeated inside the chart: a region
// draws its heading directly above the chart.
func setRegionChart(region, chart map[string]any) {
	inner := make(map[string]any, len(chart))
	for k, v := range chart {
		inner[k] = v
	}
	if title, ok := inner["title"].(string); ok {
		region["heading"] = title
		delete(inner, "title")
	}
	region["chart"] = inner
}

// mergeDecisionIntoMatrix turns a decision's enumerated options into the
// option matrix's rows, carrying the recommended one.
func mergeDecisionIntoMatrix(fields map[string]any, n namedSlide) {
	if opts, ok := n.fields["options"].([]any); ok {
		rows := make([]any, 0, len(opts))
		for _, o := range opts {
			om, _ := o.(map[string]any)
			row := map[string]any{"name": om["label"]}
			if d, ok := om["detail"]; ok {
				row["detail"] = d
			}
			if om["recommended"] == true {
				fields["recommended"] = om["label"]
			}
			rows = append(rows, row)
		}
		fields["options"] = rows
	}
	if rec, ok := n.fields["recommendation"]; ok {
		fields["takeaway"] = rec
	}
}

// fillComparisonFromOptions fills an "B vs C" comparison's columns from the
// enumerated options whose markers match the headers.
func fillComparisonFromOptions(item *outlineItem, n namedSlide) {
	cols, _ := item.fields["columns"].([]any)
	opts, _ := n.fields["options"].([]any)
	if len(cols) != 2 || len(opts) == 0 {
		return
	}
	for _, c := range cols {
		col, _ := c.(map[string]any)
		header := strings.ToLower(fmt.Sprint(col["header"]))
		for _, o := range opts {
			om, _ := o.(map[string]any)
			label := strings.ToLower(fmt.Sprint(firstNonNil(om["label"], om["name"])))
			if strings.HasPrefix(label, header+":") || strings.HasPrefix(label, header+" ") || label == header {
				items := []any{fmt.Sprint(firstNonNil(om["label"], om["name"]))}
				if d, ok := om["detail"]; ok {
					for _, piece := range splitListItems(fmt.Sprint(d)) {
						items = append(items, piece)
					}
				}
				col["items"] = items
				item.merged = true
			}
		}
	}
}

func firstNonNil(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return ""
}

// regionsOutlineItem reads an outline item that puts two or three visuals on
// one slide ("market chart beside the 2.3% share number and position
// bullets", "losses chart on the left with the headline number and two
// bullets on the right") as a regions request: columns for two visuals, the
// first as the main region beside a stack for three.
func regionsOutlineItem(text string) *regionRequest {
	if !regionsJoiner.MatchString(text) {
		return nil
	}
	type hit struct {
		at   int
		kind string
		word string
	}
	var hits []hit
	for _, cue := range outlineRegionCues {
		for _, loc := range cue.re.FindAllStringIndex(text, -1) {
			hits = append(hits, hit{loc[0], cue.kind, strings.ToLower(text[loc[0]:loc[1]])})
		}
	}
	if len(hits) < 2 {
		return nil
	}
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].at < hits[j-1].at; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	seen := map[string]bool{}
	var regions []regionReq
	for _, h := range hits {
		if seen[h.kind] || h.kind == "" {
			continue
		}
		seen[h.kind] = true
		r := regionReq{kind: h.kind, visual: h.word, text: text}
		if h.kind == "chart" {
			r.chartType = "bar_chart"
			if strings.Contains(strings.ToLower(text), "trend") || strings.Contains(strings.ToLower(text), "line chart") {
				r.chartType = "line_chart"
			}
		}
		regions = append(regions, r)
	}
	if len(regions) < 2 || len(regions) > 3 || (len(regions) > 0 && regions[0].kind != "chart" && !seen["chart"]) {
		if len(regions) < 2 || len(regions) > 3 {
			return nil
		}
	}
	req := &regionRequest{regions: regions, remainder: ""}
	req.arrangement = "columns"
	if len(regions) == 3 {
		req.arrangement = "main_left"
	}
	positions := []string{"left", "right", "lower right"}
	if len(regions) == 3 {
		positions = []string{"left", "upper right", "lower right"}
	}
	for i := range req.regions {
		req.regions[i].position = positions[i]
	}
	return req
}

var (
	regionsJoiner     = regexp.MustCompile(`(?i)\b(?:beside|alongside|next to|on the left|on the right|side by side|split with)\b`)
	outlineRegionCues = []regionCue{
		{regexp.MustCompile(`(?i)\b(?:(?:line|bar|column|area|pie|donut|trend)\s+)?(?:charts?|graphs?)\b`), "chart"},
		{regexp.MustCompile(`(?i)\b(?:kpis|metrics|scorecard|kpi tiles)\b`), "kpis"},
		{regexp.MustCompile(`(?i)\b(?:(?:share|headline|big|hero|key)\s+number|headline|big number|stat|kpi)\b`), "stat"},
		{regexp.MustCompile(`(?i)\b(?:timeline|milestones|roadmap)\b`), "timeline"},
		{regexp.MustCompile(`(?i)\btables?\b`), "table"},
		{regexp.MustCompile(`(?i)\b(?:image|photo|picture|screenshot)\b`), "image"},
		{regexp.MustCompile(`(?i)\b(?:bullets|narrative|commentary|text|comparison|insights?)\b`), "text"},
	}
)

// routeRest gives the facts of the rest of the brief to the outline items
// built to show them and returns what no item had room for.
func (o *briefOutline) routeRest(facts []briefFact) []string {
	placed := make([]bool, len(facts))
	place := func(want func(briefFact) bool, kinds ...string) {
		for _, kind := range kinds {
			for i := range o.items {
				it := &o.items[i]
				if it.def.kind != kind {
					continue
				}
				for fi, f := range facts {
					if len(it.facts)-1 >= it.def.capacity {
						break
					}
					if placed[fi] || !want(f) {
						continue
					}
					it.facts = append(it.facts, f.text)
					placed[fi] = true
				}
			}
		}
	}
	place(func(f briefFact) bool { return f.series }, "chart_insight", "regions")
	place(func(f briefFact) bool { return f.option }, "option_matrix", "comparison", "decision")
	place(func(f briefFact) bool { return f.numeric }, "kpi_snapshot", "chart_insight", "regions", "executive_summary", "image_case")
	place(func(f briefFact) bool { return f.risk }, "table", "risk_heatmap", "matrix_2x2")
	place(func(f briefFact) bool { return f.ask || f.recommend }, "decision", "next_steps")
	place(func(f briefFact) bool { return f.dated && !f.action }, "timeline", "roadmap")
	place(func(f briefFact) bool { return f.action }, "next_steps", "process")
	place(func(f briefFact) bool { return f.quote }, "quote")
	place(func(f briefFact) bool { return true }, "executive_summary", "pillars")
	unplaced := []string{}
	for fi, f := range facts {
		if !placed[fi] {
			unplaced = append(unplaced, f.text)
		}
	}
	return unplaced
}
