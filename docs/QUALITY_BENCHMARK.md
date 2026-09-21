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

## Collect two human reviews

Give each reviewer only `sheets/` and a separate copy of
`ratings_template.csv`. Keep `blind_key.json` private until both files are
returned. Reviewers fill every row with:

- `reviewer`: stable reviewer identifier;
- `reviewer_type`: `human` (an optional model rater uses `llm`);
- five 1–5 scores: readability, hierarchy, template fidelity, factual
  completeness, and usability;
- `lost_critical_fact` and `critical_template_defect`: `true` or `false`.

Apply both files together:

```bash
go run ./cmd/qualitybench \
  --report tests/quality/results/<agent-report>.json \
  --ratings ratings-reviewer-a.csv,ratings-reviewer-b.csv
```

The command rejects missing columns, scores outside 1–5, invalid booleans, and
unknown blind IDs. A rated pair requires two distinct reviewers marked
`human`; duplicate rows and `llm` ratings do not satisfy the release gate.

## Refresh the committed baseline

After an engine or workflow change:

1. Run the same frozen briefs, template specification, configurations, and two
   repetitions used by the prior baseline.
2. Collect two independent human rating files without sharing the blind key.
3. Apply both CSVs to the new report.
4. Commit the rated report under `tests/quality/results/` with the agent model,
   version, prompt, tool calls, artifacts, cost, iterations, and summary intact.
5. Compare usable rate, Wilson interval, critical failures, disagreements, and
   paired improvement with the previous rated report.

The preregistered pass bar is at least 80% usable, zero lost critical facts,
zero critical template defects, complete paired ratings, and higher redesigned
usability than baseline. An unrated or partially rated report remains `hold` or
`inconclusive`.
