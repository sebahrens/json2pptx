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
| `accent_bar` | bool | Show accent bar decoration |
| `emphasis` | `"bold"` / `"italic"` / `"bold-italic"` | Text emphasis |
| `align` | `"l"` / `"ctr"` / `"r"` | Horizontal alignment |
| `vertical_align` | `"t"` / `"ctr"` / `"b"` | Vertical alignment |
| `font_size` | number (6-120) | Font size in points |
| `color` | string | Text color (scheme ref, e.g. `"dk1"`) |

**MUST NOT** accept arbitrary nested `shape_grid` fragments or geometry changes. Cells are addressed by zero-based index as string keys (`"0"`, `"1"`, ...). The pattern's `Validate` must reject unknown override keys with an error citing the D15 whitelist.

## card-grid styles + surface overrides

`card-grid` (`cardgrid.go`) exposes two complementary knobs through pattern-level `overrides` (distinct from the per-cell `cell_overrides` whitelist above):

- `style` — visual treatment enum: `filled` (default, solid accent + light text), `accent-stripe`, `numbered-badge`, `icon-card`, `tinted` (alternating lt1/lt2), `soft-card` (single pale surface, dark text, explicit no-border line).
- Generic surface overrides that apply on top of **any** style:

| Override | Type | Effect |
|---|---|---|
| `card_fill` | string (hex or scheme name) | Repaints every card's fill. A raw hex (e.g. `#FFF5ED`) is accepted only when the caller supplies it; the engine never hardcodes a brand surface. Constrained `design_mode` rejects raw hex — pass a scheme color there. |
| `line_color` | string (hex or scheme name) | Explicit card border color. Takes precedence over `border`. |
| `line_width` | number (0–12 pt) | Card border width; defaults to 1 pt when `line_color` is set without a width. |
| `border` | `none` / `subtle` / `accent` | Keyword border: `none` emits an explicit no-fill line (suppresses theme default), `subtle` is a thin dk1 hairline, `accent` is a 1 pt accent-colored border. Ignored when `line_color`/`line_width` are set. |

