package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/placeholderrole"
	"github.com/sebahrens/json2pptx/internal/policy/inlinemarkup"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// DefaultFindingBudget is the maximum number of findings returned per slide
// before overflow is summarised. Use verbose=true in BudgetFitFindings to
// bypass the limit.
const DefaultFindingBudget = 5

// collectFitFindings runs all fit-report detectors (text overflow, placeholder
// overflow, title wraps, footer collision, bounds overflow) and returns sorted
// findings. The result is sorted by ActionRank descending (most severe first),
// then by slide index ascending.
//
// When theme is non-nil, preflight predictors that need theme colors run too:
// currently this gates the contrast_predicted detector. Pass nil to skip
// those (callers that don't have a parsed template theme).
func collectFitFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for i, slide := range input.Slides {
		if finding, ok := derivedColumnLayoutFinding(slide, i); ok {
			findings = append(findings, finding)
		}
	}
	var geometryErrors []patterns.FitFinding
	input, layouts, geometryErrors = withDerivedFitLayouts(input, layouts, slideWidth, slideHeight)
	findings = append(findings, geometryErrors...)

	// Expand compose envelopes into ShapeGrid so downstream detectors evaluate
	// against post-merge cell geometry. Without this step, a diagram placed in
	// a horizontally-merged segment would never trip CheckDiagramInNarrowBoundsFinding
	// or aspect-mismatch findings during preflight — the unexpanded slide
	// carries ShapeGrid == nil, so checkShapeGridStructural is skipped.
	var composeFindings []patterns.FitFinding
	input, composeFindings = expandComposeForPreflightWithTheme(input, slideWidth, slideHeight, theme, layouts...)
	findings = append(findings, composeFindings...)

	// Expand slide-level patterns the same way, so every detector below — text
	// capacity, readability, structural geometry, table and chart preflight —
	// measures the cells generation renders. A pattern slide carries no
	// ShapeGrid until generation, so the whole text-density family silently
	// skipped the surface the skill tells agents to author through
	// (go-slide-creator-adur). Paths are rerooted at /slides/N/pattern before
	// the findings are returned.
	// A pattern's own post-expand warnings are findings like any other: they
	// used to reach agents only through preview_presentation_plan, so validate
	// and generate reported "no issues" on a deck the pattern itself had
	// already objected to (go-slide-creator-wn4v). Collected from the
	// unexpanded input, before the grid replaces the pattern below.
	findings = append(findings, collectPatternPostExpandFindings(input, slideWidth, slideHeight, theme, layouts...)...)

	input, patternSlides := expandPatternsForFit(input, slideWidth, slideHeight, theme, layouts...)

	// 1. Text-fit findings from existing generateFitReport (tables + shape-grid
	// text). Pass the resolved layout geometry so shape_grid cells are measured
	// against the SAME bounds generation renders, matching the structural pass
	// below (go-slide-creator-ur3z).
	for _, tf := range generateFitReport(input, layouts, slideWidth, slideHeight) {
		findings = append(findings, convertTextFitFinding(tf))
	}

	// 2. Structural findings using template layout data.
	findings = append(findings,
		collectStructuralFindings(input, layouts, slideWidth, slideHeight)...)
	findings = append(findings, collectChromeCollisionFindings(input, layouts, slideWidth)...)

	// 2b. Measured title fit (TITLE_OVERFLOW / title_wraps) against the
	// resolved title placeholder and inherited title style.
	titleFindings, measuredTitles := collectTitleFitFindings(input, layouts)
	findings = append(findings, titleFindings...)
	// The injected takeaway is outside shape_grid and placeholder text, so its
	// fixed chrome band needs an explicit measurement before it can ship.
	findings = append(findings, collectTakeawayFitFindings(input, layouts, slideWidth, slideHeight)...)

	// 2c. Readability policy: shape_grid text the renderer would shrink below
	// the viewing_mode floor for its role (TEXT_BELOW_READABLE_MIN).
	findings = append(findings, collectReadabilityFindings(input, layouts, slideWidth, slideHeight)...)

	// 3. Grid rhythm violations when a deck-level grid is configured.
	if input.Grid != nil {
		if err := validateGridConfig(input.Grid); err == nil {
			rg := resolveGrid(input.Grid, layouts, slideWidth, slideHeight)
			findings = append(findings, detectGridViolations(rg, layouts, input.Slides)...)
		}
	}

	// 3a. Layout resolution: a template missing the role a slide needs used to
	// pass validate silently and fail generate (go-slide-creator-9svyz).
	findings = append(findings, collectLayoutResolutionFindings(input, layouts)...)

	// 3b. Inline markup the renderer does not support. Unsupported tags are
	// passed through to the text run verbatim, so they print as literal
	// XML-ish garbage on the slide; nothing used to report them
	// (go-slide-creator-510u).
	findings = append(findings, inlinemarkup.Validate(input)...)

	// 4. Grid occupancy: raw-grid underfill / known-pattern overcrowding.
	findings = append(findings, collectGridOccupancyFindings(input)...)

	// 4a. Structural smells on authored grids: stacked tables, crushed
	// dividers, hex/scheme fill mixing and accent overload. The detectors
	// existed and were documented, but nothing called them, so the codes
	// could never reach an agent (go-slide-creator-s1uvj.13).
	findings = append(findings, collectStructuralSmellFindings(input)...)

	// 4b. Sparse single-row flow guard: a slide-level process-flow /
	// timeline-horizontal ("dots") with sparse labels and no height cap stretches
	// its lone row to fill the slide (SPARSE_SINGLE_ROW_FLOW). Reads slide.Pattern
	// directly, so compose / nested-cell patterns (which have a second zone) are
	// exempt by construction.
	findings = append(findings, collectSparseSingleRowFlowFindings(input)...)

	// 4c. Pattern-choice & rendering-geometry QA heuristics (J2P-VQA-009):
	// over-tall single-row flow lanes (the cases SPARSE_SINGLE_ROW_FLOW does not
	// cover), decision diamonds with no branch zone, agenda slides drawn as
	// flowcharts, and rotated axis bands that intrude after rotation. Each
	// carries a rendering-vs-pattern-choice class via patterns.FindingClass.
	findings = append(findings, collectPatternChoiceFindings(input)...)

	// 5. Preflight predictions for render-time-only findings: table_font_scaled,
	// table_rows_truncated, column_width_deficit, text_trimmed,
	// readability_trimmed. These mirror the renderer's scaling/trimming logic
	// without rendering.
	findings = append(findings, collectTablePreflightFindings(input, layouts)...)
	findings = append(findings, collectTextAutofitPreflightFindings(input, layouts)...)

	// 6. Contrast prediction (contrast_predicted) — runs only when theme
	// colors are available to resolve scheme references.
	if theme != nil {
		findings = append(findings, collectContrastPreflightFindings(input, layouts, theme.Colors)...)
	}

	// 7. Chart / diagram dry-render findings (chart.tick_thinned,
	// chart.label_clipped, chart.legend_overflow_dropped, etc.) — runs
	// svggen's layout/labeling pass per chart/diagram content item and
	// surfaces render-only findings at validate/preview time, closing the
	// validate → preview → generate feedback loop for visual chart issues.
	var chartThemeColors []types.ThemeColor
	var chartBodyFont string
	if theme != nil {
		chartThemeColors = theme.Colors
		chartBodyFont = theme.BodyFont
	}
	findings = append(findings,
		collectChartDryRenderFindingsInFrames(input, chartThemeColors, chartBodyFont, "warn", layouts, slideWidth, slideHeight)...)

	// 8. Content lint: headline word count, body word count, bullet nesting
	// depth. Advisory findings that flag verbose / over-nested authoring
	// before render.
	findings = append(findings, collectContentLintFindings(input, measuredTitles)...)
	// 8b. Pattern / content mismatch: a pattern that does not match the shape
	// of the content authored into it (go-slide-creator-h339i). Reads
	// slide.Pattern directly, before expansion erases which pattern was asked
	// for.
	findings = append(findings, collectPatternMismatchFindings(input)...)
	// 8c. Bundled icon names in diagram panels that do not resolve. The panel
	// renders without its icon while its siblings keep theirs, and the only
	// trace used to be a server log line (go-slide-creator-puki).
	findings = append(findings, collectDiagramIconFindings(input)...)
	findings = append(findings, collectBackgroundFindings(input)...)
	findings = append(findings, collectNativeImageContrastFindings(input, layouts)...)

	// 9. Accessibility lint: alt text on images / icons sourced from
	// path/url/svg_data, and on charts, diagrams and tables. Bundled icon names
	// are exempt (implicit captions); so are the chart/diagram/table cells of a
	// pattern-expanded grid, which the author cannot annotate.
	findings = append(findings, collectAltTextFindings(input, patternSlides)...)

	// 9b. Content substance: exemplar copy, titleless slides, near-empty slides
	// and deck monotony — the defect classes the score could not see
	// (go-slide-creator-q7ar).
	findings = append(findings, collectSubstanceFindings(input, layouts...)...)
	findings = append(findings, collectSectionNumberSequenceFindings(input, layouts)...)

	// 10. Deck-level duplicate-title lint: flags content slides that share a
	// title (case-insensitive, whitespace-normalized) so authors don't ship
	// decks where multiple content slides announce the same point. Title and
	// section-divider slides are exempt because cover/closing slides
	// legitimately repeat phrasing.
	findings = append(findings, collectDuplicateTitleFindings(input)...)

	// 11. Deterministic geometry: TEXT_EXCEEDS_SHAPE / SPARSE_FILL / SLIDE_UNDERUSED.
	findings = append(findings, collectGeometryFindings(input, layouts, slideWidth, slideHeight, theme)...)

	// Deduplicate findings that share (Code, Path, Action, Message). This guards
	// against the case where a pre-compose detector and the post-compose
	// structural pass both emit the same diagnostic for one cell — the
	// post-compose check on the merged grid should not double-surface.
	findings = dedupFitFindings(findings)

	// Canonical findings order: (severity desc, slide_index asc, code asc).
	// See patterns.SortCanonical and docs/FIT_FINDINGS.md "Sort invariant".
	patterns.SortCanonical(findings, slidepath.SlideIndex)

	// Reroot findings emitted against a pattern's expanded grid: the deck has no
	// shape_grid at that index, so the synthetic path would point at nothing.
	if len(patternSlides) > 0 {
		for i := range findings {
			findings[i].Path = rerootPatternPath(findings[i].Path, patternSlides)
			if findings[i].Pattern == "" {
				findings[i].Pattern = patternNameForSlide(input, findings[i].Path)
			}
			findings[i].Fix = patternCellFix(findings[i].Fix, findings[i].Pattern)
		}
	}

	// Attach next_tool_call to actionable findings, then keep every suggestion
	// callable: under the core profile a suggestion naming a hidden tool (e.g.
	// recommend_pattern) is a dead end (go-slide-creator-mvny).
	patterns.AttachNextToolCalls(findings, slidepath.SlideIndex)
	retargetUnadvertisedSuggestions(findings)

	return findings
}

