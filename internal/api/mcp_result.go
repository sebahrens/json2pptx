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
// and a text block for hosts that read only Content. Legacy sessions receive
// the complete compact JSON; modern sessions receive a bounded synopsis.
func MCPSuccessResult(ctx context.Context, data any) (*mcp.CallToolResult, error) {
	res := &mcp.CallToolResult{StructuredContent: data}
	textJSON, err := mcpResponseText(ctx, data)
	if err != nil {
		return nil, err
	}
	res.Content = []mcp.Content{mcp.TextContent{Type: "text", Text: string(textJSON)}}
	return res, nil
}

// MCPResultFor is MCPSuccessResult for a tool that was asked to PRODUCE
// something — a rendered deck, a compiled deck. When the payload reports its own
// failure (a top-level "ok": false or "success": false) the result is marked
// IsError, because nothing was produced.
//
// isError is the only protocol-level failure signal, and it used to be absent
// on every DOMAIN failure of the semantic tools (template not found, an
// unparseable spec, an unknown diagram type) while argument-level failures on
// the same tools set it. An agent or harness branching on isError read "a deck
// that was never written" as done, then called render_deck_thumbnails on a path
// that did not exist (go-slide-creator-swak).
//
// Tools that ASSESS rather than produce — validate_input, validate_pattern,
// validate_deck_spec — keep reporting an invalid deck as a successful call with
// ok:false. Their verdict IS the product, and the repo's contract tests pin
// that: "validation failures should not be IsError".
//
// The structured payload is unchanged either way: diagnostics and the
// explanation are exactly as valuable on a failure, and only the flag moves.
func MCPResultFor(ctx context.Context, data any) (*mcp.CallToolResult, error) {
	res, err := MCPSuccessResult(ctx, data)
	if err != nil {
		return nil, err
	}
	marshalled, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	res.IsError = payloadReportsFailure(marshalled)
	return res, nil
}

// payloadReportsFailure reports whether a marshalled tool response declares its
// own failure via a top-level "ok" or "success" boolean set to false. Payloads
// with neither field (the many tools that just return data) are never errors.
func payloadReportsFailure(marshalled []byte) bool {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(marshalled, &top); err != nil {
		return false // not an object: nothing to read a status from
	}
	for _, key := range []string{"ok", "success"} {
		raw, present := top[key]
		if !present {
			continue
		}
		var b bool
		if err := json.Unmarshal(raw, &b); err == nil && !b {
			return true
		}
	}
	return false
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
