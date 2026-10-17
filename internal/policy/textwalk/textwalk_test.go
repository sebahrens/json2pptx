package textwalk

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStringsAtTraversesAuthoredPayloads(t *testing.T) {
	input := struct {
		Slides []any  `json:"slides"`
		Hidden string `json:"-"`
	}{
		Slides: []any{map[string]any{"text": "hello", "raw": json.RawMessage(`{"a/b~c":["nested",42,null]}`)}}, Hidden: "ignore",
	}
	got := map[string][2]string{}
	StringsAt(input, func(value, path, pointer string) { got[pointer] = [2]string{value, path} })
	want := map[string][2]string{"/slides/0/text": {"hello", "slides[0].text"}, "/slides/0/raw/a~1b~0c/0": {"nested", "slides[0].raw.a/b~c[0]"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visits=%v want=%v", got, want)
	}
}
func TestStringsHandlesUnsupportedAndRootValues(t *testing.T) {
	for _, input := range []any{nil, make(chan int), json.RawMessage(`{`)} {
		Strings(input, func(string, string) { t.Errorf("invalid input visited: %T", input) })
	}
	StringsAt("root", func(value, path, pointer string) {
		if value != "root" || path != "" || pointer != "" {
			t.Errorf("root visit: %q %q %q", value, path, pointer)
		}
	})
	Strings("safe", nil)
	StringsAt("safe", nil)
	if DisplayPath("") != "<root>" || DisplayPath("slides[0]") != "slides[0]" {
		t.Fatal("display path changed")
	}
}
