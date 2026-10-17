# Layout review 2026-10-17: visuals, missing layout types, pane splitting

Scope: how agents' slides can look better, which consulting layout types the engine still lacks, and how slide real estate is split into panes horizontally and vertically. Follows the 2026-10-02 round (`layout-composition-20261002/report.json`, 12 beads, all closed) that added the typed `regions` kind and the discovery path. Every finding below is one bead labelled `layout-review-20261017`.

## Method

1. External research on consulting slide layouts (sources below) to build a reference taxonomy of body divisions and slide archetypes.
2. Four code audits: compose/regions/shape_grid splitting; pattern + diagram + DeckSpec kind coverage against the taxonomy; native slide-type geometry and `analyze_deck_rhythm`; quality evidence in `tests/quality/` versus engine finding codes.
3. Each claim spot-checked against the current `main` (38f1e36d); beads cited by the evidence files were re-checked and all 18 were already closed, so only gaps with no finding code or no pattern were filed.

## Reference taxonomy from the research

**Anatomy.** Action title (<= 2 lines) -> optional unit / subheading line -> body -> source and footnote block -> page number; optional tracker (section) and sticker (Preliminary / Illustrative). A takeaway box is a secondary inference, used on < 20% of slides and never a restatement of the title.

**Body divisions that recur across the three independent taxonomies** (strategyu 14, deckary 9, tosea 14):

| Division | Proportions | Notes |
|---|---|---|
| Single exhibit | chart 70-80% of slide | McKinsey, IB |
| Exhibit + insight callout | 2/3 : 1/3 | McKinsey exhibit left, BCG chart + callout |
| Text + visual two-column | 50/50 or 60/40 | BCG/Bain: takeaways beside the exhibit |
| Mirrored comparison | equal | same attributes, same order |
| Three equal columns | 33/33/33 | pillars; exec summary as S/F/R |
| Grid / bento | 2x2, 3x2, <= 6 boxes | dashboards |
| KPI tile row over two panels | 4-6 tiles + 2 panels | check-ins |
| Timeline with three layers | bars / milestones / risks | roadmaps |
| Quote + evidence | 1/2 : 1/2 | research synthesis |
| Full-bleed image + band | overlay | dividers, < 10% of slides |

Rules: 12-column grid, 24-32px gutters; >= 15-20% of the area empty; 3-5 layout types per deck; 3-4 colours; identical title/footer positions; column headings <= 3 words.

**Archetype canon** (deckary 10, strategyu 14, pptpowertools 21, think-cell): executive summary, issue/hypothesis tree, 2x2, waterfall, Gantt/roadmap, RACI, process flow, comparison table, Harvey-ball scorecard, chart + callout, big number, three pillars, funnel, quote + evidence, dashboard, layered timeline, next steps, frameworks (SWOT, Porter, 7S), option assessment, competitor matrix, org chart, Mekko, butterfly, combination chart, Pareto, decision tree, fishbone, bullseye, staircase. About 20 slide types cover 80% of real McKinsey layouts (Slideworks corpus).

## What is already covered

56 named patterns, 21 diagram types, 29 DeckSpec kinds. 2/3:1/3 asymmetry (`chart-insights-split`, regions `main_*`), L/T shapes (regions `main_*`, compose depth 2), unequal splits (`size_pct`), bottom takeaway band, Harvey-ball matrix, risk heat map (3x3/5x5), org chart, funnel, waterfall, cycles, pull-quote with headshot, annotated image (`image_case`), agenda/tracker.

## Findings -> beads

### Pane splitting: compose / regions / shape_grid

- `go-slide-creator-g6kgx` compose: let a segment hold a table, text block, image or raw shape_grid, not only pattern | compose | diagram
- `go-slide-creator-t7i8w` compose: per-segment heading / caption instead of the heavy banner
- `go-slide-creator-ogzhj` Vertical pane divider: column rule in shape_grid, compose and regions (only horizontal row rules exist)
- `go-slide-creator-bklk1` compose: banner and callout bands are appended after segments are sized, so segments are expanded for height they do not get
- `go-slide-creator-0pmcd` regions: four-region arrangements (2x2 quad, main + 3, KPI tile row over two panels)
- `go-slide-creator-mejgz` compose: content-sized vertical segment (size: "content") so a short pane does not claim its percentage share
- `go-slide-creator-y88ny` compose: raise the nesting cap to 3 (or make it leaf-bounded) so an L-shaped layout can re-split its side pane
- `go-slide-creator-po8ym` Sibling panes do not share one body type size unless a grow mode is active (compose segments, regions)
- `go-slide-creator-ds1qu` compose / regions: per-seam gap and an explicit gap on regions slides
- `go-slide-creator-qa0j0` docs: SEMANTIC_COMPILER.md regions row omits the cycle and process region kinds
- `go-slide-creator-45uez` compose: smart_compose weights by non-empty cell count and is ignored once any size_pct is set
- `go-slide-creator-dklkh` compose: COMPOSE_HORIZONTAL_TRUNCATION drops cells beyond a segment's column count as a warning, not a refusal

### Native slide types

- `go-slide-creator-xl48v` Native two-column: offer 60/40, 70/30 and 2/3-1/3 splits, not only native / 65 / 35
- `go-slide-creator-qhsg7` Layout aliases three-column / sidebar-* / four-column map to generated IDs (content-3, content-2-70-30) that most templates never synthesize
- `go-slide-creator-1foqw` Native stacked split: chart above table / text above diagram with a tunable share (fixed 25% text today)
- `go-slide-creator-x4zx7` image-left / image-right native layouts exist only on the modern template; derive an image column elsewhere
- `go-slide-creator-9rcbq` Narrow-placeholder chart warning hard-codes a 16:9 slide width
- `go-slide-creator-8r651` Native two-column: explicit per-column heading field instead of the both-columns heuristic
- `go-slide-creator-t2k26` bullet_groups cannot flow groups across two native columns

