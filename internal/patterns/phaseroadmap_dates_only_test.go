package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// TestPhaseRoadmap_DatesOnlyPanelsAreContentSized: a roadmap whose phases
// carry a date range and no description keeps its panels at the height of
// that one line. They used to be grown toward 80% of the content area like
// panels of descriptions, and stood empty under the dates
// (go-slide-creator-v6f8j). With a description the growth is unchanged.
func TestPhaseRoadmap_DatesOnlyPanelsAreContentSized(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	ctx := ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	panelRow := func(vals *PhaseRoadmapValues) jsonschema.GridRowInput {
		t.Helper()
		grid, err := p.Expand(ctx, vals, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		row := grid.Rows[1] // band, panels (no milestones in these fixtures)
		if row.MinHeight <= 0 || row.MinHeight != row.MaxHeight {
			t.Fatalf("panel row must be pinned to one height, got min=%v max=%v", row.MinHeight, row.MaxHeight)
		}
		return row
	}

	withDescriptions := validPhaseRoadmapValues()
	datesOnly := validPhaseRoadmapValues()
	oneDescription := validPhaseRoadmapValues()
	for i := range datesOnly.Phases {
		datesOnly.Phases[i].Description = ""
		if i != 1 {
			oneDescription.Phases[i].Description = ""
		}
	}

	// One bold date line and the shape's top / bottom insets, no more.
	oneLine := phaseRoadmapDatePt*contentLineHeight + 2*defaultShapeInsetTBPt
	dates := panelRow(datesOnly)
	if dates.MinHeight > oneLine+4 {
		t.Errorf("dates-only panels are %.0fpt high for one %.0fpt date line (about %.0fpt with insets): empty panel under the date",
			dates.MinHeight, phaseRoadmapDatePt, oneLine)
	}
	for i, c := range dates.Cells {
		if c.Shape == nil || len(c.Shape.Text) == 0 {
			t.Errorf("dates-only panel %d lost its date range", i)
		}
	}

	// With descriptions the panels still grow into the content area.
	full := panelRow(withDescriptions)
	if full.MinHeight < 2*dates.MinHeight {
		t.Errorf("panels with descriptions are %.0fpt, want them grown well past the %.0fpt date line", full.MinHeight, dates.MinHeight)
	}
	if one := panelRow(oneDescription); one.MinHeight <= dates.MinHeight {
		t.Errorf("a roadmap with one description keeps grown panels, got %.0fpt vs %.0fpt dates-only", one.MinHeight, dates.MinHeight)
	}
}
