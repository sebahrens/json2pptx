package main

import (
	"testing"

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
	if got := warnings[0]; got == "" {
		t.Error("warning message is empty")
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
			name: "large MaxChars",
			ph: types.PlaceholderInfo{
				ID: "body", Type: types.PlaceholderBody,
				MaxChars: 100, FontSize: 8000,
				Bounds: types.BoundingBox{Y: 500000},
			},
		},
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
