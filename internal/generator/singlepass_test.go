package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// TestInsertSlideEntriesIntoPresentationXML tests the deterministic function
// that inserts slide entries into presentation.xml
func TestInsertSlideEntriesIntoPresentationXML(t *testing.T) {
	tests := []struct {
		name            string
		presentationXML string
		entries         []string
		wantContains    []string
		wantNotChange   bool // if true, expect no change
	}{
		{
			name: "insert single slide entry",
			presentationXML: `<?xml version="1.0"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`,
			entries: []string{`<p:sldId id="257" r:id="rId3"/>`},
			wantContains: []string{
				`<p:sldId id="257" r:id="rId3"/>`,
				`</p:sldIdLst>`,
			},
		},
		{
			name: "insert multiple slide entries",
			presentationXML: `<?xml version="1.0"?>
<p:presentation>
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`,
			entries: []string{
				`<p:sldId id="257" r:id="rId3"/>`,
				`<p:sldId id="258" r:id="rId4"/>`,
				`<p:sldId id="259" r:id="rId5"/>`,
			},
			wantContains: []string{
				`<p:sldId id="257" r:id="rId3"/>`,
				`<p:sldId id="258" r:id="rId4"/>`,
				`<p:sldId id="259" r:id="rId5"/>`,
			},
		},
		{
			name: "no entries to insert",
			presentationXML: `<?xml version="1.0"?>
<p:presentation>
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`,
			entries:       []string{},
			wantNotChange: true,
		},
		{
			name: "fallback to non-namespaced closing tag",
			presentationXML: `<?xml version="1.0"?>
<presentation>
  <sldIdLst>
    <sldId id="256" r:id="rId2"/>
  </sldIdLst>
</presentation>`,
			entries: []string{`<p:sldId id="257" r:id="rId3"/>`},
			wantContains: []string{
				`<p:sldId id="257" r:id="rId3"/>`,
			},
		},
		{
			name: "self-closing p:sldIdLst expands correctly",
			presentationXML: `<?xml version="1.0"?>
<p:presentation>
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
  <p:sldIdLst/>
  <p:sldSz cx="12192000" cy="6858000"/>
</p:presentation>`,
			entries: []string{`<p:sldId id="257" r:id="rId3"/>`},
			wantContains: []string{
				`<p:sldIdLst><p:sldId id="257" r:id="rId3"/></p:sldIdLst>`,
			},
		},
		{
			name: "self-closing sldIdLst without namespace expands correctly",
			presentationXML: `<?xml version="1.0"?>
<presentation>
  <sldIdLst/>
  <sldSz cx="12192000" cy="6858000"/>
</presentation>`,
			entries: []string{`<p:sldId id="257" r:id="rId3"/>`},
			wantContains: []string{
				`<sldIdLst><p:sldId id="257" r:id="rId3"/></sldIdLst>`,
			},
		},
		{
			name: "missing sldIdLst inserts before sldSz",
			presentationXML: `<?xml version="1.0"?>
<p:presentation>
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
  <p:notesMasterIdLst><p:notesMasterId r:id="rId2"/></p:notesMasterIdLst>
  <p:sldSz cx="12192000" cy="6858000"/>
</p:presentation>`,
			entries: []string{`<p:sldId id="257" r:id="rId3"/>`},
			wantContains: []string{
				`<p:sldIdLst><p:sldId id="257" r:id="rId3"/></p:sldIdLst>`,
				`</p:sldIdLst><p:sldSz`,
			},
		},
		{
			name: "missing sldIdLst fallback inserts before closing presentation",
			presentationXML: `<?xml version="1.0"?>
<p:presentation>
  <p:other>content</p:other>
</p:presentation>`,
			entries: []string{`<p:sldId id="257" r:id="rId3"/>`},
			wantContains: []string{
				`<p:sldIdLst><p:sldId id="257" r:id="rId3"/></p:sldIdLst>`,
				`</p:sldIdLst></p:presentation>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := insertSlideEntriesIntoPresentationXML(tt.presentationXML, tt.entries)

			if tt.wantNotChange {
				if result != tt.presentationXML {
					t.Errorf("expected no change, but got different result")
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing expected content: %q\ngot: %s", want, result)
				}
			}

			// Ensure the closing tag still exists
			if !strings.Contains(result, "</p:sldIdLst>") && !strings.Contains(result, "</sldIdLst>") {
				t.Errorf("closing tag was removed from result")
			}
		})
	}
}

// TestSinglePassContext_ScanTemplate tests the scanTemplate method
func TestSinglePassContext_ScanTemplate(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	// Open template
	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Create context with some slide specs
	ctx := newSinglePassContext("", []SlideSpec{
		{LayoutID: "slideLayout1", Content: []ContentItem{}},
		{LayoutID: "slideLayout2", Content: []ContentItem{}},
	}, nil, false, nil)
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)

	// Run scan
	err = ctx.scanTemplate()
	if err != nil {
		t.Fatalf("scanTemplate failed: %v", err)
	}

	// Verify results
	if ctx.existingSlides < 1 {
		t.Errorf("expected at least 1 existing slide, got %d", ctx.existingSlides)
	}

	// Verify slide content map was built correctly
	expectedSlides := ctx.existingSlides + len(ctx.slideSpecs)
	if len(ctx.slideContentMap) != len(ctx.slideSpecs) {
		t.Errorf("slideContentMap has %d entries, expected %d", len(ctx.slideContentMap), len(ctx.slideSpecs))
	}

	// Verify slide numbers are correct (existingSlides+1, existingSlides+2, ...)
	for i := range ctx.slideSpecs {
		slideNum := ctx.existingSlides + i + 1
		if _, ok := ctx.slideContentMap[slideNum]; !ok {
			t.Errorf("slideContentMap missing entry for slide %d", slideNum)
		}
	}

	t.Logf("Found %d existing slides, new slides will be %d-%d",
		ctx.existingSlides, ctx.existingSlides+1, expectedSlides)
}

// TestScanTemplate_ExcludeTemplateSlides tests that slide numbering starts at 1
// when ExcludeTemplateSlides is enabled
func TestScanTemplate_ExcludeTemplateSlides(t *testing.T) {
	templatePath := "../../templates/template-simple.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("template not found, skipping test")
	}

	// Open template
	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Create context with ExcludeTemplateSlides enabled
	ctx := newSinglePassContext("", []SlideSpec{
		{LayoutID: "slideLayout1", Content: []ContentItem{}},
		{LayoutID: "slideLayout2", Content: []ContentItem{}},
	}, nil, true, nil) // excludeTemplateSlides = true
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)

	// Run scan
	err = ctx.scanTemplate()
	if err != nil {
		t.Fatalf("scanTemplate failed: %v", err)
	}

	// Verify existingSlides is counted
	if ctx.existingSlides < 1 {
		t.Errorf("expected at least 1 existing slide, got %d", ctx.existingSlides)
	}

	// Verify slide content map starts at 1 (not existingSlides+1)
	if len(ctx.slideContentMap) != len(ctx.slideSpecs) {
		t.Errorf("slideContentMap has %d entries, expected %d", len(ctx.slideContentMap), len(ctx.slideSpecs))
	}

	// Verify new slides are numbered 1, 2 (not existingSlides+1, existingSlides+2)
	for i := range ctx.slideSpecs {
		slideNum := i + 1 // Should be 1, 2
		if _, ok := ctx.slideContentMap[slideNum]; !ok {
			t.Errorf("slideContentMap missing entry for slide %d (expected 1-based numbering)", slideNum)
		}
	}

	// Verify existingSlides-based numbering is NOT used
	oldStyleNum := ctx.existingSlides + 1
	if ctx.existingSlides > 0 {
		if _, ok := ctx.slideContentMap[oldStyleNum]; ok && oldStyleNum > len(ctx.slideSpecs) {
			t.Errorf("slideContentMap has entry for slide %d - should use 1-based numbering when ExcludeTemplateSlides=true", oldStyleNum)
		}
	}

	t.Logf("ExcludeTemplateSlides=true: %d existing slides counted, new slides numbered 1-%d",
		ctx.existingSlides, len(ctx.slideSpecs))
}

// TestReplaceSlideListInPresentationXML tests the new function for excluding template slides
func TestReplaceSlideListInPresentationXML(t *testing.T) {
	// Sample presentation.xml with existing slides
	input := `<?xml version="1.0"?><p:presentation><p:sldIdLst><p:sldId id="256" r:id="rId2"/><p:sldId id="257" r:id="rId3"/></p:sldIdLst></p:presentation>`

	newSlideEntries := []string{
		`<p:sldId id="258" r:id="rId10"/>`,
		`<p:sldId id="259" r:id="rId11"/>`,
	}

	result := replaceSlideListInPresentationXML(input, newSlideEntries)

	// Verify old slides are removed
	if strings.Contains(result, `id="256"`) {
		t.Error("result should not contain old slide id 256")
	}
	if strings.Contains(result, `id="257"`) {
		t.Error("result should not contain old slide id 257")
	}

	// Verify new slides are present
	if !strings.Contains(result, `id="258"`) {
		t.Error("result should contain new slide id 258")
	}
	if !strings.Contains(result, `id="259"`) {
		t.Error("result should contain new slide id 259")
	}

	// Verify structure is maintained
	if !strings.Contains(result, "<p:sldIdLst>") || !strings.Contains(result, "</p:sldIdLst>") {
		t.Error("result should contain sldIdLst tags")
	}

	t.Logf("Result: %s", result)
}

// TestReplaceSlideListInPresentationXML_SelfClosing tests replace with self-closing sldIdLst
func TestReplaceSlideListInPresentationXML_SelfClosing(t *testing.T) {
	input := `<?xml version="1.0"?><p:presentation><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:sldIdLst/><p:sldSz cx="12192000" cy="6858000"/></p:presentation>`

	newSlideEntries := []string{
		`<p:sldId id="258" r:id="rId10"/>`,
		`<p:sldId id="259" r:id="rId11"/>`,
	}

	result := replaceSlideListInPresentationXML(input, newSlideEntries)

	if !strings.Contains(result, `id="258"`) {
		t.Error("result should contain new slide id 258")
	}
	if !strings.Contains(result, `id="259"`) {
		t.Error("result should contain new slide id 259")
	}
	if !strings.Contains(result, "<p:sldIdLst>") || !strings.Contains(result, "</p:sldIdLst>") {
		t.Error("result should contain expanded sldIdLst tags")
	}
	if strings.Contains(result, "<p:sldIdLst/>") {
		t.Error("result should not contain self-closing sldIdLst")
	}

	t.Logf("Result: %s", result)
}

// TestReplaceSlideListInPresentationXML_Missing tests replace when sldIdLst is entirely absent
func TestReplaceSlideListInPresentationXML_Missing(t *testing.T) {
	input := `<?xml version="1.0"?><p:presentation><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:notesMasterIdLst><p:notesMasterId r:id="rId2"/></p:notesMasterIdLst><p:sldSz cx="12855575" cy="7231063"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`

	newSlideEntries := []string{
		`<p:sldId id="258" r:id="rId10"/>`,
		`<p:sldId id="259" r:id="rId11"/>`,
	}

	result := replaceSlideListInPresentationXML(input, newSlideEntries)

	if !strings.Contains(result, `id="258"`) {
		t.Errorf("result should contain new slide id 258\ngot: %s", result)
	}
	if !strings.Contains(result, `id="259"`) {
		t.Errorf("result should contain new slide id 259\ngot: %s", result)
	}
	if !strings.Contains(result, "<p:sldIdLst>") || !strings.Contains(result, "</p:sldIdLst>") {
		t.Errorf("result should contain sldIdLst tags\ngot: %s", result)
	}
	// sldIdLst should appear before sldSz
	sldIDLstIdx := strings.Index(result, "</p:sldIdLst>")
	sldSzIdx := strings.Index(result, "<p:sldSz")
	if sldIDLstIdx == -1 || sldSzIdx == -1 || sldIDLstIdx > sldSzIdx {
		t.Errorf("sldIdLst should appear before sldSz in the XML\ngot: %s", result)
	}

	t.Logf("Result: %s", result)
}

// TestSinglePassContext_PopulateTextInSlide tests text and bullet population
func TestSinglePassContext_PopulateTextInSlide(t *testing.T) {
	// Create a mock slide with normalized canonical placeholder names.
	// After normalization, placeholders have canonical names like "title", "body", "body_2".
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "title"},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "title"},
							},
						},
						TextBody: &textBodyXML{},
					},
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "body"},
							},
						},
						TextBody: &textBodyXML{},
					},
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "body_2"},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "body"},
							},
						},
						TextBody: &textBodyXML{},
					},
				},
			},
		},
	}

	ctx := &singlePassContext{}

	tests := []struct {
		name            string
		content         []ContentItem
		wantWarnings    int
		wantTitleText   string
		wantBodyBullets int
	}{
		{
			name: "populate title text",
			content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Test Title"},
			},
			wantWarnings:  0,
			wantTitleText: "Test Title",
		},
		{
			name: "populate bullets",
			content: []ContentItem{
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Bullet 1", "Bullet 2", "Bullet 3"}},
			},
			wantWarnings:    0,
			wantBodyBullets: 3,
		},
		{
			name: "populate by canonical name",
			content: []ContentItem{
				{PlaceholderID: "body_2", Type: ContentText, Value: "Second body content"},
			},
			wantWarnings: 0,
		},
		{
			name: "missing placeholder warning",
			content: []ContentItem{
				{PlaceholderID: "nonexistent", Type: ContentText, Value: "Should warn"},
			},
			wantWarnings: 1,
		},
		{
			name: "empty text clears content",
			content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: ""},
			},
			wantWarnings:  0,
			wantTitleText: "",
		},
		{
			name: "empty bullets clears content",
			content: []ContentItem{
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{}},
			},
			wantWarnings:    0,
			wantBodyBullets: 0,
		},
		{
			name: "invalid text value type",
			content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: 12345},
			},
			wantWarnings: 1,
		},
		{
			name: "invalid bullets value type",
			content: []ContentItem{
				{PlaceholderID: "body", Type: ContentBullets, Value: "not a slice"},
			},
			wantWarnings: 1,
		},
		{
			name: "skip image content",
			content: []ContentItem{
				{PlaceholderID: "pic", Type: ContentImage, Value: ImageContent{Path: "test.png"}},
			},
			wantWarnings: 0, // Should skip without error
		},
		{
			name: "skip chart content",
			content: []ContentItem{
				{PlaceholderID: "chart", Type: ContentDiagram, Value: []byte{}},
			},
			wantWarnings: 0, // Should skip without error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset slide state
			for i := range slide.CommonSlideData.ShapeTree.Shapes {
				slide.CommonSlideData.ShapeTree.Shapes[i].TextBody = &textBodyXML{}
			}

			warnings := ctx.populateTextInSlide(slide, tt.content, "slideLayout1")

			if len(warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.wantWarnings, warnings)
			}

			// Check title text if expected
			if tt.wantTitleText != "" {
				titleShape := slide.CommonSlideData.ShapeTree.Shapes[0]
				if titleShape.TextBody == nil || len(titleShape.TextBody.Paragraphs) == 0 {
					t.Errorf("expected title text, got nil or empty")
				} else if len(titleShape.TextBody.Paragraphs[0].Runs) > 0 {
					gotText := titleShape.TextBody.Paragraphs[0].Runs[0].Text
					if gotText != tt.wantTitleText {
						t.Errorf("title text = %q, want %q", gotText, tt.wantTitleText)
					}
				}
			}

			// Check bullets count if expected
			if tt.wantBodyBullets > 0 {
				bodyShape := slide.CommonSlideData.ShapeTree.Shapes[1]
				if bodyShape.TextBody == nil {
					t.Errorf("expected body bullets, got nil TextBody")
				} else if len(bodyShape.TextBody.Paragraphs) != tt.wantBodyBullets {
					t.Errorf("body paragraphs = %d, want %d", len(bodyShape.TextBody.Paragraphs), tt.wantBodyBullets)
				}
			}
		})
	}
}

// intPtr is a helper to create *int for tests
func intPtr(i int) *int {
	return &i
}

// TestSinglePassContext_PopulateTextInSlide_ByShapeName tests placeholder lookup by canonical name.
// After normalization, shapes have canonical names ("title", "body", "image") and lookup is name-only.
func TestSinglePassContext_PopulateTextInSlide_ByShapeName(t *testing.T) {
	// Create a mock slide with canonical shape names (post-normalization)
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{
								ID:   2,
								Name: "title",
							},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "title"},
							},
						},
						TextBody: &textBodyXML{},
					},
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{
								ID:   3,
								Name: "body",
							},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "body", Index: intPtr(1)},
							},
						},
						TextBody: &textBodyXML{},
					},
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{
								ID:   11,
								Name: "image",
							},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "pic", Index: intPtr(13)},
							},
						},
						TextBody: &textBodyXML{},
					},
				},
			},
		},
	}

	ctx := &singlePassContext{}

	tests := []struct {
		name         string
		content      []ContentItem
		wantWarnings int
		wantText     string
		shapeIdx     int // which shape to check
	}{
		{
			name: "populate by canonical name - title",
			content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Named Title"},
			},
			wantWarnings: 0,
			wantText:     "Named Title",
			shapeIdx:     0,
		},
		{
			name: "populate by canonical name - body",
			content: []ContentItem{
				{PlaceholderID: "body", Type: ContentText, Value: "Named Content"},
			},
			wantWarnings: 0,
			wantText:     "Named Content",
			shapeIdx:     1,
		},
		{
			name: "nonexistent name produces warning",
			content: []ContentItem{
				{PlaceholderID: "nonexistent", Type: ContentText, Value: "Should warn"},
			},
			wantWarnings: 1,
			wantText:     "",
			shapeIdx:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset slide state
			for i := range slide.CommonSlideData.ShapeTree.Shapes {
				slide.CommonSlideData.ShapeTree.Shapes[i].TextBody = &textBodyXML{}
			}

			warnings := ctx.populateTextInSlide(slide, tt.content, "slideLayout1")

			if len(warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.wantWarnings, warnings)
			}

			// Check that the correct shape got the text
			if tt.wantText != "" {
				shape := slide.CommonSlideData.ShapeTree.Shapes[tt.shapeIdx]
				if shape.TextBody == nil || len(shape.TextBody.Paragraphs) == 0 {
					t.Errorf("expected text in shape %d, got nil or empty", tt.shapeIdx)
				} else if len(shape.TextBody.Paragraphs[0].Runs) > 0 {
					gotText := shape.TextBody.Paragraphs[0].Runs[0].Text
					if gotText != tt.wantText {
						t.Errorf("shape %d text = %q, want %q", tt.shapeIdx, gotText, tt.wantText)
					}
				}
			}
		})
	}
}

// TestSinglePassContext_InsertNativeSVGPics tests the SVG picture insertion
func TestSinglePassContext_InsertNativeSVGPics(t *testing.T) {
	// Create minimal slide XML
	slideData := []byte(`<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp id="10">existing shape</p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	ctx := newSinglePassContext("", nil, nil, false, nil)

	nativeSVGs := []nativeSVGInsert{
		{
			pngRelID: "rId10",
			svgRelID: "rId11",
			offsetX:  1000000,
			offsetY:  2000000,
			extentCX: 3000000,
			extentCY: 4000000,
		},
	}

	result, err := ctx.insertNativeSVGPics(1, slideData, nativeSVGs)
	if err != nil {
		t.Fatalf("insertNativeSVGPics failed: %v", err)
	}

	resultStr := string(result)

	// Verify p:pic was inserted
	if !strings.Contains(resultStr, "p:pic") {
		t.Error("expected p:pic element to be inserted")
	}

	// Verify the closing tag is still present
	if !strings.Contains(resultStr, "</p:spTree>") {
		t.Error("expected </p:spTree> closing tag to be preserved")
	}

	// Verify relationship IDs are present
	if !strings.Contains(resultStr, "rId10") || !strings.Contains(resultStr, "rId11") {
		t.Error("expected relationship IDs to be present")
	}

	t.Logf("Result length: %d bytes", len(result))
}

// TestSinglePassContext_InsertNativeSVGPics_NoSpTreeError tests error when spTree is missing
func TestSinglePassContext_InsertNativeSVGPics_NoSpTreeError(t *testing.T) {
	// Slide XML without spTree closing tag
	slideData := []byte(`<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
  </p:cSld>
</p:sld>`)

	ctx := newSinglePassContext("", nil, nil, false, nil)

	nativeSVGs := []nativeSVGInsert{
		{pngRelID: "rId10", svgRelID: "rId11"},
	}

	_, err := ctx.insertNativeSVGPics(1, slideData, nativeSVGs)
	if err == nil {
		t.Error("expected error when spTree is missing, got nil")
	}
	if !strings.Contains(err.Error(), "spTree") {
		t.Errorf("expected error about spTree, got: %v", err)
	}
}

// TestMediaRel tests the mediaRel struct for both file and byte-based media
func TestMediaRel_Structure(t *testing.T) {
	// File-based media
	fileRel := mediaRel{
		imagePath:     "/path/to/image.png",
		mediaFileName: "image1.png",
		data:          nil,
	}

	if fileRel.imagePath == "" {
		t.Error("file-based mediaRel should have imagePath")
	}
	if fileRel.data != nil {
		t.Error("file-based mediaRel should have nil data")
	}

	// Byte-based media (charts)
	chartData := []byte("fake PNG data")
	byteRel := mediaRel{
		imagePath:     "",
		mediaFileName: "chart1.png",
		data:          chartData,
	}

	if byteRel.imagePath != "" {
		t.Error("byte-based mediaRel should have empty imagePath")
	}
	if byteRel.data == nil {
		t.Error("byte-based mediaRel should have data")
	}
}

// TestNativeSVGInsert_Structure tests the nativeSVGInsert struct
func TestNativeSVGInsert_Structure(t *testing.T) {
	insert := nativeSVGInsert{
		svgPath:        "/path/to/chart.svg",
		pngPath:        "/tmp/chart-fallback.png",
		svgMediaFile:   "image1.svg",
		pngMediaFile:   "image2.png",
		svgRelID:       "rId5",
		pngRelID:       "rId4",
		offsetX:        914400,
		offsetY:        914400,
		extentCX:       4572000,
		extentCY:       3429000,
		shapeID:        100,
		placeholderIdx: 2,
	}

	// Verify all fields are populated
	if insert.svgPath == "" || insert.pngPath == "" {
		t.Error("paths should be set")
	}
	if insert.svgMediaFile == "" || insert.pngMediaFile == "" {
		t.Error("media files should be set")
	}
	if insert.svgRelID == "" || insert.pngRelID == "" {
		t.Error("relationship IDs should be set")
	}
	if insert.extentCX <= 0 || insert.extentCY <= 0 {
		t.Error("extents should be positive")
	}
}

// TestSlideFileRegexSP tests the slide file matching regex
// Note: The regex does not use a $ anchor, so it matches prefixes.
// This is intentional behavior for the current implementation.
func TestSlideFileRegexSP(t *testing.T) {
	tests := []struct {
		path    string
		matches bool
	}{
		{"ppt/slides/slide1.xml", true},
		{"ppt/slides/slide10.xml", true},
		{"ppt/slides/slide999.xml", true},
		{"ppt/slides/slide.xml", false},
		{"ppt/slides/slideA.xml", false},
		{"ppt/slideLayouts/slideLayout1.xml", false},
		// Note: .rels files also match because regex has no $ anchor
		// This is acceptable because the regex is used in a context where
		// we iterate over ZIP entries and the .rels files are handled separately
		{"ppt/slides/slide1.xml.rels", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := slideFileRegexSP.MatchString(tt.path)
			if got != tt.matches {
				t.Errorf("slideFileRegexSP.MatchString(%q) = %v, want %v", tt.path, got, tt.matches)
			}
		})
	}
}

// TestGenerateSinglePass_ErrorHandling tests error cases for generateSinglePass
func TestGenerateSinglePass_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		req        GenerationRequest
		wantErrMsg string
	}{
		{
			name: "template not found",
			req: GenerationRequest{
				TemplatePath: "/nonexistent/template.pptx",
				OutputPath:   filepath.Join(t.TempDir(), "output.pptx"),
				Slides:       []SlideSpec{{LayoutID: "slideLayout1"}},
			},
			wantErrMsg: "failed to open template",
		},
		{
			name: "invalid output directory",
			req: GenerationRequest{
				TemplatePath: "../template/testdata/standard.pptx",
				OutputPath:   "/nonexistent/dir/output.pptx",
				Slides:       []SlideSpec{{LayoutID: "slideLayout1"}},
			},
			wantErrMsg: "failed to create output file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if template doesn't exist for tests that need it
			if tt.req.TemplatePath == "../template/testdata/standard.pptx" {
				if _, err := os.Stat(tt.req.TemplatePath); os.IsNotExist(err) {
					t.Skip("test template not found")
				}
			}

			_, _, err := generateSinglePass(context.Background(), tt.req)
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErrMsg)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrMsg)
			}
		})
	}
}