// BudgetFitFindings enforces a per-slide finding budget. Within each slide,
// findings are ranked by severity (ActionRank descending) then actionability
// (findings with a Fix set are ranked above those without). If a slide exceeds
// the budget, only the top findings are kept and a summary finding is appended
// indicating how many were suppressed.
//
// When verbose is true the budget is not applied and all findings are returned.
func BudgetFitFindings(findings []patterns.FitFinding, budget int, verbose bool) []patterns.FitFinding {
	if verbose || len(findings) == 0 {
		return findings
	}
	if budget <= 0 {
		budget = DefaultFindingBudget
	}

	// Group findings by slide index.
	type group struct {
		slideIdx int
		items    []patterns.FitFinding
	}
	order := []int{} // insertion-order slide indices
	bySlide := map[int]*group{}

	for _, f := range findings {
		si := slidepath.SlideIndex(f.Path)
		g, ok := bySlide[si]
		if !ok {
			g = &group{slideIdx: si}
			bySlide[si] = g
			order = append(order, si)
		}
		g.items = append(g.items, f)
	}

	// Sort each group: ActionRank desc, then Fix-present before Fix-absent.
	for _, si := range order {
		g := bySlide[si]
		sort.SliceStable(g.items, func(i, j int) bool {
			ri := patterns.ActionRank(g.items[i].Action)
			rj := patterns.ActionRank(g.items[j].Action)
			if ri != rj {
				return ri > rj
			}
			fi := g.items[i].Fix != nil
			fj := g.items[j].Fix != nil
			if fi != fj {
				return fi
			}
			return false
		})
	}

	// Apply budget per slide.
	var result []patterns.FitFinding
	for _, si := range order {
		g := bySlide[si]
		if len(g.items) <= budget {
			result = append(result, g.items...)
			continue
		}
		result = append(result, g.items[:budget]...)
		suppressed := g.items[budget:]
		path := slidepath.Slide(si)
		if si < 0 {
			path = "/slides/?"
		}
		topCodes := findingCodeHistogram(suppressed)
		result = append(result, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    path,
				Code:    "findings_truncated",
				Message: fmt.Sprintf("%d more findings suppressed on this slide; use verbose_fit to see all", len(suppressed)),
				Fix: &patterns.FixSuggestion{
					Kind: "truncation_summary",
					Params: map[string]any{
						"suppressed_count": len(suppressed),
						"top_codes":        topCodes,
					},
				},
			},
			Action: "info",
		})
	}

	// The per-slide budget selection groups findings by insertion-order of
	// slide index, which can scramble the global (severity desc, slide asc,
	// code asc) invariant. Re-apply the canonical sort so the post-budget
	// slice still satisfies it.
	patterns.SortCanonical(result, slidepath.SlideIndex)

	return result
}

// budgetLocalFindings applies the same per-slide budget as BudgetFitFindings
// but operates on the local fitFinding type used by generateFitReport.
func budgetLocalFindings(findings []fitFinding, budget int, verbose bool) []fitFinding {
	if verbose || len(findings) == 0 {
		return findings
	}
	if budget <= 0 {
		budget = DefaultFindingBudget
	}

	type group struct {
		slideIdx int
		items    []fitFinding
	}
	order := []int{}
	bySlide := map[int]*group{}

	for _, f := range findings {
		si := slidepath.SlideIndex(f.Path)
		g, ok := bySlide[si]
		if !ok {
			g = &group{slideIdx: si}
			bySlide[si] = g
			order = append(order, si)
		}
		g.items = append(g.items, f)
	}

	for _, si := range order {
		g := bySlide[si]
		sort.SliceStable(g.items, func(i, j int) bool {
			ri := patterns.ActionRank(g.items[i].Action)
			rj := patterns.ActionRank(g.items[j].Action)
			if ri != rj {
				return ri > rj
			}
			fi := g.items[i].Fix != nil
			fj := g.items[j].Fix != nil
			if fi != fj {
				return fi
			}
			return false
		})
	}

	var result []fitFinding
	for _, si := range order {
		g := bySlide[si]
		if len(g.items) <= budget {
			result = append(result, g.items...)
			continue
		}
		result = append(result, g.items[:budget]...)
		suppressed := g.items[budget:]
		path := slidepath.Slide(si)
		if si < 0 {
			path = "/slides/?"
		}
		topCodes := localFindingCodeHistogram(suppressed)
		result = append(result, fitFinding{
			Code:    "findings_truncated",
			Path:    path,
			Message: fmt.Sprintf("%d more findings suppressed on this slide; use --verbose-fit to see all", len(suppressed)),
			Action:  "info",
			Fix: &patterns.FixSuggestion{
				Kind: "truncation_summary",
				Params: map[string]any{
					"suppressed_count": len(suppressed),
					"top_codes":        topCodes,
				},
			},
		})
	}

	// Mirror collectFitFindings' canonical post-budget order:
	// (severity desc, slide_index asc, code asc).
	sortLocalFitFindingsCanonical(result)

	return result
}

// sortLocalFitFindingsCanonical sorts the local fitFinding slice into the
// canonical (severity_desc, slide_index_asc, code_asc) order — the local-type
// equivalent of patterns.SortCanonical.
func sortLocalFitFindingsCanonical(findings []fitFinding) {
	if len(findings) <= 1 {
		return
	}
	sort.Slice(findings, func(i, j int) bool {
		ri := patterns.ActionRank(findings[i].Action)
		rj := patterns.ActionRank(findings[j].Action)
		if ri != rj {
			return ri > rj
		}
		si := slidepath.SlideIndex(findings[i].Path)
		sj := slidepath.SlideIndex(findings[j].Path)
		if si != sj {
			return si < sj
		}
		return findings[i].Code < findings[j].Code
	})
}

// convertTextFitFinding converts a local fitFinding to patterns.FitFinding.
func convertTextFitFinding(tf fitFinding) patterns.FitFinding {
	f := patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    tf.Path,
			Code:    tf.Code,
			Message: tf.Message,
			Fix:     tf.Fix,
		},
		Action: tf.Action,
	}
	if tf.RequiredPt > 0 || tf.AllocatedPt > 0 {
		f.Measured = &patterns.Extent{HeightEMU: int64(tf.RequiredPt * 12700)}
		f.Allowed = &patterns.Extent{HeightEMU: int64(tf.AllocatedPt * 12700)}
		if tf.AllocatedPt > 0 {
			f.OverflowRatio = tf.RequiredPt / tf.AllocatedPt
		}
	}
	return f
}

// collectStructuralFindings runs placeholder overflow, title wraps, footer
// collision, and bounds overflow detectors using template layout data.
func collectStructuralFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []patterns.FitFinding {
	var findings []patterns.FitFinding
	rhythm := resolvedValidRhythmGrid(input, layouts, slideWidth, slideHeight)

	footerEnabled := footerConfigForInput(input, len(input.Slides)) != nil
	var skipFooterByLayout map[string]bool
	if input.Chrome != nil {
		specs := make([]generator.SlideSpec, len(layouts))
		for i := range layouts {
			specs[i].LayoutID = layouts[i].ID
		}
		applyChromeSkip(specs, input.Chrome, input.Slides, layouts)
		skipFooterByLayout = make(map[string]bool, len(layouts))
		for i := range specs {
			skipFooterByLayout[specs[i].LayoutID] = specs[i].SkipFooter
		}
	}

	for si, slide := range input.Slides {
		layout := findLayoutForSlide(&slide, layouts)

		// Placeholder overflow and title wraps.
		if layout != nil {
			findings = append(findings, checkPlaceholderFindings(&slide, si, layout)...)
		}

		// Takeaway / source band: the band stack is derived from the layout's
		// geometry; when it cannot fit, render skips it rather than overlap.
		if f := checkChromeBandFit(&slide, si, layouts, slideWidth, slideHeight); f != nil {
			findings = append(findings, *f)
		}

		// Shape grid: title/footer collision and bounds overflow. Resolve the
		// SAME layout-aware geometry generation uses (resolveGridGeometry), so
		// virtual-layout slides (no explicit layout_id) are checked too and the
		// resolved cell bounds match render — the gap that let title overlaps
		// slip past preflight (go-slide-creator-s1rd).
		if slide.ShapeGrid != nil {
			geom, _ := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, rhythm)
			// Footer derivation needs the concrete layout when present, else the
			// virtual base layout the geometry resolved.
			footerLayout := layout
			if footerLayout == nil {
				footerLayout = findLayoutByID(layouts, geom.LayoutID)
			}
			patternName := ""
			if slide.Pattern != nil {
				patternName = slide.Pattern.Name
			}
			findings = append(findings,
				checkShapeGridStructural(slide.ShapeGrid, si, slideWidth, slideHeight, footerLayout, geom,
					footerEnabled && (footerLayout == nil || !skipFooterByLayout[footerLayout.ID]), patternName)...)
		}
	}

	return findings
}

