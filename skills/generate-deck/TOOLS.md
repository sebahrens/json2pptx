# MCP tool routing

Use the live `tools/list` descriptions and input schemas for signatures;
`get_capabilities().mcp_tools_available` is the current searchable catalog.
The default `deckspec` profile advertises only the DeckSpec path;
`--tools core` (alias `raw`) adds the raw-JSON path and `--tools all` the
rest. Hidden tools stay callable by name, and a response that mentions one
lists it in `hidden_tools`. Folded aliases are never listed:
`recommend_pattern` → `recommend_visual`, `render_slide_image` →
`render_deck_thumbnails` `slide_indices`, `repair_slides_batch` →
`repair_slide`, `get_chart_capabilities` / `get_diagram_capabilities` →
`list_templates` `fields:"full"`.

## Workflow contract

<!-- workflow-contract:start -->
Default for content-bearing decks: author real content as a DeckSpec; call `list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`. Revise a DeckSpec with `deck_id` + `patch` on `validate_deck_spec` / `render_deck_spec`. Use the raw PresentationInput path only when the spec cannot express a needed feature or the source deck is already raw; on that path, inspect chosen patterns with `show_pattern` / `expand_pattern`, then call `validate_input` (with `fit_report: true`) before `generate_presentation`. `make_deck` creates an exemplar skeleton, never a publishable deck. On either path, render every slide of the final revision and inspect its image before approval.
<!-- workflow-contract:end -->

## Phase map

| Need | Start with | Continue with |
|---|---|---|
| Server/session state | `get_started` | `get_capabilities`, `get_input_schema`, `get_data_format_hints` |
| Templates and palette | `list_templates` (`fields:"names"` to pick one) | `examine_template`, `resolve_theme` |
| New semantic deck | `plan_deck` (`format:"deckspec"`), `list_slide_kinds` | `list_deck_archetypes`, `explain_deck_spec`, `validate_deck_spec`, `render_deck_spec` |
| Semantic escape hatch | `compile_deck_spec` | `validate_input`, `generate_presentation` |
| Raw planning | `recommend_visual` | `plan_deck`, `analyze_deck_rhythm` |
| Raw pattern | `list_patterns`, `show_pattern` | `validate_pattern`, `expand_pattern`, `expand_patterns` |
| Raw schema and visual inputs | `get_shape_catalog`, `list_icons`, `preview_icon` | `table_density_guide` |
| Raw preflight/generation | `validate_input` | `preview_presentation_plan`, `preview_slide_wireframe`, `generate_presentation` |
| Repair and quality | `describe_finding`, `score_deck` | `score_candidates`, `propose_repairs`, `repair_slide`, `apply_deck_patch`, `auto_repair` |
| Rendered inspection | `render_deck_thumbnails` | `render_slide_image_from_json`, `inspect_slide_images`, `submit_visual_review` |
| Existing artifact | `read_presentation`, `validate_presentation_output` | `audit_palette`, `export_deck`, `purge_render_cache` |
| Settings (gated write) | `list_template_settings` | `register_template_setting`, `delete_template_setting` |
| Wireframe facade | `make_deck` | Replace exemplar content; never ship it directly |

For a new deck, use [DECKSPEC.md](DECKSPEC.md); for raw-input-only
preconditions, [RAW_PATH.md](RAW_PATH.md); the four phases are in
[WORKFLOW.md](WORKFLOW.md).

## Tool semantics that change decisions

- `get_started.runtime.render_available` tells you whether this server can
  render slides. Missing render tooling means the artifact is unreviewed.
  `get_capabilities` exposes schema version, feature flags, tool
  classification, core-profile membership, each repair fix kind's params
  (`vocabularies.repair_fix_kind_params`) and, with `output_schema:"<tool>"`,
  one tool's result schema (`tools/list` carries none unless the server runs
  `--tools all --output-schemas`).
- `list_templates` defaults to compact data and may write layout-preview
  cache files; use `read_only:true` for side-effect-free discovery.
  `list_templates`, `list_patterns`, and `list_icons` support filtering,
  pagination, and compact/full projection. Fetch full data only when needed.
- `list_slide_kinds` and `recommend_visual` take `preview: true`: one image
  per named kind (1–4 `kinds`; its `example` on `template`) or per leading
  candidate (max 4; name them in `candidates`), rendered as
  `render_deck_spec` renders it. `previews[] {name, template,
  image_content_index, content_hash, error}` maps names to images; an entry
  with `error` has none. Without it `recommend_visual` returns
  `preview_call`. No pattern preview is a file path.
