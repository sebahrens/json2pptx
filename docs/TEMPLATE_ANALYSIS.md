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
| `blue-corporate.pptx` | designer | **No** | FAIL (5 FAIL, 5 WARN) | Designer-owned. No native Title Slide; Section Divider lacks `title`; Blank and Blank + Title missing. Repair tracked under `go-slide-creator-pxdp` (parent `go-slide-creator-vqad`). |
| `business-template.pptx` | designer | **No** | FAIL (6 FAIL, 4 WARN) | Designer-owned. Title Slide lacks `subtitle`; Two Content, Section Divider, Blank, Blank + Title, Closing missing. Repair tracked under `go-slide-creator-mzhu` (parent `go-slide-creator-vqad`). |
| `modern.pptx` | designer | **No** | FAIL (5 FAIL, 4 WARN) | Designer-owned. Two Content, Section Divider, Blank, Blank + Title, Closing missing. Repair tracked under `go-slide-creator-4wvi` (parent `go-slide-creator-vqad`). |
| `modern-template.pptx` | designer | **No** | FAIL (3 FAIL, 2 WARN) | Designer-owned. Two Content, Blank, Blank + Title missing — engine synthesises these at runtime (see `docs/TEMPLATE_SPEC.md` "Known Exceptions"). Repair tracked under `go-slide-creator-iy2k` (parent `go-slide-creator-vqad`). |
| `modern-yellow.pptx` | designer | **No** | FAIL (6 FAIL, 3 WARN) | Designer-owned. Title Slide lacks `subtitle`; Two Content, Section Divider, Blank, Blank + Title, Closing missing; theme `dk1=#FFFFFF` (polarity wrong). Repair tracked under `go-slide-creator-9gxk` (parent `go-slide-creator-vqad`). |

Conformance counts come from `json2pptx template-check`. Allow-list entries (with `sha256` and tracking issue) live at `internal/template/testdata/conformance_allowlist.json`; the corpus test `internal/template/conformance_corpus_test.go` is the gate.

## Why designer templates must NOT be regenerated

Designer templates carry visual identity that `cmd/mktemplate` cannot reproduce:

- **Embedded decorative assets** under `ppt/media/` (PNG, SVG, WDP) — backgrounds, logos, illustrations, photo frames. `mktemplate` only emits theme XML; it has no path to recreate raster or vector art.
- **Custom per-layout decorative shapes** — gradient bars, accent ribbons, photo frames placed directly in `slideLayout*.xml`. These shapes are tuned to the template's brand and would be lost on regeneration.
- **Intentional theme polarities** — `modern-yellow.pptx` defines `dk1=#FFFFFF` deliberately (light-on-dark inversion). Regenerating from `mktemplate` would force the default dark/light polarity and destroy the look.
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
