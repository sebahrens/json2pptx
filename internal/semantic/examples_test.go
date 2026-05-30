package semantic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Bundled semantic authoring examples, relative to this package dir. These are
// the same files shipped under examples/semantic/ for agents and humans to copy.
const (
	salesPitchExamplePath = "../../examples/semantic/sales_pitch.yaml"
	invalidExamplePath    = "../../examples/semantic/invalid.yaml"
)

// TestCompileSalesPitch_Golden pins the compiled raw JSON of the bundled
// sales_pitch example. It exercises a second archetype (sales_pitch ->
// warm-coral, non-executive) and a section divider, so the golden guards a
// different compile path than the QBR fixture. Run with -update-golden to
// regenerate after an intentional compiler change.
func TestCompileSalesPitch_Golden(t *testing.T) {
	input, _ := compileExample(t, salesPitchExamplePath)

	got, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		t.Fatalf("marshal compiled deck: %v", err)
	}
	got = append(got, '\n')

	golden := filepath.Join("testdata", "sales_pitch_compiled.json")
	if *updateGolden {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden %s", golden)
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden %s (run with -update-golden to create): %v", golden, err)
	}
	if string(got) != string(want) {
		t.Errorf("compiled deck does not match golden %s\n--- got ---\n%s", golden, got)
	}
}

func TestCompileSalesPitch_Deterministic(t *testing.T) {
	first, _ := compileExample(t, salesPitchExamplePath)
	second, _ := compileExample(t, salesPitchExamplePath)

	a, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(a) != string(b) {
		t.Error("two compiles of the sales_pitch spec produced different JSON")
	}
}

// TestCompileSalesPitch_Resolution checks the archetype-driven template and the
// emitted slide count, the two contract surfaces a sales_pitch author relies on.
func TestCompileSalesPitch_Resolution(t *testing.T) {
	input, result := compileExample(t, salesPitchExamplePath)
	if input.Template != "warm-coral" {
		t.Errorf("template = %q, want warm-coral (sales_pitch archetype default)", input.Template)
	}
	if result.IR.Executive {
		t.Error("sales_pitch is not an executive archetype; IR.Executive should be false")
	}
	if len(input.Slides) != 7 {
		t.Errorf("compiled sales_pitch has %d slides, want 7", len(input.Slides))
	}
}

// TestCompileSalesPitch_PatternsValidate resolves every emitted pattern and runs
// its validator, the unit-level proxy for "compiled JSON passes json2pptx
// validate" on the pattern-bearing slides.
func TestCompileSalesPitch_PatternsValidate(t *testing.T) {
	input, _ := compileExample(t, salesPitchExamplePath)

	patternSlides := 0
	for i := range input.Slides {
		p := input.Slides[i].Pattern
		if p == nil {
			continue
		}
		patternSlides++
		pat, ok := patterns.Default().Get(p.Name)
		if !ok {
			t.Errorf("slide %d: emitted unknown pattern %q", i, p.Name)
			continue
		}
		values := pat.NewValues()
		if err := json.Unmarshal(p.Values, values); err != nil {
			t.Errorf("slide %d: pattern %q values do not decode: %v", i, p.Name, err)
			continue
		}
		var overrides any
		if len(p.Overrides) > 0 {
			overrides = pat.NewOverrides()
			if err := json.Unmarshal(p.Overrides, overrides); err != nil {
				t.Errorf("slide %d: pattern %q overrides do not decode: %v", i, p.Name, err)
				continue
			}
		}
		if err := pat.Validate(values, overrides, nil); err != nil {
			t.Errorf("slide %d: pattern %q failed validation: %v", i, p.Name, err)
		}
	}
	if patternSlides == 0 {
		t.Fatal("expected at least one pattern-bearing slide in the sales_pitch deck")
	}
}

// TestSalesPitchExample_NoBlockingFindings asserts the bundled sales_pitch
// example validates with no error-severity findings at the default strictness,
// so it stays a clean, copyable template.
func TestSalesPitchExample_NoBlockingFindings(t *testing.T) {
	data, err := os.ReadFile(salesPitchExamplePath)
	if err != nil {
		t.Fatal(err)
	}
	ds := Check("sales_pitch.yaml", data, StrictnessWarn)
	if diagnostics.HasErrors(ds) {
		t.Fatalf("sales_pitch example produced error diagnostics: %v", ds)
	}
}

// TestInvalidExample_MultipleDiagnosticsStablePaths is the acceptance test for
// the negative fixture: the bundled invalid example must surface several
// distinct semantic diagnostics, each anchored to a stable, predictable source
// path. The (code, path) pairs are pinned so the fixture keeps teaching the same
// lessons as the validator evolves.
func TestInvalidExample_MultipleDiagnosticsStablePaths(t *testing.T) {
	data, err := os.ReadFile(invalidExamplePath)
	if err != nil {
		t.Fatal(err)
	}
	ds := Check("invalid.yaml", data, StrictnessWarn)

	// The fixture must block (have at least one error-severity finding).
	if !diagnostics.HasErrors(ds) {
		t.Fatalf("invalid example must produce error findings, got %v", ds)
	}

	want := []struct {
		code     string
		path     string
		severity diagnostics.Severity
	}{
		{diagnostics.CodeSemanticRequired, "meta.title", diagnostics.SeverityError},
		{diagnostics.CodeSemanticUnknownArchetype, "meta.archetype", diagnostics.SeverityError},
		{diagnostics.CodeSemanticDensity, "slides[0].points", diagnostics.SeverityWarning},
		{diagnostics.CodeSemanticTakeawayRequired, "slides[0].takeaway", diagnostics.SeverityWarning},
		{diagnostics.CodeSemanticRequired, "slides[1].kpis", diagnostics.SeverityError},
		{diagnostics.CodeSemanticUnknownKind, "slides[2].kind", diagnostics.SeverityError},
		{diagnostics.CodeSemanticWeakContent, "slides[3].title", diagnostics.SeverityWarning},
		{diagnostics.CodeSemanticDensity, "slides[3].chart.series", diagnostics.SeverityWarning},
	}

	codes := map[string]bool{}
	paths := map[string]bool{}
	for _, w := range want {
		d, ok := findAt(ds, w.code, w.path)
		if !ok {
			t.Errorf("expected %s at %s, not found in %v", w.code, w.path, ds)
			continue
		}
		if d.Severity != w.severity {
			t.Errorf("%s at %s severity = %q, want %q", w.code, w.path, d.Severity, w.severity)
		}
		codes[w.code] = true
		paths[w.path] = true
	}

	// "Multiple semantic diagnostics" — guard the breadth, not just the presence
	// of any one finding.
	if len(codes) < 4 {
		t.Errorf("invalid fixture surfaced %d distinct diagnostic codes, want >= 4", len(codes))
	}
	if len(paths) < 6 {
		t.Errorf("invalid fixture surfaced %d distinct source paths, want >= 6", len(paths))
	}
}

// TestInvalidExample_Deterministic guards that validating the negative fixture
// twice yields byte-identical diagnostics, so the taught (code, path) set is
// stable across runs.
func TestInvalidExample_Deterministic(t *testing.T) {
	data, err := os.ReadFile(invalidExamplePath)
	if err != nil {
		t.Fatal(err)
	}
	first := Check("invalid.yaml", data, StrictnessWarn)
	second := Check("invalid.yaml", data, StrictnessWarn)

	a, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first diagnostics: %v", err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second diagnostics: %v", err)
	}
	if string(a) != string(b) {
		t.Errorf("invalid fixture diagnostics are non-deterministic:\n--- first ---\n%s\n--- second ---\n%s", a, b)
	}
}
