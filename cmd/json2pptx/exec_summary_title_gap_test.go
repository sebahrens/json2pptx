package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestExecutiveSummaryReclaimsTallTitleBandOnPStyle(t *testing.T) {
	path := filepath.Join("..", "..", "templates", "p-style.pptx")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("local p-style template unavailable")
	}
	ctx, err := loadPreviewTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ctx.reader.Close() }()
	title := "Every pilot case needs an accountable owner"
	values := json.RawMessage(`{"points":[{"lead":"Assign the owner","support":"Record one accountable person for every case."},{"lead":"Separate the review","support":"Have a second person judge the result."},{"lead":"Keep the record","support":"Store the reason behind every decision."},{"lead":"Track completion","support":"Count each case once after closure."},{"lead":"Review exceptions","support":"Escalate any unassigned or unresolved case."}],"bottom_line":"Assign owners and reviewers before the pilot starts."}`)
	slide := SlideInput{LayoutID: "blank-title", Pattern: &PatternInput{Name: "exec-summary", Values: values}, Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}}
	geom := resolveGridGeometry(slide, ctx.layouts, ctx.slideWidth, ctx.slideHeight)
	area := resolveGridBounds(&ShapeGridInput{}, geom.OverrideBounds, geom.Zone, ctx.slideWidth, ctx.slideHeight)
	grid, _ := expandSlidePatternGridWithWarningsAtBounds(&slide, 0, ctx.slideWidth, ctx.slideHeight, nil, patterns.LayoutBounds{X: area.X, Y: area.Y, Width: area.CX, Height: area.CY}, geom.Zone)
	if grid == nil {
		t.Fatal("pattern expansion failed")
	}
	out, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), geom.OverrideBounds, geom.Zone, ctx.slideWidth, ctx.slideHeight, nil)
	if err != nil {
		t.Fatal(err)
	}
	const beforeY = int64(1946276)
	found := false
	for _, cell := range out.Cells {
		if cell.ShapeSpec == nil || !strings.Contains(string(cell.ShapeSpec.Text), "Assign the owner") {
			continue
		}
		found = true
		if cell.Bounds.Y >= beforeY-30*12700 {
			t.Fatalf("first point stayed in old title dead band: y%d, before%d", cell.Bounds.Y, beforeY)
		}
		if cell.Bounds.Y < geom.Zone.TitleBottom {
			t.Fatal("first point intersects measured title")
		}
		t.Logf("first point y %d -> %d (%.1fpt earlier)", beforeY, cell.Bounds.Y, float64(beforeY-cell.Bounds.Y)/12700)
	}
	if !found {
		t.Fatal("first point missing")
	}
}

func TestExecutiveSummaryMeasuredTitleClearanceAndGenerationParity(t *testing.T) {
	for _, name := range []string{"midnight-blue", "p-style"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "templates", name+".pptx")
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skip("local template unavailable")
			}
			ctx, err := loadPreviewTemplate(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ctx.reader.Close() }()
			for _, lines := range []int{1, 2, 3} {
				title := strings.TrimSuffix(strings.Repeat("Accountable owners close every case\n", lines), "\n")
				values := json.RawMessage(`{"points":[{"lead":"Assign the owner","support":"Record one accountable person."},{"lead":"Check","support":"Review the result."},{"lead":"Keep","support":"Store the reason."},{"lead":"Track","support":"Count each case."},{"lead":"Review","support":"Escalate unresolved cases."}],"bottom_line":"Assign owners before the pilot starts."}`)
				slide := SlideInput{LayoutID: "blank-title", Pattern: &PatternInput{Name: "exec-summary", Values: values}, Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}}
				expanded, _ := expandPatternsForFit(&PresentationInput{Slides: []SlideInput{slide}}, ctx.slideWidth, ctx.slideHeight, nil, ctx.layouts...)
				actual := expanded.Slides[0]
				geom := resolveGridGeometry(actual, ctx.layouts, ctx.slideWidth, ctx.slideHeight)
				out, err := resolveShapeGrid(actual.ShapeGrid, pptx.NewShapeIDAllocator(nil), geom.OverrideBounds, geom.Zone, ctx.slideWidth, ctx.slideHeight, nil)
				if err != nil {
					t.Fatal(err)
				}
				top, _ := blockExtent(out.Cells)
				if top < geom.Zone.TitleBottom {
					t.Fatalf("%d-line title overlap: first point%d title bottom%d", lines, top, geom.Zone.TitleBottom)
				}
				specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, ctx.layouts, ctx.slideWidth, ctx.slideHeight, nil, nil, "", nil, false)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, raw := range specs[0].RawShapeXML {
					if !strings.Contains(string(raw), "Assign the owner") {
						continue
					}
					found = true
					match := shapeOffYRegexp.FindSubmatch(raw)
					if len(match) != 2 {
						t.Fatal("missing generated first-point geometry")
					}
					y, err := strconv.ParseInt(string(match[1]), 10, 64)
					if err != nil {
						t.Fatal(err)
					}
					if y != top {
						t.Fatalf("%d-line title preflight%d generation%d", lines, top, y)
					}
				}
				if !found {
					t.Fatal("generated first point missing")
				}
			}
		})
	}
}

