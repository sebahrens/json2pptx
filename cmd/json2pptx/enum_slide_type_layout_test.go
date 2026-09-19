package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/layout"
)

// TestSlideTypeCanonicalLayoutNameSuggestsLayoutID is the go-slide-creator-ejh5u
// acceptance test: slide_type "closing" is rejected (it is a layout name, not a
// slide type), and the rejection has to say where the value belongs. It used to
// return only the nine allowed slide types, which do not include the word, so an
// author hard-coded per-template layout ids instead.
func TestSlideTypeCanonicalLayoutNameSuggestsLayoutID(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{
		{SlideType: "title"},
		{SlideType: "closing"},
	}}
	errs := checkInputEnumValues(input)
	if len(errs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(errs), errs)
	}
	e := errs[0]
	if e.Path != "/slides/1/slide_type" {
		t.Errorf("path = %q, want /slides/1/slide_type", e.Path)
	}
	if !strings.Contains(e.Message, "layout_id") {
		t.Errorf("message does not name layout_id: %q", e.Message)
	}
	if !strings.Contains(e.Message, "title, content, section") {
		t.Errorf("message should still list the slide types: %q", e.Message)
	}
	if e.Fix == nil || e.Fix.Kind != "rename_field" {
		t.Fatalf("fix = %+v, want a rename_field", e.Fix)
	}
	if got := e.Fix.Params["from"]; got != "slide_type" {
		t.Errorf("fix.params.from = %v, want slide_type", got)
	}
	if got := e.Fix.Params["to"]; got != "layout_id" {
		t.Errorf("fix.params.to = %v, want layout_id", got)
	}
	if got := e.Fix.Params["value"]; got != "closing" {
		t.Errorf("fix.params.value = %v, want closing", got)
	}
}

// TestSlideTypeLayoutNameRuleIsGeneral pins that the suggestion is not a
// "closing" special case: every canonical layout name gets it, and a value that
// is neither a slide type nor a layout name keeps the plain enum error.
func TestSlideTypeLayoutNameRuleIsGeneral(t *testing.T) {
	for _, name := range layout.CanonicalNames() {
		if slidesContains(canonicalSlideTypes, name) {
			continue // "title", "content", "section", "two-column" are both
		}
		t.Run(name, func(t *testing.T) {
			errs := checkInputEnumValues(&PresentationInput{Slides: []SlideInput{{SlideType: name}}})
			if len(errs) != 1 {
				t.Fatalf("got %d findings, want 1", len(errs))
			}
			if errs[0].Fix == nil || errs[0].Fix.Kind != "rename_field" {
				t.Errorf("%s: fix = %+v, want rename_field", name, errs[0].Fix)
			}
		})
	}

	t.Run("a genuine typo keeps the plain enum error", func(t *testing.T) {
		errs := checkInputEnumValues(&PresentationInput{Slides: []SlideInput{{SlideType: "contnet"}}})
		if len(errs) != 1 {
			t.Fatalf("got %d findings, want 1", len(errs))
		}
		if errs[0].Fix == nil || errs[0].Fix.Kind != "use_one_of" {
			t.Errorf("fix = %+v, want the plain use_one_of", errs[0].Fix)
		}
		if strings.Contains(errs[0].Message, "layout_id") {
			t.Errorf("a typo must not be sent to layout_id: %q", errs[0].Message)
		}
	})
}

// TestSlideTypeLayoutNameFixIsApplicable pins that the suggested fix is one
// repair_slide can actually run: renaming the key moves the value to layout_id.
func TestSlideTypeLayoutNameFixIsApplicable(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{SlideType: "closing"}}}
	errs := checkInputEnumValues(input)
	if len(errs) != 1 || errs[0].Fix == nil {
		t.Fatalf("expected one finding with a fix, got %+v", errs)
	}
	applied := applyRenameField(input, 0, errs[0].Fix.Params)
	if !applied.Applied {
		t.Fatalf("repair_slide could not apply the suggested fix: %s", applied.Message)
	}
	slide := input.Slides[0]
	if slide.LayoutID != "closing" {
		t.Errorf("layout_id = %q, want closing", slide.LayoutID)
	}
	if slide.SlideType != "" {
		t.Errorf("slide_type = %q, want it cleared", slide.SlideType)
	}
	if rest := checkInputEnumValues(input); len(rest) != 0 {
		t.Errorf("the repaired deck should validate clean, got %+v", rest)
	}
}

func slidesContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