// TestGenerateSinglePass_LayoutNotFound tests error when layout is not found
func TestGenerateSinglePass_LayoutNotFound(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{LayoutID: "nonExistentLayout", Content: []ContentItem{}},
		},
	}

	_, _, err := generateSinglePass(context.Background(), req)
	if err == nil {
		t.Error("expected error for non-existent layout, got nil")
		return
	}
	if !strings.Contains(err.Error(), "not found in template") {
		t.Errorf("error = %q, want to contain 'not found in template'", err.Error())
	}
	if !strings.Contains(err.Error(), "nonExistentLayout") {
		t.Errorf("error = %q, want to contain the invalid layout ID", err.Error())
	}
	if !strings.Contains(err.Error(), "available layouts:") {
		t.Errorf("error = %q, want to contain 'available layouts:'", err.Error())
	}
}

// TestGenerateSinglePass_BasicGeneration tests successful generation
func TestGenerateSinglePass_BasicGeneration(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")

	req := GenerationRequest{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Slides: []SlideSpec{
			{
				LayoutID: "slideLayout1",
				Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: "Test Title"},
				},
			},
			{
				LayoutID: "slideLayout2",
				Content: []ContentItem{
					{PlaceholderID: "title", Type: ContentText, Value: "Slide 2"},
					{PlaceholderID: "body", Type: ContentBullets, Value: []string{"Item 1", "Item 2"}},
				},
			},
		},
	}

	result, warnings, err := generateSinglePass(context.Background(), req)
	if err != nil {
		t.Fatalf("generateSinglePass failed: %v", err)
	}

	// Verify result
	if result.SlideCount != 2 {
		t.Errorf("SlideCount = %d, want 2", result.SlideCount)
	}
	if result.FileSize <= 0 {
		t.Error("FileSize should be > 0")
	}
	if result.OutputPath != outputPath {
		t.Errorf("OutputPath = %q, want %q", result.OutputPath, outputPath)
	}

	// Verify output file exists and is valid
	r, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("output is not a valid PPTX: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Count slides
	slideCount := 0
	for _, f := range r.File {
		if matched, _ := filepath.Match("ppt/slides/slide*.xml", f.Name); matched {
			slideCount++
		}
	}

	if slideCount < 2 {
		t.Errorf("expected at least 2 slides in output, found %d", slideCount)
	}

	t.Logf("Generated %d slides with %d warnings", result.SlideCount, len(warnings))
}

// TestGenerateSinglePass_SVGStrategy tests SVG strategy configuration
func TestGenerateSinglePass_SVGStrategy(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		svgStrategy string
		svgScale    float64
	}{
		{"default PNG strategy", "", 0},
		{"explicit PNG strategy", "png", 2.0},
		{"EMF strategy", "emf", 1.0},
		{"native strategy", "native", 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := filepath.Join(tmpDir, tt.name+".pptx")

			req := GenerationRequest{
				TemplatePath: templatePath,
				OutputPath:   outputPath,
				Slides: []SlideSpec{
					{LayoutID: "slideLayout1", Content: []ContentItem{}},
				},
				SVGStrategy: tt.svgStrategy,
				SVGScale:    tt.svgScale,
			}

			result, _, err := generateSinglePass(context.Background(), req)
			if err != nil {
				t.Fatalf("generateSinglePass failed: %v", err)
			}

			if result.SlideCount != 1 {
				t.Errorf("SlideCount = %d, want 1", result.SlideCount)
			}
		})
	}
}

// TestSinglePassContext_AllocateNativeSVGRelIDs tests relationship ID allocation
func TestSinglePassContext_AllocateNativeSVGRelIDs(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)

	// Add native SVG inserts for slide 3 (new slide)
	ctx.nativeSVGInserts[3] = []nativeSVGInsert{
		{svgPath: "/path/to/svg1.svg", pngPath: "/path/to/png1.png"},
		{svgPath: "/path/to/svg2.svg", pngPath: "/path/to/png2.png"},
	}

	// Also add some regular media to slide 3
	ctx.slideRelUpdates[3] = []mediaRel{
		{imagePath: "/path/to/image.png", mediaFileName: "image1.png"},
	}

	err = ctx.allocateNativeSVGRelIDs()
	if err != nil {
		t.Fatalf("allocateNativeSVGRelIDs failed: %v", err)
	}

	// Verify IDs were allocated
	for i, svg := range ctx.nativeSVGInserts[3] {
		if svg.pngRelID == "" {
			t.Errorf("native SVG %d: pngRelID not allocated", i)
		}
		if svg.svgRelID == "" {
			t.Errorf("native SVG %d: svgRelID not allocated", i)
		}
		t.Logf("SVG %d: pngRelID=%s, svgRelID=%s", i, svg.pngRelID, svg.svgRelID)
	}
}

