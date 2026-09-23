package template

import "testing"

func TestParseDecorRegionsKeepsOnlyVisibleNonPlaceholders(t *testing.T) {
	data := []byte(`<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr name="opaque"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="10" y="20"/><a:ext cx="30" cy="40"/></a:xfrm><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></p:spPr></p:sp>
<p:sp><p:nvSpPr><p:cNvPr name="transparent"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="10" y="20"/><a:ext cx="30" cy="40"/></a:xfrm><a:solidFill><a:srgbClr val="FF0000"><a:alpha val="50000"/></a:srgbClr></a:solidFill></p:spPr></p:sp>
<p:sp><p:nvSpPr><p:cNvPr name="placeholder"/><p:nvPr><p:ph type="body"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="10" y="20"/><a:ext cx="30" cy="40"/></a:xfrm><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></p:spPr></p:sp>
<p:pic><p:nvPicPr><p:cNvPr name="photo"/><p:nvPr/></p:nvPicPr><p:spPr><a:xfrm><a:off x="50" y="60"/><a:ext cx="70" cy="80"/></a:xfrm></p:spPr></p:pic>
</p:spTree></p:cSld></p:sldMaster>`)
	regions, err := parseDecorRegions(data, "master")
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) != 2 || regions[0].Name != "opaque" || regions[1].Name != "photo" || regions[1].X != 50 || regions[1].Width != 70 {
		t.Fatalf("decor regions = %+v", regions)
	}
}
