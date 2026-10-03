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
      - label: Hold current coverage
        detail: Accept slower EMEA conversion.
      - label: Add four enterprise AEs
        detail: Hire in Q3 to convert the EMEA pipeline.
        recommended: true
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
- `next_steps`
- `closing`
- `regions`
- `raw_json2pptx`

### Slide kind → compiled visual

Each content-bearing kind compiles to the named pattern its plan advertises (the same pattern `semantic explain` reports), so explain and compile stay in lock-step. When a payload falls outside the pattern's shape the slide **degrades** to a safe content slide (title + readable bullets — never a Go `map[...]` dump) and semantic validation emits a `SEMANTIC_DENSITY` advisory naming the count that caused the degradation. A `kpi_snapshot` additionally degrades to the bullet fallback when an individual metric value is too long for the compact KPI cards (e.g. `CHF 142.3M` exceeds the big-number budget): a value that satisfies the schema but not the rendered card is lowered to readable bullets rather than emitted as raw JSON the renderer would reject.

| Kind | Pattern | Payload | Fits the visual when |
|------|---------|---------|----------------------|
| `kpi_snapshot` | `kpi-2up`…`kpi-6up` | `kpis: [{value,label,delta?,comparator?}]` | 2–6 KPIs; `comparator` (alias `vs`, ≤24 chars, "vs plan +4 pts") renders as its own line under the caption |
| `chart_insight` | `chart-insights-split` | `chart: {type,data}`, `insights: [string]`, `insight?: string` | 1–6 bullets or one scalar `insight` rendered as a so-what callout; a usable chart with neither falls back to the `takeaway` as one bullet, so the chart is never silently dropped |
| `comparison` | `comparison-2col` | `columns: [{title, items:[string]}, …]` | exactly 2 columns with equal, non-empty item counts (≤10 rows) |
| `stat` | `stat-hero` | `value`, `label` (+ `unit?`, `context?`, `source?`) | the number ≤20 chars, label ≤80, unit ≤10, context ≤120, source ≤80 |
| `timeline` | `timeline-horizontal` | `milestones: [{label, date?, end_date?, body?}]` | 3–7 milestones; label ≤60 chars, date ≤30, body ≤200; with any `end_date` the dates must be readable as dates (`2026-03`, `Mar 2026`, `Q1 2026`, …) to be drawn to scale |
| `matrix_2x2` | `matrix-2x2` | `x_axis`, `y_axis`, `quadrants: [{header, body?}] x4` | exactly 4 headed quadrants and both axes named; header ≤80 chars, body ≤200, x axis ≤16, y axis ≤60, axis end ≤11 |
| `framework` (`bmc`) | `bmc-canvas` | `sections: {key_partners…revenue_streams}` | all 9 cells present; ≤10 items each, ≤200 chars per item |
| `framework` (`swot`, `porters_five_forces`) | *native diagram, no pattern* | `sections: {strengths…threats}` / `{rivalry…buyers}` | all 4 / all 5 parts present |
| `image_case` | `image-text-split` | `body` or `bullets` (+ `image?` (`fit: contain` keeps a whole screenshot), `callouts?: [{label, x, y, units?}]`, `eyebrow?`, `heading?`, `metrics?`, `caption?`) | body ≤300 chars, eyebrow ≤30, heading ≤80, ≤5 bullets ≤140 each, ≤3 metrics, ≤6 callouts with a label ≤40 |
| `decision` | `numbered-step-strip` / `card-grid` | `options: [{label, detail?, recommended?}]`, `recommendation`, `recommended?` | 3–6 options (label ≤60 chars, detail ≤180), exactly 2 each with a detail, or 7–12 each with a detail (label ≤80, detail ≤160) |
| `next_steps` | `next-steps` | `actions: [{action, owner?, date?}]` (aliases `next_steps`, `steps`), `decisions?: [string]`, `decisions_label?` | 2–6 actions (action ≤90, owner ≤30, date ≤20) and 0–3 decisions ≤120; otherwise bullets that keep owner, date and each decision. `plan_deck format:"deckspec"` closes on it; `closing` stays the plain Q&A page |
| `pillars` | `strategy-house` / `stylish-panels` | `pillars: [{title, body?: [string]}]`, `objective?`, `foundation?` (a string, or a list of levels each a string or a list of strings), `beam?`, `roof_badges?` | 3–5 pillars; a house when `objective` and `foundation` are both given: ≤3 foundation levels, ≤5 cells per split level (≤40 chars each, bands ≤140), cell counts that share a column grid with the pillars. `foundation[i]` and `foundation[i][j]` map to the same paths in the compiled `pattern.values` |
| `process` | `numbered-step-strip` / `process-flow` | `steps: [{label, description?, type?}]` | 3–6 described steps (label ≤60 chars, description ≤180), or 3–8 bare / branching ones (≤80 per box) |
| `roadmap` | `phase-roadmap` | `phases: [{name, date_label?, description?, active?, milestone?}]` | 3–6 named phases |
| `regions` | *one shape grid; nested `stat-hero` / `kpi-Nup` / `timeline-horizontal` per region* | `arrangement`, `regions: [{kind, size_pct?, heading?, source?, …}]` | 2–3 regions (`columns`/`rows`) or exactly 3 (`main_left`/`main_right`/`main_top`/`main_bottom`), each share 15–85%; region kinds `chart`, `stat`, `kpis` (2–4), `table` (≤4×5), `timeline` (3–7), `image`, `text`. No degrade: a region outside its budget is an error at `slides[i].regions[k]`. A `kpis` region renders through `kpi-Nup` like `kpi_snapshot` (an open strip for plain value + label metrics), steps its type down to fit its share (40/14 → 28/12 → 24/12pt), and when even that does not fit is refused with an error `fit_overflow` at `slides[i].regions[k]` from both `validate_deck_spec` and `render_deck_spec` |

