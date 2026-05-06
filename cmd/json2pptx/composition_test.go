package main

import (
	"testing"
)

func TestCompositionAxis_PatternRun(t *testing.T) {
	// 5 consecutive card-grid slides should trigger pattern_run warning.
	slides := make([]SlideInput, 5)
	for i := range slides {
		slides[i] = SlideInput{
			Pattern: &PatternInput{Name: "card-grid"},
		}
	}

	result := compositionAxis(slides)
	if result == nil {
		t.Fatal("expected non-nil composition result")
	}
	if result.Score >= 100 {
		t.Errorf("expected composition score < 100 for monotonous deck, got %d", result.Score)
	}

	found := false
	for _, d := range result.Diagnostics {
		if d.Code == "pattern_run" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected pattern_run diagnostic for 5 consecutive card-grid slides")
	}
}

func TestCompositionAxis_MissingEmphasis(t *testing.T) {
	// 12 content slides with no stat-hero or pull-quote.
	slides := make([]SlideInput, 12)
	for i := range slides {
		slides[i] = SlideInput{SlideType: "content"}
	}

	result := compositionAxis(slides)
	if result == nil {
		t.Fatal("expected non-nil composition result")
	}

	found := false
	for _, d := range result.Diagnostics {
		if d.Code == "missing_emphasis" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing_emphasis diagnostic for 12-slide deck with no emphasis slides")
	}
}

func TestCompositionAxis_NoDiagnosticsForVariedDeck(t *testing.T) {
	// A varied deck should have fewer diagnostics.
	slides := []SlideInput{
		{SlideType: "title"},
		{Pattern: &PatternInput{Name: "agenda"}},
		{Pattern: &PatternInput{Name: "card-grid"}},
		{Pattern: &PatternInput{Name: "stat-hero"}},
		{Pattern: &PatternInput{Name: "process-flow"}},
	}

	result := compositionAxis(slides)
	if result == nil {
		t.Fatal("expected non-nil composition result")
	}

	for _, d := range result.Diagnostics {
		if d.Code == "pattern_run" {
			t.Error("should not flag pattern_run on a varied deck")
		}
	}
}

func TestCompositionAxis_Empty(t *testing.T) {
	result := compositionAxis(nil)
	if result != nil {
		t.Error("expected nil for empty slides")
	}
}
