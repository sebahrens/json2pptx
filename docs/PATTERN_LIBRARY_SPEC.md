# Pattern Library — Final Spec v3

> Final spec. v0 (seed) + v1-golang (engineering) + v2 (review synthesis) + codex debate consensus all incorporated. Decisions are now firm. Ready to be sliced into beads.

## 1. Problem

`shape_grid` JSON costs ~600 model tokens per non-trivial slide. ~70% of complex slides in `examples/` reuse a small set of layouts (KPI grids, BMC, 2×2 matrix, timeline, card grid, icon row, two-col compare). A named-pattern primitive replaces ~600 tokens with ~100 by lifting recurring `shape_grid` skeletons into the binary, theme-aware and validated.

## 1.1 Naming convention & aliases

Canonical pattern names follow the form **`{noun}-{qualifier}`**:

| Qualifier style | Examples |
|---|---|
| count + suffix | `kpi-3up`, `kpi-4up` |
| dimensions | `matrix-2x2` |
| column count | `comparison-2col` |
| orientation | `timeline-horizontal` |
| compound noun | `card-grid`, `icon-row`, `bmc-canvas` |

When a pattern is the only variant of its noun (e.g. only one timeline layout exists), the registry provides a **short alias** that drops the qualifier:

| Alias | Canonical |
|---|---|
| `timeline` | `timeline-horizontal` |
| `bmc` | `bmc-canvas` |
| `matrix` | `matrix-2x2` |
| `comparison` | `comparison-2col` |

Aliases resolve transparently in `Registry.Get` — callers never need to distinguish alias from canonical. `Registry.List` returns only canonical patterns (aliases are excluded). `Registry.Suggest` considers both canonical names and aliases for typo correction.

**Rules for adding aliases:**
- An alias MUST NOT collide with any canonical name.
- Only add an alias when there is exactly one variant of a noun. If `timeline-vertical` is added later, the `timeline` alias should be removed.
- Aliases are registered in `internal/patterns/z_aliases.go` (sorted last to run after all pattern `init` functions).

## 2. Final decisions

