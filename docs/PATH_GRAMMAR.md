# Path Grammar

Authored-content paths use **JSON Pointer** (RFC 6901) notation with `/` as the separator and numeric array indices. `repair_slide` also accepts an older placeholder-name selector; that selector is not a JSON Pointer into the authored deck.

## Format

A finding's `path` is a JSON Pointer into the deck the author sent: it starts with `/`, descends through the JSON structure with `/` separators only (never `.` or `[n]`), references array elements by 0-based index, and resolves in that deck. A slide's content starts with `/slides/{index}`; a deck-level field is `/{field}`.

```
/slides/{slideIdx}/...
/{deckField}/...
```

`TestVerdictParityMutationCorpus`, `TestExampleCorpusFindingPathsAreAuthoredPointers` and `TestFaultyDeckFindingPaths` (`cmd/json2pptx`) hold every finding of the raw-deck surfaces to this rule; the only paths that do not resolve are the forms under [Paths that name something the deck does not contain](#paths-that-name-something-the-deck-does-not-contain).

A path that is not a pointer is a tool argument, not a place in the deck: `presentation` (a deck the tool could not read), `base_dir`, `deck_id`, `patch[0].path`, and `template` when the caller passed the tool's own `template` argument.

## Path Examples

### Placeholder content (by name, `repair_slide` selector only)

```
/slides/0/content/body
/slides/1/content/title
/slides/2/content/subtitle
```

### Content array (by index)

```
/slides/0/content/0
/slides/0/content/1/placeholder_id
/slides/0/content/1/diagram_value
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

A table's fields sit under the object that holds the table: `table_value` for a placeholder table (`value` in the legacy form), `table` for a `shape_grid` cell.

```
/slides/0/content/1/table_value/headers/2                        -- header cell
/slides/0/content/1/table_value/rows/3/1                         -- data cell [row][col]
/slides/0/content/1/table_value/rows                             -- the rows (a row wider than the headers)
/slides/0/content/1/table_value/style/style_id                   -- a style field
/slides/0/content/1/table_value/rows/3/1/conditional/rule        -- a cell's conditional format
/slides/0/shape_grid/rows/1/cells/2/table/rows/3/1/conditional/rule
/slides/0/content/1                                              -- the table as a whole (density, style_collision)
```

### Deck-level fields

```
/template                          -- also /template_path, /slides, /structure, /grid
/viewing_mode                      -- an enum outside its set; also /design_mode, /accent_strategy, /type_scale
/theme_override/colors/accent1
/chrome/page_numbers/format        -- an unknown key is reported at /{object}/{key}
/defaults/table_style/style_id     -- a default every table adopts is reported here once, not at each table
/defaults/cell_style/fill
```

A value a table or cell took from `defaults` (it wrote none of its own) is the deck's field, so the finding names `/defaults/...`. A value the table or cell wrote itself is reported at the table or cell.

## Paths that name something the deck does not contain

Three forms do not resolve. Each is still a JSON Pointer, and each resolves up to the parent stated here.

| Form | Example | Resolves up to |
|---|---|---|
| A field to add: `required`, `REQUIRED`, `takeaway_missing`, `DATA_WITHOUT_SOURCE`, and `INVALID_PARAMETER` for a content block with no value | `/slides/1/layout_id`, `/slides/2/takeaway`, `/slides/2/source`, `/template`, `/slides/1/content/1/text_value` | the object the field belongs in |
| A cell of the grid a pattern or `compose` expands to | `/slides/6/pattern/rows/0/cells/0/shape/text`, `/slides/3/shape_grid/rows/0/cells/1/grid/rows/1/cells/0/diagram` on a `compose` slide | the `pattern` object (edit its `values`; `repair_slide` maps a pattern cell back to the value that produced it), or the slide that authors `compose` |
| Something written at render time | `/slides/5/chrome` (footer / page-number chrome on that slide), `/slides/1/rendered_shapes/200/paragraphs/0` (a written shape) | the slide |

Slide indices are those of the expanded deck: a `split_slide` entry counts once per page and a `structure` block is counted as the slides it builds, so on such a deck `/slides/N` is the N-th rendered slide, not the N-th entry the author wrote (go-slide-creator-9564b, go-slide-creator-2v8me).

## Use in Fit Findings

Every `FitFinding` emits a `path` field. Placeholder, chart, diagram, table, and text-fit findings use authored-content JSON Pointers; the by-name form above remains an accepted `repair_slide` selector for older callers. The path identifies the offending element so that agents can:

1. Map a finding back to the JSON input element
2. Pass the path to `repair_slide` as the `path` parameter for disambiguation

Examples from finding codes:

| Finding Code | Example Path |
|---|---|
| `placeholder_overflow` | `/slides/0/content/1` |
| `title_wraps` | `/slides/1/content/0` |
| `slide_bounds_overflow` | `/slides/2/shape_grid/rows/1/cells/0` |
| `footer_collision` | `/slides/3/shape_grid/rows/2/cells/0` |
| `sparse_layout` | `/slides/1/shape_grid` |
| `fit_overflow` | `/slides/0/content/0/table_value/rows/3/1` |
| `density_exceeded` | `/slides/0/content/0` |
| `contrast_autofixed` | `/slides/3/shape_grid/rows/0/cells/2/shape/text` — the authored element, as `contrast_predicted` names it (layout/run text uses the slide-level `/slides/1`) |

## Use in repair_slide

All `repair_slide` fix kinds accept an optional `path` parameter (JSON Pointer) to disambiguate which content element to target. When omitted, the fix applies to the first matching element on the slide.

```json
{
  "kind": "reduce_text",
  "params": {
    "path": "/slides/0/content/1",
    "max_items": 5
  }
}
```

The `path` can be copied directly from a fit finding's `path` field for round-trip usage:

```
fit finding: { "path": "/slides/0/content/1", "code": "placeholder_overflow", "fix": {"kind": "reduce_text"} }
                                                                                          |
repair_slide fix: { "kind": "reduce_text", "params": { "path": "/slides/0/content/1", "max_items": 5 } }
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
| `Content(si, phID)` | `/slides/{si}/content/{phID}` (legacy by-name selector, not an authored JSON Pointer) |
| `ContentIndex(si, ci)` | `/slides/{si}/content/{ci}` |
| `ContentField(si, ci, f)` | `/slides/{si}/content/{ci}/{f}` |
| `SlideField(si, f)` | `/slides/{si}/{f}` |
| `ShapeGrid(si)` | `/slides/{si}/shape_grid` |
| `GridCell(si, ri, ci)` | `/slides/{si}/shape_grid/rows/{ri}/cells/{ci}` |
| `GridCellField(si, ri, ci, f)` | `/slides/{si}/shape_grid/rows/{ri}/cells/{ci}/{f}` |
| `GridRow(si, ri)` | `/slides/{si}/shape_grid/rows/{ri}` |
| `GridRowRange(si, s, e)` | `/slides/{si}/shape_grid/rows/{s}:{e}` |
| `TableHeader(prefix, hi)` | `{prefix}/headers/{hi}` — `prefix` is the table object (`…/table_value`, `…/cells/{c}/table`) |
| `TableCell(prefix, ri, ci)` | `{prefix}/rows/{ri}/{ci}` — same prefix |
| `Join(prefix, suffix)` | `{prefix}/{suffix}` |
| `Field(prefix, field)` | Converts dotted/indexed svggen fields (e.g. `data.series[0].values`) into escaped JSON Pointer segments; an unindexed wildcard stops at its array |
