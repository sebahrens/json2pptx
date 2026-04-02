// Package layout provides layout selection logic for slides.
package layout

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// loggerFromContext returns a logger from context or the default logger.
func loggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	// Check for logger in context (using standard approach)
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// loggerKey is the context key for the logger.
type loggerKey struct{}

// ContextWithLogger returns a context with the given logger.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// SelectionRequest contains all information needed for layout selection.
type SelectionRequest struct {
	Slide   types.SlideDefinition  // From parser
	Layouts []types.LayoutMetadata // From template analyzer
	Context SelectionContext       // Presentation context
	Theme   *types.ThemeInfo       // Template theme info (optional)
}

// SelectionContext provides presentation-level context for selection.
type SelectionContext struct {
	Position     int            // Slide index (0-based)
	TotalSlides  int            // Total presentation length
	PreviousType string         // Type of previous slide layout
	Theme        string         // Presentation theme name
	UsedLayouts  map[string]int // Track layout usage counts for variety bonus (optional)
}

// SelectionResult contains the selected layout and associated metadata.
type SelectionResult struct {
	LayoutID   string           // Selected layout ID
	Confidence float64          // 0.0-1.0 confidence score
	Reasoning  string           // Explanation (for debugging)
	Mappings   []ContentMapping // How content maps to placeholders
	Warnings   []string         // Potential issues
}

// ContentMapping describes how slide content maps to layout placeholders.
type ContentMapping struct {
	ContentField  string // "title", "body", "bullets", "image", "chart"
	PlaceholderID string // Target placeholder
	Truncated     bool   // Content will be truncated
	TruncateAt    int    // Character position if truncated
}

// scoredLayout holds a layout and its computed score.
type scoredLayout struct {
	layout       *types.LayoutMetadata
	score        float64
	typeScore    float64
	capScore     float64
	balScore     float64
	consScore    float64
	varietyScore float64
	reasoning    string
}

// SelectLayout chooses the best layout for a slide using heuristic rules.
// This is a convenience wrapper around SelectLayoutWithContext for backward compatibility.
func SelectLayout(req SelectionRequest) (*SelectionResult, error) {
	return SelectLayoutWithContext(context.Background(), req)
}

// SelectLayoutWithContext chooses the best layout for a slide using heuristic rules.
// Accepts a context for logging. Use ContextWithLogger to include a custom logger.
func SelectLayoutWithContext(ctx context.Context, req SelectionRequest) (*SelectionResult, error) {
	logger := loggerFromContext(ctx)

	// Validate inputs
	if len(req.Layouts) == 0 {
		logger.Debug("layout selection failed: no layouts provided",
			slog.Int("slide_index", req.Context.Position),
			slog.String("slide_type", string(req.Slide.Type)),
		)
		return nil, fmt.Errorf("no layouts provided")
	}

	logger.Debug("layout selection started",
		slog.Int("slide_index", req.Context.Position),
		slog.String("slide_type", string(req.Slide.Type)),
		slog.String("slide_title", req.Slide.Title),
		slog.Int("layout_count", len(req.Layouts)),
	)

	// Score all layouts
	scored := make([]scoredLayout, 0, len(req.Layouts))
	for i := range req.Layouts {
		sl := scoreLayout(&req.Layouts[i], req)
		scored = append(scored, sl)

		// Log individual layout scores at debug level
		logger.Debug("layout scored",
			slog.String("layout_id", req.Layouts[i].ID),
			slog.String("layout_name", req.Layouts[i].Name),
			slog.Float64("total_score", sl.score),
			slog.Float64("type_score", sl.typeScore),
			slog.Float64("capacity_score", sl.capScore),
			slog.Float64("balance_score", sl.balScore),
			slog.Float64("consistency_score", sl.consScore),
			slog.Float64("variety_score", sl.varietyScore),
		)
	}

	// Find top two scores
	if len(scored) == 0 {
		return nil, fmt.Errorf("no layouts available")
	}

	// Sort layouts by score descending to find best suitable layout
	sortScoredLayouts(scored)

	// Find the best suitable layout
	topIdx := -1
	for i := range scored {
		if isLayoutSuitable(*scored[i].layout, req.Slide) {
			topIdx = i
			break
		}
		logger.Debug("layout unsuitable, trying next",
			slog.String("layout_id", scored[i].layout.ID),
			slog.String("layout_name", scored[i].layout.Name),
			slog.Float64("score", scored[i].score),
		)
	}

	if topIdx == -1 {
		logger.Info("layout selection failed: no suitable layout",
			slog.Int("slide_index", req.Context.Position),
			slog.String("slide_type", string(req.Slide.Type)),
			slog.String("required", requiredCapability(req.Slide)),
		)
		return nil, fmt.Errorf("no suitable layout found for slide type '%s': requires %s",
			req.Slide.Type, requiredCapability(req.Slide))
	}

	top := scored[topIdx]

	// Find second-best suitable layout for confidence calculation
	secondIdx := -1
	for i := topIdx + 1; i < len(scored); i++ {
		if isLayoutSuitable(*scored[i].layout, req.Slide) {
			secondIdx = i
			break
		}
	}

	// Calculate confidence
	confidence := calculateConfidence(top.score, scored, secondIdx)

	// Build content mappings
	mappings, warnings := buildMappings(*top.layout, req.Slide)

	result := &SelectionResult{
		LayoutID:   top.layout.ID,
		Confidence: confidence,
		Reasoning:  top.reasoning,
		Mappings:   mappings,
		Warnings:   warnings,
	}

	// Log successful selection at info level
	logger.Info("layout selected",
		slog.Int("slide_index", req.Context.Position),
		slog.String("slide_type", string(req.Slide.Type)),
		slog.String("method", "heuristic"),
		slog.String("layout_id", result.LayoutID),
		slog.String("layout_name", top.layout.Name),
		slog.Float64("confidence", confidence),
		slog.Float64("score", top.score),
		slog.Int("warning_count", len(warnings)),
	)

	return result, nil
}

// RankedResult contains a layout selection result with its runner-up alternatives.
type RankedResult struct {
	Primary    *SelectionResult   // Best layout
	Alternates []*SelectionResult // Runner-up layouts (for retry on overflow)
}

// SelectLayoutRanked returns the top-N layout candidates ranked by score.
// The primary result is the best layout; alternates are suitable fallbacks
// for retry when the primary layout causes text overflow.
func SelectLayoutRanked(req SelectionRequest, maxResults int) (*RankedResult, error) {
	if len(req.Layouts) == 0 {
		return nil, fmt.Errorf("no layouts provided")
	}
	if maxResults < 1 {
		maxResults = 2
	}

	// Score and sort all layouts
	scored := make([]scoredLayout, 0, len(req.Layouts))
	for i := range req.Layouts {
		scored = append(scored, scoreLayout(&req.Layouts[i], req))
	}
	sortScoredLayouts(scored)

	// Collect top-N suitable layouts
	var suitable []scoredLayout
	for i := range scored {
		if isLayoutSuitable(*scored[i].layout, req.Slide) {
			suitable = append(suitable, scored[i])
			if len(suitable) >= maxResults {
				break
			}
		}
	}

	if len(suitable) == 0 {
		return nil, fmt.Errorf("no suitable layout found for slide type '%s': requires %s",
			req.Slide.Type, requiredCapability(req.Slide))
	}

	ranked := &RankedResult{}
	for i, sl := range suitable {
		mappings, warnings := buildMappings(*sl.layout, req.Slide)
		confidence := calculateConfidence(sl.score, scored, -1)
		result := &SelectionResult{
			LayoutID:   sl.layout.ID,
			Confidence: confidence,
			Reasoning:  sl.reasoning,
			Mappings:   mappings,
			Warnings:   warnings,
		}
		if i == 0 {
			ranked.Primary = result
		} else {
			ranked.Alternates = append(ranked.Alternates, result)
		}
	}

	return ranked, nil
}

