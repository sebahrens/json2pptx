package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/template"
)

// handleTestSpec is a five-slide DeckSpec: enough slides that "which one did my
// patch touch" is a real question.
const handleTestSpec = `{
  "meta": {"title": "Quarterly Review", "template": "midnight-blue"},
  "slides": [
    {"kind": "title", "title": "Quarterly Review", "subtitle": "FY26 Q2"},
    {"kind": "section", "title": "Where we stand"},
    {"kind": "kpi_snapshot", "title": "The numbers", "kpis": [{"value": "42%", "label": "Growth"}, {"value": "1.2M", "label": "ARR"}]},
    {"kind": "section", "title": "What we do next"},
    {"kind": "closing", "title": "Questions?"}
  ]
}`

// handleTestConfig is a config with a live handle store.
func handleTestConfig(t *testing.T) *mcpConfig {
	t.Helper()
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(time.Hour),
		deckHandles:  newDeckHandleStore(deckHandleTTL),
	}
}

// deckSpecEnvelope re-decodes a validate_deck_spec result.
func deckSpecEnvelope(t *testing.T, res *mcp.CallToolResult) deckSpecEnvelopeResponse {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool returned an error result: %+v", res.Content)
	}
	var out deckSpecEnvelopeResponse
	structuredInto(t, res.StructuredContent, &out)
	return out
}

// TestDeckHandleRoundTrip is the go-slide-creator-voxp acceptance test: a spec
// validated once comes back with a deck_id, and the revision that follows is a
// deck_id plus a one-op patch — no spec — which the server applies to its own
// copy and reports back as a single changed slide.
func TestDeckHandleRoundTrip(t *testing.T) {
	mc := handleTestConfig(t)

	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": handleTestSpec}))
	if !first.OK {
		t.Fatalf("baseline spec should validate clean, got %+v", first.Findings)
	}
	if first.DeckID == "" {
		t.Fatal("validate_deck_spec returned no deck_id — the revision has nothing to hold on to")
	}
	if len(first.ChangedSlides) != 0 {
		t.Errorf("a first call changed nothing, got changed_slides=%v", first.ChangedSlides)
	}

	// The whole revision: a handle and one op. This is the request body the bead
	// measured as ~120 bytes against a 3,695-byte spec re-upload.
	patch := []any{map[string]any{"op": "replace", "path": "/slides/3/title", "value": "Where we go next"}}
	args := map[string]any{"deck_id": first.DeckID, "patch": patch}
	body, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal patch call: %v", err)
	}
	if len(body) > 200 {
		t.Errorf("a one-field revision costs %d request bytes, want a small patch call", len(body))
	}

	second := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, args))
	if !second.OK {
		t.Fatalf("patched spec should validate clean, got %+v", second.Findings)
	}
	if second.DeckID != first.DeckID {
		t.Errorf("deck_id changed across a patch: %q → %q; one handle should cover the session", first.DeckID, second.DeckID)
	}
	if want := []int{3}; !equalInts(second.ChangedSlides, want) {
		t.Errorf("changed_slides = %v, want %v", second.ChangedSlides, want)
	}

	// The patch persisted: a third call with the handle alone sees the new title
	// and reports nothing changed.
	third := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID}))
	if len(third.ChangedSlides) != 0 {
		t.Errorf("an unpatched handle call changed nothing, got %v", third.ChangedSlides)
	}
	stored, ok := mc.deckHandles.Load(first.DeckID)
	if !ok {
		t.Fatal("handle disappeared after three calls")
	}
	if !strings.Contains(string(stored.Spec), "Where we go next") {
		t.Errorf("stored spec did not keep the patch:\n%s", stored.Spec)
	}
}

// TestDeckHandleExpiry pins the TTL contract: a handle older than the TTL is
// gone, and using it is a diagnosed error naming spec as the way back — not a
// silent render of a stale deck.
func TestDeckHandleExpiry(t *testing.T) {
	ctx := context.Background()
	mc := handleTestConfig(t)

	clock := time.Now()
	mc.deckHandles.now = func() time.Time { return clock }

	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": handleTestSpec}))
	id := env.DeckID
	if id == "" {
		t.Fatal("no deck_id to expire")
	}
	if _, ok := mc.deckHandles.Load(id); !ok {
		t.Fatal("handle not loadable immediately after it was minted")
	}

	// Just inside the TTL: still there.
	clock = clock.Add(deckHandleTTL - time.Minute)
	if _, ok := mc.deckHandles.Load(id); !ok {
		t.Fatalf("handle expired %v early", time.Minute)
	}

	// Past it: gone, and the call that names it is an error.
	clock = clock.Add(2 * time.Minute)
	if _, ok := mc.deckHandles.Load(id); ok {
		t.Fatal("handle survived its TTL")
	}
	res, err := mc.handleValidateDeckSpec(ctx, makeRequest(map[string]any{"deck_id": id}))
	if err != nil {
		t.Fatalf("validate_deck_spec(expired deck_id) returned a go error: %v", err)
	}
	if !res.IsError {
		t.Fatal("an expired deck_id must be a tool error, not a silent pass")
	}
	text := resultText(res)
	if !strings.Contains(text, "expired") || !strings.Contains(text, "spec") {
		t.Errorf("expiry error should say the handle expired and name spec as the way back, got:\n%s", text)
	}
}

