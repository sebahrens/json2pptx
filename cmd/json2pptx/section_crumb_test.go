package main

import (
	"strings"
	"testing"
)

// go-slide-creator-ynfv. get_capabilities advertised section_crumb as supported
// and SKILL.md repeated the claim, but ChromeInput.SectionCrumb was read and
// then referenced nowhere: composeChromeLine ignored it and FooterConfig had a
// single deck-wide string with nowhere to vary. Every slide's footer read
// "Confidential — Acme | Sept 2026".

func crumbStructure() *StructureInput {
	cover := SlideInput{SlideType: "title"}
	closing := SlideInput{SlideType: "title"}
	mk := func(t string) SlideInput {
		v := t
		return SlideInput{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &v}}}
	}
	return &StructureInput{
		Cover:      &cover,
		AutoAgenda: true,
		Closing:    &closing,
		Sections: []SectionInput{
			{Title: "Market context", Slides: []SlideInput{mk("Size"), mk("Growth")}},
			{Title: "Where we win", Slides: []SlideInput{mk("Edge")}},
		},
	}
}

// TestExpandStructureStampsSectionTitle pins where the crumb comes from: every
// content slide knows its section, and the slides outside one do not.
func TestExpandStructureStampsSectionTitle(t *testing.T) {
	slides, err := expandStructure(crumbStructure())
	if err != nil {
		t.Fatal(err)
	}
	// cover, agenda, divider, Size, Growth, divider, Edge, closing
	want := []string{"", "", "", "Market context", "Market context", "", "Where we win", ""}
	if len(slides) != len(want) {
		t.Fatalf("expanded %d slides, want %d", len(slides), len(want))
	}
	for i, w := range want {
		if slides[i].SectionTitle != w {
			t.Errorf("slide %d SectionTitle = %q, want %q", i, slides[i].SectionTitle, w)
		}
	}
}

// TestSectionCrumbFooterLines is the fix itself: a per-slide footer line.
func TestSectionCrumbFooterLines(t *testing.T) {
	slides, err := expandStructure(crumbStructure())
	if err != nil {
		t.Fatal(err)
	}
	cfg := chromeToFooterConfig(&ChromeInput{
		Confidentiality: "Confidential",
		ClientName:      "Acme",
		FooterDate:      "Sept 2026",
		SectionCrumb:    true,
	}, len(slides), slides)

	base := "Confidential — Acme | Sept 2026"
	if cfg.LeftText != base {
		t.Fatalf("deck-wide line = %q, want %q", cfg.LeftText, base)
	}
	cases := map[int]string{
		0: base,                       // cover
		2: base,                       // divider announces the section in 60pt
		3: base + " | Market context", // first content slide of the section
		4: base + " | Market context", // and the second
		6: base + " | Where we win",   // next section
		7: base,                       // closing
	}
	for idx, want := range cases {
		if got := cfg.LeftTextFor(idx); got != want {
			t.Errorf("slide %d footer = %q, want %q", idx, got, want)
		}
	}
}

// TestSectionCrumbOffIsUnchanged is the guard that matters most: a deck that
// does not ask for a crumb must generate exactly as before.
func TestSectionCrumbOffIsUnchanged(t *testing.T) {
	slides, err := expandStructure(crumbStructure())
	if err != nil {
		t.Fatal(err)
	}
	cfg := chromeToFooterConfig(&ChromeInput{ClientName: "Acme"}, len(slides), slides)
	if cfg.LeftTextBySlide != nil {
		t.Errorf("section_crumb off should leave no per-slide lines, got %v", cfg.LeftTextBySlide)
	}
	for i := range slides {
		if got := cfg.LeftTextFor(i); got != "Acme" {
			t.Errorf("slide %d footer = %q, want the deck-wide line", i, got)
		}
	}
}

// TestSectionCrumbWithoutStructureIsInert covers a deck that sets the flag on a
// flat slides[] list: no slide belongs to a section, so nothing varies.
func TestSectionCrumbWithoutStructureIsInert(t *testing.T) {
	slides := []SlideInput{{SlideType: "content"}, {SlideType: "content"}}
	cfg := chromeToFooterConfig(&ChromeInput{ClientName: "Acme", SectionCrumb: true}, len(slides), slides)
	if cfg.LeftTextBySlide != nil {
		t.Errorf("no sections means no per-slide lines, got %v", cfg.LeftTextBySlide)
	}
	if got := cfg.LeftTextFor(0); got != "Acme" {
		t.Errorf("footer = %q, want the deck-wide line", got)
	}
}

// TestSectionCrumbWithEmptyChromeLine: a deck whose only chrome is the crumb
// gets the section title alone, not a line starting with " | ".
func TestSectionCrumbWithEmptyChromeLine(t *testing.T) {
	slides, err := expandStructure(crumbStructure())
	if err != nil {
		t.Fatal(err)
	}
	cfg := chromeToFooterConfig(&ChromeInput{SectionCrumb: true}, len(slides), slides)
	if got := cfg.LeftTextFor(3); got != "Market context" {
		t.Errorf("footer = %q, want just the section title", got)
	}
	if strings.HasPrefix(cfg.LeftTextFor(3), "|") || strings.Contains(cfg.LeftTextFor(3), "  ") {
		t.Errorf("footer = %q has a dangling separator", cfg.LeftTextFor(3))
	}
}
