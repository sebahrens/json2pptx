# Examples

Ready-to-run input files for `json2pptx`. This directory contains both **Markdown** and **JSON** examples.

## JSON Examples

JSON input mode provides direct, programmatic control over slide layout and content placement. Each slide specifies an exact `layout_id` and maps content to specific `placeholder_id` values from the template.

| File | Description | Template |
|------|-------------|----------|
| `basic-deck.json` | Text and bullet slides: title, agenda, content, two-column, section divider, closing | midnight-blue |
| `charts.json` | Chart types: bar, line, pie, donut, area, funnel | forest-green |
| `diagrams.json` | Advanced chart types: waterfall, radar, gauge, treemap, stacked bar | warm-coral |
| `full-showcase.json` | All content types combined in a product launch strategy deck | midnight-blue |
| `patterns-smoke.json` | Pattern library smoke test: one slide per v1 pattern (kpi-3up, kpi-4up, bmc-canvas, matrix-2x2, timeline-horizontal, card-grid, icon-row, comparison-2col) | midnight-blue |

### Running a JSON example

Generate a PPTX file:

```bash
json2pptx generate -json examples/basic-deck.json -output ./output
```

Validate without generating (dry-run):

```bash
json2pptx generate -json examples/basic-deck.json -n
```

Read from stdin:

```bash
cat examples/charts.json | json2pptx generate -json - -output ./output
```

Write structured output as JSON:

```bash
json2pptx generate -json examples/basic-deck.json -json-output result.json
```

### JSON input structure

```json
{
  "template": "midnight-blue",
  "output_filename": "my-deck.pptx",
  "slides": [
    {
      "layout_id": "slideLayout2",
      "content": [
        {
          "placeholder_id": "title",
          "type": "text",
          "text_value": "Slide Title"
        },
        {
          "placeholder_id": "body",
          "type": "bullets",
          "bullets_value": ["Point one", "Point two"]
        }
      ]
    }
  ]
}
```

### Available templates

The built-in templates are: `midnight-blue`, `forest-green`, `warm-coral`.

List all available templates (including any installed locally):

```bash
json2pptx skill-info --mode list
```

### Layout reference

Each built-in template provides six layouts. Use the **portable placeholder IDs** (not raw template names) for cross-template compatibility:

| Layout ID | Name | Portable Placeholder IDs |
|-----------|------|--------------------------|
| `slideLayout1` | Title Slide | `title` (center title), `subtitle` (subtitle) |
| `slideLayout2` | One Content | `title` (title), `body` (body) |
| `slideLayout3` | Two Content | `title` (title), `body` (left), `body_2` (right) |
| `slideLayout4` | Section Divider | `title` (section title) |
| `slideLayout5` | Closing | `title` (closing text), `subtitle` (subtitle) |
| `slideLayout6` | Blank | (no placeholders) |

### Content types

Each content item uses a **typed value field** matching its `type` discriminator:

| Type | Typed field | Value format | Example |
|------|------------|-------------|---------|
| `text` | `text_value` | String | `"Hello World"` |
| `bullets` | `bullets_value` | Array of strings | `["Point 1", "Point 2"]` |
| `image` | `image_value` | Object with `path` and `alt` | `{"path": "photo.png", "alt": "Description"}` |
| `chart` | `chart_value` | Object with `type`, `title`, `data` | `{"type": "bar", "title": "Sales", "data": [...]}` |
| `table` | `table_value` | Object with `headers` and `rows` | `{"headers": ["A", "B"], "rows": [["1", "2"]]}` |
| `diagram` | `diagram_value` | Object with `type` and type-specific fields | `{"type": "timeline", "events": [...]}` |
| `body_and_bullets` | `body_and_bullets_value` | Object with `body` and `bullets` | `{"body": "Intro text", "bullets": ["A", "B"]}` |
| `bullet_groups` | `bullet_groups_value` | Object with `groups` array | `{"groups": [{"heading": "H", "bullets": ["A"]}]}` |

Supported chart types: `bar`, `line`, `pie`, `donut`, `area`, `radar`, `scatter`, `stacked_bar`, `waterfall`, `funnel`, `gauge`, `treemap`.

## Diagram Examples

For individual diagram type examples, see the `diagrams/` directory which contains JSON examples for each supported diagram type (timeline, SWOT, process flow, etc.).

See [docs/diagrams/README.md](../docs/diagrams/README.md) for the full diagram gallery and decision tree.

## Compatibility: legacy authoring form

> **Do not use for new decks.** The legacy form is accepted for backward compatibility only.

Older examples used an untyped `value` field and raw OOXML placeholder names. The parser still accepts this form, but validation will emit an informational `legacy_authoring_form` finding with a `rewrite_field` fix suggestion.

```json
{
  "placeholder_id": "Title 1",
  "type": "text",
  "value": "Slide Title"
}
```

The canonical equivalent is:

```json
{
  "placeholder_id": "title",
  "type": "text",
  "text_value": "Slide Title"
}
```

Raw OOXML names (`Title 1`, `Content Placeholder 2`, `Subtitle 2`, `Text Placeholder 1`) resolve via semantic fallback but are not portable across templates. Portable IDs (`title`, `subtitle`, `body`, `body_2`) resolve at the exact tier and work across all templates.