// TestStreamBytesToZip_EdgeCases tests edge cases for streaming bytes to ZIP
func TestStreamBytesToZip_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		zipPath string
	}{
		{
			name:    "empty data",
			data:    []byte{},
			zipPath: "ppt/media/empty.png",
		},
		{
			name:    "small data",
			data:    []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
			zipPath: "ppt/media/small.png",
		},
		{
			name:    "1MB data",
			data:    make([]byte, 1024*1024),
			zipPath: "ppt/media/large.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer func() { _ = tmpFile.Close() }()

			w := zip.NewWriter(tmpFile)

			err = streamBytesToZip(w, tt.data, tt.zipPath)
			if err != nil {
				t.Fatalf("streamBytesToZip failed: %v", err)
			}

			if err := w.Close(); err != nil {
				t.Fatalf("failed to close ZIP: %v", err)
			}

			// Verify contents
			_ = tmpFile.Close()
			r, err := zip.OpenReader(tmpFile.Name())
			if err != nil {
				t.Fatalf("failed to open ZIP: %v", err)
			}
			defer func() { _ = r.Close() }()

			found := false
			for _, f := range r.File {
				if f.Name == tt.zipPath {
					found = true
					if f.UncompressedSize64 != uint64(len(tt.data)) {
						t.Errorf("size mismatch: got %d, want %d", f.UncompressedSize64, len(tt.data))
					}
					break
				}
			}

			if !found {
				t.Errorf("file %q not found in ZIP", tt.zipPath)
			}
		})
	}
}

// TestUpdateContentTypes_Singlepass tests updating [Content_Types].xml with image extensions
func TestUpdateContentTypes_Singlepass(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()
	idx := utils.BuildZipIndex(&reader.Reader)

	tests := []struct {
		name       string
		extensions map[string]bool
		wantPNG    bool
		wantJPG    bool
		wantSVG    bool
	}{
		{
			name:       "add PNG extension",
			extensions: map[string]bool{"png": true},
			wantPNG:    true,
		},
		{
			name:       "add multiple extensions",
			extensions: map[string]bool{"png": true, "jpg": true, "svg": true},
			wantPNG:    true,
			wantJPG:    true,
			wantSVG:    true,
		},
		{
			name:       "empty extensions",
			extensions: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := updateContentTypes(idx, tt.extensions)
			if err != nil {
				t.Fatalf("updateContentTypes failed: %v", err)
			}

			resultStr := string(result)

			if tt.wantPNG && !strings.Contains(resultStr, "image/png") {
				t.Error("expected PNG content type")
			}
			if tt.wantJPG && !strings.Contains(resultStr, "image/jpeg") {
				t.Error("expected JPEG content type")
			}
			if tt.wantSVG && !strings.Contains(resultStr, "image/svg+xml") {
				t.Error("expected SVG content type")
			}
		})
	}
}

// TestAddSlideContentTypeOverrides tests that slide content type overrides are added correctly.
func TestAddSlideContentTypeOverrides(t *testing.T) {
	// Build a minimal [Content_Types].xml with existing slide overrides (like a 2-slide template)
	baseContentTypes := pptx.ContentTypesXML{
		XMLName: xml.Name{Space: pptx.NsContentTypes, Local: "Types"},
		Defaults: []pptx.ContentTypeDefault{
			{Extension: "rels", ContentType: "application/vnd.openxmlformats-package.relationships+xml"},
			{Extension: "xml", ContentType: "application/xml"},
		},
		Overrides: []pptx.ContentTypeOverride{
			{PartName: "/ppt/presentation.xml", ContentType: pptx.ContentTypePresentationMain},
			{PartName: "/ppt/slides/slide1.xml", ContentType: pptx.ContentTypeSlide},
			{PartName: "/ppt/slides/slide2.xml", ContentType: pptx.ContentTypeSlide},
		},
	}
	baseCTData, err := xml.MarshalIndent(baseContentTypes, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal base content types: %v", err)
	}
	baseCTData = append([]byte(xml.Header), baseCTData...)

	t.Run("appends overrides for new slides", func(t *testing.T) {
		slideContentMap := map[int]SlideSpec{3: {}, 4: {}, 5: {}}
		result, err := addSlideContentTypeOverrides(baseCTData, slideContentMap, 2, false)
		if err != nil {
			t.Fatalf("addSlideContentTypeOverrides failed: %v", err)
		}
		resultStr := string(result)
		// Original slides preserved
		if !strings.Contains(resultStr, "/ppt/slides/slide1.xml") {
			t.Error("expected slide1 override to be preserved")
		}
		if !strings.Contains(resultStr, "/ppt/slides/slide2.xml") {
			t.Error("expected slide2 override to be preserved")
		}
		// New slides added
		for _, n := range []int{3, 4, 5} {
			partName := fmt.Sprintf("/ppt/slides/slide%d.xml", n)
			if !strings.Contains(resultStr, partName) {
				t.Errorf("expected override for %s", partName)
			}
		}
	})

	t.Run("excludeTemplateSlides removes old and adds new", func(t *testing.T) {
		slideContentMap := map[int]SlideSpec{1: {}, 2: {}, 3: {}}
		result, err := addSlideContentTypeOverrides(baseCTData, slideContentMap, 2, true)
		if err != nil {
			t.Fatalf("addSlideContentTypeOverrides failed: %v", err)
		}
		// Parse result to check overrides precisely
		var ct pptx.ContentTypesXML
		if err := xml.Unmarshal(result, &ct); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		slideOverrides := 0
		for _, ovr := range ct.Overrides {
			if ovr.ContentType == pptx.ContentTypeSlide {
				slideOverrides++
			}
		}
		if slideOverrides != 3 {
			t.Errorf("expected 3 slide overrides, got %d", slideOverrides)
		}
	})

	t.Run("does not duplicate existing overrides", func(t *testing.T) {
		// Slide 1 and 2 already exist in base — this tests the no-dup path
		slideContentMap := map[int]SlideSpec{1: {}, 2: {}}
		result, err := addSlideContentTypeOverrides(baseCTData, slideContentMap, 2, false)
		if err != nil {
			t.Fatalf("addSlideContentTypeOverrides failed: %v", err)
		}
		var ct pptx.ContentTypesXML
		if err := xml.Unmarshal(result, &ct); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		slideOverrides := 0
		for _, ovr := range ct.Overrides {
			if ovr.ContentType == pptx.ContentTypeSlide {
				slideOverrides++
			}
		}
		if slideOverrides != 2 {
			t.Errorf("expected 2 slide overrides (no duplicates), got %d", slideOverrides)
		}
	})
}

// TestCreateSlideFromLayout tests creating slide XML from layout
func TestCreateSlideFromLayout(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Find a layout file
	var layoutData []byte
	for _, f := range reader.File {
		if strings.HasPrefix(f.Name, "ppt/slideLayouts/slideLayout") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open layout: %v", err)
			}
			layoutData, err = readAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("failed to read layout: %v", err)
			}
			break
		}
	}

	if layoutData == nil {
		t.Skip("no layout file found in template")
	}

	// Create slide from layout (nil master positions - testing without inheritance)
	slide, err := createSlideFromLayout(layoutData, 5, nil)
	if err != nil {
		t.Fatalf("createSlideFromLayout failed: %v", err)
	}

	// Verify it has the expected name
	if slide.CommonSlideData.Name != "Slide 5" {
		t.Errorf("slide name = %q, want 'Slide 5'", slide.CommonSlideData.Name)
	}

	t.Logf("Created slide with %d shapes", len(slide.CommonSlideData.ShapeTree.Shapes))
}

// readAll is a helper that reads all data from a reader
func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var result []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}
	return result, nil
}

// TestBuildPlaceholderMap tests the shared placeholder map building function
func TestBuildPlaceholderMap(t *testing.T) {
	// After normalization, buildPlaceholderMap maps only by canonical shape name.
	tests := []struct {
		name           string
		shapes         []shapeXML
		wantKeys       []string
		wantMissingKey string
	}{
		{
			name: "canonical name lookup",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "title"},
					},
				},
			},
			wantKeys: []string{"title"},
		},
		{
			name: "body placeholder by name",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: intPtr(1)}},
					},
				},
			},
			wantKeys:       []string{"body"},
			wantMissingKey: "1", // index-based lookup no longer supported
		},
		{
			name: "multiple canonical names",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "title"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "title"}},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: intPtr(1)}},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "body_2"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: intPtr(2)}},
					},
				},
			},
			wantKeys: []string{"title", "body", "body_2"},
		},
		{
			name: "non-placeholder shapes also mapped",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Decorative Line"},
					},
				},
			},
			wantKeys: []string{"Decorative Line"},
		},
		{
			name:           "empty shapes returns empty map",
			shapes:         []shapeXML{},
			wantKeys:       []string{},
			wantMissingKey: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPlaceholderMap(tt.shapes)

			// Verify expected keys exist
			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("buildPlaceholderMap() missing expected key %q", key)
				}
			}

			// Verify missing key doesn't exist
			if tt.wantMissingKey != "" {
				if _, ok := result[tt.wantMissingKey]; ok {
					t.Errorf("buildPlaceholderMap() should not have key %q", tt.wantMissingKey)
				}
			}
		})
	}
}

// TestBuildPlaceholderMap_IndexValues verifies correct shape indices are mapped by name
func TestBuildPlaceholderMap_IndexValues(t *testing.T) {
	shapes := []shapeXML{
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "First Shape"},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body_2"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Index: intPtr(10)}},
			},
		},
	}

	result := buildPlaceholderMap(shapes)

	tests := []struct {
		key       string
		wantIndex int
	}{
		{"First Shape", 0},
		{"body", 1},
		{"body_2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			gotIndex, ok := result[tt.key]
			if !ok {
				t.Errorf("key %q not found in map", tt.key)
				return
			}
			if gotIndex != tt.wantIndex {
				t.Errorf("key %q maps to index %d, want %d", tt.key, gotIndex, tt.wantIndex)
			}
		})
	}
}

// TestGetPlaceholderBounds tests bounds extraction from shapes
func TestGetPlaceholderBounds(t *testing.T) {
	tests := []struct {
		name           string
		shape          *shapeXML
		explicitBounds *types.BoundingBox
		wantBounds     types.BoundingBox
	}{
		{
			name: "extract from shape transform",
			shape: &shapeXML{
				ShapeProperties: shapePropertiesXML{
					Transform: &transformXML{
						Offset: offsetXML{X: 100, Y: 200},
						Extent: extentXML{CX: 300, CY: 400},
					},
				},
			},
			explicitBounds: nil,
			wantBounds:     types.BoundingBox{X: 100, Y: 200, Width: 300, Height: 400},
		},
		{
			name: "explicit bounds override shape transform",
			shape: &shapeXML{
				ShapeProperties: shapePropertiesXML{
					Transform: &transformXML{
						Offset: offsetXML{X: 100, Y: 200},
						Extent: extentXML{CX: 300, CY: 400},
					},
				},
			},
			explicitBounds: &types.BoundingBox{X: 500, Y: 600, Width: 700, Height: 800},
			wantBounds:     types.BoundingBox{X: 500, Y: 600, Width: 700, Height: 800},
		},
		{
			name: "nil transform returns zero bounds",
			shape: &shapeXML{
				ShapeProperties: shapePropertiesXML{Transform: nil},
			},
			explicitBounds: nil,
			wantBounds:     types.BoundingBox{},
		},
		{
			name: "nil transform with explicit bounds",
			shape: &shapeXML{
				ShapeProperties: shapePropertiesXML{Transform: nil},
			},
			explicitBounds: &types.BoundingBox{X: 10, Y: 20, Width: 30, Height: 40},
			wantBounds:     types.BoundingBox{X: 10, Y: 20, Width: 30, Height: 40},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPlaceholderBounds(tt.shape, tt.explicitBounds)
			if got != tt.wantBounds {
				t.Errorf("getPlaceholderBounds() = %+v, want %+v", got, tt.wantBounds)
			}
		})
	}
}

// TestAllocateMediaSlot tests media slot allocation
func TestAllocateMediaSlot(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	// Pre-seed the allocator so numbering starts at 5 (simulate 4 existing images)
	ctx.media.ScanPaths([]string{"ppt/media/image4.png"})

	tests := []struct {
		name              string
		imagePath         string
		wantMediaFile     string
		wantNextAfter     int
		wantExtensionUsed string
	}{
		{
			name:              "new PNG file",
			imagePath:         "/path/to/image.png",
			wantMediaFile:     "image5.png",
			wantNextAfter:     6,
			wantExtensionUsed: "png",
		},
		{
			name:              "same file returns cached",
			imagePath:         "/path/to/image.png",
			wantMediaFile:     "image5.png",
			wantNextAfter:     6, // no increment
			wantExtensionUsed: "png",
		},
		{
			name:              "new JPG file",
			imagePath:         "/path/to/photo.jpg",
			wantMediaFile:     "image6.jpg",
			wantNextAfter:     7,
			wantExtensionUsed: "jpg",
		},
		{
			name:              "file without extension defaults to png",
			imagePath:         "/path/to/noext",
			wantMediaFile:     "image7.png",
			wantNextAfter:     8,
			wantExtensionUsed: "png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctx.allocateMediaSlot(tt.imagePath)

			if got != tt.wantMediaFile {
				t.Errorf("allocateMediaSlot() = %q, want %q", got, tt.wantMediaFile)
			}
			if ctx.nextMediaNum() != tt.wantNextAfter {
				t.Errorf("nextMediaNum() = %d, want %d", ctx.nextMediaNum(), tt.wantNextAfter)
			}
			if !ctx.usedExtensions[tt.wantExtensionUsed] {
				t.Errorf("extension %q not marked as used", tt.wantExtensionUsed)
			}
		})
	}
}

