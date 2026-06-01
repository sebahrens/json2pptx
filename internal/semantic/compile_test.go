package semantic

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// updateGolden regenerates the golden compiled-JSON fixtures when set.
var updateGolden = flag.Bool("update-golden", false, "rewrite golden compiled-JSON fixtures")

// qbrExamplePath is the QBR semantic example, relative to this package dir.
const qbrExamplePath = "../../examples/semantic/qbr.yaml"

// compileExample parses and compiles a semantic example file, failing the test
// on any parse, validation, or compile error.
func compileExample(t *testing.T, path string) (*deckinput.PresentationInput, *CompileResult) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	spec, pdiags := Parse(path, data)
	if pdiags.HasErrors() {
		t.Fatalf("parse %s: %v", path, pdiags)
	}
	input, result, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile %s: %v", path, err)
	}
	return input, result
}

func TestCompileQBR_Golden(t *testing.T) {
	input, _ := compileExample(t, qbrExamplePath)

	got, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		t.Fatalf("marshal compiled deck: %v", err)
	}
	got = append(got, '\n')

	golden := filepath.Join("testdata", "qbr_compiled.json")
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

func TestCompileQBR_Deterministic(t *testing.T) {
	first, _ := compileExample(t, qbrExamplePath)
	second, _ := compileExample(t, qbrExamplePath)

	a, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(a) != string(b) {
		t.Error("two compiles of the same spec produced different JSON")
	}
}

// TestCompileQBR_PatternsValidate resolves every emitted pattern against the
// pattern registry and runs its validator, mirroring the check json2pptx
// validate performs. A clean pass here is the unit-level proxy for "compiled
// JSON passes json2pptx validate" for the pattern-bearing slides.
func TestCompileQBR_PatternsValidate(t *testing.T) {
	input, _ := compileExample(t, qbrExamplePath)

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
		t.Fatal("expected at least one pattern-bearing slide in the QBR deck")
	}
}

// TestCompileQBR_SourceMapCoverage asserts the source map traces the generated
// user-authored fields back to the semantic source paths the author wrote.
func TestCompileQBR_SourceMapCoverage(t *testing.T) {
	_, result := compileExample(t, qbrExamplePath)
	sm := result.SourceMap
	if sm == nil || sm.Len() == 0 {
		t.Fatal("compile produced an empty source map")
	}

	// Spot-check representative raw paths across kinds: title text, summary
	// bullets, a KPI cell, the chart, the insights list, and the decision body.
	checks := []struct {
		raw  string
		want string
	}{
		{"slides[0].content[0].text_value", "slides[0].title"},
		{"slides[1].content[1].bullets_value", "slides[1].points"},
		{"slides[2].pattern.values[0]", "slides[2].kpis[0]"},
		{"slides[3].pattern.values.chart", "slides[3].chart"},
		{"slides[3].pattern.values.insights", "slides[3].insights"},
		{"slides[4].content[1].body_and_bullets_value.body", "slides[4].recommendation"},
	}
	for _, c := range checks {
		e, ok := sm.Lookup(c.raw)
		if !ok {
			t.Errorf("source map has no entry for raw path %q", c.raw)
			continue
		}
		if e.SemanticPath != c.want {
			t.Errorf("source map %q -> %q, want %q", c.raw, e.SemanticPath, c.want)
		}
	}

	// Every emitted text/bullet field's nested paths must resolve via the map's
	// ancestor walk, so a fit-report path under any mapped field traces back.
	if _, ok := sm.Lookup("slides[2].pattern.values[0].big"); !ok {
		t.Error("nested KPI cell path should resolve to its mapped ancestor")
	}
}

func TestCompile_BlockingErrorsDoNotEmit(t *testing.T) {
	// A deck missing its required title is a hard error; compile must refuse to
	// emit and surface the diagnostics on the result.
	spec := &DeckSpec{
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Hello"}},
		},
	}
	input, result, err := Compile(spec, CompileOptions{})
	if err == nil {
		t.Fatal("expected a blocking error for a deck with no title")
	}
	if input != nil {
		t.Error("no PresentationInput should be emitted when validation blocks")
	}
	if result == nil || len(result.Diagnostics) == 0 {
		t.Fatal("blocking compile should still return diagnostics on the result")
	}
}

// TestCompile_ZeroSlidesRefuses is the compile-path half of the
// go-slide-creator-444x guard: a deck with a valid title but no slides must not
// compile to a null/empty deck — Compile must refuse and surface the blocking
// slides finding instead.
func TestCompile_ZeroSlidesRefuses(t *testing.T) {
	spec := &DeckSpec{Meta: DeckMeta{Title: "Empty Deck"}}
	input, result, err := Compile(spec, CompileOptions{})
	if err == nil {
		t.Fatal("expected a blocking error for a deck with no slides")
	}
	if input != nil {
		t.Error("no PresentationInput should be emitted for a zero-slide deck")
	}
	if result == nil {
		t.Fatal("blocking compile should still return a result")
	}
	if _, ok := findAt(result.Diagnostics, diagnostics.CodeSemanticRequired, "slides"); !ok {
		t.Fatalf("expected SEMANTIC_REQUIRED at slides, got %v", result.Diagnostics)
	}
}

func TestCompile_AppliesDefaultsAndTemplate(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Untemplated Deck"},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Untemplated Deck"}},
		},
	}
	input, _, err := Compile(spec, CompileOptions{DefaultTemplate: "midnight-blue", OutputFilename: "out.pptx"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.Template != "midnight-blue" {
		t.Errorf("Template = %q, want midnight-blue (from DefaultTemplate)", input.Template)
	}
	if input.DesignMode != "constrained" {
		t.Errorf("DesignMode = %q, want constrained", input.DesignMode)
	}
	if input.OutputFilename != "out.pptx" {
		t.Errorf("OutputFilename = %q, want out.pptx", input.OutputFilename)
	}
}
