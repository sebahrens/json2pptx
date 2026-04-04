package generator

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestPanelBulletsToOOXML_Empty(t *testing.T) {
	result := panelBulletsToOOXML("", panelBodyFontSize)
	if result != "" {
		t.Errorf("empty input should return empty string, got %q", result)
	}
}

func TestPanelBulletsToOOXML_BulletLines(t *testing.T) {
	result := panelBulletsToOOXML("- First bullet\n- Second bullet", panelBodyFontSize)

	// Should contain bullet character reference
	if !strings.Contains(result, `buChar char=`) {
		t.Error("bulleted lines should contain bullet character")
	}

	// Should contain both bullet texts
	if !strings.Contains(result, "First bullet") {
		t.Error("should contain first bullet text")
	}
	if !strings.Contains(result, "Second bullet") {
		t.Error("should contain second bullet text")
	}

	// Should use scheme color for bullets
	if !strings.Contains(result, `schemeClr val="accent1"`) {
		t.Error("bullet color should use schemeClr accent1")
	}

	// Should NOT contain srgbClr (no hardcoded hex colors)
	if strings.Contains(result, "srgbClr") {
		t.Error("bullet XML should not contain srgbClr (hardcoded hex)")
	}

	// Should use correct font size
	if !strings.Contains(result, fmt.Sprintf(`sz="%d"`, panelBodyFontSize)) {
		t.Errorf("should use font size %d", panelBodyFontSize)
	}

	// Should have margin and indent
	if !strings.Contains(result, fmt.Sprintf(`marL="%d"`, pptx.BulletMarginLeft)) {
		t.Error("bullet paragraphs should have marL attribute")
	}
	if !strings.Contains(result, fmt.Sprintf(`indent="%d"`, pptx.BulletIndent)) {
		t.Error("bullet paragraphs should have indent attribute")
	}
}

func TestPanelBulletsToOOXML_PlainLines(t *testing.T) {
	result := panelBulletsToOOXML("Plain text line", panelBodyFontSize)

	// Should NOT contain bullet elements
	if strings.Contains(result, "buChar") {
		t.Error("plain lines should not contain buChar")
	}

	// Should contain the text
	if !strings.Contains(result, "Plain text line") {
		t.Error("should contain the plain text")
	}
}

func TestPanelBulletsToOOXML_MixedLines(t *testing.T) {
	result := panelBulletsToOOXML("Header text\n- Bullet one\n- Bullet two\nFooter text", panelBodyFontSize)

	// Count bullet chars — should be exactly 2
	count := strings.Count(result, `buChar char=`)
	if count != 2 {
		t.Errorf("expected 2 bullet paragraphs, got %d", count)
	}

	// Count total paragraphs
	pCount := strings.Count(result, "<a:p>")
	if pCount != 4 {
		t.Errorf("expected 4 paragraphs, got %d", pCount)
	}
}

func TestPanelBulletsToOOXML_XMLEscaping(t *testing.T) {
	result := panelBulletsToOOXML("- Smith & Jones <2025>", panelBodyFontSize)

	if !strings.Contains(result, "Smith &amp; Jones &lt;2025&gt;") {
		t.Error("special XML characters should be escaped")
	}
}

func TestGeneratePanelHeaderXML(t *testing.T) {
	result := generatePanelHeaderXML(
		"Test Title",
		100000, 200000, 3000000, 700000,
		42,
		"accent1", 15000, 85000,
	)

	// Should be a p:sp element
	if !strings.HasPrefix(result, "<p:sp>") {
		t.Error("should start with <p:sp>")
	}
	if !strings.HasSuffix(result, "</p:sp>") {
		t.Error("should end with </p:sp>")
	}

	// Should contain scheme color fill with lumMod/lumOff
	if !strings.Contains(result, `schemeClr val="accent1"`) {
		t.Error("header fill should use schemeClr")
	}
	if !strings.Contains(result, `lumMod val="15000"`) {
		t.Error("header fill should have lumMod")
	}
	if !strings.Contains(result, `lumOff val="85000"`) {
		t.Error("header fill should have lumOff")
	}

	// Should NOT contain srgbClr
	if strings.Contains(result, "srgbClr") {
		t.Error("header should not contain srgbClr (hardcoded hex)")
	}

	// Should have centered bold text
	if !strings.Contains(result, `b="1"`) {
		t.Error("header text should be bold")
	}
	if !strings.Contains(result, `algn="ctr"`) {
		t.Error("header text should be centered")
	}

	// Should use scheme color for text
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("header text should use dk1 scheme color")
	}

	// Should contain the title text
	if !strings.Contains(result, "Test Title") {
		t.Error("should contain title text")
	}

	// Should have correct position
	if !strings.Contains(result, `x="100000"`) || !strings.Contains(result, `y="200000"`) {
		t.Error("should contain correct offset")
	}
	if !strings.Contains(result, `cx="3000000"`) || !strings.Contains(result, `cy="700000"`) {
		t.Error("should contain correct extent")
	}

	// Should have noAutofit
	if !strings.Contains(result, "<a:noAutofit/>") {
		t.Error("header should use noAutofit")
	}
}

