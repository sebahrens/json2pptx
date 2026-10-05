# Findings and repair decisions

Use this guide when a deck tool returns findings. Do **not** load a static
finding-code catalog: call `describe_finding` for an unfamiliar code. Deeper
rationale: [docs/FIT_FINDINGS.md](../../docs/FIT_FINDINGS.md).

## DeckSpec findings (`validate_deck_spec`, `render_deck_spec`)

Each finding has `code`, `severity`, `blocking` (true exactly when severity
is `error`), `path` — a JSON Pointer (0-based) into the spec you sent, the
string a patch takes — `missing_path` when the field does not exist yet
(`path` is then its parent) and `slide_number` (1-based; "slide N" in any
message is that number, "slide index N" is 0-based). Ignore `debug`.

- **One entry per cause.** `occurrences` + `paths` stand for several
  findings: fix every pointer; `evidence.measured` / `allowed` / `text` and a
  budget in `params` are lists in that order when they differ.
  `TEXT_BELOW_READABLE_MIN` is per slide, `evidence.text` the text measured;
  on a `raw_json2pptx` slide `path` is the pattern value
  (`…/slide/pattern/values/…`). `symptoms[]` (codes
  only) are consequences of the entry — fix the entry, not the symptoms.
- **One remedy.** `message` says what to do in the fields of your spec,
  `remediation.primary` is the action — `apply_patch`, `shorten_text`
  (`max_chars` / `max_words`), `reduce_items` (`max_items`, `min_items`),
  `split_slide` (`max_rows`, `row`), `replace_value` (`did_you_mean`,
  `available`, `expected_shape`, `example`), `add_detail_or_merge` (`hint`;
  `SLIDE_UNDERUSED` / `SPARSE_FILL` / `SPARSE_SINGLE_ROW_FLOW`, render's
  `recommended_edit`) — and
  `next_tool_call` the patch.
  `patch_verified: true`: the server applied that patch and re-validated —
  the finding is gone and nothing new blocks; send it unchanged. A patch
  without the flag holds an `<instruction>` value: write your own words
  within the budget. An over-full slide's message ends `— verified fix:
  removing /slides/7/options/0/detail … clears this` (least loss first: a
  line, a list's detail lines, an entry, the layout switch; never a required
  takeaway); a label the layout
  writes ("RECOMMENDED") is never yours to shorten — cut what the slide
  holds. `describe_finding` is offered only on a blocker with no remedy.
- `SEMANTIC_PATTERN_DEGRADED` is reported per over-budget item
  (`params.max_chars`; `from` = the pattern whose budget was missed) and is
  an `error` when the fallback does not fit either. `SEMANTIC_UNKNOWN_KIND` /
  `SEMANTIC_UNKNOWN_FIELD` carry `did_you_mean` (with `hosted_type`, the name
  was a chart or diagram type and `did_you_mean` is the kind that hosts it:
  `funnel` → `chart_insight`); the patch moves an unknown key to it, or
  removes it. A finding with `evidence.caused_by` follows from the dropped
  key there: fix the key first.
- `QUALITY_GATE` (error at `slides`) has no fix of its own:
  `evidence.criterion` (`min_score` | `max_topic_title_pct` |
  `min_composition_score` | `max_problem_slides_pct`) and
  `evidence.counted[] {code, path}` name the advisories to clear.
- Composition faults cost 25 points on their slide and count toward
  `max_problem_slides_pct`; none blocks alone, a deck of them fails the
  gate. `VERTICAL_IMBALANCE` / `HORIZONTAL_IMBALANCE`: PATTERNS.md →
  Placement.
  `TEXT_WRAPS_NARROW`: a paragraph wraps to 5+ lines of ≤3 words — cut each
  box to `max_words`, or keep at most `max_boxes` boxes on the row; `paths`
  lists the box behind each hit. `SIBLING_SIZE_MISMATCH`:
  peer labels at two sizes (`cells[].written_pt` is the file's,
  `rendered_pt` a renderer's) — shorten the longest or use fewer columns.

## Raw-deck findings

Each finding has `{path, code, severity, action, fix}`;
`fix` has `kind` and `params`. `path` is a JSON Pointer that resolves in the
deck you sent (`/template`, `/slides/1/content/1/table_value/rows/0/2/conditional/rule`,
`/defaults/table_style/style_id` for a default a table adopted,
`/slides/1/base/…` in a `split_slide`, `/structure/sections/0/slides/2/…`); a
field to add is named by the pointer it will have. `slide_number` is the
rendered slide (1-based); a tool's `slide_index` is `slide_number - 1`.
Ignore `debug`. The envelope has `ok` plus an ordered `findings[]`. Work
top-down: severity descending, then slide, then code; deck-level first.
A pattern text-budget warning (`BODY_TOO_LONG`, `HEADLINE_TOO_LONG`,
`TEXT_EXCEEDS_SHAPE`) that names one value has that value's pointer as
`path` (`/slides/N/pattern/values/2/big`, `…/values/steps/3/label`) and,
when it states a budget, `fix: {kind: "rewrite_field", params: {path,
max_chars, pattern}}`: rewrite that field yourself within `max_chars`.

### What to do with a finding

- `refuse`: the engine cannot safely produce the result.
  Repair the source and validate again; a written artifact is not success.
- `shrink_or_split`: the content needs more room or less text. Preserve facts;
  split the slide or rewrite copy, never truncate blindly.
- `review` / `info`: a judgment for the author; inspect the rendered slide
  and decide. `fit_overflow` at `info` (one
  per slide): a `shape_grid` cell fits only through a stored autofit scale,
  so its size differs between PowerPoint and LibreOffice. Apply `fix`
  (`reduce_cell_text`: `cell_path`, `max_chars`; `cells` lists every
  affected cell) or give the cells height.

`score_deck` classifies a finding as `pattern_choice` (a
different visual family), `rendering` (fit, geometry or contrast repair) or
`content` (e.g. `TITLE_NOT_ACTION`, `TITLE_TOO_LONG`: a better title,
evidence, labels or copy). `DATA_WITHOUT_SOURCE` (review: a
chart, figures table / matrix or KPI / stat pattern, unsourced) is one:
set `slides[N].source` (`fix.params.field: "source"`), a chart `footnote` or
a deck default (QUALITY.md §5); never invent one.

Storyline gate (`score_deck`):
`takeaway_missing` → require_takeaway_on_charts; `TITLE_NOT_ACTION` (over 15
words, stock label, no verb/number, or `topic_label`: a ≤4-word label over a
`takeaway`; `fix.params.reason`) on >25% of slides →
max_topic_title_pct; `NO_EXECUTIVE_SUMMARY` (6+ slides) and
`CLOSING_WITHOUT_NEXT_STEPS` ("Thank you" closer, no ask) → require_storyline.
`SLIDE_TEXT_DENSE` (>6 bullets, >80 words, bullet over 2 lines) costs score.

On raw decks, `propose_repairs` turns findings into executable `directives`
and `advisory[]`. The
executable and advisory vocabularies are
`get_capabilities().vocabularies.repair_fix_kinds` (params in
`repair_fix_kind_params`) and `advisory_fix_kinds`. `repair_slide` applies
one slide's executable directives. A finding's
`fix.kind` is not necessarily executable: if it needs human judgment,
`repair_slide` returns `advisory_fix_kind` with alternatives.

Text-reduction fixes refuse with `semantic_review_required` if they would
erase a number, unit, negation, or qualifier. A fix aimed at the wrong structure returns
`wrong_kind_for_target` and a `next_tool_call` with a suitable kind/path.
A finding on a pattern or `compose` cell is at the value behind it
(`…/pattern/values/2/small`); `repair_slide` takes it as `cell_path`. A split must leave **both** halves valid under
that pattern's minimum counts.

`validate_input`, `generate -dry-run`, `generate_presentation` and CLI
`generate` give one verdict: what one refuses all refuse, with the same code
at the same path, before a file is written. `INVALID_SLIDE` names the field (`/slides/N/overlays/K`,
`…/content/M/link`, `…/source_link`); a chart or diagram that cannot render
is `diagram_render_failed`.

A generation refusal for unreadable or lost text (`TEXT_BELOW_READABLE_MIN`
et al.) names the slide (1-based), its pattern or diagram and the authored
path; `validate_input` with the target template reports the same refusal as
an error first. CLI `generate --partial` skips the refused slide and reports
`CONTENT_DROPPED` with `fix.params.cause` and `refused_path`.

`CONTENT_DROPPED` with
`cause:"placeholder_not_found"` or `"placeholder_occupied"` (two blocks on
one placeholder) is `action:"refuse"`, an error at `/slides/N/content/M`
that `validate_input` reports and generation refuses on;
`fix.params.options` lists the remedies (`split_slide`,
`choose_layout`, `retarget_placeholder`, `merge_blocks`). Charts and
diagrams share the native 12pt (`present`) floor at their placed size: a
crowded diagram shows `diagram.text_overlap` instead of shrinking, and text
still below the floor is a `TEXT_BELOW_READABLE_MIN` refusal
(`simplify_or_enlarge_diagram`): enlarge the cell or cut categories.
`strict_fit` promotes fit and chart findings; trust the returned
severity/action.
