package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

// chromeSkipLayouts returns a small layout set covering the four canonical types
// that matter for chrome skipping, including the title-only Section Divider that
// also carries the structural "title-slide" tag (the historical divergence).
func chromeSkipLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{ID: "slideLayout1", Name: "Title Slide", Tags: []string{"title-slide"}, CanonicalType: types.CanonicalLayoutTitleSlide},
		{ID: "slideLayout2", Name: "Closing", Tags: []string{"title-slide", "closing"}, CanonicalType: types.CanonicalLayoutClosing},
		{ID: "slideLayout3", Name: "Section Divider", Tags: []string{"title-slide", "section-header"}, CanonicalType: types.CanonicalLayoutSectionDivider},
		{ID: "slideLayout4", Name: "Content", Tags: []string{"content"}, CanonicalType: types.CanonicalLayoutOneContent},
	}
}

func chromeSkipSpecs() []generator.SlideSpec {
	return []generator.SlideSpec{
		{LayoutID: "slideLayout1"},
		{LayoutID: "slideLayout2"},
		{LayoutID: "slideLayout3"},
		{LayoutID: "slideLayout4"},
	}
}

func TestApplyChromeSkip_CanonicalTaxonomy(t *testing.T) {
	skipPtr := func(s []string) *ChromeInput {
		return &ChromeInput{PageNumbers: &PageNumbersInput{Skip: s}}
	}

	tests := []struct {
		name   string
		chrome *ChromeInput
		// want SkipFooter per layout: [title, closing, section-divider, content]
		want [4]bool
	}{
		{
			// Default (no explicit skip) = title + closing. The Section Divider
			// classifies canonically as Section Divider — not Title Slide — so it
			// is NOT skipped despite carrying the structural "title-slide" tag.
			name:   "default skips title and closing only",
			chrome: &ChromeInput{},
			want:   [4]bool{true, true, false, false},
		},
		{
			name:   "explicit title only",
			chrome: skipPtr([]string{"title"}),
			want:   [4]bool{true, false, false, false},
		},
		{
			name:   "explicit closing only",
			chrome: skipPtr([]string{"closing"}),
			want:   [4]bool{false, true, false, false},
		},
		{
			// Arbitrary structural tags still match against layout.Tags, so a
			// caller can opt section dividers back out of chrome.
			name:   "arbitrary section-header tag",
			chrome: skipPtr([]string{"section-header"}),
			want:   [4]bool{false, false, true, false},
		},
		{
			name:   "well-known names plus arbitrary tag combine",
			chrome: skipPtr([]string{"title", "closing", "section-header"}),
			want:   [4]bool{true, true, true, false},
		},
		{
			// Empty skip list means skip nothing.
			name:   "empty skip list skips nothing",
			chrome: skipPtr([]string{}),
			want:   [4]bool{false, false, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := chromeSkipSpecs()
			applyChromeSkip(specs, tt.chrome, nil, chromeSkipLayouts())
			for i := range tt.want {
				if specs[i].SkipFooter != tt.want[i] {
					t.Errorf("layout %s: SkipFooter = %v, want %v",
						specs[i].LayoutID, specs[i].SkipFooter, tt.want[i])
				}
			}
		})
	}
}

// TestApplyChromeSkip_EmptySkipListVsNil distinguishes a nil Skip (use default)
// from an explicitly empty Skip slice (skip nothing).
func TestApplyChromeSkip_EmptySkipListVsNil(t *testing.T) {
	// nil PageNumbers → default title+closing.
	specs := chromeSkipSpecs()
	applyChromeSkip(specs, &ChromeInput{PageNumbers: nil}, nil, chromeSkipLayouts())
	if !specs[0].SkipFooter || !specs[1].SkipFooter {
		t.Errorf("nil page_numbers should default-skip title and closing")
	}
	if specs[2].SkipFooter || specs[3].SkipFooter {
		t.Errorf("nil page_numbers must not skip section divider or content")
	}
}
