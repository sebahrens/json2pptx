package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestValidateGridConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *GridConfig
		wantErr bool
	}{
		{name: "nil is valid", cfg: nil, wantErr: false},
		{name: "empty config is valid", cfg: &GridConfig{}, wantErr: false},
		{name: "valid full config", cfg: &GridConfig{
			Columns:          12,
			GutterEMU:        228600,
			TitleBaselinePct: 8,
			ContentTopPct:    15,
			ContentBottomPct: 92,
			LeftMarginPct:    5,
			RightMarginPct:   5,
		}, wantErr: false},
		{name: "columns out of range", cfg: &GridConfig{Columns: 30}, wantErr: true},
		{name: "negative gutter", cfg: &GridConfig{GutterEMU: -1}, wantErr: true},
		{name: "pct over 100", cfg: &GridConfig{ContentTopPct: 101}, wantErr: true},
		{name: "pct negative", cfg: &GridConfig{ContentTopPct: -5}, wantErr: true},
		{name: "title_baseline >= content_top", cfg: &GridConfig{
			TitleBaselinePct: 20,
			ContentTopPct:    15,
		}, wantErr: true},
		{name: "content_top >= content_bottom", cfg: &GridConfig{
			ContentTopPct:    80,
			ContentBottomPct: 70,
		}, wantErr: true},
		{name: "margins sum >= 100", cfg: &GridConfig{
			LeftMarginPct:  60,
			RightMarginPct: 50,
		}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGridConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGridConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveGrid_ExplicitValues(t *testing.T) {
	cfg := &GridConfig{
		TitleBaselinePct: 10,
		ContentTopPct:    15,
		ContentBottomPct: 90,
		LeftMarginPct:    5,
		RightMarginPct:   5,
	}

	sw := shapegrid.DefaultSlideWidthEMU  // 12192000
	sh := shapegrid.DefaultSlideHeightEMU // 6858000

	rg := resolveGrid(cfg, nil, sw, sh)

	// Title baseline at 10% of slide height.
	wantTitle := shapegrid.PctToEMU(10, sh)
	if rg.TitleBaselineY != wantTitle {
		t.Errorf("TitleBaselineY = %d, want %d", rg.TitleBaselineY, wantTitle)
	}

	// Content top at 15%.
	wantTop := shapegrid.PctToEMU(15, sh)
	if rg.ContentTopY != wantTop {
		t.Errorf("ContentTopY = %d, want %d", rg.ContentTopY, wantTop)
	}

	// Content bottom at 90%.
	wantBottom := shapegrid.PctToEMU(90, sh)
	if rg.ContentBottomY != wantBottom {
		t.Errorf("ContentBottomY = %d, want %d", rg.ContentBottomY, wantBottom)
	}

	// Left margin at 5%.
	wantLeft := shapegrid.PctToEMU(5, sw)
	if rg.LeftMarginX != wantLeft {
		t.Errorf("LeftMarginX = %d, want %d", rg.LeftMarginX, wantLeft)
	}

	// Right edge = slide width - 5%.
	wantRight := sw - shapegrid.PctToEMU(5, sw)
	if rg.RightEdgeX != wantRight {
		t.Errorf("RightEdgeX = %d, want %d", rg.RightEdgeX, wantRight)
	}
}

func TestResolveGrid_TemplateDefaults(t *testing.T) {
	// Empty grid config should derive values from layout metadata.
	cfg := &GridConfig{}

	layouts := []types.LayoutMetadata{
		{
			ID: "slideLayout1",
			Placeholders: []types.PlaceholderInfo{
				{
					Type: types.PlaceholderTitle,
					Bounds: types.BoundingBox{
						X: 600000, Y: 200000, Width: 10000000, Height: 400000,
					},
				},
				{
					Type: types.PlaceholderBody,
					Bounds: types.BoundingBox{
						X: 600000, Y: 700000, Width: 10000000, Height: 5000000,
					},
				},
			},
		},
	}

	sw := shapegrid.DefaultSlideWidthEMU
	sh := shapegrid.DefaultSlideHeightEMU

	rg := resolveGrid(cfg, layouts, sw, sh)

	// Title baseline should come from the title placeholder: Y + Height = 600000.
	if rg.TitleBaselineY != 600000 {
		t.Errorf("TitleBaselineY = %d, want 600000 (from template title placeholder)", rg.TitleBaselineY)
	}

	// Content top should come from body placeholder Y.
	if rg.ContentTopY != 700000 {
		t.Errorf("ContentTopY = %d, want 700000 (from template body placeholder)", rg.ContentTopY)
	}

	// Left margin from body placeholder X.
	if rg.LeftMarginX != 600000 {
		t.Errorf("LeftMarginX = %d, want 600000 (from template body placeholder)", rg.LeftMarginX)
	}
}

func TestSnapBoundsToGrid(t *testing.T) {
	rg := &resolvedGrid{
		TitleBaselineY: 600000,
		ContentTopY:    800000,
		ContentBottomY: 6200000,
		LeftMarginX:    500000,
		RightEdgeX:     11500000,
		SlideWidth:     shapegrid.DefaultSlideWidthEMU,
		SlideHeight:    shapegrid.DefaultSlideHeightEMU,
	}

	// Bounds that are slightly off from the grid.
	bounds := pptx.RectEmu{
		X:  480000,   // slightly left of grid
		Y:  850000,   // slightly below content top
		CX: 11000000, // width
		CY: 5300000,  // height
	}

	snapped, adjustments := snapBoundsToGrid(bounds, rg)

	// Should snap to grid positions.
	if snapped.X != rg.LeftMarginX {
		t.Errorf("snapped X = %d, want %d", snapped.X, rg.LeftMarginX)
	}
	if snapped.Y != rg.ContentTopY {
		t.Errorf("snapped Y = %d, want %d", snapped.Y, rg.ContentTopY)
	}
	if snapped.Y+snapped.CY != rg.ContentBottomY {
		t.Errorf("snapped bottom = %d, want %d", snapped.Y+snapped.CY, rg.ContentBottomY)
	}
	if snapped.X+snapped.CX != rg.RightEdgeX {
		t.Errorf("snapped right = %d, want %d", snapped.X+snapped.CX, rg.RightEdgeX)
	}

	// Should have recorded 4 adjustments.
	if len(adjustments) != 4 {
		t.Errorf("got %d adjustments, want 4", len(adjustments))
	}
}

func TestSnapBoundsToGrid_AlreadyAligned(t *testing.T) {
	rg := &resolvedGrid{
		TitleBaselineY: 600000,
		ContentTopY:    800000,
		ContentBottomY: 6200000,
		LeftMarginX:    500000,
		RightEdgeX:     11500000,
		SlideWidth:     shapegrid.DefaultSlideWidthEMU,
		SlideHeight:    shapegrid.DefaultSlideHeightEMU,
	}

	// Bounds that are already aligned.
	bounds := pptx.RectEmu{
		X:  500000,
		Y:  800000,
		CX: 11000000,
		CY: 5400000,
	}

	_, adjustments := snapBoundsToGrid(bounds, rg)

	if len(adjustments) != 0 {
		t.Errorf("expected no adjustments for aligned bounds, got %d", len(adjustments))
	}
}

func TestGridToContentZone(t *testing.T) {
	rg := &resolvedGrid{
		TitleBaselineY: 600000,
		ContentTopY:    800000,
		ContentBottomY: 6200000,
		LeftMarginX:    500000,
		RightEdgeX:     11500000,
		SlideWidth:     shapegrid.DefaultSlideWidthEMU,
		SlideHeight:    shapegrid.DefaultSlideHeightEMU,
	}

	zone := gridToContentZone(rg)

	if zone.TitleBottom != rg.TitleBaselineY {
		t.Errorf("TitleBottom = %d, want %d", zone.TitleBottom, rg.TitleBaselineY)
	}
	if zone.FooterTop != rg.ContentBottomY {
		t.Errorf("FooterTop = %d, want %d", zone.FooterTop, rg.ContentBottomY)
	}
	if zone.LeftMargin != rg.LeftMarginX {
		t.Errorf("LeftMargin = %d, want %d", zone.LeftMargin, rg.LeftMarginX)
	}
	if zone.RightEdge != rg.RightEdgeX {
		t.Errorf("RightEdge = %d, want %d", zone.RightEdge, rg.RightEdgeX)
	}
}

func TestDetectGridViolations(t *testing.T) {
	sw := shapegrid.DefaultSlideWidthEMU
	sh := shapegrid.DefaultSlideHeightEMU

	rg := &resolvedGrid{
		TitleBaselineY: 600000,
		ContentTopY:    800000,
		ContentBottomY: 6200000,
		LeftMarginX:    500000,
		RightEdgeX:     11500000,
		SlideWidth:     sw,
		SlideHeight:    sh,
	}

	layouts := []types.LayoutMetadata{
		{
			ID: "layout1",
			Placeholders: []types.PlaceholderInfo{
				{
					ID:   "title",
					Type: types.PlaceholderTitle,
					Bounds: types.BoundingBox{
						X: 500000, Y: 100000, Width: 10000000,
						// Height makes bottom = 100000 + 600000 = 700000, which deviates from 600000 by 100000 > threshold
						Height: 600000,
					},
				},
				{
					ID:   "body",
					Type: types.PlaceholderBody,
					Bounds: types.BoundingBox{
						X: 500000, Y: 800000, Width: 10000000, Height: 5000000,
					},
				},
			},
		},
	}

	slides := []SlideInput{
		{LayoutID: "layout1"},
	}

	findings := detectGridViolations(rg, layouts, slides)

	// Should flag the title bottom deviation (700000 vs 600000 = 100000 > 45720).
	var foundTitle bool
	for _, f := range findings {
		if f.Code == "grid_violation" {
			foundTitle = true
		}
	}
	if !foundTitle {
		t.Error("expected grid_violation finding for title bottom deviation")
	}
}

func TestDetectGridViolations_WithinThreshold(t *testing.T) {
	sw := shapegrid.DefaultSlideWidthEMU
	sh := shapegrid.DefaultSlideHeightEMU

	rg := &resolvedGrid{
		TitleBaselineY: 600000,
		ContentTopY:    800000,
		ContentBottomY: 6200000,
		LeftMarginX:    500000,
		RightEdgeX:     11500000,
		SlideWidth:     sw,
		SlideHeight:    sh,
	}

	// Title bottom = 100000 + 510000 = 610000, deviation = 10000 < 45720 threshold.
	layouts := []types.LayoutMetadata{
		{
			ID: "layout1",
			Placeholders: []types.PlaceholderInfo{
				{
					ID:   "title",
					Type: types.PlaceholderTitle,
					Bounds: types.BoundingBox{
						X: 500000, Y: 100000, Width: 10000000, Height: 510000,
					},
				},
			},
		},
	}

	slides := []SlideInput{
		{LayoutID: "layout1"},
	}

	findings := detectGridViolations(rg, layouts, slides)

	for _, f := range findings {
		if f.Code == "grid_violation" {
			t.Error("did not expect grid_violation finding within threshold")
		}
	}
}

func TestMedianInt64(t *testing.T) {
	tests := []struct {
		name string
		vals []int64
		want int64
	}{
		{"empty", nil, 0},
		{"single", []int64{42}, 42},
		{"odd count", []int64{3, 1, 2}, 2},
		{"even count", []int64{4, 2, 1, 3}, 3}, // picks upper median
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := medianInt64(tt.vals)
			if got != tt.want {
				t.Errorf("medianInt64(%v) = %d, want %d", tt.vals, got, tt.want)
			}
		})
	}
}

