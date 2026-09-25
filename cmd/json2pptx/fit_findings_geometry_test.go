package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func geomSlides(t *testing.T, slidesJSON string) *PresentationInput {
	t.Helper()
	var in PresentationInput
	if err := json.Unmarshal([]byte(`{"template":"midnight-blue","slides":`+slidesJSON+`}`), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &in
}

func findingsByCode(fs []patterns.FitFinding, code string) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range fs {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

// TestGeometry_ChevronStripTextExceedsShape mirrors the strategy-deck repro:
// numbered-step-strip chevrons whose notches leave no text width.
// The numbered-step-strip chevron keeps its labels clear of the notch
// (go-slide-creator-c11o), so the pattern itself must not trip the check.
func TestGeometry_ChevronStripPatternFits(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"numbered-step-strip","values":{"style":"chevron","steps":[
		{"label":"Attract","body":"Paid social"},{"label":"Convert","body":"Landing pages"},
		{"label":"Onboard","body":"Welcome series"},{"label":"Retain","body":"Loyalty"},{"label":"Advocate","body":"Referrals"}]}}}]`)
	if fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeTextExceedsShape); len(fs) != 0 {
		t.Fatalf("chevron pattern labels should fit, got %+v", fs)
	}
}

// Hand-authored narrow chevrons whose labels cannot fit between the notches
// are still flagged, aggregated per slide.
func TestGeometry_ChevronTextExceedsShape(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":8,"rows":[{"cells":[
		{"shape":{"geometry":"chevron","fill":"accent1","text":{"content":"Decommissioning","size":18}}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":{"content":"Transformation","size":18}}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Ok"}},{"shape":{"geometry":"chevron","fill":"accent1","text":"Ok"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Ok"}},{"shape":{"geometry":"chevron","fill":"accent1","text":"Ok"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Ok"}},{"shape":{"geometry":"chevron","fill":"accent1","text":"Ok"}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeTextExceedsShape)
	if len(fs) != 1 {
		t.Fatalf("want one aggregated TEXT_EXCEEDS_SHAPE, got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if !strings.HasPrefix(f.Path, "/slides/0/shape_grid/rows/0/cells/") || f.Action != "shrink_or_split" {
		t.Errorf("unexpected path/action: %s %s", f.Path, f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "widen_shape_text_area" || f.Fix.Params["geometry"] != "chevron" {
		t.Fatalf("unexpected fix: %+v", f.Fix)
	}
	if f.OverflowRatio < 2 || f.Severity() != "warning" || f.Fix.Params["minimum_glyph_pt"] == nil {
		t.Errorf("a box too narrow for one glyph must warn and request geometry change: %+v", f)
	}
	if f.Fix.Params["patch_path"] != "/slides/0/shape_grid/rows/0/cells/0/shape/geometry" {
		t.Errorf("raw grid needs a shape geometry patch: %+v", f.Fix.Params)
	}
	patterns.AttachNextToolCalls(fs, func(string) int { return 0 })
	if fs[0].NextToolCall != nil {
		t.Errorf("advisory geometry fix must not offer a non-executable reduce_text call: %+v", fs[0].NextToolCall)
	}
	// Tall, narrow chevrons leave (almost) no width between the notches, so
	// every cell is legitimately flagged; the long labels must be among them.
	cells, _ := f.Fix.Params["cells"].([]string)
	if len(cells) < 2 || cells[0] != "/slides/0/shape_grid/rows/0/cells/0/shape/text" || cells[1] != "/slides/0/shape_grid/rows/0/cells/1/shape/text" {
		t.Errorf("want the long-label cells flagged, got %v", cells)
	}
}

func TestGeometry_RectLongWordAndShortLabelOK(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":8,"rows":[{"cells":[
		{"shape":{"geometry":"rect","fill":"accent1","text":{"content":"Decommissioning","size":16}}},
		{"shape":{"geometry":"rect","fill":"accent1","text":"Ok"}},{"shape":{"geometry":"rect","fill":"accent1","text":"Ok"}},
		{"shape":{"geometry":"rect","fill":"accent1","text":"Ok"}},{"shape":{"geometry":"rect","fill":"accent1","text":"Ok"}},
		{"shape":{"geometry":"rect","fill":"accent1","text":"Ok"}},{"shape":{"geometry":"rect","fill":"accent1","text":"Ok"}},
		{"shape":{"geometry":"rect","fill":"accent1","text":"Ok"}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeTextExceedsShape)
	if len(fs) != 1 || fs[0].Path != "/slides/0/shape_grid/rows/0/cells/0/shape/text" {
		t.Fatalf("want TEXT_EXCEEDS_SHAPE on cell 0 only, got %+v", fs)
	}
	if cells, _ := fs[0].Fix.Params["cells"].([]string); len(cells) != 1 {
		t.Errorf("short labels must not be flagged: %v", cells)
	}
}

func TestGeometry_PointyOverflowSuppressesRaiseBandAdvice(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"bounds":{"x":0,"y":0,"width":100,"height":35},"columns":8,"rows":[{"cells":[
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Baseline"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Design"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Consult"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Migrate"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Stabilise"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Launch"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Validate"}},
		{"shape":{"geometry":"chevron","fill":"accent1","text":"Close"}}]}]}}]`)
	fs := collectGeometryFindings(in, nil, 0, 0, nil)
	if len(findingsByCode(fs, patterns.ErrCodeTextExceedsShape)) == 0 {
		t.Fatal("test setup needs a width-constrained chevron")
	}
	under := findingsByCode(fs, patterns.ErrCodeSlideUnderused)
	if len(under) != 1 {
		t.Fatalf("want one underused finding, got %+v", under)
	}
	hint, _ := under[0].Fix.Params["hint"].(string)
	if strings.Contains(hint, "raise or remove") || !strings.Contains(hint, "taller band would narrow") {
		t.Errorf("contradictory band advice: %q", hint)
	}
}