// checkChromeBandFit emits chrome_band_no_fit when a slide's takeaway/source
// band stack cannot be placed on its layout (the same layout-derived frame the
// generator emits the band into) without climbing into the title or starving
// the layout's content placeholders. Returns nil when the slide has no
// takeaway/source, no layouts are known, or the band fits.
func checkChromeBandFit(slide *SlideInput, si int, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) *patterns.FitFinding {
	if (slide.Takeaway == "" && slide.Source == "") || len(layouts) == 0 {
		return nil
	}
	frame := slideChromeFrame(*slide, "", layouts, slideWidth, slideHeight)
	if frame.Fits {
		return nil
	}
	field := "takeaway"
	if slide.Takeaway == "" {
		field = "source"
	}
	f := &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    slidepath.SlideField(si, field),
			Code:    patterns.ErrCodeChromeBandNoFit,
			Message: fmt.Sprintf("slide %d: the takeaway/source band does not fit layout %q (it would overlap the title or leave too little room for content); the band is skipped at render", si+1, slide.LayoutID),
		},
		Action: "review",
	}
	// Suggest the template's One Content layout, whose body column the band
	// is designed around, when the slide is not already on it.
	if ref := template.ChromeReferenceLayout(layouts); ref != nil && ref.ID != slide.LayoutID {
		f.Fix = &patterns.FixSuggestion{Kind: "swap_layout", Params: map[string]any{"layout_id": ref.ID}}
	}
	return f
}

// checkPlaceholderFindings checks a slide's content placeholders for title
// wraps, body overflow, and (for diagram content) intrinsic aspect conflicts.
func checkPlaceholderFindings(slide *SlideInput, si int, layout *types.LayoutMetadata) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for ci, content := range slide.Content {
		ph := findPlaceholderByID(content.PlaceholderID, layout.Placeholders)
		if ph == nil || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
			continue
		}

		paragraphs := extractContentParagraphs(&content)
		if len(paragraphs) == 0 {
			continue
		}

		path := slidepath.ContentIndex(si, ci)

		// Titles are measured by collectTitleFitFindings (title_measure.go).
		if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
			if f := generator.DetectPlaceholderOverflow(generator.PlaceholderOverflowInput{
				SlideIndex:  si,
				Path:        path,
				Paragraphs:  paragraphs,
				WidthEMU:    ph.Bounds.Width,
				HeightEMU:   ph.Bounds.Height,
				FontSizeHPt: effectivePlaceholderFontSizeHPt(&content, ph, len(paragraphs)),
				FontName:    ph.FontFamily,
			}); f != nil {
				findings = append(findings, *f)
			}
		}
	}
	return findings
}

func authoredOrTemplateFontSizeHPt(content *ContentInput, templateHPt int) int {
	if content.FontSize != nil && *content.FontSize > 0 {
		return int(*content.FontSize * 100)
	}
	return templateHPt
}

func contentUsesBodyTypography(content *ContentInput, ph *types.PlaceholderInfo) bool {
	if content.FontSize != nil && *content.FontSize > 0 || ph.FontSize > 4000 {
		return false
	}
	id := strings.ToLower(content.PlaceholderID)
	if id == "title" || strings.HasPrefix(id, "title_") || strings.Contains(id, "subtitle") {
		return false
	}
	switch content.Type {
	case "text", "bullets", "body_and_bullets", "bullet_groups":
		return true
	default:
		return false
	}
}

func effectivePlaceholderFontSizeHPt(content *ContentInput, ph *types.PlaceholderInfo, paragraphs int) int {
	if contentUsesBodyTypography(content, ph) {
		size, _ := generator.BodySizeForDensity(ph.FontSize, paragraphs)
		return size
	}
	return authoredOrTemplateFontSizeHPt(content, ph.FontSize)
}

// gridContext holds pre-computed layout data for shape grid structural checks.
type gridContext struct {
	gridX, gridY         int64
	footerY, footerCY    int64
	layoutDeclaresFooter bool
	footerEnabled        bool
	// titleBottom is the Y of the resolved content zone's title bottom edge
	// (EMU). layoutDeclaresTitle gates the title-collision detector so it only
	// fires when a real title-anchored zone was resolved.
	titleBottom         int64
	layoutDeclaresTitle bool
	slideWidth          int64
	slideHeight         int64
}

// resolveGridContext extracts footer and grid origin data from a layout. The
// layout may be nil (e.g. a compose-merged grid resolved without template
// layouts); footer/grid-origin defaults are then left in place.
func resolveGridContext(grid *ShapeGridInput, layout *types.LayoutMetadata, slideWidth, slideHeight int64, footerEnabled bool) gridContext {
	ctx := gridContext{
		gridX:         457200,  // 0.5 inch default
		gridY:         1600200, // ~1.26 inch default (below title)
		footerEnabled: footerEnabled,
		slideWidth:    slideWidth,
		slideHeight:   slideHeight,
	}

	if layout != nil {
		if top, ok := template.FooterTop(layout, slideHeight); ok {
			ctx.footerY = top
			ctx.footerCY = slideHeight - top
			ctx.layoutDeclaresFooter = true
		}
		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderOther && ph.Bounds.Height > 0 && !ctx.layoutDeclaresFooter {
				ctx.footerY = ph.Bounds.Y
				ctx.footerCY = ph.Bounds.Height
				ctx.layoutDeclaresFooter = true
			}
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
				ctx.gridX = ph.Bounds.X
				ctx.gridY = ph.Bounds.Y
			}
		}
	}

	if grid.Bounds != nil {
		if grid.Bounds.X > 0 {
			sw := slideWidth
			if sw <= 0 {
				sw = shapegrid.DefaultSlideWidthEMU
			}
			ctx.gridX = int64(float64(sw) * grid.Bounds.X / 100.0)
		}
		if grid.Bounds.Y > 0 {
			sh := slideHeight
			if sh <= 0 {
				sh = shapegrid.DefaultSlideHeightEMU
			}
			ctx.gridY = int64(float64(sh) * grid.Bounds.Y / 100.0)
		}
	}

	return ctx
}

// checkShapeGridStructural checks shape_grid cells for title collision, footer
// collision, bounds overflow, sparse layout, and approximable non-text visual
// issues using resolved cell positions from shapegrid.Resolve. The grid is
// resolved with the SAME layout-aware geometry (geom) generation uses, so the
// cells evaluated here match render coordinates (go-slide-creator-s1rd).
//
// Non-text grid findings approximable at preflight:
//   - grid_diagram_narrow: complex diagram in a narrow cell (same logic as render-time)
//
// Non-text grid findings that remain render-time-only:
//   - diagram_clamped: requires actual SVG render to know output dimensions
//   - diagram_render_failed: only knowable when rendering is attempted
//   - image file/dimension issues: require filesystem access and image decoding
//   - icon resolution failures: require loading SVG icons from the registry
func checkShapeGridStructural(grid *ShapeGridInput, slideIdx int, slideWidth, slideHeight int64, layout *types.LayoutMetadata, geom GridGeometry, footerEnabled bool, patternName string) []patterns.FitFinding {
	if len(grid.Rows) == 0 {
		return nil
	}

	ctx := resolveGridContext(grid, layout, slideWidth, slideHeight, footerEnabled)

	// Inject the resolved content zone's title edge so checkCellStructural can
	// flag cells that intrude upward into the title chrome (go-slide-creator-s1rd).
	if geom.Zone != nil && geom.Zone.TitleBottom > 0 {
		ctx.titleBottom = geom.Zone.TitleBottom
		ctx.layoutDeclaresTitle = true
	}

	// Resolve the grid to get authoritative cell bounds, using the SAME
	// layout-aware geometry generation applies (zone clamp / override bounds)
	// so the cells preflight evaluates land where they will render.
	result := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)

	var findings []patterns.FitFinding

	if result != nil {
		findings = append(findings, checkGridCellsStructural(grid, result, slideIdx, slideWidth, slideHeight, slidepath.ShapeGrid(slideIdx), ctx, 0)...)
	}

	// Sparse layout detection: bounds are authoritative (never shrink), so
	// content may occupy a small fraction of the allocated bounds.
	bounds := resolveGridBounds(grid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
	if f := detectSparseLayoutForGrid(grid, result, bounds, slideIdx, patternName); f != nil {
		findings = append(findings, *f)
	}

	return findings
}

func checkGridCellsStructural(grid *ShapeGridInput, result *shapegrid.ResolveResult, slideIdx int, slideWidth, slideHeight int64, base string, ctx gridContext, depth int) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for _, rc := range result.Cells {
		if rc.RowIdx < 0 || rc.RowIdx >= len(grid.Rows) {
			continue
		}
		cell := gridCellAtResolved(grid, rc.RowIdx, rc.ColIdx)
		if cell == nil {
			continue
		}
		path := fmt.Sprintf("%s/rows/%d/cells/%d", base, rc.RowIdx, rc.ColIdx)
		if cell.Shape != nil || cell.Table != nil || cell.Icon != nil || cell.Image != nil || cell.Diagram != nil {
			findings = append(findings, checkCellStructural(path, slideIdx, rc.CellBounds.X, rc.CellBounds.Y, rc.CellBounds.CX, rc.CellBounds.CY, ctx)...)
		}
		if cell.Diagram != nil {
			findings = append(findings, checkGridDiagramPreflightPath(cell.Diagram, path+"/diagram", rc.CellBounds.CX, rc.CellBounds.CY, rc.Bounds.CX, rc.Bounds.CY)...)
		}
		if rc.Kind == shapegrid.CellKindSubGrid && cell.Grid != nil && depth < maxGeomNestingDepth {
			bounds := pptx.RectEmu{X: rc.Bounds.X + subGridInsetEMU, Y: rc.Bounds.Y + subGridInsetEMU, CX: rc.Bounds.CX - 2*subGridInsetEMU, CY: rc.Bounds.CY - 2*subGridInsetEMU}
			if bounds.CX <= 0 || bounds.CY <= 0 {
				bounds = rc.Bounds
			}
			if sub := resolveGridForStructural(cell.Grid, &bounds, nil, slideWidth, slideHeight); sub != nil {
				findings = append(findings, checkGridCellsStructural(cell.Grid, sub, slideIdx, slideWidth, slideHeight, path+"/grid", ctx, depth+1)...)
			}
		}
	}
	return findings
}

