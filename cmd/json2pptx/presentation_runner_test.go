package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestRunPresentation_ChartPatternFallbackResolution(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		if _, err := exec.LookPath("resvg"); err != nil {
			t.Skip("chart PNG fallback requires rsvg-convert or resvg")
		}
	}
	data, err := os.ReadFile("../../examples/chart-insights-split.json")
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	applyDefaults(&input)
	res, cleanup, err := RunPresentation(context.Background(), &input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: runnerTestTemplatesDir(t), StrictFit: "warn", OutputValidation: "strict",
	})
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	report, err := pptx.ValidateOutputFile(res.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Code == "OOXML_LOW_FALLBACK_DPI" {
			t.Errorf("chart pattern still ships a low-resolution fallback: %+v", finding)
		}
	}
	archive, err := zip.OpenReader(res.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()
	chartPNGs := 0
	for _, file := range archive.File {
		if !strings.HasPrefix(file.Name, "ppt/media/") || !strings.HasSuffix(file.Name, ".png") {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		chartImage, decodeErr := png.Decode(reader)
		_ = reader.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		bounds := chartImage.Bounds()
		if bounds.Dx() < 1000 || bounds.Dy() < 500 {
			continue
		}
		chartPNGs++
		// The chart fixture places category labels below the axis, in the
		// bottom 8% of its SVG. The old canvas rasterizer produced a large
		// fallback but dropped every <text> element, leaving this band blank.
		dark := 0
		for y := bounds.Min.Y + bounds.Dy()*92/100; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X + bounds.Dx()/10; x < bounds.Max.X; x++ {
				r, g, b, _ := chartImage.At(x, y).RGBA()
				if r < 45000 && g < 45000 && b < 45000 {
					dark++
				}
			}
		}
		if dark < 20 {
			t.Errorf("%s has no visible category labels in the PNG fallback", file.Name)
		}
	}
	if chartPNGs != 2 {
		t.Errorf("high-resolution chart fallbacks = %d, want 2", chartPNGs)
	}
}

func TestRunPresentation_BlankSlideTitleSurvives(t *testing.T) {
	text := "Blank canvas heading"
	input := &PresentationInput{
		Template: "midnight-blue", OutputFilename: "blank-title.pptx",
		Slides: []SlideInput{
			{SlideType: "blank", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &text}}},
			{SlideType: "blank", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &text}}},
			{SlideType: "content", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Middle")}}},
			{SlideType: "blank", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &text}}},
		},
	}
	applyDefaults(input)
	res, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: runnerTestTemplatesDir(t), StrictFit: "warn", OutputValidation: "strict",
	})
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(res.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()
	for _, slideNo := range []int{1, 2, 4} {
		path := fmt.Sprintf("ppt/slides/slide%d.xml", slideNo)
		found := false
		for _, file := range archive.File {
			if file.Name != path {
				continue
			}
			found = true
			reader, openErr := file.Open()
			if openErr != nil {
				t.Fatal(openErr)
			}
			data, readErr := io.ReadAll(reader)
			_ = reader.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !strings.Contains(string(data), text) {
				t.Errorf("%s dropped the authored blank-slide title", path)
			}
		}
		if !found {
			t.Errorf("generated deck has no %s", path)
		}
	}
}

// runnerTestTemplatesDir returns the repo's templates/ directory relative to
// this test file. resolveTemplatePath also falls back to embedded templates,
// so this is a best-effort hint rather than a hard requirement.
func runnerTestTemplatesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// cmd/json2pptx/<file> -> repo root is two levels up.
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	return filepath.Join(root, "templates")
}

// TestRunPresentation_InMemory exercises the reusable render runner with a
// compiled, in-memory *deckinput.PresentationInput — no intermediate raw JSON
// is written to disk. It asserts a valid .pptx is produced and that the file
// passes pptx.ValidateOutputFile (the same strict gate the CLI/MCP paths use).
func TestRunPresentation_InMemory(t *testing.T) {
	templatesDir := runnerTestTemplatesDir(t)
	outDir := t.TempDir()

	// Build the deck entirely in memory using typed value fields.
	titleText := "Reusable Runner Smoke Test"
	bodyText := "Rendered straight from an in-memory PresentationInput."
	input := &PresentationInput{
		Template:       "midnight-blue",
		OutputFilename: "in-memory.pptx",
		Slides: []SlideInput{
			{
				LayoutID: "title",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &titleText},
				},
			},
			{
				LayoutID: "content",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &bodyText},
					{PlaceholderID: "body", Type: "bullets", BulletsValue: &[]string{
						"First point",
						"Second point",
						"Third point",
					}},
				},
			},
		},
	}

	// Apply deck-level defaults exactly as the callers do before invoking the
	// runner. No structure expansion / boundary validation needed for this deck.
	applyDefaults(input)

	res, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir:        outDir,
		TemplatesDir:     templatesDir,
		StrictFit:        "warn",
		OutputValidation: "strict",
		AccentStrategy:   patterns.AccentStrategy(input.AccentStrategy),
	})
	defer cleanup()
	if err != nil {
		t.Fatalf("RunPresentation failed: %v", err)
	}

	// The runner must have produced a real file on disk.
	if res.OutputPath == "" {
		t.Fatal("RunPresentation returned an empty output path")
	}
	if _, statErr := os.Stat(res.OutputPath); statErr != nil {
		t.Fatalf("output file not created at %s: %v", res.OutputPath, statErr)
	}
	if filepath.Dir(res.OutputPath) != outDir {
		t.Fatalf("output written outside temp dir: got %s, want under %s", res.OutputPath, outDir)
	}

	// Generator result sanity: both slides should be present.
	if res.GenResult == nil {
		t.Fatal("RunPresentation returned nil GenResult")
	}
	if res.GenResult.SlideCount != 2 {
		t.Fatalf("expected 2 slides, got %d", res.GenResult.SlideCount)
	}

	// The produced .pptx must pass the same strict output validation the
	// default render contract guarantees.
	report, valErr := pptx.ValidateOutputFile(res.OutputPath)
	if valErr != nil {
		t.Fatalf("ValidateOutputFile errored: %v", valErr)
	}
	if !report.IsValid() {
		t.Fatalf("generated .pptx failed output validation: %d blocking finding(s): %v",
			len(report.Blocking()), report.Blocking())
	}
}

