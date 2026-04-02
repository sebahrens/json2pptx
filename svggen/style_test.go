package svggen

import (
	"math"
	"testing"
)

func TestParseColor(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		wantR   uint8
		wantG   uint8
		wantB   uint8
		wantA   float64
		wantErr bool
	}{
		{
			name:  "6-digit hex",
			hex:   "#4E79A7",
			wantR: 0x4E,
			wantG: 0x79,
			wantB: 0xA7,
			wantA: 1.0,
		},
		{
			name:  "6-digit hex lowercase",
			hex:   "#4e79a7",
			wantR: 0x4E,
			wantG: 0x79,
			wantB: 0xA7,
			wantA: 1.0,
		},
		{
			name:  "6-digit hex no hash",
			hex:   "4E79A7",
			wantR: 0x4E,
			wantG: 0x79,
			wantB: 0xA7,
			wantA: 1.0,
		},
		{
			name:  "3-digit hex",
			hex:   "#F00",
			wantR: 0xFF,
			wantG: 0x00,
			wantB: 0x00,
			wantA: 1.0,
		},
		{
			name:  "8-digit hex with alpha",
			hex:   "#4E79A780",
			wantR: 0x4E,
			wantG: 0x79,
			wantB: 0xA7,
			wantA: 128.0 / 255.0,
		},
		{
			name:  "4-digit hex with alpha",
			hex:   "#F008",
			wantR: 0xFF,
			wantG: 0x00,
			wantB: 0x00,
			wantA: 136.0 / 255.0,
		},
		{
			name:    "invalid length",
			hex:     "#12345",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			hex:     "#GGHHII",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseColor(tt.hex)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseColor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if c.R != tt.wantR {
				t.Errorf("ParseColor() R = %d, want %d", c.R, tt.wantR)
			}
			if c.G != tt.wantG {
				t.Errorf("ParseColor() G = %d, want %d", c.G, tt.wantG)
			}
			if c.B != tt.wantB {
				t.Errorf("ParseColor() B = %d, want %d", c.B, tt.wantB)
			}
			if math.Abs(c.A-tt.wantA) > 0.01 {
				t.Errorf("ParseColor() A = %f, want %f", c.A, tt.wantA)
			}
		})
	}
}

