---
title: "Consulting visual additions"
type: documentation
summary: "Native consulting visuals, chart contracts, and authoring alternatives"
status: active
last_updated: "2026-10-17"
tags: [json2pptx, consulting-visuals]
---

# Consulting visual additions

Choose the form from the claim; back to the [catalog](layout-catalog.md),
[visual vocabulary](visual-vocabulary.md) and [segment guide](slide-estate-and-segments.md).
`list_slide_kinds(kinds:[chosen], fields:["example","item_schema_full"])`
is the live closed contract. For a raw pattern use `show_pattern(name)`.
These forms preserve authored text and refuse geometry that cannot hold it.
Native shapes/text/tables stay editable; SVG charts are pictures, not Office
chart objects.
Built-in image-hero kind, pattern and gallery previews use an illustrative
operations image. Replace it with your own image when authoring a deck;
missing authored paths still produce an asset error. The hero composition
uses the full-slide `blank` layout so its headline stays inside the image band.

## Native kinds and their matching patterns

| Kind → pattern | Use and bounds |
|---|---|
| `raci` → `raci-matrix` | `roles` 2–6 and `activities` 3–10 `{activity, scores}`; each score R/A/C/I/-, exactly one A per row; printed legend. |
| `status_board` → `status-board` | `columns` 2–5, `rows` 3–10 `{workstream, statuses, trend?, comment?}`; red/amber/green per column, trend up/flat/down. Used status inks require contrast; optional `rag_colors`. |
| `checklist` → `checklist` | `items` 3–10 `{label, state: done|open|blocked, detail?}`; states have text and symbols. |
| `prose_2col` → `prose-2col` | Exactly two `columns`, each `{heading?, paragraphs}` with 1–4 paragraphs; running prose rather than bullet fragments. |
| `ipo_flow` → `ipo-flow` | Exactly three `boxes` `{heading, bullets}`; 2–5 bullets per input/process/output box. |
| `three_horizons` → `three-horizons` | Exactly three `horizons` `{title, timeframe, items}`; 1–4 initiatives per band. |
| `sources` → `sources-list` | `entries` 4–16 `{text, url?}`; numbered appendix references, measured at 10pt. |
| `timeline_vertical` → `timeline-vertical` | `stops` 3–8 `{date, title, detail?}`; optional `alternating`, `highlight`. A timeline region also accepts `orientation: vertical` with 3–7 dated milestones and no ranges. |
| `spectrum` → `spectrum` | `left_label`, `right_label`, 2–6 `markers` `{label, position}`; position 0–1, optional highlight. |
| `issue_tree` → `issue-tree` | `nodes` 3–15 `{id, parent?, label, edge?, status?, value?}`; one root, parent before child, 2–4 levels; status confirmed/rejected/open. |
| `decision_tree` → `decision-tree` | Same explicit node contract, 2–3 levels, labelled branches and optional outcome value. |
| `stakeholder_map` → `stakeholder-map` | `points` 2–12 `{label, x, y, size?, status?}`, x/y 0–100, size 1–40; supporter/neutral/blocker status and used-status legend; `x_axis_label`, `y_axis_label`. |
| `bullet_chart` → `progress-list` | `rows` 3–8 `{label, actual, target, unit?}`; finite nonnegative actual, positive target; actual bar and target tick on a shared row scale. |
| `image_hero` → `image-hero` | `image` with exactly one path/url, `headline` ≤160, optional eyebrow; `band: top|bottom|left` (default bottom). Full-slide image with native dark scrim and readable white text. Optional `takeaway` / `source` (≤200 each) stay inside the band; photo-overlapping automatic footer chrome is suppressed. On the raw path, slide/deck source and takeaway route into the same band; use `values.takeaway` instead of an external pattern callout. Use sparingly for opening or transitions. |

Except source appendices and dense numeric-matrix value captions, native
text is measured at ≥12pt. A contract's
maximum count does not promise that every maximum-length field fits together;
shorten the copy or allocate a larger frame when a measured refusal occurs.
RACI/status matrices retain all rows at the readable floor; ordinary raw
native tables keep their existing density limits.

Seven framework host kinds expose the existing diagram families directly:
`venn`, `fishbone`, `pestel`, `nine_box_talent`, `heatmap`, `value_chain`,
`pyramid`. Their closed data fields and examples come from the corresponding
pattern; use live discovery rather than placing arbitrary diagram data in a
kind. A quantitative heatmap differs from `risk_heatmap`'s likelihood × impact.
Its row and column labels stay at 12pt. Numeric values use the dense-caption
role (10pt minimum); the current native pipeline writes them at 12pt.
Crowded or long labels are refused instead of shortened silently. Stakeholder
labels also stay at 12pt, with explicit refusal when markers or labels cannot
be placed without collisions. Fishbone frameworks refuse actual SVG content
loss; the general raw diagram family retains its own contract.

## Three chart contracts

These are available as chart types inside `chart_insight`, chart regions,
raw chart content or diagram segments. Use `list_templates(fields:"full")`
for capability details and examples.

| Type | Data |
|---|---|
| `mekko_chart` | `segments` 2–8 `{name, width, parts}`; positive width weights, each with 2–6 matching named `{name, value}` parts, nonnegative values and positive total. Width encodes segment scale; heights encode each segment's 100% composition. |
| `combo_chart` | Shared `categories` 2–12, `bars` 1–3 named series, `lines` 1–2 named series, one finite nonnegative value per category, explicit `bar_unit` and `line_unit`. `secondary_axis: true` labels a separate right scale; `highlight` names a bar series. |
| `butterfly_chart` | `categories` 2–12 with `left` and `right` named series, finite nonnegative values; mirrored bars use one common scale and centred category labels. |

Labels remain at 12pt or the frame is refused. Do not compress a full-size
chart into a tiny segment to defeat that guard. Authored one-series legends
or vertical gridlines on bar/line-family charts produce `REDUNDANT_LEGEND`
/ `CHART_VERTICAL_GRIDLINES` advice; intentional style is retained.

A `spectrum` is a supporting band. Pair it with the argument it qualifies in
a composition; recommendation recipes include that companion explicitly.
The standalone semantic kind remains available and still reports sparse use.

## Flow alternatives

Sankey is not supported. For origin/destination shares, use `stacked_bar`
with named series and categories. For net movement from opening to closing,
use a waterfall (`bridge`) with explicitly signed drivers and reconciled
totals. If both questions matter, use two slides; neither alternative claims
to preserve a many-to-many flow topology.

## Native layout and chrome choices

Use `two-column-60-40`, `two-column-40-60`, `two-column-70-30` or
`two-column-30-70`; `sidebar-right` / `sidebar-left` select 70/30 and 30/70.
`stacked`, `stacked-60-40`, `stacked-40-60` split top/bottom while preserving
title/footer chrome. `image-left` / `image-right` derive from native
columns when no dedicated layout exists. Unsupported three/four/five-column
aliases return an actionable alternative; use `regions` or a named pattern.
Content `heading` reserves a styled heading above any native content type.
Agenda aliases `contents`, `outline` and `toc` resolve to an available agenda
host by its tags; a missing host is not advertised.

A semantic deck can set `meta.chrome.sticker: Preliminary` (≤20 characters)
or override it with slide `sticker`. A `section` with `show_chapters: true`
uses a chapter agenda with the current section highlighted. Keep these labels
short enough to clear the title and tracker on both templates.
