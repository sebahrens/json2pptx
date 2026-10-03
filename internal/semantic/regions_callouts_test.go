package semantic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-n3j96: an image region draws no callouts. The schema says
// where they belong, and callouts written on a region are refused with that
// pointer — not with "did you mean caption?", which is one edit away.
func TestRegionsImageRegionPointsCalloutsAtImageCase(t *testing.T) {
	schema, err := json.Marshal(KindItemSchema(KindRegions))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(schema), "use the image_case kind (callouts [{label, x, y}])") {
		t.Error("the regions schema does not send callouts on a picture to image_case")
	}

	body := chartStatTimeline()
	body["arrangement"] = "columns"
	body["regions"] = []any{
		body["regions"].([]any)[0],
		map[string]any{"kind": "image", "image": map[string]any{"path": "console.png", "alt": "Console"},
			"callouts": []any{map[string]any{"label": "Delayed queue", "x": 0.5, "y": 0.5}}},
	}
	var got []diagnostics.Diagnostic
	for _, d := range Validate(regionsSpec(body), StrictnessWarn) {
		if d.Path == "slides[0].regions[1].callouts" {
			got = append(got, d)
		}
	}
	if len(got) != 1 || got[0].Code != diagnostics.CodeSemanticUnknownField || got[0].Severity != diagnostics.SeverityError {
		t.Fatalf("callouts on an image region: got %+v, want one %s error", got, diagnostics.CodeSemanticUnknownField)
	}
	if !strings.Contains(got[0].Message, "image_case") || strings.Contains(got[0].Message, "caption") || got[0].Fix != nil {
		t.Errorf("the refusal should point at image_case and suggest no rename: %q fix=%+v", got[0].Message, got[0].Fix)
	}
}
