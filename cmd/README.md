# Command Index

This directory holds every executable in the repository. `json2pptx` is the
primary tool; the rest are companion utilities for template authoring, image
conversion, validation, and testing.

Each tool prints its own flags with `-h` (or `help` for `testkit`). Treat that
output as authoritative — the examples below are starting points, not a full
reference.

| Tool | Audience | One-liner |
|------|----------|-----------|
| [`json2pptx`](#json2pptx) | Everyone | Generate decks; run the HTTP API and MCP server |
| [`pptx2jpg`](#pptx2jpg) | Contributors, visual QA | Render a `.pptx` to per-slide images |
| [`mktemplate`](#mktemplate) | Template authors | Generate the bundled `.pptx` templates from theme defs |
| [`templatecaps`](#templatecaps) | Template authors | Inspect a template's layouts, placeholders, and capabilities |
| [`debugcolors`](#debugcolors) | Template authors | Dump the theme/accent palette resolved from a template |
| [`validatepptx`](#validatepptx) | Contributors, CI | Validate PPTX structure and embedded media standalone |
| [`testrand`](#testrand-alias-testkit) (alias `testkit`) | Contributors, CI | Random/stress deck generation, validation, and visual QA |

## json2pptx

The main CLI: a semantic deck compiler, batch JSON→PPTX converter, HTTP API
host (`serve`), and MCP server (`mcp`). This is the only tool most users need.

For agent-authored decks, start with semantic specs:

```bash
json2pptx semantic validate --spec examples/semantic/qbr.yaml
json2pptx semantic render --spec examples/semantic/qbr.yaml --output /tmp/qbr.pptx
```

Use raw JSON when debugging compiled output or exercising low-level features:

```bash
json2pptx generate -json examples/basic-deck.json -template midnight-blue \
  -templates-dir templates -output /tmp/out
```

Full subcommand and flag reference lives in the top-level
[README](../README.md#cli-reference). Run `json2pptx -h` for the subcommand
list and `json2pptx <subcommand> -h` for per-command flags.

## pptx2jpg

Convert a generated `.pptx` into one image per slide for visual inspection.
Used throughout visual QA. Requires **LibreOffice** and **ImageMagick** on
`PATH`.

```bash
pptx2jpg -input /tmp/out/basic-deck.pptx -output /tmp/slides/ -density 150
```

Run `pptx2jpg -h` for full flags (`-input`, `-output`, `-density`).

## mktemplate

Generate the bundled `.pptx` templates from the theme definitions baked into
the tool. Run this after editing a template's colors, fonts, or layouts to
regenerate the file checked into `templates/`.

```bash
# Regenerate a single template
go run ./cmd/mktemplate -name midnight-blue -out templates/midnight-blue.pptx

# Regenerate every bundled template
go run ./cmd/mktemplate -all -outdir templates/
```

Run `mktemplate -h` for full flags (`-name`, `-all`, `-out`, `-outdir`).

## templatecaps

Inspect what a template can do: its layouts, placeholders, theme, and the
synthesized capabilities (charts, images, two-column) the generator relies on.
Use it when authoring a template or debugging why a layout isn't matched.

```bash
templatecaps templates/midnight-blue.pptx
```

Run `templatecaps -h` for options (`-h` shows the usage banner; pass a
`<template.pptx>` argument).

## debugcolors

Display and debug the color palette extracted from a template — theme colors,
accent colors, and the computed palette the engine resolves semantic color
names against. Useful when a deck's colors look wrong and you need to see what
the template actually exposes.

```bash
# Dump every bundled template's palette
debugcolors

# Just one template, as JSON
debugcolors -template midnight-blue -json
```

Run `debugcolors -h` for full flags (`-template`, `-json`).

## validatepptx

Standalone structural and OOXML validator for a `.pptx` file. Reports slide
count and SVG/PNG/chart-embed statistics, and exits non-zero on failure — handy
in CI or after a manual edit to a generated file.

```bash
validatepptx -min-slides 5 -require-svg /tmp/out/basic-deck.pptx
```

Run `validatepptx -h` for full flags (`-json`, `-require-svg`, `-min-slides`).
Exit codes: `0` passed, `1` validation failed, `2` error (bad file, etc.).

## testrand (alias testkit)

The test toolbox. The same binary installs under two interchangeable names:
`testkit` (preferred) and `testrand` (compatibility alias). It bundles
randomized deck generators with systematic test utilities, all seed-based so
every failure is reproducible (each run prints its seed).

Subcommands: `generate` (random JSON deck), `visual` (systematic visual stress
deck), `validate` (validate a generated PPTX), `svg-stress` (exercise every
diagram type), and `qa` (AI-powered visual QA on slide images; needs
`ANTHROPIC_API_KEY`).

```bash
# Generate a random deck from a fixed seed (reproducible)
testkit generate --seed=42 --output=/tmp/random-deck.json

# Validate a generated PPTX has the expected slide count
testkit validate --pptx=/tmp/out/deck.pptx --expected-slides=10
```

Run `testkit help` for the full command and flag reference.