// scoreLayout computes a weighted score for a layout.
func scoreLayout(layout *types.LayoutMetadata, req SelectionRequest) scoredLayout {
	typeScore := scoreTypeMatch(*layout, req.Slide, req.Context)
	capScore := scoreCapacity(*layout, req.Slide)
	balScore := scoreVisualBalance(*layout, req.Slide)
	consScore := scoreConsistency(*layout, req.Context)
	varietyScore := scoreVariety(*layout, req.Context)

	// Apply semantic bonus if content matches layout's semantic tags
	semanticBonus := scoreSemanticMatch(*layout, req.Slide)

	// Weighted scoring formula from spec, with semantic bonus and variety bonus
	// Variety bonus helps prevent repetitive use of the same layouts
	total := typeScore*0.35 + capScore*0.25 + balScore*0.15 + consScore*0.1 + varietyScore*0.05 + semanticBonus*0.1

	// Penalize layouts that would place complex diagrams in narrow placeholders.
	// Applied as a post-scoring penalty to override type/capacity preferences.
	narrowPenalty := penalizeNarrowDiagramSlot(*layout, req.Slide)
	total -= narrowPenalty

	// Clamp total to [0, 1]
	if total < 0.0 {
		total = 0.0
	}
	if total > 1.0 {
		total = 1.0
	}

	reasoning := fmt.Sprintf("Type:%.2f Cap:%.2f Bal:%.2f Cons:%.2f Var:%.2f Sem:%.2f = %.2f",
		typeScore, capScore, balScore, consScore, varietyScore, semanticBonus, total)

	return scoredLayout{
		layout:       layout,
		score:        total,
		typeScore:    typeScore,
		capScore:     capScore,
		balScore:     balScore,
		consScore:    consScore,
		varietyScore: varietyScore,
		reasoning:    reasoning,
	}
}

// Score constants for type matching.
const (
	scorePerfectMatch = 1.0 // Perfect match for layout type
	scoreGoodMatch    = 0.9 // Good alternative match
	scoreAcceptable   = 0.8 // Acceptable fallback (raised from 0.7 to reduce generic fallbacks)
	scorePartialMatch = 0.5 // Partial compatibility
	scorePoorMatch    = 0.4 // Low compatibility
	scoreMinimalMatch = 0.3 // Minimal compatibility
	scoreNoMatch      = 0.0 // Incompatible
)

// SemanticMatcher defines the interface for semantic pattern matching.
// Each implementation matches a specific layout pattern (quote, agenda, timeline, etc.).
type SemanticMatcher interface {
	// Tag returns the layout tag this matcher handles.
	Tag() string
	// Match checks if the slide content matches this semantic pattern.
	// Returns the score and true if the pattern matches.
	Match(title, body string, slide types.SlideDefinition) (float64, bool)
}

// semanticMatchers is the registry of all semantic matchers.
// New patterns can be added by appending to this slice.
var semanticMatchers = []SemanticMatcher{
	&quoteMatcher{},
	&sectionHeaderMatcher{},
	&agendaMatcher{},
	&timelineMatcher{},
	&bigNumberMatcher{},
	&statementMatcher{},
}

// quoteMatcher matches quote-style slides.
type quoteMatcher struct{}

func (m *quoteMatcher) Tag() string { return "quote" }

func (m *quoteMatcher) Match(title, body string, _ types.SlideDefinition) (float64, bool) {
	if strings.Contains(title, "quote") || strings.Contains(body, "\"") ||
		strings.Contains(body, "\u201C") || strings.Contains(body, "\u2014") {
		return scorePerfectMatch, true
	}
	return scoreNoMatch, false
}

// sectionHeaderMatcher matches section header slides.
type sectionHeaderMatcher struct{}

func (m *sectionHeaderMatcher) Tag() string { return "section-header" }

func (m *sectionHeaderMatcher) Match(title, _ string, _ types.SlideDefinition) (float64, bool) {
	if strings.HasPrefix(title, "section") || strings.HasPrefix(title, "part ") ||
		strings.Contains(title, "overview") || strings.Contains(title, "introduction") {
		return scorePerfectMatch, true
	}
	return scoreNoMatch, false
}

// agendaMatcher matches agenda/outline slides.
type agendaMatcher struct{}

func (m *agendaMatcher) Tag() string { return "agenda" }

func (m *agendaMatcher) Match(title, _ string, _ types.SlideDefinition) (float64, bool) {
	if strings.Contains(title, "agenda") || strings.Contains(title, "outline") ||
		strings.Contains(title, "table of contents") || strings.Contains(title, "topics") {
		return scorePerfectMatch, true
	}
	return scoreNoMatch, false
}

// timelineMatcher matches timeline/roadmap slides.
type timelineMatcher struct{}

func (m *timelineMatcher) Tag() string { return "timeline-capable" }

func (m *timelineMatcher) Match(title, _ string, _ types.SlideDefinition) (float64, bool) {
	if strings.Contains(title, "timeline") || strings.Contains(title, "roadmap") ||
		strings.Contains(title, "milestone") || strings.Contains(title, "process") ||
		strings.Contains(title, "journey") {
		return scorePerfectMatch, true
	}
	return scoreNoMatch, false
}

// bigNumberMatcher matches KPI/metrics slides with prominent numbers.
type bigNumberMatcher struct{}

func (m *bigNumberMatcher) Tag() string { return "big-number" }

func (m *bigNumberMatcher) Match(title, _ string, slide types.SlideDefinition) (float64, bool) {
	// Look for KPI-style content: short body with prominent numbers
	if len(slide.Content.Bullets) <= 3 && len(slide.Content.Body) < 100 {
		if containsProminentNumber(slide.Content.Body) ||
			strings.Contains(title, "metric") || strings.Contains(title, "kpi") ||
			strings.Contains(title, "result") || strings.Contains(title, "stats") {
			return scorePerfectMatch, true
		}
	}
	return scoreNoMatch, false
}

// statementMatcher matches single statement/impact slides.
type statementMatcher struct{}

func (m *statementMatcher) Tag() string { return "statement" }

func (m *statementMatcher) Match(_, _ string, slide types.SlideDefinition) (float64, bool) {
	if len(slide.Content.Bullets) == 0 && len(slide.Content.Body) > 0 &&
		len(slide.Content.Body) < 200 && slide.Content.ImagePath == "" {
		return scoreGoodMatch, true
	}
	return scoreNoMatch, false
}

// scoreSemanticMatch scores a layout based on semantic content-to-tag matching.
// Detects patterns in slide title/content that suggest a specific semantic layout.
// Uses registered SemanticMatcher implementations for extensibility.
func scoreSemanticMatch(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	title := strings.ToLower(slide.Title)
	body := strings.ToLower(slide.Content.Body)

	for _, matcher := range semanticMatchers {
		if hasTag(layout.Tags, matcher.Tag()) {
			if score, matched := matcher.Match(title, body, slide); matched {
				return score
			}
		}
	}

	return scoreNoMatch
}

