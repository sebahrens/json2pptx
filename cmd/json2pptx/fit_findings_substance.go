// fit_findings_substance.go measures what a slide SAYS, not just how it fits
// (go-slide-creator-q7ar).
//
// score_deck was a fit-findings aggregator: 100 minus the weight of whatever
// codes happened to exist. Across a 16-deck calibration set graded blind from
// the renders, it correlated with the human grade at Spearman +0.17 and passed
// 14 of 16 decks — including one whose every slide reads "Lorem ipsum" /
// "Click to add title" / "XX%" (score 99), eight identical KPI slides (100),
// four slides with no title (100) and five slides carrying a single one-word
// bullet (100). Nothing in the formula looked at content substance, title
// presence, or monotony, so an agent optimising the number ships junk.
//
// These detectors add the missing defect classes. They are deliberately
// conservative — each fires on evidence a human would name out loud ("this says
// Lorem ipsum", "this slide has no title", "these eight slides are the same") —
// because a false positive here blocks a deck.
package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/placeholderrole"
	"github.com/sebahrens/json2pptx/internal/policy/textwalk"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

var numericSectionLabelRE = regexp.MustCompile(`^\d+$`)

// collectSectionNumberSequenceFindings compares authored numeric divider labels
// with the same 1-based sequence convertPresentationSlides auto-injects. Custom
// non-numeric labels remain an explicit authoring choice and are left alone.
func collectSectionNumberSequenceFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	predicted := predictSlideLayouts(input, layouts)
	sectionIndex := 0
	var out []patterns.FitFinding
	for slideIndex, slide := range input.Slides {
		if !isSectionSlideInput(slide, layouts) {
			continue
		}
		sectionIndex++
		var resolved *types.LayoutMetadata
		if slideIndex < len(predicted) {
			resolved = predicted[slideIndex]
		}
		contentIndex, actual, ok := authoredSectionLabel(slide.Content, resolved)
		if !ok || !numericSectionLabelRE.MatchString(actual) {
			continue
		}
		if normalizeDecimalLabel(actual) == fmt.Sprintf("%d", sectionIndex) {
			continue
		}
		expected := fmt.Sprintf("%02d", sectionIndex)
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    fmt.Sprintf("/slides/%d/content/%d/text_value", slideIndex, contentIndex),
				Code:    patterns.ErrCodeSectionNumberSequenceMismatch,
				Message: fmt.Sprintf("slide %d is section %d but its authored numeric label is %q; use %q or remove the label and let json2pptx number sections automatically", slideIndex+1, sectionIndex, actual, expected),
				Fix: &patterns.FixSuggestion{Kind: "provide_value", Params: map[string]any{
					"path":          fmt.Sprintf("slides[%d].content[%d].text_value", slideIndex, contentIndex),
					"value":         expected,
					"expected":      expected,
					"actual":        actual,
					"section_index": sectionIndex,
					"slide_index":   slideIndex,
					"content_index": contentIndex,
				}},
			},
			Action: "refuse",
		})
	}
	return out
}

func normalizeDecimalLabel(value string) string {
	normalized := strings.TrimLeft(value, "0")
	if normalized == "" {
		return "0"
	}
	return normalized
}

// authoredSectionLabel returns the first value occupying the resolved section
// number slot. Dedicated section-number aliases win; templates without that
// slot use the body placeholder, matching injectSectionNumber.
func authoredSectionLabel(content []ContentInput, layout *types.LayoutMetadata) (int, string, bool) {
	dedicated := layout != nil && layoutHasSectionNumberPlaceholder(*layout)
	for i := range content {
		item := &content[i]
		isTarget := placeholderrole.IsSectionNumberAlias(item.PlaceholderID)
		if !isTarget {
			id := strings.ToLower(item.PlaceholderID)
			isTarget = strings.Contains(id, "section") && strings.Contains(id, "number")
		}
		if dedicated && !isTarget {
			continue
		}
		if !dedicated && item.PlaceholderID != "body" && !isTarget {
			continue
		}
		value := strings.TrimSpace(contentItemText(item))
		if value != "" {
			return i, value, true
		}
	}
	return 0, "", false
}

