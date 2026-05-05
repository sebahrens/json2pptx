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
  "schema_version": "1.0",
  "name": "Template Name",
  "description": "Brief description",
  "author": "Author Name",
  "tags": ["professional", "dark"],
  "aspect_ratio": "16:9",
  "layout_hints": {},
  "semantic_accents": {}
}
```

Metadata is optional. When absent, the engine infers properties from the theme and layout structure.

## Conformance Checking

Run the conformance checker against any template:

```bash
json2pptx template-check <template.pptx>
json2pptx template-check --json <template.pptx>   # machine-readable output
```

The checker verifies:
1. All 7 mandatory layouts are present (by name or tag)
2. Each mandatory layout has its required placeholders
3. Section Number placeholder meets size/position requirements
4. Theme defines all 12 scheme colors
5. Theme defines major and minor fonts
6. Dark/light color luminance polarity is correct

Exit codes:
- **0**: All checks pass
- **1**: One or more mandatory checks failed

### Known Exceptions

**`modern-template.pptx`** natively lacks `Two Content`, `Blank`, and `Blank + Title` layouts. The engine synthesizes these at runtime, so the template works correctly for generation. However, `template-check` reports failures because it validates native layouts only. This is an accepted exception — new templates should include all 7 layouts natively.

## Creating a New Template

1. Start from an existing conformant template (e.g., `midnight-blue.pptx`)
2. Modify theme colors, fonts, and layout styling in PowerPoint
3. Ensure all 7 mandatory layouts are present with correct placeholder names
4. Run `json2pptx template-check <your-template.pptx>` to verify
5. Optionally add metadata at `ppt/go-slide-creator-metadata.json`
6. Test with `json2pptx generate -json examples/basic-deck.json -template <name>`
