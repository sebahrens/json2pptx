# PESTEL Analysis

Display the six macro-environmental factors: Political, Economic, Social, Technological, Environmental, and Legal.

## Type Identifier

`pestel`

## Use Cases

- Macro-environmental scanning
- Market entry analysis
- Strategic planning context
- Risk assessment frameworks

## Data Structure

```json
{
  "type": "pestel",
  "title": "Market Entry Analysis",
  "data": {
    "political": ["Stable government", "Trade agreements", "Tax incentives"],
    "economic": ["GDP growth 3%", "Low inflation", "Strong currency"],
    "social": ["Young population", "Rising middle class", "Digital adoption"],
    "technological": ["5G rollout", "Cloud infrastructure", "AI readiness"],
    "environmental": ["ESG regulations", "Carbon targets", "Green subsidies"],
    "legal": ["IP protection", "Data privacy laws", "Labor regulations"]
  }
}
```

## Required Fields

Use individual category keys or a segments array:

### Individual Keys

| Field | Type | Description |
|-------|------|-------------|
| `political` | `string[]` | Political factors |
| `economic` | `string[]` | Economic factors |
| `social` | `string[]` | Social factors |
| `technological` | `string[]` | Technological factors |
| `environmental` | `string[]` | Environmental factors |
| `legal` | `string[]` | Legal factors |

### Segments Array (Alternative)

| Field | Type | Description |
|-------|------|-------------|
| `segments` | `object[]` | Segments with `name` and `items`. Alias: `factors` |
| `segments[].name` | `string` | Segment label |
| `segments[].items` | `string[]` | Items in this segment |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle below title |

## Examples

### Using Segments Array

```json
{
  "type": "pestel",
  "title": "Industry Outlook",
  "data": {
    "segments": [
      {"name": "Political", "items": ["Election year", "Trade policy shifts"]},
      {"name": "Economic", "items": ["Recession risk", "Interest rate hikes"]},
      {"name": "Social", "items": ["Remote work trend", "Skills shortage"]},
      {"name": "Technological", "items": ["AI disruption", "Cybersecurity threats"]},
      {"name": "Environmental", "items": ["Net-zero mandates"]},
      {"name": "Legal", "items": ["GDPR enforcement", "Antitrust scrutiny"]}
    ]
  }
}
```

## See Also

- [SWOT](./swot.md) - For internal/external factor analysis
- [Porter's Five Forces](./porters_five_forces.md) - For industry competitive analysis
