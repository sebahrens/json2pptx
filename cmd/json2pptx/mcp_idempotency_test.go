package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
)

// idempotencyMC builds an mcpConfig wired with an idempotency cache so the
// handler under test can exercise both the miss-and-store and hit-and-replay
// paths.
func idempotencyMC(t *testing.T) *mcpConfig {
	t.Helper()
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
		idempotency:  newIdempotencyCache(idempotencyCacheTTL),
	}
}

// TestIdempotencyCache_GetMissReturnsFalse asserts the empty-cache base case
// and the "no key" base case. Both must report a miss without crashing —
// `handleGenerate` calls Get unconditionally, so failure here would block
// every request.
func TestIdempotencyCache_GetMissReturnsFalse(t *testing.T) {
	c := newIdempotencyCache(time.Hour)

	if _, ok := c.Get("generate_presentation", "unknown-key"); ok {
		t.Fatal("expected miss on empty cache")
	}
	if _, ok := c.Get("generate_presentation", ""); ok {
		t.Fatal("expected miss for empty key")
	}
}

// TestIdempotencyCache_NilReceiverIsSafe asserts the cache treats nil as
// "no cache configured" so tests that omit the field (and existing call sites
// that initialize mcpConfig inline) keep working.
func TestIdempotencyCache_NilReceiverIsSafe(t *testing.T) {
	var c *idempotencyCache

	if _, ok := c.Get("generate_presentation", "k"); ok {
		t.Fatal("expected nil cache to miss")
	}
	c.Set("generate_presentation", "k", "anything") // should not panic
}

// TestIdempotencyCache_SetThenGetHits asserts the round-trip: a Set with a
// non-empty key is visible to a subsequent Get with the same (tool, key) pair.
// Tool namespace must isolate keys — the same key used against a different
// tool must not collide.
func TestIdempotencyCache_SetThenGetHits(t *testing.T) {
	c := newIdempotencyCache(time.Hour)
	payload := JSONOutput{Success: true, OutputPath: "/tmp/x.pptx"}

	c.Set("generate_presentation", "k1", payload)
	got, ok := c.Get("generate_presentation", "k1")
	if !ok {
		t.Fatal("expected hit after set")
	}
	if out, ok := got.(JSONOutput); !ok || out.OutputPath != "/tmp/x.pptx" {
		t.Fatalf("payload round-trip failed: %#v", got)
	}

	// Tool scoping: same key, different tool → miss.
	if _, ok := c.Get("auto_repair", "k1"); ok {
		t.Fatal("key must be scoped per tool")
	}
}

// TestIdempotencyCache_ExpiredEntriesEvict asserts TTL is enforced by the
// injected clock. The expired entry must be dropped from the map on access so
// long-lived servers don't leak entries indefinitely.
func TestIdempotencyCache_ExpiredEntriesEvict(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	c := newIdempotencyCache(time.Minute)
	c.now = func() time.Time { return now }

	c.Set("generate_presentation", "k", JSONOutput{Success: true})
	if _, ok := c.Get("generate_presentation", "k"); !ok {
		t.Fatal("expected hit before expiry")
	}

	now = now.Add(2 * time.Minute)
	if _, ok := c.Get("generate_presentation", "k"); ok {
		t.Fatal("expected miss after TTL elapsed")
	}
	// Eviction is lazy but must happen on access. The next Get must still
	// miss (entry stays gone after expiry).
	if _, ok := c.Get("generate_presentation", "k"); ok {
		t.Fatal("expired entry must be evicted, not refreshed")
	}
}

