package generator

import (
	"archive/zip"
	"image/color"
	"sort"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
	"github.com/tdewolff/canvas"
)

// Footer line fitting by priority (go-slide-creator-m2tlt).
//
// The left footer line is one string composed from the chrome fields. It used
// to be shrunk to 8pt and then ellipsized from the right, so on a template
// with a narrow footer slot the client name lost its tail and the date
// vanished, on every slide, with no finding. The line is now fitted in three
// steps, and anything short of the full line is reported as CHROME_TRUNCATED:
//
//  1. the full line, at the largest footer size that fits;
//  2. the caller's fallbacks in order (project_code dropped, then footer_date);
//  3. the last fallback ellipsized, where even that is too wide.
//
// The step is chosen per footer slot, not per slide: every slide whose slot
// has the same width carries the same fields, so the content slides of a deck
// read alike even when a section crumb makes some lines longer than others,
// while a section divider with a narrower slot does not cost every other
// slide its project code.

// defaultFooterTextPath is the finding path of a footer line that did not come
// from the chrome block.
const defaultFooterTextPath = "/footer/left_text"

// footerLineBox is the box one slide's left footer text is drawn into.
type footerLineBox struct {
	slideIndex int   // 0-based
	widthEMU   int64 // box width, insets included
}

// footerLineOutcome is one way the line fell short of the full line, and the
// slides it happened on.
type footerLineOutcome struct {
	// level is how many fallbacks were applied (LeftFallbacks[level-1]); 0 is
	// the full line, which is then necessarily truncated.
	level     int
	truncated bool
	slides    []int // 0-based, ascending
	// line is the full line of the tightest of those slides and maxChars how
	// many of its characters fit at the footer floor.
	line     string
	maxChars int
}

// footerLineFit is the outcome of fitting the left footer line to a deck.
type footerLineFit struct {
	// levels is the fallback level of each slide that does not carry the full
	// line, by 0-based slide index.
	levels map[int]int
	// outcomes are the distinct shortfalls, in slide order. Empty when the
	// full line fits everywhere.
	outcomes []footerLineOutcome
}

// leftTextAt returns a slide's left footer line at a fallback level.
func (c *FooterConfig) leftTextAt(level, slideIndex int) string {
	if c == nil {
		return ""
	}
	if level <= 0 || level > len(c.LeftFallbacks) {
		return c.LeftTextFor(slideIndex)
	}
	fb := c.LeftFallbacks[level-1]
	if slideIndex >= 0 && slideIndex < len(fb.LeftTextBySlide) {
		if t := fb.LeftTextBySlide[slideIndex]; t != "" {
			return t
		}
	}
	return fb.LeftText
}

// withLeftLevels returns the config a deck renders with once the fitter has
// chosen a fallback level per slide: a copy whose per-slide lines are the
// chosen ones. c is returned unchanged when no slide left the full line.
func (c *FooterConfig) withLeftLevels(levels map[int]int) *FooterConfig {
	if c == nil || len(levels) == 0 {
		return c
	}
	n := len(c.LeftTextBySlide)
	for i := range levels {
		if i+1 > n {
			n = i + 1
		}
	}
	lines := make([]string, n)
	copy(lines, c.LeftTextBySlide)
	for i, level := range levels {
		lines[i] = c.leftTextAt(level, i)
	}
	out := *c
	out.LeftTextBySlide = lines
	out.LeftFallbacks = nil
	return &out
}

// footerLineMeasure reports whether text fits one line of a footer box at a
// size (hundredths of a point). ok is false when the body font has no metrics,
// in which case nothing can be decided and the line is left alone.
func footerLineMeasure(fontName string) (fits func(text string, widthEMU int64, size int) bool, ok bool) {
	ff, _, _ := fontcache.Resolve(fontName, "Arial")
	if ff == nil {
		return nil, false
	}
	return func(text string, widthEMU int64, size int) bool {
		// Usable width: the box minus its 0.1in left/right insets, in mm
		// (canvas text-line bounds are millimetres; the face size is points).
		usableMM := float64(widthEMU-2*91440) / 36000
		face := ff.Face(float64(size)/100, color.Black, canvas.FontRegular, canvas.FontNormal)
		return textfit.LineWidthMM(face, text) <= usableMM
	}, true
}

