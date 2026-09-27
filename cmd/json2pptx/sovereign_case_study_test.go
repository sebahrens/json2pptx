package main

import (
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

	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestSovereignCaseStudyContinuationsPreserveCompleteSource(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testutil.RepoRoot(), "examples", "sovereign-ai-strategy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	if len(input.Slides) != 28 {
		t.Fatalf("continuation example has %d slides, want 28", len(input.Slides))
	}
	data, err = os.ReadFile(filepath.Join(testutil.RepoRoot(), "tests", "quality", "source_references", "sovereign-case-study-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var original SlideInput
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatal(err)
	}
	if len(original.Content) != 3 || original.Content[0].TextValue == nil || original.Content[1].BodyAndBulletsValue == nil || original.Content[2].BodyAndBulletsValue == nil {
		t.Fatal("original case-study fixture is incomplete")
	}
	var expected []string
	for _, item := range original.Content[1:] {
		expected = append(expected, item.BodyAndBulletsValue.Bullets...)
	}
	pages := input.Slides[21:25]
	var actual []string
	for i, page := range pages {
		if page.LayoutID != "content" || len(page.Content) != 2 || page.Content[0].TextValue == nil || *page.Content[0].TextValue != *original.Content[0].TextValue {
			t.Fatalf("page %d lost full-width native layout or complete title", i+1)
		}
		body := page.Content[1].BodyAndBulletsValue
		if body == nil || body.Body != original.Content[1+i/2].BodyAndBulletsValue.Body || len(body.Bullets) != 2 || body.TrailingBody != "" {
			t.Fatalf("page %d source grouping changed", i+1)
		}
		actual = append(actual, body.Bullets...)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("case studies were shortened, reordered or omitted")
	}
	for _, name := range testutil.AllTestTemplateNames() {
		t.Run(name, func(t *testing.T) {
			var deck PresentationInput
			data, err := os.ReadFile(filepath.Join(testutil.RepoRoot(), "examples", "sovereign-ai-strategy.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &deck); err != nil {
				t.Fatal(err)
			}
			deck.Template, deck.OutputFilename = name, "case-studies.pptx"
			deck.Slides = deck.Slides[21:25]
			result, cleanup, err := RunPresentation(context.Background(), &deck, RenderOptions{OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off"})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil || result.GenResult == nil || result.GenResult.SlideCount != 4 {
				t.Fatalf("readable native continuations refused: %+v %v", result, err)
			}
			var allTexts []string
			for pageIndex, page := range pages {
				decoder := xml.NewDecoder(strings.NewReader(readZipText(t, result.OutputPath, fmt.Sprintf("ppt/slides/slide%d.xml", pageIndex+1))))
				var texts []string
				for {
					token, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if start, ok := token.(xml.StartElement); ok && start.Name.Local == "t" {
						var text string
						if err := decoder.DecodeElement(&text, &start); err != nil {
							t.Fatal(err)
						}
						texts = append(texts, text)
					}
				}
				joined := strings.Join(texts, "\n")
				body := page.Content[1].BodyAndBulletsValue
				for _, text := range []string{*original.Content[0].TextValue, body.Body} {
					if strings.Count(joined, text) != 1 {
						t.Fatalf("page %d lost or duplicated title/group heading: %q", pageIndex+1, text)
					}
				}
				previous := -1
				for _, text := range body.Bullets {
					position := strings.Index(joined, text)
					if strings.Count(joined, text) != 1 || position <= previous {
						t.Fatalf("source wording missing, duplicate or reordered: %q", text)
					}
					previous = position
				}
				allTexts = append(allTexts, texts...)
			}
			joined := strings.Join(allTexts, "\n")
			for _, text := range expected {
				if strings.Count(joined, text) != 1 {
					t.Fatalf("case study duplicated across continuation pages: %q", text)
				}
			}
		})
	}
}
