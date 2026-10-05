# Storyline and structure

How a consulting deck argues, and how that maps onto a DeckSpec. Read before
authoring; the engine's gate (`score_deck`, `validate_deck_spec`) enforces the
mechanics below (`TITLE_NOT_ACTION`, `NO_EXECUTIVE_SUMMARY`,
`CLOSING_WITHOUT_NEXT_STEPS`, `takeaway_missing`, `DATA_WITHOUT_SOURCE`), but it
cannot tell a good argument from a bad one. Back to the [hub](README.md).

## 1. Ghost deck first

Write the titles alone, numbered, in order, and read them as one paragraph.
If that paragraph persuades, the deck will; if it lists topics, start again.
Each title is a full sentence with a verb and, where one exists, the number:

```
1. Project Falcon — Commercial due diligence proposal
2. A four-week CDD can price the two risks that decide the Nordbolt bid
3. The market adds EUR 1.2bn by 2028, but Nordbolt holds only 2.3% of it
4. EUR 6.5M of the EUR 7.0M EBITDA gain since 2023 is price
5. On Project Keel the same test moved the bid by EUR 40M
6. Scope B is the only option that prices both risks inside the window
…
12. Confirm scope B by Friday and we start in the data room on Monday
```

Budgets: `list_slide_kinds fields:["brief"] template:"<yours>"` returns the
measured `title` budget (about 67 characters, two lines, on the tightest
template; longer titles wrap to three lines on p-style's serif face and crowd
the content). A topic label ("Market overview", "Options comparison") is
reported as `TITLE_NOT_ACTION`; the point then sits in the takeaway, which is
backwards.

## 2. The argument shapes

**Pyramid (answer first).** Recommendation → the two or three reasons → the
evidence for each. The executive summary *is* the pyramid's top two layers; the
body slides are the evidence in the same order. Use this for proposals, IC
papers, steering-committee cases, audit-committee readouts — any audience that
decides.

**SCQA (situation, complication, question, answer).** For an audience that
needs to be walked to the problem: regulator findings, a cost base nobody has
looked at, a platform that is failing quietly. The first three body slides are
S, C and Q; the executive summary still leads with A. There is a `scqa-summary`
pattern (raw path) when the four belong on one slide.

**Options → criteria → recommendation.** The deals and risk-consulting staple:
`option_matrix` (options scored against criteria, one `recommended`) or
`decision` (3–6 options with one `recommended: true` and the ask in
`recommendation`). Show the criteria before the scores; never score only the
option you want.

**Before → after → path.** Transformation and technology cases: `comparison`
(today vs target, aligned row by row), then `roadmap` / `timeline`, then the
ask. The improvement must be visible in the comparison, not inferred.

## 3. The spine of a 10–14 slide deck

| Position | Kind | Rule |
|---|---|---|
| 1 | `title` | Deck name as the title, audience and date as the subtitle. |
| 2 | `executive_summary` | 3–5 `{lead, support}` points and a `bottom_line` (the ask). Leads are the action titles of the body, compressed. |
| 3 | context | One slide that sizes the situation: `kpi_snapshot` (2–6 numbers), `regions` (chart beside the number), or `stat`. |
| 4–n | evidence | One message per slide; the kind follows the message ([visual-vocabulary.md](visual-vocabulary.md)). Follow a dense slide (table, matrix) with a light one (stat, quote, chart). |
| n+1 | the decision | `option_matrix` or `decision`, recommended option marked. |
| n+2 | the plan | `roadmap`, `timeline` or `process`; owners and dates where they exist. |
| n+3 | credibility | `team` (proposals), `quote` (voice of customer), `image_case` (a comparable). Optional. |
| last | `next_steps` | 2–6 actions `{action, owner, date}` and 0–3 `decisions`. This is the closer; a "Thank you" slide is not. |
| after | appendix | `section` with `appendix: true`, then the backup tables; page numbers read A1, A2 and the main deck's `{total}` excludes them. |

Twelve slides or more: use `structure.sections` with `auto_agenda` and let the
compiler number the dividers; under twelve, dividers cost more than they give
and the rhythm checks say so.

## 4. Every evidence slide carries three things

- **An action title** (above).
- **A `takeaway`**: one line under the exhibit that says what the audience
  should conclude — not a restatement of the title, not a second message.
  Kinds with their own conclusion band use it instead: `executive_summary` →
  `bottom_line`, `decision` → `recommendation`, `chart_insight` → `insights[]`
  or `insight`; a duplicate `takeaway` there goes to the notes.
- **A `source`**: origin and base ("Regulator's 2025 review; management
  information 2023–2025"). `meta.source` sets the deck default; a data slide
  without any source is `DATA_WITHOUT_SOURCE`.

## 5. Chrome, confidentiality, numbering

`meta.chrome` sets the footer: `confidentiality`, `client`, `project_code`,
`date`, page numbers (on by default; title and closing skipped). The footer is
one line: a long client label plus a project code plus the date overflows and
is truncated with an ellipsis on narrow templates without a finding today
(`go-slide-creator-m2tlt`) — keep labels short and check the footer on the
second template.

```yaml
meta:
  title: FY26 IT General Controls Review
  template: forest-green
  date: October 2026
  source: Cobalt Insurance Internal Audit, FY26 ITGC review
  chrome:
    confidentiality: Confidential - Audit Committee
    client: Cobalt Insurance
slides:
  - kind: title
    title: FY26 IT General Controls Review
    subtitle: Audit Committee readout, October 2026
  - kind: executive_summary
    title: ITGCs are partially effective; two high findings need fixing by December
    points:
      - lead: 48 key controls across 5 domains were tested on 1,160 samples.
        support: Computer operations was fully effective; change and development were amber.
      - lead: Access management and third-party / cloud are rated red.
        support: Five of the seven exceptions sit in these two domains, including both high findings.
      - lead: The trend is improving.
        support: Exceptions fell from 11 to 7 and high findings from 4 to 2 since 2024.
      - lead: Management has accepted all seven actions.
        support: Quick wins land in Q4 2026, access tooling in H1 2027, re-test in Q3 2027.
    bottom_line: The committee is asked to note the opinion and endorse the remediation plan.
  - kind: next_steps
    title: Four actions start in October; two decisions are requested today
    actions:
      - {action: Launch the privileged-access recertification run, owner: Head of IAM, date: 15 Oct 2026}
      - {action: Review the two outstanding SOC 2 reports, owner: Head of Third-Party Risk, date: 30 Nov 2026}
      - {action: Approve the access-tooling business case, owner: CIO, date: Dec 2026}
    decisions:
      - "Note the FY26 ITGC opinion: partially effective"
      - Endorse the remediation plan and its Q3 2027 re-test
```

## 6. Rhythm

`analyze_deck_rhythm(deck_id)` and the `SEMANTIC_RHYTHM_*` warnings watch for
three of the same visual family in a row, one motif on more than half the
content slides, and dense runs (table → matrix → table). Break a run by
changing the *kind* of the middle slide, not its content: a KPI row, a stat,
a chart or a quote. Two halves of a split table are one exhibit when the second
is titled `<same title> (2/2)`; keep them adjacent.

## 7. Tone

Numbers with units and bases; "EUR 11.2M" not "11.2"; "61 of the last 90
nights" not "frequently"; name the owner and the date on every action; one
decimal where the source has one. Say what the audience must decide in the
first slide and the last.
