# Deck archetypes: the canonical skeletons, mapped onto kinds

What a partner expects to see, in what order, for the nine deliverables
agents are most often asked for — each as a slide skeleton naming the
DeckSpec kind for every slide, with a ghost deck (the action titles alone)
to copy the shape of. Lengths are for the main body; the appendix is extra.
Back to the [hub](README.md); the argument shapes behind these are in
[storyline-and-structure.md](storyline-and-structure.md), the four
domain playbooks go one level deeper.

**Engine archetypes.** `meta.archetype` takes `board_update`, `qbr`,
`sales_pitch`, `strategy_proposal`, `project_roadmap` or `market_analysis`
(`list_deck_archetypes`, hidden but callable). It sets a default template
(`midnight-blue`, `midnight-blue`, `warm-coral`, `forest-green`,
`modern-template`, `forest-green`) and, for the first four, marks the deck
*executive*: the rhythm checks then expect a synthesis slide
(`executive_summary` or `decision`). The nine skeletons below map onto those
six as noted; `plan_deck(brief, audience, slide_budget, format:"deckspec")`
drafts any of them from a brief and `explain_deck_spec` reads the rhythm and
layout coverage back without rendering.

## 1. Proposal / pitch — `sales_pitch`, 12–20 slides

The client's situation in their words, then the approach, then why us, then
the ask. Answer-first: the second slide already says what the engagement
delivers.

| # | Kind | Carries |
|---|---|---|
| 1 | `title` | Project name; audience and date in the subtitle |
| 2 | `executive_summary` | What we would do, why it decides the question, what it costs, `bottom_line` = the decision requested |
| 3 | `kpi_snapshot` or `regions` | Our understanding: the situation sized in 3–5 numbers (S and C of SCQ) |
| 4 | `chart_insight` / `regions` | The one analysis that shows we already see the issue |
| 5 | `option_matrix` | Scope options against the client's criteria, one `recommended` |
| 6 | `process` or `roadmap` | Approach: phases, deliverables, decision points |
| 7 | `timeline` | Workplan in weeks; `end_date` on the phases that overlap |
| 8 | `image_case` or `quote` | A comparable engagement with 2–3 result `metrics`, or the client's voice |
| 9 | `team` | 3–6 named people, role on every card |
| 10 | `table` | Fees and deliverables, `totals_row: true` |
| 11 | `next_steps` | Mobilisation actions with owner and date; `decisions` = confirm scope, sign |

Ghost deck: *A four-week CDD can price the two risks that decide the bid* →
*The market adds EUR 1.2bn by 2028, but the target holds 2.3% of it* → *EUR 6.5M
of the EUR 7.0M EBITDA gain is price* → *Scope B is the only option that
prices both risks inside the window* → *Four weeks from data room to IC pack*
→ *On Project Keel the same test moved the bid by EUR 40M* → *A team that has
done three fastener diligences* → *Eight deliverables make up the EUR 320k fee*
→ *Confirm scope B by Friday and we start on Monday*. The worked deck is
[playbook-consulting-pitch-and-deals.md](playbook-consulting-pitch-and-deals.md).

## 2. Strategy recommendation — `strategy_proposal`, 15–25 + appendix

The pyramid in full: answer, reasons, evidence, then the plan and the asks.

| # | Kind | Carries |
|---|---|---|
| 1 | `title` | |
| 2 | `executive_summary` | The recommendation and its 3–4 reasons; `bottom_line` = decisions |
| 3 | `agenda` | 12+ slides: the chapters, `current` left unset here |
| 4–5 | `regions`, `chart_insight`, `stat` | Context: where the value is, sized |
| 6–9 | `bridge`, `table`, `matrix_2x2`, `framework` | Diagnostic: the drivers (bridge), the segments (table), the positions (2×2), the structure (SWOT / five forces) |
| 10 | `comparison` or `decision` | Options considered; `decision` when the ask is one of them |
| 11 | `option_matrix` | Options scored on the criteria the board set, `recommended` |
| 12 | `pillars` (house) or `architecture` | The target model the recommendation builds |
| 13 | `kpi_snapshot` or `bridge` | Impact: the business case in 3–5 numbers or a value walk |
| 14 | `roadmap` | Phases with `parallel_tracks`; `milestone` on the decision points |
| 15 | `risk_heatmap` or `table` | Risks and mitigations with owners |
| 16 | `next_steps` | Decisions requested, then the first 90 days |
| A | `section` `appendix: true` + `table`s | Backup: full P&L, methodology, interview list |

Use `structure.sections` with `auto_agenda` so dividers and the tracker come
from the compiler; set `meta.chrome.tracker: true` for the section marker
above each title.

## 3. Due diligence / investment committee paper — `strategy_proposal`

Red-flag report: 8–12 slides, deal-breakers only. Full CDD: 25–40 body.
IC paper: 10–15.

