package api

import (
	"context"
	"encoding/json"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// MarshalMCPResponse serializes v for MCP tool output.
//
// Compact JSON is the default: the text block is a machine-read fallback, not
// a human document, and one pass over the core tools carried 53,134 B of pure
// two-space indentation (go-slide-creator-vxre). Set JSON2PPTX_MCP_PRETTY=1 to
// indent it when reading raw transcripts by hand.
//
// Responses are always compact JSON; the server still advertises
// experimental.compact_responses: true and still honours the client capability
// and the deprecated MCP_COMPACT_RESPONSES=1 environment variable, but neither
// changes anything.
func MarshalMCPResponse(ctx context.Context, v any) ([]byte, error) {
	if os.Getenv("JSON2PPTX_MCP_PRETTY") == "1" && !isCompactSession(ctx) && os.Getenv("MCP_COMPACT_RESPONSES") != "1" {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
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
