// Package generator provides PPTX file generation from slide specifications.
// This file implements diagram rendering by calling svggen directly.
package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// DiagramRenderResult contains rendered image data and fit metadata for PPTX embedding.
// When FitMode is "contain" (auto-applied for all charts), the content dimensions
// and offsets are used to properly position the image within the placeholder.
type DiagramRenderResult struct {
	// SVG is the rendered SVG XML content (for native OOXML embedding via asvg:svgBlip).
	SVG []byte

	// PNG is the rendered image bytes (used as fallback for native SVG, or primary for raster strategy).
	PNG []byte

	// ContentWidth is the actual content width in pixels (after fit mode applied).
	ContentWidth float64

	// ContentHeight is the actual content height in pixels (after fit mode applied).
	ContentHeight float64

	// OffsetX is the horizontal offset to center the content in the container (pixels).
	OffsetX float64

	// OffsetY is the vertical offset to center the content in the container (pixels).
	OffsetY float64

	// FitMode indicates the fit mode used ("contain", "cover", or "" for stretch).
	FitMode string

	// ContainerWidth is the original container width before fit mode (pixels).
	ContainerWidth float64

	// ContainerHeight is the original container height before fit mode (pixels).
	ContainerHeight float64

	// Findings contains structured render-time findings from the chart/diagram
	// render (e.g., clamped values, data density warnings). Always non-nil
	// (empty slice when no findings). Consumed by the fit-report pipeline.
	Findings []svggen.Finding
}

// RenderDiagramSpec renders a DiagramSpec directly using svggen and returns the PNG bytes.
// This is the unified rendering function for all diagram types (charts and infographics).
//
// The themeColors parameter allows injecting template colors for consistent styling.
// Returns PNG bytes suitable for embedding in a PPTX document.
func RenderDiagramSpec(spec *types.DiagramSpec, themeColors []types.ThemeColor) ([]byte, error) {
	result, err := renderDiagramSpecFull(spec, themeColors, 0, false, "")
	if err != nil {
		return nil, err
	}
	return result.PNG, nil
}

// RenderDiagramSpecWithMetadata renders a DiagramSpec and returns both image bytes and fit metadata.
// This is used for proper PPTX embedding with contain-mode positioning.
//
// The returned DiagramRenderResult contains:
//   - SVG bytes for native embedding (when svgOnly or both formats requested)
//   - PNG bytes for the image (when svgOnly is false)
//   - ContentWidth/ContentHeight: the actual image dimensions (may differ from container for "contain" mode)
//   - OffsetX/OffsetY: positioning offset to center the image in the placeholder
//   - FitMode: the fit mode that was applied
//
// The themeColors parameter allows injecting template colors for consistent styling.
// maxPNGWidth caps the PNG pixel width (0 = no cap).
// When svgOnly is true, only SVG is rendered (no rasterization), which eliminates the
// tdewolff/canvas mutex bottleneck for diagrams. The caller must supply a fallback PNG
// (e.g., the 1x1 transparent constant) when embedding with native SVG strategy.
func RenderDiagramSpecWithMetadata(spec *types.DiagramSpec, themeColors []types.ThemeColor, maxPNGWidth int, svgOnly bool) (*DiagramRenderResult, error) {
	return renderDiagramSpecFull(spec, themeColors, maxPNGWidth, svgOnly, "")
}

// renderDiagramSpecFull is the internal implementation that accepts strictFit.
func renderDiagramSpecFull(spec *types.DiagramSpec, themeColors []types.ThemeColor, maxPNGWidth int, svgOnly bool, strictFit string) (*DiagramRenderResult, error) {
	if spec == nil {
		return nil, fmt.Errorf("diagram spec is required")
	}

	if spec.Type == "" {
		return nil, fmt.Errorf("diagram type is required")
	}

	// Convert DiagramSpec to svggen RequestEnvelope
	req := diagramSpecToSVGGen(spec, themeColors, maxPNGWidth, strictFit)

	// Determine which formats to request from svggen.
	// When svgOnly is true (native SVG strategy), skip PNG rasterization entirely.
	var formats []string
	if svgOnly {
		formats = []string{"svg"}
	} else {
		formats = []string{"svg", "png"}
	}

	output, err := svggen.RenderMultiFormatWithFindings(req, formats...)
	if err != nil {
		return nil, fmt.Errorf("svggen render failed: %w", err)
	}

	// When PNG was requested, verify we got data back.
	if !svgOnly && len(output.PNG) == 0 {
		return nil, fmt.Errorf("svggen returned empty PNG data")
	}

	// Extract fit metadata from SVGDocument
	renderResult := &DiagramRenderResult{
		PNG:      output.PNG,
		Findings: append(output.Findings, DiagramStyleColorFindings(spec, themeColors)...),
	}

	// Include SVG data for native embedding
	if output.SVG != nil && len(output.SVG.Content) > 0 {
		renderResult.SVG = output.SVG.Content
	}

	if output.SVG != nil {
		renderResult.FitMode = output.SVG.FitMode
		renderResult.ContentWidth = output.SVG.Width
		renderResult.ContentHeight = output.SVG.Height
		renderResult.OffsetX = output.SVG.OffsetX
		renderResult.OffsetY = output.SVG.OffsetY
		renderResult.ContainerWidth = output.SVG.ContainerWidth
		renderResult.ContainerHeight = output.SVG.ContainerHeight
	}

	return renderResult, nil
}

