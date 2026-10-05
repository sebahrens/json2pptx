package generator

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// diagramPlaceholders are the diagram placeholder each shipped template (and
// the local p-style) gives a native diagram, in points, with the template's
// body face: the regions go-slide-creator-6ne1m and -35rsq were measured in.
var diagramPlaceholders = []struct {
	template string
	w, h     int64
	font     string
}{
	{"abstract", 828, 342, "Tenorite"},
	{"blue-corporate", 825, 328, "Aptos Light"},
	{"business-template", 951, 369, "Calibri"},
	{"forest-green", 828, 342, "Calibri"},
	{"midnight-blue", 796, 342, "Calibri"},
	{"modern", 850, 273, "Calibri"},
	{"modern-template", 824, 282, "Poppins Light"},
	{"modern-yellow", 863, 290, "Segoe UI"},
	{"warm-coral", 828, 342, "Calibri"},
	{"p-style", 899, 315, "Arial"},
}

func ptBounds(w, h int64) types.BoundingBox {
	return types.BoundingBox{X: 600000, Y: 1800000, Width: w * int64(types.EMUPerPoint), Height: h * int64(types.EMUPerPoint)}
}

var (
	testRunSizeRE   = regexp.MustCompile(`<a:rPr[^>]* sz="(\d+)"`)
	testFontScaleRE = regexp.MustCompile(`fontScale="(\d+)"`)
)

// nativeGroupFor lays spec out in bounds and renders its group.
func nativeGroupFor(t *testing.T, spec *types.DiagramSpec, bounds types.BoundingBox, font string) (string, nativeDiagramLayout) {
	t.Helper()
	env := nativeDiagramEnv{fontName: font}
	layout, err := layoutNativeDiagram(spec, bounds, env, nativeDiagramSite{})
	if err != nil {
		t.Fatal(err)
	}
	group := renderNativeInsert(&layout.insert, 100, env)
	if group == "" {
		t.Fatal("no native group rendered")
	}
	return group, layout
}

// The shipped native-diagram examples are written at the 12pt body step or
// above, with no stored shrink, in every shipped template's diagram
// placeholder. Porter's factors were 10pt under a 9pt intensity line and
// nine-box names 10pt, both stored at 88-96% on the three short templates;
// the BMC, value chain and pyramid wrote 10pt and 9pt with no shrink for any
// check to see (go-slide-creator-6ne1m, go-slide-creator-35rsq).
func TestNativeDiagramExamplesAreWrittenAtBodySize(t *testing.T) {
	for _, name := range []string{"porters_five_forces", "nine_box_talent", "business_model_canvas", "pyramid", "value_chain", "swot", "house"} {
		spec := exampleDiagramSpec(t, name)
		for _, p := range diagramPlaceholders {
			t.Run(name+"/"+p.template, func(t *testing.T) {
				group, layout := nativeGroupFor(t, spec, ptBounds(p.w, p.h), p.font)
				for _, m := range testRunSizeRE.FindAllStringSubmatch(group, -1) {
					if sz, _ := strconv.Atoi(m[1]); sz < tokens.TypeScaleBodyHPt {
						t.Errorf("a run is written at %.1fpt, below the 12pt body step", float64(sz)/100)
						break
					}
				}
				if m := testFontScaleRE.FindStringSubmatch(group); m != nil {
					t.Errorf("a shape stores an autofit shrink (fontScale %s)", m[1])
				}
				if layout.insert.fitBudget != nil {
					t.Errorf("the layout reports the example does not fit: %+v", *layout.insert.fitBudget)
				}
				if f := unreadableAutofitFindings(group, tokens.ViewingModePresentation, func(string) string { return "" }, nil); len(f) != 0 {
					t.Errorf("TEXT_BELOW_READABLE_MIN: %s", f[0].Message)
				}
			})
		}
	}
}

