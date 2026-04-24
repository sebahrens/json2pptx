package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPlaceholderTypeConstants(t *testing.T) {
	tests := []struct {
		constant PlaceholderType
		expected string
	}{
		{PlaceholderTitle, "title"},
		{PlaceholderSubtitle, "subtitle"},
		{PlaceholderBody, "body"},
		{PlaceholderImage, "image"},
		{PlaceholderChart, "chart"},
		{PlaceholderTable, "table"},
		{PlaceholderContent, "content"},
		{PlaceholderOther, "other"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("PlaceholderType constant = %q, want %q", tt.constant, tt.expected)
			}
		})
	}
}

func TestMetadataVersionConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"MetadataVersionCurrent", MetadataVersionCurrent, "1.0"},
		{"MetadataVersionMin", MetadataVersionMin, "1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestTemplateMetadata_JSON(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		metadata TemplateMetadata
		wantJSON string
	}{
		{
			name: "full metadata",
			metadata: TemplateMetadata{
				Version:     "1.0",
				Name:        "Corporate Template",
				Description: "A professional template",
				Author:      "Test Author",
				Tags:        []string{"corporate", "professional"},
				CreatedAt:   &now,
				UpdatedAt:   &now,
				AspectRatio: "16:9",
				LayoutHints: map[string]LayoutHint{
					"title": {
						PreferredFor: []string{"title"},
						MaxBullets:   0,
						MaxChars:     100,
						Deprecated:   false,
					},
				},
			},
		},
		{
			name: "minimal metadata",
			metadata: TemplateMetadata{
				Version: "1.0",
			},
		},
		{
			name:     "empty metadata",
			metadata: TemplateMetadata{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.metadata)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Test unmarshaling back
			var decoded TemplateMetadata
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Verify version is preserved
			if decoded.Version != tt.metadata.Version {
				t.Errorf("Version = %q, want %q", decoded.Version, tt.metadata.Version)
			}

			// Verify name is preserved
			if decoded.Name != tt.metadata.Name {
				t.Errorf("Name = %q, want %q", decoded.Name, tt.metadata.Name)
			}

			// Verify tags are preserved
			if len(decoded.Tags) != len(tt.metadata.Tags) {
				t.Errorf("Tags length = %d, want %d", len(decoded.Tags), len(tt.metadata.Tags))
			}
		})
	}
}

func TestTemplateMetadata_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		jsonInput string
		want      TemplateMetadata
		wantErr   bool
	}{
		{
			name:      "valid full metadata",
			jsonInput: `{"version":"1.0","name":"Test","description":"Desc","author":"Author","tags":["a","b"],"aspect_ratio":"16:9"}`,
			want: TemplateMetadata{
				Version:     "1.0",
				Name:        "Test",
				Description: "Desc",
				Author:      "Author",
				Tags:        []string{"a", "b"},
				AspectRatio: "16:9",
			},
		},
		{
			name:      "empty JSON object",
			jsonInput: `{}`,
			want:      TemplateMetadata{},
		},
		{
			name:      "with layout hints",
			jsonInput: `{"version":"1.0","layout_hints":{"content":{"preferred_for":["bullets"],"max_bullets":5}}}`,
			want: TemplateMetadata{
				Version: "1.0",
				LayoutHints: map[string]LayoutHint{
					"content": {
						PreferredFor: []string{"bullets"},
						MaxBullets:   5,
					},
				},
			},
		},
		{
			name:      "invalid JSON",
			jsonInput: `{invalid`,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got TemplateMetadata
			err := json.Unmarshal([]byte(tt.jsonInput), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if got.Version != tt.want.Version {
				t.Errorf("Version = %q, want %q", got.Version, tt.want.Version)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
		})
	}
}

