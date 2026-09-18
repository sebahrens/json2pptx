package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// reserveTestLayouts is a One Content layout with a 16:9 body column and
// master footer placeholders at y=6356350, mirroring the bundled templates.
func reserveTestLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{{
		ID:            "slideLayout2",
		Name:          "One Content",
		CanonicalType: types.CanonicalLayoutOneContent,
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 838200, Y: 365125, Width: 10515600, Height: 1325563}},
			{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Bounds: types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}},
		},
		FooterRegions: []types.ChromeRegion{
			{Type: "dt", X: 838200, Y: 6356350, Width: 2743200, Height: 365125},
			{Type: "ftr", X: 4038600, Y: 6356350, Width: 4114800, Height: 365125},
		},
	}}
}

// TestReserveTakeawayBand_LowersFooterTop verifies that a slide carrying a
// takeaway headline pulls the content zone's FooterTop above the layout-derived
// takeaway band so full-area patterns (e.g. kpi-Nup) leave a clear gap instead
// of crowding the takeaway text (go-slide-creator-rdtn, go-slide-creator-7m9v).
func TestReserveTakeawayBand_LowersFooterTop(t *testing.T) {
	layouts := reserveTestLayouts()
	footerTop := shapegrid.DefaultSlideHeightEMU - shapegrid.MinBottomMarginEMU // 6492240
	zone := &shapegrid.ContentZone{
		TitleBottom: 1000000,
		FooterTop:   footerTop,
		LeftMargin:  838200,
		RightEdge:   11353800,
		SlideWidth:  shapegrid.DefaultSlideWidthEMU,
		SlideHeight: shapegrid.DefaultSlideHeightEMU,
	}
	bounds := shapegrid.DefaultBoundsFromZone(*zone, gridChromeGapPt)
	g := GridGeometry{Zone: zone, OverrideBounds: &bounds}
	slide := SlideInput{LayoutID: "slideLayout2", Takeaway: "Revenue grew 32% YoY"}

	out := reserveTakeawayBand(g, slide, layouts)

	frame := template.ResolveChromeFrame(&layouts[0], &layouts[0], zone.SlideWidth, zone.SlideHeight, true, false)
	bandTop := frame.Content.Bottom()
	if out.Zone.FooterTop != bandTop {
		t.Errorf("FooterTop = %d, want %d (layout-derived content bottom)", out.Zone.FooterTop, bandTop)
	}
	if frame.Takeaway.Bottom() > 6356350 {
		t.Errorf("takeaway band bottom %d overlaps the footer placeholders at 6356350", frame.Takeaway.Bottom())
	}
	// Override bounds must not extend into the band; leave the standard gap above it.
	maxBottom := bandTop - int64(gridChromeGapPt*12700)
	if got := out.OverrideBounds.Y + out.OverrideBounds.CY; got > maxBottom {
		t.Errorf("override bounds bottom = %d, want <= %d", got, maxBottom)
	}
	// The original zone must not be mutated (reservation works on a copy).
	if zone.FooterTop != footerTop {
		t.Errorf("input zone was mutated: FooterTop = %d, want %d", zone.FooterTop, footerTop)
	}
}

// TestReserveTakeawayBand_NoTakeaway verifies the zone is untouched when the
// slide has no takeaway headline.
func TestReserveTakeawayBand_NoTakeaway(t *testing.T) {
	footerTop := shapegrid.DefaultSlideHeightEMU - shapegrid.MinBottomMarginEMU
	zone := &shapegrid.ContentZone{FooterTop: footerTop}
	g := GridGeometry{Zone: zone}

	out := reserveTakeawayBand(g, SlideInput{}, reserveTestLayouts())

	if out.Zone.FooterTop != footerTop {
		t.Errorf("FooterTop = %d, want %d (unchanged)", out.Zone.FooterTop, footerTop)
	}
}