// checkGridDiagramPreflightPath runs the diagram preflight detectors (narrow cell
// legibility, explicit-spec aspect mismatch, and natural-aspect conflict) for
// one resolved grid cell carrying a diagram. Extracted from
// checkShapeGridStructural to keep that function's cognitive complexity under
// the gocognit lint threshold.
//
// cellCX/cellCY are the original (pre-fit) cell allocation; renderCX/renderCY are
// the post-fit frame the diagram is sized into. Legibility and conflict checks
// use the render frame (matching render-time findings); the aspect-mismatch check
// receives both so its evidence can separate authoring intent from the fit frame.
func checkGridDiagramPreflightPath(diagram *types.DiagramSpec, diagPath string, cellCX, cellCY, renderCX, renderCY int64) []patterns.FitFinding {
	if diagram == nil {
		return nil
	}
	var findings []patterns.FitFinding
	if f := generator.CheckDiagramInNarrowBoundsFinding(diagram, renderCX, diagPath); f != nil {
		findings = append(findings, *f)
	}
	cellBox := types.BoundingBox{Width: cellCX, Height: cellCY}
	renderBox := types.BoundingBox{Width: renderCX, Height: renderCY}
	if f := generator.CheckDiagramAspectMismatchFinding(diagram, cellBox, renderBox, diagPath); f != nil {
		findings = append(findings, *f)
	}
	// Non-chart diagrams only — chart aspect issues come from svggen dry-render.
	return findings
}

// resolveGridForStructural builds and resolves a shapegrid.Grid from a
// ShapeGridInput. Bounds are computed via the shared resolveGridBounds helper,
// so passing overrideBounds/zone yields the SAME geometry generation renders;
// passing nil/nil reduces to the legacy "explicit bounds or DefaultBounds"
// behavior. Returns nil if resolution fails.
func resolveGridForStructural(grid *ShapeGridInput, overrideBounds *pptx.RectEmu, zone *shapegrid.ContentZone, slideWidth, slideHeight int64) *shapegrid.ResolveResult {
	colWidths, err := resolveColumnsDTO(grid.Columns, grid.Rows)
	if err != nil {
		return nil
	}

	colGap := grid.ColGap
	if colGap == 0 {
		colGap = grid.Gap
	}
	rowGap := grid.RowGap
	if rowGap == 0 {
		rowGap = grid.Gap
	}

	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}

	bounds := resolveGridBounds(grid, overrideBounds, zone, slideWidth, slideHeight)

	vAlign, _ := shapegrid.ParseVerticalAlign(grid.VerticalAlign)
	sgGrid := &shapegrid.Grid{
		Bounds:    bounds,
		TypeScale: grid.TypeScale,
		Columns:   colWidths,
		Rows:      convertGridRows(grid.Rows),
		ColGap:    colGap,
		RowGap:    rowGap,
		VAlign:    vAlign,
	}

	if vErr := shapegrid.Validate(sgGrid); vErr != nil {
		return nil
	}

	alloc := pptx.NewShapeIDAllocator(nil)
	result, err := shapegrid.Resolve(sgGrid, alloc)
	if err != nil {
		return nil
	}
	return result
}

// detectSparseLayoutForGrid uses the same resolved geometry as rendering. A
// painted card occupies its frame; unfilled text uses its wrapped ink height.
func detectSparseLayoutForGrid(grid *ShapeGridInput, resolved *shapegrid.ResolveResult, bounds pptx.RectEmu, slideIdx int, patternName string) *patterns.FitFinding {
	// Named patterns size their own cells: a KPI card is 2.6in tall because the
	// pattern says so, not because its two short paragraphs need the room. This
	// estimator measures TEXT height against bounds height, so it called a clean
	// three-card KPI slide "6% filled" and, once breadth started counting, that
	// false positive blocked a good deck. The real emptiness signals on pattern
	// slides are SPARSE_FILL and SLIDE_UNDERUSED, which measure resolved ink
	// against resolved geometry (go-slide-creator-q7ar).
	if patternName != "" {
		return nil
	}
	if resolved == nil || bounds.CX <= 0 || bounds.CY <= 0 {
		return nil
	}
	contentH := measuredGridContentHeightEMU(grid, resolved, bounds, 0)

	// Count filled slots and grid dimensions for reshape recommendation.
	numCols := inferGridColumns(grid)
	numRows := len(grid.Rows)
	filledSlots := occupiedGridSlots(grid)

	path := slidepath.ShapeGrid(slideIdx)
	f := generator.DetectSparseLayout(generator.SparseLayoutInput{
		SlideIndex:       slideIdx,
		Path:             path,
		BoundsHeightEMU:  bounds.CY,
		ContentHeightEMU: contentH,
		AreaMeasured:     true,
		PatternName:      patternName,
		FilledSlots:      filledSlots,
		GridRows:         numRows,
		GridCols:         numCols,
	})
	if f != nil && patternName == "" {
		// A raw grid has no pattern sizing contract to tighten. Recommend
		// choosing a purpose-sized pattern with the same real content.
		f.Fix = &patterns.FixSuggestion{Kind: "adopt_pattern", Params: map[string]any{
			"filled_pct": f.OverflowRatio, "filled_slots": filledSlots,
			"grid_rows": numRows, "grid_cols": numCols,
		}}
	}
	return f
}

// measuredGridContentHeightEMU is painted area divided by grid width: an
// equivalent height that accounts for both row height and horizontal coverage.
func measuredGridContentHeightEMU(grid *ShapeGridInput, resolved *shapegrid.ResolveResult, bounds pptx.RectEmu, depth int) int64 {
	if resolved == nil || bounds.CX <= 0 || bounds.CY <= 0 {
		return 0
	}
	densities := textcapacity.ForResolvedGrid(resolved)
	var paintedArea float64
	for i, cell := range resolved.Cells {
		visible := clippedGridCellBounds(cell.Bounds, bounds)
		if visible.CX <= 0 || visible.CY <= 0 {
			continue
		}
		if cell.Kind == shapegrid.CellKindSubGrid && depth < maxGeomNestingDepth {
			if authored := gridCellAtResolved(grid, cell.RowIdx, cell.ColIdx); authored != nil && authored.Grid != nil {
				childBounds := pptx.RectEmu{X: cell.Bounds.X + subGridInsetEMU, Y: cell.Bounds.Y + subGridInsetEMU,
					CX: cell.Bounds.CX - 2*subGridInsetEMU, CY: cell.Bounds.CY - 2*subGridInsetEMU}
				if childBounds.CX <= 0 || childBounds.CY <= 0 {
					childBounds = cell.Bounds
				}
				child := resolveGridForStructural(authored.Grid, &childBounds, nil, 0, 0)
				childH := measuredGridContentHeightEMU(authored.Grid, child, childBounds, depth+1)
				childVisible := clippedGridCellBounds(childBounds, bounds)
				paintedArea += float64(childVisible.CX) * float64(sparseMin64(childVisible.CY, childH))
			}
			continue
		}
		inkH := int64(math.Round(densities[i].RequiredHeightPt * 12700))
		if cell.Kind != shapegrid.CellKindShape && cell.Kind != shapegrid.CellKindSubGrid || cell.ShapeSpec != nil && sparseVisibleFill(cell.ShapeSpec.Fill) {
			inkH = visible.CY
		}
		if cell.IconBounds.CY > inkH {
			inkH = cell.IconBounds.CY
		}
		inkH = sparseMax64(0, sparseMin64(visible.CY, inkH))
		paintedArea += float64(visible.CX) * float64(inkH)
	}
	return int64(math.Round(math.Min(float64(bounds.CY), paintedArea/float64(bounds.CX))))
}

func clippedGridCellBounds(cell, bounds pptx.RectEmu) pptx.RectEmu {
	x1 := sparseMax64(cell.X, bounds.X)
	y1 := sparseMax64(cell.Y, bounds.Y)
	x2 := sparseMin64(cell.X+cell.CX, bounds.X+bounds.CX)
	y2 := sparseMin64(cell.Y+cell.CY, bounds.Y+bounds.CY)
	return pptx.RectEmu{X: x1, Y: y1, CX: sparseMax64(0, x2-x1), CY: sparseMax64(0, y2-y1)}
}

func sparseMin64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func sparseMax64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func sparseVisibleFill(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var name string
	if json.Unmarshal(raw, &name) == nil {
		return !sparseInvisibleFillName(name)
	}
	var fill struct {
		Color string   `json:"color"`
		Alpha *float64 `json:"alpha"`
	}
	if json.Unmarshal(raw, &fill) != nil || sparseInvisibleFillName(fill.Color) {
		return false
	}
	if fill.Alpha == nil {
		return true
	}
	if *fill.Alpha <= 1 {
		return *fill.Alpha >= 0.2
	}
	return *fill.Alpha >= 20
}

func sparseInvisibleFillName(name string) bool {
	switch strings.ToLower(strings.TrimPrefix(name, "#")) {
	case "", "none", "transparent":
		return true
	}
	return false
}

