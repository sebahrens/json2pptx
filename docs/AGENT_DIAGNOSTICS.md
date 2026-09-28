# Agent Diagnostics — the Finding Envelope

This document defines the unified, machine-readable contract that every
diagnostic-bearing json2pptx surface uses to report problems to an agent: the
**Finding Envelope**. It is the single shape an agent parses regardless of which
command, MCP tool, or HTTP endpoint produced it.

The Go types live in [`internal/diagnostics/envelope.go`](../internal/diagnostics/envelope.go);
the wire schema is committed at
[`docs/api/finding-envelope.schema.json`](api/finding-envelope.schema.json).

## 1. Why one envelope

Before this contract, each surface returned its own shape: `dryRunOutput`
carried `diagnostics[]`, the MCP error path carried `{diagnostics[], summary}`,
`repair` carried `applied_fixes[]` and `new_findings[]`, and HTTP errors used a
separate `{success, error{code,message,details}}` body. An agent had to special-
case every command. The Finding Envelope replaces those ad-hoc shapes with one
contract so an agent can:

- branch on a single `ok` boolean,
- correlate the response with its request via `input_sha256`,
- read every issue from one `findings[]` array with stable field names,
- pick a repair path from `remediation.primary` / `remediation.alternatives`,
- resolve any unfamiliar `code` with the executable `describe_command`.

The existing transport-neutral `diagnostics.Diagnostic` type and its `From*`
converters remain the **adapter input**: callers build envelopes from
`[]Diagnostic` via `diagnostics.BuildEnvelope`, never by hand-assembling
`Finding` values. This keeps a single conversion point from the legacy shapes
(`patterns.ValidationError`, `patterns.FitFinding`, joined errors).

### Semantic deck-spec diagnostics

The semantic compiler uses the same envelope. Semantic validation and render
surfaces (`json2pptx semantic validate`, `json2pptx semantic render`,
`validate_deck_spec`, `render_deck_spec`, and the semantic HTTP endpoints) emit
findings whose `evidence.path` points to the semantic authoring field whenever
possible. The semantic family is `INPUT`-namespaced and declared in
`internal/diagnostics/codes.go`: the per-spec gates `SEMANTIC_REQUIRED`,
`SEMANTIC_UNKNOWN_KIND`, `SEMANTIC_UNKNOWN_FIELD`, `SEMANTIC_UNKNOWN_ARCHETYPE`,
`SEMANTIC_TAKEAWAY_REQUIRED`, `SEMANTIC_DENSITY`, `SEMANTIC_WEAK_CONTENT`,
`SEMANTIC_FIELD_TYPE`, plus
the deck-rhythm advisories `SEMANTIC_RHYTHM_MONOTONY`, `SEMANTIC_RHYTHM_DENSITY`,
`SEMANTIC_RHYTHM_SECTIONING`, and `SEMANTIC_RHYTHM_SYNTHESIS`. Each resolves via
`json2pptx describe-finding <code>` like any other code. For example:

```json
{
  "code": "INPUT.SEMANTIC_DENSITY",
  "severity": "error",
  "where": {"slide": 2},
  "message": "kpi snapshot has 9 usable KPIs; 2–6 is recommended",
  "evidence": {"path": "slides[2].kpis"},
  "describe_command": "json2pptx describe-finding SEMANTIC_DENSITY"
}
```

(`SEMANTIC_DENSITY` and the other advisory codes are `info`/`warning` by
default and become `error` under `strict`; they carry no `remediation` of their
own.) Content-bearing list fields are counted **after** the compiler's own
trimming/extraction, not by raw entry count: a required list whose entries are
all blank or labelless (e.g. `steps: ["", " "]`, KPI cells with neither a number
nor a caption, columns with no header or items) clears the raw presence gate but
compiles to a title-only slide, so validation emits a blocking `SEMANTIC_REQUIRED`
at the field path — always an error regardless of strictness, since it is a
missing-content condition — instead of letting the body silently drop. A
payload field present with the wrong JSON type for its kind (a numeric/boolean
title, `points`/`steps`/`columns` given as a scalar instead of an array) is
silently dropped by the per-kind compilers, so validation emits a
`SEMANTIC_FIELD_TYPE` advisory at the field path naming the expected type —
`warning` under `warn`, `error` under `strict` — rather than shipping the
content-less slide behind a green gate. The
`SEMANTIC_DENSITY` range checks (process 3–8 steps, roadmap 3–6 phases, kpi 2–6
KPIs, comparison 1–10 balanced rows per column, chart_insight 1–6 insights)
likewise use the usable count, so validation and compile agree on whether a visual
pattern will be emitted — a balanced comparison whose columns exceed the 10-row cap,
or a chart_insight with more than 6 insights, degrades to a native layout (the latter
to a `two-column` slide: chart in `body`, the full insight list in `body_2`), so
validation flags the over-cap count rather than passing the raw shape. After compilation, raw validation/fit/output findings are mapped back
through the semantic source map. For example, a raw overflow at
`/slides/2/shape_grid/rows/0/cells/1/shape/text/content` is reported to agents
as `slides[1].kpis[1]` with the raw path preserved only as fallback evidence
(`evidence.raw_path`) when useful, and — for the common density/overflow
failures — a `recommended_edit` (`shorten_text`, `split_slide`, `reduce_items`,
`simplify_side`, or `split_phases`) naming the semantic edit that resolves it.
A **post-compile raw preflight** runs the renderer's pattern-validation gate over
the lowered deck inside `Compile`, so a lowered pattern that would be rejected at
render (e.g. a KPI value too long for the `kpi-Nup` cards) surfaces as a blocking
finding from `compile_deck_spec`/`render_deck_spec` — mapped to the semantic path
with the same `evidence.raw_path` and `recommended_edit` — rather than only at
render time. Agents should edit the semantic spec first; compiled raw JSON is an
escape hatch for advanced repairs.

