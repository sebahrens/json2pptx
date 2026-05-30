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

Most `t.Skip("test template not found")` / `t.Skip("test image not found")`
guards in `internal/generator/*_test.go` are **defensive only** — the fixtures
they look for (`internal/template/testdata/standard.pptx`,
`internal/generator/testdata/test_image_*.png`, `test_image.svg`) are committed,
so the guards do not fire in a normal checkout and these tests run as unit
tests. The guards stay as cheap protection against a corrupted/partial checkout;
they are not an integration boundary.

## 2. Tool-gated render tier

`internal/render` drives LibreOffice + ImageMagick. The integration tests skip
when those binaries are absent:

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

**CI:** the `corpus-headless` job runs `go test -tags=integration ... -run Corpus`.

New genuinely-integration tests (require external tools or are slow/fixture
heavy) should adopt this `//go:build integration` tag and be wired into a CI job
with the required tools, rather than relying on a runtime `t.Skip`.

## 5. Env-gated opt-in tier

Cross-engine palette parity is expensive and tool-heavy, so it is opt-in:

- `tests/palette_parity/...` and `TestRunAuditPalette_EndToEnd` skip unless
  `PALETTE_PARITY_TEST` is set.

**CI:** the `palette-parity` job installs LibreOffice + poppler and runs with
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
