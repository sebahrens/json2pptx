package pagination

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func boolPtr(b bool) *bool { return &b }

func TestPaginate_EnabledByDefault(t *testing.T) {
	bullets := make([]string, 20)
	for i := range bullets {
		bullets[i] = "bullet"
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"},
		Slides: []types.SlideDefinition{
			{Title: "Big Slide", Content: types.SlideContent{
				Bullets: bullets,
			}},
		},
	}

	warnings := Paginate(pres)
	// With default (nil) autopaginate, pagination should run
	if len(pres.Slides) < 2 {
		t.Errorf("expected pagination to split overflowing slide by default, got %d slides", len(pres.Slides))
	}
	if len(warnings) == 0 {
		t.Error("expected pagination warnings by default")
	}
}

func TestPaginate_ExplicitlyDisabled(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test", Autopaginate: boolPtr(false)},
		Slides: []types.SlideDefinition{
			{Title: "Big Slide", Content: types.SlideContent{
				Bullets: make([]string, 20),
			}},
		},
	}

	warnings := Paginate(pres)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings when autopaginate is explicitly false, got %v", warnings)
	}
	if len(pres.Slides) != 1 {
		t.Errorf("expected 1 slide when autopaginate is explicitly false, got %d", len(pres.Slides))
	}
}

func TestPaginate_SplitsFlatBullets(t *testing.T) {
	bullets := make([]string, 20)
	for i := range bullets {
		bullets[i] = "Bullet point"
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test", Autopaginate: boolPtr(true)},
		Slides: []types.SlideDefinition{
			{
				Title:   "Market Analysis",
				Type:    types.SlideTypeContent,
				Content: types.SlideContent{Bullets: bullets, Body: "Introduction text"},
			},
		},
	}

	warnings := Paginate(pres)

	// 20 bullets / 8 per page = 3 pages
	if len(pres.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(pres.Slides))
	}

	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}

	// Check titles
	tests := []struct {
		idx           int
		expectedTitle string
		expectedCount int
	}{
		{0, "Market Analysis (1/3)", 8},
		{1, "Market Analysis (2/3)", 8},
		{2, "Market Analysis (3/3)", 4},
	}

	for _, tt := range tests {
		t.Run(tt.expectedTitle, func(t *testing.T) {
			s := pres.Slides[tt.idx]
			if s.Title != tt.expectedTitle {
				t.Errorf("slide %d title = %q, want %q", tt.idx, s.Title, tt.expectedTitle)
			}
			if len(s.Content.Bullets) != tt.expectedCount {
				t.Errorf("slide %d bullets = %d, want %d", tt.idx, len(s.Content.Bullets), tt.expectedCount)
			}
			if s.Index != tt.idx {
				t.Errorf("slide %d index = %d, want %d", tt.idx, s.Index, tt.idx)
			}
		})
	}

	// Body text only on first page
	if pres.Slides[0].Content.Body != "Introduction text" {
		t.Error("first page should have body text")
	}
	if pres.Slides[1].Content.Body != "" {
		t.Error("continuation page should not have body text")
	}
}

func TestPaginate_SplitsByGroups(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test", Autopaginate: boolPtr(true)},
		Slides: []types.SlideDefinition{
			{
				Title: "Roadmap",
				Type:  types.SlideTypeContent,
				Content: types.SlideContent{
					BulletGroups: []types.BulletGroup{
						{Header: "**Phase 1**", Bullets: []string{"a", "b", "c", "d", "e"}},
						{Header: "**Phase 2**", Bullets: []string{"f", "g", "h", "i", "j"}},
						{Header: "**Phase 3**", Bullets: []string{"k", "l"}},
					},
					Bullets: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"},
				},
			},
		},
	}

	Paginate(pres)

	// Phase 1 (5 bullets) fits in page 1
	// Phase 2 (5 bullets) would overflow page 1, goes to page 2
	// Phase 3 (2 bullets) fits in page 2
	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	if pres.Slides[0].Title != "Roadmap (1/2)" {
		t.Errorf("slide 0 title = %q, want %q", pres.Slides[0].Title, "Roadmap (1/2)")
	}
	if len(pres.Slides[0].Content.BulletGroups) != 1 {
		t.Errorf("slide 0 groups = %d, want 1", len(pres.Slides[0].Content.BulletGroups))
	}
	if len(pres.Slides[1].Content.BulletGroups) != 2 {
		t.Errorf("slide 1 groups = %d, want 2", len(pres.Slides[1].Content.BulletGroups))
	}
}

