# Deck Skeletons

Five canonical JSON skeletons covering the most common deck archetypes. Copy a skeleton, replace every `__FILL_*__` token, strip the `_*` documentation keys (`_skeleton`, `_description`, `_instructions`), and run `validate_input` before generating.

Skeletons are pre-shaped for **constrained mode**: they set `design_mode: "constrained"`, `accent_strategy: "rotate"`, `contrast_check: true` on every slide, and (for skeletons containing chart or matrix slides) include the `takeaway` field required by Rule 11a. The rhythm of each skeleton has been picked so no pattern appears 3+ times consecutively — do not stack additional slides of the same pattern type without re-running `analyze_deck_rhythm`.

| File | Slides | When to use | Patterns used |
|------|--------|-------------|---------------|
| [`exec-summary.json`](exec-summary.json) | 6 | Board updates, executive briefings — lead with the headline number, then evidence, decision, validation, close | stat-hero, kpi-3up, matrix-2x2, pull-quote |
| [`data-heavy.json`](data-heavy.json) | 7 | QBR, performance reviews, any deck dominated by charts — every chart slide carries a takeaway | chart (line, bar, donut), table, kpi-4up |
| [`comparison.json`](comparison.json) | 5 | Today-vs-target, vendor selection, positioning narratives — three comparison frames (transformation, capability table, 2x2) | before-after, comparison-2col, matrix-2x2 |
| [`process-roadmap.json`](process-roadmap.json) | 6 | Program walkthroughs, delivery plans, operating-model rollouts | process-flow, swimlane, timeline-horizontal, kpi-3up |
| [`pitch.json`](pitch.json) | 9 | Investor pitches, sales decks, partner proposals — classic problem→solution→traction→ask arc | icon-row, stat-hero, before-after, kpi-3up, card-grid, pyramid, pull-quote |

## How to use

1. **Pick** the skeleton closest to your deck's purpose.
2. **Copy** it to your working directory (or load it inline).
3. **Replace** every `__FILL_*__` token with concrete content. Tokens are descriptively named (`__FILL_KPI1_BIG__`, `__FILL_MATRIX_TAKEAWAY__`, etc.) — the hint in parentheses tells you what kind of content the field expects.
4. **Strip** the documentation-only keys: `_skeleton`, `_description`, `_instructions`. These exist so agents can recognize the skeleton at a glance; they will be flagged as unknown keys by `validate_input --strict-unknown-keys`.
5. **Validate** with `validate_input` (MCP) or `json2pptx validate --fit-report` (CLI). Fix any findings before calling `generate_presentation`.
6. **Generate**.

## Why skeletons exist

Generating a deck from scratch each time leads to two recurring problems: (1) agents forget non-obvious requirements (the takeaway field on chart/matrix slides, `accent_strategy: "rotate"` for long decks, `contrast_check` defaults), and (2) agents default to a single pattern (often 6-card-grids-in-a-row), producing visually monotonous decks. Skeletons encode the rhythm and the required fields up-front so the agent's job is content, not structure.

## Customizing a skeleton

- **Need more slides?** Insert them between existing slides, but check `analyze_deck_rhythm` — do not introduce a third consecutive use of any pattern (see [RULES.md](../../RULES.md) → Pattern monotony).
- **Different template?** Any of `midnight-blue`, `forest-green`, `modern-template`, `warm-coral` will work. Patterns and layouts are template-portable.
- **Need a chart inside a pattern slide?** Use a `compose` envelope (see [WORKFLOW.md](../../WORKFLOW.md)) — do not flatten the pattern through `shape_grid`.
- **Need to swap a pattern?** Use `recommend_visual` first; do not guess pattern names.