## 2. The envelope contract

### 2.1 Top-level envelope

| Field            | Type        | Required | Meaning                                                       |
| ---------------- | ----------- | -------- | ------------------------------------------------------------ |
| `schema_version` | string      | yes      | Wire version. Currently `"1.0"`.                             |
| `tool`           | string      | yes      | Producing tool, e.g. `"json2pptx"`.                          |
| `subcommand`     | string      | yes      | Surface that produced it, e.g. `"validate"`.                 |
| `input_sha256`   | string      | no       | Hex SHA-256 of the request payload, for correlation.        |
| `template`       | string      | no       | Template the run targeted, when applicable.                  |
| `ok`             | boolean     | yes      | `true` when no finding has `error` severity.                |
| `summary`        | string      | yes      | Short roll-up, e.g. `"2 errors, 1 warning"`.                |
| `findings`       | `Finding[]` | yes      | The issues; may be empty.                                    |

### 2.2 Finding

| Field              | Type          | Required | Meaning                                                              |
| ------------------ | ------------- | -------- | ------------------------------------------------------------------- |
| `id`               | string        | yes      | Unique within the envelope, e.g. `"fit-1"`.                         |
| `code`             | string        | yes      | Dotted, namespaced code, e.g. `"FIT.placeholder_overflow"`.        |
| `severity`         | enum          | yes      | `error` \| `warning` \| `info`.                                    |
| `category`         | enum          | yes      | The namespace prefix of `code` (see §2.4).                          |
| `where`            | `Where`       | no       | Location in the deck/template (see §2.3).                           |
| `message`          | string        | yes      | Human-readable description.                                         |
| `evidence`         | object        | no       | Numeric/enum facts only — never prose.                             |
| `remediation`      | `Remediation` | no       | Structured repair (see §2.5).                                      |
| `next_tool_call`   | object        | no       | Replayable tool-call hop to recover/investigate: `{tool, args_template}`. |
| `example_value`    | any           | no       | Representative valid value for the offending argument/field.        |
| `doc_url`          | string        | no       | Human documentation for the code.                                  |
| `describe_command` | string        | no       | Executable lookup, `json2pptx describe-finding <code>`.            |

`evidence` carries only machine-actionable facts: measured-vs-allowed extents,
overflow ratios, the offending JSON `path`, the `expected_type`, the fit
`action`, etc. Free-form text and arbitrary nested objects are dropped during
adaptation so an agent can rely on the map being parseable facts.

`next_tool_call` and `example_value` are carried verbatim from the source
`Diagnostic` so the adapter loses no agent-recovery information: `next_tool_call`
names a tool an agent can replay to recover (it differs from `remediation`,
which describes *what to change* rather than *which tool to call*), and
`example_value` is a representative valid value that may be a scalar or a nested
object — which is why it is a dedicated field rather than an `evidence` entry.

### 2.3 Where

All fields optional; an all-empty `where` is omitted entirely.

| Field              | Type    | Meaning                                            |
| ------------------ | ------- | -------------------------------------------------- |
| `slide`            | integer | 0-based slide index.                               |
| `slide_id`         | string  | Stable slide identifier when the deck supplies one.|
| `layout_id`        | string  | Template layout the slide resolved to.             |
| `layout_role`      | string  | Canonical role of that layout.                     |
| `placeholder_id`   | string  | Offending placeholder's portable id.               |
| `placeholder_role` | string  | Canonical role of that placeholder.                |

When a finding originates from input validation, the slide index is recovered
best-effort from the JSON `path` (e.g. `slides[2].content.body` → `slide: 2`).
Richer location fields are populated by surfaces that know the resolved layout
and placeholder roles.

### 2.4 Code namespaces

Every `code` is `"<NAMESPACE>.<legacy-code>"`, and `category` equals the
namespace. The legacy (un-prefixed) code is what `describe_command` passes to
`describe-finding`, so the command stays runnable against the registry.
`describe-finding` also accepts the dotted namespaced form directly (it strips a
leading known-namespace prefix before lookup), and the CLI takes the code either
positionally (`json2pptx describe-finding <code>`) or via `-code`. The
`describe_finding` lookup is the single read surface for code metadata: it
resolves every code in this section — the lowercase fit/pattern codes and dotted
`chart.*` codes from `internal/patterns`, plus every `SCREAMING_SNAKE` code
declared in `internal/diagnostics/codes.go` (`MISSING_PARAMETER`, `UNKNOWN_PARAMETER`,
`TEMPLATE_NOT_FOUND`, `RENDER_FAILED`, `INTERNAL`, …) backed by the registry in
`internal/diagnostics/describe.go`. `TestDescribeCoversAllDiagnosticCodes` fails
CI when a declared code has no describe entry.