`option_matrix` compiles to `table-highlight` within its 2–6 × 2–6 bounds and to a scored bullet list outside them. `table` compiles to a native table content block styled by the template's own table style (a layout, not a pattern), and to bullets when there is no header row. `comparison` compiles to `comparison-2col` for a balanced pair, `stylish-panels` for 3–5 columns (each a titled panel with its own bullets), `card-grid` for 2–5 columns those cannot hold and for 6–12 columns whose items join to at most 160 characters (the pattern arranges the cards from their count — 7 as 4 + 3 — so an odd count is not a fallback), and bullets beyond that. `executive_summary` compiles to the `exec-summary` pattern at 3–5 points and to a plain content slide (bullets) otherwise; its plan advertises the pattern only when the payload will actually reach it. `architecture` compiles to `arch-stack` for 3–6 tiers that fit the pattern's label/detail/rail budgets, and to a bullet list carrying every word outside them (never a truncation, and never a blocking maxLength). A tier's `items` pass through as the pattern's `components` — one block each inside the tier band — when there are 1–12 of them, each within 40 characters; a longer list, or one with an over-long name, is joined into the tier's one-line `description` as before, and an explicit `description` always wins over `items` (go-slide-creator-6h1fy). `stat` compiles to `stat-hero` — one number, the words beneath it, and optionally a context line and a source — and to a content slide carrying every word past any of those budgets; its `label` defaults to the slide title, since an author who wrote only a title meant it as the words under the number, and its `source` is left to the slide's attribution band rather than repeated as a bullet. `timeline` compiles to `timeline-horizontal` for 3–7 dated milestones and to a dated bullet list outside them; a payload where any milestone carries an `end_date` is compiled with `style: gantt`, because the pattern rejects an end date in any other style and a milestone that spans a period is a bar rather than a dot. The gantt places every milestone on a shared, labelled time axis: a range is a bar from the start of `date` to the end of `end_date`, a milestone without `end_date` is a diamond marker at its date, and a value that is not a date (`"Autumn"`) gets no bar and reports `TIMELINE_DATE_UNPARSEABLE` at the slide's milestones (go-slide-creator-o34er; it used to draw one identical full-width bar per milestone). `roadmap` keeps `phase-roadmap`: phases with workstreams are a different slide from dates on a line. `matrix_2x2` compiles to `matrix-2x2` when it has four headed quadrants and two named axes, and to a bullet list otherwise — one that names each quadrant's position and both axes with their ends, because a quadrant stripped of where it sits on the axes has lost the point of the slide. Its `quadrants` list is read clockwise from the top left. `framework` is the one kind that reaches a native diagram: `bmc` compiles to the `bmc-canvas` pattern, while `swot` and `porters_five_forces` compile to the native OOXML diagram of that name in a one-cell shape grid on `blank-title` — so their plan advertises a layout only, which is what compile emits. All three degrade to bullets grouped under each part's own heading when a part is missing: a SWOT without its threats is not a SWOT, and an empty quadrant reads as a rendering bug rather than as missing content. `image_case` compiles to `image-text-split` — a picture beside the story and up to three result metrics — and to a content slide past the column's budgets, where the caption or the image's alt text stands in for the picture that cannot come with it. A picture with nothing said about it is a plain image slide rather than a case study, and is refused by the kind and by the pattern alike. `process` compiles to `numbered-step-strip` when its steps carry descriptions — a bold label over its own detail line — and to `process-flow` when they are bare labels or the process branches (`type: decision`), which is what the diamonds are for. The two used to be one path: the description was concatenated onto the label and centred in a flow box at ~9pt reversed out of solid accent (go-slide-creator-61up). `decision` compiles to `numbered-step-strip` (stacked-box) for 3–6 options to a two-card `card-grid` for exactly two that each carry a detail, and to a `card-grid` the pattern arranges from the count for 7–12 that each carry a detail (label ≤80, detail ≤160), with the `recommendation` in the pattern's callout band beneath them; outside those it keeps the content slide it has always produced — the recommendation as a lead-in over option bullets. The ask is the slide a board deck exists for, and it used to be the plainest page in it (go-slide-creator-4ndv). Structural kinds (`title`, `section`, `closing`) and the `raw_json2pptx` escape hatch carry no pattern either. The explain↔compile parity gate (`internal/semantic.TestExplainCompileParity`) asserts every kind's advertised pattern equals the one compile emits.

