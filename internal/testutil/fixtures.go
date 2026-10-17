package testutil

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
)

// RequireRenderTools skips optional local rendering, but fails CI when its
// advertised toolchain is unavailable. Short mode never invokes external tools.
func RequireRenderTools(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, missing := render.DependencyStatus(); !ok {
		if os.Getenv("CI") != "" {
			t.Fatalf("required render tools unavailable in CI: %v", missing)
		}
		t.Skipf("optional local render tools unavailable: %v", missing)
	}
}

// StandardTemplatePath returns the tracked fixture; fixture loss is a failure.
func StandardTemplatePath(t testing.TB) string {
	t.Helper()
	path := filepath.Join(RepoRoot(), "internal", "template", "testdata", "standard.pptx")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("required standard template: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("required standard template is not a regular file: %s", path)
	}
	return path
}

// WriteTestPNG writes a transparent PNG with known dimensions.
func WriteTestPNG(t testing.TB, path string, width, height int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
