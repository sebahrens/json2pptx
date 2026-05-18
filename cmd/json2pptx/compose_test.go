package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestExpandCompose_Vertical(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				SizePct: 40,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime SLA"}`),
				},
			},
			{
				SizePct: 60,
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "$4M", "small": "ARR"}, {"big": "98%", "small": "NRR"}, {"big": "1K", "small": "Customers"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandCompose returned nil grid")
	}
	if len(grid.Rows) == 0 {
		t.Fatal("merged grid has no rows")
	}

	// Verify the merged grid has valid row heights
	totalHeight := 0.0
	for _, row := range grid.Rows {
		totalHeight += row.Height
	}
	// Should sum to approximately 100% (40 + 60)
	if totalHeight < 99 || totalHeight > 101 {
		t.Errorf("row heights should sum to ~100%%, got %.1f%%", totalHeight)
	}
}

func TestExpandCompose_Horizontal(t *testing.T) {
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{
				SizePct: 50,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "3x", "label": "Growth"}`),
				},
			},
			{
				SizePct: 50,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime"}`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandCompose returned nil grid")
	}

	// Should have columns matching segment count
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil {
		t.Fatalf("columns should be array of percentages: %v", err)
	}
	if len(cols) != 2 {
		t.Errorf("expected 2 columns, got %d", len(cols))
	}
}

func TestExpandCompose_EqualSplit(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "A", "label": "First"}`),
				},
			},
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "B", "label": "Second"}`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}

	// Equal split: each segment gets 50%
	totalHeight := 0.0
	for _, row := range grid.Rows {
		totalHeight += row.Height
	}
	if totalHeight < 99 || totalHeight > 101 {
		t.Errorf("row heights should sum to ~100%%, got %.1f%%", totalHeight)
	}
}

