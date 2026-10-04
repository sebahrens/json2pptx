# Findings and repair decisions

Use this guide when a deck tool returns findings. Do **not** load a static
finding-code catalog: call `describe_finding` for an unfamiliar code (summary,
severity, when it blocks, remediation steps, examples, related codes). Deeper
rationale: [docs/FIT_FINDINGS.md](../../docs/FIT_FINDINGS.md).

## DeckSpec findings (`validate_deck_spec`, `render_deck_spec`)

Each finding has `code`, `severity`, `blocking` (true exactly when severity
is `error`), `path` — a JSON Pointer (0-based) into the spec you sent, the
string a patch takes — `missing_path` when the field does not exist yet
(`path` is then its parent) and `slide_number` (1-based; "slide N" in a
message is that number). Ignore `debug` (compiled-deck locators).

- **One entry per cause.** `occurrences` + `paths` stand for several
  findings: fix every pointer; `evidence.measured` / `allowed` and a budget
  in `params` are lists in that order when they differ. `symptoms[]` (codes
  only) are consequences of the entry — fix the entry, not the symptoms.
- **One remedy.** `message` says what to do in the fields of your spec,
  `remediation.primary` is the action — `apply_patch`, `shorten_text`
  (`max_chars` / `max_words`), `reduce_items` (`max_items`, `min_items`),
  `split_slide` (`max_rows`, `row`), `replace_value` (`did_you_mean`,
  `available`, `expected_shape`, `example`), `add_detail_or_merge` (`hint`;
  `SLIDE_UNDERUSED` / `SPARSE_FILL`, render's `recommended_edit`) — and
  `next_tool_call` the patch.
  `patch_verified: true`: the server applied that patch and re-validated —
  the finding is gone and nothing new blocks; send it unchanged. A patch
  without the flag holds an `<instruction>` value: write your own words
  within the budget. An over-full slide's message ends `— verified fix:
  removing /slides/7/options/0/detail … clears this`; a label the layout
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
  gate. `VERTICAL_IMBALANCE` / `HORIZONTAL_IMBALANCE`: 40% or more of the
  content area is empty below / beside the block (PATTERNS.md → Placement).
  `TEXT_WRAPS_NARROW`: a paragraph wraps to 5+ lines of ≤3 words — cut each
  box to `max_words`, or keep at most `max_boxes` boxes on the row; `paths`
  lists the box behind each hit (`/slides/8/steps/0`). `SIBLING_SIZE_MISMATCH`:
  peer headers or card titles render at different sizes — shorten the
  longest or use fewer columns.

## Raw-deck findings

Each finding has stable machine fields `{path, code, severity, action, fix}`;
`fix` has `kind` and `params`. The prose `message` is explanatory,
not a programmatic key. The shared finding envelope has `ok` plus an
ordered `findings[]`. Work top-down: severity descending, then slide index,
then code. A deck-level finding precedes slide 0 at equal severity.

### What to do with a finding

- `refuse`: the engine cannot safely produce/accept the requested result.
  Repair the source, validate again, and do not infer success from an artifact
  that happened to be written.
- `shrink_or_split`: the content needs more room or less text. Preserve facts;
  split the slide or rewrite copy rather than blindly truncating.
- `review` / `info`: a judgment for the author; inspect the rendered slide
  and decide. An advisory is not a failed call, and a clean score does not
  replace final-revision visual inspection. `INPUT_CONTROL_CHARS_REMOVED` (info):
  invisible bidi controls / BOMs were stripped from that string; drop them
  from your source text. `BODY_TOO_LONG` is a `refuse` when one placeholder
  carries over 200 paragraphs (`fix.params.max_paragraphs`): split it.
  `grid_violation` (info): content starts off the template's
  `grid.content_frame` — drop explicit `bounds`, keep one layout family.

`score_deck` classifies a finding as `pattern_choice`, `rendering`, or
`content`. A pattern-choice problem usually calls for a different visual
family. A rendering problem calls for fit, geometry, or contrast repair.
A content problem (e.g. `TITLE_NOT_ACTION`, `TITLE_TOO_LONG`) calls for a
better title, evidence, labels, or copy. `DATA_WITHOUT_SOURCE` (review: a
chart, figures table / matrix or KPI / stat pattern, unsourced) is one:
set `slides[N].source` (`fix.params.field: "source"`), a chart `footnote` or
a deck default (QUALITY.md §5); never invent one.

Storyline gate (`score_deck`; details in `docs/FIT_FINDINGS.md`):
`takeaway_missing` → require_takeaway_on_charts; `TITLE_NOT_ACTION` (over 15
words, stock label, no verb/number, or `topic_label`: a ≤4-word label over a
`takeaway`; `fix.params.reason`) on >25% of slides →
max_topic_title_pct; `NO_EXECUTIVE_SUMMARY` (6+ slides) and
`CLOSING_WITHOUT_NEXT_STEPS` ("Thank you" closer, no ask) → require_storyline.
`SLIDE_TEXT_DENSE` (>6 bullets, >80 words, bullet over 2 lines) costs score.

On raw decks, `propose_repairs` translates findings to candidate directives
and separates executable `directives` from `advisory[]`. The authoritative
executable and advisory vocabularies are
`get_capabilities().vocabularies.repair_fix_kinds` (params in
`repair_fix_kind_params`) and `advisory_fix_kinds`. `repair_slide` applies
one slide's executable directives. A finding's
`fix.kind` is not necessarily executable: if it needs human judgment,
`repair_slide` returns `advisory_fix_kind` with alternatives. Act on its
guidance; do not retry the same advisory kind.

Text-reduction fixes refuse with `semantic_review_required` if they would
erase a number, unit, negation, or qualifier. Prefer authoring a shorter
sentence or splitting the slide. A fix aimed at the wrong structure returns
`wrong_kind_for_target` and a `next_tool_call` with a suitable kind/path.
On a named-pattern slide, the expanded grid is generated output: when a
finding names a pattern cell, edit the corresponding pattern `values`,
then re-expand and validate. A split must leave **both** halves valid under
that pattern's minimum counts.

A generation refusal for unreadable or lost text (`TEXT_BELOW_READABLE_MIN`
et al.) names the slide (1-based), its pattern or diagram and the authored
path; `validate_input` with the target template reports the same refusal as
an error first. CLI `generate --partial` skips the refused slide and reports
`CONTENT_DROPPED` with `fix.params.cause` and `refused_path`.

Strict output validation is separate from fit: `generate_presentation`
defaults to a blocking OPC/OOXML pass. `CONTENT_DROPPED` with
`cause:"placeholder_not_found"` or `"placeholder_occupied"` (two blocks on
one placeholder) is `action:"refuse"`: the content is absent from the
result; `fix.params.options` lists the remedies (`split_slide`,
`choose_layout`, `retarget_placeholder`, `merge_blocks`). Charts and
diagrams share the native 12pt (`present`) floor at their placed size: a
crowded diagram shows `diagram.text_overlap` instead of shrinking, and text
still below the floor is a `TEXT_BELOW_READABLE_MIN` refusal
(`simplify_or_enlarge_diagram`): enlarge the cell or cut categories.
`diagram.region_overflow` names a Venn intersection caption that crosses its
region's outline (shorten `fix.params.label`); `diagram.data_key_ignored`
(review) a `business_model_canvas` data key the canvas never reads (rename
it to `fix.params.did_you_mean`). See [RAW_PATH.md](RAW_PATH.md) for the
raw response protocol. `strict_fit` controls promotion of fit and chart
findings; trust the returned severity/action, not an old promotion table.
