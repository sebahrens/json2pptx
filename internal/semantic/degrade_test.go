package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// degradeCases are one spec per slide kind that announces a fallback, so the
// contract is asserted where the advisories actually live rather than on a
// hand-maintained list of codes.
func degradeCases() map[string]SlideSpec {
	return map[string]SlideSpec{
		"agenda": {Kind: KindAgenda, Body: map[string]any{
			"title": "Agenda", "sections": []any{"only one"},
		}},
		"quote": {Kind: KindQuote, Body: map[string]any{
			"title": "Voices", "quotes": []any{
				map[string]any{"text": "First", "name": "A"},
				map[string]any{"text": "Second", "name": "B"},
			},
		}},
		"executive_summary": {Kind: KindExecutiveSummary, Body: map[string]any{
			"title": "Summary", "takeaway": "t", "points": []any{"one point"},
		}},
		"kpi_snapshot": {Kind: KindKPISnapshot, Body: map[string]any{
			"title": "KPIs", "takeaway": "t",
			"kpis": []any{map[string]any{"value": "1", "label": "a"}},
		}},
		"process": {Kind: KindProcess, Body: map[string]any{
			"title": "Process", "steps": []any{"a", "b"},
		}},
		"roadmap": {Kind: KindRoadmap, Body: map[string]any{
			"title": "Roadmap", "phases": []any{"one"},
		}},
		"chart_insight": {Kind: KindChartInsight, Body: map[string]any{
			"title": "Chart", "takeaway": "t", "insight": "i",
			"chart": map[string]any{"type": "bar"},
		}},
		"comparison": {Kind: KindComparison, Body: map[string]any{
			"title": "A vs B", "takeaway": "t",
			"columns": []any{
				map[string]any{"header": "A", "items": []any{"x"}},
				map[string]any{"header": "B", "items": []any{"y"}},
				map[string]any{"header": "C", "items": []any{"z"}},
				map[string]any{"header": "D", "items": []any{"w"}},
				map[string]any{"header": "E", "items": []any{"v"}},
				map[string]any{"header": "F", "items": []any{"u"}},
			},
		}},
		"option_matrix": {Kind: KindOptionMatrix, Body: map[string]any{
			"title": "Options", "takeaway": "t",
			"criteria": []any{"only one"},
			"options": []any{
				map[string]any{"name": "A", "scores": []any{1}},
				map[string]any{"name": "B", "scores": []any{2}},
			},
		}},
	}
}

// TestEveryFallbackAdvisoryIsPatternDegraded is the bead's first VERIFY clause
// (go-slide-creator-kjc8l): every advisory that announces a fallback carries
// SEMANTIC_PATTERN_DEGRADED, not SEMANTIC_DENSITY.
func TestEveryFallbackAdvisoryIsPatternDegraded(t *testing.T) {
	for name, slide := range degradeCases() {
		t.Run(name, func(t *testing.T) {
			ds := Validate(&DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{slide}}, StrictnessWarn)
			found := findingsWithCode(ds, diagnostics.CodeSemanticPatternDegraded)
			if len(found) == 0 {
				t.Fatalf("no SEMANTIC_PATTERN_DEGRADED, got %v", codesOf(ds))
			}
			d := found[0]
			if d.Fix == nil {
				t.Fatal("degrade advisory carries no fix")
			}
			for _, key := range []string{"from", "to", "reason"} {
				if _, ok := d.Fix.Params[key]; !ok {
					t.Errorf("fix.params is missing %q: %v", key, d.Fix.Params)
				}
			}
			switch to := d.Fix.Params["to"]; to {
			case degradeToBullets, degradeToContent, degradeToTwoColumn, degradeToInsightsOnly:
			default:
				t.Errorf("fix.params.to = %v, not one of the documented targets", to)
			}
		})
	}
}

// TestNoFallbackAdvisoryStaysOnDensity is the second VERIFY clause: an advisory
// that only recommends a count range keeps SEMANTIC_DENSITY. A table wider than
// the renderer lays out still renders as a table.
func TestNoFallbackAdvisoryStaysOnDensity(t *testing.T) {
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{{
		Kind: KindTable, Body: map[string]any{
			"title": "Wide", "takeaway": "t",
			"headers": []any{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			"rows":    []any{[]any{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}},
		},
	}}}
	ds := Validate(spec, StrictnessWarn)
	if !hasCode(ds, diagnostics.CodeSemanticDensity) {
		t.Fatalf("expected SEMANTIC_DENSITY for an over-wide table, got %v", codesOf(ds))
	}
	if hasCode(ds, diagnostics.CodeSemanticPatternDegraded) {
		t.Errorf("a table that still renders as a table must not report a pattern degrade: %v", codesOf(ds))
	}
}

// TestDegradeMessagesStillNameTheFallback guards the messages: the code is the
// machine-readable half, but a human reading the finding must still be told
// what the slide turns into.
func TestDegradeMessagesStillNameTheFallback(t *testing.T) {
	for name, slide := range degradeCases() {
		t.Run(name, func(t *testing.T) {
			ds := Validate(&DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{slide}}, StrictnessWarn)
			for _, d := range findingsWithCode(ds, diagnostics.CodeSemanticPatternDegraded) {
				msg := strings.ToLower(d.Message)
				if !strings.Contains(msg, "degrade") && !strings.Contains(msg, "dropped") {
					t.Errorf("message does not say what is lost: %q", d.Message)
				}
			}
		})
	}
}

// TestPatternDegradedIsDescribable is the third VERIFY clause:
// describe_finding("SEMANTIC_PATTERN_DEGRADED") must return remediation steps.
func TestPatternDegradedIsDescribable(t *testing.T) {
	d, ok := diagnostics.Describe(string(diagnostics.CodeSemanticPatternDegraded))
	if !ok {
		t.Fatal("SEMANTIC_PATTERN_DEGRADED has no describe entry")
	}
	if len(d.RemediationSteps) == 0 {
		t.Error("describe entry carries no remediation steps")
	}
	if !strings.Contains(strings.Join(d.RemediationSteps, " "), "fix.params") {
		t.Error("remediation steps should point the agent at fix.params")
	}
}
