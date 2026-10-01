# Style Defaults

Deck-level defaults let you set table styling and shape grid cell styling once at the top of a deck JSON file, instead of repeating it on every slide. Defaults are shallow-applied to every matching block before validation or conversion — so they work with all entry points (CLI `generate`, MCP `generate_presentation`, `validate`, and `dry_run`).

## Supported Kinds (V1)

| Kind | JSON key | Applies to |
|------|----------|------------|
| `table_style` | `defaults.table_style` | Every `type:"table"` content block and every table embedded in a `shape_grid` cell |
| `cell_style` | `defaults.cell_style` | Every `shape` in a `shape_grid` cell |

## Syntax

Add a top-level `"defaults"` object alongside `"slides"`:

```json
{
  "template": "midnight-blue",
  "defaults": {
    "table_style": {
      "use_table_style": true,
      "style_id": "@template-default",
      "borders": "all",
      "striped": true
    },
    "cell_style": {
      "geometry": "roundRect",
      "fill": "accent1"
    }
  },
  "slides": [ ... ]
}
```

### `table_style` Fields

These mirror the `style` object on a table (`TableStyleInput`):

| Field | Type | Description |
|-------|------|-------------|
| `header_background` | string | Semantic color for the header row background (e.g. `"accent1"`) |
| `borders` | string | Border mode: `"all"`, `"none"`, `"horizontal"`, `"vertical"` |
| `striped` | bool | Alternate row shading |
| `use_table_style` | bool | Enable OOXML table style rendering |
| `style_id` | string | Table style GUID or `"@template-default"` sentinel |

### `cell_style` Fields

These mirror `ShapeSpecInput`, the shape definition on a grid cell:

| Field | Type | Description |
|-------|------|-------------|
| `geometry` | string | Preset geometry name (e.g. `"roundRect"`, `"rect"`) |
| `fill` | string/object | Semantic color string or `{"color": "...", "alpha": N}` |
| `line` | string/object | Outline specification |
| `text` | object | Default text properties |
| `rotation` | number | Rotation in degrees |
| `adjustments` | object | Geometry adjustment handles |
| `icon` | object | Icon overlay specification |

## Swap-Only Semantics

Defaults use **swap-only** (shallow) merge — there is no deep merge of nested objects.

The rule: if a block sets a field inline, that field wins entirely. If a field is absent (zero value), the default fills it in.

```json
{
  "defaults": {
    "table_style": {
      "borders": "all",
      "striped": true,
      "style_id": "@template-default"
    }
  },
  "slides": [{
    "content": [{
      "type": "table",
      "table_value": {
        "headers": ["A", "B"],
        "rows": [["1", "2"]],
        "style": {
          "borders": "none"
        }
      }
    }]
  }]
}
```

Result for this table:
- `borders` = `"none"` — inline wins
- `striped` = `true` — filled from default (not set inline)
- `style_id` = `"@template-default"` — filled from default (not set inline)

If a table has **no** `style` object at all, the entire default is adopted as a single unit.

## Engine Table Defaults (no style authored)

Independent of the `defaults` block, the renderer applies a consulting table look whenever the author leaves the style unset (go-slide-creator-weaq, restyled by go-slide-creator-1iiej). Only theme scheme colors are used, so the look follows the template. The **engine default** applies when `use_table_style` is false and `style_id` is unset or the engine default GUID (a `header_background` does not leave it — see below) — and as the fallback for `use_table_style` / `"@template-default"` when the template ships no formatting for its table style (see below):

| Aspect | Default | Opt out |
|--------|---------|---------|
| Header row | **no fill** (never a solid black or accent bar), 11pt bold text color (`dk1`) over a 1pt `dk1` rule | `header_background` (e.g. `"accent1"`, `"lt2"`) adds a header fill only; `use_table_style: true` with a template that defines its style, or an explicit template `style_id`, leaves the engine default |
| Body rows | 12pt, separated by 0.5pt `dk1`-at-15% hairline rules; no vertical rules, no rule under the last row | set `borders` (`"all"`, `"horizontal"`, `"outer"`, `"none"`) for the legacy grid rules |
| Banding | **no zebra stripes** | `striped: true` |
| First column | bold (the row label) | — |
| Numeric columns | right-aligned (data and header) — applies to every table, not only the engine default | `column_types` unset; a column whose every non-empty cell reads as a number (`24.1`, `+4%`, `$1.2M`, `2.8x`, `(3.0)`); opt out with `column_types` or a per-column `column_alignments` entry |
| Total row | bold with a 1pt `dk1` top rule | first cell is `Total` / `Totals` / `Grand total` / `Sum` / `Subtotal` (case-insensitive) — in addition to `totals_row: true` for the last row; rename the label to opt out |
| Row height | content-driven minimum (one line at the table font plus insets, never below 0.4in); rows are never stretched to fill the placeholder, and the table is top-anchored under the title | — |

