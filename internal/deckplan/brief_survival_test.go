package deckplan

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// The three briefs of the 2026-10-03 agent-journey review, verbatim from
// tests/quality/results/agent-journey-20261003/{a-coldstart,e-revise}/log-calls.jsonl
// and d-cli/findings.json (go-slide-creator-hf8tf, go-slide-creator-58qda).
const (
	journeyBriefBoard    = "Board deck on our Q3 FY26 results, 9-10 slides. Facts: revenue €48.2m (+14% YoY, plan €46.0m); gross margin 61.5% (Q2: 63.0%) because cloud costs rose 22%; net revenue retention 112%; churn 2.1% monthly in SMB vs 0.6% enterprise; quarterly revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2; three options to fix margin: renegotiate cloud contract (saves €1.2m/yr, 2 months), re-platform storage tier (saves €2.0m/yr, 6 months, €0.8m one-off), raise SMB prices 5% (adds €0.9m/yr, churn risk); recommendation: renegotiate now and start re-platforming; next steps with owners CFO / CTO / CRO and dates in October–December 2026; source: management accounts Q3 FY26."
	journeyBriefInvestor = "Investor update for a fictional climate-tech company, 8 slides: headline results, ARR grew from $6.1m to $9.4m in 12 months, burn fell from $1.1m to $0.7m a month, 18 months runway, three product milestones (pilot plant Q1 2027, first commercial unit Q3 2027, series B Q4 2027), main risk is permitting delay, ask is introductions to two strategic partners."
	journeyBriefPitch    = "7-slide investor pitch deck for a fictional climate-tech startup: problem, solution, market size chart, traction KPIs, business model, team of 4, the ask"
)

// survivalBrief is one brief of the survival corpus.
type survivalBrief struct {
	name     string
	brief    string
	budget   int // 0: the caller passed no slide_budget
	audience string
	// must lists the brief's named items — things with no digit or capital to
	// find them by — that have to reach a slot or unplaced_facts.
	must []string
	// constraints is the kind=value of every deck instruction in the brief.
	constraints []string
	// wantBudget is the budget the plan must be drafted to.
	wantBudget int
	// outline is the kind of each listed item, in order, when the brief
	// enumerates its slides.
	outline []string
	// chaptered says the brief asks for an agenda or dividers.
	chaptered bool
}