// diagramSpecToSVGGen converts a types.DiagramSpec to an svggen.RequestEnvelope.
// maxPNGWidth caps the PNG output width (0 = no cap).
// strictFit is threaded to OutputSpec.StrictFit for future severity promotion.
func diagramSpecToSVGGen(spec *types.DiagramSpec, themeColors []types.ThemeColor, maxPNGWidth int, strictFit string) *svggen.RequestEnvelope { //nolint:gocognit,gocyclo
	// Build style spec
	style := svggen.StyleSpec{}
	effectiveTheme := themeColors
	if spec.Style != nil && len(spec.Style.ThemeColors) > 0 {
		effectiveTheme = spec.Style.ThemeColors
	}

	// Apply explicit style colors if available.
	//
	// When the caller supplies spec.Style.Colors (an accent-only palette), we
	// must still forward dk1/dk2/lt1/lt2 from the effective theme so
	// StyleGuideFromSpec can populate TextPrimary/Secondary/Background/Surface
	// from the real template. Without this, svggen takes the PaletteSpec.Colors
	// branch and inherits text/bg from DefaultPalette (Tableau-10), which silently
	// drifts away from the native OOXML pipeline.
	//
	// Strategy (wbc7.7): synthesize a ThemeColors slice with accent1..accentN
	// pulled from spec.Style.Colors plus dk/lt slots from the effective theme,
	// and route through the ThemeColors branch.
	if spec.Style != nil && len(spec.Style.Colors) > 0 {
		// Cap synthesized accent names at 6 (the slots svggen recognizes).
		// Additional authored colors are not representable by the current
		// six-slot palette; see go-slide-creator-30j2u.
		accentLimit := len(spec.Style.Colors)
		if accentLimit > 6 {
			accentLimit = 6
		}
		inputs := make([]svggen.ThemeColorInput, 0, accentLimit+4)
		for i := 0; i < accentLimit; i++ {
			color, ok := ResolveDiagramStyleColor(spec.Style.Colors[i], effectiveTheme)
			if !ok {
				color = ChartAccentFallback(i, effectiveTheme)
			}
			inputs = append(inputs, svggen.ThemeColorInput{
				Name: fmt.Sprintf("accent%d", i+1),
				RGB:  color,
			})
		}
		for _, tc := range effectiveTheme {
			switch tc.Name {
			case "dk1", "dk2", "lt1", "lt2":
				inputs = append(inputs, svggen.ThemeColorInput{Name: tc.Name, RGB: tc.RGB})
			}
		}
		style.ThemeColors = inputs
	} else if spec.Style != nil && len(spec.Style.ThemeColors) > 0 {
		// Pass full theme colors so StyleGuideFromSpec can build a complete
		// palette with semantic colors (Success, Warning, Error, text colors, etc.)
		themeInputs := make([]svggen.ThemeColorInput, len(spec.Style.ThemeColors))
		for i, tc := range spec.Style.ThemeColors {
			themeInputs[i] = svggen.ThemeColorInput{
				Name: tc.Name,
				RGB:  tc.RGB,
			}
		}
		style.ThemeColors = themeInputs
	} else if len(themeColors) > 0 {
		// Pass full theme colors from template
		themeInputs := make([]svggen.ThemeColorInput, len(themeColors))
		for i, tc := range themeColors {
			themeInputs[i] = svggen.ThemeColorInput{
				Name: tc.Name,
				RGB:  tc.RGB,
			}
		}
		style.ThemeColors = themeInputs
	}

	// Embedded PPTX diagrams must render their theme accents with the exact
	// <a:schemeClr> hex that native shape_grid fills emit for the same accent
	// on the same slide. svggen's default StyleGuideFromSpec path runs
	// EnforceAccentContrast, which mutates low-contrast or near-duplicate
	// accents for chart legibility and so drifts away from the native palette.
	// Scope raw-theme parity to this embedded generator bridge only (it is
	// reached exclusively from PPTX render paths); standalone svggen CLI/MCP
	// keeps its readability-enforcement default. Explicit user colors arrive
	// via spec.Style.Colors / DataPalette above and are likewise preserved
	// verbatim, which is the desired behaviour. See go-slide-creator-gmv5.
	if len(style.ThemeColors) > 0 {
		style.DisablePaletteEnforcement = true
	}

	// Forward lt1 (background) and lt2 (surface) from the effective theme so
	// svggen's contrast calculations match native enforceTextContrastInSlide
	// on templates whose visible slide surface is tinted (not pure white).
	// Per-spec ThemeColors take priority over caller-supplied themeColors.
	bg, surface := lookupBackgroundAndSurface(effectiveTheme)
	if surface != "" {
		style.Surface = surface
	}
	if bg != "" {
		style.Background = bg
	}

	// Apply other style settings
	if spec.Style != nil {
		style.SemanticAccents = svggen.SemanticAccentSpec{
			Positive: spec.Style.SemanticAccents["positive"],
			Negative: spec.Style.SemanticAccents["negative"],
			Neutral:  spec.Style.SemanticAccents["neutral"],
		}
		style.ShowLegend = spec.Style.ShowLegend
		style.ShowValues = spec.Style.ShowValues
		if spec.Style.FontFamily != "" {
			style.FontFamily = spec.Style.FontFamily
		}
		if spec.Style.Background != "" {
			// Explicit per-spec background wins over theme lt1.
			if color, ok := ResolveDiagramStyleColor(spec.Style.Background, effectiveTheme); ok {
				style.Background = color
			}
		}
		if len(spec.Style.DataPalette) > 0 {
			style.DataPalette = spec.Style.DataPalette
		}
		// One number format for the whole chart: the value axis, the data labels
		// and any in-mark label (go-slide-creator-e2ck9).
		if vf := spec.Style.ValueFormat; vf != nil {
			style.ValueFormat = &svggen.ValueFormatSpec{
				Style:        vf.Style,
				Decimals:     vf.Decimals,
				Prefix:       vf.Prefix,
				Suffix:       vf.Suffix,
				ThousandsSep: vf.ThousandsSep,
			}
		}
	}

	// A chart's first series must not disappear merely because a template's
	// accent1 is nearly the canvas color. Preserve explicit author colors, but
	// filter automatic theme/data-palette choices below 2:1 on the effective
	// chart background. Diagrams retain their native theme-accent mapping.
	if len(style.ThemeColors) > 0 && isSVGChartType(spec.Type) &&
		(spec.Style == nil || len(spec.Style.Colors) == 0) {
		style.DataPalette = visibleChartPalette(style.ThemeColors, style.DataPalette, style.Background)
	}

	// Forward per-slide chart_style token overrides (vertical gridlines,
	// single-series legend, etc.) into the svggen StyleSpec. The two structs
	// are field-for-field copies — see internal/types.ChartStyleOverrides
	// and svggen/core.ChartStyleOverrides.
	if spec.ChartStyle != nil {
		style.ChartStyle = &svggen.ChartStyleOverrides{
			ShowVerticalGridlines:  spec.ChartStyle.ShowVerticalGridlines,
			ShowSingleSeriesLegend: spec.ChartStyle.ShowSingleSeriesLegend,
		}
	}

	// Build output spec
	output := svggen.OutputSpec{
		Format:      "png",
		FitMode:     spec.FitMode,
		MaxPNGWidth: maxPNGWidth,
		StrictFit:   strictFit,
	}

	if spec.Width > 0 {
		output.Width = spec.Width
	} else {
		output.Width = types.DefaultChartWidth
	}

	if spec.Height > 0 {
		output.Height = spec.Height
	} else {
		output.Height = types.DefaultChartHeight
	}

	// Use dynamic scale if set, otherwise use default minimum scale
	if spec.Scale > 0 {
		output.Scale = spec.Scale
	} else {
		output.Scale = types.DefaultMinScale
	}

	return &svggen.RequestEnvelope{
		Type:     spec.Type,
		Title:    spec.Title,
		Subtitle: spec.Subtitle,
		Data:     spec.Data,
		Output:   output,
		Style:    style,
	}
}

