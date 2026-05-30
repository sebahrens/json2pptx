package semantic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// findAt reports whether ds contains a diagnostic with the given code at the
// given path, and returns it.
func findAt(ds []diagnostics.Diagnostic, code, path string) (diagnostics.Diagnostic, bool) {
	for _, d := range ds {
		if d.Code == code && d.Path == path {
			return d, true
		}
	}
	return diagnostics.Diagnostic{}, false
}

func hasCode(ds []diagnostics.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestValidateCleanFixtureHasNoErrors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "board_update.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ds := Check("board_update.yaml", data, StrictnessWarn)
	if diagnostics.HasErrors(ds) {
		t.Fatalf("clean fixture produced error diagnostics: %v", ds)
	}
}

func TestValidateMissingTitleIsRequired(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{Archetype: ArchetypeBoardUpdate},
		Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Hi"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticRequired, "meta.title")
	if !ok {
		t.Fatalf("expected SEMANTIC_REQUIRED at meta.title, got %v", ds)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("meta.title severity = %q, want error", d.Severity)
	}
}

func TestValidateUnknownKindAndArchetype(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck", Archetype: Archetype("nope")},
		Slides: []SlideSpec{{Kind: SlideKind("bogus"), Body: map[string]any{"title": "x"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticUnknownArchetype, "meta.archetype"); !ok {
		t.Errorf("expected SEMANTIC_UNKNOWN_ARCHETYPE at meta.archetype, got %v", ds)
	}
	if _, ok := findAt(ds, diagnostics.CodeSemanticUnknownKind, "slides[0].kind"); !ok {
		t.Errorf("expected SEMANTIC_UNKNOWN_KIND at slides[0].kind, got %v", ds)
	}
}

func TestValidateRequiredPayloadField(t *testing.T) {
	// kpi_snapshot requires "kpis".
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{"title": "Metrics"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[0].kpis"); !ok {
		t.Fatalf("expected SEMANTIC_REQUIRED at slides[0].kpis, got %v", ds)
	}
}

func TestValidateKPIDensity(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{
			"title":    "Metrics",
			"kpis":     []any{map[string]any{"label": "A", "value": "1"}},
			"takeaway": "One number that matters.",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticDensity, "slides[0].kpis")
	if !ok {
		t.Fatalf("expected SEMANTIC_DENSITY at slides[0].kpis, got %v", ds)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("density severity = %q, want warning under warn strictness", d.Severity)
	}
}

func TestValidateChartSeriesAdvisory(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
			"title":   "Trend",
			"chart":   map[string]any{"type": "bar", "series": []any{}},
			"insight": "It goes up.",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticDensity, "slides[0].chart.series")
	if !ok {
		t.Fatalf("expected SEMANTIC_DENSITY at slides[0].chart.series, got %v", ds)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("chart series severity = %q, want warning under warn strictness", d.Severity)
	}
	// And under off it must be suppressed (advisory).
	if hasCode(Validate(spec, StrictnessOff), diagnostics.CodeSemanticDensity) {
		t.Error("advisory chart-series finding must be suppressed under off")
	}
}

func TestValidateComparisonBalance(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindComparison, Body: map[string]any{
			"title":    "A vs B",
			"takeaway": "Pick A.",
			"columns": []any{
				map[string]any{"header": "A", "items": []any{"x", "y"}},
				map[string]any{"header": "B", "items": []any{"z"}},
			},
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticDensity, "slides[0].columns"); !ok {
		t.Fatalf("expected SEMANTIC_DENSITY (unbalanced) at slides[0].columns, got %v", ds)
	}
}

func TestValidateTakeawayRequiredAndStrictness(t *testing.T) {
	newSpec := func() *DeckSpec {
		return &DeckSpec{
			Meta: DeckMeta{Title: "Deck"},
			Slides: []SlideSpec{{Kind: KindProcess, Body: map[string]any{
				"title": "How it works",
				"steps": []any{"one", "two"},
			}}},
		}
	}

	warnDS := Validate(newSpec(), StrictnessWarn)
	d, ok := findAt(warnDS, diagnostics.CodeSemanticTakeawayRequired, "slides[0].takeaway")
	if !ok {
		t.Fatalf("expected SEMANTIC_TAKEAWAY_REQUIRED at slides[0].takeaway, got %v", warnDS)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("warn severity = %q, want warning", d.Severity)
	}

	strictDS := Validate(newSpec(), StrictnessStrict)
	d, ok = findAt(strictDS, diagnostics.CodeSemanticTakeawayRequired, "slides[0].takeaway")
	if !ok {
		t.Fatalf("expected takeaway finding under strict, got %v", strictDS)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("strict severity = %q, want error", d.Severity)
	}

	offDS := Validate(newSpec(), StrictnessOff)
	if hasCode(offDS, diagnostics.CodeSemanticTakeawayRequired) {
		t.Errorf("advisory takeaway finding must be suppressed under off, got %v", offDS)
	}
}

func TestValidateWeakContent(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindSection, Body: map[string]any{
			"title": "TBD",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticWeakContent, "slides[0].title"); !ok {
		t.Fatalf("expected SEMANTIC_WEAK_CONTENT at slides[0].title, got %v", ds)
	}
}

func TestValidateWrapsIntoEnvelope(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{}, // missing title -> error
		Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Hi"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	env := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "semantic validate"}, ds)
	if env.OK {
		t.Error("envelope OK should be false when a required field is missing")
	}
	if len(env.Findings) == 0 {
		t.Fatal("envelope should carry findings")
	}
	if env.Findings[0].Category != diagnostics.NamespaceInput {
		t.Errorf("finding category = %q, want INPUT", env.Findings[0].Category)
	}
}

func TestToDiagnosticsBridgesParseFindings(t *testing.T) {
	src := "slides:\n  - kind: bogus_kind\n    title: Oops\n"
	_, parseDiags := ParseYAML([]byte(src))
	ds := parseDiags.ToDiagnostics()
	if _, ok := findAt(ds, diagnostics.CodeSemanticUnknownKind, "slides[0].kind"); !ok {
		t.Fatalf("expected bridged SEMANTIC_UNKNOWN_KIND at slides[0].kind, got %v", ds)
	}
}