| Red-flag report | IC paper |
|---|---|
| `title` | `title` |
| `executive_summary`: deal conclusion, the 3–5 findings that matter, implications for price and SPA | `executive_summary`: thesis as 2–3 falsifiable claims ("what must be true"), headline returns, top 3 risks, `bottom_line` = approve / approve with conditions / decline |
| `option_matrix` with `rag`: key-issues dashboard (issue × severity, price impact, SPA impact) | `kpi_snapshot`: entry multiple, equity cheque, IRR, MOIC, leverage |
| one slide per red flag: raw `labeled-rows` (ISSUE / EVIDENCE / IMPACT ON PRICE OR STRUCTURE / NEXT STEP) | `regions` `main_left`: market size chart + target share `stat` + `text` |
| `bridge`: reported → adjusted EBITDA (the quality-of-earnings walk) | `bridge`: value creation from entry to exit EBITDA |
| `table`: net debt and debt-like items; NWC peg | `comparison`: management case vs adjusted case, row by row |
| `next_steps`: what closes each flag, by whom, before signing | `option_matrix`: exit routes or structures; `decision` when one is proposed |
| | `table`: returns in base / upside / downside with IRR and MOIC |
| | `risk_heatmap` or `table`: risks tied to data-room evidence, with mitigants |
| | `next_steps`: approval sought, conditions precedent, 100-day plan as the appendix |

## 4. Business case — `strategy_proposal`, 15–25

The Five Case Model (HM Treasury Green Book) is the structure finance
committees recognise even outside government: strategic, economic,
commercial, financial, management. Each case is a chapter.

| Case | Kinds | Carries |
|---|---|---|
| Strategic | `kpi_snapshot`, `image_case`, `stat` | The case for change in numbers; the evidence on the screen; SMART objectives as a `comparison` (today vs target) |
| Economic | `comparison` (long list → short list), `option_matrix` (short list incl. business-as-usual and do-minimum, `decisive_criterion`), `bridge` or `chart_insight` (NPV / payback), `table` (sensitivity on the 2–3 assumptions that move the answer) | The preferred option and why the baseline loses |
| Commercial | `process` or `timeline` | Procurement route, contract shape, milestones |
| Financial | `regions` `main_left` (run cost chart + saving `stat` + TCO `table`), `table` (5-year TCO: licences, implementation, migration, integration, change, run, internal labour) | Affordability year by year |
| Management | `roadmap` with `parallel_tracks`, `org`, `table` (benefits: measure, baseline, target, date, owner), `risk_heatmap` | Governance, benefits realisation, risk |
| Close | `next_steps` | Approval sought at this stage (SOC / OBC / FBC), next gate |

The worked deck is [playbook-technology-and-data.md](playbook-technology-and-data.md).

## 5. Steering committee / status update — `project_roadmap`, 8–12

Decidable in fifteen minutes: the overall status and the decisions on page
two, every amber or red with a root cause and a dated action.

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  archetype: project_roadmap
  source: Harbour Bank programme office, October 2026
  chrome:
    confidentiality: Confidential - Steering Committee
    client: Harbour Bank
slides:
  - kind: title
    title: ERM uplift
    subtitle: Steering committee, 14 October 2026
  - kind: executive_summary
    title: Amber on schedule, green on spend; two decisions are needed today
    points:
      - lead: Fourteen of twenty-two month-three milestones are met.
        support: Both slips are in hiring and are recovered by December if the October wave lands.
      - lead: Spend is EUR 1.9M against a EUR 2.1M budget.
        support: The vendor contract signed five weeks late moved EUR 0.3M into Q4.
      - lead: Two risks need executive input.
        support: Second-line hiring and the data platform cutover date.
    bottom_line: Approve the external recruiter and the 1 December cutover.
  - kind: option_matrix
    title: Three workstreams are green; hiring is amber with a recovery plan in place
    criteria:
      - {label: Status, scale: rag}
      - {label: This month, scale: text}
      - {label: Root cause / action, scale: text}
    options:
      - {name: Appetite cascade, scores: [green, "4 of 4 done", "-"]}
      - {name: Control framework, scores: [green, "5 of 5 done", "-"]}
      - {name: Second-line hiring, scores: [amber, "31 of 44 in post", "Recruiter approved today"]}
      - {name: Data platform, scores: [green, "Contract signed", "Cutover 1 Dec"]}
    recommended: [Second-line hiring]
    highlight_label: Needs decision
    decisive_criterion: Status
    takeaway: One amber, one owner, one date.
  - kind: regions
    title: Milestones run two behind plan; spend is inside budget
    arrangement: columns
    regions:
      - kind: chart
        size_pct: 60
        heading: Milestones met vs plan
        chart:
          type: line
          data:
            categories: [Jul, Aug, Sep]
            series:
              - {name: Plan, values: [4, 10, 16]}
              - {name: Actual, values: [4, 9, 14]}
            highlight: [Actual]
      - kind: kpis
        kpis:
          - {value: EUR 1.9M, label: spent to date, comparator: budget EUR 2.1M}
          - {value: "2", label: milestones behind plan, comparator: recovered by Dec}
    takeaway: The gap opened in August and has not widened since.
  - kind: next_steps
    title: Two decisions today; four actions before the November committee
    actions:
      - {action: Appoint the external recruiter for the remaining 13 roles, owner: CRO, date: 21 Oct 2026}
      - {action: Confirm the 1 December cutover with the vendor, owner: Programme director, date: 28 Oct 2026}
      - {action: Re-baseline the hiring milestones, owner: PMO, date: 31 Oct 2026}
    decisions:
      - Approve the external recruiter (EUR 120k, inside contingency)
      - Approve the 1 December data platform cutover
