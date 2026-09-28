package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// go-slide-creator-7oz3c: a bidi override followed by an emoji and a
// combining mark panicked the HarfBuzz shaper inside text measurement, so any
// measured text field crashed validate, generate and the MCP server. The
// prefixes are built from code points so no invisible character sits in this
// file.
var (
	rloEmojiPrefix = string([]rune{0x202E, 0x1F600, 0x0301}) // stripped at the boundary
	// rloMarkPrefix also panics the shaper but carries no emoji, so the deck
	// passes the no-emoji policy and reaches the fit report.
	rloMarkPrefix = string([]rune{0x202E, 0x00E9, 0x0301})
	// rlmCheckPrefix is NOT stripped (U+200F is legitimate in RTL prose) and
	// contains no emoji, so it reaches measurement AND rendering: only the
	// recover() guard in internal/textfit keeps it from panicking.
	rlmCheckPrefix = string([]rune{0x200F, 0x2713, 0x0301})
)

// controlCharDeck puts prefix in every measured text field kind: title,
// subtitle, body text, bullets and a shape_grid cell.
func controlCharDeck(t *testing.T, prefix string) string {
	t.Helper()
	txt := func(s string) string { b, _ := json.Marshal(prefix + s); return string(b) }
	return `{"template":"midnight-blue","slides":[
	 {"slide_type":"title","content":[
	   {"placeholder_id":"title","type":"text","text_value":` + txt("Q3 Strategy Review") + `},
	   {"placeholder_id":"subtitle","type":"text","text_value":` + txt("Board update") + `}]},
	 {"slide_type":"content","content":[
	   {"placeholder_id":"title","type":"text","text_value":"Revenue grew on enterprise renewals"},
	   {"placeholder_id":"body","type":"text","text_value":` + txt("Renewals drove most of the quarter's growth across regions") + `}]},
	 {"slide_type":"content","content":[
	   {"placeholder_id":"title","type":"text","text_value":"Three levers explain the margin gain"},
	   {"placeholder_id":"body","type":"bullets","bullets_value":[` + txt("Pricing discipline") + `,` + txt("Mix shift to services") + `]}]},
	 {"slide_type":"blank","content":[
	   {"placeholder_id":"title","type":"text","text_value":"Four phases take the program to value"}],
	  "shape_grid":{"columns":2,"rows":[{"cells":[
	   {"shape":{"geometry":"rect","fill":"accent1","text":{"content":` + txt("Discover") + `}}},
	   {"shape":{"geometry":"rect","fill":"accent1","text":{"content":"Deliver"}}}]}]}}
	]}`
}

func TestControlCharsStrippedAtDecodeWithFinding(t *testing.T) {
	var input PresentationInput
	if err := json.Unmarshal([]byte(controlCharDeck(t, rloEmojiPrefix)), &input); err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"/slides/0/content/0/text_value",
		"/slides/0/content/1/text_value",
		"/slides/1/content/1/text_value",
		"/slides/2/content/1/bullets_value/0",
		"/slides/2/content/1/bullets_value/1",
		"/slides/3/shape_grid/rows/0/cells/0/shape/text/content",
	}
	var got []string
	for _, s := range input.ControlCharSites {
		got = append(got, s.Path)
	}
	if strings.Join(got, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("sites = %v\nwant %v", got, wantPaths)
	}
	if tv := *input.Slides[0].Content[0].TextValue; strings.ContainsRune(tv, 0x202E) {
		t.Errorf("RLO survived decode: %+q", tv)
	}

	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(controlCharDeck(t, rloMarkPrefix)), "fit_report": true,
	}))
	if err != nil || result == nil {
		t.Fatalf("validate_input failed: %v %v", err, result)
	}
	text := textContent(result)
	if strings.Contains(text, "PANIC") || strings.Contains(text, "RENDER.INTERNAL") {
		t.Fatalf("validate_input reported a crash: %s", text)
	}
	for _, p := range wantPaths {
		if !strings.Contains(text, p) {
			t.Errorf("validate_input response lacks %s finding at %s", patterns.ErrCodeInputControlCharsRemoved, p)
		}
	}
	if !strings.Contains(text, patterns.ErrCodeInputControlCharsRemoved) {
		t.Errorf("validate_input response lacks %s: %s", patterns.ErrCodeInputControlCharsRemoved, text)
	}
}

// The RLM variant is kept at the boundary, so it exercises the measurement
// guard through the whole validate + generate pipeline.
func TestShaperPanicTextSurvivesValidateAndGenerate(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	deck := mustParseJSON(controlCharDeck(t, rlmCheckPrefix))

	validated, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": true}))
	if err != nil || validated == nil {
		t.Fatalf("validate_input failed: %v", err)
	}
	if text := textContent(validated); strings.Contains(text, "PANIC") || strings.Contains(text, "RENDER.INTERNAL") {
		t.Fatalf("validate_input reported a crash: %s", text)
	}

	generated, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck, "strict_fit": "off"}))
	if err != nil || generated == nil {
		t.Fatalf("generate failed: %v", err)
	}
	if text := textContent(generated); generated.IsError || strings.Contains(text, "PANIC") {
		t.Fatalf("generate reported an error: %s", text)
	}
}
