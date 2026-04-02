package template

import (
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// EMU (English Metric Units) constants for slide layout classification.
// Based on standard 16:9 slide dimensions: 9144000 x 6858000 EMU (10 inches wide).
const (
	// emuSlideHeight is the typical 16:9 slide height (7.5 inches in EMUs).
	emuSlideHeight = 6858000

	// emuLargeImageWidth is the threshold for "large" image width (>50% of slide).
	emuLargeImageWidth = 4500000

	// emuLargeImageHeight is the threshold for "large" image height (>50% of slide).
	emuLargeImageHeight = 3400000

	// emuYTolerance is the tolerance for Y alignment (20% of slide height).
	// Placeholders within this distance are considered side-by-side.
	emuYTolerance = 1300000

	// emuXMinSeparation is the minimum X separation for side-by-side detection
	// (10% of slide width).
	emuXMinSeparation = 900000

	// emuTitleBottomThreshold is the Y position threshold for detecting "title at bottom"
	// layouts. Titles positioned below this threshold (>50% of slide height) are considered
	// to be at the bottom, which is unusual for slides that need to display a standard
	// title-above-content layout.
	emuTitleBottomThreshold = emuSlideHeight / 2 // 3429000 EMU
)

// placeholderCounts holds pre-computed counts of placeholder types.
type placeholderCounts struct {
	title             int
	subtitle          int  // Subtitle placeholders (typically on title slides)
	visibleTitle      int  // Title placeholders that are visible (Y >= 0)
	titleAtBottom     bool // True if all title placeholders are positioned at the bottom of the slide
	body              int
	usableBody        int // Body placeholders large enough for content (MaxChars >= minUsableBodyChars)
	image             int
	chart             int
}

// minUsableBodyChars is the minimum character capacity for a body placeholder
// to be considered usable for content (bullet points, paragraphs, etc.).
// Placeholders below this threshold are likely designed for decorative elements
// like section numbers ("#") or short labels, not for actual content.
const minUsableBodyChars = 100

// countPlaceholders pre-computes placeholder type counts for classification.
// Note: PlaceholderOther (date, footer, slide number) is intentionally not counted
// as these are utility placeholders that don't contribute to content capacity.
// Title placeholders with negative Y positions (off-screen) are counted separately
// as they cannot display content to the user.
func countPlaceholders(placeholders []types.PlaceholderInfo) placeholderCounts {
	var counts placeholderCounts
	titleAtTopCount := 0
	titleAtBottomCount := 0

	for _, ph := range placeholders {
		switch ph.Type {
		case types.PlaceholderTitle:
			counts.title++
			// Only count visible titles (Y >= 0, on-screen)
			if ph.Bounds.Y >= 0 {
				counts.visibleTitle++
				// Check if title is at the bottom of the slide (Y > 50% of slide height)
				if ph.Bounds.Y > emuTitleBottomThreshold {
					titleAtBottomCount++
				} else {
					titleAtTopCount++
				}
			}
		case types.PlaceholderSubtitle:
			counts.subtitle++
		case types.PlaceholderBody, types.PlaceholderContent:
			counts.body++
			// Count usable body placeholders (large enough for actual content)
			// If MaxChars is set, use it; otherwise estimate usability from bounds
			usable := ph.MaxChars >= minUsableBodyChars
			if ph.MaxChars == 0 && ph.Bounds.Width > 0 && ph.Bounds.Height > 0 {
				// For test fixtures without MaxChars, estimate from bounds:
				// Consider usable if area is significant (roughly 2x2 inch = 4 sq inches)
				const minAreaEMU int64 = 2000000 * 2000000
				area := ph.Bounds.Width * ph.Bounds.Height
				usable = area >= minAreaEMU
			}
			if usable {
				counts.usableBody++
			}
		case types.PlaceholderImage:
			counts.image++
		case types.PlaceholderChart:
			counts.chart++
			// PlaceholderOther (dt, ftr, sldNum, hdr) - intentionally not counted
		}
	}

	// Layout has title at bottom if there are title placeholders and all visible ones
	// are positioned below the 50% threshold
	counts.titleAtBottom = counts.title > 0 && titleAtTopCount == 0 && titleAtBottomCount > 0

	return counts
}

// ClassifyLayout assigns classification tags to a layout based on its placeholders.
func ClassifyLayout(layout *types.LayoutMetadata) { //nolint:gocyclo
	if layout == nil {
		return
	}

	counts := countPlaceholders(layout.Placeholders)
	var tags []string

	// Title slide: Single visible title + optional subtitle, no body
	// Uses visibleTitle to ensure the title is actually displayed
	if counts.visibleTitle > 0 && counts.body == 0 && counts.image == 0 && counts.chart == 0 {
		tags = append(tags, "title-slide")
	}

	// Content: Visible title + usable body placeholder
	// Only layouts with visible (on-screen) title placeholders are tagged as content.
	// Layouts with off-screen titles (like "Statement" layouts) should not be used
	// for slides that need to display a title.
	// Uses usableBody to ensure placeholders are large enough for actual content;
	// tiny placeholders (e.g., section number "#") don't make a layout suitable for content.
	if counts.visibleTitle > 0 && counts.usableBody > 0 {
		tags = append(tags, "content")
	}

	// Title-hidden: Has title placeholder but it's off-screen (negative Y)
	// These layouts are designed for statement/quote slides where only body shows
	if counts.title > 0 && counts.visibleTitle == 0 && counts.body > 0 {
		tags = append(tags, "title-hidden")
	}

	// Title-at-bottom: Title placeholder is positioned in the lower half of the slide
	// These are special layouts (Quote, Statement, etc.) where the title appears at the
	// bottom by design. They should not be used for slides that expect standard
	// title-above-content positioning.
	if counts.titleAtBottom && counts.body > 0 {
		tags = append(tags, "title-at-bottom")
	}

	// Two-column and Comparison: At least two body placeholders side-by-side
	// Note: Many templates have additional placeholders (date, footer, etc.) that inflate body count,
	// so we check for >= 2 body placeholders with at least one pair positioned side-by-side
	if counts.body >= 2 && areSideBySide(layout.Placeholders, types.PlaceholderBody, types.PlaceholderContent) {
		tags = append(tags, "two-column", "comparison")
	}

	// Image-left or Image-right: Image placeholder with content
	if counts.image > 0 && counts.body > 0 {
		if hasImageOnLeft(layout.Placeholders) {
			tags = append(tags, "image-left")
		} else {
			tags = append(tags, "image-right")
		}
	}

	// Full-image: Large image placeholder, minimal text
	if counts.image > 0 && counts.body == 0 && hasLargeImage(layout.Placeholders) {
		tags = append(tags, "full-image")
	}

	// Chart-capable: Contains chart placeholder
	if counts.chart > 0 {
		tags = append(tags, "chart-capable")
	}

	// Blank: No placeholders
	if len(layout.Placeholders) == 0 {
		tags = append(tags, "blank")
	}

	// Blank-title: Exactly one visible title, no other content placeholders,
	// AND the layout name suggests it's a blank variant (not a title/section slide).
	// Used as base for virtual layouts (grid-based content overlaid via SVG).
	if counts.visibleTitle == 1 && counts.title == 1 && counts.subtitle == 0 &&
		counts.body == 0 && counts.image == 0 && counts.chart == 0 &&
		isBlankTitleByName(layout.Name) {
		tags = append(tags, "blank-title")
	}

	// Semantic tags based on layout name patterns
	tags = append(tags, classifyByName(layout.Name, counts)...)

	layout.Tags = tags
}

// layoutClassification defines a rule for inferring a semantic tag from layout name keywords.
type layoutClassification struct {
	tag              string   // The semantic tag to apply
	keywords         []string // Keywords to match (substring match)
	wordBoundaryKeys []string // Keywords that require word boundary matching (e.g., "end")
}

// layoutClassifications defines all the classification rules for layout names.
// Each rule specifies a tag and the keywords that trigger it.
// New classifications can be added by appending to this slice.
var layoutClassifications = []layoutClassification{
	// Quote layouts - typically have centered text for quotations
	{tag: "quote", keywords: []string{"quote", "quotation"}},

	// Statement layouts - single impactful phrase, often large centered text
	{tag: "statement", keywords: []string{"statement"}},

	// Big number / metric / KPI layouts - for data highlights
	{tag: "big-number", keywords: []string{"number", "metric", "kpi", "stats", "statistic"}},

	// Section header / divider layouts - for transitions between sections
	{tag: "section-header", keywords: []string{"section", "divider", "break", "transition"}},

	// Agenda layouts - structured list of topics
	{tag: "agenda", keywords: []string{"agenda", "outline", "contents", "overview"}},

	// Timeline layouts - sequential content with markers
	{tag: "timeline-capable", keywords: []string{"timeline", "process", "roadmap", "milestone"}},

	// Icon grid layouts - multiple icon placeholders
	{tag: "icon-grid", keywords: []string{"icon", "grid", "matrix"}},

	// Closing layouts - for final slides (AC13: Last Slide Closing Layout)
	// Note: "end" requires word boundary to avoid false matches (e.g., "Agenda" contains "end")
	{tag: "closing", keywords: []string{"closing", "close", "final", "conclusion"}, wordBoundaryKeys: []string{"end"}},

	// Thank-you layouts
	{tag: "thank-you", keywords: []string{"thank", "thanks", "q&a", "questions"}},
}

// classifyByName infers semantic tags from layout name keywords.
// These tags help match layouts to specific content types beyond structural analysis.
// Uses the layoutClassifications registry for easy maintenance and extensibility.
func classifyByName(name string, _ placeholderCounts) []string {
	var tags []string
	lower := strings.ToLower(name)

	for _, classification := range layoutClassifications {
		if matchesClassification(lower, classification) {
			tags = append(tags, classification.tag)
		}
	}

	return tags
}

// matchesClassification checks if a layout name matches a classification rule.
func matchesClassification(name string, classification layoutClassification) bool {
	// Check substring keywords
	for _, keyword := range classification.keywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}

	// Check word-boundary keywords
	for _, keyword := range classification.wordBoundaryKeys {
		if containsWord(name, keyword) {
			return true
		}
	}

	return false
}