// TestResolveDiagramBytes tests diagram byte resolution from DiagramSpec
func TestResolveDiagramBytes(t *testing.T) {
	t.Run("valid DiagramSpec renders successfully", func(t *testing.T) {
		ctx := newSinglePassContext("", nil, nil, false, nil)
		item := ContentItem{
			PlaceholderID: "chart1",
			Type:          ContentDiagram,
			Value: &types.DiagramSpec{
				Type:  "bar_chart",
				Title: "Test",
				Data:  map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Data", "values": []float64{10, 20}}}},
			},
		}
		gotBytes, gotOK := ctx.resolveDiagramBytes(1, item, types.BoundingBox{})
		if !gotOK {
			t.Error("resolveDiagramBytes() expected ok=true for valid DiagramSpec")
		}
		if len(gotBytes) == 0 {
			t.Error("resolveDiagramBytes() returned empty bytes for valid DiagramSpec")
		}
	})

	t.Run("invalid value type adds warning", func(t *testing.T) {
		ctx := newSinglePassContext("", nil, nil, false, nil)
		item := ContentItem{
			PlaceholderID: "chart2",
			Type:          ContentDiagram,
			Value:         "invalid-string-value",
		}
		gotBytes, gotOK := ctx.resolveDiagramBytes(2, item, types.BoundingBox{})
		if gotOK {
			t.Error("resolveDiagramBytes() expected ok=false for invalid value type")
		}
		if gotBytes != nil {
			t.Errorf("resolveDiagramBytes() expected nil bytes, got %d bytes", len(gotBytes))
		}
		if len(ctx.warnings) != 1 {
			t.Errorf("resolveDiagramBytes() expected 1 warning, got %d", len(ctx.warnings))
		}
	})
}

// TestPrepareImages_DiagramContent tests diagram processing in prepareImages
func TestPrepareImages_DiagramContent(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.svgConverter = NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyNative,
		Scale:    DefaultSVGScale,
	})

	diagramSpec := &types.DiagramSpec{
		Type:  "bar_chart",
		Title: "Test",
		Data:  map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Data", "values": []float64{10, 20}}}},
	}

	// Create a slide with a chart placeholder
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "chart_placeholder"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "chart"}},
						},
						TextBody: &textBodyXML{Paragraphs: []paragraphXML{{}}},
					},
				},
			},
		},
	}
	ctx.templateSlideData[1] = slide
	ctx.slideContentMap[1] = SlideSpec{
		Content: []ContentItem{
			{
				PlaceholderID: "chart_placeholder",
				Type:          ContentDiagram,
				Value:         diagramSpec,
			},
		},
	}

	err := ctx.prepareImages()
	if err != nil {
		t.Fatalf("prepareImages() error = %v", err)
	}

	// With native SVG strategy (default), diagrams embed as native SVG + PNG fallback
	if len(ctx.nativeSVGInserts[1]) != 1 {
		t.Errorf("expected 1 native SVG insert, got %d", len(ctx.nativeSVGInserts[1]))
	}

	if !ctx.usedExtensions["svg"] {
		t.Error("svg extension should be marked as used")
	}
	if !ctx.usedExtensions["png"] {
		t.Error("png extension should be marked as used")
	}

	// Verify placeholder index is tracked for removal during slide writing
	insert := ctx.nativeSVGInserts[1][0]
	if insert.placeholderIdx != 0 {
		t.Errorf("placeholderIdx = %d, want 0", insert.placeholderIdx)
	}
	if len(insert.svgData) == 0 {
		t.Error("expected non-empty SVG data")
	}
	if len(insert.pngData) == 0 {
		t.Error("expected non-empty PNG fallback data")
	}
}

// TestProcessDiagramContent_PreservesPosition verifies that diagram position is preserved from placeholder transform.
// With FitMode "contain" (auto-applied to all diagrams), the chart is aspect-ratio-fitted and
// centered within the placeholder bounds, so offsets and extents reflect the fitted dimensions.
func TestProcessDiagramContent_PreservesPosition(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.svgConverter = NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyNative,
		Scale:    DefaultSVGScale,
	})

	// Create a shape with explicit transform (position and size)
	// Placeholder: 5 inches wide x 3 inches tall (ratio ≈ 1.67)
	shape := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{
			ConnectionNonVisual: connectionNonVisualXML{Name: "chart_placeholder"},
			NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "chart"}},
		},
		ShapeProperties: shapePropertiesXML{
			Transform: &transformXML{
				Offset: offsetXML{X: 914400, Y: 1371600},    // 1 inch, 1.5 inches
				Extent: extentXML{CX: 4572000, CY: 2743200}, // 5 inches, 3 inches
			},
		},
		TextBody: &textBodyXML{Paragraphs: []paragraphXML{{}}},
	}

	item := ContentItem{
		PlaceholderID: "chart_placeholder",
		Type:          ContentDiagram,
		Value: &types.DiagramSpec{
			Type:  "bar_chart",
			Title: "Test",
			Data:  map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Data", "values": []float64{10, 20}}}},
		},
	}

	ctx.processDiagramContent(1, item, shape, 0, newPlaceholderResolver([]shapeXML{*shape}))

	// With native SVG strategy, diagrams embed as native SVG + PNG fallback
	if len(ctx.nativeSVGInserts[1]) != 1 {
		t.Fatalf("expected 1 native SVG insert, got %d", len(ctx.nativeSVGInserts[1]))
	}

	// With placeholder-matched render dimensions, the content ratio closely
	// matches the placeholder ratio, so "contain" mode fills the full placeholder.
	insert := ctx.nativeSVGInserts[1][0]

	// Position and size should approximately match the placeholder bounds
	if insert.offsetX != 914400 {
		t.Errorf("native SVG offset X = %d, want 914400", insert.offsetX)
	}
	if insert.offsetY != 1371600 {
		t.Errorf("native SVG offset Y = %d, want 1371600", insert.offsetY)
	}
	if insert.extentCX != 4572000 {
		t.Errorf("native SVG extent CX = %d, want 4572000", insert.extentCX)
	}
	if insert.extentCY != 2743200 {
		t.Errorf("native SVG extent CY = %d, want 2743200", insert.extentCY)
	}
	if insert.placeholderIdx != 0 {
		t.Errorf("native SVG placeholderIdx = %d, want 0", insert.placeholderIdx)
	}
}

// TestProcessDiagramContent_TracksPositionForPicInsertion verifies diagram records position for p:pic generation
func TestProcessDiagramContent_TracksPositionForPicInsertion(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.svgConverter = NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyNative,
		Scale:    DefaultSVGScale,
	})

	// Create a shape without transform (will have zero bounds)
	shape := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{
			ConnectionNonVisual: connectionNonVisualXML{Name: "chart_placeholder"},
			NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "chart"}},
		},
		ShapeProperties: shapePropertiesXML{
			// No Transform - simulates placeholder without explicit bounds
		},
		TextBody: &textBodyXML{Paragraphs: []paragraphXML{{}}},
	}

	item := ContentItem{
		PlaceholderID: "chart_placeholder",
		Type:          ContentDiagram,
		Value: &types.DiagramSpec{
			Type:  "bar_chart",
			Title: "Test",
			Data:  map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Data", "values": []float64{10, 20}}}},
		},
	}

	ctx.processDiagramContent(1, item, shape, 0, newPlaceholderResolver([]shapeXML{*shape}))

	// With native SVG strategy, diagrams embed as native SVG + PNG fallback
	if len(ctx.nativeSVGInserts[1]) != 1 {
		t.Fatalf("expected 1 native SVG insert, got %d", len(ctx.nativeSVGInserts[1]))
	}

	insert := ctx.nativeSVGInserts[1][0]
	if insert.placeholderIdx != 0 {
		t.Errorf("placeholder index = %d, want 0", insert.placeholderIdx)
	}
	// Shape had no transform, so bounds should be zero
	if insert.extentCX != 0 || insert.extentCY != 0 {
		t.Errorf("expected zero dimensions for shape without transform, got cx=%d cy=%d", insert.extentCX, insert.extentCY)
	}
}

// TestRemoveNativeSVGPlaceholders tests removal of placeholder shapes being replaced by SVG
func TestRemoveNativeSVGPlaceholders(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)

	tests := []struct {
		name           string
		slide          *slideXML
		nativeSVGs     []nativeSVGInsert
		wantShapeCount int
		wantShapeNames []string
	}{
		{
			name: "remove single placeholder",
			slide: &slideXML{
				CommonSlideData: commonSlideDataXML{
					ShapeTree: shapeTreeXML{
						Shapes: []shapeXML{
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Title"}}},
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Image"}}},
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Footer"}}},
						},
					},
				},
			},
			nativeSVGs:     []nativeSVGInsert{{placeholderIdx: 1}},
			wantShapeCount: 2,
			wantShapeNames: []string{"Title", "Footer"},
		},
		{
			name: "remove multiple placeholders",
			slide: &slideXML{
				CommonSlideData: commonSlideDataXML{
					ShapeTree: shapeTreeXML{
						Shapes: []shapeXML{
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Title"}}},
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Image1"}}},
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Image2"}}},
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Footer"}}},
						},
					},
				},
			},
			nativeSVGs:     []nativeSVGInsert{{placeholderIdx: 1}, {placeholderIdx: 2}},
			wantShapeCount: 2,
			wantShapeNames: []string{"Title", "Footer"},
		},
		{
			name: "empty nativeSVGs - no changes",
			slide: &slideXML{
				CommonSlideData: commonSlideDataXML{
					ShapeTree: shapeTreeXML{
						Shapes: []shapeXML{
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Title"}}},
							{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "Content"}}},
						},
					},
				},
			},
			nativeSVGs:     []nativeSVGInsert{},
			wantShapeCount: 2,
			wantShapeNames: []string{"Title", "Content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctx.removeNativeSVGPlaceholders(tt.slide, tt.nativeSVGs)

			if len(result.CommonSlideData.ShapeTree.Shapes) != tt.wantShapeCount {
				t.Errorf("got %d shapes, want %d", len(result.CommonSlideData.ShapeTree.Shapes), tt.wantShapeCount)
			}

			for i, wantName := range tt.wantShapeNames {
				if i >= len(result.CommonSlideData.ShapeTree.Shapes) {
					t.Errorf("missing shape at index %d", i)
					continue
				}
				gotName := result.CommonSlideData.ShapeTree.Shapes[i].NonVisualProperties.ConnectionNonVisual.Name
				if gotName != wantName {
					t.Errorf("shape[%d] name = %q, want %q", i, gotName, wantName)
				}
			}
		})
	}
}

// TestAppendNativeSVGRelationships tests adding SVG+PNG relationships
func TestAppendNativeSVGRelationships(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)

	tests := []struct {
		name          string
		initialRels   int
		nativeSVGs    []nativeSVGInsert
		wantRelsCount int
	}{
		{
			name:        "add single SVG with PNG",
			initialRels: 2,
			nativeSVGs: []nativeSVGInsert{
				{pngRelID: "rId3", svgRelID: "rId4", pngMediaFile: "image1.png", svgMediaFile: "image2.svg"},
			},
			wantRelsCount: 4, // 2 initial + 2 (PNG + SVG)
		},
		{
			name:        "add multiple SVGs",
			initialRels: 1,
			nativeSVGs: []nativeSVGInsert{
				{pngRelID: "rId2", svgRelID: "rId3", pngMediaFile: "image1.png", svgMediaFile: "image2.svg"},
				{pngRelID: "rId4", svgRelID: "rId5", pngMediaFile: "image3.png", svgMediaFile: "image4.svg"},
			},
			wantRelsCount: 5, // 1 initial + 4 (2 PNG + 2 SVG)
		},
		{
			name:          "empty nativeSVGs - no change",
			initialRels:   3,
			nativeSVGs:    []nativeSVGInsert{},
			wantRelsCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rels := pptx.RelationshipsXML{}
			for i := 0; i < tt.initialRels; i++ {
				rels.Relationships = append(rels.Relationships, pptx.RelationshipXML{
					ID: fmt.Sprintf("rId%d", i+1),
				})
			}

			ctx.appendNativeSVGRelationships(&rels, tt.nativeSVGs)

			if len(rels.Relationships) != tt.wantRelsCount {
				t.Errorf("got %d relationships, want %d", len(rels.Relationships), tt.wantRelsCount)
			}

			// Verify relationship IDs are correct
			for _, svg := range tt.nativeSVGs {
				foundPNG := false
				foundSVG := false
				for _, rel := range rels.Relationships {
					if rel.ID == svg.pngRelID {
						foundPNG = true
						if !strings.Contains(rel.Target, svg.pngMediaFile) {
							t.Errorf("PNG rel %s target = %q, want to contain %q", svg.pngRelID, rel.Target, svg.pngMediaFile)
						}
					}
					if rel.ID == svg.svgRelID {
						foundSVG = true
						if !strings.Contains(rel.Target, svg.svgMediaFile) {
							t.Errorf("SVG rel %s target = %q, want to contain %q", svg.svgRelID, rel.Target, svg.svgMediaFile)
						}
					}
				}
				if !foundPNG {
					t.Errorf("PNG relationship %s not found", svg.pngRelID)
				}
				if !foundSVG {
					t.Errorf("SVG relationship %s not found", svg.svgRelID)
				}
			}
		})
	}
}


// TestProcessImageContent_EdgeCases tests edge cases in image content processing
func TestProcessImageContent_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		item         ContentItem
		wantWarnings int
	}{
		{
			name: "invalid image value type",
			item: ContentItem{
				PlaceholderID: "pic",
				Type:          ContentImage,
				Value:         "not-an-ImageContent",
			},
			wantWarnings: 1,
		},
		{
			name: "image file not found",
			item: ContentItem{
				PlaceholderID: "pic",
				Type:          ContentImage,
				Value:         ImageContent{Path: "/nonexistent/image.png"},
			},
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newSinglePassContext("", nil, []string{"/nonexistent"}, false, nil)

			slide := &slideXML{
				CommonSlideData: commonSlideDataXML{
					ShapeTree: shapeTreeXML{
						Shapes: []shapeXML{
							{
								NonVisualProperties: nonVisualPropertiesXML{
									ConnectionNonVisual: connectionNonVisualXML{Name: "pic"},
								},
								ShapeProperties: shapePropertiesXML{
									Transform: &transformXML{
										Offset: offsetXML{X: 100, Y: 100},
										Extent: extentXML{CX: 200, CY: 200},
									},
								},
							},
						},
					},
				},
			}

			ctx.templateSlideData[1] = slide
			ctx.slideContentMap[1] = SlideSpec{
				Content: []ContentItem{tt.item},
			}

			err := ctx.prepareImages()
			if err != nil {
				t.Fatalf("prepareImages() error = %v", err)
			}

			if len(ctx.warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d: %v", len(ctx.warnings), tt.wantWarnings, ctx.warnings)
			}
		})
	}
}

