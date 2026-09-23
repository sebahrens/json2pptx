package main

import (
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestModernYellowPopulatedBodyReportsChromeCollision(t *testing.T) {
	r, err := template.OpenTemplate(filepath.Join("..", "..", "templates", "modern-yellow.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	p, err := template.BuildProfile(r)
	if err != nil {
		t.Fatal(err)
	}
	id := p.RoleBindings[types.CanonicalLayoutOneContent]
	input := &PresentationInput{Slides: []SlideInput{{LayoutID: id, Content: []ContentInput{{PlaceholderID: "title", Type: "text"}, {PlaceholderID: "body", Type: "text"}}}}}
	findings := collectChromeCollisionFindings(input, p.Layouts, p.SlideWidth)
	if len(findings) != 1 || findings[0].Code != patterns.ErrCodeChromeCollision || findings[0].Path != "/slides/0/content/body" {
		t.Fatalf("findings = %+v, want one body CHROME_COLLISION", findings)
	}
	var inReport bool
	for _, f := range collectFitFindings(input, p.Layouts, p.SlideWidth, p.SlideHeight, nil) {
		if f.Code == patterns.ErrCodeChromeCollision && f.Path == "/slides/0/content/body" {
			inReport = true
		}
	}
	if !inReport {
		t.Fatal("preflight fit report omitted CHROME_COLLISION")
	}
	input.Slides[0].Content = input.Slides[0].Content[:1]
	if got := collectChromeCollisionFindings(input, p.Layouts, p.SlideWidth); len(got) != 0 {
		t.Fatalf("title-only slide produced chrome collision: %+v", got)
	}
}

func TestSideArtworkOverlapRequiresSubstantialEdgeIntersection(t *testing.T) {
	body := types.BoundingBox{X: 800000, Y: 1500000, Width: 5000000, Height: 3500000}
	for _, tc := range []struct {
		name string
		art  types.DecorRegion
		want bool
	}{
		{"left disc", types.DecorRegion{X: 0, Y: 1800000, Width: 2000000, Height: 3000000}, true},
		{"thin rule", types.DecorRegion{X: 0, Y: 1800000, Width: 2000000, Height: 100000}, false},
		{"center panel", types.DecorRegion{X: 2200000, Y: 1800000, Width: 1000000, Height: 3000000}, false},
		{"broad background", types.DecorRegion{X: 0, Y: 0, Width: 11000000, Height: 6858000}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sideArtworkOverlaps(body, tc.art, 12192000); got != tc.want {
				t.Errorf("overlap = %v, want %v", got, tc.want)
			}
		})
	}
}