// containsProminentNumber checks if text contains numbers that suggest metrics/KPIs.
func containsProminentNumber(text string) bool {
	// Look for percentages, large numbers, currency
	for _, r := range text {
		if r == '%' || r == '$' || r == '€' || r == '£' {
			return true
		}
	}
	// Check for digit sequences (simple heuristic)
	digitCount := 0
	for _, r := range text {
		if r >= '0' && r <= '9' {
			digitCount++
			if digitCount >= 2 {
				return true
			}
		} else {
			digitCount = 0
		}
	}
	return false
}

// scoreTypeMatch checks if layout tags match slide type and position.
func scoreTypeMatch(layout types.LayoutMetadata, slide types.SlideDefinition, ctx SelectionContext) float64 {
	// Position-based bonus for first slide
	if ctx.Position == 0 && hasTag(layout.Tags, "title-slide") {
		return scorePerfectMatch
	}

	// Position-based bonus for last slide (AC13: Last Slide Closing Layout)
	// Prefer layouts tagged with "closing" or "thank-you" for the final slide,
	// but ONLY when the slide's content type is compatible. Chart, diagram, and
	// image slides need full-area placeholders that closing layouts don't provide.
	isLastSlide := ctx.TotalSlides > 0 && ctx.Position == ctx.TotalSlides-1
	if isLastSlide {
		// Skip closing bonus for slide types that need full content area.
		// Tables (Content type with TableRaw) also need full-width body
		// placeholders that closing layouts don't provide.
		hasTable := slide.Content.TableRaw != ""
		hasDenseContent := hasTable ||
			len(slide.Content.BulletGroups) > 0 ||
			len(slide.Content.Bullets) > 4 ||
			(slide.Content.Body != "" && len(slide.Content.Bullets) > 0)
		needsFullArea := slide.Type == types.SlideTypeChart ||
			slide.Type == types.SlideTypeDiagram ||
			slide.Type == types.SlideTypeImage ||
			slide.Type == types.SlideTypeTwoColumn ||
			slide.Type == types.SlideTypeComparison ||
			(slide.Type == types.SlideTypeContent && hasDenseContent)
		if !needsFullArea {
			isClosing := hasTag(layout.Tags, "closing") || hasTag(layout.Tags, "thank-you")
			isTitleSlide := hasTag(layout.Tags, "title-slide")
			if slide.Type == types.SlideTypeTitle {
				// Title-type slides need ctrTitle + subTitle placeholders. Only
				// give the closing bonus to layouts that ALSO have "title-slide"
				// tag; closing/section layouts without it lack ctrTitle and the
				// body text overwrites the title in a single placeholder.
				if isClosing && isTitleSlide {
					return scorePerfectMatch
				}
				if isTitleSlide && !isClosing {
					return scoreGoodMatch // demote plain title-slide so closing+title-slide wins
				}
			} else {
				if isClosing {
					return scorePerfectMatch
				}
				// Demote generic title-slide layouts on the last slide when we could have closing/thank-you
				if isTitleSlide && !isClosing {
					return scoreGoodMatch // 0.9 instead of 1.0, giving closing layouts priority
				}
			}
		}
	}

	return scoreSlideTypeMatch(layout, slide)
}

// scoreSlideTypeMatch scores a layout based on slide type compatibility.
func scoreSlideTypeMatch(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	switch slide.Type {
	case types.SlideTypeTitle:
		return scoreTitleSlide(layout)
	case types.SlideTypeChart, types.SlideTypeDiagram:
		return scoreChartSlide(layout, slide)
	case types.SlideTypeImage:
		return scoreImageSlide(layout)
	case types.SlideTypeComparison:
		return scoreComparisonSlide(layout)
	case types.SlideTypeTwoColumn:
		return scoreTwoColumnSlide(layout)
	case types.SlideTypeContent:
		return scoreContentSlide(layout, slide)
	case types.SlideTypeBlank:
		return scoreBlankSlide(layout)
	case types.SlideTypeSection:
		return scoreSectionSlide(layout)
	default:
		return scorePartialMatch
	}
}

func scoreTitleSlide(layout types.LayoutMetadata) float64 {
	if hasTag(layout.Tags, "title-slide") {
		return scorePerfectMatch
	}
	return scoreNoMatch
}

func scoreChartSlide(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	if layout.Capacity.HasChartSlot {
		return scorePerfectMatch
	}
	// Closing and section-header layouts have body placeholders designed for
	// short text (thin strips), not full-area chart/diagram content. Demote them
	// so content layouts with full-area body placeholders are preferred.
	if hasTag(layout.Tags, "closing") || hasTag(layout.Tags, "section-header") {
		return scorePoorMatch
	}
	// Charts fall back to body placeholders (rendered as embedded images).
	// Even when the slide has body text, prefer single-body-placeholder layouts
	// so the chart renders full-width. The body paragraph is supplementary
	// (intro/caption) — splitting into two columns wastes space.
	// Use <!-- type: two-column --> explicitly for side-by-side text+chart.
	bodyPH := findPlaceholder(layout, types.PlaceholderBody)
	if bodyPH != nil {
		return scoreAcceptable
	}
	return scoreNoMatch // Charts require chart or body capability
}

func scoreImageSlide(layout types.LayoutMetadata) float64 {
	if layout.Capacity.HasImageSlot {
		return scorePerfectMatch
	}
	if hasTag(layout.Tags, "full-image") {
		return scoreGoodMatch
	}
	return scoreMinimalMatch
}

func scoreComparisonSlide(layout types.LayoutMetadata) float64 {
	if hasTag(layout.Tags, "comparison") || hasTag(layout.Tags, "two-column") {
		return scorePerfectMatch
	}
	return scorePoorMatch
}

func scoreTwoColumnSlide(layout types.LayoutMetadata) float64 {
	if hasTag(layout.Tags, "two-column") {
		// Penalize unbalanced two-column layouts where one column is much wider
		// than the other (e.g., 1/3 + 2/3 split). Content intended for equal
		// columns renders poorly when proportions are severely skewed.
		penalty := penalizeUnbalancedColumns(layout)
		score := scorePerfectMatch - penalty
		if score < scoreAcceptable {
			score = scoreAcceptable
		}
		return score
	}
	// Content layout with a body placeholder can serve as single-column fallback
	// (left/right content will be merged). This is better than section/title layouts.
	if hasTag(layout.Tags, "content") {
		return scoreAcceptable
	}
	return scorePartialMatch
}

// penalizeUnbalancedColumns returns a penalty (0.0–0.2) when a two-column layout
// has body placeholders with severely different widths. Layouts where the narrower
// column is less than 60% of the wider column's width receive a proportional penalty.
// A perfectly balanced (50/50) layout returns 0.0; a 1/3+2/3 layout returns ~0.13.
func penalizeUnbalancedColumns(layout types.LayoutMetadata) float64 {
	bodyPHs := findBodyPlaceholders(layout)
	if len(bodyPHs) < 2 {
		return 0.0
	}
	w1 := bodyPHs[0].Bounds.Width
	w2 := bodyPHs[1].Bounds.Width
	if w1 <= 0 || w2 <= 0 {
		return 0.0 // No bounds data; can't judge balance
	}
	wider := w1
	narrower := w2
	if w2 > w1 {
		wider = w2
		narrower = w1
	}
	ratio := float64(narrower) / float64(wider) // 0.0–1.0
	// Balanced threshold: narrower >= 60% of wider → no penalty.
	// Below 60%: linear penalty up to 0.2 at 0% ratio.
	const balanceThreshold = 0.60
	const maxPenalty = 0.20
	if ratio >= balanceThreshold {
		return 0.0
	}
	// Linear interpolation: 0.60 → 0.0 penalty, 0.0 → 0.20 penalty
	return maxPenalty * (1.0 - ratio/balanceThreshold)
}