| Namespace | Covers                                                  |
| --------- | ------------------------------------------------------ |
| `TPL`     | Template structure / metadata problems.                |
| `FIT`     | Content overflow / density diagnostics.                |
| `GRID`    | Shape-grid / pattern layout problems.                  |
| `RENDER`  | Generation / rendering / media failures.               |
| `POLICY`  | Content-policy violations (e.g. emoji).                |
| `INPUT`   | Request / JSON-payload problems.                       |

`diagnostics.ClassifyCode` maps a legacy code to its namespace: codes declared
in `internal/diagnostics/codes.go` are looked up directly, and the lowercase
fit/pattern codes (`placeholder_overflow`, `accent_overload`, …) and dotted
`chart.*` codes are classified by heuristic.

### 2.5 Remediation and the action vocabulary

`remediation` carries a `primary` action plus ranked `alternatives`, so an agent
chooses a repair path rather than receiving a single take-it-or-leave-it fix.
Each `RemediationAction` has an `action` from the fixed vocabulary and an
`action`-specific `params` object:

| Action                | Use                                                        |
| --------------------- | --------------------------------------------------------- |
| `shorten_text`        | Trim text to fit a budget.                                |
| `replace_value`       | Supply / replace a field value (incl. enum corrections).  |
| `apply_patch`         | Apply a structured deck patch.                            |
| `switch_layout`       | Move the slide to a different layout.                     |
| `split_slide`         | Split content across slides.                              |
| `move_to_placeholder` | Move content to a different placeholder.                  |
| `remove_emoji`        | Strip emoji per content policy.                           |
| `regenerate_pattern`  | Re-expand the pattern with corrected values.              |

Legacy `Fix.Kind` values are mapped onto this vocabulary by
`diagnostics.mapFixKindToAction`; the original kind is preserved in `params`
when it is not already an action verb.

## 3. Adoption status

The shared contract — types, action vocabulary, namespace prefixes, JSON
schema, and the `BuildEnvelope` adapter over `[]diagnostics.Diagnostic` — is the
foundation that every diagnostic-bearing surface adopts. The per-command and
per-tool wire migration runs as a separate phase because each step is a breaking
change to an existing response contract.

`repair_slide` and `repair_slides_batch` have migrated: their residual post-patch
fit findings ship as a `FindingEnvelope` under the `findings` key (replacing the
legacy `new_findings []FitFinding` array), always present so an agent can branch
on `findings.ok`.

`validate_input` / CLI `validate` and `generate -dry-run` have migrated too: the
success-path response collapses the legacy `warnings[]`, `validation_warnings[]`,
`errors[]`, `diagnostics[]`, and `fit_findings[]` arrays into a single
`FindingEnvelope` under the `findings` key (built from the boundary + slide
diagnostics plus the fit-report findings, with fit findings carrying category
`FIT`). The structural fields (`valid`, the `*_count` totals, `slides[]`,
`response_fingerprint`) are unchanged. Note that `findings.ok` reflects
finding severity (it is `false` when any error-severity finding, including a
`refuse`-action fit finding, is present), which can legitimately differ from the
structural `valid` flag.

**When `IsError` is set.** A tool asked to PRODUCE something (`render_deck_spec`,
`compile_deck_spec`) sets `IsError` whenever its payload reports `ok: false` /
`success: false`, including domain failures like a template that does not
resolve or a spec that does not parse — `isError` is the only protocol-level
failure signal, and leaving it absent let a harness treat an unwritten deck as
done (go-slide-creator-swak). A tool asked to ASSESS (`validate_deck_spec`,
`validate_input`, `validate_pattern`) reports an invalid deck as a *successful*
call with `ok: false`: the verdict is the product, not a failure of the call.
A render that succeeded but produced an unshippable deck is neither — it returns
`success: true` with `publishable: false` and `blocking_reasons[]`.

The MCP **error** envelope has migrated: every tool error result
(`IsError=true`) — arg-validation, template/asset resolution, strict-fit
refusal, slide-level validation, and a failing `validate_input` — now carries a
`FindingEnvelope` as `StructuredContent` (and as the text fallback), replacing
the legacy `{diagnostics, summary}` shape. `MCPDiagnosticsError` /
`MCPSimpleError` still take `[]diagnostics.Diagnostic` (no call-site churn) and
build the envelope via `BuildEnvelope`; the error envelope is stamped with the
generic `subcommand: "mcp"` because the shared builders do not plumb a per-tool
name. The lossless agent-recovery fields (`next_tool_call`, `expected_type`,
`example_value`) survive, and the adapter now also carries **list facts** (e.g.
icon-name `suggestions`, allowed enum values) in `evidence` — only arbitrary
nested objects are dropped.

