# recommend_visual

Rank candidate visuals for a slide intent across **every** category —
placeholder layouts, named patterns, charts, diagrams, raw `shape_grid` and
`compose` — with scores, rationales, confidence bands and placement guidance.

**Added in:** 3.1.0

This is the primary recommender. The older `recommend_pattern` tool is a folded
alias that ranks named patterns only; see [recommend_pattern](./recommend_pattern.md)
for its pattern-only response (`beyond_patterns`, `expansion_preview`).

## When to Use

Ask `recommend_visual` first when you know **what a slide should show** but not
**how to show it**, then build the slide with the winning category's tool
(`show_pattern` / `expand_pattern` for a named pattern, the chart or diagram
schema from `get_input_schema` for a chart or diagram). Use `plan_deck` for a
whole-deck outline.

## Input Schema

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `intent` | string | Yes | — | Natural-language description of what the slide should show |
| `content_hints` | object | No | — | `item_count`, `has_chart`, `has_metrics`, `columns`, `data_points`, `series_count`, `audience`, `density_hint` (`low` / `medium` / `high`) |
| `recent_patterns` | array of strings | No | — | Patterns used on preceding slides, in order |
| `prefer_variety` | boolean | No | false | Apply a recency-decay penalty to `recent_patterns` |
| `slide_index` | number | No | — | 0-based index of the slide being built |
| `candidates` | array of strings | No | — | Explicit shortlist to rank; every name is returned (no threshold cutoff, no truncation), category auto-resolved, unknown names at score 0 |
| `template` | string | No | — | Template name; each candidate then carries `template_support` `{status, reasons[], required_layout}` and unsupported / risky candidates are demoted |

The live tool description and `get_input_schema` are authoritative; this table
summarises them.

## Output Schema

| Field | Description |
|-------|-------------|
| `candidates[]` | `category` (`placeholder_layout`, `named_pattern`, `chart`, `diagram`, `raw_shape_grid`, `compose`), `name`, `score`, `rationale`, `confidence_band` (`high` / `medium` / `low`), optional `diversity_bonus`, `placement`, `template_support`, `example` |
| `query_understood_as` | How the intent was interpreted |
| `unsupported_visual` | Set (e.g. `sankey`) when the requested visual has no renderer; `candidates` is then empty |
| `disambiguating_questions[]` | Questions to ask when the intent is ambiguous |
| `response_fingerprint` | sha256 of the canonical response; cache key / drift detector |
