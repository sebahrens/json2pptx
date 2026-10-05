package deckplan

import (
	"fmt"
	"strings"
	"testing"
)

// go-slide-creator-xbwlt: slides the brief names in the kind catalogue's own
// vocabulary are drafted as those kinds, with the matched facts in the kind's
// fields.

func namedKinds(brief string) []string {
	facts := extractBriefFacts(brief)
	var kinds []string
	for _, n := range detectNamedSlides(brief, facts) {
		kinds = append(kinds, n.kind)
	}
	return kinds
}

func TestNamedSlidesFollowTheKindVocabulary(t *testing.T) {
	for _, tc := range []struct {
		brief string
		want  string
	}{
		{"Board deck. Margin bridge 2023→2025: 2023 EBITDA 24.0, price +6.5, volume +3.2, raw material −4.1, opex +1.4 = 31.0.", "bridge"},
		{"Board deck. EBITDA walk FY25 to FY26: FY25 EBITDA 40, volume +5, price +3, costs −6 = 42.", "bridge"},
		{"Board deck. Top risks on a likelihood/impact view: cyber (high/high), fraud (medium/low), climate (low/medium).", "risk_heatmap"},
		{"Board deck. Initiatives on an impact/effort 2x2: automation (high/low), replatforming (high/high), reporting (low/low).", "matrix_2x2"},
		{"Board deck. A comparable: last year's DD on \"Project Keel\" found a 9-point price-gap risk that moved the bid by EUR 40M.", "image_case"},
		{"Board deck. The ops console screenshot at `$J/assets/ops.png` shows the nightly load dashboard; call out the SLA breach banner (top-left) and the job queue (centre).", "image_case"},
		{"Board deck. Target architecture, top to bottom: consumption (Power BI, ML feature store), serving (semantic layer), storage (Delta lake), with security and FinOps as cross-cutting.", "architecture"},
		{"Board deck. Team: Anna Lindqvist (partner, 18 years industrials), Marco Rossi (manager, 3 prior DDs), two consultants.", "team"},
		{"Board deck. Three scope options: (A) desktop-only model, EUR 180k, 3 weeks; (B) model plus interviews, EUR 320k, 4 weeks; (C) B plus a survey, EUR 410k, 5 weeks — we recommend B.", "decision"},
		{"Board deck. Three vendor options scored on cost, migration risk, skills availability, lock-in: Databricks, Snowflake, a native hyperscaler stack; recommend Databricks.", "option_matrix"},
		{"Board deck. Workplan (B): week 1 data room and market model, week 2 interviews wave 1, week 3 synthesis, week 4 IC pack.", "roadmap"},
		{"Board deck. Roadmap: Mobilise (Jul–Aug 2026), Design (Sep–Nov 2026), Build (Dec 2026–Mar 2027), Embed (Apr–Jun 2027); parallel tracks: data platform, training.", "roadmap"},
		{"Board deck. Remediation plan: Q4 2026 quick wins (recertification run), Q1–Q2 2027 access tooling, Q3 2027 re-test.", "roadmap"},
		{"Board deck. Testing approach: plan (scoping, risk assessment), walkthroughs, design evaluation, operating-effectiveness testing, reporting.", "process"},
		{"Board deck. Pricing schedule has eight line items you may invent sensibly (phase, deliverable, days, fee).", "table"},
		{"Board deck. Seven findings, each with a severity (2 high, 3 medium, 2 low), an owner and a due date between Dec 2026 and Jun 2027.", "table"},
		{"Board deck. Results by domain — access management: 14 controls, 3 exceptions, rating red; change management: 11 controls, 1 exception, amber; operations: 9 controls, 0 exceptions, green.", "table"},
		{"Board deck. KPIs after: load window 9.5 h → 1.5 h, SLA misses 61 → under 3 per 90 days, product-record defects 23% → under 5%.", "comparison"},
		{"Board deck. Control maturity by domain on a 1–5 scale: access 2, change 3, operations 4; target 4 everywhere by FY28.", "comparison"},
		{"Board deck. Risk appetite dashboard today: credit — within appetite (NPL 2.1% vs limit 3%), liquidity — within (LCR 148% vs 110%), operational — breached (losses EUR 11.2M vs EUR 8M).", "kpi_snapshot"},
		{"Board deck. Facts: scope was 5 domains, 48 key controls, 1,160 samples tested across 9 applications.", "kpi_snapshot"},
		{"Board deck. Current operational-risk losses: 2023 EUR 6.1M, 2024 EUR 8.4M, 2025 EUR 11.2M; peer median is about 0.02% of assets.", "chart_insight"},
		{"Board deck. Facts: the European fastener market grew from EUR 8.1bn (2021) to EUR 9.4bn (2025), forecast EUR 10.6bn by 2028; Nordbolt holds 2.3% share.", "chart_insight"},
		{"Board deck. Year-on-year: exceptions 2024: 11, 2025: 9, 2026: 7; high-rated findings 2024: 4, 2025: 3, 2026: 2.", "chart_insight"},
		{"Board deck. A cost driver view: run cost splits into compute 48%, storage 12%, licences 22%, people 18%.", "chart_insight"},
		{"Board deck. Target operating model has four pillars: governance and appetite, risk identification and assessment, control framework, reporting and data.", "pillars"},
		{"Board deck. Ask: approve option 2 and a steering committee chaired by the CRO.", "next_steps"},
	} {
		got := namedKinds(tc.brief)
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%q\n  named kinds = %v, want [%s]", tc.brief, got, tc.want)
		}
	}
}

