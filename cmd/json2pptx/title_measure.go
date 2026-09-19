package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// titleMeasurement is the single measured verdict for a title against its
// resolved title placeholder, shared by validate, the quality score and the
// fit report (go-slide-creator-vjwn). It replaces the three divergent
// character thresholds (placeholder max_chars, 60 chars, none) with the same
// measured fit the generator applies at render time (go-slide-creator-6cjs).
type titleMeasurement struct {
	// OK is false when the title could not be measured (no geometry or
	// unknown font size); callers fall back to their legacy heuristic.
	OK bool
	// Overflow: the title cannot fit even at the minimum autofit size.
	Overflow bool
	// Shrinks: the title fits only below the comfort scale or with reduced
	// line spacing.
	Shrinks bool
	// ScalePct is the font scale the title needs (100 = template size).
	ScalePct int
	// FontPt is the template title size in points.
	FontPt float64
	// Chars is the title length in runes; MaxChars the longest prefix that
	// fits at the comfort scale (only computed when flagged).
	Chars    int
	MaxChars int

	input  generator.TitleFitInput
	result textfit.FitResult
}

// Flagged reports whether the title should be shortened.
func (m titleMeasurement) Flagged() bool { return m.OK && (m.Overflow || m.Shrinks) }

// measureTitleInPlaceholder measures title text against a title placeholder's
// resolved bounds and inherited style (size, caps, line spacing).
func measureTitleInPlaceholder(text string, ph *types.PlaceholderInfo) titleMeasurement {
	if ph == nil || text == "" || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 || ph.FontSize <= 0 {
		return titleMeasurement{}
	}
	in := generator.TitleFitInput{
		Title:     text,
		WidthEMU:  ph.Bounds.Width,
		HeightEMU: ph.Bounds.Height,
		Style: template.InheritedTextStyle{
			SizeHPt:        ph.FontSize,
			CapsAll:        ph.TextCaps,
			LineSpacingPct: ph.LineSpacingPct,
		},
		FontName: ph.FontFamily,
	}
	res, err := generator.MeasureTitleFit(in)
	if err != nil {
		return titleMeasurement{}
	}
	m := titleMeasurement{
		OK:       true,
		Overflow: res.Overflow,
		Shrinks:  generator.TitleNeedsShortening(res, ph.FontSize),
		ScalePct: 100,
		FontPt:   float64(ph.FontSize) / 100.0,
		Chars:    len([]rune(text)),
		input:    in,
		result:   res,
	}
	if res.FontScale > 0 {
		m.ScalePct = res.FontScale / 1000
	}
	if m.Flagged() {
		m.MaxChars = generator.MaxTitleCharsAtScale(in, generator.TitleComfortScalePct(ph.FontSize))
	}
	return m
}

// describe renders a one-line human explanation of a flagged measurement.
func (m titleMeasurement) describe() string {
	if m.Overflow {
		return fmt.Sprintf("title (%d chars) does not fit its title placeholder even at the minimum autofit size (template %.0fpt); shorten to ≤ %d chars",
			m.Chars, m.FontPt, m.MaxChars)
	}
	return fmt.Sprintf("title (%d chars) only fits its title placeholder at %d%% of the template %.0fpt size; shorten to ≤ %d chars",
		m.Chars, m.ScalePct, m.FontPt, m.MaxChars)
}

// fitFinding converts a flagged measurement into a fit-report finding:
// TITLE_OVERFLOW when it overflows, else an escalated title_wraps.
func (m titleMeasurement) fitFinding(path string) *patterns.FitFinding {
	if !m.Flagged() {
		return nil
	}
	if m.Overflow {
		return generator.DetectTitleOverflow(path, m.input)
	}
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "placeholder",
			Path:    path,
			Code:    patterns.ErrCodeTitleWraps,
			Message: m.describe(),
			Fix: &patterns.FixSuggestion{
				Kind: "shorten_title",
				Params: map[string]any{
					"current_chars": m.Chars,
					"max_chars":     m.MaxChars,
					"fit_scale_pct": m.ScalePct,
				},
			},
		},
		Action: "shrink_or_split",
	}
}

// flaggedTitleFitFindings returns the must-fix measured title findings
// (TITLE_OVERFLOW and shrink_or_split title_wraps) in the local fit-report
// shape used by the CLI `validate --fit-report` human output.
func flaggedTitleFitFindings(input *PresentationInput, layouts []types.LayoutMetadata) []fitFinding {
	var out []fitFinding
	for _, f := range collectTitleFitFindings(input, layouts) {
		if f.Action != "shrink_or_split" {
			continue
		}
		out = append(out, fitFinding{
			Code:     f.Code,
			Path:     f.Path,
			Severity: "warning",
			Message:  f.Message,
			Fix:      f.Fix,
			Action:   f.Action,
		})
	}
	return out
}

