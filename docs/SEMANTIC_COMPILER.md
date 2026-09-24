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
- `option_matrix`
- `table`
- `stat`
- `timeline`
- `matrix_2x2`
- `framework`
- `image_case`
- `process`
- `roadmap`
- `decision`
- `closing`
- `raw_json2pptx`

### Slide kind → compiled visual

Each content-bearing kind compiles to the named pattern its plan advertises (the same pattern `semantic explain` reports), so explain and compile stay in lock-step. When a payload falls outside the pattern's shape the slide **degrades** to a safe content slide (title + readable bullets — never a Go `map[...]` dump) and semantic validation emits a `SEMANTIC_DENSITY` advisory naming the count that caused the degradation. A `kpi_snapshot` additionally degrades to the bullet fallback when an individual metric value is too long for the compact KPI cards (e.g. `CHF 142.3M` exceeds the big-number budget): a value that satisfies the schema but not the rendered card is lowered to readable bullets rather than emitted as raw JSON the renderer would reject.

| Kind | Pattern | Payload | Fits the visual when |
|------|---------|---------|----------------------|
| `kpi_snapshot` | `kpi-2up`…`kpi-6up` | `kpis: [{value,label}]` | 2–6 KPIs |
| `chart_insight` | `chart-insights-split` | `chart: {type,data}`, `insights: [string]` | 1–6 insights (a usable chart with no `insights`/`insight` falls back to the `takeaway` as the single insight, so the chart is never silently dropped) |
| `comparison` | `comparison-2col` | `columns: [{title, items:[string]}, …]` | exactly 2 columns with equal, non-empty item counts (≤10 rows) |
| `stat` | `stat-hero` | `value`, `label` (+ `unit?`, `context?`, `source?`) | the number ≤20 chars, label ≤80, unit ≤10, context ≤120, source ≤80 |
| `timeline` | `timeline-horizontal` | `milestones: [{label, date?, end_date?, body?}]` | 3–7 milestones; label ≤60 chars, date ≤30, body ≤200 |
| `matrix_2x2` | `matrix-2x2` | `x_axis`, `y_axis`, `quadrants: [{header, body?}] x4` | exactly 4 headed quadrants and both axes named; header ≤80 chars, body ≤200, axis ≤60, axis end ≤20 |
| `framework` (`bmc`) | `bmc-canvas` | `sections: {key_partners…revenue_streams}` | all 9 cells present; ≤10 items each, ≤200 chars per item |
| `framework` (`swot`, `porters_five_forces`) | *native diagram, no pattern* | `sections: {strengths…threats}` / `{rivalry…buyers}` | all 4 / all 5 parts present |
| `image_case` | `image-text-split` | `body` or `bullets` (+ `image?`, `eyebrow?`, `heading?`, `metrics?`, `caption?`) | body ≤300 chars, eyebrow ≤30, heading ≤80, ≤5 bullets ≤140 each, ≤3 metrics |
| `decision` | `numbered-step-strip` / `card-grid` | `options: [{label, detail?}]`, `recommendation` | 3–6 options (label ≤60 chars, detail ≤180), or exactly 2 each with a detail |
| `process` | `numbered-step-strip` / `process-flow` | `steps: [{label, description?, type?}]` | 3–6 described steps (label ≤60 chars, description ≤180), or 3–8 bare / branching ones (≤80 per box) |
| `roadmap` | `phase-roadmap` | `phases: [{name, date_label?, description?, active?, milestone?}]` | 3–6 named phases |

