package emoji

import "testing"

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
		if got := IsEmoji(tt.r); got != tt.want {
			t.Errorf("IsEmoji(U+%04X) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestContains(t *testing.T) {
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
		if got := Contains(tt.s); got != tt.want {
			t.Errorf("Contains(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestExtractSample(t *testing.T) {
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
		if got := ExtractSample(tt.in, tt.max); got != tt.want {
			t.Errorf("ExtractSample(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"clean unchanged", "Quarterly Results", "Quarterly Results"},
		{"clean preserves spacing", "   Leading and trailing   ", "   Leading and trailing   "},
		{"empty", "", ""},
		{"non-latin clean", "日本語テスト", "日本語テスト"},
		{"strips leading emoji", "📊 Revenue", "Revenue"},
		{"strips trailing emoji", "Target ✅", "Target"},
		{"strips and collapses", "Revenue 📈 Growth 🎯", "Revenue Growth"},
		{"strips zwj sequence", "👨‍💻 Developer", "Developer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.in)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if Contains(got) {
				t.Errorf("Sanitize(%q) = %q still contains emoji", tt.in, got)
			}
		})
	}
}

func TestScanFindsViolationsWithPaths(t *testing.T) {
	input := map[string]any{
		"title": "Plain",
		"slides": []any{
			map[string]any{"heading": "📊 Q1"},
			map[string]any{"heading": "clean"},
		},
	}
	violations := Scan(input)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Path != "slides[0].heading" {
		t.Errorf("path = %q, want slides[0].heading", violations[0].Path)
	}
	if violations[0].Sample != "📊" {
		t.Errorf("sample = %q, want 📊", violations[0].Sample)
	}
}

func TestScanNilAndClean(t *testing.T) {
	if got := Scan(nil); got != nil {
		t.Errorf("Scan(nil) = %+v, want nil", got)
	}
	if got := Scan(map[string]any{"a": "clean", "b": "text"}); len(got) != 0 {
		t.Errorf("Scan(clean) = %+v, want empty", got)
	}
}

func TestValidateNoEmojiInTextBuildsFindings(t *testing.T) {
	input := map[string]any{"caption": "Growth 🚀"}
	findings := ValidateNoEmojiInText(input)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Action != "refuse" {
		t.Errorf("action = %q, want refuse", f.Action)
	}
	if f.Code != "no_emoji_violation" {
		t.Errorf("code = %q, want no_emoji_violation", f.Code)
	}
	if f.Pattern != "no_emoji" {
		t.Errorf("pattern = %q, want no_emoji", f.Pattern)
	}
	if f.Fix == nil || f.Fix.Kind != "remove_emoji" {
		t.Errorf("fix = %+v, want kind remove_emoji", f.Fix)
	}
}

func TestValidateNoEmojiInTextCleanReturnsNil(t *testing.T) {
	if got := ValidateNoEmojiInText(map[string]any{"caption": "Growth"}); got != nil {
		t.Errorf("expected nil findings for clean input, got %+v", got)
	}
}

// A feature-comparison table written with ✓ and ✗ — the single most common
// encoding for a competitor matrix — was refused with one blocking error per
// cell, because U+2600-27BF was blocked wholesale. The remediation the message
// offered (a bundled SVG icon) cannot go in a table cell, which takes only
// strings, so the only way through was √ and ×, which reads as a typo
// (go-slide-creator-l38d).
func TestTypographicSymbolsAreNotEmoji(t *testing.T) {
	allowed := []struct {
		r    rune
		name string
	}{
		{'✓', "check mark"},
		{'✔', "heavy check mark"},
		{'✗', "ballot X"},
		{'✘', "heavy ballot X"},
		{'★', "black star"},
		{'☆', "white star"},
		{'☐', "ballot box"},
		{'☑', "ballot box with check"},
		{'☒', "ballot box with X"},
		{'©', "copyright"},
		{'®', "registered"},
		{'™', "trade mark"},
	}
	for _, tc := range allowed {
		if IsEmoji(tc.r) {
			t.Errorf("IsEmoji(U+%04X %s) = true; it is a monochrome text glyph and a deck needs it", tc.r, tc.name)
		}
		if Contains(string(tc.r)) {
			t.Errorf("Contains(%q) = true for %s", string(tc.r), tc.name)
		}
	}

	// The pictographic neighbours in the same blocks stay blocked.
	blocked := []struct {
		r    rune
		name string
	}{
		{'✅', "white heavy check mark (emoji presentation)"},
		{'❌', "cross mark (emoji presentation)"},
		{'⭐', "star (U+2B50, emoji presentation)"},
		{'☀', "black sun with rays"},
		{'☺', "white smiling face"},
		{'🚀', "rocket"},
		{'😀', "grinning face"},
		{0xFE0F, "variation selector-16"},
		{0x200D, "zero width joiner"},
	}
	for _, tc := range blocked {
		if !IsEmoji(tc.r) {
			t.Errorf("IsEmoji(U+%04X %s) = false; it needs an emoji font", tc.r, tc.name)
		}
	}
}

// A permitted symbol followed by U+FE0F asks for the colour glyph, so the
// sequence is still a violation — and Sanitize keeps the symbol while dropping
// the selector.
func TestVariationSelectorStillBlockedAfterAllowedSymbol(t *testing.T) {
	const withVS = "SSO ✓️"
	if !Contains(withVS) {
		t.Error("✓ followed by VS16 should still be a violation")
	}
	if got := Sanitize(withVS); got != "SSO ✓" {
		t.Errorf("Sanitize(%q) = %q, want the symbol without the selector", withVS, got)
	}
}

// A competitor matrix survives the scan intact.
func TestScanAllowsACompetitorMatrix(t *testing.T) {
	deck := map[string]any{
		"rows": []any{
			[]any{"SSO / SCIM", "✓", "✓", "✗"},
			[]any{"On-prem deploy", "✓", "✗", "✓"},
			[]any{"Vendor", "Acme®", "Globex™", "Initech©"},
		},
	}
	if v := Scan(deck); len(v) != 0 {
		t.Errorf("a check/cross matrix was flagged: %+v", v)
	}
}

func TestAllowedSymbolsIsSortedAndComplete(t *testing.T) {
	got := AllowedSymbols()
	if len(got) != len(TypographicSymbols) {
		t.Fatalf("AllowedSymbols returned %d of %d symbols", len(got), len(TypographicSymbols))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("AllowedSymbols is not in codepoint order at %d: %q then %q", i, got[i-1], got[i])
		}
	}
}