// TestReserveTakeawayBand_FooterAlreadyAboveBand verifies that a footer
// already sitting above the band is left untouched (no upward push that would
// shrink the content area unnecessarily).
func TestReserveTakeawayBand_FooterAlreadyAboveBand(t *testing.T) {
	layouts := reserveTestLayouts()
	frame := template.ResolveChromeFrame(&layouts[0], nil, 0, 0, true, false)
	footerTop := frame.Content.Bottom() - 200000 // already above the band
	zone := &shapegrid.ContentZone{FooterTop: footerTop, SlideWidth: shapegrid.DefaultSlideWidthEMU, SlideHeight: shapegrid.DefaultSlideHeightEMU}
	g := GridGeometry{Zone: zone}

	out := reserveTakeawayBand(g, SlideInput{LayoutID: "slideLayout2", Takeaway: "Headline"}, layouts)

	if out.Zone.FooterTop != footerTop {
		t.Errorf("FooterTop = %d, want %d (unchanged)", out.Zone.FooterTop, footerTop)
	}
}

// TestReserveTakeawayBand_NilZone verifies the helper tolerates a nil zone.
func TestReserveTakeawayBand_NilZone(t *testing.T) {
	g := GridGeometry{}
	out := reserveTakeawayBand(g, SlideInput{Takeaway: "Headline"}, reserveTestLayouts())
	if out.Zone != nil {
		t.Errorf("Zone = %+v, want nil", out.Zone)
	}
}

// TestReserveTakeawayBand_BoundsNotClampedWhenAboveBand verifies override bounds
// that already end above the reserved gap are left unchanged.
func TestReserveTakeawayBand_BoundsNotClampedWhenAboveBand(t *testing.T) {
	zone := &shapegrid.ContentZone{FooterTop: shapegrid.DefaultSlideHeightEMU - shapegrid.MinBottomMarginEMU, SlideWidth: shapegrid.DefaultSlideWidthEMU, SlideHeight: shapegrid.DefaultSlideHeightEMU}
	bounds := pptx.RectEmu{X: 838200, Y: 1000000, CX: 10515600, CY: 2000000} // bottom 3000000, well above band
	g := GridGeometry{Zone: zone, OverrideBounds: &bounds}

	out := reserveTakeawayBand(g, SlideInput{LayoutID: "slideLayout2", Takeaway: "Headline"}, reserveTestLayouts())

	if out.OverrideBounds.CY != bounds.CY {
		t.Errorf("override bounds CY = %d, want %d (unchanged)", out.OverrideBounds.CY, bounds.CY)
	}
}

// TestCheckChromeBandFit verifies preflight reports chrome_band_no_fit when the
// layout-derived band stack cannot be placed without overlapping chrome, and
// stays silent when it fits (go-slide-creator-7m9v).
func TestCheckChromeBandFit(t *testing.T) {
	layouts := reserveTestLayouts()
	cramped := types.LayoutMetadata{
		ID: "slideLayout9",
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 838200, Y: 300000, Width: 10515600, Height: 4000000}},
			{ID: "body", Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 838200, Y: 3900000, Width: 10515600, Height: 1500000}},
		},
		FooterRegions: []types.ChromeRegion{{Type: "ftr", X: 838200, Y: 5600000, Width: 4000000, Height: 300000}},
	}
	layouts = append(layouts, cramped)

	if f := checkChromeBandFit(&SlideInput{LayoutID: "slideLayout2", Takeaway: "Fits"}, 0, layouts, 12192000, 6858000); f != nil {
		t.Fatalf("unexpected finding on a roomy layout: %+v", f)
	}
	if f := checkChromeBandFit(&SlideInput{LayoutID: "slideLayout9"}, 0, layouts, 12192000, 6858000); f != nil {
		t.Fatal("no takeaway/source must not emit a finding")
	}
	f := checkChromeBandFit(&SlideInput{LayoutID: "slideLayout9", Source: "Annual report"}, 3, layouts, 12192000, 6858000)
	if f == nil || f.Code != "chrome_band_no_fit" || f.Action != "review" {
		t.Fatalf("expected chrome_band_no_fit review finding, got %+v", f)
	}
	if f.Path != "/slides/3/source" {
		t.Errorf("path = %q", f.Path)
	}
	if f.Fix == nil || f.Fix.Kind != "swap_layout" || f.Fix.Params["layout_id"] != "slideLayout2" {
		t.Errorf("fix = %+v, want swap_layout to the One Content layout", f.Fix)
	}
}
