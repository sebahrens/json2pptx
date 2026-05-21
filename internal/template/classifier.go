package template

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Canonical layout role names, as defined in docs/TEMPLATE_SPEC.md. These are
// the roles that template-check considers mandatory and that the repair
// pipeline uses when deciding whether to rename an existing layout vs. author
// a new one. Keep these strings stable — they appear in template-check output
// and in agent-visible warnings.
const (
	CanonicalRoleTitleSlide     = "Title Slide"
	CanonicalRoleOneContent     = "One Content"
	CanonicalRoleTwoContent     = "Two Content"
	CanonicalRoleSectionDivider = "Section Divider"
	CanonicalRoleBlank          = "Blank"
	CanonicalRoleBlankTitle     = "Blank + Title"
	CanonicalRoleClosing        = "Closing"
)

// CanonicalConfidenceThreshold is the confidence above which ClassifyCanonicalRole
// is considered "confident enough" to drive a rename suggestion. Layouts at or
// above this threshold should be renamed in place rather than duplicated.
const CanonicalConfidenceThreshold = 0.8

// CanonicalLayoutNames maps each canonical role to its accepted layout names
// (case-insensitive). The first entry is the canonical name used in rename
// suggestions; subsequent entries are accepted aliases. This mapping is the
// single source of truth shared by template-check and the repair pipeline.
var CanonicalLayoutNames = map[string][]string{
	CanonicalRoleTitleSlide:     {"Title Slide"},
	CanonicalRoleOneContent:     {"One Content", "Content"},
	CanonicalRoleTwoContent:     {"Two Content", "Comparison"},
	CanonicalRoleSectionDivider: {"Section Divider", "Section Header"},
	CanonicalRoleBlank:          {"Blank"},
	CanonicalRoleBlankTitle:     {"Blank + Title", "Blank Layout"},
	CanonicalRoleClosing:        {"Closing", "Thank You", "End Slide"},
}

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
	title         int
	subtitle      int  // Subtitle placeholders (typically on title slides)
	visibleTitle  int  // Title placeholders that are visible (Y >= 0)
	titleAtBottom bool // True if all title placeholders are positioned at the bottom of the slide
	compactTitle  bool // True if a visible title placeholder only fits a short single-line title
	body          int
	usableBody    int // Body placeholders large enough for content (MaxChars >= minUsableBodyChars)
	image         int
	chart         int
}

// minUsableBodyChars is the minimum character capacity for a body placeholder
// to be considered usable for content (bullet points, paragraphs, etc.).
// Placeholders below this threshold are likely designed for decorative elements
// like section numbers ("#") or short labels, not for actual content.
const minUsableBodyChars = 100

// compactTitleMaxChars is the capacity ceiling below which a visible title
// placeholder is considered "compact" — it only holds a short, single-line
// title. Once estimateMaxChars scales by font size, large-font title slots
// report ~14–32 characters while roomy multi-line title slots (e.g. some
// templates' Section Divider) report ~69+, so a ceiling of 35 sits above the
// tight cluster and below the roomy slots. Used (in combination with
// title-at-bottom geometry) to emit the "compact-title" planning hint.
const compactTitleMaxChars = 35

