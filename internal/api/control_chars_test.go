package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/template"
)

// go-slide-creator-7oz3c: a bidi control + symbol + combining mark panicked
// the text shaper, so /api/v1/convert answered 500 PANIC_RECOVERED. Built from
// code points so no invisible character sits in this file.
func TestConvertSurvivesShaperPanicText(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	for _, d := range []string{templatesDir, outputDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// A shipped template: the SimpleMinimal fixture the older convert tests
	// use is not checked in, so they skip.
	tpl, err := os.ReadFile("../../templates/midnight-blue.pptx")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatesDir+"/test-template.pptx", tpl, 0o644); err != nil {
		t.Fatal(err)
	}
	templateService := NewTemplateService(templatesDir, template.NewMemoryCache(24*60*60), false)
	service := NewConvertService(templatesDir, outputDir, templateService, pipeline.NewPipeline())

	rlo := string([]rune{0x202E, 0x00E9, 0x0301}) // stripped at the boundary
	rlm := string([]rune{0x200F, 0x2713, 0x0301}) // kept; only the measurement guard saves it
	reqBody := ConvertRequest{
		Template: "test-template",
		Slides: []APISlide{
			{Type: "title", Title: rlo + "Q3 Strategy Review"},
			{Type: "content", Title: rlm + "Revenue", Content: APIContent{Body: rlo + "Renewals drove growth."}},
			{Type: "content", Title: "Levers", Content: APIContent{Bullets: []string{rlo + "Pricing", rlm + "Mix"}}},
		},
		Options: &ConvertOptions{OutputFormat: "file"},
	}
	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	service.ConvertHandler()(w, httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp ConvertResponseFile
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	var n int
	for _, warn := range resp.Stats.Warnings {
		if strings.HasPrefix(warn, "INPUT_CONTROL_CHARS_REMOVED") {
			n++
		}
	}
	if n != 3 {
		t.Errorf("want 3 INPUT_CONTROL_CHARS_REMOVED warnings (title, body, bullet), got %d: %v", n, resp.Stats.Warnings)
	}
}