func scoreContentSlide(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	hasTable := slide.Content.TableRaw != ""
	// Dense content (tables, bullet groups, long bullet lists) needs a
	// full-size body placeholder. Constrained layouts (closing, section-header)
	// cause text truncation with "..." when dense content doesn't fit.
	isDense := hasTable ||
		len(slide.Content.BulletGroups) > 0 ||
		len(slide.Content.Bullets) > 4 ||
		(slide.Content.Body != "" && len(slide.Content.Bullets) > 0)

	if hasTag(layout.Tags, "content") {
		// Section dividers tagged "content" can host content but have limited
		// text area designed for short section titles. Demote them so true
		// content layouts (One Content, Two Content) are preferred for dense
		// content like bullet_groups and long bullet lists.
		if hasTag(layout.Tags, "section-header") {
			if isDense {
				return scorePoorMatch
			}
			return scoreGoodMatch
		}
		// Closing/thank-you layouts have minimal body areas designed for
		// "Questions?" or "Thank you" text, not dense content. Without
		// this penalty, variety scoring can push content-heavy slides into
		// closing layouts that lack adequate space, causing text truncation.
		if isDense && (hasTag(layout.Tags, "closing") || hasTag(layout.Tags, "thank-you")) {
			return scorePoorMatch
		}
		// Two-column layouts tagged "content" should only be preferred when
		// the slide explicitly uses slot markers (::slot1::, ::slot2::).
		// Without slots, single-body-placeholder layouts are better because
		// they give content (tables, text, diagrams) the full slide width.
		// Without this penalty, variety scoring can push non-slot content
		// into two-column layouts, squeezing tables to half-width.
		if hasTag(layout.Tags, "two-column") && !slide.HasSlots() {
			return scorePartialMatch
		}
		return scorePerfectMatch
	}
	// Semantic layouts can also work for content but with lower priority.
	// Layouts without the "content" tag (e.g., standalone section-header)
	// have narrow or oddly-positioned placeholders unsuitable for dense content.
	semanticTags := []string{"quote", "statement", "big-number", "section-header", "agenda", "timeline-capable", "icon-grid"}
	for _, tag := range semanticTags {
		if hasTag(layout.Tags, tag) {
			if isDense {
				return scoreMinimalMatch
			}
			return scoreAcceptable // Usable but not preferred for generic content
		}
	}
	// Closing/thank-you layouts without the "content" tag have minimal body
	// areas unsuitable for dense content.
	if isDense && (hasTag(layout.Tags, "closing") || hasTag(layout.Tags, "thank-you")) {
		return scoreMinimalMatch
	}
	// Any non-special layout can work for content
	if !hasTag(layout.Tags, "title-slide") && !hasTag(layout.Tags, "blank") {
		return scoreAcceptable
	}
	return scoreNoMatch
}

func scoreBlankSlide(layout types.LayoutMetadata) float64 {
	if hasTag(layout.Tags, "blank") {
		return scorePerfectMatch
	}
	return scorePartialMatch
}

func scoreSectionSlide(layout types.LayoutMetadata) float64 {
	if hasTag(layout.Tags, "section-header") {
		return scorePerfectMatch
	}
	// Title-slide layouts work as fallback for section dividers, but with a
	// substantial penalty so that section-header layouts are strongly preferred.
	// The old scoreGoodMatch (0.9) created only a 0.035 weighted gap that was
	// easily overcome by capacity/balance/variety scoring, causing section
	// slides to incorrectly select title-slide layouts.
	if hasTag(layout.Tags, "title-slide") {
		return scorePartialMatch
	}
	return scoreNoMatch
}

// scoreCapacity checks if content fits in the layout.
func scoreCapacity(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	score := 1.0

	// Slot-aware scoring: when a slide uses ::slot1::/::slot2:: markers,
	// it needs at least as many content placeholders as slots.
	// Without this, single-placeholder layouts can win and silently drop content.
	if slide.HasSlots() {
		slotCount := len(slide.Slots)
		phCount := countContentPlaceholders(layout)
		if phCount < slotCount {
			// Heavy penalty: each missing placeholder reduces score significantly.
			// A 2-slot slide on a 1-placeholder layout gets 0.3 (barely viable).
			// A 3-slot slide on a 1-placeholder layout gets 0.0.
			missing := float64(slotCount - phCount)
			score -= missing * 0.7
			if score < 0.0 {
				score = 0.0
			}
			return score
		}
		// Slot check passed: enough placeholders exist. Skip bullet overflow
		// scoring below because slide.Content.Bullets is populated from slot
		// content (AST lower leaks bullets from slots into the main Content
		// struct for backward compatibility). Counting these bullets against
		// the layout's MaxBullets capacity incorrectly penalizes two-column
		// layouts with small per-column bullet limits (e.g., maxBullets=3),
		// causing single-column layouts to outscore them and silently drop
		// the second slot's content (charts, diagrams, etc.).
		return score
	}

	bulletCount := len(slide.Content.Bullets)
	if len(slide.Content.Left) > 0 || len(slide.Content.Right) > 0 {
		bulletCount = max(len(slide.Content.Left), len(slide.Content.Right))
	}

	// If no bullets, capacity is perfect
	if bulletCount == 0 {
		return score
	}

	maxBullets := layout.Capacity.MaxBullets
	if maxBullets == 0 {
		maxBullets = 6 // Default assumption
	}

	// Perfect fit
	if bulletCount <= maxBullets {
		return score
	}

	// Penalize overflow
	overflow := float64(bulletCount - maxBullets)
	penalty := overflow / float64(maxBullets)
	score = math.Max(0.0, score-penalty)

	return score
}

// countContentPlaceholders counts placeholders that can serve as slot targets.
// This mirrors generator.FilterContentPlaceholders logic to avoid cross-package dependency.
func countContentPlaceholders(layout types.LayoutMetadata) int {
	count := 0
	for _, ph := range layout.Placeholders {
		switch ph.Type {
		case types.PlaceholderBody, types.PlaceholderContent,
			types.PlaceholderImage, types.PlaceholderChart, types.PlaceholderTable:
			count++
		}
	}
	return count
}

// scoreVisualBalance prefers layouts matching content density.
func scoreVisualBalance(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	// Count total content elements
	contentCount := len(slide.Content.Bullets)
	if len(slide.Content.Left) > 0 {
		contentCount += len(slide.Content.Left)
	}
	if len(slide.Content.Right) > 0 {
		contentCount += len(slide.Content.Right)
	}
	if slide.Content.Body != "" {
		contentCount += 1
	}
	if slide.Content.ImagePath != "" {
		contentCount += 1
	}
	if slide.Content.DiagramSpec != nil {
		contentCount += 1
	}

	// Categorize density
	isDense := contentCount > 5
	isSparse := contentCount <= 2

	// Match layout characteristics
	if isDense && layout.Capacity.TextHeavy {
		return 1.0
	}
	if isSparse && layout.Capacity.VisualFocused {
		return 1.0
	}
	if !isDense && !isSparse {
		return 0.8 // Medium density is flexible
	}

	return 0.5
}

// scoreConsistency penalizes repeating the same layout.
func scoreConsistency(layout types.LayoutMetadata, ctx SelectionContext) float64 {
	if ctx.PreviousType == "" {
		return 1.0
	}

	// Penalty for using same layout consecutively
	if layout.ID == ctx.PreviousType {
		return 0.5
	}

	return 1.0
}