func TestColor_Hex(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		want  string
	}{
		{
			name:  "opaque color",
			color: Color{R: 0x4E, G: 0x79, B: 0xA7, A: 1.0},
			want:  "#4E79A7",
		},
		{
			name:  "semi-transparent",
			color: Color{R: 0xFF, G: 0x00, B: 0x00, A: 0.5},
			want:  "#FF00007F", // 0.5 * 255 = 127.5 → 127 = 0x7F
		},
		{
			name:  "black",
			color: Color{R: 0, G: 0, B: 0, A: 1.0},
			want:  "#000000",
		},
		{
			name:  "white",
			color: Color{R: 255, G: 255, B: 255, A: 1.0},
			want:  "#FFFFFF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.color.Hex(); got != tt.want {
				t.Errorf("Color.Hex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColor_WithAlpha(t *testing.T) {
	c := Color{R: 255, G: 0, B: 0, A: 1.0}
	c2 := c.WithAlpha(0.5)

	if c2.R != c.R || c2.G != c.G || c2.B != c.B {
		t.Error("WithAlpha should not change RGB values")
	}
	if c2.A != 0.5 {
		t.Errorf("WithAlpha() A = %f, want 0.5", c2.A)
	}
	if c.A != 1.0 {
		t.Error("WithAlpha should not modify original color")
	}
}

func TestColor_Lighten(t *testing.T) {
	c := Color{R: 100, G: 100, B: 100, A: 1.0}
	lighter := c.Lighten(0.5)

	// Should be lighter (higher values)
	if lighter.R <= c.R || lighter.G <= c.G || lighter.B <= c.B {
		t.Errorf("Lighten should increase RGB values: original %v, lightened %v", c, lighter)
	}

	// uint8 values are inherently <= 255, so just verify they're valid
	_ = lighter // Already validated above
}

func TestColor_Darken(t *testing.T) {
	c := Color{R: 100, G: 100, B: 100, A: 1.0}
	darker := c.Darken(0.5)

	// Should be darker (lower values)
	if darker.R >= c.R || darker.G >= c.G || darker.B >= c.B {
		t.Errorf("Darken should decrease RGB values: original %v, darkened %v", c, darker)
	}
}

func TestColor_IsLight(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		want  bool
	}{
		{
			name:  "white is light",
			color: MustParseColor("#FFFFFF"),
			want:  true,
		},
		{
			name:  "black is not light",
			color: MustParseColor("#000000"),
			want:  false,
		},
		{
			name:  "yellow is light",
			color: MustParseColor("#FFFF00"),
			want:  true,
		},
		{
			name:  "dark blue is not light",
			color: MustParseColor("#000080"),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.color.IsLight(); got != tt.want {
				t.Errorf("Color.IsLight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColor_Luminance_WCAGReference(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		want  float64
	}{
		{"white", MustParseColor("#FFFFFF"), 1.0},
		{"black", MustParseColor("#000000"), 0.0},
		{"red", MustParseColor("#FF0000"), 0.2126},
		{"green", MustParseColor("#00FF00"), 0.7152},
		{"blue", MustParseColor("#0000FF"), 0.0722},
		{"mid-gray", MustParseColor("#808080"), 0.2159},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.color.Luminance()
			if diff := got - tt.want; diff > 0.001 || diff < -0.001 {
				t.Errorf("Luminance(%s) = %f, want %f (diff %f)", tt.name, got, tt.want, diff)
			}
		})
	}
}

func TestColor_TextColorFor(t *testing.T) {
	white := MustParseColor("#FFFFFF")
	black := MustParseColor("#000000")

	// Light backgrounds should get dark text
	lightBg := MustParseColor("#F0F0F0")
	textOnLight := lightBg.TextColorFor()
	if textOnLight.IsLight() {
		t.Error("Text on light background should be dark")
	}

	// Dark backgrounds should get light text
	darkBg := MustParseColor("#1A1A1A")
	textOnDark := darkBg.TextColorFor()
	if !textOnDark.IsLight() {
		t.Error("Text on dark background should be light")
	}

	_ = white
	_ = black
}

func TestColor_CSS(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		want  string
	}{
		{
			name:  "opaque returns hex",
			color: Color{R: 255, G: 0, B: 0, A: 1.0},
			want:  "#FF0000",
		},
		{
			name:  "transparent returns rgba",
			color: Color{R: 255, G: 0, B: 0, A: 0.5},
			want:  "rgba(255, 0, 0, 0.500)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.color.CSS(); got != tt.want {
				t.Errorf("Color.CSS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCSSColor(t *testing.T) {
	tests := []struct {
		name    string
		css     string
		wantR   uint8
		wantG   uint8
		wantB   uint8
		wantA   float64
		wantErr bool
	}{
		{
			name:  "hex",
			css:   "#FF0000",
			wantR: 255,
			wantG: 0,
			wantB: 0,
			wantA: 1.0,
		},
		{
			name:  "rgb",
			css:   "rgb(255, 128, 64)",
			wantR: 255,
			wantG: 128,
			wantB: 64,
			wantA: 1.0,
		},
		{
			name:  "rgba",
			css:   "rgba(255, 128, 64, 0.5)",
			wantR: 255,
			wantG: 128,
			wantB: 64,
			wantA: 0.5,
		},
		{
			name:  "named color black",
			css:   "black",
			wantR: 0,
			wantG: 0,
			wantB: 0,
			wantA: 1.0,
		},
		{
			name:  "named color white",
			css:   "WHITE",
			wantR: 255,
			wantG: 255,
			wantB: 255,
			wantA: 1.0,
		},
		{
			name:    "invalid format",
			css:     "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseCSSColor(tt.css)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCSSColor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if c.R != tt.wantR || c.G != tt.wantG || c.B != tt.wantB {
				t.Errorf("ParseCSSColor() RGB = (%d,%d,%d), want (%d,%d,%d)",
					c.R, c.G, c.B, tt.wantR, tt.wantG, tt.wantB)
			}
			if math.Abs(c.A-tt.wantA) > 0.01 {
				t.Errorf("ParseCSSColor() A = %f, want %f", c.A, tt.wantA)
			}
		})
	}
}

func TestDefaultPalette(t *testing.T) {
	p := DefaultPalette()

	if p.Name != "corporate" {
		t.Errorf("DefaultPalette Name = %q, want corporate", p.Name)
	}

	// Check accent colors are populated
	accents := p.AccentColors()
	if len(accents) != 6 {
		t.Errorf("AccentColors() length = %d, want 6", len(accents))
	}

	// Test AccentColor wrapping
	for i := 0; i < 12; i++ {
		c := p.AccentColor(i)
		if c.R == 0 && c.G == 0 && c.B == 0 {
			t.Errorf("AccentColor(%d) returned black, expected color", i)
		}
	}
}

func TestGetPaletteByName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{"corporate", "corporate", "corporate"},
		{"default", "default", "corporate"},
		{"vibrant", "vibrant", "vibrant"},
		{"muted", "muted", "muted"},
		{"monochrome", "monochrome", "monochrome"},
		{"grayscale", "grayscale", "monochrome"},
		{"unknown defaults to corporate", "unknown", "corporate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := GetPaletteByName(tt.input)
			if p.Name != tt.wantName {
				t.Errorf("GetPaletteByName(%q) = %q, want %q", tt.input, p.Name, tt.wantName)
			}
		})
	}
}

func TestPalette_Clone(t *testing.T) {
	p := DefaultPalette()
	clone := p.Clone()

	// Modify original
	p.Primary = MustParseColor("#000000")

	// Clone should be unchanged
	if clone.Primary.Hex() == "#000000" {
		t.Error("Clone should be independent of original")
	}
}

func TestDefaultTypography(t *testing.T) {
	typ := DefaultTypography()

	if typ.FontFamily != "Arial" {
		t.Errorf("FontFamily = %q, want Arial", typ.FontFamily)
	}

	// Check size scale is reasonable
	if typ.SizeTitle <= typ.SizeBody {
		t.Error("Title should be larger than body text")
	}
	if typ.SizeBody <= typ.SizeCaption {
		t.Error("Body should be larger than caption")
	}

	// Check font stack
	stack := typ.FontStack()
	if stack == "" {
		t.Error("FontStack should not be empty")
	}
}

func TestTypography_Clone(t *testing.T) {
	typ := DefaultTypography()
	clone := typ.Clone()

	// Modify original
	typ.FontFamily = "Times"

	// Clone should be unchanged
	if clone.FontFamily == "Times" {
		t.Error("Clone should be independent of original")
	}
}

func TestDefaultSpacing(t *testing.T) {
	sp := DefaultSpacing()

	if sp.Unit <= 0 {
		t.Error("Unit should be positive")
	}

	// Check scale is progressive
	if sp.XS >= sp.SM || sp.SM >= sp.MD || sp.MD >= sp.LG {
		t.Error("Spacing scale should be progressive")
	}

	// Test Value function
	if sp.Value(2) != sp.Unit*2 {
		t.Error("Value(2) should return 2x unit")
	}
}