For `executive_summary` and visual `decision` slides, the bottom-line or recommendation callout is the slide's takeaway, so `SEMANTIC_TAKEAWAY_REQUIRED` is not emitted when one is present. A slide carries one conclusion band (go-slide-creator-zvu7c): when the exec-summary pattern or a numbered-option / paired-card decision visual renders, a `takeaway` written beside `bottom_line` / `recommendation` is moved to the speaker notes (`Takeaway: …`, after any authored `notes`) instead of stacking a second near-duplicate band, and validation emits `SEMANTIC_DUPLICATE_CALLOUT` — "repeats" for near-duplicate wording, "moved to the speaker notes" for a distinct one. If the executive-summary bottom line alone exceeds its 160-character pattern budget, the slide degrades to bullets with the bottom line and takeaway both retained.

Layouts (go-slide-creator-ngbnf, go-slide-creator-maq6l): `table`, `org` and the `framework` kind's SWOT / five-forces diagrams host their native table / org chart / framework shapes in a one-cell shape grid on `blank-title`, the layout every pattern kind uses, so the title keeps one position and size across the deck (`TestDeckSpecContentKindsShareTitleGeometry` asserts one layout and one title xfrm per deck on abstract and p-style). A plain `closing` (no bullets) compiles to the template's own `closing` layout rather than the cover; a closing title over 40 characters emits the advisory `SEMANTIC_DENSITY` (a template-blind length; the measured budget for a template is `list_slide_kinds` `budgets`). An `agenda` with subtitles stays the numbered `agenda` pattern (`values.subtitles`) with the `current` section highlighted; `agenda-with-images` is used only on request or for a subtitle over 120 characters. An `image_case` with no `image` / `photo` / `screenshot` and no `image_label` emits the advisory `SEMANTIC_IMAGE_MISSING` (the slide renders an "Image placeholder" box).

An `agenda`'s `title` is optional: without one the slide renders the default title "Agenda" (the title a structure-mode deck's generated agenda carries) and raises no `MISSING_TITLE` (go-slide-creator-y81vn). Render diagnostics always carry a `semantic_path`: a slide-level finding with no source link names the slide (`slides[2]`, or its structure-mode locator), and `MISSING_TITLE` names the field to write (`slides[2].title`); only a deck-level finding has none.

