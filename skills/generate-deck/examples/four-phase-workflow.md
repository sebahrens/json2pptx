# Worked Example: Plan → Vary → Render → Repair

The default DeckSpec path for an 8-slide Series B strategy deck. Raw
`PresentationInput` steps are in [../RAW_PATH.md](../RAW_PATH.md).

User prompt: "Create a strategy deck for our Series B fundraise. AI
infrastructure company, $50M ARR, 140% NRR, 3,200 customers, raising $75M."

## Phase 1: PLAN — ghost deck first

Write the titles alone and read them as the whole argument
([../QUALITY.md](../QUALITY.md)). Every content title is a sentence that
states its conclusion and carries its number:

```
1. title             Series B: scaling AI infrastructure
2. executive_summary $50M ARR and 140% NRR show a platform ready to scale 3x
3. kpi_snapshot      3,200 customers expand 40% a year without a field sales team
4. chart_insight     ARR grew from $12M to $50M in two years on self-serve adoption
5. comparison        Managed inference costs customers 45% less than DIY clusters
6. roadmap           Three phases take us from $50M to $150M ARR by 2028
7. decision          $75M funds GPU capacity and enterprise sales, not new products
8. closing           Close the round by June to secure Q3 GPU allocation
```

Slide 5 was first drafted as "Competitive landscape" — a topic, not a
message — and rewritten once the claim was known. The kinds come from the
message → visual table: growth over time is a chart, a cost claim is a
comparison, the plan is a roadmap, the ask is a decision.

`plan_deck` with `format: "deckspec"` drafts the same structure from a brief
(kinds plus narrative slots); treat its titles as placeholders to rewrite.

## Phase 2: VARY — check the sequence before rendering

`explain_deck_spec` (full tool profile) previews the resolved visual for
every slide. The sequence above never repeats a visual family, puts a light
KPI slide between the summary and the chart, and keeps `meta.accent_strategy`
at the default `primary` — rotation is only worth it when the template's
safe-accent set is large (see RULES.md → Accent monotony).

## Phase 3: RENDER

```yaml
meta:
  title: "Series B: scaling AI infrastructure"
  template: midnight-blue
slides:
  - kind: title
    title: "Series B: scaling AI infrastructure"
    subtitle: Confidential — board and investors
  - kind: kpi_snapshot
    title: 3,200 customers expand 40% a year without a field sales team
    takeaway: Expansion, not new logos, is the growth engine.
    source: Company data, FY26 Q1
    kpis:
      - { value: "$50M", label: "Annual recurring revenue" }
      - { value: "140%", label: "Net revenue retention" }
      - { value: "3,200", label: "Paying customers" }
  # … the remaining slides follow the ghost deck
```

`validate_deck_spec` → `render_deck_spec` → `render_deck_thumbnails` (all
slides). Keep the returned `deck_id`.

## Phase 4: REPAIR — review every slide, then patch

Apply the per-slide rubric in [../WORKFLOW.md](../WORKFLOW.md) to each image.
Here slide 4's chart had no unit on its axis and slide 6's title ran to three
lines. Both are spec edits, sent as one patch:

```json
render_deck_spec({
  "deck_id": "<deck_id>",
  "patch": [
    {"op": "replace", "path": "/slides/3/chart/title", "value": "ARR ($M)"},
    {"op": "replace", "path": "/slides/5/title", "value": "Three phases take ARR from $50M to $150M by 2028"}
  ]
})
```

Re-render `changed_slides` with `render_deck_thumbnails(slide_indices)`, then
make one full pass over the final revision and record it with
`submit_visual_review`. Stop after three repair rounds and report what is
still open instead of looping.
