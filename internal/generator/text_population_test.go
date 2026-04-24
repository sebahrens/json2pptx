package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

func TestHasBulletsDisabled(t *testing.T) {
	tests := []struct {
		name     string
		pProps   *paragraphPropertiesXML
		expected bool
	}{
		{
			name:     "nil properties",
			pProps:   nil,
			expected: false,
		},
		{
			name:     "empty inner",
			pProps:   &paragraphPropertiesXML{Inner: ""},
			expected: false,
		},
		{
			name:     "buNone element",
			pProps:   &paragraphPropertiesXML{Inner: `<a:buNone/>`},
			expected: true,
		},
		{
			name:     "buNone with other elements",
			pProps:   &paragraphPropertiesXML{Inner: `<a:buFont typeface="Arial"/><a:buNone/>`},
			expected: true,
		},
		{
			name:     "buChar element (has bullet)",
			pProps:   &paragraphPropertiesXML{Inner: `<a:buChar char="•"/>`},
			expected: false,
		},
		{
			name:     "buFont without buNone",
			pProps:   &paragraphPropertiesXML{Inner: `<a:buFont typeface="Arial"/>`},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasBulletsDisabled(tt.pProps)
			if result != tt.expected {
				t.Errorf("hasBulletsDisabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindFirstBulletLevel(t *testing.T) {
	tests := []struct {
		name     string
		styles   []bulletLevelStyle
		expected int
	}{
		{
			name:     "empty styles",
			styles:   nil,
			expected: -1,
		},
		{
			name: "first level has bullets",
			styles: []bulletLevelStyle{
				{pProps: &paragraphPropertiesXML{Inner: `<a:buChar char="•"/>`}},
				{pProps: &paragraphPropertiesXML{Inner: `<a:buChar char="-"/>`}},
			},
			expected: 0,
		},
		{
			name: "first level disabled, second has bullets",
			styles: []bulletLevelStyle{
				{pProps: &paragraphPropertiesXML{Inner: `<a:buFont typeface="Arial"/><a:buNone/>`}},
				{pProps: &paragraphPropertiesXML{Inner: `<a:buChar char="•"/>`}},
			},
			expected: 1,
		},
		{
			name: "Consulting-style: levels 0-1 disabled, level 2 has bullets",
			styles: []bulletLevelStyle{
				{pProps: &paragraphPropertiesXML{Inner: `<a:buFont typeface="Arial"/><a:buNone/><a:defRPr sz="1500" b="1"/>`}},
				{pProps: &paragraphPropertiesXML{Inner: `<a:buFont typeface="Arial"/><a:buNone/><a:defRPr sz="1400"/>`}},
				{pProps: &paragraphPropertiesXML{Inner: `<a:buFont typeface="Arial"/><a:buChar char="•"/><a:defRPr sz="1400"/>`}},
			},
			expected: 2,
		},
		{
			name: "all levels disabled",
			styles: []bulletLevelStyle{
				{pProps: &paragraphPropertiesXML{Inner: `<a:buNone/>`}},
				{pProps: &paragraphPropertiesXML{Inner: `<a:buNone/>`}},
			},
			expected: -1,
		},
		{
			name: "nil pProps (no styling defined)",
			styles: []bulletLevelStyle{
				{pProps: nil},
				{pProps: &paragraphPropertiesXML{Inner: `<a:buChar char="•"/>`}},
			},
			expected: 0, // nil pProps means no buNone, so bullets not disabled
		},
		{
			name: "empty Inner (no buNone)",
			styles: []bulletLevelStyle{
				{pProps: &paragraphPropertiesXML{Inner: ""}},
				{pProps: &paragraphPropertiesXML{Inner: `<a:buChar char="•"/>`}},
			},
			expected: 0, // empty Inner means no buNone, so bullets not disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findFirstBulletLevel(tt.styles)
			if result != tt.expected {
				t.Errorf("findFirstBulletLevel() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestExtractBulletTemplateStyles(t *testing.T) {
	tests := []struct {
		name       string
		paragraphs []paragraphXML
		wantLen    int
	}{
		{
			name:       "empty paragraphs",
			paragraphs: nil,
			wantLen:    0,
		},
		{
			name: "single paragraph with level",
			paragraphs: []paragraphXML{
				{
					Properties: &paragraphPropertiesXML{
						Level: bulletIntPtr(0),
						Inner: `<a:buChar char="•"/>`,
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "multiple levels",
			paragraphs: []paragraphXML{
				{Properties: &paragraphPropertiesXML{Level: bulletIntPtr(0), Inner: `<a:buNone/>`}},
				{Properties: &paragraphPropertiesXML{Level: bulletIntPtr(1), Inner: `<a:buNone/>`}},
				{Properties: &paragraphPropertiesXML{Level: bulletIntPtr(2), Inner: `<a:buChar char="•"/>`}},
			},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBulletTemplateStyles(tt.paragraphs)
			if len(result) != tt.wantLen {
				t.Errorf("extractBulletTemplateStyles() returned %d styles, want %d", len(result), tt.wantLen)
			}
		})
	}
}

// bulletIntPtr returns a pointer to an int (for bullet tests)
func bulletIntPtr(i int) *int {
	return &i
}

func TestApplySmartAutofit(t *testing.T) {
	// Helper to make a shape with dimensions and text
	shapeWithText := func(widthEMU, heightEMU int64, texts ...string) *shapeXML {
		paragraphs := make([]paragraphXML, len(texts))
		for i, text := range texts {
			paragraphs[i] = paragraphXML{
				Runs: []runXML{{Text: text}},
			}
		}
		return &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Extent: extentXML{CX: widthEMU, CY: heightEMU},
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Inner: ""},
				Paragraphs:     paragraphs,
			},
		}
	}

	t.Run("nil TextBody", func(t *testing.T) {
		shape := &shapeXML{}
		applySmartAutofit(shape) // should not panic
	})

	t.Run("nil BodyProperties", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{}}
		applySmartAutofit(shape) // should not panic
	})

	t.Run("skip if normAutofit already present", func(t *testing.T) {
		shape := shapeWithText(5*914400, 3*914400, "text")
		shape.TextBody.BodyProperties.Inner = `<a:normAutofit/>`
		applySmartAutofit(shape)
		if shape.TextBody.BodyProperties.Inner != `<a:normAutofit/>` {
			t.Errorf("should not modify existing autofit, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})

	t.Run("skip if noAutofit present", func(t *testing.T) {
		shape := shapeWithText(5*914400, 3*914400, "text")
		shape.TextBody.BodyProperties.Inner = `<a:noAutofit/>`
		applySmartAutofit(shape)
		if shape.TextBody.BodyProperties.Inner != `<a:noAutofit/>` {
			t.Errorf("should not modify existing noAutofit, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})

	t.Run("short text no scaling needed", func(t *testing.T) {
		shape := shapeWithText(5*914400, 3*914400, "Short text")
		applySmartAutofit(shape)
		// normAutofit is always added as a safety net (preserves master's shrink-to-fit)
		// but should NOT have fontScale or lnSpcReduction attributes
		if !strings.Contains(shape.TextBody.BodyProperties.Inner, "normAutofit") {
			t.Error("short text should still get normAutofit as safety net")
		}
		if strings.Contains(shape.TextBody.BodyProperties.Inner, "fontScale") {
			t.Error("short text should not have fontScale")
		}
	})

	t.Run("no dimensions falls back to basic autofit", func(t *testing.T) {
		shape := &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Inner: ""},
				Paragraphs: []paragraphXML{
					{Runs: []runXML{{Text: "text"}}},
				},
			},
		}
		applySmartAutofit(shape)
		// Should fall back to basic normAutofit (no fontScale) for few paragraphs
		if !strings.Contains(shape.TextBody.BodyProperties.Inner, "normAutofit") {
			t.Error("no-dimensions shape should get basic normAutofit fallback")
		}
		if strings.Contains(shape.TextBody.BodyProperties.Inner, "fontScale") {
			t.Error("few paragraphs without dimensions should not get fontScale")
		}
	})

	t.Run("no dimensions with many paragraphs gets fontScale", func(t *testing.T) {
		// 18 paragraphs (like 4-group bullet groups) without shape dimensions
		paragraphs := make([]paragraphXML, 18)
		for i := range paragraphs {
			paragraphs[i] = paragraphXML{Runs: []runXML{{Text: "Bullet item"}}}
		}
		shape := &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Inner: ""},
				Paragraphs:     paragraphs,
			},
		}
		applySmartAutofit(shape)
		inner := shape.TextBody.BodyProperties.Inner
		if !strings.Contains(inner, "normAutofit") {
			t.Error("many paragraphs without dimensions should get normAutofit")
		}
		if !strings.Contains(inner, "fontScale") {
			t.Error("18 paragraphs without dimensions should get fontScale")
		}
		// 14/18 = 77%, so fontScale should be 77000
		if !strings.Contains(inner, `fontScale="77000"`) {
			t.Errorf("expected fontScale=77000, got %q", inner)
		}
	})

	t.Run("many bullets get scaled", func(t *testing.T) {
		texts := make([]string, 20)
		for i := range texts {
			texts[i] = "This is a bullet point with descriptive text"
		}
		shape := shapeWithText(3*914400, 2*914400, texts...)
		applySmartAutofit(shape)
		inner := shape.TextBody.BodyProperties.Inner
		if !strings.Contains(inner, "normAutofit") {
			t.Error("many bullets should trigger autofit")
		}
		if !strings.Contains(inner, "fontScale") {
			t.Error("many bullets should have fontScale attribute")
		}
	})

	// Regression test for pptx-1dx: dense bullets with pre-existing normAutofit
	// should NOT be aggressively truncated. The old code had an early-return path
	// that preserved the template's normAutofit and used a 60% floor instead of
	// the caller's 45%, causing 12 bullets to be truncated to 4.
	t.Run("dense bullets with existing normAutofit use caller options", func(t *testing.T) {
		texts := make([]string, 12)
		for i := range texts {
			texts[i] = "Bullet item"
		}
		// 5" wide × 3" tall — enough for 12 short bullets at 45% scale
		shape := shapeWithText(5*914400, 3*914400, texts...)
		shape.TextBody.BodyProperties.Inner = `<a:normAutofit fontScale="62500"/>`
		applySmartAutofitWithOptions(shape, withReadabilityMinScale(35000), withMinFontScalePct(45))

		// Existing normAutofit should be replaced, not preserved
		inner := shape.TextBody.BodyProperties.Inner
		if strings.Contains(inner, `fontScale="62500"`) {
			t.Error("template normAutofit fontScale should be replaced with computed value")
		}
		if !strings.Contains(inner, "normAutofit") {
			t.Error("should have normAutofit after computation")
		}

		// With the 45% font scale floor, most bullets should be preserved.
		// The old code's 60% floor + early-return would truncate to ~4 paragraphs.
		if len(shape.TextBody.Paragraphs) < 8 {
			t.Errorf("expected at least 8 paragraphs with 45%% floor, got %d", len(shape.TextBody.Paragraphs))
		}
	})
}

func TestExtractFontSizeFromShape(t *testing.T) {
	tests := []struct {
		name     string
		shape    *shapeXML
		wantSize int
	}{
		{
			name:     "nil TextBody",
			shape:    &shapeXML{},
			wantSize: 0,
		},
		{
			name: "sz in run properties",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					Paragraphs: []paragraphXML{
						{Runs: []runXML{{
							RunProperties: &runPropertiesXML{Inner: `<a:latin typeface="Arial"/><a:sz val="1800"/>`, Lang: "en-US"},
							Text:          "test",
						}}},
					},
				},
			},
			wantSize: 0, // sz is an attribute not val
		},
		{
			name: "sz as attribute in run inner",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					Paragraphs: []paragraphXML{
						{Runs: []runXML{{
							RunProperties: &runPropertiesXML{Inner: `sz="1800"`, Lang: "en-US"},
							Text:          "test",
						}}},
					},
				},
			},
			wantSize: 1800,
		},
		{
			name: "sz in paragraph defRPr",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					Paragraphs: []paragraphXML{
						{Properties: &paragraphPropertiesXML{Inner: `<a:defRPr sz="1400"/>`}},
					},
				},
			},
			wantSize: 1400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFontSizeFromShape(tt.shape)
			if got != tt.wantSize {
				t.Errorf("extractFontSizeFromShape() = %d, want %d", got, tt.wantSize)
			}
		})
	}
}

