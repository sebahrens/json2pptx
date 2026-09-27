package main

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

func TestZZDebugScales(t *testing.T) {
	path := os.Getenv("ZZ_DECK")
	if path == "" {
		t.Skip()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var in PresentationInput
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	tpl := os.Getenv("ZZ_TPL")
	if tpl == "" {
		tpl = "midnight-blue"
	}
	geom := loadSchemaMaximaGeometry(t, tpl)
	specs, _, _, err := convertPresentationSlides(in.Slides, geom.layouts, geom.width, geom.height, nil, nil, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`id="(\d+)"[\s\S]*?<a:ext cx="(\d+)" cy="(\d+)"[\s\S]*?<a:bodyPr([^>]*>(?:<a:normAutofit[^>]*>)?)[\s\S]*?<a:t>([^<]{0,40})`)
	for si, s := range specs {
		for _, raw := range s.RawShapeXML {
			for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
				t.Logf("slide %d id=%s cx=%s cy=%s body=%s text=%q", si, m[1], m[2], m[3], m[4], m[5])
			}
		}
	}
}
