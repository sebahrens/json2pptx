# JSON Slide Format — Quick Tutorial

`json2pptx` turns a JSON description of a presentation into a `.pptx` file using a chosen template. This page is a short tutorial. For the canonical list of fields, types, enums, and required-vs-optional flags, query the schema directly via `get_input_schema` (MCP) or `json2pptx input-schema` (CLI) — both are generated from the live Go input structs and are the single source of truth.

## Minimal example

```json
{
  "template": "midnight-blue",
  "output_filename": "review.pptx",
  "slides": [
    {
      "layout_id": "title",
      "content": [
        {"placeholder_id": "title",    "type": "text", "text_value": "Q1 2026 Review"},
        {"placeholder_id": "subtitle", "type": "text", "text_value": "Strategy Team"}
      ]
    },
    {
      "layout_id": "content",
      "content": [
        {"placeholder_id": "title", "type": "text", "text_value": "Agenda"},
        {"placeholder_id": "body",  "type": "bullets",
         "bullets_value": ["Financials", "Customer metrics", "Roadmap"]}
      ]
    }
  ]
}
```

Generate with:

```bash
json2pptx generate -json deck.json -template midnight-blue \
  -templates-dir templates -output /tmp/out
```

## Shape of the document

- A presentation is a top-level object with a `template` and a `slides` array.
- Each slide pins a layout (canonical `layout_id` such as `title`, `content`, `two-column`, `section`, `blank`) or hints a type (`slide_type`).
- Each slide carries `content`: an ordered list of typed items that target placeholders by their normalized canonical id (`title`, `subtitle`, `body`, `body_2`, `image`, ...).
- Each content item declares its `type` and supplies the matching typed value field. The schema enforces this pairing as a discriminator: `type: "text"` requires `text_value`, `type: "bullets"` requires `bullets_value`, `type: "table"` requires `table_value`, and so on through `body_and_bullets`, `body_and_lead`, `bullet_groups`, `chart`, `diagram`, and `image`.

For richer visuals, a slide may carry a `shape_grid` (custom grid of preset shapes), a named `pattern` (e.g. `bmc-canvas`, `kpi-4up`, `process-flow`), or a `compose` envelope (nested pattern composition). These three are mutually exclusive but each may coexist with `content`.

## Icon policy

Wherever an icon is accepted, supply exactly one of `name` (bundled SVG), `path`, `url`, or `svg_data`. **Emoji codepoints are rejected** anywhere in deck JSON — icon fields, pattern values, shape text, titles, bullets, table cells, and speaker notes. Plain Unicode symbols (`→`, `•`, en/em dashes) are still allowed in text.

Run `json2pptx icons list` (CLI) or call `list_icons` (MCP) for the bundled catalog.

## Canonical schema

The Go input structs live in `cmd/json2pptx/json_schema.go`. Their JSON Schema form is published via:

- `get_input_schema` MCP tool — returns the full schema with `x-field-scope` annotations (`deck` / `slide` / `content` / `shape` / `split`), inline `enum` arrays, and `oneOf`/`allOf`/`if-then` discriminators. Digest-cacheable.
- `json2pptx input-schema` CLI — same payload printed to stdout.

Treat the schema as the contract. This tutorial is just an on-ramp.

## Validating before generating

- `json2pptx validate <input.json>` — runs the same validator the engine runs at generate time.
- `json2pptx validate <input.json> -fit-report` — adds layout-fit diagnostics (overflow, density, typography).
- MCP `validate_input` — wraps both. Errors carry structured `code` fields catalogued in `docs/FIT_FINDINGS.md`.

## Where to go next

- `docs/INPUT_FORMAT.md` — slightly longer tutorial with `shape_grid`, charts, and patch operations.
- `docs/PATTERNS.md` — authoring guide for named patterns.
- `docs/FIT_FINDINGS.md` — finding-code catalogue with severity and action semantics.
- `skills/generate-deck/` — the agent-facing skill bundle with workflow, rules, and pattern guidance.
