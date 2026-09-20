package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// Preview must make the same automatic canvas choice and place the same cells
// as generation for every grid-bearing slide surface (go-slide-creator-iy892).
func TestPreviewAutoCompositionMatchesGeneration(t *testing.T) {
	tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tctx.reader.Close() }()
	wantLayout, ok := layout.ResolveCanonicalLayoutID("blank-title", tctx.layouts)
	if !ok {
		t.Fatal("template has no blank-title layout")
	}

	cases := []struct {
		name  string
		slide SlideInput
	}{
		{"pattern", SlideInput{Pattern: &PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$4M | ARR","98% | NRR","1K | Users"]`)}}},
		{"pattern_explicit_bounds", SlideInput{Pattern: &PatternInput{Name: "kpi-3up", Bounds: &GridBoundsInput{X: 10, Y: 30, Width: 40, Height: 45}, Values: json.RawMessage(`["$4M | ARR","98% | NRR","1K | Users"]`)}}},
		{"pattern_height_cap", SlideInput{Pattern: &PatternInput{Name: "process-flow", MaxHeightPct: 30, Values: json.RawMessage(`{"steps":[{"label":"Plan"},{"label":"Build"},{"label":"Launch"}]}`)}}},
		{"compose", SlideInput{Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
			{SizePct: 50, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Uptime"}`)}},
			{SizePct: 50, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"3x","label":"Growth"}`)}},
		}}}},
		{"compose_capped_row", SlideInput{Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
			{SizePct: 50, Pattern: PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$4M | ARR","98% | NRR","1K | Users"]`)}},
			{SizePct: 50, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"3x","label":"Growth"}`)}},
		}}}},
		{"compose_multirow", SlideInput{Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
			{SizePct: 50, Pattern: PatternInput{Name: "pull-quote", Values: json.RawMessage(`{"quote":"Teams move faster with clear ownership.","attribution":"Ada"}`)}},
			{SizePct: 50, Pattern: PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"3x","label":"Growth"}`)}},
		}}}},
		{"shape_grid", SlideInput{ShapeGrid: &ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []GridRowInput{{Cells: []*GridCellInput{{Shape: &ShapeSpecInput{Geometry: "rect"}}}}}}}},
	}

	for _, tc := range cases {
		for _, variant := range []struct {
			name string
			grid *GridConfig
		}{
			{name: "template"},
			{name: "rhythm", grid: &GridConfig{TitleBaselinePct: 18, ContentTopPct: 22, ContentBottomPct: 75, LeftMarginPct: 8, RightMarginPct: 8}},
		} {
			t.Run(tc.name+"/"+variant.name, func(t *testing.T) {
				input := &PresentationInput{Grid: variant.grid, Slides: []SlideInput{tc.slide}}
				var rhythm *resolvedGrid
				if variant.grid != nil {
					rhythm = resolveGrid(variant.grid, tctx.layouts, tctx.slideWidth, tctx.slideHeight)
				}
				preview := resolvePreviewSlides(input, tctx)
				if len(preview.Errors) > 0 {
					t.Fatalf("preview errors: %v", preview.Errors)
				}
				if len(preview.ResolvedSlides) != 1 {
					t.Fatalf("preview returned %d slides", len(preview.ResolvedSlides))
				}
				rs := preview.ResolvedSlides[0]
				diagCtx := &GridDiagramContext{ThemeColors: tctx.theme.Colors, FontFamily: tctx.theme.BodyFont}
				specs, _, _, err := convertPresentationSlides([]SlideInput{tc.slide}, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.metadata, rhythm, "", diagCtx, false)
				if err != nil {
					t.Fatal(err)
				}
				if rs.LayoutID != wantLayout || specs[0].LayoutID != wantLayout {
					t.Fatalf("preview layout %q, generate layout %q, want %q", rs.LayoutID, specs[0].LayoutID, wantLayout)
				}
				if rs.ShapeGridResolution == nil {
					t.Fatal("preview returned no grid resolution")
				}
				geom, _ := patternExpansionGeometry(input.Slides[0], tctx.layouts, tctx.slideWidth, tctx.slideHeight, rhythm)
				alloc := pptx.NewShapeIDAllocator(nil)
				alloc.SetMinID(200)
				resolved, err := resolveShapeGrid(input.Slides[0].ShapeGrid, alloc, geom.OverrideBounds, geom.Zone, tctx.slideWidth, tctx.slideHeight, nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(resolved.Shapes) != len(specs[0].RawShapeXML) || len(resolved.Cells) != len(rs.ShapeGridResolution.Cells) {
					t.Fatalf("shape/cell counts differ: preview %d/%d, generate %d", len(resolved.Shapes), len(rs.ShapeGridResolution.Cells), len(specs[0].RawShapeXML))
				}
				for i, cell := range resolved.Cells {
					pc := rs.ShapeGridResolution.Cells[i]
					if pc.X != cell.CellBounds.X || pc.Y != cell.CellBounds.Y || pc.W != cell.CellBounds.CX || pc.H != cell.CellBounds.CY {
						t.Errorf("cell %d preview rect %+v differs from generation bounds %+v", i, pc, cell.CellBounds)
					}
				}
				for i := range resolved.Shapes {
					if !bytes.Equal(resolved.Shapes[i], specs[0].RawShapeXML[i]) {
						t.Errorf("rendered shape %d differs between preview expansion and generation", i)
					}
				}
			})
		}
	}
}

