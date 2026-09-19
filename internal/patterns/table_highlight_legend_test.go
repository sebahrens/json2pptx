package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// go-slide-creator-z0up. thLegendCell emitted three swatches per scale but
// indexed the caller's legend_labels with the same i for both, so a deck mixing
// harvey and RAG columns showed six swatches carrying duplicated words — the
// green RAG dot wearing the Harvey "low" label, a legend that said the opposite
// of its own chart. With six entries on one row the text also autofit down to
// ~2pt.

func TestLegendLabelsAreSeparatePerScale(t *testing.T) {
	v := &TableHighlightValues{
		LegendLabels:    []string{"Fully meets", "Partially meets", "Does not meet"},
		LegendLabelsRAG: []string{"On track", "At risk", "Off track"},
	}
	harvey := thLegendLabelsFor(v, thScaleHarvey)
	rag := thLegendLabelsFor(v, thScaleRAG)

	if harvey[0] != "Fully meets" || harvey[2] != "Does not meet" {
		t.Errorf("harvey labels = %v", harvey)
	}
	if rag[0] != "On track" || rag[2] != "Off track" {
		t.Errorf("rag labels = %v — the RAG scale must not reuse the harvey words", rag)
	}
}

func TestLegendLabelDefaults(t *testing.T) {
	v := &TableHighlightValues{}
	if got := thLegendLabelsFor(v, thScaleHarvey); got[0] != "Fully meets" {
		t.Errorf("harvey default = %v", got)
	}
	// The RAG default must not be the harvey wording: a green dot labelled
	// "does not meet" is worse than a green dot labelled "Green".
	rag := thLegendLabelsFor(v, thScaleRAG)
	if rag[0] != "Green" || rag[1] != "Amber" || rag[2] != "Red" {
		t.Errorf("rag default = %v, want [Green Amber Red]", rag)
	}
	if rag[0] == thLegendLabelsFor(v, thScaleHarvey)[0] {
		t.Error("the two scales share a default label")
	}

	// A RAG set of the wrong length falls back rather than indexing out of range.
	v.LegendLabelsRAG = []string{"only one"}
	if got := thLegendLabelsFor(v, thScaleRAG); len(got) != 3 || got[0] != "Green" {
		t.Errorf("short rag set = %v, want the default", got)
	}
}

func TestLegendLabelsRAGIsValidated(t *testing.T) {
	v := &TableHighlightValues{
		Criteria: []TableHighlightCriterion{{Label: "A", Scale: "rag"}, {Label: "B", Scale: "rag"}},
		Options: []TableHighlightOption{
			{Name: "One", Scores: []TableHighlightScore{"green", "red"}},
			{Name: "Two", Scores: []TableHighlightScore{"amber", "green"}},
		},
		LegendLabelsRAG: []string{"a", "b"},
	}
	err := (&tableHighlight{}).Validate(v, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "legend_labels_rag") {
		t.Errorf("a two-entry legend_labels_rag was accepted: %v", err)
	}
}

// TestLegendGetsOneRowPerScale: six entries on one row squeezed the labels to
// ~2pt; each scale now owns a row, so each label gets a third of the table.
func TestLegendGetsOneRowPerScale(t *testing.T) {
	grid := expandTableHighlightForTest(t, []string{"harvey", "rag"})
	legendRows := 0
	for _, row := range grid.Rows {
		if len(row.Cells) == 1 && row.Cells[0].Grid != nil {
			legendRows++
		}
	}
	if legendRows != 2 {
		t.Errorf("got %d legend rows for two scales, want 2", legendRows)
	}

	single := expandTableHighlightForTest(t, []string{"harvey"})
	legendRows = 0
	for _, row := range single.Rows {
		if len(row.Cells) == 1 && row.Cells[0].Grid != nil {
			legendRows++
		}
	}
	if legendRows != 1 {
		t.Errorf("got %d legend rows for one scale, want 1", legendRows)
	}
}

// TestLegendRowLabelsAreNotDuplicated reads the expanded grid: with both scales
// present no label may appear twice.
func TestLegendRowLabelsAreNotDuplicated(t *testing.T) {
	grid := expandTableHighlightForTest(t, []string{"harvey", "rag"})
	seen := map[string]int{}
	for _, row := range grid.Rows {
		if len(row.Cells) != 1 || row.Cells[0].Grid == nil {
			continue
		}
		for _, c := range row.Cells[0].Grid.Rows[0].Cells {
			if c.Shape == nil || len(c.Shape.Text) == 0 {
				continue
			}
			var txt struct {
				Paragraphs []struct {
					Content string `json:"content"`
				} `json:"paragraphs"`
			}
			if err := json.Unmarshal(c.Shape.Text, &txt); err != nil || len(txt.Paragraphs) == 0 {
				continue
			}
			seen[txt.Paragraphs[0].Content]++
		}
	}
	if len(seen) == 0 {
		t.Fatal("no legend labels found in the expanded grid")
	}
	for label, n := range seen {
		if n > 1 {
			t.Errorf("legend label %q appears %d times — the two scales share wording", label, n)
		}
	}
}

// expandTableHighlightForTest expands a two-option table using the given scales.
func expandTableHighlightForTest(t *testing.T, scales []string) *jsonschema.ShapeGridInput {
	t.Helper()
	v := &TableHighlightValues{}
	for i, s := range scales {
		v.Criteria = append(v.Criteria, TableHighlightCriterion{Label: string(rune('A' + i)), Scale: s})
	}
	score := func(scale string, high bool) TableHighlightScore {
		switch {
		case scale == "rag" && high:
			return "green"
		case scale == "rag":
			return "red"
		case high:
			return "4"
		default:
			return "0"
		}
	}
	for i := 0; i < 2; i++ {
		var row []TableHighlightScore
		for _, s := range scales {
			row = append(row, score(s, i == 0))
		}
		v.Options = append(v.Options, TableHighlightOption{Name: "Option " + string(rune('1'+i)), Scores: row})
	}
	grid, err := (&tableHighlight{}).Expand(ExpandContext{}, v, &TableHighlightOverrides{}, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return grid
}
