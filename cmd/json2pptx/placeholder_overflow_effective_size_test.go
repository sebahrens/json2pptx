package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestPlaceholderOverflowUsesEffectiveGeneratedFontSize(t *testing.T) {
	bullets := []string{
		"Manual handoffs delay the month-end close", "Customer growth is accelerating across regions",
		"Reporting remains fragmented between systems", "Rework slows the finance team",
		"Decisions arrive too late for effective action",
	}
	ph := types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, FontSize: 1400, FontFamily: "Arial",
		Bounds: types.BoundingBox{Width: 3000000, Height: 250000}}
	layout := &types.LayoutMetadata{ID: "content", Placeholders: []types.PlaceholderInfo{ph}}
	authorPt := 24.0
	for _, tc := range []struct {
		name    string
		author  *float64
		wantHPt int
	}{
		{"authored override", &authorPt, 2400},
		{"density-normalized template", nil, 1800},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slide := &SlideInput{Content: []ContentInput{{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets, FontSize: tc.author}}}
			got := checkPlaceholderFindings(slide, 0, layout)
			want := generator.DetectPlaceholderOverflow(generator.PlaceholderOverflowInput{
				Path: "/slides/0/content/0", Paragraphs: bullets, WidthEMU: ph.Bounds.Width,
				HeightEMU: ph.Bounds.Height, FontSizeHPt: tc.wantHPt, FontName: ph.FontFamily,
			})
			if want == nil || len(got) != 1 {
				t.Fatalf("fixture needs one overflow finding: got=%+v want=%+v", got, want)
			}
			if got[0].Code != patterns.ErrCodePlaceholderOverflow || got[0].OverflowRatio != want.OverflowRatio {
				t.Errorf("overflow measured at wrong size: got=%+v want=%+v", got[0], want)
			}
		})
	}
}

func TestEffectivePlaceholderFontSizeKeepsDisplayAndInRangeStyles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		phSize int
		count  int
		want   int
	}{
		{"in-range regular body", 2000, 5, 2000},
		{"dense regular body", 2200, 10, 1400},
		{"display body", 9600, 5, 9600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := &ContentInput{PlaceholderID: "body", Type: "bullets"}
			ph := &types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, FontSize: tc.phSize}
			if got := effectivePlaceholderFontSizeHPt(content, ph, tc.count); got != tc.want {
				t.Errorf("size = %d, want %d", got, tc.want)
			}
		})
	}
}
