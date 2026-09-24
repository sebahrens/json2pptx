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

// An unresolved template background gives preflight no measurable canvas.
func TestContrastPreflight_UnresolvedLayoutBackgroundNotPredicted(t *testing.T) {
	in := &PresentationInput{Slides: []SlideInput{s7wmhSlide("")}}
	if f := contrastPredictions(collectContrastPreflightFindings(in, s7wmhLayouts, s7wmhCmdTheme)); len(f) != 0 {
		t.Errorf("a slide with no resolvable background should draw no prediction, got %+v", f)
	}
}

func TestContrastPreflight_TemplateBackgroundMatchesRendererReplacement(t *testing.T) {
	layouts := []types.LayoutMetadata{{
		ID: "slideLayout1", BackgroundRef: "#FFE8D4",
		Placeholders: []types.PlaceholderInfo{{ID: "title", Type: types.PlaceholderTitle,
			FontColor: "#FD5108", FontSize: 2400}},
	}}
	input := &PresentationInput{Slides: []SlideInput{{LayoutID: "slideLayout1",
		Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Readable title")}},
	}}}
	findings := contrastPredictions(collectContrastPreflightFindings(input, layouts, s7wmhCmdTheme))
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "#FD5108 → #F34E08 (on #FFE8D4") {
		t.Fatalf("template-background prediction = %+v, want visible 3:1 repair", findings)
	}
	if findings[0].Fix != nil {
		t.Errorf("template-owned text has no authored replace_color target: %+v", findings[0].Fix)
	}
}

func TestContrastPreflight_ChromeRespectsLayoutSourceAndSkip(t *testing.T) {
	layouts := []types.LayoutMetadata{{
		ID: "slideLayout1", CanonicalType: types.CanonicalLayoutOneContent,
		ChromeBackgroundRef: "#000000", ChromeTextRef: "tx1",
	}}
	input := &PresentationInput{Chrome: &ChromeInput{}, Slides: []SlideInput{{LayoutID: "slideLayout1"}}}
	check := func(want int) {
		t.Helper()
		got := contrastPredictions(collectContrastPreflightFindings(input, layouts, s7wmhCmdTheme))
		if len(got) != want {
			t.Fatalf("chrome predictions = %+v, want %d", got, want)
		}
		if want > 0 && (got[0].Path != "/slides/0/chrome" || got[0].Fix != nil ||
			!strings.Contains(got[0].Message, "#000000 → #FFFFFF (on #000000")) {
			t.Errorf("chrome prediction = %+v", got[0])
		}
	}
	check(1)
	input.Chrome.PageNumbers = &PageNumbersInput{Skip: []string{"content"}}
	layouts[0].Tags = []string{"content"}
	check(0)
	input.Chrome = nil
	check(0)
	input.Footer = &JSONFooter{Enabled: true}
	check(1)
	layouts[0].ChromeBackgroundRef = ""
	check(0)
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
