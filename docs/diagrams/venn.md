# Venn Diagram

Show overlapping relationships between 2-3 sets with labeled intersections.

## Type Identifier

`venn`

## Use Cases

- Skill overlap between teams
- Market segment intersections
- Feature comparison between products
- Audience overlap analysis

## Data Structure

```json
{
  "type": "venn",
  "title": "Team Skills Overlap",
  "data": {
    "circles": [
      {"label": "Engineering", "items": ["Go", "Python", "Docker"]},
      {"label": "Data Science", "items": ["ML", "Statistics", "R"]},
      {"label": "DevOps", "items": ["Terraform", "K8s", "CI/CD"]}
    ],
    "intersections": {
      "ab": {"label": "Eng + DS", "items": ["Python", "SQL"]},
      "bc": {"label": "DS + DevOps", "items": ["Airflow"]},
      "ac": {"label": "Eng + DevOps", "items": ["Docker", "Git"]},
      "abc": {"label": "All", "items": ["Linux"]}
    }
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `circles` | `object[]` | 2-3 circles. Alias: `sets` |
| `circles[].label` | `string` | Circle label |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle below title |
| `circles[].items` | `string[]` | - | Items unique to this circle |
| `intersections` | `object` | - | Intersection regions (keys: `ab`, `ac`, `bc`, `abc`) |
| `intersections.*.label` | `string` | - | Intersection label |
| `intersections.*.items` | `string[]` | - | Items in this intersection |
| `circle_opacity` | `number` | - | Fill opacity (0-1) |
| `stroke_width` | `number` | - | Circle border width |
| `overlap_ratio` | `number` | - | How much circles overlap (0-1) |

## Examples

### Two-Circle Venn

```json
{
  "type": "venn",
  "title": "Product Comparison",
  "data": {
    "circles": [
      {"label": "Product A", "items": ["Feature 1", "Feature 2"]},
      {"label": "Product B", "items": ["Feature 3", "Feature 4"]}
    ],
    "intersections": {
      "ab": {"label": "Shared", "items": ["Core API", "SSO"]}
    }
  }
}
```

### Simple Labels Only

```json
{
  "type": "venn",
  "title": "Market Segments",
  "data": {
    "circles": [
      {"label": "Enterprise"},
      {"label": "Mid-Market"},
      {"label": "SMB"}
    ]
  }
}
```

## See Also

- [2x2 Matrix](./matrix_2x2.md) - For two-dimensional positioning
- [SWOT](./swot.md) - For structured factor analysis
