package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// TestParseTheme_StandardTemplate tests theme extraction from a standard PPTX template.
// AC5: Theme Extraction - Given template with custom theme colors, when analyzed,
// then ThemeInfo.Colors contains accent colors.
func TestParseTheme_StandardTemplate(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("Failed to open test template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	theme := ParseTheme(reader)

	// Verify theme has a name
	if theme.Name == "" {
		t.Error("Expected theme to have a name")
	}

	// Verify colors were extracted
	if len(theme.Colors) == 0 {
		t.Fatal("Expected theme to have colors")
	}

	// Verify specific accent colors are present
	accentColors := 0
	for _, color := range theme.Colors {
		if len(color.Name) >= 6 && color.Name[:6] == "accent" {
			accentColors++

			// Verify RGB format
			if len(color.RGB) != 7 || color.RGB[0] != '#' {
				t.Errorf("Color %s has invalid RGB format: %s (expected #RRGGBB)", color.Name, color.RGB)
			}
		}
	}

	if accentColors < 6 {
		t.Errorf("Expected at least 6 accent colors, got %d", accentColors)
	}

	// Verify fonts were extracted
	if theme.TitleFont == "" {
		t.Error("Expected theme to have a title font")
	}
	if theme.BodyFont == "" {
		t.Error("Expected theme to have a body font")
	}

	t.Logf("Theme: %s", theme.Name)
	t.Logf("Title Font: %s", theme.TitleFont)
	t.Logf("Body Font: %s", theme.BodyFont)
	t.Logf("Colors: %d", len(theme.Colors))
	for _, color := range theme.Colors {
		t.Logf("  %s: %s", color.Name, color.RGB)
	}
}

// TestParseTheme_MissingTheme tests fallback when theme file is missing.
func TestParseTheme_MissingTheme(t *testing.T) {
	// Create a mock reader that returns no theme files
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("Failed to open test template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// We can't easily test missing theme without modifying the Reader
	// But we can test the default theme function
	defaultTheme := getDefaultTheme()

	if defaultTheme.Name != "Default" {
		t.Errorf("Expected default theme name 'Default', got '%s'", defaultTheme.Name)
	}

	if len(defaultTheme.Colors) == 0 {
		t.Error("Expected default theme to have colors")
	}

	if defaultTheme.TitleFont == "" || defaultTheme.BodyFont == "" {
		t.Error("Expected default theme to have fonts")
	}
}

// TestExtractThemeColors tests color extraction from color scheme.
func TestExtractThemeColors(t *testing.T) {
	tests := []struct {
		name     string
		scheme   pptx.ColorSchemeXML
		wantMin  int    // Minimum number of colors expected
		wantName string // Name to verify
	}{
		{
			name: "sRGB colors",
			scheme: pptx.ColorSchemeXML{
				Accent1: pptx.ColorDefXML{
					SRGBColor: pptx.SRGBColorXML{Val: "FF0000"},
				},
				Accent2: pptx.ColorDefXML{
					SRGBColor: pptx.SRGBColorXML{Val: "00FF00"},
				},
				Dark1: pptx.ColorDefXML{
					SRGBColor: pptx.SRGBColorXML{Val: "000000"},
				},
			},
			wantMin:  3,
			wantName: "accent1",
		},
		{
			name: "system colors with lastClr",
			scheme: pptx.ColorSchemeXML{
				Dark1: pptx.ColorDefXML{
					SystemColor: pptx.SysColorXML{
						Val:     "windowText",
						LastClr: "000000",
					},
				},
				Light1: pptx.ColorDefXML{
					SystemColor: pptx.SysColorXML{
						Val:     "window",
						LastClr: "FFFFFF",
					},
				},
			},
			wantMin:  2,
			wantName: "dk1",
		},
		{
			name: "mixed color types",
			scheme: pptx.ColorSchemeXML{
				Dark1: pptx.ColorDefXML{
					SystemColor: pptx.SysColorXML{
						Val:     "windowText",
						LastClr: "000000",
					},
				},
				Accent1: pptx.ColorDefXML{
					SRGBColor: pptx.SRGBColorXML{Val: "8C8D86"},
				},
				Accent2: pptx.ColorDefXML{
					SRGBColor: pptx.SRGBColorXML{Val: "E6C069"},
				},
			},
			wantMin:  3,
			wantName: "accent2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colors := extractThemeColors(tt.scheme)

			if len(colors) < tt.wantMin {
				t.Errorf("Expected at least %d colors, got %d", tt.wantMin, len(colors))
			}

			// Verify at least one expected color name exists
			found := false
			for _, color := range colors {
				if color.Name == tt.wantName {
					found = true
					// Verify RGB format
					if len(color.RGB) != 7 || color.RGB[0] != '#' {
						t.Errorf("Color %s has invalid RGB format: %s", color.Name, color.RGB)
					}
					break
				}
			}

			if !found {
				t.Errorf("Expected to find color named '%s'", tt.wantName)
			}
		})
	}
}

// TestExtractRGB tests RGB extraction from different color types.
func TestExtractRGB(t *testing.T) {
	tests := []struct {
		name     string
		colorDef pptx.ColorDefXML
		want     string
	}{
		{
			name: "sRGB color",
			colorDef: pptx.ColorDefXML{
				SRGBColor: pptx.SRGBColorXML{Val: "FF0000"},
			},
			want: "#FF0000",
		},
		{
			name: "sRGB color lowercase",
			colorDef: pptx.ColorDefXML{
				SRGBColor: pptx.SRGBColorXML{Val: "abc123"},
			},
			want: "#ABC123",
		},
		{
			name: "system color with lastClr",
			colorDef: pptx.ColorDefXML{
				SystemColor: pptx.SysColorXML{
					Val:     "windowText",
					LastClr: "000000",
				},
			},
			want: "#000000",
		},
		{
			name: "system color with lastClr and hash",
			colorDef: pptx.ColorDefXML{
				SystemColor: pptx.SysColorXML{
					Val:     "window",
					LastClr: "#FFFFFF",
				},
			},
			want: "#FFFFFF",
		},
		{
			name: "sRGB takes precedence",
			colorDef: pptx.ColorDefXML{
				SRGBColor: pptx.SRGBColorXML{Val: "FF0000"},
				SystemColor: pptx.SysColorXML{
					Val:     "windowText",
					LastClr: "000000",
				},
			},
			want: "#FF0000",
		},
		{
			name:     "no color",
			colorDef: pptx.ColorDefXML{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRGB(tt.colorDef)
			if got != tt.want {
				t.Errorf("extractRGB() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNormalizeRGB tests RGB normalization.
func TestNormalizeRGB(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already normalized",
			input: "#FF0000",
			want:  "#FF0000",
		},
		{
			name:  "no hash",
			input: "FF0000",
			want:  "#FF0000",
		},
		{
			name:  "lowercase",
			input: "abc123",
			want:  "#ABC123",
		},
		{
			name:  "with whitespace",
			input: " FF0000 ",
			want:  "#FF0000",
		},
		{
			name:  "invalid length",
			input: "FF00",
			want:  "",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "too long",
			input: "FF0000FF",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRGB(tt.input)
			if got != tt.want {
				t.Errorf("normalizeRGB(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGetDefaultTheme tests the default theme fallback.
func TestGetDefaultTheme(t *testing.T) {
	theme := getDefaultTheme()

	if theme.Name != "Default" {
		t.Errorf("Expected default theme name 'Default', got '%s'", theme.Name)
	}

	if len(theme.Colors) < 6 {
		t.Errorf("Expected at least 6 colors in default theme, got %d", len(theme.Colors))
	}

	if theme.TitleFont == "" {
		t.Error("Expected default theme to have a title font")
	}

	if theme.BodyFont == "" {
		t.Error("Expected default theme to have a body font")
	}

	// Verify all colors have valid RGB format
	for _, color := range theme.Colors {
		if len(color.RGB) != 7 || color.RGB[0] != '#' {
			t.Errorf("Default theme color %s has invalid RGB format: %s", color.Name, color.RGB)
		}
	}
}

// TestThemeIntegration tests theme parsing as part of full template analysis.
func TestThemeIntegration(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("Failed to open test template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Parse theme
	theme := ParseTheme(reader)

	// Verify theme is valid
	if theme.Name == "" {
		t.Error("Theme should have a name")
	}

	// Verify we have both dark and light colors for contrast
	hasDark := false
	hasLight := false
	hasAccent := false

	for _, color := range theme.Colors {
		if color.Name == "dk1" || color.Name == "dk2" {
			hasDark = true
		}
		if color.Name == "lt1" || color.Name == "lt2" {
			hasLight = true
		}
		if len(color.Name) >= 6 && color.Name[:6] == "accent" {
			hasAccent = true
		}
	}

	if !hasDark {
		t.Error("Theme should have dark colors (dk1/dk2)")
	}
	if !hasLight {
		t.Error("Theme should have light colors (lt1/lt2)")
	}
	if !hasAccent {
		t.Error("Theme should have accent colors")
	}

	// Verify fonts are not empty
	if theme.TitleFont == "" || theme.BodyFont == "" {
		t.Error("Theme should have both title and body fonts defined")
	}

	t.Logf("Successfully parsed theme '%s' with %d colors", theme.Name, len(theme.Colors))
}

// TestColorNamesPresent verifies all expected color names are present.
func TestColorNamesPresent(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("Failed to open test template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	theme := ParseTheme(reader)

	expectedColorNames := []string{"dk1", "lt1", "dk2", "lt2", "accent1", "accent2"}
	colorMap := make(map[string]bool)

	for _, color := range theme.Colors {
		colorMap[color.Name] = true
	}

	for _, expectedName := range expectedColorNames {
		if !colorMap[expectedName] {
			t.Errorf("Expected color name '%s' not found in theme", expectedName)
		}
	}
}

// TestThemeColorsAreValid verifies all theme colors are properly formatted.
func TestThemeColorsAreValid(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("Failed to open test template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	theme := ParseTheme(reader)

	for _, color := range theme.Colors {
		// Verify name is not empty
		if color.Name == "" {
			t.Error("Color should have a name")
		}

		// Verify RGB is in correct format
		if len(color.RGB) != 7 {
			t.Errorf("Color %s has invalid RGB length: %s (expected 7 chars)", color.Name, color.RGB)
		}

		if color.RGB[0] != '#' {
			t.Errorf("Color %s RGB should start with #: %s", color.Name, color.RGB)
		}

		// Verify hex characters
		for i, c := range color.RGB[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
				t.Errorf("Color %s RGB has invalid character at position %d: %c", color.Name, i+1, c)
			}
		}
	}
}
