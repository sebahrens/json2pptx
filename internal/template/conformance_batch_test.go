package template

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func mandatoryStatus(checks []ConformanceCheck, role string) (ConformanceStatus, string) {
	for _, c := range checks {
		if c.Check == "Mandatory layout: "+role {
			return c.Status, c.Detail
		}
	}
	return "", ""
}

// Two Content shares the "content" tag and Closing the "title-slide" tag, but
// neither is what layout_id "content" / "title" resolves to, so template-check
// must not accept them for One Content / Title Slide (go-slide-creator-csclk.32).
func TestMandatoryLayoutsRequireGeneratorResolvableRoles(t *testing.T) {
	title := types.PlaceholderInfo{ID: "title", Type: types.PlaceholderTitle}
	layouts := []types.LayoutMetadata{
		{ID: "slideLayout3", Name: "Two Content", Tags: []string{"content", "two-column"}, Placeholders: []types.PlaceholderInfo{
			title, {ID: "body", Type: types.PlaceholderBody, Index: 1}, {ID: "body_2", Type: types.PlaceholderBody, Index: 2}}},
		{ID: "slideLayout5", Name: "Closing", Tags: []string{"title-slide", "closing"}, Placeholders: []types.PlaceholderInfo{
			title, {ID: "subtitle", Type: types.PlaceholderSubtitle, Index: 1}}},
	}
	checks := checkMandatoryLayouts(layouts)
	for _, role := range []string{CanonicalRoleOneContent, CanonicalRoleTitleSlide} {
		status, detail := mandatoryStatus(checks, role)
		if status != ConformanceStatusFail {
			t.Errorf("%s status = %q (%s), want FAIL", role, status, detail)
		}
	}
}

// A One Content layout named canonically but left untagged (oversized body
// font) fails with a capacity-specific detail (go-slide-creator-csclk.37).
func TestUntaggedOneContentFailsWithCapacityDetail(t *testing.T) {
	layouts := []types.LayoutMetadata{{ID: "slideLayout2", Name: "One Content", Placeholders: []types.PlaceholderInfo{
		{ID: "title", Type: types.PlaceholderTitle, FontSize: 4400},
		{ID: "body", Type: types.PlaceholderBody, Index: 1, FontSize: 4400, MaxChars: 60},
	}}}
	status, detail := mandatoryStatus(checkMandatoryLayouts(layouts), CanonicalRoleOneContent)
	if status != ConformanceStatusFail || !strings.Contains(detail, "reduce the first-level body font size") {
		t.Errorf("One Content = %q (%s), want FAIL naming the body font", status, detail)
	}
}

// An idx-less body placeholder defaults to idx 0 and used to sort ahead of the
// real first body, remapping body_2 onto body (go-slide-creator-csclk.38).
func TestIdxLessBodyPlaceholderIsRenumbered(t *testing.T) {
	one := 1
	shapes := []shapeXML{
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "title"}, Placeholder: &placeholderXML{Type: "title"}}},
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "body"}, Placeholder: &placeholderXML{Type: "body", Index: &one}}},
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "body_2"}, Placeholder: &placeholderXML{Type: "body"}}},
	}
	phs := extractPlaceholders(shapes, "Two Content", nil, nil, nil, nil)
	if len(phs) != 3 || phs[2].ID != "body_2" || phs[2].Index <= phs[1].Index {
		t.Fatalf("placeholders = %+v, want body_2 renumbered after body", phs)
	}
}

func TestNormalizeRGBRejectsNonHex(t *testing.T) {
	if got := normalizeRGB("GGHHII"); got != "" {
		t.Errorf("normalizeRGB(GGHHII) = %q, want empty", got)
	}
	if got := normalizeRGB("2e5090"); got != "#2E5090" {
		t.Errorf("normalizeRGB(2e5090) = %q, want #2E5090", got)
	}
}

func TestPlaceholdersOnCanvasWarnsOnOverflow(t *testing.T) {
	layouts := []types.LayoutMetadata{{Name: "One Content", Placeholders: []types.PlaceholderInfo{
		{ID: "body", Bounds: types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}},
	}}}
	checks := checkPlaceholdersOnCanvas(layouts, 9144000, 6858000)
	if len(checks) != 1 || checks[0].Status != ConformanceStatusWarn {
		t.Fatalf("checks = %+v, want one WARN on a 4:3 canvas", checks)
	}
	if checks := checkPlaceholdersOnCanvas(layouts, 12192000, 6858000); checks[0].Status != ConformanceStatusPass {
		t.Errorf("16:9 canvas = %+v, want PASS", checks)
	}
}
