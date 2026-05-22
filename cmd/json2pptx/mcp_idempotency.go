// mcp_idempotency.go implements an in-process idempotency cache for the
// long-running MCP tools (generate_presentation, auto_repair, make_deck).
//
// Why this exists: MCP transport layers retry on timeouts. Without an
// idempotency token, every retry runs the tool again from scratch and writes
// a fresh output file (output.pptx, output_1.pptx, output_2.pptx, …). The
// caller is also charged again for inference + render cost even though the
// first call already succeeded.
//
// Contract: the caller passes a stable string in `idempotency_key`. The
// server hashes that into a tool-scoped cache key and stores the structured
// success response. Subsequent calls with the same key replay the cached
// response (marked `idempotent_replay: true`) instead of regenerating.
//
// Scope: only successful responses are cached. Error responses surface
// every time so the agent can fix bad input. The cache is per-process —
// restarting the MCP server drops it.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// idempotencyCacheTTL bounds how long a cached response stays replayable.
// One hour comfortably covers transport-retry windows while preventing the
// cache from holding stale outputs indefinitely.
const idempotencyCacheTTL = time.Hour

type idempotencyEntry struct {
	data        any
	fingerprint string
	expiresAt   time.Time
}

// idempotencyStatus is the outcome of an idempotency cache lookup.
type idempotencyStatus int

const (
	// idempotencyMiss: no live entry for this (tool, key) — caller should run
	// the request and Set the result.
	idempotencyMiss idempotencyStatus = iota
	// idempotencyHit: a live entry exists whose request fingerprint matches —
	// caller should replay the cached response.
	idempotencyHit
	// idempotencyConflict: a live entry exists but its request fingerprint
	// differs — replaying would hand back a deck for the wrong content, so the
	// caller must refuse with an IDEMPOTENCY_CONFLICT diagnostic.
	idempotencyConflict
)

// idempotencyCache is a simple in-memory cache keyed by `<tool>:<key>` with a
// TTL. Safe for concurrent use across MCP handler goroutines.
type idempotencyCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]idempotencyEntry
	now     func() time.Time
}

func newIdempotencyCache(ttl time.Duration) *idempotencyCache {
	return &idempotencyCache{
		ttl:     ttl,
		entries: make(map[string]idempotencyEntry),
		now:     time.Now,
	}
}

// Lookup returns the cached response for the (tool, key) pair together with the
// outcome status:
//
//   - idempotencyHit: data is the cached response and stored is the matching
//     fingerprint.
//   - idempotencyConflict: data is nil and stored is the fingerprint of the
//     original request held under this key (caller-supplied fingerprint differs).
//   - idempotencyMiss: data is nil, stored is empty (no live entry, empty key,
//     or nil receiver).
//
// Treats nil receivers as "no cache" so callers can leave the field unset in
// tests without crashing.
func (c *idempotencyCache) Lookup(tool, key, fingerprint string) (data any, stored string, status idempotencyStatus) {
	if c == nil || key == "" {
		return nil, "", idempotencyMiss
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	full := cacheKey(tool, key)
	entry, ok := c.entries[full]
	if !ok {
		return nil, "", idempotencyMiss
	}
	if c.now().After(entry.expiresAt) {
		delete(c.entries, full)
		return nil, "", idempotencyMiss
	}
	if entry.fingerprint != fingerprint {
		return nil, entry.fingerprint, idempotencyConflict
	}
	return entry.data, entry.fingerprint, idempotencyHit
}

// Set stores the structured content under the (tool, key) pair along with the
// request fingerprint that produced it. No-op when the cache is nil, the key is
// empty, or the data is nil.
func (c *idempotencyCache) Set(tool, key, fingerprint string, data any) {
	if c == nil || key == "" || data == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey(tool, key)] = idempotencyEntry{
		data:        data,
		fingerprint: fingerprint,
		expiresAt:   c.now().Add(c.ttl),
	}
}

func cacheKey(tool, key string) string {
	return tool + ":" + key
}

// requestFingerprint computes a stable hash over the request arguments that
// define the requested work, excluding idempotency_key — the key is the retry
// identity, not part of the request identity. Two calls sharing a key but
// carrying different fingerprints are different requests and must not replay
// each other's output.
//
// Go's encoding/json marshals maps with sorted keys (recursively) and preserves
// array order, so logically identical argument sets hash identically regardless
// of how the caller ordered object members.
func requestFingerprint(request mcp.CallToolRequest) string {
	args := request.GetArguments()
	filtered := make(map[string]any, len(args))
	for k, v := range args {
		if k == "idempotency_key" {
			continue
		}
		filtered[k] = v
	}
	b, err := json.Marshal(filtered)
	if err != nil {
		// MCP arguments were already decoded from JSON, so a marshal failure is
		// unexpected. Fall back to a sentinel that still differs from any real
		// hash so an un-marshalable request never silently aliases a cached one.
		return "unfingerprintable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// idempotencyConflictResult builds the MCP error returned when a caller reuses
// an idempotency_key for a request whose normalized fingerprint differs from
// the one stored under that key. Replaying the stale response would return a
// deck for the wrong content, so the server refuses and reports both
// fingerprints: the agent can either issue a new key (intentional new request)
// or restore the original input (accidental edit) to replay.
func idempotencyConflictResult(tool, key, current, original string) *mcp.CallToolResult {
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{
		{
			Code:     diagnostics.CodeIdempotencyConflict,
			Severity: diagnostics.SeverityError,
			Message: fmt.Sprintf(
				"idempotency_key %q was already used by %s for a different request; "+
					"reusing a key with changed input would replay the original result. "+
					"Use a new idempotency_key for new content, or restore the original input to replay.",
				key, tool,
			),
			Details: map[string]any{
				"idempotency_key":      key,
				"tool":                 tool,
				"current_fingerprint":  current,
				"original_fingerprint": original,
			},
		},
	})
}

// idempotencyKeyParamDescription documents the idempotency_key MCP parameter
// uniformly across the tools that accept it.
const idempotencyKeyParamDescription = "Optional caller-supplied retry token. When set, the server caches the first successful response under this key and returns the cached result on subsequent calls within the cache TTL (default 1 hour). Use this to make transport-layer retries safe: without it, retries regenerate the PPTX and create duplicate files (output.pptx, output_1.pptx, …). The cache is per-process and per-tool, so the same key used against different tools never collides."

// idempotencyKeyToolParam returns the WithString builder used to declare the
// idempotency_key parameter on every tool that supports it.
func idempotencyKeyToolParam() mcp.ToolOption {
	return mcp.WithString("idempotency_key", mcp.Description(idempotencyKeyParamDescription))
}

// idempotencyKey reads the idempotency_key argument from a request. Returns
// the empty string when absent or non-string so callers can treat the empty
// string as "no caching requested".
func idempotencyKey(request mcp.CallToolRequest) string {
	raw, ok := request.GetArguments()["idempotency_key"]
	if !ok {
		return ""
	}
	s, _ := raw.(string)
	return s
}
