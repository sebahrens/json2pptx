package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

var testIconTheme = []types.ThemeColor{
	{Name: "accent1", RGB: "#1F3864"},
	{Name: "lt1", RGB: "FFFFFF"},
	{Name: "dk1", RGB: "#000000"},
}

func TestResolveIconFillHex(t *testing.T) {
	tests := []struct {
		fill string
		want string
	}{
		{"accent1", "#1F3864"},
		{"lt1", "#FFFFFF"},
		{"bg1", "#FFFFFF"}, // alias → lt1
		{"tx1", "#000000"}, // alias → dk1
		{"#ff0000", "#FF0000"},
		{"00ff00", "#00FF00"},
		{"#abc", "#AABBCC"},
		{"accent4", ""},   // not in theme
		{"notacolor", ""}, // garbage
		{"", ""},
	}
	for _, tt := range tests {
		if got := resolveIconFillHex(tt.fill, testIconTheme); got != tt.want {
			t.Errorf("resolveIconFillHex(%q) = %q, want %q", tt.fill, got, tt.want)
		}
	}
	// Without a theme a scheme name cannot be resolved.
	if got := resolveIconFillHex("accent1", nil); got != "" {
		t.Errorf("resolveIconFillHex(accent1, nil) = %q, want empty", got)
	}
}

func TestApplyIconFill_RejectsNonHex(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" fill="none" stroke="currentColor"><path d="M0 0"/></svg>`)
	if got := applyIconFill(svg, "accent1"); !bytes.Equal(got, svg) {
		t.Errorf("scheme name must not be written verbatim into SVG, got %s", got)
	}
	got := string(applyIconFill(svg, "#1f3864"))
	if !strings.Contains(got, `stroke="#1F3864"`) {
		t.Errorf("expected resolved stroke, got %s", got)
	}
}

// TestResolveIconSVGThemed_SchemeFill is the ra7t regression: a bundled icon
// with fill "accent1" must come out with a concrete hex stroke, never the raw
// scheme name (which is an invalid SVG paint and renders as nothing).
func TestResolveIconSVGThemed_SchemeFill(t *testing.T) {
	data, err := resolveIconSVGThemed(&shapegrid.IconSpec{Name: "rocket", Fill: "accent1"}, testIconTheme)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "accent1") {
		t.Fatalf("scheme name leaked into SVG: %s", s)
	}
	if !strings.Contains(s, `stroke="#1F3864"`) && !strings.Contains(s, `fill="#1F3864"`) {
		t.Fatalf("expected #1F3864 paint in SVG, got %s", s)
	}
}

// TestPatternIconsRenderWithHexPaint generates a kpi-3up deck on the bundled
// midnight-blue template and asserts every embedded icon SVG carries a hex
// paint (no raw scheme names) and ships a real (non-1x1) PNG fallback.
func TestPatternIconsRenderWithHexPaint(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a full deck")
	}
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")
	dir := t.TempDir()
	title := "Icons"
	values := json.RawMessage(`[{"big":"$4.2M","small":"ARR","icon":"currency-dollar"},{"big":"127%","small":"NRR","icon":"trending-up"},{"big":"12d","small":"Cycle","icon":"clock"}]`)
	input := PresentationInput{
		Template:       "midnight-blue",
		OutputFilename: "icons.pptx",
		Slides: []SlideInput{{
			LayoutID: "content",
			Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
			Pattern:  &PatternInput{Name: "kpi-3up", Values: values},
		}},
	}
	b, _ := json.Marshal(input)
	inPath := filepath.Join(dir, "in.json")
	if err := os.WriteFile(inPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	resPath := filepath.Join(dir, "res.json")
	if err := runJSONMode(inPath, resPath, templatesDir, dir, "", false, false, "midnight-blue", "off", false, "warn", "free", false); err != nil {
		t.Fatalf("runJSONMode: %v", err)
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "icons.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	hexPaint := regexp.MustCompile(`(stroke|fill)="#[0-9A-F]{6}"`)
	svgCount, pngOK := 0, 0
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/media/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		switch {
		case strings.HasSuffix(f.Name, ".svg"):
			svgCount++
			s := string(data)
			for _, bad := range []string{`="accent`, `="lt1"`, `="dk1"`, `="tx1"`, `="bg1"`} {
				if strings.Contains(s, bad) {
					t.Errorf("%s: raw scheme paint %s in SVG", f.Name, bad)
				}
			}
			if !hexPaint.MatchString(s) {
				t.Errorf("%s: no hex paint in icon SVG: %.200s", f.Name, s)
			}
		case strings.HasSuffix(f.Name, ".png"):
			img, err := png.Decode(bytes.NewReader(data))
			if err == nil && img.Bounds().Dx() > 1 {
				pngOK++
			}
		}
	}
	if svgCount < 3 {
		t.Fatalf("expected >= 3 icon SVGs, got %d", svgCount)
	}
	if pngOK < 3 {
		t.Errorf("expected >= 3 real PNG fallbacks, got %d", pngOK)
	}
}
