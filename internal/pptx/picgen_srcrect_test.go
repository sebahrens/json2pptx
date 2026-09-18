package pptx

import (
	"strings"
	"testing"
)

func TestGeneratePic_SrcRect(t *testing.T) {
	xml, err := GeneratePic(PicOptions{ID: 7, PNGRelID: "rId3", ExtentCX: 100, ExtentCY: 100, SrcRect: &SrcRect{L: 16666, R: 16666}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	crop := `<a:srcRect l="16666" t="0" r="16666" b="0"/>`
	if !strings.Contains(s, crop) {
		t.Fatalf("missing srcRect crop: %s", s)
	}
	// CT_BlipFillProperties order: blip, srcRect, stretch.
	if !(strings.Index(s, "<a:blip ") < strings.Index(s, crop) && strings.Index(s, crop) < strings.Index(s, "<a:stretch>")) {
		t.Errorf("srcRect must sit between a:blip and a:stretch: %s", s)
	}

	for _, r := range []*SrcRect{nil, {}} {
		plain, err := GeneratePic(PicOptions{ID: 8, PNGRelID: "rId4", ExtentCX: 100, ExtentCY: 100, SrcRect: r})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(plain), "srcRect") {
			t.Errorf("no crop requested (%v) but srcRect emitted", r)
		}
	}
}
