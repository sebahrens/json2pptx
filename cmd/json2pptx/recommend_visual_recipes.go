package main

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// recommend_visual recipes (go-slide-creator-b7qqg.24).
//
// The default "deckspec" MCP profile has no semantic kind for most chart and
// diagram types (gantt, venn, pestel, ...) and hides the raw-JSON tools and
// get_data_format_hints. A chart / diagram recommendation that carried only
// placement metadata therefore dead-ended: the agent had to leave the
// advertised workflow and guess the payload. Every chart and diagram candidate
// now carries:
//
//   - data_contract: the required/optional data keys for that type (the same
//     entry get_data_format_hints serves), so the field vocabulary is
//     discoverable without a hidden tool;
//   - next_tool_call: a render_deck_spec call whose spec is a complete,
//     schema-valid DeckSpec — one raw_json2pptx slide hosting the chart /
//     diagram in the body placeholder with sample data that renders.
//
// TestRecommendVisualRecipesRenderForEveryType renders the recipe of every
// ready chart and diagram type, so a new svggen type without a recipe (or a
// recipe the renderer rejects) fails the build rather than reaching an agent.

// visualRecipeData is sample data, per chart / diagram type, that renders.
// Keep each payload small: it travels inside every recommend_visual response
// that ranks the type.
var visualRecipeData = map[string]map[string]any{
	// --- charts ---
	"bar": {
		"categories": []any{"Q1", "Q2", "Q3", "Q4"},
		"series":     []any{map[string]any{"name": "Revenue ($M)", "values": []any{2.8, 3.1, 3.2, 3.4}}},
	},
	// bar_ranked is the bar recipe for a ranking intent: horizontal bars,
	// sorted largest first by default.
	"bar_ranked": {
		"orientation": "horizontal",
		"categories":  []any{"Germany", "France", "United Kingdom", "Italy", "Spain"},
		"series":      []any{map[string]any{"name": "Revenue ($M)", "values": []any{42, 35, 31, 18, 12}}},
	},
	// The multi-series recipes name the series the slide is about in
	// highlight: it keeps its colour and the rest turn grey context
	// (go-slide-creator-kbzu2).
	"grouped_bar": {
		"categories": []any{"North", "South", "East"},
		"series": []any{
			map[string]any{"name": "2025", "values": []any{120, 98, 145}},
			map[string]any{"name": "2026", "values": []any{135, 112, 152}},
		},
		"highlight": []any{"2026"},
	},
	"stacked_bar": {
		"categories": []any{"Q1", "Q2", "Q3"},
		"series": []any{
			map[string]any{"name": "Product", "values": []any{8, 9, 10}},
			map[string]any{"name": "Services", "values": []any{4, 5, 5}},
		},
	},
	"line": {
		"categories": []any{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
		"series": []any{
			map[string]any{"name": "Mobile app", "values": []any{85, 92, 98, 110, 125, 138}},
			map[string]any{"name": "Web", "values": []any{120, 118, 121, 119, 117, 116}},
		},
		"highlight": []any{"Mobile app"},
	},
	// Small multiples share one y scale: the 10x region draws 10x taller.
	"small_multiples": {
		"categories": []any{"Q1", "Q2", "Q3", "Q4"},
		"series": []any{
			map[string]any{"name": "North", "values": []any{10, 20, 15, 25}},
			map[string]any{"name": "South", "values": []any{100, 200, 150, 250}},
			map[string]any{"name": "East", "values": []any{40, 45, 50, 48}},
			map[string]any{"name": "West", "values": []any{30, 28, 35, 41}},
		},
	},
	"area": {
		"categories": []any{"Jan", "Feb", "Mar", "Apr"},
		"series":     []any{map[string]any{"name": "Cumulative sign-ups (k)", "values": []any{10, 24, 41, 60}}},
	},
	"stacked_area": {
		"categories": []any{"2023", "2024", "2025", "2026"},
		"series": []any{
			map[string]any{"name": "Cloud", "values": []any{20, 30, 42, 55}},
			map[string]any{"name": "On-prem", "values": []any{40, 36, 30, 24}},
		},
	},
	"pie": {
		"categories": []any{"Enterprise", "Mid-Market", "SMB"},
		"values":     []any{52, 30, 18},
	},
	"donut": {
		"categories": []any{"Engineering", "Sales", "Operations"},
		"values":     []any{45, 35, 20},
	},
	"scatter": {
		"x_label": "Price index",
		"y_label": "Win rate (%)",
		"series": []any{map[string]any{"name": "Deals", "points": []any{
			map[string]any{"x": 90, "y": 42}, map[string]any{"x": 100, "y": 35},
			map[string]any{"x": 110, "y": 28}, map[string]any{"x": 120, "y": 21},
		}}},
	},
	"bubble": {
		"x_label": "Market growth (%)",
		"y_label": "Relative share",
		"series": []any{map[string]any{"name": "Segments", "points": []any{
			map[string]any{"x": 4, "y": 1.2, "size": 30, "label": "Retail"},
			map[string]any{"x": 9, "y": 0.6, "size": 18, "label": "Health"},
			map[string]any{"x": 12, "y": 1.8, "size": 42, "label": "Energy"},
		}}},
	},
	"radar": {
		"categories": []any{"Speed", "Quality", "Cost", "Scale", "Security"},
		"series": []any{
			map[string]any{"name": "Us", "values": []any{8, 7, 6, 9, 7}},
			map[string]any{"name": "Peer", "values": []any{6, 8, 7, 6, 8}},
		},
	},
	"waterfall": {
		"points": []any{
			map[string]any{"label": "Revenue", "value": 21.3, "type": "total"},
			map[string]any{"label": "COGS", "value": -6.7, "type": "decrease"},
			map[string]any{"label": "Gross profit", "value": 14.6, "type": "subtotal"},
			map[string]any{"label": "OpEx", "value": -10.1, "type": "decrease"},
			map[string]any{"label": "Net income", "value": 4.5, "type": "total"},
		},
	},
	"funnel": {
		"values": []any{
			map[string]any{"label": "Leads", "value": 10000},
			map[string]any{"label": "Qualified", "value": 4500},
			map[string]any{"label": "Proposals", "value": 2000},
			map[string]any{"label": "Won", "value": 350},
		},
	},
	"gauge": {"value": 72, "min": 0, "max": 100, "label": "Target attainment", "unit": "%"},
	"treemap": {
		"nodes": []any{
			map[string]any{"label": "Platform", "value": 45},
			map[string]any{"label": "SMB suite", "value": 25},
			map[string]any{"label": "API", "value": 15},
			map[string]any{"label": "Services", "value": 15},
		},
	},

	// --- diagrams ---
	"timeline": {
		"events": []any{
			map[string]any{"label": "Discovery", "start_date": "2026-01-01", "end_date": "2026-03-31"},
			map[string]any{"label": "Build", "start_date": "2026-04-01", "end_date": "2026-08-31"},
			map[string]any{"label": "Rollout", "start_date": "2026-09-01", "end_date": "2026-12-31"},
		},
		"milestones": []any{map[string]any{"label": "Go-live", "date": "2026-09-01"}},
	},
	"process_flow": {
		"steps": []any{
			map[string]any{"id": "intake", "label": "Intake"},
			map[string]any{"id": "review", "label": "Review"},
			map[string]any{"id": "approve", "label": "Approve"},
			map[string]any{"id": "deliver", "label": "Deliver"},
		},
		"connections": []any{
			map[string]any{"from": "intake", "to": "review"},
			map[string]any{"from": "review", "to": "approve"},
			map[string]any{"from": "approve", "to": "deliver"},
		},
	},
	"pyramid": {
		"levels": []any{
			map[string]any{"label": "Vision"},
			map[string]any{"label": "Strategy"},
			map[string]any{"label": "Operations"},
		},
	},
	"venn": {
		"circles": []any{
			map[string]any{"label": "Customer need", "items": []any{"Fast setup"}},
			map[string]any{"label": "Our strengths", "items": []any{"Automation"}},
		},
		"intersections": map[string]any{"ab": map[string]any{"label": "Sweet spot"}},
	},
	"swot": {
		"strengths":     []any{"Loyal customer base", "Strong brand", "Efficient supply chain"},
		"weaknesses":    []any{"High operating costs", "Aging IT estate", "Thin Asia presence"},
		"opportunities": []any{"Emerging markets", "AI-driven automation", "Strategic partnerships"},
		"threats":       []any{"New low-cost entrants", "Regulatory change", "Input-cost inflation"},
	},
	"org_chart": {
		"root": map[string]any{"name": "Dana Lee", "title": "CEO", "children": []any{
			map[string]any{"name": "Sam Patel", "title": "CFO"},
			map[string]any{"name": "Ava Chen", "title": "CTO"},
		}},
	},
	"gantt": {
		"tasks": []any{
			map[string]any{"id": "t1", "label": "Requirements", "start_date": "2026-01-05", "end_date": "2026-02-13"},
			map[string]any{"id": "t2", "label": "Build", "start_date": "2026-02-02", "end_date": "2026-04-24"},
			map[string]any{"id": "t3", "label": "Testing", "start_date": "2026-04-06", "end_date": "2026-05-29"},
		},
		"milestones": []any{map[string]any{"id": "m1", "label": "Go-live", "date": "2026-06-01"}},
	},
	// Plotted points are what only the diagram does (the matrix-2x2 pattern
	// holds text per quadrant), so the recipe plots them: x / y on 0–100,
	// the quadrant split at 50 (go-slide-creator-bdvhj).
	"matrix_2x2": {
		"x_axis_label":    "Effort",
		"y_axis_label":    "Impact",
		"quadrant_labels": []any{"Quick wins", "Major projects", "Fill-ins", "Time sinks"},
		"points": []any{
			map[string]any{"label": "Self-serve export", "x": 20, "y": 80},
			map[string]any{"label": "Usage alerts", "x": 35, "y": 65},
			map[string]any{"label": "AI assistant", "x": 80, "y": 85},
			map[string]any{"label": "Emoji support", "x": 15, "y": 20},
			map[string]any{"label": "Legacy import", "x": 75, "y": 25},
		},
	},
	"porters_five_forces": {
		"industry_name": "Enterprise cloud",
		"forces": []any{
			map[string]any{"type": "rivalry", "intensity": 0.9},
			map[string]any{"type": "new_entrants", "intensity": 0.3},
			map[string]any{"type": "substitutes", "intensity": 0.4},
			map[string]any{"type": "suppliers", "intensity": 0.5},
			map[string]any{"type": "buyers", "intensity": 0.7},
		},
	},
	"house_diagram": {
		"roof": "Leader in digital payments",
		"sections": []any{
			map[string]any{"label": "Technology", "items": []any{"Cloud platform", "Open APIs", "Security"}},
			map[string]any{"label": "Product", "items": []any{"Mobile wallet", "Merchant tools", "Analytics"}},
			map[string]any{"label": "People", "items": []any{"Talent", "Culture", "Leadership"}},
		},
		"floors": []any{
			"Shared data platform",
			map[string]any{"sections": []any{"Risk", "Controls", "Partners"}},
		},
		"foundation": "Trust and compliance",
	},
	"business_model_canvas": {
		"key_partners":       []any{"Cloud providers"},
		"key_activities":     []any{"Platform development"},
		"key_resources":      []any{"Engineering team"},
		"value_propositions": []any{"No-code pipelines"},
		"customer_relations": []any{"Self-service"},
		"channels":           []any{"Direct sales"},
		"customer_segments":  []any{"Mid-market IT"},
		"cost_structure":     []any{"Hosting"},
		"revenue_streams":    []any{"Subscriptions"},
	},
	"value_chain": {
		"primary": []any{
			map[string]any{"label": "Inbound"},
			map[string]any{"label": "Operations"},
			map[string]any{"label": "Outbound"},
			map[string]any{"label": "Sales"},
			map[string]any{"label": "Service"},
		},
		"support": []any{
			map[string]any{"label": "Technology"},
			map[string]any{"label": "HR"},
		},
	},
	"nine_box_talent": {
		"employees": []any{
			map[string]any{"name": "Sarah Chen", "performance": "high", "potential": "high"},
			map[string]any{"name": "James Kim", "performance": "medium", "potential": "high"},
			map[string]any{"name": "Ana Silva", "performance": "medium", "potential": "medium"},
		},
	},
	"kpi_dashboard": {
		"metrics": []any{
			map[string]any{"label": "Revenue", "value": "$4.2M", "change": "+12%"},
			map[string]any{"label": "NPS", "value": "62", "change": "+5"},
			map[string]any{"label": "Churn", "value": "3.1%", "change": "-0.4pt"},
			map[string]any{"label": "Gross margin", "value": "68%", "change": "+2pt"},
		},
	},
	"heatmap": {
		"row_labels": []any{"North", "South", "East"},
		"col_labels": []any{"Q1", "Q2", "Q3"},
		"values":     []any{[]any{3, 5, 8}, []any{2, 4, 6}, []any{7, 6, 9}},
	},
	"fishbone": {
		"effect": "Late deliveries",
		"categories": []any{
			map[string]any{"name": "People", "causes": []any{"Understaffed shifts"}},
			map[string]any{"name": "Process", "causes": []any{"Manual hand-offs"}},
			map[string]any{"name": "Systems", "causes": []any{"Legacy WMS"}},
		},
	},
	"pestel": {
		"political":     []any{"Trade policy shifts", "Public-sector digital mandates"},
		"economic":      []any{"Rising interest rates", "Tighter IT budgets"},
		"social":        []any{"Remote-work adoption", "Demand for self-service"},
		"technological": []any{"Generative AI", "Cloud-native platforms"},
		"environmental": []any{"Net-zero targets", "Data-centre energy caps"},
		"legal":         []any{"Data-privacy rules", "AI Act compliance"},
	},
	"panel_layout": {
		"layout": "columns",
		"panels": []any{
			map[string]any{"title": "Speed", "body": "Ship weekly"},
			map[string]any{"title": "Quality", "body": "Zero P1 defects"},
			map[string]any{"title": "Scale", "body": "10x volume"},
		},
	},
	"icon_columns": {
		"panels": []any{
			map[string]any{"title": "Speed", "body": "Ship weekly"},
			map[string]any{"title": "Quality", "body": "Zero P1 defects"},
			map[string]any{"title": "Scale", "body": "10x volume"},
		},
	},
	"icon_rows": {
		"panels": []any{
			map[string]any{"title": "Speed", "body": "Ship weekly"},
			map[string]any{"title": "Quality", "body": "Zero P1 defects"},
			map[string]any{"title": "Scale", "body": "10x volume"},
		},
	},
	"stat_cards": {
		"panels": []any{
			map[string]any{"title": "Revenue", "value": "$4.2M"},
			map[string]any{"title": "Customers", "value": "1,240"},
			map[string]any{"title": "NPS", "value": "62"},
		},
	},
}

// visualRecipeTemplate is the template a recipe names when recommend_visual was
// called without one; the spec renders on every shipped template.
const visualRecipeTemplate = "midnight-blue"

// visualRecipeDeckSpecFrom returns a complete DeckSpec rendering one chart or
// diagram candidate through the raw_json2pptx escape hatch, with the sample
// data of recipe dataKey (the type name, or a variant such as "bar_ranked"),
// or nil when the candidate is not a chart / diagram or has no recipe.
// valueKey is the typed content field (chart_value / diagram_value) that
// carries the data.
func visualRecipeDeckSpecFrom(category patterns.VisualCategory, name, dataKey, templateName string) (spec map[string]any, valueKey string) {
	var itemType string
	switch category {
	case patterns.VisualCategoryChart:
		itemType, valueKey = "chart", "chart_value"
	case patterns.VisualCategoryDiagram:
		itemType, valueKey = "diagram", "diagram_value"
	default:
		return nil, ""
	}
	data, ok := visualRecipeData[dataKey]
	if !ok {
		return nil, ""
	}
	if templateName == "" {
		templateName = visualRecipeTemplate
	}
	label := strings.ReplaceAll(name, "_", " ")
	slide := map[string]any{
		"slide_type": "content",
		"content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Replace with the action title this " + label + " supports"},
			map[string]any{"placeholder_id": "body", "type": itemType, valueKey: map[string]any{
				"type": name,
				"data": deepCopyAny(data),
				"alt":  "Replace with one sentence saying what this " + label + " shows",
			}},
		},
	}
	if category == patterns.VisualCategoryChart {
		slide["source"] = "Illustrative sample data; replace with the real source"
	}
	return map[string]any{
		"meta":   map[string]any{"title": "Recommended " + label, "template": templateName},
		"slides": []any{map[string]any{"kind": "raw_json2pptx", "slide": slide}},
	}, valueKey
}

