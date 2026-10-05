---
name: generate-deck
schema_version: 4.169.0
description: >-
  Create or revise PowerPoint decks with json2pptx. Use for presentation and
  slide-deck requests that need template-aware authoring, validation, rendering,
  and visual review.
---

# Deck Generation Skill

**Visual standard:** Apply [native slide composition](RULES.md#native-slide-composition).

Call `get_started` first, passing this frontmatter's `schema_version` as
`skill_version`. If it returns `skill_warning`, run `json2pptx skill install`
before relying on installed instructions (`json2pptx skill status` checks every
file). Check `get_started.runtime`: if render
tooling is unavailable, deliver the PPTX as **UNREVIEWED**, not as a finished deck.
Use the live MCP schemas for arguments (the default `tools/list` is abridged:
`get_started` `tool:"<name>"` returns a tool's full description and schema);
do not infer a tool's signature from an old example.

**Completion rule (single source — same text as `get_started.completion_protocol.rule` and the MCP
server `instructions`):** A deck is done only after every slide of the CURRENT revision has been rendered (render_deck_thumbnails) and looked at by you. A passing deterministic gate, score, or validate result is a precondition for that review, never completion. After a repair, re-render and re-inspect the slides that changed (render_deck_thumbnails with slide_indices), then make one full-deck pass over the final revision: the revision you ship is the one that has to have been seen.

**Must-read before authoring:** [QUALITY.md](QUALITY.md) — ghost deck of
titles first, full-sentence action titles (≤15 words, carrying the number),
one message per slide, a `takeaway` and `source` on every evidence slide
(deck-wide default: DeckSpec `meta.source` / raw top-level `source`), and the
message → visual table. `score_deck`'s gate enforces the storyline
(`takeaway_missing`, `TITLE_NOT_ACTION`, `NO_EXECUTIVE_SUMMARY`,
`CLOSING_WITHOUT_NEXT_STEPS`, `DATA_WITHOUT_SOURCE`, `SLIDE_TEXT_DENSE`:
[FINDINGS.md](FINDINGS.md)). A deck that passes every gate with topic titles
is not finished. Back matter: `sections[].appendix: true` or raw
`section_number: false|"A"`. Keep `accent_weight: "strong"` patterns
(`list_patterns` full) to two in a row.

**One conclusion band per slide:** `takeaway` is the fallback for kinds with
their own conclusion field — on `executive_summary` write `bottom_line`, on
`decision` write `recommendation`, and leave `takeaway` out (a duplicate goes
to the speaker notes, `SEMANTIC_DUPLICATE_CALLOUT`).

## Choose the authoring path

<!-- workflow-contract:start -->
Default for content-bearing decks: author real content as a DeckSpec; call `list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`. Revise a DeckSpec with `deck_id` + `patch` on `validate_deck_spec` / `render_deck_spec`. Use the raw PresentationInput path only when the spec cannot express a needed feature or the source deck is already raw; on that path, inspect chosen patterns with `show_pattern` / `expand_pattern`, then call `validate_input` (with `fit_report: true`) before `generate_presentation`. `make_deck` creates an exemplar skeleton, never a publishable deck. On either path, render every slide of the final revision and inspect its image before approval.
<!-- workflow-contract:end -->

For a new content-bearing deck, write a semantic **DeckSpec** (`meta` plus
`slides[].kind`, or chapter-based `structure`). `get_started(task:"brief")`
returns this path as its `sequence`, written for the tools the server lists
(`hidden_tools` names the callable rest). `plan_deck` with
`format:"deckspec"` drafts the storyline. `list_slide_kinds` lists the kinds;
`kinds:[chosen]` returns their copy-ready examples, `fields` their
signatures and text budgets (DECKSPEC.md) and `preview: true` each example
as a rendered image. Then call `validate_deck_spec`,
`render_deck_spec`, `render_deck_thumbnails`, and `submit_visual_review`.
Edit the spec at a finding's `path`, send a `patch_verified` `next_tool_call`
as given, and repeat ([FINDINGS.md](FINDINGS.md)). For a visual no kind
covers (gantt, venn, swimlane, ...), a `recommend_visual` candidate carries
`data_contract` and a runnable `next_tool_call` (`render_deck_spec` with a
`raw_json2pptx` slide); rewrite its sample content first (TOOLS.md).
One measure across 2–6 groups over the same
periods: the `small_multiples` chart (RULES.md 10i). Several views proving
one title may share a slide; unrelated conclusions get separate slides.
Prefer `kind: regions`; only when its region kinds cannot express the
slide, make that one slide `raw_json2pptx` ([WORKFLOW.md](WORKFLOW.md) →
spatial planning). Read [DECKSPEC.md](DECKSPEC.md) for budgets,
degradation, required-layout coverage, handles, and revision rules.

Use raw `PresentationInput` only for a feature the semantic schema cannot
express, a targeted low-level repair, or an existing raw deck. Read
[RAW_PATH.md](RAW_PATH.md) first; its preconditions are **not** universal
DeckSpec requirements. The raw path is `recommend_visual` (when visual choice
is unclear) → `list_patterns` / `show_pattern` → `expand_pattern` →
`validate_input` → `generate_presentation` → render and inspect;
`get_input_schema` has the raw fields. How patterns fit text, shrink, and
report `BODY_TOO_LONG` / `TEXT_EXCEEDS_SHAPE` is in [PATTERNS.md](PATTERNS.md);
patterns spend at most one solid accent block, in `color_roles.primary_fill`.

Both render tools return `deterministic_ready` (a precondition, not proof
anybody looked), `publishable` (false on a fresh render), `blocking_reasons`
and `next_tool_call` (blocking fix, else `render_deck_thumbnails` →
`submit_visual_review` with `pptx_revision` = `content_hash`). Submit a
verdict only after inspecting each current-revision image; it never clears
blockers. A changed slide invalidates its verdict; image identity
(`content_hash`, `image_sha256`) is a pixel hash, so re-rendering an
unchanged revision keeps earlier images valid. For an unfamiliar finding call
`describe_finding`; use
`get_capabilities().vocabularies.repair_fix_kinds` to distinguish executable
repairs from advice.

## References, loaded only when relevant

- [QUALITY.md](QUALITY.md): storyline and visual choice (always read).
- [DECKSPEC.md](DECKSPEC.md): semantic authoring, content budgets, degradation,
  chapter structure, footer chrome (`CHROME_TRUNCATED`), required layouts,
  and spec-level iteration.
- [WORKFLOW.md](WORKFLOW.md): Plan → Vary → Render → Repair, the per-slide
  review rubric (recorded in `submit_visual_review`), and the three-round
  repair cap. Read before the first render.
- [RAW_PATH.md](RAW_PATH.md): raw preconditions, planning and rhythm, strict
  output validation, repair, images and assets, SVG/diagram integration.
- [TOOLS.md](TOOLS.md): phase map, tool-profile discovery, MCP-only
  operations, and composition recipes.
- [RULES.md](RULES.md): shape-grid, chart, table, content, contrast,
  typography, and anti-pattern rules.
- [PATTERNS.md](PATTERNS.md): pattern selection, placement, text capacity and
  accent defaults; the live catalog and schemas come from `list_patterns` /
  `show_pattern`.
- [FINDINGS.md](FINDINGS.md): how to read and repair a finding (DeckSpec and
  raw).
- [../template-deck/TEMPLATE_GUIDE.md](../template-deck/TEMPLATE_GUIDE.md):
  template/layout fields and raw slide structure.

Examples: [semantic specs](../../examples/semantic/),
[raw skeletons](examples/skeletons/README.md), and the
[layout-coverage example](examples/layout-coverage-deckspec.md).

## Operational boundaries

Start the deck server with `json2pptx mcp` (profiles: TOOLS.md; `--tools all`
lists every tool, `get_capabilities().mcp_tools_available` the callable ones).
`list_templates` `read_only: true` skips cache writes. Template precedence:
DECKSPEC.md. A CLI command that answers in JSON prints exactly one JSON
document on stdout, also when it fails (`{subcommand, ok:false,
findings:[{code, message}]}`; a failed `generate` reports the findings
`validate` reports, else `STRICT_FIT` / `GENERATION_FAILED`); stderr is logs.
Args over 2 MiB of text are refused; a raced
`deck_id` patch returns `STALE_REVISION` (reload, re-apply).
`render_deck_spec` resolves relative asset paths (image, photos) against
`base_dir` and fetches `url` assets; a missing asset fails the render. A
`template_path` + `base_dir` render is kept on the `deck_id` for later calls.

Responses are always compact JSON; the server still advertises `experimental.compact_responses: {}` and still honours the client capability and the deprecated `MCP_COMPACT_RESPONSES=1` environment variable, but neither changes anything.
