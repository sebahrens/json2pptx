package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Exercise the real CLI runner and MCP conversion, not just the shared table
// emitter: a missed resolver at either boundary formerly produced bad tables.
func TestTemplateTableInheritanceAcrossGenerationSurfaces(t *testing.T) {
	path := testutil.TableStyleTemplate(t, true)
	table := `{"headers":["Phase","Deliverable","Days"],"rows":[["Week 1","Discovery","8"],["Week 1","Design","9"],["Week 2","Build","10"],["Week 2","Review","11"],["Week 3","Test","12"],["Week 3","Train","13"],["Week 4","Launch","14"],["Week 4","Support","15"]]}`
	grid := `{"columns":1,"rows":[{"cells":[{"table":` + table + `}]}]}`
	title := `{"placeholder_id":"title","type":"text","text_value":"Delivery schedule"}`
	cases := []struct{ name, slide, defaults string }{
		{"placeholder", `{"slide_type":"content","content":[` + title + `,{"placeholder_id":"body","type":"table","table_value":` + table + `}]}`, ``},
		{"grid", `{"slide_type":"content","content":[` + title + `],"shape_grid":` + grid + `}`, ``},
		{"nested", `{"slide_type":"content","content":[` + title + `],"shape_grid":{"columns":1,"rows":[{"cells":[{"grid":` + grid + `}]}]}}`, ``},
		{"deck defaults", `{"slide_type":"content","content":[` + title + `],"shape_grid":` + grid + `}`, `,"defaults":{"table_style":{"style_id":"@template-default","use_table_style":true}}`},
		{"explicit brand", `{"slide_type":"content","content":[` + title + `,{"placeholder_id":"body","type":"table","table_value":` + strings.TrimSuffix(table, `}`) + `,"style":{"style_id":"` + testutil.BrandTableStyleID + `","use_table_style":true}}}]}`, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"template":"brand-table","output_filename":"table.pptx","slides":[%s]%s}`, tc.slide, tc.defaults)
			var input PresentationInput
			if err := json.Unmarshal([]byte(raw), &input); err != nil {
				t.Fatal(err)
			}
			applyDefaults(&input)
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input.json")
			if err := os.WriteFile(inputPath, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			validation := validateJSONFile(inputPath, filepath.Dir(path), "", true, "strict")
			if !validation.Valid {
				t.Fatalf("validation rejects brand table: %v", validation.Errors)
			}
			result, cleanup, err := RunPresentation(context.Background(), &input, RenderOptions{OutputDir: dir, TemplatesDir: filepath.Dir(path), StrictFit: "strict", OutputValidation: "strict"})
			defer cleanup()
			if err != nil {
				t.Fatalf("runner: %v", err)
			}
			assertBrandTableOutput(t, result.OutputPath, 9)
			var deck map[string]any
			if err := json.Unmarshal([]byte(raw), &deck); err != nil {
				t.Fatal(err)
			}
			mc := testMCPConfig(t)
			mc.templatesDir = filepath.Dir(path)
			answer, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck, "strict_fit": "strict", "fit_report": true}))
			if err != nil {
				t.Fatal(err)
			}
			if answer.IsError {
				t.Fatalf("MCP: %s", textContent(answer))
			}
			var output JSONOutput
			if err := json.Unmarshal([]byte(textContent(answer)), &output); err != nil {
				t.Fatal(err)
			}
			assertBrandTableOutput(t, output.OutputPath, 9)
		})
	}
}

func assertBrandTableOutput(t *testing.T, path string, wantRows int) {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	parts := map[string]string{}
	for _, f := range archive.File {
		if f.Name != "ppt/slides/slide1.xml" && f.Name != "ppt/tableStyles.xml" {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[f.Name] = string(data)
	}
	slide := parts["ppt/slides/slide1.xml"]
	start := strings.Index(slide, "<p:graphicFrame>")
	end := strings.Index(slide, "</p:graphicFrame>")
	if start < 0 || end < start {
		t.Fatal("no table")
	}
	frame := slide[start:end]
	if !strings.Contains(frame, `<a:tableStyleId>`+testutil.BrandTableStyleID+`</a:tableStyleId>`) {
		t.Fatal("template default was not used")
	}
	for _, masked := range []string{`<a:ln`, `<a:noFill`, `<a:solidFill`, `b="0"`, `b="1"`, `sz="1800"`} {
		if strings.Contains(frame, masked) {
			t.Errorf("style masked by %s", masked)
		}
	}
	if !strings.Contains(frame, `sz="1200"`) || !strings.Contains(frame, `sz="1100"`) {
		t.Error("table typography changed")
	}
	if strings.Count(frame, "<a:tr ") != wantRows {
		t.Errorf("table lost rows: %d", strings.Count(frame, "<a:tr "))
	}
	if !strings.Contains(parts["ppt/tableStyles.xml"], `styleName="Brand Table 3"`) || !strings.Contains(parts["ppt/tableStyles.xml"], `<a:firstRow>`) {
		t.Error("brand definition lost")
	}
}

func TestGridTemplateDefaultWithoutDefinitionUsesFallback(t *testing.T) {
	path := testutil.TableStyleTemplate(t, false)
	var input PresentationInput
	raw := `{"template":"brand-table","output_filename":"fallback.pptx","slides":[{"slide_type":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Schedule"}],"shape_grid":{"columns":1,"rows":[{"cells":[{"table":{"headers":["Phase","Days"],"rows":[["Discovery","8"],["Build","12"]]}}]}]}}]}`
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatal(err)
	}
	result, cleanup, err := RunPresentation(context.Background(), &input, RenderOptions{TemplatesDir: filepath.Dir(path), OutputDir: t.TempDir(), StrictFit: "strict", OutputValidation: "strict"})
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	slide := slideXMLParts(t, result.OutputPath)["ppt/slides/slide1.xml"]
	if !strings.Contains(slide, `<a:tableStyleId>`+testutil.BrandTableStyleID+`</a:tableStyleId>`) || !strings.Contains(slide, `<a:lnB w="12700"`) || !strings.Contains(slide, `sz="1200"`) || !strings.Contains(slide, `sz="1100" b="1"`) {
		t.Fatal("undefined grid style did not get compact readable fallback")
	}
}
