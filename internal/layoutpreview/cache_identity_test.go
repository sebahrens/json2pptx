package layoutpreview

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestPreviewCacheIdentityTracksRenderingInputs(t *testing.T) {
	analysis := &types.TemplateAnalysis{Layouts: []types.LayoutMetadata{{ID: "one", Name: "One Content", Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderBody}}}}}
	key := func(templateHash, engine, office, raster string, a *types.TemplateAnalysis, dpi int) string {
		t.Helper()
		got, err := previewCacheIdentity(templateHash, engine, office, raster, a, dpi)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	base := key("template", "engine", "office", "raster", analysis, 96)
	if base != key("template", "engine", "office", "raster", analysis, (&Options{}).dpi()) || base != key("template", "engine", "office", "raster", analysis, (*Options)(nil).dpi()) {
		t.Fatal("equivalent effective DPI did not reuse cache identity")
	}
	changed := &types.TemplateAnalysis{Layouts: []types.LayoutMetadata{{ID: "one", Name: "Section Divider", CanonicalType: types.CanonicalLayoutSectionDivider, Placeholders: analysis.Layouts[0].Placeholders}}}
	for name, value := range map[string]string{
		"template":  key("different", "engine", "office", "raster", analysis, 96),
		"generator": key("template", "rebuilt", "office", "raster", analysis, 96),
		"office":    key("template", "engine", "upgraded", "raster", analysis, 96),
		"raster":    key("template", "engine", "office", "upgraded", analysis, 96),
		"dpi":       key("template", "engine", "office", "raster", analysis, 192),
		"recipe":    key("template", "engine", "office", "raster", changed, 96),
	} {
		if value == base {
			t.Errorf("%s change reused stale identity", name)
		}
	}
}

func TestPreviewToolIdentityFailsClosed(t *testing.T) {
	if _, err := previewToolIdentity(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing renderer accepted")
	}
	if _, err := currentPreviewCacheIdentity(filepath.Join(t.TempDir(), "missing.pptx"), &types.TemplateAnalysis{}, nil); err == nil || !strings.Contains(err.Error(), "hash template") {
		t.Fatalf("missing template not reported: %v", err)
	}
	path := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(path, []byte("not a renderer"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := previewToolIdentity(path); err == nil {
		t.Fatal("unexecutable renderer accepted")
	}
	identity, err := previewEngineIdentity()
	if err != nil || len(identity) != 64 {
		t.Fatalf("current executable fingerprint unavailable: %q %v", identity, err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want, err := fileHash(executable)
	if err != nil || identity != want {
		t.Fatal("engine identity does not fingerprint actual executable bytes")
	}
}

func TestPreviewToolIdentityTracksWrapperTargetVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX renderer wrapper fixture")
	}
	path := filepath.Join(t.TempDir(), "renderer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nread version < \"$0.version\"\necho \"$version\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".version", []byte("renderer1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := previewToolIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".version", []byte("renderer2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := previewToolIdentity(path)
	if err != nil || first == second {
		t.Fatalf("unchanged wrapper concealed renderer upgrade: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := previewToolIdentity(path); err == nil {
		t.Fatal("failed renderer-version probe accepted")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := previewToolIdentity(path); err == nil {
		t.Fatal("empty renderer version accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := previewRendererVersion(ctx, path); err == nil {
		t.Fatal("cancelled version probe accepted")
	}
}
