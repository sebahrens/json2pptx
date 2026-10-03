package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestSvggenDiagramWrongDataKeys_RefusedInEveryPlacement is the
// go-slide-creator-x9s5i repro: svggen diagrams parse their data by known
// keys and skip the rest, so timeline items written {"title", "when"} lost
// their dates, venn circles written {"title"} drew unlabelled circles and a
// misspelled series key dropped the series' values — while validate called
// the deck clean. validate and generate must now refuse in a body
// placeholder, a shape_grid cell and a compose segment, naming the key the
// renderer reads.
func TestSvggenDiagramWrongDataKeys_RefusedInEveryPlacement(t *testing.T) {
	cases := []struct {
		name     string
		diagram  string
		wantKeys map[string]string // unknown key -> did-you-mean
	}{
		{
			name:     "timeline",
			diagram:  `{"type": "timeline", "data": {"items": [{"title": "Kickoff", "when": "2026-01"}, {"title": "Launch", "when": "2026-06"}]}}`,
			wantKeys: map[string]string{"when": "date"},
		},
		{
			name:     "venn",
			diagram:  `{"type": "venn", "data": {"circles": [{"title": "Speed", "label": ""}, {"title": "Cost"}]}}`,
			wantKeys: map[string]string{"title": "label"},
		},
		{
			name:     "bar_chart series",
			diagram:  `{"type": "bar_chart", "data": {"categories": ["Q1", "Q2"], "series": [{"name": "Revenue", "valeus": [1, 2]}]}}`,
			wantKeys: map[string]string{"valeus": "values"},
		},
		{
			name:     "org_chart nested",
			diagram:  `{"type": "org_chart", "data": {"root": {"name": "Ada", "children": [{"name": "Bo", "role": "CTO"}]}}}`,
			wantKeys: map[string]string{"role": "title"},
		},
	}
	for _, tc := range cases {
		for _, placement := range []string{"placeholder", "grid", "compose"} {
			t.Run(tc.name+"/"+placement, func(t *testing.T) {
				slide := hdx2lSlide(t, placement, tc.diagram)
				errs := hdx2lValidate(slide)
				found := map[string]bool{}
				for _, d := range errs {
					if d.Code != patterns.ErrCodeUnknownKey {
						continue
					}
					for key, want := range tc.wantKeys {
						if !strings.HasSuffix(d.Path, "/"+key) {
							continue
						}
						found[key] = true
						if d.Fix == nil || d.Fix.Kind != "rename_field" || d.Fix.Params["to"] != want {
							t.Errorf("%s: fix = %+v, want rename_field to %q", d.Path, d.Fix, want)
						}
						if !strings.Contains(d.Message, fmt.Sprintf("did you mean %q", want)) {
							t.Errorf("message lacks did-you-mean %q: %s", want, d.Message)
						}
					}
				}
				for key := range tc.wantKeys {
					if !found[key] {
						t.Errorf("validate did not refuse key %q; errors: %+v", key, errs)
					}
				}

				err := generateErr(slide)
				if err == nil {
					t.Fatal("generate accepted a payload whose key the renderer never reads")
				}
				for key, want := range tc.wantKeys {
					if !strings.Contains(err.Error(), fmt.Sprintf("field %q", key)) ||
						!strings.Contains(err.Error(), fmt.Sprintf("did you mean %q", want)) {
						t.Errorf("generate error does not name %q -> %q: %v", key, want, err)
					}
				}
			})
		}
	}
}

// TestSvggenDiagramCorrectDataKeys_AcceptedInEveryPlacement: the same
// diagrams with the keys the renderers read pass validate's data checks.
func TestSvggenDiagramCorrectDataKeys_AcceptedInEveryPlacement(t *testing.T) {
	diagrams := map[string]string{
		"timeline":  `{"type": "timeline", "data": {"items": [{"title": "Kickoff", "date": "2026-01"}, {"label": "Launch", "date": "2026-06"}]}}`,
		"venn":      `{"type": "venn", "data": {"circles": [{"label": "Speed"}, {"label": "Cost", "items": ["Opex"]}], "intersections": {"ab": "Value"}}}`,
		"bar_chart": `{"type": "bar_chart", "data": {"categories": ["Q1", "Q2"], "series": [{"name": "Revenue", "values": [1, 2]}]}}`,
		"org_chart": `{"type": "org_chart", "data": {"root": {"name": "Ada", "children": [{"name": "Bo", "title": "CTO"}]}}}`,
	}
	for name, diagram := range diagrams {
		for _, placement := range []string{"placeholder", "grid", "compose"} {
			t.Run(name+"/"+placement, func(t *testing.T) {
				for _, d := range hdx2lValidate(hdx2lSlide(t, placement, diagram)) {
					if d.Code == patterns.ErrCodeUnknownKey || d.Code == string(diagnostics.CodeInvalidSlide) || d.Code == string(diagnostics.CodeInvalidGrid) {
						t.Errorf("unexpected data error: %+v", d)
					}
				}
			})
		}
	}
}

// TestSvggenChartContentWrongSeriesKey_Refused: a chart content item (the
// chart_value path) is held to the same contract inside series[].
func TestSvggenChartContentWrongSeriesKey_Refused(t *testing.T) {
	var slide SlideInput
	raw := `{"layout_id": "content-slide", "content": [{"placeholder_id": "body", "type": "chart", "chart_value":
		{"type": "bubble_chart", "data": {"series": [{"name": "S", "x_values": [1, 2], "values": [3, 4], "sizes": [5, 6]}]}}}]}`
	if err := json.Unmarshal([]byte(raw), &slide); err != nil {
		t.Fatal(err)
	}
	var hit bool
	for _, d := range hdx2lValidate(slide) {
		if d.Code == patterns.ErrCodeUnknownKey && d.Path == "/slides/0/content/0/chart_value/data/series/0/sizes" {
			hit = d.Fix != nil && d.Fix.Params["to"] == "bubble_values"
		}
	}
	if !hit {
		t.Errorf("validate did not refuse series[0].sizes with did-you-mean bubble_values: %+v", hdx2lValidate(slide))
	}
	if err := generateErr(slide); err == nil || !strings.Contains(err.Error(), `"sizes"`) {
		t.Errorf("generate error = %v, want a refusal naming sizes", err)
	}
}
