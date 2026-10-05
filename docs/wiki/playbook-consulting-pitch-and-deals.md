# Playbook: consulting pitch and deals

Proposals, pitches, commercial / operational due diligence, information
memoranda, investment-committee papers, 100-day plans. The worked deck is a
four-week commercial due diligence proposal to a private-equity fund:
[`examples/semantic/playbooks/deals-cdd-proposal.yaml`](../../examples/semantic/playbooks/deals-cdd-proposal.yaml)
(12 slides, authored on `p-style`, cross-rendered on `blue-corporate`). Back to
the [hub](README.md).

## Audience and the ask

A deal team under time pressure: a partner, a principal, an associate who
will read it on a phone. They decide one thing — scope and price — by a date
the exclusivity window sets. The deck therefore leads with the two risks that
decide the bid, shows the one comparable that proves you have done it before,
prices three scopes against each other, and asks for a signature by Friday.
Tone: numbers first, no adjectives, every claim sourced.

## Storyline skeleton (ghost deck)

```
1. Project Falcon — Commercial due diligence proposal
2. A four-week CDD can price the two risks that decide the Nordbolt bid      executive_summary
3. The market adds EUR 1.2bn by 2028, but Nordbolt holds only 2.3% of it     regions (chart + stat + text)
4. EUR 6.5M of the EUR 7.0M EBITDA gain since 2023 is price                  regions (waterfall + text)
5. On Project Keel the same test moved the bid by EUR 40M                     image_case
6. Scope B is the only option that prices both risks inside the window        option_matrix
7. C adds channel coverage, but costs a week the exclusivity window does not have   comparison
8. Eight deliverables over four weeks make up the EUR 320k fee                table
9. Four weeks from data room to IC pack                                       timeline
10. A team that has done three fastener diligences before                     team
11. Confirm scope B by Friday and we start in the data room on Monday         next_steps
```

Read it top to bottom: situation (3), the risk (4), credibility (5), the
options (6–7), what it costs and when (8–9), who (10), the ask (11). That is
the pyramid with the answer in slide 2.

## Slide-by-slide blueprint

| # | Kind | What goes in | Watch |
|---|---|---|---|
| 1 | `title` | Code name; "Commercial due diligence proposal, <month>" | `meta.chrome.confidentiality` and `client` set once. |
| 2 | `executive_summary` | 4 points: the target in one line; the risk; what the scope tests; the ask | `bottom_line` is the ask with the date. |
| 3 | `regions` `main_left` | Market bar chart (3 years incl. forecast) left; share as `stat`, position bullets as `text` right | Accent the forecast bar; share number large. |
| 4 | `regions` `columns` | EBITDA bridge as a waterfall `chart` (66%) + `text` with the implication (34%) | Totals must sum; check the opening bar. |
| 5 | `image_case` | Site photo, `eyebrow` "Comparable: …", `heading`, body, 3 bullets, 2 `metrics` | A real image path; no placeholder frames. |
| 6 | `option_matrix` | 3 scopes × 4 criteria on the harvey scale, `recommended: B`, `decisive_criterion` | `detail` per option carries price and duration. |
| 7 | `comparison` | The two finalists, 3 aligned rows | Title states the trade-off. |
| 8 | `table` | Phase / deliverable / days / fee, 8 rows + `totals_row` | Nine data rows is the limit; right-align numbers. |
| 9 | `timeline` | Week 1–4 stops with one line each | Dates as "Week 1"; a `body` per stop. |
| 10 | `team` | Partner with `photo`, others with initials `photo_label`, role + one-line bio | Bios within two lines. |
| 11 | `next_steps` | 5 actions with owner and date; 2 `decisions` | The Friday ask is decision 1. |

## The deals-specific splits

- **Market + share** (slide 3) and **bridge + implication** (slide 4) are the
  two splits every CDD and IM has; both are `regions`
  ([split-and-complex-layouts.md](split-and-complex-layouts.md) §1–2).
- **Fee table** on one slide: eight rows plus a total fit since the compact row
  pitch; the first journey run had to split it into two half-empty slides.
- **Option matrix then comparison**: score all options, then compare the two
  that survive. Never show only the recommended option.

## Variants

| Document | Change the spine to |
|---|---|
| Information memorandum (sell side) | `executive_summary` → investment highlights as `kpi_snapshot` → market `regions` → business model `pillars` or `framework: bmc` → financials `table` + `bridge` → management `team` → process `timeline`. No `decision`; the closer is the process timetable. |
| IC paper (buy side) | Thesis `executive_summary` → `kpi_snapshot` (entry multiple, IRR, MOIC, leverage) → value-creation `bridge` → key risks `option_matrix` (`rag` likelihood, text mitigation) → exit routes `comparison` → `next_steps` (approvals). |
| 100-day plan | `kpi_snapshot` (day-1 baseline) → workstreams `pillars` → `roadmap` (30/60/100) with parallel tracks (raw `roadmap-phased`) → governance `org` → `next_steps`. |
| Pitch for a strategy engagement | Same spine; slide 4 becomes the client's own issue tree (`driver-tree`, raw) or a `matrix_2x2` of options, and slide 5 two or three `quote`s from references. |

## Review points for this audience

1. Every number in the executive summary appears again on a body slide with a
   source.
2. The recommended scope is visibly marked (badge or fill) on the matrix *and*
   named in the comparison title.
3. Fees and days reconcile to the total row; the total appears in the title.
4. Title length: p-style's serif face wraps at about 60 characters; a title
   that fits on `p-style` may still wrap to two lines of capitals on
   `blue-corporate` and crowd the body — check the second render.
5. Footer: client label + date fit on one line on both templates.

## Render

```bash
json2pptx semantic validate examples/semantic/playbooks/deals-cdd-proposal.yaml --templates-dir templates
json2pptx semantic render   examples/semantic/playbooks/deals-cdd-proposal.yaml --templates-dir templates --out /tmp/falcon.pptx
json2pptx semantic render   examples/semantic/playbooks/deals-cdd-proposal.yaml --templates-dir templates --template blue-corporate --out /tmp/falcon-bc.pptx
```
