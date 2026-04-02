// Package testrand — visual_deck.go generates systematic (non-random) JSON decks
// covering ALL visual element types for comprehensive visual stress testing.
// Unlike generator.go (random fuzz), this produces a deterministic deck with
// exactly one instance of every content type, chart type, and diagram type.
package testrand

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VisualDeckGenerator produces a comprehensive, deterministic test deck
// covering every visual element type supported by json2pptx.
type VisualDeckGenerator struct {
	template string
}

// NewVisualDeckGenerator creates a generator targeting a specific template.
func NewVisualDeckGenerator(template string) *VisualDeckGenerator {
	return &VisualDeckGenerator{template: template}
}

// Generate produces the comprehensive PresentationInput.
func (v *VisualDeckGenerator) Generate() *PresentationInput {
	p := &PresentationInput{
		Template:       v.template,
		OutputFilename: fmt.Sprintf("visual_stress_%s.pptx", v.template),
		Footer: &Footer{
			Enabled:  true,
			LeftText: "Visual Stress Test — Confidential",
		},
		Slides: make([]SlideInput, 0, 80),
	}

	// 1. Title slide
	p.Slides = append(p.Slides, v.titleSlide())

	// 2. Section divider
	p.Slides = append(p.Slides, v.sectionSlide("Content Type Coverage"))

	// 3-5. Body text variations
	p.Slides = append(p.Slides, v.bodyTextSlide())
	p.Slides = append(p.Slides, v.bulletsSlide(3, "Key Priorities"))
	p.Slides = append(p.Slides, v.bulletsSlide(6, "Comprehensive Analysis"))
	p.Slides = append(p.Slides, v.bulletsSlide(8, "Detailed Findings"))

	// 6-7. Body and bullets
	p.Slides = append(p.Slides, v.bodyAndBulletsSlide())

	// 8-9. Bullet groups
	p.Slides = append(p.Slides, v.bulletGroupsSlide(2))
	p.Slides = append(p.Slides, v.bulletGroupsSlide(4))

	// 10. Section divider for tables
	p.Slides = append(p.Slides, v.sectionSlide("Table Coverage"))

	// 11-14. Tables
	p.Slides = append(p.Slides, v.tableSlide(2, 2, "Compact Summary", "accent1", "all", false))
	p.Slides = append(p.Slides, v.tableSlide(4, 6, "Financial Overview", "accent2", "horizontal", true))
	p.Slides = append(p.Slides, v.tableSlide(3, 4, "Regional Analysis", "accent3", "outer", false))
	p.Slides = append(p.Slides, v.tableSlide(5, 3, "Metric Dashboard", "none", "none", true))

	// 15. Section divider for charts
	p.Slides = append(p.Slides, v.sectionSlide("Chart Type Coverage"))

	// 16-30. Every chart type
	for _, ct := range chartTypes {
		p.Slides = append(p.Slides, v.chartSlide(ct))
	}

	// Section divider for diagrams
	p.Slides = append(p.Slides, v.sectionSlide("Diagram Type Coverage"))

	// Every diagram type
	for _, dt := range diagramTypes {
		p.Slides = append(p.Slides, v.diagramSlide(dt))
	}

	// Additional SVG-only diagram types not in the random generator's list
	// but registered in svggen/init.go
	extraDiagramTypes := []string{"panel_layout"}
	for _, dt := range extraDiagramTypes {
		p.Slides = append(p.Slides, v.diagramSlide(dt))
	}

	// Section divider for two-column layouts
	p.Slides = append(p.Slides, v.sectionSlide("Two-Column Layout Coverage"))

	// Two-column combinations
	p.Slides = append(p.Slides, v.twoColumnSlide("Bullets + Chart", "bullets", "chart"))
	p.Slides = append(p.Slides, v.twoColumnSlide("Chart + Bullets", "chart", "bullets"))
	p.Slides = append(p.Slides, v.twoColumnSlide("Diagram + Table", "diagram", "table"))
	p.Slides = append(p.Slides, v.twoColumnSlide("Table + Diagram", "table", "diagram"))
	p.Slides = append(p.Slides, v.twoColumnSlide("Bullets + Bullets", "bullets", "bullets"))
	p.Slides = append(p.Slides, v.twoColumnSlide("BAB + Chart", "bab", "chart"))

	// Section divider for edge cases
	p.Slides = append(p.Slides, v.sectionSlide("Edge Case Coverage"))

	// Edge cases
	p.Slides = append(p.Slides, v.edgeLongTitle())
	p.Slides = append(p.Slides, v.edgeUnicode())
	p.Slides = append(p.Slides, v.edgeSpecialChars())
	p.Slides = append(p.Slides, v.edgeManyBullets())

	return p
}

