package svggen_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// TestFontSizeInspection generates SVGs for all major diagram types at
// both full-width and half-width PPTX placeholder sizes, converts them
// to PNGs via rsvg-convert, and creates an HTML gallery for visual inspection.
//
// Run with: FONT_INSPECT=1 go test ./... -run TestFontSizeInspection -v -count=1
// Then open: /tmp/svg_font_inspection/gallery.html
func TestFontSizeInspection(t *testing.T) {
	if os.Getenv("FONT_INSPECT") == "" {
		t.Skip("Set FONT_INSPECT=1 to run visual font inspection")
	}

	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skipf("rsvg-convert not found: %v", err)
	}

	outDir := "/tmp/svg_font_inspection"
	os.MkdirAll(outDir, 0755)

	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"full", 720, 420},
		{"half", 360, 420},
	}

	diagrams := []struct {
		name string
		req  svggen.RequestEnvelope
	}{
		// Standard chart types (categories + series format)
		{
			"bar_chart",
			svggen.RequestEnvelope{
				Type:  "bar_chart",
				Title: "Revenue by Region",
				Data: map[string]any{
					"categories": []any{"North", "South", "East", "West"},
					"series": []any{
						map[string]any{"name": "2025", "values": []any{250.0, 180.0, 220.0, 310.0}},
					},
				},
				Style: svggen.StyleSpec{ShowValues: true, ShowGrid: true},
			},
		},
		{
			"line_chart",
			svggen.RequestEnvelope{
				Type:  "line_chart",
				Title: "Monthly Trend",
				Data: map[string]any{
					"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
					"series": []any{
						map[string]any{"name": "Revenue", "values": []any{10.0, 25.0, 18.0, 35.0, 28.0, 42.0}},
					},
				},
				Style: svggen.StyleSpec{ShowGrid: true},
			},
		},
		{
			"area_chart",
			svggen.RequestEnvelope{
				Type:  "area_chart",
				Title: "Growth Trend",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "Revenue", "values": []any{100.0, 150.0, 130.0, 200.0}},
					},
				},
				Style: svggen.StyleSpec{ShowGrid: true},
			},
		},
		{
			"stacked_bar_chart",
			svggen.RequestEnvelope{
				Type:  "stacked_bar_chart",
				Title: "Revenue by Product Line",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "Product A", "values": []any{30.0, 35.0, 40.0, 45.0}},
						map[string]any{"name": "Product B", "values": []any{20.0, 25.0, 22.0, 28.0}},
						map[string]any{"name": "Product C", "values": []any{15.0, 18.0, 20.0, 22.0}},
					},
				},
				Style: svggen.StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			"grouped_bar_chart",
			svggen.RequestEnvelope{
				Type:  "grouped_bar_chart",
				Title: "Regional Comparison",
				Data: map[string]any{
					"categories": []any{"Q1", "Q2", "Q3", "Q4"},
					"series": []any{
						map[string]any{"name": "2024", "values": []any{120.0, 150.0, 180.0, 200.0}},
						map[string]any{"name": "2025", "values": []any{140.0, 170.0, 210.0, 240.0}},
					},
				},
				Style: svggen.StyleSpec{ShowLegend: true, ShowGrid: true, ShowValues: true},
			},
		},
		{
			"radar_chart",
			svggen.RequestEnvelope{
				Type:  "radar_chart",
				Title: "Capability Assessment",
				Data: map[string]any{
					"categories": []any{"Strategy", "Execution", "Innovation", "Culture", "Technology"},
					"series": []any{
						map[string]any{"name": "Current", "values": []any{80.0, 65.0, 90.0, 75.0, 85.0}},
					},
					"max_value": 100.0,
				},
			},
		},
		{
			"scatter_chart",
			svggen.RequestEnvelope{
				Type:  "scatter_chart",
				Title: "Risk vs Return",
				Data: map[string]any{
					"series": []any{
						map[string]any{
							"name": "Portfolio",
							"points": []any{
								map[string]any{"x": 10.0, "y": 5.0},
								map[string]any{"x": 25.0, "y": 12.0},
								map[string]any{"x": 15.0, "y": 8.0},
								map[string]any{"x": 35.0, "y": 20.0},
								map[string]any{"x": 20.0, "y": 15.0},
							},
						},
					},
				},
				Style: svggen.StyleSpec{ShowGrid: true},
			},
		},

		// Pie/donut (categories + values format)
		{
			"pie_chart",
			svggen.RequestEnvelope{
				Type:  "pie_chart",
				Title: "Market Share",
				Data: map[string]any{
					"categories": []any{"Product A", "Product B", "Product C", "Other"},
					"values":     []any{40.0, 25.0, 20.0, 15.0},
				},
				Style: svggen.StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},
		{
			"donut_chart",
			svggen.RequestEnvelope{
				Type:  "donut_chart",
				Title: "Budget Allocation",
				Data: map[string]any{
					"categories": []any{"R&D", "Marketing", "Operations", "Sales"},
					"values":     []any{35.0, 25.0, 25.0, 15.0},
				},
				Style: svggen.StyleSpec{ShowLegend: true, ShowValues: true},
			},
		},

		// Waterfall (points format)
		{
			"waterfall",
			svggen.RequestEnvelope{
				Type:  "waterfall",
				Title: "Revenue Bridge",
				Data: map[string]any{
					"points": []any{
						map[string]any{"label": "FY23 Start", "value": 1000, "type": "increase"},
						map[string]any{"label": "New Customers", "value": 250, "type": "increase"},
						map[string]any{"label": "Churn", "value": -120, "type": "decrease"},
						map[string]any{"label": "Expansion", "value": 180, "type": "increase"},
						map[string]any{"label": "FY24 End", "value": 1310, "type": "total"},
					},
				},
				Style: svggen.StyleSpec{ShowValues: true, ShowGrid: true},
			},
		},

		// Specialty chart types
		{
			"funnel_chart",
			svggen.RequestEnvelope{
				Type:  "funnel_chart",
				Title: "Sales Funnel",
				Data: map[string]any{
					"categories": []any{"Leads", "Qualified", "Proposal", "Negotiation", "Closed"},
					"values":     []any{1000.0, 600.0, 400.0, 200.0, 100.0},
				},
			},
		},
		{
			"gauge_chart",
			svggen.RequestEnvelope{
				Type:  "gauge_chart",
				Title: "NPS Score",
				Data: map[string]any{
					"value":     72.0,
					"min_value": 0.0,
					"max_value": 100.0,
				},
			},
		},

		// Diagram types
		{
			"timeline",
			svggen.RequestEnvelope{
				Type:  "timeline",
				Title: "Project Milestones",
				Data: map[string]any{
					"activities": []any{
						map[string]any{"label": "Planning", "start_date": "2026-01-01", "end_date": "2026-03-01"},
						map[string]any{"label": "Development", "start_date": "2026-02-15", "end_date": "2026-06-01"},
						map[string]any{"label": "Testing", "start_date": "2026-05-01", "end_date": "2026-08-01"},
						map[string]any{"label": "Launch", "start_date": "2026-07-15", "end_date": "2026-09-01"},
					},
					"milestones": []any{
						map[string]any{"label": "Kickoff", "date": "2026-01-01"},
						map[string]any{"label": "Beta", "date": "2026-06-01"},
						map[string]any{"label": "GA", "date": "2026-09-01"},
					},
				},
			},
		},
		{
			"matrix_2x2",
			svggen.RequestEnvelope{
				Type:  "matrix_2x2",
				Title: "Priority Matrix",
				Data: map[string]any{
					"x_axis": "Effort",
					"y_axis": "Impact",
					"quadrants": []any{
						map[string]any{"label": "Quick Wins", "items": []any{"Automate reports", "Fix dashboards"}},
						map[string]any{"label": "Major Projects", "items": []any{"New platform", "Rewrite backend"}},
						map[string]any{"label": "Fill-ins", "items": []any{"Update docs", "Clean up tests"}},
						map[string]any{"label": "Thankless Tasks", "items": []any{"Legacy migration"}},
					},
				},
			},
		},
		{
			"swot",
			svggen.RequestEnvelope{
				Type:  "swot",
				Title: "SWOT Analysis",
				Data: map[string]any{
					"strengths":     []any{"Strong brand recognition", "Loyal customer base", "Efficient supply chain"},
					"weaknesses":    []any{"Limited market share", "High operating costs"},
					"opportunities": []any{"Emerging markets", "New technologies", "Strategic partnerships"},
					"threats":       []any{"Intense competition", "Regulatory changes"},
				},
			},
		},
		{
			"pestel",
			svggen.RequestEnvelope{
				Type:  "pestel",
				Title: "Market Entry Analysis",
				Data: map[string]any{
					"segments": []any{
						map[string]any{"name": "Political", "items": []any{"Stable government", "Trade agreements"}},
						map[string]any{"name": "Economic", "items": []any{"GDP growth 3.5%", "Low inflation"}},
						map[string]any{"name": "Social", "items": []any{"Aging population", "Digital natives"}},
						map[string]any{"name": "Technological", "items": []any{"5G rollout", "AI adoption"}},
						map[string]any{"name": "Environmental", "items": []any{"Carbon targets", "Recycling"}},
						map[string]any{"name": "Legal", "items": []any{"GDPR compliance", "Patent protection"}},
					},
				},
			},
		},
		{
			"fishbone",
			svggen.RequestEnvelope{
				Type:  "fishbone",
				Title: "Root Cause Analysis",
				Data: map[string]any{
					"effect": "Low Productivity",
					"categories": []any{
						map[string]any{"name": "People", "causes": []any{"Skill gaps", "Low morale"}},
						map[string]any{"name": "Process", "causes": []any{"Bottlenecks", "Rework"}},
						map[string]any{"name": "Technology", "causes": []any{"Outdated tools", "Downtime"}},
						map[string]any{"name": "Environment", "causes": []any{"Noise", "Poor layout"}},
					},
				},
			},
		},
		{
			"heatmap",
			svggen.RequestEnvelope{
				Type:  "heatmap",
				Title: "Correlation Matrix",
				Data: map[string]any{
					"row_labels": []any{"Sales", "Marketing", "Support", "Engineering"},
					"col_labels": []any{"Q1", "Q2", "Q3", "Q4"},
					"values": []any{
						[]any{85.0, 72.0, 90.0, 65.0},
						[]any{70.0, 88.0, 55.0, 78.0},
						[]any{60.0, 45.0, 82.0, 91.0},
						[]any{95.0, 80.0, 68.0, 74.0},
					},
				},
			},
		},
	}

	var generated int
	for _, size := range sizes {
		for _, diag := range diagrams {
			req := diag.req
			req.Output.Width = size.width
			req.Output.Height = size.height

			doc, err := svggen.Render(&req)
			if err != nil {
				t.Logf("SKIP %s/%s: %v", size.name, diag.name, err)
				continue
			}

			svgFile := filepath.Join(outDir, fmt.Sprintf("%s_%s.svg", diag.name, size.name))
			pngFile := filepath.Join(outDir, fmt.Sprintf("%s_%s.png", diag.name, size.name))

			if err := os.WriteFile(svgFile, []byte(doc.Content), 0644); err != nil {
				t.Errorf("write %s: %v", svgFile, err)
				continue
			}

			cmd := exec.Command("rsvg-convert", "-d", "192", "-p", "192", "-o", pngFile, svgFile) //nolint:gosec // G204: test-only code; arguments are constructed from test fixtures, not user input
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("rsvg-convert %s: %v\n%s", svgFile, err, string(out))
				continue
			}

			generated++
		}
	}

	// Generate HTML gallery
	var html strings.Builder
	html.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8">