// collectSubstanceFindings returns the content-substance findings for a deck:
// placeholder copy, missing titles, near-empty slides and deck monotony.
func collectSubstanceFindings(input *PresentationInput, layouts ...types.LayoutMetadata) []patterns.FitFinding {
	if input == nil || len(input.Slides) == 0 {
		return nil
	}
	var findings []patterns.FitFinding
	findings = append(findings, collectPlaceholderContentFindings(input)...)
	findings = append(findings, collectSlideSubstanceFindings(input, layouts...)...)
	findings = append(findings, collectMonotonyFindings(input)...)
	findings = append(findings, collectChartLegibilityFindings(input)...)
	return findings
}

// --- placeholder copy ---

// placeholderPhrases are fragments that only appear in exemplar copy. Matched
// case-insensitively anywhere in a string.
var placeholderPhrases = []string{
	"lorem ipsum",
	"dolor sit amet",
	"consectetur adipiscing",
	"click to add",
	"presentation title",
	"subtitle goes here",
	"title goes here",
	"your text here",
	"body text here",
	"placeholder text",
	"sample text",
}

// placeholderWholeValues are strings that are placeholders when they are the
// WHOLE value — "TBD" inside a sentence is a real statement about a date, but a
// KPI whose value is "TBD" has no number yet.
var placeholderWholeValues = map[string]bool{
	"tbd":        true,
	"tba":        true,
	"todo":       true,
	"n/a":        true,
	"xx":         true,
	"xx%":        true,
	"xxx":        true,
	"x%":         true,
	"$x":         true,
	"$xx":        true,
	"$x.xm":      true,
	"$xxm":       true,
	"###":        true,
	"???":        true,
	"your title": true,
}

// exemplarLabelRE matches a pattern's own exemplar labels — "Card 1",
// "Description 2", "Metric 3" — as a WHOLE value. A real slide's "Option 1:
// direct sales team in Berlin" carries more than the label, so it does not
// match (go-slide-creator-7ucp asked for the exemplar values to be covered).
var exemplarLabelRE = regexp.MustCompile(`^(card|description|item|metric|feature|benefit|point|bullet|column|row|label|heading|title|kpi|value)\s+\d+$`)

// placeholderNumberRE matches masked numbers like "XX%", "$X.XM", "X,XXX" —
// the shape a value takes before anyone has filled it in.
var placeholderNumberRE = regexp.MustCompile(`^[$€£]?[xX#?]+([.,][xX#?]+)*\s?[%kKmMbB]?$`)

// slideWalkPathRE pulls the slide index out of a textwalk accessor path.
var slideWalkPathRE = regexp.MustCompile(`^slides\[(\d+)\]`)

// structuralKeys are JSON keys whose strings identify STRUCTURE, not content:
// a placeholder called "subtitle" or a shape whose geometry is "rect" says
// nothing about what the slide claims. Walking every string without this
// exclusion flagged a legitimate deck because one of its placeholders is
// literally named "subtitle" (go-slide-creator-q7ar).
var structuralKeys = map[string]bool{
	"placeholder_id": true, "type": true, "kind": true, "name": true,
	"slide_type": true, "layout_id": true, "id": true, "ref": true,
	"geometry": true, "fill": true, "color": true, "text_color": true,
	"border_color": true, "accent": true, "style": true, "align": true,
	"anchor": true, "transition": true, "transition_speed": true, "build": true,
	"font": true, "font_family": true, "template": true, "icon": true,
	"path": true, "url": true, "image": true, "svg_data": true, "role": true,
	"variant": true, "direction": true, "shape": true, "marker": true,
	"format": true, "unit_position": true, "viewing_mode": true,
	"design_mode": true, "accent_strategy": true, "archetype": true,
}

// isStructuralPath reports whether a walked string is a structural identifier
// rather than authored copy.
func isStructuralPath(path string) bool {
	last := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		last = path[i+1:]
	}
	// Strip an array index so "bullets_value[2]" resolves to "bullets_value".
	if i := strings.IndexByte(last, '['); i >= 0 {
		last = last[:i]
	}
	return structuralKeys[last]
}

