package examine

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/types"
)

// conformantLayouts returns a minimal set of layouts that satisfies every gate
// check: one layout per content-bearing canonical family, all tagged, the
// title placeholders named "title", and the section divider carrying a
// "Section Number" placeholder. Individual tests mutate a copy to trip exactly
// one check.
func conformantLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{
			Index: 0, ID: "slideLayout1", Name: "Title Slide",
			CanonicalType: types.CanonicalLayoutTitleSlide,
			Tags:          []string{"title-slide"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "subtitle", Type: types.PlaceholderSubtitle},
			},
		},
		{
			Index: 1, ID: "slideLayout2", Name: "One Content",
			CanonicalType: types.CanonicalLayoutOneContent,
			Tags:          []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "body", Type: types.PlaceholderBody, Index: 1},
			},
		},
		{
			Index: 2, ID: "slideLayout3", Name: "Section Divider",
			CanonicalType: types.CanonicalLayoutSectionDivider,
			Tags:          []string{"section-header"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "Section Number", Type: types.PlaceholderBody, Index: 10},
			},
		},
		{
			Index: 3, ID: "slideLayout4", Name: "Closing",
			CanonicalType: types.CanonicalLayoutClosing,
			Tags:          []string{"closing"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "subtitle", Type: types.PlaceholderSubtitle},
			},
		},
	}
}

func conformantReport(layouts []types.LayoutMetadata) *Report {
	return BuildReport(Inputs{
		Template: "fixture.pptx",
		Layouts:  layouts,
	})
}

func codesOf(violations []GateViolation) map[string]int {
	out := map[string]int{}
	for _, v := range violations {
		out[v.Code]++
	}
	return out
}

func TestGatePassesConformantTemplate(t *testing.T) {
	report := conformantReport(conformantLayouts())
	if v := Gate(report); len(v) != 0 {
		t.Fatalf("expected no gate violations for a conformant template, got %d: %+v", len(v), v)
	}
}

func TestGateNilReport(t *testing.T) {
	if v := Gate(nil); v != nil {
		t.Fatalf("expected nil violations for nil report, got %+v", v)
	}
}

func TestGateEmptyTags(t *testing.T) {
	layouts := conformantLayouts()
	layouts[1].Tags = nil // One Content with no tags
	report := conformantReport(layouts)

	v := Gate(report)
	if got := codesOf(v)[GateCodeEmptyTags]; got != 1 {
		t.Fatalf("expected exactly 1 %s violation, got %d (all: %+v)", GateCodeEmptyTags, got, v)
	}
	if !mentions(v, GateCodeEmptyTags, "One Content") {
		t.Errorf("empty-tags violation should name the offending layout: %+v", v)
	}
}

func TestGateCanonicalCoverageIncomplete(t *testing.T) {
	// Drop the section divider entirely.
	layouts := conformantLayouts()
	layouts = append(layouts[:2], layouts[3:]...)
	report := conformantReport(layouts)

	v := Gate(report)
	if got := codesOf(v)[GateCodeCanonicalCoverage]; got != 1 {
		t.Fatalf("expected exactly 1 %s violation, got %d (all: %+v)", GateCodeCanonicalCoverage, got, v)
	}
	if !mentions(v, GateCodeCanonicalCoverage, string(types.LayoutFamilySectionDivider)) {
		t.Errorf("coverage violation should name the missing family: %+v", v)
	}
}

func TestGateTitlePlaceholderNaming(t *testing.T) {
	layouts := conformantLayouts()
	layouts[0].Placeholders[0].ID = "Title 1" // PowerPoint default name
	report := conformantReport(layouts)

	v := Gate(report)
	if got := codesOf(v)[GateCodeTitleNaming]; got != 1 {
		t.Fatalf("expected exactly 1 %s violation, got %d (all: %+v)", GateCodeTitleNaming, got, v)
	}
	if !mentions(v, GateCodeTitleNaming, "Title 1") {
		t.Errorf("title-naming violation should quote the offending name: %+v", v)
	}
}

func TestGateSectionNumberMissing(t *testing.T) {
	layouts := conformantLayouts()
	// Rename the Section Number frame to a non-canonical name.
	layouts[2].Placeholders[1].ID = "Big Number"
	report := conformantReport(layouts)

	v := Gate(report)
	if got := codesOf(v)[GateCodeSectionNumber]; got != 1 {
		t.Fatalf("expected exactly 1 %s violation, got %d (all: %+v)", GateCodeSectionNumber, got, v)
	}
	if !mentions(v, GateCodeSectionNumber, "Section Divider") {
		t.Errorf("section-number violation should name the offending layout: %+v", v)
	}
}

func TestGateErrorFinding(t *testing.T) {
	report := BuildReport(Inputs{
		Template: "fixture.pptx",
		Layouts:  conformantLayouts(),
		MetadataDiagnostics: []diagnostics.Diagnostic{
			{
				Code:     diagnostics.CodeTemplateError,
				Severity: diagnostics.SeverityError,
				Message:  "synthetic structural error",
			},
		},
	})

	v := Gate(report)
	if got := codesOf(v)[GateCodeErrorFinding]; got != 1 {
		t.Fatalf("expected exactly 1 %s violation, got %d (all: %+v)", GateCodeErrorFinding, got, v)
	}
}

// TestGateAccumulatesMultipleViolations confirms a thoroughly broken template
// reports every distinct failure class rather than short-circuiting on the
// first, so a reviewer sees the full repair list in one CI run.
func TestGateAccumulatesMultipleViolations(t *testing.T) {
	layouts := conformantLayouts()
	layouts[1].Tags = nil                       // empty tags
	layouts[0].Placeholders[0].ID = "Title 4"   // title naming
	layouts[2].Placeholders[1].ID = "Big Digit" // section number missing
	report := conformantReport(layouts)

	codes := codesOf(Gate(report))
	for _, want := range []string{GateCodeEmptyTags, GateCodeTitleNaming, GateCodeSectionNumber} {
		if codes[want] == 0 {
			t.Errorf("expected at least one %s violation; got codes %+v", want, codes)
		}
	}
}

// mentions reports whether any violation with the given code contains substr.
func mentions(violations []GateViolation, code, substr string) bool {
	for _, v := range violations {
		if v.Code == code && strings.Contains(v.Message, substr) {
			return true
		}
	}
	return false
}
