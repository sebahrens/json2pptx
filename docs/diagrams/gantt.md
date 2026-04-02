# Gantt Chart

Visualize project schedules with task bars and milestones on a timeline.

## Type Identifier

`gantt`

## Use Cases

- Project planning and scheduling
- Resource allocation timelines
- Sprint planning visualization
- Release roadmaps

## Data Structure

```json
{
  "type": "gantt",
  "title": "Product Launch Plan",
  "data": {
    "tasks": [
      {"name": "Research", "start": "2024-01-01", "end": "2024-02-15"},
      {"name": "Design", "start": "2024-02-01", "end": "2024-03-15"},
      {"name": "Development", "start": "2024-03-01", "end": "2024-06-01"},
      {"name": "Testing", "start": "2024-05-15", "end": "2024-06-15"},
      {"name": "Deployment", "start": "2024-06-15", "end": "2024-07-01"}
    ],
    "milestones": [
      {"name": "Beta Release", "date": "2024-05-01"},
      {"name": "GA Launch", "date": "2024-07-01"}
    ]
  }
}
```

## Required Fields

At least one of `tasks` or `milestones` must be provided:

| Field | Type | Description |
|-------|------|-------------|
| `tasks` | `object[]` | Task bars on the timeline |
| `tasks[].name` | `string` | Task label |
| `tasks[].start` | `string` | Start date (YYYY-MM-DD) |
| `tasks[].end` | `string` | End date (YYYY-MM-DD) |

| Field | Type | Description |
|-------|------|-------------|
| `milestones` | `object[]` | Diamond markers on the timeline |
| `milestones[].name` | `string` | Milestone label |
| `milestones[].date` | `string` | Date (YYYY-MM-DD) |

## Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `title` | `string` | - | Chart title |
| `subtitle` | `string` | - | Subtitle below title |
| `tasks[].progress` | `number` | - | Completion (0-1) |
| `tasks[].color` | `string` | - | Custom hex color |
| `tasks[].category` | `string` | - | Group label |
| `tasks[].swimlane` | `string` | - | Swimlane assignment |
| `time_unit` | `string` | - | Granularity: `"day"`, `"week"`, `"month"` |
| `show_progress` | `bool` | `false` | Display progress bars |
| `show_grid` | `bool` | `false` | Show time grid |
| `label_width` | `number` | - | Width for task labels |

## Examples

### With Progress

```json
{
  "type": "gantt",
  "title": "Sprint Progress",
  "data": {
    "tasks": [
      {"name": "Auth module", "start": "2024-03-01", "end": "2024-03-08", "progress": 1.0},
      {"name": "API endpoints", "start": "2024-03-04", "end": "2024-03-12", "progress": 0.7},
      {"name": "UI components", "start": "2024-03-06", "end": "2024-03-15", "progress": 0.3},
      {"name": "Integration tests", "start": "2024-03-11", "end": "2024-03-15", "progress": 0.0}
    ],
    "show_progress": true
  }
}
```

### Milestones Only

```json
{
  "type": "gantt",
  "title": "Key Dates",
  "data": {
    "milestones": [
      {"name": "Kickoff", "date": "2024-01-15"},
      {"name": "Design Review", "date": "2024-03-01"},
      {"name": "Code Freeze", "date": "2024-05-15"},
      {"name": "Launch", "date": "2024-06-01"}
    ]
  }
}
```

## See Also

- [Timeline](./timeline.md) - For simpler activity timelines
- [Process Flow](./process_flow.md) - For workflow diagrams
