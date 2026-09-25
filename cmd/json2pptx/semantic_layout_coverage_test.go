package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

func TestTemplateResolutionCodeSeparatesMissingNameFromLoadFailure(t *testing.T) {
	_, cleanup, missing := resolveTemplatePath("definitely-not-a-template", "../../templates")
	defer cleanup()
	if !errors.Is(missing, errTemplateNameNotFound) || templateResolutionCode(missing) != diagnostics.CodeTemplateNotFound {
		t.Fatalf("missing template classification = %v / %v", missing, templateResolutionCode(missing))
	}
	loadFailure := fmt.Errorf("extract embedded template: %w", errors.New("disk full"))
	if templateResolutionCode(loadFailure) != diagnostics.CodeTemplateError {
		t.Fatalf("template load failure misreported as a missing name: %v", templateResolutionCode(loadFailure))
	}
}

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
