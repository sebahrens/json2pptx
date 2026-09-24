package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestDerivedTwoColumnSlidesAcrossTemplateCorpus(t *testing.T) {
	for _, templatePath := range testutil.TestTemplatePaths() {
		name := filepath.Base(templatePath)
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(templatePath)
			if err != nil {
				t.Fatal(err)
			}
			layouts, err := template.ParseLayouts(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			layoutID := ""
			for _, candidate := range layouts {
				if role, _, _ := template.ClassifyCanonicalRole(&candidate); role == template.CanonicalRoleTwoContent {
					layoutID = candidate.ID
					break
				}
			}
			if layoutID == "" {
				t.Fatal("template has no Two Content layout")
			}
			output := filepath.Join(t.TempDir(), "columns.pptx")
			columnContent := []ContentItem{
				{PlaceholderID: "body", Type: ContentText, Value: "Left column"},
				{PlaceholderID: "body_2", Type: ContentText, Value: "Right column"},
			}
			_, err = Generate(context.Background(), GenerationRequest{
				TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{
					{LayoutID: layoutID, ColumnLeftPercent: 65, Content: columnContent},
					{LayoutID: layoutID, ColumnLeftPercent: 35, Content: columnContent},
					{LayoutID: layoutID, Content: columnContent},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			var leftEdges, rightEdges [3]int64
			for slideNum, wantPercent := range []int{65, 35, 0} {
				var slide slideXML
				if err := xml.Unmarshal(zipSlideXML(t, &z.Reader, slideNum+1), &slide); err != nil {
					t.Fatal(err)
				}
				shapes := slide.CommonSlideData.ShapeTree.Shapes
				indices := buildPlaceholderMap(shapes)
				if _, ok := indices["body"]; !ok {
					t.Fatalf("slide %d has no body placeholder", slideNum+1)
				}
				if _, ok := indices["body_2"]; !ok {
					t.Fatalf("slide %d has no body_2 placeholder", slideNum+1)
				}
				left, right := shapes[indices["body"]].ShapeProperties.Transform, shapes[indices["body_2"]].ShapeProperties.Transform
				if left == nil || right == nil {
					t.Fatalf("slide %d lacks resolved body transforms", slideNum+1)
				}
				if left.Offset.X > right.Offset.X {
					left, right = right, left
				}
				leftEdges[slideNum], rightEdges[slideNum] = left.Offset.X, right.Offset.X+right.Extent.CX
				gap := right.Offset.X - left.Offset.X - left.Extent.CX
				if wantPercent != 0 {
					if gap < minTwoColumnGutterEMU {
						t.Errorf("slide %d gap = %d EMU, want >= %d", slideNum+1, gap, minTwoColumnGutterEMU)
					}
					gotPercent := float64(left.Extent.CX) / float64(left.Extent.CX+right.Extent.CX) * 100
					if gotPercent < float64(wantPercent)-0.01 || gotPercent > float64(wantPercent)+0.01 {
						t.Errorf("slide %d left share = %.3f%%, want %d%%", slideNum+1, gotPercent, wantPercent)
					}
				} else {
					if name == "abstract.pptx" || name == "forest-green.pptx" || name == "midnight-blue.pptx" || name == "warm-coral.pptx" {
						if gap < minTwoColumnGutterEMU {
							t.Errorf("native Two Content gap = %d EMU, want >= %d", gap, minTwoColumnGutterEMU)
						}
					}
				}
			}
			for i := 0; i < 2; i++ {
				if leftEdges[i] != leftEdges[2] || rightEdges[i] != rightEdges[2] {
					t.Errorf("slide %d outer edges = %d..%d, native = %d..%d", i+1, leftEdges[i], rightEdges[i], leftEdges[2], rightEdges[2])
				}
			}
		})
	}
}
