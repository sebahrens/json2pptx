package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Shared helpers of the circular family's tests (cycle-ring, cycle-nodes,
// cycle-intake, cycle-figure-eight, radial-hub, concentric-rings): one way to
// load a template with its theme, to resolve a slide's grid the way validate
// predicts it and generation writes it, and to read the drawing out of the
// resolved cells — the round layers, the square they share, and the text
// cells around them.

// circularTemplate is a template's geometry and theme, parsed once per test
// binary.
type circularTemplate struct {
	name          string
	layouts       []types.LayoutMetadata
	width, height int64
	theme         types.ThemeInfo
}

var circularTemplates sync.Map // name -> *circularTemplate

func loadCircularTemplate(t *testing.T, name string) *circularTemplate {
	t.Helper()
	if cached, ok := circularTemplates.Load(name); ok {
		return cached.(*circularTemplate)
	}
	layouts, _, width, height, _, theme, _ := analyzeTemplateLayouts(filepath.Join(testutil.TemplatesDir(), name+".pptx"))
	if layouts == nil {
		t.Fatalf("cannot analyze template %q", name)
	}
	tpl := &circularTemplate{name: name, layouts: layouts, width: width, height: height, theme: theme}
	circularTemplates.Store(name, tpl)
	return tpl
}

// circularDeck decodes a deck written as JSON, the form an agent sends.
func circularDeck(t *testing.T, deckJSON string) *PresentationInput {
	t.Helper()
	var input PresentationInput
	if err := json.Unmarshal([]byte(deckJSON), &input); err != nil {
		t.Fatalf("deck JSON: %v\n%s", err, deckJSON)
	}
	return &input
}

// circularResolvedSlide resolves the deck's slide slideIdx into its grid
// cells through the chain validate runs before its preflights (compose and
// pattern expansion, nested cell patterns, then resolveShapeGrid in the
// slide's content rectangle). Generation writes the same shapes
// (TestValidateExpandsPatternsLikeGeneration), so the cells are what the slide draws. nil means
// the slide has no grid or generation refuses it.
func circularResolvedSlide(t *testing.T, input *PresentationInput, tpl *circularTemplate, slideIdx int) *ShapeGridResult {
	t.Helper()
	// A private copy: applyDefaults and the expansions rewrite the deck.
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	deck := circularDeck(t, string(raw))
	applyDefaults(deck)
	resolveCanonicalLayoutIDs(deck.Slides, tpl.layouts)
	fitInput, fitLayouts, _ := withDerivedFitLayouts(deck, tpl.layouts, tpl.width, tpl.height)
	fitInput, _ = expandComposeForPreflightWithTheme(fitInput, tpl.width, tpl.height, &tpl.theme, fitLayouts...)
	fitInput, _ = expandPatternsForFit(fitInput, tpl.width, tpl.height, &tpl.theme, fitLayouts...)
	if slideIdx >= len(fitInput.Slides) || fitInput.Slides[slideIdx].ShapeGrid == nil {
		return nil
	}
	slide := fitInput.Slides[slideIdx]
	rhythm := resolvedValidRhythmGrid(fitInput, fitLayouts, tpl.width, tpl.height)
	geom, contentBounds := patternExpansionGeometry(slide, fitLayouts, tpl.width, tpl.height, rhythm)
	result, _ := predictedSlideGridShapes(slide.ShapeGrid, slideIdx, nestedExpansionGeometry{
		geom: geom, contentBounds: contentBounds, slideWidth: tpl.width, slideHeight: tpl.height,
		theme: &tpl.theme, strategy: patterns.AccentStrategy(deck.AccentStrategy), slideIdx: slideIdx,
		sectionIdx: slideSectionIndices(fitInput.Slides, fitLayouts)[slideIdx],
	})
	return result
}

// circularRoundGeometries are the presets whose frame must be a square for
// the shape to read as part of a circle.
var circularRoundGeometries = map[string]bool{
	"ellipse": true, "blockArc": true, "circularArrow": true, "donut": true, "pie": true, "arc": true,
}

// circularText is a text-bearing lattice cell (not a layer): a label, a
// legend row, a numeral, or the neighbouring segment's copy.
type circularText struct {
	text     string
	geometry string
	bounds   pptx.RectEmu
}

// interlocks reports a pair of pointed shapes that nest into each other by
// design: the chevrons of an intake lane overlap by their notch.
func (a circularText) interlocks(b circularText) bool {
	pointed := map[string]bool{"chevron": true, "homePlate": true}
	return pointed[a.geometry] && pointed[b.geometry]
}

// circularDrawing is what a resolved slide grid draws of the family.
type circularDrawing struct {
	// round are the round layers (segments, nodes, badges, hub, satellites,
	// rings), in drawing order.
	round []shapegrid.ResolvedCell
	// squares are the frames the round layers of one host cell share, one per
	// host cell in left-to-right order: the ring's square, the hub's ring,
	// each lobe of a figure eight.
	squares []pptx.RectEmu
	// texts are the slide's text cells outside the layers.
	texts []circularText
}

