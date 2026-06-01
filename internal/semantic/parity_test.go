package semantic

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// everyKindSpec is a deck exercising every registered slide kind with a valid,
// non-degrading payload, so each first-class visual kind compiles to the named
// pattern its plan advertises. It is the fixture behind the explain↔compile
// parity gate (go-slide-creator-6wss.1): explain must promise exactly the
// pattern compile emits.
func everyKindSpec() *DeckSpec {
	return &DeckSpec{
		Meta: DeckMeta{Title: "Parity Deck", Template: "midnight-blue"},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Parity Deck", "subtitle": "Sub"}},
			{Kind: KindSection, Body: map[string]any{"title": "A Section"}},
			{Kind: KindExecutiveSummary, Body: map[string]any{
				"title":    "Where We Stand",
				"takeaway": "Momentum is real but uneven.",
				"points":   []any{"Revenue up", "Churn up", "Cycle shorter"},
			}},
			{Kind: KindKPISnapshot, Body: map[string]any{
				"title":    "The Numbers",
				"takeaway": "Most metrics improved.",
				"kpis": []any{
					map[string]any{"value": "$48M", "label": "ARR"},
					map[string]any{"value": "118%", "label": "NRR"},
					map[string]any{"value": "41d", "label": "Cycle"},
					map[string]any{"value": "2.4%", "label": "Churn"},
				},
			}},
			{Kind: KindChartInsight, Body: map[string]any{
				"title":   "Revenue Trajectory",
				"insight": "EMEA launch landed early.",
				"chart": map[string]any{
					"type": "bar_chart",
					"data": map[string]any{
						"categories": []any{"Q1", "Q2"},
						"series":     []any{map[string]any{"name": "Rev", "values": []any{34, 40}}},
					},
				},
				"insights": []any{"Grew 41%", "Q4 step-up", "Pipeline strong"},
			}},
			{Kind: KindComparison, Body: map[string]any{
				"title":    "Build vs Buy",
				"takeaway": "Buy wins on time-to-value.",
				"columns": []any{
					map[string]any{"title": "Build", "items": []any{"Slower", "Flexible"}},
					map[string]any{"title": "Buy", "items": []any{"Faster", "Rigid"}},
				},
			}},
			{Kind: KindProcess, Body: map[string]any{
				"title":    "How It Works",
				"takeaway": "Four clean steps.",
				"steps": []any{
					map[string]any{"title": "Discover", "description": "Gather inputs"},
					map[string]any{"title": "Build", "description": "Produce deck"},
					map[string]any{"title": "Review", "description": "Check fit"},
				},
			}},
			{Kind: KindRoadmap, Body: map[string]any{
				"title":    "The Plan",
				"takeaway": "Three phases to GA.",
				"phases": []any{
					map[string]any{"name": "Pilot", "date_label": "Q1", "description": "Prove value"},
					map[string]any{"name": "Expand", "date_label": "Q2", "description": "Scale up"},
					map[string]any{"name": "GA", "date_label": "Q3", "description": "Launch"},
				},
			}},
			{Kind: KindDecision, Body: map[string]any{
				"title":          "The Bet",
				"takeaway":       "Fund the pod now.",
				"recommendation": "Stand up an SMB success pod in Q3.",
				"options":        []any{"Hold coverage", "Fund the pod", "Offshore support"},
			}},
			{Kind: KindClosing, Body: map[string]any{"title": "Questions?", "subtitle": "Thank you"}},
			{Kind: KindRawJSON2pptx, Body: map[string]any{
				"slide": map[string]any{
					"slide_type": "title",
					"content":    []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Raw"}},
				},
			}},
		},
	}
}

// TestExplainCompileParity is the core regression gate for go-slide-creator-6wss.1:
// for every slide kind, the pattern the explain projection advertises must be
// exactly the pattern the compiler emits. A first-class visual kind that explain
// promises (process/roadmap/comparison) but compile renders as a generic content
// slide — or vice versa — fails here.
func TestExplainCompileParity(t *testing.T) {
	spec := everyKindSpec()

	exp := ExplainSpec(spec)
	input, _, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile every-kind deck: %v", err)
	}
	if len(exp.Slides) != len(input.Slides) {
		t.Fatalf("explain has %d slides, compile emitted %d", len(exp.Slides), len(input.Slides))
	}

	for i := range exp.Slides {
		wantPattern := exp.Slides[i].Pattern
		gotPattern := ""
		if p := input.Slides[i].Pattern; p != nil {
			gotPattern = p.Name
		}
		if gotPattern != wantPattern {
			t.Errorf("slide %d (%s): explain advertises pattern %q but compile emitted %q",
				i, exp.Slides[i].Kind, wantPattern, gotPattern)
		}
	}
}