Validation rejects unknown colors (non-hex, non-scheme), `line_width` outside 0–12, and unknown `border`/`style` keywords. `soft-card` plus `card_fill` is the canonical recipe for a no-border pale brand surface that keeps dark, contrast-safe text.

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
| Big-number KPIs (2–6 items) | `kpi-Nup` | Fixed count, ≤8-char metrics; optional per-cell `sub` (delta/trend annotation, aliases `delta`/`trend`/`change`) |
| Ranked horizontal bars (3–8) with per-bar insight | `horizontal-bar-with-callouts` | One callout per bar, accent-bar bound to the row; omit every `callout` and the column is dropped so the bars span the full width |
| Single dominant metric | `stat-hero` | One hero number with context |
| Feature/capability cards | `card-grid` | Multi-line body text per card |
| Sequential process | `process-flow` | Ordered steps with arrows |
| Ordered steps / annotated ToC (no branching) | `numbered-step-strip` | 3–6 numbered steps with an optional per-step detail zone; `chevron` ribbon, `stacked-box` scorecard, or `toc` agenda — never emits decision diamonds (use `process-flow` for branching) |
| Two parallel process tracks | `process-grid-2row` | Two rows × 3–6 phase columns sharing the same N columns; dk2 row-label column on the left, per-row accent fill |
| Porter / supply value chain | `value-chain` | 4–10 step columns with bold label + 1–3 line description |
| Maturity ladder / current-state journey | `journey-maturity-model` | 3–6 stage columns with numbered headers, description, and optional 'where we are' marker |
| P&L walk / cost-driver bridge | `waterfall-bridge` | 3–10 columns of total + delta + subtotal bars; floating deltas with auto-computed subtotals, grey bridge lines between bar levels; `unit` currency symbols render as a prefix (`"$m"` → `$210m`), value labels move outside bars too thin to hold them and sit against the bar rather than clear of it; optional `caption` states the scale once, since a bridge draws no value axis |
| Value / cost driver tree | `driver-tree` | Root metric → 2–4 branches → 1–4 leaves each, with optional per-branch annotations (for **people/role** hierarchies use svggen `org_chart` instead) |
| Temporal sequence | `timeline-horizontal` | Date-labeled stops |
| Layer/stack diagram | `arch-stack` | Vertical tier ordering |
| Narrowing hierarchy | `pyramid` | Visual narrowing (top < bottom) |
| Before/after comparison | `before-after` | Temporal transformation |
| Option/pros-cons comparison | `comparison-2col` | Non-temporal side-by-side |
| Options × criteria evaluation | `table-highlight` | 2–6 options × 2–6 criteria rated with Harvey balls (0–4), RAG (`red`/`amber`/`green`) or ≤24-char text per column (`scale` or per-criterion `{label, scale}`); `highlight_row` tints + bars the recommended option, `highlight_col` tints the decisive criterion; one legend row per symbol scale. **`legend_labels` is `[HIGH, MID, LOW]`** — the full Harvey ball first, the empty one last — and the RAG scale takes its own `legend_labels_rag` in the same order (`[green, amber, red]`, default `["Green", "Amber", "Red"]`). A deck mixing both scales used to reuse the Harvey words for the RAG swatches, so a green dot carried the "does not meet" label (go-slide-creator-z0up). RAG uses conventional status colours (`overrides.rag_colors` swaps them) — the one non-theme palette, because status must read the same on every template |
| 4-quadrant positioning | `matrix-2x2` | Axis-labeled quadrants; each axis is an arrow pointing to its high end (right / up) flanked by low/high end labels — optional `x_low` / `x_high` / `y_low` / `y_high` (≤20 chars, default `Low` / `High`) |
| Phased plan with workstreams | `roadmap-phased` | Named phases × workstreams grid |
| Single-track phased roadmap | `phase-roadmap` | Phases + timeline bar + dates + per-phase description (+ milestones) |
| Cross-functional swimlanes | `swimlane` | Multiple parallel tracks |
| Executive summary (problem framing) | `scqa-summary` | 4-row Situation/Complication/Questions/Answer narrative arc |
| Executive summary (key messages) | `exec-summary` | 3–5 bold lead-in conclusions (≤90 chars) each with one supporting sentence (≤200 chars), rules between rows, optional `bottom_line` ask (a pointing accent flag labelled BOTTOM LINE, then the statement in a tinted box); answer-first rather than an SCQA arc |
| Deck section list | `agenda` | Numbered section outline |
| Visual deck preview | `agenda-with-images` | Numbered agenda rows with image/quote placeholders alongside the title (3–6 items); the placeholder column is all-or-nothing — a row with no `image_label` still gets an empty placeholder |
| Team / 'Our People' page | `team-bios` | 1–8 named people with a headshot (or initials placeholder) + role + short bio, up to 4 per row |
| Joint-venture / engagement-team paired roles | `dual-org-ladder` | Two parallel columns of 2–6 paired role cards with an org-name header above each column (optional connector line per row) |
| Icon + caption row | `icon-row` | Visual categories, 3–5 items |
| Photo / case study beside text | `image-text-split` | One `image` (`path` resolved against the deck dir, or `url`) beside eyebrow + heading + body + ≤5 bullets and 0–3 result `metrics`; `image_side` left/right, `image_width_pct` 30–60. Without an image it draws a dashed placeholder (`image_label`). Implements `ImageAssetPattern` so hosts resolve its image like a shape_grid image cell |
| Callout / testimonial | `pull-quote` | Attributed quotation, optionally beside a headshot |
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
3. Report what still does not fit from `PostExpandWarnings` as
   `TEXT_EXCEEDS_SHAPE`. That code from a pattern is **blocking**
   (`shrink_or_split`), because the pattern has measured the failure rather
   than estimating it — see docs/FIT_FINDINGS.md.

