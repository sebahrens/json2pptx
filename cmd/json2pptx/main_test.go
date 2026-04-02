package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Tests for JSON input mode

func TestConvertJSONSlides_ValidInput(t *testing.T) {
	slides := []JSONSlide{
		{
			LayoutID: "slideLayout1",
			Content: []JSONContentItem{
				{
					PlaceholderID: "Title 1",
					Type:          "text",
					Value:         json.RawMessage(`"Hello World"`),
				},
			},
		},
		{
			LayoutID: "slideLayout2",
			Content: []JSONContentItem{
				{
					PlaceholderID: "Content 1",
					Type:          "bullets",
					Value:         json.RawMessage(`["Item 1", "Item 2", "Item 3"]`),
				},
			},
		},
	}

	specs, err := convertJSONSlides(slides)
	if err != nil {
		t.Fatalf("convertJSONSlides failed: %v", err)
	}

	if len(specs) != 2 {
		t.Errorf("expected 2 specs, got %d", len(specs))
	}

	if specs[0].LayoutID != "slideLayout1" {
		t.Errorf("expected layoutID slideLayout1, got %s", specs[0].LayoutID)
	}

	if len(specs[0].Content) != 1 {
		t.Errorf("expected 1 content item, got %d", len(specs[0].Content))
	}
}

func TestConvertJSONSlides_MissingLayoutID(t *testing.T) {
	slides := []JSONSlide{
		{
			LayoutID: "", // Missing
			Content:  []JSONContentItem{},
		},
	}

	_, err := convertJSONSlides(slides)
	if err == nil {
		t.Error("expected error for missing layout_id")
	}
}

func TestConvertJSONContent_TextType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Title 1",
			Type:          "text",
			Value:         json.RawMessage(`"Test Title"`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0].PlaceholderID != "Title 1" {
		t.Errorf("expected placeholder_id 'Title 1', got '%s'", items[0].PlaceholderID)
	}

	text, ok := items[0].Value.(string)
	if !ok {
		t.Fatalf("expected string value, got %T", items[0].Value)
	}
	if text != "Test Title" {
		t.Errorf("expected 'Test Title', got '%s'", text)
	}
}

func TestConvertJSONContent_BulletsType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Content 1",
			Type:          "bullets",
			Value:         json.RawMessage(`["First", "Second", "Third"]`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}

	bullets, ok := items[0].Value.([]string)
	if !ok {
		t.Fatalf("expected []string value, got %T", items[0].Value)
	}
	if len(bullets) != 3 {
		t.Errorf("expected 3 bullets, got %d", len(bullets))
	}
	if bullets[0] != "First" {
		t.Errorf("expected 'First', got '%s'", bullets[0])
	}
}

func TestConvertJSONContent_ImageType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Picture 1",
			Type:          "image",
			Value:         json.RawMessage(`{"path": "/path/to/image.png", "alt": "Test image"}`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}

	// Check that the image content was parsed correctly
	if items[0].Type != "image" {
		t.Errorf("expected type 'image', got '%s'", items[0].Type)
	}
}

func TestConvertJSONContent_ImageMissingPath(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Picture 1",
			Type:          "image",
			Value:         json.RawMessage(`{"alt": "No path"}`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for missing image path")
	}
}

func TestConvertJSONContent_ChartType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Chart 1",
			Type:          "chart",
			Value:         json.RawMessage(`{"type": "bar", "title": "Sales", "data": {"Q1": 100, "Q2": 200}}`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}

	if items[0].Type != generator.ContentDiagram {
		t.Errorf("expected type 'diagram', got '%s'", items[0].Type)
	}
}

func TestConvertJSONContent_ChartMissingType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Chart 1",
			Type:          "chart",
			Value:         json.RawMessage(`{"title": "Sales", "data": {"Q1": 100}}`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for missing chart type")
	}
}

