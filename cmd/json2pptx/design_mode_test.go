package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDesignMode_ConstrainedRejectsHexFill(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{
									Shape: &ShapeSpecInput{
										Geometry: "roundRect",
										Fill:     json.RawMessage(`"#FF0000"`),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	findings := validateDesignMode(input)
	if len(findings) == 0 {
		t.Fatal("expected design_mode_violation for raw hex fill, got none")
	}
	if findings[0].Code != "design_mode_violation" {
		t.Errorf("expected code design_mode_violation, got %q", findings[0].Code)
	}
	if findings[0].Action != "refuse" {
		t.Errorf("expected action refuse, got %q", findings[0].Action)
	}
}

func TestValidateDesignMode_ConstrainedAllowsSchemeColor(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{
									Shape: &ShapeSpecInput{
										Geometry: "roundRect",
										Fill:     json.RawMessage(`"accent1"`),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	findings := validateDesignMode(input)
	if len(findings) != 0 {
		t.Errorf("expected no violations for scheme color fill, got %d: %v", len(findings), findings)
	}
}

func TestValidateDesignMode_FreeAllowsHex(t *testing.T) {
	input := &PresentationInput{
		Template:   "midnight-blue",
		DesignMode: "free",
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{
									Shape: &ShapeSpecInput{
										Geometry: "roundRect",
										Fill:     json.RawMessage(`"#FF0000"`),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	findings := validateDesignMode(input)
	if len(findings) != 0 {
		t.Errorf("expected no violations in free mode, got %d", len(findings))
	}
}

func TestValidateDesignMode_ConstrainedRejectsAbsoluteSize(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{
									Shape: &ShapeSpecInput{
										Geometry: "roundRect",
										Text:     json.RawMessage(`{"content": "Hello", "size": 24}`),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	findings := validateDesignMode(input)
	if len(findings) == 0 {
		t.Fatal("expected design_mode_violation for absolute size, got none")
	}

	found := false
	for _, f := range findings {
		if f.Path == "shape_grid.rows[0].cells[0].shape.text.size" {
			found = true
		}
	}
	if !found {
		t.Error("expected violation for text.size path")
	}
}

func TestValidateDesignMode_ConstrainedRejectsHexTextColor(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{
									Shape: &ShapeSpecInput{
										Geometry: "roundRect",
										Text:     json.RawMessage(`{"content": "Hello", "color": "#FFFFFF"}`),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	findings := validateDesignMode(input)
	if len(findings) == 0 {
		t.Fatal("expected design_mode_violation for hex text color, got none")
	}
	if findings[0].Path != "shape_grid.rows[0].cells[0].shape.text.color" {
		t.Errorf("unexpected path: %s", findings[0].Path)
	}
}

func TestDesignModeDiagnostics_IncludesNextToolCall(t *testing.T) {
	input := &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{
									Shape: &ShapeSpecInput{
										Geometry: "roundRect",
										Fill:     json.RawMessage(`"#FF0000"`),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	violations := validateDesignMode(input)
	if len(violations) == 0 {
		t.Fatal("expected violations, got none")
	}

	diags := designModeDiagnostics(violations)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics, got none")
	}

	d := diags[0]
	if d.Code != "design_mode_violation" {
		t.Errorf("code = %q, want design_mode_violation", d.Code)
	}
	if d.NextToolCall == nil {
		t.Fatal("NextToolCall is nil, want non-nil")
	}
	if d.NextToolCall.Tool != "generate_presentation" {
		t.Errorf("NextToolCall.Tool = %q, want generate_presentation", d.NextToolCall.Tool)
	}
	if dm, ok := d.NextToolCall.ArgsTemplate["design_mode"].(string); !ok || dm != "free" {
		t.Errorf("NextToolCall.ArgsTemplate[design_mode] = %v, want \"free\"", d.NextToolCall.ArgsTemplate["design_mode"])
	}
	if d.Fix == nil {
		t.Fatal("Fix is nil, want non-nil")
	}
	if d.Fix.Kind != "use_semantic_color" {
		t.Errorf("Fix.Kind = %q, want use_semantic_color", d.Fix.Kind)
	}
}

func TestSuggestNearestSchemeColor(t *testing.T) {
	tests := []struct {
		hex  string
		want string
	}{
		{"FF0000", "accent2"}, // red -> closest to orange accent
		{"000000", "dk1"},     // black -> dk1
		{"FFFFFF", "lt1"},     // white -> lt1
		{"4472C4", "accent1"}, // blue -> accent1
	}

	for _, tc := range tests {
		got := suggestNearestSchemeColor(tc.hex)
		if got != tc.want {
			t.Errorf("suggestNearestSchemeColor(%q) = %q, want %q", tc.hex, got, tc.want)
		}
	}
}

func TestEffectiveDesignMode_DefaultsToConstrained(t *testing.T) {
	input := &PresentationInput{Template: "test"}
	if got := effectiveDesignMode(input); got != "constrained" {
		t.Errorf("expected constrained, got %q", got)
	}
}

func TestEffectiveDesignMode_RespectsExplicitFree(t *testing.T) {
	input := &PresentationInput{Template: "test", DesignMode: "free"}
	if got := effectiveDesignMode(input); got != "free" {
		t.Errorf("expected free, got %q", got)
	}
}

// TestParseJSONInput_DesignModeOverride verifies the CLI --design-mode flag
// override replaces the JSON field after parsing. This is what lets users
// generate decks whose JSON declares design_mode:"constrained" by opting
// into "free" at the command line without editing the file.
func TestParseJSONInput_DesignModeOverride(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/deck.json"
	body := `{"template":"test","design_mode":"constrained","slides":[{"layout_id":"L1"}]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	input, _, err := parseJSONInput(path, "", "free", false)
	if err != nil {
		t.Fatalf("parseJSONInput: %v", err)
	}
	if input.DesignMode != "free" {
		t.Errorf("expected design_mode override 'free', got %q", input.DesignMode)
	}
}

// TestValidateSlideDesignMode_SingleSlide verifies that the per-slide helper
// inspects only the slide passed in (and is independent of deck-level mode,
// since the caller is expected to gate on effectiveDesignMode).
func TestValidateSlideDesignMode_SingleSlide(t *testing.T) {
	slide := SlideInput{
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{
				{Cells: []*GridCellInput{
					{Shape: &ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(`"#FF0000"`),
					}},
				}},
			},
		},
	}

	findings := validateSlideDesignMode(&slide, 7)
	if len(findings) == 0 {
		t.Fatal("expected violation for raw hex fill, got none")
	}
	if !strings.Contains(findings[0].Message, "slide 7") {
		t.Errorf("expected message to reference slide 7, got %q", findings[0].Message)
	}
}

// TestRunJSONMode_PartialSkipsDesignModeViolations verifies the bug fix:
// --partial mode should skip slides with constrained-mode violations (raw hex
// colors / absolute font sizes) and warn, rather than aborting the whole deck.
func TestRunJSONMode_PartialSkipsDesignModeViolations(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "deck.json")
	outputJSON := filepath.Join(tmpDir, "result.json")
	templatesDir := filepath.Join("..", "..", "templates")

	// Two slides: slide 1 has a raw hex fill (violation), slide 2 is clean.
	input := `{
		"template": "midnight-blue",
		"output_filename": "out.pptx",
		"slides": [
			{
				"layout_id": "title",
				"shape_grid": {
					"rows": [
						{"cells": [{"shape": {"geometry": "rect", "fill": "#FF0000"}}]}
					]
				}
			},
			{
				"layout_id": "title",
				"content": [
					{"placeholder_id": "title", "type": "text", "text_value": "Clean Slide"}
				]
			}
		]
	}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	// partial=true: must succeed, skip slide 1, generate slide 2
	err := runJSONMode(jsonPath, outputJSON, templatesDir, tmpDir, "", false, false, "", "off", true, "off", "", false)
	if err != nil {
		t.Fatalf("runJSONMode (partial=true): unexpected error: %v", err)
	}

	resultBytes, err := os.ReadFile(outputJSON)
	if err != nil {
		t.Fatalf("read result JSON: %v", err)
	}
	var result JSONOutput
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success=true in partial mode, got error=%q", result.Error)
	}

	// Verify a per-slide warning about the skipped slide was emitted.
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "slide 1") && strings.Contains(w, "skipped (partial mode)") && strings.Contains(w, "design_mode") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected warning about slide 1 being skipped in partial mode for design_mode violation, got warnings: %v", result.Warnings)
	}
}

// TestRunJSONMode_PartialDropWarningsSurfacedOnStdout verifies that when no
// -json-output path is given (the human-readable CLI summary path), partial-mode
// dropped-slide warnings are still printed via the logger. Regression for
// go-slide-creator-nvdz: --partial silently produced fewer slides than the input
// with no message naming the dropped slides.
func TestRunJSONMode_PartialDropWarningsSurfacedOnStdout(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "deck.json")
	templatesDir := filepath.Join("..", "..", "templates")

	// Slide 1 has a raw hex fill (constrained-mode violation -> dropped in partial
	// mode); slide 2 is clean and renders.
	input := `{
		"template": "midnight-blue",
		"output_filename": "out.pptx",
		"slides": [
			{
				"layout_id": "title",
				"shape_grid": {
					"rows": [
						{"cells": [{"shape": {"geometry": "rect", "fill": "#FF0000"}}]}
					]
				}
			},
			{
				"layout_id": "title",
				"content": [
					{"placeholder_id": "title", "type": "text", "text_value": "Clean Slide"}
				]
			}
		]
	}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	// Capture the default logger output (the stdout-summary path logs via slog).
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)

	// jsonOutputPath is empty -> exercises the human-readable summary branch.
	err := runJSONMode(jsonPath, "", templatesDir, tmpDir, "", false, false, "", "off", true, "off", "", false)
	if err != nil {
		t.Fatalf("runJSONMode (partial=true): unexpected error: %v", err)
	}

	logged := buf.String()
	if !strings.Contains(logged, "slide 1") || !strings.Contains(logged, "skipped (partial mode)") {
		t.Errorf("expected logged warning naming the dropped slide, got logger output:\n%s", logged)
	}
}

// TestRunJSONMode_NoPartialAbortsOnDesignModeViolation verifies that without
// --partial, a constrained-mode violation still aborts the deck (existing
// behavior preserved).
func TestRunJSONMode_NoPartialAbortsOnDesignModeViolation(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "deck.json")
	outputJSON := filepath.Join(tmpDir, "result.json")
	templatesDir := filepath.Join("..", "..", "templates")

	input := `{
		"template": "midnight-blue",
		"output_filename": "out.pptx",
		"slides": [
			{
				"layout_id": "title",
				"shape_grid": {
					"rows": [
						{"cells": [{"shape": {"geometry": "rect", "fill": "#FF0000"}}]}
					]
				}
			}
		]
	}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	// partial=false: must return error.
	err := runJSONMode(jsonPath, outputJSON, templatesDir, tmpDir, "", false, false, "", "off", false, "off", "", false)
	if err == nil {
		t.Fatal("expected runJSONMode to return error without --partial on design_mode violation")
	}
}

func TestParseJSONInput_DesignModeOverrideEmptyPreservesJSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/deck.json"
	body := `{"template":"test","design_mode":"free","slides":[{"layout_id":"L1"}]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	input, _, err := parseJSONInput(path, "", "", false)
	if err != nil {
		t.Fatalf("parseJSONInput: %v", err)
	}
	if input.DesignMode != "free" {
		t.Errorf("expected JSON field preserved as 'free', got %q", input.DesignMode)
	}
}
