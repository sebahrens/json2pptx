package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-waft9: plan_deck format:"deckspec" drafts the DeckSpec the
// default path authors — kinds per narrative slot, not raw pattern skeletons.
func TestPlanDeckDeckSpecFormat(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	brief := "Board update on SMB churn. Churn rose to 4.2% in Q2, NRR fell to 108%, and the EU expansion slipped a quarter. We propose a dedicated success pod."
	res, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{
		"brief": brief, "slide_budget": float64(9), "format": "deckspec",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("plan_deck deckspec failed: %v %s", err, textContent(res))
	}
	var plan deckplan.DeckSpecPlan
	if err := json.Unmarshal([]byte(textContent(res)), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Format != "deckspec" || len(plan.Slots) == 0 {
		t.Fatalf("format=%q slots=%d, want a deckspec draft", plan.Format, len(plan.Slots))
	}

	// The draft is a DeckSpec the compiler parses as-is, and it renders within
	// the budget — generated agenda and dividers included.
	raw, err := json.Marshal(plan.DeckSpec)
	if err != nil {
		t.Fatal(err)
	}
	spec, diags := semantic.ParseJSON(raw)
	if spec == nil {
		t.Fatalf("draft does not parse as a DeckSpec: %+v\n%s", diags, raw)
	}
	if n := semantic.ExpandedSlideCount(spec); n > 9 {
		t.Errorf("draft renders %d slides, over the 9-slide budget", n)
	}

	var slots []string
	for _, slot := range plan.Slots {
		slots = append(slots, slot.Slot)
		if !semantic.SlideKind(slot.Kind).Valid() {
			t.Errorf("slot %s uses unknown DeckSpec kind %q", slot.Slot, slot.Kind)
		}
		if slot.Guidance == "" || slot.Path == "" {
			t.Errorf("slot %s has no guidance or path", slot.Slot)
		}
	}
	// A board update naming a problem and proposing a fix: the problem in the
	// brief's numbers, its causes, and the ask — between cover and close
	// (go-slide-creator-khzni).
	want := "cover answer problem cause ask closing"
	if got := strings.Join(slots, " "); got != want {
		t.Errorf("storyline = %q, want %q", got, want)
	}
	// The brief's numbers land on the problem slide (several -> KPI cards).
	problem := plan.Slots[2]
	if problem.Kind != "kpi_snapshot" || len(problem.Facts) < 2 {
		t.Errorf("problem slot = %+v, want kpi_snapshot carrying the churn/NRR facts", problem)
	}
	chrome, _ := plan.DeckSpec.Meta["chrome"].(map[string]any)
	if pn, _ := chrome["page_numbers"].(map[string]any); pn["enabled"] != true {
		t.Errorf("draft meta.chrome must turn page numbers on (go-slide-creator-1iy0x): %v", plan.DeckSpec.Meta)
	}
	if plan.UnplacedFacts == nil {
		t.Error("unplaced_facts must always be present")
	}
}

func TestPlanDeckDeckSpecSmallBudgetKeepsTheAsk(t *testing.T) {
	plan := deckplan.BuildDeckSpecPlan(deckplan.Params{Brief: "Approve the pilot budget", SlideBudget: 3})
	var kinds []string
	for _, s := range plan.Slots {
		kinds = append(kinds, s.Kind)
	}
	if got := strings.Join(kinds, " "); got != "title decision next_steps" {
		t.Errorf("3-slide storyline = %q, want the cover, the ask and the close", got)
	}
	big := deckplan.BuildDeckSpecPlan(deckplan.Params{Brief: "Strategy review", SlideBudget: 30})
	if len(big.Slots) >= 30 || big.BudgetNote == "" {
		t.Errorf("a 30-slide budget should not be padded with evidence slides: %d slots, note %q", len(big.Slots), big.BudgetNote)
	}
}

// khzni: a draft with back matter parses as a DeckSpec, closes on next steps
// before the appendix, and its appendix slides are outside the rhythm runs.
func TestPlanDeckDeckSpecAppendixDraftParses(t *testing.T) {
	for _, tc := range []struct {
		brief  string
		budget int
	}{
		{"Q3 quarterly business review: revenue grew 18% YoY to $42M, gross margin 61%, NRR 112%, churn rose to 4.1%; priorities for Q4 are hiring 12 AEs and fixing onboarding; include backup detail on the regional breakdown and the forecast methodology", 14},
		{"Product update for customers: we shipped SSO and audit logs in Q3; adoption is 38% of active accounts; include backup detail on adoption by segment", 8},
	} {
		plan := deckplan.BuildDeckSpecPlan(deckplan.Params{Brief: tc.brief, SlideBudget: tc.budget})
		raw, err := json.Marshal(plan.DeckSpec)
		if err != nil {
			t.Fatal(err)
		}
		spec, diags := semantic.ParseJSON(raw)
		if spec == nil {
			t.Fatalf("draft does not parse as a DeckSpec: %+v\n%s", diags, raw)
		}
		if n := semantic.ExpandedSlideCount(spec); n > tc.budget {
			t.Errorf("budget %d: draft renders %d slides", tc.budget, n)
		}
		ir := semantic.Normalize(spec)
		closeAt, firstBackup := -1, -1
		for i, s := range ir.Slides {
			if s.Kind == semantic.KindNextSteps {
				closeAt = i
			}
			if s.Appendix && firstBackup < 0 {
				firstBackup = i
			}
		}
		if firstBackup < 0 {
			t.Fatalf("budget %d: brief asks for backup detail but the draft has no appendix: %s", tc.budget, raw)
		}
		if closeAt < 0 || closeAt > firstBackup {
			t.Errorf("budget %d: next_steps (slide %d) must close the argument before the appendix (slide %d)", tc.budget, closeAt, firstBackup)
		}
		for i := firstBackup; i < len(ir.Slides); i++ {
			if !ir.Slides[i].Appendix {
				t.Errorf("budget %d: slide %d (%s) after the appendix divider is not back matter", tc.budget, i, ir.Slides[i].Kind)
			}
		}
	}
}

func TestPlanDeckRejectsUnknownFormat(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	res, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{"brief": "x", "format": "pptx"}))
	if err != nil || res == nil || !res.IsError || !strings.Contains(textContent(res), "format") {
		t.Fatalf("unknown format should be an INVALID_PARAMETER on format: %v %s", err, textContent(res))
	}
}

