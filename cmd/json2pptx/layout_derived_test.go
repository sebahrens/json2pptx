package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCanonicalAsymmetricLayoutSurvivesResolution(t *testing.T) {
	layouts := []types.LayoutMetadata{{ID: "slideLayout3", Name: "Two Content", Tags: []string{"content", "two-column", "comparison"}}}
	for _, tt := range []struct {
		name        string
		layoutID    string
		wantPercent int
		wantFinding bool
	}{
		{"wide left", "two-column-wide-narrow", 65, true},
		{"wide right", "two-column-narrow-wide", 35, true},
		{"case-insensitive", " TWO-COLUMN-WIDE-NARROW ", 65, true},
		{"balanced native", "two-column", 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			slides := []SlideInput{{LayoutID: tt.layoutID}}
			resolveCanonicalLayoutIDs(slides, layouts)
			if slides[0].LayoutID != "slideLayout3" || slides[0].ColumnLeftPercent != tt.wantPercent {
				t.Fatalf("resolved slide = %+v, want slideLayout3 and %d%% left", slides[0], tt.wantPercent)
			}
			specs, _, findings, err := convertPresentationSlides(slides, layouts, 12192000, 6858000, nil, nil, "", nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(specs) != 1 || specs[0].ColumnLeftPercent != tt.wantPercent || specs[0].LayoutID != "slideLayout3" {
				t.Errorf("generator specs = %+v, want %d%% left on slideLayout3", specs, tt.wantPercent)
			}
			found := false
			for _, finding := range findings {
				if finding.Code == patterns.ErrCodeLayoutDerived {
					found = true
					if finding.Path != "/slides/0/layout_id" || finding.Action != "info" || !strings.Contains(finding.Message, "0.3-inch gutter") {
						t.Errorf("derived layout finding = %+v", finding)
					}
				}
			}
			if found != tt.wantFinding {
				t.Errorf("LAYOUT_DERIVED found = %t, want %t; findings = %+v", found, tt.wantFinding, findings)
			}
			preflightFound := false
			for _, finding := range collectFitFindings(&PresentationInput{Slides: slides}, layouts, 12192000, 6858000, nil) {
				if finding.Code == patterns.ErrCodeLayoutDerived {
					preflightFound = true
					if finding.Path != "/slides/0/layout_id" || finding.Action != "info" {
						t.Errorf("preflight derived layout finding = %+v", finding)
					}
				}
			}
			if preflightFound != tt.wantFinding {
				t.Errorf("preflight LAYOUT_DERIVED found = %t, want %t", preflightFound, tt.wantFinding)
			}
		})
	}
}

func TestDerivedFitLayoutsArePerSlide(t *testing.T) {
	layouts := []types.LayoutMetadata{{ID: "slideLayout3", Name: "Two Content", Tags: []string{"two-column"}, Placeholders: []types.PlaceholderInfo{
		{ID: "body", Bounds: types.BoundingBox{X: 100000, Y: 100000, Width: 4900000, Height: 5000000}, MaxChars: 100},
		{ID: "body_2", Bounds: types.BoundingBox{X: 5100000, Y: 100000, Width: 4900000, Height: 5000000}, MaxChars: 100},
	}}}
	input := &PresentationInput{Slides: []SlideInput{
		{LayoutID: "slideLayout3", ColumnLeftPercent: 65},
		{LayoutID: "slideLayout3", ColumnLeftPercent: 35},
		{LayoutID: "slideLayout3"},
	}}
	fitInput, fitLayouts, errors := withDerivedFitLayouts(input, layouts, 12192000, 6858000)
	if len(errors) != 0 || len(fitLayouts) != 3 {
		t.Fatalf("derived layouts = %+v; errors = %+v", fitLayouts, errors)
	}
	if input.Slides[0].LayoutID != "slideLayout3" || fitInput.Slides[2].LayoutID != "slideLayout3" {
		t.Error("derivation mutated the author input or the balanced slide")
	}
	for i, wantPercent := range []int{65, 35} {
		derived := fitLayouts[i+1]
		if fitInput.Slides[i].LayoutID != derived.ID || derived.ID == "slideLayout3" {
			t.Errorf("slide %d did not select its own derived metadata", i)
		}
		left, right := derived.Placeholders[0].Bounds, derived.Placeholders[1].Bounds
		gotPercent := float64(left.Width) / float64(left.Width+right.Width) * 100
		if gotPercent < float64(wantPercent)-0.01 || gotPercent > float64(wantPercent)+0.01 {
			t.Errorf("slide %d left share = %.3f%%, want %d%%", i, gotPercent, wantPercent)
		}
		if right.X-left.X-left.Width < 274320 {
			t.Errorf("slide %d derived gap too narrow", i)
		}
		if wantPercent == 65 && (derived.Placeholders[0].MaxChars <= 100 || derived.Placeholders[1].MaxChars >= 100) {
			t.Errorf("wide/narrow preflight budgets did not follow widths: %d/%d", derived.Placeholders[0].MaxChars, derived.Placeholders[1].MaxChars)
		}
		if wantPercent == 35 && (derived.Placeholders[0].MaxChars >= 100 || derived.Placeholders[1].MaxChars <= 100) {
			t.Errorf("narrow/wide preflight budgets did not follow widths: %d/%d", derived.Placeholders[0].MaxChars, derived.Placeholders[1].MaxChars)
		}
	}
	for _, ph := range layouts[0].Placeholders {
		if ph.Bounds.Width != 4900000 {
			t.Error("derivation mutated the native layout")
		}
	}
}

