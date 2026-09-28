---
name: generate-deck
schema_version: 4.153.0
description: >-
  Create or revise PowerPoint decks with json2pptx. Use for presentation and
  slide-deck requests that need template-aware authoring, validation, rendering,
  and visual review.
---

# Deck Generation Skill

Call `get_started` first, passing this frontmatter's `schema_version` as
`skill_version`. If it returns `skill_warning`, run `make install-skill`
before relying on installed instructions. Check `get_started.runtime`: if render
tooling is unavailable, deliver the PPTX as **UNREVIEWED**, not as a finished deck.
Use the live MCP schemas and `get_capabilities` for arguments and availability;
do not infer a tool's signature from an old example.

**Completion rule (single source — same text as `get_started.completion_protocol.rule` and the MCP
server `instructions`):** A deck is done only after every slide of the CURRENT revision has been rendered (render_deck_thumbnails) and looked at by you. A passing deterministic gate, score, or validate result is a precondition for that review, never completion. After a repair, re-render and re-inspect the slides that changed (render_deck_thumbnails with slide_indices), then make one full-deck pass over the final revision: the revision you ship is the one that has to have been seen.

**Must-read before authoring:** [QUALITY.md](QUALITY.md) — ghost deck of
titles first, full-sentence action titles (≤15 words, carrying the number),
one message per slide, a `takeaway` and `source` on every evidence slide, and
the message → visual table. A deck that passes every gate with topic titles
is not finished.

## Choose the authoring path

<!-- workflow-contract:start -->
Default for content-bearing decks: author real content as a DeckSpec; call `list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`. Revise a DeckSpec with `deck_id` + `patch` on `validate_deck_spec` / `render_deck_spec`. Use the raw PresentationInput path only when the spec cannot express a needed feature or the source deck is already raw; on that path, inspect chosen patterns with `show_pattern` / `expand_pattern`, then call `validate_input` (with `fit_report: true`) before `generate_presentation`. `make_deck` creates an exemplar skeleton, never a publishable deck. On either path, render every slide of the final revision and inspect its image before approval.
<!-- workflow-contract:end -->

For a new content-bearing deck, write a semantic **DeckSpec** (`meta` plus
`slides[].kind`, or chapter-based `structure`). `get_started(task:"brief")`
returns this path as its `sequence` (the raw chain is `raw_sequence`):
`plan_deck` with `format:"deckspec"` drafts the storyline as a DeckSpec
(a kind per narrative slot, brief facts routed to each); discover kinds with
`list_slide_kinds` using its compact fields, requesting `item_schema` and
`compositions` only for selected kinds. Then call `validate_deck_spec`,
`render_deck_spec`, `render_deck_thumbnails`, and `submit_visual_review`. Edit the spec at a finding's
`semantic_path` and repeat. `make_deck` creates an exemplar-filled wireframe,
not a publishable authored deck. Read [DECKSPEC.md](DECKSPEC.md) for budgets,
degradation behavior, required-layout coverage, handles, and revision rules.

Use raw `PresentationInput` only for a feature the semantic schema cannot
express, a targeted low-level repair, or an existing raw deck. Discover
patterns with `list_patterns` and the chosen pattern's live value schema with
`show_pattern`; use `get_input_schema` for raw fields. The raw path is
`recommend_visual` (when visual choice is unclear) → `expand_pattern` (when
using a pattern) → `validate_input` → `generate_presentation` → render and
inspect. Read [RAW_PATH.md](RAW_PATH.md) before authoring raw JSON. Its
preconditions are **not** universal DeckSpec requirements. Two raw-only
patterns cover pages DeckSpec kinds do not: `contact-directory` (key contacts
/ "who to call": grouped rows of circular headshots, names and titles, up to
24 people) and `text-sidebar` (prose introduction or foreword beside one large
key-message panel). A shape-grid `image` cell accepts `geometry: "ellipse"` for
a circular picture frame. Asset paths expand only `$HOME`, `$BRAND_ASSETS`,
`$JSON2PPTX_*`; `icon.path` obeys `ALLOWED_IMAGE_PATHS`.

