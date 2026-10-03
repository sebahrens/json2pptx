# Pattern Authoring Guide

How to add a new named pattern to `internal/patterns/`.

## When to add a pattern

A pattern is justified only when its `shape_grid` expansion is reused across **3 or more example decks**. If a layout appears in fewer than three decks, use `shape_grid` directly. This rule prevents pattern proliferation.

## File naming

Follow the existing convention in `internal/patterns/`:

| Pattern name | File |
|---|---|
| `kpi-3up` | `kpi_parametric.go` (parametric) |
| `kpi-4up` | `kpi_parametric.go` (parametric) |
| `bmc-canvas` | `bmccanvas.go` |
| `timeline-horizontal` | `timelinehorizontal.go` |
| `card-grid` | `cardgrid.go` |

Strip hyphens, lowercase. Test file: `<name>_test.go`. If two patterns share helpers (like `kpi-3up` and `kpi-4up`), put shared code in a `_common.go` file (e.g. `kpi_common.go`).

### Parametric patterns (kpi-Nup convention)

When a family of patterns differs only by cell count, use the **parametric adapter** pattern instead of writing individual Go files. The `kpi-Nup` family (N = 2..6) demonstrates this:

- A single `kpi_parametric.go` defines the `kpiNup` struct implementing `Pattern`
- A config struct (`KPINupConfig`) carries the count, density class, and exemplar values
- `kpi_variants.go` registers all variants via a config slice in `init()`

To add `kpi-7up`, append one entry to `kpiVariants` in `kpi_variants.go` — no new file needed. This pattern applies whenever multiple variants share identical `Expand`/`Validate` logic differing only by a numeric parameter.

## Contributor checklist

Every pattern PR must include all of these:

- [ ] **Implementation** (`internal/patterns/<name>.go`)
  - Unexported struct implementing `Pattern` interface
  - `init()` registering via `Default().Register(&myPattern{})`
  - Typed `Values`, `Overrides`, `CellOverride` structs
  - `Schema()` returning a hand-authored JSON Schema (see below)
  - `Validate()` using `errors.Join` aggregation. **Length budgets count characters, not bytes** — use `runeLen(s)` (never `len(s)`) both in the guard and in the `errMaxLength` count, because the budget is a proxy for what fits in a box and a box does not care how a character is encoded. `len("€186.4M")` is 9; the value is 7 characters and fits an 8-character budget (go-slide-creator-5ok4).
  - `Expand()` returning `*jsonschema.ShapeGridInput`
  - `CellsHint()` (part of the core `Pattern` interface)
  - `Taxonomy()` returning `PatternTaxonomy` (see below)
- [ ] **Tests** (`internal/patterns/<name>_test.go`)
  - Metadata: `Name()`, `UseWhen()`, `NotWhen()` non-empty (D6), `Version()`
  - Taxonomy: all fields populated with valid values
  - Schema validity: marshals to valid JSON Schema draft 2020-12
  - Validate: happy path, wrong count with sibling hint (D4), missing required fields, max length exceeded (include one multi-byte value — a euro sign or an umlaut — inside the budget, so a byte count cannot creep back in), invalid cell override keys
  - Expand: default accent, accent override, cell override application
- [ ] **Golden file** (`internal/patterns/testdata/<name>/default.golden.json`)
  - Created by running tests with `UPDATE_GOLDEN=1 go test ./internal/patterns/ -run TestMyPattern/golden`
  - Committed alongside the code
- [ ] **Smoke entry** in `examples/patterns-smoke.json`
  - One slide exercising the pattern with representative values
- [ ] **use_when / not_when text** reviewed (see below)
- [ ] **Taxonomy fields** reviewed (see below)

## Hand-authored Schema convention (D13)

Each pattern's `Schema()` method returns the **authoritative external contract** for agent-facing discovery. Key rules:

- One Schema per pattern, defined in the pattern's `.go` file
- Use the helpers in `schema.go`: `ObjectSchema`, `ArraySchema`, `StringSchema`, `NumberSchema`, `IntegerSchema`, `EnumSchema`, `BooleanSchema`
- Call `.AsRoot()` on the top-level schema (adds `$schema` draft 2020-12)
- Call `.WithDescription(...)` for agent-readable field docs
- Call `.WithAdditionalProperties(false)` on objects to reject unknown keys
- **When the Values struct changes, the Schema must change in the same PR.** The Schema is the discovery surface; the Go `Validate` is the enforcement surface. They must stay in sync.
- Runtime enforcement (`Validate`) may express semantic invariants beyond what JSON Schema can capture (e.g. "exactly N cells", "no overlapping spans"). That's expected.

## cell_overrides scope (D15)

Per-cell overrides are narrowly scoped to text/style/decoration adjustments only:

| Allowed key | Type | Description |
|---|---|---|
| `accent_bar` | bool | Show accent bar decoration. In patterns that draw the bar by default (`horizontal-bar-with-callouts` callout rows) only an explicit `false` removes it; an absent key or any other override keeps it |
| `emphasis` | `"bold"` / `"italic"` / `"bold-italic"` | Text emphasis |
| `align` | `"l"` / `"ctr"` / `"r"` | Horizontal alignment |
| `vertical_align` | `"t"` / `"ctr"` / `"b"` | Vertical alignment |
| `font_size` | number (6-120) | Font size in points |
| `color` | string | Text color (scheme ref, e.g. `"dk1"`) |

**MUST NOT** accept arbitrary nested `shape_grid` fragments or geometry changes. Cells are addressed by zero-based index as string keys (`"0"`, `"1"`, ...). The pattern's `Validate` must reject unknown override keys with an error citing the D15 whitelist.

**Every advertised key must change the output.** A pattern whose schema uses `CellOverrideDefSchema()` must honour all six keys. The five text keys go through the shared `applyCellTextOverride(cell, ovr)` helper (`cell_override.go`), which rewrites the target cell's primary text — a shape's `text`, a composite cell's text shape, or an image cell's overlay label:

- `font_size`, `emphasis`, `color` apply to **every** paragraph of that text (a header/body cell loses its size hierarchy under `font_size`). `emphasis: "bold"` clears italic and `"italic"` clears bold; `"bold-italic"` sets both.
- `align` sets the text default **and** each paragraph's own `align` (a paragraph align wins over the default, so the default alone would be a no-op).
- `vertical_align` sets the anchor. The sparse-card re-centring pass (`anchorSparseText`) keeps an explicit `"b"`.

Call the helper where the cell is built, before any content-sized row measurement, so a larger `font_size` grows the row. If a pattern truly cannot honour a key, give it a narrowed cellOverride schema and make `Validate` reject that key as `unknown_key` — never accept and ignore it. `TestCellOverrideKeys_ChangeExpandOrAreRejected` (`cell_override_text_test.go`) enforces this for every registered pattern: each advertised key must change the `Expand` output (and the text values must appear in it), and each unadvertised D15 key must be rejected.

Where an index addresses a composite of several shapes, the text keys land on the primary text shape; the accent bar keeps its existing placement:

| Pattern | Index → text target |
|---|---|
| `agenda`, `agenda-with-images` | item → title cell (not the number badge) |
| `dual-org-ladder` | `0` → both org headers; `i+1` → both role cards of row `i` |
| `exec-summary` | point → bold lead-in cell (not the support sentence) |
| `hero-detail` | `0` → hero stat (now also takes `accent_bar`); `i+1` → detail card `i` |
| `horizontal-bar-with-callouts` | bar → its callout; the bar label when the row has no callout |
| `journey-maturity-model` | stage → stage header |
| `labeled-rows` | row → label block (label + sublabel) |
| `metric-list` | item → big value (not label/detail) |
| `stylish-panels` | panel → body (not the ribbon header) |
| `table-highlight` | option → option-name cell (not the score cells) |
| `team-bios` | member → name/role/bio text cell (not the photo) |
| `timeline-horizontal` | stop → label cell (`dots`), chevron (`chevron`), bar (`gantt`) |
| `value-chain`, `waterfall-bridge` | step / column → label cell |

All other patterns apply the text keys to the one shape the index addresses (banner / pillar / foundation / roof in `strategy-house`, a quadrant in `matrix-2x2`, a card in `card-grid`, and so on). The `kpi-*` family already did this through the same helper (formerly `applyKPICellTextOverrides`).

## card-grid styles + surface overrides

`card-grid` (`cardgrid.go`) exposes two complementary knobs through pattern-level `overrides` (distinct from the per-cell `cell_overrides` whitelist above):

- `style` — visual treatment enum: `filled` (default, solid accent + light text; several peer cards become the neutral surface + accent rule), `accent-stripe`, `numbered-badge`, `icon-card` (neutral 4% cards, never white on white), `tinted` (alternating template surface / neutral steps, no outline), `soft-card` (single neutral surface, dark text, explicit no-border line).
- Generic surface overrides that apply on top of **any** style:

| Override | Type | Effect |
|---|---|---|
| `card_fill` | string (hex or scheme name) | Repaints every card's fill. A raw hex (e.g. `#FFF5ED`) is accepted only when the caller supplies it; the engine never hardcodes a brand surface. Constrained `design_mode` rejects raw hex — pass a scheme color there. |
| `line_color` | string (hex or scheme name) | Explicit card border color. Takes precedence over `border`. |
| `line_width` | number (0–12 pt) | Card border width; defaults to 1 pt when `line_color` is set without a width. |
| `border` | `none` / `subtle` / `accent` | Keyword border: `none` emits an explicit no-fill line (suppresses theme default), `subtle` is a thin dk1 hairline, `accent` is a 1 pt accent-colored border. Ignored when `line_color`/`line_width` are set. |

Validation rejects unknown colors (non-hex, non-scheme), `line_width` outside 0–12, and unknown `border`/`style` keywords. A `subtle` / `accent` border or a `line_color` / `line_width` outlines filled cards, so it is honoured but reported as the advisory `FILLED_SHAPE_OUTLINED`. `soft-card` plus `card_fill` is the canonical recipe for a no-border pale brand surface that keeps dark, contrast-safe text.

## Writing use_when / not_when text (D6, wobw contract)

The `UseWhen()` and `NotWhen()` strings together form an **anti-misuse guardrail**. They tell agents (and humans) when this pattern is — and is not — the right choice.

### UseWhen() — contrastive guidance

- Be prescriptive: state when to use, including the data shape expected
- **Contrastive**: explicitly name sibling patterns that would be better in adjacent scenarios
- Keep it to one sentence
- Example: `"Exactly 3 big-number KPIs with short captions; prefer stat-hero for a single dominant metric, card-grid when items need multi-line body text"`

### NotWhen() — explicit anti-patterns

- State the scenarios where this pattern is wrong, each pointing to the correct alternative
- Use semicolons or commas to separate scenarios
- Example: `"Items need multi-line descriptions (use card-grid), a single metric should dominate (use stat-hero), or items are not numeric KPIs (use icon-row)"`

The pair is symmetrical: `UseWhen` says "choose me when X", `NotWhen` says "do NOT choose me when Y — use Z instead."

### Choosing between similar patterns

