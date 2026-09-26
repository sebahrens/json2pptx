package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// Keep the deliberately unsafe gradient as a test-only variant. A shipped
// template repair must not remove coverage of preflight/refusal behavior.
func unsafeModernGradientDir(t *testing.T) string {
	t.Helper()
	r, err := zip.OpenReader(filepath.Join(testutil.TemplatesDir(), "modern.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "modern.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	patched := 0
	for _, entry := range r.File {
		if entry.Name != "ppt/slideLayouts/slideLayout2.xml" && entry.Name != "ppt/slideLayouts/slideLayout4.xml" {
			if err := w.Copy(entry); err != nil {
				t.Fatal(err)
			}
			continue
		}
		src, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			t.Fatal(err)
		}
		const repaired = `<a:lumMod val="50000"/><a:lumOff val="0"/>`
		const unsafe = `<a:lumMod val="97000"/><a:lumOff val="3000"/>`
		if bytes.Count(body, []byte(repaired)) != 1 {
			t.Fatalf("%s no longer matches the reviewed repaired gradient", entry.Name)
		}
		body = bytes.Replace(body, []byte(repaired), []byte(unsafe), 1)
		dst, err := w.CreateHeader(&entry.FileHeader)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(body); err != nil {
			t.Fatal(err)
		}
		patched++
	}
	if patched != 2 {
		t.Fatalf("unsafe fixture patched %d layout parts, want2", patched)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestModernSubtitleGradientPreflightBlocksFalseContrastFix(t *testing.T) {
	analysis, err := getOrAnalyzeTemplate(filepath.Join(unsafeModernGradientDir(t), "modern.pptx"), template.NewMemoryCache(0))
	if err != nil {
		t.Fatal(err)
	}
	input := &PresentationInput{Template: "modern", Slides: []SlideInput{{
		SlideType: "title", LayoutID: "slideLayout2",
		Content: []ContentInput{{PlaceholderID: "subtitle", Type: "text", TextValue: strPtr("A subtitle")}},
	}}}
	applyDefaults(input)
	pairs := placeholderContrastPairs(input, analysis.Layouts, analysis.Theme.Colors)
	if len(pairs) != 1 || len(pairs[0].Backgrounds) != 3 || pairs[0].Source != "placeholder_fill" {
		t.Fatalf("modern subtitle pair did not retain the visible gradient: %+v", pairs)
	}
	white := svggen.MustParseColor("#FFFFFF")
	wantColors := []string{"#00023A", "#A53F51", "#E99A5B"}
	for i, color := range pairs[0].Backgrounds {
		if color != wantColors[i] {
			t.Errorf("gradient stop %d = %s, want %s", i, color, wantColors[i])
		}
		ratio := white.ContrastWith(svggen.MustParseColor(color))
		if (ratio < 3) != (i == 2) {
			t.Errorf("gradient stop %d contrast = %.2f:1, want only orange to fail 3:1", i, ratio)
		}
	}
	findings := collectContrastPreflightFindings(input, analysis.Layouts, analysis.Theme.Colors)
	var unresolved *patterns.FitFinding
	for i := range findings {
		if findings[i].Code == patterns.ErrCodeContrastUnresolved {
			unresolved = &findings[i]
		}
	}
	if unresolved == nil || unresolved.Action != "refuse" || unresolved.Fix != nil {
		t.Fatalf("modern subtitle must yield a blocking unresolved contrast finding: %+v", findings)
	}
	withoutFullReport := unresolvedPlaceholderContrastFindings(input, analysis.Layouts, analysis.Theme.Colors)
	if len(withoutFullReport) != 1 || withoutFullReport[0].Code != patterns.ErrCodeContrastUnresolved {
		t.Fatalf("generate without fit_report lost the blocking finding: %+v", withoutFullReport)
	}
}

func TestGenerateWithoutFitReportStillReportsModernGradient(t *testing.T) {
	mc := &mcpConfig{templatesDir: unsafeModernGradientDir(t), outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	deck := mustParseJSON(`{"template":"modern","slides":[{"slide_type":"title","layout_id":"slideLayout2","content":[{"placeholder_id":"subtitle","type":"text","text_value":"A subtitle"}]}]}`)
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": deck, "strict_fit": "off", "fit_report": false,
	}))
	if err != nil || result == nil || result.IsError {
		t.Fatalf("generate failed: %v %v", err, result)
	}
	var out JSONOutput
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatal(err)
	}
	for _, finding := range out.FitFindings {
		if finding.Code == patterns.ErrCodeContrastUnresolved && finding.Action == "refuse" {
			return
		}
	}
	t.Fatalf("generate response lost unresolved gradient without fit_report: %+v", out.FitFindings)
}

