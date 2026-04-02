package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestMasterFontResolver_resolveFontReference(t *testing.T) {
	tests := []struct {
		name     string
		theme    *types.ThemeInfo
		typeface string
		want     string
	}{
		{
			name:     "explicit font name returned as-is",
			theme:    &types.ThemeInfo{TitleFont: "Calibri Light", BodyFont: "Calibri"},
			typeface: "Arial",
			want:     "Arial",
		},
		{
			name:     "major latin font resolves to title font",
			theme:    &types.ThemeInfo{TitleFont: "Calibri Light", BodyFont: "Calibri"},
			typeface: "+mj-lt",
			want:     "Calibri Light",
		},
		{
			name:     "major east asian font resolves to title font",
			theme:    &types.ThemeInfo{TitleFont: "MS PGothic", BodyFont: "MS Gothic"},
			typeface: "+mj-ea",
			want:     "MS PGothic",
		},
		{
			name:     "major complex script font resolves to title font",
			theme:    &types.ThemeInfo{TitleFont: "Arial", BodyFont: "Tahoma"},
			typeface: "+mj-cs",
			want:     "Arial",
		},
		{
			name:     "minor latin font resolves to body font",
			theme:    &types.ThemeInfo{TitleFont: "Calibri Light", BodyFont: "Calibri"},
			typeface: "+mn-lt",
			want:     "Calibri",
		},
		{
			name:     "minor east asian font resolves to body font",
			theme:    &types.ThemeInfo{TitleFont: "MS PGothic", BodyFont: "MS Gothic"},
			typeface: "+mn-ea",
			want:     "MS Gothic",
		},
		{
			name:     "minor complex script font resolves to body font",
			theme:    &types.ThemeInfo{TitleFont: "Arial", BodyFont: "Tahoma"},
			typeface: "+mn-cs",
			want:     "Tahoma",
		},
		{
			name:     "nil theme returns typeface unchanged",
			theme:    nil,
			typeface: "+mj-lt",
			want:     "+mj-lt",
		},
		{
			name:     "unknown reference returned as-is",
			theme:    &types.ThemeInfo{TitleFont: "Calibri Light", BodyFont: "Calibri"},
			typeface: "+unknown",
			want:     "+unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &MasterFontResolver{theme: tt.theme}
			got := resolver.resolveFontReference(tt.typeface)
			if got != tt.want {
				t.Errorf("resolveFontReference(%q) = %q, want %q", tt.typeface, got, tt.want)
			}
		})
	}
}

func TestMasterFontResolver_resolveSchemeColor(t *testing.T) {
	themeWithColors := &types.ThemeInfo{
		Colors: []types.ThemeColor{
			{Name: "dk1", RGB: "#000000"},
			{Name: "lt1", RGB: "#FFFFFF"},
			{Name: "dk2", RGB: "#1F497D"},
			{Name: "lt2", RGB: "#EEECE1"},
			{Name: "accent1", RGB: "#4F81BD"},
			{Name: "accent2", RGB: "#C0504D"},
			{Name: "hlink", RGB: "#0000FF"},
			{Name: "folHlink", RGB: "#800080"},
		},
	}

	tests := []struct {
		name       string
		theme      *types.ThemeInfo
		schemeName string
		want       string
	}{
		{
			name:       "tx1 maps to dk1",
			theme:      themeWithColors,
			schemeName: "tx1",
			want:       "#000000",
		},
		{
			name:       "tx2 maps to dk2",
			theme:      themeWithColors,
			schemeName: "tx2",
			want:       "#1F497D",
		},
		{
			name:       "bg1 maps to lt1",
			theme:      themeWithColors,
			schemeName: "bg1",
			want:       "#FFFFFF",
		},
		{
			name:       "bg2 maps to lt2",
			theme:      themeWithColors,
			schemeName: "bg2",
			want:       "#EEECE1",
		},
		{
			name:       "accent1 maps to accent1",
			theme:      themeWithColors,
			schemeName: "accent1",
			want:       "#4F81BD",
		},
		{
			name:       "hlink maps to hlink",
			theme:      themeWithColors,
			schemeName: "hlink",
			want:       "#0000FF",
		},
		{
			name:       "folHlink maps to folHlink",
			theme:      themeWithColors,
			schemeName: "folHlink",
			want:       "#800080",
		},
		{
			name:       "nil theme returns empty",
			theme:      nil,
			schemeName: "tx1",
			want:       "",
		},
		{
			name:       "unknown scheme returns empty",
			theme:      themeWithColors,
			schemeName: "unknownScheme",
			want:       "",
		},
		{
			name:       "scheme exists but color not in theme returns empty",
			theme:      &types.ThemeInfo{Colors: []types.ThemeColor{}},
			schemeName: "tx1",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &MasterFontResolver{theme: tt.theme}
			got := resolver.resolveSchemeColor(tt.schemeName)
			if got != tt.want {
				t.Errorf("resolveSchemeColor(%q) = %q, want %q", tt.schemeName, got, tt.want)
			}
		})
	}
}

