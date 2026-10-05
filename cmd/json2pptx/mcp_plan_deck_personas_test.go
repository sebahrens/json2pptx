package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-xbwlt: four product-only agent journeys (f-deals-pitch,
// g-risk-consulting, h-tech-data, i-risk-assurance) discarded plan_deck's
// format:"deckspec" draft because it routed the slides the brief named in the
// kind catalogue's own words to narrative defaults (a margin bridge to
// pillars, a photo case to kpi_snapshot, a pricing schedule to timeline, a
// chart slide to executive_summary, a heat map to table), stranded 13–28
// facts, emitted an ellipsised brief fragment as the deck title, drafted a
// duplicate closer, and spent the slide budget on structure.
//
// The corpus is the four persona briefs verbatim
// (tests/quality/journey/personas/{f,g,h,i}-*.md, the blockquote) and the
// exact brief each journey agent sent to plan_deck (the "journey" rows, with
// the arguments it passed). Required kinds follow each persona's S1–S4 and
// "Also required" lines; where the blockquote alone cannot say a slide is
// split (S1 wants the chart beside the share number) the row accepts the
// chart on its own.
//
// unplaced_facts ≤ 5: each brief carries one to three clauses no kind owns
// ("We are pitching a 4-week commercial due diligence", "Management has
// accepted all seven actions"); five leaves room for those while a stranded
// block — three options, four phases, six heat-map placements — is at least
// three facts and still fails.

const maxPersonaUnplacedFacts = 5

type personaPlanCase struct {
	name   string
	brief  string
	args   map[string]any
	deck   string   // the deck name the brief gives: the cover title
	minMax [2]int   // the brief's slide range (or the slide_budget passed)
	kinds  []string // required kinds; "a|b" accepts either
}

const (
	personaBriefF = "Build the proposal deck for \"Project Falcon\": Meridian Capital Partners is considering acquiring Nordbolt, a European industrial fastener maker (revenue EUR 212M, EBITDA EUR 31M, 14.6% margin). We are pitching a 4-week commercial due diligence. 10–12 slides on `p-style` when `$REPO/templates/p-style.pptx` exists, otherwise `blue-corporate`. Facts: the European fastener market grew from EUR 8.1bn (2021) to EUR 9.4bn (2025), forecast EUR 10.6bn by 2028; Nordbolt holds 2.3% share, number 6 in Europe; top three competitors are Bossard (9.1%), Würth (7.4%) and Fabory (3.8%). Nordbolt's revenue by segment: automotive 41%, construction 27%, machinery 22%, other 10%. Margin bridge 2023→2025: 2023 EBITDA 24.0, price +6.5, volume +3.2, raw material −4.1, opex +1.4 = 31.0. Three scope options for the DD: (A) desktop-only market model, EUR 180k, 3 weeks; (B) market model plus 25 customer interviews, EUR 320k, 4 weeks; (C) B plus a channel-partner survey, EUR 410k, 5 weeks — we recommend B. Workplan (B): week 1 data room and market model, week 2 interviews wave 1, week 3 interviews wave 2 and synthesis, week 4 red-flag report and IC pack. Team: Anna Lindqvist (partner, 18 years industrials), Marco Rossi (manager, 3 prior fastener DDs), two consultants. A comparable: last year's DD on \"Project Keel\" (fastener distributor) found a 9-point price-gap risk that moved the bid by EUR 40M. Fees for B: market model 120k, interviews 140k, synthesis and IC pack 60k. Pricing schedule has eight line items you may invent sensibly (phase, deliverable, days, fee). The ask: confirm scope B and sign the engagement letter by Friday."

	personaBriefG = "Build the proposal for \"Harbour Bank\" (mid-sized retail bank, EUR 42bn assets, 3,800 staff): a 12-month enterprise risk management (ERM) uplift after the regulator's 2025 review rated the bank \"needs improvement\" on risk governance. 10–12 slides on `midnight-blue`. Facts: the regulator raised 14 findings — 3 high (risk appetite not cascaded, 2nd line under-resourced, no integrated risk reporting), 6 medium, 5 low; remediation deadline 30 June 2027. Current operational-risk losses: 2023 EUR 6.1M, 2024 EUR 8.4M, 2025 EUR 11.2M; peer median is about 0.02% of assets. Risk appetite dashboard today: credit — within appetite (NPL 2.1% vs limit 3%), liquidity — within (LCR 148% vs 110%), operational — breached (losses EUR 11.2M vs EUR 8M), conduct — amber (37 complaints per 10k vs 30), cyber — breached (2 critical unpatched vulnerabilities older than 30 days vs 0). Three lines of defence: 1st line business units own controls (1,200 control owners), 2nd line Risk & Compliance (28 FTE, target 44), 3rd line Internal Audit (12 FTE). Target operating model has four pillars: governance and appetite, risk identification and assessment, control framework, reporting and data. Three delivery options: (1) advisory only, EUR 0.9M, we design and the bank implements; (2) co-delivery, EUR 2.1M, joint team; (3) full outsource of the programme office, EUR 3.4M. Recommend option 2. Roadmap: Mobilise (Jul–Aug 2026), Design (Sep–Nov 2026), Build (Dec 2026–Mar 2027), Embed (Apr–Jun 2027); parallel tracks: data and reporting platform, training 1,200 control owners. Top risks on a likelihood/impact view: cyber (high/high), third-party outage (medium/high), conduct (medium/medium), model risk (low/high), climate (low/medium), fraud (medium/low). Ask: approve option 2 and a steering committee chaired by the CRO."

	personaBriefH = "Build the steering-committee business case for \"Atlas Retail\" (EUR 3.1bn revenue, 640 stores): replace the on-premise data warehouse with a cloud lakehouse. 11–13 slides on `modern-template`. Facts: today 14 source systems feed a 2009 warehouse through 1,900 hand-written ETL jobs; nightly load takes 9.5 hours and missed its 6 am SLA on 61 of the last 90 days; data quality: 23% of product records have a missing or inconsistent attribute; 4 FTE spend 60% of their time on reconciliations. Target architecture, top to bottom: consumption (Power BI, 3 data products, ML feature store), serving (semantic layer, governed marts), processing (Spark batch, streaming), storage (Delta lake, bronze/silver/gold), ingestion (CDC from 14 sources, event streaming), with security/governance and FinOps as cross-cutting. Three vendor options scored on cost, migration risk, skills availability, lock-in: Databricks, Snowflake, a native hyperscaler stack; recommend Databricks. Five-year TCO: current EUR 18.4M, target EUR 12.9M (saving EUR 5.5M); year-by-year run cost current 3.6/3.7/3.7/3.7/3.7 vs target 4.8/3.1/1.9/1.6/1.5 (year 1 includes migration). Migration roadmap: Foundation (Q1–Q2 2027: landing zone, governance, 3 pilot sources), Migrate (Q3 2027–Q2 2028: 14 sources, 1,900 jobs refactored to 400 pipelines), Optimise (Q3–Q4 2028: decommission warehouse); parallel tracks: data literacy for 300 analysts, FinOps guardrails. KPIs after: load window 9.5 h → 1.5 h, SLA misses 61 → under 3 per 90 days, product-record defects 23% → under 5%, reconciliation effort −70%. A cost driver view: run cost splits into compute 48%, storage 12%, licences 22%, people 18%. The ops console screenshot at `$J/assets/screenshot-ops-console.png` shows the nightly load dashboard; call out the SLA breach banner (roughly top-left) and the job queue (roughly centre). Ask: approve EUR 4.8M for year 1 and the Databricks contract."

	personaBriefI = "Build the audit-committee readout for \"Cobalt Insurance\": the FY26 IT general controls (ITGC) and SOX-style assurance review across five domains. 12–14 slides on `forest-green`. Facts: scope was 5 domains, 48 key controls, 1,160 samples tested across 9 applications. Results by domain — access management: 14 controls, 3 exceptions, rating red; change management: 11 controls, 1 exception, amber; computer operations: 9 controls, 0 exceptions, green; program development: 6 controls, 1 exception, amber; third-party / cloud: 8 controls, 2 exceptions, red. Overall opinion: \"partially effective\". Seven findings (invent titles consistent with the domains), each with a severity (2 high, 3 medium, 2 low), an owner and a due date between Dec 2026 and Jun 2027; the two high findings are \"privileged access not recertified for 3 of 9 applications\" and \"cloud provider SOC 2 reports not reviewed for 2 of 4 providers\". Testing approach: plan (scoping, risk assessment), walkthroughs, design evaluation, operating-effectiveness testing, reporting. Year-on-year: exceptions 2024: 11, 2025: 9, 2026: 7; high-rated findings 2024: 4, 2025: 3, 2026: 2. Control maturity by domain on a 1–5 scale: access 2, change 3, operations 4, development 3, third-party 2; target 4 everywhere by FY28. Remediation plan: Q4 2026 quick wins (recertification run, SOC 2 review), Q1–Q2 2027 access tooling, Q3 2027 re-test. Management has accepted all seven actions. Ask: the committee notes the opinion and endorses the remediation plan."

	// The briefs the journey agents actually sent (/tmp/jj/wave/<persona>/log/calls.jsonl).
	journeyBriefF = "Proposal deck for Project Falcon: Meridian Capital Partners is considering acquiring Nordbolt, a European industrial fastener maker (revenue EUR 212M, EBITDA EUR 31M, 14.6% margin). We are pitching a 4-week commercial due diligence. Facts: the European fastener market grew from EUR 8.1bn (2021) to EUR 9.4bn (2025), forecast EUR 10.6bn by 2028; Nordbolt holds 2.3% share, number 6 in Europe; top three competitors are Bossard (9.1%), Wuerth (7.4%) and Fabory (3.8%). Nordbolt revenue by segment: automotive 41%, construction 27%, machinery 22%, other 10%. Margin bridge 2023 to 2025: 2023 EBITDA 24.0, price +6.5, volume +3.2, raw material -4.1, opex +1.4 = 31.0. Three scope options for the DD: (A) desktop-only market model, EUR 180k, 3 weeks; (B) market model plus 25 customer interviews, EUR 320k, 4 weeks; (C) B plus a channel-partner survey, EUR 410k, 5 weeks; we recommend B. Workplan (B): week 1 data room and market model, week 2 interviews wave 1, week 3 interviews wave 2 and synthesis, week 4 red-flag report and IC pack. Team: Anna Lindqvist (partner, 18 years industrials), Marco Rossi (manager, 3 prior fastener DDs), two consultants. Comparable: last year's DD on Project Keel (fastener distributor) found a 9-point price-gap risk that moved the bid by EUR 40M. Fees for B: market model 120k, interviews 140k, synthesis and IC pack 60k; an eight-line pricing schedule (phase, deliverable, days, fee). The ask: confirm scope B and sign the engagement letter by Friday. Required slides: market chart beside the 2.3% share number and position bullets; margin bridge waterfall beside a what-it-means narrative; Project Keel case with site photo and result metrics; two-column comparison B vs C; three-option evaluation matrix (cost, duration, confidence on price risk, confidence on volume risk); eight-row pricing schedule; team with headshot; next steps closer with the Friday ask."

	journeyBriefG = "Proposal for Harbour Bank (mid-sized retail bank, EUR 42bn assets, 3,800 staff): a 12-month enterprise risk management (ERM) uplift after the regulator's 2025 review rated the bank 'needs improvement' on risk governance. 10-12 slides. Facts: the regulator raised 14 findings: 3 high (risk appetite not cascaded, 2nd line under-resourced, no integrated risk reporting), 6 medium, 5 low; remediation deadline 30 June 2027. Current operational-risk losses: 2023 EUR 6.1M, 2024 EUR 8.4M, 2025 EUR 11.2M; peer median is about 0.02% of assets. Risk appetite dashboard today: credit within appetite (NPL 2.1% vs limit 3%), liquidity within (LCR 148% vs 110%), operational breached (losses EUR 11.2M vs EUR 8M), conduct amber (37 complaints per 10k vs 30), cyber breached (2 critical unpatched vulnerabilities older than 30 days vs 0). Three lines of defence: 1st line business units own controls (1,200 control owners), 2nd line Risk & Compliance (28 FTE, target 44), 3rd line Internal Audit (12 FTE). Target operating model has four pillars: governance and appetite, risk identification and assessment, control framework, reporting and data. Three delivery options: (1) advisory only, EUR 0.9M, we design and the bank implements; (2) co-delivery, EUR 2.1M, joint team; (3) full outsource of the programme office, EUR 3.4M. Recommend option 2. Roadmap: Mobilise (Jul-Aug 2026), Design (Sep-Nov 2026), Build (Dec 2026-Mar 2027), Embed (Apr-Jun 2027); parallel tracks: data and reporting platform, training 1,200 control owners. Top risks on a likelihood/impact view: cyber (high/high), third-party outage (medium/high), conduct (medium/medium), model risk (low/high), climate (low/medium), fraud (medium/low). Ask: approve option 2 and a steering committee chaired by the CRO. Required slides: an action-titled executive summary; the risk appetite dashboard as a status board (five risk types with status, metric, limit); op-risk losses chart 2023-2025 on the left with the headline EUR 11.2M and peer comparison plus two bullets on the right; a likelihood x impact heat map with the six risks; current vs target three lines of defence in two aligned columns; the four-pillar operating model; the three-option decision with option 2 recommended; the phased roadmap with the two parallel tracks; a next-steps closer."

	journeyBriefH = "Steering-committee business case for Atlas Retail (EUR 3.1bn revenue, 640 stores): replace the on-premise data warehouse with a cloud lakehouse. 11-13 slides. Facts: today 14 source systems feed a 2009 warehouse through 1,900 hand-written ETL jobs; nightly load takes 9.5 hours and missed its 6 am SLA on 61 of the last 90 days; data quality: 23% of product records have a missing or inconsistent attribute; 4 FTE spend 60% of their time on reconciliations. Target architecture, top to bottom: consumption (Power BI, 3 data products, ML feature store), serving (semantic layer, governed marts), processing (Spark batch, streaming), storage (Delta lake, bronze/silver/gold), ingestion (CDC from 14 sources, event streaming), with security/governance and FinOps as cross-cutting side rails. Three vendor options scored on cost, migration risk, skills availability, lock-in: Databricks, Snowflake, a native hyperscaler stack; recommend Databricks. Five-year TCO: current EUR 18.4M, target EUR 12.9M (saving EUR 5.5M); year-by-year run cost current 3.6/3.7/3.7/3.7/3.7 vs target 4.8/3.1/1.9/1.6/1.5 (year 1 includes migration). Migration roadmap: Foundation (Q1-Q2 2027: landing zone, governance, 3 pilot sources), Migrate (Q3 2027-Q2 2028: 14 sources, 1,900 jobs refactored to 400 pipelines), Optimise (Q3-Q4 2028: decommission warehouse); parallel tracks: data literacy for 300 analysts, FinOps guardrails. KPIs after: load window 9.5 h to 1.5 h, SLA misses 61 to under 3 per 90 days, product-record defects 23% to under 5%, reconciliation effort -70%. Cost driver view: run cost splits into compute 48%, storage 12%, licences 22%, people 18%. An ops console screenshot shows the nightly load dashboard with an SLA breach banner (top-left) and the job queue (centre). Ask: approve EUR 4.8M for year 1 and the Databricks contract."

	journeyBriefI = "Build the audit-committee readout for Cobalt Insurance: the FY26 IT general controls (ITGC) and SOX-style assurance review across five domains. 12-14 slides. Facts: scope was 5 domains, 48 key controls, 1,160 samples tested across 9 applications. Results by domain: access management 14 controls, 3 exceptions, rating red; change management 11 controls, 1 exception, amber; computer operations 9 controls, 0 exceptions, green; program development 6 controls, 1 exception, amber; third-party / cloud 8 controls, 2 exceptions, red. Overall opinion: partially effective. Seven findings, each with a severity (2 high, 3 medium, 2 low), an owner and a due date between Dec 2026 and Jun 2027; the two high findings are 'privileged access not recertified for 3 of 9 applications' and 'cloud provider SOC 2 reports not reviewed for 2 of 4 providers'. Testing approach: plan (scoping, risk assessment), walkthroughs, design evaluation, operating-effectiveness testing, reporting. Year-on-year: exceptions 2024: 11, 2025: 9, 2026: 7; high-rated findings 2024: 4, 2025: 3, 2026: 2. Control maturity by domain on a 1-5 scale: access 2, change 3, operations 4, development 3, third-party 2; target 4 everywhere by FY28. Remediation plan: Q4 2026 quick wins (recertification run, SOC 2 review), Q1-Q2 2027 access tooling, Q3 2027 re-test. Management has accepted all seven actions. Ask: the committee notes the opinion and endorses the remediation plan. Required: results-by-domain board with RAG rating visible as colour; the seven-finding register as a table (title, severity, owner, due date); a year-on-year trend chart on the left with the overall opinion as headline on the right; control maturity today vs FY28 target as a ladder; scope and approach as a five-step process; the two high findings each on their own structured slide; the remediation roadmap; an action-titled executive summary; a next-steps closer with the committee ask; detailed testing statistics in an appendix."
)

func personaPlanCases() []personaPlanCase {
	return []personaPlanCase{
		{
			name: "f-deals-pitch/blockquote", brief: personaBriefF, deck: "Project Falcon", minMax: [2]int{10, 12},
			kinds: []string{"executive_summary", "chart_insight|regions", "bridge", "image_case", "table", "team", "decision|option_matrix", "roadmap", "next_steps"},
		},
		{
			name: "g-risk-consulting/blockquote", brief: personaBriefG, deck: "Harbour Bank", minMax: [2]int{10, 12},
			kinds: []string{"executive_summary", "kpi_snapshot|table", "chart_insight|regions", "risk_heatmap", "pillars", "decision|option_matrix", "roadmap", "next_steps"},
		},
		{
			name: "h-tech-data/blockquote", brief: personaBriefH, deck: "Atlas Retail", minMax: [2]int{11, 13},
			kinds: []string{"executive_summary", "kpi_snapshot", "architecture", "option_matrix", "chart_insight|regions", "roadmap", "comparison", "image_case", "next_steps"},
		},
		{
			name: "i-risk-assurance/blockquote", brief: personaBriefI, deck: "Cobalt Insurance", minMax: [2]int{12, 14},
			kinds: []string{"executive_summary", "kpi_snapshot", "table", "process", "chart_insight|regions", "comparison", "roadmap", "next_steps"},
		},
		{
			name: "f-deals-pitch/journey", brief: journeyBriefF, deck: "Project Falcon", minMax: [2]int{9, 11},
			args:  map[string]any{"slide_budget": float64(11), "audience": "private equity deal team and investment committee"},
			kinds: []string{"regions", "bridge", "image_case", "comparison", "option_matrix", "table", "team", "roadmap", "next_steps"},
		},
		{
			name: "g-risk-consulting/journey", brief: journeyBriefG, deck: "Harbour Bank", minMax: [2]int{10, 11},
			args:  map[string]any{"slide_budget": float64(11), "audience": "bank executive committee and CRO"},
			kinds: []string{"executive_summary", "kpi_snapshot|table", "regions", "risk_heatmap", "comparison", "pillars", "decision", "roadmap", "next_steps"},
		},
		{
			name: "h-tech-data/journey", brief: journeyBriefH, deck: "Atlas Retail", minMax: [2]int{11, 13},
			kinds: []string{"executive_summary", "kpi_snapshot", "architecture", "option_matrix", "chart_insight|regions", "roadmap", "comparison", "image_case", "next_steps"},
		},
		{
			name: "i-risk-assurance/journey", brief: journeyBriefI, deck: "Cobalt Insurance", minMax: [2]int{12, 14},
			kinds: []string{"executive_summary", "table", "regions|chart_insight", "comparison", "process", "roadmap", "next_steps"},
		},
	}
}

// planFor runs plan_deck format:"deckspec" through the MCP handler, as the
// journeys did, and decodes the plan.
func planFor(t *testing.T, tc personaPlanCase) deckplan.DeckSpecPlan {
	t.Helper()
	mc := &mcpConfig{templatesDir: "../../templates"}
	args := map[string]any{"brief": tc.brief, "format": "deckspec"}
	for k, v := range tc.args {
		args[k] = v
	}
	res, err := mc.handlePlanDeck(context.Background(), makeRequest(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("plan_deck failed: %v %s", err, textContent(res))
	}
	var plan deckplan.DeckSpecPlan
	if err := json.Unmarshal([]byte(textContent(res)), &plan); err != nil {
		t.Fatal(err)
	}
	return plan
}

// draftTitles collects every title in the draft, cover and structure included.
func draftTitles(v any, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		if s, ok := x["title"].(string); ok {
			*out = append(*out, s)
		}
		for _, child := range x {
			draftTitles(child, out)
		}
	case []any:
		for _, child := range x {
			draftTitles(child, out)
		}
	case []map[string]any:
		for _, child := range x {
			draftTitles(child, out)
		}
	}
}

func kindCounts(plan deckplan.DeckSpecPlan) map[string]int {
	counts := map[string]int{}
	for _, s := range plan.Slots {
		counts[s.Kind]++
	}
	return counts
}

func TestPlanDeckPersonaBriefsRouteNamedSlides(t *testing.T) {
	for _, tc := range personaPlanCases() {
		t.Run(tc.name, func(t *testing.T) {
			plan := planFor(t, tc)
			counts := kindCounts(plan)
			var kinds []string
			for _, s := range plan.Slots {
				kinds = append(kinds, s.Kind)
			}
			t.Logf("kinds: %s; unplaced %d; budget %+v", strings.Join(kinds, " "), len(plan.UnplacedFacts), plan.Budget)

			// 1. The kinds the brief names are the kinds drafted.
			for _, want := range tc.kinds {
				found := false
				for _, alt := range strings.Split(want, "|") {
					if counts[alt] > 0 {
						found = true
					}
				}
				if !found {
					t.Errorf("required kind %q missing from %v", want, kinds)
				}
			}

			// 2. One closer: the brief names its ask, so no automatic second
			// next_steps is added.
			if counts["next_steps"] != 1 {
				t.Errorf("next_steps drafted %d times, want exactly one closer", counts["next_steps"])
			}
			if counts["executive_summary"] > 1 {
				t.Errorf("executive_summary drafted %d times", counts["executive_summary"])
			}

			// 3. Titles: the deck name on the cover and in meta, never an
			// ellipsised fragment of the brief; unknown titles are __FILL__.
			raw, err := json.Marshal(plan.DeckSpec)
			if err != nil {
				t.Fatal(err)
			}
			var generic map[string]any
			if err := json.Unmarshal(raw, &generic); err != nil {
				t.Fatal(err)
			}
			var titles []string
			draftTitles(generic, &titles)
			for _, title := range titles {
				if strings.Contains(title, "…") || strings.Contains(title, "...") {
					t.Errorf("title %q is an ellipsised brief fragment", title)
				}
			}
			if got := plan.DeckSpec.Meta["title"]; got != tc.deck {
				t.Errorf("meta.title = %v, want the deck name %q", got, tc.deck)
			}
			cover := map[string]any{}
			if plan.DeckSpec.Structure != nil {
				cover = plan.DeckSpec.Structure.Cover
			} else if len(plan.DeckSpec.Slides) > 0 {
				cover = plan.DeckSpec.Slides[0]
			}
			if cover["title"] != tc.deck {
				t.Errorf("cover title = %v, want %q", cover["title"], tc.deck)
			}

			// 4. The facts reach the slots.
			if n := len(plan.UnplacedFacts); n > maxPersonaUnplacedFacts {
				t.Errorf("unplaced_facts = %d, want <= %d:\n  %s", n, maxPersonaUnplacedFacts, strings.Join(plan.UnplacedFacts, "\n  "))
			}

			// 5. The slide budget is the brief's range, structure included.
			spec, diags := semantic.ParseJSON(raw)
			if spec == nil {
				t.Fatalf("draft does not parse as a DeckSpec: %+v", diags)
			}
			if n := semantic.ExpandedSlideCount(spec); n < tc.minMax[0] || n > tc.minMax[1] {
				t.Errorf("draft renders %d slides, want %d–%d (%s)", n, tc.minMax[0], tc.minMax[1], plan.BudgetNote)
			}
			if plan.Budget.Planned != semantic.ExpandedSlideCount(spec) {
				t.Errorf("budget.planned %d != rendered %d", plan.Budget.Planned, semantic.ExpandedSlideCount(spec))
			}
		})
	}
}

// The matched facts land in the kind's own fields, not only in slots[].facts.
func TestPlanDeckPersonaDraftsCarryStructuredFields(t *testing.T) {
	slideByKind := func(plan deckplan.DeckSpecPlan, kind string) map[string]any {
		var all []map[string]any
		all = append(all, plan.DeckSpec.Slides...)
		if st := plan.DeckSpec.Structure; st != nil {
			for _, sec := range st.Sections {
				all = append(all, sec.Slides...)
			}
		}
		// The first slide of the kind that carries fields beyond kind and
		// title; a default sink slot (kpi_snapshot for leftover numbers) has
		// none and comes second.
		var bare map[string]any
		for _, s := range all {
			if s["kind"] != kind {
				continue
			}
			if len(s) > 2 {
				return s
			}
			if bare == nil {
				bare = s
			}
		}
		return bare
	}
	listLen := func(s map[string]any, field string) int {
		l, _ := s[field].([]any)
		return len(l)
	}

	f := planFor(t, personaPlanCases()[0])
	if bridge := slideByKind(f, "bridge"); bridge == nil || listLen(bridge, "columns") != 6 {
		t.Errorf("f: bridge columns = %v, want 6 (2023 EBITDA, price, volume, raw material, opex, 2025 EBITDA)", bridge)
	} else {
		cols, _ := bridge["columns"].([]any)
		first, _ := cols[0].(map[string]any)
		last, _ := cols[5].(map[string]any)
		if first["type"] != "total" || fmt.Sprint(first["value"]) != "24" || last["type"] != "total" || fmt.Sprint(last["value"]) != "31" {
			t.Errorf("f: bridge ends = %v … %v, want totals 24 and 31", first, last)
		}
	}
	if d := slideByKind(f, "decision"); d != nil {
		if listLen(d, "options") != 3 || d["recommendation"] == nil {
			t.Errorf("f: decision options = %v, want (A) (B) (C) with B recommended", d)
		}
	} else if om := slideByKind(f, "option_matrix"); om == nil || listLen(om, "options") != 3 {
		t.Errorf("f: no decision / option_matrix with the three scope options")
	}
	if r := slideByKind(f, "roadmap"); r == nil || listLen(r, "phases") != 4 {
		t.Errorf("f: roadmap phases = %v, want the four workplan weeks", r)
	}
	if team := slideByKind(f, "team"); team == nil || listLen(team, "members") < 3 {
		t.Errorf("f: team members = %v, want Anna, Marco and the consultants", team)
	}

	g := planFor(t, personaPlanCases()[1])
	if k := slideByKind(g, "kpi_snapshot"); k == nil || listLen(k, "kpis") < 3 {
		t.Errorf("g: kpi_snapshot kpis = %v, want the metric: value clauses", k)
	}
	if m := slideByKind(g, "risk_heatmap"); m == nil || listLen(m, "items") != 6 {
		t.Errorf("g: risk_heatmap = %v, want the six risks, each with its likelihood and impact", m)
	} else {
		raw, _ := json.Marshal(m["items"])
		for _, risk := range []string{"cyber", "third-party outage", "conduct", "model risk", "climate", "fraud"} {
			if !strings.Contains(string(raw), risk) {
				t.Errorf("g: heat map does not place %q: %s", risk, raw)
			}
		}
	}
	if r := slideByKind(g, "roadmap"); r == nil || listLen(r, "phases") != 4 {
		t.Errorf("g: roadmap phases = %v, want Mobilise / Design / Build / Embed", r)
	} else {
		phase, _ := r["phases"].([]any)[0].(map[string]any)
		if phase["name"] != "Mobilise" || !strings.Contains(fmt.Sprint(phase["date_label"]), "Jul") {
			t.Errorf("g: first phase = %v", phase)
		}
	}

	h := planFor(t, personaPlanCases()[2])
	if a := slideByKind(h, "architecture"); a == nil || listLen(a, "tiers") != 5 || listLen(a, "rails") != 2 {
		t.Errorf("h: architecture = %v, want five tiers and two rails", a)
	}
	if om := slideByKind(h, "option_matrix"); om == nil || listLen(om, "criteria") != 4 || listLen(om, "options") != 3 {
		t.Errorf("h: option_matrix = %v, want 4 criteria x 3 vendors", om)
	}

	i := planFor(t, personaPlanCases()[3])
	if p := slideByKind(i, "process"); p == nil || listLen(p, "steps") != 5 {
		t.Errorf("i: process steps = %v, want the five-step testing approach", p)
	}
}
