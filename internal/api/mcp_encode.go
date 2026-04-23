package api

import (
	"context"
	"encoding/json"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// MarshalMCPResponse serializes v for MCP tool output.
//
// Compact mode (no indentation) is enabled when EITHER:
//   - The client negotiated compact_responses during MCP initialization
//     (experimental.compact_responses capability), OR
//   - The MCP_COMPACT_RESPONSES=1 environment variable is set (deprecated;
//     will be removed in a future release).
//
// Otherwise it uses json.MarshalIndent with two-space indentation.
func MarshalMCPResponse(ctx context.Context, v any) ([]byte, error) {
	if isCompactSession(ctx) || os.Getenv("MCP_COMPACT_RESPONSES") == "1" {
		return json.Marshal(v)
	}
	return json.MarshalIndent(v, "", "  ")
}

// isCompactSession checks whether the current MCP session negotiated
// compact_responses via the experimental capability.
func isCompactSession(ctx context.Context) bool {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return false
	}
	sci, ok := session.(server.SessionWithClientInfo)
	if !ok {
		return false
	}
	caps := sci.GetClientCapabilities()
	if caps.Experimental == nil {
		return false
	}
	v, exists := caps.Experimental["compact_responses"]
	if !exists {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}
