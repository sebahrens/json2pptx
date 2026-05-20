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
  - `Validate()` using `errors.Join` aggregation
  - `Expand()` returning `*jsonschema.ShapeGridInput`
  - `CellsHint()` (part of the core `Pattern` interface)
  - `Taxonomy()` returning `PatternTaxonomy` (see below)
- [ ] **Tests** (`internal/patterns/<name>_test.go`)
  - Metadata: `Name()`, `UseWhen()`, `NotWhen()` non-empty (D6), `Version()`
  - Taxonomy: all fields populated with valid values
  - Schema validity: marshals to valid JSON Schema draft 2020-12
  - Validate: happy path, wrong count with sibling hint (D4), missing required fields, max length exceeded, invalid cell override keys
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
| `accent_bar` | bool | Show accent bar decoration |
| `emphasis` | `"bold"` / `"italic"` / `"bold-italic"` | Text emphasis |
| `align` | `"l"` / `"ctr"` / `"r"` | Horizontal alignment |
| `vertical_align` | `"t"` / `"ctr"` / `"b"` | Vertical alignment |
| `font_size` | number (6-120) | Font size in points |
| `color` | string | Text color (scheme ref, e.g. `"dk1"`) |

**MUST NOT** accept arbitrary nested `shape_grid` fragments or geometry changes. Cells are addressed by zero-based index as string keys (`"0"`, `"1"`, ...). The pattern's `Validate` must reject unknown override keys with an error citing the D15 whitelist.

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
| Big-number KPIs (2–6 items) | `kpi-Nup` | Fixed count, ≤8-char metrics |
| Ranked horizontal bars (3–8) with per-bar insight | `horizontal-bar-with-callouts` | One callout per bar, accent-bar bound to the row |
| Single dominant metric | `stat-hero` | One hero number with context |
| Feature/capability cards | `card-grid` | Multi-line body text per card |
| Sequential process | `process-flow` | Ordered steps with arrows |
| Porter / supply value chain | `value-chain` | 4–10 step columns with bold label + 1–3 line description |
| Maturity ladder / current-state journey | `journey-maturity-model` | 3–6 stage columns with numbered headers, description, and optional 'where we are' marker |
| P&L walk / cost-driver bridge | `waterfall-bridge` | 3–10 columns of total + delta + subtotal bars; floating deltas with auto-computed subtotals |
| Value / cost driver tree | `driver-tree` | Root metric → 2–4 branches → 1–4 leaves each, with optional per-branch annotations (for **people/role** hierarchies use svggen `org_chart` instead) |
| Temporal sequence | `timeline-horizontal` | Date-labeled stops |
| Layer/stack diagram | `arch-stack` | Vertical tier ordering |
| Narrowing hierarchy | `pyramid` | Visual narrowing (top < bottom) |
| Before/after comparison | `before-after` | Temporal transformation |
| Option/pros-cons comparison | `comparison-2col` | Non-temporal side-by-side |
| 4-quadrant positioning | `matrix-2x2` | Axis-labeled quadrants |
| Phased plan with workstreams | `roadmap-phased` | Named phases × workstreams grid |
| Single-track phased roadmap | `phase-roadmap` | Phases + timeline bar + dates + per-phase description (+ milestones) |
| Cross-functional swimlanes | `swimlane` | Multiple parallel tracks |
| Executive summary (problem framing) | `scqa-summary` | 4-row Situation/Complication/Questions/Answer narrative arc |
| Deck section list | `agenda` | Numbered section outline |
| Visual deck preview | `agenda-with-images` | Numbered agenda rows with image/quote placeholders alongside the title (3–6 items) |
| Team / 'Our People' page | `team-bios` | 1–8 named people with photo placeholder + role + short bio, up to 4 per row |
| Joint-venture / engagement-team paired roles | `dual-org-ladder` | Two parallel columns of 2–6 paired role cards with an org-name header above each column (optional connector line per row) |
| Icon + caption row | `icon-row` | Visual categories, 3–5 items |
| Callout / testimonial | `pull-quote` | Attributed quotation |
| Stakeholder quote cluster | `quote-cluster` | 3–8 attributed quote bubbles in a 3-column grid (voice-of-customer slides) |

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
- **PairsWith**: 2–4 sibling patterns that create good rhythm when sequenced **after** this one (next-slide adjacency). Used by `recommend_pattern` diversity scoring and `analyze_deck_rhythm` run detection
- **ComposesWith**: sibling patterns that can **share a slide** with this one through a compose envelope (D18). Distinct from `PairsWith`, which is purely about next-slide sequencing. Populate when the pattern naturally combines (e.g. `stylish-panels` + `pull-quote` for a pillars+callout layout). Leave empty for patterns that should always occupy the whole slide.
- **RoleOnSlide**: which slot(s) this pattern occupies in a compose envelope. Patterns can fill more than one role (e.g. `kpi-3up` works as either `banner` or `foundation`). Leave empty for patterns not intended for compose-envelope use.
- **DensityClass**: visual density — affects rhythm analysis and variety recommendations
- **AccentWeight**: how much accent color this pattern uses — "strong" patterns (KPIs, stat-hero) need breathing room before/after

