# PPTX Generator

Generate PowerPoint files by populating template layouts with content.

## Scope

This specification covers ONLY PPTX file generation. It does NOT cover:
- JSON input parsing (see `docs/INPUT_FORMAT.md`)
- Template analysis (see `02-template-analyzer.md`)
- Layout selection (see `03-layout-selector.md`)
- Chart rendering (see `05-chart-renderer.md`)

## Purpose

Given a template, layout assignments, and content mappings, produce a valid PPTX file.

## Input

```go
type GenerationRequest struct {
    TemplatePath      string      // Path to template PPTX file
    OutputPath        string      // Where to write generated PPTX
    Slides            []SlideSpec // Slide specifications
    AllowedImagePaths []string    // Allowed base paths for image loading (security)
    SVGStrategy       string      // SVG conversion strategy: "png" (default), "emf", or "native"
    SVGScale          float64     // Scale factor for SVG to PNG conversion (default: 2.0)
    SVGNativeCompat   string      // Native SVG compatibility mode: "warn" (default), "fallback", "strict", "ignore"
}

type SlideSpec struct {
    LayoutID string        // Layout to use (e.g., "slideLayout1")
    Content  []ContentItem // Content items to populate
}

type ContentItem struct {
    PlaceholderID string      // Target placeholder ID (shape name, placeholder type, or index)
    Type          ContentType // Type of content
    Value         any         // Type-specific value
}

type ContentType string
const (
    ContentText           ContentType = "text"
    ContentBullets        ContentType = "bullets"
    ContentBodyAndBullets ContentType = "body_and_bullets" // Body text followed by bullets
    ContentBulletGroups   ContentType = "bullet_groups"    // Grouped bullets with section headers
    ContentImage          ContentType = "image"
    ContentChart          ContentType = "chart"
    ContentTable          ContentType = "table"            // Data table
    ContentDiagram        ContentType = "diagram"          // Unified diagram type (charts, infographics)
)
```

Content value types by ContentType:
- `text`: `string`
- `bullets`: `[]string`
- `body_and_bullets`: `BodyAndBulletsContent` struct
- `bullet_groups`: `[]BulletGroup` (grouped bullets with section headers)
- `image`: `ImageContent` struct
- `chart`: `*types.ChartSpec` (chart specification, rendered during generation)
- `table`: `*types.TableSpec` (data table specification)
- `diagram`: `*types.DiagramSpec` (unified diagram specification)

```go
type BodyAndBulletsContent struct {
    Body    string   // Body text paragraph (no bullet marker)
    Bullets []string // Bullet points (with bullet markers)
}

type ImageContent struct {
    Path   string             // File path to image
    Alt    string             // Alt text for accessibility
    Bounds *types.BoundingBox // Optional bounds override
}
```

## Output

A `.pptx` file written to OutputPath.

Return value:
```go
type GenerationResult struct {
    OutputPath   string
    FileSize     int64
    SlideCount   int
    Warnings     []string
    Duration     time.Duration
}
```

## Generator Interface

The generator provides a pluggable interface for dependency injection and testing:

```go
type Generator interface {
    Generate(req GenerationRequest) (*GenerationResult, error)
}

type DefaultGenerator struct{}

func NewGenerator() *DefaultGenerator
func (g *DefaultGenerator) Generate(req GenerationRequest) (*GenerationResult, error)
```

## Generation Process

The implementation uses a **single-pass ZIP generation** for optimal performance:

1. **Initialize Context**: Open template as ZIP reader, create output file
2. **Scan Template**: Count existing slides, find max media number, parse theme colors
3. **Prepare Slides**: Create slide XML from layouts, resolve empty transforms from masters
4. **Pre-render Charts**: Render all charts inline during single-pass generation
5. **Prepare Images**: Validate paths, scale images, track media files
6. **Write Output**: Single-pass copy unchanged files, write modified/new files, add media
7. **Finalize**: Close ZIP, move temp file to output path

