package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/templatepreview"
)

// TestEmbeddedTemplateKeepsItsName is go-slide-creator-qbks: an embedded
// template was materialised as json2pptx-template-356087472.pptx, so everything
// downstream that reads a template's identity off its base name saw a template
// called "json2pptx-template-356087472" — and the layout previews, which are
// keyed by template name, resolved to nothing on a server started without
// --templates-dir.
func TestEmbeddedTemplateKeepsItsName(t *testing.T) {
	path, cleanup, err := resolveTemplatePath("midnight-blue", filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("resolve embedded template: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	if got := filepath.Base(path); got != "midnight-blue.pptx" {
		t.Errorf("materialised as %q, want midnight-blue.pptx — the name is the template's identity", got)
	}
	if !strings.Contains(path, "json2pptx-template-") {
		t.Errorf("path %q should still live in a uniquely named temp dir", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("materialised template is not readable: %v", err)
	}

	// The whole point: the embedded layout preview now resolves.
	preview := templatepreview.Resolve(path, "slideLayout2")
	if preview == "" {
		t.Fatal("no layout preview resolved for an embedded template")
	}
	if _, err := os.Stat(preview); err != nil {
		t.Errorf("preview path %q does not exist: %v", preview, err)
	}
}

// TestEmbeddedTemplateCleanupRemovesTheDir: the temp dir must not leak.
func TestEmbeddedTemplateCleanupRemovesTheDir(t *testing.T) {
	path, cleanup, err := resolveTemplatePath("forest-green", filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cleanup == nil {
		t.Fatal("an extracted template must come with a cleanup")
	}
	cleanup()
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("temp dir %q survived cleanup (err=%v)", filepath.Dir(path), err)
	}
}

// TestOnDiskTemplateIsUnchanged: a template found on disk is returned as-is,
// with no cleanup to run.
func TestOnDiskTemplateIsUnchanged(t *testing.T) {
	path, cleanup, err := resolveTemplatePath("midnight-blue", filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatalf("resolve on-disk template: %v", err)
	}
	if cleanup != nil {
		cleanup()
	}
	if !strings.HasSuffix(path, filepath.Join("templates", "midnight-blue.pptx")) {
		t.Errorf("on-disk template resolved to %q", path)
	}
}
