package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestSessionStoresSweepExpiredEntries pins go-slide-creator-csclk.84: every
// per-process MCP store drops expired entries on insert instead of retaining
// them until the same key is looked up again.
func TestSessionStoresSweepExpiredEntries(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }

	decks := newDeckHandleStore(time.Hour)
	decks.now = clock
	loops := newLoopSessionStore(time.Hour)
	loops.now = clock
	idem := newIdempotencyCache(time.Hour)
	idem.now = clock

	for i := 0; i < 200; i++ {
		decks.Save(&deckHandle{Spec: []byte("{}")})
		loops.Save(&loopCheckpoint{})
		idem.Set("t", "k"+strings.Repeat("x", i), "fp", "data")
	}
	now = now.Add(48 * time.Hour)
	decks.Save(&deckHandle{Spec: []byte("{}")})
	loops.Save(&loopCheckpoint{})
	idem.Set("t", "fresh", "fp", "data")

	if n := len(decks.entries); n != 1 {
		t.Errorf("deck handles = %d, want 1", n)
	}
	if n := len(loops.entries); n != 1 {
		t.Errorf("loop sessions = %d, want 1", n)
	}
	if n := len(idem.entries); n != 1 {
		t.Errorf("idempotency entries = %d, want 1", n)
	}

	// Live entries are capped too.
	for i := 0; i < maxSessionStoreEntries+50; i++ {
		decks.Save(&deckHandle{Spec: []byte("{}")})
	}
	if n := len(decks.entries); n > maxSessionStoreEntries {
		t.Errorf("deck handles = %d, want <= %d", n, maxSessionStoreEntries)
	}
}

// TestDeckHandleUpdateIsCompareAndSwap pins go-slide-creator-csclk.123: a
// patch computed from a stale base must not overwrite a newer stored spec.
func TestDeckHandleUpdateIsCompareAndSwap(t *testing.T) {
	store := newDeckHandleStore(time.Hour)
	id := store.Save(&deckHandle{Spec: []byte(`{"v":0}`)})
	base := []byte(`{"v":0}`)
	if !store.Update(id, base, &deckHandle{Spec: []byte(`{"v":1}`)}) {
		t.Fatal("first update from the current base was refused")
	}
	if store.Update(id, base, &deckHandle{Spec: []byte(`{"v":2}`)}) {
		t.Fatal("second update from a stale base was accepted")
	}
	h, _ := store.Load(id)
	if string(h.Spec) != `{"v":1}` {
		t.Errorf("stored spec = %s, want the first writer's", h.Spec)
	}
}

// TestArgSizeMiddlewareRejectsOversizedCalls pins go-slide-creator-csclk.131.
func TestArgSizeMiddlewareRejectsOversizedCalls(t *testing.T) {
	called := false
	h := argSizeMiddleware()(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return &mcp.CallToolResult{}, nil
	})
	res, _ := h(context.Background(), makeRequest(map[string]any{"json": strings.Repeat("a", maxMCPArgumentBytes+1)}))
	if called || res == nil || !res.IsError {
		t.Fatalf("oversized call reached the handler (called=%v)", called)
	}
	_, _ = h(context.Background(), makeRequest(map[string]any{"json": "{}"}))
	if !called {
		t.Error("small call was refused")
	}
}
