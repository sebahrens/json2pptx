# plan_deck

Plan a presentation deck from a natural-language brief — returns an ordered slide outline with recommended patterns and narrative roles.

**Added in:** 3.1.0

## When to Use

Use `plan_deck` as the **first step** when building a deck from scratch. It converts a brief into a structured outline that:
- Assigns narrative roles (opening, framework, evidence, comparison, emphasis, closing)
- Recommends patterns for each slide using taxonomy-aware matching
- Enforces deck-rhythm rules automatically (no 3+ consecutive same-pattern runs, emphasis every ~5 slides)
- Produces output directly consumable as the `slides` array in `generate_presentation`

Skip `plan_deck` when you already have a detailed slide-by-slide outline or when modifying an existing deck.

## Input Schema

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `brief` | string | Yes | — | Natural-language description of the deck purpose and content |
| `slide_budget` | number | No | 10 | Target number of slides (clamped to 3–30) |
| `audience` | string | No | — | Target audience (influences pattern selection) |
| `must_include` | array of strings | No | — | Pattern names that must appear in the plan |

## Output Schema

```json
{
  "slides": [
    {
      "slide_index": 0,
      "narrative_role": "opening",
      "recommended_pattern": "stat-hero",
      "content_seed": "Title and context: Pitch our Series B...",
      "rationale": "fallback selection for opening"
    }
  ],
  "brief": "Pitch our Series B for an AI infra company",
  "slide_budget": 10,
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
| `recommended_pattern` | string | Pattern name to use (from `list_patterns`) |
| `content_seed` | string | Brief hint of what content belongs on this slide |
| `rationale` | string | Why this pattern was selected (e.g., "required by must_include", "rhythm break") |

### Rhythm Check

| Field | Type | Description |
|-------|------|-------------|
| `longest_pattern_run` | int | Longest consecutive run of the same pattern (target: ≤2) |
| `has_emphasis` | bool | Whether at least one emphasis slide (stat-hero or pull-quote) exists |
| `emphasis_count` | int | Number of emphasis slides |
| `pattern_variety` | int | Count of unique patterns used |

## Narrative Roles

The planner distributes slides across a standard narrative arc:

| Role | Fraction | Purpose |
|------|----------|---------|
| `opening` | ~10% | Title, context-setting |
| `framework` | ~15% | Structure or methodology overview |
| `evidence` | ~40% | Supporting data, details, case studies |
| `comparison` | ~15% | Compare alternatives or trade-offs |
| `emphasis` | ~10% | Standout metric or memorable quote (visual breathing room) |
| `closing` | ~10% | Summary and next steps |

## Rhythm Rules

The planner automatically enforces:

1. **No 3+ consecutive runs** — if detected, the middle slide is swapped to a pattern from a different visual family
2. **Emphasis injection** — at least one `stat-hero` or `pull-quote` every ~5 slides
3. **Variety awareness** — `recommend_pattern` is called with `prefer_variety=true` to penalize recently-used patterns

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
    {"slide_index": 0, "narrative_role": "opening",    "recommended_pattern": "stat-hero",      "content_seed": "Title and context: Series B pitch for an AI infrastructure...", "rationale": "fallback selection for opening"},
    {"slide_index": 1, "narrative_role": "framework",  "recommended_pattern": "kpi-3up",        "content_seed": "Structure or methodology overview", "rationale": "required by must_include"},
    {"slide_index": 2, "narrative_role": "evidence",   "recommended_pattern": "arch-stack",     "content_seed": "Key data point or supporting detail", "rationale": "taxonomy match: evidence+structural"},
    {"slide_index": 3, "narrative_role": "evidence",   "recommended_pattern": "card-grid",      "content_seed": "Key data point or supporting detail", "rationale": "variety pick: different visual family"},
    {"slide_index": 4, "narrative_role": "emphasis",   "recommended_pattern": "pull-quote",     "content_seed": "Standout metric or memorable takeaway", "rationale": "emphasis injection: visual breathing room every ~5 slides"},
    {"slide_index": 5, "narrative_role": "comparison", "recommended_pattern": "comparison-2col","content_seed": "Comparison of alternatives or trade-offs", "rationale": "taxonomy match: comparison"},
    {"slide_index": 6, "narrative_role": "evidence",   "recommended_pattern": "roadmap-phased", "content_seed": "Detailed evidence or case study", "rationale": "required by must_include"},
    {"slide_index": 7, "narrative_role": "closing",    "recommended_pattern": "icon-row",       "content_seed": "Summary, next steps, or call to action", "rationale": "fallback selection for closing"}
  ],
  "brief": "Series B pitch for an AI infrastructure company with $50M ARR",
  "slide_budget": 8,
  "rhythm_check": {
    "longest_pattern_run": 1,
    "has_emphasis": true,
    "emphasis_count": 1,
    "pattern_variety": 7
  }
}
```

## Composing with Other Tools

| Workflow Step | Tool | Purpose |
|---|---|---|
| 1. Plan | **`plan_deck`** | Get a rhythm-aware outline from a brief |
| 2. Validate rhythm | `analyze_deck_rhythm` | Confirm the plan has good visual variety |
| 3. Fill content | Use `show_pattern` + `expand_pattern` | Get schema for each pattern and populate values |
| 4. Generate | `generate_presentation` | Render the final PPTX |
| 5. Repair | `repair_slide` | Fix overflow or color issues |

### From plan_deck output to generate_presentation input

The `plan_deck` output is a skeleton — to generate slides you need to:

1. For each slide with a `recommended_pattern`:
   - Call `show_pattern(name)` to get the values schema
   - Populate `values` based on your content and the `content_seed` hint
   - Include as `{"pattern": {"name": "...", "values": {...}}, "title": "..."}`

2. For slides where `recommended_pattern` is a slide type (e.g., `"content"`):
   - Use standard content items: `{"slide_type": "content", "content": [...]}`

## Error Codes

| Code | Cause |
|------|-------|
| `MISSING_PARAMETER` | `brief` not provided |
| `INVALID_PARAMETER` | A `must_include` pattern name doesn't exist |