// containsWord checks if a string contains a word as a complete word (not as a substring).
// For example, "end slide" contains word "end", but "agenda" does not contain word "end".
func containsWord(s, word string) bool {
	word = strings.ToLower(word)
	s = strings.ToLower(s)

	// Check if word appears at start of string
	if strings.HasPrefix(s, word) {
		if len(s) == len(word) {
			return true // Exact match
		}
		// Check if character after word is non-letter (word boundary)
		nextChar := rune(s[len(word)])
		if !isLetter(nextChar) {
			return true
		}
	}

	// Check if word appears elsewhere with word boundaries
	for i := 0; i <= len(s)-len(word); i++ {
		// Check if word starts after a non-letter boundary
		if i > 0 && isLetter(rune(s[i-1])) {
			continue
		}
		// Check if substring matches
		if s[i:i+len(word)] != word {
			continue
		}
		// Check if word ends at string end or before a non-letter
		endPos := i + len(word)
		if endPos == len(s) || !isLetter(rune(s[endPos])) {
			return true
		}
	}

	return false
}

// isLetter checks if a rune is a letter (a-z or A-Z).
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isBlankTitleByName checks if a layout name suggests a blank+title variant.
// Matches names containing "blank" (e.g., "Blank", "Blank + Title", "Blank Layout").
// Also matches synthesized blank-title layouts ("Blank + Title").
func isBlankTitleByName(name string) bool {
	return strings.Contains(strings.ToLower(name), "blank")
}