// collectPlaceholderContentFindings reports exemplar copy anywhere in the deck —
// titles, bullets, pattern values, grid cells — one finding per slide naming
// the offending strings.
func collectPlaceholderContentFindings(input *PresentationInput) []patterns.FitFinding {
	hits := map[int][]string{}
	order := []int{}
	textwalk.Strings(input, func(value, path string) {
		if isStructuralPath(path) || !isPlaceholderCopy(value) {
			return
		}
		idx := slideIndexFromWalkPath(path)
		if idx < 0 {
			return
		}
		if len(hits[idx]) == 0 {
			order = append(order, idx)
		}
		if len(hits[idx]) < 3 && !containsStr(hits[idx], value) {
			hits[idx] = append(hits[idx], value)
		}
	})

	var out []patterns.FitFinding
	for _, idx := range order {
		samples := hits[idx]
		quoted := make([]string, len(samples))
		for i, s := range samples {
			quoted[i] = fmt.Sprintf("%q", truncateForMessage(s, 48))
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: slidepath.Slide(idx),
				Code: patterns.ErrCodeWeakContent,
				Message: fmt.Sprintf("slide %d still carries exemplar copy (%s) — replace it with the deck's real content before shipping",
					idx+1, strings.Join(quoted, ", ")),
				Fix: &patterns.FixSuggestion{
					Kind:   "replace_placeholder",
					Params: map[string]any{"samples": toAnySlice(samples)},
				},
			},
			Action: "refuse",
		})
	}
	return out
}

// isPlaceholderCopy reports whether a string is exemplar copy rather than
// content.
func isPlaceholderCopy(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, phrase := range placeholderPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	if placeholderWholeValues[lower] {
		return true
	}
	if exemplarLabelRE.MatchString(lower) {
		return true
	}
	// A masked number is only a placeholder as a whole value; "X" inside prose
	// is a variable name, not an unfilled field.
	return len(trimmed) <= 8 && placeholderNumberRE.MatchString(trimmed)
}

// --- per-slide substance ---

// minSlideWords is the fewest words a content slide can carry and still say
// something. Below it the slide is a heading with nothing under it.
const minSlideWords = 8

// collectSlideSubstanceFindings reports content slides with no title and
// content slides that carry almost no text.
func collectSlideSubstanceFindings(input *PresentationInput, layouts ...types.LayoutMetadata) []patterns.FitFinding {
	var out []patterns.FitFinding
	// A deck whose every slide is content-free is not a deck to grade: it is
	// almost always a DeckSpec handed to a PresentationInput tool. Say that
	// once, rather than reporting each slide as empty (go-slide-creator-q7ar).
	if allSlidesContentFree(input) {
		return append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: slidepath.Slide(0),
				Code: patterns.ErrCodeSlideNearlyEmpty,
				Message: fmt.Sprintf("none of the %d slides carry content this tool recognises — every slide would render blank; if this JSON is a DeckSpec (kind / points / takeaway), compile it with validate_deck_spec + render_deck_spec, whose findings describe the spec itself",
					len(input.Slides)),
				Fix: &patterns.FixSuggestion{
					Kind:   "provide_value",
					Params: map[string]any{"path": "slides[].content", "value": "<content items, or compile the DeckSpec first>"},
				},
			},
			Action: "review",
		})
	}

	for si, slide := range input.Slides {
		if isContentFreeSlide(slide) {
			// Not "empty by design": an authored blank slide sets slide_type.
			// A slide with nothing the engine recognises renders blank — which
			// is also what a DeckSpec looks like when it is handed to a
			// PresentationInput tool by mistake (go-slide-creator-q7ar).
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path: slidepath.Slide(si),
					Code: patterns.ErrCodeSlideNearlyEmpty,
					Message: fmt.Sprintf("slide %d carries no content the engine recognises — it renders blank; if this JSON is a DeckSpec (kind / points / takeaway), compile it with validate_deck_spec + render_deck_spec instead",
						si+1),
					Fix: &patterns.FixSuggestion{
						Kind:   "provide_value",
						Params: map[string]any{"path": "content", "value": "<the slide's content items>"},
					},
				},
				Action: "review",
			})
			continue
		}
		if !slideCarriesArgument(slide) {
			continue
		}
		headlineRenders := strings.TrimSpace(slide.Headline) != "" && isBlankCanvasLayout(slide.LayoutID, layouts)
		if _, title := extractTitleText(slide); title == "" && patternTitleValue(slide) == "" && !headlineRenders {
			fixPath := "title"
			if isBlankCanvasLayout(slide.LayoutID, layouts) {
				fixPath = "headline"
			}
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path: slidepath.Slide(si),
					Code: patterns.ErrCodeMissingTitle,
					Message: fmt.Sprintf("slide %d has no title — the audience cannot tell what it is about, and the deck has no navigable structure",
						si+1),
					Fix: &patterns.FixSuggestion{
						Kind:   "provide_value",
						Params: map[string]any{"path": fixPath, "value": "<the point this slide makes>"},
					},
				},
				Action: "review",
			})
		}
		if hasNonTextContent(slide) {
			continue
		}
		if words := slideBodyWordCount(slide); words < minSlideWords {
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path: slidepath.Slide(si),
					Code: patterns.ErrCodeSlideNearlyEmpty,
					Message: fmt.Sprintf("slide %d carries %d word(s) of body content (minimum %d) — it reads as a heading with nothing under it; add the substance or merge it into a neighbouring slide",
						si+1, words, minSlideWords),
					Fix: &patterns.FixSuggestion{
						Kind:   "add_items",
						Params: map[string]any{"current_words": words, "min_words": minSlideWords},
					},
				},
				Action: "review",
			})
		}
	}
	return out
}

