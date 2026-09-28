package semantic

import (
	"strings"
	"testing"
)

// A typed-decode failure names the author's field path and the expected shape,
// never a Go struct or package name (go-slide-creator-6p9mm).
func TestTypedDecodeErrorsNameTheFieldPath(t *testing.T) {
	for _, tc := range []struct {
		name, file, doc string
	}{
		{"json", "deck.json", `{"meta":{"title":"T","chrome":{"page_numbers":true}},"slides":[]}`},
		{"yaml", "deck.yaml", "meta:\n  title: T\n  chrome:\n    page_numbers: true\nslides: []\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ds := Parse(tc.file, []byte(tc.doc))
			if !ds.HasErrors() {
				t.Fatal("expected a decode error")
			}
			d := ds[0]
			if d.Path != "meta.chrome.page_numbers" || d.Code != CodeInvalidMeta {
				t.Errorf("path/code = %q/%q, want meta.chrome.page_numbers/%s", d.Path, d.Code, CodeInvalidMeta)
			}
			for _, leak := range []string{"Go struct", "semantic.", "ChromeSpec", "unmarshal"} {
				if strings.Contains(d.Message, leak) {
					t.Errorf("message leaks %q: %s", leak, d.Message)
				}
			}
			if !strings.Contains(d.Message, `did you mean {"enabled": true}`) || !strings.Contains(d.Message, "got a boolean") {
				t.Errorf("message lacks the shape and suggestion: %s", d.Message)
			}
		})
	}
}
