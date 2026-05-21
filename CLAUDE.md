# json2pptx

Go CLI and library for generating PowerPoint presentations from structured JSON input.

## Quick Reference

```bash
# Build
make                                    # Build all binaries
go build ./cmd/json2pptx                # Build just the main CLI

# Test
go test ./...                           # All tests
go test ./internal/generator/...        # Specific package
cd svggen && go test ./...              # SVG generation (separate go.work module)

# Lint (MUST pass before committing)
golangci-lint run ./...
cd svggen && golangci-lint run ./...

# Generate a deck
json2pptx generate -json examples/basic-deck.json -template midnight-blue -templates-dir templates -output /tmp/out

# Convert to images (needs LibreOffice + ImageMagick)
pptx2jpg -input /tmp/out/basic-deck.pptx -output /tmp/slides/ -density 150

# Validate
json2pptx validate examples/basic-deck.json
json2pptx validate-template templates/midnight-blue.pptx
```

## Before Committing

Always run this checklist before declaring work complete:

1. `golangci-lint run ./...` -- fix all lint errors in main module
2. `cd svggen && golangci-lint run ./...` -- fix all lint errors in svggen module
3. `go test ./...` -- all tests pass in main module
4. `cd svggen && go test ./...` -- all tests pass in svggen module
5. `go build ./cmd/json2pptx` -- binary builds cleanly
6. **Skill / docs / code sync verified** -- see policy below

## Skill, Docs, and Code Sync (mandatory)

The agent-facing skill (`skills/generate-deck/SKILL.md`) and contributor-facing docs (`docs/FIT_FINDINGS.md`, `docs/PATTERNS.md`, `docs/STYLE_DEFAULTS.md`, `docs/INPUT_FORMAT.md`) are part of this repo. They are not external. Any change that alters an agent-visible or contributor-visible surface MUST update them in the same commit.

**Surfaces that count as agent-facing (require SKILL.md update):**

- CLI / MCP response shapes for `generate`, `validate`, `validate_input`, `expand_pattern`, `show_pattern`, `list_patterns`, `list_templates`, `analyze_deck_rhythm`, `recommend_visual`, `plan_deck`, `repair_slide`
- Fit-report finding codes, severities, action semantics (`fix.kind` and `params`)
- JSON-schema additions to slide / pattern / overrides shapes (new fields, new enum values)
- Template metadata fields readable from the engine (e.g., `accent_usage_guide`, `color_roles`)
- Recommendation message formats from `analyze_deck_rhythm`

**Surfaces that count as contributor-facing (require `docs/` update):**

- New finding codes -> `docs/FIT_FINDINGS.md`
- New pattern overrides, new pattern authoring contract rules -> `docs/PATTERNS.md`
- Deck-level defaults, swap semantics, scope rules -> `docs/STYLE_DEFAULTS.md`
- Top-level JSON shape changes -> `docs/INPUT_FORMAT.md`

**The rule (symmetric):**

1. Code change to an agent-facing surface without `skills/generate-deck/SKILL.md` update in the same commit is incomplete.
2. SKILL.md or `docs/` change describing non-existent code is incomplete -- either land the code in the same commit or revert the doc change.
3. Pre-commit / pre-PR check: open SKILL.md and grep for any string the diff added or removed (finding code, response field, override key). If it should be there and isn't, update SKILL.md before declaring done.

**Exemption:** If a code change is a pure refactor (rename, reorganization) with no agent-visible behavior change, add a commit trailer `Skill-Sync-Exempt: <reason>` (reason ≥ 20 chars). Heuristic CI checks honor the trailer; reviewers verify the claim.

**Why:** drift here is silent. The engine keeps working, but agents stop using new capabilities and start citing removed ones. A single PR that breaks sync costs more to fix later than the seconds it took to update SKILL.md alongside the code.

## Project Structure

