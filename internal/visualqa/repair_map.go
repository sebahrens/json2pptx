package visualqa

// categoryFixMap maps visual QA finding categories to candidate repair_slide
// fix kinds, ordered by preference (most targeted first). When the visual QA
// agent detects a category, these are the fix kinds an agent should try via
// repair_slide's autofix_visual kind.
//
// Categories intentionally absent from this map have no deterministic
// source-side auto-fix and are review-only:
//   - image_quality: requires human judgement on image replacement
//   - aspect_ratio: requires human decision on cropping or source replacement
//   - border_style: cosmetic; no safe deterministic mutation
var categoryFixMap = map[string][]string{
	"text_overflow":     {"reduce_cell_text", "split_at_row", "reshape_grid"},
	"text_truncation":   {"reduce_cell_text", "split_at_row", "reshape_grid"},
	"contrast":          {"replace_color", "use_semantic_color"},
	"alignment":         {"reshape_grid", "swap_layout"},
	"spacing":           {"reshape_grid"},
	"overlap":           {"reshape_grid", "swap_layout"},
	"font_size":         {"reduce_text", "reshape_grid"},
	"missing_content":   {"provide_value"},
	"table_readability": {"split_at_row", "reshape_grid"},
	"layout_balance":    {"reshape_grid", "swap_layout"},
	"color_consistency": {"replace_color", "use_semantic_color"},
	"visual_hierarchy":  {"swap_layout", "reshape_grid"},
	"chart_readability": {"reshape_grid"},
	"footer_clearance":  {"reshape_grid"},
}

// ReviewOnlyCategories lists visual QA categories that have no deterministic
// auto-fix mapping. Agents receiving findings in these categories should
// present them for human review rather than attempting repair_slide.
var ReviewOnlyCategories = []string{
	"image_quality",
	"aspect_ratio",
	"border_style",
}

// SuggestedFixesForCategory returns the candidate repair_slide fix kinds for
// a visual QA finding category. Returns nil for categories with no mapped
// fixes (review-only categories like image_quality, aspect_ratio, border_style).
func SuggestedFixesForCategory(category string) []SuggestedFix {
	kinds, ok := categoryFixMap[category]
	if !ok {
		return nil
	}
	fixes := make([]SuggestedFix, len(kinds))
	for i, k := range kinds {
		fixes[i] = SuggestedFix{Kind: k}
	}
	return fixes
}

// IsReviewOnly reports whether a visual QA category has no deterministic
// auto-fix and should be presented for human review only.
func IsReviewOnly(category string) bool {
	if _, ok := categoryFixMap[category]; ok {
		return false
	}
	// Valid category with no mapping → review-only.
	return ValidCategory(category)
}

// BBox is a finding's region on the rendered slide in normalized slide
// coordinates: X/Y are the top-left corner and W/H the size, each a fraction
// of the slide width/height in [0, 1].
type BBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Valid reports whether the box has a positive area and lies (within a small
// tolerance) inside the unit slide.
func (b BBox) Valid() bool {
	const tol = 0.01
	return b.W > 0 && b.H > 0 && b.X >= -tol && b.Y >= -tol && b.X+b.W <= 1+tol && b.Y+b.H <= 1+tol
}

func (b BBox) area() float64 { return b.W * b.H }

func (b BBox) intersection(o BBox) float64 {
	w := min(b.X+b.W, o.X+o.W) - max(b.X, o.X)
	h := min(b.Y+b.H, o.Y+o.H) - max(b.Y, o.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

func (b BBox) contains(x, y float64) bool {
	return x >= b.X && x <= b.X+b.W && y >= b.Y && y <= b.Y+b.H
}

// ElementBox is the generated bounding box of one addressable slide element
// (e.g. a shape_grid cell) in the same normalized coordinates as BBox, keyed by
// the JSON Pointer a repair directive should target.
type ElementBox struct {
	Path string `json:"path"`
	Box  BBox   `json:"box"`
}

// minElementCoverage is the fraction of a finding's bbox that must overlap an
// element for the overlap fallback to attribute the finding to it.
const minElementCoverage = 0.5

// ElementPathAt hit-tests a finding bbox against generated element boxes and
// returns the path of the element the finding is about. Elements containing
// the bbox centre win, the smallest such element first (a cell beats a larger
// region around it). Otherwise the element covering the largest share of the
// bbox wins when it covers at least half of it. Returns ok=false when the bbox
// is invalid or no element matches, so callers fall back to a slide path.
func ElementPathAt(b BBox, elements []ElementBox) (string, bool) {
	if !b.Valid() || len(elements) == 0 {
		return "", false
	}
	cx, cy := b.X+b.W/2, b.Y+b.H/2
	best, bestArea := -1, 0.0
	for i, e := range elements {
		if e.Box.area() <= 0 || !e.Box.contains(cx, cy) {
			continue
		}
		if best < 0 || e.Box.area() < bestArea {
			best, bestArea = i, e.Box.area()
		}
	}
	if best >= 0 {
		return elements[best].Path, true
	}
	bestCover := 0.0
	for i, e := range elements {
		if c := b.intersection(e.Box) / b.area(); c > bestCover {
			best, bestCover = i, c
		}
	}
	if best >= 0 && bestCover >= minElementCoverage {
		return elements[best].Path, true
	}
	return "", false
}
