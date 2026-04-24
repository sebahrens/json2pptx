package main

import (
	"errors"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestCheckInputEnumValues_ValidValues(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				Transition:      "fade",
				TransitionSpeed: "slow",
				Build:           "bullets",
				Background: &BackgroundInput{
					Fit: "cover",
				},
			},
			{
				Transition:      "PUSH", // case-insensitive
				TransitionSpeed: "medium",
				Background: &BackgroundInput{
					Fit: "tile",
				},
			},
			{
				// empty values — should not produce errors
			},
		},
	}

	errs := checkInputEnumValues(input)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid enum values, got %d: %v", len(errs), errs)
	}
}

func TestCheckInputEnumValues_InvalidTransition(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{Transition: "slide-left"},
		},
	}
	errs := checkInputEnumValues(input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != patterns.ErrCodeUnknownEnum {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeUnknownEnum, errs[0].Code)
	}
	if errs[0].Path != "slides[0].transition" {
		t.Errorf("expected path slides[0].transition, got %q", errs[0].Path)
	}
	if !errors.Is(errs[0], patterns.ErrUnknownEnum) {
		t.Error("expected error to wrap ErrUnknownEnum")
	}
	if errs[0].Fix == nil || errs[0].Fix.Kind != "use_one_of" {
		t.Error("expected fix with kind use_one_of")
	}
}

func TestCheckInputEnumValues_InvalidTransitionSpeed(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{TransitionSpeed: "snappy"},
		},
	}
	errs := checkInputEnumValues(input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Path != "slides[0].transition_speed" {
		t.Errorf("expected path slides[0].transition_speed, got %q", errs[0].Path)
	}
}

func TestCheckInputEnumValues_InvalidBuild(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{Build: "stagger"},
		},
	}
	errs := checkInputEnumValues(input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Path != "slides[0].build" {
		t.Errorf("expected path slides[0].build, got %q", errs[0].Path)
	}
}

func TestCheckInputEnumValues_InvalidBackgroundFit(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{Background: &BackgroundInput{Fit: "contin"}},
		},
	}
	errs := checkInputEnumValues(input)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Path != "slides[0].background.fit" {
		t.Errorf("expected path slides[0].background.fit, got %q", errs[0].Path)
	}
}

func TestCheckInputEnumValues_MultipleErrors(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				Transition:      "slide-left",
				TransitionSpeed: "snappy",
				Build:           "stagger",
				Background:      &BackgroundInput{Fit: "contin"},
			},
			{
				Transition: "zoom",
			},
		},
	}
	errs := checkInputEnumValues(input)
	if len(errs) != 5 {
		t.Errorf("expected 5 errors, got %d", len(errs))
		for _, e := range errs {
			t.Logf("  %s", e.Error())
		}
	}
}

func TestCheckInputEnumValues_NoneTransition(t *testing.T) {
	// "none" is a valid transition value (explicitly disables)
	input := &PresentationInput{
		Slides: []SlideInput{
			{Transition: "none"},
		},
	}
	errs := checkInputEnumValues(input)
	if len(errs) != 0 {
		t.Errorf("expected no errors for transition=none, got %d", len(errs))
	}
}