The author's fix is a shorter label or fewer steps. That is a real constraint,
not a defect to engineer away: ten columns across a 13.3" slide leave about
65pt of text width, which is nine or ten characters at 12pt.

### Pictures in pattern values

Three patterns take a real picture in their `values`, all through the same
`{path | url, alt}` reference (`PhotoSchema` / `validatePatternPhoto` in
`internal/patterns/pattern_photo.go`):

| Pattern | Field | Without it |
|---------|-------|------------|
| `image-text-split` | `values.image` | Dashed wireframe placeholder labelled with `image_label` |
| `team-bios` | `values.members[].photo` | Initials tile (`photo_label`, else initials derived from `name`) |
| `pull-quote` | `values.image` (+ `overrides.image_side`, `overrides.image_width_pct`) | No picture column at all — the quote keeps the full width |

Rules a new picture-taking pattern must follow:

- Implement `ImageAssetPattern`. Its `ImageAssets` must return refs that point
  **into** the decoded values (not copies), because the host rewrites
  `Path` in place when it resolves a relative path or downloads a URL. A ref
  returning a copy silently discards the resolved path.
- The `Field` of each ref is the JSON pointer under `values`
  (`"image"`, `"members/0/photo"`), which the host prefixes with
  `/slides/N/pattern/values/` when it reports a finding against it.
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
- **Content-structured layouts** (bmc-canvas, agenda, agenda-with-images, roadmap-phased, phase-roadmap, scqa-summary, swimlane, timeline-horizontal, team-bios, quote-cluster, dual-org-ladder, table-highlight, image-text-split): cell fills are determined by content structure (lanes, phases, sections, member cards, quote bubbles, org-paired rows, highlighted table row/column) rather than peer ordering.

Each non-grid pattern should document in its `UseWhen`/`NotWhen` text or code comments why it does not expose the override.

### Test guidance

Every grid-shaped pattern must include a table-driven test exercising all three modes (`uniform`, `alternate`, `progressive`) against at least two different base accents (e.g., `accent1` and `accent3`). Verify that the emitted cells carry the expected accent strings. See `overrides_test.go::TestResolveCellAccent` for the shared function tests; pattern-level tests should exercise the full `Expand()` path.

## Cell Capacity Contract

The engine computes a deterministic text budget for every shape grid cell. Pattern authors do not implement capacity logic — the `internal/textcapacity` package derives budgets externally from the resolved grid geometry.

### Core rules

1. **`Expand()` must remain pure.** A pattern's `Expand(values, overrides, ctx)` converts structured values into a `*ShapeGridInput`. It must not call `textcapacity` or perform any capacity calculations. Capacity is computed downstream by the expand command or MCP tool after `Expand()` returns.

2. **Density is a HEIGHT ratio; `max_chars` is a derived hint.** `DensityPct` is the measured height of the wrapped text block — every paragraph laid out at **its own** font size, summed — over the height the cell offers. `max_chars` remains as a sizing hint, computed at the cell's *dominant* size (the one carrying the most characters).

   It used to be a character ratio against a single size, the **largest** paragraph in the cell, which made every mixed-size cell nonsense in both directions: a `stat-hero` cell with a 120pt number above three small support lines reported 911% "overflow" while rendering with room to spare, and most cells of most patterns reported "underfilled" (go-slide-creator-yj77). An unsized cell is also now measured at the size it renders at (`shapegrid.DefaultTextSizePt`, 14pt) rather than a legacy 11pt budget default.

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
| Underfilled | < 35% | `"underfilled"` | Add content or pick a smaller grid |
| Optimal | 35–110% | `"optimal"` | No action needed |
| Overflow | > 110% | `"overflow"` | The renderer will shrink this cell's text to fit (`<a:normAutofit/>`). Trim content or pick a larger grid if the shrink would push text below the readable floor. |

