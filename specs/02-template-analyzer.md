# PPTX Template Analyzer

Extract layout metadata from PowerPoint templates for informed layout selection.

## Scope

This specification covers ONLY template analysis. It does NOT cover:
- JSON input parsing (see `docs/INPUT_FORMAT.md`)
- Layout selection logic (see `03-layout-selector.md`)
- PPTX generation (see `04-pptx-generator.md`)

## Purpose

PowerPoint templates contain slide layouts with placeholders. This component extracts:
- Available layouts and their characteristics
- Placeholder positions, types, and capacities
- Theme colors and fonts
- Layout suitability hints for different content types
- Optional embedded metadata for enhanced layout hints

## Input

A `.pptx` file path. PPTX files are ZIP archives containing Office Open XML.

Key paths within the archive:
- `ppt/presentation.xml` - Master slide references
- `ppt/slideLayouts/slideLayoutN.xml` - Individual layout definitions
- `ppt/slideMasters/slideMasterN.xml` - Master slide styles
- `ppt/theme/themeN.xml` - Color schemes and fonts
- `ppt/go-slide-creator-metadata.json` - Optional embedded metadata

## Output

```go
type TemplateAnalysis struct {
    TemplatePath string
    Hash         string            // SHA256 of file for cache validation
    AspectRatio  string            // "16:9" or "4:3" (configurable via API param)
    Layouts      []LayoutMetadata
    Theme        ThemeInfo
    AnalyzedAt   time.Time
    Metadata     *TemplateMetadata // Optional embedded metadata (nil if not present)
}

// Analysis options for API param support
type AnalysisOptions struct {
    AspectRatio string // Override auto-detected ratio: "16:9" (default) or "4:3"
}

type LayoutMetadata struct {
    ID           string            // Internal layout ID from XML (e.g., "slideLayout1")
    Name         string            // Human-readable name
    Index        int               // Position in template (zero-based)
    Placeholders []PlaceholderInfo
    Capacity     CapacityEstimate
    Tags         []string          // Classification tags
}

type PlaceholderInfo struct {
    ID       string          // Placeholder ID from cNvPr name attribute
    Type     PlaceholderType // title, body, image, chart, table, content, other
    Index    int             // Placeholder index for population
    Bounds   BoundingBox     // Position in EMUs
    MaxChars int             // Estimated character capacity
}

type PlaceholderType string
const (
    PlaceholderTitle   PlaceholderType = "title"   // Title placeholder
    PlaceholderBody    PlaceholderType = "body"    // Body text placeholder
    PlaceholderImage   PlaceholderType = "image"   // Image placeholder
    PlaceholderChart   PlaceholderType = "chart"   // Chart placeholder
    PlaceholderTable   PlaceholderType = "table"   // Table placeholder
    PlaceholderContent PlaceholderType = "content" // Generic content placeholder
    PlaceholderOther   PlaceholderType = "other"   // Non-content utility placeholders (date, footer, slide number, header)
)

type BoundingBox struct {
    X      int64 // EMUs from left edge
    Y      int64 // EMUs from top edge
    Width  int64 // Width in EMUs
    Height int64 // Height in EMUs
}

type CapacityEstimate struct {
    MaxBullets    int  // Comfortable bullet count
    MaxTextLines  int  // Text lines before overflow
    HasImageSlot  bool // Contains image placeholder
    HasChartSlot  bool // Contains chart placeholder
    TextHeavy     bool // Primarily text-focused
    VisualFocused bool // Primarily visual-focused
}

type ThemeInfo struct {
    Name        string
    Colors      []ThemeColor
    TitleFont   string
    BodyFont    string
}

type ThemeColor struct {
    Name string // accent1, accent2, dk1, lt1, etc.
    RGB  string // Hex color value (e.g., "#FF0000")
}
```

## Template Metadata

Templates may include an optional embedded metadata file at `ppt/go-slide-creator-metadata.json`:

```go
type TemplateMetadata struct {
    Version     string                   // Metadata schema version (e.g., "1.0")
    Name        string                   // Template display name
    Description string                   // Template purpose and style
    Author      string                   // Template creator
    Tags        []string                 // Keywords for categorization
    CreatedAt   *time.Time               // Creation timestamp
    UpdatedAt   *time.Time               // Last modification timestamp
    AspectRatio string                   // Override auto-detected ratio
    LayoutHints map[string]LayoutHint    // Per-layout hints (keyed by ID or name)
}

type LayoutHint struct {
    PreferredFor []string // Content types this layout works best with
    MaxBullets   int      // Override computed bullet capacity
    MaxChars     int      // Override computed character capacity
    Deprecated   bool     // Mark layout as deprecated (skip in auto-selection)
}
```

