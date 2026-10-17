package main

import (
	"encoding/json"
	"fmt"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"testing"
)

func TestLayoutReviewExplicitBlockAlignment(t *testing.T) {
	for _, anchor := range []string{"ctr", "center", "bottom", "b", "top"} {
		t.Run(anchor, func(t *testing.T) {
			raw := fmt.Sprintf(`[{"layout_id":"content","shape_grid":{"type_scale":"compact","columns":2,"rows":[{"min_height":100,"max_height":100,"cells":[{"shape":{"text":{"content":"Short label","size":14,"vertical_align":%q}}},{"shape":{"text":{"vertical_align":%q,"paragraphs":[{"content":"Long label","size":14},{"content":"Second line","size":14}]}}}]}]}}]`, anchor, anchor)
			input := geomSlides(t, raw)
			if fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeBaselineMisalign); len(fs) > 0 {
				t.Fatalf("intentional %s blocks: %+v", anchor, fs)
			}
			var text map[string]any
			cell := input.Slides[0].ShapeGrid.Rows[0].Cells[1]
			if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
				t.Fatal(err)
			}
			text["inset_top"], text["inset_bottom"] = 25, 0
			cell.Shape.Text, _ = json.Marshal(text)
			if fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeBaselineMisalign); len(fs) == 0 && anchor != "bottom" && anchor != "b" {
				t.Fatal("unequal alignment insets were ignored")
			}
		})
	}
}

func TestRiskHeatmapIndependentCenteredLists(t *testing.T) {
	values := json.RawMessage(`{"items":[{"name":"Supplier","likelihood":"high","impact":"high"},{"name":"Delay","likelihood":"high","impact":"high"},{"name":"Cost","likelihood":"medium","impact":"high"},{"name":"Policy","likelihood":"low","impact":"medium"},{"name":"Demand","likelihood":"medium","impact":"low"}]}`)
	ctx := measuredComposeContext()
	grid, _, err := expandPattern(&PatternInput{Name: "risk-heatmap", Values: values}, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	input := &PresentationInput{Slides: []SlideInput{{LayoutID: "content", ShapeGrid: grid}}}
	if fs := findingsByCode(collectGeometryFindings(input, nil, ctx.SlideWidth, ctx.SlideHeight, nil), patterns.ErrCodeBaselineMisalign); len(fs) > 0 {
		t.Fatalf("independent lists flagged: %+v", fs)
	}
}

func TestPortraitIsNotSoleEvidenceExhibit(t *testing.T) {
	input := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":[1,3],"rows":[{"cells":[{"grid":{"columns":1,"rows":[{"cells":[{"image":{"path":"portrait.png"}}]}]}},{"shape":{"text":"Expert biography"}}]}]}}]`)
	for _, source := range []string{"pattern:team-bios", "pattern:contact-directory", "pattern:image-text-split", ""} {
		input.Slides[0].ShapeGrid.Source = source
		fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeExhibitTooSmall)
		portrait := source == "pattern:team-bios" || source == "pattern:contact-directory"
		if (len(fs) == 0) != portrait {
			t.Errorf("source %s findings %+v", source, fs)
		}
	}
}
