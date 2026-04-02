package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestBuildContentItems(t *testing.T) {
	tests := []struct {
		name     string
		slide    types.SlideDefinition
		mappings []layout.ContentMapping
		want     []ContentItem
	}{
		{
			name: "empty slide and mappings returns empty items",
			slide: types.SlideDefinition{
				Title: "",
			},
			mappings: []layout.ContentMapping{},
			want:     []ContentItem{},
		},
		{
			name: "title only",
			slide: types.SlideDefinition{
				Title: "Test Title",
			},
			mappings: []layout.ContentMapping{
				{ContentField: "title", PlaceholderID: "title_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "title_1", Type: ContentText, Value: "Test Title"},
			},
		},
		{
			name: "body text without bullets",
			slide: types.SlideDefinition{
				Title: "Test",
				Content: types.SlideContent{
					Body: "This is body text",
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body", PlaceholderID: "body_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "body_1", Type: ContentText, Value: "This is body text"},
			},
		},
		{
			name: "bullets only",
			slide: types.SlideDefinition{
				Title: "Test",
				Content: types.SlideContent{
					Bullets: []string{"Point 1", "Point 2"},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "bullets", PlaceholderID: "content_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "content_1", Type: ContentBullets, Value: []string{"Point 1", "Point 2"}},
			},
		},
		{
			name: "body and bullets same placeholder combines to BodyAndBullets",
			slide: types.SlideDefinition{
				Title: "Test",
				Content: types.SlideContent{
					Body:    "Intro text",
					Bullets: []string{"Point 1", "Point 2"},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body", PlaceholderID: "content_1"},
				{ContentField: "bullets", PlaceholderID: "content_1"},
			},
			want: []ContentItem{
				// Body is skipped when bullets exist for same placeholder
				{PlaceholderID: "content_1", Type: ContentBodyAndBullets, Value: BodyAndBulletsContent{
					Body:    "Intro text",
					Bullets: []string{"Point 1", "Point 2"},
				}},
			},
		},
		{
			name: "body and bullets different placeholders remain separate",
			slide: types.SlideDefinition{
				Title: "Test",
				Content: types.SlideContent{
					Body:    "Intro text",
					Bullets: []string{"Point 1"},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body", PlaceholderID: "body_1"},
				{ContentField: "bullets", PlaceholderID: "content_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "body_1", Type: ContentText, Value: "Intro text"},
				{PlaceholderID: "content_1", Type: ContentBullets, Value: []string{"Point 1"}},
			},
		},
		{
			name: "left and right columns for comparison",
			slide: types.SlideDefinition{
				Title: "Comparison",
				Content: types.SlideContent{
					Left:  []string{"Left 1", "Left 2"},
					Right: []string{"Right 1"},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "left", PlaceholderID: "left_col"},
				{ContentField: "right", PlaceholderID: "right_col"},
			},
			want: []ContentItem{
				{PlaceholderID: "left_col", Type: ContentBullets, Value: []string{"Left 1", "Left 2"}},
				{PlaceholderID: "right_col", Type: ContentBullets, Value: []string{"Right 1"}},
			},
		},
		{
			name: "image content",
			slide: types.SlideDefinition{
				Title: "Image Slide",
				Content: types.SlideContent{
					ImagePath: "/path/to/image.png",
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "image", PlaceholderID: "image_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "image_1", Type: ContentImage, Value: ImageContent{Path: "/path/to/image.png", Alt: "Slide image"}},
			},
		},
		{
			name: "chart content",
			slide: types.SlideDefinition{
				Title: "Chart Slide",
				Content: types.SlideContent{
					DiagramSpec: &types.DiagramSpec{
						Type: "bar_chart",
						Data: map[string]any{"A": 10, "B": 20},
					},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "chart", PlaceholderID: "chart_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "chart_1", Type: ContentDiagram, Value: &types.DiagramSpec{
					Type: "bar_chart",
					Data: map[string]any{"A": 10, "B": 20},
				}},
			},
		},
		{
			name: "infographic content",
			slide: types.SlideDefinition{
				Title: "Infographic Slide",
				Content: types.SlideContent{
					DiagramSpec: &types.DiagramSpec{
						Type: "process",
					},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "infographic", PlaceholderID: "infographic_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "infographic_1", Type: ContentDiagram, Value: &types.DiagramSpec{Type: "process"}},
			},
		},
		{
			name: "unknown content field is skipped",
			slide: types.SlideDefinition{
				Title: "Test",
			},
			mappings: []layout.ContentMapping{
				{ContentField: "unknown_field", PlaceholderID: "placeholder_1"},
			},
			want: []ContentItem{},
		},
		{
			name: "empty string values still create items but empty bullets are skipped",
			slide: types.SlideDefinition{
				Title: "", // Empty title - still creates item with empty string value
				Content: types.SlideContent{
					Body:    "", // Empty body - still creates item with empty string value
					Bullets: []string{}, // Empty bullets - skipped due to len check
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "title", PlaceholderID: "title_1"},
				{ContentField: "body", PlaceholderID: "body_1"},
				{ContentField: "bullets", PlaceholderID: "content_1"},
			},
			want: []ContentItem{
				// Empty strings are non-nil so items are created
				{PlaceholderID: "title_1", Type: ContentText, Value: ""},
				{PlaceholderID: "body_1", Type: ContentText, Value: ""},
				// Empty bullets slice doesn't create an item (len check in implementation)
			},
		},
		{
			name: "mixed content types",
			slide: types.SlideDefinition{
				Title: "Complex Slide",
				Content: types.SlideContent{
					Body:      "Introduction",
					Bullets:   []string{"Point 1"},
					ImagePath: "/image.png",
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "title", PlaceholderID: "title_1"},
				{ContentField: "body", PlaceholderID: "content_1"},
				{ContentField: "bullets", PlaceholderID: "content_1"},
				{ContentField: "image", PlaceholderID: "image_1"},
			},
			want: []ContentItem{
				{PlaceholderID: "title_1", Type: ContentText, Value: "Complex Slide"},
				// Body skipped because bullets share same placeholder
				{PlaceholderID: "content_1", Type: ContentBodyAndBullets, Value: BodyAndBulletsContent{
					Body:    "Introduction",
					Bullets: []string{"Point 1"},
				}},
				{PlaceholderID: "image_1", Type: ContentImage, Value: ImageContent{Path: "/image.png", Alt: "Slide image"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildContentItems(tt.slide, tt.mappings)

			if len(got) != len(tt.want) {
				t.Fatalf("BuildContentItems() returned %d items, want %d\ngot: %+v\nwant: %+v",
					len(got), len(tt.want), got, tt.want)
			}

			for i := range got {
				if got[i].PlaceholderID != tt.want[i].PlaceholderID {
					t.Errorf("item[%d].PlaceholderID = %q, want %q",
						i, got[i].PlaceholderID, tt.want[i].PlaceholderID)
				}
				if got[i].Type != tt.want[i].Type {
					t.Errorf("item[%d].Type = %v, want %v",
						i, got[i].Type, tt.want[i].Type)
				}
				// Compare values based on type
				switch expected := tt.want[i].Value.(type) {
				case string:
					if got[i].Value != expected {
						t.Errorf("item[%d].Value = %v, want %v", i, got[i].Value, expected)
					}
				case []string:
					gotSlice, ok := got[i].Value.([]string)
					if !ok {
						t.Errorf("item[%d].Value is not []string", i)
						continue
					}
					if len(gotSlice) != len(expected) {
						t.Errorf("item[%d].Value length = %d, want %d", i, len(gotSlice), len(expected))
					}
				case ImageContent:
					gotImg, ok := got[i].Value.(ImageContent)
					if !ok {
						t.Errorf("item[%d].Value is not ImageContent", i)
						continue
					}
					if gotImg.Path != expected.Path {
						t.Errorf("item[%d].Value.Path = %q, want %q", i, gotImg.Path, expected.Path)
					}
				case BodyAndBulletsContent:
					gotBAB, ok := got[i].Value.(BodyAndBulletsContent)
					if !ok {
						t.Errorf("item[%d].Value is not BodyAndBulletsContent", i)
						continue
					}
					if gotBAB.Body != expected.Body {
						t.Errorf("item[%d].Value.Body = %q, want %q", i, gotBAB.Body, expected.Body)
					}
					if len(gotBAB.Bullets) != len(expected.Bullets) {
						t.Errorf("item[%d].Value.Bullets length = %d, want %d", i, len(gotBAB.Bullets), len(expected.Bullets))
					}
				case *types.DiagramSpec:
					gotChart, ok := got[i].Value.(*types.DiagramSpec)
					if !ok {
						t.Errorf("item[%d].Value is not *types.DiagramSpec", i)
						continue
					}
					if gotChart.Type != expected.Type {
						t.Errorf("item[%d].Value.Type = %v, want %v", i, gotChart.Type, expected.Type)
					}
				// DiagramSpec comparison already handled above
				}
			}
		})
	}
}

func TestBuildContentItems_SectionTitle(t *testing.T) {
	// Section slides should produce ContentSectionTitle instead of ContentText
	// so the generator preserves the template's large font for section dividers.
	slide := types.SlideDefinition{
		Title: "Performance Overview",
		Type:  types.SlideTypeSection,
	}

	mappings := []layout.ContentMapping{
		{ContentField: "title", PlaceholderID: "body_1"},
	}

	got := BuildContentItems(slide, mappings)

	if len(got) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(got))
	}

	if got[0].Type != ContentSectionTitle {
		t.Errorf("Expected ContentSectionTitle for section slide, got %v", got[0].Type)
	}

	if got[0].Value != "Performance Overview" {
		t.Errorf("Value = %q, want %q", got[0].Value, "Performance Overview")
	}

	// Non-section slides should still use ContentText for titles
	contentSlide := types.SlideDefinition{
		Title: "Regular Title",
		Type:  types.SlideTypeContent,
	}

	gotContent := BuildContentItems(contentSlide, mappings)
	if len(gotContent) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(gotContent))
	}
	if gotContent[0].Type != ContentText {
		t.Errorf("Expected ContentText for content slide, got %v", gotContent[0].Type)
	}
}

func TestBuildContentItems_CombinedColumnsForSinglePlaceholder(t *testing.T) {
	// When a two-column slide falls back to a single-placeholder layout,
	// the mapping uses ContentField="bullets" but the slide has Left/Right
	// instead of Bullets. The builder should merge them into a single bullet list.
	slide := types.SlideDefinition{
		Title: "Side by Side",
		Type:  types.SlideTypeTwoColumn,
		Content: types.SlideContent{
			Left:  []string{"Left 1", "Left 2"},
			Right: []string{"Right 1", "Right 2"},
		},
	}

	mappings := []layout.ContentMapping{
		{ContentField: "title", PlaceholderID: "title_1"},
		{ContentField: "bullets", PlaceholderID: "body_1"},
	}

	got := BuildContentItems(slide, mappings)

	if len(got) != 2 {
		t.Fatalf("Expected 2 items, got %d: %+v", len(got), got)
	}

	// Title
	if got[0].Type != ContentText || got[0].Value != "Side by Side" {
		t.Errorf("item[0] = %v, want title ContentText", got[0])
	}

	// Combined bullets from Left+Right
	if got[1].Type != ContentBullets {
		t.Errorf("item[1].Type = %v, want ContentBullets", got[1])
	}

	combined, ok := got[1].Value.([]string)
	if !ok {
		t.Fatalf("item[1].Value is not []string: %T", got[1].Value)
	}

	want := []string{"Left 1", "Left 2", "Right 1", "Right 2"}
	if len(combined) != len(want) {
		t.Fatalf("combined length = %d, want %d", len(combined), len(want))
	}
	for i := range want {
		if combined[i] != want[i] {
			t.Errorf("combined[%d] = %q, want %q", i, combined[i], want[i])
		}
	}
}

func TestBuildContentItems_TitleBodySharedPlaceholder(t *testing.T) {
	// When title and body both map to the same placeholder (e.g., closing slides
	// where the subtitle is too small and body is routed to the title placeholder),
	// the title item should combine title + body text and the body mapping should
	// be skipped to avoid overwriting.
	slide := types.SlideDefinition{
		Title: "Discussion",
		Type:  types.SlideTypeTitle,
		Content: types.SlideContent{
			Body: "Questions & Next Steps",
		},
	}

	mappings := []layout.ContentMapping{
		{ContentField: "title", PlaceholderID: "title"},
		{ContentField: "body", PlaceholderID: "title"}, // same placeholder
	}

	got := BuildContentItems(slide, mappings)

	if len(got) != 1 {
		t.Fatalf("Expected 1 item (combined), got %d", len(got))
	}

	if got[0].PlaceholderID != "title" {
		t.Errorf("PlaceholderID = %q, want %q", got[0].PlaceholderID, "title")
	}
	if got[0].Type != ContentTitleSlideTitle {
		t.Errorf("Type = %v, want ContentTitleSlideTitle", got[0].Type)
	}
	want := "Discussion\nQuestions & Next Steps"
	if got[0].Value != want {
		t.Errorf("Value = %q, want %q", got[0].Value, want)
	}
}

func TestBuildContentItems_DiagramMapping(t *testing.T) {
	// Test that "diagram" content field maps to DiagramSpec
	slide := types.SlideDefinition{
		Title: "Diagram Slide",
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{
				Type: "flowchart",
			},
		},
	}

	mappings := []layout.ContentMapping{
		{ContentField: "diagram", PlaceholderID: "diagram_1"},
	}

	got := BuildContentItems(slide, mappings)

	if len(got) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(got))
	}

	if got[0].Type != ContentDiagram {
		t.Errorf("Expected ContentDiagram, got %v", got[0].Type)
	}

	if got[0].PlaceholderID != "diagram_1" {
		t.Errorf("PlaceholderID = %q, want %q", got[0].PlaceholderID, "diagram_1")
	}
}

func TestBuildContentItems_SectionBodyUsesContentSectionTitle(t *testing.T) {
	// Section divider body placeholders (e.g., decorative section number "01")
	// should use ContentSectionTitle to preserve the template's large font,
	// not ContentText which caps at 24pt.
	slide := types.SlideDefinition{
		Title: "Performance Overview",
		Type:  types.SlideTypeSection,
		Content: types.SlideContent{
			Body: "01", // Section number set by pipeline
		},
	}

	mappings := []layout.ContentMapping{
		{ContentField: "title", PlaceholderID: "title_1"},
		{ContentField: "body", PlaceholderID: "body_1"},
	}

	got := BuildContentItems(slide, mappings)

	if len(got) != 2 {
		t.Fatalf("Expected 2 items, got %d: %+v", len(got), got)
	}

	// Title should be ContentSectionTitle
	if got[0].Type != ContentSectionTitle {
		t.Errorf("item[0].Type = %v, want ContentSectionTitle", got[0].Type)
	}

	// Body should also be ContentSectionTitle (not ContentText)
	if got[1].Type != ContentSectionTitle {
		t.Errorf("item[1].Type = %v, want ContentSectionTitle for section body", got[1].Type)
	}
	if got[1].Value != "01" {
		t.Errorf("item[1].Value = %q, want %q", got[1].Value, "01")
	}

	// Non-section slide body should still use ContentText
	contentSlide := types.SlideDefinition{
		Title: "Regular",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Body: "Some text",
		},
	}

	gotContent := BuildContentItems(contentSlide, mappings)
	for _, item := range gotContent {
		if item.PlaceholderID == "body_1" && item.Type != ContentText {
			t.Errorf("content slide body Type = %v, want ContentText", item.Type)
		}
	}
}

