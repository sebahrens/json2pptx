package layout

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-ujya: on modern-template a two-column slide (a chart beside
// eight insights) scored "Section Divider" 0.83 and "Title Slide" 0.82 over
// "Two Content" 0.15 — the insights landed in the divider's "Section Number"
// placeholder, rendered at ~54pt in four overlapping lines, and ran off the
// slide. Three things were wrong: a layout with too few content placeholders
// still earned capacity credit, bullet overflow drove the only viable layout to
// zero, and the narrow-diagram penalty punished the side-by-side the author
// asked for.
func TestTwoColumnSlidePrefersTwoContentOverChrome(t *testing.T) {
	half := int64(4_000_000)
	twoContent := types.LayoutMetadata{
		ID: "two-content", Name: "Two Content",
		Tags:          []string{"content", "two-column", "comparison"},
		CanonicalType: types.CanonicalLayoutTwoContent,
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Role: types.PlaceholderRoleTitle},
			{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 1, Bounds: types.BoundingBox{Width: half}},
			{ID: "body_2", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 2, Bounds: types.BoundingBox{Width: half}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 3},
	}
	divider := types.LayoutMetadata{
		ID: "divider", Name: "Section Divider",
		Tags:          []string{"content", "section-header"},
		CanonicalType: types.CanonicalLayoutSectionDivider,
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Role: types.PlaceholderRoleTitle},
			{ID: "number", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 1, Bounds: types.BoundingBox{Width: half}},
			{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 2, Bounds: types.BoundingBox{Width: half}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 5},
	}
	titleSlide := types.LayoutMetadata{
		ID: "title-slide", Name: "Title Slide",
		Tags:          []string{"title-slide"},
		CanonicalType: types.CanonicalLayoutTitleSlide,
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Role: types.PlaceholderRoleTitle},
		},
	}
	oneContent := types.LayoutMetadata{
		ID: "one-content", Name: "One Content",
		Tags:          []string{"content"},
		CanonicalType: types.CanonicalLayoutOneContent,
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Role: types.PlaceholderRoleTitle},
			{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 1, Bounds: types.BoundingBox{Width: 2 * half}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 7},
	}

	slide := types.SlideDefinition{
		Title: "Revenue grew across all four regions",
		Type:  types.SlideTypeTwoColumn,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{Type: "bar_chart"},
			Left:        []string{"chart"},
			Right:       []string{"i1", "i2", "i3", "i4", "i5", "i6", "i7", "i8"},
		},
	}
	layouts := []types.LayoutMetadata{twoContent, divider, titleSlide, oneContent}
	req := SelectionRequest{Slide: slide, Layouts: layouts}

	scores := map[string]float64{}
	for i := range layouts {
		scores[layouts[i].ID] = scoreLayout(&layouts[i], req).score
	}
	t.Logf("scores: %v", scores)
	for _, worse := range []string{"divider", "title-slide", "one-content"} {
		if scores["two-content"] <= scores[worse] {
			t.Errorf("two-content scored %.2f, not above %s at %.2f — the slide's second column would be dropped",
				scores["two-content"], worse, scores[worse])
		}
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.LayoutID != "two-content" {
		t.Errorf("selected %q, want two-content", result.LayoutID)
	}
}

// A layout with fewer content placeholders than the slide needs cannot host it,
// whatever its bullet capacity says.
func TestCapacityRejectsInsufficientPlaceholders(t *testing.T) {
	oneBody := types.LayoutMetadata{
		ID: "one", Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle},
			{ID: "body", Type: types.PlaceholderBody, Index: 1},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 20},
	}
	twoColumn := types.SlideDefinition{
		Type:    types.SlideTypeTwoColumn,
		Content: types.SlideContent{Left: []string{"a"}, Right: []string{"b"}},
	}
	if got := scoreCapacity(oneBody, twoColumn); got > 0.35 {
		t.Errorf("capacity = %.2f for a two-column slide on a one-body layout, want a heavy penalty", got)
	}

	single := types.SlideDefinition{Type: types.SlideTypeContent, Content: types.SlideContent{Bullets: []string{"a", "b"}}}
	if got := scoreCapacity(oneBody, single); got != 1.0 {
		t.Errorf("capacity = %.2f for a one-column slide that fits, want 1.0", got)
	}
}

// A complex diagram keeps its narrow-column penalty even on an explicit
// two-column slide: no author intent makes a business model canvas readable at
// half width.
func TestNarrowDiagramPenaltyStillAppliesToFullWidthTypes(t *testing.T) {
	half := int64(4_000_000)
	twoColumn := types.LayoutMetadata{
		ID: "two", Tags: []string{"content", "two-column"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle},
			{ID: "body", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: half}},
			{ID: "body_2", Type: types.PlaceholderBody, Index: 2, Bounds: types.BoundingBox{Width: half}},
		},
	}
	slide := types.SlideDefinition{
		Type: types.SlideTypeTwoColumn,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{Type: "business_model_canvas"},
			Left:        []string{"diagram"}, Right: []string{"text"},
		},
	}
	if got := penalizeNarrowDiagramSlot(twoColumn, slide); got == 0 {
		t.Error("a business model canvas in a half-width column must still be penalised")
	}
	slide.Content.DiagramSpec = &types.DiagramSpec{Type: "bar_chart"}
	if got := penalizeNarrowDiagramSlot(twoColumn, slide); got != 0 {
		t.Errorf("an ordinary chart beside text is the point of a two-column slide, got penalty %.2f", got)
	}
}