A roadmap phase's `items[]` render as native bullets under its description, not one ` · `-joined paragraph (the bullet fallback still joins them onto the phase's line). An `option_matrix` whose `recommended` option sums below another option on its Harvey or RAG scale emits the advisory `SEMANTIC_RECOMMENDATION_OUTSCORED`, unless the recommendation wins or ties the `decisive_criterion`. `SEMANTIC_RHYTHM_SECTIONING` counts body slides only (cover, agenda, closing excluded) and fires above 10 — `plan_deck`'s default `slide_budget` — so a default-size plan plus cover and closing is one chapter.

**Continued exhibits are one unit.** Slides whose titles mark them as parts of one exhibit — `Savings by lever (1/2)` and `(2/2)`, `[2 of 3]`, or a trailing `(cont.)` — count once in `SEMANTIC_RHYTHM_MONOTONY`, so splitting a table as `SEMANTIC_DENSITY` advises does not create a run; the monotony message now names the run (`slides[4] to slides[6]`). A slide sitting between two parts (up to three slides apart; typically a `section` divider added to break a run) is reported at that slide as the warning `SEMANTIC_RHYTHM_CONTINUATION_SPLIT`: move it before the first part or after the last.

**Combined recommendations** (go-slide-creator-3hcw6). `option_matrix.recommended` takes one option (name or 0-based index) or a list; the first resolved row is emitted as `table-highlight`'s `highlight_row` and the rest as `highlight_rows`, so a single value compiles byte-for-byte as before. `decision` marks options with `options[].recommended: true` or names them in a slide-level `recommended` (label, index, or list) — at least one is required, more than one is a combined recommendation. With two or more recommended options and no authored `recommendation` / `takeaway`, the band reads `Recommended: A and B` (`slides.CombinedRecommendation`), mapped back to `options` (decision) or `recommended` (option_matrix); authored wording is never replaced. `SEMANTIC_RECOMMENDATION_OUTSCORED` compares one recommended row only.

**Callouts on a picture** (go-slide-creator-n3j96). `image_case.callouts` compile to slide `overlays[]` of kind `callout` whose `to` is an `anchor_image` point on the pattern's picture — grid row 0, column 0, or column 1 when `image_side` is `right` — with no `from`: the engine places each label beside its target inside the picture's frame and resolves the picture even when a `caption` nests it one grid down. Each overlay is source-mapped to `slides[i].callouts[j]`, in dotted and JSON Pointer form, so `OVERLAY_TARGET_CROPPED` lands on the authored callout. The content and two-column fallbacks cannot anchor to a picture and keep the labels as bullets. `recommend_visual` answers a callout intent ("screenshot with callouts", "annotated screenshot") with `image_case` first, and its `next_tool_call` recipe carries `callouts` on a sample screenshot the server writes to `<tmp>/json2pptx-samples/` (callouts need a real picture; replace `image.path` with yours). A `regions` image region draws no callouts: `callouts` there is refused (`SEMANTIC_UNKNOWN_FIELD`) with a pointer to `image_case`.