This approach performs only ONE ZIP read and ONE ZIP write operation (vs. 3x in naive implementation).

## Placeholder Population

### Text Content

Replace placeholder text run with new content:

```xml
<!-- Before -->
<p:txBody>
  <a:p>
    <a:r><a:t>Click to add title</a:t></a:r>
  </a:p>
</p:txBody>

<!-- After -->
<p:txBody>
  <a:p>
    <a:r><a:t>Actual Title Text</a:t></a:r>
  </a:p>
</p:txBody>
```

### Bullet Content

Create paragraph per bullet with bullet marker:

```xml
<p:txBody>
  <a:p>
    <a:pPr><a:buChar char="•"/></a:pPr>
    <a:r><a:t>First bullet</a:t></a:r>
  </a:p>
  <a:p>
    <a:pPr><a:buChar char="•"/></a:pPr>
    <a:r><a:t>Second bullet</a:t></a:r>
  </a:p>
</p:txBody>
```

### Image Content

1. Copy image file to `ppt/media/imageN.png`
2. Add relationship in slide's .rels file
3. Replace placeholder with picture element:

```xml
<p:pic>
  <p:nvPicPr>...</p:nvPicPr>
  <p:blipFill>
    <a:blip r:embed="rId2"/>  <!-- relationship ID -->
  </p:blipFill>
  <p:spPr>
    <a:xfrm>
      <a:off x="..." y="..."/>
      <a:ext cx="..." cy="..."/>
    </a:xfrm>
  </p:spPr>
</p:pic>
```

### Body and Bullets Content

Body text followed by bullet points in the same placeholder:

```xml
<p:txBody>
  <a:p>
    <!-- Body paragraph: no bullet marker, inherits lstStyle from layout -->
    <a:r><a:t>Introductory body text without bullet</a:t></a:r>
  </a:p>
  <a:p>
    <a:pPr lvl="0">...</a:pPr>
    <a:r><a:t>First bullet point</a:t></a:r>
  </a:p>
  <a:p>
    <a:pPr lvl="0">...</a:pPr>
    <a:r><a:t>Second bullet point</a:t></a:r>
  </a:p>
</p:txBody>
```

### Chart Content

Charts are rendered during generation using `*types.ChartSpec`:

1. Chart specs are rendered inline during single-pass ZIP generation
2. Theme colors from template are injected into chart styling
3. Rendered PNG is embedded as image
4. No embedded Excel data (static image only)

See `05-chart-renderer.md` for chart specification format.

## Styling Preservation

Content inherits styling from template:
- Font family from theme
- Font size from placeholder default
- Colors from theme color scheme
- Bullet style from master slide

Do NOT override template styling unless explicitly requested.

## Slide Transitions

Slide transitions are supported via `internal/generator/transition.go`. Each slide can specify a transition effect applied when entering the slide.

Supported transition types: `fade`, `push`, `wipe`, `cover`, `cut`.

## Bullet Build Animations

Bullet points can be configured with build animations (appear one-by-one) via the transition system.

## Speaker Notes

Speaker notes are supported via `internal/generator/notes.go`. Each slide can include presenter notes that appear in PowerPoint's Notes pane.

## URL Images

Remote images (HTTP/HTTPS URLs) are supported via `internal/resource/resolver.go`. URLs are fetched at generation time with SSRF protection (private IP ranges blocked via `internal/resource/ssrf.go`).

## Acceptance Criteria

### AC1: Valid PPTX Output
- Given valid template and content
- When generated
- Then output file opens in PowerPoint without errors

### AC2: Slide Count
- Given request with N slides
- When generated
- Then output contains exactly N slides

### AC3: Text Population
- Given text content for title placeholder
- When generated
- Then slide title displays the text

### AC4: Bullet Population
- Given 5 bullet strings
- When generated
- Then slide shows 5 bulleted paragraphs

### AC5: Image Embedding
- Given valid image path
- When generated
- Then image appears in slide at placeholder position

### AC6: Image Scaling
- Given image larger than placeholder
- When generated
- Then image is scaled to fit (maintaining aspect ratio)

