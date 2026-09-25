// Package generator: preflight predictor for the contrast-autofix
// render-time finding.
//
// The renderer's contrast pass (text_contrast.go) auto-replaces low-contrast
// text and emits contrast_autofixed findings. This preflight detector
// predicts where a swap would happen using only theme colors and JSON
// content — no rendering.
package generator

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// ContrastPreflightPair is one (foreground, background) pair to check at
// preflight time. Both colors may be supplied as hex (e.g. "#FFFFFF") or
// as scheme names (e.g. "accent1", "lt1"). Scheme names are resolved
// against ThemeColors before the contrast check.
type ContrastPreflightPair struct {
	// Path is the JSON pointer where the pair is authored.
	Path string
	// Foreground is the text color (hex or scheme name).
	Foreground string
	// ForegroundMods are OOXML transforms on inherited template text color.
	// The source reference remains in Foreground for replacement-mode parity.
	ForegroundMods types.BackgroundColorModifiers
	// Background is the fill / layout background color (hex or scheme name).
	Background string
	// Backgrounds are the ordered colors of a placeholder gradient. A single
	// replacement cannot be predicted from one stop as if it fixed them all.
	Backgrounds []string
	// Gradient marks a placeholder gradient even when its stops could not be
	// fully resolved, so preflight never falls back to the canvas silently.
	Gradient bool
	// Source is a short tag for the message (e.g. "shape_grid", "layout").
	Source string
	// TextPt is the text size in points and Bold its weight. They decide which
	// WCAG AA ratio applies: 3:1 only for genuinely large text (>=18pt, or
	// >=14pt bold), 4.5:1 otherwise. Zero means "unknown", which is treated as
	// small text — the conservative reading (go-slide-creator-9ux4).
	TextPt float64
	Bold   bool
	// AuthorBackground marks a background the author introduced (a slide
	// background.color or scrim) rather than one the template supplies. The
	// replacement snaps to the palette there instead of lerping, so the
	// prediction matches what the renderer does (go-slide-creator-s7wmh). A
	// shape-grid cell is NOT one of these: its fill and text were chosen
	// together.
	AuthorBackground bool
}

// PredictCompiledGridContrast runs the same contrast pass used by generation on
// already compiled shape XML. Copying the slice keeps group rewrites from
// changing the caller's shapes; the XML bytes themselves are replaced, not
// edited in place, by the pass.
func PredictCompiledGridContrast(shapes [][]byte, themeColors []types.ThemeColor, slideIndex int, slideBackground string) []ContrastSwap {
	if len(shapes) == 0 {
		return nil
	}
	input := append([][]byte(nil), shapes...)
	_, swaps := enforceShapeGridContrast(input, themeColors, computeWhiteTextSafeHex(themeColors), slideIndex, slideBackground)
	return swaps
}

// CompiledGridSwapFinding reports a render-time decision at validate time.
// A grouped decision cannot be repaired by changing one authored cell, so it
// deliberately has no single-cell fix suggestion.
func CompiledGridSwapFinding(swap ContrastSwap, path, authoredColor, source string, themeColors []types.ThemeColor) patterns.FitFinding {
	if path == "" {
		path = swap.Path
	}
	if source == "" {
		source = swap.Source
	}
	var fix *patterns.FixSuggestion
	if authoredColor != "" && swap.Cells < 2 && source == "shape_grid" {
		mode := ""
		fg, fgErr := svggen.ParseColor(swap.OriginalColor)
		bg, bgErr := svggen.ParseColor(swap.BackgroundColor)
		if fgErr == nil && bgErr == nil {
			_, mode = contrastReplacement(authoredColor, fg, bg, themeColors, svggen.WCAGAANormal, false)
		}
		fix = &patterns.FixSuggestion{
			Kind: "replace_color",
			Params: map[string]any{
				"from":                  authoredColor,
				"to":                    swap.ReplacedColor,
				"target":                "text",
				"path":                  path,
				"original_color":        swap.OriginalColor,
				"predicted_replacement": swap.ReplacedColor,
				"replacement_color":     swap.ReplacedColor,
				"background_color":      swap.BackgroundColor,
				"contrast_ratio_before": math.Round(swap.RatioBefore*100) / 100,
				"contrast_ratio_after":  math.Round(swap.RatioAfter*100) / 100,
				"replacement_mode":      mode,
				"source":                source,
			},
		}
	}
	message := fmt.Sprintf("predicted: low-contrast text will be auto-replaced — %s → %s (on %s, ratio %.1f → %.1f)",
		swap.OriginalColor, swap.ReplacedColor, swap.BackgroundColor,
		math.Round(swap.RatioBefore*100)/100, math.Round(swap.RatioAfter*100)/100)
	if swap.Cells > 1 {
		message = fmt.Sprintf("%s; shared across %d cells", message, swap.Cells)
	}
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path, Code: patterns.ErrCodeContrastPredicted, Message: message, Fix: fix,
		},
		Action: "info",
	}
}