func TestValidateCompose_Errors(t *testing.T) {
	tests := []struct {
		name      string
		compose   ComposeInput
		wantErr   string
		wantNoErr bool
	}{
		{
			name: "invalid direction",
			compose: ComposeInput{
				Direction: "diagonal",
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "direction must be",
		},
		{
			name: "too few segments",
			compose: ComposeInput{
				Direction: "vertical",
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "at least 2 segments",
		},
		{
			name: "too many segments",
			compose: ComposeInput{
				Direction: "vertical",
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "maximum 8 segments",
		},
		{
			name: "exactly at cap (8 segments) passes structural checks",
			compose: ComposeInput{
				Direction: "vertical",
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			// no wantErr — this is the boundary case
			wantNoErr: true,
		},
		{
			name: "size exceeds 100",
			compose: ComposeInput{
				Direction: "vertical",
				Segments: []SegmentInput{
					{SizePct: 70, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
					{SizePct: 50, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			},
			wantErr: "exceeds 100%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCompose(&tt.compose)
			if tt.wantNoErr {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestExpandCompose_SmartCompose_StatHeroKpi3up(t *testing.T) {
	compose := &ComposeInput{
		Direction:    "vertical",
		SmartCompose: true,
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime SLA"}`),
				},
			},
			{
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "$4M", "small": "ARR"}, {"big": "98%", "small": "NRR"}, {"big": "1K", "small": "Customers"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandCompose returned nil grid")
	}

	// stat-hero has 1 cell, kpi-3up has 3 cells -> 1:3 ratio
	// hero should get ~25%, kpi should get ~75%
	// Collect heights by segment: stat-hero rows come first
	heroRows := 0
	kpiRows := 0
	var heroHeight, kpiHeight float64
	for _, row := range grid.Rows {
		if heroRows == 0 || kpiHeight == 0 {
			// First row(s) belong to stat-hero (1 row in stat-hero)
			heroHeight += row.Height
			heroRows++
			if heroRows == 1 {
				// stat-hero only has 1 row, switch to kpi
				continue
			}
		}
		kpiHeight += row.Height
		kpiRows++
	}
	// Re-collect properly: stat-hero = 1 row, kpi-3up = 1 row
	// So grid.Rows[0] = hero, grid.Rows[1] = kpi
	if len(grid.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(grid.Rows))
	}
	heroHeight = grid.Rows[0].Height
	kpiHeight = grid.Rows[1].Height

	// hero should be ~25% (1/(1+3)*100), kpi ~75% (3/(1+3)*100)
	if heroHeight < 20 || heroHeight > 30 {
		t.Errorf("hero height should be ~25%%, got %.1f%%", heroHeight)
	}
	if kpiHeight < 70 || kpiHeight > 80 {
		t.Errorf("kpi height should be ~75%%, got %.1f%%", kpiHeight)
	}
}

func TestExpandCompose_SmartCompose_ExplicitSizePctOverrides(t *testing.T) {
	// When explicit SizePct is set, smart_compose should NOT override it
	compose := &ComposeInput{
		Direction:    "vertical",
		SmartCompose: true,
		Segments: []SegmentInput{
			{
				SizePct: 60,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime"}`),
				},
			},
			{
				SizePct: 40,
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "A", "small": "X"}, {"big": "B", "small": "Y"}, {"big": "C", "small": "Z"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if len(grid.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(grid.Rows))
	}

	// Explicit sizes: hero=60%, kpi=40% — smart compose doesn't override
	totalHeight := 0.0
	for _, row := range grid.Rows {
		totalHeight += row.Height
	}
	if totalHeight < 99 || totalHeight > 101 {
		t.Errorf("row heights should sum to ~100%%, got %.1f%%", totalHeight)
	}
}

func TestExpandCompose_SmartCompose_FalseUsesEqualSplit(t *testing.T) {
	compose := &ComposeInput{
		Direction:    "vertical",
		SmartCompose: false,
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "A", "label": "X"}`),
				},
			},
			{
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "1", "small": "a"}, {"big": "2", "small": "b"}, {"big": "3", "small": "c"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if len(grid.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(grid.Rows))
	}

	// Without smart compose, both segments get 50%
	heroHeight := grid.Rows[0].Height
	kpiHeight := grid.Rows[1].Height
	if heroHeight < 49 || heroHeight > 51 {
		t.Errorf("hero height should be ~50%% without smart compose, got %.1f%%", heroHeight)
	}
	if kpiHeight < 49 || kpiHeight > 51 {
		t.Errorf("kpi height should be ~50%% without smart compose, got %.1f%%", kpiHeight)
	}
}

func TestExpandCompose_ChildValidationBubbles(t *testing.T) {
	// stat-hero requires value and label — omit them to trigger child validation error
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "", "label": ""}`),
				},
			},
			{
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big": "X", "small": "Y"}, {"big": "A", "small": "B"}, {"big": "C", "small": "D"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	_, _, err := expandCompose(compose, ctx, patterns.Default())
	if err == nil {
		t.Fatal("expected validation error from child pattern")
	}
	if !contains(err.Error(), "segment[0]") {
		t.Errorf("error should reference segment index, got: %v", err)
	}
}

// TestExpandCompose_Horizontal_PreservesAllCells is the regression test for
// the mergeHorizontal silent-data-loss bug (go-slide-creator-f1ic.4). Before
// the fix, mergeRowCells collapsed each segment row to a single cell, so a
// horizontal compose of three iconrow/kpi-3up segments dropped 2/3 of every
// segment's cards. After the fix, the merged grid must contain every input
// cell — total content-bearing cells must equal the sum of input cells
// across all segments.
func TestExpandCompose_Horizontal_PreservesAllCells(t *testing.T) {
	threeKpiValues := `[{"big": "A", "small": "1"}, {"big": "B", "small": "2"}, {"big": "C", "small": "3"}]`
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "kpi-3up", Values: json.RawMessage(threeKpiValues)}},
			{Pattern: PatternInput{Name: "kpi-3up", Values: json.RawMessage(threeKpiValues)}},
			{Pattern: PatternInput{Name: "kpi-3up", Values: json.RawMessage(threeKpiValues)}},
		},
	}
	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	// Count input content-bearing cells per segment.
	inputCells := 0
	for i, seg := range compose.Segments {
		g, _, err := expandPattern(&seg.Pattern, ctx, patterns.Default())
		if err != nil {
			t.Fatalf("segment[%d] expandPattern failed: %v", i, err)
		}
		for _, row := range g.Rows {
			for _, c := range row.Cells {
				if cellHasContent(c) {
					inputCells++
				}
			}
		}
	}

	merged, warnings, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no truncation warnings, got: %v", warnings)
	}

	mergedCells := 0
	for _, row := range merged.Rows {
		for _, c := range row.Cells {
			if cellHasContent(c) {
				mergedCells++
			}
		}
	}
	if mergedCells != inputCells {
		t.Errorf("merged cell count %d != sum of input cells %d (silent data loss regression)", mergedCells, inputCells)
	}

	// Merged grid should expose all 9 columns (3 segments × 3 cols each).
	var cols []float64
	if err := json.Unmarshal(merged.Columns, &cols); err != nil {
		t.Fatalf("merged columns should be array of percentages: %v", err)
	}
	if len(cols) != 9 {
		t.Errorf("expected 9 columns (3 segments × 3 cols), got %d", len(cols))
	}
}

// TestExpandCompose_Horizontal_TruncationWarning verifies that when a
// segment row's ColSpan-weighted occupancy overflows the segment's allocated
// column range, the dropped excess cells produce a COMPOSE_HORIZONTAL_TRUNCATION
// warning instead of being silently shed.
func TestExpandCompose_Horizontal_TruncationWarning(t *testing.T) {
	// Hand-craft two grids directly so we can force over-occupancy.
	// Segment 0 reports columns=2 but its row has 3 cells (each ColSpan=1) — overflow by 1.
	seg0 := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}},
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}},
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}},
			},
		}},
	}
	seg1 := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{
				{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}},
			},
		}},
	}

	merged, warnings, err := mergeHorizontal(
		[]*jsonschema.ShapeGridInput{seg0, seg1},
		[]float64{50, 50},
		0,
	)
	if err != nil {
		t.Fatalf("mergeHorizontal failed: %v", err)
	}
	if merged == nil {
		t.Fatal("mergeHorizontal returned nil grid")
	}

	foundTruncation := false
	for _, w := range warnings {
		if contains(w, "COMPOSE_HORIZONTAL_TRUNCATION") && contains(w, "segment[0]") {
			foundTruncation = true
			break
		}
	}
	if !foundTruncation {
		t.Errorf("expected COMPOSE_HORIZONTAL_TRUNCATION warning for segment[0], got: %v", warnings)
	}
}

