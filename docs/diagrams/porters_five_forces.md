# Porter's Five Forces

Analyze industry competitive dynamics using Michael Porter's framework.

## Type Identifier

`porters_five_forces`

## Use Cases

- Industry analysis
- Competitive strategy development
- Market entry assessment
- Strategic planning

## Data Structure

```json
{
  "type": "porters_five_forces",
  "title": "Industry Analysis",
  "data": {
    "industry_name": "Cloud Computing",
    "forces": [
      {"type": "rivalry", "label": "Competitive Rivalry", "intensity": 0.8},
      {"type": "new_entrants", "label": "Threat of New Entrants", "intensity": 0.4},
      {"type": "substitutes", "label": "Threat of Substitutes", "intensity": 0.3},
      {"type": "suppliers", "label": "Supplier Power", "intensity": 0.5},
      {"type": "buyers", "label": "Buyer Power", "intensity": 0.7}
    ]
  }
}
```

## Required Fields

Two input shapes are accepted: the **array form** (`forces`) and the **object-keyed form**. Use either one.

### Array form

| Field | Type | Description |
|-------|------|-------------|
| `forces` | `object[]` | The five forces data |
| `forces[].type` | `string` | Force type — **must be one of the canonical values** `rivalry`, `new_entrants`, `substitutes`, `suppliers`, `buyers` (the array form does **not** accept synonyms; an unrecognized `type` is dropped and that box falls back to defaults) |
| `forces[].intensity` | `number` | Force strength, `0.0` to `1.0`. **Omit it and the force is drawn unscored**: the neutral surface, and no intensity line. It used to default to `0.5`, so an unscored chart printed "Medium (50%)" under all five forces and tinted them all alike — an assessment nobody made (go-slide-creator-ceodq). |

### Object-keyed form

Instead of an array, each force can be a top-level key in `data`. This form **does** accept synonyms (mapped to the canonical force). Forces are rendered in fixed layout order regardless of key order.

```json
{
  "type": "porters_five_forces",
  "data": {
    "industry_name": "Cloud Computing",
    "rivalry":        {"label": "Competitive Rivalry", "intensity": 0.8, "description": "Many hyperscalers"},
    "new_entrants":   {"intensity": 0.4, "factors": ["High capital", "Economies of scale"]},
    "substitutes":    {"intensity": 0.3},
    "supplier_power": {"intensity": 0.5},
    "buyer_power":    {"intensity": 0.7}
  }
}
```

| Canonical force | Accepted keys (synonyms) |
|-----------------|--------------------------|
| `rivalry` | `rivalry`, `competitive_rivalry` |
| `new_entrants` | `new_entrants`, `threat_of_new_entrants` |
| `substitutes` | `substitutes`, `threat_of_substitutes` |
| `suppliers` | `suppliers`, `supplier_power`, `bargaining_power_of_suppliers` |
| `buyers` | `buyers`, `buyer_power`, `bargaining_power_of_buyers` |

Each force object takes `{label?, intensity (0.0-1.0), factors?: string[], description?}`. When `factors` is absent, a `description` string is shown as a single supporting line.

> **Note:** an object-keyed payload with **no** recognized force keys renders a blank diagram. If a slide comes back empty, check that your keys match the table above.

## Force Types

| Type | Position | Description |
|------|----------|-------------|
| `rivalry` | Center | Competitive rivalry among existing firms |
| `new_entrants` | Top | Threat of new entrants |
| `substitutes` | Bottom | Threat of substitute products |
| `suppliers` | Left | Bargaining power of suppliers |
| `buyers` | Right | Bargaining power of buyers |

## Intensity Scale

| Value | Meaning | Fill of the force |
|-------|---------|-------|
| 0.0 - 0.33 | Low intensity | The accent's Lighter 90% swatch |
| 0.34 - 0.66 | Medium intensity | Lighter 80% |
| 0.67 - 1.0 | High intensity | Lighter 50% |
| omitted | Unscored | The neutral surface (`dk1` at 4%) |

