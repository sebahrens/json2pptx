# Template Specification

This document defines what a json2pptx-compatible PPTX template must contain. Templates that conform to this spec work out of the box with the generator, the layout selector, and the MCP agent. Templates that violate mandatory rules will fail `json2pptx template-check`.

## Mandatory Layouts

Every template must contain **all** of the following layouts. Layout names are matched case-insensitively; common synonyms are accepted (noted in parentheses).

| # | Layout Role | Accepted Names | Required Placeholders | Classification Tags |
|---|-------------|---------------|----------------------|-------------------|
| 1 | **Title Slide** | `Title Slide` | `title` (type `ctrTitle` or `title`) + `subtitle` | `title-slide` |
| 2 | **One Content** | `One Content`, `Content` | `title` + `body` (idx 1) | `content` |
| 3 | **Two Content** | `Two Content`, `Comparison` | `title` + `body` (idx 1) + `body_2` (idx 2), side-by-side | `content`, `two-column` |
| 4 | **Section Divider** | `Section Divider`, `Section Header` | `title` + body placeholder named `Section Number` (idx varies) | `section-header` |
| 5 | **Blank** | `Blank` | _(none)_ | `blank` |
| 6 | **Blank + Title** | `Blank + Title`, `Blank Layout` | `title` only (no body, no subtitle) | `blank-title` |
| 7 | **Closing** | `Closing`, `Thank You`, `End Slide` | `title` (type `ctrTitle` or `title`) + `subtitle` | `closing` |

**Notes:**
- If a mandatory layout is missing, `template-check` reports an error and exits non-zero.
- The engine can synthesize `Two Content` and `Blank + Title` from other layouts when missing, but native layouts are always preferred. Synthesis triggers a warning.
- Layout order within the PPTX file does not matter — layouts are matched by name and tag.

## Optional Layouts

These layouts are recognized and utilized when present but are not required:

| Layout Role | Accepted Names | Tags |
|-------------|---------------|------|
| Agenda | `Agenda`, `Table of Contents` | `agenda` |
| Quote | `Quote`, `Quotation` | `quote` |
| Statement | `Statement` | `statement` |
| Image Left | `Picture with Caption` (image on left) | `image-left` |
| Image Right | `Picture with Caption` (image on right) | `image-right` |

## Placeholder Naming Convention

Placeholders are identified by the `cNvPr` name attribute in the slide layout XML. The generator resolves placeholders by canonical name (via the normalizer), so templates must use recognizable names.

### Canonical Placeholder Names

| Canonical Name | XML Placeholder Type | Purpose |
|---------------|---------------------|---------|
| `title` | `type="title"` or `type="ctrTitle"` | Slide title |
| `subtitle` | `type="subTitle"` | Subtitle (title and closing slides) |
| `body` | `type="body"` (idx 1) | Primary content area |
| `body_2` | `type="body"` (idx 2) | Second column (two-column layouts) |
| `Section Number` | `type="body"` (by name) | Section number display on divider layouts |
| `dt` | `type="dt"` | Date field (utility, not content) |
| `ftr` | `type="ftr"` | Footer field (utility, not content) |
| `sldNum` | `type="sldNum"` | Slide number field (utility, not content) |

### Section Number Placeholder

The `Section Number` placeholder on section divider layouts has specific requirements:

- **Name**: `cNvPr` name must be `"Section Number"` (case-sensitive in the XML, matched case-insensitively by the resolver)
- **Position**: Upper-right quadrant of the slide
- **Minimum width**: 2,743,200 EMU (3 inches) — must fit two-digit numbers
- **Default font size**: ≥ 13,600 half-points (≥ 100pt) — large display numeral
- **Alignment**: Right-aligned
- **Color**: Should use an accent color from the theme (typically `accent1`)

## Typography Constraints

### Body Text
- Default font size for body placeholders: **18–24pt** (2,400–3,200 half-points)
- The generator's text fitting system (overflow levels P0–P5) assumes this range. Templates with body fonts outside this range may produce unexpected fit behavior.

### Title Text
- Default font size for title placeholders: **28–44pt** (3,600–5,600 half-points)
- No strict enforcement — varies by layout role.

### Fonts
- Templates should embed or reference fonts available on the target rendering system
- The theme must define both `majorFont` (titles) and `minorFont` (body text)

## Theme Requirements

Every template's theme (`ppt/theme/theme1.xml`) must define:

| Element | Description |
|---------|-------------|
| **12 scheme colors** | `dk1`, `dk2`, `lt1`, `lt2`, `accent1`–`accent6`, `hlink`, `folHlink` |
| **Major font** | Used for titles (`a:majorFont`) |
| **Minor font** | Used for body text (`a:minorFont`) |

### Color Contrast

