package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

// RequestEnvelope is the top-level container for diagram generation requests.
// It specifies the diagram type, content, and styling options.
type RequestEnvelope struct {
	// Type identifies the diagram type (e.g., "bar_chart", "matrix_2x2").
	// Required.
	Type string `json:"type" yaml:"type"`

	// Title is the diagram title. Optional.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`

	// Subtitle is displayed below the title. Optional.
	Subtitle string `json:"subtitle,omitempty" yaml:"subtitle,omitempty"`

	// Data contains the diagram-specific payload.
	// Structure varies by diagram type.
	Data map[string]any `json:"data" yaml:"data"`

	// Output specifies the desired output format and dimensions.
	Output OutputSpec `json:"output,omitempty" yaml:"output,omitempty"`

	// Style contains theming and appearance options.
	Style StyleSpec `json:"style,omitempty" yaml:"style,omitempty"`
}

// OutputSpec defines the output format and dimensions.
type OutputSpec struct {
	// Format is the output format: "svg" (default), "png", or "pdf".
	Format string `json:"format,omitempty" yaml:"format,omitempty"`

	// Width in pixels (for raster) or points (for vector).
	// Default: 800.
	Width int `json:"width,omitempty" yaml:"width,omitempty"`

	// Height in pixels (for raster) or points (for vector).
	// Default: 600.
	Height int `json:"height,omitempty" yaml:"height,omitempty"`

	// Scale is the resolution multiplier for raster output.
	// Default: 2.0 (2x resolution).
	Scale float64 `json:"scale,omitempty" yaml:"scale,omitempty"`

	// Preset is a named layout preset for sizing (e.g., "slide_16x9", "half_16x9").
	// When set, overrides Width/Height unless explicit dimensions are also provided.
	// See layoutPresetDimensions for available presets.
	Preset string `json:"preset,omitempty" yaml:"preset,omitempty"`

	// FitMode controls how content is scaled to fit within the output dimensions.
	// Values: "stretch" (default), "contain" (letterbox), "cover" (crop).
	// Auto-applied as "contain" for all charts; can be overridden with explicit "stretch".
	FitMode string `json:"fit_mode,omitempty" yaml:"fit_mode,omitempty"`

	// AspectRatio is the target aspect ratio (width/height) for the content.
	// When set to 0 (default), uses the source content's natural aspect ratio.
	// When FitMode is "contain" or "cover", this ratio is used for scaling calculations.
	// Example: 1.0 for square content, 1.777 for 16:9, 1.333 for 4:3.
	AspectRatio float64 `json:"aspect_ratio,omitempty" yaml:"aspect_ratio,omitempty"`

	// MaxPNGWidth caps the pixel width of rasterized PNG output.
	// When the output width (= Width × Scale × 4/3) would exceed this value,
	// the scale is reduced automatically. 0 means no cap.
	MaxPNGWidth int `json:"max_png_width,omitempty" yaml:"max_png_width,omitempty"`

	// StrictFit controls how chart/diagram fit findings affect generation.
	// Values: "off" (skip fit checks), "warn" (default; report findings but proceed),
	// "strict" (refuse generation if unfittable findings exist).
	// Currently accepted but not acted upon — severity promotion will be wired
	// in a follow-up once chart findings are emitted.
	StrictFit string `json:"strict_fit,omitempty" yaml:"strict_fit,omitempty"`
}

// PaletteSpec specifies a color palette by name or by custom hex colors.
// It accepts either a string (built-in palette name like "vibrant") or an
// array of hex color strings (e.g., ["#336699", "#993366"]).
type PaletteSpec struct {
	// Name is the built-in palette name (e.g., "corporate", "vibrant", "muted", "monochrome").
	// Empty when Colors is set.
	Name string

	// Colors is a list of custom hex color strings.
	// Empty when Name is set.
	Colors []string
}

// IsZero returns true if neither a name nor colors are set.
func (p PaletteSpec) IsZero() bool {
	return p.Name == "" && len(p.Colors) == 0
}

// MarshalJSON encodes PaletteSpec as a string (when Name is set) or as an
// array of strings (when Colors is set).
func (p PaletteSpec) MarshalJSON() ([]byte, error) {
	if p.Name != "" {
		return json.Marshal(p.Name)
	}
	if len(p.Colors) > 0 {
		return json.Marshal(p.Colors)
	}
	return []byte("null"), nil
}

// UnmarshalJSON decodes a string or array of strings into PaletteSpec.
func (p *PaletteSpec) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		p.Name = s
		return nil
	}
	// Try array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		p.Colors = arr
		return nil
	}
	return fmt.Errorf("svggen: palette must be a string or array of strings")
}

// MarshalYAML encodes PaletteSpec for YAML output.
func (p PaletteSpec) MarshalYAML() (interface{}, error) {
	if p.Name != "" {
		return p.Name, nil
	}
	if len(p.Colors) > 0 {
		return p.Colors, nil
	}
	return nil, nil
}

