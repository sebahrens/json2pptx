package templatepreview

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
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

// TestRenderingPackageDirsMatchImports keeps the hashed package allowlist equal
// to cmd/templatepreviews' real in-repo import set, so a new rendering
// dependency cannot escape provenance (go-slide-creator-csclk.1).
func TestRenderingPackageDirsMatchImports(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}
	root := testutil.RepoRoot()
	cmd := exec.Command(goBin, "list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}", "./cmd/templatepreviews") //nolint:gosec // fixed arguments
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var got []string
	for _, dir := range strings.Fields(string(out)) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue // third-party module
		}
		got = append(got, filepath.ToSlash(rel))
	}
	sort.Strings(got)
	want := append([]string(nil), RenderingPackageDirs...)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("RenderingPackageDirs out of sync with `go list -deps ./cmd/templatepreviews`:\n got  %v\n want %v", got, want)
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
	for _, path := range []string{"internal/templatepreview/recipe_test.go", "internal/templatepreview/generated.png", "internal/api/unrelated.go"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte("not rendering source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	same, err := RenderingSourceHash(root)
	if err != nil || same != before {
		t.Fatal("tests, generated images or a non-rendering package changed provenance")
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