// TestProcessImageContent_SecurityValidation tests image path security validation
func TestProcessImageContent_SecurityValidation(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a test image
	testImg := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(testImg, []byte{0x89, 0x50, 0x4E, 0x47}, 0644); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	tests := []struct {
		name              string
		allowedPaths      []string
		imagePath         string
		wantSecurityError bool
	}{
		{
			name:              "path outside allowed paths",
			allowedPaths:      []string{"/other/path"},
			imagePath:         testImg,
			wantSecurityError: true,
		},
		{
			name:              "path within allowed paths",
			allowedPaths:      []string{tmpDir},
			imagePath:         testImg,
			wantSecurityError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newSinglePassContext("", nil, tt.allowedPaths, false, nil)

			slide := &slideXML{
				CommonSlideData: commonSlideDataXML{
					ShapeTree: shapeTreeXML{
						Shapes: []shapeXML{
							{
								NonVisualProperties: nonVisualPropertiesXML{
									ConnectionNonVisual: connectionNonVisualXML{Name: "pic"},
								},
								ShapeProperties: shapePropertiesXML{
									Transform: &transformXML{
										Offset: offsetXML{X: 100, Y: 100},
										Extent: extentXML{CX: 200, CY: 200},
									},
								},
							},
						},
					},
				},
			}

			ctx.templateSlideData[1] = slide
			ctx.slideContentMap[1] = SlideSpec{
				Content: []ContentItem{
					{
						PlaceholderID: "pic",
						Type:          ContentImage,
						Value:         ImageContent{Path: tt.imagePath},
					},
				},
			}

			err := ctx.prepareImages()
			if err != nil {
				t.Fatalf("prepareImages() error = %v", err)
			}

			hasSecurityWarning := false
			for _, w := range ctx.warnings {
				if strings.Contains(w, "security") {
					hasSecurityWarning = true
					break
				}
			}

			if hasSecurityWarning != tt.wantSecurityError {
				t.Errorf("security warning = %v, want %v, warnings: %v", hasSecurityWarning, tt.wantSecurityError, ctx.warnings)
			}
		})
	}
}

// TestPrepareImages_MissingPlaceholder tests warning when image placeholder is not found
func TestPrepareImages_MissingPlaceholder(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)

	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Title"},
						},
					},
				},
			},
		},
	}

	ctx.templateSlideData[1] = slide
	ctx.slideContentMap[1] = SlideSpec{
		Content: []ContentItem{
			{
				PlaceholderID: "nonexistent_image",
				Type:          ContentImage,
				Value:         ImageContent{Path: "/path/to/image.png"},
			},
		},
	}

	err := ctx.prepareImages()
	if err != nil {
		t.Fatalf("prepareImages() error = %v", err)
	}

	if len(ctx.warnings) != 1 {
		t.Errorf("expected 1 warning for missing placeholder, got %d: %v", len(ctx.warnings), ctx.warnings)
	}

	if !strings.Contains(ctx.warnings[0], "not found in layout") {
		t.Errorf("expected warning about missing placeholder, got: %s", ctx.warnings[0])
	}
	if !strings.Contains(ctx.warnings[0], "nonexistent_image") {
		t.Errorf("expected warning to include invalid placeholder ID, got: %s", ctx.warnings[0])
	}
	if !strings.Contains(ctx.warnings[0], "available placeholders:") {
		t.Errorf("expected warning to list available placeholders, got: %s", ctx.warnings[0])
	}
}

// TestProcessChartContent_FailedRendering tests chart rendering failure handling
func TestProcessChartContent_FailedRendering(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)

	shape := &shapeXML{
		NonVisualProperties: nonVisualPropertiesXML{
			NvPr: nvPrXML{Placeholder: &placeholderXML{Type: "chart"}},
		},
		TextBody: &textBodyXML{},
	}

	// Invalid DiagramSpec that will fail rendering
	item := ContentItem{
		PlaceholderID: "chart1",
		Type:          ContentDiagram,
		Value:         &types.DiagramSpec{Type: "bar_chart"}, // Minimal spec should fail
	}

	ctx.processDiagramContent(1, item, shape, 0, newPlaceholderResolver([]shapeXML{*shape}))

	// Should have a warning about failed chart rendering
	if len(ctx.warnings) == 0 {
		t.Error("expected warning for failed chart rendering")
	}

	// A placeholder image should be inserted for graceful degradation
	// (the insertDiagramPlaceholder feature adds a styled PNG so slides
	// don't show blank areas when chart rendering fails)
	if len(ctx.slideRelUpdates[1]) != 1 {
		t.Errorf("expected 1 placeholder media relationship, got %d", len(ctx.slideRelUpdates[1]))
	}
	if len(ctx.slideRelUpdates[1]) == 1 {
		rel := ctx.slideRelUpdates[1][0]
		if len(rel.data) == 0 {
			t.Error("placeholder image data should not be empty")
		}
	}
}

// TestWriteByteBasedMedia_Deduplication tests that duplicate media files are not written twice
func TestWriteByteBasedMedia_Deduplication(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.outputWriter = zip.NewWriter(tmpFile)

	// Add a file-based media (already "written")
	ctx.mediaFiles["/path/to/image.png"] = "image1.png"

	// Add byte-based media that references the same file
	ctx.slideRelUpdates[1] = []mediaRel{
		{mediaFileName: "image1.png", data: []byte("data1")}, // Should be skipped - already in mediaFiles
		{mediaFileName: "image2.png", data: []byte("data2")}, // Should be written
	}

	ctx.writeByteBasedMedia()

	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	// Verify only image2.png was written
	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	foundImage1 := false
	foundImage2 := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "image1.png") {
			foundImage1 = true
		}
		if strings.Contains(f.Name, "image2.png") {
			foundImage2 = true
		}
	}

	if foundImage1 {
		t.Error("image1.png should not be written (deduplicated)")
	}
	if !foundImage2 {
		t.Error("image2.png should be written")
	}
}

// TestBuildSVGConfig tests SVG configuration building with defaults
func TestBuildSVGConfig(t *testing.T) {
	tests := []struct {
		name           string
		req            GenerationRequest
		wantStrategy   string
		wantScale      float64
		wantCompatMode string
	}{
		{
			name:           "empty request uses defaults",
			req:            GenerationRequest{},
			wantStrategy:   "native",
			wantScale:      2.0, // DefaultSVGScale
			wantCompatMode: "ignore",
		},
		{
			name: "explicit values are used",
			req: GenerationRequest{
				SVGStrategy:     "native",
				SVGScale:        3.0,
				SVGNativeCompat: "strict",
			},
			wantStrategy:   "native",
			wantScale:      3.0,
			wantCompatMode: "strict",
		},
		{
			name: "zero scale uses default",
			req: GenerationRequest{
				SVGStrategy: "emf",
				SVGScale:    0,
			},
			wantStrategy:   "emf",
			wantScale:      2.0, // DefaultSVGScale
			wantCompatMode: "ignore",
		},
		{
			name: "negative scale uses default",
			req: GenerationRequest{
				SVGScale: -1.0,
			},
			wantStrategy:   "native",
			wantScale:      2.0, // DefaultSVGScale
			wantCompatMode: "ignore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svgCfg, compatMode := buildSVGConfig(tt.req)

			if string(svgCfg.Strategy) != tt.wantStrategy {
				t.Errorf("strategy = %q, want %q", svgCfg.Strategy, tt.wantStrategy)
			}
			if svgCfg.Scale != tt.wantScale {
				t.Errorf("scale = %f, want %f", svgCfg.Scale, tt.wantScale)
			}
			if string(compatMode) != tt.wantCompatMode {
				t.Errorf("compatMode = %q, want %q", compatMode, tt.wantCompatMode)
			}
		})
	}
}

// TestPopulateShapeText_UnsupportedType tests handling of unsupported content types
func TestPopulateShapeText_UnsupportedType(t *testing.T) {
	shape := &shapeXML{
		TextBody: &textBodyXML{},
	}

	item := ContentItem{
		PlaceholderID: "test",
		Type:          "unsupported_type",
		Value:         "some value",
	}

	err := populateShapeText(shape, item, -1, "")
	if err == nil {
		t.Error("expected error for unsupported content type")
	}
	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Errorf("error = %q, want to contain 'unsupported content type'", err.Error())
	}
}

// TestPopulateShapeText_NilTextBody tests that nil TextBody is initialized
func TestPopulateShapeText_NilTextBody(t *testing.T) {
	shape := &shapeXML{
		TextBody: nil,
	}

	item := ContentItem{
		PlaceholderID: "title",
		Type:          ContentText,
		Value:         "Test Title",
	}

	err := populateShapeText(shape, item, -1, "")
	if err != nil {
		t.Fatalf("populateShapeText() error = %v", err)
	}

	if shape.TextBody == nil {
		t.Error("TextBody should be initialized")
	}
	if len(shape.TextBody.Paragraphs) != 1 {
		t.Errorf("expected 1 paragraph, got %d", len(shape.TextBody.Paragraphs))
	}
}

// TestPopulateShapeText_BodyAndBullets tests the combined body+bullets content type.
// This ensures body text appears without a bullet marker followed by bulleted items.
func TestPopulateShapeText_BodyAndBullets(t *testing.T) {
	shape := &shapeXML{
		TextBody: nil,
	}

	item := ContentItem{
		PlaceholderID: "content",
		Type:          ContentBodyAndBullets,
		Value: BodyAndBulletsContent{
			Body:    "Introduction paragraph without bullet",
			Bullets: []string{"First point", "Second point", "Third point"},
		},
	}

	err := populateShapeText(shape, item, -1, "")
	if err != nil {
		t.Fatalf("populateShapeText() error = %v", err)
	}

	if shape.TextBody == nil {
		t.Fatal("TextBody should be initialized")
	}

	// Should have 4 paragraphs: 1 body + 3 bullets
	if len(shape.TextBody.Paragraphs) != 4 {
		t.Fatalf("expected 4 paragraphs, got %d", len(shape.TextBody.Paragraphs))
	}

	// First paragraph (body) should have buNone to suppress bullet marker from lstStyle
	if shape.TextBody.Paragraphs[0].Properties == nil {
		t.Error("body paragraph should have paragraph properties with buNone")
	} else if !strings.Contains(shape.TextBody.Paragraphs[0].Properties.Inner, "buNone") {
		t.Error("body paragraph should have buNone to suppress bullet marker")
	}
	if len(shape.TextBody.Paragraphs[0].Runs) == 0 || shape.TextBody.Paragraphs[0].Runs[0].Text != "Introduction paragraph without bullet" {
		t.Errorf("body text mismatch, got '%s'", shape.TextBody.Paragraphs[0].Runs[0].Text)
	}

	// Remaining paragraphs should have paragraph properties (for bullet level)
	// The actual bullet styling comes from lstStyle in the layout, we just set lvl="0"
	for i := 1; i <= 3; i++ {
		p := shape.TextBody.Paragraphs[i]
		if p.Properties == nil {
			t.Errorf("paragraph %d should have paragraph properties for bullet level", i)
		}
	}
}

// TestPopulateShapeText_BodyAndBullets_EmptyBody tests body+bullets with empty body text.
func TestPopulateShapeText_BodyAndBullets_EmptyBody(t *testing.T) {
	shape := &shapeXML{
		TextBody: nil,
	}

	item := ContentItem{
		PlaceholderID: "content",
		Type:          ContentBodyAndBullets,
		Value: BodyAndBulletsContent{
			Body:    "", // Empty body
			Bullets: []string{"First point", "Second point"},
		},
	}

	err := populateShapeText(shape, item, -1, "")
	if err != nil {
		t.Fatalf("populateShapeText() error = %v", err)
	}

	// Should have only 2 paragraphs (bullets only, no empty body paragraph)
	if len(shape.TextBody.Paragraphs) != 2 {
		t.Fatalf("expected 2 paragraphs (no empty body), got %d", len(shape.TextBody.Paragraphs))
	}

	// Both should have paragraph properties (for bullet level)
	for i, p := range shape.TextBody.Paragraphs {
		if p.Properties == nil {
			t.Errorf("paragraph %d should have paragraph properties for bullet level", i)
		}
	}
}

// TestPopulateShapeText_BulletGroups tests the bullet groups content type with body.
// When the first group has a header, the order is: header → body → bullets
// Body is always rendered before groups (it is intro text from the markdown source).
func TestPopulateShapeText_BulletGroups(t *testing.T) {
	shape := &shapeXML{
		TextBody: nil,
	}

	item := ContentItem{
		PlaceholderID: "content",
		Type:          ContentBulletGroups,
		Value: BulletGroupsContent{
			Body: "Our Q4 performance exceeded expectations with strong growth.",
			Groups: []BulletGroup{
				{
					Header:  "**Key Highlights:**",
					Bullets: []string{"Revenue increased 25%", "Customer satisfaction high"},
				},
			},
		},
	}

	err := populateShapeText(shape, item, -1, "")
	if err != nil {
		t.Fatalf("populateShapeText() error = %v", err)
	}

	if shape.TextBody == nil {
		t.Fatal("TextBody should be initialized")
	}

	// Should have 4 paragraphs: 1 body + 1 header + 2 bullets
	if len(shape.TextBody.Paragraphs) != 4 {
		t.Fatalf("expected 4 paragraphs (body + header + 2 bullets), got %d", len(shape.TextBody.Paragraphs))
	}

	// First paragraph (body intro) should have buNone
	bodyPara := shape.TextBody.Paragraphs[0]
	if bodyPara.Properties == nil || !strings.Contains(bodyPara.Properties.Inner, "buNone") {
		t.Error("body paragraph should have buNone to suppress bullet marker")
	}
	if len(bodyPara.Runs) == 0 || bodyPara.Runs[0].Text != "Our Q4 performance exceeded expectations with strong growth." {
		t.Errorf("body text mismatch, got runs=%v", bodyPara.Runs)
	}

	// Second paragraph (header) should have buNone and spcBef
	headerPara := shape.TextBody.Paragraphs[1]
	if headerPara.Properties == nil || !strings.Contains(headerPara.Properties.Inner, "buNone") {
		t.Error("header paragraph should have buNone to suppress bullet marker")
	}

	// Third and fourth paragraphs (bullets) should have paragraph properties for bullet level
	for i := 2; i <= 3; i++ {
		p := shape.TextBody.Paragraphs[i]
		if p.Properties == nil {
			t.Errorf("bullet paragraph %d should have paragraph properties for bullet level", i)
		}
	}
}

