package main

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// Builders for the high-frequency *runtime* failures of primary MCP tools —
// missing template, missing/unreadable PPTX, failed output validation, and a
// bad slide index. Unlike the arg-validation helpers in mcp_errors.go (which
// fire while parsing arguments), these fire after arguments parse, when the
// named resource is absent or broken. Each returns a single-diagnostic envelope
// that preserves a concise human message but adds the machine-readable fields an
// agent needs to recover without guessing: the offending JSON path, the expected
// type, an example value, contextual details, and an executable next_tool_call.

// mcpDiagError wraps a single Diagnostic in the standard MCP error envelope,
// defaulting Severity to error so the result is marked IsError.
func mcpDiagError(d diagnostics.Diagnostic) *mcp.CallToolResult {
	if d.Severity == "" {
		d.Severity = diagnostics.SeverityError
	}
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{d})
}

// mcpTemplateNotFoundError reports a template name that could not be resolved in
// any search location. It routes the agent to list_templates and surfaces both
// the requested name and the available names so a typo can be fixed in one hop.
func mcpTemplateNotFoundError(templateName, templatesDir string) *mcp.CallToolResult {
	available := listAvailableTemplates(templatesDir)
	details := map[string]any{"template_name": templateName}
	var example any
	if len(available) > 0 {
		details["available_templates"] = available
		example = available[0]
	}
	return mcpDiagError(diagnostics.Diagnostic{
		Code:         diagnostics.CodeTemplateNotFound,
		Path:         "template",
		Message:      templateNotFoundError(templateName, templatesDir),
		ExpectedType: "string",
		ExampleValue: example,
		Details:      details,
		NextToolCall: nextCallListTemplates(),
	})
}

// mcpFileNotFoundError reports a PPTX path that does not exist on disk. pathArg
// is the JSON argument name (pptx_path or path) so the envelope's path field
// matches the tool's schema; tool is the registered MCP tool name so the
// next_tool_call retry is executable verbatim once the agent corrects the path.
func mcpFileNotFoundError(tool, pathArg, missingPath string) *mcp.CallToolResult {
	return mcpDiagError(diagnostics.Diagnostic{
		Code:         diagnostics.CodeFileNotFound,
		Path:         pathArg,
		Message:      fmt.Sprintf("pptx file not found: %s", missingPath),
		ExpectedType: "string",
		ExampleValue: "/tmp/out/deck.pptx",
		Details:      map[string]any{"file_path": missingPath},
		NextToolCall: nextCallRetry(tool, pathArg),
	})
}

// mcpReadFailedError reports a PPTX that exists but could not be parsed by the
// deterministic reader. The recovery hop is validate_presentation_output, whose
// structural checks explain why the package is malformed.
func mcpReadFailedError(pptxPath string, cause error) *mcp.CallToolResult {
	return mcpDiagError(diagnostics.Diagnostic{
		Code:         diagnostics.CodeReadFailed,
		Path:         "pptx_path",
		Message:      fmt.Sprintf("failed to read presentation: %v", cause),
		Details:      map[string]any{"file_path": pptxPath},
		NextToolCall: nextCallValidateOutput(pptxPath),
	})
}

// mcpValidationFailedError reports that the output validator itself errored on a
// file that exists. The recovery hop is read_presentation, whose best-effort
// deterministic read lets the agent inspect the file's contents.
func mcpValidationFailedError(pptxPath string, cause error) *mcp.CallToolResult {
	return mcpDiagError(diagnostics.Diagnostic{
		Code:         diagnostics.CodeValidationFailed,
		Path:         "path",
		Message:      fmt.Sprintf("failed to validate: %v", cause),
		Details:      map[string]any{"file_path": pptxPath},
		NextToolCall: nextCallReadPresentation(pptxPath),
	})
}

// mcpInvalidSlideIndexError reports a slide_index outside the presentation's
// range. It carries the slide count so the agent can pick a valid index and
// retry read_presentation without re-reading the whole deck.
func mcpInvalidSlideIndexError(idx, slideCount int) *mcp.CallToolResult {
	return mcpDiagError(diagnostics.Diagnostic{
		Code:         diagnostics.CodeInvalidSlideIndex,
		Path:         "slide_index",
		Message:      fmt.Sprintf("slide_index %d out of range (presentation has %d slides)", idx, slideCount),
		ExpectedType: "integer",
		ExampleValue: 0,
		Details:      map[string]any{"slide_count": slideCount},
		NextToolCall: nextCallRetry("read_presentation", "slide_index"),
	})
}
