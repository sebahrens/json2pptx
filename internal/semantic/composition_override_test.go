package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// comparisonSpec builds a two-column comparison slide with an optional
// composition override.
func comparisonSpec(override map[string]any) *DeckSpec {
	body := map[string]any{
		"title": "Us vs them",
		"columns": []any{
			map[string]any{"header": "Us", "items": []any{"Fast", "Cheap"}},
			map[string]any{"header": "Them", "items": []any{"Slow", "Dear"}},
		},
	}
	for k, v := range override {
		body[k] = v
	}
	return &DeckSpec{
		Meta:   DeckMeta{Title: "Deck", Template: "midnight-blue"},
		Slides: []SlideSpec{{Kind: KindComparison, Body: body}},
	}
}

func findingsWithCode(diags []diagnostics.Diagnostic, code diagnostics.Code) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, d := range diags {
		if d.Code == string(code) {
			out = append(out, d)
		}
	}
	return out
}

// The schema documents pattern / layout as "one of this kind's alternative
// patterns", but anything outside that (undisclosed) list was a no-op with no
// diagnostic: {kind: comparison, pattern: table-highlight} compiled to
// comparison-2col and validated clean, so an agent following
// analyze_deck_rhythm's "break this run" advice could not tell its variation
// had been dropped (go-slide-creator-u5az).
func TestUnavailableCompositionOverrideIsReported(t *testing.T) {
	diags := Validate(comparisonSpec(map[string]any{"pattern": "table-highlight"}), StrictnessWarn)
	found := findingsWithCode(diags, diagnostics.CodeSemanticPatternNotAvailable)
	if len(found) != 1 {
		t.Fatalf("expected 1 finding, got %d (%+v)", len(found), diags)
	}
	d := found[0]
	if d.Path != "slides[0].pattern" {
		t.Errorf("path = %q, want slides[0].pattern", d.Path)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("severity = %q, want warning under warn", d.Severity)
	}
	// The message and the fix must both carry what IS allowed: "not available"
	// with no list is the same dead end as saying nothing.
	if !strings.Contains(d.Message, "comparison-2col") {
		t.Errorf("message does not name the available patterns: %q", d.Message)
	}
	if d.Fix == nil || d.Fix.Kind != "use_one_of" {
		t.Fatalf("fix = %+v, want use_one_of", d.Fix)
	}
	allowed, _ := d.Fix.Params["allowed"].([]string)
	if len(allowed) == 0 {
		t.Errorf("fix.params.allowed is empty: %+v", d.Fix.Params)
	}
}

// A layout override is judged against the same list.
func TestUnavailableLayoutOverrideIsReported(t *testing.T) {
	diags := Validate(comparisonSpec(map[string]any{"layout": "two-column"}), StrictnessWarn)
	found := findingsWithCode(diags, diagnostics.CodeSemanticPatternNotAvailable)
	if len(found) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(found))
	}
	if found[0].Path != "slides[0].layout" {
		t.Errorf("path = %q, want slides[0].layout", found[0].Path)
	}
}

// An override the kind DOES accept compiles silently — and actually takes
// effect, which is the whole point of reporting the ones that do not.
func TestAvailableCompositionOverrideIsSilentAndApplied(t *testing.T) {
	spec := comparisonSpec(map[string]any{"pattern": "card-grid"})
	if d := findingsWithCode(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticPatternNotAvailable); len(d) != 0 {
		t.Errorf("a valid alternative drew %+v", d)
	}
	ir := Normalize(spec)
	if got := ir.Slides[0].Visual.Pattern; got != "card-grid" {
		t.Errorf("planned pattern = %q, want the requested card-grid", got)
	}
}

// Under strict the advisory becomes an error, like the other composition rules.
func TestUnavailableOverrideIsAnErrorUnderStrict(t *testing.T) {
	diags := Validate(comparisonSpec(map[string]any{"pattern": "table-highlight"}), StrictnessStrict)
	found := findingsWithCode(diags, diagnostics.CodeSemanticPatternNotAvailable)
	if len(found) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(found))
	}
	if found[0].Severity != diagnostics.SeverityError {
		t.Errorf("severity = %q, want error under strict", found[0].Severity)
	}
}

// A slide with no override draws nothing, and neither does a kind with no
// alternatives at all.
func TestNoOverrideDrawsNothing(t *testing.T) {
	if d := findingsWithCode(Validate(comparisonSpec(nil), StrictnessWarn), diagnostics.CodeSemanticPatternNotAvailable); len(d) != 0 {
		t.Errorf("a slide with no override drew %+v", d)
	}
	titleSpec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Q3 review"}}},
	}
	if d := findingsWithCode(Validate(titleSpec, StrictnessWarn), diagnostics.CodeSemanticPatternNotAvailable); len(d) != 0 {
		t.Errorf("a title slide with no override drew %+v", d)
	}
}
