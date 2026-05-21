package examine

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func itoa(n int) string { return strconv.Itoa(n) }

// fullCoverageLayouts returns one layout per content-bearing canonical family.
func fullCoverageLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{ID: "slideLayout1", Name: "Title Slide", CanonicalType: types.CanonicalLayoutTitleSlide, CanonicalConfidence: 0.9},
		{ID: "slideLayout2", Name: "One Content", CanonicalType: types.CanonicalLayoutOneContent, CanonicalConfidence: 0.9},
		{ID: "slideLayout3", Name: "Section Divider", CanonicalType: types.CanonicalLayoutSectionDivider, CanonicalConfidence: 0.9},
		{ID: "slideLayout4", Name: "Closing", CanonicalType: types.CanonicalLayoutClosing, CanonicalConfidence: 0.9},
	}
}

func hasFinding(r *Report, code string) bool {
	for _, f := range r.Findings.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestBuildReport_FullCoverageHasNoMissingRole(t *testing.T) {
	r := BuildReport(Inputs{
		Template:       "full.pptx",
		SlideWidthEMU:  defaultSlideWidthEMU,
		SlideHeightEMU: defaultSlideHeightEMU,
		Layouts:        fullCoverageLayouts(),
	})

	for _, fam := range []string{"title-slide", "section-divider", "one-content", "qa-closing"} {
		if !r.CanonicalCoverage[fam].Present {
			t.Errorf("family %q should be present", fam)
		}
	}
	if hasFinding(r, "TPL.LAYOUT.MISSING_ROLE") {
		t.Errorf("full-coverage template should not emit TPL.LAYOUT.MISSING_ROLE; findings=%+v", r.Findings.Findings)
	}
	if !r.Findings.OK {
		t.Errorf("findings.ok should be true for a clean template")
	}
}

func TestBuildReport_MissingSectionDividerEmitsFinding(t *testing.T) {
	layouts := fullCoverageLayouts()
	// Drop the Section Divider layout.
	pruned := layouts[:0]
	for _, l := range layouts {
		if l.CanonicalType == types.CanonicalLayoutSectionDivider {
			continue
		}
		pruned = append(pruned, l)
	}

	r := BuildReport(Inputs{
		Template:       "synthetic.pptx",
		SlideWidthEMU:  defaultSlideWidthEMU,
		SlideHeightEMU: defaultSlideHeightEMU,
		Layouts:        pruned,
	})

	cov := r.CanonicalCoverage["section-divider"]
	if cov.Present {
		t.Errorf("section-divider should be absent")
	}
	if len(cov.Layouts) != 0 {
		t.Errorf("section-divider layouts should be empty, got %v", cov.Layouts)
	}

	var got *struct{ code, category, severity string }
	for _, f := range r.Findings.Findings {
		if f.Code == "TPL.LAYOUT.MISSING_ROLE" {
			got = &struct{ code, category, severity string }{f.Code, f.Category, string(f.Severity)}
			if fam, _ := f.Evidence["family"].(string); fam != "section-divider" {
				t.Errorf("finding evidence.family = %q, want section-divider", fam)
			}
		}
	}
	if got == nil {
		t.Fatalf("expected a TPL.LAYOUT.MISSING_ROLE finding; findings=%+v", r.Findings.Findings)
	}
	if got.category != "TPL" {
		t.Errorf("finding category = %q, want TPL", got.category)
	}
	if got.severity != "warning" {
		t.Errorf("finding severity = %q, want warning", got.severity)
	}
	// Other families remain present.
	if !r.CanonicalCoverage["title-slide"].Present {
		t.Errorf("title-slide should still be present")
	}
}

func TestRenderLayoutSVG_NumbersMatchReport(t *testing.T) {
	r := BuildReport(Inputs{
		Template:       "t.pptx",
		SlideWidthEMU:  defaultSlideWidthEMU,
		SlideHeightEMU: defaultSlideHeightEMU,
		Layouts: []types.LayoutMetadata{{
			ID:                  "slideLayout1",
			Name:                "One Content",
			CanonicalType:       types.CanonicalLayoutOneContent,
			CanonicalConfidence: 0.9,
			Placeholders: []types.PlaceholderInfo{
				{
					ID:             "title",
					Type:           types.PlaceholderTitle,
					Role:           types.PlaceholderRoleTitle,
					RoleConfidence: 0.95,
					FontSize:       4400,
					MaxChars:       29,
					Bounds:         types.BoundingBox{X: 838200, Y: 365125, Width: 10515600, Height: 1325563},
				},
				{
					ID:             "section_number",
					Type:           types.PlaceholderBody,
					Role:           types.PlaceholderRoleSectionNumber,
					RoleConfidence: 0.95,
					FontSize:       12000,
					MaxChars:       2,
					Bounds:         types.BoundingBox{X: 500000, Y: 500000, Width: 1500000, Height: 1500000},
				},
			},
		}},
	})

	l := r.Layouts[0]
	svg := RenderLayoutSVG(l, r.Slide, r.Theme)

	if !strings.HasPrefix(svg, "<svg") {
		t.Fatalf("SVG should start with <svg, got %.20q", svg)
	}

	// Every placeholder's report numbers must appear verbatim on the SVG.
	for _, p := range l.Placeholders {
		checks := []string{
			`data-ph-id="` + p.ID + `"`,
			`data-font-pt="` + fnum(p.FontPt) + `"`,
			`data-max-chars="` + itoa(p.MaxChars) + `"`,
			`data-z="` + itoa(p.ZIndex) + `"`,
			`data-x-in="` + fnum(p.Bounds.XIn) + `"`,
			`data-y-in="` + fnum(p.Bounds.YIn) + `"`,
			`data-w-in="` + fnum(p.Bounds.WIn) + `"`,
			`data-h-in="` + fnum(p.Bounds.HIn) + `"`,
		}
		for _, c := range checks {
			if !strings.Contains(svg, c) {
				t.Errorf("placeholder %q: SVG missing %s", p.ID, c)
			}
		}
	}

	// The section-number frame gets a badge.
	if !strings.Contains(svg, "#d62728") {
		t.Errorf("expected a section-number badge color in the SVG")
	}
}

func TestComputeZone_TitleBottomAndFooterTop(t *testing.T) {
	phs := []PlaceholderReport{
		{Role: "title", Bounds: BoundsReport{XEMU: 838200, YEMU: 365125, WEMU: 10515600, HEMU: 1325563}},
		{Role: "body", Bounds: BoundsReport{XEMU: 838200, YEMU: 1900000, WEMU: 10515600, HEMU: 4000000}},
		{Role: "footer", Bounds: BoundsReport{XEMU: 838200, YEMU: 6400000, WEMU: 3000000, HEMU: 300000}},
	}
	z := computeZone(phs, defaultSlideWidthEMU, defaultSlideHeightEMU)
	if z.TopEMU != 365125+1325563 {
		t.Errorf("zone top = %d, want title bottom %d", z.TopEMU, 365125+1325563)
	}
	if z.BottomEMU != 6400000 {
		t.Errorf("zone bottom = %d, want footer top %d", z.BottomEMU, 6400000)
	}
	if z.LeftEMU != 838200 {
		t.Errorf("zone left = %d, want body left %d", z.LeftEMU, 838200)
	}
}