// allSlidesContentFree reports a deck in which no slide carries anything the
// engine can render.
func allSlidesContentFree(input *PresentationInput) bool {
	for _, slide := range input.Slides {
		if !isContentFreeSlide(slide) {
			return false
		}
	}
	return len(input.Slides) > 0
}

// isContentFreeSlide reports a slide that carries nothing the engine can
// render: no content items, no pattern, no grid, no compose, and no explicit
// slide_type saying it is deliberately blank.
func isContentFreeSlide(slide SlideInput) bool {
	if slide.SlideType != "" || slide.Background != nil {
		return false
	}
	return len(slide.Content) == 0 && slide.Pattern == nil && slide.ShapeGrid == nil && slide.Compose == nil
}

// slideCarriesArgument reports whether a slide is expected to make a point.
// Title, section and blank slides are chrome, not argument.
func slideCarriesArgument(slide SlideInput) bool {
	switch types.SlideType(slide.SlideType) {
	case types.SlideTypeTitle, types.SlideTypeSection:
		return false
	case types.SlideTypeBlank:
		return hasArgumentContent(slide)
	}
	if slide.SlideType == "" {
		switch inferSlideType(slide) {
		case types.SlideTypeTitle, types.SlideTypeSection:
			return false
		case types.SlideTypeBlank:
			return hasArgumentContent(slide)
		}
	}
	return true
}

func hasArgumentContent(slide SlideInput) bool {
	return len(slide.Content) > 0 || slide.ShapeGrid != nil || slide.Pattern != nil ||
		slide.Compose != nil || len(slide.Overlays) > 0 || slide.Background != nil
}

// hasNonTextContent reports whether the slide carries a chart, diagram, table,
// image or grid — content a word count says nothing about.
func hasNonTextContent(slide SlideInput) bool {
	if slide.ShapeGrid != nil || slide.Pattern != nil || slide.Compose != nil {
		return true
	}
	for _, item := range slide.Content {
		switch item.Type {
		case "chart", "diagram", "table", "image":
			return true
		}
	}
	return false
}

// slideBodyWordCount counts the words in a slide's non-title text content.
func slideBodyWordCount(slide SlideInput) int {
	words := 0
	for i := range slide.Content {
		item := &slide.Content[i]
		if isHeadlinePlaceholderID(item.PlaceholderID) {
			continue
		}
		words += len(strings.Fields(contentItemText(item)))
	}
	return words
}

