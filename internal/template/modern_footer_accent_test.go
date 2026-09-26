package template_test

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"testing"
)

func TestModernContentFooterAccentIsContainedThinBand(t *testing.T) {
	z, err := zip.OpenReader("../../templates/modern.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	read := func(name string) []byte {
		t.Helper()
		for _, part := range z.File {
			if part.Name != name {
				continue
			}
			f, err := part.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(f)
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
		t.Fatalf("missing part %s", name)
		return nil
	}
	var canvas struct {
		Size struct {
			W int64 `xml:"cx,attr"`
			H int64 `xml:"cy,attr"`
		} `xml:"sldSz"`
	}
	if err := xml.Unmarshal(read("ppt/presentation.xml"), &canvas); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Shapes []struct {
			NonVisual struct {
				Name struct {
					Value string `xml:"name,attr"`
				} `xml:"cNvPr"`
			} `xml:"nvSpPr"`
			Properties struct {
				Transform struct {
					Offset struct {
						X int64 `xml:"x,attr"`
						Y int64 `xml:"y,attr"`
					} `xml:"off"`
					Extent struct {
						W int64 `xml:"cx,attr"`
						H int64 `xml:"cy,attr"`
					} `xml:"ext"`
				} `xml:"xfrm"`
				Gradient *struct{} `xml:"gradFill"`
			} `xml:"spPr"`
		} `xml:"cSld>spTree>sp"`
	}
	if err := xml.Unmarshal(read("ppt/slideLayouts/slideLayout3.xml"), &document); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, shape := range document.Shapes {
		if shape.NonVisual.Name.Value != "Rectangle 7" {
			continue
		}
		count++
		frame := shape.Properties.Transform
		if shape.Properties.Gradient == nil || frame.Offset.X != 0 || frame.Extent.W != canvas.Size.W || frame.Extent.H <= 0 || frame.Extent.H > 100000 || frame.Offset.Y+frame.Extent.H != canvas.Size.H {
			t.Fatalf("content accent must be a gradient band spanning the canvas at its bottom, no taller than 100000EMU: %+v", frame)
		}
	}
	if count != 1 {
		t.Fatalf("accent shapes=%d want1", count)
	}
}
