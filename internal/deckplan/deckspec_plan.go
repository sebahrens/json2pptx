package deckplan

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// FormatDeckSpec is the plan_deck format that drafts a semantic DeckSpec
// instead of raw pattern skeletons.
const FormatDeckSpec = "deckspec"

// DeckSpecPlan is plan_deck's format:"deckspec" response: a DeckSpec draft
// whose slides follow a consulting narrative (answer first, then problem,
// cause, evidence, plan, roadmap, ask), plus per-slide slot guidance and the
// brief facts routed to each slot.
//
// The raw plan chooses a *pattern* per slide and splits the brief into seeds;
// an agent on the default DeckSpec path then had to translate patterns back
// into kinds and reassemble the story from fragments (go-slide-creator-waft9).
// This plan speaks the DeckSpec's own vocabulary: a kind per narrative slot,
// so the draft is edited in place and sent to validate_deck_spec.
type DeckSpecPlan struct {
	Format      string `json:"format"`
	Brief       string `json:"brief"`
	SlideBudget int    `json:"slide_budget"`
	// DeckSpec is the draft: meta plus slides[] with kind and a __FILL__
	// title. Every other field of each kind comes from list_slide_kinds.
	DeckSpec DeckSpecDraft `json:"deck_spec"`
	// Slots explains each draft slide: its narrative slot, what the slide must
	// argue, and the brief facts routed to it.
	Slots []DeckSpecSlot `json:"slots"`
	// UnplacedFacts lists brief facts no slot had room for. Always present.
	UnplacedFacts []string `json:"unplaced_facts"`
	// BudgetNote explains a draft shorter than slide_budget: the narrative
	// is not padded with repeated evidence slides.
	BudgetNote          string `json:"budget_note,omitempty"`
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`
}

// DeckSpecDraft is the DeckSpec skeleton: meta and slides in DeckSpec shape.
type DeckSpecDraft struct {
	Meta   map[string]any   `json:"meta"`
	Slides []map[string]any `json:"slides"`
}

// DeckSpecSlot annotates one draft slide.
type DeckSpecSlot struct {
	SlideIndex int      `json:"slide_index"`
	Slot       string   `json:"slot"`
	Kind       string   `json:"kind"`
	Guidance   string   `json:"guidance"`
	Facts      []string `json:"facts,omitempty"`
}

// deckSpecSlotDef is one narrative slot of the storyline.
type deckSpecSlotDef struct {
	slot     string
	kind     string
	guidance string
	capacity int  // brief facts the slot can carry
	numeric  bool // prefers quantity facts
}

// Narrative slots in deck order. The problem slot's kind depends on how many
// numbers the brief carries (see problemSlot).
var (
	slotCover   = deckSpecSlotDef{"cover", "title", "Deck title and a subtitle naming audience and date.", 0, false}
	slotAnswer  = deckSpecSlotDef{"answer", "executive_summary", "The whole answer up front: 3-5 points, each a conclusion a later slide proves; the title is the single-sentence answer.", 3, false}
	slotCause   = deckSpecSlotDef{"cause", "pillars", "Why it is happening: 3-5 drivers, each with its evidence.", 2, false}
	slotEvid    = deckSpecSlotDef{"evidence", "chart_insight", "One chart that proves one claim: the title states the claim with its number; set takeaway and source; never invent data.", 2, true}
	slotPlan    = deckSpecSlotDef{"plan", "process", "What we will do: 3-6 steps, each an action with its outcome.", 2, false}
	slotRoadmap = deckSpecSlotDef{"roadmap", "roadmap", "When it happens: 3-6 phases with dates and the milestone that ends each.", 2, false}
	slotAsk     = deckSpecSlotDef{"ask", "decision", "The decision needed now: the options, the recommended one, and the ask with owner and date.", 1, false}
	slotClosing = deckSpecSlotDef{"closing", "closing", "Restate the decision and the next step, with owner and date.", 0, false}
)

// maxEvidenceSlots caps repeated evidence slides: past this the draft stops
// short of the budget rather than padding the story.
const maxEvidenceSlots = 4

// problemSlot picks the problem slide's kind from the brief's numbers: KPI
// cards for several, one hero statistic for one, a today-vs-needed
// comparison when the brief has none (a chart would need invented data).
func problemSlot(numericFacts int) deckSpecSlotDef {
	g := "What is wrong or at stake, shown with the brief's own numbers; the title states the problem as a sentence carrying its number."
	switch {
	case numericFacts >= 2:
		return deckSpecSlotDef{"problem", "kpi_snapshot", g, 6, true}
	case numericFacts == 1:
		return deckSpecSlotDef{"problem", "stat", g, 1, true}
	default:
		return deckSpecSlotDef{"problem", "comparison", "What is wrong or at stake: where we are against where we need to be; the title states the gap as a sentence.", 2, false}
	}
}

// BuildDeckSpecPlan drafts a DeckSpec storyline for the brief within the
// slide budget. Slots are admitted in priority order (cover, ask, closing,
// answer, problem, plan, cause, roadmap, then up to maxEvidenceSlots
// evidence slides) and laid out in narrative order.
func BuildDeckSpecPlan(p Params) *DeckSpecPlan {
	budget := p.SlideBudget
	facts := extractBriefFacts(p.Brief)
	numeric := 0
	for _, f := range facts {
		if f.numeric {
			numeric++
		}
	}

	priority := []deckSpecSlotDef{slotCover, slotAsk, slotClosing, slotAnswer, problemSlot(numeric), slotPlan, slotCause, slotRoadmap}
	if budget < len(priority) {
		priority = priority[:budget]
	}
	evidence := budget - len(priority)
	if evidence > maxEvidenceSlots {
		evidence = maxEvidenceSlots
	}
	admitted := map[string]deckSpecSlotDef{}
	for _, s := range priority {
		admitted[s.slot] = s
	}

	var ordered []deckSpecSlotDef
	for _, name := range []string{"cover", "answer", "problem", "cause"} {
		if s, ok := admitted[name]; ok {
			ordered = append(ordered, s)
		}
	}
	for i := 0; i < evidence; i++ {
		ordered = append(ordered, slotEvid)
	}
	for _, name := range []string{"plan", "roadmap", "ask", "closing"} {
		if s, ok := admitted[name]; ok {
			ordered = append(ordered, s)
		}
	}

	slots := make([]DeckSpecSlot, len(ordered))
	for i, s := range ordered {
		slots[i] = DeckSpecSlot{SlideIndex: i, Slot: s.slot, Kind: s.kind, Guidance: s.guidance}
	}
	unplaced := routeDeckSpecFacts(ordered, slots, facts)

	topic := deckTopic(p.Brief)
	meta := map[string]any{"title": topic}
	if p.TemplateName != "" {
		meta["template"] = p.TemplateName
	}
	if p.Audience != "" {
		meta["audience"] = p.Audience
	}
	slides := make([]map[string]any, len(ordered))
	for i, s := range ordered {
		slide := map[string]any{"kind": s.kind, "title": patterns.FillPlaceholder}
		if s.slot == "cover" {
			slide["title"] = topic
			slide["subtitle"] = patterns.FillPlaceholder
		}
		slides[i] = slide
	}

	plan := &DeckSpecPlan{
		Format:        FormatDeckSpec,
		Brief:         p.Brief,
		SlideBudget:   budget,
		DeckSpec:      DeckSpecDraft{Meta: meta, Slides: slides},
		Slots:         slots,
		UnplacedFacts: unplaced,
	}
	if len(ordered) < budget {
		plan.BudgetNote = "The storyline needs fewer slides than slide_budget; it is not padded with repeated evidence slides. Add a slide only for a claim the brief can prove."
	}
	return plan
}

// routeDeckSpecFacts assigns brief facts to slots: quantities to numeric
// slots (problem, evidence) first, then every remaining fact in brief order to
// the answer, cause, plan, roadmap and ask slots. It returns what no slot had
// room for.
func routeDeckSpecFacts(defs []deckSpecSlotDef, slots []DeckSpecSlot, facts []briefFact) []string {
	placed := make([]bool, len(facts))
	fill := func(wantNumeric bool) {
		for i := range slots {
			if defs[i].numeric != wantNumeric || defs[i].capacity == 0 {
				continue
			}
			for fi, f := range facts {
				if len(slots[i].Facts) >= defs[i].capacity {
					break
				}
				if placed[fi] || (wantNumeric && !f.numeric) {
					continue
				}
				slots[i].Facts = append(slots[i].Facts, f.text)
				placed[fi] = true
			}
		}
	}
	fill(true)
	fill(false)
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
	if parts := factClauseSplit.Split(first, 2); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		first = strings.TrimSpace(parts[0])
	}
	return TruncateBrief(first, 80)
}
