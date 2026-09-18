package inlinemarkup

import (
	"strings"
	"testing"
)

// go-slide-creator-510u: get_capabilities advertised supports_inline_markup
// [b,i,u] and any other tag was passed through to the text run verbatim, so a
// deck using <sup>, <a>, <color> or <code> shipped visible XML-ish garbage —
// and validate said nothing.
func TestScan_FlagsUnsupportedTagsOnly(t *testing.T) {
	type deck struct {
		Bullets []string          `json:"bullets"`
		Title   string            `json:"title"`
		Cells   map[string]string `json:"cells"`
	}

	in := deck{
		Title: "Pricing <b>reset</b>",
		Bullets: []string{
			"Pricing reset delivered +210bps<sup>1</sup> with no volume loss",
			`Link: <a href="http://example.com">example.com</a> and <color val="red">coloured</color> and <code>fn()</code>`,
			"Plain prose with a < that is not a tag",
			"Nested <b><i>emphasis</i></b> is fine",
		},
		Cells: map[string]string{"a1": "H<sub>2</sub>O"},
	}

	violations := Scan(in)
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation (the <a>/<color>/<code> bullet), got %d: %+v", len(violations), violations)
	}
	v := violations[0]
	if v.Path != "bullets[1]" {
		t.Errorf("path = %q, want bullets[1]", v.Path)
	}
	want := []string{"a", "code", "color"}
	if strings.Join(v.Tags, ",") != strings.Join(want, ",") {
		t.Errorf("tags = %v, want %v (sorted, deduped)", v.Tags, want)
	}
}

// Supported tags — including the newly added sup/sub — must never be reported.
func TestScan_SupportedTagsAreQuiet(t *testing.T) {
	for _, tag := range SupportedTags {
		t.Run(tag, func(t *testing.T) {
			text := "before <" + tag + ">inner</" + tag + "> after"
			if v := Scan(map[string]string{"t": text}); len(v) != 0 {
				t.Errorf("supported tag <%s> reported as unsupported: %+v", tag, v)
			}
		})
	}
}

// The finding must be advisory and carry both vocabularies so an agent can
// repair without a second lookup.
func TestValidate_FindingShape(t *testing.T) {
	findings := Validate(map[string]string{"text_value": "see <footnote>1</footnote>"})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Action != "review" {
		t.Errorf("action = %q, want review (the deck still renders, just wrongly)", f.Action)
	}
	if f.Code != "UNSUPPORTED_INLINE_MARKUP" {
		t.Errorf("code = %q", f.Code)
	}
	if f.Fix == nil || f.Fix.Kind != "remove_key" {
		t.Fatalf("fix = %+v, want kind remove_key", f.Fix)
	}
	if got, ok := f.Fix.Params["unsupported"].([]string); !ok || len(got) != 1 || got[0] != "footnote" {
		t.Errorf("fix.params.unsupported = %v, want [footnote]", f.Fix.Params["unsupported"])
	}
	if got, ok := f.Fix.Params["supported"].([]string); !ok || len(got) != len(SupportedTags) {
		t.Errorf("fix.params.supported = %v, want the supported vocabulary", f.Fix.Params["supported"])
	}
	if !strings.Contains(f.Message, "<footnote>") || !strings.Contains(f.Message, "print literally") {
		t.Errorf("message should name the tag and its effect, got: %s", f.Message)
	}
}

// Prose with no angle bracket must not be walked for tags at all.
func TestScan_NoTagsIsQuiet(t *testing.T) {
	if v := Scan(map[string]any{
		"a": "Ordinary prose, 12 > 3 in value terms",
		"b": []string{"one", "two"},
	}); len(v) != 0 {
		t.Errorf("expected no violations, got %+v", v)
	}
}