// hasLargeImage checks if any image placeholder takes up significant space.
func hasLargeImage(placeholders []types.PlaceholderInfo) bool {
	for _, ph := range placeholders {
		if ph.Type == types.PlaceholderImage {
			if ph.Bounds.Width > emuLargeImageWidth && ph.Bounds.Height > emuLargeImageHeight {
				return true
			}
		}
	}
	return false
}

// areSideBySide checks if two placeholders of given types are positioned side-by-side.
func areSideBySide(placeholders []types.PlaceholderInfo, type1, type2 types.PlaceholderType) bool {
	var targets []types.PlaceholderInfo

	for _, ph := range placeholders {
		if ph.Type == type1 || ph.Type == type2 {
			targets = append(targets, ph)
		}
	}

	if len(targets) < 2 {
		return false
	}

	// Check if the first two have similar Y positions but different X positions
	ph1 := targets[0]
	ph2 := targets[1]

	yDiff := math.Abs(float64(ph1.Bounds.Y - ph2.Bounds.Y))
	xDiff := math.Abs(float64(ph1.Bounds.X - ph2.Bounds.X))

	return yDiff < emuYTolerance && xDiff > emuXMinSeparation
}

// hasImageOnLeft checks if any image placeholder is positioned on the left side.
func hasImageOnLeft(placeholders []types.PlaceholderInfo) bool {
	var imageX int64 = math.MaxInt64
	var bodyX int64 = math.MaxInt64
	foundImage := false
	foundBody := false

	for _, ph := range placeholders {
		if ph.Type == types.PlaceholderImage && ph.Bounds.X < imageX {
			imageX = ph.Bounds.X
			foundImage = true
		}
		if (ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent) && ph.Bounds.X < bodyX {
			bodyX = ph.Bounds.X
			foundBody = true
		}
	}

	// Need both image and body to determine position
	if !foundImage || !foundBody {
		return false
	}

	// Image is on left if its X position is less than body X position
	return imageX < bodyX
}
