# j-ops-operating-rhythm: an operations review built around a loop (MCP, default tools)

You are an operations lead's AI assistant. The brief:

> Build the operations review for "Harbourline Logistics" for the COO. 6–7 slides on `midnight-blue`. Our operating rhythm is a six-phase continuous improvement loop fed by a two-step onboarding intake. Intake: sign the service contract, onboard the site. The monthly loop: plan the month, run the service, measure against the SLA, review with the client, fix root causes, reset the targets. KPIs: on-time delivery 96.5%, rework −34%, sites onboarded 14, reviews held on time 4 of 9. The review phase is the weak one: it slipped past month end in 5 of the last 9 months. Ask: approve a dedicated review lead from November.

One slide is **required to be the loop as a picture**: the two onboarding steps drawn as steps that happen once, feeding a ring of the six monthly phases, with the review phase the one that stands out. Not a bullet list, not a straight process. Say in your report how you found the shape, how you expressed it and how many attempts it took.

Also required: an action-titled executive summary, the four KPIs, and a next-steps closer that carries the ask.

## Do

1. Start the bridge with the default tools. Read `log/initialize.json` and `tools/list`; follow what the product tells you to do first.
2. Call `plan_deck` with the brief (`format: "deckspec"`). Record the kind and the fields it drafted for the loop slide, and whether you kept them.
3. Before authoring the loop slide, ask `recommend_visual` for it in your own words (twice: once as "operating rhythm", once describing the intake) and `list_slide_kinds` for the kind it names, with `preview: true`. Record whether the first candidate was what you used and whether the previews settled the style.
4. Author, validate, render. Then break the loop slide on purpose, one at a time, and record what each finding told you to do and whether doing it worked first time: (a) nine phases; (b) the loop as two coupled loops in the left half of a slide with the KPIs beside it; (c) a hub-and-spoke style with no centre.
5. Render every slide, look at every image, repair what looks wrong (three rounds at most), finish the completion protocol.
6. Render the final spec on `p-style` (when `$REPO/templates/p-style.pptx` exists, otherwise `warm-coral`) and look at every slide again.
7. Save every spec revision you sent as `$J/<persona>/spec-vN.json` (or .yaml); keep the final PPTX paths in `report.md`.

## Measure

- Calls from first contact to a loop slide that renders as a loop, and which response named the kind and the style.
- For each deliberate break in step 4: the finding's code and path, whether its message alone was enough to fix the slide, round trips to clean.
- Whether the ring stayed a circle, the intake read as "once, then the loop", and the highlighted phase was the only solid accent — on both templates.
- Validate and render calls to the first `deterministic_ready: true`.
- Every slide the tools scored clean that looked wrong, with the image path.
