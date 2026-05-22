package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func TestDiagramSpecToSVGGen(t *testing.T) {
	tests := []struct {
		name        string
		spec        *types.DiagramSpec
		themeColors []types.ThemeColor
		wantType    string
		wantFitMode string
		wantWidth   int
		wantHeight  int
	}{
		{
			name: "basic diagram spec",
			spec: &types.DiagramSpec{
				Type:  "bar_chart",
				Title: "Test Chart",
				Data:  map[string]any{"categories": []string{"A", "B"}, "values": []float64{10, 20}},
			},
			wantType:    "bar_chart",
			wantFitMode: "",
			wantWidth:   types.DefaultChartWidth,
			wantHeight:  types.DefaultChartHeight,
		},
		{
			name: "diagram spec with FitMode contain",
			spec: &types.DiagramSpec{
				Type:    "pie_chart",
				Title:   "Pie Chart",
				Data:    map[string]any{"categories": []string{"A"}, "values": []float64{100}},
				FitMode: "contain",
			},
			wantType:    "pie_chart",
			wantFitMode: "contain",
			wantWidth:   types.DefaultChartWidth,
			wantHeight:  types.DefaultChartHeight,
		},
		{
			name: "diagram spec with FitMode cover",
			spec: &types.DiagramSpec{
				Type:    "donut_chart",
				Data:    map[string]any{"categories": []string{"A"}, "values": []float64{100}},
				FitMode: "cover",
			},
			wantType:    "donut_chart",
			wantFitMode: "cover",
			wantWidth:   types.DefaultChartWidth,
			wantHeight:  types.DefaultChartHeight,
		},
		{
			name: "diagram spec with custom dimensions and FitMode",
			spec: &types.DiagramSpec{
				Type:    "radar_chart",
				Data:    map[string]any{"categories": []string{"A", "B", "C"}, "values": []float64{10, 20, 30}},
				Width:   1024,
				Height:  768,
				FitMode: "contain",
			},
			wantType:    "radar_chart",
			wantFitMode: "contain",
			wantWidth:   1024,
			wantHeight:  768,
		},
		{
			name: "diagram spec with subtitle passthrough",
			spec: &types.DiagramSpec{
				Type:     "bar_chart",
				Title:    "Revenue by Region",
				Subtitle: "Note: APAC includes Japan launch in Q3",
				Data:     map[string]any{"categories": []string{"EMEA", "Americas"}, "values": []float64{45, 62}},
			},
			wantType:   "bar_chart",
			wantWidth:  types.DefaultChartWidth,
			wantHeight: types.DefaultChartHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := diagramSpecToSVGGen(tt.spec, tt.themeColors, 0, "")

			if result.Type != tt.wantType {
				t.Errorf("diagramSpecToSVGGen() Type = %v, want %v", result.Type, tt.wantType)
			}
			if result.Output.FitMode != tt.wantFitMode {
				t.Errorf("diagramSpecToSVGGen() Output.FitMode = %v, want %v", result.Output.FitMode, tt.wantFitMode)
			}
			if result.Output.Width != tt.wantWidth {
				t.Errorf("diagramSpecToSVGGen() Output.Width = %v, want %v", result.Output.Width, tt.wantWidth)
			}
			if result.Output.Height != tt.wantHeight {
				t.Errorf("diagramSpecToSVGGen() Output.Height = %v, want %v", result.Output.Height, tt.wantHeight)
			}
			if result.Output.Format != "png" {
				t.Errorf("diagramSpecToSVGGen() Output.Format = %v, want png", result.Output.Format)
			}
			if result.Output.Scale != 2.0 {
				t.Errorf("diagramSpecToSVGGen() Output.Scale = %v, want 2.0", result.Output.Scale)
			}
			// Verify subtitle passthrough
			if result.Subtitle != tt.spec.Subtitle {
				t.Errorf("diagramSpecToSVGGen() Subtitle = %q, want %q", result.Subtitle, tt.spec.Subtitle)
			}
		})
	}

	// Verify lt1 (Background) and lt2 (Surface) are forwarded to StyleSpec
	// so svggen's contrast pipeline matches native enforceTextContrastInSlide
	// on tinted-surface templates (wbc7.6).
	t.Run("background_and_surface_forwarded_from_theme", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
		}
		theme := []types.ThemeColor{
			{Name: "dk1", RGB: "#000000"},
			{Name: "lt1", RGB: "#FFFFFF"},
			{Name: "lt2", RGB: "#F5EFE0"}, // beige surface
			{Name: "accent1", RGB: "#336699"},
		}
		result := diagramSpecToSVGGen(spec, theme, 0, "")
		if result.Style.Background != "#FFFFFF" {
			t.Errorf("Background = %q, want %q (lt1)", result.Style.Background, "#FFFFFF")
		}
		if result.Style.Surface != "#F5EFE0" {
			t.Errorf("Surface = %q, want %q (lt2)", result.Style.Surface, "#F5EFE0")
		}
	})

	// Explicit spec.Style.Background must still win over theme lt1.
	t.Run("explicit_style_background_overrides_lt1", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
			Style: &types.DiagramStyle{
				Background: "#112233",
			},
		}
		theme := []types.ThemeColor{
			{Name: "lt1", RGB: "#FFFFFF"},
			{Name: "lt2", RGB: "#F5EFE0"},
		}
		result := diagramSpecToSVGGen(spec, theme, 0, "")
		if result.Style.Background != "#112233" {
			t.Errorf("Background = %q, want explicit %q", result.Style.Background, "#112233")
		}
		if result.Style.Surface != "#F5EFE0" {
			t.Errorf("Surface = %q, want %q (lt2)", result.Style.Surface, "#F5EFE0")
		}
	})

	// Per-spec ThemeColors take priority over caller themeColors.
	t.Run("spec_theme_colors_take_priority", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
			Style: &types.DiagramStyle{
				ThemeColors: []types.ThemeColor{
					{Name: "lt1", RGB: "#101010"},
					{Name: "lt2", RGB: "#202020"},
				},
			},
		}
		caller := []types.ThemeColor{
			{Name: "lt1", RGB: "#FFFFFF"},
			{Name: "lt2", RGB: "#EEEEEE"},
		}
		result := diagramSpecToSVGGen(spec, caller, 0, "")
		if result.Style.Background != "#101010" {
			t.Errorf("Background = %q, want %q (spec lt1)", result.Style.Background, "#101010")
		}
		if result.Style.Surface != "#202020" {
			t.Errorf("Surface = %q, want %q (spec lt2)", result.Style.Surface, "#202020")
		}
	})

	// When spec.Style.Colors is supplied, accents must be synthesized as
	// ThemeColors so svggen still gets dk/lt slots from the effective theme
	// (wbc7.7). Otherwise svggen took the PaletteSpec.Colors branch and
	// silently fell back to DefaultPalette for text/bg/surface.
	t.Run("style_colors_promote_to_theme_colors_with_dk_lt", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
			Style: &types.DiagramStyle{
				Colors: []string{"#AA0000", "#00AA00", "#0000AA"},
			},
		}
		theme := []types.ThemeColor{
			{Name: "dk1", RGB: "#111111"},
			{Name: "dk2", RGB: "#222222"},
			{Name: "lt1", RGB: "#FAFAFA"},
			{Name: "lt2", RGB: "#EFEFEF"},
			{Name: "accent1", RGB: "#998877"}, // must be overridden by spec.Style.Colors[0]
		}
		result := diagramSpecToSVGGen(spec, theme, 0, "")
		if len(result.Style.Palette.Colors) != 0 {
			t.Errorf("Style.Palette.Colors = %v, want empty (route should use ThemeColors instead)", result.Style.Palette.Colors)
		}
		// Build lookup of synthesized ThemeColors.
		got := map[string]string{}
		for _, tc := range result.Style.ThemeColors {
			got[tc.Name] = tc.RGB
		}
		wantAccents := map[string]string{
			"accent1": "#AA0000",
			"accent2": "#00AA00",
			"accent3": "#0000AA",
		}
		for name, rgb := range wantAccents {
			if got[name] != rgb {
				t.Errorf("ThemeColors[%s] = %q, want %q", name, got[name], rgb)
			}
		}
		for _, name := range []string{"dk1", "dk2", "lt1", "lt2"} {
			if got[name] == "" {
				t.Errorf("ThemeColors missing %s (must be forwarded from effective theme)", name)
			}
		}
		// And the dedicated Background/Surface fields must still be set by
		// the lookupBackgroundAndSurface pass so contrast calculations match.
		if result.Style.Background != "#FAFAFA" {
			t.Errorf("Background = %q, want %q (lt1)", result.Style.Background, "#FAFAFA")
		}
		if result.Style.Surface != "#EFEFEF" {
			t.Errorf("Surface = %q, want %q (lt2)", result.Style.Surface, "#EFEFEF")
		}
	})

	// Verify StrictFit is threaded to OutputSpec.
	t.Run("strict_fit_threaded", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "series": []any{}},
		}
		result := diagramSpecToSVGGen(spec, nil, 0, "strict")
		if result.Output.StrictFit != "strict" {
			t.Errorf("StrictFit = %q, want %q", result.Output.StrictFit, "strict")
		}
		resultOff := diagramSpecToSVGGen(spec, nil, 0, "")
		if resultOff.Output.StrictFit != "" {
			t.Errorf("StrictFit = %q, want empty", resultOff.Output.StrictFit)
		}
	})

	// Verify per-slide chart_style overrides survive the bridge so svggen
	// can honour them. Without this both fields silently default to nil and
	// the override is ignored.
	t.Run("chart_style_overrides_threaded", func(t *testing.T) {
		tru := true
		fls := false
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "series": []any{}},
			ChartStyle: &types.ChartStyleOverrides{
				ShowVerticalGridlines:  &tru,
				ShowSingleSeriesLegend: &fls,
			},
		}
		result := diagramSpecToSVGGen(spec, nil, 0, "")
		if result.Style.ChartStyle == nil {
			t.Fatal("StyleSpec.ChartStyle = nil; bridge dropped per-slide override")
		}
		if result.Style.ChartStyle.ShowVerticalGridlines == nil || !*result.Style.ChartStyle.ShowVerticalGridlines {
			t.Errorf("ShowVerticalGridlines not forwarded: %+v", result.Style.ChartStyle.ShowVerticalGridlines)
		}
		if result.Style.ChartStyle.ShowSingleSeriesLegend == nil || *result.Style.ChartStyle.ShowSingleSeriesLegend {
			t.Errorf("ShowSingleSeriesLegend should be &false, got: %+v", result.Style.ChartStyle.ShowSingleSeriesLegend)
		}

		// Absent override must remain absent — never materialise into an
		// empty non-nil struct.
		specNoOverride := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "series": []any{}},
		}
		if diagramSpecToSVGGen(specNoOverride, nil, 0, "").Style.ChartStyle != nil {
			t.Error("missing chart_style should leave StyleSpec.ChartStyle nil")
		}
	})
}