func TestPreviewThemeMatchesGenerationForFontSensitivePatterns(t *testing.T) {
	tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", "modern-yellow.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tctx.reader.Close() }()
	if tctx.theme == nil || tctx.theme.BodyFont == "" || tctx.theme.BodyFont == "Arial" {
		t.Fatalf("expected a non-default template body font, got %+v", tctx.theme)
	}
	diagCtx := &GridDiagramContext{ThemeColors: tctx.theme.Colors, FontFamily: tctx.theme.BodyFont}
	if got := patternThemeFromDiag(diagCtx).BodyFont; got != tctx.theme.BodyFont {
		t.Fatalf("generation theme body font = %q, want %q", got, tctx.theme.BodyFont)
	}
	kpi := PatternInput{Name: "kpi-3up", Values: json.RawMessage(`["$123.45M | Enterprise bookings over three years","$987.65M | International recurring revenue","$555.55M | Weighted opportunity pipeline"]`)}
	hero := PatternInput{Name: "stat-hero", Values: json.RawMessage(`{"value":"99%","label":"Annual uptime"}`)}
	for _, tc := range []struct {
		name  string
		slide SlideInput
	}{
		{"pattern", SlideInput{Pattern: &kpi}},
		{"compose", SlideInput{Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{SizePct: 50, Pattern: kpi}, {SizePct: 50, Pattern: hero}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := &PresentationInput{Slides: []SlideInput{tc.slide}}
			preview := resolvePreviewSlides(input, tctx)
			if len(preview.Errors) > 0 || input.Slides[0].ShapeGrid == nil {
				t.Fatalf("preview errors: %v", preview.Errors)
			}
			specs, _, _, err := convertPresentationSlides([]SlideInput{tc.slide}, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.metadata, nil, "", diagCtx, false)
			if err != nil {
				t.Fatal(err)
			}
			geom, _ := patternExpansionGeometry(input.Slides[0], tctx.layouts, tctx.slideWidth, tctx.slideHeight, nil)
			alloc := pptx.NewShapeIDAllocator(nil)
			alloc.SetMinID(200)
			resolved, err := resolveShapeGrid(input.Slides[0].ShapeGrid, alloc, geom.OverrideBounds, geom.Zone, tctx.slideWidth, tctx.slideHeight, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(resolved.Shapes) != len(specs[0].RawShapeXML) {
				t.Fatalf("preview shapes %d, generation shapes %d", len(resolved.Shapes), len(specs[0].RawShapeXML))
			}
			for i := range resolved.Shapes {
				if !bytes.Equal(resolved.Shapes[i], specs[0].RawShapeXML[i]) {
					t.Errorf("shape %d differs between preview and generation with body font %q", i, tctx.theme.BodyFont)
				}
			}
		})
	}
}
