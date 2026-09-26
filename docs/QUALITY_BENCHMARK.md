# Deck Quality Benchmark

`cmd/qualitybench` measures complete generated decks against frozen briefs and
template families. It keeps authoring runs, blind rendering evidence, reviewer
ratings, and the release decision in one report.

## Default template set

The default set uses two bundled templates plus the purpose-built
`portability-side-logo` fixture as the held-out family. The fixture lives under
`tests/quality/fixtures/portability/templates/`, outside the normal
`templates/` directory, so day-to-day template tuning does not train against
the held-out case.

Template entries accept either form:

```text
name:family[:heldout]
name=path/to/template.pptx:family[:heldout]
```

Relative explicit paths are resolved from the working directory and then from
`--templates-dir`. Use a distinct `name` for every entry because it becomes
part of the run ID.

## Produce blind evidence

An agent run is opt-in and requires an explicit executable:

```bash
go run ./cmd/qualitybench \
  --agent "/absolute/path/to/agent --its-flag" \
  --agent-model "provider-model-id" \
  --agent-version "agent-cli-version" \
  --parallel 4 \
  --out output/quality-benchmark \
  --results-dir tests/quality/results
```

Agent runs default to one worker. Use `--parallel N` only when the configured
provider and local renderer can sustain `N` independent runs; evidence stays in
the same deterministic request order regardless of completion order.
Pass `--agent-model` and `--agent-version` from provider or CLI configuration
so every evidence record carries the same authoritative identity instead of a
model-generated label.

For a provider-free harness check, use `--dry-run`. It renders deterministic
reference decks and is useful for testing the fixture and contact-sheet path;
it is not an agent-quality result.

The run writes:

- `sheets/<blind_id>.png`: contact sheets given to reviewers;
- `ratings_template.csv`: blank rating form;
- `blind_key.json`: unblinding map retained by the benchmark operator;
- a results JSON under `--results-dir`.

## Collect one blind review

Before rating historical sheets after generator changes, replay their frozen
authoring inputs into a **new** directory:

```bash
go run ./cmd/qualitybench \
  --refresh-report tests/quality/results/agent-20260921T101451Z.json \
  --out output/quality-benchmark-current
```

This regenerates all decks and sheets, compares decoded pixels, preserves the
historical bundle, and writes `report.json` plus an operator-only
`refresh_audit.json` (source report, engine commit, input and pixel hashes).
Semantic inputs (`meta` / slide `kind`) use `semantic render`; raw inputs
use `generate`. Both retain the original input location for relative resources.
It is a current-engine replay of frozen agent outputs, **not a new agent
authoring run**. Review `output/quality-benchmark-current` instead and apply
the exported CSV to its `report.json`. Do not reuse ratings from old pixels.
Browser progress is keyed by sheet contents so changed sheets start a fresh
ballot. A failed replay retains partial output for diagnosis; choose a new
directory for the next attempt.

One blind reviewer is required, human **or AI** (user decision, 2026-09-26).
Additional reviews are optional. Give the reviewer only anonymized contact
sheets, the brief, and the scoring rubric—never configuration labels or
`blind_key.json`. AI ballots must use `reviewer_type=llm`, not `human`.
A fresh-context AI reviewer avoids bias from the operator's prior inspection.
Pixel heuristics alone do not count as review. The report retains the raw
ratings and records reviewer types; AI results are not human validation.

Prepare an independently re-anonymized packet for that reviewer:

```bash
go run ./cmd/qualitybench \
  --prepare-blind-review output/quality-benchmark-current/report.json \
  --agent-review-dir output/quality-benchmark-blind
```

Give the reviewer **only** `output/quality-benchmark-blind/packet/`. It contains
new random IDs, sheets, briefs, and the rubric. Keep `operator-report.json`
and `operator-key.json` private. The reviewer returns `ratings.csv` and
per-item reasoning. Apply those ratings to the operator report, whose IDs
match the packet. Failed generation items have no sheet and cannot receive
visual scores; they remain explicit failures and block release approval.

### Browser UI (no Excel)

