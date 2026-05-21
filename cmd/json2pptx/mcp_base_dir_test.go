package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
)

// TestResolveBaseDir_AbsoluteDir accepts an absolute directory and returns it
// resolved via EvalSymlinks. This is the happy path agents take.
func TestResolveBaseDir_AbsoluteDir(t *testing.T) {
	tmp := t.TempDir()
	req := makeRequest(map[string]any{"base_dir": tmp})

	got, errResult := resolveBaseDir(req)
	if errResult != nil {
		t.Fatalf("unexpected error result: %v", errResult)
	}
	want, _ := filepath.EvalSymlinks(tmp)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveBaseDir_RelativeRejected ensures a relative base_dir is rejected
// with an INVALID_PARAMETER diagnostic. Relative paths re-introduce the
// CWD-coupling that base_dir exists to eliminate.
func TestResolveBaseDir_RelativeRejected(t *testing.T) {
	req := makeRequest(map[string]any{"base_dir": "decks"})

	got, errResult := resolveBaseDir(req)
	if errResult == nil {
		t.Fatalf("expected error result, got base_dir=%q", got)
	}
	env := parseMCPError(t, errResult)
	d := requireDiagCode(t, env.Diagnostics, "INVALID_PARAMETER")
	if d.Path != "base_dir" {
		t.Errorf("expected path=base_dir, got %q", d.Path)
	}
}

// TestResolveBaseDir_MissingDirRejected ensures a non-existent base_dir is
// rejected with INVALID_PARAMETER so agents see a typed signal instead of
// N broken-asset findings.
func TestResolveBaseDir_MissingDirRejected(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	req := makeRequest(map[string]any{"base_dir": missing})

	_, errResult := resolveBaseDir(req)
	if errResult == nil {
		t.Fatal("expected error result for missing base_dir")
	}
	env := parseMCPError(t, errResult)
	requireDiagCode(t, env.Diagnostics, "INVALID_PARAMETER")
}

// TestResolveBaseDir_FileRejected ensures a base_dir that points at a file
// (not a directory) is rejected with INVALID_PARAMETER.
func TestResolveBaseDir_FileRejected(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "not-a-dir.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	req := makeRequest(map[string]any{"base_dir": f})

	_, errResult := resolveBaseDir(req)
	if errResult == nil {
		t.Fatal("expected error result for file base_dir")
	}
	env := parseMCPError(t, errResult)
	d := requireDiagCode(t, env.Diagnostics, "INVALID_PARAMETER")
	if d.Path != "base_dir" {
		t.Errorf("expected path=base_dir, got %q", d.Path)
	}
}

// TestResolveBaseDir_OmittedFallsBackToCWD verifies the legacy fallback when
// base_dir is absent: we still return a path (the server CWD) so existing
// callers that supply absolute paths in JSON keep working.
func TestResolveBaseDir_OmittedFallsBackToCWD(t *testing.T) {
	req := makeRequest(map[string]any{})

	got, errResult := resolveBaseDir(req)
	if errResult != nil {
		t.Fatalf("unexpected error result: %v", errResult)
	}
	if got == "" {
		t.Error("expected non-empty CWD fallback, got empty")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if got != cwd {
		t.Errorf("got %q, want %q", got, cwd)
	}
}

// TestHandleValidate_BaseDirResolvesRelativeAsset confirms that validate_input
// accepts a relative asset path when the agent supplies an absolute base_dir.
// The asset materializes inside base_dir, so the per-asset walker resolves it
// to an absolute path without emitting a finding — regardless of where the
// server was launched from.
func TestHandleValidate_BaseDirResolvesRelativeAsset(t *testing.T) {
	baseDir := t.TempDir()
	bgFile := filepath.Join(baseDir, "bg.jpg")
	if err := os.WriteFile(bgFile, []byte("x"), 0644); err != nil {
		t.Fatalf("write %s: %v", bgFile, err)
	}

	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Run from a CWD that has no relation to baseDir to prove the server's
	// process CWD is no longer load-bearing.
	otherCWD := t.TempDir()
	origCWD, _ := os.Getwd()
	if err := os.Chdir(otherCWD); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origCWD) }()

	presentation := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id":  "slideLayout2",
				"background": map[string]any{"image": "bg.jpg"},
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hi"},
				},
			},
		},
	}

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": presentation,
		"base_dir":     baseDir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %+v", result.StructuredContent)
	}

	b, _ := json.Marshal(result.StructuredContent)
	var resp dryRunOutput
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Asset findings are folded into the namespaced Findings envelope
	// (e.g. INPUT.BACKGROUND_IMAGE_PATH); none should appear.
	for _, f := range resp.Findings.Findings {
		if strings.Contains(f.Code, "IMAGE_PATH") {
			t.Errorf("unexpected asset finding: %+v", f)
		}
	}
}

