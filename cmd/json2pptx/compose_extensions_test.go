package main

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

func measuredComposeContext() patterns.ExpandContext {
	return patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000, LayoutBounds: patterns.LayoutBounds{X: 400000, Y: 1200000, Width: 850 * 12700, Height: 350 * 12700}}
}
func composeTextLeaf(body string) SegmentInput {
	return SegmentInput{Text: &deckComposeText{Body: body}}
}

type deckComposeText = deckinput.ComposeTextInput

func TestComposeNativeLeavesHeadingsAndBands(t *testing.T) {
	c := &ComposeInput{Direction: "horizontal", Divider: true, Gaps: []float64{20}, Banner: &patterns.BannerSpec{Text: "Business result"}, Callout: &patterns.PatternCallout{Text: "Choose one accountable owner"}, Segments: []SegmentInput{
		{Heading: "Evidence", Caption: "Synthetic test data", Diagram: &types.DiagramSpec{Type: "bar", Data: map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Cases", "values": []float64{2, 3}}}}}},
		{Heading: "Implications", Text: &deckComposeText{Body: "Owners keep the decision record.", Bullets: []string{"Assign the owner", "Separate the review", "Keep the record", "Track completion"}}},
	}}
	ctx := measuredComposeContext()
	g, _, err := expandCompose(c, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(g)
	for _, text := range []string{"Business result", "Choose one accountable owner", "Implications", "Evidence", "Synthetic test data", "Track completion"} {
		if !strings.Contains(string(raw), text) {
			t.Fatalf("leaf/decor lost: %s", text)
		}
	}
	if c.Segments[1].Text.Body != "Owners keep the decision record." {
		t.Fatal("source mutated")
	}
	geom := pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
	out, err := resolveShapeGrid(g, pptx.NewShapeIDAllocator(nil), &geom, nil, ctx.SlideWidth, ctx.SlideHeight, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Shapes) == 0 {
		t.Fatal("native pane shapes missing")
	}
}
func TestComposeNativeLeafXORAndGapValidation(t *testing.T) {
	leaf := composeTextLeaf("One native paragraph")
	for _, c := range []*ComposeInput{
		{Direction: "horizontal", Gaps: []float64{1, 2}, Segments: []SegmentInput{leaf, leaf}},
		{Direction: "vertical", Gap: math.NaN(), Segments: []SegmentInput{leaf, leaf}},
		{Direction: "vertical", Gaps: []float64{-1}, Segments: []SegmentInput{leaf, leaf}},
		{Direction: "horizontal", Segments: []SegmentInput{{Text: leaf.Text, Table: &jsonschema.TableInput{Headers: []string{"A"}}}, leaf}},
		{Direction: "horizontal", Segments: []SegmentInput{{Text: leaf.Text, Size: "content"}, leaf}},
	} {
		if validateCompose(c) == nil {
			t.Fatalf("invalid compose accepted: %+v", c)
		}
	}
}
func TestComposeContentSizeAndSmartFixedShares(t *testing.T) {
	ctx := measuredComposeContext()
	c := &ComposeInput{Direction: "vertical", Gaps: []float64{0}, Segments: []SegmentInput{{Size: "content", Text: &deckComposeText{Body: "Short KPI commentary"}}, {Diagram: &types.DiagramSpec{Type: "bar"}}}}
	grids, sizes, _, err := composeSegmentGrids(c, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if sizes[0] >= 30 || sizes[1] <= 70 {
		t.Fatalf("content strip claims percentage-sized band: %v", sizes)
	}
	if len(grids) != 2 {
		t.Fatal("pane lost")
	}
	smart := &ComposeInput{Direction: "horizontal", SmartCompose: true, Segments: []SegmentInput{{SizePct: 60, Diagram: &types.DiagramSpec{Type: "bar"}}, composeTextLeaf("Short"), composeTextLeaf(strings.Repeat("Evidence requires careful interpretation. ", 8))}}
	_, shares, _, err := composeSegmentGrids(smart, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if shares[0] != 60 || shares[2] <= shares[1] || math.Abs(shares[1]+shares[2]-40) > 0.001 {
		t.Fatalf("smart implicit remainder/fixed share: %v", shares)
	}
	tooSmall := ctx
	tooSmall.LayoutBounds.Height = 20 * 12700
	if _, _, err := expandCompose(c, tooSmall, patterns.Default()); err == nil {
		t.Fatal("content exhaustion accepted")
	}
}
func TestComposeHorizontalContentLossIsRefused(t *testing.T) {
	raw := &jsonschema.ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{composePaneText("Keep A", 12, false), composePaneText("Keep B", 12, false)}}}}
	if _, _, err := mergeHorizontalIn([]*jsonschema.ShapeGridInput{raw, composeNativeLeaf(composeTextLeaf("C"))}, []float64{50, 50}, 8, 850*12700); err == nil || !strings.Contains(err.Error(), "TRUNCATION") {
		t.Fatalf("lossy row accepted: %v", err)
	}
}

func TestComposeDepthThreeDeepestPaneReadabilityIsInspected(t *testing.T) {
	var leaf jsonschema.ShapeGridInput
	if err := json.Unmarshal([]byte(`{"columns":1,"rows":[{"cells":[{"shape":{"geometry":"rect","text":{"content":"Deepest evidence","size":12,"bold":true}}}]}]}`), &leaf); err != nil {
		t.Fatal(err)
	}
	c := &ComposeInput{Direction: "horizontal", Banner: &patterns.BannerSpec{Text: "Outer band"}, Segments: []SegmentInput{composeTextLeaf("Context"), {Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{composeTextLeaf("Bridge"), {Heading: "Deep heading", Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{Heading: "Evidence", ShapeGrid: &leaf}, composeTextLeaf("Related detail")}}}}}}}}
	if err := validateCompose(c); err != nil {
		t.Fatal(err)
	}
	ctx := measuredComposeContext()
	ctx.LayoutBounds.Height = 700 * 12700
	grid, _, err := expandCompose(c, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	// ShapeGrid keeps the authored sizes here so this fixture exercises the
	// native inspection depth instead of the renderer's12pt clamping policy.
	var keep func(*jsonschema.ShapeGridInput)
	keep = func(g *jsonschema.ShapeGridInput) {
		g.KeepTextSizes = true
		for _, r := range g.Rows {
			for _, cell := range r.Cells {
				if cell != nil && cell.Shape != nil && strings.Contains(string(cell.Shape.Text), "Deepest evidence") {
					cell.Shape.Text, _ = json.Marshal(map[string]any{"content": strings.Repeat("Deepest evidence must remain readable. ", 40), "size": 12, "bold": true})
				}
				if cell != nil && cell.Grid != nil {
					keep(cell.Grid)
				}
			}
		}
	}
	keep(grid)
	input := &PresentationInput{ViewingMode: "read", Slides: []SlideInput{{ShapeGrid: grid}}}
	found := false
	for _, f := range collectReadability(input, nil, ctx.SlideWidth, ctx.SlideHeight, nil, false) {
		if f.Code == patterns.ErrCodeTextBelowReadableMin && strings.Contains(f.Message, "card-title") {
			found = true
		}
	}
	if !found {
		t.Fatal("deepest pane's unreadable text disappeared behind compose wrappers")
	}
}

