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
  - A **motif** entry in `patternMotifs` (`internal/patterns/motif.go`, see "Visual motif" below) — `TestEveryPatternDeclaresMotif` fails without one
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
| `dual-org-ladder` | `0` → both org headings; `i+1` → both roles of row `i` |
| `exec-summary` | point → bold lead-in cell (not the support sentence) |
| `hero-detail` | `0` → hero stat (now also takes `accent_bar`); `i+1` → detail card `i` |
| `horizontal-bar-with-callouts` | bar → its callout; the bar label when the row has no callout |
| `journey-maturity-model` | stage → stage step (header box in `columns` / `flat`) |
| `labeled-rows` | row → label block (label + sublabel) |
| `metric-list` | item → big value (not label/detail) |
| `stylish-panels` | panel → body (not the heading / ribbon header) |
| `table-highlight` | option → option-name cell (not the score cells) |
| `team-bios` | member → name/role/bio text cell (not the photo) |
| `timeline-horizontal` | stop → label cell (`dots`), chevron (`chevron`), bar or marker (`gantt`) |
| `value-chain`, `waterfall-bridge` | step / column → label cell |

All other patterns apply the text keys to the one shape the index addresses (roof objective / pillar / foundation cell / beam / roof badge line in `strategy-house` — see [its index map](#strategy-house-and-house_diagram-one-house-builder) — a quadrant in `matrix-2x2`, a card in `card-grid`, and so on). The `kpi-*` family already did this through the same helper (formerly `applyKPICellTextOverrides`).

## Open defaults: containers only where they carry meaning

The native-slide-composition rule (layout nativeness review, 2026-10-03): group with whitespace, headings and hairline rules; a box or fill is drawn only where it means something (the one emphasised element, an org header, a data tier). The patterns below default to an open look and keep their previous look behind a documented style. A pattern converted here must (a) emit unfilled, unlined text cells for its default, (b) keep the old look reachable by one override, (c) keep every authored input valid, and (d) re-measure its text budgets on the new geometry.

| Pattern | Default (open) | Previous look | Emphasis |
|---|---|---|---|
| `icon-row` | A row of accent icons standing on the slide (height 26% of the content area, 40–88pt, at most half a column; `overrides.icon_size` sets it) over a row of top-anchored bold captions (14pt) with an optional `description` line (12pt, ≤80 chars). Both rows are one cell per item, so captions share a baseline. | `overrides.style: "tile"` — neutral tile under an accent rule per item. Items with a `secondary` chart always render as tiles (the chart needs the composite cell). | — |
| `quote-cluster` | Each quote is unfilled text under an opening quote mark in the accent, the italic quote, then one attribution line (bold name, title). The mark is the first thing to give way when the quotes need the height: 28pt, then 18pt, then no mark line and the quote set in quotation marks — so the open cluster holds the copy the tiles held (budgets re-measured: unchanged). Rows are sized to the written fit of their tallest quote, top-anchored so the marks line up. | `overrides.style: "tile"` — tinted rounded tiles with accent names. `overrides.style: "bubble"` — a `wedgeRoundRectCallout` per quote with the attribution under its tail; 7–8 bubbles take four columns (two shapes per quote do not fit three rows). | `quotes[].highlight: true` (at most one): open — an accent tint band and the only accent mark, the other marks dimmed neutral; bubble / tile — the one solid accent fill. |
| `comparison-2col` | Headers are bold left-aligned headings over a 1.5pt accent rule; body rows are unfilled text separated by 0.75pt hairlines. Rules are drawn per column (the gutter stays open) as rule rows between the text rows, and each left / right pair shares one grid row, so rows align across the columns. The grid row gap is zero and each text row grows by its share of the gaps the tiles had: the block is exactly as tall as the tile block plus its rules, so the measured character budgets carry over unchanged. `connectors` keeps the 45 / 10 / 45 gutter and the badge, without the stripe, tint and joining rule. `row_fill` still paints every body row as authored. | `overrides.style: "tiles"` — every cell a filled tile, zebra-striped body rows, softened header tiles. | `rows[].highlight: true` (at most one) tints that row; `overrides.highlight_column: "left" / "right"` gives that column a solid accent header and an unbroken accent tint band. One or the other, never both. |
| `stylish-panels` | A column is one element: a bold left-aligned heading, a 1.5pt accent rule and an open bullet list; columns are separated by the gutter. Rows are heading / rule / body, and the rule with its two half gaps is as tall as the one gap the ribbon and its tile had, so the block height and the measured bullet budgets are unchanged. | `overrides.style: "ribbon"` — a filled ribbon header over a tinted body tile per column. `overrides.ribbon` (`dark` / `accent`) is a ribbon-style option and selects that style when set alone. | `values[].highlight: true` (at most one): that column's heading takes the solid accent — the only solid fill. |
| `before-after`, `before-after-compact` | Each state is a bold heading over a rule and an open bullet list. The two states differ by their rules, not by boxes: neutral (dk1 40%) under the before heading, the accent under the after heading. The transition chevron is kept and spans heading, rule and body. Same row arithmetic as stylish-panels: block height and budgets unchanged. | `overrides.style: "panels"` — a header tile over a body tile per state. | `overrides.emphasis: "before" / "after"` fills that state's heading with the solid accent — the only solid fill. |
| `matrix-2x2` | Two crossing axis lines (1pt, dk1 50%) through the matrix and four open quadrants. Columns are `[y strip 12%, left, axis, right]` and rows `[top, axis, bottom, x strip]` with a zero grid gap, so the lines cross and touch the quadrants. The y strip (HIGH / rotated title / LOW) runs the height of the vertical axis and the x strip (LOW / title / HIGH) the width of the horizontal one; the x strip is content-sized, so the quadrants have more room than the tiles had (budget probe: every field at its schema maximum). | `overrides.style: "tiles"` — four filled quadrant tiles with arrow axes above and beside them. | `<quadrant>.highlight: true` (at most one): an accent tint running up to both axes — the only filled area. |
| `framework-grid` | Each dimension row is a bold label and 1–4 open cards (accent title + body) with no fill; rows are separated by full-width 0.75pt hairlines. The label is top-anchored at the cards' inset so it starts on the card titles' line. The column gap is zero (the cards' text margins keep the columns apart), so cards are a little wider than the tiles were and a highlighted row is one unbroken band. Ragged rows still leave their trailing columns empty. | `overrides.style: "tiles"` — a filled label band and a filled tile per card. | `rows[].highlight: true` (at most one): an accent tint band across the label, the cards and the trailing space. |
| `dual-org-ladder` | No header tiles. Each org is a bold 18pt heading (14pt when a name does not hold one line) on its own 2pt accent rule; each pair is ONE pale neutral band across `[org A, gutter, org B]` (56pt gutter, zero column gap) with a neutral dark `leftRightArrow` layered in the gutter; names (bold 14pt) and roles (12pt) are left-aligned on the heading's edge. Bands take their written fit plus up to 20pt of air and flex to 78pt (88 / 98pt for three / two pairs); wrapped text on four pairs gives up the band's top / bottom margin (14 → 9 → 5pt) before it shrinks. `show_connectors: false` drops the arrow, `accent_b` colours only org B's rule. Four pairs hold any one name or title at its schema maximum; a title over 60 characters beside a name over 51 reports `BODY_TOO_LONG`. | `overrides.style: "lines"` — two header tiles over centred entries joined by a 1pt pairing line (the default before go-slide-creator-kyvk5; budgets about 62 title characters, 47 each when name and title are both long). `overrides.style: "tiles"` — every role a filled card with a stub connector. | `rows[].highlight: true` (at most one): the band takes the accent tint with a 4pt accent edge and the arrow the solid accent (`lines`: an accent tint band across the pair). |

Three more patterns left the tile under an accent top rule (go-slide-creator-mot7a):

| Pattern | Default | Previous look | Emphasis |
|---|---|---|---|
| `card-grid` | `overrides.style: "open"`: a card is three grid rows — a bold `dk1` heading bottom-anchored on a 1pt neutral rule (`dk1` at 60%), then the body; whitespace gutters (24pt) instead of tiles, an 18pt band between two rows of cards. Headings share a baseline because they are their own row, so no filler lines are needed. Text margins are tight (heading 2 / 4pt, body 5 / 2pt) so a card is one element; rows are never under the written fit of their tallest cell. The grid is set at 18 / 14pt when its content area holds it and at 16 (rendered 14) / 12pt otherwise — a tile's text was grown into its padding by the renderer, an open card has no padding to grow into — and `header_size` / `body_size` stand when authored. A grid with height to spare adds up to 8pt of air under each body and between rows of cards, so the block keeps about the footprint of the tiles. A ragged last row is centred as before. `text_budget_guide` and the per-card body budget (`BODY_TOO_LONG`) are still measured on the filled card, the figure that holds in either style; `SLIDE_UNDERUSED` counts an open card by its slot (`openColumnPatterns`). | `overrides.style: "filled"` — the pale tile under an accent rule (the default until go-slide-creator-mot7a). The default also stays `filled` when `card_fill`, `line_color`, `line_width` or `border` is set, or a card carries a `secondary` chart: all of them ask for a card surface. | `cell_overrides` `accent_bar`: that card's rule is the accent, 2pt. |
| `labeled-rows` | `overrides.label_style: "tab"`: the label is a `homePlate` pentagon in the accent's Lighter 80% swatch pointing into its row (12pt point on every tab, whatever the row height), keyword and sublabel in measured ink. | `label_style: "tinted"` — neutral block under an accent rule with an accent keyword. | `label_style: "filled"` — solid accent blocks. |
| `scqa-summary` | Labels are pentagon tabs (12pt point) in the accent's Lighter 80% swatch. | — (no style: the softened accent tiles were never an authored look) | The Answer row, always: a solid accent tab leading a Lighter 90% band. |

**Ink of an open pattern.** `SLIDE_UNDERUSED` and the raw-grid `sparse_layout` estimator count an unfilled text cell by its glyph block. For the patterns in this table (and `card-grid`, `matrix-2x2`, `framework-grid`, `dual-org-ladder`, `agenda`, `swimlane`, `next-steps`, `numbered-step-strip`) the visual unit is the content-sized column or row slot the tile used to fill, so `cmd/json2pptx` counts their unfilled text cells and standalone icons by slot (`openColumnPatterns`, keyed on the grid's `source: "pattern:<name>"` stamp so compose segments are covered). `hero-detail` counts only its accent-ruled detail cards that way (`ruledColumnPatterns`); its hero figure counts by its glyphs. The circular family (`cycle-ring`, `cycle-nodes`, `cycle-intake`, `cycle-figure-eight`, `radial-hub`, `concentric-rings`) counts its label rows by slot as well, and its figure — the layers of the ring cell — as the bounding rectangle of those layers (`figureCellPatterns`): a ring is one shape the size of its circle, not a set of discs. Removing a container does not turn the same content into an underused slide; a sparse payload is still a small block and still reports. `pattern_overcrowded` counts text cells for `comparison-2col`, `icon-row` and `card-grid`, so rule rows, icon rows and ragged-row spacers are not items.

## card-grid shape: the grid follows the cards

`values.columns` and `values.rows` are optional (go-slide-creator-0w4va). `CardGridValues.Shape()` resolves the grid every caller uses (Validate, Expand, `CardGridBodyBudgets`):

| Given | Grid |
|---|---|
| neither | arranged from the card count: 1–3 in one row, 4 as 2 × 2, 5 as 3 + 2, 6 as 3 × 2, 7 as 4 + 3, 8 as 4 × 2, 9 as 3 × 3, 10 as 5 × 2, 11 as 4 + 4 + 3, 12 as 4 × 3, 13–25 on five columns |
| `columns` only | rows = ceil(cards / columns), at most 5 |
| `rows` only | columns = ceil(cards / rows), at most 5 |
| both | the grid only has to hold the cards: fewer cards than `columns × rows` leave the last row short (an unused row is dropped); more is `count_mismatch` naming the room the grid has |

A last row that is not full keeps the card width of the rows above it and is centred (`overrides.last_row: "left"` aligns it left). Centring is done on a grid of twice the columns with every card spanning two, so an odd shortfall still centres; a full grid is emitted exactly as before. The short row is sized like any other: content-sized height, headers padded to one body baseline at the full row's card width. A count the grid cannot hold no longer carries a `wrong_pattern` swap hint — the fix is the grid, not another pattern; the KPI-shaped-headers hint is unchanged.

## card-grid styles + surface overrides

`card-grid` (`cardgrid.go`) exposes two complementary knobs through pattern-level `overrides` (distinct from the per-cell `cell_overrides` whitelist above):

- `style` — visual treatment enum: `open` (default: bold heading on one neutral rule, body beneath, no tile — see "Open defaults"), `filled` (solid accent + light text; several peer cards become the neutral surface + accent rule; the default until go-slide-creator-mot7a, and still the default when a surface override or a `secondary` chart is present), `accent-stripe`, `numbered-badge`, `icon-card` (neutral 4% cards, never white on white), `tinted` (alternating template surface / neutral steps, no outline), `soft-card` (single neutral surface, dark text, explicit no-border line).
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
| Big-number KPIs (2–6 items) | `kpi-Nup` | Fixed count, ≤12-char metrics (a hard maximum; five / six cards hold about 11 / 9 digits on one line on the narrowest shipped template — measured one-line fit warnings); optional per-cell `sub` (delta/trend annotation, aliases `delta`/`trend`/`change`) and `comparator` (≤24 chars, alias `vs`: the reference the number is read against, "vs plan +4 pts"), rendered as its own line in the delta's size and ink and reserved on every card when any card has one. `overrides.style`: `open` (value and caption on the canvas between 0.75pt neutral hairline dividers) or `tiles` (tinted card under an accent rule); unset, plain value + caption cells are `open` and a row with an icon, `semantic_accent` or a non-uniform `cell_accent_mode` keeps its tiles. Every cell's text is top-anchored in the same box, so values and captions share baselines whatever the caption line counts; a delta / comparator sits directly under its caption (cards whose captions differ by one line still share the annotation baseline). In an area too short for the 40/14pt default the row steps to 28/12pt, then 24/12pt, then drops its vertical text margin; a row that still does not fit is refused with `fit_overflow` at the pattern's `values` (see [KPI rows](#kpi-rows-kpi-nup-and-kpi-inline)). `kpi-inline` (height-capped) rejects `comparator`; its `overrides.style` is `open` (default for plain cells), `tinted` (neutral cells under a thin accent rule; default with icons / semantic or per-cell accents) or `solid` (accent blocks) |
| Ranked horizontal bars (3–8) with per-bar insight | `horizontal-bar-with-callouts` | One callout per bar, accent-bar bound to the row; omit every `callout` and the column is dropped so the bars span the full width |
| Single dominant metric | `stat-hero` | One hero number with context; `unit` is a trailing run at 40% of the value size on its baseline (shape-grid paragraph `suffix` / `suffix_size`). The 120pt figure is for a whole slide: in a compose segment, regions cell or grid cell the figure takes the largest size that writes the stack unshrunk with no word of the number wrapped (120 → 40pt at the 18/14pt label/context, then 40 → 28pt at 14/12pt, then without the top/bottom text inset); authored sizes are kept, and a stack that does not fit even then reports `BODY_TOO_LONG` (go-slide-creator-hidji) |
| Stat stack / "by the numbers" list | `metric-list` | 3–7 rows read top to bottom: big right-aligned accent `value` (≤12 chars, one shared size on the 40/36/32/28/24pt ladder — never under 24pt; the value column widens from 28% up to 40% for a long value) + bold `label` + optional `detail`, hairline rules; at most one `highlight: true` row (Lighter-80% accent band + accent bar, value ink measured against the band) and an optional `callout` rendered as the takeaway band (see [The takeaway component](#the-takeaway-component)). Details hold 120 chars through 4 items, ~90 at 5, none at 6–7. Use `kpi-Nup` for side-by-side cards, `stat-hero` / `hero-detail` for one dominant number |
| Agenda / contents page | `agenda` | 2–10 rows: a 28pt serif (`+mj-lt`) accent numeral beside a 14pt item, 0.5pt rules between content-height rows, no filled tiles, the block placed by the shared policy (the numeral steps to 18pt only when the rows do not fit). A short agenda — its rows within 80% of the area with 18pt items, two to four one-line items on the bundled templates — promotes the items to 18pt: the numeral already sets the row height (go-slide-creator-yhzxt). `overrides.highlight` marks the current section on a repeated agenda: that row bold dk1, every other row at 50% opacity (stepped up just enough to keep 4.5:1 / 3:1 on lt1). Optional `values.subtitles` (parallel to `items`, ≤120 chars each) set a muted line 4pt smaller (≥12pt) under each title; it dims with its row. A pale accent falls back to dk2 / dk1 numerals by measured contrast |
| Closing next steps | `next-steps` | 2–6 numbered action rows (`action` ≤90, `owner` ≤30, `date` ≤20; the owner / date column drops when no action has one) under a quiet column header, 0.5pt rules, plus 0–3 `decisions` (≤120) in a "Decisions requested" band (`decisions_label` overrides) drawn as the takeaway band: the dark neutral fill, bold text in measured ink, no outline (see [The takeaway component](#the-takeaway-component)). The closer instead of "Thank you"; use `numbered-step-strip` for a process to explain, `phase-roadmap` for a dated schedule |
| Feature/capability cards | `card-grid` | Multi-line body text per card |
| Sequential process | `process-flow` | Ordered steps carried by their shapes (`overrides.style: "chevrons"`, the default; go-slide-creator-cuq95): a plain `step` is an interlocking arrow — a pentagon opens the flow, each later step is a chevron whose notch tucks round the point before it over a 4pt slanted gap (`bleed_left`; the point is 9% of the step width, 8–18pt) — in a light tint of the accent, with no connector lines. `process-flow` sets a large numeral (`01`, `02`, …; 24 / 20 / 18pt at up to 4 / 6 / 8 steps on a row) over each left-aligned, top-anchored label, so the numerals of a row share a line; decisions are not numbered or counted, and a flow that names a `chevron` or `arrow` step is not numbered. `process-flow-compact` centres bare labels. The solid accent fills one step only — `steps[].highlight` (at most one), else the flow's single `decision` (further decisions get an accent outline). `decision` stays a diamond in its own column; the explicit `chevron` (deep 30% notch) and `arrow` (block arrow) types keep their free-standing geometry and take the same tint. **The flowchart look** — neutral rectangles joined by accent connector arrows, the default until 2026-10 — is `overrides.style: "tinted"`; `"solid"` fills every box with the accent (and applies `cell_accent_mode`). `numbered-step-strip` is the pattern for a short label plus a detail line (a solid numbered ribbon over a detail zone) and `value-chain` for stage names over descriptions (slim neutral arrows); a process-flow step is one statement inside its own arrow. Steps are content-sized (written fit plus 1.25 spare lines where the area has the room, floored at 0.4× the step width, capped at 30% of the content height; a diamond is measured in its inner half-size text rectangle, an `arrow` in its shaft); `process-flow-compact` is shallower (0.28×, capped at 22%) and top-anchored under the title. **Two rows:** 7–8 steps bend onto two rows (`overrides.rows`, 1 or 2; default 2 from 7 steps, else 1; 2 needs at least 4 steps): the first half runs left to right and the second half runs back right to left under it — in the chevron look its arrows are mirrored (shape `flip_h`, labels upright), interlock the other way and carry on the numbering, with a 12pt row gap and no connector (a returning step bleeds only into a step that has a notch to take its point, not into a diamond); in the flowchart look a connector drops from the first row's last step — so a box is as wide as in a four-step flow and holds the four-step budget (80 characters). A flow of only `chevron` / `arrow` steps draws no connectors and wraps left to right like a line of text instead. A flow that mixes pointed and plain steps bends like a plain one, from 7 steps or with `rows: 2`: the connector drops from the first row's last step, the second row runs back right to left, and the chevrons / arrows of that returning row are mirrored (shape `flip_h`) so they point left, the way the flow runs; their labels stay upright. Connectors meet a pointed step on its outline — the tip, the bottom of a chevron's notch, the middle of its top or bottom edge (an arrow's shaft) — and the drop at the turn is written unattached, because a chevron's and an arrow's own top / bottom connection sites sit at the start of the point and bent the drop into a diagonal. Until grid shapes could be mirrored, a pointed step at the turn or on the returning row was refused and a mixed flow of 7–8 steps stayed on one row of narrow boxes (go-slide-creator-r0csi, go-slide-creator-yniru). `rows: 1` keeps 7–8 steps on one row, whose boxes hold about 72 / 71 characters (chevrons 12 / 10) and wrap long labels into narrow columns (`TEXT_WRAPS_NARROW`). `process-flow-compact` is one band and refuses `rows`. **Connectors** (the `tinted` / `solid` flowchart look only) are as long as the step gap, which is the connector's own length rather than the grid gap — 32pt at up to 5 steps on a row, 26pt at 6, and the 20pt minimum at 7–8 (one row), never scaled with the template gutter — on a 2pt accent line with the large arrowhead (`connector.head: "lg"`, about 10pt long and wide). A row of only `chevron` / `arrow` steps keeps the 12pt grid gap and draws no connector. A **decision** whose longest word needs a wider diamond takes a wider column (the other steps give up at most a quarter of their width); a word that still cannot fit is reported as `TEXT_EXCEEDS_SHAPE`. The native `process_flow` diagram shares this look: neutral steps, accent-outlined decisions, the same connector |
| Ordered steps / annotated ToC (no branching) | `numbered-step-strip` | 3–7 numbered steps (chevron ≤6) with an optional per-step detail zone and, in `stacked-box` / `toc`, an optional `steps[].icon` between the number badge and the label; `chevron` ribbon, `stacked-box` scorecard, or `toc` agenda — never emits decision diamonds (use `process-flow` for branching). Stacked-box numbers sit on a neutral lane and toc numbers are unfilled accent numerals; `overrides.style: "solid"` (not `values.style`) restores accent lanes / badges. Stacked-box / toc rows are pinned at their written fit against the zone left after chrome bands (see the written-fit rule). **Detail column:** when every label fits one line in 36% of the label + body width and every body fits one line in the rest (both measured 1.2× up, for the placement policy's type step) and no step is `recommended`, the bodies sit in a detail column beside their labels (`number | label | detail`, one more grid column) instead of on a line under them — stacked one-line rows ended before mid-slide and raised `HORIZONTAL_IMBALANCE` (go-slide-creator-yhzxt). A longer label or body keeps the stacked cell, whose text reaches across the slide. `cell_overrides[i]` lands on the label cell; its text keys apply to the detail cell too |
| Two parallel process tracks | `process-grid-2row` | Two lanes × 3–6 phases sharing the same N columns. Default `overrides.style: "lanes"` (go-slide-creator-06bnr): each track is a pentagon row label pointing into interlocking chevrons (tails tucked under the point before them via `bleed_left`, 4pt slanted gap, no connectors or rules), in PowerPoint's "lighter" swatches of one accent (`row1_color`, default the template's primary fill) — first lane a solid label (shaded until `lt1` reads) over Lighter 70% phases, second lane a Lighter 40% label over Lighter 85% phases, ink measured on every fill; an authored `row2_color` gives the second lane the first lane's depths in that colour. Lanes are content-sized (1.6× the tallest label's written fit, 56–96pt, both equal); phase labels are 14pt when each takes at most two lines, else 12pt; the point gets blunter (down to 6pt) before a word breaks; the label column widens 12 → 28% before the 14pt row label steps to 12pt. Optional `column_headers` (≤24 chars) are bold text standing on the lanes and `outcomes` (≤32 chars) bold accent-ink text under them, both centred over their chevron's text rectangle, no underline or pill. `overrides.style: "tinted"` is the previous default (dark label block, neutral-tint phase boxes under a rule in the track colour, accent-underlined headers, tinted outcome pills, filling the area) and `"solid"` its per-row accent fills |
| Porter / supply value chain | `value-chain` | 4–10 step columns: a row of interlocking arrows (a pentagon, then chevrons whose tails tuck under the point before them via the cell's `bleed_left`) over a 1–3 line description per step. Arrows are a neutral tint and the one `highlight` step takes the accent. Labels wrap at spaces inside the arrow's own text rectangle; the point gets blunter (down to 6pt) before a label shrinks, a label never goes below 12pt, and a word no arrow can hold is reported as `TEXT_EXCEEDS_SHAPE`. `overrides.style: "boxes"` restores rectangular labels joined by small connector arrows |
| Maturity ladder / current-state journey | `journey-maturity-model` | 3–6 stages as an ascending staircase of solid steps: one `rect` per stage standing on a shared floor, each one rise taller than the last (up to 64pt, shrinking to a 10pt minimum when the copy needs the height), filled as a tonal ladder of the accent's "Lighter" palette swatches (`lumMod` / `lumOff`) that deepens up to the solid accent on the `current` stage — the last stage when none is current — while stages ahead of it keep the palest swatch; one dark ink is measured over all tinted steps. Each step carries a 28pt stage numeral over the bold 14pt name; descriptions are unboxed 12pt text under the floor on the step's left text edge, all in one row height so every stage renders at one size. The marker is a bold "We are here" label over a solid pointer in the current step's tone, standing in the air above that step (it reserves height only above the last stages). The step row holds layers-only cells (`step-N`, `marker-label`, `marker-pointer`, `accent-bar-N`); `cell_overrides` text keys act on the step text. `overrides.style: "columns"` restores the earlier default (a header box over a description box per stage, each column starting higher, accent-outlined callout beneath) and `"flat"` equal boxes in one row joined by small arrows; `cell_accent_mode` alternate / progressive paints the non-current steps in their own accent slots |
| P&L walk / cost-driver bridge | `waterfall-bridge` | 3–10 columns of total + delta + subtotal bars; floating deltas with auto-computed subtotals, grey bridge lines between bar levels; `unit` currency symbols render as a prefix (`"$m"` → `$210m`), value labels sit just outside each bar (above rises, below falls), headroom kept under the title; optional `caption` states the scale once, since a bridge draws no value axis; increases take the accent (accent1), decreases its Lighter 50% tint (`negative_accent` sets a solid colour), totals / subtotals dk1 at 38%, and the delta labels are bold |
| Two coupled loops on one path (infinity / DevOps loop) | `cycle-figure-eight` | 4–8 `phases` (`label` ≤ 26, optional `description` ≤ 60) on two lobes side by side; `left_count` of them (2–4, default half rounded up) sit on the left lobe and the rest (2–4) on the right. The path starts at the crossing, runs **counter-clockwise** round the left lobe (up, over the top, back along the bottom), through the crossing and **clockwise** round the right lobe; badges count along it, so the right lobe continues where the left stops. Each lobe is its own lattice cell of [co-located layers](#co-located-layers-when-the-lattice-is-not-enough) (`ringSegmentLayers` / `ringBadgeLayers` with `Prefix` `left-` / `right-` and `NumberFrom`), an open arc built by `newRingArcSpec`, placed as a spine column through its centre (`ringSpinePlacement`, fit `fit-height`); the two squares share an edge, whose middle is the crossing point. The opening at the crossing is the angle at which a lobe's centreline is tangent to the straight line through the crossing point (cos δ = R / 0.5: 39° either side for the regular band), and along those lines run the four **arms** of the crossing — two rotated `rect` layers per lobe (`left-arm-in`, `left-arm-out`, …), exactly as wide as the band, in the band's neutral fill, from just inside the lobe's end segment to just past the crossing point, so the ribbon runs through the crossing in one piece and its only gaps are the ones between numbered segments. A layer's unrotated frame must stay inside its cell, which a full-width arm does only when the ring is a little smaller than its square (`cfeBandRadius`: R = √((0.5 − thickness)/2), a ring of 99% / 98% / 95% of the square for thin / regular / thick), so the two rings stop short of each other and the arms bridge it. A segment with a colour of its own (the highlight, or any segment under a tinted `cell_accent_mode`) keeps it to its end: the arm beside it starts a segment gap away. The direction cue is a small `triangle` (`left-arrowhead` / `right-arrowhead`, dk1 at 60%, longer than wide) on the band of each arm that **enters** a lobe — the two upper arms, mirror images — pointing away from the crossing; never a page-coloured cut-out. Labels are cycle-ring's rows (`cycleRingOutsideRows`, `labelPlaces`): the left lobe's right-aligned to its left, the right lobe's left-aligned to its right, each as near its badge's height as its neighbours allow and at one constant gap from its own lobe's outer edge, so they follow the lobes' curve; optional `left_label` / `right_label` (≤ 16) title the lobes in their holes (18pt down to 12pt, a word never breaks). At most one `phases[].highlight` takes the solid accent; arms stay neutral; `cell_accent_mode` runs over the whole path; `overrides.thickness` as for `cycle-ring`. Budgets: labels 26 at every count; descriptions 60 while each lobe holds at most three phases, 40 once a lobe holds four (7–8 phases, or six split 2 + 4) — `BODY_TOO_LONG` past it; the lobes give up to 30% of their side to the label columns before a row is cut. **Wide only**: the figure needs a content area at least 580pt wide and 140pt tall (two 140pt lobes and a 150pt label column either side). In a narrower area — a horizontal 50% or 60% compose segment, a nested cell — `Expand` returns a `fit_overflow` `ValidationError` at `values` whose fix is `swap_pattern` → `cycle-ring`; validate and generate report the same finding (`slidePatternDiagnostics`). A full-width vertical compose segment is fine |
| Recurring cycle / closed loop of phases | `cycle-ring` | 4–8 `phases` (`label` ≤ 28, optional `description` ≤ 90) as equal segments of one ring, phase 1 starting at 12 o'clock and running clockwise (`overrides.direction: "counter_clockwise"` turns it). The ring is one lattice cell of [co-located layers](#co-located-layers-when-the-lattice-is-not-enough) (a spine column with fit `fit-height` under outside labels, the fit-`contain` square in the legend layout): `blockArc` segments in the dk1 16% neutral, a numbered accent badge on each segment's centreline, and an optional unfilled `center` label (≤ 24, plus `sublabel` ≤ 32) sized from 18pt down to 12pt so no word breaks (each word within 85% of the hole's text width by the theme font's metrics, the room a substituted, wider face needs). Labels are their own lattice cells **outside** the ring, left and right of it: the numeral in its badge's colour — the text ink beside a neutral-dark badge, accent ink beside the highlight's (the cue that ties a label to its badge), the bold label (14pt, stepping to 12pt when the rows need it) and the muted description, each row as near its badge's height as its neighbours allow and at one constant gap from the ring's outer edge at its own height, so the rows follow the ring's curve instead of sharing a column. Each side keeps a label column of at least 150pt (22% of a wide area); phases without descriptions keep only what their longest label needs on one line, so in a split the ring takes the width short labels leave (go-slide-creator-cxidm). At most one `phases[].highlight` takes the solid accent (its badge flips to the page colour); `cell_accent_mode` `alternate` / `progressive` tints each segment with its own accent. `overrides.style: "arrows"` draws `circularArrow` layers chasing each other; `thickness` `thin` / `regular` / `thick` is 14% / 20% / 28% of the diameter. Budgets (label / description): 4 phases 28 / 90, 5–6 phases 28 / 70, 7–8 phases 24 / 50 — `BODY_TOO_LONG` past them. A content area under about 450pt wide (a 50% compose segment, a nested cell) takes `overrides.labels: "legend"` by itself: the ring on the left, one numbered list beside it, descriptions left off (with `BODY_TOO_LONG`) when the list cannot hold them. Use `cycle-nodes` for 3 phases or discrete stations, `cycle-intake` when linear steps feed the loop, `cycle-figure-eight` for two coupled loops, `radial-hub` for unordered items round a centre, `process-flow` when nothing loops back |
| Value / cost driver tree | `driver-tree` | Root metric → 2–4 branches → 1–4 leaves each, drawn as label-sized nodes: a parent is centred on its children and joined to them by elbow connectors, the root is the only solid accent node, and whitespace separates the levels; a branch's optional annotation sits beside its leaves behind a thin rule. `overrides.style: "slabs"` restores root and branch boxes as tall as the rows they decompose into (for **people/role** hierarchies use svggen `org_chart` instead) |
| Temporal sequence | `timeline-horizontal` | Date-labeled stops |
| Layer/stack diagram | `arch-stack` | Vertical tier ordering. A tier that lists `components` (1–12, each ≤40 characters) is a band with its name on the left and one block per component beside it (7–12 wrap to two rows of blocks); once any tier has components the whole stack takes that band layout, a `description`-only tier shows its text where the blocks would be, and the side rails take a tint of the accent. A tier sets `components` or `description`, not both. A stack with no components is drawn as before: tiers are content-sized (the tallest tier's written fit + 8pt, capped at 20% of the content height unless the text needs more) and the stack is centred |
| Narrowing hierarchy | `pyramid` | Visual narrowing (top < bottom) |
| Before/after comparison | `before-after` | Temporal transformation; each state a heading over a rule (neutral before, accent after) and open bullets, chevron between (`style: "panels"` for tiles, `emphasis` to fill one heading) |
| Option/pros-cons comparison | `comparison-2col` | Non-temporal side-by-side; open rows separated by hairline rules and aligned across the columns (`style: "tiles"` for filled tiles; one `rows[].highlight` or `highlight_column` for emphasis); rows are content-sized (the header band at its written fit, body rows grown towards half the content area by at most 1.4× their need, block centred); `overrides.connectors: true` adds a centre gutter with a per-row accent connector badge (left cells get an accent stripe, right cells an accent tint) for "from → to" shifts |
| Recurring loop of discrete steps (plan-do-check-act, feedback loop) | `cycle-nodes` | 3–8 `steps` (`label` ≤28, optional `description` ≤70) as numbered circles on a ring, step 1 at 12 o'clock, joined by grey `circularArrow` links (`arrows: "none"` for plain gaps, `direction: "counter_clockwise"`). Nodes are the accent's Lighter 80% swatch with the number in measured ink, joined by `circularArrow` links in a neutral, their shaft nearly twice the hairline it replaces; a step is numbered once, in its node — the numeral leads the label only on a row whose node shows an icon and in the legend layout; `highlight` (1-based) makes one node the only solid accent; optional `center.label` (≤24) inside the ring. Labels stand outside, each row beside its own node at one constant gap from that node's circle (so the labels follow the ring instead of sharing a column; the label of the node at 12 o'clock stands above its centre line and the one at 6 o'clock below it, clear of the arrows) and led by the step number, the label stepping 14 → 12pt before any row shrinks. `labels: "legend"` sets one numbered list beside the ring — the default layout takes it by itself when two columns would be under 120pt wide (a compose half) — and `labels: "inside"` puts 3–5 labels of ≤14 characters in larger nodes with the number on a small badge (no descriptions). Copy budget on a full slide: 28 / 70 characters with 3–6 steps, 22 / 40 with 7–8; a compose half holds about 28 / 50 with 3–5 steps, a 20-character description with 6–7 and labels only with 8. `BODY_TOO_LONG` names the step that outgrows its row and a centre label that does not fit the ring; an inside label that does not fit its node is `NODE_LABEL_TOO_LONG`, whose executable fix (`remove_key` `labels`) moves the labels back outside. An optional `steps[].icon` (bundled name or `{name|path|url|svg_data}`) is drawn in the node in place of its number, at half the node's diameter and in the numeral's ink (measured on the node's fill, so the highlighted node's icon takes the on-accent ink); the number stays as the cue beside the label, and steps without an icon keep their numeral. Icons are refused with `labels: "inside"` (the node carries the label). No `cell_overrides` |
| Linear lead-in feeding a recurring loop (onboarding then the service cycle, deal intake then the portfolio review) | `cycle-intake` | 1–3 `intake` steps (`label` ≤24, with 3 steps in words of about 9 letters; optional `description` ≤60) as a pentagon and interlocking chevrons on the ring's band tint (the accent's Lighter 80%), on the ring's horizontal centreline, with the descriptions centred under their arrows; a small solid-accent arrow runs from the lane to the ring square's left edge. The loop is 3–8 `loop` phases (`label` ≤26, optional `description` ≤70) drawn by the ring family's builders with phase 1 beginning at 9 o'clock, clockwise: ring segments with numbered neutral-dark badges, or `loop_style: "nodes"` for numbered circles joined by grey `circularArrow` links (node 1 sits at 9 o'clock under the arrow's tip). One `loop[].highlight` is the only solid accent of the loop; optional `center.label` (≤20). The lane takes the left of the slide, so every loop label stands in ONE numbered list right of the ring (accent numeral + bold label 14 → 12pt + muted description, one row pitch: when one row is measured a line taller than the rest and the height does not hold every row at that height, the ring gives up to 15% of its side to the list so the long row sets on the lines of the others). Lane and ring take 17% / 36%, 29% / 33% and 39% / 30% of the width with 1, 2 and 3 intake steps. Loop copy budgets by intake steps 1 / 2 / 3 — label 26 / 26 / 22 (26 with 3 phases); description with 3 phases 70 / 70 / 70, 4: 70 / 70 / 65, 5: 70 / 60 / 45, 6–7: 40 / 30 / 22, 8: one line, drawn only where the content area is about 320pt tall or more (left off on `abstract`, with `BODY_TOO_LONG`). A content area too narrow for lane, ring and a 96pt list side by side (three intake steps in a 50% or 60% compose segment) stacks: the lane across the top with its last arrow above the ring, a down arrow into 12 o'clock (phase 1 then begins there), the list right-aligned left of the ring, intake descriptions left off (`BODY_TOO_LONG`). No `exit` arrow: the list takes the ring's right side |
| Today vs. future state in numbered stages | `state-shift-hub` | Central accent hub circle (`hub_label`, shrinks to fit, floor 12pt) with 3–4 numbered `pairs` (`before` / `after` + optional shared `title` or per-side `before_title` / `after_title`); today items right-aligned on the left, future items left-aligned on the right, nodes on an arc around the hub; optional `left_header` / `right_header`. Use `before-after` for one before/after block, `journey-maturity-model` for a maturity ladder |
| Layers that contain one another (onion, core → adjacent → ecosystem) | `concentric-rings` | 3–5 `layers` ordered inner → outer (`label` ≤ 24, optional `description` ≤ 70), drawn as nested `ellipse` layers of one `fit: "contain"` cell: the rings share their base, so each ring's crest is one ladder row tall and at that row's height. Fills are a ladder of the accent's Lighter swatches (90 / 80 / 68 / 56 / 44%, lightest outside) with one solid accent ring — `highlight` (1-based, default 1 = the core). Labels are never set in the bands (a band of five rings is 29pt wide on the shortest template): every ring has a ladder row to the right (bold label + description, 12pt floor) joined by a hairline leader that ends in a dot in the band; leader pieces and dots take the ink measured on the fill under them. The step between rings at the base gives way (down to circles that touch) when a row needs the height, then the label steps 14 → 12pt, then the square shrinks to widen the ladder; what still does not fit is `BODY_TOO_LONG` naming the layer. Measured budgets: a 24-character label over a 70-character description of realistic copy is written unshrunk at every count on every shipped template (with 5 layers on abstract the square gives width to the ladder); a half-width compose segment or cell holds labels only. `cell_accent_mode` `alternate` / `progressive` paint every ring the solid accent of its index. Prefer `pyramid` for a ranked hierarchy, `radial-hub` for a centre with satellites, `cycle-ring` for a loop |
| One central idea with unordered peers around it (hub and spoke) | `radial-hub` | Solid accent hub circle (`center.label` ≤24 + optional `sublabel` ≤32; the hub grows towards the satellites before its label drops under 12pt) with 4–8 `spokes` (`label` ≤26, optional `description` ≤60) as neutral satellites joined to the hub by thin accent spokes — no numbers, no arrows, no order. A satellite is a disc (0.19 of the ring square) only when it has something to hold: when any spoke carries an `icon` every satellite is that disc, and when none does they are node dots (0.07) at the spoke ends, because a large empty disc reads as a missing icon (go-slide-creator-8hwcw). Labels sit left and right of the ring on the row of their satellite, each at one constant gap from its own satellite's disc (they follow the ring instead of sharing a column); an odd count labels its 6 o'clock item below the ring. `overrides.labels`: `inside` (4–6 spokes, labels ≤14 in words of about 8, no descriptions: the label goes into a larger satellite) or `legend` (ring on the left, keyed list A, B, C … on the right, two columns when one overflows); the default falls back to `legend` when the area is too narrow for two label columns or the ring's hub could not hold its label (a 50% compose segment). `highlight` (1-based) tints one satellite in the accent, thickens its spoke and sets its label in accent ink while the other spokes turn neutral; `overrides.spokes: "none"` drops the lines; `cell_accent_mode` `alternate` / `progressive` fills the satellites with accents. An optional `spokes[].icon` (bundled name or `{name|path|url|svg_data}`) is drawn centred in the satellite disc at 55% of its diameter — a [layer icon](#co-located-layers-when-the-lattice-is-not-enough) — in the ink measured on that disc's fill: the accent where it reads at 3:1 on the tint, else the template's dark or on-accent ink. In the legend the icon is the item's key, in the disc and beside its row, in place of the letter (spokes without an icon keep theirs); icons are refused with `labels: "inside"` (a satellite holds its label only). Budgets by measurement: label 26 / description 60 with 4–6 spokes, 22 / 40 with 7–8; `BODY_TOO_LONG` names the spoke whose row outgrows its room, or the hub label that does not fit its circle; an inside label that does not fit its satellite is `NODE_LABEL_TOO_LONG` (executable fix `remove_key` `labels`: the labels move back outside). Use `cycle-ring` / `cycle-nodes` for an ordered loop and `state-shift-hub` for today/future pairs |
| Options × criteria evaluation | `table-highlight` | 2–6 options × 2–6 criteria rated with Harvey balls (0–4), RAG (`red`/`amber`/`green`) or ≤24-char text per column (`scale` or per-criterion `{label, scale}`); `highlight_row` tints the recommended option (accent 10%) with a 3pt accent bar (`highlight_rows: [i, …]` highlights further rows the same way, each carrying `highlight_label`, for a recommendation that combines options), `highlight_col` tints the decisive criterion (its header stays unfilled, set in the accent when it reads); the table follows the engine table default (go-slide-creator-1iiej) — unfilled 11pt bold `dk1` header over a 1pt `dk1` rule, 12pt rows, 0.5pt `dk1`-15% hairlines, no zebra, no cell gutters, content-height rows top-anchored under the title (`header_size` accepts 11–28pt); the header cells opt out of `type_scale` growth (`type_scale: "compact"` on the cell), so every header label is written at the one header size — growth is per cell, and a short label used to grow to 14pt beside a wrapping one left at 12pt (go-slide-creator-fr538); one legend row per symbol scale (on a table too tall for its content area the legend is set slim, `highlight_label` joins its name's line and an unworded RAG legend is dropped before any text shrinks — see "A `table-highlight` gives up everything but its text" below). **`legend_labels` is `[HIGH, MID, LOW]`** — the full Harvey ball first, the empty one last — and the RAG scale takes its own `legend_labels_rag` in the same order (`[green, amber, red]`, default `["Green", "Amber", "Red"]`). A deck mixing both scales used to reuse the Harvey words for the RAG swatches, so a green dot carried the "does not meet" label (go-slide-creator-z0up). When any Harvey cell scores 1 or 3 the legend lists all five balls: the three `legend_labels` stay on the full / half / empty balls and the quarter / three-quarter balls read "Mostly meets" / "Slightly meets" with the default labels, or stay unlabelled with custom ones (go-slide-creator-csclk.100). RAG uses conventional status colours (`overrides.rag_colors` swaps them) — the one non-theme palette, because status must read the same on every template |
| Activities rated by function (capability / automation heatmap) | `capability-heatmap` | 3–8 function columns, each a pointed `homePlate` header (bold title + optional sublabel, `header_shape: "rect"` for flat headers) over 1–6 activity cells filled by `tier` (0 = accent, 1 = light accent tint, 2 = neutral grey, 3 = lightest grey with a hairline), plus a legend of 2–4 tier swatches with label + optional description (`show_legend: false` hides it). Colour carries the rating, so there is no `cell_accent_mode`. `header_size` / `cell_size` accept 12–40pt; values outside are rejected rather than silently clamped. Prefer `table-highlight` when every row is scored against the same criteria |
| Levers grouped by dimension (framework) | `framework-grid` | 2–6 rows, each a bold label followed by 1–4 open cards (accent title + short body), rows separated by hairline rules (`style: "tiles"` for a filled label band and card tiles; one `rows[].highlight`); the longest row sets the column count and shorter rows leave trailing space empty. `label_width_pct` (10–35, default 18), `title_size` / `body_size` (12–40pt; values outside are rejected), `cell_accent_mode` (per card column). Prefer `card-grid` when there are no row labels, `stylish-panels` for pillars with bullet lists |
| Named risks by likelihood and impact (risk heat map) | `risk-heatmap` | `items[{name ≤40, likelihood, impact}]` (1–20) on a 3 × 3 grid, or 5 × 5 with `size: 5`. A level is a 1-based number (1 = lowest), `low` / `medium` / `high` (`very low` … `very high` at size 5) or one of the grid's own `likelihood_levels` / `impact_levels` labels (lowest first, one per level, ≤14 chars); a level off the grid is rejected with the allowed values. All `size`² cells are drawn; a cell's fill is its band — likelihood × impact of 1–2 low / 3–4 medium / 6–9 high on a 3 × 3 and 1–4 / 5–12 / 15–25 on a 5 × 5 — never an authored colour: a neutral step, a light tint and the solid of the template's `negative` semantic accent (`overrides.accent` / `semantic_accent` change the hue), with the text ink measured per fill. Impact runs bottom-to-top with its title turned along the rows (`impact_label`), likelihood left-to-right (`likelihood_label`); the legend names the bands (`tier_labels` `[low, medium, high]`, `show_legend: false` hides it). Risks sharing a cell stack one per line; names are 14pt and the grid gives up cell padding (10 → 7pt) and then type (12pt, down to 5pt of padding) before it reports `BODY_TOO_LONG` naming the fullest cell. `item_size` 12–20pt holds the size. Prefer `matrix-2x2` for four described quadrants, a `table` for a register with mitigations and owners |
| 4-quadrant positioning | `matrix-2x2` | Axis-labeled quadrants; horizontal title ≤16 chars, vertical title ≤60; each axis is an arrow pointing to its high end (right / up) flanked by low/high end labels — optional `x_low` / `x_high` / `y_low` / `y_high` (≤11 chars, default `Low` / `High`) |
| Phased plan with workstreams | `roadmap-phased` | Period headers form a time axis (an accent segment under each; `current_phase` fills the header of the period the plan is in). A workstream carries `bars`: `{label, start, end?}` or `{label, start, span?}` runs as one bar from its start period to its end period (one period when neither is given), and `{label, start, milestone: true}` is a marker with its label. Bars of one workstream that share a period stack in lanes, first fit in the order written, and the workstream label spans its lanes; rows are as tall as their text, so the block is not stretched over the slide. One-item-per-period `items` still render — each non-empty item as a one-period bar — and `overrides.layout: "grid"` keeps the legacy table of equal tiles for that input. Bars are a light tint of the accent with dark text (`overrides.style: "solid"` fills bars and period headers with the accent; emphasise one activity with `cell_overrides` `accent_bar`). `cell_overrides` indices run: period headers, then per workstream its label followed by one slot per bar (or per item) |
| Single-track phased roadmap | `phase-roadmap` | One band of interlocking phase shapes (a pentagon, then chevrons; no outline, no connector, no timeline rule): the `active` phase is the one solid accent shape, the others a light tint of the accent, each name's ink measured on its fill. Under each phase a panel in the lightest neutral surface (the template's `subtle` surface where it is clearly lighter than the band's tint) holds the date range (bold, 14pt) over the description (12pt; 14pt for up to four phases whose panels have the room; a `- ` line is a native bullet); the panels share one height and grow toward 80% of the content area, at most 2.6× their text. Phases with no date and no description draw the band alone. Milestones are an accent diamond beside a one-line bold label set directly on the band over their phase. Optional `parallel_tracks` (0–4 cross-cutting workstreams, ≤90 chars) render as full-width pointed bars in the band's tint below the panels, each as tall as its own line of text, with a bold `parallel_label` (default "In parallel") at left. Every row is pinned in points: when the block does not fit the content area the milestone row and the panels, then the track bars, give up padding, then the band becomes slimmer (50 → 40 → 32pt), and only then do the phase names step down to 12pt (never below; an authored `header_size` is kept). What still overflows is reported as `BODY_TOO_LONG` per over-long `phases[i].description` with the characters the area holds, or at the last `parallel_tracks[k]` with the number of tracks there is room for. `cell_overrides` indices are unchanged (phases, one reserved index for the former timeline rule, date ranges, milestones, descriptions, track label and bars); a date-range or description override restyles that part of the panel |
| Cross-functional swimlanes | `swimlane` | 2–6 actor lanes × 2–8 step columns. **Default look, `bands` (go-slide-creator-vx7wk):** every lane is one pale band — a row `band` in the 4% neutral, one rectangle behind the row, lanes 2–4pt apart; it takes no row, so lane `i` is grid row `i` for `links` and overlay anchors — headed by a pentagon tab (`homePlate`, neutral 12%, flush with the band's left end) that carries the actor in bold. The tab is 15% of the width, down to 11% when every label needs less on one line and up to 26% when a word needs more (a swimlane in half a slide). Steps are pentagons in the accent's light tint with measured ink, one height for the whole slide, centred in their lane: 80% of the lane (never less than the longest step needs, at most 1.3 times that need and 0.9 of the step's width), and a lane is no taller than that step with its air, so a short flow is a content-sized block that the placement policy composes. A point is 20% of the step's height (5–14pt, at most a tenth of the column); the label's right margin gives back the half point the preset's text rectangle already keeps clear, so a pentagon holds what a rectangle of its size holds. `values.highlight` (`[lane, step]`, a non-empty step) makes one step the solid accent with bold measured ink — the only solid fill. **Arrows:** a step followed by the next column of its own lane gets none (the pentagon points at it); every other transition — another lane, a skipped column, a loop back — gets one neutral 1.5pt arrow with the large head (`dk2` where it carries the brand, `dk1` where `dk2` is black): straight down for a hand-off within one column, an elbow through the column gutter otherwise. The columns end in a zero-width edge column, so the last step stays one gap off the band's right end. When every step and actor fits at 14pt the lane is set in 14pt instead of 12pt; `overrides.header_size` / `body_size` keep authored sizes. Budgets were re-measured on this geometry (`TestSwimlaneBudgetProbe`). **Previous look, `overrides.style: "tiles"`:** lanes between full-width hairline rules (row `rule`s) with the actor as bold unfilled text, grey step tiles at 75% of the lane, and an accent arrow between every two consecutive steps (go-slide-creator-jz5r9, -0e0en); `highlight` fills its tile with the solid accent there too. **Order:** `values.flow` states it as `[lane, step]` pairs (0-based), one per step in the order the process visits them. Without `flow` the arrows follow the columns left to right and, inside a column, the lanes top to bottom, which is the process only when every column holds one step (empty strings elsewhere); a column shared by two lanes with no `flow` is reported as `SWIMLANE_FLOW_AMBIGUOUS`. Empty positions (`""`) are unpainted spacers that arrows skip |
| Executive summary (problem framing) | `scqa-summary` | 4-row Situation/Complication/Questions/Answer narrative arc |
| Executive summary (key messages) | `exec-summary` | 3–5 bold lead-in conclusions (≤90 chars) each with one supporting sentence (≤200 chars) — lead column 45% of the width, and with no supports at all the lead spans the width; every row shares one lead and one support size (the cells opt out of deck type-scale growth), and with a template the overflow warning is the measured fit rather than the character averages — rules between rows, optional `bottom_line` ask (the takeaway band: a full-width dark neutral fill + bold statement — see [The takeaway component](#the-takeaway-component)); answer-first rather than an SCQA arc |
| Keyword-labelled rows (WHY / WHAT / HOW) | `labeled-rows` | 2–6 rows: a label on the left (`label_style: tab`, the default: a pale accent pentagon tab pointing into its row with a bold keyword + optional smaller `sublabel`; `tinted` = neutral-tint block under a thin accent rule with a bold accent keyword; `filled` = solid accent block; `text` = accent-coloured bold keyword with no fill) beside 1–4 lines of `body` (**bold** allowed), rules between content-sized rows. The keyword shrinks as one shared size until no word breaks mid-word. Bodies hold 300 chars through 4 rows, ~190 at 5–6 rows (and 5–6 rows leave no room for multi-line sublabels). Use `exec-summary` when rows are sentence-length conclusions, `metric-list` when each row leads with a number |
| Deck section list | `agenda` | Numbered section outline |
| Visual deck preview | `agenda-with-images` | Numbered agenda rows (unfilled bold accent numerals; `overrides.style: "solid"` restores accent number squares) with image/quote placeholders alongside the title (3–6 items); the placeholder column is all-or-nothing — a row with no `image_label` still gets an empty placeholder |
| Team / 'Our People' page | `team-bios` | 1–8 named people with a headshot (or initials placeholder) + role + short bio, up to 4 per row. The headshot is the same people primitive `contact-directory` draws — a photo cropped into a circle, else a circular accent-tint disc with bold initials scaled to the disc — and name / role / bio centre under it |
| Key contacts / directory | `contact-directory` | 1–4 groups (regions, offices, practices), each an accent heading over a rule, then up to 24 people in rows of `columns` (3–5, default 4): circular headshot (`photo`, drawn with image `geometry: "ellipse"`) or initials disc + bold name + muted title — no bios (use `team-bios` for those). One or two rows of people stack a large headshot above a centred name; denser directories put the headshot left of the text and step type (14→12pt) and headshot size down until the measured block fits — and, without an explicit `columns`, drop to fewer people per row (down to 3) — else report `BODY_TOO_LONG` with the height needed, or naming the person whose name / title word would break mid-word. An area too narrow for even the minimum headshot is an expand error, not a panic |
| Joint-venture / engagement-team paired roles | `dual-org-ladder` | Two org headings on a rule over 2–4 paired roles, each pair one band across both columns (optional pairing arrow per row) |
| Icon + caption row | `icon-row` | Visual categories, 3–5 items; open icons over captions with an optional one-line description (`style: "tile"` for tiles) |
| Photo / case study beside text | `image-text-split` | One `image` (`path` resolved against the deck dir, or `url`; `fit` `cover` (default, crops to fill) or `contain` (whole screenshot / exhibit)) beside eyebrow + heading + body + ≤5 bullets and 0–3 result `metrics`; `image_side` left/right, `image_width_pct` 30–60. Without an image it draws a dashed placeholder (`image_label`). Implements `ImageAssetPattern` so hosts resolve its image like a shape_grid image cell |
| Callout / testimonial | `pull-quote` | Attributed quotation, optionally beside a headshot. The quote and attribution hang flush off the accent rule (left-aligned beside a `left` rule, right-aligned beside a `right` one) and are centred only with `accent_side: "none"` |
| Narrative intro / foreword | `text-sidebar` | Main column (optional heading, 1–4 paragraphs, 0–6 bullets placed after the first paragraph ending in a colon) beside a sidebar panel with one large bold key message (≤200 chars). `sidebar_style` `tinted` (pale accent surface + top accent bar) or `filled` (solid accent); the sidebar ink is measured against the fill at the 3:1 large-text bar. `sidebar_side`, `sidebar_width_pct` (25–40), `body_size`, `sidebar_size`. Short copy is centred; body copy sets at 14pt and steps down to 12pt, then reports `BODY_TOO_LONG` |
| Stakeholder quote cluster | `quote-cluster` | 3–8 attributed quotes in a 3-column grid (voice-of-customer slides): open quotes under a quote mark by default, `style` `bubble` / `tile`, one optional `highlight` |

**The circular family on the DeckSpec path.** `cycle-ring`, `cycle-nodes`, `cycle-intake`, `cycle-figure-eight`, `radial-hub` and `concentric-rings` are reached by one DeckSpec kind, `cycle`, whose `style` (`ring` default, `nodes`, `intake`, `figure_eight`, `radial`, `concentric`) picks the pattern; the kind maps one payload (`phases`, `center`, `highlight`, `intake`) onto each pattern's own values (`internal/semantic/slides/cycle.go`, table in [SEMANTIC_COMPILER.md](SEMANTIC_COMPILER.md)). A new circular pattern needs a style there, its `internal/semantic/reach.go` entry, and a `recommend.go` rule whose keywords cover how a brief names it (`cycle-intake`: "loop fed by", "onboarding intake", "intake steps").

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

## Visual motif (every pattern declares one)

A pattern's **motif** is what its slide looks like at a glance, whatever the pattern is called. `analyze_deck_rhythm` and `score_deck` count motif runs and each motif's share of the content slides next to pattern runs, because a deck that alternates `kpi-4up`, `stylish-panels` and `icon-row` uses three patterns and shows the audience a row of open columns three times (go-slide-creator-rd7oj).

Declare the motif in the `patternMotifs` table in `internal/patterns/motif.go` — one entry per registered pattern; `TestEveryPatternDeclaresMotif` fails for a pattern without one, and `TestMotifStyleKeysAreSchemaStyles` fails when an entry switches on a style the schema does not accept.

| Motif | What the eye sees | Patterns (default look) |
|-------|-------------------|-------------------------|
| `tiles` | a row or grid of filled tiles / cards | `card-grid`, `bmc-canvas`, `capability-heatmap` |
| `open-columns` | peer columns standing open on the slide (a heading, number, icon or portrait over text), no tile | `kpi-2up` … `kpi-6up`, `kpi-inline`, `stylish-panels`, `icon-row`, `quote-cluster`, `team-bios`, `contact-directory`, `before-after`, `before-after-compact`, `comparison-2col` |
| `open-list` | a stack of rows separated by rules or whitespace | `agenda`, `agenda-with-images`, `exec-summary`, `scqa-summary`, `labeled-rows`, `metric-list`, `next-steps`, `numbered-step-strip`, `framework-grid`, `dual-org-ladder` |
| `table` | a grid read by row and column headers | `table-highlight`, `roadmap-phased` |
| `chart` | a data chart | `chart-insights-split`, `horizontal-bar-with-callouts`, `waterfall-bridge` |
| `diagram` | a drawing whose shape carries the meaning | `pyramid`, `strategy-house`, `state-shift-hub`, `cycle-intake`, `cycle-nodes`, `driver-tree`, `matrix-2x2`, `risk-heatmap`, `arch-stack`, `journey-maturity-model`, `concentric-rings`, `cycle-ring`, `cycle-figure-eight`, `radial-hub` |
| `flow` | steps or stops along one line | `process-flow`, `process-flow-compact`, `process-grid-2row`, `value-chain`, `timeline-horizontal`, `phase-roadmap`, `swimlane` |
| `hero-number` | one dominant number | `stat-hero`, `hero-detail` |
| `quote` | one dominant quotation | `pull-quote` |
| `split` | running text beside an image or a sidebar panel | `image-text-split`, `text-sidebar` |

Rules:

- **Derive it from the render, not the name.** Generate the pattern's exemplar on a template and name what it draws today. When a default look changes (tiles → open), change the entry in the same commit.
- **An explicit style that changes the look changes the motif.** List it under `overrideStyles` (an `overrides.style` value) or `valueStyles` (a `values.style` value): `stylish-panels` `ribbon`, `kpi-Nup` `tiles`, `icon-row` `tile`, `comparison-2col` / `framework-grid` / `dual-org-ladder` / `matrix-2x2` `tiles`, `before-after` `panels`, `quote-cluster` `bubble` / `tile` and `kpi-inline` / `process-grid-2row` `tinted` / `solid` are `tiles`; `dual-org-ladder` `lines` is `open-columns`; `numbered-step-strip` `values.style: chevron` and `journey-maturity-model` `flat` are `flow`. A style that only recolours (tinted / solid fills on a pattern that is tiles either way) needs no entry.
- **`chart` and `diagram` are distinctive**: two slides of these motifs are look-alikes only when the same pattern (or chart type) drew them — a pyramid, a house and a driver tree are three different slides. Every other motif forms runs across patterns.
- A hand-built `shape_grid` and a `compose` slide have no motif (`none`) and take no part in motif runs or shares. Content slides without a pattern take theirs from the content: `chart` (with its type), `diagram`, `table`, `image`, else `text`.

`patterns.MotifFor(name, values, overrides)` resolves a slide's motif; `patterns.PatternMotif(name)` is the default look's.

## The tonal system: every default fill plays a role (authoring contract)

Many equal mid-grey rectangles read as generated: nothing is grouped and nothing leads (design audit 2026-10-06, go-slide-creator-x5m8f). A default fill is chosen by the ROLE its shape plays, through `internal/patterns/tonal_system.go`, never by a pattern picking a grey:

| Role | What it is | Fill | Ink | Helper |
|---|---|---|---|---|
| Panel | A backdrop that groups content: a tier band, a pillar shaft, a row-label tile, a lane | The lightest neutral, `dk1` at 4% — or the template's declared `subtle` surface when it is at least 0.05 of relative luminance lighter than the accent's content swatch | measured (`dk1` / `dk2`) | `tonalPanel(ctx, accent)` |
| Content | A shape that IS the content: a step, a tier, a ring segment, a node, a tab, a band of a house | The accent's "Lighter 80%" swatch (`lumMod` 20000 / `lumOff` 80000); a shape that heads or crosses others one rung deeper (Lighter 60%), one that recedes one lighter (Lighter 90%); a ladder runs 90 / 80 / 68 / 56 / 44 | measured on the rung | `tonalContent`, `tonalRung(ctx, accent, pct)`, `tonalInk` |
| Emphasis | The one item the slide is about | The solid accent, deepened where white would not read | `lt1`, else measured | `tonalEmphasis(ctx, accent)` |
| Anchor | A numbered badge that ties a label to a shape | The neutral dark: `dk2` when it carries the brand, `dk1` at 80% when `dk2` is black | page colour | `tonalBadge(ctx)` |

Rules that follow from it:

- **The mid grey (`NeutralTint16`) is not a structural default.** It is kept deliberately in three places only: (1) as the fallback of `tonalContent` / `tonalRung` on a template whose accent tint collides — less than 1.12:1 against the page, or less than 1.6:1 against the solid accent (`tonalAccentCollides`; a yellow accent) — where the ladder is rebuilt from the neutral at 8 / 16 / 24%; (2) in data ladders, where neutral is a VALUE (`capability-heatmap` and `risk-heatmap` low tier, bars that are not the highlight in the chart patterns); (3) in explicit legacy styles (`journey-maturity-model` `flat`, `process-grid-2row` `tinted`, `swimlane` tabs, which sit on a 4% band and need the step).
- **A panel is never darker than the shapes it groups.** That is why a declared `subtle` surface is only kept when it stays clearly lighter than the content swatch (`phase-roadmap`'s rule, now shared).
- **A badge is not an emphasis.** Ring badges are the neutral dark with a white numeral; only the highlighted segment's badge carries the accent (inverted: page-colour disc, accent numeral), and the numeral beside a label takes its badge's colour. `cell_accent_mode` `alternate` / `progressive` is the author asking for accent badges and gets them.
- **Tints are not solid accent.** The accent-restraint gallery counts solid fills only, so a row of Lighter 80% steps beside one solid step passes; two solid steps do not.
- **A tile under an accent top rule is not a default container** (go-slide-creator-mot7a). The defaults are a bold heading standing on ONE neutral rule (`card-grid` `open`), a pentagon tab pointing into its row (`labeled-rows` `tab`, `scqa-summary`), or one panel for a whole zone (`arch-stack` tier band). The tile stays behind an explicit style (`card-grid` `filled`, `labeled-rows` `tinted`, the `tiles` / `tile` styles of the open patterns) and behind `SoftenPeerFills`, which still softens solid accent peer cards of an explicit `filled` grid.

Per-pattern roles: `value-chain` steps, `cycle-ring` / `cycle-intake` / `cycle-figure-eight` segments, arms and intake chevrons, `cycle-nodes` nodes, `radial-hub` satellites, `arch-stack` components and description tiers, `strategy-house` beam and foundation, `scqa-summary` and `labeled-rows` tabs and the native `value_chain` primary activities are content (Lighter 80%); `concentric-rings` is a five-rung ladder ending in the solid core; `arch-stack` side rails, the native value chain's margin and the highlighted `radial-hub` satellite are the deeper rung; `arch-stack` tier bands, `strategy-house` pillars and `roadmap-phased` row labels are panels; the `scqa-summary` Answer row is the emphasis (solid tab, Lighter 90% band).

## Restrained accent defaults (authoring contract)

A solid accent fill is the slide's emphasis, so a pattern spends it on at most one block by default (go-slide-creator-fl11f). Structural cells — roadmap activities, process steps, KPI tiles, detail cards, panel bodies — take the tone of their role in the tonal system above (a panel the lightest neutral, a content shape the accent's Lighter 80% swatch — `inactiveTintTone` is that swatch, as on the steps of `process-flow[-compact]`) with measured ink, and the accent marks structure with rules, connectors and small markers. Keep a legacy all-accent look behind an explicit opt-in (`overrides.style: "solid"` on `roadmap-phased`, `process-flow[-compact]` and `kpi-inline`; `overrides.ribbon: "accent"` on `stylish-panels`, whose ribbons default to the structural dark tone; `overrides.style: "cards"` on `hero-detail`, which defaults to `minimal`; `overrides.style: "solid"` on `agenda-with-images`, `numbered-step-strip` (stacked-box lanes, toc badges) and `process-grid-2row`; `label_style: "filled"` on `labeled-rows`, which defaults to `tab`). Where the fill is the data (heatmap tiers, waterfall totals) it is exempt. `cmd/json2pptx/accent_restraint_gallery_test.go` expands every exemplar and fails when a pattern renders more than one solid-accent block of 0.5in or more; its pending list is empty and must stay so.

The default accent itself is the template's primary fill (go-slide-creator-2mia4): `ExpandContext.ResolveAccent` with the primary (or unset) strategy, and every pattern default that used to hardcode `accent1`, resolves through `ExpandContext.DefaultAccent()`, which returns the first slot of `PrimaryFillCandidates` — the same list `list_templates` reports as `color_roles.primary_fill` (accents passing 4.5:1 against white, then 3:1, then `dk2` / `dk1`), and `accent1` when the theme is unknown. Templates whose accent1 carries white text are unchanged; `warm-coral` defaults to `accent2`, `business-template` to `accent3` and `blue-corporate` to `dk2`. Never hardcode `"accent1"` as a pattern default; call `ctx.DefaultAccent()` (schema `WithDefault("accent1")` stays as the documented common case).

## Cell Accent Variety (authoring contract)

Grid-shaped patterns — those that emit multiple peer cells through the shape grid engine — must support `cell_accent_mode` in their overrides. The contract:

### Grid-shaped patterns (must expose `cell_accent_mode`)

1. **Embed `TextOverrides`** (or the pattern-specific overrides struct that includes `CellAccentMode string`). The shared `TextOverrides` struct in `overrides.go` carries the `cell_accent_mode` field.
2. **Validate** by calling `ValidateCellAccentMode(patternName, ovr.CellAccentMode)` in the pattern's `Validate()` method. This rejects unknown modes with a structured `ValidationError`.
3. **Resolve per-cell accent** by calling `ResolveCellAccent(baseAccent, cellIndex, cellAccentMode)` in the cell-emission loop of `Expand()`. The function returns the accent string for each cell position given the base accent and mode.
4. **Schema** must include `cell_accent_mode` in the overrides object — use the shared helper: `EnumSchema("uniform", "alternate", "progressive").WithDescription(...)`.

`metric-list` (value colour per row) and `labeled-rows` (label-block fill per row) follow this contract.
`concentric-rings` follows it too: the default (and `uniform`) keeps the neutral tint ladder with one solid ring; `alternate` / `progressive` paint every ring the solid accent of its index.
`cycle-nodes` follows it per node: `uniform` keeps every node the neutral tint, `alternate` / `progressive` tint each node, and colour its number, in its own accent.
`cycle-intake` follows it for the loop only (segments or nodes, and the list numerals); the intake arrows stay neutral and the entry arrow keeps the base accent.

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
   row-label column from 12% to at most 22% (28% in the default `lanes`
   style) at the requested label size, and
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
- **Content-structured layouts** (bmc-canvas, agenda, agenda-with-images, roadmap-phased, phase-roadmap, scqa-summary, swimlane, timeline-horizontal, team-bios, quote-cluster, dual-org-ladder, table-highlight, image-text-split, capability-heatmap, state-shift-hub, contact-directory, text-sidebar, cycle-ring, cycle-figure-eight): cell fills are determined by content structure (lanes, phases, sections, member cards, quote bubbles, org-paired rows, highlighted table row/column, rating tier, today-vs-future nodes around a hub, contact groups, the one key-message panel, ring segments in phase order) rather than peer ordering.

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
  content area turns the arc slightly but never distorts a circle. (A ring
  whose labels stand inside its bounding square is a spine cell with
  `fit: "fit-height"` instead: see "Drawing a ring" below.)
- Row connectors cannot draw the spokes: a connector only fans out from a
  row-spanning cell to cells on its RIGHT, so a centred hub would connect to
  one side only. Let the geometry carry the relationship instead.

Each non-grid pattern should document in its `UseWhen`/`NotWhen` text or code comments why it does not expose the override.

### Co-located layers (when the lattice is not enough)

The lattice gives every shape its own rectangle, so it cannot put two shapes
in one place. A ring is exactly that: N `blockArc` segments, their badges, an
optional centre label and arrowheads all share one bounding square. For those
a cell carries `Layers` (go-slide-creator-x1fjb):

```go
ring := &jsonschema.GridCellInput{
	Fit: "contain", // the layers' frame is then the centred square
	Layers: []jsonschema.LayerInput{
		{Name: "segment-1", Frame: jsonschema.LayerFrameInput{X: 0, Y: 0, W: 1, H: 1},
			Shape: &jsonschema.ShapeSpecInput{
				Geometry:    "blockArc",
				Fill:        neutralFill, // one solid accent segment at most
				Line:        noLine,
				Adjustments: map[string]int64{"adj1": 270 * 60000, "adj2": 0, "adj3": 20000},
			}},
		{Name: "badge-1", Frame: jsonschema.LayerFrameInput{X: 0.72, Y: 0.06, W: 0.2, H: 0.2},
			Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: accentFill, Line: noLine, Text: badgeText}},
	},
}
```

- **Frames are fractions of the cell's fitted bounds**: `X`, `Y`, `W`, `H` in
  0..1 of the rectangle the cell's own shape gets — after `Fit`, `BleedLeft` /
  `BleedTop`, `InsetTop` / `InsetBottom` and `MaxHeight`. With
  `Fit: "contain"` frame `(0, 0, 1, 1)` is the centred square, so a ring stays
  round in a compose segment, a nested grid cell or a short content area
  without the pattern recomputing anything. Compute ring geometry in the unit
  square and never in points.
- **Z-order is input order**: the cell's own shape, then `Layers[0]`,
  `Layers[1]`, … Later layers sit on top. Connectors stay behind all cells.
- **A cell may hold only layers.** Its own shape is optional; without one the
  cell is a transparent canvas.
- **A layer shape is a cell shape.** It goes through the same conversion,
  writer and checks: geometry, fill, line, text, `Rotation`, `FlipH`,
  `Adjustments`, the expansion's `MeasureFonts` and type-scale stamps, autofit
  measurement, capacity budgets, readability, geometry and contrast findings.
  Size its text at 12pt or above and give filled layers `noLine`, as for any
  shape.
- **A layer shape takes an icon** (`Shape.Icon`, go-slide-creator-yjsvq): the
  cell-shape icon overlay, laid out inside the layer's frame by the same
  `iconOverlayBounds` — centred at 60% of the frame's shorter side on a
  textless shape (`Scale` overrides), beside or above the text otherwise —
  and embedded as native SVG in the template's colour for its scheme `Fill`.
  Resolve it with `IconRef.Resolve(iconFillOn(ctx, fill, accent), "center")`
  so the ink is measured on the layer's own fill (accent on a tint, the
  on-accent ink on the solid highlight). Icons are pictures and are written
  after the grid's shapes: an icon sits above **every** layer, so put nothing
  over it. Validation, URL / path resolution, alt-text and design-mode
  findings address it at `…/layers/<i>/shape/icon`.
- **Findings name the layer by index**: `<cell path>/layers/<i>`, text at
  `…/layers/<i>/shape/text`. `Name` is a stable id for tests and for the
  `LAYER_FRAME_OUT_OF_CELL` validation error; it is not part of the path.
- **Frames must stay inside the cell**: `W`, `H` > 0, `X + W <= 1`,
  `Y + H <= 1` (0.0001 of slack for trigonometry). A label that must sit
  outside the ring therefore needs a ring frame smaller than the cell, or its
  own lattice cell beside the ring.
- **What a layer is not**: it does not size its row (auto-height and row
  estimates read the cell's own text), it is not a connector / `links`
  endpoint, it does not share its row's autofit shrink, and
  `defaults.cell_style` / `named_style` do not reach it.
- Layers are refused on `table`, `diagram`, `composite`, `pattern` and `grid`
  cells.

Preset angles (verified in LibreOffice on midnight-blue and p-style): 60000ths
of a degree, 0° at 3 o'clock, increasing **clockwise** (90° = 6 o'clock,
270° = 12 o'clock).

- `blockArc`: the band runs clockwise from `adj1` (start angle) to `adj2`
  (end angle); `adj3` is its thickness as a fraction of the shorter side
  (1/100000: `25000` = half the radius, `50000` and above = a full pie
  slice). Defaults `10800000 / 0 / 25000` are the top half. `adj1 == adj2`
  draws the whole ring. `Rotation` turns the shape clockwise; `FlipH` mirrors
  it.
- `circularArrow`: the shaft runs clockwise from `adj4` (start, the tail) to
  `adj3` (where the head begins); the head covers the next `adj2` of angle,
  so the tip is at `adj3 + adj2`. `adj1` is the shaft thickness and `adj5`
  the head's overhang on each side of the shaft, both as fractions of the
  shorter side. Defaults `12500 / 1142319 (19.04°) / 20457681 (340.96°) /
  10800000 (180°) / 12500`: tail at 9 o'clock, tip at 3 o'clock. Keep `adj1`
  at or under `adj5` and `adj5` at or under `12500`: at `adj1 = 2 × adj5` the
  head disappears into the shaft, and at `adj5 = 25000` LibreOffice draws a
  broken outline. `FlipH` makes it run anticlockwise.

**Drawing a ring.** `internal/patterns/ring_common.go` is the geometry (a
`ringSpec`, its items, frames, adjust values, label rows, the lattice) and
`internal/patterns/ring_draw.go` turns it into shape-grid pieces; `cycle-ring`
is the reference user. A circular pattern builds its `ringSpec` (a full ring,
or an open arc for one lobe), fills a `ringPaint` (per-item accents, the one
highlight, badge size, first badge number, a layer-name prefix) and calls
`ringSegmentLayers` (blockArc, or `circularArrow` with `Arrows`),
`ringBadgeLayers` and `ringCentreLayer`, joins them with `ringCell` and places
that cell with `ringLattice`. Labels beside the ring are `ringNumberCell` +
`ringLabelTextCell(ringLabelText(…))`, each row as tall as `ringLabelNeedPt`
says. Segments are the accent's Lighter 80% swatch (`ringBandTone`, the
tonal system's content tone; the item's own tint under a cell accent mode),
the highlight is the only solid accent, badges are the neutral dark
(`tonalBadge`) except the highlight's, and every ink is measured against its
fill.

**Labels keep a constant gap from their circle** (go-slide-creator-70q6f).
A label beside a ring belongs to one circle — its node (`cycle-nodes`), its
satellite (`radial-hub`), or the ring's / lobe's outer edge at its own height
(`cycle-ring`, `cycle-figure-eight`) — and its block (number cue included)
starts `ringLabelGapPt` (12pt, through `ctx.Gap`) from that circle, measured
horizontally where the circle reaches furthest within the row's height
(`ringLabelEdgeX`): right-hand labels are left-aligned and start at the gap,
left-hand labels are right-aligned and end at it. What varies from row to row
is the label's x, never its distance from the circle; do not give a side's
labels one column x. The rules that follow from it:

- The row's x depends on where the spreader put it, and its width on its x, so
  rows are placed with `ringSettleRows`: heights at the narrowest width (beside
  3 / 9 o'clock) always hold, and a row is measured again at the wider cell its
  place gives it. Budgets are therefore the ones of the narrowest row.
- A label that follows the curve stands inside the ring's bounding square, and
  two lattice cells never overlap, so the ring cell is not that square: place
  it with `ringSpinePlacement` — a 4pt column through the ring's centre, as
  tall as the ring, with `fit: "fit-height"`, whose fitted bounds are the
  square of the cell's height centred on the spine. The ring stays round at
  any resolved width; layouts whose labels stay outside the square (legend,
  inside) keep the fit-`contain` square cell.
- Pass every block's near edge to `ringProtectEdges` before `ringLattice`: the
  lattice merges x edges under 1pt apart into the lower one, which would move
  a gap edge that happens to lie beside another cell's edge.
- Where something leaves the circle along the ring (the arrows of
  `cycle-nodes`), set `ringRowsSpec.Outward` so the label of the item at 12
  o'clock stands above its centre line and the one at 6 o'clock below it, and
  give the ring the headroom those rows need (`headroomSide`) instead of
  moving a label away from its circle.

### Circular layouts in split placements

Every circular pattern draws alone on a slide and beside a second zone: a
horizontal compose segment (50%, 60%, on either side), a vertical segment
above a strip, and a `pattern` nested in a shape-grid cell. Each pattern
chooses its own narrow layout from the rectangle it is given — there is no
family-wide switch and no finding for it — and the ring is fitted to a square
in every one of them (a fit-`contain` cell, or the `fit-height` spine of the
outside-label layouts), so the circle stays a circle whatever the cell's shape.

| Pattern | In a narrow area | Smallest area |
|---------|------------------|---------------|
| `cycle-ring` | Under about 450pt wide (a 50% segment, a nested cell) the labels leave their two outside columns for one numbered legend right of a smaller ring (`labels: "legend"` by itself); descriptions are left off with `BODY_TOO_LONG` when the list cannot hold them. A 60% segment on a 13.33in template keeps the outside columns | 330 × 180pt verified at 8 phases |
| `cycle-nodes` | When two outside label columns would be under 120pt each (a 50% segment) the labels become one numbered legend right of the ring; so does a 60% segment on a 13.33in template, while a full-width vertical segment keeps the labels beside their nodes | 330 × 180pt verified at 8 steps |
| `cycle-intake` | When lane, ring and a 96pt list do not fit side by side (most 50% and 60% segments, a nested cell) the layout stacks: the lane across the top, a down arrow into 12 o'clock, the list left of the ring; intake descriptions are left off with `BODY_TOO_LONG`. A full-width vertical segment keeps the side-by-side layout, and eight phases in one shorter than about 200pt report `BODY_TOO_LONG` for the list rows | 330 × 200pt verified at 3 + 8 |
| `cycle-figure-eight` | Refuses: an area under 580pt wide or 140pt tall is a `fit_overflow` validation error at the pattern's `values` whose fix is `swap_pattern` → `cycle-ring`, the same finding from validate and generate. That is every 50% and 60% horizontal segment and every half-width cell on the shipped templates. A full-width vertical segment is fine | 580 × 140pt |
| `radial-hub` | Without room for two 110pt label columns, or when the hub of the ring that fits could not hold its label (seldom with node-dot satellites, whose longer spokes leave the hub room to grow), the labels become a keyed legend (A, B, C … in the satellites, the list on the right). Wider templates keep the outside columns in a 50% segment | 330 × 180pt verified at 8 spokes |
| `concentric-rings` | Keeps its layout: the ring square shrinks so the ladder keeps at least 130pt, the label steps 14 → 12pt, the step between rings gives way. Use labels only; a description that outgrows its row is `BODY_TOO_LONG` | 330 × 180pt verified at 5 layers |

"Verified" is the smallest rectangle the family gate exercises (half of
`abstract`'s content area, and the upper 70% of `modern`'s): the ring there is
still about 120pt or more across and every label is written at 12pt. Short labels
without descriptions are the copy a ring holds beside a second zone.

The gate is `TestCircularSplitMatrixAcrossTemplates`
(`cmd/json2pptx/circular_split_matrix_test.go`): each pattern × six placements
× every template × its smallest and largest count, asserting from the resolved
grid that every round layer's fitted bounds are a square, that no text cell
reaches into a round layer's circle or overlaps another text cell (a label
that follows the curve may stand inside the ring's bounding square, so the
circle is what is tested), that nothing is written under 12pt, and that
validate and generate return the same verdict. A new circular pattern joins
`circularFamily` there (`TestCircularSplitMatrixCoversTheFamily` fails until
it does); the shared helpers are in `circular_helpers_test.go`.

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

- **Native diagrams give up top / bottom padding before type (go-slide-creator-6ne1m).** The native builders of `porters_five_forces`, `nine_box_talent`, `business_model_canvas`, `pestel`, `value_chain` and `pyramid` keep the uniform margin when their text fits at 12pt. When — and only when — it does not, the top and bottom text margins of the diagram's boxes step down `nativeVerticalPadSteps` (0.5 cm → 10 → 7 → 5pt, `internal/generator/native_text_need.go`), the same steps the ruled patterns take, and the space between bullets goes with them. Left and right margins are never tightened. A value chain's support bars are one-line strips and always take the 7pt row padding. A layout measures its boxes with `nativeTextNeedAtMarginEMU` — the height at which the text keeps the margin it declares, on a line 5% narrower than the box (20% when the template's face is host-dependent) — so a box is never sized to a need the writer's own margin clamp would have to rescue.
- **Patterns do not write `inset_*`, with one measured exception.** A pattern cell's text object carries no `inset_left` / `inset_right` / `inset_top` / `inset_bottom`: `shapegrid.ResolveTextInput` defaults every side to the uniform margin. The exception is row padding (below). Otherwise the only pattern-authored inset is a *top offset that aligns first baselines* (`exec-summary`, `chart-insights-split`) or centres a card block (`framework-grid`): it is the uniform margin **plus** the offset, never less. Room a pattern needs for an icon overlay or a ribbon is added on top of the margin (`ResolvedCell.TextInsets`, stylish-panels' ribbon clearance), never instead of it.
- **Rows give up padding before text (go-slide-creator-vg73u).** A ruled list or table at its documented maximum does not fit a short content area at 2 × 0.5 cm of vertical margin per row (six `next-steps` rows alone are 6 × 43pt). When — and only when — a layout does not fit at the uniform margin, `next-steps`, `exec-summary`, `table-highlight` and `risk-heatmap` (its risk cells) write `inset_top` / `inset_bottom` on every cell of every row at the first step of `rowPadStepsPt` (10 → 7 → 5pt, `internal/patterns/row_pad.go`) at which the largest readable type fits (`next-steps` tries the 10pt and 7pt steps before each smaller type step, see below), and share any height left over among the rows (`spreadRowSlack`). `BuildHouse` does the same for the roof's eaves band and the band levels — never the pillars — before the gable goes below its minimum pitch. Left and right margins are never tightened, the row is measured on the cell it writes (`writtenFitHeightPt` reads the declared insets), and a layout that fits at the uniform margin is byte-identical to before. `TestEveryKindRendersAtItsDocumentedCounts` holds every kind to its documented maximum on every shipped template. A `table-highlight` that is over-full even at the last step keeps that step and its writer-measured rows, and reserves each legend row what the grid's scale-down must leave it (`thLayout.reserveLegend`): the text then shrinks by what the table is short of, and the size its refusal reports is that shortfall. Going back to the uniform margin took the same points from far less text height, so a status board 5pt too tall was reported at 6.5pt (go-slide-creator-u8orh). The same bead made the "from four options a row holds no readable detail" budget the answer for an unmeasured layout only: where the content area is measured and the table fits it, the rows are as tall as their written text and a one-line detail under each of five names is written at 12pt with no finding.
- **A `table-highlight` gives up everything but its text before it is refused (go-slide-creator-dwha2).** The last shared padding step left a status board of five rows × three columns, a line of detail under each name and a takeaway 5pt taller than the 253pt content area the shipped templates then left under a takeaway band and a source line (35pt with a `highlight_label` on two rows; 94pt on `modern-template`, whose face wraps the details in the standard option column). That area is 270pt since the band is reserved by the takeaway's line count (go-slide-creator-me53q, below). A table still too tall at the 5pt step takes these steps in order (`thLayout.fit`), stops at the first that fits and shares what is left among the rows; a table that fits at a shared step is byte-identical to before:
  1. **Header padding.** The padding above the header labels goes to 1pt (the row is unfilled, bottom-anchored on its rule and first in the content area): 25 → 20pt.
  2. **Option column.** The column widens by 6 or 12 points of the table width (30 → 36 → 42% at three criteria) when that keeps names and details on fewer lines. It is measured on the written cells, so a width that wraps a criterion label or a text score instead is not taken.
  3. **Slim legend.** Each legend row is set as a table note: a 17pt nested row of 12pt labels (25pt with the sub-grid inset, was 34pt).
  4. **Tag beside the name.** `highlight_label` joins the option name's line (`**Operational** · Breached`, name bold, tag regular, one ink) when every tagged name holds it on one line; tagged rows are then as tall as the others.
  5. **Row padding floor.** Rows go to 4pt of top / bottom margin (`thRowPadFloorPt`; PowerPoint's own table cell margin is 3.6pt). No other pattern uses this step.
  6. **Unworded RAG legend.** A RAG legend row left at its default "Green / Amber / Red" wording, on a table whose author did not set `show_legend`, is dropped: it restates the dots. `legend_labels_rag`, `show_legend: true` and every Harvey legend keep their row.

  What that buys, in points of content area at one-line names, details and header labels: `24 + 37 × options`, plus 25 per legend row that stays — 208pt for five options (233pt with a Harvey or worded legend), 246pt for six (271pt). The shipped content areas under a one-line takeaway band are 259pt (`modern`), 284pt (`modern-template`), 295pt (`blue-corporate`), 298pt (the other five 13.33in templates) and 352pt (`business-template`); a takeaway that wraps to a second line takes 17pt more (the areas were 242 / 267 / 278 / 281 / 335pt under every takeaway before go-slide-creator-me53q), and a source line takes 28pt more (29pt on `business-template`). So five options with details fit every shipped template (on `modern` with a source line only with the default RAG legend), and six fit without a source line on all but `modern`, and with one on the five 13.33in templates with the 298pt area and on `business-template`. A table that still does not fit is `BODY_TOO_LONG` naming that least height and the area's (`needs about 271pt … holds 267pt`, `blue-corporate` with a source line). `TestTableHighlightStatusBoardsFitTheShippedContentAreas` and `TestStatusBoardWithDetailsIsReadableOnEveryTemplate` hold both boards to every template.
- **Degenerate shapes clamp, they do not overflow.** When a shape is too small to hold one line of its text plus 2 × 0.5 cm on an axis — a pill, a number badge or initials disc, an axis or legend label, a thin caption band — the writer shrinks that axis's margin (both sides, proportionally, never below zero) to what still leaves one line at the largest run size (vertically) or the widest word plus its paragraph's `marL` / `marR` with `pptx.WordFitSlack` (5%; `StandInWordFitSlack`, 20%, for a host-dependent face) of room (horizontally — a word handed exactly its width broke its last glyph, go-slide-creator-v74wv), measured against the preset's own text rectangle (the inscribed square of an ellipse, half a diamond, …). The other axis keeps the full margin. This is `pptx.EffectiveTextInsets`; `pptx.GenerateShape` applies it before measuring the autofit scale, so the stored insets and the stored scale agree. Prefer giving a badge or row enough room to keep the full margin: the clamp is a floor, not a layout tool.
- **Estimators use the same numbers.** `defaultShapeInsetLRPt` / `defaultShapeInsetTBPt` / `sizingInsetLRPt` / `sizingInsetTBPt` in `internal/patterns` are the uniform margin; `writtenFitHeightPt`, `internal/textcapacity`, the shapegrid row estimate, the fit geometry detector (`TEXT_EXCEEDS_SHAPE`) and the native-diagram preflight all resolve the writer's text body and apply `pptx.EffectiveTextInsets` (or `pptx.UniformInsetFor`) rather than assuming the OOXML 0.1" / 0.05" defaults. `pptx.AutofitScaleFor` measures the text area left after the declared insets (it no longer subtracts textfit's own 7.2pt sides a second time).
- **Budgets are measured with the margin.** The margin costs ~28pt of height per stacked text row, so every published copy budget, max count and `BODY_TOO_LONG` threshold was re-measured with the `*BudgetProbe` tests; several dense configurations now hold no body / detail / subtitle at all and say so.

## Type scale and typographic finishing (go-slide-creator-30471, -58dhw)

Pattern text uses the five-step type scale in `internal/tokens/typography.go`: `TypeScaleDisplayHPt` 28pt, `TypeScaleLeadHPt` 18pt, `TypeScaleSubheadHPt` 14pt, `TypeScaleBodyHPt` 12pt, `TypeScaleCaptionHPt` 10pt, plus `TypeScaleKPIMinHPt`–`TypeScaleKPIMaxHPt` (40–48pt) for KPI values; `BodyTextMinHPt` (11pt) is the body minimum and `DenseBodyTextMinHPt` (10pt) is for tables and dense matrices. The role ladders in `tokens.go` are ranges on this scale, and the readability floors (`MinReadableHPt`) are scale steps, so nothing here loosens `TEXT_BELOW_READABLE_MIN`.

Name every default size by its step: `internal/patterns/type_scale.go` mirrors the scale (`scaleDisplayPt` … `scaleCaptionPt`, `scaleKPIPt`), and an off-scale default (a 16pt header measured with headroom, a 120pt stat-hero figure) is a named constant with a reason in `offScaleDefaultReasons`. `TestPatternDefaultSizesOnTypeScale` fails on a numeric literal used as a `ResolveSize` fallback, a paragraph `Size` or a size variable, and on an off-scale constant that is not allow-listed. Shrink ladders a pattern walks while measuring (`execSummarySteps`, `metricListScales`, `labeledRowsScales`, `nextStepsScales`, …) name scale steps too, so a ladder measures text at the size it renders and steps from one step to the next (exec-summary: 14/14 → 14/12 → 12/12; metric-list values 40 → 28 → 18pt). `TestPatternFitLaddersOnTypeScale` scans every composite literal assigned or appended to a `*steps` / `*scales` / `*sizes` variable and fails on a literal size or an off-scale step; the only off-scale ladder steps are display figures above 28pt (hero-detail's 80pt hero) or a half-step whose removal would lower a `TestSchemaMaximaStayReadable` pin, each with a reason in `offScaleLadderReasons` (none is needed today). A derived step (a 2pt label bump, a heading body+6pt) is settled with `snapPt`.

**Ratios.** The steps are caption 10 → body 12 → subhead 14 → lead 18 → display 28pt, ratios 1.20 / 1.17 / 1.29 / 1.56: a ~1.2 minor-third ladder through the reading sizes (caption, body, card title) and a wider ~1.3–1.6 jump to the headline and display sizes, so a slide uses at most four sizes and each level reads as a level. 11pt (dense body) is a floor for dense cells, not a display step; KPI values sit in the 40–48pt band (1.43–1.71 × display). Chart text in `svggen` uses the same steps (`svggen/type_roles.go`: title 18, subtitle 14, heading/body 12, labels/captions 10pt; the layout-preset tiers and `ScaleForDimensions`' floors and caps are steps too — a large canvas lifts each role at most to the next step above it), and `internal/tokens/chart_scale_test.go` pins svggen's mirrored steps, roles, presets and caps to the slide scale.

- **No shape depends on a stored autofit scale to fit (go-slide-creator-5x4w4, -217cd).** PowerPoint applies the `fontScale` a shape stores; LibreOffice ignores it and fits the shape again. `shapegrid.Resolve` therefore writes a shrink into the text sizes — the shrink a row's same-size siblings share, and the shrink of a cell with no sibling — and stores no scale. The exception is text the shrink would take under the grid's 12pt floor, or whose runs carry sizes of their own: such a cell keeps its stored scale and validation says so (`fit_overflow` at action `info`, see [FIT_FINDINGS.md](FIT_FINDINGS.md)). A clamped one-line box (a pill, an axis end, a KPI value in a short row) is written with a margin that leaves 1.3 em for its line, where every estimate reserves 1.2 em: a renderer sets a line at its face's ascent plus descent (1.22 em in Carlito), and LibreOffice tightened the line spacing of boxes clamped to exactly 1.2 em (go-slide-creator-bhbtk). `TestRenderTruthExemplars` reads back what LibreOffice does to every exemplar (docs/TESTING.md).
- **Sibling labels share one size.** `type_scale` growth (`growShapeText`) is measured cell by cell, so a row of sibling labels — table headers, column heads, axis ends — comes out at mixed sizes whenever one label has room to grow and its neighbour does not. Set `TypeScale: "compact"` on such cells (as `table-highlight` does on its header row) so the row keeps the size the pattern chose; cells at one declared size also share one autofit shrink (`shareRowAutofitScale`). **Give the row the height a renderer's wrap needs (go-slide-creator-4fz04):** a one-line label in a row one line tall is shrunk by the renderer alone when its face runs wider than the measured one, which the stored scale cannot prevent. `table-highlight` takes its header height at `thHeaderLineWidthPt` — the label's text width divided by 1.12, or the 80% atomic-token width for a substituted face — so a label that wraps there ("Commercial traction" on modern-yellow, abstract, blue-corporate) has its second line and every header stays at the header size.
- **Pick sizes from the scale.** `shapegrid.Resolve` settles every sized cell paragraph onto the step at or below it (`snapShapeTextToScale`, `tokens.SnapTextHPt`), after `type_scale` growth and before the row-shared autofit, so render, preflight and readability checks see the same size. Display figures (≤25 runes containing a digit at 18pt+), a paragraph its pattern marks `"figure": true` at 16pt or more (a value measured to fit one line — the kpi-Nup value; `shapegrid.FigureMinSizePt`), text at 28pt+ and a `design_mode: "free"` deck's own grids (`ShapeGridInput.KeepTextSizes`) are exempt. Snapping only shrinks, so an off-scale constant costs hierarchy, not fit — author on the scale instead.
- **Caps labels are tracked, not spaced.** A paragraph that is a short, bold ALL-CAPS label (≥3 letters, ≤40 runes, ≤18pt) gets +7% letter-spacing (`a:rPr spc`) when the tracked label still wraps to the same number of lines and every word of it holds the tracked line (`trackCapsLabels`). The lines are those of the shape's own text rectangle (an ellipse or a chevron pulls it inside the shape), measured in the template face where the measurer has it (`pptx.ParagraphFitFace`), else in Liberation Sans. Caps labels of one size in one grid are peers: when one of them has no room for tracking, none of that size is tracked (`shareCapsTracking`), so two row labels never read as two styles (go-slide-creator-217cd, -69ums, -5uqfo). Emit the label text in caps; never pad it with spaces. Regular-weight caps (initialisms in body cells such as a next-steps owner "VP CS") are body text and are never tracked (go-slide-creator-y1476).
- **Bold headings and names do not end on a lone word.** A bold, unbulleted paragraph of 3–16 words whose last line holds one word gets a right margin (`a:pPr marR`) in the middle of the measured balanced range (`balanceHeadingLines`, `textfit.BalancedMarginEMU`), measured like caps tracking in the shape's text rectangle and the template face; the line count never changes. The paragraphs after a balanced one state their own side margins (`marL` / `marR`, zero unless they carry one): LibreOffice keeps the margin of the paragraph before for a paragraph that states none, so a body under a balanced heading wrapped short of its column (go-slide-creator-alcw7). Content-layout titles get the same treatment in the generator (`balanceTitleLines`) when their alignment and font metrics are known.
- **One alignment per card.** Centred paragraphs above left-aligned text in the same shape are set left (`unifyCardAlignment`); all-centred tiles and right-aligned figures are unchanged. A header cell stacked over a left-aligned body cell (before-after) is authored left-aligned.
- **Person names stay on one line.** `contact-directory` first searches type step, headshot size and column count for a layout that sets every name on one line, and only then accepts names wrapping between words.

## A pointed shape's text has to clear its own point

A chevron, a right arrow and a home plate all draw their point INSIDE the bounding box, so text laid out to that box is drawn into the notch and the tip. `process-flow`'s chevron step type did this — the first and last characters of a label disappeared into the geometry — and its row connector was drawn straight through the shape (go-slide-creator-czk4; `numbered-step-strip` had the same fix in round 1).

- Set `adjustments: {"adj": chevronAdj}` (30% rather than the OOXML default 50%). The preset's own text rectangle already stops short of the point and the notch; the uniform 0.5 cm margin sits inside it, so a pointed label needs no pattern-authored inset.
- **A right arrow's text rectangle is its shaft, and the preset's shaft is half the shape's height (go-slide-creator-fx48s).** With the uniform 0.5 cm margin above and below, a 60pt `process-flow` arrow step had 2pt of text height and every renderer shrank its label to about 3pt. An arrow that carries text is a block arrow: `adjustments: {"adj1": 70000, "adj2": 50000}` (a shaft 70% of the height under a head half the height long), `inset_top` / `inset_bottom` of 3pt inside the shaft, and a row sized so the shaft — not the shape — holds the label at the written size (`processFlowWrittenNeedPt`; the head's length follows the row height, so the row is re-measured at the height it comes to). The writer and the fit checks measure a `rightArrow` / `leftArrow` in that rectangle (`pptx.SideArrowTextRectSize`), as they do an `upArrow`.
- **`flip_h` mirrors a shape left to right (go-slide-creator-yniru).** `{"shape": {"geometry": "chevron", "flip_h": true}}` writes `<a:xfrm flipH="1">`: the outline is mirrored — a chevron or arrow points left — while the text is not; PowerPoint and LibreOffice both keep it upright, in the mirrored text rectangle. Use it for a pointed step on a row that runs right to left; never rotate the shape 180° instead, which turns the label upside down. Row connectors and `links` read the flag: they leave a mirrored shape at its tip and arrive in its notch (`pptx.RouteOutline`).
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

Both follow the design review's rule language (go-slide-creator-r3gsw, go-slide-creator-7lzdh): serif accent numerals, 0.5pt `dk1`-at-30% rule rows between content-height rows, no tile fills, a block placed by the shared placement policy with surplus height passed to the item rows (`fillCappedRows`, 62% / 55% minimum fill). `next-steps` sets numeral / action / meta at 18/14/14, then 18/14/12, then 14/12/12, and its rows give up padding before the type steps down (go-slide-creator-yqlxf): each type step is tried at the uniform margin and then at the 10pt and 7pt row-padding steps before the next smaller one, and the 5pt margin is kept for a list that fits no other way, so a four- or five-action list keeps 14pt actions in tighter rows instead of dropping to 12pt at full padding. `overrides.action_size` holds the action and decision size instead of the ladder (owner and date 2pt smaller, never under 12). The "Decisions requested" rule fills a 3pt leading column of the grid, flush on the edge the row rules start on, with the band text `TakeawayTextInsetPt` (12pt) to its right; the numeral and header cells span that column (go-slide-creator-le9d0). Six one-line actions and a decision fit every shipped template, and a list that does not fit even at 12pt and 5pt reports the height it needs "at the smallest type scale and its tightest rows". Dimmed text (the non-current agenda rows, the next-steps column header) uses the shape-grid paragraph `alpha` (text opacity in percent, needs an explicit `color`); pick it with `readableDimAlpha` so the WCAG pass keeps the intended ink instead of swapping it.

### KPI rows: `kpi-Nup` and `kpi-inline`

`layoutKPIRow` (`kpi_common.go`) sizes a KPI row to its content and is the one place its type, height and fit are decided.

- **Open strip by default (go-slide-creator-8zles).** Plain value + caption cells are unpainted (`fill: none`, no line): the value in the accent ink that clears 3:1 on the slide background (`kpiOpenInks`), the caption and delta / comparator lines in `dk1`, and every cell after the first carrying a 0.75pt left hairline in `dk1` at 30% (`kpiDivider`; resolved to a colour because an accent bar takes no tint). The strip's column gap is 4.75pt so the hairline sits centred between two cells. Cells that carry an icon, or a row that sets `semantic_accent` or a non-uniform `cell_accent_mode`, keep the container (`kpiDefaultOpen`); `overrides.style` (`open` / `tiles`; kpi-inline `open` / `tinted` / `solid`) picks either explicitly. The grid is still one row of N cells, so cell indices, `cell_overrides` and the peer-fill pass are unchanged.
- **One baseline per row (go-slide-creator-0cy3p).** `kpiText.json` anchors every cell's text to the top of its box (`vertical_align: "t"`) at one shared value size, so values and captions line up whatever the caption line counts; a centred block rode up half a line per extra caption line. When any cell has a delta or comparator, the cells that carry one share its baseline: a shorter caption gets blank caption lines (`kpiCaptionPadLines`) up to the longest caption **among the cells that carry an annotation** — a long caption on a bare card never pushes a neighbour's comparator away from its label — and only while those captions differ by at most one line (`kpiCaptionPadMaxLines`); past that each annotation sits directly under its own caption rather than floating two lines below it (go-slide-creator-18dqh: "SLA is 6 am" under a six-up card). A delta and a comparator take two lines only when some card carries both; otherwise they share the one line under the caption, so a comparator is not set a blank delta line below its label. The row height is the tallest cell's written fit (`writtenFitHeightPt`, plus the top-icon zone) and 4pt of slack — the text is never stored shrunk, which would break the shared size.
- **A lone open row grows into the free height (go-slide-creator-i7yju).** A kpi-Nup open strip that is a slide's own block (no `bounds` / `max_height_pct`, `vertical_align` `auto`, not a compose segment or regions cell) and needs under 75% of the content area is the one block the placement policy grows after its type step: `growsLoneRow` (cmd/json2pptx) sets `shapegrid.Grid.ComposeGrow` for a `pattern:kpi-Nup` grid of one row whose cells are all unpainted, and `grownLoneRow` (`internal/shapegrid/compose.go`) raises the row to half the area (`composeGrowFill`), at most 1.8× the height its content needs (`composeGrowMax`; a filled box stops at the 1.6× of `contentStretchMax`, but these cells are unpainted and the surplus is air between hairlines). The cells' shapes keep their content height — the values still share one baseline and no type changes beyond the step — and are set where the row's tallest text block is centred in the taller row (`Cell.InsetTop` / `InsetBottom`), so what grows is the hairline dividers: a band of figures between rules instead of a strip in the middle of the slide. Tiles (`overrides.style: "tiles"`, icon cells, `semantic_accent`, a non-uniform `cell_accent_mode`) keep their content height, since a filled tile grown the same way is the empty box go-slide-creator-wntyw removed; `kpi-inline` is not grown. A KPI slide whose row still leaves the lower third of the content area empty reports `SLIDE_UNDERUSED` (see [FIT_FINDINGS.md](FIT_FINDINGS.md#slide_underused)).
- **A single-band flow grows clear of the lower third (go-slide-creator-kgfs1).** A `value-chain`, and a `process-flow` whose steps average 40 characters or more, that is a slide's own block and would leave a third or more of the content area under it has its rows grown until the block, set at the optical centre, leaves 28% (`composeBandTarget`): `scalesToBand` (cmd/json2pptx) sets `shapegrid.Grid.ComposeBand` and `bandScaled` (`internal/shapegrid/compose.go`) scales every content-sized row and the row gap by the one factor that reaches it, taking the type step (12 → 14pt) with the taller rows when the text then fits. The growth stops at 1.6× (`composeBandMaxScale`, the `contentStretchMax` of a filled box) for a value-chain's arrows and description row, and at the height where the narrowest step box is a 4:5 portrait card for a process-flow (`ComposeBandSquare`, `composeBandCardAspect`): a box holding a sentence reads as a card up to there and as a slab past it. An interlocking chevron's notch is its `adj` share of the shorter side, so it deepens with the row: `scaledGrid` grows a chevron's `bleed_left` by the same factor and `chevronBleedEMU` holds a bleed deeper than the notch drawn (a chevron grown taller than wide, whose notch has stopped growing) to that notch, so the slanted gap between the steps keeps its width (go-slide-creator-cuq95; this holds for `value-chain` and `cycle-intake` arrows too). Three to five sentence steps reach the 28% on the shipped templates. A block that would need more keeps its height and is reported — six sentence steps on one row, or a flow of one-word labels, which is never grown (a taller box around one word is a slab) and reports `SPARSE_SINGLE_ROW_FLOW`. The text patterns are scaled as a whole instead (next rule).
- **A sparse full-slide exhibit is scaled as a whole (go-slide-creator-cyyiy).** Every other pattern that is a slide's own block — not a kpi-Nup row, a single-band flow, a supporting band or a hero statement (`stat-hero`, `pull-quote`) — is zoomed when its content-sized block needs under 70% of the content area (`composeZoomFill`): `zoomsSlideBlock` (cmd/json2pptx) sets `shapegrid.Grid.ComposeZoom` and `zoomScaled` (`internal/shapegrid/compose.go`) grows every content-sized row and the row gap by the one factor that reaches the 70%, at most 1.6× (`composeBandMaxScale`), and lets the type follow the rows up the zoom ladder 12 → 14 → 18 → 24pt: two steps when the rows grow 1.35× or more (`composeZoomTwoStepMin`), else one, and always the largest step the writer's measure still holds in the grown cells (no word broken, 4% of headroom — the same `stepFit` test as the one-step policy — and no one-line paragraph of up to eight words wrapped, `composeZoomLabelWords`, where the one-step policy holds three: a sublabel stepped past its box ends on an orphaned word). Sizes move per rendered size across the whole grid, never per cell, so text of one role keeps one size across its peers. The caps keep short content from becoming a billboard: the smallest text level stops at 18pt, a level reaches 24pt (`composeZoomHeadingPt`, the one size the render snap leaves off the scale) only over smaller text, display figures grow 1.2× per step up to 48pt, and when no step fits the taller rows the block keeps its type and takes at most 1.35× of row growth (`composeZoomAirMax`) — taller rows around unchanged text are only air. A block between 70% and 75% keeps its rows and takes one step if the rows as they are (or grown to at most 85% of the area, `composeMaxFill`) hold it; a block of 75% or more is dense and untouched, so budgets and `BODY_TOO_LONG` behave as before. A compose segment, a regions cell and a nested sub-grid are not the slide's block and stay content-sized; `vertical_align` other than `auto`, `bounds` / `max_height_pct` and `type_scale: "compact"` keep the pattern's own sizes as for the one-step policy. On p-style `before-after` goes from a 46% block at 14 / 18pt to 65% at 18 / 24pt, `comparison-2col` from 50% to 69% at 18 / 24pt, `labeled-rows` from 12 / 18pt to 14 / 24pt; `icon-row`, `timeline-horizontal`, `stylish-panels` and `process-grid-2row` keep their type (already at the 18pt cap, or pinned) and take the taller rows. A slide still too thin at the 1.6× limit reports `SLIDE_UNDERUSED` with `empty_band_pct`. `TestComposeZoomScalesASparseExhibitAsAWhole`, `TestComposeZoomStepsOnlyAsFarAsTheTextFits`, `TestComposeZoomCaps`.
- **Supporting bands.** `kpi-inline`, `before-after-compact` and `process-flow-compact` (`supportingBandPatterns`) are one content-sized band for a compose segment or a regions cell. Alone on a slide they report `SLIDE_UNDERUSED` — by the lower-third rule or by coverage — with a hint naming the full-slide pattern (`kpi-Nup`, `before-after`, `process-flow`); their exemplars are the only ones allowed to.
- **Short areas step down, then refuse (go-slide-creator-uj9zq).** With no authored `big_size` / `small_size` the row walks `kpiNupSteps` (40/14 → 28/12 → 24/12pt) and takes the first step whose height fits the area it was given (the content area, a compose segment, a regions cell); if none does it drops the vertical text margin to 6pt at the last step. A row that still does not fit makes `Expand` return a `fit_overflow` `ValidationError` at `values` ("4 KPIs need 58pt … but their area is 27pt tall"). `expandPattern` reports it as a located pattern finding; the fit report raises it at `refuse` from the nested cell (`patternAreaRefusals`), so `validate_deck_spec` / `validate -fit-report` say what generation says. Authored sizes are measured as given and refused, not stepped down.
- **A value keeps its line (go-slide-creator-kjrxx).** `kpiFitBigSize` shrinks the row's value size until every value is one line inside 96% of its card's text width (`kpiValueLineFrac`; a substituted template face is measured against 80% of it first). A row in which some value would not stay on one line 1.2× larger (`kpiValueGrowthRoom`, the grid's type step for a display figure) is **pinned**: every cell gets `type_scale: "compact"`, so the grid neither steps nor grows it and the row is written at the sizes it was measured at. Left to the step, "EUR 48.2M" was written a step larger than it was fitted at; LibreOffice, which does not apply the stored autofit scale, broke it at the space, shrank that card alone and left the row at two sizes with its captions out of line. A value that is not one line even at the 16pt floor is `BODY_TOO_LONG` on `values[i].big`, and the message says how many of its characters the card holds ("holds about 8 characters like these") — on a DeckSpec slide that is `max_chars` at `/slides/N/kpis/i/value`, on a raw deck `fix.params.max_chars` at `/slides/N/pattern/values/i/big`.
- **The value budget is per KPI count and template (go-slide-creator-6xgxm).** 12 characters is the hard maximum (`max_length`), not what a card holds: `KPIValueLineBudget(ctx, n)` is the digits one card of an n-KPI open strip holds on one line at the 16pt floor in the template's body face — the measure the finding applies, so that many digits are never reported and one more is. On the shipped templates two to four KPIs hold the full 12; five hold 11 and six hold 9 on the narrowest wide-faced ones (abstract, blue-corporate, modern-yellow; modern-template holds 10 with six), which the `kpi-5up` / `kpi-6up` schema, the `kpi_snapshot` field description and `list_slide_kinds` budgets state (`kpis[].value` is a `measured` budget there, with the per-count digits in its note). Capitals and "M" run wider than digits; the finding quotes the count for the value's own glyphs.
- **A long value stays above its caption (go-slide-creator-6xgxm, -a5ogo).** A value is written at the size it was fitted to, down to the 16pt floor: its paragraph carries `"figure": true`, which exempts it from the grid's type-scale snap (the snap would set a 16–17pt value in 14pt, the size of a subhead) and holds for a value with no digit, so one row of values is one size. When the fitted value is under the 18pt lead step the caption (and with it the delta / comparator lines) takes the 12pt body step, whatever `small_size` says: bold accent 16pt over regular 12pt. A row of shorter values is unchanged. A row with a value that does not fit one line even at the 16pt floor is written at the 14pt subhead step (where the long value has the better chance of staying whole) and reports `BODY_TOO_LONG`, which quotes both: "cannot fit on one line at the 16pt minimum … so the row's values are written at 14pt".
- `kpi-inline` keeps its 24/11pt sizes. Its bar is one content-sized row, as tall as its text needs (a bar with no measurable text is capped at 25% of the area), with no bounds box of its own, so on a slide of its own it is composed like every other sparse block (28/14pt at the optical centre, go-slide-creator-yhzxt) instead of hanging from the top of a 25% band; `pattern.vertical_align: "top"` keeps the bar under the title. It is a supporting band: alone on a slide it leaves about 41% of the content area empty beneath it and reports `SLIDE_UNDERUSED` (go-slide-creator-i7yju) — use it as a compose segment or regions cell, or use kpi-Nup for a slide of its own.

### Row-list patterns: `metric-list` and `labeled-rows`

Both are content-sized row stacks separated by 0.75pt hairline rule rows, sized like `exec-summary`: they try a descending type scale, keep the largest whose measured natural height fits the content area, and pass surplus height to the content rows (`fillCappedRows`, 68% / 60% minimum fill, each row at most 1.6× its natural height). Overrides that set a size pin the scale. What they cannot fix is reported from `PostExpandWarnings`: `TEXT_EXCEEDS_SHAPE` for a value (metric-list) or a keyword word (labeled-rows) that still breaks at the floor, and `BODY_TOO_LONG` when the stack is taller than the content area at the smallest scale.

`metric-list` values are the hero of the row and stay on their ladder (go-slide-creator-1vmsk): `metricListScales` walks value/label 40/18 → 36/18 → 32/14 → 28/14 → 24/14 → 24/12 (six or seven rows start at 32/14) and takes the first step at which every value fits its column on one line and the list fits its area. A step whose longest value would wrap is skipped rather than shrunk between steps, and with no `overrides.value_width_pct` the value column first widens from 28% up to 40%. A list that does not fit at 24/12 drops the rows' top / bottom text margin from the uniform 0.5 cm to 4pt (`inset_top` / `inset_bottom` on the row cells) before anything else; one that still does not fit keeps its 24pt values and reports `BODY_TOO_LONG` naming the row count ("7 rows need …pt with 24pt values …"), at validate and at render alike. `overrides.value_size` is not a ladder step: it is kept as authored and shrinks continuously (floor 16pt) only so its longest value stays on one line.

`metric-list` sets `col_gap` to 0.1pt, not 0: a highlighted row tints both of its cells, and a real gap (0 resolves to shapegrid's 8pt default) shows as a white seam through the band. The gutter comes from the two cells' uniform text margins instead, and each highlighted cell is outlined in its own band colour (lumMod / lumOff, which shape lines honour; tint they do not) so no hairline shows.

## The takeaway component

One component carries the "so what" of a pattern (go-slide-creator-7b5o6, redesigned in go-slide-creator-3a1rm): chart-insights-split `so_what`, exec-summary `bottom_line`, metric-list `callout` and the next-steps "Decisions requested" band. Pattern surfaces build it with `patterns.TakeawayRows` (`internal/patterns/takeaway.go`); next-steps fills its own label + bullets cell from `patterns.TakeawayBandTone`.

**The default is a band.** The one sentence the slide exists for used to be 14pt bold beside a 3pt bar — the weakest element on the page. It now closes the block as a filled band:

- **Fill:** the template's dark structural neutral, never an accent — `dk2` where it carries the brand (midnight-blue's navy, abstract's slate, warm-coral's brown, forest-green), `dk1` at 85% where `dk2` is black (p-style: a charcoal one step off the black of the type). The band can therefore close a slide that already spends its one solid accent block; `TestAccentRestraint*` does not count it.
- **Ink:** `readableTextOn` the fill (`lt1` on every shipped template), bold, 14pt (`TakeawaySizePt`; chart-insights-split sets 13pt / 12pt in its narrow column and 18pt for a lone callout). The band opts out of grow-to-fill.
- **Geometry:** one `rect`, no outline, text 12pt in from both edges and centred on the band's height, 8pt above and below the lines (`TakeawayBandPadPt`); sized to its measured lines, budgeted to two.
- **Width:** the full width of the block. `TakeawayRows` returns cells of the HOST grid — a spacer row, then the band spanning the host's columns — so the band is flush with the rules and tiles above it. A nested grid stands 4pt in on each side (`SubGridInsetPt`), which read as a band that missed its margins. In chart-insights-split the band lives in the insights column, itself a nested grid, and rides in one `TakeawayRow`.
- **Air:** 16pt above the band (`TakeawayGapAbovePt`): the spacer row is 16pt less the host's row gap above and below it, and is omitted when the host gap is 8pt or more.
- The band row is an auto-height row floored at its measured height, never a `max_height` pin (see below). `TakeawayRowHeightPt` is what the takeaway claims in its host, spacer and inner gap included; `TakeawaySqueezePt` is the part a host may give up before text shrinks.

**Variants** — `overrides.takeaway_emphasis` on exec-summary / metric-list / chart-insights-split:

- `bar` — the previous default: a flush 3pt accent bar (accent1, or the pattern's resolved accent) on the left of 14pt bold `dk1` text, top-anchored, no fill and no outline.
- `subtle` — the bar plus a 5% `dk1` tint behind the text.
- `strong` — a solid accent band with measured-contrast ink. It IS a solid accent block: do not use it on a slide that already has one.

**Still the bar** (same constants, `TakeawayBarPt` / `TakeawayInk` / `TakeawayPadPt`):

- The pattern / compose envelope `callout` (`cmd/json2pptx/pattern_resolve.go` `appendCalloutRow`, `TakeawayEmphasisBar`): a nested-grid row under any pattern, whose `emphasis` takes `subtle` / `strong`, and `italic` / `bold-italic` for the (always bold) text. A pattern callout also shrinks the pattern's expansion bounds by the band's height before expanding. `TakeawayRow` stays an auto row floored at its height because any max on a row switches the host grid from stretching its rows to content-sized rows, which shrank auto-height hosts to their estimates (`numbered-step-strip` rows are pinned at their written fit).
- The slide `takeaway` band (`internal/generator/takeaway_note.go`), which is chrome above the footer rather than part of a pattern. It is reserved by its line count (go-slide-creator-me53q): the chrome frame measures the takeaway in the template's body font at the layout's real band width (`template.ResolveChromeFrameForTakeaway`, the one call preflight and generation both make) and reserves 23.5pt for one line or 40.5pt for a takeaway that wraps (7.5% of a 7.5in slide, less one 17pt line). A takeaway within 4% of the line end reserves the second line, so a renderer's substituted face cannot wrap it into a one-line band. At least 12pt separates it from the source line or footer (`template.ResolveChromeFrame`). On a layout whose background would leave `dk1` unreadable, it inks in the theme colour the chrome contrast check picks. `examine_template`'s `profile_geometry` shows the frame for a one-line takeaway and a source line.

Moving these two to the band is a follow-up: both are described in `skills/generate-deck`.

The chevron "BOTTOM LINE" flag, the peach accent-tint band with a 1pt accent outline, the solid accent metric-list banner and the solid accent callout strip are gone.

## Peers share one type size

A designer picks one size for a set of peers — the rows of one SCQA, the people of one directory row, the cards of one grid. A fit made shape by shape sets only the cells that need it smaller (or leaves only the sparse ones larger), and a slide whose peers differ by a point or two is the clearest sign that a program set it (go-slide-creator-riyh7). Two engine steps used to work per cell or per row and now work per peer group (`internal/shapegrid/peer_growth.go`):

- **Grow-to-fill** (`type_scale` `comfortable` — what every DeckSpec deck and the pattern gallery use — and `presentation`). Each shape still measures how far its own text may grow (`shapeGrowScale`), but the peers of a group take the SMALLEST of their answers (`sharePeerGrowth`) before any size is written. A group grows together or not at all; a shape that cannot grow (its text fills its box, its role is at its cap, its pattern pinned it `compact`) holds its group.
- **The shared autofit shrink** (`shareRowAutofitScale` / `writeSharedShrink`). The shrink the longest label needs was shared along a row; it is now shared across the row AND the peer group, so one long activity in a heatmap no longer sets its row at 10.8pt between rows at 12pt.

**Who is a peer** is read off the resolved geometry, so patterns and authored grids get it with nothing to declare. Two text shapes are peers when they have the same preset geometry, their first paragraphs are set alike (size and weight), and their frames stand as peers: the same size (swimlane steps, wherever their lanes put them), or a shared row or column edge with one dimension in common (cards of a row; content-sized cells of a column; the steps of a staircase, which share their foot and width; ranked bars, which share their left and height). Peer groups are the connected sets of that relation — a 3 × 2 card grid is one group. Fill is not part of it: the highlighted card grows with its neighbours.

**What a pattern still owes:**

- **Peers in separate nested grids are invisible to the resolver** — each nested grid is resolved on its own. Pin such shapes with `TypeScale: peerTextTypeScale` (`compact`): the labels and values of `horizontal-bar-with-callouts` (one sub-grid per bar) and the component blocks of `arch-stack` (one sub-grid per tier) do. The same goes for peers the geometry cannot relate.
- **Sizes a pattern picks itself are picked per role over the whole set** (kpi-Nup values, metric-list's value ladder, exec-summary's lead / support, journey-maturity-model's description row): never per item.

`TestPatternPeersShareOneTypeSize` (cmd/json2pptx) is the gate. It generates every pattern exemplar and an uneven variant of it — one item of every list grown 4×, 2× or 1.3× toward its schema maximum, the first the pattern can still hold — under each type scale on midnight-blue and the local p-style, reads the slide XML and fails when (a) one role of one peer group (same geometry, fill, outline; paragraph index, weight, slant, ink) is written at more than one size, or (b) peers that are one size under `compact` are several sizes under a grow-to-fill scale. `PEER_KEEP=<dir>` keeps the decks. `SIBLING_SIZE_MISMATCH` remains the validate-time warning for the case no file can fix: a renderer whose face runs wider re-fitting one cell of a row.


## Text on a tinted fill must be chosen by measurement too

The same rule applies to the text a pattern paints INSIDE a fill it tints itself. `timeline-horizontal` tints each bar of its gantt and chevron chains — shade 70000 at the first stop through tint 40000 at the last — and hardcoded `lt1` inside every one of them, so the lightest bar measured **1.54:1** in a real midnight-blue render and its date label was invisible (go-slide-creator-5qotm).

- Ask `readableTextOn(ctx, tone, fallback)` (`internal/patterns/fill_contrast.go`) which of the light / dark text roles reads on the fill's EFFECTIVE colour. Without a theme, `timeline-horizontal` uses `dk2` on tinted links and `lt1` on darker links; a portable expansion must not bake white text onto a pale tint.
- Build the fill from a `fillTone` and emit it with `tone.fillJSON()`, so the tone you measured and the fill you paint cannot drift apart.
- **One left edge (go-slide-creator-svrpx).** Unfilled, unoutlined, left-aligned first-column text (exec-summary lead-in numerals, next-steps numerals, agenda numerals, `text`-style labeled-rows keywords) starts on the title's text edge: the grid receives `ContentZone.TextLeft` (title placeholder X — or the side-decor-shifted content column — plus the title's resolved `lIns`, layout → master → 91440) and `alignFirstColumnText` reduces those cells' left inset to reach it (never increases it; explicit `inset_left` and icon reservations are kept). Filled cards keep their padding. On a title-only layout the takeaway / source bands use the title column (`ChromeFrame.Basis` `layout_title`), the same span `titleOnlyContentZone` gives the pattern, instead of the reference layout's body column.
- **Brand-coloured display text is judged per run (go-slide-creator-tinsz).** The render-time shape-grid contrast pass holds neutral inks (lt1 / dk1 / white / black) to the body's smallest-text bar so one cell never splits white and black, but judges a non-neutral colour per run (`a:rPr` / `a:defRPr` / `a:endParaRPr` block) at that run's own size and weight: a 40pt accent value clearing 3:1 keeps the accent while its 11pt label is fixed, and a large accent that misses 3:1 on a light fill is lerped darker in its own hue rather than snapped to the palette. Sibling grouping and per-fill harmonisation follow the same per-run bars. Softened KPI peer cards (`SoftenPeerFills`) draw their ≥24pt figure in the accent, or the minimal linear-light darken of it that clears 3:1 on the neutral surface.
- **Shade the fill before blackening the type (go-slide-creator-v9tup).** When `lt1` misses the bar on a mid-tone accent the pattern itself draws (abstract #8E8172, warm-coral #E64A19, p-style #FD5108, blue-corporate #55BC7E, business-template #AD84C6), `accentFillAndInk(ctx, tone, minContrast)` returns the accent deepened by the smallest `a:shade` (linear-light keep ≥ 45%) at which `lt1` clears it, keeping the hue, theme link and white type. `ApplyReadableInk` does the same for every shape whose text is only light ink. Pale accents (lt1 below 1.8:1), tints, translucent fills (alpha < 80%) and shapes that mix dark and light ink keep their fill and take the first readable ink `lt1 → dk2 → dk1`. Hex fills are returned as the shaded hex, since the shape-grid resolver honours modifiers on scheme colours only.
- **A fill no theme ink reads on is deepened too (go-slide-creator-pr5bx).** `readableTextOn` answers the best of `lt1 → dk2 → dk1` even when none clears the bar. On a template whose darks are a soft charcoal (`dk1` = `dk2` = #2E353A) a plain #FD5108 block got `dk2` at 3.77:1, and the light-ink rule above never saw it. `ApplyReadableInk` now also takes a shape whose theme ink (`lt1` / `dk1` / `dk2`) fails while no theme ink reads on its fill: the fill gets the same smallest `a:shade` and every theme ink in the shape becomes `lt1`. `timeline-horizontal` chooses its inks itself and solves its gradient as one chain (`timelineGradientChain`, go-slide-creator-c22bn): the ramp runs monotonically from `a:shade` 70000 at the first stop through the plain accent at the midpoint to `a:tint` 40000 at the last (both values are the share of the accent kept), a shaded link no ink reads on is lightened toward the next link when a dark ink reads before it gets there and is otherwise deepened for `lt1` within the same `shadeMinKeep` floor, with the links before it keeping their proportion, and a tinted link no ink reads on is lightened for dark ink, so the chain still gets lighter at every step. A theme with a black `dk1` always has a reading ink, so nothing changes there; tints, neutrals and pale accents are left alone as above. `TestPatternInkContrastOnLocalTemplateCorpus` (cmd/json2pptx) expands every pattern under every enum override on every local template and on a soft-darks stand-in, and fails on any theme-ink text below its bar.
- `tint` and `shade` are **linear-light mixes** toward white and black, not the HSL lightness that `lumMod` / `lumOff` act on. `EffectiveColorMods` models all four; both transforms are pinned against measured render pixels in `internal/patterns/timeline_contrast_test.go`. A model that treats a tint as a lumMod is out by 30-50 per channel, which is the difference between "readable" and "invisible".
- In `timeline-horizontal` chevrons, measure the label and body against the chevron's usable text width and capped row height. Emit `BODY_TOO_LONG` with the available body-line count when the description would clip; a raw character limit misses narrow seven-stop layouts.
- Chevron `body_size` controls both the emitted paragraph and its fit budget. Sizes below shape-grid's 12pt rendering floor are measured and emitted at 12pt, including the default derived from `label_size`.
- Chevron dates use that same 12pt rendering floor for their one-line row height. A date that wraps at the effective font size and template width receives a `BODY_TOO_LONG` fit finding instead of relying on a fixed character count.

## timeline-horizontal `gantt`: bars on a time axis (go-slide-creator-o34er)

`style: "gantt"` used to draw every stop as one full-width bar: a point milestone, a three-month range and a seven-month range were the same length, with no axis. The style now reads `date` / `end_date` as dates (`internal/patterns/timeline_dates.go`) and lays the track out as a time axis (`timeline_gantt.go`):

- **Grammar.** `2026-03-15`, `2026-03`, `2026`, `Mar 2026`, `March 2026`, `Mar '26`, `5 Mar 2026`, `Mar 5, 2026`, `05-Mar-2026`, `Q1 2026`, `2026 Q1`, `2026Q1`, `H1 2026`, and the yearless forms `Mar`, `Apr 30`, `Q2`, `H1`. Each names a period (a day, month, quarter, half-year or year). Yearless stops are ordered among themselves in authored order (`Nov`, `Dec`, `Jan` runs on across the year end) and the axis prints no year; a yearless stop beside dated ones cannot be placed.
- **Bars and markers.** A stop with `end_date` is a bar from the start of `date` to the end of `end_date` (`Oct 2026` → `Dec 2026` is three months). A stop without `end_date` is a diamond marker at the middle of the period its `date` names; give `end_date` (it may equal `date`) for a bar.
- **Columns are date segments.** The grid's columns are the label column (30%, widening to 45% for wrapped labels) followed by one column per segment between consecutive ticks, bar edges and marker edges, with a 0.01pt column gap, so a cell's `col_span` is exactly the dates it covers. Empty stretches are spacer cells (`{"col_span": N}` with no content), which the resolver keeps as footprint. A bar edge within 2pt of a tick shares its boundary; the shortest bar is 6pt and a marker 14pt.
- **Axis.** A row of unit labels, each starting at its tick, over a 0.75pt rule. The unit is the finest of days, weeks, months, quarters, half-years, years (then 2 / 5 / 10 / 25 / 50 years) that covers the stops in at most 8 divisions of at least 46pt; the year is printed on the first label and on each January / Q1 / H1 (`Oct '26`, `Nov`, `Dec`, `Jan '27`).
- **Date text.** The `date → end_date` text sits inside a bar that holds it on one line, else beside the bar (after it, or before it when there is no room after), measured at the size the writer writes it at. Unfilled track text (axis labels, dates beside a bar) carries a 3pt side margin and no vertical margin so it starts at the tick or bar edge.
- **Dates that cannot be read.** A value that is not a date (`Summer`, `TBD`), an `end_date` before its `date`, or a yearless date beside dated stops is not an error: the row keeps its label and date text but gets no bar, and `PostExpandWarnings` emits one `TIMELINE_DATE_UNPARSEABLE` line naming every `values[i].date` / `.end_date` it could not place. With no stop placed there is no axis either. Never draw a bar of invented length.
- **Density.** `collectGridOccupancyFindings` counts a gantt as two cells per stop; the segment columns are geometry, not content.

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

## strategy-house and house_diagram (one house builder)

The `strategy-house` pattern, the DeckSpec `pillars` kind that compiles to it, and the native `house_diagram` are drawn by one builder, `BuildHouse` in `internal/patterns/house_builder.go` (go-slide-creator-vlef3, -bjxb9, -x25dq). Each caller maps its input onto a `HouseModel` — a roof plus levels, top to bottom — and places the returned grid; `TestHouseDiagramMatchesTheStrategyHousePattern` fails if the two outputs differ for the same content. Change the house in the builder, never in a caller.

**Shape.** The roof is one gable pentagon (`upArrow` with `adj1` 100000, so there is no seam between gable and eaves) in the slide's accent; the objective — under the roof-badge line when `roof_badges` are given — sits in the eaves band, the preset's own text rectangle. Under it come the levels: an optional `beam` band, the pillar row (the panel surface, accent title, `dk1` bullets; a 4pt accent rule only where `cell_overrides` `accent_bar` asks for it), and the beam and foundation levels (bands in the accent's Lighter 80% swatch, `HouseStyle.BandFill`, bold labels in measured ink). A pillar without bullets beside bulleted ones is top-aligned with them.

**`foundation`** is a string (one band — the shape every existing deck uses) or a list of 1–3 levels, top to bottom, where each level is a string (a full-width band) or a list of 2–5 short strings (a row of equal cells, ≤40 characters each):

```json
"foundation": ["One operating model in every market", ["People", "Technology", "Data"]]
```

Validation refuses, with the counts, more than 3 levels, more than 5 cells in a level, and levels whose cell counts cannot share one column grid with the pillars (the least common multiple of the pillar count and the split levels' counts must be at most 24 — 5 pillars over a 3-cell and a 4-cell level is refused).

**Heights come from the content, not from percentages.** Every band is pinned to its measured text (`writtenFitHeightPt`), the pillar row to its tallest column. What is left of the region is spent, in order, on a gable at the minimum pitch (rise = width / 20), on breathing room around the text, on the designed pitch (width / 12, at most 22% of the region height) and on pillars up to 1.2× their content. A house short of height gives these back in reverse order: breathing room first, then the gable down to a floor (width / 40), and only then the pillar rows, whose text is then written smaller. `PostExpandWarnings` reports the last two states as `BODY_TOO_LONG` (roof flattened; pillars squeezed), which is how a house in a region too small for it — a narrow `compose` segment — is reported rather than clipped. A house whose levels leave less than the minimum-pitch gable first tightens the top / bottom text margin of the eaves band and the band levels (`rowPadStepsPt`, 10 → 7 → 5pt; the pillars keep the uniform margin) and only flattens the gable when the tightest bands still leave too little (go-slide-creator-vg73u).

**`cell_overrides` indices.** `0` roof objective, `1..N` pillars, then the foundation cells in reading order, then the beam (when present), then the roof-badge line (when `roof_badges` are present). A house with one foundation band and no beam keeps the indices it always had (`N+1` foundation, `N+2` badges). `accent_bar` on index 0 draws a thin rule under the eaves.

**Advisory `HOUSE_SHAPE_FORCED`** (go-slide-creator-qad87, see [FIT_FINDINGS.md](FIT_FINDINGS.md)) nudges when a foundation band joins three or more short items with separators, or a pillar has no body beside pillars with three or more bullets. The exemplar and the DeckSpec `pillars` example are deliberately not the 3 × 2 silhouette: four uneven pillars over a band and a three-cell level.

**Native `house_diagram`** maps `roof` / `sections` / `floors` / `foundation` onto the same model (`docs/diagrams/house.md`); six or more sections in a row switch to the dense type sizes. The native path has no template metadata, so its pillar surface is always the neutral 4% step where the pattern uses the template's declared `subtle` surface.

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

**A horizontal segment is its share of the envelope** (go-slide-creator-uhe09). The width is divided once: one `gap` between neighbouring segments, the rest by `size_pct` (`horizontalSegmentWidths`). A segment's own lattice columns are weighted inside that width (`mergeHorizontalIn`), so a 60% segment is 60% of the room whether its pattern draws in one column or in twenty, and the rectangle a pattern is expanded in (`composeSegmentBounds`) is the rectangle it is drawn in. Counting a gap per lattice column instead gave a ring lattice at 60% two thirds of the slide, and told a pattern whose lattice changes with its width (a ring's labels moving into a legend) a width it did not get.

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
- kpi-Nup icons default to an accent-sized footprint (top: 1.1× the value size, at most 45% of the card width, directly above the top-anchored value; left: ≤ 40% of card height / 20% of width) by setting the overlay `scale`; an authored `scale` wins.
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
    Type       string    // "sparkline" | "bar" | "line" ("bar_chart" / "line_chart" are aliases)
    Values     []float64 // 2–12 numeric data points
    Categories []string  // optional x-axis labels; length must match Values when set
    Color      string    // optional hex/scheme color override
}
```

**Caps (enforced by `validateSecondaryChart`):**

- At most one secondary per cell (a single pointer field, not an array).
- `type` is restricted to `sparkline`, `bar`, `line`; `bar_chart` and `line_chart` are accepted aliases of `bar` and `line`.
- `values` must be 2–12 numbers.
- `categories`, when set, must have the same length as `values`.

**Expansion.** When `Secondary` is set, the cell's base `Shape` is wrapped via `wrapCellWithSecondary` into a `CompositeInput{Text: <original shape>, SubDiagram: <built diagram>, Split: "top", Ratio: 0.6}` so the existing text+styling renders on top and the chart below. `sparkline` is mapped to a `line_chart` `DiagramSpec` with `Style.ShowLegend = false`; `bar_chart` and `line_chart` pass through.

When adding the same slot to a new grid-shaped pattern, reuse `SecondaryChartSchema()`, `validateSecondaryChart`, and `wrapCellWithSecondary` rather than duplicating their logic.

## chart-insights-split (data + narrative composite)

The `chart-insights-split` pattern is the canonical "chart on the left, takeaways on the right" consulting layout. The pattern emits a 65/35 column split: the left panel is a `Diagram` cell rendered by svggen; the right panel is a Shape cell with the title (defaults to `Key Insights`) and 1–6 bullet takeaways, or a lone `so_what` callout when there are no bullets. At least one bullet or a nonempty `so_what` is required. **A lone callout beside a chart is set as the slide's one statement** (go-slide-creator-e0xvy): the split is 75/25, and the takeaway band is written at the 18pt lead step (14pt when it would run past five lines) and centred on the chart's height (`vertical_align: "center"` on the band grid). At the 13pt label size, top-anchored in a 35% column, it was one small line over an empty column. `chart_width_pct` still pins the ratio; with a `headline` or bullets the stacked column below applies. **The default widens to 75/25 when the insights column is sparse** — at most two bullets, ≤140 characters in total, and no headline or so-what (or the so-what alone) — because a 35% column holding one short bullet leaves a large empty block while the chart is squeezed into 55% of the slide (go-slide-creator-pyxn). A thin vertical accent divider can be toggled via `overrides.show_divider`, and `overrides.chart_width_pct` (clamped 40–80) pins the ratio, overriding both defaults.

**The stacked headline / so-what column is template-aware** (go-slide-creator-bzh34). With a `headline` or `so_what`, the column is a nested grid whose headline and so-what rows are pinned at the height the shape writer needs (`writtenFitHeightPt`, at the column's real inner width after the 8pt column gap and 4pt sub-grid inset), and the insights panel takes the rest. When the insights would not fit the slide's content area, the headline steps to 26pt and the so-what to 12pt, then (unless `chart_width_pct` pins it) the chart narrows in 5-point steps to 55%. If nothing fits, the closest layout is used and `PostExpandWarnings` emits `BODY_TOO_LONG` naming what to drop. Sizing those rows as percentages of an estimated area wrote the insights at 88% autofit (10.6pt) on midnight-blue while p-style fit. `table-highlight` follows the same rule: a table taller than the content area (a short template or a `takeaway` bar) re-measures its rows with the writer and compacts the legend reserve before the grid can over-fill, and reports `BODY_TOO_LONG` when it still does not fit. `image-text-split` sizes its text column with the writer at the nested width and narrows the image (to 30%, unless `image_width_pct` pins it) when the text would not fit.

For readability the right panel applies vertical rhythm via per-paragraph `space_after` (points): the title carries extra separation below it so it reads as a header, and non-final bullets carry inter-bullet breathing room so the column does not render as a dense block. `space_after` is a general field on the shape-grid `paragraphs[]` cell-text form (points, converted to hundredths of a point), available to any pattern that emits paragraph arrays.

**Bulleted paragraphs.** The `paragraphs[]` form also takes `bullet`: `true` for the default `•`, or a marker string of at most two characters (e.g. `"–"`). It emits a real `<a:buChar>` with a hanging indent (`marL` = −`indent` = 0.65em of the paragraph size, at most 14pt), so a wrapped line aligns with the text, not under the marker. Patterns never prepend a typed `"• "` to paragraph content; `before-after`, `bmc-canvas`, `chart-insights-split`, `image-text-split`, `next-steps`, `scqa-summary` (multi-item), `strategy-house`, `stylish-panels` and `text-sidebar` emit `bullet: true` (go-slide-creator-zieyk). Pattern sizing (`sizedPara.bullet`) measures a bulleted paragraph with the hang's width.

`values.chart` accepts any svggen `DiagramSpec` payload or the flat `{label: value}` shorthand that `chart_value` accepts (e.g. `{"type": "bar", "data": {"Q1": 12, "Q2": 14}}`); the shorthand is normalized to `categories`/`series` at decode time, preserving key order. Validation expands the pattern and dry-renders the chart, so `validate_input` rejects exactly the charts `generate_presentation` would.

`values.chart` is **optional**. When omitted, the pattern collapses to a single-column insights cell at 100% width and emits the structured warning `CHART_PLACEHOLDER_EMPTY: chart-insights-split rendered insights-only; provide a chart spec to fill the left panel` via the `PostExpandWarner` interface. Every surface converts that warning into a `FitFinding` with `code = "CHART_PLACEHOLDER_EMPTY"` and `action = "review"`: `validate_input`, `generate_presentation(fit_report=true)`, `score_deck`, `preview_presentation_plan` and the `validate --fit-report` / `generate --json-output-report` CLI, plus `expand_pattern`'s own `warnings[]`. Until go-slide-creator-wn4v only preview did, so the documented validate → generate loop called a 75%-empty slide clean. Agents should either supply a chart spec or switch to an insights-only pattern (e.g. `card-grid`, `pull-quote`).

`values.chart` is a regular `types.DiagramSpec` — pass the same shape used in slide-level diagram content (`type` + `data`, optional `title` / `style`).

**So-what extensions (go-slide-creator-pzrs).** Consulting chart slides state the figure and the implication, not just the bullets:

- `headline` `{value ≤12, label ≤60}` — a big accent number (32pt, 26pt with ≥5 insights; `overrides.headline_size`) at the top of the insights column.
- `so_what` (≤160) — the takeaway band (see [The takeaway component](#the-takeaway-component)) at the bottom of the column; when it is the only insight it is centred beside the chart at 18pt (14pt past five lines), or top-anchored at 14pt without a chart. Under insights or a headline it is 13pt beside a chart (12pt at 5+ insights), 14pt without one, and no longer carries a "So what:" label — the accent bar marks it.
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

Do not let cards / steps stretch to the full content height just because the grid is full-area. Every expanded pattern grid gets `vertical_align: "auto"` (`patterns.ApplyGridDefaults`, applied by `expandPattern` and every other `Expand` caller; no pattern sets another value for a slide-level block; a slide `pattern.vertical_align` — `auto` / `top` / `center` / `bottom` / `stretch` — overrides it), and the shapegrid resolver keeps a content-sized block when **at least one row sets `max_height`** (points):

- **One placement policy for every content-sized block (go-slide-creator-yhzxt).** A pattern never places itself: it caps its rows and the resolver composes the slide (`internal/shapegrid/compose.go`, applied to a slide's own grid — `Grid.Compose` — whose `vertical_align` is `auto`). A block that needs **75% of the content area or more** is dense: it keeps its sizes and hangs from the body line (next bullet). A block that needs less is sparse and is composed in two steps. (1) **One type step up**: every word level moves to the next step of the type scale (12→14pt, 14→18pt; text at 18pt or more stays), display figures grow ×1.2 up to the 48pt KPI step, and every capped row, capped cell and the row gap grow by the first of 1.15× / 1.25× / 1.35× at which the stepped text fits its cells; rules and connector lines under 4pt keep their thickness. The step is all-or-nothing and is skipped when a level would close on the level above it (a 14pt body under an 18pt header keeps both), when a word or a KPI value that fit its line would no longer fit within 80% of it, when the text needs more than 1.35× the row, when the block would pass 85% of the area, or for text pinned to `type_scale: "compact"` (a pattern's peer labels, the strategy-house roof, or a deck / pattern set to `compact`). (2) **Optical centre**: 45% of the spare height above the block and 55% below, never above the body line. The takeaway / source chrome band is reserved before either step, so it keeps its place under the block; a pattern's own takeaway row is part of the block and moves with it. Nothing here is per pattern: do not add a pattern-side centring or size bump, and do not set `vertical_align` in a pattern to work around it. Author `bounds` / `max_height_pct` (which resolve to `stretch`) and an explicit `pattern.vertical_align` win — `"top"` restores the body-line hang at the pattern's own sizes. Nested cell patterns and compose segments are not composed (their area is the cell). `ResolveResult.Composed` tells preflight the block's top is deliberate, so `grid_violation` does not report its `content_top`. **A short label stays on one line:** a paragraph of at most three words that fit one line before the step must still fit one line after it (`ComposeLabelMaxWords`), or the block keeps its sizes — a KPI caption broken as "Logo churn (SMB-" / "weighted)" at 18pt read worse than the caption whole at 14pt; sentences may take another line. **Capacity advice resolves the same block:** `expand_pattern` `cell_budgets`, the `internal/textcapacity` budget guide and the cells handed to a nested cell pattern of a slide's own grid (`nestedPatternCellBounds`) resolve with `Grid.Compose` too, so budgets for a sparse slide-level pattern are computed at the stepped sizes and grown rows generation renders (`TestCellBudgetsResolveTheComposedBlock`, `TestNestedPatternCellsFollowTheComposedParent`).
- **strategy-house spends spare height on its roof first.** After the text, the breathing room, the designed 1:12 pitch and the 1.2× pillar growth, height still spare steepens the gable up to a 1:8 pitch (at most 30% of the height available) before the block is left to the placement policy.

- Cap rows in points, not percentages, derived from `contentAreaPt(ctx)` (e.g. process-flow `0.45 ×` content height), or from a text estimate (`textBlockHeightPt`) for text rows. Point caps keep nested / composed use sane: inside a small compose cell the cap exceeds the cell and the row simply fills it.
- **No stretch-to-fill; cap boxes at 1.6× their content (go-slide-creator-wntyw).** A filled card / tile / pillar / panel is at most `contentStretchMax` (1.6) × its measured content height — or its content plus the minimum padding and insets when that is larger — and the block takes the `auto` placement: it hangs from the template's body placeholder top (`ContentZone.BodyTop`, the line native bullets start on; for a title-only layout, the reference one-content layout's when both draw the same title box) as far as its slack allows, whatever its fill; only on a template without a body line does `auto` centre a block filling over 60% (go-slide-creator-e17xy). Default grid bounds (`ContentZone.ContentTop`) start on the body line as well, capped at 9pt under the title box: a measured short title never pulls a full-height pattern up above the line native bullets start on, and a template whose body placeholder sits lower than that (p-style, modern-yellow, business-template) keeps the title-box start so the zone every schema-maxima pin is sized for (`TestSchemaMaximaStayReadable`) loses no height. Native SWOT / PESTEL / KPI-dashboard / house diagrams hang from their body placeholder top the same way. `kpi-Nup` (`kpiRowMaxHeightPt`: content × 1.6, floor content + 12pt + insets, ceiling the content area), `card-grid` (`contentSizedRow`), `before-after` / `before-after-compact` (header band + bullet panels that hug their lists) and `strategy-house` (bands pinned to their text + 10pt, pillars capped at their tallest title + bullets + card padding) follow it. `kpiBaseCardHeightFrac` (0.70) is only the card-height *estimate* that picks the icon position and default icon footprint; it is neither a floor nor a cap on the rendered card. Never pad a box to satisfy `SLIDE_UNDERUSED`: the content-sized box patterns are judged against a 20% ink threshold instead of 29%.
- **Never emit a row below its own written fit (go-slide-creator-k3eb3).** A pattern that sizes a row (a `max_height` pin, a `min_height` floor, or flex weights) measures the text it will write — the same cell JSON Expand emits, via `writtenFitHeightPt` at the cell's real column width, including baseline nudges and paragraph spacing — and never hands the grid a row shorter than that. A one-line probe or the theme-font model alone is not enough: with the uniform 0.5 cm margin (28pt of every row) the writer stored autofit shrinks that took 12–14pt runs to 9–11.8pt on the short content areas (abstract 687×294pt, modern 851×311pt) and generation refused them. When the rows do not fit, give way in this order: air (row gaps, the space above a band), geometry (state-shift-hub narrows its hub, flex rows are floored at their needs with `floorFlexRowsAtNeeds` so crowded rows take height from sparse ones), then type steps down to the 12pt floor (`next-steps` and `metric-list` take their floor step only when it makes the list fit, keeping the schema-maxima pins where an overflowing list is shrunk either way). What still does not fit is reported by `PostExpandWarnings` as `BODY_TOO_LONG`, measured against the template's content area when `LayoutBounds` is known (character budgets remain the contract without it). Exemplars must fit the shortest shipped content area: `TestShortContentAreaExemplarsStayAboveFloor` and `TestTemplatePatternMatrix` hold that. The same rule covers (go-slide-creator-n1muf): `icon-row` (the card is its caption's written fit plus the top-icon zone; the 45% strip cap gives way up to the content area), `labeled-rows` (row pins are the larger of the theme-font model and the written fit of the label block and body), `matrix-2x2` (quadrant rows floored at their written fit plus any top-icon zone with `floorFlexRowsAtNeeds`; the default 16pt header steps through 14 to 12pt), `process-grid-2row` (header / outcome rows never below their written fit, tracks floored at theirs; the default 14pt row label steps to 12pt) and `process-flow` / `process-flow-compact` (the step row or compact band grows to the tallest label's written fit — the writer measures the full shape bounds whatever the preset's notch — up to the content area). `TestExtremePayloadsWriteAboveFloorOrWarn` holds extreme legal payloads of these patterns on the short areas to "written at ≥12pt or `BODY_TOO_LONG`". `numbered-step-strip` stacked-box / toc rows follow it too (go-slide-creator-ni71s): they were `auto_height` rows estimated from newline counts, so a takeaway or takeaway + source band (which takes 70–100pt off the zone) scaled every row down and the writer stored 12–14pt labels at 6–11.5pt, a refused deck. Each row is now pinned (`min_height` = `max_height`) at the written fit of its tallest cell at the real column widths, measured against `LayoutBounds` — the zone after title, footer and bands — and gives way in order: row gap 6 → 2pt, single-line rows to the writer's clamped one-line margin, then the label (stacked-box 13pt) / title (toc 14pt) to 12pt, taken only when it makes the strip fit. Past that `PostExpandWarnings` reports a measured `BODY_TOO_LONG` naming the need and the area. The character budgets in the schema assume no band: under a takeaway band, five stacked-box rows with bodies (any length) or five toc rows with bodies do not fit the bundled templates, and six or seven rows still hold labels only. `TestNumberedStepRowsReadableOrReportedUnderChromeBands` and `TestPatternBlockStaysAboveChrome` hold it. The n1muf start set follows the same rule (go-slide-creator-n1muf): `stylish-panels` sizes its ribbon and body rows to their written fit and steps bullets 14 → 12pt, then ribbons 16 → 14pt; `team-bios` floors text rows at their written fit and shrinks the headshot row (never below a square that holds its initials) before the name steps 14 → 12pt; `framework-grid` also measures its label band and tightens card padding and row gaps (10 → 4pt, 8 → 4pt) before titles step to 12pt; `contact-directory` floors person rows at their written fit and tightens row and group gaps before reporting; `card-grid` already holds (its budgets are measured against the area). Each of these reports `BODY_TOO_LONG` against `LayoutBounds` when it still does not fit. Size with `writtenNeedOrOverflowPt` rather than `writtenFitHeightPt(…, 0)` when overflowing text must fail a fit check: the latter returns its `minPt` for text still shrunk past its 400pt search window. `TestPanelPatternsReadableOrReportedOnShortAreas` holds the contract. The same rule now holds for `timeline-horizontal` (dots stop rows grow past their 40% cap and the default 14pt label steps to 12pt, then the date and stop cells drop the top / bottom text inset away from the axis and then the rest — the 47pt date row hugs its 12pt line — and a timeline that still does not fit is scaled whole, row gaps included, so its text is never laid out in zero height and more height never turns a fit into a shrink (go-slide-creator-wj8uz); chevrons grow past 25%; gantt rows whose label wraps hold their fit and the label column widens 30 → 45%, labels measured at the atomic-token width when the face is substituted) and `pull-quote` (the attribution row holds its fit; the default quote steps 36 → 28 → 18pt before a long quote is left to autofit); `text-sidebar` already sized both columns at or above their written fit (go-slide-creator-n1muf). Single-word labels are atomic tokens too: `scqa-summary` widens its label column to 1.3 : 4, then steps the label down, until every label fits `textfit.AtomicTokenWidthPt`, so "Complication" never breaks mid-word in a substituted face.
- **The written-fit rule for agenda, before-after[-compact], comparison-2col and hero-detail (go-slide-creator-n1muf).** These pinned rows from the theme-font model alone, so legal payloads inside their character budgets were written 6.5–11.8pt on the short areas. Their rows are now floored at `writtenFitHeightPt` (via `rowTextNeedPt`, which reports text taller than the helper's probe as beyond any slide instead of 0), and each gives way in the rule's order, taking a step only when it makes the block fit: `agenda` adds an 18/12pt floor scale after its 28/14 → 18/14 steps; `before-after` tightens the header/body gap 8 → 4pt, then steps the header 16 → 14pt; `before-after-compact` tightens the gap 6 → 4pt, lets its 60% height cap grow to the whole area, then steps the header 14 → 12pt; `comparison-2col` tightens the row gap 8 → 4pt, then steps to a 14pt header / 12pt body, and past the fit keeps its header band while body rows share the rest in proportion to their needs, at the first step the writer stores whole (see "An overflowing comparison" below); `hero-detail` sets the hero figure at the larger of 80 / 48pt that keeps the hero within 45% of the area (stepping further when the cards need it), then tightens the gap 10 → 4pt, then steps the label to 14pt and card titles to 12pt, and sizes icon cards to the writer's top-icon zone. Each reports `BODY_TOO_LONG` measured against `LayoutBounds` only when the writer would actually store a shrink (a one-line cell in a short row can still be written whole). `TestAgendaWrittenFitOnShortAreas` and its siblings (abstract, modern, midnight-blue, warm-coral and p-style areas with their theme fonts) hold it.
- Row-list patterns of **unfilled** rows (`exec-summary`, `metric-list`, `labeled-rows`) may open up their spacing with `fillCappedRows` towards a minimum share of the zone, but every grown row stays within 1.6× its natural height; dividers and callouts keep their heights. Sparse agendas and flows promote their default type size; explicit size overrides still win. The same holds for two patterns whose columns are fitted to their labels, where the policy's step is refused because a stepped word no longer fits its column (go-slide-creator-yhzxt): a dots `timeline-horizontal` whose block needs no more than 60% of the area with date and body at the label's 14pt (and whose dates, label words and three-word bodies stay whole at 18pt) starts from 14 / 14 / 14pt, which the policy then steps to 18pt together; a `driver-tree` whose rows need no more than 85% of the area at 18 / 14 / 14pt (root / branch / leaf) is laid out at those sizes, with nodes 1.2× their written fit when that fits too. A timeline or tree with real copy, or in a short cell, keeps the sizes its budgets are measured at. Use `*-compact` variants when the pattern is supporting context rather than the slide's main content. Pointed flows remain aspect-capped to keep chevron/arrow labels readable.
- **Thin connective geometry (go-slide-creator-7z5we).** Axes, spines and transition markers are rules, not blocks: matrix-2x2's tiles-style axes are 1.5pt arrows with an 8pt head (`matrix2x2AxisArrowPt`, held by a cell `max_height` / a narrow column), the timeline-horizontal dots axis is a `timelineRulePt` (3pt) rule centred in its row (a cell `max_height`; phase-roadmap no longer draws a spine — its interlocking band carries the order, go-slide-creator-dlfm6), and the before-after transition chevron is a 24pt marker (`beforeAfterChevronPt`, `max_height` + `fit: "contain"`) centred in the gutter. A top `accent_bar` is the card's top rule: flush on the cell's top edge, inside the cell — it never floats above the card or reaches into the row gap above it.
- The body zone starts 18pt (`bodyZoneTitleGapPt`) below a measured top-anchored title's text; the takeaway band is reserved before the block is centred (`reserveTakeawayBand`), so the group centres in the remaining zone.
- Fixed `height` percentages in a grid that has a capped row are kept as absolute shares (no re-normalisation), so the slack is real and the block is centred.
- Grids without any capped row keep the legacy proportional stretch (agenda lists, stacked steps, team-bios rely on it).
- **Points follow the canvas (go-slide-creator-ttpae).** A pattern's row heights, gaps and type sizes are points designed on the standard 13.33 x 7.5in slide. On a larger slide the resolver scales them by the slide's size relative to that one (`shapegrid.CanvasScaleFor`: the smaller of the width and height ratios, 1.10 on business-template's 14.7 x 8.3in, exactly 1 on a standard or smaller slide, so standard templates render as before). `cmd/json2pptx` sets `Grid.CanvasScale` on every grid a pattern expanded and on the sub-grids it nested (they carry the pattern's `source` stamp), and on the grids the semantic compiler builds by hand (a `regions` slide, stamped `source: "compiler:regions"`; go-slide-creator-o9n8u); an authored `shape_grid` keeps its points. The resolver (`internal/shapegrid/canvas.go`) grows point-valued `min_height` / `max_height`, cell caps and gaps by the scale — percentage heights already follow the area and hairlines keep their thickness — resolves the grid as designed (placement policy, type step, snap onto the type scale) and then writes each paragraph at its size times the scale, rounded to a whole point (12 → 13, 14 → 15, 18 → 20). It scales a size level (every paragraph the design writes at one size) only when that holds: a level with a word, short label or KPI value that would lose its line keeps its design size (a token may take 98% of its line measured in the template's own face, 80% in the Liberation Sans stand-in, or the share of its line it took as designed, since text and line grow together; go-slide-creator-5x4w4), as does the largest level of a cell that would need more of its height than before once taller rows (x1.12, x1.25) have been tried; a block that hangs from the body line grows no further than the area under the line. A grid none of whose text can move resolves exactly as designed. Do not compensate in a pattern: size it in points for the standard slide and let the resolver scale it. Generation, preflight and the budget estimators all resolve through `gridCanvasScale`, so findings measure the scaled sizes.
- **Step and scale decisions measure the template face (go-slide-creator-5x4w4).** The type step of a sparse block (`stepFit`), the canvas scale (`canvasCheck`) and `type_scale` growth measure each paragraph in the face it renders in when that face measures the same on every host (`pptx.ParagraphFitFace`: Calibri as Carlito, Arial as Liberation Sans, Lora, Poppins Light), and in the Liberation Sans stand-in with its 80% token margin otherwise (Aptos, Segoe UI, Tenorite, Gill Sans, Georgia, and every authored grid). The stand-in is wider than Calibri and narrower than Poppins Light, so it refused steps Calibri holds (a five-KPI row, a seven-stop chevron timeline) and allowed steps Poppins Light does not (quote-cluster and timeline-horizontal on modern-template). A pattern does not rely on a stored autofit `fontScale` to make siblings match: a shrink a row's same-size cells share is written into their sizes (`writeSharedShrink`), and a label column measured for a stand-in face keeps the 80% margin (`labelFitWidthPt`).
- **A one-line label gets room for a renderer's wrap (go-slide-creator-bhoo3).** A label written on one line of a box one line tall, within 12% of the box's width (`shapegrid.RenderFaceSlack`), stays on its line in the engine's face and wraps in a renderer whose face runs wider; the box has no second line, the renderer's autofit shrinks that cell alone and the row reads at two sizes — what `SIBLING_SIZE_MISMATCH` reports. What sizes a row for such a label gives it the second line instead: the type step of a sparse block counts a stepped one-paragraph label that close to its box as the two lines it wraps to, so it takes the row growth that holds them or the block keeps its type (`stepFit`, `slackLabelHeightPt`; `examples/comparison-2col-connectors.json` slide 2 on midnight-blue is three 78pt rows at 18pt, was 67pt rows with "Always-on client portal with live exposures" across 95% of its box); `comparison-2col` sizes a row for the wrap while the rows still fit (`labelRowNeedPt`); `next-steps` widens its owner column (22% up to 30%) and date column (15% up to 20%) until the longest label has the slack to spare, and sizes the row for the wrap when a label is too long for that; `table-highlight` measures its header row at the narrowed width (go-slide-creator-4fz04). A pattern that pins a row of sibling labels to one line measures with `labelRowNeedPt`. `TestComposeStepLeavesRoomForARendererWrap`, `TestComparisonRowsHoldARendererWrap`, `TestNextStepsOwnerColumnHoldsItsLongestLabel` and `TestExamplesRowsReadAtOneSizeAcrossTemplates` (every deck under `examples/`, compiled semantic specs included, on midnight-blue, modern-template and p-style) hold it.
- **An overflowing comparison takes the first step it is written whole at (go-slide-creator-bhoo3).** When no step holds every `comparison-2col` row at its full margins, the default sizes were kept whatever the writer then did, and the writer shrank the crowded rows alone (six rows on modern-template: five at 14pt, one at 13.4pt, with `BODY_TOO_LONG`). The comparison now takes the first of its steps (default, 4pt row gap, 14pt header / 12pt body) at which the writer stores every cell without a shrink — one size for the whole comparison, and no `BODY_TOO_LONG`, because nothing is shrunk — and keeps the default sizes only when every step is shrunk. Without a known content area (`LayoutBounds`) the default sizes are kept as before. `TestOverflowingComparisonIsWrittenAtOneSize` reads the sizes back from the slide XML.
- Height-capped, top-anchored pattern `bounds` (`y: 0`, `height < 100`) follow the same `auto` rule, or `center` / `bottom` when set; the content area already excludes the takeaway/source chrome band. A slide-level box under 75% of the area whose rows fill it (numbered-step-strip's chevron style) sits at the optical centre like a sparse row block, without the type step. **A pattern whose rows are content-sized does not set a bounds box** (go-slide-creator-yhzxt): `kpi-inline`, `before-after-compact` (header and body rows pinned at their fit, laid out for 60% of the area) and `process-flow-compact` (one row capped at the band height, 22% of the area or the tallest label's fit) pin their rows and are composed like any other block — stepped type and the optical centre on a slide of their own, filling their cell as a compose segment. Their `expand_pattern` output carries no `bounds`; `pattern.vertical_align: "top"` restores the band under the title. `process-flow-compact` alone on a slide still reports `SLIDE_UNDERUSED` (a 4-step band covers ~15% of the area): it is supporting context, to be paired with a second zone.
- Big single-token values (KPI numbers) must shrink to fit one line (`fitSingleLineSize`) rather than wrap.
- Header bands: fix the row with `min_height = max_height = headerRowPt(...)` (~1.2× the header line height + padding), never a percentage of the grid.
- Cards: size with `contentCardHeightPt(shapeTextHeightPt(...), cardW, hasTopIcon)` and pass sparse card text through `anchorSparseText` so a short body is centred instead of hanging top-left (target: < 30% unused area per card). Skip content-sizing for rows that host a secondary chart.
- **A row's height is the row's, not the cell's.** Every card in a grid row is as tall as the tallest one, and without a `max_height` the row also stretches to fill the content area. Both together put a one-line quote in the top fifth of a tall tinted box (go-slide-creator-pr3g). `contentSizedRow(ctx, cells, cols)` is the shared answer — it sizes the row to the tallest card's own text and runs every sparse card through `anchorSparseText` — and `card-grid`, `quote-cluster` and `icon-row` all go through it. A text-only row (value-chain's descriptions) needs the same cap without the card padding: measure with `shapeTextHeightPt` and set `MaxHeight` directly.
- **A fill of `lt1` is invisible.** `strategy-house` filled its pillars white on a white slide, so a pillar column vanished below its last bullet and the gap to the foundation read as empty space rather than as the pillars holding the house up. Use `surfaceFillJSON(ctx, "subtle", NeutralTint4)` for a panel that must read as a surface (the template's declared surface, else the neutral 4% step; never `lt1`), and `surfacePairJSON(ctx)` for alternating rows.
- **Gaps scale with the template gutter (go-slide-creator-5ms8c).** Author gaps against the engine's 8pt gutter and pass every gap you emit (`gap` / `col_gap` / `row_gap`) or subtract while sizing through `ctx.Gap(x)`; a template whose metadata declares `grid.gutter_pt` scales them by `gutter_pt / 8`, and with no declared gutter `ctx.Gap` returns `x` unchanged. Gaps of 1pt or less and gaps that are drawing geometry (connector channels, axis arrows) stay fixed. `TestPatternGapsFollowTemplateGutter` expands every registered pattern under a 16pt gutter and fails when its largest gap does not double. See [TEMPLATE_SPEC.md](TEMPLATE_SPEC.md#grid).
- **No outline on a filled shape (go-slide-creator-pgdkp).** Every filled shape a pattern emits has `line: "none"` (`noLine`). Separate neighbours with the grid gap (4–6pt white gutters) over a neutral field, or with two neutral steps: `neutralFillJSON(NeutralTint4)` / `NeutralTint8` (dk1 at 4% / 8% via `lumMod`/`lumOff`, so the grey is the template's own ink, never a hard-coded warm or cool grey). matrix-2x2 and bmc-canvas are built from gutters, not borders. Strokes survive only as rules: a 0.5pt row divider, a 1pt header underline, a 2–3pt `accent_bar`, or an unfilled ring. `TestNoPatternOutlinesAFilledShape` expands every registered pattern and fails on an outlined fill; an authored outline on a raw grid (or a card-grid `border` override) reports `FILLED_SHAPE_OUTLINED`.
- **Native diagrams take the same surface (go-slide-creator-amtkg).** The native framework builders in `internal/generator` (`swot`, `business_model_canvas`, `pestel`, `nine_box_talent`, `porters_five_forces`, `value_chain`, `kpi_dashboard`, the panel family) do not go through a shape grid, so they read the style from the exports in `neutral_surface.go` and `accent_styles.go` rather than restating it: `SurfaceGeometry` (square corners), `NeutralSurfaceColor` + `NeutralSurfaceMods(pct)` (the same `dk1` `lumMod`/`lumOff` pair `neutralFillJSON` writes), `AccentInkOnNeutral` (`accentInkOnTone` for a title on a neutral step) and `PrimaryFill` (the one accent). `internal/generator/native_surface_style.go` is the only place that turns them into OOXML fills; a native builder that picks its own corner radius, tint or header colour is a regression. Change the ladder or the header rule here and both routes move together — `bmc-canvas` and native `business_model_canvas` are the same cell. Semantic colour stays where it is data (nine-box score bands, Porter intensity steps, heatmap scales, KPI trend ink); `style.colors` on the diagram is the opt-in back to accent-tinted cards.
- **Neutral surfaces, one accent (go-slide-creator-8xsj3).** Pattern surfaces default to the tonal system's roles (a panel the neutral 4% step, a content shape the accent's Lighter 80% swatch — see "The tonal system") with measured ink; `accent1` at 100% belongs on the slide's one emphasised element (the highlighted value-chain step, the current maturity stage, the active roadmap phase, the strategy-house objective, a recommended card). Do not paint 40–70% accent mid-tints or alpha ramps as decoration — tint ramps are for ordinal encodings only (capability-heatmap tiers). Chains and stacks (value-chain, arch-stack) are the accent's Lighter 80% swatch with the highlight solid or a 3pt bar in the accent. Where a saturated fill remains, choose its ink by measurement at the text's own size (`readableInkOn(ctx, tone, "lt1", TextContrastThreshold(size, bold))`). Never use `dk2` as a fill when it is black: `structuralDarkTone(ctx)` gives `dk2` when it carries a brand hue and `dk1` at 60% otherwise.
- **Keep the template's accents; change the ink, not the fill.** Patterns write `lt1` text on accent fills. On light-accent templates (p-style's oranges) that text is unreadable, and rotation used to replace the fill with `dk2` / black, painting every card black. Now a light accent stays when a theme dark ink reads on it (and the fill still stands out from `lt1` at >= 2:1), and `ApplyReadableInk` swaps the pattern's light text for the first readable ink (`lt1`, `dk2`, `dk1`) after expansion. Author-chosen colours and dark text are never rewritten (the one exception is a fill no theme ink reads on, go-slide-creator-pr5bx above); translucent fills are skipped.
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

## Previews are the generated slide

A pattern has one picture: the slide generation produces for its
`ExemplarValues()` under a title. Nothing draws a pattern any other way
(go-slide-creator-r1uy7, -ueopl).

- `renderSpecPreview` (`cmd/json2pptx/mcp_preview_recipe.go`) compiles a
  one-slide DeckSpec with `render_deck_spec`'s compiler and runner, then
  renders slide 1 through the render cache, keyed by the slide's visible
  identity (`render.VisibleSlideKeys`). It backs `list_slide_kinds(preview:
  true)`, `recommend_visual(preview: true)` and `json2pptx preview-patterns`.
- A pattern must therefore implement `Exemplar`, and its exemplar values must
  render under strict output validation on every template.
  `TestPatternPreviewIsTheGeneratedSlide` fails for a registered pattern
  without a preview recipe, and for a preview deck whose slide differs from
  `render_deck_spec`'s. `TestPatternPreviewPixelsMatchGeneratedSlide` compares
  pixels against an independent conversion (needs LibreOffice; a sample by
  default, every pattern with `PREVIEW_PIXEL_FULL=1`).
- No gallery is committed. `make preview-patterns` (or `json2pptx
  preview-patterns -template <name> -pattern <name>`) writes one locally to
  the gitignored `assets/pattern-previews/`. Look at it after changing a
  pattern's layout — and at `generate` output for real content, since the
  exemplar is one content shape.

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
