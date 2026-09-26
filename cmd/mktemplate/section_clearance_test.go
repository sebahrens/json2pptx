package main

import (
	"bytes"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegeneratedClosingPreservesReviewedNativeGeometry(t *testing.T) {
	for _, def := range templates {
		t.Run(def.Name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), def.Name+".pptx")
			if err := generateTemplate(def, path); err != nil {
				t.Fatal(err)
			}
			fresh, _, err := readZipParts(path)
			if err != nil {
				t.Fatal(err)
			}
			current, _, err := readZipParts(filepath.Join("..", "..", "templates", def.Name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			for _, part := range []string{"ppt/slideLayouts/slideLayout1.xml", "ppt/slideLayouts/slideLayout5.xml"} {
				if !bytes.Equal(current[part], fresh[part]) {
					t.Fatalf("regeneration changes reviewed title/closing part %s", part)
				}
			}
		})
	}
}

func TestForestSectionNumeralCentredWithoutChangingOtherTemplates(t *testing.T) {
	for _, def := range templates {
		t.Run(def.Name, func(t *testing.T) {
			data := sectionLayout(def)
			var parsed struct {
				Shapes []struct {
					NV struct {
						Name string `xml:"name,attr"`
					} `xml:"nvSpPr>cNvPr"`
					Body struct {
						Anchor string `xml:"anchor,attr"`
					} `xml:"txBody>bodyPr"`
				} `xml:"cSld>spTree>sp"`
			}
			if err := xml.Unmarshal([]byte(data), &parsed); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, shape := range parsed.Shapes {
				if shape.NV.Name != "Section Number" {
					continue
				}
				found = true
				want := ""
				if def.Name == "forest-green" {
					want = "ctr"
				}
				if shape.Body.Anchor != want {
					t.Fatalf("number anchor=%q, want %q", shape.Body.Anchor, want)
				}
			}
			if !found {
				t.Fatal("missing section number")
			}
			if !strings.Contains(data, `sz="13600"`) || !strings.Contains(data, `<a:off x="7886700" y="1268413"/>`) {
				t.Fatal("section numeral typography or frame changed")
			}
		})
	}
}
