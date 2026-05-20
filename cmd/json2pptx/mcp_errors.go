package main

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Shared MCP arg-validation error envelope. Every arg-validation failure from
// an MCP handler should flow through these helpers so agents see a consistent
// envelope: a stable code, the JSON path of the bad/missing argument, the
// expected JSON-schema-style type, an optional example value, and a
// next_tool_call suggestion the agent can replay verbatim.

// argErrorEnvelope groups the metadata that every arg-validation error should
// carry. ExpectedType and ExampleValue are optional. NextToolCall should be
// supplied whenever the agent can recover by replaying the same tool with
// corrected arguments or by hopping to a discovery tool (e.g. get_input_schema,
// list_templates).
type argErrorEnvelope struct {
	Code         string
	Path         string
	Message      string
	ExpectedType string
	ExampleValue any
	NextToolCall *patterns.ToolCallSuggestion
}

// argError builds a structured MCP error result for an arg-validation failure.
// Use this from every MCP handler so agents see a consistent envelope.
func argError(env argErrorEnvelope) *mcp.CallToolResult {
	if env.Code == "" {
		env.Code = "INVALID_ARG"
	}
	d := diagnostics.Diagnostic{
		Code:         env.Code,
		Path:         env.Path,
		Message:      env.Message,
		Severity:     diagnostics.SeverityError,
		ExpectedType: env.ExpectedType,
		ExampleValue: env.ExampleValue,
		NextToolCall: env.NextToolCall,
	}
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{d})
}

// argMissing builds an error for a missing required argument. The default
// next_tool_call replays the same tool with the required arg as a placeholder
// so the agent can substitute and call again. Pass next=nil to omit; pass an
// explicit suggestion to route the agent elsewhere (e.g. nextCallListTemplates
// when the missing arg is a template name).
func argMissing(tool, path, expectedType string, example any, next *patterns.ToolCallSuggestion) *mcp.CallToolResult {
	msg := path + " is required"
	if expectedType != "" {
		msg = path + " is required (expected " + expectedType + ")"
	}
	if next == nil && tool != "" {
		next = nextCallRetry(tool, path)
	}
	return argError(argErrorEnvelope{
		Code:         "MISSING_PARAMETER",
		Path:         path,
		Message:      msg,
		ExpectedType: expectedType,
		ExampleValue: example,
		NextToolCall: next,
	})
}

// argInvalidJSON builds an error for a JSON parse / shape failure on an
// object-typed argument. Pass next=nil to use the default get_input_schema
// suggestion.
func argInvalidJSON(path, message, expectedType string, example any, next *patterns.ToolCallSuggestion) *mcp.CallToolResult {
	if next == nil {
		next = nextCallGetInputSchema()
	}
	return argError(argErrorEnvelope{
		Code:         "INVALID_JSON",
		Path:         path,
		Message:      message,
		ExpectedType: expectedType,
		ExampleValue: example,
		NextToolCall: next,
	})
}

// argInvalidValue builds an error for an argument that parsed but failed a
// type/range/enum check. Pass next=nil to default to a retry of the same tool
// with the offending path as a placeholder.
func argInvalidValue(tool, code, path, message, expectedType string, example any, next *patterns.ToolCallSuggestion) *mcp.CallToolResult {
	if code == "" {
		code = "INVALID_PARAMETER"
	}
	if next == nil && tool != "" {
		next = nextCallRetry(tool, path)
	}
	return argError(argErrorEnvelope{
		Code:         code,
		Path:         path,
		Message:      message,
		ExpectedType: expectedType,
		ExampleValue: example,
		NextToolCall: next,
	})
}
