package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

var (
	shapeOffXRegexp = regexp.MustCompile(`<a:off x="(-?\d+)"`)
	shapeLInsRegexp = regexp.MustCompile(`lIns="(\d+)"`)
)

// go-slide-creator-svrpx: unfilled first-column text of the row-list patterns
// starts on the title's text edge (title X + its lIns), and the takeaway band
// shares the pattern's left edge.
func TestPatternTextSharesTitleLeftEdge(t *testing.T) {
	templates := []string{"abstract", "forest-green", "midnight-blue"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	for _, tmpl := range templates {
		tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", tmpl+".pptx"))
		if err != nil {
			t.Fatal(err)
		}
		// metric-list leads with a right-aligned value column and filled
		// labeled-rows blocks keep their padding; their rules / blocks start
		// at the pattern edge, checked through the takeaway alignment below.
		for _, name := range []string{"exec-summary", "next-steps", "agenda", "labeled-rows"} {
			t.Run(tmpl+"/"+name, func(t *testing.T) {
				pat, _ := patterns.Default().Get(name)
				ex, ok := pat.(patterns.Exemplar)
				if !ok {
					t.Skip("no exemplar")
				}
				values, _ := json.Marshal(ex.ExemplarValues())
				title := "A one-line action title"
				slide := SlideInput{
					LayoutID: "blank-title",
					Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
					Pattern:  &PatternInput{Name: name, Values: values, Overrides: overridesFor(name)},
					Takeaway: "The bottom line.",
				}
				geom := resolveGridGeometry(slide, tctx.layouts, tctx.slideWidth, tctx.slideHeight)
				if geom.Zone == nil || geom.Zone.TextLeft == 0 {
					t.Skip("no title text edge resolved")
				}
				specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.metadata, nil, "", nil, false)
				if err != nil {
					t.Fatal(err)
				}
				textLeft := int64(-1)
				for _, sh := range specs[0].RawShapeXML {
					if !regexp.MustCompile(`<a:t>[^<]+</a:t>`).Match(sh) {
						continue
					}
					xm, im := shapeOffXRegexp.FindSubmatch(sh), shapeLInsRegexp.FindSubmatch(sh)
					if xm == nil || im == nil {
						continue
					}
					x, _ := strconv.ParseInt(string(xm[1]), 10, 64)
					ins, _ := strconv.ParseInt(string(im[1]), 10, 64)
					if textLeft < 0 || x+ins < textLeft {
						textLeft = x + ins
					}
				}
				if textLeft < 0 {
					t.Fatal("no text shapes")
				}
				if d := float64(textLeft-geom.Zone.TextLeft) / 12700; d < -2 || d > 2 {
					t.Errorf("first-column text starts %.1fpt from the title text edge (want within 2pt)", d)
				}
				layout := findLayoutByID(tctx.layouts, specs[0].LayoutID)
				frame := template.ResolveChromeFrame(layout, template.ChromeReferenceLayout(tctx.layouts), tctx.slideWidth, tctx.slideHeight, true, false)
				if d := float64(frame.Takeaway.X-geom.Zone.LeftMargin) / 12700; d < -2 || d > 2 {
					t.Errorf("takeaway band left %.1fpt from the pattern's left edge (want within 2pt)", d)
				}
			})
		}
		_ = tctx.reader.Close()
	}
}

func overridesFor(name string) json.RawMessage {
	if name == "labeled-rows" {
		return json.RawMessage(`{"label_style":"text"}`)
	}
	return nil
}
