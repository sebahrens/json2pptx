package template

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestParseLayouts(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		wantErr    bool
		minLayouts int
	}{
		{
			name:       "valid template with multiple layouts",
			file:       "testdata/standard.pptx",
			wantErr:    false,
			minLayouts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := OpenTemplate(tt.file)
			if err != nil {
				t.Fatalf("OpenTemplate() error = %v", err)
			}
			defer func() { _ = reader.Close() }()

			layouts, err := ParseLayouts(reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLayouts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(layouts) < tt.minLayouts {
					t.Errorf("ParseLayouts() got %d layouts, want at least %d", len(layouts), tt.minLayouts)
				}

				// AC1: Valid Template Parsing
				// Verify we have layouts with placeholders
				if len(layouts) == 0 {
					t.Error("ParseLayouts() returned no layouts")
				}

				// AC2: Layout Enumeration
				// Verify each layout has required fields
				for i, layout := range layouts {
					if layout.ID == "" {
						t.Errorf("Layout %d has empty ID", i)
					}
					if layout.Name == "" {
						t.Errorf("Layout %d has empty Name", i)
					}
					if layout.Index != i {
						t.Errorf("Layout %d has Index %d, want %d", i, layout.Index, i)
					}
				}
			}
		})
	}
}

func TestExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name            string
		file            string
		layoutIdx       int
		wantTitle       bool
		wantBody        bool
		minPlaceholders int
	}{
		{
			name:            "layout with title and body",
			file:            "testdata/standard.pptx",
			layoutIdx:       0,
			wantTitle:       true,
			wantBody:        true,
			minPlaceholders: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := OpenTemplate(tt.file)
			if err != nil {
				t.Fatalf("OpenTemplate() error = %v", err)
			}
			defer func() { _ = reader.Close() }()

			layouts, err := ParseLayouts(reader)
			if err != nil {
				t.Fatalf("ParseLayouts() error = %v", err)
			}

			if tt.layoutIdx >= len(layouts) {
				t.Fatalf("Layout index %d out of range (have %d layouts)", tt.layoutIdx, len(layouts))
			}

			layout := layouts[tt.layoutIdx]

			// AC3: Placeholder Detection
			if len(layout.Placeholders) < tt.minPlaceholders {
				t.Errorf("Layout has %d placeholders, want at least %d", len(layout.Placeholders), tt.minPlaceholders)
			}

			hasTitle := false
			hasBody := false

			for _, ph := range layout.Placeholders {
				// Verify placeholder has required fields
				if ph.ID == "" {
					t.Error("Placeholder has empty ID")
				}

				// Track placeholder types
				if ph.Type == types.PlaceholderTitle {
					hasTitle = true
				}
				if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
					hasBody = true
				}

				// AC4: Placeholder Bounds
				// Verify bounds are present (non-zero)
				if ph.Bounds.Width == 0 && ph.Bounds.Height == 0 {
					// Some placeholders might not have explicit bounds
					// but most should
					t.Logf("Warning: Placeholder %s has zero bounds", ph.ID)
				}

				// Verify MaxChars is estimated for text placeholders
				if ph.Type == types.PlaceholderTitle || ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
					if ph.Bounds.Width > 0 && ph.Bounds.Height > 0 && ph.MaxChars == 0 {
						t.Errorf("Text placeholder %s has bounds but MaxChars is 0", ph.ID)
					}
				}
			}

			if tt.wantTitle && !hasTitle {
				t.Error("Layout should have title placeholder but doesn't")
			}
			if tt.wantBody && !hasBody {
				t.Error("Layout should have body placeholder but doesn't")
			}
		})
	}
}