Density % is `required text height / available text height`, so >100% means "needs an autofit shrink", not "clipped". Two further signals separate those cases:

- **`fits: false`** (Density.Fits) — the block does not fit even at the smallest shrink the renderer applies (`textcapacity.AutofitFloorScale`, 20%). This, and only this, is what `fit_overflow` reports for a shape_grid cell: text that is actually clipped.
- **`TEXT_BELOW_READABLE_MIN`** — the predicted post-autofit size is under the viewing mode's floor for that text role. This is the finding for "it fits, but only because it shrank too far"; it is advisory (`review`).

These thresholds are defined in `internal/textcapacity/textcapacity.go` and are stable — do not hardcode different values in patterns.

## Highlight fills must be chosen by measurement (authoring contract)

A pattern whose one semantic signal is a highlighted cell must not paint it from a fixed scheme slot. Contrast between two slots is template-dependent: value-chain painted steps `dk2` and the highlighted step `accent2`, which is 3.21:1 on midnight-blue and **1.48:1 on warm-coral** — the highlight simply disappeared, and no test could see it because the pattern was tuned on one template (go-slide-creator-ah5s).

- Pick the default with `pickDistinctFill(ctx, base, fillDistinctnessMin, candidates...)` (`internal/patterns/fill_contrast.go`): it returns the first candidate whose EFFECTIVE colour (tints and alpha composited) clears 3:1 against the base fill. List candidates in preference order — the brand accent first — and you lose your preference only to a measurement.
- Without a theme there is nothing to measure: fall back to the historical default so an expansion with no template is unchanged.
- An AUTHORED highlight is always honoured, and reported through `PostExpandWarnings` as `LOW_CONTRAST_HIGHLIGHT` when it measures below the bar.
- Measured bars for the bundled templates live in `internal/patterns/valuechain_highlight_test.go`; `cmd/json2pptx/value_chain_highlight_test.go` runs the same rule against the real `templates/*.pptx`, so a palette change is caught rather than shipped.

## A pointed shape's text has to be inset past its own point

A chevron, a right arrow and a home plate all draw their point INSIDE the bounding box, so text laid out to that box is drawn into the notch and the tip. `process-flow`'s chevron step type did this — the first and last characters of a label disappeared into the geometry — and its row connector was drawn straight through the shape (go-slide-creator-czk4; `numbered-step-strip` had the same fix in round 1).

- Set `adjustments: {"adj": chevronAdj}` (30% rather than the OOXML default 50%) and inset the text by the notch depth plus a few points on BOTH sides.
- The notch is `adj x the SHORTER side`, so compute it from the pattern's own cell geometry — borrowing another pattern's number is how an inset ends up 16pt short. A row of pointed shapes should also cap its height at half its step width, or the shape's own height sets the notch and the label is left a column.
- Drop the connector between pointed steps: they already say which way the flow runs, and the arrow was drawn through the notch.
- **Reconcile the inset with the shape width, and measure against what the renderer actually gives the text.** `numbered-step-strip`'s chevrons kept a fixed 30% notch as the strip got denser: at 6 steps a chevron is 131pt wide and the inset took 47pt of it, so "Qualification" rendered as "Qualific / ation" on midnight-blue and warm-coral — with no finding, because two lines still fit inside the shape (go-slide-creator-e97v). Three things were wrong at once, and all three are easy to repeat:
  - The step width was the column's share of the content area. A grid `gap` of `0` reads as "unset" in the DTO and resolves to shapegrid's **8pt default**, so each shape is `(contentW - 8pt × (n-1)) / n` wide, not `contentW / n`.
  - The renderer lays text out inside the chevron's OWN text rectangle, already pulled past the point and the notch; the `lIns`/`rIns` the pattern emits stack on top. Budget **two notches plus the inset** per side, or the measurement is nearly double the real width.
  - The measurement runs on whichever font the machine has (Liberation Sans substitutes for a missing template font) while the renderer uses its own. Require the label to fit inside ~90% of the computed width; the last few points are not knowledge you have.