// TestHandleValidate_BaseDirMissingAssetEmitsStructuredFinding confirms that
// when an asset referenced by the JSON does not exist under base_dir, the
// validate handler emits a structured per-asset diagnostic instead of a
// stack trace or bag-of-strings error.
func TestHandleValidate_BaseDirMissingAssetEmitsStructuredFinding(t *testing.T) {
	baseDir := t.TempDir()

	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	presentation := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id":  "slideLayout2",
				"background": map[string]any{"image": "missing-bg.jpg"},
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hi"},
				},
			},
		},
	}

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": presentation,
		"base_dir":     baseDir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A missing asset invalidates the deck, so validate returns the MCP
	// diagnostics error envelope (IsError=true, {diagnostics, summary}) with the
	// raw, un-namespaced diagnostic codes and their Details preserved.
	if !result.IsError {
		t.Fatalf("expected IsError=true for missing background asset, got success: %+v", result.StructuredContent)
	}
	env := parseMCPError(t, result)
	d := requireDiagCode(t, env.Diagnostics, "BACKGROUND_IMAGE_PATH")
	if d.Details["asset_kind"] != "background" {
		t.Errorf("expected asset_kind=background in details, got %v", d.Details["asset_kind"])
	}
	if d.Details["input_value"] != "missing-bg.jpg" {
		t.Errorf("expected input_value=missing-bg.jpg, got %v", d.Details["input_value"])
	}
}

// TestHandleValidate_InvalidBaseDirShortCircuits ensures a bad base_dir
// emits an INVALID_PARAMETER diagnostic before any per-asset findings — the
// agent can't fix individual paths until the base directory is correct.
func TestHandleValidate_InvalidBaseDirShortCircuits(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	presentation := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "slideLayout2",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hi"},
				},
			},
		},
	}

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": presentation,
		"base_dir":     "relative/path/not/allowed",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	requireStructuredError(t, result, "INVALID_PARAMETER")
}

// TestCapabilities_AdvertisesBaseDir confirms get_capabilities lists every
// tool that honours the base_dir parameter so agents can capability-gate
// without parsing tool descriptions.
func TestCapabilities_AdvertisesBaseDir(t *testing.T) {
	resp := getCapabilitiesResult(t)

	want := map[string]bool{
		"generate_presentation":        true,
		"validate_input":               true,
		"preview_presentation_plan":    true,
		"render_slide_image_from_json": true,
		"score_deck":                   true,
		"auto_repair":                  true,
		"make_deck":                    true,
	}
	got := make(map[string]bool, len(resp.Features.BaseDir))
	for _, n := range resp.Features.BaseDir {
		got[n] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("expected features.base_dir to include %q, got %v", name, resp.Features.BaseDir)
		}
	}
	if v, ok := resp.Features.FeatureVersions["base_dir"]; !ok || v == "" {
		t.Errorf("expected feature_versions[base_dir] to be set, got %q", v)
	}
}
