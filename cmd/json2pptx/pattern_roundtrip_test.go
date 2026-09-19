package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// The documented workflow is expand_pattern -> inspect -> tweak -> generate,
// and the last step was impossible: the expanders emit explicit font sizes,
// constrained design mode refuses them from an author, and a single
// scqa-summary expansion produced twenty design_mode_violations the agent had
// no way to remove (go-slide-creator-c3po).
func TestExpandedPatternGridSurvivesResubmission(t *testing.T) {
	pi := &PatternInput{
		Name: "scqa-summary",
		Values: json.RawMessage(`{
			"situation": "Two of three clearers have migrated on the rehearsed plan.",
			"complication": "The third clearer has asked for a six-week deferral.",
			"questions": ["Do we hold the wave plan or re-cut it around the deferral?"],
			"answer": ["Hold the plan.", "Run the third clearer as a separate wave in Q3."]
		}`),
	}
	grid, _, err := expandPattern(pi, patterns.ExpandContext{}, patterns.Default())
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	// The expansion stamps its own provenance.
	if grid.Source != "pattern:scqa-summary" {
		t.Fatalf("grid source = %q, want pattern:scqa-summary", grid.Source)
	}
	// It really does carry the sizes that used to be refused, so the test is
	// not passing because there is nothing to refuse.
	if !gridCarriesAnAbsoluteSize(t, grid) {
		t.Fatal("the expansion carries no absolute font size; this test would pass vacuously")
	}

	if findings := checkShapeGrid(grid, 1); len(findings) != 0 {
		t.Errorf("a stamped expansion still reports %d design-mode violations: %+v", len(findings), findings)
	}

	// Without the stamp the sizes are refused exactly as before: the waiver is
	// the engine's provenance, not a general relaxation.
	unstamped := *grid
	unstamped.Source = ""
	if findings := checkShapeGrid(&unstamped, 1); len(findings) == 0 {
		t.Error("an unstamped grid of the same sizes reports nothing; the rule has been lost")
	}

	// A source the engine would not have written waives nothing.
	forged := *grid
	forged.Source = "pattern:not-a-registered-pattern"
	if findings := checkShapeGrid(&forged, 1); len(findings) == 0 {
		t.Error("an unregistered pattern name waived the size rule")
	}
}

// The waiver is for sizes only: a raw hex colour is still refused on a stamped
// grid, and the expanders never emit one.
func TestPatternStampDoesNotWaiveRawHexColours(t *testing.T) {
	grid := &jsonschema.ShapeGridInput{
		Source: "pattern:card-grid",
		Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"#FF00FF"`),
				Text:     json.RawMessage(`{"paragraphs":[{"content":"Hello","size":20}]}`),
			},
		}}}},
	}
	findings := checkShapeGrid(grid, 1)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want the hex one alone: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "raw hex color") {
		t.Errorf("finding = %q, want the raw-hex rule", findings[0].Message)
	}
}

// gridCarriesAnAbsoluteSize reports whether any cell text declares a size.
func gridCarriesAnAbsoluteSize(t *testing.T, grid *jsonschema.ShapeGridInput) bool {
	t.Helper()
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 {
				continue
			}
			var probe struct {
				Size       float64 `json:"size"`
				Paragraphs []struct {
					Size float64 `json:"size"`
				} `json:"paragraphs"`
			}
			if json.Unmarshal(cell.Shape.Text, &probe) != nil {
				continue
			}
			if probe.Size > 0 {
				return true
			}
			for _, p := range probe.Paragraphs {
				if p.Size > 0 {
					return true
				}
			}
		}
	}
	return false
}
