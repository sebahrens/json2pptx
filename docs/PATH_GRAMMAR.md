# Path Grammar

Authored-content paths use **JSON Pointer** (RFC 6901) notation with `/` as the separator and numeric array indices. `repair_slide` also accepts an older placeholder-name selector; that selector is not a JSON Pointer into the authored deck.

## Format

A finding's `path` is a JSON Pointer into the deck the author sent: it starts with `/`, descends through the JSON structure with `/` separators only (never `.` or `[n]`), references array elements by 0-based index, and resolves in that deck. A slide's content starts with `/slides/{index}`; a deck-level field is `/{field}`.

```
/slides/{slideIdx}/...
/{deckField}/...
```

`TestVerdictParityMutationCorpus`, `TestExampleCorpusFindingPathsAreAuthoredPointers`, `TestFaultyDeckFindingPaths` and `TestExpandedDeckFindingsAreAuthored` (`cmd/json2pptx`) hold every finding of the raw-deck surfaces to this rule, for a deck with `slides`, with `split_slide` entries and with a `structure` block. The one path that does not resolve names [a field to add](#a-field-to-add), and its parent does.

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

## A field to add

A finding about a field the deck does not have names the pointer the field will have; the object it belongs in resolves. This is the one form that does not resolve: `required`, `REQUIRED`, `takeaway_missing`, `DATA_WITHOUT_SOURCE`, and `INVALID_PARAMETER` for a content block with no value.

```
/template                          -- REQUIRED
/slides/1/layout_id                -- required
/slides/2/takeaway                 -- takeaway_missing
/slides/2/source                   -- DATA_WITHOUT_SOURCE
/slides/1/content/1/text_value     -- INVALID_PARAMETER: the block has no value
```

## The authored deck and the engine's deck

The engine checks a deck it has expanded: a flat slide list, the shape grid a pattern or a `compose` envelope becomes, the chrome a template draws, the shapes a slide is written as. A finding is reported where the author wrote the thing, never where the engine holds it. Every raw-deck surface passes its findings through one translation (`authoredPaths`, `cmd/json2pptx/authored_paths.go`) on the way out; no check knows about it.

| The engine addresses | The finding's `path` |
|---|---|
| A page of a `split_slide` (`/slides/3/content/0` for page 3 of the entry at `/slides/1`) | The envelope's base: `/slides/1/base/content/0`. A table row is its row in the authored table: page 3, row 0 of pages of three is `/slides/1/base/content/1/table_value/rows/6`. A finding every page repeats (the title, the layout) is reported once. |
| A slide after a `split_slide` (`/slides/4`) | The entry the author wrote: `/slides/2`. |
| A slide of a `structure` deck (`/slides/3/content/1`) | `/structure/cover/…`, `/structure/closing/…`, `/structure/sections/0/slides/0/content/1`. |
| A section divider the engine built | Its title is `/structure/sections/0/title`; anything else about the slide is the section, `/structure/sections/0`. |
| The agenda slide the engine built | An item is the section title it lists, `/structure/sections/1/title`; anything else is `/structure/auto_agenda`. |
| A cell of the grid a pattern expands to (`/slides/6/pattern/rows/0/cells/2/shape/text`) | The pattern value the cell shows: `/slides/6/pattern/values/2/small` for one paragraph of its text, `/slides/6/pattern/values/2` for a cell that shows several fields of one value. A row, a connector or text the pattern composes is the pattern, `/slides/6/pattern`. The same holds for a pattern in a `shape_grid` cell or a `compose` segment. |
| The grid a `compose` envelope expands to (`/slides/3/shape_grid/rows/1/cells/0/grid/…`) | The segment that owns the cell and, inside it, its pattern's value, its diagram or its nested envelope: `/slides/3/compose/segments/1/pattern/values/steps/0/label`, `/slides/3/compose/segments/1/compose/segments/0/diagram`. The banner and callout bands are `/slides/3/compose/banner` and `/slides/3/compose/callout`; the grid as a whole is `/slides/3/compose`. |
| Footer or page-number chrome on a slide (`/slides/5/chrome`) | The slide, `/slides/5`: the template draws the chrome, the deck's `chrome` block configures it for every slide. |
| A written shape (`/slides/1/rendered_shapes/200/paragraphs/0`) | The element that produced it when the generator knows — a grid cell's `/slides/1/shape_grid/rows/0/cells/0/shape/text`, a pattern value — otherwise the slide, `/slides/1`. |
| A slide left after `generate --partial` dropped one before it | The slide the author wrote; the indices do not move up. |

Beside `path` a finding on a slide carries:

- **`slide_number`** — the 1-based number of the *rendered* slide: page 3 of a split is its own slide, a `structure` deck's agenda and dividers count. It matches a thumbnail. `where.slide` is the same slide, 0-based. A slide `--partial` dropped has none.
- **`debug.locator`** — the engine's own path, when it differs from `path`. Nothing outside `debug` names an object the author did not write.

**Slide indices in tool arguments are rendered indices.** `repair_slide.slide_index`, `render_slide_image.slide_index`, `slide_indices` and a finding's `next_tool_call.args_template.slide_index` count the slides of the expanded deck: `slide_number - 1`. On a deck without `split_slide`, `structure` or dropped slides that is the index in `path`; on one with them it is not, and `slide_number` is the value to use.

**A tool that takes a path back accepts both forms.** `repair_slide` / `repair_slides_batch` `params.path` and `params.cell_path`, and the findings handed to `propose_repairs`, may carry the authored pointer a finding reports or the engine's locator (`debug.locator`, or the `/slides/{slide_index}/…` form older callers send). `reduce_cell_text` takes a pattern value's pointer (`/slides/2/pattern/values/2/small`, `/slides/3/compose/segments/1/pattern/values/steps/0/label`) and shortens that value. A `fix.params` or `next_tool_call` path is the authored pointer whenever that names the same element; where the authored address is only the enclosing object (a pattern for a cell whose text it composes) the argument stays the engine's locator, which the tool can act on.

`repair_slide`, `repair_slides_batch` and `propose_repairs` work on the expanded deck and return it as `patched_deck` (or embed it in a planned call): a `split_slide` entry has become its pages, and a `structure` block the cover, agenda, dividers and slides it renders, with the block itself dropped. The findings in the answer address that deck.

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
| `sparse_layout` | `/slides/1/shape_grid` (`/slides/1/pattern`, `/slides/1/compose` on a pattern or compose slide) |
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

The `path` can be copied directly from a fit finding's `path` field for round-trip usage, with `slide_index` set to the finding's `slide_number - 1` (see [The authored deck and the engine's deck](#the-authored-deck-and-the-engines-deck)):

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

Inside the engine the slide index is the second path segment:

```
/slides/0/content/body
        ^
        slide index = 0
```

Use `slidepath.SlideIndex(path)` in Go code to extract it. Returns -1 for invalid paths.

A reader of a finding uses `slide_number` instead: the authored path of a `structure` slide has no `/slides` index, and the one in a `split_slide` deck's path is the authored entry, not the rendered slide.

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