func TestParseSzAttr(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{`sz="1800"`, 1800},
		{`<a:defRPr sz="1400" b="1"/>`, 1400},
		{`no size here`, 0},
		{`sz="abc"`, 0},
		{``, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseSzAttr(tt.input); got != tt.want {
				t.Errorf("parseSzAttr(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCapLstStyleFontSize(t *testing.T) {
	tests := []struct {
		name     string
		lstInner string
		maxHPt   int
		wantSz   int // expected sz after capping, 0 means unchanged
	}{
		{"nil TextBody", "", 4800, 0},
		{"under cap", `<a:lvl1pPr><a:defRPr sz="2400"/></a:lvl1pPr>`, 4800, 2400},
		{"at cap", `<a:lvl1pPr><a:defRPr sz="4800"/></a:lvl1pPr>`, 4800, 4800},
		{"over cap", `<a:lvl1pPr><a:defRPr sz="9600"/></a:lvl1pPr>`, 4800, 4800},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var shape *shapeXML
			if tt.lstInner == "" {
				shape = &shapeXML{}
			} else {
				shape = &shapeXML{
					TextBody: &textBodyXML{
						ListStyle: &listStyleXML{Inner: tt.lstInner},
					},
				}
			}
			capLstStyleFontSize(shape, tt.maxHPt)
			if shape.TextBody == nil {
				return // nil TextBody case
			}
			got := parseSzAttr(shape.TextBody.ListStyle.Inner)
			if got != tt.wantSz {
				t.Errorf("capLstStyleFontSize() sz = %d, want %d", got, tt.wantSz)
			}
		})
	}
}

func TestFloorLstStyleFontSize(t *testing.T) {
	tests := []struct {
		name     string
		lstInner string
		minHPt   int
		wantSz   int
	}{
		{"nil TextBody", "", 1200, 0},
		{"above floor", `<a:lvl1pPr><a:defRPr sz="2400"/></a:lvl1pPr>`, 1200, 2400},
		{"at floor", `<a:lvl1pPr><a:defRPr sz="1200"/></a:lvl1pPr>`, 1200, 1200},
		{"below floor", `<a:lvl1pPr><a:defRPr sz="600"/></a:lvl1pPr>`, 1200, 1200},
		{"very tiny font", `<a:lvl1pPr><a:defRPr sz="100"/></a:lvl1pPr>`, 1200, 1200},
		{"no sz attr", `<a:lvl1pPr><a:defRPr b="1"/></a:lvl1pPr>`, 1200, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var shape *shapeXML
			if tt.lstInner == "" {
				shape = &shapeXML{}
			} else {
				shape = &shapeXML{
					TextBody: &textBodyXML{
						ListStyle: &listStyleXML{Inner: tt.lstInner},
					},
				}
			}
			floorLstStyleFontSize(shape, tt.minHPt)
			if shape.TextBody == nil {
				return // nil TextBody case
			}
			got := parseSzAttr(shape.TextBody.ListStyle.Inner)
			if got != tt.wantSz {
				t.Errorf("floorLstStyleFontSize() sz = %d, want %d", got, tt.wantSz)
			}
		})
	}
}

