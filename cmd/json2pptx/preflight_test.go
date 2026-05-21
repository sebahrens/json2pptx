package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// preflightOnJSON runs the preflight core on a deck JSON string against the
// bundled templates and returns the resulting envelope.
func preflightOnJSON(t *testing.T, deck string, strict bool) diagnostics.FindingEnvelope {
	t.Helper()
	return runPreflightCore([]byte(deck), preflightOptions{templatesDir: testTemplatesDir, strict: strict})
}

// stageOfFinding returns the stage tag a preflight finding carries in its
// evidence, or "" when absent.
func stageOfFinding(f diagnostics.Finding) string {
	if f.Evidence == nil {
		return ""
	}
	if s, ok := f.Evidence["stage"].(string); ok {
		return s
	}
	return ""
}

// findFindingByStage returns the first finding tagged with the given stage, or
// nil when none is present.
func findFindingByStage(env diagnostics.FindingEnvelope, stage string) *diagnostics.Finding {
	for i := range env.Findings {
		if stageOfFinding(env.Findings[i]) == stage {
			return &env.Findings[i]
		}
	}
	return nil
}

func TestPreflightCore_CleanDeckExits0(t *testing.T) {
	deck := `{
      "template": "midnight-blue",
      "slides": [{
        "layout_id": "slideLayout2",
        "slide_type": "content",
        "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hello"}]
      }]
    }`
	env := preflightOnJSON(t, deck, false)
	if !env.OK {
		t.Fatalf("expected ok=true on a clean deck, got findings: %+v", env.Findings)
	}
	if len(env.Findings) != 0 {
		t.Fatalf("expected no findings on a clean deck, got %d: %+v", len(env.Findings), env.Findings)
	}
	if got := preflightExitCode(env, false); got != 0 {
		t.Errorf("expected exit 0 on a clean deck, got %d", got)
	}
	if env.Subcommand != "preflight" {
		t.Errorf("expected subcommand=preflight, got %q", env.Subcommand)
	}
	if env.InputSHA256 == "" {
		t.Error("expected a non-empty input_sha256 for request correlation")
	}
	if env.Template != "midnight-blue" {
		t.Errorf("expected template=midnight-blue, got %q", env.Template)
	}
}

// Each stage test exercises a different finding code and asserts the stage tag
// preflight assigns to it (acceptance criterion: each stage unit-tested).

func TestPreflightCore_InputStage_InvalidJSON(t *testing.T) {
	env := preflightOnJSON(t, `{not valid json`, false)
	f := findFindingByStage(env, preflightStageInput)
	if f == nil {
		t.Fatalf("expected an INPUT-stage finding for malformed JSON, got: %+v", env.Findings)
	}
	if f.Severity != diagnostics.SeverityError {
		t.Errorf("expected error severity, got %q", f.Severity)
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2 on a parse error, got %d", got)
	}
}

func TestPreflightCore_InputStage_BadEnum(t *testing.T) {
	deck := `{
      "template": "midnight-blue",
      "slides": [{
        "layout_id": "slideLayout2",
        "slide_type": "content",
        "transition": "NOT_A_REAL_TRANSITION",
        "content": [{"placeholder_id": "title", "type": "text", "text_value": "Hi"}]
      }]
    }`
	env := preflightOnJSON(t, deck, false)
	f := findFindingByStage(env, preflightStageInput)
	if f == nil {
		t.Fatalf("expected an INPUT-stage finding for a bad enum, got: %+v", env.Findings)
	}
	if env.OK {
		t.Error("expected ok=false on an enum error")
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2, got %d", got)
	}
}

