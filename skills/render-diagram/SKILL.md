---
name: render-diagram
description: >
  Render and validate standalone SVG/PNG charts and diagrams via the svggen-mcp
  server. Use when the user wants a single chart/diagram artifact (SVG markup
  or PNG bytes) without generating a full PowerPoint deck. For embedding the
  rendered SVG inside a deck slide (shape_grid icon.svg_data), see
  ../generate-deck/SKILL.md.
---

# Render Diagram Skill

You drive **`svggen-mcp`** — the standalone SVG/PNG renderer for charts and diagrams. This skill is scoped to that one server (six tools, no deck-level concerns).

Use this skill when:
- The user wants a single chart or diagram as SVG markup or a PNG image.
- You need to validate or preflight a chart payload before any rendering work.
- You need to discover what diagram types exist and what data shape each expects.

**Do not** use this skill when generating a full deck. For deck workflows — patterns, layouts, fit-report, repair — load `../generate-deck/SKILL.md`. That skill cross-links back here for the inline-embed case (`shape_grid` `icon.svg_data`).

---

## Connecting to `svggen-mcp`

`svggen-mcp` is a stdio MCP server. Build it from this repo:

```bash
cd svggen && go build ./cmd/svggen-mcp
```

Register it with your MCP client:

```json
{
  "mcpServers": {
    "svggen": {
      "command": "/absolute/path/to/svggen-mcp"
    }
  }
}
```

The server has **no flags** beyond `--version` / `--help`. It logs to stderr and serves the MCP protocol over stdio.

---

## Tool Catalog

These are the only tools served by `svggen-mcp`. Treat this table as authoritative; if `get_capabilities` reports a different list, the server has drifted and you should re-read this skill.

| Tool | Purpose | Notes |
|---|---|---|
| `get_started` | Returns the recommended ordered call sequence for a stated task. | Tasks: `"render"` (default), `"preflight-render"`, `"embed-in-deck"`. Unknown values fall back to `"render"`. Call this **first** if you are unsure of the workflow. |
| `get_capabilities` | Returns `{schema_version, tool_list, chart_types, diagram_types, chart_capabilities, diagram_capabilities, deprecations, features:{dry_render, structured_errors}}`. | `schema_version` is sourced from the svggen library version. Call once per session and compare against any cached value — a change means the contract may have shifted. **Distinct from `json2pptx-mcp.get_capabilities`** (that one is scoped to the deck engine). |
| `list_diagram_types` | Lists every registered diagram/chart type as `[{name, aliases?}]`. | `name` is the canonical ID (e.g., `bar_chart`). `aliases` enumerates accepted short forms (e.g., `["bar"]`). Prefer the canonical name in new code; aliases remain accepted everywhere `render_diagram` takes a `type`. |
| `get_diagram_schema` | Returns the input schema for a specific `type`, plus `example_values` with both `minimal` (smallest valid input) and `realistic` (representative shape and content). | Mirrors `json2pptx-mcp.show_pattern.example_values` — copy a working example instead of guessing field names. The legacy top-level `example` field is retained as a back-compat alias for `example_values.realistic`. |
| `validate_diagram` | Validates a `{type, data}` payload **without** rendering. | Returns `{valid, errors?:[diagnostic]}`. Use this when structured feedback is cheaper than a failed render. |
| `render_diagram` | Renders to SVG (default) or PNG. Optional `dry_run`. | Required: `type`, `data`. Optional: `format` (`"svg"` \| `"png"`), `width`, `height`, `title`, `style`, `dry_run` (bool). |

---

## Workflow 1: One-shot render

Use when you trust your data shape.

```
get_capabilities         → cache schema_version + feature flags
list_diagram_types       → confirm the type (or pick canonical for an alias)
get_diagram_schema       → fetch data shape + example_values
render_diagram           → produce SVG or PNG
```

Equivalent to `get_started({task: "render"})`.

---

## Workflow 2: Preflight render (`validate → dry_run → render`)

Use when validation feedback is cheaper than a failed render — bulk payloads, untrusted input, automation pipelines.