// A PESTEL of three bullets a segment — the documented sample — is written at
// 12pt on the shortest placeholders. Header and body each kept the uniform
// margin at their shared seam and the 11pt bullets were stored at 78%.
func TestPESTELSampleIsWrittenAtBodySize(t *testing.T) {
	spec := &types.DiagramSpec{Type: "pestel", Data: map[string]any{
		"political":     []any{"Stable government", "Trade agreements", "Tax incentives"},
		"economic":      []any{"GDP growth 3%", "Low inflation", "Strong currency"},
		"social":        []any{"Young population", "Rising middle class", "Digital adoption"},
		"technological": []any{"5G rollout", "Cloud infrastructure", "AI readiness"},
		"environmental": []any{"ESG regulations", "Carbon targets", "Green subsidies"},
		"legal":         []any{"IP protection", "Data privacy laws", "Labor regulations"},
	}}
	for _, p := range diagramPlaceholders {
		t.Run(p.template, func(t *testing.T) {
			group, _ := nativeGroupFor(t, spec, ptBounds(p.w, p.h), p.font)
			if m := testFontScaleRE.FindStringSubmatch(group); m != nil {
				t.Errorf("a shape stores an autofit shrink (fontScale %s)", m[1])
			}
			if strings.Contains(group, `sz="1100"`) {
				t.Error("bullets are still written at 11pt")
			}
		})
	}
}

func porterTestSpec(factors int, factor string) *types.DiagramSpec {
	var list []any
	for i := 0; i < factors; i++ {
		list = append(list, factor)
	}
	var forces []any
	for _, ft := range []string{"rivalry", "new_entrants", "substitutes", "suppliers", "buyers"} {
		forces = append(forces, map[string]any{"type": ft, "intensity": 0.5, "factors": list})
	}
	return &types.DiagramSpec{Type: "porters_five_forces", Data: map[string]any{"forces": forces}}
}

// Porter's factors are a real bulleted list: a glyph in a bullet font, a left
// margin and a hanging indent. With no margin the bullet sat against its text
// and a wrapped line started under the glyph (go-slide-creator-6ne1m).
func TestPorterFactorBulletsHang(t *testing.T) {
	group, _ := nativeGroupFor(t, exampleDiagramSpec(t, "porters_five_forces"), ptBounds(824, 282), "Poppins Light")
	bullets := 0
	for _, para := range regexp.MustCompile(`(?s)<a:p>.*?</a:p>`).FindAllString(group, -1) {
		if !strings.Contains(para, "<a:buChar") {
			continue
		}
		bullets++
		if !strings.Contains(para, `marL="177800"`) || !strings.Contains(para, `indent="-177800"`) {
			t.Fatalf("bullet paragraph has no hanging indent: %s", para)
		}
		if strings.Contains(para, "<a:t>*") || strings.Contains(para, "<a:t>- ") {
			t.Fatalf("bullet is a literal character in the text: %s", para)
		}
	}
	if bullets != 15 {
		t.Fatalf("bullet paragraphs = %d, want the example's 15 factors", bullets)
	}
}

// Every factor the author gave is drawn. The list was cut at four (three for
// rivalry) with nothing said about the rest.
func TestPorterDrawsEveryFactor(t *testing.T) {
	spec := porterTestSpec(0, "")
	forces := spec.Data["forces"].([]any)
	for _, f := range forces {
		f.(map[string]any)["factors"] = []any{"One", "Two", "Three", "Four", "Five", "Six"}
	}
	group, _ := nativeGroupFor(t, spec, ptBounds(900, 500), "Calibri")
	if got := strings.Count(group, "<a:t>Six</a:t>"); got != 5 {
		t.Fatalf("sixth factor drawn in %d of 5 forces", got)
	}
}

// On a region wide enough the boxes above and below rivalry are bands — the
// factors beside the header — so the cross fits a short content area at 12pt;
// a narrow region stacks them. Rivalry stays in the centre with the arrows
// pointing in.
func TestPorterBandsUseTheWidthOfALandscapeRegion(t *testing.T) {
	spec := exampleDiagramSpec(t, "porters_five_forces")
	forces := porterForcesFromPanels(porterPanels(spec))
	env := nativeDiagramEnv{fontName: "Calibri"}

	wide := layoutPorter(forces, ptBounds(850, 273), env)
	if !wide.fits {
		t.Fatal("the example does not fit the shortest shipped placeholder")
	}
	center, top, bottom, left, right := wide.boxes[0], wide.boxes[1], wide.boxes[2], wide.boxes[3], wide.boxes[4]
	if top.factorText == nil || bottom.factorText == nil {
		t.Fatal("wide region: new entrants / substitutes are not bands")
	}
	if center.factorText != nil || left.factorText != nil || right.factorText != nil {
		t.Fatal("rivalry and the side forces stack their factors under the header")
	}
	if top.rect.Y+top.rect.CY >= center.rect.Y || center.rect.Y+center.rect.CY >= bottom.rect.Y {
		t.Fatalf("the column overlaps: top %+v center %+v bottom %+v", top.rect, center.rect, bottom.rect)
	}
	if gap := center.rect.Y - (top.rect.Y + top.rect.CY); gap < porterMinConnectorGapEMU {
		t.Fatalf("connector gap %.1fpt is shorter than an arrowhead needs", float64(gap)/float64(types.EMUPerPoint))
	}
	if left.rect.X+left.rect.CX > top.rect.X || top.rect.X+top.rect.CX > right.rect.X {
		t.Fatalf("a band reaches over a side box: left %+v top %+v right %+v", left.rect, top.rect, right.rect)
	}

	narrow := layoutPorter(forces, ptBounds(400, 400), env)
	for _, b := range narrow.boxes {
		if b.factorText != nil {
			t.Fatal("narrow region: a band has no room for two columns")
		}
	}
}