| # | Decision |
|---|---|
| D1 | **Pattern is a slide-level field (`slide.pattern`), mutually exclusive with `slide.shape_grid`.** A slide carrying `pattern` MUST be treated as non-empty by `computeQualityScore` and any other consumer that today only reads `slide.Content`. |
| D2 | **One accent per pattern via `overrides.accent` (default `accent1`).** Per-cell tinting handled inside `Expand`, not user-tunable globally. |
| D3 | **`layout_id` optional.** Falls back to the existing virtual-layout resolver. |
| D4 | **Length mismatch in `values` is a hard error**, with a sibling-pattern hint in the error payload. No truncation, no auto-promotion. |
| D5 | **Collapse `card-grid-2x3` and `card-grid-3x2` into one parameterized `card-grid`** with required `columns` + `rows`. |
| D6 | **`use_when` field on every pattern.** Surfaced in compact and full discovery. AI-slop guardrail. |
| D7 | **Compact `patterns_compact[]` block in default skill-info** (≤ 40 tokens per pattern). Full schemas only in `--mode=full` or via `show_pattern`. |
| D8 | **`expand_pattern` is a v1 MCP tool** (and CLI `patterns expand`, HTTP `POST /patterns/{name}/expand`). |
| D9 | **Validate parity across CLI/MCP/HTTP**: `validate_pattern` MCP tool + `POST /api/v1/patterns/{name}/validate` HTTP endpoint. |
| D10 | **Structured error shape**: `{path, expected, got, pattern, hint, sibling_patterns?}`. Returned by `validate_pattern`/`generate`; CLI prints both human and `--json` forms. |
| D11 | **Override semantics**: overrides apply during `Expand`, before downstream theme resolution by `pptx.ResolveColorString`. Patterns emit scheme strings (`accent2`), never hex. |
| D12 | **Size metric is heuristic, not tokenizer-exact.** Field renamed `estimated_prompt_size_bytes` (or `relative_size_score`); compute by byte count of the canonical raw `shape_grid` equivalent. **No `tiktoken-go` dependency.** Goldens record the byte counts. |
| D13 | **Hand-authored JSON Schemas per pattern are the authoritative external contract** for agent-facing discovery. Runtime enforcement remains Go validation (`errors.Join`-aggregated `Validate(...) error`); semantic invariants ("exactly 3 cells", "no overlapping spans") live in code and may exceed what the schema expresses. Reflection-based generation, if used at all, is internal scaffolding only — never presented to clients as the contract. |
| D14 | **PR 1 = extract `ShapeGridInput` and friends from `package main` into a new `internal/jsonschema/` package** so `internal/patterns` can import it without a circular dependency. |
| D15 | **`cell_overrides` ships in v1, narrowly scoped.** Only per-cell text/style/decoration adjustments allowed: `accent_bar`, `emphasis`, `align`, `vertical_align`, `font_size`, `color`. **MUST NOT accept arbitrary nested `shape_grid` fragments or geometry changes.** Cells are addressed by zero-based index. Pattern's `Validate` rejects unknown override keys per cell. |
| D16 | **No JSON-side `pattern_version` in v1.** The implementation is selected by `pattern.name` + binary version. A future breaking change ships under a new pattern name OR as additive-compatible extension. The internal `Pattern.Version() int` method exists for tests/debug, but is not serialized. |
| D17 | **Fix `computeQualityScore`** (`cmd/json2pptx/json_mode.go:1130–1140`) so a slide with `pattern` (or `shape_grid`) is not scored empty when `slide.Content` is empty. This is a pre-existing bug surfaced by the debate; bundled with PR 4 (pipeline wiring). |
| D18 | **Envelope-level callout for opt-in patterns.** Patterns that implement `CalloutSupport` accept an optional `callout: {text, emphasis?, accent?}` at the pattern envelope level. The callout renders as a full-width band below the pattern grid. Opt-in design: each pattern author decides whether callouts make sense for their layout by implementing the interface. Callout cells are NOT part of the `cell_overrides` index space — they occupy a separate row appended after expansion, so existing cell indices remain stable. Rationale: a single documentation site for the callout contract (the pattern envelope) avoids per-pattern duplication; opt-in preserves per-pattern authorship control over whether a bottom band is visually appropriate. |

## 3. JSON surface

### 3.1 Slide-level `pattern`

```json
{
  "layout_id": "Title and Content",
  "pattern": {
    "name": "kpi-3up",
    "values": [
      {"big": "$4.2M", "small": "ARR"},
      {"big": "127%",  "small": "NRR"},
      {"big": "12d",   "small": "Sales cycle"}
    ],
    "overrides": { "accent": "accent2" },
    "cell_overrides": {
      "1": { "accent_bar": true, "emphasis": "bold" }
    }
  }
}
```

XOR rule: `pattern` and `shape_grid` cannot coexist on the same slide. Hard error.

### 3.2 Go types (in `internal/jsonschema/`)

```go
type SlideInput struct {
    // ... existing ...
    ShapeGrid *ShapeGridInput `json:"shape_grid,omitempty"`
    Pattern   *PatternInput   `json:"pattern,omitempty"`
}

type PatternInput struct {
    Name          string                     `json:"name"`
    Values        json.RawMessage            `json:"values"`
    Overrides     json.RawMessage            `json:"overrides,omitempty"`
    CellOverrides map[string]json.RawMessage `json:"cell_overrides,omitempty"` // key = "0".."N-1"
}
```

### 3.3 Pattern interface (in `internal/patterns/`)

```go
type Pattern interface {
    Name() string
    Description() string
    UseWhen() string
    Version() int

    NewValues() any
    NewOverrides() any
    NewCellOverride() any                                  // nil if pattern allows no per-cell overrides

    // Schema returns the hand-authored JSON Schema for the pattern's external contract.
    // Authoritative for discovery surfaces.
    Schema() *Schema

    Validate(values, overrides any, cellOverrides map[int]any) error
    Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*ShapeGridInput, error)
}
```

