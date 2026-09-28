package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Locating a generation refusal (go-slide-creator-ygaln).
//
// The generator refuses a deck whose written text would be unreadable or lose
// source. Its error named only the element kind ("body text renders at 2.0pt")
// and a path indexed by the generator's own slide slice, so finding the slide
// in a 29-slide deck meant bisecting it. The helpers below name the authored
// slide (1-based), its layout and pattern / diagram, and rewrite the path to
// the authored slide index — which differs from the generator's once --partial
// has dropped a slide — and let --partial skip the refused slide instead of
// failing the whole deck.

// generationRefusal returns the source-preserving refusal carried by a
// generation error, or nil for any other failure.
func generationRefusal(err error) *patterns.ValidationError {
	var loss *patterns.ValidationError
	if !errors.As(err, &loss) || loss == nil {
		return nil
	}
	switch loss.Code {
	case patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed,
		patterns.ErrCodeTableRowsTruncated, patterns.ErrCodeTextBelowReadableMin:
		return loss
	}
	return nil
}

// locateRefusal rewrites loss.Path from the generator's slide index to the
// authored one (specToInput maps the first to the second; nil is the
// identity), fills loss.Pattern from the authored slide when the generator did
// not name a diagram, and returns the authored slide index and a one-line
// location: `slide 3 (layout "Two Content", pattern "kpi-3up") at /slides/2/…`.
// It returns -1 and "" when the path names no slide.
func locateRefusal(input *PresentationInput, specs []generator.SlideSpec, layouts []types.LayoutMetadata, specToInput []int, loss *patterns.ValidationError) (int, string) {
	if loss == nil {
		return -1, ""
	}
	specIdx := slidepath.SlideIndex(loss.Path)
	if specIdx < 0 {
		return -1, ""
	}
	inputIdx := specIdx
	if specToInput != nil {
		if specIdx >= len(specToInput) {
			return -1, ""
		}
		inputIdx = specToInput[specIdx]
	}
	if rest := strings.TrimPrefix(loss.Path, slidepath.Slide(specIdx)); rest != loss.Path {
		loss.Path = slidepath.Slide(inputIdx) + rest
	}

	var details []string
	if specIdx < len(specs) {
		layout := specs[specIdx].LayoutID
		for _, l := range layouts {
			if l.ID == layout && l.Name != "" {
				layout = l.Name
				break
			}
		}
		if layout != "" {
			details = append(details, fmt.Sprintf("layout %q", layout))
		}
	}
	if input != nil && inputIdx < len(input.Slides) {
		slide := input.Slides[inputIdx]
		switch {
		case slide.Pattern != nil && slide.Pattern.Name != "":
			details = append(details, fmt.Sprintf("pattern %q", slide.Pattern.Name))
			if loss.Pattern == "" {
				loss.Pattern = slide.Pattern.Name
			}
		case slide.Compose != nil:
			details = append(details, "compose")
		}
	}
	if loss.Pattern != "" && !strings.Contains(strings.Join(details, " "), fmt.Sprintf("%q", loss.Pattern)) {
		details = append(details, fmt.Sprintf("diagram %q", loss.Pattern))
	}
	location := fmt.Sprintf("slide %d", inputIdx+1)
	if len(details) > 0 {
		location += " (" + strings.Join(details, ", ") + ")"
	}
	return inputIdx, location + " at " + loss.Path
}

// generateSkippingRefusedSlides runs generation for RunPresentation. In
// partial mode a slide generation refuses (unreadable or lossy text) is dropped
// and the rest of the deck regenerated, with a CONTENT_DROPPED finding and a
// warning naming the slide, instead of failing the whole deck; the last slide
// is never dropped, since an empty deck is not a partial one. A refusal that
// still fails the deck names the authored slide, layout, pattern / diagram and
// path (go-slide-creator-ygaln).
func generateSkippingRefusedSlides(ctx context.Context, input *PresentationInput, genReq *generator.GenerationRequest, layouts []types.LayoutMetadata, conversionFindings []patterns.FitFinding, partial bool, res *RenderResult) (*generator.GenerationResult, error) {
	specToInput := authoredSlideMap(len(input.Slides), conversionFindings)
	genResult, genErr := generator.Generate(ctx, *genReq)
	for genErr != nil && partial && len(genReq.Slides) > 1 {
		loss := generationRefusal(genErr)
		specIdx := -1
		if loss != nil {
			specIdx = slidepath.SlideIndex(loss.Path)
		}
		if specIdx < 0 || specIdx >= len(genReq.Slides) || specIdx >= len(specToInput) {
			break
		}
		inputIdx, location := locateRefusal(input, genReq.Slides, layouts, specToInput, loss)
		res.GridVisualFindings = append(res.GridVisualFindings, refusedSlideSkipped(inputIdx, location, loss))
		res.GridDiagWarnings = append(res.GridDiagWarnings, fmt.Sprintf("%s: skipped (partial mode), generation refused it: %s", location, loss.Message))
		genReq.Slides = append(append([]generator.SlideSpec(nil), genReq.Slides[:specIdx]...), genReq.Slides[specIdx+1:]...)
		specToInput = append(append([]int(nil), specToInput[:specIdx]...), specToInput[specIdx+1:]...)
		genReq.Footer = footerConfigForInput(input, len(genReq.Slides))
		res.SlideSpecs = genReq.Slides
		genResult, genErr = generator.Generate(ctx, *genReq)
	}
	if genErr == nil {
		return genResult, nil
	}
	if loss := generationRefusal(genErr); loss != nil {
		if _, location := locateRefusal(input, genReq.Slides, layouts, specToInput, loss); location != "" {
			return nil, fmt.Errorf("failed to generate PPTX: %s: %w", location, genErr)
		}
	}
	return nil, fmt.Errorf("failed to generate PPTX: %w", genErr)
}

// authoredSlideMap maps each generator slide to its authored slide, skipping
// the slides convertPresentationSlides dropped in partial mode (each leaves a
// CONTENT_DROPPED finding at its bare /slides/N path).
func authoredSlideMap(slideCount int, conversionFindings []patterns.FitFinding) []int {
	dropped := map[int]bool{}
	for _, f := range conversionFindings {
		if f.Code == patterns.ErrCodeContentDropped {
			if idx := slidepath.SlideIndex(f.Path); idx >= 0 && f.Path == slidepath.Slide(idx) {
				dropped[idx] = true
			}
		}
	}
	m := make([]int, 0, slideCount)
	for i := 0; i < slideCount; i++ {
		if !dropped[i] {
			m = append(m, i)
		}
	}
	return m
}

// refusedSlideSkipped is the CONTENT_DROPPED finding for a slide --partial
// dropped because generation refused it; the refusal's own code, path and
// message travel in its params.
func refusedSlideSkipped(inputIdx int, location string, loss *patterns.ValidationError) patterns.FitFinding {
	f := patterns.ContentDropped(slidepath.Slide(inputIdx), fmt.Sprintf("slide %d", inputIdx+1),
		fmt.Sprintf("skipped in partial mode, generation refused %s with %s: %s", location, loss.Code, loss.Message))
	f.Fix.Params["cause"] = loss.Code
	f.Fix.Params["refused_path"] = loss.Path
	if loss.Pattern != "" {
		f.Pattern = loss.Pattern
	}
	return f
}
