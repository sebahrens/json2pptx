package semantic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// findDiag returns the first diagnostic with the given code, or nil.
func findDiag(ds []diagnostics.Diagnostic, code string) *diagnostics.Diagnostic {
	for i := range ds {
		if ds[i].Code == code {
			return &ds[i]
		}
	}
	return nil
}

// TestCompileKPISnapshot_LongValueFallsBackToContent is the regression for the
// field-report bug: a KPI value that is valid semantically but too long for the
// compact kpi-2up cards ("CHF 142.3M") must degrade to a content slide at
// compile time, not compile clean and then fail at render.
func TestCompileKPISnapshot_LongValueFallsBackToContent(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Results"},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Results"}},
			{Kind: KindKPISnapshot, Body: map[string]any{
				"title": "Q2 Results",
				"kpis": []any{
					map[string]any{"value": "CHF 142.3M", "label": "Revenue"},
					map[string]any{"value": "23%", "label": "Growth"},
				},
			}},
		},
	}

	input, _, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile should succeed via the KPI fallback, got: %v", err)
	}
	kpi := input.Slides[1]
	if kpi.Pattern != nil {
		t.Fatalf("a long KPI value should degrade to a content slide, but a %q pattern was emitted", kpi.Pattern.Name)
	}
	if len(kpi.Content) == 0 {
		t.Fatal("the fallback content slide should carry the metrics as content")
	}
}

// TestCompileKPISnapshot_ShortValuesEmitPattern guards against an over-eager
// fallback: KPI values that fit the cards must still produce a kpi-Nup pattern.
func TestCompileKPISnapshot_ShortValuesEmitPattern(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Results"},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Results"}},
			{Kind: KindKPISnapshot, Body: map[string]any{
				"title": "Q2 Results",
				"kpis": []any{
					map[string]any{"value": "$4.2M", "label": "Revenue"},
					map[string]any{"value": "23%", "label": "Growth"},
				},
			}},
		},
	}

	input, _, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	kpi := input.Slides[1]
	if kpi.Pattern == nil || kpi.Pattern.Name != "kpi-2up" {
		t.Fatalf("short KPI values should emit a kpi-2up pattern, got %+v", kpi.Pattern)
	}
}

// TestPreflightRawPatterns_MapsToSemanticWithEdit asserts the preflight catches
// a raw pattern that fails validation, traces it back to the semantic source
// path via the source map, and attaches a recommended semantic edit.
func TestPreflightRawPatterns_MapsToSemanticWithEdit(t *testing.T) {
	sm := NewSourceMap()
	sm.Add("slides[0].pattern.values[0]", "slides[0].kpis[0]", 0)

	input := &deckinput.PresentationInput{
		Slides: []deckinput.SlideInput{
			{
				SlideType: "content",
				Pattern: &deckinput.PatternInput{
					Name:   "kpi-2up",
					Values: json.RawMessage(`[{"big":"CHF 142.3M","small":"Revenue"},{"big":"23%","small":"Growth"}]`),
				},
			},
		},
	}

	diags := preflightRawPatterns(input, sm)
	d := findDiag(diags, "max_length")
	if d == nil {
		t.Fatalf("expected a max_length preflight finding, got: %+v", diags)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("preflight finding severity = %q, want error", d.Severity)
	}
	if d.Path != "slides[0].kpis[0]" {
		t.Errorf("semantic path = %q, want slides[0].kpis[0]", d.Path)
	}
	if rp, _ := d.Details["raw_path"].(string); !strings.HasPrefix(rp, "slides[0].pattern.values[0]") {
		t.Errorf("raw_path = %v, want a slides[0].pattern.values[0].* pointer", d.Details["raw_path"])
	}
	edit, ok := d.Details["recommended_edit"].(*SemanticEdit)
	if !ok {
		t.Fatalf("expected a recommended_edit of type *SemanticEdit, got %T", d.Details["recommended_edit"])
	}
	if edit.Kind != EditShortenText {
		t.Errorf("recommended edit kind = %q, want %q", edit.Kind, EditShortenText)
	}
}

// TestCompile_PreflightBlocksInvalidRawEscapeHatch asserts the post-compile
// preflight refuses to emit a deck whose raw_json2pptx escape-hatch slide
// carries a pattern that would fail at render, and that the blocking diagnostic
// traces back to the author's escape-hatch block.
func TestCompile_PreflightBlocksInvalidRawEscapeHatch(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Raw"},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Raw"}},
			{Kind: KindRawJSON2pptx, Body: map[string]any{
				"slide": map[string]any{
					"slide_type": "content",
					"pattern": map[string]any{
						"name": "kpi-2up",
						"values": []any{
							map[string]any{"big": "WAY TOO LONG", "small": "Revenue"},
							map[string]any{"big": "23%", "small": "Growth"},
						},
					},
				},
			}},
		},
	}

	input, result, err := Compile(spec, CompileOptions{})
	if err == nil {
		t.Fatal("preflight should block a deck whose raw pattern fails validation")
	}
	if input != nil {
		t.Error("no PresentationInput should be emitted when the preflight blocks")
	}
	if result == nil {
		t.Fatal("a blocking compile should still return diagnostics on the result")
	}
	d := findDiag(result.Diagnostics, "max_length")
	if d == nil {
		t.Fatalf("expected a max_length preflight finding, got: %+v", result.Diagnostics)
	}
	if !strings.HasPrefix(d.Path, "slides[1]") {
		t.Errorf("preflight finding should map back to semantic slides[1].*, got %q", d.Path)
	}
}