`inspect` (the `inspect_slide_images` MCP tool and the `inspect` CLI subcommand)
has migrated: the response keeps the `visualqa.Report` rollups (`mode`,
`results[]`, `total_p0..p3`, per-finding `suggested_fixes[]`) at the top level and
adds a `FindingEnvelope` under the `findings` key, projecting every per-slide
visual finding into the shared shape. The P0..P3 visual severity maps onto the
three-level diagnostic vocabulary (P0/P1 → `error` so `findings.ok` is false on a
deck that still needs repair, P2 → `warning`, P3 → `info`); the precise P-level is
preserved in `evidence.visual_severity` and the report rollups. Visual categories
are namespaced `FIT` for content overflow (`text_overflow`, `text_truncation`) and
`RENDER` for every other defect, and the first `suggested_fix` becomes the
finding's remediation.

A slide whose inspection *failed* — an API/transport/decode error or malformed
model output in `mode: "vision"`, a vision deadline (`VISION_TIMEOUT`), or an
undecodable image in `mode: "heuristic"` — is no longer silently dropped. The
per-slide `SlideResult.error` projects to an **error-severity** finding
(`RENDER.VISION_INSPECTION_FAILED`, `RENDER.VISION_TIMEOUT`, or
`RENDER.HEURISTIC_INSPECTION_FAILED`), with the failure mode in
`evidence.source` (`vision`/`heuristic`) and the source image in
`evidence.image_path` when known. Because the finding is error-severity,
`findings.ok` is false even when no visual *defects* were returned. Two new
top-level fields make the clean-vs-failed distinction explicit without scanning
findings: `failed_slide_count` (slides whose inspection failed) and
`inspection_status` (`complete` / `partial` / `failed`). An agent must never read
an empty findings list as a clean deck when `inspection_status != "complete"`.

The `auto_repair` / `make_deck` `visual_qa` phase carries the same signal: each
`passes[]` entry adds `failed_slide_count` and `inspection_status`, and the
`visual_qa` block adds `inspection_complete` (false when any pass had inspection
failures) and a roll-up `failed_slide_count`. When an inspected pass fails, the
loop records a `notes[]` entry and does not treat zero actionable findings as a
clean convergence.

HTTP serve mode has migrated its one diagnostic-bearing endpoint: pattern
validation (`POST /api/v1/patterns/{name}/validate` and `/expand`) now emits a
`FindingEnvelope` as the response body — stamped with `subcommand:
"validate_pattern"` or `"expand_pattern"` — replacing the legacy
`apierrors.Response` with `error.details.validation_errors[]`. The originating
pattern name rides on every finding's `evidence.pattern` so an agent can
correlate the failure without a separate top-level field. The HTTP **transport**
errors — the convert endpoint's request/content validation and JSON parse
errors, plus every transport status (404 not-found, 415 content-type, 413
too-large, 504 timeout, 500 internal/expand-failed) — intentionally keep the
simple `apierrors.Response` shape (`{success, error{code, message, details}}`):
those paths build ad-hoc typed errors rather than `[]diagnostics.Diagnostic`, so
migrating them would either split a single endpoint across two shapes or churn
the whole convert surface for no agent-recovery gain.

`examine-template` has migrated (CLI): it emits the envelope natively under the
`findings` key of its `report.json` (see section 4). MCP `examine_template`
parity is tracked separately.

`preflight` (CLI) emits the `FindingEnvelope` natively as its entire stdout
payload — stamped with `subcommand: "preflight"` — across every static-check
stage in one pass (see section 6). MCP `preflight` parity is tracked separately.

When adding a new diagnostic-bearing surface, return a `FindingEnvelope` built
with `diagnostics.BuildEnvelope` and add any new code to
`internal/diagnostics/codes.go` plus its `describe-finding` entry in
`internal/diagnostics/describe.go` (lowercase fit/pattern codes live in the
`internal/patterns` registry instead). Codes that
are dotted (e.g. `LAYOUT.MISSING_ROLE`) cannot live in `codes.go` (the
`SCREAMING_SNAKE` invariant forbids the dot); classify them with a prefix rule
in `diagnostics.ClassifyCode` instead — the `layout.` prefix routes to the `TPL`
namespace.

## 4. `examine-template` — the template capability report

`json2pptx examine-template <template.pptx> --out <dir>` is the deepest
read-only template diagnostic. It is a thin CLI over the reusable
`internal/examine` service (`examine.Examine(reader, opts) (*Report, error)`),
so the same report can back an MCP `examine_template` tool, template CI, and
docs without re-deriving the facts.

It writes a directory an agent or human can read to know exactly what a
user-provided template supports:

```
<out>/
  report.json            FindingEnvelope (nested under "findings") + slide
                         dimensions + theme + canonical_coverage +
                         derivable_layouts + layouts[]
  report.md              Human-readable pass/fail matrix + remediation list
  theme.json             Scheme colors + major/minor fonts
  conformance.json       validate-template + template-check evidence, merged
  canonical_roles.json   Per-layout canonical group + per-placeholder role
  layouts/
    slideLayoutN__<canonical>.json   Parsed LayoutReport
    slideLayoutN__<canonical>.xml    Pretty-printed raw layout XML
    slideLayoutN__<canonical>.svg    Annotated overlay (see below)
    slideLayoutN__<canonical>.png    Rendered layout (best-effort; needs
                                     LibreOffice + ImageMagick)
  master/
    slideMasterN.{xml,json}
```

