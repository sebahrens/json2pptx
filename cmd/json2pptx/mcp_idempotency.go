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
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// idempotencyCacheTTL bounds how long a cached response stays replayable.
// One hour comfortably covers transport-retry windows while preventing the
// cache from holding stale outputs indefinitely.
const idempotencyCacheTTL = time.Hour

type idempotencyEntry struct {
	data      any
	expiresAt time.Time
}

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

// Get returns the cached structured content for the (tool, key) pair, or
// (nil, false) when absent / expired. Treats nil receivers as "no cache" so
// callers can leave the field unset in tests without crashing.
func (c *idempotencyCache) Get(tool, key string) (any, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	full := cacheKey(tool, key)
	entry, ok := c.entries[full]
	if !ok {
		return nil, false
	}
	if c.now().After(entry.expiresAt) {
		delete(c.entries, full)
		return nil, false
	}
	return entry.data, true
}

// Set stores the structured content under the (tool, key) pair. No-op when
// the cache is nil, the key is empty, or the data is nil.
func (c *idempotencyCache) Set(tool, key string, data any) {
	if c == nil || key == "" || data == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey(tool, key)] = idempotencyEntry{
		data:      data,
		expiresAt: c.now().Add(c.ttl),
	}
}

func cacheKey(tool, key string) string {
	return tool + ":" + key
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
