package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
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
	// Refuse marks a section title that cannot fit at the 28pt floor.
	Refuse bool
	// Shrinks: the title fits only below the comfort scale or with reduced
	// line spacing.
	Shrinks bool
	// ScalePct is the font scale the title needs (100 = starting size).
	ScalePct int
	// FontPt is the effective starting title size in points.
	FontPt float64
	// Chars is the title length in runes; MaxChars the longest prefix of THIS
	// title that fits at the comfort scale (only computed when flagged) — the
	// shorten target. The placeholder's own capacity, which does not depend on
	// the current text, is generator.TitlePlaceholderCapacityChars.
	Chars    int
	MaxChars int

	input  generator.TitleFitInput
	result textfit.FitResult
}

// Flagged reports whether the title should be shortened.
func (m titleMeasurement) Flagged() bool { return m.OK && (m.Refuse || m.Overflow || m.Shrinks) }

// authoredTitlePlaceholder returns the title style generation will measure:
// an explicit content font_size replaces the inherited template size, while
// every other inherited property and the original template metadata survive.
func authoredTitlePlaceholder(ph *types.PlaceholderInfo, content *ContentInput) *types.PlaceholderInfo {
	if ph == nil || content == nil || content.FontSize == nil || *content.FontSize <= 0 {
		return ph
	}
	withSize := *ph
	withSize.FontSize = int(*content.FontSize * 100)
	return &withSize
}

// measureTitleInPlaceholder measures title text against a title placeholder's
// resolved bounds and inherited style (size, caps, line spacing).
func measureTitleInPlaceholder(text string, ph *types.PlaceholderInfo, sectionTitle ...bool) titleMeasurement {
	if ph == nil || text == "" || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 || ph.FontSize <= 0 {
		return titleMeasurement{}
	}
	in := generator.TitleFitInputForPlaceholder(text, ph)
	isSection := len(sectionTitle) > 0 && sectionTitle[0]
	if isSection {
		in.MinFontHPt = generator.SectionTitleMinHPt
		if in.Style.SizeHPt < 3200 {
			in.Style.SizeHPt = 3200 // generation boosts divider titles to at least 32pt
		}
	}
	res, err := generator.MeasureTitleFit(in)
	if err != nil {
		return titleMeasurement{}
	}
	m := titleMeasurement{
		OK:       true,
		Overflow: res.Overflow,
		Refuse:   isSection && (res.Overflow || res.LnSpcReduction > 0),
		Shrinks:  !isSection && generator.TitleNeedsShortening(res, ph.FontSize),
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
		if isSection {
			minScalePct := (in.MinFontHPt*100 + in.Style.SizeHPt - 1) / in.Style.SizeHPt
			m.MaxChars = generator.MaxTitleCharsAtScale(in, minScalePct)
		} else {
			m.MaxChars = generator.MaxTitleCharsAtScale(in, generator.TitleComfortScalePct(ph.FontSize))
		}
	}
	return m
}

// describe renders a one-line human explanation of a flagged measurement.
func (m titleMeasurement) describe() string {
	if m.Refuse {
		return fmt.Sprintf("section title (%d chars) cannot fit its divider at the 28pt floor; shorten to about %d chars", m.Chars, m.MaxChars)
	}
	if m.Overflow {
		return fmt.Sprintf("title (%d chars) does not fit its title placeholder even at the minimum autofit size (starting %.0fpt); shorten to ≤ %d chars",
			m.Chars, m.FontPt, m.MaxChars)
	}
	return fmt.Sprintf("title (%d chars) only fits its title placeholder at %d%% of the starting %.0fpt size; shorten to ≤ %d chars",
		m.Chars, m.ScalePct, m.FontPt, m.MaxChars)
}

// fitFinding converts a flagged measurement into a fit-report finding:
// TITLE_TRUNCATED for section-floor failure, TITLE_OVERFLOW for ordinary
// overflow, or an escalated title_wraps for uncomfortable ordinary fit.
func (m titleMeasurement) fitFinding(path string) *patterns.FitFinding {
	if !m.Flagged() {
		return nil
	}
	if m.Refuse {
		return generator.DetectSectionTitleFloor(path, m.input)
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

// diagnostic is the validate-time form of the measured verdict: the very same
// finding collectTitleFitFindings emits, converted to a Diagnostic. It carries
// the identical code, path and message, so the two surfaces collapse to one
// entry in the findings envelope instead of reporting one wrapped title twice
// under two codes (max_length and title_wraps) — go-slide-creator-jcph.
// Returns nil when the title was not measurable or fits comfortably.
func (m titleMeasurement) diagnostic(slideIdx, contentIdx int) *diagnostics.Diagnostic {
	f := m.fitFinding(slidepath.ContentIndex(slideIdx, contentIdx))
	if f == nil {
		return nil
	}
	d := diagnostics.FromFitFinding(*f)
	return &d
}

// measuredTitleKey identifies a content item whose title was measured against a
// resolved title placeholder.
type measuredTitleKey struct {
	slide       int
	placeholder string
}

// measuredTitleSet is the set of titles the measured check owns. Every
// character- and word-count title heuristic stands down for a title in this
// set, so one over-long headline is reported once, by one code, from every
// surface — the three contradictory numbers an agent used to see on one deck
// (60-char score fallback, 12-word HEADLINE_TOO_LONG, a geometric max_chars)
// were all guesses at what this measurement answers exactly
// (go-slide-creator-jcph).
type measuredTitleSet map[measuredTitleKey]bool

// has reports whether the title on this slide's placeholder was measured.
func (s measuredTitleSet) has(slide int, placeholder string) bool {
	return s[measuredTitleKey{slide: slide, placeholder: placeholder}]
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
// placeholder: section titles use the 28pt floor and TITLE_TRUNCATED refusal;
// ordinary titles use TITLE_OVERFLOW or the comfort-scale title_wraps finding.
//
// The second return value names every title this measurement covered, so the
// word-count content lint stands down for them (go-slide-creator-jcph).
func collectTitleFitFindings(input *PresentationInput, layouts []types.LayoutMetadata) ([]patterns.FitFinding, measuredTitleSet) {
	var findings []patterns.FitFinding
	measured := make(measuredTitleSet)
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
			path := slidepath.ContentIndex(si, ci)
			effectivePh := authoredTitlePlaceholder(ph, content)
			m := measureTitleInPlaceholder(title, effectivePh, isSectionSlideInput(*slide, layouts))
			if m.OK {
				measured[measuredTitleKey{slide: si, placeholder: content.PlaceholderID}] = true
			}
			if f := m.fitFinding(path); f != nil {
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
				FontSizeHPt: effectivePh.FontSize,
				FontName:    ph.FontFamily,
			}); f != nil {
				findings = append(findings, *f)
			}
		}
	}
	return findings, measured
}