// TestExpandCompose_SegmentBoundsIgnoredWarning is the regression test for
// go-slide-creator-f1ic.7. Before the fix, PatternInput.Bounds on a compose
// segment was accepted by the unmarshaler but mergeVertical/mergeHorizontal
// never referenced it — the bounds were silently dropped during merge. After
// the fix, expandCompose surfaces COMPOSE_SEGMENT_BOUNDS_IGNORED so agents
// know the bounds were not honored.
func TestExpandCompose_SegmentBoundsIgnoredWarning(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				SizePct: 50,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "A", "label": "First"}`),
				},
			},
			{
				SizePct: 50,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "B", "label": "Second"}`),
					// Explicit bounds on a compose segment — cannot be honored.
					Bounds: &jsonschema.GridBoundsInput{
						X: 60, Y: 60, Width: 35, Height: 35,
					},
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	grid, warnings, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandCompose returned nil grid")
	}

	foundBoundsIgnored := false
	for _, w := range warnings {
		if contains(w, "COMPOSE_SEGMENT_BOUNDS_IGNORED") && contains(w, "segment[1]") {
			foundBoundsIgnored = true
			break
		}
	}
	if !foundBoundsIgnored {
		t.Errorf("expected COMPOSE_SEGMENT_BOUNDS_IGNORED warning for segment[1], got: %v", warnings)
	}

	// Sanity: a segment that does NOT set bounds must NOT trigger the warning.
	for _, w := range warnings {
		if contains(w, "COMPOSE_SEGMENT_BOUNDS_IGNORED") && contains(w, "segment[0]") {
			t.Errorf("did not expect warning for segment[0] (no bounds set): %v", w)
		}
	}
}

