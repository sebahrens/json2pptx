# Template Specification

Template analysis resolves each selected layout through its own
`slideLayout -> slideMaster -> theme` relationship chain. Layout order and file
names are not role bindings. The reusable profile is keyed by the template's
content SHA-256 plus the profile parser version and records effective slide
dimensions, canonical role bindings, inherited placeholder geometry and text
styles, master/theme paths, and ambiguity or unusable-geometry diagnostics.

Dimensions and placeholder coordinates are stored in EMUs (914,400 EMUs per
inch). Font sizes are stored in hundredths of a point. Consumers must not mix
these units or infer a 16:9 canvas when the package declares 4:3 or another
aspect ratio.

This document defines what a json2pptx-compatible PPTX template must contain. Templates that conform to this spec work out of the box with the generator, the layout selector, and the MCP agent. Templates that violate mandatory rules will fail `json2pptx template-check`.

> **Canonical doc — template *authoring & validation*.** This is the
> authoritative reference for *building or validating* a template (mandatory
> layouts, placeholder naming, theme requirements, conformance checks). Other
> docs should link here rather than restate these rules. For *using* a template
> from JSON (placeholder IDs, layout tags, layout→slide-type mapping), see the
> companion canonical doc
> [skills/template-deck/TEMPLATE_GUIDE.md](../skills/template-deck/TEMPLATE_GUIDE.md).

## Mandatory Layouts

Every template must contain **all** of the following layouts. Layout names are matched case-insensitively; common synonyms are accepted (noted in parentheses).

| # | Layout Role | Accepted Names | Required Placeholders | Classification Tags |
|---|-------------|---------------|----------------------|-------------------|
| 1 | **Title Slide** | `Title Slide` | `title` (type `ctrTitle` or `title`) + `subtitle` | `title-slide` |
| 2 | **One Content** | `One Content`, `Content` | `title` + `body` (idx 1) | `content` |
| 3 | **Two Content** | `Two Content`, `Comparison` | `title` + `body` (idx 1) + `body_2` (idx 2), side-by-side | `content`, `two-column`, `comparison` |
| 4 | **Section Divider** | `Section Divider`, `Section Header` | `title` + body placeholder named `Section Number` (idx varies) | `section-header` |
| 5 | **Blank** | `Blank` | _(none)_ | `blank` |
| 6 | **Blank + Title** | `Blank + Title`, `Blank Layout` | `title` only (no body, no subtitle) | `blank-title` (plus `title-slide`) |
| 7 | **Closing** | `Closing`, `Thank You`, `End Slide` | `title` (type `ctrTitle` or `title`) + `subtitle` | `closing` |

**Notes:**
- If a mandatory layout is missing, `template-check` reports an error and exits non-zero.
- **Title Slide** and **One Content** must also resolve through the generator's `layout_id` aliases `title` and `content`: the layout needs the `title-slide` tag without `closing` / `blank-title`, or the `content` tag without `two-column` / `section-header`. A template whose only `content`-tagged layout is Two Content, or whose only `title-slide`-tagged layout is Closing, fails `template-check` even though a layout "shares" the tag. A One Content layout whose first-level body font is so large that the body holds fewer than 100 characters loses its `content` tag; the FAIL detail names the body size and capacity.
- The engine can synthesize `Two Content` and `Blank + Title` from other layouts when missing, but native layouts are always preferred. Synthesis logs a `WARN` ("template missing layout capabilities, synthesizing").
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