func TestPaginate_SkipsDiagramSlides(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test", Autopaginate: boolPtr(true)},
		Slides: []types.SlideDefinition{
			{
				Title: "Chart Slide",
				Content: types.SlideContent{
					Bullets:     make([]string, 20),
					DiagramSpec: &types.DiagramSpec{Type: "bar_chart"},
				},
			},
		},
	}

	Paginate(pres)
	if len(pres.Slides) != 1 {
		t.Errorf("diagram slides should not be paginated, got %d slides", len(pres.Slides))
	}
}

func TestPaginate_SkipsTwoColumnSlides(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test", Autopaginate: boolPtr(true)},
		Slides: []types.SlideDefinition{
			{
				Title: "Two Column",
				Content: types.SlideContent{
					Bullets: make([]string, 20),
					Left:    []string{"left content"},
					Right:   []string{"right content"},
				},
			},
		},
	}

	Paginate(pres)
	if len(pres.Slides) != 1 {
		t.Errorf("two-column slides should not be paginated, got %d slides", len(pres.Slides))
	}
}

func TestPaginate_NoSplitUnderThreshold(t *testing.T) {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test", Autopaginate: boolPtr(true)},
		Slides: []types.SlideDefinition{
			{Title: "Small Slide", Content: types.SlideContent{
				Bullets: []string{"a", "b", "c"},
			}},
		},
	}

	warnings := Paginate(pres)
	if len(pres.Slides) != 1 {
		t.Errorf("expected 1 slide, got %d", len(pres.Slides))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestPaginate_ReindexesMultipleSlides(t *testing.T) {
	bullets := make([]string, 20)
	for i := range bullets {
		bullets[i] = "bullet"
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test", Autopaginate: boolPtr(true)},
		Slides: []types.SlideDefinition{
			{Index: 0, Title: "First Slide", Content: types.SlideContent{
				Bullets: []string{"a"},
			}},
			{Index: 1, Title: "Big Slide", Content: types.SlideContent{
				Bullets: bullets,
			}},
			{Index: 2, Title: "Last Slide", Content: types.SlideContent{
				Bullets: []string{"z"},
			}},
		},
	}

	Paginate(pres)

	// 1 + 3 (20/8=3) + 1 = 5 slides
	if len(pres.Slides) != 5 {
		t.Fatalf("expected 5 slides, got %d", len(pres.Slides))
	}

	// Verify indices are sequential
	for i, s := range pres.Slides {
		if s.Index != i {
			t.Errorf("slide %d has index %d", i, s.Index)
		}
	}

	// Original slides preserved
	if pres.Slides[0].Title != "First Slide" {
		t.Errorf("first slide title = %q, want %q", pres.Slides[0].Title, "First Slide")
	}
	if pres.Slides[4].Title != "Last Slide" {
		t.Errorf("last slide title = %q, want %q", pres.Slides[4].Title, "Last Slide")
	}
}

func TestPaginate_DefaultNilMeansEnabled(t *testing.T) {
	// When Autopaginate is nil (not specified in frontmatter),
	// pagination should be enabled by default
	bullets := make([]string, 12)
	for i := range bullets {
		bullets[i] = "bullet"
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"}, // Autopaginate not set (nil)
		Slides: []types.SlideDefinition{
			{
				Title:   "Dense Content",
				Type:    types.SlideTypeContent,
				Content: types.SlideContent{Bullets: bullets},
			},
		},
	}

	warnings := Paginate(pres)

	// 12 bullets / 8 per page = 2 pages
	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides when autopaginate is nil (default enabled), got %d", len(pres.Slides))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}

	if pres.Slides[0].Title != "Dense Content (1/2)" {
		t.Errorf("slide 0 title = %q, want %q", pres.Slides[0].Title, "Dense Content (1/2)")
	}
	if pres.Slides[1].Title != "Dense Content (2/2)" {
		t.Errorf("slide 1 title = %q, want %q", pres.Slides[1].Title, "Dense Content (2/2)")
	}
}

func TestPaginate_DenseRiskItems(t *testing.T) {
	// Reproduce the exact bug scenario: 5 risk items with 3 sub-bullets each
	// This simulates the bug report where risk #4 was cut off and #5 was missing
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"}, // default autopaginate (nil = enabled)
		Slides: []types.SlideDefinition{
			{
				Title: "Key Risks",
				Type:  types.SlideTypeContent,
				Content: types.SlideContent{
					BulletGroups: []types.BulletGroup{
						{Header: "**Risk 1: Market**", Bullets: []string{"Sub 1a", "Sub 1b", "Sub 1c"}},
						{Header: "**Risk 2: Technical**", Bullets: []string{"Sub 2a", "Sub 2b", "Sub 2c"}},
						{Header: "**Risk 3: Regulatory**", Bullets: []string{"Sub 3a", "Sub 3b", "Sub 3c"}},
						{Header: "**Risk 4: Financial**", Bullets: []string{"Sub 4a", "Sub 4b", "Sub 4c"}},
						{Header: "**Risk 5: Operational**", Bullets: []string{"Sub 5a", "Sub 5b", "Sub 5c"}},
					},
					Bullets: []string{
						"Sub 1a", "Sub 1b", "Sub 1c",
						"Sub 2a", "Sub 2b", "Sub 2c",
						"Sub 3a", "Sub 3b", "Sub 3c",
						"Sub 4a", "Sub 4b", "Sub 4c",
						"Sub 5a", "Sub 5b", "Sub 5c",
					},
				},
			},
		},
	}

	warnings := Paginate(pres)

	// Should be split into continuation slides
	if len(pres.Slides) < 2 {
		t.Fatalf("expected at least 2 slides for 15 bullets in 5 groups, got %d", len(pres.Slides))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}

	// Verify all risks are present across continuation slides
	totalBullets := 0
	for _, s := range pres.Slides {
		totalBullets += len(s.Content.Bullets)
	}
	if totalBullets != 15 {
		t.Errorf("total bullets across all pages = %d, want 15 (no content lost)", totalBullets)
	}

	// Verify titles have continuation numbering
	for _, s := range pres.Slides {
		if s.Title == "Key Risks" {
			t.Error("expected paginated title with (N/M) suffix, got plain title")
		}
	}
}

