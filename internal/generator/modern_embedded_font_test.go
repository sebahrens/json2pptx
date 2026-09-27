package generator

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestModernOriginalEmbeddedFontsPreserved(t *testing.T) {
	const source = "../../templates/modern-template.pptx"
	check := func(t *testing.T, path string) {
		t.Helper()
		for _, face := range []struct{ style, sha string }{
			{"Regular", "c72a925082ba55885e7b4ead25d34a3c91fa44edda72a16a3a2983fc7fa7cef3"},
			{"Bold", "32baf740eac1bf3f257904a6cf89e4e067ca1566b375bf22f743a26fc88b3791"},
		} {
			font := []byte(readZipFileString(t, path, "ppt/fonts/Lora-"+face.style+".fntdata"))
			if len(font) < 80 {
				t.Fatalf("missing or truncated embedded %s", face.style)
			}
			if binary.LittleEndian.Uint32(font[:4]) != uint32(len(font)) || binary.LittleEndian.Uint32(font[8:12]) != 0x00020001 || binary.LittleEndian.Uint16(font[34:36]) != 0x504c || binary.LittleEndian.Uint16(font[32:34]) != 0 {
				t.Fatalf("invalid unrestricted EOT header for %s", face.style)
			}
			size := int(binary.LittleEndian.Uint32(font[4:8]))
			if size <= 0 || size > len(font)-80 {
				t.Fatalf("invalid original font size for %s", face.style)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(font[len(font)-size:])); got != face.sha {
				t.Errorf("embedded original %s changed: %s", face.style, got)
			}
		}
		var presentation struct {
			Embed  string `xml:"embedTrueTypeFonts,attr"`
			Subset string `xml:"saveSubsetFonts,attr"`
			Fonts  []struct {
				Font struct {
					Typeface string `xml:"typeface,attr"`
				} `xml:"font"`
				Regular struct {
					ID string `xml:"id,attr"`
				} `xml:"regular"`
				Bold struct {
					ID string `xml:"id,attr"`
				} `xml:"bold"`
			} `xml:"embeddedFontLst>embeddedFont"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/presentation.xml")), &presentation); err != nil {
			t.Fatal(err)
		}
		if presentation.Embed != "1" || presentation.Subset != "0" {
			t.Error("original font embedding disabled or subset-only")
		}
		found := 0
		for _, font := range presentation.Fonts {
			if font.Font.Typeface != "Lora" {
				continue
			}
			found++
			if font.Regular.ID != "rIdLoraRegular" || font.Bold.ID != "rIdLoraBold" {
				t.Error("native Lora face references changed")
			}
		}
		if found != 1 {
			t.Errorf("found %d native embedded Lora families, want one", found)
		}
		var relationships struct {
			Entries []struct {
				ID     string `xml:"Id,attr"`
				Type   string `xml:"Type,attr"`
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "ppt/_rels/presentation.xml.rels")), &relationships); err != nil {
			t.Fatal(err)
		}
		for _, style := range []string{"Regular", "Bold"} {
			found := 0
			for _, relationship := range relationships.Entries {
				if relationship.ID != "rIdLora"+style {
					continue
				}
				found++
				if relationship.Target != "fonts/Lora-"+style+".fntdata" || relationship.Type != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" {
					t.Errorf("invalid %s font relationship", style)
				}
			}
			if found != 1 {
				t.Errorf("missing or duplicated %s font relationship", style)
			}
		}
		var contentTypes struct {
			Defaults []struct {
				Extension string `xml:"Extension,attr"`
				Type      string `xml:"ContentType,attr"`
			} `xml:"Default"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, "[Content_Types].xml")), &contentTypes); err != nil {
			t.Fatal(err)
		}
		found = 0
		for _, part := range contentTypes.Defaults {
			if part.Extension == "fntdata" {
				found++
				if part.Type != "application/x-fontdata" {
					t.Error("incorrect native font content type")
				}
			}
		}
		if found != 1 {
			t.Error("missing or duplicated font content type")
		}
	}
	check(t, source)
	for _, exclude := range []bool{true, false} {
		t.Run(fmt.Sprintf("exclude-template-slides=%v", exclude), func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "original-fonts.pptx")
			const title = "Embedded font preservation"
			_, _, err := generateSinglePass(context.Background(), GenerationRequest{
				TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: exclude,
				Slides: []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: title},
					{PlaceholderID: "body", Type: ContentText, Value: "Original native typography"},
					{PlaceholderID: "Section Number", Type: ContentText, Value: "07"},
				}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			check(t, output)
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			found := false
			for _, part := range z.File {
				if strings.HasPrefix(part.Name, "ppt/slides/slide") && strings.HasSuffix(part.Name, ".xml") {
					slide := readZipFileString(t, output, part.Name)
					if strings.Contains(slide, title) {
						found = true
						for _, wording := range []string{"Original native typography", "07"} {
							if !strings.Contains(slide, wording) {
								t.Errorf("authored wording lost: %s", wording)
							}
						}
					}
				}
			}
			if !found {
				t.Error("authored native title lost")
			}
		})
	}
}