// checkCellStructural runs bounds overflow and footer collision on one cell.
func checkCellStructural(path string, slideIdx int, x, y, cx, cy int64, ctx gridContext) []patterns.FitFinding {
	var findings []patterns.FitFinding

	if f := generator.DetectSlideBoundsOverflow(generator.BoundsCheckInput{
		SlideIndex:  slideIdx,
		Path:        path,
		X:           x,
		Y:           y,
		CX:          cx,
		CY:          cy,
		SlideWidth:  ctx.slideWidth,
		SlideHeight: ctx.slideHeight,
	}); f != nil {
		findings = append(findings, *f)
	}

	if ctx.layoutDeclaresTitle {
		if f := generator.DetectTitleCollision(generator.TitleCollisionInput{
			SlideIndex:          slideIdx,
			Path:                path,
			ShapeX:              x,
			ShapeY:              y,
			ShapeCX:             cx,
			ShapeCY:             cy,
			TitleBottom:         ctx.titleBottom,
			LayoutDeclaresTitle: true,
			StrictFit:           "warn",
		}); f != nil {
			findings = append(findings, *f)
		}
	}

	if ctx.footerEnabled && ctx.layoutDeclaresFooter {
		if f := generator.DetectFooterCollision(generator.FooterCollisionInput{
			SlideIndex:           slideIdx,
			Path:                 path,
			ShapeX:               x,
			ShapeY:               y,
			ShapeCX:              cx,
			ShapeCY:              cy,
			FooterY:              ctx.footerY,
			FooterCY:             ctx.footerCY,
			LayoutDeclaresFooter: true,
			StrictFit:            "warn",
		}); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings
}

// findLayoutForSlide resolves the layout metadata for a slide input.
func findLayoutForSlide(slide *SlideInput, layouts []types.LayoutMetadata) *types.LayoutMetadata {
	if slide.LayoutID == "" {
		return nil
	}
	for i := range layouts {
		if layouts[i].ID == slide.LayoutID {
			return &layouts[i]
		}
	}
	return nil
}

// findPlaceholderByID finds a placeholder by its ID within a layout.
func findPlaceholderByID(id string, phs []types.PlaceholderInfo) *types.PlaceholderInfo {
	for i := range phs {
		if phs[i].ID == id {
			return &phs[i]
		}
	}
	return nil
}

func findContrastPlaceholderByID(id string, layout *types.LayoutMetadata) *types.PlaceholderInfo {
	if layout == nil {
		return nil
	}
	if strings.HasPrefix(id, "idx:") {
		index, err := strconv.Atoi(strings.TrimPrefix(id, "idx:"))
		if err != nil {
			return nil
		}
		for i := range layout.Placeholders {
			if layout.Placeholders[i].Index == index {
				return &layout.Placeholders[i]
			}
		}
		return nil
	}
	if placeholderrole.IsSectionNumberAlias(id) {
		// Match the generator's section-number resolver: a named frame wins
		// even when another shape has an exact alias name.
		for i := range layout.Placeholders {
			if strings.EqualFold(layout.Placeholders[i].ID, "Section Number") {
				return &layout.Placeholders[i]
			}
		}
		for i := range layout.Placeholders {
			if layout.Placeholders[i].Index == 1 && (layout.Placeholders[i].Type == types.PlaceholderBody || layout.Placeholders[i].Type == types.PlaceholderContent) {
				return &layout.Placeholders[i]
			}
		}
	}
	if ph := findPlaceholderByID(id, layout.Placeholders); ph != nil {
		return ph
	}
	if placeholderrole.IsSectionNumberAlias(id) {
		for i := range layout.Placeholders {
			if layout.Placeholders[i].Role == types.PlaceholderRoleSectionNumber {
				return &layout.Placeholders[i]
			}
		}
	}
	return nil
}

// extractContentParagraphs extracts text paragraphs from a content input.
func extractContentParagraphs(c *ContentInput) []string {
	switch c.Type {
	case "text":
		if c.TextValue != nil && *c.TextValue != "" {
			return []string{*c.TextValue}
		}
	case "bullets":
		if c.BulletsValue != nil {
			return *c.BulletsValue
		}
	case "body_and_bullets":
		if c.BodyAndBulletsValue != nil {
			var paras []string
			if c.BodyAndBulletsValue.Body != "" {
				paras = append(paras, c.BodyAndBulletsValue.Body)
			}
			paras = append(paras, c.BodyAndBulletsValue.Bullets...)
			if c.BodyAndBulletsValue.TrailingBody != "" {
				paras = append(paras, c.BodyAndBulletsValue.TrailingBody)
			}
			return paras
		}
	case "bullet_groups":
		if c.BulletGroupsValue != nil {
			var paras []string
			if c.BulletGroupsValue.Body != "" {
				paras = append(paras, c.BulletGroupsValue.Body)
			}
			for _, g := range c.BulletGroupsValue.Groups {
				if g.Header != "" {
					paras = append(paras, g.Header)
				}
				paras = append(paras, g.Bullets...)
			}
			if c.BulletGroupsValue.TrailingBody != "" {
				paras = append(paras, c.BulletGroupsValue.TrailingBody)
			}
			return paras
		}
	}
	return nil
}

// findingCodeHistogram builds a sorted "code:count" list from suppressed
// patterns.FitFinding items, ordered by count descending.
func findingCodeHistogram(items []patterns.FitFinding) []string {
	counts := map[string]int{}
	for _, f := range items {
		counts[f.Code]++
	}
	return formatCodeCounts(counts)
}

// localFindingCodeHistogram builds a sorted "code:count" list from suppressed
// fitFinding items, ordered by count descending.
func localFindingCodeHistogram(items []fitFinding) []string {
	counts := map[string]int{}
	for _, f := range items {
		counts[f.Code]++
	}
	return formatCodeCounts(counts)
}

// formatCodeCounts formats a code→count map as a sorted "code:N" string slice,
// ordered by count descending, then code ascending for stability.
func formatCodeCounts(counts map[string]int) []string {
	type codeCount struct {
		code  string
		count int
	}
	pairs := make([]codeCount, 0, len(counts))
	for code, n := range counts {
		pairs = append(pairs, codeCount{code, n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].code < pairs[j].code
	})
	result := make([]string, len(pairs))
	for i, p := range pairs {
		result[i] = fmt.Sprintf("%s:%d", p.code, p.count)
	}
	return result
}

// contrastSwapsToFindings converts generator ContrastSwap records into
// patterns.FitFinding values with action "info" and code "contrast_autofixed".
// An executable fix is offered only when the rendered shape index maps
// unambiguously to an authored raw grid cell with the original text color.
func contrastSwapsToFindings(swaps []generator.ContrastSwap, input *PresentationInput, themeColors []types.ThemeColor) []patterns.FitFinding {
	if len(swaps) == 0 {
		return nil
	}
	findings := make([]patterns.FitFinding, 0, len(swaps))
	for _, s := range swaps {
		params := map[string]any{
			"original_color":        s.OriginalColor,
			"replacement_color":     s.ReplacedColor,
			"background_color":      s.BackgroundColor,
			"contrast_ratio_before": s.RatioBefore,
			"contrast_ratio_after":  s.RatioAfter,
		}
		// Surface the offending surface so an agent can locate the swap. The
		// owning slide is carried by the finding Path (slide index is derived
		// from it like every other finding); source names the text surface.
		if s.Source != "" {
			params["source"] = s.Source
		}
		// One decision can cover several sibling cells: the colour is chosen
		// once for the group, against its worst fill, so the finding says how
		// many cells it moved rather than repeating itself per shape
		// (go-slide-creator-tnx3e).
		scope := ""
		if s.Cells > 1 {
			params["cells"] = s.Cells
			scope = fmt.Sprintf(" across %d sibling cells", s.Cells)
		}
		var fix *patterns.FixSuggestion
		if path, from, ok := authoredContrastSwapCell(s, input, themeColors); ok {
			params["from"] = from
			params["to"] = s.ReplacedColor
			params["target"] = "text"
			params["path"] = path
			fix = &patterns.FixSuggestion{Kind: "replace_color", Params: params}
		}
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: s.Path,
				Code: "contrast_autofixed",
				Message: fmt.Sprintf(
					"auto-fixed low-contrast text%s on %s: %s → %s (on %s, ratio %.1f → %.1f)",
					scope, s.Source, s.OriginalColor, s.ReplacedColor, s.BackgroundColor,
					s.RatioBefore, s.RatioAfter,
				),
				Fix: fix,
			},
			Action: "info",
		})
	}
	return findings
}

// authoredContrastSwapCell accepts the generator's /shape_grid/shapes/I path
// only for a simple raw grid where each authored cell emits exactly one XML
// shape in row-major order. Connectors, spans, groups, nested grids, and other
// cell kinds break that index mapping and therefore cannot receive a safe fix.
func authoredContrastSwapCell(s generator.ContrastSwap, input *PresentationInput, themeColors []types.ThemeColor) (string, string, bool) {
	if input == nil || s.Source != "shape_grid" || s.Cells > 1 || s.SlideIndex < 0 || s.SlideIndex >= len(input.Slides) {
		return "", "", false
	}
	slide := &input.Slides[s.SlideIndex]
	if slide.ShapeGrid == nil || slide.Pattern != nil || slide.Compose != nil {
		return "", "", false
	}
	prefix := slidepath.ShapeGrid(s.SlideIndex) + "/shapes/"
	if !strings.HasPrefix(s.Path, prefix) {
		return "", "", false
	}
	shapeIndex, err := strconv.Atoi(strings.TrimPrefix(s.Path, prefix))
	if err != nil || shapeIndex < 0 {
		return "", "", false
	}
	index := 0
	var targetPath, authoredColor string
	for ri, row := range slide.ShapeGrid.Rows {
		if row.Connector != nil {
			return "", "", false
		}
		for ci, cell := range row.Cells {
			if !simpleContrastGridCell(cell) {
				return "", "", false
			}
			if index == shapeIndex {
				for _, color := range extractShapeTextColors(cell.Shape.Text) {
					resolved, ok := themeHex(color.Color, themeColors)
					if ok && normalizeColor(resolved.Hex()) == normalizeColor(s.OriginalColor) {
						targetPath = slidepath.GridCellField(s.SlideIndex, ri, ci, "shape/text")
						authoredColor = color.Color
						break
					}
				}
			}
			index++
		}
	}
	return targetPath, authoredColor, targetPath != ""
}

func simpleContrastGridCell(cell *GridCellInput) bool {
	return cell != nil && cell.Shape != nil && !cell.Group && cell.ColSpan <= 1 && cell.RowSpan <= 1 &&
		cell.Table == nil && cell.Icon == nil && cell.Image == nil && cell.Diagram == nil &&
		cell.Composite == nil && len(cell.Pattern) == 0 && cell.Grid == nil && cell.AccentBar == nil
}

