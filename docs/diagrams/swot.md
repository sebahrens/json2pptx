# SWOT Analysis

Display a four-quadrant Strengths, Weaknesses, Opportunities, and Threats framework.

## Type Identifier

`swot`

## Use Cases

- Strategic planning workshops
- Competitive analysis presentations
- Business case evaluations
- Product positioning reviews

## Data Structure

```json
{
  "type": "swot",
  "title": "Market Entry SWOT",
  "data": {
    "strengths": ["Strong brand", "Experienced team", "Patented technology"],
    "weaknesses": ["Limited funding", "Small market presence", "No local partners"],
    "opportunities": ["Growing market", "Regulatory changes", "Digital transformation"],
    "threats": ["Established competitors", "Price pressure", "Economic uncertainty"]
  }
}
```

## Required Fields

At least one quadrant must be provided:

| Field | Type | Description |
|-------|------|-------------|
| `strengths` | `string[]` | Internal positive factors |
| `weaknesses` | `string[]` | Internal negative factors |
| `opportunities` | `string[]` | External positive factors |
| `threats` | `string[]` | External negative factors |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle below title |
| `gap` | `number` | - | Spacing between quadrants |
| `quadrant_opacity` | `number` | - | Fill opacity (0-1) |
| `corner_radius` | `number` | - | Rounded corner radius |

## Examples

### Product Launch

```json
{
  "type": "swot",
  "title": "Product Launch Assessment",
  "data": {
    "strengths": ["Unique features", "First-mover advantage", "Strong engineering"],
    "weaknesses": ["No brand awareness", "Limited support staff"],
    "opportunities": ["Untapped market segment", "Partnership potential"],
    "threats": ["Fast followers", "Regulatory hurdles"]
  }
}
```

## See Also

- [PESTEL](./pestel.md) - For macro-environmental analysis
- [Porter's Five Forces](./porters_five_forces.md) - For industry competitive analysis
- [2x2 Matrix](./matrix_2x2.md) - For custom two-dimensional frameworks