// ResolveDiagramStyleColor accepts authored hex colors and resolves semantic
// scheme names against the same effective theme used by native OOXML shapes.
func ResolveDiagramStyleColor(value string, theme []types.ThemeColor) (string, bool) {
	value = strings.TrimSpace(value)
	if color := resolveSchemeColorToHex(value, theme); color != "" {
		if _, err := svggen.ParseColor(color); err == nil {
			return color, true
		}
	}
	if strings.HasPrefix(value, "#") {
		if _, err := svggen.ParseColor(value); err == nil {
			return value, true
		}
	}
	return "", false
}

// ChartAccentFallback preserves series position when an authored color cannot
// be resolved, preferring the corresponding template accent.
func ChartAccentFallback(index int, theme []types.ThemeColor) string {
	if color := resolveSchemeColorToHex(fmt.Sprintf("accent%d", index+1), theme); color != "" {
		if _, err := svggen.ParseColor(color); err == nil {
			return color
		}
	}
	return svggen.DefaultPalette().AccentColor(index).Hex()
}

// DiagramStyleColorFindings reports authored colors the render path cannot
// resolve; otherwise svggen silently substitutes its default palette.
func DiagramStyleColorFindings(spec *types.DiagramSpec, themeColors []types.ThemeColor) []svggen.Finding {
	if spec == nil || spec.Style == nil {
		return nil
	}
	if len(spec.Style.ThemeColors) > 0 {
		themeColors = spec.Style.ThemeColors
	}
	var findings []svggen.Finding
	check := func(value, field string) {
		if _, ok := ResolveDiagramStyleColor(value, themeColors); ok {
			return
		}
		findings = append(findings, svggen.Finding{
			Field: field, Code: "CUSTOM_COLOR_DROPPED", Severity: "info",
			Message: fmt.Sprintf("chart style color %q cannot be resolved against the effective theme; the template/default color is used", value),
			Fix:     &svggen.FixSuggestion{Kind: "use_semantic_color", Params: map[string]any{"field": field}},
		})
	}
	for i, color := range spec.Style.Colors {
		if i >= 6 {
			break
		}
		check(color, fmt.Sprintf("style.colors[%d]", i))
	}
	if spec.Style.Background != "" {
		check(spec.Style.Background, "style.background")
	}
	return findings
}