Type size: the engine default starts at 12pt rows / 11pt header (a template `style_id` / `use_table_style` that opts out keeps the legacy 18pt start). Wide tables are capped at the 18pt-equivalent width budget (18pt × 4 / columns), so a default table stays at 12pt up to six columns and shrinks from seven, never below the 10pt table readability floor; the `table_font_scaled` finding and the validate-time preflight predict the same sizes. TDR density limits (rows ≤ 7, cols ≤ 6) are unchanged.

**`header_background`, `borders` and `striped` are additive** (go-slide-creator-87eu0): each changes only its own aspect on top of the engine default. A `header_background` alone fills the header row and keeps the 12pt / 11pt type, the horizontal rules (no vertical rules, no outer frame) and the unbanded rows; `borders: "all"` or `striped: true` opt in to the grid or zebra stripes individually. Header text on a filled header is `lt1` or `dk1`, whichever contrasts more with the resolved fill (scheme names resolve through the template theme, hex values directly); without a theme, dark scheme names (`accent1`–`accent6`, `dk1`, `dk2`, `tx1`, `tx2`) get `lt1`.

## Application Order

Defaults are applied **after** JSON unmarshal but **before** struct validation or conversion into internal types. The call sequence is:

1. JSON → `PresentationInput` (including `split_slide` expansion)
2. **`applyDefaults()`** — fills missing fields from `defaults`
3. Struct validation / type coercion
4. Generation pipeline

This means defaults participate in all downstream validation (fit-report, strict-fit checks, etc.) exactly as if the author had written them inline.

## Scope Rules

- **Per-deck only**: defaults apply within a single JSON input file. There is no cross-deck inheritance in V1.
- **No cascade**: defaults do not cascade into nested structures beyond the immediate target. For example, `cell_style` applies to `shape_grid` cell shapes but does not reach into a table embedded inside that cell — use `table_style` for that.
- **Nested sub-grids count**: a `shape_grid` cell whose content is a nested `grid` is still part of the shape grid, so `cell_style` and `table_style` reach the shapes and tables inside it, at any depth.

## Namespace: `@template-default` Sentinel

The `style_id` field accepts the special value `"@template-default"`, which resolves to the template's declared default table style GUID at generation time (see `internal/template/table_style_resolver.go`).

Resolution rules:
- `"@template-default"` → template's `tableStyles.xml` `def` attribute, falling back to the engine default GUID if the template declares none
- `"{GUID}"` present in the template → returned as-is (validated)
- Empty string → engine default GUID

The `@template-default` sentinel lives in a separate namespace from user-authored style IDs — there is no collision risk with OOXML GUIDs.

If the resolved GUID has no non-empty `<a:tblStyle>` definition in the template, the output keeps that GUID but renders the table explicitly in the engine default look above (unfilled bold header over a 1pt rule, hairline row rules, no zebra); a `header_background` adds the header fill on top. This prevents an empty style list or portability-only stub from leaving the table visually plain. Defined template styles remain in control.

**Validation:** a `style_id` must be empty, the `@template-default` sentinel, or a well-formed OOXML table style GUID (`{8-4-4-4-12}` hex, e.g. `{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}`). Any other value — a typo or a string containing XML metacharacters such as `"&<` — is rejected with an `INVALID_PARAMETER` validation error and is never emitted into slide XML or `ppt/tableStyles.xml` (the renderer drops it defensively even when validation is skipped). A well-formed GUID that the template does not declare is allowed but produces the advisory `unknown_table_style_id` warning.

## Template Surface Properties

Templates expose two additional style-related properties via their embedded metadata (`ppt/go-slide-creator-metadata.json`). These are **not** user-settable in the input JSON — they are authored into the template and consumed automatically by the generator and pattern system.

### `surface_tints`

A map of surface roles to scheme color names. Patterns use `ResolveSurface(role)` to pick background fills that harmonize with the template's visual identity.

| Role       | Purpose                                        | Typical Value |
|------------|------------------------------------------------|---------------|
| `subtle`   | Lightest tint (alternate row, card background) | `"lt2"`       |
| `paper`    | Card/panel background                          | `"lt1"`       |
| `elevated` | Raised surface for contrast                    | `"lt2"`       |
| `inverse`  | Dark surface for high-contrast sections        | `"dk2"`       |

### `data_palette`