func TestEmuToPct(t *testing.T) {
	sh := int64(6858000)
	got := emuToPct(685800, sh) // 10%
	if got != 10.0 {
		t.Errorf("emuToPct(685800, %d) = %.2f, want 10.00", sh, got)
	}
}

// TestGridViolationIsFitFinding ensures grid_violation uses the standard
// FitFinding structure that downstream can consume.
func TestGridViolationIsFitFinding(t *testing.T) {
	rg := &resolvedGrid{
		TitleBaselineY: 600000,
		ContentTopY:    800000,
		ContentBottomY: 6200000,
		LeftMarginX:    500000,
		RightEdgeX:     11500000,
		SlideWidth:     shapegrid.DefaultSlideWidthEMU,
		SlideHeight:    shapegrid.DefaultSlideHeightEMU,
	}

	layouts := []types.LayoutMetadata{
		{
			ID: "layout1",
			Placeholders: []types.PlaceholderInfo{
				{
					ID:   "title",
					Type: types.PlaceholderTitle,
					Bounds: types.BoundingBox{
						X: 500000, Y: 100000, Width: 10000000, Height: 600000,
					},
				},
			},
		},
	}

	slides := []SlideInput{{LayoutID: "layout1"}}
	findings := detectGridViolations(rg, layouts, slides)

	if len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}

	f := findings[0]
	if f.Code != "grid_violation" {
		t.Errorf("Code = %q, want %q", f.Code, "grid_violation")
	}
	if f.Action != "review" {
		t.Errorf("Action = %q, want %q", f.Action, "review")
	}
	if f.Fix == nil {
		t.Fatal("expected Fix to be set")
	}
	if f.Fix.Kind != "reposition_shape" {
		t.Errorf("Fix.Kind = %q, want %q", f.Fix.Kind, "reposition_shape")
	}

	// Verify it converts cleanly to a Diagnostic.
	_ = patterns.FitFinding(f)
}
