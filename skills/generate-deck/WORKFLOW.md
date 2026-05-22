# Workflow: Plan → Vary → Render → Repair

The 4-phase deep dive. SKILL.md has the short overview and the PRECONDITION list; this file walks each phase in detail and covers the post-generation visual-inspection loop.

---

## Phase 1: PLAN — Design the Deck Outline

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

**Cold-start checklist (for decks ≥4 slides):**

1. Pick a template. Call `list_templates` for available options, `canonical_layout_ids`, and `color_roles`. Use `canonical_layout_ids` to map canonical names (`title`, `content`, `blank`, `section`, `closing`, etc.) to concrete layout IDs — do not reverse-engineer raw `slideLayoutN` IDs. The compact response also includes `layout_summaries[]` with per-layout `placeholders[]` (each entry has `id`, `type`, `max_chars`) — use these to rough-size content before calling `expand_pattern`. For full placeholder bounds and font details, switch to full mode.
2. Choose `accent_strategy` — `"rotate"` for multi-section decks, `"section-keyed"` for decks with distinct chapters, `"primary"` only for ≤5-slide decks.
3. For each slide intent, call `recommend_visual` to determine the best visual approach. It ranks across placeholder layouts, named patterns, charts, diagrams, and compose envelopes — not just patterns. Avoid choosing the same pattern more than twice in a row.
4. Ensure the outline alternates density: high-density slides (tables, grids) should be followed by low-density (stat-hero, pull-quote, section divider). Place a narrative-break pattern (stat-hero, pull-quote) every ~5 slides.
5. Check the outline against the rhythm rule: no pattern should appear 3+ times consecutively. If it does, swap the middle occurrence for a contrasting pattern from a different visual family.

For longer decks, `plan_deck` produces a structured outline (ordered slides with per-slide pattern recommendations, narrative roles, content seeds, accent rotation) enforcing the rhythm rules above.

Each `slides[]` entry in the `plan_deck` response carries three fillable-skeleton fields so you do not re-derive slide structure from the prose `content_seed`:

- `suggested_pattern` — first-choice pattern name (mirrors `recommended_pattern`).
- `suggested_pattern_fallback` — second-choice pattern when the suggested pattern's content shape does not fit (drawn from `alternatives[0]`).
- `skeleton` — a partial `SlideInput` JSON object with `__FILL__` tokens for every agent-supplied string. Copy the skeleton verbatim and replace each `__FILL__` with real content rather than authoring the slide from scratch. The skeleton already encodes the layout_id (canonical), title placeholder, and pattern envelope with the correct value shape; numeric structural defaults (grid dimensions, flags) are preserved.

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
- `aggregates.longest_run` — longest consecutive run of one pattern (target: ≤2)
- `aggregates.repetition_index` — 0.0 (all unique) to 1.0 (all same) (target: <0.5)
- `aggregates.accent_balance` — fraction of slides per accent (target: no single accent >80%)
- `aggregates.density_cv` — density variation coefficient (target: >0.1 for decks >3 slides)
- `aggregates.density_distribution` — `{underfilled_cells, optimal_cells, overflow_cells}` totals across all shape_grid cells in the deck
- `composition_score` — 0–100 overall quality score
- `recommendations` — actionable suggestions with `recommended_break_patterns`

**Act on rhythm findings before generating:**

- `longest_run ≥ 3` → swap the middle slide of the run to a `recommended_break_patterns` suggestion
- `accent_balance` shows one accent at >80% → set `accent_strategy: "rotate"` or manually vary accent fills
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
- `"rotate"` — engine cycles through `accent1`–`accent6` across slides. Use for visual variety.
- `"section-keyed"` — accents rotate per section (slides between `section` layout slides share an accent).

**Pre-emit checklist (verify BEFORE outputting JSON):**

1. Every table: logical rows × cols ≤ TDR ceiling (rows ≤ 7, cols ≤ 6, font ≥ 9pt) — see Rule 20 in RULES.md. Count multiline cells as N rows.
2. Every fill is semantic (`accent1`, `lt2`, `dk1`, etc.) except documented brand-color allowlist — no mixed hex+semantic on any slide (Rule 12).
3. No sibling shapes in any `shape_grid` with computed gap < 4pt — no stacked tables separated by hairline dividers.
4. Patterns with 4+ peer cells use `cell_accent_mode: "progressive"` (or `"alternate"` for paired layouts) unless visual consistency is intentional — see Cell Accent Variety in RULES.md.
5. Every cell at 60–110% density — compare your text length against `max_chars` from `expand_pattern`'s `cell_budgets[]`. See Text Capacity Awareness in PATTERNS.md.

---

## Phase 4: REPAIR — Validate, Render, Verify, Fix

