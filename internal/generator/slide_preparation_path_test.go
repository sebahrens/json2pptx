package generator

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTextAutofitFindingsUseAuthoredContentIndex(t *testing.T) {
	slide := &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
		NonVisualProperties: nonVisualPropertiesXML{
			ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
			NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
		},
		ShapeProperties: shapePropertiesXML{Transform: &transformXML{
			Extent: extentXML{CX: 1000000, CY: 300000},
		}},
		TextBody: &textBodyXML{},
	}}}}}
	ctx := newSinglePassContext("", nil, nil, false, nil)
	content := []ContentItem{
		{PlaceholderID: "missing", Type: ContentText, Value: "First"},
		{PlaceholderID: "body", Type: ContentBullets, Value: []string{strings.Repeat("Long bullet text ", 20), strings.Repeat("More text ", 20)}},
	}
	ctx.populateTextInSlide(slide, content, "slideLayout1", 0, "")
	if len(ctx.fitFindings) == 0 {
		t.Fatal("expected autofit finding for cramped body text")
	}
	var authored any
	if err := json.Unmarshal([]byte(`{"slides":[{"content":[{"placeholder_id":"missing"},{"placeholder_id":"body"}]}]}`), &authored); err != nil {
		t.Fatal(err)
	}
	var sawBodyFinding bool
	for _, finding := range ctx.fitFindings {
		if finding.Path == "/slides/0/content/1" {
			sawBodyFinding = true
		}
		if _, err := resolveTestJSONPointer(authored, finding.Path); err != nil {
			t.Errorf("autofit finding path %q does not resolve: %v", finding.Path, err)
		}
	}
	if !sawBodyFinding {
		t.Errorf("cramped body findings do not target authored content index 1: %+v", ctx.fitFindings)
	}
}
