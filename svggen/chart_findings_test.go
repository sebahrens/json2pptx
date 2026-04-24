package svggen

import (
	"testing"

	"github.com/sebahrens/json2pptx/svggen/core"
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

// TestTruncateText_EmitsEllipsizedFinding verifies that TruncateText emits
// a chart.label_ellipsized finding when text is truncated with ellipsis.
func TestTruncateText_EmitsEllipsizedFinding(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	text := "This is a very long label that will need to be truncated"

	result, _ := b.TruncateText(text, 40, TextOverflowEllipsis)
	if result == text {
		t.Skip("text fits without truncation at this width")
	}

	found := findFindingByCode(b.Findings(), FindingLabelEllipsized)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingLabelEllipsized, b.Findings())
	}
	if found.Severity != "info" {
		t.Errorf("severity = %q, want %q", found.Severity, "info")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindTruncateOrSplit {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindTruncateOrSplit)
	}
}

// TestTruncateText_EmitsClippedFinding verifies that TruncateText emits
// a chart.label_clipped finding when text is hard-clipped.
func TestTruncateText_EmitsClippedFinding(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	text := "This is a very long label that will need to be clipped"

	result, _ := b.TruncateText(text, 40, TextOverflowClip)
	if result == text {
		t.Skip("text fits without truncation at this width")
	}

	found := findFindingByCode(b.Findings(), FindingLabelClipped)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingLabelClipped, b.Findings())
	}
	if found.Severity != "info" {
		t.Errorf("severity = %q, want %q", found.Severity, "info")
	}
}

// TestTruncateText_NoFindingWhenFits verifies that TruncateText does NOT emit
// a finding when text fits without truncation.
func TestTruncateText_NoFindingWhenFits(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	text := "Hi"

	b.TruncateText(text, 500, TextOverflowEllipsis)

	if len(b.Findings()) > 0 {
		t.Errorf("expected no findings for text that fits, got: %v", b.Findings())
	}
}

// TestLabelFit_EmitsTruncatedFinding verifies that LabelFitStrategy.Fit
// emits a chart.label_truncated finding when text is truncated as last resort.
func TestLabelFit_EmitsTruncatedFinding(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	strategy := LabelFitStrategy{
		PreferredSize: 12,
		MinSize:       6,
		AllowWrap:     false,
	}

	// Very narrow width forces truncation
	result := strategy.Fit(b, "This is a label that definitely does not fit in tiny space", 30, 0)
	if result.DisplayText == "This is a label that definitely does not fit in tiny space" {
		t.Skip("text fits without truncation")
	}

	found := findFindingByCode(b.Findings(), FindingLabelTruncated)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingLabelTruncated, b.Findings())
	}
	if found.Severity != "info" {
		t.Errorf("severity = %q, want %q", found.Severity, "info")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindTruncateOrSplit {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindTruncateOrSplit)
	}
}

// TestLegendGridOverflow_EmitsFinding verifies that when legend items are
// dropped due to insufficient height, a chart.legend_overflow_dropped finding
// is emitted.
func TestLegendGridOverflow_EmitsFinding(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	style := b.StyleGuide()
	config := PresentationPieLegendConfig(style)

	labels := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	legend := NewLegend(b, config)
	for _, label := range labels {
		legend.AddItem(label, MustParseColor("#4E79A7"))
	}

	// Very small height forces overflow
	bounds := Rect{X: 10, Y: 200, W: 380, H: 15}
	legend.Draw(bounds)

	found := findFindingByCode(b.Findings(), FindingLegendOverflowDropped)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingLegendOverflowDropped, b.Findings())
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindReduceItems {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindReduceItems)
	}
}

// TestLegendVerticalOverflow_EmitsFinding verifies that vertical legend layout
// emits a chart.legend_overflow_dropped finding when items are dropped.
func TestLegendVerticalOverflow_EmitsFinding(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()
	config.Layout = LegendLayoutVertical

	labels := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}
	legend := NewLegend(b, config)
	for _, label := range labels {
		legend.AddItem(label, MustParseColor("#F28E2B"))
	}

	// Very small height forces overflow
	bounds := Rect{X: 10, Y: 200, W: 380, H: 20}
	legend.Draw(bounds)

	found := findFindingByCode(b.Findings(), FindingLegendOverflowDropped)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingLegendOverflowDropped, b.Findings())
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
}

// TestLegendNoOverflow_NoFinding verifies that when all legend items fit,
// no chart.legend_overflow_dropped finding is emitted.
func TestLegendNoOverflow_NoFinding(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()

	legend := NewLegend(b, config)
	legend.AddItem("A", MustParseColor("#4E79A7"))
	legend.AddItem("B", MustParseColor("#F28E2B"))

	bounds := Rect{X: 10, Y: 10, W: 380, H: 280}
	legend.Draw(bounds)

	found := findFindingByCode(b.Findings(), FindingLegendOverflowDropped)
	if found != nil {
		t.Errorf("did not expect %q finding when all items fit, got: %v", FindingLegendOverflowDropped, found)
	}
}

