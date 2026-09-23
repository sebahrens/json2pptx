package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// TestBeforeAfter_HeaderTextVerticalAnchorCenter is a regression for
// go-slide-creator-hjgz: the before-after header band must vertically center
// its text inside the colored bar. The pattern builds the header text JSON
// with vertical_align="ctr" — this test pins that contract so a future change
// can't silently top-anchor the header again.
func TestBeforeAfter_HeaderTextVerticalAnchorCenter(t *testing.T) {
	got := string(buildBeforeAfterTextContent("Before", 16, true, "lt1", "ctr"))
	if !strings.Contains(got, `"vertical_align":"ctr"`) {
		t.Errorf("before-after header text must emit vertical_align=ctr; got %s", got)
	}
}

// TestIconRow_CaptionTextVerticalAnchorCenter is a regression for
// go-slide-creator-hjgz: icon-row captions previously emitted
// vertical_align="bottom" (not a valid OOXML anchor value) which PowerPoint
// silently fell back to top alignment, leaving conspicuous empty space below
// the caption. The pattern now centers the caption text.
func TestIconRow_CaptionTextVerticalAnchorCenter(t *testing.T) {
	got := string(buildIconRowCaptionOnly("Launch", 12))
	if !strings.Contains(got, `"vertical_align":"ctr"`) {
		t.Errorf("icon-row caption text must emit vertical_align=ctr; got %s", got)
	}
	if strings.Contains(got, `"vertical_align":"bottom"`) {
		t.Errorf("icon-row caption text must not emit invalid anchor 'bottom'; got %s", got)
	}
}

func TestBeforeAfter_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("before-after")
	if !ok {
		t.Fatal("before-after pattern not registered")
	}

	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Before", Items: []string{"Slow", "Manual"}},
		After:  BeforeAfterColumn{Header: "After", Items: []string{"Fast", "Automated"}},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 2 {
		t.Errorf("expected 2 rows (header + body), got %d", len(grid.Rows))
	}
	// Header row has 3 cells (before, chevron, after)
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("expected 3 header cells, got %d", len(grid.Rows[0].Cells))
	}
}

func TestBeforeAfterFullPanelsUseSurplusHeight(t *testing.T) {
	pat := &beforeAfter{}
	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Today", Items: []string{"Manual", "Slow"}},
		After:  BeforeAfterColumn{Header: "Target", Items: []string{"Automated", "Fast"}},
	}
	grid, err := pat.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, zoneH := sizingAreaPt(fullThemeCtx())
	if got := grid.Rows[0].MaxHeight + grid.Rows[1].MaxHeight + grid.Gap; got < zoneH*0.59 {
		t.Errorf("full block %.1fpt uses less than 60%% of %.1fpt zone", got, zoneH)
	}
	if got := grid.Rows[0].Cells[1]; got.RowSpan != 2 || got.Shape.Geometry != "chevron" || len(got.Shape.Text) != 0 {
		t.Errorf("separator should be a full-height chevron without a redundant arrow: %+v", got)
	}
	if len(grid.Rows[1].Cells) != 2 {
		t.Fatalf("body should contain two panels, with center reserved by chevron: %+v", grid.Rows[1].Cells)
	}
	for _, cell := range grid.Rows[1].Cells {
		if string(cell.Shape.Fill) != `{"color":"accent1","tint":12000}` || !strings.Contains(string(cell.Shape.Text), `"vertical_align":"ctr"`) {
			t.Errorf("expanded body panel should be visible and centered: %+v", cell.Shape)
		}
	}
}

func TestBeforeAfterPanelToneHexOverride(t *testing.T) {
	pat := &beforeAfter{}
	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Today", Items: []string{"Manual"}},
		After:  BeforeAfterColumn{Header: "Target", Items: []string{"Automated"}},
	}
	grid, err := pat.Expand(fullThemeCtx(), vals, &BeforeAfterOverrides{Accent: "#2E5090"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tone := beforeAfterPanelTone("#2E5090")
	if tone.Tint != 0 || !isHexColor(tone.Color) {
		t.Fatalf("hex accent should be pre-mixed into a solid fill, got %+v", tone)
	}
	for _, cell := range grid.Rows[1].Cells {
		if string(cell.Shape.Fill) != `"`+tone.Color+`"` {
			t.Errorf("body panel fill %s does not use tinted hex accent %s", cell.Shape.Fill, tone.Color)
		}
	}
	c := svggen.MustParseColor(tone.Color)
	if c.Luminance() < 0.85 || c.Hex() == "#FFFFFF" {
		t.Errorf("panel should be super-light but visibly accent-derived, got %s", c.Hex())
	}
}

func TestBeforeAfterPanelsFollowPerCellAccents(t *testing.T) {
	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "Today", Items: []string{"Manual"}},
		After:  BeforeAfterColumn{Header: "Target", Items: []string{"Automated"}},
	}
	grid, err := (&beforeAfter{}).Expand(fullThemeCtx(), vals, &BeforeAfterOverrides{CellAccentMode: "alternate"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{`{"color":"accent1","tint":12000}`, `{"color":"accent2","tint":12000}`} {
		if got := string(grid.Rows[1].Cells[i].Shape.Fill); got != want {
			t.Errorf("panel %d fill = %s, want %s", i, got, want)
		}
	}
}

func TestBeforeAfter_ValidateMissingHeader(t *testing.T) {
	p, ok := Default().Get("before-after")
	if !ok {
		t.Fatal("before-after pattern not registered")
	}

	vals := &BeforeAfterValues{
		Before: BeforeAfterColumn{Header: "", Items: []string{"Slow"}},
		After:  BeforeAfterColumn{Header: "After", Items: []string{"Fast"}},
	}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for missing before.header")
	}
}
