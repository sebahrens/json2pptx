package svggen

import (
	"strconv"
	"testing"
)

// =============================================================================
// Benchmark Request Generators
// =============================================================================

// These functions generate representative requests for each diagram type.
// Each request is designed to exercise typical rendering paths.

func benchmarkBarChartRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "bar_chart",
		Title: "Quarterly Revenue",
		Data: map[string]any{
			"categories": []any{"Q1", "Q2", "Q3", "Q4"},
			"series": []any{
				map[string]any{"name": "2023", "values": []any{100.0, 120.0, 95.0, 140.0}},
				map[string]any{"name": "2024", "values": []any{110.0, 135.0, 105.0, 155.0}},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowLegend: true, ShowValues: true, ShowGrid: true},
	}
}

func benchmarkLineChartRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "line_chart",
		Title: "Monthly Trends",
		Data: map[string]any{
			"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
			"series": []any{
				map[string]any{"name": "Sales", "values": []any{50.0, 60.0, 55.0, 70.0, 65.0, 80.0}},
				map[string]any{"name": "Costs", "values": []any{40.0, 42.0, 38.0, 45.0, 48.0, 50.0}},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowLegend: true, ShowValues: true, ShowGrid: true},
	}
}

func benchmarkPieChartRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "pie_chart",
		Title: "Market Share",
		Data: map[string]any{
			"categories": []any{"Product A", "Product B", "Product C", "Product D"},
			"values":     []any{35.0, 25.0, 22.0, 18.0},
		},
		Output: OutputSpec{Width: 600, Height: 600},
		Style:  StyleSpec{ShowLegend: true, ShowValues: true},
	}
}

func benchmarkDonutChartRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "donut_chart",
		Title: "Expense Breakdown",
		Data: map[string]any{
			"categories": []any{"Salary", "Marketing", "Operations", "R&D", "Other"},
			"values":     []any{40.0, 20.0, 15.0, 15.0, 10.0},
		},
		Output: OutputSpec{Width: 600, Height: 600},
		Style:  StyleSpec{ShowLegend: true, ShowValues: true},
	}
}

func benchmarkWaterfallRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "waterfall",
		Title: "Revenue Bridge",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Start", "value": 100, "type": "increase"},
				map[string]any{"label": "New Sales", "value": 30, "type": "increase"},
				map[string]any{"label": "Upsell", "value": 15, "type": "increase"},
				map[string]any{"label": "Churn", "value": -20, "type": "decrease"},
				map[string]any{"label": "Downgrades", "value": -10, "type": "decrease"},
				map[string]any{"label": "End", "value": 115, "type": "total"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowValues: true, ShowGrid: true},
	}
}

func benchmarkMatrix2x2Request() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "matrix_2x2",
		Title: "Priority Matrix",
		Data: map[string]any{
			"x_axis": map[string]any{
				"label":      "Impact",
				"low_label":  "Low",
				"high_label": "High",
			},
			"y_axis": map[string]any{
				"label":      "Effort",
				"low_label":  "Low",
				"high_label": "High",
			},
			"quadrants": []any{
				map[string]any{"label": "Quick Wins", "position": "top_left", "description": "High impact, low effort"},
				map[string]any{"label": "Major Projects", "position": "top_right", "description": "High impact, high effort"},
				map[string]any{"label": "Fill-Ins", "position": "bottom_left", "description": "Low impact, low effort"},
				map[string]any{"label": "Thankless Tasks", "position": "bottom_right", "description": "Low impact, high effort"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}
}

func benchmarkPortersFiveForcesRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "porters_five_forces",
		Title: "Industry Analysis",
		Data: map[string]any{
			"forces": map[string]any{
				"supplier_power":         map[string]any{"level": "high", "description": "Few suppliers, high switching costs"},
				"buyer_power":            map[string]any{"level": "medium", "description": "Price sensitive but fragmented"},
				"competitive_rivalry":    map[string]any{"level": "high", "description": "Many well-funded competitors"},
				"threat_of_substitution": map[string]any{"level": "low", "description": "Few viable alternatives"},
				"threat_of_new_entry":    map[string]any{"level": "medium", "description": "Moderate capital requirements"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}
}

func benchmarkTimelineRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "timeline",
		Title: "Project Timeline",
		Data: map[string]any{
			"activities": []any{
				map[string]any{"label": "Discovery", "start": "2024-01-01", "end": "2024-02-01"},
				map[string]any{"label": "Design", "start": "2024-02-01", "end": "2024-03-15"},
				map[string]any{"label": "Development", "start": "2024-03-01", "end": "2024-06-01"},
				map[string]any{"label": "Testing", "start": "2024-05-15", "end": "2024-07-01"},
				map[string]any{"label": "Deployment", "start": "2024-07-01", "end": "2024-07-15"},
			},
			"milestones": []any{
				map[string]any{"label": "Kickoff", "date": "2024-01-01"},
				map[string]any{"label": "Design Complete", "date": "2024-03-15"},
				map[string]any{"label": "Launch", "date": "2024-07-15"},
			},
		},
		Output: OutputSpec{Width: 1000, Height: 500},
	}
}

func benchmarkBusinessModelCanvasRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "business_model_canvas",
		Title: "SaaS Business Model",
		Data: map[string]any{
			"blocks": map[string]any{
				"key_partners":           []any{"Cloud providers", "Payment processors", "Integration partners"},
				"key_activities":         []any{"Product development", "Customer support", "Marketing"},
				"key_resources":          []any{"Engineering team", "Platform", "Data"},
				"value_propositions":     []any{"Ease of use", "Integration", "Analytics", "Support"},
				"customer_relationships": []any{"Self-service", "Premium support", "Community"},
				"channels":               []any{"Website", "App stores", "Partners", "Sales"},
				"customer_segments":      []any{"SMBs", "Enterprise", "Developers"},
				"cost_structure":         []any{"Infrastructure", "Salaries", "Marketing", "Support"},
				"revenue_streams":        []any{"Subscriptions", "Enterprise deals", "Marketplace"},
			},
		},
		Output: OutputSpec{Width: 1200, Height: 800},
	}
}

func benchmarkValueChainRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "value_chain",
		Title: "Porter's Value Chain",
		Data: map[string]any{
			"primary_activities": []any{
				map[string]any{"label": "Inbound Logistics", "items": []any{"Receiving", "Storage", "Inventory"}},
				map[string]any{"label": "Operations", "items": []any{"Manufacturing", "Assembly", "Testing"}},
				map[string]any{"label": "Outbound Logistics", "items": []any{"Warehousing", "Distribution", "Delivery"}},
				map[string]any{"label": "Marketing & Sales", "items": []any{"Advertising", "Promotion", "Pricing"}},
				map[string]any{"label": "Service", "items": []any{"Installation", "Repair", "Training"}},
			},
			"support_activities": []any{
				map[string]any{"label": "Firm Infrastructure", "items": []any{"Management", "Finance", "Legal"}},
				map[string]any{"label": "Human Resource Mgmt", "items": []any{"Recruiting", "Training", "Compensation"}},
				map[string]any{"label": "Technology Development", "items": []any{"R&D", "IT", "Automation"}},
				map[string]any{"label": "Procurement", "items": []any{"Purchasing", "Supplier management"}},
			},
		},
		Output: OutputSpec{Width: 1200, Height: 700},
	}
}