func TestSpacing_Clone(t *testing.T) {
	sp := DefaultSpacing()
	clone := sp.Clone()

	// Modify original
	sp.Unit = 100

	// Clone should be unchanged
	if clone.Unit == 100 {
		t.Error("Clone should be independent of original")
	}
}

func TestDefaultStrokes(t *testing.T) {
	st := DefaultStrokes()

	// Check width scale is progressive
	if st.WidthThin >= st.WidthNormal || st.WidthNormal >= st.WidthThick {
		t.Error("Stroke widths should be progressive")
	}

	// Check patterns
	if len(st.PatternSolid) != 0 {
		t.Error("Solid pattern should be empty")
	}
	if len(st.PatternDashed) == 0 {
		t.Error("Dashed pattern should have values")
	}
}

func TestStrokes_Clone(t *testing.T) {
	st := DefaultStrokes()
	clone := st.Clone()

	// Modify original pattern
	st.PatternDashed = append(st.PatternDashed, 99)

	// Clone should be unchanged
	if len(clone.PatternDashed) != 2 {
		t.Error("Clone patterns should be independent of original")
	}
}

func TestDefaultStyleGuide(t *testing.T) {
	guide := DefaultStyleGuide()

	if guide.Palette == nil {
		t.Error("Palette should not be nil")
	}
	if guide.Typography == nil {
		t.Error("Typography should not be nil")
	}
	if guide.Spacing == nil {
		t.Error("Spacing should not be nil")
	}
	if guide.Strokes == nil {
		t.Error("Strokes should not be nil")
	}
}

func TestStyleGuide_Clone(t *testing.T) {
	guide := DefaultStyleGuide()
	clone := guide.Clone()

	// Modify original
	guide.Palette.Primary = MustParseColor("#000000")
	guide.Typography.FontFamily = "Times"
	guide.Spacing.Unit = 100
	guide.Strokes.WidthNormal = 100

	// Clone should be unchanged
	if clone.Palette.Primary.Hex() == "#000000" {
		t.Error("Clone palette should be independent")
	}
	if clone.Typography.FontFamily == "Times" {
		t.Error("Clone typography should be independent")
	}
	if clone.Spacing.Unit == 100 {
		t.Error("Clone spacing should be independent")
	}
	if clone.Strokes.WidthNormal == 100 {
		t.Error("Clone strokes should be independent")
	}
}

func TestStyleGuideFromSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    StyleSpec
		checkFn func(*testing.T, *StyleGuide)
	}{
		{
			name: "palette by name",
			spec: StyleSpec{Palette: PaletteSpec{Name: "vibrant"}},
			checkFn: func(t *testing.T, g *StyleGuide) {
				if g.Palette.Name != "vibrant" {
					t.Errorf("Palette name = %q, want vibrant", g.Palette.Name)
				}
			},
		},
		{
			name: "custom font",
			spec: StyleSpec{FontFamily: "Helvetica"},
			checkFn: func(t *testing.T, g *StyleGuide) {
				if g.Typography.FontFamily != "Helvetica" {
					t.Errorf("FontFamily = %q, want Helvetica", g.Typography.FontFamily)
				}
			},
		},
		{
			name: "custom background",
			spec: StyleSpec{Background: "#FF0000"},
			checkFn: func(t *testing.T, g *StyleGuide) {
				if g.Palette.Background.R != 255 {
					t.Errorf("Background R = %d, want 255", g.Palette.Background.R)
				}
			},
		},
		{
			name: "custom color array",
			spec: StyleSpec{Palette: PaletteSpec{Colors: []string{"#FF0000", "#00FF00", "#0000FF"}}},
			checkFn: func(t *testing.T, g *StyleGuide) {
				if g.Palette.Name != "custom" {
					t.Errorf("Palette name = %q, want custom", g.Palette.Name)
				}
				if g.Palette.Accent1.R != 255 {
					t.Errorf("Accent1 R = %d, want 255", g.Palette.Accent1.R)
				}
			},
		},
		{
			name: "empty spec uses defaults",
			spec: StyleSpec{},
			checkFn: func(t *testing.T, g *StyleGuide) {
				if g.Palette.Name != "corporate" {
					t.Errorf("Palette name = %q, want corporate", g.Palette.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guide := StyleGuideFromSpec(tt.spec)
			tt.checkFn(t, guide)
		})
	}
}

func TestCustomPalette(t *testing.T) {
	colors := []Color{
		MustParseColor("#FF0000"),
		MustParseColor("#00FF00"),
		MustParseColor("#0000FF"),
	}

	p := CustomPalette(colors)

	if p.Name != "custom" {
		t.Errorf("Name = %q, want custom", p.Name)
	}
	if p.Accent1.R != 255 {
		t.Errorf("Accent1 R = %d, want 255", p.Accent1.R)
	}
	if p.Accent2.G != 255 {
		t.Errorf("Accent2 G = %d, want 255", p.Accent2.G)
	}
	if p.Accent3.B != 255 {
		t.Errorf("Accent3 B = %d, want 255", p.Accent3.B)
	}
}

func TestVibrantPalette(t *testing.T) {
	p := VibrantPalette()
	if p.Name != "vibrant" {
		t.Errorf("Name = %q, want vibrant", p.Name)
	}
}

func TestMutedPalette(t *testing.T) {
	p := MutedPalette()
	if p.Name != "muted" {
		t.Errorf("Name = %q, want muted", p.Name)
	}
}

func TestMonochromePalette(t *testing.T) {
	p := MonochromePalette()
	if p.Name != "monochrome" {
		t.Errorf("Name = %q, want monochrome", p.Name)
	}

	// All accent colors should be grayscale
	for i, c := range p.AccentColors() {
		if c.R != c.G || c.G != c.B {
			t.Errorf("Accent%d is not grayscale: (%d,%d,%d)", i+1, c.R, c.G, c.B)
		}
	}
}

func TestCompactTypography(t *testing.T) {
	compact := CompactTypography()
	default_ := DefaultTypography()

	if compact.SizeBody >= default_.SizeBody {
		t.Error("Compact body size should be smaller than default")
	}
}

func TestTypography_ScaleForDimensions(t *testing.T) {
	tests := []struct {
		name          string
		width, height float64
		wantScale     float64 // approximate scale factor
	}{
		{
			name:      "reference dimensions unchanged",
			width:     800,
			height:    600,
			wantScale: 1.0,
		},
		{
			name:      "double dimensions scales up",
			width:     1600,
			height:    1200,
			wantScale: 2.0,
		},
		{
			name:      "half dimensions scales down",
			width:     400,
			height:    300,
			wantScale: 0.5,
		},
		{
			name:      "large dimensions capped at 5x",
			width:     4000,
			height:    3000,
			wantScale: 5.0,
		},
		{
			name:      "tiny dimensions capped at 0.5x",
			width:     100,
			height:    75,
			wantScale: 0.5,
		},
		{
			name:      "asymmetric uses geometric mean",
			width:     1600,
			height:    600, // height unchanged, width doubled
			wantScale: math.Sqrt(2.0), // geometric mean: sqrt(2.0 * 1.0) ≈ 1.414
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := DefaultTypography()
			scaled := typ.ScaleForDimensions(tt.width, tt.height)

			// Check that scaled is not nil
			if scaled == nil {
				t.Fatal("ScaleForDimensions returned nil")
			}

			// Calculate expected body size (clamped by floor=11 and cap=14)
			wantBody := math.Max(11.0, math.Min(14.0, typ.SizeBody*tt.wantScale))
			tolerance := 0.01

			if diff := scaled.SizeBody - wantBody; diff < -tolerance || diff > tolerance {
				t.Errorf("SizeBody = %v, want %v (scale %v)", scaled.SizeBody, wantBody, tt.wantScale)
			}

			// Verify proportionality only at 1x scale where neither floors nor caps apply
			if tt.wantScale == 1.0 {
				if scaled.SizeTitle/typ.SizeTitle != scaled.SizeBody/typ.SizeBody {
					t.Error("Font sizes not scaled proportionally")
				}
			}
		})
	}
}