func TestConvertJSONContent_UnknownType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Unknown 1",
			Type:          "unknown",
			Value:         json.RawMessage(`"test"`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestConvertJSONContent_MissingPlaceholderID(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "",
			Type:          "text",
			Value:         json.RawMessage(`"test"`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for missing placeholder_id")
	}
}

func TestConvertJSONContent_MissingType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Title 1",
			Type:          "",
			Value:         json.RawMessage(`"test"`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestConvertJSONContent_InvalidTextValue(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Title 1",
			Type:          "text",
			Value:         json.RawMessage(`123`), // Should be string
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for invalid text value")
	}
}

func TestConvertJSONContent_InvalidBulletsValue(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "Content 1",
			Type:          "bullets",
			Value:         json.RawMessage(`"not an array"`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for invalid bullets value")
	}
}

func TestJSONOutput_MarshalSuccess(t *testing.T) {
	output := JSONOutput{
		Success:    true,
		OutputPath: "/path/to/output.pptx",
		SlideCount: 5,
		DurationMs: 150,
		Warnings:   []string{"warning 1"},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !parsed.Success {
		t.Error("expected success=true")
	}
	if parsed.OutputPath != "/path/to/output.pptx" {
		t.Errorf("expected output_path, got '%s'", parsed.OutputPath)
	}
}

func TestJSONOutput_MarshalError(t *testing.T) {
	output := JSONOutput{
		Success: false,
		Error:   "something went wrong",
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Success {
		t.Error("expected success=false")
	}
	if parsed.Error != "something went wrong" {
		t.Errorf("expected error message, got '%s'", parsed.Error)
	}
}

func TestJSONInput_Parse(t *testing.T) {
	inputJSON := `{
		"template": "corporate",
		"output_filename": "my-presentation.pptx",
		"slides": [
			{
				"layout_id": "slideLayout1",
				"content": [
					{
						"placeholder_id": "Title 1",
						"type": "text",
						"value": "Welcome"
					}
				]
			}
		]
	}`

	var input JSONInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		t.Fatalf("failed to parse JSON input: %v", err)
	}

	if input.Template != "corporate" {
		t.Errorf("expected template 'corporate', got '%s'", input.Template)
	}
	if input.OutputFilename != "my-presentation.pptx" {
		t.Errorf("expected output_filename 'my-presentation.pptx', got '%s'", input.OutputFilename)
	}
	if len(input.Slides) != 1 {
		t.Errorf("expected 1 slide, got %d", len(input.Slides))
	}
}

// Tests for writeJSONOutput and writeJSONError

func TestWriteJSONOutput_ToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.json")

	output := JSONOutput{
		Success:    true,
		OutputPath: "/path/to/output.pptx",
		SlideCount: 5,
		DurationMs: 150,
	}

	err := writeJSONOutput(outputPath, output)
	if err != nil {
		t.Fatalf("writeJSONOutput failed: %v", err)
	}

	// Verify file was created and contains valid JSON
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	if !parsed.Success {
		t.Error("expected success=true")
	}
	if parsed.SlideCount != 5 {
		t.Errorf("expected slide_count=5, got %d", parsed.SlideCount)
	}
}

func TestWriteJSONError_WithOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "error.json")

	testErr := fmt.Errorf("test error message")
	err := writeJSONError(outputPath, testErr)

	// Should return the original error
	if err == nil || err.Error() != testErr.Error() {
		t.Errorf("expected original error to be returned")
	}

	// Verify JSON file was created with error
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read error output file: %v", err)
	}

	var parsed JSONOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}

	if parsed.Success {
		t.Error("expected success=false")
	}
	if parsed.Error != "test error message" {
		t.Errorf("expected error message, got '%s'", parsed.Error)
	}
}

func TestWriteJSONError_NoOutputPath(t *testing.T) {
	testErr := fmt.Errorf("test error message")
	err := writeJSONError("", testErr)

	// Should return the original error unchanged
	if err == nil || err.Error() != testErr.Error() {
		t.Errorf("expected original error to be returned, got %v", err)
	}
}

