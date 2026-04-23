package patterns

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestSizeMetricFunctions(t *testing.T) {
	p := &kpi3up{}
	vals := Kpi3upValues{
		{Big: "$4.2M", Small: "ARR"},
		{Big: "127%", Small: "NRR"},
		{Big: "12d", Small: "Sales cycle"},
	}

	t.Run("canonical_size_positive", func(t *testing.T) {
		n, err := CanonicalSizeBytes(p, &vals)
		if err != nil {
			t.Fatalf("CanonicalSizeBytes: %v", err)
		}
		if n <= 0 {
			t.Errorf("expected positive byte count, got %d", n)
		}
	})

	t.Run("pattern_input_size_positive", func(t *testing.T) {
		n, err := PatternInputSizeBytes(p, &vals)
		if err != nil {
			t.Fatalf("PatternInputSizeBytes: %v", err)
		}
		if n <= 0 {
			t.Errorf("expected positive byte count, got %d", n)
		}
	})

	t.Run("compact_smaller_than_expanded", func(t *testing.T) {
		expanded, _ := CanonicalSizeBytes(p, &vals)
		compact, _ := PatternInputSizeBytes(p, &vals)
		if compact >= expanded {
			t.Errorf("compact (%d) should be smaller than expanded (%d)", compact, expanded)
		}
	})
}

func TestSizeMetricGoldens(t *testing.T) {
	reg := Default()
	for _, p := range reg.List() {
		p := p
		t.Run(p.Name(), func(t *testing.T) {
			ex, ok := p.(Exemplar)
			if !ok {
				t.Skipf("pattern %q does not implement Exemplar", p.Name())
			}
			vals := ex.ExemplarValues()

			expanded, err := CanonicalSizeBytes(p, vals)
			if err != nil {
				t.Fatalf("CanonicalSizeBytes: %v", err)
			}
			compact, err := PatternInputSizeBytes(p, vals)
			if err != nil {
				t.Fatalf("PatternInputSizeBytes: %v", err)
			}

			got := SizeMetric{
				Pattern:       p.Name(),
				ExpandedBytes: expanded,
				CompactBytes:  compact,
			}

			goldenPath := filepath.Join("testdata", p.Name(), "sizemetric.golden.json")

			if os.Getenv("UPDATE_GOLDEN") == "1" {
				data, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(goldenPath, append(data, '\n'), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Log("golden file updated")
				return
			}

			raw, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
			}
			var want SizeMetric
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("unmarshal golden: %v", err)
			}

			checkDrift(t, "expanded_bytes", want.ExpandedBytes, got.ExpandedBytes)
			checkDrift(t, "compact_bytes", want.CompactBytes, got.CompactBytes)
		})
	}
}

// checkDrift fails the test if actual drifts more than 10% from expected.
func checkDrift(t *testing.T, field string, expected, actual int) {
	t.Helper()
	if expected == 0 {
		if actual != 0 {
			t.Errorf("%s: expected 0, got %d", field, actual)
		}
		return
	}
	drift := math.Abs(float64(actual-expected)) / float64(expected)
	if drift > 0.10 {
		t.Errorf("%s: drifted %.1f%% (expected %d, got %d); update golden with UPDATE_GOLDEN=1 if intentional",
			field, drift*100, expected, actual)
	}
}