## 4. Discovery parity

| Capability | CLI | MCP | HTTP | skill-info |
|---|---|---|---|---|
| List | `patterns list` | `list_patterns` | `GET /api/v1/patterns` | `patterns_compact[]` (always) |
| Show | `patterns show <name>` | `show_pattern` | `GET /api/v1/patterns/{name}` | `--mode=full` only |
| Validate | `patterns validate <name> <file>` | `validate_pattern` | `POST /api/v1/patterns/{name}/validate` | n/a |
| Expand | `patterns expand <name> <file>` | `expand_pattern` | `POST /api/v1/patterns/{name}/expand` | n/a |

## 5. Pipeline integration

`cmd/json2pptx/pattern_resolve.go`, called from `convertPresentationToSlideSpecs` (`json_mode.go:561–596`), after JSON unmarshal, before `resolveShapeGrid`. Emits a `*ShapeGridInput`, then existing path runs unchanged. Contrast enforcement (`internal/generator/slide_preparation.go:117–125`) may rewrite pattern-emitted colors; emit a debug log when that happens.

## 6. v1 pattern set

| Name | Cells | `use_when` |
|---|---|---|
| `kpi-3up` | 3 | Three big-number KPIs with short captions |
| `kpi-4up` | 4 | Four big-number KPIs |
| `bmc-canvas` | 9 | Formal Business Model Canvas methodology only — for general feature cards prefer `card-grid` |
| `matrix-2x2` | 4 + axes | Quadrant/positioning matrix with axis labels |
| `timeline-horizontal` | N (3–7) | Linear timeline with stops |
| `card-grid` | rows × cols | N×M titled cards |
| `icon-row` | 3–5 | Icon + caption row |
| `comparison-2col` | 2 + header | Two-column compare (pros/cons, before/after) |

Each pattern's `Values`/`Overrides`/`CellOverride` Go structs and hand-authored JSON Schema are formally specified in the per-pattern PR (acceptance criterion).

## 7. Work breakdown — atomic beads

Each row is one bead. Dependencies noted. Acceptance criteria are concrete enough for an implementation agent to execute without further design input.

