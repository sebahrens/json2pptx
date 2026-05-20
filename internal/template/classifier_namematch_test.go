package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// titleSlidePlaceholdersWithSubtitle returns the placeholders for a classic
// ctrTitle + subTitle layout (the structural fingerprint of a Title Slide,
// regardless of what the layout is named in the template).
func titleSlidePlaceholdersWithSubtitle() []types.PlaceholderInfo {
	return []types.PlaceholderInfo{
		{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 500000, Y: 1500000, Width: 8000000, Height: 1500000}},
		{Type: types.PlaceholderSubtitle, Bounds: types.BoundingBox{X: 500000, Y: 3500000, Width: 8000000, Height: 800000}},
	}
}

// titleSlidePlaceholdersTitleOnly returns the placeholders for a hero/cover
// slide that has a centered title but no subtitle.
func titleSlidePlaceholdersTitleOnly() []types.PlaceholderInfo {
	return []types.PlaceholderInfo{
		{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 500000, Y: 1500000, Width: 8000000, Height: 1500000}},
	}
}

func TestClassifyCanonicalRole_TitleSlideNameVariants(t *testing.T) {
	// ≥10 distinct names that designers commonly use for a structural Title
	// Slide. All MUST classify as the canonical "Title Slide" role with
	// confidence ≥ CanonicalConfidenceThreshold so that the repair pipeline
	// renames them rather than authoring duplicates.
	titleSlideNames := []string{
		"Title Slide",
		"Cover",
		"Cover Slide",
		"Hero",
		"Hero Slide",
		"Opening",
		"Opening Slide",
		"Intro",
		"Title",
		"Front Page",
		"Welcome",
	}

	for _, name := range titleSlideNames {
		t.Run("with-subtitle/"+name, func(t *testing.T) {
			layout := &types.LayoutMetadata{
				Name:         name,
				Placeholders: titleSlidePlaceholdersWithSubtitle(),
			}
			role, sig, conf := ClassifyCanonicalRole(layout)
			if role != CanonicalRoleTitleSlide {
				t.Fatalf("name=%q: role=%q, want %q (sig=%q, conf=%.2f)", name, role, CanonicalRoleTitleSlide, sig, conf)
			}
			if conf < CanonicalConfidenceThreshold {
				t.Fatalf("name=%q: conf=%.2f, want >= %.2f", name, conf, CanonicalConfidenceThreshold)
			}
			if sig == "" {
				t.Fatalf("name=%q: empty signature", name)
			}
		})

		t.Run("title-only/"+name, func(t *testing.T) {
			layout := &types.LayoutMetadata{
				Name:         name,
				Placeholders: titleSlidePlaceholdersTitleOnly(),
			}
			role, sig, conf := ClassifyCanonicalRole(layout)
			if role != CanonicalRoleTitleSlide {
				t.Fatalf("name=%q: role=%q, want %q (sig=%q, conf=%.2f)", name, role, CanonicalRoleTitleSlide, sig, conf)
			}
			if conf < CanonicalConfidenceThreshold {
				t.Fatalf("name=%q: conf=%.2f, want >= %.2f", name, conf, CanonicalConfidenceThreshold)
			}
		})
	}
}

func TestClassifyCanonicalRole_SectionDividerNameVariants(t *testing.T) {
	names := []string{
		"Section Divider",
		"Section Header",
		"Section break with image",
		"Section",
		"Chapter Break",
		"Divider",
		"Transition",
		"Part Break",
		"Section Title",
		"Section Intro",
		"Module Break",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			layout := &types.LayoutMetadata{
				Name: name,
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 500000, Y: 1500000, Width: 8000000, Height: 1500000}},
				},
			}
			role, _, conf := ClassifyCanonicalRole(layout)
			// Section, divider, break, transition keywords trigger
			// classifyByName → "section-header" tag. Names without those
			// keywords ("Chapter Break", "Module Break", "Part Break") still
			// match because "break" is in the section-header keyword list.
			// Names without any matching keyword fall back to Title Slide,
			// which is acceptable structurally — but we expect at least the
			// keyword-bearing ones to classify as Section Divider.
			if containsAnySectionKeyword(name) {
				if role != CanonicalRoleSectionDivider {
					t.Fatalf("name=%q: role=%q, want %q (conf=%.2f)", name, role, CanonicalRoleSectionDivider, conf)
				}
				if conf < CanonicalConfidenceThreshold {
					t.Fatalf("name=%q: conf=%.2f, want >= %.2f", name, conf, CanonicalConfidenceThreshold)
				}
			}
		})
	}
}

func TestClassifyCanonicalRole_ClosingNameVariants(t *testing.T) {
	names := []string{
		"Closing",
		"Thank You",
		"End Slide",
		"Conclusion",
		"Q&A",
		"Questions",
		"Final",
		"The End",
		"Closing Slide",
		"Thanks",
		"Final Slide",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			layout := &types.LayoutMetadata{
				Name: name,
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 500000, Y: 1500000, Width: 8000000, Height: 1500000}},
				},
			}
			role, _, conf := ClassifyCanonicalRole(layout)
			if role != CanonicalRoleClosing {
				t.Fatalf("name=%q: role=%q, want %q (conf=%.2f)", name, role, CanonicalRoleClosing, conf)
			}
			if conf < CanonicalConfidenceThreshold {
				t.Fatalf("name=%q: conf=%.2f, want >= %.2f", name, conf, CanonicalConfidenceThreshold)
			}
		})
	}
}

