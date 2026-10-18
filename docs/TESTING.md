# Testing tiers

`go test ./...` passing does **not** mean every meaningful path was exercised.
Many SVG, template, render, and headless-PPTX tests skip themselves at runtime
when an external tool or an optional fixture is unavailable. This document
classifies the test suite into tiers, names the gating mechanism for each, and
maps every tier to the CI job that actually runs it (or documents why it can't).

It is the reference for the audit tracked by `go-slide-creator-p9vi`.

## Tiers at a glance

| Tier | Gate | Runs by default? | CI job |
|------|------|------------------|--------|
| Unit | none | yes | `test` (`go test ./... -short`) |
| Tool-gated render | `CheckDependencies()` / `IsAvailable()` runtime skip | only if tool present | `render-integration` |
| Tool-gated SVG conversion | `rsvg-convert` / `inkscape` runtime skip | only if tool present | `render-integration`, `test` (rsvg only) |
| Build-tagged integration | `//go:build integration` | no (tag required) | `corpus-headless` |
| Env-gated opt-in | env var must be set | no | `palette-parity` (`PALETTE_PARITY_TEST=1`) |
| Headless conformance | external binary in job script | n/a (not `go test`) | `pptx-validation`, `corpus-headless` |
| Skill/docs drift | deployed-skill path must exist | no (canonical repo path differs) | n/a — intentional, see below |

## 1. Unit tier (default)

Everything not gated below. Runs on every push/PR via the `test` job with
`-short -race` and a 70% coverage threshold.

The standard-template fixture in `internal/generator/*_test.go` is mandatory.
Other image guards refer to the fixtures
they look for (`internal/template/testdata/standard.pptx`,
`internal/generator/testdata/test_image_*.png`, `test_image.svg`) are committed,
so the guards do not fire in a normal checkout and these tests run as unit
tests. Required standard-template guards now fail a corrupted/partial checkout via
`testutil.StandardTemplatePath(t)`; optional local fixtures remain explicit skips.

## 2. Tool-gated render tier

`internal/render` drives LibreOffice + ImageMagick. Some LibreOffice builds
(e.g. 24.2 on Linux) do not load fonts embedded in the deck, so templates that
ship their own faces (`modern-template`: Lora titles, Poppins body) render in a
fallback sans such as DejaVu Sans while fit measurement uses the real metrics.
Check `pdffonts` on the intermediate PDF before judging typeface or line breaks,
and install `svggen/fonts/*.ttf` system-wide (fontconfig) for faithful renders.
The integration tests skip when those binaries are absent:

- `TestIntegrationRenderSlide`, `TestIntegrationRenderDeck`
  (`internal/render/render_test.go`) — gated by `CheckDependencies()` (needs
  `libreoffice` and `magick` on `PATH`) **and** an input deck.

The input deck is resolved by `integrationPPTXPath()`: it honours the
`RENDER_TEST_PPTX` environment variable and otherwise falls back to
`/tmp/render-test/basic-deck.pptx`. This lets the tests be pointed at any
generated deck instead of depending on one hard-coded path. The 7-slide
truncation assertion only applies to the default `basic-deck` fixture.

**CI:** the `render-integration` job installs the full toolchain, generates
`examples/basic-deck.json` to the expected path, sets `RENDER_TEST_PPTX`, and
runs `go test ./internal/render/... -run Integration`.

### Render truth: reading back what LibreOffice re-fits

`cmd/json2pptx` `TestRenderTruthExemplars` generates the exemplar deck of every
named pattern, converts it pptx → pptx through LibreOffice and reads the
`fontScale` / `lnSpcReduction` LibreOffice wrote onto each `normAutofit` shape.
A shape that comes back with a `fontScale` is a box the engine sized too small
for its text at the size it wrote: the test fails and names the slide, pattern
and text. A shape with only `lnSpcReduction` (line spacing tightened, font
kept) is logged. It is the one fit check that does not share the engine's own
text metrics.

It is skipped unless `RENDER_TRUTH_TEMPLATES` names the templates to sweep
(space or comma separated), because it needs LibreOffice and the templates'
faces — a face the host substitutes with a wider one reports shrinks that are
the host's:

```bash
RENDER_TRUTH_TEMPLATES="midnight-blue forest-green" \
  go test ./cmd/json2pptx -run TestRenderTruthExemplars -count=1 -v
```

`RENDER_TRUTH_KEEP=<dir>` keeps the generated and round-tripped decks.

**CI:** the `render-integration` job installs `fonts-crosextra-carlito`
(LibreOffice draws Calibri in it), checks that Calibri resolves to it, and runs
the sweep on `midnight-blue` and `forest-green`.

## 3. Tool-gated SVG-conversion tier

`internal/generator/svg_test.go` converts SVG → PNG/EMF via `rsvg-convert`
(and a few `inkscape`-only cases). Each test skips with
`t.Skip("rsvg-convert not available")` / `"inkscape not available"` when the
tool is missing. Examples: `TestSVGConverter_ConvertToPNG`,
`TestSVGConverter_ConvertToEMF`, `TestSVGConverter_Convert_EMF`.

**CI:** the `test` job installs `librsvg2-bin`, so the `rsvg-convert` cases run
there. The `render-integration` job additionally runs the
`SVGConverter`-matched tests under the full toolchain. The `inkscape`-only
cases remain skip-by-default in CI (Inkscape is not installed on the runners);
this is an accepted gap — they are smoke checks for an alternate converter, not
a primary path.

## 4. Build-tagged integration tier

Tests behind `//go:build integration` do not compile into the default build and
must be requested with `-tags=integration`:

- `cmd/json2pptx/corpus_headless_test.go` — generates every `examples/*.json`
  against every bundled template and round-trips each through headless
  LibreOffice, asserting no repair/corruption warnings.

- `cmd/json2pptx/full_matrix_corpus_test.go` — `TestShortReducedMatricesCorpus`
  runs, in full, every test of the package that does less under `-short` (a
  part of its matrix, or a skip; see "The sharded race step" below), and
  `TestShortSensitiveTestsRunInCorpus` reads the test sources and fails when a
  test that reads `testing.Short()` is run without `-short` by no CI job.

**CI:** the `corpus-headless` job runs
`go test -tags=integration ... -run 'Corpus|AcrossTemplates'` (as two parallel legs: the tests named `Corpus`, and the `AcrossTemplates` tests not named `Corpus`), without `-short`
and without `-race`.

### The sharded race step and its time budget

`cmd/json2pptx` is the heaviest package under `-race`: the `test` job runs it
through `scripts/ci_test_cmd_shards.sh 4`, which builds the test binary once
and runs it as four shards. The step is killed at 50 minutes, and the script
fails it when a shard takes more than 35 (`SHARD_BUDGET_SECONDS`, 15 minutes
under the step limit; the slowest shard took 28 on run 37235543451; enforced in
CI by `SHARD_BUDGET_ENFORCE=1`; locally it only warns), printing each shard's
time and the slowest tests.

- **Balance.** Shards are filled longest test first from
  `scripts/ci_test_durations.txt`. Refresh it from a CI job log:
  `gh run view <run> --log --job <job> | scripts/ci_test_durations.sh > scripts/ci_test_durations.txt`.
  A test the file does not list gets a default weight, so a new test needs no
  edit.
- **Affinity.** Tests that share a once-per-binary fixture are listed as one
  group in `scripts/ci_test_affinity.txt` and land in one shard, so the
  fixture is computed once (the agent-journey group: the short parity corpus,
  the short patch harness, the twelve-flaw journey, the over-full summary's
  answer on modern; the two `semantic kinds <kind>` tests; the two exemplar
  deck tests). Before a test asks the tools for an answer another test of the
  package already asks for — the same spec, template and arguments — give
  both one fixture and one group.
- **Short matrices.** A test whose cost is a matrix takes two templates (or a
  few decks) under `-short` and the whole matrix otherwise. The whole matrix
  must still run in CI: name the test `...AcrossTemplates` or `...Corpus`, or
  add it to `shortSensitiveTests` in
  `cmd/json2pptx/full_matrix_corpus_test.go`; the `corpus-headless` job runs
  those without `-short`. The same holds for a test that skips under
  `-short`: the `test` job is `-short` only, so its long form runs nowhere
  else (a guard test in that file enforces the list). In the other packages
  the `render-integration` job runs `go test ./internal/...` and the svggen
  module without `-short`.

New genuinely-integration tests (require external tools or are slow/fixture
heavy) should adopt this `//go:build integration` tag and be wired into a CI job
with the required tools, rather than relying on a runtime `t.Skip`.

## 5. Env-gated opt-in tier

Cross-engine palette parity is expensive and tool-heavy, so it is opt-in:

- `tests/palette_parity/...` and `TestRunAuditPalette_EndToEnd` skip unless
  `PALETTE_PARITY_TEST` is set.

**CI:** the `palette-parity` job installs LibreOffice, poppler and ImageMagick (the audit rasterises through `internal/render`, like every other render path) and runs with
`PALETTE_PARITY_TEST=1`.

## 6. Headless conformance (script-driven, not `go test` skips)

`pptx-validation` and `corpus-headless` jobs build the CLIs and round-trip
generated decks through LibreOffice directly in shell. These are not gated by
Go skips; they live entirely in CI job scripts.

## 7. Intentional, non-CI skips

These skip in a normal checkout **by design** and should not be "fixed" by
forcing them to run:

- `skill_drift_test.go` / `skill_sync_doctor_test.go` look for
  `.claude/skills/generate-deck/SKILL.md`. The canonical skill lives at
  `skills/generate-deck/SKILL.md`; `.claude/skills/` is a deployment artifact
  that is intentionally **not** tracked (enforced by
  `scripts/check-skill-canonical.sh`). The drift check therefore only runs in an
  environment where the skill has been deployed to `.claude/`.
- `section_font_debug_test.go` references a test-only template
  (`testdata/templates/template_2.pptx`) that is not committed; it is a manual
  debug aid, not a coverage gate.

## Adding a test — which tier?

1. **Pure logic, fixtures embeddable?** Unit tier. Prefer small committed or
   in-test fixtures over a runtime skip so the test always runs.
2. **Needs LibreOffice/ImageMagick/Inkscape, or is slow?** Build-tagged
   integration (`//go:build integration`) **and** add it to a CI job that
   installs the tool. Don't leave it as a silent runtime `t.Skip` with no CI
   job exercising it.
3. **Expensive opt-in (parity/visual)?** Env-gate it and add a dedicated CI job.

## Reproducing CI and retaining local output-quality coverage

Set `JSON2PPTX_SKIP_LOCAL_TEMPLATES=1 make test` to exclude the ignored p-style
fixture from shared matrices and explicit optional-template branches. Leave it
unset for local output-quality reviews: those reviews must include p-style.
`make test-local-templates` requires p-style to exist, then runs the supplemental
cross-template and designer-template suites. It never commits that template.

The optional p-style cases have no CI fixture: `TestDerivedAccentUsageGuideForTemplateWithoutMetadata`,
`TestDeterministicCorpus/p-style`, `TestMeasuredMatricesSurviveSemanticPipeline/p-style`,
`TestExecutiveSummaryReclaimsTallTitleBandOnPStyle`,
`TestStakeholderBoundaryMarkersClearWrittenAxisBands/p-style`,
`TestStakeholderPaddingRetainsExhibitCoverage/p-style`,
`TestNativeDisplayAlignmentInGeneratedPPTX/p-style`, and
`TestGenerateDropsFooterSegmentInsteadOfEllipsis/p-style`. Their shipped-template
siblings still execute in CI. Corrupt shipped templates fail; only the named
optional fixture may be absent.

`make determinism-linux` cross-compiles the test binary and runs the canonical
Linux byte-golden check in the existing `lo24-rt` Docker image. Set
`DETERMINISM_IMAGE` to another local render-toolchain image if needed. Docker and
that image must be installed; the target does not silently fall back to macOS.
The source mount is read-only, so this check cannot rebaseline goldens.

## Calibration probes and environment switches

Calibration functions are behind `//go:build probe`; default `go test` and shard
inventory exclude them. Their helpers and genuine assertion tests stay in the
ordinary suite. Opt in explicitly, for example:

```bash
JSON2PPTX_CYCLE_NODES_BUDGET_PROBE=1 go test -tags=probe ./cmd/json2pptx -run '^TestCycleNodesBudgetProbe$' -count=1 -v
CYCLE_INTAKE_PROBE=1 go test -tags=probe ./internal/patterns -run '^TestCycleIntakeBudgetProbe$' -count=1 -v
(cd svggen && FONT_INSPECT=1 go test -tags=probe -run '^TestFontSizeInspection$' -count=1 -v)
```

No CI job sets the calibration gates. The `*_BUDGET_PROBE` family includes
`JSON2PPTX_` prefixes for HORIZONTAL_BAR, AGENDA_IMAGES, BEFORE_AFTER_COMPACT,
PHASE_ROADMAP, TIMELINE, STYLISH_PANELS, HERO_DETAIL, TABLE_HIGHLIGHT,
QUOTE_CLUSTER, DUAL_ORG, EXEC_SUMMARY, WATERFALL, STAT_HERO, SCQA, AGENDA,
VALUE_CHAIN, STATE_SHIFT_HUB, STRATEGY_HOUSE, JOURNEY, PROCESS_COMPACT,
PROCESS_FLOW, ARCH_STACK, PYRAMID, CHART_INSIGHTS, MATRIX, DRIVER, COMPARISON,
ROADMAP, TEAM_BIOS, SWIMLANE, NUMBERED, KPI_INLINE, CYCLE_RING,
CYCLE_FIGURE_EIGHT, CYCLE_NODES, CYCLE_INTAKE, RADIAL_HUB and CONCENTRIC_RINGS.
Other calibration gates are `JSON2PPTX_BUDGET_PROBE`,
`JSON2PPTX_DRIVER_SPAN_PROBE`, `CYCLE_INTAKE_PROBE`,
`JSON2PPTX_CYCLE_FIGURE_EIGHT_RENDER`, `J2P_FIT_DECK` (path), `J2P_FIT_GEOM`,
`DEBUG_RENDER`, and `FONT_INSPECT`. Use the corresponding probe file's test name
with `-run`; set the gate to `1` except path-valued switches.

| Switch | Purpose | CI job |
| --- | --- | --- |
| `PALETTE_PARITY_TEST` | Cross-engine palette assertions | palette-parity |
| `RENDER_TEST_PPTX` | Input deck for render integration | render-integration |
| `RENDER_TRUTH_TEMPLATES` | Template list for LibreOffice refit audit | render-integration |
| `RENDER_TRUTH_PATTERNS`, `RENDER_TRUTH_KEEP` | Limit patterns, retain audit artifacts | none |
| `JSON2PPTX_BINARY` | Prebuilt CLI for portability integration | template-portability-rendered |
| `PORTABILITY_RENDER_*` | Rendered portability inventory/artifacts | template-portability-rendered |
| `NATIVE_LAYOUT_CORPUS_OUT` | Retained all-native-layout review output | none |
| `NATIVE_LAYOUT_TEMPLATE_DIR`, `NATIVE_LAYOUT_IMAGE_SOURCE`, `NATIVE_LAYOUT_STRICT_FIT`, `NATIVE_LAYOUT_PHOTO_BACKDROP`, `NATIVE_LAYOUT_TABLE_CONTINUATIONS`, `NATIVE_LAYOUT_BULLET_CONTINUATIONS` | Native layout review inputs and mode | none |
| `DISCLOSURE_ALIGNMENT_ARTIFACT_DIR`, `FOOTER_PROMPT_CANDIDATE_DIR` | Retain generator review artifacts | none |
| `PREVIEW_PIXEL_FULL` | Expand preview pixel comparison matrix | none |
| `SWEEP_KEEP`, `PEER_KEEP` | Retain typography sweep artifacts | none |
| `UPDATE_GOLDEN` | Explicit golden regeneration | none |
| `JOURNEY_RECORD`, `JOURNEY_COMMIT`, `JOURNEY_NOTE` | Record journey metrics and provenance | none |
| `JSON2PPTX_SKIP_LOCAL_TEMPLATES` | Reproduce shipped-only template inventory | optional local use |
| `SHARD_BUDGET_*`, `SHARD_TIMEOUT`, `CI_TEST_*`, `SHARD_PLAN_ONLY`, `SHARD_TESTS` | Shard planning, duration calibration and runtime budget | test; plan/filter overrides are local |

`GET_STARTED_RENDER_INTEGRATION` is retired: the non-short corpus always runs
those render steps. `testutil.RequireRenderTools` skips missing optional tools
locally and fails with their names when `CI` is set. Corpus members that skip at
the top level in CI also fail, so membership alone no longer implies execution.
The corpus-headless job installs LibreOffice, ImageMagick and the Calibri-compatible
Carlito face. Child-process plumbing (`JSON2PPTX_HEADLESS_TEST_HELPER`,
`J2P_CLI_DISPATCH_HELPER`, `JSON2PPTX_GUARD_*`, `RENDER_CONVERT_*`) is internal test
protocol, not an opt-in coverage tier.

## Shared fixture timing and duration refresh

Exemplar findings use one `sync.Once` per template. Selected exemplar tests warm
their shared template/font/fit state in `TestMain` before individual test timers;
`-list` and unrelated focused selections do not pay that cost. Setup time remains
part of shard wall time and its enforced budget. The short exemplar area test
uses business-template; the full corpus retains midnight-blue as well. Compose
examples still receive fit validation on every template under `-short`, while
PPTX writing uses modern-template; corpus-headless writes the full matrix.

`scripts/ci_test_durations.sh` records the measured top-level inventory count
alongside its deliberately sparse (at least one second) table. The shard runner
reports the default-weight inventory and warns on growth, failing above
`CI_TEST_MAX_UNMEASURED` (100 by default). Legacy tables without the count emit a
refresh warning. The scheduled **Refresh test durations** workflow selects the
latest successful main CI run with a completed successful CLI shard step every
Monday (or on manual dispatch). It skips metadata-only runs, searches all attempts
after partial reruns, and downloads the newest successful CLI job's log. It regenerates
the table and uploads `calibrated-test-durations` for review and commit. It does
not substitute laptop timings or commit thousands of default-weight entries.
After fixture changes, review that artifact before updating the tracked table.

The semantic example corpus now runs the same scratch-render evaluation as
`semantic validate`, preserving each pinned template (midnight-blue for an
unpinned example). Input-only checks in `internal/semantic` remain useful but
are not the render-stage quality gate.

The live MCP output-contract guard covers the registered tool inventory:
`TestMCPDiscoveryLiveOutputSchemas` and `TestMCPWorkflowLiveOutputSchemas` call
real JSON-RPC handlers and validate successful structured responses against their
registered output schemas. `TestMCPLiveOutputSchemaFixturesCoverRegistry` fails
when a tool has no executable fixture. The five external-render workflows run in
`TestMCPWorkflowRenderLiveOutputSchemas`, also registered in corpus-headless.
