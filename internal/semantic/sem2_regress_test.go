package semantic

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func oneSlideSpec(kind SlideKind, body map[string]any) *DeckSpec {
	return &DeckSpec{
		Meta:   DeckMeta{Title: "Deck", Template: "midnight-blue"},
		Slides: []SlideSpec{{Kind: kind, Body: body}},
	}
}

// A per-slide pattern object (e.g. {overrides:{type_scale}}) was silently
// ignored (go-slide-creator-csclk.50).
func TestNonStringPatternOverrideIsFieldType(t *testing.T) {
	spec := oneSlideSpec(KindStat, map[string]any{
		"title": "Growth", "value": "42%", "label": "YoY growth",
		"pattern": map[string]any{"overrides": map[string]any{"type_scale": "compact"}},
	})
	if got := findingsWithCode(Validate(spec, StrictnessWarn), diagnostics.CodeSemanticFieldType); len(got) != 1 || got[0].Path != "slides[0].pattern" {
		t.Fatalf("want one SEMANTIC_FIELD_TYPE at slides[0].pattern, got %+v", got)
	}
}

// Dangling highlight references vanished without a finding (go-slide-creator-csclk.51).
func TestDanglingReferencesAreReported(t *testing.T) {
	matrix := oneSlideSpec(KindOptionMatrix, map[string]any{
		"title":              "Options",
		"criteria":           []any{"Cost", "Speed"},
		"options":            []any{map[string]any{"name": "A", "scores": []any{1, 2}}, map[string]any{"name": "B", "scores": []any{3, 4}}},
		"recommended":        "Z",
		"decisive_criterion": "q",
		"takeaway":           "A wins.",
	})
	if got := findingsWithCode(Validate(matrix, StrictnessWarn), diagnostics.CodeSemanticReferenceUnresolved); len(got) != 2 {
		t.Fatalf("option matrix: want 2 unresolved references, got %+v", got)
	}
	agenda := oneSlideSpec(KindAgenda, map[string]any{
		"title": "Agenda", "sections": []any{"Context", "Options", "Plan"}, "current": 9,
	})
	if got := findingsWithCode(Validate(agenda, StrictnessWarn), diagnostics.CodeSemanticReferenceUnresolved); len(got) != 1 {
		t.Fatalf("agenda: want 1 unresolved reference, got %+v", got)
	}
	agenda.Slides[0].Body["current"] = 2
	if got := findingsWithCode(Validate(agenda, StrictnessWarn), diagnostics.CodeSemanticReferenceUnresolved); len(got) != 0 {
		t.Fatalf("agenda: in-range current reported %+v", got)
	}
}

// validate passed raw pattern values render then refused (go-slide-creator-csclk.86).
func TestRawPatternValuesAreValidated(t *testing.T) {
	spec := oneSlideSpec(KindRawJSON2pptx, map[string]any{
		"slide": map[string]any{
			"layout_id": "blank-title",
			"pattern": map[string]any{
				"name":   "capability-heatmap",
				"values": map[string]any{"tiers": []any{"High", "Low"}},
			},
		},
	})
	if !diagnostics.HasErrors(Validate(spec, StrictnessWarn)) {
		t.Fatal("invalid raw pattern values validated clean")
	}
}
