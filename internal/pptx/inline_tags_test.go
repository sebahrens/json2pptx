package pptx

import "testing"

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