func TestNamedSlideFields(t *testing.T) {
	fieldsOf := func(brief, kind string) map[string]any {
		t.Helper()
		for _, n := range detectNamedSlides(brief, extractBriefFacts(brief)) {
			if n.kind == kind {
				return n.fields
			}
		}
		t.Fatalf("no %s drafted from %q", kind, brief)
		return nil
	}
	list := func(m map[string]any, key string) []any {
		l, _ := m[key].([]any)
		return l
	}

	bridge := fieldsOf("Deck. Margin bridge 2023→2025: 2023 EBITDA 24.0, price +6.5, volume +3.2, raw material −4.1, opex +1.4 = 31.0.", "bridge")
	cols := list(bridge, "columns")
	if len(cols) != 6 {
		t.Fatalf("bridge columns = %v", cols)
	}
	want := []string{"2023 EBITDA total 24", "price delta 6.5", "volume delta 3.2", "raw material delta -4.1", "opex delta 1.4", "2025 EBITDA total 31"}
	for i, c := range cols {
		m := c.(map[string]any)
		if got := fmt.Sprintf("%v %v %v", m["label"], m["type"], m["value"]); got != want[i] {
			t.Errorf("column %d = %q, want %q", i, got, want[i])
		}
	}

	kpi := fieldsOf("Deck. Risk appetite dashboard today: credit — within appetite (NPL 2.1% vs limit 3%), liquidity — within (LCR 148% vs 110%), operational — breached (losses EUR 11.2M vs EUR 8M).", "kpi_snapshot")
	kpis := list(kpi, "kpis")
	if len(kpis) != 3 {
		t.Fatalf("kpis = %v", kpis)
	}
	if first := kpis[0].(map[string]any); first["label"] != "credit" || first["value"] != "within appetite" || first["comparator"] != "NPL 2.1% vs limit 3%" {
		t.Errorf("dashboard kpi = %v", first)
	}
	scope := fieldsOf("Deck. Facts: scope was 5 domains, 48 key controls, 1,160 samples tested across 9 applications.", "kpi_snapshot")
	if k := list(scope, "kpis"); len(k) != 3 || k[1].(map[string]any)["value"] != "48" || k[1].(map[string]any)["label"] != "key controls" {
		t.Errorf("scope kpis = %v", k)
	}

	dec := fieldsOf("Deck. Three scope options for the DD: (A) desktop-only market model, EUR 180k, 3 weeks; (B) market model plus 25 customer interviews, EUR 320k, 4 weeks; (C) B plus a channel-partner survey, EUR 410k, 5 weeks — we recommend B.", "decision")
	opts := list(dec, "options")
	if len(opts) != 3 {
		t.Fatalf("options = %v", opts)
	}
	if b := opts[1].(map[string]any); b["label"] != "B: market model plus 25 customer interviews" || b["detail"] != "EUR 320k, 4 weeks" || b["recommended"] != true {
		t.Errorf("option B = %v", b)
	}
	if !strings.Contains(fmt.Sprint(dec["recommendation"]), "recommend B") {
		t.Errorf("recommendation = %v", dec["recommendation"])
	}
	numbered := fieldsOf("Deck. Three delivery options: (1) advisory only, EUR 0.9M, we design and the bank implements; (2) co-delivery, EUR 2.1M, joint team; (3) full outsource of the programme office, EUR 3.4M. Recommend option 2.", "decision")
	if o := list(numbered, "options"); len(o) != 3 || o[1].(map[string]any)["recommended"] != true {
		t.Errorf("numbered options = %v (recommend option 2 must mark the second)", o)
	}

	om := fieldsOf("Deck. Three vendor options scored on cost, migration risk, skills availability, lock-in: Databricks, Snowflake, a native hyperscaler stack; recommend Databricks.", "option_matrix")
	if c := list(om, "criteria"); len(c) != 4 || c[3] != "lock-in" {
		t.Errorf("criteria = %v", c)
	}
	if o := list(om, "options"); len(o) != 3 || o[0].(map[string]any)["name"] != "Databricks" || om["recommended"] != "Databricks" {
		t.Errorf("options = %v recommended = %v", o, om["recommended"])
	}

	road := fieldsOf("Deck. Migration roadmap: Foundation (Q1–Q2 2027: landing zone, governance, 3 pilot sources), Migrate (Q3 2027–Q2 2028: 14 sources, 1,900 jobs refactored to 400 pipelines), Optimise (Q3–Q4 2028: decommission warehouse); parallel tracks: data literacy for 300 analysts, FinOps guardrails.", "roadmap")
	phases := list(road, "phases")
	if len(phases) != 3 {
		t.Fatalf("phases = %v", phases)
	}
	if p := phases[0].(map[string]any); p["name"] != "Foundation" || p["date_label"] != "Q1–Q2 2027" || p["description"] != "landing zone, governance, 3 pilot sources" {
		t.Errorf("phase = %v", p)
	}
	weeks := fieldsOf("Deck. Workplan (B): week 1 data room and market model, week 2 interviews wave 1, week 3 interviews wave 2 and synthesis, week 4 red-flag report and IC pack.", "roadmap")
	if p := list(weeks, "phases"); len(p) != 4 || p[0].(map[string]any)["date_label"] != "week 1" || p[0].(map[string]any)["name"] != "data room and market model" {
		t.Errorf("week phases = %v", p)
	}

	team := fieldsOf("Deck. Team: Anna Lindqvist (partner, 18 years industrials), Marco Rossi (manager, 3 prior fastener DDs), two consultants.", "team")
	members := list(team, "members")
	if len(members) != 4 {
		t.Fatalf("members = %v", members)
	}
	if m := members[0].(map[string]any); m["name"] != "Anna Lindqvist" || m["role"] != "partner" || m["bio"] != "18 years industrials" {
		t.Errorf("member = %v", m)
	}
	if m := members[3].(map[string]any); m["role"] != "consultant" || m["name"] != "__FILL__" {
		t.Errorf("consultant placeholder = %v", m)
	}

	// go-slide-creator-ec74l: rated risks draft the risk heat map, every risk
	// with its own likelihood and impact — a "medium" is not folded into high.
	heat := fieldsOf("Deck. Top risks on a likelihood/impact view: cyber (high/high), third-party outage (medium/high), conduct (medium/medium), model risk (low/high), climate (low/medium), fraud (medium/low).", "risk_heatmap")
	risks := list(heat, "items")
	if len(risks) != 6 {
		t.Fatalf("heat map places %d of 6 risks: %v", len(risks), risks)
	}
	if r := risks[1].(map[string]any); r["name"] != "third-party outage" || r["likelihood"] != "medium" || r["impact"] != "high" {
		t.Errorf("risk = %v, want third-party outage at medium likelihood / high impact", r)
	}
	if r := risks[5].(map[string]any); r["likelihood"] != "medium" || r["impact"] != "low" {
		t.Errorf("risk = %v, want fraud at medium / low", r)
	}
	swapped := fieldsOf("Deck. Top risks on an impact/likelihood view: cyber (high/medium), fraud (low/high).", "risk_heatmap")
	if r := list(swapped, "items")[0].(map[string]any); r["impact"] != "high" || r["likelihood"] != "medium" {
		t.Errorf("impact-first ratings = %v, want impact high / likelihood medium", r)
	}

	matrix := fieldsOf("Deck. Initiatives on an impact/effort 2x2: automation (high/low), replatforming (high/high), reporting (low/low).", "matrix_2x2")
	if matrix["x_axis"] != "Impact" || matrix["y_axis"] != "Effort" {
		t.Errorf("axes = %v / %v", matrix["x_axis"], matrix["y_axis"])
	}
	quads := list(matrix, "quadrants")
	if len(quads) != 4 {
		t.Fatalf("quadrants = %v", quads)
	}
	placed := 0
	for _, q := range quads {
		placed += strings.Count(fmt.Sprint(q.(map[string]any)["body"]), "(")
	}
	if placed != 3 {
		t.Errorf("matrix places %d of 3 initiatives: %v", placed, quads)
	}

	arch := fieldsOf("Deck. Target architecture, top to bottom: consumption (Power BI, 3 data products, ML feature store), serving (semantic layer, governed marts), processing (Spark batch, streaming), storage (Delta lake, bronze/silver/gold), ingestion (CDC from 14 sources, event streaming), with security/governance and FinOps as cross-cutting.", "architecture")
	if tiers := list(arch, "tiers"); len(tiers) != 5 || tiers[0].(map[string]any)["label"] != "consumption" || len(list(tiers[0].(map[string]any), "items")) != 3 {
		t.Errorf("tiers = %v", tiers)
	}
	if rails := list(arch, "rails"); len(rails) != 2 || rails[1] != "FinOps" {
		t.Errorf("rails = %v", rails)
	}

	proc := fieldsOf("Deck. Testing approach: plan (scoping, risk assessment), walkthroughs, design evaluation, operating-effectiveness testing, reporting.", "process")
	if steps := list(proc, "steps"); len(steps) != 5 || steps[0].(map[string]any)["label"] != "plan" || steps[0].(map[string]any)["description"] != "scoping, risk assessment" {
		t.Errorf("steps = %v", steps)
	}

	table := fieldsOf("Deck. Pricing schedule has eight line items you may invent sensibly (phase, deliverable, days, fee).", "table")
	if h := list(table, "headers"); len(h) != 4 || h[0] != "phase" {
		t.Errorf("headers = %v", h)
	}
	if rows := list(table, "rows"); len(rows) != 8 {
		t.Errorf("rows = %d, want eight __FILL__ line items", len(rows))
	}
	register := fieldsOf("Deck. Seven findings (invent titles consistent with the domains), each with a severity (2 high, 3 medium, 2 low), an owner and a due date between Dec 2026 and Jun 2027; the two high findings are \"privileged access not recertified for 3 of 9 applications\" and \"cloud provider SOC 2 reports not reviewed for 2 of 4 providers\".", "table")
	if h := list(register, "headers"); len(h) != 4 || h[1] != "severity" || h[3] != "due date" {
		t.Errorf("register headers = %v", h)
	}
	if rows := list(register, "rows"); len(rows) != 7 || !strings.Contains(fmt.Sprint(rows[0]), "privileged access") {
		t.Errorf("register rows = %v", rows)
	}
	results := fieldsOf("Deck. Results by domain — access management: 14 controls, 3 exceptions, rating red; change management: 11 controls, 1 exception, amber; computer operations: 9 controls, 0 exceptions, green.", "table")
	if rows := list(results, "rows"); len(rows) != 3 || fmt.Sprint(rows[0]) != "[access management 14 3 red]" {
		t.Errorf("results rows = %v", rows)
	}
	if h := list(results, "headers"); len(h) != 4 || h[1] != "Controls" || h[3] != "Rating" {
		t.Errorf("results headers = %v", h)
	}

	cmp := fieldsOf("Deck. KPIs after: load window 9.5 h → 1.5 h, SLA misses 61 → under 3 per 90 days, product-record defects 23% → under 5%, reconciliation effort −70%.", "comparison")
	cols2 := list(cmp, "columns")
	if len(cols2) != 2 {
		t.Fatalf("comparison columns = %v", cols2)
	}
	if items := list(cols2[0].(map[string]any), "items"); len(items) != 4 || items[0] != "load window 9.5 h" {
		t.Errorf("today column = %v", items)
	}
	if items := list(cols2[1].(map[string]any), "items"); len(items) != 4 || items[1] != "under 3 per 90 days" {
		t.Errorf("target column = %v", items)
	}

	chart := fieldsOf("Deck. Current operational-risk losses: 2023 EUR 6.1M, 2024 EUR 8.4M, 2025 EUR 11.2M; peer median is about 0.02% of assets.", "chart_insight")
	data, _ := chart["chart"].(map[string]any)["data"].(map[string]any)
	if cats := list(data, "categories"); len(cats) != 3 || cats[0] != "2023" {
		t.Errorf("categories = %v", cats)
	}
	if series := list(data, "series"); len(series) != 1 || fmt.Sprint(list(series[0].(map[string]any), "values")) != "[6.1 8.4 11.2]" {
		t.Errorf("series = %v", series)
	}
	yoy := fieldsOf("Deck. Year-on-year: exceptions 2024: 11, 2025: 9, 2026: 7; high-rated findings 2024: 4, 2025: 3, 2026: 2.", "chart_insight")
	data, _ = yoy["chart"].(map[string]any)["data"].(map[string]any)
	if series := list(data, "series"); len(series) != 2 || series[1].(map[string]any)["name"] != "high-rated findings" {
		t.Errorf("yoy series = %v", series)
	}
	pie := fieldsOf("Deck. A cost driver view: run cost splits into compute 48%, storage 12%, licences 22%, people 18%.", "chart_insight")
	if pie["chart"].(map[string]any)["type"] != "pie" {
		t.Errorf("percent split chart = %v", pie["chart"])
	}
	run := fieldsOf("Deck. Five-year TCO: current EUR 18.4M, target EUR 12.9M (saving EUR 5.5M); year-by-year run cost current 3.6/3.7/3.7/3.7/3.7 vs target 4.8/3.1/1.9/1.6/1.5 (year 1 includes migration).", "chart_insight")
	data, _ = run["chart"].(map[string]any)["data"].(map[string]any)
	if series := list(data, "series"); len(series) != 2 || len(list(data, "categories")) != 5 {
		t.Errorf("run cost series = %v categories = %v", series, list(data, "categories"))
	}

	img := fieldsOf("Deck. The ops console screenshot at `$J/assets/screenshot-ops-console.png` shows the nightly load dashboard; call out the SLA breach banner (roughly top-left) and the job queue (roughly centre).", "image_case")
	if image, _ := img["image"].(map[string]any); image["path"] != "$J/assets/screenshot-ops-console.png" {
		t.Errorf("image = %v", img["image"])
	}
	if callouts := list(img, "callouts"); len(callouts) != 2 || callouts[0].(map[string]any)["label"] != "SLA breach banner" {
		t.Errorf("callouts = %v", callouts)
	}
}