var survivalCorpus = []survivalBrief{
	{
		name: "journey A5: board results", brief: journeyBriefBoard, budget: 10, audience: "board of directors",
		must:        []string{"re-platform storage tier", "raise SMB prices", "renegotiate cloud contract", "renegotiate now and start re-platforming", "management accounts", "cloud costs rose", "net revenue retention"},
		constraints: []string{"slide_count=9-10"}, wantBudget: 10,
	},
	{
		name: "journey E1/E2: investor update", brief: journeyBriefInvestor, budget: 8, audience: "investors",
		must:        []string{"permitting delay", "introductions to two strategic partners", "headline results", "pilot plant", "first commercial unit", "runway"},
		constraints: []string{"slide_count=8"}, wantBudget: 8,
		outline: []string{"executive_summary", "chart_insight", "chart_insight", "stat", "timeline", "table", "next_steps"},
	},
	{
		name: "journey A14: investor pitch", brief: journeyBriefPitch, budget: 7,
		must:        []string{"problem", "solution", "market size chart", "traction KPIs", "business model", "team of 4", "the ask"},
		constraints: []string{"slide_count=7"}, wantBudget: 7,
		outline: []string{"pillars", "pillars", "chart_insight", "kpi_snapshot", "pillars", "team", "next_steps"},
	},
	{
		name: "QBR", brief: reviewBriefQBR, budget: 10,
		must:       []string{"launching the enterprise tier", "fixing SMB onboarding", "hiring 12 AEs", "EMEA pipeline"},
		wantBudget: 10,
	},
	{
		name: "strategy recommendation", brief: reviewBriefStrategy, budget: 10,
		must:       []string{"partner with Globex", "acquire Initech", "time-to-market", "term sheet", "should Acme enter the mid-market segment"},
		wantBudget: 10,
	},
	{
		name: "product update", brief: reviewBriefProduct, budget: 8,
		must:       []string{"audit logs", "analytics dashboard", "mobile app beta", "API v2", "SOC 2 Type II"},
		wantBudget: 8,
	},
	{
		name: "service pilot", brief: reviewBriefPilot, budget: 9,
		must:       []string{"internal support team", "outsourcing", "retain customer relationships", "funding approval", "backup assumptions"},
		wantBudget: 9,
	},
	{
		name:        "labelled outline with a template",
		brief:       "Sales kickoff deck for the EMEA team. Slides: agenda, FY26 results (bookings €31m, +22%), win stories, 2027 territories, comp plan changes, next steps. Use the forest-green template.",
		must:        []string{"win stories", "comp plan changes", "territories", "bookings"},
		constraints: []string{"template=forest-green"}, wantBudget: 10,
		outline: []string{"agenda", "kpi_snapshot", "pillars", "pillars", "pillars", "next_steps"},
	},
	{
		name:        "numbered outline",
		brief:       "Project Atlas steering committee update, 6 slides\n1. Status summary\n2. Budget: spent $2.4m of $3.1m\n3. Milestones: design freeze 15 March 2027, pilot in June 2027\n4. Top risks\n5. Decision on vendor Initech\n6. Next steps",
		must:        []string{"Status summary", "design freeze", "Top risks", "Decision on vendor Initech"},
		constraints: []string{"slide_count=6"}, wantBudget: 6,
		outline: []string{"executive_summary", "table", "timeline", "table", "decision", "next_steps"},
	},
	{
		name:        "sentences with audience, duration, count and template",
		brief:       "Audience: the supervisory board. A 20-minute presentation, max 12 slides, using the midnight-blue template. Operating profit rose from €14.2m to €17.9m in FY25. Headcount reached 1,240. The main risk is the Basel IV capital rule taking effect in January 2027. We ask the board to approve a €5m buffer by 15 December 2026.",
		must:        []string{"Basel IV capital rule", "Operating profit", "Headcount", "buffer"},
		constraints: []string{"audience=the supervisory board", "duration=20", "slide_count=12", "template=midnight-blue"}, wantBudget: 12,
	},
	{
		name:        "agenda and dividers asked for in a short deck",
		brief:       "Kickoff deck for Project Orion with an agenda and section dividers, 9 slides. Scope covers 14 depots across DACH. Go-live is planned for September 2027. Budget is €3.2m. Three phases: discovery in Q1 2027, build in Q2 2027, rollout in Q3 2027. Steering committee meets monthly.",
		must:        []string{"Steering committee meets monthly", "discovery", "rollout", "depots"},
		constraints: []string{"structure=agenda, dividers", "slide_count=9"}, wantBudget: 9, chaptered: true,
	},
	{
		name:        "bulleted facts are not an outline",
		brief:       "Q3 review for the leadership team, 5 slides.\n- ARR reached €12.5m\n- churn fell to 2.1%\n- NPS 54\n- hiring 12 AEs by December",
		must:        []string{"hiring 12 AEs"},
		constraints: []string{"slide_count=5"}, wantBudget: 5,
	},
	{
		name:       "prose with no numbers",
		brief:      "Town hall on the new hybrid work policy. Teams choose two anchor days a week. Managers approve exceptions. The policy starts in January. Feedback goes to the people team.",
		must:       []string{"anchor days", "Managers approve exceptions", "Feedback goes to the people team"},
		wantBudget: 10,
	},
}

var (
	// survivalAmount is every number in a brief, with its decimals: an amount,
	// a percentage, a count, a day or a year.
	survivalAmount = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
	// survivalDate is every month, quarter, half and fiscal year in a brief.
	survivalDate = regexp.MustCompile(`\b(?:January|February|March|April|May|June|July|August|September|October|November|December|[QH][1-4]|FY\d{2,4})\b`)
	// survivalName is every acronym and every capitalised word that does not
	// open a sentence or a line: the brief's names.
	survivalName = regexp.MustCompile(`\b[A-Z]{2,}[a-z]?\b|\b[A-Z][a-z]+\b`)
	// survivalListMarker is the numbering of a list line, which is formatting.
	survivalListMarker = regexp.MustCompile(`(?m)^\s*\(?\d{1,2}[.)]\s+`)
	survivalSentence   = regexp.MustCompile(`(?:^|[.!?:]\s+|\n\s*(?:[-*•]|\d{1,2}[.)])?\s*)([A-Z][a-z]+)\b`)
)

// briefTokens returns every amount, date and name of the brief once its deck
// instructions are taken out.
func briefTokens(brief string, constraints []Constraint) []string {
	for _, c := range constraints {
		brief = strings.Replace(brief, c.Text, " ", 1)
	}
	brief = survivalListMarker.ReplaceAllString(brief, "")
	seen := map[string]bool{}
	var out []string
	add := func(tokens []string) {
		for _, tok := range tokens {
			if !seen[tok] {
				seen[tok] = true
				out = append(out, tok)
			}
		}
	}
	add(survivalAmount.FindAllString(brief, -1))
	add(survivalDate.FindAllString(brief, -1))
	opener := map[string]bool{}
	for _, m := range survivalSentence.FindAllStringSubmatch(brief, -1) {
		opener[m[1]] = true
	}
	for _, name := range survivalName.FindAllString(brief, -1) {
		// A word that only ever opens a sentence is capitalised by
		// convention; the explicit must list covers those that are names.
		if !opener[name] {
			add([]string{name})
		}
	}
	return out
}