// The summary deliberately uses the reclaimed title edge as its body line.
// Recording that line must change balance metadata, never the written shapes.
func TestExecutiveSummaryEffectiveBodyAnchorPreservesOOXML(t *testing.T) {
	for _, name := range []string{"midnight-blue", "p-style"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "templates", name+".pptx")
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skip("local template unavailable")
			}
			ctx, err := loadPreviewTemplate(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ctx.reader.Close() }()
			for _, count := range []int{3, 4, 5} {
				for _, lines := range []int{1, 2, 3} {
					t.Run(fmt.Sprintf("%dpoints/%dlines", count, lines), func(t *testing.T) {
						title := strings.TrimSuffix(strings.Repeat("Accountable owners close every case\n", lines), "\n")
						points := make([]map[string]string, count)
						for i := range points {
							points[i] = map[string]string{"lead": fmt.Sprintf("Assign owner %d", i+1), "support": "Record one accountable person for every case."}
						}
						values, _ := json.Marshal(map[string]any{"points": points, "bottom_line": "Assign owners and reviewers before the pilot starts."})
						slide := SlideInput{LayoutID: "blank-title", Pattern: &PatternInput{Name: "exec-summary", Values: values}, Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}}
						expanded, _ := expandPatternsForFit(&PresentationInput{Slides: []SlideInput{slide}}, ctx.slideWidth, ctx.slideHeight, nil, ctx.layouts...)
						actual := expanded.Slides[0]
						geom := resolveGridGeometry(actual, ctx.layouts, ctx.slideWidth, ctx.slideHeight)
						safe := contentRelativeBoundsBase(geom.OverrideBounds, geom.Zone, ctx.slideWidth, ctx.slideHeight)
						if geom.Zone == nil || geom.Zone.BodyTop != safe.Y {
							t.Fatalf("effective anchor does not match content top: %+v vs %+v", geom.Zone, safe)
						}
						beforeZone := *geom.Zone
						beforeZone.BodyTop = 0
						before, err := resolveShapeGrid(actual.ShapeGrid, pptx.NewShapeIDAllocator(nil), geom.OverrideBounds, &beforeZone, ctx.slideWidth, ctx.slideHeight, nil)
						if err != nil {
							t.Fatal(err)
						}
						after, err := resolveShapeGrid(actual.ShapeGrid, pptx.NewShapeIDAllocator(nil), geom.OverrideBounds, geom.Zone, ctx.slideWidth, ctx.slideHeight, nil)
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(before.Shapes, after.Shapes) {
							t.Fatal("anchor metadata changed generated OOXML")
						}
						for _, emptyPct := range []int64{35, 45} {
							ink := []pptx.RectEmu{{X: safe.X, Y: safe.Y, CX: safe.CX, CY: safe.CY * (100 - emptyPct) / 100}}
							finding := checkVerticalImbalance(ink, safe, &actual, 0, "exec-summary", geom.Zone.BodyTop, true)
							if (finding != nil) != (emptyPct > 40) {
								t.Fatalf("%d%% empty: finding=%v", emptyPct, finding)
							}
						}
					})
				}
			}
		})
	}
}
