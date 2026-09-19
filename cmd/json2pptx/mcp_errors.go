package main

import (
	"encoding/json"
	"fmt"
	"strings"

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

// argRequired reports a required argument the caller did not supply — or, when
// the argument IS present, the type error it actually is.
//
// The typed accessors cannot tell the two apart: RequireString on a number
// returns an error, and every handler turned that into "code is required". An
// agent told a field is missing when it is sitting right there in the call
// re-sends it unchanged, or drops it; neither is the fix. The distinction is one
// map lookup away, so it is made here rather than at 60 call sites
// (go-slide-creator-6072).
func argRequired(request mcp.CallToolRequest, tool, path, expectedType string, example any, next *patterns.ToolCallSuggestion) *mcp.CallToolResult {
	if v, ok := lookupArgPath(request.GetArguments(), path); ok && v != nil {
		if next == nil && tool != "" {
			next = nextCallRetry(tool, path)
		}
		return argError(argErrorEnvelope{
			Code:         diagnostics.CodeInvalidParameter,
			Path:         path,
			Message:      fmt.Sprintf("%s must be %s, got %s", path, articleFor(expectedType), jsonTypeName(v)),
			ExpectedType: expectedType,
			ExampleValue: example,
			NextToolCall: next,
		})
	}
	return argMissing(tool, path, expectedType, example, next)
}

// lookupArgPath resolves a dotted argument path ("presentation.slides") against
// a call's arguments. A path whose parent is not an object reports not-found:
// the parent's own type error is the one worth reporting.
func lookupArgPath(args map[string]any, path string) (any, bool) {
	if args == nil || path == "" {
		return nil, false
	}
	var current any = args
	for _, segment := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// jsonTypeName names the JSON type of a decoded argument value, in the
// vocabulary an agent reading the schema will recognise.
func jsonTypeName(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case float64, int, int64, json.Number:
		return "a number"
	case string:
		return "a string"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	default:
		return fmt.Sprintf("%T", t)
	}
}

// articleFor renders an expected-type name with its article, so the message
// reads "must be a string" / "must be an object|string".
func articleFor(expectedType string) string {
	switch expectedType {
	case "":
		return "of the documented type"
	case "object", "array", "integer":
		return "an " + expectedType
	default:
		return "a " + expectedType
	}
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
