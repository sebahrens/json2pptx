package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

var shapeOffYRegexp = regexp.MustCompile(`<a:off x="-?\d+" y="(\d+)"`)

// go-slide-creator-e17xy: a short pattern block on a content layout starts
// where the template's native body text starts (the body placeholder top),
// not hard under the title and not floating mid-slide.
func TestShortPatternStartsAtBodyPlaceholderTop(t *testing.T) {
	templates := []string{"abstract", "forest-green", "modern-template"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tctx.reader.Close() }()
			title := "Card grid beside a native title"
			slide := SlideInput{
				LayoutID: "content",
				Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
				Pattern: &PatternInput{Name: "card-grid", Values: json.RawMessage(`{"columns":3,"rows":1,"cells":[` +
					`{"header":"People","body":"Retrain 40 agents on the unified playbook"},` +
					`{"header":"Process","body":"Single intake form with skill-based routing"},` +
					`{"header":"Technology","body":"Consolidate onto one ITSM platform"}]}`)},
			}
			specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.metadata, nil, "", nil, false)
			if err != nil {
				t.Fatal(err)
			}
			layout := findLayoutByID(tctx.layouts, specs[0].LayoutID)
			if layout == nil {
				t.Fatalf("layout %q not found", specs[0].LayoutID)
			}
			body, ok := firstBodyOrContentBounds(layout)
			if !ok {
				t.Skipf("layout %q has no body placeholder", layout.ID)
			}
			top := int64(-1)
			for _, sh := range specs[0].RawShapeXML {
				for _, m := range shapeOffYRegexp.FindAllSubmatch(sh, -1) {
					y, _ := strconv.ParseInt(string(m[1]), 10, 64)
					if top < 0 || y < top {
						top = y
					}
				}
			}
			if top < 0 {
				t.Fatal("no pattern shapes rendered")
			}
			if d := float64(top-body.Y) / 12700; d < -6 || d > 6 {
				t.Errorf("pattern block starts %.1fpt from the body placeholder top (want within 6pt)", d)
			}
		})
	}
}
