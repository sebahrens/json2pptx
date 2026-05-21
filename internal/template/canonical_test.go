package template_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ph is a terse constructor for a placeholder fixture.
func ph(id string, t types.PlaceholderType, x, y, w, h int64, maxChars, fontSize int) types.PlaceholderInfo {
	return types.PlaceholderInfo{
		ID:       id,
		Type:     t,
		Bounds:   types.BoundingBox{X: x, Y: y, Width: w, Height: h},
		MaxChars: maxChars,
		FontSize: fontSize,
	}
}

// TestClassifyLayoutCanonical_Fixtures pins one synthetic layout per canonical
// type to its expected ClassifyLayoutCanonical result. ClassifyLayoutCanonical is
// a typed facade over ClassifyCanonicalRole, so this also guards that the wire
// IDs stay aligned with types.CanonicalLayoutType.
func TestClassifyLayoutCanonical_Fixtures(t *testing.T) {
	const (
		bigBody = 3000000 // ~3.3in: usable body width/height
	)
	cases := []struct {
		name   string
		layout types.LayoutMetadata
		want   types.CanonicalLayoutType
	}{
		{
			name: "title slide",
			layout: types.LayoutMetadata{Name: "Title Slide", Placeholders: []types.PlaceholderInfo{
				ph("title", types.PlaceholderTitle, 500000, 1000000, 4000000, 1000000, 80, 4000),
				ph("subtitle", types.PlaceholderSubtitle, 500000, 2200000, 4000000, 600000, 60, 2000),
			}},
			want: types.CanonicalLayoutTitleSlide,
		},
		{
			name: "one content",
			layout: types.LayoutMetadata{Name: "One Content", Placeholders: []types.PlaceholderInfo{
				ph("title", types.PlaceholderTitle, 500000, 300000, 8000000, 800000, 80, 3200),
				ph("body", types.PlaceholderBody, 500000, 1300000, bigBody, bigBody, 400, 1800),
			}},
			want: types.CanonicalLayoutOneContent,
		},
		{
			name: "two content",
			layout: types.LayoutMetadata{Name: "Two Content", Placeholders: []types.PlaceholderInfo{
				ph("title", types.PlaceholderTitle, 500000, 300000, 8000000, 800000, 80, 3200),
				ph("body", types.PlaceholderBody, 500000, 1300000, bigBody, bigBody, 400, 1800),
				ph("body_2", types.PlaceholderBody, 4800000, 1300000, bigBody, bigBody, 400, 1800),
			}},
			want: types.CanonicalLayoutTwoContent,
		},
		{
			name: "section divider",
			layout: types.LayoutMetadata{Name: "Section Divider", Placeholders: []types.PlaceholderInfo{
				ph("title", types.PlaceholderTitle, 500000, 1000000, 8000000, 800000, 80, 3600),
				ph("Section Number", types.PlaceholderBody, 500000, 2000000, 1000000, 1500000, 0, 14400),
			}},
			want: types.CanonicalLayoutSectionDivider,
		},
		{
			name:   "blank",
			layout: types.LayoutMetadata{Name: "Blank", Placeholders: nil},
			want:   types.CanonicalLayoutBlank,
		},
		{
			name: "blank + title",
			layout: types.LayoutMetadata{Name: "Blank + Title", Placeholders: []types.PlaceholderInfo{
				ph("title", types.PlaceholderTitle, 500000, 300000, 8000000, 800000, 80, 3200),
			}},
			want: types.CanonicalLayoutBlankTitle,
		},
		{
			name: "closing",
			layout: types.LayoutMetadata{Name: "Closing", Placeholders: []types.PlaceholderInfo{
				ph("title", types.PlaceholderTitle, 500000, 1000000, 8000000, 1000000, 80, 4000),
				ph("subtitle", types.PlaceholderSubtitle, 500000, 2200000, 8000000, 600000, 60, 2000),
			}},
			want: types.CanonicalLayoutClosing,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			layout := tc.layout
			// Tags drive the name-based canonical branches (section/closing).
			template.ClassifyLayout(&layout)
			got, conf := template.ClassifyLayoutCanonical(&layout)
			if got != tc.want {
				t.Fatalf("ClassifyLayoutCanonical(%s) = %q (conf %.2f), want %q",
					tc.name, got, conf, tc.want)
			}
			if conf <= 0 {
				t.Errorf("ClassifyLayoutCanonical(%s) confidence = %.2f, want > 0", tc.name, conf)
			}
			if fam := got.Family(); fam == "" {
				t.Errorf("Family() for %q returned empty", got)
			}
		})
	}
}

