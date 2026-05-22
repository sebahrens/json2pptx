package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
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

// TestIdempotencyCache_LookupMissReturnsMiss asserts the empty-cache base case
// and the "no key" base case. Both must report a miss without crashing —
// `handleGenerate` calls Lookup unconditionally, so failure here would block
// every request.
func TestIdempotencyCache_LookupMissReturnsMiss(t *testing.T) {
	c := newIdempotencyCache(time.Hour)

	if _, _, status := c.Lookup("generate_presentation", "unknown-key", "fp"); status != idempotencyMiss {
		t.Fatalf("expected miss on empty cache, got %v", status)
	}
	if _, _, status := c.Lookup("generate_presentation", "", "fp"); status != idempotencyMiss {
		t.Fatalf("expected miss for empty key, got %v", status)
	}
}

// TestIdempotencyCache_NilReceiverIsSafe asserts the cache treats nil as
// "no cache configured" so tests that omit the field (and existing call sites
// that initialize mcpConfig inline) keep working.
func TestIdempotencyCache_NilReceiverIsSafe(t *testing.T) {
	var c *idempotencyCache

	if _, _, status := c.Lookup("generate_presentation", "k", "fp"); status != idempotencyMiss {
		t.Fatalf("expected nil cache to miss, got %v", status)
	}
	c.Set("generate_presentation", "k", "fp", "anything") // should not panic
}

// TestIdempotencyCache_SetThenLookupHits asserts the round-trip: a Set with a
// non-empty key is visible to a subsequent Lookup with the same (tool, key,
// fingerprint) triple. Tool namespace must isolate keys — the same key used
// against a different tool must not collide — and a different fingerprint under
// the same key must be reported as a conflict, not a hit.
func TestIdempotencyCache_SetThenLookupHits(t *testing.T) {
	c := newIdempotencyCache(time.Hour)
	payload := JSONOutput{Success: true, OutputPath: "/tmp/x.pptx"}

	c.Set("generate_presentation", "k1", "fp-1", payload)
	got, stored, status := c.Lookup("generate_presentation", "k1", "fp-1")
	if status != idempotencyHit {
		t.Fatalf("expected hit after set, got %v", status)
	}
	if stored != "fp-1" {
		t.Errorf("stored fingerprint = %q, want fp-1", stored)
	}
	if out, ok := got.(JSONOutput); !ok || out.OutputPath != "/tmp/x.pptx" {
		t.Fatalf("payload round-trip failed: %#v", got)
	}

	// Same key, different fingerprint → conflict (not a hit, not a miss). The
	// stored fingerprint of the original request must be reported back.
	data, original, status := c.Lookup("generate_presentation", "k1", "fp-2")
	if status != idempotencyConflict {
		t.Fatalf("expected conflict on fingerprint mismatch, got %v", status)
	}
	if data != nil {
		t.Errorf("conflict must not return cached data, got %#v", data)
	}
	if original != "fp-1" {
		t.Errorf("conflict must report original fingerprint fp-1, got %q", original)
	}

	// Tool scoping: same key, different tool → miss.
	if _, _, status := c.Lookup("auto_repair", "k1", "fp-1"); status != idempotencyMiss {
		t.Fatalf("key must be scoped per tool, got %v", status)
	}
}

