package deckplan

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// FormatDeckSpec is the plan_deck format that drafts a semantic DeckSpec
// instead of raw pattern skeletons.
const FormatDeckSpec = "deckspec"

// DeckSpecPlan is plan_deck's format:"deckspec" response: a DeckSpec draft
// whose slides follow a consulting narrative routed from the brief (answer
// first, then the situation, its drivers, the options, the plan, the ask),
// plus per-slide slot guidance and the brief facts routed to each slot.
//
// The raw plan chooses a *pattern* per slide and splits the brief into seeds;
// an agent on the default DeckSpec path then had to translate patterns back
// into kinds and reassemble the story from fragments (go-slide-creator-waft9).
// This plan speaks the DeckSpec's own vocabulary: a kind per narrative slot,
// so the draft is edited in place and sent to validate_deck_spec.
//
// The storyline is drafted from the brief's signals (go-slide-creator-khzni):
// an option evaluation gets an option_matrix, a chart is drafted only for a
// fact that describes change over time, a cause slide only when the brief
// names a problem, a decision only when it asks for one, and a customer update
// that reports shipped work gets highlights and a dated timeline instead of a
// problem/cause/decision arc. A budget of 8+ slides is drafted in chapters
// (structure.sections with an auto agenda and dividers).
type DeckSpecPlan struct {
	Format      string `json:"format"`
	Brief       string `json:"brief"`
	SlideBudget int    `json:"slide_budget"`
	// DeckSpec is the draft: meta plus either slides[] (a flat deck) or
	// structure (cover, auto agenda, sections, closing), each slide with kind
	// and a __FILL__ title. Every other field of each kind comes from
	// list_slide_kinds.
	DeckSpec DeckSpecDraft `json:"deck_spec"`
	// Slots explains each authored draft slide in storyline order: its
	// narrative slot, what the slide must argue, and the brief facts routed to
	// it. Generated agenda and divider slides have no slot.
	Slots []DeckSpecSlot `json:"slots"`
	// UnplacedFacts lists brief facts no slot had room for. Always present.
	UnplacedFacts []string `json:"unplaced_facts"`
	// BudgetNote explains a draft shorter than slide_budget: the narrative
	// is not padded with repeated evidence slides.
	BudgetNote          string `json:"budget_note,omitempty"`
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`
}

// DeckSpecDraft is the DeckSpec skeleton: meta and either slides or structure
// in DeckSpec shape.
type DeckSpecDraft struct {
	Meta      map[string]any          `json:"meta"`
	Slides    []map[string]any        `json:"slides,omitempty"`
	Structure *DeckSpecStructureDraft `json:"structure,omitempty"`
}

// DeckSpecStructureDraft is the chapter form of the draft: DeckSpec's
// structure block. auto_agenda generates the contents page and each section
// generates its divider, so both count toward the slide budget.
type DeckSpecStructureDraft struct {
	Cover      map[string]any         `json:"cover,omitempty"`
	AutoAgenda bool                   `json:"auto_agenda"`
	Sections   []DeckSpecSectionDraft `json:"sections"`
	Closing    map[string]any         `json:"closing,omitempty"`
}

// DeckSpecSectionDraft is one chapter of the draft.
type DeckSpecSectionDraft struct {
	Title  string           `json:"title"`
	Slides []map[string]any `json:"slides"`
}

// DeckSpecSlot annotates one draft slide.
type DeckSpecSlot struct {
	SlideIndex int `json:"slide_index"`
	// Path is where the slide lives in deck_spec: "slides[3]" for a flat
	// draft, "structure.sections[1].slides[0]" / "structure.cover" /
	// "structure.closing" for a chaptered one.
	Path     string   `json:"path"`
	Slot     string   `json:"slot"`
	Kind     string   `json:"kind"`
	Section  string   `json:"section,omitempty"`
	Guidance string   `json:"guidance"`
	Facts    []string `json:"facts,omitempty"`
}

// deckSpecSlotDef is one narrative slot of the storyline.
type deckSpecSlotDef struct {
	slot     string
	kind     string
	guidance string
	capacity int // brief facts the slot can carry
	// chapter groups the slot into a section of a chaptered draft.
	chapter string
}

// Chapters of a chaptered draft, in deck order.
const (
	chapterSituation = "situation"
	chapterDiagnosis = "diagnosis"
	chapterOptions   = "options"
	chapterPlan      = "plan"
)

// maxEvidenceSlots caps chart evidence slides: one per fact that describes
// change over time, never more than two, so the draft never carries a run of
// three charts.
const maxEvidenceSlots = 2

// briefSignals are the storyline cues read from the brief.
type briefSignals struct {
	metrics   int  // facts a KPI card or stat can carry
	series    int  // metrics that describe change over time
	dated     int  // dated milestone facts that are not to-dos
	actions   int  // to-do facts
	options   bool // the brief weighs options against criteria
	decision  bool // the brief asks for a decision
	problem   bool // the brief names a problem
	shipped   bool // the brief reports delivered work
	plan      bool // the brief carries a plan or priorities
	process   bool // the plan is a sequence of steps
	phases    bool // the brief carries a roadmap, phases or workstreams
	phased    bool // the brief names phases or workstreams outright
	customers bool // customer-facing deck
}

var (
	cueOptions   = regexp.MustCompile(`(?i)\b(?:two|three|four|five|\d)\s+(?:options|alternatives|scenarios|paths)\b|\b(?:options|alternatives|scenarios)\b[^.;]*\b(?:evaluated|assessed|scored|compared|against|criteria)\b|\bbuild,?\s+(?:vs\.?\s+|or\s+)?buy\b`)
	cueDecision  = regexp.MustCompile(`(?i)\b(?:approve|approval|decide|decision|recommend\w*|propos\w*|go/no-go|sign[- ]off|green[- ]light|funding request|asks?\s+(?:the|for))\b`)
	cueShould    = regexp.MustCompile(`\b[Ss]hould (?:we|[A-Z][a-z]+)\b`)
	cueProblem   = regexp.MustCompile(`(?i)\b(?:problem|issues?|declin\w*|dropp?\w*|fell|falling|rose|rising|at risk|slipp?\w*|missed|missing|behind|gaps?|challeng\w*|churn|loss\w*|threat\w*|pressure|shortfall|underperform\w*|why)\b`)
	cueShipped   = regexp.MustCompile(`(?i)\b(?:shipped|launched|released|delivered|introduced|went live|rolled out|now available|new features?)\b`)
	cuePlan      = regexp.MustCompile(`(?i)\b(?:plan|priorities|initiatives|next (?:quarter|year|steps)|we will|rollout|roll out|implementation|programme|program|workstreams?)\b`)
	cueProcess   = regexp.MustCompile(`(?i)\b(?:steps?|process|sequence|workflow|stages?|onboarding flow|first,? then)\b`)
	cuePhased    = regexp.MustCompile(`(?i)\b(?:phases?|workstreams?|waves?)\b`)
	cueCustomers = regexp.MustCompile(`(?i)\b(?:customers?|clients?|users|partners)\b`)
)

func readBriefSignals(brief, audience string, facts []briefFact) briefSignals {
	s := briefSignals{
		options:  cueOptions.MatchString(brief) || briefHasComparisonMatrix(brief),
		decision: cueDecision.MatchString(brief) || cueShould.MatchString(brief),
		problem:  cueProblem.MatchString(brief),
		shipped:  cueShipped.MatchString(brief),
		plan:     cuePlan.MatchString(brief),
		process:  cueProcess.MatchString(brief),
		phases:   briefHasPhaseSequence(brief),
		phased:   cuePhased.MatchString(brief),
	}
	// A customer-facing deck is one whose audience — or opening clause — names
	// customers; "churn among SMB customers" in a board brief is not.
	head := brief
	if clauses := splitBriefClauses(brief); len(clauses) > 0 {
		head = clauses[0]
	}
	s.customers = cueCustomers.MatchString(audience) || cueCustomers.MatchString(head)
	for _, f := range facts {
		switch {
		case f.numeric:
			s.metrics++
			if f.series {
				s.series++
			}
		case f.action:
			s.actions++
		case f.dated:
			s.dated++
		}
	}
	if s.customers && !factAsk.MatchString(brief) {
		// A customer update asks nothing of its audience: no decision slide,
		// whatever "recommend" or "should" appears in the prose.
		s.decision = false
	}
	return s
}

// storylineSlots drafts the narrative for the brief: every admitted slot in
// narrative order, and each slot's admission priority (lower is kept first
// when the budget is short).
func storylineSlots(sig briefSignals) (ordered []deckSpecSlotDef, priority map[string]int) {
	priority = map[string]int{}
	add := func(rank int, def deckSpecSlotDef) {
		ordered = append(ordered, def)
		if _, ok := priority[def.slot]; !ok {
			priority[def.slot] = rank
		}
	}
	pillarsUsed := false

	add(0, deckSpecSlotDef{"cover", "title", "Deck title and a subtitle naming audience and date.", 0, ""})
	add(3, deckSpecSlotDef{"answer", "executive_summary", "The whole answer up front: 3-5 points, each a conclusion a later slide proves; the title is the single-sentence answer.", 3, chapterSituation})

	// The situation, in the brief's own numbers.
	contextSlot, contextGuide := "context", "Where things stand, in the brief's own numbers; the title states the headline as a sentence carrying its number."
	if sig.problem {
		contextSlot, contextGuide = "problem", "What is wrong or at stake, shown with the brief's own numbers; the title states the problem as a sentence carrying its number."
	}
	switch {
	case sig.metrics >= 2:
		add(4, deckSpecSlotDef{contextSlot, "kpi_snapshot", contextGuide, 6, chapterSituation})
	case sig.metrics == 1:
		add(4, deckSpecSlotDef{contextSlot, "stat", contextGuide, 1, chapterSituation})
	case sig.problem:
		add(4, deckSpecSlotDef{"problem", "comparison", "What is wrong or at stake: where we are against where we need to be; the title states the gap as a sentence.", 2, chapterSituation})
	}

	if sig.shipped {
		pillarsUsed = true
		add(6, deckSpecSlotDef{"highlights", "pillars", "What was delivered: 3-5 items, each with what it changes for the audience; the title says what the release adds up to.", 3, chapterSituation})
	}

	// Charts only for facts that describe change over time — a chart drawn
	// from anything else needs invented data.
	evidence := min(sig.series, maxEvidenceSlots)
	for i := 0; i < evidence; i++ {
		add(7, deckSpecSlotDef{"evidence", "chart_insight", "One chart that proves one claim: the title states the claim with its number; set takeaway and source; never invent data.", 1, chapterSituation})
	}

	if sig.problem && !sig.customers {
		kind := "pillars"
		if pillarsUsed {
			kind = "table"
		}
		pillarsUsed = pillarsUsed || kind == "pillars"
		add(8, deckSpecSlotDef{"cause", kind, "Why it is happening: 3-5 drivers, each with its evidence.", 2, chapterDiagnosis})
	}

	if sig.options {
		add(5, deckSpecSlotDef{"options", "option_matrix", "The options scored against the same criteria, with the recommended option highlighted; the title names the winner and why.", 3, chapterOptions})
	}

	if sig.plan && (sig.actions > 0 || sig.process) {
		kind, guide := "pillars", "What we will do: 3-5 priorities, each an action with its owner and outcome."
		switch {
		case sig.process:
			kind, guide = "process", "What we will do: 3-6 steps in order, each an action with its outcome."
		case pillarsUsed:
			kind, guide = "table", "What we will do: one row per priority with its owner, outcome and date."
		}
		add(9, deckSpecSlotDef{"plan", kind, guide, 3, chapterPlan})
	}

	// Dated milestones without phases are a timeline; phases (or a roadmap
	// with nothing dated) are a roadmap.
	switch {
	case sig.dated >= 2 && !sig.phased:
		add(10, deckSpecSlotDef{"roadmap", "timeline", "When it happens: the dated milestones from the brief, in order; the title says what the sequence delivers.", 4, chapterPlan})
	case sig.phases:
		add(10, deckSpecSlotDef{"roadmap", "roadmap", "When it happens: 3-6 phases with dates and the milestone that ends each.", 4, chapterPlan})
	}

	if sig.decision {
		add(2, deckSpecSlotDef{"ask", "decision", "The decision needed now: the options, the recommended one, and the ask with owner and date.", 2, chapterPlan})
	}

	add(1, deckSpecSlotDef{"closing", "next_steps", "Close on next steps, not \"Thank you\": 2-6 actions, each with an owner and a date, and the decisions requested. A plain closing kind stays available for a Q&A page.", 4, ""})
	return ordered, priority
}

// fitToBudget drops the lowest-priority slots until the storyline fits n
// slides, keeping narrative order. The second evidence slide ranks below the
// first.
func fitToBudget(ordered []deckSpecSlotDef, priority map[string]int, n int) []deckSpecSlotDef {
	type ranked struct {
		def  deckSpecSlotDef
		rank int
	}
	items := make([]ranked, len(ordered))
	seen := map[string]int{}
	for i, d := range ordered {
		items[i] = ranked{d, priority[d.slot]*10 + seen[d.slot]*25}
		seen[d.slot]++
	}
	for len(items) > n && len(items) > 0 {
		worst := 0
		for i := range items {
			if items[i].rank >= items[worst].rank {
				worst = i
			}
		}
		items = append(items[:worst], items[worst+1:]...)
	}
	out := make([]deckSpecSlotDef, len(items))
	for i := range items {
		out[i] = items[i].def
	}
	return out
}

// draftSection is a chapter of the draft with its slots.
type draftSection struct {
	chapters []string
	slots    []int // indices into the ordered storyline
}

// sectionTitle names a chapter from what it holds. The title is a draft for
// the agenda; the guidance asks the agent to make it the chapter's message.
func sectionTitle(sec draftSection, defs []deckSpecSlotDef) string {
	has := map[string]bool{}
	for _, c := range sec.chapters {
		has[c] = true
	}
	slot := map[string]bool{}
	for _, i := range sec.slots {
		slot[defs[i].slot] = true
	}
	switch {
	case has[chapterOptions] && slot["ask"]:
		return "Options and recommendation"
	case has[chapterOptions]:
		return "Options"
	case has[chapterSituation] && has[chapterDiagnosis]:
		return "Where we stand and why"
	case has[chapterSituation] && slot["highlights"] && !slot["problem"]:
		return "What we delivered"
	case has[chapterSituation]:
		return "Where we stand"
	case has[chapterDiagnosis]:
		return "What is driving it"
	case slot["ask"]:
		return "Recommendation and plan"
	default:
		return "The way forward"
	}
}

// chapterSections groups the body slots (everything but cover and closing)
// into sections by chapter, merging single-slide chapters into a neighbour
// while more than two sections remain.
func chapterSections(defs []deckSpecSlotDef) []draftSection {
	var secs []draftSection
	for i, d := range defs {
		if d.chapter == "" {
			continue
		}
		if n := len(secs); n > 0 && secs[n-1].chapters[len(secs[n-1].chapters)-1] == d.chapter {
			secs[n-1].slots = append(secs[n-1].slots, i)
			continue
		}
		secs = append(secs, draftSection{chapters: []string{d.chapter}, slots: []int{i}})
	}
	for len(secs) > 2 {
		merged := false
		for i := range secs {
			if len(secs[i].slots) != 1 {
				continue
			}
			// Diagnosis and situation read as one chapter; options join the
			// plan they lead to.
			into := i - 1
			if into < 0 || (secs[i].chapters[0] == chapterOptions && i+1 < len(secs)) {
				into = i + 1
			}
			a, b := min(i, into), max(i, into)
			secs[a].chapters = append(secs[a].chapters, secs[b].chapters...)
			secs[a].slots = append(secs[a].slots, secs[b].slots...)
			secs = append(secs[:b], secs[b+1:]...)
			merged = true
			break
		}
		if !merged {
			break
		}
	}
	return secs
}

// structuredSlideCount is the rendered length of a chaptered draft: cover,
// generated agenda, one divider per section, the body slides, and closing.
func structuredSlideCount(defs []deckSpecSlotDef, secs []draftSection) int {
	return len(defs) + 1 + len(secs)
}

// minChapterBudget is the smallest budget drafted in chapters: below it the
// agenda and dividers would crowd out the content.
const minChapterBudget = 8

// chapterDraft decides whether a storyline is drafted in chapters. A storyline
// that fills minChapterBudget slides on its own is always chaptered — its
// lowest-priority body slides give way to the agenda and dividers, never the
// last slide of a chapter. A shorter storyline is chaptered only when the
// agenda and dividers fit in the budget's spare room: chapters must not cost a
// small deck its evidence. Returns the (possibly trimmed) storyline and its
// sections, or no sections for a flat draft.
func chapterDraft(ordered []deckSpecSlotDef, priority map[string]int, budget int) ([]deckSpecSlotDef, []draftSection) {
	mustChapter := len(ordered) >= minChapterBudget
	cand := ordered
	for {
		secs := chapterSections(cand)
		if len(secs) < 2 {
			return ordered, nil
		}
		if structuredSlideCount(cand, secs) <= budget {
			return cand, secs
		}
		if !mustChapter {
			return ordered, nil
		}
		drop := -1
		for _, sec := range secs {
			if len(sec.slots) < 2 {
				continue
			}
			for _, i := range sec.slots {
				if drop < 0 || priority[cand[i].slot] >= priority[cand[drop].slot] {
					drop = i
				}
			}
		}
		if drop < 0 {
			return ordered, nil
		}
		next := make([]deckSpecSlotDef, 0, len(cand)-1)
		next = append(next, cand[:drop]...)
		cand = append(next, cand[drop+1:]...)
	}
}

// BuildDeckSpecPlan drafts a DeckSpec storyline for the brief within the
// slide budget. See DeckSpecPlan for how the brief shapes the storyline.
func BuildDeckSpecPlan(p Params) *DeckSpecPlan {
	budget := p.SlideBudget
	facts := extractBriefFacts(p.Brief)
	sig := readBriefSignals(p.Brief, p.Audience, facts)
	full, priority := storylineSlots(sig)

	ordered := fitToBudget(full, priority, budget)
	var secs []draftSection
	if budget >= minChapterBudget {
		ordered, secs = chapterDraft(ordered, priority, budget)
	}

	slots := make([]DeckSpecSlot, len(ordered))
	for i, s := range ordered {
		slots[i] = DeckSpecSlot{SlideIndex: i, Slot: s.slot, Kind: s.kind, Guidance: s.guidance}
	}
	unplaced := routeDeckSpecFacts(ordered, slots, facts)

	topic := deckTopic(p.Brief)
	meta := map[string]any{"title": topic, "date": patterns.FillPlaceholder}
	if p.TemplateName != "" {
		meta["template"] = p.TemplateName
	}
	if p.Audience != "" {
		meta["audience"] = p.Audience
	}
	// Consulting chrome (go-slide-creator-1iy0x): page numbers on every slide
	// but the title and closing, meta.date in the footer, and the section
	// tracker on a chaptered deck. Written out so the agent sees — and can
	// edit — the furniture the deck renders with.
	chrome := map[string]any{"page_numbers": map[string]any{"enabled": true}}
	if len(secs) > 0 {
		chrome["tracker"] = true
	}
	meta["chrome"] = chrome

	slideFor := func(s deckSpecSlotDef) map[string]any {
		slide := map[string]any{"kind": s.kind, "title": patterns.FillPlaceholder}
		if s.slot == "cover" {
			slide["title"] = topic
			slide["subtitle"] = patterns.FillPlaceholder
		}
		return slide
	}

	draft := DeckSpecDraft{Meta: meta}
	rendered := len(ordered)
	if len(secs) == 0 {
		draft.Slides = make([]map[string]any, len(ordered))
		for i, s := range ordered {
			draft.Slides[i] = slideFor(s)
			slots[i].Path = fmt.Sprintf("slides[%d]", i)
		}
	} else {
		st := &DeckSpecStructureDraft{AutoAgenda: true}
		for i, s := range ordered {
			switch s.slot {
			case "cover":
				st.Cover = slideFor(s)
				slots[i].Path = "structure.cover"
			case "closing":
				st.Closing = slideFor(s)
				slots[i].Path = "structure.closing"
			}
		}
		for si, sec := range secs {
			title := sectionTitle(sec, ordered)
			section := DeckSpecSectionDraft{Title: title}
			for j, idx := range sec.slots {
				section.Slides = append(section.Slides, slideFor(ordered[idx]))
				slots[idx].Path = fmt.Sprintf("structure.sections[%d].slides[%d]", si, j)
				slots[idx].Section = title
			}
			st.Sections = append(st.Sections, section)
		}
		draft.Structure = st
		rendered = structuredSlideCount(ordered, secs)
	}

	plan := &DeckSpecPlan{
		Format:        FormatDeckSpec,
		Brief:         p.Brief,
		SlideBudget:   budget,
		DeckSpec:      draft,
		Slots:         slots,
		UnplacedFacts: unplaced,
	}
	if rendered < budget {
		plan.BudgetNote = fmt.Sprintf("The storyline needs %d of the %d slides: it is not padded with repeated evidence slides. Add a slide only for a claim the brief can prove.", rendered, budget)
	}
	return plan
}

// routeDeckSpecFacts assigns brief facts to slots by what each fact is
// (go-slide-creator-gvbw8): change-over-time metrics to chart evidence,
// option metrics to the option matrix, other metrics to the KPI / stat slide,
// asks to the decision, dated milestones to the roadmap, to-dos to the plan
// and next steps, and everything else in brief order to the answer, cause,
// highlights, options and plan. A to-do never lands on a KPI card. It returns
// what no slot had room for.
func routeDeckSpecFacts(defs []deckSpecSlotDef, slots []DeckSpecSlot, facts []briefFact) []string {
	placed := make([]bool, len(facts))
	place := func(want func(briefFact) bool, slotNames ...string) {
		for _, name := range slotNames {
			for i := range slots {
				if defs[i].slot != name {
					continue
				}
				for fi, f := range facts {
					if len(slots[i].Facts) >= defs[i].capacity {
						break
					}
					if placed[fi] || !want(f) {
						continue
					}
					slots[i].Facts = append(slots[i].Facts, f.text)
					placed[fi] = true
				}
			}
		}
	}
	place(func(f briefFact) bool { return f.series }, "evidence")
	place(func(f briefFact) bool { return f.numeric && f.option }, "options")
	place(func(f briefFact) bool { return f.numeric }, "problem", "context", "evidence", "options", "answer")
	place(func(f briefFact) bool { return f.ask }, "ask", "closing")
	place(func(f briefFact) bool { return f.dated && !f.action }, "roadmap")
	place(func(f briefFact) bool { return f.option }, "options")
	place(func(f briefFact) bool { return f.action }, "plan", "closing", "ask")
	place(func(f briefFact) bool { return f.dated }, "roadmap", "plan", "closing")
	place(func(briefFact) bool { return true }, "answer", "highlights", "cause", "options", "plan")
	unplaced := []string{}
	for fi, f := range facts {
		if !placed[fi] {
			unplaced = append(unplaced, f.text)
		}
	}
	return unplaced
}

// deckTopic is the brief's first clause, used as the draft deck title.
func deckTopic(brief string) string {
	first := strings.TrimSpace(brief)
	if parts := splitBriefClauses(first); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		first = strings.TrimSpace(parts[0])
	}
	return TruncateBrief(first, 80)
}
