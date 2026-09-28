package semantic

import "testing"

// TestSourcePointerMapsToSemanticSource pins go-slide-creator-cuszt: a
// DATA_WITHOUT_SOURCE finding addresses /slides/N/source, which a data slide
// without a source never populated. The source map must still resolve it to
// the DeckSpec field the author fills.
func TestSourcePointerMapsToSemanticSource(t *testing.T) {
	spec := &DeckSpec{Meta: DeckMeta{Title: "Numbers"}, Slides: []SlideSpec{{
		Kind: KindKPISnapshot,
		Body: map[string]any{
			"title": "Q4 at a glance",
			"kpis": []any{
				map[string]any{"value": "$48M", "label": "Revenue"},
				map[string]any{"value": "118%", "label": "Net retention"},
				map[string]any{"value": "41d", "label": "Sales cycle"},
			},
			"takeaway": "Growth and efficiency both improved.",
		},
	}}}
	_, result, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v; diagnostics: %+v", err, result.Diagnostics)
	}
	path, idx, ok := result.SourceMap.ResolveSemantic("/slides/0/source")
	if !ok || path != "slides[0].source" || idx != 0 {
		t.Errorf("ResolveSemantic(/slides/0/source) = %q, %d, %v; want slides[0].source on slide 0", path, idx, ok)
	}
}
