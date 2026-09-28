package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

func titleBandLayout(anchor string) types.LayoutMetadata {
	return types.LayoutMetadata{
		ID: "content-t",
		Placeholders: []types.PlaceholderInfo{{
			ID: "title", Type: types.PlaceholderTitle, Anchor: anchor,
			Bounds:   types.BoundingBox{X: 397666, Y: 402336, Width: 11421056, Height: 1295399},
			FontSize: 2800, FontFamily: "Arial", LineSpacingPct: 90,
		}},
	}
}

func titleBandSlide(title string) SlideInput {
	return SlideInput{LayoutID: "content-t", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}}
}

// go-slide-creator-pymy7: a tall top-anchored title box must not reserve its
// empty lower part — content starts a fixed gap below the title's text.
func TestReserveMeasuredTitle(t *testing.T) {
	boxBottom := int64(402336 + 1295399)
	zone := func() GridGeometry {
		return GridGeometry{LayoutID: "content-t", Zone: &shapegrid.ContentZone{TitleBottom: boxBottom, FooterTop: 6400000, SlideWidth: 12192000, SlideHeight: 6858000}}
	}

	layouts := []types.LayoutMetadata{titleBandLayout("t")}
	short := reserveMeasuredTitle(zone(), titleBandSlide("KPI 3-Up"), layouts)
	if short.Zone.TitleBottom >= boxBottom {
		t.Fatalf("one-line top-anchored title kept the box edge %d", short.Zone.TitleBottom)
	}
	// One 28pt line at 90% spacing is ~30pt; the band must stay well above
	// the box bottom but below the text.
	if short.Zone.TitleBottom < 402336+int64(28*12700) {
		t.Errorf("title bottom %d cuts into a one-line title", short.Zone.TitleBottom)
	}
	long := reserveMeasuredTitle(zone(), titleBandSlide("A deliberately long action title that states the full so-what of this slide and wraps onto a second line"), layouts)
	if long.Zone.TitleBottom <= short.Zone.TitleBottom {
		t.Errorf("two-line title bottom %d not below one-line %d", long.Zone.TitleBottom, short.Zone.TitleBottom)
	}

	for _, anchor := range []string{"ctr", "b", ""} {
		g := reserveMeasuredTitle(zone(), titleBandSlide("KPI 3-Up"), []types.LayoutMetadata{titleBandLayout(anchor)})
		if g.Zone.TitleBottom != boxBottom {
			t.Errorf("anchor %q moved the title edge to %d", anchor, g.Zone.TitleBottom)
		}
	}
	if g := reserveMeasuredTitle(zone(), SlideInput{LayoutID: "content-t"}, layouts); g.Zone.TitleBottom != boxBottom {
		t.Errorf("slide without title text moved the title edge to %d", g.Zone.TitleBottom)
	}
}
