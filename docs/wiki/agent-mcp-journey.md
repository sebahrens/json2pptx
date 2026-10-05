# Agent journey through the MCP

The path an agent takes from a brief to a reviewed PPTX over `json2pptx mcp`,
with what each response carries and where agents actually lost time. The
sequence below is the default `deckspec` tool profile (13 tools); `--tools
all` adds 35 more and about 200 KB to the first `tools/list` — in four
product-only runs the only extra tool that earned its place was
`show_pattern`, which is why it is now in the default profile.

Back to the [wiki hub](README.md). The contract behind this page is
[`skills/generate-deck/SKILL.md`](../../skills/generate-deck/SKILL.md) and
[`TOOLS.md`](../../skills/generate-deck/TOOLS.md); this page is the walk-through.

## The sequence

```
get_started(task:"brief")                 what to call, render_available, hidden_tools
list_templates(fields:"names")            pick a template (p-style, midnight-blue, …)
list_slide_kinds()                        one line per kind
list_slide_kinds(kinds:[…])               copy-ready example per kind you will use
list_slide_kinds(fields:["brief"], template) title / takeaway budgets measured on your template
recommend_visual(intent, template, preview:true)   only when the visual is unclear
validate_deck_spec(spec, template)        findings at JSON Pointer paths; deck_id
render_deck_spec(deck_id, patch)          pptx_path, deterministic_ready, changed_slides
render_deck_thumbnails(pptx_path)         one image per slide — LOOK at each
validate_deck_spec / render_deck_spec(deck_id, patch)   repair at the finding's path
render_deck_thumbnails(pptx_path, known_hashes)         only changed slides come back
submit_visual_review(pptx_revision, slides[])           the completion step
render_deck_spec(deck_id, template:"other")             the second-template pass
```

Twenty to forty-five calls is normal for a 12-slide deck with one repair round
and a second template; the four journey runs took 23, 28, 27 and 44.

## Step by step

### 1. First contact

`initialize` returns the server instructions (the completion rule), and
`tools/list` is abridged on purpose. Call `get_started` with
`task: "brief"` and the skill's `schema_version` as `skill_version`. Read
`runtime.render_available`: without LibreOffice the deck cannot be reviewed and
must be delivered as **unreviewed**. `get_started(tool: "<name>")` returns any
tool's full description and schema — use it instead of guessing an argument.

### 2. Discover, then write the storyline

`list_slide_kinds()` is the catalogue: 27 kinds, one line each. Call it again
with `kinds: [the kinds you will use]` to get a copy-ready example of each, and
with `fields: ["brief"]` plus your `template` to get the measured title and
takeaway budgets (titles ≤ ~67 characters on the tightest templates, two lines).
Write the **ghost deck** — the action titles alone, in order — before any slide
body ([storyline-and-structure.md](storyline-and-structure.md)).

`plan_deck(format: "deckspec")` drafts slots from a brief. It routes the
slides a brief names on the kind vocabulary (bridge / walk → `bridge`, photo /
screenshot / comparable → `image_case`, schedule / register → `table`, options
with a recommendation → `decision` or `option_matrix`, phases → `roadmap`,
likelihood × impact → `matrix_2x2`, "chart on the left, number on the right"
→ `regions`, one closer), fills the kinds' structured fields from the brief's
sentences, and never ellipsises a title — the four journey briefs went from
13–28 unplaced facts to 0–4 (`go-slide-creator-xbwlt`). Still read
`unplaced_facts[]` and `constraints[]`, replace every `__FILL__`, and check each
slot's kind against [visual-vocabulary.md](visual-vocabulary.md) before
authoring; the draft is a scaffold, not the deck.

### 3. Choose visuals you have not seen

`recommend_visual(intent, template, preview: true)` renders up to four
candidates as images before you author anything — the arch-stack preview
settled an architecture slide on the first attempt, and the comparison preview
showed row alignment. The runs exposed ranking gaps that are now closed
(`go-slide-creator-ux1fl`): "beside / next to / the remaining third" returns
the `regions` kind first with a `compose` form below it, a status board with
limits and breaches ranks `table-highlight` (DeckSpec `option_matrix`, `rag`
scale), "one finding, what we found / why it matters / action / owner" ranks
`labeled-rows`, a share split that sums to 100% gates to pie / donut / stacked
bar, and a `candidates: [...]` shortlist is scored like the open ranking. A
vague intent still ranks noisily: say what the slide shows, with its numbers.
Every candidate carries a `data_contract` and a
runnable `next_tool_call`; copy that slide and replace every `Replace with …`
string — a recipe rendered verbatim is blocked as `SEMANTIC_WEAK_CONTENT`.

### 4. Validate, read the paths, patch

`validate_deck_spec(spec, template)` returns the same findings `render_deck_spec`
would, each at a JSON Pointer `path` into your spec (`/slides/3/kpis/4/value`)
with a 1-based `slide_number`. Fix everything in one pass: the first validate
reports all it can even when a slide does not compile. Three kinds of finding:

- `error` — blocks; `ok: false`. Unknown fields (`SEMANTIC_UNKNOWN_FIELD`:
  content DROPPED, with `did_you_mean`), budgets a visual cannot hold
  (`SEMANTIC_PATTERN_DEGRADED` with `max_chars` / `max_items`), data loss
  (`table_rows_truncated` with `split_at_row`).