// UnmarshalYAML decodes a string or array of strings from YAML into PaletteSpec.
func (p *PaletteSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try string first
	var s string
	if err := unmarshal(&s); err == nil {
		p.Name = s
		return nil
	}
	// Try array of strings
	var arr []string
	if err := unmarshal(&arr); err == nil {
		p.Colors = arr
		return nil
	}
	return fmt.Errorf("svggen: palette must be a string or array of strings")
}

// ThemeColorInput carries a single theme color from the PPTX template.
type ThemeColorInput struct {
	// Name is the theme color name (e.g., "accent1", "accent2", "dk1", "lt1").
	Name string `json:"name" yaml:"name"`

	// RGB is the hex color value (e.g., "#FF0000").
	RGB string `json:"rgb" yaml:"rgb"`
}

// StyleSpec defines theming and appearance options.
type StyleSpec struct {
	// Palette is the color scheme name or custom colors.
	// Built-in palettes: "corporate", "vibrant", "muted", "monochrome".
	// Custom: array of hex colors ["#336699", "#993366", ...].
	Palette PaletteSpec `json:"palette,omitempty" yaml:"palette,omitempty"`

	// FontFamily is the primary font for text.
	// Default: "Arial".
	FontFamily string `json:"font_family,omitempty" yaml:"font_family,omitempty"`

	// Background is the background color.
	// Default: "transparent" for SVG, "white" for raster.
	Background string `json:"background,omitempty" yaml:"background,omitempty"`

	// Surface is the slide surface color (theme lt2).
	// Used for contrast calculations when the visible slide background
	// differs from Background (e.g. template_2's beige surface).
	Surface string `json:"surface,omitempty" yaml:"surface,omitempty"`

	// ThemeColors carries the raw theme color inputs from the PPTX template.
	// When set, StyleGuideFromSpec uses NewPaletteFromThemeColors to build
	// a full palette with semantic colors (Success, Warning, Error, etc.)
	// and text/background colors from the template theme—instead of only
	// carrying the 6 accent hex values.
	ThemeColors []ThemeColorInput `json:"theme_colors,omitempty" yaml:"theme_colors,omitempty"`

	// ShowLegend enables the legend display.
	ShowLegend bool `json:"show_legend,omitempty" yaml:"show_legend,omitempty"`

	// ShowValues enables value labels on data points.
	ShowValues bool `json:"show_values,omitempty" yaml:"show_values,omitempty"`

	// ShowGrid enables background grid lines.
	ShowGrid bool `json:"show_grid,omitempty" yaml:"show_grid,omitempty"`
}

// Validate checks that the RequestEnvelope has required fields.
func (r *RequestEnvelope) Validate() error {
	if r.Type == "" {
		return errors.New("svggen: type is required")
	}
	if r.Data == nil {
		return errors.New("svggen: data is required")
	}
	return nil
}

// SVGDocument represents a rendered SVG document.
type SVGDocument struct {
	// Content is the raw SVG XML content.
	Content []byte

	// Width is the document width in CSS pixels (matching the SVG viewport).
	Width float64

	// Height is the document height in CSS pixels (matching the SVG viewport).
	Height float64

	// ViewBox is the SVG viewBox attribute value.
	ViewBox string

	// FitMode indicates how the content was fit into the container.
	// Empty string or "stretch" means no fit mode was applied.
	FitMode string

	// ContainerWidth is the original container width before fit mode was applied.
	// Only set when FitMode is "contain" or "cover".
	ContainerWidth float64

	// ContainerHeight is the original container height before fit mode was applied.
	// Only set when FitMode is "contain" or "cover".
	ContainerHeight float64

	// OffsetX is the horizontal offset to center the content in the container.
	// Only set when FitMode is "contain" or "cover".
	OffsetX float64

	// OffsetY is the vertical offset to center the content in the container.
	// Only set when FitMode is "contain" or "cover".
	OffsetY float64
}

// String returns the SVG content as a string.
func (d *SVGDocument) String() string {
	return string(d.Content)
}

// Bytes returns the SVG content as bytes.
func (d *SVGDocument) Bytes() []byte {
	return d.Content
}

// RenderResult contains the output of a render operation.
type RenderResult struct {
	// SVG is the rendered SVG document (if format is "svg").
	SVG *SVGDocument

	// PNG is the rendered PNG bytes (if format is "png").
	PNG []byte

	// PDF is the rendered PDF bytes (if format is "pdf").
	PDF []byte

	// Format is the actual output format used.
	Format string

	// Width is the output width.
	Width int

	// Height is the output height.
	Height int
}

// RenderOutput wraps RenderResult with an optional Findings slice.
// Callers that want structured findings opt into this type via
// RenderMultiFormatWithFindings; the existing RenderResult-returning
// functions remain unchanged.
type RenderOutput struct {
	*RenderResult

	// Findings contains structured render-time findings (e.g. clamped values,
	// capacity limits, label truncation). Always non-nil (empty slice when
	// no findings). Each Finding carries a severity and optional fix suggestion.
	Findings []Finding
}

// ParseRequest parses a JSON request into a RequestEnvelope.
func ParseRequest(data []byte) (*RequestEnvelope, error) {
	var req RequestEnvelope
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("svggen: invalid JSON: %w", err)
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return &req, nil
}
