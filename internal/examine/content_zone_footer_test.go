package examine

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// zoneTestLayout is a layout with a title and NO footer placeholder of its own —
// the case that produced the bug: the footer is inherited from the master, so
// the layout's own placeholders say nothing about it.
func zoneTestLayout(id string, footerRegions []types.ChromeRegion) types.LayoutMetadata {
	return types.LayoutMetadata{
		ID:   id,
		Name: id,
		Placeholders: []types.PlaceholderInfo{{
			ID:     "title",
			Type:   types.PlaceholderTitle,
			Bounds: types.BoundingBox{X: 838200, Y: 365125, Width: 10515600, Height: 1325563},
		}},
		FooterRegions: footerRegions,
	}
}

// TestContentZoneStopsAboveTheFooter is the go-slide-creator-p41d6 acceptance
// test: examine_template reported a content zone that reached 0.17in INTO the
// footer band, because the zone was derived from the layout's own placeholders
// while the footer came from the master. An agent computing bounds from the
// zone put content under the footer text.
func TestContentZoneStopsAboveTheFooter(t *testing.T) {
	const slideW, slideH = 12192000, 6858000
	const footerTop = 6356350
	layout := zoneTestLayout("slideLayout2", []types.ChromeRegion{
		{Type: "ftr", X: 838200, Y: footerTop, Width: 4000000, Height: 365125},
		{Type: "sldNum", X: 10000000, Y: footerTop, Width: 1000000, Height: 365125},
	})

	report := BuildReport(Inputs{
		SlideWidthEMU:  slideW,
		SlideHeightEMU: slideH,
		Layouts:        []types.LayoutMetadata{layout},
		Profile: &template.TemplateProfile{
			Layouts: []types.LayoutMetadata{layout},
			Geometry: []template.LayoutGeometry{{
				LayoutID: "slideLayout2",
				Frame:    template.ChromeFrame{FooterTop: footerTop, HasFooter: true},
			}},
		},
	})

	if len(report.Layouts) != 1 {
		t.Fatalf("got %d layouts, want 1", len(report.Layouts))
	}
	zone := report.Layouts[0].ContentZone
	if zone.BottomEMU > footerTop {
		t.Errorf("content_zone.bottom_emu %d is below footer_top_emu %d — an agent laying out to this zone writes into the footer", zone.BottomEMU, footerTop)
	}
	if gap := footerTop - zone.BottomEMU; gap < footerClearanceEMU {
		t.Errorf("clearance above the footer is %d EMU, want at least %d", gap, footerClearanceEMU)
	}
	if zone.BottomEMU <= zone.TopEMU {
		t.Errorf("clamping collapsed the zone: top %d bottom %d", zone.TopEMU, zone.BottomEMU)
	}
}

// TestContentZoneUntouchedWithoutAFooter pins that the clamp only fires when
// there IS a footer: a layout with none keeps the zone its placeholders imply.
func TestContentZoneUntouchedWithoutAFooter(t *testing.T) {
	const slideW, slideH = 12192000, 6858000
	layout := zoneTestLayout("slideLayout6", nil)

	withProfile := BuildReport(Inputs{
		SlideWidthEMU: slideW, SlideHeightEMU: slideH,
		Layouts: []types.LayoutMetadata{layout},
		Profile: &template.TemplateProfile{
			Layouts:  []types.LayoutMetadata{layout},
			Geometry: []template.LayoutGeometry{{LayoutID: "slideLayout6", Frame: template.ChromeFrame{HasFooter: false}}},
		},
	})
	bare := BuildReport(Inputs{
		SlideWidthEMU: slideW, SlideHeightEMU: slideH,
		Layouts: []types.LayoutMetadata{layout},
	})

	if withProfile.Layouts[0].ContentZone != bare.Layouts[0].ContentZone {
		t.Errorf("a footerless layout's zone changed: %+v vs %+v",
			withProfile.Layouts[0].ContentZone, bare.Layouts[0].ContentZone)
	}
}