// The three briefs of the 2026-10-03 agent-journey review, driven through the
// plan_deck handler exactly as the journey agents called it
// (tests/quality/results/agent-journey-20261003/*/log-calls.jsonl):
// go-slide-creator-hf8tf, go-slide-creator-58qda.
func TestPlanDeckJourneyBriefs(t *testing.T) {
	mc := testMCPConfig(t)
	cases := []struct {
		name string
		args map[string]any
		// kinds is the draft's slide kinds in order.
		kinds []string
		// facts must each be one slot's fact, verbatim.
		facts []string
		// constraint is the slide-count instruction, which is no fact.
		constraint string
		planned    int
	}{
		{
			name: "A5 board results",
			args: map[string]any{
				"brief":    "Board deck on our Q3 FY26 results, 9-10 slides. Facts: revenue \u20ac48.2m (+14% YoY, plan \u20ac46.0m); gross margin 61.5% (Q2: 63.0%) because cloud costs rose 22%; net revenue retention 112%; churn 2.1% monthly in SMB vs 0.6% enterprise; quarterly revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2; three options to fix margin: renegotiate cloud contract (saves \u20ac1.2m/yr, 2 months), re-platform storage tier (saves \u20ac2.0m/yr, 6 months, \u20ac0.8m one-off), raise SMB prices 5% (adds \u20ac0.9m/yr, churn risk); recommendation: renegotiate now and start re-platforming; next steps with owners CFO / CTO / CRO and dates in October\u2013December 2026; source: management accounts Q3 FY26.",
				"audience": "board of directors", "format": "deckspec", "slide_budget": float64(10), "template": "warm-coral",
			},
			facts: []string{
				"quarterly revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2",
				"re-platform storage tier (saves \u20ac2.0m/yr, 6 months, \u20ac0.8m one-off)",
				"raise SMB prices 5% (adds \u20ac0.9m/yr, churn risk)",
				"recommendation: renegotiate now and start re-platforming",
			},
			constraint: "9-10 slides", planned: 10,
		},
		{
			name: "E1/E2 investor update",
			args: map[string]any{
				"brief":  "Investor update for a fictional climate-tech company, 8 slides: headline results, ARR grew from $6.1m to $9.4m in 12 months, burn fell from $1.1m to $0.7m a month, 18 months runway, three product milestones (pilot plant Q1 2027, first commercial unit Q3 2027, series B Q4 2027), main risk is permitting delay, ask is introductions to two strategic partners.",
				"format": "deckspec", "slide_budget": float64(8), "audience": "investors", "template": "modern-yellow",
			},
			kinds:      []string{"title", "executive_summary", "chart_insight", "chart_insight", "stat", "timeline", "table", "next_steps"},
			facts:      []string{"main risk is permitting delay", "ask is introductions to two strategic partners", "18 months runway"},
			constraint: "8 slides", planned: 8,
		},
		{
			name: "A14 investor pitch",
			args: map[string]any{
				"brief":  "7-slide investor pitch deck for a fictional climate-tech startup: problem, solution, market size chart, traction KPIs, business model, team of 4, the ask",
				"format": "deckspec", "slide_budget": float64(7),
			},
			kinds:      []string{"title", "pillars", "pillars", "chart_insight", "kpi_snapshot", "pillars", "team", "next_steps"},
			facts:      []string{"problem", "solution", "market size chart", "traction KPIs", "business model", "team of 4", "the ask"},
			constraint: "7-slide", planned: 8,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := mc.handlePlanDeck(context.Background(), makeRequest(tc.args))
			if err != nil || res == nil || res.IsError {
				t.Fatalf("plan_deck failed: %v %s", err, textContent(res))
			}
			body := textContent(res)
			var plan deckplan.DeckSpecPlan
			if err := json.Unmarshal([]byte(body), &plan); err != nil {
				t.Fatal(err)
			}

			// 58qda: flat under 12 slides, and the budget is always stated.
			if plan.DeckSpec.Structure != nil {
				t.Errorf("draft spends the budget on an agenda and dividers: %+v", plan.DeckSpec.Structure)
			}
			var kinds, all []string
			for _, s := range plan.DeckSpec.Slides {
				kinds = append(kinds, s["kind"].(string))
			}
			for _, k := range kinds {
				if k == "agenda" || k == "section" {
					t.Errorf("draft carries a %s slide the brief did not ask for: %v", k, kinds)
				}
			}
			if tc.kinds != nil && strings.Join(kinds, " ") != strings.Join(tc.kinds, " ") {
				t.Errorf("kinds = %v, want one slide per outline item: %v", kinds, tc.kinds)
			}
			b := plan.Budget
			if b.Requested != plan.SlideBudget || b.Planned != tc.planned || b.Planned != len(kinds) || b.Structural != 1 || b.Content != tc.planned-1 {
				t.Errorf("budget = %+v, want %d planned: the cover and %d content slides", b, tc.planned, tc.planned-1)
			}
			if !strings.Contains(body, `"budget_note":"Planned `) || !strings.Contains(body, `"cut":[]`) {
				t.Errorf("response must always carry budget_note and budget.cut: %s", body)
			}

			// hf8tf: the facts reach a slot whole; the instruction is a
			// constraint and reaches none.
			for _, s := range plan.Slots {
				all = append(all, s.Facts...)
				for _, f := range s.Facts {
					if strings.Contains(f, tc.constraint) {
						t.Errorf("deck instruction %q routed to slot %s as the fact %q", tc.constraint, s.Slot, f)
					}
				}
			}
			for _, want := range tc.facts {
				found := false
				for _, f := range all {
					found = found || f == want
				}
				if !found {
					t.Errorf("fact %q is on no slot (unplaced: %q)", want, plan.UnplacedFacts)
				}
			}
			if len(plan.Constraints) != 1 || plan.Constraints[0].Kind != deckplan.ConstraintSlideCount || plan.Constraints[0].Text != tc.constraint {
				t.Errorf("constraints = %+v, want the slide count %q", plan.Constraints, tc.constraint)
			}

			// The draft parses, and validate flags every __FILL__ it carries
			// — meta.date included (E18).
			raw, err := json.Marshal(plan.DeckSpec)
			if err != nil {
				t.Fatal(err)
			}
			spec, diags := semantic.ParseJSON(raw)
			if spec == nil {
				t.Fatalf("draft does not parse as a DeckSpec: %+v\n%s", diags, raw)
			}
			if n := semantic.ExpandedSlideCount(spec); n != tc.planned {
				t.Errorf("draft renders %d slides, budget.planned says %d", n, tc.planned)
			}
			flagged := false
			for _, d := range semantic.Validate(spec, semantic.StrictnessWarn) {
				flagged = flagged || (d.Code == "SEMANTIC_WEAK_CONTENT" && d.Path == "meta.date")
			}
			if !flagged {
				t.Errorf("validate did not flag meta.date %q", spec.Meta.Date)
			}
		})
	}
}