// assertSurvives checks that every token and named item is in the haystack and
// that no deck instruction is.
func assertSurvives(t *testing.T, tc survivalBrief, constraints []Constraint, facts []string) {
	t.Helper()
	hay := strings.Join(facts, "\n")
	for _, tok := range append(briefTokens(tc.brief, constraints), tc.must...) {
		if !strings.Contains(hay, tok) {
			t.Errorf("%q is in neither a slot's facts nor unplaced_facts:\n%s", tok, hay)
		}
	}
	for _, c := range constraints {
		for _, f := range facts {
			if strings.Contains(f, c.Text) {
				t.Errorf("deck instruction %q was routed as the fact %q", c.Text, f)
			}
		}
	}
	var got []string
	for _, c := range constraints {
		got = append(got, c.Kind+"="+c.Value)
	}
	if !reflect.DeepEqual(got, tc.constraints) {
		t.Errorf("constraints = %q, want %q", got, tc.constraints)
	}
}

func survivalParams(tc survivalBrief) Params {
	p := Params{Brief: tc.brief, SlideBudget: 10, Audience: tc.audience}
	if tc.budget > 0 {
		p.SlideBudget, p.BudgetExplicit = tc.budget, true
	}
	return p
}

// TestDeckSpecPlanBriefSurvival is the go-slide-creator-hf8tf and
// go-slide-creator-58qda acceptance test over the survival corpus: every
// amount, date and named item of the brief is in slots[].facts or
// unplaced_facts; deck instructions are constraints, not facts; an enumerated
// outline gets one slot per item in order; a budget under 12 has no agenda or
// dividers unless the brief asks; and the plan always says how the budget was
// spent.
func TestDeckSpecPlanBriefSurvival(t *testing.T) {
	if len(survivalCorpus) < 10 {
		t.Fatalf("the survival corpus needs at least ten briefs, has %d", len(survivalCorpus))
	}
	for _, tc := range survivalCorpus {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildDeckSpecPlan(survivalParams(tc))

			var facts []string
			for _, s := range p.Slots {
				facts = append(facts, s.Facts...)
			}
			if p.UnplacedFacts == nil {
				t.Fatal("unplaced_facts must always be an array")
			}
			assertSurvives(t, tc, p.Constraints, append(facts, p.UnplacedFacts...))
			if p.SlideBudget != tc.wantBudget || p.Budget.Requested != tc.wantBudget {
				t.Errorf("slide_budget = %d, budget.requested = %d, want %d", p.SlideBudget, p.Budget.Requested, tc.wantBudget)
			}

			// An enumerated outline: one slot per item, in order, after the cover.
			kinds := storylineKinds(p)
			if tc.outline != nil {
				want := append([]string{"title"}, tc.outline...)
				if !reflect.DeepEqual(kinds, want) {
					t.Errorf("outline kinds = %v, want %v", kinds, want)
				}
			}

			// The budget: content first, and always accounted for.
			structural := 0
			for _, s := range p.Slots {
				if s.Kind == "agenda" || s.Kind == "section" || s.Slot == "cover" {
					structural++
				}
			}
			rendered := len(p.Slots)
			if st := p.DeckSpec.Structure; st != nil {
				if !tc.chaptered && p.SlideBudget < chapterBudget {
					t.Errorf("a %d-slide draft carries an agenda and dividers the brief did not ask for: %+v", p.SlideBudget, st)
				}
				rendered += 1 + len(st.Sections)
				structural += 1 + len(st.Sections)
			} else if tc.chaptered {
				t.Errorf("the brief asks for an agenda and dividers and got a flat draft: %v", kinds)
			}
			if p.SlideBudget < chapterBudget && !tc.chaptered && tc.outline == nil {
				for _, k := range kinds {
					if k == "agenda" || k == "section" {
						t.Errorf("a %d-slide draft carries a %s slide the brief did not ask for: %v", p.SlideBudget, k, kinds)
					}
				}
			}
			b := p.Budget
			if b.Planned != rendered || b.Structural != structural || b.Content != rendered-structural || len(b.StructuralSlides) != structural {
				t.Errorf("budget = %+v, want planned %d = %d content + %d structural", b, rendered, rendered-structural, structural)
			}
			if b.Cut == nil || b.StructuralSlides == nil {
				t.Errorf("budget.cut and budget.structural_slides must always be arrays: %+v", b)
			}
			if tc.outline == nil && b.Planned > b.Requested {
				t.Errorf("the draft renders %d slides, over the budget of %d", b.Planned, b.Requested)
			}
			wantNote := fmt.Sprintf("Planned %d of %d slides: %d content", b.Planned, b.Requested, b.Content)
			if !strings.HasPrefix(p.BudgetNote, wantNote) {
				t.Errorf("budget_note = %q, want it to open %q", p.BudgetNote, wantNote)
			}
			for _, s := range p.Slots {
				if s.Path == "" || s.Guidance == "" {
					t.Errorf("slot %s has no path or guidance", s.Slot)
				}
			}
		})
	}
}

