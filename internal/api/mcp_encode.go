package api

import (
	"context"
	"encoding/json"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// MarshalMCPResponse serializes v for MCP tool output.
//
// The server advertises experimental.compact_responses: true in its
// initialize response; compaction itself is controlled by client opt-in (the
// client sends experimental.compact_responses: true in its capabilities) or
// the deprecated MCP_COMPACT_RESPONSES=1 environment variable. When neither
// is set the response is indented with two spaces.
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
