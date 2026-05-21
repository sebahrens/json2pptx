package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/policy/emoji"
)

// These tests exercise the cmd/json2pptx boundary: the no-emoji policy applied
// to this package's PresentationInput type and its conversion to diagnostics.
// The emoji-detection predicate, sanitizer, and FitFinding builder are unit
// tested in internal/policy/emoji.

func TestValidateNoEmojiInText_CleanInput(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "title",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: strPtr("Quarterly Results")},
				},
			},
		},
	}
	if got := emoji.ValidateNoEmojiInText(input); len(got) != 0 {
		t.Errorf("expected no findings, got %d: %+v", len(got), got)
	}
}

func TestValidateNoEmojiInText_DetectsEmojiInTitle(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "title",
				Eyebrow:  "📊 Q1",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: strPtr("Plain title")},
				},
			},
		},
	}
	findings := emoji.ValidateNoEmojiInText(input)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Action != "refuse" {
		t.Errorf("action = %q, want refuse", findings[0].Action)
	}
	if findings[0].Code != "no_emoji_violation" {
		t.Errorf("code = %q, want no_emoji_violation", findings[0].Code)
	}
	if !strings.Contains(findings[0].Path, "eyebrow") {
		t.Errorf("path %q should mention eyebrow", findings[0].Path)
	}
	if !strings.Contains(findings[0].Message, "bundled SVG icon") {
		t.Errorf("message %q should reference bundled SVG icons", findings[0].Message)
	}
}

func TestValidateNoEmojiInText_DetectsEmojiInBullets(t *testing.T) {
	bullets := []string{"First clean bullet", "🚀 Launch readiness"}
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "content",
				Content: []ContentInput{
					{
						PlaceholderID: "body",
						Type:          "bullets",
						BulletsValue:  &bullets,
					},
				},
			},
		},
	}
	findings := emoji.ValidateNoEmojiInText(input)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Path, "bullets_value") {
		t.Errorf("path %q should mention bullets_value", findings[0].Path)
	}
}

func TestValidateNoEmojiInText_DetectsEmojiInPatternOverrides(t *testing.T) {
	overrides := json.RawMessage(`{"label": "Target ✅"}`)
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "content",
				Pattern: &PatternInput{
					Name:      "kpi-3up",
					Overrides: overrides,
				},
			},
		},
	}
	findings := emoji.ValidateNoEmojiInText(input)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for emoji in pattern overrides")
	}
}

func TestValidateNoEmojiInText_DiagnosticConversion(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				LayoutID: "content",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: strPtr("📈 Growth")},
				},
			},
		},
	}
	violations := emoji.ValidateNoEmojiInText(input)
	diags := noEmojiDiagnostics(violations)
	if len(diags) != len(violations) {
		t.Fatalf("expected %d diagnostics, got %d", len(violations), len(diags))
	}
	if diags[0].Code != "no_emoji_violation" {
		t.Errorf("code = %q, want no_emoji_violation", diags[0].Code)
	}
	if diags[0].NextToolCall == nil || diags[0].NextToolCall.Tool != "validate_input" {
		t.Errorf("next_tool_call should point at validate_input, got %+v", diags[0].NextToolCall)
	}
	if diags[0].Fix == nil || diags[0].Fix.Kind != "remove_emoji" {
		t.Errorf("fix should be remove_emoji, got %+v", diags[0].Fix)
	}
}

// TestValidateNoEmojiInText_ExampleDecksAreClean is a regression guard for
// go-slide-creator-2f8c. It loads the example decks that previously embedded
// emoji codepoints (which fell through to text rendering and produced
// mixed-style icon rows) and asserts the no-emoji validator finds nothing.
// If a future edit re-introduces emoji into these decks, this test fails
// loudly before the example ships.
func TestValidateNoEmojiInText_ExampleDecksAreClean(t *testing.T) {
	decks := []string{
		"patterns-smoke.json",
		"varied-pitch-deck.json",
		"sovereign-ai-strategy.json",
	}
	for _, name := range decks {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "examples", name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			var input PresentationInput
			if err := json.Unmarshal(data, &input); err != nil {
				t.Fatalf("unmarshal %s: %v", name, err)
			}
			findings := emoji.ValidateNoEmojiInText(&input)
			if len(findings) != 0 {
				for _, f := range findings {
					t.Errorf("%s: %s — %s", name, f.Path, f.Message)
				}
			}
		})
	}
}
