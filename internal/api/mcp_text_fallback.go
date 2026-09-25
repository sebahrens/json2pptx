package api

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/server"
)

// The text copy of every tool response (go-slide-creator-vxre).
//
// MCPSuccessResult put the whole payload in Content[0].text AND in
// StructuredContent. One pass over the 18 callable core tools weighed
// 221,206 B of text plus 168,072 B of structuredContent — 389 KB on the wire
// for 168 KB of information, of which 53,134 B was nothing but two-space
// indentation. The bloat scales with the input too: a 1 MB title produced a
// 2,003,105 B response because both copies echo it.
//
// Compact JSON is the default for older clients. Modern clients receive a
// bounded text synopsis alongside complete structuredContent, so a host that
// ignores structuredContent still sees a useful result without a large copy.

// TextFallbackMode decides whether Content carries the complete JSON copy or
// only a bounded synopsis. Content is never empty for a JSON tool result.
type TextFallbackMode string

const (
	// TextFallbackAuto sends a synopsis on modern sessions and the complete
	// JSON fallback on older or unrecognized sessions. This is the default.
	TextFallbackAuto TextFallbackMode = "auto"
	// TextFallbackAlways keeps the text copy for every client — the pre-
	// go-slide-creator-vxre behaviour, for a client that reads content[0].text
	// even though it negotiated a modern protocol.
	TextFallbackAlways TextFallbackMode = "always"
	// TextFallbackNever uses a synopsis unconditionally (never a full copy).
	TextFallbackNever TextFallbackMode = "never"
)

// structuredContentProtocol is the first protocol version in which
// structuredContent is part of the tool-result contract, so a client that
// negotiated it (or anything later) can be relied on to read it.
const structuredContentProtocol = "2025-06-18"

var (
	textFallbackMu   sync.RWMutex
	textFallbackMode = TextFallbackAuto

	protocolMu       sync.RWMutex
	protocolVersions = map[string]string{}
)

// SetTextFallbackMode sets the server-wide policy. An unrecognised value is
// ignored, leaving the current mode in place.
func SetTextFallbackMode(mode TextFallbackMode) {
	switch mode {
	case TextFallbackAuto, TextFallbackAlways, TextFallbackNever:
		textFallbackMu.Lock()
		textFallbackMode = mode
		textFallbackMu.Unlock()
	}
}

// RecordProtocolVersion remembers the protocol version a session negotiated.
// The MCP session interface exposes client info and capabilities but not the
// negotiated version, so the server records it from the initialize request.
func RecordProtocolVersion(ctx context.Context, version string) {
	id := sessionID(ctx)
	if id == "" || version == "" {
		return
	}
	protocolMu.Lock()
	protocolVersions[id] = version
	protocolMu.Unlock()
}

// ForgetProtocolVersion drops a finished session's recorded version.
func ForgetProtocolVersion(ctx context.Context) {
	id := sessionID(ctx)
	if id == "" {
		return
	}
	protocolMu.Lock()
	delete(protocolVersions, id)
	protocolMu.Unlock()
}

// includeTextFallback reports whether this response should carry the complete
// JSON text copy. A false return still sends a bounded synopsis.
func includeTextFallback(ctx context.Context) bool {
	textFallbackMu.RLock()
	mode := textFallbackMode
	textFallbackMu.RUnlock()

	switch mode {
	case TextFallbackAlways:
		return true
	case TextFallbackNever:
		return false
	}
	// auto: keep the copy unless the client negotiated a protocol that
	// guarantees structuredContent. An unknown version (no initialize seen, a
	// direct handler call in a test) keeps it — the safe direction.
	return negotiatedProtocolVersion(ctx) < structuredContentProtocol
}

// negotiatedProtocolVersion returns the version recorded for this session, or
// "" when none was.
func negotiatedProtocolVersion(ctx context.Context) string {
	id := sessionID(ctx)
	if id == "" {
		return ""
	}
	protocolMu.RLock()
	defer protocolMu.RUnlock()
	return protocolVersions[id]
}

// sessionID returns the MCP session ID in ctx, or "".
func sessionID(ctx context.Context) string {
	session := server.ClientSessionFromContext(ctx)
	if session == nil {
		return ""
	}
	return session.SessionID()
}
