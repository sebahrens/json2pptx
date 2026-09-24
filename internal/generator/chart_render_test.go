package generator

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func TestChartBackgroundTransparentUnlessAuthored(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "lt1", RGB: "#DDEEFF"},
		{Name: "lt2", RGB: "#EDF4FA"},
		{Name: "dk1", RGB: "#172536"},
		{Name: "accent1", RGB: "#355F95"},
	}
	spec := &types.DiagramSpec{Type: "bar_chart", Width: 320, Height: 220, Data: map[string]any{
		"categories": []string{"A"},
		"series":     []map[string]any{{"name": "Data", "values": []float64{1}}},
	}}
	for _, tc := range []struct {
		name, authoredBackground string
		wantAlpha                uint32
	}{
		{name: "theme contrast without fill", wantAlpha: 0},
		{name: "authored background", authoredBackground: "#DDEEFF", wantAlpha: 0xFFFF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.authoredBackground != "" {
				spec.Style = &types.DiagramStyle{Background: tc.authoredBackground}
			} else {
				spec.Style = nil
			}
			rendered, err := RenderDiagramSpecWithMetadata(spec, theme, 0, false)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(rendered.PNG))
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, alpha := img.At(img.Bounds().Min.X+10, img.Bounds().Min.Y+10).RGBA()
			if alpha != tc.wantAlpha {
				t.Errorf("chart corner alpha = %d, want %d", alpha, tc.wantAlpha)
			}
		})
	}
}

func TestNonChartDiagramKeepsThemeBackground(t *testing.T) {
	spec := &types.DiagramSpec{Type: "venn"}
	theme := []types.ThemeColor{{Name: "lt1", RGB: "#DDEEFF"}, {Name: "accent1", RGB: "#355F95"}}
	if got := diagramSpecToSVGGen(spec, theme, 0, "").Style.Background; got != "#DDEEFF" {
		t.Errorf("non-chart diagram background = %q, want theme lt1", got)
	}
}

