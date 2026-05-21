package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCheckSectionNumberNaming_NoWarningForCorrectName(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			Name: "Section Header",
			Tags: []string{"section-header"},
			Placeholders: []types.PlaceholderInfo{
				{
					ID:       "Section Number",
					Type:     types.PlaceholderBody,
					MaxChars: 2,
					FontSize: 8000, // 80pt
					Bounds:   types.BoundingBox{X: 100000, Y: 500000},
				},
			},
		},
	}
	warnings := checkSectionNumberNaming(layouts)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for correctly-named Section Number, got %v", warnings)
	}
}

func TestCheckSectionNumberNaming_WarnsForMisnamedPlaceholder(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			Name: "Section Divider",
			Tags: []string{"section-header"},
			Placeholders: []types.PlaceholderInfo{
				{
					ID:       "body",
					Type:     types.PlaceholderBody,
					MaxChars: 2,
					FontSize: 8000, // 80pt
					Bounds:   types.BoundingBox{X: 100000, Y: 500000},
				},
			},
		},
	}
	warnings := checkSectionNumberNaming(layouts)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if got := warnings[0].Message; got == "" {
		t.Error("warning message is empty")
	}
	if got := warnings[0].Code; got != diagnostics.CodeTemplateSectionNumberNaming {
		t.Errorf("warning code = %q, want %q", got, diagnostics.CodeTemplateSectionNumberNaming)
	}
	if got := warnings[0].Severity; got != diagnostics.SeverityWarning {
		t.Errorf("warning severity = %q, want %q", got, diagnostics.SeverityWarning)
	}
}

func TestCheckSectionNumberNaming_NoWarningForNonSectionLayout(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			Name: "Content",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{
					ID:       "body",
					Type:     types.PlaceholderBody,
					MaxChars: 2,
					FontSize: 8000,
					Bounds:   types.BoundingBox{X: 100000, Y: 500000},
				},
			},
		},
	}
	warnings := checkSectionNumberNaming(layouts)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for non-section layout, got %v", warnings)
	}
}

func TestCheckSectionNumberNaming_NoWarningWhenBelowThresholds(t *testing.T) {
	tests := []struct {
		name     string
		ph       types.PlaceholderInfo
	}{
		{
			name: "small font",
			ph: types.PlaceholderInfo{
				ID: "body", Type: types.PlaceholderBody,
				MaxChars: 2, FontSize: 1400, // 14pt
				Bounds: types.BoundingBox{Y: 500000},
			},
		},
		{
			name: "lower position",
			ph: types.PlaceholderInfo{
				ID: "body", Type: types.PlaceholderBody,
				MaxChars: 2, FontSize: 8000,
				Bounds: types.BoundingBox{Y: 3000000}, // below upper third
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layouts := []types.LayoutMetadata{
				{
					Name:         "Section Header",
					Tags:         []string{"section-header"},
					Placeholders: []types.PlaceholderInfo{tt.ph},
				},
			}
			warnings := checkSectionNumberNaming(layouts)
			if len(warnings) != 0 {
				t.Errorf("expected no warnings, got %v", warnings)
			}
		})
	}
}

// TestCheckSectionNumberNaming_WarnsForLargeFontDecorativeNumber is the
// acceptance test for go-slide-creator-ksq2: a section-header layout with a
// misnamed body placeholder rendered at 100pt in the top third must produce a
// warning, even though it is not named "Section Number".
func TestCheckSectionNumberNaming_WarnsForLargeFontDecorativeNumber(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			Name: "Section Divider",
			Tags: []string{"section-header"},
			Placeholders: []types.PlaceholderInfo{
				{
					ID:       "TextBox 7",          // decorative number frame, but misnamed
					Type:     types.PlaceholderBody, // normalizer would stuff prose into it
					MaxChars: 2,
					FontSize: 10000,                            // 100pt
					Bounds:   types.BoundingBox{X: 100000, Y: 500000, Width: 914400, Height: 914400},
				},
			},
		},
	}
	warnings := checkSectionNumberNaming(layouts)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for misnamed 100pt decorative number, got %d: %v", len(warnings), warnings)
	}
	if got := warnings[0].Code; got != diagnostics.CodeTemplateSectionNumberNaming {
		t.Errorf("warning code = %q, want %q", got, diagnostics.CodeTemplateSectionNumberNaming)
	}
}