func TestPreflightCore_InputStage_MissingTemplateShortCircuits(t *testing.T) {
	deck := `{"slides": [{"layout_id": "slideLayout2", "content": []}]}`
	env := preflightOnJSON(t, deck, false)
	if findFindingByStage(env, preflightStageInput) == nil {
		t.Fatalf("expected an INPUT-stage required finding, got: %+v", env.Findings)
	}
	// A missing template must short-circuit before the TEMPLATE stage — there is
	// nothing to resolve.
	if f := findFindingByStage(env, preflightStageTemplate); f != nil {
		t.Errorf("expected no TEMPLATE-stage finding when template is missing, got %+v", f)
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2, got %d", got)
	}
}

func TestPreflightCore_PolicyStage_Emoji(t *testing.T) {
	deck := `{
      "template": "midnight-blue",
      "slides": [{
        "layout_id": "slideLayout2",
        "slide_type": "content",
        "content": [{"placeholder_id": "title", "type": "text", "text_value": "Launch 🚀 plan"}]
      }]
    }`
	env := preflightOnJSON(t, deck, false)
	f := findFindingByStage(env, preflightStagePolicy)
	if f == nil {
		t.Fatalf("expected a POLICY-stage finding for emoji, got: %+v", env.Findings)
	}
	if f.Code != diagnostics.DottedCode(diagnostics.NamespacePolicy, "no_emoji_violation") {
		t.Errorf("expected the no_emoji_violation code, got %q", f.Code)
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2 on a policy error, got %d", got)
	}
}

func TestPreflightCore_TemplateStage_NotFound(t *testing.T) {
	deck := `{
      "template": "does-not-exist",
      "slides": [{"layout_id": "slideLayout2", "content": [{"placeholder_id": "title", "type": "text", "text_value": "x"}]}]
    }`
	env := preflightOnJSON(t, deck, false)
	f := findFindingByStage(env, preflightStageTemplate)
	if f == nil {
		t.Fatalf("expected a TEMPLATE-stage finding for a missing template, got: %+v", env.Findings)
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2, got %d", got)
	}
}

func TestPreflightCore_LayoutStage_UnknownLayout(t *testing.T) {
	deck := `{
      "template": "midnight-blue",
      "slides": [{"layout_id": "slideLayoutNope", "content": [{"placeholder_id": "title", "type": "text", "text_value": "x"}]}]
    }`
	env := preflightOnJSON(t, deck, false)
	f := findFindingByStage(env, preflightStageLayout)
	if f == nil {
		t.Fatalf("expected a LAYOUT-stage finding for an unknown layout_id, got: %+v", env.Findings)
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2, got %d", got)
	}
}

func TestPreflightCore_PlaceholderStage_MaxLength(t *testing.T) {
	// A title far over the placeholder's character budget. The over-budget check
	// is a character count (deterministic across platforms — no font metrics).
	longTitle := ""
	for i := 0; i < 40; i++ {
		longTitle += "verylongword "
	}
	deck := `{
      "template": "midnight-blue",
      "slides": [{
        "layout_id": "slideLayout2",
        "slide_type": "content",
        "content": [{"placeholder_id": "title", "type": "text", "text_value": "` + longTitle + `"}]
      }]
    }`
	env := preflightOnJSON(t, deck, false)
	f := findFindingByStage(env, preflightStagePlaceholder)
	if f == nil {
		t.Fatalf("expected a PLACEHOLDER-stage finding for an over-budget title, got: %+v", env.Findings)
	}
	// A max-length over-budget title is a warning, so a non-strict run still
	// exits 0; --strict promotes it to a failure.
	if got := preflightExitCode(env, false); got != 0 {
		t.Errorf("expected exit 0 on warning-only deck (non-strict), got %d", got)
	}
	if got := preflightExitCode(env, true); got != 2 {
		t.Errorf("expected exit 2 on warning-only deck (--strict), got %d", got)
	}
}

