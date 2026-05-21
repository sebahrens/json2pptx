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
`describe-finding`, so the command stays runnable against the existing registry.

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
on `findings.ok`. The remaining ad-hoc shapes in `validate`, `generate -dry-run`,
`inspect`, the MCP error envelope, and HTTP serve mode are still pending, along
with emitting the envelope natively from the forthcoming `preflight` and
`examine-template` subcommands.

When adding a new diagnostic-bearing surface, return a `FindingEnvelope` built
with `diagnostics.BuildEnvelope` and add any new code to
`internal/diagnostics/codes.go` plus the `describe-finding` registry.
