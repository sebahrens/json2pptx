package pipeline

import (
	"testing"

	"github.com/ahrens/go-slide-creator/internal/layout"
	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestSplitParagraphs(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "single line",
			text: "hello world",
			want: []string{"hello world"},
		},
		{
			name: "multiple lines",
			text: "line one\nline two\nline three",
			want: []string{"line one", "line two", "line three"},
		},
		{
			name: "trims whitespace",
			text: "  padded  \n  also padded  ",
			want: []string{"padded", "also padded"},
		},
		{
			name: "empty string",
			text: "",
			want: []string{""},
		},
		{
			name: "blank lines",
			text: "first\n\nsecond",
			want: []string{"first", "", "second"},
		},
		{
			name: "trailing newline",
			text: "hello\n",
			want: []string{"hello", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitParagraphs(tt.text)
			if len(got) != len(tt.want) {
				t.Fatalf("splitParagraphs(%q) returned %d items, want %d", tt.text, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitParagraphs(%q)[%d] = %q, want %q", tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindBodyPlaceholder(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			ID:   "layout-title",
			Name: "Title Slide",
			Placeholders: []types.PlaceholderInfo{
				{ID: "p1", Type: types.PlaceholderTitle},
				{ID: "p2", Type: types.PlaceholderSubtitle},
			},
		},
		{
			ID:   "layout-content",
			Name: "Content Slide",
			Placeholders: []types.PlaceholderInfo{
				{ID: "p1", Type: types.PlaceholderTitle},
				{ID: "p2", Type: types.PlaceholderBody, FontSize: 1800, FontFamily: "Calibri"},
			},
		},
		{
			ID:   "layout-generic",
			Name: "Generic Slide",
			Placeholders: []types.PlaceholderInfo{
				{ID: "p1", Type: types.PlaceholderTitle},
				{ID: "p2", Type: types.PlaceholderContent, FontSize: 2000},
			},
		},
	}

	tests := []struct {
		name     string
		layoutID string
		wantID   string
		wantNil  bool
	}{
		{
			name:     "finds body placeholder",
			layoutID: "layout-content",
			wantID:   "p2",
		},
		{
			name:     "finds content placeholder",
			layoutID: "layout-generic",
			wantID:   "p2",
		},
		{
			name:     "returns nil for layout without body",
			layoutID: "layout-title",
			wantNil:  true,
		},
		{
			name:     "returns nil for unknown layout",
			layoutID: "nonexistent",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findBodyPlaceholder(tt.layoutID, layouts)
			if tt.wantNil {
				if got != nil {
					t.Errorf("findBodyPlaceholder(%q) = %+v, want nil", tt.layoutID, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("findBodyPlaceholder(%q) = nil, want placeholder %q", tt.layoutID, tt.wantID)
			}
			if got.ID != tt.wantID {
				t.Errorf("findBodyPlaceholder(%q).ID = %q, want %q", tt.layoutID, got.ID, tt.wantID)
			}
		})
	}
}

func TestFindBodyPlaceholder_PrefersFirstMatch(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			ID: "multi-body",
			Placeholders: []types.PlaceholderInfo{
				{ID: "p1", Type: types.PlaceholderTitle},
				{ID: "p2", Type: types.PlaceholderBody},
				{ID: "p3", Type: types.PlaceholderContent},
			},
		},
	}

	got := findBodyPlaceholder("multi-body", layouts)
	if got == nil {
		t.Fatal("findBodyPlaceholder returned nil")
	}
	if got.ID != "p2" {
		t.Errorf("findBodyPlaceholder prefers first body/content match; got ID=%q, want p2", got.ID)
	}
}

func TestFindBodyPlaceholder_EmptyLayouts(t *testing.T) {
	got := findBodyPlaceholder("any", nil)
	if got != nil {
		t.Errorf("findBodyPlaceholder with nil layouts = %+v, want nil", got)
	}

	got = findBodyPlaceholder("any", []types.LayoutMetadata{})
	if got != nil {
		t.Errorf("findBodyPlaceholder with empty layouts = %+v, want nil", got)
	}
}

func TestCollectBodyParagraphs(t *testing.T) {
	tests := []struct {
		name     string
		slide    types.SlideDefinition
		mappings []layout.ContentMapping
		want     []string
	}{
		{
			name: "body mapping with text",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					Body: "Hello world",
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body", PlaceholderID: "p2"},
			},
			want: []string{"Hello world"},
		},
		{
			name: "body mapping with multiline text",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					Body: "Line one\nLine two",
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body", PlaceholderID: "p2"},
			},
			want: []string{"Line one", "Line two"},
		},
		{
			name: "body mapping with empty body",
			slide: types.SlideDefinition{
				Content: types.SlideContent{Body: ""},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body", PlaceholderID: "p2"},
			},
			want: nil,
		},
		{
			name: "bullets mapping collects body and bullets",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					Body:    "Intro text",
					Bullets: []string{"bullet one", "bullet two"},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "bullets", PlaceholderID: "p2"},
			},
			want: []string{"Intro text", "bullet one", "bullet two"},
		},
		{
			name: "bullets mapping with bullet groups",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					BulletGroups: []types.BulletGroup{
						{Header: "Group A", Bullets: []string{"a1", "a2"}},
						{Header: "", Bullets: []string{"b1"}},
					},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "bullets", PlaceholderID: "p2"},
			},
			want: []string{"Group A", "a1", "a2", "b1"},
		},
		{
			name: "bullets mapping with body after bullets",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					Bullets:          []string{"item"},
					BodyAfterBullets: "Conclusion text",
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "bullets", PlaceholderID: "p2"},
			},
			want: []string{"item", "Conclusion text"},
		},
		{
			name: "body_and_bullets mapping",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					Body:    "Intro",
					Bullets: []string{"point 1"},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body_and_bullets", PlaceholderID: "p2"},
			},
			want: []string{"Intro", "point 1"},
		},
		{
			name: "unrelated mapping produces no paragraphs",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					Body:    "Some body text",
					Bullets: []string{"item"},
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "title", PlaceholderID: "p1"},
				{ContentField: "image", PlaceholderID: "p3"},
			},
			want: nil,
		},
		{
			name: "empty mappings",
			slide: types.SlideDefinition{
				Content: types.SlideContent{Body: "text"},
			},
			mappings: nil,
			want:     nil,
		},
		{
			name: "multiple body mappings accumulate",
			slide: types.SlideDefinition{
				Content: types.SlideContent{
					Body: "shared body",
				},
			},
			mappings: []layout.ContentMapping{
				{ContentField: "body", PlaceholderID: "p2"},
				{ContentField: "body", PlaceholderID: "p3"},
			},
			want: []string{"shared body", "shared body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectBodyParagraphs(tt.slide, tt.mappings)
			if len(got) != len(tt.want) {
				t.Fatalf("collectBodyParagraphs() returned %d paragraphs %v, want %d %v",
					len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("paragraph[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