func TestGeometry_OverflowRatioControlsWorstHitAndPatternPatch(t *testing.T) {
	if got := geometryOverflowRatio(textExceedsHit{wordPt: 49, availPt: 0}); got < 2 {
		t.Fatalf("zero usable width must be severe, got ratio %g", got)
	}
	acc := &geomAccumulator{exceeds: []textExceedsHit{
		{path: "/slides/2/pattern/rows/0/cells/0/shape/text", word: "Longer", geometry: "rect", wordPt: 80, availPt: 40, minGlyphPt: 5},
		{path: "/slides/2/pattern/rows/0/cells/1/shape/text", word: "Baseline", geometry: "chevron", wordPt: 49.4, availPt: 3.3, minGlyphPt: 7},
	}}
	fs := acc.findings("process-flow-compact", false)
	if len(fs) != 1 {
		t.Fatalf("want one finding, got %+v", fs)
	}
	f := fs[0]
	if f.OverflowRatio < 14 || f.Action != "shrink_or_split" || f.Fix.Params["word"] != "Baseline" {
		t.Errorf("most severe ratio should win: %+v", f)
	}
	if got := f.Fix.Params["patch_path"]; got != "/slides/2/pattern/max_height_pct" {
		t.Errorf("pattern cap patch path = %v", got)
	}
	if hint, _ := f.Fix.Params["hint"].(string); !strings.Contains(hint, "one glyph") {
		t.Errorf("non-convergent text shortening not explained: %q", hint)
	}
	if got := acc.findings("process-flow-compact", true)[0].Fix.Params["patch_path"]; got != "/slides/2/pattern/bounds/height" {
		t.Errorf("explicit bounds must take priority over max_height_pct: %v", got)
	}
}

func TestGeometry_SparseFillOnBigEmptyCards(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":3,"rows":[{"cells":[
		{"shape":{"geometry":"rect","fill":"accent1","text":"$45M"}},
		{"shape":{"geometry":"rect","fill":"accent1","text":"$310M"}},
		{"shape":{"geometry":"rect","fill":"accent1","text":"Year 4"}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSparseFill)
	if len(fs) != 1 {
		t.Fatalf("want one SPARSE_FILL, got %+v", fs)
	}
	if cells, _ := fs[0].Fix.Params["cells"].([]string); len(cells) != 3 || fs[0].Fix.Kind != "add_detail_or_resize" {
		t.Errorf("unexpected fix: %+v", fs[0].Fix)
	}
}