func TestApplyFontSizeOverride(t *testing.T) {
	t.Run("sets sz on all runs", func(t *testing.T) {
		shape := &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
				ListStyle:      &listStyleXML{Inner: `<a:lvl1pPr><a:defRPr sz="2400"/></a:lvl1pPr>`},
				Paragraphs: []paragraphXML{
					{Runs: []runXML{
						{RunProperties: &runPropertiesXML{Lang: "en-US"}, Text: "Hello"},
						{RunProperties: &runPropertiesXML{Lang: "en-US"}, Text: " World"},
					}},
				},
			},
		}
		applyFontSizeOverride(shape, 7200) // 72pt
		for _, p := range shape.TextBody.Paragraphs {
			for _, r := range p.Runs {
				if r.RunProperties == nil || r.RunProperties.FontSize != "7200" {
					t.Errorf("expected sz=7200, got %v", r.RunProperties)
				}
			}
		}
		// lstStyle should also be updated
		if !strings.Contains(shape.TextBody.ListStyle.Inner, `sz="7200"`) {
			t.Errorf("expected lstStyle sz=7200, got %s", shape.TextBody.ListStyle.Inner)
		}
	})

	t.Run("creates rPr when nil", func(t *testing.T) {
		shape := &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
				ListStyle:      &listStyleXML{},
				Paragraphs: []paragraphXML{
					{Runs: []runXML{{Text: "Test"}}},
				},
			},
		}
		applyFontSizeOverride(shape, 3600) // 36pt
		rProps := shape.TextBody.Paragraphs[0].Runs[0].RunProperties
		if rProps == nil || rProps.FontSize != "3600" {
			t.Errorf("expected rPr with sz=3600, got %v", rProps)
		}
	})

	t.Run("nil TextBody is safe", func(t *testing.T) {
		shape := &shapeXML{}
		applyFontSizeOverride(shape, 7200) // should not panic
	})
}

func TestPopulateShapeTextFontSizeOverride(t *testing.T) {
	shape := &shapeXML{
		TextBody: &textBodyXML{
			BodyProperties: &bodyPropertiesXML{},
			ListStyle:      &listStyleXML{Inner: `<a:lvl1pPr><a:defRPr sz="2400"/></a:lvl1pPr>`},
			Paragraphs:     []paragraphXML{emptyParagraph()},
		},
	}
	item := ContentItem{
		PlaceholderID: "title",
		Type:          ContentText,
		Value:         "Thank You",
		FontSize:      7200, // 72pt
	}
	if err := populateShapeText(shape, item, -1, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify runs have sz override
	for _, p := range shape.TextBody.Paragraphs {
		for _, r := range p.Runs {
			if r.RunProperties == nil || r.RunProperties.FontSize != "7200" {
				t.Errorf("expected sz=7200 on run, got %v", r.RunProperties)
			}
		}
	}
}

func TestCollectParagraphTexts(t *testing.T) {
	paragraphs := []paragraphXML{
		{Runs: []runXML{{Text: "Hello"}, {Text: " World"}}},
		{Runs: []runXML{{Text: "Second paragraph"}}},
		{Runs: nil}, // empty paragraph
	}

	texts := collectParagraphTexts(paragraphs)
	if len(texts) != 3 {
		t.Fatalf("expected 3 texts, got %d", len(texts))
	}
	if texts[0] != "Hello World" {
		t.Errorf("texts[0] = %q, want %q", texts[0], "Hello World")
	}
	if texts[1] != "Second paragraph" {
		t.Errorf("texts[1] = %q, want %q", texts[1], "Second paragraph")
	}
	if texts[2] != "" {
		t.Errorf("texts[2] = %q, want empty", texts[2])
	}
}

func TestParseSpcBefPt(t *testing.T) {
	tests := []struct {
		name string
		xml  string
		want float64
	}{
		{"empty", "", 0},
		{"no spcBef", `<a:buNone/>`, 0},
		{"spcBef 12pt", `<a:spcBef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:spcPts val="1200"/></a:spcBef>`, 12.0},
		{"spcBef 24pt", `<a:spcBef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:spcPts val="2400"/></a:spcBef>`, 24.0},
		{"spcBef with buNone", `<a:spcBef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:spcPts val="1200"/></a:spcBef><a:buNone xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`, 12.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSpcBefPt(tt.xml)
			if got != tt.want {
				t.Errorf("parseSpcBefPt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractParagraphSpacings(t *testing.T) {
	baseSpacing := 12.0

	t.Run("no explicit spacing returns nil", func(t *testing.T) {
		paragraphs := []paragraphXML{
			{Properties: nil},
			{Properties: &paragraphPropertiesXML{Inner: `<a:buNone/>`}},
		}
		result := extractParagraphSpacings(paragraphs, baseSpacing)
		if result != nil {
			t.Errorf("expected nil for no explicit spacing, got %v", result)
		}
	})

	t.Run("header with spcBef", func(t *testing.T) {
		paragraphs := []paragraphXML{
			{Properties: &paragraphPropertiesXML{
				Inner: `<a:spcBef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:spcPts val="1200"/></a:spcBef><a:buNone xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`,
			}},
			{Properties: nil}, // bullet
			{Properties: nil}, // bullet
		}
		result := extractParagraphSpacings(paragraphs, baseSpacing)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result) != 3 {
			t.Fatalf("expected 3 spacings, got %d", len(result))
		}
		if result[0] != 12.0 {
			t.Errorf("header spacing = %v, want 12.0", result[0])
		}
		// Non-explicit paragraphs get 0 spacing when explicit spacings exist,
		// because normAutofit handles master-inherited spacing.
		if result[1] != 0 {
			t.Errorf("bullet spacing = %v, want 0", result[1])
		}
	})

	t.Run("trailing body with large spcBef", func(t *testing.T) {
		paragraphs := []paragraphXML{
			{Properties: nil}, // bullet
			{Properties: &paragraphPropertiesXML{
				Inner: `<a:spcBef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:spcPts val="2400"/></a:spcBef>`,
			}},
		}
		result := extractParagraphSpacings(paragraphs, baseSpacing)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		// Non-explicit paragraphs get 0 when explicit spacings exist.
		if result[0] != 0 {
			t.Errorf("bullet spacing = %v, want 0", result[0])
		}
		if result[1] != 24.0 {
			t.Errorf("trailing body spacing = %v, want 24.0", result[1])
		}
	})
}

func TestEnforceTextWrap(t *testing.T) {
	t.Run("nil TextBody is no-op", func(t *testing.T) {
		shape := &shapeXML{}
		enforceTextWrap(shape) // should not panic
	})

	t.Run("nil BodyProperties is no-op", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{}}
		enforceTextWrap(shape) // should not panic
	})

	t.Run("sets wrap=square on empty bodyPr", func(t *testing.T) {
		shape := &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
			},
		}
		enforceTextWrap(shape)
		if shape.TextBody.BodyProperties.Wrap != "square" {
			t.Errorf("Wrap = %q, want %q", shape.TextBody.BodyProperties.Wrap, "square")
		}
	})

	t.Run("preserves existing wrap=square", func(t *testing.T) {
		shape := &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Wrap: "square"},
			},
		}
		enforceTextWrap(shape)
		if shape.TextBody.BodyProperties.Wrap != "square" {
			t.Errorf("Wrap = %q, want %q", shape.TextBody.BodyProperties.Wrap, "square")
		}
	})

	t.Run("respects existing wrap=none", func(t *testing.T) {
		shape := &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Wrap: "none"},
			},
		}
		enforceTextWrap(shape)
		if shape.TextBody.BodyProperties.Wrap != "none" {
			t.Errorf("Wrap = %q, want %q (should not override explicit wrap=none)", shape.TextBody.BodyProperties.Wrap, "none")
		}
	})
}

