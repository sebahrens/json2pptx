# PPTX/SVG Rendering Compatibility

Specification for PPTX/SVG rendering behaviors across PowerPoint versions and alternative viewers.

## Overview

This document describes compatibility assumptions, fallback behaviors, and testing requirements for SVG content embedded in generated PPTX files.

## Viewer Compatibility Matrix

| Viewer | Version | Native SVG | EMF Vector | PNG Fallback | Notes |
|--------|---------|------------|------------|--------------|-------|
| Microsoft PowerPoint | 2016+ (v16.0+) | ✅ Native | ✅ | ✅ | Full native SVG support via `asvg:svgBlip` extension |
| Microsoft PowerPoint | 2013 (v15.0) | ❌ PNG fallback | ✅ | ✅ | Ignores SVG extension, displays PNG |
| Microsoft PowerPoint | 2010 and earlier | ❌ PNG fallback | ✅ | ✅ | No SVG support |
| LibreOffice Impress | All | ❌ PNG fallback | ✅ | ✅ | Does not recognize `asvg:svgBlip` |
| Google Slides | Web | ✅ Native | ❌ | ✅ | Converts on import |
| Apple Keynote | macOS/iOS | ⚠️ Varies | ✅ | ✅ | May convert to shapes |
| Web Browsers | Modern | ✅ Native | ❌ | ✅ | Direct SVG rendering |

## Conversion Strategies

### Strategy: PNG (Default)

**Behavior:** SVG converted to high-resolution PNG (default 2x scale)

**Dependencies:**
- `rsvg-convert` (librsvg) - Primary converter
- `resvg` (Rust-based) - Fallback converter

**Compatibility:** Universal - works with all viewers

**Trade-offs:**
- ✅ Maximum compatibility
- ✅ Predictable rendering
- ❌ Loss of vector scaling
- ❌ Larger file size for simple graphics

### Strategy: EMF

**Behavior:** SVG converted to Enhanced Metafile format

**Dependencies:**
- `inkscape` CLI (required)

**Compatibility:** PowerPoint 2003+, LibreOffice, most viewers

**Trade-offs:**
- ✅ Vector quality preserved
- ✅ Wide compatibility (since 2003)
- ❌ Conversion quality varies by SVG complexity
- ❌ External tool dependency

### Strategy: Native

**Behavior:** SVG embedded with PNG fallback using `asvg:svgBlip` OOXML extension

**Dependencies:** None (PNG fallback data required)

**Compatibility:** PowerPoint 2016+ for native; older viewers see PNG fallback

**Trade-offs:**
- ✅ Crisp scaling at any resolution
- ✅ Editable in PowerPoint (can ungroup to shapes)
- ⚠️ Older viewers show rasterized fallback
- ⚠️ File contains both SVG and PNG (larger for complex SVGs)

## OOXML Native SVG Structure

```xml
<a:blip r:embed="rId2">  <!-- PNG fallback -->
  <a:extLst>
    <a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}">
      <asvg:svgBlip xmlns:asvg="http://schemas.microsoft.com/office/drawing/2016/SVG/main"
                    r:embed="rId3"/>  <!-- SVG file -->
    </a:ext>
  </a:extLst>
</a:blip>
```

**Key Constants:**
- SVG Extension GUID: `{96DAC541-7B7A-43D3-8B79-37D633B846F1}`
- SVG Namespace: `http://schemas.microsoft.com/office/drawing/2016/SVG/main`
- Relationship Type: `http://schemas.openxmlformats.org/officeDocument/2006/relationships/image`

## Content Types

Generated PPTX files include these content types in `[Content_Types].xml`:

```xml
<Default Extension="png" ContentType="image/png"/>
<Default Extension="svg" ContentType="image/svg+xml"/>
<Default Extension="emf" ContentType="image/x-emf"/>
```

## Compatibility Detection

The system detects PowerPoint version from template's `docProps/app.xml`:

```xml
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Microsoft Macintosh PowerPoint</Application>
  <AppVersion>16.0000</AppVersion>
</Properties>
```

**Version Interpretation:**
- `16.xxxx` → PowerPoint 2016+ (native SVG supported)
- `15.xxxx` → PowerPoint 2013 (use fallback)
- `14.xxxx` → PowerPoint 2010 (use fallback)
- Missing/Unknown → Assume compatible (optimistic default)

## Compatibility Modes

The `SVGNativeCompatibility` setting controls behavior when native SVG may not be supported:

| Mode | Behavior |
|------|----------|
| `warn` (default) | Log warning, proceed with native strategy |
| `fallback` | Auto-switch to PNG if version unknown or unsupported |
| `strict` | Fail with error if native not confirmed supported |
| `ignore` | Skip all compatibility checks |

