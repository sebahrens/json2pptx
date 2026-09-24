package template

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCheckContentTitleBodyHierarchy(t *testing.T) {
	title := func(size int) types.PlaceholderInfo {
		return types.PlaceholderInfo{ID: "title", Type: types.PlaceholderTitle, FontSize: size,
			Bounds: types.BoundingBox{X: 0, Y: 0, Width: 4000000, Height: 500000}}
	}
	body := func(id string, x int64, size int) types.PlaceholderInfo {
		return types.PlaceholderInfo{ID: id, Type: types.PlaceholderBody, FontSize: size,
			Bounds: types.BoundingBox{X: x, Y: 1000000, Width: 2000000, Height: 3000000}}
	}
	tests := []struct {
		name  string
		shape []types.PlaceholderInfo
		want  bool
	}{
		{"body larger", []types.PlaceholderInfo{title(2000), body("body", 0, 2800)}, true},
		{"body equal", []types.PlaceholderInfo{title(2000), body("body", 0, 2000)}, true},
		{"title larger", []types.PlaceholderInfo{title(2800), body("body", 0, 2000)}, false},
		{"unknown body size", []types.PlaceholderInfo{title(2000), body("body", 0, 0)}, false},
		{"unknown title size", []types.PlaceholderInfo{title(0), body("body", 0, 2800)}, false},
		{"two content", []types.PlaceholderInfo{title(2000), body("left", 0, 1800), body("right", 3000000, 2400)}, true},
		{"smaller visible title matters", []types.PlaceholderInfo{title(2000), title(3200), body("body", 0, 2400)}, true},
		{"hidden title ignored", []types.PlaceholderInfo{title(2000), {ID: "hidden", Type: types.PlaceholderTitle, FontSize: 4000}, body("body", 0, 2400)}, true},
		{"hidden body ignored", []types.PlaceholderInfo{title(2000), body("body", 0, 1800), {ID: "hidden", Type: types.PlaceholderBody, FontSize: 2800}}, false},
		{"title only", []types.PlaceholderInfo{title(2000)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkContentTitleBodyHierarchy([]types.LayoutMetadata{{ID: "slideLayout2", Name: "One Content", Placeholders: tc.shape}})
			if (len(got) > 0) != tc.want {
				t.Fatalf("warning=%v, want %v: %+v", len(got) > 0, tc.want, got)
			}
			if tc.want && (len(got) != 1 || got[0].Status != ConformanceStatusWarn || !strings.Contains(got[0].Detail, "slideLayout2")) {
				t.Errorf("warning lacks layout identity or status: %+v", got)
			}
		})
	}
}

func TestRepairedContentLayoutsHaveVisibleHierarchy(t *testing.T) {
	tests := []struct {
		file, layoutID string
		minTitle       int
		maxBody        int
	}{
		{"blue-corporate.pptx", "slideLayout2", 2800, 2000},
		{"blue-corporate.pptx", "slideLayout5", 2800, 1800},
		{"business-template.pptx", "slideLayout3", 3200, 2200},
	}
	for _, tc := range tests {
		t.Run(tc.file+"/"+tc.layoutID, func(t *testing.T) {
			reader, err := OpenTemplate(filepath.Join("..", "..", "templates", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			layouts, err := ParseLayouts(reader)
			if err != nil {
				t.Fatal(err)
			}
			for _, layout := range layouts {
				if layout.ID != tc.layoutID {
					continue
				}
				var title, body *types.PlaceholderInfo
				for i := range layout.Placeholders {
					ph := &layout.Placeholders[i]
					if ph.Type == types.PlaceholderTitle {
						title = ph
					}
					if ph.Type == types.PlaceholderBody && (body == nil || ph.FontSize > body.FontSize) {
						body = ph
					}
				}
				if title == nil || body == nil {
					t.Fatalf("layout lacks title/body placeholders: %+v", layout.Placeholders)
				}
				if title.FontSize < tc.minTitle || body.FontSize > tc.maxBody || title.FontSize <= body.FontSize {
					t.Errorf("title=%d body=%d hundredths pt; want title >=%d, body <=%d and title > body", title.FontSize, body.FontSize, tc.minTitle, tc.maxBody)
				}
				if title.Bounds.Y+title.Bounds.Height >= body.Bounds.Y {
					t.Errorf("title box overlaps body: title=%+v body=%+v", title.Bounds, body.Bounds)
				}
				if tc.file == "business-template.pptx" {
					xml, err := reader.ReadFile("ppt/slideLayouts/slideLayout3.xml")
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(xml), `<a:lvl2pPr><a:defRPr sz="2000"/></a:lvl2pPr>`) {
						t.Error("nested body text still inherits the master 28pt size")
					}
				}
				return
			}
			t.Fatalf("layout %q not found", tc.layoutID)
		})
	}
}
