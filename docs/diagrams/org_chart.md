# Org Chart

Display hierarchical organizational structures as a tree of connected nodes.

## Type Identifier

`org_chart`

**Aliases:** `orgchart`, `org`

## Use Cases

- Company organizational structure
- Team reporting hierarchies
- Department breakdowns
- Project governance structures

## Data Structure

```json
{
  "type": "org_chart",
  "title": "Engineering Organization",
  "data": {
    "root": {
      "name": "CTO",
      "title": "Jane Smith",
      "children": [
        {
          "name": "VP Engineering",
          "title": "Alice Chen",
          "children": [
            {"name": "Backend Lead", "title": "Bob Jones"},
            {"name": "Frontend Lead", "title": "Carol Wu"}
          ]
        },
        {
          "name": "VP Infrastructure",
          "title": "Dave Kim",
          "children": [
            {"name": "SRE Lead", "title": "Eve Park"},
            {"name": "Platform Lead", "title": "Frank Lee"}
          ]
        }
      ]
    }
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `root` | `object` | Root node of the tree |
| `root.name` | `string` | Node label (role or name) |

Alternatively, provide `name` or `title` at the top level of data (treated as root).

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle below title |
| `root.title` | `string` | - | Secondary label (e.g., person name) |
| `root.children` | `object[]` | - | Child nodes (recursive structure) |
| `node_width` | `number` | - | Width of each node |
| `node_height` | `number` | - | Height of each node |
| `horizontal_gap` | `number` | - | Spacing between sibling nodes |
| `vertical_gap` | `number` | - | Spacing between levels |
| `corner_radius` | `number` | - | Rounded corners on nodes |
| `max_visible_siblings` | `number` | - | Max siblings before "..." collapse |

## Examples

### Simple Team

```json
{
  "type": "org_chart",
  "title": "Product Team",
  "data": {
    "root": {
      "name": "Product Manager",
      "children": [
        {"name": "Designer"},
        {"name": "Engineer 1"},
        {"name": "Engineer 2"},
        {"name": "QA"}
      ]
    }
  }
}
```

### Deep Hierarchy

```json
{
  "type": "org_chart",
  "title": "Company Structure",
  "data": {
    "root": {
      "name": "CEO",
      "children": [
        {
          "name": "COO",
          "children": [
            {"name": "Operations"},
            {"name": "HR"}
          ]
        },
        {
          "name": "CFO",
          "children": [
            {"name": "Finance"},
            {"name": "Legal"}
          ]
        }
      ]
    }
  }
}
```

## See Also

- [Process Flow](./process_flow.md) - For workflow diagrams
- [Pyramid](./pyramid.md) - For hierarchical level visualization