// GenerateJSON produces the JSON bytes for the visual deck.
func (v *VisualDeckGenerator) GenerateJSON() ([]byte, error) {
	return json.MarshalIndent(v.Generate(), "", "  ")
}

// --- Slide builders ---

func (v *VisualDeckGenerator) titleSlide() SlideInput {
	title := "Comprehensive Visual Stress Test"
	subtitle := fmt.Sprintf("Template: %s — All Content Types", v.template)
	return SlideInput{
		SlideType: "title",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "subtitle", Type: "text", TextValue: &subtitle},
		},
	}
}

func (v *VisualDeckGenerator) sectionSlide(title string) SlideInput {
	return SlideInput{
		SlideType: "section",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
		},
	}
}

func (v *VisualDeckGenerator) bodyTextSlide() SlideInput {
	title := "Executive Summary"
	body := "The organization achieved **15% revenue growth** in Q4, driven by expansion in APAC and EMEA markets. Key operational metrics exceeded targets across all business units, with particular strength in digital transformation initiatives. The strategic portfolio review identified three priority areas for FY26 investment."
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "text", TextValue: &body},
		},
	}
}

func (v *VisualDeckGenerator) bulletsSlide(count int, title string) SlideInput {
	allBullets := []string{
		"Revenue increased by 23% year-over-year to $4.2B",
		"EBITDA margin expanded 340bps to 28.5%",
		"Customer acquisition cost reduced by 18%",
		"Net Promoter Score improved from 42 to 67",
		"Employee engagement reached all-time high of 84%",
		"Market share grew from 12% to 15.3% in core segments",
		"Digital channel revenue now represents 45% of total",
		"Operating cash flow conversion improved to 92%",
	}
	bullets := allBullets[:count]
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets},
		},
	}
}

func (v *VisualDeckGenerator) bodyAndBulletsSlide() SlideInput {
	title := "Strategic Priorities for FY26"
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "body_and_bullets", BABValue: &BABInput{
				Body: "Three strategic priorities have been identified based on the portfolio review and market analysis:",
				Bullets: []string{
					"**Digital Transformation** — Accelerate cloud migration and AI adoption across all BUs",
					"**Market Expansion** — Enter Southeast Asia and Latin America with localized offerings",
					"**Operational Excellence** — Achieve 30% cost reduction through process automation",
					"**Talent Development** — Build next-gen leadership pipeline with 200 high-potential candidates",
				},
				TrailingBody: "Each priority has a dedicated executive sponsor and quarterly milestone cadence.",
			}},
		},
	}
}

func (v *VisualDeckGenerator) bulletGroupsSlide(groupCount int) SlideInput {
	title := fmt.Sprintf("Workstream Update (%d Groups)", groupCount)
	allGroups := []GroupInput{
		{
			Header:  "Revenue Growth",
			Body:    "On track to exceed annual target",
			Bullets: []string{"Q4 pipeline at $1.8B", "Win rate improved to 34%", "Average deal size up 22%"},
		},
		{
			Header:  "Cost Optimization",
			Bullets: []string{"Vendor consolidation saving $12M", "Automation reducing FTE by 15%", "Real estate footprint reduced 30%"},
		},
		{
			Header:  "Innovation Pipeline",
			Body:    "12 initiatives in development",
			Bullets: []string{"AI-powered analytics platform", "Next-gen mobile app", "Blockchain supply chain"},
		},
		{
			Header:  "Risk Management",
			Bullets: []string{"Cybersecurity posture strengthened", "Regulatory compliance at 99.2%", "Business continuity plans tested"},
		},
	}
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullet_groups", BGValue: &BGInput{
				Body:   "Status across key workstreams:",
				Groups: allGroups[:groupCount],
			}},
		},
	}
}

