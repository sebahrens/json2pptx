package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// A template grid's gutter_pt reaches pattern-internal gaps on the
// generation path (the ContentZone the pattern expands against carries it),
// so a pattern's cards sit one template gutter apart in proportion — the
// kpi-Nup card gap is authored as 12pt against the 8pt default, so a 16pt
// gutter opens it to 24pt (go-slide-creator-5ms8c). The cards are the tiles
// style; an open strip's cells are a divider's width apart.
func TestTemplateGridGutterReachesPatternGaps(t *testing.T) {
	plain, w, h := parseTemplateLayouts(t, filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	gridded, _, _ := parseTemplateLayouts(t, writeTemplateWithGrid(t, "midnight-blue", map[string]any{"gutter_pt": 16}))

	title := "Results so far"
	var slide SlideInput
	if err := json.Unmarshal([]byte(`{"layout_id":"content","pattern":{"name":"kpi-3up","overrides":{"style":"tiles"},"values":[
		{"big":"42%","small":"Faster resolution"},{"big":"$3.1M","small":"Annual savings"},{"big":"4.6","small":"CSAT score"}]}}`), &slide); err != nil {
		t.Fatal(err)
	}
	slide.Content = []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}

	cardGap := func(layouts []types.LayoutMetadata) (float64, float64) {
		geom, bounds := patternExpansionGeometry(slide, layouts, w, h, nil)
		ctx := patterns.ExpandContext{
			ContentZone: geom.Zone, SlideWidth: w, SlideHeight: h,
			LayoutBounds: patterns.LayoutBounds{X: bounds.X, Y: bounds.Y, Width: bounds.CX, Height: bounds.CY},
		}
		grid, _, err := expandPattern(slide.Pattern, ctx, patterns.Default())
		if err != nil {
			t.Fatal(err)
		}
		authored := grid.Gap
		res, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), geom.OverrideBounds, geom.Zone, w, h, nil)
		if err != nil {
			t.Fatal(err)
		}
		c := res.Cells
		if len(c) < 2 || c[1].Bounds.Y != c[0].Bounds.Y {
			t.Fatalf("kpi-3up resolved to %d cells; want a row of cards", len(c))
		}
		return authored, float64(c[1].Bounds.X-c[0].Bounds.X-c[0].Bounds.CX) / 12700
	}
	plainAuthored, plainGap := cardGap(plain)
	griddedAuthored, griddedGap := cardGap(gridded)
	if plainAuthored != 12 || griddedAuthored != 24 {
		t.Errorf("kpi-3up authored gap = %g (plain) / %g (16pt gutter), want 12 / 24", plainAuthored, griddedAuthored)
	}
	if plainGap < 11.9 || plainGap > 12.1 || griddedGap < 23.9 || griddedGap > 24.1 {
		t.Errorf("rendered card gap = %.1fpt (plain) / %.1fpt (16pt gutter), want 12 / 24", plainGap, griddedGap)
	}
}

// slidePicOffsets returns the x offset and width (EMU) of every p:pic on
// slide n of a generated deck.
// picFrame is one picture's horizontal frame (EMU).
type picFrame struct{ x, cx int64 }

func slidePicOffsets(t *testing.T, pptxPath string, n int) []picFrame {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	var xml string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide"+strconv.Itoa(n)+".xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			xml = string(data)
		}
	}
	pic := regexp.MustCompile(`(?s)<p:pic>.*?</p:pic>`)
	off := regexp.MustCompile(`<a:off x="(-?\d+)" y="-?\d+"/>\s*<a:ext cx="(\d+)"`)
	var out []picFrame
	for _, m := range pic.FindAllString(xml, -1) {
		if g := off.FindStringSubmatch(m); g != nil {
			x, _ := strconv.ParseInt(g[1], 10, 64)
			cx, _ := strconv.ParseInt(g[2], 10, 64)
			out = append(out, picFrame{x, cx})
		}
	}
	return out
}

// A placeholder chart's frame follows the template grid margin: with
// margin_pct declared the chart sits inside the margin frame, while the same
// deck on the undeclared template keeps the placeholder's own frame
// (go-slide-creator-5ms8c).
func TestTemplateGridMarginFramesPlaceholderCharts(t *testing.T) {
	const marginPct = 12.0
	griddedPath := writeTemplateWithGrid(t, "midnight-blue", map[string]any{"margin_pct": marginPct})
	slide := map[string]any{
		"slide_type": "chart",
		"content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Revenue rose every quarter"},
			map[string]any{"placeholder_id": "body", "type": "chart", "chart_value": map[string]any{
				"type": "bar", "title": "Revenue by quarter ($M)",
				"data": map[string]any{"Q1": 12.0, "Q2": 14.5, "Q3": 15.2, "Q4": 18.0},
			}},
		},
	}
	plainDeck := generateDeckForChromeTest(t, filepath.Join("..", "..", "templates"), "midnight-blue", slide)
	griddedDeck := generateDeckForChromeTest(t, filepath.Dir(griddedPath), "midnight-blue-grid", slide)

	margin := int64(marginPct / 100 * chromeTestSlideW)
	plainPics := slidePicOffsets(t, plainDeck, 1)
	griddedPics := slidePicOffsets(t, griddedDeck, 1)
	if len(plainPics) == 0 || len(griddedPics) == 0 {
		t.Fatalf("no chart picture on the slide (plain %d, gridded %d)", len(plainPics), len(griddedPics))
	}
	if plainPics[0].x >= margin {
		t.Fatalf("plain chart already starts inside the %g%% margin (x=%d); the test needs a wider margin", marginPct, plainPics[0].x)
	}
	for _, p := range griddedPics {
		if p.x < margin || p.x+p.cx > chromeTestSlideW-margin {
			t.Errorf("chart frame x=%d..%d escapes the %g%% template margin frame %d..%d", p.x, p.x+p.cx, marginPct, margin, chromeTestSlideW-margin)
		}
	}
}

// Compose segments are separated by the template gutter when the envelope
// sets no gap of its own (go-slide-creator-5ms8c).
func TestComposeGapDefaultsToTemplateGutter(t *testing.T) {
	c := &ComposeInput{}
	if got := composeGapPt(c, patterns.ExpandContext{}); got != types.DefaultGridGutterPt {
		t.Errorf("compose gap without a template grid = %g, want %g", got, types.DefaultGridGutterPt)
	}
	ctx := patterns.ExpandContext{Metadata: &types.TemplateMetadata{Grid: &types.TemplateGrid{GutterPt: 14}}}
	if got := composeGapPt(c, ctx); got != 14 {
		t.Errorf("compose gap under a 14pt gutter = %g, want 14", got)
	}
	c.Gap = 5
	if got := composeGapPt(c, ctx); got != 5 {
		t.Errorf("authored compose gap = %g, want 5", got)
	}
}
