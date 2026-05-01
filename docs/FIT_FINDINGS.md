# Fit Findings

Fit findings are structured diagnostics emitted when generated slide content may not render correctly — text overflowing placeholders, shapes falling outside slide bounds, or tables exceeding density limits. They are surfaced via the MCP `generate_presentation` tool (when `fit_report=true`) and the CLI `validate -fit-report` command.

## Finding Structure

Every finding is a `FitFinding` (defined in `internal/patterns/fit_finding.go`) that embeds `ValidationError`. In JSON output, all fields are flattened to the top level:

```json
{
  "pattern": "placeholder",
  "path": "slides[0].content.body",
  "code": "placeholder_overflow",
  "message": "text overflows placeholder by 42% (360pt frame, autofit=none); overflow persists at minimum font scale",
  "fix": { "kind": "reduce_text" },
  "action": "shrink_or_split",
  "measured": { "width_emu": 7772400, "height_emu": 6515100 },
  "allowed": { "width_emu": 7772400, "height_emu": 4572000 },
  "overflow_ratio": 1.42
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `pattern` | string | Source context: `"placeholder"`, `"table"`, `"shape_grid"` |
| `path` | string | JSON path to the offending element, e.g. `slides[2].content.body` |
| `code` | string | Machine-readable code (see catalog below) |
| `message` | string | Human-readable description |
| `fix` | object | Structured remediation: `{kind, params?}` |
| `action` | string | Recommended severity/remediation action |
| `measured` | object | Actual content extent in EMU (omitted when N/A) |
| `allowed` | object | Available frame extent in EMU (omitted when N/A) |
| `overflow_ratio` | float | `measured / allowed` as a fraction (omitted when 0) |

## Actions

Actions indicate severity and recommended remediation. They are ranked from most to least severe:

| Rank | Action | Meaning |
|------|--------|---------|
| 3 | `refuse` | Content cannot be rendered correctly. In `strict` mode, generation is blocked. |
| 2 | `shrink_or_split` | Content overflows significantly. Agent should reduce text, split the slide, or restructure. |
| 1 | `review` | Content may not render ideally. Human or agent review recommended but not blocking. |
| 0 | `info` | Informational signal. No action required. |

The `ActionRank(action)` function returns these numeric ranks. Unknown actions return -1. Findings are sorted by rank descending (most severe first), then by slide index ascending.

## Finding Codes

### `placeholder_overflow`

**Action:** `shrink_or_split`
**Pattern:** `placeholder`
**Fix kind:** `reduce_text`

Text in a body or content placeholder overflows its frame. Emitted only when all three conditions hold simultaneously:

1. **Significant overshoot** — measured height exceeds frame height by >15% (the `overflowThreshold` constant filters measurement noise).
2. **No autofit** — the placeholder's OOXML autofit mode is `noAutofit` or absent. When `normAutofit` or `spAutoFit` is active, PowerPoint auto-shrinks text, so the finding is suppressed.
3. **Unfixable at min scale** — even at the minimum autofit font scale, `textfit.Calculate` still reports overflow. If hypothetically adding normAutofit would fix it, the finding is not emitted.

```json
{
  "pattern": "placeholder",
  "path": "slides[0].content.body",
  "code": "placeholder_overflow",
  "message": "text overflows placeholder by 42% (360pt frame, autofit=none); overflow persists at minimum font scale",
  "fix": { "kind": "reduce_text" },
  "action": "shrink_or_split",
  "measured": { "width_emu": 7772400, "height_emu": 6515100 },
  "allowed": { "width_emu": 7772400, "height_emu": 4572000 },
  "overflow_ratio": 1.42
}
```

### `title_wraps`

**Action:** `review`
**Pattern:** `placeholder`
**Fix kind:** `shorten_title`

Title text wraps to multiple lines within its placeholder. This is common and often acceptable, so the action is `review` rather than `shrink_or_split`. Emitted when the measured text height exceeds a single-line height (computed as `fontSize * 1.2 line spacing`).

```json
{
  "pattern": "placeholder",
  "path": "slides[1].content.title",
  "code": "title_wraps",
  "message": "title wraps to multiple lines (36pt font, 9.1\" wide placeholder)",
  "fix": { "kind": "shorten_title" },
  "action": "review",
  "measured": { "width_emu": 8229600, "height_emu": 731520 },
  "allowed": { "width_emu": 8229600, "height_emu": 548640 },
  "overflow_ratio": 1.33
}
```

### `slide_bounds_overflow`

**Action:** `shrink_or_split`
**Pattern:** `shape_grid`
**Fix kind:** `reposition_shape`

A JSON-authored shape's center falls outside the slide rectangle. Uses center-based threshold (not corner-based) to avoid false positives from 1-EMU rounding. Only checks shapes authored in JSON input — layout-inherited shapes are excluded.

```json
{
  "pattern": "shape_grid",
  "path": "slides[2].shape_grid.rows[1].cells[0]",
  "code": "slide_bounds_overflow",
  "message": "shape center (10058400, 7315200) EMU falls outside slide bounds (9144000 x 6858000) vertically",
  "fix": { "kind": "reposition_shape" },
  "action": "shrink_or_split",
  "measured": { "width_emu": 4572000, "height_emu": 3429000 },
  "allowed": { "width_emu": 9144000, "height_emu": 6858000 }
}
```

### `footer_collision`

**Action:** `review` (default) or `refuse` (strict mode)
**Pattern:** `shape_grid`
**Fix kind:** `reposition_shape`

A JSON-authored shape intrudes into the footer reserved area. The action depends on the `strict_fit` setting: `"strict"` produces `refuse`, `"warn"` produces `review`, `"off"` suppresses entirely.

Only fires when the slide's resolved layout declares a footer placeholder (date, footer text, or slide number). This prevents false positives on layouts that use heuristic fallback positioning.

```json
{
  "pattern": "shape_grid",
  "path": "slides[3].shape_grid.rows[2].cells[0]",
  "code": "footer_collision",
  "message": "shape bottom edge (6400000 EMU) intrudes 142000 EMU into footer area (top=6258000 EMU)",
  "fix": { "kind": "reposition_shape" },
  "action": "review",
  "measured": { "width_emu": 4572000, "height_emu": 3429000 },
  "allowed": { "width_emu": 4572000, "height_emu": 2829000 }
}
```

### `fit_overflow`

**Action:** `shrink_or_split` (mapped from internal `unfittable`)
**Pattern:** `table`
**Fix kind:** `reduce_text` (headers) or `split_at_row` (data cells)

Text in a table cell exceeds the available cell height. The `split_at_row` fix includes a `row` parameter suggesting where to split the table.

```json
{
  "pattern": "table",
  "path": "slides[0].content[0].rows[3][1]",
  "code": "fit_overflow",
  "message": "text needs 4 lines @ 12pt; cell allows 2",
  "fix": { "kind": "split_at_row", "params": { "row": 4 } },
  "action": "shrink_or_split",
  "measured": { "height_emu": 609600 },
  "allowed": { "height_emu": 370840 },
  "overflow_ratio": 1.64
}
```

### `density_exceeded`

**Action:** `shrink_or_split` (mapped from internal `unfittable`)
**Pattern:** `table`
**Fix kind:** `split_at_row`

Table has more cells than the TDR (Table Density Ratio) ceiling allows for the computed font size. The ceiling varies by font size: 60 cells at 18pt, 80 at 14pt, 100 at 12pt, 120 at 10pt.

```json
{
  "pattern": "table",
  "path": "slides[0].content[0]",
  "code": "density_exceeded",
  "message": "table has 72 cells (8 rows × 9 cols) at 12pt; TDR ceiling is 60",
  "fix": { "kind": "split_at_row", "params": { "row": 4 } },
  "action": "shrink_or_split"
}
```

### `contrast_autofixed`

**Action:** `info`
**Pattern:** `placeholder`
**Fix kind:** `replace_color`

Text color was automatically replaced to meet WCAG AA contrast requirements against the resolved layout background. This is informational — the fix has already been applied. The `fix.params` include the original and replacement colors, the background color, and the contrast ratios before and after the swap.

```json
{
  "pattern": "placeholder",
  "path": "slides[1].content.body",
  "code": "contrast_autofixed",
  "message": "auto-fixed low-contrast text: #FFFFFF → #1A1A1A (on #F5F5F5, ratio 1.3 → 15.2)",
  "fix": {
    "kind": "replace_color",
    "params": {
      "original_color": "#FFFFFF",
      "replacement_color": "#1A1A1A",
      "background_color": "#F5F5F5",
      "contrast_ratio_before": 1.3,
      "contrast_ratio_after": 15.2
    }
  },
  "action": "info"
}
```

## Scope Rules

Fit findings are scoped to **JSON-authored content only**. Content inherited from template layouts or masters is never checked.

### What is checked

- **Placeholder text** — body, content, and title placeholders populated from `slides[].content[]`
- **Shape grid cells** — shapes and tables authored in `slides[].shape_grid`
- **Content-level tables** — tables in `slides[].content[]` with `type: "table"`

### What is excluded

- **Layout-inherited shapes** — shapes that come from the template's slide layout or master are never checked. Callers filter these before passing to detectors.
- **Decorative shapes** — shapes with `role: "background"` or `role: "decor"` are skipped by `slide_bounds_overflow` and `footer_collision`. These are intentionally placed at edges or off-slide.
- **Autofit placeholders** — `placeholder_overflow` is suppressed when the placeholder has `normAutofit` or `spAutoFit` set, because PowerPoint will auto-shrink text to fit.
- **Layouts without footer** — `footer_collision` only fires when the slide's resolved layout declares a footer placeholder (dt, ftr, or sldNum). No finding is emitted on layouts using heuristic fallback positioning.

## Fix Kinds

Each finding includes a structured `fix` object with a machine-readable `kind`:

| Kind | Params | Description |
|------|--------|-------------|
| `reduce_text` | — | Shorten the text content to fit the available space |
| `shorten_title` | — | Shorten the title to avoid wrapping |
| `reposition_shape` | — | Move or resize the shape to stay within bounds |
| `split_at_row` | `row: int` | Split the table at the suggested row index |
| `use_one_of` | `available: string`, `did_you_mean?: string` | Replace the value with one of the listed alternatives |
| `replace_color` | `original_color: string`, `replacement_color: string`, `background_color: string`, `contrast_ratio_before: float`, `contrast_ratio_after: float` | Text color was auto-replaced for WCAG AA contrast compliance |

## Per-Slide Finding Budget

To prevent noisy output on dense decks, findings are capped at **5 per slide** by default. Within each slide, findings are ranked by:

1. **Severity** — `refuse` > `shrink_or_split` > `review` > `info`
2. **Actionability** — findings with a `fix` object rank above those without

When more than 5 findings exist on a slide, the top 5 are returned plus a summary finding with code `findings_truncated`:

```json
{
  "path": "slides[2]",
  "code": "findings_truncated",
  "message": "8 more findings suppressed on this slide; use verbose_fit to see all",
  "action": "info"
}
```

To bypass the budget and see all findings, pass `verbose_fit: true` (MCP) or `--verbose-fit` (CLI).

## Accessing Fit Findings

### MCP (generate_presentation)

Pass `fit_report: true` in the tool input. Findings appear in the response under `fit_findings`:

```json
{
  "file_path": "/tmp/out/deck.pptx",
  "fit_findings": [ ... ]
}
```

Pass `verbose_fit: true` to return all findings without the per-slide budget limit.

### CLI (validate)

```bash
json2pptx validate -fit-report examples/basic-deck.json
json2pptx validate -fit-report -verbose-fit examples/basic-deck.json
```

Findings are printed to stderr grouped by slide. Exit code is nonzero only if any finding has action `refuse`.

### Compact Responses

MCP clients can negotiate compact (non-indented) JSON output via capability negotiation. Send `experimental.compact_responses: true` in the client capabilities during the MCP `initialize` handshake. The server advertises support for this in its own `experimental` capabilities.

The `MCP_COMPACT_RESPONSES=1` environment variable is still honored as a fallback but is deprecated and will be removed in a future release.
