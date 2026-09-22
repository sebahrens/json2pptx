package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

func TestRequiredLayoutTemplateDiagnosticsAndExplanation(t *testing.T) {
	layouts := titleAndBlankOnly()[:1] // selected template has no true blank canvas
	ds := requiredLayoutTemplateDiagnostics([]string{"title", "blank-canvas"}, "title-only", layouts)
	if len(ds) != 1 || ds[0].Code != string(diagnostics.CodeSemanticRequiredLayoutMissing) || ds[0].Path != "meta.required_layouts[1]" {
		t.Fatalf("diagnostics = %+v", ds)
	}

	explanation := semantic.DeckExplanation{
		Template: "title-only",
		LayoutCoverage: semantic.LayoutCoverage{
			Requested: []string{"title", "blank-canvas"},
			Assigned: []semantic.LayoutAssignment{
				{Layout: "title", SlideIndex: 0},
				{Layout: "blank-canvas", SlideIndex: 1},
			},
		},
	}
	reconcileExplanationTemplateCoverage(&explanation, layouts)
	if len(explanation.LayoutCoverage.Assigned) != 1 || len(explanation.LayoutCoverage.Missing) != 1 || explanation.LayoutCoverage.Missing[0] != "blank-canvas" {
		t.Fatalf("coverage = %+v", explanation.LayoutCoverage)
	}
	if len(explanation.RhythmWarnings) != 1 || explanation.RhythmWarnings[0].Code != string(diagnostics.CodeSemanticRequiredLayoutMissing) {
		t.Fatalf("warnings = %+v", explanation.RhythmWarnings)
	}
}
