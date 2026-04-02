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
	paras := ParseBulletText("1. Item one\n12. Item twelve", BulletTextOptions{
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