func TestGeometry_ExplicitlyEmptyFilledCardsAreNotContent(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":3,"rows":[{"cells":[{"shape":{"geometry":"rect","fill":"accent1","text":""}},{"shape":{"geometry":"rect","fill":"accent1","text":""}},{"shape":{"geometry":"rect","fill":"accent1","text":""}}]}]}}]`)
	fs := collectGeometryFindings(in, nil, 0, 0, nil)
	if sparse := findingsByCode(fs, patterns.ErrCodeSparseFill); len(sparse) != 1 {
		t.Fatalf("empty filled cards need a SPARSE_FILL finding: %+v", fs)
	}
	if under := findingsByCode(fs, patterns.ErrCodeSlideUnderused); len(under) != 1 || under[0].Fix.Params["content_area_pct"] != float64(0) {
		t.Fatalf("empty filled cards must not count as slide content ink: %+v", fs)
	}
	decorative := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":1,"rows":[{"cells":[{"shape":{"geometry":"rect","fill":"accent1"}}]}]}}]`)
	if sparse := findingsByCode(collectGeometryFindings(decorative, nil, 0, 0, nil), patterns.ErrCodeSparseFill); len(sparse) != 0 {
		t.Fatalf("fill-only decorative shape should not be treated as an empty text card: %+v", sparse)
	}
}

