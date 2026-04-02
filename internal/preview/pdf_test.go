package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGeneratePDF_NilPresentation(t *testing.T) {
	_, err := GeneratePDF(Request{
		OutputPath: filepath.Join(t.TempDir(), "empty.pdf"),
	})
	if err == nil {
		t.Fatal("expected error for nil presentation")
	}
}

func TestGeneratePDF_EmptySlides(t *testing.T) {
	_, err := GeneratePDF(Request{
		OutputPath:   filepath.Join(t.TempDir(), "empty.pdf"),
		Presentation: &types.PresentationDefinition{},
	})
	if err == nil {
		t.Fatal("expected error for empty slides")
	}
}

func TestGeneratePDF_TitleSlide(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "title.pdf")
	result, err := GeneratePDF(Request{
		OutputPath: outPath,
		Presentation: &types.PresentationDefinition{
			Slides: []types.SlideDefinition{
				{
					Title: "Test Title",
					Type:  types.SlideTypeTitle,
				},
			},
		},
		Theme: &types.ThemeInfo{
			Colors: []types.ThemeColor{
				{Name: "accent1", RGB: "#0064B4"},
			},
		},
	})
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
	if result.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", result.PageCount)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if fi.Size() == 0 {
		t.Error("output file is empty")
	}
}

func TestGeneratePDF_ContentSlide(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "content.pdf")
	result, err := GeneratePDF(Request{
		OutputPath: outPath,
		Presentation: &types.PresentationDefinition{
			Slides: []types.SlideDefinition{
				{
					Title: "Key Findings",
					Type:  types.SlideTypeContent,
					Content: types.SlideContent{
						Body: "Our analysis reveals several important trends.",
						Bullets: []string{
							"Revenue grew 15% year-over-year",
							"Customer acquisition cost decreased by 20%",
							"Net promoter score reached an all-time high of 72",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
	if result.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", result.PageCount)
	}
}

func TestGeneratePDF_MultipleSlides(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "multi.pdf")
	result, err := GeneratePDF(Request{
		OutputPath: outPath,
		Presentation: &types.PresentationDefinition{
			Slides: []types.SlideDefinition{
				{Title: "Slide 1", Type: types.SlideTypeTitle},
				{
					Title: "Slide 2",
					Type:  types.SlideTypeContent,
					Content: types.SlideContent{
						Body: "Some body text.",
					},
				},
				{
					Title: "Slide 3",
					Type:  types.SlideTypeContent,
					Content: types.SlideContent{
						Bullets: []string{"A", "B", "C"},
					},
					Source: "Internal data, Q4 2025",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
	if result.PageCount != 3 {
		t.Errorf("PageCount = %d, want 3", result.PageCount)
	}
}

func TestGeneratePDF_BulletGroups(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "groups.pdf")
	_, err := GeneratePDF(Request{
		OutputPath: outPath,
		Presentation: &types.PresentationDefinition{
			Slides: []types.SlideDefinition{
				{
					Title: "Grouped Bullets",
					Type:  types.SlideTypeContent,
					Content: types.SlideContent{
						Body: "Overview text.",
						BulletGroups: []types.BulletGroup{
							{Header: "Section A", Bullets: []string{"A1", "A2"}},
							{Header: "Section B", Bullets: []string{"B1", "B2", "B3"}},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
}

func TestGeneratePDF_TwoColumn(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "twocol.pdf")
	_, err := GeneratePDF(Request{
		OutputPath: outPath,
		Presentation: &types.PresentationDefinition{
			Slides: []types.SlideDefinition{
				{
					Title: "Comparison",
					Type:  types.SlideTypeTwoColumn,
					Content: types.SlideContent{
						Left:  []string{"Left 1", "Left 2"},
						Right: []string{"Right 1", "Right 2"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
}

func TestWrapText(t *testing.T) {
	lines := wrapText("hello world", nil, 0)
	if len(lines) != 1 {
		t.Errorf("expected 1 line for maxWidth=0, got %d", len(lines))
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input   string
		r, g, b uint8
		wantErr bool
	}{
		{"#FF0000", 255, 0, 0, false},
		{"0064B4", 0, 100, 180, false},
		{"invalid", 0, 0, 0, true},
		{"#FFF", 0, 0, 0, true},
	}

	for _, tt := range tests {
		c, err := parseHexColor(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseHexColor(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseHexColor(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if c.R != tt.r || c.G != tt.g || c.B != tt.b {
			t.Errorf("parseHexColor(%q) = (%d,%d,%d), want (%d,%d,%d)", tt.input, c.R, c.G, c.B, tt.r, tt.g, tt.b)
		}
	}
}
