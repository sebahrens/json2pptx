# h-tech-data: a data-platform modernisation business case (MCP, `--tools all`)

You are a technology-and-data consulting team's AI assistant. The brief:

> Build the steering-committee business case for "Atlas Retail" (EUR 3.1bn revenue, 640 stores): replace the on-premise data warehouse with a cloud lakehouse. 11–13 slides on `modern-template`. Facts: today 14 source systems feed a 2009 warehouse through 1,900 hand-written ETL jobs; nightly load takes 9.5 hours and missed its 6 am SLA on 61 of the last 90 days; data quality: 23% of product records have a missing or inconsistent attribute; 4 FTE spend 60% of their time on reconciliations. Target architecture, top to bottom: consumption (Power BI, 3 data products, ML feature store), serving (semantic layer, governed marts), processing (Spark batch, streaming), storage (Delta lake, bronze/silver/gold), ingestion (CDC from 14 sources, event streaming), with security/governance and FinOps as cross-cutting. Three vendor options scored on cost, migration risk, skills availability, lock-in: Databricks, Snowflake, a native hyperscaler stack; recommend Databricks. Five-year TCO: current EUR 18.4M, target EUR 12.9M (saving EUR 5.5M); year-by-year run cost current 3.6/3.7/3.7/3.7/3.7 vs target 4.8/3.1/1.9/1.6/1.5 (year 1 includes migration). Migration roadmap: Foundation (Q1–Q2 2027: landing zone, governance, 3 pilot sources), Migrate (Q3 2027–Q2 2028: 14 sources, 1,900 jobs refactored to 400 pipelines), Optimise (Q3–Q4 2028: decommission warehouse); parallel tracks: data literacy for 300 analysts, FinOps guardrails. KPIs after: load window 9.5 h → 1.5 h, SLA misses 61 → under 3 per 90 days, product-record defects 23% → under 5%, reconciliation effort −70%. A cost driver view: run cost splits into compute 48%, storage 12%, licences 22%, people 18%. The ops console screenshot at `$J/assets/screenshot-ops-console.png` shows the nightly load dashboard; call out the SLA breach banner (roughly top-left) and the job queue (roughly centre). Ask: approve EUR 4.8M for year 1 and the Databricks contract.

Four slides are **required to be split or complex layouts**, exactly as described; say in your report how you expressed each one and how many attempts it took:

- **S1** — five-year run cost as a two-series chart (current vs target) on the LEFT, and on the RIGHT the "EUR 5.5M saved" number over a short three-row table (current / target / saving). One title, one source.
- **S2** — the target architecture as a tiered stack, with the two cross-cutting concerns shown as side rails, not as another tier.
- **S3** — the ops console screenshot with the two callouts on it, and the text beside it (what the dashboard shows, the 61-of-90 fact, the after-state KPI).
- **S4** — the KPI before/after as a side-by-side: four metrics, "today" and "target", aligned row by row, the improvement visible.

Also required: the vendor option matrix with Databricks recommended, the cost driver split as a visual (not a bullet list), the phased roadmap with its two parallel tracks, an action-titled executive summary, and a next-steps closer.

## Do

1. Start the bridge with `--tools all`. Read `log/initialize.json` and `tools/list`; follow what the product tells you to do first. Where the product offers a preview of a kind or a candidate, use it before authoring and say whether it helped.
2. Plan, author, validate, repair, render. Use `recommend_visual` for at least S2 and the cost-driver slide, and record whether its top candidate was what you used.
3. Render every slide, look at every image, repair what looks wrong (three rounds at most), finish the completion protocol.
4. Render the final spec on `p-style` (when `$REPO/templates/p-style.pptx` exists, otherwise `warm-coral`) and look at every slide again.
5. Save every spec revision you sent as `$J/<persona>/spec-vN.json` (or .yaml); keep the final PPTX paths in `report.md`.

## Measure

- For S1–S4: the shape you used, attempts, whether the callouts landed on the right spots, whether the stack's rails read as rails, whether the narrow column stayed readable.
- `--tools all` against the default: did the extra tools help or distract; which ones you used; how many bytes the first contact cost.
- Validate and render calls to the first `deterministic_ready: true`.
- Every slide the tools scored clean that looked wrong, with the image path.