// TestDeckHandleRefreshesTTLOnUse pins that an actively edited deck does not
// expire out from under a long revision loop.
func TestDeckHandleRefreshesTTLOnUse(t *testing.T) {
	mc := handleTestConfig(t)
	clock := time.Now()
	mc.deckHandles.now = func() time.Time { return clock }

	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": handleTestSpec}))
	id := env.DeckID

	// Touch it every half-TTL for three TTLs' worth of wall clock.
	for i := 0; i < 6; i++ {
		clock = clock.Add(deckHandleTTL / 2)
		if _, ok := mc.deckHandles.Load(id); !ok {
			t.Fatalf("handle expired at step %d despite continuous use", i)
		}
		res := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id}))
		if res.DeckID != id {
			t.Fatalf("step %d: deck_id changed to %q", i, res.DeckID)
		}
	}
}

// TestDeckHandleExactlyOneSpecSource pins the three accepted call forms: spec,
// deck_id, or deck_id+patch. Sending both, or a patch with no handle, is a
// diagnosed error rather than a guess at which the caller meant.
func TestDeckHandleExactlyOneSpecSource(t *testing.T) {
	mc := handleTestConfig(t)

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"spec and deck_id", map[string]any{"spec": handleTestSpec, "deck_id": "deck_abc"}, "not both"},
		{"patch without deck_id", map[string]any{"spec": handleTestSpec, "patch": []any{}}, "needs deck_id"},
		{"neither", map[string]any{}, "spec"},
		{"unknown deck_id", map[string]any{"deck_id": "deck_nope"}, "unknown or expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(tc.args))
			if err != nil {
				t.Fatalf("go error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("want a tool error, got %+v", res.StructuredContent)
			}
			if text := resultText(res); !strings.Contains(text, tc.want) {
				t.Errorf("error text should mention %q, got:\n%s", tc.want, text)
			}
		})
	}
}

// TestSpecPatchOps covers the patch vocabulary against a stored deck, including
// the two structural edits a revision needs (insert a slide, drop one) and what
// each reports as changed.
func TestSpecPatchOps(t *testing.T) {
	newSlide := map[string]any{"kind": "section", "title": "Competition"}
	cases := []struct {
		name    string
		ops     []any
		want    []int
		slides  int
		contain string
	}{
		{
			name:    "replace a field",
			ops:     []any{map[string]any{"op": "replace", "path": "/slides/0/subtitle", "value": "FY26 Q3"}},
			want:    []int{0},
			slides:  5,
			contain: "FY26 Q3",
		},
		{
			name:    "insert a slide shifts the tail",
			ops:     []any{map[string]any{"op": "add", "path": "/slides/2", "value": newSlide}},
			want:    []int{2, 3, 4, 5},
			slides:  6,
			contain: "Competition",
		},
		{
			name:   "append a slide",
			ops:    []any{map[string]any{"op": "add", "path": "/slides/-", "value": newSlide}},
			want:   []int{5},
			slides: 6,
		},
		{
			name:   "remove a slide",
			ops:    []any{map[string]any{"op": "remove", "path": "/slides/1"}},
			want:   []int{1, 2, 3},
			slides: 4,
		},
		{
			name:    "retarget the template",
			ops:     []any{map[string]any{"op": "replace", "path": "/meta/template", "value": "forest-green"}},
			want:    nil,
			slides:  5,
			contain: "forest-green",
		},
		{
			name: "several ops apply in order",
			ops: []any{
				map[string]any{"op": "replace", "path": "/slides/1/title", "value": "Standing"},
				map[string]any{"op": "add", "path": "/slides/-", "value": newSlide},
			},
			want:   []int{1, 5},
			slides: 6,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mc := handleTestConfig(t)
			first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": handleTestSpec}))
			got := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "patch": tc.ops}))
			if !equalInts(got.ChangedSlides, tc.want) {
				t.Errorf("changed_slides = %v, want %v", got.ChangedSlides, tc.want)
			}
			stored, ok := mc.deckHandles.Load(first.DeckID)
			if !ok {
				t.Fatal("handle lost")
			}
			if n := len(slideDigests(stored.Spec)); n != tc.slides {
				t.Errorf("stored spec has %d slides, want %d", n, tc.slides)
			}
			if tc.contain != "" && !strings.Contains(string(stored.Spec), tc.contain) {
				t.Errorf("stored spec missing %q:\n%s", tc.contain, stored.Spec)
			}
		})
	}
}

