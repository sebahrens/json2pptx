package generator

import (
	"strings"
	"testing"
)

// go-slide-creator-1iy0x: chrome.page_numbers.enabled:false cleared the page
// number format, but an empty format already meant "plain slide number", so
// disabling did not disable. HidePageNumber is the explicit switch.
func TestGenerateFooterShapes_HidePageNumber(t *testing.T) {
	positions := map[string]*transformXML{
		"type:dt":     {Offset: offsetXML{X: 457200, Y: 6492875}, Extent: extentXML{CX: 3200400, CY: 365125}},
		"type:ftr":    {Offset: offsetXML{X: 4038600, Y: 6492875}, Extent: extentXML{CX: 4114800, CY: 365125}},
		"type:sldNum": {Offset: offsetXML{X: 8610600, Y: 6492875}, Extent: extentXML{CX: 3200400, CY: 365125}},
	}
	got := generateFooterShapes(positions, &FooterConfig{Enabled: true, LeftText: "Acme Corp", HidePageNumber: true}, 100, "", "", 0)
	if strings.Contains(got, "Footer Right") || strings.Contains(got, `type="slidenum"`) {
		t.Errorf("HidePageNumber still emitted a slide number:\n%s", got)
	}
	if !strings.Contains(got, "Acme Corp") {
		t.Error("HidePageNumber must keep the left footer text")
	}
}
