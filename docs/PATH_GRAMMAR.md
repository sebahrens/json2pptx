# Path Grammar

All paths in fit findings, validation errors, and `repair_slide` targeting use **JSON Pointer** (RFC 6901) notation with `/` as the separator and numeric array indices.

## Format

Every path starts with `/slides/{index}` and descends through the JSON structure using `/` separators. Array elements are referenced by their 0-based numeric index.

```
/slides/{slideIdx}/...
```

## Path Examples

### Placeholder content (by name)

```
/slides/0/content/body
/slides/1/content/title
/slides/2/content/subtitle
```

### Content array (by index)

```
/slides/0/content/0
/slides/0/content/1/placeholder_id
```

### Slide-level fields

```
/slides/0/layout_id
/slides/0/transition
/slides/0/transition_speed
/slides/0/build
/slides/0/background/fit
```

### Shape grid

```
/slides/0/shape_grid                               -- grid root
/slides/0/shape_grid/rows/1/cells/2                -- specific cell
/slides/0/shape_grid/rows/1/cells/2/shape/fill     -- cell shape fill
/slides/0/shape_grid/rows/1/cells/2/table          -- embedded table
/slides/0/shape_grid/rows/1/cells/2/shape/text     -- shape text
/slides/0/shape_grid/rows/2                        -- row
/slides/0/shape_grid/rows/1:2                      -- row range
```

### Tables

```
/slides/0/content/0/headers/2                      -- header cell
/slides/0/content/0/rows/3/1                       -- data cell [row][col]
```

## Use in Fit Findings

Every `FitFinding` emits a `path` field using this grammar. The path identifies the offending element so that agents can:

1. Map a finding back to the JSON input element
2. Pass the path to `repair_slide` as the `path` parameter for disambiguation

Examples from finding codes:

| Finding Code | Example Path |
|---|---|
| `placeholder_overflow` | `/slides/0/content/body` |
| `title_wraps` | `/slides/1/content/title` |
| `slide_bounds_overflow` | `/slides/2/shape_grid/rows/1/cells/0` |
| `footer_collision` | `/slides/3/shape_grid/rows/2/cells/0` |
| `sparse_layout` | `/slides/1/shape_grid` |
| `fit_overflow` | `/slides/0/content/0/rows/3/1` |
| `density_exceeded` | `/slides/0/content/0` |
| `contrast_autofixed` | `/slides/1/content/body` |

## Use in repair_slide

All `repair_slide` fix kinds accept an optional `path` parameter (JSON Pointer) to disambiguate which content element to target. When omitted, the fix applies to the first matching element on the slide.

```json
{
  "kind": "reduce_text",
  "params": {
    "path": "/slides/0/content/body",
    "max_items": 5
  }
}
```

The `path` can be copied directly from a fit finding's `path` field for round-trip usage:

```
fit finding: { "path": "/slides/0/content/body", "code": "placeholder_overflow", "fix": {"kind": "reduce_text"} }
                                                                                          |
repair_slide fix: { "kind": "reduce_text", "params": { "path": "/slides/0/content/body", "max_items": 5 } }
```

### Path matching rules

- Match by placeholder name: `/slides/0/content/body`
- Match by content array index: `/slides/0/content/1`
- Sub-paths also match: `/slides/0/content/body/text` matches content with `placeholder_id: "body"`

## Extracting the Slide Index

The slide index is always the second path segment:

```
/slides/0/content/body
        ^
        slide index = 0
```

Use `slidepath.SlideIndex(path)` in Go code to extract it. Returns -1 for invalid paths.

## Implementation

All path construction is centralized in `internal/slidepath/`. Available builders:

| Function | Output |
|---|---|
| `Slide(idx)` | `/slides/{idx}` |
| `Content(si, phID)` | `/slides/{si}/content/{phID}` |
| `ContentIndex(si, ci)` | `/slides/{si}/content/{ci}` |
| `ContentField(si, ci, f)` | `/slides/{si}/content/{ci}/{f}` |
| `SlideField(si, f)` | `/slides/{si}/{f}` |
| `ShapeGrid(si)` | `/slides/{si}/shape_grid` |
| `GridCell(si, ri, ci)` | `/slides/{si}/shape_grid/rows/{ri}/cells/{ci}` |
| `GridCellField(si, ri, ci, f)` | `/slides/{si}/shape_grid/rows/{ri}/cells/{ci}/{f}` |
| `GridRow(si, ri)` | `/slides/{si}/shape_grid/rows/{ri}` |
| `GridRowRange(si, s, e)` | `/slides/{si}/shape_grid/rows/{s}:{e}` |
| `TableHeader(prefix, hi)` | `{prefix}/headers/{hi}` |
| `TableCell(prefix, ri, ci)` | `{prefix}/rows/{ri}/{ci}` |
| `Join(prefix, suffix)` | `{prefix}/{suffix}` |
