package main

import (
	"encoding/json"
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
	if !strings.HasPrefix(f.Path, "/slides/0/shape_grid/rows/0/cells/") || f.Action != "review" {
		t.Errorf("unexpected path/action: %s %s", f.Path, f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "reduce_text" || f.Fix.Params["geometry"] != "chevron" {
		t.Errorf("unexpected fix: %+v", f.Fix)
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