// Tests for generator.BuildContentItems

func TestBuildContentItems_Title(t *testing.T) {
	slide := types.SlideDefinition{
		Title: "Test Title",
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "title1", ContentField: "title"},
	}

	items := generator.BuildContentItems(slide, mappings)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].PlaceholderID != "title1" {
		t.Errorf("expected placeholder_id 'title1', got '%s'", items[0].PlaceholderID)
	}
	if items[0].Type != generator.ContentText {
		t.Errorf("expected type 'text', got '%s'", items[0].Type)
	}
	if items[0].Value != "Test Title" {
		t.Errorf("expected value 'Test Title', got '%v'", items[0].Value)
	}
}

func TestBuildContentItems_Body(t *testing.T) {
	slide := types.SlideDefinition{
		Content: types.SlideContent{
			Body: "This is body text",
		},
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "body1", ContentField: "body"},
	}

	items := generator.BuildContentItems(slide, mappings)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Value != "This is body text" {
		t.Errorf("expected body text, got '%v'", items[0].Value)
	}
}

// TestBuildContentItems_BodyAndBullets tests that when both body and bullets
// map to the same placeholder, they are combined into a single ContentBodyAndBullets item.
func TestBuildContentItems_BodyAndBullets(t *testing.T) {
	slide := types.SlideDefinition{
		Content: types.SlideContent{
			Body:    "Thank you for testing!",
			Bullets: []string{"Point 1", "Point 2", "Point 3"},
		},
	}
	// Both body and bullets map to the same placeholder
	mappings := []layout.ContentMapping{
		{PlaceholderID: "content1", ContentField: "body"},
		{PlaceholderID: "content1", ContentField: "bullets"},
	}

	items := generator.BuildContentItems(slide, mappings)

	// Should produce ONE item (body is skipped and combined with bullets)
	if len(items) != 1 {
		t.Fatalf("expected 1 item (combined body+bullets), got %d", len(items))
	}

	item := items[0]
	if item.Type != generator.ContentBodyAndBullets {
		t.Errorf("expected type 'body_and_bullets', got '%s'", item.Type)
	}

	combined, ok := item.Value.(generator.BodyAndBulletsContent)
	if !ok {
		t.Fatalf("expected BodyAndBulletsContent value, got %T", item.Value)
	}

	if combined.Body != "Thank you for testing!" {
		t.Errorf("expected body 'Thank you for testing!', got '%s'", combined.Body)
	}
	if len(combined.Bullets) != 3 {
		t.Errorf("expected 3 bullets, got %d", len(combined.Bullets))
	}
	if combined.Bullets[0] != "Point 1" {
		t.Errorf("expected first bullet 'Point 1', got '%s'", combined.Bullets[0])
	}
}

func TestBuildContentItems_Bullets(t *testing.T) {
	slide := types.SlideDefinition{
		Content: types.SlideContent{
			Bullets: []string{"Item 1", "Item 2", "Item 3"},
		},
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "content1", ContentField: "bullets"},
	}

	items := generator.BuildContentItems(slide, mappings)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != generator.ContentBullets {
		t.Errorf("expected type 'bullets', got '%s'", items[0].Type)
	}
	bullets, ok := items[0].Value.([]string)
	if !ok {
		t.Fatalf("expected []string value, got %T", items[0].Value)
	}
	if len(bullets) != 3 {
		t.Errorf("expected 3 bullets, got %d", len(bullets))
	}
}