## Cell Accent Variety (authoring contract)

Grid-shaped patterns — those that emit multiple peer cells through the shape grid engine — must support `cell_accent_mode` in their overrides. The contract:

### Grid-shaped patterns (must expose `cell_accent_mode`)

1. **Embed `TextOverrides`** (or the pattern-specific overrides struct that includes `CellAccentMode string`). The shared `TextOverrides` struct in `overrides.go` carries the `cell_accent_mode` field.
2. **Validate** by calling `ValidateCellAccentMode(patternName, ovr.CellAccentMode)` in the pattern's `Validate()` method. This rejects unknown modes with a structured `ValidationError`.
3. **Resolve per-cell accent** by calling `ResolveCellAccent(baseAccent, cellIndex, cellAccentMode)` in the cell-emission loop of `Expand()`. The function returns the accent string for each cell position given the base accent and mode.
4. **Schema** must include `cell_accent_mode` in the overrides object — use the shared helper: `EnumSchema("uniform", "alternate", "progressive").WithDescription(...)`.

### Non-grid patterns (do not expose `cell_accent_mode`)

These patterns have structurally determined accent logic and do not expose `cell_accent_mode`:

- **Single-cell patterns** (stat-hero, pull-quote): one cell, no variation needed.
- **Axis-bound matrices** (matrix-2x2): quadrant fills are semantically tied to axis positions, not peer cells.
- **Fixed-progression patterns** (pyramid): tier fills follow a structural hierarchy, not a peer-cell walk.
- **Content-structured layouts** (bmc-canvas, agenda, agenda-with-images, roadmap-phased, phase-roadmap, scqa-summary, swimlane, timeline-horizontal, team-bios, quote-cluster, dual-org-ladder): cell fills are determined by content structure (lanes, phases, sections, member cards, quote bubbles, org-paired rows) rather than peer ordering.

Each non-grid pattern should document in its `UseWhen`/`NotWhen` text or code comments why it does not expose the override.

### Test guidance

Every grid-shaped pattern must include a table-driven test exercising all three modes (`uniform`, `alternate`, `progressive`) against at least two different base accents (e.g., `accent1` and `accent3`). Verify that the emitted cells carry the expected accent strings. See `overrides_test.go::TestResolveCellAccent` for the shared function tests; pattern-level tests should exercise the full `Expand()` path.

## Cell Capacity Contract

The engine computes a deterministic text budget for every shape grid cell. Pattern authors do not implement capacity logic — the `internal/textcapacity` package derives budgets externally from the resolved grid geometry.

### Core rules

1. **`Expand()` must remain pure.** A pattern's `Expand(values, overrides, ctx)` converts structured values into a `*ShapeGridInput`. It must not call `textcapacity` or perform any capacity calculations. Capacity is computed downstream by the expand command or MCP tool after `Expand()` returns.

2. **Budget targets the body paragraph.** When a cell contains multiple paragraphs at different font sizes (e.g., a 16pt header + 11pt body), the budget is calculated against the body paragraph's font size. The header consumes vertical space but the `max_chars` value reflects body-text capacity.

3. **Font precedence.** The font size used for budget computation follows this resolution chain (first non-zero wins):
   - Paragraph-level `size` in the cell's text content
   - Shape-level `font_size` on the `ShapeSpecInput`
   - Pattern override `font_size` (from `cell_overrides` or pattern-level overrides)
   - Pattern default font size (set in `Expand()`)
   - Template theme body font size

   Pattern authors control the default by setting `FontSize` on emitted `ShapeSpecInput` structs. If a pattern does not set a font size, the template theme default applies.

