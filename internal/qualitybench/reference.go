package qualitybench

import (
	"encoding/json"
	"strings"
)

// ReferenceConfiguration names the deterministic, agent-free configuration
// used by the dry-run. It exercises the full benchmark harness (deck
// generation on every template, rendering, blind contact sheets, results
// JSON) without any model/provider call. Its decks are fixed per brief
// category, so ratings of it measure template/engine rendering quality, not
// agent authoring quality — never report them as agent results.
const ReferenceConfiguration = "dry-run-reference"

// ReferenceDeck returns a deterministic PresentationInput JSON for a brief:
// a title slide plus two category-appropriate pattern slides. The template
// field is set to tmpl.
func ReferenceDeck(b Brief, tmpl string) ([]byte, error) {
	title := briefTitle(b)
	slides := []map[string]any{{
		"layout_id": "title",
		"content": []map[string]any{
			{"placeholder_id": "title", "type": "text", "text_value": title},
			{"placeholder_id": "subtitle", "type": "text", "text_value": "Quality benchmark reference deck — " + b.Category},
		},
	}}
	for _, s := range categorySlides(strings.ToLower(b.Category)) {
		slides = append(slides, map[string]any{
			"layout_id": "content",
			"content":   []map[string]any{{"placeholder_id": "title", "type": "text", "text_value": s.title}},
			"pattern":   map[string]any{"name": s.pattern, "values": s.values},
		})
	}
	deck := map[string]any{
		"template":        tmpl,
		"output_filename": b.ID + ".pptx",
		"slides":          slides,
	}
	return json.MarshalIndent(deck, "", "  ")
}

type refSlide struct {
	title, pattern string
	values         any
}

func briefTitle(b Brief) string {
	t := strings.TrimSpace(b.Prompt)
	if i := strings.IndexAny(t, ".:"); i > 0 {
		t = t[:i]
	}
	if r := []rune(t); len(r) > 60 {
		cut := string(r[:57])
		if i := strings.LastIndex(cut, " "); i > 30 {
			cut = cut[:i]
		}
		t = strings.TrimRight(strings.TrimSpace(cut), ",;") + "…"
	}
	return t
}

var revenueChart = map[string]any{
	"type": "bar_chart",
	"data": map[string]any{
		"categories": []string{"FY22", "FY23", "FY24", "FY25", "FY26"},
		"series":     []map[string]any{{"name": "Revenue ($M)", "values": []float64{42, 48, 55, 63, 71}}},
	},
}

func categorySlides(category string) []refSlide {
	insights := refSlide{"Revenue has compounded at ~14% a year", "chart-insights-split", map[string]any{
		"chart":    revenueChart,
		"insights": []string{"Growth is broad-based across regions.", "Margin expanded 3 pts on mix shift.", "Pipeline supports continued double-digit growth."},
		"source":   "Source: benchmark reference data",
	}}
	switch category {
	case "kpi":
		return []refSlide{{"Quarter at a glance", "kpi-4up", []map[string]string{
			{"big": "+12%", "small": "Revenue growth"}, {"big": "38%", "small": "Gross margin"},
			{"big": "94%", "small": "Net retention"}, {"big": "$21M", "small": "Free cash flow"},
		}}, insights}
	case "comparison":
		return []refSlide{{"Options compared", "comparison-2col", map[string]any{
			"header_left": "Option A", "header_right": "Option B",
			"rows": []map[string]string{
				{"left": "Lower upfront cost", "right": "Faster implementation"},
				{"left": "Mature capability set", "right": "Stronger roadmap fit"},
				{"left": "Higher integration risk", "right": "Vendor concentration risk"},
			},
		}}, {"Recommendation rationale", "scqa-summary", scqaValues()}}
	case "process":
		return []refSlide{{"How the process runs", "numbered-step-strip", map[string]any{
			"style": "stacked-box",
			"steps": []map[string]string{
				{"label": "Discover", "body": "Confirm scope, owners and success measures"},
				{"label": "Design", "body": "Agree target process and decision gates"},
				{"label": "Build", "body": "Configure tooling and train owners"},
				{"label": "Launch", "body": "Go live with hypercare support"},
				{"label": "Improve", "body": "Review metrics monthly and iterate"},
			},
		}}, insights}
	case "chart":
		return []refSlide{insights, {"What drives the result", "scqa-summary", scqaValues()}}
	default: // narrative and anything else
		return []refSlide{{"Where we are and what we recommend", "scqa-summary", scqaValues()}, insights}
	}
}

func scqaValues() map[string]any {
	return map[string]any{
		"situation":    "The market is shifting toward integrated, data-driven offerings.",
		"complication": []string{"Legacy channels are flattening.", "Competitors are moving faster on digital."},
		"questions":    []string{"Where should we invest to keep growing?"},
		"answer":       []string{"Focus investment on the two highest-growth segments.", "Fund it by simplifying the legacy portfolio."},
	}
}