Both render tools return `deterministic_ready`, `publishable` (false on a fresh
render), `blocking_reasons` and `next_tool_call` (blocking fix, else
`render_deck_thumbnails` → `submit_visual_review` with `pptx_revision` =
`content_hash`). `deterministic_ready` is a precondition, not proof anybody
looked: submit a verdict only after inspecting each current-revision image; it
never clears deterministic blockers (`reviewed_deterministic_blockers`).
Any changed slide invalidates its previous visual verdict. If a finding is
unfamiliar, call `describe_finding`; use
`get_capabilities().vocabularies.repair_fix_kinds` to distinguish executable
repairs from advice. Do not retry an advisory fix kind as though it were an
executable one.

## References, loaded only when relevant

- [QUALITY.md](QUALITY.md): storyline, action titles, one message per slide,
  takeaways, sources, and choosing the visual from the message (always read).
- [DECKSPEC.md](DECKSPEC.md): semantic authoring, content budgets, degradation,
  chapter structure, required layouts, and spec-level iteration.
- [RAW_PATH.md](RAW_PATH.md): raw `PresentationInput` preconditions, raw
  planning and rhythm, strict output validation, repair, assets, and
  SVG/diagram integration.
- [TOOLS.md](TOOLS.md): concise phase map, tool-profile discovery, MCP-only
  operations, and composition recipes.
- [WORKFLOW.md](WORKFLOW.md): Plan → Vary → Render → Repair on the DeckSpec
  path, the per-slide review rubric (recorded in `submit_visual_review`), and
  the three-round repair cap. Read before the first render.
- [RULES.md](RULES.md): shape-grid, content, contrast, typography, and
  anti-pattern rules. Waterfall `type` sets the sign
  (`chart.waterfall_total_mismatch`). Charts show the title's point: a
  single-series bar chart (and `horizontal-bar-with-callouts`) is neutral
  grey with accent1 only on `highlight` bars (0-based indices or names;
  default the last period of a time series, else the top bar); set it to the
  bar(s) the title names. Waterfalls accent the decreases.
  A table with no `style` renders as a
  consulting table (unfilled 11pt bold header over a 1pt rule, 12pt rows,
  hairline rules, no zebra); `table-highlight` matches it.
- [PATTERNS.md](PATTERNS.md): pattern selection and text-capacity guidance;
  get the current catalog, per-pattern schema and per-field copy targets from
  `list_patterns` / `show_pattern`. Out-of-range pattern text sizes are
  rejected, not clamped.
- [FINDINGS.md](FINDINGS.md): legacy finding and fix details for cases not yet
  covered by `describe_finding`; prefer the live tool for known codes.
- [../template-deck/TEMPLATE_GUIDE.md](../template-deck/TEMPLATE_GUIDE.md):
  template/layout fields and raw slide structure.

Examples: [semantic specs](../../examples/semantic/),
[raw skeletons](examples/skeletons/README.md), and the
[layout-coverage example](examples/layout-coverage-deckspec.md).
For a standalone SVG diagram, use the `svggen-mcp` server and its
`get_started` / `get_diagram_schema`; this skill owns deck assembly.

## Operational boundaries

Start the deck server with `json2pptx mcp`; its default `deckspec`
`tools/list` carries only the DeckSpec path. `--tools core` adds the raw
path and `get_capabilities().mcp_tools_available` (tools callable by name);
`--tools all` (or `JSON2PPTX_MCP_TOOLS=all`) lists the rest. If
`list_templates` preview-cache writes are undesirable during discovery,
pass `read_only: true`. Args over 2 MiB of text are refused; a raced
`deck_id` patch returns `STALE_REVISION` (reload, re-apply).

Responses are always compact JSON; the server still advertises `experimental.compact_responses: {}` and still honours the client capability and the deprecated `MCP_COMPACT_RESPONSES=1` environment variable, but neither changes anything.