Metadata hints are merged into layout analysis results via `ApplyMetadataHints()`, overriding computed values where specified.

## Layout Classification

Each layout receives classification tags based on placeholder analysis:

### Usable Body Threshold

Body placeholders are only counted as "usable" if their `MaxChars` meets a minimum threshold (`minUsableBodyChars`). This prevents tiny or decorative placeholders from triggering content-bearing tags like `content` or `two-column`.

### Structural Tags (based on placeholder analysis)

| Tag | Criteria |
|-----|----------|
| `title-slide` | Single visible title + optional subtitle, no body/image/chart |
| `content` | Visible title + body placeholder |
| `title-hidden` | Has title placeholder but it's off-screen (Y < 0); for statement/quote slides |
| `two-column` | At least two body/content placeholders positioned side-by-side |
| `comparison` | Same as two-column (added together) |
| `image-left` | Image placeholder on left, body on right |
| `image-right` | Body on left, image placeholder on right |
| `full-image` | Large image placeholder (>50% slide area), no body |
| `chart-capable` | Contains chart placeholder |
| `blank` | No placeholders |
| `blank-title` | Title placeholder only, no usable body placeholders |

### Semantic Tags (based on layout name keywords)

| Tag | Name Keywords |
|-----|---------------|
| `quote` | "quote", "quotation" |
| `statement` | "statement" |
| `big-number` | "number", "metric", "kpi", "stats", "statistic" |
| `section-header` | "section", "divider", "break", "transition" |
| `agenda` | "agenda", "outline", "contents", "overview" |
| `timeline-capable` | "timeline", "process", "roadmap", "milestone" |
| `icon-grid` | "icon", "grid", "matrix" |
| `closing` | "closing", "close", "final", "conclusion", "end" (word boundary) |

## Character Capacity Estimation

Character capacity is estimated based on placeholder dimensions:

```
MaxChars = (Width / CharWidthEMU) * (Height / LineHeightEMU) * 0.4
```

Where:
- `CharWidthEMU` = 91440 (1 inch / 10 characters at standard font)
- `LineHeightEMU` = 274320 (0.3 inches per line)
- `0.4` = Safety factor for margins, padding, and typical text density

Values are clamped to a maximum of 10000 characters to avoid unrealistic estimates.

## Caching

Template analysis is expensive. Results should be cached:

```go
// TemplateCache provides caching for template analysis results.
type TemplateCache interface {
    Get(path string) (*TemplateAnalysis, bool)
    Set(path string, analysis *TemplateAnalysis)
    Invalidate(path string)
    Clear()    // Clear removes all entries from the cache
    Size() int // Size returns the number of entries in the cache
}

// FastValidationCache is an optional interface for caches that support fast modTime-based validation.
// This avoids expensive hash calculation on every request.
type FastValidationCache interface {
    TemplateCache
    GetWithFastValidation(path string) (*TemplateAnalysis, bool)
    SetWithModTime(path string, analysis *TemplateAnalysis, modTime time.Time)
}
```

### MemoryCache Implementation

The `MemoryCache` implementation provides:
- **TTL expiration**: Default 24 hours (templates rarely change)
- **LRU eviction**: Bounded cache with configurable `maxSize`
- **Fast validation**: Uses file modification time to skip hash recalculation
- **Thread-safe**: All operations are protected by RWMutex
- **FileStat abstraction**: Enables testing without real file system access

Cache key: File path
Cache invalidation: When file modification time or hash changes

## Acceptance Criteria

### AC1: Valid Template Parsing
- Given a valid .pptx template file
- When analyzed
- Then TemplateAnalysis contains layouts with placeholders

### AC2: Layout Enumeration
- Given template with N slide layouts
- When analyzed
- Then Layouts array contains N entries

### AC3: Placeholder Detection
- Given layout with title and body placeholders
- When analyzed
- Then PlaceholderInfo includes both with correct types

