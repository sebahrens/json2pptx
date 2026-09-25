package template

import (
	"encoding/xml"
	"testing"
)

func TestModernSubtitleRetainsGradientStopsForPreflight(t *testing.T) {
	layouts := parseBundledLayouts(t, "modern.pptx")
	subtitle := placeholderOn(t, layouts, "slideLayout2", "subtitle")
	stops := subtitle.FillStops
	if len(stops) != 3 {
		t.Fatalf("modern subtitle gradient stops = %+v, want three", stops)
	}
	wantRefs := []string{"accent5", "accent1", "accent2"}
	wantPositions := []int{0, 50000, 100000}
	for i, stop := range stops {
		if stop.Ref != wantRefs[i] || stop.Position != wantPositions[i] {
			t.Errorf("stop %d = %+v, want %s at %d", i, stop, wantRefs[i], wantPositions[i])
		}
	}
	if !stops[2].Mods.HasLumMod || stops[2].Mods.LumMod != 97000 || stops[2].Mods.LumOff != 3000 {
		t.Errorf("orange stop lost its luminance modifiers: %+v", stops[2].Mods)
	}
}

func TestPlaceholderSolidFillRespectsColorMapAndNoFill(t *testing.T) {
	var shape shapeXML
	if err := xml.Unmarshal([]byte(`<p:sp xmlns:p="p" xmlns:a="a"><p:spPr><a:solidFill><a:schemeClr val="bg1"/></a:solidFill></p:spPr></p:sp>`), &shape); err != nil {
		t.Fatal(err)
	}
	stops := placeholderFillStops(&shape, map[string]string{"bg1": "dk1"})
	if len(stops) != 1 || stops[0].Ref != "dk1" {
		t.Fatalf("solid fill with color-map override = %+v, want dk1", stops)
	}
	if got := placeholderFillStops(&shapeXML{}, nil); len(got) != 0 {
		t.Errorf("unfilled placeholder has stops: %+v", got)
	}
}

func TestUnsupportedGradientStopIsRetainedAsUnresolved(t *testing.T) {
	var shape shapeXML
	if err := xml.Unmarshal([]byte(`<p:sp xmlns:p="p" xmlns:a="a"><p:spPr><a:gradFill><a:gsLst><a:gs pos="0"><a:sysClr val="window"/></a:gs></a:gsLst></a:gradFill></p:spPr></p:sp>`), &shape); err != nil {
		t.Fatal(err)
	}
	stops := placeholderFillStops(&shape, nil)
	if len(stops) != 1 || stops[0].Ref != "" {
		t.Fatalf("unsupported stop should remain detectable, got %+v", stops)
	}
}
