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

Independent of the `defaults` block, the renderer applies a consulting-grade table look whenever the author leaves the relevant field unset (go-slide-creator-weaq). Only theme scheme colors are used, so the look follows the template:

| Aspect | Default | Applies when | Opt out |
|--------|---------|--------------|---------|
| Header row | `accent1` fill, bold, `lt1` text | `header_background` unset, `use_table_style` false, and `style_id` unset or the engine default GUID | set `header_background` (e.g. `"none"`, `"lt2"`), `use_table_style: true`, or an explicit `style_id` |
| Numeric columns | right-aligned (data and header) | `column_types` unset; a column whose every non-empty cell reads as a number (`24.1`, `+4%`, `$1.2M`, `2.8x`, `(3.0)`) | set `column_types` or a per-column `column_alignments` entry |
| Total row | bold with a `dk1` top rule | first cell is `Total` / `Totals` / `Grand total` / `Sum` / `Subtotal` (case-insensitive) — in addition to `totals_row: true` for the last row | rename the label |
| Row height | content-driven minimum (one line at the table font plus insets, never below 0.4in); rows are no longer stretched to fill the placeholder | always | — |

A header with an explicit dark scheme `header_background` (`accent1`–`accent6`, `dk1`, `dk2`, `tx1`, `tx2`) also gets `lt1` header text.

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

If the resolved GUID has no non-empty `<a:tblStyle>` definition in the template, the output keeps that GUID but renders the table with explicit theme-based header fill, bold text, and cell formatting. This prevents an empty style list or portability-only stub from leaving the table visually plain. Defined template styles remain in control.

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