func TestGeneratePanelHeaderXML_XMLEscaping(t *testing.T) {
	result := generatePanelHeaderXML(
		"Risk & Compliance",
		0, 0, 1000000, 500000,
		1,
		"accent1", 15000, 85000,
	)

	if !strings.Contains(result, "Risk &amp; Compliance") {
		t.Error("title should be XML-escaped")
	}
}

func TestGeneratePanelBodyXML(t *testing.T) {
	result := generatePanelBodyXML(
		"- Item 1\n- Item 2",
		100000, 900000, 3000000, 3000000,
		43,
		"tx1",
		panelBodyFontSize,
	)

	// Should be a p:sp element
	if !strings.HasPrefix(result, "<p:sp>") {
		t.Error("should start with <p:sp>")
	}

	// Should have noFill background
	if !strings.Contains(result, "<a:noFill/>") {
		t.Error("body should have noFill")
	}

	// Should have scheme color border
	if !strings.Contains(result, `schemeClr val="tx1"`) {
		t.Error("body border should use schemeClr tx1")
	}

	// Should NOT contain srgbClr
	if strings.Contains(result, "srgbClr") {
		t.Error("body should not contain srgbClr (hardcoded hex)")
	}

	// Should have normAutofit
	if !strings.Contains(result, "<a:normAutofit/>") {
		t.Error("body should use normAutofit")
	}

	// Should contain bullet paragraphs
	if !strings.Contains(result, `buChar char=`) {
		t.Error("body with bullet text should contain bullet characters")
	}

	// Should have text margins
	if !strings.Contains(result, fmt.Sprintf(`lIns="%d"`, panelBodyMarginLeft)) {
		t.Error("should have left text inset")
	}
	if !strings.Contains(result, fmt.Sprintf(`bIns="%d"`, panelBodyMarginBottom)) {
		t.Error("should have bottom text inset")
	}

	// Should have anchor="t" (top-aligned)
	if !strings.Contains(result, `anchor="t"`) {
		t.Error("body text should be top-aligned")
	}
}

func TestGeneratePanelBodyXML_EmptyBody(t *testing.T) {
	result := generatePanelBodyXML(
		"",
		0, 0, 1000000, 2000000,
		5,
		"tx1",
		panelBodyFontSize,
	)

	// Should still produce a valid shape
	if !strings.Contains(result, "<p:sp>") {
		t.Error("empty body should still produce a p:sp")
	}

	// Should NOT contain any a:p elements (no bullet text)
	if strings.Contains(result, "<a:p>") {
		t.Error("empty body should not contain <a:p> paragraphs")
	}
}

