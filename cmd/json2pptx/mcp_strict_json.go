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

// resolveStringOrObject resolves a parameter that can be provided as either a
// JSON string (stringParam) or a structured object (objectParam). If both are
// provided, it returns an ambiguous_input error. If the object form is provided,
// it is marshalled to a JSON string. Returns the JSON string and an optional
// error result (non-nil means the caller should return it immediately).
func resolveStringOrObject(request mcp.CallToolRequest, stringParam, objectParam string) (string, *mcp.CallToolResult) {
	args := request.GetArguments()
	hasString := false
	hasObject := false

	jsonStr, err := request.RequireString(stringParam)
	if err == nil && jsonStr != "" {
		hasString = true
	}

	objRaw, objPresent := args[objectParam]
	if objPresent && objRaw != nil {
		hasObject = true
	}

	if hasString && hasObject {
		return "", mcpParseErrorWithFix("ambiguous_input", objectParam,
			fmt.Sprintf("both %q and %q provided; use one or the other", stringParam, objectParam),
			&diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"allowed": []string{stringParam, objectParam}}},
		)
	}

	if hasObject {
		b, marshalErr := json.Marshal(objRaw)
		if marshalErr != nil {
			return "", mcpParseError("INVALID_JSON", objectParam, fmt.Sprintf("failed to encode %s: %v", objectParam, marshalErr))
		}
		return string(b), nil
	}

	if hasString {
		return jsonStr, nil
	}

	// Neither provided.
	return "", nil
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
