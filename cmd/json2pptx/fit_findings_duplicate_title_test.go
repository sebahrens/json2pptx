package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func contentSlideWithTitle(title string) SlideInput {
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: strPtr(title)},
			{PlaceholderID: "body", Type: "text", TextValue: strPtr("body text that makes this a content slide")},
		},
	}
}

func TestDuplicateTitle_FlagsRepeatedTitlesOnContentSlides(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{SlideType: "title", Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Cover")},
			}},
			contentSlideWithTitle("Next Steps"),
			contentSlideWithTitle("Market Analysis"),
			contentSlideWithTitle("  next   steps "), // duplicate with whitespace + case variance
			contentSlideWithTitle("Next steps"),       // duplicate
		},
	}

	findings := collectDuplicateTitleFindings(input)
	if len(findings) != 2 {
		t.Fatalf("expected 2 DUPLICATE_TITLE findings (slides 4 and 5), got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Code != patterns.ErrCodeDuplicateTitle {
			t.Errorf("finding.code = %q, want DUPLICATE_TITLE", f.Code)
		}
		if f.Action != "review" {
			t.Errorf("finding.action = %q, want review", f.Action)
		}
		if f.Fix == nil || f.Fix.Kind != "shorten_title" {
			t.Errorf("finding.fix kind = %v, want shorten_title", f.Fix)
		}
		if f.Fix.Params["duplicate_of_slide"] != 2 {
			t.Errorf("fix.params.duplicate_of_slide = %v, want 2 (1-based)", f.Fix.Params["duplicate_of_slide"])
		}
		if f.Fix.Params["duplicate_count"] != 3 {
			t.Errorf("fix.params.duplicate_count = %v, want 3", f.Fix.Params["duplicate_count"])
		}
	}

	// First slide of the duplicate group (slide index 1, path .../1/...) must not carry the finding.
	for _, f := range findings {
		if strings.HasPrefix(f.Path, "/slides/1/") {
			t.Errorf("first occurrence (slide 2) should not be annotated, got finding at %s", f.Path)
		}
	}
}

func TestDuplicateTitle_TitleAndSectionSlidesAreExempt(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{SlideType: "title", Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Thank You")},
			}},
			{SlideType: "section", Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Strategy")},
			}},
			{SlideType: "title", Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Thank You")},
			}},
			{SlideType: "section", Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Strategy")},
			}},
		},
	}
	findings := collectDuplicateTitleFindings(input)
	if len(findings) != 0 {
		t.Errorf("title/section slides must be exempt from DUPLICATE_TITLE; got %+v", findings)
	}
}

func TestDuplicateTitle_EmptyAndWhitespaceTitlesIgnored(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			contentSlideWithTitle(""),
			contentSlideWithTitle("   "),
			contentSlideWithTitle(""),
		},
	}
	if findings := collectDuplicateTitleFindings(input); len(findings) != 0 {
		t.Errorf("empty/whitespace titles should not produce DUPLICATE_TITLE findings; got %+v", findings)
	}
}

func TestDuplicateTitle_UniqueTitlesProduceNoFindings(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			contentSlideWithTitle("Market Analysis"),
			contentSlideWithTitle("Competitive Landscape"),
			contentSlideWithTitle("Customer Segmentation"),
		},
	}
	if findings := collectDuplicateTitleFindings(input); len(findings) != 0 {
		t.Errorf("distinct titles should not produce DUPLICATE_TITLE findings; got %+v", findings)
	}
}

func TestDuplicateTitle_PatternBearingSlidesAreContentSlides(t *testing.T) {
	// A slide with only a title text run but a pattern body must still
	// participate in the duplicate-title check — inferSlideType would call
	// such a slide "title" without the shape_grid/pattern/compose escape hatch.
	patternSlide := func(title string) SlideInput {
		return SlideInput{
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr(title)},
			},
			Pattern: &PatternInput{Name: "card-grid"},
		}
	}
	input := &PresentationInput{
		Slides: []SlideInput{
			patternSlide("Next Steps"),
			patternSlide("Next Steps"),
		},
	}
	findings := collectDuplicateTitleFindings(input)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for pattern-bearing duplicate, got %d: %+v", len(findings), findings)
	}
}

func TestValidateStructure_DuplicateTitleWarning(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			contentSlideWithTitle("Pricing"),
			contentSlideWithTitle("pricing"),
		},
	}
	warnings := validateStructure(input)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "duplicating an earlier slide title") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected validateStructure to surface duplicate-title warning, got %+v", warnings)
	}
}

func TestCompositionAxis_DuplicateTitleLowersScore(t *testing.T) {
	// Baseline: 5 distinct content slides.
	baseline := []SlideInput{
		{SlideType: "title", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Cover")}}},
		contentSlideWithTitle("A"),
		contentSlideWithTitle("B"),
		contentSlideWithTitle("C"),
		contentSlideWithTitle("D"),
	}
	baseResult := compositionAxis(baseline)
	if baseResult == nil {
		t.Fatal("baseline composition result is nil")
	}

	// With two duplicates the composition score must drop and the diagnostic
	// must appear.
	dupSlides := []SlideInput{
		{SlideType: "title", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Cover")}}},
		contentSlideWithTitle("Repeat"),
		contentSlideWithTitle("B"),
		contentSlideWithTitle("Repeat"),
		contentSlideWithTitle("Repeat"),
	}
	dupResult := compositionAxis(dupSlides)
	if dupResult == nil {
		t.Fatal("duplicate composition result is nil")
	}
	if dupResult.Score >= baseResult.Score {
		t.Errorf("duplicate-title deck should score lower than baseline; baseline=%d dup=%d", baseResult.Score, dupResult.Score)
	}

	found := false
	for _, d := range dupResult.Diagnostics {
		if d.Code == "duplicate_title" {
			found = true
			if d.Severity != "warning" {
				t.Errorf("duplicate_title severity = %q, want warning", d.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate_title composition diagnostic, got %+v", dupResult.Diagnostics)
	}
}
