package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// TestRepairGateDefaultsAreTheShipGate is the go-slide-creator-ie9v acceptance
// test: auto_repair's default gate and score_deck's ship gate are the same
// numbers. They were 75/0/2 and 80/0/0, and both tools return a field called
// gate_passed — so a deck could converge in the repair loop at 79 and be refused
// by the ship check at 80, and an agent that stopped at auto_repair shipped a
// deck the project's own gate rejects.
func TestRepairGateDefaultsAreTheShipGate(t *testing.T) {
	ship := deterministic.DefaultQualityGateCriteria()
	cases := []struct {
		name          string
		repair, shipV int
	}{
		{"min_score", defaultAutoRepairMinScore, ship.MinScore},
		{"max_p0_findings", defaultAutoRepairMaxP0Findings, ship.MaxP0Findings},
		{"max_p1_findings", defaultAutoRepairMaxP1Findings, ship.MaxP1Findings},
	}
	for _, tc := range cases {
		if tc.repair != tc.shipV {
			t.Errorf("auto_repair default %s = %d, score_deck ship gate = %d — one definition of done", tc.name, tc.repair, tc.shipV)
		}
	}
	if !defaultAutoRepairRequireTakeawayOnCharts || !ship.RequireTakeawayOnCharts {
		t.Error("both gates should require a takeaway on charts")
	}
}

// TestRepairGateAgreesWithScoreDeck runs both tools over one deck and asserts
// they reach the same verdict on the same score. It is the behavioural half of
// the contract: the constants could drift apart through the request parser.
func TestRepairGateAgreesWithScoreDeck(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	ctx := context.Background()
	deck := mustParseJSON(autoRepairDeck(3))

	scoreRes, err := mc.handleScoreDeck(ctx, makeRequest(map[string]any{"presentation": deck}))
	if err != nil || scoreRes.IsError {
		t.Fatalf("score_deck: %v %s", err, resultText(scoreRes))
	}
	var score struct {
		OverallScore int `json:"overall_score"`
		QualityGate  struct {
			Passed   bool `json:"passed"`
			Criteria struct {
				MinScore      int `json:"min_score"`
				MaxP0Findings int `json:"max_p0_findings"`
				MaxP1Findings int `json:"max_p1_findings"`
			} `json:"criteria"`
		} `json:"quality_gate"`
	}
	structuredInto(t, scoreRes.StructuredContent, &score)

	// One pass, so auto_repair reports its verdict on the SAME deck rather than
	// on a repaired one.
	repairRes, err := mc.handleAutoRepair(ctx, makeRequest(map[string]any{
		"presentation":    deck,
		"max_passes":      float64(1),
		"output_filename": "gate_agreement.pptx",
	}))
	if err != nil || repairRes.IsError {
		t.Fatalf("auto_repair: %v %s", err, resultText(repairRes))
	}
	raw, err := json.Marshal(repairRes.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var repair struct {
		FinalScore int  `json:"final_score"`
		GatePassed bool `json:"gate_passed"`
		Gate       struct {
			MinScore      int `json:"min_score"`
			MaxP0Findings int `json:"max_p0_findings"`
			MaxP1Findings int `json:"max_p1_findings"`
		} `json:"gate"`
	}
	if err := json.Unmarshal(raw, &repair); err != nil {
		t.Fatalf("decode auto_repair response: %v", err)
	}

	if repair.Gate.MinScore != score.QualityGate.Criteria.MinScore ||
		repair.Gate.MaxP0Findings != score.QualityGate.Criteria.MaxP0Findings ||
		repair.Gate.MaxP1Findings != score.QualityGate.Criteria.MaxP1Findings {
		t.Errorf("gates differ: auto_repair %+v vs score_deck %+v", repair.Gate, score.QualityGate.Criteria)
	}
	if repair.FinalScore == score.OverallScore && repair.GatePassed != score.QualityGate.Passed {
		t.Errorf("same deck, same score %d, but auto_repair says gate_passed=%v and score_deck says passed=%v",
			repair.FinalScore, repair.GatePassed, score.QualityGate.Passed)
	}
}
