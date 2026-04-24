package svggen

import (
	"testing"
)

// TestPieChart_ZeroSumFinding verifies that a pie chart with all-zero values
// emits a chart.zero_sum_pie finding and still produces valid SVG output.
func TestPieChart_ZeroSumFinding(t *testing.T) {
	req := &RequestEnvelope{
		Type: "pie_chart",
		Data: map[string]any{
			"labels": []any{"A", "B", "C"},
			"values": []any{0.0, 0.0, 0.0},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingZeroSumPie)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingZeroSumPie, output.Findings)
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindReplaceValue {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindReplaceValue)
	}
}

// TestBarChart_AllZeroSeriesFinding verifies that a bar chart with all-zero
// values emits a chart.all_zero_series finding.
func TestBarChart_AllZeroSeriesFinding(t *testing.T) {
	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"categories": []any{"Q1", "Q2", "Q3"},
			"series": []any{
				map[string]any{
					"name":   "Revenue",
					"values": []any{0.0, 0.0, 0.0},
				},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingAllZeroSeries)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingAllZeroSeries, output.Findings)
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
}

// TestBarChart_NegativeOnLogFinding verifies that a bar chart with zero
// values on an auto-log scale emits a chart.negative_on_log finding.
// Auto-log triggers when all-positive data spans 3+ orders of magnitude;
// zero values are then clamped to the baseline.
func TestBarChart_NegativeOnLogFinding(t *testing.T) {
	// Data spans 0.001–10000 (7 orders), triggering auto-log-scale.
	// The zero value at index 2 will be clamped to the log baseline.
	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"categories": []any{"A", "B", "C", "D"},
			"series": []any{
				map[string]any{
					"name":   "Wide Range",
					"values": []any{0.001, 10000.0, 0.0, 500.0},
				},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingNegativeOnLog)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingNegativeOnLog, output.Findings)
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindExplicitScale {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindExplicitScale)
	}
}

// TestLineChart_InvalidTimeFormatFinding verifies that a line chart with
// unparseable time_strings emits a chart.invalid_time_format finding.
func TestLineChart_InvalidTimeFormatFinding(t *testing.T) {
	req := &RequestEnvelope{
		Type: "line_chart",
		Data: map[string]any{
			"series": []any{
				map[string]any{
					"name":         "Bad Times",
					"time_strings": []any{"not-a-date", "also-bad", "nope"},
					"values":       []any{10.0, 20.0, 30.0},
				},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingInvalidTimeFormat)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingInvalidTimeFormat, output.Findings)
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindReplaceValue {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindReplaceValue)
	}
}

// TestPieChart_NonZeroSum_NoFinding verifies that a pie chart with valid
// positive values does NOT emit a zero_sum_pie finding.
func TestPieChart_NonZeroSum_NoFinding(t *testing.T) {
	req := &RequestEnvelope{
		Type: "pie_chart",
		Data: map[string]any{
			"labels": []any{"A", "B", "C"},
			"values": []any{10.0, 20.0, 30.0},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}

	found := findFindingByCode(output.Findings, FindingZeroSumPie)
	if found != nil {
		t.Errorf("did not expect %q finding for valid pie chart, got: %v", FindingZeroSumPie, found)
	}
}

// TestBarChart_AutoLogScaleFinding verifies that a bar chart spanning 3+
// orders of magnitude emits a chart.auto_log_scale_applied finding.
func TestBarChart_AutoLogScaleFinding(t *testing.T) {
	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"categories": []any{"A", "B", "C", "D"},
			"series": []any{
				map[string]any{
					"name":   "Wide Range",
					"values": []any{0.001, 10000.0, 5.0, 500.0},
				},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingAutoLogScaleApplied)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingAutoLogScaleApplied, output.Findings)
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindExplicitScale {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindExplicitScale)
	}
}

// TestBarChart_NoAutoLogScale_NoFinding verifies that a bar chart with
// a narrow value range does NOT emit auto_log_scale_applied.
func TestBarChart_NoAutoLogScale_NoFinding(t *testing.T) {
	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"categories": []any{"A", "B", "C"},
			"series": []any{
				map[string]any{
					"name":   "Narrow Range",
					"values": []any{10.0, 50.0, 30.0},
				},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}

	found := findFindingByCode(output.Findings, FindingAutoLogScaleApplied)
	if found != nil {
		t.Errorf("did not expect %q finding for narrow-range bar chart, got: %v", FindingAutoLogScaleApplied, found)
	}
}

// TestBarChart_TickThinnedXAxis verifies that a bar chart with many categories
// emits a chart.tick_thinned finding for x-axis label thinning.
func TestBarChart_TickThinnedXAxis(t *testing.T) {
	// Create 40 categories to trigger AdaptXLabels thinning.
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
				map[string]any{
					"name":   "Dense",
					"values": vals,
				},
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingTickThinned)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingTickThinned, output.Findings)
	}
	if found.Severity != "info" {
		t.Errorf("severity = %q, want %q", found.Severity, "info")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindReduceItems {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindReduceItems)
	}
}

// findFindingByCode returns the first Finding with the given code, or nil.
func findFindingByCode(findings []Finding, code string) *Finding {
	for i := range findings {
		if findings[i].Code == code {
			return &findings[i]
		}
	}
	return nil
}
