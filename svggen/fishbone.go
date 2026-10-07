package svggen

import (
	"fmt"
	"math"
	"strings"
)

// =============================================================================
// Fishbone (Ishikawa) Diagram
// =============================================================================

// FishboneConfig holds configuration for fishbone diagrams.
type FishboneConfig struct {
	ChartConfig

	// BranchAngle is the lean of a category bone from the vertical, in
	// degrees. A bone never leans further than its column allows.
	BranchAngle float64

	// SpineWidth is the stroke width of the main spine.
	SpineWidth float64

	// BranchWidth is the stroke width of branches.
	BranchWidth float64

	// CornerRadius is kept for callers that set it; category heads and the
	// effect are square-cornered shapes and ignore it.
	CornerRadius float64

	// MaxVisibleCategories is the maximum number of categories rendered
	// before collapsing the rest into an overflow indicator. Zero means
	// no limit; the default is 10.
	MaxVisibleCategories int
}

// DefaultFishboneConfig returns default fishbone configuration.
func DefaultFishboneConfig(width, height float64) FishboneConfig {
	return FishboneConfig{
		ChartConfig:          DefaultChartConfig(width, height),
		BranchAngle:          30,
		SpineWidth:           4,
		BranchWidth:          2,
		CornerRadius:         0,
		MaxVisibleCategories: 10,
	}
}

// FishboneData represents the data for a fishbone diagram.
type FishboneData struct {
	Title    string
	Subtitle string
	// Effect is the main issue/effect at the head of the fish.
	Effect string
	// Categories are the cause categories branching off the spine.
	Categories []FishboneCategory
}

// FishboneCategory represents a cause category on the fishbone.
type FishboneCategory struct {
	// Name is the category name (e.g., "People", "Process", "Materials").
	Name string
	// Causes are individual causes within this category.
	Causes []string
}

// FishboneChart renders fishbone/Ishikawa diagrams.
type FishboneChart struct {
	builder *SVGBuilder
	config  FishboneConfig
}

// NewFishboneChart creates a new fishbone chart renderer.
func NewFishboneChart(builder *SVGBuilder, config FishboneConfig) *FishboneChart {
	return &FishboneChart{
		builder: builder,
		config:  config,
	}
}

// The fishbone's proportions, in multiples of the cause type size unless
// noted. The drawing is sized from the canvas it is handed: the bones run
// from the spine to category heads on the top and bottom edges, so the causes
// have the whole half-height to stand in (go-slide-creator-o8cqh).
const (
	fishboneLineFactor      = 1.3  // line pitch of cause and head text
	fishboneTabPadX         = 0.75 // head padding left and right
	fishboneTabPadY         = 0.4  // head padding above and below
	fishboneTickLen         = 0.9  // the small bone a cause hangs from
	fishboneLabelGap        = 0.35 // tick end to cause text
	fishboneMinLabelChars   = 4.0  // narrowest cause column worth drawing
	fishboneMaxCauseLines   = 3
	fishboneMaxBoneLeanFrac = 0.30 // of the column width
	fishboneEffectMinFrac   = 0.14 // effect pentagon width, share of the plot
	fishboneEffectMaxFrac   = 0.30
	fishboneEdgePad         = 0.25 // canvas edge to the drawing
	fishboneLoneJoinFrac    = 0.62 // where a single column's bones meet the spine
)

// fishboneBone is the resolved geometry of one category: its head on the top
// or bottom edge, the bone from the spine to the head, and the rows its
// causes stand in.
type fishboneBone struct {
	catIndex int
	isTop    bool
	// joinX is where the bone meets the spine; headX, headY where it meets
	// the head's edge facing the spine.
	joinX, headX, headY float64
	// tab is the category head; tabLines its label, already wrapped.
	tab      Rect
	tabLines []string
	// slotLeft is the left limit of the bone's cause labels.
	slotLeft float64
	// causes are the rows drawn; hidden counts the causes that did not fit.
	causes []fishboneCauseRow
	hidden int
	// countOnly is set when the column is too narrow for any cause label:
	// the head then carries the count.
	countOnly bool
}