func TestEffectiveMaxBullets(t *testing.T) {
	tests := []struct {
		name    string
		layouts []types.LayoutMetadata
		want    int
	}{
		{
			name:    "no layouts returns default",
			layouts: nil,
			want:    DefaultMaxBullets,
		},
		{
			name: "visual-only layouts returns default",
			layouts: []types.LayoutMetadata{
				{Name: "Image", Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderImage},
				}, Capacity: types.CapacityEstimate{MaxBullets: 0}},
			},
			want: DefaultMaxBullets,
		},
		{
			name: "layout capacity lower than default",
			layouts: []types.LayoutMetadata{
				{Name: "Content", Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody},
				}, Capacity: types.CapacityEstimate{MaxBullets: 4}},
			},
			want: 4,
		},
		{
			name: "layout capacity higher than default uses default",
			layouts: []types.LayoutMetadata{
				{Name: "Big Content", Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody},
				}, Capacity: types.CapacityEstimate{MaxBullets: 12}},
			},
			want: DefaultMaxBullets,
		},
		{
			name: "uses highest text-capable layout",
			layouts: []types.LayoutMetadata{
				{Name: "Small", Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody},
				}, Capacity: types.CapacityEstimate{MaxBullets: 3}},
				{Name: "Large", Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderContent},
				}, Capacity: types.CapacityEstimate{MaxBullets: 6}},
				{Name: "Image Only", Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderImage},
				}, Capacity: types.CapacityEstimate{MaxBullets: 0}},
			},
			want: 6,
		},
		{
			name: "zero capacity treated as no info",
			layouts: []types.LayoutMetadata{
				{Name: "Content", Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody},
				}, Capacity: types.CapacityEstimate{MaxBullets: 0}},
			},
			want: DefaultMaxBullets,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveMaxBullets(tt.layouts)
			if got != tt.want {
				t.Errorf("effectiveMaxBullets() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPaginateWithLayouts_SplitsAtLayoutCapacity(t *testing.T) {
	// 6 bullets should be split when layout only supports 4
	bullets := []string{"a", "b", "c", "d", "e", "f"}

	layouts := []types.LayoutMetadata{
		{Name: "Content", Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody},
		}, Capacity: types.CapacityEstimate{MaxBullets: 4}},
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"},
		Slides: []types.SlideDefinition{
			{Title: "Key Points", Content: types.SlideContent{Bullets: bullets}},
		},
	}

	warnings := PaginateWithLayouts(pres, layouts)

	// 6 bullets / 4 per page = 2 pages
	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
	if len(pres.Slides[0].Content.Bullets) != 4 {
		t.Errorf("page 1 bullets = %d, want 4", len(pres.Slides[0].Content.Bullets))
	}
	if len(pres.Slides[1].Content.Bullets) != 2 {
		t.Errorf("page 2 bullets = %d, want 2", len(pres.Slides[1].Content.Bullets))
	}
}

