package svggen

import (
	"archive/zip"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// readTemplateSemanticAccents reads semantic_accents from a template's
// go-slide-creator metadata (empty when absent).
func readTemplateSemanticAccents(t *testing.T, path string) SemanticAccentSpec {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != "ppt/go-slide-creator-metadata.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return SemanticAccentSpec{}
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		var meta struct {
			SemanticAccents SemanticAccentSpec `json:"semantic_accents"`
		}
		_ = json.Unmarshal(data, &meta)
		return meta.SemanticAccents
	}
	return SemanticAccentSpec{}
}

// TestSeriesPalette_PerTemplateRestrained is the go-slide-creator-orqni
// acceptance test. On every bundled template (and the local p-style when
// present), the automatic colours of a 2-5 series chart:
//   - are never a text ink, nor within MinSeriesDeltaE of dk1;
//   - never use the template's semantic positive / negative accent for a
//     2-3 series chart (red / green read as a verdict).
func TestSeriesPalette_PerTemplateRestrained(t *testing.T) {
	files, err := filepath.Glob(chartPaletteTemplatesGlob)
	if err != nil || len(files) == 0 {
		t.Fatalf("no templates under %s: %v", chartPaletteTemplatesGlob, err)
	}
	sort.Strings(files)
	for _, f := range files {
		name := filepath.Base(f)
		themeColors, paletteHex, _, err := loadChartPaletteFromTemplate(f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		semantic := readTemplateSemanticAccents(t, f)
		guide := StyleGuideFromSpec(StyleSpec{
			ThemeColors: themeColors, DataPalette: paletteHex, SemanticAccents: semantic,
			DisablePaletteEnforcement: true, Background: "transparent",
		})
		p := guide.Palette
		slot := func(n string) string {
			for _, tc := range themeColors {
				if tc.Name == n {
					return strings.ToUpper(tc.RGB)
				}
			}
			return ""
		}
		alert := map[string]bool{}
		for _, n := range []string{semantic.Positive, semantic.Negative} {
			if n != "" {
				alert[slot(n)] = true
			}
		}
		for n := 2; n <= 5; n++ {
			colors := resolveColors(nil, guide, n)
			for i, c := range colors {
				if c == p.TextPrimary || c == p.TextSecondary {
					t.Errorf("%s n=%d series %d is a text ink %s", name, n, i, c.Hex())
				}
				if d := deltaE76(c, p.TextPrimary); d < MinSeriesDeltaE {
					t.Errorf("%s n=%d series %d %s is within ΔE %.1f of dk1", name, n, i, c.Hex(), d)
				}
				if n <= 3 && i > 0 && alert[strings.ToUpper(c.Hex())] {
					t.Errorf("%s n=%d series %d uses the semantic alert accent %s", name, n, i, c.Hex())
				}
			}
		}
	}
}