// fishboneCauseRow is one cause label: lines right-aligned at labelRight and
// centred on y, with its tick running from tickX to the bone at boneX.
type fishboneCauseRow struct {
	lines      []string
	y          float64
	boneX      float64
	tickX      float64
	labelRight float64
	overflow   bool // the "+N more" line, drawn muted and without a tick
}

// boneXAt is the bone's x at height y, on the line from the spine join to
// the head.
func (bn fishboneBone) boneXAt(y, spineY float64) float64 {
	span := bn.headY - spineY
	if span == 0 {
		return bn.joinX
	}
	return bn.joinX + (bn.headX-bn.joinX)*(y-spineY)/span
}

// fishboneFit is the type size and line pitch shared by every bone.
type fishboneFit struct {
	font  float64 // cause and head text
	lineH float64
}

// Draw renders the fishbone diagram.
func (fc *FishboneChart) Draw(data FishboneData) error {
	if data.Effect == "" {
		return fmt.Errorf("fishbone diagram requires an 'effect'")
	}

	b := fc.builder
	style := b.StyleGuide()
	typ := style.Typography

	font := math.Max(typ.SizeSmall, b.MinFontSize())
	fit := fishboneFit{font: font, lineH: font * fishboneLineFactor}

	// The drawing runs to the edges of its canvas: the page margin is the
	// placeholder's, and a second one inside it left the fish adrift.
	edge := font * fishboneEdgePad
	plot := Rect{X: edge, Y: edge, W: math.Max(0, fc.config.Width-2*edge), H: math.Max(0, fc.config.Height-2*edge)}
	headerHeight := 0.0
	if fc.config.ShowTitle && data.Title != "" {
		headerHeight = typ.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += typ.SizeSubtitle + style.Spacing.XS
		}
	}
	if headerHeight > 0 {
		top := fc.config.MarginTop + headerHeight
		plot.H = math.Max(0, plot.H-(top-plot.Y))
		plot.Y = top
	}
	spineY := plot.Y + plot.H/2

	// The effect: a solid accent pentagon whose point closes the fish.
	effect := fc.layoutEffect(data.Effect, plot, spineY, fit)

	spineWidth := math.Max(fc.config.SpineWidth, font*0.22)
	neutral := NeutralInk(style.Palette, tonalRuleShare)
	b.Push()
	b.SetStrokeColor(NeutralInk(style.Palette, tonalSpineShare))
	b.SetStrokeWidth(spineWidth)
	b.DrawLine(plot.X, spineY, effect.rect.X, spineY)
	b.Pop()
	fc.drawEffect(effect)

	visible := data.Categories
	overflow := 0
	if limit := fc.config.MaxVisibleCategories; limit > 0 && len(visible) > limit {
		overflow = len(visible) - limit
		visible = visible[:limit]
	}

	bones := fc.layoutBones(visible, plot, spineY, effect.rect.X, fit)
	for _, bn := range bones {
		fc.drawBone(bn, visible[bn.catIndex], spineY, neutral, fit)
	}
	fc.reportHiddenCauses(bones, visible)

	if overflow > 0 {
		b.Push()
		b.SetFontSize(font)
		b.SetFillColor(diagramMutedInk(style))
		b.DrawText(fmt.Sprintf("+%d more categories", overflow), plot.X, spineY+spineWidth+font*0.4, TextAlignLeft, TextBaselineTop)
		b.Pop()
		b.AddFinding(Finding{
			Field:    "categories",
			Code:     FindingDiagramItemsDropped,
			Severity: "warning",
			Message: fmt.Sprintf("fishbone: %d of %d categories are not drawn — a fishbone holds %d; merge categories or split the analysis over two slides",
				overflow, len(data.Categories), len(visible)),
			Fix: &FixSuggestion{Kind: FixKindReduceItems, Params: map[string]any{
				"dropped_count": overflow, "total_count": len(data.Categories), "rendered": len(visible), "diagram_type": "fishbone",
			}},
		})
	}

	if fc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: fc.config.Width, H: headerHeight + fc.config.MarginTop})
	}
	return nil
}

// fishboneEffect is the laid-out head of the fish.
type fishboneEffect struct {
	rect  Rect    // the pentagon's bounding box
	point float64 // depth of its point
	lines []string
	font  float64
}

