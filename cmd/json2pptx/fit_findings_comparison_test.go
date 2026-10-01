package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestComparisonSlideSteersToPattern pins go-slide-creator-gndpw: a
// slide_type "comparison" slide gets an info finding pointing at
// comparison-2col, and its bullet columns are flagged for header treatment.
func TestComparisonSlideSteersToPattern(t *testing.T) {
	title := "Acquisition wins on speed"
	left, right := []string{"Greenfield", "Full control"}, []string{"Acquisition", "Faster ramp"}
	in := &PresentationInput{Slides: []SlideInput{
		{SlideType: "comparison", Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &left},
			{PlaceholderID: "body_2", Type: "bullets", BulletsValue: &right},
		}},
		{SlideType: "two-column", Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &left},
			{PlaceholderID: "body_2", Type: "bullets", BulletsValue: &right},
		}},
	}}
	got := collectComparisonSlideFindings(in)
	if len(got) != 1 || got[0].Code != patterns.ErrCodeComparisonPreferPattern || got[0].Path != "/slides/0/slide_type" ||
		got[0].Action != "info" || got[0].Fix == nil || got[0].Fix.Params["to"] != "comparison-2col" {
		t.Fatalf("findings = %+v, want one COMPARISON_PREFER_PATTERN on slide 0", got)
	}

	items, err := convertPresentationContent(in.Slides[0].Content, 1, types.SlideTypeComparison)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Type == generator.ContentBullets && !it.ColumnHeader {
			t.Errorf("comparison bullets %q not flagged for column-header treatment", it.PlaceholderID)
		}
	}
	items, err = convertPresentationContent(in.Slides[1].Content, 2, types.SlideTypeTwoColumn)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ColumnHeader {
			t.Errorf("two-column bullets %q flagged for comparison headers", it.PlaceholderID)
		}
	}
}