func TestAbstractChartPaletteUsesVisibleFirstAccent(t *testing.T) {
	reader, err := template.OpenTemplate("../../templates/abstract.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	theme := template.ParseTheme(reader).Colors
	spec := &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{
		"categories": []string{"A"},
		"series":     []map[string]any{{"name": "Data", "values": []float64{1}}},
	}}
	req := diagramSpecToSVGGen(spec, theme, 0, "")
	guide := svggen.StyleGuideFromSpec(req.Style)
	if req.Style.Background != "transparent" {
		t.Errorf("unfilled chart background = %q, want transparent", req.Style.Background)
	}
	background := guide.Palette.Background
	for i, color := range guide.Palette.AccentColors() {
		if ratio := color.ContrastWith(background); ratio < 2 {
			t.Errorf("automatic chart series %d uses near-background %s at %.2f:1", i+1, color.Hex(), ratio)
		}
	}
	if got := guide.Palette.Accent1.Hex(); got != "#8E8172" {
		t.Errorf("first chart color = %s, want abstract theme accent1 #8E8172", got)
	}
	rendered, err := RenderDiagramSpecWithMetadata(spec, theme, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	svg := strings.ToUpper(string(rendered.SVG))
	if !strings.Contains(svg, "#8E8172") {
		t.Error("rendered chart does not use abstract accent1")
	}
	if strings.Contains(svg, "#E9E6DF") {
		t.Error("rendered chart still contains near-background abstract accent1")
	}
}

func TestChartStyleSchemeColorsResolveAgainstEffectiveTheme(t *testing.T) {
	caller := []types.ThemeColor{
		{Name: "accent1", RGB: "#112233"}, {Name: "accent3", RGB: "#445566"},
		{Name: "lt1", RGB: "#FFFFFF"}, {Name: "lt2", RGB: "#EEEEEE"},
	}
	spec := &types.DiagramSpec{Type: "bar_chart", Style: &types.DiagramStyle{
		Colors: []string{"accent3", "accent1"}, Background: "lt2",
	}, Data: map[string]any{"categories": []string{"A"}, "series": []map[string]any{{"name": "Data", "values": []float64{1}}}}}
	req := diagramSpecToSVGGen(spec, caller, 0, "")
	if got := svggen.StyleGuideFromSpec(req.Style).Palette.Accent1.Hex(); got != "#445566" {
		t.Errorf("first series = %s, want template accent3", got)
	}
	if got := req.Style.Background; got != "#EEEEEE" {
		t.Errorf("background = %s, want template lt2", got)
	}
	if got := DiagramStyleColorFindings(spec, caller); len(got) != 0 {
		t.Errorf("valid scheme colors should not be dropped: %+v", got)
	}

	// Per-diagram theme colors take precedence over the caller's template theme.
	spec.Style.ThemeColors = []types.ThemeColor{
		{Name: "accent1", RGB: "#778899"}, {Name: "accent3", RGB: "#AABBCC"},
		{Name: "lt1", RGB: "#FAFAFA"}, {Name: "lt2", RGB: "#F0F0F0"},
	}
	req = diagramSpecToSVGGen(spec, caller, 0, "")
	if got := svggen.StyleGuideFromSpec(req.Style).Palette.Accent1.Hex(); got != "#AABBCC" {
		t.Errorf("first series = %s, want per-diagram accent3", got)
	}
	if got := req.Style.Background; got != "#F0F0F0" {
		t.Errorf("background = %s, want per-diagram lt2", got)
	}
}

func TestChartStyleUnresolvedColorsFallBackWithFindings(t *testing.T) {
	theme := []types.ThemeColor{{Name: "accent1", RGB: "#123456"}, {Name: "lt1", RGB: "#FFFFFF"}}
	spec := &types.DiagramSpec{Type: "bar_chart", Style: &types.DiagramStyle{
		Colors: []string{"accent9"}, Background: "lt2",
	}, Data: map[string]any{"categories": []string{"A"}, "series": []map[string]any{{"name": "Data", "values": []float64{1}}}}}
	req := diagramSpecToSVGGen(spec, theme, 0, "")
	if got := svggen.StyleGuideFromSpec(req.Style).Palette.Accent1.Hex(); got != "#123456" {
		t.Errorf("unresolved accent fallback = %s, want template accent1", got)
	}
	if got := req.Style.Background; got != "transparent" {
		t.Errorf("unresolved background fallback = %s, want transparent", got)
	}
	findings := DiagramStyleColorFindings(spec, theme)
	if len(findings) != 2 || findings[0].Field != "style.colors[0]" || findings[1].Field != "style.background" {
		t.Fatalf("unresolved color findings = %+v", findings)
	}
	for _, f := range findings {
		if f.Code != "CUSTOM_COLOR_DROPPED" || f.Severity != "info" {
			t.Errorf("unresolved color finding = %+v", f)
		}
	}
	rendered, err := RenderDiagramSpecWithMetadata(spec, theme, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered.Findings) < 2 || rendered.Findings[len(rendered.Findings)-2].Code != "CUSTOM_COLOR_DROPPED" || rendered.Findings[len(rendered.Findings)-1].Code != "CUSTOM_COLOR_DROPPED" {
		t.Errorf("render-time dropped color findings = %+v", rendered.Findings)
	}
	if got := strings.ToUpper(string(rendered.SVG)); !strings.Contains(got, "#123456") {
		t.Error("rendered chart did not use the template accent fallback")
	}
	if _, ok := ResolveDiagramStyleColor("bad", theme); ok {
		t.Error("unrecognized scheme-like value was accepted as unprefixed shorthand hex")
	}
	if _, ok := ResolveDiagramStyleColor("accent1", nil); ok {
		t.Error("scheme name without an effective theme must be reported unresolved")
	}
	// Free-mode authored hex remains valid; constrained mode rejects it earlier.
	spec.Style.Colors = []string{"#ABCDEF"}
	spec.Style.Background = "#F5F5F5"
	if got := DiagramStyleColorFindings(spec, theme); len(got) != 0 {
		t.Errorf("valid hex colors should not be dropped: %+v", got)
	}
}

func TestChartStyleSevenSeriesColorsSurviveThemeBridge(t *testing.T) {
	colors := []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "#BADA55"}
	theme := []types.ThemeColor{
		{Name: "accent1", RGB: "#110000"}, {Name: "accent2", RGB: "#220000"},
		{Name: "accent3", RGB: "#330000"}, {Name: "accent4", RGB: "#440000"},
		{Name: "accent5", RGB: "#550000"}, {Name: "accent6", RGB: "#660000"},
		{Name: "lt1", RGB: "#FFFFFF"}, {Name: "lt2", RGB: "#EEEEEE"},
	}
	series := make([]map[string]any, len(colors))
	for i := range series {
		series[i] = map[string]any{"name": "Series " + string(rune('A'+i)), "values": []float64{float64(i + 1)}}
	}
	spec := &types.DiagramSpec{Type: "bar_chart", Style: &types.DiagramStyle{Colors: colors},
		Data: map[string]any{"categories": []string{"A"}, "series": series}}
	req := diagramSpecToSVGGen(spec, theme, 0, "")
	accents := svggen.StyleGuideFromSpec(req.Style).Palette.AccentColors()
	if len(accents) != 7 {
		t.Fatalf("accent count = %d, want 7", len(accents))
	}
	if got := accents[6].Hex(); got != "#BADA55" {
		t.Errorf("seventh series accent = %s, want #BADA55", got)
	}
	rendered, err := RenderDiagramSpecWithMetadata(spec, theme, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(string(rendered.SVG)), "#BADA55") {
		t.Error("rendered chart lost the seventh authored series color")
	}
	if got := DiagramStyleColorFindings(spec, theme); len(got) != 0 {
		t.Errorf("seven valid colors should not be dropped: %+v", got)
	}
	// An unresolved seventh entry must be reported, not silently cycled.
	spec.Style.Colors[6] = "accent9"
	findings := DiagramStyleColorFindings(spec, theme)
	if len(findings) != 1 || findings[0].Field != "style.colors[6]" {
		t.Errorf("unresolved seventh color findings = %+v", findings)
	}
}