## Known Rendering Differences

### Font Handling

- **System fonts:** Must be installed on viewing system
- **Embedded fonts:** May not render in PNG fallback
- **Web fonts:** Not supported in PPTX context

**Recommendation:** Use common system fonts (Arial, Calibri, Times New Roman) for maximum compatibility.

### Complex SVG Features

| Feature | Native SVG | PNG Conversion | EMF Conversion |
|---------|------------|----------------|----------------|
| Gradients | ✅ | ✅ | ⚠️ May simplify |
| Drop shadows | ✅ | ✅ | ⚠️ May lose |
| Filters (blur, etc.) | ⚠️ May differ | ✅ | ❌ Often lost |
| CSS animations | ❌ | ❌ | ❌ |
| JavaScript | ❌ | ❌ | ❌ |
| External resources | ❌ | ⚠️ Must be inlined | ⚠️ Must be inlined |

### Color Accuracy

- **RGB colors:** Consistent across viewers
- **Color profiles:** May vary between viewers
- **Transparency:** Generally well-supported

## Testing Requirements

### E2E Test Coverage

The following scenarios must be tested:

1. **Native SVG Embedding** (`svg_e2e_test.go`)
   - Insert SVG with PNG fallback
   - Verify `asvg:svgBlip` extension in slide XML
   - Verify both media files present (SVG + PNG)
   - Verify relationship IDs correct

2. **PNG Fallback** (`svg_fallback_e2e_test.go`)
   - Configure PNG-only strategy
   - Verify SVG converted to PNG
   - Verify no `asvg:svgBlip` extension
   - Verify PNG quality at configured scale

3. **Compatibility Detection** (`svg_compat_test.go`)
   - PowerPoint 2016 template → native supported
   - PowerPoint 2013 template → fallback recommended
   - Unknown version → optimistic default with warning

### Failure Diagnostics

When tests fail, error messages must indicate:

1. **Which viewer/version assumption was violated**
   - Example: `"expected native SVG support for PowerPoint 16.0, but asvg:svgBlip extension missing"`

2. **Which OOXML structure is incorrect**
   - Example: `"slide XML missing required pattern: {96DAC541-7B7A-43D3-8B79-37D633B846F1}"`

3. **Which fallback scenario failed**
   - Example: `"PNG fallback test failed: expected 1 media file, got 2 (SVG should not be present)"`

## Configuration

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `SVG_PNG_CONVERTER` | Preferred PNG converter | `auto` |
| `SVG_SCALE` | PNG conversion scale factor | `2.0` |

### API Configuration

```go
type SVGConfig struct {
    Strategy            SVGConversionStrategy   // "png", "emf", "native"
    Scale               float64                 // Default: 2.0
    NativeCompatibility SVGNativeCompatibility  // warn/fallback/strict/ignore
    PreferredPNGConverter string                // auto/rsvg-convert/resvg
}
```

## Troubleshooting

### SVG Not Displaying in PowerPoint

1. Check PowerPoint version (need 2016+ for native)
2. Verify `[Content_Types].xml` includes `image/svg+xml`
3. Verify relationship IDs match between slide and rels file
4. Try opening with LibreOffice to see PNG fallback

### Blurry Images

1. Increase `SVGConfig.Scale` (try 3.0 or 4.0)
2. Check source SVG dimensions
3. Verify template slide dimensions match expected aspect ratio

### File Size Too Large

1. Switch from native to PNG-only strategy (native includes both files)
2. Reduce `SVGConfig.Scale` if using PNG
3. Optimize source SVG (remove unused elements, simplify paths)

## References

- [PowerPoint SVG Support (Stratadata)](https://www.stratadata.co.uk/blog/index.php/2017/11/02/using-svg-files-in-microsoft-office/)
- [SVG File Size Issues (Neuxpower)](https://neuxpower.com/blog/why-does-adding-svg-images-to-powerpoint-sometimes-make-the-file-so-large)
- [OOXML Anatomy](http://officeopenxml.com/anatomyofOOXML-pptx.php)
- [Office File Formats (Indezine)](https://www.indezine.com/products/powerpoint/learn/interface/365/file-formats.html)

## Related Files

| File | Purpose |
|------|---------|
| `internal/generator/svg.go` | SVG conversion engine |
| `internal/generator/svg_compat.go` | Compatibility detection |
| `internal/generator/native_svg.go` | Native SVG embedding |
| `internal/pptx/picgen.go` | `asvg:svgBlip` XML generation |
| `specs/15-svg-handling-decision.md` | Strategy decision document |
