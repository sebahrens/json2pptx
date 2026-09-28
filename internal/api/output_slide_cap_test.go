package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/template"
)

// go-slide-creator-tcxsq: max_slides_per_request only counted input slides,
// so one slide with 10,000 bullets paginated into ~2,500 output slides. The
// cap now applies after pagination too.
func TestConvertCapsPaginatedOutputSlides(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := tempDir + "/templates"
	outputDir := tempDir + "/output"
	for _, d := range []string{templatesDir, outputDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tpl, err := os.ReadFile("../../templates/midnight-blue.pptx")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatesDir+"/test-template.pptx", tpl, 0o644); err != nil {
		t.Fatal(err)
	}
	templateService := NewTemplateService(templatesDir, template.NewMemoryCache(24*60*60), false)
	service := NewConvertService(templatesDir, outputDir, templateService, pipeline.NewPipeline())
	service.SetLimits(3, 0)

	bullets := make([]string, 200)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("Point %d", i+1)
	}
	body, _ := json.Marshal(ConvertRequest{
		Template: "test-template",
		Slides:   []APISlide{{Type: "content", Title: "Everything", Content: APIContent{Bullets: bullets}}},
		Options:  &ConvertOptions{OutputFormat: "file"},
	})
	w := httptest.NewRecorder()
	service.ConvertHandler()(w, httptest.NewRequest(http.MethodPost, "/api/v1/convert", bytes.NewReader(body)))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "paginates into") {
		t.Fatalf("want 400 naming the paginated slide count, got %d: %s", w.Code, w.Body.String())
	}
}