// countPlaceholders pre-computes placeholder type counts for classification.
// Note: PlaceholderOther (date, footer, slide number) is intentionally not counted
// as these are utility placeholders that don't contribute to content capacity.
// Title placeholders with negative Y positions (off-screen) are counted separately
// as they cannot display content to the user.
func countPlaceholders(placeholders []types.PlaceholderInfo) placeholderCounts {
	var counts placeholderCounts
	titleAtTopCount := 0
	titleAtBottomCount := 0
	visibleTitleHasChars := false
	minVisibleTitleMaxChars := 0

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
				// Track the tightest visible title capacity so we can flag
				// layouts whose title slot only fits a short single-line title.
				// MaxChars == 0 means the font size was unknown, so capacity is
				// indeterminate and must not count as "compact".
				if ph.MaxChars > 0 && (!visibleTitleHasChars || ph.MaxChars < minVisibleTitleMaxChars) {
					minVisibleTitleMaxChars = ph.MaxChars
					visibleTitleHasChars = true
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

	// Layout has a compact title if its tightest visible title placeholder only
	// holds a short single-line title.
	counts.compactTitle = visibleTitleHasChars && minVisibleTitleMaxChars < compactTitleMaxChars

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

	// Compact-title: a visible title placeholder that is BOTH positioned low on
	// the slide (title-at-bottom geometry) AND small enough to hold only a short
	// single-line title. This marks the specific truncation hazard where a
	// descriptive section title is squeezed into a small slot beneath a large
	// decorative element (e.g. modern-yellow's Section Divider, where a 208pt
	// "Section Number" frame sits above a 48pt title slot). Planners should keep
	// titles on these layouts terse; per-layout MaxChars carries the raw budget.
	// Gating on title-at-bottom keeps the hint targeted: ordinary content/title
	// slots are also capacity-limited but are not an unexpected hazard. The
	// gate mirrors the title-at-bottom tag condition exactly (counts.titleAtBottom
	// && counts.body > 0) so "compact-title" is always emitted alongside it.
	if counts.titleAtBottom && counts.body > 0 && counts.compactTitle {
		tags = append(tags, "compact-title")
	}

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

// ClassifyCanonicalRole maps a layout to its canonical role (per
// docs/TEMPLATE_SPEC.md) based on its structural fingerprint — placeholder
// types, count, and side-by-side relationships — and the existing semantic
// tags computed by ClassifyLayout.
//
// Returns:
//   - canonicalRole: one of the CanonicalRole* constants, or "" if the layout
//     does not structurally correspond to any canonical role.
//   - signature: a deterministic, sorted, structural fingerprint string
//     (e.g. "blank", "title", "title+subtitle", "title+body",
//     "title+body+body[side-by-side]"). Two layouts with the same signature
//     are structurally equivalent and one of them is a duplicate that the
//     repair pipeline must surface.
//   - confidence: 0.0–1.0. The repair pipeline and the name-mismatch
//     template-check use CanonicalConfidenceThreshold to decide whether to
//     trust the classification.
//
// This function is idempotent and side-effect-free with respect to the layout
// argument; it does not mutate the layout (unlike ClassifyLayout, which sets
// layout.Tags). Callers that need tags must call ClassifyLayout separately.
func ClassifyCanonicalRole(layout *types.LayoutMetadata) (canonicalRole, signature string, confidence float64) {
	if layout == nil {
		return "", "", 0
	}

	counts := countPlaceholders(layout.Placeholders)
	signature = layoutSignature(layout.Placeholders, counts)

	// Compute tags locally so we don't depend on layout.Tags being populated.
	// We don't mutate layout — work on a shallow copy.
	localTags := classifyByName(layout.Name, counts)
	tagsSet := make(map[string]bool, len(localTags))
	for _, t := range localTags {
		tagsSet[t] = true
	}

	hasUsableBody := counts.usableBody > 0
	hasAnyBody := counts.body > 0

	// Blank: no placeholders at all. Highest possible structural confidence.
	if len(layout.Placeholders) == 0 {
		return CanonicalRoleBlank, signature, 1.0
	}

	// Two Content: title + at least two body placeholders that sit side-by-side.
	// Structural fingerprint disambiguates from Title Slide and One Content,
	// so it can be evaluated before the name-based checks.
	if counts.visibleTitle > 0 && counts.body >= 2 &&
		areSideBySide(layout.Placeholders, types.PlaceholderBody, types.PlaceholderContent) {
		return CanonicalRoleTwoContent, signature, 0.95
	}

	// Closing, Section Divider, and Blank+Title share the same structural
	// fingerprint as Title Slide (a centered title, sometimes with subtitle).
	// They MUST be evaluated before the Title Slide branch — otherwise a
	// layout named "Closing" or "Section Divider" would mis-classify as
	// Title Slide.

	// Closing: name-based signal ("closing", "end slide", "thank you", "q&a").
	if tagsSet["closing"] || tagsSet["thank-you"] {
		return CanonicalRoleClosing, signature, 0.9
	}

	// Section Divider: name-based signal. Section dividers look like Title
	// Slides structurally, so we rely on the name tag.
	if tagsSet["section-header"] {
		return CanonicalRoleSectionDivider, signature, 0.9
	}

	// Blank + Title: single visible title and nothing else, AND a name that
	// suggests "blank". The strict structural test mirrors ClassifyLayout's
	// "blank-title" tag.
	if counts.visibleTitle == 1 && counts.title == 1 && counts.subtitle == 0 &&
		!hasAnyBody && counts.image == 0 && counts.chart == 0 &&
		isBlankTitleByName(layout.Name) {
		return CanonicalRoleBlankTitle, signature, 0.9
	}

	// Title Slide: visible title (often paired with subtitle), no body, no
	// image/chart content. Tightest purely-structural fingerprint.
	if counts.visibleTitle > 0 && !hasAnyBody && counts.image == 0 && counts.chart == 0 {
		// title+subtitle is the strongest signal; title-only is a hero/cover
		// variant that we still map to Title Slide with slightly lower confidence.
		if counts.subtitle > 0 {
			return CanonicalRoleTitleSlide, signature, 0.95
		}
		return CanonicalRoleTitleSlide, signature, 0.85
	}

	// One Content: visible title + at least one usable body placeholder, and
	// not already classified as Two Content above.
	if counts.visibleTitle > 0 && hasUsableBody {
		return CanonicalRoleOneContent, signature, 0.85
	}

	return "", signature, 0
}

// CanonicalNameFor returns the preferred (canonical) display name for a role,
// or "" if the role is not known. This is the name that name-mismatch warnings
// recommend renaming to.
func CanonicalNameFor(role string) string {
	names, ok := CanonicalLayoutNames[role]
	if !ok || len(names) == 0 {
		return ""
	}
	return names[0]
}

// IsCanonicalLayoutName reports whether name (case-insensitive) is an accepted
// name for the given canonical role.
func IsCanonicalLayoutName(role, name string) bool {
	names, ok := CanonicalLayoutNames[role]
	if !ok {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, n := range names {
		if lower == strings.ToLower(n) {
			return true
		}
	}
	return false
}

// layoutSignature builds a deterministic structural fingerprint string for a
// layout. Two layouts that produce the same signature are structurally
// equivalent and only differ in name / decorative content. The signature is
// stable across runs (no map iteration, sorted ordering) and intentionally
// excludes utility placeholders (date, footer, slide number).
//
// Component vocabulary (kept short on purpose so signatures stay readable in
// agent-visible output):
//
//	title          – visible title placeholder
//	titlehidden    – title placeholder positioned off-screen
//	titlebottom    – title placeholder positioned in the lower half
//	subtitle       – subtitle placeholder
//	body           – body / content placeholder usable for text
//	image          – image placeholder
//	chart          – chart placeholder
//
// A trailing "[side-by-side]" annotation marks layouts where two body or
// content placeholders sit on the same row (two-column / comparison layouts).
func layoutSignature(placeholders []types.PlaceholderInfo, counts placeholderCounts) string {
	if len(placeholders) == 0 {
		return "blank"
	}

	parts := make(map[string]int)
	for _, ph := range placeholders {
		switch ph.Type {
		case types.PlaceholderTitle:
			switch {
			case ph.Bounds.Y < 0:
				parts["titlehidden"]++
			case ph.Bounds.Y > emuTitleBottomThreshold:
				parts["titlebottom"]++
			default:
				parts["title"]++
			}
		case types.PlaceholderSubtitle:
			parts["subtitle"]++
		case types.PlaceholderBody, types.PlaceholderContent:
			parts["body"]++
		case types.PlaceholderImage:
			parts["image"]++
		case types.PlaceholderChart:
			parts["chart"]++
		case types.PlaceholderOther:
			// utility placeholders (date, footer, sldNum) intentionally skipped
		}
	}

	if len(parts) == 0 {
		return "blank"
	}

	keys := make([]string, 0, len(parts))
	for k := range parts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("+")
		}
		if parts[k] == 1 {
			sb.WriteString(k)
		} else {
			fmt.Fprintf(&sb, "%s*%d", k, parts[k])
		}
	}

	if counts.body >= 2 && areSideBySide(placeholders, types.PlaceholderBody, types.PlaceholderContent) {
		sb.WriteString("[side-by-side]")
	}

	return sb.String()
}
