package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func edgeArtLayout(decor ...types.DecorRegion) *types.LayoutMetadata {
	return &types.LayoutMetadata{
		ID: "edge",
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 838200, Y: 365125, Width: 10515600, Height: 1325563}},
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}},
		},
		DecorRegions: decor,
	}
}

// TestChromeFrameClearsEdgeArt pins the edge-art rule (go-slide-creator-oa0ru):
// full-height art wholly inside one side margin pushes the content column in
// so it clears the art by the undecorated side's margin; art on both sides,
// short accents, and a layout without art leave the column alone.
func TestChromeFrameClearsEdgeArt(t *testing.T) {
	const w, h = 12192000, 6858000
	bar := types.DecorRegion{X: 0, Y: 0, Width: 144000, Height: h}
	stripe := types.DecorRegion{X: 324000, Y: 0, Width: 72000, Height: h}
	rightStripe := types.DecorRegion{X: w - 396000, Y: 0, Width: 72000, Height: h}
	shortAccent := types.DecorRegion{X: 0, Y: 0, Width: 144000, Height: 600000}
	cases := []struct {
		name              string
		decor             []types.DecorRegion
		wantLeft, wantRgt int64
		wantInset         bool
	}{
		{"no art", nil, 838200, 11353800, false},
		{"left bar and stripe", []types.DecorRegion{bar, stripe}, 396000 + 838200, 11353800, true},
		{"right stripe", []types.DecorRegion{rightStripe}, 838200, w - 396000 - 838200, true},
		{"both sides", []types.DecorRegion{bar, rightStripe}, 838200, 11353800, false},
		{"short accent", []types.DecorRegion{shortAccent}, 838200, 11353800, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := ResolveChromeFrame(edgeArtLayout(tc.decor...), nil, w, h, true, true)
			if f.Content.X != tc.wantLeft || f.Content.X+f.Content.CX != tc.wantRgt {
				t.Errorf("content span = %d..%d, want %d..%d", f.Content.X, f.Content.X+f.Content.CX, tc.wantLeft, tc.wantRgt)
			}
			if f.Source.X != f.Content.X || f.Takeaway.X != f.Content.X {
				t.Errorf("bands at x=%d/%d, want the content edge %d", f.Source.X, f.Takeaway.X, f.Content.X)
			}
			if f.SideDecorInset != tc.wantInset {
				t.Errorf("SideDecorInset = %v, want %v", f.SideDecorInset, tc.wantInset)
			}
		})
	}
}

// TestChromeFrameSourceZoneClearance pins the source zone's place in the band
// stack (go-slide-creator-cuszt): directly above the footer, with at least
// 12pt between it and the takeaway band (or the content) above.
func TestChromeFrameSourceZoneClearance(t *testing.T) {
	layout := edgeArtLayout()
	layout.FooterRegions = []types.ChromeRegion{{Type: "ftr", X: 4038600, Y: 6356350, Width: 4114800, Height: 365125}}
	for _, takeaway := range []bool{true, false} {
		f := ResolveChromeFrame(layout, nil, 12192000, 6858000, takeaway, true)
		if !f.Fits {
			t.Fatalf("takeaway=%v: frame does not fit", takeaway)
		}
		if f.Source.Bottom() > f.FooterTop {
			t.Errorf("takeaway=%v: source zone %+v runs into the footer at %d", takeaway, f.Source, f.FooterTop)
		}
		above := f.Content.Bottom()
		if takeaway {
			above = f.Takeaway.Bottom()
		}
		if gap := f.Source.Y - above; gap < 12*12700 {
			t.Errorf("takeaway=%v: gap above the source zone = %d EMU, want >= 12pt", takeaway, gap)
		}
	}
}
