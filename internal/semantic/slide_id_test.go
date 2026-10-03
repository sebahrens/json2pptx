package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-1w3uo: every kind accepts an optional id, and an id that
// could not address exactly one slide is a blocking finding.
func TestSlideIDField(t *testing.T) {
	spec := func(firstID, secondID string) []byte {
		return []byte(`{"meta": {"title": "Ids"}, "slides": [
  {"id": ` + firstID + `, "kind": "title", "title": "Ids"},
  {"id": ` + secondID + `, "kind": "closing", "title": "Questions?"}
]}`)
	}
	idFindings := func(data []byte) []diagnostics.Diagnostic {
		var out []diagnostics.Diagnostic
		for _, d := range Check("deck.json", data, StrictnessWarn) {
			if strings.HasSuffix(d.Path, ".id") {
				out = append(out, d)
			}
		}
		return out
	}

	if got := idFindings(spec(`"cover"`, `"the-end_2"`)); len(got) != 0 {
		t.Errorf("well-formed unique ids should validate clean, got %+v", got)
	}
	for _, kind := range AllSlideKinds() {
		if _, ok := kindPayloadFields[kind]["id"]; !ok {
			t.Errorf("kind %s does not accept id", kind)
		}
	}

	cases := []struct {
		name          string
		first, second string
		path, want    string
	}{
		{"duplicate", `"cover"`, `"cover"`, "slides[1].id", "already used by slides[0]"},
		{"reads as an index", `"3"`, `"end"`, "slides[0].id", "start with a letter"},
		{"has a slash", `"a/b"`, `"end"`, "slides[0].id", "start with a letter"},
		{"not a string", `7`, `"end"`, "slides[0].id", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := idFindings(spec(tc.first, tc.second))
			if len(got) != 1 {
				t.Fatalf("want one id finding, got %+v", got)
			}
			d := got[0]
			if d.Path != tc.path || d.Code != diagnostics.CodeSemanticFieldType || d.Severity != diagnostics.SeverityError {
				t.Errorf("finding = %s %s at %s (%s)", d.Severity, d.Code, d.Path, d.Message)
			}
			if !strings.Contains(d.Message, tc.want) {
				t.Errorf("message %q should mention %q", d.Message, tc.want)
			}
		})
	}
}

// The structured form's slides carry ids too, and the expanded stream keeps
// each slide's source path.
func TestExpandedSlideSources(t *testing.T) {
	data := []byte(`{"meta": {"title": "Chapters"}, "structure": {
  "cover": {"id": "cover", "kind": "title", "title": "Chapters"},
  "sections": [{"title": "One", "slides": [{"id": "a", "kind": "closing", "title": "A"}]}],
  "closing": {"kind": "closing", "title": "End"}
}}`)
	spec, diags := Parse("deck.json", data)
	if diags.HasErrors() {
		t.Fatalf("parse: %+v", diags.ToDiagnostics())
	}
	got := ExpandedSlideSources(spec)
	want := []struct{ path, id string }{
		{"structure.cover", "cover"},
		{"structure.sections[0]", ""},
		{"structure.sections[0].slides[0]", "a"},
		{"structure.closing", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("expanded %d slides, want %d", len(got), len(want))
	}
	for i, w := range want {
		id, _ := got[i].Slide.Body["id"].(string)
		if got[i].SourcePath != w.path || id != w.id {
			t.Errorf("slide %d = %s (id %q), want %s (id %q)", i, got[i].SourcePath, id, w.path, w.id)
		}
	}
}
