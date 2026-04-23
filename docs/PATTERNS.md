# Pattern Authoring Guide

How to add a new named pattern to `internal/patterns/`.

## When to add a pattern

A pattern is justified only when its `shape_grid` expansion is reused across **3 or more example decks**. If a layout appears in fewer than three decks, use `shape_grid` directly. This rule prevents pattern proliferation.

## File naming

Follow the existing convention in `internal/patterns/`:

| Pattern name | File |
|---|---|
| `kpi-3up` | `kpi3up.go` |
| `bmc-canvas` | `bmccanvas.go` |
| `timeline-horizontal` | `timelinehorizontal.go` |
| `card-grid` | `cardgrid.go` |

Strip hyphens, lowercase. Test file: `<name>_test.go`. If two patterns share helpers (like `kpi-3up` and `kpi-4up`), put shared code in a `_common.go` file (e.g. `kpi_common.go`).

## Contributor checklist

Every pattern PR must include all of these:

- [ ] **Implementation** (`internal/patterns/<name>.go`)
  - Unexported struct implementing `Pattern` interface
  - `init()` registering via `Default().Register(&myPattern{})`
  - Typed `Values`, `Overrides`, `CellOverride` structs
  - `Schema()` returning a hand-authored JSON Schema (see below)
  - `Validate()` using `errors.Join` aggregation
  - `Expand()` returning `*jsonschema.ShapeGridInput`
  - `CellsHint()` (implement `CellDescriber` interface)
- [ ] **Tests** (`internal/patterns/<name>_test.go`)
  - Metadata: `Name()`, `UseWhen()` non-empty (D6), `Version()`
  - Schema validity: marshals to valid JSON Schema draft 2020-12
  - Validate: happy path, wrong count with sibling hint (D4), missing required fields, max length exceeded, invalid cell override keys
  - Expand: default accent, accent override, cell override application
- [ ] **Golden file** (`internal/patterns/testdata/<name>/default.golden.json`)
  - Created by running tests with `UPDATE_GOLDEN=1 go test ./internal/patterns/ -run TestMyPattern/golden`
  - Committed alongside the code
- [ ] **Smoke entry** in `examples/patterns-smoke.json`
  - One slide exercising the pattern with representative values
- [ ] **use_when text** reviewed (see below)

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

## Writing use_when text (D6)

The `UseWhen()` string is an **anti-misuse guardrail**, not just a description. It tells agents (and humans) the specific scenario where this pattern is the right choice. Guidelines:

- Be prescriptive: state when to use, not what it does
- Mention the expected data shape (e.g. "Three big-number KPIs with short captions")
- Call out when a sibling pattern is better (e.g. `bmc-canvas`'s use_when explicitly says "for general feature cards prefer `card-grid`")
- Keep it under one sentence

## Expand conventions

- Emit scheme color strings (`"accent1"`, `"dk1"`), never hex values. Theme resolution happens downstream via `pptx.ResolveColorString`.
- Use `json.RawMessage` for fill and text content fields in `ShapeSpecInput`.
- Default accent is `"accent1"` unless overridden.
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