// TestRawPlanBriefSurvival holds the raw plan to the same contract: every
// amount, date and named item of the brief is on a slide's facts, in the
// opening slide's seed (the topic and the source) or in unplaced_facts.
func TestRawPlanBriefSurvival(t *testing.T) {
	reg := patterns.Default()
	outlinePatterns := map[string]string{
		"executive_summary": "exec-summary", "chart_insight": "chart-insights-split", "stat": "stat-hero",
		"timeline": "timeline-horizontal", "table": "labeled-rows", "next_steps": "next-steps", "pillars": "card-grid",
		"kpi_snapshot": "kpi-3up", "team": "team-bios", "agenda": "agenda", "decision": "next-steps",
	}
	for _, tc := range survivalCorpus {
		t.Run(tc.name, func(t *testing.T) {
			res := BuildDeckPlan(reg, survivalParams(tc), nil)
			if len(res.Slides) == 0 || res.Slides[0].NarrativeRole != "opening" {
				t.Fatalf("plan must open on the title slide: %v", planPatterns(res))
			}
			// Title and closing slides take no facts in the raw plan: the
			// opening's seed carries the topic and the source instead.
			facts := []string{res.Slides[0].ContentSeed}
			for _, s := range res.Slides {
				facts = append(facts, s.Facts...)
			}
			assertSurvives(t, tc, res.Constraints, append(facts, res.UnplacedFacts...))

			if tc.outline != nil {
				var got, want []string
				for _, s := range res.Slides[1:] {
					got = append(got, s.RecommendedPattern)
				}
				for _, k := range tc.outline {
					want = append(want, outlinePatterns[k])
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("outline patterns = %v, want %v", got, want)
				}
			}
			structural := 0
			for _, s := range res.Slides {
				if isStructuralRole(s.NarrativeRole) || strings.HasPrefix(s.RecommendedPattern, "agenda") {
					structural++
				}
				if strings.HasPrefix(s.RecommendedPattern, "agenda") && tc.outline == nil && res.SlideBudget < chapterBudget {
					t.Errorf("a %d-slide plan carries an agenda the brief did not ask for", res.SlideBudget)
				}
			}
			b := res.Budget
			if b.Requested != tc.wantBudget || b.Planned != len(res.Slides) || b.Structural != structural || b.Content != len(res.Slides)-structural {
				t.Errorf("budget = %+v, want requested %d, planned %d, structural %d", b, tc.wantBudget, len(res.Slides), structural)
			}
			if !strings.HasPrefix(res.BudgetNote, fmt.Sprintf("Planned %d of %d slides:", b.Planned, b.Requested)) {
				t.Errorf("budget_note = %q", res.BudgetNote)
			}
		})
	}
}