// TestExpandCompose_SegmentMaxHeightPctIgnoredWarning covers the
// max_height_pct convenience alias on the same code path: it is a thin
// shorthand over Bounds and must also surface COMPOSE_SEGMENT_BOUNDS_IGNORED
// when used inside a compose segment.
func TestExpandCompose_SegmentMaxHeightPctIgnoredWarning(t *testing.T) {
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{
				Pattern: PatternInput{
					Name:         "stat-hero",
					Values:       json.RawMessage(`{"value": "A", "label": "X"}`),
					MaxHeightPct: 40,
				},
			},
			{
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "B", "label": "Y"}`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}

	_, warnings, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}

	found := false
	for _, w := range warnings {
		if contains(w, "COMPOSE_SEGMENT_BOUNDS_IGNORED") && contains(w, "segment[0]") && contains(w, "max_height_pct") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected COMPOSE_SEGMENT_BOUNDS_IGNORED warning mentioning max_height_pct for segment[0], got: %v", warnings)
	}
}

// TestExpandCompose_NoBoundsNoBoundsWarning ensures the bounds-ignored
// warning is not emitted when no segment sets bounds.
func TestExpandCompose_NoBoundsNoBoundsWarning(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "A", "label": "X"}`)}},
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "B", "label": "Y"}`)}},
		},
	}
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}

	_, warnings, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	for _, w := range warnings {
		if contains(w, "COMPOSE_SEGMENT_BOUNDS_IGNORED") {
			t.Errorf("did not expect COMPOSE_SEGMENT_BOUNDS_IGNORED warning when no segment sets bounds: %v", w)
		}
	}
}

// cellHasContent reports whether a GridCellInput holds renderable content
// (used by horizontal-preservation tests to ignore empty padding cells).
func cellHasContent(c *jsonschema.GridCellInput) bool {
	if c == nil {
		return false
	}
	return c.Shape != nil || c.Table != nil || c.Icon != nil ||
		c.Image != nil || c.Diagram != nil
}

// TestExpandCompose_NestedVerticalContainingHorizontal verifies that a
// vertical compose envelope whose middle segment nests a horizontal compose
// (banner / pillars-row / foundation) expands cleanly and the merged grid
// preserves every leaf pattern's content cells. This is the canonical use
// case from go-slide-creator-f1ic.2.
func TestExpandCompose_NestedVerticalContainingHorizontal(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				SizePct: 20,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "Mission", "label": "Banner"}`),
				},
			},
			{
				SizePct: 60,
				Compose: &ComposeInput{
					Direction: "horizontal",
					Segments: []SegmentInput{
						{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "P1", "label": "Pillar A"}`)}},
						{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "P2", "label": "Pillar B"}`)}},
						{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "P3", "label": "Pillar C"}`)}},
					},
				},
			},
			{
				SizePct: 20,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "Foundation", "label": "Footer"}`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	grid, warnings, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expandCompose returned nil grid")
	}
	for _, w := range warnings {
		if contains(w, "COMPOSE_HORIZONTAL_TRUNCATION") {
			t.Errorf("nested expansion should not truncate cells: %v", w)
		}
	}

	// Count cells with rendered content across the merged grid; every leaf
	// pattern should contribute at least one content cell, so we expect >=5.
	var contentCells int
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cellHasContent(cell) {
				contentCells++
			}
		}
	}
	if contentCells < 5 {
		t.Errorf("nested compose should preserve all five leaf patterns' content; got %d content cells", contentCells)
	}

	// Row heights from the three outer segments should sum to ~100%.
	totalHeight := 0.0
	for _, row := range grid.Rows {
		totalHeight += row.Height
	}
	if totalHeight < 99 || totalHeight > 101 {
		t.Errorf("outer row heights should sum to ~100%%, got %.2f", totalHeight)
	}
}

// TestValidateCompose_NestedDepth ensures the validator accepts depth-2
// envelopes, rejects depth-3 envelopes, and applies the same structural
// checks (e.g., segment count) inside nested envelopes.
func TestValidateCompose_NestedDepth(t *testing.T) {
	makeLeaf := func() SegmentInput {
		return SegmentInput{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}}
	}

	t.Run("depth 2 accepted", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "vertical",
			Segments: []SegmentInput{
				makeLeaf(),
				{Compose: &ComposeInput{
					Direction: "horizontal",
					Segments:  []SegmentInput{makeLeaf(), makeLeaf()},
				}},
			},
		}
		if err := validateCompose(c); err != nil {
			t.Fatalf("expected depth=2 envelope to validate, got %v", err)
		}
	})

	t.Run("depth 3 rejected", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "vertical",
			Segments: []SegmentInput{
				makeLeaf(),
				{Compose: &ComposeInput{
					Direction: "vertical",
					Segments: []SegmentInput{
						makeLeaf(),
						{Compose: &ComposeInput{
							Direction: "horizontal",
							Segments:  []SegmentInput{makeLeaf(), makeLeaf()},
						}},
					},
				}},
			},
		}
		err := validateCompose(c)
		if err == nil || !contains(err.Error(), "nesting depth") {
			t.Fatalf("expected nesting-depth error, got %v", err)
		}
	})

	t.Run("inner envelope must pass structural checks", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "vertical",
			Segments: []SegmentInput{
				makeLeaf(),
				{Compose: &ComposeInput{
					Direction: "horizontal",
					Segments:  []SegmentInput{makeLeaf()}, // only 1 — invalid
				}},
			},
		}
		err := validateCompose(c)
		if err == nil || !contains(err.Error(), "at least 2 segments") {
			t.Fatalf("expected inner-envelope min-segments error, got %v", err)
		}
	})
}

