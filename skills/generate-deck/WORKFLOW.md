# Workflow: Plan → Vary → Render → Repair

The 4-phase deep dive. SKILL.md routes to semantic or raw authoring;
RAW_PATH.md scopes the raw preconditions. This file walks each phase in
detail and covers the post-generation visual-inspection loop.

---

## Phase 1: PLAN — Design the Deck Outline

For new content-bearing decks, the preferred artifact is a semantic **DeckSpec**,
not raw `PresentationInput` JSON. Capture the same outline below, then encode it
as semantic YAML/JSON (`meta` metadata plus `slides[].kind`) and render it with
`render_deck_spec` — see [DECKSPEC.md](DECKSPEC.md)
for budgets, live kind discovery, and the spec-level repair contract. Use
raw JSON only when the user asks for low-level control or the required visual is
outside the semantic schema.

Before writing any JSON, produce a short outline:

```
Deck: [title]
Template: [template name]
Accent strategy: [primary | rotate | section-keyed]
Slides:
  1. [layout] — [title] — [pattern or content type] — [accent]
  2. [layout] — [title] — [pattern or content type] — [accent]
  ...
```

Each line picks a `layout_id` and a visual approach. For shape grid slides, name the pattern (call `list_patterns` via MCP, or `json2pptx patterns list` from the CLI, for the catalog). For content slides, note the content type (bullets, chart, table, diagram). Use `recommend_visual` when unsure which visual approach fits a slide intent — it ranks across all categories (layouts, patterns, charts, diagrams). Use `recommend_pattern` only when you already know you need a named pattern.

Present the outline to the user. Proceed to Phase 2 only after approval or if the user asked for the full deck directly.

**Narrative coherence matters.** A consulting deck tells a story: situation, complication, resolution, evidence, implementation, call to action. The outline is where you design the argument arc. Do not fragment this across phases.

### Layout coverage requests

When the brief asks for a complete deck that also showcases layouts, plan the argument and chapters
before applying the coverage constraint:

1. Write the story and section outline without counting layouts as slides.
2. Choose a slide budget that supports the subject. “Complete” usually needs repeated content
   layouts and more slides than the number of required layouts.
3. Put chapter boundaries in DeckSpec `structure.sections`; use `auto_agenda` when there are at least
   two sections. The engine creates and numbers dividers.
4. Add canonical IDs to `meta.required_layouts`, then run `explain_deck_spec`. Review every
   `layout_coverage.assigned` entry and resolve `missing` by adding a compatible narrative slide.
5. Keep DeckSpec as the authoring source. Use `raw_json2pptx` only for the smallest visual the
   semantic kinds cannot express.
6. If `blank-canvas` is assigned to a content slide, the compiler carries its title into the raw
   slide's `headline` field and reserves the remaining canvas for the visual.

Reject the plan before rendering when it produces an isolated numeric divider, an untitled visual
canvas, one slide per required layout, or a deck made only of bullets and colored rectangles.
Evidence-oriented decks need at least one data-bearing visual and enough distinct visual families to
separate evidence, analysis, process, and chapter structure.

**Cold-start checklist (for decks ≥4 slides):**

1. Pick a template. Call `list_templates` for available options, `canonical_layout_ids`, and `color_roles`. Use `canonical_layout_ids` to map canonical names (`title`, `content`, `blank`, `section`, `closing`, etc.) to concrete layout IDs — do not reverse-engineer raw `slideLayoutN` IDs. The default `fields="compact"` response omits per-layout details; when you need rough placeholder capacity, request legacy `mode="compact"` for `layout_summaries[].placeholders[]` (`id`, `type`, `max_chars`). For full placeholder bounds and font details, use `fields="full"`.
2. Choose `accent_strategy` per RULES.md → Accent monotony: `"primary"` by default, `"section-keyed"` for chaptered decks, `"rotate"` only when the template's safe-accent set is large.
3. For each slide intent, call `recommend_visual` to determine the best visual approach. It ranks across placeholder layouts, named patterns, charts, diagrams, and compose envelopes — not just patterns. Avoid choosing the same pattern more than twice in a row.
4. Ensure the outline alternates density: high-density slides (tables, grids) should be followed by low-density (stat-hero, pull-quote, section divider). Place a narrative-break pattern (stat-hero, pull-quote) every ~5 slides.
5. Check the outline against the rhythm rule: no pattern should appear 3+ times consecutively. If it does, swap the middle occurrence for a contrasting pattern from a different visual family.

