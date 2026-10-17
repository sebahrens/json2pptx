package generator

import (
	"strings"
	"testing"
)

func TestLayeredBasePictureDeferredBelowNativeShapes(t *testing.T) {
	ctx := &singlePassContext{MediaContext: MediaContext{slideRelUpdates: map[int][]mediaRel{}}}
	original := []byte(`<p:sld><p:cSld><p:spTree><p:nvGrpSpPr/><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="200" name="Scrim"/></p:nvSpPr><p:spPr/></p:sp><p:sp><p:nvSpPr><p:cNvPr id="201" name="Headline"/></p:nvSpPr><p:spPr/></p:sp></p:spTree></p:cSld></p:sld>`)
	media := []mediaRel{{behindRawShapes: true, relID: "rId2", description: "base image", extentCX: 1000000, extentCY: 1000000, placeholderIdx: -1}}
	early, err := ctx.insertMediaPics(1, original, media)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(early), "<p:pic>") {
		t.Fatal("base picture inserted before native shape phase")
	}
	final, err := ctx.insertMediaPicsPhase(1, early, ctx.slideRelUpdates[1], true)
	if err != nil {
		t.Fatal(err)
	}
	xml := string(final)
	pic := strings.Index(xml, "<p:pic>")
	scrim := strings.Index(xml, `name="Scrim"`)
	headline := strings.Index(xml, `name="Headline"`)
	if pic < 0 || pic > scrim || scrim > headline {
		t.Fatalf("wrong picture/scrim/headline order: %s", xml)
	}
}
