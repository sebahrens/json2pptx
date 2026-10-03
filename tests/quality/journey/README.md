# Agent journey: harness, metrics and results

A first-time agent's path through json2pptx (first contact, plan, author, validate, repair, render, look) is evaluated in two ways:

| | What | Where | When it runs |
|---|---|---|---|
| Deterministic metrics | Bytes read before the first slide, round-trips to a clean render, validate/render parity, patches that clear their finding | `TestAgentJourneyMetrics` in `cmd/json2pptx/agent_journey_metrics_test.go` | Every `go test ./...`, CI included |
| Delights | What agents said worked well, one check each | `TestAgentJourneyDelights` in `cmd/json2pptx/agent_journey_delights_test.go` | Every `go test ./...` |
| Agent runs | Five personas play the journey against the built binary and write a friction log | This directory | By hand, after a change to what agents read first |

`results.jsonl` holds one row per recorded run of either kind. The first review (2026-10-03, enjoyment 6-7 of 10, 58 findings) is in `tests/quality/results/agent-journey-20261003/`.

## Files

| File | |
|---|---|
| `mcpd.py` | Stdio-MCP bridge: keeps one `json2pptx mcp` session open on a unix socket and logs every call |
| `BRIEF.md` | Rules common to all personas: learn only from the product, what to log, what to hand back |
| `personas/a-coldstart.md` | A board deck from a cold start, on two templates |
| `personas/b-discovery.md` | Finding the right layout for fourteen slide intents, default and `--tools all` |
| `personas/c-repair.md` | Repairing a twelve-flaw draft from diagnostics alone |
| `personas/d-cli.md` | CLI only, with the installed skill |
| `personas/e-revise.md` | Eight revision turns on one deck |
| `fixtures/coldstart-board-update.json` | The deck the cold-start run of 2026-10-04 finished with; the tests send it as a first draft |
| `results.jsonl` | The trend: one JSON row per recorded run |

## Running an agent journey

Needs Python 3, LibreOffice (`soffice`) and `pdftoppm` for thumbnails. No model API is called by the harness; the agent is whoever drives it.

```bash
# from the repository root
REPO=$PWD; BIN=/tmp/jj/j2p; J=/tmp/jj/run; P=a-coldstart
go build -o $BIN ./cmd/json2pptx
mkdir -p $J/$P/out
M=$REPO/tests/quality/journey/mcpd.py

# 1. Start the bridge (one per persona; add `--tools all` after `mcp` where the persona says so).
#    Keep the socket path short: unix sockets stop at about 100 characters.
(cd $J/$P && nohup python3 $M serve /tmp/jj-$P.sock $J/$P/log \
   -- $BIN mcp --templates-dir $REPO/templates --out $J/$P/out > bridge.log 2>&1 &)

# 2. Drive it.
python3 $M list  /tmp/jj-$P.sock
python3 $M call  /tmp/jj-$P.sock get_started '{"task":"brief"}'
python3 $M call  /tmp/jj-$P.sock validate_deck_spec @$J/$P/v1.json

# 3. Numbers for the results row, then stop.
python3 $M summary $J/$P/log
python3 $M stop    /tmp/jj-$P.sock
```

`call` prints the tool's `structuredContent` as JSON on stdout; the call number, saved images (`log/images/nNNN-II.jpeg`) and `[isError=true]` go to stderr. Every call is logged to `log/calls.jsonl` and `log/resp-NNN.json`.

To hand a persona to an agent, give it `BRIEF.md`, its `personas/<persona>.md`, and the values of `$J`, `$BIN` and `$REPO`. The agent must not read the repository: it learns from the product alone. One agent per persona; the five are independent and can run in parallel.

### What to record

From each persona, its `findings.json` (format in `BRIEF.md`): the `mcpd.py summary` numbers (`tool_calls`, `total_response_bytes`, `response_bytes_excluding_images`, `validates_before_first_ready_render`, `renders_to_first_ready`, `wall_minutes`), its self-reported `enjoyment_1_to_10` with reasons, the findings and the delights. Keep the run's evidence (reports, `calls.jsonl`) under `tests/quality/results/agent-journey-<yyyymmdd>/` when the run is a review that files findings.

A new delight belongs in the `journeyDelights` table of `agent_journey_delights_test.go`, with a check or the reason it has none; the test reads the delights of the 2026-10-03 report and fails for one the table does not know.

### Adding a results row for an agent run

Append one line to `results.jsonl`:

```json
{"date":"2026-10-04","commit":"62581cb9","schema_version":"4.159.0","source":"agent run: a-coldstart on midnight-blue and p-style","personas":{"a-coldstart":{"tool_calls":17,"total_response_bytes":818574,"response_bytes_excluding_images":113946,"validates_before_first_ready_render":1,"renders_to_first_ready":1,"enjoyment_1_to_10":8}}}
```

`date`, `commit` and `source` are required; rows stay in date order. A row carries `personas` (an agent run), `metrics` (the test) or both (the baseline).

## The deterministic metrics

`TestAgentJourneyMetrics` measures, with the templates the binary embeds copied into a temp directory (a local `templates/p-style.pptx` and machine paths do not move a number):

| Metric | What | Limit |
|---|---|---|
| `onboarding_*_bytes` | `initialize` instructions, default `tools/list`, `get_started(brief)`, the `list_slide_kinds` catalogue, `list_templates fields:"names"`, and their sum (first contact) | The budgets of `TestOnboardingPayloadBudgets`; first contact at most 40 KiB |
| `twelve_flaw_validate_round_trips`, `twelve_flaw_calls_to_ready_render` | The c-repair persona's draft, repaired from the findings alone on midnight-blue | 3 validates, 4 calls |
| `twelve_flaw_first_response_bytes`, `_findings` | The first validate response to that draft | 8 KiB, 16 findings |
| `clean_spec_validate_round_trips`, `clean_spec_calls_to_ready_render` | The cold-start fixture sent as a first draft | 1 validate, 2 calls |
| `parity_pairs_compared`, `parity_disagreements` | `validate_deck_spec` against `render_deck_spec` over the short corpus | At least 10 pairs, no disagreement |
| `patches_offered`, `patches_server_verified`, `patches_not_clearing` | Every patch a response offers, applied | At least 8 and 3; none that fails to clear its finding |

Each number is held to its limit and compared with the last row the test recorded. Against that row, bytes may grow 5%, counts that follow text measurement (findings, patches) have a slack of 2-3 because font metrics differ between macOS and the Linux CI, and structural counts (round-trips, disagreements) have none. The measuring code is shared with the acceptance test of each piece (`TestOnboardingPayloadBudgets`, `TestTwelveFlawDraftCleanInThreeRoundTrips`, `TestTwelveFlawDraftFirstResponse`, `TestDeckSpecFindingParityCorpus`, `TestEveryEmittedPatchClearsItsFinding`).

When a change moves a number on purpose, record a row and commit it with the change:

```bash
JOURNEY_RECORD=1 go test -short ./cmd/json2pptx -run TestAgentJourneyMetrics
# optional: JOURNEY_NOTE="why the numbers moved"  JOURNEY_COMMIT=<sha> (default: HEAD, +dirty when the tree has other changes)
```

A run that breaks a limit is not recorded.

## Delights that have no check

`TestAgentJourneyDelights` logs these with the reason: anything that needs a LibreOffice render (thumbnail hashes, the review hand-off, the single-slide image loop, the thumbnail cache), wall-clock claims (speed), judgements about how a slide looks (the maturity marker, the value-chain highlight), and template-specific findings after a template switch (they follow font metrics). An agent run is the check for those.
