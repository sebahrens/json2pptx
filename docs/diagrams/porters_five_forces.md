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

| Field | Type | Description |
|-------|------|-------------|
| `forces` | `object[]` | The five forces data |
| `forces[].type` | `string` | Force type (see below) |
| `forces[].intensity` | `number` | Force strength (0.0 to 1.0) |

## Force Types

| Type | Position | Description |
|------|----------|-------------|
| `rivalry` | Center | Competitive rivalry among existing firms |
| `new_entrants` | Top | Threat of new entrants |
| `substitutes` | Bottom | Threat of substitute products |
| `suppliers` | Left | Bargaining power of suppliers |
| `buyers` | Right | Bargaining power of buyers |

## Intensity Scale

| Value | Meaning | Color |
|-------|---------|-------|
| 0.0 - 0.33 | Low intensity | Green |
| 0.34 - 0.66 | Medium intensity | Yellow |
| 0.67 - 1.0 | High intensity | Red |

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