For longer decks, `plan_deck` produces a structured outline (ordered slides with per-slide pattern recommendations, narrative roles, content seeds, accent rotation) enforcing the rhythm rules above. Each slide carries a canonical `layout`: slide 0 is `"title"` and the last slide is `"closing"` with no pattern (`recommended_pattern: ""`); content slides are `"blank-title"` + a pattern. Comparison slots use only `comparison-2col` / `before-after`, and emphasis patterns (`stat-hero`, `pull-quote`, `kpi-inline`) are capped at ceil(n/5). Brief facts (numbers, named entities) arrive verbatim in each slide's `content_seed` / `facts[]`; facts with no slot are in `unplaced_facts[]` — place them, don't drop them.

Each `slides[]` entry in the `plan_deck` response carries three fillable-skeleton fields so you do not re-derive slide structure from the prose `content_seed`:

- `suggested_pattern` — first-choice pattern name (mirrors `recommended_pattern`).
- `suggested_pattern_fallback` — second-choice pattern when the suggested pattern's content shape does not fit (drawn from `alternatives[0]`).
- `skeleton` — a partial `SlideInput` JSON object with `__FILL__` tokens for free-form copy and schema-valid defaults for typed fields (enums, icons, scores). Copy the skeleton, replace each `__FILL__` with real content, review the typed choices listed in its `__CHOOSE__` speaker note, and remove that draft note. The skeleton already encodes the layout_id (canonical), title placeholder, and pattern envelope with the correct value shape; numeric structural defaults (grid dimensions, flags) are preserved.

  **Replace every `__FILL__` before publishable generation.** Skeletons stay structurally valid (`valid: true`) because `__FILL__` is a non-empty string, but `validate_input` and `generate_presentation` now scan for leftover tokens and report each one as an `unresolved_placeholder` warning carrying its JSON path. This is intentional for draft scaffolding; for a finished deck, pass `placeholder_policy: "strict"` (the publishable/gated mode) so any remaining `__FILL__` becomes a blocking error instead of a warning.

---

## Phase 2: VARY — Check Rhythm and Accent Balance

After building the JSON but **before** generating the PPTX, run `analyze_deck_rhythm` to catch monotony and accent imbalance.

```
analyze_deck_rhythm(presentation: {template: "...", slides: [...]})
```

Returns:
- `per_slide` — visual fingerprint per slide (pattern, density_class, accent_role, dominant_visual, within_slide_accent_variety)
- `per_slide[].within_slide_accent_variety` — count of distinct accent slots used across the slide's shape_grid cells (0 for non-grid slides)
- `aggregates.longest_run` — longest consecutive run of one visual family, including variants such as `kpi-3up`/`kpi-4up` (target: ≤2)
- `aggregates.repetition_index` — 0.0 (all unique) to 1.0 (all same) (target: <0.5)
- `aggregates.accent_balance` — fraction of slides per accent (target: no single accent >80%)
- `aggregates.density_cv` — density variation coefficient (target: >0.1 for decks >3 slides)
- `aggregates.density_distribution` — `{underfilled_cells, optimal_cells, overflow_cells}` totals across all shape_grid cells in the deck
- `composition_score` — 0–100 overall quality score
- `recommendations` — actionable suggestions with `recommended_break_patterns`

**Act on rhythm findings before generating:**