```

Add `risk_heatmap` or a `table` for the top 3–5 risks needing executive
input, a `roadmap` for the next period, and a `table` actions log when the
committee tracks one.

## 6. Board paper — `board_update`, 5–10 + appendix

Purpose first (for decision / discussion / noting), the resolution, then
background and options.

| # | Kind | Carries |
|---|---|---|
| 1 | `title` | Paper title; `eyebrow` = "For decision" |
| 2 | `executive_summary` | One-page summary; `bottom_line` = the resolution proposed |
| 3 | `kpi_snapshot` or `stat` | Background in numbers |
| 4 | `option_matrix` or `decision` | Options and analysis; the recommended one marked |
| 5 | `comparison` or `table` | Implications: financial, legal, risk, reputational |
| 6 | `next_steps` | The resolution restated as `decisions`; implementation actions |
| A | appendix | Detail the board may ask for |

The same skeleton with a `quote` or `image_case` slide and `qbr` as the
archetype is the quarterly business review.

## 7. Audit / assurance readout — 10–20

Opinion first, then the findings in a fixed order, one page per finding.

| # | Kind | Carries |
|---|---|---|
| 1 | `title` | |
| 2 | `executive_summary` | Scope in one line, the overall opinion, the two or three findings that drive it, `bottom_line` = what the committee is asked to note and endorse |
| 3 | `process` | Scope and approach (plan → walkthroughs → design → operating effectiveness → reporting) |
| 4 | `option_matrix` (`text` counts + `rag` rating) | Results by domain |
| 5 | `regions` `main_left` | Trend chart + opinion `stat` + `text` |
| 6 | `risk_heatmap` or `table` | Findings by rating, or the register with owner and date |
| 7–n | raw `labeled-rows` | One finding per slide: CONDITION / CRITERIA / CAUSE / CONSEQUENCE / ACTION — owner, date and management response in the last row |
| n+1 | `chart_insight` or `comparison` | Themes; maturity today vs target |
| n+2 | `roadmap` | Remediation plan with the re-test phase |
| n+3 | `next_steps` | Actions with owners; `decisions` = note the opinion, endorse the plan |
| A | `table` | Testing statistics, rating definitions, exclusions |

The worked deck is [playbook-risk-assurance.md](playbook-risk-assurance.md).

## 8. Transformation roadmap — `project_roadmap`, 15–25

| # | Kind | Carries |
|---|---|---|
| 1–2 | `title`, `executive_summary` | Ambition, value at stake, the first wave, the asks |
| 3 | `kpi_snapshot` or `stat` | The case for change |
| 4 | `comparison` (`connectors: true`, `highlight_column: right`) | Today vs target state, metric by metric |
| 5 | `pillars` (house) or `architecture` | The target model |
| 6 | `bridge` or `chart_insight` | Value at stake by lever |
| 7 | `matrix_2x2` or raw `framework-grid` | The initiative portfolio: impact × effort, or levers by dimension |
| 8 | `roadmap` with `parallel_tracks` and `milestone`s | Waves with dependencies |
| 9 | `table` or `process` | Wave 1 in detail |
| 10 | `org` | Governance: sponsor, steering committee, PMO, decision rights |
| 11 | `table` | Benefits and KPIs: measure, baseline, target, date, owner |
| 12 | `risk_heatmap` | Risks |
| 13 | `next_steps` | Decisions and the first 90 days |

The worked deck is [playbook-risk-consulting.md](playbook-risk-consulting.md).

## 9. Workshop pre-read — `read` viewing mode, 6–15

Sent ahead and read alone, so every slide carries complete sentences and the
exhibits explain themselves. Set `meta.viewing_mode: read`.

| # | Kind | Carries |
|---|---|---|
| 1 | `title` | `eyebrow` = "Pre-read"; subtitle = the session and date |
| 2 | `executive_summary` | Purpose, the decisions the workshop must reach, what has been done |
| 3 | raw `text-sidebar` | Context as prose with one key message in the sidebar |
| 4–n | `chart_insight`, `table`, `comparison` | Facts and analysis to date, each with its `source` |
| n+1 | `decision` or `comparison` | Hypotheses or options on the table, none marked recommended yet |
| n+2 | `agenda` | The session plan; `current` unset |
| n+3 | `next_steps` | Preparation asked of each attendee, by name |

## Choosing length and mode

Decks that are **read** (pre-reads, board papers, IC papers, assurance
reports) carry dense, complete slides and a `source` on every exhibit; decks
that are **presented** (pitches, steering committees) carry one statement per
slide and leave the detail to the appendix. `meta.viewing_mode` tells the
engine which; the number of slides follows the number of messages, never a
target. A deck of twelve or more slides gets `agenda` and section dividers
(`structure.sections`, `auto_agenda`); under twelve they cost more than they
give and the rhythm checks say so.
