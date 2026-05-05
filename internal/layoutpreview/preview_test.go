package layoutpreview

import (
	"os"
	"os/exec"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGenerate(t *testing.T) {
	templatePath := "../../templates/midnight-blue.pptx"
	if _, err := os.Stat(templatePath); err != nil {
		t.Skip("template not found")
	}
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}
	if !hasImageMagick() {
		t.Skip("imagemagick not available")
	}

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("parse layouts: %v", err)
	}
	theme := template.ParseTheme(reader)
	_ = reader.Close()

	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Layouts:      layouts,
		Theme:        theme,
	}

	tmpDir := t.TempDir()
	opts := &Options{CacheDir: tmpDir, DPI: 72}

	result, err := Generate(templatePath, analysis, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil — no previews generated")
	}

	t.Logf("Generated %d previews out of %d layouts", len(result.Paths), len(analysis.Layouts))
	for id, path := range result.Paths {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("layout %s: file not found at %s", id, path)
			continue
		}
		t.Logf("  %s: %d bytes", id, info.Size())
	}

	if len(result.Paths) == 0 {
		t.Error("expected at least one preview PNG")
	}
}

func TestGenerateCache(t *testing.T) {
	templatePath := "../../templates/midnight-blue.pptx"
	if _, err := os.Stat(templatePath); err != nil {
		t.Skip("template not found")
	}
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}
	if !hasImageMagick() {
		t.Skip("imagemagick not available")
	}

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("parse layouts: %v", err)
	}
	theme := template.ParseTheme(reader)
	_ = reader.Close()

	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Layouts:      layouts,
		Theme:        theme,
	}

	tmpDir := t.TempDir()
	opts := &Options{CacheDir: tmpDir, DPI: 72}

	// First call generates
	result1, err := Generate(templatePath, analysis, opts)
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}
	if result1 == nil {
		t.Fatal("first result is nil")
	}

	// Second call should hit cache
	result2, err := Generate(templatePath, analysis, opts)
	if err != nil {
		t.Fatalf("Generate (cached): %v", err)
	}
	if result2 == nil {
		t.Fatal("cached result is nil")
	}

	if len(result2.Paths) != len(result1.Paths) {
		t.Errorf("cache mismatch: first=%d, second=%d", len(result1.Paths), len(result2.Paths))
	}
}

func TestGenerateNoLibreOffice(t *testing.T) {
	// When LibreOffice is not on PATH, Generate should return nil gracefully
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{{ID: "slideLayout1", Name: "Title"}},
	}

	// We can't easily test this without removing LibreOffice from PATH,
	// so just verify the function signature and nil return behavior.
	if !hasLibreOffice() {
		result, err := Generate("nonexistent.pptx", analysis, nil)
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result when LibreOffice unavailable")
		}
	}
}