- `warning` — renders, but something was assumed (`SEMANTIC_TAKEAWAY_REQUIRED`,
  rhythm runs). Clear them; a deck with warnings is rarely finished.
- `info` — a note (`contrast_predicted`, `table_font_scaled`). Read it, act
  only if the render shows a problem.

A finding with `patch_verified: true` carries a `next_tool_call` whose patch the
server already tried: send it as given. Keep the `deck_id` the first validate
returns and send `deck_id` + `patch` (`[{op, path, value}]`) instead of the whole
spec; the response lists `changed_slides` and `slide_changes[]`. The stored spec
is one call away — `validate_deck_spec {deck_id, read: "spec"}` — so your local
copy never has to drift (one run re-rendered a stale copy for want of this).

### 5. Render, look, repair, look again

`render_deck_spec` writes the PPTX. `success: true` means a file exists;
`deterministic_ready: true` means no blocking finding remains; `publishable`
stays false until a visual review is recorded. Then `render_deck_thumbnails
{pptx_path}` returns one JPEG per slide (about 400 KB for 11 slides at the
default density); open every one. For 12pt text send the response's
`larger_render` (one slide at `density: 100`). Score each slide against the
ten-point rubric in [quality-and-review.md](quality-and-review.md).

Repair at the finding's path (or the field the rubric names), re-render with
`deck_id` + `patch`, and call `render_deck_thumbnails` again with the
`known_hashes` the render's `next_tool_call` pre-fills: unchanged slides come
back as `{unchanged: true}` with their `path` and `content_hash` but no image,
so you re-inspect only what moved. `changed_slides` names the slides that
render differently: a footer edit in `meta.chrome` leaves out a cover that
prints no footer. If you keep the deck in a file and send the whole revised
spec instead of a patch, give every slide an `id`: a spec with the title,
template and slide ids of a stored deck is that `deck_id`'s next revision, so
the diff and `known_hashes` work the same (`fork: true` starts a new deck).
Stop after three repair rounds and hand back open findings per slide rather
than loop.

### 6. Submit, then the second template

`submit_visual_review {pptx_revision: <content_hash>, slides: [{index,
image_sha256 | path, verdict, findings[]}]}` must cover every slide of the
current revision with the image you were sent for it; `status:
"visually_reviewed_current_revision"` is the completion signal. Then render the
same `deck_id` with `template: "p-style"` (or the client's `template_path` via
`examine_template` first): the response forks a new `deck_id`; look at every
slide again — titles that fit on one template wrap on another, and chrome that
fit may truncate. Do not restyle content for the second template; fix the
spec so it holds on both.

## Where the runs lost time (and what to do instead)

| Friction | What happened | Do this |
|---|---|---|
| Table refused at 8 rows | The old 7-row cap refused tables that render cleanly and forced two half-empty slides | Fixed: up to 10 logical rows (header included) render, one-line rows past 7 at a compact pitch. Split only at `table_rows_truncated` or when the slide reads dense. |
| `regions` kind list | The unknown-region-kind error listed slide kinds, so `bridge` looked available | Region kinds are `chart / stat / kpis / table / timeline / image / text`. A bridge beside text is a `chart` region with `type: waterfall` ([split-and-complex-layouts.md](split-and-complex-layouts.md)); `recommend_visual` now says so for "beside" intents. |
| Parallel tracks on a roadmap | `roadmap` had no `parallel_tracks`; agents dropped to a raw `roadmap-phased` slide | Fixed: `roadmap.parallel_tracks` (0–4) and `parallel_label` (`go-slide-creator-ptazs`); [playbook-risk-consulting.md](playbook-risk-consulting.md) uses them. |
| Before / after with connectors | `comparison` could not pass `connectors` / `highlight_column` | Fixed: `comparison.connectors`, `highlight_column`, `highlight_row`; [playbook-technology-and-data.md](playbook-technology-and-data.md) uses them. The screenshot's `image_case.image_width_pct` landed in the same change. |
| Footer client name truncated | `meta.chrome.client` plus `project_code` plus date overflowed the footer silently on 11 slides | Keep the client label short, drop `project_code` on narrow templates, and look at the footer on the second template (`go-slide-creator-m2tlt`). |
| Waterfall axis started at 22 | The opening total drew as a stub | Fixed: non-negative bars, lines and waterfalls start at zero; `data.y_min` / `y_max` zoom deliberately (`go-slide-creator-929jm`). |
| `--tools all` first contact | 224 KB, 48 tools, 11 used | Stay on the default profile; `get_started(tool:"<hidden tool>")` reaches any hidden tool by name. |
| Thumbnail bytes | ~2 MB of images across a run | Render all once, then `known_hashes`; `density: 100` only for the slide you cannot read. |
| Info notes in strict reviews | `table_font_scaled`, `contrast_predicted` are notes | Act on them only when the image shows a problem; a strict standard splits the table instead of accepting 11pt. |

## What worked, so keep doing it

- `patch_verified` fixes cleared their finding on the next call every time.
- `SEMANTIC_UNKNOWN_FIELD` named the accepted keys and said the content was
  dropped — the cheapest error in the product.
- `regions` with `arrangement: main_left` gave chart-left / number-and-bullets-
  right on the first attempt in all four runs.
- `option_matrix` with a `rag` criterion rendered a real status board with
  coloured dots and survived both templates.
- `section` with `appendix: true` produced an unnumbered divider and A1-style
  page numbers without further instruction.
- Per-slide `content_hash` proved exactly which slides a patch changed.