func (v *VisualDeckGenerator) tableSlide(cols, rows int, title, headerBG, borders string, striped bool) SlideInput {
	headers := []string{"Category", "Q1", "Q2", "Q3", "Q4", "FY Total"}[:cols]
	rowData := [][]interface{}{
		{"North America", "$42M", "$45M", "$48M", "$52M", "$187M"},
		{"EMEA", "$28M", "$31M", "$33M", "$37M", "$129M"},
		{"APAC", "$15M", "$18M", "$22M", "$25M", "$80M"},
		{"LATAM", "$8M", "$9M", "$11M", "$13M", "$41M"},
		{"Middle East", "$5M", "$6M", "$7M", "$8M", "$26M"},
		{"Africa", "$3M", "$4M", "$4M", "$5M", "$16M"},
	}

	tableRows := make([][]interface{}, 0, rows)
	for i := 0; i < rows && i < len(rowData); i++ {
		tableRows = append(tableRows, rowData[i][:cols])
	}

	aligns := []string{"left", "right", "right", "right", "right", "right"}[:cols]

	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "table", TableValue: &TableInput{
				Headers:          headers,
				Rows:             tableRows,
				Style:            &TableStyle{HeaderBackground: headerBG, Borders: borders, Striped: striped},
				ColumnAlignments: aligns,
			}},
		},
	}
}

func (v *VisualDeckGenerator) chartSlide(chartType string) SlideInput {
	title := fmt.Sprintf("Chart: %s", typeDisplayName(chartType))
	return SlideInput{
		SlideType: "chart",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "chart", ChartValue: v.chartData(chartType)},
		},
	}
}

func (v *VisualDeckGenerator) chartData(chartType string) *ChartInput {
	chart := &ChartInput{
		Type:  chartType,
		Title: fmt.Sprintf("%s Performance", typeDisplayName(chartType)),
	}

	switch chartType {
	case "pie", "donut":
		chart.Data = map[string]any{
			"Enterprise": 35.0, "Mid-Market": 28.0, "SMB": 22.0, "Startup": 15.0,
		}
	case "gauge":
		chart.Data = map[string]any{"Score": 73.0}
	case "waterfall":
		chart.Data = map[string]any{
			"Starting": 100.0, "Product": 45.0, "Services": 25.0,
			"Returns": -15.0, "Discounts": -10.0, "Ending": 145.0,
		}
		chart.DataOrder = []string{"Starting", "Product", "Services", "Returns", "Discounts", "Ending"}
	case "funnel":
		chart.Data = map[string]any{
			"Leads": 10000.0, "MQL": 4500.0, "SQL": 2000.0,
			"Opportunity": 800.0, "Won": 320.0,
		}
		chart.DataOrder = []string{"Leads", "MQL", "SQL", "Opportunity", "Won"}
	case "treemap":
		chart.Data = map[string]any{
			"Technology": 35.0, "Healthcare": 25.0, "Finance": 20.0,
			"Energy": 12.0, "Consumer": 8.0,
		}
	case "scatter":
		chart.Data = map[string]any{
			"Series A": []map[string]any{
				{"x": 10.0, "y": 25.0}, {"x": 30.0, "y": 45.0},
				{"x": 50.0, "y": 35.0}, {"x": 70.0, "y": 65.0},
				{"x": 90.0, "y": 80.0},
			},
		}
	case "bubble":
		chart.Data = map[string]any{
			"Markets": []map[string]any{
				{"x": 20.0, "y": 30.0, "size": 15.0},
				{"x": 45.0, "y": 55.0, "size": 30.0},
				{"x": 70.0, "y": 40.0, "size": 25.0},
				{"x": 35.0, "y": 70.0, "size": 20.0},
			},
		}
	case "stacked_bar", "grouped_bar", "stacked_area":
		chart.Data = map[string]any{
			"Q1": []any{42.0, 28.0, 15.0},
			"Q2": []any{45.0, 31.0, 18.0},
			"Q3": []any{48.0, 33.0, 22.0},
			"Q4": []any{52.0, 37.0, 25.0},
		}
		chart.DataOrder = []string{"Q1", "Q2", "Q3", "Q4"}
		chart.SeriesLabels = []string{"Americas", "EMEA", "APAC"}
	default: // bar, line, area, radar
		chart.Data = map[string]any{
			"Q1": 42.0, "Q2": 45.0, "Q3": 48.0, "Q4": 52.0,
		}
		chart.DataOrder = []string{"Q1", "Q2", "Q3", "Q4"}
	}

	return chart
}

func (v *VisualDeckGenerator) diagramSlide(diagType string) SlideInput {
	title := fmt.Sprintf("Diagram: %s", typeDisplayName(diagType))
	return SlideInput{
		SlideType: "diagram",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "diagram", DiagramValue: &DiagramInput{
				Type:  diagType,
				Title: fmt.Sprintf("%s Overview", typeDisplayName(diagType)),
				Data:  v.diagramData(diagType),
			}},
		},
	}
}

