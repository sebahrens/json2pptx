package semantic

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func init() {
	for kind := range consultingVisualKinds {
		probes := map[string]fieldProbe{}
		info := slideKindRegistry[kind]
		fields := append(append([]string{}, info.RequiredFields...), info.TypicalFields...)
		for _, field := range fields {
			field := field
			rendered := !(kind == KindImageHero && field == "title") && !(kind == KindHeatmap && field == "values")
			probes[field] = fieldProbe{rendered: rendered, why: "Image hero title is metadata; headline is the visible title. Numeric heatmap values preserve numbers rather than accepting a text sentinel; numeric parity is covered separately.", inject: func(sentinel string) map[string]any {
				raw, _ := json.Marshal(KindExample(kind))
				var body map[string]any
				_ = json.Unmarshal(raw, &body)
				delete(body, "kind")
				if rendered {
					body[field] = consultingSentinel(body[field], sentinel)
				}
				return body
			}}
		}
		payloadFieldCoverage[kind] = probes
	}
}
func consultingSentinel(value any, sentinel string) any {
	switch value := value.(type) {
	case string:
		return sentinel
	case []any:
		if len(value) > 0 {
			value[0] = consultingSentinel(value[0], sentinel)
		}
		return value
	case map[string]any:
		for _, key := range []string{"label", "name", "title", "heading", "activity", "workstream", "text", "date", "path", "paragraphs", "items", "description"} {
			if child, ok := value[key]; ok {
				value[key] = consultingSentinel(child, sentinel)
				return value
			}
		}
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			value[keys[0]] = consultingSentinel(value[keys[0]], sentinel)
		}
		return value
	}
	return value
}

func TestConsultingHeatmapNumericPayloadRoundTrip(t *testing.T) {
	body := KindExample(KindHeatmap)
	delete(body, "kind")
	numbers := []any{[]any{-7.25, 1e30}, []any{0.0, 5.5}}
	body["values"] = numbers
	input, _, err := Compile(&DeckSpec{Meta: DeckMeta{Title: "Numeric evidence"}, Slides: []SlideSpec{{Kind: KindHeatmap, Body: body}}}, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(input.Slides[0].Pattern.Values, &values); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values["values"], numbers) {
		t.Fatalf("numeric data changed: %#v", values["values"])
	}
}