func TestLayoutHint_JSON(t *testing.T) {
	tests := []struct {
		name string
		hint LayoutHint
	}{
		{
			name: "full hint",
			hint: LayoutHint{
				PreferredFor: []string{"title", "content"},
				MaxBullets:   5,
				MaxChars:     500,
				Deprecated:   true,
			},
		},
		{
			name: "minimal hint",
			hint: LayoutHint{
				MaxBullets: 3,
			},
		},
		{
			name: "empty hint",
			hint: LayoutHint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.hint)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded LayoutHint
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if decoded.MaxBullets != tt.hint.MaxBullets {
				t.Errorf("MaxBullets = %d, want %d", decoded.MaxBullets, tt.hint.MaxBullets)
			}
			if decoded.MaxChars != tt.hint.MaxChars {
				t.Errorf("MaxChars = %d, want %d", decoded.MaxChars, tt.hint.MaxChars)
			}
			if decoded.Deprecated != tt.hint.Deprecated {
				t.Errorf("Deprecated = %v, want %v", decoded.Deprecated, tt.hint.Deprecated)
			}
		})
	}
}

func TestBoundingBox_ZeroValue(t *testing.T) {
	var bb BoundingBox

	if bb.X != 0 {
		t.Errorf("zero BoundingBox.X = %d, want 0", bb.X)
	}
	if bb.Y != 0 {
		t.Errorf("zero BoundingBox.Y = %d, want 0", bb.Y)
	}
	if bb.Width != 0 {
		t.Errorf("zero BoundingBox.Width = %d, want 0", bb.Width)
	}
	if bb.Height != 0 {
		t.Errorf("zero BoundingBox.Height = %d, want 0", bb.Height)
	}
}

func TestBoundingBox_Values(t *testing.T) {
	// Test with typical slide dimensions in EMUs
	// A typical 16:9 slide is 9144000 x 5143500 EMUs
	bb := BoundingBox{
		X:      914400,  // 1 inch from left
		Y:      457200,  // 0.5 inch from top
		Width:  7315200, // 8 inches wide
		Height: 4572000, // 5 inches tall
	}

	if bb.X != 914400 {
		t.Errorf("BoundingBox.X = %d, want 914400", bb.X)
	}
	if bb.Y != 457200 {
		t.Errorf("BoundingBox.Y = %d, want 457200", bb.Y)
	}
	if bb.Width != 7315200 {
		t.Errorf("BoundingBox.Width = %d, want 7315200", bb.Width)
	}
	if bb.Height != 4572000 {
		t.Errorf("BoundingBox.Height = %d, want 4572000", bb.Height)
	}
}

func TestCapacityEstimate_ZeroValue(t *testing.T) {
	var ce CapacityEstimate

	if ce.MaxBullets != 0 {
		t.Errorf("zero CapacityEstimate.MaxBullets = %d, want 0", ce.MaxBullets)
	}
	if ce.MaxTextLines != 0 {
		t.Errorf("zero CapacityEstimate.MaxTextLines = %d, want 0", ce.MaxTextLines)
	}
	if ce.HasImageSlot {
		t.Error("zero CapacityEstimate.HasImageSlot = true, want false")
	}
	if ce.HasChartSlot {
		t.Error("zero CapacityEstimate.HasChartSlot = true, want false")
	}
	if ce.TextHeavy {
		t.Error("zero CapacityEstimate.TextHeavy = true, want false")
	}
	if ce.VisualFocused {
		t.Error("zero CapacityEstimate.VisualFocused = true, want false")
	}
}

func TestCapacityEstimate_Values(t *testing.T) {
	tests := []struct {
		name           string
		capacity       CapacityEstimate
		wantMaxBullets int
		wantImageSlot  bool
		wantTextHeavy  bool
	}{
		{
			name: "text-heavy layout",
			capacity: CapacityEstimate{
				MaxBullets:   8,
				MaxTextLines: 12,
				TextHeavy:    true,
			},
			wantMaxBullets: 8,
			wantTextHeavy:  true,
		},
		{
			name: "visual layout",
			capacity: CapacityEstimate{
				MaxBullets:    3,
				HasImageSlot:  true,
				VisualFocused: true,
			},
			wantMaxBullets: 3,
			wantImageSlot:  true,
		},
		{
			name: "chart layout",
			capacity: CapacityEstimate{
				MaxBullets:    2,
				HasChartSlot:  true,
				VisualFocused: true,
			},
			wantMaxBullets: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.capacity.MaxBullets != tt.wantMaxBullets {
				t.Errorf("MaxBullets = %d, want %d", tt.capacity.MaxBullets, tt.wantMaxBullets)
			}
			if tt.capacity.HasImageSlot != tt.wantImageSlot {
				t.Errorf("HasImageSlot = %v, want %v", tt.capacity.HasImageSlot, tt.wantImageSlot)
			}
			if tt.capacity.TextHeavy != tt.wantTextHeavy {
				t.Errorf("TextHeavy = %v, want %v", tt.capacity.TextHeavy, tt.wantTextHeavy)
			}
		})
	}
}