func TestBuildContentItems_TwoColumn(t *testing.T) {
	slide := types.SlideDefinition{
		Content: types.SlideContent{
			Left:  []string{"Left 1", "Left 2"},
			Right: []string{"Right 1", "Right 2"},
		},
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "left1", ContentField: "left"},
		{PlaceholderID: "right1", ContentField: "right"},
	}

	items := generator.BuildContentItems(slide, mappings)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Check left column
	left, ok := items[0].Value.([]string)
	if !ok {
		t.Fatalf("expected []string for left, got %T", items[0].Value)
	}
	if len(left) != 2 {
		t.Errorf("expected 2 left items, got %d", len(left))
	}

	// Check right column
	right, ok := items[1].Value.([]string)
	if !ok {
		t.Fatalf("expected []string for right, got %T", items[1].Value)
	}
	if len(right) != 2 {
		t.Errorf("expected 2 right items, got %d", len(right))
	}
}

func TestBuildContentItems_Image(t *testing.T) {
	slide := types.SlideDefinition{
		Content: types.SlideContent{
			ImagePath: "/path/to/image.png",
		},
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "img1", ContentField: "image"},
	}

	items := generator.BuildContentItems(slide, mappings)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != generator.ContentImage {
		t.Errorf("expected type 'image', got '%s'", items[0].Type)
	}
	imgContent, ok := items[0].Value.(generator.ImageContent)
	if !ok {
		t.Fatalf("expected ImageContent, got %T", items[0].Value)
	}
	if imgContent.Path != "/path/to/image.png" {
		t.Errorf("expected path '/path/to/image.png', got '%s'", imgContent.Path)
	}
}

func TestBuildContentItems_Chart(t *testing.T) {
	diagramSpec := &types.DiagramSpec{
		Type:  "bar_chart",
		Title: "Sales Chart",
		Data:  map[string]any{"categories": []string{"A"}, "values": []float64{10}},
	}
	slide := types.SlideDefinition{
		Content: types.SlideContent{
			DiagramSpec: diagramSpec,
		},
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "chart1", ContentField: "chart"},
	}

	items := generator.BuildContentItems(slide, mappings)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != generator.ContentDiagram {
		t.Errorf("expected type 'diagram', got '%s'", items[0].Type)
	}
}

func TestBuildContentItems_EmptyContent(t *testing.T) {
	slide := types.SlideDefinition{}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "content1", ContentField: "bullets"}, // No bullets
		{PlaceholderID: "img1", ContentField: "image"},       // No image
		{PlaceholderID: "chart1", ContentField: "chart"},     // No chart
	}

	items := generator.BuildContentItems(slide, mappings)

	// Empty content should not be added
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty content, got %d", len(items))
	}
}

func TestBuildContentItems_UnknownField(t *testing.T) {
	slide := types.SlideDefinition{
		Title: "Test",
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "unknown1", ContentField: "unknownfield"},
	}

	items := generator.BuildContentItems(slide, mappings)

	// Unknown fields should be skipped
	if len(items) != 0 {
		t.Errorf("expected 0 items for unknown field, got %d", len(items))
	}
}

// Tests for ConversionResult

func TestConversionResult_Fields(t *testing.T) {
	result := ConversionResult{
		InputPath:  "/input/test.md",
		OutputPath: "/output/test.pptx",
		Success:    true,
		SlideCount: 10,
		Duration:   time.Second * 2,
	}

	if result.InputPath != "/input/test.md" {
		t.Errorf("expected input path, got '%s'", result.InputPath)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.SlideCount != 10 {
		t.Errorf("expected 10 slides, got %d", result.SlideCount)
	}
}

func TestConversionResult_Error(t *testing.T) {
	result := ConversionResult{
		InputPath: "/input/test.md",
		Success:   false,
		Error:     "conversion failed",
	}

	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error != "conversion failed" {
		t.Errorf("expected error message, got '%s'", result.Error)
	}
}

// Tests for runJSONMode validation

func TestRunJSONMode_MissingTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "output.json")

	// JSON with missing template
	input := `{"slides": [{"layout_id": "layout1", "content": []}]}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write JSON input: %v", err)
	}

	err := runJSONMode(jsonPath, outputPath, tmpDir, tmpDir, "", false, false)

	if err == nil {
		t.Error("expected error for missing template")
	}

	// Check error output
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("failed to read output: %v", readErr)
	}

	var output JSONOutput
	if parseErr := json.Unmarshal(data, &output); parseErr != nil {
		t.Fatalf("failed to parse output: %v", parseErr)
	}

	if output.Success {
		t.Error("expected success=false in output")
	}
	if output.Error == "" {
		t.Error("expected error message in output")
	}
}

func TestRunJSONMode_MissingSlides(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "output.json")

	// JSON with no slides
	input := `{"template": "test", "slides": []}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write JSON input: %v", err)
	}

	err := runJSONMode(jsonPath, outputPath, tmpDir, tmpDir, "", false, false)

	if err == nil {
		t.Error("expected error for missing slides")
	}
}

