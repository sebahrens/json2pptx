# i-risk-assurance: an IT general controls assurance readout (MCP, default tool profile)

You are an internal-audit / risk-assurance team's AI assistant. The brief:

> Build the audit-committee readout for "Cobalt Insurance": the FY26 IT general controls (ITGC) and SOX-style assurance review across five domains. 12–14 slides on `forest-green`. Facts: scope was 5 domains, 48 key controls, 1,160 samples tested across 9 applications. Results by domain — access management: 14 controls, 3 exceptions, rating red; change management: 11 controls, 1 exception, amber; computer operations: 9 controls, 0 exceptions, green; program development: 6 controls, 1 exception, amber; third-party / cloud: 8 controls, 2 exceptions, red. Overall opinion: "partially effective". Seven findings (invent titles consistent with the domains), each with a severity (2 high, 3 medium, 2 low), an owner and a due date between Dec 2026 and Jun 2027; the two high findings are "privileged access not recertified for 3 of 9 applications" and "cloud provider SOC 2 reports not reviewed for 2 of 4 providers". Testing approach: plan (scoping, risk assessment), walkthroughs, design evaluation, operating-effectiveness testing, reporting. Year-on-year: exceptions 2024: 11, 2025: 9, 2026: 7; high-rated findings 2024: 4, 2025: 3, 2026: 2. Control maturity by domain on a 1–5 scale: access 2, change 3, operations 4, development 3, third-party 2; target 4 everywhere by FY28. Remediation plan: Q4 2026 quick wins (recertification run, SOC 2 review), Q1–Q2 2027 access tooling, Q3 2027 re-test. Management has accepted all seven actions. Ask: the committee notes the opinion and endorses the remediation plan.

Four slides are **required to be split or complex layouts**, exactly as described; say in your report how you expressed each one and how many attempts it took:

- **S1** — the results-by-domain board: five domains × (controls tested, exceptions, rating). The red / amber / green rating must be visible as colour or a marker, not just a word.
- **S2** — the seven-finding register as a table: title, severity, owner, due date. Seven rows plus a header is more than one slide of your table kind may hold: do what the product tells you; no finding may be dropped.
- **S3** — year-on-year trend chart (exceptions and high findings, 2024–2026) on the LEFT; on the RIGHT the overall opinion "Partially effective" as the headline, with two short bullets on what drove it.
- **S4** — control maturity today vs FY28 target by domain as a ladder or side-by-side, five domains aligned row by row.

Also required: scope and approach as a five-step process, the two high findings each on their own slide (what we found, why it matters, the action, owner, date — in a structured layout, not a bullet dump), the remediation roadmap, an action-titled executive summary, and a next-steps closer with the committee ask. Put the detailed testing statistics in an appendix.

## Do

1. Start the bridge with the default tool profile. Read `log/initialize.json` and `tools/list`; follow what the product tells you to do first.
2. Plan, author, validate, repair, render.
3. Render every slide, look at every image, repair what looks wrong (three rounds at most), finish the completion protocol.
4. Render the final spec on `p-style` (when `$REPO/templates/p-style.pptx` exists, otherwise `midnight-blue`) and look at every slide again.
5. Save every spec revision you sent as `$J/<persona>/spec-vN.json` (or .yaml); keep the final PPTX paths in `report.md`.

## Measure

- For S1–S4: the shape you used, attempts, whether the RAG distinction survived the render, what happened to the seven-row table, whether the narrow column stayed readable.
- The two structured finding slides: which kind you chose and whether the product offered one.
- The appendix: did the product understand "appendix" and number it apart.
- Validate and render calls to the first `deterministic_ready: true`.
- Every slide the tools scored clean that looked wrong, with the image path.
