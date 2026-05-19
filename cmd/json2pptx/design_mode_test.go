package main

import (
	"encoding/json"
	"os"
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

	input, _, err := parseJSONInput(path, "", "free")
	if err != nil {
		t.Fatalf("parseJSONInput: %v", err)
	}
	if input.DesignMode != "free" {
		t.Errorf("expected design_mode override 'free', got %q", input.DesignMode)
	}
}

func TestParseJSONInput_DesignModeOverrideEmptyPreservesJSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/deck.json"
	body := `{"template":"test","design_mode":"free","slides":[{"layout_id":"L1"}]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	input, _, err := parseJSONInput(path, "", "")
	if err != nil {
		t.Fatalf("parseJSONInput: %v", err)
	}
	if input.DesignMode != "free" {
		t.Errorf("expected JSON field preserved as 'free', got %q", input.DesignMode)
	}
}
