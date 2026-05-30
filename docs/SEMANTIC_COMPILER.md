# Semantic Compiler Target State

The semantic compiler is a first-class authoring layer inside `json2pptx`. It lets agents write compact YAML/JSON that describes deck intent and content, then compiles that spec to the existing raw `PresentationInput` model and renders through the normal json2pptx engine.

```text
semantic YAML/JSON
  → internal/semantic parse + validate
  → archetype defaults + rhythm policy
  → compile to raw PresentationInput + SourceMap
  → existing validation / generation / output validation
  → semantic diagnostics mapped back to source fields
```

## Why it lives in json2pptx

The compiler depends on renderer-owned knowledge: pattern schemas and exemplars, template capability analysis, deck rhythm rules, fit findings, repair fix kinds, and output validation. Host applications may provide defaults, storage, and product workflow, but they should call the json2pptx semantic surface rather than reimplementing pattern selection or shape-grid construction.

## Authoring contract

Semantic specs accept YAML and JSON:

Deck-level intent lives under `meta`; each slide is a `kind`-tagged payload whose remaining fields are the kind's content (see `list_slide_kinds` / `json2pptx semantic schema` for the per-kind fields). A larger, test-verified example lives at [examples/semantic/qbr.yaml](../examples/semantic/qbr.yaml).

```yaml
meta:
  title: Q2 Business Review
  subtitle: Momentum improving, execution risk remains
  archetype: qbr
  audience: board
  template: midnight-blue

slides:
  - kind: title
    title: Q2 Business Review
    subtitle: Momentum improving, execution risk remains
  - kind: kpi_snapshot
    title: Quarter at a glance
    takeaway: Growth recovered, but margin remains below target.
    kpis:
      - { value: "$12.4M", label: "Revenue" }
      - { value: "61%", label: "Gross margin" }
  - kind: decision
    title: Approve EMEA sales capacity
    takeaway: Add enterprise sales capacity now to convert EMEA pipeline.
    recommendation: Add four enterprise AEs in Q3.
    options:
      - Hold current coverage and accept slower EMEA conversion.
      - Add four enterprise AEs in Q3 (recommended).
```

Initial archetypes:

- `board_update`
- `qbr`
- `sales_pitch`
- `strategy_proposal`
- `project_roadmap`
- `market_analysis`

Initial slide kinds:

- `title`
- `section`
- `executive_summary`
- `kpi_snapshot`
- `chart_insight`
- `comparison`
- `process`
- `roadmap`
- `decision`
- `closing`
- `raw_json2pptx`

## Surfaces

CLI:

```bash
json2pptx semantic validate --spec deck.yaml
json2pptx semantic compile --spec deck.yaml --output compiled.json
json2pptx semantic render --spec deck.yaml --output deck.pptx
json2pptx semantic explain --spec deck.yaml
json2pptx semantic schema
```

MCP:

- `validate_deck_spec`
- `compile_deck_spec`
- `render_deck_spec`
- `explain_deck_spec`
- `list_deck_archetypes`
- `list_slide_kinds`

HTTP:

- `GET /api/v1/semantic/schema`
- `POST /api/v1/semantic/validate`
- `POST /api/v1/semantic/compile`
- `POST /api/v1/semantic/render`

## Package layout

```text
internal/
  deckinput/        # importable raw PresentationInput model
  semantic/
    spec.go         # DeckSpec, DeckMeta, SlideSpec, typed slide payloads
    yaml.go         # YAML/JSON parsing
    validate.go     # semantic validation gates
    ir.go           # normalized DeckIR / SlideIR
    archetypes.go   # default rhythm and deck-shape policies
    rhythm.go       # density and visual-family checks
    compile.go      # DeckSpec → deckinput.PresentationInput
    sourcemap.go    # raw JSON pointer → semantic path mapping
    explain.go      # compiler decision explanations
    schema.go       # semantic JSON Schema export
```

`cmd/json2pptx` owns the CLI and MCP adapters. The raw render runner is factored so semantic render and existing raw generation use the same validation/generation/output-validation path.

## Diagnostics and repair

Semantic validation returns the shared `FindingEnvelope` from `internal/diagnostics`. Findings prefer semantic paths such as `slides[2].kpis[1].label`. When a compiled raw deck triggers a fit or output-validation finding, the compiler maps the raw JSON pointer back through its `SourceMap` (exact match first, then nearest ancestor) and preserves the generated pointer as fallback evidence. The semantic slide index is recovered from the raw `slides[N]` prefix even when no mapping exists, so a finding always carries at least a slide-level locator. For the common density/overflow failures — an overlong metric label/value, an overfull KPI snapshot, an overlong takeaway, a dense comparison side, or a crowded roadmap phase list — the finding also carries a `recommended_edit` (`shorten_text`, `split_slide`, `reduce_items`, `simplify_side`, or `split_phases`) so an agent repairs the semantic source it authored rather than the generated shape_grid JSON.

Agents should repair semantic YAML/JSON first. Raw `PresentationInput` and `repair_slide` remain available for mechanical fixes and advanced escape-hatch workflows.

## Implementation beads

The implementation is tracked under epic `go-slide-creator-m0jg` with child beads `go-slide-creator-m0jg.1` through `go-slide-creator-m0jg.14`.
