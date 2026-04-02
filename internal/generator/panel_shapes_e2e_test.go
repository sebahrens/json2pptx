package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestPanelNativeShapes_E2E generates a PPTX with a 4-panel columns layout using
// the native OOXML shape pipeline and validates the output structure.
func TestPanelNativeShapes_E2E(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "panel_e2e.pptx")

	slides := []SlideSpec{
		{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{
					PlaceholderID: "title",
					Type:          ContentText,
					Value:         "Panel Layout Test",
				},
				{
					PlaceholderID: "body",
					Type:          ContentDiagram,
					Value: &types.DiagramSpec{
						Type: "panel_layout",
						Data: map[string]any{
							"layout": "columns",
							"panels": []any{
								map[string]any{
									"title": "Strategy",
									"body":  "- Define goals\n- Align teams",
								},
								map[string]any{
									"title": "Operations",
									"body":  "- Streamline processes",
								},
								map[string]any{
									"title": "Technology",
									"body":  "- Modernize stack\n- Cloud migration",
								},
								map[string]any{
									"title": "People",
									"body":  "- Hire talent\n- Upskill teams",
								},
							},
						},
					},
				},
			},
		},
	}

	req := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slides,
		ExcludeTemplateSlides: true,
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", result.SlideCount)
	}

	// Open generated PPTX and read slide1 XML
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("failed to open output PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	slideXML := readZipEntry(t, r, "ppt/slides/slide1.xml")

	// 1. Should contain <p:grpSp> element (native panel group)
	if !bytes.Contains(slideXML, []byte("<p:grpSp>")) {
		t.Error("slide XML should contain <p:grpSp> element for native panel shapes")
	}

	// 2. Group should contain child p:sp elements (headers + bodies = 2 per panel = 8 total)
	spCount := bytes.Count(slideXML, []byte("<p:sp>"))
	// The shape tree also has a root nvGrpSpPr, but child shapes should total 8
	// (4 headers + 4 bodies). Allow for some base shapes from the template.
	if spCount < 8 {
		t.Errorf("expected at least 8 <p:sp> elements (4 headers + 4 bodies), got %d", spCount)
	}

	// 3. Body text should contain <a:p> paragraphs matching input
	slideStr := string(slideXML)
	for _, title := range []string{"Strategy", "Operations", "Technology", "People"} {
		if !strings.Contains(slideStr, title) {
			t.Errorf("slide XML should contain panel title %q", title)
		}
	}

	// 4. Bullet text should be present
	for _, text := range []string{"Define goals", "Align teams", "Streamline processes", "Modernize stack", "Cloud migration", "Hire talent", "Upskill teams"} {
		if !strings.Contains(slideStr, text) {
			t.Errorf("slide XML should contain bullet text %q", text)
		}
	}

	// 5. Should contain bullet character references
	if !strings.Contains(slideStr, `buChar char=`) {
		t.Error("slide XML should contain bullet characters")
	}

	// 6. Should use scheme colors (theme-aware), NOT srgbClr
	if !strings.Contains(slideStr, "schemeClr") {
		t.Error("native panel shapes should use schemeClr for theme-aware colors")
	}
	// Panels should reference accent1 for header fill
	if !strings.Contains(slideStr, `schemeClr val="accent1"`) {
		t.Error("header fill should use schemeClr val=\"accent1\"")
	}
	// Body border should use tx1
	if !strings.Contains(slideStr, `schemeClr val="tx1"`) {
		t.Error("body border should use schemeClr val=\"tx1\"")
	}

	// 7. The generated XML should be well-formed (parseable)
	spTree, err := pptx.ExtractSpTree(slideXML)
	if err != nil {
		t.Errorf("ExtractSpTree failed: %v", err)
	}
	var parsed interface{}
	if err := xml.Unmarshal(spTree, &parsed); err != nil {
		t.Errorf("spTree XML should be well-formed, got parse error: %v", err)
	}

	// 8. Slide rels should exist
	slideRels := readZipEntry(t, r, "ppt/slides/_rels/slide1.xml.rels")
	if len(slideRels) == 0 {
		t.Error("slide1.xml.rels should exist")
	}

	t.Logf("Panel E2E test passed: %d bytes output, %d sp elements", result.FileSize, spCount)
}

