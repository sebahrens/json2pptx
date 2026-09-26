package quality

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

type continuationSource struct{ column, depth int }

type continuationFontProps struct {
	Size *int `xml:"sz,attr"`
}

// Check written runs, not template predictions. Keep native root levels (p-style
// starts at level 2) while independently checking the authored relative depth.
func checkReadableContinuation(body []byte, wanted map[string]continuationSource, nativeLevels map[int]int) error {
	var document struct {
		Shapes []struct {
			Text struct {
				Style struct {
					Levels []struct {
						XMLName xml.Name
						Default continuationFontProps `xml:"defRPr"`
					} `xml:",any"`
				} `xml:"lstStyle"`
				Body struct {
					Fit *struct {
						Scale *int `xml:"fontScale,attr"`
					} `xml:"normAutofit"`
				} `xml:"bodyPr"`
				Paragraphs []struct {
					Props struct {
						Level   int                   `xml:"lvl,attr"`
						Default continuationFontProps `xml:"defRPr"`
					} `xml:"pPr"`
					Runs []struct {
						Props continuationFontProps `xml:"rPr"`
						Text  string                `xml:"t"`
					} `xml:"r"`
				} `xml:"p"`
			} `xml:"txBody"`
		} `xml:"cSld>spTree>sp"`
	}
	if err := xml.Unmarshal(body, &document); err != nil {
		return err
	}
	seen := map[string]int{}
	for _, shape := range document.Shapes {
		scale := 100000
		if shape.Text.Body.Fit != nil && shape.Text.Body.Fit.Scale != nil {
			scale = *shape.Text.Body.Fit.Scale
		}
		for _, paragraph := range shape.Text.Paragraphs {
			var text strings.Builder
			for _, run := range paragraph.Runs {
				text.WriteString(run.Text)
			}
			value := text.String()
			source, ok := wanted[value]
			if !ok {
				continue
			}
			seen[value]++
			for _, run := range paragraph.Runs {
				size := run.Props.Size
				if size == nil {
					size = paragraph.Props.Default.Size
				}
				if size == nil {
					for _, level := range shape.Text.Style.Levels {
						if level.XMLName.Local == fmt.Sprintf("lvl%dpPr", paragraph.Props.Level+1) {
							size = level.Default.Size
							break
						}
					}
				}
				if size == nil {
					for _, level := range shape.Text.Style.Levels {
						if level.XMLName.Local == "defPPr" {
							size = level.Default.Size
							break
						}
					}
				}
				resolved := 0
				if size != nil {
					resolved = *size
				}
				if run.Text != "" && (scale <= 0 || float64(resolved)*float64(scale)/100000 < 1800) {
					return fmt.Errorf("source %q emitted below authored 18pt after autofit (size=%d scale=%d)", value, resolved, scale)
				}
			}
			base := paragraph.Props.Level - source.depth
			if previous, exists := nativeLevels[source.column]; exists && previous != base {
				return fmt.Errorf("source %q changed native root/relative nested level: %d, want %d", value, base, previous)
			}
			nativeLevels[source.column] = base
		}
	}
	for text := range wanted {
		if seen[text] != 1 {
			return fmt.Errorf("complete source %q occurs %d times, want exactly once", text, seen[text])
		}
	}
	return nil
}

func TestReadableContinuationInheritedFontsRespectOverrides(t *testing.T) {
	const paragraph = `<a:p><a:pPr lvl="2"/><a:r><a:rPr/><a:t>Complete source</a:t></a:r></a:p>`
	const inherited = `<p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree><p:sp><p:txBody><a:bodyPr/><a:lstStyle><a:lvl3pPr><a:defRPr sz="1800"/></a:lvl3pPr></a:lstStyle>` + paragraph + `</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`
	wanted := map[string]continuationSource{"Complete source": {0, 0}}
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"native level style", inherited, true},
		{"default paragraph style", strings.ReplaceAll(inherited, "lvl3pPr", "defPPr"), true},
		{"small inherited font", strings.Replace(inherited, `sz="1800"`, `sz="1200"`, 1), false},
		{"paragraph overrides small style", strings.Replace(strings.Replace(inherited, `sz="1800"`, `sz="1200"`, 1), `<a:pPr lvl="2"/>`, `<a:pPr lvl="2"><a:defRPr sz="1800"/></a:pPr>`, 1), true},
		{"small explicit run overrides style", strings.Replace(inherited, `<a:rPr/>`, `<a:rPr sz="1200"/>`, 1), false},
		{"zero explicit run cannot inherit", strings.Replace(inherited, `<a:rPr/>`, `<a:rPr sz="0"/>`, 1), false},
		{"stored autofit affects inherited font", strings.Replace(inherited, `<a:bodyPr/>`, `<a:bodyPr><a:normAutofit fontScale="50000"/></a:bodyPr>`, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkReadableContinuation([]byte(tc.body), wanted, map[int]int{})
			if (err == nil) != tc.valid {
				t.Fatalf("font resolution accepted=%v want=%v; %v", err == nil, tc.valid, err)
			}
		})
	}
}

