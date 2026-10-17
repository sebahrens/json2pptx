package main

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"testing"
)

func TestLayoutReviewTableHeaderUnitContext(t *testing.T) {
	input := geomSlides(t, `[{"content":[{"placeholder_id":"body","type":"table","table_value":{"headers":["Revenue (EUR m)","Different USD bn"],"rows":[["1,2","1.2"],["1.3","2.5"]]}}]}]`)
	fs := collectNumberFormatFindings(input)
	if len(fs) != 1 || fs[0].Code != patterns.ErrCodeNumberFormatInconsistent {
		t.Fatalf("header unit context missing: %+v", fs)
	}
	input.Slides[0].Content[0].TableValue.Rows[1][0].Content = "1,3"
	if fs := collectNumberFormatFindings(input); len(fs) != 0 {
		t.Fatalf("different units compared: %+v", fs)
	}
}

func TestLayoutReviewMergedHeaderUnitsAreNotGuessed(t *testing.T) {
	input := geomSlides(t, `[{"content":[{"placeholder_id":"body","type":"table","table_value":{"headers":["Label","EUR m","USD m"],"rows":[[{"content":"Group","col_span":2},"1.2"],["Other","1,2","2.5"]]}}]}]`)
	if fs := collectNumberFormatFindings(input); len(fs) != 0 {
		t.Fatalf("merged cells assigned to wrong unit: %+v", fs)
	}
}

func TestLayoutReviewAutomaticGutterDirectionsAndAuthoredExemption(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		t.Run(map[bool]string{false: "columns", true: "rows"}[vertical], func(t *testing.T) {
			layouts := []types.LayoutMetadata{}
			input := &PresentationInput{}
			bulletText := []string{"Evidence"}
			for i, gap := range []int64{10, 10, 20} {
				id := []string{"a", "b", "c"}[i]
				a := types.BoundingBox{Width: 200 * 12700, Height: 200 * 12700}
				b := a
				if vertical {
					b.Y = (200 + gap) * 12700
				} else {
					b.X = (200 + gap) * 12700
				}
				layouts = append(layouts, types.LayoutMetadata{ID: id, Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderBody, Bounds: a}, {ID: "body_2", Type: types.PlaceholderBody, Bounds: b}}})
				input.Slides = append(input.Slides, SlideInput{LayoutID: id, SlideType: "content", Content: []ContentInput{{PlaceholderID: "body", Type: "bullets", BulletsValue: &bulletText}, {PlaceholderID: "body_2", Type: "bullets", BulletsValue: &bulletText}}})
			}
			fs := collectGutterFindings(input, layouts, 0, 0)
			if len(fs) != 1 || fs[0].Path != "/slides/2" {
				t.Fatalf("gutter outlier incorrect: %+v", fs)
			}
		})
	}
	input := geomSlides(t, `[{"shape_grid":{"gap":24,"columns":2,"rows":[{"cells":[{"shape":{"text":"A"}},{"shape":{"text":"B"}}]}]}}]`)
	a, b := automaticSlideGutters(input.Slides[0], nil, nil, 0, 0)
	if len(a) != 0 || len(b) != 0 {
		t.Fatal("authored gutter compared against automatic layout")
	}
}