func TestGeneratePanelGroupXML_TwoPanels(t *testing.T) {
	panels := []nativePanelData{
		{title: "Panel A", body: "- Item 1"},
		{title: "Panel B", body: "- Item 2"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 6000000, Height: 4000000}

	result := generatePanelGroupXML(panels, bounds, 100)

	// Should be a group shape
	if !strings.HasPrefix(result, "<p:grpSp>") {
		t.Error("should start with <p:grpSp>")
	}
	if !strings.HasSuffix(result, "</p:grpSp>") {
		t.Error("should end with </p:grpSp>")
	}

	// Should contain chOff and chExt matching off and ext (identity transform)
	if !strings.Contains(result, fmt.Sprintf(`<a:chOff x="%d" y="%d"/>`, bounds.X, bounds.Y)) {
		t.Error("chOff should match group offset")
	}
	if !strings.Contains(result, fmt.Sprintf(`<a:chExt cx="%d" cy="%d"/>`, bounds.Width, bounds.Height)) {
		t.Error("chExt should match group extent")
	}

	// Should contain 2 panel titles
	if !strings.Contains(result, "Panel A") || !strings.Contains(result, "Panel B") {
		t.Error("should contain both panel titles")
	}

	// Should contain 4 child shapes (2 headers + 2 bodies)
	spCount := strings.Count(result, "<p:sp>")
	if spCount != 4 {
		t.Errorf("expected 4 child shapes (2 headers + 2 bodies), got %d", spCount)
	}

	// Width check: 2 panels, 1 gap
	// panelWidth = (6000000 - 1 * 202441) / 2 = 2898779
	expectedWidth := (bounds.Width - panelGap) / 2
	if !strings.Contains(result, fmt.Sprintf(`cx="%d"`, expectedWidth)) {
		t.Errorf("panel width should be %d", expectedWidth)
	}

	// Should use scheme colors, NOT hardcoded hex
	if strings.Contains(result, "srgbClr") {
		t.Error("group XML should not contain srgbClr (hardcoded hex colors)")
	}
	if !strings.Contains(result, "schemeClr") {
		t.Error("group XML should contain schemeClr references")
	}
}

func TestGeneratePanelGroupXML_FourPanels(t *testing.T) {
	panels := []nativePanelData{
		{title: "Strategy", body: "- Goal 1\n- Goal 2"},
		{title: "Operations", body: "- Process A"},
		{title: "Technology", body: "- System X\n- System Y"},
		{title: "People", body: "- Training"},
	}
	bounds := types.BoundingBox{X: 329610, Y: 2129246, Width: 11850000, Height: 4197531}

	result := generatePanelGroupXML(panels, bounds, 200)

	// Should contain all 4 panel titles
	for _, p := range panels {
		if !strings.Contains(result, p.title) {
			t.Errorf("should contain panel title %q", p.title)
		}
	}

	// Should contain 8 child shapes (4 headers + 4 bodies)
	spCount := strings.Count(result, "<p:sp>")
	if spCount != 8 {
		t.Errorf("expected 8 child shapes, got %d", spCount)
	}

	// Width: (11850000 - 3*202441) / 4 = 2811169 (integer division)
	expectedWidth := (bounds.Width - 3*panelGap) / 4
	if !strings.Contains(result, fmt.Sprintf(`cx="%d"`, expectedWidth)) {
		t.Errorf("4-panel width should be %d", expectedWidth)
	}
}

func TestGeneratePanelGroupXML_SixPanels(t *testing.T) {
	panels := make([]nativePanelData, 6)
	for i := range panels {
		panels[i] = nativePanelData{
			title: fmt.Sprintf("Panel %d", i+1),
			body:  fmt.Sprintf("- Content %d", i+1),
		}
	}
	bounds := types.BoundingBox{X: 300000, Y: 2000000, Width: 12000000, Height: 4000000}

	result := generatePanelGroupXML(panels, bounds, 300)

	// 6 panels should still render (readability concern noted in task)
	spCount := strings.Count(result, "<p:sp>")
	if spCount != 12 {
		t.Errorf("expected 12 child shapes (6 headers + 6 bodies), got %d", spCount)
	}

	// Width: (12000000 - 5*202441) / 6 = 1831298 (integer division)
	expectedWidth := (bounds.Width - 5*panelGap) / 6
	if expectedWidth <= 0 {
		t.Fatal("6-panel width should be positive")
	}
	if !strings.Contains(result, fmt.Sprintf(`cx="%d"`, expectedWidth)) {
		t.Errorf("6-panel width should be %d", expectedWidth)
	}
}

func TestGeneratePanelGroupXML_Empty(t *testing.T) {
	result := generatePanelGroupXML(nil, types.BoundingBox{}, 100)
	if result != "" {
		t.Errorf("empty panels should return empty string, got %q", result)
	}
}

func TestGeneratePanelGroupXML_MissingIcon(t *testing.T) {
	panels := []nativePanelData{
		{title: "Panel 1", body: "- Content", iconBytes: nil},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 5000000, Height: 4000000}

	result := generatePanelGroupXML(panels, bounds, 50)

	// Should NOT contain p:pic elements when icon is nil
	if strings.Contains(result, "<p:pic>") {
		t.Error("nil icon bytes should not produce p:pic element")
	}

	// Should NOT contain PANEL_ICON_REL placeholder
	if strings.Contains(result, "PANEL_ICON_REL") {
		t.Error("should not contain PANEL_ICON_REL placeholder")
	}
}

func TestGeneratePanelGroupXML_ChildOffsetArithmetic(t *testing.T) {
	panels := []nativePanelData{
		{title: "Only Panel", body: "- Single item"},
	}
	bounds := types.BoundingBox{X: 500000, Y: 1000000, Width: 4000000, Height: 3000000}

	result := generatePanelGroupXML(panels, bounds, 10)

	// chOff must match group offset
	expectedChOff := fmt.Sprintf(`<a:chOff x="%d" y="%d"/>`, bounds.X, bounds.Y)
	if !strings.Contains(result, expectedChOff) {
		t.Errorf("chOff should be %s", expectedChOff)
	}

	// chExt must match group extent
	expectedChExt := fmt.Sprintf(`<a:chExt cx="%d" cy="%d"/>`, bounds.Width, bounds.Height)
	if !strings.Contains(result, expectedChExt) {
		t.Errorf("chExt should be %s", expectedChExt)
	}

	// Header should start at group origin
	expectedHeaderOff := fmt.Sprintf(`<a:off x="%d" y="%d"/>`, bounds.X, bounds.Y)
	if !strings.Contains(result, expectedHeaderOff) {
		t.Error("header offset should match group origin")
	}

	// Body Y should be offset by header height + gap
	headerCY := int64(float64(bounds.Height) * panelHeaderHeightRatio)
	gapCY := int64(float64(bounds.Height) * panelGapHeightRatio)
	bodyY := bounds.Y + headerCY + gapCY
	expectedBodyOff := fmt.Sprintf(`<a:off x="%d" y="%d"/>`, bounds.X, bodyY)
	if !strings.Contains(result, expectedBodyOff) {
		t.Errorf("body offset should be at y=%d (header=%d + gap=%d)", bodyY, headerCY, gapCY)
	}
}

func TestGeneratePanelGroupXML_ValidXML(t *testing.T) {
	panels := []nativePanelData{
		{title: "Test Panel", body: "- Bullet 1\n- Bullet 2\nPlain line"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 5000000, Height: 4000000}

	result := generatePanelGroupXML(panels, bounds, 1)

	// Verify the generated XML is well-formed by parsing it
	var parsed interface{}
	err := xml.Unmarshal([]byte(result), &parsed)
	if err != nil {
		t.Errorf("generated XML should be well-formed, got parse error: %v\nXML:\n%s", err, result)
	}
}

func TestGeneratePanelGroupXML_SchemeColorAssertions(t *testing.T) {
	panels := []nativePanelData{
		{title: "Themed Panel", body: "- Theme-aware content"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 5000000, Height: 4000000}

	result := generatePanelGroupXML(panels, bounds, 1)

	// MUST contain schemeClr for fills and borders
	if !strings.Contains(result, "schemeClr") {
		t.Error("output must contain schemeClr references for theme-aware rendering")
	}

	// MUST NOT contain srgbClr (no hardcoded hex colors)
	if strings.Contains(result, "srgbClr") {
		t.Error("output must not contain srgbClr — all colors should use scheme references")
	}

	// Should reference accent1 for header fill
	if !strings.Contains(result, `schemeClr val="accent1"`) {
		t.Error("header fill should reference accent1 scheme color")
	}

	// Should reference tx1 for body border
	if !strings.Contains(result, `schemeClr val="tx1"`) {
		t.Error("body border should reference tx1 scheme color")
	}

	// Should reference dk1 for header text
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("header text should reference dk1 scheme color")
	}
}

func TestGeneratePanelGroupXML_MultiPanelPositions(t *testing.T) {
	panels := []nativePanelData{
		{title: "A", body: ""},
		{title: "B", body: ""},
		{title: "C", body: ""},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 9000000, Height: 4000000}

	result := generatePanelGroupXML(panels, bounds, 50)

	// Calculate expected panel width: (9000000 - 2*202441) / 3
	expectedWidth := (bounds.Width - 2*panelGap) / 3

	// Panel 1: x = 100000
	// Panel 2: x = 100000 + expectedWidth + panelGap
	// Panel 3: x = 100000 + 2*(expectedWidth + panelGap)
	panel1X := bounds.X
	panel2X := bounds.X + expectedWidth + panelGap
	panel3X := bounds.X + 2*(expectedWidth+panelGap)

	if !strings.Contains(result, fmt.Sprintf(`x="%d"`, panel1X)) {
		t.Errorf("panel 1 should start at x=%d", panel1X)
	}
	if !strings.Contains(result, fmt.Sprintf(`x="%d"`, panel2X)) {
		t.Errorf("panel 2 should start at x=%d", panel2X)
	}
	if !strings.Contains(result, fmt.Sprintf(`x="%d"`, panel3X)) {
		t.Errorf("panel 3 should start at x=%d", panel3X)
	}
}

func TestAllocatePanelIconRelIDs_IconAndNoIcon(t *testing.T) {
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				3: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 6000000, Height: 4000000},
						panels: []nativePanelData{
							{title: "With Icon", body: "- Content", iconBytes: []byte{0x89, 0x50, 0x4E, 0x47}},
							{title: "No Icon", body: "- Other content", iconBytes: nil},
						},
					},
				},
			},
		},
		MediaContext: MediaContext{
			media:           pptx.NewMediaAllocator(),
			usedExtensions:  make(map[string]bool),
			mediaFiles:      make(map[string]string),
			slideRelUpdates: make(map[int][]mediaRel),
		},
		SVGContext: SVGContext{
			nativeSVGInserts: make(map[int][]nativeSVGInsert),
		},
	}

	ctx.allocatePanelIconRelIDs()

	inserts := ctx.panelShapeInserts[3]
	if len(inserts) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(inserts))
	}

	panels := inserts[0].panels

	// Panel with icon should get rel ID and media file
	if panels[0].iconRelID == "" {
		t.Error("panel with iconBytes should have iconRelID allocated")
	}
	if panels[0].iconMediaFile == "" {
		t.Error("panel with iconBytes should have iconMediaFile allocated")
	}

	// Panel without icon should have empty rel ID
	if panels[1].iconRelID != "" {
		t.Errorf("panel without iconBytes should have empty iconRelID, got %q", panels[1].iconRelID)
	}
	if panels[1].iconMediaFile != "" {
		t.Errorf("panel without iconBytes should have empty iconMediaFile, got %q", panels[1].iconMediaFile)
	}

	// usedExtensions should include "png"
	if !ctx.usedExtensions["png"] {
		t.Error("usedExtensions should include 'png' after icon allocation")
	}

	// groupXML should be non-empty (generated during allocation)
	if inserts[0].groupXML == "" {
		t.Error("groupXML should be generated during allocatePanelIconRelIDs")
	}

	// groupXML should NOT contain PANEL_ICON_REL placeholder tokens
	if strings.Contains(inserts[0].groupXML, "PANEL_ICON_REL") {
		t.Error("groupXML should not contain PANEL_ICON_REL placeholder tokens")
	}

	// groupXML should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed); err != nil {
		t.Errorf("groupXML should be valid XML, got parse error: %v", err)
	}
}

