package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNativeDiagramPreflightBMCStressAndClean(t *testing.T) {
	longItems := make([]any, 9)
	for i := range longItems {
		longItems[i] = strings.Repeat("Long architecture capability ", 3)
	}
	stress := &types.DiagramSpec{Type: "business_model_canvas", Data: map[string]any{
		"key_partners": longItems, "key_activities": longItems, "key_resources": longItems,
		"value_propositions": longItems, "customer_relations": longItems, "channels": longItems,
		"customer_segments": longItems, "cost_structure": longItems, "revenue_streams": longItems,
	}}
	got := NativeDiagramPreflight(stress, "Arial", "slides[0].content[0].diagram_value")
	if len(got) == 0 || got[0].Code != "diagram.text_overlap" || !strings.Contains(got[0].Message, "Key Partners") {
		t.Fatalf("stress findings = %+v", got)
	}
	clean := &types.DiagramSpec{Type: "business_model_canvas", Data: map[string]any{
		"key_partners": []any{"Cloud partner", "Integrator"}, "value_propositions": []any{"Faster deployment"},
	}}
	if got := NativeDiagramPreflight(clean, "Arial", "p"); len(got) != 0 {
		t.Fatalf("clean BMC produced findings: %+v", got)
	}
}

func TestNativeDiagramPreflightPanelAndHeatmap(t *testing.T) {
	panels := make([]any, 6)
	for i := range panels {
		panels[i] = map[string]any{"title": "Panel", "body": strings.Repeat("dense content ", 80)}
	}
	if got := NativeDiagramPreflight(&types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"panels": panels}}, "Arial", "p"); len(got) == 0 {
		t.Fatal("dense panels must report text overlap")
	}
	labels := make([]any, 14)
	values := make([]any, 14)
	for i := range labels {
		labels[i] = strings.Repeat("Long header ", 4)
		row := make([]any, 14)
		for j := range row {
			row[j] = float64(i + j)
		}
		values[i] = row
	}
	heatmap := &types.DiagramSpec{Type: "heatmap", Data: map[string]any{"row_labels": labels, "column_labels": labels, "values": values}}
	if got := NativeDiagramPreflight(heatmap, "Arial", "p"); len(got) == 0 {
		t.Fatal("dense heatmap must report shortened labels")
	}
}

func TestNativeDiagramDensityBudgets(t *testing.T) {
	for _, tc := range []struct {
		typeName string
		clean    int
		stress   int
	}{
		{"house_diagram", 207, 1889}, {"icon_rows", 178, 1241},
		{"nine_box_talent", 138, 691}, {"pestel", 202, 2223},
		{"porters_five_forces", 246, 1301}, {"process_flow", 86, 729},
		{"pyramid", 182, 1292},
	} {
		t.Run(tc.typeName, func(t *testing.T) {
			clean := &types.DiagramSpec{Type: tc.typeName, Data: map[string]any{"label": strings.Repeat("c", tc.clean)}}
			if got := NativeDiagramPreflight(clean, "Arial", "p"); len(got) != 0 {
				t.Fatalf("clean payload produced findings: %+v", got)
			}
			stress := &types.DiagramSpec{Type: tc.typeName, Data: map[string]any{"label": strings.Repeat("s", tc.stress)}}
			if got := NativeDiagramPreflight(stress, "Arial", "p"); len(got) != 1 || got[0].Code != "diagram.text_overlap" {
				t.Fatalf("stress payload findings: %+v", got)
			}
		})
	}
}

func TestNativeProcessFlowVerticalPreflightUsesResolvedBoxes(t *testing.T) {
	data := map[string]any{
		"direction": "vertical",
		"steps": []any{
			map[string]any{"id": "start", "label": "Start", "type": "start"},
			map[string]any{"id": "check", "label": "Approved?", "type": "decision"},
			map[string]any{"id": "yes", "label": "Release"},
			map[string]any{"id": "no", "label": "Revise"},
			map[string]any{"id": "end", "label": "Complete", "type": "end"},
		},
		"connections": []any{
			map[string]any{"from": "start", "to": "check"},
			map[string]any{"from": "check", "to": "yes", "label": "Yes"},
			map[string]any{"from": "check", "to": "no", "label": "No"},
			map[string]any{"from": "yes", "to": "end"},
			map[string]any{"from": "no", "to": "end"},
		},
	}
	clean := &types.DiagramSpec{Type: "process_flow", Data: data}
	if got := NativeDiagramPreflight(clean, "Arial", "slides[0].content[0]"); len(got) != 0 {
		t.Fatalf("standard vertical flow findings = %+v, want none", got)
	}

	shortFrame := &types.DiagramSpec{Type: "process_flow", Data: data, Height: 120}
	got := NativeDiagramPreflight(shortFrame, "Arial", "slides[0].content[0]")
	if len(got) == 0 {
		t.Fatal("short vertical frame must report text/label overlap")
	}
	for _, finding := range got {
		if finding.Code == "diagram.text_overlap" {
			return
		}
	}
	t.Fatalf("short vertical frame findings = %+v, want diagram.text_overlap", got)
}

func TestNativeStatCardTinyCaption(t *testing.T) {
	stress := &types.DiagramSpec{Type: "stat_cards", Data: map[string]any{"panels": []any{
		map[string]any{"title": "Rechtsschutzversicherungsgesellschaften", "value": "1,234,567,890.12"},
		map[string]any{"title": "Enterprise Platform Subscriptions", "value": "-0.0003%"},
		map[string]any{"title": "Mid-market Professional Services", "value": "0"},
		map[string]any{"title": "Donaudampfschifffahrtsgesellschaft", "value": "EUR 12.5bn"},
	}}}
	got := NativeDiagramPreflight(stress, "Arial", "p")
	if len(got) == 0 || got[0].Code != "diagram.text_below_readable_min" {
		t.Fatalf("stress findings = %+v", got)
	}
	clean := &types.DiagramSpec{Type: "stat_cards", Data: map[string]any{"panels": []any{
		map[string]any{"title": "ARR", "value": "EUR 184m"}, map[string]any{"title": "NRR", "value": "118%"},
	}}}
	if got := NativeDiagramPreflight(clean, "Arial", "p"); len(got) != 0 {
		t.Fatalf("clean stat cards produced findings: %+v", got)
	}
}
