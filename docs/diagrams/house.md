# House Diagram

Radial relationship diagram with a central concept and surrounding elements.

## Type Identifier

`house_diagram`

## Use Cases

- McKinsey 7S Framework
- Organizational structure
- Concept relationships
- Hub-and-spoke models

## Data Structure

```json
{
  "type": "house_diagram",
  "title": "McKinsey 7S Framework",
  "data": {
    "center_element": {"label": "Shared Values"},
    "outer_elements": [
      {"label": "Strategy"},
      {"label": "Structure"},
      {"label": "Systems"},
      {"label": "Style"},
      {"label": "Staff"},
      {"label": "Skills"}
    ],
    "show_connectors": true
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `center_element` | `object` | Central hub element |
| `center_element.label` | `string` | Center label |
| `outer_elements` | `object[]` | Surrounding elements (2-12) |
| `outer_elements[].label` | `string` | Element label |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `show_connectors` | `bool` | `true` | Draw lines from center to outer |
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle |
| `footnote` | `string` | - | Footnote text |

## Element Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique identifier |
| `description` | `string` | Additional details |
| `icon` | `string` | Emoji or icon |
| `color` | `string` | Custom color |
| `size` | `number` | Relative size multiplier (1.0 = normal) |

## Examples

### McKinsey 7S Model

```json
{
  "type": "house_diagram",
  "title": "McKinsey 7S Framework",
  "subtitle": "Organizational Analysis",
  "data": {
    "center_element": {
      "label": "Shared Values",
      "description": "Core beliefs and culture"
    },
    "outer_elements": [
      {"label": "Strategy", "description": "Plans for competitive advantage"},
      {"label": "Structure", "description": "Organization hierarchy"},
      {"label": "Systems", "description": "Processes and procedures"},
      {"label": "Style", "description": "Leadership approach"},
      {"label": "Staff", "description": "Employees and HR"},
      {"label": "Skills", "description": "Capabilities and competencies"}
    ],
    "show_connectors": true
  }
}
```

### Product Ecosystem

```json
{
  "type": "house_diagram",
  "title": "Platform Ecosystem",
  "data": {
    "center_element": {
      "label": "Core Platform",
      "icon": "⚡"
    },
    "outer_elements": [
      {"label": "API", "icon": "🔌"},
      {"label": "Mobile App", "icon": "📱"},
      {"label": "Web App", "icon": "🌐"},
      {"label": "Analytics", "icon": "📊"},
      {"label": "Integrations", "icon": "🔗"}
    ]
  }
}
```

### Stakeholder Map

```json
{
  "type": "house_diagram",
  "title": "Project Stakeholders",
  "data": {
    "center_element": {
      "label": "Project",
      "description": "Digital Transformation"
    },
    "outer_elements": [
      {"label": "Executive Sponsor", "size": 1.2},
      {"label": "IT Team"},
      {"label": "Business Users"},
      {"label": "Customers"},
      {"label": "Partners"},
      {"label": "Compliance"}
    ]
  }
}
```

### Competency Model

```json
{
  "type": "house_diagram",
  "title": "Leadership Competencies",
  "data": {
    "center_element": {"label": "Leadership"},
    "outer_elements": [
      {"label": "Vision", "color": "#3B82F6"},
      {"label": "Communication", "color": "#10B981"},
      {"label": "Execution", "color": "#F59E0B"},
      {"label": "Empathy", "color": "#EC4899"},
      {"label": "Decision Making", "color": "#8B5CF6"},
      {"label": "Accountability", "color": "#EF4444"}
    ]
  }
}
```

### Minimal Diagram

```json
{
  "type": "house_diagram",
  "title": "Core Values",
  "data": {
    "center_element": {"label": "Mission"},
    "outer_elements": [
      {"label": "Integrity"},
      {"label": "Innovation"},
      {"label": "Excellence"}
    ],
    "show_connectors": true
  }
}
```

## Layout Notes

- Elements are distributed evenly around the center
- First element starts at the top (12 o'clock)
- Elements proceed clockwise
- Connectors are drawn as straight lines from center to each outer element
- Optimal number of outer elements: 4-8

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Process Flow](./process_flow.md) - For sequential relationships
- [Business Model Canvas](./business_model_canvas.md) - For structured frameworks