func TestStrictGenerateRefusesModernGradientContrast(t *testing.T) {
	mc := &mcpConfig{templatesDir: unsafeModernGradientDir(t), outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	deck := mustParseJSON(`{"template":"modern","slides":[{"slide_type":"title","layout_id":"slideLayout2","content":[{"placeholder_id":"subtitle","type":"text","text_value":"A subtitle"}]}]}`)
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": deck, "strict_fit": "strict", "fit_report": false,
	}))
	if err != nil || result == nil || !result.IsError || !strings.Contains(textContent(result), patterns.ErrCodeContrastUnresolved) {
		t.Fatalf("strict generation should refuse the unsafe gradient: err=%v result=%v", err, result)
	}
}

func TestValidateModernGradientIsNotFalseGreen(t *testing.T) {
	mc := &mcpConfig{templatesDir: unsafeModernGradientDir(t), outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	deck := mustParseJSON(`{"template":"modern","slides":[{"slide_type":"title","layout_id":"slideLayout2","content":[{"placeholder_id":"subtitle","type":"text","text_value":"A subtitle"}]}]}`)
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": deck, "fit_report": true,
	}))
	if err != nil || result == nil {
		t.Fatalf("validate failed: %v %v", err, result)
	}
	var out struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatal(err)
	}
	if out.Valid || !strings.Contains(textContent(result), patterns.ErrCodeContrastUnresolved) {
		t.Fatalf("validate must report unsafe gradient instead of valid=true: %s", textContent(result))
	}
}

func TestRepairedModernGradientPassesValidateAndStrictGenerate(t *testing.T) {
	mc := &mcpConfig{templatesDir: testutil.TemplatesDir(), outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	deck := mustParseJSON(`{"template":"modern","slides":[{"slide_type":"title","layout_id":"slideLayout2","content":[{"placeholder_id":"subtitle","type":"text","text_value":"A subtitle"}]}]}`)
	validated, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": true}))
	if err != nil || validated == nil {
		t.Fatalf("validate failed: %v", err)
	}
	var out struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal([]byte(textContent(validated)), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Valid || strings.Contains(textContent(validated), patterns.ErrCodeContrastUnresolved) {
		t.Fatalf("repaired gradient falsely refused: %s", textContent(validated))
	}
	generated, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck, "strict_fit": "strict", "fit_report": false}))
	if err != nil || generated == nil || generated.IsError || strings.Contains(textContent(generated), patterns.ErrCodeContrastUnresolved) {
		t.Fatalf("strict repaired-gradient generation failed: %v %v", err, generated)
	}
}

func TestPlaceholderOwnSolidFillWinsOverWhiteCanvas(t *testing.T) {
	layouts := []types.LayoutMetadata{{
		ID: "slideLayout1", BackgroundHex: "#FFFFFF",
		Placeholders: []types.PlaceholderInfo{{
			ID: "subtitle", InheritedFontColor: "#FFFFFF",
			FillStops: []types.PlaceholderFillStop{{Ref: "#000000"}},
		}},
	}}
	input := &PresentationInput{Slides: []SlideInput{{
		SlideType: "title", LayoutID: "slideLayout1",
		Content: []ContentInput{{PlaceholderID: "subtitle", Type: "text", TextValue: strPtr("Readable")}},
	}}}
	pairs := placeholderContrastPairs(input, layouts, nil)
	if len(pairs) != 1 || pairs[0].Background != "#000000" || pairs[0].Source != "placeholder_fill" {
		t.Fatalf("preflight ignored placeholder's solid fill: %+v", pairs)
	}
	if findings := generator.DetectContrastPreflight(pairs, nil); len(findings) != 0 {
		t.Fatalf("white text on black placeholder fill is readable: %+v", findings)
	}
}