### AC4: Placeholder Bounds
- Given placeholder with defined position in XML
- When analyzed
- Then BoundingBox contains EMU coordinates matching XML

### AC5: Theme Extraction
- Given template with custom theme colors
- When analyzed
- Then ThemeInfo.Colors contains accent colors

### AC6: Capacity Estimation
- Given body placeholder of known dimensions
- When analyzed
- Then MaxChars is within 20% of expected value

### AC7: Layout Classification
- Given layout with two side-by-side body placeholders
- When analyzed
- Then Tags includes "two-column"

### AC8: Cache Hit
- Given previously analyzed template (unchanged)
- When analyzed again
- Then returns cached result without re-parsing XML

### AC9: Cache Invalidation
- Given template file modified after caching
- When analyzed
- Then re-parses XML (hash mismatch)

### AC10: Invalid Template
- Given corrupted or non-PPTX file
- When analyzed
- Then returns error with descriptive message

### AC11: Missing Layouts
- Given template with no slide layouts
- When analyzed
- Then returns error "templates must have layouts: no slideLayout files found"

### AC12: Template Metadata
- Given template with embedded metadata file
- When analyzed
- Then TemplateAnalysis.Metadata is populated with parsed values

### AC13: Metadata Validation
- Given template with invalid/malformed metadata
- When validated in soft mode
- Then returns warnings but validation passes

### AC14: Metadata Version Check
- Given template with unsupported metadata version
- When validated
- Then returns MetadataVersionError with version range

## Placeholder Normalization

After parsing layouts, `NormalizePlaceholderNames()` rewrites raw OOXML shape names to canonical IDs. This ensures JSON input uses stable, template-independent placeholder names.

### Canonical Naming Scheme

| Canonical ID | Source | Rule |
|---|---|---|
| `title` | `ph.type="title"` or `"ctrTitle"` | One per layout |
| `subtitle` | `ph.type="subTitle"` | One per layout |
| `body` | `ph.type="body"` or implicit body | First by X position |
| `body_2`, `body_3` | Additional bodies | Ordered left-to-right, top-to-bottom tiebreaker |
| `image`, `image_2` | `ph.type="pic"` | Ordered left-to-right |

Utility placeholders (`dt`, `ftr`, `sldNum`, `hdr`) retain their OOXML names.

### Normalization Result

```go
type NormalizationResult struct {
    Renames    []PlaceholderRename // Shape name renames applied
    TypeFixes  []TypeInjection     // Implicit body placeholders that had type="body" injected
    Warnings   []string            // Structural warnings (e.g., duplicate canonical names)
}
```

### Implicit Body Detection

Placeholders with no explicit `ph.type` attribute but with a valid index (not a utility placeholder) are treated as implicit body placeholders and receive `type="body"` injection.

## Canonical Layout Name Resolution

The layout selector supports human-friendly layout names (e.g., `"content"`, `"two-column"`, `"section"`) via tag-based resolution in `internal/layout/canonical.go`. Each canonical name maps to a set of required/excluded tags and an optional name hint for disambiguation.

| Canonical Name | Required Tags | Notes |
|---|---|---|
| `title` | `title-slide` | Excludes `blank-title`, `closing` |
| `content` | `content` | Excludes `two-column`, `section-header` |
| `section` | `section-header` | |
| `closing` | `closing` | |
| `blank` | `blank-title`, `blank` | |
| `two-column` | `two-column` | Prefers `50` in name |
| `image-left` | `image-left` | |
| `image-right` | `image-right` | |
| `quote` | `quote`, `statement` | |
| `agenda` | `agenda` | |

## Font Resolution

`MasterFontResolver` resolves font properties from slide masters when a layout's placeholder has no explicit font definitions. This ensures accurate `max_chars` estimation and consistent typography.

```go
type MasterFontResolver struct {
    reader *Reader
    cache  map[string]*MasterFontStyles
    theme  *types.ThemeInfo
}

type MasterFontStyles struct {
    TitleStyle *FontStyle
    BodyStyle  map[int]*FontStyle  // By list level (0-8)
    OtherStyle map[int]*FontStyle
}

type FontStyle struct {
    FontFamily string // Resolved font family name
    FontSize   int    // Hundredths of a point (1400 = 14pt)
    FontColor  string // Hex color
}
```