// Without slide_budget the plan takes the slide count the brief states; the
// raw format follows an enumerated outline too.
func TestPlanDeckBudgetFromBriefAndRawOutline(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	brief := "7-slide investor pitch deck for a fictional climate-tech startup: problem, solution, market size chart, traction KPIs, business model, team of 4, the ask"
	res, err := mc.handlePlanDeck(context.Background(), makeRequest(map[string]any{"brief": brief}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("plan_deck failed: %v %s", err, textContent(res))
	}
	var plan deckplan.Result
	if err := json.Unmarshal([]byte(textContent(res)), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SlideBudget != 7 || plan.Budget.Requested != 7 {
		t.Errorf("slide_budget = %d, want the 7 the brief states", plan.SlideBudget)
	}
	var got []string
	for _, s := range plan.Slides {
		got = append(got, s.RecommendedPattern)
		if s.RecommendedPattern != "" && len(s.Skeleton) == 0 {
			t.Errorf("slide %d (%s) has no skeleton", s.SlideIndex, s.RecommendedPattern)
		}
	}
	want := " card-grid card-grid chart-insights-split kpi-3up card-grid team-bios next-steps"
	if strings.Join(got, " ") != want {
		t.Errorf("raw outline patterns = %q, want %q", strings.Join(got, " "), want)
	}
	if plan.Budget.Planned != 8 || plan.Budget.Content != 7 || !strings.Contains(plan.BudgetNote, "1 over the budget") {
		t.Errorf("budget = %+v, note %q", plan.Budget, plan.BudgetNote)
	}
}
