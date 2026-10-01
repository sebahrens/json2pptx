package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

var titleXfrmRE = regexp.MustCompile(`name="title"[\s\S]*?<a:off x="(-?\d+)" y="(-?\d+)"\s*/?>`)

// TestContentTitleSitsOnOneBaselineEveryTemplate pins go-slide-creator-kyk01
// on rendered output: the title of a bullet slide (One Content), a side by
// side slide (Two Content) and a pattern slide (Blank + Title) starts at the
// same x / y (±0.1in) on every template, so the headline does not jump from
// slide to slide. abstract's One Content title used to sit 0.5in right and
// ~1in lower than the other two.
func TestContentTitleSitsOnOneBaselineEveryTemplate(t *testing.T) {
	const tol = 91440
	for _, tpl := range testutil.AllTestTemplateNames() {
		t.Run(tpl, func(t *testing.T) {
			if tpl == "modern-template" {
				// Allow-listed in internal/template/testdata/conformance_allowlist.json:
				// its reviewed bottom-anchored content title cannot move without
				// lowering pattern readability (go-slide-creator-kyk01).
				t.Skip("modern-template title geometry is a tracked exception")
			}
			t1, t2, t3 := "Revenue grew 17% on new logos", "Two entry routes remain", "Three numbers frame the decision"
			b1, b2, b3 := []string{"Revenue EUR 21M", "Margin 68%"}, []string{"Greenfield", "Capex EUR 9M"}, []string{"Acquisition", "Capex EUR 14M"}
			in := &PresentationInput{Template: tpl, OutputFilename: "titles.pptx", Slides: []SlideInput{
				{LayoutID: "content", Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &t1},
					{PlaceholderID: "body", Type: "bullets", BulletsValue: &b1},
				}},
				{LayoutID: "two-column", Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &t2},
					{PlaceholderID: "body", Type: "bullets", BulletsValue: &b2},
					{PlaceholderID: "body_2", Type: "bullets", BulletsValue: &b3},
				}},
				{LayoutID: "blank-title", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &t3}},
					Pattern: &PatternInput{Name: "kpi-3up", Values: json.RawMessage(`[{"big":"17%","small":"Revenue growth"},{"big":"68%","small":"Gross margin"},{"big":"EUR 32M","small":"Cash"}]`)}},
			}}
			applyDefaults(in)
			res, cleanup, err := RunPresentation(context.Background(), in, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			titleAt := func(n int) (x, y int64) {
				m := titleXfrmRE.FindSubmatch(readDeckSlide(t, res.OutputPath, n))
				if m == nil {
					t.Fatalf("slide %d: no title transform in the generated slide", n)
				}
				x, _ = strconv.ParseInt(string(m[1]), 10, 64)
				y, _ = strconv.ParseInt(string(m[2]), 10, 64)
				return x, y
			}
			x0, y0 := titleAt(1)
			for _, n := range []int{2, 3} {
				x, y := titleAt(n)
				if d := x - x0; d > tol || d < -tol {
					t.Errorf("slide %d (%s) title x=%d differs from the bullet slide's %d", n, res.SlideSpecs[n-1].LayoutID, x, x0)
				}
				if d := y - y0; d > tol || d < -tol {
					t.Errorf("slide %d (%s) title y=%d differs from the bullet slide's %d", n, res.SlideSpecs[n-1].LayoutID, y, y0)
				}
			}
		})
	}
}