// TestIdempotencyCache_ExpiredEntriesEvict asserts TTL is enforced by the
// injected clock. The expired entry must be dropped from the map on access so
// long-lived servers don't leak entries indefinitely.
func TestIdempotencyCache_ExpiredEntriesEvict(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	c := newIdempotencyCache(time.Minute)
	c.now = func() time.Time { return now }

	c.Set("generate_presentation", "k", "fp", JSONOutput{Success: true})
	if _, _, status := c.Lookup("generate_presentation", "k", "fp"); status != idempotencyHit {
		t.Fatalf("expected hit before expiry, got %v", status)
	}

	now = now.Add(2 * time.Minute)
	if _, _, status := c.Lookup("generate_presentation", "k", "fp"); status != idempotencyMiss {
		t.Fatalf("expected miss after TTL elapsed, got %v", status)
	}
	// Eviction is lazy but must happen on access. The next Lookup must still
	// miss (entry stays gone after expiry).
	if _, _, status := c.Lookup("generate_presentation", "k", "fp"); status != idempotencyMiss {
		t.Fatalf("expired entry must be evicted, not refreshed, got %v", status)
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
	cached, _, status := mc.idempotency.Lookup("generate_presentation", "agent-retry-token-1", requestFingerprint(makeRequest(req)))
	if status != idempotencyHit {
		t.Fatalf("expected cache to retain entry after replay, got status %v", status)
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

// TestNewServerMCPConfig_WiresIdempotency asserts the production stdio server
// constructor wires the idempotency cache. Before this fix runMCP built the
// config inline without it, so idempotency_key was silently ignored outside the
// CLI/test helper path. The handlers tolerate a nil cache, so the only way to
// catch the regression is to assert the constructor populates the field.
func TestNewServerMCPConfig_WiresIdempotency(t *testing.T) {
	cfg := config.DefaultConfig()
	mc := newServerMCPConfig(cfg)

	if mc.idempotency == nil {
		t.Fatal("production MCP config must wire an idempotency cache so idempotency_key works in real server use")
	}
	// Sanity-check the cache is functional, not just non-nil.
	mc.idempotency.Set("generate_presentation", "k", "fp", JSONOutput{Success: true})
	if _, _, status := mc.idempotency.Lookup("generate_presentation", "k", "fp"); status != idempotencyHit {
		t.Fatalf("wired idempotency cache did not round-trip, got status %v", status)
	}
}

// assertIdempotencyConflict verifies result is an IDEMPOTENCY_CONFLICT error
// carrying both the current and original request fingerprints, and that they
// differ (the whole point of the conflict).
func assertIdempotencyConflict(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	if result == nil {
		t.Fatal("expected a conflict result, got nil")
	}
	if !result.IsError {
		t.Fatalf("expected conflict to be an error result, got success: %s", textContent(result))
	}
	var env diagnostics.FindingEnvelope
	if err := json.Unmarshal([]byte(textContent(result)), &env); err != nil {
		t.Fatalf("unmarshal conflict envelope: %v", err)
	}
	var found *diagnostics.Finding
	for i := range env.Findings {
		if strings.Contains(env.Findings[i].Code, "IDEMPOTENCY_CONFLICT") {
			found = &env.Findings[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected IDEMPOTENCY_CONFLICT finding, got: %s", textContent(result))
	}
	cur, _ := found.Evidence["current_fingerprint"].(string)
	orig, _ := found.Evidence["original_fingerprint"].(string)
	if cur == "" || orig == "" {
		t.Fatalf("conflict finding must report current and original fingerprints, evidence=%v", found.Evidence)
	}
	if cur == orig {
		t.Fatalf("conflict must report differing fingerprints, both = %q", cur)
	}
}

// TestHandleGenerate_SameKeyDifferentRequestConflicts asserts that reusing an
// idempotency_key for a deck with edited content does NOT replay the stale
// output — it returns an IDEMPOTENCY_CONFLICT diagnostic with both fingerprints.
// Without this, an accidental key reuse after editing input would silently
// return a PPTX for the wrong content.
func TestHandleGenerate_SameKeyDifferentRequestConflicts(t *testing.T) {
	mc := idempotencyMC(t)

	deckA := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "title",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Original"},
				},
			},
		},
	}
	deckB := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "title",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Edited"},
				},
			},
		},
	}

	first, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation":    deckA,
		"output_filename": "conflict.pptx",
		"idempotency_key": "shared-key",
	}))
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if first.IsError {
		t.Fatalf("first call returned error: %s", textContent(first))
	}

	// Same key, different content → conflict, not replay.
	second, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation":    deckB,
		"output_filename": "conflict.pptx",
		"idempotency_key": "shared-key",
	}))
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	assertIdempotencyConflict(t, second)
}