// layoutEffect sizes the effect pentagon to its text: as narrow as the words
// allow between fishboneEffectMinFrac and fishboneEffectMaxFrac of the plot,
// wrapped, bold, at the cause size or one step above it.
func (fc *FishboneChart) layoutEffect(text string, plot Rect, spineY float64, fit fishboneFit) fishboneEffect {
	b := fc.builder
	style := b.StyleGuide()
	font := math.Max(fit.font, math.Min(style.Typography.SizeBody, fit.font*1.2))
	lineH := font * fishboneLineFactor
	pad := font * fishboneTabPadX
	minW, maxW := plot.W*fishboneEffectMinFrac, plot.W*fishboneEffectMaxFrac

	b.Push()
	defer b.Pop()
	b.SetFontSize(font)
	b.SetFontWeight(style.Typography.WeightBold)

	var out fishboneEffect
	for w := minW; ; w += plot.W * 0.02 {
		w = math.Min(w, maxW)
		h := 0.0
		var lines []string
		// The point takes a share of the box; iterate once so the text
		// column is measured beside the point it leaves.
		point := font
		for range 2 {
			lines = fishboneWrap(b, text, w-point-2*pad)
			h = math.Max(float64(len(lines))*lineH+2*pad, 2.6*font)
			point = math.Min(h*0.35, w*0.3)
		}
		out = fishboneEffect{
			rect:  Rect{X: plot.X + plot.W - w, Y: spineY - h/2, W: w, H: h},
			point: point, lines: lines, font: font,
		}
		widest := 0.0
		for _, line := range lines {
			lw, _ := b.MeasureText(line)
			widest = math.Max(widest, lw)
		}
		if (widest <= w-point-2*pad && h <= plot.H*0.5) || w >= maxW {
			break
		}
	}
	textW := out.rect.W - out.point - 2*pad
	if maxLines := int((plot.H*0.8 - 2*pad) / lineH); len(out.lines) > maxLines && maxLines >= 1 {
		out.lines = out.lines[:maxLines]
		out.lines[maxLines-1] += "…"
		out.rect.H = float64(maxLines)*lineH + 2*pad
		out.rect.Y = spineY - out.rect.H/2
	}
	for i, line := range out.lines {
		if lw, _ := b.MeasureText(line); lw > textW {
			out.lines[i] = b.TruncateToWidth(line, textW)
		}
	}
	return out
}

// drawEffect paints the effect pentagon and its label.
func (fc *FishboneChart) drawEffect(e fishboneEffect) {
	b := fc.builder
	style := b.StyleGuide()
	fill := diagramAccent(style)
	r := e.rect
	b.Push()
	b.SetFillColor(fill)
	b.SetStrokeWidth(0)
	b.DrawPolygon([]Point{
		{X: r.X, Y: r.Y},
		{X: r.X + r.W - e.point, Y: r.Y},
		{X: r.X + r.W, Y: r.Y + r.H/2},
		{X: r.X + r.W - e.point, Y: r.Y + r.H},
		{X: r.X, Y: r.Y + r.H},
	})
	b.Pop()

	b.Push()
	b.SetFontSize(e.font)
	b.SetFontWeight(style.Typography.WeightBold)
	b.SetFillColor(diagramInkOn(style, fill))
	lineH := e.font * fishboneLineFactor
	cx := r.X + (r.W-e.point/2)/2
	top := r.Y + r.H/2 - float64(len(e.lines))*lineH/2
	for i, line := range e.lines {
		b.DrawText(line, cx, top+(float64(i)+0.5)*lineH, TextAlignCenter, TextBaselineMiddle)
	}
	b.Pop()
}