// TestExplainCompileParity_ComparisonOverCap is the explain/compile parity guard
// for go-slide-creator-wzd4: a balanced comparison whose columns exceed the
// comparison-2col row cap degrades to a content slide at compile, so explain must
// not advertise the comparison-2col pattern for it.
func TestExplainCompileParity_ComparisonOverCap(t *testing.T) {
	rows := func(n int) []any {
		items := make([]any, n)
		for i := range items {
			items[i] = "row"
		}
		return items
	}
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck", Template: "midnight-blue"},
		Slides: []SlideSpec{{Kind: KindComparison, Body: map[string]any{
			"title":    "A vs B",
			"takeaway": "Pick A.",
			"columns": []any{
				map[string]any{"header": "A", "items": rows(11)},
				map[string]any{"header": "B", "items": rows(11)},
			},
		}}},
	}

	exp := ExplainSpec(spec)
	if got := exp.Slides[0].Pattern; got != "" {
		t.Errorf("explain advertises pattern %q for an over-cap comparison; want none", got)
	}

	input, _, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile over-cap comparison: %v", err)
	}
	if p := input.Slides[0].Pattern; p != nil {
		t.Errorf("compile emitted pattern %q for an over-cap comparison; want none (content fallback)", p.Name)
	}
}

// TestExplainCompileParity_ChartInsightOverCap is the explain/compile parity
// guard for go-slide-creator-hadk: a chart_insight with more than 6 insights
// degrades to a content slide at compile (dropping the chart), so explain must
// not advertise the chart-insights-split pattern for it.
func TestExplainCompileParity_ChartInsightOverCap(t *testing.T) {
	insights := func(n int) []any {
		items := make([]any, n)
		for i := range items {
			items[i] = "insight"
		}
		return items
	}
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck", Template: "midnight-blue"},
		Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
			"title":    "Revenue",
			"takeaway": "It grew.",
			"chart": map[string]any{
				"type": "bar_chart",
				"data": map[string]any{
					"categories": []any{"Q1", "Q2"},
					"series":     []any{map[string]any{"name": "Rev", "values": []any{1, 2}}},
				},
			},
			"insights": insights(7),
		}}},
	}

	exp := ExplainSpec(spec)
	if got := exp.Slides[0].Pattern; got != "" {
		t.Errorf("explain advertises pattern %q for an over-cap chart_insight; want none", got)
	}

	input, _, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile over-cap chart_insight: %v", err)
	}
	if p := input.Slides[0].Pattern; p != nil {
		t.Errorf("compile emitted pattern %q for an over-cap chart_insight; want none (content fallback)", p.Name)
	}
}

// TestExplainCompileParity_PatternsValidate confirms every pattern the every-kind
// deck compiles to decodes and validates against the registry — so "compiles to
// the advertised visual" also means "renders", not just "names a pattern".
func TestExplainCompileParity_PatternsValidate(t *testing.T) {
	input, _, err := Compile(everyKindSpec(), CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	wantPatterns := map[string]bool{
		"kpi-4up": false, "chart-insights-split": false,
		"comparison-2col": false, "process-flow": false, "phase-roadmap": false,
	}
	for i := range input.Slides {
		p := input.Slides[i].Pattern
		if p == nil {
			continue
		}
		if _, expected := wantPatterns[p.Name]; expected {
			wantPatterns[p.Name] = true
		}
		pat, ok := patterns.Default().Get(p.Name)
		if !ok {
			t.Errorf("slide %d: emitted unknown pattern %q", i, p.Name)
			continue
		}
		values := pat.NewValues()
		if err := json.Unmarshal(p.Values, values); err != nil {
			t.Errorf("slide %d: pattern %q values do not decode: %v", i, p.Name, err)
			continue
		}
		if err := pat.Validate(values, nil, nil); err != nil {
			t.Errorf("slide %d: pattern %q failed validation: %v", i, p.Name, err)
		}
	}
	for name, seen := range wantPatterns {
		if !seen {
			t.Errorf("expected the every-kind deck to emit pattern %q, but it did not", name)
		}
	}
}
