package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// Geometry tests for go-slide-creator-3i7c: header bands sized to the header
// line height, cards / bodies sized to their content, blocks centred.

const pt = 12700

func rowCells(cells []shapegrid.ResolvedCell, row int) []shapegrid.ResolvedCell {
	var out []shapegrid.ResolvedCell
	for _, c := range cells {
		if c.RowIdx == row {
			out = append(out, c)
		}
	}
	return out
}

func maxCellHeight(cells []shapegrid.ResolvedCell) int64 {
	var h int64
	for _, c := range cells {
		if c.CellBounds.CY > h {
			h = c.CellBounds.CY
		}
	}
	return h
}

func TestBeforeAfter_HeaderBandAndContentBody(t *testing.T) {
	res, _ := resolvePatternForTest(t, "before-after", `{"before":{"header":"Current State (FY25)","items":["Manual reconciliation across 14 regional ledgers","5-day month-end close with frequent restatements","Spreadsheet-based forecasting owned by individuals","Limited audit trail; 23 control deficiencies flagged"]},"after":{"header":"Target State (FY27)","items":["Single global ledger with automated intercompany matching","2-day close with continuous accounting","Driver-based rolling forecast in a shared planning tool","End-to-end audit trail; controls embedded in workflow"]}}`)
	// 16pt header: ~1.2x line height + insets + padding, never 25% of the area.
	if h := maxCellHeight(rowCells(res.Cells, 0)); h > 50*pt {
		t.Errorf("header band %.0fpt, want <= 50pt", float64(h)/pt)
	}
	// Four 14pt bullets per side hug their text (~4-6 lines), far below the
	// ~75% of the content area the flex body used to take.
	if h := maxCellHeight(rowCells(res.Cells, 1)); float64(h) > 0.45*float64(contentRect.CY) {
		t.Errorf("body row %.0fpt exceeds 45%% of the content height", float64(h)/pt)
	}
	assertCentred(t, "before-after", res.Cells)
}

func TestBeforeAfterCompact_WithinSixtyPercent(t *testing.T) {
	res, _ := resolvePatternForTest(t, "before-after-compact", `{"before":{"header":"Today","items":["3 disconnected CRMs","Manual lead routing","No shared pipeline view"]},"after":{"header":"Target","items":["One CRM instance","Rules-based routing","Real-time pipeline dashboard"]}}`)
	top, bottom := blockExtent(res.Cells)
	if float64(bottom-top) > 0.6*float64(contentRect.CY) {
		t.Errorf("compact block %.0fpt exceeds 60%% of the content height", float64(bottom-top)/pt)
	}
	if h := maxCellHeight(rowCells(res.Cells, 0)); h > 45*pt {
		t.Errorf("compact header band %.0fpt, want <= 45pt", float64(h)/pt)
	}
}

func TestStylishPanels_HeaderBand(t *testing.T) {
	res, _ := resolvePatternForTest(t, "stylish-panels", `[{"title":"Grow the core","body":["Defend share in top 5 markets","Lift price realisation 3 pts"]},{"title":"Build adjacencies","body":["Launch services offering"]},{"title":"Fund the journey","body":["$340M cost programme"]}]`)
	if h := maxCellHeight(rowCells(res.Cells, 0)); h > 50*pt {
		t.Errorf("ribbon header %.0fpt, want <= 50pt (was 20%% of the area)", float64(h)/pt)
	}
	if h := maxCellHeight(rowCells(res.Cells, 1)); float64(h) > 0.4*float64(contentRect.CY) {
		t.Errorf("panel body %.0fpt should hug two bullets", float64(h)/pt)
	}
	assertCentred(t, "stylish-panels", res.Cells)
}

func TestHeroDetail_CardsHugContent(t *testing.T) {
	res, _ := resolvePatternForTest(t, "hero-detail", `{"hero":{"value":"$340M","label":"Annual run-rate savings"},"details":[{"title":"Procurement","body":"$145M from supplier consolidation"},{"title":"Operations","body":"$120M from network redesign"},{"title":"SG&A","body":"$75M from shared services"}]}`)
	if h := maxCellHeight(rowCells(res.Cells, 1)); float64(h) > 0.3*float64(contentRect.CY) {
		t.Errorf("detail cards %.0fpt should hug a title + one line (<= 30%% of content)", float64(h)/pt)
	}
	assertCentred(t, "hero-detail", res.Cells)
}

func TestCardGrid_RowsHugContent(t *testing.T) {
	res, _ := resolvePatternForTest(t, "card-grid", `{"columns":3,"rows":2,"cells":[{"header":"Pricing","body":"Close 4-6% realised-price leakage."},{"header":"Procurement","body":"Consolidate suppliers."},{"header":"Network","body":"Close 3 of 11 DCs."},{"header":"SG&A","body":"Shared services."},{"header":"Working capital","body":"Reduce DIO by 12 days."},{"header":"Digital","body":"Grow e-commerce share."}]}`)
	for _, c := range res.Cells {
		if float64(c.CellBounds.CY) > 0.3*float64(contentRect.CY) {
			t.Errorf("card %d/%d is %.0fpt tall; one-line cards should hug their text", c.RowIdx, c.ColIdx, float64(c.CellBounds.CY)/pt)
		}
	}
	assertCentred(t, "card-grid", res.Cells)
}