// circularCellText is the visible text of a resolved shape cell.
func circularCellText(c shapegrid.ResolvedCell) string {
	if c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(c.ShapeSpec.Text, &v); err != nil {
		return ""
	}
	var parts []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			if s := strings.TrimSpace(x); s != "" {
				parts = append(parts, s)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			if c, ok := x["content"]; ok {
				walk(c)
			}
			if p, ok := x["paragraphs"]; ok {
				walk(p)
			}
			if r, ok := x["runs"]; ok {
				walk(r)
			}
			if s, ok := x["text"].(string); ok {
				walk(s)
			}
		}
	}
	walk(v)
	return strings.Join(parts, " / ")
}

func circularDrawingOf(result *ShapeGridResult) circularDrawing {
	var d circularDrawing
	if result == nil {
		return d
	}
	hosts := map[pptx.RectEmu]pptx.RectEmu{}
	for _, c := range result.Cells {
		if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil {
			continue
		}
		if c.Layer {
			if !circularRoundGeometries[c.ShapeSpec.Geometry] {
				continue
			}
			d.round = append(d.round, c)
			hosts[c.CellBounds] = circularUnion(hosts[c.CellBounds], c.Bounds)
			continue
		}
		if text := circularCellText(c); text != "" {
			d.texts = append(d.texts, circularText{text: text, geometry: c.ShapeSpec.Geometry, bounds: c.Bounds})
		}
	}
	for _, sq := range hosts {
		d.squares = append(d.squares, sq)
	}
	sort.Slice(d.squares, func(i, j int) bool { return d.squares[i].X < d.squares[j].X })
	return d
}

func circularUnion(a, b pptx.RectEmu) pptx.RectEmu {
	if a.CX == 0 && a.CY == 0 {
		return b
	}
	x0, y0 := minI64(a.X, b.X), minI64(a.Y, b.Y)
	x1, y1 := maxI64(a.X+a.CX, b.X+b.CX), maxI64(a.Y+a.CY, b.Y+b.CY)
	return pptx.RectEmu{X: x0, Y: y0, CX: x1 - x0, CY: y1 - y0}
}

// circularOverlapPt is how far two rectangles overlap on their shallower
// axis, in points; 0 when they are apart or only touch.
func circularOverlapPt(a, b pptx.RectEmu) float64 {
	w := minI64(a.X+a.CX, b.X+b.CX) - maxI64(a.X, b.X)
	h := minI64(a.Y+a.CY, b.Y+b.CY) - maxI64(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return float64(minI64(w, h)) / 12700
}

func circularRectPt(r pptx.RectEmu) string {
	return fmt.Sprintf("%.0f,%.0f %.0fx%.0fpt", float64(r.X)/12700, float64(r.Y)/12700, float64(r.CX)/12700, float64(r.CY)/12700)
}

var circularRunSizeRE = regexp.MustCompile(`<a:rPr\b[^>]*\bsz="(\d+)"`)

// circularWrittenTextBelow lists the grid shapes of a written slide whose
// text is set under floorPt once the stored autofit shrink is applied
// ("9.6pt (12.0pt x 80%) \"text\""). Placeholders (the title, the footer) are
// the template's and are left out, and so is any shape carrying one of skip's
// texts (a neighbouring pattern's captions, set on their own role floor).
func circularWrittenTextBelow(slideXML string, floorPt float64, skip ...string) []string {
	var out []string
shapes:
	for _, shape := range matrixRenderedShapeRE.FindAllString(slideXML, -1) {
		if !strings.Contains(shape, "<a:t>") || strings.Contains(shape, "<p:ph") {
			continue
		}
		for _, s := range skip {
			if strings.Contains(shape, "<a:t>"+s+"</a:t>") {
				continue shapes
			}
		}
		scale := 100000
		if m := matrixFontScaleRE.FindStringSubmatch(shape); len(m) == 2 {
			scale, _ = strconv.Atoi(m[1])
		}
		smallest := 0
		for _, run := range circularRunSizeRE.FindAllStringSubmatch(shape, -1) {
			if size, _ := strconv.Atoi(run[1]); smallest == 0 || size < smallest {
				smallest = size
			}
		}
		if smallest == 0 {
			continue
		}
		if effective := float64(smallest) * float64(scale) / 100000 / 100; effective < floorPt-0.005 {
			var text []string
			for _, tm := range shapeRunTextRe.FindAllStringSubmatch(shape, -1) {
				text = append(text, tm[1])
			}
			out = append(out, fmt.Sprintf("%.1fpt (%.1fpt x %d%%) %q", effective, float64(smallest)/100, scale/1000, strings.Join(text, " ")))
		}
	}
	return out
}