- Give up notch depth BEFORE type size: a blunter arrow still reads as an arrow, and short labels keep the full 30% because the search starts there and stops at the first depth that fits. Shrink the label only after the shallowest allowed notch, never below the renderer's readable floor, and emit a `BODY_TOO_LONG` advisory from `PostExpandWarnings` when even that wraps — at that point the label is too long for the step count and only the author can fix it.

## A field's maxLength is the budget of the pattern's SMALLEST shape

A pattern's JSON schema is the contract an agent sizes its copy against, and a per-field `maxLength` can only state one number. For a pattern whose cell count varies, that number is necessarily the budget of the *smallest* grid: card-grid's 300-character body is readable in a 1x1 and renders at **2.6pt in a 5x5**. Content that respects the schema in every particular is still a wall of unreadable text, and the schema cannot say so (go-slide-creator-0g6p).

- Keep the `maxLength` at the small-shape budget (it is a real bound: past it even one card overflows) and say in the field's description that denser shapes hold less, with the numbers.
- Emit the shape-scaled budget as a `BODY_TOO_LONG` warning from `PostExpandWarnings`, naming the shape and the number the author has to hit — "a 4x3 grid holds about 60 per card". `TEXT_BELOW_READABLE_MIN` already says the text will shrink; it does not say how much is affordable.
- Derive the budget by MEASUREMENT, not by arithmetic: run the payload at each shape through the same readability prediction the fit report gives an agent. `cmd/json2pptx.TestSchemaMaximaStayReadable` does this for every registered pattern on all four bundled templates and pins the result, so a schema maximum cannot quietly get worse and an improvement cannot be given back.
- card-grid's measured table: 1x1–2x2 → 300, 3x2 → 220, 4x2 → 160, 3x3 → 100, 4x3 → 60, 5x3 → 40, 4x4 and denser → 20.

## Text on a tinted fill must be chosen by measurement too

The same rule applies to the text a pattern paints INSIDE a fill it tints itself. `timeline-horizontal` tints each bar of its gantt and chevron chains — shade 70000 at the first stop through tint 40000 at the last — and hardcoded `lt1` inside every one of them, so the lightest bar measured **1.54:1** in a real midnight-blue render and its date label was invisible (go-slide-creator-5qotm).

