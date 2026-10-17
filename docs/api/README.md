# Go Slide Creator API Guide

A REST API for converting JSON slide definitions into professional PowerPoint presentations.

## Table of Contents

- [Overview](#overview)
- [Base URL](#base-url)
- [Error Handling](#error-handling)
- [Endpoints](#endpoints)
  - [Health Check](#health-check)
  - [Capabilities](#capabilities)
  - [List Templates](#list-templates)
  - [Get Template Details](#get-template-details)
  - [Convert Slides](#convert-slides)
  - [Semantic Deck Specs](#semantic-deck-specs)
  - [Download File](#download-file)
  - [List Slide Types](#list-slide-types)
- [Patterns](#patterns)
  - [List Patterns](#list-patterns)
  - [Get Pattern Details](#get-pattern-details)
  - [Validate Pattern Data](#validate-pattern-data)
  - [Expand Pattern](#expand-pattern)
- [Common Use Cases](#common-use-cases)
- [SDK Examples](#sdk-examples)
- [MCP Tool Reference Pages](#mcp-tool-reference-pages)

---

## Overview

The Go Slide Creator HTTP API enables programmatic generation of PowerPoint presentations from structured JSON and semantic deck specifications. The service:

- Accepts JSON input with **text and bullet content**
- Accepts semantic deck specs for content-bearing business decks (`kpi_snapshot`, `chart_insight`, `decision`, etc.)
- Analyzes PPTX templates to discover available layouts
- Automatically selects optimal layouts based on slide type
- Generates production-ready PPTX files
- Provides read-only access to the named pattern library

> **Feature boundary:** `POST /api/v1/convert` remains the basic text/bullet
> endpoint. For normal agent-authored decks, prefer `/api/v1/semantic/*` or the
> semantic MCP/CLI surfaces. For raw charts, diagrams, tables, images, shape
> grids, named patterns, and deck-structure features that are not expressible in
> the semantic schema, use the **MCP interface** (`json2pptx mcp`) or the **CLI**
> (`json2pptx generate`). See `GET /api/v1/capabilities` for the machine-readable boundary.

## Base URL

| Environment | URL |
|-------------|-----|
| Local Development | `http://localhost:8080` |
| Docker | `http://localhost:8080` |

### Access and browser clients

The binary binds to `127.0.0.1` by default. Set `--bind 0.0.0.0`,
`JSON2PPTX_BIND`, or `server.bind` to expose it on a network. The Docker image
explicitly sets `JSON2PPTX_BIND=0.0.0.0` so published ports continue to work.

Set the environment-only `JSON2PPTX_API_TOKEN` to require
`Authorization: Bearer <token>` on every route except `GET`/`HEAD /api/v1/health`.
An unset or empty token preserves unauthenticated operation. Downloads and
readiness are protected when a token is configured. Use TLS at the reverse
proxy for remote token-authenticated access.

CORS is disabled by default. Set `JSON2PPTX_CORS_ALLOWED_ORIGINS` to a
comma-separated list of exact origins such as `https://slides.example`, or set
`server.cors_allowed_origins` in YAML. An explicit `*` opts into any origin.
Allowed preflights support `GET`, `HEAD`, `POST` and the `Authorization`,
`Content-Type`, `X-Request-ID` headers. Preflights need no token; actual requests
still require it. Cookies/credentialed CORS are not enabled. Responses expose
`X-Request-ID` and `Retry-After` to allowed browser clients.

`X-Request-ID` is echoed only when it matches `^[A-Za-z0-9._-]{1,64}$`;
otherwise the server generates an ID. The same ID is used in server logs.

## Error Handling

Transport and request errors follow a consistent JSON structure:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable description",
    "details": {
      "field": "affected_field"
    }
  }
}
```

> **Exception — diagnostic-bearing endpoints.** The semantic validate/compile
> endpoints and the pattern validation endpoints (`POST /api/v1/patterns/{name}/validate`
> and `/expand`) carry a `FindingEnvelope` — the same agent-facing diagnostic
> contract emitted by the CLI and MCP surfaces (see
> [docs/AGENT_DIAGNOSTICS.md](../AGENT_DIAGNOSTICS.md)). `POST /api/v1/semantic/validate`
> responds **HTTP 200** after completed validation with `{valid, findings, validation_scope}` (validation completing is
> the success case; `valid`/`findings.ok` report the result).
> `POST /api/v1/semantic/compile` responds **HTTP 200** on success and **HTTP 422**
> with `{ok:false, error, findings}` when the spec cannot compile. Pattern
> validation failures use **HTTP 400**. Transport failures on these endpoints
> (400/401/403/404/413/415/500/503/504) still use the simple shape above.

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid configured bearer token |
| `INVALID_REQUEST` | 400, 403 | Malformed parameters or conflicting template sources; 403 for CORS rejection |
| `INVALID_JSON` | 400 | Malformed JSON body |
| `INVALID_YAML` | 400 | Malformed YAML body |
| `INVALID_TEMPLATE` | 400 | Invalid template name or uploaded template |
| `TEMPLATE_NOT_FOUND` | 404 | Registered template does not exist |
| `TEMPLATE_ERROR` | 500 | Template analysis failed |
| `INVALID_INPUT` | 400 | Input validation failed |
| `INVALID_SLIDE_TYPE` | 400 | Unsupported basic slide type |
| `INVALID_CONTENT_TYPE` | 415 | Unsupported request Content-Type |
| `GENERATION_ERROR` | 500 | PPTX generation failed |
| `RENDER_ERROR` | 500 | Semantic trial or thumbnail rendering failed internally |
| `FILE_NOT_FOUND` | 404 | Generated file not found or expired |
| `FILE_ERROR` | 500 | File system access failed |
| `RATE_LIMITED` | 503 | Shared work capacity exhausted; retry after Retry-After |
| `REQUEST_TOO_LARGE` | 413 | Request exceeds the endpoint body limit |
| `REQUEST_TIMEOUT` | 504 | Work exceeded its request deadline |
| `PATTERN_NOT_FOUND` | 404 | Named pattern does not exist |
| `PATTERN_VALIDATION_FAILED` | 400 | Reserved pattern-data validation transport code; diagnostic failures normally use FindingEnvelope |
| `PATTERN_EXPAND_FAILED` | 500 | Pattern expansion failed internally |
| `UNSUPPORTED_FEATURE` | 400, 501 | Unsupported input feature; 501 when an optional server adapter is unavailable |
| `INTERNAL_ERROR` | 500 | Unexpected server failure |
| `PANIC_RECOVERED` | 500 | Recovered handler panic |

---

## Endpoints

### Health Check

Check process liveness. This endpoint stays public when bearer authentication is enabled.
Dependency readiness is reported separately at `/api/v1/ready`.

**Endpoint:** `GET /api/v1/health`

#### Request

```bash
curl http://localhost:8080/api/v1/health
```

#### Response

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "commit_sha": "a1b2c3d",
  "build_time": "2026-01-15T10:30:00Z",
  "uptime_seconds": 3600
}
```

---

### Readiness

`GET /api/v1/ready` returns `200` when the complete HTTP workflow's dependencies
are available, otherwise `503`. Its `checks` object reports `templates`,
`output`, and `render_toolchain`; `missing_render_tools` names missing tools.
The probe checks that at least one `.pptx` can be opened, creates/writes/removes
an output probe, and checks for LibreOffice and Poppler executables. It does
not parse every template or launch a conversion. A missing thumbnail toolchain
makes readiness fail even if PPTX-only generation would succeed.

### Capabilities

Get the machine-readable feature boundary of the HTTP API.

**Endpoint:** `GET /api/v1/capabilities`

#### Request

```bash
curl http://localhost:8080/api/v1/capabilities
```

#### Response

Returns a JSON object describing supported fields, unsupported features,
available endpoints, and MCP-only tools. `version` is the actual binary build
version; `schema_version` and `schema_fingerprint` match the executable schema
identity (also exposed by health). `semantic.validation_mode` is `trial_render`
for `json2pptx serve`, or `schema_only` for an embedded server without a validator.
See the OpenAPI spec for the full schema.

---

### List Templates

Retrieve all available PPTX templates.

**Endpoint:** `GET /api/v1/templates`


#### Request

```bash
curl http://localhost:8080/api/v1/templates
```

#### Response

```json
{
  "templates": [
    {
      "name": "midnight-blue",
      "display_name": "Midnight Blue",
      "aspect_ratio": "16:9",
      "layout_count": 6
    },
    {
      "name": "forest-green",
      "display_name": "Forest Green",
      "aspect_ratio": "16:9",
      "layout_count": 6
    },
    {
      "name": "modern-template",
      "display_name": "Modern Template",
      "aspect_ratio": "16:9",
      "layout_count": 6
    },
    {
      "name": "warm-coral",
      "display_name": "Warm Coral",
      "aspect_ratio": "16:9",
      "layout_count": 6
    }
  ]
}
```

---

### Get Template Details

Get detailed information about a specific template including layouts, colors, and fonts.
`slide_width_emu` and `slide_height_emu` preserve the authored dimensions;
`aspect_ratio` supports 4:3, 16:9, 21:9 and other measured ratios.
`theme.semantic_accents` exposes metadata role-to-theme mappings such as
`positive: accent4`. Discovery and generation use the same cached analyzer.

**Endpoint:** `GET /api/v1/templates/{name}`


#### Parameters

| Parameter | Location | Required | Description |
|-----------|----------|----------|-------------|
| `name` | path | Yes | Template name (without `.pptx` extension) |

#### Request

```bash
curl http://localhost:8080/api/v1/templates/midnight-blue
```

#### Response

```json
{
  "name": "midnight-blue",
  "display_name": "Midnight Blue",
  "aspect_ratio": "16:9",
  "layouts": [
    {
      "id": "layout1",
      "name": "Title Slide",
      "tags": ["title", "opening"],
      "placeholders": ["title", "body"]
    },
    {
      "id": "layout2",
      "name": "Title and Content",
      "tags": ["content", "bullets"],
      "placeholders": ["title", "body", "content"]
    }
  ],
  "theme": {
    "colors": ["#1F4E79", "#2E75B6", "#9DC3E6", "#DEEBF7", "#000000", "#FFFFFF"],
    "fonts": {
      "title": "Calibri Light",
      "body": "Calibri"
    }
  }
}
```

---

### Convert Slides

Convert JSON slide definitions to a PowerPoint presentation.

**Endpoint:** `POST /api/v1/convert`


#### Request Body

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `template` | string | Yes | - | Template name |
| `slides` | array | Yes | - | Array of slide definitions |
| `options.output_format` | string | No | `"file"` | `"file"` or `"base64"` |
| `options.svg_scale` | number | No | `2.0` | Scale factor for SVG chart rendering |
| `options.exclude_template_slides` | boolean | No | `true` | Exclude original template slides from output |

#### Slide Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Slide type: `title`, `content`, `section`, `two-column`, `chart`, `diagram`, `image`, `comparison`, `blank`. See [List Slide Types](#list-slide-types) for descriptions. |
| `title` | string | No | Slide title |
| `content` | object | No | Content with `body` (string) and/or `bullets` (string[]). **Note:** The HTTP API only supports text/bullet content. For chart data, diagram specs, tables, or images, use MCP or CLI. |
| `speaker_notes` | string | No | Speaker notes |
| `source` | string | No | Source attribution |

#### Request (File Output)

```bash
curl -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {
        "type": "title",
        "title": "Q4 Report",
        "content": {"body": "Jane Smith | Strategy Team"}
      },
      {
        "type": "content",
        "title": "Introduction",
        "content": {
          "bullets": [
            "Revenue growth of 15%",
            "New market expansion",
            "Team grew by 20%"
          ]
        }
      }
    ]
  }'
```

#### Response (File Output)

```json
{
  "success": true,
  "file_url": "/api/v1/download/a1b2c3d4e5f6789012345678.pptx",
  "expires_at": "2025-01-17T15:30:00Z",
  "stats": {
    "slide_count": 2,
    "processing_time_ms": 1250,
    "warnings": []
  }
}
```

#### Request (Base64 Output)

```bash
curl -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "warm-coral",
    "slides": [
      {
        "type": "content",
        "title": "Summary",
        "content": {"bullets": ["Point one", "Point two"]}
      }
    ],
    "options": {
      "output_format": "base64"
    }
  }'
```

#### Response (Base64 Output)

```json
{
  "success": true,
  "data": "UEsDBBQAAAAIAG5hdGl2ZS1tYWluLnB...",
  "filename": "Quick Report.pptx",
  "stats": {
    "slide_count": 1,
    "processing_time_ms": 1100,
    "warnings": []
  }
}
```

#### Error Responses

**Missing slides:**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Slides array is required",
    "details": {
      "field": "slides"
    }
  }
}
```

**Template not found:**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_TEMPLATE",
    "message": "Template 'nonexistent' not found"
  }
}
```

---

### Semantic Deck Specs

Author compact semantic YAML/JSON and let json2pptx compile it to the raw
`PresentationInput` model. The **request body is the spec document itself** —
`Content-Type: application/json` is parsed as JSON, `application/x-yaml` as YAML.
Tuning options ride as query parameters so the body stays a clean spec.

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/semantic/kinds` | Discover slide kinds, examples and template-aware budgets |
| `GET /api/v1/semantic/schema` | Return the semantic deck-spec JSON Schema |
| `POST /api/v1/semantic/validate` | Trial-render validation; returns `{valid, findings, validation_scope}` (HTTP 200 on completion) |
| `POST /api/v1/semantic/compile` | Compile a semantic spec to raw `PresentationInput` JSON |
| `POST /api/v1/semantic/render` | Compile and render a semantic spec to `.pptx` (same runner as `json2pptx semantic render` / `render_deck_spec`) |

Query parameters:

| Parameter | Endpoint | Default | Description |
|-----------|----------|---------|-------------|
| `strict` | validate, compile, render | `warn` | Advisory-rule severity: `off`, `warn`, or `strict` |
| `template` | validate, compile, render | — | Registered template for this request; replaces the spec's `meta.template` (`warnings[]` says so) |
| `include_compiled_json` | compile | `false` | When `true`, include the full compiled `PresentationInput` under `compiled_json` |
| `output_validation` | render | `strict` | Post-generation output validation: `off`, `warn`, or `strict` |

Example request:

```bash
curl -X POST "http://localhost:8080/api/v1/semantic/compile?include_compiled_json=true" \
  -H "Content-Type: application/x-yaml" \
  --data-binary @qbr.yaml
```

Semantic findings carry `path`, a JSON Pointer into the spec that was sent
(`/slides/2/kpis/1/label`; it always resolves there), `missing_path` for a
field the spec lacks, `slide_number` and `blocking` — the same form
`validate_deck_spec` reports. When a raw validator emits a generated JSON
pointer, the semantic source map translates it back to the closest semantic
field and keeps the raw pointer under `debug`.

#### Discover and validate

`GET /api/v1/semantic/kinds?kinds=title&fields=budgets&template=midnight-blue`
uses the same catalogue builder and filters as MCP `list_slide_kinds`.
`kinds` and `fields` are comma-separated lists. Omit filters for the complete
catalogue; invalid filters return `400` with a `FindingEnvelope`. Supply a
registered `template` to compute its budgets before authoring.

Validation with a selected template performs a scratch render through the
HTTP renderer, including native text fit/readability, asset policy and strict
output validation. It publishes no deck. `validation_scope` reports how far
checks ran: `schema` for an unparseable document, `compile` for compilation
failure, `template_free` when no template is selected, and `render` for a trial.
A successful template-free check is not a template-specific fit guarantee.
`template` overrides `meta.template`, with `warnings[]` explaining the change.
Validation accepts raw JSON/YAML, so uploaded assets/templates must be checked
with multipart `/semantic/render`. Server embeddings without a trial validator
report `schema` and reject the `template` query parameter instead of ignoring it.
Validation, kind discovery, rendering, conversion and thumbnails share the
configured work semaphore; saturation returns `503 RATE_LIMITED` with
`Retry-After`, and deadline expiry returns `504 REQUEST_TIMEOUT`.

#### Render

The `template` query parameter names a registered template. It renders the
deck whether or not the spec pins `meta.template`; when it replaces a pin the
result's `warnings[]` says so.

`POST /api/v1/semantic/render` runs the same presentation runner as the
`json2pptx semantic render` CLI and the `render_deck_spec` MCP tool — same
compile, design-mode, asset, template, text-fit / readability and output
validation (strict by default) policies — using the server's own runtime
configuration (templates dir, SVG strategy, `ALLOWED_IMAGE_PATHS`).

The body is either the raw spec document (as for validate/compile) or
`multipart/form-data` with:

| Part | Required | Description |
|------|----------|-------------|
| `spec` | Yes | The spec (file or field). A `.json` filename or a `{`-leading field parses as JSON, otherwise YAML |
| `template` | No | A bring-your-own `.pptx`. Used when the spec pins no `meta.template` (`meta.template` wins over an uploaded file and a warning says so) |
| `assets` | No, repeatable | Asset files. Relative asset paths in the spec resolve against them by base filename |

Combining the `template` query parameter with an uploaded template is rejected
with `400 INVALID_REQUEST` before rendering. A spec pin and uploaded template
retain the documented precedence above.

A relative asset path to a file that was not uploaded is refused. Local image
paths are restricted to the uploaded assets, the URL download cache and any
configured `ALLOWED_IMAGE_PATHS` roots, so a request cannot embed arbitrary
server files.

```bash
curl -X POST "http://localhost:8080/api/v1/semantic/render" \
  -F "spec=@deck.yaml" -F "template=@brand.pptx" -F "assets=@logo.png"
```

Success (HTTP 200) returns the CLI render result with server-local paths
replaced by download and thumbnail links:

```json
{
  "ok": true,
  "file_url": "/api/v1/download/a1b2c3d4e5f6789012345678abcdef01.pptx",
  "expires_at": "2026-09-30T10:00:00Z",
  "template": "midnight-blue",
  "slide_count": 6,
  "content_hash": "…",
  "quality": {"score": 96, "…": "…"},
  "deterministic_ready": true,
  "publishable": false,
  "warnings": [],
  "diagnostics": []
}
```

A refused render (HTTP 422) returns `ok: false`, `error` and diagnostics
addressed at the DeckSpec (`semantic_path`, with the compiled `raw_path` kept as
evidence) — e.g. a `TEXT_BELOW_READABLE_MIN` readability refusal at
`slides[3].slide`, an `IMAGE_PATH` for a missing asset, or `TEMPLATE_NOT_FOUND`
at `meta.template`. Renders share the `/convert` concurrency limit (HTTP 503
with `Retry-After` when saturated).

---

### Thumbnails

`GET /api/v1/thumbnails/{filename}` renders a generated download artifact into
inline PNG images for visual inspection. Use the `thumbnails_url` returned by
semantic render, or the filename from a basic convert download URL. Only the
server's generated 32-hex-character `.pptx` filenames are accepted; arbitrary
paths and symlinks are rejected. Retention is the same as the download.

Optional query parameters: `density` (36–150 DPI, default 72), `max_slides`
(1–50, default 20), or comma-separated zero-based `slide_indices` (at most 50,
overrides `max_slides`). The response includes `slides`, `slide_count`,
`truncated`, and `selected` for selective rendering. Each image carries
`index`, `png_base64`, `width`, `height`, `content_hash` (pixels) and `source_hash`
(PPTX); server-local file paths are never returned. Out-of-range indices return
`400`; missing/expired artifacts return `404`.

### Download File

Download a previously generated PPTX file.

**Endpoint:** `GET /api/v1/download/{filename}`


**File Expiration:** 1 hour after generation

#### Parameters

| Parameter | Location | Required | Description |
|-----------|----------|----------|-------------|
| `filename` | path | Yes | Filename from convert response (e.g., `a1b2c3d4e5f6789012345678.pptx`) |

#### Request

```bash
curl -O http://localhost:8080/api/v1/download/a1b2c3d4e5f6789012345678.pptx
```

#### Response

- **Content-Type:** `application/vnd.openxmlformats-officedocument.presentationml.presentation`
- **Content-Disposition:** `attachment; filename="presentation.pptx"`
- **Body:** Binary PPTX file data

#### Error Response

```json
{
  "success": false,
  "error": {
    "code": "FILE_NOT_FOUND",
    "message": "File not found or expired"
  }
}
```

---

### List Slide Types

Retrieve supported slide types.

**Endpoint:** `GET /api/v1/slide-types`


#### Request

```bash
curl http://localhost:8080/api/v1/slide-types
```

#### Response

```json
{
  "slide_types": [
    {"type": "title", "description": "Opening slide with title and optional subtitle"},
    {"type": "content", "description": "Standard bullet slide with title and body"},
    {"type": "two-column", "description": "Side-by-side content layout"},
    {"type": "image", "description": "Image-focused slide with title"},
    {"type": "chart", "description": "Data visualization slide"},
    {"type": "comparison", "description": "Comparison layout for side-by-side elements"},
    {"type": "blank", "description": "Empty slide with no placeholders"},
    {"type": "section", "description": "Section divider slide for separating presentation sections"},
    {"type": "diagram", "description": "Diagram slide (pyramid, venn, org chart, etc.)"}
  ]
}
```

---

### List Patterns

Retrieve all available named patterns (reusable shape_grid skeletons).

**Endpoint:** `GET /api/v1/patterns`

#### Request

```bash
curl http://localhost:8080/api/v1/patterns
```

#### Response

```json
{
  "patterns": [
    {
      "name": "bmc",
      "description": "Business Model Canvas with 9 labeled cells",
      "use_when": "User asks for a Business Model Canvas",
      "version": 1,
      "cells_hint": "9 cells: key_partners, key_activities, key_resources, value_propositions, customer_relationships, channels, customer_segments, cost_structure, revenue_streams",
      "supports_callout": false
    }
  ]
}
```

---

### Get Pattern Details

Get full details for a specific pattern including its JSON schema.

**Endpoint:** `GET /api/v1/patterns/{name}`

#### Parameters

| Parameter | Location | Required | Description |
|-----------|----------|----------|-------------|
| `name` | path | Yes | Pattern name (e.g., `bmc`, `kpi-dashboard`) |

#### Request

```bash
curl http://localhost:8080/api/v1/patterns/bmc
```

#### Response

```json
{
  "name": "bmc",
  "description": "Business Model Canvas with 9 labeled cells",
  "use_when": "User asks for a Business Model Canvas",
  "version": 1,
  "cells_hint": "9 cells: ...",
  "supports_callout": false,
  "schema": { "type": "object", "properties": { "..." : {} } }
}
```

#### Error Response

```json
{
  "success": false,
  "error": {
    "code": "PATTERN_NOT_FOUND",
    "message": "Pattern \"nonexistent\" not found; did you mean \"bmc\"?"
  }
}
```

---

### Validate Pattern Data

Validate pattern input values against the pattern's schema without expanding.

**Endpoint:** `POST /api/v1/patterns/{name}/validate`

#### Parameters

| Parameter | Location | Required | Description |
|-----------|----------|----------|-------------|
| `name` | path | Yes | Pattern name |

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `values` | object | Yes | Pattern-specific data values |
| `overrides` | object | No | Style overrides for the pattern |
| `cell_overrides` | object | No | Per-cell overrides keyed by cell index (string) |

#### Request

```bash
curl -X POST http://localhost:8080/api/v1/patterns/bmc/validate \
  -H "Content-Type: application/json" \
  -d '{
    "values": {
      "key_partners": "Suppliers",
      "key_activities": "Manufacturing",
      "value_propositions": "Quality products"
    }
  }'
```

#### Response (Valid)

```json
{"ok": true}
```

#### Response (Invalid)

A validation failure returns a `FindingEnvelope` (HTTP 400) — the shared
agent-facing diagnostic contract also emitted by the CLI and MCP surfaces, not
the simple error envelope. The originating pattern name rides on each finding's
`evidence.pattern`. (Transport failures — 404/415/413 — still use the simple
`{success, error{...}}` shape.)

```json
{
  "schema_version": "1.0",
  "tool": "json2pptx",
  "subcommand": "validate_pattern",
  "ok": false,
  "summary": "1 error",
  "findings": [
    {
      "id": "input-1",
      "code": "INPUT.required",
      "severity": "error",
      "category": "INPUT",
      "message": "missing required field: value_propositions",
      "evidence": {"pattern": "bmc"}
    }
  ]
}
```

---

### Expand Pattern

Validate and expand pattern values into a `shape_grid` ready for slide generation.

**Endpoint:** `POST /api/v1/patterns/{name}/expand`

#### Parameters

| Parameter | Location | Required | Description |
|-----------|----------|----------|-------------|
| `name` | path | Yes | Pattern name |

#### Request Body

Same as [Validate Pattern Data](#validate-pattern-data).

#### Request

```bash
curl -X POST http://localhost:8080/api/v1/patterns/bmc/expand \
  -H "Content-Type: application/json" \
  -d '{
    "values": {
      "key_partners": "Suppliers",
      "key_activities": "Manufacturing",
      "value_propositions": "Quality products"
    }
  }'
```

#### Response

```json
{
  "shape_grid": {
    "bounds": {"x": 5, "y": 20, "width": 90, "height": 73.3},
    "rows": [
      {
        "cells": [
          {"shape": {"geometry": "rect", "text": "Key Partners\nSuppliers", "fill": "#4472C4"}}
        ]
      }
    ]
  }
}
```

---

## Common Use Cases

### Generate and Download Presentation

```bash
# 1. Convert JSON to PPTX
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {
        "type": "content",
        "title": "Hello World",
        "content": {"bullets": ["First slide content"]}
      }
    ]
  }')

# 2. Extract download URL
FILE_URL=$(echo $RESPONSE | jq -r '.file_url')

# 3. Download the file
curl -o presentation.pptx "http://localhost:8080${FILE_URL}"
```

### Generate Base64 for Direct Use

```bash
# Get base64-encoded PPTX
curl -s -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {
        "type": "content",
        "title": "Summary",
        "content": {"bullets": ["Key point"]}
      }
    ],
    "options": {
      "output_format": "base64"
    }
  }' | jq -r '.data' | base64 -d > report.pptx
```

### Explore Available Templates

```bash
# List all templates
curl -s http://localhost:8080/api/v1/templates | jq '.templates[].name'

# Get details for a specific template
curl -s http://localhost:8080/api/v1/templates/midnight-blue | jq '.layouts[].name'
```

### Check Service Health

```bash
# Basic health check
curl -s http://localhost:8080/api/v1/health | jq '.status'

# Monitor health
watch -n 5 'curl -s http://localhost:8080/api/v1/health | jq'
```

---

## SDK Examples

### JavaScript/TypeScript

```typescript
interface ConvertOptions {
  output_format?: 'file' | 'base64';
  svg_scale?: number;
  exclude_template_slides?: boolean;
}

interface SlideContent {
  body?: string;
  bullets?: string[];
}

interface Slide {
  type: string;
  title?: string;
  content?: SlideContent;
  speaker_notes?: string;
  source?: string;
}

interface ConvertRequest {
  template: string;
  slides: Slide[];
  options?: ConvertOptions;
}

interface ConvertResponse {
  success: boolean;
  file_url?: string;
  expires_at?: string;
  data?: string;
  filename?: string;
  stats: {
    slide_count: number;
    processing_time_ms: number;
    warnings: string[];
  };
}

async function convertSlides(request: ConvertRequest): Promise<ConvertResponse> {
  const response = await fetch('http://localhost:8080/api/v1/convert', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error.message);
  }

  return response.json();
}

// Usage
const result = await convertSlides({
  template: 'midnight-blue',
  slides: [
    {
      type: 'title',
      title: 'Q4 Report',
      content: { body: 'Strategy Team' },
    },
    {
      type: 'content',
      title: 'Introduction',
      content: { bullets: ['Revenue growth of 15%', 'New market expansion'] },
    },
  ],
});

if (result.file_url) {
  console.log(`Download: http://localhost:8080${result.file_url}`);
}
```

### Go

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type SlideCreatorClient struct {
	BaseURL string
	Client  *http.Client
}

type SlideContent struct {
	Body    string   `json:"body,omitempty"`
	Bullets []string `json:"bullets,omitempty"`
}

type Slide struct {
	Type         string       `json:"type"`
	Title        string       `json:"title,omitempty"`
	Content      SlideContent `json:"content,omitempty"`
	SpeakerNotes string       `json:"speaker_notes,omitempty"`
}

type ConvertRequest struct {
	Template string          `json:"template"`
	Slides   []Slide         `json:"slides"`
	Options  *ConvertOptions `json:"options,omitempty"`
}

type ConvertOptions struct {
	OutputFormat          string  `json:"output_format,omitempty"`
	SVGScale              float64 `json:"svg_scale,omitempty"`
	ExcludeTemplateSlides bool    `json:"exclude_template_slides,omitempty"`
}

type ConvertResponse struct {
	Success   bool   `json:"success"`
	FileURL   string `json:"file_url,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Data      string `json:"data,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Stats     struct {
		SlideCount       int      `json:"slide_count"`
		ProcessingTimeMs int64    `json:"processing_time_ms"`
		Warnings         []string `json:"warnings"`
	} `json:"stats"`
}

func NewClient(baseURL string) *SlideCreatorClient {
	return &SlideCreatorClient{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
}

func (c *SlideCreatorClient) Convert(req ConvertRequest) (*ConvertResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Post(
		c.BaseURL+"/api/v1/convert",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ConvertResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *SlideCreatorClient) Download(fileURL, outputPath string) error {
	resp, err := c.Client.Get(c.BaseURL + fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func main() {
	client := NewClient("http://localhost:8080")

	result, err := client.Convert(ConvertRequest{
		Template: "midnight-blue",
		Slides: []Slide{
			{
				Type:  "title",
				Title: "Q4 Report",
				Content: SlideContent{Body: "Strategy Team"},
			},
			{
				Type:  "content",
				Title: "Introduction",
				Content: SlideContent{
					Bullets: []string{"Revenue growth of 15%", "New market expansion"},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	if result.FileURL != "" {
		err = client.Download(result.FileURL, "presentation.pptx")
		if err != nil {
			panic(err)
		}
		fmt.Printf("Downloaded: presentation.pptx (%d slides)\n", result.Stats.SlideCount)
	}
}
```

---

## OpenAPI Specification

The complete OpenAPI 3.0 specification is available at:

- **File:** [`openapi.yaml`](./openapi.yaml)
- **Swagger UI:** Import the spec into [Swagger Editor](https://editor.swagger.io/)
- **Postman:** Import the OpenAPI spec directly into Postman for interactive testing

---

## MCP Tool Reference Pages

Detailed reference documentation for individual MCP tools:

- [analyze_deck_rhythm](./analyze_deck_rhythm.md) — Analyze visual rhythm, detect monotony, score composition
- [plan_deck](./plan_deck.md) — Plan a deck outline from a brief with rhythm-aware pattern selection
- [recommend_visual](./recommend_visual.md) — Rank layouts, patterns, charts and diagrams for a content intent (primary recommender)
- [recommend_pattern](./recommend_pattern.md) — Folded pattern-only alias of `recommend_visual`

---

## Support

For issues and feature requests, please open an issue in the repository.
