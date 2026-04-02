package pipeline

import (
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestEnrichTitleSlides(t *testing.T) {
	tests := []struct {
		name     string
		metadata types.Metadata
		slides   []types.SlideDefinition
		wantBody map[int]string // slideIndex -> expected Body
	}{
		{
			name: "subtitle from frontmatter fills empty title slide",
			metadata: types.Metadata{
				Title:    "My Deck",
				Subtitle: "Strategy & Operations",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
				{Index: 1, Title: "Content", Type: types.SlideTypeContent, Content: types.SlideContent{Body: "some text"}},
			},
			wantBody: map[int]string{0: "Strategy & Operations"},
		},
		{
			name: "author and date compose subtitle when no explicit subtitle",
			metadata: types.Metadata{
				Title:    "My Deck",
				Author:   "John Smith",
				Date:     "March 2026",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
			},
			wantBody: map[int]string{0: "John Smith | March 2026"},
		},
		{
			name: "author only as subtitle",
			metadata: types.Metadata{
				Title:    "My Deck",
				Author:   "Jane Doe",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
			},
			wantBody: map[int]string{0: "Jane Doe"},
		},
		{
			name: "date only as subtitle",
			metadata: types.Metadata{
				Title:    "My Deck",
				Date:     "Q4 2025",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
			},
			wantBody: map[int]string{0: "Q4 2025"},
		},
		{
			name: "explicit subtitle takes precedence over author/date",
			metadata: types.Metadata{
				Title:    "My Deck",
				Subtitle: "Custom Subtitle",
				Author:   "John Smith",
				Date:     "March 2026",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
			},
			wantBody: map[int]string{0: "Custom Subtitle"},
		},
		{
			name: "does not overwrite existing body text without date",
			metadata: types.Metadata{
				Title:    "My Deck",
				Subtitle: "Frontmatter Subtitle",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{Body: "Markdown Subtitle"}},
			},
			wantBody: map[int]string{0: "Markdown Subtitle"},
		},
		{
			name: "appends date to existing body text",
			metadata: types.Metadata{
				Title:    "My Deck",
				Author:   "Strategy & Operations",
				Date:     "2025-12-15",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{Body: "Strategy & Operations"}},
			},
			wantBody: map[int]string{0: "Strategy & Operations | 2025-12-15"},
		},
		{
			name: "does not duplicate date already in body",
			metadata: types.Metadata{
				Title:    "My Deck",
				Date:     "March 2026",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{Body: "Overview | March 2026"}},
			},
			wantBody: map[int]string{0: "Overview | March 2026"},
		},
		{
			name: "enriches multiple title slides",
			metadata: types.Metadata{
				Title:    "My Deck",
				Subtitle: "Overview",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "Opening", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
				{Index: 1, Title: "Content", Type: types.SlideTypeContent, Content: types.SlideContent{}},
				{Index: 2, Title: "Thank You", Type: types.SlideTypeTitle, Content: types.SlideContent{Body: "Q&A"}},
				{Index: 3, Title: "Summary", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
			},
			wantBody: map[int]string{
				0: "Overview",
				2: "Q&A",       // Not overwritten
				3: "Overview",  // Enriched
			},
		},
		{
			name: "no metadata means no enrichment",
			metadata: types.Metadata{
				Title:    "My Deck",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle, Content: types.SlideContent{}},
			},
			wantBody: map[int]string{0: ""},
		},
		{
			name: "content slides are not enriched",
			metadata: types.Metadata{
				Title:    "My Deck",
				Subtitle: "Subtitle",
				Template: "test",
			},
			slides: []types.SlideDefinition{
				{Index: 0, Title: "Content", Type: types.SlideTypeContent, Content: types.SlideContent{}},
			},
			wantBody: map[int]string{0: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pres := &types.PresentationDefinition{
				Metadata: tt.metadata,
				Slides:   tt.slides,
			}

			EnrichTitleSlides(pres)

			for idx, want := range tt.wantBody {
				got := pres.Slides[idx].Content.Body
				if got != want {
					t.Errorf("slide %d Body = %q, want %q", idx, got, want)
				}
			}
		})
	}
}