// layoutBones places the categories in columns of two (one bone above the
// spine, one below, meeting it at one point) and fits each bone's causes.
func (fc *FishboneChart) layoutBones(cats []FishboneCategory, plot Rect, spineY, effectX float64, fit fishboneFit) []fishboneBone {
	n := len(cats)
	if n == 0 {
		return nil
	}
	b := fc.builder
	cols := (n + 1) / 2
	// The last bone meets the spine a little before the effect.
	usable := effectX - fit.font - plot.X
	slotW := usable / float64(cols)
	angle := fc.config.BranchAngle * math.Pi / 180
	padX := fit.font * fishboneTabPadX

	bones := make([]fishboneBone, n)
	for i, cat := range cats {
		col := i / 2
		bn := fishboneBone{catIndex: i, isTop: i%2 == 0}
		slotX := plot.X + float64(col)*slotW
		bn.slotLeft = slotX
		bn.joinX = slotX + slotW
		if cols == 1 {
			// One or two categories: the bones sit over the middle of the
			// spine, not against the effect.
			bn.joinX = slotX + slotW*fishboneLoneJoinFrac
		}
		if col > 0 {
			// Clear of the previous column's bones and their join.
			bn.slotLeft += fit.font * 0.6
		}

		// Head: the category name, bold, on a small filled tab at the edge.
		b.Push()
		b.SetFontSize(fit.font)
		b.SetFontWeight(b.StyleGuide().Typography.WeightBold)
		maxTextW := slotW*0.92 - 2*padX
		lines := fishboneWrap(b, cat.Name, maxTextW)
		if len(lines) > 2 {
			lines = lines[:2]
			lines[1] += "…"
		}
		textW := 0.0
		for li, line := range lines {
			if lw, _ := b.MeasureText(line); lw > maxTextW {
				lines[li] = b.TruncateToWidth(line, maxTextW)
			}
			lw, _ := b.MeasureText(lines[li])
			textW = math.Max(textW, lw)
		}
		b.Pop()
		tabW := textW + 2*padX
		tabH := float64(max(len(lines), 1))*fit.lineH + 2*fit.font*fishboneTabPadY

		// The bone leans back from the join by its angle, but never out of
		// its column.
		var span float64
		if bn.isTop {
			bn.headY = plot.Y + tabH
			span = spineY - bn.headY
		} else {
			bn.headY = plot.Y + plot.H - tabH
			span = bn.headY - spineY
		}
		lean := math.Min(math.Max(span, 0)*math.Tan(angle), slotW*fishboneMaxBoneLeanFrac)
		bn.headX = bn.joinX - lean

		tabX := math.Min(math.Max(bn.headX-tabW/2, slotX), bn.joinX-tabW)
		tabY := plot.Y
		if !bn.isTop {
			tabY = plot.Y + plot.H - tabH
		}
		bn.tab = Rect{X: tabX, Y: tabY, W: tabW, H: tabH}
		bn.tabLines = lines

		fc.fitCauses(&bn, cat.Causes, spineY, fit)
		bones[i] = bn
	}
	return bones
}

// fitCauses places a bone's causes in equal rows between its head and the
// spine, in reading order from the top. Every cause that has a line of room
// is drawn, wrapped to the lines its row holds; only when the rows run out
// does the last row become "+N more" (which then stands for at least two
// causes: one hidden cause is never traded for an indicator of the same
// height).
func (fc *FishboneChart) fitCauses(bn *fishboneBone, causes []string, spineY float64, fit fishboneFit) {
	if len(causes) == 0 {
		return
	}
	b := fc.builder
	gap := fit.font * 0.5
	top, bottom := bn.headY+gap, spineY-gap
	if !bn.isTop {
		top, bottom = spineY+gap, bn.headY-gap
	}
	region := bottom - top
	capacity := int(region / fit.lineH)

	tick := fit.font * fishboneTickLen
	labelGap := fit.font * fishboneLabelGap
	// The narrowest row is the one nearest the head.
	narrowest := math.Min(bn.headX, bn.joinX) - tick - labelGap - bn.slotLeft
	if capacity < 1 || narrowest < fit.font*fishboneMinLabelChars {
		bn.countOnly = true
		bn.hidden = len(causes)
		return
	}

	shown := len(causes)
	if shown > capacity {
		shown = capacity - 1
		bn.hidden = len(causes) - shown
	}
	rows := shown
	if bn.hidden > 0 {
		rows++
	}
	rowH := region / float64(rows)
	maxLines := min(max(int(rowH/fit.lineH), 1), fishboneMaxCauseLines)

	b.Push()
	defer b.Pop()
	b.SetFontSize(fit.font)
	b.SetFontWeight(b.StyleGuide().Typography.WeightNormal)
	for r := 0; r < rows; r++ {
		y := top + (float64(r)+0.5)*rowH
		row := fishboneCauseRow{y: y, boneX: bn.boneXAt(y, spineY)}
		row.tickX = row.boneX - tick
		row.labelRight = row.tickX - labelGap
		width := row.labelRight - bn.slotLeft
		var text string
		if r < shown {
			text = causes[r]
		} else {
			text = fmt.Sprintf("+%d more", bn.hidden)
			row.overflow = true
		}
		lines := fishboneWrap(b, text, width)
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines[maxLines-1] += "…"
		}
		for li, line := range lines {
			if lw, _ := b.MeasureText(line); lw > width {
				lines[li] = b.TruncateToWidth(line, width)
			}
		}
		if !row.overflow && strings.Join(lines, " ") != strings.Join(strings.Fields(text), " ") {
			b.AddFinding(Finding{
				Field:    "categories",
				Code:     FindingLabelTruncated,
				Severity: "info",
				Message:  fmt.Sprintf("fishbone: cause %q is cut to fit its bone — shorten it or use fewer categories", text),
				Fix: &FixSuggestion{Kind: FixKindTruncateOrSplit, Params: map[string]any{
					"original": text, "truncated": strings.Join(lines, " "), "font_size": fit.font,
				}},
			})
		}
		row.lines = lines
		bn.causes = append(bn.causes, row)
	}
}