func (v *VisualDeckGenerator) diagramData(diagType string) map[string]any {
	switch diagType {
	case "timeline":
		return map[string]any{
			"phases": []map[string]any{
				{"name": "Phase 1: Discovery", "start": "2026-01", "end": "2026-03"},
				{"name": "Phase 2: Design", "start": "2026-04", "end": "2026-06"},
				{"name": "Phase 3: Build", "start": "2026-07", "end": "2026-10"},
				{"name": "Phase 4: Launch", "start": "2026-11", "end": "2026-12"},
			},
		}
	case "process_flow":
		return map[string]any{
			"steps": []map[string]any{
				{"label": "Receive Request"},
				{"label": "Validate Input"},
				{"label": "Approved?", "type": "decision"},
				{"label": "Process Order"},
				{"label": "Deliver"},
			},
		}
	case "matrix_2x2":
		return map[string]any{
			"x_axis": "Impact",
			"y_axis": "Effort",
			"quadrants": []map[string]any{
				{"label": "Quick Wins", "items": []string{"Automate reporting", "Fix login bug"}},
				{"label": "Major Projects", "items": []string{"Platform migration", "New mobile app"}},
				{"label": "Fill Ins", "items": []string{"Update docs", "Clean backlog"}},
				{"label": "Thankless Tasks", "items": []string{"Legacy maintenance", "Compliance audit"}},
			},
		}
	case "pyramid":
		return map[string]any{
			"levels": []map[string]any{
				{"label": "Vision"},
				{"label": "Strategy"},
				{"label": "Goals"},
				{"label": "Initiatives"},
				{"label": "Tasks"},
			},
		}
	case "swot":
		return map[string]any{
			"strengths":     []string{"Strong brand recognition", "Proprietary technology", "Experienced leadership"},
			"weaknesses":    []string{"Limited global presence", "High customer acquisition cost"},
			"opportunities": []string{"Emerging markets expansion", "AI-powered products", "Strategic partnerships"},
			"threats":       []string{"Increasing competition", "Regulatory changes", "Economic uncertainty"},
		}
	case "venn":
		return map[string]any{
			"circles": []map[string]any{
				{"label": "Engineering", "items": []string{"Technical excellence", "Innovation"}},
				{"label": "Product", "items": []string{"User research", "Roadmap"}},
				{"label": "Design", "items": []string{"UX patterns", "Visual identity"}},
			},
		}
	case "org_chart":
		return map[string]any{
			"name":  "Sarah Chen",
			"title": "CEO",
			"children": []map[string]any{
				{"name": "Alex Park", "title": "CTO", "children": []map[string]any{
					{"name": "Dev Team", "title": "Engineering"},
				}},
				{"name": "Maria Lopez", "title": "CFO"},
				{"name": "James Wilson", "title": "COO"},
			},
		}
	case "gantt":
		return map[string]any{
			"tasks": []map[string]any{
				{"name": "Requirements", "start": "2026-01-01", "end": "2026-02-28", "progress": 100.0},
				{"name": "Design", "start": "2026-02-01", "end": "2026-04-30", "progress": 80.0},
				{"name": "Development", "start": "2026-03-01", "end": "2026-08-31", "progress": 45.0},
				{"name": "Testing", "start": "2026-07-01", "end": "2026-09-30", "progress": 10.0},
				{"name": "Deployment", "start": "2026-10-01", "end": "2026-11-30", "progress": 0.0},
			},
		}
	case "kpi_dashboard":
		return map[string]any{
			"kpis": []map[string]any{
				{"label": "Revenue", "value": 4200.0, "target": 4000.0},
				{"label": "EBITDA", "value": 1197.0, "target": 1100.0},
				{"label": "NPS", "value": 67.0, "target": 60.0},
				{"label": "Churn", "value": 3.2, "target": 5.0},
				{"label": "CAC", "value": 285.0, "target": 300.0},
			},
		}
	case "heatmap":
		return map[string]any{
			"row_labels": []string{"Product", "Sales", "Marketing", "Engineering"},
			"col_labels": []string{"Q1", "Q2", "Q3", "Q4"},
			"values": [][]float64{
				{85, 72, 90, 95},
				{60, 78, 82, 88},
				{70, 65, 75, 80},
				{92, 88, 95, 97},
			},
		}
	case "fishbone":
		return map[string]any{
			"effect": "Declining Customer Satisfaction",
			"categories": []map[string]any{
				{"name": "People", "causes": []string{"Training gaps", "High turnover"}},
				{"name": "Process", "causes": []string{"Slow response time", "Complex escalation"}},
				{"name": "Technology", "causes": []string{"Outdated CRM", "System downtime"}},
				{"name": "Policy", "causes": []string{"Rigid refund rules", "Limited support hours"}},
			},
		}
	case "pestel":
		return map[string]any{
			"political":     []string{"Trade policy shifts", "Regulatory changes"},
			"economic":      []string{"Interest rate increases", "Currency fluctuations"},
			"social":        []string{"Remote work trends", "Demographic shifts"},
			"technological": []string{"AI adoption", "Cloud migration"},
			"environmental": []string{"Carbon neutrality goals", "Supply chain sustainability"},
			"legal":         []string{"Data privacy regulations", "Antitrust scrutiny"},
		}
	case "business_model_canvas":
		return map[string]any{
			"key_partners":           []string{"Cloud providers", "System integrators"},
			"key_activities":         []string{"Platform development", "Customer success"},
			"key_resources":          []string{"Engineering team", "IP portfolio"},
			"value_propositions":     []string{"AI-powered automation", "Enterprise security"},
			"customer_relationships": []string{"Dedicated CSM", "Self-service portal"},
			"channels":               []string{"Direct sales", "Partner network"},
			"customer_segments":      []string{"Enterprise", "Mid-market"},
			"cost_structure":         []string{"Engineering 45%", "Sales 25%", "G&A 15%"},
			"revenue_streams":        []string{"SaaS subscriptions", "Professional services"},
		}
	case "value_chain":
		return map[string]any{
			"primary": []map[string]any{
				{"name": "Inbound Logistics", "items": []string{"Supplier management", "Inventory optimization"}},
				{"name": "Operations", "items": []string{"Manufacturing", "Quality control"}},
				{"name": "Outbound Logistics", "items": []string{"Distribution", "Order fulfillment"}},
				{"name": "Marketing & Sales", "items": []string{"Brand management", "Digital campaigns"}},
				{"name": "Service", "items": []string{"Customer support", "Warranty programs"}},
			},
			"support": []map[string]any{
				{"name": "Infrastructure", "items": []string{"Finance", "Legal", "IT"}},
				{"name": "HR", "items": []string{"Recruiting", "Training"}},
			},
		}
	case "nine_box_talent":
		return map[string]any{
			"people": []map[string]any{
				{"name": "Alice", "performance": 3.0, "potential": 3.0},
				{"name": "Bob", "performance": 2.0, "potential": 3.0},
				{"name": "Carol", "performance": 3.0, "potential": 2.0},
				{"name": "Dave", "performance": 1.0, "potential": 2.0},
				{"name": "Eve", "performance": 2.0, "potential": 1.0},
				{"name": "Frank", "performance": 1.0, "potential": 1.0},
			},
		}
	case "porters_five_forces":
		return map[string]any{
			"rivalry":        []string{"Intense competition from 5+ major players", "Price wars in commodity segments"},
			"new_entrants":   []string{"Low barrier in SaaS", "High capital requirements in hardware"},
			"substitutes":    []string{"Open-source alternatives", "In-house development"},
			"buyer_power":    []string{"Large enterprise buyers with leverage", "Switching costs moderate"},
			"supplier_power": []string{"Cloud provider concentration", "Talent market tight"},
		}
	case "house_diagram":
		return map[string]any{
			"roof":       "Customer Value",
			"pillars":    []string{"Innovation", "Quality", "Efficiency", "Service"},
			"foundation": "Operational Excellence",
		}
	case "funnel_chart":
		return map[string]any{
			"values": []map[string]any{
				{"label": "Awareness", "value": 50000.0},
				{"label": "Interest", "value": 25000.0},
				{"label": "Consideration", "value": 10000.0},
				{"label": "Intent", "value": 4000.0},
				{"label": "Purchase", "value": 1500.0},
			},
		}
	case "panel_layout":
		return map[string]any{
			"panels": []map[string]any{
				{"icon": "📊", "title": "Revenue", "value": "$4.2B", "subtitle": "+15% YoY"},
				{"icon": "👥", "title": "Customers", "value": "12,500", "subtitle": "+22% YoY"},
				{"icon": "🎯", "title": "NPS", "value": "67", "subtitle": "+25 pts"},
				{"icon": "📈", "title": "Growth", "value": "23%", "subtitle": "vs 18% target"},
			},
		}
	default:
		return map[string]any{
			"items": []string{"Item 1: Strategic initiative", "Item 2: Operational goal", "Item 3: Key metric"},
		}
	}
}

