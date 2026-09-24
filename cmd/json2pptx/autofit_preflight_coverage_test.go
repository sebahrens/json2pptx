package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func denseBulletsSlide(layoutID string, n int) SlideInput {
	bullets := make([]string, n)
	for i := range bullets {
		bullets[i] = "Revenue grew eighteen percent year over year to forty two million dollars driven mainly by " +
			"enterprise renewals and a strong second half in the Nordics, with the pipeline weighted to the public sector."
	}
	title := "Dense body"
	return SlideInput{
		SlideType: "content",
		LayoutID:  layoutID,
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: &title},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets},
		},
	}
}

func autofitCodes(findings []patterns.FitFinding) []string {
	var out []string
	for _, f := range findings {
		switch f.Code {
		case patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed, patterns.ErrCodeTextBelowReadableMin:
			out = append(out, f.Code)
		}
	}
	return out
}

// The autofit prediction keyed on an explicit layout_id, so a deck that lets
// the engine choose its layout — most decks, and every deck the semantic
// compiler emits — got no text_trimmed, no readability_trimmed and no
// readability verdict at all. The findings arrived a render round-trip late
// (go-slide-creator-nlrg; go-slide-creator-t64e fixed the same gap for
// measured titles).
func TestAutofitPreflightRunsWithoutAnExplicitLayoutID(t *testing.T) {
	// Real layouts: the selector scores tags, capacity and canonical roles, so
	// a synthetic one-layout fixture would not exercise the path this test is
	// about.
	layouts := loadLayoutsForChromeTest(t, filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	named := layoutIDWithBody(layouts)
	if named == "" {
		t.Fatal("midnight-blue has no layout with a body placeholder")
	}

	withLayout := &PresentationInput{Slides: []SlideInput{denseBulletsSlide(named, 14)}}
	pinnedFindings := collectTextAutofitPreflightFindings(withLayout, layouts)
	pinned := autofitCodes(pinnedFindings)
	if len(pinned) == 0 {
		t.Fatal("no autofit prediction for a slide that names its layout — the fixture is not dense enough")
	}
	for _, finding := range pinnedFindings {
		if finding.Path != "/slides/0/content/1" {
			t.Errorf("autofit finding path = %q, want authored body item", finding.Path)
		}
		assertFindingPointerReplaceable(t, withLayout, finding.Path)
	}

	auto := &PresentationInput{Slides: []SlideInput{denseBulletsSlide("", 14)}}
	got := autofitCodes(collectTextAutofitPreflightFindings(auto, layouts))
	if len(got) == 0 {
		t.Errorf("a slide without layout_id drew no autofit prediction; the slide that names the same layout drew %v", pinned)
	}
	if strings.Join(got, ",") != strings.Join(pinned, ",") {
		t.Errorf("predictions differ by how the layout was chosen: %v (named) vs %v (selected)", pinned, got)
	}
}

// The readability verdict is independent of trimming — the renderer trims and
// THEN judges the size it ended up with — so a trim finding must not mask it.
func TestAutofitPreflightReportsTrimAndReadabilityTogether(t *testing.T) {
	// A body whose predicted size lands under the floor: a small base size is
	// what the renderer floors dense lists to, and the prediction is judged
	// against the same policy.
	layouts := []types.LayoutMetadata{{
		ID:            "slideLayout2",
		CanonicalType: types.CanonicalLayoutType("content"),
		Placeholders: []types.PlaceholderInfo{{
			ID: "body", Type: types.PlaceholderBody, FontSize: 1200, FontFamily: "Calibri",
			Bounds: types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 1200000},
		}},
	}}
	input := &PresentationInput{Slides: []SlideInput{denseBulletsSlide("slideLayout2", 14)}}

	var sawTrim, sawReadable bool
	for _, code := range autofitCodes(collectTextAutofitPreflightFindings(input, layouts)) {
		switch code {
		case patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed:
			sawTrim = true
		case patterns.ErrCodeTextBelowReadableMin:
			sawReadable = true
		}
	}
	if !sawTrim {
		t.Error("expected a trim prediction for 14 long bullets in a short placeholder")
	}
	if !sawReadable {
		t.Error("expected TEXT_BELOW_READABLE_MIN alongside the trim: generate reports both for the same placeholder")
	}
}

// layoutIDWithBody returns the id of the first layout carrying a usable body
// placeholder — the layout the autofit prediction has something to measure in.
func layoutIDWithBody(layouts []types.LayoutMetadata) string {
	for i := range layouts {
		for _, ph := range layouts[i].Placeholders {
			if ph.Type == types.PlaceholderBody && ph.Bounds.Width > 0 && ph.Bounds.Height > 0 {
				return layouts[i].ID
			}
		}
	}
	return ""
}