// patternRecommendedMax maps known patterns to their recommended maximum cell
// count. Patterns not listed here have no overcrowding limit enforced.
//
// The unit is GRID CELLS, not the pattern's items: a pattern that draws each
// item as a stack of cells needs its limit multiplied accordingly.
// timeline-horizontal's dots layout emits three cells per stop (date, dot,
// label), so its entry was 6 — the pattern's own stop maximum, in the wrong
// unit — and every conforming timeline, down to three stops, was reported as
// overcrowded (go-slide-creator-wrsb).
var patternRecommendedMax = map[string]int{
	"card-grid": 9,
	"icon-row":  5,
	"kpi-3up":   3,
	"kpi-4up":   4,
	// matrix-2x2 is deliberately absent: its shape is fixed at four quadrants by
	// its own validator, and the grid it expands to carries the axis shapes as
	// cells too, so a cell-count limit here can only fire on a conforming 2x2.
	"timeline-horizontal": 21, // 7 stops (the pattern's own maximum) x 3 cells

	"process-flow":    8,
	"comparison-2col": 8,
	"before-after":    8,
}

// collectGridOccupancyFindings checks authored raw-grid slot underfill and
// known-pattern overcrowding. Expander padding is not missing authored content.
// collectStructuralSmellFindings runs pipeline.DetectStructuralSmells on every
// author-written shape_grid. Pattern and compose expansions are skipped: their
// fills, gaps and accent rotation are the expander's contract (and are
// contrast-guarded there), not a choice the author can act on.
func collectStructuralSmellFindings(input *PresentationInput) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for si, slide := range input.Slides {
		if slide.ShapeGrid == nil || slide.Pattern != nil || slide.Compose != nil {
			continue
		}
		for _, w := range pipeline.DetectStructuralSmells(slide.ShapeGrid, si) {
			findings = append(findings, patterns.FitFinding{ValidationError: *w, Action: "review"})
		}
	}
	return findings
}

func collectGridOccupancyFindings(input *PresentationInput) []patterns.FitFinding {
	var findings []patterns.FitFinding

	for si, slide := range input.Slides {
		grid := slide.ShapeGrid
		if grid == nil || len(grid.Rows) == 0 {
			continue
		}

		// Determine column count.
		numCols := inferGridColumns(grid)

		// Count filled slots.
		totalSlots := len(grid.Rows) * numCols
		filledSlots := occupiedGridSlots(grid)

		if totalSlots <= 0 {
			continue
		}

		// Determine pattern name.
		patternName := ""
		if slide.Pattern != nil {
			patternName = slide.Pattern.Name
		} else if slide.Compose != nil {
			patternName = "compose"
		}

		path := slidepath.ShapeGrid(si)

		// Pattern expanders use grid slots as an implementation detail: a
		// driver tree can pad rows to align leaves, and a framework can reserve
		// a blank cell intentionally. Only raw grids have an authored slot
		// capacity. Generated grids are assessed by resolved visible ink below.
		if slide.Pattern == nil && slide.Compose == nil {
			if f := generator.DetectPatternUnderfilled(generator.GridOccupancyInput{
				SlideIndex:  si,
				Path:        path,
				PatternName: patternName,
				FilledSlots: filledSlots,
				TotalSlots:  totalSlots,
			}); f != nil {
				findings = append(findings, *f)
			}
		}

		// Check overcrowded (only for known patterns with a recommended max).
		recMax := 0
		if patternName != "" {
			recMax = patternRecommendedMax[patternName]
		}
		if recMax > 0 {
			if f := generator.DetectPatternOvercrowded(generator.GridOccupancyInput{
				SlideIndex:     si,
				Path:           path,
				PatternName:    patternName,
				FilledSlots:    filledSlots,
				TotalSlots:     totalSlots,
				RecommendedMax: recMax,
			}); f != nil {
				findings = append(findings, *f)
			}
		}
	}

	return findings
}

// occupiedGridSlots counts column slots, including those covered by a cell's
// col_span. A horizontal compose segment occupies several columns through one
// nested-grid placeholder, so counting only slice entries makes it look sparse.
func occupiedGridSlots(grid *ShapeGridInput) int {
	if grid == nil {
		return 0
	}
	total := 0
	for _, row := range grid.Rows {
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			total += max(1, cell.ColSpan)
		}
	}
	return total
}

// inferGridColumns determines the column count from the grid's Columns field
// or infers from the maximum cells per row.
func inferGridColumns(grid *ShapeGridInput) int {
	if len(grid.Columns) > 0 {
		var n float64
		if err := json.Unmarshal(grid.Columns, &n); err == nil {
			return int(n)
		}
		var arr []float64
		if err := json.Unmarshal(grid.Columns, &arr); err == nil {
			return len(arr)
		}
	}
	numCols := 0
	for _, row := range grid.Rows {
		if len(row.Cells) > numCols {
			numCols = len(row.Cells)
		}
	}
	return numCols
}

// =============================================================================
// Preflight predictors for render-time-only findings.
// =============================================================================

// collectTablePreflightFindings walks all content-level tables and shape_grid
// embedded tables, predicting render-time scaling/truncation/deficit findings
// using only the JSON content and template-resolved bounds.
func collectTablePreflightFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	var findings []patterns.FitFinding

	for si, slide := range input.Slides {
		layout := findLayoutForSlide(&slide, layouts)

		// Content-level tables: bounds inherit from the placeholder.
		for ci, content := range slide.Content {
			if content.Type != "table" {
				continue
			}
			table := resolveTableFromContent(&content)
			if table == nil {
				continue
			}
			spec := table.ToTableSpec()
			bounds := tablePlaceholderBounds(content.PlaceholderID, layout)
			pathPrefix := slidepath.ContentIndex(si, ci)
			findings = append(findings, generator.DetectTablePreflight(generator.TablePreflightInput{
				Path:    pathPrefix,
				Headers: spec.Headers,
				Rows:    spec.Rows,
				Bounds:  bounds,
			})...)
		}

		// Shape-grid embedded tables: bounds derived from the resolved grid.
		if slide.ShapeGrid != nil {
			findings = append(findings, collectGridTablePreflight(slide.ShapeGrid, si)...)
		}
	}

	return findings
}

// tablePlaceholderBounds returns the bounds for a content-level table by
// looking up the placeholder in the layout. Returns a zero box when the
// placeholder can't be found — preflight then skips row/width checks.
func tablePlaceholderBounds(placeholderID string, layout *types.LayoutMetadata) types.BoundingBox {
	if layout == nil {
		return types.BoundingBox{}
	}
	if ph := findPlaceholderByID(placeholderID, layout.Placeholders); ph != nil {
		return ph.Bounds
	}
	return types.BoundingBox{}
}

// collectGridTablePreflight walks shape_grid cells and emits table preflight
// findings for any embedded tables, using the resolved cell bounds.
func collectGridTablePreflight(grid *ShapeGridInput, slideIdx int) []patterns.FitFinding {
	result := resolveGridForStructural(grid, nil, nil, 0, 0)
	if result == nil {
		return nil
	}
	return collectGridTablePreflightResolved(grid, result, slidepath.ShapeGrid(slideIdx), 0)
}

func collectGridTablePreflightResolved(grid *ShapeGridInput, result *shapegrid.ResolveResult, base string, depth int) []patterns.FitFinding {
	var findings []patterns.FitFinding
	for _, rc := range result.Cells {
		cell := gridCellAtResolved(grid, rc.RowIdx, rc.ColIdx)
		if cell == nil {
			continue
		}
		path := fmt.Sprintf("%s/rows/%d/cells/%d", base, rc.RowIdx, rc.ColIdx)
		if cell.Table != nil {
			spec := cell.Table.ToTableSpec()
			findings = append(findings, generator.DetectTablePreflight(generator.TablePreflightInput{
				Path:    path + "/table",
				Headers: spec.Headers,
				Rows:    spec.Rows,
				Bounds: types.BoundingBox{
					X: rc.CellBounds.X, Y: rc.CellBounds.Y,
					Width: rc.CellBounds.CX, Height: rc.CellBounds.CY,
				},
			})...)
		}
		if rc.Kind == shapegrid.CellKindSubGrid && cell.Grid != nil && depth < maxGeomNestingDepth {
			bounds := pptx.RectEmu{X: rc.Bounds.X + subGridInsetEMU, Y: rc.Bounds.Y + subGridInsetEMU, CX: rc.Bounds.CX - 2*subGridInsetEMU, CY: rc.Bounds.CY - 2*subGridInsetEMU}
			if bounds.CX <= 0 || bounds.CY <= 0 {
				bounds = rc.Bounds
			}
			if sub := resolveGridForStructural(cell.Grid, &bounds, nil, 0, 0); sub != nil {
				findings = append(findings, collectGridTablePreflightResolved(cell.Grid, sub, path+"/grid", depth+1)...)
			}
		}
	}
	return findings
}

// collectTextAutofitPreflightFindings walks placeholder content (body /
// content placeholders) and emits text_trimmed / readability_trimmed
// predictions. Title placeholders are excluded — those have their own
// DetectTitleWraps detector.
func collectTextAutofitPreflightFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	var findings []patterns.FitFinding

	// A slide without an explicit layout_id still lands on a layout — the one
	// the selector picks — and its placeholder text is autofitted against THAT
	// geometry. Keying on layout_id alone meant the whole autofit prediction
	// (text_trimmed, readability_trimmed, TEXT_BELOW_READABLE_MIN) was silent
	// for every deck that lets the engine choose, which is most of them
	// (go-slide-creator-nlrg; go-slide-creator-t64e fixed the same gap for
	// measured titles).
	predicted := predictSlideLayouts(input, layouts)

	for si, slide := range input.Slides {
		layout := predicted[si]
		if layout == nil {
			continue
		}
		for ci, content := range slide.Content {
			ph := findPlaceholderByID(content.PlaceholderID, layout.Placeholders)
			if ph == nil || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
				continue
			}
			if ph.Type != types.PlaceholderBody && ph.Type != types.PlaceholderContent {
				continue
			}
			paragraphs := extractContentParagraphs(&content)
			if len(paragraphs) == 0 {
				continue
			}
			path := slidepath.ContentIndex(si, ci)
			fontSizeHPt := authoredOrTemplateFontSizeHPt(&content, ph.FontSize)
			findings = append(findings, generator.DetectTextAutofitPreflight(generator.TextAutofitPreflightInput{
				Path:          path,
				Paragraphs:    paragraphs,
				WidthEMU:      ph.Bounds.Width,
				HeightEMU:     ph.Bounds.Height,
				FontSizeHPt:   fontSizeHPt,
				FontName:      ph.FontFamily,
				NormalizeBody: contentUsesBodyTypography(&content, ph),
				// The readability policy the render-time autofit tags this text
				// with, so validate reports TEXT_BELOW_READABLE_MIN wherever
				// generate would (go-slide-creator-nlrg).
				ViewingMode: tokens.ParseViewingMode(input.ViewingMode),
				TextRole:    tokens.TextRoleBody,
				// The per-paragraph space-before the master declares. The
				// renderer budgets it whenever the shape carries no explicit
				// size, so without it the prediction under-counts the height a
				// dense list needs (go-slide-creator-nlrg).
				ExtraSpacingPt: generator.InheritedParagraphSpacingPt(ph.SpcBefPt),
			})...)
		}
	}

	return findings
}

