package generator

import (
	"testing"
)

// TestBulletIndentDepthMapsToParagraphLevel pins the depth→level mapping. The
// renderer used to emit every bullet at the layout's first bullet level, so an
// authored "\tSecond level" rendered with the parent's glyph and size and the
// text pushed right — it read as a typo (go-slide-creator-gyfl).
func TestBulletParagraphLevel(t *testing.T) {
	tests := []struct {
		base, depth, want int
	}{
		{0, 0, 0},
		{0, 1, 1},
		{0, 2, 2},
		{1, 1, 2},
		// The slide master defines nine levels; past the fourth the glyphs and
		// sizes stop being distinguishable, so the mapping clamps.
		{0, 7, maxBulletNestingLevel},
		{2, 9, maxBulletNestingLevel},
		{0, -1, 0},
	}
	for _, tt := range tests {
		if got := bulletParagraphLevel(tt.base, tt.depth); got != tt.want {
			t.Errorf("bulletParagraphLevel(%d, %d) = %d, want %d", tt.base, tt.depth, got, tt.want)
		}
	}
}

// TestBulletNestingLevelsStripsIndent checks that the text handed to the run
// builder has the indent removed: the tab used to survive into <a:t>, which is
// why the glyph stayed left and the text moved right.
func TestBulletNestingLevelsStripsIndent(t *testing.T) {
	depths, texts := bulletNestingLevels([]string{
		"Revenue grew 18%",
		"\tEnterprise added $12.8M",
		"\t\tThree accounts are half of it",
		"    Two spaces per level",
		"  ",
	})
	wantDepths := []int{0, 1, 2, 2, 0}
	wantTexts := []string{
		"Revenue grew 18%",
		"Enterprise added $12.8M",
		"Three accounts are half of it",
		"Two spaces per level",
		"",
	}
	for i := range wantDepths {
		if depths[i] != wantDepths[i] {
			t.Errorf("bullet %d: depth = %d, want %d", i, depths[i], wantDepths[i])
		}
		if texts[i] != wantTexts[i] {
			t.Errorf("bullet %d: text = %q, want %q", i, texts[i], wantTexts[i])
		}
	}
}

// TestSetBulletParagraphsEmitsNestedLevels is the end-to-end check on the XML
// the renderer writes: three authored depths become three paragraph levels,
// and no tab survives into the text run.
func TestSetBulletParagraphsEmitsNestedLevels(t *testing.T) {
	shape := &shapeXML{TextBody: &textBodyXML{
		BodyProperties: &bodyPropertiesXML{},
		ListStyle:      &listStyleXML{},
		Paragraphs:     []paragraphXML{emptyParagraph()},
	}}
	bullets := []string{
		"Revenue grew 18% quarter over quarter",
		"\tEnterprise added $12.8M",
		"\t\tThree accounts are half of it",
		"Retention held at 118%",
	}
	if err := setBulletParagraphs(shape, "body", bullets, 0); err != nil {
		t.Fatalf("setBulletParagraphs: %v", err)
	}
	wantLevels := []int{0, 1, 2, 0}
	if len(shape.TextBody.Paragraphs) != len(wantLevels) {
		t.Fatalf("got %d paragraphs, want %d", len(shape.TextBody.Paragraphs), len(wantLevels))
	}
	for i, para := range shape.TextBody.Paragraphs {
		if para.Properties == nil || para.Properties.Level == nil {
			t.Fatalf("paragraph %d has no level", i)
		}
		if *para.Properties.Level != wantLevels[i] {
			t.Errorf("paragraph %d: lvl = %d, want %d", i, *para.Properties.Level, wantLevels[i])
		}
		for _, run := range para.Runs {
			if run.Text != "" && run.Text[0] == '\t' {
				t.Errorf("paragraph %d still carries the indent in its text: %q", i, run.Text)
			}
		}
	}
}
