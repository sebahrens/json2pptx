package generator

import (
	"testing"
)

// testSlideXML is a representative slide XML that exercises all namespace
// replacement paths: a: elements, p: elements, ext (both a:ext and p:ext),
// masterClrMapping, and ph elements.
const testSlideXML = `<sld><cSld><spTree><nvGrpSpPr><cNvPr id="1" name=""/><cNvGrpSpPr/><nvPr/></nvGrpSpPr><grpSpPr><xfrm><off x="0" y="0"/><ext cx="12192000" cy="6858000"/><chOff x="0" y="0"/><chExt cx="12192000" cy="6858000"/></xfrm></grpSpPr><sp><nvSpPr><cNvPr id="2" name="Title"/><cNvSpPr><spLocks noGrp="1"/></cNvSpPr><nvPr><ph xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" type="title"/></nvPr></nvSpPr><spPr><xfrm><off x="457200" y="274638"/><ext cx="8229600" cy="1143000"/></xfrm><prstGeom prst="rect"><avLst/></prstGeom><solidFill><schemeClr val="accent1"/></solidFill><ln><noFill/></ln></spPr><txBody><bodyPr wrap="square" rtlCol="0"><normAutofit/></bodyPr><lstStyle/><p><r><rPr lang="en-US" dirty="0"/><t>Sample Title</t></r><endParaRPr lang="en-US"/></p></txBody></sp><sp><nvSpPr><cNvPr id="3" name="Content"/><cNvSpPr><spLocks noGrp="1"/></cNvSpPr><nvPr><ph xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" idx="1"/></nvPr></nvSpPr><spPr><xfrm><off x="457200" y="1600200"/><ext cx="8229600" cy="4525963"/></xfrm><prstGeom prst="rect"><avLst/></prstGeom></spPr><txBody><bodyPr wrap="square" rtlCol="0"><normAutofit/></bodyPr><lstStyle><lvl1pPr><defRPr sz="2800"/></lvl1pPr><lvl2pPr><defRPr sz="2400"/></lvl2pPr><lvl3pPr><defRPr sz="2000"/></lvl3pPr></lstStyle><p><pPr lvl="0"><buChar char="•"/></pPr><r><rPr lang="en-US" dirty="0"><solidFill><srgbClr val="333333"/></solidFill></rPr><t>First bullet point</t></r></p><p><pPr lvl="1"><buNone/></pPr><r><rPr lang="en-US"/><t>Sub item</t></r></p></txBody></sp><pic><nvPicPr><cNvPr id="4" name="Picture"/><cNvPicPr/><nvPr/></nvPicPr><blipFill><blip r:embed="rId2"><lumMod val="50000"/></blip><stretch><fillRect/></stretch></blipFill><spPr><xfrm><off x="100" y="200"/><ext cx="500" cy="300"/></xfrm></spPr></pic></spTree><extLst><ext uri="{BB962C8B-B14F-4D97-AF65-F5344CB8AC3E}"><p14:creationId xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" val="12345"/></ext></extLst></cSld><clrMapOvr><masterClrMapping/></clrMapOvr></sld>`

// expectedSlideXML is the expected output after namespace fixup.
const expectedSlideXML = `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="12192000" cy="6858000"/><a:chOff x="0" y="0"/><a:chExt cx="12192000" cy="6858000"/></a:xfrm></p:grpSpPr><p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="457200" y="274638"/><a:ext cx="8229600" cy="1143000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:ln><a:noFill/></a:ln></p:spPr><p:txBody><a:bodyPr wrap="square" rtlCol="0"><a:normAutofit/></a:bodyPr><a:lstStyle/><a:p><a:r><a:rPr lang="en-US" dirty="0"/><a:t>Sample Title</a:t></a:r><a:endParaRPr lang="en-US"/></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="3" name="Content"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="457200" y="1600200"/><a:ext cx="8229600" cy="4525963"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr wrap="square" rtlCol="0"><a:normAutofit/></a:bodyPr><a:lstStyle><a:lvl1pPr><a:defRPr sz="2800"/></a:lvl1pPr><a:lvl2pPr><a:defRPr sz="2400"/></a:lvl2pPr><a:lvl3pPr><a:defRPr sz="2000"/></a:lvl3pPr></a:lstStyle><a:p><a:pPr lvl="0"><a:buChar char="•"/></a:pPr><a:r><a:rPr lang="en-US" dirty="0"><a:solidFill><a:srgbClr val="333333"/></a:solidFill></a:rPr><a:t>First bullet point</a:t></a:r></a:p><a:p><a:pPr lvl="1"><a:buNone/></a:pPr><a:r><a:rPr lang="en-US"/><a:t>Sub item</a:t></a:r></a:p></p:txBody></p:sp><p:pic><p:nvPicPr><p:cNvPr id="4" name="Picture"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"><a:lumMod val="50000"/></a:blip><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="500" cy="300"/></a:xfrm></p:spPr></p:pic></p:spTree><p:extLst><p:ext uri="{BB962C8B-B14F-4D97-AF65-F5344CB8AC3E}"><p14:creationId xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" val="12345"/></p:ext></p:extLst></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`

func TestFixOOXMLNamespaces(t *testing.T) {
	got := string(fixOOXMLNamespaces([]byte(testSlideXML)))
	if got != expectedSlideXML {
		// Find first difference for debugging
		minLen := len(got)
		if len(expectedSlideXML) < minLen {
			minLen = len(expectedSlideXML)
		}
		for i := 0; i < minLen; i++ {
			if got[i] != expectedSlideXML[i] {
				start := i - 40
				if start < 0 {
					start = 0
				}
				end := i + 40
				if end > minLen {
					end = minLen
				}
				t.Fatalf("mismatch at byte %d:\n  got:  ...%s...\n  want: ...%s...",
					i, got[start:end], expectedSlideXML[start:end])
				return
			}
		}
		if len(got) != len(expectedSlideXML) {
			t.Fatalf("length mismatch: got %d, want %d", len(got), len(expectedSlideXML))
		}
	}
}

func BenchmarkFixOOXMLNamespaces(b *testing.B) {
	data := []byte(testSlideXML)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		fixOOXMLNamespaces(data)
	}
}
