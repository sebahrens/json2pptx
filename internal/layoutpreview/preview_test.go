package layoutpreview

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestSampleContentKeepsSectionDividerPreviewCompact(t *testing.T) {
	layout := types.LayoutMetadata{
		Name:          "Section Divider",
		CanonicalType: types.CanonicalLayoutSectionDivider,
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle},
			{ID: "Section Number", Type: types.PlaceholderBody, Role: types.PlaceholderRoleSectionNumber},
			{ID: "body", Type: types.PlaceholderBody},
		},
	}
	want := []generator.ContentItem{
		{PlaceholderID: "title", Type: generator.ContentText, Value: "Section"},
		{PlaceholderID: "Section Number", Type: generator.ContentText, Value: "01"},
		{PlaceholderID: "body", Type: generator.ContentText, Value: "Overview"},
	}
	if got := sampleContent(layout); !reflect.DeepEqual(got, want) {
		t.Errorf("section preview content = %#v, want %#v", got, want)
	}
	ordinary := types.LayoutMetadata{Name: "One Content", Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderBody}}}
	got := sampleContent(ordinary)
	if len(got) != 1 || got[0].Type != generator.ContentBullets {
		t.Errorf("ordinary content preview lost sample bullets: %#v", got)
	}
}

func TestGenerate(t *testing.T) {
	templatePath := "../../templates/midnight-blue.pptx"
	if _, err := os.Stat(templatePath); err != nil {
		t.Skip("template not found")
	}
	if !hasLibreOffice() {
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
	opts := &Options{CacheDir: tmpDir, DPI: 72, LibreOfficeProfileDir: t.TempDir()}

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

	if len(result.Paths) != len(analysis.Layouts) {
		t.Error("every layout must have a preview")
	}
}

func TestGenerateCache(t *testing.T) {
	templatePath := "../../templates/midnight-blue.pptx"
	if _, err := os.Stat(templatePath); err != nil {
		t.Skip("template not found")
	}
	if !hasLibreOffice() {
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
	opts := &Options{CacheDir: tmpDir, DPI: 72, LibreOfficeProfileDir: t.TempDir()}

	// First call generates
	result1, err := Generate(templatePath, analysis, opts)
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}
	if result1 == nil {
		t.Fatal("first result is nil")
	}
	timestamps := map[string]int64{}
	for id, path := range result1.Paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		timestamps[id] = info.ModTime().UnixNano()
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
	for id, path := range result2.Paths {
		info, err := os.Stat(path)
		if err != nil || path != result1.Paths[id] || info.ModTime().UnixNano() != timestamps[id] {
			t.Fatalf("identical render did not reuse unchanged preview: %s %v", id, err)
		}
	}
	// Same template, different density must produce a distinct cache and pixels
	// at the requested resolution, not reuse the old readable but wrong PNGs.
	opts.DPI = 144
	result3, err := Generate(templatePath, analysis, opts)
	if err != nil || result3 == nil {
		t.Fatalf("higher-resolution Generate: %v", err)
	}
	for id, path := range result3.Paths {
		if path == result1.Paths[id] {
			t.Fatalf("different DPI reused preview path: %s", id)
		}
		bounds := func(path string) image.Config {
			t.Helper()
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			config, err := png.DecodeConfig(f)
			if err != nil {
				t.Fatal(err)
			}
			return config
		}
		low, high := bounds(result1.Paths[id]), bounds(path)
		if high.Width != low.Width*2 || high.Height != low.Height*2 {
			t.Fatalf("%s: DPI doubled but dimensions %dx%d -> %dx%d", id, low.Width, low.Height, high.Width, high.Height)
		}
	}
}

func TestCachedPreviewsRequireEveryLayoutAndReadablePNG(t *testing.T) {
	dir := t.TempDir()
	analysis := &types.TemplateAnalysis{Layouts: []types.LayoutMetadata{{ID: "one"}, {ID: "two"}}}
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	one, two := filepath.Join(dir, "one.png"), filepath.Join(dir, "two.png")
	if err := os.WriteFile(one, body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if result, err := collectCachedPreviews(dir, analysis); err == nil || result != nil {
		t.Fatal("missing second layout accepted as complete")
	}
	if err := os.WriteFile(two, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result, err := collectCachedPreviews(dir, analysis); err == nil || result != nil {
		t.Fatal("corrupt second layout accepted as complete")
	}
	if err := os.WriteFile(two, body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if result, err := collectCachedPreviews(dir, analysis); err != nil || len(result.Paths) != 2 {
		t.Fatalf("complete set rejected: %+v %v", result, err)
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