// TestValidateCompose_SegmentXOR enforces that each segment carries exactly
// one of pattern or compose. Specifying both or neither must be rejected
// because the expansion path is otherwise ambiguous.
func TestValidateCompose_SegmentXOR(t *testing.T) {
	leaf := SegmentInput{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}}
	innerCompose := &ComposeInput{
		Direction: "horizontal",
		Segments:  []SegmentInput{leaf, leaf},
	}

	t.Run("both pattern and compose rejected", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "vertical",
			Segments: []SegmentInput{
				leaf,
				{
					Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)},
					Compose: innerCompose,
				},
			},
		}
		err := validateCompose(c)
		if err == nil || !contains(err.Error(), "more than one") {
			t.Fatalf("expected XOR error, got %v", err)
		}
	})

	t.Run("neither pattern nor compose rejected", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "vertical",
			Segments:  []SegmentInput{leaf, {}},
		}
		err := validateCompose(c)
		if err == nil || !contains(err.Error(), "must set exactly one") {
			t.Fatalf("expected XOR error, got %v", err)
		}
	})
}

// TestValidateCompose_TotalLeafCap caps the total number of leaf patterns
// across the entire envelope tree so nesting cannot smuggle more patterns
// past the per-envelope max_segments cap.
func TestValidateCompose_TotalLeafCap(t *testing.T) {
	makeLeaf := func() SegmentInput {
		return SegmentInput{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}}
	}
	// Outer (vertical): 2 leaves + 1 nested compose.
	// Nested (horizontal): max_segments = 8 leaves.
	// Total leaves = 2 + 8 = 10 (still <= 12). Make outer reach 5 leaves so
	// total = 5 + 8 = 13 > 12 to trip the cap.
	outerSegs := []SegmentInput{makeLeaf(), makeLeaf(), makeLeaf(), makeLeaf(), makeLeaf()}
	inner := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			makeLeaf(), makeLeaf(), makeLeaf(), makeLeaf(),
			makeLeaf(), makeLeaf(), makeLeaf(), makeLeaf(),
		},
	}
	outerSegs = append(outerSegs, SegmentInput{Compose: inner})
	c := &ComposeInput{Direction: "vertical", Segments: outerSegs}
	err := validateCompose(c)
	if err == nil || !contains(err.Error(), "total leaf patterns") {
		t.Fatalf("expected total-leaf-cap error, got %v", err)
	}
}

// TestCountComposeLeafPatterns checks the leaf counter walks nested
// envelopes correctly so the cap enforcement is anchored on the real total.
func TestCountComposeLeafPatterns(t *testing.T) {
	makeLeaf := func() SegmentInput {
		return SegmentInput{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}}
	}
	c := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			makeLeaf(),
			{Compose: &ComposeInput{
				Direction: "horizontal",
				Segments:  []SegmentInput{makeLeaf(), makeLeaf(), makeLeaf()},
			}},
			makeLeaf(),
		},
	}
	got := countComposeLeafPatterns(c)
	if got != 5 {
		t.Errorf("countComposeLeafPatterns = %d, want 5", got)
	}
}

