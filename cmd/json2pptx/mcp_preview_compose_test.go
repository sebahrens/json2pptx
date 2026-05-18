package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// TestBuildExpandedCompose_Vertical verifies the per-segment metadata
// (pattern, cell count, bounds_pct, row_range, col_range) populated by
// buildExpandedCompose for a vertical compose envelope.
// Regression for go-slide-creator-f1ic.6.
func TestBuildExpandedCompose_Vertical(t *testing.T) {
	compose := &ComposeInput{
		Direction: "vertical",
		Segments: []SegmentInput{
			{
				SizePct: 40,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime"}`),
				},
			},
			{
				SizePct: 60,
				Pattern: PatternInput{
					Name:   "kpi-3up",
					Values: json.RawMessage(`[{"big":"$4M","small":"ARR"},{"big":"98%","small":"NRR"},{"big":"1K","small":"Cust"}]`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	merged, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}

	ec := buildExpandedCompose(compose, ctx, patterns.Default(), merged)
	if ec == nil {
		t.Fatal("buildExpandedCompose returned nil")
	}
	if ec.Direction != "vertical" {
		t.Errorf("Direction = %q, want vertical", ec.Direction)
	}
	if len(ec.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(ec.Segments))
	}

	s0, s1 := ec.Segments[0], ec.Segments[1]
	if s0.Index != 0 || s0.Pattern != "stat-hero" {
		t.Errorf("segment[0] = {Index:%d, Pattern:%q}, want {0, stat-hero}", s0.Index, s0.Pattern)
	}
	if s1.Index != 1 || s1.Pattern != "kpi-3up" {
		t.Errorf("segment[1] = {Index:%d, Pattern:%q}, want {1, kpi-3up}", s1.Index, s1.Pattern)
	}

	// bounds_pct: vertical stacks top-to-bottom, full width, heights = size_pct.
	if s0.BoundsPct.YPct != 0 || s0.BoundsPct.HeightPct != 40 || s0.BoundsPct.WidthPct != 100 {
		t.Errorf("segment[0].BoundsPct = %+v, want {x:0,y:0,w:100,h:40}", s0.BoundsPct)
	}
	if s1.BoundsPct.YPct != 40 || s1.BoundsPct.HeightPct != 60 || s1.BoundsPct.WidthPct != 100 {
		t.Errorf("segment[1].BoundsPct = %+v, want {x:0,y:40,w:100,h:60}", s1.BoundsPct)
	}

	// Row ranges must tile the merged grid (contiguous, no overlap).
	if s0.RowRange[0] != 0 {
		t.Errorf("segment[0].RowRange = %v, want start 0", s0.RowRange)
	}
	if s0.RowRange[1] != s1.RowRange[0] {
		t.Errorf("row ranges not contiguous: s0=%v s1=%v", s0.RowRange, s1.RowRange)
	}
	if s1.RowRange[1] != len(merged.Rows) {
		t.Errorf("segment[1].RowRange end = %d, want %d (merged rows)", s1.RowRange[1], len(merged.Rows))
	}
}

// TestBuildExpandedCompose_Horizontal verifies the per-segment col ranges
// and bounds for a horizontal compose envelope.
func TestBuildExpandedCompose_Horizontal(t *testing.T) {
	compose := &ComposeInput{
		Direction: "horizontal",
		Segments: []SegmentInput{
			{
				SizePct: 30,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "3x", "label": "Growth"}`),
				},
			},
			{
				SizePct: 70,
				Pattern: PatternInput{
					Name:   "stat-hero",
					Values: json.RawMessage(`{"value": "99%", "label": "Uptime"}`),
				},
			},
		},
	}

	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	merged, _, err := expandCompose(compose, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expandCompose failed: %v", err)
	}

	ec := buildExpandedCompose(compose, ctx, patterns.Default(), merged)
	if ec == nil {
		t.Fatal("buildExpandedCompose returned nil")
	}
	if ec.Direction != "horizontal" {
		t.Errorf("Direction = %q, want horizontal", ec.Direction)
	}
	if len(ec.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(ec.Segments))
	}

	s0, s1 := ec.Segments[0], ec.Segments[1]
	if s0.BoundsPct.XPct != 0 || s0.BoundsPct.WidthPct != 30 || s0.BoundsPct.HeightPct != 100 {
		t.Errorf("segment[0].BoundsPct = %+v, want {x:0,y:0,w:30,h:100}", s0.BoundsPct)
	}
	if s1.BoundsPct.XPct != 30 || s1.BoundsPct.WidthPct != 70 || s1.BoundsPct.HeightPct != 100 {
		t.Errorf("segment[1].BoundsPct = %+v, want {x:30,y:0,w:70,h:100}", s1.BoundsPct)
	}

	if s0.ColRange[1] != s1.ColRange[0] {
		t.Errorf("col ranges not contiguous: s0=%v s1=%v", s0.ColRange, s1.ColRange)
	}
}

// TestComposeWarningAsFinding verifies the structured fit-finding conversion
// from the two known compose warning shapes: COMPOSE_HORIZONTAL_TRUNCATION
// and COMPOSE_SEGMENT_BOUNDS_IGNORED. Both must carry SegmentIndex so agents
// can attribute the issue to a specific child segment.
func TestComposeWarningAsFinding(t *testing.T) {
	cases := []struct {
		name       string
		warning    string
		wantCode   string
		wantSegIdx int
		wantAction string
	}{
		{
			name:       "horizontal truncation",
			warning:    "COMPOSE_HORIZONTAL_TRUNCATION: segment[2] row[1] cells span more columns (3 so far + 2) than the segment's allocated width (4); excess cells dropped",
			wantCode:   "COMPOSE_HORIZONTAL_TRUNCATION",
			wantSegIdx: 2,
			wantAction: "shrink_or_split",
		},
		{
			name:       "bounds ignored",
			warning:    `COMPOSE_SEGMENT_BOUNDS_IGNORED: segment[1] pattern "stat-hero" sets bounds, but segment placement inside a compose envelope is governed by compose.direction + size_pct; the bounds are dropped during merge`,
			wantCode:   "COMPOSE_SEGMENT_BOUNDS_IGNORED",
			wantSegIdx: 1,
			wantAction: "review",
		},
		{
			name:    "non-segment warning is ignored",
			warning: "UNRELATED_NOTE: something happened",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := composeWarningAsFinding(0, tc.warning)
			if tc.wantCode == "" {
				if f != nil {
					t.Errorf("expected nil finding for non-matching warning, got %+v", f)
				}
				return
			}
			if f == nil {
				t.Fatalf("expected finding, got nil")
			}
			if f.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", f.Code, tc.wantCode)
			}
			if f.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", f.Action, tc.wantAction)
			}
			if f.SegmentIndex == nil || *f.SegmentIndex != tc.wantSegIdx {
				t.Errorf("SegmentIndex = %v, want %d", f.SegmentIndex, tc.wantSegIdx)
			}
			if f.Path == "" {
				t.Errorf("Path is empty; want a /slides/N/compose path")
			}
		})
	}
}

// TestAttachComposeSegmentIndex verifies that grid-cell-targeted findings on
// a compose slide get tagged with the originating segment_index based on the
// merged grid's row/col falling within a segment's RowRange/ColRange.
func TestAttachComposeSegmentIndex(t *testing.T) {
	resolved := []resolvedSlide{
		{
			SlideIndex: 0,
			ExpandedCompose: &resolvedCompose{
				Direction: "vertical",
				Segments: []resolvedComposeSegment{
					{Index: 0, RowRange: [2]int{0, 2}, ColRange: [2]int{0, 3}},
					{Index: 1, RowRange: [2]int{2, 5}, ColRange: [2]int{0, 3}},
				},
			},
		},
		{
			// Non-compose slide — findings here must remain untagged.
			SlideIndex:      1,
			ExpandedCompose: nil,
		},
	}

	findings := []patterns.FitFinding{
		{ValidationError: patterns.ValidationError{Path: slidepath.GridCell(0, 0, 1)}},
		{ValidationError: patterns.ValidationError{Path: slidepath.GridCell(0, 3, 2)}},
		{ValidationError: patterns.ValidationError{Path: slidepath.GridCell(1, 0, 0)}},
		{ValidationError: patterns.ValidationError{Path: slidepath.SlideField(0, "layout_id")}}, // non-cell path
		// Pre-tagged finding must not be overwritten.
		{
			ValidationError: patterns.ValidationError{Path: slidepath.GridCell(0, 0, 0)},
			SegmentIndex:    intPtr(99),
		},
	}

	attachComposeSegmentIndex(findings, resolved)

	if findings[0].SegmentIndex == nil || *findings[0].SegmentIndex != 0 {
		t.Errorf("findings[0].SegmentIndex = %v, want 0", findings[0].SegmentIndex)
	}
	if findings[1].SegmentIndex == nil || *findings[1].SegmentIndex != 1 {
		t.Errorf("findings[1].SegmentIndex = %v, want 1", findings[1].SegmentIndex)
	}
	if findings[2].SegmentIndex != nil {
		t.Errorf("findings[2].SegmentIndex = %v, want nil (non-compose slide)", findings[2].SegmentIndex)
	}
	if findings[3].SegmentIndex != nil {
		t.Errorf("findings[3].SegmentIndex = %v, want nil (non-cell path)", findings[3].SegmentIndex)
	}
	if findings[4].SegmentIndex == nil || *findings[4].SegmentIndex != 99 {
		t.Errorf("findings[4].SegmentIndex = %v, want 99 (pre-tagged preserved)", findings[4].SegmentIndex)
	}
}

// TestComposeErrorAsFinding verifies that the wrapped "compose: segment[N]:..."
// error from expandCompose is converted to a structured fit finding tagged
// with the offending segment index and action=refuse.
func TestComposeErrorAsFinding(t *testing.T) {
	cases := []struct {
		name       string
		errMsg     string
		wantSegIdx int
		wantNil    bool
	}{
		{name: "segment expand error", errMsg: "compose: segment[3]: card-grid: cells[0].header is required", wantSegIdx: 3},
		{name: "unrelated error returns nil", errMsg: "compose: direction must be vertical or horizontal", wantNil: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := composeErrorAsFinding(7, errString(tc.errMsg))
			if tc.wantNil {
				if f != nil {
					t.Errorf("expected nil, got %+v", f)
				}
				return
			}
			if f == nil {
				t.Fatalf("expected finding, got nil")
			}
			if f.Action != "refuse" {
				t.Errorf("Action = %q, want refuse", f.Action)
			}
			if f.SegmentIndex == nil || *f.SegmentIndex != tc.wantSegIdx {
				t.Errorf("SegmentIndex = %v, want %d", f.SegmentIndex, tc.wantSegIdx)
			}
			if f.Code != "COMPOSE_SEGMENT_EXPAND_FAILED" {
				t.Errorf("Code = %q, want COMPOSE_SEGMENT_EXPAND_FAILED", f.Code)
			}
		})
	}
}

// jsonschema import used to keep the package compile-self-consistent even if
// future cases reference its types directly. Tests above use the package via
// SegmentInput/PatternInput which embed it.
var _ = jsonschema.ShapeGridInput{}

// errString returns an error that wraps msg. Defined here (rather than using
// errors.New inline) so the test file can keep the import list minimal.
func errString(msg string) error { return composeTestErr(msg) }

type composeTestErr string

func (e composeTestErr) Error() string { return string(e) }
