package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
)

func TestExportDeckPDFWithoutImageMagick(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake LibreOffice uses POSIX shell")
	}
	binDir := t.TempDir()
	stub := `#!/bin/sh
outdir=""
input=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --outdir) outdir="$2"; shift 2 ;;
    *.pptx) input="$1"; shift ;;
    *) shift ;;
  esac
done
base="${input##*/}"
base="${base%.pptx}"
printf '%%PDF-1.4 stub' > "$outdir/$base.pdf"
`
	if err := os.WriteFile(filepath.Join(binDir, "soffice"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir) // no ImageMagick: PDF export must not need it
	pptxPath := filepath.Join(t.TempDir(), "review.pptx")
	if err := os.WriteFile(pptxPath, []byte("dummy deck"), 0o600); err != nil {
		t.Fatal(err)
	}
	mc := cliMCPConfig("../../templates", t.TempDir())
	result, err := mc.handleExportDeck(context.Background(), makeRequest(map[string]any{"pptx_path": pptxPath, "format": "pdf"}))
	if err != nil || result == nil || result.IsError {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	var out exportDeckResult
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if out.Format != "pdf" || !filepath.IsAbs(out.OutputPath) || !strings.HasPrefix(string(data), "%PDF-") || out.Bytes != int64(len(data)) {
		t.Fatalf("export = %+v, contents = %q", out, data)
	}
}

func TestExportDeckNotesFromGeneratedPresentation(t *testing.T) {
	mc := cliMCPConfig("../../templates", t.TempDir())
	deckJSON := `{"template":"midnight-blue","slides":[
      {"slide_type":"title","speaker_notes":"Open with the context.","content":[{"placeholder_id":"title","type":"text","text_value":"Opening"}]},
      {"slide_type":"content","speaker_notes":"Explain the evidence.","content":[{"placeholder_id":"title","type":"text","text_value":"Evidence"},{"placeholder_id":"body","type":"bullets","bullets_value":["First point"]}]},
      {"slide_type":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Closing"}]}]}`
	gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(deckJSON)}))
	if err != nil || gen == nil || gen.IsError {
		t.Fatalf("generate = %+v, err = %v", gen, err)
	}
	var generated JSONOutput
	if err := json.Unmarshal([]byte(textContent(gen)), &generated); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir()) // notes extraction must work without render binaries
	result, err := mc.handleExportDeck(context.Background(), makeRequest(map[string]any{"pptx_path": generated.OutputPath, "format": "notes"}))
	if err != nil || result == nil || result.IsError {
		t.Fatalf("export = %+v, err = %v", result, err)
	}
	var out exportDeckResult
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"## Slide 1 — Opening", "Open with the context.", "## Slide 2 — Evidence", "Explain the evidence.", "## Slide 3 — Closing", "(No speaker notes)"} {
		if !strings.Contains(text, want) {
			t.Errorf("handout missing %q:\n%s", want, text)
		}
	}
	if out.Format != "notes" || out.SlideCount != 3 || out.Bytes != int64(len(data)) {
		t.Fatalf("export = %+v", out)
	}
}

func TestExportDeckPDFIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("real LibreOffice conversion")
	}
	if _, err := render.OfficeCommand(); err != nil {
		t.Skip("LibreOffice is unavailable")
	}
	mc := cliMCPConfig("../../templates", t.TempDir())
	deckJSON := `{"template":"midnight-blue","slides":[{"slide_type":"title","content":[{"placeholder_id":"title","type":"text","text_value":"PDF Review"}]}]}`
	gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(deckJSON)}))
	if err != nil || gen == nil || gen.IsError {
		t.Fatalf("generate = %+v, err = %v", gen, err)
	}
	var generated JSONOutput
	if err := json.Unmarshal([]byte(textContent(gen)), &generated); err != nil {
		t.Fatal(err)
	}
	result, err := mc.handleExportDeck(context.Background(), makeRequest(map[string]any{"pptx_path": generated.OutputPath, "format": "pdf"}))
	if err != nil || result == nil || result.IsError {
		t.Fatalf("export = %+v, err = %v", result, err)
	}
	var out exportDeckResult
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 100 || !strings.HasPrefix(string(data), "%PDF-") {
		t.Fatalf("export is not a real PDF: path=%q, bytes=%d", out.OutputPath, len(data))
	}
}
