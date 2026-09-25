package template

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCheckConflictingLayoutTags(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags []string
		want ConformanceStatus
	}{
		{"content section conflict", []string{"content", "section-header"}, ConformanceStatusWarn},
		{"ordinary content", []string{"content"}, ConformanceStatusPass},
		{"two-column comparison", []string{"content", "two-column", "comparison"}, ConformanceStatusPass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkConflictingLayoutTags([]types.LayoutMetadata{{ID: "slideLayout2", Name: "Section", Tags: tc.tags}})
			if len(got) != 1 || got[0].Status != tc.want {
				t.Fatalf("checks = %+v, want one %s", got, tc.want)
			}
			if tc.want == ConformanceStatusWarn && !strings.Contains(got[0].Detail, "slideLayout2") {
				t.Errorf("warning omits layout ID: %+v", got[0])
			}
		})
	}
}

func TestCheckTextPlaceholderOverlap(t *testing.T) {
	box := func(x, y, w, h int64) types.BoundingBox {
		return types.BoundingBox{X: x, Y: y, Width: w, Height: h}
	}
	title := types.PlaceholderInfo{ID: "title", Type: types.PlaceholderTitle, Bounds: box(0, 0, 2000000, 500000)}
	for _, tc := range []struct {
		name  string
		other types.PlaceholderInfo
		want  ConformanceStatus
	}{
		{"clear overlap", types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, Bounds: box(0, 400000, 2000000, 1000000)}, ConformanceStatusWarn},
		{"touching edges", types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, Bounds: box(0, 500000, 2000000, 1000000)}, ConformanceStatusPass},
		{"tiny overlap", types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, Bounds: box(0, 480000, 2000000, 1000000)}, ConformanceStatusPass},
		{"unknown geometry", types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody}, ConformanceStatusPass},
		{"invalid negative extent", types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, Bounds: box(0, 0, -100000, 1000000)}, ConformanceStatusPass},
		{"section number overlay", types.PlaceholderInfo{ID: "Section Number", Type: types.PlaceholderBody, Role: types.PlaceholderRoleSectionNumber, Bounds: box(0, 400000, 2000000, 1000000)}, ConformanceStatusPass},
		{"picture overlay", types.PlaceholderInfo{ID: "image", Type: types.PlaceholderImage, Bounds: box(0, 0, 2000000, 1000000)}, ConformanceStatusPass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkTextPlaceholderOverlap([]types.LayoutMetadata{{ID: "slideLayout2", Name: "Content", Placeholders: []types.PlaceholderInfo{title, tc.other}}})
			if len(got) != 1 || got[0].Status != tc.want {
				t.Fatalf("checks = %+v, want one %s", got, tc.want)
			}
			if tc.want == ConformanceStatusWarn && (!strings.Contains(got[0].Detail, "title") || !strings.Contains(got[0].Detail, "body")) {
				t.Errorf("warning omits overlapping slots: %+v", got[0])
			}
		})
	}
}

func TestMaterialPlaceholderOverlapDoesNotOverflow(t *testing.T) {
	a := types.BoundingBox{X: int64(^uint64(0)>>1) - 2000000, Y: 0, Width: 1000000, Height: 1000000}
	b := types.BoundingBox{X: 0, Y: 0, Width: 1000000, Height: 1000000}
	if materialPlaceholderOverlap(a, b) {
		t.Fatal("far-apart large-coordinate rectangles overlap")
	}
}

func TestCheckImageTitleLayerOrder(t *testing.T) {
	box := types.BoundingBox{X: 0, Y: 0, Width: 2000000, Height: 1000000}
	title := types.PlaceholderInfo{ID: "title", Type: types.PlaceholderTitle, Bounds: box}
	image := types.PlaceholderInfo{ID: "image", Type: types.PlaceholderImage, Bounds: box}
	for _, tc := range []struct {
		name  string
		slots []types.PlaceholderInfo
		want  ConformanceStatus
	}{
		{"image after title", []types.PlaceholderInfo{title, image}, ConformanceStatusWarn},
		{"image behind title", []types.PlaceholderInfo{image, title}, ConformanceStatusPass},
		{"image beside title", []types.PlaceholderInfo{title, {ID: "image", Type: types.PlaceholderImage, Bounds: types.BoundingBox{X: 2000000, Width: 1000000, Height: 1000000}}}, ConformanceStatusPass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkImageTitleLayerOrder([]types.LayoutMetadata{{ID: "slideLayout1", Name: "Title", Placeholders: tc.slots}})
			if len(got) != 1 || got[0].Status != tc.want {
				t.Fatalf("checks = %+v, want one %s", got, tc.want)
			}
		})
	}
}

func TestCheckConformanceReportsStructuralChecks(t *testing.T) {
	report, err := CheckConformance("../../templates/midnight-blue.pptx")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"Footer placeholder set complete or absent": false,
		"No conflicting structural tags":            false,
		"Text placeholders do not overlap":          false,
		"Image does not cover title":                false,
	}
	for _, check := range report.Checks {
		if _, ok := want[check.Check]; ok {
			want[check.Check] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("template-check omitted %q", name)
		}
	}
}