| # | Title | Type | Priority | Depends on | Acceptance |
|---|---|---|---|---|---|
| 1 | Extract `ShapeGridInput` to `internal/jsonschema/` | task | 1 | — | New package contains `ShapeGridInput`, `GridRowInput`, `GridCellInput`, `ShapeSpecInput`, etc. `cmd/json2pptx` imports from it. All existing tests pass; lint clean. |
| 2 | `internal/patterns/` skeleton | task | 1 | 1 | `Pattern` interface (D6 `UseWhen()`, D15 `NewCellOverride()`/`Validate(...,cellOverrides)`, D13 `Schema()`), `Registry`, `Register`, `Default()`. Duplicate-name panic. Unit tests for registry. No patterns yet. |
| 3 | Hand-authored Schema scaffolding | task | 1 | 2 | `internal/patterns/Schema` type + helpers for declaring object/array/string/number constraints. Test that a sample pattern's schema serializes to valid JSON Schema draft 2020-12. |
| 4 | Implement `kpi-3up` | feature | 1 | 3 | `internal/patterns/kpi3up.go` with values struct, hand-authored Schema, Validate (errors.Join, exact-count, max lengths), Expand (theme-aware), table-driven tests, golden file. |
| 5 | Implement `kpi-4up` | feature | 2 | 4 | Same pattern as #4. Reuses any common helpers via unexported funcs. |
| 6 | Pipeline wiring + computeQualityScore fix | feature | 1 | 4 | `cmd/json2pptx/pattern_resolve.go`. XOR enforcement (pattern vs shape_grid). End-to-end test using `kpi-3up`. **Includes D17**: `computeQualityScore` (`json_mode.go:1130–1140`) treats slides with `pattern` or `shape_grid` as non-empty when `Content` is empty. Test added. |
| 7 | `cell_overrides` resolver in pattern path | task | 2 | 6 | Per-cell override unmarshal + per-pattern validation. Allowed keys whitelisted (D15: `accent_bar`, `emphasis`, `align`, `vertical_align`, `font_size`, `color`). Unknown keys → hard validation error. Test on `kpi-3up`. |
| 8 | `patterns` CLI subcommand | feature | 2 | 6 | `json2pptx patterns list / show <name> / validate <name> <file> / expand <name> <file>`. Both human and `--json` output. Errors use the D10 structured shape. |
| 9 | MCP tools | feature | 2 | 6 | Register `list_patterns`, `show_pattern`, `validate_pattern`, `expand_pattern` in `mcp.go`. Schema in responses is the hand-authored Schema (D13). |
| 10 | HTTP endpoints | feature | 2 | 6 | `GET /api/v1/patterns`, `GET /api/v1/patterns/{name}`, `POST /api/v1/patterns/{name}/validate`, `POST /api/v1/patterns/{name}/expand`. Wired in `internal/api/server.go`. |
| 11 | `skill-info` integration | feature | 2 | 6 | `patterns_compact[]` (≤ 40 tokens/pattern: name, cells, use_when, estimated_prompt_size_bytes) in default mode. Full Schemas in `--mode=full`. |
| 12 | Heuristic size-metric harness | task | 3 | 11 | Compute `estimated_prompt_size_bytes` as len(json.Marshal(rawShapeGridEquivalent)) for each pattern's golden. Recorded in goldens; CI catches regressions. **No tokenizer dependency.** (D12) |
| 13 | Implement `bmc-canvas` | feature | 2 | 6 | Pattern impl + Schema + Validate + Expand + tests + golden. `use_when` text is load-bearing; review-required wording. |
| 14 | Implement `matrix-2x2` | feature | 2 | 6 | Same. Includes axis-label values. |
| 15 | Implement `timeline-horizontal` | feature | 2 | 6 | Same. Variable cell count 3–7; Validate enforces range. |
| 16 | Implement `card-grid` | feature | 2 | 6 | Same. Required `columns` and `rows` value fields; matrix of cells. |
| 17 | Implement `icon-row` | feature | 2 | 6 | Same. 3–5 cells. |
| 18 | Implement `comparison-2col` | feature | 2 | 6 | Same. Two columns + optional header row. |
| 19 | `examples/patterns-smoke.json` + golden e2e | task | 2 | 13–18 | Single deck exercises every pattern. Locked in CI as a golden test on the rendered `.pptx` validity (`shapegrid.Validate` over expanded grids; no visual regression). |
| 20 | Slim `skills/generate-deck/SKILL.md` | task | 3 | 19 | Remove now-redundant Pattern 1–6 prose. Replace with one paragraph + reference to `patterns list`. Verify token cost reduction in skill load. |
| 21 | Documentation: pattern authoring guide | task | 3 | 19 | `docs/PATTERNS.md` covering: contributor checklist (impl + Schema + Validate + Expand + tests + golden + smoke entry), hand-authored Schema convention, cell_overrides scope, ≥3-deck reuse rule. |

PRs 1–6 are critical path. 7–11 parallelizable after 6. 13–18 are N independent PRs after 6. 19 needs all patterns. 20–21 are second-order wins.

## 8. Risks and notes

- Type duplication tax for PR 1 (~1 engineer-day, spans `mcp.go`, `serve.go`, tests).
- Theme contrast may rewrite pattern-emitted colors; debug log required (D11).
- Hand-authored Schemas can drift from Go structs; `Validate` is authoritative — schema is for *discovery*. Add a contributor norm: when `Values` struct changes, Schema must change in the same PR.
- AI slop: addressed by `use_when` (D6) + `sibling_patterns` hints (D10). Worth measuring after v1 ships.
- Pattern proliferation anti-goal: justified only when pattern is reused in ≥3 example decks. PR review checklist.
