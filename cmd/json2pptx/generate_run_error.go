package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// generateRunError adapts the shared runner's typed failures to the MCP
// boundary. Authored paths and strict-fit NDJSON use the existing shared tools.
func generateRunError(input *PresentationInput, authored *authoredPaths, run RenderResult, err error) *mcp.CallToolResult {
	var fit *StrictFitRefusal
	if errors.As(err, &fit) {
		writeFitReportNDJSON(os.Stderr, fit.Findings)
		return authored.mcpError(diagnostics.FromFitFindings(fit.Findings))
	}
	var validation *OutputValidationFailure
	if errors.As(err, &validation) {
		return mcpOutputValidationError(validation.Report)
	}
	if loss := generationRefusal(err); loss != nil {
		if _, where := locateRefusal(input, run.SlideSpecs, run.TemplateLayouts, nil, loss); where != "" {
			loss.Message = where + ": " + loss.Message
		}
	}
	if ds := refusalDiagnostics(err); len(ds) > 0 {
		return authored.mcpError(ds)
	}
	if ds := generationRefusalDiagnostics(err); len(ds) > 0 {
		return authored.mcpError(ds)
	}
	code, prefix := "GENERATION_FAILED", "generation failed"
	switch {
	case run.TemplateLayouts == nil:
		code, prefix = "TEMPLATE_ERROR", "template analysis failed"
	case run.GenResult != nil:
		code, prefix = "OUTPUT_VALIDATION_ERROR", "output validation failed"
	case len(run.SlideSpecs) == 0:
		code, prefix = "INVALID_SLIDE", "invalid slide specification"
	}
	return api.MCPSimpleError(code, fmt.Sprintf("%s: %v", prefix, err))
}
