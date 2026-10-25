package semantic

import (
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"strings"
	"testing"
)

func TestSemanticFocalPointSurvivesCompileAndValidates(t *testing.T) {
	for _, kind := range []SlideKind{KindImageCase, KindRegions} {
		picture := map[string]any{"path": "photo.png", "focal_point": map[string]any{"x": .1, "y": .5}}
		body := map[string]any{"title": "Subject matters", "heading": "Customer voice", "body": "The customer explains the change.", "image": picture}
		if kind == KindRegions {
			body = map[string]any{"title": "Subject matters", "regions": []any{map[string]any{"kind": "image", "image": picture}, map[string]any{"kind": "text", "body": "The customer explains the change."}}}
		}
		spec := &DeckSpec{Meta: DeckMeta{Title: "Story"}, Slides: []SlideSpec{{Kind: kind, Body: body}}}
		input, result, err := Compile(spec, CompileOptions{})
		if err != nil {
			t.Fatalf("%s: %v: %+v", kind, err, result.Diagnostics)
		}
		encoded, _ := json.Marshal(input)
		if !strings.Contains(string(encoded), `"focal_point":{"x":0.1,"y":0.5}`) {
			t.Fatalf("focal point lost: %s", encoded)
		}
		picture["focal_point"] = map[string]any{"x": 1.1}
		found := false
		for _, d := range Validate(spec, StrictnessWarn) {
			if d.Code == diagnostics.CodeSemanticFieldType && strings.Contains(d.Path, ".image.focal_point.") {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s accepted invalid focal point", kind)
		}
	}
}
