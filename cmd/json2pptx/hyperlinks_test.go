package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestHyperlinkInputConversion(t *testing.T) {
	var slide SlideInput
	if err := json.Unmarshal([]byte(`{
		"content":[{"placeholder_id":"body","type":"bullets","bullets_value":["Evidence"],"link":{"url":"https://example.com"}}],
		"source":"Annual report","source_link":{"url":"https://example.com/report"},
		"shape_grid":{"columns":1,"rows":[{"cells":[{"shape":{"geometry":"rect","text":"Appendix","link":{"slide":2}}}]}]},
		"overlays":[{"kind":"badge","from":{"x":1,"y":1},"text":"Next","link":{"slide":2}}]
	}`), &slide); err != nil {
		t.Fatal(err)
	}
	if got := toGeneratorLink(slide.SourceLink); got == nil || got.URL != "https://example.com/report" {
		t.Fatalf("source link lost: %+v", got)
	}
	items, err := convertPresentationContent(slide.Content, 1, "content")
	if err != nil || len(items) != 1 || items[0].Link == nil || items[0].Link.URL != "https://example.com" {
		t.Fatalf("content link lost: %+v, %v", items, err)
	}
	cell := convertGridCell(slide.ShapeGrid.Rows[0].Cells[0])
	if cell.Shape == nil || cell.Shape.Link == nil || cell.Shape.Link.Slide != 2 {
		t.Fatalf("grid link lost: %+v", cell.Shape)
	}
	gridXML, err := shapegrid.GenerateShapeXML(cell.Shape, 201, pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000})
	if err != nil || !strings.Contains(string(gridXML), `action="ppaction://hlinksldjump"`) {
		t.Fatalf("grid link not emitted: %s, %v", gridXML, err)
	}
	overlays, err := resolveOverlays(slide.Overlays, nil, &pptx.ShapeIDAllocator{}, 10000000, 6000000, nil)
	if err != nil || len(overlays) != 1 || !strings.Contains(string(overlays[0]), `r:id="json2pptx_overlay_link_0"`) {
		t.Fatalf("badge link not emitted: %v, %v", overlays, err)
	}
}
