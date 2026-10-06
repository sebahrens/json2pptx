package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestChartTitleRepeatsSlideTitle(t *testing.T) {
	for _, c := range []struct {
		chart, slide string
		want         bool
	}{
		{"Revenue by region ($M)", "Revenue by region ($M)", true},
		{"revenue by  region", "Revenue by Region", true},
		{"Revenue by region ($M)", "stacked_bar: Revenue by region ($M)", true},
		{"Revenue by region", "Revenue by region: EMEA leads the year", true},
		// The chart title carries a unit or period the slide title lacks.
		{"Revenue by region ($M)", "Revenue by region", false},
		{"Revenue by region, FY24", "Revenue by region", false},
		{"Revenue ($M)", "Revenue (M)", false},
		// A different statement, or no title at all.
		{"Quarterly revenue", "EMEA is the only region still growing", false},
		{"", "Revenue by region", false},
		{"Revenue by region", "", false},
		// Words must be adjacent and in order.
		{"Revenue region", "Revenue by region", false},
	} {
		if got := chartTitleRepeatsSlideTitle(c.chart, c.slide); got != c.want {
			t.Errorf("chartTitleRepeatsSlideTitle(%q, %q) = %v, want %v", c.chart, c.slide, got, c.want)
		}
	}
}

func TestWithoutDuplicateChartTitleKeepsSpecAndAlt(t *testing.T) {
	spec := &types.DiagramSpec{Type: "bar_chart", Title: "Revenue by region", Data: map[string]any{"A": 1.0}}
	slide := SlideSpec{Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Revenue by region"}}}
	item := withoutDuplicateChartTitleItem(ContentItem{Type: ContentDiagram, Value: spec}, slide)
	got := item.Value.(*types.DiagramSpec)
	if !got.TitleOnSlide {
		t.Fatal("a chart title equal to the slide title should be marked as carried by the slide")
	}
	if got.Title != "Revenue by region" {
		t.Errorf("Title = %q; it must stay for the alt text", got.Title)
	}
	if spec.TitleOnSlide {
		t.Error("the authored spec was mutated")
	}
	if req := diagramSpecToSVGGen(got, nil, 0, ""); req.Title != "" {
		t.Errorf("render request title = %q, want none", req.Title)
	}

	other := SlideSpec{Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "EMEA leads"}}}
	item = withoutDuplicateChartTitleItem(ContentItem{Type: ContentDiagram, Value: spec}, other)
	if item.Value.(*types.DiagramSpec).TitleOnSlide {
		t.Error("a chart title the slide title does not carry must stay")
	}
}