The annotated SVG shows, per placeholder: id, derived role, FontSize-aware
`max_chars`, exact bounds in inches, and z-index. The content zone
(title-bottom, footer-top, side-margins) is drawn as a dashed inset, and
section-number frames get a badge. Every placeholder group carries the same
numbers as `report.json` on `data-*` attributes, so the overlay and the JSON
cannot drift.

**Why `report.json` nests the envelope.** The envelope schema is
`additionalProperties: false`, so a `FindingEnvelope` cannot carry the extra
structural fields `report.json` needs at its top level. Following the
`validate-template` precedent, the envelope lives one level down under
`findings`, where it validates against
[`docs/api/finding-envelope.schema.json`](api/finding-envelope.schema.json); the
sibling `canonical_coverage` / `derivable_layouts` / `layouts` fields describe
capability rather than diagnose.

The single new finding code is `TPL.LAYOUT.MISSING_ROLE` (warning), emitted once
per absent content-bearing canonical family (section 5). Its
`evidence.family` names the missing family (`title-slide`, `section-divider`,
`one-content`, or `qa-closing`) and `evidence.expected_layout_type` names the
canonical layout that would satisfy it; `canonical_coverage.<family>.present` is
`false` for the same gap.

## 5. Canonical layout groups

Every layout is classified into one canonical type
(`types.CanonicalLayoutType`) by the single authoritative classifier
(`template.ClassifyLayoutCanonical`), which collapses into four coarse
**content-bearing families** (`types.CanonicalLayoutFamily`) that every usable
template should cover:

| Family | Canonical types | Role |
| --- | --- | --- |
| `title-slide` | Title Slide | Opening / cover slide. |
| `section-divider` | Section Divider | Section break with optional number. |
| `one-content` | One Content, Two Content | The body-bearing workhorse layouts. |
| `qa-closing` | Closing | Thank-you / Q&A / end slide. |

Utility layouts (`Blank`, `Blank + Title`) map to the `other` family and are not
required. `examine-template` reports `canonical_coverage` keyed by family
(`present` + the layout names that provide it) and emits a
`TPL.LAYOUT.MISSING_ROLE` finding for each of the four families that is absent.

`derivable_layouts` is the complementary view: higher-level layouts the engine
can synthesise or overlay from the base layouts (`two-content`, `comparison`,
`full-image`, `blank-title`, `stat-grid`, `timeline`, `journey`,
`panel-layout`), each with `ready: true|false` and `missing[]` naming the absent
prerequisite. `ready` means the engine can produce the capability, **not** that
its name is a valid `layout_id`. Use `addressable_as` as `layout_id` when it is a
string; when it is `null`, use the `request_via` surface (`type:image` or
`shape_grid_or_pattern`). It is produced by `template.DerivableLayouts`.

## 6. `preflight` — the single static-check pass

`json2pptx preflight --json <deck.json> --templates-dir <dir> [--strict]` runs
every static check on a deck JSON without writing a `.pptx` (no LibreOffice, no
PNG conversion). It is the agent-native counterpart to `validate`: a primitive
checker that emits deterministic facts and a stage/severity ordering, with no
repair planning or aesthetic decisions. It shares the canonical
placeholder-role classifier and the layout-aware `ContentZone` resolver with
generation, so the geometry it evaluates is the geometry that will render.

Its **entire stdout payload is the `FindingEnvelope`** (pretty-printed JSON),
stamped with `subcommand: "preflight"` and `input_sha256` for correlation. The
deck path may be passed via `--json` or as a positional argument; `--json -`
reads from stdin.

**Stages.** Checks run in a fixed order; every finding is tagged with the stage
that produced it under `evidence.stage`, and the envelope's `findings[]` are
ordered by stage then severity:

| # | `evidence.stage`    | Covers                                                              |
| - | ------------------- | ------------------------------------------------------------------ |
| 1 | `INPUT`             | JSON parse, structure expansion, unknown keys (warn), enum values, required top-level fields. |
| 2 | `POLICY`            | Design-mode constraints and the no-emoji content policy.           |
| 3 | `TEMPLATE`          | Template resolves; layouts parse; canonical roles resolve.         |
| 4 | `LAYOUT`            | Each slide resolves to a real layout (`unknown_layout_id`, missing `layout_id`/`slide_type`). |
| 5 | `PLACEHOLDER`       | Per-placeholder fit: char budget vs `MaxChars`, content-type checks, text overflow. |
| 6 | `GRID`              | shape_grid structure, bounds inside the `ContentZone`, cell fit, contrast. |
| 7 | `PATTERN`           | Patterns / compose envelopes resolve and required slots are populated. |
| 8 | `RENDER_PROJECTION` | Dry-render geometry: title-overlaps-body, footer overlap, title wrap. |

The stage tag is the only `preflight`-specific addition to a finding; every
other field is the standard envelope contract from section 2. Findings keep
their existing `category` namespace (e.g. an `unknown_layout_id` finding stays
`INPUT`-namespaced even though its `evidence.stage` is `LAYOUT`) — the stage is
preflight's execution grouping, not a recategorization.