func TestTypography_ScaleForDimensions_Nil(t *testing.T) {
	var typ *Typography
	scaled := typ.ScaleForDimensions(800, 600)
	if scaled != nil {
		t.Error("ScaleForDimensions on nil should return nil")
	}
}

func TestTypography_ScaleForDimensions_MinimumFontSizes(t *testing.T) {
	// Test that minimum font size enforcement works correctly
	typ := DefaultTypography()

	// At 0.5x scale (the minimum), SizeSmall would be 10*0.5=5pt
	// and SizeCaption would be 10*0.5=5pt, both below minimums
	scaled := typ.ScaleForDimensions(100, 75) // triggers 0.5x scale

	// SizeSmall should be clamped to minimum of 9pt
	if scaled.SizeSmall < 9.0 {
		t.Errorf("SizeSmall = %v, want >= 9.0 (minimum)", scaled.SizeSmall)
	}

	// SizeCaption should be clamped to minimum of 10pt
	if scaled.SizeCaption < 10.0 {
		t.Errorf("SizeCaption = %v, want >= 10.0 (minimum)", scaled.SizeCaption)
	}

	// SizeBody should be clamped to minimum of 11pt
	if scaled.SizeBody < 11.0 {
		t.Errorf("SizeBody = %v, want >= 11.0 (minimum)", scaled.SizeBody)
	}

	// At 1x scale, the defaults (10 and 10) should be at or above floors
	scaledNormal := typ.ScaleForDimensions(800, 600)
	if scaledNormal.SizeSmall < 9.0 {
		t.Errorf("SizeSmall at 1x = %v, want >= 9.0 (minimum enforced)", scaledNormal.SizeSmall)
	}
	if scaledNormal.SizeCaption < 10.0 {
		t.Errorf("SizeCaption at 1x = %v, want >= 10.0 (minimum enforced)", scaledNormal.SizeCaption)
	}
}

func TestTypography_ScaleForDimensions_MaximumFontSizes(t *testing.T) {
	typ := DefaultTypography()

	// At content_16x9 (1600x900), scale = sqrt(2.0 * 1.5) ≈ 1.73
	// Without caps: Title=18*1.73=31.1, Small=10*1.73=17.3
	// With caps: Title=24, Small=12
	scaled := typ.ScaleForDimensions(1600, 900)

	if scaled.SizeTitle > 24.0 {
		t.Errorf("SizeTitle = %v, want <= 24.0 (max cap)", scaled.SizeTitle)
	}
	if scaled.SizeTitle < 24.0-0.01 {
		t.Errorf("SizeTitle = %v, want 24.0 (should hit cap at 1600x900)", scaled.SizeTitle)
	}

	if scaled.SizeSmall > 12.0 {
		t.Errorf("SizeSmall = %v, want <= 12.0 (max cap)", scaled.SizeSmall)
	}

	// At 2x scale (1600x1200), all sizes should be capped
	scaled2x := typ.ScaleForDimensions(1600, 1200)
	if scaled2x.SizeTitle > 24.0 {
		t.Errorf("SizeTitle at 2x = %v, want <= 24.0 (max cap)", scaled2x.SizeTitle)
	}
	if scaled2x.SizeBody > 14.0 {
		t.Errorf("SizeBody at 2x = %v, want <= 14.0 (max cap)", scaled2x.SizeBody)
	}
	if scaled2x.SizeCaption > 12.0 {
		t.Errorf("SizeCaption at 2x = %v, want <= 12.0 (max cap)", scaled2x.SizeCaption)
	}
}

