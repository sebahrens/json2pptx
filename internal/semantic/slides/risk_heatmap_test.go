package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

// The go-slide-creator-ec74l payload: six named risks, four with a "medium".
func riskHeatmapBody() map[string]any {
	return map[string]any{
		"title": "Cyber is the only high-likelihood, high-impact risk",
		"items": []any{
			map[string]any{"name": "Cyber", "likelihood": "high", "impact": "high"},
			map[string]any{"name": "Third-party outage", "likelihood": "medium", "impact": "high"},
			map[string]any{"name": "Model risk", "likelihood": "low", "impact": "high"},
			map[string]any{"name": "Conduct", "likelihood": 2, "impact": 2},
			map[string]any{"name": "Climate", "likelihood": "low", "impact": "medium"},
			map[string]any{"name": "Fraud", "likelihood": "medium", "impact": "low"},
		},
	}
}

func TestCompileRiskHeatmapPattern(t *testing.T) {
	body := riskHeatmapBody()
	slide, links, err := CompileRiskHeatmap(Input{Title: "Cyber is the only high-likelihood, high-impact risk", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "risk-heatmap" || slide.LayoutID != "blank-title" {
		t.Fatalf("slide = %+v, want the risk-heatmap pattern", slide)
	}
	var v riskHeatmapValues
	if err := json.Unmarshal(slide.Pattern.Values, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Items) != 6 {
		t.Fatalf("items = %+v, want all six risks", v.Items)
	}
	if v.Items[2] != (riskHeatmapItem{Name: "Model risk", Likelihood: "low", Impact: "high"}) {
		t.Errorf("risk = %+v", v.Items[2])
	}
	if v.Items[3].Likelihood != "2" || v.Items[3].Impact != "2" {
		t.Errorf("numeric levels = %+v", v.Items[3])
	}
	if len(links) < 2 {
		t.Errorf("links = %+v, want title and items", links)
	}

	body["size"] = 5
	if RiskHeatmapPattern(body) != "risk-heatmap" {
		t.Errorf("a 5 × 5 does not reach the pattern: %s", RiskHeatmapDegradeReason(body))
	}
}

// A level the grid does not have degrades with a reason, and the bullets keep
// every risk with both of its ratings.
func TestCompileRiskHeatmapFallback(t *testing.T) {
	body := riskHeatmapBody()
	body["items"].([]any)[0].(map[string]any)["impact"] = "catastrophic"
	reason := RiskHeatmapDegradeReason(body)
	if RiskHeatmapPattern(body) != "" || !strings.Contains(reason, "items[0].impact") || !strings.Contains(reason, "low, medium, high") {
		t.Fatalf("degrade reason = %q, want the field and the allowed levels", reason)
	}
	slide, _, err := CompileRiskHeatmap(Input{Title: "Top risks", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern != nil {
		t.Fatalf("pattern = %+v, want a bullet fallback", slide.Pattern)
	}
	var all []string
	for _, c := range slide.Content {
		if c.BulletsValue != nil {
			all = append(all, *c.BulletsValue...)
		}
	}
	joined := strings.Join(all, "\n")
	for _, want := range []string{"Cyber — likelihood high, impact catastrophic", "Conduct — likelihood 2, impact 2", "Fraud — likelihood medium, impact low"} {
		if !strings.Contains(joined, want) {
			t.Errorf("fallback bullets miss %q:\n%s", want, joined)
		}
	}

	body["size"] = 4
	if r := RiskHeatmapDegradeReason(body); !strings.Contains(r, "size") {
		t.Errorf("size 4 reason = %q", r)
	}
}
