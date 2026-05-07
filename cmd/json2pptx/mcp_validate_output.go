package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// mcpValidateOutputTool defines the validate_presentation_output MCP tool.
func mcpValidateOutputTool() mcp.Tool {
	return mcp.NewTool("validate_presentation_output",
		mcp.WithDescription("Validate a generated PPTX file for structural and OOXML content correctness. Runs the unified output-validation suite (OPC package integrity + OOXML content checks). Use this to verify a previously generated presentation before delivery."),
		mcp.WithRawOutputSchema(outputSchemaValidateOutput),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute or relative path to the PPTX file to validate."),
		),
	)
}

// validateOutputResponse is the JSON output for validate_presentation_output.
type validateOutputResponse struct {
	IsValid  bool            `json:"is_valid"`
	FilePath string          `json:"file_path"`
	Summary  string          `json:"summary"`
	Findings []pptx.Finding  `json:"findings,omitempty"`
}

// handleValidateOutput validates a generated PPTX file using the unified output-validation suite.
func handleValidateOutput(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil || path == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "path is required"), nil
	}

	// Check file exists
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return api.MCPSimpleError("FILE_NOT_FOUND", fmt.Sprintf("file not found: %s", path)), nil
	}

	report, err := pptx.ValidateOutputFile(path)
	if err != nil {
		return api.MCPSimpleError("VALIDATION_FAILED", fmt.Sprintf("failed to validate: %v", err)), nil
	}

	blocking := report.Blocking()
	warnings := report.Warnings()

	summary := "valid"
	if len(blocking) > 0 {
		summary = fmt.Sprintf("%d blocking, %d warning finding(s)", len(blocking), len(warnings))
	} else if len(warnings) > 0 {
		summary = fmt.Sprintf("%d warning finding(s)", len(warnings))
	}

	resp := validateOutputResponse{
		IsValid:  report.IsValid(),
		FilePath: path,
		Summary:  summary,
		Findings: report.Findings,
	}

	mcpResult, err := api.MCPSuccessResult(context.Background(), resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// outputValidationErrorEnvelope is the structured error payload when strict
// output validation fails during generation.
type outputValidationErrorEnvelope struct {
	Summary  string         `json:"summary"`
	Findings []pptx.Finding `json:"findings"`
}

// mcpOutputValidationError builds an error CallToolResult from an output
// validation report that contains blocking findings. Used by generate_presentation
// in strict output_validation mode.
func mcpOutputValidationError(report *pptx.Report) *mcp.CallToolResult {
	blocking := report.Blocking()
	warnings := report.Warnings()

	envelope := outputValidationErrorEnvelope{
		Summary:  fmt.Sprintf("output validation failed: %d blocking, %d warning finding(s)", len(blocking), len(warnings)),
		Findings: report.Findings,
	}

	fallback, err := json.Marshal(envelope)
	if err != nil {
		fallback = []byte(envelope.Summary)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(fallback),
			},
		},
		StructuredContent: envelope,
		IsError:           true,
	}
}
