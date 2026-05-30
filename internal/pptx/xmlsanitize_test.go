package pptx

import "testing"

func TestIsIllegalXMLChar(t *testing.T) {
	legal := []rune{'\t', '\n', '\r', ' ', 'a', '~', 0x20, 0xD7FF, 0xE000, 0xFFFD, 0x10000, 0x10FFFF, '€', '😀'}
	for _, r := range legal {
		if IsIllegalXMLChar(r) {
			t.Errorf("IsIllegalXMLChar(%#U) = true, want false", r)
		}
	}

	illegal := []rune{0x00, 0x01, 0x08, 0x0B, 0x0C, 0x0E, 0x1F, 0xD800, 0xDFFF, 0xFFFE, 0xFFFF}
	for _, r := range illegal {
		if !IsIllegalXMLChar(r) {
			t.Errorf("IsIllegalXMLChar(%#U) = false, want true", r)
		}
	}
}

func TestEscapeXMLText_StripsIllegalControls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"null byte stripped", "hello\x00world", "helloworld"},
		{"c0 controls stripped", "a\x01b\x08c\x0Bd\x0Ce\x0Ef\x1Fg", "abcdefg"},
		{"metachars still escaped", "<a> & </b>", "&lt;a&gt; &amp; &lt;/b&gt;"},
		{"tab newline cr preserved", "a\tb\nc\rd", "a\tb\nc\rd"},
		{"valid unicode preserved", "café €", "café €"},
		{"control between metachars", "x\x00<y\x1F>z", "x&lt;y&gt;z"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeXMLText(tc.input); got != tc.expected {
				t.Errorf("escapeXMLText(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestEscapeXMLAttr_StripsIllegalControls(t *testing.T) {
	if got := escapeXMLAttr("alt\x00\x1Btext\"q\""); got != "alttext&quot;q&quot;" {
		t.Errorf("escapeXMLAttr stripped result = %q, want %q", got, "alttext&quot;q&quot;")
	}
}
