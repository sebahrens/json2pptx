package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
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
	for _, cell := range []*jsonschema.GridCellInput{grid.Rows[1].Cells[0], grid.Rows[1].Cells[2]} {
		if string(cell.Shape.Fill) != `"lt2"` || !strings.Contains(string(cell.Shape.Text), `"vertical_align":"ctr"`) {
			t.Errorf("expanded body panel should be visible and centered: %+v", cell.Shape)
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
