package generator

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestYellowNativeArtworkStaysInGutter(t *testing.T) {
	templatePath := "../../templates/modern-yellow.pptx"
	check := func(path string) {
		t.Helper()
		var master struct {
			Shapes []struct {
				Identity struct {
					Name string `xml:"name,attr"`
				} `xml:"nvSpPr>cNvPr"`
				Properties struct {
					Transform struct {
						Offset struct {
							X int64 `xml:"x,attr"`
							Y int64 `xml:"y,attr"`
						} `xml:"off"`
						Extent struct {
							CX int64 `xml:"cx,attr"`
							CY int64 `xml:"cy,attr"`
						} `xml:"ext"`
					} `xml:"xfrm"`
					Geometry struct {
						Inner string `xml:",innerxml"`
					} `xml:"custGeom"`
					Fill struct {
						Color string `xml:"val,attr"`
						Alpha struct {
							Value string `xml:"val,attr"`
						} `xml:"alpha"`
					} `xml:"solidFill>srgbClr"`
					ShadowAlpha struct {
						Value string `xml:"val,attr"`
					} `xml:"effectLst>outerShdw>schemeClr>alpha"`
				} `xml:"spPr"`
			} `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/slideMasters/slideMaster1.xml")), &master); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, shape := range master.Shapes {
			if shape.Identity.Name != "Freeform 3" {
				continue
			}
			found++
			p := shape.Properties
			if p.Transform.Offset.X != -1779706 || p.Transform.Offset.Y != 1898247 || p.Transform.Extent.CX != 2079706 || p.Transform.Extent.CY != 4140000 || p.Transform.Offset.X+p.Transform.Extent.CX != 300000 {
				t.Fatalf("native disc no longer stays in its gutter: %+v", p.Transform)
			}
			if p.Fill.Color != "FECE00" || p.Fill.Alpha.Value != "24000" || p.ShadowAlpha.Value != "8000" {
				t.Fatalf("quiet native artwork styling drifted: %+v", p)
			}
			if fmt.Sprintf("%x", sha256.Sum256([]byte(p.Geometry.Inner))) != "b5734680909455314774b0cb940f1c172fb686ec28d0de62e27984a6e520ad96" {
				t.Fatal("existing native custom artwork path changed")
			}
		}
		if found != 1 {
			t.Fatalf("expected one native disc, got %d", found)
		}
		checkedFrames, minimumClearance := 0, int64(1<<62)
		for index := 1; index <= 8; index++ {
			var layout struct {
				Shapes []struct {
					Placeholder struct {
						Type string `xml:"type,attr"`
					} `xml:"nvSpPr>nvPr>ph"`
					Offset struct {
						X int64 `xml:"x,attr"`
					} `xml:"spPr>xfrm>off"`
				} `xml:"cSld>spTree>sp"`
			}
			if err := xml.Unmarshal([]byte(readZipFileString(t, path, fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", index))), &layout); err != nil {
				t.Fatal(err)
			}
			for _, shape := range layout.Shapes {
				if shape.Placeholder.Type != "body" {
					continue
				}
				if shape.Offset.X <= 0 || shape.Offset.X-300000 < 180000 {
					t.Fatalf("native body frame on layout%d lacks resolved 0.5cm clearance from disc edge", index)
				}
				checkedFrames++
				minimumClearance = min(minimumClearance, shape.Offset.X-300000)
			}
		}
		if checkedFrames < 4 {
			t.Fatalf("native body frame coverage missing: %d", checkedFrames)
		}
		t.Logf("checked %d native body frames: minimum geometry clearance %.3fcm (shadow bounds excluded)", checkedFrames, float64(minimumClearance)/360000)
	}
	check(templatePath)
	output := filepath.Join(t.TempDir(), "yellow-gutter.pptx")
	_, _, err := generateSinglePass(context.Background(), GenerationRequest{TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout3", Content: []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Required review title"},
		{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Required parent wording", "\tRequired child wording", "Required final wording"}},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	check(output)
	parts := []string{"ppt/theme/theme1.xml", "ppt/slideMasters/slideMaster1.xml", "ppt/slideMasters/_rels/slideMaster1.xml.rels"}
	for index := 1; index <= 8; index++ {
		parts = append(parts, fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", index), fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", index))
	}
	for _, part := range parts {
		if readZipFileString(t, templatePath, part) != readZipFileString(t, output, part) {
			t.Fatalf("generation changed preserved native theme/layout payload: %s", part)
		}
	}
	slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
	for _, text := range []string{"Required review title", "Required parent wording", "Required child wording", "Required final wording"} {
		if strings.Count(slide, "<a:t>"+text+"</a:t>") != 1 {
			t.Fatalf("complete source wording missing or duplicated: %s", text)
		}
	}
}
