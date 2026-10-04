package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// contrastSwapKeyRE extracts the "#FROM → #TO (on #BG" core that both the
// preflight prediction and the render-time record carry in their messages.
var contrastSwapKeyRE = regexp.MustCompile(`#[0-9A-Fa-f]{6} → #[0-9A-Fa-f]{6} \(on #[0-9A-Fa-f]{6}`)

// supersedeRealizedContrastPredictions drops a contrast_predicted finding once
// generation has recorded the very swap it predicted (same slide, same
// from/to/background colours) as contrast_autofixed. Both are info; reporting
// the forecast next to the outcome doubled ~25 contrast lines per deck and
// drowned the findings an agent has to act on (go-slide-creator-fabz4).
// Predictions the render did not confirm are kept.
func supersedeRealizedContrastPredictions(in []patterns.FitFinding) []patterns.FitFinding {
	type key struct {
		slide int
		swap  string
	}
	realized := map[key]bool{}
	for _, f := range in {
		if f.Code != "contrast_autofixed" {
			continue
		}
		if m := contrastSwapKeyRE.FindString(f.Message); m != "" {
			realized[key{slidepath.SlideIndex(f.Path), strings.ToUpper(m)}] = true
		}
	}
	if len(realized) == 0 {
		return in
	}
	out := make([]patterns.FitFinding, 0, len(in))
	for _, f := range in {
		if f.Code == patterns.ErrCodeContrastPredicted {
			if m := contrastSwapKeyRE.FindString(f.Message); m != "" && realized[key{slidepath.SlideIndex(f.Path), strings.ToUpper(m)}] {
				continue
			}
		}
		out = append(out, f)
	}
	return collapseFixlessContrastRecords(out)
}

// collapseFixlessContrastRecords folds a slide's contrast_autofixed records
// that carry no executable fix into one finding. They are informational and
// nothing can be repaired from them one by one; eight lines saying the same
// thing about one card grid read as eight problems. Records with a
// replace_color fix stay separate so each remains executable.
func collapseFixlessContrastRecords(in []patterns.FitFinding) []patterns.FitFinding {
	count := map[int]int{}
	for _, f := range in {
		if f.Code == "contrast_autofixed" && f.Fix == nil {
			count[slidepath.SlideIndex(f.Path)]++
		}
	}
	out := make([]patterns.FitFinding, 0, len(in))
	emitted := map[int]bool{}
	for _, f := range in {
		if f.Code != "contrast_autofixed" || f.Fix != nil {
			out = append(out, f)
			continue
		}
		si := slidepath.SlideIndex(f.Path)
		if emitted[si] {
			continue
		}
		emitted[si] = true
		if n := count[si]; n > 1 {
			f.Message = fmt.Sprintf("auto-fixed low-contrast text on %d surfaces of this slide, e.g. %s", n, f.Message)
		}
		out = append(out, f)
	}
	return out
}

// unpredictedContrastSwapFindings returns the contrast_autofixed records for
// the swaps generation made that no contrast_predicted finding in predicted
// forecast (same slide, same from/to/background colours).
//
// The DeckSpec render surfaces report the forecast, so that validate and
// render say the same thing about the same deck; a swap the forecast missed
// used to be reported by neither — generation recoloured the text and the
// result said nothing (go-slide-creator-sw78d). A predicted swap is not
// repeated: the forecast already names it, at the same path.
func unpredictedContrastSwapFindings(swaps []patterns.FitFinding, predicted []patterns.FitFinding) []patterns.FitFinding {
	if len(swaps) == 0 {
		return nil
	}
	type key struct {
		slide int
		swap  string
	}
	forecast := map[key]bool{}
	for _, f := range predicted {
		if f.Code != patterns.ErrCodeContrastPredicted {
			continue
		}
		if m := contrastSwapKeyRE.FindString(f.Message); m != "" {
			forecast[key{slidepath.SlideIndex(f.Path), strings.ToUpper(m)}] = true
		}
	}
	var out []patterns.FitFinding
	for _, f := range swaps {
		if m := contrastSwapKeyRE.FindString(f.Message); m != "" && forecast[key{slidepath.SlideIndex(f.Path), strings.ToUpper(m)}] {
			continue
		}
		out = append(out, f)
	}
	return collapseFixlessContrastRecords(out)
}
