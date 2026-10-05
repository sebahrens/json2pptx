# Playbook: risk consulting

Enterprise-risk uplifts, regulatory remediation programmes, risk-appetite and
three-lines-of-defence reviews, operational-resilience and third-party-risk
proposals. The worked deck is a 12-month ERM uplift proposal to a bank's
executive committee after a "needs improvement" regulatory rating:
[`examples/semantic/playbooks/risk-erm-uplift-proposal.yaml`](../../examples/semantic/playbooks/risk-erm-uplift-proposal.yaml)
(11 slides, `midnight-blue`, cross-rendered on `p-style`). Back to the
[hub](README.md).

## Audience and the ask

A CRO and an executive committee with a regulator's deadline. They need to see
that you understand the findings, that the current position is measurably
outside appetite, what the target model is, and which delivery option gets
them there in time. The ask is an option and a governance structure. Tone:
evidence from their own data (losses, appetite metrics, FTE), the regulator's
language, no scare words.

## Storyline skeleton (SCQA inside a pyramid)

```
1. Enterprise risk management uplift
2. Approve co-delivery now to close 14 findings before June 2027                executive_summary (answer)
3. 14 regulatory findings must be closed in nine months                         kpi_snapshot (situation)
4. Two of five risk appetite metrics are breached, one is amber                 option_matrix, rag (complication)
5. Operational losses rose 84% in two years to EUR 11.2M                        regions (complication, quantified)
6. Cyber is the only high-likelihood, high-impact risk; three more are high impact   raw capability-heatmap
7. The target model adds 16 FTE to the 2nd line and one integrated report       comparison (answer: the model)
8. Four pillars take risk governance from 'needs improvement' to effective      pillars (house)
9. Co-delivery at EUR 2.1M balances speed, cost and capability transfer         decision
10. Four phases and two parallel tracks finish before the 30 June 2027 deadline  roadmap (parallel_tracks)
11. Four actions start the programme this quarter                               next_steps
```

## Slide-by-slide blueprint

| # | Kind | What goes in | Watch |
|---|---|---|---|
| 2 | `executive_summary` | The rating; losses vs appetite; metrics breached; the option | `bottom_line`: the option and the steering committee chair. |
| 3 | `kpi_snapshot` | Findings, high findings, losses, deadline — 4 KPIs with `comparator` | Four cards leave the lower half empty on some templates (`go-slide-creator-18dqh`); consider `regions` with a chart if you have a series. |
| 4 | `option_matrix` | Risk type rows; `Status` (`rag`), `Current` and `Limit` (`text`); breached rows in `recommended` with `highlight_label: Breached` | No `detail` lines at five rows. |
| 5 | `regions` `main_left` | Loss bar chart; `stat` EUR 11.2M with the peer median in `context`; two bullets on causes | Peer comparison in the `context` line, not a second chart. |
| 6 | raw `capability-heatmap` | Columns = likelihood (low / medium / high) with `sublabel` counts; `tiers` = impact; each cell a named risk | The legend explains tiers; six risks placed, none on a line. |
| 7 | `comparison` | Three lines today vs target, then the weaknesses they close | Rows aligned: line 1 ↔ line 1. |
| 8 | `pillars` | `objective` in the roof, four pillars with two bullets each, `foundation` of a band + a 3-cell row | A house needs objective + foundation; otherwise it renders as panels. |
| 9 | `decision` | Advisory / co-delivery / outsource with price and one consequence each | `recommended: true` on one; the ask in `recommendation`. |
| 10 | `roadmap` | Four phases with `date_label` and one-line descriptions; two `parallel_tracks` | Tracks carry their own date span in the label; no milestones alongside tracks on short templates. |
| 11 | `next_steps` | Approve; constitute the committee; hiring plan; submit plan to regulator | Dates inside the quarter. |

## The risk-specific visuals