The resolver caches results per master path and resolves theme font references (`+mj-lt`, `+mn-lt`) to actual font family names using `ThemeInfo`.

## Transform Inheritance

When a placeholder in a layout lacks explicit transform (position/size) data:

1. **Inheritance Lookup**: Attempt to inherit transform from the corresponding placeholder in the slide master
2. **Error on Failure**: If transform cannot be resolved from inheritance, return error "placeholder {id} has no transform and cannot inherit from master"

This ensures all placeholders have valid bounds for content placement.

## Layout Synthesis

When a template lacks required layout capabilities (e.g., no two-column layout), the analyzer synthesizes missing layouts from an existing content layout.

### Synthesis Pipeline

After `ParseLayouts()`, `SynthesizeIfNeeded()` runs automatically:

1. **Capability Check**: Scan existing layouts for required capabilities (two-column, three-column, etc.)
2. **Base Layout Selection**: Find the best single-content layout as synthesis base (highest body placeholder area)
3. **Style Extraction**: Extract `PlaceholderStyle` from the base layout (font, margins, bullets, spacing)
4. **Content Area Extraction**: Extract `ContentAreaBounds` from the base layout's body placeholder
5. **Layout Generation**: Call `GenerateHorizontalLayouts()` for only the missing capabilities
6. **Metadata Conversion**: Convert `GeneratedLayout` → `LayoutMetadata` with proper tags
7. **Manifest Storage**: Store generated XML bytes in `SynthesisManifest`

### SynthesisManifest

```go
type SynthesisManifest struct {
    // SyntheticFiles maps layout paths (e.g., "ppt/slideLayouts/slideLayout99.xml")
    // to their generated XML bytes. Also includes .rels files.
    SyntheticFiles map[string][]byte
}
```

Added to `TemplateAnalysis`:
```go
type TemplateAnalysis struct {
    // ... existing fields ...
    Synthesis *SynthesisManifest // nil if no synthesis needed
}
```

### Key Constraints

- **Demand-aware**: Only generate layouts for missing capabilities, not all 18 variations
- **No override**: Templates with native two-column layouts keep using them
- **Styled XML**: Generated layouts include `bodyPr` margins and `lstStyle` with font/bullet attributes from the base layout
- **No Expander.pptx.Package**: Generate layout XML bytes directly; the generator writes them via its streaming ZIP architecture
- **Placeholder indices**: Start at idx=10+ to avoid collision with master placeholders (title=0, body=1)

### Acceptance Criteria

#### AC15: Synthetic Layout Generation
- Given template with only single-content layouts
- When analyzed
- Then SynthesisManifest contains two-column layout(s)

#### AC16: Synthetic Layout Styling
- Given synthetic layout generated from styled base layout
- When generated XML is inspected
- Then bodyPr contains margin attributes from base PlaceholderStyle
- And lstStyle contains font family, size, and bullet properties

#### AC17: Native Layout Preservation
- Given template with native two-column layout
- When analyzed
- Then no two-column synthetic layouts are generated

## Error Handling

| Scenario | Behavior |
|----------|----------|
| File not found | Return error "template file not found: {path}" |
| Path is directory | Return error "template path is a directory, not a file: {path}" |
| Not a ZIP file | Return error "invalid PPTX format (not a ZIP archive): {err}" |
| Missing presentation.xml | Return error "corrupted template: missing ppt/presentation.xml" |
| Missing layouts | Return error "templates must have layouts: no slideLayout files found" |
| Missing theme | Continue with default theme (Calibri fonts, standard colors) |
| Unparseable layout XML | Skip layout, continue with remaining layouts |
| No parseable layouts | Return error "failed to parse any layouts from template" |
| Missing placeholder transform | Attempt master inheritance; error if not found |
| Missing metadata | Return nil metadata (OK - metadata is optional) |
| Malformed metadata JSON | Return MetadataParseError |
| Unsupported metadata version | Return MetadataVersionError |

## Testing Requirements

Test fixtures:
- `standard.pptx` - Normal template with 5+ layouts
- `minimal.pptx` - Single layout template
- `custom_theme.pptx` - Non-default colors/fonts
- `wide.pptx` - 16:9 aspect ratio
- `standard_43.pptx` - 4:3 aspect ratio
- `corrupted.pptx` - Invalid ZIP structure
- `no_layouts.pptx` - Template with layouts removed