func isSVGChartType(name string) bool {
	for _, capability := range svggen.ChartCapabilities() {
		if capability.Type == name {
			return true
		}
		for _, alias := range capability.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return false
}

// visibleChartPalette retains preferred chart order while removing colors
// with less than 2:1 contrast on the chart canvas. Dark theme slots provide
// distinct fallbacks when a template has fewer than six visible accents.
func visibleChartPalette(theme []svggen.ThemeColorInput, preferred []string, backgroundHex string) []string {
	background, err := svggen.ParseColor(backgroundHex)
	if err != nil {
		background = svggen.MustParseColor("#FFFFFF")
	}
	ordered := append([]string{}, preferred...)
	for i := 1; i <= 6; i++ {
		name := fmt.Sprintf("accent%d", i)
		for _, tc := range theme {
			if tc.Name == name {
				ordered = append(ordered, tc.RGB)
				break
			}
		}
	}
	for _, name := range []string{"dk2", "dk1", "lt2", "lt1"} {
		for _, tc := range theme {
			if tc.Name == name {
				ordered = append(ordered, tc.RGB)
				break
			}
		}
	}
	// A malformed or monochrome theme may have fewer than six usable slots.
	// Borrow distinct default chart colors only after exhausting its own
	// palette, so multiple data series do not become identical.
	for _, color := range svggen.DefaultPalette().AccentColors() {
		ordered = append(ordered, color.Hex())
	}
	ordered = append(ordered, "#000000", "#FFFFFF")
	visible := make([]string, 0, 6)
	seen := make(map[string]bool, len(ordered))
	for _, hex := range ordered {
		color, err := svggen.ParseColor(hex)
		if err != nil || color.ContrastWith(background) < 2 {
			continue
		}
		key := strings.ToUpper(color.Hex())
		if seen[key] {
			continue
		}
		seen[key] = true
		visible = append(visible, key)
		if len(visible) == 6 {
			return visible
		}
	}
	if len(visible) == 0 {
		black := svggen.MustParseColor("#000000")
		white := svggen.MustParseColor("#FFFFFF")
		if white.ContrastWith(background) > black.ContrastWith(background) {
			visible = append(visible, white.Hex())
		} else {
			visible = append(visible, black.Hex())
		}
	}
	baseCount := len(visible)
	for len(visible) < 6 {
		visible = append(visible, visible[len(visible)%baseCount])
	}
	return visible
}

// lookupBackgroundAndSurface returns the hex values for the theme's lt1 (slide
// background) and lt2 (alternate surface) colors. Either return value is "" if
// the corresponding entry is missing from the input.
func lookupBackgroundAndSurface(colors []types.ThemeColor) (background, surface string) {
	for _, c := range colors {
		switch c.Name {
		case "lt1":
			background = c.RGB
		case "lt2":
			surface = c.RGB
		}
	}
	return background, surface
}
