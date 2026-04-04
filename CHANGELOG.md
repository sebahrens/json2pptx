# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- CI pipeline with GitHub Actions (test, lint, security)
- Test coverage threshold enforcement (70% minimum)
- govulncheck security scanning in CI
- golangci-lint configuration with complexity checks
- Docker containerization support
- MIT LICENSE file
- CONTRIBUTING.md guide
- Package-level documentation (doc.go files)

### Changed

- Unified ChartSpec and InfographicSpec into DiagramSpec
- Migrated to structured slog logging
- Refactored layout classification with lookup tables
- Applied Strategy Pattern to semantic matching

### Fixed

- Panic recovery middleware for HTTP server
- SVG scale validation upper bounds
- Error message sanitization in upload handler

### Security

- YAML bomb protection in SVG API decoder
- Error message sanitization to prevent path disclosure

## [0.1.0] - 2026-01-18

### Added

- Initial release of Go Slide Creator
- Markdown to PPTX conversion
- Support for multiple chart types (bar, line, pie, radar)
- SVG diagram generation (matrix, Porter's Five Forces, timeline)
- Template analysis and layout selection
- LLM-assisted content refinement
- Visual inspection for quality assurance
- HTTP API for slide generation
- Inline chart rendering during single-pass generation
- Template caching with LRU eviction

[Unreleased]: https://github.com/sebahrens/json2pptx/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sebahrens/json2pptx/releases/tag/v0.1.0