func TestGeometry_DenseCardsNotSparse(t *testing.T) {
	long := strings.Repeat("Detailed supporting narrative text that fills the card. ", 12)
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":2,"rows":[{"cells":[
		{"shape":{"geometry":"rect","fill":"accent1","text":{"content":"`+long+`","size":14}}},
		{"shape":{"geometry":"rect","fill":"accent1","text":{"content":"`+long+`","size":14}}}]}]}}]`)
	fs := collectGeometryFindings(in, nil, 0, 0, nil)
	if got := findingsByCode(fs, patterns.ErrCodeSparseFill); len(got) != 0 {
		t.Errorf("dense cards flagged SPARSE_FILL: %+v", got)
	}
	if got := findingsByCode(fs, patterns.ErrCodeSlideUnderused); len(got) != 0 {
		t.Errorf("full-width filled grid flagged SLIDE_UNDERUSED: %+v", got)
	}
}

func TestGeometry_SlideUnderusedForSmallUnfilledText(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":1,"rows":[{"cells":[
		{"shape":{"geometry":"rect","fill":"none","text":"One short line"}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
	if len(fs) != 1 || fs[0].Path != "/slides/0" {
		t.Fatalf("want SLIDE_UNDERUSED at /slides/0, got %+v", fs)
	}

	// Body placeholder content shares the content area: no verdict.
	in.Slides[0].Content = []ContentInput{{PlaceholderID: "body", Type: "text"}}
	if fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused); len(fs) != 0 {
		t.Errorf("slides with body placeholder content must be skipped: %+v", fs)
	}
}

func TestGeometry_RegisteredCodesAndMeta(t *testing.T) {
	for _, code := range []string{patterns.ErrCodeTextExceedsShape, patterns.ErrCodeSparseFill, patterns.ErrCodeSlideUnderused} {
		if _, ok := patterns.GetFindingMeta(code); !ok {
			t.Errorf("%s has no finding meta", code)
		}
		found := false
		for _, c := range patterns.AllFitFindingCodes() {
			found = found || c == code
		}
		if !found {
			t.Errorf("%s missing from AllFitFindingCodes", code)
		}
	}
}

func TestGeometryTextWidthEMU(t *testing.T) {
	b := pptx.RectEmu{CX: 2000000, CY: 1000000}
	cases := map[string]int64{"rect": 2000000, "chevron": 1000000, "homePlate": 1750000, "diamond": 1000000}
	for geo, want := range cases {
		if got := geometryTextWidthEMU(&shapegrid.ShapeSpec{Geometry: geo}, b); got != want {
			t.Errorf("%s: got %d want %d", geo, got, want)
		}
	}
	adj := &shapegrid.ShapeSpec{Geometry: "chevron", Adjustments: map[string]int64{"adj": 20000}}
	if got := geometryTextWidthEMU(adj, b); got != 1600000 {
		t.Errorf("chevron adj=20000: got %d", got)
	}
}

func TestShapeIsFilled(t *testing.T) {
	cases := map[string]bool{
		``: false, `"none"`: false, `"accent1"`: true, `"lt1"`: false,
		`{"color":"accent2"}`: true, `{"color":"accent2","alpha":10}`: false,
		`{"color":"accent2","alpha":0.5}`: true, `{"color":"lt1","alpha":0}`: false,
	}
	for raw, want := range cases {
		if got := shapeIsFilled(json.RawMessage(raw)); got != want {
			t.Errorf("%s: got %v want %v", raw, got, want)
		}
	}
}

// go-slide-creator-up04. SLIDE_UNDERUSED measured a content-sized pattern band
// against the whole content zone and demanded 45%, so the deliberately-shorter
// slides go-slide-creator-7km8 produced were all flagged — on three hand-crafted
// decks every firing was a false positive. Worse, the fix told the agent to
// "remove bounds / max_height_pct caps" it had never set: the height came from
// inside the pattern, so the advice could not be acted on.

// geomPatternSlide builds a process-flow slide, optionally capped by the author.
// process-flow's own content-derived band covers 35% of the content zone — under
// the old flat 45% threshold, so it was flagged for being exactly the height
// go-slide-creator-7km8 gave it.
func geomPatternSlide(t *testing.T, maxHeightPct float64) *PresentationInput {
	t.Helper()
	cap := ""
	if maxHeightPct > 0 {
		cap = fmt.Sprintf(`"max_height_pct":%g,`, maxHeightPct)
	}
	return geomSlides(t, `[{"layout_id":"content","pattern":{"name":"process-flow",`+cap+`
		"values":{"steps":[{"label":"Discover"},{"label":"Build"},{"label":"Ship"}]}}}]`)
}

func TestGeometry_ContentSizedPatternBandIsNotUnderused(t *testing.T) {
	for _, pattern := range []string{
		`{"name":"process-flow","values":{"steps":[{"label":"Discover"},{"label":"Build"},{"label":"Ship"}]}}`,
		`{"name":"process-flow-compact","values":{"steps":[{"label":"Discover"},{"label":"Build"},{"label":"Ship"}]}}`,
	} {
		in := geomSlides(t, `[{"layout_id":"content","pattern":`+pattern+`}]`)
		if fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused); len(fs) != 0 {
			t.Errorf("a pattern at its own content-derived height was flagged: %+v", fs)
		}
	}
}

func TestGeometry_KPIEmptyVerticalBandReportsDespiteCardArea(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"kpi-3up","values":[{"big":"41%","small":"Share of revenue"},{"big":"17","small":"New logos"},{"big":"3x","small":"Pipeline growth"}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
	if len(fs) != 1 {
		t.Fatalf("centered KPI row should report its empty vertical band: %+v", fs)
	}
	if gap, ok := fs[0].Fix.Params["largest_empty_band_in"].(float64); !ok || gap < .75 {
		t.Fatalf("missing measured vertical gap: %+v", fs[0].Fix)
	}
}

func TestGeometry_SparsePanelsNowReportVisibleInk(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"stylish-panels","values":[{"title":"A","body":["one","two"]},{"title":"B","body":["one","two"]},{"title":"C","body":["one","two"]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
	if len(fs) != 1 || fs[0].Fix.Params["content_area_pct"].(float64) >= 20 {
		t.Fatalf("sparse panels should report their visible ink, got %+v", fs)
	}
}

func TestGeometry_SparseSemanticPatternKindsReportUnderfill(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values string
	}{
		{"team-bios", `{"members":[{"name":"Amara Okafor","role":"Partner","bio":"Leads the work."},{"name":"Jonas Weber","role":"Delivery lead","bio":"Runs the plan."},{"name":"Priya Raman","role":"Data lead","bio":"Owns reporting."}]}`},
		{"phase-roadmap", `{"phases":[{"name":"Pilot","date_label":"Q1","description":"Prove value"},{"name":"Expand","date_label":"Q2","description":"Scale"},{"name":"Launch","date_label":"Q3","description":"Go live"}]}`},
		{"agenda", `{"items":["Performance","Risks","Investment"]}`},
		{"icon-row", `[{"icon":"rocket","caption":"Launch"},{"icon":"trending-up","caption":"Growth"},{"icon":"currency-dollar","caption":"Revenue"}]`},
		{"quote-cluster", `{"quotes":[{"text":"Faster decisions.","name":"A. Lee"},{"text":"Better data.","name":"J. Smith"},{"text":"Less rework.","name":"P. Kim"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"`+tc.name+`","values":`+tc.values+`}}]`)
			fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
			if len(fs) != 1 {
				t.Fatalf("sparse %s should report slide underfill: %+v", tc.name, fs)
			}
		})
	}
}

func TestInkCoverageFractionUsesUnionWithinSafeZone(t *testing.T) {
	safe := pptx.RectEmu{CX: 100, CY: 100}
	for _, tc := range []struct {
		name string
		ink  []pptx.RectEmu
		want float64
	}{
		{"opposite corners", []pptx.RectEmu{{CX: 10, CY: 10}, {X: 90, Y: 90, CX: 10, CY: 10}}, 0.02},
		{"overlap counted once", []pptx.RectEmu{{CX: 40, CY: 40}, {X: 20, Y: 20, CX: 40, CY: 40}}, 0.28},
		{"clipped at zone edge", []pptx.RectEmu{{X: 90, Y: 90, CX: 30, CY: 30}}, 0.01},
		{"outside excluded", []pptx.RectEmu{{X: 120, Y: 120, CX: 20, CY: 20}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := inkCoverageFraction(tc.ink, safe); math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("coverage = %g, want %g", got, tc.want)
			}
		})
	}
}

func TestGeometry_KPIGapBelowThresholdDoesNotFlagFilledSlide(t *testing.T) {
	safe := pptx.RectEmu{CX: 1000000, CY: 2000000}
	ink := []pptx.RectEmu{{Y: 500000, CX: 1000000, CY: 1000000}}
	if got := largestVerticalInkGap(ink, safe); got != 500000 {
		t.Fatalf("largest gap = %d EMU, want 500000", got)
	}
	slide := &SlideInput{Pattern: &PatternInput{Name: "kpi-3up"}}
	if f := checkSlideUnderused(ink, safe, slide, 0, "kpi-3up", false); f != nil {
		t.Fatalf("filled KPI slide with sub-threshold gap was flagged: %+v", f)
	}
}

func TestGridOccupancyDoesNotCountPatternPadding(t *testing.T) {
	pattern := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"driver-tree","values":{"root":{"label":"Growth"},"branches":[{"label":"Demand","leaves":["Reach","Convert"]},{"label":"Supply","leaves":["Build","Ship"]}]}}}]`)
	if fs := findingsByCode(collectFitFindings(pattern, nil, 0, 0, nil), patterns.ErrCodePatternUnderfilled); len(fs) != 0 {
		t.Fatalf("expander padding is not unused authored capacity: %+v", fs)
	}
	raw := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":4,"rows":[{"cells":[{"shape":{"geometry":"rect","text":"One"}}]},{"cells":[{"shape":{"geometry":"rect","text":"Two"}}]}]}}]`)
	if fs := findingsByCode(collectGridOccupancyFindings(raw), patterns.ErrCodePatternUnderfilled); len(fs) != 1 {
		t.Fatalf("authored raw slots still need occupancy advice: %+v", fs)
	}
	grid := raw.Slides[0].ShapeGrid
	if preview := computeResolvedOccupancy(grid, false); preview == nil || preview.FilledPct != 25 {
		t.Fatalf("raw preview should keep authored capacity: %+v", preview)
	}
	if preview := computeResolvedOccupancy(grid, true); preview == nil || preview.FilledPct != 100 {
		t.Fatalf("generated preview should exclude padded slots: %+v", preview)
	}
	if exposed := computeGridOccupancy(grid, patterns.ExpandContext{}); exposed.FilledPct != 100 {
		t.Fatalf("validate_pattern should not expose padded slots as omissions: %+v", exposed)
	}
}

