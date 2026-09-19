package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// kpiSpec builds a one-slide kpi_snapshot deck around the given metric values.
func kpiSpec(values ...string) *DeckSpec {
	kpis := make([]any, len(values))
	for i, v := range values {
		kpis[i] = map[string]any{"value": v, "label": "Metric"}
	}
	return &DeckSpec{
		Meta: DeckMeta{Title: "Deck", Template: "forest-green"},
		Slides: []SlideSpec{{
			Kind: KindKPISnapshot,
			Body: map[string]any{"title": "Results", "takeaway": "Growth held", "kpis": kpis},
		}},
	}
}

// A euro sign is one character and three bytes. Counting bytes made "€186.4M"
// measure 9 against the 8-character big-number budget, so the compiler dropped
// the whole KPI visual and emitted a bullet list — with no finding to say so
// (go-slide-creator-5ok4).
func TestKPISnapshotKeepsCardsForMultiByteValues(t *testing.T) {
	spec := kpiSpec("€186.4M", "117%", "6.2%")

	input, result, err := Compile(spec, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	slide := input.Slides[0]
	if slide.Pattern == nil || slide.Pattern.Name != "kpi-3up" {
		t.Fatalf("expected a kpi-3up pattern, got %+v", slide.Pattern)
	}
	for _, d := range result.Diagnostics {
		if d.Code == string(diagnostics.CodeSemanticDensity) {
			t.Errorf("content inside the budget drew a degrade advisory: %s", d.Message)
		}
	}
	// The plan must name what compile emitted.
	if got := result.IR.Slides[0].Visual.Pattern; got != "kpi-3up" {
		t.Errorf("plan pattern = %q, want kpi-3up", got)
	}
}

// A value genuinely past the budget still degrades — but no longer silently:
// validate, compile and render all carry the advisory, and the plan stops
// advertising a KPI visual the compiler will not emit.
func TestKPISnapshotReportsBudgetDegrade(t *testing.T) {
	spec := kpiSpec("EUR 1,186.42 million", "117%", "6.2%")

	input, result, err := Compile(spec, CompileOptions{Strict: StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if input.Slides[0].Pattern != nil {
		t.Fatalf("expected the bullet fallback, got pattern %+v", input.Slides[0].Pattern)
	}

	var found *diagnostics.Diagnostic
	for i, d := range result.Diagnostics {
		if strings.Contains(d.Message, "degrades to a bullet list") {
			found = &result.Diagnostics[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no degrade finding; the slide lost its KPI cards silently (%+v)", result.Diagnostics)
	}
	if found.Code != string(diagnostics.CodeSemanticDensity) {
		t.Errorf("code = %q, want %q", found.Code, diagnostics.CodeSemanticDensity)
	}
	// The author needs the field and the budget, not just "it degraded".
	for _, want := range []string{"kpi-3up", "values[0].big", "maxLength 8"} {
		if !strings.Contains(found.Message, want) {
			t.Errorf("message %q does not name %q", found.Message, want)
		}
	}
	if !strings.HasPrefix(found.Path, "slides[0]") {
		t.Errorf("path = %q, want the offending slide", found.Path)
	}

	// Explain/compile parity: the plan must not promise kpi-3up here.
	if got := result.IR.Slides[0].Visual.Pattern; got != "" {
		t.Errorf("plan pattern = %q, want none — compile emitted bullets", got)
	}
}

// Validate alone (validate_deck_spec, which never compiles) must carry the same
// finding: it is the tool the workflow says to call before rendering.
func TestValidateReportsKPIBudgetDegrade(t *testing.T) {
	diags := Validate(kpiSpec("EUR 1,186.42 million", "117%", "6.2%"), StrictnessWarn)
	for _, d := range diags {
		if strings.Contains(d.Message, "degrades to a bullet list") {
			return
		}
	}
	t.Errorf("validate reported no degrade finding: %+v", diags)
}
