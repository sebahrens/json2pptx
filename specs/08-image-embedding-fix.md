# Image Embedding Fix

**Status: IMPLEMENTED** ✅

Complete the PPTX picture element structure to properly embed images.

## Scope

This specification covers ONLY fixing the incomplete image embedding. It does NOT cover:
- Adding new dependencies
- Rewriting chart rendering

## Implementation Status

The image embedding feature has been fully implemented. All acceptance criteria pass.

## Implemented OOXML Structure

A picture element in PPTX is generated with the complete structure:

```xml
<p:pic>
  <p:nvPicPr>
    <p:cNvPr id="4" name="Picture 4"/>
    <p:cNvPicPr>
      <a:picLocks noChangeAspect="1"/>
    </p:cNvPicPr>
    <p:nvPr/>
  </p:nvPicPr>
  <p:blipFill>
    <a:blip r:embed="rId2"/>
    <a:stretch>
      <a:fillRect/>
    </a:stretch>
  </p:blipFill>
  <p:spPr>
    <a:xfrm>
      <a:off x="1524000" y="1397000"/>
      <a:ext cx="4572000" cy="3429000"/>
    </a:xfrm>
    <a:prstGeom prst="rect">
      <a:avLst/>
    </a:prstGeom>
  </p:spPr>
</p:pic>
```

## Implementation Details

### 1. Picture XML Types

Implemented in `internal/pptx/xml_types.go`:

```go
// PictureXML represents a picture (p:pic) element.
type PictureXML struct {
    XMLName  xml.Name           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic"`
    NvPicPr  NvPicPropertiesXML `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvPicPr"`
    BlipFill BlipFillXML        `xml:"http://schemas.openxmlformats.org/presentationml/2006/main blipFill"`
    SpPr     PicSpPrXML         `xml:"http://schemas.openxmlformats.org/presentationml/2006/main spPr"`
}

type NvPicPropertiesXML struct {
    CNvPr    CNvPrXML    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvPr"`
    CNvPicPr CNvPicPrXML `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvPicPr"`
    NvPr     struct{}    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvPr"`
}

type CNvPrXML struct {
    ID   uint32 `xml:"id,attr"`
    Name string `xml:"name,attr"`
}

type CNvPicPrXML struct {
    PicLocks PicLocksXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main picLocks"`
}

type PicLocksXML struct {
    NoChangeAspect string `xml:"noChangeAspect,attr"`
}

type BlipFillXML struct {
    Blip    BlipXML    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip"`
    Stretch StretchXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch"`
}

type BlipXML struct {
    Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
}