func TestDeckNameFromTopic(t *testing.T) {
	for _, tc := range []struct{ topic, want string }{
		{"Build the proposal deck for \"Project Falcon\"", "Project Falcon"},
		{"Proposal for Harbour Bank (mid-sized retail bank, EUR 42bn assets, 3,800 staff)", "Harbour Bank"},
		{"Steering-committee business case for Atlas Retail (EUR 3.1bn revenue, 640 stores)", "Atlas Retail"},
		{"Build the audit-committee readout for Cobalt Insurance", "Cobalt Insurance"},
		{"Investor pitch deck for a fictional climate-tech startup", ""},
		{"Board update on SMB churn", ""},
		{"Strategy recommendation for the board", ""},
	} {
		if got := deckName(tc.topic); got != tc.want {
			t.Errorf("deckName(%q) = %q, want %q", tc.topic, got, tc.want)
		}
	}
}

func TestDeckSpecTitleIsNeverAnEllipsisedFragment(t *testing.T) {
	long := "Quarterly business review of the consolidated European industrial fasteners distribution portfolio for the investment committee and the operating partners. Revenue grew 18% YoY to $42M."
	p := BuildDeckSpecPlan(Params{Brief: long, SlideBudget: 6})
	if got, _ := p.DeckSpec.Meta["title"].(string); strings.Contains(got, "...") || strings.Contains(got, "…") {
		t.Errorf("meta.title = %q is a truncated fragment", got)
	}
	if got := p.DeckSpec.Meta["title"]; got != "__FILL__" {
		t.Errorf("meta.title = %v, want __FILL__ for a topic too long to be a title", got)
	}
	short := BuildDeckSpecPlan(Params{Brief: "Board update on SMB churn. Churn rose to 4.2%.", SlideBudget: 6})
	if got := short.DeckSpec.Meta["title"]; got != "Board update on SMB churn" {
		t.Errorf("meta.title = %v, want the topic when it fits", got)
	}
}

func TestTemplateOnConstraint(t *testing.T) {
	cleaned, cs := parseConstraints("Proposal. 10–12 slides on `p-style` when `$REPO/templates/p-style.pptx` exists, otherwise `blue-corporate`. Facts: revenue EUR 212M.")
	if got := constraintValue(cs, ConstraintTemplate); got != "p-style" {
		t.Errorf("template constraint = %q from %+v", got, cs)
	}
	if strings.Contains(cleaned, "p-style") || strings.Contains(cleaned, "blue-corporate") {
		t.Errorf("template instruction left in the brief: %q", cleaned)
	}
	cleaned, cs = parseConstraints("Proposal. 10–12 slides on `midnight-blue`. Facts: revenue EUR 212M.")
	if got := constraintValue(cs, ConstraintTemplate); got != "midnight-blue" || strings.Contains(cleaned, "midnight") {
		t.Errorf("template constraint = %q, cleaned %q", got, cleaned)
	}
}
