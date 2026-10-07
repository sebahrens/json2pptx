# House Diagram

A strategy house: a gabled roof (the vision or objective) over a row of pillars, any further levels, and a foundation band.

`house_diagram` is drawn as native PowerPoint shapes by the same builder as the [`strategy-house` pattern](../PATTERNS.md#strategy-house-and-house_diagram-one-house-builder), so the two look alike on a template: one solid accent for the roof, its tint for the bands and the pillar caps, pale pillar shafts, a neutral-dark base (the lowest level), a gable pitched from the house's width, and levels sized from their text.

## Type Identifier

`house_diagram`

## Use Cases

- Strategy house: vision, strategic pillars, shared foundation
- Operating model: principles over capabilities over enablers
- Framework with one frame and parallel elements (the 7S elements under shared values)

Prefer the `strategy-house` pattern (or the DeckSpec `pillars` kind) on a pattern slide; use `house_diagram` where a diagram is placed — a body placeholder, a `shape_grid` cell or a `compose` segment.

## Data Structure

```json
{
  "type": "house_diagram",
  "data": {
    "roof": "Leader in digital payments",
    "sections": [
      {"label": "Technology", "items": ["Cloud platform", "Open APIs"]},
      {"label": "Product", "items": ["Mobile wallet"]},
      {"label": "People", "items": ["Talent", "Culture"]}
    ],
    "floors": [
      "Shared data platform",
      {"sections": ["Risk", "Controls", "Partners"]}
    ],
    "foundation": "Trust and compliance"
  }
}
```

## Fields

| Field | Type | Description |
|-------|------|-------------|
| `roof` | `string` or `{label}` | Text in the roof. Omitted: a bare gable. |
| `sections` | `(string \| {label, items?})[]` | The pillar row (aliases: `pillars`, `columns`; give one). `items` are bullets. |
| `floors` | `floor[]` | Further levels, drawn top to bottom under the pillar row. |
| `foundation` | `string` or `{label}` | The bottom band. |

A **floor** is one of:

| Shape | Drawn as |
|-------|----------|
| `"Band label"` | a full-width band |
| `{"label": "Band label", "items"?: ["…"]}` | a full-width band; items on one line under the label |
| `{"sections": [{"label": "…", "items"?: ["…"]}]}` | a row of cells (strings allowed). A row with `items` is a pillar row; a label-only row is a level split into boxes |

`"type": "single"` / `"parallel"` may be written on a floor object and must agree with its shape.

Without `sections`, `floors` alone lists every level in the order drawn (a band above the pillars, the pillar row, bands below).

`center_element` (for `roof`) and `outer_elements` (for `sections`) are read when the house has no `roof` / no `sections` and no `floors`.

## What Is Refused

Validation and generation refuse, at the field, anything the builder would not draw:

- a `floors` value that is not a list, or a floor that is not one of the shapes above (a number, a nested list, an object with neither `label` nor `sections`, an object with both)
- `sections` entries that are not a string or an object; `items` that are not a list of strings
- the pillar row given twice (`sections` and `pillars`), or `outer_elements` beside `sections` / `floors`
- more than 12 cells in a level
- levels whose cell counts cannot share one column grid (the least common multiple of the counts must be at most 24: 4 over 2, 3 over 6, 5 over 5)
- unknown keys (`title` for `label`, `bullets` for `items`), with the key that is drawn

## Layout Notes

- The roof is a gable pentagon spanning the house; its text sits in the band under the slope. The gable rises one twelfth of the width where the height allows and flattens toward one twentieth as the levels need the height.
- Every band is as tall as its text; a pillar row is as tall as its tallest column and grows slightly into spare height. The house hangs from the top of its region.
- Six or more sections in a row switch the house to the dense type sizes (12pt labels, 11pt items).
- A region too small for the levels' text is reported (`DIAGRAM_REGION_TOO_SMALL`, `diagram.text_below_readable_min`), not clipped.

## Examples

### Classic house

```json
{
  "type": "house_diagram",
  "data": {
    "roof": "Vision: Be the Global Leader in Digital Payments",
    "sections": [
      {"label": "Technology", "items": ["Cloud infrastructure", "API platform", "Security"]},
      {"label": "Product", "items": ["Mobile payments", "Merchant tools", "Analytics"]},
      {"label": "People", "items": ["Talent development", "Culture", "Leadership"]},
      {"label": "Operations", "items": ["Process excellence", "Compliance", "Risk management"]}
    ],
    "foundation": "Core Values: Integrity, Innovation, Customer Focus, Collaboration"
  }
}
```

### Every level in `floors`

```json
{
  "type": "house_diagram",
  "data": {
    "roof": "Mission",
    "floors": [
      {"label": "One operating model", "items": ["Group standards", "Local execution"]},
      {"sections": [{"label": "Grow", "items": ["New markets"]}, {"label": "Run", "items": ["Lean core"]}]},
      "Enablers"
    ],
    "foundation": "Values"
  }
}
```

### 7S elements under shared values

```json
{
  "type": "house_diagram",
  "data": {
    "center_element": {"label": "Shared Values"},
    "outer_elements": ["Strategy", "Structure", "Systems", "Style", "Staff", "Skills"]
  }
}
```

## See Also

- [Patterns: strategy-house](../PATTERNS.md#strategy-house-and-house_diagram-one-house-builder)
- [Pyramid](./pyramid.md) - For a hierarchy that narrows
- [Business Model Canvas](./business_model_canvas.md) - For structured frameworks
