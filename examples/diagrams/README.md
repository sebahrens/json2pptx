# Diagram Example Decks

One small json2pptx **deck** per chart / diagram type. Each file is a normal
`PresentationInput` (title slide + one chart or diagram slide on the
`midnight-blue` template) — not an svggen request — so run them through the
json2pptx CLI or MCP server:

```bash
json2pptx validate examples/diagrams/swot.json
json2pptx generate -json examples/diagrams/swot.json -output ./output
```

Render every example:

```bash
for f in examples/diagrams/*.json; do
  json2pptx generate -json "$f" -output ./output
done
```

The chart or diagram lives in the slide's `chart_value` / `diagram_value`
content item. To render one of those payloads standalone (SVG/PNG, no deck),
use the svggen tools described in
[skills/render-diagram/SKILL.md](../../skills/render-diagram/SKILL.md).

## Examples

| File | Content | `type` | Shows |
|------|---------|--------|-------|
| `bar_chart.json` | `chart_value` | `grouped_bar` | Multi-series sales by region |
| `line_chart.json` | `chart_value` | `line` | Revenue trend, two years |
| `pie_chart.json` | `chart_value` | `pie` | Market share breakdown |
| `donut_chart.json` | `chart_value` | `donut` | Budget allocation with scheme accent colors |
| `waterfall.json` | `chart_value` | `waterfall` | Profit bridge |
| `timeline.json` | `diagram_value` | `timeline` | Release schedule |
| `gantt.json` | `diagram_value` | `gantt` | Development timeline |
| `process_flow.json` | `diagram_value` | `process_flow` | Order fulfilment workflow |
| `matrix_2x2.json` | `diagram_value` | `matrix_2x2` | Effort vs value prioritisation |
| `swot.json` | `diagram_value` | `swot` | Company SWOT |
| `pyramid.json` | `diagram_value` | `pyramid` | Tiered hierarchy |
| `porters_five_forces.json` | `diagram_value` | `porters_five_forces` | Industry analysis |
| `business_model_canvas.json` | `diagram_value` | `business_model_canvas` | SaaS business model |
| `value_chain.json` | `diagram_value` | `value_chain` | Manufacturing value chain |
| `nine_box_talent.json` | `diagram_value` | `nine_box_talent` | Team assessment |
| `house.json` | `diagram_value` | `house_diagram` | Strategy house |

For the full list of chart and diagram types, run `json2pptx capabilities`
(`vocabularies.chart_types` / `vocabularies.diagram_types`) or call the
`get_chart_capabilities` / `get_diagram_capabilities` MCP tools.
