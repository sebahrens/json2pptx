package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// paragraphSizesOf lists the paragraph sizes of a cell's text.
func paragraphSizesOf(t *testing.T, raw json.RawMessage) []float64 {
	t.Helper()
	var obj struct {
		Paragraphs []struct {
			Size float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("text: %v", err)
	}
	out := make([]float64, len(obj.Paragraphs))
	for i, p := range obj.Paragraphs {
		out[i] = p.Size
	}
	return out
}

// go-slide-creator-yhzxt: a sparse dots timeline promotes its date and body
// to the label's size (the placement policy then steps all three together);
// a timeline that carries real copy keeps the sizes its budgets are measured
// at, and authored sizes are kept.
func TestTimelineDotsPromotesSparseStops(t *testing.T) {
	p := &timelineHorizontal{}
	sizes := func(vals TimelineHorizontalValues, ovr *TimelineHorizontalOverrides) (date, label, body float64) {
		t.Helper()
		grid, err := p.Expand(fullThemeCtx(), &vals, ovr, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(grid.Rows) != 3 {
			t.Fatalf("rows = %d, want date, axis, stops", len(grid.Rows))
		}
		date = paragraphSizesOf(t, grid.Rows[0].Cells[0].Shape.Text)[0]
		stop := paragraphSizesOf(t, grid.Rows[2].Cells[0].Shape.Text)
		return date, stop[0], stop[1]
	}
	sparse := TimelineHorizontalValues{
		{Label: "Discovery", Date: "Q1 2025", Body: "Baseline costs and interview 40 stakeholders"},
		{Label: "Pilot", Date: "Q2 2025", Body: "Run the new model in two regions"},
		{Label: "Scale-up", Date: "Q3 2025", Body: "Roll out to all twelve markets"},
	}
	if d, l, b := sizes(sparse, &TimelineHorizontalOverrides{}); d != scaleSubheadPt || l != scaleSubheadPt || b != scaleSubheadPt {
		t.Errorf("sparse timeline date/label/body = %v/%v/%v, want all at %vpt", d, l, b, scaleSubheadPt)
	}
	if d, l, b := sizes(sparse, &TimelineHorizontalOverrides{BodySize: 12}); d != scaleBodyPt || l != scaleSubheadPt || b != 12 {
		t.Errorf("authored body size: date/label/body = %v/%v/%v, want the pattern's 12/14/12", d, l, b)
	}
	dense := append(TimelineHorizontalValues(nil), sparse...)
	for i := range dense {
		dense[i].Body = strings.Repeat("Baseline costs and interview forty stakeholders across the regions. ", 5)
	}
	if d, _, b := sizes(dense, &TimelineHorizontalOverrides{}); d != scaleBodyPt || b != scaleBodyPt {
		t.Errorf("dense timeline date/body = %v/%v, want the 12pt defaults", d, b)
	}
	// Narrow stop columns cannot hold a date at the lead step.
	narrow := TimelineHorizontalValues{}
	for i := 0; i < 7; i++ {
		narrow = append(narrow, TimelineStop{Label: "Milestone", Date: "September 2025", Body: "Go"})
	}
	if d, _, b := sizes(narrow, &TimelineHorizontalOverrides{}); d != scaleBodyPt || b != scaleBodyPt {
		t.Errorf("seven-stop timeline date/body = %v/%v, want the 12pt defaults", d, b)
	}
}

// go-slide-creator-yhzxt: a small driver tree is laid out one type step up;
// a tree with more leaves than fit at those sizes, or authored sizes, keeps
// the dense sizes.
func TestDriverTreePromotesSmallTree(t *testing.T) {
	dt := &driverTree{}
	rootAndLeaf := func(vals *DriverTreeValues, ovr *DriverTreeOverrides) (root, leaf float64) {
		t.Helper()
		if ovr == nil {
			ovr = &DriverTreeOverrides{}
		}
		grid, err := dt.Expand(fullThemeCtx(), vals, ovr, nil)
		if err != nil {
			t.Fatal(err)
		}
		cells := grid.Rows[0].Cells
		return paragraphSizesOf(t, cells[0].Shape.Text)[0], paragraphSizesOf(t, cells[2].Shape.Text)[0]
	}
	small := dt.ExemplarValues().(*DriverTreeValues)
	if root, leaf := rootAndLeaf(small, nil); root != scaleLeadPt || leaf != scaleSubheadPt {
		t.Errorf("small tree root/leaf = %v/%v, want %v/%v", root, leaf, scaleLeadPt, scaleSubheadPt)
	}
	authored := &DriverTreeOverrides{}
	authored.BodySize = 12
	if root, leaf := rootAndLeaf(small, authored); root != scaleSubheadPt || leaf != 12 {
		t.Errorf("authored body size: root/leaf = %v/%v, want the pattern's %v/12", root, leaf, scaleSubheadPt)
	}
	big := &DriverTreeValues{Root: small.Root}
	for b := 0; b < 4; b++ {
		branch := DriverTreeBranch{Label: "Branch"}
		for l := 0; l < 4; l++ {
			branch.Leaves = append(branch.Leaves, "Reduce unscheduled outages")
		}
		big.Branches = append(big.Branches, branch)
	}
	if root, leaf := rootAndLeaf(big, nil); root != scaleSubheadPt || leaf != sizeDenseCaptionPt {
		t.Errorf("sixteen-leaf tree root/leaf = %v/%v, want the dense %v/%v", root, leaf, scaleSubheadPt, sizeDenseCaptionPt)
	}
}