func TestPreflightCore_GridStage_InvalidGeometry(t *testing.T) {
	deck := `{
      "template": "midnight-blue",
      "slides": [{
        "layout_id": "slideLayout2",
        "slide_type": "content",
        "content": [{"placeholder_id": "title", "type": "text", "text_value": "x"}],
        "shape_grid": {
          "columns": 1,
          "rows": [{"cells": [{"shape": {"geometry": "notARealGeometry", "fill": "accent1", "text": "X"}}]}]
        }
      }]
    }`
	env := preflightOnJSON(t, deck, false)
	f := findFindingByStage(env, preflightStageGrid)
	if f == nil {
		t.Fatalf("expected a GRID-stage finding for an invalid geometry, got: %+v", env.Findings)
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2 on a grid error, got %d", got)
	}
}

func TestPreflightCore_PatternStage_UnknownPattern(t *testing.T) {
	deck := `{
      "template": "midnight-blue",
      "slides": [{"layout_id": "slideLayout2", "pattern": {"name": "no-such-pattern", "values": {}}}]
    }`
	env := preflightOnJSON(t, deck, false)
	f := findFindingByStage(env, preflightStagePattern)
	if f == nil {
		t.Fatalf("expected a PATTERN-stage finding for an unknown pattern, got: %+v", env.Findings)
	}
	if got := preflightExitCode(env, false); got != 2 {
		t.Errorf("expected exit 2 on a pattern error, got %d", got)
	}
}

