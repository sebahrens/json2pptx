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
// problem/cause/decision arc. A brief that enumerates its slides ("7-slide
// pitch: problem, solution, market size chart, …") is drafted one slot per
// listed item, in the brief's order (go-slide-creator-hf8tf).
//
// The budget counts content slides first (go-slide-creator-58qda): a draft is
// chaptered (structure.sections with an auto agenda and dividers) only at
// chapterBudget slides or more, or when the brief asks for an agenda or
// dividers, and budget / budget_note always say how the budget was spent.
type DeckSpecPlan struct {
	Format string `json:"format"`
	Brief  string `json:"brief"`
	// SlideBudget is the budget the draft was made to: the slide_budget
	// argument, else the slide count the brief states, else the default.
	SlideBudget int `json:"slide_budget"`
	// Constraints lists the instructions about the deck itself that the brief
	// states (slide count, template, audience, duration, agenda / dividers).
	// They are not facts and reach no slot.
	Constraints []Constraint `json:"constraints,omitempty"`
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
	// Every clause of the brief after its topic is in a slot's facts or here.
	UnplacedFacts []string `json:"unplaced_facts"`
	// UnsupportedRegions lists same-slide region requirements the draft could
	// not honour as asked (an unknown visual, or positions outside the
	// supported arrangements), each with the reason (go-slide-creator-vae7f).
	UnsupportedRegions []UnsupportedRegion `json:"unsupported_regions,omitempty"`
	// Budget is how the draft spent the slide budget: content and structural
	// slides, and what was cut. Always present.
	Budget Budget `json:"budget"`
	// BudgetNote says the same in one line, and explains a draft shorter or
	// longer than slide_budget. Always present.
	BudgetNote          string `json:"budget_note"`
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
	// Appendix marks the back-matter section: an unnumbered divider, left out
	// of the auto agenda, its slides' running section reading "Appendix", and
	// exempt from the deck-rhythm run checks.
	Appendix bool `json:"appendix,omitempty"`
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
	// Appendix is true for back-matter slots (the appendix divider of a flat
	// draft and the backup slides): reference pages after the close, not part
	// of the argument.
	Appendix bool `json:"appendix,omitempty"`
	// Regions describes the drafted regions of a mixed-region slot (kind
	// regions): each region's path, position, role, kind and facts.
	Regions []RegionSlot `json:"regions,omitempty"`
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
	risks     int  // facts that name a risk
	options   bool // the brief weighs options against criteria
	compare   bool // the brief asks outright to compare two alternatives
	decision  bool // the brief asks for a decision
	problem   bool // the brief names a problem
	shipped   bool // the brief reports delivered work
	plan      bool // the brief carries a plan or priorities
	process   bool // the plan is a sequence of steps
	phases    bool // the brief carries a roadmap, phases or workstreams
	phased    bool // the brief names phases or workstreams outright
	customers bool // customer-facing deck
	backup    bool // the brief asks for backup / detail material
	method    bool // the brief carries a methodology or assumptions
}

var (
	cueOptions   = regexp.MustCompile(`(?i)\b(?:two|three|four|five|\d)\s+(?:options|alternatives|scenarios|paths)\b|\b(?:options|alternatives|scenarios)\b[^.;]*\b(?:evaluated|assessed|scored|compared|against|criteria)\b|\bbuild,?\s+(?:vs\.?\s+|or\s+)?buy\b`)
	cueDecision  = regexp.MustCompile(`(?i)\b(?:approve|approval|decide|decision|recommend\w*|propos\w*|go/no-go|sign[- ]off|green[- ]light|funding request|asks?\s+(?:the|for))\b`)
	cueShould    = regexp.MustCompile(`\b[Ss]hould (?:we|[A-Z][a-z]+)\b`)
	cueProblem   = regexp.MustCompile(`(?i)\b(?:problem|issues?|declin\w*|dropp?\w*|fell|falling|rose|rising|at risk|slipp?\w*|missed|missing|behind|gaps?|challeng\w*|churn|loss\w*|threat\w*|pressure|shortfall|underperform\w*|why)\b`)
	cueShipped   = regexp.MustCompile(`(?i)\b(?:shipped|launched|released|delivered|introduced|went live|rolled out|now available|new features?)\b`)
	cuePlan      = regexp.MustCompile(`(?i)\b(?:plan|priorities|initiatives|next (?:quarter|year|steps)|we will|rollout|roll out|implementation|programme|program|workstreams?)\b`)
	cueProcess   = regexp.MustCompile(`(?i)\b(?:steps?|process|sequence|workflow|stages?|onboarding flow|first,? then)\b`)
	cueNextSteps = regexp.MustCompile(`(?i)\bnext steps?\b`)
	cuePhased    = regexp.MustCompile(`(?i)\b(?:phases?|workstreams?|waves?)\b`)
	cueCustomers = regexp.MustCompile(`(?i)\b(?:customers?|clients?|users|partners)\b`)
	cueBackup    = regexp.MustCompile(`(?i)\b(?:appendix|appendices|backup|back-up|supporting (?:data|detail|analysis|material)|detailed (?:data|figures|financials|breakdown|analysis)|deep[- ]dives?|breakdowns?|data tables?|reference material)\b`)
	cueMethod    = regexp.MustCompile(`(?i)\b(?:methodology|method|assumptions?|approach to the analysis|data sources?)\b`)
)

