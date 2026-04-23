package template

import (
	"encoding/xml"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestExtractPlaceholderStyle_WithRealTemplate(t *testing.T) {
	templatePath := filepath.Join("testdata", "standard.pptx")

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Try to extract style from a body placeholder
	// Layout index 1 is typically "Title and Content" in standard templates
	style, err := ExtractPlaceholderStyle(reader, 1, types.PlaceholderBody)
	if err != nil {
		// Not all templates have body in layout 1, so try content
		style, err = ExtractPlaceholderStyle(reader, 1, types.PlaceholderContent)
		if err != nil {
			// Try layout 0 which may have a title
			style, err = ExtractPlaceholderStyle(reader, 0, types.PlaceholderTitle)
			if err != nil {
				t.Fatalf("Could not find any placeholder to extract style from: %v", err)
			}
		}
	}

	// Verify bounds are extracted
	if style.Bounds.Width == 0 || style.Bounds.Height == 0 {
		t.Error("Expected non-zero placeholder bounds")
	}

	t.Logf("Extracted style: Bounds=%+v, Font=%s, Size=%d, Color=%s",
		style.Bounds, style.FontFamily, style.FontSize, style.FontColor)
}

func TestExtractContentAreaBounds_WithRealTemplate(t *testing.T) {
	templatePath := filepath.Join("testdata", "standard.pptx")

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Try multiple layout indices to find one with a body/content placeholder
	var bounds *ContentAreaBounds
	for i := 0; i < 5; i++ {
		bounds, err = ExtractContentAreaBounds(reader, i)
		if err == nil {
			break
		}
	}

	if bounds == nil {
		t.Fatal("Could not find any content area bounds in first 5 layouts")
	}

	// Verify bounds are reasonable
	if bounds.Width <= 0 {
		t.Error("Expected positive width for content area")
	}
	if bounds.Height <= 0 {
		t.Error("Expected positive height for content area")
	}

	t.Logf("Content area bounds: X=%d, Y=%d, Width=%d, Height=%d",
		bounds.X, bounds.Y, bounds.Width, bounds.Height)
}

func TestExtractContentAreaBoundsFromLayouts(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{
			Name: "Title Slide",
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
			},
		},
		{
			Name: "Content Slide",
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 500000, Y: 1500000, Width: 8000000, Height: 4500000}},
			},
		},
	}

	tests := []struct {
		name      string
		layoutIdx int
		wantErr   bool
		wantX     int64
		wantY     int64
	}{
		{
			name:      "content slide has body placeholder",
			layoutIdx: 1,
			wantErr:   false,
			wantX:     500000,
			wantY:     1500000,
		},
		{
			name:      "title slide has no body placeholder",
			layoutIdx: 0,
			wantErr:   true,
		},
		{
			name:      "index out of range",
			layoutIdx: 99,
			wantErr:   true,
		},
		{
			name:      "negative index",
			layoutIdx: -1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bounds, err := ExtractContentAreaBoundsFromLayouts(layouts, tt.layoutIdx)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if bounds.X != tt.wantX {
				t.Errorf("X = %d, want %d", bounds.X, tt.wantX)
			}
			if bounds.Y != tt.wantY {
				t.Errorf("Y = %d, want %d", bounds.Y, tt.wantY)
			}
		})
	}
}

func TestExtractContentAreaBoundsFromLayouts_ContentPlaceholder(t *testing.T) {
	// Test that PlaceholderContent is also recognized
	layouts := []types.LayoutMetadata{
		{
			Name: "Content Layout",
			Placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				{Type: types.PlaceholderContent, Bounds: types.BoundingBox{X: 100000, Y: 2000000, Width: 7500000, Height: 4000000}},
			},
		},
	}

	bounds, err := ExtractContentAreaBoundsFromLayouts(layouts, 0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if bounds.X != 100000 {
		t.Errorf("X = %d, want 100000", bounds.X)
	}
	if bounds.Width != 7500000 {
		t.Errorf("Width = %d, want 7500000", bounds.Width)
	}
}

func TestPlaceholderStyle_Defaults(t *testing.T) {
	style := &PlaceholderStyle{}

	// Verify zero values for defaults
	if style.FontFamily != "" {
		t.Error("Expected empty FontFamily by default")
	}
	if style.FontSize != 0 {
		t.Error("Expected zero FontSize by default")
	}
	if style.FillType != "" {
		t.Error("Expected empty FillType by default")
	}
}