// collectContrastPreflightFindings walks shape_grid cells that author both a
// fill color and a text color, and emits contrast_predicted findings where
// the renderer would auto-replace the text color. It also covers placeholder
// text on authored or template backgrounds and footer/page-number chrome.
func collectContrastPreflightFindings(input *PresentationInput, layouts []types.LayoutMetadata, themeColors []types.ThemeColor) []patterns.FitFinding {
	if len(themeColors) == 0 {
		return nil
	}

	pairs := placeholderContrastPairs(input, layouts, themeColors)
	predictedLayouts := predictSlideLayouts(input, layouts)
	findings := generator.DetectContrastPreflight(pairs, themeColors)
	findings = append(findings, collectChromeContrastFindings(input, layouts, predictedLayouts, themeColors)...)
	for si, slide := range input.Slides {
		if slide.ShapeGrid == nil || (slide.ContrastCheck != nil && !*slide.ContrastCheck) {
			continue
		}
		inheritedBackground := ""
		if predictedLayouts[si] != nil {
			inheritedBackground = predictedLayouts[si].BackgroundHex
			if predictedLayouts[si].BackgroundRef != "" {
				inheritedBackground = template.ResolveBackgroundRefHexWithMods(
					predictedLayouts[si].BackgroundRef, predictedLayouts[si].BackgroundMods, themeColors)
			}
		}
		gridBackground := generator.EffectiveGridBackgroundHex(backgroundSpecFor(&slide), inheritedBackground, themeColors)
		source := contrastGridSource(slide)
		cells := compiledGridContrastCells(slide.ShapeGrid, slidepath.ShapeGrid(si), 0)
		shapes := make([][]byte, len(cells))
		for i := range cells {
			shapes[i] = cells[i].xml
		}
		for _, swap := range generator.PredictCompiledGridContrast(shapes, themeColors, si, gridBackground) {
			path := swap.Path
			authoredColor := ""
			if swap.Cells < 2 {
				indexText := strings.TrimPrefix(swap.Path, slidepath.ShapeGrid(si)+"/shapes/")
				if idx, err := strconv.Atoi(indexText); err == nil && idx >= 0 && idx < len(cells) {
					path = cells[idx].path
					for _, tc := range extractShapeTextColors(cells[idx].text) {
						resolved, ok := themeHex(tc.Color, themeColors)
						if ok && strings.EqualFold(resolved.Hex(), swap.OriginalColor) {
							authoredColor = tc.Color
							break
						}
					}
				}
			}
			findings = append(findings, generator.CompiledGridSwapFinding(swap, path, authoredColor, source, themeColors))
		}
	}

	return findings
}

// unresolvedPlaceholderContrastFindings keeps unrepairable fill failures
// visible in generate responses even when the optional full fit report is off.
func unresolvedPlaceholderContrastFindings(input *PresentationInput, layouts []types.LayoutMetadata, themeColors []types.ThemeColor) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, finding := range generator.DetectContrastPreflight(placeholderContrastPairs(input, layouts, themeColors), themeColors) {
		if finding.Code == patterns.ErrCodeContrastUnresolved {
			out = append(out, finding)
		}
	}
	return out
}

func collectChromeContrastFindings(input *PresentationInput, layouts []types.LayoutMetadata, predictedLayouts []*types.LayoutMetadata, themeColors []types.ThemeColor) []patterns.FitFinding {
	if footer := footerConfigForInput(input, len(input.Slides)); footer != nil && footer.Enabled {
		specs := make([]generator.SlideSpec, len(input.Slides))
		var findings []patterns.FitFinding
		for si, layout := range predictedLayouts {
			if layout != nil {
				specs[si].LayoutID = layout.ID
			}
		}
		if input.Chrome != nil {
			applyChromeSkip(specs, input.Chrome, input.Slides, layouts)
		}
		for si, layout := range predictedLayouts {
			if layout == nil || specs[si].SkipFooter || layout.ChromeBackgroundRef == "" {
				continue
			}
			bg := template.ResolveBackgroundRefHexWithMods(layout.ChromeBackgroundRef, layout.ChromeBackgroundMods, themeColors)
			fg := template.ResolveBackgroundRefHex(layout.ChromeTextRef, themeColors)
			if swap := generator.PredictChromeContrast(bg, fg, themeColors, si); swap != nil {
				findings = append(findings, generator.CompiledGridSwapFinding(*swap, swap.Path, "", "chrome", themeColors))
			}
		}
		return findings
	}
	return nil
}

type compiledGridContrastCell struct {
	xml  []byte
	path string
	text json.RawMessage
}

// Compile the text-bearing grid shapes with the same shape XML writer that
// generation uses. Bounds affect placement, not text colors or run sizes. The
// resulting shape order also preserves sibling-group decisions.
func compiledGridContrastCells(grid *ShapeGridInput, base string, depth int) []compiledGridContrastCell {
	if grid == nil || depth > maxGeomNestingDepth {
		return nil
	}
	var cells []compiledGridContrastCell
	var nested []compiledGridContrastCell
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellPath := slidepath.Join(base, fmt.Sprintf("rows/%d/cells/%d", ri, ci))
			shape, field := cell.Shape, "shape/text"
			if cell.Composite != nil && cell.Composite.Text != nil {
				shape, field = cell.Composite.Text, "composite/text/text"
			}
			if shape != nil {
				spec := convertGridCell(&GridCellInput{Shape: shape}).Shape
				xml, err := shapegrid.GenerateShapeXML(spec, 1, pptx.RectEmu{CX: 1000000, CY: 500000})
				if err == nil {
					cells = append(cells, compiledGridContrastCell{xml: xml, path: slidepath.Join(cellPath, field), text: shape.Text})
				}
			}
			if cell.Image != nil && cell.Image.Text != nil {
				label := cell.Image.Text
				spec := &shapegrid.ImageText{
					Content: label.Content, Size: label.Size, Bold: label.Bold,
					Color: label.Color, Align: label.Align, VerticalAlign: label.VerticalAlign, Font: label.Font,
				}
				xml, err := shapegrid.GenerateImageTextXML(spec, 1, pptx.RectEmu{CX: 1000000, CY: 500000})
				if err == nil {
					cells = append(cells, compiledGridContrastCell{xml: xml, path: slidepath.Join(cellPath, "image/text")})
				}
			}
			if cell.Grid != nil {
				nested = append(nested, compiledGridContrastCells(cell.Grid, slidepath.Join(cellPath, "grid"), depth+1)...)
			}
		}
	}
	return append(cells, nested...)
}

func contrastGridSource(slide SlideInput) string {
	// These cells were expanded for measurement; repair_slide receives the
	// authored pattern/compose, not an editable shape_grid.
	if slide.Pattern != nil || slide.Compose != nil {
		return "derived_shape_grid"
	}
	return "shape_grid"
}

// transparentShapeFill distinguishes a true no-fill cell from an unsupported
// fill specification whose visible color cannot be inferred.
func transparentShapeFill(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return strings.TrimSpace(value) == "" || strings.EqualFold(strings.TrimSpace(value), "none")
	}
	var object struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		return strings.EqualFold(strings.TrimSpace(object.Color), "none")
	}
	return false
}

// placeholderContrastPairs pairs the text colour each populated placeholder
// inherits with the background the renderer checks, authored or inherited.
//
// This is the half the shape_grid walk cannot see: the JSON names no text
// colour for template placeholders. AuthorBackground preserves the renderer's
// snap-to-palette vs template-background hue-preserving replacement choice.
func placeholderContrastPairs(input *PresentationInput, layouts []types.LayoutMetadata, themeColors []types.ThemeColor) []generator.ContrastPreflightPair {
	if input == nil || len(layouts) == 0 {
		return nil
	}
	predicted := predictSlideLayouts(input, layouts)
	sectionNumbers := contrastSectionNumbers(input, layouts)

	var pairs []generator.ContrastPreflightPair
	for si := range input.Slides {
		slide := &input.Slides[si]
		// A slide that opts out of contrast checking gets no swap at generate
		// time, so predicting one here would be a finding about nothing.
		if slide.ContrastCheck != nil && !*slide.ContrastCheck {
			continue
		}
		layout := predicted[si]
		if layout == nil {
			continue
		}
		bgHex, authorBackground := effectivePlaceholderBackground(slide, layout, themeColors)
		source := "template_background"
		if authorBackground {
			source = "slide_background"
		}
		contents := injectSectionNumber(slide.Content, layout, sectionNumbers[si])
		imageFrames := authoredNativeImageFrames(slide, layout)
		for ci := range contents {
			content := &contents[ci]
			ph := findContrastPlaceholderByID(content.PlaceholderID, layout)
			if ph == nil {
				continue
			}
			if generator.NativeImageOverlapsText(*ph, imageFrames) {
				continue // Unknown image pixels are not the canvas behind them.
			}
			// A layout that leaves its placeholder colour to the master's
			// txStyles states no FontColor, and the preflight used to skip it
			// entirely — so generate swapped the colour and validate said
			// nothing (go-slide-creator-j4364). InheritedFontColor is what the
			// placeholder actually renders at, resolved through the layout's
			// clrMapOvr the same way the render-time pass resolves it.
			fg := ph.InheritedFontColor
			mods := ph.InheritedFontColorMods
			if fg == "" {
				fg = ph.FontColor
				mods = ph.FontColorMods
			}
			if fg == "" {
				continue
			}
			if len(extractContentParagraphs(content)) == 0 {
				continue
			}
			pairBackground, gradient, pairSource, pairAuthorBackground, unresolved := placeholderPairBackground(ph, bgHex, source, authorBackground, themeColors)
			if pairBackground == "" && !ph.FillGradient && !unresolved {
				continue
			}
			path := slidepath.ContentIndex(si, ci)
			if ci >= len(slide.Content) {
				path = slidepath.Content(si, ph.ID)
			}
			pairs = append(pairs, generator.ContrastPreflightPair{
				Path:                 path,
				Foreground:           fg,
				ForegroundMods:       mods,
				Background:           pairBackground,
				Backgrounds:          gradient,
				Gradient:             ph.FillGradient,
				UnresolvedBackground: unresolved,
				Source:               pairSource,
				// Bold is unknown from placeholder metadata; false is the
				// conservative reading, matching the unknown-size rule.
				TextPt:           float64(ph.FontSize) / 100.0,
				AuthorBackground: pairAuthorBackground,
			})
		}
	}
	return pairs
}