// titleFitDiagnostic is the validate-time title check. measured is false when
// the title could not be measured (caller falls back to the max_chars
// estimate); d is nil when the measured title fits comfortably.
func titleFitDiagnostic(text string, ph *types.PlaceholderInfo, slideIdx, contentIdx int) (d *diagnostics.Diagnostic, measured bool) {
	m := measureTitleInPlaceholder(text, ph)
	if !m.OK {
		return nil, false
	}
	if !m.Flagged() {
		return nil, true
	}
	code := patterns.ErrCodeMaxLength
	if m.Overflow {
		code = patterns.ErrCodeTitleOverflow
	}
	return &diagnostics.Diagnostic{
		Code:     code,
		Path:     slidepath.ContentField(slideIdx, contentIdx, "text"),
		Message:  fmt.Sprintf("slide %d, content %d: %s", slideIdx+1, contentIdx+1, m.describe()),
		Severity: diagnostics.SeverityWarning,
		Fix: &diagnostics.Fix{
			Kind: "shorten_title",
			Params: map[string]any{
				"max_chars":     m.MaxChars,
				"current_chars": m.Chars,
				"fit_scale_pct": m.ScalePct,
			},
		},
	}, true
}

// layoutForSlideResolved finds a slide's layout by concrete id, falling back
// to canonical layout names ("content", "title", …) the way generation does.
func layoutForSlideResolved(slide *SlideInput, layouts []types.LayoutMetadata) *types.LayoutMetadata {
	if l := findLayoutForSlide(slide, layouts); l != nil {
		return l
	}
	if slide.LayoutID == "" {
		return nil
	}
	if id, ok := layout.ResolveCanonicalLayoutID(slide.LayoutID, layouts); ok {
		return findLayoutByID(layouts, id)
	}
	return nil
}

// titlePlaceholderFor resolves the title placeholder a slide's content item
// targets (canonical layout ids resolved). Returns nil when the item does not
// target a title placeholder.
func titlePlaceholderFor(slide *SlideInput, placeholderID string, layouts []types.LayoutMetadata) *types.PlaceholderInfo {
	return titlePlaceholderIn(layoutForSlideResolved(slide, layouts), placeholderID)
}

// titlePlaceholderIn returns the title placeholder with the given id on a
// resolved layout, or nil when the layout is unknown or the id is not a title.
func titlePlaceholderIn(l *types.LayoutMetadata, placeholderID string) *types.PlaceholderInfo {
	if l == nil {
		return nil
	}
	ph := findPlaceholderByID(placeholderID, l.Placeholders)
	if ph == nil || ph.Type != types.PlaceholderTitle {
		return nil
	}
	return ph
}

// collectTitleFitFindings measures every title against its resolved title
// placeholder: TITLE_OVERFLOW when it cannot fit, an escalated title_wraps
// (shrink_or_split, shorten_title) when it only fits below the comfort scale,
// and the informational title_wraps otherwise.
func collectTitleFitFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	var findings []patterns.FitFinding
	// A slide without an explicit layout_id still lands on a layout — the one
	// the heuristic selector picks. Measuring against it is the whole point of
	// a measured check: without this the semantic path, which never sets a
	// layout_id, fell back to a 60-character rule of thumb
	// (go-slide-creator-t64e).
	predicted := predictSlideLayouts(input, layouts)
	for si := range input.Slides {
		slide := &input.Slides[si]
		for ci := range slide.Content {
			content := &slide.Content[ci]
			ph := titlePlaceholderIn(predicted[si], content.PlaceholderID)
			if ph == nil || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
				continue
			}
			title := strings.Join(extractContentParagraphs(content), " ")
			if title == "" {
				// Legacy untyped "value" payloads.
				if resolved, _ := content.ResolveValue(); resolved != nil {
					title, _ = resolved.(string)
				}
			}
			if title == "" {
				continue
			}
			path := slidepath.Content(si, content.PlaceholderID)
			if f := measureTitleInPlaceholder(title, ph).fitFinding(path); f != nil {
				findings = append(findings, *f)
				continue
			}
			// Informational wrap notice: emitted once the slide's layout is
			// known concretely, whether the author named it or the selector
			// predicted it. Canonical-id slides (layout_id: "content") still
			// get only the measured verdict above, which is more specific.
			if findLayoutForSlide(slide, layouts) == nil && slide.LayoutID != "" {
				continue
			}
			if f := generator.DetectTitleWraps(generator.TitleWrapsInput{
				SlideIndex:  si,
				Path:        path,
				Title:       title,
				WidthEMU:    ph.Bounds.Width,
				HeightEMU:   ph.Bounds.Height,
				FontSizeHPt: ph.FontSize,
				FontName:    ph.FontFamily,
			}); f != nil {
				findings = append(findings, *f)
			}
		}
	}
	return findings
}