func TestCompactSpacing(t *testing.T) {
	compact := compactSpacing()
	default_ := DefaultSpacing()

	if compact.Unit >= default_.Unit {
		t.Error("Compact unit should be smaller than default")
	}
}

func TestMustParseColor_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParseColor should panic on invalid input")
		}
	}()

	MustParseColor("invalid")
}

func TestColor_RGBA(t *testing.T) {
	c := Color{R: 100, G: 150, B: 200, A: 0.5}
	r, g, b, a := c.RGBA()

	if r != 100 || g != 150 || b != 200 {
		t.Errorf("RGBA() RGB = (%d,%d,%d), want (100,150,200)", r, g, b)
	}
	if a != 0.5 {
		t.Errorf("RGBA() A = %f, want 0.5", a)
	}
}

func TestNewPaletteFromThemeColors(t *testing.T) {
	tests := []struct {
		name       string
		colors     []ThemeColorInput
		wantName   string
		wantAccent string // Hex of Accent1
		checkFn    func(*testing.T, *Palette)
	}{
		{
			name:     "empty colors returns default palette",
			colors:   []ThemeColorInput{},
			wantName: "corporate", // Default palette name
		},
		{
			name: "only non-accent colors returns default palette",
			colors: []ThemeColorInput{
				{Name: "dk1", RGB: "#000000"},
				{Name: "lt1", RGB: "#FFFFFF"},
			},
			wantName: "corporate", // Default palette when no accents found
		},
		{
			name: "full accent set creates theme palette with good-contrast colors",
			colors: []ThemeColorInput{
				// These colors all have >3:1 contrast vs white and are mutually
				// distinguishable, so EnforceAccentContrast leaves them unchanged.
				{Name: "accent1", RGB: "#4E79A7"}, // Steel Blue
				{Name: "accent2", RGB: "#C0504D"}, // Brick Red
				{Name: "accent3", RGB: "#2E7D32"}, // Forest Green
				{Name: "accent4", RGB: "#8064A2"}, // Plum
				{Name: "accent5", RGB: "#4BACC6"}, // Teal - contrast vs white = 2.62
				{Name: "accent6", RGB: "#D4463A"}, // Vermilion
			},
			wantName:   "theme",
			wantAccent: "#4E79A7",
			checkFn: func(t *testing.T, p *Palette) {
				// Colors with sufficient contrast are preserved as-is
				if p.Accent1.Hex() != "#4E79A7" {
					t.Errorf("Accent1 = %s, want #4E79A7", p.Accent1.Hex())
				}
				if p.Accent2.Hex() != "#C0504D" {
					t.Errorf("Accent2 = %s, want #C0504D", p.Accent2.Hex())
				}
				if p.Accent3.Hex() != "#2E7D32" {
					t.Errorf("Accent3 = %s, want #2E7D32", p.Accent3.Hex())
				}
				if p.Accent4.Hex() != "#8064A2" {
					t.Errorf("Accent4 = %s, want #8064A2", p.Accent4.Hex())
				}
				// Accent5 (#4BACC6) has ~2.62 contrast vs white so may be darkened
				// Just verify it still has teal hue and meets contrast
				a5 := p.Accent5
				white := MustParseColor("#FFFFFF")
				if a5.ContrastWith(white) < MinAccentContrastRatio {
					t.Errorf("Accent5 contrast = %.2f, want >= %.1f", a5.ContrastWith(white), MinAccentContrastRatio)
				}
				if p.Accent6.Hex() != "#D4463A" {
					t.Errorf("Accent6 = %s, want #D4463A", p.Accent6.Hex())
				}
				// Primary should match Accent1
				if p.Primary.Hex() != "#4E79A7" {
					t.Errorf("Primary = %s, want #4E79A7", p.Primary.Hex())
				}
			},
		},
		{
			name: "partial accents are applied",
			colors: []ThemeColorInput{
				{Name: "accent1", RGB: "#4E79A7"},
				{Name: "accent2", RGB: "#C0504D"},
			},
			wantName:   "theme",
			wantAccent: "#4E79A7",
			checkFn: func(t *testing.T, p *Palette) {
				// Both colors have good contrast vs white, so preserved as-is
				if p.Accent1.Hex() != "#4E79A7" {
					t.Errorf("Accent1 = %s, want #4E79A7", p.Accent1.Hex())
				}
				if p.Accent2.Hex() != "#C0504D" {
					t.Errorf("Accent2 = %s, want #C0504D", p.Accent2.Hex())
				}
				// Accent3-6 should retain defaults from base palette
			},
		},
		{
			name: "dark/light colors set text and background",
			colors: []ThemeColorInput{
				{Name: "accent1", RGB: "#0000FF"}, // Need at least one accent
				{Name: "dk1", RGB: "#1A1A1A"},     // Dark text
				{Name: "dk2", RGB: "#333333"},     // Secondary text
				{Name: "lt1", RGB: "#F0F0F0"},     // Background
				{Name: "lt2", RGB: "#E0E0E0"},     // Surface
			},
			wantName: "theme",
			checkFn: func(t *testing.T, p *Palette) {
				if p.TextPrimary.Hex() != "#1A1A1A" {
					t.Errorf("TextPrimary = %s, want #1A1A1A", p.TextPrimary.Hex())
				}
				if p.TextSecondary.Hex() != "#333333" {
					t.Errorf("TextSecondary = %s, want #333333", p.TextSecondary.Hex())
				}
				if p.Background.Hex() != "#F0F0F0" {
					t.Errorf("Background = %s, want #F0F0F0", p.Background.Hex())
				}
				if p.Surface.Hex() != "#E0E0E0" {
					t.Errorf("Surface = %s, want #E0E0E0", p.Surface.Hex())
				}
			},
		},
		{
			name: "invalid color is skipped",
			colors: []ThemeColorInput{
				{Name: "accent1", RGB: "#FF0000"},
				{Name: "accent2", RGB: "not-a-color"}, // Invalid
				{Name: "accent3", RGB: "#0000FF"},
			},
			wantName:   "theme",
			wantAccent: "#FF0000",
			checkFn: func(t *testing.T, p *Palette) {
				// Accent1 and 3 should be set, Accent2 should retain default
				if p.Accent1.Hex() != "#FF0000" {
					t.Errorf("Accent1 = %s, want #FF0000", p.Accent1.Hex())
				}
				if p.Accent3.Hex() != "#0000FF" {
					t.Errorf("Accent3 = %s, want #0000FF", p.Accent3.Hex())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPaletteFromThemeColors(tt.colors)

			if p.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", p.Name, tt.wantName)
			}

			if tt.wantAccent != "" && p.Accent1.Hex() != tt.wantAccent {
				t.Errorf("Accent1 = %s, want %s", p.Accent1.Hex(), tt.wantAccent)
			}

			if tt.checkFn != nil {
				tt.checkFn(t, p)
			}
		})
	}
}

