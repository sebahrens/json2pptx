package main

import "github.com/sebahrens/json2pptx/internal/patterns"

// assembleGenerationFindings merges all finding sources before deduplication
// and budgeting. CLI and MCP differ only in whether full preflight is requested.
func assembleGenerationFindings(input *PresentationInput, run RenderResult, fullPreflight, verbose bool) []patterns.FitFinding {
	var findings []patterns.FitFinding
	if fullPreflight {
		findings = collectFitFindings(input, run.TemplateLayouts, run.SlideWidth, run.SlideHeight, &run.TemplateTheme)
	} else {
		findings = unresolvedPlaceholderContrastFindings(input, run.TemplateLayouts, run.TemplateTheme.Colors)
		findings = append(findings, collectPatternPostExpandFindings(input, run.SlideWidth, run.SlideHeight, &run.TemplateTheme)...)
		findings = append(findings, collectDiagramIconFindings(input)...)
	}
	findings = append(findings, run.SynthesisFindings...)
	if run.GenResult != nil {
		findings = append(findings, run.GenResult.FitFindings...)
		findings = append(findings, contrastSwapsToFindings(run.GenResult.ContrastSwaps, input, run.TemplateTheme.Colors)...)
	}
	findings = append(findings, validateSlidesChartDiagnostics(input.Slides)...)
	findings = append(findings, run.GridVisualFindings...)
	findings = append(findings, collectDroppedDiagramColorWarnings(input)...)
	findings = append(findings, run.StrictFitFindings...)
	findings = supersedeRealizedContrastPredictions(dedupFitFindings(findings))
	return BudgetFitFindings(findings, DefaultFindingBudget, verbose)
}