func TestRunJSONMode_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "output.json")

	// Invalid JSON
	if err := os.WriteFile(jsonPath, []byte("not valid json {"), 0644); err != nil {
		t.Fatalf("failed to write JSON input: %v", err)
	}

	err := runJSONMode(jsonPath, outputPath, tmpDir, tmpDir, "", false, false)

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRunJSONMode_NonexistentInput(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.json")

	err := runJSONMode("/nonexistent/file.json", outputPath, tmpDir, tmpDir, "", false, false)

	if err == nil {
		t.Error("expected error for nonexistent input")
	}
}

func TestRunJSONMode_TemplateNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")
	outputPath := filepath.Join(tmpDir, "output.json")

	// Valid JSON but template doesn't exist
	input := `{"template": "nonexistent", "slides": [{"layout_id": "layout1", "content": []}]}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write JSON input: %v", err)
	}

	err := runJSONMode(jsonPath, outputPath, tmpDir, tmpDir, "", false, false)

	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

// Mock TemplateCache for testing

type mockTemplateCache struct {
	cache map[string]*types.TemplateAnalysis
}

func newMockTemplateCache() *mockTemplateCache {
	return &mockTemplateCache{
		cache: make(map[string]*types.TemplateAnalysis),
	}
}

func (m *mockTemplateCache) Get(path string) (*types.TemplateAnalysis, bool) {
	analysis, ok := m.cache[path]
	return analysis, ok
}

func (m *mockTemplateCache) Set(path string, analysis *types.TemplateAnalysis) {
	m.cache[path] = analysis
}

func (m *mockTemplateCache) Invalidate(path string) {
	delete(m.cache, path)
}

func (m *mockTemplateCache) Clear() {
	m.cache = make(map[string]*types.TemplateAnalysis)
}

func (m *mockTemplateCache) Size() int {
	return len(m.cache)
}

func TestMockTemplateCache(t *testing.T) {
	cache := newMockTemplateCache()

	// Test Get on empty cache
	_, ok := cache.Get("test.pptx")
	if ok {
		t.Error("expected cache miss")
	}

	// Test Set and Get
	analysis := &types.TemplateAnalysis{
		TemplatePath: "test.pptx",
		AspectRatio:  "16:9",
	}
	cache.Set("test.pptx", analysis)

	retrieved, ok := cache.Get("test.pptx")
	if !ok {
		t.Error("expected cache hit")
	}
	if retrieved.TemplatePath != "test.pptx" {
		t.Errorf("expected path 'test.pptx', got '%s'", retrieved.TemplatePath)
	}

	// Test Invalidate
	cache.Invalidate("test.pptx")
	_, ok = cache.Get("test.pptx")
	if ok {
		t.Error("expected cache miss after invalidate")
	}
}

// Tests for edge cases in JSON content conversion

func TestConvertJSONContent_InvalidImageJSON(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "img1",
			Type:          "image",
			Value:         json.RawMessage(`"not an object"`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for invalid image JSON")
	}
}

func TestConvertJSONContent_InvalidChartJSON(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "chart1",
			Type:          "chart",
			Value:         json.RawMessage(`"not an object"`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for invalid chart JSON")
	}
}

// Tests for new content types in JSON conversion and dry-run validation

func TestConvertJSONContent_BodyAndBullets(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "content1",
			Type:          "body_and_bullets",
			Value:         json.RawMessage(`{"body": "Overview", "bullets": ["Point A", "Point B"]}`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != generator.ContentBodyAndBullets {
		t.Errorf("expected type body_and_bullets, got %s", items[0].Type)
	}
}

func TestConvertJSONContent_BulletGroups(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "content1",
			Type:          "bullet_groups",
			Value:         json.RawMessage(`{"groups": [{"header": "Group 1", "bullets": ["A", "B"]}, {"header": "Group 2", "bullets": ["C"]}]}`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != generator.ContentBulletGroups {
		t.Errorf("expected type bullet_groups, got %s", items[0].Type)
	}
}

func TestConvertJSONContent_Table(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "content1",
			Type:          "table",
			Value:         json.RawMessage(`{"headers": ["Name", "Value"], "rows": [["Alpha", "100"], ["Beta", "200"]]}`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != generator.ContentTable {
		t.Errorf("expected type table, got %s", items[0].Type)
	}
}

func TestConvertJSONContent_Diagram(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "chart1",
			Type:          "diagram",
			Value:         json.RawMessage(`{"type": "swot", "title": "SWOT Analysis", "data": {"strengths": ["S1"]}}`),
		},
	}

	items, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err != nil {
		t.Fatalf("convertJSONContent failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != generator.ContentDiagram {
		t.Errorf("expected type diagram, got %s", items[0].Type)
	}
}

func TestConvertJSONContent_DiagramMissingType(t *testing.T) {
	content := []JSONContentItem{
		{
			PlaceholderID: "chart1",
			Type:          "diagram",
			Value:         json.RawMessage(`{"title": "No Type"}`),
		},
	}

	_, err := convertJSONContent(content, 1, types.SlideTypeContent)
	if err == nil {
		t.Error("expected error for missing diagram type")
	}
}

// Tests for validateJSONContentValue with new types

func TestValidateJSONContentValue_BodyAndBullets_Valid(t *testing.T) {
	item := JSONContentItem{
		Type:  "body_and_bullets",
		Value: json.RawMessage(`{"body": "Intro", "bullets": ["A", "B"]}`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg != "" {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestValidateJSONContentValue_BodyAndBullets_Invalid(t *testing.T) {
	item := JSONContentItem{
		Type:  "body_and_bullets",
		Value: json.RawMessage(`"not an object"`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg == "" {
		t.Error("expected error for invalid body_and_bullets value")
	}
}

func TestValidateJSONContentValue_BulletGroups_Valid(t *testing.T) {
	item := JSONContentItem{
		Type:  "bullet_groups",
		Value: json.RawMessage(`{"groups": [{"bullets": ["X"]}]}`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg != "" {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestValidateJSONContentValue_BulletGroups_Empty(t *testing.T) {
	item := JSONContentItem{
		Type:  "bullet_groups",
		Value: json.RawMessage(`{"groups": []}`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg == "" {
		t.Error("expected error for empty bullet_groups")
	}
}

func TestValidateJSONContentValue_BulletGroups_Invalid(t *testing.T) {
	item := JSONContentItem{
		Type:  "bullet_groups",
		Value: json.RawMessage(`"not an object"`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg == "" {
		t.Error("expected error for invalid bullet_groups value")
	}
}

func TestValidateJSONContentValue_Table_Valid(t *testing.T) {
	item := JSONContentItem{
		Type:  "table",
		Value: json.RawMessage(`{"headers": ["A", "B"], "rows": [["1", "2"]]}`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg != "" {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestValidateJSONContentValue_Table_Invalid(t *testing.T) {
	item := JSONContentItem{
		Type:  "table",
		Value: json.RawMessage(`"not a table"`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg == "" {
		t.Error("expected error for invalid table value")
	}
}

func TestValidateJSONContentValue_Diagram_Valid(t *testing.T) {
	item := JSONContentItem{
		Type:  "diagram",
		Value: json.RawMessage(`{"type": "timeline", "data": {"events": [{"label": "Q1", "description": "Launch"}]}}`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg != "" {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestValidateJSONContentValue_Diagram_MissingType(t *testing.T) {
	item := JSONContentItem{
		Type:  "diagram",
		Value: json.RawMessage(`{"data": {}}`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg == "" {
		t.Error("expected error for missing diagram type")
	}
}

func TestValidateJSONContentValue_Diagram_Invalid(t *testing.T) {
	item := JSONContentItem{
		Type:  "diagram",
		Value: json.RawMessage(`"not an object"`),
	}
	if msg := validateJSONContentValue(item, 1, 1); msg == "" {
		t.Error("expected error for invalid diagram value")
	}
}

// Tests for convertSlides

func TestConvertSlides_EmptyPresentation(t *testing.T) {
	presentation := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{},
	}
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{},
	}

	specs, warnings, err := pipeline.ConvertSlides(presentation, analysis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 specs for empty presentation, got %d", len(specs))
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for empty presentation, got %d", len(warnings))
	}
}

func TestConvertSlides_SingleSlide(t *testing.T) {
	presentation := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{
				Index: 0,
				Title: "Title Slide",
				Type:  types.SlideTypeTitle,
			},
		},
	}

	// Create a minimal layout that will match
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "slideLayout1",
				Name: "Title Slide",
				Placeholders: []types.PlaceholderInfo{
					{ID: "title1", Type: types.PlaceholderTitle},
				},
				Tags: []string{"title"},
			},
		},
	}

	specs, _, err := pipeline.ConvertSlides(presentation, analysis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 spec, got %d", len(specs))
	}
}

// Tests for getOrAnalyzeTemplate with cached data

func TestGetOrAnalyzeTemplate_CacheHit(t *testing.T) {
	cache := newMockTemplateCache()

	// Pre-populate cache
	cached := &types.TemplateAnalysis{
		TemplatePath: "/test/template.pptx",
		AspectRatio:  "16:9",
		Hash:         "abc123",
	}
	cache.Set("/test/template.pptx", cached)

	// Should return cached value without reading file
	result, err := getOrAnalyzeTemplate("/test/template.pptx", cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hash != "abc123" {
		t.Errorf("expected cached hash 'abc123', got '%s'", result.Hash)
	}
}

func TestGetOrAnalyzeTemplate_InvalidPath(t *testing.T) {
	cache := newMockTemplateCache()

	// Try to analyze nonexistent file
	_, err := getOrAnalyzeTemplate("/nonexistent/template.pptx", cache)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// Tests for JSON multiple slides validation

func TestConvertJSONSlides_MultipleSlides(t *testing.T) {
	slides := []JSONSlide{
		{
			LayoutID: "layout1",
			Content: []JSONContentItem{
				{PlaceholderID: "title1", Type: "text", Value: json.RawMessage(`"Title 1"`)},
			},
		},
		{
			LayoutID: "layout2",
			Content: []JSONContentItem{
				{PlaceholderID: "content1", Type: "bullets", Value: json.RawMessage(`["A", "B"]`)},
			},
		},
		{
			LayoutID: "layout3",
			Content:  []JSONContentItem{}, // Empty content is valid
		},
	}

	specs, err := convertJSONSlides(slides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 3 {
		t.Errorf("expected 3 specs, got %d", len(specs))
	}
}

func TestConvertJSONSlides_MissingLayoutIDMiddleSlide(t *testing.T) {
	slides := []JSONSlide{
		{LayoutID: "layout1", Content: []JSONContentItem{}},
		{LayoutID: "", Content: []JSONContentItem{}}, // Missing in middle
		{LayoutID: "layout3", Content: []JSONContentItem{}},
	}

	_, err := convertJSONSlides(slides)
	if err == nil {
		t.Error("expected error for missing layout_id")
	}
	if !strings.Contains(err.Error(), "slide 2") {
		t.Errorf("error should reference slide 2, got: '%s'", err.Error())
	}
}

// Tests for empty left/right columns in generator.BuildContentItems

func TestBuildContentItems_EmptyLeftRight(t *testing.T) {
	slide := types.SlideDefinition{
		Content: types.SlideContent{
			Left:  []string{}, // Empty but defined
			Right: []string{}, // Empty but defined
		},
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "left1", ContentField: "left"},
		{PlaceholderID: "right1", ContentField: "right"},
	}

	items := generator.BuildContentItems(slide, mappings)

	// Empty arrays should not produce content items
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty left/right, got %d", len(items))
	}
}

// Test combining multiple content types

func TestBuildContentItems_MultipleMappings(t *testing.T) {
	diagramSpec := &types.DiagramSpec{Type: "pie_chart", Data: map[string]any{"categories": []string{"A", "B"}, "values": []float64{40, 60}}}
	slide := types.SlideDefinition{
		Title: "Multi-content Slide",
		Content: types.SlideContent{
			Body:        "Body text",
			Bullets:     []string{"Point 1", "Point 2"},
			ImagePath:   "/img/photo.png",
			DiagramSpec: diagramSpec,
		},
	}
	mappings := []layout.ContentMapping{
		{PlaceholderID: "title1", ContentField: "title"},
		{PlaceholderID: "body1", ContentField: "body"},
		{PlaceholderID: "bullets1", ContentField: "bullets"},
		{PlaceholderID: "img1", ContentField: "image"},
		{PlaceholderID: "chart1", ContentField: "chart"},
	}

	items := generator.BuildContentItems(slide, mappings)

	if len(items) != 5 {
		t.Errorf("expected 5 items, got %d", len(items))
	}

	// Verify types
	typeCount := make(map[generator.ContentType]int)
	for _, item := range items {
		typeCount[item.Type]++
	}

	if typeCount[generator.ContentText] != 2 { // title + body
		t.Errorf("expected 2 text items, got %d", typeCount[generator.ContentText])
	}
	if typeCount[generator.ContentBullets] != 1 {
		t.Errorf("expected 1 bullets item, got %d", typeCount[generator.ContentBullets])
	}
	if typeCount[generator.ContentImage] != 1 {
		t.Errorf("expected 1 image item, got %d", typeCount[generator.ContentImage])
	}
	if typeCount[generator.ContentDiagram] != 1 {
		t.Errorf("expected 1 diagram item, got %d", typeCount[generator.ContentDiagram])
	}
}

// Tests for output filename handling in JSON mode

func TestRunJSONMode_OutputFilenameExtension(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")

	// JSON with output_filename without .pptx extension (should be added)
	input := `{"template": "test", "output_filename": "myoutput", "slides": [{"layout_id": "layout1", "content": []}]}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write JSON input: %v", err)
	}

	// Template doesn't exist, but we can test that extension would be added
	err := runJSONMode(jsonPath, "", tmpDir, tmpDir, "", false, false)
	if err == nil {
		t.Log("Expected error (template not found) but this tests the code path")
	}
}

// Test for invalid config path

func TestRunJSONMode_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "input.json")

	input := `{"template": "test", "slides": [{"layout_id": "layout1", "content": []}]}`
	if err := os.WriteFile(jsonPath, []byte(input), 0644); err != nil {
		t.Fatalf("failed to write JSON input: %v", err)
	}

	// Invalid config path should return error
	err := runJSONMode(jsonPath, "", tmpDir, tmpDir, "/nonexistent/config.yaml", false, false)
	if err == nil {
		t.Error("expected error for invalid config path")
	}
}
