package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// s7wmhTheme mirrors midnight-blue: a near-black dk1, a navy dk2 the layouts
// draw titles in, and a white lt1.
var s7wmhCmdTheme = []types.ThemeColor{
	{Name: "dk1", RGB: "#000000"},
	{Name: "dk2", RGB: "#1B2A4A"},
	{Name: "lt1", RGB: "#FFFFFF"},
	{Name: "accent1", RGB: "#2E5090"},
}

// s7wmhLayouts is one section layout whose title the template draws in dk2.
var s7wmhLayouts = []types.LayoutMetadata{{
	ID:   "slideLayout1",
	Name: "Section Divider",
	Placeholders: []types.PlaceholderInfo{{
		ID:        "title",
		Type:      types.PlaceholderTitle,
		FontColor: "dk2",
		FontSize:  3600,
		Bounds:    types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1200000},
	}},
}}

func s7wmhSlide(background string) SlideInput {
	slide := SlideInput{
		SlideType: "section",
		LayoutID:  "slideLayout1",
		Content: []ContentInput{{
			PlaceholderID: "title",
			Type:          "text",
			TextValue:     strPtr("Where we stand"),
		}},
	}
	if background != "" {
		slide.Background = &BackgroundInput{Color: background}
	}
	return slide
}

func contrastPredictions(findings []patterns.FitFinding) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range findings {
		if f.Code == patterns.ErrCodeContrastPredicted {
			out = append(out, f)
		}
	}
	return out
}

// TestContrastPreflight_AuthorBackgroundPredicted is the validate half of
// go-slide-creator-s7wmh: a slide that darkens its own background gets a
// contrast_predicted finding naming the palette colour generate will swap in.
// Nothing in the JSON names the title's colour — it is the template's — so the
// shape_grid walk never saw this case and the deck validated clean, then
// reported a swap at generate time.
func TestContrastPreflight_AuthorBackgroundPredicted(t *testing.T) {
	in := &PresentationInput{Slides: []SlideInput{s7wmhSlide("dk1")}}

	findings := contrastPredictions(collectContrastPreflightFindings(in, s7wmhLayouts, s7wmhCmdTheme))
	if len(findings) != 1 {
		t.Fatalf("expected 1 contrast_predicted finding, got %d (%+v)", len(findings), findings)
	}
	f := findings[0]
	if f.Fix != nil {
		t.Errorf("inherited placeholder text cannot be repaired by editing shape_grid: %+v", f.Fix)
	}
	if !strings.Contains(f.Message, "#1B2A4A → #FFFFFF (on #000000") {
		t.Errorf("prediction lost original/replacement/background colors: %q", f.Message)
	}
	if !strings.Contains(f.Path, "/slides/0") {
		t.Errorf("path = %q, want the slide's title content", f.Path)
	}
}

// TestContrastPreflight_LayoutBackgroundNotPredicted pins the bead's
// no-change clause: a deck whose background comes from its layout is not the
// author's doing, and validate must stay silent about it here — the template
// chose that pairing.
func TestContrastPreflight_LayoutBackgroundNotPredicted(t *testing.T) {
	in := &PresentationInput{Slides: []SlideInput{s7wmhSlide("")}}
	if f := contrastPredictions(collectContrastPreflightFindings(in, s7wmhLayouts, s7wmhCmdTheme)); len(f) != 0 {
		t.Errorf("a slide with no background of its own should draw no prediction, got %+v", f)
	}
}

// TestContrastPreflight_ContrastCheckOptOutSilences guards against predicting a
// swap the renderer will not make: contrast_check: false turns the whole pass
// off for that slide.
func TestContrastPreflight_ContrastCheckOptOutSilences(t *testing.T) {
	slide := s7wmhSlide("dk1")
	off := false
	slide.ContrastCheck = &off
	in := &PresentationInput{Slides: []SlideInput{slide}}
	if f := contrastPredictions(collectContrastPreflightFindings(in, s7wmhLayouts, s7wmhCmdTheme)); len(f) != 0 {
		t.Errorf("contrast_check: false should silence the prediction, got %+v", f)
	}
}

// TestContrastPreflight_ReadableOnAuthorBackground: an author background the
// template's title colour already reads well on draws nothing.
func TestContrastPreflight_ReadableOnAuthorBackground(t *testing.T) {
	in := &PresentationInput{Slides: []SlideInput{s7wmhSlide("lt1")}}
	if f := contrastPredictions(collectContrastPreflightFindings(in, s7wmhLayouts, s7wmhCmdTheme)); len(f) != 0 {
		t.Errorf("dk2 on white is readable; expected no prediction, got %+v", f)
	}
}
