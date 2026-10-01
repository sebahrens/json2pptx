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

A flat `nodes` array of `{id, name, title, parent}` is also accepted and is
converted to the tree above. Box text comes only from `name` and `title` — a
node written with `label` draws as an empty box. A `parent` that matches no
`id`, a duplicate `id`, a node that is its own parent, a parent cycle, or a
node with neither `name` nor `title` emits `diagram.org_chart_nodes_invalid`
(warning, fix kind `replace_value`, `params.issues` lists each problem); the
unresolvable node is drawn under the top node rather than dropped.

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
| `max_visible_siblings` | `number` | `9` | Max siblings before "+N more" collapse |
| `accent_strategy` | `string` | the deck's | `rotate` / `section-keyed` give each level its own accent; anything else (the default `primary`) draws every level in accent1 and lighter tints of it. In a deck the generator passes the deck-level `accent_strategy` here unless the diagram sets one |

## Overflow

Two independent mechanisms shrink a tree that does not fit, and both report what
they hid:

- **Sibling collapse.** More than `max_visible_siblings` children under one
  parent: the excess becomes a single "+N more" box (`chart.overflow_suppressed`).
- **Depth pruning.** When the boxes would fall below the legible minimum, the
  deepest level is removed entirely and its parent gains a `+N reports` line
  **under its own title**, which is preserved (`diagram.org_chart_depth_pruned`
  names how many nodes went). A 25-node, three-level tree renders as 7 boxes —
  so `max_nodes: 50` is a ceiling, not a guarantee: a deep tree hits the depth
  prune long before it (go-slide-creator-s5ur).

Split a large org across slides rather than relying on either.

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