func TestComposeNativePanesAcrossTemplateGeometry(t *testing.T) {
	for _, name := range testutil.AllTestTemplateNames() {
		t.Run(name, func(t *testing.T) {
			tc, err := loadPreviewTemplate(filepath.Join(testutil.TemplatesDir(), name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tc.reader.Close() }()
			title := "Evidence supports a clear next step"
			c := &ComposeInput{Direction: "horizontal", Gaps: []float64{20}, Divider: true, Segments: []SegmentInput{
				{Heading: "Evidence", Diagram: &types.DiagramSpec{Type: "bar", Data: map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Cases", "values": []float64{2, 3}}}}}},
				{Heading: "Implications", Text: &deckComposeText{Bullets: []string{"Assign the owner", "Separate the review", "Keep the record", "Track completion"}}},
			}}
			slide := SlideInput{LayoutID: "blank-title", Compose: c, Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}}
			geom, area := patternExpansionGeometry(slide, tc.layouts, tc.slideWidth, tc.slideHeight, nil)
			ctx := slideExpandContext(tc.theme, tc.metadata, geom.Zone, patterns.LayoutBounds{X: area.X, Y: area.Y, Width: area.CX, Height: area.CY}, tc.slideWidth, tc.slideHeight, "", 0, 0)
			grid, _, err := expandCompose(c, ctx, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			out, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &area, geom.Zone, tc.slideWidth, tc.slideHeight, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Shapes) == 0 {
				t.Fatal("no native pane shapes")
			}
			input := &PresentationInput{ViewingMode: "read", Slides: []SlideInput{{ShapeGrid: grid, LayoutID: slide.LayoutID, Content: slide.Content}}}
			if got := readabilityCodes(collectReadability(input, tc.layouts, tc.slideWidth, tc.slideHeight, tc.theme, false)); len(got) != 0 {
				t.Fatalf("nativepane unreadable: %+v", got)
			}
		})
	}
}

func TestComposeSeamGapsMatchCandidateGeometry(t *testing.T) {
	ctx := measuredComposeContext()
	c := &ComposeInput{Direction: "horizontal", Gaps: []float64{0, 24}, Segments: []SegmentInput{composeTextLeaf("First"), composeTextLeaf("Second"), composeTextLeaf("Third")}}
	grid, _, err := expandCompose(c, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	rect := pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
	resolved := resolveGridForStructural(grid, &rect, nil, ctx.SlideWidth, ctx.SlideHeight)
	var panes []shapegrid.ResolvedCell
	for _, cell := range resolved.Cells {
		if cell.Kind == shapegrid.CellKindSubGrid {
			panes = append(panes, cell)
		}
	}
	if len(panes) != 3 {
		t.Fatalf("panes%d", len(panes))
	}
	for i, want := range []int64{0, 24 * 12700} {
		got := panes[i+1].CellBounds.X - (panes[i].CellBounds.X + panes[i].CellBounds.CX)
		if math.Abs(float64(got-want)) > 2 {
			t.Fatalf("seam%d got%d want%d", i, got, want)
		}
	}
}

func TestComposeRawGridUsesNormalDesignAndUnknownKeyChecks(t *testing.T) {
	var g jsonschema.ShapeGridInput
	if err := json.Unmarshal([]byte(`{"columns":1,"rows":[{"cells":[{"shape":{"geometry":"rect","fill":"#FF0000","text":{"content":"Evidence","size":17}}}]}]}`), &g); err != nil {
		t.Fatal(err)
	}
	c := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{ShapeGrid: &g}, composeTextLeaf("Details")}}
	findings := checkCompose(c, 1, "/slides/0/compose")
	if len(findings) == 0 || !strings.Contains(findings[0].Path, "/compose/segments/0/shape_grid/") {
		t.Fatalf("rawgrid bypasses design mode: %+v", findings)
	}
	raw := json.RawMessage(`{"direction":"horizontal","segments":[{"text":{"body":"Keep","typo":"lost"}},{"text":{"body":"Other"}}]}`)
	unknown := checkComposeUnknownKeys(raw, "/slides/0/compose")
	if len(unknown) != 1 || !strings.Contains(unknown[0].Path, "/text/typo") {
		t.Fatalf("nested text typo silently dropped: %+v", unknown)
	}
}

func TestComposeTableImageAndRawGridRemainNative(t *testing.T) {
	table := &jsonschema.TableInput{Headers: []string{"Term", "Meaning"}, Rows: [][]jsonschema.TableCellInput{{{Content: "SLA"}, {Content: "Accountable owner"}}}}
	raw := &jsonschema.ShapeGridInput{Columns: json.RawMessage(`1`), Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{composePaneText("Native grid evidence", 12, false)}}}}
	image := &jsonschema.GridImageInput{Path: filepath.Join("..", "..", "examples", "images", "ops-console.png"), Alt: "Operations evidence", Fit: "contain"}
	c := &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{Table: table}, {Image: image}, {ShapeGrid: raw}}}
	ctx := measuredComposeContext()
	grid, _, err := expandCompose(c, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	rect := pptx.RectEmu{X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y, CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
	out, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &rect, nil, ctx.SlideWidth, ctx.SlideHeight, nil)
	if err != nil {
		t.Fatal(err)
	}
	nativeTable, nativeText := false, false
	for _, xml := range out.Shapes {
		nativeTable = nativeTable || strings.Contains(string(xml), "<a:tbl>")
		nativeText = nativeText || strings.Contains(string(xml), "Native grid evidence")
	}
	if !nativeTable || !nativeText || len(out.ImageInserts) != 1 {
		t.Fatalf("nativeleaf missing table%v text%v images%d", nativeTable, nativeText, len(out.ImageInserts))
	}
}