// TestPopulateShapeText_BulletGroups_BodyBeforeHeaderlessGroup tests that when the
// first group has no header, the body renders before the group (intro paragraph).
func TestPopulateShapeText_BulletGroups_BodyBeforeHeaderlessGroup(t *testing.T) {
	shape := &shapeXML{
		TextBody: nil,
	}

	item := ContentItem{
		PlaceholderID: "content",
		Type:          ContentBulletGroups,
		Value: BulletGroupsContent{
			Body: "Here is our summary:",
			Groups: []BulletGroup{
				{
					Header:  "", // No header on first group
					Bullets: []string{"Item A", "Item B"},
				},
			},
		},
	}

	err := populateShapeText(shape, item, -1, "")
	if err != nil {
		t.Fatalf("populateShapeText() error = %v", err)
	}

	if shape.TextBody == nil {
		t.Fatal("TextBody should be initialized")
	}

	// Should have 3 paragraphs: 1 body (intro) + 2 bullets
	if len(shape.TextBody.Paragraphs) != 3 {
		t.Fatalf("expected 3 paragraphs (body + 2 bullets), got %d", len(shape.TextBody.Paragraphs))
	}

	// First paragraph should be the body (before bullets, since no header on first group)
	bodyPara := shape.TextBody.Paragraphs[0]
	if bodyPara.Properties == nil || !strings.Contains(bodyPara.Properties.Inner, "buNone") {
		t.Error("body paragraph should have buNone")
	}
	if len(bodyPara.Runs) == 0 || bodyPara.Runs[0].Text != "Here is our summary:" {
		t.Errorf("body text mismatch, got runs=%v", bodyPara.Runs)
	}
}

// TestPopulateShapeText_BulletGroups_EmptyBody tests bullet groups without body text.
func TestPopulateShapeText_BulletGroups_EmptyBody(t *testing.T) {
	shape := &shapeXML{
		TextBody: nil,
	}

	item := ContentItem{
		PlaceholderID: "content",
		Type:          ContentBulletGroups,
		Value: BulletGroupsContent{
			Body: "", // Empty body - should NOT add a body paragraph
			Groups: []BulletGroup{
				{
					Header:  "**Section:**",
					Bullets: []string{"Point 1", "Point 2"},
				},
			},
		},
	}

	err := populateShapeText(shape, item, -1, "")
	if err != nil {
		t.Fatalf("populateShapeText() error = %v", err)
	}

	// Should have 3 paragraphs: 1 header + 2 bullets (no empty body paragraph)
	if len(shape.TextBody.Paragraphs) != 3 {
		t.Fatalf("expected 3 paragraphs (header + 2 bullets, no empty body), got %d", len(shape.TextBody.Paragraphs))
	}
}

// TestLoadOrCreateRelationships tests loading relationships from template
func TestLoadOrCreateRelationships(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)

	tests := []struct {
		name         string
		relsFileName string
		wantEmpty    bool
	}{
		{
			name:         "existing relationships file",
			relsFileName: "ppt/slides/_rels/slide1.xml.rels",
			wantEmpty:    false,
		},
		{
			name:         "non-existent relationships file",
			relsFileName: "ppt/slides/_rels/slide999.xml.rels",
			wantEmpty:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rels := ctx.loadOrCreateRelationships(tt.relsFileName)

			if tt.wantEmpty && len(rels.Relationships) > 0 {
				t.Errorf("expected empty relationships, got %d", len(rels.Relationships))
			}
			if !tt.wantEmpty && len(rels.Relationships) == 0 {
				t.Error("expected non-empty relationships")
			}
		})
	}
}

// TestConvertSVGToRaster_UnavailableConverter tests SVG conversion when converter is unavailable
func TestConvertSVGToRaster_UnavailableConverter(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	// Create a converter that reports as unavailable
	ctx.svgConverter = &SVGConverter{
		Strategy: SVGStrategyPNG,
		Scale:    2.0,
	}

	// Set up placeholder bounds for the fallback image (4x3 inches in EMUs)
	placeholderBounds := types.BoundingBox{
		X:      914400,
		Y:      914400,
		Width:  914400 * 4,
		Height: 914400 * 3,
	}
	slideNum := 1
	shapeIdx := 0

	// Mock unavailability by not having rsvg-convert installed
	// The actual test depends on system state, so we check the warning behavior
	convertedPath, ok := ctx.convertSVGToRaster("/path/to/test.svg", placeholderBounds, slideNum, shapeIdx)

	// If converter is available, we can't really test this
	// But we can verify the function returns expected values
	if ok && convertedPath == "" {
		t.Error("if conversion succeeded, path should not be empty")
	}
	if !ok && convertedPath != "" {
		t.Error("if conversion failed, path should be empty")
	}

	// If conversion failed, verify that fallback image was inserted
	if !ok {
		rels := ctx.slideRelUpdates[slideNum]
		if len(rels) == 0 {
			t.Error("expected fallback image to be inserted when conversion fails")
		} else {
			// Verify the fallback image has valid data
			if len(rels[0].data) == 0 {
				t.Error("expected fallback image data to be non-empty")
			}
		}
	}
}

// TestWriteRelationships_BothMediaAndNativeSVG tests writing relationships with both regular media and native SVG
func TestWriteRelationships_BothMediaAndNativeSVG(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)
	ctx.outputWriter = zip.NewWriter(tmpFile)

	// Set up slideContentMap since writeRelationships iterates over it
	ctx.slideContentMap[3] = SlideSpec{LayoutID: "slideLayout2"}

	// Add both regular media and native SVG for the same slide
	ctx.slideRelUpdates[3] = []mediaRel{
		{imagePath: "/path/image.png", mediaFileName: "image1.png"},
	}
	ctx.nativeSVGInserts[3] = []nativeSVGInsert{
		{pngRelID: "rId10", svgRelID: "rId11", pngMediaFile: "image2.png", svgMediaFile: "image3.svg"},
	}

	err = ctx.writeRelationships()
	if err != nil {
		t.Fatalf("writeRelationships() error = %v", err)
	}

	// Verify relationships file was created
	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	found := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "slide3.xml.rels") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected slide3.xml.rels to be written")
	}
}

// TestWriteNativeSVGOnlyRelationships tests writing relationships for slides with only native SVG (no regular media)
func TestWriteNativeSVGOnlyRelationships(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)
	ctx.outputWriter = zip.NewWriter(tmpFile)

	// Add only native SVG for slide 5 (no regular media)
	ctx.nativeSVGInserts[5] = []nativeSVGInsert{
		{pngRelID: "rId5", svgRelID: "rId6", pngMediaFile: "image1.png", svgMediaFile: "image2.svg"},
	}

	err = ctx.writeNativeSVGOnlyRelationships(5, ctx.nativeSVGInserts[5])
	if err != nil {
		t.Fatalf("writeNativeSVGOnlyRelationships() error = %v", err)
	}

	// Verify relationships file was created
	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	found := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "slide5.xml.rels") {
			found = true
			// Verify content
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open rels file: %v", err)
			}
			content, _ := readAll(rc)
			_ = rc.Close()

			contentStr := string(content)
			if !strings.Contains(contentStr, "rId5") || !strings.Contains(contentStr, "rId6") {
				t.Error("expected SVG relationship IDs in content")
			}
			break
		}
	}

	if !found {
		t.Error("expected slide5.xml.rels to be written")
	}
}

// TestWriteNativeSVGMedia tests writing native SVG media files
func TestWriteNativeSVGMedia(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test SVG and PNG files
	svgPath := filepath.Join(tmpDir, "test.svg")
	pngPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(svgPath, []byte("<svg></svg>"), 0644); err != nil {
		t.Fatalf("failed to create SVG: %v", err)
	}
	if err := os.WriteFile(pngPath, []byte{0x89, 0x50, 0x4E, 0x47}, 0644); err != nil {
		t.Fatalf("failed to create PNG: %v", err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.outputWriter = zip.NewWriter(tmpFile)

	ctx.nativeSVGInserts[1] = []nativeSVGInsert{
		{
			svgPath:      svgPath,
			pngPath:      pngPath,
			svgMediaFile: "image1.svg",
			pngMediaFile: "image2.png",
		},
	}

	ctx.writeNativeSVGMedia()

	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	foundSVG := false
	foundPNG := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "image1.svg") {
			foundSVG = true
		}
		if strings.Contains(f.Name, "image2.png") {
			foundPNG = true
		}
	}

	if !foundSVG {
		t.Error("expected SVG file to be written")
	}
	if !foundPNG {
		t.Error("expected PNG file to be written")
	}
}

// TestWriteNativeSVGMedia_MissingFiles tests warning when SVG/PNG files don't exist
func TestWriteNativeSVGMedia_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	tmpFile, err := os.CreateTemp(tmpDir, "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.outputWriter = zip.NewWriter(tmpFile)

	ctx.nativeSVGInserts[1] = []nativeSVGInsert{
		{
			svgPath:      "/nonexistent/test.svg",
			pngPath:      "/nonexistent/test.png",
			svgMediaFile: "image1.svg",
			pngMediaFile: "image2.png",
		},
	}

	ctx.writeNativeSVGMedia()

	// Should have warnings for missing files
	if len(ctx.warnings) == 0 {
		t.Error("expected warnings for missing SVG/PNG files")
	}
}

// TestWriteSingleSlide_WithNativeSVG tests writing a slide that has native SVG inserts
func TestWriteSingleSlide_WithNativeSVG(t *testing.T) {
	tmpDir := t.TempDir()

	tmpFile, err := os.CreateTemp(tmpDir, "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.outputWriter = zip.NewWriter(tmpFile)

	// Create a minimal slide with shapes
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			Name: "Test Slide",
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{ID: 1, Name: "Title"},
						},
					},
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{ID: 2, Name: "Image"},
						},
					},
				},
			},
		},
	}

	// Add native SVG that replaces the second shape
	ctx.nativeSVGInserts[3] = []nativeSVGInsert{
		{
			pngRelID:       "rId5",
			svgRelID:       "rId6",
			placeholderIdx: 1, // Remove second shape
			offsetX:        100,
			offsetY:        200,
			extentCX:       300,
			extentCY:       400,
		},
	}

	err = ctx.writeSingleSlide(3, slide)
	if err != nil {
		t.Fatalf("writeSingleSlide() error = %v", err)
	}

	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	found := false
	for _, f := range r.File {
		if f.Name == "ppt/slides/slide3.xml" {
			found = true
			rc, _ := f.Open()
			content, _ := readAll(rc)
			_ = rc.Close()

			contentStr := string(content)
			// Should contain p:pic element for native SVG
			if !strings.Contains(contentStr, "p:pic") {
				t.Error("expected p:pic element for native SVG")
			}
			// Should have removed the placeholder shape
			if strings.Contains(contentStr, "Image") && strings.Contains(contentStr, `Name="Image"`) {
				t.Error("expected Image placeholder to be removed")
			}
			break
		}
	}

	if !found {
		t.Error("expected slide3.xml to be written")
	}
}

// TestWriteSlides_NoModifiedPresentation tests writeSlides when presentation.xml is not modified
func TestWriteSlides_NoModifiedPresentation(t *testing.T) {
	tmpDir := t.TempDir()

	tmpFile, err := os.CreateTemp(tmpDir, "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.outputWriter = zip.NewWriter(tmpFile)

	// No modified files, no template slides
	err = ctx.writeSlides()
	if err != nil {
		t.Fatalf("writeSlides() error = %v", err)
	}

	// Should complete without error even with no slides to write
	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
}

// TestFinalizePPTX_Success tests successful PPTX finalization
func TestFinalizePPTX_Success(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.pptx")
	tmpPath := outputPath + ".tmp"

	// Create the tmp file for writing
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	ctx := newSinglePassContext(outputPath, nil, nil, false, nil)
	ctx.tmpPath = tmpPath
	ctx.outputFile = tmpFile
	ctx.outputWriter = zip.NewWriter(tmpFile)

	// Write something to make it a valid ZIP
	fw, _ := ctx.outputWriter.Create("test.txt")
	_, _ = fw.Write([]byte("test content"))

	// Now test finalizePPTX - it should close the writer and file
	result, err := ctx.finalizePPTX(3)
	if err != nil {
		t.Fatalf("finalizePPTX() error = %v", err)
	}

	if result.SlideCount != 3 {
		t.Errorf("SlideCount = %d, want 3", result.SlideCount)
	}
	if result.OutputPath != outputPath {
		t.Errorf("OutputPath = %q, want %q", result.OutputPath, outputPath)
	}
	if result.FileSize <= 0 {
		t.Error("FileSize should be > 0")
	}

	// Verify the output file was renamed correctly
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("expected output file to exist")
	}
}

// TestWriteContentTypes_Error tests writeContentTypes error handling
func TestWriteContentTypes_Error(t *testing.T) {
	templatePath := "../template/testdata/standard.pptx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("test template not found")
	}

	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatalf("failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "test-*.zip")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()

	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)
	ctx.outputWriter = zip.NewWriter(tmpFile)
	ctx.usedExtensions = map[string]bool{"png": true, "jpg": true}

	err = ctx.writeContentTypes()
	if err != nil {
		t.Fatalf("writeContentTypes() error = %v", err)
	}

	// Verify content types file was written
	if err := ctx.outputWriter.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	_ = tmpFile.Close()
	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	found := false
	for _, f := range r.File {
		if f.Name == "[Content_Types].xml" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected [Content_Types].xml to be written")
	}
}

// TestFindMaxShapeID tests the regex-based shape ID extraction
func TestFindMaxShapeID(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected uint32
	}{
		{
			name:     "empty input",
			data:     "",
			expected: 0,
		},
		{
			name:     "no IDs",
			data:     `<p:sp><p:nvSpPr></p:nvSpPr></p:sp>`,
			expected: 0,
		},
		{
			name:     "single ID",
			data:     `<p:cNvPr id="5" name="Shape"/>`,
			expected: 5,
		},
		{
			name:     "multiple IDs ascending",
			data:     `<p:cNvPr id="1" name="A"/><p:cNvPr id="2" name="B"/><p:cNvPr id="3" name="C"/>`,
			expected: 3,
		},
		{
			name:     "multiple IDs descending",
			data:     `<p:cNvPr id="10" name="A"/><p:cNvPr id="5" name="B"/><p:cNvPr id="2" name="C"/>`,
			expected: 10,
		},
		{
			name:     "mixed IDs with max in middle",
			data:     `<p:cNvPr id="5" name="A"/><p:cNvPr id="100" name="B"/><p:cNvPr id="3" name="C"/>`,
			expected: 100,
		},
		{
			name: "realistic slide XML",
			data: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
      </p:nvGrpSpPr>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="42" name="Title"/>
        </p:nvSpPr>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="17" name="Content"/>
        </p:nvSpPr>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`,
			expected: 42,
		},
		{
			name:     "large ID value",
			data:     `<p:cNvPr id="999999" name="Large"/>`,
			expected: 999999,
		},
		{
			name:     "ID not on word boundary ignored",
			data:     `<p:cNvPr xid="5" name="Shape"/>`,
			expected: 0, // xid="5" should not match due to \b word boundary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMaxShapeID([]byte(tt.data))
			if result != tt.expected {
				t.Errorf("findMaxShapeID() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// BenchmarkFindMaxShapeID benchmarks the regex-based shape ID extraction
func BenchmarkFindMaxShapeID(b *testing.B) {
	// Simulate a typical slide with ~20 shapes
	slideXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
      <p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="3" name="Content"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="4" name="Footer"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="5" name="Date"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="6" name="Slide Number"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="7" name="Bullet 1"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="8" name="Bullet 2"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="9" name="Bullet 3"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="10" name="Image 1"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="11" name="Image 2"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="12" name="Chart"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="13" name="Table"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="14" name="Shape"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="15" name="Textbox"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="16" name="Arrow"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="17" name="Callout"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="18" name="SmartArt"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="19" name="Video"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="20" name="Audio"/></p:nvSpPr></p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`

	data := []byte(slideXML)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = findMaxShapeID(data)
	}
}