// briefComparesAlternatives reports whether any clause of the brief asks to
// compare two alternatives ("Compare internal support (€0.8m per year) with
// outsourcing (€0.5m per year) on cost and launch speed"). cueOptions only
// knew enumerated options, options-vs-criteria and build/buy wording, so an
// explicit comparison drafted a single-stat slide instead of an option
// matrix (go-slide-creator-fu6uy). "Revenue compared with plan" is a
// benchmark, not a choice.
func briefComparesAlternatives(brief string) bool {
	for _, c := range splitBriefClauses(brief) {
		if comparesAlternatives(c) {
			return true
		}
	}
	return false
}

func readBriefSignals(brief, audience string, facts []briefFact) briefSignals {
	s := briefSignals{
		compare:  briefComparesAlternatives(brief),
		decision: cueDecision.MatchString(brief) || cueShould.MatchString(brief),
		problem:  cueProblem.MatchString(brief),
		shipped:  cueShipped.MatchString(brief),
		plan:     cuePlan.MatchString(brief),
		process:  cueProcess.MatchString(cueNextSteps.ReplaceAllString(brief, "")),
		phases:   briefHasPhaseSequence(brief),
		phased:   cuePhased.MatchString(brief),
		backup:   cueBackup.MatchString(brief),
		method:   cueMethod.MatchString(brief),
	}
	s.options = s.compare || cueOptions.MatchString(brief) || briefHasComparisonMatrix(brief)
	// A customer-facing deck is one whose audience — or opening clause — names
	// customers; "churn among SMB customers" in a board brief is not.
	head := brief
	if clauses := splitBriefClauses(brief); len(clauses) > 0 {
		head = clauses[0]
	}
	s.customers = cueCustomers.MatchString(audience) || cueCustomers.MatchString(head)
	for _, f := range facts {
		if f.risk && f.group == "" && !f.numeric {
			s.risks++
		}
		switch {
		case f.numeric && f.option && s.options:
			// The option matrix carries the options' own figures; counting
			// them as KPIs drafted a stat slide that stole the comparison
			// (go-slide-creator-fu6uy).
		case f.numeric:
			s.metrics++
			if f.series {
				s.series++
			}
		case f.action:
			// "Next steps with owners …" is the close; it does not make a
			// plan slide of its own.
			if !cueNextSteps.MatchString(f.text) {
				s.actions++
			}
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
func storylineSlots(sig briefSignals, regions bool) (ordered []deckSpecSlotDef, priority map[string]int) {
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

	// A slide the brief laid out in regions is the slide it asked for by
	// name: it ranks with the close, so a short budget keeps it
	// (go-slide-creator-vae7f). Its facts come from the region clauses, not
	// from routing.
	if regions {
		add(1, deckSpecSlotDef{"regions", "regions", regionsGuidance, 0, chapterSituation})
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
		// A brief that asks outright to compare two alternatives is about
		// that comparison: the matrix outranks the situation slide when the
		// budget is short (go-slide-creator-fu6uy).
		rank := 5
		if sig.compare {
			rank = 3
		}
		add(rank, deckSpecSlotDef{"options", "option_matrix", "The options scored against the same criteria, with the recommended option highlighted; the title names the winner and why.", 3, chapterOptions})
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

	// A risk the brief names gets a slide of its own: it was dropped with
	// unplaced_facts empty (go-slide-creator-hf8tf).
	if sig.risks > 0 {
		add(6, deckSpecSlotDef{"risks", "table", guideRisks, 3, chapterPlan})
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

// appendixTitle is the title of the drafted back-matter section / divider.
const appendixTitle = "Appendix"

// appendixSlots drafts the back matter (go-slide-creator-khzni): a consulting
// deck ends on its next steps and keeps the detail a reader may ask for in an
// appendix after them. Back matter is drafted only from spare budget — it never
// displaces a body slide — and only when there is backup material: the brief
// asks for backup / detail, carries a methodology or assumptions, has facts the
// body had no room for, or has enough figures (3+) for a detailed data table.
// room is the budget left after the body; the appendix divider costs one slide.
func appendixSlots(sig briefSignals, unplaced, room int) []deckSpecSlotDef {
	var defs []deckSpecSlotDef
	if sig.backup || unplaced > 0 || sig.metrics >= 3 {
		defs = append(defs, deckSpecSlotDef{"backup", "table",
			"Backup, not argument: the detailed figures behind the body's claims (or the facts the body had no room for) in one table with its source; the title says what the table shows. Drop it if the body already carries every number.",
			6, ""})
	}
	if sig.method {
		defs = append(defs, deckSpecSlotDef{"methodology", "table",
			"How the numbers were built: one row per assumption or data source, with its value and where it comes from.",
			4, ""})
	}
	if n := room - 1; len(defs) > n {
		defs = defs[:max(n, 0)]
	}
	return defs
}

// structuredSlideCount is the rendered length of a chaptered draft: cover,
// generated agenda, one divider per section, the body slides, and closing.
func structuredSlideCount(defs []deckSpecSlotDef, secs []draftSection) int {
	return len(defs) + 1 + len(secs)
}

// chapterDraft decides whether a storyline is drafted in chapters. The budget
// counts content slides first (go-slide-creator-58qda): a storyline is
// chaptered only when the agenda and dividers fit in the room the content left
// over. When the brief asks for an agenda or dividers, the lowest-priority
// body slides give way to them — never the last slide of a chapter. Returns
// the (possibly trimmed) storyline and its sections, or no sections for a flat
// draft.
func chapterDraft(ordered []deckSpecSlotDef, priority map[string]int, budget int, asked bool) ([]deckSpecSlotDef, []draftSection) {
	cand := ordered
	for {
		secs := chapterSections(cand)
		if len(secs) < 2 {
			return ordered, nil
		}
		if structuredSlideCount(cand, secs) <= budget {
			return cand, secs
		}
		if !asked {
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

// recordCuts lists the slots of before that are missing from after.
func recordCuts(account *Budget, before, after []deckSpecSlotDef, reason string) {
	j := 0
	for _, d := range before {
		if j < len(after) && after[j] == d {
			j++
			continue
		}
		account.cut(fmt.Sprintf("%s (%s)", d.slot, d.kind), reason)
	}
}

// outlineStatedCount is the slide count an outline is measured against: the
// count the brief states, else the budget the caller chose, else none.
func outlineStatedCount(p Params, cs []Constraint, budget int) int {
	if n := statedSlideCount(cs); n > 0 {
		return n
	}
	if p.BudgetExplicit {
		return budget
	}
	return 0
}

// outlineDeckSpecSlots drafts an enumerated outline: the cover, one slot per
// listed item in the brief's order, and a next-steps close when the outline
// does not end on one and the budget has room. The outline is kept whole even
// when it is longer than the budget; the budget account says so.
func outlineDeckSpecSlots(o *briefOutline, budget int, account *Budget) ([]deckSpecSlotDef, []DeckSpecSlot, []string, []string) {
	var sources, rest []briefFact
	for _, f := range extractBriefFacts(o.rest) {
		if f.source {
			sources = append(sources, f)
			continue
		}
		rest = append(rest, f)
	}
	unplaced := o.routeRest(rest)

	ordered := []deckSpecSlotDef{{"cover", "title", outlineCover.guidance, 0, ""}}
	for _, it := range o.items {
		ordered = append(ordered, deckSpecSlotDef{it.def.slot, it.def.kind, it.def.guidance, it.def.capacity, ""})
	}
	switch {
	case o.endsOnClose():
	case len(ordered)+1 <= budget:
		ordered = append(ordered, deckSpecSlotDef{"closing", "next_steps", guideClosing, 4, ""})
	default:
		account.cut("closing (next_steps)", fmt.Sprintf("the outline's %d items fill the budget", len(o.items)))
	}
	slots := make([]DeckSpecSlot, len(ordered))
	for i, d := range ordered {
		slots[i] = DeckSpecSlot{SlideIndex: i, Slot: d.slot, Kind: d.kind, Guidance: d.guidance}
		if i >= 1 && i <= len(o.items) {
			slots[i].Facts = o.items[i-1].facts
		}
	}
	return ordered, slots, unplaced, factTexts(sources)
}

// factTexts returns the facts' texts.
func factTexts(facts []briefFact) []string {
	out := make([]string, len(facts))
	for i, f := range facts {
		out[i] = f.text
	}
	return out
}

// sourceValue is what a "source: …" fact names.
func sourceValue(fact string) string {
	return strings.TrimSpace(factSource.ReplaceAllString(fact, ""))
}

// BuildDeckSpecPlan drafts a DeckSpec storyline for the brief within the
// slide budget. See DeckSpecPlan for how the brief shapes the storyline.
func BuildDeckSpecPlan(p Params) *DeckSpecPlan {
	// Instructions about the deck itself are constraints, not facts
	// (go-slide-creator-hf8tf).
	cleaned, constraints := parseConstraints(p.Brief)
	budget := effectiveBudget(p, constraints)
	audience := p.Audience
	if audience == "" {
		audience = constraintValue(constraints, ConstraintAudience)
	}
	chapters, chaptersAsked := chaptersAllowed(budget, constraints)
	account := newBudget(budget)

	// Region clauses ("left two-thirds a line chart …; upper right a KPI")
	// become one regions slide; the rest of the brief is planned as before
	// (go-slide-creator-vae7f).
	req := parseRegionRequest(cleaned)
	brief := cleaned
	if req != nil {
		brief = req.remainder
	}

	var outline *briefOutline
	if !req.draftable() {
		outline = parseOutline(brief, outlineStatedCount(p, constraints, budget))
	}
	var d specDraft
	topicBrief := cleaned
	if outline != nil {
		d.ordered, d.slots, d.unplaced, d.sources = outlineDeckSpecSlots(outline, budget, &account)
		topicBrief = outline.rest
	} else {
		d = storylineDraft(brief, audience, budget, req, chapters, chaptersAsked, &account)
	}

	topic := sentenceCase(deckTopic(topicBrief))
	// The cover carries the topic — and the source the brief names — so every
	// date and name in them is in a slot's facts.
	for i := range d.slots {
		if d.slots[i].Slot == "cover" {
			d.slots[i].Facts = append([]string{topic}, d.sources...)
		}
	}
	meta := map[string]any{"title": topic, "date": patterns.FillPlaceholder}
	if p.TemplateName != "" {
		meta["template"] = p.TemplateName
	}
	if audience != "" {
		meta["audience"] = audience
	}
	if len(d.sources) > 0 {
		meta["source"] = sourceValue(d.sources[0])
	}
	// Consulting chrome (go-slide-creator-1iy0x): page numbers on every slide
	// but the title and closing, meta.date in the footer, and the section
	// tracker on a chaptered deck. Written out so the agent sees — and can
	// edit — the furniture the deck renders with.
	chrome := map[string]any{"page_numbers": map[string]any{"enabled": true}}
	if len(d.secs) > 0 {
		chrome["tracker"] = true
	}
	meta["chrome"] = chrome

	slideFor := func(s deckSpecSlotDef) map[string]any {
		if s.slot == "regions" {
			return req.draftBody()
		}
		slide := map[string]any{"kind": s.kind, "title": patterns.FillPlaceholder}
		if s.slot == "cover" {
			slide["title"] = topic
			slide["subtitle"] = patterns.FillPlaceholder
		}
		return slide
	}

	rendered := len(d.ordered)
	if len(d.secs) > 0 {
		rendered = structuredSlideCount(d.ordered, d.secs)
	}
	// Back matter needs an appendix divider: it is drafted where chapters are
	// allowed, or when the brief asks for backup material outright.
	var backMatter []deckSpecSlotDef
	if outline == nil && (chapters || d.sig.backup) {
		backMatter = appendixSlots(d.sig, len(d.unplaced), budget-rendered)
	}
	if len(backMatter) > 0 {
		d.unplaced = routeAppendixFacts(backMatter, &d.slots, d.unplaced)
		rendered += 1 + len(backMatter)
	}

	draft := DeckSpecDraft{Meta: meta}
	if len(d.secs) == 0 {
		draft.Slides = draftFlat(d.ordered, backMatter, &d.slots, slideFor)
	} else {
		draft.Structure = draftStructure(d.ordered, d.secs, backMatter, d.slots, slideFor)
	}

	for i := range d.slots {
		if d.slots[i].Slot == "regions" {
			path := d.slots[i].Path
			d.slots[i].Regions = req.regionSlots(func(k int) string { return fmt.Sprintf("%s.regions[%d]", path, k) })
		}
	}

	account.Planned = rendered
	accountDeckSpec(&account, d.ordered, draft, len(backMatter) > 0)

	plan := &DeckSpecPlan{
		Format:        FormatDeckSpec,
		Brief:         p.Brief,
		SlideBudget:   budget,
		Constraints:   constraints,
		DeckSpec:      draft,
		Slots:         d.slots,
		UnplacedFacts: d.unplaced,
		Budget:        account,
		BudgetNote:    account.note(deckSpecBudgetNotes(account, outline, chapters, len(d.unplaced))...),
	}
	if req != nil {
		plan.UnsupportedRegions = req.unsupported
	}
	return plan
}

// specDraft is a drafted storyline before it is laid out: its slots in order,
// its chapters (none for a flat draft), the facts routed to each slot, and
// what was left over.
type specDraft struct {
	ordered  []deckSpecSlotDef
	secs     []draftSection
	slots    []DeckSpecSlot
	unplaced []string
	sources  []string // the "source: …" facts, carried by the cover
	sig      briefSignals
}

// storylineDraft drafts the consulting storyline for a brief that does not
// enumerate its slides: the slots its signals admit, fitted to the budget —
// content first — with the brief's facts routed to them. What the budget
// leaves out is recorded in the account.
func storylineDraft(brief, audience string, budget int, req *regionRequest, chapters, chaptersAsked bool, account *Budget) specDraft {
	facts := extractBriefFacts(brief)
	d := specDraft{sig: readBriefSignals(brief, audience, facts)}
	full, priority := storylineSlots(d.sig, req.draftable())

	d.ordered = fitToBudget(full, priority, budget)
	recordCuts(account, full, d.ordered, fmt.Sprintf("lowest priority in a budget of %d", budget))
	if chapters {
		fitted := d.ordered
		d.ordered, d.secs = chapterDraft(d.ordered, priority, budget, chaptersAsked)
		recordCuts(account, fitted, d.ordered, "gave way to the agenda and dividers the brief asks for")
	}

	d.slots = make([]DeckSpecSlot, len(d.ordered))
	for i, s := range d.ordered {
		d.slots[i] = DeckSpecSlot{SlideIndex: i, Slot: s.slot, Kind: s.kind, Guidance: s.guidance}
	}
	d.unplaced = routeDeckSpecFacts(d.ordered, d.slots, facts)
	for i := range d.slots {
		switch d.slots[i].Slot {
		case "regions":
			d.slots[i].Facts = req.facts()
		case "cover":
			d.sources = d.slots[i].Facts
		}
	}
	return d
}

// accountDeckSpec splits the draft's slides into content and structural:
// every slide is content unless it only frames the deck (the cover, an
// agenda, section dividers, the appendix divider).
func accountDeckSpec(account *Budget, ordered []deckSpecSlotDef, draft DeckSpecDraft, appendix bool) {
	for _, d := range ordered {
		if d.slot == "cover" || d.kind == "agenda" {
			account.structural(d.slot)
		}
	}
	if st := draft.Structure; st != nil {
		account.structural("agenda")
		for _, sec := range st.Sections {
			if !sec.Appendix {
				account.structural("divider: " + sec.Title)
			}
		}
	}
	if appendix {
		account.structural("appendix divider")
	}
	account.Content = account.Planned - account.Structural
}

// deckSpecBudgetNotes explains a draft shorter or longer than its budget, why
// a short deck has no chapters, and how many facts found no slot.
func deckSpecBudgetNotes(account Budget, outline *briefOutline, chapters bool, unplaced int) []string {
	var notes []string
	budget, rendered := account.Requested, account.Planned
	switch {
	case outline != nil && rendered > budget:
		notes = append(notes, fmt.Sprintf("The brief's outline lists %d items and is kept whole, so the draft is %d over the budget: merge two items or drop the cover to land on %d", len(outline.items), rendered-budget, budget))
	case outline != nil && rendered < budget:
		notes = append(notes, fmt.Sprintf("The brief's outline lists %d items: the draft follows it and is not padded to %d slides", len(outline.items), budget))
	case outline != nil:
		notes = append(notes, fmt.Sprintf("One slot per item of the brief's outline, in its order (%d items)", len(outline.items)))
	case rendered < budget:
		notes = append(notes, "The storyline is not padded with repeated evidence slides to reach the budget: add a slide only for a claim the brief can prove")
	}
	if !chapters && outline == nil && budget < chapterBudget {
		notes = append(notes, fmt.Sprintf("No agenda or section dividers under %d slides unless the brief asks for them", chapterBudget))
	}
	if unplaced > 0 {
		notes = append(notes, fmt.Sprintf("%d brief fact(s) found no slot and are in unplaced_facts: add a slide for them or fold them into one", unplaced))
	}
	return notes
}

// routeAppendixFacts appends the back-matter slots after the storyline's and
// gives them the facts the body had no room for. It returns what is still
// unplaced.
func routeAppendixFacts(defs []deckSpecSlotDef, slots *[]DeckSpecSlot, unplaced []string) []string {
	rest := unplaced
	for _, d := range defs {
		slot := DeckSpecSlot{SlideIndex: len(*slots), Slot: d.slot, Kind: d.kind, Guidance: d.guidance, Section: appendixTitle, Appendix: true}
		for len(rest) > 0 && len(slot.Facts) < d.capacity {
			slot.Facts = append(slot.Facts, rest[0])
			rest = rest[1:]
		}
		*slots = append(*slots, slot)
	}
	return append([]string{}, rest...)
}

// appendixDividerGuidance explains the authored appendix divider of a flat
// draft.
const appendixDividerGuidance = "Opens the back matter after the close: appendix: true leaves the divider unnumbered and the slides after it out of the deck-rhythm checks. Keep the title Appendix."

// draftFlat lays a flat draft out as slides[]: the storyline, then — when
// there is back matter — an appendix divider (section kind, appendix: true)
// and the backup slides. The divider gets its own slot, inserted before the
// backup slots.
func draftFlat(ordered, backMatter []deckSpecSlotDef, slots *[]DeckSpecSlot, slideFor func(deckSpecSlotDef) map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(ordered)+1+len(backMatter))
	for i, s := range ordered {
		out = append(out, slideFor(s))
		(*slots)[i].Path = fmt.Sprintf("slides[%d]", i)
	}
	if len(backMatter) == 0 {
		return out
	}
	divider := DeckSpecSlot{Slot: "appendix", Kind: "section", Guidance: appendixDividerGuidance, Section: appendixTitle, Appendix: true}
	all := append(append(append([]DeckSpecSlot{}, (*slots)[:len(ordered)]...), divider), (*slots)[len(ordered):]...)
	out = append(out, map[string]any{"kind": "section", "title": appendixTitle, "appendix": true})
	for _, s := range backMatter {
		out = append(out, slideFor(s))
	}
	for i := range all {
		all[i].SlideIndex = i
		all[i].Path = fmt.Sprintf("slides[%d]", i)
	}
	*slots = all
	return out
}

// draftStructure lays a chaptered draft out as a structure block. With back
// matter, the next-steps close becomes the last slide of the last chapter and
// an appendix section (appendix: true) follows it: structure.closing renders
// after every section, and a deck ends its argument before the backup pages.
func draftStructure(ordered []deckSpecSlotDef, secs []draftSection, backMatter []deckSpecSlotDef, slots []DeckSpecSlot, slideFor func(deckSpecSlotDef) map[string]any) *DeckSpecStructureDraft {
	st := &DeckSpecStructureDraft{AutoAgenda: true}
	closing := -1
	for i, s := range ordered {
		switch s.slot {
		case "cover":
			st.Cover = slideFor(s)
			slots[i].Path = "structure.cover"
		case "closing":
			closing = i
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
	if len(backMatter) == 0 {
		return st
	}
	if closing >= 0 {
		last := len(st.Sections) - 1
		st.Sections[last].Slides = append(st.Sections[last].Slides, st.Closing)
		st.Closing = nil
		slots[closing].Path = fmt.Sprintf("structure.sections[%d].slides[%d]", last, len(st.Sections[last].Slides)-1)
		slots[closing].Section = st.Sections[last].Title
	}
	appendix := DeckSpecSectionDraft{Title: appendixTitle, Appendix: true}
	si := len(st.Sections)
	for j, s := range backMatter {
		appendix.Slides = append(appendix.Slides, slideFor(s))
		slots[len(ordered)+j].Path = fmt.Sprintf("structure.sections[%d].slides[%d]", si, j)
	}
	st.Sections = append(st.Sections, appendix)
	return st
}

// routeDeckSpecFacts assigns brief facts to slots by what each fact is
// (go-slide-creator-gvbw8): change-over-time metrics to chart evidence,
// option metrics to the option matrix, other metrics to the KPI / stat slide,
// the recommendation and asks to the decision, dated milestones to the
// roadmap, to-dos to the plan and next steps, and everything else in brief order to the answer, cause,
// highlights, options and plan. A to-do never lands on a KPI card. It returns
// what no slot had room for.
func routeDeckSpecFacts(defs []deckSpecSlotDef, slots []DeckSpecSlot, facts []briefFact) []string {
	placed := make([]bool, len(facts))
	// fill gives the named slots the facts that match, in brief order; whole
	// ignores a slot's capacity, for a list that must not be split.
	fill := func(whole bool, want func(briefFact) bool, slotNames ...string) {
		for _, name := range slotNames {
			for i := range slots {
				if defs[i].slot != name {
					continue
				}
				for fi, f := range facts {
					if !whole && len(slots[i].Facts) >= defs[i].capacity {
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
	place := func(want func(briefFact) bool, slotNames ...string) { fill(false, want, slotNames...) }

	// A series written out as a list is the chart; a single change figure
	// ("+14% YoY") takes a chart only when no list wants it.
	place(func(f briefFact) bool { return f.list }, "evidence")
	place(func(f briefFact) bool { return f.series }, "evidence")
	// The recommendation — the alternative backed and why — belongs with the
	// ask (go-slide-creator-fu6uy).
	place(func(f briefFact) bool { return f.recommend && !f.numeric }, "ask", "options", "answer")
	// The options the brief lists stay together on the option slide, however
	// many there are: two of three used to land in unplaced_facts
	// (go-slide-creator-hf8tf).
	fill(true, func(f briefFact) bool { return f.group == groupOption }, "options")
	fill(true, func(f briefFact) bool { return f.source }, "cover")
	// A list the brief itemises under a header stays on one slide: the phases
	// on the roadmap, the risks on the risk slide, the actions on the plan.
	fill(true, func(f briefFact) bool { return f.group == groupMilestone }, "roadmap")
	fill(true, func(f briefFact) bool { return f.group == groupRisk }, "risks")
	fill(true, func(f briefFact) bool { return f.group == groupAction && !cueNextSteps.MatchString(f.text) }, "plan")
	place(func(f briefFact) bool { return f.numeric && f.option }, "options")
	// An ask that names an amount ("approve a €5m buffer by 15 December") is
	// the ask, not a KPI.
	place(func(f briefFact) bool { return f.numeric && !f.ask }, "problem", "context", "evidence", "answer")
	place(func(f briefFact) bool { return f.risk }, "risks")
	// "Next steps with owners …" is the close, not a plan slide.
	place(func(f briefFact) bool { return cueNextSteps.MatchString(f.text) }, "closing")
	place(func(f briefFact) bool { return f.ask }, "ask", "closing")
	place(func(f briefFact) bool { return f.numeric }, "problem", "context", "evidence", "answer")
	place(func(f briefFact) bool { return f.dated && !f.action }, "roadmap")
	place(func(f briefFact) bool { return f.option }, "options")
	place(func(f briefFact) bool { return f.action }, "plan", "closing", "ask")
	place(func(f briefFact) bool { return f.dated }, "roadmap", "plan", "closing")
	place(func(f briefFact) bool { return f.risk }, "cause", "plan")
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
	first = strings.TrimRight(first, " ,;:")
	return TruncateBrief(first, 80)
}
