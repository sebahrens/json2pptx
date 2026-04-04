package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/internal/types"
)

// consultingThemeColors returns consulting-style theme colors for testing.
func consultingThemeColors() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#000000"},
		{Name: "lt2", RGB: "#EBEBEB"},
		{Name: "accent1", RGB: "#FD5108"}, // brand orange
		{Name: "accent2", RGB: "#FE7C39"},
		{Name: "accent3", RGB: "#FFAA72"},
		{Name: "accent4", RGB: "#A1A8B3"},
		{Name: "accent5", RGB: "#B5BCC4"},
		{Name: "accent6", RGB: "#CBD1D6"},
	}
}

func TestExtractLayoutBackgroundColor(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		want     string
	}{
		{
			name: "sRGB solid fill background",
			xml: `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
				<p:cSld name="Section Header">
					<p:bg>
						<p:bgPr>
							<a:solidFill>
								<a:srgbClr val="FFE8D4"/>
							</a:solidFill>
						</p:bgPr>
					</p:bg>
					<p:spTree/>
				</p:cSld>
			</p:sldLayout>`,
			want: "#FFE8D4",
		},
		{
			name: "scheme color background",
			xml: `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
				<p:cSld>
					<p:bg>
						<p:bgPr>
							<a:solidFill>
								<a:schemeClr val="bg2"/>
							</a:solidFill>
						</p:bgPr>
					</p:bg>
					<p:spTree/>
				</p:cSld>
			</p:sldLayout>`,
			want: "#EBEBEB",
		},
		{
			name: "no background element",
			xml: `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
				<p:cSld>
					<p:spTree/>
				</p:cSld>
			</p:sldLayout>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLayoutBackgroundColor([]byte(tt.xml), consultingThemeColors())
			if got != tt.want {
				t.Errorf("extractLayoutBackgroundColor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSchemeColorToHex(t *testing.T) {
	tc := consultingThemeColors()

	tests := []struct {
		scheme string
		want   string
	}{
		{"accent1", "#FD5108"},
		{"accent2", "#FE7C39"},
		{"tx1", "#000000"},
		{"bg1", "#FFFFFF"},
		{"bg2", "#EBEBEB"},
		{"unknown_color", ""},
	}

	for _, tt := range tests {
		t.Run(tt.scheme, func(t *testing.T) {
			got := resolveSchemeColorToHex(tt.scheme, tc)
			if got != tt.want {
				t.Errorf("resolveSchemeColorToHex(%q) = %q, want %q", tt.scheme, got, tt.want)
			}
		})
	}
}

func TestFixSchemeColorsForContrast(t *testing.T) {
	tc := consultingThemeColors()
	// Section divider background: #FFE8D4 (peach/cream)
	bgColor := svggen.MustParseColor("#FFE8D4")

	tests := []struct {
		name       string
		xmlIn      string
		wantChange bool    // true if we expect the XML to be modified
		wantNoScheme bool  // true if scheme color should be replaced with sRGB
	}{
		{
			name: "accent1 on cream background - low contrast, should fix",
			xmlIn: `<a:lvl1pPr><a:defRPr sz="2400"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl1pPr>`,
			wantChange:   true,
			wantNoScheme: true,
		},
		{
			name: "tx1 (black) on cream background - good contrast, no change",
			xmlIn: `<a:lvl1pPr><a:defRPr sz="2400"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl1pPr>`,
			wantChange:   false,
			wantNoScheme: false,
		},
		{
			name: "no solidFill - no change",
			xmlIn: `<a:lvl1pPr><a:defRPr sz="2400"/></a:lvl1pPr>`,
			wantChange:   false,
			wantNoScheme: false,
		},
		{
			name: "sRGB color already - not touched",
			xmlIn: `<a:lvl1pPr><a:defRPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:defRPr></a:lvl1pPr>`,
			wantChange:   false,
			wantNoScheme: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixSchemeColorsForContrast(tt.xmlIn, bgColor, tc, "TestShape", "test")

			changed := got != tt.xmlIn
			if changed != tt.wantChange {
				t.Errorf("changed = %v, want %v\n  input:  %s\n  output: %s", changed, tt.wantChange, tt.xmlIn, got)
			}

			if tt.wantNoScheme && strings.Contains(got, "schemeClr") {
				t.Errorf("expected schemeClr to be replaced with srgbClr, but still found schemeClr in: %s", got)
			}

			if tt.wantNoScheme && !strings.Contains(got, "srgbClr") {
				t.Errorf("expected srgbClr in output, but not found in: %s", got)
			}

			// Verify the replacement color has good contrast
			if tt.wantChange && tt.wantNoScheme {
				// Extract the new sRGB color value
				idx := strings.Index(got, `srgbClr val="`)
				if idx < 0 {
					t.Fatal("could not find srgbClr val in output")
				}
				hexStart := idx + len(`srgbClr val="`)
				hexEnd := strings.Index(got[hexStart:], `"`)
				newHex := "#" + got[hexStart:hexStart+hexEnd]

				newColor, err := svggen.ParseColor(newHex)
				if err != nil {
					t.Fatalf("failed to parse replacement color %s: %v", newHex, err)
				}

				ratio := newColor.ContrastWith(bgColor)
				if ratio < svggen.WCAGAALarge {
					t.Errorf("replacement color %s has contrast ratio %.2f against %s, want >= %.1f",
						newHex, ratio, bgColor.Hex(), svggen.WCAGAALarge)
				}
			}
		})
	}
}

