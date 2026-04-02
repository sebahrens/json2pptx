# SVG Handling Strategy Decision

Decision document for A13: How to handle SVG content in PPTX generation.

## Decision

**Strategy: Multi-Strategy SVG Handling with PNG as Default**

The implementation supports three conversion strategies, selectable via `SVGConfig.Strategy`:
1. **PNG Rasterization** (default) - Universal compatibility via `rsvg-convert`
2. **EMF Conversion** - Vector quality preserved, requires Inkscape
3. **Native SVG Embedding** - Best quality for PowerPoint 2016+, includes PNG fallback

All three strategies are fully implemented. PNG remains the default for maximum compatibility.

## Options Evaluated

### Option A: Native SVG Embedding (Implemented as `SVGStrategyNative`)

**How it works**: PowerPoint 2016+ supports native SVG via the `asvg:svgBlip` extension in `<a:extLst>`:
```xml
<a:blip r:embed="rId2">
  <a:extLst>
    <a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}">
      <asvg:svgBlip xmlns:asvg="http://schemas.microsoft.com/office/drawing/2016/SVG/main"
                    r:embed="rId3"/>
    </a:ext>
  </a:extLst>
</a:blip>
```
The `r:embed="rId2"` references a PNG fallback, while `rId3` references the SVG.

**Pros**:
- Crisp scaling at any resolution
- Smaller file size for complex vector art
- Editable in PowerPoint (can ungroup to shapes)

**Cons**:
- Requires PowerPoint 2016+ (Office 365, 2019, 2021)
- PowerPoint auto-generates PNG fallback on import (can be 100x larger than SVG)
- LibreOffice and older PowerPoint versions show fallback only
- Requires maintaining two media files per image (SVG + PNG fallback)

**Implementation**: Fully implemented via `InsertSVG()` in `internal/pptx/api.go` and `GeneratePic()` with `SVGRelID` in `internal/pptx/picgen.go`. Includes compatibility checking via `SVGCompatibilityChecker` that parses `docProps/app.xml` to detect PowerPoint version.

### Option B: EMF Conversion (Implemented as `SVGStrategyEMF`)

**How it works**: Convert SVG to Enhanced Metafile (EMF) format, which has been supported since PowerPoint 2003.

**Pros**:
- Vector quality preserved
- Works with PowerPoint 2003+
- Single file (no fallback needed)

**Cons**:
- Requires Inkscape CLI (`inkscape --export-type=emf`)
- EMF conversion quality varies by tool
- Complex path/gradient support may degrade
- Runtime dependency on system tools

**Implementation**: Fully implemented in `SVGConverter.ConvertToEMF()`. Requires Inkscape to be installed and available in PATH. The converter auto-detects Inkscape availability via `exec.LookPath("inkscape")`.

### Option C: PNG Rasterization (Implemented as `SVGStrategyPNG` - Default)

**How it works**: Convert SVG to PNG at high resolution (default: 2x scale) during generation.

**Pros**:
- Universal compatibility (any PowerPoint, LibreOffice, Keynote)
- Predictable output quality
- Already established pattern in codebase for chart rendering

**Cons**:
- Loss of vector scaling
- Larger file size for simple vector graphics
- Not editable in PowerPoint
- Requires `rsvg-convert` (librsvg) external tool

**Implementation**: Default strategy. Implemented in `SVGConverter.ConvertToPNG()` using `rsvg-convert -z <scale> -o output.png input.svg`. Scale is configurable via `SVGConfig.Scale` (default: 2.0).

## Implementation Details

### Actual Implementation (All Phases Complete)

All three strategies are implemented in `internal/generator/svg.go` and `internal/generator/svg_types.go`:

**Type definitions** (`svg_types.go`):
```go
type SVGConversionStrategy string

const (
    SVGStrategyPNG    SVGConversionStrategy = "png"    // Default
    SVGStrategyEMF    SVGConversionStrategy = "emf"    // Requires Inkscape
    SVGStrategyNative SVGConversionStrategy = "native" // PowerPoint 2016+
)

type SVGNativeCompatibility string

const (
    SVGCompatWarn     SVGNativeCompatibility = "warn"     // Default: warn but proceed
    SVGCompatFallback SVGNativeCompatibility = "fallback" // Auto-fallback to PNG
    SVGCompatStrict   SVGNativeCompatibility = "strict"   // Fail if incompatible
    SVGCompatIgnore   SVGNativeCompatibility = "ignore"   // No checks
)

type SVGConfig struct {
    Strategy            SVGConversionStrategy   `yaml:"strategy"`
    Scale               float64                 `yaml:"scale"` // Default: 2.0
    NativeCompatibility SVGNativeCompatibility  `yaml:"native_compatibility"`
}
```