func TestNativeReadableContinuationsCoverEveryReviewedDenseCase(t *testing.T) {
	root := testutil.RepoRoot()
	raw, err := os.ReadFile(filepath.Join(root, "tests/quality/results/native-candidate-blind-audit-20260926.json"))
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		Remaining []struct{ Template, Layout, Issue string } `json:"remaining_unsafe"`
	}
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, path := range nativeCorpusPaths(t) {
		paths[strings.TrimSuffix(filepath.Base(path), ".pptx")] = path
	}
	cases, tested, pagesChecked, sentences := 0, 0, 0, 0
	seenCases := map[string]bool{}
	for _, target := range audit.Remaining {
		if target.Issue != "dense body readability" {
			continue
		}
		cases++
		key := target.Template + "/" + target.Layout
		if seenCases[key] {
			t.Fatalf("duplicate reviewed case: %s", key)
		}
		seenCases[key] = true
		path, ok := paths[target.Template]
		if !ok {
			if target.Template == "p-style" {
				if _, err := os.Stat(filepath.Join(root, "templates/p-style.pptx")); os.IsNotExist(err) {
					continue
				}
			}
			t.Fatalf("reviewed native template not discovered: %s", target.Template)
		}
		tested++
		t.Run(key, func(t *testing.T) {
			r, err := template.OpenTemplate(path)
			if err != nil {
				t.Fatal(err)
			}
			layouts, err := template.ParseLayouts(r)
			closeErr := r.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("parse native layouts: %v; close: %v", err, closeErr)
			}
			var original *nativeProbe
			for _, layout := range layouts {
				if layout.ID != target.Layout {
					continue
				}
				for _, p := range makeNativeProbes(layout, nativeReferenceImage()) {
					if p.Profile == "dense" {
						copy := p
						original = &copy
					}
				}
			}
			if original == nil {
				t.Fatal("reviewed native dense probe missing")
			}
			before, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			pages, err := nativeBulletContinuations(*original, 3)
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 4 {
				t.Fatalf("reviewed four-page continuation changed: %d", len(pages))
			}
			var slides []generator.SlideSpec
			for i := range pages {
				for j := range pages[i].Slide.Content {
					if pages[i].Slide.Content[j].Type == generator.ContentBullets {
						pages[i].Slide.Content[j].FontSize = 1800
					}
				}
				slides = append(slides, pages[i].Slide)
			}
			for j, item := range original.Slide.Content {
				if item.Type != generator.ContentBullets {
					continue
				}
				var joined []string
				for _, page := range pages {
					joined = append(joined, page.Slide.Content[j].Value.([]string)...)
				}
				if !reflect.DeepEqual(joined, item.Value) {
					t.Fatal("complete source strings reordered, rewritten or omitted")
				}
				sentences += len(joined)
			}
			nativeLevels := map[int]int{}
			var visualParts map[string][]byte
			authoredBefore, err := json.Marshal(slides)
			if err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"", "warn", "off", "strict"} {
				t.Run(fmt.Sprintf("mode=%q", mode), func(t *testing.T) {
					output := filepath.Join(t.TempDir(), "readable.pptx")
					result, err := generator.Generate(context.Background(), generator.GenerationRequest{TemplatePath: path, OutputPath: output, Slides: slides, StrictFit: mode, ExcludeTemplateSlides: true})
					if err != nil || result == nil || result.SlideCount != 4 || len(result.ValidationErrors) != 0 || len(result.MediaFailures) != 0 {
						t.Fatalf("continuation generation incomplete: %+v; %v", result, err)
					}
					authoredAfter, err := json.Marshal(slides)
					if err != nil || !bytes.Equal(authoredBefore, authoredAfter) {
						t.Fatal("generation mutated authored continuation source/composition")
					}
					z, err := zip.OpenReader(output)
					if err != nil {
						t.Fatal(err)
					}
					defer z.Close()
					parts := map[string][]byte{}
					for _, part := range z.File {
						if !strings.HasPrefix(part.Name, "ppt/") {
							continue
						}
						r, err := part.Open()
						if err != nil {
							t.Fatal(err)
						}
						data, err := io.ReadAll(r)
						closeErr := r.Close()
						if err != nil || closeErr != nil {
							t.Fatalf("read written slide: %v; %v", err, closeErr)
						}
						parts[part.Name] = data
					}
					if visualParts == nil {
						visualParts = parts
					} else if !reflect.DeepEqual(visualParts, parts) {
						t.Fatal("renderable slide/layout/master/theme/media payloads differ across fit modes; default-mode review cannot be reused")
					}
					for i, page := range pages {
						body := parts[fmt.Sprintf("ppt/slides/slide%d.xml", i+1)]
						rels := parts[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1)]
						if len(body) == 0 || !bytes.Contains(rels, []byte("/"+target.Layout+".xml")) {
							t.Fatal("native slide/layout relationship lost")
						}
						if failures := checkNativeProbeSlide(body, page); len(failures) != 0 {
							t.Fatal(failures)
						}
						wanted := map[string]continuationSource{}
						for column, item := range page.Slide.Content {
							if item.Type != generator.ContentBullets {
								continue
							}
							for _, value := range item.Value.([]string) {
								depth, text := pptx.BulletIndentDepth(value)
								if _, exists := wanted[text]; exists {
									t.Fatal("duplicate authored sentence")
								}
								wanted[text] = continuationSource{column, depth}
							}
						}
						if len(wanted) == 0 {
							t.Fatal("continuation has no source to measure")
						}
						if err := checkReadableContinuation(body, wanted, nativeLevels); err != nil {
							t.Fatal(err)
						}
						pagesChecked++
					}
				})
			}
			after, err := json.Marshal(original)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("original probe mutated")
			}
		})
	}
	wantCases, wantSentences := 13, 150
	if _, present := paths["p-style"]; present {
		wantCases, wantSentences = 15, 180
	}
	if cases != 15 || tested != wantCases || pagesChecked != 16*wantCases || sentences != wantSentences {
		t.Fatalf("review coverage incomplete: targets=%d tested=%d pages=%d sentences=%d", cases, tested, pagesChecked, sentences)
	}
	t.Logf("checked %d reviewed cases, %d continuation pages across all four fit modes and %d unique full source sentences; font/structure controls, not visual approval", tested, pagesChecked, sentences)
}