```
cmd/
  json2pptx/      # Main CLI (generate, validate, serve, mcp)
  pptx2jpg/       # PPTX to JPG conversion via LibreOffice
  mktemplate/     # Template creation helper
  debugcolors/    # Theme color debug tool
  templatecaps/   # Template capabilities inspector
  validatepptx/   # PPTX validation tool
  testrand/       # Random deck generator for testing

internal/
  generator/      # Core PPTX generation engine (slide_preparation, text_contrast, shapes)
  pptx/           # Low-level OOXML manipulation (XML types, fills, bullets)
  template/       # Template parsing (themes, fonts, layouts)
  shapegrid/      # Shape grid layout engine
  types/          # Shared data types (Presentation, Slide, Template)
  api/            # HTTP API server
  layout/         # Layout matching and selection
  textfit/        # Text fitting and overflow handling
  pagination/     # Slide pagination / content splitting
  pipeline/       # Generation pipeline orchestration
  visualqa/       # Visual QA agent integration
  resource/       # Embedded resource handling
  config/         # Configuration
  safeyaml/       # Safe YAML parsing
  testrand/       # Random test data generation
  testutil/       # Test helpers
  utils/          # Utilities

svggen/           # SVG chart/diagram generation (separate Go module via go.work)
  charts.go       # Bar, line, pie, etc.
  contrast.go     # WCAG contrast calculations
  style.go        # Theme-aware styling

templates/        # PPTX template files (4: forest-green, midnight-blue, modern-template, warm-coral)
examples/         # Example JSON input files (19 decks)
```

## Key Architectural Decisions

- **Template-driven**: All visual identity comes from `.pptx` template files. The engine never hardcodes colors/fonts.
- **Semantic colors**: JSON uses scheme names (`accent1`, `lt2`, `dk1`) not hex. The engine resolves via the template's theme.
- **Contrast enforcement**: `internal/generator/text_contrast.go` auto-fixes low-contrast text on layout backgrounds (WCAG AA). Shape grid text is warn-only (user-specified colors preserved).
- **SVG for charts/diagrams**: The `svggen/` module renders charts as SVG, embedded as EMF in the PPTX.
- **Shape grids**: Complex layouts (BMC, KPI dashboards, timelines) use `shape_grid` in JSON, rendered by `internal/shapegrid/`.
- **Named patterns**: Reusable `shape_grid` skeletons in `internal/patterns/` that expand at generation time. See [docs/PATTERNS.md](docs/PATTERNS.md) for the authoring guide.
- **Fit findings**: Content overflow and density diagnostics emitted by `validate -fit-report` and MCP `generate_presentation(fit_report=true)`. See [docs/FIT_FINDINGS.md](docs/FIT_FINDINGS.md) for the code catalog, action semantics, and scope rules.
- **Style defaults**: Deck-level `defaults` block for `table_style` and `cell_style`, shallow-applied before validation. See [docs/STYLE_DEFAULTS.md](docs/STYLE_DEFAULTS.md) for swap-only semantics, scope rules, and the `@template-default` sentinel.

## Testing Notes

- Golden file tests use `testdata/` directories within packages
- Font metrics differ across platforms (macOS vs Linux CI) -- some tests use `t.Logf` instead of `t.Errorf` for font-dependent assertions
- `svggen/` is a separate module -- run its tests with `cd svggen && go test ./...`
- CI runs on GitHub Actions (`.github/workflows/ci.yml`)

## Templates

4 bundled templates: `forest-green`, `midnight-blue`, `modern-template`, `warm-coral`. Each has its own theme colors, fonts, and slide layouts. Use `json2pptx validate-template` to inspect.

Template rules are documented in two canonical docs — link to them rather than restating their detail here:

- **Using a template** (placeholder IDs, layout tags, layout→slide-type mapping) → [skills/template-deck/TEMPLATE_GUIDE.md](skills/template-deck/TEMPLATE_GUIDE.md)
- **Authoring / validating a template** (mandatory layouts, theme requirements, conformance checks) → [docs/TEMPLATE_SPEC.md](docs/TEMPLATE_SPEC.md)

## Common Patterns

Quick reference only — the full placeholder/layout catalog lives in [skills/template-deck/TEMPLATE_GUIDE.md](skills/template-deck/TEMPLATE_GUIDE.md).

- Slide types: `title`, `content`, `section`, `two-column`, `blank`, `chart`, `diagram`
- Placeholder IDs: `title`, `subtitle`, `body`, `body_2` (portable across templates)
- Content types: `text`, `bullets`, `chart`, `diagram`, `table`, `image`, `body_and_bullets`, `bullet_groups`

### Named Patterns (registered in `internal/patterns/`)