func TestNormalizeColorHex(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  string
	}{
		{
			name:  "already normalized with hash",
			color: "#FF0000",
			want:  "#FF0000",
		},
		{
			name:  "missing hash prefix",
			color: "FF0000",
			want:  "#FF0000",
		},
		{
			name:  "lowercase converted to uppercase",
			color: "#ff0000",
			want:  "#FF0000",
		},
		{
			name:  "mixed case normalized",
			color: "aAbBcC",
			want:  "#AABBCC",
		},
		{
			name:  "whitespace trimmed",
			color: "  FF0000  ",
			want:  "#FF0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeColorHex(tt.color)
			if got != tt.want {
				t.Errorf("normalizeColorHex(%q) = %q, want %q", tt.color, got, tt.want)
			}
		})
	}
}

func TestMasterFontResolver_ResolvePlaceholderFonts(t *testing.T) {
	titleStyle := &FontStyle{FontFamily: "Calibri Light", FontSize: 4400, FontColor: "#000000"}
	bodyStyle0 := &FontStyle{FontFamily: "Calibri", FontSize: 2400, FontColor: "#333333"}
	otherStyle0 := &FontStyle{FontFamily: "Arial", FontSize: 1200, FontColor: "#666666"}

	masterStyles := &MasterFontStyles{
		TitleStyle: titleStyle,
		BodyStyle:  map[int]*FontStyle{0: bodyStyle0},
		OtherStyle: map[int]*FontStyle{0: otherStyle0},
	}

	tests := []struct {
		name            string
		shape           *shapeXML
		placeholderType types.PlaceholderType
		masterStyles    *MasterFontStyles
		wantStyle       *FontStyle
	}{
		{
			name:            "title placeholder uses TitleStyle",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderTitle,
			masterStyles:    masterStyles,
			wantStyle:       titleStyle,
		},
		{
			name:            "subtitle placeholder uses BodyStyle level 0",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderSubtitle,
			masterStyles:    masterStyles,
			wantStyle:       bodyStyle0,
		},
		{
			name:            "body placeholder uses BodyStyle level 0",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderBody,
			masterStyles:    masterStyles,
			wantStyle:       bodyStyle0,
		},
		{
			name:            "content placeholder uses BodyStyle level 0",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderContent,
			masterStyles:    masterStyles,
			wantStyle:       bodyStyle0,
		},
		{
			name:            "unknown placeholder uses OtherStyle level 0",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderType("custom"),
			masterStyles:    masterStyles,
			wantStyle:       otherStyle0,
		},
		{
			name:            "nil masterStyles returns nil",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderTitle,
			masterStyles:    nil,
			wantStyle:       nil,
		},
		{
			name:            "empty masterStyles returns nil for title",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderTitle,
			masterStyles:    &MasterFontStyles{BodyStyle: make(map[int]*FontStyle), OtherStyle: make(map[int]*FontStyle)},
			wantStyle:       nil,
		},
		{
			name:            "empty body style returns nil for body",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderBody,
			masterStyles:    &MasterFontStyles{TitleStyle: titleStyle, BodyStyle: make(map[int]*FontStyle), OtherStyle: make(map[int]*FontStyle)},
			wantStyle:       nil,
		},
		{
			name:            "empty other style returns nil for custom",
			shape:           &shapeXML{},
			placeholderType: types.PlaceholderType("footer"),
			masterStyles:    &MasterFontStyles{TitleStyle: titleStyle, BodyStyle: make(map[int]*FontStyle), OtherStyle: make(map[int]*FontStyle)},
			wantStyle:       nil,
		},
		{
			name: "shape with own lstStyle takes precedence",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					ListStyle: &listStyleXML{
						Lvl1pPr: &levelParagraphPropsXML{
							DefRPr: &defaultRunPropsXML{
								Size: 3200,
								Latin: &latinFontXML{
									Typeface: "Times New Roman",
								},
							},
						},
					},
				},
			},
			placeholderType: types.PlaceholderTitle,
			masterStyles:    masterStyles,
			wantStyle:       &FontStyle{FontFamily: "Times New Roman", FontSize: 3200, FontColor: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &MasterFontResolver{
				theme: &types.ThemeInfo{TitleFont: "Calibri Light", BodyFont: "Calibri"},
			}
			got := resolver.ResolvePlaceholderFonts(tt.shape, tt.placeholderType, tt.masterStyles)

			if tt.wantStyle == nil {
				if got != nil {
					t.Errorf("ResolvePlaceholderFonts() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("ResolvePlaceholderFonts() = nil, want %+v", tt.wantStyle)
				return
			}

			if got.FontFamily != tt.wantStyle.FontFamily {
				t.Errorf("FontFamily = %q, want %q", got.FontFamily, tt.wantStyle.FontFamily)
			}
			if got.FontSize != tt.wantStyle.FontSize {
				t.Errorf("FontSize = %d, want %d", got.FontSize, tt.wantStyle.FontSize)
			}
			if got.FontColor != tt.wantStyle.FontColor {
				t.Errorf("FontColor = %q, want %q", got.FontColor, tt.wantStyle.FontColor)
			}
		})
	}
}

