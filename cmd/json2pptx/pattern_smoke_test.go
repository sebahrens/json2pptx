package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

var updatePatternGolden = flag.Bool("update-pattern-golden", false, "update pattern smoke golden file")

// patternSmokeSnapshot captures the expanded grid structure for golden comparison.
type patternSmokeSnapshot struct {
	PatternName string `json:"pattern_name"`
	RowCount    int    `json:"row_count"`
	CellCounts  []int  `json:"cell_counts"` // cells per row
}

func TestPatternSmoke_AllPatterns(t *testing.T) {
	// Load the smoke deck JSON
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "patterns-smoke.json"))
	if err != nil {
		t.Fatalf("failed to read patterns-smoke.json: %v", err)
	}

	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Standard slide dimensions (10"×7.5" in EMU)
	const slideWidth int64 = 12192000
	const slideHeight int64 = 6858000

	ctx := patterns.ExpandContext{
		SlideWidth:  slideWidth,
		SlideHeight: slideHeight,
	}

	reg := patterns.Default()
	var snapshots []patternSmokeSnapshot

	for i, slide := range input.Slides {
		if slide.Pattern == nil {
			continue
		}

		t.Run(slide.Pattern.Name, func(t *testing.T) {
			// 1. Expand the pattern
			grid, _, err := expandPattern(slide.Pattern, ctx, reg)
			if err != nil {
				t.Fatalf("slide %d (%s): expandPattern failed: %v", i+1, slide.Pattern.Name, err)
			}
			if grid == nil {
				t.Fatalf("slide %d (%s): expandPattern returned nil grid", i+1, slide.Pattern.Name)
			}
			if len(grid.Rows) == 0 {
				t.Fatalf("slide %d (%s): expanded grid has no rows", i+1, slide.Pattern.Name)
			}

			// 2. Convert to shapegrid.Grid for validation
			colWidths, err := resolveColumnsDTO(grid.Columns, grid.Rows)
			if err != nil {
				t.Fatalf("slide %d (%s): resolveColumnsDTO failed: %v", i+1, slide.Pattern.Name, err)
			}

			rows := convertGridRows(grid.Rows)
			sgGrid := &shapegrid.Grid{
				Bounds:  shapegrid.DefaultBounds(slideWidth, slideHeight),
				Columns: colWidths,
				Rows:    rows,
				ColGap:  grid.ColGap,
				RowGap:  grid.RowGap,
			}

			if err := shapegrid.Validate(sgGrid); err != nil {
				t.Errorf("slide %d (%s): shapegrid.Validate failed: %v", i+1, slide.Pattern.Name, err)
			}

			// 3. Record snapshot for golden comparison
			cellCounts := make([]int, len(grid.Rows))
			for j, row := range grid.Rows {
				cellCounts[j] = len(row.Cells)
			}
			snapshots = append(snapshots, patternSmokeSnapshot{
				PatternName: slide.Pattern.Name,
				RowCount:    len(grid.Rows),
				CellCounts:  cellCounts,
			})
		})
	}

	// 4. Golden file comparison
	goldenPath := filepath.Join("testdata", "json", "patterns_smoke.golden.json")
	got, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal snapshots: %v", err)
	}

	if *updatePatternGolden {
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file not found: %s (run with -update-pattern-golden to create)", goldenPath)
	}

	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\nGot:\n%s\n\nWant:\n%s", goldenPath, string(got), string(want))
	}
}

// TestPatternSmoke_PatternCount verifies every v1 pattern is exercised.
func TestPatternSmoke_PatternCount(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "patterns-smoke.json"))
	if err != nil {
		t.Fatalf("failed to read patterns-smoke.json: %v", err)
	}

	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	expectedPatterns := map[string]bool{
		"kpi-3up":              false,
		"kpi-4up":              false,
		"bmc-canvas":           false,
		"matrix-2x2":          false,
		"timeline-horizontal": false,
		"card-grid":           false,
		"icon-row":            false,
		"comparison-2col":     false,
	}

	for _, slide := range input.Slides {
		if slide.Pattern == nil {
			continue
		}
		if _, ok := expectedPatterns[slide.Pattern.Name]; ok {
			expectedPatterns[slide.Pattern.Name] = true
		}
	}

	for name, found := range expectedPatterns {
		if !found {
			t.Errorf("pattern %q not exercised in patterns-smoke.json", name)
		}
	}
}
