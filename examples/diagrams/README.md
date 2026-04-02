# SVGGen Diagram Examples

This directory contains JSON examples for each diagram type in the svggen package.

## Usage

### Go Package

```go
import (
    "os"
    "encoding/json"
    "github.com/ahrens/svggen"
)

// Load example
data, _ := os.ReadFile("examples/diagrams/bar_chart.json")
var req svggen.RequestEnvelope
json.Unmarshal(data, &req)

// Render
doc, _ := svggen.Render(&req)
fmt.Println(doc.String())
```

### HTTP API

```bash
curl -X POST http://localhost:3001/render \
  -H "Content-Type: application/json" \
  -d @examples/diagrams/bar_chart.json
```

## Examples

| File | Diagram Type | Description |
|------|--------------|-------------|
| `bar_chart.json` | bar_chart | Multi-series sales comparison |
| `line_chart.json` | line_chart | Revenue trend over time |
| `pie_chart.json` | pie_chart | Market share breakdown |
| `donut_chart.json` | donut_chart | Budget allocation |
| `waterfall.json` | waterfall | Profit breakdown |
| `timeline.json` | timeline | Project roadmap |
| `process_flow.json` | process_flow | Order processing workflow |
| `matrix_2x2.json` | matrix_2x2 | Priority matrix |
| `porters_five_forces.json` | porters_five_forces | Industry analysis |
| `business_model_canvas.json` | business_model_canvas | SaaS business model |
| `value_chain.json` | value_chain | Manufacturing value chain |
| `nine_box_talent.json` | nine_box_talent | Team assessment |
| `house.json` | house | McKinsey 7S framework |

## Rendering All Examples

```bash
#!/bin/bash
for f in examples/diagrams/*.json; do
  name=$(basename "$f" .json)
  curl -X POST http://localhost:3001/render \
    -H "Content-Type: application/json" \
    -d @"$f" \
    -o "output/${name}.svg"
done
```
