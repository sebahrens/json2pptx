---
name: generate-deck
schema_version: 4.142.0
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
server `instructions`):** A deck is done only after every slide of the CURRENT revision has been rendered (render_deck_thumbnails) and looked at by you. A passing deterministic gate, score, or validate result is a precondition for that review, never completion. After a repair, re-render and re-inspect the slides that changed (render_deck_thumbnails with slide_indices, or render_slide_image for a single one), then make one full-deck pass over the final revision: the revision you ship is the one that has to have been seen.

## Choose the authoring path

<!-- workflow-contract:start -->
Default for content-bearing decks: author real content as a DeckSpec; call `list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`. Revise a DeckSpec with `deck_id` + `patch` on `validate_deck_spec` / `render_deck_spec`. Use the raw PresentationInput path only when the spec cannot express a needed feature or the source deck is already raw; on that path, inspect chosen patterns with `show_pattern` / `expand_pattern`, then call `validate_input` (with `fit_report: true`) before `generate_presentation`. `make_deck` creates an exemplar skeleton, never a publishable deck. On either path, render every slide of the final revision and inspect its image before approval.
<!-- workflow-contract:end -->

For a new content-bearing deck, write a semantic **DeckSpec** (`meta` plus
`slides[].kind`, or chapter-based `structure`). Discover available kinds with
`list_slide_kinds` using its compact fields; request `item_schema` and
`compositions` only for selected kinds. Then call `validate_deck_spec`,
`render_deck_spec`, and `render_deck_thumbnails`. Edit the spec at a finding's
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
a circular picture frame.

For both paths, a passing `quality_gate` or `deterministic_ready` field is a
precondition, not proof that anybody looked at the slides. On a fresh semantic
render, `publishable` remains false until a complete approved visual verdict
for the current artifact exists. For a recorded publishable verdict, use
`submit_visual_review` only after inspecting each current-revision image.
Any changed slide invalidates its previous visual verdict. If a finding is
unfamiliar, call `describe_finding`; use
`get_capabilities().vocabularies.repair_fix_kinds` to distinguish executable
repairs from advice. Do not retry an advisory fix kind as though it were an
executable one.

## References, loaded only when relevant

- [DECKSPEC.md](DECKSPEC.md): semantic authoring, content budgets, degradation,
  chapter structure, required layouts, and spec-level iteration.
- [RAW_PATH.md](RAW_PATH.md): raw `PresentationInput` preconditions, strict
  output validation, repair, assets, and SVG/diagram integration.
- [TOOLS.md](TOOLS.md): concise phase map, tool-profile discovery, MCP-only
  operations, and composition recipes.
- [WORKFLOW.md](WORKFLOW.md): detailed Plan → Vary → Render → Repair workflow,
  visual inspection, resumable calls, and idempotency.
- [RULES.md](RULES.md): shape-grid, content, contrast, typography, and
  anti-pattern rules.
- [PATTERNS.md](PATTERNS.md): pattern selection and text-capacity guidance
  (including tier-rated `capability-heatmap`, labelled-row `framework-grid`
  and per-pair-count `state-shift-hub` budgets); get the current catalog and per-pattern schema from
  `list_patterns` / `show_pattern`.
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

Start the deck server with `json2pptx mcp`; its default core `tools/list`
profile is deliberately compact. `get_capabilities().mcp_tools_available`
identifies the current core set and tools callable by name in this session.
For full discovery, use `--tools all` or `JSON2PPTX_MCP_TOOLS=all`. If
`list_templates` preview-cache writes are undesirable during discovery,
pass `read_only: true`.

Responses are always compact JSON; the server still advertises `experimental.compact_responses: true` and still honours the client capability and the deprecated `MCP_COMPACT_RESPONSES=1` environment variable, but neither changes anything.
