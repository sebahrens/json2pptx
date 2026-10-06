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
cd svggen && go test ./...              # SVG generation (separate module; root go.mod replace)

# Lint (MUST pass before committing) — use make lint: it pins the version CI runs
make lint

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

1. `make lint` -- lints BOTH modules with the golangci-lint version CI runs, pinned in `.golangci-version`. Do not run a bare `golangci-lint` off PATH: versions disagree about real findings (v2.8.0's gosec raises G602 where v2.12.2 does not), so a local pass on the wrong version is not evidence CI will pass
2. `go test ./...` -- all tests pass in main module
3. `cd svggen && go test ./...` -- all tests pass in svggen module
4. `go build ./cmd/json2pptx` -- binary builds cleanly
5. **Skill / docs / code sync verified** -- see policy below

## Skill, Docs, and Code Sync (mandatory)

The agent-facing skill (`skills/generate-deck/SKILL.md`), the contributor-facing docs (`docs/FIT_FINDINGS.md`, `docs/PATTERNS.md`, `docs/STYLE_DEFAULTS.md`, `docs/INPUT_FORMAT.md`) and the agent wiki (`docs/wiki/`, whose YAML blocks and `examples/semantic/playbooks/` decks are validated by `internal/semantic` tests) are part of this repo. They are not external. Any change that alters an agent-visible or contributor-visible surface MUST update them in the same commit.

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
- DeckSpec kind fields, split-layout recipes, MCP call sequence, bead references a page cites as "pending" -> `docs/wiki/` (the page that describes the surface; `docs/wiki/README.md` lists them)

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

svggen/           # SVG chart/diagram generation (separate Go module, wired in by a replace directive in the root go.mod; go.work is gitignored)
  charts.go       # Bar, line, pie, etc.
  contrast.go     # WCAG contrast calculations
  style.go        # Theme-aware styling

templates/        # Shipped PPTX templates (see templates/embed.go for the full list)
examples/         # Example JSON input decks (+ diagrams/, semantic/)
```

## Key Architectural Decisions

- **Template-driven**: All visual identity comes from `.pptx` template files. The engine never hardcodes colors/fonts.
- **Semantic colors**: JSON uses scheme names (`accent1`, `lt2`, `dk1`) not hex. The engine resolves via the template's theme.
- **Contrast enforcement**: `internal/generator/text_contrast.go` auto-fixes low-contrast text on layout backgrounds (WCAG AA). Shape grid text is warn-only (user-specified colors preserved).
- **SVG for charts/diagrams**: The `svggen/` module renders charts as SVG, embedded as native SVG with a PNG fallback (default; `--chart-png` forces PNG).
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

`templates/` ships these templates, all embedded in the binary: `abstract`, `blue-corporate`, `business-template`, `forest-green`, `midnight-blue`, `modern`, `modern-template`, `modern-yellow`, `warm-coral`. The four with full cross-template test coverage are `forest-green`, `midnight-blue`, `modern-template`, and `warm-coral`; the others receive smoke and portability coverage. A local gitignored `templates/p-style.pptx` joins cross-template tests when present but is never embedded. Each has its own theme colors, fonts, and slide layouts. Use `json2pptx validate-template` to inspect.

Template rules are documented in two canonical docs — link to them rather than restating their detail here:

- **Using a template** (placeholder IDs, layout tags, layout→slide-type mapping) → [skills/template-deck/TEMPLATE_GUIDE.md](skills/template-deck/TEMPLATE_GUIDE.md)
- **Authoring / validating a template** (mandatory layouts, theme requirements, conformance checks) → [docs/TEMPLATE_SPEC.md](docs/TEMPLATE_SPEC.md)

## Common Patterns

Quick reference only — the full placeholder/layout catalog lives in [skills/template-deck/TEMPLATE_GUIDE.md](skills/template-deck/TEMPLATE_GUIDE.md).

- Slide types: `title`, `content`, `section`, `two-column`, `blank`, `chart`, `diagram`, `image`, `comparison`
- Placeholder IDs: `title`, `subtitle`, `body`, `body_2` (portable across templates)
- Content types: `text`, `bullets`, `chart`, `diagram`, `table`, `image`, `body_and_bullets`, `body_and_lead`, `bullet_groups`

### Named Patterns (registered in `internal/patterns/`)

| Pattern | Description |
|---------|-------------|
| `agenda` | Numbered section list for agenda / table-of-contents slides: 28pt serif accent numerals, 14pt items (18pt for two to four one-line items), 0.5pt rules, no tiles; `highlight` bolds the current section and dims the rest to 50%; optional `subtitles` set a muted line under each item |
| `agenda-with-images` | Numbered agenda rows (3–6) with title/subtitle and image/quote placeholder per row; the placeholder column is all-or-nothing |
| `arch-stack` | Architecture stack of 3–6 tier bands, each with one block per component (`tiers[].components`, 1–12; 7+ wrap to two rows) or a line of detail, plus optional accent-tinted cross-cutting side rails |
| `before-after` | Two-column before/after with transition chevron: each state a heading over a rule (neutral before, accent after) and open bullets; `overrides.style` `panels` for tiles, `overrides.emphasis` fills one heading |
| `before-after-compact` | Compact before/after for brief lists: content-sized rows, no `bounds` box, for a compose segment or cell; alone on a slide it is composed like any sparse block (`vertical_align: "top"` keeps the band under the title) and reports `SLIDE_UNDERUSED` when the lower third stays empty — use `before-after` there |
| `bmc-canvas` | Formal 9-cell Business Model Canvas (Osterwalder) |
| `capability-heatmap` | Capability / automation heatmap: 3–8 function columns with pointed headers (bold title + optional sublabel) over 1–6 activity cells each, filled by rating tier (2–4 levels, darkest = highest, text ink measured per fill) with a tier legend; shorter columns leave the bottom empty |
| `card-grid` | Grid of 2–12 titled cards; `columns` / `rows` are optional and a last row that is not full stays short (5 = 3 + 2, 7 = 4 + 3) |
| `chart-insights-split` | Left chart panel + right insights column (65/35 split, 75/25 when the insights are sparse or the only text is `so_what`, which is then set at 18pt centred beside the chart) with optional headline number and so-what callout, a series/unit caption and auto data labels; falls back to insights-only when chart is omitted, emitting `CHART_PLACEHOLDER_EMPTY` |
| `comparison-2col` | Two-column comparison: optional headers over open rows separated by hairline rules and aligned across the columns (`overrides.style` `tiles` for filled tiles; one `rows[].highlight` or `overrides.highlight_column`); `overrides.connectors` draws a per-row accent connector badge in a centre gutter ("from → to" shifts) |
| `concentric-rings` | Onion / layers-of-influence model: 3–5 nested rings sharing a base (`layers[]` inner → outer, core innermost), a neutral tint ladder with one solid accent ring (`highlight`, default the core), every ring labelled on a side ladder (bold label + optional `description`) joined by a hairline leader; `cell_accent_mode` `alternate` / `progressive` paint every ring a solid accent; a half-width cell holds labels only |
| `contact-directory` | Key-contacts directory: 1–4 groups (regions / practices), each an accent heading over a rule, then up to 24 people in rows of 3–5 — circular headshot (`photo`) or initials disc + bold name + muted title; sparse directories stack a large headshot above a centred name |
| `cycle-figure-eight` | Figure-eight / infinity loop of 4–8 phases on two coupled lobes side by side (build ↔ run, dev ↔ ops, plan ↔ deliver): two open rings joined by a crossing, the path numbered round the left lobe (counter-clockwise), through the crossing and round the right (clockwise), `left_count` phases on the left (default half), each lobe's labels in a column on its own side, optional `left_label` / `right_label` lobe titles, one optional `phases[].highlight`; needs width — a content area under 580pt wide (a horizontal 50% or 60% compose segment) is refused with `fit_overflow` and a `swap_pattern` fix to `cycle-ring` |
| `cycle-intake` | Linear intake feeding a recurring loop (onboard once, then the service cycle): 1–3 interlocking intake arrows (`intake[]`, neutral, optional description under each) on the ring's centreline, an accent arrow into the ring at 9 o'clock where phase 1 begins, then a loop of 3–8 numbered phases (`loop[]`) with every phase label in one numbered list right of the ring; one optional `loop[].highlight` is the only solid accent, optional `center` label; `overrides.loop_style` `nodes` for circles joined by curved arrows; a narrow area (a compose half) stacks the lane above the loop with a down arrow into 12 o'clock |
| `cycle-nodes` | Recurring loop of 3–8 numbered circles on a ring joined by curved arrows (plan-do-check-act): label + optional description per step outside the ring, each row led by its step number (an optional `steps[].icon` takes the number's place in the circle); one optional `highlight` step is the only solid accent circle, optional `center` label; `overrides.labels` `legend` (one list beside the ring, taken by itself in a compose half) or `inside` (3–5 short labels in the nodes), `direction`, `arrows` `none` |
| `cycle-ring` | Closed ring of 4–8 equal phase segments (lifecycle, PDCA, operating rhythm, flywheel) starting at 12 o'clock: neutral segments with numbered accent badges, the phase labels outside beside their segments (accent numeral + bold label + muted description), one optional `phases[].highlight` solid segment, optional `center` label; `overrides.style` `arrows` for chasing arrows, `direction`, `thickness`; a content area under about 450pt wide (a compose segment) draws a numbered legend beside the ring instead |
| `driver-tree` | Value / cost driver tree: root metric → 2–4 branches → 1–4 leaf items each as label-sized nodes centred on their children and joined by elbow connectors (root the only solid accent node; `overrides.style` `slabs` for the legacy full-height boxes), with optional per-branch annotations (use svggen `org_chart` for people/role hierarchies) |
| `dual-org-ladder` | Two parallel org columns: a bold org heading on a rule above each and 2–4 paired roles, each pair one pale band with a centre arrow, names left-aligned (joint-venture / engagement-team slides); one optional `rows[].highlight` pair, `overrides.style` `lines` for header tiles over centred entries or `tiles` for role cards |
| `exec-summary` | Executive summary of 3–5 bold lead-in statements, each with one supporting sentence, separated by rules, plus an optional bottom-line takeaway band (content-sized rows) |
| `framework-grid` | Framework of 2–6 labelled dimension rows separated by hairline rules, each a bold label followed by 1–4 open cards (accent title + short body); one optional `rows[].highlight` band, `overrides.style` `tiles` for filled tiles; the longest row sets the column count and shorter rows leave trailing space empty |
| `hero-detail` | One dominant metric with 2–4 supporting detail bullets; use `stat-hero` for the metric alone or `kpi-3up` for equally weighted metrics |
| `horizontal-bar-with-callouts` | Ranked horizontal bars (3–8) on the left with a per-bar accent-anchored insight callout on the right; callouts are optional and the column is dropped when none are given |
| `icon-row` | Horizontal row of 3–5 open icons, each over a caption and an optional one-line `description`; `overrides.style` `tile` puts each item in a tile |
| `image-text-split` | One photo / screenshot beside a text column (eyebrow, heading, body, up to 5 bullets) with 0–3 result metrics; real images are cover-cropped, otherwise a dashed placeholder (case study / customer story slides) |
| `journey-maturity-model` | Horizontal maturity ladder of 3–6 stage columns with numbered headers, descriptions, and an optional 'where we are' marker on the current stage |
| `kpi-2up` | Two big-number KPI cards with short captions (kpi-Nup cells take an optional `comparator` line, e.g. "vs plan +4 pts"; an open kpi-Nup row alone on a slide grows its band of dividers to half the content area) |
| `kpi-3up` | Three big-number KPI cards with short captions |
| `kpi-4up` | Four big-number KPI cards with short captions |
| `kpi-5up` | Five big-number KPI cards with short captions (a value holds about 11 digits on the narrowest templates; 12 characters is the hard maximum) |
| `kpi-6up` | Six big-number KPI cards with short captions (a value holds about 9 digits on the narrowest templates) |
| `kpi-inline` | Horizontal inline KPI bar: one content-sized row for supporting context in a compose segment or cell; alone on a slide it reports `SLIDE_UNDERUSED` — use a kpi-Nup row there |
| `labeled-rows` | 2–6 rows of a keyword label block (WHY / WHAT / HOW; `label_style` `tinted` (default) neutral block under an accent rule, `filled` accent block or accent `text`) with optional sublabel beside 1–4 lines of body text, rules between content-sized rows |
| `matrix-2x2` | 2×2 matrix: two crossing axis lines with open quadrants, axis titles and low/high ends along the left and bottom edges, one optional `highlight` quadrant; `overrides.style` `tiles` for filled quadrant tiles with arrow axes |
| `metric-list` | Vertical "by the numbers" stack of 3–7 metrics: big right-aligned accent value + bold label + optional detail line, hairline rules, optional highlighted row (`highlight: true`, at most one) and a bottom takeaway-band callout |
| `next-steps` | Closing next-steps slide: 2–6 numbered action rows (action / owner / date; empty owner or date columns drop) separated by 0.5pt rules, plus an optional "Decisions requested" band (left accent rule, no outline, no fill) — the closer instead of "Thank you" |
| `numbered-step-strip` | Ordered numbered steps (3–7; chevron ≤6) WITHOUT flowchart diamonds, in `chevron` / `stacked-box` / `toc` styles, each with an optional per-step detail zone (stacked-box / toc rows of one-line labels and bodies put the body in a detail column beside the label) and an optional `steps[].icon` |
| `process-flow` | Left-to-right process flow with steps and decision points (3–8 steps); 7–8 steps bend onto two rows of four so a box holds 80 characters at every count (`overrides.rows` `1` keeps one row of narrow boxes; `2` needs at least 4 steps); the returning row runs right to left with its chevrons / arrows mirrored to point left (a flow of chevrons / arrows only wraps left to right); `arrow` steps are block arrows with the label at the flow's type size; draws one path, no yes/no branches; a flow of sentence-length steps alone on a slide grows its boxes (at most to 4:5 cards, 14pt labels) to clear the lower third, a flow of short labels reports `SPARSE_SINGLE_ROW_FLOW` |
| `process-flow-compact` | Compact process flow for short labels: one content-sized band, no `bounds` box; alone on a slide it reports `SLIDE_UNDERUSED` — pair it with a second zone or use `process-flow` |
| `process-grid-2row` | Two parallel process tracks: dk2 row-label column on the left + 3–6 equal-width phase boxes per row (e.g., Design / Production, Strategy / Execution), with optional per-column `column_headers` and `outcomes` pills |
| `pull-quote` | Italic quote block with attribution and an optional headshot column (`values.image`) |
| `phase-roadmap` | Single-track phased roadmap: one band of interlocking phase chevrons (current phase solid accent, the others a light tint) heading equal panels in the lightest neutral that hold the bold date range and description; optional milestone diamonds on the band and 0–4 full-width pointed `parallel_tracks` bars ("In parallel" workstreams) |
| `pyramid` | Stacked trapezoid hierarchy (3-5 tiers) |
| `quote-cluster` | Structured 3-column grid of 3–8 attributed stakeholder quotes (voice-of-customer slides): open quotes under a quote mark by default, `overrides.style` `bubble` (speech bubbles) or `tile`, one optional `quotes[].highlight` |
| `radial-hub` | Hub-and-spoke: one solid accent hub circle (label + optional sublabel) with 4–8 thin spokes to neutral satellites (discs holding `spokes[].icon` when any spoke has one, node dots otherwise), no sequence and no numbers; labels beside their satellites (an odd count labels its 6 o'clock item below it), `overrides.labels` `inside` (4–6 short labels in larger satellites) or `legend` (keyed list A, B, C … beside the ring; the default falls back to it in a narrow area); one optional `highlight` spoke, `overrides.spokes` `none` |
| `risk-heatmap` | Risk heat map: 1–20 named risks (`items[{name, likelihood, impact}]`) placed on a likelihood × impact grid (3 × 3, or `size: 5`), every cell filled by its likelihood × impact band (neutral / tint / solid of the template's negative accent, text ink measured per fill), level labels along both axes and a band legend; risks sharing a cell stack and a crowded grid reports `BODY_TOO_LONG` |
| `roadmap-phased` | Phased roadmap on a shared time axis: workstream rows carry `bars` from a `start` to an `end` period (or `span`), overlaps stack in lanes, `milestone` markers, optional `current_phase`; one-item-per-period `items` render as one-period bars (`overrides.layout` `grid` for the legacy tile table) |
| `scqa-summary` | 4-row SCQA executive summary (Situation / Complication / Questions / Answer) |
| `state-shift-hub` | Central accent hub circle (short label) with 3–4 numbered today/future stage pairs on an arc around it: today items right-aligned on the left, future items left-aligned on the right, optional column headers |
| `stat-hero` | Single oversized statistic with label and optional context |
| `strategy-house` | Strategy-house framework: gabled roof carrying the objective (and optional roof badges) + optional `beam` band + 3-5 pillars + a foundation of 1–3 levels, each a band or a row of 2–5 cells; shares one builder with the native `house_diagram` |
| `stylish-panels` | 3–5 titled columns for pillars, capabilities, or workstreams: heading, accent rule and open bullets; one `values[].highlight` column takes the only solid fill; `overrides.style` `ribbon` for ribbon headers over body tiles |
| `swimlane` | Horizontal swimlane diagram with actors and steps; step tiles take 75% of their lane's height and lanes are set in 14pt when every step fits |
| `table-highlight` | Options × criteria evaluation matrix (2–6 × 2–6) scored with Harvey balls (0–4), RAG dots or short text, with a highlighted recommended row / decisive column and a legend (content-sized rows) |
| `team-bios` | Team / 'Our People' grid of 1–8 members with a headshot (`members[].photo`) or initials placeholder + name + role + short bio (up to 4 per row); emits `BODY_TOO_LONG` when a bio exceeds the ~2-line budget |
| `text-sidebar` | Narrative intro / foreword page: main column (optional heading, 1–4 paragraphs, 0–6 bullets) beside a tinted or accent-filled sidebar panel with one large bold key message (`sidebar_side`, `sidebar_style`, `sidebar_width_pct`) |
| `timeline-horizontal` | Linear horizontal timeline with stops (dots, chevron or `gantt` style: bars drawn to scale on a labelled time axis); a sparse dots timeline renders date, label and body at one size |
| `value-chain` | Horizontal value chain of 4–10 step columns (bold label + per-step description, optional highlight); alone on a slide its arrows and description row grow up to 1.6× so the lower third is not left empty |
| `waterfall-bridge` | Waterfall / bridge bar chart of 3–10 columns showing P&L walks or cost-driver decomposition; floating delta bars with auto-computed subtotals, optional `caption` for the scale |

### Key Top-Level Fields

- `accent_strategy` — controls accent color rotation: `"primary"` (default), `"rotate"`, `"section-keyed"`
- `compose` — slide-level nested pattern composition envelope (multiple patterns on one slide)

### Composition Awareness

- Use `plan_deck` MCP tool as the recommended entry point for new decks (produces a structured slide plan)
- Use `analyze_deck_rhythm` MCP tool to detect visual monotony, accent imbalance, and missing emphasis
- Use `recommend_visual` MCP tool to rank candidate layouts/patterns/charts for a given slide intent


## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` for full workflow context; use `bd ready` / `bd show <id>` / `bd update <id> --claim` / `bd close <id>` for everyday work. Use `bd` for ALL task tracking (not TodoWrite or markdown TODO lists) and `bd remember` for persistent knowledge.

The canonical command reference and the mandatory session-close checklist (including the required `git push` at session end) live in [CONTRIBUTING.md](CONTRIBUTING.md#beads-command-and-session-close-reference) — follow it before ending any session.
