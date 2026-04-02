package generator

import (
	"testing"
)

func TestParseInlineTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []TextRun
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "plain text",
			input: "hello world",
			want:  []TextRun{{Text: "hello world"}},
		},
		{
			name:  "bold only",
			input: "<b>bold</b>",
			want:  []TextRun{{Text: "bold", Bold: true}},
		},
		{
			name:  "italic only",
			input: "<i>italic</i>",
			want:  []TextRun{{Text: "italic", Italic: true}},
		},
		{
			name:  "bold in middle",
			input: "start <b>bold</b> end",
			want: []TextRun{
				{Text: "start "},
				{Text: "bold", Bold: true},
				{Text: " end"},
			},
		},
		{
			name:  "nested bold italic",
			input: "<b><i>both</i></b>",
			want:  []TextRun{{Text: "both", Bold: true, Italic: true}},
		},
		{
			name:  "bold wrapping italic",
			input: "<b>bold <i>both</i> bold</b>",
			want: []TextRun{
				{Text: "bold ", Bold: true},
				{Text: "both", Bold: true, Italic: true},
				{Text: " bold", Bold: true},
			},
		},
		{
			name:  "italic wrapping bold",
			input: "<i>italic <b>both</b> italic</i>",
			want: []TextRun{
				{Text: "italic ", Italic: true},
				{Text: "both", Bold: true, Italic: true},
				{Text: " italic", Italic: true},
			},
		},
		{
			name:  "unknown tag passthrough",
			input: "<span>text</span>",
			want:  []TextRun{{Text: "<span>text</span>"}},
		},
		{
			name:  "br tag passthrough",
			input: "line1<br>line2",
			want:  []TextRun{{Text: "line1<br>line2"}},
		},
		{
			name:  "unclosed bold",
			input: "<b>unclosed",
			want:  []TextRun{{Text: "unclosed", Bold: true}},
		},
		{
			name:  "unclosed italic",
			input: "<i>unclosed",
			want:  []TextRun{{Text: "unclosed", Italic: true}},
		},
		{
			name:  "empty tags skipped",
			input: "<b></b>",
			want:  nil,
		},
		{
			name:  "multiple separate bold sections",
			input: "<b>one</b> and <b>two</b>",
			want: []TextRun{
				{Text: "one", Bold: true},
				{Text: " and "},
				{Text: "two", Bold: true},
			},
		},
		{
			name:  "angle bracket not a tag",
			input: "2 < 3 and 5 > 2",
			want:  []TextRun{{Text: "2 < 3 and 5 > 2"}},
		},
		{
			name:  "mixed known and unknown tags",
			input: "<b>bold</b> and <div>div</div>",
			want: []TextRun{
				{Text: "bold", Bold: true},
				{Text: " and <div>div</div>"},
			},
		},
		{
			name:  "double nesting same tag",
			input: "<b><b>double</b></b>",
			want:  []TextRun{{Text: "double", Bold: true}},
		},
		{
			name:  "extra close tag ignored gracefully",
			input: "text</b>after",
			want:  []TextRun{{Text: "textafter"}},
		},
		// Underline support
		{
			name:  "underline maps to bold",
			input: "normal <u>underlined</u> normal",
			want: []TextRun{
				{Text: "normal "},
				{Text: "underlined", Bold: true},
				{Text: " normal"},
			},
		},
		// Case-insensitive tags
		{
			name:  "uppercase bold tag",
			input: "<B>BOLD</B>",
			want:  []TextRun{{Text: "BOLD", Bold: true}},
		},
		{
			name:  "mixed case italic tag",
			input: "<I>text</i>",
			want:  []TextRun{{Text: "text", Italic: true}},
		},
		// Adjacent run merging
		{
			name:  "adjacent bold and italic",
			input: "<b>bold</b><i>italic</i>",
			want: []TextRun{
				{Text: "bold", Bold: true},
				{Text: "italic", Italic: true},
			},
		},
		{
			name:  "multiple segments",
			input: "<b>A</b> and <i>B</i> and <b><i>C</i></b>",
			want: []TextRun{
				{Text: "A", Bold: true},
				{Text: " and "},
				{Text: "B", Italic: true},
				{Text: " and "},
				{Text: "C", Bold: true, Italic: true},
			},
		},
		// Unclosed angle bracket
		{
			name:  "unclosed angle bracket",
			input: "Hello <b world",
			want:  []TextRun{{Text: "Hello <b world"}},
		},
		// Math-style angle brackets
		{
			name:  "math angle brackets preserved",
			input: "5 < 10 and 10 > 5",
			want:  []TextRun{{Text: "5 < 10 and 10 > 5"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseInlineTags(tt.input)
			if tt.want == nil && got != nil {
				t.Errorf("expected nil, got %v", got)
				return
			}
			if tt.want != nil && got == nil {
				t.Errorf("expected %v, got nil", tt.want)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("expected %d runs, got %d: %+v", len(tt.want), len(got), got)
				return
			}
			for i, exp := range tt.want {
				if got[i].Text != exp.Text {
					t.Errorf("run[%d].Text = %q, expected %q", i, got[i].Text, exp.Text)
				}
				if got[i].Bold != exp.Bold {
					t.Errorf("run[%d].Bold = %v, expected %v", i, got[i].Bold, exp.Bold)
				}
				if got[i].Italic != exp.Italic {
					t.Errorf("run[%d].Italic = %v, expected %v", i, got[i].Italic, exp.Italic)
				}
			}
		})
	}
}