// TestRunPresentation_StrictOutputValidationDefault verifies that an empty
// OutputValidation in RenderOptions defaults to strict (the standing
// 'zero needs repair' guarantee), so a clean deck still produces a validated
// file without the caller explicitly opting in.
func TestRunPresentation_StrictOutputValidationDefault(t *testing.T) {
	templatesDir := runnerTestTemplatesDir(t)
	outDir := t.TempDir()

	titleText := "Default Validation"
	input := &PresentationInput{
		Template:       "midnight-blue",
		OutputFilename: "default-validation.pptx",
		Slides: []SlideInput{
			{
				LayoutID: "title",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &titleText},
				},
			},
		},
	}
	applyDefaults(input)

	res, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir:    outDir,
		TemplatesDir: templatesDir,
		// StrictFit and OutputValidation intentionally left empty to exercise
		// the strict-by-default behaviour.
	})
	defer cleanup()
	if err != nil {
		t.Fatalf("RunPresentation failed: %v", err)
	}
	if _, statErr := os.Stat(res.OutputPath); statErr != nil {
		t.Fatalf("output file not created: %v", statErr)
	}
	// Validation ran by default; re-running it independently must also pass.
	report, valErr := pptx.ValidateOutputFile(res.OutputPath)
	if valErr != nil || !report.IsValid() {
		t.Fatalf("default-validated deck is not valid: err=%v valid=%v", valErr, report.IsValid())
	}
}

// TestRunPresentation_PreConvertHook verifies the PreConvert hook runs after
// strict_fit and before slide conversion, and that an error it raises aborts
// the run verbatim.
func TestRunPresentation_PreConvertHook(t *testing.T) {
	templatesDir := runnerTestTemplatesDir(t)
	outDir := t.TempDir()

	titleText := "Hook Test"
	input := &PresentationInput{
		Template:       "midnight-blue",
		OutputFilename: "hook.pptx",
		Slides: []SlideInput{
			{LayoutID: "title", Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: &titleText},
			}},
		},
	}
	applyDefaults(input)

	called := false
	_, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir:    outDir,
		TemplatesDir: templatesDir,
		PreConvert: func() error {
			called = true
			return context.Canceled // sentinel error, aborts the run
		},
	})
	defer cleanup()
	if !called {
		t.Fatal("PreConvert hook was not invoked")
	}
	if err == nil {
		t.Fatal("expected error from PreConvert hook, got nil")
	}
	// No file should have been generated because the hook aborted before convert.
	if _, statErr := os.Stat(filepath.Join(outDir, "hook.pptx")); statErr == nil {
		t.Fatal("output file should not exist after PreConvert abort")
	}
}

// go-slide-creator-p327: the chart data palette and the svggen theme colours
// both resolve scheme names through the theme struct, so resolving them against
// the PRE-override theme painted chart series in the template's original
// palette while the artifact's theme part carried the override.
func TestResolveDataPalette_FollowsThemeOverride(t *testing.T) {
	metadata := &types.TemplateMetadata{DataPalette: []string{"accent1", "accent2"}}
	theme := types.ThemeInfo{
		TitleFont: "Gill Sans",
		BodyFont:  "Calibri",
		Colors: []types.ThemeColor{
			{Name: "accent1", RGB: "#2E5090"},
			{Name: "accent2", RGB: "#C0504D"},
		},
	}

	before := resolveDataPalette(metadata, theme.Colors)
	if len(before) != 2 || before[0] != "#2E5090" {
		t.Fatalf("baseline palette = %v, want the template colours", before)
	}

	overridden, warnings := theme.ApplyOverride(&types.ThemeOverride{
		Colors:   map[string]string{"accent1": "#6A1B9A"},
		BodyFont: "Verdana",
	})
	after := resolveDataPalette(metadata, overridden.Colors)

	if len(after) != 2 {
		t.Fatalf("palette = %v, want 2 entries", after)
	}
	if !strings.EqualFold(after[0], "#6A1B9A") {
		t.Errorf("palette[0] = %q, want the overridden #6A1B9A", after[0])
	}
	if !strings.EqualFold(after[1], "#C0504D") {
		t.Errorf("palette[1] = %q, want the untouched #C0504D", after[1])
	}
	// The font advisory must be available to surface as a deck warning.
	if len(warnings) == 0 {
		t.Error("overriding body_font to a font the template does not embed should warn")
	}
}