// TestCheckSectionNumberNaming_WarnsForWideBannerNumber covers the safety-net
// loosening: a wide banner-style decorative number frame reports a high MaxChars
// even at large font, so the validator must not gate on MaxChars < 5. The
// large-font + upper-third + body-typed signal alone must trigger the warning.
func TestCheckSectionNumberNaming_WarnsForWideBannerNumber(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			Name: "Section Divider",
			Tags: []string{"section-header"},
			Placeholders: []types.PlaceholderInfo{
				{
					ID:       "Rectangle 4",
					Type:     types.PlaceholderBody,
					MaxChars: 100, // wide frame -> high reported capacity
					FontSize: 8000,
					Bounds:   types.BoundingBox{Y: 500000},
				},
			},
		},
	}
	warnings := checkSectionNumberNaming(layouts)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for wide banner number frame, got %d: %v", len(warnings), warnings)
	}
}

func TestCheckSectionNumberNaming_CaseInsensitiveName(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			Name: "Section Header",
			Tags: []string{"section-header"},
			Placeholders: []types.PlaceholderInfo{
				{
					ID:       "section number", // lowercase
					Type:     types.PlaceholderBody,
					MaxChars: 2,
					FontSize: 8000,
					Bounds:   types.BoundingBox{Y: 500000},
				},
			},
		},
	}
	warnings := checkSectionNumberNaming(layouts)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for case-insensitive match, got %v", warnings)
	}
}

// TestBuildTemplateFindings_EnvelopeShape asserts validate-template folds its
// metadata-validation and section-number diagnostics into a single
// FindingEnvelope: namespaced TPL.* codes, error-before-warning ordering, the
// run-level metadata, and the OK flag derived from error-severity findings.
func TestBuildTemplateFindings_EnvelopeShape(t *testing.T) {
	metaDiags := []diagnostics.Diagnostic{
		{
			Code:     diagnostics.CodeTemplateAspectRatioInvalid,
			Severity: diagnostics.SeverityWarning,
			Message:  "invalid aspect ratio format: 16x9 (expected format like '16:9')",
		},
		{
			Code:     diagnostics.CodeTemplateError,
			Severity: diagnostics.SeverityError,
			Message:  "unexpected metadata failure",
		},
	}
	sectionDiags := []diagnostics.Diagnostic{
		{
			Code:     diagnostics.CodeTemplateSectionNumberNaming,
			Severity: diagnostics.SeverityWarning,
			Message:  `Layout "Section Divider" has a placeholder named "body".`,
		},
	}

	env := buildTemplateFindings("midnight-blue.pptx", metaDiags, sectionDiags)

	if env.SchemaVersion != diagnostics.SchemaVersion {
		t.Errorf("schema_version = %q, want %q", env.SchemaVersion, diagnostics.SchemaVersion)
	}
	if env.Subcommand != "validate-template" {
		t.Errorf("subcommand = %q, want validate-template", env.Subcommand)
	}
	if env.Template != "midnight-blue.pptx" {
		t.Errorf("template = %q, want midnight-blue.pptx", env.Template)
	}
	if env.OK {
		t.Error("ok = true, want false (an error-severity finding is present)")
	}
	if len(env.Findings) != 3 {
		t.Fatalf("findings count = %d, want 3", len(env.Findings))
	}
	// Errors sort before warnings: findings[0] is the error.
	if env.Findings[0].Severity != diagnostics.SeverityError {
		t.Errorf("findings[0].severity = %q, want error", env.Findings[0].Severity)
	}
	// Every finding carries a TPL-namespaced code and category.
	for i, f := range env.Findings {
		if f.Category != diagnostics.NamespaceTemplate {
			t.Errorf("findings[%d].category = %q, want %q", i, f.Category, diagnostics.NamespaceTemplate)
		}
		wantPrefix := diagnostics.NamespaceTemplate + "."
		if len(f.Code) < len(wantPrefix) || f.Code[:len(wantPrefix)] != wantPrefix {
			t.Errorf("findings[%d].code = %q, want %s* prefix", i, f.Code, wantPrefix)
		}
	}
}

// TestBuildTemplateFindings_CleanTemplate asserts a template with no issues
// produces an OK envelope with an empty (non-nil) findings list.
func TestBuildTemplateFindings_CleanTemplate(t *testing.T) {
	env := buildTemplateFindings("forest-green.pptx", nil, nil)
	if !env.OK {
		t.Error("ok = false, want true for a clean template")
	}
	if env.Findings == nil {
		t.Error("findings is nil, want empty non-nil slice")
	}
	if len(env.Findings) != 0 {
		t.Errorf("findings count = %d, want 0", len(env.Findings))
	}
	if env.Summary != "no issues" {
		t.Errorf("summary = %q, want \"no issues\"", env.Summary)
	}
}
