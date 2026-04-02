# Process Flow

Document workflows with steps, decisions, and connections.

## Type Identifier

`process_flow`

## Use Cases

- Business process documentation
- System workflows
- Decision trees
- User journey mapping

## Data Structure

```json
{
  "type": "process_flow",
  "title": "Order Processing",
  "data": {
    "steps": [
      {"id": "start", "label": "Order Received", "type": "start"},
      {"id": "check", "label": "Check Inventory", "type": "decision"},
      {"id": "ship", "label": "Ship Order", "type": "step"},
      {"id": "backorder", "label": "Create Backorder", "type": "step"},
      {"id": "notify", "label": "Notify Customer", "type": "step"},
      {"id": "end", "label": "Complete", "type": "end"}
    ],
    "connections": [
      {"from": "start", "to": "check"},
      {"from": "check", "to": "ship", "label": "In Stock"},
      {"from": "check", "to": "backorder", "label": "Out of Stock"},
      {"from": "ship", "to": "notify"},
      {"from": "backorder", "to": "notify"},
      {"from": "notify", "to": "end"}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `steps` | `object[]` | Process steps |
| `steps[].id` | `string` | Unique step identifier |
| `steps[].label` | `string` | Display text |
| `steps[].type` | `string` | Step type |

## Step Types

| Type | Description | Shape |
|------|-------------|-------|
| `start` | Process start point | Rounded rectangle |
| `end` | Process end point | Rounded rectangle |
| `step` | Standard process step | Rectangle |
| `decision` | Decision/branching point | Diamond |
| `subprocess` | Reference to subprocess | Rectangle with bars |

## Connection Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from` | `string` | Yes | Source step ID |
| `to` | `string` | Yes | Target step ID |
| `label` | `string` | No | Connection label (e.g., "Yes", "No") |
| `style` | `string` | No | Line style: `solid`, `dashed`, `dotted` |
| `color` | `string` | No | Custom hex color |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle |
| `direction` | `string` | `horizontal` | Flow direction: `horizontal`, `vertical` |
| `footnote` | `string` | - | Footnote text |

## Step Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | `string` | Additional details |
| `icon` | `string` | Emoji or icon |
| `color` | `string` | Custom hex color |

## Examples

### Linear Process

```json
{
  "type": "process_flow",
  "title": "Simple Workflow",
  "data": {
    "steps": [
      {"id": "1", "label": "Start", "type": "start"},
      {"id": "2", "label": "Step 1", "type": "step"},
      {"id": "3", "label": "Step 2", "type": "step"},
      {"id": "4", "label": "End", "type": "end"}
    ]
  }
}
```

Without `connections`, steps are connected sequentially.

### Decision Flow

```json
{
  "type": "process_flow",
  "title": "Approval Process",
  "data": {
    "steps": [
      {"id": "submit", "label": "Submit Request", "type": "start"},
      {"id": "review", "label": "Manager Review", "type": "decision"},
      {"id": "approve", "label": "Approved", "type": "step"},
      {"id": "reject", "label": "Rejected", "type": "step"},
      {"id": "done", "label": "Complete", "type": "end"}
    ],
    "connections": [
      {"from": "submit", "to": "review"},
      {"from": "review", "to": "approve", "label": "Yes"},
      {"from": "review", "to": "reject", "label": "No"},
      {"from": "approve", "to": "done"},
      {"from": "reject", "to": "done"}
    ],
    "direction": "vertical"
  }
}
```

### Support Ticket Flow

```json
{
  "type": "process_flow",
  "title": "Support Ticket Workflow",
  "data": {
    "steps": [
      {"id": "new", "label": "New Ticket", "type": "start", "icon": "📩"},
      {"id": "triage", "label": "Triage", "type": "decision"},
      {"id": "l1", "label": "L1 Support", "type": "step"},
      {"id": "l2", "label": "L2 Support", "type": "step"},
      {"id": "resolve", "label": "Resolve", "type": "step"},
      {"id": "close", "label": "Closed", "type": "end"}
    ],
    "connections": [
      {"from": "new", "to": "triage"},
      {"from": "triage", "to": "l1", "label": "Simple"},
      {"from": "triage", "to": "l2", "label": "Complex"},
      {"from": "l1", "to": "resolve"},
      {"from": "l2", "to": "resolve"},
      {"from": "resolve", "to": "close"}
    ]
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Timeline](./timeline.md) - For schedules
- [Value Chain](./value_chain.md) - For operational flows
