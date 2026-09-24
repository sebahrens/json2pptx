package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func derivedColumnLeftPercent(slide deckinput.SlideInput) int {
	if percent := layout.DerivedColumnLeftPercent(slide.LayoutID); percent != 0 {
		return percent
	}
	if slide.ColumnBaseLayoutID != "" && slide.ColumnBaseLayoutID != slide.LayoutID {
		return 0
	}
	if slide.ColumnLeftPercent != 0 {
		return slide.ColumnLeftPercent
	}
	return 0
}

func derivedColumnLayoutFinding(slide deckinput.SlideInput, slideIndex int) (patterns.FitFinding, bool) {
	percent := derivedColumnLeftPercent(slide)
	if percent == 0 {
		return patterns.FitFinding{}, false
	}
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    slidepath.Slide(slideIndex) + "/layout_id",
			Code:    patterns.ErrCodeLayoutDerived,
			Message: fmt.Sprintf("slide %d: derived %d/%d two-column geometry from layout %q, preserving its styling and using at least a 0.3-inch gutter", slideIndex+1, percent, 100-percent, slide.LayoutID),
		},
		Action: "info",
	}, true
}

// deriveLayoutMetadata mirrors the generator's chrome reservation followed by
// its per-slide column split. It is used by preview and fit preflight; neither
// may show the native 50/50 bounds for an asymmetric slide.
func deriveLayoutMetadata(slide deckinput.SlideInput, base types.LayoutMetadata, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) (types.LayoutMetadata, error) {
	percent := derivedColumnLeftPercent(slide)
	if percent == 0 {
		return base, nil
	}
	derived := base
	derived.Placeholders = append([]types.PlaceholderInfo(nil), base.Placeholders...)
	leftIdx, rightIdx := -1, -1
	for i, ph := range derived.Placeholders {
		switch ph.ID {
		case "body":
			leftIdx = i
		case "body_2":
			rightIdx = i
		}
	}
	if leftIdx < 0 || rightIdx < 0 {
		return base, fmt.Errorf("derived two-column layout %q requires body and body_2 placeholders", base.ID)
	}
	frame := slideChromeFrame(slide, base.ID, layouts, slideWidth, slideHeight)
	if slide.Takeaway == "" && slide.Source == "" {
		frame.Fits = false
	}
	for _, i := range []int{leftIdx, rightIdx} {
		derived.Placeholders[i].Bounds = clipDerivedColumnToChrome(derived.Placeholders[i].Bounds, frame)
	}
	left := derived.Placeholders[leftIdx].Bounds
	right := derived.Placeholders[rightIdx].Bounds
	if right.X < left.X {
		leftIdx, rightIdx = rightIdx, leftIdx
		left, right = right, left
	}
	var err error
	left, right, err = layout.DeriveColumnBounds(left, right, percent)
	if err != nil {
		return base, fmt.Errorf("layout %q: %w", base.ID, err)
	}
	for _, replacement := range []struct {
		idx int
		box types.BoundingBox
	}{{leftIdx, left}, {rightIdx, right}} {
		ph := &derived.Placeholders[replacement.idx]
		oldWidth := ph.Bounds.Width
		ph.Bounds = replacement.box
		if oldWidth > 0 && ph.MaxChars > 0 {
			ph.MaxChars = max(1, int(int64(ph.MaxChars)*ph.Bounds.Width/oldWidth))
		}
	}
	return derived, nil
}

func clipDerivedColumnToChrome(box types.BoundingBox, frame template.ChromeFrame) types.BoundingBox {
	if box.Width <= 0 || frame.Content.CX <= 0 {
		return box
	}
	right := box.X + box.Width
	if box.X < frame.Content.X && right > frame.Content.X {
		box.X = frame.Content.X
	}
	limit := frame.Content.X + frame.Content.CX
	if right > limit && box.X < limit {
		right = limit
	}
	if right > box.X {
		box.Width = right - box.X
	}
	if frame.Fits && box.Y < frame.Content.Bottom() && box.Y+box.Height > frame.Content.Bottom() {
		box.Height = frame.Content.Bottom() - box.Y
	}
	return box
}

func withDerivedFitLayouts(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) (*PresentationInput, []types.LayoutMetadata, []patterns.FitFinding) {
	var slides []SlideInput
	var derivedLayouts []types.LayoutMetadata
	var errors []patterns.FitFinding
	for i, slide := range input.Slides {
		if derivedColumnLeftPercent(slide) == 0 {
			continue
		}
		base := template.FindLayout(layouts, slide.LayoutID)
		if base == nil {
			continue // Existing layout-resolution finding handles an unknown ID.
		}
		derived, err := deriveLayoutMetadata(slide, *base, layouts, slideWidth, slideHeight)
		if err != nil {
			errors = append(errors, patterns.FitFinding{ValidationError: patterns.ValidationError{
				Path: slidepath.Slide(i) + "/layout_id", Code: patterns.ErrCodeLayoutUnresolvable, Message: err.Error(),
			}, Action: "refuse"})
			continue
		}
		if slides == nil {
			slides = append([]SlideInput(nil), input.Slides...)
			derivedLayouts = append([]types.LayoutMetadata(nil), layouts...)
		}
		derived.ID = fmt.Sprintf("__derived_two_column_slide_%d_%d", i, derivedColumnLeftPercent(slide))
		derivedLayouts = append(derivedLayouts, derived)
		slides[i].LayoutID = derived.ID
	}
	if slides == nil {
		return input, layouts, errors
	}
	copyInput := *input
	copyInput.Slides = slides
	return &copyInput, derivedLayouts, errors
}