func TestAllocatePanelIconRelIDs_RelIDAfterExistingRels(t *testing.T) {
	// Verify that icon rel IDs come after media and SVG rel IDs.
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				5: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 5000000, Height: 3000000},
						panels: []nativePanelData{
							{title: "Icon Panel", body: "- text", iconBytes: []byte{0x01}},
						},
					},
				},
			},
		},
		MediaContext: MediaContext{
			media:          pptx.NewMediaAllocator(),
			usedExtensions: make(map[string]bool),
			mediaFiles:     make(map[string]string),
			slideRelUpdates: map[int][]mediaRel{
				5: {{mediaFileName: "image1.png", relID: "rId2"}}, // 1 media rel
			},
		},
		SVGContext: SVGContext{
			nativeSVGInserts: map[int][]nativeSVGInsert{
				5: {
					{pngRelID: "rId3", svgRelID: "rId4"}, // 1 SVG insert = 2 rels
				},
			},
		},
	}

	ctx.allocatePanelIconRelIDs()

	panels := ctx.panelShapeInserts[5][0].panels
	// rId1 = layout, rId2 = media, rId3+rId4 = SVG, so icon should be rId5
	if panels[0].iconRelID != "rId5" {
		t.Errorf("icon relID should be rId5 (after 1 media + 2 SVG rels), got %q", panels[0].iconRelID)
	}
}