func TestBuildContentItems_SectionTitleBodySharedPlaceholder(t *testing.T) {
	// template_2's section divider (slideLayout3) has only one placeholder
	// ("Text Placeholder 5"). Both title and body map to it. The builder must
	// combine them so the title text is not lost.
	slide := types.SlideDefinition{
		Title: "Content Type Coverage",
		Type:  types.SlideTypeSection,
		Content: types.SlideContent{
			Body: "01",
		},
	}

	// Both title and body mapped to the same placeholder (single-placeholder layout)
	mappings := []layout.ContentMapping{
		{ContentField: "title", PlaceholderID: "Text Placeholder 5"},
		{ContentField: "body", PlaceholderID: "Text Placeholder 5"},
	}

	got := BuildContentItems(slide, mappings)

	// Should produce exactly 1 combined item (not 2 separate ones)
	if len(got) != 1 {
		t.Fatalf("Expected 1 combined item for shared placeholder, got %d: %+v", len(got), got)
	}

	// Combined item should contain both the title and the section number
	val, ok := got[0].Value.(string)
	if !ok {
		t.Fatalf("Expected string value, got %T", got[0].Value)
	}
	if !strings.Contains(val, "Content Type Coverage") {
		t.Errorf("Combined value %q missing title text", val)
	}
	if !strings.Contains(val, "01") {
		t.Errorf("Combined value %q missing section number", val)
	}
	if got[0].Type != ContentSectionTitle {
		t.Errorf("Type = %v, want ContentSectionTitle", got[0].Type)
	}
}