// A cross that cannot hold its factors at 12pt is not shrunk silently: the
// layout measures what the region holds and the readability finding raised on
// its shapes carries that budget.
func TestNativeShrinkCarriesAMeasuredBudget(t *testing.T) {
	spec := porterTestSpec(7, "A factor long enough to wrap onto a second line in any box")
	bounds := ptBounds(850, 273)
	group, layout := nativeGroupFor(t, spec, bounds, "Calibri")
	b := layout.insert.fitBudget
	if b == nil {
		t.Fatal("an over-full cross measured no budget")
	}
	if b.item != "factor" || b.container != "force" || b.maxItems < 1 || b.maxItems >= 7 || b.maxChars < 10 {
		t.Fatalf("budget = %+v", *b)
	}
	// The budget is what fits: that many one-line factors are written unshrunk.
	fits, _ := nativeGroupFor(t, porterTestSpec(b.maxItems, strings.Repeat("x", b.maxChars-2)), bounds, "Calibri")
	if m := testFontScaleRE.FindStringSubmatch(fits); m != nil {
		t.Fatalf("%d factors of %d characters — the reported budget — are stored at fontScale %s", b.maxItems, b.maxChars, m[1])
	}

	if !testFontScaleRE.MatchString(group) {
		t.Fatal("the over-full cross stores no shrink for the readability check to see")
	}
	ctx := &singlePassContext{}
	ctx.nativeShapeSources = []nativeShapeSource{{
		slideIndex: 1, lo: 100, hi: 120, path: "/slides/1/content/1/diagram_value", diagramType: spec.Type, budget: b,
	}}
	findings := unreadableAutofitFindings(group, tokens.ViewingModePresentation, func(id string) string {
		return "/slides/1/rendered_shapes/" + id
	}, nil)
	if len(findings) == 0 {
		t.Fatal("no TEXT_BELOW_READABLE_MIN for a stored shrink below 12pt")
	}
	f := findings[0]
	ctx.applyNativeFitBudget(&f, 1)
	if f.Code != patterns.ErrCodeTextBelowReadableMin || !strings.Contains(f.Message, "one-line factors per force") {
		t.Fatalf("finding = %s: %s", f.Code, f.Message)
	}
	if f.Fix == nil || f.Fix.Params["max_items_per_force"] != b.maxItems || f.Fix.Params["max_chars_per_item"] != b.maxChars ||
		f.Fix.Params["diagram_type"] != "porters_five_forces" || f.Fix.Params["min_pt"] != 12.0 {
		t.Fatalf("fix params = %+v", f.Fix)
	}
	// A shape of another slide, or outside the diagram's ids, is left alone.
	other := findings[0]
	ctx.applyNativeFitBudget(&other, 2)
	if strings.Contains(other.Message, "per force") {
		t.Fatal("budget applied to a shape of another slide")
	}
}

func nineBoxTestSpec(names int, name string, extra map[string]any) *types.DiagramSpec {
	var items []any
	for i := 0; i < names; i++ {
		items = append(items, name)
	}
	var cells []any
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			cells = append(cells, map[string]any{"row": float64(row), "col": float64(col), "items": items})
		}
	}
	data := map[string]any{"cells": cells}
	for k, v := range extra {
		data[k] = v
	}
	return &types.DiagramSpec{Type: "nine_box_talent", Data: data}
}

