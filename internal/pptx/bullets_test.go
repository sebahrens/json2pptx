package pptx

import "testing"

func TestParseBulletText_Empty(t *testing.T) {
	paras := ParseBulletText("", BulletTextOptions{FontSize: 1400})
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph for empty input, got %d", len(paras))
	}
	if paras[0].Runs[0].Text != "" {
		t.Errorf("expected empty text, got %q", paras[0].Runs[0].Text)
	}
}

func TestParseBulletText_PlainLines(t *testing.T) {
	paras := ParseBulletText("Line one\nLine two", BulletTextOptions{FontSize: 1400})
	if len(paras) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(paras))
	}
	for i, p := range paras {
		if p.Bullet != nil {
			t.Errorf("paragraph %d: expected no bullet", i)
		}
	}
	if paras[0].Runs[0].Text != "Line one" {
		t.Errorf("paragraph 0 text: got %q", paras[0].Runs[0].Text)
	}
	if paras[1].Runs[0].Text != "Line two" {
		t.Errorf("paragraph 1 text: got %q", paras[1].Runs[0].Text)
	}
}

func TestParseBulletText_DashBullets(t *testing.T) {
	paras := ParseBulletText("- First\n- Second", BulletTextOptions{FontSize: 1400})
	if len(paras) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(paras))
	}
	for i, p := range paras {
		if p.Bullet == nil {
			t.Fatalf("paragraph %d: expected bullet", i)
		}
		if p.Bullet.Char != "\u2022" {
			t.Errorf("paragraph %d: bullet char: got %q, want %q", i, p.Bullet.Char, "\u2022")
		}
		if p.MarginL != BulletMarginLeft {
			t.Errorf("paragraph %d: MarginL: got %d, want %d", i, p.MarginL, BulletMarginLeft)
		}
		if p.Indent != BulletIndent {
			t.Errorf("paragraph %d: Indent: got %d, want %d", i, p.Indent, BulletIndent)
		}
	}
	if paras[0].Runs[0].Text != "First" {
		t.Errorf("paragraph 0 text: got %q, want %q", paras[0].Runs[0].Text, "First")
	}
}

func TestParseBulletText_UnicodeBulletPrefix(t *testing.T) {
	paras := ParseBulletText("\u2022 First\n\u2022 Second", BulletTextOptions{FontSize: 1400})
	if len(paras) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(paras))
	}
	for i, p := range paras {
		if p.Bullet == nil {
			t.Fatalf("paragraph %d: expected bullet", i)
		}
		if p.Bullet.Char != "\u2022" {
			t.Errorf("paragraph %d: bullet char: got %q, want %q", i, p.Bullet.Char, "\u2022")
		}
		if p.MarginL != BulletMarginLeft {
			t.Errorf("paragraph %d: MarginL: got %d, want %d", i, p.MarginL, BulletMarginLeft)
		}
		if p.Indent != BulletIndent {
			t.Errorf("paragraph %d: Indent: got %d, want %d", i, p.Indent, BulletIndent)
		}
	}
	if paras[0].Runs[0].Text != "First" {
		t.Errorf("paragraph 0 text: got %q, want %q", paras[0].Runs[0].Text, "First")
	}
	if paras[1].Runs[0].Text != "Second" {
		t.Errorf("paragraph 1 text: got %q, want %q", paras[1].Runs[0].Text, "Second")
	}
}