func TestDiagramBridgePreservesTemplateSemanticAccents(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "accent1", RGB: "#C00000"},
		{Name: "accent2", RGB: "#777777"},
		{Name: "accent3", RGB: "#008000"},
	}
	spec := &types.DiagramSpec{Type: "gantt", Style: &types.DiagramStyle{
		SemanticAccents: map[string]string{"positive": "accent3", "negative": "accent1", "neutral": "accent2"},
		DataPalette:     []string{"#123456", "#654321", "#ABCDEF"},
	}}
	guide := svggen.StyleGuideFromSpec(diagramSpecToSVGGen(spec, theme, 0, "").Style)
	if got := guide.Palette.Success.Hex(); got != "#008000" {
		t.Errorf("positive = %s", got)
	}
	if got := guide.Palette.Error.Hex(); got != "#C00000" {
		t.Errorf("negative = %s", got)
	}
	if got := guide.Palette.Warning.Hex(); got != "#777777" {
		t.Errorf("neutral = %s", got)
	}
}

func TestPlaceholderDiagramReceivesTemplateSemanticAccents(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.svgConverter = NewSVGConverterWithConfig(SVGConfig{Strategy: SVGStrategyNative, Scale: DefaultSVGScale})
	ctx.themeColors = []types.ThemeColor{
		{Name: "accent1", RGB: "#C00000"},
		{Name: "accent2", RGB: "#777777"},
		{Name: "accent3", RGB: "#008000"},
	}
	ctx.semanticAccents = map[string]string{"positive": "accent3", "negative": "accent1", "neutral": "accent2"}
	spec := &types.DiagramSpec{Type: "gantt", Data: map[string]any{
		"show_progress": true,
		"tasks":         []any{map[string]any{"name": "Task", "start": "2024-01-01", "end": "2024-02-01", "progress": 100.0}},
	}}
	item := ContentItem{PlaceholderID: "body", Type: ContentDiagram, Value: spec}
	bounds := types.BoundingBox{Width: 5_000_000, Height: 3_000_000}
	result, ok := ctx.resolveDiagramWithMetadata(1, item, bounds)
	if !ok {
		t.Fatalf("diagram failed: %v", ctx.warnings)
	}
	if got := spec.Style.SemanticAccents["positive"]; got != "accent3" {
		t.Errorf("injected positive = %q", got)
	}
	if !strings.Contains(strings.ToLower(string(result.SVG)), `fill="#008000"`) && !strings.Contains(strings.ToLower(string(result.SVG)), `fill="#080"`) {
		t.Error("completed Gantt segment does not use template's positive accent")
	}
}

func TestExplicitChartColorsRemainAuthorControlled(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "accent1", RGB: "#E9E6DF"},
		{Name: "accent2", RGB: "#44546A"},
	}
	spec := &types.DiagramSpec{Type: "bar_chart", Style: &types.DiagramStyle{
		Colors: []string{"#E9E6DF", "#44546A"},
	}}
	req := diagramSpecToSVGGen(spec, theme, 0, "")
	if got := svggen.StyleGuideFromSpec(req.Style).Palette.Accent1.Hex(); got != "#E9E6DF" {
		t.Errorf("explicit chart color rewritten to %s", got)
	}
}