// contentItemText flattens a content item's authored text.
func contentItemText(item *ContentInput) string {
	var b strings.Builder
	if item.TextValue != nil {
		b.WriteString(*item.TextValue)
		b.WriteString(" ")
	}
	if item.BulletsValue != nil {
		b.WriteString(strings.Join(*item.BulletsValue, " "))
		b.WriteString(" ")
	}
	if bab := item.BodyAndBulletsValue; bab != nil {
		b.WriteString(bab.Body + " " + strings.Join(bab.Bullets, " ") + " " + bab.TrailingBody + " ")
	}
	if bg := item.BulletGroupsValue; bg != nil {
		b.WriteString(bg.Body + " ")
		for _, g := range bg.Groups {
			b.WriteString(g.Header + " " + g.Body + " " + strings.Join(g.Bullets, " ") + " ")
		}
	}
	if b.Len() == 0 {
		if resolved, err := item.ResolveValue(); err == nil {
			if s, ok := resolved.(string); ok {
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

// patternTitleValue returns a title carried inside a pattern's values, which is
// how several patterns author their headline.
func patternTitleValue(slide SlideInput) string {
	if slide.Pattern == nil {
		return ""
	}
	title := ""
	textwalk.Strings(slide.Pattern, func(value, path string) {
		if title != "" {
			return
		}
		if strings.HasSuffix(path, "values.title") || strings.HasSuffix(path, "values.headline") {
			title = strings.TrimSpace(value)
		}
	})
	return title
}

// --- deck monotony ---

// monotonyRunReview is the run length at which repeating one slide shape reads
// as monotony; monotonyRunRefuse is where the deck is a spreadsheet in slide
// form.
const (
	monotonyRunReview = 4
	monotonyRunRefuse = 6
)

// collectMonotonyFindings reports runs of consecutive slides built the same way.
// The composition analysis already computed this signal; the score threw it
// away, so eight identical KPI slides scored 100 (go-slide-creator-q7ar).
func collectMonotonyFindings(input *PresentationInput) []patterns.FitFinding {
	var out []patterns.FitFinding
	runStart, runShape := -1, ""
	flush := func(end int) {
		if runStart < 0 {
			return
		}
		runLen := end - runStart
		if runLen < monotonyRunReview || runShape == "" {
			return
		}
		action := "review"
		if runLen >= monotonyRunRefuse {
			action = "refuse"
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: slidepath.Slide(runStart),
				Code: patterns.ErrCodeDeckMonotony,
				Message: fmt.Sprintf("slides %d-%d are all %s — %d consecutive slides with the same shape read as one long slide; vary the visual family or merge them",
					runStart+1, end, runShape, runLen),
				Fix: &patterns.FixSuggestion{
					Kind: "swap_pattern",
					Params: map[string]any{
						"run_length":  runLen,
						"slide_range": []any{runStart, end - 1},
						"shape":       runShape,
					},
				},
			},
			Action: action,
		})
	}
	for i, slide := range input.Slides {
		shape := slideShapeKey(slide)
		if shape != runShape {
			flush(i)
			runStart, runShape = i, shape
		}
	}
	flush(len(input.Slides))
	return out
}

// slideShapeKey describes how a slide is built — the pattern name, else the
// content types it carries. Chrome slides return "" so they never form a run.
func slideShapeKey(slide SlideInput) string {
	if !slideCarriesArgument(slide) {
		return ""
	}
	if slide.Pattern != nil && slide.Pattern.Name != "" {
		return "pattern " + slide.Pattern.Name
	}
	if slide.Compose != nil {
		return ""
	}
	types := make([]string, 0, len(slide.Content))
	for i := range slide.Content {
		item := &slide.Content[i]
		if isHeadlinePlaceholderID(item.PlaceholderID) || item.Type == "" {
			continue
		}
		if !containsStr(types, item.Type) {
			types = append(types, item.Type)
		}
	}
	if len(types) == 0 {
		return ""
	}
	return strings.Join(types, "+") + " slides"
}

// --- helpers ---

func slideIndexFromWalkPath(path string) int {
	m := slideWalkPathRE.FindStringSubmatch(path)
	if m == nil {
		return -1
	}
	idx := 0
	for _, c := range m[1] {
		idx = idx*10 + int(c-'0')
	}
	return idx
}

// containsStr reports whether needle is already in haystack.
func containsStr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func truncateForMessage(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max-1]) + "…"
}

