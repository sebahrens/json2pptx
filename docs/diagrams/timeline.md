# Timeline

Visualize project schedules with activities, milestones, and phases.

## Type Identifier

`timeline`

## Use Cases

- Project roadmaps
- Product launch schedules
- Historical event visualization
- Sprint planning views

## Data Structure

```json
{
  "type": "timeline",
  "title": "Project Roadmap",
  "data": {
    "activities": [
      {"id": "1", "label": "Planning", "type": "activity", "start_date": "2024-01-01", "end_date": "2024-02-15"},
      {"id": "2", "label": "Development", "type": "activity", "start_date": "2024-02-15", "end_date": "2024-06-01"},
      {"id": "3", "label": "Testing", "type": "activity", "start_date": "2024-05-01", "end_date": "2024-06-15"},
      {"id": "4", "label": "Beta Launch", "type": "milestone", "date": "2024-06-15"},
      {"id": "5", "label": "GA Release", "type": "milestone", "date": "2024-07-01"}
    ]
  }
}
```

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `activities` | `object[]` | Timeline items |
| `activities[].id` | `string` | Unique identifier |
| `activities[].label` | `string` | Display text |
| `activities[].type` | `string` | Item type: `activity`, `milestone`, `phase` |

## Activity Types

| Type | Description | Visual |
|------|-------------|--------|
| `activity` | Duration-based task | Horizontal bar |
| `milestone` | Point-in-time event | Diamond marker |
| `phase` | Background grouping | Colored band |

## Date Fields

For `activity` and `phase` types:
- `start_date` - Start date (ISO 8601: `YYYY-MM-DD`)
- `end_date` - End date (ISO 8601: `YYYY-MM-DD`)

For `milestone` type:
- `date` - Milestone date (ISO 8601: `YYYY-MM-DD`)

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Diagram title |
| `subtitle` | `string` | - | Subtitle |
| `show_today` | `bool` | `false` | Draw "today" line |
| `today_label` | `string` | `Today` | Label for today line |
| `time_unit` | `string` | auto | Display unit: `day`, `week`, `month`, `quarter`, `year` |

## Activity Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | `string` | Additional text |
| `icon` | `string` | Emoji or icon |
| `color` | `string` | Custom hex color |
| `progress` | `number` | Completion % (0-100) |
| `row` | `number` | Manual row assignment |

## Examples

### Product Launch Timeline

```json
{
  "type": "timeline",
  "title": "2024 Product Launch",
  "data": {
    "activities": [
      {"id": "phase1", "label": "Q1", "type": "phase", "start_date": "2024-01-01", "end_date": "2024-03-31"},
      {"id": "phase2", "label": "Q2", "type": "phase", "start_date": "2024-04-01", "end_date": "2024-06-30"},
      {"id": "design", "label": "Design", "type": "activity", "start_date": "2024-01-15", "end_date": "2024-02-28"},
      {"id": "dev", "label": "Development", "type": "activity", "start_date": "2024-02-15", "end_date": "2024-05-15"},
      {"id": "test", "label": "QA Testing", "type": "activity", "start_date": "2024-04-01", "end_date": "2024-05-31"},
      {"id": "m1", "label": "Alpha", "type": "milestone", "date": "2024-04-15"},
      {"id": "m2", "label": "Beta", "type": "milestone", "date": "2024-05-15"},
      {"id": "m3", "label": "Launch", "type": "milestone", "date": "2024-06-01"}
    ],
    "show_today": true
  }
}
```

### Sprint Timeline with Progress

```json
{
  "type": "timeline",
  "title": "Sprint 12",
  "data": {
    "activities": [
      {"id": "1", "label": "Auth System", "type": "activity", "start_date": "2024-03-01", "end_date": "2024-03-08", "progress": 100},
      {"id": "2", "label": "Dashboard UI", "type": "activity", "start_date": "2024-03-04", "end_date": "2024-03-12", "progress": 75},
      {"id": "3", "label": "API Integration", "type": "activity", "start_date": "2024-03-08", "end_date": "2024-03-15", "progress": 30},
      {"id": "4", "label": "Sprint Review", "type": "milestone", "date": "2024-03-15"}
    ],
    "time_unit": "day"
  }
}
```

## Output Formats

- SVG (default)
- PNG (requires `output.format: "png"`)
- PDF (requires `output.format: "pdf"`)

## See Also

- [Process Flow](./process_flow.md) - For workflows
- [Value Chain](./value_chain.md) - For operational flow