func TestPlaceholderSolidAlphaPreflightUsesVisibleCanvas(t *testing.T) {
	fill := types.PlaceholderFillStop{Ref: "#FF0000", Mods: types.BackgroundColorModifiers{HasAlpha: true, Alpha: 50000}}
	layouts := []types.LayoutMetadata{{ID: "slideLayout1", BackgroundHex: "#FFFFFF", Placeholders: []types.PlaceholderInfo{{
		ID: "subtitle", InheritedFontColor: "#FFFFFF", FillSolid: true, FillStops: []types.PlaceholderFillStop{fill},
	}}}}
	for _, tc := range []struct {
		name, canvas, image, wantBackground, wantCode string
	}{
		{"dark canvas", "#000000", "", "#800000", ""},
		{"light canvas", "#FFFFFF", "", "#FF8080", patterns.ErrCodeContrastPredicted},
		{"photo canvas", "", "photo.png", "", patterns.ErrCodeContrastUnresolved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slide := SlideInput{SlideType: "title", LayoutID: "slideLayout1",
				Content: []ContentInput{{PlaceholderID: "subtitle", Type: "text", TextValue: strPtr("Readable")}}}
			if tc.canvas != "" {
				slide.Background = &BackgroundInput{Color: tc.canvas}
			} else if tc.image != "" {
				slide.Background = &BackgroundInput{Image: tc.image}
			}
			pairs := placeholderContrastPairs(&PresentationInput{Slides: []SlideInput{slide}}, layouts, nil)
			if len(pairs) != 1 || pairs[0].Background != tc.wantBackground {
				t.Fatalf("visible pair = %+v, want background %s", pairs, tc.wantBackground)
			}
			findings := generator.DetectContrastPreflight(pairs, nil)
			if tc.wantCode == "" && len(findings) != 0 {
				t.Fatalf("readable fill reported: %+v", findings)
			}
			if tc.wantCode != "" && (len(findings) != 1 || findings[0].Code != tc.wantCode) {
				t.Fatalf("fill findings = %+v, want %s", findings, tc.wantCode)
			}
		})
	}
}

func TestUnresolvablePlaceholderGradientDoesNotFallBackToCanvas(t *testing.T) {
	for _, tc := range []struct {
		name string
		stop types.PlaceholderFillStop
	}{
		{"unresolvable color", types.PlaceholderFillStop{Ref: "accent99"}},
		{"translucent stop", types.PlaceholderFillStop{Ref: "#000000", Mods: types.BackgroundColorModifiers{HasAlpha: true, Alpha: 50000}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layouts := []types.LayoutMetadata{{
				ID: "slideLayout1", BackgroundHex: "#000000",
				Placeholders: []types.PlaceholderInfo{{
					ID: "subtitle", InheritedFontColor: "#FFFFFF", FillGradient: true,
					FillStops: []types.PlaceholderFillStop{tc.stop},
				}},
			}}
			input := &PresentationInput{Slides: []SlideInput{{
				SlideType: "title", LayoutID: "slideLayout1",
				Content: []ContentInput{{PlaceholderID: "subtitle", Type: "text", TextValue: strPtr("Unknown fill")}},
			}}}
			findings := unresolvedPlaceholderContrastFindings(input, layouts, nil)
			if len(findings) != 1 || findings[0].Action != "refuse" || findings[0].Code != patterns.ErrCodeContrastUnresolved {
				t.Fatalf("gradient must not inherit the readable canvas verdict: %+v", findings)
			}
		})
	}
}