- `longest_run ≥ 3` → swap the middle slide of the run to a `recommended_break_patterns` suggestion
- `accent_balance` shows one accent at >80% → vary grid cells with `cell_accent_mode`, or use `"section-keyed"` on a chaptered deck (RULES.md → Accent monotony)
- `density_cv < 0.1` on a 5+ slide deck → insert a low-density narrative break (stat-hero, pull-quote)
- `within_slide_accent_variety == 1` on a slide with 5+ cells → add `cell_accent_mode: progressive` to the pattern overrides
- `density_distribution.underfilled_cells > 30%` of total → add detail text or switch to smaller grid patterns
- `composition_score < 70` → iterate on the outline until score ≥ 70

This is a lightweight static check (no PPTX generation cost). Run it iteratively: fix → re-analyze → confirm score improved.

---

## Phase 3: RENDER — Generate Full JSON and PPTX

Generate the complete JSON in one pass. Use named patterns for shape grid slides — call `show_pattern` (MCP) or `json2pptx patterns show <name>` (CLI) for each pattern's value schema, then fill in content. Set the pattern at the slide level via the `pattern` field (XOR with `shape_grid` — never set both).

**Accent strategy.** Set `accent_strategy` at the top level of the presentation JSON:
- `"primary"` (default) — all slides use the template's primary accent. Good for short decks.
- `"rotate"` — cycles through the template's safe accents (light text readable, not the negative accent), keyed to each pattern's content, so inserting a slide recolours nothing; the excluded accents are reported once per deck.
- `"section-keyed"` — accents rotate per section (slides between `section` layout slides share an accent).

**Pre-emit checklist (verify BEFORE outputting JSON):**

1. Every table: logical rows × cols ≤ TDR ceiling (rows ≤ 7, cols ≤ 6, font ≥ 9pt) — see Rule 20 in RULES.md. Count multiline cells as N rows.
2. Every fill is semantic (`accent1`, `lt2`, `dk1`, etc.) except documented brand-color allowlist — no mixed hex+semantic on any slide (Rule 12).
3. No sibling shapes in any `shape_grid` with computed gap < 4pt — no stacked tables separated by hairline dividers.
4. Patterns with 4+ peer cells use `cell_accent_mode: "progressive"` (or `"alternate"` for paired layouts) unless visual consistency is intentional — see Cell Accent Variety in RULES.md.
5. Every cell at 35–110% density — read `density_pct` (a height ratio) from `expand_pattern`'s `cell_budgets[]`. See Text Capacity Awareness in PATTERNS.md.

---

## Phase 4: REPAIR — Validate, Render, Verify, Fix

Validation is NOT verification. `validate_input` checks JSON structure; it does not judge whether the deck looks right. Contrast auto-fix, sizing choices, overflowing text, and mis-chosen layouts are all visible in pixels and invisible in JSON. **Images are truth.**