// deepCopyAny copies a JSON-shaped value so a response never aliases the
// shared recipe table.
func deepCopyAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = deepCopyAny(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = deepCopyAny(e)
		}
		return out
	default:
		return v
	}
}

// rankingIntentWords mark an intent that ranks items by size.
var rankingIntentWords = []string{"rank", "ranking", "ranked", "largest", "biggest", "smallest", "highest", "lowest", "top", "leaders", "leading"}

// rankingIntent reports whether the recommend_visual query asks for a ranking.
func rankingIntent(query string) bool {
	for _, w := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		for _, k := range rankingIntentWords {
			if w == k {
				return true
			}
		}
	}
	return false
}

// attachVisualRecipes gives every candidate its data contract and a runnable
// render_deck_spec next call — in its DeckSpec kind when one compiles to it,
// else as a raw slide — and every compose candidate its region contracts and
// runnable compose recipe (go-slide-creator-okg00).
func attachVisualRecipes(rec *patterns.RecommendVisualResult, templateName string) {
	hints := buildDataFormatHints()
	ranking := rankingIntent(rec.QueryUnderstood)
	callouts := calloutIntent(rec.QueryUnderstood)
	reg := patterns.Default()
	for i := range rec.Candidates {
		c := &rec.Candidates[i]
		if c.Category == patterns.VisualCategoryCompose {
			attachComposeRecipe(c, templateName, reg, hints)
			continue
		}
		// Every other candidate: the DeckSpec kind that compiles to it, or a
		// raw slide with the contract inline (go-slide-creator-x97m6, -3ujfq).
		attachCandidateRecipe(c, templateName, reg, hints, ranking)
		// "screenshot with callouts": the image_case recipe carries them
		// (go-slide-creator-n3j96).
		if callouts {
			attachCalloutRecipe(c)
		}
	}
}
