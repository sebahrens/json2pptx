package quality

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Refusal is not visual approval: each unchanged adverse input must either
// preserve every required source marker/string or produce no published deck.
func TestEveryNativeProbePreservesSourceOrRefuses(t *testing.T) {
	imagePath := nativeReferenceImage()
	for _, mode := range []string{"", "warn", "off", "strict"} {
		t.Run(fmt.Sprintf("mode=%q", mode), func(t *testing.T) {
			accepted, refused, probes, layoutsCount := 0, 0, 0, 0
			for _, path := range nativeCorpusPaths(t) {
				name := strings.TrimSuffix(filepath.Base(path), ".pptx")
				reader, err := template.OpenTemplate(path)
				if err != nil {
					t.Fatal(err)
				}
				layouts, err := template.ParseLayouts(reader)
				if closeErr := reader.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if err != nil {
					t.Fatal(err)
				}
				layoutsCount += len(layouts)
				for _, layout := range layouts {
					for _, original := range makeNativeProbes(layout, imagePath) {
						p := original
						p.ExpectedText = append([]string(nil), p.ExpectedText...)
						// Markers alone cannot catch a sentence shortened after its ID.
						for _, item := range p.Slide.Content {
							switch item.Type {
							case generator.ContentText:
								if text, ok := item.Value.(string); ok {
									p.ExpectedText = append(p.ExpectedText, text)
								}
							case generator.ContentBullets:
								for _, text := range item.Value.([]string) {
									p.ExpectedText = append(p.ExpectedText, strings.TrimSpace(text))
								}
							case generator.ContentTable:
								table := item.Value.(*types.TableSpec)
								p.ExpectedText = append(p.ExpectedText, table.Headers...)
								for _, row := range table.Rows {
									for _, cell := range row {
										p.ExpectedText = append(p.ExpectedText, cell.Content)
									}
								}
							}
						}
						t.Run(name+"/"+p.ID, func(t *testing.T) {
							probes++
							dir := t.TempDir()
							output := filepath.Join(dir, "deck.pptx")
							result, err := generator.Generate(context.Background(), generator.GenerationRequest{TemplatePath: path, OutputPath: output, StrictFit: mode, Slides: []generator.SlideSpec{p.Slide}, ExcludeTemplateSlides: true, AllowedImagePaths: []string{testutil.RepoRoot()}})
							if err != nil {
								var loss *patterns.ValidationError
								if result != nil || !errors.As(err, &loss) || (loss.Code != patterns.ErrCodeTextTrimmed && loss.Code != patterns.ErrCodeReadabilityTrimmed && loss.Code != patterns.ErrCodeTableRowsTruncated) || loss.Path == "" {
									t.Fatalf("unexpected refusal: result=%+v err=%v", result, err)
								}
								files, readErr := os.ReadDir(dir)
								if readErr != nil || len(files) != 0 {
									t.Fatalf("refusal published or leaked files: %v %v", files, readErr)
								}
								refused++
								t.Logf("REFUSED source-loss=%s path=%s; not visually approved", loss.Code, loss.Path)
								return
							}
							if result == nil || result.SlideCount != 1 || len(result.MediaFailures) != 0 || len(result.ValidationErrors) != 0 {
								t.Fatalf("successful source probe incomplete: %+v", result)
							}
							zr, err := zip.OpenReader(output)
							if err != nil {
								t.Fatal(err)
							}
							defer zr.Close()
							var body []byte
							for _, f := range zr.File {
								if f.Name != "ppt/slides/slide1.xml" {
									continue
								}
								r, err := f.Open()
								if err != nil {
									t.Fatal(err)
								}
								body, err = io.ReadAll(r)
								if err != nil {
									t.Fatal(err)
								}
								if err := r.Close(); err != nil {
									t.Fatal(err)
								}
							}
							if len(body) == 0 {
								t.Fatal("successful slide missing")
							}
							if failures := checkNativeProbeSlide(body, p); len(failures) != 0 {
								t.Fatalf("published source loss: %v", failures)
							}
							accepted++
						})
					}
				}
			}
			t.Logf("templates=%d layouts=%d probes=%d accepted=%d refused=%d; source presence is not visual approval", len(nativeCorpusPaths(t)), layoutsCount, probes, accepted, refused)
			if probes == 0 || accepted == 0 || refused == 0 {
				t.Fatalf("missing positive/adverse coverage: %d/%d/%d", probes, accepted, refused)
			}
		})
	}
}