Validation is NOT verification. `validate_input` checks JSON structure; it does not judge whether the deck looks right. Contrast auto-fix, sizing choices, overflowing text, and mis-chosen layouts are all visible in pixels and invisible in JSON. **Images are truth.**

1. **Schema + fit check.** Call `validate_input` with `fit_report: true` (MCP) or run `json2pptx validate -fit-report` (CLI). The CLI form `-fit-report=path.json` writes **NDJSON** (one finding per line, no array wrapping); `-fit-report=-` writes NDJSON to stdout; bare `-fit-report` prints a human-readable summary to stderr. Validate exits 0 even with unfittable cells — refusal comes via `strict_fit` on generate. Fix only failing slides, don't regenerate the deck. The fit-report surfaces diagnostics with `fix.kind` hints that are directly actionable. See `FINDINGS.md` for the full code catalog and `fix.kind` enums. Input JSON is validated with `additionalProperties: false` — unknown fields produce warnings identifying the unexpected key and its location.

   **Findings sort invariant.** Every `findings` / `fit_findings` array — across `validate_input`, `preview_presentation_plan`, `generate_presentation`, `score_deck`, and `repair_slide` — is sorted by `(severity desc, slide_index asc, code asc)`. `findings[0]` is always the most important fix to attempt first. Deck-level findings (path doesn't match `/slides/N/...`) sort before slide 0 at equal severity. The ordering is deterministic across runs, so agents can address findings top-to-bottom without re-prioritising.
2. **Generate.** Call `generate_presentation` with `strict_fit: "warn"` (default) or `"strict"` for refuse-on-overflow (MCP), or `json2pptx generate -strict-fit warn|strict` (CLI). The strict-fit ladder: `off` (legacy, silent shrink+truncate); `warn` (shrink + emit fit-findings); `strict` (refuse on overflow with `fix.kind: split_at_row|reduce_text`). Both native layout findings and chart findings participate in the ladder — see FINDINGS.md for which codes promote at which level. On refusal, MCP returns the shared FindingEnvelope with `IsError=true`:
   ```json
   {
     "schema_version": "1.0",
     "tool": "json2pptx",
     "subcommand": "mcp",
     "ok": false,
     "summary": "1 error",
     "findings": [
       {
         "id": "fit-1",
         "code": "FIT.placeholder_overflow",
         "category": "FIT",
         "severity": "error",
         "where": { "slide": 2 },
         "message": "text overflows placeholder by 42%",
         "evidence": { "path": "slides[2].content.body" },
         "remediation": { "primary": { "action": "shorten_text", "params": { "kind": "reduce_text" } } }
       }
     ]
   }
   ```
3. **Render to images.** Call `render_slide_image` (one slide) or `render_deck_thumbnails` (whole deck) over MCP — preferred over the `pptx2jpg -input <out.pptx> -output <dir>/ -density 150` shell-out. Both paths require LibreOffice + ImageMagick on the server's PATH; if unavailable, **say so explicitly** and flag data-dense slides for manual inspection before declaring done. To get a deck-level quality signal, also call `score_deck` — it returns a 0-100 score plus structured findings keyed to the same `code` vocabulary as fit-report.
4. **Inspection checklist (per slide).** Before handing back to the user, confirm:
   - [ ] Text fits its shape or cell — no clipping, no visible overflow.
   - [ ] Chart axes/legends are readable at deck-viewing size.
   - [ ] Every placeholder and grid cell shows the content you intended.
   - [ ] Text color is intentional — no surprise grays from contrast auto-fix (see Rule 16 in RULES.md).
   - [ ] Footer and source render where expected; no "Source: Source:" double prefix (see Rule 18).
5. **Repair.** Prefer `repair_slide` (MCP) over hand-editing JSON — it accepts the same `Fix.Kind` vocabulary fit-report emits and patches one slide without regenerating the deck. Pass the deck JSON, the 0-based `slide_index`, and a `fixes` array of `{kind, params}` directives. Returns the patched deck plus post-patch fit findings for the modified slide. Supported `repair_slide` apply-only kinds are a *superset* of the fit-report enum — see "Fix kinds for `repair_slide`" in FINDINGS.md. Common repairs:
   - Text clipping or overflow → `repair_slide` with `{kind:"reduce_text", params:{max_items|max_length}}`, `{kind:"shorten_title", params:{max_length}}`, or `{kind:"split_at_row", params:{row}}`. For shape_grid cells specifically, `{kind:"reduce_cell_text", params:{cell_path, max_chars}}` truncates a single cell's text to a character budget (with ellipsis). Prefer rewriting content to fit over truncation; use `reduce_cell_text` only when the agent should not rephrase the text (e.g., user-supplied verbatim content). As a last resort, lower font size or increase cell/row allocation in JSON.
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

---

## Machine-Actionable `next_tool_call`

Pattern validation errors (`validate_pattern`), density warnings (`expand_pattern`), fit-report findings (`validate_input`, `generate_presentation`), and boundary errors from the candidate-decision tools (`plan_deck`, `recommend_pattern`, `recommend_visual`, `validate_input`, `preview_presentation_plan`, `score_deck`) include an optional `next_tool_call` field when the error has an actionable recovery. This is a machine-readable hint: the exact MCP tool name and an `args_template` pre-filled with fix parameters. Invoke the suggested tool directly without inferring the protocol from the error message.

Boundary-error mappings used by the candidate-decision tools:

- `MISSING_PARAMETER` / `INVALID_JSON` on `presentation` → `get_input_schema` (fetch the schema and retry)
- `MISSING_PARAMETER` on `template` (or `TEMPLATE_NOT_FOUND` / `TEMPLATE_ERROR`) → `list_templates`
- `MISSING_PARAMETER` on `brief` / `intent` → retry the same tool with the missing argument
- `INVALID_PARAMETER` for an unknown pattern name → `list_patterns`
- `STRUCTURE_AND_SLIDES` on `structure` → remove one of the two — `structure` and top-level `slides` are mutually exclusive. The `fix.params.field` names which side to drop (`"slides"`).
- `INVALID_STRUCTURE` on `structure` → repair the structure block (missing section title, empty sections, section with no slides). The underlying expansion error is in `fix.params.error`.

```json
{
  "field": "values.title",
  "code": "unknown_key",
  "message": "unknown field \"titl\" (did you mean \"title\"?)",
  "fix": { "kind": "rename_field", "params": { "from": "titl", "to": "title" } },
  "next_tool_call": {
    "tool": "repair_slide",
    "args_template": {
      "slide_index": -1,
      "pattern": "card-grid",
      "fixes": [{ "kind": "rename_field", "params": { "from": "titl", "to": "title" } }]
    }
  }
}
```

- `slide_index: -1` means "caller must supply the actual slide index" — `validate_pattern` operates without slide context.
- For `swap_pattern` fix kinds, `next_tool_call` points to `recommend_pattern` instead of `repair_slide`.
- Internal-only errors (marshal failures, unrecognized fix kinds inside content-finding errors) may omit `next_tool_call` (the field is absent, not null). Boundary errors from candidate-decision tools always carry it.

---

## `response_fingerprint` — server-side cache key

`validate_input`, `preview_presentation_plan`, `plan_deck`, and `recommend_visual` responses include a top-level `response_fingerprint` field: a sha256 hex digest (64 chars) of the canonical JSON of the response body with the fingerprint field itself zeroed. These four paths are deterministic — identical inputs produce identical fingerprints — so agents may use the fingerprint directly as a memoisation cache key without re-hashing the body. To verify a fingerprint, parse the response, zero `response_fingerprint`, re-marshal canonically, and sha256-hash the result.

---

## `idempotency_key` — safe retries for generate / auto_repair / make_deck

`generate_presentation`, `auto_repair`, and `make_deck` accept an optional top-level `idempotency_key` string. When set, the server caches the first successful response under that key and replays it on subsequent calls within the cache TTL (1 hour, per-process), **but only when the request content is unchanged**. The replay response carries `"idempotent_replay": true` so the caller can tell a deduped retry from a fresh run.

The key is a *retry token*, not a request identity: the server also stores a fingerprint of the normalized request (every argument except `idempotency_key`). Reusing the same key with edited input is treated as a different request — the server refuses with an `IDEMPOTENCY_CONFLICT` error (carrying `current_fingerprint` and `original_fingerprint` in the finding evidence) instead of replaying the original deck for the wrong content. Issue a fresh key for new content, or restore the original input to replay.

Use this to make transport-layer retries safe. Without an idempotency key, every retry runs the full pipeline again and writes a fresh output file (`output.pptx`, `output_1.pptx`, `output_2.pptx`, …); the caller is also billed for the wasted inference + render cost.

```json
{
  "presentation": { "template": "midnight-blue", "slides": [/* … */] },
  "output_filename": "deck.pptx",
  "idempotency_key": "agent-session-abc123/turn-7"
}
```

Rules of thumb:

- Generate the key from something stable across retries (session id + turn number, or a hash of the input). Never use a timestamp — every retry would get a new key.
- Keys are scoped per-tool, so the same string used against `generate_presentation` and `auto_repair` will not collide.
- A replay requires the request to be byte-for-byte equivalent (modulo object-key ordering). If you edit the deck/outline or any other argument and keep the key, you get an `IDEMPOTENCY_CONFLICT` error, never a stale replay — bump the key whenever the content changes.
- Only successful responses are cached. Error responses surface every time so the agent can fix the underlying input.
- The cache is in-memory and per-process. Restarting the MCP server drops it — design retries to tolerate a fresh run after a server bounce.