// TestDiagramSpecToSVGGen_RawPalette verifies that the embedded PPTX bridge
// opts out of svggen's accent-contrast enforcement whenever it routes theme
// colors, so diagram accents render with the same raw schemeClr hex as native
// shape_grid fills (go-slide-creator-gmv5). It also confirms standalone-style
// usage (no theme colors) leaves the default in place and that data_palette
// ordering survives the raw path.
func TestDiagramSpecToSVGGen_RawPalette(t *testing.T) {
	// Warm, low-contrast accent set: known to be mutated by EnforceAccentContrast
	// (mirrors svggen TestStyleGuideFromSpec_DisablePaletteEnforcement fixture).
	lowContrastTheme := []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "accent1", RGB: "#FD5108"},
		{Name: "accent2", RGB: "#FE7C39"},
		{Name: "accent3", RGB: "#FFAA72"},
		{Name: "accent4", RGB: "#A1A8B3"},
		{Name: "accent5", RGB: "#B5BCC4"},
		{Name: "accent6", RGB: "#CBD1D6"},
	}
	rawAccents := []string{"#FD5108", "#FE7C39", "#FFAA72", "#A1A8B3", "#B5BCC4", "#CBD1D6"}

	accentHexes := func(p *svggen.Palette) []string {
		return []string{
			p.Accent1.Hex(), p.Accent2.Hex(), p.Accent3.Hex(),
			p.Accent4.Hex(), p.Accent5.Hex(), p.Accent6.Hex(),
		}
	}

	// Caller-supplied template theme colors must flip on raw-theme parity, and
	// the resulting palette must preserve every accent hex verbatim.
	t.Run("caller_theme_colors_preserve_raw_accents", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
		}
		result := diagramSpecToSVGGen(spec, lowContrastTheme, 0, "")
		if !result.Style.DisablePaletteEnforcement {
			t.Fatal("DisablePaletteEnforcement = false, want true for embedded theme-color path")
		}

		// Precondition: with enforcement re-enabled, the same fixture is mutated,
		// proving the flag does real work rather than passing trivially.
		enforced := result.Style
		enforced.DisablePaletteEnforcement = false
		enforcedGuide := svggen.StyleGuideFromSpec(enforced)
		if equalStringSlices(accentHexes(enforcedGuide.Palette), rawAccents) {
			t.Fatal("test precondition failed: enforcement left fixture unchanged; pick a more aggressive palette")
		}

		rawGuide := svggen.StyleGuideFromSpec(result.Style)
		got := accentHexes(rawGuide.Palette)
		for i, want := range rawAccents {
			if got[i] != want {
				t.Errorf("Accent%d = %s, want raw %s (must match native schemeClr hex)", i+1, got[i], want)
			}
		}
	})

	// Explicit spec.Style.Colors are promoted to ThemeColors, so the same
	// raw-theme parity must apply — user-chosen accents render verbatim too.
	t.Run("explicit_style_colors_disable_enforcement", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
			Style: &types.DiagramStyle{
				Colors: []string{"#FD5108", "#FE7C39", "#FFAA72"},
			},
		}
		result := diagramSpecToSVGGen(spec, lowContrastTheme, 0, "")
		if !result.Style.DisablePaletteEnforcement {
			t.Fatal("DisablePaletteEnforcement = false, want true when spec.Style.Colors drive accents")
		}
		guide := svggen.StyleGuideFromSpec(result.Style)
		if h := guide.Palette.Accent1.Hex(); h != "#FD5108" {
			t.Errorf("Accent1 = %s, want raw %s (explicit user color must be preserved)", h, "#FD5108")
		}
	})

	// data_palette ordering must survive the raw path: each entry maps to the
	// corresponding accent slot, in order, with the exact input hex.
	t.Run("data_palette_ordering_preserved", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
			Style: &types.DiagramStyle{
				DataPalette: []string{"#101010", "#202020", "#303030"},
			},
		}
		result := diagramSpecToSVGGen(spec, lowContrastTheme, 0, "")
		if !result.Style.DisablePaletteEnforcement {
			t.Fatal("DisablePaletteEnforcement = false, want true for embedded theme-color path")
		}
		guide := svggen.StyleGuideFromSpec(result.Style)
		want := []string{"#101010", "#202020", "#303030"}
		got := []string{guide.Palette.Accent1.Hex(), guide.Palette.Accent2.Hex(), guide.Palette.Accent3.Hex()}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Accent%d = %s, want %s (data_palette ordering must be preserved)", i+1, got[i], want[i])
			}
		}
	})

	// Standalone-style usage (no template theme colors) must NOT touch the flag,
	// so svggen keeps its readability-enforcement default outside the bridge.
	t.Run("no_theme_colors_keeps_default", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
		}
		result := diagramSpecToSVGGen(spec, nil, 0, "")
		if result.Style.DisablePaletteEnforcement {
			t.Error("DisablePaletteEnforcement = true, want false when no theme colors are routed")
		}
	})
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRenderDiagramSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *types.DiagramSpec
		wantErr bool
	}{
		{
			name:    "nil spec",
			spec:    nil,
			wantErr: true,
		},
		{
			name: "empty type",
			spec: &types.DiagramSpec{
				Type: "",
				Data: map[string]any{"categories": []string{"A"}, "values": []float64{10}},
			},
			wantErr: true,
		},
		{
			name: "bar chart with FitMode",
			spec: &types.DiagramSpec{
				Type:    "bar_chart",
				Title:   "FitMode Test",
				Data:    map[string]any{"categories": []string{"A", "B"}, "series": []map[string]any{{"name": "Data", "values": []float64{10, 20}}}},
				FitMode: "contain",
			},
			wantErr: false,
		},
		{
			name: "pie chart with FitMode contain",
			spec: &types.DiagramSpec{
				Type:    "pie_chart",
				Title:   "Pie FitMode",
				Data:    map[string]any{"categories": []string{"A", "B"}, "values": []float64{40, 60}},
				Width:   800,
				Height:  400,
				FitMode: "contain",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderDiagramSpec(tt.spec, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RenderDiagramSpec() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("RenderDiagramSpec() unexpected error: %v", err)
				return
			}

			// Verify we got PNG data
			if len(result) == 0 {
				t.Errorf("RenderDiagramSpec() returned empty result")
				return
			}

			// Verify PNG signature (first 8 bytes)
			if len(result) < 8 {
				t.Errorf("RenderDiagramSpec() result too short for PNG")
				return
			}

			pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
			for i, b := range pngSignature {
				if result[i] != b {
					t.Errorf("RenderDiagramSpec() result is not a valid PNG (byte %d: got %02x, want %02x)", i, result[i], b)
					return
				}
			}

			t.Logf("Successfully rendered %s diagram: %d bytes", tt.spec.Type, len(result))
		})
	}
}

func TestRenderDiagramSpecWithMetadata_Treemap(t *testing.T) {
	// Simulates the full e2e path: chartutil.BuildChartDataPayload → DiagramSpec → RenderDiagramSpecWithMetadata
	// buildLabelValuePoints returns []map[string]any (NOT []any), which Go cannot type-assert to []any.
	// This was the root cause of go-slide-creator-wionh (treemap "Data unavailable").
	spec := &types.DiagramSpec{
		Type:  "treemap_chart",
		Title: "treemap Performance",
		Data: map[string]any{
			"values": []map[string]any{
				{"label": "Technology", "value": 35.0},
				{"label": "Healthcare", "value": 25.0},
				{"label": "Finance", "value": 20.0},
				{"label": "Energy", "value": 12.0},
				{"label": "Consumer", "value": 8.0},
			},
		},
		Width:  800,
		Height: 600,
	}

	result, err := RenderDiagramSpecWithMetadata(spec, nil, 0, false)
	if err != nil {
		t.Fatalf("RenderDiagramSpecWithMetadata() error = %v", err)
	}
	if result == nil || len(result.PNG) == 0 {
		t.Fatal("Expected non-empty PNG result")
	}
}
