// Package types provides shared data structures for the slide generator.
package types

// SVGConversionStrategy defines how SVG files are converted for PPTX embedding.
type SVGConversionStrategy string

const (
	// SVGStrategyPNG converts SVG to PNG (default, universal compatibility).
	SVGStrategyPNG SVGConversionStrategy = "png"
	// SVGStrategyEMF converts SVG to EMF (vector quality, requires Inkscape).
	SVGStrategyEMF SVGConversionStrategy = "emf"
	// SVGStrategyNative embeds SVG directly using svgBlip (best quality, PowerPoint 365+).
	SVGStrategyNative SVGConversionStrategy = "native"
)

// SVGNativeCompatibility defines how to handle native SVG compatibility concerns.
type SVGNativeCompatibility string

const (
	// SVGCompatWarn logs a warning but proceeds with native SVG (default).
	// The output will include PNG fallbacks for older viewers.
	SVGCompatWarn SVGNativeCompatibility = "warn"

	// SVGCompatFallback automatically falls back to PNG strategy if
	// compatibility cannot be confirmed. Produces universal output.
	SVGCompatFallback SVGNativeCompatibility = "fallback"

	// SVGCompatStrict fails generation if native SVG is requested
	// but compatibility cannot be confirmed.
	SVGCompatStrict SVGNativeCompatibility = "strict"

	// SVGCompatIgnore proceeds without any compatibility checks or warnings.
	// Use only when target viewer is known to support native SVG.
	SVGCompatIgnore SVGNativeCompatibility = "ignore"
)

// SVGConfig holds configuration for SVG file handling.
type SVGConfig struct {
	// Strategy selects how SVG files are converted for PPTX embedding.
	// Options: "png" (default), "emf", "native"
	// PNG: Universal compatibility, rasterized output. Requires rsvg-convert or resvg.
	// EMF: Vector quality preserved, works with PowerPoint 2010+. Requires Inkscape.
	// Native: Direct SVG embedding via svgBlip, best quality, requires PowerPoint 2016+.
	Strategy SVGConversionStrategy `yaml:"strategy"`

	// Scale factor for PNG conversion (default: 2.0 = 2x resolution).
	// Only used when Strategy is "png". Higher values produce larger, sharper images.
	Scale float64 `yaml:"scale"`

	// NativeCompatibility controls behavior when native SVG strategy is used.
	// Options: "warn" (default), "fallback", "strict", "ignore"
	// Native SVG requires PowerPoint 2016+ (version 16.0+). Older versions
	// will display the PNG fallback image instead of the SVG.
	NativeCompatibility SVGNativeCompatibility `yaml:"native_compatibility"`

	// PreferredPNGConverter specifies which PNG converter to try first.
	// Options: "auto" (default), "rsvg-convert", "resvg"
	// - "auto": tries rsvg-convert first, then resvg as fallback
	// - "rsvg-convert": forces rsvg-convert only (librsvg)
	// - "resvg": forces resvg only (Rust-based alternative)
	// Only used when Strategy is "png".
	PreferredPNGConverter string `yaml:"preferred_png_converter"`

	// MaxPNGWidth caps the pixel width of PNG fallback images for SVG content.
	// PowerPoint re-saves images at ~2500px, so embedding larger PNGs wastes
	// file size. When the rendered width would exceed this value, the scale
	// factor is reduced automatically. Set to 0 to disable the cap.
	// Default: 2500.
	MaxPNGWidth int `yaml:"max_png_width"`
}

// DefaultSVGScale is the default scale factor for SVG to PNG conversion.
// 2x provides good quality for most presentations without excessive file size.
const DefaultSVGScale = 2.0

// DefaultMaxPNGWidth is the default maximum pixel width for PNG fallback images.
// PowerPoint caps embedded images at approximately 2500px on re-save.
const DefaultMaxPNGWidth = 2500

// PNG converter preference constants.
const (
	// PNGConverterAuto uses rsvg-convert first, then resvg as fallback.
	PNGConverterAuto = "auto"
	// PNGConverterRsvg forces use of rsvg-convert only.
	PNGConverterRsvg = "rsvg-convert"
	// PNGConverterResvg forces use of resvg only.
	PNGConverterResvg = "resvg"
)
