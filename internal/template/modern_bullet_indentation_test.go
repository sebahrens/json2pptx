package template_test

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"testing"
)

// Parent and child bullets must have distinct text origins. Paragraph lvl alone
// does not prove visible hierarchy when local list styles override the master.
func TestModernNativeBulletLevelsHaveVisibleIndentation(t *testing.T) {
	z, err := zip.OpenReader("../../templates/modern.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	seen := 0
	for _, part := range z.File {
		if part.Name != "ppt/slideLayouts/slideLayout3.xml" && part.Name != "ppt/slideLayouts/slideLayout7.xml" {
			continue
		}
		seen++
		r, err := part.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Shapes []struct {
				Text struct {
					Style struct {
						Levels []struct {
							XMLName xml.Name
							Margin  int64 `xml:"marL,attr"`
							Indent  int64 `xml:"indent,attr"`
						} `xml:",any"`
					} `xml:"lstStyle"`
				} `xml:"txBody"`
			} `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		bodies := 0
		for _, shape := range doc.Shapes {
			levels := shape.Text.Style.Levels
			if len(levels) < 2 { // Title/artwork have no nested bullet list.
				continue
			}
			bodies++
			for i, level := range levels {
				want := int64(i+1) * 228600
				if level.Margin != want || level.Indent != -228600 {
					t.Errorf("%s/%s: nested level %d margin=%d indent=%d; want margin=%d indent=-228600", part.Name, level.XMLName.Local, i, level.Margin, level.Indent, want)
				}
			}
		}
		wantBodies := 1
		if part.Name == "ppt/slideLayouts/slideLayout7.xml" {
			wantBodies = 2
		}
		if bodies != wantBodies {
			t.Errorf("%s: verified bullet bodies=%d want%d", part.Name, bodies, wantBodies)
		}
	}
	if seen != 2 {
		t.Fatalf("reviewed layouts=%d want2", seen)
	}
}
