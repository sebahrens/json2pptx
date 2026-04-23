package pptx

import (
	"testing"
)

func TestSplitInlineTags_NoTags(t *testing.T) {
	base := Run{Text: "hello world", FontSize: 1400, FontFamily: "+mn-lt"}
	runs := SplitInlineTags(base)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Text != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", runs[0].Text)
	}
}

func TestSplitInlineTags_Bold(t *testing.T) {
	base := Run{Text: "start <b>bold</b> end", FontSize: 1400}
	runs := SplitInlineTags(base)
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d: %+v", len(runs), runs)
	}
	if runs[0].Text != "start " || runs[0].Bold {
		t.Errorf("run[0] = %+v", runs[0])
	}
	if runs[1].Text != "bold" || !runs[1].Bold {
		t.Errorf("run[1] = %+v", runs[1])
	}
	if runs[2].Text != " end" || runs[2].Bold {
		t.Errorf("run[2] = %+v", runs[2])
	}
}

func TestSplitInlineTags_Underline(t *testing.T) {
	base := Run{Text: "before <u>underlined</u> after", FontSize: 1400}
	runs := SplitInlineTags(base)
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d: %+v", len(runs), runs)
	}
	if runs[1].Text != "underlined" || !runs[1].Underline {
		t.Errorf("run[1] = %+v", runs[1])
	}
}

func TestSplitInlineTags_Nested(t *testing.T) {
	base := Run{Text: "<b><i>both</i></b>", FontSize: 1400}
	runs := SplitInlineTags(base)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d: %+v", len(runs), runs)
	}
	if !runs[0].Bold || !runs[0].Italic {
		t.Errorf("expected bold+italic, got %+v", runs[0])
	}
}

func TestSplitInlineTags_PreservesBase(t *testing.T) {
	base := Run{Text: "plain <b>bold</b> plain", FontSize: 1400, FontFamily: "+mn-lt", Lang: "en-US"}
	runs := SplitInlineTags(base)
	for i, r := range runs {
		if r.FontSize != 1400 {
			t.Errorf("run[%d].FontSize = %d, expected 1400", i, r.FontSize)
		}
		if r.FontFamily != "+mn-lt" {
			t.Errorf("run[%d].FontFamily = %q, expected +mn-lt", i, r.FontFamily)
		}
	}
}

func TestSplitInlineTags_BaseBoldWithTags(t *testing.T) {
	// If base run is already bold, all runs should be bold; <i> adds italic on top
	base := Run{Text: "bold <i>bold+italic</i> bold", Bold: true, FontSize: 1400}
	runs := SplitInlineTags(base)
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d: %+v", len(runs), runs)
	}
	if !runs[0].Bold || runs[0].Italic {
		t.Errorf("run[0]: expected bold only, got %+v", runs[0])
	}
	if !runs[1].Bold || !runs[1].Italic {
		t.Errorf("run[1]: expected bold+italic, got %+v", runs[1])
	}
	if !runs[2].Bold || runs[2].Italic {
		t.Errorf("run[2]: expected bold only, got %+v", runs[2])
	}
}

func TestSplitInlineTags_UnknownTag(t *testing.T) {
	base := Run{Text: "<span>text</span>", FontSize: 1400}
	runs := SplitInlineTags(base)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d: %+v", len(runs), runs)
	}
	if runs[0].Text != "<span>text</span>" {
		t.Errorf("expected literal passthrough, got %q", runs[0].Text)
	}
}

// ---------------------------------------------------------------------------
// ConvertMarkdownEmphasis tests
// ---------------------------------------------------------------------------

func TestConvertMarkdownEmphasis(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no asterisks", "plain text", "plain text"},
		{"bold", "hello **world** end", "hello <b>world</b> end"},
		{"italic", "hello *world* end", "hello <i>world</i> end"},
		{"bold-italic", "***both***", "<b><i>both</i></b>"},
		{"escaped asterisk", `not \*italic\*`, "not *italic*"},
		{"unmatched single", "price is 5*", "price is 5*"},
		{"unmatched double", "price is 5**", "price is 5**"},
		{"multiple bold", "**a** and **b**", "<b>a</b> and <b>b</b>"},
		{"mixed bold and italic", "**bold** then *italic*", "<b>bold</b> then <i>italic</i>"},
		{"empty bold", "** **", "** **"},
		{"adjacent to text", "pre**bold**post", "pre<b>bold</b>post"},
		{"no content asterisks", "****", "****"},
		{"existing tags pass through", "<b>already</b> tagged", "<b>already</b> tagged"},
		{"bold with existing tags", "**new** and <i>old</i>", "<b>new</b> and <i>old</i>"},
		{"spaces prevent closing", "** not bold **", "** not bold **"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMarkdownEmphasis(tt.input)
			if got != tt.want {
				t.Errorf("ConvertMarkdownEmphasis(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertMarkdownEmphasis_EndToEnd(t *testing.T) {
	// Verify that markdown → tags → SplitInlineTags produces correct runs
	text := "Revenue grew **42%** in *Q3* overall"
	converted := ConvertMarkdownEmphasis(text)
	base := Run{Text: converted, FontSize: 1200}
	runs := SplitInlineTags(base)

	if len(runs) != 5 {
		t.Fatalf("expected 5 runs, got %d: %+v", len(runs), runs)
	}
	// "Revenue grew " — plain
	if runs[0].Text != "Revenue grew " || runs[0].Bold || runs[0].Italic {
		t.Errorf("run[0] = %+v", runs[0])
	}
	// "42%" — bold
	if runs[1].Text != "42%" || !runs[1].Bold || runs[1].Italic {
		t.Errorf("run[1] = %+v", runs[1])
	}
	// " in " — plain
	if runs[2].Text != " in " || runs[2].Bold || runs[2].Italic {
		t.Errorf("run[2] = %+v", runs[2])
	}
	// "Q3" — italic
	if runs[3].Text != "Q3" || runs[3].Bold || !runs[3].Italic {
		t.Errorf("run[3] = %+v", runs[3])
	}
	// " overall" — plain
	if runs[4].Text != " overall" || runs[4].Bold || runs[4].Italic {
		t.Errorf("run[4] = %+v", runs[4])
	}
}