// TestParseBulletText_NoSpuriousWhitespace is a regression guard for
// go-slide-creator-fh8r: a bug report claimed "KEY PARTNERS" became
// "KEY PART NERS" through the renderer. The OOXML the renderer emits is
// driven by ParseBulletText, so any spurious whitespace insertion in this
// path would surface as a header artifact in shape_grid / bmc-canvas cells.
func TestParseBulletText_NoSpuriousWhitespace(t *testing.T) {
	inputs := []string{
		"KEY PARTNERS",
		"KEY ACTIVITIES",
		"CUSTOMER RELATIONSHIPS",
		"BUSINESS MODEL CANVAS",
		"REVENUE STREAMS",
	}
	for _, in := range inputs {
		paras := ParseBulletText(in, BulletTextOptions{FontSize: 1200, InlineTags: true, DetectNumbered: true})
		if len(paras) != 1 {
			t.Fatalf("input %q: expected 1 paragraph, got %d", in, len(paras))
		}
		if len(paras[0].Runs) != 1 {
			t.Fatalf("input %q: expected 1 run, got %d", in, len(paras[0].Runs))
		}
		got := paras[0].Runs[0].Text
		if got != in {
			t.Errorf("input %q: ParseBulletText mutated text → %q", in, got)
		}
	}
}

func TestParseBulletText_NumberedDisabledByDefault(t *testing.T) {
	paras := ParseBulletText("1. Item one", BulletTextOptions{FontSize: 1400})
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Bullet != nil {
		t.Error("numbered list should NOT be detected when DetectNumbered is false")
	}
	if paras[0].Runs[0].Text != "1. Item one" {
		t.Errorf("text should be unmodified: got %q", paras[0].Runs[0].Text)
	}
}

func TestParseBulletText_NumberedEnabled(t *testing.T) {
	paras := ParseBulletText("1. Item one\n2. Item twelve", BulletTextOptions{
		FontSize:       1400,
		DetectNumbered: true,
	})
	if len(paras) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(paras))
	}
	for i, p := range paras {
		if p.Bullet == nil {
			t.Fatalf("paragraph %d: expected bullet", i)
		}
	}
	if paras[0].Runs[0].Text != "Item one" {
		t.Errorf("paragraph 0 text: got %q", paras[0].Runs[0].Text)
	}
	if paras[1].Runs[0].Text != "Item twelve" {
		t.Errorf("paragraph 1 text: got %q", paras[1].Runs[0].Text)
	}
	for i, p := range paras {
		if p.Bullet.AutoNum != "arabicPeriod" {
			t.Errorf("paragraph %d: bullet = %+v, want arabicPeriod", i, p.Bullet)
		}
	}
}

// A shape whose text is "2. Enabler bar: funded first" was written as an
// auto-numbered paragraph with the "2." stripped and no start value, so the
// slide showed "1. Enabler bar: funded first" (go-slide-creator-zdzk2). The
// renderer may only take over a number it will count to again.
func TestParseBulletText_NumbersRenderAsWritten(t *testing.T) {
	opts := BulletTextOptions{FontSize: 1400, DetectNumbered: true}
	type para struct {
		text string
		auto bool // auto-numbered by the renderer
	}
	cases := []struct {
		name string
		in   string
		want []para
	}{
		{"a lone numbered line keeps its number", "2. Enabler bar: funded first",
			[]para{{"2. Enabler bar: funded first", false}}},
		{"a lone line numbered one keeps its number too", "1. Enabler bar",
			[]para{{"1. Enabler bar", false}}},
		{"a list from one", "1. One\n2. Two\n3. Three",
			[]para{{"One", true}, {"Two", true}, {"Three", true}}},
		{"a list that starts at three keeps its typed numbers", "3. Third\n4. Fourth\n5. Fifth",
			[]para{{"3. Third", false}, {"4. Fourth", false}, {"5. Fifth", false}}},
		{"numbers that do not count up stay literal", "1. One\n12. Twelve",
			[]para{{"1. One", false}, {"12. Twelve", false}}},
		{"a line between two numbers breaks the list", "Intro\n2. Odd one\nMiddle\n7. Seven\n8. Eight",
			[]para{{"Intro", false}, {"2. Odd one", false}, {"Middle", false}, {"7. Seven", false}, {"8. Eight", false}}},
		{"a list after a heading line", "Steps\n1. One\n2. Two",
			[]para{{"Steps", false}, {"One", true}, {"Two", true}}},
		{"a repeated number ends the list", "1. One\n2. Two\n2. Two again",
			[]para{{"One", true}, {"Two", true}, {"2. Two again", false}}},
		{"a year is prose", "2024. A big year", []para{{"2024. A big year", false}}},
		{"a decimal is not a prefix", "2.5 points", []para{{"2.5 points", false}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paras := ParseBulletText(tc.in, opts)
			if len(paras) != len(tc.want) {
				t.Fatalf("got %d paragraphs, want %d", len(paras), len(tc.want))
			}
			for i, w := range tc.want {
				p := paras[i]
				if got := p.Runs[0].Text; got != w.text {
					t.Errorf("paragraph %d text = %q, want %q", i, got, w.text)
				}
				if !w.auto {
					if p.Bullet != nil {
						t.Errorf("paragraph %d is %+v, want plain text", i, p.Bullet)
					}
					continue
				}
				if p.Bullet == nil || p.Bullet.AutoNum != "arabicPeriod" {
					t.Errorf("paragraph %d bullet = %+v, want auto-numbering", i, p.Bullet)
				}
			}
		})
	}
}

