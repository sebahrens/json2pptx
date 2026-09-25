package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestRawDeckHandleRevisionLoop(t *testing.T) {
	mc := repairMC(t)
	mc.deckHandles = newDeckHandleStore(deckHandleTTL)
	deck := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Operating update"},
		map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []string{"One", "Two", "Three"}},
	)
	generated := mustCall(t, mc.handleGenerate, map[string]any{"presentation": mustParseJSON(deck), "output_validation": "off"})
	if generated.IsError {
		t.Fatalf("generate_presentation failed: %s", textContent(generated))
	}
	var first JSONOutput
	structuredInto(t, generated.StructuredContent, &first)
	if first.DeckID == "" {
		t.Fatal("generated raw presentation has no deck_id")
	}

	fixes := []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 2}}}
	res := mustCall(t, mc.handleRepairSlide, map[string]any{"deck_id": first.DeckID, "slide_index": float64(0), "fixes": fixes})
	if res.IsError {
		t.Fatalf("repair_slide by deck_id failed: %s", textContent(res))
	}
	var repaired repairSlideOutput
	structuredInto(t, res.StructuredContent, &repaired)
	if repaired.DeckID != first.DeckID || repaired.ChangedSlides == nil || !equalInts(*repaired.ChangedSlides, []int{0}) {
		t.Fatalf("compact repair identity/change = %q/%v", repaired.DeckID, repaired.ChangedSlides)
	}
	if len(repaired.PatchedDeck) != 0 {
		t.Fatalf("handle repair echoed %d deck bytes without return_deck", len(repaired.PatchedDeck))
	}
	if !repaired.AppliedFixes[0].Applied {
		t.Fatalf("reduce_text was not applied: %+v", repaired.AppliedFixes)
	}
	noChange := mustCall(t, mc.handleRepairSlide, map[string]any{"deck_id": first.DeckID, "slide_index": float64(0),
		"fixes": []any{map[string]any{"kind": "reposition_shape"}}})
	var unchanged repairSlideOutput
	structuredInto(t, noChange.StructuredContent, &unchanged)
	if unchanged.ChangedSlides == nil || len(*unchanged.ChangedSlides) != 0 || len(unchanged.PatchedDeck) != 0 {
		t.Fatalf("no-op handle repair must report an empty change set without deck echo: %+v", unchanged)
	}

	stored, ok := mc.deckHandles.Load(first.DeckID)
	if !ok || stored.RawPresentation == nil {
		t.Fatal("stored raw revision disappeared")
	}
	if !strings.Contains(string(stored.RawPresentation), `"One"`) || strings.Contains(string(stored.RawPresentation), `"Three"`) {
		t.Fatalf("stored raw revision not updated: %s", stored.RawPresentation)
	}
	for _, tc := range []struct {
		name string
		call func() *mcp.CallToolResult
	}{
		{"preview_presentation_plan", func() *mcp.CallToolResult {
			return mustCall(t, mc.handlePreviewPlan, map[string]any{"deck_id": first.DeckID, "fit_report": false})
		}},
		{"analyze_deck_rhythm", func() *mcp.CallToolResult {
			return mustCall(t, mc.handleAnalyzeDeckRhythm, map[string]any{"deck_id": first.DeckID})
		}},
		{"score_deck", func() *mcp.CallToolResult {
			return mustCall(t, mc.handleScoreDeck, map[string]any{"deck_id": first.DeckID, "slide_indices": []any{0}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if result := tc.call(); result.IsError {
				t.Fatalf("%s rejected raw deck_id: %s", tc.name, textContent(result))
			}
		})
	}

	second := mustCall(t, mc.handleRepairSlide, map[string]any{"deck_id": first.DeckID, "slide_index": float64(0),
		"fixes": []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 1}}}, "return_deck": true})
	var expanded repairSlideOutput
	structuredInto(t, second.StructuredContent, &expanded)
	if len(expanded.PatchedDeck) == 0 || expanded.DeckID != first.DeckID {
		t.Fatalf("return_deck did not include full revision: %+v", expanded)
	}
	var patched PresentationInput
	if err := json.Unmarshal(expanded.PatchedDeck, &patched); err != nil || len(*patched.Slides[0].Content[1].BulletsValue) != 1 {
		t.Fatalf("patched deck does not contain the one-bullet revision: %v", err)
	}

	regenerated := mustCall(t, mc.handleGenerate, map[string]any{"deck_id": first.DeckID, "output_validation": "off"})
	var last JSONOutput
	structuredInto(t, regenerated.StructuredContent, &last)
	if last.DeckID != first.DeckID || !last.Success {
		t.Fatalf("regeneration by raw deck_id lost the handle or failed: %+v", last)
	}
}

