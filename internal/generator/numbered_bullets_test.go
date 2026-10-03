package generator

import (
	"strings"
	"testing"
)

// bullets_value ["1. First", "2. Then"] rendered as "• 1. First": the layout's
// glyph AND the author's number. Auto-numbering existed but only the shape_grid
// path ever asked for it (go-slide-creator-6or2).
func TestNumberedList(t *testing.T) {
	cases := []struct {
		name    string
		bullets []string
		want    []string
	}{
		{
			name:    "a complete ordered list",
			bullets: []string{"1. Freeze the schema", "2. Replay the log", "3. Cut over"},
			want:    []string{"Freeze the schema", "Replay the log", "Cut over"},
		},
		{"partly numbered", []string{"1. Freeze", "Replay", "3. Cut over"}, nil},
		{"starts at two", []string{"2. Replay", "3. Cut over"}, nil},
		{"starts at zero", []string{"0. Freeze", "1. Replay"}, nil},
		{"repeats a number", []string{"1. Freeze", "1. Replay"}, nil},
		{"skips a number", []string{"1. Freeze", "3. Cut over"}, nil},
		{"a lone line that opens with a number is prose", []string{"2024. A big year"}, nil},
		{"a prefix with no text", []string{"1. ", "2. Replay"}, nil},
		{"an unnumbered list", []string{"Freeze", "Replay"}, nil},
		{"empty", nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, numbered := NumberedList(tc.bullets)
			if (tc.want == nil) == numbered {
				t.Fatalf("NumberedList(%v) numbered = %v, want %v", tc.bullets, numbered, tc.want != nil)
			}
			if tc.want == nil {
				return
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("stripped = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasNumberedPrefix(t *testing.T) {
	yes := []string{"1. Freeze", "12. Twelve", " 3. Padded"}
	no := []string{"Freeze", "1.No space", "- 1. dash first", "2024"}
	for _, s := range yes {
		if !HasNumberedPrefix(s) {
			t.Errorf("HasNumberedPrefix(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if HasNumberedPrefix(s) {
			t.Errorf("HasNumberedPrefix(%q) = true, want false", s)
		}
	}
}

// The inherited glyph must be REPLACED, not joined: a paragraph carrying both
// buChar and buAutoNum is what produced the double marker in the first place.
// The marker also has to land in a valid position in pPr's child sequence.
func TestApplyAutoNumbering(t *testing.T) {
	cases := []struct {
		name  string
		inner string
		want  string
	}{
		{
			name:  "replaces an inherited glyph",
			inner: `<a:buFont typeface="Arial"/><a:buChar char="•"/>`,
			want:  `<a:buFont typeface="Arial"/><a:buAutoNum type="arabicPeriod"/>`,
		},
		{
			name:  "replaces buNone",
			inner: `<a:buNone/>`,
			want:  `<a:buAutoNum type="arabicPeriod"/>`,
		},
		{
			name:  "keeps the marker before defRPr",
			inner: `<a:buChar char="•"/><a:defRPr sz="1800"/>`,
			want:  `<a:buAutoNum type="arabicPeriod"/><a:defRPr sz="1800"/>`,
		},
		{
			name:  "empty properties",
			inner: ``,
			want:  `<a:buAutoNum type="arabicPeriod"/>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pProps := &paragraphPropertiesXML{Inner: tc.inner}
			applyAutoNumbering(pProps)
			if pProps.Inner != tc.want {
				t.Errorf("inner = %q, want %q", pProps.Inner, tc.want)
			}
			if strings.Count(pProps.Inner, "buChar") != 0 {
				t.Error("the inherited glyph survived beside the auto-number")
			}
		})
	}
}

// End to end through the placeholder bullets path: the numbers come from OOXML
// and the typed prefixes are gone from the text.
func TestSetBulletParagraphsNumbersOrderedLists(t *testing.T) {
	shape := &shapeXML{TextBody: &textBodyXML{}}
	bullets := []string{"1. Freeze the schema", "2. Replay the log", "3. Cut over"}
	if err := setBulletParagraphs(shape, "body", bullets, 0); err != nil {
		t.Fatalf("setBulletParagraphs: %v", err)
	}

	paragraphs := shape.TextBody.Paragraphs
	if len(paragraphs) != 3 {
		t.Fatalf("got %d paragraphs, want 3", len(paragraphs))
	}
	for i, para := range paragraphs {
		if para.Properties == nil || !strings.Contains(para.Properties.Inner, "buAutoNum") {
			t.Errorf("paragraph %d carries no auto-numbering", i)
		}
		for _, run := range para.Runs {
			if HasNumberedPrefix(run.Text) {
				t.Errorf("paragraph %d still carries the typed prefix: %q", i, run.Text)
			}
		}
	}

	// A list continued from an earlier slide keeps the numbers the author
	// typed, as its only marker: no auto-numbering (which would count from 1)
	// and no layout glyph beside the number (go-slide-creator-zdzk2).
	continued := &shapeXML{TextBody: &textBodyXML{}}
	if err := setBulletParagraphs(continued, "body", []string{"4. Fourth step", "5. Fifth step"}, 0); err != nil {
		t.Fatalf("setBulletParagraphs: %v", err)
	}
	for i, want := range []string{"4. Fourth step", "5. Fifth step"} {
		para := continued.TextBody.Paragraphs[i]
		if para.Properties == nil || strings.Contains(para.Properties.Inner, "buAutoNum") ||
			strings.Contains(para.Properties.Inner, "buChar") || !strings.Contains(para.Properties.Inner, "<a:buNone/>") {
			t.Errorf("continued paragraph %d should carry buNone only: %+v", i, para.Properties)
		}
		if len(para.Runs) == 0 || para.Runs[0].Text != want {
			t.Errorf("continued paragraph %d text = %+v, want %q", i, para.Runs, want)
		}
	}
	if _, start, ok := NumberedListStart([]string{"4. Fourth step", "5. Fifth step"}); !ok || start != 4 {
		t.Errorf("NumberedListStart = %d, %v; want 4, true", start, ok)
	}
	if _, _, ok := NumberedListStart([]string{"4. Fourth step", "6. Sixth step"}); ok {
		t.Error("a list that skips a number is not an ordered list")
	}

	// A lone bullet that opens with a number is prose and keeps its number.
	lone := &shapeXML{TextBody: &textBodyXML{}}
	if err := setBulletParagraphs(lone, "body", []string{"2. Enabler bar: funded first"}, 0); err != nil {
		t.Fatalf("setBulletParagraphs: %v", err)
	}
	if p := lone.TextBody.Paragraphs[0]; (p.Properties != nil && strings.Contains(p.Properties.Inner, "buAutoNum")) ||
		len(p.Runs) == 0 || !strings.HasPrefix(p.Runs[0].Text, "2. ") {
		t.Errorf("a lone numbered bullet lost its literal number: %+v", p)
	}

	// An unordered list is untouched.
	plain := &shapeXML{TextBody: &textBodyXML{}}
	if err := setBulletParagraphs(plain, "body", []string{"Freeze", "Replay"}, 0); err != nil {
		t.Fatalf("setBulletParagraphs: %v", err)
	}
	for i, para := range plain.TextBody.Paragraphs {
		if para.Properties != nil && strings.Contains(para.Properties.Inner, "buAutoNum") {
			t.Errorf("paragraph %d of an unordered list was auto-numbered", i)
		}
	}
}
