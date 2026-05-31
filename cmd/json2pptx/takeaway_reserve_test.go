package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// TestReserveTakeawayBand_LowersFooterTop verifies that a slide carrying a
// takeaway headline pulls the content zone's FooterTop above the fixed takeaway
// band so full-area patterns (e.g. kpi-Nup) leave a clear gap instead of
// crowding the takeaway text (go-slide-creator-rdtn).
func TestReserveTakeawayBand_LowersFooterTop(t *testing.T) {
	// FooterTop below the band: the default no-footer min-margin line.
	footerTop := shapegrid.DefaultSlideHeightEMU - shapegrid.MinBottomMarginEMU // 6492240
	if footerTop <= generator.TakeawayBandTopEMU {
		t.Fatalf("test precondition: footerTop %d should sit below band top %d", footerTop, generator.TakeawayBandTopEMU)
	}
	zone := &shapegrid.ContentZone{
		TitleBottom: 1000000,
		FooterTop:   footerTop,
		LeftMargin:  457200,
		RightEdge:   11734800,
		SlideWidth:  shapegrid.DefaultSlideWidthEMU,
		SlideHeight: shapegrid.DefaultSlideHeightEMU,
	}
	bounds := shapegrid.DefaultBoundsFromZone(*zone, gridChromeGapPt)
	g := GridGeometry{Zone: zone, OverrideBounds: &bounds}

	out := reserveTakeawayBand(g, SlideInput{Takeaway: "Revenue grew 32% YoY"})

	if out.Zone.FooterTop != generator.TakeawayBandTopEMU {
		t.Errorf("FooterTop = %d, want %d (band top)", out.Zone.FooterTop, generator.TakeawayBandTopEMU)
	}
	// Override bounds must not extend into the band; leave the standard gap above it.
	maxBottom := generator.TakeawayBandTopEMU - int64(gridChromeGapPt*12700)
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

	out := reserveTakeawayBand(g, SlideInput{})

	if out.Zone.FooterTop != footerTop {
		t.Errorf("FooterTop = %d, want %d (unchanged)", out.Zone.FooterTop, footerTop)
	}
}

// TestReserveTakeawayBand_FooterAlreadyAboveBand verifies that a footer
// placeholder already sitting above the band is left untouched (no upward push
// that would shrink the content area unnecessarily).
func TestReserveTakeawayBand_FooterAlreadyAboveBand(t *testing.T) {
	footerTop := generator.TakeawayBandTopEMU - 200000 // already above the band
	zone := &shapegrid.ContentZone{FooterTop: footerTop}
	g := GridGeometry{Zone: zone}

	out := reserveTakeawayBand(g, SlideInput{Takeaway: "Headline"})

	if out.Zone.FooterTop != footerTop {
		t.Errorf("FooterTop = %d, want %d (unchanged)", out.Zone.FooterTop, footerTop)
	}
}

// TestReserveTakeawayBand_NilZone verifies the helper tolerates a nil zone.
func TestReserveTakeawayBand_NilZone(t *testing.T) {
	g := GridGeometry{}
	out := reserveTakeawayBand(g, SlideInput{Takeaway: "Headline"})
	if out.Zone != nil {
		t.Errorf("Zone = %+v, want nil", out.Zone)
	}
}

// TestReserveTakeawayBand_BoundsNotClampedWhenAboveBand verifies override bounds
// that already end above the reserved gap are left unchanged.
func TestReserveTakeawayBand_BoundsNotClampedWhenAboveBand(t *testing.T) {
	zone := &shapegrid.ContentZone{FooterTop: shapegrid.DefaultSlideHeightEMU - shapegrid.MinBottomMarginEMU}
	bounds := pptx.RectEmu{X: 457200, Y: 1000000, CX: 11277600, CY: 2000000} // bottom 3000000, well above band
	g := GridGeometry{Zone: zone, OverrideBounds: &bounds}

	out := reserveTakeawayBand(g, SlideInput{Takeaway: "Headline"})

	if out.OverrideBounds.CY != bounds.CY {
		t.Errorf("override bounds CY = %d, want %d (unchanged)", out.OverrideBounds.CY, bounds.CY)
	}
}