// TestHandleAutoRepair_IdempotencyReplayAndConflict covers both branches for
// auto_repair: a same-key/same-request retry replays the prior response, and a
// same-key/different-request call conflicts instead of replaying stale output.
func TestHandleAutoRepair_IdempotencyReplayAndConflict(t *testing.T) {
	mc := idempotencyMC(t)

	reqA := map[string]any{
		"presentation":    mustParseJSON(autoRepairDeck(2)),
		"max_passes":      float64(1),
		"output_filename": "ar_idem.pptx",
		"idempotency_key": "ar-key",
	}

	first, err := mc.handleAutoRepair(context.Background(), makeRequest(reqA))
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if first.IsError {
		t.Fatalf("first call returned error: %s", textContent(first))
	}
	var firstOut autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(first)), &firstOut); err != nil {
		t.Fatalf("unmarshal first response: %v", err)
	}
	if firstOut.IdempotentReplay {
		t.Error("first call must not be a replay")
	}

	// Same key, same request → replay.
	second, err := mc.handleAutoRepair(context.Background(), makeRequest(reqA))
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if second.IsError {
		t.Fatalf("replay returned error: %s", textContent(second))
	}
	var secondOut autoRepairOutput
	if err := json.Unmarshal([]byte(textContent(second)), &secondOut); err != nil {
		t.Fatalf("unmarshal replay response: %v", err)
	}
	if !secondOut.IdempotentReplay {
		t.Error("same key/same request must replay (idempotent_replay=true)")
	}
	if secondOut.Path != firstOut.Path {
		t.Errorf("replay path differs: first=%q second=%q", firstOut.Path, secondOut.Path)
	}

	// Same key, different request (4 slides instead of 2) → conflict. The
	// conflict short-circuits before the repair loop runs.
	reqB := map[string]any{
		"presentation":    mustParseJSON(autoRepairDeck(4)),
		"max_passes":      float64(1),
		"output_filename": "ar_idem.pptx",
		"idempotency_key": "ar-key",
	}
	third, err := mc.handleAutoRepair(context.Background(), makeRequest(reqB))
	if err != nil {
		t.Fatalf("third call error: %v", err)
	}
	assertIdempotencyConflict(t, third)
}

// TestHandleMakeDeck_IdempotencyReplayAndConflict covers both branches for
// make_deck: a same-key/same-request retry replays the prior response, and a
// same-key/different-outline call conflicts instead of replaying stale output.
func TestHandleMakeDeck_IdempotencyReplayAndConflict(t *testing.T) {
	mc := idempotencyMC(t)

	reqA := map[string]any{
		"outline":         "Pitch our Series B for an AI infrastructure company",
		"template":        "midnight-blue",
		"output_filename": "md_idem.pptx",
		"idempotency_key": "md-key",
		"style_hints":     map[string]any{"slide_budget": float64(4)},
	}

	first, err := mc.handleMakeDeck(context.Background(), makeRequest(reqA))
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if first.IsError {
		t.Fatalf("first call returned error: %s", textContent(first))
	}
	var firstOut makeDeckOutput
	if err := json.Unmarshal([]byte(textContent(first)), &firstOut); err != nil {
		t.Fatalf("unmarshal first response: %v", err)
	}
	if firstOut.IdempotentReplay {
		t.Error("first call must not be a replay")
	}

	// Same key, same request → replay.
	second, err := mc.handleMakeDeck(context.Background(), makeRequest(reqA))
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if second.IsError {
		t.Fatalf("replay returned error: %s", textContent(second))
	}
	var secondOut makeDeckOutput
	if err := json.Unmarshal([]byte(textContent(second)), &secondOut); err != nil {
		t.Fatalf("unmarshal replay response: %v", err)
	}
	if !secondOut.IdempotentReplay {
		t.Error("same key/same request must replay (idempotent_replay=true)")
	}
	if secondOut.Path != firstOut.Path {
		t.Errorf("replay path differs: first=%q second=%q", firstOut.Path, secondOut.Path)
	}

	// Same key, different outline → conflict. The conflict short-circuits
	// before planning/expansion runs.
	reqB := map[string]any{
		"outline":         "Quarterly board update for a logistics company",
		"template":        "midnight-blue",
		"output_filename": "md_idem.pptx",
		"idempotency_key": "md-key",
		"style_hints":     map[string]any{"slide_budget": float64(4)},
	}
	third, err := mc.handleMakeDeck(context.Background(), makeRequest(reqB))
	if err != nil {
		t.Fatalf("third call error: %v", err)
	}
	assertIdempotencyConflict(t, third)
}
