package svggen

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// rgbRegex matches rgb() and rgba() CSS color functions.
var rgbRegex = regexp.MustCompile(`rgba?\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+))?\s*\)`)

// StyleGuide provides a centralized design system for consulting-grade SVG output.
// It defines palettes, typography, spacing, and stroke configurations that ensure
// visual consistency across all diagram types.
type StyleGuide struct {
	// Palette defines the color scheme.
	Palette *Palette

	// Typography defines font settings.
	Typography *Typography

	// Spacing defines the spacing scale.
	Spacing *Spacing

	// Strokes defines stroke configurations.
	Strokes *Strokes
}

// DefaultStyleGuide returns the default style guide with consulting-grade settings.
func DefaultStyleGuide() *StyleGuide {
	return &StyleGuide{
		Palette:    DefaultPalette(),
		Typography: DefaultTypography(),
		Spacing:    DefaultSpacing(),
		Strokes:    DefaultStrokes(),
	}
}

// Clone creates a deep copy of the style guide.
func (s *StyleGuide) Clone() *StyleGuide {
	return &StyleGuide{
		Palette:    s.Palette.Clone(),
		Typography: s.Typography.Clone(),
		Spacing:    s.Spacing.Clone(),
		Strokes:    s.Strokes.Clone(),
	}
}

// =============================================================================
// Palette
// =============================================================================

// Palette defines a color scheme for diagrams.
type Palette struct {
	// Name is the palette identifier.
	Name string

	// Primary colors for main content.
	Primary   Color
	Secondary Color
	Tertiary  Color

	// Accent colors for highlights and emphasis.
	Accent1 Color
	Accent2 Color
	Accent3 Color
	Accent4 Color
	Accent5 Color
	Accent6 Color

	// Semantic colors.
	Success Color
	Warning Color
	Error   Color
	Info    Color

	// Neutral colors for backgrounds and text.
	Background    Color
	Surface       Color
	Border        Color
	TextPrimary   Color
	TextSecondary Color
	TextMuted     Color
}

// Color represents an RGBA color.
type Color struct {
	R, G, B uint8
	A       float64 // 0.0 to 1.0
}

// Hex returns the color as a hex string (#RRGGBB or #RRGGBBAA).
func (c Color) Hex() string {
	if c.A < 1.0 {
		return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, uint8(c.A*255))
	}
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// RGBA returns the color as RGBA values (0-255, 0-255, 0-255, 0-1).
func (c Color) RGBA() (uint8, uint8, uint8, float64) {
	return c.R, c.G, c.B, c.A
}

// WithAlpha returns a copy of the color with the specified alpha.
func (c Color) WithAlpha(a float64) Color {
	return Color{R: c.R, G: c.G, B: c.B, A: a}
}

// ParseColor parses a hex color string into a Color.
// Supports formats: #RGB, #RGBA, #RRGGBB, #RRGGBBAA.
func ParseColor(hex string) (Color, error) {
	hex = strings.TrimPrefix(hex, "#")

	var r, g, b uint8
	a := 1.0

	switch len(hex) {
	case 3: // #RGB
		if _, err := fmt.Sscanf(hex, "%1x%1x%1x", &r, &g, &b); err != nil {
			return Color{}, fmt.Errorf("invalid color format: %s", hex)
		}
		r = r * 17 // Expand to 0-255
		g = g * 17
		b = b * 17
	case 4: // #RGBA
		var aInt uint8
		if _, err := fmt.Sscanf(hex, "%1x%1x%1x%1x", &r, &g, &b, &aInt); err != nil {
			return Color{}, fmt.Errorf("invalid color format: %s", hex)
		}
		r = r * 17
		g = g * 17
		b = b * 17
		a = float64(aInt*17) / 255.0
	case 6: // #RRGGBB
		if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err != nil {
			return Color{}, fmt.Errorf("invalid color format: %s", hex)
		}
	case 8: // #RRGGBBAA
		var aInt uint8
		if _, err := fmt.Sscanf(hex, "%02x%02x%02x%02x", &r, &g, &b, &aInt); err != nil {
			return Color{}, fmt.Errorf("invalid color format: %s", hex)
		}
		a = float64(aInt) / 255.0
	default:
		return Color{}, fmt.Errorf("invalid color length: %d", len(hex))
	}

	return Color{R: r, G: g, B: b, A: a}, nil
}

// MustParseColor parses a hex color string, panicking on error.
func MustParseColor(hex string) Color {
	c, err := ParseColor(hex)
	if err != nil {
		panic(err)
	}
	return c
}

// AccentColors returns the accent colors as a slice.
func (p *Palette) AccentColors() []Color {
	return []Color{p.Accent1, p.Accent2, p.Accent3, p.Accent4, p.Accent5, p.Accent6}
}

// AccentColor returns the accent color at the specified index (wrapping if needed).
func (p *Palette) AccentColor(index int) Color {
	accents := p.AccentColors()
	return accents[index%len(accents)]
}