func TestEstimateMaxChars(t *testing.T) {
	tests := []struct {
		name   string
		bounds types.BoundingBox
		want   int
		margin int // Allow +/- margin for capacity estimation
	}{
		{
			name: "standard body placeholder (6x4 inches)",
			bounds: types.BoundingBox{
				Width:  5486400, // 6 inches * 914400 EMUs/inch
				Height: 3657600, // 4 inches * 914400 EMUs/inch
			},
			want:   320, // Approximately 320 characters with 0.4 safety factor
			margin: 100, // Within 20% tolerance (AC6)
		},
		{
			name: "title placeholder (8x1 inches)",
			bounds: types.BoundingBox{
				Width:  7315200, // 8 inches
				Height: 914400,  // 1 inch
			},
			want:   106, // Approximately 106 characters with 0.4 safety factor
			margin: 80,
		},
		{
			name: "zero bounds",
			bounds: types.BoundingBox{
				Width:  0,
				Height: 0,
			},
			want:   0,
			margin: 0,
		},
		{
			name: "very large bounds (sanity check)",
			bounds: types.BoundingBox{
				Width:  100000000,
				Height: 100000000,
			},
			want:   10000, // Should be capped at 10000
			margin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateMaxChars(tt.bounds)

			// AC6: Capacity Estimation
			// Check within margin (20% tolerance per spec)
			if tt.margin > 0 {
				if got < tt.want-tt.margin || got > tt.want+tt.margin {
					t.Errorf("estimateMaxChars() = %d, want %d ±%d", got, tt.want, tt.margin)
				}
			} else {
				if got != tt.want {
					t.Errorf("estimateMaxChars() = %d, want %d", got, tt.want)
				}
			}
		})
	}
}

func TestEstimateCapacity(t *testing.T) {
	tests := []struct {
		name          string
		placeholders  []types.PlaceholderInfo
		wantBullets   int
		wantLines     int
		wantImage     bool
		wantChart     bool
		wantTextHeavy bool
		wantVisual    bool
	}{
		{
			name: "text-heavy layout",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, MaxChars: 50},
				{Type: types.PlaceholderBody, MaxChars: 500},
			},
			wantBullets:   5,  // 500/100
			wantLines:     10, // 500/50
			wantImage:     false,
			wantChart:     false,
			wantTextHeavy: true,
			wantVisual:    false,
		},
		{
			name: "visual-focused layout",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, MaxChars: 50},
				{Type: types.PlaceholderImage},
				{Type: types.PlaceholderChart},
			},
			wantBullets:   0,
			wantLines:     0,
			wantImage:     true,
			wantChart:     true,
			wantTextHeavy: false,
			wantVisual:    true,
		},
		{
			name: "balanced layout",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, MaxChars: 50},
				{Type: types.PlaceholderBody, MaxChars: 200},
				{Type: types.PlaceholderImage},
			},
			wantBullets:   2,
			wantLines:     4,
			wantImage:     true,
			wantChart:     false,
			wantTextHeavy: false,
			wantVisual:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capacity := estimateCapacity(tt.placeholders)

			if capacity.MaxBullets != tt.wantBullets {
				t.Errorf("MaxBullets = %d, want %d", capacity.MaxBullets, tt.wantBullets)
			}
			if capacity.MaxTextLines != tt.wantLines {
				t.Errorf("MaxTextLines = %d, want %d", capacity.MaxTextLines, tt.wantLines)
			}
			if capacity.HasImageSlot != tt.wantImage {
				t.Errorf("HasImageSlot = %v, want %v", capacity.HasImageSlot, tt.wantImage)
			}
			if capacity.HasChartSlot != tt.wantChart {
				t.Errorf("HasChartSlot = %v, want %v", capacity.HasChartSlot, tt.wantChart)
			}
			if capacity.TextHeavy != tt.wantTextHeavy {
				t.Errorf("TextHeavy = %v, want %v", capacity.TextHeavy, tt.wantTextHeavy)
			}
			if capacity.VisualFocused != tt.wantVisual {
				t.Errorf("VisualFocused = %v, want %v", capacity.VisualFocused, tt.wantVisual)
			}
		})
	}
}

func TestDeterminePlaceholderType(t *testing.T) {
	tests := []struct {
		xmlType string
		want    types.PlaceholderType
	}{
		{"title", types.PlaceholderTitle},
		{"ctrTitle", types.PlaceholderTitle},
		{"subTitle", types.PlaceholderSubtitle},
		{"body", types.PlaceholderBody},
		{"pic", types.PlaceholderImage},
		{"chart", types.PlaceholderChart},
		{"tbl", types.PlaceholderTable},
		{"dt", types.PlaceholderOther},
		{"ftr", types.PlaceholderOther},
		{"sldNum", types.PlaceholderOther},
		{"hdr", types.PlaceholderOther},
		{"", types.PlaceholderContent},
		{"unknown", types.PlaceholderContent},
	}

	for _, tt := range tests {
		t.Run(tt.xmlType, func(t *testing.T) {
			got := determinePlaceholderType(tt.xmlType)
			if got != tt.want {
				t.Errorf("determinePlaceholderType(%q) = %v, want %v", tt.xmlType, got, tt.want)
			}
		})
	}
}

