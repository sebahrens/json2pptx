package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// panelBounds is the body placeholder a 16:9 content layout hands a native
// diagram — the box the round-2 review saw a three-panel group drawn 80% empty
// inside (go-slide-creator-5smrk).
var panelBounds = types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}

// TestPanelColumnsBodyHugsText: a one-sentence body must not be given the whole
// remaining placeholder height. The body box is allowed at most 30% unused
// space — the same target docs/PATTERNS.md states for cards.
func TestPanelColumnsBodyHugsText(t *testing.T) {
	panels := []nativePanelData{
		{title: "Discovery", body: "Map the current process and agree the target state."},
		{title: "Build", body: "Ship the first release to the pilot group."},
		{title: "Scale", body: "Roll out to the remaining four regions."},
	}
	panelWidth := (panelBounds.Width - 2*panelGap) / 3

	_, _, _, _, bodyCY := panelColumnsLayout(panelBounds, panels, panelWidth, "")
	_, _, _, fullBodyCY := panelColumnsBands(panelBounds.Height, false)
	if bodyCY >= fullBodyCY {
		t.Fatalf("body band %d was not shrunk below the full %d", bodyCY, fullBodyCY)
	}

	tallest := int64(0)
	for i := range panels {
		if h := panelBodyBoxHeight(panels[i].body, "", panelWidth); h > tallest {
			tallest = h
		}
	}
	if tallest <= 0 {
		t.Skip("no font metrics available to measure panel bodies")
	}
	if unused := float64(bodyCY-tallest) / float64(bodyCY); unused > 0.30 {
		t.Errorf("body box is %.0f%% empty (%d EMU for %d EMU of content), want <= 30%%",
			unused*100, bodyCY, tallest)
	}
}

// TestPanelColumnsGroupIsCentred: the shrunk group sits at the vertical centre
// of the placeholder, not pinned to its top.
func TestPanelColumnsGroupIsCentred(t *testing.T) {
	panels := []nativePanelData{{title: "A", body: "One line."}, {title: "B", body: "One line."}}
	panelWidth := (panelBounds.Width - panelGap) / 2

	offsetY, iconBandCY, headerCY, gapCY, bodyCY := panelColumnsLayout(panelBounds, panels, panelWidth, "")
	used := iconBandCY + headerCY + gapCY + bodyCY
	if used >= panelBounds.Height {
		t.Skip("bodies need the full height on this platform's metrics")
	}
	below := panelBounds.Height - used - offsetY
	if diff := offsetY - below; diff > 1 || diff < -1 {
		t.Errorf("group not centred: %d above, %d below", offsetY, below)
	}
}

// TestPanelColumnsDenseBodyKeepsFullBand: content that needs everything
// available keeps the original band, so shrinking can never clip a dense panel.
func TestPanelColumnsDenseBodyKeepsFullBand(t *testing.T) {
	dense := strings.Repeat("- A long bullet describing one step of the workstream in detail\n", 12)
	panels := []nativePanelData{{title: "Scope", body: dense}, {title: "Build", body: "Short."}}
	panelWidth := (panelBounds.Width - panelGap) / 2

	offsetY, _, _, _, bodyCY := panelColumnsLayout(panelBounds, panels, panelWidth, "")
	_, _, _, fullBodyCY := panelColumnsBands(panelBounds.Height, false)
	if bodyCY != fullBodyCY {
		t.Errorf("dense body band = %d, want the full %d", bodyCY, fullBodyCY)
	}
	if offsetY != 0 {
		t.Errorf("a full-height group must not be offset, got %d", offsetY)
	}
}