- Ask `readableTextOn(ctx, tone, fallback)` (`internal/patterns/fill_contrast.go`) which of the light / dark text roles reads on the fill's EFFECTIVE colour. Without a theme, `timeline-horizontal` uses `dk2` on tinted links and `lt1` on darker links; a portable expansion must not bake white text onto a pale tint.
- Build the fill from a `fillTone` and emit it with `tone.fillJSON()`, so the tone you measured and the fill you paint cannot drift apart.
- `tint` and `shade` are **linear-light mixes** toward white and black, not the HSL lightness that `lumMod` / `lumOff` act on. `EffectiveColorMods` models all four; both transforms are pinned against measured render pixels in `internal/patterns/timeline_contrast_test.go`. A model that treats a tint as a lumMod is out by 30-50 per channel, which is the difference between "readable" and "invisible".
- In `timeline-horizontal` chevrons, measure the label and body against the chevron's usable text width and capped row height. Emit `BODY_TOO_LONG` with the available body-line count when the description would clip; a raw character limit misses narrow seven-stop layouts.

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
- `position` — `left`, `top`, or `center`. Defaults to the pattern-specific position (kpi-Nup → `left` on landscape cards (width ≥ 1.2× height) and `top` on square/narrow cards; kpi-inline → `left`; card-grid/iconrow/herodetail/matrix → `top`) when blank. On any shape, a `left` overlay icon is capped at 25% of the shape width (the text's extra left inset is icon + 6pt padding), and a default-scale `top` icon on a landscape shape is capped at 40% of the shape height.
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

The `chart-insights-split` pattern is the canonical "chart on the left, takeaways on the right" consulting layout. The pattern emits a 65/35 column split: the left panel is a `Diagram` cell rendered by svggen; the right panel is a Shape cell with the title (defaults to `Key Insights`) and 1–6 bullet takeaways. **The default widens to 75/25 when the insights column is sparse** — at most two bullets, ≤140 characters in total, and no headline or so-what — because a 35% column holding one short bullet leaves a large empty block while the chart is squeezed into 55% of the slide (go-slide-creator-pyxn). A thin vertical accent divider can be toggled via `overrides.show_divider`, and `overrides.chart_width_pct` (clamped 40–80) pins the ratio, overriding both defaults.

For readability the right panel applies vertical rhythm via per-paragraph `space_after` (points): the title carries extra separation below it so it reads as a header, and non-final bullets carry inter-bullet breathing room so the column does not render as a dense block. `space_after` is a general field on the shape-grid `paragraphs[]` cell-text form (points, converted to hundredths of a point), available to any pattern that emits paragraph arrays.

`values.chart` accepts any svggen `DiagramSpec` payload or the flat `{label: value}` shorthand that `chart_value` accepts (e.g. `{"type": "bar", "data": {"Q1": 12, "Q2": 14}}`); the shorthand is normalized to `categories`/`series` at decode time, preserving key order. Validation expands the pattern and dry-renders the chart, so `validate_input` rejects exactly the charts `generate_presentation` would.

`values.chart` is **optional**. When omitted, the pattern collapses to a single-column insights cell at 100% width and emits the structured warning `CHART_PLACEHOLDER_EMPTY: chart-insights-split rendered insights-only; provide a chart spec to fill the left panel` via the `PostExpandWarner` interface. Every surface converts that warning into a `FitFinding` with `code = "CHART_PLACEHOLDER_EMPTY"` and `action = "review"`: `validate_input`, `generate_presentation(fit_report=true)`, `score_deck`, `preview_presentation_plan` and the `validate --fit-report` / `generate --json-output-report` CLI, plus `expand_pattern`'s own `warnings[]`. Until go-slide-creator-wn4v only preview did, so the documented validate → generate loop called a 75%-empty slide clean. Agents should either supply a chart spec or switch to an insights-only pattern (e.g. `card-grid`, `pull-quote`).

`values.chart` is a regular `types.DiagramSpec` — pass the same shape used in slide-level diagram content (`type` + `data`, optional `title` / `style`).

**So-what extensions (go-slide-creator-pzrs).** Consulting chart slides state the figure and the implication, not just the bullets:

- `headline` `{value ≤12, label ≤60}` — a big accent number (32pt, 26pt with ≥5 insights; `overrides.headline_size`) at the top of the insights column.
- `so_what` (≤160) — a tinted, accent-barred callout ("**So what:** …") at the bottom of the column.
- Chart caption — single-series charts render without a legend, so the series name used to disappear. A bold caption above the chart now shows `chart_label` (≤60) or, when the chart has no `title`, the single series name plus `unit` (≤12): `"Revenue"` + `"$M"` → `Revenue ($M)`; multi-series charts get `Values in <unit>` (their legend names the series).
- Data labels — `Style.ShowValues` is switched on by default for bar-type charts with ≤16 points and single-series line / area charts with ≤12 points; `overrides.data_labels` forces on / off and an explicit `data.data_labels` payload is left to svggen. The caller's chart spec is never mutated.

With a headline or so-what the insights cell becomes a nested column (headline / insights / so-what rows sized from the measured text) and the vertical divider is omitted; without them the panel is unchanged. The source row is pinned at 30pt so its 12pt floor never autofits.

## Bounds Override

Patterns assume `full_content_area` by default — the grid fills the entire layout content area. For patterns with short content this produces oversized cells. Constrain the grid with:

- **`max_height_pct`** (number, 1–99): constrains grid height to this percentage of the content area.
- **`bounds`** (object: `{x, y, width, height}` as percentages of slide dimensions): explicit bounding rectangle.

These fields live on `PatternInput` (slide-level JSON) and on the `expand_pattern` MCP tool parameters. When set, the expanded grid gets a `bounds` field on the `ShapeGridInput`, which the shapegrid resolver and density math respect automatically.

`bounds` takes priority over `max_height_pct`. If neither is set, the grid uses the full content area (backward-compatible default). A user `bounds` / `max_height_pct` override also resets the grid's `vertical_align` to `stretch`, so the block stays where the user put it.

## Content-sized heights and vertical centring (authoring contract)

Do not let cards / steps stretch to the full content height just because the grid is full-area. Every expanded pattern grid gets `vertical_align: "center"` (`patterns.ApplyGridDefaults`, applied by `expandPattern` and every other `Expand` caller), and the shapegrid resolver keeps a content-sized block when **at least one row sets `max_height`** (points):

- Cap rows in points, not percentages, derived from `contentAreaPt(ctx)` (e.g. kpi-Nup `0.45 × content height`, process-flow `0.35 ×`), or from a text estimate (`textBlockHeightPt`) for text rows. Point caps keep nested / composed use sane: inside a small compose cell the cap exceeds the cell and the row simply fills it.
- Be precise about which bound a fraction is. kpi-Nup's `0.45` is a **base**, not a cap: `kpiRowMaxHeightPt` passes it to `clampPt` as the *lower* bound and raises the row to whatever the tallest card's measured content needs, bounded only by the content area. It was called `kpiMaxCardHeightFrac`, so every doc and reviewer assertion of a "45% cap" was false (go-slide-creator-4uxi); it is now `kpiBaseCardHeightFrac`. If a pattern needs a true ceiling, say so and make `clampPt`'s `hi` that value — but note a row can never exceed the content area anyway, so a tighter ceiling only clips cards whose text genuinely needs the height.
- Fixed `height` percentages in a grid that has a capped row are kept as absolute shares (no re-normalisation), so the slack is real and the block is centred.
- Grids without any capped row keep the legacy proportional stretch (agenda lists, stacked steps, team-bios rely on it).
- Height-capped, top-anchored pattern `bounds` (`y: 0`, `height < 100`) are centred inside the content area under `vertical_align: "center"`; the content area already excludes the takeaway/source chrome band.
- Big single-token values (KPI numbers) must shrink to fit one line (`fitSingleLineSize`) rather than wrap.
- Header bands: fix the row with `min_height = max_height = headerRowPt(...)` (~1.2× the header line height + padding), never a percentage of the grid.
- Cards: size with `contentCardHeightPt(shapeTextHeightPt(...), cardW, hasTopIcon)` and pass sparse card text through `anchorSparseText` so a short body is centred instead of hanging top-left (target: < 30% unused area per card). Skip content-sizing for rows that host a secondary chart.
- **A row's height is the row's, not the cell's.** Every card in a grid row is as tall as the tallest one, and without a `max_height` the row also stretches to fill the content area. Both together put a one-line quote in the top fifth of a tall tinted box (go-slide-creator-pr3g). `contentSizedRow(ctx, cells, cols)` is the shared answer — it sizes the row to the tallest card's own text and runs every sparse card through `anchorSparseText` — and `card-grid`, `quote-cluster` and `icon-row` all go through it. A text-only row (value-chain's descriptions) needs the same cap without the card padding: measure with `shapeTextHeightPt` and set `MaxHeight` directly.
- **A fill of `lt1` is invisible.** `strategy-house` filled its pillars white on a white slide, so a pillar column vanished below its last bullet and the gap to the foundation read as empty space rather than as the pillars holding the house up. Use `ctx.ResolveSurface("subtle", "lt2")` for a panel that must read as a surface.
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