// The journey findings, one assertion each.
func TestDeckSpecPlanJourneyFindings(t *testing.T) {
	// A5: the five-quarter series whole on the chart slide, all three options
	// on the option slide, the recommendation with the ask.
	board := BuildDeckSpecPlan(Params{Brief: journeyBriefBoard, SlideBudget: 10, BudgetExplicit: true, Audience: "board of directors"})
	if got := factsOf(board, "evidence"); !strings.Contains(got, "quarterly revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2") {
		t.Errorf("A5: the revenue series was split; evidence facts = %q", got)
	}
	options := slotByName(board, "options")
	if options == nil {
		t.Fatalf("A5: no option slot: %v", storylineKinds(board))
	}
	for _, opt := range []string{"renegotiate cloud contract (saves €1.2m/yr, 2 months)", "re-platform storage tier (saves €2.0m/yr, 6 months, €0.8m one-off)", "raise SMB prices 5% (adds €0.9m/yr, churn risk)"} {
		if !containsStr(options.Facts, opt) {
			t.Errorf("A5: option %q is not on the option slide: %q", opt, options.Facts)
		}
	}
	if len(board.UnplacedFacts) != 0 {
		t.Errorf("A5: unplaced_facts = %q, want none", board.UnplacedFacts)
	}
	if got := factsOf(board, "ask"); !strings.Contains(got, "recommendation: renegotiate now and start re-platforming") {
		t.Errorf("A5: the recommendation lost its content: ask facts = %q", got)
	}
	if got := factsOf(board, "problem") + factsOf(board, "context"); !strings.Contains(got, "gross margin 61.5% (Q2: 63.0%) because cloud costs rose 22%") {
		t.Errorf("A5: the margin fact was cut at its bracket: %q", got)
	}
	if got := factsOf(board, "closing"); !strings.Contains(got, "next steps with owners CFO / CTO / CRO") {
		t.Errorf("A5: the next steps are not on the close: %q", got)
	}
	if board.DeckSpec.Meta["source"] != "management accounts Q3 FY26" {
		t.Errorf("A5: meta.source = %v, want the source the brief names", board.DeckSpec.Meta["source"])
	}
	if board.DeckSpec.Structure != nil {
		t.Errorf("58qda: a 10-slide draft must not spend slides on an agenda and dividers: %+v", board.DeckSpec.Structure)
	}

	// E1 / E2: seven content slots after the cover, the risk and the ask
	// among them, and no agenda or dividers.
	inv := BuildDeckSpecPlan(Params{Brief: journeyBriefInvestor, SlideBudget: 8, BudgetExplicit: true, Audience: "investors"})
	if inv.DeckSpec.Structure != nil || len(inv.DeckSpec.Slides) != 8 {
		t.Fatalf("E2: want a flat 8-slide draft, got %+v", inv.DeckSpec)
	}
	if inv.Budget.Content != 7 || inv.Budget.Structural != 1 {
		t.Errorf("E2: budget = %+v, want 7 content slides and the cover", inv.Budget)
	}
	if got := factsOf(inv, "risks"); got != "main risk is permitting delay" {
		t.Errorf("E1: risk slot facts = %q", got)
	}
	if got := factsOf(inv, "closing"); got != "ask is introductions to two strategic partners" {
		t.Errorf("E1: closing slot facts = %q", got)
	}

	// A14: the raw plan follows the outline too, and the deck title is the
	// topic, not the instruction.
	pitch := BuildDeckSpecPlan(Params{Brief: journeyBriefPitch, SlideBudget: 7, BudgetExplicit: true})
	if got := pitch.DeckSpec.Meta["title"]; got != "Investor pitch deck for a fictional climate-tech startup" {
		t.Errorf("A14: meta.title = %q", got)
	}
	if !strings.Contains(pitch.BudgetNote, "1 over the budget") || pitch.Budget.Planned != 8 {
		t.Errorf("A14: an outline longer than the budget must say so: %+v %q", pitch.Budget, pitch.BudgetNote)
	}

	// Without slide_budget the brief's own count is the budget.
	if p := BuildDeckSpecPlan(Params{Brief: journeyBriefInvestor, SlideBudget: 10}); p.SlideBudget != 8 {
		t.Errorf("slide_budget = %d, want the 8 the brief states", p.SlideBudget)
	}
	if p := BuildDeckSpecPlan(Params{Brief: journeyBriefBoard, SlideBudget: 10}); p.SlideBudget != 10 {
		t.Errorf("slide_budget = %d, want the upper end of \"9-10 slides\"", p.SlideBudget)
	}
}

// A cut never names a slot the plan still shows: of two evidence slides the
// one left out is "a further evidence" (the cold-start journey of 2026-10-04
// read "Cut: evidence (chart_insight)" beside a planned chart_insight).
func TestDeckSpecPlanCutNamesTheFurtherSlot(t *testing.T) {
	const brief = "Q3 FY26 board update for Northwind Logistics, 7 slides. Revenue was EUR 48.2M, up 12% year on year. " +
		"EBITDA margin 14.1% against a plan of 13%. Quarterly revenue FY25 Q3 to FY26 Q3: 43.0, 44.1, 45.6, 46.9, 48.2. " +
		"SMB churn rose from 2.2% to 3.1%. Three options to fix SMB churn: do nothing, a dedicated success team for EUR 1.2M, " +
		"a self-serve portal for EUR 2.4M; we recommend the success team. Plan: hire in Q4 FY26, pilot in Q1 FY27, full rollout in Q2 FY27. " +
		"Ask: approve EUR 1.2M and eight hires."
	plan := BuildDeckSpecPlan(Params{Brief: brief, SlideBudget: 10})
	planned := map[string]bool{}
	for _, s := range plan.Slots {
		planned[fmt.Sprintf("%s (%s)", s.Slot, s.Kind)] = true
	}
	further := 0
	for _, c := range plan.Budget.Cut {
		if planned[c.What] {
			t.Errorf("the account cuts %q, which the plan still carries", c.What)
		}
		if rest, ok := strings.CutPrefix(c.What, "a further "); ok {
			further++
			if !planned[rest] {
				t.Errorf("%q is cut as a further slot, and the plan has no %s", c.What, rest)
			}
		}
	}
	if further == 0 {
		t.Errorf("the brief no longer cuts a second evidence slide; cut = %+v", plan.Budget.Cut)
	}
	if !strings.Contains(plan.BudgetNote, "a further evidence (chart_insight)") {
		t.Errorf("budget_note = %q", plan.BudgetNote)
	}

	// go-slide-creator-5iy8n: the same journey put the series' label in the
	// closing slot and its numbers in the evidence slot. A label and the
	// numbers it names are one fact, in one slot.
	const series = "Quarterly revenue FY25 Q3 to FY26 Q3: 43.0, 44.1, 45.6, 46.9, 48.2"
	if got := factsOf(plan, "evidence"); got != series {
		t.Errorf("evidence facts = %q, want the labelled series %q", got, series)
	}
	for _, s := range plan.Slots {
		if s.Slot == "evidence" {
			continue
		}
		for _, f := range s.Facts {
			if strings.Contains(f, "Quarterly revenue") || strings.Contains(f, "43.0") {
				t.Errorf("slot %s carries part of the revenue series: %q", s.Slot, f)
			}
		}
	}
}

