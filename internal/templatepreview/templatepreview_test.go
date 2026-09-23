package templatepreview

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/templates"
)

// TestPreviewsShipForAllBundledTemplates verifies shipped previews are embedded
// and optional local test-template previews are valid on disk when available.
// Regenerate with `make template-previews`.
func TestPreviewsShipForAllBundledTemplates(t *testing.T) {
	templatesDir := filepath.Join("..", "..", "templates")
	paths := testutil.TestTemplatePaths()
	if len(paths) == 0 {
		t.Fatal("no bundled templates")
	}
	builtin := make(map[string]bool)
	for _, name := range testutil.AllBuiltinTemplateNames() {
		builtin[name] = true
	}
	for _, tplPath := range paths {
		name := strings.TrimSuffix(filepath.Base(tplPath), ".pptx")
		r, err := template.OpenTemplate(tplPath)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		layouts, err := template.ParseLayouts(r)
		_ = r.Close()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := map[string]bool{}
		for _, l := range layouts {
			want[l.ID+".png"] = true
			rel := RelPath(name, l.ID)
			f, err := os.Open(filepath.Join(templatesDir, rel)) //nolint:gosec // test path
			if err != nil {
				t.Errorf("%s/%s: missing preview (run make template-previews): %v", name, l.ID, err)
				continue
			}
			cfg, err := png.DecodeConfig(f)
			_ = f.Close()
			if err != nil || cfg.Width != DefaultWidth {
				t.Errorf("%s/%s: preview width %d (err %v), want %d", name, l.ID, cfg.Width, err, DefaultWidth)
			}
			if builtin[name] {
				if _, err := templates.Embedded.ReadFile(filepath.ToSlash(rel)); err != nil {
					t.Errorf("%s/%s: preview not embedded: %v", name, l.ID, err)
				}
			} else if _, err := templates.Embedded.ReadFile(filepath.ToSlash(rel)); err == nil {
				t.Errorf("%s/%s: local preview must not be embedded", name, l.ID)
			}
			if got := Resolve(tplPath, l.ID); got == "" || !filepath.IsAbs(got) {
				t.Errorf("%s/%s: Resolve = %q, want absolute path", name, l.ID, got)
			}
		}
		entries, _ := os.ReadDir(filepath.Join(templatesDir, DirName, name))
		for _, e := range entries {
			if !want[e.Name()] {
				t.Errorf("%s: stale preview %s has no layout", name, e.Name())
			}
		}
	}
}

func TestDownscalePNG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	f, err := os.Create(src) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	dst := filepath.Join(dir, "dst.png")
	if err := downscalePNG(src, dst, 320); err != nil {
		t.Fatal(err)
	}
	out, err := os.Open(dst) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	cfg, err := png.DecodeConfig(out)
	if err != nil || cfg.Width != 320 || cfg.Height != 180 {
		t.Fatalf("downscaled to %dx%d (err %v), want 320x180", cfg.Width, cfg.Height, err)
	}
	if err := downscalePNG(filepath.Join(dir, "missing.png"), dst, 320); err == nil {
		t.Error("missing source must error")
	}
}

// TestResolveEmbeddedFallback verifies a template resolved outside the
// templates directory (as embedded templates are, via a temp file) still gets
// its preview from the embedded copy.
func TestResolveEmbeddedFallback(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "midnight-blue.pptx")
	got := Resolve(tmp, "slideLayout2")
	if got == "" {
		t.Fatal("expected embedded preview to be materialised")
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("materialised preview missing: %v", err)
	}
	if Resolve(tmp, "slideLayout999") != "" || Resolve("", "slideLayout2") != "" {
		t.Error("unknown layouts / empty paths must resolve to empty")
	}
}