func benchmarkNineBoxTalentRequest() *RequestEnvelope {
	return &RequestEnvelope{
		Type:  "nine_box_talent",
		Title: "Talent Assessment",
		Data: map[string]any{
			"x_axis": map[string]any{
				"label": "Performance",
			},
			"y_axis": map[string]any{
				"label": "Potential",
			},
			"employees": []any{
				map[string]any{"name": "Alice", "performance": "high", "potential": "high"},
				map[string]any{"name": "Bob", "performance": "medium", "potential": "high"},
				map[string]any{"name": "Carol", "performance": "high", "potential": "medium"},
				map[string]any{"name": "Dave", "performance": "medium", "potential": "medium"},
				map[string]any{"name": "Eve", "performance": "low", "potential": "high"},
				map[string]any{"name": "Frank", "performance": "high", "potential": "low"},
			},
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}
}

// =============================================================================
// Individual Diagram Benchmarks
// =============================================================================

func BenchmarkBarChart(b *testing.B) {
	req := benchmarkBarChartRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkLineChart(b *testing.B) {
	req := benchmarkLineChartRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkPieChart(b *testing.B) {
	req := benchmarkPieChartRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkDonutChart(b *testing.B) {
	req := benchmarkDonutChartRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkWaterfall(b *testing.B) {
	req := benchmarkWaterfallRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkMatrix2x2(b *testing.B) {
	req := benchmarkMatrix2x2Request()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkPortersFiveForces(b *testing.B) {
	req := benchmarkPortersFiveForcesRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}


func BenchmarkTimeline(b *testing.B) {
	req := benchmarkTimelineRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkBusinessModelCanvas(b *testing.B) {
	req := benchmarkBusinessModelCanvasRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkValueChain(b *testing.B) {
	req := benchmarkValueChainRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

func BenchmarkNineBoxTalent(b *testing.B) {
	req := benchmarkNineBoxTalentRequest()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Render(req)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}

// =============================================================================
// Aggregate Benchmarks
// =============================================================================

// BenchmarkAllDiagrams runs all diagram types in sequence.
// This simulates a batch rendering scenario.
func BenchmarkAllDiagrams(b *testing.B) {
	requests := []*RequestEnvelope{
		benchmarkBarChartRequest(),
		benchmarkLineChartRequest(),
		benchmarkPieChartRequest(),
		benchmarkDonutChartRequest(),
		benchmarkWaterfallRequest(),
		benchmarkMatrix2x2Request(),
		benchmarkPortersFiveForcesRequest(),
		benchmarkTimelineRequest(),
		benchmarkBusinessModelCanvasRequest(),
		benchmarkValueChainRequest(),
		benchmarkNineBoxTalentRequest(),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, req := range requests {
			_, err := Render(req)
			if err != nil {
				b.Fatalf("render failed for %s: %v", req.Type, err)
			}
		}
	}
}

// =============================================================================
// Scaling Benchmarks
// =============================================================================

// BenchmarkBarChart_DataScaling tests how rendering scales with data size.
func BenchmarkBarChart_DataScaling(b *testing.B) {
	sizes := []struct {
		name       string
		categories int
		series     int
	}{
		{"4x2", 4, 2},
		{"8x4", 8, 4},
		{"12x6", 12, 6},
		{"20x8", 20, 8},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			req := generateBarChartRequest(size.categories, size.series)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := Render(req)
				if err != nil {
					b.Fatalf("render failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkPieChart_DataScaling tests how pie chart rendering scales with slice count.
func BenchmarkPieChart_DataScaling(b *testing.B) {
	sliceCounts := []int{4, 8, 12, 16}

	for _, count := range sliceCounts {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			req := generatePieChartRequest(count)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := Render(req)
				if err != nil {
					b.Fatalf("render failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkTimeline_DataScaling tests how timeline rendering scales.
func BenchmarkTimeline_DataScaling(b *testing.B) {
	activityCounts := []int{3, 6, 10, 15}

	for _, count := range activityCounts {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			req := generateTimelineRequest(count)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := Render(req)
				if err != nil {
					b.Fatalf("render failed: %v", err)
				}
			}
		})
	}
}

// =============================================================================
// Resolution Benchmarks
// =============================================================================

// BenchmarkBarChart_Resolution tests how rendering scales with output resolution.
func BenchmarkBarChart_Resolution(b *testing.B) {
	resolutions := []struct {
		name   string
		width  int
		height int
	}{
		{"400x300", 400, 300},
		{"800x600", 800, 600},
		{"1200x900", 1200, 900},
		{"1920x1080", 1920, 1080},
	}

	for _, res := range resolutions {
		b.Run(res.name, func(b *testing.B) {
			req := benchmarkBarChartRequest()
			req.Output.Width = res.width
			req.Output.Height = res.height
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := Render(req)
				if err != nil {
					b.Fatalf("render failed: %v", err)
				}
			}
		})
	}
}

// =============================================================================
// Helpers
// =============================================================================

func generateBarChartRequest(categories, seriesCount int) *RequestEnvelope {
	cats := make([]any, categories)
	for i := 0; i < categories; i++ {
		cats[i] = "Cat" + strconv.Itoa(i+1)
	}

	series := make([]any, seriesCount)
	for s := 0; s < seriesCount; s++ {
		values := make([]any, categories)
		for i := 0; i < categories; i++ {
			values[i] = float64(50 + i*10 + s*5)
		}
		series[s] = map[string]any{
			"name":   "Series" + strconv.Itoa(s+1),
			"values": values,
		}
	}

	return &RequestEnvelope{
		Type:  "bar_chart",
		Title: "Scaled Bar Chart",
		Data: map[string]any{
			"categories": cats,
			"series":     series,
		},
		Output: OutputSpec{Width: 800, Height: 600},
		Style:  StyleSpec{ShowLegend: true, ShowValues: true},
	}
}

func generatePieChartRequest(slices int) *RequestEnvelope {
	cats := make([]any, slices)
	values := make([]any, slices)
	for i := 0; i < slices; i++ {
		cats[i] = "Slice" + strconv.Itoa(i+1)
		values[i] = float64(100 / slices)
	}

	return &RequestEnvelope{
		Type:  "pie_chart",
		Title: "Scaled Pie Chart",
		Data: map[string]any{
			"categories": cats,
			"values":     values,
		},
		Output: OutputSpec{Width: 600, Height: 600},
		Style:  StyleSpec{ShowLegend: true, ShowValues: true},
	}
}

func generateTimelineRequest(activities int) *RequestEnvelope {
	acts := make([]any, activities)
	for i := 0; i < activities; i++ {
		acts[i] = map[string]any{
			"label": "Activity" + strconv.Itoa(i+1),
			"start": dateFromOffset(i * 30),
			"end":   dateFromOffset(i*30 + 30),
		}
	}

	return &RequestEnvelope{
		Type:  "timeline",
		Title: "Scaled Timeline",
		Data: map[string]any{
			"activities": acts,
			"milestones": []any{},
		},
		Output: OutputSpec{Width: 1000, Height: 400 + activities*20},
	}
}

func dateFromOffset(days int) string {
	// Simple date calculation starting from 2024-01-01
	month := (days / 30) + 1
	day := (days % 30) + 1
	if month > 12 {
		month = 12
		day = 28
	}
	return "2024-" + padZero(month) + "-" + padZero(day)
}

func padZero(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
