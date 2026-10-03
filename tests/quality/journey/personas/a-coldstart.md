# a-coldstart: a board deck from a cold start (MCP, default tool profile)

You have never used json2pptx. A user hands you this brief:

> Build the Q3 FY26 board update for Northwind Logistics, 6-8 slides. Revenue was EUR 48.2M, up 12% year on year. EBITDA margin was 14.1% against a plan of 13%. On-time delivery was 96.4%. Quarterly revenue from Q3 FY25 to Q3 FY26: 43.0, 44.1, 45.6, 46.9, 48.2. SMB churn rose from 2.2% to 3.1%. There are three options to fix SMB churn: do nothing, a dedicated success team for EUR 1.2M, or a self-serve portal for EUR 2.4M; we recommend the success team. Plan: hire in Q4 FY26, pilot in Q1 FY27, full rollout in Q2 FY27. The ask: approve EUR 1.2M and eight hires.

## Do

1. Start the bridge with the default tool profile (no `--tools`). Read `log/initialize.json` and `tools/list`, then follow what the product tells you to do first.
2. Plan, author, validate, repair and render the deck on the template `midnight-blue`.
3. Render every slide (`render_deck_thumbnails`), look at every image, repair what looks wrong, and finish the product's completion protocol.
4. Deliver the same deck on a second template (`p-style` when `$REPO/templates/p-style.pptx` exists, otherwise `modern-template`), keeping the first deck as it is. Look at every slide again.

## Measure

- Bytes read before the first slide was authored (initialize + tools/list + every discovery call).
- Validate calls and render calls until the first render with `deterministic_ready: true`.
- Calls and content cuts the second template cost.
- Every place where a slide looked wrong although the tools reported it clean.
