package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Every shape_grid text body carries the uniform 0.5 cm margin unless the deck
// author sets a side explicitly; an explicit side (including 0) replaces that
// side only.
func TestResolveTextInputUsesTheUniformShapeMargin(t *testing.T) {
	for _, raw := range []string{
		`"Plain string"`,
		`{"content":"Object form"}`,
		`{"paragraphs":[{"content":"Paragraphs form"}]}`,
	} {
		tb, err := ResolveTextInput(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		if tb.Insets != pptx.ShapeTextInsets() {
			t.Errorf("%s: insets = %v, want the uniform margin", raw, tb.Insets)
		}
	}

	tb, err := ResolveTextInput(json.RawMessage(`{"content":"x","inset_top":0,"inset_left":6}`))
	if err != nil {
		t.Fatal(err)
	}
	want := [4]int64{6 * 12700, 0, pptx.ShapeTextInsetEMU, pptx.ShapeTextInsetEMU}
	if tb.Insets != want {
		t.Errorf("authored sides: insets = %v, want %v", tb.Insets, want)
	}
}
