package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

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