// A colon label of any length stays with the numbers after it; a label
// followed by words is split as before (go-slide-creator-5iy8n).
func TestSplitClausesKeepsALabelWithItsNumbers(t *testing.T) {
	for _, tt := range []struct {
		brief string
		want  []string
	}{
		{"Update. Quarterly revenue from Q3 FY25 to Q3 FY26: 43.0, 44.1, 45.6, 46.9, 48.2. Churn rose.",
			[]string{"Quarterly revenue from Q3 FY25 to Q3 FY26: 43.0, 44.1, 45.6, 46.9, 48.2", "Churn rose"}},
		{"Update. Net revenue retention in the enterprise segment: 112%; churn 2%",
			[]string{"Net revenue retention in the enterprise segment: 112%", "churn 2%"}},
		{"Update. Headcount by site at the end of June: 120, 85 and 40",
			[]string{"Headcount by site at the end of June: 120, 85 and 40"}},
		{"Update. Key figures: 41.0, 42.3, 44.1", []string{"41.0, 42.3, 44.1"}},
		{"Update. What the board asked us to look at this quarter: pricing in the SMB segment",
			[]string{"What the board asked us to look at this quarter", "pricing in the SMB segment"}},
	} {
		if got := factTexts(extractBriefFacts(tt.brief)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: facts = %q, want %q", tt.brief, got, tt.want)
		}
	}
	labelled := extractBriefFacts("Update. Quarterly revenue from Q3 FY25 to Q3 FY26: 43.0, 44.1, 45.6, 46.9, 48.2")
	if len(labelled) != 1 || !labelled[0].list || !labelled[0].series {
		t.Errorf("a labelled series must classify as a chartable list: %+v", labelled)
	}

	// go-slide-creator-rep2b: a list whose last value is joined by "and" is
	// the same series, and it reaches the evidence slot; two values are not.
	for text, want := range map[string]bool{
		"Headcount by site at the end of June: 120, 85 and 40": true,
		"Headcount by site: 120, 85, 60 and 40":                true,
		"Margins of 12%, 14% And 17%":                          true,
		"Headcount by site: 85 and 40":                         false,
		"Sites 3 and 4 open in May, 2 and 5 in June":           false,
	} {
		if f := classifyFact(text, true); f.list != want || f.series != want {
			t.Errorf("classifyFact(%q): list=%v series=%v, want %v", text, f.list, f.series, want)
		}
	}
	plan := BuildDeckSpecPlan(Params{Brief: "Site update for the board. Headcount by site at the end of June: 120, 85 and 40. Attrition fell.", SlideBudget: 6})
	if got := factsOf(plan, "evidence"); got != "Headcount by site at the end of June: 120, 85 and 40" {
		t.Errorf("evidence facts = %q, want the three-site series", got)
	}

	// "and" / "or" are never a unit.
	for s, want := range map[string]bool{
		"85 and": false, "85 or": false, "40 And": false, "85 and 40 or": false,
		"85 and 40": true, "and 48.2": true, "€4m": true, "12%": true, "3 bn": true, "85 pts or 40 pts": true,
	} {
		if got := isBareNumber(s); got != want {
			t.Errorf("isBareNumber(%q) = %v, want %v", s, got, want)
		}
	}
}

