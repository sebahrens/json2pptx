package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

var parityTheme = []types.ThemeColor{
	{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
	{Name: "dk2", RGB: "#1B2A4A"}, {Name: "lt2", RGB: "#E8ECF1"},
	{Name: "accent1", RGB: "#2E5090"}, {Name: "accent2", RGB: "#D4463A"},
	{Name: "accent3", RGB: "#E8A838"}, {Name: "accent4", RGB: "#43A047"},
	{Name: "accent5", RGB: "#5C6BC0"}, {Name: "accent6", RGB: "#26A69A"},
}

func renderParitySVG(t *testing.T, spec *types.DiagramSpec) []byte {
	t.Helper()
	spec.Width, spec.Height = 800, 450
	res, err := RenderDiagramSpecWithMetadata(spec, parityTheme, 0, true)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return res.SVG
}

// TestRawChartMatchesPatternLabelledDefault is the go-slide-creator-oocqj
// parity test: the same bar data rendered on the raw slide_type chart path
// (chart_value, no style) comes out identical to the labelled chart the
// chart-insights-split pattern hands the renderer (style.show_values on),
// and an explicit show_values:false still opts out.
func TestRawChartMatchesPatternLabelledDefault(t *testing.T) {
	newRaw := func(style *types.ChartStyle) *types.DiagramSpec {
		cs := &types.ChartSpec{ //nolint:staticcheck // ChartSpec is the authored chart_value shape
			Type:      types.ChartBar,
			Title:     "Revenue by quarter ($M)",
			Data:      map[string]any{"Q1 2025": 12.0, "Q2 2025": 14.5, "Q3 2025": 15.2, "Q4 2025": 18.0, "Q1 2026": 21.3},
			DataOrder: []string{"Q1 2025", "Q2 2025", "Q3 2025", "Q4 2025", "Q1 2026"},
			Style:     style,
		}
		return cs.ToDiagramSpec()
	}
	raw := renderParitySVG(t, newRaw(nil))

	on := true
	pattern := newRaw(nil)
	pattern.Style = &types.DiagramStyle{ShowValues: &on}
	labelled := renderParitySVG(t, pattern)

	if !bytes.Equal(raw, labelled) {
		t.Error("raw chart_value bar chart differs from the pattern's labelled rendering of the same data")
	}
	if !strings.Contains(string(raw), ">21.3<") || strings.Contains(string(raw), "stroke:#e0e0e0") {
		t.Error("raw bar chart should be labelled with no gridlines")
	}

	off := false
	optOut := renderParitySVG(t, newRaw(&types.ChartStyle{ShowValues: &off}))
	if strings.Contains(string(optOut), ">21.3<") || !strings.Contains(string(optOut), "stroke:#e0e0e0") {
		t.Error("style.show_values:false should keep the axis and gridlines and drop the labels")
	}
}

// TestTitleSeriesHighlight covers go-slide-creator-kbzu2's default: the one
// series a slide title names is highlighted; none or two named leave the
// chart alone, and an authored highlight wins.
func TestTitleSeriesHighlight(t *testing.T) {
	spec := func(extra map[string]any) *types.DiagramSpec {
		d := map[string]any{"categories": []any{"Q1", "Q2"}, "series": []map[string]any{
			{"name": "EMEA", "values": []any{1.0, 2.0}}, {"name": "APAC", "values": []any{2.0, 1.0}},
		}}
		for k, v := range extra {
			d[k] = v
		}
		return &types.DiagramSpec{Type: "line_chart", Data: d}
	}
	slide := func(title string) SlideSpec {
		return SlideSpec{Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: title}}}
	}
	src := spec(nil)
	got := withTitleSeriesHighlight(src, slide("EMEA is the only region still growing"))
	if hl, _ := got.Data["highlight"].([]any); len(hl) != 1 || hl[0] != "EMEA" {
		t.Errorf("highlight = %v, want [EMEA]", got.Data["highlight"])
	}
	if _, mutated := src.Data["highlight"]; mutated {
		t.Error("the authored spec was mutated")
	}
	if got := withTitleSeriesHighlight(spec(nil), slide("EMEA and APAC both grew")); got.Data["highlight"] != nil {
		t.Error("two named series must not pick a highlight")
	}
	authored := spec(map[string]any{"highlight": []any{"APAC"}})
	if got := withTitleSeriesHighlight(authored, slide("EMEA grew")); got.Data["highlight"].([]any)[0] != "APAC" {
		t.Error("an authored highlight must win")
	}
}
