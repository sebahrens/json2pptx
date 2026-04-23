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

## Namespace: `@template-default` Sentinel

The `style_id` field accepts the special value `"@template-default"`, which resolves to the template's declared default table style GUID at generation time (see `internal/template/table_style_resolver.go`).

Resolution rules:
- `"@template-default"` → template's `tableStyles.xml` `def` attribute, falling back to the engine default GUID if the template declares none
- `"{GUID}"` present in the template → returned as-is (validated)
- Empty string → engine default GUID

The `@template-default` sentinel lives in a separate namespace from user-authored style IDs — there is no collision risk with OOXML GUIDs.

## What Is NOT Defaultable

In V1, only `table_style` and `cell_style` are supported. The following are **not** part of the defaults system:

- Slide-level properties (background, transition, speaker notes)
- Content-level font size overrides
- Chart or diagram styling
- Pattern parameters
- Footer configuration
- Theme overrides

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
