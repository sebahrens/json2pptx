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

### Added

- **Agent wiki** (`docs/wiki/`): four new pages — the layout catalog (every
  kind with its forms, all 58 patterns by family with their DeckSpec reach,
  16 charts, 21 diagrams, and how to preview each), the slide-estate and
  segments guide (six `regions` arrangements, eight region kinds, raw
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

- **Agent wiki** synced to schema 4.175: 29 kinds, `cycle` as the eighth
  region kind, one-line `detail`s fit at five options, whole-slide scaling
  under 70%, `exec-summary` reachable from `executive_summary`, the complete
  raw-only pattern list, multi-template `validate_deck_spec`, and the hidden
  tools worth calling by name (`explain_deck_spec`, `list_deck_archetypes`,
  `analyze_deck_rhythm`, `export_deck`). `SKILL.md` points at the wiki from a
  repository checkout.

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

[Unreleased]: https://github.com/sebahrens/json2pptx/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/sebahrens/json2pptx/releases/tag/v1.0.0