// TestBodyPropertiesXML_AttributePreservation verifies that bodyPr attributes
// survive XML round-tripping. Before the fix for tfk1, only innerxml was
// preserved, losing attributes like wrap, anchor, lIns, rIns, etc.
func TestBodyPropertiesXML_AttributePreservation(t *testing.T) {
	t.Run("wrap and anchor preserved", func(t *testing.T) {
		bp := bodyPropertiesXML{
			Wrap:   "square",
			Anchor: "ctr",
			Inner:  `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`,
		}
		if bp.Wrap != "square" {
			t.Errorf("Wrap = %q, want %q", bp.Wrap, "square")
		}
		if bp.Anchor != "ctr" {
			t.Errorf("Anchor = %q, want %q", bp.Anchor, "ctr")
		}
	})

	t.Run("insets preserved", func(t *testing.T) {
		var zero int64
		bp := bodyPropertiesXML{
			LIns: &zero,
			RIns: &zero,
			TIns: &zero,
			BIns: &zero,
		}
		if bp.LIns == nil || *bp.LIns != 0 {
			t.Errorf("LIns = %v, want 0", bp.LIns)
		}
		if bp.RIns == nil || *bp.RIns != 0 {
			t.Errorf("RIns = %v, want 0", bp.RIns)
		}
	})

	t.Run("nil insets omitted", func(t *testing.T) {
		bp := bodyPropertiesXML{Wrap: "square"}
		if bp.LIns != nil {
			t.Errorf("LIns should be nil when not set, got %v", bp.LIns)
		}
	})
}

