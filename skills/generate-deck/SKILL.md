---
name: generate-deck
schema_version: 5.1.0
description: >-
  Create or revise PowerPoint decks with json2pptx. Apply template-aware
  authoring, validation, rendering and visual review.
---

# Deck Generation Skill

**Visual standard:** Apply [native slide composition](RULES.md#native-slide-composition).

Call `get_started(skill_version:"5.1.0")` first. On `skill_warning`,
run `json2pptx skill install`; `skill status` checks installed files.
Without runtime render tooling, deliver as **UNREVIEWED**. Use live schemas:
`get_started(tool:"<name>")` returns a full schema; `tools/list` is abridged.

**Completion:** Render and inspect every slide of the CURRENT revision with `render_deck_thumbnails`. A passing gate, score or validation is only a precondition. After repairs, re-render and inspect changed slides with `slide_indices`, then make one full-deck pass over the revision you ship.

**Read before authoring:** [QUALITY.md](QUALITY.md) — ghost deck of
titles first, full-sentence action titles (≤15 words, carrying the number),
one message per slide, a `takeaway` and `source` on every evidence or
figure-comparison slide (deck default: DeckSpec `meta.source` / raw top-level `source`), and the
message → visual table. `score_deck`'s gate enforces the storyline
(`takeaway_missing`, `TITLE_NOT_ACTION`, `NO_EXECUTIVE_SUMMARY`,
`CLOSING_WITHOUT_NEXT_STEPS`, `DATA_WITHOUT_SOURCE`, `SLIDE_TEXT_DENSE`:
[FINDINGS.md](FINDINGS.md)). A deck that passes every gate with topic titles
is not finished. Back matter: `sections[].appendix: true` or raw
`section_number: false|"A"`. Keep `accent_weight: "strong"` patterns
(`list_patterns` full) to two in a row.

**One conclusion band:** use `bottom_line` on `executive_summary`,
`recommendation` on `decision`; omit duplicate `takeaway`
(`SEMANTIC_DUPLICATE_CALLOUT`, retained in notes).

## Choose the authoring path

<!-- workflow-contract:start -->
Default for content-bearing decks: author real content as a DeckSpec; call `list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`. Revise a DeckSpec with `deck_id` + `patch` on `validate_deck_spec` / `render_deck_spec`. Use the raw PresentationInput path only when the spec cannot express a needed feature or the source deck is already raw; on that path, inspect chosen patterns with `show_pattern` / `expand_pattern`, then call `validate_input` (with `fit_report: true`) before `generate_presentation`. `make_deck` creates an exemplar skeleton, never a publishable deck. On either path, render every slide of the final revision and inspect its image before approval.
<!-- workflow-contract:end -->

A **DeckSpec** uses `meta` and `slides[].kind`, or chaptered `structure`.
`get_started(task:"brief")` returns the `sequence` and callable `hidden_tools`. `plan_deck` with
`format:"deckspec"` drafts the storyline. Use `list_deck_archetypes` and
`explain_deck_spec` to plan its structure and check its rhythm.
`list_slide_kinds` lists the kinds;
`kinds:[chosen]` returns their copy-ready examples, `fields` their
signatures and text budgets (DECKSPEC.md) and `preview: true` each example
as a rendered image. Then call `validate_deck_spec`,
`render_deck_spec`, `render_deck_thumbnails`, and `submit_visual_review`.
Edit the spec at a finding's `path`, send a `patch_verified` `next_tool_call`
as given, and repeat ([FINDINGS.md](FINDINGS.md)). For a visual no kind
covers (gantt, swimlane, ...), a `recommend_visual` candidate carries
`data_contract` and a runnable `next_tool_call` (`render_deck_spec` with a
`raw_json2pptx` slide); rewrite its sample content first (TOOLS.md).
Chart callouts avoid labels or refuse. Stakeholder maps protect axis labels.
One measure across 2–6 groups, same periods: `small_multiples`
(RULES.md 10i). Share a slide only for views proving one title.
Prefer `kind: regions`; only when its region kinds cannot express the
slide, make that one slide `raw_json2pptx` ([WORKFLOW.md](WORKFLOW.md) →
spatial planning). Read [DECKSPEC.md](DECKSPEC.md) for budgets,
degradation, required-layout coverage, handles, and revision rules.

Use raw `PresentationInput` only for a feature the semantic schema cannot
express, a targeted low-level repair, or an existing raw deck. Read
[RAW_PATH.md](RAW_PATH.md) first; its preconditions are **not** universal
DeckSpec requirements. The raw path is `recommend_visual` (visual choice
unclear) → `list_patterns` / `show_pattern` → `expand_pattern` →
`validate_input` → `generate_presentation` → render and inspect;
`get_input_schema` has the raw fields. How patterns fit text and
report `BODY_TOO_LONG` / `TEXT_EXCEEDS_SHAPE` is in [PATTERNS.md](PATTERNS.md);
Read `segment_fit` placement constraints on recommendations (TOOLS.md).
Shortlists retain scores, rationale and composition order.
Patterns spend at most one solid accent block, in `color_roles.primary_fill`.

Both render tools return `deterministic_ready` (a precondition, not proof),
`publishable` (false on a fresh render), `blocking_reasons`
and `next_tool_call` (blocking fix, else `render_deck_thumbnails` →
`submit_visual_review` with `pptx_revision` = `content_hash`). Submit a
verdict only after inspecting each current-revision image; it never clears
blockers. A changed slide invalidates its verdict; image identity
(`content_hash`, `image_sha256`) is a pixel hash, so re-rendering an
unchanged revision keeps earlier images valid. Layout review reports baseline, hierarchy, pane fill, exhibit size, gutter,
number formatting, orphan, caption proximity, continuation and status-colour
issues. Rhythm also checks repeated visual sides, low visual share, excessive
variety and takeaway repetition/overuse. Inspect measured evidence; these
are advisory. `describe_finding` explains a code;
`get_capabilities().vocabularies.repair_fix_kinds` separates executable
repairs from advice.

## References, loaded only when relevant

- [QUALITY.md](QUALITY.md): storyline and visual choice (always read).
- [DECKSPEC.md](DECKSPEC.md): kinds, budgets, chapters, chrome, revisions.
- [WORKFLOW.md](WORKFLOW.md): Plan → Vary → Render → Repair; visual rubric.
- [RAW_PATH.md](RAW_PATH.md): raw validation, rhythm, assets and repair.
- [TOOLS.md](TOOLS.md): MCP discovery and composition recipes.
- [RULES.md](RULES.md): grid, chart, table and contrast rules.
- [PATTERNS.md](PATTERNS.md): pattern placement and capacity; live schemas:
  `list_patterns` / `show_pattern`. Hero examples are illustrative; supply your image.
- [FINDINGS.md](FINDINGS.md): finding paths, actions and repairs.
- [../template-deck/TEMPLATE_GUIDE.md](../template-deck/TEMPLATE_GUIDE.md):
  native layouts and full-slide images.

Examples: [semantic specs](../../examples/semantic/) and
[raw skeletons](examples/skeletons/README.md). Explore capabilities with companion `llm-wiki` (`json2pptx-wiki`);
content lives in `docs/wiki/`.

## Operational boundaries

Run `json2pptx mcp`; profiles: TOOLS.md. `--tools all` lists all tools;
`get_capabilities().mcp_tools_available` lists callable ones. `list_templates`
`read_only:true` skips cache writes. Template precedence: DECKSPEC.md.

Schema 5 validation and CLI validate/dry-run expose `ok`, `summary`, `findings[]`
at top level; `legacy_findings` keeps the prior envelope. Raw generation adds
`ok`, `pptx_path`, `quality_summary`, `diagnostics`; old keys remain. Both accept
`template` overrides. CLI generate/dry-run shares `--fit-report`, `--verbose-fit`,
`--placeholder-policy`, `--base-dir`.

JSON commands print one stdout document, including failures; stderr is logs.
Generate refusals carry validate's findings, else `STRICT_FIT` / `GENERATION_FAILED`.
Text args over 2 MiB fail. Raced `deck_id` patches return `STALE_REVISION`: reload.
Render assets resolve against `base_dir`; URLs are fetched, missing assets fail.
Stored template paths and base directories remain bound to the handle.

Scale advice: edit authored chart data, then revalidate.
Responses are always compact JSON; the server still advertises `experimental.compact_responses: {}` and still honours the client capability and the deprecated `MCP_COMPACT_RESPONSES=1` environment variable, but neither changes anything.
