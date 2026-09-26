package templatepreview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewManifestRejectsStaleOrMismatchedEvidence(t *testing.T) {
	png := []byte("known PNG bytes")
	base := previewManifest{Schema: 1, Recipe: previewRecipe, TemplateHash: "template", Images: map[string]string{"one": hashBytes(png)}}
	encode := func(m previewManifest) []byte {
		t.Helper()
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if !validManifest(encode(base), png, "template", "one") {
		t.Fatal("valid provenance rejected")
	}
	for name, mutate := range map[string]func(*previewManifest){
		"schema":    func(m *previewManifest) { m.Schema++ },
		"recipe":    func(m *previewManifest) { m.Recipe = "old recipe" },
		"template":  func(m *previewManifest) { m.TemplateHash = "old template" },
		"image":     func(m *previewManifest) { m.Images = map[string]string{"one": "wrong pixels"} },
		"inventory": func(m *previewManifest) { m.Images = map[string]string{} },
	} {
		t.Run(name, func(t *testing.T) {
			m := base
			mutate(&m)
			if validManifest(encode(m), png, "template", "one") {
				t.Fatal("stale manifest accepted")
			}
		})
	}
	if validManifest([]byte("broken"), png, "template", "one") || validManifest(encode(base), []byte("corrupted"), "template", "one") {
		t.Fatal("corrupt metadata or PNG accepted")
	}
}

func TestRenderingSourceHashTracksCodeNotGeneratedAssets(t *testing.T) {
	root := t.TempDir()
	for path, data := range map[string]string{
		"internal/templatepreview/recipe.go": "package recipe\n",
		"svggen/go.mod":                      "module diagram\n",
		"cmd/templatepreviews/main.go":       "package main\n",
		"go.mod":                             "module fixture\n", "go.sum": "dependency\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := RenderingSourceHash(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"internal/templatepreview/recipe_test.go", "internal/templatepreview/generated.png"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte("not rendering source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	same, err := RenderingSourceHash(root)
	if err != nil || same != before {
		t.Fatal("tests/generated images caused circular provenance")
	}
	meta, err := json.Marshal(previewManifest{SourceHash: before})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "templates", "fixture.pptx")
	if !previewSourcesAreCurrent(path, meta) {
		t.Fatal("current source provenance rejected")
	}
	if err := os.WriteFile(filepath.Join(root, "internal/templatepreview/recipe.go"), []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := RenderingSourceHash(root)
	if err != nil || before == changed || previewSourcesAreCurrent(path, meta) {
		t.Fatal("rendering source change did not invalidate preview provenance")
	}
	if _, err := RenderingSourceHash(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing source inventory accepted")
	}
}