**Fail-fast.** A stage whose failure makes later stages impossible
short-circuits the rest: an unparseable deck, a missing required field, a
template that will not resolve or analyze. Content-policy findings (stage 2) do
**not** short-circuit — `preflight` is the "run every static check" surface and
reports the full picture in one pass.

**Exit codes.**

| Code | Meaning                                                                 |
| ---- | ---------------------------------------------------------------------- |
| `0`  | No error-severity finding (and, under `--strict`, no warnings either). |
| `2`  | At least one error-severity finding — or, under `--strict`, any warning. |
| `3`  | Internal failure (e.g. the envelope was computed but could not be written). |

The envelope's `ok` flag always reflects error severity only; `--strict` raises
the *exit code* on warnings without changing `ok`, so the wire shape stays
consistent across surfaces.

## 7. MCP call mechanics

Moved from the generate-deck skill so the skill bundle stays within its
budget; the skill's TOOLS.md summarises these in three sentences.

### 7.1 Machine-actionable `next_tool_call`

Pattern validation errors (`validate_pattern`), density warnings (`expand_pattern`), fit-report findings (`validate_input`, `generate_presentation`), and boundary errors from the candidate-decision tools (`plan_deck`, `recommend_pattern`, `recommend_visual`, `validate_input`, `preview_presentation_plan`, `score_deck`) include an optional `next_tool_call` field when the error has an actionable recovery. This is a machine-readable hint: the exact MCP tool name and an `args_template` pre-filled with fix parameters. Invoke the suggested tool directly without inferring the protocol from the error message.

Boundary-error mappings used by the candidate-decision tools:

- `MISSING_PARAMETER` / `INVALID_JSON` on `presentation` → `get_input_schema` (fetch the schema and retry)
- `MISSING_PARAMETER` on `template` (or `TEMPLATE_NOT_FOUND` / `TEMPLATE_ERROR`) → `list_templates`
- `MISSING_PARAMETER` on `brief` / `intent` → retry the same tool with the missing argument
- `INVALID_PARAMETER` for an unknown pattern name → `list_patterns`
- `UNKNOWN_PARAMETER` (any tool) → the argument name is not accepted by the tool; rename it to `fix.params.did_you_mean` (e.g. `plan_deck` `slide_count` → `slide_budget`) and retry via `next_tool_call`
- `STRUCTURE_AND_SLIDES` on `structure` → remove one of the two — `structure` and top-level `slides` are mutually exclusive. The `fix.params.field` names which side to drop (`"slides"`).
- `INVALID_STRUCTURE` on `structure` → repair the structure block (missing section title, empty sections, section with no slides). The underlying expansion error is in `fix.params.error`.

```json
{
  "field": "values.title",
  "code": "unknown_key",
  "message": "unknown field \"titl\" (did you mean \"title\"?)",
  "fix": { "kind": "rename_field", "params": { "from": "titl", "to": "title" } },
  "next_tool_call": {
    "tool": "repair_slide",
    "args_template": {
      "slide_index": -1,
      "pattern": "card-grid",
      "fixes": [{ "kind": "rename_field", "params": { "from": "titl", "to": "title" } }]
    }
  }
}
```

- `slide_index: -1` means "caller must supply the actual slide index" — `validate_pattern` operates without slide context.
- For `swap_pattern` / `adopt_pattern` fix kinds, `next_tool_call` points to `recommend_visual` (`{intent, content_hints: {item_count}}`, in every tool profile) instead of `repair_slide`; fill in `intent` before calling.
- Internal-only errors (marshal failures, unrecognized fix kinds inside content-finding errors) may omit `next_tool_call` (the field is absent, not null). Boundary errors from candidate-decision tools always carry it.


### 7.2 `response_fingerprint` — server-side cache key

`validate_input`, `preview_presentation_plan`, `plan_deck`, and `recommend_visual` responses include a top-level `response_fingerprint` field: a sha256 hex digest (64 chars) of the canonical JSON of the response body with the fingerprint field itself zeroed. These four paths are deterministic — identical inputs produce identical fingerprints — so agents may use the fingerprint directly as a memoisation cache key without re-hashing the body. To verify a fingerprint, parse the response, zero `response_fingerprint`, re-marshal canonically, and sha256-hash the result.


### 7.3 `idempotency_key` — safe retries for generate / auto_repair / make_deck

`generate_presentation`, `auto_repair`, and `make_deck` accept an optional top-level `idempotency_key` string. When set, the server caches the first successful response under that key and replays it on subsequent calls within the cache TTL (1 hour, per-process), **but only when the request content is unchanged**. The replay response carries `"idempotent_replay": true` so the caller can tell a deduped retry from a fresh run.

The key is a *retry token*, not a request identity: the server also stores a fingerprint of the normalized request (every argument except `idempotency_key`). Reusing the same key with edited input is treated as a different request — the server refuses with an `IDEMPOTENCY_CONFLICT` error (carrying `current_fingerprint` and `original_fingerprint` in the finding evidence) instead of replaying the original deck for the wrong content. Issue a fresh key for new content, or restore the original input to replay.

