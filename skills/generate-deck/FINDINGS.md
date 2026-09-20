# Finding Codes and Fix Kinds

Complete catalog of finding codes (emitted by `validate_input`, `preview_presentation_plan`, `generate_presentation`, and svggen) and the `fix.kind` vocabularies (`fit-report` enum and `repair_slide` apply-only superset). All findings follow the `{path, code, severity, action, message, fix}` envelope. Stable fields for programmatic matching: `code`, `severity`, `action`, `fix.kind`, `fix.params`. Advisory (may change): `message`.

For the contributor-facing catalog (with longer rationale + emission paths) see `../../docs/FIT_FINDINGS.md`.

**Runtime lookup:** for any single unfamiliar code, call the `describe_finding` MCP tool — it returns the same `{summary, severity, when_emitted, remediation_steps[], example_before, example_after, related_codes[]}` envelope this catalog documents, sourced from the engine's own registry so it cannot drift. One tool call resolves any code without loading this file.

**Sort invariant.** Every `findings` / `fit_findings` array is returned in `(severity desc, slide_index asc, code asc)` order across every tool that emits findings (`validate_input`, `preview_presentation_plan`, `generate_presentation`, `repair_slide`, `score_deck`). `findings[0]` is always the most important fix to address first. Deck-level findings (path doesn't match `/slides/N/...`) sort before slide 0 at equal severity. Ordering is deterministic across runs — safe to walk top-down.

**One code, one severity.** Severity is derived from the finding's `action` through a single mapping — `refuse` → `error`, `shrink_or_split` → `warning`, `review` / `info` → `info` — used by every envelope and by `score_deck`'s per-slide findings alike. An emitter that states no action takes the one the code declares in the finding registry (what `describe_finding` returns as `severity`), so the same code cannot arrive at two severities depending on which detector raised it or which tool you asked. Findings that say the same thing about the same path are collapsed to one entry, keeping the record that carries a remediation. Filtering a work list on severity therefore gives the same answer from `validate_input` and `score_deck`.

**Finding class (rendering vs pattern-choice).** `score_deck` tags every `per_slide[].findings[]` entry with a `class`: one of `pattern_choice`, `rendering`, or `content`. The split a reviewer cares about most is **pattern-choice vs rendering**:

- `pattern_choice` — the author picked a layout family ill-suited to the content (a single-row flow stretched over-tall, an agenda drawn as a flowchart, a decision diamond with no branch zone). The engine rendered exactly what it was asked; the fix is to **swap to a better-suited pattern**, not to patch a render. Codes: `OVERTALL_FLOW_LANE`, `FLOW_DIAMOND_NO_CONTENT`, `TOC_FLOWCHART_VOCAB`, `SPARSE_SINGLE_ROW_FLOW`, `wrong_pattern`, `pattern_overcrowded`, `pattern_underfilled`, `sparse_layout`, `cell_underfilled`.
- `rendering` — the engine could not fit, had to adjust, or produced a geometry artifact for what was authored (overflow, contrast auto-fix, clamping, a rotated band that renders mis-shaped, e.g. `MATRIX_AXIS_IMBALANCE`). Engine-side signal: shrink/split, or a genuine rendering issue to fix. This is the default class for any unmapped code.
- `content` — an authoring-text problem independent of pattern or render (`HEADLINE_TOO_LONG`, `BODY_TOO_LONG`, `BULLET_NESTING_DEEP`, `NUMBERED_LIST_NOT_APPLIED`, `MISSING_ALT_TEXT`, `DUPLICATE_TITLE`, `takeaway_missing`).

---

## Layout Finding Codes

Native (non-chart) findings. No prefix — the `chart.*` namespace below covers charts and diagrams.

### Pre-flight codes — emitted when measuring the deck before render

| Code | When emitted | Default action | Typical `fix.kind` |
|------|-------------|----------------|--------------------|
| `placeholder_overflow` | Body text overflows placeholder after autofit (three-condition gate: overshoot > 15%, autofit off/unavailable, can't fit at min font) | `shrink_or_split` | `reduce_text` |
| `title_wraps` | Title placeholder measures >1 line (informational, distinct from `placeholder_overflow`). **Escalates to `shrink_or_split`** (`fix.kind: shorten_title`, `fix.params: {current_chars, max_chars, fit_scale_pct}`) when the measured title only fits its resolved title box below the comfort size (80% of the template title size, or 32pt for larger display titles) or only with reduced line spacing. Measured with the inherited title style (size, all-caps, line spacing); canonical `layout_id`s resolved. This is the **single** title-length code: `validate` emits the identical finding (same code, path and message, deduped in the envelope), the `quality` score reports the same sentence, and neither the old 60-character fallback nor `HEADLINE_TOO_LONG` fires alongside it | `review` / `shrink_or_split` | `reduce_text` / `shorten_title` |
| `TITLE_OVERFLOW` (preflight) | Same measurement as the render-time `TITLE_OVERFLOW` below, predicted by `validate --fit-report` / `validate_input` / preview: the title cannot fit its resolved title placeholder even at the minimum autofit size | `shrink_or_split` | `shorten_title` |
| `slide_bounds_overflow` | JSON-authored shape center falls outside slide rect (center-based threshold, not corners) | `shrink_or_split` | `reduce_text` |
| `footer_collision` | Authored shape bbox intersects footer area on a layout that declares a footer placeholder | `review` (strict: `refuse`) | `reduce_text` |
| `takeaway_missing` | A slide that argues from data has an empty `takeaway`: it carries a chart content item, a chart-shaped `diagram_value`, or a pattern marked `data_visual` in its taxonomy (`chart-insights-split`, `waterfall-bridge`, `horizontal-bar-with-callouts`, `table-highlight`, `matrix-2x2` — `show_pattern` returns the flag). Suppressed when the slide title is itself the argument: six words or more containing a verb. Advisory — never blocks generation | `review` | `provide_value` (`field: "takeaway"`) |
| `chrome_band_no_fit` | The slide's `takeaway` / `source` band cannot be placed on its layout: the band stack (body column x-range, stacked above the layout's dt/ftr/sldNum footer placeholders) would climb into the title or leave <20% of the slide height for the layout's content. The band is **skipped at render** instead of overlapping chrome | `review` | `swap_layout` (to the One Content layout) |
| `title_collision` | Authored shape top edge intrudes upward into the title chrome (resolved content zone's title bottom). Title-side mirror of `footer_collision`; catches "title at bottom, body above" flipped-geometry layouts. Preflight resolves grid cells with the same layout-aware geometry as generation. Only fires when a title-anchored zone was resolved | `review` (strict: `refuse`) | `reposition_shape` |
| `fit_overflow` | Per-cell: text needs more lines than cell height allows at the declared font | `refuse` | `split_at_row` / `reduce_text` |
| `density_exceeded` | Table rows × cols beyond TDR ceiling at the declared font (Rule 20) | `review` | `split_at_row` |
| `stacked_tables` | Sibling tables in a shape_grid with `row_gap < 4pt` (two-tables-one-grid anti-pattern) | `review` | `split_at_row` |
| `divider_too_thin` | Divider shape height < 4% of slide height | `review` | — |
| `hex_fill_non_brand` | Non-allowlisted `#RRGGBB` fill on a shape | `review` | `use_semantic_color` |
| `mixed_fill_scheme` | Slide mixes semantic (`accent1`, `lt2`) and hex fills (hex-fill mix anti-pattern) | `review` | `use_semantic_color` |
| `accent_overload` | Slide uses more than two distinct accent hues (`accent1`..`accent6`) — pick one base accent and use `cell_accent_mode` for variety | `review` | `consolidate_accents` |
| `LAYOUT_UNRESOLVABLE` | No layout in the template can host the slide's declared `slide_type` — the template is missing that role entirely (e.g. it carries only Title Slide and Blank). Reported at validate/preview time for **every** affected slide, naming the `slide_type` you wrote rather than the type the engine coerces it to internally. A `pattern` / `shape_grid` / `compose` slide then falls back to the template's blank canvas with a deck warning (those need only a canvas); any other slide refuses at generate time. `action: review`, `fix.kind: swap_layout`, `fix.params: {slide_type, candidates[], fallback_layout_id?}` where `candidates` are the layouts this template actually declares. Agent action: set `layout_id` explicitly from `candidates`, or pick a template that declares the role (`list_templates` `canonical_layout_ids`, or `examine_template`'s missing-role findings) | `review` | `swap_layout` |
| `WEAK_CONTENT` | The raw-path twin of the compiler's `SEMANTIC_WEAK_CONTENT`: a slide still carries exemplar copy — lorem ipsum, `Click to add title`, `Presentation Title`, a masked number (`XX%`, `$X.XM`) a whole value of `TBD` / `N/A`, or a pattern's own exemplar label (`Card 1`, `Description 2`). Structural strings (placeholder IDs, geometry names) are exempt. `fix.params.samples[]` lists up to three offending strings per slide | `refuse` | `replace_placeholder` (advisory) |
| `MISSING_TITLE` | A slide expected to make a point carries no title placeholder text and no pattern title value. Set `slide_type` to `section` / `blank` for deliberate chrome slides | `review` | `provide_value` |
| `SLIDE_NEARLY_EMPTY` | A text-only content slide carries fewer than 8 words of body content — or, once per deck, none of the slides carry content the tool recognises (usually a DeckSpec handed to a PresentationInput tool) | `review` | `add_items` / `provide_value` |
| `DECK_MONOTONY` | Four or more consecutive argument slides share one shape (same pattern, or same content types); six or more raises it to `refuse`. Title/section/blank slides never join a run | `review` / `refuse` at 6+ | `swap_pattern` |
| `CHART_OVERLOADED` | A pie/donut with more than 7 slices, or another chart type with more than 12 categories (8 when labels average 24+ characters) — the renderer rotates and truncates them into an unreadable band | `review` | `reduce_items` |
| `cell_underfilled` | **One finding per slide**, not per cell: the grid's text cells fill under 35% of their box height. Metric values and short labels/captions are exempt (a KPI card holding `$12.4M` is correct, not underfilled). `fix.params.cells[]` lists each offending cell with its `path`, `chars` and `density_pct`; `slide_fill_pct` is the grid's overall fill. Advisory (`info`) unless `fix.params.slide_mostly_empty` is true — the grid carries under 30% of its capacity across 3+ cells — which escalates it to `review` | `info` / `review` when mostly empty | `add_detail_or_resize` |
| `CHART_PLACEHOLDER_EMPTY` | `chart-insights-split` pattern rendered without a `chart` spec — left panel collapses and insights expand to full width. `action: review`. Agent action: supply a chart spec or switch to an insights-only pattern (e.g. `card-grid`, `pull-quote`) | `review` | — |
| `LOW_CONTRAST_HIGHLIGHT` | A pattern's authored highlight colour does not read as a highlight against the structure it sits in (under 3:1 fill-vs-fill — the WCAG non-text bar). Emitted by `value-chain` when `highlight_color` is set and a step is highlighted. `fix.params` carry the measured ratio in the message. Agent action: omit `highlight_color` and let the engine pick the first accent clearing 3:1 for this template, or choose a different accent — contrast is template-dependent, so a slot that works on one palette vanishes on another | `review` | — |
| `HEADLINE_TOO_LONG` | Title-class placeholder text exceeds 12 whitespace-separated words **and the title has no measurable placeholder**. Fires on `title`, `headline`, `ctrTitle` text content items. Where the layout resolves, `title_wraps` / `TITLE_OVERFLOW` is the only title-length finding and this one stands down, so a wrapped headline is reported once, by one code. Advisory — never blocks render. `fix.params: {current_words, max_words}` | `review` | `shorten_title` |
| `BODY_TOO_LONG` | A single text block exceeds 80 whitespace-separated words. Applies to `text`, `bullets`, `body_and_bullets`, and `bullet_groups` content items (bullet words aggregate per block). Advisory — never blocks render. `fix.params: {current_words, max_words}` | `review` | `reduce_text` |
| `BULLET_NESTING_DEEP` | Bullet list nests more than 2 levels. Depth is measured from leading whitespace (each tab or every 2 leading spaces = 1 indent unit); `bullet_groups` bullets start at level 2 (header is level 1). The same indent drives the rendered paragraph level, so a `"\tSecond level"` bullet really is a sub-bullet with the template's smaller size and secondary glyph (clamped at level 4). Advisory — never blocks render. `fix.params: {current_depth, max_depth}` | `review` | `reduce_text` |
| `NUMBERED_LIST_NOT_APPLIED` | Bullets carry typed `"N. "` prefixes the engine will NOT convert to auto-numbering, so they print beside the layout's own bullet glyph as a double marker. A list numbered from 1 with no gaps IS converted (the prefixes are stripped and the engine draws the numbers) and never draws this; a single line that merely opens with a number is prose and is exempt. Agent action: number every bullet from 1, or drop the prefixes | `review` | `renumber_bullets` |
| `MISSING_ALT_TEXT` | A visual has an empty `alt`. Six surfaces: an image or icon sourced from `path` / `url` / `svg_data` (`image_value`, shape-grid `image`, cell-level `icon`, shape-overlay `icon`), a chart or diagram (`chart_value`, `diagram_value`, an authored grid cell's `diagram`), and a table (`table_value`, an authored grid cell's `table`). Bundled built-in icons referenced by `name` are exempt (the qualified name supplies an implicit caption); so are the `diagram` / `table` cells of a pattern-expanded grid. A chart/diagram/table still renders with a description derived from its payload — set `alt` to one sentence saying what it shows. Advisory — never blocks render. `fix.kind: provide_value`, `fix.params: {field: "alt", kind, source}` for assets, `{field: "alt", kind, on}` for chart/diagram/table | `review` | `provide_value` |
| `DUPLICATE_TITLE` | Two or more content slides share the same title text after case-folding and whitespace collapse. Emitted on the second and later occurrences of any duplicate; the first slide is treated as canonical. Title and section-divider slides are exempt. Also lowers `score_deck.composition.score` (5 points per duplicate, capped at 30). `fix.params: {duplicate_of_slide, duplicate_slide_numbers, duplicate_count, placeholder_id}` | `review` | `shorten_title` |
| `TEXT_EXCEEDS_SHAPE` | A word in a shape_grid shape (raw grid or pattern-expanded) is wider than the text area left by the shape's preset geometry minus insets (a `chevron` keeps `w − 2·notch`, diamonds/triangles `w/2`, ellipses `0.7·w`), so it breaks mid-word or is clipped — e.g. `numbered-step-strip` chevron labels. One finding per slide. `fix.params: {cells, word, required_pt, available_pt, geometry, hint}`. Agent action: shorten the label, lower the size, or use `rect` / `homePlate` / the `stacked-box` style | `review` | `reduce_text` |
| `SPARSE_FILL` | A filled shape covering >10% of the slide holds text filling <20% of it (large, mostly empty coloured box, e.g. `kpi-3up` cards with one number). One finding per slide. `fix.params: {cells, max_text_area_pct, hint}`. Agent action: add detail, cap `max_height_pct`, or use a compact / unfilled variant | `review` | `add_detail_or_resize` |
| `SLIDE_UNDERUSED` | The ink bounding box of the slide's grid (filled shapes, text blocks, media) covers too little of the safe content area; skipped when non-title placeholders carry content (a near-empty placeholder slide is `SLIDE_NEARLY_EMPTY`). The threshold depends on who set the band's height: **45%** when the slide caps it (`bounds` / `max_height_pct`), **22%** when the pattern derived it from its content — a content-sized pattern is supposed to be shorter than the zone. `fix.params: {content_area_pct, threshold_pct, band_capped_by: author\|pattern, hint}`. Agent action: `author` → raise or drop the cap; `pattern` → add detail, pair it with a supporting zone via `compose`, or choose a denser pattern (there is no cap to remove) | `review` | `add_detail_or_resize` |
| `SPARSE_SINGLE_ROW_FLOW` | A slide-level `process-flow` (or single-row `dots` `timeline-horizontal`) of 3–6 cells whose average per-cell text is below the sparse threshold (~40 chars) and which has no `bounds` / `max_height_pct` cap, so its lone row stretches vertically into oversized boxes. Compose segments and nested cell patterns are exempt (a second zone already absorbs the height). Advisory — never blocks render. `fix.kind: swap_pattern`, `fix.params: {from, item_count, avg_chars, reason: "single_row_sparse", suggested: [{to, rationale}, …]}` ranked toward `numbered-step-strip` (detail zone), then `process-grid-2row` / `phase-roadmap`. Agent action: swap to a denser layout family via `recommend_pattern`, or set `max_height_pct` on the existing pattern | `review` | `swap_pattern` |
| `OVERTALL_FLOW_LANE` | The complement to `SPARSE_SINGLE_ROW_FLOW`: a slide-level `process-flow` / `timeline-horizontal` whose lane occupies more than ~50% of the content height with short per-cell labels, in cases the sparse guard does not cover — a `max_height_pct` cap that is still too tall (≥50), or a 7–8 step row whose narrow boxes still stretch vertically. The two never fire on the same slide. Class `pattern_choice`. Advisory. `fix.kind: swap_pattern`, `fix.params: {from, item_count, avg_chars, lane_height_pct, reason: "overtall_flow_lane", suggested: [{to, rationale}, …]}`. Agent action: swap to `numbered-step-strip` / `process-grid-2row`, or cap `max_height_pct` to ~35 | `review` | `swap_pattern` |
| `FLOW_DIAMOND_NO_CONTENT` | A standalone `process-flow` carries a decision diamond (`steps[].type: "decision"`) but has no supporting content zone to explain the yes/no branch outcomes. Compose envelopes and nested cell patterns are exempt (a second zone explains the branch). Class `pattern_choice`. Advisory. `fix.kind: swap_pattern`, `fix.params: {from, diamond_count, reason: "decision_without_branch_zone", suggested: [{to, rationale}, …]}`. Agent action: switch to `numbered-step-strip` with per-step detail, or pair the flow with an explanatory panel via `compose` | `review` | `swap_pattern` |
| `TOC_FLOWCHART_VOCAB` | An agenda / table-of-contents slide (title matches `agenda` / `table of contents` / `what we'll cover`) is drawn with a sequential flowchart pattern (`process-flow`, `process-flow-compact`, `swimlane`, `timeline-horizontal`) — a contents list is not a sequence with arrows. Class `pattern_choice`. Advisory. `fix.kind: swap_pattern`, `fix.params: {from, reason: "toc_as_flowchart", suggested: [{to, rationale}, …]}`. Agent action: switch to the `agenda` pattern, or `numbered-step-strip` in `toc` style | `review` | `swap_pattern` |
| `MATRIX_AXIS_IMBALANCE` | A `shape_grid` cell whose text-bearing shape is rotated within ~15° of 90°/270° and spans rows or columns (an axis band). Rotating the band flips its width/height about its center, so it renders wide-short (or tall-narrow) and intrudes into adjacent cells (the J2P-MATRIX-005 anti-pattern). Class `rendering` (a geometry artifact, not a pattern-choice mismatch). `matrix-2x2` now uses `vert270` text in an unrotated band, so this guards against regressions and hand-authored rotated bands. Advisory. `fix.kind: autofix_visual`, `fix.params: {reason: "rotated_band_aspect_flip", rotation_deg, guidance}`. Agent action: set the band rotation to 0 and rotate only the text via `vert: "vert270"` | `review` | `autofix_visual` |

### Density-band severity for cell capacity findings

`fit_overflow` and `cell_underfilled` share the density axis:

| Density band | Code | Severity | Action | Guidance |
|---|---|---|---|---|
| >130% | `fit_overflow` | `error` | `refuse` | Must reduce text — blocks under `strict_fit: "strict"` |
| 110–130% | `fit_overflow` | `warning` | `review` | Should reduce text before shipping |
| 60–110% | *(none)* | — | — | Healthy range — no finding emitted |
| <60% | `cell_underfilled` | `info` | `info` | Collected into ONE per-slide finding; advisory |
| <60% on a grid under 30% full | `cell_underfilled` | `warning` | `review` | The slide is mostly empty — add detail or use a smaller grid |

Underfill is aggregated because it used to be emitted per cell: a slide of KPI cards accumulated 20+ review-weight findings and bottomed its score out, while a genuinely broken layout on the same deck cost 5 points. Metric and label roles are exempt from the check entirely.

**How to triage capacity findings:**
- **`info` severity** — consider acting; not blocking under any `strict_fit` mode
- **`warning` severity** — should act before shipping; not blocking under `strict_fit: "warn"` or `"off"`
- **`error` severity** — must act; blocks generation under `strict_fit: "strict"` (MCP returns `IsError=true`)

`strict_fit` interaction: only `error`-severity findings with action `refuse` block generation in strict mode. `cell_underfilled` never blocks because its maximum severity is `warning`.

### Render-time codes — emitted during `generate_presentation` when the engine adjusted content to fit

| Code | When emitted |
|------|-------------|
| `CONTENT_DROPPED` | Shared signal for **any** path that fails to place author-provided content (a slide skipped in `--partial` mode, an unplaced content block, a truncated column, a dropped payload field). The drop has already happened — advisory, never blocks. `action: review`, `fix.kind: review` (no deterministic auto-fix). `fix.params: {locator, reason}` — `locator` labels what was dropped (`"slide 4"`, `"content block 3 (table)"`), `reason` explains why. Path targets the dropped element (`/slides/{i}` for a whole slide, `/slides/{i}/content/{n}` for a content block — e.g. a second chart/table/image resolving to a placeholder that already holds one visual). Agent action: read `locator`/`reason`, then restructure or split the slide (or use `compose` to give each visual its own region), or fix the underlying spec error so the content is no longer dropped |
| `CUSTOM_COLOR_DROPPED` | In **constrained** mode, a diagram's data payload embeds raw hex colors in per-item fields (e.g. `pyramid` `levels[].color`) that the engine ignores in favor of the template scheme. Advisory — never blocks (`action: info`; MCP `warning` severity). `fix.kind: set_design_mode_free`, `fix.params: {path, dropped_colors}`. Agent action: rerun with `design_mode: "free"` to honor the custom colors, or accept the template scheme. Scheme-color names in the same payload are allowed and not reported |
| `text_trimmed` | Trailing paragraphs trimmed to fit placeholder |
| `text_overflow` | Text still overflows placeholder after trimming |
| `TEXT_BELOW_READABLE_MIN` | Text ends up below the readability floor for its role in the deck `viewing_mode` (`present` default: 12pt body/card text, 10pt captions, 20pt titles; `read` is lower). Emitted by the fit report / preflight AND at generate time, for placeholder autofit and for `shape_grid` cells the renderer would shrink — `validate_input(fit_report: true)` predicts the same font scale generate applies, because both read the master's own body style (size, line spacing, per-paragraph space-before) (roles inferred: ≥24pt kpi-value, bold card-title, ≤40 chars caption, else card-body). `action: review`, `fix.kind: reduce_text`, `fix.params: {strategy: shorten|split, role, actual_pt, min_pt, viewing_mode}`. Agent action: shorten or split the text, or use larger cells; set `viewing_mode: "read"` only for decks that are not projected |
| `TITLE_OVERFLOW` | Title does not fit its resolved title placeholder even at the minimum autofit size, measured with the template's inherited title style (master size, all-caps, line spacing) and exact glyph widths. When a long title *does* fit by shrinking, the generator bakes the reduced size into the title runs instead (no finding). `action: shrink_or_split`, `fix.kind: shorten_title`, `fix.params: {current_chars, max_chars, font_pt, min_font_pt}`. Agent action: shorten the title to ≤ `max_chars` or move detail into the body/takeaway |
| `readability_trimmed` | Paragraphs trimmed for readability floor |
| `no_autofit_overflow` | Text overflows placeholder that has `noAutofit` set |
| `table_rows_truncated` | Table rows do not fit the placeholder height even at the minimum font scale, so trailing rows are replaced by an `"…and N more rows"` cell — **their data is absent from the deck**. Content loss, not a styling nit: `action: refuse`, so it blocks under `--strict-fit=strict` and carries a hard penalty in the deck score. Predicted BEFORE generation (so `validate --fit-report` / `validate_input` catch it) and re-emitted at render time with the same action. `fix.kind: split_at_row`, `fix.params: {visible_rows, hidden_rows, split_at_row}` — the split point is already computed. Agent action: `repair_slide(kind=split_at_row)` at `fix.params.split_at_row`, or use the `split_slide` envelope (`by: "table.rows"`) to paginate the rows across slides |
| `table_font_scaled` | Table font scaled down to the minimum floor |
| `diagram_clamped` | Diagram placeholder dimensions clamped to minimum. `action: review`, `fix.kind: swap_layout`, `fix.params: {dimension, original_emu, clamped_emu}`. Agent action: switch to a wider layout via `repair_slide` |
| `diagram_render_failed` | The chart/diagram did not render, so the slide carries a slide-sized grey `"Data unavailable"` placeholder where the visual should be (or, on a shape_grid / pattern surface, generation aborts). A lost visual is content loss: `action: refuse`, so it blocks under `--strict-fit=strict` and carries a hard penalty in the deck score. Predicted at validate/preview time for **both** surfaces, so `validate --fit-report` / `validate_input` catch it before generation. `fix.kind: review` (no deterministic auto-fix); `fix.params: {diagram_type, reason}` where `reason` is svggen's own error — for an unrecognised type it names the closest registered type (`did you mean "bar_chart"?`) and lists every allowed one. Agent action: fix the type or data shape; if the visual you want has no registered type (combo, sankey, choropleth), pick a supported type carrying the same argument or supply an image. **Not emitted for the natively-rendered diagram types** (`swot`, `business_model_canvas`, `value_chain`, `heatmap`, `pestel`, `nine_box_talent`, `porters_five_forces`, `kpi_dashboard`, `pyramid`, `house_diagram`, `process_flow` and the panel family) — the generator draws those as OOXML shapes, so svggen's verdict does not apply to them |
| `column_width_deficit` | Column widths fell back to global floor |
| `pagination_default_threshold` | Pagination used default threshold (no template capacity available) |
| `contrast_autofixed` | Text color auto-replaced to reach WCAG AA **for that text's size**: 4.5:1 for normal text, 3:1 only for genuinely large text (≥18pt, or ≥14pt bold). `action: info`, `fix.kind: replace_color`, `fix.params: {original_color, replacement_color, background_color, contrast_ratio_before, contrast_ratio_after, source}` where `source` is `shape_grid`, `shape_grid_group`, `lstStyle`, `run`, `chrome`, `layout-lstStyle` or `master-txStyles` (`shape_grid_group` is one decision covering several sibling cells — `fix.params.cells` counts them, the path is the grid rather than a single shape, and the ratios are measured against the worst fill in the group) (the last two are text that named no color and inherited an unreadable one from the layout or the master). Path locates the swap: `/slides/{i}/shape_grid/shapes/{n}` for grid cells (flat rendered-shape index), or the slide-level `/slides/{i}` for layout/run text. Slide index is derived from `path` |
| `contrast_predicted` | Preflight prediction (validate / preview) that a shape-grid text color will be auto-replaced for WCAG AA at render time. Uses the **same replacement algorithm** as the render-time pass, so `predicted_replacement` equals the `contrast_autofixed` color for the same resolved fg/bg/template. Same `fix.kind` (`replace_color`) as `contrast_autofixed`, with `fix.params.predicted_replacement`, `replacement_mode` (`flip` = pure-neutral white/black text snapped to the first template color meeting AA, tried in order `lt1`, `dk2`, `dk1`, then a darker/lighter shade of the fill; `lerp` = darkened/lightened via EnsureContrast), and `source` (`shape_grid`, or `slide_background` for placeholder text on a background the slide sets itself). On an author-set background (`background.color`, or an opaque scrim) the replacement always `flip`s — the template's text colour was not chosen against that background, so there is no hue to preserve — while the template's own backgrounds and `shape_grid` cells keep the `lerp`. `action: info` |
| `placeholder_remapped` | A content `placeholder_id` was resolved to a different placeholder on the layout (e.g. `subtitle` → `body` on the `section` layout). Emitted both pre-flight (`preview_presentation_plan` and `generate_presentation` with `fit_report:true` — one finding per `resolved_slides[].placeholders[]` with `remapped:true`) and at render time. `action: info`, `fix.kind: remap_placeholder`, `fix.params: {from, to}`. Path targets `/slides/{i}/content/{j}/placeholder_id`. Agent action: author the resolved id directly to avoid the implicit rewrite |
| `grid_diagram_narrow` | Complex diagram in a narrow grid cell (<50% slide width). `action: review`, `fix.kind: reshape_grid`, `fix.params: {diagram_type, complexity, cell_width_pct, cell_width_emu, threshold_emu}`. Path targets the diagram field (e.g. `/slides/0/shape_grid/rows/0/cells/1/diagram`). Agent action: widen cell via `repair_slide` with `reshape_grid` fix |
| `diagram_aspect_mismatch` | Fires **only** when a diagram sets **both** explicit `diagram.width` and `diagram.height` and that **authored** aspect differs from the **post-fit render frame** aspect by >25%; the chart will be stretched or letterboxed. Unset or single-axis (`width`-only / `height`-only) diagrams adapt to the frame aspect at render and are **not** flagged (natural-aspect types are covered by `diagram_aspect_conflict`). `action: review`, `fix.kind: reshape_grid`. `fix.params` carry four aspect signals so you can separate an authoring mistake from a fit-driven mismatch: `{diagram_type, authored_width, authored_height, authored_aspect, effective_width, effective_height, effective_aspect, dimension_source, cell_width_emu, cell_height_emu, cell_aspect, render_width_emu, render_height_emu, render_aspect, fit_adjusted, deviation}` — `authored_*` = the explicit dims; `effective_*` + `dimension_source` = what the resolver produced (equal to authored for an explicit spec, `dimension_source: "explicit"`); `cell_*` = the **original (pre-fit)** cell; `render_*` = the **post-fit** frame; `fit_adjusted: true` means a `cell.fit` reshaped the cell into the frame. `measured` is the post-fit render frame, `allowed` the effective render dims. Path targets the diagram field. Agent action: resize the cell, set `cell.fit` (`contain`/`fit-width`/`fit-height`), or change the explicit `diagram.width`/`diagram.height` to match the frame |
| `diagram_aspect_conflict` | Non-chart diagram cell (or placeholder) aspect deviates from the diagram type's natural svggen viewBox aspect by >30%. Emitted for diagrams pinned to fixed natural aspects via `svggen.NaturalAspect` (currently `timeline` 2:1, `gantt` ~1.8:1, `org_chart` ~1.57:1). Silent for chart types (covered by svggen dry-render `chart.*`) and for diagrams with explicit `DiagramSpec.Width/Height` (covered by `diagram_aspect_mismatch`). `action: review`, `fix.kind: reshape_grid`, `fix.params: {diagram_type, natural_aspect, cell_aspect, deviation, cell_width_emu, cell_height_emu}`. Path targets the diagram field. Agent action: resize the cell, set `cell.fit`, or set explicit `diagram.width`/`diagram.height`. Available at validate + preview without invoking resvg/inkscape |

### Budget summary code

Emitted when more than `DefaultFindingBudget` (5) findings exist on a slide and `verbose_fit:false`:

| Code | When emitted |
|------|-------------|
| `findings_truncated` | Per-slide finding budget exceeded; remaining findings suppressed. `action: info`, `fix.kind: truncation_summary`, `fix.params: {suppressed_count: int, top_codes: ["code:count", ...] sorted by count desc}`. Pass `verbose_fit: true` (MCP) or `--verbose-fit` (CLI) to see all findings without truncation |

### Icon preflight codes — emitted before render to catch broken `icon.name` / `icon.path` fields

| Code | When emitted |
|------|-------------|
| `ICON_BUNDLED_NAME_UNKNOWN` | `icon.name` does not resolve in the bundled icon registry. Emitted by `validate_input` and `generate_presentation` preflight so agents can fix typos and missing `filled:` prefixes without burning a generate cycle. `severity: error`. `details: {input_value, suggestions: ["chart-pie", ...], slide_index, remediation}`. Path targets the icon node, e.g. `/slides/0/shape_grid/rows/0/cells/0/icon`. `suggestions[0]` is the highest-ranked Levenshtein match (or qualified cross-set form when the bare base name only resolves in a non-default set). Agent action: replace `icon.name` with the suggested value, or call `list_icons` to discover the canonical `qualified_name` |
| `ICON_NOT_FOUND` | `icon.path` does not point at an existing file after resolution against the JSON input directory (CLI) or server CWD (MCP). `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}`. Agent action: verify the file path is correct, switch to a bundled icon via `name`, or supply inline `svg_data` |
| `ICON_PATH_EXT_INVALID` | `icon.path` extension is not `.svg`. Icons must be SVG so they can be re-tinted; raster images should use `image_value` or shape-grid `image` cells. `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}` |
| `ICON_PATH_TRAVERSAL` | `icon.path` contains `..` components. Rejected before `filepath.Clean` collapses them, so agents can't escape the base directory via a constructed relative path. `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}` |
| `ICON_PATH_SYMLINK_ESCAPE` | `icon.path` is relative and its symlink chain resolves outside the base directory. `severity: error`. `details: {input_value, resolved_path, asset_kind: "icon", slide_index, remediation}`. Agent action: pin an absolute path explicitly, or remove the offending symlink |
| `ICON_PATH` | Other `icon.path` resolution failures not covered by the more specific codes above (symlink loop, permission denied, etc.). `severity: error`. `details: {input_value, asset_kind: "icon", slide_index, remediation}` |
| `ICON_AMBIGUOUS` | Multiple icon source fields are set (e.g., both `name` and `path`). `severity: error`. `details: {conflicting_fields: ["name", "path", ...], slide_index, remediation}`. Message names exactly which fields conflict so the agent does not have to re-read the JSON. Agent action: keep one of `name`, `path`, `url`, or `svg_data` and remove the others |
| `ICON_MISSING` | No icon source field is set on an `icon` node. `severity: error`. `details: {slide_index, remediation, example}`. Message includes a 4-line copy-paste example block, one per source variant. Agent action: pick the variant that fits the use case |
| `ICON_FILL_IGNORED_ON_INLINE` | `icon.fill` is set together with `icon.svg_data`. The inline SVG is rendered verbatim, so `fill` has no effect. `severity: warning` (non-blocking). `details: {input_value, slide_index, remediation}`. Agent action: either pre-color the inline `svg_data` markup, or remove `svg_data` and use `name`/`path` with `fill` |

### Pattern input codes — emitted by `validate_input` / `generate_presentation` / `expand_pattern` before a pattern expands

A pattern failure is reported **one finding per problem**, each addressed at `/slides/{i}/pattern/values/...` (a nested cell pattern keeps its coordinates: `/slides/{i}/shape_grid/rows/{r}/cells/{c}/pattern/values/...`). Messages name the field and the expected shape in schema terms — never a Go type — and every finding carries `next_tool_call: show_pattern{name}`.

| Code | Meaning | Severity | `fix.kind` |
|------|---------|----------|------------|
| `PATTERN_UNKNOWN_FIELD` | A key in `values` / `overrides` / `cell_overrides` is never read by the pattern, so the text under it never reaches the slide. Detected by decoding with the pattern's own decoder and checking whether the content survives — the tolerated aliases (a KPI cell's `{value, label}`, the `"$4.2M \| ARR"` shorthand) never trip it, and free-form objects (a chart's map-form `data`) are not judged. **Keys inside a cell count too** (`values[0].bogus`, `values.cells[2].bodySize`): a list element declared as `oneOf{shorthand string, object}` is read through its object branch, which is where the field names live (go-slide-creator-4cqh). `fix.params: {path, from, to, did_you_mean}` when a property is close (`title` → `role`, `label` → `name`, `columns` → `rows`, `date` → `date_label`), else `{path, allowed: [...]}`. Agent action: rename the key, or move the content to a field the pattern reads | `error` | `rename_field` / `remove_field` |
| `invalid_shape` | The JSON type at a path is one the pattern cannot read: a string where an object belongs, an object wrapping the array `values` IS. `fix.params: {path, expected, got}` plus `example` (a copy-ready value built from your own content, e.g. `{"label": "A"}` for a bare `"A"`) or `unwrap_key` when the payload wraps the array the pattern wants. Agent action: apply the example, or unwrap the named key | `error` | `reshape_value` |
| `UNKNOWN_PATTERN` | `pattern.name` is not registered. `did_you_mean` folds number words to digits before matching, so `kpi-four-up` → `kpi-4up` and `matrix2x2` → `matrix-2x2` resolve. `fix.params: {from, to, did_you_mean}`; `next_tool_call` is `show_pattern{name: <suggestion>}`, or `list_patterns` when nothing is close | `error` | `swap_pattern` |

Every `maxLength` budget counts **characters, not bytes**: `"€980.2M"` is 7 characters and fits an 8-character budget, and `"Vollständig erfüllt"` is 18 against a 20-character label. A `max_length` finding's `(N chars)` is that character count (go-slide-creator-5ok4).

Pattern rule violations (`required`, `max_length`, `min_items`, `max_items`, `count_mismatch`, `out_of_range`, `unknown_enum`, `wrong_pattern`, `ICON_BUNDLED_NAME_UNKNOWN`) are reported the same way — one finding per failing field, at the field's own path — rather than newline-joined into a single `PATTERN_ERROR` message.

### Slide-background codes — emitted by `validate_input` / `generate_presentation`

| Code | Meaning | Severity | `fix.kind` |
|------|---------|----------|------------|
| `TEXT_OVER_IMAGE_UNVERIFIED` | A slide sets `background.image` / `background.url` with no `background.overlay`, and puts text on it. The contrast pass reads a solid background fill, so a photo is invisible to it — the template's own title colour can land on the dark half of the picture and nothing reports it. `fix.params: {path, value:{color,alpha}, hint}`. Agent action: add `background.overlay {color:"dk1", alpha:0.45}`, move the text off the picture (`image-text-split`), or set `contrast_check: false` when the photo is known to be uniform under the text | `review` | `provide_value` |

### Content-policy codes — emitted by `validate_input` / `generate_presentation` over user-visible text

| Code | Meaning | Severity | `fix.kind` |
|------|---------|----------|------------|
| `unresolved_placeholder` | A user-visible string still holds the `__FILL__` skeleton placeholder that `plan_deck` emits. The JSON-based scan covers placeholder text values, bullets, speaker notes, shape_grid cell text, table cells, chart/diagram labels, and pattern values. Controlled by the `placeholder_policy` parameter (`off`\|`warn`\|`strict`, default `warn`): `warn` reports each token with its JSON path while keeping `valid: true`; `strict` promotes them to errors that fail validation / refuse generation (the publishable/gated mode). `fix.params: {path, token, hint}`; `next_tool_call` re-runs `validate_input`. Agent action: replace every `__FILL__` with real content before publishable generation | `warning` (strict: `error`) | `replace_placeholder` |

### Scoring-facade evidence codes — emitted by `score_deck` / `auto_repair` / `make_deck`

| Code | Meaning | Action | `fix.kind` |
|------|---------|--------|------------|
| `RENDER_EVIDENCE_INCOMPLETE` | The render pass that backs the deterministic score failed (slide conversion, temp-dir creation, or generation), so the score reflects static analysis only. Emitted so an empty render-finding set is never mistaken for a clean render; it counts toward the P0 gate criterion and blocks `quality_gate` / `gate_passed`. The facades also attach a structured `render_evidence` block (`{complete, stage, detail, degraded}`). Pass `allow_degraded_scoring: true` to drop it to advisory (`review`) and converge on static analysis alone — `evidence_complete` then stays `false` and final `output_validation` still blocks. See [Validation evidence on the repair facades](SKILL.md). Not source-repairable. | `refuse` (degraded: `review`) | none |

### Action semantics (shared with chart codes)

- `refuse` — with `strict_fit: "strict"`, generation is blocked and MCP returns `IsError=true`; with `warn`, emits finding only
- `shrink_or_split` — content will be adjusted or distributed; strict promotes to `refuse` for content-loss codes
- `review` — informational; agent should inspect but no automatic remediation
- `info` — advisory/telemetry only, never promoted

---

## `fix.kind` enum (fit-report — stable for programmatic matching)

This is the enum the engine emits in `validate_input` / `preview_presentation_plan` / `generate_presentation` fit-report findings. `repair_slide` accepts a *superset* — see the next section.

| Kind | Semantics | Params |
|------|-----------|--------|
| `reduce_text` | Shorten text content in the indicated path | — |
| `split_at_row` | Emit `split_slide` at the given row index | `row: int` |
| `use_semantic_color` | Replace hex fill with `accent1`/`lt2`/`dk1`/… | `message?` |
| `replace_color` | Swap one explicit color for another | `from, to` |
| `replace_value` | Replace an invalid value with a suggested one | `suggestion, allowed?` |
| `provide_value` | Required field is missing | `field` |
| `use_one_of` | Value must be one of an allowed set | `allowed` |
| `rename_field` | Unknown field name close to a known one | `from, to` |
| `reshape_value` | Value has wrong structure (array vs object, etc.) | `path, value` |
| `remove_field` | Unknown field should be removed | — |
| `add_detail_or_resize` | Cell is underfilled — add more text or use a smaller grid | `current_density_pct: int` |

Chart/diagram codes (below) introduce their own `fix.kind` values: `reduce_items`, `explicit_scale`, `truncate_or_split`, `align_series`, `increase_canvas`.

---

## Pattern slides are measured too

`slides[].pattern` carries no `shape_grid` until generation, so every fit finding below is evaluated against the pattern **expanded once** into the grid generation renders. Two consequences for agents:

- Paths on a pattern slide are rooted at `/slides/N/pattern/rows/R/cells/C/...`, not `/slides/N/shape_grid/...` — the deck has no `shape_grid` at that index.
- A text fix that would edit a grid cell (`reduce_cell_text`) is replaced by the advisory `rewrite_field`, carrying the measured `max_chars` and the `pattern` name. The cell does not exist in your JSON: shorten the pattern's own `values`, then re-run `expand_pattern` to confirm the new density.

## Fix kinds for `repair_slide` — complete table

The apply-only superset accepted by `repair_slide` is broader than the fit-report `fix.kind` enum: fit-report only emits kinds the engine can derive automatically, while `repair_slide` also accepts kinds the *agent* decides to apply (e.g., `swap_layout`, `swap_pattern`, `autofix_visual`). Every kind below is a `case` in `applyRepairFix` (`cmd/json2pptx/mcp_repair.go`); the drift test `cmd/json2pptx/skill_drift_test.go` enforces this list and the kinds advertised by `get_capabilities().vocabularies.repair_fix_kinds` stay in sync.

| Kind | Semantics | Required params | Optional params |
|------|-----------|-----------------|-----------------|
| `reduce_text` | Shorten a content item's text, bullets, `body_and_bullets` or `bullet_groups`. A **word** budget (`max_words`) or **character** budget (`max_chars`, alias `max_length`) is distributed across bullets in proportion to their length, so nine long bullets become nine short ones (each cut at a word boundary with a single `…`) rather than the list being truncated — dropping bullets would lose points. `max_items` still cuts the list itself. Floors: 4 words / 24 chars per bullet. Refuses with `code: "semantic_review_required"` when a trim would drop a number, unit, negation, or qualifier (override with `confirm_semantic_change: true`) — for a bullet list, for `body_and_bullets`, and for `bullet_groups`, where the guard reads the dropped groups' `group_label`, `header`, `body` and bullets alike, because removing a group removes all of them (go-slide-creator-sx53). On a slide whose text lives in a `shape_grid`, it cannot reach the text: the answer is `code: "wrong_kind_for_target"`, `did_you_mean: "reduce_cell_text"` and a `next_tool_call` carrying the corrected directive with `cell_path`. | at least one of `max_words: int`, `max_chars: int` (alias `max_length`), `max_items: int` | `path: string` (JSON Pointer to one content item), `confirm_semantic_change: bool` |
| `shorten_title` | Truncate the title placeholder text | — | `max_length: int` (default 50), `path: string` |
| `renumber_bullets` | Renumber a placeholder's bullets `1. `, `2. `, `3. ` … — the shape the engine converts to OOXML auto-numbering, which strips the typed prefixes and draws one marker. Pass `strip: true` to remove the prefixes instead, when the list is not ordered after all. Reaches `bullets_value` and `body_and_bullets.bullets` | — | `path: string` (JSON Pointer to the content item), `strip: bool` |
| `split_at_row` | Wrap the slide in a `split_slide` envelope, distributing table rows across pages | `row: int` (rows per page; alias `group_size`) | `title_suffix: string` (default ` ({page}/{total})`), `repeat_headers: bool` (default true), `path: string` |
| `swap_layout` | Change the slide's `layout_id` | `layout_id: string` | — |
| `use_one_of` | Replace a slide-level enum field (`layout_id`, `transition`, `transition_speed`, `build`) or a content `type` with an allowed value | `path: string`, `value: string` | — |
| `replace_color` | Replace a specific fill color anywhere in `shape_grid` cells (string or object form). Accepts `contrast_autofixed` finding params as aliases. | `from: string` (or `original_color`), `to: string` (or `replacement_color`) | — |
| `use_semantic_color` | Replace hex fills with a semantic scheme name (`accent1`, `dk1`, ...). With `path` set, targets one cell; without, replaces all hex fills on the slide. | `value: string` (scheme color name) | `path: string` (cell fill path) |
| `split_pattern` | Split a pattern-driven shape_grid into two slides at a computed row boundary. Useful for overflowing grids without changing the pattern. With `path` on a pattern slide, splits the `pattern.values[path]` array instead (slide 1 keeps the first `first` items, slide 2 the rest) — the fact-preserving alternative to `reduce_items`/`resize_list`. | — | `first: int` (cells/items on slide 1; default = half), `title_part_2: string` (suffix; default `"(continued)"`), `path: string` (pattern.values array key) |
| `swap_pattern` | Replace the slide's pattern with a different one; optionally replace `values`, `overrides`, `cell_overrides`. Clears any expanded `shape_grid` for re-expansion. | `to: string` (target pattern name) | `values: object`, `overrides: object`, `cell_overrides: object` |
| `reshape_grid` | Change grid dimensions. For pattern slides, updates `rows`/`columns` in pattern values; for raw `shape_grid` slides, redistributes cells into a new row/column layout. | One of `rows: int` or `columns: int \| [int]` | both |
| `set_pattern_style` | Set the `style` key in the pattern's `overrides` (e.g., `timeline-horizontal` from `"dots"` to `"chevron"`) and clear expanded grid for re-expansion. | `style: string` | — |
| `set_max_height_pct` | Cap a pattern slide's height budget (`slides[i].pattern.max_height_pct`) so its boxes shrink to their content instead of stretching to fill the slide, and clear the expanded grid for re-expansion. The mechanical remedy the underfill / overtall-lane findings point at (~35 for a single sparse row). Refused on a slide with no `pattern` — cap a raw `shape_grid` with explicit bounds or row heights instead. | `max_height_pct: number` (0 < pct ≤ 100) | — |
| `reduce_cell_text` | Truncate one cell's text to a character budget. On a **pattern** slide (no `shape_grid` in the deck JSON) the cell path is resolved back to the `pattern.values` string that produced the cell and that value is shortened; when the cell text is composed at expansion and matches no single value, it refuses with `code: "wrong_kind_for_target"`, `did_you_mean: "replace_value"`. Accepts either spelling of the path (`/slides/N/shape_grid/rows/R/cells/C` or `/slides/N/pattern/rows/R/cells/C`), appending U+2026 and stripping orphaned markdown emphasis markers. Use only when the agent should not rephrase the text. | `cell_path: string` (JSON Pointer e.g. `"/slides/0/shape_grid/rows/1/cells/2"`), `max_chars: int` (> 1) | — |
| `rename_field` | Rename a top-level key. Searches pattern values first, then slide-level fields via JSON round-trip. | `from: string`, `to: string` | — |
| `reshape_value` | Replace a pattern-values field with a restructured replacement (array→object, etc.). | `path: string` (field name), `value: any` | — |
| `provide_value` | Set a pattern-values field that is missing | `path: string`, `value: any` | — |
| `replace_value` | Replace an existing pattern-values field with a new value (e.g., to bring it within valid bounds) | `path: string`, `value: any` | — |
| `reduce_items` | Truncate a pattern-values array field to `max_items`. **Refused** (`code: "semantic_review_required"`, `next_tool_call` → `repair_slide` with `split_pattern{path, first}`) when a dropped item contains a number, unit, negation, or qualifier. | `path: string`, `max_items: int` (> 0) | `confirm_semantic_change: bool` (bypass the guard) |
| `add_items` | Append agent-supplied items to a pattern-values array field | `path: string`, `items: array` | — |
| `resize_list` | Resize a pattern-values array field to exactly `count` items. Truncates if too many (same fact-loss guard as `reduce_items`); returns not-applied if too few (agent must follow up with `add_items`). | `path: string`, `count: int` (> 0) | `confirm_semantic_change: bool` |
| `remove_key` | Delete a key from the pattern's `overrides` (preferred) or `values` | `key: string` | — |
| `remove_field` | Delete a top-level field from pattern values or from the slide (via JSON round-trip) | `path: string` | — |
| `autofix_visual` | Map a visual-QA finding category to one or more candidate fix kinds and try them in order. Caller-supplied params are forwarded (caller wins). | `category: string` (visual QA finding category) | any params forwarded to the underlying kind |

### Executable vs advisory fix kinds

The table above is the **executable** vocabulary: what `repair_slide` applies, and exactly what `get_capabilities().vocabularies.repair_fix_kinds` advertises. Findings also emit **advisory** kinds — real, documented remedies that need an authoring decision rather than a mechanical edit (`add_detail_or_resize`, `adopt_pattern`, `consolidate_accents`, `fix_structure`, `grow_pattern`, `increase_gap`, `increase_row_height`, `provide_data`, `provide_native_format`, `provide_numeric_value`, `reduce_columns`, `remap_placeholder`, `remove_emoji`, `remove_field_or_switch_pattern`, `replace_placeholder`, `reposition_shape`, `review`, `review_layout`, `rewrite_field`, `set_design_mode_free`, `shrink_text`, `text`, `truncation_summary`). They are advertised as `get_capabilities().vocabularies.advisory_fix_kinds`. On a clean deck most fix-carrying findings are advisory — that is the normal end state of the repair loop, not a failure.

An **advisory** kind sent to `repair_slide` returns the decision to make, not a rejection:

```json
{
  "applied": false,
  "code": "advisory_fix_kind",
  "message": "The shape is much larger than its text. Add the supporting detail the box was sized for, cap the grid height (pattern.max_height_pct or explicit bounds) so it shrinks to its content, or use a compact pattern variant. …",
  "alternatives": ["set_max_height_pct", "reshape_grid", "swap_pattern"],
  "supported_kinds": ["add_items", "autofix_visual", "provide_value", ...]
}
```

Act on `message`, or apply one of `alternatives` (all executable). Do **not** retry the same kind. `propose_repairs` does the same split for you: advisory findings land in `advisory[]` with `{kind, guidance, alternatives, code, slide_index, path, message, params}` and are counted in `summary.advisory_findings`.

A kind aimed at text it cannot reach returns the kind that can, plus a ready-to-send directive:

```json
{
  "applied": false,
  "code": "wrong_kind_for_target",
  "did_you_mean": "reduce_cell_text",
  "message": "this slide's text lives in a shape_grid cell, not a content item — reduce_text cannot reach it; apply reduce_cell_text with cell_path \"/slides/0/shape_grid/rows/0/cells/0\"",
  "next_tool_call": {"tool": "repair_slide", "args_template": {"slide_index": 0, "fixes": [{"kind": "reduce_cell_text", "params": {"cell_path": "/slides/0/shape_grid/rows/0/cells/0", "max_chars": 90}}]}}
}
```

Submit that `next_tool_call` verbatim. `propose_repairs` applies the same correction up front: a fit finding whose path points inside a `shape_grid` cell yields a `reduce_cell_text` directive with the cell path, never `reduce_text`.

An **unknown** kind — one in neither vocabulary — is a caller mistake and keeps the original answer:

```json
{
  "applied": false,
  "code": "kind_not_supported",
  "message": "kind_not_supported",
  "supported_kinds": ["add_items", "autofix_visual", "provide_value", ...],
  "next_tool_call": {"tool": "get_capabilities", "args_template": {}}
}
```

`supported_kinds` is the full authoritative executable vocabulary inline — recover by retrying with one of those kinds instead of issuing a separate `get_capabilities` call. The `next_tool_call` is still surfaced as a fallback for agents that want to consume the canonical capabilities snapshot. Both vocabularies live in `internal/patterns/fix_kinds.go`; `TestEveryEmittedFixKindIsRegistered` fails if a finding invents a kind, and `TestRepairFixKindsMatchApplySwitch` keeps the executable list, the `applyRepairFix` switch, and `get_capabilities` in lock-step.

---

## Chart Finding Codes

Charts and diagrams emit structured findings at render time, following the same `{path, code, message, fix}` envelope as native layout findings. Codes use the `chart.*` prefix.

**Dry-render parity:** `validate_input` (with `fit_report: true`) and `preview_presentation_plan` now invoke svggen's layout/labeling pass for every `chart_value` / `diagram_value` content item and merge the resulting `chart.*` findings into `fit_findings`. Agents see `chart.tick_thinned`, `chart.label_clipped`, `chart.legend_overflow_dropped`, `chart.label_truncated`, and `chart.scatter_label_skipped` BEFORE calling `generate_presentation` — no full render required. The same strict-fit severity ladder applies. For ad-hoc per-diagram dry-runs use the svggen-mcp `render_diagram` tool with `dry_run: true`.

### Data-integrity codes — indicate bad input data

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.invalid_numeric` | NaN/Inf values clamped during render | `replace_value` |
| `chart.zero_sum_pie` | Pie/donut with all-zero or all-negative values | `replace_value` |
| `chart.negative_on_log` | Negative values on a log-scale chart | `explicit_scale` |
| `chart.all_zero_series` | All series values are zero (flat chart) | `replace_value` |
| `chart.capacity_exceeded` | Series/points/categories exceed renderer limits | `reduce_items` |
| `chart.invalid_time_format` | Time-series string cannot be parsed | `replace_value` |

### Content-loss codes — successful degradation that dropped or truncated payload; promoted under `warn`

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.legend_overflow_dropped` | Legend entries dropped (area exceeded). The chart draws a `+N more` row in the last slot rather than stopping silently, so the slide itself says entries are missing | `reduce_items` |
| `chart.overflow_suppressed` | Overflow content suppressed or truncated | `reduce_items` |

(`chart.capacity_exceeded` is also a content-loss code but is grouped with data-integrity above because strict promotes it all the way to `refuse`.)

### Advisory codes — informational fitting/labeling adjustments; never promoted

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.auto_log_scale_applied` | Auto-switched to log scale based on data range | `explicit_scale` |
| `chart.tick_thinned` | Axis tick labels thinned to prevent overlap | `reduce_items` |
| `chart.scatter_label_skipped` | Scatter label skipped due to collision | `increase_canvas` |
| `chart.label_truncated` | Label truncated to fit available space | `increase_canvas` |
| `chart.label_ellipsized` | Label shortened with ellipsis (x-axis categories: only after a two-line horizontal wrap and rotation both fail) | `increase_canvas` |
| `chart.label_clipped` | Label hard-clipped at container boundary | `increase_canvas` |

### Strict-fit promotion ladder for chart codes

Matches `svggen/core/finding_codes.go::promotionTable`:

| Level | `chart.capacity_exceeded` | `chart.legend_overflow_dropped`, `chart.overflow_suppressed` | Data-integrity codes (5) | Advisory codes (6) |
|-------|---------------------------|-----------------------------------------------------|------------------------|--------------------|
| `off` | (no promotion) | (no promotion) | (no promotion) | (no promotion) |
| `warn` | `shrink_or_split` | `shrink_or_split` | (no promotion) | (no promotion) |
| `strict` | `refuse` | `shrink_or_split` | `refuse` | (no promotion) |

Example chart finding in a fit report:

```json
{
  "path": "slides[1].content.chart_value",
  "code": "chart.capacity_exceeded",
  "message": "12 series exceeds max_series=50 — truncated to first 50",
  "severity": "shrink_or_split",
  "fix": { "kind": "reduce_items", "params": { "limit": 50 } }
}
```