### Missing archetypes

- `go-slide-creator-xf0nr` New pattern/kind: RACI responsibility matrix (recommender currently routes 'raci' to swimlane)
- `go-slide-creator-jymhe` New pattern/kind: RAG scorecard / status board with trend arrows
- `go-slide-creator-32ofk` New pattern/kind: issue tree / hypothesis tree (driver-tree is metric-shaped and capped at 3 levels)
- `go-slide-creator-pkqso` DeckSpec: give venn, fishbone, pestel, nine_box_talent, heatmap, value_chain and pyramid a host kind
- `go-slide-creator-0bykm` Stakeholder power-interest map: labelled, sized, sentiment-coded bubbles on a 2x2
- `go-slide-creator-0s0nb` Bullet chart / target-vs-actual progress bars list
- `go-slide-creator-u4om1` Mekko / Marimekko chart
- `go-slide-creator-a6vqk` Vertical timeline pattern (top-to-bottom dated milestones)
- `go-slide-creator-anphd` Decision tree with branches and outcomes
- `go-slide-creator-8sy4w` Checklist / tick list and Do-Don't tick-cross variant of comparison-2col
- `go-slide-creator-z6pmd` Full-bleed image hero with headline band pattern
- `go-slide-creator-o6oq1` Combination chart: columns with a line on a secondary axis (revenue bars + margin % line)
- `go-slide-creator-p67bm` Butterfly / tornado chart (two mirrored horizontal bar series)
- `go-slide-creator-nztns` Sankey / flow-of-value diagram (explicitly unsupported today)
- `go-slide-creator-9yajk` Three-horizons / S-curve framework pattern
- `go-slide-creator-17sn0` Spectrum / continuum slider (position options along one axis)
- `go-slide-creator-2o6qn` Sticker chrome element: 'Preliminary' / 'Illustrative' / 'For discussion' top-right tag
- `go-slide-creator-wt5c4` Sources / bibliography appendix pattern
- `go-slide-creator-ti9rm` Section divider with chapter list (current chapter emphasised)
- `go-slide-creator-qjyk7` Two-column running-prose memo pattern
- `go-slide-creator-4gpsg` Input -> process -> output three-box pattern with stacked lists per box

### Visual quality and rhythm checks

- `go-slide-creator-7o6o8` Engine finding for baseline misalignment across sibling panes (BASELINE_MISALIGN is QA-only)
- `go-slide-creator-c0s64` Rhythm: fingerprint pane count, visual side and split ratio; add same_side_run and an area-based visual share
- `go-slide-creator-n69rt` Finding for type hierarchy: title / body / caption size ratios
- `go-slide-creator-xpxk0` Finding for inconsistent gutters and margins across a deck
- `go-slide-creator-bx05x` Finding for uneven whitespace distribution within a slide (cramped pane beside an empty one)
- `go-slide-creator-jeeav` Finding for an evidence image that is too small for its role
- `go-slide-creator-3uwfx` Finding for orphan words and bad breaks in titles and headings
- `go-slide-creator-11tio` Finding for inconsistent number and unit formatting across a deck
- `go-slide-creator-6w91j` Decide chart styling checks from chart data, not screenshots (redundant legend, vertical gridlines)
- `go-slide-creator-hw8oc` Finding when status colours on one slide are not distinguishable from each other
- `go-slide-creator-hqnke` Finding for a table-split continuation slide left mostly empty
- `go-slide-creator-9yqcv` Finding for an element detached from the content it belongs to (proximity / grouping)
- `go-slide-creator-ek2tp` Takeaway discipline: flag a takeaway that restates the title and a deck where most slides carry one
- `go-slide-creator-p23nz` Rhythm: upper bound on layout variety (too many distinct families per deck)

## Priority view

P2 (file first): compose leaf kinds (g6kgx), per-segment heading (t7i8w), vertical divider (ogzhj), compose band reserve bug (bklk1), four-region arrangements (0pmcd), content-sized vertical segment (mejgz), shared sibling type size (po8ym), native 60/40 + 70/30 (xl48v), dangling three-column aliases (qhsg7), RACI (xf0nr), RAG status board (jymhe), issue tree (32ofk), host kinds for 7 diagrams (pkqso), BASELINE_MISALIGN engine finding (7o6o8), rhythm pane/side fingerprint (c0s64).

## Sources

- https://strategyu.co/slide-layouts/
- https://deckary.com/blog/powerpoint-layout-ideas
- https://deckary.com/blog/consulting-slide-templates
- https://deckary.com/blog/consulting-slide-standards
- https://tosea.ai/blog/powerpoint-layout-patterns-complete-guide
- https://pptpowertools.com/slides-every-consultant-needs-in-their-template-deck/
- https://pptpowertools.com/the-anatomy-of-a-consulting-slide/
- https://www.theanalystacademy.com/takeaway-box/
- https://www.thinkcell.com/en/resources/content-hub/find-the-right-visualization-for-your-presentation
- https://slideworks.io/resources/47-real-mckinsey-presentations
- https://slideworks.io/resources/how-mckinsey-consultants-make-presentations
- https://poesius.com/blog/consulting-slide-design-mckinsey-bcg-bain-guide
- https://www.auxi.ai/blog/ultimate-guide-to-consulting-powerpoint-slides

Code evidence per finding is in each bead's description (file:line on 38f1e36d). Internal evidence files: `tests/quality/findings_histogram.csv`, `tests/quality/results/agent-journey-20261003/`, `agent-journey-20261005/`, `consulting-benchmark-20261003/`.
