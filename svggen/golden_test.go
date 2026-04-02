package svggen

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden test files")

// testdataDir returns the path to the testdata directory relative to the test file.
func testdataDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// NormalizeSVG removes non-deterministic elements from SVG for comparison.
// This includes:
// - Timestamp-based IDs or generated IDs
// - Floating point precision differences
// - Whitespace normalization
// - Embedded font data (base64)
func NormalizeSVG(svg []byte) []byte {
	content := string(svg)

	// Normalize embedded font data (base64 fonts can have slight variations)
	content = normalizeFonts(content)

	// Normalize whitespace in attribute values
	content = normalizeWhitespace(content)

	// Normalize floating point numbers to 2 decimal places
	content = normalizeFloats(content)

	// Normalize generated IDs (e.g., id="path123" -> id="path_N")
	content = normalizeIDs(content)

	// Normalize newlines
	content = strings.ReplaceAll(content, "\r\n", "\n")

	return []byte(content)
}

// normalizeFonts replaces embedded font data with a placeholder and sorts
// @font-face declarations for deterministic output (the canvas library may
// emit regular and bold faces in arbitrary order).
func normalizeFonts(s string) string {
	// Replace base64 font data with a placeholder.
	re := regexp.MustCompile(`data:[^;]+;base64,[A-Za-z0-9+/=]+`)
	s = re.ReplaceAllString(s, "data:font/normalized;base64,NORMALIZED_FONT_DATA")

	// Sort @font-face declarations within each <style> block so that the order
	// is deterministic regardless of how the canvas library emits them.
	styleRe := regexp.MustCompile(`(?s)<style>\n?(.*?)</style>`)
	s = styleRe.ReplaceAllStringFunc(s, func(block string) string {
		inner := styleRe.FindStringSubmatch(block)
		if len(inner) < 2 {
			return block
		}
		lines := strings.Split(strings.TrimSpace(inner[1]), "\n")
		sort.Strings(lines)
		return "<style>\n" + strings.Join(lines, "\n") + "\n</style>"
	})
	return s
}

// normalizeWhitespace normalizes excessive whitespace.
func normalizeWhitespace(s string) string {
	// Replace multiple spaces with single space
	re := regexp.MustCompile(`[ \t]+`)
	s = re.ReplaceAllString(s, " ")

	// Remove spaces around = in attributes
	s = regexp.MustCompile(` *= *`).ReplaceAllString(s, "=")

	return s
}

// normalizeFloats rounds floating point numbers to 2 decimal places.
func normalizeFloats(s string) string {
	re := regexp.MustCompile(`(\d+)\.(\d{3,})`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Keep only 2 decimal places for consistency
		parts := regexp.MustCompile(`(\d+)\.(\d{2})\d*`).FindStringSubmatch(match)
		if len(parts) == 3 {
			return parts[1] + "." + parts[2]
		}
		return match
	})
}

// normalizeIDs replaces generated IDs with sequential placeholders.
func normalizeIDs(s string) string {
	// Pattern matches common generated ID patterns
	patterns := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		// Normalize gradient IDs
		{regexp.MustCompile(`id="(gradient|linearGradient|radialGradient)-[a-f0-9]+"`), `id="$1-N"`},
		{regexp.MustCompile(`url\(#(gradient|linearGradient|radialGradient)-[a-f0-9]+\)`), `url(#$1-N)`},
		// Normalize clip-path IDs
		{regexp.MustCompile(`id="clip-[a-f0-9]+"`), `id="clip-N"`},
		{regexp.MustCompile(`clip-path="url\(#clip-[a-f0-9]+\)"`), `clip-path="url(#clip-N)"`},
		// Normalize filter IDs
		{regexp.MustCompile(`id="filter-[a-f0-9]+"`), `id="filter-N"`},
		{regexp.MustCompile(`filter="url\(#filter-[a-f0-9]+\)"`), `filter="url(#filter-N)"`},
	}

	result := s
	for _, p := range patterns {
		result = p.pattern.ReplaceAllString(result, p.replacement)
	}

	return result
}

