# g-risk-consulting: an enterprise-risk transformation proposal for a bank (MCP, default tool profile)

You are a risk-consulting director's AI assistant. The brief:

> Build the proposal for "Harbour Bank" (mid-sized retail bank, EUR 42bn assets, 3,800 staff): a 12-month enterprise risk management (ERM) uplift after the regulator's 2025 review rated the bank "needs improvement" on risk governance. 10–12 slides on `midnight-blue`. Facts: the regulator raised 14 findings — 3 high (risk appetite not cascaded, 2nd line under-resourced, no integrated risk reporting), 6 medium, 5 low; remediation deadline 30 June 2027. Current operational-risk losses: 2023 EUR 6.1M, 2024 EUR 8.4M, 2025 EUR 11.2M; peer median is about 0.02% of assets. Risk appetite dashboard today: credit — within appetite (NPL 2.1% vs limit 3%), liquidity — within (LCR 148% vs 110%), operational — breached (losses EUR 11.2M vs EUR 8M), conduct — amber (37 complaints per 10k vs 30), cyber — breached (2 critical unpatched vulnerabilities older than 30 days vs 0). Three lines of defence: 1st line business units own controls (1,200 control owners), 2nd line Risk & Compliance (28 FTE, target 44), 3rd line Internal Audit (12 FTE). Target operating model has four pillars: governance and appetite, risk identification and assessment, control framework, reporting and data. Three delivery options: (1) advisory only, EUR 0.9M, we design and the bank implements; (2) co-delivery, EUR 2.1M, joint team; (3) full outsource of the programme office, EUR 3.4M. Recommend option 2. Roadmap: Mobilise (Jul–Aug 2026), Design (Sep–Nov 2026), Build (Dec 2026–Mar 2027), Embed (Apr–Jun 2027); parallel tracks: data and reporting platform, training 1,200 control owners. Top risks on a likelihood/impact view: cyber (high/high), third-party outage (medium/high), conduct (medium/medium), model risk (low/high), climate (low/medium), fraud (medium/low). Ask: approve option 2 and a steering committee chaired by the CRO.

Four slides are **required to be split or complex layouts**, exactly as described; say in your report how you expressed each one and how many attempts it took:

- **S1** — the risk appetite dashboard: five risk types with status (within / amber / breached), the metric and the limit. It must be readable as a status board, not as a plain bullet list, and the breached rows must be visibly distinguished.
- **S2** — op-risk losses as a chart (2023–2025) on the LEFT and, on the RIGHT, the headline number "EUR 11.2M" with the peer-median comparison and two short bullets on why losses are rising.
- **S3** — a likelihood × impact heat map with the six named risks placed in it. Use whatever the product offers (matrix kind, framework, pattern); state whether all six landed where they belong.
- **S4** — current vs target for the three lines of defence: left "today" (1,200 owners / 28 FTE / 12 FTE, three weaknesses), right "target" (44 FTE in the 2nd line, integrated reporting, cascaded appetite). Two columns, aligned row by row.

Also required: the four-pillar operating model, the three-option decision with option 2 recommended, the phased roadmap with its two parallel tracks, an action-titled executive summary, and a next-steps closer.

## Do

1. Start the bridge with the default tool profile. Read `log/initialize.json` and `tools/list`; follow what the product tells you to do first.
2. Plan, author, validate, repair, render.
3. Render every slide, look at every image, repair what looks wrong (three rounds at most), finish the completion protocol.
4. Render the final spec on `p-style` (when `$REPO/templates/p-style.pptx` exists, otherwise `forest-green`) and look at every slide again.
5. Save every spec revision you sent as `$J/<persona>/spec-vN.json` (or .yaml); keep the final PPTX paths in `report.md`.

## Measure

- For S1–S4: the shape you used, attempts, whether the status / highlight distinction survived the render, whether the narrow column stayed readable, and whether anything degraded to bullets (`SEMANTIC_PATTERN_DEGRADED`) without you asking.
- How you found the heat map (what you searched, which tool answered).
- Validate and render calls to the first `deterministic_ready: true`.
- Every slide the tools scored clean that looked wrong, with the image path.
