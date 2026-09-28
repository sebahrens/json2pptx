package svggen

import (
	"math"
	"regexp"
	"strings"
	"testing"
)

func unicodeBarRequest(categories ...string) *RequestEnvelope {
	cats := make([]any, len(categories))
	vals := make([]any, len(categories))
	for i, c := range categories {
		cats[i] = c
		vals[i] = float64(i + 1)
	}
	return &RequestEnvelope{Type: "bar_chart", Data: map[string]any{
		"categories": cats,
		"series":     []any{map[string]any{"name": "值", "values": vals}},
	}}
}

// go-slide-creator-s27x0: CJK / emoji / Hebrew / Arabic labels resolved to
// .notdef in the embedded face and no finding said so.
func TestGlyphMissing_ReportedForCJKEmojiRTL(t *testing.T) {
	req := unicodeBarRequest("שלום עולם", "مرحبا بالعالم", "日本語テキスト", "😀🚀 emoji", "Ünïcödé")
	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}
	var got *Finding
	for i := range findings {
		if findings[i].Code == FindingGlyphMissing {
			if got != nil {
				t.Fatalf("expected one %s finding per render, got several", FindingGlyphMissing)
			}
			got = &findings[i]
		}
	}
	if got == nil {
		t.Skip("embedded chart font covers every test script; nothing to report")
	}
	for _, script := range []string{"Han", "Arabic", "emoji/symbols"} {
		if !strings.Contains(got.Message, script) {
			t.Errorf("finding message does not name script %q: %s", script, got.Message)
		}
	}
	if got.Fix == nil || got.Fix.Kind != FixKindReplaceValue || got.Fix.Params["characters"] == nil {
		t.Errorf("fix = %+v, want replace_value with characters", got.Fix)
	}
	if strings.Contains(got.Message, "Ü") {
		t.Errorf("Latin-1 characters must not be reported missing: %s", got.Message)
	}
}

func TestGlyphMissing_LatinOnlyEmitsNothing(t *testing.T) {
	findings, err := DryRender(unicodeBarRequest("Ünïcödé", "Café", "Straße"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Code == FindingGlyphMissing && strings.Contains(f.Message, "Ü") {
			t.Errorf("unexpected finding for Latin text: %s", f.Message)
		}
	}
}

// A Japanese label mixing Han and Katakana was emitted as two tspans with
// their own x; under text-anchor="middle" the second was drawn LEFT of the
// first. It must be one tspan holding the whole logical string.
func TestCJKLabelIsOneTspan(t *testing.T) {
	doc, err := Render(unicodeBarRequest("日本語テキスト", "한국어 텍스트", "中文 mixed Latin"))
	if err != nil {
		t.Fatal(err)
	}
	svg := string(doc.Content)
	for _, label := range []string{"日本語テキスト", "한국어 텍스트", "中文 mixed Latin"} {
		re := regexp.MustCompile(`<text[^>]*>((?:<tspan[^>]*>[^<]*</tspan>)+)</text>`)
		found := false
		for _, m := range re.FindAllStringSubmatch(svg, -1) {
			if !strings.Contains(stripTags(m[1]), label) {
				continue
			}
			found = true
			if n := strings.Count(m[1], "<tspan"); n != 1 {
				t.Errorf("label %q emitted as %d tspans: %s", label, n, m[1])
			}
		}
		if !found {
			t.Errorf("label %q not emitted as a contiguous string", label)
		}
	}
}

// direction:rtl on a tspan makes text-anchor="start" mean the RIGHT edge, so
// left-aligned RTL text hung outside its box. It must not be emitted.
func TestRTLLabelsCarryNoDirectionOverride(t *testing.T) {
	doc, err := Render(unicodeBarRequest("שלום עולם", "مرحبا بالعالم", "plain"))
	if err != nil {
		t.Fatal(err)
	}
	svg := string(doc.Content)
	if strings.Contains(svg, "direction:rtl") {
		t.Error("SVG still carries direction:rtl")
	}
	for _, label := range []string{"שלום עולם", "مرحبا بالعالم"} {
		if !strings.Contains(svg, label) {
			t.Errorf("RTL label %q not emitted in logical order", label)
		}
	}
}

// Missing glyphs measured as .notdef advances; wide characters must now
// measure about one em each.
func TestMeasureTextEstimatesMissingWideGlyphs(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	b.SetFontSize(20)
	face, err := b.safeFace(colorToRGBA(b.StyleGuide().Palette.TextPrimary))
	if err != nil {
		t.Fatal(err)
	}
	const cjk = "日本語テキスト"
	if len(missingGlyphRunes(face, cjk)) != len([]rune(cjk)) {
		t.Skip("embedded chart font has CJK glyphs")
	}
	w, _ := b.MeasureText(cjk)
	want := float64(len([]rune(cjk))) * 20 * missingGlyphWideEm
	if math.Abs(w-want) > 0.5 {
		t.Errorf("MeasureText(%q) = %.2f, want ~%.2f (one em per wide glyph)", cjk, w, want)
	}
	// Mixed: the Latin part is measured, the CJK part estimated.
	wLatin, _ := b.MeasureText("abc")
	wMixed, _ := b.MeasureText("abc日本")
	if math.Abs(wMixed-(wLatin+2*20)) > 0.5 {
		t.Errorf("MeasureText(mixed) = %.2f, want %.2f", wMixed, wLatin+40)
	}
}

func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}