// 58qda: what the budget leaves out is named, and an agenda the brief asks
// for is drafted even in a short deck — and can be refused in a long one.
func TestDeckSpecPlanBudgetAccount(t *testing.T) {
	tight := BuildDeckSpecPlan(Params{Brief: journeyBriefBoard, SlideBudget: 6, BudgetExplicit: true})
	if len(tight.Budget.Cut) == 0 || !strings.Contains(tight.BudgetNote, "Cut: ") {
		t.Fatalf("a 6-slide draft of a 10-slot storyline must list what was cut: %+v %q", tight.Budget, tight.BudgetNote)
	}
	for _, c := range tight.Budget.Cut {
		if c.What == "" || c.Reason == "" {
			t.Errorf("cut entry without what or reason: %+v", c)
		}
	}
	if tight.Budget.Planned != 6 || tight.DeckSpec.Structure != nil {
		t.Errorf("tight draft = %+v", tight.Budget)
	}
	// Facts the cut slots would have carried are not lost.
	hay := planFactsAndUnplaced(tight)
	for _, want := range []string{"41.0, 42.3, 44.1, 45.9, 48.2", "re-platform storage tier", "€0.9m/yr"} {
		if !strings.Contains(hay, want) {
			t.Errorf("tight draft lost %q", want)
		}
	}

	full := BuildDeckSpecPlan(Params{Brief: journeyBriefBoard, SlideBudget: 10, BudgetExplicit: true})
	if len(full.Budget.Cut) != 0 || !strings.Contains(full.BudgetNote, "Nothing was cut.") {
		t.Errorf("a draft that fits must say nothing was cut: %+v %q", full.Budget, full.BudgetNote)
	}

	asked := BuildDeckSpecPlan(Params{Brief: reviewBriefQBR + ". Include an agenda.", SlideBudget: 9, BudgetExplicit: true})
	if asked.DeckSpec.Structure == nil || !asked.DeckSpec.Structure.AutoAgenda {
		t.Errorf("an agenda the brief asks for must be drafted at 9 slides: %v", storylineKinds(asked))
	}
	if asked.Budget.Planned > 9 {
		t.Errorf("asked-for agenda overran the budget: %+v", asked.Budget)
	}
	refused := BuildDeckSpecPlan(Params{Brief: reviewBriefQBR + ". No agenda.", SlideBudget: 16, BudgetExplicit: true})
	if refused.DeckSpec.Structure != nil {
		t.Errorf("\"No agenda\" must keep a 16-slide draft flat: %+v", refused.DeckSpec.Structure)
	}

	raw, err := json.Marshal(BuildDeckSpecPlan(Params{Brief: "Pitch our Series B for an AI infra company", SlideBudget: 5}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"budget":{"requested":5,"planned":3,"content":2,"structural":1,"structural_slides":["cover"],"cut":[]}`, `"budget_note":"Planned 3 of 5 slides`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("plan JSON lacks %s:\n%s", want, raw)
		}
	}
}

func TestParseConstraints(t *testing.T) {
	tests := []struct {
		brief string
		want  []string // kind=value
		clean string
	}{
		{"Board deck on our Q3 FY26 results, 9-10 slides. Facts: revenue €48.2m", []string{"slide_count=9-10"}, "Board deck on our Q3 FY26 results. Facts: revenue €48.2m"},
		{"Investor update for Acme, 8 slides: headline results, ARR", []string{"slide_count=8"}, "Investor update for Acme: headline results, ARR"},
		{"7-slide investor pitch deck: problem, solution", []string{"slide_count=7"}, "investor pitch deck: problem, solution"},
		{"Strategy review in ten slides for the board", []string{"slide_count=ten"}, "Strategy review for the board"},
		{"QBR. Use the warm-coral template. Revenue grew 12%.", []string{"template=warm-coral"}, "QBR. Revenue grew 12%."},
		{"QBR, template: modern. Revenue grew 12%.", []string{"template=modern"}, "QBR. Revenue grew 12%."},
		{"QBR. Audience: the executive committee. Revenue grew 12%.", []string{"audience=the executive committee"}, "QBR. Revenue grew 12%."},
		{"A 15-minute pitch on pricing. Churn is 4%.", []string{"duration=15"}, "on pricing. Churn is 4%."},
		{"Kickoff deck with an agenda and section dividers. Scope is 14 depots.", []string{"structure=agenda, dividers"}, "Kickoff deck. Scope is 14 depots."},
		{"Kickoff deck, no agenda. Scope is 14 depots.", []string{"structure=no agenda"}, "Kickoff deck. Scope is 14 depots."},
		{"Pricing review aimed at investors and covering 3 options. Churn is 4%.", []string{"audience=investors"}, "Pricing review and covering 3 options. Churn is 4%."},
		{"Results presented to the board showed 12% growth", nil, "Results presented to the board showed 12% growth"},
		// Not instructions: a period label, a duration that is a metric, a
		// second slide count that is about content.
		{"Response time fell to 5 minutes and team of 4 grew to 12", nil, "Response time fell to 5 minutes and team of 4 grew to 12"},
		{"Board update for the board of directors: churn 4%", nil, "Board update for the board of directors: churn 4%"},
	}
	for _, tt := range tests {
		clean, cs := parseConstraints(tt.brief)
		var got []string
		for _, c := range cs {
			got = append(got, c.Kind+"="+c.Value)
			if !strings.Contains(tt.brief, c.Text) {
				t.Errorf("%q: constraint text %q is not the brief's own words", tt.brief, c.Text)
			}
		}
		if !reflect.DeepEqual(got, tt.want) || clean != tt.clean {
			t.Errorf("parseConstraints(%q) = %q, %q; want %q, %q", tt.brief, clean, got, tt.clean, tt.want)
		}
	}
	if n := statedSlideCount([]Constraint{{Kind: ConstraintSlideCount, Value: "9-10"}}); n != 10 {
		t.Errorf("statedSlideCount(9-10) = %d, want 10", n)
	}
	if n := statedSlideCount([]Constraint{{Kind: ConstraintSlideCount, Value: "ten"}}); n != 10 {
		t.Errorf("statedSlideCount(ten) = %d, want 10", n)
	}
}

