package quality

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestModernClosingPreservesNativeContentWithoutStrayConnector(t *testing.T) {
	path := filepath.Join(testutil.RepoRoot(), "templates", "modern-template.pptx")
	reader, err := template.OpenTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	layouts, err := template.ParseLayouts(reader)
	if err != nil || len(layouts) != 7 {
		t.Fatalf("native layout inventory changed: %d, %v", len(layouts), err)
	}
	body, err := reader.ReadFile("ppt/slideLayouts/slideLayout5.xml")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("<p:cxnSp>")) {
		t.Fatal("Closing must not inherit the isolated caret-like connector")
	}
	for _, required := range []string{`name="Closing"`, `name="title"`, `name="subtitle"`, `type="ctrTitle"`, `type="subTitle"`, `<a:defRPr sz="4500">`, `<a:defRPr sz="1800"/>`} {
		if !bytes.Contains(body, []byte(required)) {
			t.Fatalf("native Closing content/style lost: %s", required)
		}
	}
	if bytes.Count(body, []byte("<p:sp>")) != 2 || bytes.Count(body, []byte("<p:pic>")) != 1 {
		t.Fatal("native title, subtitle or artwork inventory changed")
	}
	foundClosing := false
	for _, layout := range layouts {
		if layout.ID != "slideLayout5" {
			continue
		}
		foundClosing = true
		probes := makeNativeProbes(layout, nativeReferenceImage())
		if len(probes) != 2 || probes[0].Profile != "representative" || probes[1].Profile != "dense" {
			t.Fatal("both original Closing source profiles must remain covered")
		}
		for _, mode := range []string{"", "warn", "off", "strict"} {
			for _, original := range probes {
				t.Run(fmt.Sprintf("mode=%q/%s", mode, original.Profile), func(t *testing.T) {
					p := original
					p.ExpectedText = append([]string(nil), p.ExpectedText...)
					for _, item := range p.Slide.Content {
						text, ok := item.Value.(string)
						if !ok || text == "" {
							t.Fatal("Closing probe must preserve complete nonempty source text")
						}
						p.ExpectedText = append(p.ExpectedText, text)
					}
					output := filepath.Join(t.TempDir(), "deck.pptx")
					result, err := generator.Generate(context.Background(), generator.GenerationRequest{TemplatePath: path, OutputPath: output, StrictFit: mode, Slides: []generator.SlideSpec{p.Slide}, ExcludeTemplateSlides: true})
					if err != nil || result == nil || result.SlideCount != 1 || len(result.ValidationErrors) != 0 || len(result.MediaFailures) != 0 {
						t.Fatalf("Closing generation incomplete: %+v, %v", result, err)
					}
					z, err := zip.OpenReader(output)
					if err != nil {
						t.Fatal(err)
					}
					defer z.Close()
					parts := map[string][]byte{}
					for _, f := range z.File {
						if f.Name != "ppt/slides/slide1.xml" && f.Name != "ppt/slides/_rels/slide1.xml.rels" && f.Name != "ppt/slideLayouts/slideLayout5.xml" {
							continue
						}
						r, err := f.Open()
						if err != nil {
							t.Fatal(err)
						}
						data, err := io.ReadAll(r)
						closeErr := r.Close()
						if err != nil || closeErr != nil {
							t.Fatalf("read %s: %v, %v", f.Name, err, closeErr)
						}
						parts[f.Name] = data
					}
					if len(parts) != 3 || !bytes.Contains(parts["ppt/slides/_rels/slide1.xml.rels"], []byte("/slideLayout5.xml")) {
						t.Fatal("native Closing layout or required output parts missing")
					}
					if failures := checkNativeProbeSlide(parts["ppt/slides/slide1.xml"], p); len(failures) != 0 {
						t.Fatal(failures)
					}
					for _, name := range []string{"ppt/slides/slide1.xml", "ppt/slideLayouts/slideLayout5.xml"} {
						if bytes.Contains(parts[name], []byte("<p:cxnSp>")) {
							t.Fatalf("stray connector reintroduced in %s", name)
						}
					}
				})
			}
		}
	}
	if !foundClosing {
		t.Fatal("native Closing layout was not parsed; profile coverage cannot be skipped")
	}
}
