# Examples

Ready-to-run input files for `json2pptx`: raw **JSON** decks (this directory), one deck per chart/diagram type in [`diagrams/`](diagrams/README.md), and compact **semantic YAML** specs in `semantic/` (see [docs/SEMANTIC_COMPILER.md](../docs/SEMANTIC_COMPILER.md)).

## JSON Examples

JSON input mode provides direct, programmatic control over slide layout and content placement. Each slide specifies an exact `layout_id` and maps content to specific `placeholder_id` values from the template.

### Recommended starting points for agents

These pattern-rich decks are the best entry points for AI agents building new decks:

| File | Description | Template |
|------|-------------|----------|
| `varied-pitch-deck.json` | Investor pitch deck demonstrating pattern variety: icon-row, stat-hero, kpi-3up, before-after, process-flow, card-grid, timeline-horizontal, pyramid, comparison-2col, pull-quote | midnight-blue |
| `board-deck.json` | Executive board update using stat-hero, kpi-4up, matrix-2x2, pull-quote, arch-stack, before-after, kpi-2up, swimlane | forest-green |

### Pattern-first examples

| File | Description | Template |
|------|-------------|----------|
| `patterns-smoke.json` | Pattern library smoke test: one slide per pattern across a broad slice of the registry (KPI, canvas, matrix, timeline, card-grid, roadmap, heatmap, directory, bios, and more) | midnight-blue |
| `<pattern-name>.json` | Many patterns also have a dedicated deck named after the pattern (e.g. `driver-tree.json`, `waterfall-bridge.json`, `team-bios.json`, `radial-hub.json`) | varies |

### Testing & QA decks

| File | Description | Template |
|------|-------------|----------|
| `template-qa-deck.json` | Fixed reference deck used by `scripts/test_template_visual_qa.sh`. Exercises every mandatory layout role (title, content, two-column, section, blank-canvas, blank-title, closing) plus three representative pattern families (hand-built `shape_grid`, `comparison-2col`, `journey-maturity-model`). The two blank roles are pinned by their explicit canonical IDs (`blank-canvas` for the empty canvas, `blank-title` for the title canvas) so the same deck renders against every template. | any (override with `-template`) |

### Placeholder-first examples

| File | Description | Template |
|------|-------------|----------|
| `basic-deck.json` | Text and bullet slides: title, agenda, content, two-column, section divider, closing | midnight-blue |
| `charts.json` | Chart types: bar, line, pie, donut, area, funnel | forest-green |
| `diagrams.json` | Advanced chart types: waterfall, radar, gauge, treemap, stacked bar | warm-coral |
| `full-showcase.json` | All content types combined in a product launch strategy deck | midnight-blue |

### Pattern coverage

The pattern registry grows faster than a hand-kept matrix, so this README does
not list per-pattern coverage. For the live list of registered patterns run
`json2pptx patterns list` (MCP `list_patterns`); `json2pptx patterns show <name>`
prints canonical `example_values` for every pattern, including ones no example
deck uses yet.

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
json2pptx generate -json examples/basic-deck.json -json-output-report result.json
```

Validate a semantic YAML spec from `semantic/` — `json2pptx validate` reads raw
JSON only and rejects YAML with a parse error, so use the semantic validator
(`semantic/invalid.yaml` fails on purpose):

```bash
json2pptx semantic validate --spec examples/semantic/qbr.yaml
```

### JSON input structure

```json
{
  "template": "midnight-blue",
  "output_filename": "my-deck.pptx",
  "slides": [
    {
      "layout_id": "content",
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

The bundled templates are `abstract`, `blue-corporate`, `business-template`, `forest-green`, `midnight-blue`, `modern`, `modern-template`, `modern-yellow`, and `warm-coral` (all embedded in the binary).

List all available templates (including any installed locally):

```bash
json2pptx skill-info --mode list
```

### Layout reference

Every conforming template provides these canonical layouts. Use **canonical layout IDs** (not raw `slideLayoutN` IDs or display names) for cross-template compatibility:

| Canonical `layout_id` | Template Display Name | Placeholders |
|-----------------------|----------------------|--------------|
| `title` | Title Slide | `title`, `subtitle` |
| `content` | One Content | `title`, `body` |
| `two-column` | Two Content | `title`, `body`, `body_2` |
| `section` | Section Divider | `title` |
| `closing` | Closing | `title`, `subtitle` |
| `blank-title` | Blank + Title | `title` only (body content via `shape_grid` or `pattern`) |
| `blank-canvas` | Blank | none — `shape_grid`/`pattern` only, no title |
| `blank` (legacy) | Blank + Title | alias for `blank-title`; prefer the explicit IDs above |

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