```bash
go run ./cmd/qualitybench --review-ui --out output/quality-benchmark-codex \
  --report tests/quality/results/agent-20260921T101451Z.json
```

Open `http://127.0.0.1:8765`. Enter a reviewer name, inspect/enlarge each sheet,
and select the five scores and two defect answers. **Compare matching deck**
shows the same brief/template/repetition without revealing either configuration;
**Rate this deck** swaps which side you score. **Pin for comparison** keeps any
sheet beside another. Scores always apply to the current deck. Use
**Save & next unrated** to advance. Progress is saved per reviewer and dataset
in this browser. Download a JSON backup regularly, especially before switching
browsers or computers; restore it using the same reviewer name. CSV export is
available only once every sheet is complete. The server exposes only opaque
IDs and their sheets, binds to loopback, and never receives or writes ratings.
Use `--listen 127.0.0.1:8766` if the default port is occupied.

Alternatively, fill `ratings_template.csv` directly. Every row needs:

- `reviewer`: stable reviewer identifier;
- `reviewer_type`: `human` for a person, `llm` for a blind AI reviewer;
- five 1–5 scores: readability, hierarchy, template fidelity, factual
  completeness, and usability;
- `lost_critical_fact` and `critical_template_defect`: `true` or `false`.

Apply the exported file:

```bash
go run ./cmd/qualitybench \
  --report tests/quality/results/<agent-report>.json \
  --ratings ratings-reviewer.csv
```

The command rejects missing columns, scores outside 1–5, invalid booleans, and
unknown blind IDs. A rated deck requires at least one blind reviewer marked
`human` or `llm`; duplicate rows do not add votes. The legacy summary field
`RatedPairs` now counts rated decks, not pairs of reviewers. If a second review
is supplied, both must consider a deck usable and disagreement statistics are
retained. Any generation failure blocks approval even if scores are supplied.

## Compare three agent reviewers across two runs

Blind AI ratings are eligible for the release gate under the single-reviewer
policy. This optional multi-agent flow compares rating stability; it does not
turn AI ratings into human validation.
Prepare six blank blinded ballots without launching any reviewer:

```bash
go run ./cmd/qualitybench \
  --prepare-agent-ratings output/quality-benchmark/ratings_template.csv \
  --agent-reviewers agent-a,agent-b,agent-c \
  --agent-review-runs 2 \
  --agent-review-dir output/quality-benchmark/agent-reviews
```

The command writes `manifest.json` and one CSV for each agent and run. Give an
agent only its ballot and `sheets/`; keep `blind_key.json`, the other ballots,
and prior ratings private. Use a fresh context for run 2 so the second run does
not inherit scores or discussion from run 1.

After all six CSVs are complete, compare them with:

```bash
go run ./cmd/qualitybench \
  --compare-agent-ratings output/quality-benchmark/agent-reviews/manifest.json
```

The comparison rejects missing agents, runs, blind IDs, scores, and duplicate
ballots. It writes detailed JSON with all six raw ballots, per-agent two-run
means, run-to-run repeatability, and consensus values, plus a compact consensus
CSV. Numeric consensus is the median of the three agent means, where each
agent mean averages its two runs. Boolean defect flags use a strict majority
across all six ballots: four or more `true` votes resolve to `true`, two or
fewer resolve to `false`, and a 3–3 split is reported as `tie` for adjudication.

## Refresh the committed baseline

After an engine or workflow change:

1. Run the same frozen briefs, template specification, configurations, and two
   repetitions used by the prior baseline.
2. Collect one blind human or AI rating file without sharing the blind key.
3. Apply both CSVs to the new report.
4. Commit the rated report under `tests/quality/results/` with the agent model,
   version, prompt, tool calls, artifacts, cost, iterations, and summary intact.
5. Compare usable rate, Wilson interval, critical failures, disagreements, and
   paired improvement with the previous rated report.

The preregistered pass bar is at least 80% usable, zero lost critical facts,
zero critical template defects, complete paired ratings, and higher redesigned
usability than baseline. An unrated or partially rated report remains `hold` or
`inconclusive`.
