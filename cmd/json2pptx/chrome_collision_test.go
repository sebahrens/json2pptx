package main

import (
	"encoding/xml"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestModernYellowPopulatedBodyClearsGutterAndSyntheticCollisionReported(t *testing.T) {
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
	if got := collectChromeCollisionFindings(input, p.Layouts, p.SlideWidth); len(got) != 0 {
		t.Fatalf("repaired gutter artwork falsely collides: %+v", got)
	}
	// Keep actual reporting coverage with the original intrusive opaque geometry.
	layouts := append([]types.LayoutMetadata(nil), p.Layouts...)
	for i := range layouts {
		if layouts[i].ID == id {
			layouts[i].DecorRegions = []types.DecorRegion{{Source: "master", Name: "synthetic opaque disc", X: 0, Y: 1898247, Width: 2079706, Height: 4140000}}
		}
	}
	findings := collectChromeCollisionFindings(input, layouts, p.SlideWidth)
	if len(findings) != 1 || findings[0].Code != patterns.ErrCodeChromeCollision || findings[0].Path != "/slides/0/content/1" {
		t.Fatalf("findings = %+v, want one body CHROME_COLLISION", findings)
	}
	assertFindingPointerReplaceable(t, input, findings[0].Path)
	var inReport bool
	for _, f := range collectFitFindings(input, layouts, p.SlideWidth, p.SlideHeight, nil) {
		if f.Code == patterns.ErrCodeChromeCollision && f.Path == "/slides/0/content/1" {
			inReport = true
		}
	}
	if !inReport {
		t.Fatal("preflight fit report omitted CHROME_COLLISION")
	}
	input.Slides[0].Content = input.Slides[0].Content[:1]
	if got := collectChromeCollisionFindings(input, layouts, p.SlideWidth); len(got) != 0 {
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

func yellowNativeDiscRight(t *testing.T, reader *template.Reader) int64 {
	t.Helper()
	data, err := reader.ReadFile("ppt/slideMasters/slideMaster1.xml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Shapes []struct {
			Identity struct {
				Name string `xml:"name,attr"`
			} `xml:"nvSpPr>cNvPr"`
			Transform struct {
				Offset struct {
					X int64 `xml:"x,attr"`
				} `xml:"off"`
				Extent struct {
					Width int64 `xml:"cx,attr"`
				} `xml:"ext"`
			} `xml:"spPr>xfrm"`
		} `xml:"cSld>spTree>sp"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	found := 0
	var right int64
	for _, shape := range doc.Shapes {
		if shape.Identity.Name == "Freeform 3" {
			found++
			right = shape.Transform.Offset.X + shape.Transform.Extent.Width
		}
	}
	if found != 1 || right != 300000 {
		t.Fatalf("actual native gutter disc geometry drifted: count=%d right=%d", found, right)
	}
	return right
}