// drawBone paints one category: bone, head, cause ticks and labels.
func (fc *FishboneChart) drawBone(bn fishboneBone, cat FishboneCategory, spineY float64, neutral Color, fit fishboneFit) {
	b := fc.builder
	style := b.StyleGuide()

	b.Push()
	b.SetStrokeColor(neutral)
	b.SetStrokeWidth(fc.config.BranchWidth)
	b.DrawLine(bn.joinX, spineY, bn.headX, bn.headY)
	b.SetStrokeWidth(math.Max(1, fc.config.BranchWidth/2))
	for _, row := range bn.causes {
		if !row.overflow {
			b.DrawLine(row.boneX, row.y, row.tickX, row.y)
		}
	}
	b.Pop()

	fill := diagramContentFill(style)
	b.Push()
	b.SetFillColor(fill)
	b.SetStrokeWidth(0)
	b.DrawRect(bn.tab)
	b.Pop()

	b.Push()
	b.SetFontSize(fit.font)
	b.SetFontWeight(style.Typography.WeightBold)
	b.SetFillColor(diagramInkOn(style, fill))
	top := bn.tab.Y + bn.tab.H/2 - float64(len(bn.tabLines))*fit.lineH/2
	for i, line := range bn.tabLines {
		b.DrawText(line, bn.tab.X+bn.tab.W/2, top+(float64(i)+0.5)*fit.lineH, TextAlignCenter, TextBaselineMiddle)
	}
	b.Pop()

	b.Push()
	b.SetFontSize(fit.font)
	b.SetFontWeight(style.Typography.WeightNormal)
	for _, row := range bn.causes {
		if row.overflow {
			b.SetFillColor(diagramMutedInk(style))
		} else {
			b.SetFillColor(style.Palette.TextPrimary)
		}
		rowTop := row.y - float64(len(row.lines))*fit.lineH/2
		for i, line := range row.lines {
			b.DrawText(line, row.labelRight, rowTop+(float64(i)+0.5)*fit.lineH, TextAlignRight, TextBaselineMiddle)
		}
	}
	if bn.countOnly && len(cat.Causes) > 0 {
		// Too narrow for any cause: the head says how many there are.
		y := bn.tab.Y + bn.tab.H + fit.lineH*0.6
		if !bn.isTop {
			y = bn.tab.Y - fit.lineH*0.6
		}
		b.SetFillColor(diagramMutedInk(style))
		b.DrawText(fmt.Sprintf("(%d causes)", len(cat.Causes)), bn.tab.X+bn.tab.W/2, y, TextAlignCenter, TextBaselineMiddle)
	}
	b.Pop()
}