### AC7: Missing Placeholder
- Given content for non-existent placeholder ID
- When generated
- Then warning added, content skipped (not an error)

### AC8: Theme Preservation
- Given template with custom theme colors
- When generated
- Then output uses same theme colors

### AC9: Layout Accuracy
- Given specific layout ID
- When generated
- Then slide uses that layout (visual inspection)

### AC10: File Size Reasonable
- Given 10-slide presentation with 3 images
- When generated
- Then file size < 10MB

### AC11: Unicode Support
- Given text with non-ASCII characters (emoji, CJK)
- When generated
- Then characters render correctly

### AC12: Empty Content Handling
- Given slide with no content items
- When generated
- Then slide created with placeholder text removed

## Placeholder Lookup

Placeholders are resolved using multiple lookup strategies (in priority order):

1. **Shape name** (primary): e.g., "Title 1", "Content Placeholder 2"
2. **Placeholder type**: e.g., "title", "body", "ctrTitle", "pic"
3. **Placeholder index**: e.g., "1", "2", "13"

This allows flexible content targeting based on layout structure.

## SVG Handling

SVG images support multiple conversion strategies:

| Strategy | Description | Requirements |
|----------|-------------|--------------|
| `png` (default) | Convert SVG to PNG | rsvg-convert |
| `emf` | Convert SVG to EMF (vector) | Inkscape |
| `native` | Direct SVG embedding via svgBlip | PowerPoint 2016+ |

Native SVG compatibility modes:
- `warn`: Log warning, proceed with native (includes PNG fallback)
- `fallback`: Auto-fall back to PNG if compatibility unconfirmed
- `strict`: Fail if compatibility cannot be confirmed
- `ignore`: Proceed without compatibility checks

## Security

Image path validation prevents path traversal attacks:
- Checks for ".." path components
- Validates resolved absolute path is within `AllowedImagePaths`
- Returns `ErrPathTraversal` or `ErrPathOutsideAllowed` on violation

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Template not found | Error with path |
| Template path is empty | Error: "template path is required" |
| Output path is empty | Error: "output path is required" |
| No slides specified | Error: "at least one slide is required" |
| Output directory not exist | Error with path |
| Invalid template format | Error |
| Layout ID not in template | Error |
| Image file not found | Warning, skip image |
| Image path traversal | Error: ErrPathTraversal |
| Image outside allowed paths | Error: ErrPathOutsideAllowed |
| Chart render failure | Warning, skip chart |
| Output path not writable | Error |
| Disk space insufficient | Error |

## Limitations

1. **No Embedded Charts**: Charts are rendered as SVG/PNG images, not editable Excel-backed charts
2. **No SmartArt**: Not supported
3. **No Editable Tables**: Tables are rendered as OOXML `<a:tbl>` elements but do not link to embedded Excel data

## Testing Requirements

Test cases:
- Simple text-only presentation
- Presentation with bullets
- Body and bullets combined content
- Presentation with images (various formats: PNG, JPG, SVG)
- Large image scaling
- Unicode content
- Empty slides
- Maximum slide count (100 slides)
- Template preservation (compare before/after theme)
- Path traversal attack prevention
- Chart rendering with theme colors
- Concurrent chart rendering performance

Test fixtures:
- Sample images in various sizes
- SVG files for conversion testing
- Templates with different layouts
- Expected output files for comparison

## Content Building

The `BuildContentItems` function converts `SlideDefinition` and layout content mappings into `ContentItem` slices:

```go
func BuildContentItems(slide types.SlideDefinition, mappings []layout.ContentMapping) []ContentItem
```

Supported content fields:
- `title`: Maps to ContentText
- `body`: Maps to ContentText (or combined with bullets)
- `bullets`: Maps to ContentBullets or ContentBodyAndBullets
- `left`/`right`: Maps to ContentBullets (for comparison layouts)
- `image`: Maps to ContentImage
- `chart`: Maps to ContentChart
