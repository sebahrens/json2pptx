package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// threeSlideDeck returns a minimal valid deck with three title-only slides whose
// titles ("Slide A/B/C") make slide identity easy to assert after a patch.
func threeSlideDeck() string {
	return `{
		"template": "midnight-blue",
		"slides": [
			{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Slide A"}]},
			{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Slide B"}]},
			{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Slide C"}]}
		]
	}`
}

// patchTitles extracts the title text of every slide from a patched deck, in
// order, so reorder/insert/remove/duplicate ops can be asserted positionally.
func patchTitles(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var deck struct {
		Slides []struct {
			Content []struct {
				PlaceholderID string `json:"placeholder_id"`
				TextValue     string `json:"text_value"`
			} `json:"content"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(raw, &deck); err != nil {
		t.Fatalf("parse patched_deck: %v", err)
	}
	titles := make([]string, 0, len(deck.Slides))
	for _, s := range deck.Slides {
		title := ""
		for _, c := range s.Content {
			if c.PlaceholderID == "title" {
				title = c.TextValue
			}
		}
		titles = append(titles, title)
	}
	return titles
}

// applyPatchOK runs apply_deck_patch and fails the test if it returns an error
// result. Returns the decoded success output.
func applyPatchOK(t *testing.T, mc *mcpConfig, deckJSON string, ops []any) applyDeckPatchOutput {
	t.Helper()
	result, err := mc.handleApplyDeckPatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"ops":          ops,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	var out applyDeckPatchOutput
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestApplyDeckPatch_RemoveSlide(t *testing.T) {
	mc := testMCPConfig(t)
	out := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{"op": "remove_slide", "index": 1},
	})
	if got, want := patchTitles(t, out.PatchedDeck), []string{"Slide A", "Slide C"}; !sameStrings(got, want) {
		t.Errorf("remove_slide titles = %v, want %v", got, want)
	}
	if len(out.AppliedOps) != 1 || !out.AppliedOps[0].Applied {
		t.Errorf("expected one applied op, got %+v", out.AppliedOps)
	}
	if out.Findings.SchemaVersion == "" {
		t.Error("expected findings envelope to be present")
	}
	if !out.Findings.OK {
		t.Errorf("expected findings.ok=true for a clean patch, got %s", out.Findings.Summary)
	}
}

func TestApplyDeckPatch_MoveSlide(t *testing.T) {
	mc := testMCPConfig(t)
	out := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{"op": "move_slide", "from": 2, "to": 0},
	})
	if got, want := patchTitles(t, out.PatchedDeck), []string{"Slide C", "Slide A", "Slide B"}; !sameStrings(got, want) {
		t.Errorf("move_slide titles = %v, want %v", got, want)
	}
}

func TestApplyDeckPatch_DuplicateSlide(t *testing.T) {
	mc := testMCPConfig(t)
	out := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{"op": "duplicate_slide", "index": 0},
	})
	if got, want := patchTitles(t, out.PatchedDeck), []string{"Slide A", "Slide A", "Slide B", "Slide C"}; !sameStrings(got, want) {
		t.Errorf("duplicate_slide titles = %v, want %v", got, want)
	}
}

func TestApplyDeckPatch_ReplaceSlide(t *testing.T) {
	mc := testMCPConfig(t)
	out := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{
			"op":    "replace_slide",
			"index": 1,
			"slide": mustParseJSON(`{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"New B"}]}`),
		},
	})
	if got, want := patchTitles(t, out.PatchedDeck), []string{"Slide A", "New B", "Slide C"}; !sameStrings(got, want) {
		t.Errorf("replace_slide titles = %v, want %v", got, want)
	}
}

func TestApplyDeckPatch_InsertSlide(t *testing.T) {
	mc := testMCPConfig(t)
	out := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{
			"op":    "insert_slide",
			"index": 1,
			"slide": mustParseJSON(`{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Inserted"}]}`),
		},
	})
	if got, want := patchTitles(t, out.PatchedDeck), []string{"Slide A", "Inserted", "Slide B", "Slide C"}; !sameStrings(got, want) {
		t.Errorf("insert_slide titles = %v, want %v", got, want)
	}

	// Omitting index appends to the end.
	out2 := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{
			"op":    "insert_slide",
			"slide": mustParseJSON(`{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Appended"}]}`),
		},
	})
	if got, want := patchTitles(t, out2.PatchedDeck), []string{"Slide A", "Slide B", "Slide C", "Appended"}; !sameStrings(got, want) {
		t.Errorf("insert_slide (append) titles = %v, want %v", got, want)
	}
}

