package generator

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// seriesHighlightChartTypes are the multi-series charts whose series a slide
// title can single out (go-slide-creator-kbzu2).
var seriesHighlightChartTypes = map[string]bool{
	"line_chart": true, "area_chart": true, "grouped_bar_chart": true, "bar_chart": true,
	"line": true, "area": true, "grouped_bar": true, "bar": true,
}

// withTitleSeriesHighlight returns the diagram with data.highlight set to the
// one series its slide's title or takeaway names, when the author gave no
// highlight and exactly one series name appears there ("EMEA is the only
// region still growing" highlights the EMEA line). Otherwise the spec is
// returned unchanged. The caller's data map is never mutated.
func withTitleSeriesHighlight(spec *types.DiagramSpec, slide SlideSpec) *types.DiagramSpec {
	if spec == nil || !seriesHighlightChartTypes[spec.Type] {
		return spec
	}
	if _, set := spec.Data["highlight"]; set {
		return spec
	}
	names := chartSeriesNamesOf(spec.Data["series"])
	if len(names) < 2 {
		return spec
	}
	text := slideTitleText(slide)
	if slide.Takeaway != "" {
		text += " " + slide.Takeaway
	}
	idx := svggen.DefaultSeriesHighlight(text, names)
	if idx < 0 {
		return spec
	}
	data := make(map[string]any, len(spec.Data)+1)
	for k, v := range spec.Data {
		data[k] = v
	}
	data["highlight"] = []any{names[idx]}
	cp := *spec
	cp.Data = data
	return &cp
}

// withTitleSeriesHighlightItem applies withTitleSeriesHighlight to a diagram
// content item; any other item is returned unchanged.
func withTitleSeriesHighlightItem(item ContentItem, slide SlideSpec) ContentItem {
	if ds, ok := item.Value.(*types.DiagramSpec); ok {
		item.Value = withTitleSeriesHighlight(ds, slide)
	}
	return item
}

// chartSeriesNamesOf reads the series names of a categories/series payload,
// whichever slice type the payload was built with.
func chartSeriesNamesOf(raw any) []string {
	var maps []map[string]any
	switch s := raw.(type) {
	case []map[string]any:
		maps = s
	case []any:
		for _, e := range s {
			m, _ := e.(map[string]any)
			maps = append(maps, m)
		}
	}
	names := make([]string, 0, len(maps))
	for _, m := range maps {
		name, _ := m["name"].(string)
		names = append(names, name)
	}
	return names
}

// slideTitleText is the slide's title placeholder text, if any.
func slideTitleText(slide SlideSpec) string {
	for _, item := range slide.Content {
		if item.Type != ContentText || !isTitlePlaceholder(item.PlaceholderID) {
			continue
		}
		if s, ok := item.Value.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
