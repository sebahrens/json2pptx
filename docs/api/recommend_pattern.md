# recommend_pattern

Recommend named patterns for a content intent. Returns ranked candidates with scores, rationales, confidence bands, and expansion previews.

**Added in:** 2.0.0

## When to Use

Use `recommend_pattern` when you know **what a slide should show** but not **which pattern to use**. It maps natural-language intents to the best-matching patterns from the catalog.

Use `plan_deck` instead when you need a full deck outline — it calls `recommend_pattern` internally with variety awareness.

## Input Schema

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `intent` | string | Yes | — | Natural-language description of what the slide should show |
| `content_hints` | object | No | — | Structured hints to refine ranking |
| `recent_patterns` | array of strings | No | — | Pattern names used on preceding slides (in order) |
| `prefer_variety` | boolean | No | false | Penalize patterns in `recent_patterns` and inject diversity bonus candidate |
| `slide_index` | number | No | — | 0-based index of the slide being built (context for diversity scoring) |

### content_hints Object

| Field | Type | Description |
|-------|------|-------------|
| `item_count` | int | Number of items to display (e.g., 3 KPIs, 5 cards) |
| `has_chart` | bool | Whether the slide includes a chart |
| `has_metrics` | bool | Whether the content is metric-heavy |
| `columns` | int | Desired number of columns |

## Output Schema

```json
{
  "candidates": [
    {
      "pattern_name": "kpi-3up",
      "score": 0.92,
      "rationale": "Matches 'show 3 KPIs' — designed for exactly 3 metric displays",
      "confidence_band": "high",
      "diversity_bonus": false,
      "expansion_preview": { "...shape_grid..." }
    }
  ],
  "query_understood_as": "show 3 key performance indicators",
  "near_misses": [
    {
      "pattern_name": "card-grid",
      "score": 0.61,
      "would_tip_if": "item_count were 4+ or content were non-metric"
    }
  ],
  "disambiguating_questions": [
    "Are the metrics time-series (→ chart) or point-in-time (→ kpi-Nup)?"
  ]
}
```

### Candidate Fields

| Field | Type | Description |
|-------|------|-------------|
| `pattern_name` | string | Pattern name (use with `show_pattern` or `expand_pattern`) |
| `score` | float | Match score (0.0–1.0) |
| `rationale` | string | Why this pattern was recommended |
| `confidence_band` | string | `"high"`, `"medium"`, or `"low"` — indicates match confidence |
| `diversity_bonus` | bool | True if this candidate was injected for variety (when `prefer_variety=true`) |
| `expansion_preview` | object or null | Shape grid expanded with exemplar values (preview of the visual layout) |

### Near Misses

Patterns that almost matched but fell below the threshold. Each includes `would_tip_if` — the condition that would make it a top candidate. Useful for understanding the decision boundary.

### Disambiguating Questions

When the intent is ambiguous (multiple patterns could work), questions are returned to help the caller refine. Example: "Are these metrics time-series or point-in-time?"

## Variety Mode

When `prefer_variety=true` and `recent_patterns` is provided:

1. **Recency penalty**: Patterns appearing in `recent_patterns` get a score reduction proportional to how recently they were used
2. **Diversity bonus**: A candidate from a different visual family may be injected with `diversity_bonus: true`
3. **Slide index context**: `slide_index` helps calibrate how aggressive the variety penalty should be (early slides tolerate less repetition)

This prevents decks from becoming monotonous when building slides sequentially.

## Examples

### Basic intent matching

```json
// Request
{
  "intent": "compare two product options side by side"
}

// Response
{
  "candidates": [
    {"pattern_name": "comparison-2col", "score": 0.95, "rationale": "Direct match: 2-column comparison layout", "confidence_band": "high"},
    {"pattern_name": "before-after",    "score": 0.72, "rationale": "Side-by-side before/after layout",        "confidence_band": "medium"},
    {"pattern_name": "matrix-2x2",      "score": 0.58, "rationale": "2x2 matrix could show option trade-offs", "confidence_band": "low"}
  ],
  "query_understood_as": "compare two options side by side",
  "near_misses": [],
  "disambiguating_questions": []
}
```

### With variety awareness

```json
// Request
{
  "intent": "show key metrics",
  "content_hints": {"item_count": 3, "has_metrics": true},
  "recent_patterns": ["kpi-3up", "stat-hero", "kpi-3up"],
  "prefer_variety": true,
  "slide_index": 5
}

// Response — kpi-3up penalized because it appeared twice recently
{
  "candidates": [
    {"pattern_name": "card-grid",  "score": 0.78, "rationale": "Metrics in card format; variety pick", "confidence_band": "medium", "diversity_bonus": true},
    {"pattern_name": "kpi-3up",    "score": 0.65, "rationale": "Natural fit but penalized for recency", "confidence_band": "medium", "diversity_bonus": false},
    {"pattern_name": "icon-row",   "score": 0.52, "rationale": "Metrics with icons",                   "confidence_band": "low",    "diversity_bonus": false}
  ],
  "query_understood_as": "display 3 key metrics",
  "near_misses": [],
  "disambiguating_questions": []
}
```

### No match

```json
// Request
{
  "intent": "show a completely custom freeform layout"
}

// Response
{
  "candidates": [],
  "query_understood_as": "custom freeform layout",
  "suggestion": "No patterns matched this intent. Consider using shape_grid directly to build a custom layout, or try rephrasing with keywords like: kpi, compare, timeline, matrix, bmc, icon, card.",
  "near_misses": [],
  "disambiguating_questions": []
}
```

## Composing with Other Tools

| Workflow | Tool Chain |
|----------|-----------|
| Single slide | `recommend_pattern` → `show_pattern` → `expand_pattern` → include in `generate_presentation` |
| Full deck | `plan_deck` (calls recommend internally) → `analyze_deck_rhythm` → `generate_presentation` |
| Iterative refinement | `recommend_pattern` with `prefer_variety` + `recent_patterns` for each new slide |

### recommend_pattern vs plan_deck

| | `recommend_pattern` | `plan_deck` |
|---|---|---|
| Scope | One slide at a time | Entire deck |
| Rhythm enforcement | Manual (caller passes `recent_patterns`) | Automatic |
| Narrative structure | None — caller decides role | Built-in arc (opening→closing) |
| Use case | Adding a slide to an existing deck | Building from scratch |

## Error Codes

| Code | Cause |
|------|-------|
| `MISSING_PARAMETER` | `intent` not provided |
