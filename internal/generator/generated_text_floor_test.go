package generator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestGeneratedExtremeScaleRefusesBelowEveryRoleFloor(t *testing.T) {
	for _, tc := range []struct{ scale, size, action string }{
		{"20000", "1200", "refuse"},
		{"50000", "1200", "refuse"},
		{"69900", "1000", "refuse"},
		{"70000", "1000", "review"},
		{"80000", "1200", "review"},
	} {
		t.Run(tc.scale+"/"+tc.size, func(t *testing.T) {
			got := readabilityFindingsFor(t, slideWithAutofit(tc.scale, tc.size))
			if len(got) != 1 || got[0].Action != tc.action {
				t.Fatalf("findings=%+v, want action %s", got, tc.action)
			}
		})
	}
}

func TestGeneratedFontFloorIgnoresUnusedAndEmptyRunStyles(t *testing.T) {
	shape := slideWithAutofit("90000", "2400")
	shape = strings.Replace(shape, "<p:txBody>", `<p:txBody><a:lstStyle><a:lvl9pPr><a:defRPr sz="100"/></a:lvl9pPr></a:lstStyle>`, 1)
	shape = strings.Replace(shape, "</p:txBody>", `<a:p><a:r><a:rPr sz="100"/><a:t> </a:t></a:r><a:endParaRPr sz="100"/></a:p></p:txBody>`, 1)
	if got := readabilityFindingsFor(t, shape); len(got) != 0 {
		t.Fatalf("unused styles refused readable populated text: %+v", got)
	}
}

func TestGeneratedFontFloorWithoutExplicitScale(t *testing.T) {
	for _, bodyPr := range []string{
		`<a:bodyPr/>`,
		`<a:bodyPr><a:noAutofit/></a:bodyPr>`,
		`<a:bodyPr><a:normAutofit/></a:bodyPr>`,
		`<a:bodyPr><a:normAutofit lnSpcReduction="0" fontScale="100000"/></a:bodyPr>`,
	} {
		t.Run(bodyPr, func(t *testing.T) {
			for _, size := range []string{"240", "600", "699"} {
				xml := strings.Replace(slideWithAutofit("100000", size), `<a:bodyPr><a:normAutofit fontScale="100000"/></a:bodyPr>`, bodyPr, 1)
				got := readabilityFindingsFor(t, xml)
				if len(got) != 1 || got[0].Action != "refuse" || got[0].Fix != nil {
					t.Fatalf("%s hundredths-of-a-point font bypassed publication guard: %+v", size, got)
				}
			}
			// Unknown automatic shrink is not evidence for rejecting a readable
			// authored size; the bare-scale scan enforces only the universal floor.
			xml := strings.Replace(slideWithAutofit("100000", "700"), `<a:bodyPr><a:normAutofit fontScale="100000"/></a:bodyPr>`, bodyPr, 1)
			for _, f := range readabilityFindingsFor(t, xml) {
				if f.Action == "refuse" {
					t.Fatalf("7pt boundary falsely refused: %+v", f)
				}
			}
		})
	}
}

func TestStoredShapeFontScale(t *testing.T) {
	for _, tc := range []struct {
		xml    string
		scale  int
		stored bool
	}{
		{`<a:noAutofit/>`, 100000, false},
		{`<a:normAutofit/>`, 100000, false},
		{`<a:normAutofit lnSpcReduction="0" fontScale="50000"/>`, 50000, true},
		{`<a:normAutofit fontScale="0"/>`, 0, true},
		{`<a:normAutofit fontScale="999999999999999999999999999999"/>`, 0, true},
	} {
		t.Run(tc.xml, func(t *testing.T) {
			scale, stored := storedShapeFontScale(tc.xml)
			if scale != tc.scale || stored != tc.stored {
				t.Fatalf("got (%d,%v), want (%d,%v)", scale, stored, tc.scale, tc.stored)
			}
		})
	}
}

func TestDirectGeneratorDoesNotPublishExtremeStoredShapeScale(t *testing.T) {
	for _, scale := range []string{"20000", ""} {
		for _, name := range testutil.AllTestTemplateNames() {
			for _, mode := range []string{"off", "warn", "strict"} {
				t.Run(scale+"/"+name+"/"+mode, func(t *testing.T) {
					dir := t.TempDir()
					output := filepath.Join(dir, "deck.pptx")
					if err := os.WriteFile(output, []byte("existing destination"), 0600); err != nil {
						t.Fatal(err)
					}
					raw := slideWithAutofit(scale, "1200")
					if scale == "" {
						raw = slideWithAutofit("", "240")
					}
					request := GenerationRequest{TemplatePath: filepath.Join(testutil.TemplatesDir(), name+".pptx"), OutputPath: output, StrictFit: mode, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout1", RawShapeXML: [][]byte{[]byte(raw)}}}}
					before, err := json.Marshal(request.Slides)
					if err != nil {
						t.Fatal(err)
					}
					result, err := Generate(context.Background(), request)
					var failure *patterns.ValidationError
					if result != nil || !errors.As(err, &failure) || failure.Code != patterns.ErrCodeTextBelowReadableMin || failure.Path != "/slides/0/rendered_shapes/1" {
						t.Fatalf("extreme stored text published or incorrect refusal: %+v %v", result, err)
					}
					if failure.Fix != nil || strings.Contains(failure.Message, "shorten") || !strings.Contains(failure.Message, "preserving groups and any fixed slide count") {
						t.Fatalf("unsafe repair: %+v", failure)
					}
					after, err := json.Marshal(request.Slides)
					if err != nil || string(before) != string(after) {
						t.Fatal("source mutated")
					}
					data, err := os.ReadFile(output)
					if err != nil || string(data) != "existing destination" {
						t.Fatal("destination changed")
					}
					files, err := os.ReadDir(dir)
					if err != nil || len(files) != 1 {
						t.Fatal("temporary output leaked")
					}
				})
			}
		}
	}
}
