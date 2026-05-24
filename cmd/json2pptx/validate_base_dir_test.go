package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// These tests are the regression guard for go-slide-creator-afxh: `json2pptx
// validate <deck.json>` must resolve relative asset paths from the deck's own
// directory (matching `generate`), not the process CWD, with an optional
// --base-dir override and a stdin CWD fallback.

// writeTinyPNG writes a valid 1x1 PNG so both asset-existence resolution and
// real PPTX embedding succeed — the generate half of the parity tests actually
// embeds the bytes, so an invalid file would make them flaky.
func writeTinyPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write png: %v", err)
	}
}

// deckWithBackgroundImage returns a minimal valid deck whose only local asset
// is a relative slide background image path.
func deckWithBackgroundImage(imagePath string) string {
	return `{
  "template": "midnight-blue",
  "slides": [{
    "layout_id": "slideLayout2",
    "slide_type": "content",
    "background": {"image": "` + imagePath + `"},
    "content": [{"placeholder_id": "title", "type": "text", "text_value": "Test"}]
  }]
}`
}

// absTemplatesForTest resolves the shared templates dir to an absolute path so a
// test that t.Chdir-s away from the package directory can still find templates.
func absTemplatesForTest(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(testTemplatesDir)
	if err != nil {
		t.Fatalf("abs templates dir: %v", err)
	}
	return abs
}

func TestValidateBaseDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("file path resolves to its own absolute directory", func(t *testing.T) {
		got := validateBaseDir(filepath.Join("examples", "deck.json"), "")
		want := filepath.Join(cwd, "examples")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("stdin with no override returns empty (CWD fallback)", func(t *testing.T) {
		if got := validateBaseDir("-", ""); got != "" {
			t.Errorf("got %q, want empty string (CWD fallback)", got)
		}
	})

	t.Run("override wins over the file directory", func(t *testing.T) {
		got := validateBaseDir(filepath.Join("examples", "deck.json"), "/abs/assets")
		if got != "/abs/assets" {
			t.Errorf("got %q, want /abs/assets", got)
		}
	})

	t.Run("override wins for stdin", func(t *testing.T) {
		if got := validateBaseDir("-", "/abs/assets"); got != "/abs/assets" {
			t.Errorf("got %q, want /abs/assets", got)
		}
	})

	t.Run("relative override is made absolute", func(t *testing.T) {
		got := validateBaseDir("deck.json", "assets")
		want := filepath.Join(cwd, "assets")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestValidateJSONFile_RelativeAssetResolvesFromInputDir is the pass case: an
// asset that lives beside the deck (but not in the process CWD) validates
// successfully. The package CWD does not contain logo.png, so a CWD-based
// resolution would fail — success proves resolution uses the deck directory.
func TestValidateJSONFile_RelativeAssetResolvesFromInputDir(t *testing.T) {
	deckDir := t.TempDir()
	writeTinyPNG(t, filepath.Join(deckDir, "logo.png"))
	deckPath := filepath.Join(deckDir, "deck.json")
	if err := os.WriteFile(deckPath, []byte(deckWithBackgroundImage("logo.png")), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(deckPath, testTemplatesDir, "", false, "warn")
	if !result.Valid {
		t.Fatalf("expected valid (logo.png exists beside the deck), got errors: %v", result.Errors)
	}
}

// TestValidateJSONFile_RelativeAssetMissingFails is the fail case: a relative
// asset that does not exist beside the deck is reported as an error.
func TestValidateJSONFile_RelativeAssetMissingFails(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	if err := os.WriteFile(deckPath, []byte(deckWithBackgroundImage("missing.png")), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(deckPath, testTemplatesDir, "", false, "warn")
	if result.Valid {
		t.Fatal("expected invalid (missing.png does not exist beside the deck)")
	}
	if !anyContains(result.Errors, "missing.png") {
		t.Errorf("expected an error mentioning missing.png, got: %v", result.Errors)
	}
}

// TestValidateJSONFile_IgnoresSameNamedCWDAsset is the accidental-CWD-asset
// regression: an identically-named asset sits in the process CWD but not beside
// the deck. Resolving from CWD would spuriously pass; resolving from the deck
// directory (the fix) must fail.
func TestValidateJSONFile_IgnoresSameNamedCWDAsset(t *testing.T) {
	absTemplates := absTemplatesForTest(t)

	cwdDir := t.TempDir()
	writeTinyPNG(t, filepath.Join(cwdDir, "logo.png"))
	t.Chdir(cwdDir)

	deckDir := t.TempDir() // a different directory, with no logo.png
	deckPath := filepath.Join(deckDir, "deck.json")
	if err := os.WriteFile(deckPath, []byte(deckWithBackgroundImage("logo.png")), 0644); err != nil {
		t.Fatal(err)
	}

	result := validateJSONFile(deckPath, absTemplates, "", false, "warn")
	if result.Valid {
		t.Fatal("expected invalid: logo.png exists in CWD but not beside the deck; validate must resolve from the deck directory")
	}
	if !anyContains(result.Errors, "logo.png") {
		t.Errorf("expected an error mentioning logo.png, got: %v", result.Errors)
	}
}

// TestValidateJSONFile_BaseDirOverride verifies --base-dir wins: a deck whose
// asset lives in a separate directory fails by default but passes when
// --base-dir points at the asset directory.
func TestValidateJSONFile_BaseDirOverride(t *testing.T) {
	assetDir := t.TempDir()
	writeTinyPNG(t, filepath.Join(assetDir, "logo.png"))

	deckDir := t.TempDir() // no logo.png beside the deck
	deckPath := filepath.Join(deckDir, "deck.json")
	if err := os.WriteFile(deckPath, []byte(deckWithBackgroundImage("logo.png")), 0644); err != nil {
		t.Fatal(err)
	}

	if r := validateJSONFile(deckPath, testTemplatesDir, "", false, "warn"); r.Valid {
		t.Fatal("expected invalid without --base-dir (logo.png not beside the deck)")
	}
	if r := validateJSONFile(deckPath, testTemplatesDir, assetDir, false, "warn"); !r.Valid {
		t.Fatalf("expected valid with --base-dir=%s, got errors: %v", assetDir, r.Errors)
	}
}

// writeTinySVG writes a minimal valid SVG so icon file-path resolution and
// native-SVG embedding both succeed.
func writeTinySVG(t *testing.T, path string) {
	t.Helper()
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke="currentColor" fill="none"><circle cx="12" cy="12" r="9"/></svg>`
	if err := os.WriteFile(path, []byte(svg), 0644); err != nil {
		t.Fatalf("write svg: %v", err)
	}
}

// deckWithPanelIconPath returns a minimal valid deck whose stat_cards panel
// carries a relative .svg icon file path — the asset kind that flows through
// resolvePanelDiagramIcons -> resolveIconInputPath (whose symlink-escape check
// assumes an absolute base dir).
func deckWithPanelIconPath(iconPath string) string {
	return `{
  "template": "midnight-blue",
  "slides": [{
    "layout_id": "slideLayout2",
    "slide_type": "content",
    "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Stats"},
      {"placeholder_id": "body", "type": "diagram", "diagram_value": {
        "type": "stat_cards",
        "data": {"panels": [
          {"title": "Revenue", "value": "$4.2M", "icon": {"path": "` + iconPath + `"}},
          {"title": "Growth", "value": "32%"},
          {"title": "Margin", "value": "18%"}
        ]}
      }}
    ]
  }]
}`
}

// TestGenerateRelativeJSONPathResolvesIconPath is the regression guard for the
// generate/validate base-dir drift: `generate -json deck.json` (a *relative*
// -json path, as an agent runs it from beside the deck) must resolve a relative
// icon file path from the deck's own directory, exactly as validate does.
//
// Before the fix, generate computed inputDir := filepath.Dir("deck.json") == "."
// and passed that relative base dir straight through. resolveIconInputPath
// absolutizes only the base side of its symlink-escape comparison, so the
// relative resolved path could not be made relative to the absolute base and the
// valid icon was falsely flagged ICON_PATH_SYMLINK_ESCAPE. Absolutizing the base
// dir (via validateBaseDir, shared with validate) fixes it.
func TestGenerateRelativeJSONPathResolvesIconPath(t *testing.T) {
	absTemplates := absTemplatesForTest(t)

	deckDir := t.TempDir()
	writeTinySVG(t, filepath.Join(deckDir, "icon.svg"))
	if err := os.WriteFile(filepath.Join(deckDir, "deck.json"),
		[]byte(deckWithPanelIconPath("icon.svg")), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deckDir)

	outDir := t.TempDir()
	// The relative -json path is the crux: filepath.Dir("deck.json") == "." pre-fix.
	genErr := runJSONMode("deck.json", filepath.Join(outDir, "out.json"), absTemplates, outDir,
		"", false, false, "", "off", false, "off", "", false)
	if genErr != nil {
		t.Fatalf("generate with relative -json path: expected success, got: %v", genErr)
	}
}

// TestValidateGenerateAssetParity proves validate and generate agree on whether
// a deck's relative assets resolve, for a deck living in its own directory: both
// succeed when the asset is present and both fail when it is missing.
func TestValidateGenerateAssetParity(t *testing.T) {
	absTemplates := absTemplatesForTest(t)

	t.Run("asset present: validate valid and generate succeeds", func(t *testing.T) {
		deckDir := t.TempDir()
		writeTinyPNG(t, filepath.Join(deckDir, "logo.png"))
		deckPath := filepath.Join(deckDir, "deck.json")
		if err := os.WriteFile(deckPath, []byte(deckWithBackgroundImage("logo.png")), 0644); err != nil {
			t.Fatal(err)
		}

		if vr := validateJSONFile(deckPath, absTemplates, "", false, "warn"); !vr.Valid {
			t.Fatalf("validate: expected valid, got errors: %v", vr.Errors)
		}

		outDir := t.TempDir()
		genErr := runJSONMode(deckPath, filepath.Join(outDir, "out.json"), absTemplates, outDir,
			"", false, false, "", "off", false, "off", "", false)
		if genErr != nil {
			t.Fatalf("generate: expected success, got: %v", genErr)
		}
	})

	t.Run("asset missing: validate invalid and generate fails", func(t *testing.T) {
		deckDir := t.TempDir()
		deckPath := filepath.Join(deckDir, "deck.json")
		if err := os.WriteFile(deckPath, []byte(deckWithBackgroundImage("missing.png")), 0644); err != nil {
			t.Fatal(err)
		}

		if vr := validateJSONFile(deckPath, absTemplates, "", false, "warn"); vr.Valid {
			t.Fatal("validate: expected invalid for a missing relative asset")
		}

		outDir := t.TempDir()
		genErr := runJSONMode(deckPath, filepath.Join(outDir, "out.json"), absTemplates, outDir,
			"", false, false, "", "off", false, "off", "", false)
		if genErr == nil {
			t.Fatal("generate: expected failure for a missing relative asset")
		}
	})
}
