package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
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
			// A missing template style falls back to the engine's consulting
			// default (go-slide-creator-1iiej): an unfilled bold header over a
			// 1pt text-color rule, never a solid header bar.
			ruled := bytes.Contains(frame, []byte(`<a:lnB w="12700" cap="flat" cmpd="sng"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:lnB>`))
			if !defined && (filled || !bold || !ruled) {
				t.Errorf("missing template style did not get the engine-default header: fill=%t bold=%t rule=%t", filled, bold, ruled)
			}
		})
	}
	if changedDefaults == 0 {
		t.Fatal("corpus has no template whose default differs from the engine default")
	}
}

// A template default with no style definition renders the engine-default
// header: unfilled, over a 1pt text-colour rule (go-slide-creator-1iiej).
//
// The test used to compare the default against a header_background "none"
// control and require the two to differ, which held while the fallback was a
// solid header bar. Since 1iiej the default header is itself unfilled, so the
// two slides are the same picture and the comparison could only fail — on any
// host, in the non-short mode CI does not run (go-slide-creator-ip7ll). It
// now reads the render for what the default promises: the header rule is
// drawn, and the slide differs from the same table with a solid accent header.
func TestTemplateDefaultWithoutDefinitionRendersStyledHeader(t *testing.T) {
	if testing.Short() {
		t.Skip("pixel comparison requires LibreOffice")
	}
	if ok, missing := render.DependencyStatus(); !ok {
		t.Skipf("render dependencies unavailable: %v", missing)
	}
	for _, name := range []string{"forest-green", "midnight-blue", "modern-yellow", "warm-coral"} {
		t.Run(name, func(t *testing.T) {
			templatePath := filepath.Join(testutil.TemplatesDir(), name+".pptx")
			layoutID, _, defined := tableTestLayoutAndDefaultStyle(t, templatePath)
			if defined {
				t.Skip("template defines its default table style")
			}
			solid := tableTestSpec("accent1")
			solid.Style.UseTableStyle = false
			output := filepath.Join(t.TempDir(), "header-pair.pptx")
			_, err := Generate(context.Background(), GenerationRequest{
				TemplatePath:          templatePath,
				OutputPath:            output,
				ExcludeTemplateSlides: true,
				Slides: []SlideSpec{
					{LayoutID: layoutID, Content: []ContentItem{{PlaceholderID: "body", Type: ContentTable, Value: tableTestSpec("")}}},
					{LayoutID: layoutID, Content: []ContentItem{{PlaceholderID: "body", Type: ContentTable, Value: solid}}},
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
			styled := decodePNGFile(t, pngs[0])
			control := decodePNGFile(t, pngs[1])
			if styled.Bounds() != control.Bounds() {
				t.Fatalf("rendered slide dimensions differ: %v vs %v", styled.Bounds(), control.Bounds())
			}
			bounds := styled.Bounds()
			changed := 0
			for y := bounds.Dy() / 5; y < bounds.Dy()*4/5; y++ {
				for x := bounds.Dx() / 10; x < bounds.Dx()*9/10; x++ {
					if styled.At(x, y) != control.At(x, y) {
						changed++
					}
				}
			}
			if changed < 2000 {
				t.Errorf("only %d central pixels differ from the solid accent header control; the default header renders as a filled bar", changed)
			}
			t.Logf("%d central pixels differ from the solid control; longest rule %dpx of %dpx", changed, longestRuleRun(styled), bounds.Dx())
			if run, want := longestRuleRun(styled), bounds.Dx()/3; run < want {
				t.Errorf("longest horizontal rule in the render is %dpx, want at least %dpx: the header rule is not drawn", run, want)
			}
		})
	}
}

// longestRuleRun is the longest horizontal run of pixels, in the central
// band of img, that stand clearly apart from the slide background (the colour
// of the band's top-left pixel): a drawn rule. Text never runs a third of the
// slide unbroken, and the 15% hairlines between rows stay under the contrast
// bar.
func longestRuleRun(img image.Image) int {
	b := img.Bounds()
	lum := func(x, y int) int {
		r, g, bl, _ := img.At(x, y).RGBA()
		return int((299*r + 587*g + 114*bl) / 1000 >> 8)
	}
	x0, x1 := b.Min.X+b.Dx()/20, b.Min.X+b.Dx()*19/20
	y0, y1 := b.Min.Y+b.Dy()/5, b.Min.Y+b.Dy()*4/5
	bg := lum(x0, y0)
	best := 0
	for y := y0; y < y1; y++ {
		run := 0
		for x := x0; x < x1; x++ {
			d := lum(x, y) - bg
			if d < 0 {
				d = -d
			}
			if d > 60 {
				run++
				best = max(best, run)
			} else {
				run = 0
			}
		}
	}
	return best
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
