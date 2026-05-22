package main

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Suggestion factories for common next_tool_call values returned from
// agent-facing handler error paths. They centralize the "what should the agent
// call next" decision so every error response chains forward consistently.

// nextCallGetInputSchema points the agent at the authoritative input schema
// when their JSON failed to parse or required fields were missing.
func nextCallGetInputSchema() *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool:         "get_input_schema",
		ArgsTemplate: map[string]any{},
	}
}

// nextCallListTemplates points the agent at the available templates when a
// template name was unknown or template analysis failed.
func nextCallListTemplates() *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool:         "list_templates",
		ArgsTemplate: map[string]any{},
	}
}

// nextCallListPatterns points the agent at the pattern catalog when a pattern
// name was unknown or unsupported in the requested context.
func nextCallListPatterns() *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool:         "list_patterns",
		ArgsTemplate: map[string]any{},
	}
}

// nextCallInspectSlideImages points the agent at the canonical vision-based
// visual QA tool. Used when a caller asks for a heuristic/vision pass on a
// tool that only ships the deterministic axis.
func nextCallInspectSlideImages() *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool: "inspect_slide_images",
		ArgsTemplate: map[string]any{
			"slide_images": "<array of {index, path|png_base64} from render_deck_thumbnails>",
		},
	}
}

// nextCallReadPresentation suggests read_presentation against a concrete file
// path. Used as the recovery hop when an existing PPTX could not be validated:
// the deterministic reader surfaces what the file actually contains so the agent
// can decide whether to regenerate or repair.
func nextCallReadPresentation(pptxPath string) *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool: "read_presentation",
		ArgsTemplate: map[string]any{
			"pptx_path": pptxPath,
		},
	}
}

// nextCallValidateOutput suggests validate_presentation_output against a concrete
// file path. Used as the recovery hop when an existing PPTX could not be read:
// the structural validator explains why the OPC package is malformed.
func nextCallValidateOutput(pptxPath string) *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool: "validate_presentation_output",
		ArgsTemplate: map[string]any{
			"path": pptxPath,
		},
	}
}

// nextCallRetry suggests retrying the same tool with a corrected argument.
// requiredArg is the name of the missing/invalid required parameter; the
// args template uses a placeholder string the agent must replace.
func nextCallRetry(tool, requiredArg string) *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{
		Tool: tool,
		ArgsTemplate: map[string]any{
			requiredArg: "<provide value>",
		},
	}
}

// mcpErrorWithNext builds an MCPDiagnosticsError carrying a single
// error-severity diagnostic with a NextToolCall suggestion.
func mcpErrorWithNext(code, message string, next *patterns.ToolCallSuggestion) *mcp.CallToolResult {
	return mcpStructuredError(code, "", message, nil, next)
}

// mcpParseErrorWithNext builds an MCPDiagnosticsError for JSON parse failures
// with a NextToolCall suggestion attached.
func mcpParseErrorWithNext(code, path, message string, next *patterns.ToolCallSuggestion) *mcp.CallToolResult {
	return mcpStructuredError(code, path, message, nil, next)
}

// mcpStructuredError builds an MCPDiagnosticsError for a single error-severity
// diagnostic with an optional Fix and NextToolCall.
func mcpStructuredError(code, path, message string, fix *diagnostics.Fix, next *patterns.ToolCallSuggestion) *mcp.CallToolResult {
	d := diagnostics.Diagnostic{
		Code:         code,
		Path:         path,
		Message:      message,
		Severity:     diagnostics.SeverityError,
		Fix:          fix,
		NextToolCall: next,
	}
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{d})
}
