package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraphicsAuditExplicitTemplateDirectory(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.Mkdir("templates", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("templates", "local-only.pptx"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), userTemplatesDir), 0700); err != nil {
		t.Fatal(err)
	}
	explicit := t.TempDir()
	t.Setenv(envTemplatesDir, "")
	t.Setenv(envTemplatesDirLegacy, "")
	if _, cleanup, err := resolveTemplatePath("local-only", explicit); err == nil {
		cleanup()
		t.Fatal("explicit directory leaked cwd template")
	}
	if path, cleanup, err := resolveTemplatePath("local-only", ""); err != nil {
		t.Fatalf("unconfigured discovery lost cwd fallback: %v", err)
	} else {
		defer cleanup()
		if filepath.Base(path) != "local-only.pptx" {
			t.Fatal(path)
		}
	}
	t.Setenv(envTemplatesDir, explicit)
	if _, cleanup, err := resolveTemplatePath("local-only", ""); err == nil {
		cleanup()
		t.Fatal("environment directory leaked cwd template")
	}
	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv(envTemplatesDir, missing)
	if got, _ := resolveTemplatesDir(""); got != missing {
		t.Fatalf("missing configured directory silently fell back to %s", got)
	}
}
