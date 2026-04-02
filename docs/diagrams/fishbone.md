# Fishbone Diagram

Display cause-and-effect relationships branching from a central problem (Ishikawa diagram).

## Type Identifier

`fishbone`

## Use Cases

- Root cause analysis
- Quality improvement discussions
- Problem-solving workshops
- Process failure investigation

## Data Structure

```json
{
  "type": "fishbone",
  "title": "Delivery Delays Root Cause",
  "data": {
    "effect": "Late Deliveries",
    "categories": [
      {
        "name": "People",
        "causes": ["Staff shortage", "Insufficient training", "High turnover"]
      },
      {
        "name": "Process",
        "causes": ["Manual approvals", "No priority system", "Batch processing"]
      },
      {
        "name": "Technology",
        "causes": ["Legacy systems", "No automation", "Poor integration"]
      },
      {
        "name": "Materials",
        "causes": ["Supplier delays", "Quality issues"]
      }
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `effect` | `string` | Problem/effect at the arrow head. Alias: `problem` |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle below title |
| `categories` | `object[]` | - | Cause categories branching from spine |
| `categories[].name` | `string` | - | Category label (bone label) |
| `categories[].causes` | `string[]` | - | Individual causes in this category |

## Examples

### Manufacturing Defects

```json
{
  "type": "fishbone",
  "title": "Product Defect Analysis",
  "data": {
    "effect": "High Defect Rate",
    "categories": [
      {"name": "Method", "causes": ["Outdated SOP", "No inspection step"]},
      {"name": "Machine", "causes": ["Worn tooling", "Calibration drift"]},
      {"name": "Material", "causes": ["Supplier variability", "Wrong grade"]},
      {"name": "Manpower", "causes": ["New operators", "Night shift fatigue"]},
      {"name": "Measurement", "causes": ["Gauge R&R failure"]},
      {"name": "Environment", "causes": ["Humidity", "Temperature swings"]}
    ]
  }
}
```

### Simple

```json
{
  "type": "fishbone",
  "title": "Customer Churn",
  "data": {
    "effect": "Increasing Churn Rate",
    "categories": [
      {"name": "Product", "causes": ["Missing features", "Bugs"]},
      {"name": "Support", "causes": ["Slow response", "Knowledge gaps"]},
      {"name": "Pricing", "causes": ["Competitor undercut", "No discounts"]}
    ]
  }
}
```

## See Also

- [Process Flow](./process_flow.md) - For workflow diagrams
- [SWOT](./swot.md) - For strategic factor analysis
