package generator

import "testing"

func TestEarlyPromptCleanupPreservesMappedAndNativeText(t *testing.T) {
	for _, id := range []string{"body", "idx:7"} {
		t.Run(id, func(t *testing.T) {
			idx := 7
			makeShape := func(name, kind, text string) shapeXML {
				return shapeXML{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: name},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: kind}},
					},
					TextBody: &textBodyXML{Paragraphs: []paragraphXML{{Runs: []runXML{{Text: text}}}}},
				}
			}
			mapped := makeShape("body", "body", "Required authored text")
			mapped.NonVisualProperties.NvPr.Placeholder.Index = &idx
			decoration := makeShape("copyright", "", "Native copyright")
			decoration.NonVisualProperties.NvPr.Placeholder = nil
			slide := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{
				mapped, makeShape("subtitle", "subTitle", "Unused prompt"),
				makeShape("footer", "ftr", "Confidential"), decoration,
			}}}}
			content := []ContentItem{{PlaceholderID: id, Type: ContentText, Value: "Required authored text"}}
			clearSlideUnmappedPlaceholders(slide, content)
			clearSlideUnmappedPlaceholders(slide, content) // Final cleanup remains idempotent.
			for i, want := range []string{"Required authored text", "", "Confidential", "Native copyright"} {
				shape := &slide.CommonSlideData.ShapeTree.Shapes[i]
				got := ""
				for _, p := range shape.TextBody.Paragraphs {
					for _, r := range p.Runs {
						got += r.Text
					}
				}
				if got != want {
					t.Errorf("shape %d text = %q, want %q", i, got, want)
				}
			}
		})
	}
}
