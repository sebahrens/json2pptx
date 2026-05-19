package generator

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// templateSentinelPattern matches "@" sentinels that JSON authors use to opt
// into engine- or template-resolved defaults. These must always be resolved
// before they reach the OOXML output; leaking them produces invalid attribute
// values that strict readers (e.g. LibreOffice) reject.
var templateSentinelPattern = regexp.MustCompile(`@template-[a-zA-Z0-9_-]+`)

// TestGenerate_NoSentinelsInOutput is a regression for the
// table-style-demo bug where "@template-default" leaked through to
// <a:tableStyleId>@template-default</a:tableStyleId> in the generated XML and
// caused LibreOffice to refuse to open the deck. Asserts that:
//
//  1. No "@template-*" sentinel appears in any XML file in the output ZIP.
//  2. ppt/tableStyles.xml is populated with a <a:tblStyle> entry for every
//     referenced style GUID (so the file is self-contained per OOXML schema).
func TestGenerate_NoSentinelsInOutput(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "table-sentinel.pptx")

	tableSpec := &types.TableSpec{
		Headers: []string{"Region", "Revenue", "Growth"},
		Rows: [][]types.TableCell{
			{
				{Content: "North America", ColSpan: 1, RowSpan: 1},
				{Content: "$12.4M", ColSpan: 1, RowSpan: 1},
				{Content: "+8%", ColSpan: 1, RowSpan: 1},
			},
			{
				{Content: "Europe", ColSpan: 1, RowSpan: 1},
				{Content: "$8.7M", ColSpan: 1, RowSpan: 1},
				{Content: "+5%", ColSpan: 1, RowSpan: 1},
			},
		},
		Style: types.TableStyle{
			UseTableStyle: true,
			StyleID:       "@template-default", // The sentinel under test.
			Borders:       "all",
		},
	}

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout2",
				Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: "Table"},
					{PlaceholderID: "body", Type: ContentTable, Value: tableSpec},
				},
			},
		},
	}

	if _, err := Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("open output zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var tableStylesXML string
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		if loc := templateSentinelPattern.FindIndex(data); loc != nil {
			t.Errorf("file %s contains unresolved sentinel %q (offset %d). Context: %q",
				f.Name, data[loc[0]:loc[1]], loc[0],
				snippet(data, loc[0], loc[1]))
		}
		if f.Name == PathTableStyles {
			tableStylesXML = string(data)
		}
	}

	// tableStyles.xml must declare the engine default GUID so the reference
	// from <a:tableStyleId> is satisfied within the file (criterion 2).
	if tableStylesXML == "" {
		t.Fatal("ppt/tableStyles.xml missing from output")
	}
	if !strings.Contains(tableStylesXML, "<a:tblStyle ") {
		t.Errorf("ppt/tableStyles.xml has no <a:tblStyle> entries; got %s", tableStylesXML)
	}
	if !strings.Contains(tableStylesXML, types.DefaultTableStyleID) {
		t.Errorf("ppt/tableStyles.xml missing engine default GUID %s; got %s",
			types.DefaultTableStyleID, tableStylesXML)
	}
}

func snippet(data []byte, start, end int) string {
	const window = 40
	from := start - window
	if from < 0 {
		from = 0
	}
	to := end + window
	if to > len(data) {
		to = len(data)
	}
	return string(data[from:to])
}
