package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDirectStrictGeneratorDoesNotPublishTableLoss(t *testing.T) {
	paths := testutil.TestTemplatePaths()
	fixtures, err := filepath.Glob(filepath.Join(testutil.RepoRoot(), "tests/quality/fixtures/portability/templates/*.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, fixtures...)
	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".pptx"), func(t *testing.T) {
			reader, err := template.OpenTemplate(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			layouts, err := template.ParseLayouts(reader)
			if err != nil {
				t.Fatal(err)
			}
			var layoutID string
			var slot types.PlaceholderInfo
			for _, layout := range layouts {
				for _, ph := range layout.Placeholders {
					if ph.Role == types.PlaceholderRoleBody && ph.Bounds.Width*ph.Bounds.Height > slot.Bounds.Width*slot.Bounds.Height {
						layoutID, slot = layout.ID, ph
					}
				}
			}
			if layoutID == "" {
				t.Fatal("template has no body slot for table publication control")
			}
			for _, existing := range []bool{false, true} {
				for _, mode := range []string{"", "warn", "off", "strict"} {
					t.Run(fmt.Sprintf("mode=%q/existing=%v", mode, existing), func(t *testing.T) {
						dir := t.TempDir()
						output := filepath.Join(dir, "deck.pptx")
						if existing {
							if err := os.WriteFile(output, []byte("original-destination"), 0600); err != nil {
								t.Fatal(err)
							}
						}
						rows := make([][]types.TableCell, 80)
						for i := range rows {
							rows[i] = []types.TableCell{{Content: fmt.Sprintf("REQUIRED-R%02d", i), ColSpan: 1, RowSpan: 1}, {Content: "Owner", ColSpan: 1, RowSpan: 1}}
						}
						table := &types.TableSpec{Headers: []string{"Evidence", "Owner"}, Rows: rows, Style: types.DefaultTableStyle}
						req := GenerationRequest{TemplatePath: path, OutputPath: output, StrictFit: mode, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: layoutID, Content: []ContentItem{{PlaceholderID: slot.ID, Type: ContentTable, Value: table}}}}}
						result, err := Generate(context.Background(), req)
						if result != nil || !errors.Is(err, patterns.ErrTableRowsTruncated) {
							t.Fatalf("strict table loss published: result=%+v err=%v", result, err)
						}
						var loss *patterns.ValidationError
						if !errors.As(err, &loss) || loss.Path != "/slides/0/content/0" || loss.Fix == nil || loss.Fix.Kind != "split_at_row" {
							t.Fatalf("missing structured table-loss repair: %v", err)
						}
						files, err := os.ReadDir(dir)
						if err != nil {
							t.Fatal(err)
						}
						if existing {
							data, err := os.ReadFile(output)
							if err != nil || string(data) != "original-destination" || len(files) != 1 {
								t.Fatalf("refusal changed destination or leaked archive: %q %v %v", data, err, files)
							}
						} else if len(files) != 0 {
							t.Fatalf("refusal leaked archive: %v", files)
						}
						if len(table.Rows) != 80 {
							t.Fatalf("generation mutated source rows: %d", len(table.Rows))
						}
						// Fit control must actually retain source in the emitted slide,
						// not merely return a non-nil result with no error.
						table.Rows = rows[:1]
						if result, err := Generate(context.Background(), req); err != nil || result == nil {
							t.Fatalf("fitting table refused: %+v %v", result, err)
						}
						zr, err := zip.OpenReader(output)
						if err != nil {
							t.Fatal(err)
						}
						defer zr.Close()
						var texts []string
						tables := 0
						for _, f := range zr.File {
							if f.Name != "ppt/slides/slide1.xml" {
								continue
							}
							r, err := f.Open()
							if err != nil {
								t.Fatal(err)
							}
							decoder := xml.NewDecoder(r)
							for {
								token, err := decoder.Token()
								if err == io.EOF {
									break
								}
								if err != nil {
									t.Fatal(err)
								}
								if start, ok := token.(xml.StartElement); ok && start.Name.Local == "tbl" && start.Name.Space == "http://schemas.openxmlformats.org/drawingml/2006/main" {
									tables++
								}
								if start, ok := token.(xml.StartElement); ok && start.Name.Local == "t" {
									var text string
									if err := decoder.DecodeElement(&text, &start); err != nil {
										t.Fatal(err)
									}
									texts = append(texts, text)
								}
							}
							if err := r.Close(); err != nil {
								t.Fatal(err)
							}
						}
						if tables != 1 {
							t.Fatalf("fitting table is not one native table: %d", tables)
						}
						for _, required := range []string{"Evidence", "Owner", "REQUIRED-R00"} {
							if !strings.Contains(strings.Join(texts, "\n"), required) {
								t.Fatalf("fitting table source %q absent: %v", required, texts)
							}
						}
					})
				}
			}
		})
	}
}