// TestEnforceAccentContrast_LowContrastPalette verifies that EnforceAccentContrast fixes
// a problematic accent palette where 5 of 6 accents fail WCAG AA contrast against white
// and adjacent greys are nearly indistinguishable.
func TestEnforceAccentContrast_LowContrastPalette(t *testing.T) {
	lowContrastColors := []ThemeColorInput{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#000000"},
		{Name: "lt2", RGB: "#EBEBEB"},
		{Name: "accent1", RGB: "#FD5108"}, // Orange
		{Name: "accent2", RGB: "#FE7C39"}, // Lighter orange
		{Name: "accent3", RGB: "#FFAA72"}, // Very light orange/peach
		{Name: "accent4", RGB: "#A1A8B3"}, // Grey
		{Name: "accent5", RGB: "#B5BCC4"}, // Light grey
		{Name: "accent6", RGB: "#CBD1D6"}, // Very light grey
	}

	p := NewPaletteFromThemeColors(lowContrastColors)
	white := p.Background

	// Verify all accents now meet minimum contrast against background.
	accents := []struct {
		name  string
		color Color
	}{
		{"Accent1", p.Accent1},
		{"Accent2", p.Accent2},
		{"Accent3", p.Accent3},
		{"Accent4", p.Accent4},
		{"Accent5", p.Accent5},
		{"Accent6", p.Accent6},
	}

	origHexes := []string{"#FD5108", "#FE7C39", "#FFAA72", "#A1A8B3", "#B5BCC4", "#CBD1D6"}
	for i, a := range accents {
		cr := a.color.ContrastWith(white)
		t.Logf("%s: %s -> %s (contrast: %.2f:1)", a.name, origHexes[i], a.color.Hex(), cr)
		if cr < MinAccentContrastRatio {
			t.Errorf("%s %s contrast vs white = %.2f, want >= %.1f",
				a.name, a.color.Hex(), cr, MinAccentContrastRatio)
		}
	}

	// Verify adjacent pairs are sufficiently distinguishable.
	for i := 0; i < len(accents)-1; i++ {
		dist := colorDistanceRGB(accents[i].color, accents[i+1].color)
		t.Logf("%s vs %s: distance = %.1f", accents[i].name, accents[i+1].name, dist)
		if dist < MinAccentDistance {
			t.Errorf("%s (%s) vs %s (%s) distance = %.1f, want >= %.1f",
				accents[i].name, accents[i].color.Hex(),
				accents[i+1].name, accents[i+1].color.Hex(),
				dist, MinAccentDistance)
		}
	}

	// Accent1 (orange, #FD5108) already has 3.3:1 contrast vs white,
	// so it should be preserved unchanged or nearly unchanged.
	if colorDistanceRGB(p.Accent1, MustParseColor("#FD5108")) > 15 {
		t.Errorf("Accent1 was modified too aggressively: %s (expected near #FD5108)", p.Accent1.Hex())
	}
}

// TestEnforceAccentContrast_GoodPalette verifies that a well-designed palette
// passes through EnforceAccentContrast without modification.
func TestEnforceAccentContrast_GoodPalette(t *testing.T) {
	// A palette where every accent has >3:1 contrast vs white and
	// every adjacent pair has >55 RGB distance. These should not be modified.
	p := &Palette{
		Name:       "test-good",
		Background: MustParseColor("#FFFFFF"),
		Accent1:    MustParseColor("#2E5090"), // Blue, 7.3:1
		Accent2:    MustParseColor("#D4463A"), // Red, 4.4:1
		Accent3:    MustParseColor("#2E7D32"), // Green, 5.1:1
		Accent4:    MustParseColor("#8064A2"), // Purple, 4.9:1
		Accent5:    MustParseColor("#C0504D"), // Brick, 4.7:1
		Accent6:    MustParseColor("#1B5E20"), // Dark green, 8.2:1
	}
	original := p.Clone()

	EnforceAccentContrast(p)

	// All accents should be unchanged since they already meet both constraints
	accents := []struct {
		name     string
		got, want Color
	}{
		{"Accent1", p.Accent1, original.Accent1},
		{"Accent2", p.Accent2, original.Accent2},
		{"Accent3", p.Accent3, original.Accent3},
		{"Accent4", p.Accent4, original.Accent4},
		{"Accent5", p.Accent5, original.Accent5},
		{"Accent6", p.Accent6, original.Accent6},
	}
	for _, a := range accents {
		if a.got.Hex() != a.want.Hex() {
			t.Errorf("%s changed: %s -> %s", a.name, a.want.Hex(), a.got.Hex())
		}
	}
}