// GoldenTest runs a golden test for a diagram type.
// If the -update flag is set, it writes the current output as the new golden file.
// Otherwise, it compares the current output against the golden file.
//
// In addition to golden file comparison, GoldenTest always runs structural
// quality checks (collectSVGQualityIssues) on the rendered output and logs
// any issues as warnings. This surfaces quality problems without blocking
// regression detection. The dedicated TestQualityGate_* tests in quality_test.go
// enforce quality as hard failures.
func GoldenTest(t *testing.T, name string, req *RequestEnvelope) {
	t.Helper()

	doc, err := Render(req)
	if err != nil {
		t.Fatalf("failed to render %s: %v", name, err)
	}

	// Quality gate: log structural quality issues as warnings on every golden
	// test render. This runs regardless of whether we are updating or comparing
	// golden files. Issues are logged (not failed) so golden tests remain focused
	// on regression detection. Use TestQualityGate_* tests for hard enforcement.
	if issues := collectSVGQualityIssues(string(doc.Content)); len(issues) > 0 {
		for _, issue := range issues {
			t.Logf("[SVG Quality Warning: %s] %s", issue.Category, issue.Message)
		}
	}

	normalized := NormalizeSVG(doc.Content)
	goldenPath := filepath.Join(testdataDir(), "golden", name+".svg")

	if *updateGolden {
		err := os.MkdirAll(filepath.Dir(goldenPath), 0755)
		if err != nil {
			t.Fatalf("failed to create golden directory: %v", err)
		}
		err = os.WriteFile(goldenPath, normalized, 0644)
		if err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		t.Fatalf("golden file not found: %s (run with -update to create)", goldenPath)
	}
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	if !bytes.Equal(normalized, expected) {
		t.Errorf("output differs from golden file %s", goldenPath)
		// Show a diff snippet for debugging
		showDiff(t, expected, normalized)
	}
}

// showDiff shows a summary of differences between expected and actual content.
func showDiff(t *testing.T, expected, actual []byte) {
	t.Helper()

	expectedLines := strings.Split(string(expected), "\n")
	actualLines := strings.Split(string(actual), "\n")

	// Find first difference
	diffCount := 0
	maxDiffs := 5

	for i := 0; i < len(expectedLines) || i < len(actualLines); i++ {
		if diffCount >= maxDiffs {
			t.Logf("... and more differences (showing first %d)", maxDiffs)
			break
		}

		var exp, act string
		if i < len(expectedLines) {
			exp = expectedLines[i]
		}
		if i < len(actualLines) {
			act = actualLines[i]
		}

		if exp != act {
			t.Logf("line %d differs:", i+1)
			if len(exp) > 100 {
				exp = exp[:100] + "..."
			}
			if len(act) > 100 {
				act = act[:100] + "..."
			}
			t.Logf("  expected: %s", exp)
			t.Logf("  actual:   %s", act)
			diffCount++
		}
	}
}

