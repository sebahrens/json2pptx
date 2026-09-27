package generator

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestModernClosingPreservesNativeStylesAndGroupedText(t *testing.T) {
	const source = "../../templates/modern-template.pptx"
	check := func(path string) {
		t.Helper()
		decoder := xml.NewDecoder(strings.NewReader(readZipFileString(t, path, "ppt/slideLayouts/slideLayout5.xml")))
		shape, found := "", map[string]bool{}
		checked := map[string]bool{}
		var parents []string
		for {
			token, err := decoder.Token()
			if err != nil {
				if err != io.EOF {
					t.Fatal(err)
				}
				break
			}
			start, ok := token.(xml.StartElement)
			if !ok {
				if _, end := token.(xml.EndElement); end {
					parents = parents[:len(parents)-1]
				}
				continue
			}
			parent := ""
			if len(parents) > 0 {
				parent = parents[len(parents)-1]
			}
			parents = append(parents, start.Name.Local)
			attrs := map[string]string{}
			for _, attr := range start.Attr {
				attrs[attr.Name.Local] = attr.Value
			}
			if start.Name.Local == "cNvPr" {
				shape = attrs["name"]
				found[shape] = true
			}
			if shape == "Picture 7" && start.Name.Local == "alphaModFix" {
				checked["art opacity"] = true
				if attrs["amt"] != "15000" {
					t.Error("Closing artwork opacity differs from the blind-reviewed revision")
				}
			}
			if shape != "title" && shape != "subtitle" {
				continue
			}
			if start.Name.Local == "ext" && parent == "xfrm" {
				checked[shape+" width"] = true
				want := "10330857"
				if shape == "subtitle" {
					want = "4800000"
				}
				if attrs["cx"] != want {
					t.Errorf("%s width=%s; want %s", shape, attrs["cx"], want)
				}
			}
			if start.Name.Local == "off" && parent == "xfrm" {
				checked[shape+" x"] = true
				if attrs["x"] != "1000000" {
					t.Errorf("%s left edge does not match the reviewed aligned group", shape)
				}
				if shape == "title" && attrs["y"] != "1500000" {
					t.Error("native title vertical position changed")
				}
			}
			if shape == "subtitle" && start.Name.Local == "bodyPr" {
				checked["subtitle anchor"] = true
				if attrs["anchor"] != "t" {
					t.Error("subtitle must remain top-anchored beside the title group")
				}
			}
			if shape == "subtitle" && start.Name.Local == "off" && parent == "xfrm" {
				checked["subtitle y"] = true
				if attrs["y"] != "3450000" {
					t.Error("subtitle separation regressed")
				}
			}
			if start.Name.Local == "defRPr" && !checked[shape+" size"] {
				checked[shape+" size"] = true
				want := "4500"
				if shape == "subtitle" {
					want = "1800"
				}
				if attrs["sz"] != want {
					t.Errorf("native %s font size changed", shape)
				}
			}
		}
		for _, field := range []string{"title width", "subtitle width", "title x", "subtitle x", "subtitle anchor", "subtitle y", "title size", "subtitle size", "art opacity"} {
			if !checked[field] {
				t.Errorf("missing native %s", field)
			}
		}
		if !found["title"] || !found["subtitle"] {
			t.Fatal("Closing lost a required editable placeholder")
		}
		master := readZipFileString(t, path, "ppt/slideMasters/slideMaster1.xml")
		if !strings.Contains(master, `cap="all"`) || !strings.Contains(master, `typeface="+mj-lt"`) {
			t.Fatal("native master capitalization/font must be preserved")
		}
		art := []byte(readZipFileString(t, path, "ppt/media/image4.png"))
		if fmt.Sprintf("%x", sha256.Sum256(art)) != "342e7a398e3e74a36f1d2c29cc98c440e7bcaef676f95afd641e589dcfcc132f" {
			t.Fatal("Closing artwork differs from the visually reviewed revision")
		}
	}
	check(source)
	for _, title := range []string{"Quarterly operating review", "Quarterly operating review and service delivery priorities"} {
		t.Run(title, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "closing.pptx")
			_, _, err := generateSinglePass(context.Background(), GenerationRequest{
				TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: "slideLayout5", Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: title},
					{PlaceholderID: "subtitle", Type: ContentText, Value: "Internal review of service delivery, ownership and reporting requirements"},
				}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			check(output)
			slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
			if !strings.Contains(slide, title) {
				t.Fatal("required title was lost or mutated")
			}
			if !strings.Contains(slide, "Internal review of service delivery, ownership and reporting requirements") {
				t.Fatal("required subtitle was lost or mutated")
			}
			if strings.Contains(slide, `sz="4050"`) {
				t.Fatal("reviewed wider native title still shrinks below its 45pt default")
			}
		})
	}
}
