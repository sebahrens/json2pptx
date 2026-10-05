package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-ptazs: the fields the roadmap, comparison and image_case
// kinds gained are validated with the usual findings and budgets.

func validateOne(t *testing.T, kind SlideKind, body map[string]any) []diagnostics.Diagnostic {
	t.Helper()
	return Validate(&DeckSpec{Meta: DeckMeta{Title: "Overrides"}, Slides: []SlideSpec{{Kind: kind, Body: body}}}, StrictnessWarn)
}

func findingAt(ds []diagnostics.Diagnostic, code diagnostics.Code, path string) *diagnostics.Diagnostic {
	for i := range ds {
		if ds[i].Code == code && ds[i].Path == path {
			return &ds[i]
		}
	}
	return nil
}

func roadmapBody(extra map[string]any) map[string]any {
	body := KindExample(KindRoadmap)
	delete(body, "kind")
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func TestRoadmapParallelTracksValidate(t *testing.T) {
	if ds := validateOne(t, KindRoadmap, roadmapBody(nil)); len(ds) != 0 {
		t.Fatalf("the roadmap example (which carries parallel tracks) validates with findings: %v", ds)
	}

	ds := validateOne(t, KindRoadmap, roadmapBody(map[string]any{"parallel_tracks": []any{"A", "B", "C", "D", "E"}}))
	d := findingAt(ds, diagnostics.CodeSemanticPatternDegraded, "slides[0].parallel_tracks")
	if d == nil {
		t.Fatalf("five tracks: no SEMANTIC_PATTERN_DEGRADED at slides[0].parallel_tracks: %v", ds)
	}
	if d.Fix == nil || d.Fix.Params["max_items"] != 4 || d.Fix.Params["from"] != "phase-roadmap" {
		t.Errorf("five tracks: fix = %+v, want max_items 4 from phase-roadmap", d.Fix)
	}

	ds = validateOne(t, KindRoadmap, roadmapBody(map[string]any{"parallel_tracks": []any{strings.Repeat("x", 91)}}))
	d = findingAt(ds, diagnostics.CodeSemanticPatternDegraded, "slides[0].parallel_tracks[0]")
	if d == nil {
		t.Fatalf("long track: no finding at slides[0].parallel_tracks[0]: %v", ds)
	}
	if d.Details["allowed"] != 90 || !strings.Contains(d.Message, "90") {
		t.Errorf("long track: finding does not quote the 90-character budget: %+v", d)
	}

	ds = validateOne(t, KindRoadmap, roadmapBody(map[string]any{"parallel_label": strings.Repeat("l", 25), "parallel_tracks": []any{"Governance"}}))
	if findingAt(ds, diagnostics.CodeSemanticPatternDegraded, "slides[0].parallel_label") == nil {
		t.Errorf("long parallel_label: no finding: %v", ds)
	}

	// The field type contract: a string where the list belongs is dropped
	// silently by the compiler, so it is a SEMANTIC_FIELD_TYPE finding.
	ds = validateOne(t, KindRoadmap, roadmapBody(map[string]any{"parallel_tracks": "Governance"}))
	if findingAt(ds, diagnostics.CodeSemanticFieldType, "slides[0].parallel_tracks") == nil {
		t.Errorf("string parallel_tracks: no SEMANTIC_FIELD_TYPE: %v", ds)
	}
}

func comparisonBody(extra map[string]any) map[string]any {
	body := map[string]any{
		"title": "From today's close to the target",
		"columns": []any{
			map[string]any{"header": "Today", "items": []any{"Manual reconciliation", "Monthly close in 9 days"}},
			map[string]any{"header": "Target", "items": []any{"Automated matching", "Close in 3 days"}},
		},
		"takeaway": "Automation takes six days out of the close.",
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func TestComparisonOverridesValidate(t *testing.T) {
	if ds := validateOne(t, KindComparison, comparisonBody(map[string]any{"connectors": true, "highlight_column": "Target"})); len(ds) != 0 {
		t.Fatalf("connectors + highlight_column by header validate with findings: %v", ds)
	}
	if ds := validateOne(t, KindComparison, comparisonBody(map[string]any{"highlight_row": 1})); len(ds) != 0 {
		t.Fatalf("highlight_row by index validates with findings: %v", ds)
	}

	ds := validateOne(t, KindComparison, comparisonBody(map[string]any{"highlight_column": "Middle"}))
	d := findingAt(ds, diagnostics.CodeSemanticReferenceUnresolved, "slides[0].highlight_column")
	if d == nil || d.Severity != diagnostics.SeverityError {
		t.Fatalf("highlight_column Middle: no SEMANTIC_REFERENCE_UNRESOLVED error: %v", ds)
	}
	for _, want := range []string{`"left"`, `"right"`, "Today", "Target"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("highlight_column message does not offer %s: %q", want, d.Message)
		}
	}

	ds = validateOne(t, KindComparison, comparisonBody(map[string]any{"highlight_row": 7}))
	if d := findingAt(ds, diagnostics.CodeSemanticReferenceUnresolved, "slides[0].highlight_row"); d == nil || d.Severity != diagnostics.SeverityError {
		t.Fatalf("highlight_row 7: no SEMANTIC_REFERENCE_UNRESOLVED error: %v", ds)
	}

	ds = validateOne(t, KindComparison, comparisonBody(map[string]any{"highlight_row": 0, "highlight_column": "left"}))
	if d := findingAt(ds, diagnostics.CodeSemanticFieldType, "slides[0].highlight_row"); d == nil || d.Severity != diagnostics.SeverityError {
		t.Fatalf("row and column highlights together: no error at highlight_row: %v", ds)
	}

	// On a three-column comparison the two-column overrides have nothing to
	// act on: the slide keeps its panels and the author is told.
	three := comparisonBody(map[string]any{"connectors": true})
	three["columns"] = append(three["columns"].([]any), map[string]any{"header": "Stretch", "items": []any{"Continuous close", "Real-time matching"}})
	ds = validateOne(t, KindComparison, three)
	d = findingAt(ds, diagnostics.CodeSemanticPatternNotAvailable, "slides[0].connectors")
	if d == nil || d.Severity != diagnostics.SeverityWarning {
		t.Fatalf("connectors on three columns: no SEMANTIC_PATTERN_NOT_AVAILABLE warning: %v", ds)
	}
	if !strings.Contains(d.Message, "stylish-panels") {
		t.Errorf("message does not name the visual the slide takes: %q", d.Message)
	}

	ds = validateOne(t, KindComparison, comparisonBody(map[string]any{"connectors": "yes"}))
	if findingAt(ds, diagnostics.CodeSemanticFieldType, "slides[0].connectors") == nil {
		t.Errorf("string connectors: no SEMANTIC_FIELD_TYPE: %v", ds)
	}
}

func TestImageCaseWidthValidate(t *testing.T) {
	base := func(width any) map[string]any {
		body := KindExample(KindImageCase)
		delete(body, "kind")
		delete(body, "image_label")
		body["image"] = map[string]any{"path": "shots/console.png", "fit": "contain"}
		body["image_width_pct"] = width
		return body
	}
	if ds := validateOne(t, KindImageCase, base(60)); len(ds) != 0 {
		t.Fatalf("image_width_pct 60 validates with findings: %v", ds)
	}
	ds := validateOne(t, KindImageCase, base(75))
	d := findingAt(ds, diagnostics.CodeSemanticFieldType, "slides[0].image_width_pct")
	if d == nil || d.Severity != diagnostics.SeverityError {
		t.Fatalf("image_width_pct 75: no SEMANTIC_FIELD_TYPE error: %v", ds)
	}
	if !strings.Contains(d.Message, "30") || !strings.Contains(d.Message, "60") {
		t.Errorf("message does not state the 30–60 range: %q", d.Message)
	}
	ds = validateOne(t, KindImageCase, base("60"))
	if findingAt(ds, diagnostics.CodeSemanticFieldType, "slides[0].image_width_pct") == nil {
		t.Errorf("string image_width_pct: no SEMANTIC_FIELD_TYPE: %v", ds)
	}
}

// The three kinds advertise the new fields where an agent discovers them.
func TestOverrideFieldsAreAdvertised(t *testing.T) {
	cases := map[SlideKind][]string{
		KindRoadmap:    {"parallel_tracks", "parallel_label"},
		KindComparison: {"connectors", "highlight_column", "highlight_row"},
		KindImageCase:  {"image_width_pct"},
	}
	for kind, fields := range cases {
		info, _ := LookupKind(kind)
		names := PayloadFieldNames(kind)
		for _, f := range fields {
			if !containsKey(names, f) {
				t.Errorf("%s: %q is not a payload field", kind, f)
			}
			if !containsKey(info.TypicalFields, f) {
				t.Errorf("%s: %q is not a typical field in list_slide_kinds", kind, f)
			}
			if desc := kindPayloadFields[kind][f].desc; strings.TrimSpace(desc) == "" {
				t.Errorf("%s.%s has no description", kind, f)
			}
		}
	}
}