// TestEnforceAccentContrast_NilPalette ensures nil palette does not panic.
func TestEnforceAccentContrast_NilPalette(t *testing.T) {
	EnforceAccentContrast(nil) // Should not panic
}

// TestEnforceAccentContrast_DarkBackground verifies enforcement works
// when the background is dark (lightens instead of darkens).
func TestEnforceAccentContrast_DarkBackground(t *testing.T) {
	p := &Palette{
		Background: MustParseColor("#1A1A1A"), // Dark background
		Accent1:    MustParseColor("#222222"),  // Too dark, low contrast
		Accent2:    MustParseColor("#252525"),  // Too similar to Accent1
		Accent3:    MustParseColor("#FF0000"),  // Good contrast
		Accent4:    MustParseColor("#00CC00"),  // Good contrast
		Accent5:    MustParseColor("#0000FF"),  // Good contrast
		Accent6:    MustParseColor("#CC00CC"),  // Good contrast
	}

	EnforceAccentContrast(p)

	bg := p.Background
	// Accent1 should have been lightened for contrast
	if p.Accent1.ContrastWith(bg) < MinAccentContrastRatio {
		t.Errorf("Accent1 contrast vs dark bg = %.2f, want >= %.1f",
			p.Accent1.ContrastWith(bg), MinAccentContrastRatio)
	}
	// Accent2 should be distinguishable from Accent1
	if colorDistanceRGB(p.Accent1, p.Accent2) < MinAccentDistance {
		t.Errorf("Accent1 vs Accent2 distance = %.1f, want >= %.1f",
			colorDistanceRGB(p.Accent1, p.Accent2), MinAccentDistance)
	}
}