**Risk appetite dashboard as `option_matrix`.** One scale per criterion is
what makes it work: the first criterion is `rag`, the figures stay `text`.
Breached rows are `recommended` (the highlight mechanism), labelled
"Breached":

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Harbour Bank risk appetite statement; management information Q3 2026
slides:
  - kind: option_matrix
    title: Two of five risk appetite metrics are breached, one is amber
    criteria:
      - {label: Status, scale: rag}
      - {label: Current, scale: text}
      - {label: Limit, scale: text}
    options:
      - {name: Credit, scores: [green, NPL 2.1%, "3%"]}
      - {name: Liquidity, scores: [green, LCR 148%, "110%"]}
      - {name: Operational, scores: [red, EUR 11.2M, EUR 8M]}
      - {name: Conduct, scores: [amber, "37", "30"]}
      - {name: Cyber, scores: [red, "2", "0"]}
    recommended: [Operational, Cyber]
    highlight_label: Breached
    decisive_criterion: Status
    takeaway: Operational and cyber are breached; conduct is drifting towards its limit.
```

**Heat map with named risks.** A 2 × 2 `matrix_2x2` cannot place "medium"
honestly (the journey run saw four risks land on the axis lines). The
`capability-heatmap` pattern as a 3 × 3 — likelihood columns, impact tiers,
one risk per cell — is the honest rendering until a heat-map kind exists
(`go-slide-creator-ec74l`):

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Harbour Bank top-risk register, Q3 2026
slides:
  - kind: raw_json2pptx
    slide:
      slide_type: content
      layout_id: blank-title
      content:
        - placeholder_id: title
          type: text
          text_value: Cyber is the only high-likelihood, high-impact risk; three more are high impact
      pattern:
        name: capability-heatmap
        values:
          columns:
            - header: Low likelihood
              sublabel: 2 risks
              cells:
                - {text: Model risk, tier: 0}
                - {text: Climate, tier: 1}
            - header: Medium likelihood
              sublabel: 3 risks
              cells:
                - {text: Third-party outage, tier: 0}
                - {text: Conduct, tier: 1}
                - {text: Fraud, tier: 2}
            - header: High likelihood
              sublabel: 1 risk
              cells:
                - {text: Cyber, tier: 0}
          tiers:
            - {label: High impact, description: material loss}
            - {label: Medium impact, description: contained loss}
            - {label: Low impact, description: minor loss}
```

**Three lines of defence** is a `comparison` (today / target) when the point
is the change, and `pillars` (`foundation: ["Three lines of defence with clear
accountabilities", [People, Data, Technology]]`) when the point is the model.

## Variants

| Engagement | Spine changes |
|---|---|
| Regulatory remediation plan (post-inspection) | Findings `table` (ref, finding, severity, owner, date — ≤ 9 per slide, rest in appendix) → root causes `pillars` → remediation `roadmap` → governance `org` → `next_steps` with regulator submission date. |
| Risk appetite framework design | Appetite dashboard `option_matrix` → metric definitions `table` → cascade `architecture` (board → BU → desk as tiers, "reporting" as a rail) → `decision`. |
| Operational resilience / third-party risk | Important business services `table` → impact tolerances `kpi_snapshot` → scenario heat map (raw `capability-heatmap`) → gaps `comparison` → `roadmap`. |
| Model risk / climate risk | Inventory `kpi_snapshot` → tiering `matrix_2x2` → validation plan `timeline` → `next_steps`. |

## Review points for this audience

1. Appetite metrics show value *and* limit; a status dot without the figure
   is an opinion.
2. The heat map has every named risk inside a cell; count them in the image.
3. The option slide shows the cost of every option, not only the recommended
   one.
4. The roadmap ends before the regulator's deadline and the deadline is on the
   slide.
5. On `p-style`, the decisive-column tint and the breached-row tint can share a
   hue; check that the breached rows still read as distinct
   (`go-slide-creator-18dqh` family).