// Clone creates a copy of the palette.
func (p *Palette) Clone() *Palette {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// Built-in palettes.

// DefaultPalette returns the default consulting palette based on Tableau 10.
func DefaultPalette() *Palette {
	return &Palette{
		Name:      "corporate",
		Primary:   MustParseColor("#4E79A7"),
		Secondary: MustParseColor("#59A14F"),
		Tertiary:  MustParseColor("#E15759"),

		Accent1: MustParseColor("#4E79A7"), // Blue
		Accent2: MustParseColor("#F28E2B"), // Orange
		Accent3: MustParseColor("#E15759"), // Red
		Accent4: MustParseColor("#76B7B2"), // Teal
		Accent5: MustParseColor("#59A14F"), // Green
		Accent6: MustParseColor("#EDC948"), // Yellow

		Success: MustParseColor("#59A14F"),
		Warning: MustParseColor("#F28E2B"),
		Error:   MustParseColor("#E15759"),
		Info:    MustParseColor("#4E79A7"),

		Background:    MustParseColor("#FFFFFF"),
		Surface:       MustParseColor("#F8F9FA"),
		Border:        MustParseColor("#DEE2E6"),
		TextPrimary:   MustParseColor("#212529"),
		TextSecondary: MustParseColor("#495057"),
		TextMuted:     MustParseColor("#6C757D"),
	}
}

// VibrantPalette returns a vibrant, modern color palette.
func VibrantPalette() *Palette {
	return &Palette{
		Name:      "vibrant",
		Primary:   MustParseColor("#FF6B6B"),
		Secondary: MustParseColor("#4ECDC4"),
		Tertiary:  MustParseColor("#45B7D1"),

		Accent1: MustParseColor("#FF6B6B"), // Coral
		Accent2: MustParseColor("#4ECDC4"), // Turquoise
		Accent3: MustParseColor("#45B7D1"), // Sky Blue
		Accent4: MustParseColor("#96CEB4"), // Sage
		Accent5: MustParseColor("#FFEAA7"), // Yellow
		Accent6: MustParseColor("#DDA0DD"), // Plum

		Success: MustParseColor("#96CEB4"),
		Warning: MustParseColor("#FFEAA7"),
		Error:   MustParseColor("#FF6B6B"),
		Info:    MustParseColor("#45B7D1"),

		Background:    MustParseColor("#FFFFFF"),
		Surface:       MustParseColor("#F7F9FC"),
		Border:        MustParseColor("#E1E8ED"),
		TextPrimary:   MustParseColor("#2C3E50"),
		TextSecondary: MustParseColor("#7F8C8D"),
		TextMuted:     MustParseColor("#BDC3C7"),
	}
}

// MutedPalette returns a subtle, muted color palette.
func MutedPalette() *Palette {
	return &Palette{
		Name:      "muted",
		Primary:   MustParseColor("#8B9DC3"),
		Secondary: MustParseColor("#9DC3C2"),
		Tertiary:  MustParseColor("#C3B299"),

		Accent1: MustParseColor("#8B9DC3"), // Soft Blue
		Accent2: MustParseColor("#9DC3C2"), // Soft Teal
		Accent3: MustParseColor("#C3B299"), // Soft Tan
		Accent4: MustParseColor("#C9A9A6"), // Dusty Rose
		Accent5: MustParseColor("#A6C9B5"), // Sage
		Accent6: MustParseColor("#B5A6C9"), // Lavender

		Success: MustParseColor("#A6C9B5"),
		Warning: MustParseColor("#C3B299"),
		Error:   MustParseColor("#C9A9A6"),
		Info:    MustParseColor("#8B9DC3"),

		Background:    MustParseColor("#FAFAFA"),
		Surface:       MustParseColor("#F0F0F0"),
		Border:        MustParseColor("#D0D0D0"),
		TextPrimary:   MustParseColor("#3C3C3C"),
		TextSecondary: MustParseColor("#6B6B6B"),
		TextMuted:     MustParseColor("#9B9B9B"),
	}
}

// MonochromePalette returns a grayscale palette.
func MonochromePalette() *Palette {
	return &Palette{
		Name:      "monochrome",
		Primary:   MustParseColor("#333333"),
		Secondary: MustParseColor("#666666"),
		Tertiary:  MustParseColor("#999999"),

		Accent1: MustParseColor("#1A1A1A"),
		Accent2: MustParseColor("#333333"),
		Accent3: MustParseColor("#4D4D4D"),
		Accent4: MustParseColor("#666666"),
		Accent5: MustParseColor("#808080"),
		Accent6: MustParseColor("#999999"),

		Success: MustParseColor("#4D4D4D"),
		Warning: MustParseColor("#808080"),
		Error:   MustParseColor("#333333"),
		Info:    MustParseColor("#666666"),

		Background:    MustParseColor("#FFFFFF"),
		Surface:       MustParseColor("#F5F5F5"),
		Border:        MustParseColor("#CCCCCC"),
		TextPrimary:   MustParseColor("#1A1A1A"),
		TextSecondary: MustParseColor("#4D4D4D"),
		TextMuted:     MustParseColor("#808080"),
	}
}

// GetPaletteByName returns a palette by name.
func GetPaletteByName(name string) *Palette {
	switch strings.ToLower(name) {
	case "corporate", "default":
		return DefaultPalette()
	case "vibrant":
		return VibrantPalette()
	case "muted":
		return MutedPalette()
	case "monochrome", "grayscale":
		return MonochromePalette()
	default:
		return DefaultPalette()
	}
}

// =============================================================================
// Typography
// =============================================================================

// Typography defines font settings for diagrams.
type Typography struct {
	// FontFamily is the primary font family.
	FontFamily string

	// FallbackFonts are fallback font families.
	FallbackFonts []string

	// Size scale in points.
	SizeTitle    float64 // Title text
	SizeSubtitle float64 // Subtitle text
	SizeHeading  float64 // Section headings
	SizeBody     float64 // Body text
	SizeSmall    float64 // Small labels
	SizeCaption  float64 // Captions and footnotes

	// Weight definitions.
	WeightLight  int
	WeightNormal int
	WeightMedium int
	WeightBold   int

	// Line height multiplier.
	LineHeight float64
}

// FontStack returns the font family with fallbacks as CSS-style string.
func (t *Typography) FontStack() string {
	fonts := append([]string{t.FontFamily}, t.FallbackFonts...)
	return strings.Join(fonts, ", ")
}

// Clone creates a copy of the typography settings.
func (t *Typography) Clone() *Typography {
	if t == nil {
		return nil
	}
	cp := *t
	cp.FallbackFonts = make([]string, len(t.FallbackFonts))
	copy(cp.FallbackFonts, t.FallbackFonts)
	return &cp
}

// Default reference dimensions for font scaling.
// Font sizes are designed for these dimensions.
const (
	ReferenceWidth  = 800.0
	ReferenceHeight = 600.0
)

// ScaleForDimensions returns a copy of the typography with font sizes scaled
// for the given output dimensions. The scaling is based on the geometric mean
// of width and height ratios relative to the reference dimensions (800x600).
//
// This ensures that fonts remain proportionally sized across different canvas
// sizes. For example, a 1600x1200 canvas (2x in both dimensions) will have
// 2x larger fonts, maintaining readability at the larger size.
func (t *Typography) ScaleForDimensions(width, height float64) *Typography {
	if t == nil {
		return nil
	}

	// Calculate scaling factor based on geometric mean of dimension ratios.
	// This provides balanced scaling when width and height scale differently.
	widthRatio := width / ReferenceWidth
	heightRatio := height / ReferenceHeight

	// Use geometric mean for balanced scaling
	// sqrt(widthRatio * heightRatio) gives a scale factor that's fair to both dimensions
	// This avoids underscaling that occurs with min() on wide/tall canvases.
	scale := 1.0
	if widthRatio > 0 && heightRatio > 0 {
		scale = math.Sqrt(widthRatio * heightRatio)
	}

	// Clamp scale to reasonable bounds to avoid extreme sizes
	// Minimum 0.5x (for very small canvases) to 5.0x (for high-res outputs)
	if scale < 0.5 {
		scale = 0.5
	} else if scale > 5.0 {
		scale = 5.0
	}

	// Clone and scale all font sizes
	cp := t.Clone()
	cp.SizeTitle = t.SizeTitle * scale
	cp.SizeSubtitle = t.SizeSubtitle * scale
	cp.SizeHeading = t.SizeHeading * scale
	cp.SizeBody = t.SizeBody * scale
	cp.SizeSmall = t.SizeSmall * scale
	cp.SizeCaption = t.SizeCaption * scale

	// Enforce minimum and maximum font sizes for presentation scale.
	// Font sizes are now actual rendered pt values (no pipeline reduction).
	// Floors prevent text from becoming illegible on small canvases;
	// caps prevent oversized text on large canvases.
	// Body text floors at 11pt and label/annotation text at 9pt minimum
	// to ensure legibility across all chart types and canvas sizes.
	const (
		minTitle    = 13.0 // Chart titles must remain prominent
		minSubtitle = 11.0 // Subtitles/section headers
		minHeading  = 11.0 // Legend text, section headings
		minBody     = 11.0 // Pie/donut outside labels, axis titles
		minSmall    = 9.0  // Axis tick labels, value labels
		minCaption  = 10.0 // Diagram badges, footnotes — 10pt floor for presentation readability

		maxTitle    = 24.0 // Large canvas titles
		maxSubtitle = 19.0
		maxHeading  = 16.0
		maxBody     = 14.0
		maxSmall    = 12.0
		maxCaption  = 12.0 // Allow captions to scale on larger canvases
	)
	cp.SizeTitle = math.Max(minTitle, math.Min(maxTitle, cp.SizeTitle))
	cp.SizeSubtitle = math.Max(minSubtitle, math.Min(maxSubtitle, cp.SizeSubtitle))
	cp.SizeHeading = math.Max(minHeading, math.Min(maxHeading, cp.SizeHeading))
	cp.SizeBody = math.Max(minBody, math.Min(maxBody, cp.SizeBody))
	cp.SizeSmall = math.Max(minSmall, math.Min(maxSmall, cp.SizeSmall))
	cp.SizeCaption = math.Max(minCaption, math.Min(maxCaption, cp.SizeCaption))

	return cp
}

// DefaultTypography returns consulting-grade typography settings.
// Font sizes are in points and represent the actual rendered size in PPTX
// placeholders. The reference canvas is 800x600pt; ScaleForDimensions adjusts
// sizes for other canvas dimensions with floor/cap enforcement.
func DefaultTypography() *Typography {
	return &Typography{
		FontFamily:    "Arial",
		FallbackFonts: []string{"Helvetica", "sans-serif"},

		SizeTitle:    18,
		SizeSubtitle: 15,
		SizeHeading:  13,
		SizeBody:     12,
		SizeSmall:    10,
		SizeCaption:  10,

		WeightLight:  300,
		WeightNormal: 400,
		WeightMedium: 500,
		WeightBold:   700,

		LineHeight: 1.4,
	}
}

// CompactTypography returns smaller typography for dense diagrams.
// Sizes respect the minimum font size floors (11pt body, 10pt captions) to
// ensure legibility even in information-dense layouts.
func CompactTypography() *Typography {
	return &Typography{
		FontFamily:    "Arial",
		FallbackFonts: []string{"Helvetica", "sans-serif"},

		SizeTitle:    13,
		SizeSubtitle: 11,
		SizeHeading:  11,
		SizeBody:     11,
		SizeSmall:    9,
		SizeCaption:  10,

		WeightLight:  300,
		WeightNormal: 400,
		WeightMedium: 500,
		WeightBold:   700,

		LineHeight: 1.3,
	}
}

// =============================================================================
// Preset Typography
// =============================================================================

// PresetTypography maps layout preset names to hand-tuned typography settings.
// These replace the geometric-mean computation in ScaleForDimensions for known
// presets, providing deterministic, professionally calibrated font sizes.
//
// Values are actual rendered point sizes in the PPTX placeholder.
// For unknown presets, ScaleForDimensions is used as a fallback.
var PresetTypography = map[string]*Typography{
	// Full content area (1600x900) — large canvas.
	"content_16x9": {
		SizeTitle:    20,
		SizeSubtitle: 16,
		SizeHeading:  14,
		SizeBody:     12,
		SizeSmall:    10,
		SizeCaption:  10,
		LineHeight:   1.4,
	},
	// Full slide area (1920x1080).
	"slide_16x9": {
		SizeTitle:    22,
		SizeSubtitle: 17,
		SizeHeading:  15,
		SizeBody:     13,
		SizeSmall:    11,
		SizeCaption:  11,
		LineHeight:   1.4,
	},
	// Half-width (760x720).
	"half_16x9": {
		SizeTitle:    15,
		SizeSubtitle: 13,
		SizeHeading:  11,
		SizeBody:     11,
		SizeSmall:    9,
		SizeCaption:  10,
		LineHeight:   1.4,
	},
	// Third-width (500x720).
	"third_16x9": {
		SizeTitle:    13,
		SizeSubtitle: 11,
		SizeHeading:  11,
		SizeBody:     11,
		SizeSmall:    9,
		SizeCaption:  10,
		LineHeight:   1.3,
	},
	// 4:3 full slide (1024x768).
	"slide_4x3": {
		SizeTitle:    18,
		SizeSubtitle: 14,
		SizeHeading:  13,
		SizeBody:     11,
		SizeSmall:    10,
		SizeCaption:  10,
		LineHeight:   1.4,
	},
	// 4:3 half-width (420x540).
	"half_4x3": {
		SizeTitle:    13,
		SizeSubtitle: 11,
		SizeHeading:  11,
		SizeBody:     11,
		SizeSmall:    9,
		SizeCaption:  10,
		LineHeight:   1.3,
	},
	// Square (600x600) — used for circular charts.
	"square": {
		SizeTitle:    15,
		SizeSubtitle: 12,
		SizeHeading:  11,
		SizeBody:     11,
		SizeSmall:    9,
		SizeCaption:  10,
		LineHeight:   1.4,
	},
	// Thumbnail (400x300) — small preview.
	"thumbnail": {
		SizeTitle:    13,
		SizeSubtitle: 11,
		SizeHeading:  11,
		SizeBody:     11,
		SizeSmall:    9,
		SizeCaption:  10,
		LineHeight:   1.3,
	},
}

// TypographyForPreset returns hand-tuned typography for a known layout preset.
// Returns nil if the preset is not found; callers should fall back to
// ScaleForDimensions for unknown presets.
func TypographyForPreset(preset string) *Typography {
	t, ok := PresetTypography[preset]
	if !ok {
		return nil
	}
	// Return a full Typography struct with defaults for fields the preset
	// table doesn't specify (font family, weights).
	base := DefaultTypography()
	base.SizeTitle = t.SizeTitle
	base.SizeSubtitle = t.SizeSubtitle
	base.SizeHeading = t.SizeHeading
	base.SizeBody = t.SizeBody
	base.SizeSmall = t.SizeSmall
	base.SizeCaption = t.SizeCaption
	base.LineHeight = t.LineHeight
	return base
}

// =============================================================================
// Spacing
// =============================================================================

// Spacing defines the spacing scale for consistent layouts.
type Spacing struct {
	// Base unit in points. All other values are multiples.
	Unit float64

	// Scale values (multiples of Unit).
	XXS float64 // Extra extra small (0.25x)
	XS  float64 // Extra small (0.5x)
	SM  float64 // Small (0.75x)
	MD  float64 // Medium (1x)
	LG  float64 // Large (1.5x)
	XL  float64 // Extra large (2x)
	XXL float64 // Extra extra large (3x)

	// Specific use-case spacing.
	Padding     float64 // Default padding
	Margin      float64 // Default margin
	Gap         float64 // Default gap between items
	SectionGap  float64 // Gap between sections
	ChartMargin float64 // Margin around chart content
}

// Value returns a spacing value in points.
func (s *Spacing) Value(scale float64) float64 {
	return s.Unit * scale
}

// Clone creates a copy of the spacing settings.
func (s *Spacing) Clone() *Spacing {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

// DefaultSpacing returns the default spacing scale.
func DefaultSpacing() *Spacing {
	unit := 8.0
	return &Spacing{
		Unit: unit,

		XXS: unit * 0.25, // 2pt
		XS:  unit * 0.5,  // 4pt
		SM:  unit * 0.75, // 6pt
		MD:  unit,        // 8pt
		LG:  unit * 1.5,  // 12pt
		XL:  unit * 2,    // 16pt
		XXL: unit * 3,    // 24pt

		Padding:     unit * 1.5, // 12pt
		Margin:      unit * 2,   // 16pt
		Gap:         unit,       // 8pt
		SectionGap:  unit * 2.5, // 20pt
		ChartMargin: unit * 5,   // 40pt
	}
}

// compactSpacing returns tighter spacing for dense diagrams.
func compactSpacing() *Spacing {
	unit := 6.0
	return &Spacing{
		Unit: unit,

		XXS: unit * 0.25,
		XS:  unit * 0.5,
		SM:  unit * 0.75,
		MD:  unit,
		LG:  unit * 1.5,
		XL:  unit * 2,
		XXL: unit * 3,

		Padding:     unit,
		Margin:      unit * 1.5,
		Gap:         unit * 0.75,
		SectionGap:  unit * 2,
		ChartMargin: unit * 4,
	}
}

// =============================================================================
// Strokes
// =============================================================================

// Strokes defines stroke configurations for diagrams.
type Strokes struct {
	// Width scale in points.
	WidthHairline float64 // Very thin lines
	WidthThin     float64 // Thin lines
	WidthNormal   float64 // Normal weight
	WidthMedium   float64 // Medium weight
	WidthThick    float64 // Thick lines
	WidthHeavy    float64 // Very thick lines

	// Dash patterns [dash, gap, dash, gap, ...].
	PatternSolid  []float64
	PatternDashed []float64
	PatternDotted []float64
	PatternMixed  []float64

	// Line caps: "butt", "round", "square".
	DefaultCap string

	// Line joins: "miter", "round", "bevel".
	DefaultJoin string

	// Miter limit for sharp corners.
	MiterLimit float64
}

// Clone creates a copy of the stroke settings.
func (s *Strokes) Clone() *Strokes {
	if s == nil {
		return nil
	}
	cp := *s
	// Copy slices
	cp.PatternSolid = append([]float64{}, s.PatternSolid...)
	cp.PatternDashed = append([]float64{}, s.PatternDashed...)
	cp.PatternDotted = append([]float64{}, s.PatternDotted...)
	cp.PatternMixed = append([]float64{}, s.PatternMixed...)
	return &cp
}

// DefaultStrokes returns the default stroke configuration.
func DefaultStrokes() *Strokes {
	return &Strokes{
		WidthHairline: 1.0,
		WidthThin:     1.5,
		WidthNormal:   2.5,
		WidthMedium:   3.0,
		WidthThick:    4.0,
		WidthHeavy:    5.0,

		PatternSolid:  []float64{},
		PatternDashed: []float64{6, 3},
		PatternDotted: []float64{2, 2},
		PatternMixed:  []float64{8, 3, 2, 3},

		DefaultCap:  "round",
		DefaultJoin: "round",
		MiterLimit:  4.0,
	}
}

// =============================================================================
// StyleGuide from StyleSpec
// =============================================================================

// StyleGuideFromSpec creates a StyleGuide from a StyleSpec.
func StyleGuideFromSpec(spec StyleSpec) *StyleGuide {
	guide := DefaultStyleGuide()

	// Handle palette — prefer ThemeColors (full-fidelity) over Palette (accent-only)
	if len(spec.ThemeColors) > 0 {
		guide.Palette = NewPaletteFromThemeColors(spec.ThemeColors)
	} else if !spec.Palette.IsZero() {
		if spec.Palette.Name != "" {
			guide.Palette = GetPaletteByName(spec.Palette.Name)
		} else if len(spec.Palette.Colors) > 0 {
			colors := make([]Color, 0, len(spec.Palette.Colors))
			for _, hex := range spec.Palette.Colors {
				if c, err := ParseColor(hex); err == nil {
					colors = append(colors, c)
				}
			}
			if len(colors) > 0 {
				guide.Palette = CustomPalette(colors)
			}
		}
	}

	// Handle font family
	if spec.FontFamily != "" {
		guide.Typography.FontFamily = spec.FontFamily
	}

	// Handle background
	if spec.Background != "" && spec.Background != "transparent" {
		if c, err := ParseColor(spec.Background); err == nil {
			guide.Palette.Background = c
		}
	}

	// Handle surface (theme lt2, used for contrast calculations)
	if spec.Surface != "" {
		if c, err := ParseColor(spec.Surface); err == nil {
			guide.Palette.Surface = c
		}
	}

	return guide
}

// CustomPalette creates a palette from a slice of colors.
func CustomPalette(colors []Color) *Palette {
	p := DefaultPalette()
	p.Name = "custom"

	// Assign colors to accent slots
	for i, c := range colors {
		switch i {
		case 0:
			p.Accent1 = c
			p.Primary = c
		case 1:
			p.Accent2 = c
			p.Secondary = c
		case 2:
			p.Accent3 = c
			p.Tertiary = c
		case 3:
			p.Accent4 = c
		case 4:
			p.Accent5 = c
		case 5:
			p.Accent6 = c
		}
	}

	return p
}

// =============================================================================
// Color Utilities
// =============================================================================

// Lighten returns a lighter version of the color.
func (c Color) Lighten(amount float64) Color {
	return Color{
		R: uint8(min(255, float64(c.R)+(255-float64(c.R))*amount)),
		G: uint8(min(255, float64(c.G)+(255-float64(c.G))*amount)),
		B: uint8(min(255, float64(c.B)+(255-float64(c.B))*amount)),
		A: c.A,
	}
}

// Darken returns a darker version of the color.
func (c Color) Darken(amount float64) Color {
	return Color{
		R: uint8(float64(c.R) * (1 - amount)),
		G: uint8(float64(c.G) * (1 - amount)),
		B: uint8(float64(c.B) * (1 - amount)),
		A: c.A,
	}
}

// Luminance returns the relative luminance of the color.
func (c Color) Luminance() float64 {
	// sRGB to linear RGB
	r := toLinear(float64(c.R) / 255)
	g := toLinear(float64(c.G) / 255)
	b := toLinear(float64(c.B) / 255)

	return 0.2126*r + 0.7152*g + 0.0722*b
}

// toLinear converts an sRGB channel value to linear RGB.
func toLinear(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return pow((v+0.055)/1.055, 2.4)
}

// pow computes base^exp using math.Pow.
func pow(base, exp float64) float64 {
	return math.Pow(base, exp)
}

// ContrastWith returns the contrast ratio between two colors.
func (c Color) ContrastWith(other Color) float64 {
	l1 := c.Luminance()
	l2 := other.Luminance()

	if l1 > l2 {
		return (l1 + 0.05) / (l2 + 0.05)
	}
	return (l2 + 0.05) / (l1 + 0.05)
}

// IsLight returns true if the color is considered light.
func (c Color) IsLight() bool {
	return c.Luminance() > 0.5
}

// Opaque returns the effective opaque color when composited over white.
// For fully opaque colors (A >= 1.0) the color is returned unchanged.
// For semi-transparent colors, the RGB channels are alpha-blended over white
// so that luminance and contrast calculations reflect what the user actually sees.
func (c Color) Opaque() Color {
	if c.A >= 1.0 {
		return c
	}
	a := c.A
	blend := func(fg, bg uint8) uint8 {
		return uint8(float64(fg)*a + float64(bg)*(1-a) + 0.5)
	}
	return Color{
		R: blend(c.R, 255),
		G: blend(c.G, 255),
		B: blend(c.B, 255),
		A: 1.0,
	}
}

// TextColorFor returns an appropriate text color for this background.
// Uses contrast ratio comparison to pick whichever text color (white or dark)
// provides better readability, rather than a simple luminance threshold.
// Semi-transparent backgrounds are composited over white first so the
// contrast check reflects the actual on-screen appearance.
func (c Color) TextColorFor() Color {
	bg := c.Opaque()
	dark := MustParseColor("#212529")
	white := MustParseColor("#FFFFFF")
	if bg.ContrastWith(white) >= bg.ContrastWith(dark) {
		return white
	}
	return dark
}

// =============================================================================
// CSS Helpers
// =============================================================================

// CSS returns the color as a CSS rgba() or hex string.
func (c Color) CSS() string {
	if c.A < 1.0 {
		return fmt.Sprintf("rgba(%d, %d, %d, %.3f)", c.R, c.G, c.B, c.A)
	}
	return c.Hex()
}

// NewPaletteFromThemeColors creates a Palette from theme colors extracted from a PPTX template.
// The theme colors should be in the format returned by template.ParseTheme().
// This enables charts to match the presentation's visual identity.
//
// The function expects colors with names like "accent1", "accent2", etc.
// If fewer than 6 accent colors are provided, it cycles through available colors.
// Falls back to DefaultPalette() if no accent colors are found.
func NewPaletteFromThemeColors(themeColors []ThemeColorInput) *Palette {
	// Start with default palette as base
	p := DefaultPalette()
	p.Name = "theme"

	// Extract accent colors from theme
	accentColors := make([]Color, 0, 6)
	var dk1, dk2, lt1, lt2 *Color

	for _, tc := range themeColors {
		color, err := ParseColor(tc.RGB)
		if err != nil {
			continue
		}

		switch tc.Name {
		case "accent1":
			p.Accent1 = color
			p.Primary = color
			accentColors = append(accentColors, color)
		case "accent2":
			p.Accent2 = color
			p.Secondary = color
			accentColors = append(accentColors, color)
		case "accent3":
			p.Accent3 = color
			p.Tertiary = color
			accentColors = append(accentColors, color)
		case "accent4":
			p.Accent4 = color
			accentColors = append(accentColors, color)
		case "accent5":
			p.Accent5 = color
			accentColors = append(accentColors, color)
		case "accent6":
			p.Accent6 = color
			accentColors = append(accentColors, color)
		case "dk1":
			c := color
			dk1 = &c
		case "dk2":
			c := color
			dk2 = &c
		case "lt1":
			c := color
			lt1 = &c
		case "lt2":
			c := color
			lt2 = &c
		}
	}

	// If no accent colors found, return default palette
	if len(accentColors) == 0 {
		return DefaultPalette()
	}

	// Set text colors based on dark/light theme colors
	if dk1 != nil {
		p.TextPrimary = *dk1
	}
	if dk2 != nil {
		p.TextSecondary = *dk2
	}
	if lt1 != nil {
		p.Background = *lt1
	}
	if lt2 != nil {
		p.Surface = *lt2
	}

	// Set semantic colors from accents (using common mappings)
	if len(accentColors) >= 1 {
		p.Info = accentColors[0]
	}
	if len(accentColors) >= 3 {
		p.Error = accentColors[2] // Often red/warm color
	}
	if len(accentColors) >= 5 {
		p.Success = accentColors[4] // Often green
	}
	if len(accentColors) >= 2 {
		p.Warning = accentColors[1] // Often orange/yellow
	}

	// Enforce chart-quality constraints on the accent palette.
	// Some templates define accent colors that are too light or too
	// similar to each other, making chart series nearly invisible or
	// indistinguishable on a white background.
	EnforceAccentContrast(p)

	return p
}

// ThemeColorInput is now defined in core/types.go and aliased via core_aliases.go.

// =============================================================================
// Accent Palette Quality Enforcement
// =============================================================================

// MinAccentContrastRatio is the minimum contrast ratio each accent color must
// have against the palette background for chart readability.  This uses the
// WCAG 2.1 AA threshold for non-text graphical objects (3:1).
const MinAccentContrastRatio = 3.0

// MinAccentDistance is the minimum euclidean distance in RGB space between any
// pair of adjacent accent colors.  Below this threshold the two series become
// visually indistinguishable in charts.  Typical good palettes (Tableau 10,
// Office defaults) have adjacent distances of 70-200; problematic palettes
// like monochrome ramps have distances of 30-35.
const MinAccentDistance = 55.0

// colorDistanceRGB returns the euclidean distance between two colors in RGB space.
// Range is 0 (identical) to ~441.67 (black vs white).
func colorDistanceRGB(a, b Color) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// EnforceAccentContrast adjusts accent colors in a palette so that every accent
// meets MinAccentContrastRatio against the palette Background and every pair of
// adjacent accents has at least MinAccentDistance perceptual separation.
//
// The adjustments are minimal: colors are darkened (or lightened, on dark
// backgrounds) just enough to meet the contrast threshold, preserving the
// template's intended hue.  When two adjacent accents are too similar, the
// second is shifted in hue to create separation.
//
// This function mutates the palette in place and also updates Primary,
// Secondary, Tertiary to stay in sync with Accent1-3.
func EnforceAccentContrast(p *Palette) {
	if p == nil {
		return
	}

	bg := p.Background

	// Collect accent color pointers for in-place mutation.
	accents := [6]*Color{
		&p.Accent1, &p.Accent2, &p.Accent3,
		&p.Accent4, &p.Accent5, &p.Accent6,
	}

	// Phase 1: Ensure every accent has sufficient contrast against the background.
	for _, ac := range accents {
		*ac = EnsureContrast(*ac, bg, MinAccentContrastRatio)
	}

	// Phase 2: Ensure adjacent pairs are sufficiently distinguishable.
	// Walk pairs (0,1), (1,2), ..., (4,5) and nudge the second color if needed.
	for i := 0; i < len(accents)-1; i++ {
		a := *accents[i]
		b := *accents[i+1]
		if colorDistanceRGB(a, b) < MinAccentDistance {
			// Shift b in hue while preserving lightness and saturation.
			shifted := nudgeDistinguishability(a, b, bg)
			*accents[i+1] = shifted
		}
	}

	// Keep Primary/Secondary/Tertiary in sync.
	p.Primary = p.Accent1
	p.Secondary = p.Accent2
	p.Tertiary = p.Accent3
}

// nudgeDistinguishability shifts color b away from color a in hue space
// until their RGB distance meets MinAccentDistance, while also keeping b above
// MinAccentContrastRatio against bg.
//
// Strategy: rotate b's hue in 20-degree increments (up to 120 degrees) and
// pick the first candidate that satisfies both constraints.  If no rotation
// works (e.g. both colors are achromatic greys), fall back to adjusting
// lightness to create luminance separation.
func nudgeDistinguishability(a, b, bg Color) Color {
	h, s, l := rgbToHSL(b.R, b.G, b.B)

	// Try progressive hue rotations.
	for step := 1; step <= 6; step++ {
		hCandidate := math.Mod(h+float64(step)*20, 360)
		r, g, bb := hslToRGB(hCandidate, s, l)
		candidate := Color{R: r, G: g, B: bb, A: b.A}
		// Ensure background contrast after hue shift.
		candidate = EnsureContrast(candidate, bg, MinAccentContrastRatio)
		if colorDistanceRGB(a, candidate) >= MinAccentDistance {
			return candidate
		}
	}

	// Hue rotation alone didn't help (colors might be achromatic/grey).
	// Darken or lighten b to create luminance separation from a.
	aLum := a.Luminance()
	bLum := b.Luminance()
	bgLum := bg.Luminance()

	// Decide direction: if a is lighter than b, darken b further (and vice versa).
	// But also consider the background -- avoid pushing b toward the background.
	var target Color
	if aLum > bLum {
		// b is darker -- try making it even darker
		if bgLum < 0.5 {
			// Dark background -- lighten b instead
			target = Color{R: 255, G: 255, B: 255, A: 1.0}
		} else {
			target = Color{R: 0, G: 0, B: 0, A: 1.0}
		}
	} else {
		// b is lighter -- try making it even lighter
		if bgLum > 0.5 {
			// Light background -- darken b instead
			target = Color{R: 0, G: 0, B: 0, A: 1.0}
		} else {
			target = Color{R: 255, G: 255, B: 255, A: 1.0}
		}
	}

	// Binary search for minimum blend that achieves both constraints.
	lo := 0.0
	hi := 1.0
	best := b
	for i := 0; i < 32; i++ {
		mid := (lo + hi) / 2
		candidate := lerpColors(b, target, mid)
		candidate.A = b.A
		meetsDist := colorDistanceRGB(a, candidate) >= MinAccentDistance
		meetsBg := candidate.ContrastWith(bg) >= MinAccentContrastRatio
		if meetsDist && meetsBg {
			best = candidate
			hi = mid
		} else {
			lo = mid
		}
	}
	return best
}

// =============================================================================
// Scaling Helpers for FitMode
// =============================================================================

// ScaleToFit calculates dimensions that fit within bounds while preserving aspect ratio.
// This implements "contain" fit mode - the content fits entirely within the bounds,
// potentially leaving letterbox space on sides or top/bottom.
//
// Parameters:
//   - srcW, srcH: source content dimensions (or aspect ratio, e.g., 1:1 for square)
//   - maxW, maxH: maximum container dimensions
//
// Returns:
//   - fitW, fitH: the scaled dimensions that fit within maxW x maxH
func ScaleToFit(srcW, srcH, maxW, maxH float64) (fitW, fitH float64) {
	if srcW <= 0 || srcH <= 0 || maxW <= 0 || maxH <= 0 {
		return maxW, maxH
	}

	srcRatio := srcW / srcH
	maxRatio := maxW / maxH

	if srcRatio > maxRatio {
		// Width-constrained: content is wider than container
		return maxW, maxW / srcRatio
	}
	// Height-constrained: content is taller than container
	return maxH * srcRatio, maxH
}

// ScaleToCover calculates dimensions that cover bounds while preserving aspect ratio.
// This implements "cover" fit mode - the content fills the entire bounds,
// potentially cropping on sides or top/bottom.
//
// Parameters:
//   - srcW, srcH: source content dimensions (or aspect ratio, e.g., 1:1 for square)
//   - minW, minH: minimum container dimensions to fill
//
// Returns:
//   - coverW, coverH: the scaled dimensions that cover minW x minH
func ScaleToCover(srcW, srcH, minW, minH float64) (coverW, coverH float64) {
	if srcW <= 0 || srcH <= 0 || minW <= 0 || minH <= 0 {
		return minW, minH
	}

	srcRatio := srcW / srcH
	minRatio := minW / minH

	if srcRatio < minRatio {
		// Width-constrained: need to scale up to match width
		return minW, minW / srcRatio
	}
	// Height-constrained: need to scale up to match height
	return minH * srcRatio, minH
}

// CenterInSlot calculates offset to center content within a container slot.
// Useful for positioning letterboxed content in the center of its viewport.
//
// Parameters:
//   - contentW, contentH: dimensions of the content to center
//   - slotW, slotH: dimensions of the container slot
//
// Returns:
//   - offsetX, offsetY: the translation offset to center the content
func CenterInSlot(contentW, contentH, slotW, slotH float64) (offsetX, offsetY float64) {
	return (slotW - contentW) / 2, (slotH - contentH) / 2
}

// FitDimensions calculates the final content dimensions and centering offset
// based on the fit mode. This is a convenience function that combines
// ScaleToFit/ScaleToCover with CenterInSlot.
//
// Parameters:
//   - fitMode: "stretch", "contain", or "cover"
//   - srcW, srcH: source content natural dimensions (for "contain"/"cover", this defines aspect ratio)
//   - containerW, containerH: container dimensions
//
// Returns:
//   - contentW, contentH: the final content dimensions
//   - offsetX, offsetY: the translation offset to center the content
func FitDimensions(fitMode string, srcW, srcH, containerW, containerH float64) (contentW, contentH, offsetX, offsetY float64) {
	switch fitMode {
	case "contain":
		contentW, contentH = ScaleToFit(srcW, srcH, containerW, containerH)
		offsetX, offsetY = CenterInSlot(contentW, contentH, containerW, containerH)
	case "cover":
		contentW, contentH = ScaleToCover(srcW, srcH, containerW, containerH)
		offsetX, offsetY = CenterInSlot(contentW, contentH, containerW, containerH)
	default: // "stretch" or empty (default)
		contentW, contentH = containerW, containerH
		offsetX, offsetY = 0, 0
	}
	return
}

// ParseCSSColor parses CSS color formats (hex, rgb, rgba).
func ParseCSSColor(css string) (Color, error) {
	css = strings.TrimSpace(css)

	// Try hex first
	if strings.HasPrefix(css, "#") {
		return ParseColor(css)
	}

	// Try rgb/rgba
	matches := rgbRegex.FindStringSubmatch(css)
	if matches != nil {
		r, _ := strconv.Atoi(matches[1])
		g, _ := strconv.Atoi(matches[2])
		b, _ := strconv.Atoi(matches[3])
		a := 1.0
		if matches[4] != "" {
			a, _ = strconv.ParseFloat(matches[4], 64)
		}
		return Color{R: uint8(r), G: uint8(g), B: uint8(b), A: a}, nil
	}

	// Named colors (basic set)
	named := map[string]string{
		"black":       "#000000",
		"white":       "#FFFFFF",
		"red":         "#FF0000",
		"green":       "#00FF00",
		"blue":        "#0000FF",
		"yellow":      "#FFFF00",
		"cyan":        "#00FFFF",
		"magenta":     "#FF00FF",
		"gray":        "#808080",
		"grey":        "#808080",
		"transparent": "#00000000",
	}

	if hex, ok := named[strings.ToLower(css)]; ok {
		return ParseColor(hex)
	}

	return Color{}, fmt.Errorf("unsupported color format: %s", css)
}