Use this to make transport-layer retries safe. Without an idempotency key, every retry runs the full pipeline again and writes a fresh output file (`output.pptx`, `output_1.pptx`, `output_2.pptx`, …); the caller is also billed for the wasted inference + render cost.

```json
{
  "presentation": { "template": "midnight-blue", "slides": [/* … */] },
  "output_filename": "deck.pptx",
  "idempotency_key": "agent-session-abc123/turn-7"
}
```

Rules of thumb:

- Generate the key from something stable across retries (session id + turn number, or a hash of the input). Never use a timestamp — every retry would get a new key.
- Keys are scoped per-tool, so the same string used against `generate_presentation` and `auto_repair` will not collide.
- A replay requires the request to be byte-for-byte equivalent (modulo object-key ordering). If you edit the deck/outline or any other argument and keep the key, you get an `IDEMPOTENCY_CONFLICT` error, never a stale replay — bump the key whenever the content changes.
- Only successful responses are cached. Error responses surface every time so the agent can fix the underlying input.
- The cache is in-memory and per-process. Restarting the MCP server drops it — design retries to tolerate a fresh run after a server bounce.

## 8. Appendix — catalogue codes without prose elsewhere

Every code `describe_finding` resolves must be named somewhere under `docs/`
or `skills/`. The table below is generated from the describe_finding
catalogue for the codes that no hand-written doc covers yet — mostly
structural / output-validation codes (`OPC_*`, `OOXML_*`), MCP call errors,
visual-QA codes and per-renderer chart / diagram diagnostics. Call
`describe_finding` for the full `when_emitted`, remediation steps and
examples. `TestFindingCatalogCodesAreDocumented`
(`cmd/json2pptx/finding_catalog_docs_test.go`) fails when a catalogue code
is in no doc; when you add prose for a code, regenerate so it leaves the
table:

```bash
go test ./cmd/json2pptx -run TestFindingCatalogCodesAreDocumented -update-diag-appendix
```

<!-- BEGIN GENERATED: finding-code appendix (go test ./cmd/json2pptx -run TestFindingCatalogCodesAreDocumented -update-diag-appendix) -->

