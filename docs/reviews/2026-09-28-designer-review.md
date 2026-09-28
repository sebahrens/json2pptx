# Design review: json2pptx output (p-style and midnight-blue)

Abbreviations: SOA = p-style-state-of-ai, BD = board-deck, CL = consulting-layouts, PS = patterns-smoke, MB = midnight-blue. "Engine" means the fix belongs in the default settings, not in the deck JSON.

## P0: fix first (these make the output look generated)

**1. The takeaway band (the user's first complaint)**
Evidence: SOA 9, 12, 19. Code: `internal/generator/takeaway_note.go` (fill is accent1 lumMod 20% / lumOff 80%, `takeawayRuleWidth = 12700` gives a 1pt accent1 stroke, text is 16pt bold `#1F1F1F`). `internal/template/chrome_frame.go:87` sets gap = 0.7% of height.
Problems:
- A 1pt saturated stroke around a pale tint makes the band look like a form field or error box.
- The peach wash plus bold text at 16pt competes with the slide title.
- The band spans the full width and sits about 5pt above the footer, so it touches the footer text (SOA 12, 19).
- The text is only vertically centred. There is no left padding logic.
- The colour is a hard-coded hex, even though the engine rule is "never hardcode colours".
Change (engine):
- Stroke: none.
- Replace the box with an editorial "so-what" treatment: a 3pt accent1 left bar, flush and full band height, over a very light neutral fill (lt2, or dk1 at 4%). Alternatively use no fill and a 0.75pt dk1 rule above the band.
- Text: 14pt semibold/bold in dk1 (a scheme colour, not a hex), 12pt left inset after the bar, top-anchored.
- Gap to the footer: at least 12pt. Gap to the content above: at least 16pt.
Impact: this is the single most visible change. It also unifies with item 2.

**2. Four different callout styles in one engine**
Evidence:
- SOA 6: "So what" has a grey fill and a grey 4px left bar. This is the best of the four.
- SOA 9/12/19: peach fill with an orange stroke.
- PS exec-summary: a "BOTTOM LINE" chevron tag next to a peach strip.
- PS metric-list: a solid orange band with bold text.
Problem: the same rhetorical element gets four visual languages, which reads as assembled rather than designed.
Change (engine): one callout token set, used by the takeaway band, `appendCalloutRow` (`cmd/json2pptx/pattern_resolve.go:253`, currently a solid accent1 fill with 14pt lt1 text), chart-insights `so_what`, the exec-summary bottom line and the metric-list banner. Spec: the left-bar treatment from item 1, 14pt, no stroke, no chevron tag. Keep a solid accent fill only as an explicit `emphasis: "strong"` opt-in.

**3. Stop using strokes to separate white cards from white paper (the user's second complaint)**
Evidence:
- SOA 20: row 1 has white cards with a 0.5pt grey outline and row 2 has grey fills. The alternation comes from `paperSurfaceHairline`.
- SOA 13 / BD 4 / PS 4: matrix quadrants have a 0.75pt 50% grey border (`matrix2x2CellBorderJSON`).
- PS 3: the BMC has a 25% border.
- PS "Before vs After": outlined white rows.
- PS "Joint Engagement Team": dk2 0.75pt outlines.
- BD "Deal approval": outlined empty cells.
Code: `internal/patterns/overrides.go:17` plus call sites in cardgrid, quotecluster, comparison2col, swimlane, capability_heatmap, matrix2x2, bmccanvas, dual_org_ladder.
Problem: outlines add a second contour to every box and double the visual noise. Alternating outlined and filled rows looks like a bug.
Change (engine):
- Line: none by default on every filled card. A card on paper gets a neutral tint instead: lt2, or dk1 at 4–6% (for example #F2F2F2 on p-style).
- Alternating rows use two tints (4% and 8%), never stroke versus fill.
- The BMC and 2x2 grids are built from 4pt white gutters over a tinted field, not borders.
- Keep hairlines only as horizontal row rules in lists and tables (SOA 3 does this well).

**4. Content floats: no fixed top line for the body**
Evidence:
- SOA 20: the grid starts about 150pt below the title.
- SOA 17 and SOA 5: content is vertically centred.
- SOA 7/8/14: content hugs the top and the bottom 55% of the slide is empty.
- BD 2/6: the grid is vertically centred in a void.
Problem: the eye cannot find a consistent start line across the deck.
Change (engine): top-anchor every pattern at the content-zone top, 18–24pt below the title baseline. Pattern heights become content-sized (cap the stretch). Let leftover space fall to the bottom, or use it for the takeaway. Do not stretch cards to fill the height (SOA 2 agenda tiles are 125pt tall; SOA 5 KPI cards are 270pt tall for 60pt of content).

## P1: strong improvements

**5. Too many type sizes, and body text too small**
Evidence: about nine sizes are in use (≈100 / 32 / 26 / 16 / 14 / 12 / 11 / 10 / 9pt).
- CL 1–8, PS value-chain, SOA 18 descriptions and SOA 21 pillar bullets are about 9–10pt.
- SOA 17 bullets are 11pt inside a 250pt-tall box.
Change (engine):
- Set a five-step scale: Title 26–28 / Lead 18 / Subhead 14 bold / Body 12 / Caption 10. Minimum body size 11pt. The KPI value is a separate display size (40–48pt).
- Snap computed fit sizes to the scale instead of fractional sizes.

**6. Font pairing breaks between placeholders and shapes**
Evidence:
- SOA 7/8: placeholder bullets render in Georgia (serif).
- Every shape-grid text uses Arial (SOA 3, 17, 21).
- SOA 8: column headers ("Open-weight models") are set as ordinary bullets.
Problem: the serif is meant for display, but here it appears at body size on some slides and not others.
Change: serif only for titles, section titles and the KPI/stat display figure (optional). All body text uses the minor font. Two-column headers render as 14pt bold with no bullet and 6pt space after.

**7. KPI and stat typography**
Evidence:
- SOA 5: values sit at different heights (78%, 3.4x) because a variable-height value+label block is centred.
- The negative value uses a hyphen ("-80%").
- The grey top rule is detached from the card by a 2pt white gap (`internal/shapegrid/grid.go:847`, `gapEMU = 2pt`).
- SOA 6: in "+56 pts" the label sits directly under the descenders.
- SOA 15: "1 in 3", its label and its source are stacked with no spacing.
Change:
- Top-anchor the value on a shared baseline (fixed value box height). Use the true minus sign U+2212.
- Accent bar: gapEMU = 0 (flush), 3pt, in accent1 not grey.
- Space before the label: 4pt. Space before the source: 12pt. Source text: 9pt in dk1 at 60%.
- The value may take accent1 when only one KPI is emphasised.

**8. Tint palette is incoherent (warm and cool greys mixed, a muddy mid-tint)**
Evidence:
- Colours seen on one warm-orange template: #FFDCCE (15%), #FDF0EC (8%), #FFAA70 (≈60%, SOA 21 roof/foundation, SOA 12 totals), and cool blue-greys #A0A7B2 (SOA 2 agenda), #B5BCC4 (SOA 9 bars, SOA 5 rules), plus #EAEAEA and #F0F1F3.
- MB PS 2 (KPI 4-up) turns pink with a red rule on a blue deck.
Change (engine):
- One neutral ramp derived from dk1/lt1 (4, 8, 16, 40, 60%).
- One accent ramp (10, 20, 100%). Remove the 40–70% mid-tints entirely, since black text on #FFAA70 looks muddy.
- The accent rotation should not recolour sibling KPI patterns within a deck.

**9. Heavy black fills used as "structure"**
Evidence:
- SOA 11 and PS value-chain: solid black boxes.
- SOA 19 and PS option table: solid black header row with white cell gaps.
- SOA 12: the negative bar is solid black.
Problem: dk1 as a fill is the heaviest element on the slide and outweighs the accent.
Change:
- Table header: no fill, 11pt bold dk1, 1.5pt dk1 rule under the header, 0.5pt 20% rules between rows.
- Chain/stack boxes: neutral 16% tint with dk1 text, and the one highlighted step in accent1.
- The waterfall "negative" bar uses a neutral 60% grey or the template's semantic negative colour, never black.

**10. Clip-art geometry**
Evidence:
- SOA 13 / BD 4: fat orange block arrows as the matrix axes.
- SOA 17 / BD 5: a 90pt-wide full-height chevron between the before/after panels.
- SOA 18 / PS roadmap: a 12pt-thick timeline bar.
- SOA 21: white gutters notch about 5pt into the roof. This is a rendering bug.
- CL 1: the giant chevrons dominate the row.
Change:
- Axes: 1pt dk1 lines with a small arrowhead and 10pt caps labels.
- Before/after transition: a 24pt circle with an arrow glyph, or a 20pt chevron centred in the gutter.
- Timeline: a 2pt line with 8pt dots.
- Strategy house: gutters stop at the pillar top edge.

**11. Chart styling (SOA 6, PS revenue chart)**
Problem:
- The y-axis is a heavy black line with ticks and gridlines, and it is redundant because every bar is labelled.
- The x-axis has tick marks.
- The chart title is bold 10pt, crammed at the top left.
Change (svggen defaults):
- When data labels are on: no y-axis and no gridlines.
- Baseline: 0.75pt dk1. No tick marks.
- Category labels: 10pt dk1 at 70%. Data labels: 10pt, 4pt above the bar.
- Bar width: 60% of the band.
- Only the last/highlight bar in accent1, the others in the neutral 40% tint (the insight is "2026").

## P2: polish

**12. Letter-spaced caps labels are missing**
Evidence: "TODAY'S STATE", "CASE STUDY", "BOTTOM LINE" and the axis labels use no tracking.
Change: 9–10pt caps, +8% letter-spacing (spc=80), dk1 at 70% or accent1.

**13. Widows in titles and subtitles**
Evidence: SOA 1 "…where AI value / lands", SOA 16 "What leaders should / do", PS contacts "Jane / Smith".
Change: widen the text boxes (section titles use 70% width) and add a balance-wrap pass that avoids a single-word last line.

**14. Agenda styling**
Evidence: SOA 2 has 125pt blue-grey tiles with centred numbers. PS agenda-with-images has orange squares with thick grey separator bars.
Change: 36pt accent numerals in the serif, no tile, 0.5pt rules between rows, content-height rows.

**15. Mixed alignment inside cards**
Evidence: SOA 21 pillar heads are centred over left-aligned bullets. SOA 17 headers are centred over left bullets. CL KPI scorecard centres the headline over a left-aligned breakdown.
Change: left-align everything inside a card with a single 12pt inset. Centre only standalone numerals.

**16. Legends and whitespace**
Evidence: the SOA 19 and PS option-evaluation legends are spread across the full width, with the symbols at data-mark size.
Change: a compact legend with 8pt symbols and 9pt text, grouped at the left.

**17. Footer and grid**
Evidence: the footer text is centred on the slide while the content grid is left-aligned at 38px.
Change: this is a template decision. Recommend aligning the footer to the left margin, with no content closer than 12pt to it.

## Debate response

### Verdicts on the consultant's items
1. **Takeaway component.** AGREE-WITH-CHANGE: one renderer, spec C1 below. I reject their "title minus 4pt": at 22–24pt the takeaway would compete with the title, so fix it at 14pt.
2. **No outlines.** AGREE. Their hairline "0.5pt lt2" is too faint on lt2 surfaces. Use dk1 at 15% instead. The `FILLED_SHAPE_OUTLINED` lint is a good addition.
3. **Charts show the point.** AGREE: this is the same as my item 11. Waterfall change: totals in neutral 60% (dk1 at 60%), not dk2, which is black on p-style.
4. **Sources.** AGREE (content discipline). Design spec: a 9pt source line in dk1 at 60%, left-aligned to the grid, 6pt above the takeaway band or the footer.
5. **Empty panels.** AGREE, and it is consistent with the user's constraint: content-size the boxes, then middle-anchor the group. Tile cap 1.6x content. Chevron at most 0.4in.
6. **Tracker and agenda.** AGREE. Spec C5.
7. **Action titles.** AGREE. This is not a design issue.
8. **Light text on accent.** AGREE-WITH-CHANGE: see "Resolution" below. Always forcing lt1 is wrong for bright accents.
9. **Tables.** AGREE-WITH-CHANGE. "Stretch row height to fill" is a DISAGREE: padded rows look bloated. Keep content-height rows and middle-anchor the table with the takeaway beneath.
10. **Font mixing.** AGREE, same as my item 6.
11. **Closing slide.** AGREE. Spec C6.
12. **Footer.** AGREE; it is a template decision. For the board deck, chrome on by default.
13. **Name wrapping.** AGREE, same as my item 13.
14. **Midnight-blue stripes.** AGREE; this is a template margin fix.

### My item 4 (top-anchoring): withdrawn
Per the user's constraint, patterns stay middle-anchored in the body zone. What I keep:
- (a) Content-size boxes: no stretching to fill, tile cap 1.6x content.
- (b) Reduce the dead band under the title: the body zone starts 18pt below the title box bottom.
- (c) Reserve the takeaway and source bands before centring, so the group centres in the remaining zone.

### Converged specs
**C1 Takeaway / so-what / bottom line / metric-list banner** (one renderer, engine default):
- No stroke. Fill none by default. The optional `surface: tint` setting uses lt2 or dk1 at 5%.
- A 3pt accent1 bar on the left, flush, full band height.
- Text 14pt bold dk1, left-aligned, 12pt left inset from the bar, 4pt top/bottom inset, top-anchored, maximum 2 lines.
- Band width = content width. At least 16pt above the band, at least 12pt to the footer/source.
- An optional "So what" lead-in in accent1 (same size). No chevron tag, no solid fill by default.
- Replace the `TakeawayColor` hex with dk1.

**C2 No-outline rule:**
- Filled shapes in patterns get `line: none`.
- Separation comes from 4–6pt white gutters or two neutral tints (dk1 at 4% and 8%).
- Rules are allowed only as horizontal row dividers: 0.5pt dk1 at 15%. Heavier rules only as structural rules (1pt dk1 under a header, 2–3pt accent1 as a bar).
- A white card on white paper is not allowed; it gets the 4% tint.
- Lint: `FILLED_SHAPE_OUTLINED`.

**C3 Chart highlight/neutral default:**
- All series and bars neutral (dk1 at 35–40%). Highlighted indices in accent1. With no `highlight` given, a single-series time chart accents the last bar; a ranked bar chart accents the top one.
- Waterfall: totals neutral 60%, decreases accent1, increases neutral 35%.
- Data labels 10pt, 4pt above the bar, bold on highlighted bars.
- When labels are present: no y-axis, no gridlines. Baseline 0.75pt dk1, no ticks. Bar width 60% of the band.
- Unit caption at the top left, 10pt, dk1 at 70%.

**C4 Table:**
- Header: no fill, 11pt bold dk1, 1pt dk1 rule under it.
- Rows 12pt, with 0.5pt dk1-at-15% row rules. No zebra.
- First column bold. Numbers right-aligned. Cell inset 6pt vertical, 8pt horizontal.
- Highlight row: accent1 at 10% fill plus a 3pt accent1 left bar, no stroke.
- Never black header fills. dk2 is allowed only when the template's dk2 is not black.

**C5 Tracker and agenda:**
- Tracker: 9pt caps, +8% tracking, accent1, 6pt above the title.
- Agenda: rows with a numeral in the serif at 28pt accent1 plus a 16pt item, 0.5pt rules between rows, content-height rows, middle-anchored. No filled tiles.
- On repeat, the current item is dk1 bold and the others dk1 at 50%.

**C6 Closing:** a `next-steps` pattern (numbered rows: action · owner · date, plus a "Decisions requested" C1 band). Retire "Thank You" as the default closer.

### Resolution: light text on saturated accent vs neutral tints plus one accent
- **Discipline.** Neutral tints are the default; accent1 at 100% is used for one highlight element per slide. This agrees with the consultant's "one accent plus neutrals" and removes most saturated fills.
- **Where a saturated fill remains, choose the text colour by measured contrast, not "always lt1".** On p-style's orange (#FF5000), white measures about 3.3:1, which fails AA for body text. Black measures about 6.4:1.
- **Rule:** pick lt1 if it reaches 4.5:1 (or 3:1 at 14pt bold or larger), else dk1. On midnight-blue this gives white; on p-style body-size labels it gives black.
- **Better still,** avoid small text on saturated fills: put labels beside or below accent shapes.

### Remaining disagreements
- The takeaway size: they want title minus 4pt; I say 14pt fixed.
- Always-lt1 on accent fills: I say contrast-measured.
- Stretching table rows to fill: I say no.
- dk2 for fills: on p-style dk2 is black, so it should be neutral 60% instead.