func TestPlaceholderInfo_ZeroValue(t *testing.T) {
	var pi PlaceholderInfo

	if pi.ID != "" {
		t.Errorf("zero PlaceholderInfo.ID = %q, want empty", pi.ID)
	}
	if pi.Type != "" {
		t.Errorf("zero PlaceholderInfo.Type = %q, want empty", pi.Type)
	}
	if pi.Index != 0 {
		t.Errorf("zero PlaceholderInfo.Index = %d, want 0", pi.Index)
	}
	if pi.MaxChars != 0 {
		t.Errorf("zero PlaceholderInfo.MaxChars = %d, want 0", pi.MaxChars)
	}
}

func TestPlaceholderInfo_Values(t *testing.T) {
	pi := PlaceholderInfo{
		ID:         "p1",
		Type:       PlaceholderTitle,
		Index:      0,
		Bounds:     BoundingBox{X: 914400, Y: 457200, Width: 7315200, Height: 914400},
		MaxChars:   100,
		FontFamily: "Arial",
		FontSize:   4400, // 44pt in hundredths of a point
		FontColor:  "#000000",
	}

	if pi.ID != "p1" {
		t.Errorf("PlaceholderInfo.ID = %q, want %q", pi.ID, "p1")
	}
	if pi.Type != PlaceholderTitle {
		t.Errorf("PlaceholderInfo.Type = %q, want %q", pi.Type, PlaceholderTitle)
	}
	if pi.FontFamily != "Arial" {
		t.Errorf("PlaceholderInfo.FontFamily = %q, want %q", pi.FontFamily, "Arial")
	}
	if pi.FontSize != 4400 {
		t.Errorf("PlaceholderInfo.FontSize = %d, want 4400", pi.FontSize)
	}
	if pi.FontColor != "#000000" {
		t.Errorf("PlaceholderInfo.FontColor = %q, want %q", pi.FontColor, "#000000")
	}
}

func TestLayoutMetadata_Values(t *testing.T) {
	lm := LayoutMetadata{
		ID:    "layout1",
		Name:  "Title Slide",
		Index: 0,
		Placeholders: []PlaceholderInfo{
			{ID: "p1", Type: PlaceholderTitle, Index: 0},
			{ID: "p2", Type: PlaceholderSubtitle, Index: 1},
		},
		Capacity: CapacityEstimate{
			MaxBullets:   0,
			MaxTextLines: 2,
			TextHeavy:    false,
		},
		Tags: []string{"title", "opening"},
	}

	if lm.ID != "layout1" {
		t.Errorf("LayoutMetadata.ID = %q, want %q", lm.ID, "layout1")
	}
	if lm.Name != "Title Slide" {
		t.Errorf("LayoutMetadata.Name = %q, want %q", lm.Name, "Title Slide")
	}
	if len(lm.Placeholders) != 2 {
		t.Errorf("len(Placeholders) = %d, want 2", len(lm.Placeholders))
	}
	if len(lm.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(lm.Tags))
	}
}

func TestThemeInfo_Values(t *testing.T) {
	ti := ThemeInfo{
		Name: "Corporate Theme",
		Colors: []ThemeColor{
			{Name: "accent1", RGB: "#4472C4"},
			{Name: "accent2", RGB: "#ED7D31"},
		},
		TitleFont: "Calibri Light",
		BodyFont:  "Calibri",
	}

	if ti.Name != "Corporate Theme" {
		t.Errorf("ThemeInfo.Name = %q, want %q", ti.Name, "Corporate Theme")
	}
	if len(ti.Colors) != 2 {
		t.Errorf("len(Colors) = %d, want 2", len(ti.Colors))
	}
	if ti.TitleFont != "Calibri Light" {
		t.Errorf("ThemeInfo.TitleFont = %q, want %q", ti.TitleFont, "Calibri Light")
	}
	if ti.BodyFont != "Calibri" {
		t.Errorf("ThemeInfo.BodyFont = %q, want %q", ti.BodyFont, "Calibri")
	}
}

