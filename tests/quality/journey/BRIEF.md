# Agent-journey brief (common rules)

You are evaluating **json2pptx** the way a first-time AI-agent user meets it. The goal is an honest friction log that is turned into product improvements, so that agents enjoy using the tool. Your persona file (`personas/<persona>.md`) says what to build; this file says how to work and what to hand back.

`$J` is the run directory the coordinator gave you (for example `/tmp/jj/run`), `$BIN` the prebuilt binary, `$REPO` the checkout it was built from.

## Hard rules

- Do NOT read the json2pptx repository (source, `docs/`, `skills/`, `tests/`, `examples/`) unless your persona explicitly allows a path. Learn only from what the product tells you: MCP tool descriptions and responses, CLI help and output. That is what an external agent has.
- Do not modify `$REPO`. Do not run `bd`. Do not kill processes you did not start (no `pkill` patterns). Do not call external model APIs (`json2pptx inspect`, anything that reads `ANTHROPIC_API_KEY`).
- Work only under `$J/<persona>/`.
- Report what happened, with evidence. Do not speculate about code.

## MCP bridge (MCP personas)

One server session per persona; `<sock>` must be short (unix socket paths stop at about 100 characters), so keep it under `/tmp`:

```
mkdir -p $J/<persona>/out
(cd $J/<persona> && nohup python3 $REPO/tests/quality/journey/mcpd.py serve /tmp/jj-<persona>.sock $J/<persona>/log \
   -- $BIN mcp --templates-dir $REPO/templates --out $J/<persona>/out [--tools all] > bridge.log 2>&1 &)
python3 mcpd.py list  /tmp/jj-<persona>.sock                    # tool names + first line of each description
python3 mcpd.py raw   /tmp/jj-<persona>.sock tools/list         # the tool definitions as the client receives them
python3 mcpd.py call  /tmp/jj-<persona>.sock <tool> '<json args>'   # or '@/path/to/args.json'
python3 mcpd.py summary $J/<persona>/log                        # the run's numbers, for your findings.json
python3 mcpd.py stop  /tmp/jj-<persona>.sock                    # when you are done
```

What the server tells a client, in the order a client sees it:

1. `log/initialize.json`: the `initialize` result with the server's instructions. Read it first.
2. `tools/list` (default profile): abridged. `get_started` with `tool: "<name>"` returns one tool's full description and input schema; `get_started` with `task` returns the workflow.
3. `call` prints the tool's `structuredContent` as JSON on stdout. On stderr it prints the call number (`[call n=12: 4628 bytes, 0.01s]`), every image block it saved (`[image saved: …/log/images/n012-01.jpeg]`) and `[isError=true]`. Open the saved images with your image reader: `render_deck_thumbnails` returns one image per slide, and `list_slide_kinds` / `recommend_visual` with `preview: true` return a picture of a kind or a candidate before you author it.
4. Findings are addressed by `path` (a JSON Pointer into your spec) and `slide_number` (1-based).

Every call is logged to `log/calls.jsonl` and `log/resp-NNN.json`. Cite call numbers (`n=12`) as evidence.

## What to log (as you go, not from memory at the end)

Friction: anything where you were confused, had to guess, needed more than one attempt, got an error you could not act on in one step, received a payload far larger than you needed, waited long, could not find or see something (what a layout looks like before choosing it, how many items it takes), got contradictory guidance, or produced a slide that looked wrong although the tool said it was fine.

Delights: things that worked noticeably well. They are kept under test (`TestAgentJourneyDelights`), so say exactly what happened.

## Deliverables (in `$J/<persona>/`)

1. `findings.json`:
   ```
   {"persona": "...",
    "metrics": {"tool_calls": N, "total_response_bytes": N, "response_bytes_excluding_images": N,
                "validates_before_first_ready_render": N, "renders_to_first_ready": N,
                "wall_minutes": N, "enjoyment_1_to_10": N, "enjoyment_reasons": ["..."]},
    "findings": [{"id": "A1", "title": "<short, specific>", "severity": "blocker|major|minor|polish",
                  "journey_stage": "onboarding|discovery|authoring|validation|render|review|repair|delivery",
                  "what_happened": "...", "evidence": ["call n=12: <short excerpt>", "path/to/file"],
                  "expected": "...", "suggestion": "..."}],
    "delights": [{"title": "...", "evidence": ["..."]}]}
   ```
   Take the first five metrics from `mcpd.py summary`; `enjoyment_1_to_10` is yours.
2. `report.md`: a short narrative of the journey (what you tried in order, where the time went), the top five things that would make an agent love this tool, and the final artefacts (pptx paths, the slide images you inspected).

Keep evidence excerpts short. Aim for specific, deduplicated findings (quality over quantity; typically 8-20).