**Text budgets** (go-slide-creator-iubjb). `internal/semantic/slides/budgets.go` is the table of fixed per-field budgets, built from the constants the compilers enforce (`semantic.KindFieldBudgets`). `list_slide_kinds` returns it per kind as `budgets[]` (`basis: "fixed"`) when `template` or `fields: ["budgets"]` is requested, ahead of the `measured` budgets for `title`, `subtitle` and `takeaway`: the title placeholders of the layout a kind renders on (cover, section, closing, and the tighter of One Content and Blank + Title for everything else — the numbers `examine_template` reports) and the takeaway band the takeaway fit finding measures (`takeawayBandBudget`). With no template the tightest across the embedded templates is reported. Three tests hold the statements together: `TestKindBudgetsAgreeWithSchemaDescriptions` (every fixed budget is stated in its field's schema description), `TestKindBudgetsAgreeWithFindings` (a field one character over is reported quoting that number) and `TestSlideKindBudgetsAgreeWithRenderFindings` (on every template, copy inside the measured budgets draws no title or takeaway finding, and the takeaway and stat-stack findings quote the reported budgets). `TestKindSummariesStateOnlyBudgetedLengths` keeps a summary from quoting a length the budgets do not carry.

**Pattern reachability.** `internal/semantic/reach.go` maps every registered pattern to the kind that compiles to it, or to `""` when no kind does. `semantic.UnreachablePatterns()` drives SKILL.md's "Patterns DeckSpec cannot reach" table and `cmd/json2pptx.TestPatternReachCoversTheRegistry` fails the build when a newly registered pattern has no entry — so a pattern cannot ship without someone saying whether a spec author can reach it, and the published table cannot drift from the code. A kind's `compositions` (published by `list_slide_kinds`, and what `validateCompositionOverride` accepts) lists only patterns THAT kind compiles to: a cross-kind suggestion there would validate clean and then silently do nothing, which is the failure the override reporting exists to prevent, so those suggestions live in the kind's summary instead. A listed override is honoured only when the slide compiles to it: `normalizeSlide` trial-compiles the slide with `slides.Input.Override` set, every kind compiler routes `layout: content` to its native fallback and a multi-pattern kind routes a requested pattern to that pattern's compiler, and an override the payload cannot take keeps the compiler's own plan and is reported as `SEMANTIC_PATTERN_NOT_AVAILABLE` with the reason (go-slide-creator-vj549). `TestEveryListedCompositionCompilesAsReported` holds every kind's `compositions[]` to this.

**Closed payload contract.** `internal/semantic/payload_fields.go` (`kindPayloadFields`) lists, per kind, every payload key the compiler reads — canonical names, accepted aliases, the `pattern`/`layout` composition overrides, list-entry keys, and the chart object keys (`type`, `title`, `data`). It generates the per-kind `Slide_<kind>` schema variants (`additionalProperties: false`, closed entry/chart schemas) and drives `validateUnknownFields`: any other key would be dropped silently, so validation emits `SEMANTIC_UNKNOWN_FIELD` at the exact path (`slides[i].<key>`, `slides[i].<list>[j].<key>`, `slides[i].chart.<key>`) — a blocking `error` at every strictness level (including `--strict off`), with a `rename_field` fix carrying `did_you_mean` when a known key is within a small edit distance. Chart data is checked at `slides[i].chart.data` (not the non-existent `chart.series`): a missing or series-less `data` emits a `SEMANTIC_DENSITY` advisory whose `fix` (`provide_value`) carries `path`, `expected_shape` (`{categories:[…], series:[{name, values:[…]}]}`, or `{categories, values}` for pie/donut), and an `example`. `list_slide_kinds` publishes the variant as `item_schema` plus a copy-ready `example` per kind (`kind_examples.go`; every example validates clean under strict).

The `raw_json2pptx` escape hatch is structurally validated before it is passed through (`internal/semantic.validateRawEscapeHatch`, gating both `validate` and `compile`). The `slide` payload is decoded strictly as a raw `deckinput.SlideInput`: it must be a JSON object, carry no unknown fields (`DisallowUnknownFields` — a typo'd key surfaces as a blocking `SEMANTIC_UNKNOWN_FIELD` rather than being silently dropped), set a `slide_type` or `layout_id`, and carry renderable content (`content`, `shape_grid`, `pattern`, or `compose`). The `blank` slide_type is exempt from the content requirement (a deliberate content-free canvas). Failures are hard `SEMANTIC_REQUIRED`/`SEMANTIC_UNKNOWN_FIELD` errors at `slides[i].slide`, so a malformed escape-hatch payload fails fast at validate/compile time instead of compiling to an empty slide. `swimlane` has no kind of its own and is reached this way: its arrows connect the step columns left to right, so give every step its own column (empty strings in the other lanes) or state the order in `values.flow` as `[lane, step]` pairs; lanes that share a column with no `flow` are reported as `SWIMLANE_FLOW_AMBIGUOUS` by `validate_deck_spec`'s fit findings.

## Surfaces

CLI:

```bash
json2pptx semantic validate --spec deck.yaml
json2pptx semantic validate --spec deck.yaml --template modern  # measure on the template you will render on
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

### One finding set for validate and render

`semantic validate` and `validate_deck_spec` do not predict a render: they run
the same compiled-spec run `semantic render` / `render_deck_spec` make, into a
scratch directory that is removed, and report that run's diagnostics. For one
spec revision and one template the two tools therefore return the same findings
(code, path, severity), and `validate` is `ok` exactly when the render would be
`deterministic_ready`. `TestDeckSpecFindingParityCorpus` asserts it over the
shipped examples, every slide kind and the agent-journey decks on every shipped
template.

- **Template.** Findings are measured on a template. Precedence is
  `meta.template` > the call's `template` (`--template`) > the template a
  `deck_id` is bound to > the archetype default. The validate envelope echoes
  `template` and `template_source` (`meta.template`, `template argument`,
  `deck_id`, `archetype default`) and adds a `warnings[]` entry when the spec
  pins none. A `deck_id` is bound by the first call that names a template and
  follows `meta.template`; a later call naming a different template is
  evaluated on it for that call only (with a warning), so a deck's template
  changes only by a patch to `/meta/template`. `fork: true` with that patch
  binds the new `deck_id` to the new template and leaves the source deck's
  binding alone; `restore` follows the restored revision's `meta.template`.
- **Severity.** A finding blocks exactly when its severity is `error`
  (`blocking: true`); see
  [FIT_FINDINGS.md](FIT_FINDINGS.md#severity-and-blocking-on-the-deckspec-surfaces).
  `semantic validate` and `semantic render` exit 0 exactly when no blocking
  finding remains (render: and the deck was written), whatever
  `--output-validation` is; `ok` in the printed result agrees with the exit
  status. A deck with blocking findings is still written.
- **Raw slides.** A `raw_json2pptx` slide's pattern values and chart / diagram
  data are checked against the contracts generation enforces, and each failure
  is reported inside the raw slide, e.g.
  `slides[2].slide.pattern.values.current` or
  `slides[2].slide.content[1].diagram_value.data.primary[2].highlight`.
- **Refusals.** A refused render reports the fit findings on every slide, not
  only the first refused paragraph, and folds per-field refusals under the
  slide's capacity finding (`symptoms[]`).

### Waiving storyline findings

```yaml
meta:
  title: Seed round
  archetype: sales_pitch          # does not call for an executive summary
  waivers:
    - code: CLOSING_WITHOUT_NEXT_STEPS
      reason: The brief fixes seven slides; the ask is made verbally.
```

`meta.waivers[]` takes `{code, reason}` for the storyline codes
`NO_EXECUTIVE_SUMMARY`, `CLOSING_WITHOUT_NEXT_STEPS`, `TITLE_NOT_ACTION` and
`takeaway_missing`; any other code, a repeated code or an empty reason is a
blocking `SEMANTIC_REQUIRED` at `meta.waivers[i]`. A waived finding stays in
the list as an `info` with `waived: <reason>` and is left out of the score and
the gate. A registered archetype whose defaults are not executive
(`sales_pitch`, `project_roadmap`, `market_analysis`) waives
`NO_EXECUTIVE_SUMMARY` without a `waivers` entry. Validate and render results
record what was waived under `waivers[]` as `{code, reason, source, findings}`,
`source` being `meta.waivers` or `meta.archetype`. Without a waiver nothing
changes.

Pass `--spec -` to read the spec from stdin (e.g. `… --spec - < deck.yaml`), portable across platforms. Each subcommand's `-h`/`--help` prints usage and exits **0**, so automated probes can introspect the surface without treating help as a failure.

`semantic compile` writes the raw `PresentationInput` JSON by default. Add `--envelope` to emit a structured result instead — `{ok, slide_count, template, findings, compiled_json}` — so a compile-only flow surfaces non-blocking diagnostics (density, rhythm, raw-pattern preflight) without a separate `validate` run. This mirrors the HTTP `POST /api/v1/semantic/compile?include_compiled_json=true` response shape. A blocking parse/compile failure still emits the (`ok:false`) envelope and exits non-zero.

MCP:

- `validate_deck_spec`
- `compile_deck_spec`
- `render_deck_spec`
- `explain_deck_spec`
- `list_deck_archetypes`
- `list_slide_kinds`

### Revising a stored deck (`deck_id`)

`validate_deck_spec` and `render_deck_spec` store the spec they act on and
return its `deck_id`. A revision then sends `deck_id` (not `spec`) plus a
`patch`. The handle is per-process and expires an hour after its last stored
revision.

**Slide ids.** Every slide accepts an optional `id` (a letter, then letters,
digits, `_` or `-`; at most 40 characters; unique in the deck). It never
renders. On first store, slides without one are assigned `s1`, `s2`, …; an
assigned id is never reused. An id survives inserts, removals and moves, and
it replaces the index in a patch path: `/slides/s4/title` and
`/slides/3/title` address the same slide while it sits at index 3. Index
addressing keeps working. A malformed or duplicate authored id is a
`SEMANTIC_FIELD_TYPE` error at `slides[i].id`; a patch that would duplicate
an id is refused. The structured form gets ids on `cover`, `closing` and
every `sections[].slides[]` entry; generated agenda and divider slides have
none.

**Patch ops** (`[{op, path, value | from}]`, applied in order, all or
nothing):

| op | effect |
|----|--------|
| `replace` | Set an existing field or array element. Replacing a whole slide keeps its id unless the new slide names one. |
| `add` | Create a field; in an array, insert before the index or id named (`-` appends). |
| `remove` | Delete a field or element. |
| `move` | Read `from`, remove it there, add it at `path`. As in RFC 6902 `path` is resolved after the removal: `{"op":"move","from":"/slides/5","path":"/slides/7"}` puts the slide at index 7 of the result. |
| `copy` | Add a deep copy of `from` at `path`. A copied slide gets a new id. |

**What is stored.** Responses carry `stored` and `revision`.

- `render_deck_spec`: the patch is transactional with the render. A refused
  render (`success:false`) stores nothing and reports `stored:false`; the
  suggested `next_tool_call` patch then carries the unstored ops, so it
  applies to the deck as stored. A spec sent in the call (no `deck_id`) is
  still stored when its render is refused, so it can be patched into shape.
- `validate_deck_spec`: stores whatever parses, findings or not — the way to
  keep an edit that does not render yet.
- `dry_run: true` (either tool): run the patch, store nothing.
- `fork: true`: store the result under a new `deck_id` (history starts at
  revision 1, slide ids are kept); the source deck is left as it was.
- `restore: N`: start from kept revision `N` instead of the current one; a
  `patch` applies on top and the result is a new revision. Up to 50 revisions
  are kept per handle.

**What changed.** `changed_slides` is always present. It lists the 0-based
slides that look different from the baseline — for `validate_deck_spec` the
stored revision the call started from, for `render_deck_spec` the last
rendered revision (so every slide on a first render, and edits validated
since the last render count). `slide_changes` classifies every affected
slide as `{id, index, slide_number, change, was_index?}`:

| change | meaning | in `changed_slides` |
|--------|---------|---------------------|
| `edited` | Visible content differs (includes a section divider whose chapter number changed). | yes |
| `inserted` | New since the baseline. | yes |
| `restyled` | Same content; `meta`, structure options or the template changed. | yes |
| `moved` | Same content, reordered relative to its neighbours. | no |
| `renumbered` | Same content and order; index shifted by an insert or removal. | no |
| `notes_only` | Only speaker notes differ. | no |
| `removed` | Gone since the baseline (no `index`). | no |

After a successful render `next_tool_call` asks `render_deck_thumbnails` for
`changed_slides` only (the whole deck when every slide changed) and is absent
when a re-render changed nothing visible.

The validating render `validate_deck_spec` runs is not a render of the deck: it
writes into a scratch directory, marks no revision as rendered and leaves the
last-render baseline alone (it stores the spec as any validate does, and
nothing under `dry_run`). A render on a template the `deck_id` is not bound to
(a one-off `template` argument) keeps the binding and the revision, and reports
every slide as `restyled`; so does the next render back on the bound template,
because the baseline is what was last rendered.

**Compact patch responses.** A `deck_id` + `patch` (or `restore`) render of a
deck that has rendered before returns the verdict, the change list, every
blocking diagnostic, and the scores, plan rows and findings of the slides in
`changed_slides`; `diagnostics_omitted` counts what was left out and
`verbose: true` returns the full response. Other renders are unchanged and
include `slides[{id, index, slide_number, kind}]`. Render diagnostics carry
`slide_id`; validate findings carry `evidence.slide_id`.

**Slide ids on the image tools.** `render_deck_thumbnails` `slide_indices`
takes slide ids beside 0-based indices (`[4, "costs"]`), and
`render_slide_image` takes `slide_id` in place of `slide_index`; each
returned slide carries `id` beside `index`. The CLI spells it
`render-thumbnails --slides costs,3` and `render-slide --slide-id costs`.
The image tools see a `.pptx`, so ids resolve against that exact file (by
sha256): the table of contents `render_deck_spec` recorded when it wrote it,
or the `<deck>.pptx.authoring.json` sidecar `semantic render` wrote (authored
ids only — the CLI assigns none). An unknown id is refused before rendering
with the ids the deck has; a file nothing is known about has no ids.

**Reading the store** (`validate_deck_spec`; these return without validating
or storing):

- `read: "spec"` → `spec`, the stored DeckSpec with ids.
- `read: "<slide id or 0-based index>"` → `slide` and `slide_ref`.
- `read: "history"` → `revisions[{revision, time, tool, note, changes}]` and
  `slides[{id, index, slide_number, kind, title, last_changed, last_change}]`:
  the last revision that changed each slide, and how.
- `read: "diff:A..B"` → `diff {from, to, changes[]}`: how kept revision B
  differs from kept revision A, for any two revisions (`diff:A` compares A
  with the current one; `diff:5..1` reads backwards). `changes[]` rows are
  `slide_changes` rows with B's indices (a removed slide has only
  `was_index`); an `edited` row also lists `fields`, the slide's top-level
  fields that differ. `changed_slides` holds the indices that look different
  and `summary` counts the classes. A revision the handle no longer keeps is
  refused with the kept range.
- `find: "9.4"` → `hits[{path, slide_id, index, excerpt}]` and `hit_count`
  over every string and number in the spec (meta, titles, bodies, chart data,
  notes). Text matches ignore ASCII case; a query that starts or ends with a
  digit matches whole numbers only (`9.4` finds `$9.4m`, not `19.4` or
  `9.45`). `id`, `kind`, `type`, `pattern`, `layout`, `template` and
  `archetype` values are not searched.
- `find` + `replace` rewrites every hit as one patch, then validates and
  stores it like any patch. A numeric hit is replaced only by a numeric
  replacement; otherwise it is reported with `skipped`.

HTTP:

- `GET /api/v1/semantic/schema`
- `POST /api/v1/semantic/validate`
- `POST /api/v1/semantic/compile` (add `?include_compiled_json=true` for the raw deck)
- `POST /api/v1/semantic/render` — compile and render to `.pptx` through the same runner as `json2pptx semantic render` / `render_deck_spec` (raw spec body, or multipart with a `spec` part plus an optional bring-your-own `template` .pptx and `assets` files); the runner is injected into `internal/api` from `cmd/json2pptx` at server wiring time. See `docs/api/README.md` (Render)

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

After lowering a `DeckSpec` to a raw `PresentationInput`, `Compile` runs a **post-compile raw preflight** (`internal/semantic/preflight.go`) over every emitted pattern slide and every pattern nested in a shape-grid cell (a `regions` slide's stat or timeline), whose findings map to the region that wrote it. It applies the same pattern-validation gate the renderer enforces in `expandPattern` (via the reusable `deckinput.ValidatePattern` helper) without expanding the grid, so a slide whose lowered pattern would be rejected at render — for example a KPI cell value that exceeds the `kpi-Nup` big-number budget, or a list with the wrong item count — is caught **at compile/validate time** instead of failing deep in generation. Preflight findings are error severity and block the compile (no `PresentationInput` is emitted), each mapped back through the `SourceMap` to the semantic source path with the raw pattern pointer retained under `evidence.raw_path` and a `recommended_edit` attached for the length/count failures. Because this runs inside `Compile`, it applies uniformly to `compile_deck_spec`, `render_deck_spec`, and the HTTP `compile`/`render` endpoints. The `kpi_snapshot` length degradation above pre-empts the preflight for the one kind that has a natural bullet fallback; other kinds surface the blocking preflight finding so the author edits the offending field.

Agents should repair semantic YAML/JSON first. Raw `PresentationInput` and `repair_slide` remain available for mechanical fixes and advanced escape-hatch workflows.

## Implementation beads

The implementation is tracked under epic `go-slide-creator-m0jg` with child beads `go-slide-creator-m0jg.1` through `go-slide-creator-m0jg.14`.
