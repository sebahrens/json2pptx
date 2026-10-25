package layout

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestSelectLayoutRanksActualMappedText(t *testing.T) {
	long := strings.Repeat("Complete the rollout with local teams. ", 12)
	for _, form := range []string{"body", "bullets", "slots"} {
		t.Run(form, func(t *testing.T) {
			small := types.LayoutMetadata{ID: "small", Tags: []string{"content"}, Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, MaxChars: 100},
				{ID: "body", Type: types.PlaceholderBody, MaxChars: 100},
			}}
			large := small
			large.ID = "large"
			large.Placeholders = append([]types.PlaceholderInfo(nil), small.Placeholders...)
			large.Placeholders[1].MaxChars = 1000
			slide := types.SlideDefinition{Type: types.SlideTypeContent, Title: "Topic"}
			switch form {
			case "body":
				slide.Content.Body = long
			case "bullets":
				slide.Content.Bullets = []string{long}
			case "slots":
				slide.Slots = map[int]*types.SlotContent{1: {SlotNumber: 1, Text: long, Type: types.SlotContentText}}
			}
			for _, layouts := range [][]types.LayoutMetadata{{small, large}, {large, small}} {
				got, err := SelectLayout(SelectionRequest{Slide: slide, Layouts: layouts})
				if err != nil || got.LayoutID != "large" {
					t.Fatalf("%s: got %+v, err %v", form, got, err)
				}
			}
			// Geometry and font metrics replace character hints when available.
			small.Placeholders[1].Bounds = types.BoundingBox{Width: 300 * 12700, Height: 50 * 12700}
			large.Placeholders[1].Bounds = types.BoundingBox{Width: 300 * 12700, Height: 400 * 12700}
			got, err := SelectLayout(SelectionRequest{Slide: slide, Layouts: []types.LayoutMetadata{small, large}})
			if err != nil || got.LayoutID != "large" {
				t.Fatalf("measured: got %+v, err %v", got, err)
			}
		})
	}
}

func TestSelectLayoutKeepsCompactShortStatement(t *testing.T) {
	l := types.LayoutMetadata{ID: "compact", Tags: []string{"content"}, Placeholders: []types.PlaceholderInfo{{ID: "title", Type: types.PlaceholderTitle}, {ID: "body", Type: types.PlaceholderBody, MaxChars: 100}}}
	r, err := SelectLayout(SelectionRequest{Slide: types.SlideDefinition{Type: types.SlideTypeContent, Title: "Topic", Content: types.SlideContent{Body: "Short statement"}}, Layouts: []types.LayoutMetadata{l}})
	if err != nil || r.LayoutID != "compact" {
		t.Fatalf("got %+v: %v", r, err)
	}
}