func (v *VisualDeckGenerator) twoColumnSlide(title, leftType, rightType string) SlideInput {
	s := SlideInput{
		SlideType: "two-column",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			v.slotContent("slot1", leftType),
			v.slotContent("slot2", rightType),
		},
	}
	return s
}

func (v *VisualDeckGenerator) slotContent(pid, contentType string) ContentInput {
	switch contentType {
	case "bullets":
		bullets := []string{
			"Revenue grew 15% across all segments",
			"Market share increased to 23%",
			"Customer retention at 94%",
			"Operating margin expanded 200bps",
		}
		return ContentInput{PlaceholderID: pid, Type: "bullets", BulletsValue: &bullets}
	case "chart":
		return ContentInput{
			PlaceholderID: pid,
			Type:          "chart",
			ChartValue: &ChartInput{
				Type:  "bar",
				Title: "Quarterly Revenue",
				Data:  map[string]any{"Q1": 42.0, "Q2": 45.0, "Q3": 48.0, "Q4": 52.0},
			},
		}
	case "diagram":
		return ContentInput{
			PlaceholderID: pid,
			Type:          "diagram",
			DiagramValue: &DiagramInput{
				Type:  "process_flow",
				Title: "Approval Flow",
				Data: map[string]any{
					"steps": []map[string]any{
						{"label": "Submit"},
						{"label": "Review"},
						{"label": "Approve?", "type": "decision"},
						{"label": "Complete"},
					},
				},
			},
		}
	case "table":
		return ContentInput{
			PlaceholderID: pid,
			Type:          "table",
			TableValue: &TableInput{
				Headers: []string{"Metric", "Value"},
				Rows:    [][]interface{}{{"Revenue", "$4.2B"}, {"EBITDA", "$1.2B"}, {"Margin", "28.5%"}},
				Style:   &TableStyle{HeaderBackground: "accent1", Borders: "all"},
			},
		}
	case "bab":
		return ContentInput{
			PlaceholderID: pid,
			Type:          "body_and_bullets",
			BABValue: &BABInput{
				Body:    "Key takeaways from the analysis:",
				Bullets: []string{"Strong organic growth", "Margin expansion", "Diversified revenue"},
			},
		}
	default:
		body := "Default content for slot"
		return ContentInput{PlaceholderID: pid, Type: "text", TextValue: &body}
	}
}

