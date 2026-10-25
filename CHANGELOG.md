# Changelog

All notable changes to json2pptx are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file tracks releases of the binaries, container image and install
archives. The input contract (JSON / DeckSpec schema, fit-finding codes, MCP
response shapes) is versioned separately, entry by entry, in
[docs/SCHEMA_CHANGELOG.md](docs/SCHEMA_CHANGELOG.md); each release section
below names the schema version it ships.

How to cut a release (tag, archives, container image) is described in
[CONTRIBUTING.md](CONTRIBUTING.md#releasing).

## [Unreleased]

- Schema 5.1.0: content-aware layout capacity and card arrangements, motif-aware recommendations, compatible planning alternatives, paired quotes, focal image crops and semantic chart number formatting. Explicit paragraph alignment survives finishing; grid links avoid occupied cells; log bar annotations retain their context. Rhythm density and visual QA use measured evidence and the canonical typography policy. The authoring docs and wiki describe these controls.

## [1.2.0] - 2026-10-18

Ships input/MCP schema **5.0.0** (authoring fingerprint `bb7ad471bb831a5c`).
Validation responses now expose a top-level `findings[]`; the former nested
object remains under `legacy_findings` for migration. Raw render responses add
the same canonical field names as DeckSpec rendering. See
[the schema migration](docs/SCHEMA_CHANGELOG.md) before updating clients.

- Consulting layout/review additions (schema 4.178.0): native splits and
  headings, richer regions/compose, consulting visual kinds, three chart
  types, stakeholder placement, chapter/sticker chrome and measured layout
  advisories. See the wiki consulting catalog for contracts and bounds.


### Added

- Deployable llm-wiki companion for capability exploration: typed documentation,
  real search and MCP tools, portable installation, and an authenticated HTTP
  companion configuration. See `docs/wiki/wiki-engine-and-deployment.md`.

- Consulting heatmaps retain full 12pt axis labels and readable numeric captions;
  stakeholder maps retain 12pt labels and refuse collisions. Measured framework
  placements refuse content loss, and spectrum recommendations include supporting
  argument text. Discovery, preview assets and authored finding paths stay aligned.

- Field-review improvements (schema 4.177.0): weighted table columns, process
  regions, planned cycle phases, longer measured KPI2/3 values, funnel stage
  highlighting and neutral hero-detail headings. Grouped-bar labels avoid
  collisions, RAG legends omit unused colours, executive summaries start below
  measured title text, and shared-autofit diagnostics name the overflowing peers.

- MCP pattern placement metadata exposes hard dimensions, narrow-layout
  fallbacks and supporting-band guidance before rendering. The default tool
  profile now includes deck archetypes and storyline explanation (schema 4.176.0).

- **Agent wiki** (`docs/wiki/`): four new pages — the layout catalog (every
  kind with its forms, patterns by family with their DeckSpec reach,
  charts, diagrams, and how to preview each), the slide-estate and
  segments guide (`regions` arrangements and kinds, raw
  `compose`, content-sized versus filling blocks, and the width at which each
  visual still draws), nine deck archetypes mapped slide by slide onto kinds
  with ghost decks, and the slide design rules a partner-level reviewer
  applies (Pyramid Principle tests, dot-dash storyboard, title – exhibit –
  takeaway, chart choice by comparison, the twelve failure modes and where the
  product catches them). Each playbook gains a domain-conventions section
  (five-Cs findings and rating ladders; heat-map scales and remediation
  storylines; red-flag reports and IC papers; the Five Case Model, TCO and
  benefits registers).

### Changed

- **Security baseline:** require Go 1.26.9 for builds and container images,
  update `golang.org/x/net` to v0.60.0 and its required dependencies, and pin
  golangci-lint v2.12.2 for Go 1.26 compatibility. These updates address the
  October 2026 Go HTTP/2, HTTP, TLS, and textproto vulnerabilities.

- **Agent wiki** now covers all 50 kinds and 77 patterns, with `cycle` as a
  region kind, one-line `detail`s fit at five options, whole-slide scaling
  under 70%, `exec-summary` reachable from `executive_summary`, the complete
  raw-only pattern list, multi-template `validate_deck_spec`, and the hidden
  tools worth calling by name (`explain_deck_spec`, `list_deck_archetypes`,
  `analyze_deck_rhythm`, `export_deck`). `SKILL.md` points at the wiki from a
  repository checkout.

### Fixed

- Shared CLI/MCP/HTTP generation policies, template analysis and verified table
  repair; template-aware output identities preserve second-template artifacts.
- Planner fact retention, authored diagnostic paths, actual schema coverage,
  media integrity, semantic colours and private render-cache boundaries.
- CI now executes the required render corpus, isolates probes from normal tests,
  calibrates race shards from measured Linux runs and checks deterministic output.

- **Full-slide images**: SVG image cells now use the whole rectangular cell,
  matching PNG images, and honor `image.fit` cover/contain without distortion.
  Native SVG pictures include a visible PNG fallback. Use grid bounds
  `{x: 0, y: 0, width: 100, height: 100}` for edge-to-edge images; explicit
  cell-level `fit` still selects a square frame.

- **`plan_deck`** (`format: "deckspec"`): facts routed to the items of an
  enumerated outline are returned in `slots[].facts` instead of being dropped
  with an empty `unplaced_facts`; a data list after a label about one slide
  ("A chart slide shows the market: 2021 …, 2025 …") stays on that slide
  instead of becoming the deck's outline; "… on the left and … on the right"
  drafts one `regions` slide with the chart data and the number filled in; and
  each `slots[].regions` describes its own slide.
- **Sources**: a `comparison` of figures (fee against fee, weeks against
  weeks) inherits `meta.source` and reports `DATA_WITHOUT_SOURCE` when the
  deck has none, in the two-column and the multi-column rendering.
- **`restore`**: restoring a deck revision restores the template it was bound
  to, including one named by a `template` argument or a `template_path`.

## [1.0.0] - 2026-09-29

First tagged release. Everything below was developed since the initial
commit on 2026-04-02. Ships input schema
4.154.0 (see [docs/SCHEMA_CHANGELOG.md](docs/SCHEMA_CHANGELOG.md)).

### Added

- **Generation engine**: `json2pptx generate` renders a PowerPoint deck from
  structured JSON against a `.pptx` template. All visual identity (theme
  colours, fonts, layouts) comes from the template; the input uses semantic
  colour names (`accent1`, `dk1`, …) resolved through the template theme.
- **Templates**: nine embedded templates (`abstract`, `blue-corporate`,
  `business-template`, `forest-green`, `midnight-blue`, `modern`,
  `modern-template`, `modern-yellow`, `warm-coral`) with per-layout preview
  thumbnails; bring-your-own `.pptx` templates, checked by
  `validate-template` / `template-check` against
  [docs/TEMPLATE_SPEC.md](docs/TEMPLATE_SPEC.md).
- **Content types**: text, bullets (including ordered lists), tables with a
  consulting default style, native SVG charts and diagrams from the `svggen`
  module with a PNG fallback, images with cover / contain fitting, speaker
  notes, slide chrome (trackers, section crumbs, source zones).
- **Shape grids and named patterns**: `shape_grid` layouts and 50+ named
  patterns (KPI cards, roadmaps, process flows, matrices, exec summaries,
  waterfall bridges, team bios, …) that expand at generation time, plus a
  `compose` envelope for several patterns on one slide. See
  [docs/PATTERNS.md](docs/PATTERNS.md).
- **Semantic authoring (DeckSpec)**: compact YAML / JSON describing slide
  intent and content, compiled to patterns and layouts with per-kind closed
  schemas and unknown-field diagnostics.
- **Fit and quality diagnostics**: `validate -fit-report` findings for
  overflow, density, contrast, readability (12pt floor) and composition, with
  machine-actionable fixes; see [docs/FIT_FINDINGS.md](docs/FIT_FINDINGS.md).
  Contrast on layout backgrounds is enforced to WCAG AA.
- **MCP server** (`json2pptx mcp`): generate, validate, plan_deck,
  recommend_visual, analyze_deck_rhythm, repair_slide, pattern and template
  discovery, rendered-slide images and a visual-review loop for agents.
- **HTTP API** (`json2pptx serve`) and a Docker image published to GHCR.
- **Exports**: PDF and speaker-notes handouts; `pptx2jpg` for slide images.
- **Agent skills** (`skills/`) installed alongside the binary by
  `make install`, `install.sh`, `install.ps1` and the dist archives.
- **Deck-level style defaults** for tables and cells; see
  [docs/STYLE_DEFAULTS.md](docs/STYLE_DEFAULTS.md).

[Unreleased]: https://github.com/sebahrens/json2pptx/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/sebahrens/json2pptx/compare/v1.0.0...v1.2.0
[1.0.0]: https://github.com/sebahrens/json2pptx/releases/tag/v1.0.0