**Other structural tags.** The classifier also emits tags that no mandatory
layout requires but that steer layout selection when a template has them:
`comparison` (always paired with `two-column` — every Two Content layout
carries it), `title-at-bottom` (visible title in the lower half, body present),
`compact-title` (a `title-at-bottom` title slot that fits fewer than 35
characters), `title-hidden` (off-slide title, body present), `full-image`
(large picture with no text beside it) and `chart-capable` (chart
placeholder). The rule for each tag is in the
[TEMPLATE_GUIDE structural-tag table](../skills/template-deck/TEMPLATE_GUIDE.md#structural-tags-from-placeholder-analysis).

## Placeholder Naming Convention

Placeholders are identified by the `cNvPr` name attribute in the slide layout XML. The generator resolves placeholders by canonical name (via the normalizer), so templates must use recognizable names.

### Canonical Placeholder Names

| Canonical Name | XML Placeholder Type | Purpose |
|---------------|---------------------|---------|
| `title` | `type="title"` or `type="ctrTitle"` | Slide title |
| `subtitle` | `type="subTitle"` | Subtitle (title and closing slides) |
| `legal_disclosure` | `subTitle`, `body`, `obj`, or implicit text placeholder | Explicit legal/disclosure text, separate from ordinary subtitle/body capacity |
| `body` | `type="body"` (idx 1) | Primary content area |
| `body_2` | `type="body"` (idx 2) | Second column (two-column layouts) |
| `Section Number` | `type="body"` (by name) | Section number display on divider layouts |
| `dt` | `type="dt"` | Date field (utility, not content) |
| `ftr` | `type="ftr"` | Footer field (utility, not content) |
| `sldNum` | `type="sldNum"` | Slide number field (utility, not content) |

The footer line (`footer.left_text` and the `chrome` fields) is drawn in one
box that starts at the `dt` placeholder's left edge and runs across `ftr`,
stopping short of `sldNum`; the page number takes the `sldNum` slot. A `dt`
parked wholly outside the slide (the usual way to hide the date) is not an
anchor: the line then starts at the visible `ftr` placeholder and takes its
width, as it does when the master has no `dt`. Size `ftr` for the line you
expect: `CHROME_TRUNCATED` reports how many characters the slot holds.

### Section Number Placeholder

An explicitly named `legal_disclosure` (or `Legal Disclosure`, matched
case-insensitively with surrounding whitespace ignored) retains its native
OOXML type, font, alignment and geometry. It does not satisfy a mandatory ordinary
subtitle/body requirement. Supply a separate ordinary subtitle on Title Slide
and Closing layouts. Small fonts or incidental names such as `legal notes`
do not imply this role. Inspection reports its role as `disclosure` and
reserves its text band from the content zone.

The `Section Number` placeholder on section divider layouts has specific requirements:

- **Name**: `cNvPr` name must be `"Section Number"` (case-sensitive in the XML, matched case-insensitively by the resolver). A section divider without it fails `template-check` (and `GATE.SECTION_NUMBER_MISSING` in the CI gate).
- **Position**: Upper-right quadrant of the slide is recommended; it is a design guideline and not checked (bundled `abstract` places its number lower right by design). Alignment and color below are likewise recommendations.
- **Minimum width**: 2,743,200 EMU (3 inches) — must fit two-digit numbers (`template-check` warns when narrower)
- **Font size**: Large display type appropriate to the template. Sizes are hundredths of a point: 9,600 = 96pt; 13,600 = 136pt. The structural role heuristic uses 9,000 (90pt), not a mandatory conformance minimum. Preserve existing template typography; `template-check` does not enforce a section-number font-size threshold.
- **Alignment**: Right-aligned
- **Color**: Should use an accent color from the theme (typically `accent1`)

**Auto-numbering.** The engine assigns a running section number (`01`, `02`, …) to
each section-divider slide and injects it automatically. It routes the number to
the layout's section-number frame whenever one is present — detected via the
canonical `section_number` placeholder role, a `section_number` / `section_no` /
`large_number` alias ID, or the conventional `Section Number` name — so the digit
lands in the large decorative numeral instead of any small body slot. If the
layout has no section-number frame, the number falls back to the `body`
placeholder (preserving behavior for templates that display the number inline).
Auto-injection is skipped when the section slide already carries body, bullets,
table, media, or slot content; authored body text uses the tagline slot normally.

**Optional tagline body.** A section divider may carry an extra `body`
placeholder alongside `Section Number` (for a section tagline or sub-label).
Because auto-numbering targets the `Section Number` frame, the `body` slot is
never overwritten by the running number and is free for optional tagline text;
it is treated as ordinary body text (caps/autofit), not boosted to section-title
size. The tagline slot must not overlap the title or Section Number frame.
`modern-template` keeps its tagline body below the title; templates need not
drop such a slot to be conformant. A divider with a Section Number frame is
never tagged `content`, even if its tagline body is large enough to hold text.

## Typography Constraints

### Body Text
- Default font size for body placeholders: **18–24pt** (1,800–2,400 hundredths of a point)
- The generator's text fitting system (overflow levels P0–P5) assumes this range. Templates with body fonts outside this range may produce unexpected fit behavior.

### Title Text
- Default font size for title placeholders: **28–44pt** (2,800–4,400 hundredths of a point)
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

- Common **16:9** canvas: 12,192,000 × 6,858,000 EMU
- Also supported: **4:3** (9,144,000 × 6,858,000 EMU)
- The actual canvas comes from `p:sldSz` in `ppt/presentation.xml`, including non-standard ratios. Do not infer it from a layout name or the optional metadata file.

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
  },
  "accent_usage_guide": {
    "accent1": "#2E5090 primary emphasis: headers, bands, key figures; carries white text (7.9:1)",
    "accent4": "#FFC000 near-background tint: subtle surfaces only, never text, thin rules or chart series"
  },
  "grid": {
    "margin_pct": 6,
    "columns": 12,
    "gutter_pt": 10,
    "title_gap_pt": 18
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

When `data_palette` is absent, charts start from the fixed order `accent1`–`accent6`. At render time, automatic chart colors with less than 2:1 contrast on the effective chart background are skipped; visible theme colors and dark/light theme slots fill the six series positions. When a chart uses fewer series than the palette holds, a palette color within ΔE 25 (CIE76) of one already assigned is moved later so neighbouring series stay distinguishable on single-hue templates (theme `dk2` / `dk1`, a shade of the first color, then grey fill in when the palette runs out of distinct colors); a chart that needs every palette slot keeps the declared order. Explicit author-supplied chart colors are preserved. `list_templates` keeps the authored `data_palette` ordering and reports near-background accents separately in `color_roles.near_background_accents`.

### AccentUsageGuide

One line per accent (`accent1`–`accent6`) stating its intended role and what text it can carry, surfaced by `list_templates` (`fields="full"`, CLI `skill-info --mode compact`) so agents pick accents with intent. Every bundled template authors all six lines (a corpus test enforces it). When a template has no guide — a bring-your-own template such as one without a metadata part — `list_templates` derives one from the theme's measured contrast (primary / secondary fill, white-text safety, near-background tints) and sets `accent_usage_guide_derived: true`; treat a derived guide as contrast facts only, not brand intent.

`color_roles.primary_fill` / `secondary_fill` name the first accents that carry white body text (4.5:1), then large text (3:1), then a qualifying `dk2` / `dk1`; they are not always `accent1`, and patterns use the same slot as their default accent (`ExpandContext.DefaultAccent`). `color_roles.ink_on_accent` gives, per accent, the readable text colour for a fill of that accent (`lt1` when white passes 4.5:1, else `dk2` / `dk1`) and its ratio.

### Grid

The template's grid system (go-slide-creator-5ms8c). Every key is optional; an omitted key takes the engine default, so a template without a `grid` block renders exactly as before. `ParseLayouts` attaches the grid to every parsed layout, so generation, `validate` / fit-report, previews and `score_deck` all apply the same values.

| Key | Range | Default | What the engine does with it |
|-----|-------|---------|------------------------------|
| `margin_pct` | 0–25 | the reference one-content layout's title left edge | Left / right content margin as a percentage of the slide width. Pattern and `shape_grid` zones, the chrome content column (content placeholders, placeholder charts / diagrams / tables, the takeaway and source bands) and the content-size estimate patterns use when the caller passes no layout bounds are narrowed to it (never widened over template artwork); the content frame's left edge follows it. |
| `columns` | 0–24 | 12 | Advisory column count for agents sizing their own `shape_grid` columns; reported, not enforced. |
| `gutter_pt` | 0–72 | 8 | Default column / row gap of any raw `shape_grid` and of a `compose` envelope that sets no `gap` / `col_gap` / `row_gap` of its own. Pattern-internal gaps scale with it — see below. |
| `title_gap_pt` | 0–144 | 18 | Gap between the bottom of a measured, top-anchored title's text and the start of the body zone (`reserveMeasuredTitle`); also the content top of a template whose reference layout has no body placeholder. A value above 18 also shortens the content-size estimate patterns use without layout bounds. |

Out-of-range values are reported by `validate-template` as `TEMPLATE_GRID_INVALID` and ignored (the default applies). `list_templates` (`fields="full"`, CLI `skill-info`) and `examine-template` (report.json `grid`, report.md "Grid" section, and the CLI summary) print the effective values, `declared` (the keys the template authors) and the `content_frame`: the reference one-content layout's title bottom, body top (the body line) and left / right edges in points — the lines `grid_violation` checks rendered content against (see [FIT_FINDINGS.md](FIT_FINDINGS.md#grid_violation)).

**Pattern-internal gaps.** Every named pattern authors its gaps (card gutters, column / row gaps, gaps its content sizing subtracts) against the engine's 8pt gutter. A declared `gutter_pt` scales all of them by `gutter_pt / 8` through `ExpandContext.Gap` (`internal/patterns/grid_gap.go`), in the emitted `gap` / `col_gap` / `row_gap` and in the measurement alike, so a 12pt gutter opens a kpi-Nup's 12pt card gap to 18pt and bmc-canvas's 4pt cell gap to 6pt, and text is sized against the gaps it will render with. Gaps that are part of a pattern's drawing stay fixed: hairline / sentinel gaps of 1pt or less, matrix-2x2 axis arrows, state-shift-hub's hub ring, the kpi icon-to-value offset, comparison-2col's connector channel, swimlane's connector channels (sized against its step budget) and the takeaway band's air (chrome, shared with `ResolveChromeFrame`). Pattern authors: wrap any new gap constant as `ctx.Gap(x)` wherever it is emitted or measured.

**Chart frames.** A chart or diagram placed in a grid cell (raw `shape_grid`, `chart-insights-split`, compose segments) is separated from its neighbours by the scaled gutter. A placeholder chart / diagram is clamped to the chrome content column, which a declared `margin_pct` narrows, even when its placeholder transform is inherited from the layout.

Without grid metadata — every bundled template today — the scale is exactly 1 and the frames are unchanged, so output is byte-identical to an engine without the keys.

### Template Conformance Check

Run `json2pptx template-check` to verify metadata completeness:

```bash
json2pptx template-check templates/midnight-blue.pptx
```

The checker FAILs when a `surface_tints` value is neither a scheme color name nor 6-digit hex, or a `data_palette` entry is not a theme slot name (`dk1`, `dk2`, `lt1`, `lt2`, `accent1`–`accent6`, `hlink`, `folHlink`; hex is not resolved for charts). It WARNs when `surface_tints` is present but omits one of the four roles. At generation time an invalid surface tint falls back to the pattern's default instead of being written into slide XML.

On One Content and Two Content layouts, the visible title font must be larger than the first-level body font. `template-check` reports a typography warning when the body is the same size or larger; layouts with unresolved font sizes are not judged by this check.

## Conformance Checking

Run the conformance checker against any template:

```bash
json2pptx template-check <template.pptx>
json2pptx template-check --json <template.pptx>   # machine-readable output
```

The checker verifies:
1. All 7 mandatory layouts are present (by name, tag, **or canonical-role classification**); Title Slide and One Content must additionally resolve as `layout_id` `title` / `content` (see Mandatory Layouts notes)
2. Each mandatory layout has its required placeholders
3. Section divider has a `Section Number` placeholder (FAIL when missing; WARN when narrower than 3 inches)
4. Theme part is present, parses, and every slide master's theme relationship resolves (a dangling theme rel FAILs)
5. Theme defines all 12 scheme colors, each a valid 6-digit hex value (`GGHHII` FAILs instead of being dropped silently)
6. Theme defines major and minor fonts — checked on the raw `a:fontScheme`, so a missing scheme or empty `typeface` FAILs rather than defaulting to Calibri
7. Dark/light color luminance polarity is correct
8. **Slide size** — `p:sldSz` is present and `cx` / `cy` are within the OOXML range 914,400–51,206,400 EMU (FAIL); layout placeholders extending more than 1% past the right or bottom slide edge WARN (placeholders parked at negative coordinates are ignored)
9. **Metadata colors** — `surface_tints` / `data_palette` values are valid (see Template Conformance Check above)
10. **Layout names match canonical roles** — emits WARN when a layout is structurally a canonical role (Title Slide, One Content, Two Content, Section Divider, Blank, Blank + Title, Closing) but uses a non-canonical name (e.g. "Cover Slide" → "Title Slide"). The repair pipeline must rename in place; do not author a new duplicate layout.
11. **No duplicate layout signatures** — emits WARN when two or more layouts map to the **same canonical role** AND share the **same structural signature**. Layouts that share only a signature but have different canonical roles (e.g. a Closing layout and a Title Slide both with `subtitle+title`) are not flagged.
12. **First accent visible on the light canvas** — emits WARN when `accent1` has less than 2:1 contrast against `lt1`; such a color can still be intentional on a dark surface, but should not lead charts on the light canvas.
13. **Footer chrome complete or absent** — the resolved `dt`, `ftr`, and `sldNum` slots must appear together, or not at all.
14. **No conflicting structural tags** — a layout must not be classified as both `content` and `section-header`.
15. **Text slots do not materially overlap** — title, subtitle, and body/content placeholders must not intersect by at least 1 mm in both dimensions and 2% of the smaller slot. Decorative section-number frames and picture/text overlays are excluded.
16. **Image does not cover title** — an image placeholder that overlaps a title must precede it in the layout shape tree. The generator also protects emitted slide order, but source templates should be safe independently.
17. **Content title/body hierarchy** — a visible title on One Content or Two Content must be larger than first-level body text when both sizes resolve. Layouts named as One / Two Content are judged even when an oversized body font stops the classifier from recognising the role.
18. **Content title geometry consistent** — the title placeholder of every One Content, Two Content and Blank + Title layout must match the first One Content layout's title in x, y, width, height (±0.1in) and vertical anchor (WARN). A deck mixes bullet slides, side-by-side slides and pattern canvases; differing title boxes make the headline jump from slide to slide.

Exit codes:
- **0**: All checks pass (WARN findings do not fail the check)
- **1**: One or more mandatory checks failed

### Known Exceptions

Bundled designer templates pass `template-check` with zero FAIL and zero WARN findings, except `modern-template`'s check-18 WARN (allow-listed, see below). `abstract` accent1 was deepened from `#E9E6DF` to warm neutral `#8E8172` so it is visible on the white canvas. `blue-corporate` now has a native date placeholder alongside its footer and page-number placeholders. Check 18 (go-slide-creator-kyk01) aligned the content-family titles: `abstract` One Content adopted the Two Content / Blank + Title title box (and the body moved to the same top and full width); `business-template` One Content and Blank + Title adopted Two Content's title box (One Content's subtitle now sits between title and body); `blue-corporate` Blank + Title adopted its One Content title box. `modern-template` is allow-listed for this check only (tracking go-slide-creator-kyk01): its reviewed 1.68in bottom-anchored content title cannot move without breaking reviewed typography, and moving Blank + Title down would shrink every pattern canvas.

`modern-template.pptx` previously lacked `Two Content`, `Blank`, and `Blank + Title`. Those layouts were authored into the template directly via OOXML edits (preserving all embedded media byte-for-byte) and `modern-template` is no longer allow-listed.

`abstract.pptx` previously lacked a subtitle on Title Slide, was missing `Two Content`, `Blank`, and `Blank + Title`, and its Section Divider had no `Section Number` placeholder. The Title Slide was edited to add a subtitle; `Section Break 1` was renamed to `Section Divider` and a `Section Number` placeholder was added; `Introduction` was renamed to `One Content`; new `Two Content`, `Blank`, and `Blank + Title` layouts were authored. All embedded SVG decorations preserved byte-for-byte; `abstract` is no longer allow-listed.

`blue-corporate.pptx` previously had no native Title Slide, Section Divider lacked a title placeholder, and `Blank` / `Blank + Title` were missing. Pre-flight classification (`internal/template/ClassifyCanonicalRole`) found two One Content duplicates (`Title`, `Title and content 01`, both signature `body+title` @ 0.85) and a structurally-section-divider layout (`Section break with image 01`, signature `body*2[side-by-side]` @ 0.90). The repair: `Title` was repurposed into a `Title Slide` (its huge 80pt body became `ctrTitle`, the small uppercase header became `subTitle` idx=1), `Title and content 01` was renamed to `One Content` (canonical-name fix), `Section break with image 01` was renamed to `Section Divider` and its first body placeholder was retyped to `title` while the second was renamed `Section Number` (idx=11, 208pt accent body — already structurally a section number per the existing `##` prompt, 3.94" wide ≥ 3"). New `Two Content` (slideLayout5), `Blank` (slideLayout6), and `Blank + Title` (slideLayout7) layouts were authored, registered in `slideMaster1.xml.rels` (rId6–rId8) + `sldLayoutIdLst` and `[Content_Types].xml`. All 4 embedded SVG decorations preserved byte-for-byte; `blue-corporate` is no longer allow-listed.

`modern.pptx` previously lacked `Two Content`, `Section Divider`, `Blank`, `Blank + Title`, and `Closing`. Pre-flight classification (`internal/template/ClassifyCanonicalRole`) identified two layouts that already structurally matched canonical roles — `Title + subtitle` (signature `subtitle+title` @ 0.95 → Title Slide) and `Content 1` (signature `body+title` @ 0.85 → One Content) — and two more that could be repurposed in place: `Table` (signature `title` @ 0.85 → Title Slide ambiguity, no usable body, fitting candidate for Section Divider repurpose) and `Content 3` (signature `body+image+title` @ 0.85 → One Content duplicate, dark photographic backdrop fitting a Closing visual identity). Repair: `Title + subtitle` was renamed to `Title Slide`; `Content 1` was renamed to `One Content` (the implicit-body placeholder retyped explicitly to `body` idx=1); `Table` was repurposed as `Section Divider` (tbl/dt/ftr/sldNum placeholders dropped, a `Section Number` body placeholder added in the upper-right at 3.7" wide × 136pt accent1 right-aligned with `##` prompt, and the title repositioned to the left of the section number); `Content 3` was repurposed as `Closing` (body idx=10 retyped to `subTitle` idx=1; slide4 — which uses this layout for its "THANK YOU" content — was updated in lockstep to keep its placeholder type aligned). New layouts authored: `Two Content` (slideLayout7, title + body idx=1 + body_2 idx=2 side-by-side), `Blank` (slideLayout8, empty `spTree`), `Blank + Title` (slideLayout9, title only). New layouts registered in `slideMaster1.xml.rels` (rId8/rId9/rId10), `sldLayoutIdLst` (ids 2147483700/701/702), and `[Content_Types].xml`. All 4 embedded media assets (hdphoto1.wdp, image1.png, image2.png, image3.png) preserved byte-for-byte (SHA-256 manifest verified pre/post repair). `modern` is no longer allow-listed.

`business-template.pptx` previously had no native `Two Content`, `Section Divider`, `Blank`, `Blank + Title`, or `Closing` layouts; `Title Slide` lacked a `subtitle`. Pre-flight classification (`internal/template/ClassifyCanonicalRole`) flagged four One Content candidates (`BLANK - Left LARGE Logo` @ `body+title` 0.85, `UNKNOWN IF USED 01` @ `body*2+title` 0.85, `Basic Page` @ `body+subtitle+title` 0.85) and one Title Slide candidate (`Custom Layout` @ `title` 0.85). Repair: `Custom Layout` renamed to `Title Slide` (subTitle idx=1 authored below the title; title widened to full width and repositioned at y=2.2M EMU + cy=2.2M EMU so 2-line wraps no longer collide with the subtitle); `Basic Page` renamed to `One Content` (body idx 14 → 1; sample slide3 updated in lockstep); `UNKNOWN IF USED 01` repurposed as `Section Divider` (already had a 208pt `##` body idx=13 in the upper-right at 4.94" wide — meets all template-spec thresholds — its cNvPr renamed to `Section Number` and lvl1pPr given `algn="r"` + accent1 solidFill; the extraneous 4.97" × 1.55" body idx=14 description was removed because it overflowed at text-fit P4 density and is not part of the canonical Section Divider role); `BLANK - Left LARGE Logo` repurposed as `Closing` (body idx=10 retyped to `subTitle` idx=1; slide1 updated in lockstep; and crucially, the layout's decorative bottom-right "My Consulting Company" Futura TextBox — which was also named `Title 1` and was being picked up by the engine as the title shape, routing slide titles to the wrong position — was renamed to `Company Branding`). New `Two Content` (slideLayout5, type=twoObj, title + body idx=1 + body_2 idx=2 side-by-side at 18pt), `Blank` (slideLayout6, type=blank), and `Blank + Title` (slideLayout7, type=titleOnly) layouts authored, registered in `slideMaster1.xml.rels` (rId6/rId7/rId8), `sldLayoutIdLst` (ids 2147483800/801/802), and `[Content_Types].xml`. The single embedded asset (`ppt/media/image1.emf`, sha256=8e815842b0d2622a657c6a18186292e8c3fb9a94ffc5cb558ac2961da02a2159) and `docProps/thumbnail.jpeg` (sha256=08fce7b6d96b7444b9eba655bcbf9ce9ffa8fe1447fc55df8be5cee990ea7f5c) preserved byte-for-byte in that repair. `business-template` is no longer allow-listed.

Portability follow-up (`go-slide-creator-a73sq`): removed the fixed "My Consulting Company" shapes from the Closing and One Content layouts, plus the sample logo image and its now-unused EMF relationship/content type. Layout IDs and placeholders remain unchanged. `template-check` now warns on non-placeholder text in any layout shape.

`modern-yellow.pptx` previously had `dk1=#FFFFFF` (theme polarity wrong — body text rendered white-on-white), Title Slide lacked `subtitle`, and `Two Content`, `Section Divider`, `Blank`, `Blank + Title`, `Closing` were all missing. Pre-flight classification (`internal/template/ClassifyCanonicalRole`) found two layouts that already structurally matched canonical roles — `111_Custom Layout` (signature `title` @ 0.85 → Title Slide) and `51_Custom Layout` (signature `body+title` @ 0.85 → One Content) — plus one layout (`42_Custom Layout`, two body placeholders, no title, no canonical role) whose existing `##` body placeholder in the upper-right was already structurally a Section Number, and one layout (`69_Custom Layout`, body only with decorative semicircle, no canonical role) that had no fitting canonical repurpose. Repair: `111_Custom Layout` renamed to `Title Slide` with a subtitle placeholder added below the title; `42_Custom Layout` repurposed as `Section Divider` (body idx=101 retyped to `title`; body idx=100 cNvPr renamed to `Section Number` — already had the `##` prompt + 208pt font at 4.88" wide upper-right — added right-align + accent1 fill; layout type set to `secHead`); `51_Custom Layout` renamed to `One Content` (body idx changed from 11 → 1 to satisfy canonical-placeholder gate; sample slide3 updated in lockstep); `69_Custom Layout` renamed to `Statement` (go-slide-creator-an1n) so its name-based classifier tag is `statement` rather than an empty tag set — it has no canonical role, so it stays a non-mandatory decorative single-body card; its decorative shapes are preserved and its `slideLayout4` ID is unchanged so existing `layout_id` pins still resolve. New `Two Content` (slideLayout5, type=twoObj), `Blank` (slideLayout6, type=blank), `Blank + Title` (slideLayout7, type=titleOnly), `Closing` (slideLayout8, ctrTitle + subTitle) layouts authored, registered in `slideMaster1.xml.rels` (rId6–rId9), `sldLayoutIdLst` (ids 2147483800–803), and `[Content_Types].xml`. Theme `dk1.lastClr` swapped from `FFFFFF` → `000000` so dk1 actually renders as the dark text colour (lt1 unchanged at `FFFFFF`, master clrMap unchanged). No `ppt/media/*` assets exist in this template (the only binary, `docProps/thumbnail.jpeg`, was preserved byte-for-byte; sha256=74f15ab92c943e7a908c4640d4ad8d7c8eaa74a94d9eb2930843235225ecdb20). `modern-yellow` is no longer allow-listed.

### Programmability

Templates fall into two categories:

- **Programmable** (regenerable from `cmd/mktemplate`): `forest-green`, `midnight-blue`, `warm-coral`.
- **Designer-owned** (must be repaired in place — `mktemplate` cannot reproduce embedded decorative assets, custom layout shapes, or intentional theme polarities): `abstract`, `blue-corporate`, `business-template`, `modern`, `modern-template`, `modern-yellow`.

See [TEMPLATE_ANALYSIS.md](TEMPLATE_ANALYSIS.md) for the full per-template matrix, current conformance status, and the in-place repair workflow for designer templates.

## CI Template Gate

Beyond `template-check`, every PR that touches `templates/`, `internal/template/`,
or `internal/examine/` runs the **template gate** (`.github/workflows/templates.yml`).
The gate runs `json2pptx examine-template <tpl> --gate` against every file in
`templates/` and **fails the build** if any of these hold for any template:

| Gate code | Fails when |
|-----------|-----------|
| `GATE.LAYOUT_EMPTY_TAGS` | A layout carries no classification tags (tag-based selection can never reach it). |
| `GATE.CANONICAL_COVERAGE_INCOMPLETE` | A content-bearing canonical family is missing — coverage must include all four of `title-slide`, `section-divider`, `one-content`, `qa-closing` — or the `one-content` family is covered only by Two Content (no One Content layout). |
| `GATE.TITLE_PLACEHOLDER_NAMING` | A title-typed placeholder is named anything other than exactly `title` on disk. |
| `GATE.SECTION_NUMBER_MISSING` | A section-divider layout has no placeholder named exactly `Section Number`. |
| `GATE.ERROR_FINDING` | The examination emitted any error-severity finding. |
| `GATE.TEMPLATE_CHECK_FAIL` | `template-check` reports any FAIL (one violation per failed check), so the gate never passes a template the conformance checker rejects. |
| `GATE.PROFILE_ERROR` | The template profile reports an error-severity diagnostic (`LAYOUT_RELATIONSHIP_INVALID`, including a dangling master → theme relationship, or `UNUSABLE_GEOMETRY`). |

`--gate` writes a `gate.json` verdict (`{template, passed, violations[]}`) into the
examination output directory and exits non-zero on any violation; the run always
leaves the full `examination/` tree (including annotated SVGs) behind, which the
workflow uploads as the `template-examination` artifact so reviewers can visually
inspect a template change. The gate is implemented in `internal/examine/gate.go`
and is distinct from the report's `findings` envelope: the four structural checks
are gate-only (the report surfaces missing coverage as a *warning*), the fifth
check folds in any error-severity finding the report already carries, and the last
two fold in every `template-check` FAIL and every error-severity profile diagnostic.

`validate-template` likewise folds every `template-check` FAIL into its findings
as an error-severity `TPL.TEMPLATE_ERROR` (message prefixed `template-check FAIL:`),
so `valid` is `false` and the command exits non-zero for a template that cannot
generate; `valid` no longer reflects only the optional metadata JSON.

Run the gate locally before opening a template PR:

```bash
json2pptx examine-template <your-template.pptx> --out ./examination --gate
```

## Creating a New Template

1. Start from an existing conformant template (e.g., `midnight-blue.pptx`)
2. Modify theme colors, fonts, and layout styling in PowerPoint
3. Ensure all 7 mandatory layouts are present with correct placeholder names
4. Run `json2pptx template-check <your-template.pptx>` to verify
5. Optionally add metadata at `ppt/go-slide-creator-metadata.json`
6. Test with `json2pptx generate -json examples/basic-deck.json -template <name>`
7. Run `json2pptx examine-template <your-template.pptx> --gate` — the same gate CI enforces (see [CI Template Gate](#ci-template-gate))
