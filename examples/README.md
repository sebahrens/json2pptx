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
          "placeholder_id": "Title 1",
          "type": "text",
          "value": "Slide Title"
        },
        {
          "placeholder_id": "Content Placeholder 2",
          "type": "bullets",
          "value": ["Point one", "Point two"]
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
json2pptx list-templates
```

### Layout reference

Each built-in template provides six layouts:

| Layout ID | Name | Placeholders |
|-----------|------|-------------|
| `slideLayout1` | Title Slide | `Title 1` (center title), `Subtitle 2` (subtitle) |
| `slideLayout2` | One Content | `Title 1` (title), `Content Placeholder 2` (body) |
| `slideLayout3` | Two Content | `Title 1` (title), `Content Placeholder 2` (left), `Content Placeholder 3` (right) |
| `slideLayout4` | Section Divider | `Title 1` (section title) |
| `slideLayout5` | Closing | `Text Placeholder 1` (closing text) |
| `slideLayout6` | Blank | (no placeholders) |

### Content types

| Type | Value format | Example |
|------|-------------|---------|
| `text` | String | `"Hello World"` |
| `bullets` | Array of strings | `["Point 1", "Point 2"]` |
| `image` | Object with `path` and `alt` | `{"path": "photo.png", "alt": "Description"}` |
| `chart` | Object with `type`, `title`, `data` | `{"type": "bar", "title": "Sales", "data": [{"label": "Q1", "value": 100}]}` |

Supported chart types: `bar`, `line`, `pie`, `donut`, `area`, `radar`, `scatter`, `stacked_bar`, `waterfall`, `funnel`, `gauge`, `treemap`.

## Diagram Examples

For individual diagram type examples, see the `diagrams/` directory which contains JSON examples for each supported diagram type (timeline, SWOT, process flow, etc.).

See [docs/diagrams/README.md](../docs/diagrams/README.md) for the full diagram gallery and decision tree.
