# Troubleshooting

Findings, refusals and renders that surprised agents in the journey runs, with
the fix that worked. `describe_finding(code)` explains any code; the full
catalogue is [`docs/FIT_FINDINGS.md`](../FIT_FINDINGS.md) and the agent digest
[`skills/generate-deck/FINDINGS.md`](../../skills/generate-deck/FINDINGS.md).
Back to the [hub](README.md).

## Authoring errors

| You see | It means | Do |
|---|---|---|
| `SEMANTIC_UNKNOWN_FIELD … content is DROPPED; did you mean "items"?` | The kind's schema is closed; your key is not in it | Rename to the suggested key (`comparison.columns[].items`, not `rows`/`bullets`; `architecture.rails`, not `cross_cutting`; `table.headers`, not `columns` is accepted as an alias). Check with `list_slide_kinds kinds:[kind] fields:["item_schema"]`. |
| `SEMANTIC_UNKNOWN_KIND "bridge"` on a region | Region kinds are `chart / stat / kpis / table / timeline / image / text` | A bridge beside text is a `chart` region with `type: waterfall` and `data.points`. |
| `SEMANTIC_PATTERN_DEGRADED … degrades to a bullet list`, with `max_chars` / `max_items` | The visual's budget is exceeded; the engine will draw bullets instead | Shorten to the budget, cut an item, or split — never accept the degradation silently. Executive summary: 3–5 points, ~52 characters per lead; process: label ≤ 60 and description ≤ 180 (rows) or 80 together (flow boxes); KPI values ≤ 12 characters. |
| `SEMANTIC_WEAK_CONTENT` | `__FILL__`, "Replace with …", or a file name containing "placeholder" | Write the real copy; rename the asset. |
| `SEMANTIC_IMAGE_MISSING` / `IMAGE_PATH` | The picture is not where the path says | CLI: paths resolve against the spec file's directory; MCP: against `base_dir`. Never ship a labelled placeholder frame. |
| `SEMANTIC_DENSITY … table has N rows including the header; the renderer lays out at most 10` | More than nine data rows | Move the tail to an appendix table or split as `<title> (2/2)`. A one-line cell with a comma list of 3+ items counts extra only past 48 characters. |
| `table_rows_truncated … split the table at row N` | Even the compact row pitch cannot hold the rows in this frame | Split at `split_at_row`; keep both halves' headers. |
| `INVALID_PARAMETER … table row 6 spans 7 columns but headers define 6` | A YAML flow list with an unquoted comma inside a cell | Quote cells that contain commas: `"Market model v1: size, growth, segments"`. |
| `CHART_SERIES_LENGTH_MISMATCH` | One value missing in a series | One number per category per series. |
| `SEMANTIC_TAKEAWAY_REQUIRED` | An evidence slide without a one-line conclusion | Add `takeaway`; on `executive_summary` / `decision` / `chart_insight` write the kind's own band instead. |
| `SEMANTIC_RHYTHM_MONOTONY` / `SEMANTIC_RHYTHM_DENSITY` | Three of a kind in a row, or three dense slides | Change the middle slide's kind; a split table titled `(2/2)` counts once. |
| `SEMANTIC_RHYTHM_SECTIONING` | 11+ body slides and no dividers | Use `structure.sections` with `auto_agenda`, or accept it under 12 slides (being relaxed, `go-slide-creator-th6o9`). |
| `SEMANTIC_REFERENCE_UNRESOLVED` | `recommended` / `decisive_criterion` names nothing | Match an option `name` or a criterion `label` exactly. |

## Rendering surprises (the tools said fine)

| You see in the image | Why | Do |
|---|---|---|
| Waterfall opening bar is a stub; line chart starts at the data minimum | An authored `data.y_min` zooms the axis (non-negative data starts at zero by default since `go-slide-creator-929jm`) | Remove `y_min`, or keep it and say "axis from N" in the heading; `chart.axis_not_zero` flags a `y_min` that hides more than half of the smallest bar. |
| Footer lost its project code or date, or ends in "…" | The footer line is wider than the template's slot: `CHROME_TRUNCATED` names the dropped fields and `max_chars` | Shorten `meta.chrome.client` / `confidentiality` to the budget, or remove the field you can spare (`go-slide-creator-m2tlt`). |
| Right column of a `regions` slide floats high; lower half empty | Stacked regions are content-sized, top-anchored | Add a third region (a small table, a timeline) or move to a chart-only slide (`go-slide-creator-18dqh`). |
| Four KPI cards with the bottom half of the slide empty | kpi-4up row is content-sized | Use `regions` with a chart beside the number, or six KPIs, or a `stat`. |
| Screenshot at a third of the width | `image_case` default picture column (45%) | `image_width_pct: 55` or `60` — and shorten the body, or the narrower text column drops the eyebrow below 12pt (`TEXT_BELOW_READABLE_MIN`). |
| Status board text at 5–6pt in validation, or the verified fix is `layout: content` | Per-option `detail` lines on a five-row matrix | Drop the `detail` lines; the board renders at 12pt (`go-slide-creator-u8orh`). |
| `TEXT_BELOW_READABLE_MIN` at `/slides/N/pattern/rows/3/cells/0/shape/text` with no text shown | A raw pattern cell the engine generated, reported one per render | Shorten the longest value in that row of your `pattern.values`; re-render; expect another until all fit (`go-slide-creator-llxzd`). |
| Removing a milestone made roadmap text smaller | Lane height depends on the milestone row | Keep a milestone row, or use the kind's `phases` with fewer deliverables. |
| Second template: title wraps to three lines | Serif face / capitals eat the budget | Rewrite within `list_slide_kinds fields:["brief"] template:"p-style"` budgets. |

## Tooling and discovery

| Problem | Do |
|---|---|
| `plan_deck` left facts unplaced or chose a kind you disagree with | Fixed routing in `go-slide-creator-xbwlt`; what remains in `unplaced_facts` is your checklist, and the kind table in [visual-vocabulary.md](visual-vocabulary.md) is the tie-breaker. |
| `recommend_visual` returned no split candidate / ranked KPI cards for a status board / only `content` for a finding slide | Name the candidate: `candidates: ["regions"]`, `["table-highlight"]`, `["labeled-rows"]` (fixed in `go-slide-creator-ux1fl`). |
| A raw pattern's fields are unknown | `show_pattern(name)` — in the default profile — returns the schema and `example_values`. |
| Your local spec drifted from the stored deck | `validate_deck_spec {deck_id, read: "spec"}`. |
| `deck_id` expired (one hour) or the server restarted | Send the spec again; keep the source spec yourself. |
| Thumbnail responses are large or truncated | `known_hashes` for unchanged slides; `slide_indices` for the rest; `density: 100` only for the slide you cannot read. |
| No render tooling (`runtime.render_available: false`) | Deliver as **unreviewed** and say so; do not approve. |
