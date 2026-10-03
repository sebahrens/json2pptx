# c-repair: repairing a hurried draft from diagnostics alone (MCP, default tool profile)

A colleague left you a hurried draft and went home. You fix it using only what the tools report: findings, their `path`, `next_tool_call`, remediation parameters and `describe_finding`.

## Do

1. Discovery is limited to `get_started` and `list_slide_kinds` without arguments.
2. Write the draft: "Cloud cost reduction programme", about 8-10 slides, template `modern`, with these flaws planted on purpose: an unknown `archetype`, one unknown slide kind, two misspelled keys, a chart series one value short of its categories, an executive summary with too many and too long points, seven KPIs on one slide, a ten-row table, over-long decision labels and process steps, topic titles instead of action titles, missing takeaways and no source. (The draft the first run wrote is call 4 of `tests/quality/results/agent-journey-20261003/c-repair/log-calls.jsonl`; `TestTwelveFlawDraftCleanInThreeRoundTrips` replays it. You may send that draft instead of writing your own; that one path is allowed.)
3. Validate, then repair round by round. Apply a suggested patch literally before trying your own fix, and record which of the two cleared the finding. Use `deck_id` + `patch` as soon as you have a `deck_id`.
4. Render, look at every slide, repair what only the render or the images show, look again, submit the review.
5. Render the final spec on `midnight-blue` and on `p-style` (when present) and look at those too.

## Measure

- Validate round-trips to a clean validate, render round-trips to `deterministic_ready`, and to zero diagnostics.
- Per finding: did the message alone say what to change and to what; did the suggested patch clear it; did fixing it raise a new finding.
- How much text the repairs cut, against what the messages asked for.
- Every slide the tools scored clean that looked wrong.
