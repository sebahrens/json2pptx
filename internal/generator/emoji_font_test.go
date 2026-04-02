package generator

import (
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
		{'📊', true},  // U+1F4CA Bar Chart
		{'🎯', true},  // U+1F3AF Direct Hit
		{'✅', true},  // U+2705 Check Mark
		{'📈', true},  // U+1F4C8 Chart Increasing
		{'🚀', true},  // U+1F680 Rocket
		{'👥', true},  // U+1F465 Busts in Silhouette
		{'💡', true},  // U+1F4A1 Light Bulb
		{0xFE0F, true}, // Variation Selector-16
		{0x200D, true}, // Zero Width Joiner
	}
	for _, tt := range tests {
		got := isEmoji(tt.r)
		if got != tt.want {
			t.Errorf("isEmoji(%q U+%04X) = %v, want %v", tt.r, tt.r, got, tt.want)
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
		{"Revenue 📈 Growth 🎯 Target ✅ Achieved", true},
		{"", false},
	}
	for _, tt := range tests {
		got := containsEmoji(tt.s)
		if got != tt.want {
			t.Errorf("containsEmoji(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestSplitTextByEmoji(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []emojiSegment
	}{
		{
			name: "no emoji",
			s:    "Hello World",
			want: []emojiSegment{{text: "Hello World", emoji: false}},
		},
		{
			name: "only emoji",
			s:    "📊📈",
			want: []emojiSegment{{text: "📊📈", emoji: true}},
		},
		{
			name: "emoji then text",
			s:    "📊 Revenue",
			want: []emojiSegment{
				{text: "📊", emoji: true},
				{text: " Revenue", emoji: false},
			},
		},
		{
			name: "text then emoji",
			s:    "Target ✅",
			want: []emojiSegment{
				{text: "Target ", emoji: false},
				{text: "✅", emoji: true},
			},
		},
		{
			name: "mixed",
			s:    "📊 Revenue 📈 Growth",
			want: []emojiSegment{
				{text: "📊", emoji: true},
				{text: " Revenue ", emoji: false},
				{text: "📈", emoji: true},
				{text: " Growth", emoji: false},
			},
		},
		{
			name: "empty",
			s:    "",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTextByEmoji(tt.s)
			if len(got) != len(tt.want) {
				t.Fatalf("splitTextByEmoji(%q) returned %d segments, want %d\n  got:  %+v\n  want: %+v",
					tt.s, len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i].text != tt.want[i].text || got[i].emoji != tt.want[i].emoji {
					t.Errorf("segment[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSplitEmojiRuns(t *testing.T) {
	t.Run("no emoji", func(t *testing.T) {
		runs := []runXML{
			{RunProperties: &runPropertiesXML{Lang: "en-US"}, Text: "Hello"},
		}
		got := splitEmojiRuns(runs)
		if len(got) != 1 {
			t.Fatalf("expected 1 run, got %d", len(got))
		}
		if got[0].Text != "Hello" {
			t.Errorf("text = %q, want %q", got[0].Text, "Hello")
		}
		if strings.Contains(got[0].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("non-emoji run should not have emoji font")
		}
	})

	t.Run("emoji only", func(t *testing.T) {
		runs := []runXML{
			{RunProperties: &runPropertiesXML{Lang: "en-US"}, Text: "📊"},
		}
		got := splitEmojiRuns(runs)
		if len(got) != 1 {
			t.Fatalf("expected 1 run, got %d", len(got))
		}
		if !strings.Contains(got[0].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("emoji run should have emoji font")
		}
	})

	t.Run("mixed text and emoji", func(t *testing.T) {
		runs := []runXML{
			{RunProperties: &runPropertiesXML{Lang: "en-US"}, Text: "📊 Revenue"},
		}
		got := splitEmojiRuns(runs)
		if len(got) != 2 {
			t.Fatalf("expected 2 runs, got %d: %+v", len(got), got)
		}
		// First run: emoji
		if got[0].Text != "📊" {
			t.Errorf("run[0].Text = %q, want %q", got[0].Text, "📊")
		}
		if !strings.Contains(got[0].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("run[0] should have emoji font")
		}
		// Second run: text
		if got[1].Text != " Revenue" {
			t.Errorf("run[1].Text = %q, want %q", got[1].Text, " Revenue")
		}
		if strings.Contains(got[1].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("run[1] should not have emoji font")
		}
	})

	t.Run("preserves formatting", func(t *testing.T) {
		runs := []runXML{
			{RunProperties: &runPropertiesXML{Lang: "en-US", Bold: "1"}, Text: "🚀 Launch"},
		}
		got := splitEmojiRuns(runs)
		if len(got) != 2 {
			t.Fatalf("expected 2 runs, got %d", len(got))
		}
		if got[0].RunProperties.Bold != "1" {
			t.Error("emoji run should preserve bold")
		}
		if got[1].RunProperties.Bold != "1" {
			t.Error("text run should preserve bold")
		}
	})

	t.Run("nil run properties with emoji", func(t *testing.T) {
		runs := []runXML{
			{Text: "📊"},
		}
		got := splitEmojiRuns(runs)
		if len(got) != 1 {
			t.Fatalf("expected 1 run, got %d", len(got))
		}
		if got[0].RunProperties == nil {
			t.Fatal("emoji run should have run properties")
		}
		if !strings.Contains(got[0].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("emoji run should have emoji font")
		}
	})

	t.Run("replaces existing font", func(t *testing.T) {
		runs := []runXML{
			{RunProperties: &runPropertiesXML{
				Lang:  "en-US",
				Inner: `<a:latin typeface="Arial"/>`,
			}, Text: "📊"},
		}
		got := splitEmojiRuns(runs)
		if len(got) != 1 {
			t.Fatalf("expected 1 run, got %d", len(got))
		}
		if strings.Contains(got[0].RunProperties.Inner, "Arial") {
			t.Error("emoji run should not retain original font")
		}
		if !strings.Contains(got[0].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("emoji run should have emoji font")
		}
	})
}

func TestCreateFormattedRunsEmojiIntegration(t *testing.T) {
	t.Run("emoji in plain text", func(t *testing.T) {
		runs := createFormattedRuns("📊 Revenue 📈 Growth", nil)
		// Should have 4 runs: emoji, text, emoji, text
		if len(runs) != 4 {
			t.Fatalf("expected 4 runs, got %d", len(runs))
		}
		if runs[0].Text != "📊" {
			t.Errorf("run[0].Text = %q, want emoji", runs[0].Text)
		}
		if !strings.Contains(runs[0].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("run[0] should have emoji font")
		}
		if runs[1].Text != " Revenue " {
			t.Errorf("run[1].Text = %q, want text", runs[1].Text)
		}
		if strings.Contains(runs[1].RunProperties.Inner, "Segoe UI Emoji") {
			t.Error("run[1] should not have emoji font")
		}
	})

	t.Run("emoji with bold inline tag", func(t *testing.T) {
		runs := createFormattedRuns("<b>🚀 Launch</b>", nil)
		hasEmoji := false
		for _, r := range runs {
			if strings.Contains(r.Text, "🚀") {
				hasEmoji = true
				if !strings.Contains(r.RunProperties.Inner, "Segoe UI Emoji") {
					t.Error("bold emoji run should have emoji font")
				}
				if r.RunProperties.Bold != "1" {
					t.Error("bold emoji run should be bold")
				}
			}
		}
		if !hasEmoji {
			t.Error("expected at least one run with emoji")
		}
	})

	t.Run("no emoji text unchanged", func(t *testing.T) {
		runs := createFormattedRuns("Plain text here", nil)
		if len(runs) != 1 {
			t.Fatalf("expected 1 run, got %d", len(runs))
		}
		if runs[0].Text != "Plain text here" {
			t.Errorf("text = %q, want %q", runs[0].Text, "Plain text here")
		}
	})
}

func TestStripSelfClosingElement(t *testing.T) {
	tests := []struct {
		name string
		xml  string
		tag  string
		want string
	}{
		{
			name: "strip latin",
			xml:  `<a:latin typeface="Arial"/>`,
			tag:  "a:latin",
			want: "",
		},
		{
			name: "strip with surrounding content",
			xml:  `<a:solidFill/><a:latin typeface="Arial"/><a:sz val="1800"/>`,
			tag:  "a:latin",
			want: `<a:solidFill/><a:sz val="1800"/>`,
		},
		{
			name: "no match",
			xml:  `<a:solidFill/>`,
			tag:  "a:latin",
			want: `<a:solidFill/>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSelfClosingElement(tt.xml, tt.tag)
			if got != tt.want {
				t.Errorf("stripSelfClosingElement(%q, %q) = %q, want %q", tt.xml, tt.tag, got, tt.want)
			}
		})
	}
}
