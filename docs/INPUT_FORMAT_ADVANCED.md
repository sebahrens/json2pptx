# JSON Input Format — Advanced Topics

Companion to [INPUT_FORMAT.md](INPUT_FORMAT.md), split out so the tutorial stays
within its line budget (`cmd/json2pptx/schema_docs_doctor_test.go`). The
canonical field list is still the live schema: `get_input_schema` (MCP) or
`json2pptx input-schema` (CLI).

## Patch input

Editing tools accept a *patch* envelope rather than a full `PresentationInput`:

```json
{
  "base": {
    "template": "midnight-blue",
    "slides": [
      {"layout_id": "title",   "content": [{"placeholder_id": "title", "type": "text", "text_value": "Original"}]},
      {"layout_id": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "Slide 2"}]}
    ]
  },
  "operations": [
    {"op": "replace", "slide_index": 0, "slide": {"layout_id": "title",   "content": [{"placeholder_id": "title", "type": "text", "text_value": "Updated"}]}},
    {"op": "add",     "slide_index": 2, "slide": {"layout_id": "content", "content": [{"placeholder_id": "title", "type": "text", "text_value": "New"}]}},
    {"op": "remove",  "slide_index": 1}
  ]
}
```

Operations are applied in order; indices are 0-based. The patch envelope is detected automatically when an input contains an `operations` array.

## Local asset paths

`icon.path`, `image_value.path`, shape-grid `image.path`, and `background.image` accept two convenience expansions before resolution against the input base directory:

- A leading `~/` (or bare `~`) expands to the invoking user's home directory.
- `$VAR` and `${VAR}` expand via the process environment, for allow-listed names only: `HOME`, `BRAND_ASSETS`, and any `JSON2PPTX_*` variable.

So `~/assets/logo.svg` and `$BRAND_ASSETS/logo.svg` resolve as expected instead of being passed literally to the filesystem. An unset **or non-allow-listed** environment variable yields an `ASSET_PATH_ENV_UNSET` finding (with `details.env_variable` naming the var) rather than silently collapsing to an empty path; other variables are never expanded, so their values cannot surface in a diagnostic. Path diagnostics quote only the raw input path, never the expanded or absolute one. Traversal and symlink protections still apply against the expanded path, and when `ALLOWED_IMAGE_PATHS` is configured, `icon.path` (shape-grid and panel icons, and `preview_icon`) must resolve under one of its roots, exactly like image paths. Diagram data icons rendered by svggen (e.g. timeline `items[].icon`) accept only a bundled name, inline SVG, or a `data:` URI (at most 512 KB decoded; raster at most 4096×4096 px). Remote URLs and local file paths there are ignored — use a shape-grid or panel icon, whose `path`/`url` go through the guarded resolvers.
