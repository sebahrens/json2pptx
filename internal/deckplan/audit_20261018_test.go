package deckplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const logisticsAuditBrief = "Steering committee update for Northwind Logistics FY26. Revenue GBP 412.6m up 6.8%. On-time delivery fell to 93.2% against a 95% floor: North 95.1, 94.2, 91.8, 91.0 by quarter; South 96.3, 96.0, 95.7, 95.4. Driver attrition 18.7%, North 24%. EBITDA margin up 2.1 points from fuel hedging (+4.8m) and Leeds hub consolidation (+3.1m), volume +6.2m, driver cost -2.9m, FY25 EBITDA 35.9m, FY26 47.1m. Risks: attrition stays above 20% (high/high), Leeds second shift slips (medium/high), fuel hedge rolls off (medium/medium). Ask: approve GBP 1.8m for a retention bonus (COO, 31 Oct), dynamic routing pilot (CIO, 30 Nov), Leeds second shift (hub director, 31 Jan). Slides: title; executive summary; EBITDA bridge; on-time delivery chart on the left with the attrition number on the right; risk heatmap; next steps."

func TestLogisticsBriefContractsAndFactAccounting(t *testing.T) {
	p := BuildDeckSpecPlan(Params{Brief: logisticsAuditBrief, SlideBudget: 6})
	if p.DeckSpec.Meta["title"] != "Northwind Logistics FY26" {
		t.Errorf("title lost period: %s", p.DeckSpec.Meta["title"])
	}
	byKind := map[string]map[string]any{}
	for _, slot := range p.Slots {
		byKind[slot.Kind] = draftSlideAt(t, p, slot.Path)
	}
	bridge := byKind["bridge"]["columns"].([]any)
	wantValues := []float64{35.9, 4.8, 3.1, 6.2, -2.9, 47.1}
	if len(bridge) != len(wantValues) {
		t.Fatalf("bridge: %v", bridge)
	}
	for i, raw := range bridge {
		col := raw.(map[string]any)
		wantType := "delta"
		if i == 0 || i == 5 {
			wantType = "total"
		}
		if col["value"] != wantValues[i] || col["type"] != wantType {
			t.Errorf("bridge[%d]: %v", i, col)
		}
	}
	regions := byKind["regions"]["regions"].([]any)
	chart := regions[0].(map[string]any)["chart"].(map[string]any)["data"].(map[string]any)
	if !reflect.DeepEqual(chart["categories"], []any{"Q1", "Q2", "Q3", "Q4"}) {
		t.Errorf("categories: %v", chart["categories"])
	}
	encoded, _ := json.Marshal(chart)
	for _, want := range []string{`"North"`, `"South"`, `95.1,94.2,91.8,91`, `96.3,96,95.7,95.4`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("series lost %s: %s", want, encoded)
		}
	}
	stat := regions[1].(map[string]any)
	if stat["kind"] != "stat" || stat["value"] != "18.7%" || stat["label"] != "Driver attrition" {
		t.Errorf("stat: %v", stat)
	}
	risks := byKind["risk_heatmap"]["items"].([]any)
	if len(risks) != 3 || risks[1].(map[string]any)["likelihood"] != "medium" || risks[1].(map[string]any)["impact"] != "high" {
		t.Errorf("rated risks: %v", risks)
	}
	actions := byKind["next_steps"]["actions"].([]any)
	if len(actions) != 3 {
		t.Fatalf("actions: %v", actions)
	}
	for i, want := range [][2]string{{"COO", "31 Oct"}, {"CIO", "30 Nov"}, {"hub director", "31 Jan"}} {
		got := actions[i].(map[string]any)
		if got["owner"] != want[0] || got["date"] != want[1] {
			t.Errorf("action[%d]: %v", i, got)
		}
	}
	for _, fact := range extractBriefFacts(logisticsAuditBrief[:strings.Index(logisticsAuditBrief, " Slides:")]) {
		if !planAccountsFor(p, fact.text) {
			t.Errorf("fact disappeared from slots and unplaced_facts: %q", fact.text)
		}
	}
}

func TestReportingPeriodsAreNotKPIValues(t *testing.T) {
	for _, tc := range []struct{ input, value, label string }{
		{"FY25 EBITDA 35.9m", "35.9m", "FY25 EBITDA"}, {"FY26 47.1m", "47.1m", "FY26"},
		{"Q1 FY2026 revenue 12.4m", "12.4m", "Q1 FY2026 revenue"}, {"H2 FY26 margin 18.7%", "18.7%", "H2 FY26 margin"},
		{"2026 revenue 42m", "42m", "2026 revenue"}, {"Employees 2026", "2026", "Employees"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := parseKPI(tc.input)
			if got["value"] != tc.value || got["label"] != tc.label {
				t.Errorf("got %v; want %s / %s", got, tc.label, tc.value)
			}
		})
	}
}

func TestBriefRiskAndBridgeBoundaries(t *testing.T) {
	n := readSentence("Risks: Supplier delay — likelihood high, impact medium; Fuel cost — impact high, probability low")
	fields := riskHeatmapFields(n)
	if fields == nil || len(fields["items"].([]any)) != 2 {
		t.Fatalf("axis-word risks: %v", fields)
	}
	for _, text := range []string{"Metrics: FY25 35.9m, FY26 47.1m, margin +2.1 points", "FY25 EBITDA 35.9m, price +4.8m, volume +3.1k, FY26 47.1m"} {
		if got := fiscalBridgeFields(readSentence(text)); got != nil {
			t.Errorf("invented an additive same-unit bridge for %q: %v", text, got)
		}
	}
	for _, text := range []string{"North 95.1,94.2,91.8,91.0; South 96.3,96.0,95.7,95.4", "Quarterly delivery: North 95.1, 94.2, 91.8; South 96.3, 96.0"} {
		if got := labelledQuarterSeries(readSentence(text)); got != nil {
			t.Errorf("invented quarter alignment for %q: %+v", text, got)
		}
	}
}