func TestParseBulletText_MixedBulletsAndPlain(t *testing.T) {
	paras := ParseBulletText("Header\n- Bullet one\nFooter", BulletTextOptions{FontSize: 1400})
	if len(paras) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(paras))
	}
	if paras[0].Bullet != nil {
		t.Error("paragraph 0: expected no bullet")
	}
	if paras[1].Bullet == nil {
		t.Error("paragraph 1: expected bullet")
	}
	if paras[2].Bullet != nil {
		t.Error("paragraph 2: expected no bullet")
	}
}

func TestParseBulletText_StylingApplied(t *testing.T) {
	opts := BulletTextOptions{
		FontSize:    2000,
		Bold:        true,
		Italic:      true,
		Align:       "l",
		FontFamily:  "Calibri",
		Lang:        "en-US",
		Dirty:       true,
		BulletColor: SchemeFill("accent1"),
		SpaceAfter:  600,
	}
	paras := ParseBulletText("- Item", opts)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	p := paras[0]
	r := p.Runs[0]
	if r.FontSize != 2000 {
		t.Errorf("FontSize: got %d, want 2000", r.FontSize)
	}
	if !r.Bold {
		t.Error("expected bold")
	}
	if !r.Italic {
		t.Error("expected italic")
	}
	if r.FontFamily != "Calibri" {
		t.Errorf("FontFamily: got %q, want %q", r.FontFamily, "Calibri")
	}
	if r.Lang != "en-US" {
		t.Errorf("Lang: got %q", r.Lang)
	}
	if !r.Dirty {
		t.Error("expected dirty")
	}
	if p.Align != "l" {
		t.Errorf("Align: got %q, want %q", p.Align, "l")
	}
	if p.SpaceAfter != 600 {
		t.Errorf("SpaceAfter: got %d, want 600", p.SpaceAfter)
	}
	if p.Bullet.Color.IsZero() {
		t.Error("expected bullet color to be set")
	}
}

func TestParseNumberedPrefix(t *testing.T) {
	tests := []struct {
		line    string
		wantOK  bool
		wantNum int
		wantRem string
	}{
		{"1. First", true, 1, "First"},
		{"12. Twelfth", true, 12, "Twelfth"},
		{"0. Zero", true, 0, "Zero"},
		{"abc. Not", false, 0, ""},
		{"1.NoSpace", false, 0, ""},
		{". Leading dot", false, 0, ""},
		{"", false, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			num, rest, ok := ParseNumberedPrefix(tt.line)
			if ok != tt.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if ok {
				if num != tt.wantNum {
					t.Errorf("num: got %d, want %d", num, tt.wantNum)
				}
				if rest != tt.wantRem {
					t.Errorf("rest: got %q, want %q", rest, tt.wantRem)
				}
			}
		})
	}
}