func TestCenterIfSparse(t *testing.T) {
	makeShape := func(anchor string, inner string, paraCount int) *shapeXML {
		paras := make([]paragraphXML, paraCount)
		for i := range paras {
			paras[i] = paragraphXML{Runs: []runXML{{Text: "bullet"}}}
		}
		return &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{
					Anchor: anchor,
					Inner:  inner,
				},
				Paragraphs: paras,
			},
		}
	}

	t.Run("sparse bullets get centered", func(t *testing.T) {
		shape := makeShape("t", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`, 3)
		centerIfSparse(shape, 3)
		if shape.TextBody.BodyProperties.Anchor != "ctr" {
			t.Errorf("Anchor = %q, want %q", shape.TextBody.BodyProperties.Anchor, "ctr")
		}
	})

	t.Run("dense bullets stay top-aligned", func(t *testing.T) {
		shape := makeShape("t", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`, 12)
		centerIfSparse(shape, 12)
		if shape.TextBody.BodyProperties.Anchor != "t" {
			t.Errorf("Anchor = %q, want %q", shape.TextBody.BodyProperties.Anchor, "t")
		}
	})

	t.Run("scaled-down content stays top-aligned", func(t *testing.T) {
		shape := makeShape("t", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" fontScale="60000"/>`, 5)
		centerIfSparse(shape, 5)
		if shape.TextBody.BodyProperties.Anchor != "t" {
			t.Errorf("Anchor = %q, want %q (fontScale=60000 means dense)", shape.TextBody.BodyProperties.Anchor, "t")
		}
	})

	t.Run("lightly scaled content gets centered", func(t *testing.T) {
		shape := makeShape("t", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" fontScale="90000"/>`, 5)
		centerIfSparse(shape, 5)
		if shape.TextBody.BodyProperties.Anchor != "ctr" {
			t.Errorf("Anchor = %q, want %q (fontScale=90000 is light)", shape.TextBody.BodyProperties.Anchor, "ctr")
		}
	})

	t.Run("already centered stays centered", func(t *testing.T) {
		shape := makeShape("ctr", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`, 3)
		centerIfSparse(shape, 3)
		if shape.TextBody.BodyProperties.Anchor != "ctr" {
			t.Errorf("Anchor = %q, want %q", shape.TextBody.BodyProperties.Anchor, "ctr")
		}
	})

	t.Run("bottom-aligned stays bottom", func(t *testing.T) {
		shape := makeShape("b", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`, 3)
		centerIfSparse(shape, 3)
		if shape.TextBody.BodyProperties.Anchor != "b" {
			t.Errorf("Anchor = %q, want %q", shape.TextBody.BodyProperties.Anchor, "b")
		}
	})

	t.Run("nil body properties is safe", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{}}
		centerIfSparse(shape, 3) // should not panic
	})

	t.Run("boundary at 8 paragraphs centers", func(t *testing.T) {
		shape := makeShape("t", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`, 8)
		centerIfSparse(shape, 8)
		if shape.TextBody.BodyProperties.Anchor != "ctr" {
			t.Errorf("Anchor = %q, want %q", shape.TextBody.BodyProperties.Anchor, "ctr")
		}
	})

	t.Run("9 paragraphs stays top-aligned", func(t *testing.T) {
		shape := makeShape("t", `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`, 9)
		centerIfSparse(shape, 9)
		if shape.TextBody.BodyProperties.Anchor != "t" {
			t.Errorf("Anchor = %q, want %q", shape.TextBody.BodyProperties.Anchor, "t")
		}
	})
}

// TestTwoColumnOverflow verifies that text in a narrow left-column placeholder
// (as in a two-column layout) gets wrap="square" enforced on bodyPr and
// autofit scaling applied to prevent horizontal overflow into the adjacent column.
func TestTwoColumnOverflow(t *testing.T) {
	// Simulate a two-column layout: left column is ~5.67 inches wide (5181600 EMU).
	// This is half of a standard 10-inch slide width.
	const leftColumnWidthEMU = 5181600
	const placeholderHeightEMU = 4351338

	// Create a shape mimicking the left-column placeholder from a "Two Content" layout
	makeTwoColumnShape := func() *shapeXML {
		return &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset: offsetXML{X: 838200, Y: 1825625},
					Extent: extentXML{CX: leftColumnWidthEMU, CY: placeholderHeightEMU},
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{}, // Empty like templates
				ListStyle:      &listStyleXML{},
				Paragraphs:     []paragraphXML{},
			},
		}
	}

	t.Run("bullets enforce wrap=square", func(t *testing.T) {
		shape := makeTwoColumnShape()
		// Populate with long bullet content that could overflow a narrow column
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Content 1",
			Type:          ContentBullets,
			Value: []string{
				"Revenue exceeded all quarterly targets with significant year-over-year growth across all business segments",
				"Customer acquisition costs decreased while lifetime value increased substantially",
				"Market share expanded in three key regions despite increased competitive pressure",
				"Strategic partnerships delivered measurable pipeline contributions in enterprise segment",
				"Product adoption metrics show strong engagement with recently launched features and platform",
			},
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Verify wrap="square" is set on bodyPr
		if shape.TextBody.BodyProperties.Wrap != "square" {
			t.Errorf("bodyPr wrap = %q, want %q (text must be contained within placeholder bounds)",
				shape.TextBody.BodyProperties.Wrap, "square")
		}

		// Verify normAutofit is applied (text may need scaling to fit)
		if !strings.Contains(shape.TextBody.BodyProperties.Inner, "normAutofit") {
			t.Error("bodyPr should contain normAutofit for dense bullet content in narrow column")
		}
	})

	t.Run("body_and_bullets enforce wrap=square", func(t *testing.T) {
		shape := makeTwoColumnShape()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Content 1",
			Type:          ContentBodyAndBullets,
			Value: BodyAndBulletsContent{
				Body: "Drivers of Growth in the current fiscal year have been remarkable across all dimensions",
				Bullets: []string{
					"Revenue exceeded quarterly targets with year-over-year growth across all business segments",
					"Customer acquisition costs decreased while lifetime value increased substantially",
					"Market share expanded in three key geographic regions despite competitive pressure",
				},
			},
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		if shape.TextBody.BodyProperties.Wrap != "square" {
			t.Errorf("bodyPr wrap = %q, want %q", shape.TextBody.BodyProperties.Wrap, "square")
		}
	})

	t.Run("text content enforces wrap=square", func(t *testing.T) {
		shape := makeTwoColumnShape()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Content 1",
			Type:          ContentText,
			Value:         "This is a very long paragraph that describes the drivers of growth in detail, covering revenue, customer acquisition, market share expansion, and strategic partnerships that delivered measurable results.",
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		if shape.TextBody.BodyProperties.Wrap != "square" {
			t.Errorf("bodyPr wrap = %q, want %q", shape.TextBody.BodyProperties.Wrap, "square")
		}
	})

	t.Run("bullet_groups enforce wrap=square", func(t *testing.T) {
		shape := makeTwoColumnShape()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Content 1",
			Type:          ContentBulletGroups,
			Value: BulletGroupsContent{
				Body: "Key performance indicators across business segments",
				Groups: []BulletGroup{
					{
						Header:  "Revenue Performance",
						Bullets: []string{"Exceeded targets by 15%", "Year-over-year growth of 23%"},
					},
					{
						Header:  "Customer Metrics",
						Bullets: []string{"Acquisition cost reduced 20%", "Retention rate improved to 94%"},
					},
				},
			},
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		if shape.TextBody.BodyProperties.Wrap != "square" {
			t.Errorf("bodyPr wrap = %q, want %q", shape.TextBody.BodyProperties.Wrap, "square")
		}
	})
}

func TestReplaceSpAutoFitWithNorm(t *testing.T) {
	t.Run("nil TextBody is no-op", func(t *testing.T) {
		shape := &shapeXML{}
		replaceSpAutoFitWithNorm(shape) // should not panic
	})

	t.Run("nil BodyProperties is no-op", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{}}
		replaceSpAutoFitWithNorm(shape) // should not panic
	})

	t.Run("empty Inner is no-op", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{BodyProperties: &bodyPropertiesXML{}}}
		replaceSpAutoFitWithNorm(shape)
		if shape.TextBody.BodyProperties.Inner != "" {
			t.Errorf("Inner should remain empty, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})

	t.Run("strips a:spAutoFit", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{
			BodyProperties: &bodyPropertiesXML{Inner: `<a:spAutoFit/>`},
		}}
		replaceSpAutoFitWithNorm(shape)
		if strings.Contains(shape.TextBody.BodyProperties.Inner, "spAutoFit") {
			t.Errorf("spAutoFit should be stripped, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})

	t.Run("strips spAutoFit without namespace prefix", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{
			BodyProperties: &bodyPropertiesXML{Inner: `<spAutoFit/>`},
		}}
		replaceSpAutoFitWithNorm(shape)
		if strings.Contains(shape.TextBody.BodyProperties.Inner, "spAutoFit") {
			t.Errorf("spAutoFit should be stripped, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})

	t.Run("strips spAutoFit with namespace URI", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{
			BodyProperties: &bodyPropertiesXML{Inner: `<a:spAutoFit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`},
		}}
		replaceSpAutoFitWithNorm(shape)
		if strings.Contains(shape.TextBody.BodyProperties.Inner, "spAutoFit") {
			t.Errorf("spAutoFit should be stripped, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})

	t.Run("preserves normAutofit", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{
			BodyProperties: &bodyPropertiesXML{Inner: `<a:normAutofit/>`},
		}}
		replaceSpAutoFitWithNorm(shape)
		if shape.TextBody.BodyProperties.Inner != `<a:normAutofit/>` {
			t.Errorf("normAutofit should be preserved, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})

	t.Run("preserves noAutofit", func(t *testing.T) {
		shape := &shapeXML{TextBody: &textBodyXML{
			BodyProperties: &bodyPropertiesXML{Inner: `<a:noAutofit/>`},
		}}
		replaceSpAutoFitWithNorm(shape)
		if shape.TextBody.BodyProperties.Inner != `<a:noAutofit/>` {
			t.Errorf("noAutofit should be preserved, got %q", shape.TextBody.BodyProperties.Inner)
		}
	})
}

// TestSectionHeaderOverflow verifies that long body text on a section divider layout
// "Text Placeholder 2" (which has sz="35000" / 350pt and spAutoFit in the layout) gets
// proper autofit treatment: the font is capped, spAutoFit is replaced, and normAutofit
// is applied so text shrinks to fit within the placeholder bounds.
func TestSectionHeaderOverflow(t *testing.T) {
	// Simulate the shape as it appears in a section divider layout:
	// - "Text Placeholder 2" body placeholder with 350pt font in lstStyle
	// - spAutoFit in bodyPr (grow box to fit text)
	// - Dimensions: 3462095 x 4308872 EMU (the actual placeholder bounds)
	makeSectionHeaderShape := func() *shapeXML {
		return &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset: offsetXML{X: 8193823, Y: 572408},
					Extent: extentXML{CX: 3462095, CY: 4308872},
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{
					Wrap:  "square",
					Inner: `<a:spAutoFit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`,
				},
				ListStyle: &listStyleXML{
					Inner: `<a:lvl1pPr marL="0" indent="0" algn="r"><a:lnSpc><a:spcPct val="80000"/></a:lnSpc><a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="0"/></a:spcAft><a:buNone/><a:defRPr sz="35000" b="0" kern="100" spc="-50" baseline="0"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl1pPr>`,
				},
				Paragraphs: []paragraphXML{
					{
						Properties: &paragraphPropertiesXML{Level: bulletIntPtr(0)},
						Runs:       []runXML{{RunProperties: &runPropertiesXML{Lang: "en-US"}, Text: "0"}},
					},
				},
			},
		}
	}

	t.Run("text content gets normAutofit not spAutoFit", func(t *testing.T) {
		shape := makeSectionHeaderShape()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Text Placeholder 2",
			Type:          ContentText,
			Value:         "Transforming the business through digital innovation and strategic partnerships across global markets",
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		bp := shape.TextBody.BodyProperties

		// spAutoFit must be stripped
		if strings.Contains(bp.Inner, "spAutoFit") {
			t.Error("bodyPr should NOT contain spAutoFit after text population (it causes overflow)")
		}

		// normAutofit must be applied
		if !strings.Contains(bp.Inner, "normAutofit") {
			t.Error("bodyPr should contain normAutofit to shrink text to fit placeholder")
		}

		// Font size in lstStyle must be capped (35000 -> 4800)
		lstSz := parseSzAttr(shape.TextBody.ListStyle.Inner)
		if lstSz > 4800 {
			t.Errorf("lstStyle font size should be capped to 4800, got %d", lstSz)
		}
	})

	t.Run("bullets content gets normAutofit not spAutoFit", func(t *testing.T) {
		shape := makeSectionHeaderShape()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Text Placeholder 2",
			Type:          ContentBullets,
			Value: []string{
				"Revenue exceeded quarterly targets with year-over-year growth",
				"Customer acquisition costs decreased substantially",
				"Market share expanded in three key geographic regions",
			},
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		bp := shape.TextBody.BodyProperties

		if strings.Contains(bp.Inner, "spAutoFit") {
			t.Error("bodyPr should NOT contain spAutoFit after bullet population")
		}
		if !strings.Contains(bp.Inner, "normAutofit") {
			t.Error("bodyPr should contain normAutofit to shrink text to fit placeholder")
		}
	})

	t.Run("section title caps font for placeholder width", func(t *testing.T) {
		shape := makeSectionHeaderShape()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Text Placeholder 2",
			Type:          ContentSectionTitle,
			Value:         "Performance Overview",
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Section title should be capped to a size where each word fits on one
		// line within the placeholder width. The original 350pt is too large for
		// the ~272pt-wide placeholder, causing character-level word wrapping.
		lstSz := parseSzAttr(shape.TextBody.ListStyle.Inner)
		if lstSz >= 35000 {
			t.Errorf("section title lstStyle font should be capped below 35000, got %d", lstSz)
		}
		// Should still be larger than the 24pt body text cap
		if lstSz < 2400 {
			t.Errorf("section title lstStyle font should be >2400 (larger than body cap), got %d", lstSz)
		}

		bp := shape.TextBody.BodyProperties
		if !strings.Contains(bp.Inner, "normAutofit") {
			t.Error("bodyPr should contain normAutofit for section title")
		}
		if strings.Contains(bp.Inner, "spAutoFit") {
			t.Error("bodyPr should NOT contain spAutoFit for section title")
		}
	})

	t.Run("section title short word preserves larger font", func(t *testing.T) {
		shape := makeSectionHeaderShape()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "Text Placeholder 2",
			Type:          ContentSectionTitle,
			Value:         "Risks",
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Short single word "Risks" should allow a much larger font than
		// "Performance Overview" since the longest word is shorter.
		lstSz := parseSzAttr(shape.TextBody.ListStyle.Inner)
		// "Risks" (5 chars) should fit at a much larger font than "Performance" (11 chars)
		if lstSz < 4000 {
			t.Errorf("short section title font should be >=4000 (40pt+), got %d", lstSz)
		}
	})
}

func TestBoostSectionTitleFont(t *testing.T) {
	t.Run("small font boosted to height-based minimum", func(t *testing.T) {
		// Simulate a section divider title placeholder with a small font (36pt)
		// in a tall placeholder (269pt / 3417887 EMU).
		shape := &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset: offsetXML{X: 838200, Y: 1268413},
					Extent: extentXML{CX: 6400800, CY: 3417887},
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Wrap: "square"},
				ListStyle: &listStyleXML{
					Inner: `<a:lvl1pPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:buNone/><a:defRPr sz="3600"/></a:lvl1pPr>`,
				},
				Paragraphs: []paragraphXML{
					{Runs: []runXML{{Text: "Implementation"}}},
				},
			},
		}

		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "title",
			Type:          ContentSectionTitle,
			Value:         "Implementation",
		}, -1, "")
		if err != nil {
			t.Fatal(err)
		}

		sz := parseSzAttr(shape.TextBody.ListStyle.Inner)
		// Height 269pt → minFont = 269*0.5/2.4*100 ≈ 5608, clamped to 5400
		// Original 3600 < 5400, so should be boosted
		if sz < 4000 {
			t.Errorf("section title font should be boosted above 4000 hpt, got %d", sz)
		}
	})

	t.Run("already large font not reduced", func(t *testing.T) {
		shape := &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset: offsetXML{X: 0, Y: 0},
					Extent: extentXML{CX: 9145991, CY: 3630384},
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{Wrap: "square"},
				ListStyle: &listStyleXML{
					Inner: `<a:lvl1pPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:defRPr sz="6500"/></a:lvl1pPr>`,
				},
				Paragraphs: []paragraphXML{
					{Runs: []runXML{{Text: "Summary"}}},
				},
			},
		}

		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "title",
			Type:          ContentSectionTitle,
			Value:         "Summary",
		}, -1, "")
		if err != nil {
			t.Fatal(err)
		}

		sz := parseSzAttr(shape.TextBody.ListStyle.Inner)
		if sz < 5400 {
			t.Errorf("large section title font should be preserved (>=5400), got %d", sz)
		}
	})
}

func TestMinSectionTitleFontForHeight(t *testing.T) {
	tests := []struct {
		name      string
		heightEMU int64
		wantMin   int
		wantMax   int
	}{
		{"small placeholder", 1270000, 3200, 5400},  // 100pt height
		{"medium placeholder", 3417887, 3200, 5400},  // 269pt height
		{"large placeholder", 6350000, 3200, 5400},   // 500pt height → capped at 5400
		{"zero height", 0, 3200, 3200},                // default floor
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minSectionTitleFontForHeight(tt.heightEMU)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("minSectionTitleFontForHeight(%d) = %d, want [%d, %d]",
					tt.heightEMU, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestFindMasterPathFromSyntheticRels(t *testing.T) {
	t.Run("nil map returns empty", func(t *testing.T) {
		if got := findMasterPathFromSyntheticRels(nil, "slideLayout99"); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("missing rels returns empty", func(t *testing.T) {
		files := map[string][]byte{
			"ppt/slideLayouts/slideLayout99.xml": []byte("<xml/>"),
		}
		if got := findMasterPathFromSyntheticRels(files, "slideLayout99"); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("valid rels returns master path", func(t *testing.T) {
		relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`
		files := map[string][]byte{
			"ppt/slideLayouts/_rels/slideLayout99.xml.rels": []byte(relsXML),
		}
		got := findMasterPathFromSyntheticRels(files, "slideLayout99")
		want := "ppt/slideMasters/slideMaster1.xml"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestDenseBulletAutoScaling verifies that very dense bullet lists (12+)
// use more aggressive auto-scaling to prevent overflow. Regression test
// for pptx-t20 slide 63 (12 bullets compressed into small area).
func TestDenseBulletAutoScaling(t *testing.T) {
	// Create a constrained body placeholder (mimics a constrained body area)
	makeConstrainedShape := func() *shapeXML {
		return &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset: offsetXML{X: 305816, Y: 1200000},
					Extent: extentXML{CX: 9000000, CY: 4500000}, // ~3.5 inches tall
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
				ListStyle: &listStyleXML{
					Inner: `<a:lvl1pPr><a:defRPr sz="2000"/></a:lvl1pPr>`,
				},
				Paragraphs: []paragraphXML{},
			},
		}
	}

	t.Run("12 bullets uses aggressive font scale", func(t *testing.T) {
		shape := makeConstrainedShape()
		bullets := make([]string, 12)
		for i := range bullets {
			bullets[i] = "Quarterly revenue exceeded targets with significant growth across all segments"
		}

		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "body",
			Type:          ContentBullets,
			Value:         bullets,
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		bp := shape.TextBody.BodyProperties
		if !strings.Contains(bp.Inner, "normAutofit") {
			t.Error("bodyPr should contain normAutofit for dense bullet content")
		}

		// The lstStyle floor should be 10pt (1000) for 12+ bullets, not 12pt (1200)
		lstSz := parseSzAttr(shape.TextBody.ListStyle.Inner)
		if lstSz > 2000 {
			t.Errorf("lstStyle font should be capped at 2000 (20pt), got %d", lstSz)
		}
		if lstSz < 1000 {
			t.Errorf("lstStyle font should be floored at 1000 (10pt) for dense bullets, got %d", lstSz)
		}
	})

	t.Run("8 bullets uses standard font floor", func(t *testing.T) {
		shape := makeConstrainedShape()
		// Set lstStyle to a small font that would be raised by the 12pt floor
		shape.TextBody.ListStyle.Inner = `<a:lvl1pPr><a:defRPr sz="900"/></a:lvl1pPr>`

		bullets := make([]string, 8)
		for i := range bullets {
			bullets[i] = "Standard bullet point"
		}

		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "body",
			Type:          ContentBullets,
			Value:         bullets,
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// With 8 bullets (< 12), the 12pt floor should apply
		lstSz := parseSzAttr(shape.TextBody.ListStyle.Inner)
		if lstSz < 1200 {
			t.Errorf("lstStyle font should be floored at 1200 (12pt) for non-dense bullets, got %d", lstSz)
		}
	})
}

func TestBulletGroupsIndentHierarchy(t *testing.T) {
	// Regression test: section headers (marL=0) must be at a shallower indent
	// than sub-bullets so the visual hierarchy is preserved (pptx-ioz).
	makeShape := func() *shapeXML {
		return &shapeXML{
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
				Paragraphs:     []paragraphXML{{}},
			},
		}
	}

	t.Run("sub-bullets use deeper level than headers", func(t *testing.T) {
		shape := makeShape()
		content := BulletGroupsContent{
			Groups: []BulletGroup{
				{
					Header:  "Overview",
					Bullets: []string{"First item", "Second item"},
				},
			},
		}

		err := setBulletGroupsParagraphs(shape, "body", content, -1)
		if err != nil {
			t.Fatalf("setBulletGroupsParagraphs() error = %v", err)
		}

		// Expect: 1 header paragraph + 2 bullet paragraphs = 3
		if len(shape.TextBody.Paragraphs) != 3 {
			t.Fatalf("expected 3 paragraphs, got %d", len(shape.TextBody.Paragraphs))
		}

		headerP := shape.TextBody.Paragraphs[0]
		bulletP := shape.TextBody.Paragraphs[1]

		// Header should have marL=0 (noBulletParagraphProps) with buNone
		if headerP.Properties == nil || headerP.Properties.MarL == nil || *headerP.Properties.MarL != 0 {
			t.Error("header paragraph should have marL=0")
		}
		if !strings.Contains(headerP.Properties.Inner, "buNone") {
			t.Error("header paragraph should contain buNone")
		}

		// Sub-bullet should have Level > 0 (deeper than default level 0)
		if bulletP.Properties == nil || bulletP.Properties.Level == nil {
			t.Fatal("bullet paragraph should have Level set")
		}
		if *bulletP.Properties.Level < 1 {
			t.Errorf("bullet level should be >= 1 for sub-bullets, got %d", *bulletP.Properties.Level)
		}
	})

	t.Run("sub-bullets get fallback marL when template has zero margin", func(t *testing.T) {
		zeroMarL := 0
		lvl := 0
		shape := makeShape()
		// Template with explicit marL=0 at level 0, with bullet inner content
		// so extractBulletTemplateStyles picks up the pProps.
		shape.TextBody.Paragraphs = []paragraphXML{
			{
				Properties: &paragraphPropertiesXML{
					Level: &lvl,
					MarL:  &zeroMarL,
					Inner: `<a:buChar char="•"/>`,
				},
			},
		}

		content := BulletGroupsContent{
			Groups: []BulletGroup{
				{
					Header:  "Key Metrics",
					Bullets: []string{"Revenue up 15%"},
				},
			},
		}

		err := setBulletGroupsParagraphs(shape, "body", content, 0)
		if err != nil {
			t.Fatalf("setBulletGroupsParagraphs() error = %v", err)
		}

		// Find the bullet paragraph (after header)
		if len(shape.TextBody.Paragraphs) < 2 {
			t.Fatalf("expected at least 2 paragraphs, got %d", len(shape.TextBody.Paragraphs))
		}
		bulletP := shape.TextBody.Paragraphs[1]

		// Sub-bullet should have a non-zero marL from the fallback
		if bulletP.Properties == nil || bulletP.Properties.MarL == nil {
			t.Fatal("bullet paragraph should have MarL set")
		}
		if *bulletP.Properties.MarL == 0 {
			t.Error("bullet marL should not be 0 when template has zero margin — fallback should apply")
		}
		if *bulletP.Properties.MarL != 360000 {
			t.Errorf("expected fallback marL=360000, got %d", *bulletP.Properties.MarL)
		}
	})
}