// DetectContrastPreflight predicts whether the renderer's contrast pass
// would auto-replace the foreground color in each pair. It emits a
// contrast_predicted finding for any pair below the WCAG AA ratio that pair's
// text size requires — 4.5:1 for normal text, 3:1 only for large text.
//
// Both colors are first resolved through the theme (so semantic scheme
// names work). Pairs that cannot be resolved or parsed are skipped.
func DetectContrastPreflight(pairs []ContrastPreflightPair, themeColors []types.ThemeColor) []patterns.FitFinding {
	if len(pairs) == 0 {
		return nil
	}

	var findings []patterns.FitFinding
	for _, p := range pairs {
		fgHex := resolveContrastColor(p.Foreground, themeColors)
		bgHex := resolveContrastColor(p.Background, themeColors)
		if fgHex == "" || (bgHex == "" && len(p.Backgrounds) == 0 && !p.Gradient) {
			continue
		}

		fg, err := svggen.ParseColor(fgHex)
		if err != nil {
			continue
		}
		parsedBackgrounds := resolvePreflightBackgrounds(p, bgHex, themeColors)
		if len(parsedBackgrounds) == 0 && !p.Gradient {
			continue
		}
		var bg svggen.Color
		if len(parsedBackgrounds) > 0 {
			bg = parsedBackgrounds[0]
		}
		if p.ForegroundMods != (types.BackgroundColorModifiers{}) {
			mods := p.ForegroundMods
			if mods.HasLumMod && mods.LumMod == 0 && mods.LumOff == 0 {
				fg = svggen.MustParseColor("#000000")
			}
			alpha := 1.0
			if mods.HasAlpha {
				alpha = float64(mods.Alpha) / 100000
			}
			fg = patterns.EffectiveColorMods(fg, patterns.ColorMods{
				LumMod: mods.LumMod, LumOff: mods.LumOff, Tint: mods.Tint,
				Shade: mods.Shade, Alpha: alpha, HasAlpha: mods.HasAlpha,
			}, bg)
			fgHex = strings.ToUpper(fg.Hex())
		}

		threshold := contrastThresholdFor(p.TextPt, p.Bold)
		if p.Gradient {
			translucentText := p.ForegroundMods.HasAlpha && p.ForegroundMods.Alpha < 100000
			if finding := gradientContrastFinding(p.Path, fg, parsedBackgrounds, threshold, translucentText); finding != nil {
				findings = append(findings, *finding)
			}
			continue
		}
		ratio := fg.ContrastWith(bg)
		if ratio >= threshold {
			continue
		}

		// Use the same replacement algorithm the renderer applies, classifying
		// the foreground from its *authored* form (p.Foreground) so a pure
		// neutral authored as a scheme name (e.g. "lt1") flips just as the
		// render-time pass would. This keeps the predicted color identical to
		// the contrast_autofixed swap. replacement_mode discloses which branch
		// produced it.
		replacement, mode := contrastReplacement(p.Foreground, fg, bg, themeColors, threshold, p.AuthorBackground)
		newRatio := replacement.ContrastWith(bg)
		source := p.Source
		if source == "" {
			source = "preflight"
		}

		var fix *patterns.FixSuggestion
		if source != "slide_background" && source != "template_background" && source != "derived_shape_grid" {
			fix = &patterns.FixSuggestion{
				Kind: "replace_color",
				Params: map[string]any{
					"from":                  p.Foreground,
					"to":                    replacement.Hex(),
					"target":                "text",
					"path":                  p.Path,
					"original_color":        strings.ToUpper(fgHex),
					"predicted_replacement": replacement.Hex(),
					"replacement_color":     replacement.Hex(),
					"background_color":      strings.ToUpper(bgHex),
					"contrast_ratio_before": math.Round(ratio*100) / 100,
					"contrast_ratio_after":  math.Round(newRatio*100) / 100,
					"replacement_mode":      mode,
					"source":                source,
				},
			}
		}
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: p.Path,
				Code: patterns.ErrCodeContrastPredicted,
				Message: fmt.Sprintf(
					"predicted: low-contrast text will be auto-replaced — %s → %s (on %s, ratio %.1f → %.1f)",
					strings.ToUpper(fgHex), replacement.Hex(), strings.ToUpper(bgHex),
					math.Round(ratio*100)/100, math.Round(newRatio*100)/100,
				),
				Fix: fix,
			},
			Action: "info",
		})
	}
	return findings
}