Rivalry is always the solid accent block, scored or not; its intensity is
stated by its intensity line.

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `industry_name` | `string` | - | Name of industry analyzed |
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle |
| `footnote` | `string` | - | Footnote text |

## Force Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `label` | `string` | Custom label (default: force name) |
| `description` | `string` | Additional details |
| `factors` | `string[]` | Key drivers of this force |
| `color` | `string` | Custom hex color |

## Examples

### Basic Industry Analysis

```json
{
  "type": "porters_five_forces",
  "title": "Airline Industry Analysis",
  "data": {
    "industry_name": "Commercial Airlines",
    "forces": [
      {"type": "rivalry", "intensity": 0.9, "label": "Intense Rivalry"},
      {"type": "new_entrants", "intensity": 0.3, "label": "Low Threat"},
      {"type": "substitutes", "intensity": 0.4, "label": "Rail/Video"},
      {"type": "suppliers", "intensity": 0.8, "label": "Boeing/Airbus"},
      {"type": "buyers", "intensity": 0.7, "label": "Price Sensitive"}
    ]
  }
}
```

### With Factors

```json
{
  "type": "porters_five_forces",
  "title": "SaaS Industry Analysis",
  "data": {
    "industry_name": "Enterprise SaaS",
    "forces": [
      {
        "type": "rivalry",
        "intensity": 0.75,
        "factors": ["Many competitors", "Low switching costs", "Price competition"]
      },
      {
        "type": "new_entrants",
        "intensity": 0.6,
        "factors": ["Low capital requirements", "Cloud infrastructure available"]
      },
      {
        "type": "substitutes",
        "intensity": 0.4,
        "factors": ["Custom development", "Open source alternatives"]
      },
      {
        "type": "suppliers",
        "intensity": 0.3,
        "factors": ["Multiple cloud providers", "Commodity infrastructure"]
      },
      {
        "type": "buyers",
        "intensity": 0.65,
        "factors": ["Price transparency", "Easy comparison", "Annual renewals"]
      }
    ]
  }
}
```

### Tech Industry Example

```json
{
  "type": "porters_five_forces",
  "title": "Smartphone Market",
  "data": {
    "industry_name": "Smartphones",
    "forces": [
      {"type": "rivalry", "intensity": 0.85, "label": "Apple vs Android"},
      {"type": "new_entrants", "intensity": 0.25, "label": "High Barriers"},
      {"type": "substitutes", "intensity": 0.2, "label": "Limited"},
      {"type": "suppliers", "intensity": 0.6, "label": "Chip Makers"},
      {"type": "buyers", "intensity": 0.5, "label": "Brand Loyalty"}
    ]
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [2x2 Matrix](./matrix_2x2.md) - For prioritization
- [Business Model Canvas](./business_model_canvas.md) - For business strategy
- [Value Chain](./value_chain.md) - For operations analysis

## Native Layout

Rivalry is the one solid accent block in the centre (its text in the ink that
reads on the accent). Each of the four forces around it is a pentagon whose
point aims at it: the force's box plus a point on the side facing the centre,
in the force's own fill, stopping 4pt short of the rivalry block. No connector
line is drawn — the shapes carry the direction. Headers are 14pt bold, the
intensity line and factors 12pt; factors are a bulleted list with a hanging
indent in the text ink, and every factor given is drawn.

Boxes are sized from their text. On a region at least 690pt wide the boxes
above and below rivalry are bands — header and intensity on the left, the
factors beside them — so the column of three boxes fits a short content area.
A narrower region (a `shape_grid` cell) stacks the factors under the header.
The gap between rivalry and the forces above and below it is at least 5% of
the height (14pt at least), the room their points need. A cross that
does not fit tightens the top and bottom padding of its boxes (10 / 7 / 5pt)
before any text is shrunk; one that needs a shrink reports
`TEXT_BELOW_READABLE_MIN` with `fix.params.max_items_per_force` and
`max_chars_per_item` measured for the region.