// TestSpecPatchRejectsBadOps pins that a malformed patch is refused with the
// offending index and a copy-ready example, rather than partially applied.
func TestSpecPatchRejectsBadOps(t *testing.T) {
	cases := []struct {
		name string
		ops  any
		want string
	}{
		{"unknown op", []any{map[string]any{"op": "frobnicate", "path": "/slides/0/title", "value": "x"}}, "unknown patch op"},
		{"relative path", []any{map[string]any{"op": "replace", "path": "slides/0/title", "value": "x"}}, "JSON Pointer"},
		{"missing value", []any{map[string]any{"op": "replace", "path": "/slides/0/title"}}, "needs a value"},
		{"index past the end", []any{map[string]any{"op": "replace", "path": "/slides/9/title", "value": "x"}}, "outside the array"},
		{"replace a field that is not there", []any{map[string]any{"op": "replace", "path": "/meta/client", "value": "x"}}, "use add to create it"},
		{"root", []any{map[string]any{"op": "replace", "path": "/", "value": map[string]any{}}}, "not the document root"},
		{"not an array", map[string]any{"op": "replace"}, "array of {op, path, value}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mc := handleTestConfig(t)
			first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": handleTestSpec}))
			res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"deck_id": first.DeckID, "patch": tc.ops}))
			if err != nil {
				t.Fatalf("go error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("want a tool error, got %+v", res.StructuredContent)
			}
			if text := resultText(res); !strings.Contains(text, tc.want) {
				t.Errorf("error text should mention %q, got:\n%s", tc.want, text)
			}
			// A refused patch leaves the stored deck exactly as it was.
			stored, _ := mc.deckHandles.Load(first.DeckID)
			if n := len(slideDigests(stored.Spec)); n != 5 {
				t.Errorf("a refused patch mutated the stored deck: %d slides", n)
			}
		})
	}
}

// TestDeckHandleStoreIsNilTolerant pins that a config without a handle store
// (the CLI paths and older tests) still works: no deck_id, no panic.
func TestDeckHandleStoreIsNilTolerant(t *testing.T) {
	var store *deckHandleStore
	if id := store.Save(&deckHandle{}); id != "" {
		t.Errorf("nil store minted id %q", id)
	}
	store.Update("deck_x", &deckHandle{})
	if _, ok := store.Load("deck_x"); ok {
		t.Error("nil store loaded something")
	}

	mc := &mcpConfig{templatesDir: "../../templates"}
	res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": handleTestSpec}))
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	env := deckSpecEnvelope(t, res)
	if env.DeckID != "" {
		t.Errorf("a server without a handle store must omit deck_id, got %q", env.DeckID)
	}
}

// mustCall invokes an MCP handler with the given arguments.
func mustCall(t *testing.T, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := h(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("handler returned a go error: %v", err)
	}
	return res
}

func equalInts(got, want []int) bool {
	return slices.Equal(got, want)
}

// TestDeckHandlePatchesAYAMLSpec pins that the handle normalizes what it stores:
// a YAML spec is patchable, and the stored copy re-parses as the deck it was.
func TestDeckHandlePatchesAYAMLSpec(t *testing.T) {
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": validSemanticSpec}))
	if !first.OK {
		t.Fatalf("baseline YAML spec should validate clean, got %+v", first.Findings)
	}
	if first.DeckID == "" {
		t.Fatal("a YAML spec should still get a deck_id")
	}
	second := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"deck_id": first.DeckID,
		"patch":   []any{map[string]any{"op": "replace", "path": "/slides/1/title", "value": "Thank you"}},
	}))
	if !second.OK {
		t.Fatalf("patched YAML spec should validate clean, got %+v", second.Findings)
	}
	if want := []int{1}; !equalInts(second.ChangedSlides, want) {
		t.Errorf("changed_slides = %v, want %v", second.ChangedSlides, want)
	}
	stored, _ := mc.deckHandles.Load(first.DeckID)
	if !strings.HasSuffix(stored.Filename, ".json") {
		t.Errorf("stored filename %q must claim JSON — the stored bytes are JSON and Parse dispatches on the extension", stored.Filename)
	}
	if !strings.Contains(string(stored.Spec), "Thank you") {
		t.Errorf("stored spec lost the patch:\n%s", stored.Spec)
	}
}