func TestIsTitlePlaceholder(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"title", true},
		{"title_1", true},
		{"title_2", true},
		{"Title", true},
		{"TITLE", true},
		{"Title_1", true},
		{"body", false},
		{"subtitle", false},
		{"body_2", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isTitlePlaceholder(tt.id); got != tt.want {
			t.Errorf("isTitlePlaceholder(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestEstimateWordWrapLines(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		charsPerLine int
		want         int
	}{
		{"empty", "", 40, 1},
		{"short text", "Hello", 40, 1},
		{"exactly fits", "Hello World", 11, 1},
		{"wraps to 2", "Hello World", 6, 2},
		{"wraps to 3", "This is a somewhat longer text", 12, 3},
		{"single long word", "Supercalifragilistic", 10, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateWordWrapLines(tt.text, tt.charsPerLine)
			if got != tt.want {
				t.Errorf("estimateWordWrapLines(%q, %d) = %d, want %d",
					tt.text, tt.charsPerLine, got, tt.want)
			}
		})
	}
}

func TestTruncateTextToMaxLines(t *testing.T) {
	// Use a placeholder width of ~8 inches (7315200 EMU) and 20pt font (2000 hPt).
	const widthEMU int64 = 7315200 // ~8 inches
	const fontHPt = 2000           // 20pt

	t.Run("short title unchanged", func(t *testing.T) {
		text := "Quarterly Revenue Overview"
		got := truncateTextToMaxLines(text, widthEMU, fontHPt, 3, "Arial")
		if got != text {
			t.Errorf("short title should not be truncated, got %q", got)
		}
	})

	t.Run("very long title truncated with ellipsis", func(t *testing.T) {
		// Build a title long enough to exceed 3 lines at 20pt in ~8 inches.
		// With real font metrics, Arial 20pt in 8" handles ~140 chars per line,
		// so we need >420 chars to exceed 3 wrapped lines.
		text := strings.Repeat("Comprehensive Analysis of Global Market Trends and Revenue Growth Patterns Across All Business Segments ", 5)
		text = strings.TrimSpace(text)
		got := truncateTextToMaxLines(text, widthEMU, fontHPt, 3, "Arial")
		if got == text {
			t.Error("long title should have been truncated")
		}
		if !strings.HasSuffix(got, "\u2026") {
			t.Errorf("truncated title should end with ellipsis, got %q", got)
		}
		// Verify the truncated text is shorter
		if len(got) >= len(text) {
			t.Errorf("truncated text should be shorter: got %d chars, original %d",
				len(got), len(text))
		}
		// Verify it fits within 3 lines using MeasureRun
		m, err := textfit.MeasureRun(got, "Arial", float64(fontHPt)/100.0, widthEMU, 3)
		if err != nil {
			t.Fatalf("MeasureRun failed: %v", err)
		}
		if !m.Fits {
			t.Errorf("truncated title does not fit within 3 lines (measured %d lines)", m.Lines)
		}
	})

	t.Run("zero width returns original", func(t *testing.T) {
		text := "Some Title"
		got := truncateTextToMaxLines(text, 0, fontHPt, 3, "Arial")
		if got != text {
			t.Errorf("zero width should return original, got %q", got)
		}
	})

	t.Run("zero font size returns original", func(t *testing.T) {
		text := "Some Title"
		got := truncateTextToMaxLines(text, widthEMU, 0, 3, "Arial")
		if got != text {
			t.Errorf("zero font size should return original, got %q", got)
		}
	})

	t.Run("fallback heuristic when font unavailable", func(t *testing.T) {
		// Empty fontName triggers MeasureRun fallback (ErrNoFontCache) →
		// falls through to the avgCharRatio heuristic. The function should
		// still truncate long text rather than panicking.
		text := "Comprehensive Analysis of Global Market Trends and Revenue Growth Patterns Across All Business Segments for the Current Fiscal Year Including Detailed Regional Breakdowns"
		got := truncateTextToMaxLines(text, widthEMU, fontHPt, 3, "")
		// Should either truncate or return unchanged — must not panic.
		if got == "" {
			t.Error("fallback should not return empty string")
		}
	})
}

