package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-hu58: data-format-hints advertise
// metrics: [{label, value, unit?, change?, trend?}], but parseKPIMetrics read
// only label/value/delta/trend. `unit` and `change` — the unit and the delta,
// the two things an executive KPI card exists for — never reached the slide,
// with no warning, so an agent believed it had shipped "+12.4% YoY".
func TestParseKPIMetrics_ReadsUnitAndChange(t *testing.T) {
	data := map[string]any{"metrics": []any{
		map[string]any{"label": "ARR", "value": "184.2", "unit": "EUR m", "change": "+12.4%", "trend": "up"},
		map[string]any{"label": "NRR", "value": "118", "unit": "%", "change": "+3pp", "trend": "up"},
		// "delta" stays accepted as a synonym for "change".
		map[string]any{"label": "Churn", "value": "2.1", "unit": "%", "delta": "-0.3pp", "trend": "down"},
	}}

	metrics := parseKPIMetrics(data)
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(metrics))
	}

	if metrics[0].unit != "EUR m" {
		t.Errorf("unit = %q, want %q", metrics[0].unit, "EUR m")
	}
	if metrics[0].delta != "+12.4%" {
		t.Errorf("change = %q, want %q", metrics[0].delta, "+12.4%")
	}
	if metrics[2].delta != "-0.3pp" {
		t.Errorf("delta synonym = %q, want %q", metrics[2].delta, "-0.3pp")
	}

	// The unit must be attached to the displayed value, on the correct side.
	if got, want := metrics[0].displayValue(), "184.2 EUR m"; got != want {
		t.Errorf("displayValue = %q, want %q", got, want)
	}
	if got, want := metrics[1].displayValue(), "118%"; got != want {
		t.Errorf("displayValue = %q, want %q (a symbolic unit stays tight)", got, want)
	}
}

// A metric with no unit renders exactly as authored.
func TestKPIMetric_DisplayValueWithoutUnit(t *testing.T) {
	m := kpiMetric{value: "$21M"}
	if got := m.displayValue(); got != "$21M" {
		t.Errorf("displayValue = %q, want %q", got, "$21M")
	}
	empty := kpiMetric{unit: "EUR m"}
	if got := empty.displayValue(); got != "" {
		t.Errorf("displayValue of an empty value = %q, want empty", got)
	}
}

// A currency unit leads the number rather than trailing it.
func TestFormatUnitLabel_Placement(t *testing.T) {
	cases := []struct{ value, unit, want string }{
		{"184.2", "EUR m", "184.2 EUR m"},
		{"118", "%", "118%"},
		{"210", "$m", "$210m"},
		{"14", "months", "14 months"},
		{"7", "m", "7m"},
		{"42", "", "42"},
		{"", "EUR m", ""},
	}
	for _, c := range cases {
		if got := patterns.FormatUnitLabel(c.value, c.unit); got != c.want {
			t.Errorf("FormatUnitLabel(%q, %q) = %q, want %q", c.value, c.unit, got, c.want)
		}
	}
}

// go-slide-creator-hu58 also reported 14 metrics being accepted although the
// declared capability is max_nodes 12. The excess is now dropped with a
// CONTENT_DROPPED finding naming the count, instead of silently crushing the
// cards.
func TestProcessKPIDashboard_EnforcesMaxMetrics(t *testing.T) {
	metricsData := make([]any, 14)
	for i := range metricsData {
		metricsData[i] = map[string]any{"label": "M", "value": "1", "unit": "%"}
	}

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templateSlideData[1] = &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
				NonVisualProperties: nonVisualPropertiesXML{
					ConnectionNonVisual: connectionNonVisualXML{ID: 2, Name: "body"},
					NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
				},
			}}},
		},
	}

	item := ContentItem{
		PlaceholderID: "body",
		Type:          ContentDiagram,
		Value:         &types.DiagramSpec{Type: "kpi_dashboard", Data: map[string]any{"metrics": metricsData}},
	}
	ctx.processKPIDashboardNativeShapes(1, item, 0)

	var dropped *patterns.FitFinding
	for i := range ctx.fitFindings {
		if ctx.fitFindings[i].Code == patterns.ErrCodeContentDropped {
			dropped = &ctx.fitFindings[i]
			break
		}
	}
	if dropped == nil {
		t.Fatalf("14 metrics against a max of %d must emit CONTENT_DROPPED, got %+v", kpiMaxMetrics, ctx.fitFindings)
	}
	if !strings.Contains(dropped.Message, "2 of 14") {
		t.Errorf("finding should name how many were dropped, got %q", dropped.Message)
	}

	// Exactly the capacity must render, and quietly.
	quiet := newSinglePassContext("", nil, nil, false, nil)
	quiet.templateSlideData[1] = ctx.templateSlideData[1]
	exact := ContentItem{
		PlaceholderID: "body",
		Type:          ContentDiagram,
		Value:         &types.DiagramSpec{Type: "kpi_dashboard", Data: map[string]any{"metrics": metricsData[:kpiMaxMetrics]}},
	}
	quiet.processKPIDashboardNativeShapes(1, exact, 0)
	for _, f := range quiet.fitFindings {
		if f.Code == patterns.ErrCodeContentDropped {
			t.Errorf("exactly %d metrics must not report a drop: %+v", kpiMaxMetrics, f)
		}
	}
}