func TestThemeColor_Values(t *testing.T) {
	tests := []struct {
		name      string
		color     ThemeColor
		wantName  string
		wantRGB   string
	}{
		{
			name:     "accent color",
			color:    ThemeColor{Name: "accent1", RGB: "#4472C4"},
			wantName: "accent1",
			wantRGB:  "#4472C4",
		},
		{
			name:     "dark color",
			color:    ThemeColor{Name: "dk1", RGB: "#000000"},
			wantName: "dk1",
			wantRGB:  "#000000",
		},
		{
			name:     "light color",
			color:    ThemeColor{Name: "lt1", RGB: "#FFFFFF"},
			wantName: "lt1",
			wantRGB:  "#FFFFFF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Name != tt.wantName {
				t.Errorf("ThemeColor.Name = %q, want %q", tt.color.Name, tt.wantName)
			}
			if tt.color.RGB != tt.wantRGB {
				t.Errorf("ThemeColor.RGB = %q, want %q", tt.color.RGB, tt.wantRGB)
			}
		})
	}
}

func TestTemplateAnalysis_Values(t *testing.T) {
	now := time.Now()

	ta := TemplateAnalysis{
		TemplatePath: "/path/to/template.pptx",
		Hash:         "abc123def456",
		AspectRatio:  "16:9",
		Layouts: []LayoutMetadata{
			{ID: "layout1", Name: "Title"},
			{ID: "layout2", Name: "Content"},
		},
		Theme: ThemeInfo{
			Name:      "Default",
			TitleFont: "Arial",
			BodyFont:  "Arial",
		},
		AnalyzedAt: now,
		Metadata: &TemplateMetadata{
			Version: "1.0",
			Name:    "Test Template",
		},
	}

	if ta.TemplatePath != "/path/to/template.pptx" {
		t.Errorf("TemplatePath = %q, want %q", ta.TemplatePath, "/path/to/template.pptx")
	}
	if ta.Hash != "abc123def456" {
		t.Errorf("Hash = %q, want %q", ta.Hash, "abc123def456")
	}
	if ta.AspectRatio != "16:9" {
		t.Errorf("AspectRatio = %q, want %q", ta.AspectRatio, "16:9")
	}
	if len(ta.Layouts) != 2 {
		t.Errorf("len(Layouts) = %d, want 2", len(ta.Layouts))
	}
	if ta.Metadata == nil {
		t.Error("Metadata is nil, want non-nil")
	}
	if ta.Metadata.Version != "1.0" {
		t.Errorf("Metadata.Version = %q, want %q", ta.Metadata.Version, "1.0")
	}
}

func TestTemplateAnalysis_NilMetadata(t *testing.T) {
	ta := TemplateAnalysis{
		TemplatePath: "/path/to/template.pptx",
		AspectRatio:  "4:3",
	}

	if ta.Metadata != nil {
		t.Error("Metadata = non-nil, want nil for template without embedded metadata")
	}
}