An ordered list of scheme color names for chart series. `svggen` uses this to ensure chart colors match the template's visual identity instead of using a fixed order.

Example (from midnight-blue):
```json
["accent1", "accent2", "accent3", "accent4", "accent6", "accent5"]
```

All 5 bundled templates define both `surface_tints` and `data_palette`.

## Per-Cell Conditional Formatting

A table cell can carry a `conditional` block that tints it when the cell's own
content satisfies a rule:

```json
{"content": "On track", "conditional": {"rule": "equals", "threshold": "On track", "fill": "accent3"}}
```

| `rule` | Matches when | `threshold` |
|---|---|---|
| *(omitted)* / `always` | always — the plain "highlight this cell" form | — |
| `positive` | the cell reads as a number > 0 | — |
| `negative` | the cell reads as a number < 0 (accounting parentheses count) | — |
| `threshold` / `gte` | cell ≥ threshold | number (a numeric string is accepted) |
| `lte` | cell ≤ threshold | number |
| `between` | lo ≤ cell ≤ hi, in either order | two-number array, e.g. `[0, 5]` |
| `equals` | the cell's text matches, ignoring case and padding — or the numbers match, so `50` matches `"50%"` | string or number |
| `contains` | the threshold appears in the cell's text, ignoring case | non-empty string |

`fill` is a scheme color (`accent3`) or a 6-digit hex; it renders as a 20% tint,
so the cell's own text stays legible. A rule the cell does NOT satisfy leaves it
with the table's normal fill.

Numbers are read out of the cell's text, so `"+4%"`, `"(3.2)"` and
`"EUR 1,186.4"` all compare; a percent sign is a unit, not a scale, so a
threshold for `"50%"` is written `50`.

**Diagnostics.** A rule outside the list above, or a threshold the rule cannot
use (a word where a number is compared, a `between` without two bounds), is an
`INVALID_PARAMETER` error at
`…rows[r][c].conditional.rule` / `.threshold` with the allowed list and a
`did_you_mean` for a typo. Before go-slide-creator-6hlu a string threshold
aborted the entire deck at parse time with a Go type error, and the rule itself
was never evaluated — a cell tagged `negative` was tinted whatever it said.

## What Is NOT Defaultable

In V1, only `table_style` and `cell_style` are supported. The following are **not** part of the defaults system:

- Slide-level properties (background, transition, speaker notes)
- Content-level font size overrides
- Chart or diagram styling
- Pattern parameters
- Footer configuration
- Theme overrides
- Surface tints and data palette (template-authored, not user-settable)

## Example: Multi-Table Deck with Defaults

This is a realistic pattern for agent-generated decks with many tables:

```json
{
  "template": "midnight-blue",
  "defaults": {
    "table_style": {
      "use_table_style": true,
      "style_id": "@template-default"
    }
  },
  "slides": [
    {
      "slide_type": "content",
      "content": [
        {
          "placeholder_id": "title",
          "type": "text",
          "text_value": "Revenue by Region"
        },
        {
          "placeholder_id": "body",
          "type": "table",
          "table_value": {
            "headers": ["Region", "Revenue", "Growth"],
            "rows": [
              ["North America", "$12.4M", "+8%"],
              ["Europe", "$8.7M", "+5%"],
              ["Asia-Pacific", "$6.2M", "+14%"]
            ]
          }
        }
      ]
    },
    {
      "slide_type": "content",
      "content": [
        {
          "placeholder_id": "title",
          "type": "text",
          "text_value": "Cost Breakdown"
        },
        {
          "placeholder_id": "body",
          "type": "table",
          "table_value": {
            "headers": ["Category", "Amount"],
            "rows": [
              ["Engineering", "$4.2M"],
              ["Marketing", "$2.1M"],
              ["Operations", "$1.8M"]
            ],
            "style": {
              "borders": "horizontal"
            }
          }
        }
      ]
    }
  ]
}
```

In this deck:
- The first table has no inline `style` — it gets the full `table_style` default (`use_table_style: true`, `style_id: "@template-default"`).
- The second table sets `borders: "horizontal"` inline — that field wins, but `use_table_style` and `style_id` are still filled from defaults.

## Implementation

- Type definitions: `cmd/json2pptx/json_schema.go` (`DefaultsInput`, `PresentationInput`)
- Application logic: `cmd/json2pptx/defaults.go` (`applyDefaults`, `applyTableStyleDefaults`, `applyShapeGridDefaults`, `applyCellStyleDefaults`)
- Tests: `cmd/json2pptx/defaults_test.go`
- Sentinel resolution: `internal/template/table_style_resolver.go` (`ResolveTableStyleID`)