// BenchmarkFindMaxShapeID_LargeSlide benchmarks with a larger slide
func BenchmarkFindMaxShapeID_LargeSlide(b *testing.B) {
	// Build a slide with 100+ shapes
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><p:sld><p:cSld><p:spTree>`)
	sb.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>`)
	for i := 2; i <= 150; i++ {
		sb.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Shape%d"/></p:nvSpPr></p:sp>`, i, i))
	}
	sb.WriteString(`</p:spTree></p:cSld></p:sld>`)

	data := []byte(sb.String())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = findMaxShapeID(data)
	}
}

// TestParseLayoutPictures tests extraction of p:pic elements from layout XML.
func TestParseLayoutPictures(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		wantPics int
		wantName string
		wantOffX int64
		wantExtW int64
	}{
		{
			name: "layout with logo picture",
			xml: `<?xml version="1.0"?>
<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:spTree>
      <p:pic>
        <p:nvPicPr><p:cNvPr id="14" name="Graphic 13"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>
        <p:blipFill><a:blip r:embed="rId3"/></p:blipFill>
        <p:spPr><a:xfrm><a:off x="403391" y="383381"/><a:ext cx="1064007" cy="517902"/></a:xfrm></p:spPr>
      </p:pic>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Title 1"/></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="401904" y="1963495"/><a:ext cx="5664555" cy="1828800"/></a:xfrm></p:spPr>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sldLayout>`,
			wantPics: 1,
			wantName: "Graphic 13",
			wantOffX: 403391,
			wantExtW: 1064007,
		},
		{
			name: "layout without pictures",
			xml: `<?xml version="1.0"?>
<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Title 1"/></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm></p:spPr>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sldLayout>`,
			wantPics: 0,
		},
		{
			name: "layout with background image and logo",
			xml: `<?xml version="1.0"?>
<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
             xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
             xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:spTree>
      <p:pic>
        <p:nvPicPr><p:cNvPr id="3" name="background"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>
        <p:blipFill><a:blip r:embed="rId2"/></p:blipFill>
        <p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="12188951" cy="6856285"/></a:xfrm></p:spPr>
      </p:pic>
      <p:pic>
        <p:nvPicPr><p:cNvPr id="14" name="Logo"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>
        <p:blipFill><a:blip r:embed="rId3"/></p:blipFill>
        <p:spPr><a:xfrm><a:off x="400000" y="400000"/><a:ext cx="1000000" cy="500000"/></a:xfrm></p:spPr>
      </p:pic>
    </p:spTree>
  </p:cSld>
</p:sldLayout>`,
			wantPics: 2,
			wantName: "background",
			wantOffX: 0,
			wantExtW: 12188951,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pics := parseLayoutPictures([]byte(tt.xml))
			if len(pics) != tt.wantPics {
				t.Errorf("parseLayoutPictures() returned %d pics, want %d", len(pics), tt.wantPics)
				return
			}
			if tt.wantPics > 0 {
				if pics[0].Name != tt.wantName {
					t.Errorf("pic[0].Name = %q, want %q", pics[0].Name, tt.wantName)
				}
				if pics[0].OffX != tt.wantOffX {
					t.Errorf("pic[0].OffX = %d, want %d", pics[0].OffX, tt.wantOffX)
				}
				if pics[0].ExtCX != tt.wantExtW {
					t.Errorf("pic[0].ExtCX = %d, want %d", pics[0].ExtCX, tt.wantExtW)
				}
			}
		})
	}
}

// TestDetectLogoZones_NonLogoTemplates tests that templates without logos return nil.
func TestDetectLogoZones_NonLogoTemplates(t *testing.T) {
	templates := []string{
		"forest-green.pptx",
		"midnight-blue.pptx",
		"warm-coral.pptx",
		"SimpleMinimal-Consulting.pptx",
		"template_2.pptx",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			templatePath := filepath.Join("..", "..", "testdata", "templates", tmpl)
			if _, err := os.Stat(templatePath); os.IsNotExist(err) {
				t.Skip("template not found, skipping")
			}

			reader, err := zip.OpenReader(templatePath)
			if err != nil {
				t.Fatalf("failed to open template: %v", err)
			}
			defer reader.Close()

			zones := detectLogoZones(&reader.Reader, utils.BuildZipIndex(&reader.Reader))
			if zones != nil {
				t.Errorf("detectLogoZones() returned non-nil for %s: %v",
					tmpl, zones)
			}
		})
	}
}

// TestAdjustShapesForLogoZone tests the shape adjustment logic.
func TestAdjustShapesForLogoZone(t *testing.T) {
	ctx := &singlePassContext{}
	zone := &LogoZone{
		Right:  1600000, // ~1.75 inches from left
		Bottom: 900000,  // ~1 inch from top
	}

	titleIdx := 0
	contentIdx := 1
	footerIdx := 3
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						// Title at X=400000, Y=400000 — inside logo zone
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
							NvPr:               nvPrXML{Placeholder: &placeholderXML{Type: "title", Index: &titleIdx}},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 400000, Y: 400000},
								Extent: extentXML{CX: 11400000, CY: 1300000},
							},
						},
					},
					{
						// Content at X=400000, Y=1900000 — below logo zone (Y > Bottom)
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Content 2"},
							NvPr:               nvPrXML{Placeholder: &placeholderXML{Type: "", Index: &contentIdx}},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 400000, Y: 1900000},
								Extent: extentXML{CX: 11400000, CY: 4000000},
							},
						},
					},
					{
						// Non-placeholder shape (no ph element) — should not be adjusted
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Decorative 3"},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 100000, Y: 100000},
								Extent: extentXML{CX: 500000, CY: 500000},
							},
						},
					},
					{
						// Footer at Y=6500000 — well below logo zone
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Footer 4"},
							NvPr:               nvPrXML{Placeholder: &placeholderXML{Type: "ftr", Index: &footerIdx}},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 100000, Y: 6500000},
								Extent: extentXML{CX: 9000000, CY: 200000},
							},
						},
					},
				},
			},
		},
	}

	ctx.adjustShapesForLogoZone(slide, zone)

	shapes := slide.CommonSlideData.ShapeTree.Shapes

	// Title should be shifted right
	titleShape := shapes[0]
	if titleShape.ShapeProperties.Transform.Offset.X != 1600000 {
		t.Errorf("title X = %d, want 1600000 (shifted to logo right edge)", titleShape.ShapeProperties.Transform.Offset.X)
	}
	expectedTitleW := int64(11400000 - (1600000 - 400000))
	if titleShape.ShapeProperties.Transform.Extent.CX != expectedTitleW {
		t.Errorf("title CX = %d, want %d (reduced by shift amount)", titleShape.ShapeProperties.Transform.Extent.CX, expectedTitleW)
	}

	// Content is below the logo zone (Y=1900000 > Bottom=900000), should NOT be shifted
	contentShape := shapes[1]
	if contentShape.ShapeProperties.Transform.Offset.X != 400000 {
		t.Errorf("content X = %d, want 400000 (should not be shifted, Y is below logo)", contentShape.ShapeProperties.Transform.Offset.X)
	}

	// Decorative shape without placeholder — should NOT be shifted
	decoShape := shapes[2]
	if decoShape.ShapeProperties.Transform.Offset.X != 100000 {
		t.Errorf("decorative X = %d, want 100000 (no placeholder, should not be shifted)", decoShape.ShapeProperties.Transform.Offset.X)
	}

	// Footer is well below logo zone — should NOT be shifted
	footerShape := shapes[3]
	if footerShape.ShapeProperties.Transform.Offset.X != 100000 {
		t.Errorf("footer X = %d, want 100000 (should not be shifted, Y is far below logo)", footerShape.ShapeProperties.Transform.Offset.X)
	}
}

// TestAdjustShapesForLogoZone_NilLogoZone tests that nil logo zone is a no-op.
func TestAdjustShapesForLogoZone_NilLogoZone(t *testing.T) {
	ctx := &singlePassContext{}

	titleIdx := 0
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
							NvPr:               nvPrXML{Placeholder: &placeholderXML{Type: "title", Index: &titleIdx}},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 400000, Y: 400000},
								Extent: extentXML{CX: 11400000, CY: 1300000},
							},
						},
					},
				},
			},
		},
	}

	// nil zone is a no-op
	ctx.adjustShapesForLogoZone(slide, nil)

	// Should be unchanged
	titleShape := slide.CommonSlideData.ShapeTree.Shapes[0]
	if titleShape.ShapeProperties.Transform.Offset.X != 400000 {
		t.Errorf("title X = %d, want 400000 (nil logo zone, no change expected)", titleShape.ShapeProperties.Transform.Offset.X)
	}
}

// TestPlaceholderResolver_IdxLookup tests idx:N syntax for direct OOXML index targeting.
func TestPlaceholderResolver_IdxLookup(t *testing.T) {
	idx1 := 1
	idx10 := 10
	idx11 := 11
	shapes := []shapeXML{
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: &idx1}},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body_2"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: &idx10}},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body_3"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: &idx11}},
			},
		},
	}

	resolver := newPlaceholderResolver(shapes)

	tests := []struct {
		id        string
		wantIdx   int
		wantFound bool
	}{
		{"idx:1", 0, true},
		{"idx:10", 1, true},
		{"idx:11", 2, true},
		{"idx:99", 0, false},
		{"idx:bad", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			gotIdx, found := resolver.Resolve(tt.id)
			if found != tt.wantFound {
				t.Errorf("Resolve(%q) found=%v, want %v", tt.id, found, tt.wantFound)
			}
			if found && gotIdx != tt.wantIdx {
				t.Errorf("Resolve(%q) = %d, want %d", tt.id, gotIdx, tt.wantIdx)
			}
		})
	}
}

// TestPlaceholderResolver_NameLookup tests standard name-based lookup.
func TestPlaceholderResolver_NameLookup(t *testing.T) {
	shapes := []shapeXML{
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "title"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "title"}},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
			},
		},
	}

	resolver := newPlaceholderResolver(shapes)

	if idx, ok := resolver.Resolve("title"); !ok || idx != 0 {
		t.Errorf("Resolve(title) = %d, %v; want 0, true", idx, ok)
	}
	if idx, ok := resolver.Resolve("body"); !ok || idx != 1 {
		t.Errorf("Resolve(body) = %d, %v; want 1, true", idx, ok)
	}
	if _, ok := resolver.Resolve("nonexistent"); ok {
		t.Error("Resolve(nonexistent) should return false")
	}
}

// TestPlaceholderResolver_DuplicateNames tests positional assignment when
// multiple shapes share the same name (pre-normalization edge case).
func TestPlaceholderResolver_DuplicateNames(t *testing.T) {
	shapes := []shapeXML{
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Content Placeholder 2"},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Content Placeholder 2"},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Content Placeholder 2"},
			},
		},
	}

	resolver := newPlaceholderResolver(shapes)

	// Should warn about ambiguous name
	if len(resolver.warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(resolver.warnings), resolver.warnings)
	}
	if !strings.Contains(resolver.warnings[0], "ambiguous") {
		t.Errorf("warning should mention 'ambiguous', got: %s", resolver.warnings[0])
	}

	// Successive Resolve calls should return shapes in positional order
	idx0, ok0 := resolver.Resolve("Content Placeholder 2")
	idx1, ok1 := resolver.Resolve("Content Placeholder 2")
	idx2, ok2 := resolver.Resolve("Content Placeholder 2")

	if !ok0 || !ok1 || !ok2 {
		t.Fatal("all three Resolve calls should succeed")
	}
	if idx0 != 0 {
		t.Errorf("first Resolve = %d, want 0 (leftmost)", idx0)
	}
	if idx1 != 1 {
		t.Errorf("second Resolve = %d, want 1 (middle)", idx1)
	}
	if idx2 != 2 {
		t.Errorf("third Resolve = %d, want 2 (rightmost)", idx2)
	}

	// Fourth call should return last (all consumed)
	idx3, ok3 := resolver.Resolve("Content Placeholder 2")
	if !ok3 || idx3 != 2 {
		t.Errorf("fourth Resolve = %d, %v; want 2, true (last repeated)", idx3, ok3)
	}
}

// TestPlaceholderResolver_ResolveIDToName tests idx:N to canonical name translation.
func TestPlaceholderResolver_ResolveIDToName(t *testing.T) {
	idx1 := 1
	shapes := []shapeXML{
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body", Index: &idx1}},
			},
		},
	}

	resolver := newPlaceholderResolver(shapes)

	// idx:1 should resolve to "body"
	if name := resolver.ResolveIDToName("idx:1"); name != "body" {
		t.Errorf("ResolveIDToName(idx:1) = %q, want %q", name, "body")
	}

	// Regular name passes through unchanged
	if name := resolver.ResolveIDToName("body"); name != "body" {
		t.Errorf("ResolveIDToName(body) = %q, want %q", name, "body")
	}

	// Unknown idx returns the original ID
	if name := resolver.ResolveIDToName("idx:99"); name != "idx:99" {
		t.Errorf("ResolveIDToName(idx:99) = %q, want %q", name, "idx:99")
	}
}

// TestPlaceholderResolver_Keys tests that Keys returns all registered names.
func TestPlaceholderResolver_Keys(t *testing.T) {
	shapes := []shapeXML{
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "title"}}},
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "body"}}},
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "image"}}},
	}

	resolver := newPlaceholderResolver(shapes)
	keys := resolver.Keys()

	if len(keys) != 3 {
		t.Fatalf("Keys() returned %d keys, want 3", len(keys))
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, want := range []string{"title", "body", "image"} {
		if !keySet[want] {
			t.Errorf("Keys() missing %q", want)
		}
	}
}