type composeAssetResolver struct{}

func (composeAssetResolver) ResolveImage(string) (string, error) {
	return "/tmp/compose-evidence.png", nil
}
func (composeAssetResolver) ResolveSVG(string) (string, error) { return "<svg/>", nil }
func TestComposeImageURLsUseNormalAssetResolution(t *testing.T) {
	image := &jsonschema.GridImageInput{URL: "https://example.com/evidence.png"}
	nested := &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Image: &jsonschema.GridImageInput{URL: "https://example.com/grid.png"}}}}}}
	slides := []SlideInput{{Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{{Image: image}, {ShapeGrid: nested}}}}}
	if !hasURLReferences(slides) {
		t.Fatal("native compose image URL not discovered")
	}
	if findings := resolveURLs(slides, composeAssetResolver{}); len(findings) > 0 {
		t.Fatal(findings)
	}
	if image.URL != "" || image.Path != "/tmp/compose-evidence.png" || nested.Rows[0].Cells[0].Image.URL != "" {
		t.Fatal("compose image URL escaped normal resolution")
	}
}

func TestComposeExplicitZeroGapSurvivesRoundTrip(t *testing.T) {
	var c ComposeInput
	if err := json.Unmarshal([]byte(`{"direction":"horizontal","gap":0,"segments":[{"text":{"body":"A"}},{"text":{"body":"B"}}]}`), &c); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var again ComposeInput
	if err := json.Unmarshal(raw, &again); err != nil {
		t.Fatal(err)
	}
	if !again.GapSet || composeGapPt(&again, measuredComposeContext()) != 0 {
		t.Fatalf("explicit zero gap lost: %s", raw)
	}
}

