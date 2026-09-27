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

func TestDirectGeneratorDoesNotPublishExtremeStoredShapeScale(t *testing.T) {
	for _, name := range testutil.AllTestTemplateNames() {
		for _, mode := range []string{"off", "warn", "strict"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				dir := t.TempDir()
				output := filepath.Join(dir, "deck.pptx")
				if err := os.WriteFile(output, []byte("existing destination"), 0600); err != nil {
					t.Fatal(err)
				}
				raw := slideWithAutofit("20000", "1200")
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