// TestScatterChart_LabelSkippedFinding verifies that when scatter labels
// cannot be placed due to collision with all 4 candidate positions, a
// chart.scatter_label_skipped finding is emitted.
func TestScatterChart_LabelSkippedFinding(t *testing.T) {
	// Create many overlapping points with labels to force collisions.
	values := make([]any, 30)
	xValues := make([]any, 30)
	labels := make([]any, 30)
	for i := 0; i < 30; i++ {
		values[i] = 50.0 // All at the same Y
		xValues[i] = 50.0 // All at the same X — maximum collision
		labels[i] = "Label" + string(rune('A'+i%26))
	}

	req := &RequestEnvelope{
		Type: "scatter_chart",
		Data: map[string]any{
			"series": []any{
				map[string]any{
					"name":     "Cluster",
					"values":   values,
					"x_values": xValues,
					"labels":   labels,
				},
			},
		},
		Output: OutputSpec{Width: 200, Height: 200},
		Style:  StyleSpec{ShowValues: true},
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingScatterLabelSkipped)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingScatterLabelSkipped, output.Findings)
	}
	if found.Severity != "info" {
		t.Errorf("severity = %q, want %q", found.Severity, "info")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindIncreaseCanvas {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindIncreaseCanvas)
	}
}

// TestCapacityExceeded_TooManySeries verifies that a chart with more than
// MaxSeries series emits a chart.capacity_exceeded finding.
func TestCapacityExceeded_TooManySeries(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	b.CheckChartCapacity(core.MaxSeries+1, 5)

	found := findFindingByCode(b.Findings(), FindingCapacityExceeded)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingCapacityExceeded, b.Findings())
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindReduceItems {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindReduceItems)
	}
}

// TestCapacityExceeded_NoFindingUnderLimit verifies that a chart within
// capacity limits does NOT emit a capacity_exceeded finding.
func TestCapacityExceeded_NoFindingUnderLimit(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	b.CheckChartCapacity(5, 10)

	found := findFindingByCode(b.Findings(), FindingCapacityExceeded)
	if found != nil {
		t.Errorf("did not expect %q finding within limits, got: %v", FindingCapacityExceeded, found)
	}
}

// TestGanttOverflow_EmitsOverflowSuppressed verifies that a gantt chart with
// more rows than fit in the canvas emits a chart.overflow_suppressed finding.
func TestGanttOverflow_EmitsOverflowSuppressed(t *testing.T) {
	// Create many tasks to force overflow in a small canvas.
	tasks := make([]any, 30)
	for i := 0; i < 30; i++ {
		tasks[i] = map[string]any{
			"id":    string(rune('A' + i%26)),
			"name":  "Task " + string(rune('A'+i%26)),
			"start": "2024-01-01",
			"end":   "2024-01-15",
		}
	}

	req := &RequestEnvelope{
		Type: "gantt",
		Data: map[string]any{
			"tasks": tasks,
		},
		Output: OutputSpec{Width: 400, Height: 100}, // very short
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingOverflowSuppressed)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingOverflowSuppressed, output.Findings)
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
	}
	if found.Fix == nil {
		t.Error("expected Fix to be non-nil")
	} else if found.Fix.Kind != FixKindReduceItems {
		t.Errorf("Fix.Kind = %q, want %q", found.Fix.Kind, FixKindReduceItems)
	}
}

// TestOrgChartOverflow_EmitsOverflowSuppressed verifies that an org chart with
// siblings exceeding MaxVisibleSiblings emits a chart.overflow_suppressed finding.
func TestOrgChartOverflow_EmitsOverflowSuppressed(t *testing.T) {
	// Create an org chart with many children under one parent.
	children := make([]any, 15)
	for i := 0; i < 15; i++ {
		children[i] = map[string]any{
			"name":  "Employee " + string(rune('A'+i%26)),
			"title": "Staff",
		}
	}

	req := &RequestEnvelope{
		Type: "org_chart",
		Data: map[string]any{
			"root": map[string]any{
				"name":     "CEO",
				"title":    "Chief Executive",
				"children": children,
			},
		},
		Output: OutputSpec{Width: 400, Height: 300},
		// Default MaxVisibleSiblings is 9; 15 children triggers collapsing.
	}

	output, err := RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("RenderMultiFormatWithFindings() error = %v", err)
	}
	if output.SVG == nil {
		t.Fatal("expected SVG output")
	}

	found := findFindingByCode(output.Findings, FindingOverflowSuppressed)
	if found == nil {
		t.Fatalf("expected finding with code %q, got findings: %v", FindingOverflowSuppressed, output.Findings)
	}
	if found.Severity != "warning" {
		t.Errorf("severity = %q, want %q", found.Severity, "warning")
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