type StretchXML struct {
    FillRect struct{} `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillRect"`
}
```

### 2. Picture XML Generation

Two approaches are implemented:

**a) Template-based generation** (`internal/pptx/picgen.go`):
- `GeneratePic(opts PicOptions)` - Full-featured with optional SVG blip extension
- `GeneratePicSimple()` - Convenience wrapper with namespace declarations  
- `GeneratePicSimpleNoNS()` - For insertion into existing slides
- `GeneratePicWithSVG()` - Native SVG with PNG fallback support

Key options in `PicOptions`:
- `ID`, `Name`, `Description` - Non-visual properties
- `PNGRelID`, `SVGRelID` - Relationship IDs for blip embedding
- `OffsetX`, `OffsetY`, `ExtentCX`, `ExtentCY` - Position/size in EMUs
- `OmitNamespaces` - For insertion into slides with existing namespace declarations

**b) Single-pass processing** (`internal/generator/singlepass.go`):
- `processImageContent()` - Validates path, checks file existence, routes to appropriate handler
- `processRegularImage()` - Scales image, allocates media slot, updates shape properties
- `processSVGImage()` - Routes to native SVG or conversion strategy
- `processNativeSVG()` - Generates PNG fallback, creates native SVG insert with dual media files

### 3. Media & Relationship Management

**MediaInserter** (`internal/pptx/media.go`):
- `NewMediaInserter(pkg)` - Scans existing media, parses content types
- `InsertPNG()`, `InsertSVG()` - Allocate unique paths, register content types
- `InsertSVGWithFallback()` - Dual-file insertion for native SVG
- `Finalize()` - Writes updated `[Content_Types].xml`

**Relationship handling**:
- Relationships parsed from existing `.rels` files
- IDs allocated by incrementing from max existing ID + 1
- Slide relationships updated via `slideRelUpdates` map in single-pass context

### 4. Image Scaling

Implemented in `internal/utils/image.go`:
- `ScaleImageToFit(imagePath, bounds)` - Scales to fit placeholder while maintaining aspect ratio
- Returns centered `BoundingBox` with X, Y, Width, Height in EMUs

### 5. Security

Implemented in `internal/generator/images.go` and `internal/utils/path.go`:
- `ValidateImagePathWithConfig()` - Prevents path traversal (CRIT-03)
- Checks for `..` components
- Validates against allowed base directories

## Acceptance Criteria

### AC5: Image Embedding ✅ PASS
- `TestGenerate_ImageEmbedding_AC5` verifies image appears at placeholder position

### AC6: Image Scaling ✅ PASS
- `TestGenerate_ImageScaling_AC6` verifies large images scaled to fit with aspect ratio preserved

### AC-NEW1: Picture XML Structure ✅ PASS
- `TestPictureXML_Marshal`, `TestPictureXML_Unmarshal`, `TestPictureXML_RoundTrip` verify correct XML structure

### AC-NEW2: Relationship Linkage ✅ PASS
- Integration tests verify `.rels` files contain correct relationship entries

### AC-NEW3: Media File Placement ✅ PASS
- Images placed at `ppt/media/imageN.ext` with unique naming

### AC-NEW4: Content-Types Update ✅ PASS
- `TestUpdateContentTypes_Singlepass` verifies extension mappings added

### AC-NEW5: Multiple Images Per Slide ✅ PASS
- `TestGenerate_MultipleImagesPerSlide` verifies 2 images with unique relationship IDs

## Error Handling

| Scenario | Behavior | Test Coverage |
|----------|----------|---------------|
| Image file not found | Warning, skip image | `TestProcessImageContent_EdgeCases` |
| Unsupported image format | Uses extension-based MIME mapping | `imageExtensionContentTypes` map |
| No placeholder for image | Warning, skip image | `TestPrepareImages_MissingPlaceholder` |
| Path traversal attempt | Security warning, skip | `TestProcessImageContent_SecurityValidation` |
| Path outside allowed dirs | Security warning, skip | `TestValidateImagePath_AllowedBasePaths` |

## Tests

All originally specified tests pass:

| Test | Status |
|------|--------|
| `TestPictureXML_Marshal` | ✅ PASS |
| `TestPictureXML_Unmarshal` | ✅ PASS |
| `TestPictureXML_RoundTrip` | ✅ PASS |
| `TestPictureXML_MultipleRelationshipIDs` | ✅ PASS |
| `TestScaleImageToFit` | ✅ PASS |
| `TestGenerate_ImageEmbedding_AC5` | ✅ PASS |
| `TestGenerate_ImageScaling_AC6` | ✅ PASS |
| `TestGenerate_MultipleImagesPerSlide` | ✅ PASS |
| `TestProcessImageContent_EdgeCases` | ✅ PASS |
| `TestProcessImageContent_SecurityValidation` | ✅ PASS |

## Additional Features Implemented

Beyond the original spec:

1. **SVG Support** - Native SVG embedding with PNG fallback (`SVGStrategyNative`), or conversion to PNG/EMF
2. **Streaming** - `streamImageToZip()` uses 32KB chunked streaming to minimize memory
3. **Deduplication** - `allocateMediaSlot()` tracks images to avoid duplicate media files
4. **Alt Text** - `PicOptions.Description` maps to `cNvPr/@descr` for accessibility

## Out of Scope

- Adding `beevik/etree` dependency (not needed)
- Rewriting to use different XML library (not needed)
- Animated GIF support (frames not extracted)