func TestClassifyCanonicalRole_TwoContent(t *testing.T) {
	layout := &types.LayoutMetadata{
		Name: "Compare and Contrast",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
		},
	}
	role, sig, conf := ClassifyCanonicalRole(layout)
	if role != CanonicalRoleTwoContent {
		t.Fatalf("role=%q, want %q (sig=%q, conf=%.2f)", role, CanonicalRoleTwoContent, sig, conf)
	}
	if conf < CanonicalConfidenceThreshold {
		t.Fatalf("conf=%.2f, want >= %.2f", conf, CanonicalConfidenceThreshold)
	}
	if !contains(sig, "side-by-side") {
		t.Errorf("signature %q should mark side-by-side bodies", sig)
	}
}

func TestClassifyCanonicalRole_OneContent(t *testing.T) {
	layout := &types.LayoutMetadata{
		Name: "Standard Content",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
			{Type: types.PlaceholderBody, MaxChars: 500, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
		},
	}
	role, _, conf := ClassifyCanonicalRole(layout)
	if role != CanonicalRoleOneContent {
		t.Fatalf("role=%q, want %q (conf=%.2f)", role, CanonicalRoleOneContent, conf)
	}
	if conf < CanonicalConfidenceThreshold {
		t.Fatalf("conf=%.2f, want >= %.2f", conf, CanonicalConfidenceThreshold)
	}
}

func TestClassifyCanonicalRole_Blank(t *testing.T) {
	layout := &types.LayoutMetadata{Name: "Empty", Placeholders: nil}
	role, sig, conf := ClassifyCanonicalRole(layout)
	if role != CanonicalRoleBlank {
		t.Fatalf("role=%q, want %q", role, CanonicalRoleBlank)
	}
	if sig != "blank" {
		t.Errorf("sig=%q, want %q", sig, "blank")
	}
	if conf < 0.99 {
		t.Errorf("conf=%.2f, want ~1.0", conf)
	}
}

func TestClassifyCanonicalRole_BlankTitle(t *testing.T) {
	layout := &types.LayoutMetadata{
		Name: "Blank + Title",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
		},
	}
	role, _, conf := ClassifyCanonicalRole(layout)
	if role != CanonicalRoleBlankTitle {
		t.Fatalf("role=%q, want %q (conf=%.2f)", role, CanonicalRoleBlankTitle, conf)
	}
	if conf < CanonicalConfidenceThreshold {
		t.Fatalf("conf=%.2f, want >= %.2f", conf, CanonicalConfidenceThreshold)
	}
}

func TestClassifyCanonicalRole_NoStructuralMatch(t *testing.T) {
	// A layout with only utility placeholders is not a canonical role.
	layout := &types.LayoutMetadata{
		Name: "Utility",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderOther, Bounds: types.BoundingBox{}},
		},
	}
	role, _, conf := ClassifyCanonicalRole(layout)
	if role != "" {
		t.Errorf("role=%q, want \"\" (conf=%.2f)", role, conf)
	}
}

func TestClassifyCanonicalRole_NilLayout(t *testing.T) {
	role, sig, conf := ClassifyCanonicalRole(nil)
	if role != "" || sig != "" || conf != 0 {
		t.Errorf("ClassifyCanonicalRole(nil) = (%q, %q, %.2f), want empty", role, sig, conf)
	}
}

func TestLayoutSignature_StructurallyEquivalentLayouts(t *testing.T) {
	// Two layouts with different names but identical placeholder structure
	// MUST produce the same signature so checkDuplicateLayoutSignatures can
	// detect them.
	a := &types.LayoutMetadata{Name: "Cover", Placeholders: titleSlidePlaceholdersWithSubtitle()}
	b := &types.LayoutMetadata{Name: "Title Slide", Placeholders: titleSlidePlaceholdersWithSubtitle()}

	_, sigA, _ := ClassifyCanonicalRole(a)
	_, sigB, _ := ClassifyCanonicalRole(b)
	if sigA != sigB {
		t.Errorf("structurally equivalent layouts produced different signatures: %q vs %q", sigA, sigB)
	}
	if sigA == "" || sigA == "blank" {
		t.Errorf("unexpected signature %q for title+subtitle layout", sigA)
	}
}

func TestCanonicalNameFor(t *testing.T) {
	if got := CanonicalNameFor(CanonicalRoleTitleSlide); got != "Title Slide" {
		t.Errorf("CanonicalNameFor(TitleSlide)=%q, want %q", got, "Title Slide")
	}
	if got := CanonicalNameFor("bogus"); got != "" {
		t.Errorf("CanonicalNameFor(bogus)=%q, want \"\"", got)
	}
}

func TestIsCanonicalLayoutName(t *testing.T) {
	cases := []struct {
		role string
		name string
		want bool
	}{
		{CanonicalRoleTitleSlide, "Title Slide", true},
		{CanonicalRoleTitleSlide, "title slide", true},
		{CanonicalRoleTitleSlide, "Cover", false},
		{CanonicalRoleTwoContent, "Two Content", true},
		{CanonicalRoleTwoContent, "Comparison", true},
		{CanonicalRoleTwoContent, "Compare", false},
		{CanonicalRoleClosing, "Thank You", true},
		{CanonicalRoleClosing, "End Slide", true},
		{CanonicalRoleClosing, "Goodbye", false},
		{"bogus", "anything", false},
	}
	for _, c := range cases {
		if got := IsCanonicalLayoutName(c.role, c.name); got != c.want {
			t.Errorf("IsCanonicalLayoutName(%q, %q)=%v, want %v", c.role, c.name, got, c.want)
		}
	}
}

// helpers

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// containsAnySectionKeyword reports whether the layout name contains any of
// the section-header trigger keywords ("section", "divider", "break",
// "transition") used by classifyByName.
func containsAnySectionKeyword(name string) bool {
	keywords := []string{"section", "divider", "break", "transition"}
	lower := toLowerASCII(name)
	for _, k := range keywords {
		if indexOf(lower, k) >= 0 {
			return true
		}
	}
	return false
}

func toLowerASCII(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