// TestClassifyPlaceholderRole pins each placeholder fixture to its expected
// canonical role, exercising every PlaceholderRole the classifier can produce.
func TestClassifyPlaceholderRole(t *testing.T) {
	cases := []struct {
		name string
		in   types.PlaceholderInfo
		want types.PlaceholderRole
	}{
		{"title", ph("Title 1", types.PlaceholderTitle, 0, 0, 0, 0, 80, 3600), types.PlaceholderRoleTitle},
		{"title named eyebrow -> eyebrow", ph("Eyebrow 1", types.PlaceholderTitle, 0, 0, 0, 0, 20, 1000), types.PlaceholderRoleEyebrow},
		{"subtitle type", ph("Subtitle 2", types.PlaceholderSubtitle, 0, 0, 0, 0, 40, 2000), types.PlaceholderRoleSubtitle},
		{"body", ph("body", types.PlaceholderBody, 0, 0, 0, 0, 400, 1800), types.PlaceholderRoleBody},
		{"content", ph("Content Placeholder 2", types.PlaceholderContent, 0, 0, 0, 0, 400, 1800), types.PlaceholderRoleBody},
		{"section number by alias id", ph("section_number", types.PlaceholderBody, 0, 0, 0, 0, 0, 0), types.PlaceholderRoleSectionNumber},
		{"section number by name", ph("Section Number 3", types.PlaceholderBody, 0, 0, 0, 0, 0, 0), types.PlaceholderRoleSectionNumber},
		{"section number by font", ph("Big Number", types.PlaceholderBody, 0, 0, 0, 0, 5, 14400), types.PlaceholderRoleSectionNumber},
		{"body named eyebrow -> eyebrow", ph("Eyebrow Text", types.PlaceholderBody, 0, 0, 0, 0, 30, 1000), types.PlaceholderRoleEyebrow},
		{"body named subtitle -> subtitle", ph("Subtitle Caption", types.PlaceholderBody, 0, 0, 0, 0, 60, 1600), types.PlaceholderRoleSubtitle},
		{"image", ph("Picture 1", types.PlaceholderImage, 0, 0, 0, 0, 0, 0), types.PlaceholderRoleImage},
		{"chart", ph("Chart 1", types.PlaceholderChart, 0, 0, 0, 0, 0, 0), types.PlaceholderRoleChart},
		{"table -> body", ph("Table 1", types.PlaceholderTable, 0, 0, 0, 0, 0, 0), types.PlaceholderRoleBody},
		{"other date", ph("Date Placeholder 3", types.PlaceholderOther, 0, 0, 0, 0, 0, 1200), types.PlaceholderRoleDate},
		{"other slide number", ph("Slide Number Placeholder 5", types.PlaceholderOther, 0, 0, 0, 0, 0, 1200), types.PlaceholderRolePageNumber},
		{"other footer", ph("Footer Placeholder 4", types.PlaceholderOther, 0, 0, 0, 0, 0, 1200), types.PlaceholderRoleFooter},
		{"other header -> other", ph("Header Placeholder 2", types.PlaceholderOther, 0, 0, 0, 0, 0, 1200), types.PlaceholderRoleOther},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, conf := template.ClassifyPlaceholderRole(tc.in, nil)
			if got != tc.want {
				t.Fatalf("ClassifyPlaceholderRole(%q, %s) = %q, want %q", tc.in.ID, tc.in.Type, got, tc.want)
			}
			if conf <= 0 {
				t.Errorf("confidence for %q = %.2f, want > 0", tc.name, conf)
			}
		})
	}
}