// TestPreflightCore_StrictWarningsExit2 is the explicit acceptance criterion:
// a deck with only warnings exits 0 normally and 2 under --strict.
func TestPreflightCore_StrictWarningsExit2(t *testing.T) {
	// legacy "value" authoring form is a warning, not an error, and is
	// independent of font metrics.
	deck := `{
      "template": "midnight-blue",
      "slides": [{
        "layout_id": "slideLayout2",
        "slide_type": "content",
        "content": [{"placeholder_id": "title", "type": "text", "value": "legacy"}]
      }]
    }`
	env := preflightOnJSON(t, deck, false)
	if !env.OK {
		t.Fatalf("expected ok=true (warnings only), got findings: %+v", env.Findings)
	}
	hasWarning := false
	for _, f := range env.Findings {
		if f.Severity == diagnostics.SeverityWarning {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("test fixture produced no warning; findings: %+v", env.Findings)
	}
	if got := preflightExitCode(env, false); got != 0 {
		t.Errorf("expected exit 0 on warnings-only deck (non-strict), got %d", got)
	}
	if got := preflightExitCode(env, true); got != 2 {
		t.Errorf("expected exit 2 on warnings-only deck (--strict), got %d", got)
	}
}

// TestPreflightCore_FindingsOrderedByStage verifies the envelope's findings are
// emitted in stage order (then severity within a stage).
func TestPreflightCore_FindingsOrderedByStage(t *testing.T) {
	// Emoji (POLICY, error) plus an over-budget title (PLACEHOLDER, warning) on
	// the same slide. POLICY (rank 1) must precede PLACEHOLDER (rank 4).
	longTitle := "Launch \xF0\x9F\x9A\x80 "
	for i := 0; i < 40; i++ {
		longTitle += "verylongword "
	}
	deck := `{
      "template": "midnight-blue",
      "slides": [{
        "layout_id": "slideLayout2",
        "slide_type": "content",
        "content": [{"placeholder_id": "title", "type": "text", "text_value": "` + longTitle + `"}]
      }]
    }`
	env := preflightOnJSON(t, deck, false)
	if findFindingByStage(env, preflightStagePolicy) == nil || findFindingByStage(env, preflightStagePlaceholder) == nil {
		t.Fatalf("expected both POLICY and PLACEHOLDER findings, got: %+v", env.Findings)
	}
	lastRank := -1
	for _, f := range env.Findings {
		r, ok := preflightStageRank[stageOfFinding(f)]
		if !ok {
			t.Fatalf("finding has an unrecognized stage %q: %+v", stageOfFinding(f), f)
		}
		if r < lastRank {
			t.Fatalf("findings out of stage order: stage %q (rank %d) appeared after rank %d", stageOfFinding(f), r, lastRank)
		}
		lastRank = r
	}
}

// TestClassifyFitStage covers the per-finding stage routing for fit findings,
// including RENDER_PROJECTION whose codes are geometry/font dependent and so are
// asserted here deterministically rather than through a rendered deck.
func TestClassifyFitStage(t *testing.T) {
	cases := map[string]string{
		patterns.ErrCodeFooterCollision:     preflightStageRender,
		patterns.ErrCodeTitleCollision:      preflightStageRender,
		patterns.ErrCodeTitleWraps:          preflightStageRender,
		patterns.ErrCodeSlideBoundsOverflow: preflightStageGrid,
		patterns.ErrCodeSparseLayout:        preflightStageGrid,
		patterns.ErrCodeCellUnderfilled:     preflightStageGrid,
		patterns.ErrCodeAccentOverload:      preflightStageGrid,
		patterns.ErrCodeContrastPredicted:   preflightStageGrid,
		patterns.ErrCodePatternUnderfilled:  preflightStagePattern,
		patterns.ErrCodePatternOvercrowded:  preflightStagePattern,
		patterns.ErrCodeWrongPattern:        preflightStagePattern,
		patterns.ErrCodePlaceholderOverflow: preflightStagePlaceholder,
		patterns.ErrCodeBodyTooLong:         preflightStagePlaceholder,
		patterns.ErrCodeMissingAltText:      preflightStagePlaceholder,
		"some_unknown_future_code":          preflightStagePlaceholder,
	}
	for code, want := range cases {
		got := classifyFitStage(diagnostics.Diagnostic{Code: code})
		if got != want {
			t.Errorf("classifyFitStage(%q) = %q, want %q", code, got, want)
		}
	}
}

// TestClassifySlideValidationStage covers the per-finding stage routing for the
// slide-vs-template validator.
func TestClassifySlideValidationStage(t *testing.T) {
	cases := []struct {
		code string
		path string
		want string
	}{
		{patterns.ErrCodeUnknownLayoutID, "slides[0].layout_id", preflightStageLayout},
		{patterns.ErrCodeRequired, "slides[0].layout_id", preflightStageLayout},
		{patterns.ErrCodeRequired, "slides[0].content[0].placeholder_id", preflightStagePlaceholder},
		{diagnostics.CodeInvalidGrid, "slides[0].shape_grid", preflightStageGrid},
		{patterns.ErrCodePlaceholderNotFound, "slides[0].content[0].placeholder_id", preflightStagePlaceholder},
		{patterns.ErrCodeMaxLength, "slides[0].content[0].text", preflightStagePlaceholder},
	}
	for _, tc := range cases {
		got := classifySlideValidationStage(diagnostics.Diagnostic{Code: tc.code, Path: tc.path})
		if got != tc.want {
			t.Errorf("classifySlideValidationStage(code=%q, path=%q) = %q, want %q", tc.code, tc.path, got, tc.want)
		}
	}
}

func TestPreflightExitCode(t *testing.T) {
	clean := diagnostics.FindingEnvelope{OK: true}
	if got := preflightExitCode(clean, false); got != 0 {
		t.Errorf("clean non-strict: got %d, want 0", got)
	}
	if got := preflightExitCode(clean, true); got != 0 {
		t.Errorf("clean strict: got %d, want 0", got)
	}

	withError := diagnostics.FindingEnvelope{OK: false, Findings: []diagnostics.Finding{{Severity: diagnostics.SeverityError}}}
	if got := preflightExitCode(withError, false); got != 2 {
		t.Errorf("error finding: got %d, want 2", got)
	}

	warnOnly := diagnostics.FindingEnvelope{OK: true, Findings: []diagnostics.Finding{{Severity: diagnostics.SeverityWarning}}}
	if got := preflightExitCode(warnOnly, false); got != 0 {
		t.Errorf("warning non-strict: got %d, want 0", got)
	}
	if got := preflightExitCode(warnOnly, true); got != 2 {
		t.Errorf("warning strict: got %d, want 2", got)
	}
}