// TestGolden_AllDiagramTypes runs golden tests for all registered diagram types.
func TestGolden_AllDiagramTypes(t *testing.T) {
	testCases := []struct {
		name string
		req  *RequestEnvelope
	}{
		{
			name: "waterfall_basic",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"title": "Revenue Bridge",
					"points": []any{
						map[string]any{"label": "Start", "value": 100, "type": "increase"},
						map[string]any{"label": "Growth", "value": 30, "type": "increase"},
						map[string]any{"label": "Churn", "value": -15, "type": "decrease"},
						map[string]any{"label": "End", "value": 115, "type": "total"},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowValues: true, ShowGrid: true},
			},
		},
		{
			name: "matrix2x2_basic",
			req: &RequestEnvelope{
				Type: "matrix_2x2",
				Data: map[string]any{
					"title": "Strategic Matrix",
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
						map[string]any{"label": "Quick Wins", "position": "top_left"},
						map[string]any{"label": "Major Projects", "position": "top_right"},
						map[string]any{"label": "Fill-Ins", "position": "bottom_left"},
						map[string]any{"label": "Thankless Tasks", "position": "bottom_right"},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
			},
		},
		{
			name: "timeline_basic",
			req: &RequestEnvelope{
				Type: "timeline",
				Data: map[string]any{
					"title": "Project Timeline",
					"activities": []any{
						map[string]any{"label": "Planning", "start_date": "2024-01-01", "end_date": "2024-02-01"},
						map[string]any{"label": "Development", "start_date": "2024-02-01", "end_date": "2024-04-01"},
						map[string]any{"label": "Testing", "start_date": "2024-04-01", "end_date": "2024-05-01"},
					},
					"milestones": []any{
						map[string]any{"label": "Kickoff", "date": "2024-01-01"},
						map[string]any{"label": "Launch", "date": "2024-05-01"},
					},
				},
				Output: OutputSpec{Width: 800, Height: 400},
			},
		},
		{
			name: "pie_chart_basic",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{35.0, 25.0, 20.0, 15.0, 5.0},
					"categories": []any{"Product A", "Product B", "Product C", "Product D", "Other"},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "donut_chart_basic",
			req: &RequestEnvelope{
				Type: "donut_chart",
				Data: map[string]any{
					"values":     []any{40.0, 30.0, 20.0, 10.0},
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "venn_2circle",
			req: &RequestEnvelope{
				Type:  "venn",
				Title: "Skills Overlap",
				Data: map[string]any{
					"circles": []any{
						map[string]any{"label": "Engineering", "items": []any{"Coding", "Architecture"}},
						map[string]any{"label": "Design", "items": []any{"UI/UX", "Typography"}},
					},
					"intersections": map[string]any{
						"ab": map[string]any{"label": "Product Dev", "items": []any{"Prototyping"}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
			},
		},
		{
			name: "org_chart_basic",
			req: &RequestEnvelope{
				Type:  "org_chart",
				Title: "Company Structure",
				Data: map[string]any{
					"root": map[string]any{
						"name":  "Sarah Chen",
						"title": "CEO",
						"children": []any{
							map[string]any{
								"name":  "James Park",
								"title": "CTO",
								"children": []any{
									map[string]any{"name": "Alex Rivera", "title": "Backend Lead"},
									map[string]any{"name": "Maya Patel", "title": "Frontend Lead"},
								},
							},
							map[string]any{
								"name":  "Lisa Wong",
								"title": "CFO",
							},
							map[string]any{
								"name":  "Mike Taylor",
								"title": "COO",
								"children": []any{
									map[string]any{"name": "Emma Davis", "title": "Operations Mgr"},
								},
							},
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
			},
		},
		{
			name: "gantt_basic",
			req: &RequestEnvelope{
				Type:  "gantt",
				Title: "Product Development",
				Data: map[string]any{
					"tasks": []any{
						map[string]any{"id": "design", "label": "Design Phase", "start": "2024-01-01", "end": "2024-02-15", "category": "Design"},
						map[string]any{"id": "dev", "label": "Development", "start": "2024-02-15", "end": "2024-05-01", "category": "Engineering", "dependencies": []any{"design"}},
						map[string]any{"id": "test", "label": "Testing", "start": "2024-05-01", "end": "2024-06-15", "category": "QA", "dependencies": []any{"dev"}},
						map[string]any{"id": "launch", "label": "Launch", "date": "2024-07-01", "type": "milestone"},
					},
					"milestones": []any{
						map[string]any{"label": "Kickoff", "date": "2024-01-01"},
					},
				},
				Output: OutputSpec{Width: 900, Height: 500},
			},
		},
		{
			name: "venn_3circle",
			req: &RequestEnvelope{
				Type:  "venn",
				Title: "Innovation Triangle",
				Data: map[string]any{
					"circles": []any{
						map[string]any{"label": "Technology"},
						map[string]any{"label": "Business"},
						map[string]any{"label": "Design"},
					},
					"intersections": map[string]any{
						"ab":  map[string]any{"label": "Feasibility"},
						"ac":  map[string]any{"label": "Usability"},
						"bc":  map[string]any{"label": "Viability"},
						"abc": map[string]any{"label": "Innovation"},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
			},
		},
		{
			name: "bubble_chart_basic",
			req: &RequestEnvelope{
				Type:  "bubble_chart",
				Title: "Market Opportunity",
				Data: map[string]any{
					"series": []any{
						map[string]any{
							"name":          "Products",
							"x_values":      []any{10.0, 20.0, 30.0, 40.0, 50.0},
							"values":        []any{80.0, 60.0, 45.0, 90.0, 30.0},
							"bubble_values": []any{100.0, 200.0, 150.0, 300.0, 80.0},
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowGrid: true},
			},
		},
		{
			name: "fishbone_basic",
			req: &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect": "Low Productivity",
					"categories": []any{
						map[string]any{"name": "People", "causes": []any{"Skill gaps", "Low morale"}},
						map[string]any{"name": "Process", "causes": []any{"Bottlenecks", "Rework"}},
						map[string]any{"name": "Technology", "causes": []any{"Outdated tools", "System downtime"}},
						map[string]any{"name": "Environment", "causes": []any{"Noise", "Poor layout"}},
					},
				},
				Output: OutputSpec{Width: 900, Height: 500},
			},
		},
		{
			name: "stacked_area_chart_basic",
			req: &RequestEnvelope{
				Type:  "stacked_area_chart",
				Title: "Revenue by Channel",
				Data: map[string]any{
					"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					"series": []any{
						map[string]any{"name": "Direct", "values": []any{20.0, 25.0, 30.0, 35.0, 40.0, 45.0}},
						map[string]any{"name": "Partner", "values": []any{15.0, 18.0, 22.0, 25.0, 28.0, 30.0}},
						map[string]any{"name": "Online", "values": []any{10.0, 15.0, 20.0, 25.0, 30.0, 35.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true},
			},
		},
		{
			name: "grouped_bar_chart_basic",
			req: &RequestEnvelope{
				Type:  "grouped_bar_chart",
				Title: "Quarterly Revenue Comparison",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "2024", "values": []any{120.0, 150.0, 180.0, 200.0}},
						map[string]any{"name": "2025", "values": []any{140.0, 170.0, 210.0, 240.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "bar_chart_basic",
			req: &RequestEnvelope{
				Type:  "bar_chart",
				Title: "Monthly Sales",
				Data: map[string]any{
					"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					"series": []any{
						map[string]any{"name": "Sales", "values": []any{45.0, 52.0, 48.0, 61.0, 55.0, 67.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "bar_chart_logscale",
			req: &RequestEnvelope{
				Type:  "bar_chart",
				Title: "Company Valuations (Log Scale)",
				Data: map[string]any{
					"categories": []any{"Seed", "Series A", "Series B", "Series C", "IPO"},
					"series": []any{
						map[string]any{"name": "Valuation ($)", "values": []any{500_000.0, 5_000_000.0, 50_000_000.0, 500_000_000.0, 5_000_000_000.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "line_chart_basic",
			req: &RequestEnvelope{
				Type:  "line_chart",
				Title: "Website Traffic",
				Data: map[string]any{
					"categories": []any{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
					"series": []any{
						map[string]any{"name": "Visitors", "values": []any{1200.0, 1350.0, 1100.0, 1500.0, 1800.0, 900.0, 750.0}},
						map[string]any{"name": "Page Views", "values": []any{3600.0, 4050.0, 3300.0, 4500.0, 5400.0, 2700.0, 2250.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true},
			},
		},
		{
			name: "area_chart_basic",
			req: &RequestEnvelope{
				Type:  "area_chart",
				Title: "Memory Usage Over Time",
				Data: map[string]any{
					"categories": []any{"00:00", "04:00", "08:00", "12:00", "16:00", "20:00"},
					"series": []any{
						map[string]any{"name": "Heap", "values": []any{2.1, 2.4, 3.8, 4.2, 3.5, 2.9}},
						map[string]any{"name": "Stack", "values": []any{0.5, 0.6, 0.8, 0.9, 0.7, 0.6}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true},
			},
		},
		{
			name: "radar_chart_basic",
			req: &RequestEnvelope{
				Type:  "radar_chart",
				Title: "Team Skills Assessment",
				Data: map[string]any{
					"categories": []any{"Communication", "Technical", "Leadership", "Creativity", "Teamwork"},
					"series": []any{
						map[string]any{"name": "Alice", "values": []any{85.0, 90.0, 70.0, 80.0, 95.0}},
						map[string]any{"name": "Bob", "values": []any{75.0, 95.0, 60.0, 70.0, 80.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true},
			},
		},
		{
			name: "stacked_bar_chart_basic",
			req: &RequestEnvelope{
				Type:  "stacked_bar_chart",
				Title: "Revenue by Product Line",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "Hardware", "values": []any{80.0, 95.0, 110.0, 120.0}},
						map[string]any{"name": "Software", "values": []any{60.0, 75.0, 90.0, 100.0}},
						map[string]any{"name": "Services", "values": []any{30.0, 40.0, 45.0, 55.0}},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "funnel_chart_basic",
			req: &RequestEnvelope{
				Type:  "funnel_chart",
				Title: "Sales Pipeline",
				Data: map[string]any{
					"values": []any{
						map[string]any{"label": "Leads", "value": 10000.0},
						map[string]any{"label": "Qualified", "value": 6500.0},
						map[string]any{"label": "Proposals", "value": 3200.0},
						map[string]any{"label": "Negotiations", "value": 1800.0},
						map[string]any{"label": "Closed Won", "value": 900.0},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowValues: true},
			},
		},
		{
			name: "gauge_chart_basic",
			req: &RequestEnvelope{
				Type:  "gauge_chart",
				Title: "Customer Satisfaction",
				Data: map[string]any{
					"value": 73.0,
					"min":   0.0,
					"max":   100.0,
					"label": "NPS Score",
					"unit":  "%",
				},
				Output: OutputSpec{Width: 800, Height: 600},
			},
		},
		{
			name: "treemap_chart_basic",
			req: &RequestEnvelope{
				Type:  "treemap_chart",
				Title: "Disk Usage by Category",
				Data: map[string]any{
					"nodes": []any{
						map[string]any{"label": "Documents", "value": 45.0},
						map[string]any{"label": "Photos", "value": 30.0},
						map[string]any{"label": "Videos", "value": 80.0},
						map[string]any{"label": "Music", "value": 25.0},
						map[string]any{"label": "Applications", "value": 60.0},
						map[string]any{"label": "System", "value": 35.0},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowValues: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			GoldenTest(t, tc.name, tc.req)
		})
	}
}

// TestGolden_EdgeCases tests diagram rendering at min/max element configurations.
// These fixtures verify that house diagrams and value chains handle edge cases
// correctly: single/many pillars, zero/many activities, and overflow scenarios.
func TestGolden_EdgeCases(t *testing.T) {
	testCases := []struct {
		name string
		req  *RequestEnvelope
	}{
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			GoldenTest(t, tc.name, tc.req)
		})
	}
}

// TestGolden_MultiColumnLayouts tests diagram rendering at realistic presentation layout sizes.
// These fixtures reflect real scenarios:
// - Full-width (content_16x9: 1600x900): Single large chart spanning the content area
// - Half-width (half_16x9: 760x720): Chart in a 2-column layout (~8:9 portrait aspect)
// - Third-width (third_16x9: 500x720): Chart in a 3-column layout (~5:9 portrait aspect)
// - Circular in wide container: Pie/donut with contain fit mode preserving aspect ratio
func TestGolden_MultiColumnLayouts(t *testing.T) {
	testCases := []struct {
		name string
		req  *RequestEnvelope
	}{
		// Full-width (content_16x9): Large diagrams spanning the slide content area
		{
			name: "waterfall_fullwidth",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"title": "Annual Revenue Bridge",
					"points": []any{
						map[string]any{"label": "FY23 Start", "value": 1000, "type": "increase"},
						map[string]any{"label": "New Customers", "value": 250, "type": "increase"},
						map[string]any{"label": "Expansion", "value": 180, "type": "increase"},
						map[string]any{"label": "Churn", "value": -120, "type": "decrease"},
						map[string]any{"label": "Downgrades", "value": -60, "type": "decrease"},
						map[string]any{"label": "FY24 End", "value": 1250, "type": "total"},
					},
				},
				Output: OutputSpec{Preset: "content_16x9"},
				Style:  StyleSpec{ShowValues: true, ShowGrid: true},
			},
		},
		{
			name: "timeline_fullwidth",
			req: &RequestEnvelope{
				Type: "timeline",
				Data: map[string]any{
					"title": "Product Roadmap",
					"activities": []any{
						map[string]any{"label": "Discovery", "start_date": "2024-01-01", "end_date": "2024-03-01"},
						map[string]any{"label": "Design", "start_date": "2024-02-15", "end_date": "2024-05-01"},
						map[string]any{"label": "Development", "start_date": "2024-04-01", "end_date": "2024-09-01"},
						map[string]any{"label": "Testing", "start_date": "2024-08-01", "end_date": "2024-10-15"},
						map[string]any{"label": "Launch Prep", "start_date": "2024-10-01", "end_date": "2024-12-01"},
					},
					"milestones": []any{
						map[string]any{"label": "Kickoff", "date": "2024-01-01"},
						map[string]any{"label": "Alpha", "date": "2024-06-01"},
						map[string]any{"label": "Beta", "date": "2024-09-01"},
						map[string]any{"label": "GA", "date": "2024-12-01"},
					},
				},
				Output: OutputSpec{Preset: "content_16x9"},
			},
		},
		// Half-width (half_16x9): Charts in 2-column layouts (~8:9 portrait)
		{
			name: "pie_chart_halfwidth",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{42.0, 28.0, 18.0, 12.0},
					"categories": []any{"Enterprise", "Mid-Market", "SMB", "Startup"},
				},
				Output: OutputSpec{Preset: "half_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "donut_chart_halfwidth",
			req: &RequestEnvelope{
				Type: "donut_chart",
				Data: map[string]any{
					"values":     []any{55.0, 25.0, 15.0, 5.0},
					"categories": []any{"Recurring", "Services", "Add-ons", "Other"},
				},
				Output: OutputSpec{Preset: "half_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "matrix2x2_halfwidth",
			req: &RequestEnvelope{
				Type: "matrix_2x2",
				Data: map[string]any{
					"title": "Priority Matrix",
					"x_axis": map[string]any{
						"label":      "Impact",
						"low_label":  "Low",
						"high_label": "High",
					},
					"y_axis": map[string]any{
						"label":      "Urgency",
						"low_label":  "Low",
						"high_label": "High",
					},
					"quadrants": []any{
						map[string]any{"label": "Schedule", "position": "top_left"},
						map[string]any{"label": "Do First", "position": "top_right"},
						map[string]any{"label": "Delegate", "position": "bottom_left"},
						map[string]any{"label": "Eliminate", "position": "bottom_right"},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
			},
		},
		{
			name: "waterfall_halfwidth",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"title": "Q4 Budget",
					"points": []any{
						map[string]any{"label": "Start", "value": 500, "type": "increase"},
						map[string]any{"label": "Sales", "value": 150, "type": "increase"},
						map[string]any{"label": "Costs", "value": -80, "type": "decrease"},
						map[string]any{"label": "End", "value": 570, "type": "total"},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
				Style:  StyleSpec{ShowValues: true},
			},
		},
		// Density variants: bar chart with 10 categories at narrow widths
		{
			name: "bar_chart_halfwidth_dense",
			req: &RequestEnvelope{
				Type:  "bar_chart",
				Title: "Monthly Metrics",
				Data: map[string]any{
					"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct"},
					"series": []any{
						map[string]any{"name": "Sales", "values": []any{45.0, 52.0, 48.0, 61.0, 55.0, 67.0, 72.0, 63.0, 58.0, 70.0}},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "bar_chart_thirdwidth_dense",
			req: &RequestEnvelope{
				Type:  "bar_chart",
				Title: "Monthly Metrics",
				Data: map[string]any{
					"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct"},
					"series": []any{
						map[string]any{"name": "Sales", "values": []any{45.0, 52.0, 48.0, 61.0, 55.0, 67.0, 72.0, 63.0, 58.0, 70.0}},
					},
				},
				Output: OutputSpec{Preset: "third_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		// Density variants: waterfall with 10 points at narrow widths
		{
			name: "waterfall_thirdwidth_dense",
			req: &RequestEnvelope{
				Type: "waterfall",
				Data: map[string]any{
					"title": "Annual P&L",
					"points": []any{
						map[string]any{"label": "Revenue", "value": 1000, "type": "increase"},
						map[string]any{"label": "COGS", "value": -350, "type": "decrease"},
						map[string]any{"label": "Gross Profit", "value": 650, "type": "total"},
						map[string]any{"label": "R&D", "value": -120, "type": "decrease"},
						map[string]any{"label": "Sales", "value": -90, "type": "decrease"},
						map[string]any{"label": "G&A", "value": -60, "type": "decrease"},
						map[string]any{"label": "Marketing", "value": -45, "type": "decrease"},
						map[string]any{"label": "Other", "value": 15, "type": "increase"},
						map[string]any{"label": "Net Income", "value": 350, "type": "total"},
					},
				},
				Output: OutputSpec{Preset: "third_16x9"},
				Style:  StyleSpec{ShowValues: true},
			},
		},
		// Density variants: grouped bar with 8 categories at narrow widths
		{
			name: "grouped_bar_chart_halfwidth_dense",
			req: &RequestEnvelope{
				Type:  "grouped_bar_chart",
				Title: "Regional Performance",
				Data: map[string]any{
					"categories": []any{"North", "South", "East", "West", "Central", "NE", "NW", "SE"},
					"series": []any{
						map[string]any{"name": "2025", "values": []any{80.0, 65.0, 90.0, 75.0, 60.0, 85.0, 70.0, 95.0}},
						map[string]any{"name": "2026", "values": []any{95.0, 72.0, 105.0, 88.0, 68.0, 92.0, 78.0, 110.0}},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "grouped_bar_chart_thirdwidth_dense",
			req: &RequestEnvelope{
				Type:  "grouped_bar_chart",
				Title: "Regional Performance",
				Data: map[string]any{
					"categories": []any{"North", "South", "East", "West", "Central", "NE", "NW", "SE"},
					"series": []any{
						map[string]any{"name": "2025", "values": []any{80.0, 65.0, 90.0, 75.0, 60.0, 85.0, 70.0, 95.0}},
						map[string]any{"name": "2026", "values": []any{95.0, 72.0, 105.0, 88.0, 68.0, 92.0, 78.0, 110.0}},
					},
				},
				Output: OutputSpec{Preset: "third_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		// Density variants: stacked bar with 8 categories at narrow widths
		{
			name: "stacked_bar_chart_halfwidth_dense",
			req: &RequestEnvelope{
				Type:  "stacked_bar_chart",
				Title: "Headcount by Department",
				Data: map[string]any{
					"categories": []any{"Q1'24", "Q2'24", "Q3'24", "Q4'24", "Q1'25", "Q2'25", "Q3'25", "Q4'25"},
					"series": []any{
						map[string]any{"name": "Eng", "values": []any{40.0, 42.0, 45.0, 48.0, 50.0, 53.0, 55.0, 58.0}},
						map[string]any{"name": "Sales", "values": []any{25.0, 27.0, 28.0, 30.0, 32.0, 33.0, 35.0, 36.0}},
						map[string]any{"name": "Ops", "values": []any{15.0, 15.0, 16.0, 16.0, 17.0, 17.0, 18.0, 18.0}},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			name: "stacked_bar_chart_thirdwidth_dense",
			req: &RequestEnvelope{
				Type:  "stacked_bar_chart",
				Title: "Headcount by Dept",
				Data: map[string]any{
					"categories": []any{"Q1'24", "Q2'24", "Q3'24", "Q4'24", "Q1'25", "Q2'25", "Q3'25", "Q4'25"},
					"series": []any{
						map[string]any{"name": "Eng", "values": []any{40.0, 42.0, 45.0, 48.0, 50.0, 53.0, 55.0, 58.0}},
						map[string]any{"name": "Sales", "values": []any{25.0, 27.0, 28.0, 30.0, 32.0, 33.0, 35.0, 36.0}},
						map[string]any{"name": "Ops", "values": []any{15.0, 15.0, 16.0, 16.0, 17.0, 17.0, 18.0, 18.0}},
					},
				},
				Output: OutputSpec{Preset: "third_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		// Density variants: gantt with 10 tasks at narrow widths
		{
			name: "gantt_halfwidth_dense",
			req: &RequestEnvelope{
				Type:  "gantt",
				Title: "Project Timeline",
				Data: map[string]any{
					"start_date": "2026-01-01",
					"end_date":   "2026-06-30",
					"tasks": []any{
						map[string]any{"name": "Research", "start": "2026-01-01", "end": "2026-01-31", "progress": 100},
						map[string]any{"name": "Design", "start": "2026-01-15", "end": "2026-02-28", "progress": 80},
						map[string]any{"name": "Backend", "start": "2026-02-01", "end": "2026-04-15", "progress": 60},
						map[string]any{"name": "Frontend", "start": "2026-02-15", "end": "2026-04-30", "progress": 40},
						map[string]any{"name": "Testing", "start": "2026-03-15", "end": "2026-05-15", "progress": 20},
						map[string]any{"name": "Security", "start": "2026-04-01", "end": "2026-05-30", "progress": 10},
						map[string]any{"name": "Docs", "start": "2026-04-15", "end": "2026-05-31", "progress": 0},
						map[string]any{"name": "Training", "start": "2026-05-01", "end": "2026-06-15", "progress": 0},
						map[string]any{"name": "Deploy", "start": "2026-05-15", "end": "2026-06-15", "progress": 0},
						map[string]any{"name": "Launch", "start": "2026-06-01", "end": "2026-06-30", "progress": 0},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
			},
		},
		{
			name: "gantt_thirdwidth_dense",
			req: &RequestEnvelope{
				Type:  "gantt",
				Title: "Project Timeline",
				Data: map[string]any{
					"start_date": "2026-01-01",
					"end_date":   "2026-06-30",
					"tasks": []any{
						map[string]any{"name": "Research", "start": "2026-01-01", "end": "2026-01-31", "progress": 100},
						map[string]any{"name": "Design", "start": "2026-01-15", "end": "2026-02-28", "progress": 80},
						map[string]any{"name": "Backend", "start": "2026-02-01", "end": "2026-04-15", "progress": 60},
						map[string]any{"name": "Frontend", "start": "2026-02-15", "end": "2026-04-30", "progress": 40},
						map[string]any{"name": "Testing", "start": "2026-03-15", "end": "2026-05-15", "progress": 20},
						map[string]any{"name": "Security", "start": "2026-04-01", "end": "2026-05-30", "progress": 10},
						map[string]any{"name": "Docs", "start": "2026-04-15", "end": "2026-05-31", "progress": 0},
						map[string]any{"name": "Training", "start": "2026-05-01", "end": "2026-06-15", "progress": 0},
						map[string]any{"name": "Deploy", "start": "2026-05-15", "end": "2026-06-15", "progress": 0},
						map[string]any{"name": "Launch", "start": "2026-06-01", "end": "2026-06-30", "progress": 0},
					},
				},
				Output: OutputSpec{Preset: "third_16x9"},
			},
		},
		{
			name: "fishbone_halfwidth",
			req: &RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect": "Low Productivity",
					"categories": []any{
						map[string]any{"name": "People", "causes": []any{"Skill gaps", "Low morale"}},
						map[string]any{"name": "Process", "causes": []any{"Bottlenecks", "Rework"}},
						map[string]any{"name": "Technology", "causes": []any{"Outdated tools", "System downtime"}},
						map[string]any{"name": "Environment", "causes": []any{"Noise", "Poor layout"}},
					},
				},
				Output: OutputSpec{Preset: "half_16x9"},
			},
		},
		// Third-width (third_16x9): Charts in 3-column layouts (~5:9 portrait)
		{
			name: "fishbone_thirdwidth",
			req: &RequestEnvelope{
				Type:  "fishbone",
				Title: "Quality Issues",
				Data: map[string]any{
					"effect": "Defects",
					"categories": []any{
						map[string]any{"name": "Materials", "causes": []any{"Poor quality", "Wrong spec"}},
						map[string]any{"name": "Methods", "causes": []any{"Outdated procedures"}},
						map[string]any{"name": "Machines", "causes": []any{"Wear and tear", "Calibration"}},
					},
				},
				Output: OutputSpec{Preset: "third_16x9"},
			},
		},
		{
			name: "pie_chart_thirdwidth",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{60.0, 25.0, 15.0},
					"categories": []any{"Primary", "Secondary", "Other"},
				},
				Output: OutputSpec{Preset: "third_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "donut_chart_thirdwidth",
			req: &RequestEnvelope{
				Type: "donut_chart",
				Data: map[string]any{
					"values":     []any{70.0, 30.0},
					"categories": []any{"Complete", "Remaining"},
				},
				Output: OutputSpec{Preset: "third_16x9"},
				Style:  StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "matrix2x2_thirdwidth",
			req: &RequestEnvelope{
				Type: "matrix_2x2",
				Data: map[string]any{
					"title": "Risk Assessment",
					"x_axis": map[string]any{
						"label":      "Likelihood",
						"low_label":  "Low",
						"high_label": "High",
					},
					"y_axis": map[string]any{
						"label":      "Impact",
						"low_label":  "Low",
						"high_label": "High",
					},
					"quadrants": []any{
						map[string]any{"label": "Monitor", "position": "top_left"},
						map[string]any{"label": "Mitigate", "position": "top_right"},
						map[string]any{"label": "Accept", "position": "bottom_left"},
						map[string]any{"label": "Transfer", "position": "bottom_right"},
					},
				},
				Output: OutputSpec{Preset: "third_16x9"},
			},
		},
		// Circular charts in non-square (wide) containers with contain fit mode
		// These test that pie/donut charts maintain their circular shape
		{
			name: "pie_chart_wide_contain",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{40.0, 35.0, 25.0},
					"categories": []any{"North", "South", "West"},
				},
				Output: OutputSpec{
					Width:   1200,
					Height:  500,
					FitMode: "contain",
				},
				Style: StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			name: "donut_chart_wide_contain",
			req: &RequestEnvelope{
				Type: "donut_chart",
				Data: map[string]any{
					"values":     []any{65.0, 35.0},
					"categories": []any{"Target Met", "Gap"},
				},
				Output: OutputSpec{
					Width:   1000,
					Height:  400,
					FitMode: "contain",
				},
				Style: StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		// Tall portrait containers (simulating narrow sidebars)
		{
			name: "pie_chart_tall_contain",
			req: &RequestEnvelope{
				Type: "pie_chart",
				Data: map[string]any{
					"values":     []any{50.0, 30.0, 20.0},
					"categories": []any{"Desktop", "Mobile", "Tablet"},
				},
				Output: OutputSpec{
					Width:   400,
					Height:  800,
					FitMode: "contain",
				},
				Style: StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		// Scatter chart with axis titles
		{
			name: "scatter_chart_axis_titles",
			req: &RequestEnvelope{
				Type:  "scatter_chart",
				Title: "Performance vs Experience",
				Data: map[string]any{
					"x_label": "Years of Experience",
					"y_label": "Performance Score",
					"series": []any{
						map[string]any{
							"name":     "Employees",
							"x_values": []any{1.0, 3.0, 5.0, 7.0, 10.0, 12.0},
							"values":   []any{55.0, 65.0, 72.0, 80.0, 88.0, 92.0},
						},
					},
				},
				Output: OutputSpec{Width: 800, Height: 600},
				Style:  StyleSpec{ShowGrid: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			GoldenTest(t, tc.name, tc.req)
		})
	}
}

// TestNormalizeSVG tests the SVG normalization functions.
func TestNormalizeSVG(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normalize floats",
			input:    `<rect x="10.12345" y="20.99999"/>`,
			expected: `<rect x="10.12" y="20.99"/>`,
		},
		{
			name:     "normalize whitespace",
			input:    `<rect   x = "10"  y = "20" />`,
			expected: `<rect x="10" y="20" />`,
		},
		{
			name:     "normalize gradient IDs",
			input:    `<linearGradient id="gradient-abc123"/>`,
			expected: `<linearGradient id="gradient-N"/>`,
		},
		{
			name:     "normalize gradient references",
			input:    `fill="url(#gradient-abc123)"`,
			expected: `fill="url(#gradient-N)"`,
		},
		{
			name:     "normalize embedded fonts",
			input:    `@font-face{font-family:'Arial';src:url('data:type/opentype;base64,AAEAAAAYAQAABACAR=');}`,
			expected: `@font-face{font-family:'Arial';src:url('data:font/normalized;base64,NORMALIZED_FONT_DATA');}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := string(NormalizeSVG([]byte(tc.input)))
			if result != tc.expected {
				t.Errorf("NormalizeSVG(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
