# Template Programmability Matrix

This document records, per template in `templates/`, whether it is **programmable** (regenerable from `cmd/mktemplate`) or **designer-owned** (must be repaired in place, not regenerated). It exists to prevent future agents from auto-regenerating designer artefacts and overwriting their visual identity — embedded PNG/SVG/WDP decorative assets, custom per-layout decorative shapes, intentional theme polarities, and bespoke layout naming cannot be reproduced from the `mktemplate` theme definitions.

Programmability is independent of conformance. A template that fails `json2pptx template-check` may still be designer-owned and must be repaired in place; see `docs/TEMPLATE_SPEC.md` for the conformance spec and `CONTRIBUTING.md` for the repair workflow.

## Matrix

| Template | Owner | Programmable? | Conformance (`template-check`) | Notes |
|---|---|---|---|---|
| `forest-green.pptx` | mktemplate | Yes | PASS (1 WARN — Section Number placeholder name) | Programmatic template, defined in `cmd/mktemplate/main.go`. Regenerable via `go run ./cmd/mktemplate -name forest-green -out templates/forest-green.pptx`. First-class supported template. |
| `midnight-blue.pptx` | mktemplate | Yes | PASS (1 WARN — Section Number placeholder name) | Programmatic template, defined in `cmd/mktemplate/main.go`. Regenerable via `go run ./cmd/mktemplate -name midnight-blue -out templates/midnight-blue.pptx`. First-class supported template. |
| `warm-coral.pptx` | mktemplate | Yes | PASS (1 WARN — Section Number placeholder name) | Programmatic template, defined in `cmd/mktemplate/main.go`. Regenerable via `go run ./cmd/mktemplate -name warm-coral -out templates/warm-coral.pptx`. First-class supported template. |
| `abstract.pptx` | designer | **No** | PASS | Designer-owned. Title Slide had `subtitle` added; `Section Break 1` renamed to `Section Divider` with `Section Number` placeholder added; `Introduction` renamed to `One Content`; Two Content, Blank, Blank + Title layouts authored via OOXML edits (all embedded SVG assets preserved byte-for-byte). Repair landed under `go-slide-creator-lenc` (parent `go-slide-creator-vqad`). |
| `blue-corporate.pptx` | designer | **No** | PASS | Designer-owned. `Title` repurposed as `Title Slide` (its 80pt body became `ctrTitle`, top eyebrow became `subTitle` idx=1); `Title and content 01` renamed to `One Content`; `Section break with image 01` renamed to `Section Divider` (first body retyped to `title`, second body renamed to `Section Number` cNvPr — already a structurally-valid section number per the existing `##` prompt); Two Content, Blank, Blank + Title authored via OOXML edits. All 4 embedded SVG assets preserved byte-for-byte. Repair landed under `go-slide-creator-pxdp` (parent `go-slide-creator-vqad`). |
| `business-template.pptx` | designer | **No** | PASS | Designer-owned. `Custom Layout` renamed to `Title Slide` (subTitle idx=1 authored below the title; title widened to full width and repositioned so 2-line wraps no longer collide with the subtitle); `Basic Page` renamed to `One Content` (body idx 14 → 1, slide3 updated in lockstep); `UNKNOWN IF USED 01` repurposed as `Section Divider` (body idx=13 cNvPr renamed to `Section Number` — already had `##` prompt + 208pt at 4.94" wide upper-right — added algn=r + accent1 fill; the extraneous 4.97" × 1.55" body idx=14 description placeholder was removed because it overflowed at text-fit P4 and is not part of the canonical Section Divider role); `BLANK - Left LARGE Logo` repurposed as `Closing` (body idx=10 retyped to `subTitle` idx=1; slide1 updated in lockstep; the layout's decorative bottom-right "My Consulting Company" Futura TextBox — which was also named `Title 1` and was being picked up by the engine as the title shape — renamed to `Company Branding`). New `Two Content` (slideLayout5, type=twoObj), `Blank` (slideLayout6), and `Blank + Title` (slideLayout7) layouts authored, registered in `slideMaster1.xml.rels` (rId6/rId7/rId8), `sldLayoutIdLst` (ids 2147483800/801/802), and `[Content_Types].xml`. Single embedded asset (`ppt/media/image1.emf`) preserved byte-for-byte. Repair landed under `go-slide-creator-mzhu` (parent `go-slide-creator-vqad`). |
| `modern.pptx` | designer | **No** | PASS | Designer-owned. `Title + subtitle` renamed to `Title Slide`; `Content 1` renamed to `One Content`; `Table` repurposed as `Section Divider` (tbl/dt/ftr/sldNum replaced with `Section Number` body placeholder in upper-right at 3.7" × 136pt accent1 right-aligned); `Content 3` repurposed as `Closing` (body idx=10 retyped to `subTitle` idx=1, with slide4 updated in lockstep); Two Content, Blank, Blank + Title authored via OOXML edits. All 4 embedded media assets (hdphoto1.wdp, image1–3.png) preserved byte-for-byte. Repair landed under `go-slide-creator-4wvi` (parent `go-slide-creator-vqad`). |
| `modern-template.pptx` | designer | **No** | FAIL (3 FAIL, 2 WARN) | Designer-owned. Two Content, Blank, Blank + Title missing — engine synthesises these at runtime (see `docs/TEMPLATE_SPEC.md` "Known Exceptions"). Repair tracked under `go-slide-creator-iy2k` (parent `go-slide-creator-vqad`). |
| `modern-yellow.pptx` | designer | **No** | PASS | Designer-owned. `111_Custom Layout` renamed to `Title Slide` (subtitle placeholder added below the existing title); `42_Custom Layout` repurposed as `Section Divider` (body idx=101 retyped to `title`; body idx=100 cNvPr renamed to `Section Number` — already had the `##` prompt + 208pt font at 4.88" wide upper-right — added right-align + accent1 fill, layout type set to `secHead`); `51_Custom Layout` renamed to `One Content` (body idx changed from 11 → 1 to satisfy canonical-placeholder gate; sample slide3 updated in lockstep); `69_Custom Layout` renamed to `Statement` (go-slide-creator-an1n) so it carries a `statement` tag instead of an empty tag set — no canonical role, decorative semicircle + grey rectangle preserved, `slideLayout4` ID unchanged so existing `layout_id` pins still resolve. New `Two Content` (slideLayout5, type=twoObj), `Blank` (slideLayout6, type=blank), `Blank + Title` (slideLayout7, type=titleOnly), `Closing` (slideLayout8, ctrTitle + subTitle) layouts authored, registered in `slideMaster1.xml.rels` (rId6–rId9), `sldLayoutIdLst` (ids 2147483800–803), and `[Content_Types].xml`. Theme `dk1.lastClr` swapped from `FFFFFF` → `000000` so dk1 actually renders as the dark text colour (lt1 unchanged at `FFFFFF`). No `ppt/media/*` assets in this template (the only binary, `docProps/thumbnail.jpeg`, preserved byte-for-byte; sha256=74f15ab9…ecdb20). Repair landed under `go-slide-creator-9gxk` (parent `go-slide-creator-vqad`). |

Conformance counts come from `json2pptx template-check`. Allow-list entries (with `sha256` and tracking issue) live at `internal/template/testdata/conformance_allowlist.json`; the corpus test `internal/template/conformance_corpus_test.go` is the gate.

## Why designer templates must NOT be regenerated

Designer templates carry visual identity that `cmd/mktemplate` cannot reproduce:

- **Embedded decorative assets** under `ppt/media/` (PNG, SVG, WDP) — backgrounds, logos, illustrations, photo frames. `mktemplate` only emits theme XML; it has no path to recreate raster or vector art.
- **Custom per-layout decorative shapes** — gradient bars, accent ribbons, photo frames placed directly in `slideLayout*.xml`. These shapes are tuned to the template's brand and would be lost on regeneration.
- **Intentional theme polarities** — some designer templates deliberately invert dark/light slots for light-on-dark or dark-on-light identities. Regenerating from `mktemplate` would force the default polarity and destroy the look. (Note: `modern-yellow.pptx` originally had `dk1.lastClr=#FFFFFF` which the repair under `go-slide-creator-9gxk` corrected to `#000000` because the master uses `tx1` (→ dk1) for body text against `bg1` (→ lt1, white) — the bright-on-bright was a bug, not an inversion.)
- **Custom layout naming** — designer layouts use names like `Section Break 1`, `Custom Layout`, `111_Custom Layout` that the classifier must rename in place to canonical roles; regeneration would not preserve the original layout XML.

## How to repair a designer template

Repair the existing `.pptx` **in place** — do not run `mktemplate` against it. The canonical repair workflow lives under `go-slide-creator-vqad` and is fully agent-executable (no PowerPoint required):

1. Unzip the `.pptx` (OOXML zip archive).
2. Read existing `slideLayout*.xml` + `theme1.xml` to understand background fills, decorative shapes, fonts, and colors.
3. Run the layout classifier (`internal/template/classifier`) over existing layouts and **rename in place** any layout that is structurally a canonical role under a non-canonical name — do not author a new duplicate.
4. For genuinely missing layouts, author new `slideLayout*.xml` files that copy decorative elements from existing layouts so they inherit the same look. Update `presentation.xml.rels` and `[Content_Types].xml` accordingly.
5. Apply theme polarity swaps directly in XML where required.
6. Re-zip and overwrite `templates/<name>.pptx`. Embedded assets under `ppt/media/*` MUST be preserved byte-for-byte (verify with sha256 manifest before/after).
7. Run `json2pptx template-check <template.pptx>` — must show `PASS` with zero `[WARN]` and zero `[FAIL]`.
8. Render every layout to JPG via `pptx2jpg` and run the `slide-visual-qa` skill (Haiku subagent) on the output. Iterate steps 4–7 until no critical visual findings remain.

PowerPoint may also be used to edit these templates if available, but it is not required and is not the preferred path for agent-driven repair.

## Adding a new template

- **Programmable theme variant**: add a new entry to the `templates` slice in `cmd/mktemplate/main.go`, regenerate, and verify with `template-check`. The template inherits the canonical layout XML produced by `mktemplate`.
- **Designer-owned template**: drop the `.pptx` under `templates/`, run `template-check`, and follow the repair workflow above until conformant. Add a temporary allow-list entry under `internal/template/testdata/conformance_allowlist.json` only if a tracking issue exists; the allow-list MUST shrink over time, never grow.

## Related docs

- `docs/TEMPLATE_SPEC.md` — what a conformant template must contain
- `CONTRIBUTING.md` (`Adding a Template`) — the conformance gate and allow-list mechanics
- `internal/template/testdata/conformance_allowlist.json` — current allow-list with tracking issues per template
