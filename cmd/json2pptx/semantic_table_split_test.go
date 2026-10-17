package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDenseSemanticTableOffersVerifiedLosslessSplit(t *testing.T) {
	mc := refusalTestConfig(t)
	var rows []any
	for i := range 12 {
		rows = append(rows, []any{fmt.Sprintf("Team %d", i+1), fmt.Sprintf("%d", i+1)})
	}
	headers := []any{"Team", "Count"}
	table := map[string]any{"kind": "table", "title": "Twelve teams contribute", "headers": headers, "rows": rows, "totals_row": true, "takeaway": "The portfolio includes twelve teams", "source": "Operating plan"}
	spec := map[string]any{
		"meta":   map[string]any{"title": "Operating teams", "template": "midnight-blue", "source": "Operating plan"},
		"slides": []any{map[string]any{"kind": "title", "title": "Twelve teams contribute", "subtitle": "Operating plan"}, table},
	}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	f := findingAt(t, env, "SEMANTIC_DENSITY", "/slides/1/rows")
	if f.NextToolCall == nil || !f.PatchVerified {
		t.Fatalf("dense table did not offer an executable verified split: %+v", f)
	}
	patched := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, f.NextToolCall.ArgsTemplate))
	for _, finding := range patched.Findings {
		if strings.HasSuffix(finding.Code, "SEMANTIC_DENSITY") {
			t.Fatalf("verified split left density finding: %+v", finding)
		}
	}
	read := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": env.DeckID, "read": "spec"})
	stored := roundTripJSON(t, read.StructuredContent).(map[string]any)["spec"].(map[string]any)
	slides := stored["slides"].([]any)
	if len(slides) != 3 {
		t.Fatalf("split produced %d slides; want 3", len(slides))
	}
	first, second := slides[1].(map[string]any), slides[2].(map[string]any)
	joined := append(append([]any{}, first["rows"].([]any)...), second["rows"].([]any)...)
	if !reflect.DeepEqual(joined, rows) || !reflect.DeepEqual(first["headers"], headers) || !reflect.DeepEqual(second["headers"], headers) {
		t.Fatal("split lost or reordered rows or headers")
	}
	if first["totals_row"] == true || second["totals_row"] != true || first["source"] != "Operating plan" || second["source"] != "Operating plan" {
		t.Fatalf("split changed table semantics: first=%v second=%v", first, second)
	}
	if first["title"] != "Twelve teams contribute (1/2)" || second["title"] != "Twelve teams contribute (2/2)" || first["id"] == second["id"] {
		t.Fatal("split must label both parts and retain distinct stable identities")
	}
}
