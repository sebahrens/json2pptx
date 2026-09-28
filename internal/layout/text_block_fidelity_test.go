package layout

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-k3lyz: a content slide with a title and bullets was routed
// to a title-less Statement layout (modern-yellow) and a title slide with a
// subtitle to a title-only "Blank + Title" layout (modern), so the renderer
// dropped a block. Layouts that host every text block must win when one exists.
func TestSelectLayout_PrefersLayoutHostingEveryTextBlock(t *testing.T) {
	statement := types.LayoutMetadata{
		ID: "statement", Name: "Statement", Tags: []string{"statement", "content"},
		Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderBody, Bounds: types.BoundingBox{Width: 8229600, Height: 4525963}}},
		Capacity:     types.CapacityEstimate{MaxBullets: 8, MaxTextLines: 10, TextHeavy: true},
	}
	blankTitle := types.LayoutMetadata{
		ID: "blank-title", Name: "Blank + Title", Tags: []string{"title-slide", "blank-title"},
		Placeholders: []types.PlaceholderInfo{{ID: "title", Type: types.PlaceholderTitle}},
	}
	titleSub := types.LayoutMetadata{
		ID: "title-sub", Name: "Title Slide", Tags: []string{"title-slide"},
		Placeholders: []types.PlaceholderInfo{{ID: "title", Type: types.PlaceholderTitle}, {ID: "subtitle", Type: types.PlaceholderSubtitle}},
	}

	content := types.SlideDefinition{Type: types.SlideTypeContent, Title: "Risks", Content: types.SlideContent{Bullets: []string{"a", "b"}}}
	if hostsEveryTextBlock(statement, content) {
		t.Error("a title-less layout cannot host a title plus bullets")
	}
	if !hostsEveryTextBlock(contentLayout(8), content) {
		t.Error("title + body layout must host a title plus bullets")
	}
	// Heavy prior use of the content layout pushes its variety score down; the
	// Statement layout must still not win.
	used := map[string]int{"layout-content": 20}
	got, err := SelectLayout(SelectionRequest{Slide: content, Layouts: []types.LayoutMetadata{statement, contentLayout(8)},
		Context: SelectionContext{Position: 5, TotalSlides: 10, UsedLayouts: used}})
	if err != nil || got.LayoutID != "layout-content" {
		t.Fatalf("content slide got %+v (err %v), want layout-content", got, err)
	}
	ranked, err := SelectLayoutRanked(SelectionRequest{Slide: content, Layouts: []types.LayoutMetadata{statement, contentLayout(8)},
		Context: SelectionContext{Position: 5, TotalSlides: 10, UsedLayouts: used}}, 2)
	if err != nil || ranked.Primary.LayoutID != "layout-content" || len(ranked.Alternates) != 0 {
		t.Fatalf("ranked selection = %+v (err %v), want layout-content with no title-less alternate", ranked, err)
	}

	title := types.SlideDefinition{Type: types.SlideTypeTitle, Title: "Thank you", Content: types.SlideContent{Body: "Questions"}}
	got, err = SelectLayout(SelectionRequest{Slide: title, Layouts: []types.LayoutMetadata{blankTitle, titleSub},
		Context: SelectionContext{Position: 9, TotalSlides: 10, UsedLayouts: map[string]int{"title-sub": 20}}})
	if err != nil || got.LayoutID != "title-sub" {
		t.Fatalf("title slide with subtitle got %+v (err %v), want title-sub", got, err)
	}

	// With no hosting layout at all, selection falls back instead of failing.
	got, err = SelectLayout(SelectionRequest{Slide: title, Layouts: []types.LayoutMetadata{blankTitle}})
	if err != nil || got.LayoutID != "blank-title" {
		t.Fatalf("fallback got %+v (err %v), want blank-title", got, err)
	}
}