func TestDerivedFitLayoutsRejectsMissingBodyPlaceholder(t *testing.T) {
	layouts := []types.LayoutMetadata{{ID: "slideLayout3", Placeholders: []types.PlaceholderInfo{
		{ID: "body", Bounds: types.BoundingBox{X: 100000, Y: 100000, Width: 4900000, Height: 5000000}},
	}}}
	input := &PresentationInput{Slides: []SlideInput{{LayoutID: "slideLayout3", ColumnLeftPercent: 65}}}
	fitInput, fitLayouts, findings := withDerivedFitLayouts(input, layouts, 12192000, 6858000)
	if fitInput != input || len(fitLayouts) != 1 || len(findings) != 1 || findings[0].Code != patterns.ErrCodeLayoutUnresolvable || findings[0].Action != "refuse" {
		t.Errorf("invalid derived layout response: input=%+v layouts=%+v findings=%+v", fitInput, fitLayouts, findings)
	}
	missing := &PresentationInput{Slides: []SlideInput{{LayoutID: "missing", ColumnLeftPercent: 65}}}
	fitInput, fitLayouts, findings = withDerivedFitLayouts(missing, layouts, 12192000, 6858000)
	if fitInput != missing || len(fitLayouts) != 1 || len(findings) != 0 {
		t.Errorf("unknown layout should remain for the existing resolution detector: input=%+v layouts=%+v findings=%+v", fitInput, fitLayouts, findings)
	}
}

func TestCanonicalAsymmetricLayoutReResolutionClearsStaleSplit(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{ID: "slideLayout2", Name: "One Content", Tags: []string{"content"}},
		{ID: "slideLayout3", Name: "Two Content", Tags: []string{"content", "two-column"}},
	}
	slides := []SlideInput{{LayoutID: "two-column-wide-narrow"}}
	if len(slides) == 0 {
		t.Fatal("test requires one slide")
	}
	slide := &slides[0]
	resolveCanonicalLayoutIDs(slides, layouts)
	if slide.LayoutID != "slideLayout3" || slide.ColumnLeftPercent != 65 || slide.ColumnBaseLayoutID != "slideLayout3" {
		t.Fatalf("initial derived resolution = %+v", slide)
	}
	resolveCanonicalLayoutIDs(slides, layouts)
	if slide.ColumnLeftPercent != 65 {
		t.Error("re-resolving the same concrete layout lost its split")
	}
	slide.LayoutID = "content"
	if got := derivedColumnLeftPercent(*slide); got != 0 {
		t.Errorf("edited canonical role kept stale split %d before re-resolution", got)
	}
	resolveCanonicalLayoutIDs(slides, layouts)
	if slide.LayoutID != "slideLayout2" || slide.ColumnLeftPercent != 0 || slide.ColumnBaseLayoutID != "" {
		t.Errorf("edited one-content slide retained asymmetric split: %+v", slide)
	}
	slide.LayoutID = "two-column-narrow-wide"
	resolveCanonicalLayoutIDs(slides, layouts)
	if slide.LayoutID != "slideLayout3" || slide.ColumnLeftPercent != 35 {
		t.Errorf("new asymmetric alias did not replace the old split: %+v", slide)
	}
	slide.LayoutID = "slideLayout2"
	resolveCanonicalLayoutIDs(slides, layouts)
	if slide.ColumnLeftPercent != 0 {
		t.Errorf("direct concrete layout edit retained stale split: %+v", slide)
	}
}