`option_matrix` compiles to `table-highlight` within its 2–6 × 2–6 bounds and to a scored bullet list outside them. `table` compiles to a native table content block styled by the template's own table style (a layout, not a pattern), and to bullets when there is no header row. `comparison` compiles to `comparison-2col` for a balanced pair, `stylish-panels` for 3–5 columns (each a titled panel with its own bullets), `card-grid` for 2–5 columns those cannot hold, and bullets beyond that. `executive_summary` compiles to the `exec-summary` pattern at 3–5 points and to a plain content slide (bullets) otherwise; its plan advertises the pattern only when the payload will actually reach it. `architecture` compiles to `arch-stack` for 3–6 tiers that fit the pattern's label/detail/rail budgets, and to a bullet list carrying every word outside them (never a truncation, and never a blocking maxLength). `stat` compiles to `stat-hero` — one number, the words beneath it, and optionally a context line and a source — and to a content slide carrying every word past any of those budgets; its `label` defaults to the slide title, since an author who wrote only a title meant it as the words under the number, and its `source` is left to the slide's attribution band rather than repeated as a bullet. `timeline` compiles to `timeline-horizontal` for 3–7 dated milestones and to a dated bullet list outside them; a payload where any milestone carries an `end_date` is compiled with `style: gantt`, because the pattern rejects an end date in any other style and a milestone that spans a period is a bar rather than a dot. `roadmap` keeps `phase-roadmap`: phases with workstreams are a different slide from dates on a line. `matrix_2x2` compiles to `matrix-2x2` when it has four headed quadrants and two named axes, and to a bullet list otherwise — one that names each quadrant's position and both axes with their ends, because a quadrant stripped of where it sits on the axes has lost the point of the slide. Its `quadrants` list is read clockwise from the top left. `framework` is the one kind that reaches a native diagram: `bmc` compiles to the `bmc-canvas` pattern, while `swot` and `porters_five_forces` compile to the native OOXML diagram of that name on a `diagram` slide — so their plan advertises a layout only, which is what compile emits. All three degrade to bullets grouped under each part's own heading when a part is missing: a SWOT without its threats is not a SWOT, and an empty quadrant reads as a rendering bug rather than as missing content. `image_case` compiles to `image-text-split` — a picture beside the story and up to three result metrics — and to a content slide past the column's budgets, where the caption or the image's alt text stands in for the picture that cannot come with it. A picture with nothing said about it is a plain image slide rather than a case study, and is refused by the kind and by the pattern alike. `process` compiles to `numbered-step-strip` when its steps carry descriptions — a bold label over its own detail line — and to `process-flow` when they are bare labels or the process branches (`type: decision`), which is what the diamonds are for. The two used to be one path: the description was concatenated onto the label and centred in a flow box at ~9pt reversed out of solid accent (go-slide-creator-61up). `decision` compiles to `numbered-step-strip` (stacked-box) for 3–6 options and to a two-card `card-grid` for exactly two that each carry a detail, with the `recommendation` in the pattern's callout band beneath them; outside those it keeps the content slide it has always produced — the recommendation as a lead-in over option bullets. The ask is the slide a board deck exists for, and it used to be the plainest page in it (go-slide-creator-4ndv). Structural kinds (`title`, `section`, `closing`) and the `raw_json2pptx` escape hatch carry no pattern either. The explain↔compile parity gate (`internal/semantic.TestExplainCompileParity`) asserts every kind's advertised pattern equals the one compile emits.

For `executive_summary` and visual `decision` slides, a separate `takeaway` does not create a second stacked footer band. The compiler folds distinct takeaway wording into the existing bottom-line or recommendation callout; near-duplicate wording appears once and emits `SEMANTIC_DUPLICATE_CALLOUT`. If the combined executive-summary conclusion exceeds its 160-character pattern budget, the slide degrades to bullets with the bottom line and takeaway both retained.

**Pattern reachability.** `internal/semantic/reach.go` maps every registered pattern to the kind that compiles to it, or to `""` when no kind does. `semantic.UnreachablePatterns()` drives SKILL.md's "Patterns DeckSpec cannot reach" table and `cmd/json2pptx.TestPatternReachCoversTheRegistry` fails the build when a newly registered pattern has no entry — so a pattern cannot ship without someone saying whether a spec author can reach it, and the published table cannot drift from the code. A kind's `compositions` (published by `list_slide_kinds`, and what `validateCompositionOverride` accepts) lists only patterns THAT kind compiles to: a cross-kind suggestion there would validate clean and then silently do nothing, which is the failure the override reporting exists to prevent, so those suggestions live in the kind's summary instead.

