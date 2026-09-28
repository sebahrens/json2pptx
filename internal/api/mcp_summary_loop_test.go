package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-z3pbp: the text summary keeps what the render loop needs
// for its next step.
func TestCompactSummaryKeepsLoopFields(t *testing.T) {
	hash := strings.Repeat("a", 64)
	encoded, err := compactMCPTextSummary(map[string]any{
		"ok": true, "pptx_path": "/o/d.pptx", "content_hash": hash,
		"blocking_reasons": []string{"quality gate: failed", "missing visual verdict", "c", "d"},
		"changed_slides":   []int{1, 4},
		"applied_fixes": []map[string]any{
			{"kind": "reduce_text", "applied": true},
			{"kind": "reduce_txt", "applied": false, "code": "kind_not_supported", "did_you_mean": "reduce_text", "next_tool_call": map[string]any{"tool": "repair_slide", "args_template": map[string]any{}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got["content_hash"] != hash {
		t.Errorf("content_hash dropped: %s", encoded)
	}
	if reasons, _ := got["blocking_reasons"].([]any); len(reasons) != 3 || got["blocking_reasons_count"] != float64(4) {
		t.Errorf("blocking_reasons = %v", got["blocking_reasons"])
	}
	if changed, _ := got["changed_slides"].([]any); len(changed) != 2 {
		t.Errorf("changed_slides = %v", got["changed_slides"])
	}
	failed, _ := got["failed_fixes"].([]any)
	if len(failed) != 1 || failed[0].(map[string]any)["did_you_mean"] != "reduce_text" || failed[0].(map[string]any)["next_tool"] != "repair_slide" {
		t.Errorf("failed_fixes = %v", got["failed_fixes"])
	}
}