- `recommend_visual` `content_hints` / `recent_patterns` / `candidates` and
  `plan_deck` `must_include` are type-checked: a malformed value returns
  `INVALID_PARAMETER` naming the argument instead of being ignored.
  Every candidate — pattern, layout, chart, diagram, compose,
  `raw_shape_grid` — carries `data_contract` (`form`: `deckspec_kind` |
  `raw_json2pptx`, `required_keys`, `optional_keys`, `limits[] {path,
  min_items, max_items, max_chars}`, `field_path`) and a runnable
  `next_tool_call` (`render_deck_spec`, a one-slide DeckSpec). When a kind
  compiles to the candidate it has `deckspec {kind, fields, aliases,
  example_path}` and the call renders that kind (`table-highlight` →
  `option_matrix`, `org_chart` → `org`, charts → `chart_insight`): write the
  canonical field, not a key listed in `aliases`. No `deckspec` means
  raw-only: the slide is `raw_json2pptx`. Copy the slide, keep its shape and
  rewrite every `Replace with …` string, the alt text and the sample source:
  a recipe rendered verbatim is blocked (`SEMANTIC_WEAK_CONTENT`,
  `exemplar_content`). `also_as[] {form, name, differs}` names the other
  forms of the same visual (pattern `matrix-2x2` ↔ diagram `matrix_2x2`);
  `differs_by` is one line on candidates within 0.02 of another — read it
  before picking between near ties.
  An explicit same-slide brief ("line chart left 65%, KPI upper-right,
  timeline lower-right") returns the `regions` kind first (category
  `deckspec_kind`) and the raw `compose:` form of the same slide just below
  it. A `compose` candidate carries a `composition` (`direction`,
  `regions[]` with `category`, `name`, `size_pct`, `position`,
  `segment_path`, nested `compose` and a per-region `data_contract`).
  `compose:<a>+<b>` names (charts / diagrams as `chart:<type>` /
  `diagram:<type>`) resolve in `candidates`; a malformed one returns score 0
  and the reason.
  The visual an intent names (swimlane, Gantt, a chart type, a screenshot
  with callouts) ranks first.
- `validate_deck_spec` `templates: [...]` (or `["all"]`, at most 16) adds
  `template_results[{template, ok, summary, findings}]` — the spec's errors
  and warnings on each — without changing the deck's template. Its `read`
  argument reads a stored deck instead of validating (DECKSPEC.md).
  `compile_deck_spec` answers `ok: false` when a diagnostic blocks.
- `get_input_schema` and `get_data_format_hints` support digest reuse.
  `list_slide_kinds` (DECKSPEC.md lists its `fields`), `show_pattern`, and
  `describe_finding` are the live kind, pattern, and finding catalogs.
  Prefer them to static enumerations.
- `preview_slide_wireframe` is structural-only and cannot prove visual fit.
  `render_deck_thumbnails` returns JPEG blocks (`image_mime_type`, top
  level) beside full-resolution PNG `slides[].path`; review at the default
  density and, when text of 12pt or less cannot be read, send its
  `larger_render` `{when, tool, args_template}` (one slide at `density:
  100`, 1280 px). A render step's `next_tool_call` carries `known_hashes`
  on a repeat pass (WORKFLOW.md). `slide_indices` takes slide ids beside
  0-based indices (`[4, "costs"]`) for a file `render_deck_spec` or CLI
  `semantic render` wrote, and each slide carries `id`. Rendering alone is
  not inspection.
  `submit_visual_review` requires current-revision evidence for every slide.
- `score_deck` takes a DeckSpec `deck_id` or a raw deck, never a spec
  object. `composition.diagnostics[].code` includes `motif_run`,
  `motif_dominance` and `continuation_interrupted`. A passing
  `quality_gate` stops score-driven repair, not visual review. When a render
  fails, `evidence_complete:false` or `render_evidence` prevents a clean
  verdict. `auto_repair` is a raw-deck convergence facade; `make_deck`
  produces a nonpublishable exemplar skeleton.
- `register_template_setting` and `delete_template_setting` are write
  tools gated by `JSON2PPTX_ALLOW_SETTINGS_WRITE=1`, never part of ordinary
  deck authoring. `json2pptx skill cli-map` prints each tool's CLI command
  (generated; an MCP-only tool says why; `cli_then` is a second command:
  `semantic kinds`, then `semantic kinds <kind>`), and `json2pptx
  get-started --tool <name>` one tool's description, schema and `cli`.
- Results use compact `structuredContent`; `content[0].text` may be a
  bounded synopsis, so read the structured data. A tool's `next_tool_call`
  is a suggested recovery path (`{tool, args_template}`; `slide_index: -1`
  means you supply it), not permission to mutate external state.
  `generate_presentation`, `auto_repair` and `make_deck` accept an
  `idempotency_key` retry token (a changed input under the same key is
  `IDEMPOTENCY_CONFLICT`). Details: `docs/AGENT_DIAGNOSTICS.md` §7.

## Composition recipes

For one pattern's picture in an MCP client, call `recommend_visual(intent,
template, candidates: [<pattern>], preview: true)`: it returns the pattern's
recipe rendered through generation, as an image. The CLI-only
`preview-patterns` builds the same pictures as a local batch gallery
(`-template`, `-pattern` narrow it; nothing is committed). The raw-JSON
equivalent: `list_patterns` → `show_pattern` (`example_values`) →
`expand_pattern` on the template → `render_slide_image_from_json`.

For a comparison under one template, `expand_patterns` can replace repeated
expansion calls. For template conformance, the CLI `template-check` remains
the authoring/CI command; `examine_template` is read-only MCP inspection.