// reportHiddenCauses says which bones could not hold all their causes: the
// picture shows "+N more", and the finding is what an author can act on.
func (fc *FishboneChart) reportHiddenCauses(bones []fishboneBone, cats []FishboneCategory) {
	for _, bn := range bones {
		if bn.hidden == 0 {
			continue
		}
		cat := cats[bn.catIndex]
		fc.builder.AddFinding(Finding{
			Field:    "categories",
			Code:     FindingDiagramItemsDropped,
			Severity: "warning",
			Message: fmt.Sprintf("fishbone: %d of %d causes of %q did not fit its bone and are not drawn — list fewer causes, use fewer categories, or give the diagram a taller box",
				bn.hidden, len(cat.Causes), cat.Name),
			Fix: &FixSuggestion{Kind: FixKindReduceItems, Params: map[string]any{
				"dropped_count": bn.hidden, "total_count": len(cat.Causes), "rendered": len(cat.Causes) - bn.hidden,
				"category": cat.Name, "diagram_type": "fishbone",
			}},
		})
	}
}

// fishboneWrap breaks text into lines no wider than width at the builder's
// current font, at spaces only; a single word wider than width stays whole
// for the caller to cut.
func fishboneWrap(b *SVGBuilder, text string, width float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		candidate := lines[len(lines)-1] + " " + word
		if w, _ := b.MeasureText(candidate); w <= width {
			lines[len(lines)-1] = candidate
		} else {
			lines = append(lines, word)
		}
	}
	return lines
}

// =============================================================================
// Fishbone Diagram Interface
// =============================================================================

// FishboneDiagram implements the Diagram interface for fishbone diagrams.
type FishboneDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a fishbone diagram.
func (d *FishboneDiagram) Validate(req *RequestEnvelope) error {
	data := req.Data

	// Normalize: accept "problem" as alias for "effect" (common in JSON input)
	if _, hasEffect := data["effect"]; !hasEffect {
		if problem, hasProblem := data["problem"]; hasProblem {
			data["effect"] = problem
		} else {
			return fmt.Errorf("fishbone requires 'effect' (or 'problem') field. Expected format: {\"effect\": \"Low Sales\", \"categories\": [{\"name\": \"People\", \"causes\": [\"Understaffed\"]}]}")
		}
	}

	return nil
}

// Render generates an SVG document for the fishbone diagram.
func (d *FishboneDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *FishboneDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		fbData, err := extractFishboneData(req)
		if err != nil {
			return err
		}

		assumeSlidePlacement(builder, req)
		width, height := builder.Width(), builder.Height()
		config := DefaultFishboneConfig(width, height)
		config.ShowTitle = req.Title != ""

		chart := NewFishboneChart(builder, config)
		if err := chart.Draw(fbData); err != nil {
			return fmt.Errorf("fishbone render failed: %w", err)
		}
		return nil
	})
}

// DataSchema returns the data contract for fishbone diagrams.
func (d *FishboneDiagram) DataSchema() *DataSchema {
	return diagramDataSchema("Ishikawa cause-and-effect diagram", map[string]*DataSchema{
		"effect":  StringDataSchema("Problem at the arrow head (alias: problem)"),
		"problem": StringDataSchema("Alias for effect"),
		"categories": ArrayDataSchema("Cause categories (bones)", ObjectDataSchema("A cause category", map[string]*DataSchema{
			"name":   StringDataSchema("Bone label (alias: label)"),
			"label":  StringDataSchema("Alias for name"),
			"causes": stringListSchema("Causes on this bone"),
		}, nil), 0),
	}, nil)
}

// extractFishboneData extracts FishboneData from a request envelope.
func extractFishboneData(req *RequestEnvelope) (FishboneData, error) {
	data := req.Data
	fbData := FishboneData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	if effect, ok := data["effect"].(string); ok {
		fbData.Effect = effect
	}

	if categories, ok := data["categories"]; ok {
		catSlice, ok := categories.([]any)
		if !ok {
			return fbData, fmt.Errorf("fishbone 'categories' must be an array")
		}

		for _, catItem := range catSlice {
			catMap, ok := catItem.(map[string]any)
			if !ok {
				continue
			}

			cat := FishboneCategory{}
			if name, ok := catMap["name"].(string); ok {
				cat.Name = name
			} else if label, ok := catMap["label"].(string); ok {
				cat.Name = label
			}

			if causes, ok := catMap["causes"]; ok {
				if causeSlice, ok := toStringSlice(causes); ok {
					cat.Causes = causeSlice
				}
			}

			fbData.Categories = append(fbData.Categories, cat)
		}
	}

	return fbData, nil
}
