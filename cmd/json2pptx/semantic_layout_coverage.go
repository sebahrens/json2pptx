package main

import (
	"fmt"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func semanticTemplateLayouts(templateName, templatesDir string, cache types.TemplateCache) ([]types.LayoutMetadata, *diagnostics.Diagnostic) {
	templatePath, cleanup, err := resolveTemplatePath(templateName, templatesDir)
	if err != nil {
		return nil, semanticTemplateDiagnostic(templateName, diagnostics.CodeTemplateNotFound, err)
	}
	defer cleanup()
	if cache == nil {
		cache = template.NewMemoryCache(time.Hour)
	}
	analysis, err := getOrAnalyzeTemplate(templatePath, cache)
	if err != nil {
		return nil, semanticTemplateDiagnostic(templateName, diagnostics.CodeTemplateError, err)
	}
	return analysis.Layouts, nil
}

func semanticTemplateDiagnostic(templateName string, code diagnostics.Code, err error) *diagnostics.Diagnostic {
	return &diagnostics.Diagnostic{
		Code:     string(code),
		Severity: diagnostics.SeverityError,
		Path:     "meta.template",
		Message:  fmt.Sprintf("template %q is unavailable: %v", templateName, err),
		Fix: &diagnostics.Fix{Kind: "choose_template", Params: map[string]any{
			"template": templateName,
		}},
	}
}

// requiredLayoutTemplateDiagnostics checks the second half of required-layout
// coverage: the semantic plan must have a compatible slide and the selected
// template must expose the requested native layout.
func requiredLayoutTemplateDiagnostics(required []string, templateName string, layouts []types.LayoutMetadata) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for i, name := range required {
		if _, ok := layout.ResolveCanonicalLayoutID(name, layouts); ok {
			continue
		}
		available := make([]string, 0, len(layouts))
		for _, candidate := range layouts {
			available = append(available, candidate.ID)
		}
		out = append(out, diagnostics.Diagnostic{
			Code:     string(diagnostics.CodeSemanticRequiredLayoutMissing),
			Severity: diagnostics.SeverityError,
			Path:     fmt.Sprintf("meta.required_layouts[%d]", i),
			Message:  fmt.Sprintf("required layout %q is unavailable in template %q; choose a template that provides it or remove the requirement", name, templateName),
			Fix: &diagnostics.Fix{
				Kind: "choose_template_or_remove_requirement",
				Params: map[string]any{
					"required_layout":   name,
					"template":          templateName,
					"available_layouts": available,
				},
			},
		})
	}
	return out
}

// reconcileExplanationTemplateCoverage makes explain report the template-aware
// result rather than only the narrative assignment result.
func reconcileExplanationTemplateCoverage(explanation *semantic.DeckExplanation, layouts []types.LayoutMetadata) {
	if explanation == nil || len(explanation.LayoutCoverage.Requested) == 0 {
		return
	}
	missing := make(map[string]bool, len(explanation.LayoutCoverage.Missing))
	for _, name := range explanation.LayoutCoverage.Missing {
		missing[name] = true
	}
	availableAssignments := explanation.LayoutCoverage.Assigned[:0]
	for _, assignment := range explanation.LayoutCoverage.Assigned {
		if _, ok := layout.ResolveCanonicalLayoutID(assignment.Layout, layouts); ok {
			availableAssignments = append(availableAssignments, assignment)
			continue
		}
		missing[assignment.Layout] = true
	}
	explanation.LayoutCoverage.Assigned = availableAssignments
	for _, requested := range explanation.LayoutCoverage.Requested {
		if _, ok := layout.ResolveCanonicalLayoutID(requested, layouts); ok || !missing[requested] {
			continue
		}
		alreadyListed := false
		for _, current := range explanation.LayoutCoverage.Missing {
			alreadyListed = alreadyListed || current == requested
		}
		if !alreadyListed {
			explanation.LayoutCoverage.Missing = append(explanation.LayoutCoverage.Missing, requested)
		}
		explanation.RhythmWarnings = append(explanation.RhythmWarnings, semantic.RhythmWarning{
			Code:    string(diagnostics.CodeSemanticRequiredLayoutMissing),
			Path:    "meta.required_layouts",
			Message: fmt.Sprintf("required layout %q is unavailable in template %q; choose another template or remove the requirement", requested, explanation.Template),
		})
	}
}