func TestMasterFontResolver_extractShapeFonts(t *testing.T) {
	tests := []struct {
		name      string
		shape     *shapeXML
		wantStyle *FontStyle
	}{
		{
			name:      "nil TextBody returns nil",
			shape:     &shapeXML{},
			wantStyle: nil,
		},
		{
			name: "nil ListStyle returns nil",
			shape: &shapeXML{
				TextBody: &textBodyXML{},
			},
			wantStyle: nil,
		},
		{
			name: "nil Lvl1pPr returns nil",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					ListStyle: &listStyleXML{},
				},
			},
			wantStyle: nil,
		},
		{
			name: "nil DefRPr returns nil",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					ListStyle: &listStyleXML{
						Lvl1pPr: &levelParagraphPropsXML{},
					},
				},
			},
			wantStyle: nil,
		},
		{
			name: "valid lstStyle extracts font info",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					ListStyle: &listStyleXML{
						Lvl1pPr: &levelParagraphPropsXML{
							DefRPr: &defaultRunPropsXML{
								Size: 1800,
								Latin: &latinFontXML{
									Typeface: "Verdana",
								},
							},
						},
					},
				},
			},
			wantStyle: &FontStyle{FontFamily: "Verdana", FontSize: 1800, FontColor: ""},
		},
		{
			name: "lstStyle with solidFill sRGB color",
			shape: &shapeXML{
				TextBody: &textBodyXML{
					ListStyle: &listStyleXML{
						Lvl1pPr: &levelParagraphPropsXML{
							DefRPr: &defaultRunPropsXML{
								Size: 2000,
								Latin: &latinFontXML{
									Typeface: "Georgia",
								},
								SolidFill: &solidFillXML{
									SRGBColor: &srgbColorXML{Val: "FF5500"},
								},
							},
						},
					},
				},
			},
			wantStyle: &FontStyle{FontFamily: "Georgia", FontSize: 2000, FontColor: "#FF5500"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &MasterFontResolver{
				theme: &types.ThemeInfo{TitleFont: "Calibri", BodyFont: "Calibri"},
			}
			got := resolver.extractShapeFonts(tt.shape)

			if tt.wantStyle == nil {
				if got != nil {
					t.Errorf("extractShapeFonts() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("extractShapeFonts() = nil, want %+v", tt.wantStyle)
				return
			}

			if got.FontFamily != tt.wantStyle.FontFamily {
				t.Errorf("FontFamily = %q, want %q", got.FontFamily, tt.wantStyle.FontFamily)
			}
			if got.FontSize != tt.wantStyle.FontSize {
				t.Errorf("FontSize = %d, want %d", got.FontSize, tt.wantStyle.FontSize)
			}
			if got.FontColor != tt.wantStyle.FontColor {
				t.Errorf("FontColor = %q, want %q", got.FontColor, tt.wantStyle.FontColor)
			}
		})
	}
}

