# JSON Input Format — Tutorial

`json2pptx` accepts a JSON object describing a presentation and renders it into a `.pptx` file. This page documents the **raw compiled `PresentationInput` format** with worked examples; for the **canonical list of fields, types, enums, and required-vs-optional flags**, query the schema directly.

For new agent-authored decks, prefer the semantic deck spec instead of hand-authoring this raw format. The semantic compiler accepts compact YAML/JSON slide kinds such as `kpi_snapshot`, `chart_insight`, `comparison`, `roadmap`, and `decision`, then compiles them to the raw `PresentationInput` described here:

```bash
json2pptx semantic schema
json2pptx semantic validate --spec deck.yaml
json2pptx semantic compile --spec deck.yaml --output compiled.json
json2pptx semantic render --spec deck.yaml --output deck.pptx
```

Raw `PresentationInput` remains the power-user escape hatch, the compiler output format, and the contract for low-level MCP tools such as `validate_input`, `preview_presentation_plan`, `generate_presentation`, and `repair_slide`.

## Canonical schema (single source of truth)

- MCP: `get_input_schema` — returns the JSON Schema for `PresentationInput`, with `x-field-scope` (`deck` / `slide` / `content` / `shape` / `split`), inline `enum` arrays, and discriminator constraints. Digest-cacheable.
- CLI: `json2pptx input-schema` — same payload, printed to stdout.

Semantic specs have their own schema:

- MCP: `list_deck_archetypes`, `list_slide_kinds`, and `validate_deck_spec` expose the supported semantic contract.
- CLI: `json2pptx semantic schema` — prints the semantic JSON Schema.

The raw schema is generated from the Go input structs in `internal/deckinput` (aliased through `cmd/json2pptx/json_schema.go` for CLI compatibility). This tutorial only describes shapes and gives examples; the schema is authoritative on field names, types, and required-vs-optional.

## Minimum complete deck

```json
{
  "template": "warm-coral",
  "output_filename": "review.pptx",
  "slides": [
    {
      "layout_id": "title",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Q1 Review"}
      ]
    }
  ]
}
```

A presentation is a top-level object with a `template` and a non-empty `slides` array. Each slide pins a layout via `layout_id` (canonical id, e.g. `title`, `content`, `two-column`, `section`, `blank`) or hints `slide_type` (`title`, `content`, `chart`, `section`, `two-column`, `diagram`, `image`, `comparison`, `blank`). At least one of the two must be set. `template` is a registered template NAME (a `.pptx` in the templates directory or the embedded set, without the extension) and never takes a path; `template_path` is the bring-your-own form — `{"template_path": "./client-brand.pptx", "slides": [...]}` — for a `.pptx` the engine has not registered. Set exactly one. On the CLI a relative `template_path` resolves against the deck JSON's own directory; over MCP against the call's `base_dir` (server CWD when omitted), and it must stay inside it after `~`/`$VAR` and symlink expansion — an escape is refused with `INVALID_PATH`. `examine_template`, `list_templates`, `validate_input`, `preview_presentation_plan`, `generate_presentation` and `render_deck_spec` all accept it; `get_started(task:"onboard-template")` walks the vetting sequence. Copying the file into the templates directory registers it live, no restart, as a normal `template`.
To preview the same input against another registered template without editing the deck, pass `json2pptx validate --template NAME --fit-report` (also works with `--json` / `--json-output`) or the top-level `template` argument to MCP `validate_input`. The override takes precedence over the presentation's `template` or `template_path` for that validation call only.

## A typical content slide

```json
{
  "layout_id": "content",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Highlights"},
    {"placeholder_id": "body",  "type": "bullets",
     "bullets_value": ["Revenue +25%", "NPS at all-time high", "OpEx flat"]}
  ],
  "speaker_notes": "Hit the revenue point first."
}
```

A slide's `content` array is a list of typed items, each targeting a placeholder by its canonical id (`title`, `subtitle`, `body`, `body_2`, `body_3`, `image`, `image_2`). The schema enforces the `type` → typed-value pairing via an `if`/`then` discriminator chain:

- `type: "text"`             → requires `text_value` (string)
- `type: "bullets"`          → requires `bullets_value` (string array)
- `type: "body_and_bullets"` → requires `body_and_bullets_value`
- `type: "body_and_lead"`    → requires `body_and_lead_value`
- `type: "bullet_groups"`    → requires `bullet_groups_value`
- `type: "table"`            → requires `table_value`
- `type: "chart"`            → requires `chart_value`
- `type: "diagram"`          → requires `diagram_value`
- `type: "image"`            → requires `image_value`; native `fit: "cover"` (default) crops edges, while `fit: "contain"` preserves the whole screenshot or evidence diagram without distortion. Legacy `value` image objects also accept `fit`. Check text/image contrast separately; see [native image guidance](../skills/template-deck/TEMPLATE_GUIDE.md#image).

A `table_value`'s `headers` define its column count. Each row may use `col_span` / `row_span` on a cell (`{"content":"Total","col_span":2}`); a row that covers fewer columns than `headers` is padded with empty cells (a column still covered by a `row_span` from above keeps its merge), and a row that covers more columns is rejected by `validate` and `generate` alike (`INVALID_PARAMETER`, path `.../table_value.rows`). Other `*_value` fields are forbidden for the chosen type. The legacy raw `value` field is still accepted (unconstrained) for backward compatibility. `table_value`, `chart_value`, `diagram_value` and a `shape_grid` cell's `diagram` / `table` each take an optional `alt` — one sentence a screen reader announces, written into the shape's `cNvPr/@descr`; without it the engine derives one from the payload and reports `MISSING_ALT_TEXT` (advisory, see [FIT_FINDINGS.md](FIT_FINDINGS.md#missing_alt_text)).

Text content and source attribution can carry clickable links. Set `"link": {"url": "https://example.com"}` on a text or bullets content item to link its text runs (every bullet in a bullets item), or set `"source": "Annual report", "source_link": {"url": "https://example.com/report"}` on the slide. A slide `source` renders once in the source zone just above the footer (9pt italic, muted text colour, on the content's left edge); `chart-insights-split` / `stat-hero` `values.source` is lifted into it, and a data slide without any source draws the advisory `DATA_WITHOUT_SOURCE`. URLs must be absolute HTTP/HTTPS URLs. A link object sets exactly one of `url` and `slide`.

## Section slides and the title-at-bottom convention

Section dividers (`layout_id: "section"` / `slide_type: "section"`) target a template's Section Divider layout. A slide counts as a section divider for auto-numbering when it declares `slide_type: "section"`, when its `layout_id` is `"section"`, or when the layout it names is the template's Section Divider (canonical type, or tagged `section-header`) — so a deck that reaches the divider through a template-specific layout ID still gets its sections numbered. In some templates the **visual order of the title and the decorative number is reversed** from a normal content slide: a large decorative "section number" frame sits at the top of the slide and the section *title* is placed in a smaller slot near the **bottom**. You still author it the same way — write the section title into the `title` placeholder — but the rendered title will appear lower on the slide than you might expect, and its capacity is small.

These layouts carry the `title-at-bottom` classification tag, and when the bottom title slot only fits a short single-line title they additionally carry `compact-title` (see the layout-tag tables in the template guide). When you see either tag, keep the section title to a short, single line (the reported per-layout `max_chars` on a title is the measured capacity of the box, which is not the same signal). Long, multi-line section titles overflow the slot. The decorative number is supplied by the template, not by your JSON.

## Inline formatting

Text and bullet strings accept five inline tags: `<b>`, `<i>`, `<u>`, `<sup>`, `<sub>` (the live list is `get_capabilities().features.supports_inline_markup`). They can be nested: `<b><i>bold italic</i></b>`. Plain dashes/arrows (`→`, `•`, en/em dashes) are allowed; **emoji codepoints are rejected anywhere** in deck JSON.

## A custom visual: `shape_grid`

For slides where placeholders aren't expressive enough, use `shape_grid` on a `blank` layout:

```json
{
  "layout_id": "blank",
  "content": [
    {"placeholder_id": "title", "type": "text", "text_value": "Strategic Pillars"}
  ],
  "shape_grid": {
    "columns": 3,
    "gap": 4,
    "rows": [
      {"cells": [
        {"shape": {"geometry": "roundRect", "fill": "accent1",
          "text": {"content": "Innovation\nR&D + emerging tech",
                   "size": 12, "color": "lt1", "vertical_align": "ctr"}}},
        {"shape": {"geometry": "roundRect", "fill": "accent2",
          "text": {"content": "Growth\nNew markets",
                   "size": 12, "color": "lt1", "vertical_align": "ctr"}}},
        {"shape": {"geometry": "roundRect", "fill": "accent3",
          "text": {"content": "Efficiency\nAutomation",
                   "size": 12, "color": "lt1", "vertical_align": "ctr"}}}
      ]}
    ]
  }
}
```

Each cell holds exactly one of: `shape`, `table`, `icon`, `image`, `diagram`, or `composite`. A raster `image` cell without an explicit `fit` fills its whole cell and is centre-cropped to it ("cover", via `a:srcRect`) — never stretched; with `fit` (`contain` / `fit-width` / `fit-height`) it keeps the square frame and is cover-cropped into it; SVG images keep the square frame. An image cell's optional `geometry` (`"rect"` default, or `"ellipse"`) sets the picture frame's preset shape: `{"fit": "contain", "image": {"path": "jane.jpg", "geometry": "ellipse"}}` is a circular, cover-cropped headshot. Cells can span columns/rows via `col_span` / `row_span`; an empty spacer cell (`{}` or `{"col_span": 2}`) claims its span too. A numeric `columns` must be a whole number from 1 to 24, and a `columns` array may hold at most 24 widths (`INVALID_GRID` otherwise; the bound is `shapegrid.MaxColumns`). A `columns` array's widths and each row's `height` / `flex` / `min_height` / `max_height` must be finite and >= 0, and the widths must not all be 0; `validate` rejects anything else. A cell's `max_height` (points) caps its rendered height and centers it in its row. Compose (horizontal and vertical) places each segment in a nested grid so it keeps its own column widths, gaps, row heights (including `min_height` / `max_height`) and alignment; a vertical segment gets a full-width row sized by its `size_pct`. Slide-level `overlays` can float arrows, lines, and badges over the grid; anchor them to cells by `(row, col, at)` or by percent-of-slide coordinates. Grid-level `vertical_align` (`"stretch"` default, `"top"`, `"center"`, `"bottom"`) places a block of `max_height`-capped rows inside the bounds instead of stretching it; named patterns expand with `"center"`. When rows over-commit the grid, a row held at its `min_height` keeps it and the other rows give way. **Text margin:** every shape's text sits 0.5 cm (~14.17pt) from the shape edge on all four sides, on every template — it is the engine default and needs no field. A shape's `text` object may override a side with `inset_left` / `inset_right` / `inset_top` / `inset_bottom` (points, `>= 0`); each authored side replaces only that side, the others keep 0.5 cm. Named patterns never override it. A shape too small for one line of its text plus the margin on an axis (a pill, a badge, a thin band) has that axis's margin shrunk to what still leaves one line, never below zero, rather than shrinking the text. Budget rows for the margin: a one-line 12pt row needs about 43pt, two lines about 57pt.

To make a grid shape or a free-floating badge navigate within the deck, set `"link": {"slide": 12}` on its `shape` or `overlays[]` badge object. Slide numbers refer to the final deck, starting at 1. Out-of-range slide targets are rejected. The generated shape is clickable across its full area.

## Named patterns (prefer over hand-built grids)

For common business slide shapes — KPI cards, process flows, BMC canvas, matrix-2x2, roadmap, strategy house, SCQA summary, agenda, comparison, pull-quote — use a named pattern instead of hand-authoring a `shape_grid`. Patterns are registered in `internal/patterns/` and discoverable via:

- MCP: `list_patterns`, `show_pattern`, `expand_pattern`, `recommend_visual`
- CLI: `json2pptx patterns list`

Pattern field shapes and overrides are documented in `docs/PATTERNS.md`. An expansion can be edited and resubmitted as a `shape_grid`: it carries `source: "pattern:<name>"`, which exempts the engine's own font sizes from constrained mode's absolute-size rule — keep the stamp, or those sizes are refused as if you had written them (raw hex is refused either way).

## Charts and diagrams

```json
{
  "placeholder_id": "body",
  "type": "chart",
  "chart_value": {
    "type": "bar_chart",
    "title": "Revenue by Quarter ($M)",
    "data": [
      {"label": "Q1", "value": 12},
      {"label": "Q2", "value": 18}
    ]
  }
}
```

Supported chart types: `bar_chart`, `line_chart`, `pie_chart`, `donut_chart`, `area_chart`, `radar_chart`, `scatter_chart`, `bubble_chart`, `stacked_bar_chart`, `stacked_area_chart`, `grouped_bar_chart`, `waterfall`, `funnel_chart`, `gauge_chart`, `treemap_chart`. Waterfall charts require `data.points` with an explicit `type` on every point (`increase`, `decrease`, `subtotal`, or `total`). A flat label-to-value map is rejected: its numbers do not say whether a bar changes the running balance or shows an absolute balance. For a profit bridge, use `{"points":[{"label":"Revenue","value":21.3,"type":"total"},{"label":"COGS","value":-6.7,"type":"decrease"},{"label":"Gross Profit","value":14.6,"type":"subtotal"}]}`. Pie and donut slice labels show calculated share percentages by default. Explicit `style.value_format.style: percent` formats those shares (40 and 60 become 40% and 60%, without a scale warning); explicit `plain`, `compact`, or `currency` formats raw slice values. `decimals`, `prefix`, and `suffix` apply in either mode. **Percent units.** `style.value_format.input_scale` declares what percent data means: `fraction` (0.25 → 25%, 1.2 → 120%) or `percentage_points` (0.25 → 0.25%, 25 → 25%); ticks and labels share it, and it is rejected with any style other than `percent`. Omitted (or `auto`), values all within [-1, 1] are read as fractions (info `chart.percent_scale_ambiguous`) and larger values as percentage points (warning `chart.percent_scale_ambiguous`). Chart data rules enforced by the renderer: **Waterfall direction and totals.** `type` sets the direction: a `decrease` always lowers the running total and an `increase` raises it, whatever sign `value` is written with (`{"type":"decrease","value":5}` is a −5 step). A `total` / `subtotal` that differs from the running sum of the preceding bars by more than 0.5% is still drawn at its authored value and raises `chart.waterfall_total_mismatch` (review, fix `replace_value`). Label and tick precision is chosen to keep values distinct; a tiny delta never prints as `-0.0`. **Non-negative charts.** `stacked_area_chart`, `funnel_chart` and `radar_chart` reject negative values at validation (use `stacked_bar_chart` for signed stacks, a waterfall for losses, a bar chart for signed scores). `bubble_chart` rejects negative sizes. **Bubble size.** Bubble area is proportional to size on one scale shared by every series: the largest size in the chart draws the largest bubble, equal sizes draw equal bubbles in any series, and a zero size draws a minimum 3pt dot. **Stacked-area labels.** `show_values` labels each band with its own authored value, not the running total the band's edge sits at; series keep the same palette colours as the other Cartesian chart types. **Gantt / timeline dates.** A present but unparseable `start`/`start_date`/`end`/`end_date`, an end before its start, and (gantt) a task with neither `start_date` nor `date` are rejected, naming the field. A `time_unit` that would draw more than 200 axis ticks over the date range is coarsened (day → week → month → quarter → year) and reported as `chart.tick_thinned`. **`data.data_labels`** accepts an object (`format`, `show_on`) or a bool: `true` is shorthand for `{}` (labels on with defaults), `false` leaves them off. **`matrix_2x2`** accepts a reversed axis (`x_min > x_max`, as the BCG preset uses); points are clamped to the axis span, not pinned to one edge. **Bar highlight and labelled charts.** A single-series bar chart paints its bars neutral (dk1 at 38%) and only the highlighted ones in accent1: `chart_value.highlight` (or `data.highlight`) lists 0-based category indices and/or category names; omitted, a time series (years, quarters, months) accents its last bar and any other chart its largest; `[]` accents none; an entry that names no category fails validation. Multi-series charts keep the series palette. When every bar is labelled (`style.show_values`), bar and waterfall charts drop the value axis and gridlines for a 0.75pt dk1 baseline, bars take 60% of their slot and labels are 10pt (bold on highlighted bars) with a true minus sign (U+2212). Waterfalls paint decreases accent1, increases dk1 at 35% and totals / subtotals dk1 at 60%; a waterfall whose axis does not start at zero keeps its axis.

### Legend defaults: direct labels for 2–4 series

Bar / line / grouped-bar / area charts with 2–4 series default to **inline series labels** (at the line endpoint, or above the last bar of each series) in place of a legend — per `tokens.ChartDirectLabelMaxSeries`. Above 4 series the legend reappears because in-plot labels collide. Force the legend back on with `chart_value.style.show_legend: true`. Line labels whose series end close together are stacked apart with a leader back to each line; when even stacked they will not fit, the chart draws a legend and reports `chart.overflow_suppressed`. Stacked variants and non-Cartesian chart types (pie, donut, scatter, radar, waterfall, funnel, gauge, treemap) are unaffected.

### Per-slide chart-style overrides

Add an optional `chart_style` block on a `chart_value` (or `diagram_value`) to flip an executive-default token for one chart; omit it to keep the deck-wide defaults. `show_vertical_gridlines` (default `false`) draws vertical gridlines on bar/line/area charts in addition to the horizontal ones, and `show_single_series_legend` (default `false`) renders the legend even for a single-series chart (normally suppressed because the title carries the label). See the schema for the authoritative field set.

```json
{
  "placeholder_id": "body",
  "type": "diagram",
  "diagram_value": {
    "type": "swot",
    "data": {
      "strengths":     ["Strong brand", "Loyal customers"],
      "weaknesses":    ["High costs"],
      "opportunities": ["Emerging markets"],
      "threats":       ["Competition"]
    }
  }
}
```

Supported diagram types: `timeline`, `process_flow`, `pyramid`, `venn`, `swot`, `org_chart`, `gantt`, `matrix_2x2`, `porters_five_forces`, `house_diagram`, `business_model_canvas`, `value_chain`, `nine_box_talent`, `kpi_dashboard`, `heatmap`, `fishbone`, `pestel`, `panel_layout` (aliases `icon_columns`, `icon_rows`, `stat_cards` expand to `panel_layout` with the matching `layout`, given inside `data`; each `panels[]` entry accepts a native-SVG `icon`). In `layout: "stat_cards"` the NUMBER is the hero: an explicit `value` wins, otherwise a short `body` carrying a digit (`"EUR 184m"`) is drawn at 32pt with `title` as its caption, so `{title, body}` and `{title, value}` render the same card; a prose `body` stays small under the title. A `heatmap`'s `values` rows must all have the same length, and `row_labels` / `col_labels`, when given, must match the row / column counts — `validate` rejects ragged grids and label-count mismatches; a `diverging` `color_scale` whose range spans zero is centred on 0. See `ChartSpec.type` and `DiagramSpec.type` in the schema for the authoritative list.

**Taxonomy framework colours.** `business_model_canvas`, `pestel` and `swot` fill their cells from the deck's own accent rather than rotating through `accent1`–`accent6`. Colour carries no information in these frameworks — the cells are named, laid out in a fixed grid and separated by gaps — so rotating hue was noise that also overrode `accent_strategy`. BMC and PESTEL use one hue, with the BMC Value Proposition a step deeper because the canvas privileges it; SWOT keeps two, matching its one real contrast (Strengths/Opportunities against Weaknesses/Threats). `porters_five_forces` is unaffected: its colour tracks each force's `intensity`, which is information. Set `style.colors` to override — entries apply in cell order and a short list repeats, so `["accent1","accent2","accent3","accent4","accent5","accent6"]` restores the old per-cell rotation and `["accent4"]` recolours the whole framework.

Charts and diagrams render to SVG and embed into the slide.

## Theme override

The deck's visual identity comes from the chosen `template`. Override deck-wide colors or fonts with `theme_override`:

```json
{
  "theme_override": {
    "colors": {"accent1": "#E31837"},
    "title_font": "Georgia",
    "body_font": "Arial"
  }
}
```

Each `colors` value must be six hex digits with an optional leading `#` (`#E31837` or `E31837`). Three-digit shorthand (`#abc`), color names (`navy`) and anything else are rejected by `validate` / `generate` with code `invalid_color` at path `theme_override/colors/<slot>`, because the value is written verbatim into the theme part. Font names are inserted literally (XML-escaped; `$` has no special meaning); a font name containing control characters is rejected with code `INVALID_PARAMETER` at `theme_override/title_font` or `theme_override/body_font`. In `design_mode: "constrained"` (the default), raw hex colors are restricted; switch to `design_mode: "free"` for exploratory/artistic decks.

## Viewing mode and readability

Top-level `viewing_mode`: `"present"` (default, projected: 12pt body, 10pt captions, 20pt titles) or `"read"` (on-screen/print, lower floors). It never changes rendering; text shrunk below its role's floor is reported as `TEXT_BELOW_READABLE_MIN` (see `docs/FIT_FINDINGS.md`). Top-level `type_scale` controls measured growth of sparse `shape_grid` text: `"compact"` (raw default; no growth), `"comfortable"` (up to 70% of usable height, body/caption/KPI caps 14/12/40pt), or `"presentation"` (up to 80%, caps 18/14/48pt). A semantic DeckSpec defaults to `meta.type_scale: "comfortable"`. Pattern-level `overrides.type_scale` wins over the deck policy; a raw `shape_grid.type_scale` or `shape.type_scale` can override it for a grid or cell. Dense text is not enlarged, and authored font sizes are never reduced by this setting. Independently of `type_scale`, sized `shape_grid` text settles onto the type scale 28 / 18 / 14 / 12 / 10pt (11pt is the dense-body step): an off-scale size such as 13, 16 or 20pt renders at the step at or below it (12, 14, 18pt). Display figures (a short run containing a digit at 18pt or more — KPI values, step numerals), text of 28pt and above, and a `design_mode: "free"` deck's own grids keep their sizes; pattern expansions always snap. A two-column slide whose `body` and `body_2` bullet lists both open with a short label line (e.g. "Open-weight models" over longer points) renders that line as a bold column header without a bullet. Each grid paragraph keeps its source role (a display-size figure is a KPI value; display words are not, and a lone index marker such as an axis "1" is a caption) through to its written runs: a written size below that role's floor, or a populated frame with no usable area after insets, refuses publication even with `strict_fit: "off"`, without deleting source text or overwriting an existing destination. Preflight predictions stay advisory. Enlarge the frame or redesign the layout, keeping every required item; character budgets are sizing hints, so validate and inspect the rendered slides.

## Footer, page numbers, and structure

`footer`, `chrome.page_numbers`, and `structure.sections` configure deck chrome and section grouping. See the schema for the full set; agents typically only set `footer.enabled` and `footer.left_text`. When both `footer` and `chrome` are set, enabled `footer.left_text` is retained after any distinct text composed from `chrome`; a page-number-only `chrome` block leaves the legacy line intact. `left_text` always renders on a single line: its box starts at the template's date (`dt`) placeholder and spans the footer (`ftr`) placeholder width (stopping before the slide number); text that is still too wide shrinks from 10.5pt down to 8pt and is then ellipsized (`…`). Content-bearing raw `blank-canvas` slides use `headline`; semantic `DeckSpec` supports flat `slides[]` or chaptered `structure`, plus `meta.required_layouts` for planned canonical-layout coverage. `chrome.tracker: true` sets the current section name above each content slide's title (9pt accent1 caps, +8% letter-spacing, 6pt above the title), from `section_title` or, on a flat deck, the preceding section slide; title, section, closing and agenda slides and slides with an `eyebrow` carry none. A slide's `section_title` is the section name `chrome.section_crumb` appends to its footer; the engine sets it when expanding `structure.sections` and `semantic compile` writes it onto the flat slides it emits, so do not hand-author it. See [SEMANTIC_COMPILER.md](SEMANTIC_COMPILER.md) and `get_input_schema` for the complete contracts.
## Patch input and local asset paths

The patch envelope (`base` + `operations`) and the `~` / `$VAR` expansion rules for local asset paths live in [INPUT_FORMAT_ADVANCED.md](INPUT_FORMAT_ADVANCED.md).

## Validating before generating

- CLI: `json2pptx validate <input.json>` — same validator the engine runs.
- CLI: `json2pptx validate <input.json> -fit-report` — adds layout-fit diagnostics.
- MCP: `validate_input` — wraps both.

Errors carry structured `code` fields catalogued in `docs/FIT_FINDINGS.md` (with severity and recommended action). The validator accepts both canonical and alias enum values for backward compatibility; the schema publishes only the canonical names.

## Where to go next

- `docs/SEMANTIC_COMPILER.md` — semantic deck-spec model, compiler stages, and implementation roadmap.
- `docs/PATTERNS.md` — named-pattern authoring guide.
- `docs/FIT_FINDINGS.md` — finding-code catalogue.
- `docs/STYLE_DEFAULTS.md` — deck-level defaults for table and cell styles.
- `skills/generate-deck/` — agent-facing workflow, rules, and pattern recommendations.
