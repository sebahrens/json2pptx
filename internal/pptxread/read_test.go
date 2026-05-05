package pptxread

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestReadFile_TemplateOnly(t *testing.T) {
	// Templates have slides (from layout masters) but reading a raw template
	// should still work without errors.
	templatePath := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("template file not available")
	}

	pres, err := ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if pres.SlideCount < 1 {
		t.Errorf("expected at least 1 slide in template, got %d", pres.SlideCount)
	}
	if len(pres.Slides) != pres.SlideCount {
		t.Errorf("slide count mismatch: SlideCount=%d, len(Slides)=%d", pres.SlideCount, len(pres.Slides))
	}
}

func TestReadPackage_NonExistent(t *testing.T) {
	_, err := ReadFile("/nonexistent/path.pptx")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestPlaceholderID(t *testing.T) {
	tests := []struct {
		name     string
		phType   string
		shapeName string
		want     string
	}{
		{"ctrTitle", "ctrTitle", "Title 1", "title"},
		{"title", "title", "Title 1", "title"},
		{"subTitle", "subTitle", "Subtitle 2", "subtitle"},
		{"body", "body", "Content Placeholder 3", "body"},
		{"footer", "ftr", "Footer 4", "footer"},
		{"date", "dt", "Date Placeholder 5", "date"},
		{"slideNum", "sldNum", "Slide Number 6", "slide_number"},
		{"no type fallback", "", "My Shape Name", "my_shape_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ph := &placeholderRef{Type: tt.phType}
			got := placeholderID(tt.shapeName, ph)
			if got != tt.want {
				t.Errorf("placeholderID(%q, %q) = %q, want %q", tt.shapeName, tt.phType, got, tt.want)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name string
		body *textBody
		want string
	}{
		{"nil body", nil, ""},
		{"empty paragraphs", &textBody{Paragraphs: []paragraph{}}, ""},
		{"single run", &textBody{Paragraphs: []paragraph{
			{Runs: []run{{Text: "Hello"}}},
		}}, "Hello"},
		{"multi run", &textBody{Paragraphs: []paragraph{
			{Runs: []run{{Text: "Hello "}, {Text: "World"}}},
		}}, "Hello World"},
		{"multi paragraph", &textBody{Paragraphs: []paragraph{
			{Runs: []run{{Text: "Line 1"}}},
			{Runs: []run{{Text: "Line 2"}}},
		}}, "Line 1\nLine 2"},
		{"skip empty paragraphs", &textBody{Paragraphs: []paragraph{
			{Runs: []run{{Text: "Hello"}}},
			{Runs: []run{}},
			{Runs: []run{{Text: "World"}}},
		}}, "Hello\nWorld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.body)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveLayoutID(t *testing.T) {
	// Use a real template to test layout resolution.
	templatePath := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("template file not available")
	}

	pkg, closer, err := pptx.OpenFile(templatePath)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer closer.Close()

	enum, err := pptx.NewSlideEnumerator(pkg)
	if err != nil {
		t.Fatalf("NewSlideEnumerator: %v", err)
	}

	if enum.Count() == 0 {
		t.Skip("no slides in template")
	}

	info := enum.Slides()[0]
	layoutID := resolveLayoutID(pkg, info.PartPath)
	if layoutID == "" {
		t.Error("expected non-empty layout ID for first slide")
	}
	if !hasPrefix(layoutID, "slideLayout") {
		t.Errorf("expected layout ID to start with 'slideLayout', got %q", layoutID)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
