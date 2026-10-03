# b-discovery: finding the right layout for fourteen slide intents

You know what each slide should show; you do not know what json2pptx calls it or what data it takes. Template: `midnight-blue`.

## Intents

1. A strategy house with four pillars and two enabling layers
2. Five option cards
3. A sales funnel with conversion rates
4. An org chart of seven people
5. A line chart, a KPI and a three-milestone timeline on one slide
6. Before / after of a process
7. A five-stage maturity model showing where we are
8. A value chain with one highlighted step
9. A swimlane across three teams
10. A 2x2 with six plotted items
11. A Gantt for six workstreams
12. Eight customer quotes
13. Three vendors scored on five criteria, with a recommendation
14. A screenshot with two callouts

## Do

Run two sessions: the default tool profile, then `--tools all` (its own socket and log directory). In each, for every intent:

- (a) find which visual to use; (b) find its data shape and its limits (how many items, how much text); (c) see what it looks like before authoring, if the product offers a picture (`preview: true` on `recommend_visual` and `list_slide_kinds`); (d) author one slide, validate, render, and look at the image (`render_deck_thumbnails`).
- When a recommendation has a close runner-up, render that too and say which was better.

## Measure

Per intent: calls and bytes for (a) and (b), whether a picture was available for (c), attempts to a valid render for (d), and whether the top recommendation was the best rendered option. Then the totals per profile and what the wider profile bought.
