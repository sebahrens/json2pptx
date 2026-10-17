package main

import (
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
	"strings"
	"testing"
)

func TestGraphicsAuditHeadingFitUsesRenderFont(t *testing.T) {
	heading := strings.Repeat("iii ", 13)
	slide := SlideInput{LayoutID: "one", Content: []ContentInput{{PlaceholderID: "body", Heading: heading}}}
	for _, font := range []string{"Arial", "Lora"} {
		for width := int64(40); width <= 180; width += 5 {
			base := types.LayoutMetadata{ID: "one", Placeholders: []types.PlaceholderInfo{{ID: "body", Bounds: types.BoundingBox{Width: width * 12700, Height: 5000000}}}}
			got, err := deriveLayoutMetadata(slide, base, []types.LayoutMetadata{base}, 12192000, 6858000, font)
			if err != nil {
				t.Fatal(err)
			}
			reserve := generator.NativeColumnHeadingReserve(heading, width*12700, font)
			if got.Placeholders[0].Bounds.Y != reserve {
				t.Fatalf("%s fit and render heading bounds differ", font)
			}
		}
	}
}
