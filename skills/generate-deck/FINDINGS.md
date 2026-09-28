# Findings and repair decisions

Use this guide when a deck tool returns findings. Do **not** load a static
finding-code catalog: call `describe_finding` for an unfamiliar code. Its
runtime registry supplies the current summary, severity, emission conditions,
remediation steps, examples, and related codes. The diagnostic taxonomy and
code-level coverage are tested in the server. Contributors who need emission
paths and deeper rationale can read
[docs/FIT_FINDINGS.md](../../docs/FIT_FINDINGS.md).

Each finding has stable machine fields `{path, code, severity, action, fix}`;
`fix` has `kind` and `params`. The prose `message` is explanatory,
not a programmatic key. The shared finding envelope has `ok` plus an
ordered `findings[]`. Work top-down: severity descending, then slide index,
then code. A deck-level finding precedes slide 0 at equal severity.

## What to do with a finding

- `refuse`: the engine cannot safely produce/accept the requested result.
  Repair the source, validate again, and do not infer success from an artifact
  that happened to be written.
- `shrink_or_split`: the content needs more room or less text. Preserve facts;
  split the slide or rewrite copy rather than blindly truncating.
- `review` / `info`: the engine points out a judgment for the author.
  Inspect the rendered slide and decide whether to change it. An advisory is
  not a failed tool call, and a clean deterministic score does not replace
  final-revision visual inspection.

`score_deck` classifies a finding as `pattern_choice`, `rendering`, or
`content`. A pattern-choice problem usually calls for a different visual
family. A rendering problem calls for fit, geometry, or contrast repair.
A content problem (e.g. `TITLE_NOT_ACTION`, `TITLE_TOO_LONG`) calls for a
better title, evidence, labels, or copy. `DATA_WITHOUT_SOURCE` (info) is one:
a chart, table of figures, or chart / KPI / stat pattern with no source — set
the slide's `source` (DeckSpec `slides[N].source`, `fix.params.field:
"source"`); never invent one, write "Illustrative" for estimates.
The classification is more useful than a generic increase-the-score loop.

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

Strict output validation is separate from fit: `generate_presentation`
defaults to a blocking OPC/OOXML pass. `CONTENT_DROPPED` with
`cause:"placeholder_not_found"` or `"placeholder_occupied"` (two blocks on
one placeholder) is `action:"refuse"` and blocking in strict mode because the
content is absent from the result; `fix.params.options` lists the remedies
(`split_slide`, `choose_layout`, `retarget_placeholder`, `merge_blocks`).
Charts and diagrams share the native 12pt (`present`) floor at their placed
size: the renderer draws labels at the floor, so a crowded diagram shows
`diagram.text_overlap` instead of shrinking, and any text still below it is a
`TEXT_BELOW_READABLE_MIN` refusal (`simplify_or_enlarge_diagram`): enlarge the
cell or cut categories. See [RAW_PATH.md](RAW_PATH.md) for the
raw response protocol. `strict_fit` controls promotion of fit and chart
findings; consult the returned severity/action and `describe_finding`
instead of copying an old promotion table.
