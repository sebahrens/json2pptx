package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
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
			layoutID, wantGUID, defined := tableTestLayoutAndDefaultStyle(t, templatePath)
			if wantGUID != types.DefaultTableStyleID {
				changedDefaults++
			}

			output := filepath.Join(t.TempDir(), "table.pptx")
			_, err := Generate(context.Background(), GenerationRequest{
				TemplatePath:          templatePath,
				OutputPath:            output,
				ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{
					LayoutID: layoutID,
					Content:  []ContentItem{{PlaceholderID: "body", Type: ContentTable, Value: tableTestSpec("")}},
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
			frame := tableFrameFromSlide(t, slide)
			filled := bytes.Contains(frame, []byte(`<a:solidFill><a:schemeClr val="accent1"/>`))
			bold := bytes.Contains(frame, []byte(`b="1"`))
			if defined && (filled || bold) {
				t.Errorf("explicit header styling overrides a real template table style: fill=%t bold=%t", filled, bold)
			}
			if !defined && (!filled || !bold) {
				t.Errorf("missing template style left header unstyled: fill=%t bold=%t", filled, bold)
			}
		})
	}
	if changedDefaults == 0 {
		t.Fatal("corpus has no template whose default differs from the engine default")
	}
}

func TestTemplateDefaultWithoutDefinitionRendersFilledHeader(t *testing.T) {
	if ok, missing := render.DependencyStatus(); !ok {
		t.Skipf("render dependencies unavailable: %v", missing)
	}
	for _, name := range []string{"forest-green", "midnight-blue", "modern-yellow", "warm-coral"} {
		t.Run(name, func(t *testing.T) {
			templatePath := filepath.Join(testutil.TemplatesDir(), name+".pptx")
			layoutID, _, _ := tableTestLayoutAndDefaultStyle(t, templatePath)
			output := filepath.Join(t.TempDir(), "header-pair.pptx")
			_, err := Generate(context.Background(), GenerationRequest{
				TemplatePath:          templatePath,
				OutputPath:            output,
				ExcludeTemplateSlides: true,
				Slides: []SlideSpec{
					{LayoutID: layoutID, Content: []ContentItem{{PlaceholderID: "body", Type: ContentTable, Value: tableTestSpec("")}}},
					{LayoutID: layoutID, Content: []ContentItem{{PlaceholderID: "body", Type: ContentTable, Value: tableTestSpec("none")}}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			pngs, cleanup, err := render.DeckPNGs(output, 72, true)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			if len(pngs) != 2 {
				t.Fatalf("rendered %d slides, want 2", len(pngs))
			}
			filled := decodePNGFile(t, pngs[0])
			control := decodePNGFile(t, pngs[1])
			if filled.Bounds() != control.Bounds() {
				t.Fatalf("rendered slide dimensions differ: %v vs %v", filled.Bounds(), control.Bounds())
			}
			bounds := filled.Bounds()
			changed := 0
			for y := bounds.Dy() / 5; y < bounds.Dy()*4/5; y++ {
				for x := bounds.Dx() / 10; x < bounds.Dx()*9/10; x++ {
					if filled.At(x, y) != control.At(x, y) {
						changed++
					}
				}
			}
			if changed < 2000 {
				t.Errorf("only %d central pixels changed when header fill was removed; missing style still renders plain", changed)
			}
		})
	}
}

func TestGenerateExplicitStylePreservesTemplateDefinition(t *testing.T) {
	templatePath := filepath.Join(testutil.TemplatesDir(), "modern-template.pptx")
	layoutID, guid, defined := tableTestLayoutAndDefaultStyle(t, templatePath)
	if !defined {
		t.Fatal("fixture has no style definition")
	}
	spec := tableTestSpec("")
	spec.Style.StyleID = guid
	output := filepath.Join(t.TempDir(), "explicit-style.pptx")
	_, err := Generate(context.Background(), GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            output,
		ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{
			LayoutID: layoutID,
			Content:  []ContentItem{{PlaceholderID: "body", Type: ContentTable, Value: spec}},
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
	frame := tableFrameFromSlide(t, zipSlideXML(t, &z.Reader, 1))
	if bytes.Contains(frame, []byte(`<a:solidFill><a:schemeClr val="accent1"/>`)) || bytes.Contains(frame, []byte(`b="1"`)) {
		t.Error("explicit GUID with a real template definition was replaced by fallback styling")
	}
}

func tableTestSpec(headerBackground string) *types.TableSpec {
	return &types.TableSpec{
		Headers: []string{"Region", "Revenue"},
		Rows: [][]types.TableCell{{
			{Content: "North", ColSpan: 1, RowSpan: 1},
			{Content: "$10M", ColSpan: 1, RowSpan: 1},
		}},
		Style: types.TableStyle{
			UseTableStyle:    true,
			StyleID:          template.TemplateDefaultSentinel,
			HeaderBackground: headerBackground,
		},
	}
}

func tableFrameFromSlide(t *testing.T, slide []byte) []byte {
	t.Helper()
	start := bytes.Index(slide, []byte("<p:graphicFrame>"))
	end := bytes.Index(slide, []byte("</p:graphicFrame>"))
	if start < 0 || end < start {
		t.Fatal("rendered slide has no table graphic frame")
	}
	return slide[start : end+len("</p:graphicFrame>")]
}

func tableTestLayoutAndDefaultStyle(t *testing.T, templatePath string) (string, string, bool) {
	t.Helper()
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range layouts {
		if role, _, _ := template.ClassifyCanonicalRole(&layout); role == template.CanonicalRoleOneContent {
			guid := reader.ResolveTableStyleID(template.TemplateDefaultSentinel)
			return layout.ID, guid, reader.HasTableStyleDefinition(guid)
		}
	}
	t.Fatal("template has no One Content layout")
	return "", "", false
}
