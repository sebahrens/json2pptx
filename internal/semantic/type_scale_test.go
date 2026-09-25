package semantic

import (
	"encoding/json"
	"strings"
	"testing"
)

func typeScaleDeck(mode string) *DeckSpec {
	return &DeckSpec{
		Meta: DeckMeta{Title: "Board update", TypeScale: mode},
		Slides: []SlideSpec{{Kind: KindExecutiveSummary, Body: map[string]any{
			"title": "Where we stand", "points": []any{"Revenue grew", "Churn fell"}, "takeaway": "On track",
		}}},
	}
}

func TestTypeScaleMetaCompilesWithComfortableDefault(t *testing.T) {
	for _, tc := range []struct{ authored, want string }{
		{"", "comfortable"}, {"compact", "compact"}, {"presentation", "presentation"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			input, _, err := Compile(typeScaleDeck(tc.authored), CompileOptions{Strict: StrictnessWarn})
			if err != nil {
				t.Fatal(err)
			}
			if input.TypeScale != tc.want {
				t.Errorf("compiled type_scale = %q, want %q", input.TypeScale, tc.want)
			}
		})
	}
}

func TestTypeScaleMetaRejectsUnknownAndAppearsInSchema(t *testing.T) {
	diags := Validate(typeScaleDeck("giant"), StrictnessWarn)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Path, "meta.type_scale") {
			found = true
		}
	}
	if !found {
		t.Fatalf("invalid type_scale had no meta.type_scale diagnostic: %+v", diags)
	}
	schema, err := json.Marshal(Schema())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(schema), `"type_scale"`) || !strings.Contains(string(schema), `"presentation"`) {
		t.Fatal("published DeckSpec schema omits type_scale enum")
	}
}
