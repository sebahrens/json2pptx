package patterns

import "testing"

// Suggest must be deterministic: "kpi-9up" is distance 1 from kpi-2up…kpi-6up,
// so a map-order tiebreak made two calls in one process disagree.
func TestSuggestIsDeterministicAcrossEqualDistanceCandidates(t *testing.T) {
	reg := Default()
	first, ok := reg.Suggest("kpi-9up")
	if !ok {
		t.Fatal("expected a suggestion for kpi-9up")
	}
	for i := 0; i < 200; i++ {
		got, ok := reg.Suggest("kpi-9up")
		if !ok || got != first {
			t.Fatalf("Suggest is nondeterministic: call %d returned %q (ok=%v), first call returned %q", i, got, ok, first)
		}
	}
}
