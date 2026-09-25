# MCP tool routing

Use the live `tools/list` descriptions and input schemas for signatures;
`get_capabilities().mcp_tools_available` is the current searchable catalog.
The default core profile advertises a compact subset. Other registered tools
are still callable by name; `json2pptx mcp --tools all` advertises the full
set. Do not infer that a tool is unavailable merely because it is absent
from the core listing.

## Workflow contract

<!-- workflow-contract:start -->
Default for content-bearing decks: author real content as a DeckSpec; call `list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`. Revise a DeckSpec with `deck_id` + `patch` on `validate_deck_spec` / `render_deck_spec`. Use the raw PresentationInput path only when the spec cannot express a needed feature or the source deck is already raw; on that path, inspect chosen patterns with `show_pattern` / `expand_pattern`, then call `validate_input` (with `fit_report: true`) before `generate_presentation`. `make_deck` creates an exemplar skeleton, never a publishable deck. On either path, render every slide of the final revision and inspect its image before approval.
<!-- workflow-contract:end -->

## Phase map

| Need | Start with | Continue with |
|---|---|---|
| Server/session state | `get_started` | `get_capabilities`, `get_input_schema`, `get_data_format_hints` |
| Templates and palette | `list_templates` | `examine_template`, `resolve_theme` |
| New semantic deck | `list_deck_archetypes`, `list_slide_kinds` | `explain_deck_spec`, `validate_deck_spec`, `render_deck_spec` |
| Semantic escape hatch | `compile_deck_spec` | `validate_input`, `generate_presentation` |
| Raw planning | `recommend_visual` | `plan_deck`, `recommend_pattern`, `analyze_deck_rhythm` |
| Raw pattern | `list_patterns`, `show_pattern` | `validate_pattern`, `expand_pattern`, `expand_patterns` |
| Raw schema and visual inputs | `get_shape_catalog`, `list_icons`, `preview_icon` | `get_chart_capabilities`, `get_diagram_capabilities`, `table_density_guide` |
| Raw preflight/generation | `validate_input` | `preview_presentation_plan`, `preview_slide_wireframe`, `generate_presentation` |
| Repair and quality | `describe_finding`, `score_deck` | `score_candidates`, `propose_repairs`, `repair_slide`, `repair_slides_batch`, `apply_deck_patch`, `auto_repair` |
| Rendered inspection | `render_deck_thumbnails` | `render_slide_image`, `render_slide_image_from_json`, `inspect_slide_images`, `submit_visual_review` |
| Existing artifact | `read_presentation`, `validate_presentation_output` | `audit_palette`, `export_deck`, `purge_render_cache` |
| Settings (gated write) | `list_template_settings` | `register_template_setting`, `delete_template_setting` |
| Wireframe facade | `make_deck` | Replace exemplar content; never ship it directly |

For a new deck, use [DECKSPEC.md](DECKSPEC.md); for raw-input-only
preconditions, use [RAW_PATH.md](RAW_PATH.md). The four-phase deep dive is in
[WORKFLOW.md](WORKFLOW.md). The list above is a routing aid, not a second
signature or schema registry.

## Tool semantics that change decisions

- `get_started.runtime.render_available` tells you whether this server can
  render slides. Missing render tooling means the artifact is unreviewed.
  `get_capabilities` exposes schema version, feature flags, tool
  classification, and the current core-profile membership.
- `list_templates` defaults to compact data and may write layout-preview
  cache files; use `read_only:true` for side-effect-free discovery.
  `list_templates`, `list_patterns`, and `list_icons` support filtering,
  pagination, and compact/full projection. Fetch full data only when needed.
- `get_input_schema` and `get_data_format_hints` support digest reuse.
  `list_slide_kinds`, `show_pattern`, and `describe_finding` are the live
  kind, pattern, and finding catalogs. Prefer them to static enumerations.
- `preview_slide_wireframe` is structural-only and cannot prove visual fit.
  `render_deck_thumbnails` returns images; rendering alone is not inspection.
  `submit_visual_review` requires current-revision evidence for every slide.
- `score_deck` grades raw decks; do not pass a DeckSpec. A passing
  `quality_gate` stops score-driven repair, not visual review. When a render
  fails, `evidence_complete:false` or `render_evidence` prevents a clean
  verdict. `auto_repair` is a raw-deck convergence facade; `make_deck`
  produces a nonpublishable exemplar skeleton.
- `register_template_setting` and `delete_template_setting` are write
  tools gated by `JSON2PPTX_ALLOW_SETTINGS_WRITE=1`. They are never part of
  ordinary deck authoring. `get_capabilities().cli_only_commands` explains
  commands without an MCP counterpart. MCP-only tools have no exact CLI
  replacement; prefer composition where documented below.
- Results use compact `structuredContent`. On newer MCP protocol versions,
  `content[0].text` may be a bounded synopsis, so inspect structured data
  instead of parsing a prose fallback. A tool's `next_tool_call` is a
  suggested recovery path, not permission to mutate external state.

## Composition recipes

The CLI-only `preview-patterns` command builds a batch gallery. For one
pattern preview in an MCP-only client:

1. `list_patterns` to select a pattern.
2. `show_pattern` to get its value schema and `example_values`.
3. `expand_pattern` with those values and the chosen template.
4. `render_slide_image_from_json` with the expanded grid.

Loop only over the patterns/templates you actually need. For a comparison
under one template, `expand_patterns` can replace repeated expansion calls.
For path-targeted template conformance, the CLI `template-check` remains
the authoring/CI command; `examine_template` provides read-only MCP
capability inspection, not that conformance verdict.