| Code | Severity | Summary |
|------|----------|---------|
| `AMBIGUOUS_CANONICAL_ROLE` | review | Two template layouts tie for a canonical role. |
| `ASSET_TOO_LARGE` | refuse | An asset exceeds the maximum allowed size. |
| `DANGLING_REL` | refuse | A PPTX relationship points to a missing part. |
| `DUPLICATE_ID` | refuse | A slide repeats a shape identifier. |
| `DUPLICATE_REL_ID` | refuse | A PPTX relationships file reuses an ID. |
| `DUPLICATE_SLIDE_ID` | refuse | presentation.xml repeats a slide id. |
| `EMPTY_REQUIRED_ATTR` | refuse | A required OOXML attribute is empty. |
| `ICON_AMBIGUOUS` | refuse | An icon name matches more than one bundled icon. |
| `ICON_FILL_IGNORED_ON_INLINE` | review | An icon `fill` was ignored because inline `svg_data` is set. |
| `ICON_LIST` | refuse | Listing the available icons failed. |
| `ICON_MISSING` | refuse | An icon reference is empty. |
| `ICON_PATH` | refuse | An icon path argument is invalid. |
| `ICON_PATH_SYMLINK_ESCAPE` | refuse | An icon path resolves through a symlink that escapes the allowed root. |
| `ICON_PATH_TRAVERSAL` | refuse | An icon path attempts directory traversal outside the allowed root. |
| `ILLEGAL_XML_CHAR` | refuse | Slide text contains an illegal XML character. |
| `INVALID_ARG` | refuse | A tool argument is missing or invalid. |
| `INVALID_COLOR` | refuse | An OOXML color value is invalid. |
| `INVALID_DECK_SPEC` | refuse | The DeckSpec stored under deck_id could not be parsed or compiled. |
| `INVALID_IMAGE` | refuse | An image supplied for inspection is invalid. |
| `INVALID_KEY` | refuse | An object contains a key the schema does not allow. |
| `INVALID_SCHEME` | refuse | A color references an invalid theme scheme slot. |
| `INVALID_SLIDE_INDEX` | refuse | A slide index argument is out of range. |
| `INVALID_TABLE` | refuse | A table has invalid OOXML structure. |
| `LAYOUT_DERIVED` | info | An asymmetric two-column slide was derived from a template layout. |
| `MALFORMED_XML` | refuse | A PPTX XML part is malformed. |
| `MISSING_CONTENT_TYPE` | refuse | A PPTX part has no content type. |
| `MISSING_CONTENT_TYPE_OVERRIDE` | refuse | A required PPTX content-type override is absent. |
| `MISSING_ELEMENT` | refuse | A required OOXML element is absent. |
| `MISSING_PART` | refuse | A required PPTX package part is missing. |
| `MULTIPLE_CURRENT_STAGES` | review | More than one journey stage is marked current. |
| `OOXML_DUPLICATE_ID` | refuse | A slide repeats a shape identifier. |
| `OOXML_EMPTY_REQUIRED_ATTR` | refuse | A required OOXML attribute is empty. |
| `OOXML_ILLEGAL_XML_CHAR` | refuse | Slide text contains an illegal XML character. |
| `OOXML_INVALID_COLOR` | refuse | An OOXML color value is invalid. |
| `OOXML_INVALID_SCHEME` | refuse | A color references an invalid theme scheme slot. |
| `OOXML_LOW_FALLBACK_DPI` | review | An SVG fallback image is below 96 DPI at display size. |
| `OOXML_SLIDE_COUNT_MISMATCH` | refuse | Slide references and slide files disagree in count. |
| `OOXML_ZERO_EXTENT` | refuse | A shape has zero width or height. |
| `OPC_DANGLING_REL` | refuse | A PPTX relationship points to a missing part. |
| `OPC_DUPLICATE_REL_ID` | refuse | A PPTX relationships file reuses an ID. |
| `OPC_DUPLICATE_SLIDE_ID` | refuse | presentation.xml repeats a slide id. |
| `OPC_MALFORMED_XML` | refuse | A PPTX XML part is malformed. |
| `OPC_MISSING_CONTENT_TYPE` | refuse | A PPTX part has no content type. |
| `OPC_MISSING_CONTENT_TYPE_OVERRIDE` | refuse | A required PPTX content-type override is absent. |
| `OPC_MISSING_ELEMENT` | refuse | A required OOXML element is absent. |
| `OPC_MISSING_PART` | refuse | A required PPTX package part is missing. |
| `OUTPUT_DIR` | refuse | The output directory is missing or not writable. |
| `READ_FAILED` | refuse | The input deck or a referenced file could not be read. |
| `SEMANTIC_EVIDENCE_VISUAL_MISSING` | review | A market-analysis deck has no data-bearing evidence visual. |
| `SEMANTIC_REQUIRED_LAYOUT_DUPLICATE` | refuse | meta.required_layouts lists the same canonical layout more than once. |
| `SEMANTIC_REQUIRED_LAYOUT_MISSING` | refuse | A required layout lacks a compatible slide or is unavailable in the selected template. |
| `SEMANTIC_REQUIRED_LAYOUT_UNKNOWN` | refuse | meta.required_layouts contains an unknown canonical layout ID. |
| `SEMANTIC_VISUAL_FAMILY_NARROW` | review | A longer deck uses too few non-structural visual families. |
| `SETTINGS_ERROR` | refuse | A template-settings operation failed. |
| `SETTINGS_WRITE_DISABLED` | refuse | Writing template settings is disabled. |
| `SLIDE_COUNT_MISMATCH` | refuse | Slide references and slide files disagree in count. |
| `STRICT_FIT` | refuse | Strict-fit mode refused the deck because content overflows. |
| `STYLE_NOT_FOUND` | refuse | A referenced named style is not defined. |
| `TEMPLATES_DIR` | refuse | The templates directory is missing or unreadable. |
| `TEMPLATE_ASPECT_RATIO_INVALID` | review | The metadata aspect_ratio is not in WIDTH:HEIGHT form. |
| `TEMPLATE_LAYOUT_HINT_INVALID` | review | A layout hint in the metadata is malformed (empty key or a negative budget). |
| `TEMPLATE_METADATA_PARSE` | review | The template's embedded metadata file could not be read or parsed. |
| `TEMPLATE_METADATA_VERSION` | review | The template metadata declares a version outside the supported range. |
| `TEMPLATE_SECTION_NUMBER_NAMING` | review | A section-header layout has a decorative number placeholder that is not named "Section Number". |
| `UNKNOWN_TABLE_STYLE_ID` | refuse | A table references a style_id the template does not define. |
| `UNKNOWN_THEME_COLOR` | refuse | A semantic color name is not part of the template theme. |
| `UNSUPPORTED` | refuse | The requested operation or option is not supported. |
| `URL_RESOLVER_INIT` | refuse | The URL asset resolver failed to initialize. |
| `VALIDATION_FAILED` | refuse | Input validation failed with at least one error-severity finding. |
| `ZERO_EXTENT` | refuse | A shape has zero width or height. |
| `chart.all_zero_series` | review | A chart series contains only zero values and would render as a flat line. |
| `chart.auto_log_scale_applied` | info | Legacy svggen finding for an automatically applied log scale (no longer emitted). |
| `chart.invalid_numeric` | review | Chart data contains a value that cannot be parsed as a number. |
| `chart.invalid_time_format` | review | A time-axis value is not in a recognized format. |
| `diagram.label_truncated` | review | A diagram label was shortened to fit. |
| `diagram.quadrant_position_defaulted` | review | A matrix quadrant had no valid position and was placed by its list index. |
| `invalid_enum` | refuse | A pattern field has an unsupported enum value. |
| `layout_synthesized` | review | A missing template layout was synthesized. |
| `length_mismatch` | refuse | Chart categories and values have different lengths. |
| `navigation_requires_authoring` | refuse | Bullet continuation cannot safely shift numeric slide destinations. |
| `palette_drift` | review | Rendered colors drift from the template theme palette. |

<!-- END GENERATED: finding-code appendix -->
