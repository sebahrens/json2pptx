package template

import (
	"sort"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Default slide canvas (16:9 widescreen) used when the template does not
// declare <p:sldSz>.
const (
	chromeDefaultSlideWidth  int64 = 12192000
	chromeDefaultSlideHeight int64 = 6858000
)

// Chrome frame basis values: where the horizontal extent of the frame came
// from. Exposed on examine_template so agents can tell a template-derived band
// from a slide-percentage fallback.
const (
	ChromeBasisLayout          = "layout"           // the slide's own body/content placeholders
	ChromeBasisLayoutTitle     = "layout_title"     // a title-only layout: its title column, the span its shape_grid content uses
	ChromeBasisReferenceLayout = "reference_layout" // the template's One Content binding (layout has no body or title)
	ChromeBasisSlideFallback   = "slide_fallback"   // no placeholder geometry at all: slide-percentage margins
)

// ChromeRect is a JSON-friendly EMU rectangle.
type ChromeRect struct {
	X  int64 `json:"x_emu"`
	Y  int64 `json:"y_emu"`
	CX int64 `json:"w_emu"`
	CY int64 `json:"h_emu"`
}

// Rect converts to the pptx rectangle used by shape emission.
func (r ChromeRect) Rect() pptx.RectEmu { return pptx.RectEmu{X: r.X, Y: r.Y, CX: r.CX, CY: r.CY} }

// Bottom returns the rectangle's bottom edge.
func (r ChromeRect) Bottom() int64 { return r.Y + r.CY }

// IsZero reports whether the rectangle is unset.
func (r ChromeRect) IsZero() bool { return r == ChromeRect{} }

// ChromeFrame is the layout-derived contract shared by content reservation
// (shape_grid zones, body placeholder clamping) and late shape emission
// (takeaway band, source note). Every rectangle lies inside the canvas.
//
//   - Left/right edges come from the layout's body/content placeholders (union
//     of their x .. x+cx), so the takeaway aligns with the template's text
//     column instead of a slide-percentage margin.
//   - The band stack sits above the top of the footer chrome (min Y of the
//     dt/ftr/sldNum placeholders) minus a gap, so it never overlaps footers.
//   - Content spans from below the title to above the band stack.
//
// Fits is false when the requested bands would climb into the title, or —
// on layouts with their own content placeholders — would leave less than
// minChromeContentRatio of the slide height for content. Callers must then
// skip the band and surface a finding instead of overlapping chrome.
type ChromeFrame struct {
	Canvas    ChromeRect `json:"canvas"`
	Content   ChromeRect `json:"content"`
	Takeaway  ChromeRect `json:"takeaway_band"`
	Source    ChromeRect `json:"source_band"`
	FooterTop int64      `json:"footer_top_emu"`
	HasFooter bool       `json:"has_footer"`
	Basis     string     `json:"basis"`
	Fits      bool       `json:"fits"`
	// SideDecorInset is true when master/layout side artwork (an edge stripe,
	// a tall panel) moved the content column in from the layout's own body
	// span. The title placeholder then shifts with the content so the title
	// and the content keep one left edge (go-slide-creator-oa0ru).
	SideDecorInset bool `json:"-"`
}

// sourceBandClearance is the minimum gap, in EMU, between the source/footnote
// zone and the band or content above it: 12pt.
const sourceBandClearance int64 = 12 * 12700

// minChromeContentRatio is the minimum share of the slide height (percent)
// that must remain for content once the takeaway/source bands are reserved.
const minChromeContentRatio = 20

// Takeaway band air, in EMU: 12pt to the source line / footer below and 16pt
// to the content above (patterns.TakeawayGapBelowPt / TakeawayGapAbovePt;
// the template package cannot import patterns).
const (
	takeawayGapBelowEMU int64 = 12 * 12700
	takeawayGapAboveEMU int64 = 16 * 12700
)

// ResolveChromeFrame derives the chrome frame for a slide on layout. reference
// supplies horizontal geometry when layout has no body/content placeholders
// (title-only, blank, or unknown layouts) — normally the template's One Content
// binding (see ChromeReferenceLayout). Either may be nil.
func ResolveChromeFrame(layout, reference *types.LayoutMetadata, slideWidth, slideHeight int64, hasTakeaway, hasSource bool) ChromeFrame {
	w, h := slideWidth, slideHeight
	if w <= 0 {
		w = chromeDefaultSlideWidth
	}
	if h <= 0 {
		h = chromeDefaultSlideHeight
	}
	marginX := w * 375 / 10000
	edgeMargin := h * 290 / 10000
	gap := h * 70 / 10000
	sourceH := max(h*292/10000, 160000)
	takeawayH := max(h*750/10000, 300000)

	frame := ChromeFrame{Canvas: ChromeRect{CX: w, CY: h}, Basis: ChromeBasisSlideFallback}

	left, right := marginX, w-marginX
	if l, r, ok := contentSpan(layout, w); ok {
		left, right, frame.Basis = l, r, ChromeBasisLayout
	} else if l, r, ok := titleColumnSpan(layout, w, h); ok {
		// A title-only layout's grid content spans the title column
		// (titleOnlyContentZone); the takeaway / source bands share that
		// left edge instead of the One Content layout's body edge, which
		// sat 45px right of every pattern on abstract (go-slide-creator-svrpx).
		left, right, frame.Basis = l, r, ChromeBasisLayoutTitle
	} else if l, r, ok := contentSpan(reference, w); ok {
		left, right, frame.Basis = l, r, ChromeBasisReferenceLayout
	}

	top := edgeMargin
	if tb, ok := titleBottom(layout, h); ok {
		top = tb + gap
	}

	footerLayout := layout
	if footerLayout == nil || len(footerLayout.FooterRegions) == 0 {
		// Only borrow the reference layout's footers when the slide's layout
		// is unknown; a known layout without footers genuinely has none.
		if layout == nil {
			footerLayout = reference
		}
	}
	bottom := h - edgeMargin
	if ft, ok := FooterTop(footerLayout, h); ok {
		frame.FooterTop, frame.HasFooter = ft, true
		bottom = ft - gap
	} else {
		frame.FooterTop = h - edgeMargin
	}

	top, bottom = reserveDisclosureChromeBand(layout, h, gap, top, bottom)
	// The takeaway band keeps at least 12pt to the source line / footer below
	// it and 16pt of air above it (go-slide-creator-7b5o6); the 0.7%-of-height
	// gap left it ~4pt from the footer and touching it on some templates.
	takeawayGapBelow := max(gap, takeawayGapBelowEMU)
	takeawayGapAbove := max(gap, takeawayGapAboveEMU)
	y := bottom
	if hasSource {
		y -= sourceH
		frame.Source = ChromeRect{X: left, Y: y, CX: right - left, CY: sourceH}
		// The source zone is a footnote, not part of the takeaway: keep at
		// least 12pt between the attribution and whatever sits above it (the
		// takeaway band or the content), so the two never read as one block
		// (go-slide-creator-cuszt).
		y -= max(gap, sourceBandClearance)
	}
	if hasTakeaway {
		// bottom already sits one gap above the footer, and a source band
		// is followed by one gap; top that up to the takeaway's 12pt.
		y -= takeawayGapBelow - gap
		y -= takeawayH
		frame.Takeaway = ChromeRect{X: left, Y: y, CX: right - left, CY: takeawayH}
		y -= takeawayGapAbove
	}
	// Tall, filled master/layout artwork at a side of the body column is
	// reserved as chrome. Narrow rules and large background panels do not
	// consume a whole column. The same horizontal reservation applies to the
	// content and to both optional bands.
	decorLayout := layout
	if decorLayout == nil {
		decorLayout = reference
	}
	bareLeft, bareRight := left, right
	left, right = excludeSideDecor(decorLayout, left, right, top, y, w, h)
	frame.SideDecorInset = left != bareLeft || right != bareRight
	if hasTakeaway {
		frame.Takeaway.X, frame.Takeaway.CX = left, right-left
	}
	if hasSource {
		frame.Source.X, frame.Source.CX = left, right-left
	}
	if y < top {
		frame.Content = ChromeRect{X: left, Y: top, CX: right - left, CY: 0}
	} else {
		frame.Content = ChromeRect{X: left, Y: top, CX: right - left, CY: y - top}
	}
	frame.Fits = true
	if hasTakeaway || hasSource {
		// The band stack must never climb into the title. When the layout
		// carries its own content placeholders, the reservation must also
		// leave a usable content area; title-only / blank layouts have no
		// placeholder content to protect.
		frame.Fits = y >= top
		if frame.Basis == ChromeBasisLayout {
			frame.Fits = frame.Fits && frame.Content.CY*100 >= h*minChromeContentRatio
		}
	}
	return frame
}

func reserveDisclosureChromeBand(layout *types.LayoutMetadata, slideHeight, gap, top, bottom int64) (int64, int64) {
	if layout == nil {
		return top, bottom
	}
	for _, ph := range layout.Placeholders {
		if !IsDisclosurePlaceholder(ph) {
			continue
		}
		if ph.Bounds.Y < slideHeight/2 {
			top = max(top, ph.Bounds.Y+ph.Bounds.Height+gap)
		} else {
			bottom = min(bottom, ph.Bounds.Y-gap)
		}
	}
	return top, bottom
}

func excludeSideDecor(layout *types.LayoutMetadata, left, right, top, bottom, slideWidth, slideHeight int64) (int64, int64) {
	if layout == nil || bottom <= top {
		return left, right
	}
	padding := min(slideWidth, slideHeight) * 26 / 1000 // about 0.2 in on 16:9
	origLeft, origRight := left, right
	for _, decor := range layout.DecorRegions {
		if decor.Width <= 0 || decor.Height <= 0 || decor.Width*5 >= slideWidth*2 {
			continue // no-fill/invalid or broad background panel
		}
		overlap := min(bottom, decor.Y+decor.Height) - max(top, decor.Y)
		if overlap*2 < bottom-top {
			continue // a short title/footer accent is not a side exclusion
		}
		end := decor.X + decor.Width
		if decor.X <= left && end > left && end+padding < right {
			left = end + padding
		} else if decor.X < right && end >= right && decor.X-padding > left {
			right = decor.X - padding
		}
	}
	if left != origLeft || right != origRight {
		// Art that overlapped the body column already moved it clear.
		return left, right
	}
	return excludeEdgeArt(layout, left, right, top, bottom, slideWidth, padding)
}

// excludeEdgeArt clears decorative edge art that sits wholly inside a side
// margin — midnight-blue's full-height accent bar and stripe at the left edge.
// The body column already starts to the right of such art, so the straddle
// rule above never fires, yet the art eats most of the margin and crowds the
// title and content against it. The fix is geometric, not per template: the
// content keeps, measured from the art's inner edge, the margin the layout
// gives the undecorated side. Art on both sides leaves the column unchanged
// (there is no clean side to mirror), as does a clearance that would leave
// less than half the slide width for content (go-slide-creator-oa0ru).
func excludeEdgeArt(layout *types.LayoutMetadata, left, right, top, bottom, slideWidth, padding int64) (int64, int64) {
	var leftArtEnd, rightArtStart int64 = -1, -1
	for _, decor := range layout.DecorRegions {
		if decor.Width <= 0 || decor.Height <= 0 || decor.Width*5 >= slideWidth*2 {
			continue
		}
		overlap := min(bottom, decor.Y+decor.Height) - max(top, decor.Y)
		if overlap*2 < bottom-top {
			continue
		}
		end := decor.X + decor.Width
		switch {
		case end <= left && decor.X < slideWidth/2:
			leftArtEnd = max(leftArtEnd, end)
		case decor.X >= right && decor.X > slideWidth/2:
			if rightArtStart < 0 || decor.X < rightArtStart {
				rightArtStart = decor.X
			}
		}
	}
	if (leftArtEnd >= 0) == (rightArtStart >= 0) {
		return left, right
	}
	if leftArtEnd >= 0 {
		clearance := max(slideWidth-right, padding)
		if want := leftArtEnd + clearance; want > left && (right-want)*2 >= slideWidth {
			left = want
		}
		return left, right
	}
	clearance := max(left, padding)
	if want := rightArtStart - clearance; want < right && (want-left)*2 >= slideWidth {
		right = want
	}
	return left, right
}

// FooterTop returns the top edge of the visible footer chrome on layout: the
// minimum Y of its dt/ftr/sldNum regions that sit in the lower half of the
// slide and fully on the canvas (templates often park a hidden date field
// below the slide edge).
func FooterTop(layout *types.LayoutMetadata, slideHeight int64) (int64, bool) {
	if layout == nil {
		return 0, false
	}
	var top int64
	found := false
	for _, r := range layout.FooterRegions {
		if r.Height <= 0 || r.Y < slideHeight/2 || r.Y+r.Height > slideHeight {
			continue
		}
		if !found || r.Y < top {
			top, found = r.Y, true
		}
	}
	return top, found
}

// contentSpan returns the horizontal extent (left, right) of the layout's
// content-bearing placeholders: body, generic content, picture, chart, and
// table placeholders (section-number decorations excluded).
func contentSpan(layout *types.LayoutMetadata, slideWidth int64) (int64, int64, bool) {
	if layout == nil {
		return 0, 0, false
	}
	var left, right int64
	found := false
	for i := range layout.Placeholders {
		ph := &layout.Placeholders[i]
		if IsDisclosurePlaceholder(*ph) || ph.Role == types.PlaceholderRoleSectionNumber || ph.Bounds.Width <= 0 {
			continue
		}
		switch ph.Type {
		case types.PlaceholderBody, types.PlaceholderContent, types.PlaceholderImage,
			types.PlaceholderChart, types.PlaceholderTable:
		default:
			continue
		}
		l, r := ph.Bounds.X, ph.Bounds.X+ph.Bounds.Width
		if l < 0 {
			l = 0
		}
		if r > slideWidth {
			r = slideWidth
		}
		if !found || l < left {
			left = l
		}
		if !found || r > right {
			right = r
		}
		found = true
	}
	return left, right, found && right > left
}

// titleColumnSpan returns the horizontal span shape_grid content takes on a
// title-only layout: the title's left edge, and a right edge symmetric to it
// (or the title's own right edge when that is further right) — the same rule
// as the generator's titleOnlyContentZone.
func titleColumnSpan(layout *types.LayoutMetadata, slideWidth, slideHeight int64) (int64, int64, bool) {
	if layout == nil {
		return 0, 0, false
	}
	for i := range layout.Placeholders {
		ph := &layout.Placeholders[i]
		if ph.Type != types.PlaceholderTitle || ph.Bounds.Width <= 0 || ph.Bounds.Y >= slideHeight/2 {
			continue
		}
		left := max(ph.Bounds.X, 0)
		right := max(slideWidth-left, ph.Bounds.X+ph.Bounds.Width)
		right = min(right, slideWidth)
		return left, right, right > left
	}
	return 0, 0, false
}

// titleBottom returns the bottom edge of the layout's title placeholder when
// the title lives in the upper half of the slide.
func titleBottom(layout *types.LayoutMetadata, slideHeight int64) (int64, bool) {
	if layout == nil {
		return 0, false
	}
	for i := range layout.Placeholders {
		ph := &layout.Placeholders[i]
		if ph.Type == types.PlaceholderTitle && ph.Bounds.Height > 0 && ph.Bounds.Y < slideHeight/2 {
			return ph.Bounds.Y + ph.Bounds.Height, true
		}
	}
	return 0, false
}

// ChromeReferenceLayout returns the template's One Content layout — the
// reference geometry for chrome on layouts that carry no body placeholder.
// Selection mirrors BuildProfile's role binding: highest canonical confidence,
// ties broken by layout ID. Returns nil when the template has none.
func ChromeReferenceLayout(layouts []types.LayoutMetadata) *types.LayoutMetadata {
	var candidates []int
	for i := range layouts {
		if EffectiveCanonicalType(&layouts[i]) == types.CanonicalLayoutOneContent {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := &layouts[candidates[i]], &layouts[candidates[j]]
		if a.CanonicalConfidence == b.CanonicalConfidence {
			return a.ID < b.ID
		}
		return a.CanonicalConfidence > b.CanonicalConfidence
	})
	return &layouts[candidates[0]]
}

// FindLayout returns the layout with the given ID, or nil.
func FindLayout(layouts []types.LayoutMetadata, id string) *types.LayoutMetadata {
	for i := range layouts {
		if layouts[i].ID == id {
			return &layouts[i]
		}
	}
	return nil
}