// TestExpandCompose_Banner verifies that ComposeInput.Banner is rendered as a
// full-width row prepended to the merged grid, without consuming a segment
// slot. The banner cell carries the requested accent fill and ColSpan matches
// the merged grid's column count.
func TestExpandCompose_Banner(t *testing.T) {
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "A", "label": "First"}`)}},
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "B", "label": "Second"}`)}},
		},
		Banner: &patterns.BannerSpec{
			Text:   "Strategic North Star",
			Accent: "accent2",
		},
	}
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil || len(grid.Rows) == 0 {
		t.Fatal("expected non-empty merged grid")
	}

	banner := grid.Rows[0]
	if len(banner.Cells) != 1 {
		t.Fatalf("banner row should have exactly 1 cell, got %d", len(banner.Cells))
	}
	cell := banner.Cells[0]
	if cell == nil || cell.Shape == nil {
		t.Fatal("banner cell missing shape")
	}
	totalCols := inferColumnCount(grid)
	if cell.ColSpan != totalCols {
		t.Errorf("banner ColSpan = %d, want %d (full width)", cell.ColSpan, totalCols)
	}
	if !banner.AutoHeight {
		t.Errorf("banner row should use auto_height, got false")
	}
	if !bytes.Contains(cell.Shape.Fill, []byte("accent2")) {
		t.Errorf("banner fill should reference accent2, got %s", string(cell.Shape.Fill))
	}
	if !bytes.Contains(cell.Shape.Text, []byte("Strategic North Star")) {
		t.Errorf("banner text not rendered: %s", string(cell.Shape.Text))
	}
}

// TestExpandCompose_Callout verifies that ComposeInput.Callout reuses the
// existing pattern-level callout decorator, appending a full-width row at
// the bottom of the merged grid.
func TestExpandCompose_Callout(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "A", "label": "First"}`)}},
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "B", "label": "Second"}`)}},
		},
		Callout: &patterns.PatternCallout{Text: "Bottom line: ship faster."},
	}
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil || len(grid.Rows) == 0 {
		t.Fatal("expected non-empty merged grid")
	}

	last := grid.Rows[len(grid.Rows)-1]
	if len(last.Cells) != 1 || last.Cells[0] == nil || last.Cells[0].Shape == nil {
		t.Fatal("callout row missing shape cell")
	}
	if !bytes.Contains(last.Cells[0].Shape.Text, []byte("Bottom line")) {
		t.Errorf("callout text not rendered: %s", string(last.Cells[0].Shape.Text))
	}
}

// TestExpandCompose_BannerAndCallout verifies that banner is at row 0 and
// callout is at the last row when both are set, and that segments occupy the
// rows in between.
func TestExpandCompose_BannerAndCallout(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "A", "label": "First"}`)}},
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value": "B", "label": "Second"}`)}},
		},
		Banner:  &patterns.BannerSpec{Text: "Top band"},
		Callout: &patterns.PatternCallout{Text: "Bottom band"},
	}
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}

	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if len(grid.Rows) < 3 {
		t.Fatalf("expected at least banner + segment + callout rows, got %d", len(grid.Rows))
	}

	first := grid.Rows[0]
	last := grid.Rows[len(grid.Rows)-1]
	if first.Cells[0] == nil || first.Cells[0].Shape == nil ||
		!bytes.Contains(first.Cells[0].Shape.Text, []byte("Top band")) {
		t.Errorf("banner not at row 0")
	}
	if last.Cells[0] == nil || last.Cells[0].Shape == nil ||
		!bytes.Contains(last.Cells[0].Shape.Text, []byte("Bottom band")) {
		t.Errorf("callout not at last row")
	}
}

// TestValidateCompose_BannerVsBannerLikeFirstSegment verifies that the
// validator rejects an envelope-level banner when the first segment is itself
// banner-leading (strategy-house or pull-quote), preventing duplicate banners.
func TestValidateCompose_BannerVsBannerLikeFirstSegment(t *testing.T) {
	cases := []struct {
		name        string
		firstName   string
		wantRejected bool
	}{
		{"strategy-house first segment is rejected", "strategy-house", true},
		{"pull-quote first segment is rejected", "pull-quote", true},
		{"stat-hero first segment is allowed", "stat-hero", false},
		{"kpi-3up first segment is allowed", "kpi-3up", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &ComposeInput{
				Direction: "vertical",
				Banner:    &patterns.BannerSpec{Text: "Top band"},
				Segments: []SegmentInput{
					{Pattern: PatternInput{Name: tc.firstName, Values: json.RawMessage(`{}`)}},
					{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
				},
			}
			err := validateCompose(c)
			if tc.wantRejected && err == nil {
				t.Errorf("expected validateCompose to reject banner + %s first segment, got nil", tc.firstName)
			}
			if !tc.wantRejected && err != nil {
				t.Errorf("expected validateCompose to accept %s first segment with banner, got %v", tc.firstName, err)
			}
		})
	}
}

