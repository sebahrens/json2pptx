// Package generator provides PPTX file generation from slide specifications.
package generator

import "github.com/ahrens/go-slide-creator/internal/types"

// Re-export SVG configuration types from the types package.
// These types are defined in internal/types to avoid circular dependencies.
type (
	SVGConversionStrategy  = types.SVGConversionStrategy
	SVGNativeCompatibility = types.SVGNativeCompatibility
	SVGConfig              = types.SVGConfig
)

// Re-export SVG strategy constants from the types package.
const (
	SVGStrategyPNG    = types.SVGStrategyPNG
	SVGStrategyEMF    = types.SVGStrategyEMF
	SVGStrategyNative = types.SVGStrategyNative
)

// Re-export SVG compatibility constants from the types package.
const (
	SVGCompatWarn     = types.SVGCompatWarn
	SVGCompatFallback = types.SVGCompatFallback
	SVGCompatStrict   = types.SVGCompatStrict
	SVGCompatIgnore   = types.SVGCompatIgnore
)

// Re-export PNG converter preference constants from the types package.
const (
	PNGConverterAuto  = types.PNGConverterAuto
	PNGConverterRsvg  = types.PNGConverterRsvg
	PNGConverterResvg = types.PNGConverterResvg
)

// Re-export DefaultSVGScale from the types package.
const DefaultSVGScale = types.DefaultSVGScale

// Re-export DefaultMaxPNGWidth from the types package.
const DefaultMaxPNGWidth = types.DefaultMaxPNGWidth

