# plan_deck

Plan a presentation deck from a natural-language brief — returns an ordered slide outline with recommended patterns and narrative roles.

**Added in:** 3.1.0

## When to Use

Use `plan_deck` as the **first step** when building a deck from scratch. It converts a brief into a structured outline that:
- Assigns narrative roles (opening, framework, evidence, comparison, emphasis, closing)
- Assigns every slide a canonical `layout`: the opening slide is `"title"` and the closing slide is `"closing"`, both with **no pattern**; content slides use `"blank-title"` plus a pattern
- Recommends patterns for each content slide using taxonomy-aware matching; `comparison` slots use only `comparison-2col` / `before-after`
- Enforces deck-rhythm rules automatically (no 3+ consecutive same-pattern runs, emphasis capped at ceil(n/5) and planned only for the brief's headline number or quote)
- Plans only what the brief supports: a `framework` slot needs a framework named in the brief, and evidence / comparison / emphasis slots that receive no brief fact are dropped (the plan comes back shorter, and `budget.cut` says so) instead of being seeded with placeholder prose
- Follows an outline the brief enumerates: one slide per listed item, in the brief's order (see [Enumerated outlines](#enumerated-outlines))
- Reads instructions about the deck itself (`8 slides`, `use the warm-coral template`, `audience: investors`, `with an agenda`) as `constraints[]`, never as facts
- Counts content slides first and always says how the budget was spent (`budget`, `budget_note`)
- Produces output directly consumable as the `slides` array in `generate_presentation`

Skip `plan_deck` when you already have a detailed slide-by-slide outline or when modifying an existing deck.

## Input Schema

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `brief` | string | Yes | — | Natural-language description of the deck purpose and content |
| `slide_budget` | number | No | the slide count the brief states, else 10 | Target number of slides (clamped to 3–30). `"9-10 slides"` in the brief plans to 10 |
| `audience` | string | No | — | Target audience; board/executive/investor and engineering/technical contexts refine relevant pattern scores without matching audience words as slide-content keywords |
| `must_include` | array of strings | No | — | Pattern names that must appear in the plan |
| `format` | string | No | `"raw"` | `"raw"`: pattern outline with raw `SlideInput` skeletons. `"deckspec"` (recommended for more than four slides): a DeckSpec draft (`deck_spec`, `slots[]`, `unplaced_facts[]`) with narrative slots for `validate_deck_spec` / `render_deck_spec`. Any other value is refused with `INVALID_PARAMETER` |
| `template` | string | No | — | Template name (see `list_templates`). Makes the plan template-aware: each planned slide and alternative carries `template_support` `{status: supported\|risky\|unsupported, reasons[], required_layout}`, and a pattern the template cannot host is replaced by a supported alternative when one exists |

## Output Schema

```json
{
  "slides": [
    {
      "slide_index": 0,
      "narrative_role": "opening",
      "recommended_pattern": "",
      "layout": "title",
      "suggested_pattern": "",
      "skeleton": {
        "layout_id": "title",
        "content": [
          {"placeholder_id": "title", "type": "text", "text_value": "__FILL__"},
          {"placeholder_id": "subtitle", "type": "text", "text_value": "__FILL__"}
        ]
      },
      "content_seed": "Title and context: Pitch our Series B...",
      "rationale": "opening slide: use the template's \"title\" layout with no pattern"
    },
    {
      "slide_index": 3,
      "narrative_role": "evidence",
      "recommended_pattern": "kpi-inline",
      "layout": "blank-title",
      "suggested_pattern": "kpi-inline",
      "suggested_pattern_fallback": "kpi-3up",
      "skeleton": {
        "layout_id": "blank-title",
        "content": [{"placeholder_id": "title", "type": "text", "text_value": "__FILL__"}],
        "pattern": {"name": "kpi-inline", "values": ["__FILL__"]}
      },
      "content_seed": "Key data point or supporting detail",
      "rationale": "variety pick: different visual family",
      "predicted_cell_budgets": [
        {"columns": 3, "rows": 1, "body_max_chars": 60, "header_max_chars": 20}
      ],
      "predicted_findings": [],
      "alternatives": [
        {"pattern_name": "kpi-3up", "score": 0.62, "rationale": "evidence metrics"},
        {"pattern_name": "card-grid", "score": 0.48, "rationale": "taxonomy fallback for evidence"}
      ]
    }
  ],
  "brief": "Pitch our Series B for an AI infra company",
  "slide_budget": 10,
  "unplaced_facts": [],
  "budget": {"requested": 10, "planned": 10, "content": 8, "structural": 2, "structural_slides": ["title", "closing"], "cut": []},
  "budget_note": "Planned 10 of 10 slides: 8 content, 2 structural (title, closing). Nothing was cut.",
  "rhythm_check": {
    "longest_pattern_run": 2,
    "has_emphasis": true,
    "emphasis_count": 2,
    "pattern_variety": 8
  }
}
```

### Slide Fields

| Field | Type | Description |
|-------|------|-------------|
| `slide_index` | int | 0-based position |
| `narrative_role` | string | One of: `"opening"`, `"framework"`, `"evidence"`, `"comparison"`, `"emphasis"`, `"closing"` |
| `recommended_pattern` | string | Pattern name to use (from `list_patterns`). **Empty string** for the opening (title) and closing slides — they use a structural layout, not a pattern, and carry no `alternatives` / predictions. |
| `layout` | string | Canonical `layout_id` for the slide: `"title"` (opening), `"closing"` (closing), `"blank-title"` (every pattern slide). Always equals `skeleton.layout_id`. With `template`, the title/closing slides' `template_support` vets that layout (Title Slide / Closing family) instead of a pattern. |
| `content_seed` | string | Brief hint of what content belongs on this slide. When brief facts were routed here they are prefixed verbatim (joined by `; `, then ` — ` and the role hint), e.g. `"revenue grew +23% year over year — Key data point or supporting detail"`. |
| `facts` | array of strings | Brief facts routed to this slide, verbatim (see [Brief facts](#brief-facts)). Omitted when none; never set on title/closing slides. |
| `rationale` | string | Why this pattern was selected (e.g., "required by must_include", "rhythm break") |
| `suggested_pattern` | string | First-choice pattern for this slot. Currently identical to `recommended_pattern`; kept as a separate field so the `(suggested_pattern, suggested_pattern_fallback, skeleton)` triplet reads as a single agent-facing contract. |
| `suggested_pattern_fallback` | string | Second-choice pattern when the suggested pattern's content shape does not fit. Drawn from `alternatives[0]` when available, omitted otherwise. |
| `skeleton` | object | Partial `SlideInput` JSON object with `__FILL__` tokens for free-form copy. Title/closing slides get a layout-only skeleton (`layout_id` + `title` and `subtitle` entries, no `pattern`). Pattern slides include `layout_id`, a title entry, and a `pattern` envelope (`name` + `values`). Schema-constrained fields retain valid defaults (for example, an enum choice, bundled icon name, or numeric score) and are listed in a draft-only `speaker_notes` `__CHOOSE__` reminder; review those choices and remove the reminder before publishing. Numeric and boolean structural defaults are preserved. Omitted when the recommended pattern has no `Exemplar` implementation. Leftover `__FILL__` tokens, including the reminder's draft marker, are reported as `unresolved_placeholder` warnings; pass `placeholder_policy: "strict"` to make them blocking. |
| `predicted_cell_budgets` | array | Per-configuration character budgets (body/header) the renderer would impose on this pattern. Empty for non-grid patterns (e.g. `pull-quote`, `stat-hero`). |
| `predicted_findings` | array | Up to 3 forecast fit-findings the renderer would emit when this pattern is filled with exemplar (role-default) content. Each entry has `code`, `path`, `message`, `action`, and (when applicable) `next_tool_call`. Empty when the pattern declares no exemplar or expansion fails. |
| `alternatives` | array | Up to 2 next-best ranked patterns for this slot. Each entry has `pattern_name`, `score`, and `rationale`. Includes a taxonomy fallback when the rule-based recommender returns too few. |

> **Note on predictions:** `predicted_cell_budgets` and `predicted_findings` are derived without rendering or template/theme context. Findings that require a parsed template (placeholder overflow, footer collision, contrast prediction) are skipped here — only shape-grid-resident detectors fire (text overflow, sparse layout, pattern occupancy, table preflight).

### Brief facts

The planner lifts facts out of the brief so its numbers reach the slides instead of generic seeds. Deck instructions are taken out first (see [Constraints](#constraints)). The brief is then split into clauses (sentence ends, `;`, newlines, `, `, `: `, spaced dashes — decimals like `1.5M` and separators like `1,200` survive). Lists stay whole:

- a comma or colon inside brackets does not split (`(build, partner with Globex, acquire Initech)`, `(Q2: 63.0%)`);
- a short lower-case list item rejoins the clause before it (`against cost, time-to-market, and risk`);
- a bare number rejoins the clause before it, so `revenue last 5 quarters 41.0, 42.3, 44.1, 45.9, 48.2` is one fact — a series, routed to one chart;
- a short label keeps the clause it labels (`recommendation: renegotiate now`, `source: management accounts Q3 FY26`); a label that only announces content (`Facts:`, `Key data:`) is dropped;
- a list header (`three options to fix margin: a, b, c`, `next steps: …`, `three phases: …`) is a fact, and its items form one group that is routed together.

**Every clause after the topic is a fact**: it is on a slide's `facts` or in `unplaced_facts`. The first clause is the deck topic and only counts as a fact when it holds a quantity. (Before 2026-10-03 a clause qualified only with a quantity, a name, a quote, an option or a recommendation, so `main risk is permitting delay` reached neither.)

Each fact is classified: a **metric** (percent, currency, magnitude such as `12.5m`, points, or a count that is neither a to-do nor a date), a **series** (a metric that changes over time, or three or more listed values), a **to-do / ask** (`hiring 12 AEs`, `approve a term sheet by 30 November`), a **dated milestone** (`SOC 2 Type II in December`), an **option** (`build vs buy`; `vs plan` is a benchmark, not an option), a **risk**, a **recommendation**, a **source** or a **quote**.

Facts are assigned to pattern slides (never title/closing): the options listed under one header go together to the first comparison slide and listed phases / milestones to the first roadmap slide (both whatever the slide's capacity), a listed series to `chart-insights-split`; then metrics go to numeric patterns (`kpi-*`, `stat-hero`, `kpi-inline`, `chart-insights-split`, `horizontal-bar-with-callouts`, `waterfall-bridge`, `driver-tree`), quotes to `pull-quote`, options to comparison slots; everything else is spread round-robin over evidence/comparison slides, then framework/emphasis slides — but a KPI / stat slide only ever takes a metric, a `pull-quote` a quote and a comparison slot an option. Capacity is one fact per KPI card (`kpi-3up` → 3), one for `stat-hero` / `pull-quote`, two otherwise. The source the brief names is appended to the opening slide's `content_seed`. Leftovers go to the top-level `unplaced_facts` array (always present, `[]` when empty) — add a slide or fold them in by hand.

### Constraints

Instructions about the deck itself are lifted out of the brief before any fact is read and returned in `constraints[]` (omitted when the brief states none). Each entry is `{kind, text, value}`; `text` is the brief's own wording.

| `kind` | Matches | `value` | What the planner does with it |
|---|---|---|---|
| `slide_count` | `8 slides`, `9-10 slides`, `7-slide`, `in ten slides`, `max 12 pages` (the first count in the brief) | `"8"`, `"9-10"` | Sets the budget when `slide_budget` is not passed (the upper end of a range) |
| `template` | `use the warm-coral template`, `template: modern` | the name | Reported only — pass it as the `template` argument |
| `audience` | `audience: investors`, `to be presented to the board`, `aimed at …` | who | Fills `meta.audience` (deckspec) and steers pattern scores when `audience` is not passed. `for the board` inside the topic is left in the topic |
| `duration` | `a 20-minute presentation` | minutes | Reported only |
| `structure` | `with an agenda`, `include section dividers`, `no agenda` | `agenda`, `dividers`, `no agenda`, `no dividers` (comma-separated) | Permits or rules out an agenda and dividers (see [Budget](#budget)) |

### Enumerated outlines

A brief that lists its slides gets one slide per listed item, in the brief's order, after the cover — in both formats. A list of three or more items is an outline when

- its label says so (`Slides: …`, `Outline: …`, `covering: …`, lines that start `Slide 1: …`), or
- it follows the topic's colon and its length matches the stated slide count (the count, or the count less a cover and a close) and at most half its items are figures, or
- it follows the topic's colon and every item is a short topic rather than a figure (`Investor pitch: problem, solution, market, traction, team, ask`); numbered or bulleted lines count the same way.

A list the brief labels as facts (`Facts: …`) or that a list header counts (`three options: a, b, c`) is content, not an outline. Each item picks its slide from its own words:

| Item names | deckspec slot → kind | raw pattern |
|---|---|---|
| summary, overview, headline(s), key messages | `answer` → `executive_summary` | `exec-summary` |
| KPIs, metrics, traction, results, financials | `context` → `kpi_snapshot` | `kpi-3up` |
| chart, trend, forecast, a listed series, or a metric that changes over time | `evidence` → `chart_insight` | `chart-insights-split` |
| any other single figure | `context` → `stat` | `stat-hero` |
| milestones, timeline / roadmap, phases | `roadmap` → `timeline` / `roadmap` | `timeline-horizontal` / `phase-roadmap` |
| options, alternatives, scenarios | `options` → `option_matrix` | `table-highlight` |
| risk(s), mitigations | `risks` → `table` | `labeled-rows` |
| team, founders, leadership | `topic` → `team` | `team-bios` |
| process, steps, how it works | `plan` → `process` | `numbered-step-strip` |
| competition, versus, comparison | `topic` → `comparison` | `comparison-2col` |
| ask, funding, decision, recommendation | `ask` → `decision` (the last item: `closing` → `next_steps`) | `next-steps` |
| next steps, action plan | `closing` → `next_steps` | `next-steps` |
| agenda, table of contents | `agenda` → `agenda` | `agenda` |
| problem, pain, challenge | `problem` → `pillars` | `card-grid` |
| anything else (solution, business model, …) | `topic` → `pillars` | `card-grid` |

The item is `facts[0]` of its slide; facts from the rest of the brief are added to the items built to show them, the remainder to `unplaced_facts`. A `next_steps` close is added when the outline does not end on one and the budget has room. The outline is kept whole even when it is longer than the budget: `budget.planned` then exceeds `budget.requested` and `budget_note` says by how much (a 7-item outline at `slide_budget: 7` plans 8 slides — the cover plus the seven).

### Budget

The budget counts content slides first. `budget` and `budget_note` are always present:

| Field | Description |
|---|---|
| `budget.requested` | The budget planned to (same as `slide_budget`) |
| `budget.planned` | Rendered length of the plan, generated agenda and dividers included |
| `budget.content` / `budget.structural` | Slides that carry the argument / slides that only frame it |
| `budget.structural_slides` | Each structural slide: `cover` (deckspec) or `title` and `closing` (raw — the pattern-less closing page), `agenda`, `divider: <section>`, `appendix divider` |
| `budget.cut` | `[{what, reason}]` — storyline slots that did not fit (`"cause (pillars)"`), raw slots the brief gave nothing to show, a close the outline left no room for. `[]` when nothing was cut |
| `budget_note` | The same in one line — `Planned 8 of 8 slides: 7 content, 1 structural (cover). Nothing was cut.` — followed by why the plan is shorter or longer than the budget and how many facts are unplaced |

An agenda and section dividers are planned only at 12 or more slides, or at any budget when the brief asks for them (`with an agenda`, `include section dividers`); `no agenda` / `no dividers` rules them out at any budget. Without the ask they are added only from room the content left over — they never displace a content slide. With the ask, the lowest-priority body slides give way and are listed in `budget.cut`. In the raw plan an `agenda` pattern is likewise not recommended under 12 slides unless asked or required by `must_include`.

### format: "deckspec"

`deck_spec` is drafted from the brief's signals rather than a fixed arc:

| Brief signal | Slot → kind |
|---|---|
| always | `cover` → `title`, `answer` → `executive_summary`, `closing` → `next_steps` |
| 2+ metrics / 1 metric / a named problem without numbers | `problem` (or `context` when no problem is named) → `kpi_snapshot` / `stat` / `comparison` |
| shipped / launched / released work | `highlights` → `pillars` |
| a metric that changes over time (YoY, "from X to Y", 3+ values) | `evidence` → `chart_insight` (one per such fact, at most two) |
| a named problem (not for a customer-facing deck) | `cause` → `pillars` (`table` when pillars is taken) |
| options weighed against criteria | `options` → `option_matrix` |
| a risk the brief names (not as a criterion of an option evaluation) | `risks` → `table` |
| a plan or priorities with to-dos | `plan` → `pillars` / `process` (steps; `next steps` alone is the close, not a process) / `table` |
| 2+ dated milestones / phases | `roadmap` → `timeline` / `roadmap` |
| a request for a decision (not for a customer-facing deck) | `ask` → `decision` |

Facts route by class: a listed series, then other change-over-time metrics, to `evidence`; the options listed under one header all to `options`, listed phases to `roadmap`, listed risks to `risks` (a listed group is never split, whatever the slot's capacity); other metrics to `problem`/`context` (an ask that names an amount goes to the ask); the recommendation to `ask`; risks to `risks`; `next steps …` to `closing`; asks to `ask`/`closing`; dated milestones to `roadmap`; options to `options`; to-dos to `plan`/`closing`; the rest to `answer`, `highlights`, `cause`. The `cover` slot carries the topic and the source the brief names, which is also set as `meta.source`. A brief that enumerates its slides skips this table (see [Enumerated outlines](#enumerated-outlines)).

A budget of 12+ slides — or any budget when the brief asks for an agenda or dividers — is drafted as `structure` (`auto_agenda: true`, 2–4 sections; the agenda and one divider per section count toward the budget) when the chapters fit in the room the content left (see [Budget](#budget)); `slots[].path` says where each slide lives and `slots[].section` names its chapter. `meta.chrome` sets `page_numbers.enabled: true` (and `tracker: true` for a chaptered draft); `meta.date` is `__FILL__`, which `validate_deck_spec` reports as `SEMANTIC_WEAK_CONTENT` at `meta.date` until it is replaced.

**Appendix.** A budget of 12+ slides — or a brief that asks for backup material outright — ends with back matter when the brief has backup material — it asks for backup / detail (appendix, backup, deep dive, breakdown, detailed figures), names a methodology / assumptions / data sources, has facts the body had no room for, or carries 3+ figures — and the budget still has room after the body (the appendix divider costs one slide; back matter never displaces a body slide and is never drafted as padding). Slots: `backup` → `table` (the detailed figures, carrying any unplaced facts) and `methodology` → `table` (when the brief names a methodology or assumptions). In a chaptered draft the appendix is the last `structure.sections[]` entry, `{"title": "Appendix", "appendix": true, ...}`, and the `next_steps` close moves from `structure.closing` to the end of the last body chapter so the argument ends before the backup pages. A flat draft gets `{"kind": "section", "title": "Appendix", "appendix": true}` after the close (slot `appendix`) followed by the backup slides. Back-matter slots carry `appendix: true` and `section: "Appendix"`. The appendix divider is unnumbered and left out of the auto agenda, and its slides are exempt from the run checks (`SEMANTIC_RHYTHM_MONOTONY` / `_DENSITY`, `DECK_MONOTONY`, `analyze_deck_rhythm` `break_run` / `bullets_heavy`).

### Top-level fields

| Field | Type | Description |
|-------|------|-------------|
| `unplaced_facts` | array of strings | Brief clauses no slide had room for. Always present. Every clause after the topic is on a slide's `facts` or here. |
| `slide_budget` | int | The budget planned to: the `slide_budget` argument, else the slide count the brief states, else 10. |
| `constraints` | array | Deck instructions the brief states (see [Constraints](#constraints)). Omitted when none. |
| `budget` | object | How the budget was spent (see [Budget](#budget)). Always present. |
| `budget_note` | string | The budget account in one line. Always present. |

### Rhythm Check

| Field | Type | Description |
|-------|------|-------------|
| `longest_pattern_run` | int | Longest consecutive run of the same pattern (target: ≤2) |
| `has_emphasis` | bool | Whether at least one emphasis slide (stat-hero, pull-quote, or kpi-inline) exists |
| `emphasis_count` | int | Number of emphasis slides — never more than ceil(slide_budget/5) |
| `pattern_variety` | int | Count of unique patterns used (title/closing slides have none and are not counted) |

## Narrative Roles

The planner distributes slides across a standard narrative arc:

| Role | Fraction | Purpose |
|------|----------|---------|
| `opening` | ~10% | Title, context-setting |
| `framework` | ~15% | The framework the brief names — planned only when the brief names a framework, methodology, model or structure; otherwise the share goes to evidence |
| `evidence` | ~40% | Supporting data, details, case studies |
| `comparison` | ~15% | Compare alternatives or trade-offs |
| `emphasis` | ~10% | The brief's headline number (`stat-hero`) or quote (`pull-quote`) — planned only when the brief carries one |
| `closing` | ~10% | Summary and next steps |

## Rhythm Rules

The planner automatically enforces:

1. **Structural bookends** — slide 0 is layout `title` and the last slide is layout `closing`, both with no pattern (a pattern there fights the layout's own title treatment)
2. **No 3+ consecutive runs** — if detected, the middle slide is swapped to a pattern from a different visual family
3. **No emphasis quota** — no slide is rewritten to `stat-hero` / `pull-quote` to vary the rhythm; an emphasis slide exists only for the brief's own number or quote. A straight sequence uses `numbered-step-strip`, not `process-flow` (kept for briefs with decision points)
4. **Emphasis cap** — at most ceil(n/5) emphasis slides (`stat-hero`, `pull-quote`, `kpi-inline`) per n-slide deck; extras are demoted to a non-emphasis pattern (must_include placements are kept)
5. **Comparison family** — `comparison` slots use only `comparison-2col` or `before-after` (before-after first when the brief mentions before/after or current/future state; the two alternate across multiple comparison slots)
6. **Variety awareness** — `recommend_pattern` is called with `prefer_variety=true` to penalize recently-used patterns

## Example

```json
// Request
{
  "brief": "Series B pitch for an AI infrastructure company with $50M ARR",
  "slide_budget": 8,
  "audience": "investors",
  "must_include": ["kpi-3up", "roadmap-phased"]
}
```

```json
// Response
{
  "slides": [
    {"slide_index": 0, "narrative_role": "opening",    "recommended_pattern": "",               "layout": "title",       "content_seed": "Title and context: Series B pitch for an AI infrastructure...", "rationale": "opening slide: use the template's \"title\" layout with no pattern"},
    {"slide_index": 1, "narrative_role": "evidence",   "recommended_pattern": "kpi-3up",        "content_seed": "Series B pitch for an AI infrastructure company with $50M ARR — Prove one claim with the facts routed here", "facts": ["Series B pitch for an AI infrastructure company with $50M ARR"], "rationale": "required by must_include"},
    {"slide_index": 2, "narrative_role": "evidence",   "recommended_pattern": "roadmap-phased", "content_seed": "Prove one claim with the facts routed here", "rationale": "required by must_include"},
    {"slide_index": 3, "narrative_role": "closing",    "recommended_pattern": "",               "layout": "closing",     "content_seed": "Summary, next steps, or call to action", "rationale": "closing slide: use the template's \"closing\" layout with no pattern"}
  ],
  "brief": "Series B pitch for an AI infrastructure company with $50M ARR",
  "slide_budget": 8,
  "unplaced_facts": [],
  "budget": {"requested": 8, "planned": 4, "content": 2, "structural": 2, "structural_slides": ["title", "closing"], "cut": [{"what": "4 evidence / comparison / emphasis slot(s)", "reason": "no brief fact, option or quote supports them"}]},
  "budget_note": "Planned 4 of 8 slides: 2 content, 2 structural (title, closing). Cut: 4 evidence / comparison / emphasis slot(s) (no brief fact, option or quote supports them). A slot with nothing to show would only carry placeholder prose — add the numbers, names, options or quotes the cut slides would prove.",
  "rhythm_check": {
    "longest_pattern_run": 1,
    "has_emphasis": false,
    "emphasis_count": 0,
    "pattern_variety": 2
  }
}
```

(Pattern slides 1–2 also carry `"layout": "blank-title"`; omitted above for width. A one-fact brief plans a short deck: give the brief the numbers, names, options or quotes the extra slides would prove.)

## Composing with Other Tools

| Workflow Step | Tool | Purpose |
|---|---|---|
| 1. Plan | **`plan_deck`** | Get a rhythm-aware outline from a brief |
| 2. Validate rhythm | `analyze_deck_rhythm` | Confirm the plan has good visual variety |
| 3. Fill content | Use `show_pattern` + `expand_pattern` | Get schema for each pattern and populate values |
| 4. Generate | `generate_presentation` | Render the final PPTX |
| 5. Repair | `repair_slide` | Fix overflow or color issues |

### From plan_deck output to generate_presentation input

Prefer the **per-slide `skeleton`** for assembly:

1. For each slide entry that has a `skeleton`:
   - Copy the `skeleton` object verbatim into `presentation.slides[]`.
   - Replace every `"__FILL__"` string with real content (the slide title, free-form pattern values, etc.). Review schema-constrained defaults named in the `__CHOOSE__` speaker note, then remove that draft note. Any token left unreplaced is reported as an `unresolved_placeholder` warning by `validate_input`/`generate_presentation`; pass `placeholder_policy: "strict"` on the final generate to make leftovers blocking.
   - The `skeleton` already pins `layout_id`, `pattern.name`, and the `pattern.values` shape, so you only fill in the leaves — no re-derivation of slide structure from the prose `content_seed`.
2. For slides whose recommended pattern has no `Exemplar` (so `skeleton` is omitted), fall back to the longer path:
   - Call `show_pattern(name)` to get the values schema
   - Populate `values` based on your content and the `content_seed` hint
   - Include as `{"pattern": {"name": "...", "values": {...}}, "content": [{"placeholder_id": "title", "type": "text", "text_value": "..."}]}`
3. If `suggested_pattern` does not fit your content (e.g., you have 4 columns of comparison data and the pattern only supports 2), switch to `suggested_pattern_fallback` and call `show_pattern` on that one — alternatives have been pre-ranked against the same brief.

## Error Codes

| Code | Cause |
|------|-------|
| `MISSING_PARAMETER` | `brief` not provided |
| `INVALID_PARAMETER` | A `must_include` pattern name doesn't exist |