func TestMasterFontResolver_GetMasterFontsForLayout_Integration(t *testing.T) {
	// Create a test PPTX with layout and master relationships
	layoutRelsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`

	// Use local element names without namespace prefixes for Go's xml parser
	slideMasterXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sldMaster xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"
           xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
	<txStyles>
		<titleStyle>
			<lvl1pPr xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">
				<defRPr sz="4400">
					<solidFill><srgbClr val="000000"/></solidFill>
					<latin typeface="+mj-lt"/>
				</defRPr>
			</lvl1pPr>
		</titleStyle>
		<bodyStyle>
			<lvl1pPr xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">
				<defRPr sz="2800">
					<solidFill><srgbClr val="333333"/></solidFill>
					<latin typeface="+mn-lt"/>
				</defRPr>
			</lvl1pPr>
			<lvl2pPr xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">
				<defRPr sz="2400">
					<latin typeface="+mn-lt"/>
				</defRPr>
			</lvl2pPr>
		</bodyStyle>
		<otherStyle>
			<lvl1pPr xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">
				<defRPr sz="1200">
					<latin typeface="Arial"/>
				</defRPr>
			</lvl1pPr>
		</otherStyle>
	</txStyles>
</sldMaster>`

	// GetMasterFontsForLayout expects layoutID without .xml extension (e.g., "slideLayout1")
	// It builds the rels path as: ppt/slideLayouts/_rels/{layoutID}.xml.rels
	path := createTestPPTXWithContent(t, map[string][]byte{
		"ppt/presentation.xml":                                   []byte("<presentation/>"),
		"ppt/slideLayouts/slideLayout1.xml":                      []byte("<layout/>"),
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels":           []byte(layoutRelsXML),
		"ppt/slideMasters/slideMaster1.xml":                      []byte(slideMasterXML),
	})

	reader, err := OpenTemplate(path)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	theme := &types.ThemeInfo{
		TitleFont: "Calibri Light",
		BodyFont:  "Calibri",
	}
	resolver := NewMasterFontResolver(reader, theme)

	// First call - should parse and cache (layoutID is "slideLayout1" without .xml extension)
	styles := resolver.GetMasterFontsForLayout("slideLayout1")
	if styles == nil {
		t.Fatal("GetMasterFontsForLayout() returned nil, expected styles")
	}

	// Verify title style
	if styles.TitleStyle == nil {
		t.Error("TitleStyle is nil")
	} else {
		if styles.TitleStyle.FontFamily != "Calibri Light" {
			t.Errorf("TitleStyle.FontFamily = %q, want %q", styles.TitleStyle.FontFamily, "Calibri Light")
		}
		if styles.TitleStyle.FontSize != 4400 {
			t.Errorf("TitleStyle.FontSize = %d, want %d", styles.TitleStyle.FontSize, 4400)
		}
	}

	// Verify body style level 0
	if bodyStyle, ok := styles.BodyStyle[0]; !ok {
		t.Error("BodyStyle[0] not found")
	} else {
		if bodyStyle.FontFamily != "Calibri" {
			t.Errorf("BodyStyle[0].FontFamily = %q, want %q", bodyStyle.FontFamily, "Calibri")
		}
		if bodyStyle.FontSize != 2800 {
			t.Errorf("BodyStyle[0].FontSize = %d, want %d", bodyStyle.FontSize, 2800)
		}
	}

	// Verify body style level 1
	if bodyStyle, ok := styles.BodyStyle[1]; !ok {
		t.Error("BodyStyle[1] not found")
	} else {
		if bodyStyle.FontSize != 2400 {
			t.Errorf("BodyStyle[1].FontSize = %d, want %d", bodyStyle.FontSize, 2400)
		}
	}

	// Verify other style
	if otherStyle, ok := styles.OtherStyle[0]; !ok {
		t.Error("OtherStyle[0] not found")
	} else {
		if otherStyle.FontFamily != "Arial" {
			t.Errorf("OtherStyle[0].FontFamily = %q, want %q", otherStyle.FontFamily, "Arial")
		}
	}

	// Second call - should return cached result (same pointer)
	styles2 := resolver.GetMasterFontsForLayout("slideLayout1")
	if styles != styles2 {
		t.Error("Second call did not return cached result")
	}
}