// TestHandleGenerate_IdempotencyKeyReplaysResponse is the integration check:
// two calls to generate_presentation with the same idempotency_key must return
// the same output_path and the second response must carry idempotent_replay=true.
// The first PPTX must exist on disk; the second call must not write a duplicate
// (e.g. output_1.pptx) — that's the whole point of the feature.
func TestHandleGenerate_IdempotencyKeyReplaysResponse(t *testing.T) {
	mc := idempotencyMC(t)

	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "title",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hello"},
				},
			},
		},
	}

	req := map[string]any{
		"presentation":    deck,
		"output_filename": "idem.pptx",
		"idempotency_key": "agent-retry-token-1",
	}

	first, err := mc.handleGenerate(context.Background(), makeRequest(req))
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if first.IsError {
		t.Fatalf("first call returned error: %s", textContent(first))
	}
	var firstOut JSONOutput
	if err := json.Unmarshal([]byte(textContent(first)), &firstOut); err != nil {
		t.Fatalf("unmarshal first response: %v", err)
	}
	if firstOut.IdempotentReplay {
		t.Error("first call must not be marked as a replay")
	}
	if firstOut.OutputPath == "" {
		t.Fatal("first response missing output_path")
	}
	if _, err := os.Stat(firstOut.OutputPath); err != nil {
		t.Fatalf("first PPTX missing on disk: %v", err)
	}

	second, err := mc.handleGenerate(context.Background(), makeRequest(req))
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if second.IsError {
		t.Fatalf("second call returned error: %s", textContent(second))
	}
	var secondOut JSONOutput
	if err := json.Unmarshal([]byte(textContent(second)), &secondOut); err != nil {
		t.Fatalf("unmarshal second response: %v", err)
	}
	if !secondOut.IdempotentReplay {
		t.Error("second call must be marked as idempotent_replay=true")
	}
	if secondOut.OutputPath != firstOut.OutputPath {
		t.Errorf("replay returned different output_path: first=%q second=%q", firstOut.OutputPath, secondOut.OutputPath)
	}

	// The cached entry must not have been mutated by the replay — fresh callers
	// served from the cache should keep seeing idempotent_replay=true, but the
	// stored copy itself must remain unmarked so we know it's the original.
	cached, ok := mc.idempotency.Get("generate_presentation", "agent-retry-token-1")
	if !ok {
		t.Fatal("expected cache to retain entry after replay")
	}
	if cachedOut, ok := cached.(JSONOutput); !ok || cachedOut.IdempotentReplay {
		t.Errorf("cached entry must remain unmarked: %#v", cached)
	}
}

// TestHandleGenerate_DifferentKeysDoNotReplay asserts the cache key actually
// discriminates: two calls with different idempotency_key values must both
// produce fresh responses (no false-positive replays).
func TestHandleGenerate_DifferentKeysDoNotReplay(t *testing.T) {
	mc := idempotencyMC(t)

	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "title",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hello"},
				},
			},
		},
	}

	req1 := map[string]any{
		"presentation":    deck,
		"output_filename": "idem_a.pptx",
		"idempotency_key": "key-a",
	}
	req2 := map[string]any{
		"presentation":    deck,
		"output_filename": "idem_b.pptx",
		"idempotency_key": "key-b",
	}

	first, _ := mc.handleGenerate(context.Background(), makeRequest(req1))
	second, _ := mc.handleGenerate(context.Background(), makeRequest(req2))

	if first.IsError || second.IsError {
		t.Fatalf("expected both calls to succeed; first.IsError=%v second.IsError=%v", first.IsError, second.IsError)
	}

	var firstOut, secondOut JSONOutput
	_ = json.Unmarshal([]byte(textContent(first)), &firstOut)
	_ = json.Unmarshal([]byte(textContent(second)), &secondOut)

	if firstOut.IdempotentReplay || secondOut.IdempotentReplay {
		t.Error("distinct keys must not trigger replay")
	}
}

// TestHandleGenerate_NoIdempotencyKeyAlwaysFreshens asserts the absence of
// idempotency_key disables caching — two identical calls must both produce
// fresh responses so callers who opted out of the feature retain prior
// behaviour (regeneration on every call).
func TestHandleGenerate_NoIdempotencyKeyAlwaysFreshens(t *testing.T) {
	mc := idempotencyMC(t)

	req := map[string]any{
		"presentation": map[string]any{
			"template": "midnight-blue",
			"slides": []any{
				map[string]any{
					"layout_id": "title",
					"content": []any{
						map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hello"},
					},
				},
			},
		},
		"output_filename": "no_idem.pptx",
	}

	first, _ := mc.handleGenerate(context.Background(), makeRequest(req))
	second, _ := mc.handleGenerate(context.Background(), makeRequest(req))

	if first.IsError || second.IsError {
		t.Fatalf("expected both calls to succeed; first.IsError=%v second.IsError=%v", first.IsError, second.IsError)
	}

	var firstOut, secondOut JSONOutput
	_ = json.Unmarshal([]byte(textContent(first)), &firstOut)
	_ = json.Unmarshal([]byte(textContent(second)), &secondOut)

	if firstOut.IdempotentReplay || secondOut.IdempotentReplay {
		t.Error("without idempotency_key, responses must never be replays")
	}
}
