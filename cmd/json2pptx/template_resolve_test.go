package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDirExists(t *testing.T) {
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Errorf("expected %s to exist as dir", dir)
	}
	if dirExists(filepath.Join(dir, "missing")) {
		t.Error("expected missing dir to not exist")
	}
	// A regular file is not a directory
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirExists(file) {
		t.Error("expected file path to not be reported as dir")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(file) {
		t.Errorf("expected %s to exist as file", file)
	}
	if fileExists(filepath.Join(dir, "missing")) {
		t.Error("expected missing file to not exist")
	}
	if fileExists(dir) {
		t.Error("expected directory to not be reported as file")
	}
}

// withClearedTemplateEnv saves and restores the templates env vars so individual
// tests can run resolution without surprises from the user's shell environment.
func withClearedTemplateEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{envTemplatesDir, envTemplatesDirLegacy} {
		t.Setenv(name, "")
	}
	// Move HOME to an empty directory so user-templates lookups never hit a real path.
	t.Setenv("HOME", t.TempDir())
}

func TestResolveTemplatesDir_FlagWins(t *testing.T) {
	withClearedTemplateEnv(t)
	dir := t.TempDir()
	got, embedded := resolveTemplatesDir(dir)
	if embedded {
		t.Error("expected non-embedded resolution when flag points at real dir")
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestResolveTemplatesDir_FlagMissingReturnsAnyway(t *testing.T) {
	withClearedTemplateEnv(t)
	missing := filepath.Join(t.TempDir(), "nope")
	got, embedded := resolveTemplatesDir(missing)
	if embedded {
		t.Error("expected non-embedded resolution when explicit flag is given")
	}
	if got != missing {
		t.Errorf("got %q, want %q (caller produces a clear error from this)", got, missing)
	}
}

func TestResolveTemplatesDir_EnvVar(t *testing.T) {
	withClearedTemplateEnv(t)
	dir := t.TempDir()
	t.Setenv(envTemplatesDir, dir)
	got, embedded := resolveTemplatesDir("")
	if embedded {
		t.Error("expected non-embedded resolution from env var")
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestResolveTemplatesDir_LegacyEnvVar(t *testing.T) {
	withClearedTemplateEnv(t)
	dir := t.TempDir()
	t.Setenv(envTemplatesDirLegacy, dir)
	got, _ := resolveTemplatesDir("")
	if got != dir {
		t.Errorf("got %q, want %q from legacy env", got, dir)
	}
}

func TestResolveTemplatesDir_FallsBackToEmbedded(t *testing.T) {
	withClearedTemplateEnv(t)
	// Run from a tempdir so ./templates does not exist.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	got, embedded := resolveTemplatesDir("")
	if !embedded {
		t.Errorf("expected embedded fallback, got dir=%q", got)
	}
	if got != "" {
		t.Errorf("expected empty path for embedded, got %q", got)
	}
}

func TestLocalAppDataTemplatesDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		base := `C:\Users\test\AppData\Local`
		t.Setenv("LOCALAPPDATA", base)
		got := localAppDataTemplatesDir()
		want := filepath.Join(base, "json2pptx", "templates")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		t.Setenv("LOCALAPPDATA", "")
		if got := localAppDataTemplatesDir(); got != "" {
			t.Errorf("expected empty path when LOCALAPPDATA unset, got %q", got)
		}
		return
	}
	// On non-Windows the dotfile convention applies, so the helper is a no-op.
	if got := localAppDataTemplatesDir(); got != "" {
		t.Errorf("expected empty path on %s, got %q", runtime.GOOS, got)
	}
}

func TestResolveTemplatesDir_LocalAppDataPreferredOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("LOCALAPPDATA resolution is Windows-only")
	}
	withClearedTemplateEnv(t)
	localAppData := t.TempDir()
	winDir := filepath.Join(localAppData, "json2pptx", "templates")
	if err := os.MkdirAll(winDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", localAppData)
	got, embedded := resolveTemplatesDir("")
	if embedded {
		t.Error("expected non-embedded resolution from LOCALAPPDATA")
	}
	if got != winDir {
		t.Errorf("got %q, want %q", got, winDir)
	}
}

func TestResolveTemplatePath_FromDisk(t *testing.T) {
	withClearedTemplateEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.pptx"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := resolveTemplatePath("demo", dir)
	if err != nil {
		t.Fatalf("resolveTemplatePath: %v", err)
	}
	defer cleanup()
	if !strings.HasSuffix(path, "demo.pptx") {
		t.Errorf("path %q does not end in demo.pptx", path)
	}
}

func TestResolveTemplatePath_StripsExtension(t *testing.T) {
	withClearedTemplateEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.pptx"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := resolveTemplatePath("demo.pptx", dir)
	if err != nil {
		t.Fatalf("resolveTemplatePath: %v", err)
	}
	defer cleanup()
	if !strings.HasSuffix(path, "demo.pptx") {
		t.Errorf("expected demo.pptx path, got %q", path)
	}
}

func TestResolveTemplatePath_EmbeddedFallback(t *testing.T) {
	withClearedTemplateEnv(t)
	// Use an embedded template that ships with the binary.
	path, cleanup, err := resolveTemplatePath("midnight-blue", filepath.Join(t.TempDir(), "no-such-dir"))
	if err != nil {
		t.Fatalf("resolveTemplatePath: %v", err)
	}
	defer cleanup()
	if path == "" {
		t.Fatal("expected a real temp file path for embedded template")
	}
	if !fileExists(path) {
		t.Errorf("expected embedded template extracted at %s", path)
	}
}

func TestResolveTemplatePath_NotFound(t *testing.T) {
	withClearedTemplateEnv(t)
	if _, _, err := resolveTemplatePath("definitely-not-a-template", t.TempDir()); err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestListAvailableTemplates_IncludesEmbedded(t *testing.T) {
	withClearedTemplateEnv(t)
	names := listAvailableTemplates(t.TempDir())
	// Embedded set must include the bundled templates.
	want := map[string]bool{
		"midnight-blue":   false,
		"forest-green":    false,
		"warm-coral":      false,
		"modern-template": false,
	}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("expected %q in available templates, got %v", n, names)
		}
	}
}

func TestListAvailableTemplates_DedupesDiskAndEmbedded(t *testing.T) {
	withClearedTemplateEnv(t)
	dir := t.TempDir()
	// Create an on-disk file that shadows an embedded template name.
	if err := os.WriteFile(filepath.Join(dir, "midnight-blue.pptx"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := listAvailableTemplates(dir)
	count := 0
	for _, n := range names {
		if n == "midnight-blue" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected midnight-blue to appear exactly once, got %d in %v", count, names)
	}
}

func TestTemplateNotFoundError_ListsAvailable(t *testing.T) {
	withClearedTemplateEnv(t)
	msg := templateNotFoundError("bogus", t.TempDir())
	if !strings.Contains(msg, "bogus") {
		t.Errorf("error %q missing template name", msg)
	}
	if !strings.Contains(msg, "available templates") {
		t.Errorf("error %q missing available templates section", msg)
	}
	if !strings.Contains(msg, "midnight-blue") {
		t.Errorf("error %q does not list bundled templates", msg)
	}
}