func TestEnforceTextContrastInSlide(t *testing.T) {
	tc := consultingThemeColors()
	bgHex := "#FFE8D4" // Cream background

	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Text Placeholder 2"},
						},
						TextBody: &textBodyXML{
							ListStyle: &listStyleXML{
								Inner: `<a:lvl1pPr marL="0" indent="0" algn="r"><a:lnSpc><a:spcPct val="80000"/></a:lnSpc><a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="0"/></a:spcAft><a:buNone/><a:defRPr sz="2400" b="0" kern="100" spc="-50" baseline="0"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl1pPr>`,
							},
							Paragraphs: []paragraphXML{
								{
									Runs: []runXML{
										{
											RunProperties: &runPropertiesXML{
												Lang: "en-US",
											},
											Text: "Test content",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	enforceTextContrastInSlide(slide, bgHex, tc)

	lstInner := slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner

	// Verify scheme color was replaced
	if strings.Contains(lstInner, `schemeClr val="accent1"`) {
		t.Error("expected accent1 schemeClr to be replaced, but it's still present")
	}

	if !strings.Contains(lstInner, "srgbClr") {
		t.Error("expected srgbClr replacement, but not found in lstStyle")
	}

	// Verify the defRPr and other attributes are preserved
	if !strings.Contains(lstInner, `sz="2400"`) {
		t.Error("expected sz attribute to be preserved")
	}
	if !strings.Contains(lstInner, `kern="100"`) {
		t.Error("expected kern attribute to be preserved")
	}
}

func TestEnforceTextContrastInSlide_NoBackground(t *testing.T) {
	tc := consultingThemeColors()

	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						TextBody: &textBodyXML{
							ListStyle: &listStyleXML{
								Inner: `<a:lvl1pPr><a:defRPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl1pPr>`,
							},
							Paragraphs: []paragraphXML{},
						},
					},
				},
			},
		},
	}

	original := slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner

	// No background = should not change anything
	enforceTextContrastInSlide(slide, "", tc)

	after := slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner
	if after != original {
		t.Errorf("expected no change when bgHex is empty, but lstStyle was modified:\n  before: %s\n  after:  %s", original, after)
	}
}