- `dk1` and `dk2` must be dark colors (luminance < 50%)
- `lt1` and `lt2` must be light colors (luminance > 50%)
- The generator enforces WCAG AA contrast between text and backgrounds; templates with poor scheme-color contrast will trigger automatic adjustments.

## Aspect Ratio

- Default: **16:9** (9,144,000 × 6,858,000 EMU)
- Also supported: **4:3** (9,144,000 × 6,858,000 EMU adjusted)
- Declare non-standard ratios in the metadata file (see below)

## Metadata (Optional)

Templates may include an embedded metadata file at `ppt/go-slide-creator-metadata.json`:

```json
{
  "version": "1.0",
  "name": "Template Name",
  "description": "Brief description",
  "surface_tints": {
    "subtle": "lt2",
    "paper": "lt1",
    "elevated": "lt2",
    "inverse": "dk2"
  },
  "data_palette": ["accent1", "accent2", "accent3", "accent4", "accent6", "accent5"],
  "semantic_accents": {
    "positive": "accent4",
    "negative": "accent2",
    "neutral": "accent5"
  }
}
```

Metadata is optional. When absent, the engine infers properties from the theme and layout structure.

### SurfaceTints

Maps surface roles to scheme color names. Patterns call `ResolveSurface(role)` to select tinted background fills that harmonize with the template. All four roles should be defined:

| Role       | Purpose                                                | Recommended Values |
|------------|--------------------------------------------------------|--------------------|
| `subtle`   | Lightest tint — alternate rows, card backgrounds       | `"lt2"` or a light accent |
| `paper`    | Card/panel surface — slightly off-white                | `"lt1"` |
| `elevated` | Raised surface — shadows or slight contrast step       | `"lt2"` or a muted accent |
| `inverse`  | Dark surface — high-contrast sections, headers         | `"dk2"` |

Values must be valid scheme color names (`dk1`, `dk2`, `lt1`, `lt2`, `accent1`–`accent6`). The engine resolves them through the template's theme at generation time.

When `surface_tints` is absent from metadata, patterns fall back to hardcoded defaults (`"lt1"` / `"lt2"`).

### DataPalette

An ordered list of scheme color names controlling chart series coloring. `svggen` uses this to ensure chart colors match the template's visual identity rather than using a fixed `accent1`–`accent6` ordering.

```json
"data_palette": ["accent1", "accent2", "accent5", "accent3", "accent6", "accent4"]
```

The list should contain 6 entries (one per accent slot). The ordering determines which accent is used for the first, second, third (etc.) chart series. Templates can reorder to put their most visually distinct accents first.

When `data_palette` is absent, `svggen` falls back to the fixed order `accent1`–`accent6`.

### Template Conformance Check

Run `json2pptx template-check` to verify metadata completeness:

```bash
json2pptx template-check templates/midnight-blue.pptx
```

The checker validates that `surface_tints` defines all four roles and `data_palette` contains valid scheme color names.

## Conformance Checking

Run the conformance checker against any template:

```bash
json2pptx template-check <template.pptx>
json2pptx template-check --json <template.pptx>   # machine-readable output
```

The checker verifies:
1. All 7 mandatory layouts are present (by name, tag, **or canonical-role classification**)
2. Each mandatory layout has its required placeholders
3. Section Number placeholder meets size/position requirements
4. Theme defines all 12 scheme colors
5. Theme defines major and minor fonts
6. Dark/light color luminance polarity is correct
7. **Layout names match canonical roles** — emits WARN when a layout is structurally a canonical role (Title Slide, One Content, Two Content, Section Divider, Blank, Blank + Title, Closing) but uses a non-canonical name (e.g. "Cover Slide" → "Title Slide"). The repair pipeline must rename in place; do not author a new duplicate layout.
8. **No duplicate layout signatures** — emits WARN when two or more layouts map to the **same canonical role** AND share the **same structural signature**. Layouts that share only a signature but have different canonical roles (e.g. a Closing layout and a Title Slide both with `subtitle+title`) are not flagged.

Exit codes:
- **0**: All checks pass (WARN findings do not fail the check)
- **1**: One or more mandatory checks failed

### Known Exceptions

