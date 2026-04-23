package api

import (
	"encoding/json"
	"os"
)

// MarshalMCPResponse serializes v for MCP tool output.
// When MCP_COMPACT_RESPONSES=1, it uses json.Marshal (no indentation).
// Otherwise it uses json.MarshalIndent with two-space indentation to match
// the previous default behavior.
func MarshalMCPResponse(v any) ([]byte, error) {
	if os.Getenv("MCP_COMPACT_RESPONSES") == "1" {
		return json.Marshal(v)
	}
	return json.MarshalIndent(v, "", "  ")
}