func TestApplyDeckPatch_ReplaceField(t *testing.T) {
	mc := testMCPConfig(t)
	out := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{
			"op":    "replace_field",
			"path":  "/slides/0/content/0/text_value",
			"value": "Renamed A",
		},
	})
	if got, want := patchTitles(t, out.PatchedDeck), []string{"Renamed A", "Slide B", "Slide C"}; !sameStrings(got, want) {
		t.Errorf("replace_field titles = %v, want %v", got, want)
	}

	// Replacing a top-level scalar (template) works too.
	out2 := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{"op": "replace_field", "path": "/template", "value": "forest-green"},
	})
	var deck struct {
		Template string `json:"template"`
	}
	if err := json.Unmarshal(out2.PatchedDeck, &deck); err != nil {
		t.Fatalf("parse patched_deck: %v", err)
	}
	if deck.Template != "forest-green" {
		t.Errorf("replace_field /template = %q, want %q", deck.Template, "forest-green")
	}
}

// TestApplyDeckPatch_PreservesUnknownFields confirms the round-trip through the
// generic tree keeps fields the tool does not model (e.g. speaker_notes).
func TestApplyDeckPatch_PreservesUnknownFields(t *testing.T) {
	mc := testMCPConfig(t)
	deck := `{
		"template": "midnight-blue",
		"slides": [
			{"layout_id":"slideLayout2","speaker_notes":"keep me","content":[{"placeholder_id":"title","type":"text","text_value":"Slide A"}]},
			{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Slide B"}]}
		]
	}`
	out := applyPatchOK(t, mc, deck, []any{
		map[string]any{"op": "remove_slide", "index": 1},
	})
	var patched struct {
		Slides []struct {
			SpeakerNotes string `json:"speaker_notes"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(out.PatchedDeck, &patched); err != nil {
		t.Fatalf("parse patched_deck: %v", err)
	}
	if len(patched.Slides) != 1 || patched.Slides[0].SpeakerNotes != "keep me" {
		t.Errorf("speaker_notes not preserved across patch: %+v", patched.Slides)
	}
}

// TestApplyDeckPatch_RegenerateAfterPatch is the acceptance-criteria end-to-end:
// a reorder patch produces a deck that generate_presentation accepts.
func TestApplyDeckPatch_RegenerateAfterPatch(t *testing.T) {
	mc := testMCPConfig(t)
	out := applyPatchOK(t, mc, threeSlideDeck(), []any{
		map[string]any{"op": "move_slide", "from": 0, "to": 2},
		map[string]any{"op": "duplicate_slide", "index": 0},
	})

	genResult, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(string(out.PatchedDeck)),
	}))
	if err != nil {
		t.Fatalf("generate after patch: unexpected error: %v", err)
	}
	if genResult.IsError {
		t.Fatalf("generate after patch: expected success, got error: %s", genResult.Content[0].(mcp.TextContent).Text)
	}
	var resp JSONOutput
	if err := json.Unmarshal([]byte(genResult.Content[0].(mcp.TextContent).Text), &resp); err != nil {
		t.Fatalf("parse generate response: %v", err)
	}
	if !resp.Success {
		t.Error("expected generate success=true after patch")
	}
}

// expectPatchError runs apply_deck_patch and asserts an error result carrying a
// diagnostic with the wanted code, and that no patched_deck leaked.
func expectPatchError(t *testing.T, mc *mcpConfig, deckJSON string, ops []any, wantCode string) {
	t.Helper()
	result, err := mc.handleApplyDeckPatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
		"ops":          ops,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true, got success: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := result.Content[0].(mcp.TextContent).Text
	diags := legacyDiagsFromWire(t, text)
	found := false
	for _, d := range diags {
		if d["code"] == wantCode {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic code %q, got: %s", wantCode, text)
	}
}

func TestApplyDeckPatch_RejectsUnsafeAndSchemaBreaking(t *testing.T) {
	mc := testMCPConfig(t)

	t.Run("index out of range", func(t *testing.T) {
		expectPatchError(t, mc, threeSlideDeck(), []any{
			map[string]any{"op": "remove_slide", "index": 9},
		}, "INVALID_SLIDE_INDEX")
	})

	t.Run("unknown op", func(t *testing.T) {
		expectPatchError(t, mc, threeSlideDeck(), []any{
			map[string]any{"op": "frobnicate_slide", "index": 0},
		}, "UNSUPPORTED")
	})

	t.Run("replace_field path does not exist", func(t *testing.T) {
		expectPatchError(t, mc, threeSlideDeck(), []any{
			map[string]any{"op": "replace_field", "path": "/slides/0/nonexistent", "value": "x"},
		}, "INVALID_PATH")
	})

	t.Run("replace_field array index out of range", func(t *testing.T) {
		expectPatchError(t, mc, threeSlideDeck(), []any{
			map[string]any{"op": "replace_field", "path": "/slides/9/layout_id", "value": "x"},
		}, "INVALID_PATH")
	})

	t.Run("schema break via replace_field", func(t *testing.T) {
		// Replacing the slides array with a string parses as a JSON pointer
		// write but produces a deck that no longer unmarshals as a presentation.
		expectPatchError(t, mc, threeSlideDeck(), []any{
			map[string]any{"op": "replace_field", "path": "/slides", "value": "not an array"},
		}, "INVALID_SLIDE")
	})

	t.Run("missing required field", func(t *testing.T) {
		expectPatchError(t, mc, threeSlideDeck(), []any{
			map[string]any{"op": "replace_slide", "index": 0},
		}, "INVALID_SLIDE")
	})

	t.Run("move_slide missing to", func(t *testing.T) {
		expectPatchError(t, mc, threeSlideDeck(), []any{
			map[string]any{"op": "move_slide", "from": 0},
		}, "INVALID_PARAMETER")
	})
}

// TestApplyDeckPatch_AtomicAbort verifies that when a later op fails, the earlier
// (valid) op does not leak a patched_deck — the whole patch is rejected.
func TestApplyDeckPatch_AtomicAbort(t *testing.T) {
	mc := testMCPConfig(t)
	result, err := mc.handleApplyDeckPatch(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(threeSlideDeck()),
		"ops": []any{
			map[string]any{"op": "remove_slide", "index": 0}, // valid
			map[string]any{"op": "remove_slide", "index": 9}, // invalid → aborts
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for atomic abort, got success: %s", result.Content[0].(mcp.TextContent).Text)
	}
	// The error envelope must not carry a patched_deck.
	text := result.Content[0].(mcp.TextContent).Text
	var anyMap map[string]any
	if err := json.Unmarshal([]byte(text), &anyMap); err == nil {
		if _, ok := anyMap["patched_deck"]; ok {
			t.Errorf("error envelope leaked patched_deck: %s", text)
		}
	}
}

func TestApplyDeckPatch_BoundaryErrors(t *testing.T) {
	mc := testMCPConfig(t)

	t.Run("missing presentation", func(t *testing.T) {
		result, err := mc.handleApplyDeckPatch(context.Background(), makeRequest(map[string]any{
			"ops": []any{map[string]any{"op": "remove_slide", "index": 0}},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for missing presentation")
		}
	})

	t.Run("missing ops", func(t *testing.T) {
		result, err := mc.handleApplyDeckPatch(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(threeSlideDeck()),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for missing ops")
		}
	})

	t.Run("empty ops", func(t *testing.T) {
		result, err := mc.handleApplyDeckPatch(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(threeSlideDeck()),
			"ops":          []any{},
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for empty ops")
		}
	})
}

// TestApplyDeckPatch_FindingsSurfaceValidationErrors confirms that a patch which
// is structurally valid but leaves the deck unusable (e.g. all slides removed)
// still returns the patched deck, with findings.ok=false flagging the problem.
func TestApplyDeckPatch_FindingsSurfaceValidationErrors(t *testing.T) {
	mc := testMCPConfig(t)
	twoSlide := `{
		"template": "midnight-blue",
		"slides": [
			{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Only A"}]},
			{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Only B"}]}
		]
	}`
	out := applyPatchOK(t, mc, twoSlide, []any{
		map[string]any{"op": "remove_slide", "index": 0},
		map[string]any{"op": "remove_slide", "index": 0},
	})
	if got := patchTitles(t, out.PatchedDeck); len(got) != 0 {
		t.Errorf("expected zero slides after removing both, got %v", got)
	}
	if out.Findings.OK {
		t.Error("expected findings.ok=false when the patch leaves zero slides")
	}
}
