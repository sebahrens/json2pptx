package fontcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tdewolff/canvas"
)

func TestMetricClone(t *testing.T) {
	for name, want := range map[string]string{"Calibri": "Carlito", " cambria ": "Caladea", "Gill Sans": "", "Arial": ""} {
		if got := MetricClone(name); got != want {
			t.Errorf("MetricClone(%q) = %q, want %q", name, got, want)
		}
	}
}

// go-slide-creator-tl7vf: Calibri resolves to Carlito (not a substitution)
// when the real Calibri is absent and Carlito is present.
func TestResolveCalibriToCarlitoWhenPresent(t *testing.T) {
	if err := canvas.NewFontFamily("probe").LoadSystemFont("Calibri", canvas.FontRegular); err == nil {
		t.Skip("real Calibri installed; the clone is not needed")
	}
	present := false
	if err := canvas.NewFontFamily("probe").LoadSystemFont("Carlito", canvas.FontRegular); err == nil {
		present = true
	}
	for _, dir := range cloneFontDirs() {
		if _, err := os.Stat(filepath.Join(dir, "Carlito-Regular.ttf")); err == nil {
			present = true
		}
	}
	if !present {
		t.Skip("Carlito not installed (no LibreOffice / crosextra fonts)")
	}
	Reset()
	defer Reset()
	ff, resolved, substituted := Resolve("Calibri", "Arial")
	if ff == nil {
		t.Fatal("Calibri did not resolve")
	}
	if resolved != "Carlito" || substituted {
		t.Errorf("Resolve(Calibri) = %q substituted=%v, want Carlito substituted=false", resolved, substituted)
	}
}

// A clone found only in a font directory is loaded from the file.
func TestLoadMetricCloneFromDirectory(t *testing.T) {
	src := ""
	for _, dir := range cloneFontDirs() {
		p := filepath.Join(dir, "Carlito-Regular.ttf")
		if _, err := os.Stat(p); err == nil {
			src = p
			break
		}
	}
	if src == "" {
		t.Skip("no Carlito file available to copy")
	}
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Caladea-Regular.ttf"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	orig := cloneFontDirs
	cloneFontDirs = func() []string { return []string{dir} }
	defer func() { cloneFontDirs = orig }()
	if err := canvas.NewFontFamily("probe").LoadSystemFont("Caladea", canvas.FontRegular); err == nil {
		t.Skip("Caladea registered as a system font")
	}
	ff := canvas.NewFontFamily("Cambria")
	if clone, ok := loadMetricClone(ff, "Cambria"); !ok || clone != "Caladea" {
		t.Errorf("loadMetricClone(Cambria) = %q, %v", clone, ok)
	}
}
