package semantic

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func coverageSpec() *DeckSpec {
	return &DeckSpec{
		Meta: DeckMeta{
			Title: "Coverage", Template: "midnight-blue",
			RequiredLayouts: []string{"title", "content", "two-column", "section", "blank-title", "blank-canvas", "closing"},
		},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Coverage"}},
			{Kind: KindSection, Body: map[string]any{"title": "Evidence"}},
			{Kind: KindTable, Body: map[string]any{"title": "Data", "headers": []any{"A", "B"}, "rows": []any{[]any{"x", "y"}}, "takeaway": "Data supports the case"}},
			{Kind: KindComparison, Body: map[string]any{"title": "Options", "columns": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}, "takeaway": "A leads"}},
			kpiSlide(3),
			{Kind: KindProcess, Body: map[string]any{"title": "Execution", "steps": []any{"Plan", "Build", "Scale"}, "takeaway": "Three steps"}},
			{Kind: KindClosing, Body: map[string]any{"title": "Questions"}},
		},
	}
}

func TestRequiredLayoutCoverageAssignsAndCompiles(t *testing.T) {
	spec := coverageSpec()
	ir := Normalize(spec)
	if len(ir.LayoutCoverage.Missing) != 0 || len(ir.LayoutCoverage.Assigned) != 7 {
		t.Fatalf("coverage = %+v", ir.LayoutCoverage)
	}
	if got := ir.Explain().LayoutCoverage; len(got.Assigned) != 7 {
		t.Fatalf("explain coverage = %+v", got)
	}
	input, result, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v diagnostics=%+v", err, result.Diagnostics)
	}
	seen := map[string]bool{}
	for _, slide := range input.Slides {
		seen[slide.LayoutID] = true
		if slide.LayoutID == "blank-canvas" && slide.Headline == "" {
			t.Error("blank-canvas assignment must carry the semantic title as headline")
		}
	}
	for _, required := range spec.Meta.RequiredLayouts {
		if !seen[required] {
			t.Errorf("compiled deck does not cover %q: %+v", required, seen)
		}
	}
}

func TestRequiredLayoutValidationUnknownDuplicateAndMissing(t *testing.T) {
	spec := coverageSpec()
	spec.Meta.RequiredLayouts = []string{"title", "TITLE", "not-a-layout", "image-left"}
	ds := Validate(spec, StrictnessWarn)
	codes := map[string]bool{}
	for _, d := range ds {
		codes[d.Code] = true
	}
	if !codes[string(diagnostics.CodeSemanticRequiredLayoutDuplicate)] || !codes[string(diagnostics.CodeSemanticRequiredLayoutUnknown)] {
		t.Fatalf("validation codes = %+v diagnostics=%+v", codes, ds)
	}

	spec.Meta.RequiredLayouts = []string{"image-left"}
	ds = rhythmDiagnostics(Normalize(spec), StrictnessWarn)
	if len(ds) != 1 || ds[0].Code != string(diagnostics.CodeSemanticRequiredLayoutMissing) || ds[0].Severity != diagnostics.SeverityError {
		t.Fatalf("missing coverage diagnostics = %+v", ds)
	}
}

func TestNoRequiredLayoutsLeavesPlanningUnchanged(t *testing.T) {
	spec := coverageSpec()
	spec.Meta.RequiredLayouts = nil
	ir := Normalize(spec)
	if len(ir.LayoutCoverage.Requested) != 0 || len(ir.LayoutCoverage.Assigned) != 0 || len(ir.LayoutCoverage.Missing) != 0 {
		t.Fatalf("unexpected coverage plan: %+v", ir.LayoutCoverage)
	}
	if ir.Slides[4].Visual.Layout != "blank-title" {
		t.Errorf("default KPI layout changed to %q", ir.Slides[4].Visual.Layout)
	}
}

func TestRequiredLayoutCoverageUsesCompleteMatching(t *testing.T) {
	slides := []SlideIR{
		{Kind: KindKPISnapshot, Role: RoleEvidence, SourcePath: "slides[0]", Visual: VisualPlan{Pattern: "kpi-3up"}},
		{Kind: KindTable, Role: RoleEvidence, SourcePath: "slides[1]", Visual: VisualPlan{}},
	}
	coverage := applyRequiredLayoutCoverage(slides, []string{"content", "blank-title"})
	if len(coverage.Missing) != 0 {
		t.Fatalf("complete assignment reported missing layouts: %+v", coverage)
	}
	if len(coverage.Assigned) != 2 || coverage.Assigned[0].SlideIndex != 1 || coverage.Assigned[1].SlideIndex != 0 {
		t.Fatalf("assignment = %+v, want content->table and blank-title->KPI", coverage.Assigned)
	}
}
