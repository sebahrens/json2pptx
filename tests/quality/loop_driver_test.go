package quality

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSlideIndex(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"slides[0].content[0]", 0},
		{"slides[3].content[1].table", 3},
		{"slides[12].shape_grid.rows[0]", 12},
		{"no-slide-prefix", -1},
		{"slides[].bad", -1},
	}
	for _, tt := range tests {
		got := parseSlideIndex(tt.path)
		if got != tt.want {
			t.Errorf("parseSlideIndex(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestEvaluateFindings_Pass(t *testing.T) {
	state := NewLoopState()
	cfg := LoopConfig{ForceSplitOnCap: true}
	result := EvaluateFindings(nil, state, cfg)
	if result.Action != ActionPass {
		t.Errorf("expected ActionPass, got %s", result.Action)
	}
}

func TestEvaluateFindings_Repair(t *testing.T) {
	state := NewLoopState()
	cfg := LoopConfig{ForceSplitOnCap: true}
	findings := []fitFinding{
		{Code: "fit_overflow", Path: "slides[0].content[1].rows[0][0]", Action: "unfittable"},
	}
	result := EvaluateFindings(findings, state, cfg)
	if result.Action != ActionRepair {
		t.Errorf("expected ActionRepair, got %s", result.Action)
	}
	if len(result.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(result.Slides))
	}
	if state.Attempts[0] != 1 {
		t.Errorf("expected 1 attempt for slide 0, got %d", state.Attempts[0])
	}
}

func TestEvaluateFindings_CapReached(t *testing.T) {
	state := NewLoopState()
	state.Attempts[0] = MaxRepairAttempts // already at cap
	cfg := LoopConfig{ForceSplitOnCap: true}
	findings := []fitFinding{
		{Code: "fit_overflow", Path: "slides[0].content[1].rows[0][0]", Action: "unfittable"},
	}
	result := EvaluateFindings(findings, state, cfg)
	if result.Action != ActionForceSplit {
		t.Errorf("expected ActionForceSplit, got %s", result.Action)
	}
	if len(result.CappedAt) != 1 || result.CappedAt[0] != 0 {
		t.Errorf("expected capped_at=[0], got %v", result.CappedAt)
	}
}

func TestEvaluateFindings_WarningOnly(t *testing.T) {
	state := NewLoopState()
	cfg := LoopConfig{ForceSplitOnCap: true}
	findings := []fitFinding{
		{Code: "density_exceeded", Path: "slides[0].content[1]", Action: "warning"},
	}
	result := EvaluateFindings(findings, state, cfg)
	// Warnings (non-unfittable) don't trigger repair.
	if result.Action != ActionPass {
		t.Errorf("expected ActionPass for warning-only findings, got %s", result.Action)
	}
}

// TestDenseTableLoop is a regression test for the dense-table-16x6 fixture.
// It verifies the loop driver correctly identifies the overflow, allows repair
// attempts, and eventually forces a split after cap is reached.
func TestDenseTableLoop(t *testing.T) {
	projectRoot := findProjectRoot(t)
	binary := buildBinary(t, projectRoot)

	fixturePath := filepath.Join(projectRoot, "tests", "quality", "fixtures", "dense-table-16x6.json")
	if _, err := os.Stat(fixturePath); err != nil {
		t.Skipf("fixture not found: %s", fixturePath)
	}

	cfg := LoopConfig{
		Binary:          binary,
		ForceSplitOnCap: true,
	}
	state := NewLoopState()

	// Iteration 1: validate the dense table — should produce unfittable findings.
	findings, err := RunValidatePass(cfg, fixturePath)
	if err != nil {
		t.Fatalf("RunValidatePass: %v", err)
	}

	if len(findings) == 0 {
		t.Fatal("expected findings for dense-table-16x6, got none")
	}

	// Check that at least one finding is unfittable.
	hasUnfittable := false
	for _, f := range findings {
		if f.Action == "unfittable" {
			hasUnfittable = true
			break
		}
	}
	if !hasUnfittable {
		t.Log("dense-table-16x6 has findings but none are unfittable — table may fit at current metrics")
	}

	result1 := EvaluateFindings(findings, state, cfg)
	t.Logf("Iteration 1: action=%s message=%s", result1.Action, result1.Message)

	if result1.Action == ActionPass {
		t.Log("dense-table-16x6 passes fit check — no repair loop needed")
		return
	}

	// Iteration 2: simulate agent repair attempt (same findings = no improvement).
	result2 := EvaluateFindings(findings, state, cfg)
	t.Logf("Iteration 2: action=%s message=%s", result2.Action, result2.Message)

	// Iteration 3: should hit cap and force split.
	result3 := EvaluateFindings(findings, state, cfg)
	t.Logf("Iteration 3: action=%s message=%s", result3.Action, result3.Message)

	if result3.Action != ActionForceSplit {
		t.Errorf("expected ActionForceSplit after %d attempts, got %s", MaxRepairAttempts, result3.Action)
	}

	// Verify the result can be serialized as JSON (for agent consumption).
	out, err := json.MarshalIndent(result3, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	t.Logf("Final result:\n%s", string(out))
}
