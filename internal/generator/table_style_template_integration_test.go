package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// A resolver unit test can pass while the generator still sends nil to
// PopulateTableInShape. Exercise the full output path on every available
// template, including the ignored local p-style fixture when present.
func TestGenerateTableUsesTemplateDefaultStyle(t *testing.T) {
	changedDefaults := 0
	for _, templatePath := range testutil.TestTemplatePaths() {
		name := filepath.Base(templatePath)
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(templatePath)
			if err != nil {
				t.Fatal(err)
			}
			layouts, err := template.ParseLayouts(reader)
			if err != nil {
				_ = reader.Close()
				t.Fatal(err)
			}
			layoutID := ""
			for _, layout := range layouts {
				if role, _, _ := template.ClassifyCanonicalRole(&layout); role == template.CanonicalRoleOneContent {
					layoutID = layout.ID
					break
				}
			}
			wantGUID := reader.ResolveTableStyleID(template.TemplateDefaultSentinel)
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if layoutID == "" {
				t.Fatal("template has no One Content layout")
			}
			if wantGUID != types.DefaultTableStyleID {
				changedDefaults++
			}

			output := filepath.Join(t.TempDir(), "table.pptx")
			table := &types.TableSpec{
				Headers: []string{"Region", "Revenue"},
				Rows: [][]types.TableCell{{
					{Content: "North", ColSpan: 1, RowSpan: 1},
					{Content: "$10M", ColSpan: 1, RowSpan: 1},
				}},
				Style: types.TableStyle{UseTableStyle: true, StyleID: template.TemplateDefaultSentinel},
			}
			_, err = Generate(context.Background(), GenerationRequest{
				TemplatePath:          templatePath,
				OutputPath:            output,
				ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{
					LayoutID: layoutID,
					Content:  []ContentItem{{PlaceholderID: "body", Type: ContentTable, Value: table}},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			slide := zipSlideXML(t, &z.Reader, 1)
			if !bytes.Contains(slide, []byte("<a:tableStyleId>"+wantGUID+"</a:tableStyleId>")) {
				t.Errorf("rendered table does not reference template default %s", wantGUID)
			}
		})
	}
	if changedDefaults == 0 {
		t.Fatal("corpus has no template whose default differs from the engine default")
	}
}
