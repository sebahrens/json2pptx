package diagnostics

import "testing"

// JSON-pointer paths ("/slides/2/…") locate the slide too; contrast_predicted
// and the other pointer-path fit findings had no where.slide
// (go-slide-creator-6p9mm).
func TestWhereFromPointerPath(t *testing.T) {
	for path, want := range map[string]int{"/slides/2/shape_grid/rows/0": 2, "/slides/7": 7, "slides[4].title": 4} {
		w := whereFromPath(path)
		if w.Slide == nil || *w.Slide != want {
			t.Errorf("%s: where.slide = %v, want %d", path, w.Slide, want)
		}
	}
	for _, path := range []string{"/meta/template", "/slides/x/title", "/slides/"} {
		if w := whereFromPath(path); w.Slide != nil {
			t.Errorf("%s: unexpected where.slide %d", path, *w.Slide)
		}
	}
}