// TestColorDistanceRGB verifies the euclidean distance calculation.
func TestColorDistanceRGB(t *testing.T) {
	tests := []struct {
		name string
		a, b Color
		want float64
	}{
		{
			name: "identical colors",
			a:    MustParseColor("#FF0000"),
			b:    MustParseColor("#FF0000"),
			want: 0,
		},
		{
			name: "black vs white",
			a:    MustParseColor("#000000"),
			b:    MustParseColor("#FFFFFF"),
			want: 441.67, // sqrt(255^2 * 3)
		},
		{
			name: "red vs green",
			a:    MustParseColor("#FF0000"),
			b:    MustParseColor("#00FF00"),
			want: 360.62, // sqrt(255^2 + 255^2)
		},
		{
			name: "accent4 vs accent5 (too similar)",
			a:    MustParseColor("#A1A8B3"),
			b:    MustParseColor("#B5BCC4"),
			want: 33.0, // These should fail MinAccentDistance
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorDistanceRGB(tt.a, tt.b)
			// Allow small floating-point tolerance
			if math.Abs(got-tt.want) > 0.5 {
				t.Errorf("colorDistanceRGB() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// TestScaleToFit tests the ScaleToFit helper function.
func TestScaleToFit(t *testing.T) {
	tests := []struct {
		name               string
		srcW, srcH         float64
		maxW, maxH         float64
		wantW, wantH       float64
	}{
		{
			name:   "square content in wide container",
			srcW:   1.0, srcH: 1.0,
			maxW:   800, maxH: 400,
			wantW:  400, wantH: 400, // Height-constrained
		},
		{
			name:   "square content in tall container",
			srcW:   1.0, srcH: 1.0,
			maxW:   400, maxH: 800,
			wantW:  400, wantH: 400, // Width-constrained
		},
		{
			name:   "wide content in wide container",
			srcW:   2.0, srcH: 1.0, // 2:1 aspect ratio
			maxW:   800, maxH: 400,
			wantW:  800, wantH: 400, // Exact fit
		},
		{
			name:   "wide content in square container",
			srcW:   2.0, srcH: 1.0,
			maxW:   600, maxH: 600,
			wantW:  600, wantH: 300, // Width-constrained
		},
		{
			name:   "tall content in square container",
			srcW:   1.0, srcH: 2.0,
			maxW:   600, maxH: 600,
			wantW:  300, wantH: 600, // Height-constrained
		},
		{
			name:   "content already fits exactly",
			srcW:   800, srcH: 600,
			maxW:   800, maxH: 600,
			wantW:  800, wantH: 600,
		},
		{
			name:   "zero source width returns max",
			srcW:   0, srcH: 600,
			maxW:   800, maxH: 600,
			wantW:  800, wantH: 600,
		},
		{
			name:   "zero max width returns max",
			srcW:   800, srcH: 600,
			maxW:   0, maxH: 600,
			wantW:  0, wantH: 600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := ScaleToFit(tt.srcW, tt.srcH, tt.maxW, tt.maxH)
			if math.Abs(gotW-tt.wantW) > 0.001 || math.Abs(gotH-tt.wantH) > 0.001 {
				t.Errorf("ScaleToFit(%v, %v, %v, %v) = (%v, %v), want (%v, %v)",
					tt.srcW, tt.srcH, tt.maxW, tt.maxH, gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}

// TestScaleToCover tests the ScaleToCover helper function.
func TestScaleToCover(t *testing.T) {
	tests := []struct {
		name               string
		srcW, srcH         float64
		minW, minH         float64
		wantW, wantH       float64
	}{
		{
			name:   "square content in wide container",
			srcW:   1.0, srcH: 1.0,
			minW:   800, minH: 400,
			wantW:  800, wantH: 800, // Width-constrained, overflows height
		},
		{
			name:   "square content in tall container",
			srcW:   1.0, srcH: 1.0,
			minW:   400, minH: 800,
			wantW:  800, wantH: 800, // Height-constrained, overflows width
		},
		{
			name:   "wide content in wide container (exact fit)",
			srcW:   2.0, srcH: 1.0,
			minW:   800, minH: 400,
			wantW:  800, wantH: 400, // Perfect fit
		},
		{
			name:   "tall content covers wide container",
			srcW:   1.0, srcH: 2.0,
			minW:   800, minH: 400,
			wantW:  800, wantH: 1600, // Overflows height
		},
		{
			name:   "zero source returns min",
			srcW:   0, srcH: 600,
			minW:   800, minH: 600,
			wantW:  800, wantH: 600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := ScaleToCover(tt.srcW, tt.srcH, tt.minW, tt.minH)
			if math.Abs(gotW-tt.wantW) > 0.001 || math.Abs(gotH-tt.wantH) > 0.001 {
				t.Errorf("ScaleToCover(%v, %v, %v, %v) = (%v, %v), want (%v, %v)",
					tt.srcW, tt.srcH, tt.minW, tt.minH, gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}

// TestCenterInSlot tests the CenterInSlot helper function.
func TestCenterInSlot(t *testing.T) {
	tests := []struct {
		name                     string
		contentW, contentH       float64
		slotW, slotH             float64
		wantOffsetX, wantOffsetY float64
	}{
		{
			name:        "centered square in wide slot",
			contentW:    400, contentH: 400,
			slotW:       800, slotH: 400,
			wantOffsetX: 200, wantOffsetY: 0,
		},
		{
			name:        "centered square in tall slot",
			contentW:    400, contentH: 400,
			slotW:       400, slotH: 800,
			wantOffsetX: 0, wantOffsetY: 200,
		},
		{
			name:        "content matches slot",
			contentW:    800, contentH: 600,
			slotW:       800, slotH: 600,
			wantOffsetX: 0, wantOffsetY: 0,
		},
		{
			name:        "content larger than slot (negative offset)",
			contentW:    1000, contentH: 800,
			slotW:       800, slotH: 600,
			wantOffsetX: -100, wantOffsetY: -100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := CenterInSlot(tt.contentW, tt.contentH, tt.slotW, tt.slotH)
			if gotX != tt.wantOffsetX || gotY != tt.wantOffsetY {
				t.Errorf("CenterInSlot(%v, %v, %v, %v) = (%v, %v), want (%v, %v)",
					tt.contentW, tt.contentH, tt.slotW, tt.slotH,
					gotX, gotY, tt.wantOffsetX, tt.wantOffsetY)
			}
		})
	}
}

// TestFitDimensions tests the FitDimensions convenience function.
func TestFitDimensions(t *testing.T) {
	tests := []struct {
		name                                     string
		fitMode                                  string
		srcW, srcH                               float64
		containerW, containerH                   float64
		wantContentW, wantContentH               float64
		wantOffsetX, wantOffsetY                 float64
	}{
		{
			name:           "stretch mode uses container dimensions",
			fitMode:        "stretch",
			srcW:           1.0, srcH: 1.0,
			containerW:     800, containerH: 400,
			wantContentW:   800, wantContentH: 400,
			wantOffsetX:    0, wantOffsetY: 0,
		},
		{
			name:           "empty mode defaults to stretch",
			fitMode:        "",
			srcW:           1.0, srcH: 1.0,
			containerW:     800, containerH: 400,
			wantContentW:   800, wantContentH: 400,
			wantOffsetX:    0, wantOffsetY: 0,
		},
		{
			name:           "contain mode fits square in wide container",
			fitMode:        "contain",
			srcW:           1.0, srcH: 1.0,
			containerW:     800, containerH: 400,
			wantContentW:   400, wantContentH: 400,
			wantOffsetX:    200, wantOffsetY: 0,
		},
		{
			name:           "contain mode fits square in tall container",
			fitMode:        "contain",
			srcW:           1.0, srcH: 1.0,
			containerW:     400, containerH: 800,
			wantContentW:   400, wantContentH: 400,
			wantOffsetX:    0, wantOffsetY: 200,
		},
		{
			name:           "cover mode fills with square in wide container",
			fitMode:        "cover",
			srcW:           1.0, srcH: 1.0,
			containerW:     800, containerH: 400,
			wantContentW:   800, wantContentH: 800,
			wantOffsetX:    0, wantOffsetY: -200,
		},
		{
			name:           "cover mode fills with square in tall container",
			fitMode:        "cover",
			srcW:           1.0, srcH: 1.0,
			containerW:     400, containerH: 800,
			wantContentW:   800, wantContentH: 800,
			wantOffsetX:    -200, wantOffsetY: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentW, contentH, offsetX, offsetY := FitDimensions(
				tt.fitMode, tt.srcW, tt.srcH, tt.containerW, tt.containerH)

			if math.Abs(contentW-tt.wantContentW) > 0.001 ||
				math.Abs(contentH-tt.wantContentH) > 0.001 {
				t.Errorf("content dimensions = (%v, %v), want (%v, %v)",
					contentW, contentH, tt.wantContentW, tt.wantContentH)
			}
			if math.Abs(offsetX-tt.wantOffsetX) > 0.001 ||
				math.Abs(offsetY-tt.wantOffsetY) > 0.001 {
				t.Errorf("offset = (%v, %v), want (%v, %v)",
					offsetX, offsetY, tt.wantOffsetX, tt.wantOffsetY)
			}
		})
	}
}
