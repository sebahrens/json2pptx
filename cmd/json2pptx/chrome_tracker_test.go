package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

func trackerTitled(slideType, layoutID, title string) SlideInput {
	t := title
	return SlideInput{SlideType: slideType, LayoutID: layoutID, Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &t}}}
}

// TestApplyChromeTrackerRunningSection pins chrome.tracker's section source
// (go-slide-creator-r3gsw): a slide's own section_title from
// structure.sections, else the most recent section divider on a flat deck.
// Title, divider, closing and agenda slides carry none; an eyebrow wins.
func TestApplyChromeTrackerRunningSection(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{ID: "title", CanonicalType: types.CanonicalLayoutTitleSlide},
		{ID: "section", CanonicalType: types.CanonicalLayoutSectionDivider},
		{ID: "content", CanonicalType: types.CanonicalLayoutOneContent},
		{ID: "closing", CanonicalType: types.CanonicalLayoutClosing},
	}
	eyebrowed := trackerTitled("content", "content", "Authored eyebrow")
	eyebrowed.Eyebrow = "CASE STUDY"
	structured := trackerTitled("content", "content", "From structure")
	structured.SectionTitle = "Structured section"
	agenda := trackerTitled("content", "content", "Agenda")
	agenda.Pattern = &PatternInput{Name: "agenda"}
	slides := []SlideInput{
		trackerTitled("title", "title", "Cover"),
		trackerTitled("content", "content", "Before any section"),
		trackerTitled("section", "section", "Market context"),
		trackerTitled("content", "content", "Demand is shifting"),
		eyebrowed,
		agenda,
		trackerTitled("", "closing", "Close"),
	}
	specs := make([]generator.SlideSpec, len(slides))
	for i, s := range slides {
		specs[i].LayoutID = s.LayoutID
	}
	applyChromeTracker(specs, &ChromeInput{Tracker: true}, slides, layouts)
	want := []string{"", "", "", "Market context", "", "", ""}
	for i := range specs {
		if specs[i].Tracker != want[i] {
			t.Errorf("slide %d tracker = %q, want %q", i, specs[i].Tracker, want[i])
		}
	}

	// A structure-expanded deck: sections come from section_title only, so a
	// closing content slide after the last section carries no tracker.
	closer := trackerTitled("content", "content", "Next steps")
	sSlides := []SlideInput{trackerTitled("section", "section", "Structured section"), structured, closer}
	sSpecs := make([]generator.SlideSpec, len(sSlides))
	for i, s := range sSlides {
		sSpecs[i].LayoutID = s.LayoutID
	}
	applyChromeTracker(sSpecs, &ChromeInput{Tracker: true}, sSlides, layouts)
	if sSpecs[1].Tracker != "Structured section" || sSpecs[2].Tracker != "" {
		t.Errorf("structured trackers = %q / %q, want section then none", sSpecs[1].Tracker, sSpecs[2].Tracker)
	}

	off := make([]generator.SlideSpec, len(slides))
	for i, s := range slides {
		off[i].LayoutID = s.LayoutID
	}
	applyChromeTracker(off, &ChromeInput{SectionCrumb: true}, slides, layouts)
	for i := range off {
		if off[i].Tracker != "" {
			t.Errorf("tracker off: slide %d got %q", i, off[i].Tracker)
		}
	}
}