// A cell whose names do not fit one column at 12pt sets them in two before it
// gives up anything else; a cell they fit keeps one.
func TestNineBoxSetsNamesInColumns(t *testing.T) {
	env := nativeDiagramEnv{fontName: "Calibri"}
	tints := nineBoxSemanticTints(nil)
	panels, _ := nineBoxPanels(exampleDiagramSpec(t, "nine_box_talent"))

	short := layoutNineBox(panels, ptBounds(850, 273), tints, env)
	if !short.fits {
		t.Fatal("the example does not fit the shortest shipped placeholder")
	}
	if got := len(short.cells[4].columns); got != 2 {
		t.Fatalf("Core Employee's four names are set in %d columns on a 273pt region, want 2", got)
	}
	if got := len(short.cells[2].columns); got != 1 {
		t.Fatalf("Star's two names are set in %d columns, want 1", got)
	}
	if len(short.cells[0].columns) != 0 {
		t.Fatal("an empty cell has name columns")
	}
	tall := layoutNineBox(panels, ptBounds(850, 520), tints, env)
	if got := len(tall.cells[4].columns); got != 1 {
		t.Fatalf("four names are set in %d columns where one column fits", got)
	}
	if tall.pad != pptx.ShapeTextInsetEMU {
		t.Fatal("a grid that fits gave up padding")
	}

	group := generateNineBoxGroupXML(panels, ptBounds(850, 273), 100, tints, env)
	if strings.Count(group, `name="NineBox Names"`) < 1 {
		t.Fatal("no second name column was drawn")
	}
	for _, name := range []string{"Chris Anderson", "Rachel Green", "Mike Brown", "Amy Wilson"} {
		if strings.Count(group, ">"+name+"<") != 1 {
			t.Fatalf("%q is not drawn exactly once", name)
		}
	}
}

// The grid is drawn on its axes. x_label / y_label — the spellings of the
// shipped example — were accepted and never read, so the grid had none; with
// no title stated the axes take the documented Performance / Potential. Tick
// labels are drawn only when the author names them.
func TestNineBoxDrawsItsAxes(t *testing.T) {
	bounds := ptBounds(850, 342)
	example, _ := nativeGroupFor(t, exampleDiagramSpec(t, "nine_box_talent"), bounds, "Calibri")
	for _, want := range []string{"Current Performance", "Growth Potential", `name="X-Axis"`, `name="Y-Axis"`, `vert="vert270"`, `type="triangle"`} {
		if !strings.Contains(example, want) {
			t.Errorf("example group lacks %s", want)
		}
	}
	if strings.Contains(example, ">Medium<") {
		t.Error("default tick labels are drawn for an axis the author only titled")
	}

	defaults, _ := nativeGroupFor(t, nineBoxTestSpec(1, "Ana", nil), bounds, "Calibri")
	if !strings.Contains(defaults, ">Performance<") || !strings.Contains(defaults, ">Potential<") {
		t.Error("a grid with no axis titles does not take the documented defaults")
	}

	ticks, _ := nativeGroupFor(t, nineBoxTestSpec(1, "Ana", map[string]any{
		"x_axis_labels": []any{"Developing", "Solid", "Exceptional"},
	}), bounds, "Calibri")
	for _, want := range []string{">Developing<", ">Solid<", ">Exceptional<"} {
		if !strings.Contains(ticks, want) {
			t.Errorf("authored tick label %s is not drawn", want)
		}
	}
	if err := ValidateNativeDiagramData(exampleDiagramSpec(t, "nine_box_talent")); err != nil {
		t.Errorf("x_label / y_label are refused: %v", err)
	}
}

// A grid too full for 12pt measures what a cell holds.
func TestNineBoxOverfullMeasuresABudget(t *testing.T) {
	_, layout := nativeGroupFor(t, nineBoxTestSpec(12, "Alexandra Papadopoulos", nil), ptBounds(850, 273), "Calibri")
	b := layout.insert.fitBudget
	if b == nil {
		t.Fatal("an over-full grid measured no budget")
	}
	if b.item != "name" || b.container != "cell" || b.maxItems < 1 || b.maxItems >= 12 || b.maxChars < 8 {
		t.Fatalf("budget = %+v", *b)
	}
	group, fits := nativeGroupFor(t, nineBoxTestSpec(b.maxItems, "Alexandra Papadopoulos", nil), ptBounds(850, 273), "Calibri")
	if fits.insert.fitBudget != nil || testFontScaleRE.MatchString(group) {
		t.Fatalf("%d names a cell — the reported budget — do not fit unshrunk", b.maxItems)
	}
}