**Closed payload contract.** `internal/semantic/payload_fields.go` (`kindPayloadFields`) lists, per kind, every payload key the compiler reads — canonical names, accepted aliases, the `pattern`/`layout` composition overrides, list-entry keys, and the chart object keys (`type`, `title`, `data`). It generates the per-kind `Slide_<kind>` schema variants (`additionalProperties: false`, closed entry/chart schemas) and drives `validateUnknownFields`: any other key would be dropped silently, so validation emits `SEMANTIC_UNKNOWN_FIELD` at the exact path (`slides[i].<key>`, `slides[i].<list>[j].<key>`, `slides[i].chart.<key>`) — a `warning` (never suppressed by `--strict off`), promoted to `error` under `strict`, with a `rename_field` fix carrying `did_you_mean` when a known key is within a small edit distance. Chart data is checked at `slides[i].chart.data` (not the non-existent `chart.series`): a missing or series-less `data` emits a `SEMANTIC_DENSITY` advisory whose `fix` (`provide_value`) carries `path`, `expected_shape` (`{categories:[…], series:[{name, values:[…]}]}`, or `{categories, values}` for pie/donut), and an `example`. `list_slide_kinds` publishes the variant as `item_schema` plus a copy-ready `example` per kind (`kind_examples.go`; every example validates clean under strict).