// TestPlaceholderResolver_NoDuplicateWarnings verifies no warnings when names are unique.
func TestPlaceholderResolver_NoDuplicateWarnings(t *testing.T) {
	shapes := []shapeXML{
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "body"}}},
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "body_2"}}},
		{NonVisualProperties: nonVisualPropertiesXML{ConnectionNonVisual: connectionNonVisualXML{Name: "body_3"}}},
	}

	resolver := newPlaceholderResolver(shapes)
	if len(resolver.warnings) != 0 {
		t.Errorf("expected no warnings for unique names, got: %v", resolver.warnings)
	}
}

// TestClearUnmappedPlaceholders_HasCustomPrompt tests that all unmapped
// placeholders are cleared, including title placeholders regardless of
// hasCustomPrompt, and that hasCustomPrompt is stripped from cleared shapes.
func TestClearUnmappedPlaceholders_HasCustomPrompt(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)

	bodyIdx := 12
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						// Title with hasCustomPrompt — should be cleared when unmapped
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "title"},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{
									Type:            "title",
									HasCustomPrompt: "1",
								},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "Title for accessibility"}}},
							},
						},
					},
					{
						// Body with hasCustomPrompt — should be cleared when unmapped
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "body_2"},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{
									Type:            "body",
									Index:           &bodyIdx,
									HasCustomPrompt: "1",
								},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "0"}}},
							},
						},
					},
					{
						// Regular body — should be cleared when unmapped
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "body"},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "Click to add text"}}},
							},
						},
					},
					{
						// Title WITHOUT hasCustomPrompt — should still be cleared
						// when unmapped (prevents "Click to add title" prompt leak)
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "cleared_title"},
							NvPr: nvPrXML{
								Placeholder: &placeholderXML{Type: "title"},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "Click to edit Master title style"}}},
							},
						},
					},
				},
			},
		},
	}

	ctx.templateSlideData = map[int]*slideXML{1: slide}
	// Only "body" is targeted — title, body_2, preserved_title are unmapped
	ctx.slideContentMap = map[int]SlideSpec{
		1: {Content: []ContentItem{{PlaceholderID: "body", Type: ContentText, Value: "populated"}}},
	}

	ctx.clearUnmappedPlaceholders()

	shapes := slide.CommonSlideData.ShapeTree.Shapes

	// title (hasCustomPrompt) — should be cleared and hasCustomPrompt stripped
	if len(shapes[0].TextBody.Paragraphs) != 1 || len(shapes[0].TextBody.Paragraphs[0].Runs) != 0 {
		t.Errorf("title with hasCustomPrompt should be cleared, got %d paragraphs with runs",
			len(shapes[0].TextBody.Paragraphs))
	}
	if shapes[0].TextBody.Paragraphs[0].EndParaRPr == nil {
		t.Error("cleared title paragraph missing required endParaRPr")
	}
	if shapes[0].NonVisualProperties.NvPr.Placeholder.HasCustomPrompt != "" {
		t.Errorf("hasCustomPrompt should be stripped from cleared title, got %q",
			shapes[0].NonVisualProperties.NvPr.Placeholder.HasCustomPrompt)
	}

	// body_2 (hasCustomPrompt, "0" section number) — should be cleared and hasCustomPrompt stripped
	if len(shapes[1].TextBody.Paragraphs) != 1 || len(shapes[1].TextBody.Paragraphs[0].Runs) != 0 {
		t.Errorf("body_2 with hasCustomPrompt '0' should be cleared, got %d paragraphs",
			len(shapes[1].TextBody.Paragraphs))
	}
	if shapes[1].TextBody.Paragraphs[0].EndParaRPr == nil {
		t.Error("cleared body_2 paragraph missing required endParaRPr")
	}
	if shapes[1].NonVisualProperties.NvPr.Placeholder.HasCustomPrompt != "" {
		t.Errorf("hasCustomPrompt should be stripped from cleared body_2, got %q",
			shapes[1].NonVisualProperties.NvPr.Placeholder.HasCustomPrompt)
	}

	// body — targeted, should NOT be cleared (still has original text)
	if len(shapes[2].TextBody.Paragraphs) != 1 || shapes[2].TextBody.Paragraphs[0].Runs[0].Text != "Click to add text" {
		t.Errorf("body placeholder was targeted and should not be cleared")
	}

	// cleared_title (no hasCustomPrompt) — should be cleared when unmapped
	// This prevents "Click to add title" prompt text from leaking into rendered output.
	if len(shapes[3].TextBody.Paragraphs) != 1 || len(shapes[3].TextBody.Paragraphs[0].Runs) != 0 {
		t.Errorf("title without hasCustomPrompt should be cleared when unmapped, got %d paragraphs with %d runs",
			len(shapes[3].TextBody.Paragraphs), len(shapes[3].TextBody.Paragraphs[0].Runs))
	}
	if shapes[3].TextBody.Paragraphs[0].EndParaRPr == nil {
		t.Error("cleared title paragraph missing required endParaRPr")
	}
}

// TestEmptyParagraphEndParaRPr verifies that emptyParagraph() produces
// spec-compliant XML with <a:endParaRPr lang="en-US"/>.
func TestEmptyParagraphEndParaRPr(t *testing.T) {
	p := emptyParagraph()
	if p.EndParaRPr == nil {
		t.Fatal("emptyParagraph().EndParaRPr must not be nil")
	}
	if p.EndParaRPr.Lang != "en-US" {
		t.Errorf("emptyParagraph().EndParaRPr.Lang = %q, want %q", p.EndParaRPr.Lang, "en-US")
	}
	if len(p.Runs) != 0 {
		t.Errorf("emptyParagraph() should have no runs, got %d", len(p.Runs))
	}

	// Verify XML marshaling contains <a:endParaRPr
	data, err := xml.Marshal(p)
	if err != nil {
		t.Fatalf("xml.Marshal(emptyParagraph()) error: %v", err)
	}
	xmlStr := string(data)
	if !strings.Contains(xmlStr, "endParaRPr") {
		t.Errorf("marshaled XML missing endParaRPr: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `lang="en-US"`) {
		t.Errorf("marshaled XML missing lang attribute: %s", xmlStr)
	}
}

// TestSetTextParagraph_EmptyTextHasEndParaRPr verifies that empty text input
// produces a paragraph with endParaRPr (OOXML compliance).
func TestSetTextParagraph_EmptyTextHasEndParaRPr(t *testing.T) {
	shape := &shapeXML{
		TextBody: &textBodyXML{
			BodyProperties: &bodyPropertiesXML{},
			ListStyle:      &listStyleXML{},
			Paragraphs: []paragraphXML{
				{Runs: []runXML{{Text: "Template sample text"}}},
			},
		},
	}

	err := setTextParagraph(shape, "body", "", 0, "")
	if err != nil {
		t.Fatalf("setTextParagraph error: %v", err)
	}
	if len(shape.TextBody.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(shape.TextBody.Paragraphs))
	}
	if shape.TextBody.Paragraphs[0].EndParaRPr == nil {
		t.Error("empty text paragraph missing required endParaRPr")
	}
}

// TestSyntheticLayoutNames verifies that syntheticLayoutNames() collects and
// sorts synthetic layout XML filenames correctly, excluding .rels files.
func TestSyntheticLayoutNames(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, map[string][]byte{
		"ppt/slideLayouts/slideLayout99.xml":              []byte("<xml/>"),
		"ppt/slideLayouts/_rels/slideLayout99.xml.rels":   []byte("<xml/>"),
		"ppt/slideLayouts/slideLayout100.xml":             []byte("<xml/>"),
		"ppt/slideLayouts/_rels/slideLayout100.xml.rels":  []byte("<xml/>"),
	})

	names := ctx.syntheticLayoutNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 layout names, got %d: %v", len(names), names)
	}
	if names[0] != "slideLayout100.xml" || names[1] != "slideLayout99.xml" {
		t.Errorf("unexpected layout names: %v", names)
	}
}

// TestWriteSlideMasterXMLWithSynthetic verifies that synthetic layouts are
// registered in <p:sldLayoutIdLst> with unique IDs and correct r:id values.
func TestWriteSlideMasterXMLWithSynthetic(t *testing.T) {
	// Create a temporary ZIP with a slide master XML and its .rels
	masterXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree/></p:cSld>
<p:sldLayoutIdLst>
<p:sldLayoutId id="2147483649" r:id="rId1"/>
<p:sldLayoutId id="2147483650" r:id="rId2"/>
</p:sldLayoutIdLst>
</p:sldMaster>`

	relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout2.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`

	// Write a temporary ZIP with these files
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "template.pptx")
	outputPath := filepath.Join(tmpDir, "output.pptx")

	func() {
		f, err := os.Create(templatePath)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		zw := zip.NewWriter(f)
		defer zw.Close()

		for name, content := range map[string]string{
			"ppt/slideMasters/slideMaster1.xml":                  masterXML,
			"ppt/slideMasters/_rels/slideMaster1.xml.rels":       relsXML,
		} {
			w, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
	}()

	// Open the template ZIP
	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	ctx := newSinglePassContext(outputPath, nil, nil, false, map[string][]byte{
		"ppt/slideLayouts/slideLayout99.xml":             []byte("<xml/>"),
		"ppt/slideLayouts/_rels/slideLayout99.xml.rels":  []byte("<xml/>"),
	})
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)

	// Create output ZIP
	outFile, err := os.Create(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer outFile.Close()
	ctx.outputWriter = zip.NewWriter(outFile)

	// Find the master XML file entry
	var masterEntry *zip.File
	for _, f := range reader.File {
		if f.Name == "ppt/slideMasters/slideMaster1.xml" {
			masterEntry = f
			break
		}
	}
	if masterEntry == nil {
		t.Fatal("master XML not found in template ZIP")
	}

	// Write the modified master XML
	if err := ctx.writeSlideMasterXMLWithSynthetic(masterEntry); err != nil {
		t.Fatalf("writeSlideMasterXMLWithSynthetic error: %v", err)
	}

	ctx.outputWriter.Close()

	// Read back the output ZIP and verify
	outReader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer outReader.Close()

	for _, f := range outReader.File {
		if f.Name == "ppt/slideMasters/slideMaster1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}

			content := string(data)

			// Verify synthetic layout is registered
			// rId3 is the max existing rId, so synthetic gets rId4
			if !strings.Contains(content, `r:id="rId4"`) {
				t.Error("synthetic layout rId not found in master XML")
			}

			// Verify layout ID is in the correct range (> 2147483650)
			if !strings.Contains(content, `id="2147483651"`) {
				t.Error("synthetic layout ID not found or not sequential")
			}

			// Verify original entries preserved
			if !strings.Contains(content, `id="2147483649"`) {
				t.Error("original layout ID 2147483649 missing")
			}
			if !strings.Contains(content, `id="2147483650"`) {
				t.Error("original layout ID 2147483650 missing")
			}

			return
		}
	}
	t.Error("master XML not found in output ZIP")
}

// TestReplaceOrInsertXMLElement verifies app.xml element replacement/insertion.
func TestReplaceOrInsertXMLElement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tag      string
		value    string
		contains string
	}{
		{
			name:     "replace existing element",
			input:    `<Properties><Slides>4</Slides></Properties>`,
			tag:      "Slides",
			value:    "31",
			contains: "<Slides>31</Slides>",
		},
		{
			name:     "insert missing element",
			input:    `<Properties><Application>PowerPoint</Application></Properties>`,
			tag:      "Slides",
			value:    "10",
			contains: "<Slides>10</Slides>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceOrInsertXMLElement(tt.input, tt.tag, tt.value)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %q, got: %s", tt.contains, result)
			}
		})
	}
}

// TestCountWords verifies whitespace-delimited word counting.
func TestCountWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  spaced  out  ", 2},
		{"Revenue exceeded all quarterly targets", 5},
	}
	for _, tt := range tests {
		if got := countWords(tt.input); got != tt.want {
			t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// TestUpdateAppProperties verifies that slide count and word count are updated in app.xml.
func TestUpdateAppProperties(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)

	// Simulate template app.xml stored in a ZIP
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "template.pptx")

	appXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
<Application>Microsoft Office PowerPoint</Application>
<AppVersion>16.0000</AppVersion>
<Slides>4</Slides>
<Words>13</Words>
<Paragraphs>5</Paragraphs>
<TotalTime>120</TotalTime>
<Company>Acme Corp</Company>
</Properties>`

	func() {
		f, _ := os.Create(templatePath)
		defer f.Close()
		zw := zip.NewWriter(f)
		defer zw.Close()
		w, _ := zw.Create("docProps/app.xml")
		w.Write([]byte(appXML))
	}()

	reader, _ := zip.OpenReader(templatePath)
	defer reader.Close()
	ctx.templateReader = reader
	ctx.templateIndex = utils.BuildZipIndex(&reader.Reader)

	// Add slide specs with text content
	ctx.slideSpecs = []SlideSpec{
		{Content: []ContentItem{
			{Type: ContentText, PlaceholderID: "title", Value: "Revenue Growth Overview"},
			{Type: ContentBullets, PlaceholderID: "body", Value: []string{
				"Revenue exceeded targets",
				"Customer acquisition improved",
			}},
		}},
		{Content: []ContentItem{
			{Type: ContentText, PlaceholderID: "title", Value: "Next Steps"},
		}},
	}

	ctx.updateAppProperties(2)

	data, ok := ctx.modifiedFiles[PathDocPropsApp]
	if !ok {
		t.Fatal("expected modifiedFiles to contain docProps/app.xml")
	}

	content := string(data)

	// Slide count should be 2
	if !strings.Contains(content, "<Slides>2</Slides>") {
		t.Errorf("expected <Slides>2</Slides>, got: %s", content)
	}

	// Words: "Revenue Growth Overview" (3) + "Revenue exceeded targets" (3)
	//        + "Customer acquisition improved" (3) + "Next Steps" (2) = 11
	if !strings.Contains(content, "<Words>11</Words>") {
		t.Errorf("expected <Words>11</Words>, got: %s", content)
	}

	// Paragraphs: 2 text items + 2 bullets = 4
	if !strings.Contains(content, "<Paragraphs>4</Paragraphs>") {
		t.Errorf("expected <Paragraphs>4</Paragraphs>, got: %s", content)
	}

	// Company should be preserved
	if !strings.Contains(content, "<Company>Acme Corp</Company>") {
		t.Error("template Company property was not preserved")
	}

	// TotalTime should be reset to 0
	if !strings.Contains(content, "<TotalTime>0</TotalTime>") {
		t.Errorf("expected <TotalTime>0</TotalTime>, got: %s", content)
	}
}