// --- chart legibility ---

// Category ceilings a reader can actually follow. The general ceiling sits
// below svggen's own "label thinning at 15+ categories" behaviour: by the time
// the renderer starts dropping labels, the chart has stopped being readable.
// The long-label ceiling catches the other failure mode — a dozen categories
// whose names are full site or product names, which the renderer rotates and
// truncates into a hedge.
const (
	chartMaxCategories       = 12
	chartMaxSliceCategories  = 7
	chartLongLabelCategories = 8
	chartLongLabelMeanChars  = 24
)

// collectChartLegibilityFindings reports charts with more categories than a
// reader can follow. A 14-site bar chart with 40-character site names renders as
// an unreadable hedge, and nothing in the score saw it (go-slide-creator-q7ar).
func collectChartLegibilityFindings(input *PresentationInput) []patterns.FitFinding {
	var out []patterns.FitFinding
	for si := range input.Slides {
		slide := &input.Slides[si]
		for ci := range slide.Content {
			item := &slide.Content[ci]
			if item.Type != "chart" || item.ChartValue == nil {
				continue
			}
			labels := chartCategoryLabels(item.ChartValue)
			if len(labels) == 0 {
				continue
			}
			ceiling, why := chartCategoryCeiling(string(item.ChartValue.Type), labels)
			if len(labels) <= ceiling {
				continue
			}
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path: slidepath.ContentIndex(si, ci),
					Code: patterns.ErrCodeChartOverloaded,
					Message: fmt.Sprintf("slide %d: %s chart has %d categories (%s) — the labels are thinned or rotated into an unreadable band; keep the top %d and group the rest, or split the chart",
						si+1, item.ChartValue.Type, len(labels), why, ceiling),
					Fix: &patterns.FixSuggestion{
						Kind: "reduce_items",
						Params: map[string]any{
							"current_categories": len(labels),
							"max_categories":     ceiling,
						},
					},
				},
				Action: "review",
			})
		}
	}
	return out
}

// chartCategoryCeiling returns the readable category ceiling for a chart type
// and its labels, plus the reason that ceiling applies.
func chartCategoryCeiling(chartType string, labels []string) (int, string) {
	switch strings.ToLower(chartType) {
	case "pie", "donut", "doughnut":
		return chartMaxSliceCategories, "slices below a few percent cannot be labelled"
	}
	total := 0
	for _, l := range labels {
		total += len([]rune(l))
	}
	if mean := total / len(labels); mean >= chartLongLabelMeanChars && chartLongLabelCategories < chartMaxCategories {
		return chartLongLabelCategories, fmt.Sprintf("labels average %d characters", mean)
	}
	return chartMaxCategories, "beyond what an axis can label"
}

// chartCategoryLabels returns the category labels of a chart in whichever shape
// it was authored.
func chartCategoryLabels(chart *types.ChartSpec) []string { //nolint:staticcheck // ChartSpec is deprecated but still the authored chart shape
	if chart == nil {
		return nil
	}
	source := chart.Data
	if len(chart.TimeData) > 0 {
		source = chart.TimeData
	}
	// The structured form — {categories: [...], series: [...]} — is the one
	// SKILL.md documents and every data-format hint recommends, and it has to
	// be read BEFORE DataOrder: for that shape the decoder records the map's
	// own key order, so DataOrder is ["categories", "values"] and a 15-slice
	// pie counted as TWO categories. The legibility ceiling was blind to
	// exactly the wide datasets it exists for (go-slide-creator-r87g).
	if cats, ok := chartCategoryList(source["categories"]); ok {
		return cats
	}
	if len(chart.DataOrder) > 0 {
		return chart.DataOrder
	}
	labels := make([]string, 0, len(source))
	for k := range source {
		labels = append(labels, k)
	}
	sort.Strings(labels)
	return labels
}

// chartCategoryList reads a structured categories array, reporting false when
// the value is not one.
func chartCategoryList(v any) ([]string, bool) {
	raw, ok := v.([]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			out = append(out, t)
		default:
			out = append(out, fmt.Sprintf("%v", t))
		}
	}
	return out, true
}
