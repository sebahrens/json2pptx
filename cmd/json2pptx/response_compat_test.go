package main

import (
	"reflect"
	"testing"
)

func TestRawSemanticResponseConventions(t *testing.T) {
	mc := settingsTestConfig(t)
	mc.deckHandles = newDeckHandleStore(deckHandleTTL)
	s := newMCPServer(mc)
	deck := minimalScoreDeck(2)
	deck["template"] = "missing-template-overridden-for-this-call"
	generated := assertLiveOutputSchema(t, s, "generate_presentation", map[string]any{
		"presentation": deck, "template": "midnight-blue", "output_filename": "compat.pptx",
	})
	if generated.IsError {
		t.Fatalf("registered template override failed: %s", resultText(generated))
	}
	got := roundTripJSON(t, generated.StructuredContent).(map[string]any)
	for _, pair := range [][2]string{{"ok", "success"}, {"pptx_path", "output_path"}, {"quality_summary", "quality"}} {
		if got[pair[0]] == nil || !reflect.DeepEqual(got[pair[0]], got[pair[1]]) {
			t.Errorf("canonical %s=%v differs from compatibility %s=%v", pair[0], got[pair[0]], pair[1], got[pair[1]])
		}
	}
	if _, ok := got["diagnostics"].([]any); !ok {
		t.Fatalf("diagnostics must always be an array: %v", got["diagnostics"])
	}
	validated := assertLiveOutputSchema(t, s, "validate_input", map[string]any{"presentation": deck, "template": "midnight-blue"})
	if validated.IsError {
		t.Fatalf("validate override failed: %s", resultText(validated))
	}
	got = roundTripJSON(t, validated.StructuredContent).(map[string]any)
	if got["ok"] != true || got["schema_version"] == "" {
		t.Fatalf("missing flat validation envelope: %v", got)
	}
	findings, ok := got["findings"].([]any)
	if !ok {
		t.Fatalf("top-level findings must be an array: %v", got["findings"])
	}
	legacy := got["legacy_findings"].(map[string]any)
	if !reflect.DeepEqual(findings, legacy["findings"]) {
		t.Fatal("flat and migration envelopes must carry the same findings")
	}
	if deck["template"] != "missing-template-overridden-for-this-call" {
		t.Fatal("template override mutated caller input")
	}
}