// A layout that sizes a box from its text asks for the height at which the
// text keeps its declared margins. The writer gives a margin up before a line,
// so the bare "fits" height of a one-line label is the line alone.
func TestNativeTextNeedAtMarginKeepsTheMargin(t *testing.T) {
	label := nineBoxLabelText("Star", pptx.ShapeTextInsetEMU, taxonomyTint{scheme: "accent1"}, nativeDiagramEnv{})
	limit := 400 * int64(types.EMUPerPoint)
	line := int64(12 * 1.2 * float64(types.EMUPerPoint))
	want := line + pptx.ShapeTextInsetEMU + nativeCardSeamInsetEMU
	if got := nativeTextNeedAtMarginEMU(label, 200*int64(types.EMUPerPoint), limit); got < want || got > want+2*int64(types.EMUPerPoint) {
		t.Fatalf("need at margin = %.1fpt, want about %.1fpt", float64(got)/12700, float64(want)/12700)
	}
	if got := nativeTextNeedEMU(label, 200*int64(types.EMUPerPoint), limit); got >= want {
		t.Fatalf("the writer's own fit height (%.1fpt) already keeps the margin; the test has lost its subject", float64(got)/12700)
	}
}

// The canvas sets the bullets of its two wide bottom cells in two columns
// before it tightens a margin, and tightens a margin before it shrinks type.
func TestBMCGivesWidthAndPaddingBeforeType(t *testing.T) {
	panels := bmcPanels(exampleDiagramSpec(t, "business_model_canvas"))
	tall := bmcLayoutCells(panels, ptBounds(828, 342), "Calibri")
	if !tall.fits || tall.pad != pptx.ShapeTextInsetEMU || tall.bottomCols != 1 {
		t.Fatalf("a canvas that fits changed its layout: %+v", tall)
	}
	short := bmcLayoutCells(panels, ptBounds(850, 273), "Calibri")
	if !short.fits {
		t.Fatal("the example does not fit the shortest shipped placeholder at 12pt")
	}
	if short.bottomCols == 1 && short.pad == pptx.ShapeTextInsetEMU {
		t.Fatal("the short canvas fits with neither a second column nor a tighter margin; the test has lost its subject")
	}
	// Spare height is shared: on a tall region the bottom row is not handed
	// every spare point while the stacked cells keep their bare need.
	if bottom, total := tall.cells[bmcCostStructure].h, ptBounds(828, 342).Height; bottom*100/total > 36 {
		t.Fatalf("the bottom row takes %d%% of a tall canvas for three bullets", bottom*100/total)
	}
}

// The value chain's bars are one-line strips and its chevrons take the rest of
// the height; a chevron's text is measured in the preset's own text rectangle
// rather than reserving half the tip a second time.
func TestValueChainFillsItsRegion(t *testing.T) {
	spec := exampleDiagramSpec(t, "value_chain")
	panels, meta := parseValueChainData(spec.Data)
	bounds := ptBounds(850, 273)
	l := layoutValueChain(panels, bounds, meta, "Calibri")
	if !l.fits {
		t.Fatal("the example does not fit the shortest shipped placeholder at 12pt")
	}
	if got := l.primaryY + l.primaryH; got != bounds.Y+bounds.Height {
		t.Fatalf("the chevrons end %.1fpt short of the region", float64(bounds.Y+bounds.Height-got)/12700)
	}
	if l.barH > 36*int64(types.EMUPerPoint) {
		t.Fatalf("a one-line support bar is %.1fpt tall", float64(l.barH)/12700)
	}
	group, _ := nativeGroupFor(t, spec, bounds, "Calibri")
	for _, sp := range regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`).FindAllString(group, -1) {
		if strings.Contains(sp, `name="VC Primary`) && !strings.Contains(sp, `rIns="180000"`) {
			t.Fatalf("a chevron reserves more than the uniform right margin: %.200s", sp)
		}
	}
}

// A pyramid keeps one type size at every level count. Eight levels used to be
// written at 9pt labels over 7pt descriptions.
func TestPyramidKeepsBodySizeAtEveryLevelCount(t *testing.T) {
	var levels []any
	for i := 0; i < 9; i++ {
		levels = append(levels, map[string]any{"label": "Level", "description": "Detail"})
	}
	spec := &types.DiagramSpec{Type: "pyramid", Data: map[string]any{"levels": levels}}
	group, layout := nativeGroupFor(t, spec, ptBounds(850, 273), "Calibri")
	if strings.Contains(group, `sz="900"`) || strings.Contains(group, `sz="700"`) {
		t.Fatal("a nine-level pyramid is written at 9pt / 7pt")
	}
	b := layout.insert.fitBudget
	if b == nil || b.item != "level" || b.maxItems < 3 || b.maxItems >= 9 {
		t.Fatalf("nine two-line levels in 273pt: budget = %+v", b)
	}
}
