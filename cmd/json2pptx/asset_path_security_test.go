package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExpandAssetPath_DisallowedVarNotLeaked verifies that a non-allow-listed
// env var is never expanded, so its value cannot surface in a diagnostic
// (go-slide-creator-csclk.76).
func TestExpandAssetPath_DisallowedVarNotLeaked(t *testing.T) {
	t.Setenv("DEMO_API_TOKEN", "sk-live-SUPERSECRET-123")
	got, unset := expandAssetPath("/${DEMO_API_TOKEN}.svg")
	if unset != "DEMO_API_TOKEN" || strings.Contains(got, "SUPERSECRET") {
		t.Fatalf("expected DEMO_API_TOKEN to be refused, got out=%q unset=%q", got, unset)
	}
	icon := &IconInput{Path: "/${DEMO_API_TOKEN}.svg"}
	for _, f := range resolveIconInputPath(icon, t.TempDir(), 0, "icon") {
		if strings.Contains(f.Message, "SUPERSECRET") {
			t.Fatalf("diagnostic leaked env value: %s", f.Message)
		}
	}
	// A missing-file diagnostic must not echo the absolute base_dir.
	base := t.TempDir()
	missing := &IconInput{Path: "missing.svg"}
	for _, f := range resolveIconInputPath(missing, base, 0, "icon") {
		if strings.Contains(f.Message, base) {
			t.Fatalf("diagnostic leaked base dir: %s", f.Message)
		}
	}
}

// TestResolveIconInputPath_AllowList verifies ALLOWED_IMAGE_PATHS applies to
// icon.path (go-slide-creator-csclk.3).
func TestResolveIconInputPath_AllowList(t *testing.T) {
	allowed := t.TempDir()
	secret := t.TempDir()
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	inPath := filepath.Join(allowed, "ok.svg")
	outPath := filepath.Join(secret, "icon.svg")
	for _, p := range []string{inPath, outPath} {
		if err := os.WriteFile(p, svg, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	realAllowed, _ := filepath.EvalSymlinks(allowed)
	if f := resolveIconInputPath(&IconInput{Path: outPath}, t.TempDir(), 0, "icon", realAllowed); !anyDiagnosticError(f) {
		t.Fatalf("expected icon outside ALLOWED_IMAGE_PATHS to be refused, got %+v", f)
	}
	if f := resolveIconInputPath(&IconInput{Path: inPath}, t.TempDir(), 0, "icon", realAllowed); len(f) != 0 {
		t.Fatalf("expected icon inside ALLOWED_IMAGE_PATHS to resolve, got %+v", f)
	}
}
