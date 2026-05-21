package api

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// mcpErrorSubcommand is the generic Subcommand stamped on MCP error envelopes.
// The shared error builders (MCPDiagnosticsError / MCPSimpleError) are called
// from ~150 sites that do not plumb their tool name through, so a single generic
// surface identifier is used; per-tool attribution can be threaded later without
// changing the wire shape.
const mcpErrorSubcommand = "mcp"

// MCPSuccessResult builds a CallToolResult with StructuredContent set to data
// and a JSON text fallback in Content. The text fallback respects the session's
// compact_responses negotiation (via MarshalMCPResponse).
func MCPSuccessResult(ctx context.Context, data any) (*mcp.CallToolResult, error) {
	textJSON, err := MarshalMCPResponse(ctx, data)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(textJSON),
			},
		},
		StructuredContent: data,
	}, nil
}

// MCPDiagnosticsError builds an error CallToolResult from a slice of
// Diagnostics. The result has IsError=true, StructuredContent carrying the
// shared diagnostics.FindingEnvelope wire shape, and a human-readable text
// fallback. Callers keep passing []diagnostics.Diagnostic; the envelope is
// assembled via diagnostics.BuildEnvelope, so namespaced codes, remediation,
// next_tool_call, expected_type, and example_value all survive on the wire.
func MCPDiagnosticsError(ds []diagnostics.Diagnostic) *mcp.CallToolResult {
	envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand: mcpErrorSubcommand,
	}, ds)

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

// MCPSimpleError builds an error CallToolResult for a single error with the
// given code and message. It sets IsError=true and populates StructuredContent
// with a diagnostics envelope containing one error-severity diagnostic.
func MCPSimpleError(code, message string) *mcp.CallToolResult {
	return MCPDiagnosticsError([]diagnostics.Diagnostic{
		{
			Code:     code,
			Message:  message,
			Severity: diagnostics.SeverityError,
		},
	})
}
