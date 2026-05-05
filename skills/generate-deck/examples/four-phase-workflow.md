# Worked Example: 4-Phase Deck Generation

This shows the full PLAN → VARY → RENDER → REPAIR flow for a 10-slide strategy deck.

## Phase 1: PLAN

User prompt: "Create a strategy deck for our Series B fundraise. AI infrastructure company, $50M ARR."

**Step 1 — Pick template and accent strategy:**

```
list_templates() → choose "midnight-blue" (professional, dark backgrounds)
accent_strategy: "rotate" (10 slides, want visual variety)
```

**Step 2 — Build outline using `recommend_pattern` for each intent:**

```
recommend_pattern("title slide opening")        → title (layout)
recommend_pattern("3 key metrics KPIs")          → kpi-3up
recommend_pattern("market opportunity size")     → stat-hero
recommend_pattern("product architecture layers") → arch-stack
recommend_pattern("competitive comparison")      → comparison-2col
recommend_pattern("growth trajectory chart")     → content (chart slide)
recommend_pattern("team leadership grid")        → card-grid
recommend_pattern("financial projections table") → content (table slide)
recommend_pattern("implementation roadmap")      → roadmap-phased
recommend_pattern("call to action closing")      → closing (layout)
```

**Step 3 — Check outline against rhythm rules:**

```
Outline:
  1. title         — "Series B: Scaling AI Infrastructure"
  2. kpi-3up       — "$50M ARR | 140% NRR | 3,200 Customers"
  3. stat-hero     — "The $180B Opportunity"           ← narrative break
  4. arch-stack    — "Platform Architecture"
  5. comparison-2col — "Why Us vs. Incumbents"
  6. content/chart — "Revenue Growth 2023-2026"
  7. card-grid     — "Leadership Team"
  8. content/table — "Financial Projections"
  9. roadmap-phased — "18-Month Execution Plan"
 10. closing       — "The Ask: $75M Series B"
```

No pattern repeats more than once. Density alternates (high→low→high). Narrative break at slide 3.

## Phase 2: VARY

Build the JSON, then check rhythm:

```json
analyze_deck_rhythm({
  "presentation": {
    "template": "midnight-blue",
    "slides": [...]
  }
})
```

Response:
```json
{
  "per_slide": [
    {"slide_index": 0, "pattern": "title",          "density_class": "low",  "accent_role": "none"},
    {"slide_index": 1, "pattern": "kpi-3up",        "density_class": "med",  "accent_role": "accent1"},
    {"slide_index": 2, "pattern": "stat-hero",      "density_class": "low",  "accent_role": "accent2"},
    {"slide_index": 3, "pattern": "arch-stack",     "density_class": "high", "accent_role": "accent1"},
    {"slide_index": 4, "pattern": "comparison-2col","density_class": "med",  "accent_role": "accent3"},
    {"slide_index": 5, "pattern": "content",        "density_class": "med",  "accent_role": "none"},
    {"slide_index": 6, "pattern": "card-grid",      "density_class": "med",  "accent_role": "accent1"},
    {"slide_index": 7, "pattern": "content",        "density_class": "high", "accent_role": "none"},
    {"slide_index": 8, "pattern": "roadmap-phased", "density_class": "high", "accent_role": "accent4"},
    {"slide_index": 9, "pattern": "closing",        "density_class": "low",  "accent_role": "none"}
  ],
  "aggregates": {
    "pattern_runs": [],
    "longest_run": 1,
    "repetition_index": 0.2,
    "accent_balance": {"accent1": 0.43, "accent2": 0.14, "accent3": 0.14, "accent4": 0.14},
    "density_cv": 0.33
  },
  "recommendations": [],
  "composition_score": 90
}
```

All checks pass: `longest_run=1`, `repetition_index=0.2`, `density_cv=0.33`, `composition_score=90`. Proceed to render.

## Phase 3: RENDER

```json
generate_presentation({
  "presentation": {
    "template": "midnight-blue",
    "accent_strategy": "rotate",
    "slides": [
      {
        "slide_type": "title",
        "layout_id": "title",
        "content": [
          {"placeholder_id": "title", "type": "text", "value": "Series B: Scaling AI Infrastructure"},
          {"placeholder_id": "subtitle", "type": "text", "value": "Confidential — May 2026"}
        ]
      },
      {
        "layout_id": "blank",
        "pattern": {
          "name": "kpi-3up",
          "values": {
            "title": "Traction at Scale",
            "kpis": [
              {"label": "Annual Recurring Revenue", "value": "$50M", "delta": "+140% YoY"},
              {"label": "Net Revenue Retention", "value": "140%", "delta": "Top decile"},
              {"label": "Enterprise Customers", "value": "3,200", "delta": "+85% YoY"}
            ]
          }
        }
      }
    ]
  },
  "strict_fit": "warn",
  "fit_report": true
})
```

(Remaining 8 slides follow the same pattern — each using the planned layout/pattern.)

## Phase 4: REPAIR

Fit report shows one finding:

```json
{
  "path": "slides[7].content.body",
  "code": "fit_overflow",
  "severity": "error",
  "message": "table needs 9 rows but cell height allows 6",
  "fix": {"kind": "split_at_row", "params": {"row": 6}}
}
```

Fix with `repair_slide`:

```json
repair_slide({
  "presentation": {...},
  "slide_index": 7,
  "fixes": [{"kind": "split_at_row", "params": {"row": 6}}]
})
```

This splits the financial projections table across two slides. Re-run `render_deck_thumbnails` and visually verify all slides pass the inspection checklist.