// resolveFooterLine picks the fallback level of every footer slot and records
// what did not fit. boxes are the slides that carry the footer; a slide with
// no text at a level fits it by definition.
func resolveFooterLine(cfg *FooterConfig, boxes []footerLineBox, fontName string) footerLineFit {
	var fit footerLineFit
	if cfg == nil || len(boxes) == 0 {
		return fit
	}
	fits, ok := footerLineMeasure(fontName)
	if !ok {
		return fit
	}
	last := len(cfg.LeftFallbacks)
	fitsAt := func(level int, b footerLineBox) bool {
		text := cfg.leftTextAt(level, b.slideIndex)
		return text == "" || fits(text, b.widthEMU, footerMinFontSize)
	}

	// The level of a slot is the highest any of its slides needs. Its budget
	// is that of its tightest slide: the one the fewest characters of the full
	// line fit on.
	type slotBudget struct {
		line     string
		maxChars int
	}
	slotLevel := map[int64]int{}
	slotTight := map[int64]slotBudget{}
	for _, b := range boxes {
		if fitsAt(0, b) {
			continue
		}
		level := 1
		for level < last && !fitsAt(level, b) {
			level++
		}
		if level > last {
			level = last // nothing to drop: the line is ellipsized
		}
		if level > slotLevel[b.widthEMU] {
			slotLevel[b.widthEMU] = level
		}
		full := cfg.leftTextAt(0, b.slideIndex)
		n := fittingRunes(full, b.widthEMU, fits)
		if tight, seen := slotTight[b.widthEMU]; !seen || n < tight.maxChars {
			slotTight[b.widthEMU] = slotBudget{line: full, maxChars: n}
		}
	}

	type key struct {
		level     int
		truncated bool
	}
	byKey := map[key]*footerLineOutcome{}
	var order []key
	sorted := append([]footerLineBox(nil), boxes...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].slideIndex < sorted[j].slideIndex })
	for _, b := range sorted {
		level := slotLevel[b.widthEMU]
		truncated := !fitsAt(level, b)
		if level == 0 && !truncated {
			continue
		}
		if level > 0 {
			if fit.levels == nil {
				fit.levels = map[int]int{}
			}
			fit.levels[b.slideIndex] = level
		}
		k := key{level, truncated}
		o := byKey[k]
		if o == nil {
			o = &footerLineOutcome{level: level, truncated: truncated, maxChars: -1}
			byKey[k] = o
			order = append(order, k)
		}
		o.slides = append(o.slides, b.slideIndex)
		if tight := slotTight[b.widthEMU]; o.maxChars < 0 || tight.maxChars < o.maxChars {
			o.line, o.maxChars = tight.line, tight.maxChars
		}
	}
	for _, k := range order {
		fit.outcomes = append(fit.outcomes, *byKey[k])
	}
	return fit
}

