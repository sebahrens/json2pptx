package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeLookPath returns a lookPath replacement that only "finds" the given
// executables (returning the name itself as the path).
func fakeLookPath(available ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, a := range available {
		set[a] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return name, nil
		}
		return "", errors.New("not found: " + name)
	}
}

// TestMain pins the simulated toolchain to the classic libreoffice + convert
// pair so the pre-existing mock-runner tests are host-independent. Tests
// that exercise other toolchains override lookPath locally.
func TestMain(m *testing.M) {
	lookPath = fakeLookPath("libreoffice", "convert")
	os.Exit(m.Run())
}

func withLookPath(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	orig := lookPath
	lookPath = fn
	t.Cleanup(func() { lookPath = orig })
}

func TestResolveToolchain_SofficeOnlyWithPdftoppm(t *testing.T) {
	withLookPath(t, fakeLookPath("soffice", "pdftoppm", "magick"))
	tc, err := resolveToolchain()
	if err != nil {
		t.Fatalf("resolveToolchain: %v", err)
	}
	if tc.office != "soffice" {
		t.Errorf("office = %q, want soffice", tc.office)
	}
	if tc.rasterKind != rasterPdftoppm || tc.raster != "pdftoppm" {
		t.Errorf("raster = %q (%d), want pdftoppm preferred over ImageMagick", tc.raster, tc.rasterKind)
	}
}

func TestResolveToolchain_MacAppBundleAndMagick(t *testing.T) {
	bundle := "/Applications/LibreOffice.app/Contents/MacOS/soffice"
	withLookPath(t, fakeLookPath(bundle, "magick"))
	tc, err := resolveToolchain()
	if err != nil {
		t.Fatalf("resolveToolchain: %v", err)
	}
	if tc.office != bundle {
		t.Errorf("office = %q, want app-bundle soffice", tc.office)
	}
	if tc.rasterKind != rasterMagick {
		t.Errorf("rasterKind = %d, want magick", tc.rasterKind)
	}
}

func TestResolveToolchain_Missing(t *testing.T) {
	withLookPath(t, fakeLookPath("pdftoppm"))
	if _, err := resolveToolchain(); err == nil || !strings.Contains(err.Error(), "LibreOffice not found") {
		t.Errorf("want LibreOffice-not-found error, got %v", err)
	}
	withLookPath(t, fakeLookPath("soffice"))
	if _, err := resolveToolchain(); err == nil || !strings.Contains(err.Error(), "rasterizer") {
		t.Errorf("want missing-rasterizer error, got %v", err)
	}
}

// TestConvert_SofficeAndPdftoppm simulates the macOS Homebrew-cask host from
// go-slide-creator-71kj: only `soffice` and poppler's `pdftoppm` exist (no
// `libreoffice`, no Ghostscript for ImageMagick).
func TestConvert_SofficeAndPdftoppm(t *testing.T) {
	withLookPath(t, fakeLookPath("soffice", "pdftoppm"))
	tmp := t.TempDir()
	pptx := filepath.Join(tmp, "deck.pptx")
	if err := os.WriteFile(pptx, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out")

	mock := &MockCommandRunner{RunFunc: func(name string, args ...string) error {
		switch name {
		case "soffice":
			return os.WriteFile(filepath.Join(out, "deck.pdf"), []byte("pdf"), 0o644)
		case "pdftoppm":
			prefix := args[len(args)-1]
			// pdftoppm zero-pads to the page-count width (12 pages -> 2 digits).
			for _, n := range []string{"01", "02", "10", "12"} {
				if err := os.WriteFile(prefix+"-"+n+".jpg", []byte("jpg"), 0o644); err != nil {
					return err
				}
			}
		}
		return nil
	}}

	if err := convertPPTXToJPGWithRunner(pptx, out, 110, mock); err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("calls = %v", mock.Calls)
	}
	if !strings.HasPrefix(mock.Calls[0], "soffice -env:UserInstallation=file://") {
		t.Errorf("office call should use soffice with a private profile: %s", mock.Calls[0])
	}
	if !strings.Contains(mock.Calls[1], "-r 110") || !strings.Contains(mock.Calls[1], "-jpeg") {
		t.Errorf("pdftoppm call missing density/jpeg flags: %s", mock.Calls[1])
	}
	for _, n := range []string{"1", "2", "10", "12"} {
		if _, err := os.Stat(filepath.Join(out, "deck-slide-"+n+".jpg")); err != nil {
			t.Errorf("expected normalized deck-slide-%s.jpg: %v", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "deck-slide-01.jpg")); !os.IsNotExist(err) {
		t.Errorf("zero-padded pdftoppm name should have been renamed")
	}
}