// TestLongTitleTruncation verifies that populateShapeText truncates extremely
// long titles to prevent crowding of body text (regression test for pptx-4pb).
func TestLongTitleTruncation(t *testing.T) {
	makeTitleShape := func() *shapeXML {
		return &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset: offsetXML{X: 457200, Y: 274638},
					Extent: extentXML{CX: 7315200, CY: 1143000}, // ~8" wide × ~0.9" tall
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
				ListStyle: &listStyleXML{
					Inner: `<a:lvl1pPr><a:defRPr sz="2000"/></a:lvl1pPr>`,
				},
				Paragraphs: []paragraphXML{{}},
			},
		}
	}

	t.Run("short title preserved", func(t *testing.T) {
		shape := makeTitleShape()
		title := "Revenue Overview"
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "title",
			Type:          ContentText,
			Value:         title,
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Short title should be preserved unchanged
		var text string
		for _, para := range shape.TextBody.Paragraphs {
			for _, run := range para.Runs {
				text += run.Text
			}
		}
		if text != title {
			t.Errorf("short title should be preserved, got %q", text)
		}
	})

	t.Run("extremely long title truncated with ellipsis", func(t *testing.T) {
		shape := makeTitleShape()
		longTitle := strings.Repeat("Comprehensive Analysis of Global Market Trends and Revenue Growth Patterns Across All Business Segments ", 5)
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "title",
			Type:          ContentText,
			Value:         longTitle,
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Extract rendered text
		var text string
		for _, para := range shape.TextBody.Paragraphs {
			for _, run := range para.Runs {
				text += run.Text
			}
		}
		// Should be truncated and end with ellipsis
		if !strings.Contains(text, "\u2026") {
			t.Errorf("long title should contain ellipsis, got %q", text)
		}
		if len(text) >= len(longTitle) {
			t.Errorf("truncated text should be shorter: got %d chars, original %d",
				len(text), len(longTitle))
		}
	})

	t.Run("body text not truncated regardless of length", func(t *testing.T) {
		shape := makeTitleShape()
		longBody := "Comprehensive analysis of global market trends and revenue growth patterns across all business segments for the current fiscal year including detailed regional breakdowns and forward looking projections"
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "body",
			Type:          ContentText,
			Value:         longBody,
		}, 0, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Body text should NOT be truncated (only titles get line-count limits)
		var text string
		for _, para := range shape.TextBody.Paragraphs {
			for _, run := range para.Runs {
				text += run.Text
			}
		}
		if strings.Contains(text, "\u2026") {
			t.Errorf("body text should not be truncated with ellipsis, got %q", text)
		}
	})
}