func TestEnforceTextContrastInSlide_GoodContrastUnchanged(t *testing.T) {
	tc := consultingThemeColors()

	// White background + black text (tx1 = dk1 = #000000) = great contrast
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						TextBody: &textBodyXML{
							ListStyle: &listStyleXML{
								Inner: `<a:lvl1pPr><a:defRPr><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl1pPr>`,
							},
							Paragraphs: []paragraphXML{},
						},
					},
				},
			},
		},
	}

	original := slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner
	enforceTextContrastInSlide(slide, "#FFFFFF", tc)

	after := slide.CommonSlideData.ShapeTree.Shapes[0].TextBody.ListStyle.Inner
	if after != original {
		t.Errorf("expected no change for high-contrast text, but lstStyle was modified:\n  before: %s\n  after:  %s", original, after)
	}
}

func TestFixSchemeColorsForContrast_MultipleLevels(t *testing.T) {
	tc := consultingThemeColors()
	bgColor := svggen.MustParseColor("#FFE8D4")

	// Template layout has multiple levels: lvl1 with accent1, lvl2+ with tx1
	xmlIn := `<a:lvl1pPr><a:defRPr sz="2400"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl1pPr>` +
		`<a:lvl2pPr><a:defRPr sz="2000"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl2pPr>`

	got := fixSchemeColorsForContrast(xmlIn, bgColor, tc, "TestShape", "test")

	// accent1 should be replaced (low contrast)
	if strings.Contains(got, `schemeClr val="accent1"`) {
		t.Error("expected accent1 to be replaced due to low contrast")
	}

	// tx1 (black) should NOT be replaced (good contrast against cream)
	if !strings.Contains(got, `schemeClr val="tx1"`) {
		t.Error("expected tx1 to remain unchanged (good contrast)")
	}
}

// =============================================================================
// Shape Grid Contrast Tests
// =============================================================================

