package template

import (
	"fmt"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// MinimalTemplateValidation contains the results of minimal template validation.
// A valid minimal template must have all 4 required slide types:
// title, content, section, and closing.
type MinimalTemplateValidation struct {
	Valid           bool     `json:"valid"`
	TitleSlideIdx   int      `json:"title_slide_idx"`   // -1 if missing
	ContentSlideIdx int      `json:"content_slide_idx"` // -1 if missing
	SectionSlideIdx int      `json:"section_slide_idx"` // -1 if missing
	ClosingSlideIdx int      `json:"closing_slide_idx"` // -1 if missing
	Errors          []string `json:"errors"`
	Warnings        []string `json:"warnings"`
}

// SlideTypeDetection holds detection results for a single layout.
type SlideTypeDetection struct {
	LayoutIndex int
	LayoutName  string
	DetectedAs  string  // "title", "content", "section", "closing", "unknown"
	Confidence  float64 // 0.0-1.0 confidence score
	Reasons     []string
}

// Slide type constants for detection.
const (
	SlideTypeTitle   = "title"
	SlideTypeContent = "content"
	SlideTypeSection = "section"
	SlideTypeClosing = "closing"
	SlideTypeUnknown = "unknown"
)

// Detection thresholds.
const (
	// minConfidenceThreshold is the minimum confidence to accept a detection.
	minConfidenceThreshold = 0.3

	// minBodyCapacity is the minimum MaxChars for a body placeholder to be
	// considered "significant" for content slides.
	minBodyCapacity = 100
)

// DetectSlideType analyzes a layout and determines its type.
// It uses structural analysis (placeholders) and semantic analysis (name).
func DetectSlideType(layout types.LayoutMetadata) SlideTypeDetection {
	detection := SlideTypeDetection{
		LayoutIndex: layout.Index,
		LayoutName:  layout.Name,
		DetectedAs:  SlideTypeUnknown,
		Confidence:  0.0,
		Reasons:     []string{},
	}

	// Calculate scores for each slide type
	scores := map[string]float64{
		SlideTypeTitle:   scoreTitleSlide(layout, &detection),
		SlideTypeContent: scoreContentSlide(layout, &detection),
		SlideTypeSection: scoreSectionSlide(layout, &detection),
		SlideTypeClosing: scoreClosingSlide(layout, &detection),
	}

	// Find the highest scoring type
	var bestType string
	var bestScore float64
	for slideType, score := range scores {
		if score > bestScore {
			bestScore = score
			bestType = slideType
		}
	}

	if bestScore >= minConfidenceThreshold {
		detection.DetectedAs = bestType
		detection.Confidence = bestScore
	}

	return detection
}

// scoreTitleSlide calculates a score for how likely this layout is a title slide.
// Title slides typically have:
// - A visible title placeholder
// - An optional subtitle placeholder
// - No body/content placeholder with significant text capacity
func scoreTitleSlide(layout types.LayoutMetadata, detection *SlideTypeDetection) float64 {
	var score float64
	counts := countLayoutPlaceholders(layout.Placeholders)
	lower := strings.ToLower(layout.Name)

	// Structural: Has title-slide tag (already classified)
	if hasTag(layout.Tags, "title-slide") {
		score += 0.4
		detection.Reasons = append(detection.Reasons, "has title-slide structural tag")
	}

	// Structural: Has visible title and no body
	if counts.visibleTitle > 0 && counts.body == 0 {
		score += 0.3
		detection.Reasons = append(detection.Reasons, "has visible title without body placeholder")
	}

	// Structural: Has subtitle (common for title slides)
	if counts.subtitle > 0 {
		score += 0.1
		detection.Reasons = append(detection.Reasons, "has subtitle placeholder")
	}

	// Semantic: Name contains title-related keywords
	if strings.Contains(lower, "title") &&
		!strings.Contains(lower, "section") &&
		!strings.Contains(lower, "content") {
		score += 0.2
		detection.Reasons = append(detection.Reasons, "name contains 'title'")
	}

	// Position: First layout is often title slide
	if layout.Index == 0 {
		score += 0.1
		detection.Reasons = append(detection.Reasons, "first layout in template")
	}

	// Penalty: Has body placeholder (not typical for title slides)
	if counts.body > 0 {
		score -= 0.3
		detection.Reasons = append(detection.Reasons, "has body placeholder (penalty)")
	}

	return clampScore(score)
}

// scoreContentSlide calculates a score for how likely this layout is a content slide.
// Content slides typically have:
// - A visible title placeholder
// - Exactly 1 body or content placeholder with significant capacity
func scoreContentSlide(layout types.LayoutMetadata, detection *SlideTypeDetection) float64 {
	var score float64
	counts := countLayoutPlaceholders(layout.Placeholders)
	lower := strings.ToLower(layout.Name)

	// Structural: Has content tag
	if hasTag(layout.Tags, "content") {
		score += 0.3
		detection.Reasons = append(detection.Reasons, "has content structural tag")
	}

	// Structural: Has visible title and body
	if counts.visibleTitle > 0 && counts.body > 0 {
		score += 0.3
		detection.Reasons = append(detection.Reasons, "has visible title and body placeholder")
	}

	// Structural: Body has significant capacity
	if hasSignificantBodyCapacity(layout.Placeholders) {
		score += 0.2
		detection.Reasons = append(detection.Reasons, "body has significant text capacity")
	}

	// Structural: Exactly 1 body placeholder (simple content layout)
	if counts.body == 1 {
		score += 0.1
		detection.Reasons = append(detection.Reasons, "has exactly one body placeholder")
	}

	// Semantic: Name contains content-related keywords
	if strings.Contains(lower, "content") ||
		strings.Contains(lower, "text") ||
		strings.Contains(lower, "body") {
		score += 0.1
		detection.Reasons = append(detection.Reasons, "name contains content-related keyword")
	}

	// Penalty: No body placeholder
	if counts.body == 0 {
		score -= 0.5
		detection.Reasons = append(detection.Reasons, "no body placeholder (penalty)")
	}

	// Penalty: Has section/closing keywords in name
	if strings.Contains(lower, "section") ||
		strings.Contains(lower, "closing") ||
		strings.Contains(lower, "end") {
		score -= 0.2
		detection.Reasons = append(detection.Reasons, "name suggests non-content type (penalty)")
	}

	return clampScore(score)
}

// scoreSectionSlide calculates a score for how likely this layout is a section slide.
// Section slides typically have:
// - A title placeholder (often centered or larger)
// - No body placeholder OR body is hidden/minimal
// - Name contains "section", "divider", "break"
func scoreSectionSlide(layout types.LayoutMetadata, detection *SlideTypeDetection) float64 {
	var score float64
	counts := countLayoutPlaceholders(layout.Placeholders)
	lower := strings.ToLower(layout.Name)

	// Semantic: Name strongly suggests section
	if strings.Contains(lower, "section") {
		score += 0.5
		detection.Reasons = append(detection.Reasons, "name contains 'section'")
	}
	if strings.Contains(lower, "divider") ||
		strings.Contains(lower, "break") ||
		strings.Contains(lower, "transition") {
		score += 0.3
		detection.Reasons = append(detection.Reasons, "name contains divider/break keyword")
	}

	// Structural: Has section-header tag
	if hasTag(layout.Tags, "section-header") {
		score += 0.3
		detection.Reasons = append(detection.Reasons, "has section-header structural tag")
	}

	// Structural: Has title but no or minimal body
	if counts.visibleTitle > 0 && counts.body == 0 {
		score += 0.2
		detection.Reasons = append(detection.Reasons, "has title without body")
	}

	// Structural: Title-hidden layouts can be section headers
	if hasTag(layout.Tags, "title-hidden") {
		score += 0.1
		detection.Reasons = append(detection.Reasons, "has title-hidden tag")
	}

	// Penalty: Has significant body content (more like content slide)
	if counts.body > 0 && hasSignificantBodyCapacity(layout.Placeholders) {
		score -= 0.2
		detection.Reasons = append(detection.Reasons, "has significant body capacity (penalty)")
	}

	return clampScore(score)
}

// scoreClosingSlide calculates a score for how likely this layout is a closing slide.
// Closing slides typically have:
// - A title placeholder
// - May have subtitle
// - Minimal or no body content
// - Name contains "end", "closing", "thank", "questions"
func scoreClosingSlide(layout types.LayoutMetadata, detection *SlideTypeDetection) float64 {
	var score float64
	counts := countLayoutPlaceholders(layout.Placeholders)
	lower := strings.ToLower(layout.Name)

	// Structural: Has closing or thank-you tag
	if hasTag(layout.Tags, "closing") {
		score += 0.4
		detection.Reasons = append(detection.Reasons, "has closing structural tag")
	}
	if hasTag(layout.Tags, "thank-you") {
		score += 0.4
		detection.Reasons = append(detection.Reasons, "has thank-you structural tag")
	}

	// Semantic: Name contains closing-related keywords
	if strings.Contains(lower, "closing") ||
		strings.Contains(lower, "close") ||
		strings.Contains(lower, "final") ||
		strings.Contains(lower, "conclusion") {
		score += 0.4
		detection.Reasons = append(detection.Reasons, "name contains closing keyword")
	}

	// Semantic: Name contains thank you / questions keywords
	if strings.Contains(lower, "thank") ||
		strings.Contains(lower, "question") ||
		strings.Contains(lower, "q&a") {
		score += 0.3
		detection.Reasons = append(detection.Reasons, "name contains thank-you/questions keyword")
	}

	// Semantic: Name contains "end" as a word
	if containsWord(lower, "end") {
		score += 0.3
		detection.Reasons = append(detection.Reasons, "name contains 'end'")
	}

	// Structural: Has title, minimal or no body
	if counts.visibleTitle > 0 && counts.body == 0 {
		score += 0.1
		detection.Reasons = append(detection.Reasons, "has title without body")
	}

	// Position: Last layout is often closing slide (but weak signal)
	// Note: We can't easily check "last" here without knowing total count

	// Penalty: Has significant body content
	if counts.body > 0 && hasSignificantBodyCapacity(layout.Placeholders) {
		score -= 0.2
		detection.Reasons = append(detection.Reasons, "has significant body capacity (penalty)")
	}

	return clampScore(score)
}

// ValidateMinimalTemplate validates a template has the 4 required slide types.
// It loads and analyzes the template, then detects each layout's type and
// ensures all required types are present.
func ValidateMinimalTemplate(templatePath string) (*MinimalTemplateValidation, error) {
	result := &MinimalTemplateValidation{
		Valid:           false,
		TitleSlideIdx:   -1,
		ContentSlideIdx: -1,
		SectionSlideIdx: -1,
		ClosingSlideIdx: -1,
		Errors:          []string{},
		Warnings:        []string{},
	}

	// Open and analyze template
	reader, err := OpenTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Parse layouts
	layouts, err := ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layouts: %w", err)
	}

	if len(layouts) == 0 {
		result.Errors = append(result.Errors, "template has no layouts")
		return result, nil
	}

	// Detect type for each layout
	detections := make([]SlideTypeDetection, len(layouts))
	for i, layout := range layouts {
		detections[i] = DetectSlideType(layout)
	}

	// Assign types based on best matches (avoid duplicates)
	// Use a greedy algorithm: for each required type, find the best unassigned match
	assigned := make(map[int]bool)
	requiredTypes := []string{SlideTypeTitle, SlideTypeContent, SlideTypeSection, SlideTypeClosing}

	for _, reqType := range requiredTypes {
		bestIdx := -1
		bestScore := float64(0)

		for i, det := range detections {
			if assigned[i] {
				continue // Already assigned to another type
			}
			if det.DetectedAs == reqType && det.Confidence > bestScore {
				bestScore = det.Confidence
				bestIdx = i
			}
		}

		if bestIdx >= 0 {
			assigned[bestIdx] = true
			switch reqType {
			case SlideTypeTitle:
				result.TitleSlideIdx = bestIdx
			case SlideTypeContent:
				result.ContentSlideIdx = bestIdx
			case SlideTypeSection:
				result.SectionSlideIdx = bestIdx
			case SlideTypeClosing:
				result.ClosingSlideIdx = bestIdx
			}
		}
	}

	// Check for missing types and add errors
	if result.TitleSlideIdx == -1 {
		result.Errors = append(result.Errors, "missing title slide layout")
	}
	if result.ContentSlideIdx == -1 {
		result.Errors = append(result.Errors, "missing content slide layout")
	}
	if result.SectionSlideIdx == -1 {
		result.Errors = append(result.Errors, "missing section slide layout")
	}
	if result.ClosingSlideIdx == -1 {
		result.Errors = append(result.Errors, "missing closing slide layout")
	}

	// Add warnings for low-confidence detections
	for i, det := range detections {
		if assigned[i] && det.Confidence < 0.5 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("low confidence (%.0f%%) detection for %s slide: %s",
					det.Confidence*100, det.DetectedAs, det.LayoutName))
		}
	}

	// Template is valid if all 4 types were found
	result.Valid = len(result.Errors) == 0

	return result, nil
}

// Helper types and functions

// layoutCounts holds pre-computed counts of placeholder types for validation.
type layoutCounts struct {
	title        int
	visibleTitle int
	subtitle     int
	body         int
	image        int
	chart        int
}

// countLayoutPlaceholders counts placeholder types in a layout.
func countLayoutPlaceholders(placeholders []types.PlaceholderInfo) layoutCounts {
	var counts layoutCounts
	for _, ph := range placeholders {
		switch ph.Type {
		case types.PlaceholderTitle:
			counts.title++
			if ph.Bounds.Y >= 0 {
				counts.visibleTitle++
			}
		case types.PlaceholderSubtitle:
			counts.subtitle++
		case types.PlaceholderBody, types.PlaceholderContent:
			counts.body++
		case types.PlaceholderImage:
			counts.image++
		case types.PlaceholderChart:
			counts.chart++
		}
	}
	return counts
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

// hasSignificantBodyCapacity checks if any body placeholder has significant capacity.
func hasSignificantBodyCapacity(placeholders []types.PlaceholderInfo) bool {
	for _, ph := range placeholders {
		if (ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent) &&
			ph.MaxChars >= minBodyCapacity {
			return true
		}
	}
	return false
}

// clampScore clamps a score to the range [0, 1].
func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}
