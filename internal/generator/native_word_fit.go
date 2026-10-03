package generator

import (
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Native-diagram words that break mid-word (go-slide-creator-v74wv).
//
// A native diagram's text sits at fixed sizes at or below the readable floor,
// so the writer cannot shrink a word that is wider than its shape: a pyramid
// apex in a narrow compose column rendered "Stra / teg / y", a value-chain
// chevron "Inboun / d", with no finding. Generation scans every native group it
// writes with pptx.UnfitWords — the one longest-word check, measured as the
// writer measured — and validate runs the generator's own builders into the
// same bounds and the same scan, so the two report the same TEXT_EXCEEDS_SHAPE.

// nativeWordFitFinding returns the TEXT_EXCEEDS_SHAPE finding for the words a
// rendered native group breaks mid-word, at path, or nil when every word fits.
func nativeWordFitFinding(diagramType, path, groupXML, fontName string) *patterns.FitFinding {
	unfit := pptx.UnfitWords(groupXML, fontName)
	if len(unfit) == 0 {
		return nil
	}
	worst := unfit[0]
	words := make([]string, 0, len(unfit))
	seen := map[string]bool{}
	for _, u := range unfit {
		if !seen[u.Word] {
			seen[u.Word] = true
			words = append(words, u.Word)
		}
		if wordOverflowRatio(u) > wordOverflowRatio(worst) {
			worst = u
		}
	}
	ratio := wordOverflowRatio(worst)
	action := "review"
	if ratio >= 2 {
		action = "shrink_or_split"
	}
	requiredPt := float64(worst.NeedEMU) / float64(types.EMUPerPoint)
	availablePt := float64(worst.AvailEMU) / float64(types.EMUPerPoint)
	msg := fmt.Sprintf("native %s: %q needs %.0fpt but its shape leaves %.0fpt of line width — it will break mid-word; shorten the label, give the diagram a larger region or placeholder, or use fewer items",
		diagramType, worst.Word, requiredPt, availablePt)
	if len(unfit) > 1 {
		msg = fmt.Sprintf("native %s: %d shapes have words wider than their text area (%s); worst: %q needs %.0fpt but its shape leaves %.0fpt of line width — they will break mid-word; shorten the labels, give the diagram a larger region or placeholder, or use fewer items",
			diagramType, len(unfit), strings.Join(words, ", "), worst.Word, requiredPt, availablePt)
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: diagramType,
			Path:    path,
			Code:    patterns.ErrCodeTextExceedsShape,
			Message: msg,
			Fix: &patterns.FixSuggestion{
				Kind: "widen_shape_text_area",
				Params: map[string]any{
					"diagram_type": diagramType,
					"words":        words,
					"word":         worst.Word,
					"required_pt":  math.Round(requiredPt*10) / 10,
					"available_pt": math.Round(availablePt*10) / 10,
					"hint":         "native diagram text has a fixed size: shorten or abbreviate the word, give the diagram a wider shape_grid cell / compose segment or a body placeholder, or use fewer items",
				},
			},
		},
		Action:   action,
		Measured: &patterns.Extent{WidthEMU: worst.NeedEMU},
		Allowed:  &patterns.Extent{WidthEMU: worst.AvailEMU},
	}
}

func wordOverflowRatio(u pptx.UnfitWord) float64 {
	if u.AvailEMU <= 0 {
		return math.Inf(1)
	}
	return float64(u.NeedEMU) / float64(u.AvailEMU)
}

// NativeDiagramWordFitPreflight lays the native diagram out in bounds with the
// generator's own builders and returns the TEXT_EXCEEDS_SHAPE finding
// generation raises for its mid-word breaks, at path (the authored diagram),
// or nil. bounds is the placeholder or region the diagram is drawn in.
func NativeDiagramWordFitPreflight(spec *types.DiagramSpec, bounds types.BoundingBox, fontName string, themeColors []types.ThemeColor, path string) []patterns.FitFinding {
	if !IsNativeDiagramType(spec) || bounds.Width <= 0 || bounds.Height <= 0 {
		return nil
	}
	env := nativeDiagramEnv{fontName: fontName, themeColors: themeColors}
	layout, err := layoutNativeDiagram(spec, bounds, env, nativeDiagramSite{})
	if err != nil {
		return nil
	}
	group := renderNativeInsert(&layout.insert, nativeReadabilityShapeIDBase, env)
	if f := nativeWordFitFinding(spec.Type, path, group, fontName); f != nil {
		return []patterns.FitFinding{*f}
	}
	return nil
}