// TestPanelNativeShapes_E2E_SinglePanel verifies the pipeline with a single panel.
func TestPanelNativeShapes_E2E_SinglePanel(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "panel_single.pptx")

	slides := []SlideSpec{
		{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{
					PlaceholderID: "title",
					Type:          ContentText,
					Value:         "Single Panel",
				},
				{
					PlaceholderID: "body",
					Type:          ContentDiagram,
					Value: &types.DiagramSpec{
						Type: "panel_layout",
						Data: map[string]any{
							"layout": "columns",
							"panels": []any{
								map[string]any{
									"title": "Overview",
									"body":  "- Key insight one\n- Key insight two\n- Key insight three",
								},
							},
						},
					},
				},
			},
		},
	}

	req := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slides,
		ExcludeTemplateSlides: true,
	}

	result, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer func() { _ = r.Close() }()

	slideXML := readZipEntry(t, r, "ppt/slides/slide1.xml")
	slideStr := string(slideXML)

	// Single panel: 1 group containing 2 child shapes (header + body)
	if !strings.Contains(slideStr, "<p:grpSp>") {
		t.Error("should contain group shape for single panel")
	}

	if !strings.Contains(slideStr, "Overview") {
		t.Error("should contain panel title 'Overview'")
	}

	// Verify well-formed XML
	spTree, err := pptx.ExtractSpTree(slideXML)
	if err != nil {
		t.Fatalf("ExtractSpTree failed: %v", err)
	}
	var parsed interface{}
	if err := xml.Unmarshal(spTree, &parsed); err != nil {
		t.Errorf("spTree should be valid XML: %v", err)
	}

	t.Logf("Single panel E2E passed: %d bytes", result.FileSize)
}

// TestPanelNativeShapes_E2E_DefaultLayout verifies that omitting the layout field
// defaults to "columns" and still triggers native shape generation.
func TestPanelNativeShapes_E2E_DefaultLayout(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "panel_default.pptx")

	slides := []SlideSpec{
		{
			LayoutID: "slideLayout2",
			Content: []ContentItem{
				{
					PlaceholderID: "title",
					Type:          ContentText,
					Value:         "Default Layout",
				},
				{
					PlaceholderID: "body",
					Type:          ContentDiagram,
					Value: &types.DiagramSpec{
						Type: "panel_layout",
						Data: map[string]any{
							// No "layout" field — should default to "columns"
							"panels": []any{
								map[string]any{"title": "Alpha", "body": "- Content A"},
								map[string]any{"title": "Beta", "body": "- Content B"},
							},
						},
					},
				},
			},
		},
	}

	req := GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            outputPath,
		Slides:                slides,
		ExcludeTemplateSlides: true,
	}

	_, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer func() { _ = r.Close() }()

	slideXML := readZipEntry(t, r, "ppt/slides/slide1.xml")

	// Should use native shapes (grpSp), not SVG fallback
	if !bytes.Contains(slideXML, []byte("<p:grpSp>")) {
		t.Error("default layout should trigger native panel shapes (p:grpSp)")
	}
	if !bytes.Contains(slideXML, []byte("Alpha")) {
		t.Error("should contain panel title 'Alpha'")
	}
	if !bytes.Contains(slideXML, []byte("Beta")) {
		t.Error("should contain panel title 'Beta'")
	}
}

// TestPanelNativeShapes_Golden compares generated panel group XML against a
// golden file to detect unintended structural changes.
func TestPanelNativeShapes_Golden(t *testing.T) {
	panels := []nativePanelData{
		{title: "Strategy", body: "- Define goals\n- Align teams"},
		{title: "Operations", body: "- Streamline processes"},
		{title: "Technology", body: "- Modernize stack\n- Cloud migration"},
		{title: "People", body: "- Hire talent\n- Upskill teams"},
	}
	bounds := types.BoundingBox{X: 329610, Y: 2129246, Width: 11850000, Height: 4197531}

	result := generatePanelGroupXML(panels, bounds, 10000)

	goldenPath := "testdata/golden/panel_columns_4.xml"

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(result), 0o644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Log("golden file updated")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		// First run: create the golden file automatically
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(result), 0o644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Log("golden file created; re-run test to validate")
		return
	}

	if result != string(golden) {
		t.Errorf("generated panel XML does not match golden file\n"+
			"to update golden file, run: UPDATE_GOLDEN=1 go test -run TestPanelNativeShapes_Golden\n"+
			"got length=%d, golden length=%d", len(result), len(golden))

		// Show first difference for debugging
		for i := 0; i < len(result) && i < len(golden); i++ {
			if result[i] != golden[i] {
				start := max(0, i-40)
				end := min(len(result), i+40)
				gEnd := min(len(golden), i+40)
				t.Errorf("first diff at byte %d:\n  got:    ...%s...\n  golden: ...%s...",
					i, result[start:end], string(golden[start:gEnd]))
				break
			}
		}
	}

	// Verify golden content is well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal(golden, &parsed); err != nil {
		t.Errorf("golden file should be well-formed XML: %v", err)
	}
}

// readZipEntry reads a file from a zip.ReadCloser by path name.
func readZipEntry(t *testing.T, r *zip.ReadCloser, name string) []byte {
	t.Helper()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open %s: %v", name, err)
			}
			defer func() { _ = rc.Close() }()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}
			return buf.Bytes()
		}
	}
	t.Fatalf("zip entry %q not found", name)
	return nil
}
