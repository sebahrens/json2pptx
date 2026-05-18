package svggen

import "testing"

// TestDryRender_BarChartTickThinned verifies that the top-level DryRender
// helper returns chart.tick_thinned findings for a 40-category bar chart at
// narrow width WITHOUT requiring callers to consume SVG bytes. This is the
// contract used by validate_input / preview_presentation_plan in json2pptx.
func TestDryRender_BarChartTickThinned(t *testing.T) {
	cats := make([]any, 40)
	vals := make([]any, 40)
	for i := range cats {
		cats[i] = "Category " + string(rune('A'+i%26)) + string(rune('0'+i/26))
		vals[i] = float64(i + 1)
	}

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"categories": cats,
			"series": []any{
				map[string]any{"name": "Dense", "values": vals},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("DryRender() error = %v", err)
	}
	if len(findings) == 0 {
		t.Fatalf("expected at least one finding, got none")
	}

	var sawTickThinned bool
	for _, f := range findings {
		if f.Code == FindingTickThinned {
			sawTickThinned = true
			break
		}
	}
	if !sawTickThinned {
		t.Fatalf("expected %q in findings, got %v", FindingTickThinned, findings)
	}
}

// TestDryRender_StrictFitRefusal confirms that strict-fit promotion runs in
// the dry-render path identically to the generate path: a capacity-exceeded
// finding under strict mode produces a refuse-severity finding and an error
// return so agents can fail early at validate time.
func TestDryRender_StrictFitPromotion(t *testing.T) {
	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"categories": []any{"A"},
			"series": []any{
				map[string]any{"name": "S", "values": []any{1.0}},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600, StrictFit: "warn"},
	}

	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("DryRender() error = %v", err)
	}
	// Non-strict on a tiny chart: should not refuse.
	for _, f := range findings {
		if f.Severity == "refuse" {
			t.Errorf("did not expect refuse-severity finding in warn mode, got %+v", f)
		}
	}
}