```
get_capabilities         → confirm features.dry_render is advertised
list_diagram_types       → confirm registered types
get_diagram_schema       → fetch data shape
validate_diagram         → catch required-field / type-mismatch / constraint errors
render_diagram(dry_run)  → re-run layout + labeling, return only findings (no bytes)
render_diagram           → final render once both passes are clean
```

### `validate_diagram` vs `render_diagram(dry_run: true)` — why both

They cover **different failure surfaces**. Run them in order, not as substitutes:

| Step | Catches | Does not catch |
|---|---|---|
| `validate_diagram` | Required fields, type mismatches, length/shape constraints (e.g., `series[i].values` length aligned with `categories`). | Layout findings — tick thinning, label clipping, legend overflow. |
| `render_diagram` with `dry_run: true` | Layout findings: `chart.tick_thinned`, `chart.label_clipped`, `chart.legend_overflow_dropped`, `chart.label_truncated`, `chart.scatter_label_skipped`, etc. | Schema errors validate_diagram already surfaced. |

`dry_run: true` returns the JSON envelope `{valid, findings, error?}` — no SVG/PNG bytes are produced. It's the same render path minus the byte output, so it surfaces post-layout diagnostics that pure schema validation cannot see.

---

## Workflow 3: Inline embed into a deck (`shape_grid` `icon.svg_data`)

Use when the final consumer of the SVG is a `shape_grid` cell inside a json2pptx deck. The skill that **owns** deck composition is `../generate-deck/SKILL.md` — load it for slide assembly. The svggen side is:

1. Resolve the deck template's palette **once** through `json2pptx-mcp`:

   ```jsonc
   // json2pptx-mcp.resolve_theme({"template_name": "midnight-blue"})  →
   {
     "template": "midnight-blue",
     "colors": { "accent1": "#1F4E79", "accent2": "#2E75B6", "...": "..." },
     "theme_colors": [
       { "name": "accent1", "rgb": "#1F4E79" },
       { "name": "accent2", "rgb": "#2E75B6" }
     ]
   }
   ```

2. Copy `theme_colors` **verbatim** into every `render_diagram` call's `style`:

   ```jsonc
   svggen-mcp.render_diagram({
     "type": "bar_chart",
     "format": "svg",
     "data": { /* ... */ },
     "style": {
       "theme_colors": [
         { "name": "accent1", "rgb": "#1F4E79" },
         { "name": "accent2", "rgb": "#2E75B6" }
       ]
     }
   })
   ```

3. Drop the returned SVG markup into a `shape_grid` cell:

   ```json
   "icon": {
     "svg_data": "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 800 600\">…</svg>",
     "alt": "Quarterly revenue vs. cost"
   }
   ```

### Theme-colors copy contract (mandatory)

- **Do not hand-pivot** from the `colors` map into a fabricated `theme_colors` array. Scheme-name typos (`accent1` vs. `accent_1`) would be silent.
- **Do not** strip, reorder, or rename entries from `theme_colors`. The renderer relies on positional and named role assumptions.
- If the deck applies a `theme_override`, pass that **same object** as `resolve_theme`'s `theme_override` argument so the array reflects the post-override palette, then copy from there.
- One `resolve_theme` call per deck is enough — cache the array for the session and reuse across every diagram render.

`get_started({task: "embed-in-deck"})` returns this exact sequence so an agent does not have to reconstruct it.

---

## Format choice

- `format: "svg"` (default) → returns raw SVG markup as the tool result text. **Required** for `shape_grid` `icon.svg_data` embedding.
- `format: "png"` → returns base64-encoded PNG as an MCP `ImageContent` block. Use for previewing or for consumers that cannot ingest SVG.

`dry_run: true` ignores `format` — it returns the findings JSON regardless.

---

## Error envelopes

`svggen-mcp` returns **structured errors** through `mcp.NewToolResultError` whose text body is JSON of the form:

```jsonc
{
  "valid": false,
  "diagnostics": [
    {
      "code": "required",           // lowercase_snake, e.g. "required", "invalid_type", "invalid_value", "unknown_diagram_type", "render_failed", "parse_failed"
      "message": "data is required",
      "path": "data",               // JSON path, e.g. "data.series[0].values"
      "severity": "error",
      "fix": {                      // optional structured remediation
        "kind": "provide_value",    // see fix.kind enum below
        "params": { "path": "data" }
      },
      "next_tool_call": {           // optional machine-readable next MCP call
        "tool": "get_diagram_schema",
        "args_template": { "type": "bar_chart" }
      },
      "details": { "pattern": "bar_chart" }
    }
  ]
}
```

`validate_diagram` returns the same envelope under the key `errors` (instead of `diagnostics`) and always succeeds at the MCP level — even when `valid: false`. The naming difference is deliberate: `errors` aligns with `validate_pattern` in `json2pptx-mcp`, while `diagnostics` is the channel for render-time failures.

### Common `code` values

| `code` | When | Typical `next_tool_call` |
|---|---|---|
| `required` | Required argument missing (`type`, `data`). | `list_diagram_types` (for `type`) or `get_diagram_schema` (for `data`). |
| `invalid_type` | Argument has the wrong JSON kind (e.g., `data` is not an object). | `get_diagram_schema` |
| `invalid_value` | Argument is the right kind but disallowed (e.g., unsupported `format`, malformed `style`). | none — the `fix.params.allowed` array carries the legal set. |
| `unknown_diagram_type` | `type` does not resolve to a registered diagram (alias or canonical). | `list_diagram_types` |
| `render_failed` | Renderer crashed past validation (rare; usually means the input slipped through schema checks). | `get_diagram_schema` |
| `parse_failed` | Envelope-level parse error inside `validate_diagram`. | none — fix the payload. |
| Chart-finding codes (`align_series`, `truncate_or_split`, `replace_value`, `explicit_scale`, `reduce_items`) appear as `fix.kind` values on per-error diagnostics. | Returned by `validate_diagram` / dry_run when chart-specific rules fail. | Apply the suggested fix and re-validate. |

### `fix.kind` enum

Each diagnostic's `fix.kind` is one of:

- `provide_value` — supply the missing field at `fix.params.path`.
- `replace_value` — current value at `fix.params.path` is wrong (`invalid_value` carries the rejected input; `allowed` may carry the legal set).
- `align_series` — series length must equal `len(categories)`.
- `truncate_or_split` — payload exceeds the renderer's density budget; cut entries or split across multiple diagrams.
- `explicit_scale` — supply explicit axis scale / bounds.
- `reduce_items` — too many items for the chosen renderer; trim before retry.

`fix.kind` mirrors the chart-finding enum documented in `../generate-deck/FINDINGS.md`. The values are the same vocabulary; this server emits the subset relevant to svggen.

---

## Pitfalls

- **Stale `schema_version`.** Cache `get_capabilities().schema_version` per session and revalidate at the start of each new session. A bump means tool signatures, data shapes, or feature flags may have changed.
- **Guessing field names from `list_diagram_types` alone.** `list_diagram_types` only gives names; always pull `get_diagram_schema` before building a `data` payload.
- **Confusing the two MCP servers.** `svggen-mcp` and `json2pptx-mcp` both expose `get_capabilities`, `get_started`, etc. The two are independent and report different `schema_version` values. Do not assume calling one queries the other.
- **Inline SVG vs. PNG for embedding.** `shape_grid` `icon.svg_data` requires SVG markup — do not pass base64 PNG into that field. PNG output is for previews or non-pptx consumers.
- **Style mutations after `resolve_theme`.** Renaming, sorting, or filtering `theme_colors` breaks palette assumptions silently. Pass the array through untouched.

---

## Reference

- `../generate-deck/SKILL.md` — full deck workflow; load this when the consumer is a PPTX, not a standalone SVG.
- `../generate-deck/FINDINGS.md` — full `fix.kind` enum and the layout-finding catalog.
- `../template-deck/TEMPLATE_GUIDE.md` — `shape_grid` cell schema, including `icon.svg_data`, `path`, `url`, `name`, and `alt`.
