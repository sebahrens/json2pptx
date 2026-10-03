# Agent-journey dogfood brief (common rules)

You are evaluating **json2pptx** the way a first-time AI-agent user meets it. The goal is an honest friction log that will be turned into product improvements, so that agents enjoy using the tool.

## Hard rules
- Do NOT read the json2pptx repository (source, docs/, skills/, tests/, examples/) unless your persona section explicitly allows a specific path. Learn only from what the product tells you (MCP tool descriptions/responses, CLI help/output) — exactly like an external agent.
- Do not modify the repository at /Users/seb/projects/json2pptx. Do not run `bd`. Do not kill processes you did not start (no pkill patterns).
- Work only under your own directory /tmp/jj/j/<persona>/ (create it). Use the prebuilt binary /tmp/j2p (current main). Templates dir: /Users/seb/projects/json2pptx/templates (pass it as the templates dir; you may list which .pptx files exist there but not open other repo paths).
- Report only what actually happened, with evidence. Do not speculate about code.

## MCP bridge (for MCP personas)
Start your own server session (pick the tool profile your persona says):
```
mkdir -p /tmp/jj/j/<persona>/out
(cd /tmp/jj/j/<persona> && nohup python3 /tmp/jj/j/mcpd.py serve /tmp/jj/<persona>.sock /tmp/jj/j/<persona>/log -- /tmp/j2p mcp --templates-dir /Users/seb/projects/json2pptx/templates --output /tmp/jj/j/<persona>/out [--tools all] > /tmp/jj/j/<persona>/bridge.log 2>&1 &)
python3 /tmp/jj/j/mcpd.py list /tmp/jj/<persona>.sock            # tool names + first line of description
python3 /tmp/jj/j/mcpd.py raw  /tmp/jj/<persona>.sock tools/list  # full tool definitions (large)
python3 /tmp/jj/j/mcpd.py call /tmp/jj/<persona>.sock <tool> '<json args>'      # or '@/path/to/args.json'
```
`call` prints structuredContent (or text), saves any image blocks to /tmp/jj/<persona>... `images/` next to the socket and prints their paths — open them with the Read tool to look at slides. Every call is logged to log/calls.jsonl (n, params, seconds, response_bytes, is_error) and log/resp-NNN.json; cite call numbers `n` as evidence. The server's `initialize` result (incl. its instructions) is in log/initialize.json — read that first, as a real client would show it to you.

## What to log (as you go, not from memory at the end)
Friction: anything where you were confused, had to guess, needed more than one attempt, got an error you could not act on in one step, received a payload far larger than you needed, waited long, could not find/see something (e.g. what a layout looks like before choosing it, how many items it takes), got contradictory guidance, or produced a slide that looked wrong although the tool said it was fine. Also log delights (things that worked noticeably well) — we want to keep those.

## Deliverables (in /tmp/jj/j/<persona>/)
1. `findings.json`: `{"persona":..., "metrics": {"tool_calls":N, "total_response_bytes":N, "attempts_to_first_valid_render":N, "wall_minutes":N, "enjoyment_1_to_10":N, "enjoyment_reasons":[...]}, "findings":[{"id":"A1","title":"<short, specific>","severity":"blocker|major|minor|polish","journey_stage":"onboarding|discovery|authoring|validation|render|review|repair|delivery","what_happened":"...","evidence":["call n=12: <short excerpt>", "path/to/file"],"expected":"...","suggestion":"..."}], "delights":[{"title":..., "evidence":[...]}]}`
2. `report.md`: a short narrative of the journey (what you tried in order, where time went), the top 5 things that would make an agent love this tool, and the final artefacts (pptx paths, slide images you inspected).
Keep evidence excerpts short. Aim for specific, deduplicated findings (quality over quantity; typically 8–20).