1. **Schema + fit check.** Call `validate_input` with `fit_report: true` (MCP) or run `json2pptx validate --fit-report` (CLI; add `--json` for the MCP envelope, `--format=ndjson` for one object per file). Validate exits 0 even with unfittable cells — refusal comes via `strict_fit` on generate. Fix only failing slides, don't regenerate the deck. The fit-report surfaces diagnostics with `fix.kind` hints that are directly actionable. Use `describe_finding` and `get_capabilities().vocabularies` for live codes and fix kinds; [FINDINGS.md](FINDINGS.md) explains the decision process. Input JSON is validated with `additionalProperties: false` — unknown fields produce warnings identifying the unexpected key and its location.

   **Findings sort invariant.** Every `findings` / `fit_findings` array — across `validate_input`, `preview_presentation_plan`, `generate_presentation`, `score_deck`, and `repair_slide` — is sorted by `(severity desc, slide_index asc, code asc)`. `findings[0]` is always the most important fix to attempt first. Deck-level findings (path doesn't match `/slides/N/...`) sort before slide 0 at equal severity. The ordering is deterministic across runs, so agents can address findings top-to-bottom without re-prioritising.
2. **Generate.** Call `generate_presentation` with `strict_fit: "warn"` (default) or `"strict"` for refuse-on-overflow (MCP), or `json2pptx generate -strict-fit warn|strict` (CLI). The strict-fit ladder: `off` (legacy, silent shrink+truncate); `warn` (shrink + emit fit-findings); `strict` (refuse on overflow with `fix.kind: split_at_row|reduce_text`). Both native layout findings and chart findings participate in the ladder — see FINDINGS.md for which codes promote at which level. On refusal, MCP returns the shared FindingEnvelope (see [FINDINGS.md](FINDINGS.md)) with `IsError=true`.
3. **Render to images, then inspect them.** Three distinct steps — do not conflate them:
   - **Structural wireframe** (`preview_slide_wireframe`) is geometry only. Its response is stamped `inspection_kind: "wireframe_structural"`, `contract: "structural_only"`, `not_text_flow_safe: true`. It does NOT model text wrapping, font metrics, icon/text collisions, SVG readability, or image fidelity, and emits no quality verdict. **A wireframe never satisfies visual QA** — it is a cheap pre-render sanity check, nothing more.
   - **Rendered-image generation** (`render_slide_image` for one slide / `render_deck_thumbnails` for the whole deck, preferred over the `pptx2jpg -input <out.pptx> -output <dir>/ -density 150` shell-out) produces real pixels via LibreOffice + ImageMagick. **Generating the PNG is not the same as inspecting it** — an unviewed render is evidence you have not yet read. By default these tools return every slide as a native MCP image content block you can look at directly (plus JSON metadata with each slide's on-disk `path`); pass `include_base64_json: true` only for clients that cannot display MCP images.
   - **Rendered visual inspection** is the verification step: the rendered pixels must actually be inspected — by `inspect_slide_images` (Claude vision / heuristic) or by an agent looking at the images — before the deck counts as visually verified.

   Both render paths require LibreOffice + ImageMagick on the server's PATH; if unavailable, **say so explicitly** and flag data-dense slides for manual inspection before declaring done. Some LibreOffice builds ignore embedded fonts; don't judge typefaces from them. To get a deck-level quality signal, also call `score_deck` — it returns a 0-100 score plus structured findings keyed to the same `code` vocabulary as fit-report.
4. **Inspection checklist (per slide).** Before handing back to the user, confirm:
   - [ ] Text fits its shape or cell — no clipping, no visible overflow.
   - [ ] Chart axes/legends are readable at deck-viewing size.
   - [ ] Every placeholder and grid cell shows the content you intended.
   - [ ] Text color is intentional — no surprise grays from contrast auto-fix (see Rule 16 in RULES.md).
   - [ ] Footer and source render where expected; no "Source: Source:" double prefix (see Rule 18).
5. **Repair.** Prefer `repair_slide` (MCP) over hand-editing JSON — it accepts the executable vocabulary advertised by `get_capabilities().vocabularies.repair_fix_kinds` and patches one slide without regenerating the deck. Pass the raw `deck_id` returned by `generate_presentation` (or the deck JSON), the 0-based `slide_index`, and a `fixes` array of `{kind, params}` directives. A raw-handle repair persists the change and returns `changed_slides` plus post-patch findings without echoing the full deck; pass `return_deck:true` only when you need that JSON. A stateless repair returns the patched deck as before. Supported `repair_slide` kinds are a *superset* of the fit-report's suggestions; [FINDINGS.md](FINDINGS.md) explains executable versus advisory fixes. Common repairs:
   - Text overflow: preserve required evidence. For native plain bullet columns, use `repair_slide` with `{kind:"split_bullets", params:{max_items:3}}` and render every continuation. It retains all strings and nested groups, coordinates equal-length columns and keeps the native layout. Compound lists, unequal columns, oversized nested groups or numeric slide links require explicit sibling-slide authoring. Count budgets do not guarantee fit. Use `{kind:"shorten_title", params:{max_length}}` only when meaning is preserved, or `{kind:"split_at_row", params:{row}}` for tables. `reduce_text` and `reduce_cell_text` truncate content: never remove required facts or verbatim evidence. Improve wording only if the brief permits; otherwise redistribute content/space before considering a smaller font.
   - Wrong layout for the content → `repair_slide` with `{kind:"swap_layout", params:{layout_id}}`.
   - Surprise gray text from contrast auto-fix (visible as a `contrast_autofixed` finding) → swap fill to an accent with ≥3.0 contrast against white, OR switch text color to `dk1`, OR set `"contrast_check": false` if the gray is wrong and the accent is already a compliant color (see Rule 16 in RULES.md).
   - For a no-side-effect dry run before regenerating, call `preview_presentation_plan` to inspect layout selection, placeholder mapping, and fit findings without producing a PPTX.

Do not tell the user the deck is done until the checklist passes or you have explicitly flagged what you couldn't verify.

---

## Visual inspection (Claude vision)

Pixels — not JSON — decide whether refinement is acceptable. The `inspect_slide_images` MCP tool exposes the same Claude-vision QA agent that `testrand qa` runs on the CLI: pass an array of rendered slide images, get back structured findings keyed to repair_slide fix kinds.

**When to call it.** After `render_deck_thumbnails` or `render_slide_image`, when (a) the deck has been generated and visually rendered, (b) heuristics on its own pass but the deck still feels off, or (c) the user explicitly asked for a quality pass. Skip it for sub-3-slide drafts. When `ANTHROPIC_API_KEY` is unset the tool degrades gracefully to a heuristic mode (`mode:"heuristic"`) instead of failing: findings are coarser (blank/edge-overflow/aspect-ratio only, all P3, tagged `source:"heuristic"`) but still usable as triage input.

**Shape of the call.**

```json
{
  "slide_images": [
    {"index": 0, "png_base64": "...", "slide_type": "title",   "title": "..."},
    {"index": 1, "path": "/tmp/deck/slide-1.png", "slide_type": "content"}
  ],
  "deck_metadata": {"template": "midnight-blue"}
}
```

Each entry sets exactly one of `path` (absolute, .png/.jpg/.jpeg, no `..`) or `png_base64` (raw base64). Optional `slide_type` and `title` tune the per-slide prompt; supply them when you have them — they materially improve precision.

**How to consume findings.** Each finding's `suggested_fixes[]` is pre-mapped to `repair_slide` fix kinds via `SuggestedFixesForCategory`. The agent-side pipeline is:

```
findings = inspect_slide_images(...).results[slide].findings
for f in findings where f.severity in {"P0","P1"}:
    repair_slide(presentation, slide_index=f.slide_index,
                 fixes=[{"kind":"autofix_visual","params":{"category":f.category}}])
```

`autofix_visual` consults the same category→kind map server-side and tries each candidate in order until one succeeds — so a `text_overflow` finding tries `reduce_cell_text`, then `split_at_row`, then `reshape_grid`. Three visual QA categories — `image_quality`, `aspect_ratio`, `border_style` — return empty `suggested_fixes[]` and should be surfaced for human review rather than auto-repaired.

The same response also carries a top-level `findings` `FindingEnvelope` — every per-slide finding projected into the shared diagnostics wire shape used by `validate_input`, `repair_slide`, and the MCP error path. P0/P1 map to `error` (so `findings.ok` is `false` exactly when a slide needs repair under the P0/P1 policy below), P2 to `warning`, P3 to `info`; overflow categories namespace as `FIT` and all other visual defects as `RENDER`. Branch on `findings.ok` for a one-flag stop signal, or keep reading `results[slide].findings` when you need the full per-slide `suggested_fixes[]`.

**False-positive policy.** Haiku-vision flags ~60% false positives on layout issues (top-clipping, title-cut) when running on already-correct decks. Treat P2/P3 findings as advisory; only P0/P1 should trigger automatic repair without user confirmation.
