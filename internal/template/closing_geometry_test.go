package template

import (
	"encoding/xml"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestModernClosingRuleClearsSubtitle(t *testing.T) {
	r, err := OpenTemplate(filepath.Join("..", "..", "templates", "modern-template.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	layouts, err := ParseLayouts(r)
	if err != nil {
		t.Fatal(err)
	}
	var subtitle *types.PlaceholderInfo
	for _, layout := range layouts {
		if layout.ID != "slideLayout5" {
			continue
		}
		for i := range layout.Placeholders {
			if layout.Placeholders[i].Type == types.PlaceholderSubtitle {
				subtitle = &layout.Placeholders[i]
			}
		}
	}
	if subtitle == nil {
		t.Fatal("Closing subtitle missing")
	}
	body, err := r.ReadFile("ppt/slideLayouts/slideLayout5.xml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Connectors []struct {
			Properties struct {
				Transform transformXML `xml:"xfrm"`
			} `xml:"spPr"`
		} `xml:"cSld>spTree>cxnSp"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Connectors) != 1 {
		t.Fatal("expected the existing decorative rule, not its removal")
	}
	tr := doc.Connectors[0].Properties.Transform
	if tr.Offset == nil || tr.Extents == nil || tr.Extents.CY <= 0 {
		t.Fatal("rule geometry missing")
	}
	const halfCM = 180000
	if gap := subtitle.Bounds.Y - (tr.Offset.Y + tr.Extents.CY); gap < halfCM {
		t.Fatalf("rule intrudes into subtitle clearance: gap=%d EMU, want >=%d", gap, halfCM)
	}
}

func TestClosingLayoutsCenterTitleSubtitleLikeTitleSlide(t *testing.T) {
	for _, templateName := range []string{"midnight-blue", "forest-green"} {
		t.Run(templateName, func(t *testing.T) {
			reader, err := OpenTemplate(filepath.Join("..", "..", "templates", templateName+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			layouts, err := ParseLayouts(reader)
			if err != nil {
				t.Fatal(err)
			}
			var titleSlide, closing *types.LayoutMetadata
			for i := range layouts {
				switch layouts[i].ID {
				case "slideLayout1":
					titleSlide = &layouts[i]
				case "slideLayout5":
					closing = &layouts[i]
				}
			}
			if titleSlide == nil || closing == nil {
				t.Fatal("missing Title Slide or Closing layout")
			}
			for _, role := range []types.PlaceholderType{types.PlaceholderTitle, types.PlaceholderSubtitle} {
				var source, target *types.PlaceholderInfo
				for i := range titleSlide.Placeholders {
					if titleSlide.Placeholders[i].Type == role {
						source = &titleSlide.Placeholders[i]
					}
				}
				for i := range closing.Placeholders {
					if closing.Placeholders[i].Type == role {
						target = &closing.Placeholders[i]
					}
				}
				if source == nil || target == nil {
					t.Fatalf("missing %s placeholder", role)
				}
				if target.Bounds != source.Bounds {
					t.Errorf("Closing %s bounds %+v differ from centred Title Slide %+v", role, target.Bounds, source.Bounds)
				}
			}
		})
	}
}