// fittingRunes is how many leading characters of text fit one line of a box
// at the footer floor.
func fittingRunes(text string, widthEMU int64, fits func(string, int64, int) bool) int {
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fits(string(runes[:mid]), widthEMU, footerMinFontSize) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// findings builds one CHROME_TRUNCATED finding per outcome; nil when the full
// line fits every slide.
func (fit footerLineFit) findings(cfg *FooterConfig) []patterns.FitFinding {
	if len(fit.outcomes) == 0 {
		return nil
	}
	path := cfg.LeftTextPath
	if path == "" {
		path = defaultFooterTextPath
	}
	out := make([]patterns.FitFinding, 0, len(fit.outcomes))
	for _, o := range fit.outcomes {
		t := patterns.ChromeTruncation{
			Line:      o.line,
			LineChars: utf8.RuneCountInString(o.line),
			MaxChars:  o.maxChars,
			Truncated: o.truncated,
			Slides:    make([]int, len(o.slides)),
		}
		for i, s := range o.slides {
			t.Slides[i] = s + 1
		}
		if o.level > 0 {
			t.Dropped = append([]string(nil), cfg.LeftFallbacks[o.level-1].Dropped...)
		}
		out = append(out, patterns.ChromeTruncated(path, t))
	}
	return out
}

// footerSlideLayout is the resolved footer geometry of one slide: the left
// text box (nil when the slide has no room or no anchor for it) and the page
// number, sized first because its box grows leftward into the text's.
type footerSlideLayout struct {
	box     *transformXML
	pageNum *pageNumberSizing
	label   string
}

// layoutFooterSlide resolves a slide's footer geometry. It is the one place
// the left footer box is derived, so the fitter measures the box the text is
// drawn into.
func layoutFooterSlide(positions map[string]*transformXML, config *FooterConfig, fontName string, slideIndex int, obstacles []footerObstacle, slideWidth int64) footerSlideLayout {
	var out footerSlideLayout
	// Size the page-number box first: it grows leftward from a fixed right edge,
	// so the left footer has to be laid out against the widened box or the two
	// overlap (go-slide-creator-pss1z).
	if l := config.PageLabelFor(slideIndex); l != "" {
		out.label = literalPageNumberText(config.PageNumberFormat, l)
	}
	switch {
	case config.pageNumberHiddenFor(slideIndex):
	case out.label != "":
		out.pageNum = resolvePageNumberSizingForText(positions, out.label, fontName)
	default:
		out.pageNum = resolvePageNumberSizing(positions, config.PageNumberFormat, config.TotalSlides, fontName)
	}
	if out.pageNum != nil && len(obstacles) > 0 {
		out.pageNum.box = clearPageNumberBox(out.pageNum.box, obstacles, pageNumberLeftLimit(positions), slideWidth)
		if out.pageNum.box == nil {
			out.pageNum = nil
		}
	}
	layout := positions
	if out.pageNum != nil {
		layout = withSldNum(positions, out.pageNum.box)
	} else if len(obstacles) > 0 {
		layout = withSldNum(positions, nil)
	}

	// Left footer (dt position, widened across ftr).
	out.box = leftFooterBox(layout)
	if len(obstacles) > 0 {
		out.box = clearLeftFooterBox(out.box, obstacles)
	}
	return out
}

// resolveFooterLineForDeck fits the left footer line against every slide that
// carries the footer, switches each footer slot to the fallback that fits and
// reports CHROME_TRUNCATED where that is not the full line. It runs once,
// before the first slide is written.
//
// A slide whose chrome is later suppressed because a picture covers the footer
// band is still measured here: pictures are placed while slides are written,
// and counting such a slide can only make its slot-mates' line shorter.
func (ctx *singlePassContext) resolveFooterLineForDeck() {
	if ctx.footerLineResolved {
		return
	}
	ctx.footerLineResolved = true
	cfg := ctx.footerConfig
	if cfg == nil || !cfg.Enabled {
		return
	}
	var boxes []footerLineBox
	for i, spec := range ctx.slideSpecs {
		if spec.SkipFooter || cfg.LeftTextFor(i) == "" {
			continue
		}
		positions := alignFooterWithInsetContent(ctx.getFooterPositionsForLayout(spec.LayoutID), ctx.chromeFrameForLayout(spec.LayoutID, false, false))
		if len(positions) == 0 {
			continue
		}
		lay := layoutFooterSlide(positions, cfg, ctx.themeFontName, i, ctx.footerObstaclesForLayout(spec.LayoutID), ctx.slideWidth)
		if lay.box == nil {
			continue
		}
		boxes = append(boxes, footerLineBox{slideIndex: i, widthEMU: lay.box.Extent.CX})
	}
	fit := resolveFooterLine(cfg, boxes, ctx.themeFontName)
	for _, f := range fit.findings(cfg) {
		ctx.emitFitFinding(f)
	}
	ctx.footerConfig = cfg.withLeftLevels(fit.levels)
}

// FooterLineSlide is one slide's footer geometry as preflight knows it: its
// layout, the layout's resolved dt / ftr / sldNum regions and its chrome frame.
type FooterLineSlide struct {
	SlideIndex int // 0-based
	LayoutID   string
	Regions    []types.ChromeRegion
	Frame      template.ChromeFrame
}

// FooterLineInput is what PredictFooterTruncation fits.
type FooterLineInput struct {
	Config *FooterConfig
	// Slides are the slides that carry the footer (title / closing skips
	// already removed).
	Slides   []FooterLineSlide
	FontName string // theme body font
	// SlideWidth / SlideHeight are the slide size in EMU.
	SlideWidth, SlideHeight int64
	// TemplatePath, when it names a readable template, makes the prediction
	// exact: the footer slots are resolved from the file the way generation
	// resolves them (master placeholders first, narrowed around footer
	// artwork). Without it, or for a layout the file does not hold (a
	// synthesized one), the slot comes from the layout's Regions.
	TemplatePath string
}

// PredictFooterTruncation is the preflight side of the footer line fitter: it
// runs the fit generation runs and returns the CHROME_TRUNCATED findings
// generation would emit, or nil.
func PredictFooterTruncation(in FooterLineInput) []patterns.FitFinding {
	cfg := in.Config
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	probe, closeProbe := openFooterProbe(in.TemplatePath, in.SlideWidth, in.SlideHeight)
	defer closeProbe()
	var boxes []footerLineBox
	for _, s := range in.Slides {
		if cfg.LeftTextFor(s.SlideIndex) == "" {
			continue
		}
		var positions map[string]*transformXML
		var obstacles []footerObstacle
		if probe.holdsLayout(s.LayoutID) {
			positions = probe.getFooterPositionsForLayout(s.LayoutID)
			obstacles = probe.footerObstaclesForLayout(s.LayoutID)
		} else {
			raw := make(map[string]*transformXML, len(s.Regions))
			for _, r := range s.Regions {
				raw["type:"+r.Type] = &transformXML{
					Offset: offsetXML{X: r.X, Y: r.Y},
					Extent: extentXML{CX: r.Width, CY: r.Height},
				}
			}
			positions = resolveFooterPositionsOnSlide(raw, in.SlideWidth, in.SlideHeight)
		}
		positions = alignFooterWithInsetContent(positions, s.Frame)
		lay := layoutFooterSlide(positions, cfg, in.FontName, s.SlideIndex, obstacles, in.SlideWidth)
		if lay.box == nil {
			continue
		}
		boxes = append(boxes, footerLineBox{slideIndex: s.SlideIndex, widthEMU: lay.box.Extent.CX})
	}
	return resolveFooterLine(cfg, boxes, in.FontName).findings(cfg)
}

// openFooterProbe opens a template read-only as the minimal generation
// context the footer geometry resolvers need, so preflight measures the slot
// generation draws into. A template that cannot be opened yields an empty
// probe that holds no layout.
func openFooterProbe(templatePath string, slideWidth, slideHeight int64) (*singlePassContext, func()) {
	probe := &singlePassContext{}
	probe.slideWidth, probe.slideHeight = slideWidth, slideHeight
	probe.footerPositionsByLayout = make(map[string]map[string]*transformXML)
	zr, err := zip.OpenReader(templatePath)
	if err != nil {
		return probe, func() {}
	}
	if utils.CheckZipLimits(zr.File) != nil {
		_ = zr.Close()
		return probe, func() {}
	}
	probe.templateIndex = utils.BuildZipIndex(&zr.Reader)
	return probe, func() { _ = zr.Close() }
}

// holdsLayout reports whether the opened template file contains the layout.
func (ctx *singlePassContext) holdsLayout(layoutID string) bool {
	if ctx == nil || ctx.templateIndex == nil || layoutID == "" {
		return false
	}
	_, ok := ctx.templateIndex[LayoutPath(layoutID)]
	return ok
}
