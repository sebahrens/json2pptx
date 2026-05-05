# analyze_deck_rhythm

Analyze a presentation's visual rhythm — pattern repetition, density variation, and accent usage across slides.

**Added in:** 3.1.0

## When to Use

Use `analyze_deck_rhythm` **before** calling `generate_presentation` to detect monotony and inform pattern choices. Unlike `score_deck` (which requires a full generation pass), this tool performs lightweight static analysis on the JSON input.

Typical workflow:
1. Build your slide array (manually or via `plan_deck`)
2. Call `analyze_deck_rhythm` to check rhythm quality
3. If `composition_score` is low or recommendations appear, adjust patterns
4. Call `generate_presentation`

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `presentation` | object | Yes | Presentation definition — same schema as `generate_presentation` |
| `presentation.template` | string | No | Template name (unused by analysis, but accepted for schema compatibility) |
| `presentation.slides` | array | Yes | Array of slide definitions (at least one required) |

## Output Schema

```json
{
  "per_slide": [
    {
      "slide_index": 0,
      "pattern": "kpi-3up",
      "density_class": "med",
      "accent_role": "accent1",
      "dominant_visual": "pattern"
    }
  ],
  "aggregates": {
    "pattern_runs": [
      {"name": "content", "start": 3, "len": 3}
    ],
    "longest_run": 3,
    "repetition_index": 0.4,
    "accent_balance": {"accent1": 0.6, "accent2": 0.4},
    "density_cv": 0.28
  },
  "recommendations": [
    {
      "slide_index": 4,
      "message": "break a content run (length 3); consider inserting a different pattern at slide 4",
      "recommended_break_patterns": ["arch-stack", "bmc-canvas", "stat-hero"]
    }
  ],
  "composition_score": 75
}
```

### Per-Slide Fields

| Field | Type | Values | Description |
|-------|------|--------|-------------|
| `slide_index` | int | 0-based | Position in the slides array |
| `pattern` | string | pattern name, `"shape_grid"`, `"content"`, or slide_type | Visual fingerprint of the slide |
| `density_class` | string | `"low"`, `"med"`, `"high"` | Content density estimate |
| `accent_role` | string | `"accent1"`–`"accent6"` or `"none"` | First accent color reference found |
| `dominant_visual` | string | `"chart"`, `"diagram"`, `"table"`, `"text"`, `"grid"`, `"pattern"`, `"image"` | Primary visual type |

### Aggregates

| Field | Type | Description |
|-------|------|-------------|
| `pattern_runs` | array | Consecutive runs of 2+ slides with the same pattern |
| `longest_run` | int | Length of the longest consecutive pattern run |
| `repetition_index` | float | 0.0 (all unique) to 1.0 (all same pattern) |
| `accent_balance` | object | Fraction of accented slides using each accent color (sums to 1.0) |
| `density_cv` | float | Coefficient of variation of density scores (higher = more varied density) |

### Recommendations

Generated when pattern runs reach 3+ slides. Each recommendation suggests:
- Which slide to change (`slide_index`)
- What the problem is (`message`)
- Alternative patterns from a different visual family (`recommended_break_patterns`)

### Composition Score

An overall 0–100 score reflecting deck composition quality. Penalties:
- Long pattern runs (3+): -10 per run, -5 per additional slide beyond 3
- High repetition index (>0.7): -20; (>0.5): -10
- Low density variation (CV < 0.1 with 4+ slides): -10
- Accent imbalance (one accent >80%): -10

## Example

```json
// Request
{
  "presentation": {
    "template": "midnight-blue",
    "slides": [
      {"slide_type": "title", "title": "Q4 Strategy"},
      {"pattern": {"name": "kpi-3up"}, "title": "Key Metrics"},
      {"pattern": {"name": "stat-hero"}, "title": "The Opportunity"},
      {"pattern": {"name": "arch-stack"}, "title": "Architecture"},
      {"pattern": {"name": "comparison-2col"}, "title": "Competitive Landscape"},
      {"slide_type": "content", "title": "Financial Model"},
      {"pattern": {"name": "roadmap-phased"}, "title": "Execution Plan"},
      {"slide_type": "content", "title": "Next Steps"}
    ]
  }
}
```

```json
// Response
{
  "per_slide": [
    {"slide_index": 0, "pattern": "title",          "density_class": "low",  "accent_role": "none", "dominant_visual": "text"},
    {"slide_index": 1, "pattern": "kpi-3up",        "density_class": "med",  "accent_role": "none", "dominant_visual": "pattern"},
    {"slide_index": 2, "pattern": "stat-hero",      "density_class": "med",  "accent_role": "none", "dominant_visual": "pattern"},
    {"slide_index": 3, "pattern": "arch-stack",     "density_class": "med",  "accent_role": "none", "dominant_visual": "pattern"},
    {"slide_index": 4, "pattern": "comparison-2col","density_class": "med",  "accent_role": "none", "dominant_visual": "pattern"},
    {"slide_index": 5, "pattern": "content",        "density_class": "low",  "accent_role": "none", "dominant_visual": "text"},
    {"slide_index": 6, "pattern": "roadmap-phased", "density_class": "med",  "accent_role": "none", "dominant_visual": "pattern"},
    {"slide_index": 7, "pattern": "content",        "density_class": "low",  "accent_role": "none", "dominant_visual": "text"}
  ],
  "aggregates": {
    "pattern_runs": [],
    "longest_run": 1,
    "repetition_index": 0.12,
    "accent_balance": {},
    "density_cv": 0.28
  },
  "recommendations": [],
  "composition_score": 100
}
```

## Composing with Other Tools

| Workflow Step | Tool | Purpose |
|---|---|---|
| 1. Plan outline | `plan_deck` | Get pattern recommendations with rhythm rules built in |
| 2. Check rhythm | **`analyze_deck_rhythm`** | Validate the plan before generating |
| 3. Fix runs | `recommend_pattern` (with `prefer_variety=true`) | Find replacement patterns for flagged slides |
| 4. Generate | `generate_presentation` | Render the deck |
| 5. Score | `score_deck` | Post-generation quality assessment |
| 6. Repair | `repair_slide` | Fix content overflow or color issues |

## Error Codes

| Code | Cause |
|------|-------|
| `MISSING_PARAMETER` | `presentation` not provided or has zero slides |
| `INVALID_JSON` | `presentation` object is malformed |