func TestPaginateWithLayouts_NoSplitUnderCapacity(t *testing.T) {
	// 4 bullets should NOT be split when layout supports 4
	bullets := []string{"a", "b", "c", "d"}

	layouts := []types.LayoutMetadata{
		{Name: "Content", Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody},
		}, Capacity: types.CapacityEstimate{MaxBullets: 4}},
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"},
		Slides: []types.SlideDefinition{
			{Title: "Key Points", Content: types.SlideContent{Bullets: bullets}},
		},
	}

	warnings := PaginateWithLayouts(pres, layouts)

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide (under capacity), got %d", len(pres.Slides))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestPaginateWithLayouts_13BulletsIn3BulletLayout(t *testing.T) {
	// Worst case from bug report: 13 bullets in 3-bullet layout
	bullets := make([]string, 13)
	for i := range bullets {
		bullets[i] = "bullet"
	}

	layouts := []types.LayoutMetadata{
		{Name: "Compact", Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody},
		}, Capacity: types.CapacityEstimate{MaxBullets: 3}},
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"},
		Slides: []types.SlideDefinition{
			{Title: "Dense", Content: types.SlideContent{Bullets: bullets}},
		},
	}

	warnings := PaginateWithLayouts(pres, layouts)

	// 13 / 3 = 5 pages (3, 3, 3, 3, 1)
	if len(pres.Slides) != 5 {
		t.Fatalf("expected 5 slides for 13 bullets / 3 capacity, got %d", len(pres.Slides))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}

	// Verify no content lost
	total := 0
	for _, s := range pres.Slides {
		total += len(s.Content.Bullets)
	}
	if total != 13 {
		t.Errorf("total bullets = %d, want 13", total)
	}
}

func TestPaginateWithLayouts_FallsBackToDefault(t *testing.T) {
	// When no layout capacity info, should use DefaultMaxBullets
	bullets := make([]string, 10)
	for i := range bullets {
		bullets[i] = "bullet"
	}

	// Layouts exist but have no body placeholders (image-only template)
	layouts := []types.LayoutMetadata{
		{Name: "Image", Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderImage},
		}},
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"},
		Slides: []types.SlideDefinition{
			{Title: "Points", Content: types.SlideContent{Bullets: bullets}},
		},
	}

	warnings := PaginateWithLayouts(pres, layouts)

	// 10 bullets / 8 (default) = 2 pages
	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides with default threshold, got %d", len(pres.Slides))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestPaginateWithLayouts_GroupsRespectCapacity(t *testing.T) {
	// BulletGroups with low layout capacity
	layouts := []types.LayoutMetadata{
		{Name: "Content", Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderBody},
		}, Capacity: types.CapacityEstimate{MaxBullets: 4}},
	}

	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{Title: "Test"},
		Slides: []types.SlideDefinition{
			{
				Title: "Roadmap",
				Content: types.SlideContent{
					BulletGroups: []types.BulletGroup{
						{Header: "**Q1**", Bullets: []string{"a", "b", "c"}},
						{Header: "**Q2**", Bullets: []string{"d", "e", "f"}},
						{Header: "**Q3**", Bullets: []string{"g", "h"}},
					},
					Bullets: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
				},
			},
		},
	}

	PaginateWithLayouts(pres, layouts)

	// Q1 (3 bullets) fits in page 1
	// Q2 (3 bullets) would overflow page 1 (3+3=6 > 4), goes to page 2
	// Q3 (2 bullets) fits in page 2 (3+2=5 > 4), goes to page 3
	if len(pres.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(pres.Slides))
	}

	if len(pres.Slides[0].Content.BulletGroups) != 1 {
		t.Errorf("page 1 groups = %d, want 1", len(pres.Slides[0].Content.BulletGroups))
	}
	if len(pres.Slides[1].Content.BulletGroups) != 1 {
		t.Errorf("page 2 groups = %d, want 1", len(pres.Slides[1].Content.BulletGroups))
	}
	if len(pres.Slides[2].Content.BulletGroups) != 1 {
		t.Errorf("page 3 groups = %d, want 1", len(pres.Slides[2].Content.BulletGroups))
	}
}

func TestPaginatedTitle(t *testing.T) {
	tests := []struct {
		title string
		page  int
		total int
		want  string
	}{
		{"Market Analysis", 1, 3, "Market Analysis (1/3)"},
		{"Roadmap", 2, 2, "Roadmap (2/2)"},
		{"Simple", 1, 1, "Simple"},
	}

	for _, tt := range tests {
		got := paginatedTitle(tt.title, tt.page, tt.total)
		if got != tt.want {
			t.Errorf("paginatedTitle(%q, %d, %d) = %q, want %q",
				tt.title, tt.page, tt.total, got, tt.want)
		}
	}
}