| Intent | Pattern | Disambiguator |
|---|---|---|
| Big-number KPIs (2–6 items) | `kpi-Nup` | Fixed count, ≤12-char metrics (measured one-line fit warnings); optional per-cell `sub` (delta/trend annotation, aliases `delta`/`trend`/`change`) |
| Ranked horizontal bars (3–8) with per-bar insight | `horizontal-bar-with-callouts` | One callout per bar, accent-bar bound to the row; omit every `callout` and the column is dropped so the bars span the full width; bars are neutral dk1 at 38% with accent1 only on `values.highlight` (0-based indices or labels; default the top bar, `[]` none) |
| Big-number KPIs (2–6 items) | `kpi-Nup` | Fixed count, ≤12-char metrics (measured one-line fit warnings); optional per-cell `sub` (delta/trend annotation, aliases `delta`/`trend`/`change`) and `comparator` (≤24 chars, alias `vs`: the reference the number is read against, "vs plan +4 pts"), rendered as its own line in the delta's size and ink and reserved on every card when any card has one. `kpi-inline` (height-capped) rejects `comparator` and defaults to neutral cells under a thin accent rule (`overrides.style: "solid"` restores accent blocks) |
| Ranked horizontal bars (3–8) with per-bar insight | `horizontal-bar-with-callouts` | One callout per bar, accent-bar bound to the row; omit every `callout` and the column is dropped so the bars span the full width |
| Single dominant metric | `stat-hero` | One hero number with context; `unit` is a trailing run at 40% of the value size on its baseline (shape-grid paragraph `suffix` / `suffix_size`) |
| Stat stack / "by the numbers" list | `metric-list` | 3–7 rows read top to bottom: big right-aligned accent `value` (≤12 chars, one shared size shrunk until the longest fits on one line) + bold `label` + optional `detail`, hairline rules; at most one `highlight: true` row (Lighter-80% accent band + accent bar, value ink measured against the band) and an optional `callout` rendered as the takeaway band (see [The takeaway component](#the-takeaway-component)). Details hold 120 chars through 4 items, ~90 at 5, none at 6–7. Use `kpi-Nup` for side-by-side cards, `stat-hero` / `hero-detail` for one dominant number |
| Agenda / contents page | `agenda` | 2–10 rows: a 28pt serif (`+mj-lt`) accent numeral beside a 14pt item, 0.5pt rules between content-height rows, no filled tiles, block middle-anchored (the numeral steps to 18pt only when the rows do not fit). `overrides.highlight` marks the current section on a repeated agenda: that row bold dk1, every other row at 50% opacity (stepped up just enough to keep 4.5:1 / 3:1 on lt1). Optional `values.subtitles` (parallel to `items`, ≤120 chars each) set a muted line 4pt smaller (≥12pt) under each title; it dims with its row. A pale accent falls back to dk2 / dk1 numerals by measured contrast |
| Closing next steps | `next-steps` | 2–6 numbered action rows (`action` ≤90, `owner` ≤30, `date` ≤20; the owner / date column drops when no action has one) under a quiet column header, 0.5pt rules, plus 0–3 `decisions` (≤120) in a "Decisions requested" band (`decisions_label` overrides) drawn with a 3pt left accent rule, bold text, no outline and no fill. The closer instead of "Thank you"; use `numbered-step-strip` for a process to explain, `phase-roadmap` for a dated schedule |
| Feature/capability cards | `card-grid` | Multi-line body text per card |
| Sequential process | `process-flow` | Ordered steps with accent arrows; steps are a neutral tint with dark text and the solid accent fills one step only — `steps[].highlight` (at most one), else the flow's single `decision` (further decisions get an accent outline); `overrides.style: "solid"` restores all-accent steps (and `cell_accent_mode`). Steps are content-sized (written fit, floored at 0.4× the step width, capped at 30% of the content height; a diamond is measured in its inner half-size text rectangle); `process-flow-compact` is shallower (0.28×, capped at 22%) and top-anchored under the title |
| Ordered steps / annotated ToC (no branching) | `numbered-step-strip` | 3–7 numbered steps (chevron ≤6) with an optional per-step detail zone and, in `stacked-box` / `toc`, an optional `steps[].icon` between the number badge and the label; `chevron` ribbon, `stacked-box` scorecard, or `toc` agenda — never emits decision diamonds (use `process-flow` for branching). Stacked-box numbers sit on a neutral lane and toc numbers are unfilled accent numerals; `overrides.style: "solid"` (not `values.style`) restores accent lanes / badges. Stacked-box / toc rows are pinned at their written fit against the zone left after chrome bands (see the written-fit rule) |
| Two parallel process tracks | `process-grid-2row` | Two rows × 3–6 phase columns sharing the same N columns; dk2 row-label column on the left, phase boxes a neutral tint (the second track a lighter step) under a thin rule in the track colour (`row1_color` / `row2_color`; the first track drops its rule under `column_headers`), `overrides.style: "solid"` restores per-row accent fills; optional `column_headers` (≤24 chars, accent-underlined header row) and `outcomes` (≤32 chars, tinted pills under the tracks), each one per phase column |
| Porter / supply value chain | `value-chain` | 4–10 step columns with bold label + 1–3 line description |
| Maturity ladder / current-state journey | `journey-maturity-model` | 3–6 stage columns with numbered headers, description, and optional 'where we are' marker (the current header is the one solid accent; the marker is an accent-outlined callout and the other marker cells are unfilled) |
| P&L walk / cost-driver bridge | `waterfall-bridge` | 3–10 columns of total + delta + subtotal bars; floating deltas with auto-computed subtotals, grey bridge lines between bar levels; `unit` currency symbols render as a prefix (`"$m"` → `$210m`), value labels sit just outside each bar (above rises, below falls), headroom kept under the title; optional `caption` states the scale once, since a bridge draws no value axis; decreases take the accent (accent1), increases dk1 at 35%, totals / subtotals dk1 at 60%, and only the accent labels are bold |
| Value / cost driver tree | `driver-tree` | Root metric → 2–4 branches → 1–4 leaves each, with optional per-branch annotations (for **people/role** hierarchies use svggen `org_chart` instead) |
| Temporal sequence | `timeline-horizontal` | Date-labeled stops |
| Layer/stack diagram | `arch-stack` | Vertical tier ordering; tiers are content-sized (the tallest tier's written fit + 8pt, capped at 20% of the content height unless the text needs more) and the stack is centred |
| Narrowing hierarchy | `pyramid` | Visual narrowing (top < bottom) |
| Before/after comparison | `before-after` | Temporal transformation |
| Option/pros-cons comparison | `comparison-2col` | Non-temporal side-by-side; rows are content-sized (the header band at its written fit, body rows grown towards half the content area by at most 1.4× their need, block centred); `overrides.connectors: true` adds a centre gutter with a per-row accent connector badge (left cells get an accent stripe, right cells an accent tint) for "from → to" shifts |
| Today vs. future state in numbered stages | `state-shift-hub` | Central accent hub circle (`hub_label`, shrinks to fit, floor 12pt) with 3–4 numbered `pairs` (`before` / `after` + optional shared `title` or per-side `before_title` / `after_title`); today items right-aligned on the left, future items left-aligned on the right, nodes on an arc around the hub; optional `left_header` / `right_header`. Use `before-after` for one before/after block, `journey-maturity-model` for a maturity ladder |
| Options × criteria evaluation | `table-highlight` | 2–6 options × 2–6 criteria rated with Harvey balls (0–4), RAG (`red`/`amber`/`green`) or ≤24-char text per column (`scale` or per-criterion `{label, scale}`); `highlight_row` tints the recommended option (accent 10%) with a 3pt accent bar, `highlight_col` tints the decisive criterion (its header stays unfilled, set in the accent when it reads); the table follows the engine table default (go-slide-creator-1iiej) — unfilled 11pt bold `dk1` header over a 1pt `dk1` rule, 12pt rows, 0.5pt `dk1`-15% hairlines, no zebra, no cell gutters, content-height rows top-anchored under the title (`header_size` accepts 11–28pt); one legend row per symbol scale. **`legend_labels` is `[HIGH, MID, LOW]`** — the full Harvey ball first, the empty one last — and the RAG scale takes its own `legend_labels_rag` in the same order (`[green, amber, red]`, default `["Green", "Amber", "Red"]`). A deck mixing both scales used to reuse the Harvey words for the RAG swatches, so a green dot carried the "does not meet" label (go-slide-creator-z0up). When any Harvey cell scores 1 or 3 the legend lists all five balls: the three `legend_labels` stay on the full / half / empty balls and the quarter / three-quarter balls read "Mostly meets" / "Slightly meets" with the default labels, or stay unlabelled with custom ones (go-slide-creator-csclk.100). RAG uses conventional status colours (`overrides.rag_colors` swaps them) — the one non-theme palette, because status must read the same on every template |
| Activities rated by function (capability / automation heatmap) | `capability-heatmap` | 3–8 function columns, each a pointed `homePlate` header (bold title + optional sublabel, `header_shape: "rect"` for flat headers) over 1–6 activity cells filled by `tier` (0 = accent, 1 = light accent tint, 2 = neutral grey, 3 = lightest grey with a hairline), plus a legend of 2–4 tier swatches with label + optional description (`show_legend: false` hides it). Colour carries the rating, so there is no `cell_accent_mode`. `header_size` / `cell_size` accept 12–40pt; values outside are rejected rather than silently clamped. Prefer `table-highlight` when every row is scored against the same criteria |
| Levers grouped by dimension (framework) | `framework-grid` | 2–6 rows, each a bold label on a neutral band followed by 1–4 cards (accent title + short body on a neutral surface); the longest row sets the column count and shorter rows leave trailing space empty. `label_width_pct` (10–35, default 18), `title_size` / `body_size` (12–40pt; values outside are rejected), `cell_accent_mode` (per card column). Prefer `card-grid` when there are no row labels, `stylish-panels` for pillars with bullet lists |
| 4-quadrant positioning | `matrix-2x2` | Axis-labeled quadrants; horizontal title ≤16 chars, vertical title ≤60; each axis is an arrow pointing to its high end (right / up) flanked by low/high end labels — optional `x_low` / `x_high` / `y_low` / `y_high` (≤11 chars, default `Low` / `High`) |
| Phased plan with workstreams | `roadmap-phased` | Named phases × workstreams grid; activity cells are a neutral tint with dark text and each phase header carries an accent rule (`overrides.style: "solid"` restores the all-accent grid; emphasise one activity with `cell_overrides` `accent_bar`) |
| Single-track phased roadmap | `phase-roadmap` | Phases + timeline bar + dates + per-phase description (+ milestones: a small accent diamond beside a one-line bold label, directly under the timeline rule; dates are left-aligned like the descriptions); optional `parallel_tracks` (0–4 cross-cutting workstreams, ≤90 chars) render as full-width tinted bars below the phases with a `parallel_label` (default "In parallel") at left, their height taken from the phase-box row |
| Cross-functional swimlanes | `swimlane` | 2–6 actor lanes × 2–8 step columns; steps are read column by column (top lane first within a column) and consecutive steps are joined by accent arrows — straight down for a hand-off within one column, an elbow through the column gutter for a lane change into the next column. Empty positions (`""`) are unpainted spacers that arrows skip. Put one step per column for a strict sequence |
| Executive summary (problem framing) | `scqa-summary` | 4-row Situation/Complication/Questions/Answer narrative arc |
| Executive summary (key messages) | `exec-summary` | 3–5 bold lead-in conclusions (≤90 chars) each with one supporting sentence (≤200 chars) — lead column 45% of the width, and with no supports at all the lead spans the width; every row shares one lead and one support size (the cells opt out of deck type-scale growth), and with a template the overflow warning is the measured fit rather than the character averages — rules between rows, optional `bottom_line` ask (the takeaway band: flush accent bar + bold dk1 statement, no box — see [The takeaway component](#the-takeaway-component)); answer-first rather than an SCQA arc |
| Keyword-labelled rows (WHY / WHAT / HOW) | `labeled-rows` | 2–6 rows: a label block on the left (`label_style: tinted`, the default: neutral-tint block under a thin accent rule with a bold accent keyword + optional smaller `sublabel`; `filled` = solid accent block; `text` = accent-coloured bold keyword with no fill) beside 1–4 lines of `body` (**bold** allowed), rules between content-sized rows. The keyword shrinks as one shared size until no word breaks mid-word. Bodies hold 300 chars through 4 rows, ~190 at 5–6 rows (and 5–6 rows leave no room for multi-line sublabels). Use `exec-summary` when rows are sentence-length conclusions, `metric-list` when each row leads with a number |
| Deck section list | `agenda` | Numbered section outline |
| Visual deck preview | `agenda-with-images` | Numbered agenda rows (unfilled bold accent numerals; `overrides.style: "solid"` restores accent number squares) with image/quote placeholders alongside the title (3–6 items); the placeholder column is all-or-nothing — a row with no `image_label` still gets an empty placeholder |
| Team / 'Our People' page | `team-bios` | 1–8 named people with a headshot (or initials placeholder) + role + short bio, up to 4 per row. The headshot is the same people primitive `contact-directory` draws — a photo cropped into a circle, else a circular accent-tint disc with bold initials scaled to the disc — and name / role / bio centre under it |
| Key contacts / directory | `contact-directory` | 1–4 groups (regions, offices, practices), each an accent heading over a rule, then up to 24 people in rows of `columns` (3–5, default 4): circular headshot (`photo`, drawn with image `geometry: "ellipse"`) or initials disc + bold name + muted title — no bios (use `team-bios` for those). One or two rows of people stack a large headshot above a centred name; denser directories put the headshot left of the text and step type (14→12pt) and headshot size down until the measured block fits — and, without an explicit `columns`, drop to fewer people per row (down to 3) — else report `BODY_TOO_LONG` with the height needed, or naming the person whose name / title word would break mid-word. An area too narrow for even the minimum headshot is an expand error, not a panic |
| Joint-venture / engagement-team paired roles | `dual-org-ladder` | Two parallel columns of 2–4 paired role cards with an org-name header above each column (optional connector line per row) |
| Icon + caption row | `icon-row` | Visual categories, 3–5 items |
| Photo / case study beside text | `image-text-split` | One `image` (`path` resolved against the deck dir, or `url`) beside eyebrow + heading + body + ≤5 bullets and 0–3 result `metrics`; `image_side` left/right, `image_width_pct` 30–60. Without an image it draws a dashed placeholder (`image_label`). Implements `ImageAssetPattern` so hosts resolve its image like a shape_grid image cell |
| Callout / testimonial | `pull-quote` | Attributed quotation, optionally beside a headshot. The quote and attribution hang flush off the accent rule (left-aligned beside a `left` rule, right-aligned beside a `right` one) and are centred only with `accent_side: "none"` |
| Narrative intro / foreword | `text-sidebar` | Main column (optional heading, 1–4 paragraphs, 0–6 bullets placed after the first paragraph ending in a colon) beside a sidebar panel with one large bold key message (≤200 chars). `sidebar_style` `tinted` (pale accent surface + top accent bar) or `filled` (solid accent); the sidebar ink is measured against the fill at the 3:1 large-text bar. `sidebar_side`, `sidebar_width_pct` (25–40), `body_size`, `sidebar_size`. Short copy is centred; body copy sets at 14pt and steps down to 12pt, then reports `BODY_TOO_LONG` |
| Stakeholder quote cluster | `quote-cluster` | 3–8 attributed quote bubbles in a 3-column grid (voice-of-customer slides) |

### Refined-consulting bias in the recommender (J2P-STYLE-008)

The keyword scorer in `internal/patterns/recommend.go` (shared by `recommend_pattern` and
`recommend_visual`) intentionally biases the refined consulting families above the generic
`card-grid` fallback. `card-grid`'s broadest rule scores `0.80`; each refined family below
carries a secondary rule at `baseScore: 0.82` so that even generic "cards"/"grid"/"ranking"
wording routes to the polished layout rather than a tile grid:

| Refined family | Generic-intent signal it now wins | Secondary `baseScore` |
|---|---|---|
| `horizontal-bar-with-callouts` | weighted scorecard, ranked vendors/options/drivers, rating | `0.82` |
| `driver-tree` | value/cost driver, decomposition, breakdown, contributors | `0.82` |
| `strategy-house` | strategy/governance pillars over a foundation | `0.82` |
| `journey-maturity-model` | capability/digital maturity, staged progression | `0.82` |
| `phase-roadmap` | described phase plan with dates | `0.82` |
| `value-chain` | operational sequence with per-step descriptions | `0.82` |
| `stylish-panels` | pillar / capability bullet blocks | `0.82` |

When adding or tuning a refined family, keep its fallback rule at or above `0.82` so the bias
holds; the regression is locked by `TestRecommend_RefinedConsultingBias` in
`recommend_test.go`. `card-grid` is reserved for genuinely flat catalog content (titled tiles
with no ranking, decomposition, sequence, or hierarchy).

The scorer also recognizes risk/mitigation pairs, strategic priorities, and ordered process steps explicitly. If no intent rule clears the threshold, `recommend_pattern` returns one low-confidence, item-count/density-based starting layout instead of an empty list, and asks for clarification. This generic fallback is not a semantic match and is ignored by automatic deck planning; an explicitly unsupported visual such as Sankey still returns `unsupported_visual` with no pattern candidate.

## Taxonomy fields

Every pattern must implement `Taxonomy() PatternTaxonomy` returning classification metadata used by `recommend_pattern` and `analyze_deck_rhythm`:

```go
type PatternTaxonomy struct {
    Category      string   // "data-display", "narrative", "structural", "hero"
    NarrativeRole []string // "open", "frame", "evidence", "compare", "conclude"
    PairsWith     []string // sibling pattern names that flow well as the next slide
    ComposesWith  []string // sibling pattern names that can coexist on the SAME slide via a compose envelope
    RoleOnSlide   []string // slot(s) inside a compose envelope: "banner", "pillars", "foundation", "roof", "callout"
    DensityClass  string   // "low", "medium", "high"
    AccentWeight  string   // "subtle", "normal", "strong"
}
```

Guidelines:

- **Category**: group by primary function — data patterns show metrics, narrative patterns tell stories, structural patterns frame methodology, hero patterns emphasize one thing
- **NarrativeRole**: where in a deck arc this pattern fits. A pattern can serve multiple roles (e.g. `kpi-3up` is "evidence", `agenda` is "open" + "frame")
- **PairsWith**: 2–4 sibling patterns that create good rhythm when sequenced **after** this one (next-slide adjacency). Used by `recommend_pattern` diversity scoring and to rank `analyze_deck_rhythm` break suggestions; run detection groups visual families independently.
- **ComposesWith**: sibling patterns that can **share a slide** with this one through a compose envelope (D18). Distinct from `PairsWith`, which is purely about next-slide sequencing. Populate when the pattern naturally combines (e.g. `stylish-panels` + `pull-quote` for a pillars+callout layout). Leave empty for patterns that should always occupy the whole slide.
- **RoleOnSlide**: which slot(s) this pattern occupies in a compose envelope. Patterns can fill more than one role (e.g. `kpi-3up` works as either `banner` or `foundation`). Leave empty for patterns not intended for compose-envelope use.
- **DensityClass**: visual density — affects rhythm analysis and variety recommendations
- **AccentWeight**: how much accent color this pattern uses — "strong" patterns (KPIs, stat-hero) need breathing room before/after

## Restrained accent defaults (authoring contract)

A solid accent fill is the slide's emphasis, so a pattern spends it on at most one block by default (go-slide-creator-fl11f). Structural cells — roadmap activities, process steps, KPI tiles, detail cards, panel bodies — take a neutral tint (`neutralFillJSON`, `surfaceFillJSON`) with dark text, and the accent marks structure with rules, connectors and small markers. Keep a legacy all-accent look behind an explicit opt-in (`overrides.style: "solid"` on `roadmap-phased`, `process-flow[-compact]` and `kpi-inline`; `overrides.ribbon: "accent"` on `stylish-panels`, whose ribbons default to the structural dark tone; `overrides.style: "cards"` on `hero-detail`, which defaults to `minimal`; `overrides.style: "solid"` on `agenda-with-images`, `numbered-step-strip` (stacked-box lanes, toc badges) and `process-grid-2row`; `label_style: "filled"` on `labeled-rows`, which defaults to `tinted`). Where the fill is the data (heatmap tiers, waterfall totals) it is exempt. `cmd/json2pptx/accent_restraint_gallery_test.go` expands every exemplar and fails when a pattern renders more than one solid-accent block of 0.5in or more; its pending list is empty and must stay so.

The default accent itself is the template's primary fill (go-slide-creator-2mia4): `ExpandContext.ResolveAccent` with the primary (or unset) strategy, and every pattern default that used to hardcode `accent1`, resolves through `ExpandContext.DefaultAccent()`, which returns the first slot of `PrimaryFillCandidates` — the same list `list_templates` reports as `color_roles.primary_fill` (accents passing 4.5:1 against white, then 3:1, then `dk2` / `dk1`), and `accent1` when the theme is unknown. Templates whose accent1 carries white text are unchanged; `warm-coral` defaults to `accent2`, `business-template` to `accent3` and `blue-corporate` to `dk2`. Never hardcode `"accent1"` as a pattern default; call `ctx.DefaultAccent()` (schema `WithDefault("accent1")` stays as the documented common case).

## Cell Accent Variety (authoring contract)

Grid-shaped patterns — those that emit multiple peer cells through the shape grid engine — must support `cell_accent_mode` in their overrides. The contract:

### Grid-shaped patterns (must expose `cell_accent_mode`)

1. **Embed `TextOverrides`** (or the pattern-specific overrides struct that includes `CellAccentMode string`). The shared `TextOverrides` struct in `overrides.go` carries the `cell_accent_mode` field.
2. **Validate** by calling `ValidateCellAccentMode(patternName, ovr.CellAccentMode)` in the pattern's `Validate()` method. This rejects unknown modes with a structured `ValidationError`.
3. **Resolve per-cell accent** by calling `ResolveCellAccent(baseAccent, cellIndex, cellAccentMode)` in the cell-emission loop of `Expand()`. The function returns the accent string for each cell position given the base accent and mode.
4. **Schema** must include `cell_accent_mode` in the overrides object — use the shared helper: `EnumSchema("uniform", "alternate", "progressive").WithDescription(...)`.

`metric-list` (value colour per row) and `labeled-rows` (label-block fill per row) follow this contract.

### Publish only the overrides the pattern reads

A pattern that embeds `TextOverrides` must honour every key its overrides schema publishes. When a standard key has nothing to act on — `pyramid`, `process-flow` and `process-flow-compact` have no header text, so `header_size` would be a silent no-op — publish `textOverridesSchemaWithout("header_size")` and call `rejectUnusedTextOverrides(name, ovr, "header_size")` in `Validate()`, which returns an `UNKNOWN_KEY` error with a `remove_key` fix (go-slide-creator-s1uvj.41). `pyramid` colours its tiers by `cell_accent_mode`, picking each tier's text colour against that tier's own fill.

### Non-grid patterns (do not expose `cell_accent_mode`)

These patterns have structurally determined accent logic and do not expose `cell_accent_mode`:

- **Single-cell patterns** (stat-hero, pull-quote): one text block, no variation needed. (pull-quote's grid holds the quote and its attribution in separate rows plus an accent-rule column — see go-slide-creator-36ny — and an optional headshot column beside them; there is still only one accent in play.)

### Fitting a label to its shape

A pattern that paints text into a shape it sized itself must measure the fit,
because the renderer will not rescue it: the shape_grid renderer FLOORS any
authored text size at `shapegrid.MinTextSizePt` (12pt), so a pattern that
"shrinks to 9pt" has its size silently raised back to 12 and the label breaks
mid-word anyway (go-slide-creator-vo0j1).

The contract, as `numbered-step-strip` and `value-chain` implement it:

1. Compute the text width the shape actually leaves — `equalColumnWidthPt` for
   an N-column strip, minus `2*defaultShapeInsetLRPt`, minus any geometry notch.
2. Shrink ONE shared size until every label fits on one line
   (`fitSingleLineSize`), with the floor set to the renderer's floor, never
   lower. A shared size keeps the row even.
   `fitSingleLineSize` measures against `textfit.AtomicTokenWidthPt`: the full
   width for an available face, 80% of it when the theme font is substituted
   by the measurer (LibreOffice draws "Segoe UI" ~20% wider than the Arial
   stand-in, which split "$4.2M" into "$4.2" / "M"). Where a wider column is
   possible, widen it before shrinking: `process-grid-2row` grows its
   row-label column from 12% to at most 22% at the requested label size, and
   only then drops a point (go-slide-creator-b7qqg.14 / .15). The
   `comfortable` / `presentation` type-scale growth honours the same rule —
   it never grows a word, or a whole KPI value, past the atomic-token share of
   its line (`tokenGrowthCap`).
3. Report what still does not fit from `PostExpandWarnings` as
   `TEXT_EXCEEDS_SHAPE`. That code from a pattern is **blocking**
   (`shrink_or_split`), because the pattern has measured the failure rather
   than estimating it — see docs/FIT_FINDINGS.md.

The author's fix is a shorter label or fewer steps. That is a real constraint,
not a defect to engineer away: ten columns across a 13.3" slide leave about
65pt of text width, which is nine or ten characters at 12pt.

### Pictures in pattern values

Four patterns take a real picture in their `values`, all through the same
`{path | url, alt}` reference (`PhotoSchema` / `validatePatternPhoto` in
`internal/patterns/pattern_photo.go`):

| Pattern | Field | Without it |
|---------|-------|------------|
| `image-text-split` | `values.image` | Dashed wireframe placeholder labelled with `image_label` |
| `team-bios` | `values.members[].photo` | Initials disc (`photo_label`, else initials derived from `name`) |
| `pull-quote` | `values.image` (+ `overrides.image_side`, `overrides.image_width_pct`) | No picture column at all — the quote keeps the full width |
| `contact-directory` | `values.groups[].people[].photo` | Initials disc (pale accent tint, measured ink) |

Rules a new picture-taking pattern must follow:

- Implement `ImageAssetPattern`. Its `ImageAssets` must return refs that point
  **into** the decoded values (not copies), because the host rewrites
  `Path` in place when it resolves a relative path or downloads a URL. A ref
  returning a copy silently discards the resolved path.
- The `Field` of each ref is the JSON pointer under `values`
  (`"image"`, `"members/0/photo"`), which the host prefixes with
  `/slides/N/pattern/values/` when it reports a finding against it.
- A circular headshot is the image cell's `geometry: "ellipse"` with `fit: "contain"` (a square frame clipped to a circle), not a pre-cropped file — `contact-directory` does this.
- Validate the reference with `validatePatternPhoto`: a reference with neither
  `path` nor `url` renders nothing at all, and `overlay` / `text` belong to
  shape_grid image cells, not to pattern values.
- Give the picture its own **sibling** column or row, never a nested grid
  wrapping the pattern's text. The readability preflight walks a slide's
  top-level cells, so text moved inside a nested grid stops being fit-checked
  (go-slide-creator-hdpq).
- Measure the text against the width that is **left** after the picture's
  column. Measuring against the full width sets a type scale the narrower
  column cannot hold.
- **Axis-bound matrices** (matrix-2x2): quadrant fills are semantically tied to axis positions, not peer cells.
- **Fixed-progression patterns** (pyramid): tier fills follow a structural hierarchy, not a peer-cell walk.
- **Content-structured layouts** (bmc-canvas, agenda, agenda-with-images, roadmap-phased, phase-roadmap, scqa-summary, swimlane, timeline-horizontal, team-bios, quote-cluster, dual-org-ladder, table-highlight, image-text-split, capability-heatmap, state-shift-hub, contact-directory, text-sidebar): cell fills are determined by content structure (lanes, phases, sections, member cards, quote bubbles, org-paired rows, highlighted table row/column, rating tier, today-vs-future nodes around a hub, contact groups, the one key-message panel) rather than peer ordering.

### Free-positioned shapes: the lattice technique

The shape grid cannot place a shape at an arbitrary x. A pattern that needs
one — `state-shift-hub` puts its numbered nodes on an arc around the hub, so
every row's node sits at a different horizontal offset — computes the
positions in points and turns them into a **lattice**: every x edge the layout
needs (text-box edges, node edges, hub edges) becomes a column boundary, the
column gap is a hairline (`0.01`pt, as `pyramid` does), and each shape spans
the lattice columns between its own edges. Free lattice columns before a shape
are filled with empty cells (`&GridCellInput{}` — the resolver advances one
column per empty cell and skips row-span-occupied columns on its own), so emit
one per FREE column, not one per column.

- Because each shape owns its own lattice rectangle, shapes cannot overlap at
  whatever size the grid is finally resolved at — the trig only decides where
  the edges fall. Merge edges closer than ~1pt so no column collapses to zero.
- Use `fit: "contain"` for circles (hub, nodes): the cell stays rectangular and
  the shape is drawn square inside it, so a compose cell or a different
  content area turns the arc slightly but never distorts a circle.
- Row connectors cannot draw the spokes: a connector only fans out from a
  row-spanning cell to cells on its RIGHT, so a centred hub would connect to
  one side only. Let the geometry carry the relationship instead.

Each non-grid pattern should document in its `UseWhen`/`NotWhen` text or code comments why it does not expose the override.

### Test guidance

Every grid-shaped pattern must include a table-driven test exercising all three modes (`uniform`, `alternate`, `progressive`) against at least two different base accents (e.g., `accent1` and `accent3`). Verify that the emitted cells carry the expected accent strings. See `overrides_test.go::TestResolveCellAccent` for the shared function tests; pattern-level tests should exercise the full `Expand()` path.

## Cell Capacity Contract

The engine computes a deterministic text budget for every shape grid cell. Pattern authors do not implement capacity logic — the `internal/textcapacity` package derives budgets externally from the resolved grid geometry.

### Core rules

1. **`Expand()` must remain pure.** A pattern's `Expand(values, overrides, ctx)` converts structured values into a `*ShapeGridInput`. It must not call `textcapacity` or perform any capacity calculations. Capacity is computed downstream by the expand command or MCP tool after `Expand()` returns.

2. **Density is a HEIGHT ratio; `max_chars` is a derived hint.** `DensityPct` is the measured height of the wrapped text block — every paragraph laid out at **its own** font size, summed — over the height the cell offers. `max_chars` remains as a sizing hint, computed at the cell's *dominant* size (the one carrying the most characters).

   It used to be a character ratio against a single size, the **largest** paragraph in the cell, which made every mixed-size cell nonsense in both directions: a `stat-hero` cell with a 120pt number above three small support lines reported 911% "overflow" while rendering with room to spare, and most cells of most patterns reported "underfilled" (go-slide-creator-yj77). An unsized cell is also now measured at the size it renders at (`shapegrid.DefaultTextSizePt`, 14pt) rather than a legacy 11pt budget default.

3. **Written text settings.** Capacity uses the writer's text parser: object content uses `text.size`, paragraph arrays use each paragraph's `size`, and unspecified sizes use the writer's 14pt default. Authored sizes below the shape-grid 12pt floor are raised before measurement. Set size in the emitted text JSON, not a nonexistent shape-level `font_size` field. A row sized in points for its text must also fit by the writer's own measure (`writtenFitHeightPt`), because the theme-font model can undershoot by a line fraction and generation refuses grid text whose stored autofit drops it below its role floor; size any label threshold at `shapegrid.EffectiveTextSizePt`, not the authored sub-12pt size. The model and the writer measure in the same face (go-slide-creator-ohhb2): expansion stamps every pattern shape with the theme fonts it was sized in (`ShapeSpecInput.MeasureFonts`, not part of the schema), and the writer measures the stored shrink in the body's own face when every run resolves to one host-independent face (`fontcache.HostIndependent`: Calibri as its metric clone Carlito, Lora, Poppins Light, Arial as Liberation Sans) — full width and its own weight for the widest-word check — falling back to Liberation Sans with the 3% word safety otherwise (mixed faces, host-dependent faces, authored grids). A written-fit probe must therefore measure as the writer will: pass `ctx.themeFonts()` to `writtenFitHeightPt` / `rowTextNeedPt` / `writtenFitsAt`, and set `tb.ThemeFonts = ctx.themeFonts()` before calling `pptx.AutofitFitsFor` on a body you resolved yourself. Rotated text (`vert` / `vert270`) is measured along its line, the shape's height. `TestPatternExemplarsStoreNoAutofitShrink` holds every exemplar to no stored shrink on the Calibri templates and p-style.

4. **Determinism guarantee.** `textcapacity` uses `go-fonts/liberation` embedded metrics — no OS font dependency. Given the same grid geometry, font size, and insets, budgets are identical across macOS, Linux, and CI. This is a hard invariant; if a pattern change causes budget drift in CI, the change is wrong.

5. **Insets and fitted bounds matter.** Capacity uses the actual fitted shape rectangle, not its larger allocation cell. The uniform 0.5 cm shape margin (or an authored side), plus any overlay reservation, reduces the available area once, clamped exactly as the writer clamps a degenerate shape; paragraph trailing spacing contributes to required height. A frame that cannot hold one unshrunk line has zero character capacity. Character counts are nominal font/glyph hints, not a promise that arbitrary text fits; validate and render against the chosen template.

### Testing patterns with capacity

When writing or modifying a pattern, verify that the capacity model produces sensible budgets:

```go
func TestMyPattern_CellBudgets(t *testing.T) {
    pat, _ := patterns.Default().Get("my-pattern")
    for _, config := range []struct {
        name   string
        values map[string]any
    }{
        {"3-cell", map[string]any{"items": threeItems}},
        {"5-cell", map[string]any{"items": fiveItems}},
    } {
        t.Run(config.name, func(t *testing.T) {
            grid, err := pat.Expand(toJSON(config.values), nil, defaultCtx())
            require.NoError(t, err)

            result, err := shapegrid.Resolve(gridFromInput(grid), alloc)
            require.NoError(t, err)

            densities := textcapacity.ForResolvedGrid(result)
            for i, d := range densities {
                if d.ActualChars > 0 && d.MaxChars == 0 && d.Fits {
                    t.Errorf("cell %d: populated unusable frame reported as fitting", i)
                }
                // Budget should be > 0 for text cells
                if d.Status == textcapacity.StatusOverflow && d.ActualChars > 0 {
                    t.Logf("cell %d: overflow at %d%% density", i, d.DensityPct)
                }
            }
        })
    }
}
```

Parameterize over grid configurations (different cell counts, column layouts) and assert that:
- Usable text frames have positive character budgets
- Unusable frames report zero capacity rather than an invented minimum
- Density bands shift as expected when content length varies

### Density bands reference

| Band | Density % | Status string | Agent action |
|------|-----------|---------------|--------------|
| Underfilled | < 35% | `"underfilled"` | Add content or pick a smaller grid |
| Optimal | 35–110% | `"optimal"` | No action needed |
| Overflow | > 110% | `"overflow"` | The renderer will shrink this cell's text to fit (`<a:normAutofit/>`). Trim content or pick a larger grid if the shrink would push text below the readable floor. |

Density % is `required text height / available text height`, so >100% means "needs an autofit shrink", not "clipped". Two further signals separate those cases:

- **`fits: false`** (Density.Fits) — the block does not fit even at the smallest shrink the renderer applies (`textcapacity.AutofitFloorScale`, 20%). This, and only this, is what `fit_overflow` reports for a shape_grid cell: text that is actually clipped.
- **`TEXT_BELOW_READABLE_MIN`** — the predicted post-autofit size is under the viewing mode's floor for that text role. This is the finding for "it fits, but only because it shrank too far"; it blocks (`refuse`), because generation refuses the same written run.

These thresholds are defined in `internal/textcapacity/textcapacity.go` and are stable — do not hardcode different values in patterns.

## Highlight fills must be chosen by measurement (authoring contract)

A pattern whose one semantic signal is a highlighted cell must not paint it from a fixed scheme slot. Contrast between two slots is template-dependent: value-chain painted steps `dk2` and the highlighted step `accent2`, which is 3.21:1 on midnight-blue and **1.48:1 on warm-coral** — the highlight simply disappeared, and no test could see it because the pattern was tuned on one template (go-slide-creator-ah5s).

- Pick the default with `pickDistinctFill(ctx, base, fillDistinctnessMin, candidates...)` (`internal/patterns/fill_contrast.go`): it returns the first candidate whose EFFECTIVE colour (tints and alpha composited) clears 3:1 against the base fill. List candidates in preference order — the brand accent first — and you lose your preference only to a measurement. Value-chain now paints its steps the neutral dk1 16% tint (go-slide-creator-8xsj3) and measures at `valueChainHighlightMin` (2:1): against an achromatic base a saturated accent also separates by hue.
- Without a theme there is nothing to measure: fall back to the historical default so an expansion with no template is unchanged.
- An AUTHORED highlight is always honoured, and reported through `PostExpandWarnings` as `LOW_CONTRAST_HIGHLIGHT` when it measures below the bar.
- Measured bars for the bundled templates live in `internal/patterns/valuechain_highlight_test.go`; `cmd/json2pptx/value_chain_highlight_test.go` runs the same rule against the real `templates/*.pptx`, so a palette change is caught rather than shipped.

## Every shape's text keeps a uniform 0.5 cm margin

Every text-bearing shape the engine draws — every `shape_grid` cell, so every named pattern, `compose` block and raw grid, and the native diagram shapes of `internal/generator` (panels, heatmap and nine-box cells, SWOT / PESTEL / BMC / Porter's / house / pyramid / value-chain / process-flow / KPI-dashboard shapes, the takeaway band, overlay badges) — keeps its text **0.5 cm (180000 EMU, ~14.17pt) from the shape edge on all four sides**, on every template. Template placeholders keep the template's own margins; table cells and slide chrome (footer, page number, source note) are not shapes. The value lives in one place, `pptx.ShapeTextInsetEMU` / `pptx.ShapeTextInsets()` (`internal/pptx/shape_text_inset.go`), and every emitter routes through it.

- **Patterns never write `inset_*`.** A pattern cell's text object carries no `inset_left` / `inset_right` / `inset_top` / `inset_bottom`: `shapegrid.ResolveTextInput` defaults every side to the uniform margin. The only pattern-authored inset is a *top offset that aligns first baselines* (`exec-summary`, `chart-insights-split`) or centres a card block (`framework-grid`): it is the uniform margin **plus** the offset, never less. Room a pattern needs for an icon overlay or a ribbon is added on top of the margin (`ResolvedCell.TextInsets`, stylish-panels' ribbon clearance), never instead of it.
- **Degenerate shapes clamp, they do not overflow.** When a shape is too small to hold one line of its text plus 2 × 0.5 cm on an axis — a pill, a number badge or initials disc, an axis or legend label, a thin caption band — the writer shrinks that axis's margin (both sides, proportionally, never below zero) to what still leaves one line at the largest run size (vertically) or the widest word plus its paragraph's `marL` / `marR` with `pptx.WordFitSlack` (5%; `StandInWordFitSlack`, 20%, for a host-dependent face) of room (horizontally — a word handed exactly its width broke its last glyph, go-slide-creator-v74wv), measured against the preset's own text rectangle (the inscribed square of an ellipse, half a diamond, …). The other axis keeps the full margin. This is `pptx.EffectiveTextInsets`; `pptx.GenerateShape` applies it before measuring the autofit scale, so the stored insets and the stored scale agree. Prefer giving a badge or row enough room to keep the full margin: the clamp is a floor, not a layout tool.
- **Estimators use the same numbers.** `defaultShapeInsetLRPt` / `defaultShapeInsetTBPt` / `sizingInsetLRPt` / `sizingInsetTBPt` in `internal/patterns` are the uniform margin; `writtenFitHeightPt`, `internal/textcapacity`, the shapegrid row estimate, the fit geometry detector (`TEXT_EXCEEDS_SHAPE`) and the native-diagram preflight all resolve the writer's text body and apply `pptx.EffectiveTextInsets` (or `pptx.UniformInsetFor`) rather than assuming the OOXML 0.1" / 0.05" defaults. `pptx.AutofitScaleFor` measures the text area left after the declared insets (it no longer subtracts textfit's own 7.2pt sides a second time).
- **Budgets are measured with the margin.** The margin costs ~28pt of height per stacked text row, so every published copy budget, max count and `BODY_TOO_LONG` threshold was re-measured with the `*BudgetProbe` tests; several dense configurations now hold no body / detail / subtitle at all and say so.

## Type scale and typographic finishing (go-slide-creator-30471, -58dhw)

Pattern text uses the five-step type scale in `internal/tokens/typography.go`: `TypeScaleDisplayHPt` 28pt, `TypeScaleLeadHPt` 18pt, `TypeScaleSubheadHPt` 14pt, `TypeScaleBodyHPt` 12pt, `TypeScaleCaptionHPt` 10pt, plus `TypeScaleKPIMinHPt`–`TypeScaleKPIMaxHPt` (40–48pt) for KPI values; `BodyTextMinHPt` (11pt) is the body minimum and `DenseBodyTextMinHPt` (10pt) is for tables and dense matrices. The role ladders in `tokens.go` are ranges on this scale, and the readability floors (`MinReadableHPt`) are scale steps, so nothing here loosens `TEXT_BELOW_READABLE_MIN`.

Name every default size by its step: `internal/patterns/type_scale.go` mirrors the scale (`scaleDisplayPt` … `scaleCaptionPt`, `scaleKPIPt`), and an off-scale default (a 16pt header measured with headroom, a 120pt stat-hero figure) is a named constant with a reason in `offScaleDefaultReasons`. `TestPatternDefaultSizesOnTypeScale` fails on a numeric literal used as a `ResolveSize` fallback, a paragraph `Size` or a size variable, and on an off-scale constant that is not allow-listed. Shrink ladders a pattern walks while measuring (`execSummarySteps`, `metricListScales`, `labeledRowsScales`, `nextStepsScales`, …) name scale steps too, so a ladder measures text at the size it renders and steps from one step to the next (exec-summary: 14/14 → 14/12 → 12/12; metric-list values 40 → 28 → 18pt). `TestPatternFitLaddersOnTypeScale` scans every composite literal assigned or appended to a `*steps` / `*scales` / `*sizes` variable and fails on a literal size or an off-scale step; the only off-scale ladder steps are display figures above 28pt (hero-detail's 80pt hero) or a half-step whose removal would lower a `TestSchemaMaximaStayReadable` pin, each with a reason in `offScaleLadderReasons` (none is needed today). A derived step (a 2pt label bump, a heading body+6pt) is settled with `snapPt`.

**Ratios.** The steps are caption 10 → body 12 → subhead 14 → lead 18 → display 28pt, ratios 1.20 / 1.17 / 1.29 / 1.56: a ~1.2 minor-third ladder through the reading sizes (caption, body, card title) and a wider ~1.3–1.6 jump to the headline and display sizes, so a slide uses at most four sizes and each level reads as a level. 11pt (dense body) is a floor for dense cells, not a display step; KPI values sit in the 40–48pt band (1.43–1.71 × display). Chart text in `svggen` uses the same steps (`svggen/type_roles.go`: title 18, subtitle 14, heading/body 12, labels/captions 10pt; the layout-preset tiers and `ScaleForDimensions`' floors and caps are steps too — a large canvas lifts each role at most to the next step above it), and `internal/tokens/chart_scale_test.go` pins svggen's mirrored steps, roles, presets and caps to the slide scale.

- **Pick sizes from the scale.** `shapegrid.Resolve` settles every sized cell paragraph onto the step at or below it (`snapShapeTextToScale`, `tokens.SnapTextHPt`), after `type_scale` growth and before the row-shared autofit, so render, preflight and readability checks see the same size. Display figures (≤25 runes containing a digit at 18pt+), text at 28pt+ and a `design_mode: "free"` deck's own grids (`ShapeGridInput.KeepTextSizes`) are exempt. Snapping only shrinks, so an off-scale constant costs hierarchy, not fit — author on the scale instead.
- **Caps labels are tracked, not spaced.** A paragraph that is a short, bold ALL-CAPS label (≥3 letters, ≤40 runes, ≤18pt) gets +7% letter-spacing (`a:rPr spc`) when the tracked label still wraps to the same number of lines (`trackCapsLabels`). Emit the label text in caps; never pad it with spaces. Regular-weight caps (initialisms in body cells such as a next-steps owner "VP CS") are body text and are never tracked (go-slide-creator-y1476).
- **Bold headings and names do not end on a lone word.** A bold, unbulleted paragraph of 3–16 words whose last line holds one word gets a right margin (`a:pPr marR`) in the middle of the measured balanced range (`balanceHeadingLines`, `textfit.BalancedMarginEMU`); the line count never changes. Content-layout titles get the same treatment in the generator (`balanceTitleLines`) when their alignment and font metrics are known.
- **One alignment per card.** Centred paragraphs above left-aligned text in the same shape are set left (`unifyCardAlignment`); all-centred tiles and right-aligned figures are unchanged. A header cell stacked over a left-aligned body cell (before-after) is authored left-aligned.
- **Person names stay on one line.** `contact-directory` first searches type step, headshot size and column count for a layout that sets every name on one line, and only then accepts names wrapping between words.

## A pointed shape's text has to clear its own point

A chevron, a right arrow and a home plate all draw their point INSIDE the bounding box, so text laid out to that box is drawn into the notch and the tip. `process-flow`'s chevron step type did this — the first and last characters of a label disappeared into the geometry — and its row connector was drawn straight through the shape (go-slide-creator-czk4; `numbered-step-strip` had the same fix in round 1).

- Set `adjustments: {"adj": chevronAdj}` (30% rather than the OOXML default 50%). The preset's own text rectangle already stops short of the point and the notch; the uniform 0.5 cm margin sits inside it, so a pointed label needs no pattern-authored inset.
- The notch is `adj x the SHORTER side`, so compute it from the pattern's own cell geometry — borrowing another pattern's number is how an inset ends up 16pt short. A row of pointed shapes should also cap its height at half its step width, or the shape's own height sets the notch and the label is left a column.
- Drop the connector between pointed steps: they already say which way the flow runs, and the arrow was drawn through the notch.
- **Reconcile the inset with the shape width, and measure against what the renderer actually gives the text.** `numbered-step-strip`'s chevrons kept a fixed 30% notch as the strip got denser: at 6 steps a chevron is 131pt wide and the inset took 47pt of it, so "Qualification" rendered as "Qualific / ation" on midnight-blue and warm-coral — with no finding, because two lines still fit inside the shape (go-slide-creator-e97v). Three things were wrong at once, and all three are easy to repeat:
  - The step width was the column's share of the content area. A grid `gap` of `0` reads as "unset" in the DTO and resolves to shapegrid's **8pt default**, so each shape is `(contentW - 8pt × (n-1)) / n` wide, not `contentW / n`.
  - The renderer lays text out inside the chevron's OWN text rectangle, already pulled past the point and the notch; the uniform `lIns`/`rIns` stack on top. Budget **the notch plus the margin** per side, or the measurement is far wider than the real text area.
  - The measurement runs on whichever font the machine has (Liberation Sans substitutes for a missing template font) while the renderer uses its own. Require the label to fit inside ~90% of the computed width; the last few points are not knowledge you have.
- Give up notch depth BEFORE type size: a blunter arrow still reads as an arrow, and short labels keep the full 30% because the search starts there and stops at the first depth that fits. Shrink the label only after the shallowest allowed notch, never below the renderer's readable floor, and emit a `BODY_TOO_LONG` advisory from `PostExpandWarnings` when even that wraps — at that point the label is too long for the step count and only the author can fix it.

## A field's maxLength is the budget of the pattern's SMALLEST shape

A pattern's JSON schema is the contract an agent sizes its copy against, and a per-field `maxLength` can only state one number. For a pattern whose cell count varies, that number is necessarily the budget of the *smallest* grid: card-grid's 300-character body is readable in a 1x1 and renders at **2.6pt in a 5x5**. Content that respects the schema in every particular is still a wall of unreadable text, and the schema cannot say so (go-slide-creator-0g6p).

- Keep `maxLength=300` as a hard input bound, not a promise that 300 characters fit every card. The field description directs authors to the per-card budget in `expand_pattern`.
- Emit a `BODY_TOO_LONG` warning above that same per-card budget. Both `cell_budgets[].max_chars` and the warning account for the selected content area, body font size, card header, icon/chart reservations, and grid dimensions; `actual_chars` counts the body alone. The budget is computed before the supplied body copy sizes a row, so it cannot rise as an author adds text.
- Derive the budget by MEASUREMENT, not by arithmetic: run the payload at each shape through the same readability prediction the fit report gives an agent. `cmd/json2pptx.TestSchemaMaximaStayReadable` does this for every registered pattern on all four bundled templates and pins the result, so a schema maximum cannot quietly get worse and an improvement cannot be given back.

### Rule-list patterns: `agenda` and `next-steps`

Both follow the design review's rule language (go-slide-creator-r3gsw, go-slide-creator-7lzdh): serif accent numerals, 0.5pt `dk1`-at-30% rule rows between content-height rows, no tile fills, a middle-anchored block with surplus height passed to the item rows (`fillCappedRows`, 62% / 55% minimum fill). Dimmed text (the non-current agenda rows, the next-steps column header) uses the shape-grid paragraph `alpha` (text opacity in percent, needs an explicit `color`); pick it with `readableDimAlpha` so the WCAG pass keeps the intended ink instead of swapping it.

### Row-list patterns: `metric-list` and `labeled-rows`

Both are content-sized row stacks separated by 0.75pt hairline rule rows, sized like `exec-summary`: they try a descending type scale, keep the largest whose measured natural height fits the content area, and pass surplus height to the content rows (`fillCappedRows`, 68% / 60% minimum fill, each row at most 1.6× its natural height). Overrides that set a size pin the scale. What they cannot fix is reported from `PostExpandWarnings`: `TEXT_EXCEEDS_SHAPE` for a value (metric-list) or a keyword word (labeled-rows) that still breaks at the floor, and `BODY_TOO_LONG` when the stack is taller than the content area at the smallest scale.

`metric-list` sets `col_gap` to 0.1pt, not 0: a highlighted row tints both of its cells, and a real gap (0 resolves to shapegrid's 8pt default) shows as a white seam through the band. The gutter comes from the two cells' uniform text margins instead, and each highlighted cell is outlined in its own band colour (lumMod / lumOff, which shape lines honour; tint they do not) so no hairline shows.

## The takeaway component

One renderer carries every "so what" in the engine (go-slide-creator-7b5o6): the slide `takeaway` band (`internal/generator/takeaway_note.go`), chart-insights-split `so_what`, exec-summary `bottom_line`, metric-list `callout`, and the pattern / compose envelope `callout` (`cmd/json2pptx/pattern_resolve.go` `appendCalloutRow`). Pattern surfaces build it with `patterns.TakeawayRow` / `TakeawayGrid` (`internal/patterns/takeaway.go`); the slide band reads the same constants.

- **No stroke, no fill by default.** A flush 3pt accent bar (accent1, or the pattern's resolved accent) runs the full height of the band on its left.
- **Text** is 14pt bold in the theme's `dk1` ink (never a hex), 12pt from the bar, top-anchored, budgeted to two lines; the band is sized to its measured lines.
- **Width** is the content width (the host grid's width, or the body column for the slide band), never a full bleed.
- **Air:** at least 16pt above the band (a spacer row tops up the host's row gap and the renderer's 4pt sub-grid inset) and at least 12pt below it to the source line or footer (`template.ResolveChromeFrame`).
- **Variants** — `emphasis` on a callout, `overrides.takeaway_emphasis` on exec-summary / metric-list / chart-insights-split: `subtle` adds a 5% `dk1` tint behind the text; `strong` fills the band with the accent and picks the text ink by measured contrast. A callout's `italic` / `bold-italic` set the (always bold) text italic.
- The band row is an auto-height row floored at its measured height, never a `max_height` pin: any max on a row switches the host grid from stretching its rows to content-sized rows, which shrank auto-height hosts to their estimates (`numbered-step-strip` rows are now pinned at their written fit, measured against the bounds the callout band has already shortened). A pattern callout also shrinks the pattern's expansion bounds by the band's height before expanding.
- On a layout whose background would leave `dk1` unreadable, the slide band inks in the theme colour the chrome contrast check picks (the takeaway is injected after the contrast pass).

The chevron "BOTTOM LINE" flag, the peach accent-tint band with a 1pt accent outline, the solid accent metric-list banner and the solid accent callout strip are gone.

## Text on a tinted fill must be chosen by measurement too

The same rule applies to the text a pattern paints INSIDE a fill it tints itself. `timeline-horizontal` tints each bar of its gantt and chevron chains — shade 70000 at the first stop through tint 40000 at the last — and hardcoded `lt1` inside every one of them, so the lightest bar measured **1.54:1** in a real midnight-blue render and its date label was invisible (go-slide-creator-5qotm).

- Ask `readableTextOn(ctx, tone, fallback)` (`internal/patterns/fill_contrast.go`) which of the light / dark text roles reads on the fill's EFFECTIVE colour. Without a theme, `timeline-horizontal` uses `dk2` on tinted links and `lt1` on darker links; a portable expansion must not bake white text onto a pale tint.
- Build the fill from a `fillTone` and emit it with `tone.fillJSON()`, so the tone you measured and the fill you paint cannot drift apart.
- **One left edge (go-slide-creator-svrpx).** Unfilled, unoutlined, left-aligned first-column text (exec-summary lead-in numerals, next-steps numerals, agenda numerals, `text`-style labeled-rows keywords) starts on the title's text edge: the grid receives `ContentZone.TextLeft` (title placeholder X — or the side-decor-shifted content column — plus the title's resolved `lIns`, layout → master → 91440) and `alignFirstColumnText` reduces those cells' left inset to reach it (never increases it; explicit `inset_left` and icon reservations are kept). Filled cards keep their padding. On a title-only layout the takeaway / source bands use the title column (`ChromeFrame.Basis` `layout_title`), the same span `titleOnlyContentZone` gives the pattern, instead of the reference layout's body column.
- **Brand-coloured display text is judged per run (go-slide-creator-tinsz).** The render-time shape-grid contrast pass holds neutral inks (lt1 / dk1 / white / black) to the body's smallest-text bar so one cell never splits white and black, but judges a non-neutral colour per run (`a:rPr` / `a:defRPr` / `a:endParaRPr` block) at that run's own size and weight: a 40pt accent value clearing 3:1 keeps the accent while its 11pt label is fixed, and a large accent that misses 3:1 on a light fill is lerped darker in its own hue rather than snapped to the palette. Sibling grouping and per-fill harmonisation follow the same per-run bars. Softened KPI peer cards (`SoftenPeerFills`) draw their ≥24pt figure in the accent, or the minimal linear-light darken of it that clears 3:1 on the neutral surface.
- **Shade the fill before blackening the type (go-slide-creator-v9tup).** When `lt1` misses the bar on a mid-tone accent the pattern itself draws (abstract #8E8172, warm-coral #E64A19, p-style #FD5108, blue-corporate #55BC7E, business-template #AD84C6), `accentFillAndInk(ctx, tone, minContrast)` returns the accent deepened by the smallest `a:shade` (linear-light keep ≥ 45%) at which `lt1` clears it, keeping the hue, theme link and white type. `ApplyReadableInk` does the same for every shape whose text is only light ink. Pale accents (lt1 below 1.8:1), tints, translucent fills (alpha < 80%) and shapes that mix dark and light ink keep their fill and take the first readable ink `lt1 → dk2 → dk1`. Hex fills are returned as the shaded hex, since the shape-grid resolver honours modifiers on scheme colours only.
- `tint` and `shade` are **linear-light mixes** toward white and black, not the HSL lightness that `lumMod` / `lumOff` act on. `EffectiveColorMods` models all four; both transforms are pinned against measured render pixels in `internal/patterns/timeline_contrast_test.go`. A model that treats a tint as a lumMod is out by 30-50 per channel, which is the difference between "readable" and "invisible".
- In `timeline-horizontal` chevrons, measure the label and body against the chevron's usable text width and capped row height. Emit `BODY_TOO_LONG` with the available body-line count when the description would clip; a raw character limit misses narrow seven-stop layouts.
- Chevron `body_size` controls both the emitted paragraph and its fit budget. Sizes below shape-grid's 12pt rendering floor are measured and emitted at 12pt, including the default derived from `label_size`.
- Chevron dates use that same 12pt rendering floor for their one-line row height. A date that wraps at the effective font size and template width receives a `BODY_TOO_LONG` fit finding instead of relying on a fixed character count.

## capability-heatmap and framework-grid (column- and row-structured frameworks)

Both patterns size their rows from measured text and then grow them toward a
fill target, with a ceiling, so a sparse grid does not turn into tall empty
tiles:

- **`capability-heatmap`** measures every header at the width the `homePlate`
  leaves (the preset's text rectangle stops half-way into the point, and the
  uniform right margin stacks on it and clears the rest), shrinks one shared header
  size toward the 12pt floor until no header word breaks, and reports a word
  that still cannot fit as `TEXT_EXCEEDS_SHAPE`. The point is a fixed 10pt,
  with `adj` derived from the header's own height. Cell rows share one height
  (the tallest activity), grow to at most 66pt toward 95% of the content
  height, and a `BODY_TOO_LONG` warning names the longest activity when the
  content-sized block already exceeds the content area. Text ink on every
  tier fill is chosen by `readableTextOn`. The legend is a nested grid with an
  explicit row height (see go-slide-creator-z0up); each entry's width is
  proportional to its wording. `cell_overrides` index the headers first, then
  each column's cells top to bottom.
- **`framework-grid`** gives every row the tallest row's height (so it reads
  as a grid), grows rows to at most 1.8× that toward 80% of the content
  height, and top-anchors every card at one shared top offset (the uniform
  margin plus half the row slack) so titles line up
  across a row. The card title stays in the column accent only when it clears
  the bar the render-time contrast pass applies to the WHOLE text body — that
  pass judges a shape by its smallest run, so a title above a 12pt body needs
  4.5:1 and only a title-only card at 14pt bold gets 3:1; otherwise the
  measured theme ink is used. Picking a lower bar would just hand the choice
  to the fixer, which swaps in a literal colour. `BODY_TOO_LONG` names the
  tallest row when the grid cannot fit. `cell_overrides` index row by row: the
  label, then that row's cards.

## Composition

Pattern composition is implemented via the slide-level `compose` envelope (see `cmd/json2pptx/compose.go`). A `ComposeInput` is XOR with `pattern` / `shape_grid` and arranges 2..N segments either vertically or horizontally; each `SegmentInput` carries exactly one of:

- `pattern: PatternInput` — a leaf pattern expansion (legacy behavior).
- `compose: ComposeInput` — a nested envelope, recursively expanded and merged into the parent grid.
- `diagram: types.DiagramSpec` — a standalone svggen-rendered diagram or chart. Diagram segments synthesize a single-cell grid that participates in the parent merge identically to a pattern-expanded grid, so `compose.direction` + `size_pct` + `gap` drive placement and the gutter rhythm is unified across pattern and diagram segments. This is the canonical way to let a native pattern (`pyramid`, `kpi-3up`, `card-grid`, …) coexist with an svggen visual (`process_flow`, `bar_chart`, `sparkline`, …) on the same slide without flattening the pattern through a single cell — see `go-slide-creator-zg8q.6`.

Caps are advertised via `get_capabilities().features.compose`:

- `max_segments` — per-envelope top-level cap (default 8).
- `max_nesting_depth` — recursive cap on `compose`-inside-`compose` (default 2).
- `max_leaf_patterns` — global cap on leaf segments (pattern + diagram) across the entire envelope tree (default 12).
- Flags: `supports_smart_compose`, `supports_nested_compose`, `supports_diagram_segments`.

`cell_overrides` indices remain per-pattern; the merge step does not re-number them.

## Icon slot (card-grid, kpi-Nup, kpi-inline, matrix-2x2, icon-row, hero-detail)

Pattern cells that accept an icon use the shared `*IconRef` field defined in `internal/patterns/iconref.go`. Both forms are accepted:

```jsonc
// Bundled-name shorthand — backwards-compatible
{"header": "Launch", "body": "...", "icon": "rocket"}

// Full IconRef object — for path/URL/inline SVG with optional overrides
{"header": "Brand", "body": "...", "icon": {"path": "logo.svg", "fill": "#FF0000", "alt": "company logo"}}
{"header": "API",   "body": "...", "icon": {"url": "https://example.com/icons/api.svg"}}
{"header": "Wave",  "body": "...", "icon": {"svg_data": "<svg xmlns=\"http://www.w3.org/2000/svg\">…</svg>"}}
```

When the field is a bare string, it is classified at unmarshal time by `svggen.ClassifyIcon`:

| Input string                              | Routed to       |
| ----------------------------------------- | --------------- |
| Bundled name (`"rocket"`, `"filled:x"`)   | `Name`          |
| `"http(s)://…"` or `"data:…"`             | `URL`           |
| `"<svg…>…</svg>"`                          | `SVGData`       |
| Path with `/` or `\` + `.svg`/`.png`/`.jpg` | `Path`        |
| Anything else                             | `Name` (rejected by validator if not bundled) |

**`IconRef` fields** (from `jsonschema.IconInput`):

- `name` — bundled icon name (e.g. `"rocket"`, `"filled:trending-up"`). Validated against the bundled registry via `icons.Exists`.
- `path` — local `.svg` file path (relative paths resolve against the JSON input dir; supports `~/` and `$VAR` expansion). Non-SVG extensions are rejected at validate time.
- `url` — HTTPS or `data:` URL. Network resolution happens in the asset pipeline; validators accept any string here.
- `svg_data` — inline SVG markup. No disk I/O is performed when set; `fill` is ignored (pre-style the SVG instead). The shared validator does not enforce arity beyond "exactly one of name/path/url/svg_data".
- `alt` — accessibility description; defaults to a derived value from name/path.
- `fill` — hex or scheme color override (e.g. `"accent1"`, `"#FF0000"`). Pattern code supplies a sensible default (the cell's accent) when blank; explicit values win.
- `position` — `left`, `top`, or `center`. Defaults to the pattern-specific position (kpi-Nup → `top` on every card count, so icon and value share one centred axis; kpi-inline → `left`; card-grid/iconrow/herodetail/matrix → `top`) when blank. On any shape, a `left` overlay icon is capped at 25% of the shape width (the text's extra left inset is icon + 6pt padding), and a default-scale `top` icon on a landscape shape is capped at 40% of the shape height.
- kpi-Nup icons default to an accent-sized footprint (top: ≤ 28% of card height / 45% of width; left: ≤ 40% of height / 20% of width) by setting the overlay `scale`; an authored `scale` wins.
- kpi-Nup big values never wrap: the value font shrinks (uniformly across the row, floor 16pt) until every value fits on one line in its card's text width after the icon inset.
- `scale` — optional overlay scale factor (`0 < scale <= 1`) applied when the icon is overlaid on a shape; out-of-range or unset values fall back to the `0.6` overlay default. No effect on standalone (text-free) icon cells. `IconRef.Resolve` copies it through unchanged, and the bundled-name shorthand marshal form is suppressed when `scale` is set.

**Schema authoring.** New patterns that accept an icon should reuse `IconRefSchema(description)` and `validateIconRef(pattern, path, ref)` rather than duplicate the OneOf string-or-object schema. When a pattern repeats the icon slot across many siblings (matrix-2x2's four quadrants, multi-row card grids), wrap the cell schema in `$defs` and use `RefSchema(name)` to keep the per-pattern schema under the 6 KB compression budget.

**Expansion.** Pattern `Expand` code calls `cell.Icon.Resolve(defaultFill, defaultPosition)`. Resolve returns `nil` for empty refs, applies pattern defaults only when the author left a field blank, and copies the underlying `IconInput` so downstream mutation is safe. Patterns that supported a string-only icon field bumped their `Version()` to `2` when migrating. Compute `defaultFill` with `iconFillOn(ctx, shape.Fill, accent)` — never pass the cell accent directly: on a solid accent card that paints the icon in the card's own colour and it vanishes. `iconFillOn` returns the accent when it reaches 3:1 against the effective fill (light cards), else `lt1` when that reaches 3:1 (matching the pattern's on-accent text), else the better of `lt1`/`dk1`.

## PostExpandWarnings reach every surface

A pattern knows things about the content it was handed that no geometric
detector can see: a bio past its two-line budget, a chart panel with no chart,
a maturity model with two "current" stages. `PostExpandWarner.PostExpandWarnings`
is where a pattern says so, as structured `"<CODE>: message"` lines.

Those lines are collected by `collectPatternPostExpandFindings` (cmd/json2pptx)
and converted by `patternWarningAsFinding` into review-severity fit findings
scoped to `/slides/N/pattern`. They therefore appear in:

- `validate_input` and `validate --fit-report`
- `generate_presentation(fit_report=true)` and `generate --json-output-report`
- `score_deck`, `preview_presentation_plan`, `render_deck_spec`
- `expand_pattern` / `expand_patterns` / `patterns expand`, as a `warnings[]` array

Two rules for an author writing a new warner:

- **Prefix the code.** A line without a leading `CODE:` is dropped by the
  converter and reaches only the human-readable warning list.
- **Expect to be called on an already-expanded slide.** The collector re-expands
  the pattern regardless of whether the slide carries a `shape_grid`, because on
  the generate path that grid IS the expansion — skipping those slides would
  silence the warnings on the surface that matters most (go-slide-creator-wn4v).

## Secondary chart slot (card-grid, icon-row)

`card-grid` cells (`CardGridCell`) and `icon-row` items (`IconRowItem`) accept an optional `secondary *SecondaryChart` field that embeds a small chart below the cell's title/body or caption. The field is defined once in `internal/patterns/secondary_chart.go` and reused by both patterns:

```go
type SecondaryChart struct {
    Type       string    // "sparkline" | "bar_chart" | "line_chart"
    Values     []float64 // 2–12 numeric data points
    Categories []string  // optional x-axis labels; length must match Values when set
    Color      string    // optional hex/scheme color override
}
```

**Caps (enforced by `validateSecondaryChart`):**

- At most one secondary per cell (a single pointer field, not an array).
- `type` is restricted to `sparkline`, `bar_chart`, `line_chart`.
- `values` must be 2–12 numbers.
- `categories`, when set, must have the same length as `values`.

**Expansion.** When `Secondary` is set, the cell's base `Shape` is wrapped via `wrapCellWithSecondary` into a `CompositeInput{Text: <original shape>, SubDiagram: <built diagram>, Split: "top", Ratio: 0.6}` so the existing text+styling renders on top and the chart below. `sparkline` is mapped to a `line_chart` `DiagramSpec` with `Style.ShowLegend = false`; `bar_chart` and `line_chart` pass through.

When adding the same slot to a new grid-shaped pattern, reuse `SecondaryChartSchema()`, `validateSecondaryChart`, and `wrapCellWithSecondary` rather than duplicating their logic.

## chart-insights-split (data + narrative composite)

The `chart-insights-split` pattern is the canonical "chart on the left, takeaways on the right" consulting layout. The pattern emits a 65/35 column split: the left panel is a `Diagram` cell rendered by svggen; the right panel is a Shape cell with the title (defaults to `Key Insights`) and 1–6 bullet takeaways, or a full-height `so_what` callout when there are no bullets. At least one bullet or a nonempty `so_what` is required. **The default widens to 75/25 when the insights column is sparse** — at most two bullets, ≤140 characters in total, and no headline or so-what — because a 35% column holding one short bullet leaves a large empty block while the chart is squeezed into 55% of the slide (go-slide-creator-pyxn). A thin vertical accent divider can be toggled via `overrides.show_divider`, and `overrides.chart_width_pct` (clamped 40–80) pins the ratio, overriding both defaults.

**The stacked headline / so-what column is template-aware** (go-slide-creator-bzh34). With a `headline` or `so_what`, the column is a nested grid whose headline and so-what rows are pinned at the height the shape writer needs (`writtenFitHeightPt`, at the column's real inner width after the 8pt column gap and 4pt sub-grid inset), and the insights panel takes the rest. When the insights would not fit the slide's content area, the headline steps to 26pt and the so-what to 12pt, then (unless `chart_width_pct` pins it) the chart narrows in 5-point steps to 55%. If nothing fits, the closest layout is used and `PostExpandWarnings` emits `BODY_TOO_LONG` naming what to drop. Sizing those rows as percentages of an estimated area wrote the insights at 88% autofit (10.6pt) on midnight-blue while p-style fit. `table-highlight` follows the same rule: a table taller than the content area (a short template or a `takeaway` bar) re-measures its rows with the writer and compacts the legend reserve before the grid can over-fill, and reports `BODY_TOO_LONG` when it still does not fit. `image-text-split` sizes its text column with the writer at the nested width and narrows the image (to 30%, unless `image_width_pct` pins it) when the text would not fit.

For readability the right panel applies vertical rhythm via per-paragraph `space_after` (points): the title carries extra separation below it so it reads as a header, and non-final bullets carry inter-bullet breathing room so the column does not render as a dense block. `space_after` is a general field on the shape-grid `paragraphs[]` cell-text form (points, converted to hundredths of a point), available to any pattern that emits paragraph arrays.

**Bulleted paragraphs.** The `paragraphs[]` form also takes `bullet`: `true` for the default `•`, or a marker string of at most two characters (e.g. `"–"`). It emits a real `<a:buChar>` with a hanging indent (`marL` = −`indent` = 0.65em of the paragraph size, at most 14pt), so a wrapped line aligns with the text, not under the marker. Patterns never prepend a typed `"• "` to paragraph content; `before-after`, `bmc-canvas`, `chart-insights-split`, `image-text-split`, `next-steps`, `scqa-summary` (multi-item), `strategy-house`, `stylish-panels` and `text-sidebar` emit `bullet: true` (go-slide-creator-zieyk). Pattern sizing (`sizedPara.bullet`) measures a bulleted paragraph with the hang's width.

`values.chart` accepts any svggen `DiagramSpec` payload or the flat `{label: value}` shorthand that `chart_value` accepts (e.g. `{"type": "bar", "data": {"Q1": 12, "Q2": 14}}`); the shorthand is normalized to `categories`/`series` at decode time, preserving key order. Validation expands the pattern and dry-renders the chart, so `validate_input` rejects exactly the charts `generate_presentation` would.

`values.chart` is **optional**. When omitted, the pattern collapses to a single-column insights cell at 100% width and emits the structured warning `CHART_PLACEHOLDER_EMPTY: chart-insights-split rendered insights-only; provide a chart spec to fill the left panel` via the `PostExpandWarner` interface. Every surface converts that warning into a `FitFinding` with `code = "CHART_PLACEHOLDER_EMPTY"` and `action = "review"`: `validate_input`, `generate_presentation(fit_report=true)`, `score_deck`, `preview_presentation_plan` and the `validate --fit-report` / `generate --json-output-report` CLI, plus `expand_pattern`'s own `warnings[]`. Until go-slide-creator-wn4v only preview did, so the documented validate → generate loop called a 75%-empty slide clean. Agents should either supply a chart spec or switch to an insights-only pattern (e.g. `card-grid`, `pull-quote`).

`values.chart` is a regular `types.DiagramSpec` — pass the same shape used in slide-level diagram content (`type` + `data`, optional `title` / `style`).

**So-what extensions (go-slide-creator-pzrs).** Consulting chart slides state the figure and the implication, not just the bullets:

- `headline` `{value ≤12, label ≤60}` — a big accent number (32pt, 26pt with ≥5 insights; `overrides.headline_size`) at the top of the insights column.
- `so_what` (≤160) — the takeaway band (see [The takeaway component](#the-takeaway-component)) at the bottom of the column, top-anchored in the right panel when it is the only insight. It is 13pt beside a chart (12pt at 5+ insights), 14pt without one, and no longer carries a "So what:" label — the accent bar marks it.
- Chart caption — single-series charts render without a legend, so the series name used to disappear. A bold caption above the chart now shows `chart_label` (≤60) or, when the chart has no `title`, the single series name plus `unit` (≤12): `"Revenue"` + `"$M"` → `Revenue ($M)`; multi-series charts get `Values in <unit>` (their legend names the series).
- Data labels — `Style.ShowValues` is switched on by default for bar-type charts with ≤16 points and single-series line / area charts with ≤12 points; `overrides.data_labels` forces on / off and an explicit `data.data_labels` payload is left to svggen. The caller's chart spec is never mutated.

With a headline or so-what the insights cell becomes a nested column (headline / insights / so-what rows sized from the measured text) and the vertical divider is omitted; without them the panel is unchanged. On a generated slide `values.source` (like `stat-hero`'s) is lifted into the slide's `source` before layout and renders in the 9pt chrome source zone above the footer, so the chart panel keeps the full height (go-slide-creator-cuszt); only a standalone `expand_pattern` draws the source row, pinned at 30pt so its 12pt floor never autofits.

## Bounds Override

Patterns assume `full_content_area` by default — the grid fills the entire layout content area. For patterns with short content this produces oversized cells. Constrain the grid with:

- **`max_height_pct`** (number strictly between 0 and 100, including fractions): constrains grid height to this percentage of the content area.
- **`bounds`** (object: `{x, y, width, height}` as percentages of slide dimensions): explicit bounding rectangle.

These fields live on `PatternInput` (slide-level JSON) and on the `expand_pattern` MCP tool parameters. When set, the expanded grid gets a `bounds` field on the `ShapeGridInput`, which the shapegrid resolver and density math respect automatically.

`bounds` takes priority over `max_height_pct`. If neither is set, the grid uses the full content area (backward-compatible default). A user `bounds` / `max_height_pct` override clears `vertical_align`; the omitted value means effective `stretch`, so the block stays where the user put it. The expanded JSON does not contain the literal string `"stretch"`.

## Content-sized heights and vertical centring (authoring contract)

Do not let cards / steps stretch to the full content height just because the grid is full-area. Every expanded pattern grid gets `vertical_align: "auto"` (`patterns.ApplyGridDefaults`, applied by `expandPattern` and every other `Expand` caller; `agenda` sets `"center"` itself; a slide `pattern.vertical_align` — `auto` / `top` / `center` / `bottom` / `stretch` — overrides it), and the shapegrid resolver keeps a content-sized block when **at least one row sets `max_height`** (points):

- Cap rows in points, not percentages, derived from `contentAreaPt(ctx)` (e.g. process-flow `0.45 ×` content height), or from a text estimate (`textBlockHeightPt`) for text rows. Point caps keep nested / composed use sane: inside a small compose cell the cap exceeds the cell and the row simply fills it.
- **No stretch-to-fill; cap boxes at 1.6× their content (go-slide-creator-wntyw).** A filled card / tile / pillar / panel is at most `contentStretchMax` (1.6) × its measured content height — or its content plus the minimum padding and insets when that is larger — and the block takes the `auto` placement: it hangs from the template's body placeholder top (`ContentZone.BodyTop`, the line native bullets start on; for a title-only layout, the reference one-content layout's when both draw the same title box) as far as its slack allows, whatever its fill; only on a template without a body line does `auto` centre a block filling over 60% (go-slide-creator-e17xy). Default grid bounds (`ContentZone.ContentTop`) start on the body line as well, capped at 9pt under the title box: a measured short title never pulls a full-height pattern up above the line native bullets start on, and a template whose body placeholder sits lower than that (p-style, modern-yellow, business-template) keeps the title-box start so the zone every schema-maxima pin is sized for (`TestSchemaMaximaStayReadable`) loses no height. Native SWOT / PESTEL / KPI-dashboard / house diagrams hang from their body placeholder top the same way. `kpi-Nup` (`kpiRowMaxHeightPt`: content × 1.6, floor content + 12pt + insets, ceiling the content area), `card-grid` (`contentSizedRow`), `before-after` / `before-after-compact` (header band + bullet panels that hug their lists) and `strategy-house` (bands pinned to their text + 10pt, pillars capped at their tallest title + bullets + card padding) follow it. `kpiBaseCardHeightFrac` (0.70) is only the card-height *estimate* that picks the icon position and default icon footprint; it is neither a floor nor a cap on the rendered card. Never pad a box to satisfy `SLIDE_UNDERUSED`: the content-sized box patterns are judged against a 20% ink threshold instead of 29%.
- **Never emit a row below its own written fit (go-slide-creator-k3eb3).** A pattern that sizes a row (a `max_height` pin, a `min_height` floor, or flex weights) measures the text it will write — the same cell JSON Expand emits, via `writtenFitHeightPt` at the cell's real column width, including baseline nudges and paragraph spacing — and never hands the grid a row shorter than that. A one-line probe or the theme-font model alone is not enough: with the uniform 0.5 cm margin (28pt of every row) the writer stored autofit shrinks that took 12–14pt runs to 9–11.8pt on the short content areas (abstract 687×294pt, modern 851×311pt) and generation refused them. When the rows do not fit, give way in this order: air (row gaps, the space above a band), geometry (state-shift-hub narrows its hub, flex rows are floored at their needs with `floorFlexRowsAtNeeds` so crowded rows take height from sparse ones), then type steps down to the 12pt floor (`next-steps` and `metric-list` take their floor step only when it makes the list fit, keeping the schema-maxima pins where an overflowing list is shrunk either way). What still does not fit is reported by `PostExpandWarnings` as `BODY_TOO_LONG`, measured against the template's content area when `LayoutBounds` is known (character budgets remain the contract without it). Exemplars must fit the shortest shipped content area: `TestShortContentAreaExemplarsStayAboveFloor` and `TestTemplatePatternMatrix` hold that. The same rule covers (go-slide-creator-n1muf): `icon-row` (the card is its caption's written fit plus the top-icon zone; the 45% strip cap gives way up to the content area), `labeled-rows` (row pins are the larger of the theme-font model and the written fit of the label block and body), `matrix-2x2` (quadrant rows floored at their written fit plus any top-icon zone with `floorFlexRowsAtNeeds`; the default 16pt header steps through 14 to 12pt), `process-grid-2row` (header / outcome rows never below their written fit, tracks floored at theirs; the default 14pt row label steps to 12pt) and `process-flow` / `process-flow-compact` (the step row or compact band grows to the tallest label's written fit — the writer measures the full shape bounds whatever the preset's notch — up to the content area). `TestExtremePayloadsWriteAboveFloorOrWarn` holds extreme legal payloads of these patterns on the short areas to "written at ≥12pt or `BODY_TOO_LONG`". `numbered-step-strip` stacked-box / toc rows follow it too (go-slide-creator-ni71s): they were `auto_height` rows estimated from newline counts, so a takeaway or takeaway + source band (which takes 70–100pt off the zone) scaled every row down and the writer stored 12–14pt labels at 6–11.5pt, a refused deck. Each row is now pinned (`min_height` = `max_height`) at the written fit of its tallest cell at the real column widths, measured against `LayoutBounds` — the zone after title, footer and bands — and gives way in order: row gap 6 → 2pt, single-line rows to the writer's clamped one-line margin, then the label (stacked-box 13pt) / title (toc 14pt) to 12pt, taken only when it makes the strip fit. Past that `PostExpandWarnings` reports a measured `BODY_TOO_LONG` naming the need and the area. The character budgets in the schema assume no band: under a takeaway band, five stacked-box rows with bodies (any length) or five toc rows with bodies do not fit the bundled templates, and six or seven rows still hold labels only. `TestNumberedStepRowsReadableOrReportedUnderChromeBands` and `TestPatternBlockStaysAboveChrome` hold it. The n1muf start set follows the same rule (go-slide-creator-n1muf): `stylish-panels` sizes its ribbon and body rows to their written fit and steps bullets 14 → 12pt, then ribbons 16 → 14pt; `team-bios` floors text rows at their written fit and shrinks the headshot row (never below a square that holds its initials) before the name steps 14 → 12pt; `framework-grid` also measures its label band and tightens card padding and row gaps (10 → 4pt, 8 → 4pt) before titles step to 12pt; `contact-directory` floors person rows at their written fit and tightens row and group gaps before reporting; `card-grid` already holds (its budgets are measured against the area). Each of these reports `BODY_TOO_LONG` against `LayoutBounds` when it still does not fit. Size with `writtenNeedOrOverflowPt` rather than `writtenFitHeightPt(…, 0)` when overflowing text must fail a fit check: the latter returns its `minPt` for text still shrunk past its 400pt search window. `TestPanelPatternsReadableOrReportedOnShortAreas` holds the contract. The same rule now holds for `timeline-horizontal` (dots stop rows grow past their 40% cap and the default 14pt label steps to 12pt; chevrons grow past 25%; gantt rows whose label wraps hold their fit and the label column widens 30 → 45%, labels measured at the atomic-token width when the face is substituted) and `pull-quote` (the attribution row holds its fit; the default quote steps 36 → 28 → 18pt before a long quote is left to autofit); `text-sidebar` already sized both columns at or above their written fit (go-slide-creator-n1muf). Single-word labels are atomic tokens too: `scqa-summary` widens its label column to 1.3 : 4, then steps the label down, until every label fits `textfit.AtomicTokenWidthPt`, so "Complication" never breaks mid-word in a substituted face.
- **The written-fit rule for agenda, before-after[-compact], comparison-2col and hero-detail (go-slide-creator-n1muf).** These pinned rows from the theme-font model alone, so legal payloads inside their character budgets were written 6.5–11.8pt on the short areas. Their rows are now floored at `writtenFitHeightPt` (via `rowTextNeedPt`, which reports text taller than the helper's probe as beyond any slide instead of 0), and each gives way in the rule's order, taking a step only when it makes the block fit: `agenda` adds an 18/12pt floor scale after its 28/14 → 18/14 steps; `before-after` tightens the header/body gap 8 → 4pt, then steps the header 16 → 14pt; `before-after-compact` tightens the gap 6 → 4pt, lets its 60% height cap grow to the whole area, then steps the header 14 → 12pt; `comparison-2col` tightens the row gap 8 → 4pt, then steps to a 14pt header / 12pt body, and past the fit keeps its header band while body rows share the rest in proportion to their needs; `hero-detail` sets the hero figure at the larger of 80 / 48pt that keeps the hero within 45% of the area (stepping further when the cards need it), then tightens the gap 10 → 4pt, then steps the label to 14pt and card titles to 12pt, and sizes icon cards to the writer's top-icon zone. Each reports `BODY_TOO_LONG` measured against `LayoutBounds` only when the writer would actually store a shrink (a one-line cell in a short row can still be written whole). `TestAgendaWrittenFitOnShortAreas` and its siblings (abstract, modern, midnight-blue, warm-coral and p-style areas with their theme fonts) hold it.
- Row-list patterns of **unfilled** rows (`exec-summary`, `metric-list`, `labeled-rows`) may open up their spacing with `fillCappedRows` towards a minimum share of the zone, but every grown row stays within 1.6× its natural height; dividers and callouts keep their heights. Sparse agendas and flows promote their default type size; explicit size overrides still win. Use `*-compact` variants when the pattern is supporting context rather than the slide's main content. Pointed flows remain aspect-capped to keep chevron/arrow labels readable.
- **Thin connective geometry (go-slide-creator-7z5we).** Axes, spines and transition markers are rules, not blocks: matrix-2x2 axes are 1.5pt arrows with an 8pt head (`matrix2x2AxisArrowPt`, held by a cell `max_height` / a narrow column), the phase-roadmap timeline and the timeline-horizontal dots axis are `timelineRulePt` (3pt) rules centred in their row (a cell `max_height`), and the before-after transition chevron is a 24pt marker (`beforeAfterChevronPt`, `max_height` + `fit: "contain"`) centred in the gutter. A top `accent_bar` is the card's top rule: flush on the cell's top edge, inside the cell — it never floats above the card or reaches into the row gap above it.
- The body zone starts 18pt (`bodyZoneTitleGapPt`) below a measured top-anchored title's text; the takeaway band is reserved before the block is centred (`reserveTakeawayBand`), so the group centres in the remaining zone.
- Fixed `height` percentages in a grid that has a capped row are kept as absolute shares (no re-normalisation), so the slack is real and the block is centred.
- Grids without any capped row keep the legacy proportional stretch (agenda lists, stacked steps, team-bios rely on it).
- Height-capped, top-anchored pattern `bounds` (`y: 0`, `height < 100`) follow the same `auto` rule (hung from the body line when the template has one; otherwise at most 60% of the area top-anchored, taller centred), or `center` / `bottom` when set; the content area already excludes the takeaway/source chrome band.
- Big single-token values (KPI numbers) must shrink to fit one line (`fitSingleLineSize`) rather than wrap.
- Header bands: fix the row with `min_height = max_height = headerRowPt(...)` (~1.2× the header line height + padding), never a percentage of the grid.
- Cards: size with `contentCardHeightPt(shapeTextHeightPt(...), cardW, hasTopIcon)` and pass sparse card text through `anchorSparseText` so a short body is centred instead of hanging top-left (target: < 30% unused area per card). Skip content-sizing for rows that host a secondary chart.
- **A row's height is the row's, not the cell's.** Every card in a grid row is as tall as the tallest one, and without a `max_height` the row also stretches to fill the content area. Both together put a one-line quote in the top fifth of a tall tinted box (go-slide-creator-pr3g). `contentSizedRow(ctx, cells, cols)` is the shared answer — it sizes the row to the tallest card's own text and runs every sparse card through `anchorSparseText` — and `card-grid`, `quote-cluster` and `icon-row` all go through it. A text-only row (value-chain's descriptions) needs the same cap without the card padding: measure with `shapeTextHeightPt` and set `MaxHeight` directly.
- **A fill of `lt1` is invisible.** `strategy-house` filled its pillars white on a white slide, so a pillar column vanished below its last bullet and the gap to the foundation read as empty space rather than as the pillars holding the house up. Use `surfaceFillJSON(ctx, "subtle", NeutralTint4)` for a panel that must read as a surface (the template's declared surface, else the neutral 4% step; never `lt1`), and `surfacePairJSON(ctx)` for alternating rows.
- **Gaps scale with the template gutter (go-slide-creator-5ms8c).** Author gaps against the engine's 8pt gutter and pass every gap you emit (`gap` / `col_gap` / `row_gap`) or subtract while sizing through `ctx.Gap(x)`; a template whose metadata declares `grid.gutter_pt` scales them by `gutter_pt / 8`, and with no declared gutter `ctx.Gap` returns `x` unchanged. Gaps of 1pt or less and gaps that are drawing geometry (connector channels, axis arrows) stay fixed. `TestPatternGapsFollowTemplateGutter` expands every registered pattern under a 16pt gutter and fails when its largest gap does not double. See [TEMPLATE_SPEC.md](TEMPLATE_SPEC.md#grid).
- **No outline on a filled shape (go-slide-creator-pgdkp).** Every filled shape a pattern emits has `line: "none"` (`noLine`). Separate neighbours with the grid gap (4–6pt white gutters) over a neutral field, or with two neutral steps: `neutralFillJSON(NeutralTint4)` / `NeutralTint8` (dk1 at 4% / 8% via `lumMod`/`lumOff`, so the grey is the template's own ink, never a hard-coded warm or cool grey). matrix-2x2 and bmc-canvas are built from gutters, not borders. Strokes survive only as rules: a 0.5pt row divider, a 1pt header underline, a 2–3pt `accent_bar`, or an unfilled ring. `TestNoPatternOutlinesAFilledShape` expands every registered pattern and fails on an outlined fill; an authored outline on a raw grid (or a card-grid `border` override) reports `FILLED_SHAPE_OUTLINED`.
- **Neutral surfaces, one accent (go-slide-creator-8xsj3).** Pattern surfaces default to the neutral steps (4 / 8 / 16%) with `dk1` text; `accent1` at 100% belongs on the slide's one emphasised element (the highlighted value-chain step, the current maturity stage, the active roadmap phase, the strategy-house objective, a recommended card). Do not paint 40–70% accent mid-tints or alpha ramps as decoration — tint ramps are for ordinal encodings only (capability-heatmap tiers). Chains and stacks (value-chain, arch-stack) are neutral 16% with the highlight or a 3pt bar in the accent. Where a saturated fill remains, choose its ink by measurement at the text's own size (`readableInkOn(ctx, tone, "lt1", TextContrastThreshold(size, bold))`). Never use `dk2` as a fill when it is black: `structuralDarkTone(ctx)` gives `dk2` when it carries a brand hue and `dk1` at 60% otherwise.
- **Keep the template's accents; change the ink, not the fill.** Patterns write `lt1` text on accent fills. On light-accent templates (p-style's oranges) that text is unreadable, and rotation used to replace the fill with `dk2` / black, painting every card black. Now a light accent stays when a theme dark ink reads on it (and the fill still stands out from `lt1` at >= 2:1), and `ApplyReadableInk` swaps the pattern's light text for the first readable ink (`lt1`, `dk2`, `dk1`) after expansion. Author-chosen colours and dark text are never rewritten; translucent fills are skipped.
- **Peer cards are a neutral surface, not a wall of colour.** When two or more text cards of a peer-card pattern (`kpi-Nup`, `card-grid`, `icon-row`, `scqa-summary`, `before-after[-compact]`, `comparison-2col` headers) share one unmodified accent fill, `SoftenPeerFills` swaps them for the neutral 4% surface plus a 3pt top accent rule, before `ApplyReadableInk` darkens their text (go-slide-creator-pymy7). Since go-slide-creator-8xsj3 this applies on every template — dark accents (midnight-blue navy) as well as light ones — so solid accent is left for the one emphasised element. A card that already stands apart (its own `accent_bar`, a different fill such as a recommended / highlighted card) stays solid, as does a lone solid card. Patterns whose fills encode data (heatmap tiers, waterfall bars, scores) are not in the list; add a pattern to `peerCardPatterns` only when its solid cards are equal peers. Under `accent_strategy: "rotate"`, `SharedRotationKey` gives every `kpi-*` pattern one key so sibling KPI patterns never rotate apart, and rotation skips grey slots (saturation < 0.15) and pastel slots under 2:1 against `lt1`.
- **Content starts under the title's text, not under its box.** For a top-anchored title placeholder (`bodyPr anchor="t"`, resolved layout → master → OOXML default), grid geometry measures the slide's title at the template's title size, line spacing and caps, and sets the content zone's `TitleBottom` to the text bottom plus a quarter line (the standard 9pt grid gap follows). A tall p-style title box no longer leaves a dead band above every pattern. Centre / bottom anchored titles, untitled slides and titles that fill their box keep the box edge (go-slide-creator-pymy7). The measurement uses the title's inherited weight (`b="1"`) and letter spacing (`spc`, master titleStyle overridden by the layout lstStyle, carried as `PlaceholderInfo.TextBold` / `CharSpacingHPt`), and when the title face is not available to the measurer (`textfit.FontSubstituted`) it measures a heavy-weight family name ("Segoe UI Semibold") bold and against 90% of the width — erring toward the extra line, because a band reserved for one line when the renderer draws two puts the pattern into the title (blue-corporate's tracked all-caps title, modern-yellow's 44pt Semibold; go-slide-creator-b7qqg.13).
- **A label-only column is a band, not a column.** `arch-stack`'s two cross-cutting rails took 12% of the width each — a quarter of the slide to say "Security" and "Monitoring" — leaving two tall empty columns. A rail is now 4% wide with its label rotated (`"vert": "vert270"` on the text object), which is the consulting convention and gives the width back to the tiers. **The fit detectors understand `vert`:** rotated text is measured against the shape's HEIGHT, so a thin band does not report `TEXT_EXCEEDS_SHAPE`.
- **Reconcile header zones across a row.** Cards in a row are top-anchored, so a header that wraps to two lines while its neighbour's fits on one pushes only that card's body down — three panels meant to read as one comparison come out ragged (measured 19.2pt apart in a rendered card-grid; go-slide-creator-ommn). Measure every header in the row at the card's text width, take the max line count, and pad the shorter ones with blank paragraphs at the header's own size (`alignCardHeaderLines`). Pad BEFORE measuring the row height, so the padding is part of the card the row is sized to.

## Expand conventions

- Emit scheme color strings (`"accent1"`, `"dk1"`), never hex values. Theme resolution happens downstream via `pptx.ResolveColorString`.
- Use `json.RawMessage` for fill and text content fields in `ShapeSpecInput`.
- Default accent is `"accent1"` unless overridden. Use `ctx.ResolveAccent(accent, semanticAccent)` for deck-level accent strategy support.
- Use `ctx.ResolveSurface(role, defaultColor)` for surface tint colors.
- Gap values: 10-12 is typical.
- Geometry values: `"roundRect"`, `"rect"`, `"ellipse"` etc.

## Pre-PR checklist

Before submitting:

```bash
# All must pass
go test ./internal/patterns/... -count=1
go test ./... -count=1 -timeout=120s
go vet ./...
golangci-lint run ./...
cd svggen && golangci-lint run ./...
go build ./cmd/json2pptx
```

To update golden files after intentional output changes:

```bash
UPDATE_GOLDEN=1 go test ./internal/patterns/ -run TestMyPattern/golden
```

Review the diff to confirm the golden change is intentional before committing.
