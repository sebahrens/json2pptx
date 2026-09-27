package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These guards accompany the original-size PDF and blind-review evidence for
// go-slide-creator-3qzo4.69/.109. OOXML assertions alone are not visual approval.
func TestModernContentTypographySourceAndGeneration(t *testing.T) {
	const source = "../../templates/modern-template.pptx"
	var master struct {
		TitleRun struct {
			Size int    `xml:"sz,attr"`
			Caps string `xml:"cap,attr"`
		} `xml:"txStyles>titleStyle>lvl1pPr>defRPr"`
	}
	if err := xml.Unmarshal([]byte(readZipFileString(t, source, "ppt/slideMasters/slideMaster1.xml")), &master); err != nil {
		t.Fatal(err)
	}
	if master.TitleRun.Size != 4500 || master.TitleRun.Caps != "all" {
		t.Fatal("native45pt uppercase master title styling changed")
	}
	readShapes := func(t *testing.T, path, part string) map[string]shapeXML {
		t.Helper()
		var document struct {
			Shapes []shapeXML `xml:"cSld>spTree>sp"`
		}
		if err := xml.Unmarshal([]byte(readZipFileString(t, path, part)), &document); err != nil {
			t.Fatal(err)
		}
		shapes := make(map[string]shapeXML, len(document.Shapes))
		for _, shape := range document.Shapes {
			name := shape.NonVisualProperties.ConnectionNonVisual.Name
			if _, duplicate := shapes[name]; duplicate {
				t.Fatalf("duplicate native shape %q", name)
			}
			shapes[name] = shape
		}
		return shapes
	}
	checkFramesAndStyles := func(t *testing.T, shapes map[string]shapeXML, layout string) {
		t.Helper()
		title, ok := shapes["title"]
		if !ok || title.ShapeProperties.Transform == nil || title.TextBody == nil || title.TextBody.BodyProperties == nil || title.TextBody.ListStyle == nil {
			t.Fatal("missing native title geometry or typography")
		}
		frame := title.ShapeProperties.Transform
		if frame.Offset.X != 860893 || frame.Offset.Y != 647303 || frame.Extent.CX != 10465073 || frame.Extent.CY != 1532258 {
			t.Errorf("title frame lost reviewed multiline capacity: %+v", frame)
		}
		if title.TextBody.BodyProperties.Anchor != "b" {
			t.Error("short/multiline title grouping no longer bottom-anchored")
		}
		// Decode the percentage attribute directly, independent of the style
		// resolver whose inheritance is exercised by actual generation below.
		var titleLevel struct {
			Level struct {
				Spacing struct {
					Percent struct {
						Value int `xml:"val,attr"`
					} `xml:"spcPct"`
				} `xml:"lnSpc"`
			} `xml:"lvl1pPr"`
		}
		if err := xml.Unmarshal([]byte("<style>"+title.TextBody.ListStyle.Inner+"</style>"), &titleLevel); err != nil {
			t.Fatal(err)
		}
		if titleLevel.Level.Spacing.Percent.Value != 100000 || !strings.Contains(title.TextBody.ListStyle.Inner, `typeface="+mj-lt"`) {
			t.Error("native title font reference or readable line spacing lost")
		}
		bodyNames := []string{"body"}
		if layout == "slideLayout3" {
			bodyNames = append(bodyNames, "body_2")
		}
		for _, name := range bodyNames {
			body, ok := shapes[name]
			if !ok || body.ShapeProperties.Transform == nil || body.TextBody == nil || body.TextBody.ListStyle == nil {
				t.Fatalf("missing native %s", name)
			}
			frame := body.ShapeProperties.Transform
			wantX, wantWidth := int64(860893), int64(10465073)
			if layout == "slideLayout3" {
				wantWidth = 5052536
				if name == "body_2" {
					wantX = 6273430
				}
			}
			if frame.Offset.X != wantX || frame.Offset.Y != 2367643 || frame.Extent.CX != wantWidth || frame.Extent.CY != 3583053 {
				t.Errorf("native %s frame changed: %+v", name, frame)
			}
			if frame.Offset.Y-(title.ShapeProperties.Transform.Offset.Y+title.ShapeProperties.Transform.Extent.CY) < 180000 {
				t.Error("title/body frames have less than 0.5cm clearance")
			}
			var bodyStyles struct {
				Levels []struct {
					XMLName xml.Name
					Margin  int `xml:"marL,attr"`
					Pitch   struct {
						Percent struct {
							Value int `xml:"val,attr"`
						} `xml:"spcPct"`
					} `xml:"lnSpc"`
				} `xml:",any"`
			}
			if err := xml.Unmarshal([]byte("<style>"+body.TextBody.ListStyle.Inner+"</style>"), &bodyStyles); err != nil {
				t.Fatal(err)
			}
			if len(bodyStyles.Levels) != 5 {
				t.Fatal("missing native five-level body spacing overrides")
			}
			for index, level := range bodyStyles.Levels {
				if level.XMLName.Local != fmt.Sprintf("lvl%dpPr", index+1) || level.Pitch.Percent.Value != 110000 {
					t.Error("reviewed body leading lost")
				}
				if layout == "slideLayout3" && index == 1 && level.Margin != 565200 {
					t.Error("balanced two-column child wrapping indent lost")
				}
			}
		}
	}
	compact := func(text string) string { return strings.Join(strings.Fields(text), "") }
	titles := []string{
		"Quarterly operating review",
		"Quarterly operating review and service delivery priorities",
		"Quarterly operating review\nService delivery priorities",
	}
	for _, layout := range []string{"slideLayout3", "slideLayout4"} {
		t.Run(layout, func(t *testing.T) {
			checkFramesAndStyles(t, readShapes(t, source, "ppt/slideLayouts/"+layout+".xml"), layout)
			for index, title := range titles {
				t.Run(fmt.Sprintf("title-case-%d", index), func(t *testing.T) {
					content := []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: title}}
					bodyNames := []string{"body"}
					if layout == "slideLayout3" {
						bodyNames = append(bodyNames, "body_2")
					}
					for column, name := range bodyNames {
						bullets := []string{}
						for row := 0; row < 4; row++ {
							bullet := fmt.Sprintf("C%d-B%02d Service owner confirms the reporting deadline", column+1, row)
							if row == 1 {
								bullet = "\t" + bullet
							}
							bullets = append(bullets, bullet)
						}
						content = append(content, ContentItem{PlaceholderID: name, Type: ContentBullets, Value: bullets})
					}
					output := filepath.Join(t.TempDir(), "typography.pptx")
					_, err := Generate(context.Background(), GenerationRequest{
						TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true,
						Slides: []SlideSpec{{LayoutID: layout, Content: content}}, ValidateOutput: true,
					})
					if err != nil {
						t.Fatal(err)
					}
					shapes := readShapes(t, output, "ppt/slides/slide1.xml")
					checkFramesAndStyles(t, shapes, layout)
					for _, item := range content {
						shape := shapes[item.PlaceholderID]
						if shape.TextBody == nil {
							t.Fatalf("generated %s lost", item.PlaceholderID)
						}
						var text strings.Builder
						for row, paragraph := range shape.TextBody.Paragraphs {
							for _, run := range paragraph.Runs {
								text.WriteString(run.Text)
								if item.PlaceholderID == "title" && run.RunProperties != nil && run.RunProperties.FontSize != "" {
									size, err := strconv.Atoi(run.RunProperties.FontSize)
									if err != nil || size < 3800 || size > 4500 {
										t.Errorf("title shrank below reviewed baseline or exceeded native45pt: %q", run.RunProperties.FontSize)
									}
								}
								if item.PlaceholderID != "title" {
									wantSize := "1800"
									if row == 1 {
										wantSize = "1600"
										if paragraph.Properties == nil || paragraph.Properties.Level == nil || *paragraph.Properties.Level != 1 {
											t.Error("authored child hierarchy lost")
										}
									}
									if run.RunProperties == nil || run.RunProperties.FontSize != wantSize {
										t.Errorf("reviewed native body size changed in %s row%d", item.PlaceholderID, row)
									}
								}
							}
						}
						want := title
						if item.PlaceholderID != "title" {
							want = strings.Join(item.Value.([]string), "")
						}
						if compact(text.String()) != compact(want) {
							t.Errorf("authored %s changed or lost: %q", item.PlaceholderID, text.String())
						}
					}
				})
			}
		})
	}
}
