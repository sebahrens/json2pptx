# Playbook: risk assurance

Internal-audit and controls readouts: IT general controls (ITGC), SOX /
ICFR, third-party assurance (SOC reports), control-design reviews, audit
plans. The worked deck is an FY26 ITGC readout to an audit committee:
[`examples/semantic/playbooks/assurance-itgc-readout.yaml`](../../examples/semantic/playbooks/assurance-itgc-readout.yaml)
(13 slides plus appendix, `forest-green`, cross-rendered on `p-style`). Back to
the [hub](README.md).

## Audience and the ask

An audit committee: non-executives who read for the opinion, the exceptions,
the owners and the dates. They do not want the testing method explained twice;
they want to know what is red, why, who fixes it and when it is re-tested. The
ask is usually to *note* an opinion and *endorse* a plan. Tone: the audit
vocabulary (controls, samples, exceptions, design vs operating effectiveness,
severity, management action), neutral, dated.

## Storyline skeleton

```
1. FY26 IT General Controls Review
2. ITGCs are partially effective; two high findings need fixing by December     executive_summary
3. The review followed five steps from scoping to reporting                     process
4. Two of five domains are rated red: access management and third-party / cloud  option_matrix (text, text, rag)
5. Exceptions fell for a third year, but the opinion stays partially effective  regions (line + stat + text)
6. All seven findings have an owner and a date; the two high ones close by December   table
7. High: privileged access was not recertified for 3 of 9 applications          raw labeled-rows
8. High: SOC 2 reports were not reviewed for 2 of 4 cloud providers             raw labeled-rows
9. Access and third-party controls must climb two maturity levels by FY28       chart_insight (grouped horizontal bar)
10. Remediation runs in three phases and is re-tested in Q3 2027                 roadmap
11. Four actions start in October; two decisions are requested today             next_steps
12. Appendix                                                                      section (appendix: true)
13. Appendix: testing statistics by domain                                        table (A1)
```

Opinion first (2), method once (3), results (4–5), the register (6), the two
high findings in full (7–8), where this goes (9–10), the ask (11), the
evidence in the back (A1).

## Slide-by-slide blueprint

| # | Kind | What goes in | Watch |
|---|---|---|---|
| 2 | `executive_summary` | Scope in numbers; the red domains; the trend; management acceptance | `bottom_line`: note the opinion, endorse the plan. |
| 3 | `process` | Plan / walkthroughs / design evaluation / testing / reporting, one line each | Five steps with descriptions render as chevrons with a detail zone; labels ≤ 17 characters at five steps. |
| 4 | `option_matrix` | Domain rows; `Controls tested` and `Exceptions` as `text`, `Rating` as `rag`; `decisive_criterion: Rating` | No `detail` lines; red rows may also be `recommended` with `highlight_label: Red`. |
| 5 | `regions` `main_left` | Two-series line (exceptions, high findings, 3 years) + `stat` "Partially effective" + two bullets on what drove it | Opinion as the stat `value`, "Overall opinion, FY26" as its `label`. |
| 6 | `table` | Finding / severity / owner / due date, 7 rows | Up to nine rows on one slide; `highlight_column: Severity`. |
| 7–8 | raw `labeled-rows` | WHAT WE FOUND / WHY IT MATTERS / ACTION / OWNER AND DATE, `sublabel` for domain, risk, status, severity | One finding per slide; bold the control reference and the date. |
| 9 | `chart_insight` | `grouped_bar`, `orientation: horizontal`, today vs FY28 target per domain, 3 `insights` | Scale 1–5 in the chart title. |
| 10 | `roadmap` | Quick wins / access tooling / re-test with dates and 2–3 deliverables each | Re-test phase last, dated. |
| 11 | `next_steps` | Four actions with owner and date; two `decisions` (note / endorse) | Decisions phrased as the committee minutes will record them. |
| 12–13 | `section` + `table` | `appendix: true`; samples, applications, exceptions, exception rate per domain | Pages number A1; the main deck's total excludes them. |

## The assurance-specific visuals

**Results board** — counts and a rating on one row, per domain:

```yaml
meta:
  title: FY26 IT General Controls Review
  template: forest-green
  date: October 2026
  source: Cobalt Insurance Internal Audit, FY26 ITGC review
slides:
  - kind: option_matrix
    title: "Two of five domains are rated red: access management and third-party / cloud"
    criteria:
      - {label: Controls tested, scale: text}
      - {label: Exceptions, scale: text}
      - {label: Rating, scale: rag}
    options:
      - {name: Access management, scores: ["14", "3", red]}
      - {name: Change management, scores: ["11", "1", amber]}
      - {name: Computer operations, scores: ["9", "0", green]}
      - {name: Program development, scores: ["6", "1", amber]}
      - {name: Third-party / cloud, scores: ["8", "2", red]}
    decisive_criterion: Rating
    takeaway: 48 controls tested, 7 exceptions; overall opinion partially effective.
```

**Trend beside the opinion** — `regions` `main_left` with a line chart and a
`stat` whose value is the opinion text ([split-and-complex-layouts.md](split-and-complex-layouts.md) §1). Non-negative line and bar charts start their
axis at zero, so a 4 → 2 fall reads as what it is; set `data.y_min` /
`data.y_max` only when the audience expects a zoomed axis and say so in the
heading.

**One finding per slide** — the raw `labeled-rows` slide in
[split-and-complex-layouts.md](split-and-complex-layouts.md) §9. The four
labels are the audit standard; a fifth row ("Management response") fits when
the bodies are short.

**Findings register** — a plain `table`, severity column highlighted; seven
rows on one slide. Twelve or more findings: the high and medium ones on the
slide, the rest in the appendix table — not two half-empty slides.

**Appendix** — `section` with `appendix: true` is understood without further
instruction: unnumbered divider, A1-style pages, excluded from the agenda and
from the rhythm checks.

## Variants

| Engagement | Spine changes |
|---|---|
| SOX / ICFR year-end | Scope `kpi_snapshot` (key controls, locations, deficiencies) → results board by process → deficiencies `table` (severity: deficiency / significant / material weakness) → aggregation `matrix_2x2` (likelihood × magnitude) → remediation `roadmap` → opinion `next_steps`. |
| Third-party assurance (SOC 1/2 review) | Provider inventory `table` → coverage board `option_matrix` (`rag` per provider: report obtained / reviewed / CUECs mapped) → exceptions `labeled-rows` → `next_steps`. |
| Annual audit plan | Risk universe heat map (raw `capability-heatmap`) → plan `table` (audit, quarter, days) → resourcing `kpi_snapshot` → `decision` (approve plan). |
| Control design review (pre-implementation) | Process `process` or svggen `swimlane` → control points `table` → gaps `option_matrix` (`rag`) → design actions `next_steps`. |

## Review points for this audience

1. The opinion wording is identical on slides 2, 5 and 11.
2. Every finding in the register has a severity, an owner and a date; the two
   high findings on slides 7–8 match the register row for row.
3. Counts reconcile: exceptions on the results board (3+1+0+1+2 = 7) equal
   the register's rows and the appendix total.
4. The re-test date is on the roadmap and in the decisions.
5. On the second template the RAG dots keep their three distinct colours and
   the appendix pages still read A1, A2.
