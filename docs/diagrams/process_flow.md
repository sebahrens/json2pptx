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

**A plain sequence is a band of interlocking arrows.** A horizontal flow of
`step` and `decision` steps whose connections are the generated defaults (no
`connections` key) or an unlabelled chain from each step to the next is drawn
like the `process-flow` pattern: the first step a pentagon, every later step a
chevron tucked round the point before it, each carrying a `01`, `02`, …
numeral over its bold label and its description, all in the accent's light
tint. No connector is drawn — the shapes are the order. A decision stays a
diamond (unnumbered): the solid accent when it is the flow's only decision,
the tint under an accent outline otherwise; the generated `Yes` / `No`
branches are not drawn on a band. Up to four steps in a row are set at 24pt
numerals, 16pt labels and 14pt descriptions; five or six at 20 / 14 / 12pt;
more at 18 / 14 / 12pt. A row holds as many steps as keep every label's
longest word whole (and give a description a 1.6in line); the rest wrap onto
balanced rows (seven steps with descriptions: 4 + 3), and each second row runs
back right to left with its arrows mirrored, starting under the end of the row
above.

**Anything else is a flowchart.** Authored `connections` that branch, skip or
carry a `label` or `style: "dashed"`, a `start` / `end` / `subprocess` step,
`direction: "vertical"`, or a sequence too long for three rows keep boxes and
connectors: steps in the accent's light tint (decisions outlined in the
accent) joined by neutral 1.5pt connectors (`dk1` at 50%) — a connector is
never drawn in the accent.

**No connector crosses a step.** In a horizontal flowchart only a step and its
right-hand neighbour in a row are joined by a straight connector. Every other
connection — a decision's branch to the step after next (the generated `No`
branch included), a loop back, the turn onto the next row — takes a detour
outside the rows: it leaves its source's top or bottom edge, runs along a lane
between the rows (skips forward under a row, loops back over it) and enters
its target's top or bottom edge, with its label beside the first stub.
Detours that would share a stretch of lane get lanes of their own, 0.3in
apart. A flowchart with such connections sets its rows left to right, one
under the other; a plain chain that wraps still snakes. When a detour would
have to pass a whole row, or the rows and lanes do not fit the region, the
flow is drawn in the vertical layout below instead.

In `vertical` mode, step boxes are content-sized and capped at 40% of the
diagram width so the flow retains side lanes. A decision with two or more
outgoing connections places its direct targets on left/right lanes (`Yes` left,
`No` right; otherwise alternating), and connection labels sit beside the route
inside the edge-to-edge gap. If a short frame still forces text or a connection
label into a step, preflight reports `diagram.text_overlap` instead of silently
shipping unreadable 5pt text.

A decision label is never broken inside a word: it is drawn unwrapped, broken
only at spaces, at the largest size in 14–18pt whose lines fit the diamond's
visible width; on a horizontal row the diamond is widened (other steps narrow
first) until that holds, and preflight reports `diagram.text_overlap` when even
14pt cannot fit. In `horizontal` mode a connection label (12pt) sits above its
connector on a background knock-out, clear of the line and arrowhead.

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