func TestFrameworkExemplarsDoNotReportGeneratedCellUnderfill(t *testing.T) {
	for _, name := range []string{"bmc-canvas", "journey-maturity-model", "agenda", "matrix-2x2", "driver-tree"} {
		t.Run(name, func(t *testing.T) {
			p, ok := patterns.Default().Get(name)
			if !ok {
				t.Fatalf("pattern %q is not registered", name)
			}
			ex, ok := p.(patterns.Exemplar)
			if !ok {
				t.Fatalf("pattern %q has no exemplar", name)
			}
			values, err := json.Marshal(ex.ExemplarValues())
			if err != nil {
				t.Fatal(err)
			}
			in := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"`+name+`","values":`+string(values)+`}}]`)
			for _, f := range collectFitFindings(in, nil, 0, 0, nil) {
				if f.Code == patterns.ErrCodePatternUnderfilled || f.Code == patterns.ErrCodeCellUnderfilled {
					t.Errorf("generated framework padding reported as missing content: %+v", f)
				}
			}
		})
	}
}

// The same pattern, capped by the author, still reports — and now the advice is
// something they can act on, because the cap is theirs.
func TestGeometry_AuthorCappedBandIsUnderused(t *testing.T) {
	fs := findingsByCode(collectGeometryFindings(geomPatternSlide(t, 20), nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
	if len(fs) != 1 {
		t.Fatalf("want one SLIDE_UNDERUSED for an author-capped band, got %+v", fs)
	}
	f := fs[0]
	if got := f.Fix.Params["band_capped_by"]; got != "author" {
		t.Errorf("band_capped_by = %v, want author", got)
	}
	hint, _ := f.Fix.Params["hint"].(string)
	if !strings.Contains(hint, "max_height_pct") {
		t.Errorf("an author-capped band must be told about its cap: %q", hint)
	}
	if got := f.Fix.Params["threshold_pct"]; got != math.Round(100*slideUnderusedMaxFrac) {
		t.Errorf("threshold_pct = %v, want %v", got, math.Round(100*slideUnderusedMaxFrac))
	}
}

// A band nobody capped still reports when it is a genuine sliver — a four-stop
// timeline covers 10% of the content zone — but the advice is about content,
// never about a cap that does not exist.
func TestGeometry_UncappedSliverAdvisesContentNotCaps(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"timeline-horizontal",
		"values":[{"label":"Q1"},{"label":"Q2"},{"label":"Q3"},{"label":"Q4"}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
	if len(fs) != 1 {
		t.Fatalf("want one SLIDE_UNDERUSED for a sliver, got %+v", fs)
	}
	f := fs[0]
	if got := f.Fix.Params["band_capped_by"]; got != "pattern" {
		t.Errorf("band_capped_by = %v, want pattern", got)
	}
	hint, _ := f.Fix.Params["hint"].(string)
	for _, forbidden := range []string{"remove bounds", "max_height_pct"} {
		if strings.Contains(hint, forbidden) {
			t.Errorf("hint tells the agent to edit a cap it never set: %q", hint)
		}
	}
	for _, want := range []string{"add detail", "compose"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint %q does not name an action the agent can take (%q)", hint, want)
		}
	}
}

func geomCompactChevronSlide(t *testing.T) *PresentationInput {
	t.Helper()
	return geomSlides(t, `[{"layout_id":"content","pattern":{"name":"process-flow-compact",
		"values":{"steps":[{"label":"A","type":"chevron"},{"label":"B","type":"chevron"},
		{"label":"C","type":"chevron"},{"label":"D","type":"chevron"},
		{"label":"E","type":"chevron"},{"label":"F","type":"chevron"},
		{"label":"G","type":"chevron"},{"label":"H","type":"chevron"}]}}}]`)
}

// process-flow-compact emits its own Bounds during expansion. Those bounds are
// not an author cap and must not change the threshold or remediation advice.
func TestGeometry_ExpandedPatternBoundsAreNotAuthorCaps(t *testing.T) {
	in := geomCompactChevronSlide(t)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
	if len(fs) != 1 {
		t.Fatalf("want one SLIDE_UNDERUSED for a compact pattern sliver, got %+v", fs)
	}
	f := fs[0]
	if got := f.Fix.Params["band_capped_by"]; got != "pattern" {
		t.Errorf("band_capped_by = %v, want pattern", got)
	}
	if got := f.Fix.Params["threshold_pct"]; got != math.Round(100*slideUnderusedPatternMaxFrac) {
		t.Errorf("threshold_pct = %v, want %.0f", got, 100*slideUnderusedPatternMaxFrac)
	}
	if hint, _ := f.Fix.Params["hint"].(string); strings.Contains(hint, "max_height_pct") {
		t.Errorf("pattern-owned bounds should not produce author-cap advice: %q", hint)
	}
}

// collectFitFindings expands patterns before geometry detection; this is the
// production path for all three attribution sources.
func TestFitFindings_BandCapAttribution(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   *PresentationInput
		want string
	}{
		{"pattern_owned", geomCompactChevronSlide(t), "pattern"},
		{"author_capped", geomPatternSlide(t, 20), "author"},
		{"raw_full_area", geomSparseRawGrid(t, `"bounds":{"x":0,"y":0,"width":100,"height":100},`), "none"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := findingsByCode(collectFitFindings(tt.in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
			if len(fs) != 1 {
				t.Fatalf("want one SLIDE_UNDERUSED, got %+v", fs)
			}
			if got := fs[0].Fix.Params["band_capped_by"]; got != tt.want {
				t.Errorf("band_capped_by = %v, want %s", got, tt.want)
			}
		})
	}
}

func TestGeometry_RawGridBoundsRemainAuthorCaps(t *testing.T) {
	in := geomSlides(t, `[{"layout_id":"content","shape_grid":{"bounds":{"x":0,"y":0,"width":100,"height":20},
		"columns":1,"rows":[{"cells":[{"shape":{"geometry":"rect","fill":"accent1","text":"Short"}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
	if len(fs) != 1 {
		t.Fatalf("want one SLIDE_UNDERUSED for an author-bounded raw grid, got %+v", fs)
	}
	if got := fs[0].Fix.Params["band_capped_by"]; got != "author" {
		t.Errorf("band_capped_by = %v, want author", got)
	}
}

func geomSparseRawGrid(t *testing.T, bounds string) *PresentationInput {
	t.Helper()
	return geomSlides(t, `[{"layout_id":"content","shape_grid":{`+bounds+`"columns":1,
		"rows":[{"cells":[{"shape":{"geometry":"rect","text":{"content":"Short","align":"ctr","vertical_align":"ctr"}}}]}]}}]`)
}

func TestGeometry_UncappedRawGridAdvice(t *testing.T) {
	for _, tt := range []struct {
		name   string
		bounds string
	}{
		{"no_bounds", ""},
		{"full_area_bounds", `"bounds":{"x":0,"y":0,"width":100,"height":100},`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := geomSparseRawGrid(t, tt.bounds)
			fs := findingsByCode(collectGeometryFindings(in, nil, 0, 0, nil), patterns.ErrCodeSlideUnderused)
			if len(fs) != 1 {
				t.Fatalf("want one SLIDE_UNDERUSED for sparse raw grid, got %+v", fs)
			}
			f := fs[0]
			if got := f.Fix.Params["band_capped_by"]; got != "none" {
				t.Errorf("band_capped_by = %v, want none", got)
			}
			if hint, _ := f.Fix.Params["hint"].(string); strings.Contains(hint, "max_height_pct") || strings.Contains(hint, "denser pattern") {
				t.Errorf("uncapped raw grid received cap/pattern advice: %q", hint)
			}
		})
	}
}

func TestGeometry_FullAreaPatternBoundsHaveNoCapAdvice(t *testing.T) {
	full := &GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 100}
	slide := &SlideInput{Pattern: &PatternInput{Bounds: full}, ShapeGrid: &ShapeGridInput{Bounds: full}}
	fs := checkSlideUnderused([]pptx.RectEmu{{X: 45, Y: 45, CX: 10, CY: 10}},
		pptx.RectEmu{X: 0, Y: 0, CX: 100, CY: 100}, slide, 0, "process-flow", false)
	if fs == nil {
		t.Fatal("want SLIDE_UNDERUSED for a sparse full-area pattern")
	}
	if got := fs.Fix.Params["band_capped_by"]; got != "none" {
		t.Errorf("band_capped_by = %v, want none", got)
	}
	if hint, _ := fs.Fix.Params["hint"].(string); strings.Contains(hint, "max_height_pct") {
		t.Errorf("full-area pattern bounds received cap advice: %q", hint)
	}
}

func TestBandCapSource(t *testing.T) {
	capped := geomPatternSlide(t, 20)
	if got := bandCapSource(&capped.Slides[0]); got != "author" {
		t.Error("max_height_pct on the pattern is an author cap")
	}
	uncapped := geomPatternSlide(t, 0)
	if got := bandCapSource(&uncapped.Slides[0]); got != "pattern" {
		t.Error("a pattern with no bounds and no max_height_pct is not author-capped")
	}
	bounded := geomSlides(t, `[{"layout_id":"content","pattern":{"name":"kpi-3up","bounds":{"x":0,"y":0,"width":100,"height":30},
		"values":[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]}}]`)
	if got := bandCapSource(&bounded.Slides[0]); got != "author" {
		t.Error("explicit bounds on the pattern is an author cap")
	}
	if got := bandCapSource(nil); got != "none" {
		t.Errorf("nil slide source = %q, want none", got)
	}
	for _, tt := range []struct {
		name   string
		bounds GridBoundsInput
		want   bool
	}{
		{"full_slide", GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 100}, false},
		{"inset_left", GridBoundsInput{X: 10, Y: 0, Width: 90, Height: 100}, true},
		{"inset_bottom", GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 90}, true},
		{"oversized_covering_slide", GridBoundsInput{X: -10, Y: -10, Width: 120, Height: 120}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := boundsConstrainArea(&tt.bounds); got != tt.want {
				t.Errorf("boundsConstrainArea(%+v) = %v, want %v", tt.bounds, got, tt.want)
			}
		})
	}
}

// TestGeometryHonoursRotatedText pins the fix that makes a thin band with a
// rotated label legal: TEXT_EXCEEDS_SHAPE measures a word against the line
// length it has, and rotated text runs along the shape's HEIGHT. Without this
// the conventional way to draw a cross-cutting concern — arch-stack's side
// rails — reported every label as overflowing (go-slide-creator-pr3g).
func TestGeometryHonoursRotatedText(t *testing.T) {
	// A 30pt-wide, 300pt-tall band carrying a rotated label.
	band := func(vert string) *ShapeGridInput {
		text := `{"paragraphs":[{"content":"Monitoring","size":10}],"align":"ctr","vertical_align":"ctr"}`
		if vert != "" {
			text = `{"paragraphs":[{"content":"Monitoring","size":10}],"align":"ctr","vertical_align":"ctr","vert":"` + vert + `"}`
		}
		return &ShapeGridInput{
			Bounds:  &GridBoundsInput{X: 0, Y: 0, Width: 4, Height: 80},
			Columns: json.RawMessage(`1`),
			Rows: []GridRowInput{{Cells: []*GridCellInput{{
				Shape: &ShapeSpecInput{
					Geometry: "rect",
					Fill:     json.RawMessage(`"lt2"`),
					Text:     json.RawMessage(text),
				},
			}}}},
		}
	}

	countExceeds := func(grid *ShapeGridInput) int {
		input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: grid}}}
		n := 0
		for _, f := range collectGeometryFindings(input, nil, 12192000, 6858000, nil) {
			if f.Code == patterns.ErrCodeTextExceedsShape {
				n++
			}
		}
		return n
	}

	if countExceeds(band("")) == 0 {
		t.Error("a horizontal label in a 4%-wide band should overflow — the test fixture is wrong")
	}
	if got := countExceeds(band("vert270")); got != 0 {
		t.Errorf("a rotated label has the band's height to run along; got %d TEXT_EXCEEDS_SHAPE findings", got)
	}
}