// TestPanelBodyBoxHeightGrowsWithText: the measurement must actually respond to
// content — a five-bullet body needs a taller box than a one-line body.
func TestPanelBodyBoxHeightGrowsWithText(t *testing.T) {
	const w = 3_000_000
	one := panelBodyBoxHeight("Short line.", "", w)
	many := panelBodyBoxHeight("- One\n- Two\n- Three\n- Four\n- Five", "", w)
	if one <= 0 || many <= 0 {
		t.Skip("no font metrics available")
	}
	if many <= one {
		t.Errorf("five bullets (%d) should need more height than one line (%d)", many, one)
	}
	if empty := panelBodyBoxHeight("", "", w); empty != 0 {
		t.Errorf("empty body should report 0, got %d", empty)
	}
}

// TestStatCardsHugContent: a card carrying a hero and a caption is sized to
// them, and the grid is centred rather than stretched over the placeholder.
func TestStatCardsHugContent(t *testing.T) {
	panels := []nativePanelData{
		{title: "Annual recurring revenue", value: "EUR 42m"},
		{title: "Net revenue retention", value: "118%"},
		{title: "Payback period", value: "14 mo"},
	}
	cols, rows := statCardGridLayout(len(panels))
	cardW := (panelBounds.Width - int64(cols-1)*statCardGap) / int64(cols)

	offsetY, cardH := statCardsLayout(panelBounds, panels, rows, cardW, "")
	full := panelBounds.Height / int64(rows)
	if cardH >= full {
		t.Fatalf("card height %d was not shrunk below the full %d", cardH, full)
	}

	tallest := int64(0)
	for i := range panels {
		if h := statCardBoxHeight(panels[i], "", cardW, 0); h > tallest {
			tallest = h
		}
	}
	if tallest <= 0 {
		t.Skip("no font metrics available to measure stat cards")
	}
	if unused := float64(cardH-tallest) / float64(cardH); unused > 0.30 {
		t.Errorf("stat card is %.0f%% empty (%d EMU for %d EMU of content), want <= 30%%",
			unused*100, cardH, tallest)
	}
	used := int64(rows)*cardH + int64(rows-1)*statCardGap
	if below := panelBounds.Height - used - offsetY; below-offsetY > 1 || offsetY-below > 1 {
		t.Errorf("stat grid not centred: %d above, %d below", offsetY, below)
	}
}

// TestStatCardBoxHeightCountsEveryLine: hero, caption and delta each add height,
// and a card with no text reports none.
func TestStatCardBoxHeightCountsEveryLine(t *testing.T) {
	const cardW = 3_000_000
	heroOnly := statCardBoxHeight(nativePanelData{value: "42%"}, "", cardW, 0)
	withCaption := statCardBoxHeight(nativePanelData{value: "42%", title: "Gross margin"}, "", cardW, 0)
	withDelta := statCardBoxHeight(nativePanelData{value: "42%", title: "Gross margin", body: "+3 pts"}, "", cardW, 0)
	if heroOnly <= 0 {
		t.Skip("no font metrics available")
	}
	if withCaption <= heroOnly {
		t.Errorf("caption added no height: %d vs %d", withCaption, heroOnly)
	}
	if withDelta <= withCaption {
		t.Errorf("delta line added no height: %d vs %d", withDelta, withCaption)
	}
	if empty := statCardBoxHeight(nativePanelData{}, "", cardW, 0); empty != 0 {
		t.Errorf("an empty card should report 0, got %d", empty)
	}
}

// TestMeasureWidthEMUCompensatesInsets: textfit subtracts its own 7.2pt side
// margins, so a shape declaring insets has to add them back or every block is
// measured in a narrower box than it renders in.
func TestMeasureWidthEMUCompensatesInsets(t *testing.T) {
	const shapeW = 3_000_000
	got := measureWidthEMU(shapeW, panelBodyMarginLeft, panelBodyMarginRight)
	want := shapeW - panelBodyMarginLeft - panelBodyMarginRight + 2*91440
	if got != want {
		t.Errorf("measureWidthEMU = %d, want %d", got, want)
	}
	if got <= shapeW-panelBodyMarginLeft-panelBodyMarginRight {
		t.Error("the compensated width must exceed the bare text width")
	}
}