func TestRawDeckHandleRejectsStaleConcurrentRevision(t *testing.T) {
	store := newDeckHandleStore(deckHandleTTL)
	initial := []byte(`{"slides":[{"layout_id":"title"}]}`)
	id := store.Save(&deckHandle{RawPresentation: initial, SlideDigests: slideDigests(initial)})
	first := []byte(`{"slides":[{"layout_id":"content"}]}`)
	second := []byte(`{"slides":[{"layout_id":"closing"}]}`)
	if !store.UpdateRaw(id, initial, first) {
		t.Fatal("first writer was rejected")
	}
	if store.UpdateRaw(id, initial, second) {
		t.Fatal("stale second writer overwrote the first revision")
	}
	if store.UpdateRaw(store.Save(newDeckHandleFor([]byte(handleTestSpec), "deck.json", "midnight-blue")), initial, second) {
		t.Fatal("raw update overwrote a DeckSpec handle")
	}
	stored, ok := store.Load(id)
	if !ok || string(stored.RawPresentation) != string(first) {
		t.Fatalf("raw handle lost the winning revision: %+v", stored)
	}
}

func TestRawDeckHandleExpiryIsAnError(t *testing.T) {
	mc := handleTestConfig(t)
	now := time.Now()
	mc.deckHandles.now = func() time.Time { return now }
	raw := []byte(minimalDeck())
	id := mc.deckHandles.Save(&deckHandle{RawPresentation: raw, SlideDigests: slideDigests(raw)})
	now = now.Add(deckHandleTTL + time.Second)
	_, _, result := mc.presentationForTool("score_deck", makeRequest(map[string]any{"deck_id": id}))
	if result == nil || !result.IsError || !strings.Contains(textContent(result), "expired") {
		t.Fatalf("expired raw handle was silently accepted: %+v", result)
	}
}

func TestGenerateFromSemanticHandleCreatesSeparateRawHandle(t *testing.T) {
	mc := handleTestConfig(t)
	spec := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": handleTestSpec}))
	res := mustCall(t, mc.handleGenerate, map[string]any{"deck_id": spec.DeckID, "output_validation": "off"})
	if res.IsError {
		t.Fatalf("generate from semantic handle failed: %s", textContent(res))
	}
	var out JSONOutput
	structuredInto(t, res.StructuredContent, &out)
	if out.DeckID == "" || out.DeckID == spec.DeckID {
		t.Fatalf("semantic handle was reused as mutable raw handle: semantic=%q raw=%q", spec.DeckID, out.DeckID)
	}
	semantic, ok := mc.deckHandles.Load(spec.DeckID)
	if !ok || semantic.RawPresentation != nil || len(semantic.Spec) == 0 {
		t.Fatalf("generation mutated the semantic source: %+v", semantic)
	}
}

func TestRawDeckHandleCannotBeParsedAsDeckSpec(t *testing.T) {
	mc := handleTestConfig(t)
	raw := []byte(minimalDeck())
	id := mc.deckHandles.Save(&deckHandle{RawPresentation: raw, SlideDigests: slideDigests(raw)})
	result := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id})
	if !result.IsError || !strings.Contains(textContent(result), "raw presentation") {
		t.Fatalf("semantic tool accepted a raw deck handle: %s", textContent(result))
	}
}

func TestRawDeckToolsPublishPresentationOrHandleChoice(t *testing.T) {
	for _, tool := range []mcp.Tool{mcpGenerateTool(), mcpPreviewPlanTool(), mcpRepairSlideTool(), mcpScoreDeckTool(), mcpAnalyzeDeckRhythmTool()} {
		t.Run(tool.Name, func(t *testing.T) {
			schema := publishedInputSchema(t, tool)
			_ = schemaProperty(t, schema, "deck_id")
			oneOf, _ := schema["oneOf"].([]any)
			if len(oneOf) != 2 {
				t.Fatalf("presentation/deck_id choice = %v", schema["oneOf"])
			}
			for i, name := range []string{"presentation", "deck_id"} {
				branch, _ := oneOf[i].(map[string]any)
				if !reflect.DeepEqual(branch["required"], []any{name}) {
					t.Errorf("oneOf[%d].required = %v, want %s", i, branch["required"], name)
				}
			}
		})
	}
}

func TestGenerateIdempotencyRejectsRevisedRawHandle(t *testing.T) {
	mc := repairMC(t)
	mc.deckHandles = newDeckHandleStore(deckHandleTTL)
	mc.idempotency = newIdempotencyCache(idempotencyCacheTTL)
	deck := minimalDeck(
		map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Operating update"},
		map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []string{"One", "Two", "Three"}},
	)
	args := map[string]any{"presentation": mustParseJSON(deck), "output_validation": "off", "idempotency_key": "raw-revision"}
	var first JSONOutput
	structuredInto(t, mustCall(t, mc.handleGenerate, args).StructuredContent, &first)
	if first.DeckID == "" {
		t.Fatal("generate did not return a raw handle")
	}
	var same JSONOutput
	structuredInto(t, mustCall(t, mc.handleGenerate, args).StructuredContent, &same)
	if !same.IdempotentReplay || same.DeckID != first.DeckID {
		t.Fatalf("unchanged raw handle did not replay the generation: %+v", same)
	}
	repair := mustCall(t, mc.handleRepairSlide, map[string]any{"deck_id": first.DeckID, "slide_index": float64(0),
		"fixes": []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": 2}}}})
	if repair.IsError {
		t.Fatalf("repair failed: %s", textContent(repair))
	}
	replay := mustCall(t, mc.handleGenerate, args)
	if !replay.IsError || !strings.Contains(textContent(replay), "earlier raw-deck revision") {
		t.Fatalf("idempotent replay paired the old PPTX with a revised handle: %s", textContent(replay))
	}
}
