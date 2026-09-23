package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSemanticPatchCallOnlyForExistingStringFields(t *testing.T) {
	data := []byte(handleTestSpec)
	for _, tc := range []struct {
		path, pointer string
	}{
		{"slides[0].title", "/slides/0/title"},
		{"meta.template", "/meta/template"},
	} {
		call := semanticPatchCall(data, "deck_123", tc.path, "FIT.title_wrap")
		if call == nil || call.Tool != "validate_deck_spec" {
			t.Fatalf("%s: missing semantic patch call: %+v", tc.path, call)
		}
		ops := call.ArgsTemplate["patch"].([]any)
		if got := ops[0].(map[string]any)["path"]; got != tc.pointer {
			t.Errorf("%s: patch path = %v, want %s", tc.path, got, tc.pointer)
		}
	}
	for _, path := range []string{"slides[2].kpis", "slides[99].title", "slides[0].missing", "slides[0].title/other"} {
		if call := semanticPatchCall(data, "deck_123", path, "FIT.title_wrap"); call != nil {
			t.Errorf("%s: unsafe patch suggestion: %+v", path, call)
		}
	}
}

func TestDeckSpecFindingPatchRoundTrip(t *testing.T) {
	mc := handleTestConfig(t)
	bad := strings.Replace(handleTestSpec, `"midnight-blue"`, `"missing-template"`, 1)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": bad}))
	if first.OK || first.DeckID == "" {
		t.Fatalf("expected finding and handle: %+v", first)
	}
	var args map[string]any
	for _, finding := range first.Findings {
		if finding.Evidence["path"] != "meta.template" {
			continue
		}
		if finding.NextToolCall == nil || finding.NextToolCall.Tool != "validate_deck_spec" {
			t.Fatalf("finding routed away from semantic patch: %+v", finding)
		}
		args = finding.NextToolCall.ArgsTemplate
		break
	}
	if args == nil {
		t.Fatalf("no template finding: %+v", first.Findings)
	}
	args["patch"].([]any)[0].(map[string]any)["value"] = "midnight-blue"
	second := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, args))
	if !second.OK || second.DeckID != first.DeckID {
		t.Fatalf("semantic patch did not resolve finding: %+v", second)
	}
}

func TestRenderDeckSpecFailureSuggestsSemanticPatch(t *testing.T) {
	mc := handleTestConfig(t)
	bad := strings.Replace(handleTestSpec, `"midnight-blue"`, `"missing-template"`, 1)
	res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(map[string]any{"spec": bad}))
	if err != nil {
		t.Fatal(err)
	}
	var out renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &out)
	if out.OK || out.DeckID == "" || len(out.Diagnostics) == 0 {
		t.Fatalf("expected render diagnostics and handle: %+v", out)
	}
	for _, d := range out.Diagnostics {
		if d.NextToolCall == nil || d.NextToolCall.Tool == "repair_slide" {
			t.Errorf("DeckSpec render diagnostic routed to raw repair: %+v", d)
		}
		if d.SemanticPath == "meta.template" && d.NextToolCall.Tool != "validate_deck_spec" {
			t.Errorf("template diagnostic did not offer semantic patch: %+v", d)
		}
	}
}

func TestDeckIDAnalysisAndRawRepairBoundary(t *testing.T) {
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": handleTestSpec}))
	id := first.DeckID
	ctx := context.Background()
	score, err := mc.handleScoreDeck(ctx, makeRequest(map[string]any{"deck_id": id, "slide_indices": []any{0}}))
	if err != nil || score.IsError {
		t.Fatalf("score_deck(deck_id): err=%v result=%+v", err, score)
	}
	rhythm, err := mc.handleAnalyzeDeckRhythm(ctx, makeRequest(map[string]any{"deck_id": id}))
	if err != nil || rhythm.IsError {
		t.Fatalf("analyze_deck_rhythm(deck_id): err=%v result=%+v", err, rhythm)
	}
	storedBefore, _ := mc.deckHandles.Load(id)
	repair, err := mc.handleRepairSlide(ctx, makeRequest(map[string]any{
		"deck_id": id, "slide_index": float64(0),
		"fixes": []any{map[string]any{"kind": "shorten_title", "params": map[string]any{"max_length": 8}}},
	}))
	if err != nil || repair.IsError {
		t.Fatalf("repair_slide(deck_id): err=%v result=%+v", err, repair)
	}
	var out repairSlideOutput
	structuredInto(t, repair.StructuredContent, &out)
	if out.SourceDeckID != id || !out.SemanticSourceUnchanged {
		t.Errorf("raw escape boundary not explicit: %+v", out)
	}
	storedAfter, _ := mc.deckHandles.Load(id)
	if !json.Valid(out.PatchedDeck) || string(storedAfter.Spec) != string(storedBefore.Spec) {
		t.Fatal("raw repair mutated semantic source or returned invalid raw deck")
	}
	for _, tool := range []string{"score_deck", "analyze_deck_rhythm", "repair_slide"} {
		_, _, result := mc.presentationForTool(tool, makeRequest(map[string]any{"deck_id": "missing"}))
		if result == nil || !result.IsError {
			t.Errorf("%s accepted unknown deck_id", tool)
		}
		_, _, result = mc.presentationForTool(tool, makeRequest(map[string]any{"deck_id": id, "presentation": map[string]any{"slides": []any{}}}))
		if result == nil || !result.IsError {
			t.Errorf("%s accepted ambiguous input", tool)
		}
		_, _, result = mc.presentationForTool(tool, makeRequest(map[string]any{"deck_id": id, "patch": specPatchExample()}))
		if result == nil || !result.IsError {
			t.Errorf("%s silently ignored a semantic patch", tool)
		}
	}
}
