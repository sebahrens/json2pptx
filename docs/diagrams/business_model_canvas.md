# Business Model Canvas

Map out a complete business model using the 9-section canvas framework.

## Type Identifier

`business_model_canvas`

## Use Cases

- Business strategy development
- Startup pitch decks
- Business model innovation
- Strategic planning workshops

## Data Structure

```json
{
  "type": "business_model_canvas",
  "title": "Acme SaaS",
  "data": {
    "boxes": {
      "key_partners": {"items": ["Cloud providers", "System integrators"]},
      "key_activities": {"items": ["Product development", "Customer support"]},
      "key_resources": {"items": ["Engineering team", "Proprietary algorithms"]},
      "value_proposition": {"items": ["Automated workflows", "Time savings"]},
      "customer_relationships": {"items": ["Self-service", "Dedicated support"]},
      "channels": {"items": ["Website", "App stores", "Partners"]},
      "customer_segments": {"items": ["SMBs", "Enterprise", "Startups"]},
      "cost_structure": {"items": ["Infrastructure", "Salaries", "Marketing"]},
      "revenue_streams": {"items": ["Subscriptions", "Usage fees"]}
    }
  }
}
```

## Canvas Sections

| Section Key | Position | Description |
|-------------|----------|-------------|
| `key_partners` | Top-left | External partners and suppliers |
| `key_activities` | Top-left-center | Core activities to deliver value |
| `key_resources` | Bottom-left-center | Assets required for operations |
| `value_proposition` | Center | Value delivered to customers |
| `customer_relationships` | Top-right-center | Types of relationships with customers |
| `channels` | Bottom-right-center | How value is delivered |
| `customer_segments` | Right | Target customer groups |
| `cost_structure` | Bottom-left | Major cost drivers |
| `revenue_streams` | Bottom-right | How revenue is generated |

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `boxes` | `object` | Map of section key to box data |
| `boxes[section].items` | `string[]` | Bullet points for this section |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Canvas title (company/product name) |
| `subtitle` | `string` | - | Subtitle |
| `footnote` | `string` | - | Footnote text |

## Box Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `title` | `string` | Override default section title |
| `icon` | `string` | Custom emoji/icon |
| `color` | `string` | Custom background color |

## Default Icons

Each section has a default icon:
- Key Partners: 🤝
- Key Activities: ⚡
- Key Resources: 🔧
- Value Proposition: 🎁
- Customer Relationships: ❤️
- Channels: 📦
- Customer Segments: 👥
- Cost Structure: 💰
- Revenue Streams: 💵

## Examples

### Tech Startup

```json
{
  "type": "business_model_canvas",
  "title": "AI Writing Assistant",
  "data": {
    "boxes": {
      "key_partners": {
        "items": ["OpenAI", "AWS", "Content agencies"]
      },
      "key_activities": {
        "items": ["AI model fine-tuning", "Platform development", "Content curation"]
      },
      "key_resources": {
        "items": ["ML engineers", "Training data", "GPU infrastructure"]
      },
      "value_proposition": {
        "items": ["10x faster writing", "Consistent brand voice", "SEO optimization"]
      },
      "customer_relationships": {
        "items": ["Freemium self-service", "Enterprise onboarding", "Community forums"]
      },
      "channels": {
        "items": ["Organic search", "Content marketing", "Partner referrals"]
      },
      "customer_segments": {
        "items": ["Content marketers", "Bloggers", "Agencies"]
      },
      "cost_structure": {
        "items": ["API costs", "Engineering salaries", "Cloud hosting"]
      },
      "revenue_streams": {
        "items": ["Monthly subscriptions", "Enterprise contracts", "API access"]
      }
    }
  }
}
```

### E-commerce Business

```json
{
  "type": "business_model_canvas",
  "title": "Online Marketplace",
  "data": {
    "boxes": {
      "key_partners": {"items": ["Payment processors", "Shipping carriers", "Sellers"]},
      "key_activities": {"items": ["Platform maintenance", "Seller onboarding", "Trust & safety"]},
      "key_resources": {"items": ["Technology platform", "Brand reputation", "User data"]},
      "value_proposition": {"items": ["Wide selection", "Competitive prices", "Fast delivery"]},
      "customer_relationships": {"items": ["Automated", "Community reviews", "Customer support"]},
      "channels": {"items": ["Mobile app", "Website", "Email marketing"]},
      "customer_segments": {"items": ["Budget shoppers", "Convenience seekers", "Niche collectors"]},
      "cost_structure": {"items": ["Technology", "Marketing", "Logistics"]},
      "revenue_streams": {"items": ["Transaction fees", "Advertising", "Premium listings"]}
    }
  }
}
```

## Style Options

| Option | Value | Description |
|--------|-------|-------------|
| `color_scheme` | `classic` | Grayscale boxes |
| `color_scheme` | `colorful` | Distinct colors per section |

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Value Chain](./value_chain.md) - For operations focus
- [Porter's Five Forces](./porters_five_forces.md) - For competitive analysis