func TestReadableContinuationControlsRejectDegradation(t *testing.T) {
	const good = `<p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree><p:sp><p:txBody><a:bodyPr><a:normAutofit/></a:bodyPr><a:p><a:pPr lvl="2"/><a:r><a:rPr sz="1800"/><a:t>Parent retains 12% and all qualifiers</a:t></a:r></a:p><a:p><a:pPr lvl="3"/><a:r><a:rPr sz="1800"/><a:t>Child does not omit adverse evidence</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`
	wanted := map[string]continuationSource{"Parent retains 12% and all qualifiers": {0, 0}, "Child does not omit adverse evidence": {0, 1}}
	if err := checkReadableContinuation([]byte(good), wanted, map[int]int{}); err != nil {
		t.Fatal(err)
	}
	duplicateParent := `<a:p><a:pPr lvl="2"/><a:r><a:rPr sz="1800"/><a:t>Parent retains 12% and all qualifiers</a:t></a:r></a:p>`
	for _, tc := range []struct{ name, body string }{
		{"small written font", strings.Replace(good, `sz="1800"`, `sz="1200"`, 1)},
		{"stored autofit shrink", strings.Replace(good, `<a:normAutofit/>`, `<a:normAutofit fontScale="50000"/>`, 1)},
		{"invalid zero autofit", strings.Replace(good, `<a:normAutofit/>`, `<a:normAutofit fontScale="0"/>`, 1)},
		{"missing run size", strings.Replace(good, `sz="1800"`, "", 1)},
		{"flattened child", strings.Replace(good, `lvl="3"`, `lvl="2"`, 1)},
		{"marker-only source", strings.Replace(good, "Parent retains 12% and all qualifiers", "Parent retains 12%", 1)},
		{"duplicate complete source", strings.Replace(good, `</p:txBody>`, duplicateParent+`</p:txBody>`, 1)},
		{"missing XML", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkReadableContinuation([]byte(tc.body), wanted, map[int]int{}); err == nil {
				t.Fatal("degraded output accepted")
			}
		})
	}
	if err := checkReadableContinuation([]byte(good), wanted, map[int]int{0: 1}); err == nil {
		t.Fatal("changed native base level accepted across pages")
	}
}
