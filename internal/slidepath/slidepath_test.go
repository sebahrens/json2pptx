package slidepath

import "testing"

func TestSlideIndex(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/slides/0/content/body", 0},
		{"/slides/3/shape_grid/rows/1/cells/0", 3},
		{"/slides/12/content/1", 12},
		{"/slides/0", 0},
		{"", -1},
		{"/slides/", -1},
		{"/slides/abc", -1},
		{"slides/0", -1}, // missing leading slash
	}
	for _, tt := range tests {
		got := SlideIndex(tt.path)
		if got != tt.want {
			t.Errorf("SlideIndex(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestPathBuilders(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Slide", Slide(2), "/slides/2"},
		{"Content", Content(0, "body"), "/slides/0/content/body"},
		{"ContentIndex", ContentIndex(1, 3), "/slides/1/content/3"},
		{"ContentField", ContentField(0, 1, "placeholder_id"), "/slides/0/content/1/placeholder_id"},
		{"SlideField", SlideField(0, "layout_id"), "/slides/0/layout_id"},
		{"ShapeGrid", ShapeGrid(1), "/slides/1/shape_grid"},
		{"GridCell", GridCell(2, 1, 0), "/slides/2/shape_grid/rows/1/cells/0"},
		{"GridCellField", GridCellField(0, 0, 0, "shape/fill"), "/slides/0/shape_grid/rows/0/cells/0/shape/fill"},
		{"GridRow", GridRow(0, 2), "/slides/0/shape_grid/rows/2"},
		{"GridRowRange", GridRowRange(0, 1, 2), "/slides/0/shape_grid/rows/1:2"},
		{"TableHeader", TableHeader("/slides/0/content/0", 2), "/slides/0/content/0/headers/2"},
		{"TableCell", TableCell("/slides/0/content/0", 3, 1), "/slides/0/content/0/rows/3/1"},
		{"Join", Join("/slides/0/shape_grid/rows/0/cells/0", "table"), "/slides/0/shape_grid/rows/0/cells/0/table"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		path, prefix string
		want         bool
	}{
		{"/slides/0/content/body", "/slides/0", true},
		{"/slides/0/content/body", "/slides/0/content", true},
		{"/slides/0/content/body", "/slides/0/content/body", true},
		{"/slides/0/content/body", "/slides/1", false},
		{"/slides/10", "/slides/1", false}, // must not match partial segment
	}
	for _, tt := range tests {
		got := HasPrefix(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("HasPrefix(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestParseGridCell(t *testing.T) {
	si, ri, ci, ok := ParseGridCell("/slides/0/shape_grid/rows/1/cells/2")
	if !ok || si != 0 || ri != 1 || ci != 2 {
		t.Errorf("ParseGridCell: got (%d, %d, %d, %v), want (0, 1, 2, true)", si, ri, ci, ok)
	}

	_, _, _, ok = ParseGridCell("/slides/0/content/body")
	if ok {
		t.Error("ParseGridCell should return ok=false for non-grid-cell path")
	}
}
