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
	// Background is the fill / layout background color (hex or scheme name).
	Background string
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
		if fgHex == "" || bgHex == "" {
			continue
		}

		fg, err := svggen.ParseColor(fgHex)
		if err != nil {
			continue
		}
		bg, err := svggen.ParseColor(bgHex)
		if err != nil {
			continue
		}

		threshold := contrastThresholdFor(p.TextPt, p.Bold)
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
		if source != "slide_background" && source != "derived_shape_grid" {
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