func TestVisibleChartPaletteDarkAndDegenerateThemes(t *testing.T) {
	tests := []struct {
		name       string
		theme      []svggen.ThemeColorInput
		background string
		wantFirst  string
	}{
		{
			name: "dark canvas uses light theme slot",
			theme: []svggen.ThemeColorInput{
				{Name: "accent1", RGB: "#101010"},
				{Name: "dk1", RGB: "#000000"},
				{Name: "lt1", RGB: "#FFFFFF"},
			},
			background: "#000000", wantFirst: "#FFFFFF",
		},
		{
			name:       "malformed theme falls back to distinct chart colors",
			theme:      []svggen.ThemeColorInput{{Name: "accent1", RGB: "not-a-color"}},
			background: "#FFFFFF", wantFirst: svggen.DefaultPalette().Accent1.Hex(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := visibleChartPalette(tc.theme, nil, tc.background)
			if len(got) != 6 || got[0] != tc.wantFirst {
				t.Errorf("palette = %v, want six visible colors led by %s", got, tc.wantFirst)
			}
			seen := map[string]bool{}
			background := svggen.MustParseColor(tc.background)
			for i, hex := range got {
				if seen[hex] {
					t.Errorf("color %d repeats %s despite fallback palette", i, hex)
				}
				seen[hex] = true
				if ratio := svggen.MustParseColor(hex).ContrastWith(background); ratio < 2 {
					t.Errorf("color %d = %s has %.2f:1 contrast", i, hex, ratio)
				}
			}
		})
	}
}

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

	// Theme lt1 remains the contrast reference, not a painted chart backdrop.
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
		if result.Style.Background != "transparent" {
			t.Errorf("Background = %q, want transparent", result.Style.Background)
		}
		if got := svggen.StyleGuideFromSpec(result.Style).Palette.Background.Hex(); got != "#FFFFFF" {
			t.Errorf("contrast background = %q, want lt1", got)
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
					{Name: "accent1", RGB: "#4575A0"},
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
		if result.Style.Background != "transparent" {
			t.Errorf("Background = %q, want transparent", result.Style.Background)
		}
		if got := svggen.StyleGuideFromSpec(result.Style).Palette.Background.Hex(); got != "#101010" {
			t.Errorf("contrast background = %q, want spec lt1", got)
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
		// ThemeColors still supplies the lt1 contrast reference even though the
		// actual chart canvas remains transparent.
		if result.Style.Background != "transparent" {
			t.Errorf("Background = %q, want transparent", result.Style.Background)
		}
		if got := svggen.StyleGuideFromSpec(result.Style).Palette.Background.Hex(); got != "#FAFAFA" {
			t.Errorf("contrast background = %q, want lt1", got)
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
// colors. The theme source stays raw for diagram/native parity, while chart
// series skip theme colors that nearly disappear on the background. It also
// confirms explicit colors and data_palette ordering survive that filter.
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
	accentHexes := func(p *svggen.Palette) []string {
		return []string{
			p.Accent1.Hex(), p.Accent2.Hex(), p.Accent3.Hex(),
			p.Accent4.Hex(), p.Accent5.Hex(), p.Accent6.Hex(),
		}
	}

	// The raw theme inputs stay intact, but chart series skip near-background
	// accents and preserve the order of the visible ones.
	t.Run("caller_theme_colors_filter_invisible_chart_accents", func(t *testing.T) {
		spec := &types.DiagramSpec{
			Type: "bar_chart",
			Data: map[string]any{"categories": []string{"A"}, "values": []float64{1}},
		}
		result := diagramSpecToSVGGen(spec, lowContrastTheme, 0, "")
		if !result.Style.DisablePaletteEnforcement {
			t.Fatal("DisablePaletteEnforcement = false, want true for embedded theme-color path")
		}

		for i, name := range []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"} {
			if result.Style.ThemeColors[i+2].Name != name || result.Style.ThemeColors[i+2].RGB != lowContrastTheme[i+2].RGB {
				t.Fatalf("source theme %s was changed", name)
			}
		}
		rawGuide := svggen.StyleGuideFromSpec(result.Style)
		got := accentHexes(rawGuide.Palette)
		want := []string{"#FD5108", "#FE7C39", "#A1A8B3", "#000000"}
		for i, want := range want {
			if got[i] != want {
				t.Errorf("chart Accent%d = %s, want visible %s", i+1, got[i], want)
			}
		}
		background := svggen.MustParseColor("#FFFFFF")
		for i, hex := range got {
			if ratio := svggen.MustParseColor(hex).ContrastWith(background); ratio < 2 {
				t.Errorf("chart Accent%d is only %.2f:1 on white", i+1, ratio)
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

// TestValueFormatSurvivesTheBridge pins the go-slide-creator-e2ck9 plumbing: a
// chart's value_format must reach svggen, where it formats the axis ticks and
// the data labels alike. Without the copy the field is accepted by the schema
// and silently ignored, which is worse than rejecting it.
func TestValueFormatSurvivesTheBridge(t *testing.T) {
	decimals := 1
	sep := false
	spec := &types.DiagramSpec{
		Type: "bar_chart",
		Data: map[string]any{"categories": []string{"A"}, "series": []any{}},
		Style: &types.DiagramStyle{
			ValueFormat: &types.ValueFormatSpec{
				Style:        "compact",
				Decimals:     &decimals,
				Prefix:       "€",
				Suffix:       " ARR",
				ThousandsSep: &sep,
			},
		},
	}
	got := diagramSpecToSVGGen(spec, nil, 0, "").Style.ValueFormat
	if got == nil {
		t.Fatal("StyleSpec.ValueFormat = nil; the bridge dropped value_format")
	}
	if got.Style != "compact" || got.Prefix != "€" || got.Suffix != " ARR" {
		t.Errorf("value_format notation not forwarded: %+v", got)
	}
	if got.Decimals == nil || *got.Decimals != 1 {
		t.Errorf("decimals not forwarded: %+v", got.Decimals)
	}
	if got.ThousandsSep == nil || *got.ThousandsSep {
		t.Errorf("thousands_sep not forwarded: %+v", got.ThousandsSep)
	}

	// Absent stays absent: a nil spec must not materialise as an empty struct,
	// which would read as "the caller chose plain" and suppress the defaults.
	bare := &types.DiagramSpec{
		Type:  "bar_chart",
		Data:  map[string]any{"categories": []string{"A"}, "series": []any{}},
		Style: &types.DiagramStyle{ShowValues: true},
	}
	if diagramSpecToSVGGen(bare, nil, 0, "").Style.ValueFormat != nil {
		t.Error("missing value_format should leave StyleSpec.ValueFormat nil")
	}
}

// TestChartSpecValueFormatReachesTheDiagramSpec covers the older ChartSpec
// surface: chart_value.style.value_format has to survive ToDiagramSpec, which is
// the hop every raw deck's chart takes.
func TestChartSpecValueFormatReachesTheDiagramSpec(t *testing.T) {
	cs := &types.ChartSpec{
		Type: "bar",
		Data: map[string]any{"A": 1.0},
		Style: &types.ChartStyle{
			ShowValues:  true,
			Scale:       "log",
			ValueFormat: &types.ValueFormatSpec{Style: "currency", Prefix: "$"},
		},
	}
	ds := cs.ToDiagramSpec()
	if ds.Style == nil || ds.Style.ValueFormat == nil {
		t.Fatal("ToDiagramSpec dropped style.value_format")
	}
	if ds.Style.ValueFormat.Style != "currency" || ds.Style.ValueFormat.Prefix != "$" {
		t.Errorf("value_format = %+v", ds.Style.ValueFormat)
	}
	if ds.Style.Scale != "log" {
		t.Errorf("scale = %q, want log", ds.Style.Scale)
	}
}

func TestChartScaleReachesSVGGen(t *testing.T) {
	chart := &types.ChartSpec{
		Type: "bar", Style: &types.ChartStyle{Scale: "log"},
		Data: map[string]any{
			"categories": []any{"A", "B"},
			"series":     []any{map[string]any{"name": "S", "values": []any{1.0, 10000.0}}},
		},
	}
	req := diagramSpecToSVGGen(chart.ToDiagramSpec(), nil, 0, "")
	if req.Style.Scale != "log" {
		t.Fatalf("SVG request scale = %q, want log", req.Style.Scale)
	}
	out, err := svggen.RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatal(err)
	}
	if out.SVG == nil || !strings.Contains(string(out.SVG.Bytes()), "Log scale") {
		t.Error("deck chart did not render a visibly labelled log axis")
	}
}