func TestParseOutline(t *testing.T) {
	texts := func(o *briefOutline) []string {
		if o == nil {
			return nil
		}
		var out []string
		for _, it := range o.items {
			out = append(out, it.text)
		}
		return out
	}
	tests := []struct {
		name   string
		brief  string
		stated int
		want   []string
	}{
		{"topics after the topic", "Investor pitch: problem, solution, market, traction, team, ask", 0,
			[]string{"problem", "solution", "market", "traction", "team", "ask"}},
		{"count matches", "Update for Acme: headline results, ARR grew from $6.1m to $9.4m, three milestones (pilot Q1 2027, unit Q3 2027), main risk is permitting delay", 5,
			[]string{"headline results", "ARR grew from $6.1m to $9.4m", "three milestones (pilot Q1 2027, unit Q3 2027)", "main risk is permitting delay"}},
		{"labelled", "Kickoff for EMEA. Slides: agenda, results, win stories, next steps.", 0,
			[]string{"agenda", "results", "win stories", "next steps"}},
		{"semicolons", "Offsite deck. Outline: where we are; what changed, and why; what we do next", 0,
			[]string{"where we are", "what changed, and why", "what we do next"}},
		{"numbered lines", "Steerco update\n1. Status\n2. Budget\n3. Risks", 3, []string{"Status", "Budget", "Risks"}},
		{"slide lines", "Steerco update\nSlide 1: Status\nSlide 2: Budget\nSlide 3: Risks", 0, []string{"Status", "Budget", "Risks"}},
		// Content, not outlines.
		{"facts label", "Board deck. Facts: revenue €48m; margin 61%; churn 2%", 4, nil},
		{"list header", "Board deck. Three options: build, partner with Globex, acquire Initech, wait a year", 0, nil},
		{"metrics after the topic", "Q3 review: revenue up 12%, churn down to 2%, NPS 54, ARR €12m", 5, nil},
		{"bulleted metrics", "Q3 review\n- ARR reached €12.5m\n- churn fell to 2.1%\n- NPS 54\n- 12 AEs hired", 5, nil},
		{"two items", "Pitch: problem, solution", 0, nil},
		{"criteria", "We must compare three vendor options on five criteria: cost, reach, resilience, timeline, and integration.", 0, nil},
	}
	for _, tt := range tests {
		if got := texts(parseOutline(tt.brief, tt.stated)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: parseOutline = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// The splitter keeps what the journey briefs showed it cutting apart.
func TestSplitClausesKeepsListsAndLabels(t *testing.T) {
	texts := factTexts(extractBriefFacts("Results. quarterly revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2; gross margin 61.5% (Q2: 63.0%); recommendation: renegotiate now; main risk is permitting delay; Facts: churn 2%"))
	want := []string{"quarterly revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2", "gross margin 61.5% (Q2: 63.0%)", "recommendation: renegotiate now", "main risk is permitting delay", "churn 2%"}
	if !reflect.DeepEqual(texts, want) {
		t.Errorf("facts = %q, want %q", texts, want)
	}
	facts := extractBriefFacts("Margin review. Three options to fix margin: renegotiate the contract, re-platform storage, raise prices 5%; next steps: hire a CFO, open Berlin")
	groups := map[string]string{}
	for _, f := range facts {
		groups[f.text] = f.group
	}
	for text, group := range map[string]string{
		"Three options to fix margin": groupOption, "renegotiate the contract": groupOption, "re-platform storage": groupOption,
		"raise prices 5%": groupOption, "hire a CFO": groupAction, "open Berlin": groupAction,
	} {
		if groups[text] != group {
			t.Errorf("fact %q has group %q, want %q (facts %q)", text, groups[text], group, factTexts(facts))
		}
	}
	series := classifyFact("quarterly revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2", true)
	if !series.list || !series.series || !series.numeric {
		t.Errorf("a list of five values must classify as a series: %+v", series)
	}
	if f := classifyFact("ARR reached 12,500,000 in 2026", true); f.list {
		t.Errorf("a grouped thousands figure is not a list of values: %+v", f)
	}
}