func TestAllocatePanelIconRelIDs_NoPanels(t *testing.T) {
	// Verify no-op when there are no panel inserts.
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: make(map[int][]panelShapeInsert),
		},
		MediaContext: MediaContext{
			media:           pptx.NewMediaAllocator(),
			usedExtensions:  make(map[string]bool),
			mediaFiles:      make(map[string]string),
			slideRelUpdates: make(map[int][]mediaRel),
		},
		SVGContext: SVGContext{
			nativeSVGInserts: make(map[int][]nativeSVGInsert),
		},
	}

	// Should not panic
	ctx.allocatePanelIconRelIDs()
}

func TestInsertPanelGroups(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?><p:sld><p:cSld><p:spTree><p:sp/></p:spTree></p:cSld></p:sld>`)
	inserts := []panelShapeInsert{
		{groupXML: `<p:grpSp><p:nvGrpSpPr/></p:grpSp>`},
	}

	result := insertPanelGroups(slideXML, inserts)

	// Group XML should appear before </p:spTree>
	if !strings.Contains(string(result), `<p:grpSp><p:nvGrpSpPr/></p:grpSp>`) {
		t.Error("panel group XML should be inserted into slide")
	}

	// Should still contain the closing tag
	if !strings.Contains(string(result), `</p:spTree>`) {
		t.Error("closing </p:spTree> should still be present")
	}
}

func TestInsertPanelGroups_EmptyGroupXML(t *testing.T) {
	slideXML := []byte(`<p:sld><p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`)
	inserts := []panelShapeInsert{
		{groupXML: ""}, // No XML generated (e.g., no panels)
	}

	result := insertPanelGroups(slideXML, inserts)

	// Should return unchanged
	if string(result) != string(slideXML) {
		t.Error("empty groupXML should not modify slide data")
	}
}

func TestRemovePanelPlaceholders(t *testing.T) {
	slide := &slideXML{}
	slide.CommonSlideData.ShapeTree.Shapes = []shapeXML{
		{}, // index 0
		{}, // index 1 — to be removed
		{}, // index 2
	}
	inserts := []panelShapeInsert{
		{placeholderIdx: 1},
	}

	ctx := &singlePassContext{}
	result := ctx.removePanelPlaceholders(slide, inserts)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 2 {
		t.Errorf("expected 2 shapes after removal, got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}
}

func TestIsPanelNativeLayout(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "panel_layout default (no layout field)",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"panels": []any{}}},
			expected: true,
		},
		{
			name:     "panel_layout columns explicit",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "columns"}},
			expected: true,
		},
		{
			name:     "panel_layout rows",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "rows"}},
			expected: true,
		},
		{
			name:     "panel_layout stat_cards",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "stat_cards"}},
			expected: true,
		},
		{
			name:     "panel_layout grid (unsupported)",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "grid"}},
			expected: false,
		},
		{
			name:     "bar_chart (not panel_layout)",
			spec:     &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPanelNativeLayout(tt.spec)
			if got != tt.expected {
				t.Errorf("isPanelNativeLayout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPanelLayoutMode(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected string
	}{
		{
			name:     "default (no layout field)",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{}},
			expected: "columns",
		},
		{
			name:     "explicit columns",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "columns"}},
			expected: "columns",
		},
		{
			name:     "rows",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "rows"}},
			expected: "rows",
		},
		{
			name:     "stat_cards",
			spec:     &types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "stat_cards"}},
			expected: "stat_cards",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := panelLayoutMode(tt.spec)
			if got != tt.expected {
				t.Errorf("panelLayoutMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Rows Layout Tests
// =============================================================================

func TestGeneratePanelRowsGroupXML_ThreeRows(t *testing.T) {
	panels := []nativePanelData{
		{title: "Revenue", body: "- Strong Q4 growth\n- New markets"},
		{title: "Costs", body: "- Reduced OPEX"},
		{title: "Margin", body: "- Improved by 3%"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 4000000}

	result := generatePanelRowsGroupXML(panels, bounds, 100)

	// Should be a group shape
	if !strings.HasPrefix(result, "<p:grpSp>") {
		t.Error("should start with <p:grpSp>")
	}
	if !strings.HasSuffix(result, "</p:grpSp>") {
		t.Error("should end with </p:grpSp>")
	}

	// Should contain all 3 panel titles
	for _, p := range panels {
		if !strings.Contains(result, p.title) {
			t.Errorf("should contain panel title %q", p.title)
		}
	}

	// Should contain 6 child shapes (3 headers + 3 bodies)
	spCount := strings.Count(result, "<p:sp>")
	if spCount != 6 {
		t.Errorf("expected 6 child shapes (3 headers + 3 bodies), got %d", spCount)
	}

	// Group name
	if !strings.Contains(result, `name="Panel Rows"`) {
		t.Error("group should be named 'Panel Rows'")
	}

	// Should use scheme colors
	if strings.Contains(result, "srgbClr") {
		t.Error("should not contain srgbClr (hardcoded hex colors)")
	}
	if !strings.Contains(result, "schemeClr") {
		t.Error("should contain schemeClr references")
	}

	// Header width should be ~25% of total
	expectedHeaderCX := int64(float64(bounds.Width) * rowHeaderWidthRatio)
	if !strings.Contains(result, fmt.Sprintf(`cx="%d"`, expectedHeaderCX)) {
		t.Errorf("header width should be %d", expectedHeaderCX)
	}
}

func TestGeneratePanelRowsGroupXML_Empty(t *testing.T) {
	result := generatePanelRowsGroupXML(nil, types.BoundingBox{}, 100)
	if result != "" {
		t.Error("empty panels should produce empty string")
	}
}

func TestGeneratePanelRowsGroupXML_SingleRow(t *testing.T) {
	panels := []nativePanelData{
		{title: "Key Finding", body: "- Important insight"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 4000000}

	result := generatePanelRowsGroupXML(panels, bounds, 100)

	// Single panel should use full height (no gaps)
	spCount := strings.Count(result, "<p:sp>")
	if spCount != 2 {
		t.Errorf("expected 2 child shapes (1 header + 1 body), got %d", spCount)
	}
}

// =============================================================================
// Stat Cards Layout Tests
// =============================================================================

func TestGenerateStatCardsGroupXML_ThreeCards(t *testing.T) {
	panels := []nativePanelData{
		{title: "Revenue", value: "$1.2M", body: "+12% YoY"},
		{title: "Users", value: "50K", body: "+8% MoM"},
		{title: "Churn", value: "2.1%", body: "-0.5% QoQ"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 9000000, Height: 4000000}

	result := generateStatCardsGroupXML(panels, bounds, 100)

	// Should be a group shape
	if !strings.HasPrefix(result, "<p:grpSp>") {
		t.Error("should start with <p:grpSp>")
	}
	if !strings.HasSuffix(result, "</p:grpSp>") {
		t.Error("should end with </p:grpSp>")
	}

	// Should contain all values and titles
	if !strings.Contains(result, "$1.2M") || !strings.Contains(result, "50K") || !strings.Contains(result, "2.1%") {
		t.Error("should contain all stat card values")
	}
	if !strings.Contains(result, "Revenue") || !strings.Contains(result, "Users") || !strings.Contains(result, "Churn") {
		t.Error("should contain all stat card titles")
	}

	// 3 cards = 3 child shapes (one rect each)
	spCount := strings.Count(result, "<p:sp>")
	if spCount != 3 {
		t.Errorf("expected 3 child shapes (1 per card), got %d", spCount)
	}

	// Group name
	if !strings.Contains(result, `name="Stat Cards"`) {
		t.Error("group should be named 'Stat Cards'")
	}

	// Should use roundRect geometry
	if !strings.Contains(result, `prst="roundRect"`) {
		t.Error("stat cards should use roundRect geometry")
	}

	// Delta coloring: "+12%" should use accent6, "-0.5%" should use accent2
	if !strings.Contains(result, `val="accent6"`) {
		t.Error("positive delta should use accent6 scheme color")
	}
	if !strings.Contains(result, `val="accent2"`) {
		t.Error("negative delta should use accent2 scheme color")
	}
}

func TestGenerateStatCardsGroupXML_Empty(t *testing.T) {
	result := generateStatCardsGroupXML(nil, types.BoundingBox{}, 100)
	if result != "" {
		t.Error("empty panels should produce empty string")
	}
}

func TestGenerateStatCardsGroupXML_ValueFallbackToTitle(t *testing.T) {
	panels := []nativePanelData{
		{title: "42%", body: "Completion rate"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 3000000, Height: 2000000}

	result := generateStatCardsGroupXML(panels, bounds, 100)

	// When value is empty, title should be used as the display value
	if !strings.Contains(result, "42%") {
		t.Error("should contain title used as value fallback")
	}
}

func TestGenerateStatCardsGroupXML_SixCardsGrid(t *testing.T) {
	panels := make([]nativePanelData, 6)
	for i := range panels {
		panels[i] = nativePanelData{
			title: fmt.Sprintf("Metric %d", i+1),
			value: fmt.Sprintf("%d%%", (i+1)*10),
		}
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 9000000, Height: 4000000}

	result := generateStatCardsGroupXML(panels, bounds, 100)

	// 6 cards should produce 6 child shapes
	spCount := strings.Count(result, "<p:sp>")
	if spCount != 6 {
		t.Errorf("expected 6 child shapes, got %d", spCount)
	}

	// All values should be present
	for i := range 6 {
		v := fmt.Sprintf("%d%%", (i+1)*10)
		if !strings.Contains(result, v) {
			t.Errorf("should contain value %q", v)
		}
	}
}

func TestStatCardGridLayout(t *testing.T) {
	tests := []struct {
		n            int
		expectedCols int
		expectedRows int
	}{
		{1, 1, 1},
		{2, 2, 1},
		{3, 3, 1},
		{4, 4, 1},
		{5, 3, 2},
		{6, 3, 2},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("n=%d", tt.n), func(t *testing.T) {
			cols, rows := statCardGridLayout(tt.n)
			if cols != tt.expectedCols || rows != tt.expectedRows {
				t.Errorf("statCardGridLayout(%d) = (%d, %d), want (%d, %d)",
					tt.n, cols, rows, tt.expectedCols, tt.expectedRows)
			}
		})
	}
}

// =============================================================================
// allocatePanelIconRelIDs routing tests
// =============================================================================

func TestAllocatePanelIconRelIDs_RowsMode(t *testing.T) {
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 4000000},
						panels: []nativePanelData{
							{title: "Row A", body: "- Item 1"},
							{title: "Row B", body: "- Item 2"},
						},
						rowsMode: true,
					},
				},
			},
		},
		MediaContext: MediaContext{
			usedExtensions:  make(map[string]bool),
			slideRelUpdates: make(map[int][]mediaRel),
		},
		SVGContext: SVGContext{
			nativeSVGInserts: make(map[int][]nativeSVGInsert),
		},
	}

	ctx.allocatePanelIconRelIDs()

	inserts := ctx.panelShapeInserts[1]
	if len(inserts) == 0 || inserts[0].groupXML == "" {
		t.Fatal("expected groupXML to be generated for rows mode")
	}

	if !strings.Contains(inserts[0].groupXML, `name="Panel Rows"`) {
		t.Error("rows mode should produce Panel Rows group")
	}
}

func TestAllocatePanelIconRelIDs_StatCardsMode(t *testing.T) {
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 9000000, Height: 4000000},
						panels: []nativePanelData{
							{title: "Revenue", value: "$1.2M"},
							{title: "Users", value: "50K"},
						},
						statCardsMode: true,
					},
				},
			},
		},
		MediaContext: MediaContext{
			usedExtensions:  make(map[string]bool),
			slideRelUpdates: make(map[int][]mediaRel),
		},
		SVGContext: SVGContext{
			nativeSVGInserts: make(map[int][]nativeSVGInsert),
		},
	}

	ctx.allocatePanelIconRelIDs()

	inserts := ctx.panelShapeInserts[1]
	if len(inserts) == 0 || inserts[0].groupXML == "" {
		t.Fatal("expected groupXML to be generated for stat_cards mode")
	}

	if !strings.Contains(inserts[0].groupXML, `name="Stat Cards"`) {
		t.Error("stat_cards mode should produce Stat Cards group")
	}
}
