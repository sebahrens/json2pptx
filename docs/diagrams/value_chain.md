# Value Chain

Visualize Porter's Value Chain with primary and support activities.

## Type Identifier

`value_chain`

## Use Cases

- Operations analysis
- Competitive advantage identification
- Process optimization
- Strategic cost analysis

## Data Structure

```json
{
  "type": "value_chain",
  "title": "Manufacturing Value Chain",
  "data": {
    "primary_activities": [
      {"label": "Inbound Logistics", "items": ["Warehousing", "Inventory control"]},
      {"label": "Operations", "items": ["Manufacturing", "Quality control"]},
      {"label": "Outbound Logistics", "items": ["Distribution", "Order fulfillment"]},
      {"label": "Marketing & Sales", "items": ["Advertising", "Sales force"]},
      {"label": "Service", "items": ["Installation", "Repair"]}
    ],
    "support_activities": [
      {"label": "Firm Infrastructure", "items": ["Finance", "Legal", "Planning"]},
      {"label": "HR Management", "items": ["Recruiting", "Training"]},
      {"label": "Technology", "items": ["R&D", "Process automation"]},
      {"label": "Procurement", "items": ["Purchasing", "Supplier relations"]}
    ]
  }
}
```

## Standard Activities

### Primary Activities (left to right)
1. Inbound Logistics - Receiving and warehousing
2. Operations - Transforming inputs to outputs
3. Outbound Logistics - Distribution
4. Marketing & Sales - Promotion and selling
5. Service - Post-sale support

### Support Activities (top to bottom)
1. Firm Infrastructure - General management
2. Human Resource Management - Personnel
3. Technology Development - R&D and IT
4. Procurement - Purchasing

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `primary_activities` | `object[]` | Main value-creating activities |
| `primary_activities[].label` | `string` | Activity name |
| `support_activities` | `object[]` | Supporting activities |
| `support_activities[].label` | `string` | Activity name |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle |
| `margin_label` | `string` | `Margin` | Label for profit area |
| `footnote` | `string` | - | Footnote text |

## Activity Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique identifier |
| `items` | `string[]` | Sub-activities or details |
| `description` | `string` | Additional details |
| `icon` | `string` | Emoji or icon |
| `color` | `string` | Custom color |

## Default Icons

- Inbound Logistics: 📥
- Operations: ⚙️
- Outbound Logistics: 📤
- Marketing & Sales: 📢
- Service: 🛠️
- Firm Infrastructure: 🏢
- HR Management: 👥
- Technology: 💻
- Procurement: 🛒

## Examples

### Software Company

```json
{
  "type": "value_chain",
  "title": "SaaS Value Chain",
  "data": {
    "primary_activities": [
      {"label": "Data Acquisition", "items": ["APIs", "User uploads", "Integrations"]},
      {"label": "Processing", "items": ["ML pipelines", "Data transformation"]},
      {"label": "Delivery", "items": ["CDN", "API endpoints", "Dashboards"]},
      {"label": "Marketing", "items": ["Content", "SEO", "Paid ads"]},
      {"label": "Customer Success", "items": ["Onboarding", "Training", "Support"]}
    ],
    "support_activities": [
      {"label": "Infrastructure", "items": ["Finance", "Legal", "Security"]},
      {"label": "People", "items": ["Engineering hiring", "Culture"]},
      {"label": "Technology", "items": ["Platform", "DevOps", "Architecture"]},
      {"label": "Vendor Management", "items": ["Cloud costs", "Tools"]}
    ],
    "margin_label": "Profit"
  }
}
```

### Retail Business

```json
{
  "type": "value_chain",
  "title": "Retail Value Chain",
  "data": {
    "primary_activities": [
      {"label": "Sourcing", "items": ["Supplier negotiation", "Import logistics"]},
      {"label": "Store Operations", "items": ["Merchandising", "Inventory"]},
      {"label": "Distribution", "items": ["DC operations", "Store delivery"]},
      {"label": "Sales & Marketing", "items": ["Promotions", "Store staff"]},
      {"label": "After-Sales", "items": ["Returns", "Loyalty programs"]}
    ],
    "support_activities": [
      {"label": "Corporate", "items": ["Finance", "Real estate"]},
      {"label": "HR", "items": ["Hiring", "Training", "Retention"]},
      {"label": "IT Systems", "items": ["POS", "E-commerce", "Analytics"]},
      {"label": "Buying", "items": ["Category management", "Trend analysis"]}
    ]
  }
}
```

### Healthcare Provider

```json
{
  "type": "value_chain",
  "title": "Hospital Value Chain",
  "data": {
    "primary_activities": [
      {"label": "Patient Intake", "items": ["Admissions", "Records"]},
      {"label": "Diagnosis", "items": ["Labs", "Imaging", "Consults"]},
      {"label": "Treatment", "items": ["Surgery", "Pharmacy", "Nursing"]},
      {"label": "Discharge", "items": ["Care plans", "Billing"]},
      {"label": "Follow-up", "items": ["Rehab", "Monitoring"]}
    ],
    "support_activities": [
      {"label": "Administration", "items": ["Compliance", "Finance"]},
      {"label": "Staffing", "items": ["Credentialing", "Scheduling"]},
      {"label": "Medical Tech", "items": ["EMR", "Equipment"]},
      {"label": "Supplies", "items": ["Medical supplies", "Pharmaceuticals"]}
    ]
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Business Model Canvas](./business_model_canvas.md) - For complete business model
- [Process Flow](./process_flow.md) - For detailed workflows