func placeholderPairBackground(ph *types.PlaceholderInfo, canvas, source string, authorBackground bool, themeColors []types.ThemeColor) (string, []string, string, bool, bool) {
	colors := resolvedPlaceholderFillColors(ph, themeColors, canvas)
	if len(colors) > 0 {
		return colors[0], colors, "placeholder_fill", false, false
	}
	if ph.FillGradient {
		return canvas, nil, "placeholder_fill", false, false
	}
	if ph.FillSolid {
		return canvas, nil, "placeholder_fill", false, true
	}
	return canvas, nil, source, authorBackground, false
}

func contrastSectionNumbers(input *PresentationInput, layouts []types.LayoutMetadata) []string {
	numbers := make([]string, len(input.Slides))
	sectionNum := 0
	for si, slide := range input.Slides {
		if isSectionSlideInput(slide, layouts) {
			sectionNum++
			numbers[si] = fmt.Sprintf("%02d", sectionNum)
		}
	}
	return numbers
}

func resolvedPlaceholderFillColors(ph *types.PlaceholderInfo, themeColors []types.ThemeColor, canvas string) []string {
	if ph == nil || len(ph.FillStops) == 0 {
		return nil
	}
	colors := make([]string, 0, len(ph.FillStops))
	for _, stop := range ph.FillStops {
		// A translucent gradient stop composites over the slide beneath it;
		// resolving it over theme lt1 would make a confident but false verdict.
		if ph.FillGradient && stop.Mods.HasAlpha && stop.Mods.Alpha < 100000 {
			return nil
		}
		color := template.ResolveBackgroundRefHexWithMods(stop.Ref, stop.Mods, themeColors, canvas)
		if color == "" {
			return nil
		}
		colors = append(colors, color)
	}
	return colors
}

func effectivePlaceholderBackground(slide *SlideInput, layout *types.LayoutMetadata, themeColors []types.ThemeColor) (string, bool) {
	if bg := generator.EffectiveSlideBackgroundHex(backgroundSpecFor(slide), themeColors); bg != "" {
		return bg, true
	}
	if slide.Background != nil && slide.Background.Image != "" {
		return "", false // Photo without an opaque scrim has no solid canvas.
	}
	if layout.BackgroundRef != "" {
		return template.ResolveBackgroundRefHexWithMods(layout.BackgroundRef, layout.BackgroundMods, themeColors), false
	}
	return layout.BackgroundHex, false
}

// backgroundSpecFor converts a slide's authored background into the renderer's
// own background type, so the preflight resolves the effective colour through
// exactly the code generate uses.
func backgroundSpecFor(slide *SlideInput) *generator.BackgroundImage {
	bg := slide.Background
	if bg == nil || (bg.Image == "" && bg.Color == "") {
		return nil
	}
	spec := &generator.BackgroundImage{Path: bg.Image, Fit: bg.Fit, Color: bg.Color}
	if bg.Overlay != nil {
		spec.Overlay = &generator.BackgroundOverlay{Color: bg.Overlay.Color, Alpha: bg.Overlay.Alpha}
	}
	return spec
}

// extractShapeFillColor extracts a color string from a shape fill RawMessage.
// Returns "" when fill is empty, "none", or unparseable.
func extractShapeFillColor(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// String form.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, "none") {
			return ""
		}
		return s
	}
	// Object form.
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Color != "" {
		c := strings.TrimSpace(obj.Color)
		if strings.EqualFold(c, "none") {
			return ""
		}
		return c
	}
	return ""
}

// extractShapeTextColors extracts all authored text colors from a shape text
// RawMessage. A single object-form text contributes one color (if set); a
// paragraphs-array form contributes one color per paragraph that sets one.
// String-form text contributes nothing (no authored color).
func extractShapeTextColors(raw json.RawMessage) []shapeTextColor {
	if len(raw) == 0 {
		return nil
	}
	// String form has no color.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return nil
	}
	// Object / paragraphs-array form.
	var obj struct {
		Color      string `json:"color"`
		Paragraphs []struct {
			Color string `json:"color"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var colors []shapeTextColor
	// ResolveTextInput ignores the outer color whenever paragraph-form text is
	// present. Predicting contrast for it would warn about a run never drawn.
	if len(obj.Paragraphs) == 0 && obj.Color != "" {
		colors = append(colors, shapeTextColor{Color: obj.Color})
	}
	for _, p := range obj.Paragraphs {
		if p.Color == "" {
			continue
		}
		colors = append(colors, shapeTextColor{Color: p.Color})
	}
	return colors
}

// shapeTextColor records the authored spelling of a rendered text color so a
// prediction can offer a repair against the correct JSON field.
type shapeTextColor struct {
	Color string
}

// expandComposeForPreflight returns a shallow copy of input where each slide
// whose Compose envelope has not yet been materialized into ShapeGrid has its
// ShapeGrid populated by running the compose merge. This lets preflight
// detectors (checkShapeGridStructural in particular) see post-merge cell
// geometry, so a diagram in a horizontally-narrowed compose segment trips
// grid_diagram_narrow and diagram_aspect_* findings rather than passing
// silently.
//
// The expansion uses a minimal ExpandContext (no template metadata, default
// accent strategy) because the structural detectors targeted by this pass
// depend only on cell rectangles. Compose envelopes that fail to expand are
// left as-is — render-time will surface the parse error separately, and we
// avoid masking it here.
//
// When no slide has an unexpanded compose envelope, the original input is
// returned unchanged (no allocation).
func expandComposeForPreflight(input *PresentationInput, slideWidth, slideHeight int64, layoutSets ...types.LayoutMetadata) *PresentationInput {
	expanded, _ := expandComposeForPreflightWithTheme(input, slideWidth, slideHeight, nil, layoutSets...)
	return expanded
}

func expandComposeForPreflightWithTheme(input *PresentationInput, slideWidth, slideHeight int64, theme *types.ThemeInfo, layoutSets ...types.LayoutMetadata) (*PresentationInput, []patterns.FitFinding) {
	if input == nil {
		return nil, nil
	}
	needsExpansion := false
	for i := range input.Slides {
		s := &input.Slides[i]
		if s.Compose != nil && s.ShapeGrid == nil {
			needsExpansion = true
			break
		}
	}
	if !needsExpansion {
		return input, nil
	}

	expanded := *input
	expanded.Slides = make([]SlideInput, len(input.Slides))
	copy(expanded.Slides, input.Slides)
	rhythm := resolvedValidRhythmGrid(input, layoutSets, slideWidth, slideHeight)
	sectionIndices := slideSectionIndices(input.Slides, layoutSets)
	var findings []patterns.FitFinding

	for i := range expanded.Slides {
		s := &expanded.Slides[i]
		if s.Compose == nil || s.ShapeGrid != nil {
			continue
		}
		ctx := patterns.ExpandContext{
			SlideWidth:     slideWidth,
			SlideHeight:    slideHeight,
			SlideIndex:     i,
			SectionIndex:   sectionIndices[i],
			AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
		}
		if theme != nil {
			ctx.Theme = *theme
		}
		if len(layoutSets) > 0 {
			geom, b := patternExpansionGeometry(*s, layoutSets, slideWidth, slideHeight, rhythm)
			ctx.LayoutBounds = patterns.LayoutBounds{X: b.X, Y: b.Y, Width: b.CX, Height: b.CY}
			ctx.ContentZone = geom.Zone
		}
		eg, warnings, err := expandCompose(s.Compose, ctx, patterns.Default())
		if err != nil {
			continue
		}
		for _, warning := range warnings {
			if f := composeWarningAsFinding(i, warning); f != nil {
				findings = append(findings, *f)
			}
		}
		s.ShapeGrid = eg
	}
	return &expanded, findings
}

// dedupFitFindings removes findings that share the same
// (Code, Path, Action, Message) tuple. The first occurrence is kept, preserving
// caller insertion order. Same-code findings on different paths are kept,
// since they describe different cells. This guards against double-emission
// when pre-compose detectors and the post-compose structural pass both
// surface a finding for the same diagram cell.
func dedupFitFindings(in []patterns.FitFinding) []patterns.FitFinding {
	if len(in) <= 1 {
		return in
	}
	type key struct{ code, path, action, msg string }
	seen := make(map[key]struct{}, len(in))
	out := make([]patterns.FitFinding, 0, len(in))
	for _, f := range in {
		k := key{f.Code, f.Path, f.Action, f.Message}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, f)
	}
	return out
}
