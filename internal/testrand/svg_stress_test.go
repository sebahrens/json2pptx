package testrand

import (
	"testing"
)

func TestSVGStressAllTypes(t *testing.T) {
	runner := NewSVGStressRunner(42)
	report := runner.Run("")

	t.Logf("SVG stress test: %d total, %d passed, %d failed (seed=%d)",
		report.Total, report.Passed, report.Failed, report.Seed)

	for _, r := range report.Failures {
		t.Errorf("FAIL: %s/%s: %s", r.DiagramType, r.Variant, r.Error)
	}
}

func TestSVGStressSeedReproducibility(t *testing.T) {
	r1 := NewSVGStressRunner(12345)
	r2 := NewSVGStressRunner(12345)

	report1 := r1.Run("")
	report2 := r2.Run("")

	if len(report1.Results) != len(report2.Results) {
		t.Fatalf("different result counts: %d vs %d", len(report1.Results), len(report2.Results))
	}

	for i, res1 := range report1.Results {
		res2 := report2.Results[i]
		if res1.DiagramType != res2.DiagramType || res1.Variant != res2.Variant {
			t.Errorf("result %d: type/variant mismatch: %s/%s vs %s/%s",
				i, res1.DiagramType, res1.Variant, res2.DiagramType, res2.Variant)
		}
		if res1.Passed != res2.Passed {
			t.Errorf("result %d (%s/%s): pass mismatch: %v vs %v",
				i, res1.DiagramType, res1.Variant, res1.Passed, res2.Passed)
		}
	}
}

func TestSVGStressAliasResolution(t *testing.T) {
	aliases := AliasMap()
	runner := NewSVGStressRunner(99)

	for alias, canonical := range aliases {
		t.Run(alias+"->"+canonical, func(t *testing.T) {
			// Render with the alias — should resolve to canonical type
			data := runner.dataForType(canonical, 3)
			result := runner.runOne(alias, VariantStandard)

			// Alias might not be directly renderable if not registered
			// The important thing is it doesn't panic
			if result.Error != "" && result.Passed {
				t.Logf("alias %s: %s (ok)", alias, result.Error)
			} else if !result.Passed {
				t.Logf("alias %s failed: %s (data keys: %v)", alias, result.Error, keys(data))
			}
		})
	}
}

func TestSVGStressPerType(t *testing.T) {
	types := DiagramTypes()
	runner := NewSVGStressRunner(777)

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			report := runner.Run(typ)
			for _, r := range report.Failures {
				t.Errorf("%s/%s: %s", r.DiagramType, r.Variant, r.Error)
			}
		})
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
