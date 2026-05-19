package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIsEmoji(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'A', false},
		{'z', false},
		{' ', false},
		{'日', false},
		{'📊', true},   // U+1F4CA Bar Chart
		{'🎯', true},   // U+1F3AF Direct Hit
		{'✅', true},   // U+2705 Check Mark
		{'📈', true},   // U+1F4C8 Chart Increasing
		{'🚀', true},   // U+1F680 Rocket
		{0xFE0F, true}, // Variation Selector-16
		{0x200D, true}, // Zero Width Joiner
	}
	for _, tt := range tests {
		if got := isEmoji(tt.r); got != tt.want {
			t.Errorf("isEmoji(U+%04X) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestContainsEmoji(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"Hello World", false},
		{"📊 Revenue", true},
		{"日本語テスト", false},
		{"Revenue 📈 Growth 🎯", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := containsEmoji(tt.s); got != tt.want {
			t.Errorf("containsEmoji(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

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
	if got := ValidateNoEmojiInText(input); len(got) != 0 {
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
	findings := ValidateNoEmojiInText(input)
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
	findings := ValidateNoEmojiInText(input)
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
	findings := ValidateNoEmojiInText(input)
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
	violations := ValidateNoEmojiInText(input)
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

func TestExtractEmojiSample(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"📊 Revenue 📈 Growth", 3, "📊📈"},
		{"Plain text", 3, ""},
		{"📊📊📊", 3, "📊"}, // distinct only
	}
	for _, tt := range tests {
		if got := extractEmojiSample(tt.in, tt.max); got != tt.want {
			t.Errorf("extractEmojiSample(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