// TestCanonicalCoverageCorpus asserts every shipped template provides at least
// one layout in each content-bearing canonical family (title-slide,
// section-divider, one-content, qa-closing) — i.e. 100% canonical coverage.
func TestCanonicalCoverageCorpus(t *testing.T) {
	files := shippedTemplates(t)
	required := []types.CanonicalLayoutFamily{
		types.LayoutFamilyTitleSlide,
		types.LayoutFamilySectionDivider,
		types.LayoutFamilyOneContent,
		types.LayoutFamilyQAClosing,
	}

	for _, file := range files {
		file := file
		t.Run(filepath.Base(file), func(t *testing.T) {
			layouts := parseTemplate(t, file)
			cov := template.CanonicalFamilyCoverage(layouts)
			for _, fam := range required {
				if len(cov[fam]) == 0 {
					t.Errorf("template %s has no layout covering canonical family %q", filepath.Base(file), fam)
				}
			}
		})
	}
}

// TestParseLayoutsAssignsRoles verifies ParseLayouts persists canonical types on
// layouts and roles on placeholders, so all downstream consumers see them.
func TestParseLayoutsAssignsRoles(t *testing.T) {
	layouts := parseTemplate(t, filepath.Join(templatesDir, "midnight-blue.pptx"))

	var sawCanonical, sawRole bool
	for _, l := range layouts {
		if l.CanonicalType != types.CanonicalLayoutUnknown {
			sawCanonical = true
			if l.CanonicalConfidence <= 0 {
				t.Errorf("layout %q has CanonicalType %q but confidence %.2f", l.Name, l.CanonicalType, l.CanonicalConfidence)
			}
		}
		for _, p := range l.Placeholders {
			if p.Role != "" {
				sawRole = true
				if p.RoleConfidence <= 0 {
					t.Errorf("placeholder %q (layout %q) has Role %q but confidence %.2f", p.ID, l.Name, p.Role, p.RoleConfidence)
				}
			}
		}
	}
	if !sawCanonical {
		t.Error("ParseLayouts assigned no CanonicalType to any layout")
	}
	if !sawRole {
		t.Error("ParseLayouts assigned no Role to any placeholder")
	}
}

// TestDerivableLayoutsCorpus asserts the required derivable layouts (two-content
// and blank-title) are ready on at least N-1 of the shipped templates, matching
// the acceptance criterion "ready on at least 8/9 templates".
func TestDerivableLayoutsCorpus(t *testing.T) {
	files := shippedTemplates(t)
	min := len(files) - 1

	readyCount := map[string]int{}
	for _, file := range files {
		layouts := parseTemplate(t, file)
		for _, d := range template.DerivableLayouts(layouts) {
			if d.Ready {
				readyCount[d.Name]++
			}
		}
	}

	for _, name := range []string{"two-content", "blank-title"} {
		if readyCount[name] < min {
			t.Errorf("derivable %q ready on %d/%d templates, want >= %d", name, readyCount[name], len(files), min)
		}
	}
}

// TestDerivableLayouts_MissingFindings verifies an empty layout set reports every
// derivable layout as not-ready with a specific (non-empty) missing reason.
func TestDerivableLayouts_MissingFindings(t *testing.T) {
	got := template.DerivableLayouts(nil)
	if len(got) == 0 {
		t.Fatal("DerivableLayouts(nil) returned no entries")
	}
	for _, d := range got {
		if d.Ready {
			t.Errorf("derivable %q unexpectedly ready for empty template", d.Name)
		}
		if len(d.Missing) == 0 {
			t.Errorf("derivable %q not ready but emitted no specific missing finding", d.Name)
		}
	}
}

// shippedTemplates returns the sorted list of templates/*.pptx files.
func shippedTemplates(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", templatesDir)
	}
	sort.Strings(files)
	return files
}

// parseTemplate opens and parses a template, failing the test on error.
func parseTemplate(t *testing.T, file string) []types.LayoutMetadata {
	t.Helper()
	reader, err := template.OpenTemplate(file)
	if err != nil {
		t.Fatalf("OpenTemplate(%s): %v", file, err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("ParseLayouts(%s): %v", file, err)
	}
	return layouts
}
