# e-revise: eight revision turns on one deck (MCP, default tool profile)

A user asks for a deck and then keeps changing their mind. You never resend the whole spec if the product offers something smaller.

## Do

Build the baseline: an 8-slide investor update for "Tidewater Carbon" (direct air capture; ARR EUR 9.4M; 14 months of runway; pilot plant in Q1 2027; main risk: permitting delay; ask: introductions to two strategic partners) on `modern-yellow`. Render it, look at every slide. Then apply these turns in order, each as the user would say it (1-based slide numbers):

1. "Make slide 3 a chart instead."
2. "Add a slide after the roadmap comparing us with two competitors."
3. "Move the risk slide to just before the ask, and shorten its title."
4. "ARR is 9.6, not 9.4. Fix it everywhere."
5. "Put the burn chart and the roadmap on one slide."
6. "Switch the whole deck to the `abstract` template."
7. "Undo the last change you made to the title slide."
8. "Add an appendix with the detailed assumptions, and speaker notes on every slide."

After every turn: re-render, look at the slides that changed, and say what you had to send (bytes) and what you had to remember yourself. Finish with one full-deck pass and the completion protocol.

## Measure

- Per turn: calls, request bytes, response bytes, attempts, and whether you needed anything the product did not hold for you (the stored spec, slide positions, the previous value).
- Thumbnail bytes when re-inspecting only what changed, against a full pass.
- Whether index bases (0- or 1-based) and path notations stayed consistent across responses.