4. **Determinism guarantee.** `textcapacity` uses `go-fonts/liberation` embedded metrics — no OS font dependency. Given the same grid geometry, font size, and insets, budgets are identical across macOS, Linux, and CI. This is a hard invariant; if a pattern change causes budget drift in CI, the change is wrong.

5. **Insets matter.** Cell insets (top, bottom, left, right in points) reduce the available text area. Patterns that set tight insets (< 6pt) will produce higher `max_chars` for the same cell size, but risk visual cramming. The recommended range is 6–10pt per side.

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
                if d.MaxChars < 10 {
                    t.Errorf("cell %d: max_chars=%d too small, check insets/font", i, d.MaxChars)
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
- Every text cell has `max_chars > 0`
- No cell has a budget below a plausible floor (10 chars minimum)
- Density bands shift as expected when content length varies

### Density bands reference

| Band | Density % | Status string | Agent action |
|------|-----------|---------------|--------------|
| Underfilled | < 60% | `"underfilled"` | Add content or pick a smaller grid |
| Optimal | 60–110% | `"optimal"` | No action needed |
| Overflow | > 110% | `"overflow"` | Trim content or pick a larger grid |

These thresholds are defined in `internal/textcapacity/textcapacity.go` and are stable — do not hardcode different values in patterns.

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
- `position` — `left`, `top`, or `center`. Defaults to the pattern-specific position (kpi → `left`, card-grid/iconrow/herodetail/matrix → `top`) when blank.

**Schema authoring.** New patterns that accept an icon should reuse `IconRefSchema(description)` and `validateIconRef(pattern, path, ref)` rather than duplicate the OneOf string-or-object schema. When a pattern repeats the icon slot across many siblings (matrix-2x2's four quadrants, multi-row card grids), wrap the cell schema in `$defs` and use `RefSchema(name)` to keep the per-pattern schema under the 6 KB compression budget.

**Expansion.** Pattern `Expand` code calls `cell.Icon.Resolve(defaultFill, defaultPosition)`. Resolve returns `nil` for empty refs, applies pattern defaults only when the author left a field blank, and copies the underlying `IconInput` so downstream mutation is safe. Patterns that supported a string-only icon field bumped their `Version()` to `2` when migrating.

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

The `chart-insights-split` pattern is the canonical "chart on the left, takeaways on the right" consulting layout. The pattern emits a 65/35 column split: the left panel is a `Diagram` cell rendered by svggen; the right panel is a Shape cell with the title (defaults to `Key Insights`) and 1–6 bullet takeaways. A thin vertical accent divider can be toggled via `overrides.show_divider`, and `overrides.chart_width_pct` (clamped 40–80) tunes the column ratio.

`values.chart` is **optional**. When omitted, the pattern collapses to a single-column insights cell at 100% width and emits the structured warning `CHART_PLACEHOLDER_EMPTY: chart-insights-split rendered insights-only; provide a chart spec to fill the left panel` via the `PostExpandWarner` interface. Downstream `preview_presentation_plan` and `generate_presentation` callers convert that warning into a `FitFinding` with `code = "CHART_PLACEHOLDER_EMPTY"` and `action = "review"`. Agents should either supply a chart spec or switch to an insights-only pattern (e.g. `card-grid`, `pull-quote`).

`values.chart` is a regular `types.DiagramSpec` — pass the same shape used in slide-level diagram content (`type` + `data`, optional `title` / `style`).

## Bounds Override

Patterns assume `full_content_area` by default — the grid fills the entire layout content area. For patterns with short content this produces oversized cells. Constrain the grid with:

- **`max_height_pct`** (number, 1–99): constrains grid height to this percentage of the content area.
- **`bounds`** (object: `{x, y, width, height}` as percentages of slide dimensions): explicit bounding rectangle.

These fields live on `PatternInput` (slide-level JSON) and on the `expand_pattern` MCP tool parameters. When set, the expanded grid gets a `bounds` field on the `ShapeGridInput`, which the shapegrid resolver and density math respect automatically.

`bounds` takes priority over `max_height_pct`. If neither is set, the grid uses the full content area (backward-compatible default).

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
