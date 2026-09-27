package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/utils"
)

// A copied built-in layout supplies a real package relationship chain. Only
// the explicit disclosure fixture is added; no production template is edited.
func TestDisclosureNativeAlignmentInGeneratedPPTX(t *testing.T) {
	for _, phType := range []string{"subTitle", "body", "obj", ""} {
		for _, alignment := range []string{"ctr", "r"} {
			for _, explicit := range []bool{false, true} {
				for _, id := range []string{"legal_disclosure", "idx:7"} {
					name := fmt.Sprintf("%s-%s-explicit%t-%s", phType, alignment, explicit, strings.ReplaceAll(id, ":", "-"))
					t.Run(name, func(t *testing.T) {
						dir := t.TempDir()
						if exportDir := os.Getenv("DISCLOSURE_ALIGNMENT_ARTIFACT_DIR"); exportDir != "" {
							dir = filepath.Join(exportDir, name)
							if err := os.MkdirAll(dir, 0o755); err != nil {
								t.Fatal(err)
							}
						}
						want := alignment
						paragraph := ""
						if explicit {
							want = map[string]string{"ctr": "r", "r": "ctr"}[alignment]
							paragraph = `<a:pPr algn="` + want + `"/>`
						}
						typeAttribute := ""
						if phType != "" {
							typeAttribute = ` type="` + phType + `"`
						}
						fixture := `<p:sp><p:nvSpPr><p:cNvPr id="99" name="legal_disclosure"/><p:cNvSpPr/><p:nvPr><p:ph idx="7"` + typeAttribute + `/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="500000" y="5600000"/><a:ext cx="11000000" cy="500000"/></a:xfrm></p:spPr><p:txBody><a:bodyPr anchor="b"/><a:lstStyle><a:lvl1pPr algn="` + alignment + `"><a:defRPr sz="1100"/></a:lvl1pPr></a:lstStyle><a:p>` + paragraph + `<a:r><a:t>Native disclosure prompt</a:t></a:r></a:p></p:txBody></p:sp>`
						templatePath := filepath.Join(dir, "template.pptx")
						writeDisclosureAlignmentTemplate(t, templatePath, fixture)
						outputPath := filepath.Join(dir, "generated.pptx")
						const source = "Required disclosure first line\nRequired disclosure second line"
						_, _, err := generateSinglePass(context.Background(), GenerationRequest{
							TemplatePath: templatePath, OutputPath: outputPath, ExcludeTemplateSlides: true,
							Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{
								{PlaceholderID: "title", Type: ContentText, Value: "Disclosure alignment"},
								{PlaceholderID: id, Type: ContentText, Value: source},
							}}},
						})
						if err != nil {
							t.Fatal(err)
						}
						var slide struct {
							Shapes []struct {
								Text struct {
									ListStyle struct {
										Inner string `xml:",innerxml"`
									} `xml:"lstStyle"`
									Paragraphs []struct {
										Properties struct {
											Alignment string `xml:"algn,attr"`
										} `xml:"pPr"`
										Runs []struct {
											Text string `xml:"t"`
										} `xml:"r"`
									} `xml:"p"`
								} `xml:"txBody"`
							} `xml:"cSld>spTree>sp"`
						}
						data := readZipFileString(t, outputPath, "ppt/slides/slide1.xml")
						if err := xml.Unmarshal([]byte(data), &slide); err != nil {
							t.Fatal(err)
						}
						matched := 0
						for _, shape := range slide.Shapes {
							var lines []string
							for _, p := range shape.Text.Paragraphs {
								var line strings.Builder
								for _, r := range p.Runs {
									line.WriteString(r.Text)
								}
								lines = append(lines, line.String())
							}
							if strings.Join(lines, "\n") != source {
								continue
							}
							matched++
							if !strings.Contains(shape.Text.ListStyle.Inner, `algn="`+alignment+`"`) || !strings.Contains(shape.Text.ListStyle.Inner, `sz="1100"`) {
								t.Error("native list alignment or disclosure font lost")
							}
							for _, p := range shape.Text.Paragraphs {
								if got := p.Properties.Alignment; got != "" && got != want {
									t.Errorf("emitted alignment %q overrides native %q", got, want)
								}
								if explicit && p.Properties.Alignment != want {
									t.Error("explicit native paragraph override lost")
								}
							}
						}
						if matched != 1 {
							t.Fatalf("complete disclosure appears %d times, want once", matched)
						}
					})
				}
			}
		}
	}
}

func writeDisclosureAlignmentTemplate(t *testing.T, path, fixture string, layoutRelationships ...string) {
	t.Helper()
	r, err := zip.OpenReader("../../templates/forest-green.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	changed := false
	for _, entry := range r.File {
		if entry.Name == "ppt/slideLayouts/_rels/slideLayout7.xml.rels" && len(layoutRelationships) > 0 {
			data, err := utils.ReadFileFromZip(&r.Reader, entry.Name)
			if err != nil {
				t.Fatal(err)
			}
			out, err := utils.ZipCreateDeterministic(w, entry.Name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := out.Write([]byte(strings.Replace(string(data), "</Relationships>", strings.Join(layoutRelationships, "")+"</Relationships>", 1))); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if entry.Name != "ppt/slideLayouts/slideLayout7.xml" {
			if err := utils.CopyZipFile(w, entry); err != nil {
				t.Fatal(err)
			}
			continue
		}
		data, err := utils.ReadFileFromZip(&r.Reader, entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(data), "</p:spTree>") != 1 {
			t.Fatal("fixture layout shape tree missing or ambiguous")
		}
		out, err := utils.ZipCreateDeterministic(w, entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write([]byte(strings.Replace(string(data), "</p:spTree>", fixture+"</p:spTree>", 1))); err != nil {
			t.Fatal(err)
		}
		changed = true
	}
	if !changed {
		t.Fatal("fixture layout missing")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