| Pattern | Description |
|---------|-------------|
| `agenda` | Numbered section list for agenda / table-of-contents slides |
| `agenda-with-images` | Numbered agenda rows (3–6) with title/subtitle and image/quote placeholder per row |
| `arch-stack` | Architecture stack diagram with tiers and optional side rails |
| `before-after` | Two-column before/after with transition chevron |
| `before-after-compact` | Compact before/after, height-capped at ~60% for brief content |
| `bmc-canvas` | Formal 9-cell Business Model Canvas (Osterwalder) |
| `card-grid` | Parameterized N×M grid of titled cards |
| `chart-insights-split` | Left chart panel + right insights column (65/35 split); falls back to insights-only when chart is omitted, emitting `CHART_PLACEHOLDER_EMPTY` |
| `comparison-2col` | Two-column comparison with optional headers |
| `driver-tree` | Value / cost driver tree: root metric → 2–4 branches → 1–4 leaf items each, with optional per-branch annotations and connector lines (use svggen `org_chart` for people/role hierarchies) |
| `dual-org-ladder` | Two parallel org columns with 2–6 paired role cards and an org-name header above each column (joint-venture / engagement-team slides) |
| `horizontal-bar-with-callouts` | Ranked horizontal bars (3–8) on the left with a per-bar accent-anchored insight callout on the right |
| `icon-row` | Horizontal row of icon+caption pairs |
| `journey-maturity-model` | Horizontal maturity ladder of 3–6 stage columns with numbered headers, descriptions, and an optional 'where we are' marker on the current stage |
| `kpi-2up` | Two big-number KPI cards with short captions |
| `kpi-3up` | Three big-number KPI cards with short captions |
| `kpi-4up` | Four big-number KPI cards with short captions |
| `kpi-5up` | Five big-number KPI cards with short captions |
| `kpi-6up` | Six big-number KPI cards with short captions |
| `kpi-inline` | Horizontal inline KPI bar, height-capped for supporting context |
| `matrix-2x2` | 2×2 quadrant matrix with axis labels |
| `process-flow` | Left-to-right process flow with steps and decision points |
| `process-flow-compact` | Compact process flow, height-capped at ~35% for short labels |
| `process-grid-2row` | Two parallel process tracks: dk1 row-label column on the left + 3–6 equal-width phase boxes per row (e.g., Design / Production, Strategy / Execution) |
| `pull-quote` | Italic quote block with attribution |
| `phase-roadmap` | Single-track phased roadmap: phase boxes + timeline bar + date labels + per-phase descriptions + optional milestones |
| `pyramid` | Stacked trapezoid hierarchy (3-5 tiers) |
| `quote-cluster` | Structured 3-column grid of 3–8 attributed stakeholder quote bubbles (voice-of-customer slides), with alternating tinted fills |
| `roadmap-phased` | Phased roadmap with workstreams and time periods |
| `scqa-summary` | 4-row SCQA executive summary (Situation / Complication / Questions / Answer) |
| `stat-hero` | Single oversized statistic with label and optional context |
| `strategy-house` | Strategy-house framework: objective banner + 3-5 pillars + foundation row (optional roof badges) |
| `stylish-panels` | Accent-banded panels with ribbon headers for pillars, capabilities, or workstreams |
| `swimlane` | Horizontal swimlane diagram with actors and steps |
| `team-bios` | Team / 'Our People' grid of 1–8 members with photo placeholder + name + role + short bio (up to 4 per row); emits `BODY_TOO_LONG` when a bio exceeds the ~2-line budget |
| `timeline-horizontal` | Linear horizontal timeline with stops |
| `value-chain` | Horizontal value chain of 4–10 step columns (bold label + per-step description, optional highlight) |
| `waterfall-bridge` | Waterfall / bridge bar chart of 3–10 columns showing P&L walks or cost-driver decomposition; floating delta bars with auto-computed subtotals |

### Key Top-Level Fields

- `accent_strategy` — controls accent color rotation: `"primary"` (default), `"rotate"`, `"section-keyed"`
- `compose` — slide-level nested pattern composition envelope (multiple patterns on one slide)

### Composition Awareness

- Use `plan_deck` MCP tool as the recommended entry point for new decks (produces a structured slide plan)
- Use `analyze_deck_rhythm` MCP tool to detect visual monotony, accent imbalance, and missing emphasis
- Use `recommend_visual` MCP tool to rank candidate layouts/patterns/charts for a given slide intent


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