func TestExtractShapeFillHex(t *testing.T) {
	tc := consultingThemeColors()

	tests := []struct {
		name string
		xml  string
		want string
	}{
		{
			name: "sRGB solid fill",
			xml:  `<p:sp><p:spPr><a:solidFill><a:srgbClr val="1B2A4A"/></a:solidFill></p:spPr><p:txBody/></p:sp>`,
			want: "#1B2A4A",
		},
		{
			name: "scheme fill accent1",
			xml:  `<p:sp><p:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></p:spPr><p:txBody/></p:sp>`,
			want: "#FD5108",
		},
		{
			name: "noFill returns empty",
			xml:  `<p:sp><p:spPr><a:noFill/></p:spPr><p:txBody/></p:sp>`,
			want: "",
		},
		{
			name: "no spPr returns empty",
			xml:  `<p:sp><p:txBody/></p:sp>`,
			want: "",
		},
		{
			name: "does not match text color in txBody",
			xml:  `<p:sp><p:spPr><a:noFill/></p:spPr><p:txBody><a:p><a:r><a:rPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:rPr><a:t>X</a:t></a:r></a:p></p:txBody></p:sp>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractShapeFillHex([]byte(tt.xml), tc)
			if got != tt.want {
				t.Errorf("extractShapeFillHex() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWarnShapeXMLContrast_DoesNotModify(t *testing.T) {
	tc := consultingThemeColors()

	// Dark blue fill with dark text (dk1 = black) — low contrast, but should NOT be modified
	lowContrast := `<p:sp>
  <p:spPr><a:solidFill><a:srgbClr val="1B2A4A"/></a:solidFill></p:spPr>
  <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="dk1"/></a:solidFill></a:rPr><a:t>Bad</a:t></a:r></a:p></p:txBody>
</p:sp>`

	// warnShapeXMLContrast should only warn, not modify
	warnShapeXMLContrast([]byte(lowContrast), tc)
	// No return value to check — the function only logs warnings
}

func TestWarnShapeXMLContrast_SrgbTextPreserved(t *testing.T) {
	tc := consultingThemeColors()

	// Dark fill with low-contrast sRGB text color — user-specified, must be preserved
	lowContrast := `<p:sp>
  <p:spPr><a:solidFill><a:srgbClr val="1B2A4A"/></a:solidFill></p:spPr>
  <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:solidFill><a:srgbClr val="2B3A5A"/></a:solidFill></a:rPr><a:t>Low</a:t></a:r></a:p></p:txBody>
</p:sp>`

	// warnShapeXMLContrast should only warn, not modify
	warnShapeXMLContrast([]byte(lowContrast), tc)
}

func TestWarnShapeXMLContrast_NoFillSkipped(t *testing.T) {
	tc := consultingThemeColors()

	noFill := `<p:sp>
  <p:spPr><a:noFill/></p:spPr>
  <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="dk1"/></a:solidFill></a:rPr><a:t>X</a:t></a:r></a:p></p:txBody>
</p:sp>`

	// Should not panic or error on noFill shapes
	warnShapeXMLContrast([]byte(noFill), tc)
}

func TestEnforceShapeGridContrast_PreservesUserColors(t *testing.T) {
	tc := consultingThemeColors()

	shapes := [][]byte{
		// Good contrast: white text on dark fill
		[]byte(`<p:sp><p:spPr><a:solidFill><a:srgbClr val="1B2A4A"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:rPr><a:t>OK</a:t></a:r></a:p></p:txBody></p:sp>`),
		// Low contrast: black text on dark fill — user-specified, must be preserved
		[]byte(`<p:sp><p:spPr><a:solidFill><a:srgbClr val="1B2A4A"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="dk1"/></a:solidFill></a:rPr><a:t>Bad</a:t></a:r></a:p></p:txBody></p:sp>`),
	}

	result := enforceShapeGridContrast(shapes, tc)

	if len(result) != 2 {
		t.Fatalf("expected 2 shapes, got %d", len(result))
	}

	// Both shapes should be unchanged — user-specified colors are never replaced
	if string(result[0]) != string(shapes[0]) { //nolint:gosec // test validates length above
		t.Error("first shape should be unchanged")
	}
	if string(result[1]) != string(shapes[1]) {
		t.Error("second shape (low contrast) should be unchanged — user-specified colors preserved")
	}
}

func TestWarnShapeXMLContrast_SchemeColorFill(t *testing.T) {
	tc := consultingThemeColors()

	// dk1 (#000000 = black) fill with lt1 (white) text — good contrast
	shapeXML := `<p:sp><p:spPr><a:solidFill><a:schemeClr val="dk1"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:rPr><a:t>OK</a:t></a:r></a:p></p:txBody></p:sp>`

	// Should not panic on good contrast
	warnShapeXMLContrast([]byte(shapeXML), tc)

	// accent1 (#FD5108 = orange) fill with lt1 (white) text — contrast 3.3, below WCAG AA
	// Should warn but NOT modify (user-specified)
	lowContrastXML := `<p:sp><p:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:rPr><a:t>Fix</a:t></a:r></a:p></p:txBody></p:sp>`

	warnShapeXMLContrast([]byte(lowContrastXML), tc)
}

func TestFixSrgbColorsForContrast(t *testing.T) {
	bgColor := svggen.MustParseColor("#FFFFFF")

	tests := []struct {
		name       string
		xmlIn      string
		wantChange bool
	}{
		{
			name:       "light text on white - should fix",
			xmlIn:      `<a:solidFill><a:srgbClr val="EEEEEE"/></a:solidFill>`,
			wantChange: true,
		},
		{
			name:       "dark text on white - no change",
			xmlIn:      `<a:solidFill><a:srgbClr val="000000"/></a:solidFill>`,
			wantChange: false,
		},
		{
			name:       "no solidFill - no change",
			xmlIn:      `<a:rPr sz="1200"/>`,
			wantChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixSrgbColorsForContrast(tt.xmlIn, bgColor)
			changed := got != tt.xmlIn
			if changed != tt.wantChange {
				t.Errorf("changed = %v, want %v\n  input:  %s\n  output: %s", changed, tt.wantChange, tt.xmlIn, got)
			}
		})
	}
}
