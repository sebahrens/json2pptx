package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestPlaceholderRemappedFindings verifies that placeholderRemappedFindings
// emits one info-level fit finding per remapped placeholder and skips
// placeholders whose input_id already matches the resolved_id.
// Regression for go-slide-creator-lweh.9.
func TestPlaceholderRemappedFindings(t *testing.T) {
	resolved := []resolvedSlide{
		{
			SlideIndex: 0,
			LayoutID:   "section",
			Placeholders: []resolvedPlaceholder{
				{InputID: "title", ResolvedID: "title", Remapped: false, Type: "text"},
				{InputID: "subtitle", ResolvedID: "body", Remapped: true, Type: "text"},
			},
		},
		{
			SlideIndex: 2,
			LayoutID:   "content",
			Placeholders: []resolvedPlaceholder{
				{InputID: "body", ResolvedID: "body", Remapped: false, Type: "bullets"},
			},
		},
		{
			SlideIndex: 3,
			LayoutID:   "two-column",
			Placeholders: []resolvedPlaceholder{
				{InputID: "body", ResolvedID: "body", Remapped: false, Type: "bullets"},
				{InputID: "subtitle", ResolvedID: "body_2", Remapped: true, Type: "text"},
			},
		},
	}

	findings := placeholderRemappedFindings(resolved)
	if len(findings) != 2 {
		t.Fatalf("expected 2 remap findings, got %d: %+v", len(findings), findings)
	}

	// First remap on slide 0, content index 1.
	got := findings[0]
	if got.Code != patterns.ErrCodePlaceholderRemapped {
		t.Errorf("finding[0].Code = %q, want %q", got.Code, patterns.ErrCodePlaceholderRemapped)
	}
	if got.Action != "info" {
		t.Errorf("finding[0].Action = %q, want \"info\"", got.Action)
	}
	if got.Path != "/slides/0/content/1/placeholder_id" {
		t.Errorf("finding[0].Path = %q, want /slides/0/content/1/placeholder_id", got.Path)
	}
	if got.Fix == nil || got.Fix.Kind != "remap_placeholder" {
		t.Fatalf("finding[0].Fix.Kind missing or wrong: %+v", got.Fix)
	}
	if got.Fix.Params["from"] != "subtitle" || got.Fix.Params["to"] != "body" {
		t.Errorf("finding[0].Fix.Params = %+v, want from=subtitle to=body", got.Fix.Params)
	}

	// Second remap on slide 3, content index 1 → body_2.
	got = findings[1]
	if got.Path != "/slides/3/content/1/placeholder_id" {
		t.Errorf("finding[1].Path = %q, want /slides/3/content/1/placeholder_id", got.Path)
	}
	if got.Fix == nil || got.Fix.Params["to"] != "body_2" {
		t.Errorf("finding[1].Fix.Params = %+v, want to=body_2", got.Fix.Params)
	}
}

// TestPlaceholderRemappedFindings_Empty verifies the no-remap case.
func TestPlaceholderRemappedFindings_Empty(t *testing.T) {
	resolved := []resolvedSlide{
		{
			SlideIndex: 0,
			LayoutID:   "content",
			Placeholders: []resolvedPlaceholder{
				{InputID: "title", ResolvedID: "title", Remapped: false, Type: "text"},
				{InputID: "body", ResolvedID: "body", Remapped: false, Type: "bullets"},
			},
		},
	}
	if findings := placeholderRemappedFindings(resolved); len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %+v", len(findings), findings)
	}
}