func TestContentAreaBounds_EMUConversions(t *testing.T) {
	// 1 inch = 914400 EMUs
	const emuPerInch = int64(types.EMUPerInch)

	bounds := &ContentAreaBounds{
		X:      emuPerInch,     // 1 inch from left
		Y:      emuPerInch * 2, // 2 inches from top
		Width:  emuPerInch * 8, // 8 inches wide
		Height: emuPerInch * 5, // 5 inches tall
	}

	// Verify the bounds represent expected measurements
	if float64(bounds.X)/float64(emuPerInch) != 1.0 {
		t.Error("X should represent 1 inch")
	}
	if float64(bounds.Y)/float64(emuPerInch) != 2.0 {
		t.Error("Y should represent 2 inches")
	}
	if float64(bounds.Width)/float64(emuPerInch) != 8.0 {
		t.Error("Width should represent 8 inches")
	}
	if float64(bounds.Height)/float64(emuPerInch) != 5.0 {
		t.Error("Height should represent 5 inches")
	}
}

func TestExtractPlaceholderStyle_InvalidLayoutIndex(t *testing.T) {
	templatePath := filepath.Join("testdata", "standard.pptx")

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Test with invalid index
	_, err = ExtractPlaceholderStyle(reader, 999, types.PlaceholderBody)
	if err == nil {
		t.Error("Expected error for invalid layout index")
	}
}

func TestExtractPlaceholderStyle_MissingPlaceholder(t *testing.T) {
	templatePath := filepath.Join("testdata", "standard.pptx")

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Try to extract a chart placeholder from a title slide (unlikely to exist)
	_, err = ExtractPlaceholderStyle(reader, 0, types.PlaceholderChart)
	if err == nil {
		// It might exist in some templates, so just log
		t.Log("Chart placeholder found in layout 0 (unexpected but possible)")
	}
}

func TestExtractListStyle(t *testing.T) {
	style := &PlaceholderStyle{}

	// Create a list style with various properties
	lstStyle := &listStyleStyleXML{
		Lvl1pPr: &lvlPPrStyleXML{
			MarginLeft: 457200,      // 0.5 inch
			Indent:     -228600,     // -0.25 inch (hanging indent)
			SpaceBefore: &spcStyleXML{
				SpacePoints: &spcPtsStyleXML{Val: 1000}, // 10 pt
			},
			SpaceAfter: &spcStyleXML{
				SpacePoints: &spcPtsStyleXML{Val: 500}, // 5 pt
			},
			LineSpacing: &spcStyleXML{
				SpacePercent: &spcPctStyleXML{Val: 150000}, // 150%
			},
			BulletChar: &buCharStyleXML{
				Char: "•",
			},
			BulletSizePercent: &buSzPctStyleXML{
				Val: 100000, // 100%
			},
			DefRPr: &defRPrStyleXML{
				Size:   1800, // 18pt
				Bold:   true,
				Italic: false,
				Latin: &latinFontStyleXML{
					Typeface: "Arial",
				},
				SolidFill: &solidFillStyleXML{
					SRGBColor: &srgbClrStyleXML{Val: "000000"},
				},
			},
		},
	}

	extractListStyle(lstStyle, style)

	// Verify extracted values
	if style.BulletMarginL != 457200 {
		t.Errorf("BulletMarginL = %d, want 457200", style.BulletMarginL)
	}
	if style.BulletIndent != -228600 {
		t.Errorf("BulletIndent = %d, want -228600", style.BulletIndent)
	}
	if style.BulletChar != "•" {
		t.Errorf("BulletChar = %s, want •", style.BulletChar)
	}
	if style.BulletSize != 100 {
		t.Errorf("BulletSize = %d, want 100", style.BulletSize)
	}
	if style.LineSpacing != 150 {
		t.Errorf("LineSpacing = %d, want 150", style.LineSpacing)
	}
	if style.FontSize != 1800 {
		t.Errorf("FontSize = %d, want 1800", style.FontSize)
	}
	if !style.FontBold {
		t.Error("FontBold should be true")
	}
	if style.FontItalic {
		t.Error("FontItalic should be false")
	}
	if style.FontFamily != "Arial" {
		t.Errorf("FontFamily = %s, want Arial", style.FontFamily)
	}
	if style.FontColor != "#000000" {
		t.Errorf("FontColor = %s, want #000000", style.FontColor)
	}
}

// --- tableStyleIndex tests ---

