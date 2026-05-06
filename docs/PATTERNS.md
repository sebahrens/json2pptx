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
| Single dominant metric | `stat-hero` | One hero number with context |
| Feature/capability cards | `card-grid` | Multi-line body text per card |
| Sequential process | `process-flow` | Ordered steps with arrows |
| Temporal sequence | `timeline-horizontal` | Date-labeled stops |
| Layer/stack diagram | `arch-stack` | Vertical tier ordering |
| Narrowing hierarchy | `pyramid` | Visual narrowing (top < bottom) |
| Before/after comparison | `before-after` | Temporal transformation |
| Option/pros-cons comparison | `comparison-2col` | Non-temporal side-by-side |
| 4-quadrant positioning | `matrix-2x2` | Axis-labeled quadrants |
| Phased plan | `roadmap-phased` | Named phases with items |
| Cross-functional swimlanes | `swimlane` | Multiple parallel tracks |
| Deck section list | `agenda` | Numbered section outline |
| Icon + caption row | `icon-row` | Visual categories, 3–5 items |
| Callout / testimonial | `pull-quote` | Attributed quotation |

## Taxonomy fields

Every pattern must implement `Taxonomy() PatternTaxonomy` returning classification metadata used by `recommend_pattern` and `analyze_deck_rhythm`:

```go
type PatternTaxonomy struct {
    Category      string   // "data-display", "narrative", "structural", "hero"
    NarrativeRole []string // "open", "frame", "evidence", "compare", "conclude"
    PairsWith     []string // sibling pattern names that flow well as the next slide
    DensityClass  string   // "low", "medium", "high"
    AccentWeight  string   // "subtle", "normal", "strong"
}
```

Guidelines:

- **Category**: group by primary function — data patterns show metrics, narrative patterns tell stories, structural patterns frame methodology, hero patterns emphasize one thing
- **NarrativeRole**: where in a deck arc this pattern fits. A pattern can serve multiple roles (e.g. `kpi-3up` is "evidence", `agenda` is "open" + "frame")
- **PairsWith**: 2–4 sibling patterns that create good rhythm when sequenced after this one. Used by `recommend_pattern` diversity scoring and `analyze_deck_rhythm` run detection
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
- **Content-structured layouts** (bmc-canvas, agenda, roadmap-phased, swimlane, timeline-horizontal): cell fills are determined by content structure (lanes, phases, sections) rather than peer ordering.

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

## Composition (planned)

Pattern composition — placing multiple patterns on a single slide via a `compose` envelope — is a planned feature (see bead `go-slide-creator-pbyh`). When it lands, this section will document:

- The `compose` envelope grammar (how to nest patterns)
- Layout splitting rules (how slide area is divided between composed patterns)
- Interaction with `cell_overrides` (indices are per-pattern, not global)

Until then, each slide accepts exactly one `pattern` (XOR with `shape_grid`).

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
