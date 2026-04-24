package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"

	"github.com/mark3labs/mcp-go/mcp"
)

// strictUnmarshalJSON decodes raw JSON into dst using a Decoder configured to
// reject unknown fields (DisallowUnknownFields) and trailing data. This is the
// boundary guard for all agent-facing MCP tools: if the JSON doesn't cleanly
// parse into the target type, the agent gets a typed diagnostic instead of a
// silent partial parse.
func strictUnmarshalJSON(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// Reject trailing data after the first valid JSON value.
	if tok, err := dec.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing data after JSON value (found %v)", tok)
		}
		return fmt.Errorf("unexpected trailing data: %w", err)
	}
	return nil
}

// mcpParseError builds a structured MCP error result for JSON parse failures.
// path is the JSON path context (e.g. "json_input", "values"), code is the
// diagnostic code (e.g. "INVALID_JSON").
func mcpParseError(code, path, message string) *mcp.CallToolResult {
	d := diagnostics.Diagnostic{
		Code:     code,
		Path:     path,
		Message:  message,
		Severity: diagnostics.SeverityError,
	}
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{d})
}

// mcpParseErrorWithFix builds a structured MCP error result for JSON parse
// failures with an attached fix suggestion.
func mcpParseErrorWithFix(code, path, message string, fix *diagnostics.Fix) *mcp.CallToolResult {
	d := diagnostics.Diagnostic{
		Code:     code,
		Path:     path,
		Message:  message,
		Severity: diagnostics.SeverityError,
		Fix:      fix,
	}
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{d})
}