<title>SVG Font Size Inspection</title>
<style>
body { font-family: Arial, sans-serif; background: #f5f5f5; margin: 20px; }
h1 { color: #333; }
h2 { color: #666; margin-top: 40px; border-bottom: 2px solid #ddd; padding-bottom: 8px; }
.pair { display: flex; gap: 20px; margin: 16px 0; align-items: flex-start; }
.card { background: white; border: 1px solid #ddd; border-radius: 8px; padding: 12px; }
.card h3 { margin: 0 0 8px; font-size: 14px; color: #555; }
.card img { max-width: 100%; height: auto; border: 1px solid #eee; }
.full { flex: 2; }
.half { flex: 1; }
</style></head><body>
<h1>SVG Font Size Visual Inspection</h1>
<p>Rendered from svggen at PPTX placeholder dimensions via rsvg-convert at 2x DPI (192).</p>
<p>Full width = 720x420pt, Half width = 360x420pt</p>
`)

	for _, diag := range diagrams {
		html.WriteString(fmt.Sprintf("<h2>%s</h2>\n<div class=\"pair\">\n", diag.name))
		for _, size := range sizes {
			pngName := fmt.Sprintf("%s_%s.png", diag.name, size.name)
			html.WriteString(fmt.Sprintf(
				"<div class=\"card %s\"><h3>%s (%dx%d pt)</h3><img src=\"%s\"></div>\n",
				size.name, size.name, size.width, size.height, pngName,
			))
		}
		html.WriteString("</div>\n")
	}
	html.WriteString("</body></html>")
	htmlFile := filepath.Join(outDir, "gallery.html")
	os.WriteFile(htmlFile, []byte(html.String()), 0644)

	t.Logf("Generated %d PNGs in %s", generated, outDir)
	t.Logf("Gallery: file://%s", htmlFile)
}
