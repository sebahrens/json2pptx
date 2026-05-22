package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
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
		return argMissing("validate_presentation_output", "path", "string", "/tmp/out/deck.pptx", nil), nil
	}

	// Reject malformed paths (traversal, wrong extension) with the same
	// structured INVALID_PATH envelope read_presentation uses, so an agent that
	// passes a bad path to either introspection tool gets a consistent contract.
	if err := api.ValidatePptxPath(path); err != nil {
		return argInvalidValue("validate_presentation_output", diagnostics.CodeInvalidPath, "path", err.Error(), "string", "/tmp/out/deck.pptx", nil), nil
	}

	// Check file exists
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return mcpFileNotFoundError("validate_presentation_output", "path", path), nil
	}

	report, err := pptx.ValidateOutputFile(path)
	if err != nil {
		return mcpValidationFailedError(path, err), nil
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
//
// Output-validation findings are structural OPC/OOXML problems (pptx.Finding)
// that carry no repair_slide fix directive — the engine cannot derive the
// corrective parameters (the replacement color, the target layout, …) from a
// finding alone. The earlier contract pointed NextToolCall at repair_slide with
// an empty fixes array, but repair_slide rejects an empty fixes array, so that
// "recovery hint" produced a second INVALID_PARAMETER error rather than a
// repair. NextToolCall now points at describe_finding, a directly-executable
// call that resolves the finding's meaning and remediation steps. Repairable is
// false and RepairUnavailableReason states why no repair_slide call is offered;
// agents construct a repair_slide directive themselves from each finding's
// preserved code / scope / source_path / slide_index context.
type outputValidationErrorEnvelope struct {
	Summary  string         `json:"summary"`
	Findings []pptx.Finding `json:"findings"`

	// Repairable reports whether a directly-executable repair_slide call is
	// offered. Always false for output-validation failures, so agents branch on
	// it deterministically instead of submitting the unavailable repair.
	Repairable bool `json:"repairable"`

	// RepairUnavailableReason explains why no executable repair_slide call is
	// advertised. Set when Repairable is false.
	RepairUnavailableReason string `json:"repair_unavailable_reason,omitempty"`

	// NextToolCall is a directly-executable next step. For output-validation
	// failures it points at describe_finding so the agent can resolve the
	// finding before choosing a repair — never at repair_slide with an empty
	// fixes array.
	NextToolCall *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
}

// repairUnavailableReason explains, in one line consumable by agents, why the
// envelope does not advertise an executable repair_slide call.
const repairUnavailableReason = "output-validation findings are structural OPC/OOXML problems with no auto-derivable repair_slide directive; inspect each finding's code, scope, and source_path, then construct the appropriate repair_slide fix (e.g. replace_color or use_semantic_color for scope=\"source\" findings)"

// mcpOutputValidationError builds an error CallToolResult from an output
// validation report that contains blocking findings. Used by generate_presentation
// in strict output_validation mode.
func mcpOutputValidationError(report *pptx.Report) *mcp.CallToolResult {
	blocking := report.Blocking()
	warnings := report.Warnings()

	envelope := outputValidationErrorEnvelope{
		Summary:                 fmt.Sprintf("output validation failed: %d blocking, %d warning finding(s)", len(blocking), len(warnings)),
		Findings:                report.Findings,
		Repairable:              false,
		RepairUnavailableReason: repairUnavailableReason,
		NextToolCall:            describeFindingToolCall(blocking),
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

// describeFindingToolCall builds a directly-executable describe_finding call for
// the most relevant blocking output-validation finding. describe_finding only
// requires a non-empty `code` string, so the emitted call always satisfies the
// target tool's schema. The code is the first blocking finding's own code when
// that code is describable, otherwise the umbrella OUTPUT_VALIDATION_ERROR code,
// which is always registered — guaranteeing the hint both validates and resolves
// to real remediation steps.
func describeFindingToolCall(blocking []pptx.Finding) *patterns.ToolCallSuggestion {
	code := string(diagnostics.CodeOutputValidationError)
	for _, f := range blocking {
		if f.Code == "" {
			continue
		}
		if _, ok := diagnostics.Describe(f.Code); ok {
			code = f.Code
			break
		}
	}
	return &patterns.ToolCallSuggestion{
		Tool:         "describe_finding",
		ArgsTemplate: map[string]any{"code": code},
	}
}