// TestSectionTitlePreservesLstStyleAlignment verifies that section divider
// titles (ContentSectionTitle) do not force left alignment in inline paragraph
// properties, allowing the lstStyle alignment to prevail. This ensures templates
// that center section divider titles via lstStyle algn="ctr".
// Regression test for pptx-pbs.
func TestSectionTitlePreservesLstStyleAlignment(t *testing.T) {
	// Shape with lstStyle algn="ctr" (simulating section divider title with centered alignment)
	makeCenteredSectionTitle := func() *shapeXML {
		return &shapeXML{
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset: offsetXML{X: 305816, Y: 1885212},
					Extent: extentXML{CX: 6121971, CY: 4749009},
				},
			},
			TextBody: &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
				ListStyle: &listStyleXML{
					Inner: `<a:lvl1pPr algn="ctr"><a:defRPr sz="4800" b="1"/></a:lvl1pPr>`,
				},
				Paragraphs: []paragraphXML{
					{
						Runs: []runXML{{RunProperties: &runPropertiesXML{Lang: "de-DE"}, Text: "Kapiteltrennfolie"}},
					},
				},
			},
		}
	}

	t.Run("section title does not override lstStyle center alignment", func(t *testing.T) {
		shape := makeCenteredSectionTitle()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "title",
			Type:          ContentSectionTitle,
			Value:         "Strategy Overview",
		}, -1, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Inline paragraph properties must NOT have algn="l" which would
		// override the lstStyle's algn="ctr".
		for i, p := range shape.TextBody.Paragraphs {
			if p.Properties != nil && p.Properties.Algn == "l" {
				t.Errorf("paragraph %d has algn=%q; section titles should not force left alignment", i, p.Properties.Algn)
			}
		}
	})

	t.Run("regular body text still gets left alignment", func(t *testing.T) {
		shape := makeCenteredSectionTitle()
		err := populateShapeText(shape, ContentItem{
			PlaceholderID: "body",
			Type:          ContentText,
			Value:         "Some body text",
		}, -1, "")
		if err != nil {
			t.Fatalf("populateShapeText() error = %v", err)
		}

		// Regular body text (ContentText) should still get explicit algn="l"
		// to prevent inheriting unexpected alignment from the slide master.
		for _, p := range shape.TextBody.Paragraphs {
			if p.Properties == nil || p.Properties.Algn != "l" {
				algn := ""
				if p.Properties != nil {
					algn = p.Properties.Algn
				}
				t.Errorf("body text paragraph algn=%q; want %q", algn, "l")
			}
		}
	})
}