func TestThemeInfo_ApplyOverride(t *testing.T) {
	base := ThemeInfo{
		Name:      "Corporate",
		TitleFont: "Calibri Light",
		BodyFont:  "Calibri",
		Colors: []ThemeColor{
			{Name: "dk1", RGB: "#000000"},
			{Name: "lt1", RGB: "#FFFFFF"},
			{Name: "accent1", RGB: "#4472C4"},
			{Name: "accent2", RGB: "#ED7D31"},
			{Name: "accent3", RGB: "#A5A5A5"},
		},
	}

	tests := []struct {
		name         string
		override     *ThemeOverride
		want         ThemeInfo
		wantWarnings int
	}{
		{
			name:     "nil override returns copy",
			override: nil,
			want:     base,
		},
		{
			name: "override single color",
			override: &ThemeOverride{
				Colors: map[string]string{"accent1": "#336699"},
			},
			want: ThemeInfo{
				Name:      "Corporate",
				TitleFont: "Calibri Light",
				BodyFont:  "Calibri",
				Colors: []ThemeColor{
					{Name: "dk1", RGB: "#000000"},
					{Name: "lt1", RGB: "#FFFFFF"},
					{Name: "accent1", RGB: "#336699"},
					{Name: "accent2", RGB: "#ED7D31"},
					{Name: "accent3", RGB: "#A5A5A5"},
				},
			},
		},
		{
			name: "override multiple colors",
			override: &ThemeOverride{
				Colors: map[string]string{
					"accent1": "#FF0000",
					"accent2": "#00FF00",
					"dk1":     "#111111",
				},
			},
			want: ThemeInfo{
				Name:      "Corporate",
				TitleFont: "Calibri Light",
				BodyFont:  "Calibri",
				Colors: []ThemeColor{
					{Name: "dk1", RGB: "#111111"},
					{Name: "lt1", RGB: "#FFFFFF"},
					{Name: "accent1", RGB: "#FF0000"},
					{Name: "accent2", RGB: "#00FF00"},
					{Name: "accent3", RGB: "#A5A5A5"},
				},
			},
		},
		{
			name: "override fonts only",
			override: &ThemeOverride{
				TitleFont: "Helvetica",
				BodyFont:  "Arial",
			},
			want: ThemeInfo{
				Name:      "Corporate",
				TitleFont: "Helvetica",
				BodyFont:  "Arial",
				Colors: []ThemeColor{
					{Name: "dk1", RGB: "#000000"},
					{Name: "lt1", RGB: "#FFFFFF"},
					{Name: "accent1", RGB: "#4472C4"},
					{Name: "accent2", RGB: "#ED7D31"},
					{Name: "accent3", RGB: "#A5A5A5"},
				},
			},
		},
		{
			name: "override colors and fonts",
			override: &ThemeOverride{
				Colors:    map[string]string{"accent1": "#AABBCC"},
				TitleFont: "Georgia",
			},
			want: ThemeInfo{
				Name:      "Corporate",
				TitleFont: "Georgia",
				BodyFont:  "Calibri",
				Colors: []ThemeColor{
					{Name: "dk1", RGB: "#000000"},
					{Name: "lt1", RGB: "#FFFFFF"},
					{Name: "accent1", RGB: "#AABBCC"},
					{Name: "accent2", RGB: "#ED7D31"},
					{Name: "accent3", RGB: "#A5A5A5"},
				},
			},
		},
		{
			name: "override nonexistent color warns",
			override: &ThemeOverride{
				Colors: map[string]string{"accent9": "#999999"},
			},
			want:         base,
			wantWarnings: 1,
		},
		{
			name:     "empty override changes nothing",
			override: &ThemeOverride{},
			want:     base,
		},
		{
			name: "multiple unknown keys produce multiple warnings",
			override: &ThemeOverride{
				Colors: map[string]string{
					"brand1":  "#FF0000",
					"Accent1": "#00FF00",
					"accent1": "#336699", // valid — should not warn
				},
			},
			want: ThemeInfo{
				Name:      "Corporate",
				TitleFont: "Calibri Light",
				BodyFont:  "Calibri",
				Colors: []ThemeColor{
					{Name: "dk1", RGB: "#000000"},
					{Name: "lt1", RGB: "#FFFFFF"},
					{Name: "accent1", RGB: "#336699"},
					{Name: "accent2", RGB: "#ED7D31"},
					{Name: "accent3", RGB: "#A5A5A5"},
				},
			},
			wantWarnings: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings := base.ApplyOverride(tt.override)

			if len(warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.wantWarnings, warnings)
			}

			// Verify original is not mutated
			if base.Colors[2].RGB != "#4472C4" {
				t.Fatal("original ThemeInfo was mutated")
			}

			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.TitleFont != tt.want.TitleFont {
				t.Errorf("TitleFont = %q, want %q", got.TitleFont, tt.want.TitleFont)
			}
			if got.BodyFont != tt.want.BodyFont {
				t.Errorf("BodyFont = %q, want %q", got.BodyFont, tt.want.BodyFont)
			}
			if len(got.Colors) != len(tt.want.Colors) {
				t.Fatalf("len(Colors) = %d, want %d", len(got.Colors), len(tt.want.Colors))
			}
			for i, c := range got.Colors {
				if c.Name != tt.want.Colors[i].Name || c.RGB != tt.want.Colors[i].RGB {
					t.Errorf("Colors[%d] = {%q, %q}, want {%q, %q}",
						i, c.Name, c.RGB, tt.want.Colors[i].Name, tt.want.Colors[i].RGB)
				}
			}
		})
	}
}

