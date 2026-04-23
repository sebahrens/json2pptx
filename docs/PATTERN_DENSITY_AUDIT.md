# Pattern Density Audit

Audit of all 8 registered patterns for density-related validation caps.
Conducted as part of bead `go-slide-creator-3bua`.

## Context

Table density rules (TDR) in `internal/pipeline/structural_smells.go` enforce
`TDRMaxRows=7` and `TDRMaxCols=6` for tables. Patterns bypass TDR entirely
because they expand to shape grids, not tables. This audit checks whether each
pattern's existing schema validation already provides equivalent density
protection, or whether additional caps are needed.

## Audit Checklist

### card-grid

| Check | Present? | Details |
|-------|----------|---------|
| max_items (count cap) | **Yes** | `columns` 1–5, `rows` 1–5, `cells` maxItems=25; count must equal cols×rows |
| max_chars per cell | **Yes** | header ≤80, body ≤300 |
| min_cell_emu | Computed | Grid engine computes cell size from slide bounds and col/row count |
| Density interaction (dims × text × font) | N/A | cols×rows capped at 5×5=25; text caps per cell are conservative |

**Verdict: No gap.** The 5×5 grid dimension cap plus per-cell text limits provide
adequate density protection. A 5×5 grid with full-length bodies (300 chars each)
is dense but survivable — the shape grid engine auto-sizes cells to fit.

---

### kpi-3up

| Check | Present? | Details |
|-------|----------|---------|
| max_items | **Yes** | Exactly 3 cells (hard count) |
| max_chars per cell | **Yes** | big ≤8, small ≤40 |
| min_cell_emu | N/A | Fixed 1×3 grid |
| Density interaction | N/A | Fixed count, very short text |

**Verdict: No gap.** Fixed 3 cells with ≤48 chars total per cell. Cannot overflow.

---

### kpi-4up

| Check | Present? | Details |
|-------|----------|---------|
| max_items | **Yes** | Exactly 4 cells (hard count) |
| max_chars per cell | **Yes** | big ≤8, small ≤40 |
| min_cell_emu | N/A | Fixed 1×4 grid |
| Density interaction | N/A | Fixed count, very short text |

**Verdict: No gap.** Same as kpi-3up with one extra cell.

---

### comparison-2col

| Check | Present? | Details |
|-------|----------|---------|
| max_items | **Yes** | rows 1–10 |
| max_chars per cell | **Yes** | left/right ≤200, headers ≤60 |
| min_cell_emu | Computed | 2-column fixed layout |
| Density interaction | N/A | 2 cols × 10 rows = 20 cells max; wide cells (50% slide width each) |

**Verdict: No gap.** Fixed 2-column layout means each cell gets half the slide
width. 10 rows at 200 chars is dense but each row cell is wide enough.

---

### bmc-canvas

| Check | Present? | Details |
|-------|----------|---------|
| max_items | **Yes** | Fixed 9 cells; bullets 1–10 per cell |
| max_chars per cell | **Yes** | header ≤60, bullet ≤200 |
| min_cell_emu | N/A | Fixed 5-col, 3-row non-uniform grid |
| Density interaction | **Marginal** | Smallest cells (1×1: key_activities, key_resources, customer_relations, channels) could hold 60 + 10×200 = 2060 chars |

**Verdict: Marginal but no action needed.** The 1×1 cells in the BMC layout are
~20% of slide width × ~33% of content height. At max fill (10 bullets × 200
chars = 2000 chars body), text would overflow — but the shape grid engine applies
font shrinking. The 10-bullet cap is already a reasonable proxy for density. A
dedicated density cap would duplicate the bullet count check with no additional
signal. If real-world overflow occurs, the fit-report pipeline (bead
`go-slide-creator-tme8`) will catch it at render time.

---

### matrix-2x2

| Check | Present? | Details |
|-------|----------|---------|
| max_items | **Yes** | Fixed 4 quadrants |
| max_chars per cell | **Yes** | header ≤80, body ≤200, axis labels ≤60 |
| min_cell_emu | N/A | Fixed 3-col, 3-row grid with axis cells |
| Density interaction | N/A | 4 content cells with modest text limits |

**Verdict: No gap.** Fixed 4 quadrants, each getting ~44% of content width ×
~44% of content height. 280 chars max per quadrant is comfortable.

---

### icon-row

| Check | Present? | Details |
|-------|----------|---------|
| max_items | **Yes** | 3–5 items |
| max_chars per cell | **Yes** | icon ≤20, caption ≤60 |
| min_cell_emu | N/A | Single-row grid |
| Density interaction | N/A | 5 cells max, 80 chars max per cell |

**Verdict: No gap.** Low cell count, very short text. Cannot overflow.

---

### timeline-horizontal

| Check | Present? | Details |
|-------|----------|---------|
| max_items | **Yes** | 3–7 stops |
| max_chars per cell | **Yes** | label ≤60, date ≤30, body ≤200 |
| min_cell_emu | N/A | Single-row grid |
| Density interaction | **Marginal** | 7 stops × 290 chars = 2030 chars total |

**Verdict: Marginal but no action needed.** At 7 stops each cell gets ~14% of
slide width. With 200-char body text at default 10pt, the shape grid engine
shrinks to fit. The 7-stop max already serves as an effective density cap. If
overflow occurs, the fit-report pipeline catches it.

---

## Summary

| Pattern | max_items | max_chars | Density gap? | Action |
|---------|-----------|-----------|--------------|--------|
| card-grid | ✅ 5×5=25 | ✅ h:80 b:300 | No | None |
| kpi-3up | ✅ 3 fixed | ✅ b:8 s:40 | No | None |
| kpi-4up | ✅ 4 fixed | ✅ b:8 s:40 | No | None |
| comparison-2col | ✅ 10 rows | ✅ 200/cell | No | None |
| bmc-canvas | ✅ 9 cells, 10 bullets | ✅ h:60 b:200 | Marginal | None — fit-report covers |
| matrix-2x2 | ✅ 4 quadrants | ✅ h:80 b:200 | No | None |
| icon-row | ✅ 3–5 | ✅ i:20 c:60 | No | None |
| timeline-horizontal | ✅ 3–7 | ✅ l:60 b:200 | Marginal | None — fit-report covers |

**Conclusion:** The devil's advocate position is confirmed. Every pattern already
has density-relevant caps via max_items and per-cell max_chars. No pattern
requires an additional `ErrCodeDensityExceeded` check. The two marginal cases
(bmc-canvas, timeline-horizontal) are adequately covered by existing item count
caps plus the fit-report pipeline's render-time overflow detection.
