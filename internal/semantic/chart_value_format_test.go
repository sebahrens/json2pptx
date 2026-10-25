package semantic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func TestSemanticChartNumberFormatCompilesOnBothPaths(t *testing.T) {
	for _, kind := range []SlideKind{KindChartInsight, KindRegions} {
		for _, style := range []string{"percent", "currency", "compact"} {
			format := map[string]any{"style": style, "decimals": 1, "prefix": "€"}
			if style == "percent" {
				format["input_scale"] = "fraction"
			}
			chart := map[string]any{"type": "bar_chart", "value_format": format, "data": map[string]any{"categories": []any{"A", "B"}, "series": []any{map[string]any{"name": "Margin", "values": []any{0.2, 0.3}}}}}
			body := map[string]any{"title": "Margins improve", "insight": "Growth funds the launch", "chart": chart}
			if kind == KindRegions {
				body = map[string]any{"title": "Margins improve", "regions": []any{map[string]any{"kind": "chart", "chart": chart}, map[string]any{"kind": "text", "body": "Growth funds the launch"}}}
			}
			spec := &DeckSpec{Meta: DeckMeta{Title: "Margins"}, Slides: []SlideSpec{{Kind: kind, Body: body}}}
			input, result, err := Compile(spec, CompileOptions{})
			if err != nil {
				t.Fatalf("%s %s: %v: %+v", kind, style, err, result.Diagnostics)
			}
			encoded, _ := json.Marshal(input)
			if !strings.Contains(string(encoded), `"value_format":`) || !strings.Contains(string(encoded), `"style":"`+style+`"`) {
				t.Fatalf("format lost: %s", encoded)
			}
			format["decimals"] = 1.5
			found := false
			for _, d := range Validate(spec, StrictnessWarn) {
				if d.Code == diagnostics.CodeSemanticFieldType && strings.HasSuffix(d.Path, ".value_format.decimals") {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s has no path-scoped format error", kind)
			}
		}
	}
}