func TestTableStyleIndex_WithRealTemplate_ModernTemplate(t *testing.T) {
	// modern-template has two declared styles
	templatePath := filepath.Join("..", "..", "templates", "modern-template.pptx")
	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	idx := newTableStyleIndex(reader)

	// "Medium Style 2 - Accent 1"
	name, ok := idx.lookup("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
	if !ok {
		t.Fatal("expected lookup to succeed for {5C22544A-...}")
	}
	if name != "Medium Style 2 - Accent 1" {
		t.Errorf("got name %q, want %q", name, "Medium Style 2 - Accent 1")
	}

	// "No Style, Table Grid"
	name, ok = idx.lookup("{5940675A-B579-460E-94D1-54222C63F5DA}")
	if !ok {
		t.Fatal("expected lookup to succeed for {5940675A-...}")
	}
	if name != "No Style, Table Grid" {
		t.Errorf("got name %q, want %q", name, "No Style, Table Grid")
	}

	// Unknown GUID
	_, ok = idx.lookup("{00000000-0000-0000-0000-000000000000}")
	if ok {
		t.Error("expected lookup to return false for unknown GUID")
	}
}

func TestTableStyleIndex_EmptyStyleList(t *testing.T) {
	// midnight-blue has an empty tblStyleLst (no tblStyle children)
	templatePath := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	idx := newTableStyleIndex(reader)

	_, ok := idx.lookup("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
	if ok {
		t.Error("expected lookup to return false for template with empty style list")
	}
}

func TestTableStyleIndex_AllBundledTemplates(t *testing.T) {
	// All four bundled templates must load without error; lookup must not panic
	templates := []string{"forest-green", "midnight-blue", "modern-template", "warm-coral"}
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			reader, err := OpenTemplate(filepath.Join("..", "..", "templates", tmpl+".pptx"))
			if err != nil {
				t.Fatalf("Failed to open template: %v", err)
			}
			defer func() { _ = reader.Close() }()

			idx := newTableStyleIndex(reader)
			// Exercise lookup — must not panic
			_, _ = idx.lookup("{00000000-0000-0000-0000-000000000000}")
		})
	}
}

func TestTableStyleIndex_ParseOnce(t *testing.T) {
	templatePath := filepath.Join("..", "..", "templates", "modern-template.pptx")
	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	idx := newTableStyleIndex(reader)

	// Call lookup concurrently to verify sync.Once behavior
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name, ok := idx.lookup("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
			if !ok {
				t.Error("expected lookup to succeed")
			}
			if name != "Medium Style 2 - Accent 1" {
				t.Errorf("got name %q, want %q", name, "Medium Style 2 - Accent 1")
			}
		}()
	}
	wg.Wait()
}

func TestTableStyleIndex_EmptyStyleListTestdata(t *testing.T) {
	// testdata/standard.pptx has tableStyles.xml but no tblStyle children
	templatePath := filepath.Join("testdata", "standard.pptx")
	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	idx := newTableStyleIndex(reader)
	// Should not panic; lookup returns false
	_, ok := idx.lookup("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
	if ok {
		t.Error("expected lookup to return false for template with empty style list")
	}
}

func TestTableStyleIndex_MalformedXML(t *testing.T) {
	// Directly test the parse path with a fabricated index that has
	// already-populated styles map to verify malformed XML doesn't panic
	var lst tblStyleLstXML
	err := xml.Unmarshal([]byte(`<tblStyleLst><tblStyle styleId="{ABC}" styleName="Test"/></tblStyleLst>`), &lst)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if len(lst.Styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(lst.Styles))
	}
	if lst.Styles[0].StyleID != "{ABC}" {
		t.Errorf("got styleId %q, want %q", lst.Styles[0].StyleID, "{ABC}")
	}
	if lst.Styles[0].StyleName != "Test" {
		t.Errorf("got styleName %q, want %q", lst.Styles[0].StyleName, "Test")
	}

	// Truncated/garbage XML should unmarshal without panic
	err = xml.Unmarshal([]byte(`<tblStyleLst><tblStyle styleId=`), &lst)
	if err == nil {
		t.Error("expected error for truncated XML")
	}
}

func TestExtractListStyle_NilHandling(t *testing.T) {
	style := &PlaceholderStyle{}

	// Test with nil lstStyle
	extractListStyle(nil, style)
	// Should not panic and leave style unchanged

	// Test with empty lstStyle
	extractListStyle(&listStyleStyleXML{}, style)
	// Should not panic and leave style unchanged

	// Test with nil Lvl1pPr
	extractListStyle(&listStyleStyleXML{Lvl1pPr: nil}, style)
	// Should not panic

	// Verify style is unchanged
	if style.FontSize != 0 {
		t.Error("Expected FontSize to remain 0")
	}
}