// TestValidateCompose_BannerOnlyChecksFirstSegment verifies that a banner-like
// pattern in a non-first segment slot does NOT trigger the duplicate-banner
// rejection — only the first segment governs the visual conflict.
func TestValidateCompose_BannerOnlyChecksFirstSegment(t *testing.T) {
	c := &ComposeInput{
		Direction: "vertical",
		Banner:    &patterns.BannerSpec{Text: "Top band"},
		Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}},
			{Pattern: PatternInput{Name: "pull-quote", Values: json.RawMessage(`{}`)}},
		},
	}
	if err := validateCompose(c); err != nil {
		t.Errorf("expected validateCompose to accept banner with pull-quote in second slot, got %v", err)
	}
}

// TestExpandCompose_DiagramSegment exercises the diagram-segment XOR
// alternative introduced for go-slide-creator-zg8q.6. A native pattern
// (pyramid) is placed in one half of the merged grid and an svggen
// process_flow diagram is placed in the other half — without flattening
// the pattern through a single-cell grid.
func TestExpandCompose_DiagramSegment(t *testing.T) {
	diagram := &types.DiagramSpec{
		Type: "process_flow",
		Data: map[string]any{"steps": []any{"Plan", "Build", "Ship"}},
	}
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{
				SizePct: 50,
				Pattern: PatternInput{
					Name:   "pyramid",
					Values: json.RawMessage(`{"tiers": ["Top", "Mid", "Base"]}`),
				},
			},
			{
				SizePct: 50,
				Diagram: diagram,
			},
		},
	}

	ctx := patterns.ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
	}
	grid, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}
	if grid == nil || len(grid.Rows) == 0 {
		t.Fatal("expected non-empty merged grid")
	}

	// Confirm the diagram cell survives the horizontal merge — it should
	// appear in the rightmost segment's column allocation.
	var found bool
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell != nil && cell.Diagram == diagram {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("diagram segment payload not present in merged grid cells")
	}

	// The merged grid must use the compose-level gap as its column gap so
	// gutter rhythm is unified across pattern and diagram segments.
	if grid.ColGap == 0 {
		t.Errorf("expected non-zero ColGap on horizontally-merged grid")
	}
}

// TestValidateCompose_DiagramSegmentXOR verifies the 3-way XOR contract
// between pattern, compose, and diagram segments.
func TestValidateCompose_DiagramSegmentXOR(t *testing.T) {
	leaf := SegmentInput{Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)}}
	diagram := &types.DiagramSpec{Type: "bar_chart"}

	t.Run("pattern_and_diagram_rejected", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "horizontal",
			Segments: []SegmentInput{
				leaf,
				{
					Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{}`)},
					Diagram: diagram,
				},
			},
		}
		err := validateCompose(c)
		if err == nil || !contains(err.Error(), "more than one") {
			t.Fatalf("expected XOR error, got %v", err)
		}
	})

	t.Run("diagram_segment_alone_accepted", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "horizontal",
			Segments:  []SegmentInput{leaf, {Diagram: diagram}},
		}
		if err := validateCompose(c); err != nil {
			t.Errorf("expected validateCompose to accept diagram segment, got %v", err)
		}
	})

	t.Run("diagram_segment_without_type_rejected", func(t *testing.T) {
		c := &ComposeInput{
			Direction: "horizontal",
			Segments:  []SegmentInput{leaf, {Diagram: &types.DiagramSpec{}}},
		}
		err := validateCompose(c)
		if err == nil || !contains(err.Error(), "diagram.type is required") {
			t.Fatalf("expected diagram.type required error, got %v", err)
		}
	})
}

// TestComposeCapabilities_AdvertisesDiagramSegments verifies that the
// capability descriptor surfaces the supports_diagram_segments flag so
// agents can discover the feature without trial-and-error.
func TestComposeCapabilities_AdvertisesDiagramSegments(t *testing.T) {
	caps := composeCapabilities()
	if !caps.SupportsDiagramSegments {
		t.Error("composeCapabilities().SupportsDiagramSegments must be true")
	}
}
