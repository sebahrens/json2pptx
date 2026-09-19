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

// go-slide-creator-6o1r: icon.svg_data is the documented way to supply an inline
// icon, and its markup never reaches the run builder. Scanning it reported every
// <svg>/<path>/<circle> as text that "prints literally on the slide" — false,
// and unfixable by the author — on every deck using an inline icon or a
// harvey-ball table-highlight.
func TestScanSkipsMarkupFields(t *testing.T) {
	const icon = `<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/></svg>`
	deck := map[string]any{"slides": []any{map[string]any{
		"shape_grid": map[string]any{"rows": []any{map[string]any{"cells": []any{
			map[string]any{"icon": map[string]any{"svg_data": icon, "alt": "a dot"}},
			// A nested grid's icon is as exempt as a top-level one.
			map[string]any{"grid": map[string]any{"rows": []any{map[string]any{"cells": []any{
				map[string]any{"icon": map[string]any{"svg_data": icon}},
			}}}}},
		}}}},
	}}}
	if got := Scan(deck); len(got) != 0 {
		t.Errorf("inline SVG icons reported as unsupported text markup: %+v", got)
	}

	// A field that IS authored text still reports, including one sitting beside
	// an exempt field.
	deck2 := map[string]any{"slides": []any{map[string]any{
		"shape_grid": map[string]any{"rows": []any{map[string]any{"cells": []any{
			map[string]any{
				"icon":  map[string]any{"svg_data": icon},
				"shape": map[string]any{"text": "Revenue <color>up</color>"},
			},
		}}}},
	}}}
	got := Scan(deck2)
	if len(got) != 1 {
		t.Fatalf("want exactly the text violation, got %+v", got)
	}
	if len(got[0].Tags) != 1 || got[0].Tags[0] != "color" {
		t.Errorf("tags = %v, want [color]", got[0].Tags)
	}
}

func TestAuthoredText(t *testing.T) {
	for _, path := range []string{
		"slides[0].content[0].text_value",
		"slides[0].shape_grid.rows[0].cells[0].shape.text",
		"svg_data_note", // a field merely starting with the name is not exempt
	} {
		if !authoredText(path) {
			t.Errorf("authoredText(%q) = false, want true", path)
		}
	}
	for _, path := range []string{
		"svg_data",
		"slides[0].shape_grid.rows[0].cells[0].icon.svg_data",
		"slides[0].shape_grid.rows[4].cells[0].grid.rows[0].cells[2].icon.svg_data",
	} {
		if authoredText(path) {
			t.Errorf("authoredText(%q) = true, want false", path)
		}
	}
}