// scoreVariety rewards layouts that have been used less frequently.
// Returns 1.0 for unused layouts, scaling down based on usage count.
func scoreVariety(layout types.LayoutMetadata, ctx SelectionContext) float64 {
	// If no usage tracking, return neutral score
	if len(ctx.UsedLayouts) == 0 {
		return 1.0
	}

	// Get usage count for this layout
	usageCount := ctx.UsedLayouts[layout.ID]

	// Calculate max usage to normalize the score
	maxUsage := 0
	for _, count := range ctx.UsedLayouts {
		if count > maxUsage {
			maxUsage = count
		}
	}

	// Unused layout gets perfect variety score
	if usageCount == 0 {
		return 1.0
	}

	// Score decreases based on relative usage
	// If maxUsage is 3 and this layout has been used 3 times, score = 0.4
	// If maxUsage is 3 and this layout has been used 1 time, score = 0.8
	if maxUsage > 0 {
		relativeUsage := float64(usageCount) / float64(maxUsage)
		return 1.0 - (relativeUsage * 0.6) // Range: 0.4 to 1.0
	}

	return 1.0
}

// calculateConfidence computes confidence based on score gap.
func calculateConfidence(topScore float64, scored []scoredLayout, secondIdx int) float64 {
	if secondIdx == -1 {
		// Only one layout
		return topScore
	}

	secondScore := scored[secondIdx].score
	gap := topScore - secondScore
	confidence := gap + (topScore * 0.5)

	// Clamp to [0, 1]
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// isLayoutSuitable checks if layout can handle required content types.
func isLayoutSuitable(layout types.LayoutMetadata, slide types.SlideDefinition) bool { //nolint:gocyclo
	// Reject layouts with non-standard title positioning when slide has a title.
	// These layouts have title placeholders positioned either:
	// - Off-screen (negative Y) - "title-hidden" tag
	// - At the bottom of the slide (Y > 50% of slide height) - "title-at-bottom" tag
	// Such layouts are designed for special purposes (quotes, statements) and cannot
	// properly display standard title-above-content slides.
	//
	// Exception: Section divider slides (SlideTypeSection) are exempt from this
	// rejection because section-header layouts commonly use title-at-bottom or
	// title-hidden positioning by design (e.g., some templates' Section Header layout has
	// the title in the lower half of the slide as an intentional design choice).
	if slide.Title != "" && slide.Type != types.SlideTypeSection {
		if hasTag(layout.Tags, "title-hidden") || hasTag(layout.Tags, "title-at-bottom") {
			return false
		}
	}

	switch slide.Type {
	case types.SlideTypeChart, types.SlideTypeDiagram:
		// Charts/diagrams need full-area placeholders. Section-header, closing,
		// and two-column layouts have narrow or off-center body placeholders that
		// squeeze charts into a fraction of the slide. Reject them outright so
		// variety scoring cannot override the preference for full-width content
		// layouts. Two-column layouts are designed for side-by-side content and
		// their body placeholders are 40-60% of slide width, producing
		// thumbnail-sized charts when a full-width chart is placed in one column.
		if hasTag(layout.Tags, "section-header") || hasTag(layout.Tags, "closing") || hasTag(layout.Tags, "two-column") {
			return false
		}
		// Charts/diagrams can use chart slots or body/content placeholders (rendered as embedded images)
		if layout.Capacity.HasChartSlot {
			return true
		}
		// Fall back to body placeholder for chart/diagram embedding.
		// Reject layouts whose body placeholder is too small for visual content.
		// Check both height (thin text strips < 2 inches) and width (narrow
		// columns < 55% of slide width). Without these guards, a layout with
		// a tiny body placeholder can outscore full-size content layouts and
		// render charts at thumbnail scale.
		//
		// Also reject placeholders with zero bounds (Width==0 or Height==0).
		// Zero bounds indicate unresolved master inheritance — the placeholder's
		// true size is unknown. Allowing such placeholders produces thumbnail
		// charts rendered at (0,0) with no visible area, which was the root
		// cause of SVG thumbnailing bugs on non-standard templates.
		bodyPH := findPlaceholder(layout, types.PlaceholderBody)
		if bodyPH == nil {
			return false
		}
		if bodyPH.Bounds.Height == 0 || bodyPH.Bounds.Width == 0 {
			return false
		}
		if bodyPH.Bounds.Height < minChartPlaceholderHeight {
			return false
		}
		if bodyPH.Bounds.Width < minChartPlaceholderWidth {
			return false
		}
		return true
	case types.SlideTypeImage:
		// Images can use image slots or any layout with space
		return layout.Capacity.HasImageSlot || len(layout.Placeholders) > 0
	case types.SlideTypeTwoColumn:
		// Two-column slides with explicit ::slot1::/::slot2:: markers need layouts
		// with enough content placeholders to host each slot's content. Accepting
		// a 1-placeholder layout would silently merge or drop slot content.
		if slide.HasSlots() {
			if countContentPlaceholders(layout) < len(slide.Slots) {
				return false
			}
			// Apply minimum-height guard for chart/diagram slots (same logic as
			// SlideTypeChart suitability check). Two-column layouts can have body
			// placeholders with tiny inherited heights that produce thumbnail charts.
			contentPHs := contentPlaceholdersFromLayout(layout)
			for _, slot := range slide.Slots {
				if slot == nil || slot.DiagramSpec == nil {
					continue
				}
				phIdx := slot.SlotNumber - 1
				if phIdx >= 0 && phIdx < len(contentPHs) {
					ph := contentPHs[phIdx]
					if ph.Bounds.Height > 0 && ph.Bounds.Height < minChartPlaceholderHeight {
						return false
					}
				}
			}
			return true
		}
		// Legacy two-column slides (Left/Right fields without slot markers):
		// accept 1-placeholder layouts as a fallback — buildMappings will
		// merge left/right content into a single column.
		bodyPHs := findBodyPlaceholders(layout)
		return len(bodyPHs) >= 1
	case types.SlideTypeContent:
		// Tables need full-width body placeholders. Only layouts tagged
		// "content" have proper body areas; section-header and closing
		// layouts have narrow or off-center placeholders that misalign tables.
		if slide.Content.TableRaw != "" {
			return hasTag(layout.Tags, "content")
		}
		// Content slides with body text or bullets need a layout with usable
		// body/content placeholder. Layouts tagged only as title-slide/closing
		// but NOT "content" lack body placeholders and cannot display bullet content.
		hasBodyContent := len(slide.Content.Bullets) > 0 || len(slide.Content.BulletGroups) > 0 || slide.Content.Body != ""
		if hasBodyContent {
			// Bullet groups require an actual body/content placeholder for
			// buildMappings to create a bullets mapping. Without one, the
			// bullet_groups content is silently skipped, producing a blank slide.
			// Verify the placeholder exists even for "content"-tagged layouts to
			// guard against templates where the body placeholder was removed or
			// reclassified (e.g., as image).
			if len(slide.Content.BulletGroups) > 0 {
				bodyPH := findPlaceholder(layout, types.PlaceholderBody)
				return bodyPH != nil
			}
			// Need a layout with content capability
			if hasTag(layout.Tags, "content") {
				return true
			}
			// Also accept layouts with adequate bullet capacity
			return layout.Capacity.MaxBullets > 0
		}
		// Content slide with media (chart/diagram) but no body text — needs a
		// layout with an adequate body/content placeholder. Title-slide,
		// section-header, and closing layouts have tiny subtitle placeholders
		// that squeeze media to unusable thumbnail size.
		if slide.Content.DiagramSpec != nil {
			if hasTag(layout.Tags, "title-slide") || hasTag(layout.Tags, "section-header") || hasTag(layout.Tags, "closing") {
				return false
			}
			// Need a body placeholder to embed the media
			bodyPH := findPlaceholder(layout, types.PlaceholderBody)
			return bodyPH != nil
		}
		// Content slide without body content or media can use any layout
		return true
	default:
		return true
	}
}

// requiredCapability returns a description of what's needed for a slide.
func requiredCapability(slide types.SlideDefinition) string {
	switch slide.Type {
	case types.SlideTypeChart, types.SlideTypeDiagram:
		return "chart or body placeholder"
	case types.SlideTypeImage:
		return "image placeholder"
	case types.SlideTypeTwoColumn:
		return "two body/content placeholders"
	default:
		return "text placeholders"
	}
}

// mappingBuilder accumulates content mappings and warnings.
type mappingBuilder struct {
	mappings []ContentMapping
	warnings []string
}

// addTextMapping maps a text content field to a placeholder with truncation check.
// If placeholder is nil and warnIfMissing is true, a warning is added.
func (mb *mappingBuilder) addTextMapping(field, content string, ph *types.PlaceholderInfo, warnIfMissing bool, missingMsg string) {
	if content == "" {
		return
	}
	if ph == nil {
		if warnIfMissing {
			mb.warnings = append(mb.warnings, missingMsg)
		}
		return
	}
	truncated, truncateAt := checkTruncation(content, ph.MaxChars)
	mb.mappings = append(mb.mappings, ContentMapping{
		ContentField:  field,
		PlaceholderID: ph.ID,
		Truncated:     truncated,
		TruncateAt:    truncateAt,
	})
}


// addWarning adds a warning message.
func (mb *mappingBuilder) addWarning(msg string) {
	mb.warnings = append(mb.warnings, msg)
}

// addBulletMapping adds a bullet (or bullet groups) mapping with truncation check.
// If maxBullets > 0 and bulletCount exceeds it, a warning is added.
func (mb *mappingBuilder) addBulletMapping(field string, totalLen int, ph *types.PlaceholderInfo, bulletCount, maxBullets int) {
	if ph == nil {
		return
	}
	truncated, truncateAt := checkTruncation(strings.Repeat("x", totalLen), ph.MaxChars)
	mb.mappings = append(mb.mappings, ContentMapping{
		ContentField:  field,
		PlaceholderID: ph.ID,
		Truncated:     truncated,
		TruncateAt:    truncateAt,
	})
	if maxBullets > 0 && bulletCount > maxBullets {
		mb.addWarning(fmt.Sprintf("Slide has %d bullets but layout supports %d", bulletCount, maxBullets))
	}
}

// buildMappings creates content-to-placeholder mappings.
func buildMappings(layout types.LayoutMetadata, slide types.SlideDefinition) ([]ContentMapping, []string) { //nolint:gocyclo
	mb := &mappingBuilder{
		mappings: []ContentMapping{},
		warnings: []string{},
	}

	// Find placeholders by type
	titlePH := findPlaceholder(layout, types.PlaceholderTitle)
	imagePH := findPlaceholder(layout, types.PlaceholderImage)
	chartPH := findPlaceholder(layout, types.PlaceholderChart)

	// Map title — fall back to body/subtitle placeholder for section divider layouts
	// that have no title placeholder (e.g. template_2's "Section Devider" layout
	// uses a body placeholder at idx 13 for the section title text).
	if titlePH == nil && slide.Title != "" {
		bodyPH := findPlaceholder(layout, types.PlaceholderBody)
		if bodyPH == nil {
			bodyPH = findPlaceholder(layout, types.PlaceholderSubtitle)
		}
		if bodyPH == nil {
			bodyPH = findPlaceholder(layout, types.PlaceholderContent)
		}
		if bodyPH != nil {
			mb.addTextMapping("title", slide.Title, bodyPH, false, "")
		} else {
			mb.addWarning("No title placeholder found")
		}
	} else {
		mb.addTextMapping("title", slide.Title, titlePH, true, "No title placeholder found")
	}

	// Check if this is a two-column slide with left/right content
	hasTwoColumnContent := len(slide.Content.Left) > 0 || len(slide.Content.Right) > 0

	if hasTwoColumnContent {
		// Two-column content: find multiple body placeholders
		bodyPlaceholders := findBodyPlaceholders(layout)
		if len(bodyPlaceholders) >= 2 {
			// Map left content to first body placeholder (typically positioned on left)
			// Use unique placeholder identifier (index preferred over name to handle duplicates)
			if len(slide.Content.Left) > 0 {
				mb.mappings = append(mb.mappings, ContentMapping{
					ContentField:  "left",
					PlaceholderID: getUniquePlaceholderID(bodyPlaceholders[0]),
					Truncated:     false,
				})
			}
			// Map right content to second body placeholder (typically positioned on right)
			if len(slide.Content.Right) > 0 {
				mb.mappings = append(mb.mappings, ContentMapping{
					ContentField:  "right",
					PlaceholderID: getUniquePlaceholderID(bodyPlaceholders[1]),
					Truncated:     false,
				})
			}
		} else if len(bodyPlaceholders) == 1 {
			// Only one body placeholder - combine left and right content
			mb.addWarning("Two-column content but only one body placeholder found; content will be combined")
			combined := append(slide.Content.Left, slide.Content.Right...)
			if len(combined) > 0 {
				mb.mappings = append(mb.mappings, ContentMapping{
					ContentField:  "bullets",
					PlaceholderID: bodyPlaceholders[0].ID,
					Truncated:     false,
				})
			}
		} else {
			mb.addWarning("No body placeholders found for two-column content")
		}
	} else {
		// Standard content: use single body placeholder
		bodyPH := findPlaceholder(layout, types.PlaceholderBody)

		// For title-only layouts (like "Title Slide"), fall back to subtitle placeholder
		// when body placeholder is not available. This allows content like "Questions and
		// Discussion" on Thank You slides to render in the subtitle position.
		//
		// However, skip subtitle placeholders with very small fonts (< 14pt / 1400 hPt).
		// Some templates (e.g. closing layouts) use the subtitle as a legal
		// disclaimer area (11pt, bottom-right corner). Routing body content there
		// renders it in the wrong position with illegible font size.
		// In that case, fall back to the title placeholder so BuildContentItems can
		// combine title and body text in a single placeholder.
		if bodyPH == nil {
			subtitlePH := findPlaceholder(layout, types.PlaceholderSubtitle)
			if subtitlePH != nil && (subtitlePH.FontSize == 0 || subtitlePH.FontSize >= 1400) {
				// FontSize=0 means unresolved — assume usable; >= 14pt is adequate.
				bodyPH = subtitlePH
			} else if subtitlePH != nil && slide.Content.Body != "" && titlePH != nil {
				// Subtitle too small for body content — route body to title placeholder.
				// BuildContentItems merges title + body when they share a placeholder.
				bodyPH = titlePH
			}
		}

		hasBody := slide.Content.Body != ""
		hasTable := slide.Content.TableRaw != ""
		hasBullets := len(slide.Content.Bullets) > 0
		hasBulletGroups := len(slide.Content.BulletGroups) > 0

		// Standalone table: create a body mapping so BuildContentItems can
		// route it to ContentTable. The table replaces body content.
		if hasTable && !hasBody && bodyPH != nil {
			mb.mappings = append(mb.mappings, ContentMapping{
				ContentField:  "body",
				PlaceholderID: bodyPH.ID,
			})
		}

		// Prefer BulletGroups over flat bullets (hierarchical structure with section headers)
		if hasBulletGroups && bodyPH == nil {
			mb.addWarning("No body placeholder found for bullet_groups content; slide will render blank")
		}
		if hasBulletGroups && bodyPH != nil {
			// If there's also body text, add a body mapping first so that
			// BuildContentItems can combine it with the bullet groups
			if hasBody {
				mb.addTextMapping("body", slide.Content.Body, bodyPH, false, "")
			}
			totalLen := calculateBulletGroupsLength(slide.Content.BulletGroups)
			if hasBody {
				totalLen += len(slide.Content.Body)
			}
			mb.addBulletMapping("bullets", totalLen, bodyPH, len(slide.Content.Bullets), layout.Capacity.MaxBullets)
		} else if hasBody && hasBullets && bodyPH != nil {
			// When both body and bullets are present, combine them into a single mapping
			totalLen := len(slide.Content.Body) + calculateBulletLength(slide.Content.Bullets) + len(slide.Content.BodyAfterBullets)
			mb.addBulletMapping("body_and_bullets", totalLen, bodyPH, len(slide.Content.Bullets), layout.Capacity.MaxBullets)
		} else {
			// Map body text only if no bullets
			mb.addTextMapping("body", slide.Content.Body, bodyPH, false, "")
			// Map bullets only if no body
			if hasBullets && bodyPH != nil {
				totalLen := calculateBulletLength(slide.Content.Bullets)
				mb.addBulletMapping("bullets", totalLen, bodyPH, len(slide.Content.Bullets), layout.Capacity.MaxBullets)
			}
		}
	}

	// Map image
	// First try dedicated image placeholder, then fall back to body/content placeholder.
	// This prevents blank slides when a layout has no image placeholder but the slide has image content.
	if slide.Content.ImagePath != "" {
		if imagePH != nil {
			mb.mappings = append(mb.mappings, ContentMapping{
				ContentField:  "image",
				PlaceholderID: imagePH.ID,
				Truncated:     false,
			})
		} else {
			// Fall back to body placeholder for image rendering
			bodyPHs := findBodyPlaceholders(layout)
			if len(bodyPHs) >= 1 {
				mb.mappings = append(mb.mappings, ContentMapping{
					ContentField:  "image",
					PlaceholderID: getUniquePlaceholderID(bodyPHs[0]),
					Truncated:     false,
				})
				mb.addWarning("No image placeholder found; image placed in body placeholder")
			} else {
				mb.addWarning("No image placeholder found and no body placeholder available")
			}
		}
	}

	// Map chart or diagram
	// First try dedicated chart placeholder, then fall back to body/content placeholder.
	// For two-column slides with charts, route to the unused body placeholder.
	hasChart := slide.Content.DiagramSpec != nil
	if hasChart {
		if chartPH != nil {
			mb.mappings = append(mb.mappings, ContentMapping{
				ContentField:  "chart",
				PlaceholderID: chartPH.ID,
				Truncated:     false,
			})
		} else {
			// Fall back to body/content placeholder for chart embedding as image.
			// For two-column slides, find the unused body placeholder.
			hasBodyText := slide.Content.Body != ""
			bodyPHs := findBodyPlaceholders(layout)

			if hasTwoColumnContent && len(bodyPHs) >= 2 {
				// Two-column slide with chart: find the body placeholder NOT used
				// by left/right text content. Left maps to bodyPHs[0], Right to
				// bodyPHs[1]. Put the chart in whichever is unused.
				leftUsed := len(slide.Content.Left) > 0
				rightUsed := len(slide.Content.Right) > 0
				if leftUsed && !rightUsed {
					// Left text in bodyPHs[0], chart goes in bodyPHs[1]
					mb.mappings = append(mb.mappings, ContentMapping{
						ContentField:  "diagram",
						PlaceholderID: getUniquePlaceholderID(bodyPHs[1]),
						Truncated:     false,
					})
				} else if !leftUsed && rightUsed {
					// Right text in bodyPHs[1], chart goes in bodyPHs[0]
					mb.mappings = append(mb.mappings, ContentMapping{
						ContentField:  "diagram",
						PlaceholderID: getUniquePlaceholderID(bodyPHs[0]),
						Truncated:     false,
					})
				} else {
					// Both sides have text — chart must share a placeholder
					mb.mappings = append(mb.mappings, ContentMapping{
						ContentField:  "diagram",
						PlaceholderID: getUniquePlaceholderID(bodyPHs[0]),
						Truncated:     false,
					})
					mb.addWarning("No chart placeholder found; chart shares placeholder with two-column text")
				}
			} else if len(bodyPHs) >= 1 {
				mb.mappings = append(mb.mappings, ContentMapping{
					ContentField:  "diagram",
					PlaceholderID: getUniquePlaceholderID(bodyPHs[0]),
					Truncated:     false,
				})
				if hasBodyText {
					mb.addWarning("No chart placeholder found; chart overwrites body text in body placeholder")
				} else {
					mb.addWarning("No chart placeholder found; using body placeholder for chart/diagram")
				}
			} else {
				mb.addWarning("No chart or body placeholder found for chart content")
			}
		}
	}

	return mb.mappings, mb.warnings
}

// calculateBulletLength calculates the total character length of all bullets.
func calculateBulletLength(bullets []string) int {
	totalLen := 0
	for _, bullet := range bullets {
		totalLen += len(bullet)
	}
	return totalLen
}

// calculateBulletGroupsLength calculates the total character length of bullet groups.
func calculateBulletGroupsLength(groups []types.BulletGroup) int {
	totalLen := 0
	for _, group := range groups {
		totalLen += len(group.Header)
		for _, bullet := range group.Bullets {
			totalLen += len(bullet)
		}
	}
	return totalLen
}

// findPlaceholder returns the first placeholder of a given type.
// For PlaceholderBody, it also falls back to PlaceholderContent since many templates
// use non-standard placeholder types (e.g., type="unknown") that get classified as content.
func findPlaceholder(layout types.LayoutMetadata, phType types.PlaceholderType) *types.PlaceholderInfo {
	// First, try to find exact match
	for i := range layout.Placeholders {
		if layout.Placeholders[i].Type == phType {
			return &layout.Placeholders[i]
		}
	}

	// Fallback: PlaceholderBody can use PlaceholderContent as alternative
	// This handles templates with non-standard placeholder types (type="unknown", etc.)
	if phType == types.PlaceholderBody {
		for i := range layout.Placeholders {
			if layout.Placeholders[i].Type == types.PlaceholderContent {
				return &layout.Placeholders[i]
			}
		}
	}

	return nil
}

// findBodyPlaceholders returns body/content placeholders suitable for main content, sorted by X position.
// This is used for two-column layouts where multiple body placeholders are needed.
// Returns placeholders sorted left-to-right by their X coordinate.
// Prioritizes PlaceholderBody over PlaceholderContent, and filters out date/footer/slide number placeholders.
func findBodyPlaceholders(layout types.LayoutMetadata) []types.PlaceholderInfo {
	var bodyPHs []types.PlaceholderInfo

	// First pass: collect only PlaceholderBody (primary content areas)
	for _, ph := range layout.Placeholders {
		if ph.Type == types.PlaceholderBody {
			bodyPHs = append(bodyPHs, ph)
		}
	}

	// If we found at least 2 body placeholders, use them
	if len(bodyPHs) >= 2 {
		sortPlaceholdersByX(bodyPHs)
		return bodyPHs
	}

	// Fallback: include PlaceholderContent, but filter out common non-content placeholders
	// by checking the ID contains keywords like "date", "footer", "slide number"
	for _, ph := range layout.Placeholders {
		if ph.Type == types.PlaceholderContent {
			// Skip date, footer, and slide number placeholders
			lowerID := strings.ToLower(ph.ID)
			if strings.Contains(lowerID, "date") ||
				strings.Contains(lowerID, "footer") ||
				strings.Contains(lowerID, "slide number") ||
				strings.Contains(lowerID, "slidenumber") {
				continue
			}
			bodyPHs = append(bodyPHs, ph)
		}
	}

	sortPlaceholdersByX(bodyPHs)
	return bodyPHs
}

// sortPlaceholdersByX sorts placeholders by X position (left to right).
func sortPlaceholdersByX(phs []types.PlaceholderInfo) {
	slices.SortStableFunc(phs, func(a, b types.PlaceholderInfo) int {
		return cmp.Compare(a.Bounds.X, b.Bounds.X)
	})
}

// sortScoredLayouts sorts scored layouts by score descending (highest first).
// Uses stable sort to preserve original order among equal scores for determinism.
func sortScoredLayouts(layouts []scoredLayout) {
	slices.SortStableFunc(layouts, func(a, b scoredLayout) int {
		return cmp.Compare(b.score, a.score) // descending
	})
}

// getUniquePlaceholderID returns the placeholder's canonical ID.
// After normalization, placeholder IDs are unique within a layout
// (e.g., "body", "body_2"), so the name is always sufficient.
func getUniquePlaceholderID(ph types.PlaceholderInfo) string {
	return ph.ID
}

// checkTruncation determines if content will be truncated.
func checkTruncation(content string, maxChars int) (bool, int) {
	if maxChars == 0 {
		return false, 0
	}

	if len(content) > maxChars {
		return true, maxChars
	}

	return false, 0
}

// hasTag checks if a layout has a specific tag.
func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// narrowDiagramThreshold is the minimum placeholder width (in EMUs) for complex
// diagrams to remain legible. Matches generator.narrowPlaceholderThreshold.
// Approximately 50% of a standard 16:9 slide width (12,192,000 EMU).
// This threshold allows 60/40 splits (60% column ≈ 6.4M EMU) to pass while
// penalizing 50/50 and narrower splits for complex diagrams.
const narrowDiagramThreshold int64 = 6096000

// minChartPlaceholderHeight is the minimum placeholder height (in EMUs) for a
// layout to be considered suitable for chart/diagram content. Layouts with body
// placeholders shorter than this are thin text strips unsuitable for visual
// content. 1828800 EMU ≈ 2 inches (20% of a standard 16:9 slide height).
const minChartPlaceholderHeight int64 = 1828800

// minChartPlaceholderWidth is the minimum placeholder width (in EMUs) for a
// layout to be considered suitable for chart/diagram content. Layouts with body
// placeholders narrower than this produce compressed charts with illegible
// labels. 6705600 EMU ≈ 55% of standard 16:9 slide width (12192000 EMU),
// which rejects two-column and other narrow-body layouts.
const minChartPlaceholderWidth int64 = 6705600

// needsFullWidthDiagramTypes lists diagram types that become illegible when
// compressed into narrow (<55% slide width) placeholders. These have dense
// text labels, grid layouts, or hierarchical structures requiring full width.
// Matches and extends generator.complexDiagramTypes.
var needsFullWidthDiagramTypes = map[string]bool{
	// Grid/matrix layouts — many labeled cells
	"business_model_canvas": true,
	"nine_box_talent":       true,
	"pestel":                true,
	"swot":                  true,
	"heatmap":               true,
	"matrix_2x2":            true,
	// Hierarchical/flow layouts — labels truncate in narrow columns
	"org_chart":             true,
	"fishbone":              true,
	"process_flow":          true,
	"value_chain":           true,
	"porters_five_forces":   true,
	// Timeline/Gantt — horizontal space critical for date labels
	"timeline":              true,
	"gantt":                 true,
	// Multi-element visual layouts
	"kpi_dashboard":         true,
	"venn":                  true,
	// Funnel — bottom segments too narrow for inside labels
	"funnel":                true,
	// Panel layout — multiple side-by-side panels with icons, titles, and body text
	"panel_layout":          true,
}

// penalizeNarrowDiagramSlot returns a penalty (0.0–0.5) when a chart/diagram
// would be placed in a placeholder narrower than narrowDiagramThreshold.
// This prevents layout selection from choosing layouts that compress charts and
// diagrams into a fraction of the slide when wider alternatives are available.
//
// Penalty levels:
//   - 0.5: Complex diagram types (SWOT, org_chart, heatmap, etc.) in narrow area
//   - 0.3: Any chart/diagram type in narrow area (bar_chart, pie_chart, etc.)
//   - 0.0: Content area is wide enough, or slide has no diagram
func penalizeNarrowDiagramSlot(layout types.LayoutMetadata, slide types.SlideDefinition) float64 {
	// Case 1: Slotted slides — check each slot for diagram content
	if slide.HasSlots() {
		contentPHs := contentPlaceholdersFromLayout(layout)
		for _, slot := range slide.Slots {
			if slot == nil || slot.DiagramSpec == nil {
				continue
			}
			if !needsFullWidthDiagramTypes[slot.DiagramSpec.Type] {
				continue
			}
			// Map slot number to content placeholder (1-indexed → 0-indexed)
			phIdx := slot.SlotNumber - 1
			if phIdx >= 0 && phIdx < len(contentPHs) {
				if contentPHs[phIdx].Bounds.Width > 0 && contentPHs[phIdx].Bounds.Width < narrowDiagramThreshold {
					return 0.5 // Heavy penalty — forces full-width layout to win
				}
			}
		}
	}

	// Case 2: Non-slotted chart/diagram slides
	if slide.Content.DiagramSpec != nil {
		// Determine the content width that would host the diagram.
		// Charts use the chart slot when available (scored as perfect match
		// by scoreChartSlide), otherwise the body placeholder is the fallback.
		contentWidth := int64(0)
		if layout.Capacity.HasChartSlot {
			chartPH := findPlaceholder(layout, types.PlaceholderChart)
			if chartPH != nil && chartPH.Bounds.Width > 0 {
				contentWidth = chartPH.Bounds.Width
			}
		}
		if contentWidth == 0 {
			bodyPH := findPlaceholder(layout, types.PlaceholderBody)
			if bodyPH != nil && bodyPH.Bounds.Width > 0 {
				contentWidth = bodyPH.Bounds.Width
			}
		}

		if contentWidth > 0 && contentWidth < narrowDiagramThreshold {
			if needsFullWidthDiagramTypes[slide.Content.DiagramSpec.Type] {
				return 0.5 // Heavy penalty for complex diagrams
			}
			return 0.3 // Moderate penalty for any chart in narrow area
		}
	}

	return 0.0
}

// contentPlaceholdersFromLayout returns content placeholders sorted by index.
// Mirrors generator.FilterContentPlaceholders to avoid cross-package dependency.
func contentPlaceholdersFromLayout(layout types.LayoutMetadata) []types.PlaceholderInfo {
	var content []types.PlaceholderInfo
	for _, ph := range layout.Placeholders {
		switch ph.Type {
		case types.PlaceholderBody, types.PlaceholderContent,
			types.PlaceholderImage, types.PlaceholderChart, types.PlaceholderTable:
			content = append(content, ph)
		}
	}
	slices.SortFunc(content, func(a, b types.PlaceholderInfo) int {
		return cmp.Compare(a.Index, b.Index)
	})
	return content
}