The `raw_json2pptx` escape hatch is structurally validated before it is passed through (`internal/semantic.validateRawEscapeHatch`, gating both `validate` and `compile`). The `slide` payload is decoded strictly as a raw `deckinput.SlideInput`: it must be a JSON object, carry no unknown fields (`DisallowUnknownFields` — a typo'd key surfaces as a blocking `SEMANTIC_UNKNOWN_FIELD` rather than being silently dropped), set a `slide_type` or `layout_id`, and carry renderable content (`content`, `shape_grid`, `pattern`, or `compose`). The `blank` slide_type is exempt from the content requirement (a deliberate content-free canvas). Failures are hard `SEMANTIC_REQUIRED`/`SEMANTIC_UNKNOWN_FIELD` errors at `slides[i].slide`, so a malformed escape-hatch payload fails fast at validate/compile time instead of compiling to an empty slide.

## Surfaces

CLI:

```bash
json2pptx semantic validate --spec deck.yaml
json2pptx semantic compile --spec deck.yaml --output compiled.json
json2pptx semantic compile --spec deck.yaml --envelope          # compiled_json + diagnostics
json2pptx semantic render --spec deck.yaml --output deck.pptx
json2pptx semantic explain --spec deck.yaml
json2pptx semantic schema
```

DeckSpec accepts either flat `slides[]` or a chapter form:

```yaml
meta:
  title: Quarterly review
  required_layouts: [title, content, section, closing]
structure:
  cover: {kind: title, title: Quarterly review}
  auto_agenda: true
  sections:
    - title: Performance
      slides:
        - {kind: kpi_snapshot, title: Momentum, kpis: [{value: 42, label: Wins}], takeaway: Execution accelerated}
  closing: {kind: closing, title: Decisions}
```

Structured expansion inserts section dividers and carries source paths and
section crumbs into the compiled deck. `meta.required_layouts` is applied after
narrative planning; `semantic explain` exposes
`layout_coverage.{requested,assigned,missing}`. These are assignable canonical
layout names, not the broader feasibility labels in
`recommend_visual.template_support.required_layout` (such as `full-image` or
`grid base`).

Pass `--spec -` to read the spec from stdin (e.g. `… --spec - < deck.yaml`), portable across platforms. Each subcommand's `-h`/`--help` prints usage and exits **0**, so automated probes can introspect the surface without treating help as a failure.

`semantic compile` writes the raw `PresentationInput` JSON by default. Add `--envelope` to emit a structured result instead — `{ok, slide_count, template, findings, compiled_json}` — so a compile-only flow surfaces non-blocking diagnostics (density, rhythm, raw-pattern preflight) without a separate `validate` run. This mirrors the HTTP `POST /api/v1/semantic/compile?include_compiled_json=true` response shape. A blocking parse/compile failure still emits the (`ok:false`) envelope and exits non-zero.

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
- `POST /api/v1/semantic/compile` (add `?include_compiled_json=true` for the raw deck)
- `POST /api/v1/semantic/render` — **deferred, returns HTTP 501**; the render orchestration lives in the CLI layer, so use `json2pptx semantic render` (CLI) or `render_deck_spec` (MCP)

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

`cmd/json2pptx` owns the CLI and MCP adapters. The raw render runner is factored so semantic render and existing raw generation use the same validation/generation/output-validation path. This includes asset resolution: a `raw_json2pptx` escape-hatch slide can carry image/icon URLs or relative asset paths, so `semantic render` runs the same guarded URL download and relative-path resolution `generate` does, resolving relative paths against the **spec's own directory**. Unreachable URLs or missing/oversized/wrong-extension local assets are rejected with the same `URL_FETCH_FAILED` / `IMAGE_PATH` / `BACKGROUND_IMAGE_PATH` diagnostics, so the escape hatch behaves identically under `render` and `generate`.

## Diagnostics and repair

Semantic validation returns the shared `FindingEnvelope` from `internal/diagnostics`. Findings prefer semantic paths such as `slides[2].kpis[1].label`. When a compiled raw deck triggers a fit or output-validation finding, the compiler maps the raw JSON pointer back through its `SourceMap` (exact match first, then nearest ancestor) and preserves the generated pointer as fallback evidence. The semantic slide index is recovered from the raw `slides[N]` prefix even when no mapping exists, so a finding always carries at least a slide-level locator. For the common density/overflow failures — an overlong metric label/value, an overfull KPI snapshot, an overlong takeaway, a dense comparison side, or a crowded roadmap phase list — the finding also carries a `recommended_edit` (`shorten_text`, `split_slide`, `reduce_items`, `simplify_side`, or `split_phases`) so an agent repairs the semantic source it authored rather than the generated shape_grid JSON.

### Post-compile raw preflight

After lowering a `DeckSpec` to a raw `PresentationInput`, `Compile` runs a **post-compile raw preflight** (`internal/semantic/preflight.go`) over every emitted pattern slide. It applies the same pattern-validation gate the renderer enforces in `expandPattern` (via the reusable `deckinput.ValidatePattern` helper) without expanding the grid, so a slide whose lowered pattern would be rejected at render — for example a KPI cell value that exceeds the `kpi-Nup` big-number budget, or a list with the wrong item count — is caught **at compile/validate time** instead of failing deep in generation. Preflight findings are error severity and block the compile (no `PresentationInput` is emitted), each mapped back through the `SourceMap` to the semantic source path with the raw pattern pointer retained under `evidence.raw_path` and a `recommended_edit` attached for the length/count failures. Because this runs inside `Compile`, it applies uniformly to `compile_deck_spec`, `render_deck_spec`, and the HTTP `compile`/`render` endpoints. The `kpi_snapshot` length degradation above pre-empts the preflight for the one kind that has a natural bullet fallback; other kinds surface the blocking preflight finding so the author edits the offending field.

Agents should repair semantic YAML/JSON first. Raw `PresentationInput` and `repair_slide` remain available for mechanical fixes and advanced escape-hatch workflows.

## Implementation beads

The implementation is tracked under epic `go-slide-creator-m0jg` with child beads `go-slide-creator-m0jg.1` through `go-slide-creator-m0jg.14`.