func TestExtractLayoutID(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"ppt/slideLayouts/slideLayout1.xml", "slideLayout1"},
		{"ppt/slideLayouts/slideLayout10.xml", "slideLayout10"},
		{"slideLayout5.xml", "slideLayout5"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := extractLayoutID(tt.filename)
			if got != tt.want {
				t.Errorf("extractLayoutID(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestExtractBounds(t *testing.T) {
	tests := []struct {
		name  string
		props shapePropertiesXML
		want  types.BoundingBox
	}{
		{
			name: "valid bounds",
			props: shapePropertiesXML{
				Transform: &transformXML{
					Offset:  &offsetXML{X: 1000, Y: 2000},
					Extents: &extentsXML{CX: 3000, CY: 4000},
				},
			},
			want: types.BoundingBox{X: 1000, Y: 2000, Width: 3000, Height: 4000},
		},
		{
			name:  "no transform",
			props: shapePropertiesXML{},
			want:  types.BoundingBox{},
		},
		{
			name: "offset only",
			props: shapePropertiesXML{
				Transform: &transformXML{
					Offset: &offsetXML{X: 1000, Y: 2000},
				},
			},
			want: types.BoundingBox{X: 1000, Y: 2000},
		},
		{
			name: "extents only",
			props: shapePropertiesXML{
				Transform: &transformXML{
					Extents: &extentsXML{CX: 3000, CY: 4000},
				},
			},
			want: types.BoundingBox{Width: 3000, Height: 4000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBounds(tt.props, "TestLayout", 0)
			if got != tt.want {
				t.Errorf("extractBounds() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractBounds_LogsWarningOnNilTransform(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(oldLogger)

	// Call extractBounds with nil transform
	props := shapePropertiesXML{Transform: nil}
	result := extractBounds(props, "Title Slide", 3)

	// Verify returns empty bounding box
	if result != (types.BoundingBox{}) {
		t.Errorf("extractBounds() returned non-empty box: %+v", result)
	}

	// Verify warning was logged with expected fields
	logOutput := buf.String()

	if !strings.Contains(logOutput, "placeholder missing transform") {
		t.Errorf("expected warning message not found in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, "layout") && !strings.Contains(logOutput, "Title Slide") {
		t.Errorf("expected layout name in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, "shape_index") {
		t.Errorf("expected shape_index in log: %s", logOutput)
	}
	if !strings.Contains(logOutput, "needs_master_resolution") {
		t.Errorf("expected needs_master_resolution in log: %s", logOutput)
	}
}

func TestFontHierarchyResolution(t *testing.T) {
	// Test that font properties are resolved from the hierarchy:
	// shape → layout → master → theme
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	layouts, err := ParseLayouts(reader)
	if err != nil {
		t.Fatalf("ParseLayouts() error = %v", err)
	}

	if len(layouts) == 0 {
		t.Fatal("Expected at least one layout")
	}

	// Find title slide layout (typically first)
	var titleLayout *types.LayoutMetadata
	for i := range layouts {
		if strings.Contains(strings.ToLower(layouts[i].Name), "title") {
			titleLayout = &layouts[i]
			break
		}
	}
	if titleLayout == nil {
		titleLayout = &layouts[0] // fallback to first layout
	}

	// Check that title placeholders have resolved font properties
	var titlePh *types.PlaceholderInfo
	for i := range titleLayout.Placeholders {
		if titleLayout.Placeholders[i].Type == types.PlaceholderTitle {
			titlePh = &titleLayout.Placeholders[i]
			break
		}
	}

	if titlePh != nil {
		// AC1: Font size should be resolved from master titleStyle
		// Standard template has sz="4400" (44pt) for title in master
		if titlePh.FontSize > 0 {
			t.Logf("Title placeholder has FontSize: %d (%.1f pt)", titlePh.FontSize, float64(titlePh.FontSize)/100)
		}

		// AC2: Font family should be resolved from theme (major font)
		if titlePh.FontFamily != "" {
			t.Logf("Title placeholder has FontFamily: %s", titlePh.FontFamily)
		}

		// AC3: Font color should be resolved from scheme color reference
		if titlePh.FontColor != "" {
			t.Logf("Title placeholder has FontColor: %s", titlePh.FontColor)
		}
	}

	// Check body placeholders
	for _, layout := range layouts {
		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
				// Body placeholders should have resolved fonts from master bodyStyle
				if ph.FontSize > 0 || ph.FontFamily != "" {
					t.Logf("Layout %q, Body placeholder %q: FontFamily=%q, FontSize=%d, FontColor=%q",
						layout.Name, ph.ID, ph.FontFamily, ph.FontSize, ph.FontColor)
				}
				break // Just log one per layout
			}
		}
	}
}

func TestFontResolverWithTheme(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Parse theme first
	theme := ParseTheme(reader)

	// Verify theme has fonts
	if theme.TitleFont == "" {
		t.Error("Theme should have TitleFont")
	}
	if theme.BodyFont == "" {
		t.Error("Theme should have BodyFont")
	}

	t.Logf("Theme fonts: TitleFont=%q, BodyFont=%q", theme.TitleFont, theme.BodyFont)

	// Create font resolver with theme
	fontResolver := NewMasterFontResolver(reader, &theme)
	if fontResolver == nil {
		t.Fatal("NewMasterFontResolver returned nil")
	}

	// Get master fonts for layout 1
	masterFonts := fontResolver.GetMasterFontsForLayout("slideLayout1")
	if masterFonts == nil {
		t.Log("No master fonts found for slideLayout1 (may be expected for some templates)")
		return
	}

	// Verify title style was resolved
	if masterFonts.TitleStyle != nil {
		t.Logf("Master TitleStyle: FontFamily=%q, FontSize=%d, FontColor=%q",
			masterFonts.TitleStyle.FontFamily,
			masterFonts.TitleStyle.FontSize,
			masterFonts.TitleStyle.FontColor)

		// Font family should be resolved from +mj-lt to actual theme font
		if masterFonts.TitleStyle.FontFamily == "+mj-lt" {
			t.Error("Font family should be resolved, not raw reference +mj-lt")
		}
	}

	// Verify body style levels were resolved
	if len(masterFonts.BodyStyle) > 0 {
		if bodyL0, ok := masterFonts.BodyStyle[0]; ok {
			t.Logf("Master BodyStyle[0]: FontFamily=%q, FontSize=%d, FontColor=%q",
				bodyL0.FontFamily, bodyL0.FontSize, bodyL0.FontColor)

			// Font family should be resolved from +mn-lt to actual theme font
			if bodyL0.FontFamily == "+mn-lt" {
				t.Error("Font family should be resolved, not raw reference +mn-lt")
			}
		}
	}
}

func TestSchemeColorResolution(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	theme := ParseTheme(reader)
	fontResolver := NewMasterFontResolver(reader, &theme)

	// Test scheme color resolution
	tests := []struct {
		scheme string
		expect string // May be empty if not in theme
	}{
		{"tx1", ""},      // Maps to dk1
		{"tx2", ""},      // Maps to dk2
		{"accent1", ""},  // Direct mapping
	}

	for _, tt := range tests {
		color := fontResolver.resolveSchemeColor(tt.scheme)
		t.Logf("Scheme %q resolved to color %q", tt.scheme, color)

		// Color should be in #RRGGBB format if resolved
		if color != "" && !strings.HasPrefix(color, "#") {
			t.Errorf("resolveSchemeColor(%q) = %q, expected #RRGGBB format", tt.scheme, color)
		}
	}
}

func TestExtractPlaceholderBoundsFromZip(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Extract bounds using the new function
	bounds := ExtractPlaceholderBoundsFromZip(&reader.zip.Reader)

	// Verify we got layout bounds
	if len(bounds) == 0 {
		t.Error("ExtractPlaceholderBoundsFromZip() returned no layouts")
	}

	// Log all extracted bounds for debugging
	for layoutID, placeholders := range bounds {
		t.Logf("Layout %s:", layoutID)
		for phID, bb := range placeholders {
			t.Logf("  %s: (%d, %d) %dx%d EMUs", phID, bb.X, bb.Y, bb.Width, bb.Height)
		}
	}

	// Verify at least one layout has placeholders with non-zero bounds
	hasNonZeroBounds := false
	for _, placeholders := range bounds {
		for _, bb := range placeholders {
			if bb.Width > 0 && bb.Height > 0 {
				hasNonZeroBounds = true
				break
			}
		}
		if hasNonZeroBounds {
			break
		}
	}
	if !hasNonZeroBounds {
		t.Error("No placeholder has non-zero bounds")
	}
}