func resolvePreflightBackgrounds(p ContrastPreflightPair, bgHex string, themeColors []types.ThemeColor) []svggen.Color {
	refs := p.Backgrounds
	if p.Gradient && len(refs) == 0 {
		return nil
	}
	if len(refs) == 0 {
		refs = []string{bgHex}
	}
	colors := make([]svggen.Color, 0, len(refs))
	for _, ref := range refs {
		hex := resolveContrastColor(ref, themeColors)
		if hex == "" {
			return nil
		}
		color, err := svggen.ParseColor(hex)
		if err != nil {
			return nil
		}
		colors = append(colors, color)
	}
	return colors
}

func gradientContrastFinding(path string, fg svggen.Color, stops []svggen.Color, threshold float64, translucentText bool) *patterns.FitFinding {
	if len(stops) == 0 || translucentText {
		reason := "placeholder gradient has unresolvable or translucent color stops"
		if translucentText {
			reason = "translucent placeholder text has no single foreground color across the gradient"
		}
		return &patterns.FitFinding{
			ValidationError: patterns.ValidationError{Path: path, Code: patterns.ErrCodeContrastUnresolved,
				Message: reason + "; contrast cannot be verified or safely auto-fixed"},
			Action: "refuse",
		}
	}
	minRatio := math.Inf(1)
	worst := stops[0]
	for i, stop := range stops {
		if ratio := fg.ContrastWith(stop); ratio < minRatio {
			minRatio, worst = ratio, stop
		}
		if i+1 < len(stops) {
			ratio, color := minGradientSegmentContrast(fg, stop, stops[i+1])
			if ratio < minRatio {
				minRatio, worst = ratio, color
			}
		}
	}
	if minRatio >= threshold {
		return nil
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path, Code: patterns.ErrCodeContrastUnresolved,
			Message: fmt.Sprintf("text %s on placeholder gradient fails contrast at %s (minimum gradient ratio %.2f:1; required %.1f:1); a single text-color replacement is not a verified fix", strings.ToUpper(fg.Hex()), strings.ToUpper(worst.Hex()), minRatio, threshold),
		},
		Action: "refuse",
	}
}

// minGradientSegmentContrast checks every distinct 8-bit RGB color produced
// by linear interpolation between two OOXML stops. A channel changes only
// when it crosses a half-integer boundary, so interval midpoints cover the
// whole rendered color sequence without an arbitrary sampling step.
func minGradientSegmentContrast(fg, start, end svggen.Color) (float64, svggen.Color) {
	breaks := []float64{0, 1}
	for _, pair := range [][2]uint8{{start.R, end.R}, {start.G, end.G}, {start.B, end.B}} {
		delta := int(pair[1]) - int(pair[0])
		if delta < 0 {
			delta = -delta
		}
		for step := 0; step < delta; step++ {
			breaks = append(breaks, (float64(step)+0.5)/float64(delta))
		}
	}
	sort.Float64s(breaks)
	minRatio := math.Inf(1)
	worst := start
	for i := 0; i+1 < len(breaks); i++ {
		color := lerpGradientRGB(start, end, (breaks[i]+breaks[i+1])/2)
		if ratio := fg.ContrastWith(color); ratio < minRatio {
			minRatio, worst = ratio, color
		}
	}
	return minRatio, worst
}

func lerpGradientRGB(start, end svggen.Color, t float64) svggen.Color {
	channel := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
	}
	return svggen.Color{R: channel(start.R, end.R), G: channel(start.G, end.G), B: channel(start.B, end.B), A: 1}
}

// resolveContrastColor accepts either a hex color ("#RRGGBB" or "RRGGBB")
// or a scheme name (e.g. "accent1", "lt1", "dk1") and returns a normalized
// hex string. Returns "" when the input cannot be resolved.
func resolveContrastColor(value string, themeColors []types.ThemeColor) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}

	// Hex form.
	if strings.HasPrefix(v, "#") {
		if len(v) == 7 {
			return strings.ToUpper(v)
		}
		return ""
	}
	if len(v) == 6 && isHex(v) {
		return "#" + strings.ToUpper(v)
	}

	// Scheme name → theme lookup. Reuses the schemeToThemeName map from
	// text_contrast.go.
	themeName, ok := schemeToThemeName[strings.ToLower(v)]
	if !ok {
		return ""
	}
	for _, tc := range themeColors {
		if tc.Name == themeName {
			hex := tc.RGB
			if !strings.HasPrefix(hex, "#") {
				hex = "#" + hex
			}
			return strings.ToUpper(hex)
		}
	}
	return ""
}

// isHex returns true when s is a valid 6-character hex digit string.
func isHex(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
