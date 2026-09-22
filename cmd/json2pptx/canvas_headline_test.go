package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCanvasHeadlineRendersAndReservesGridSpace(t *testing.T) {
	const width int64 = 12192000
	const height int64 = 6858000
	layouts := []types.LayoutMetadata{{ID: "blank-layout", CanonicalType: types.CanonicalLayoutBlank}}
	slide := SlideInput{LayoutID: "blank-layout", Headline: "Agents move from copilots to operators", ShapeGrid: &ShapeGridInput{}}

	geom := resolveGridGeometry(slide, layouts, width, height)
	headline := canvasHeadlineBounds(width, height)
	if geom.Zone == nil {
		t.Fatal("headline did not create a protected content zone")
	}
	if geom.Zone.TitleBottom < headline.Y+headline.CY {
		t.Fatalf("content zone starts at %d, above headline bottom %d", geom.Zone.TitleBottom, headline.Y+headline.CY)
	}

	xml, err := generateCanvasHeadline(slide.Headline, headline, &GridDiagramContext{TitleFont: "Aptos Display"})
	if err != nil {
		t.Fatalf("generate headline: %v", err)
	}
	got := string(xml)
	for _, want := range []string{"Canvas Headline", slide.Headline, `typeface="Aptos Display"`, `schemeClr val="tx1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("headline XML missing %q: %s", want, got)
		}
	}
}