**Core converter** (`svg.go`):
- `SVGConverter` struct with `Convert()`, `ConvertToPNG()`, `ConvertToEMF()` methods
- `IsSVGFile()` - Detection by extension and content magic bytes
- `IsAvailable()`, `IsPNGAvailable()`, `IsEMFAvailable()` - Tool availability checks
- Package-level `ConvertSVGToPNG()` convenience function

**Native SVG embedding** (`internal/pptx/`):
- `api.go`: `InsertSVG()` - High-level API requiring both SVG and PNG fallback data
- `picgen.go`: `GeneratePic()` - Generates `asvg:svgBlip` extension when `SVGRelID` is provided
- `media.go`: `InsertSVGWithFallback()` - Stores both media files

**Compatibility checking** (`svg_compat.go`):
- `SVGCompatibilityChecker` - Parses `docProps/app.xml` to detect PowerPoint version
- `CheckSVGCompatibility()` - Analyzes template for native SVG support
- `GenerateWarning()`, `ShouldFallback()`, `CheckStrict()` - Compatibility mode handlers

## Dependencies

### PNG Strategy (Default)
- `rsvg-convert` (librsvg-bin) - Primary PNG converter
- `resvg` (Rust-based alternative) - Alternative PNG converter with fallback support
- Converter preference configurable via `SVGConfig.PreferredPNGConverter`:
  - `"auto"` (default): tries rsvg-convert first, then resvg as fallback
  - `"rsvg-convert"`: forces rsvg-convert only
  - `"resvg"`: forces resvg only
- Environment variable: `SVG_PNG_CONVERTER`
- Fallback: Returns error if no converter available (use `IsAvailable()` to check)

### EMF Strategy
- Inkscape CLI - Required for EMF conversion
- Auto-detected via `exec.LookPath("inkscape")`

### Native Strategy
- No external tools required
- PNG fallback still required (must provide PNG data to `InsertSVG()`)

## Testing Requirements

1. **SVG Detection**: Correctly identify SVG files by extension and content
2. **PNG Conversion**: Verify PNG output from test SVG files
3. **EMF Conversion**: Verify EMF output when Inkscape available
4. **Native Embedding**: Verify `asvg:svgBlip` XML generation
5. **Resolution Scaling**: Test 1x, 2x, and custom scale settings
6. **Tool Availability**: `IsAvailable()`, `IsPNGAvailable()`, `IsEMFAvailable()` checks
7. **Compatibility Checking**: Version detection from `docProps/app.xml`
8. **Compatibility Modes**: Test warn/fallback/strict/ignore behaviors
9. **Complex SVG**: Test gradients, filters, text, embedded fonts
10. **Visual Verification**: Compare rendered output across PowerPoint versions

## Acceptance Criteria

For A13 (this decision):
- [x] Clear documented decision exists
- [x] Implications documented (quality, tooling, performance)
- [x] Required dependencies identified
- [x] All three strategies implemented (PNG, EMF, Native)
- [x] Compatibility checking implemented for native SVG
- [x] Configuration via `SVGConfig` struct

## References

- [PowerPoint SVG Support](https://www.stratadata.co.uk/blog/index.php/2017/11/02/using-svg-files-in-microsoft-office/) - SVG behavior in Office
- [PptxGenJS](https://github.com/gitbrent/PptxGenJS) - JavaScript library that rasterizes SVG to PNG
- [neuxpower SVG analysis](https://neuxpower.com/blog/why-does-adding-svg-images-to-powerpoint-sometimes-make-the-file-so-large) - SVG file size issues
- [OOXML Anatomy](http://officeopenxml.com/anatomyofOOXML-pptx.php) - PPTX structure reference
- [Office File Formats](https://www.indezine.com/products/powerpoint/learn/interface/365/file-formats.html) - Supported formats overview

## Downstream Task Updates

This decision unblocks (all now completed):
- **A14**: Implement SVG ingestion pipeline ✓
- **A16**: Add config toggle for SVG conversion strategy ✓ (all strategies available)
- **A23**: Allow chart output as SVG ✓ (via `internal/svggen/builder.go`)

## Key Files

| File | Purpose |
|------|---------|
| `internal/generator/svg.go` | SVGConverter with PNG/EMF conversion |
| `internal/generator/svg_types.go` | SVGConfig, strategy constants |
| `internal/generator/svg_compat.go` | PowerPoint version detection |
| `internal/pptx/api.go` | `InsertSVG()` API |
| `internal/pptx/picgen.go` | `asvg:svgBlip` XML generation |
| `internal/pptx/media.go` | Media file storage |
| `internal/svggen/builder.go` | Programmatic SVG creation |