func TestMasterFontResolver_GetMasterFontsForLayout_Errors(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string][]byte
		layoutID string
	}{
		{
			name: "layout rels file not found",
			files: map[string][]byte{
				"ppt/presentation.xml":              []byte("<presentation/>"),
				"ppt/slideLayouts/slideLayout1.xml": []byte("<layout/>"),
				// No _rels file
			},
			layoutID: "slideLayout1",
		},
		{
			name: "invalid rels XML",
			files: map[string][]byte{
				"ppt/presentation.xml":                          []byte("<presentation/>"),
				"ppt/slideLayouts/slideLayout1.xml":             []byte("<layout/>"),
				"ppt/slideLayouts/_rels/slideLayout1.xml.rels":  []byte("not valid xml"),
			},
			layoutID: "slideLayout1",
		},
		{
			name: "no slideMaster relationship",
			files: map[string][]byte{
				"ppt/presentation.xml":                         []byte("<presentation/>"),
				"ppt/slideLayouts/slideLayout1.xml":            []byte("<layout/>"),
				"ppt/slideLayouts/_rels/slideLayout1.xml.rels": []byte(`<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`),
			},
			layoutID: "slideLayout1",
		},
		{
			name: "master file not found",
			files: map[string][]byte{
				"ppt/presentation.xml":                         []byte("<presentation/>"),
				"ppt/slideLayouts/slideLayout1.xml":            []byte("<layout/>"),
				"ppt/slideLayouts/_rels/slideLayout1.xml.rels": []byte(`<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`),
				// No slideMaster1.xml file
			},
			layoutID: "slideLayout1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTestPPTXWithContent(t, tt.files)
			reader, err := OpenTemplate(path)
			if err != nil {
				t.Fatalf("OpenTemplate() error = %v", err)
			}
			defer func() { _ = reader.Close() }()

			resolver := NewMasterFontResolver(reader, &types.ThemeInfo{})
			styles := resolver.GetMasterFontsForLayout(tt.layoutID)

			if styles != nil {
				t.Errorf("GetMasterFontsForLayout() = %+v, want nil for error case", styles)
			}
		})
	}
}

func TestNewMasterFontResolver(t *testing.T) {
	path := createTestPPTXWithContent(t, map[string][]byte{
		"ppt/presentation.xml":              []byte("<presentation/>"),
		"ppt/slideLayouts/slideLayout1.xml": []byte("<layout/>"),
	})

	reader, err := OpenTemplate(path)
	if err != nil {
		t.Fatalf("OpenTemplate() error = %v", err)
	}
	defer func() { _ = reader.Close() }()

	theme := &types.ThemeInfo{TitleFont: "Test Title", BodyFont: "Test Body"}
	resolver := NewMasterFontResolver(reader, theme)

	if resolver == nil {
		t.Fatal("NewMasterFontResolver() returned nil")
	}
	if resolver.reader != reader {
		t.Error("resolver.reader not set correctly")
	}
	if resolver.theme != theme {
		t.Error("resolver.theme not set correctly")
	}
	if resolver.cache == nil {
		t.Error("resolver.cache is nil")
	}
}