func TestComposeEnvelopeReservationMatchesEmittedRows(t *testing.T) {
	for _, gap := range []float64{0, 19} {
		c := &ComposeInput{Direction: "horizontal", Gap: gap, GapSet: true, Banner: &patterns.BannerSpec{Text: "Evidence"}, Callout: &patterns.PatternCallout{Text: "Choose the accountable owner"}, Segments: []SegmentInput{composeTextLeaf("One"), composeTextLeaf("Two")}}
		ctx := measuredComposeContext()
		body, err := composeContentContext(c, ctx)
		if err != nil {
			t.Fatal(err)
		}
		g, _, err := expandCompose(c, ctx, patterns.Default())
		if err != nil {
			t.Fatal(err)
		}
		height := float64(body.LayoutBounds.Height) / 12700
		before, _ := composeBandRows(c, ctx)
		for i, r := range g.Rows {
			if i == len(before) {
				continue
			}
			height += r.MinHeight
		}
		height += float64(len(g.Rows)-1) * gap
		if math.Abs(height-float64(ctx.LayoutBounds.Height)/12700) > 1 {
			t.Fatalf("gap%v measured/envelope allocation mismatch: %v", gap, height)
		}
	}
}
func TestComposeFixedSharesDoNotNormalize(t *testing.T) {
	a, b := composeTextLeaf("One"), composeTextLeaf("Two")
	a.SizePct = 60
	b.SizePct = 20
	if _, _, err := expandCompose(&ComposeInput{Direction: "horizontal", Segments: []SegmentInput{a, b}}, measuredComposeContext(), patterns.Default()); err == nil {
		t.Fatal("incomplete fixed allocation silently normalized")
	}
}

func TestComposeContentKPIAboveChart(t *testing.T) {
	c := &ComposeInput{Direction: "vertical", Gaps: []float64{12}, Segments: []SegmentInput{{Size: "content", Pattern: PatternInput{Name: "kpi-inline", Values: json.RawMessage(`[{"big":"42%","small":"Adoption"},{"big":"18%","small":"Growth"}]`)}}, {Diagram: &types.DiagramSpec{Type: "bar"}}}}
	_, shares, _, err := composeSegmentGrids(c, measuredComposeContext(), patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if shares[0] >= 35 || shares[1] <= 65 {
		t.Fatalf("KPI did not reserve a compact measured strip: %v", shares)
	}
	if _, _, err := expandCompose(c, measuredComposeContext(), patterns.Default()); err != nil {
		t.Fatal(err)
	}
}
