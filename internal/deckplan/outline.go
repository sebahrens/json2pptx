package deckplan

import (
	"fmt"
	"regexp"
	"strings"
)

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
)

var outlineRules = []outlineRule{
	{regexp.MustCompile(`(?i)^(?:a\s+|the\s+)?(?:title|cover)(?:\s+(?:slide|page))?$`), outlineCover},
	{regexp.MustCompile(`(?i)\b(?:agenda|table of contents|contents page)\b`), outlineAgenda},
	{regexp.MustCompile(`(?i)\b(?:executive summary|exec summary|summary|overview|headlines?|key messages?|tl;?dr)\b`), outlineAnswer},
	{regexp.MustCompile(`(?i)\b(?:next steps?|action plan|actions|call to action|way forward|wrap[- ]up|q&a)\b`), outlineClosing},
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
}

// outlineKindFor picks the slide an outline item names: a cue word in the item
// first, then what the item is as a fact (a series is a chart, a single number
// a stat), else a points slide.
func outlineKindFor(text string) outlineKind {
	f := classifyFact(text, factQuantity.MatchString(text))
	if f.list {
		return outlineChart
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
	outlineHeader = regexp.MustCompile(`(?i)\b(?:slides?|outline|agenda|sections?|structure|storyline|story ?line|flow|chapters|topics|covering|covers?|order)\b`)

	// outlineLineItem matches a list line: a bullet, a number, or "Slide 3:".
	outlineLineItem = regexp.MustCompile(`(?i)^\s*(?:[-*•▪·–—]|\(?\d{1,2}[.)]|slide\s+\d{1,2}\s*[:.)–—-])\s*`)

	// outlineSlideLine matches a line that numbers its slide outright.
	outlineSlideLine = regexp.MustCompile(`(?i)^\s*slide\s+\d{1,2}\b`)

	// outlineSentenceEnd matches the end of a sentence inside running text.
	outlineSentenceEnd = regexp.MustCompile(`[.!?](?:\s+|$)|\n`)

	// outlineInlineColon matches a colon that opens an inline list.
	outlineInlineColon = regexp.MustCompile(`:\s+`)

	outlineLeadingAnd = regexp.MustCompile(`(?i)^(?:and|then|finally)\s+`)
)

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
	labelled := strings.HasSuffix(label, ":") && outlineHeader.MatchString(label) && !isFillerLabel(strings.TrimSuffix(label, ":"))
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
		labelled := !isTopic && outlineHeader.MatchString(label)
		if isTopic && outlineHeader.MatchString(label) && len(strings.Fields(label)) <= 4 {
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
	return strings.TrimSpace(outlineLeadingAnd.ReplaceAllString(s, ""))
}

// outlineCountMatches reports whether the list is as long as the stated slide
// count allows — the count itself, or the count less a cover and a close — and
// is not a list of figures.
func outlineCountMatches(items []string, stated int) bool {
	if stated <= 0 || len(items) > stated || len(items) < stated-2 {
		return false
	}
	figures := 0
	for _, it := range items {
		if classifyFact(it, factQuantity.MatchString(it)).numeric {
			figures++
		}
	}
	return 2*figures <= len(items)
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
		o.items = append(o.items, outlineItem{text: t, def: def, facts: []string{t}})
	}
	if len(o.items) < minOutlineItems {
		return nil
	}
	return o
}

// endsOnClose reports whether the outline's last item is the deck's close.
func (o *briefOutline) endsOnClose() bool {
	last := o.items[len(o.items)-1].def
	return last.kind == "next_steps" || last.kind == "decision"
}

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
	place(func(f briefFact) bool { return f.series }, "chart_insight")
	place(func(f briefFact) bool { return f.option }, "option_matrix", "comparison")
	place(func(f briefFact) bool { return f.numeric }, "kpi_snapshot", "chart_insight", "executive_summary")
	place(func(f briefFact) bool { return f.risk }, "table")
	place(func(f briefFact) bool { return f.ask || f.recommend }, "decision", "next_steps")
	place(func(f briefFact) bool { return f.dated && !f.action }, "timeline", "roadmap")
	place(func(f briefFact) bool { return f.action }, "next_steps", "process")
	place(func(f briefFact) bool { return f.quote }, "quote")
	unplaced := []string{}
	for fi, f := range facts {
		if !placed[fi] {
			unplaced = append(unplaced, f.text)
		}
	}
	return unplaced
}