The remaining designer templates (`business-template`, `modern-yellow`) still fail `template-check` while their per-template repairs land under the [`go-slide-creator-vqad`](https://github.com/) umbrella. Their SHA-256 fingerprints are listed in `internal/template/testdata/conformance_allowlist.json` and `canonical_placeholders_allowlist.json`; the allow-list entries auto-expire when the fingerprints change after repair.

`modern-template.pptx` previously lacked `Two Content`, `Blank`, and `Blank + Title`. Those layouts were authored into the template directly via OOXML edits (preserving all embedded media byte-for-byte) and `modern-template` is no longer allow-listed.

`abstract.pptx` previously lacked a subtitle on Title Slide, was missing `Two Content`, `Blank`, and `Blank + Title`, and its Section Divider had no `Section Number` placeholder. The Title Slide was edited to add a subtitle; `Section Break 1` was renamed to `Section Divider` and a `Section Number` placeholder was added; `Introduction` was renamed to `One Content`; new `Two Content`, `Blank`, and `Blank + Title` layouts were authored. All embedded SVG decorations preserved byte-for-byte; `abstract` is no longer allow-listed.

`blue-corporate.pptx` previously had no native Title Slide, Section Divider lacked a title placeholder, and `Blank` / `Blank + Title` were missing. Pre-flight classification (`internal/template/ClassifyCanonicalRole`) found two One Content duplicates (`Title`, `Title and content 01`, both signature `body+title` @ 0.85) and a structurally-section-divider layout (`Section break with image 01`, signature `body*2[side-by-side]` @ 0.90). The repair: `Title` was repurposed into a `Title Slide` (its huge 80pt body became `ctrTitle`, the small uppercase header became `subTitle` idx=1), `Title and content 01` was renamed to `One Content` (canonical-name fix), `Section break with image 01` was renamed to `Section Divider` and its first body placeholder was retyped to `title` while the second was renamed `Section Number` (idx=11, 208pt accent body — already structurally a section number per the existing `##` prompt, 3.94" wide ≥ 3"). New `Two Content` (slideLayout5), `Blank` (slideLayout6), and `Blank + Title` (slideLayout7) layouts were authored, registered in `slideMaster1.xml.rels` (rId6–rId8) + `sldLayoutIdLst` and `[Content_Types].xml`. All 4 embedded SVG decorations preserved byte-for-byte; `blue-corporate` is no longer allow-listed.

`modern.pptx` previously lacked `Two Content`, `Section Divider`, `Blank`, `Blank + Title`, and `Closing`. Pre-flight classification (`internal/template/ClassifyCanonicalRole`) identified two layouts that already structurally matched canonical roles — `Title + subtitle` (signature `subtitle+title` @ 0.95 → Title Slide) and `Content 1` (signature `body+title` @ 0.85 → One Content) — and two more that could be repurposed in place: `Table` (signature `title` @ 0.85 → Title Slide ambiguity, no usable body, fitting candidate for Section Divider repurpose) and `Content 3` (signature `body+image+title` @ 0.85 → One Content duplicate, dark photographic backdrop fitting a Closing visual identity). Repair: `Title + subtitle` was renamed to `Title Slide`; `Content 1` was renamed to `One Content` (the implicit-body placeholder retyped explicitly to `body` idx=1); `Table` was repurposed as `Section Divider` (tbl/dt/ftr/sldNum placeholders dropped, a `Section Number` body placeholder added in the upper-right at 3.7" wide × 136pt accent1 right-aligned with `##` prompt, and the title repositioned to the left of the section number); `Content 3` was repurposed as `Closing` (body idx=10 retyped to `subTitle` idx=1; slide4 — which uses this layout for its "THANK YOU" content — was updated in lockstep to keep its placeholder type aligned). New layouts authored: `Two Content` (slideLayout7, title + body idx=1 + body_2 idx=2 side-by-side), `Blank` (slideLayout8, empty `spTree`), `Blank + Title` (slideLayout9, title only). New layouts registered in `slideMaster1.xml.rels` (rId8/rId9/rId10), `sldLayoutIdLst` (ids 2147483700/701/702), and `[Content_Types].xml`. All 4 embedded media assets (hdphoto1.wdp, image1.png, image2.png, image3.png) preserved byte-for-byte (SHA-256 manifest verified pre/post repair). `modern` is no longer allow-listed.

### Programmability

Templates fall into two categories:

- **Programmable** (regenerable from `cmd/mktemplate`): `forest-green`, `midnight-blue`, `warm-coral`.
- **Designer-owned** (must be repaired in place — `mktemplate` cannot reproduce embedded decorative assets, custom layout shapes, or intentional theme polarities): `abstract`, `blue-corporate`, `business-template`, `modern`, `modern-template`, `modern-yellow`.

See [TEMPLATE_ANALYSIS.md](TEMPLATE_ANALYSIS.md) for the full per-template matrix, current conformance status, and the in-place repair workflow for designer templates.

## Creating a New Template

1. Start from an existing conformant template (e.g., `midnight-blue.pptx`)
2. Modify theme colors, fonts, and layout styling in PowerPoint
3. Ensure all 7 mandatory layouts are present with correct placeholder names
4. Run `json2pptx template-check <your-template.pptx>` to verify
5. Optionally add metadata at `ppt/go-slide-creator-metadata.json`
6. Test with `json2pptx generate -json examples/basic-deck.json -template <name>`