// typeDisplayName converts a snake_case type identifier to a Title Case display name.
// e.g. "stacked_bar" → "Stacked Bar", "process_flow" → "Process Flow".
func typeDisplayName(name string) string {
	words := strings.Split(strings.ReplaceAll(name, "-", "_"), "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// --- Edge case slides ---

func (v *VisualDeckGenerator) edgeLongTitle() SlideInput {
	title := "This Is an Extremely Long Title That Tests Whether the Generator Can Handle Titles That Exceed the Normal Expected Length for a Presentation Slide Header"
	body := "Testing long title wrapping and truncation behavior."
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "text", TextValue: &body},
		},
	}
}

func (v *VisualDeckGenerator) edgeUnicode() SlideInput {
	title := "Unicode & International Text"
	bullets := []string{
		"日本語: 四半期業績レビュー",
		"中文: 季度绩效回顾",
		"한국어: 분기별 실적 검토",
		"العربية: مراجعة الأداء الربع سنوي",
		"Ελληνικά: Τριμηνιαία ανασκόπηση",
		"Emoji: 📊 Revenue 📈 Growth 🎯 Target ✅ Achieved",
	}
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets},
		},
	}
}

func (v *VisualDeckGenerator) edgeSpecialChars() SlideInput {
	title := `Special Characters & Escaping`
	bullets := []string{
		`Ampersand: AT&T, M&A, R&D`,
		`Quotes: "Hello" and 'World'`,
		`Angle brackets: <script> and </div>`,
		`Percentages: 15%, 23.5%, -4.2%`,
		`Currency: $4.2B, €3.1B, ¥450B, £2.8B`,
		`Math: 2×3=6, ½, ¾, ±5%, ≥100`,
	}
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets},
		},
	}
}

func (v *VisualDeckGenerator) edgeManyBullets() SlideInput {
	title := "Stress Test: Dense Bullet List"
	bullets := make([]string, 12)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("Initiative %d: Strategic workstream delivering measurable impact across the organization with KPI tracking", i+1)
	}
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets},
		},
	}
}
