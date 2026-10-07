package svggen

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// =============================================================================
// Org Chart Diagram
// =============================================================================

// OrgChartConfig holds configuration for org chart diagrams.
type OrgChartConfig struct {
	ChartConfig

	// LevelAccents gives each hierarchy level its own theme accent. Off by
	// default: levels are the primary accent and its tints.
	LevelAccents bool

	// NodeWidth is the width of each person box in points.
	NodeWidth float64

	// NodeHeight is the height of each person box in points.
	NodeHeight float64

	// HorizontalGap is the horizontal spacing between sibling nodes.
	HorizontalGap float64

	// VerticalGap is the vertical spacing between hierarchy levels.
	VerticalGap float64

	// CornerRadius is the radius for rounded node corners. Zero (the
	// default) draws square boxes, the surface every native framework uses.
	CornerRadius float64

	// ConnectorColor overrides the color of connector lines (nil = use TextSecondary).
	ConnectorColor *Color

	// MaxVisibleSiblings is the maximum number of children to show before
	// collapsing excess nodes into a "+N more" indicator. 0 means no limit.
	MaxVisibleSiblings int

	// nameFontFloor is the minimum font size for name labels (computed during scaleToFit).
	nameFontFloor float64

	// titleFontFloor is the minimum font size for title labels (computed during scaleToFit).
	titleFontFloor float64

	// levelFontScales holds per-depth font scale multipliers computed during
	// scaleToFit for level-aware font scaling. Index is depth, value is 0..1.
	levelFontScales []float64

	// growth is the factor growToFit enlarged the tree by (1 = not grown).
	growth float64
}

// DefaultOrgChartConfig returns default org chart configuration.
func DefaultOrgChartConfig(width, height float64) OrgChartConfig {
	// Scale node sizes relative to chart size for responsive rendering.
	// Nodes must be large enough for readable text at presentation scale
	// (SVG is scaled ~0.6x when placed in PPTX placeholders).
	// Minimum 180pt wide ensures 14-char titles (e.g., "VP Engineering") fit.
	nodeW := math.Max(180, width*0.18)
	nodeH := math.Max(80, height*0.14)
	hGap := math.Max(18, width*0.028)
	vGap := math.Max(40, height*0.10)

	return OrgChartConfig{
		ChartConfig:        DefaultChartConfig(width, height),
		NodeWidth:          nodeW,
		NodeHeight:         nodeH,
		HorizontalGap:      hGap,
		VerticalGap:        vGap,
		CornerRadius:       0,
		MaxVisibleSiblings: 9,
		nameFontFloor:      8,
		titleFontFloor:     7,
	}
}

// OrgNode represents a person/role in the org chart.
type OrgNode struct {
	// Name is the person or role name (primary label).
	Name string

	// Title is the job title or role description (secondary label).
	Title string

	// Children are the direct reports.
	Children []OrgNode
}

// OrgChartData represents the data for an org chart diagram.
type OrgChartData struct {
	// Title is the diagram title.
	Title string

	// Subtitle is the diagram subtitle.
	Subtitle string

	// Root is the top-level node (CEO/leader).
	Root OrgNode

	// Footnote is an optional footnote text.
	Footnote string
}

// OrgChartRenderer renders org chart diagrams.
type OrgChartRenderer struct {
	builder *SVGBuilder
	config  OrgChartConfig
}

// NewOrgChartRenderer creates a new org chart renderer.
func NewOrgChartRenderer(builder *SVGBuilder, config OrgChartConfig) *OrgChartRenderer {
	return &OrgChartRenderer{
		builder: builder,
		config:  config,
	}
}

// layoutNode holds computed position and size for a node during layout.
type layoutNode struct {
	// The original data
	name     string
	title    string
	children []*layoutNode

	// Computed layout values
	x, y         float64 // center position of the node
	subtreeWidth float64 // total width of this node's subtree
	depth        int     // depth in the hierarchy (0 = root)
	isOverflow   bool    // true if this is a "+N more" placeholder node
	// prunedReports counts the descendants removed from under this node by
	// depth pruning. It is drawn as its own line under the node's real title,
	// which pruning used to OVERWRITE — a director's job title was replaced by
	// "+3 reports", so the chart lost data it was not asked to lose
	// (go-slide-creator-s5ur).
	prunedReports int
}

// Draw renders the org chart diagram.
func (oc *OrgChartRenderer) Draw(data OrgChartData) error {
	b := oc.builder
	style := b.StyleGuide()

	// The tree runs to the edges of its canvas: the page margin is the
	// placeholder's own, and a second one inside it left the chart using
	// half the body (go-slide-creator-mhc3k).
	plotArea := oc.config.PlotArea()
	if edge := orgChartEdgePad; plotArea.W > 0 && plotArea.H > 0 {
		top := edge
		if oc.config.ShowTitle && data.Title != "" {
			top = oc.config.MarginTop
		}
		plotArea = Rect{X: edge, Y: top, W: math.Max(0, oc.config.Width-2*edge), H: math.Max(0, oc.config.Height-top-edge)}
	}

	// Adjust for title
	headerHeight := 0.0
	if oc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Adjust for footnote
	footerHeight := 0.0
	if data.Footnote != "" {
		footerHeight = FootnoteReservedHeight(style)
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + footerHeight

	// Build layout tree from data
	root := oc.buildLayoutTree(&data.Root, 0)

	// Compute subtree widths bottom-up
	oc.computeSubtreeWidths(root)

	// Scale node dimensions if tree won't fit
	oc.scaleToFit(root, plotArea)
	// ... and let a small tree grow into the space it was given.
	oc.growToFit(root, plotArea)

	// Position nodes top-down
	oc.positionNodes(root, plotArea.X+plotArea.W/2, plotArea.Y+oc.config.NodeHeight/2, plotArea)

	// Clamp: shift the entire tree so all nodes fit within the plot area.
	// This handles residual overflow when the subtree is slightly wider
	// than the plot area after scaling.
	oc.clampToPlotArea(root, plotArea)

	// Draw connectors first (behind nodes)
	oc.drawConnectors(root)

	// Draw nodes
	oc.drawNodes(root)

	// Draw title
	if oc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: oc.config.Width, H: headerHeight + oc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: oc.config.Height - footerHeight,
			W: oc.config.Width,
			H: footerHeight,
		})
	}

	return nil
}

// buildLayoutTree converts OrgNode data into a layout tree.
// It also applies sibling overflow collapsing when MaxVisibleSiblings > 0.
func (oc *OrgChartRenderer) buildLayoutTree(node *OrgNode, depth int) *layoutNode {
	ln := &layoutNode{
		name:  node.Name,
		title: node.Title,
		depth: depth,
	}
	for i := range node.Children {
		child := oc.buildLayoutTree(&node.Children[i], depth+1)
		ln.children = append(ln.children, child)
	}

	// Collapse excess siblings: when a node has more children than
	// MaxVisibleSiblings, keep the first (MaxVisibleSiblings-1) and replace
	// the rest with a single "+N more" placeholder node.
	oc.collapseSiblings(ln)

	return ln
}

// collapseSiblings checks if a node has more children than MaxVisibleSiblings
// and, if so, collapses the excess into a "+N more" placeholder node.
func (oc *OrgChartRenderer) collapseSiblings(node *layoutNode) {
	limit := oc.config.MaxVisibleSiblings
	if limit <= 0 || len(node.children) <= limit {
		return
	}

	// Count total hidden descendants (not just direct children).
	hiddenCount := 0
	for _, child := range node.children[limit-1:] {
		hiddenCount += oc.countDescendants(child)
	}

	// Create the overflow placeholder.
	overflow := &layoutNode{
		name:       fmt.Sprintf("+%d more", hiddenCount),
		title:      "",
		depth:      node.children[0].depth,
		isOverflow: true,
	}

	// Keep first (limit-1) children + the overflow node.
	node.children = append(node.children[:limit-1], overflow)

	oc.builder.AddFinding(Finding{
		Code:     FindingOverflowSuppressed,
		Message:  fmt.Sprintf("org chart: %d nodes collapsed under %q — exceeded max visible siblings (%d)", hiddenCount, node.name, limit),
		Severity: "warning",
		Fix: &FixSuggestion{
			Kind:   FixKindReduceItems,
			Params: map[string]any{"hidden_count": hiddenCount, "parent_node": node.name, "max_visible": limit, "diagram_type": "org_chart"},
		},
	})
}

// computeSubtreeWidths computes the total width each subtree needs (bottom-up).
func (oc *OrgChartRenderer) computeSubtreeWidths(node *layoutNode) {
	if len(node.children) == 0 {
		node.subtreeWidth = oc.config.NodeWidth
		return
	}

	totalChildWidth := 0.0
	for _, child := range node.children {
		oc.computeSubtreeWidths(child)
		totalChildWidth += child.subtreeWidth
	}
	// Add gaps between children
	totalChildWidth += float64(len(node.children)-1) * oc.config.HorizontalGap

	// Subtree width is max of this node's width and children's total width
	node.subtreeWidth = math.Max(oc.config.NodeWidth, totalChildWidth)
}

// maxDepth returns the maximum depth of the tree.
func (oc *OrgChartRenderer) maxDepth(node *layoutNode) int {
	if len(node.children) == 0 {
		return node.depth
	}
	max := node.depth
	for _, child := range node.children {
		d := oc.maxDepth(child)
		if d > max {
			max = d
		}
	}
	return max
}

// maxBreadthAtDepth returns the maximum number of nodes at any single depth level.
func (oc *OrgChartRenderer) maxBreadthAtDepth(root *layoutNode) int {
	counts := map[int]int{}
	var walk func(n *layoutNode)
	walk = func(n *layoutNode) {
		counts[n.depth]++
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)

	maxB := 0
	for _, c := range counts {
		if c > maxB {
			maxB = c
		}
	}
	return maxB
}

// scaleToFit adjusts node/gap sizes if the tree doesn't fit the plot area.
// It also computes adaptive font floors based on tree complexity.
//
// When the tree is too deep or wide to render legibly even after scaling,
// scaleToFit prunes leaf-level children to keep nodes at a readable size.
func (oc *OrgChartRenderer) scaleToFit(root *layoutNode, plotArea Rect) {
	// Minimum node dimensions below which text becomes illegible. When
	// projected node sizes fall below these thresholds, the deepest level is
	// pruned to keep remaining nodes at a readable size.
	const pruneMinNodeWidth = 80.0  // threshold for pruning decisions
	const pruneMinNodeHeight = 40.0 // threshold for pruning decisions

	// Prune deep/wide trees to keep nodes at a readable size. For trees with
	// 15+ nodes, allow pruning at depth > 1 (i.e., 3-level trees can be pruned).
	// For smaller trees, only prune at depth > 2 to keep the full structure.
	totalNodes := oc.countDescendants(root)
	for {
		oc.computeSubtreeWidths(root)
		currentDepth := oc.maxDepth(root)

		// Determine minimum depth below which we don't prune.
		minPruneDepth := 2 // default: don't prune 3-level or shallower trees
		if totalNodes >= 18 {
			minPruneDepth = 1 // dense trees: allow pruning 3-level trees
		}
		if currentDepth <= minPruneDepth {
			break
		}

		totalHeight := float64(currentDepth+1)*oc.config.NodeHeight + float64(currentDepth)*oc.config.VerticalGap

		// Estimate whether we'd need to scale below minimum dimensions
		hScale := 1.0
		if root.subtreeWidth > plotArea.W {
			hScale = plotArea.W / root.subtreeWidth
		}
		vScale := 1.0
		if totalHeight > plotArea.H {
			vScale = plotArea.H / totalHeight
		}

		projectedNodeW := oc.config.NodeWidth * hScale
		projectedNodeH := oc.config.NodeHeight * vScale

		// If projected sizes are acceptable or we can't prune further, stop
		if (projectedNodeW >= pruneMinNodeWidth && projectedNodeH >= pruneMinNodeHeight) || currentDepth <= minPruneDepth {
			break
		}

		// Prune the deepest level. collapseSiblings already reports the nodes
		// IT hides, but this path reported nothing: a 21-person chart rendered
		// as 5 boxes reading "+4 reports" with no warning and no finding, so an
		// agent that fed real names in had no way to know the deck now
		// misrepresents the org (go-slide-creator-pwcg).
		if removed := oc.pruneDeepestLevel(root, currentDepth); removed > 0 {
			oc.builder.AddFinding(Finding{
				Code: FindingOrgChartDepthPruned,
				Message: fmt.Sprintf(
					"org chart: %d node(s) below depth %d removed to keep the remaining boxes readable — the chart no longer shows the full org; split it across slides or reduce its depth",
					removed, currentDepth-1),
				Severity: "warning",
				Fix: &FixSuggestion{
					Kind: FixKindReduceItems,
					Params: map[string]any{
						"removed_count": removed,
						"kept_depth":    currentDepth - 1,
						"diagram_type":  "org_chart",
					},
				},
			})
		}
		totalNodes = oc.countDescendants(root)
	}

	// Check horizontal fit. Enforce a minimum node width so text remains
	// legible (~8 chars at the smallest font size).
	const absMinNodeWidth = 80.0
	if root.subtreeWidth > plotArea.W {
		scale := plotArea.W / root.subtreeWidth
		oc.config.NodeWidth = math.Max(absMinNodeWidth, oc.config.NodeWidth*scale)
		oc.config.HorizontalGap *= scale
		// Recompute subtree widths with new dimensions
		oc.computeSubtreeWidths(root)
	}

	// Check vertical fit
	maxDepth := oc.maxDepth(root) // recalculate after pruning
	totalHeight := float64(maxDepth+1)*oc.config.NodeHeight + float64(maxDepth)*oc.config.VerticalGap
	if totalHeight > plotArea.H {
		scale := plotArea.H / totalHeight
		oc.config.NodeHeight *= scale
		oc.config.VerticalGap *= scale
	}

	// Compute adaptive font floors based on tree complexity.
	// For deep/wide trees with small nodes, lower the minimum font size
	// so text can shrink instead of being severely truncated.
	maxBreadth := oc.maxBreadthAtDepth(root)
	complexity := maxDepth * maxBreadth // proxy for visual density

	// Default floors use the builder's minimum font size (7pt) for both
	// name and title text. For complex trees, floors stay at the minimum
	// rather than dropping below the legibility threshold.
	floor := oc.builder.MinFontSize()
	minNameFloor := floor
	minTitleFloor := floor
	oc.config.nameFontFloor = math.Max(oc.config.nameFontFloor, floor)
	oc.config.titleFontFloor = math.Max(oc.config.titleFontFloor, floor)

	if complexity > 6 {
		// Scale font floor down toward minimum: at complexity 6 use default,
		// at 30+ use the floor. Linear interpolation between thresholds.
		t := math.Min(1.0, float64(complexity-6)/24.0) // t goes 0..1
		oc.config.nameFontFloor = math.Max(minNameFloor, floor+2.0-t*2.0)
		oc.config.titleFontFloor = math.Max(minTitleFloor, floor+1.0-t*1.0)
	}

	// Compute level-aware font scale factors. Deeper levels get progressively
	// smaller text. The decay rate adapts to tree depth: shallow trees (2-3
	// levels) use a gentle decay; deep trees (4+) use a steeper decay so
	// the deepest nodes have noticeably smaller text and the upper levels
	// remain prominent.
	oc.config.levelFontScales = make([]float64, maxDepth+1)
	// decayPerLevel: for 2-level trees use 0.06 (barely noticeable), for 5+
	// level trees use up to 0.12 for clear visual hierarchy.
	decayPerLevel := 0.06
	if maxDepth >= 3 {
		decayPerLevel = math.Min(0.12, 0.06+float64(maxDepth-2)*0.02)
	}
	for d := 0; d <= maxDepth; d++ {
		// Scale starts at 1.0 for root, drops by decayPerLevel per level,
		// with a floor at 0.60 to keep text minimally readable.
		oc.config.levelFontScales[d] = math.Max(0.60, 1.0-float64(d)*decayPerLevel)
	}

	// Breadth-aware gap compression: when a level is very wide, compress
	// horizontal gaps to give more space to node content. This is applied
	// as a post-pass after the main scale-to-fit above.
	if maxBreadth >= 6 {
		// Compress gaps more aggressively for wider trees.
		gapScale := math.Max(0.40, 1.0-float64(maxBreadth-5)*0.06)
		oc.config.HorizontalGap *= gapScale
		oc.computeSubtreeWidths(root)
	}

	// Final safety pass: if subtree still overflows plotArea after all
	// scaling and gap compression, squeeze gaps further (down to 2pt min)
	// and reduce node width toward absMinNodeWidth. This prevents nodes
	// from being positioned outside the SVG boundaries.
	oc.computeSubtreeWidths(root)
	if root.subtreeWidth > plotArea.W {
		const absMinGap = 2.0
		// First try compressing gaps further.
		if oc.config.HorizontalGap > absMinGap {
			ratio := (plotArea.W - oc.leafNodeTotalWidth(root)) /
				math.Max(1, root.subtreeWidth-oc.leafNodeTotalWidth(root))
			if ratio > 0 && ratio < 1 {
				oc.config.HorizontalGap = math.Max(absMinGap, oc.config.HorizontalGap*ratio)
				oc.computeSubtreeWidths(root)
			}
		}
		// If still overflowing, scale everything proportionally.
		if root.subtreeWidth > plotArea.W {
			finalScale := plotArea.W / root.subtreeWidth
			oc.config.NodeWidth *= finalScale
			oc.config.HorizontalGap = math.Max(absMinGap, oc.config.HorizontalGap*finalScale)
			oc.computeSubtreeWidths(root)
		}
	}
}

// Tree growth. A three-person chart drawn at the default box size used a
// third of a body placeholder; boxes, gaps and type grow together, up to
// orgChartMaxGrowth, until the tree reaches the plot's width or height.
const (
	orgChartMaxGrowth = 1.35
	// orgChartEdgePad is the space between the canvas edge and the tree.
	orgChartEdgePad = 4.0
	// orgChartMaxNamePt and orgChartMaxTitlePt cap the grown type.
	orgChartMaxNamePt  = 16.0
	orgChartMaxTitlePt = 14.0
	// orgChartNameStep sets a name one type step above the floor its title
	// stands on (14pt over 12pt) where the box is wide enough.
	orgChartNameStep = 14.0 / 12.0
	// orgChartMaxTitleLines is the most lines a wrapped job title takes.
	orgChartMaxTitleLines = 3
	// orgChartConnectorWidth is the connector weight: a neutral 1.25pt line.
	orgChartConnectorWidth = 1.25
)

// growToFit enlarges boxes and gaps by one factor when the tree is smaller
// than the plot in both directions, and records the factor for the type.
func (oc *OrgChartRenderer) growToFit(root *layoutNode, plotArea Rect) {
	oc.config.growth = 1
	oc.computeSubtreeWidths(root)
	depth := oc.maxDepth(root)
	totalH := float64(depth+1)*oc.config.NodeHeight + float64(depth)*oc.config.VerticalGap
	if root.subtreeWidth <= 0 || totalH <= 0 {
		return
	}
	g := math.Min(orgChartMaxGrowth, math.Min(plotArea.W/root.subtreeWidth, plotArea.H/totalH))
	if g <= 1 {
		return
	}
	oc.config.growth = g
	oc.config.NodeWidth *= g
	oc.config.NodeHeight *= g
	oc.config.HorizontalGap *= g
	oc.config.VerticalGap *= g
	oc.computeSubtreeWidths(root)
}

// pruneDeepestLevel removes all children at the specified depth, replacing
// parent nodes that had children with a "+N" suffix to indicate hidden reports.
// It returns the number of nodes removed so the caller can report the loss.
func (oc *OrgChartRenderer) pruneDeepestLevel(node *layoutNode, maxDepth int) int {
	if len(node.children) == 0 {
		return 0
	}

	removed := 0
	for _, child := range node.children {
		if child.depth == maxDepth-1 {
			// This child's children are at the deepest level — prune them
			if len(child.children) > 0 {
				count := oc.countDescendants(child) - 1 // exclude the child itself
				if count > 0 {
					child.prunedReports += count
					removed += count
				}
				child.children = nil
			}
		} else {
			removed += oc.pruneDeepestLevel(child, maxDepth)
		}
	}
	return removed
}

// leafNodeTotalWidth returns the sum of NodeWidth for all leaf nodes in the tree.
// Used to separate node-width from gap-width when compressing gaps.
func (oc *OrgChartRenderer) leafNodeTotalWidth(node *layoutNode) float64 {
	if len(node.children) == 0 {
		return oc.config.NodeWidth
	}
	total := 0.0
	for _, child := range node.children {
		total += oc.leafNodeTotalWidth(child)
	}
	return total
}

// countDescendants returns the total number of nodes in the subtree (including the node itself).
func (oc *OrgChartRenderer) countDescendants(node *layoutNode) int {
	count := 1
	for _, child := range node.children {
		count += oc.countDescendants(child)
	}
	return count
}

// positionNodes positions nodes in the layout tree (top-down).
func (oc *OrgChartRenderer) positionNodes(node *layoutNode, centerX, topY float64, plotArea Rect) {
	node.x = centerX
	node.y = topY

	if len(node.children) == 0 {
		return
	}

	// Calculate the total width of all children's subtrees
	totalChildWidth := 0.0
	for _, child := range node.children {
		totalChildWidth += child.subtreeWidth
	}
	totalChildWidth += float64(len(node.children)-1) * oc.config.HorizontalGap

	// Start position for the first child (left edge)
	startX := centerX - totalChildWidth/2

	childTopY := topY + oc.config.NodeHeight + oc.config.VerticalGap

	currentX := startX
	for _, child := range node.children {
		childCenterX := currentX + child.subtreeWidth/2
		oc.positionNodes(child, childCenterX, childTopY, plotArea)
		currentX += child.subtreeWidth + oc.config.HorizontalGap
	}
}

// clampToPlotArea shifts all node positions so that no node extends beyond
// the plot area boundaries. It first computes the bounding box of all nodes,
// then shifts the entire tree to fit. This is a final safety net for the
// SVG clipping bug (nodes rendered outside the viewBox).
func (oc *OrgChartRenderer) clampToPlotArea(root *layoutNode, plotArea Rect) {
	halfW := oc.config.NodeWidth / 2
	halfH := oc.config.NodeHeight / 2

	// Find the bounding box of all node rectangles.
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	var walkBounds func(n *layoutNode)
	walkBounds = func(n *layoutNode) {
		left := n.x - halfW
		right := n.x + halfW
		top := n.y - halfH
		bottom := n.y + halfH
		if left < minX {
			minX = left
		}
		if right > maxX {
			maxX = right
		}
		if top < minY {
			minY = top
		}
		if bottom > maxY {
			maxY = bottom
		}
		for _, c := range n.children {
			walkBounds(c)
		}
	}
	walkBounds(root)

	// Compute how much to shift to keep everything inside the plot area.
	shiftX := 0.0
	shiftY := 0.0

	if minX < plotArea.X {
		shiftX = plotArea.X - minX
	} else if maxX > plotArea.X+plotArea.W {
		shiftX = (plotArea.X + plotArea.W) - maxX
	}

	if minY < plotArea.Y {
		shiftY = plotArea.Y - minY
	} else if maxY > plotArea.Y+plotArea.H {
		shiftY = (plotArea.Y + plotArea.H) - maxY
	}

	// If the tree is wider/taller than the plot area, center it instead
	// of shifting to one side. This keeps the layout balanced.
	treeW := maxX - minX
	treeH := maxY - minY
	if treeW > plotArea.W {
		// Center horizontally within the plot area.
		treeCenterX := (minX + maxX) / 2
		plotCenterX := plotArea.X + plotArea.W/2
		shiftX = plotCenterX - treeCenterX
	}
	if treeH > plotArea.H {
		// Center vertically within the plot area.
		treeCenterY := (minY + maxY) / 2
		plotCenterY := plotArea.Y + plotArea.H/2
		shiftY = plotCenterY - treeCenterY
	}

	if shiftX == 0 && shiftY == 0 {
		return
	}

	// Apply the shift to all nodes.
	var walkShift func(n *layoutNode)
	walkShift = func(n *layoutNode) {
		n.x += shiftX
		n.y += shiftY
		for _, c := range n.children {
			walkShift(c)
		}
	}
	walkShift(root)
}

// drawConnectors draws right-angle connectors from parent to children.
func (oc *OrgChartRenderer) drawConnectors(node *layoutNode) {
	if len(node.children) == 0 {
		return
	}

	b := oc.builder
	style := b.StyleGuide()

	// A neutral line: the accent belongs to the boxes, never to a hairline.
	connColor := NeutralInk(style.Palette, tonalRuleShare)
	if oc.config.ConnectorColor != nil {
		connColor = *oc.config.ConnectorColor
	}

	b.Push()
	b.SetStrokeColor(connColor)
	b.SetStrokeWidth(orgChartConnectorWidth)
	b.SetFillColor(Color{A: 0}) // no fill

	parentBottomY := node.y + oc.config.NodeHeight/2

	// Midpoint between parent bottom and children top
	midY := parentBottomY + oc.config.VerticalGap/2

	// Draw vertical line from parent bottom to midY
	b.DrawLine(node.x, parentBottomY, node.x, midY)

	if len(node.children) == 1 {
		// Single child: straight vertical line down
		child := node.children[0]
		childTopY := child.y - oc.config.NodeHeight/2
		if child.x != node.x {
			b.DrawLine(node.x, midY, child.x, midY)
		}
		b.DrawLine(child.x, midY, child.x, childTopY)
	} else {
		// Multiple children: horizontal bar with drop-downs
		leftX := node.children[0].x
		rightX := node.children[len(node.children)-1].x

		// Horizontal bar across all children
		b.DrawLine(leftX, midY, rightX, midY)

		// Vertical drops to each child
		for _, child := range node.children {
			childTopY := child.y - oc.config.NodeHeight/2
			b.DrawLine(child.x, midY, child.x, childTopY)
		}
	}

	b.Pop()

	// Recurse into children
	for _, child := range node.children {
		oc.drawConnectors(child)
	}
}

// drawNodes draws all nodes in the layout tree.
func (oc *OrgChartRenderer) drawNodes(node *layoutNode) {
	oc.drawNode(node)
	for _, child := range node.children {
		oc.drawNodes(child)
	}
}

// drawNode draws a single org chart node (box with name and title).
func (oc *OrgChartRenderer) drawNode(node *layoutNode) {
	b := oc.builder
	style := b.StyleGuide()

	nodeW := oc.config.NodeWidth
	nodeH := oc.config.NodeHeight

	rect := Rect{
		X: node.x - nodeW/2,
		Y: node.y - nodeH/2,
		W: nodeW,
		H: nodeH,
	}

	// The level's tonal role: the root is the one solid accent block, its
	// direct reports take the accent's light tint, everyone below sits on
	// the neutral panel surface. The hierarchy is read from the fill, so the
	// boxes need no outline and no accent rule (go-slide-creator-w107j).
	fill := oc.levelFill(style, node.depth)
	ink := diagramInkOn(style, fill)
	muted := ink
	if node.depth >= 2 || fill.ContrastWith(style.Palette.Background.Opaque()) < 1.6 {
		if m := diagramMutedInk(style); m.ContrastWith(fill) >= diagramInkMinimum {
			muted = m
		}
	}

	// Overflow placeholder: a dashed neutral outline, no fill.
	if node.isOverflow {
		b.Push()
		b.SetFillColor(Color{A: 0})
		b.SetStrokeColor(NeutralInk(style.Palette, tonalRuleShare))
		b.SetStrokeWidth(1)
		b.SetDashes(4, 3)
		oc.drawNodeBox(rect)
		b.Pop()

		fontSize := math.Max(oc.config.nameFontFloor, math.Min(style.Typography.SizeSmall, nodeW*0.10))
		b.Push()
		b.SetFontSize(fontSize)
		b.SetFontWeight(style.Typography.WeightNormal)
		b.SetTextColor(diagramMutedInk(style))
		b.DrawText(node.name, node.x, node.y, TextAlignCenter, TextBaselineMiddle)
		b.Pop()
		return
	}

	b.Push()
	b.SetFillColor(fill)
	b.SetStrokeWidth(0)
	b.SetStrokeColor(Color{A: 0})
	oc.drawNodeBox(rect)
	b.Pop()

	// Tier-depth font scaling: use the pre-computed level font scales from
	// scaleToFit for adaptive per-level sizing. Falls back to the legacy
	// formula when levelFontScales hasn't been computed (shouldn't happen in
	// normal rendering flow, but guards against direct drawNode calls in tests).
	tierScale := math.Max(0.60, 1.0-float64(node.depth)*0.08)
	if oc.config.levelFontScales != nil && node.depth < len(oc.config.levelFontScales) {
		tierScale = oc.config.levelFontScales[node.depth]
	}

	// Compute base text sizes scaled by tier depth.
	// A grown tree grows its type with it, up to the caps; no level drops
	// below the floor.
	growth := math.Max(1, oc.config.growth)
	nameBase := math.Max(style.Typography.SizeBody, oc.config.nameFontFloor*orgChartNameStep)
	namePreset := math.Max(oc.config.nameFontFloor, math.Min(math.Min(nameBase*growth, math.Max(orgChartMaxNamePt, nameBase)), nodeW*0.11)*tierScale)
	titlePreset := math.Max(oc.config.titleFontFloor, math.Min(math.Min(style.Typography.SizeSmall*growth, math.Max(orgChartMaxTitlePt, style.Typography.SizeSmall)), nodeW*0.09)*tierScale)

	textMaxW := nodeW * 0.85

	// Use LabelFitStrategy for accurate font-metric measurement and truncation.
	// Set bold weight before fitting name so measurement matches rendering.
	nameFit := LabelFitStrategy{PreferredSize: namePreset, MinSize: oc.config.nameFontFloor, MinCharWidth: 5.5}
	b.Push()
	b.SetFontWeight(style.Typography.WeightBold)
	nameResult := nameFit.Fit(b, node.name, textMaxW, 0)
	b.Pop()
	nameFontSize := nameResult.FontSize

	titleFontSize := titlePreset
	titleText := ""
	// A title too long for one line at the floor takes a second line when
	// the box has the height for it: "Chief Financial Officer" cut to
	// "Chief Financ…" lost the one word that says what the person does.
	var titleLines []string
	if node.title != "" {
		b.Push()
		b.SetFontWeight(style.Typography.WeightNormal)
		b.SetFontSize(oc.config.titleFontFloor)
		if w, _ := b.MeasureText(node.title); w > textMaxW {
			lines := fishboneWrap(b, node.title, textMaxW)
			fits := len(lines) <= orgChartMaxTitleLines &&
				namePreset*orgLabelLeadingFactor+float64(len(lines))*oc.config.titleFontFloor*orgLabelLeadingFactor <= nodeH*0.92
			for _, line := range lines {
				if lw, _ := b.MeasureText(line); lw > textMaxW {
					fits = false
				}
			}
			if fits {
				titleLines = lines
				titleFontSize = oc.config.titleFontFloor
				titleText = lines[0]
			}
		}
		if titleLines == nil {
			titleFit := LabelFitStrategy{PreferredSize: titlePreset, MinSize: oc.config.titleFontFloor, MinCharWidth: 5.5}
			titleResult := titleFit.Fit(b, node.title, textMaxW, 0)
			titleFontSize = titleResult.FontSize
			titleText = titleResult.DisplayText
		}
		b.Pop()
	}

	// Smart-abbreviate the name before falling back to hard truncation.
	// orgFitText tries "Christopher Smith" → "C. Smith" → "C.S." before
	// resorting to ellipsis, producing more readable labels.
	fittedName := orgFitText(b, node.name, textMaxW, nameFontSize)

	// If orgFitText's result still overflows at the actual font metrics,
	// fall back to Fit for a pixel-accurate trim.
	nameLabelFit := LabelFitStrategy{PreferredSize: nameFontSize, MinSize: nameFontSize, MinCharWidth: 5.5}
	b.Push()
	b.SetFontWeight(style.Typography.WeightBold)
	fittedResult := nameLabelFit.Fit(b, fittedName, textMaxW, 0)
	b.Pop()
	nameText := fittedResult.DisplayText

	// Check if wrapping the name into two lines would preserve more text.
	// We wrap when the name has multiple words and truncation still lost chars.
	wrapName := false
	var nameLines [2]string
	if nameText != node.name && strings.Contains(node.name, " ") {
		parts := strings.SplitN(node.name, " ", 2)
		lineFit := LabelFitStrategy{PreferredSize: nameFontSize, MinSize: nameFontSize, MinCharWidth: 5.5}
		b.Push()
		b.SetFontWeight(style.Typography.WeightBold)
		lineResult1 := lineFit.Fit(b, parts[0], textMaxW, 0)
		lineResult2 := lineFit.Fit(b, parts[1], textMaxW, 0)
		b.Pop()
		line1 := lineResult1.DisplayText
		line2 := lineResult2.DisplayText
		if utf8.RuneCountInString(line1)+utf8.RuneCountInString(line2) >
			utf8.RuneCountInString(nameText) {
			wrapName = true
			nameLines = [2]string{line1, line2}
		}
	}

	// Draw name (primary label)
	b.Push()
	b.SetFontSize(nameFontSize)
	b.SetFontWeight(style.Typography.WeightBold)
	b.SetTextColor(ink)

	// Both labels are drawn on a middle baseline, so the whole two- or
	// three-line block is laid out from its own centre: every gap is a real
	// baseline-to-baseline leading rather than a fraction of one font size.
	// The old arithmetic (titleY = nameY + nameFontSize*0.9) left 10.3px between
	// a 12.1px name and a 10.4px title, whose half-heights sum to 11.3 — so the
	// glyphs touched in every node (go-slide-creator-s5ur).
	leading := orgLabelLeading(nameFontSize, titleFontSize)
	hasTitle := titleText != ""
	// Pruned reports get their own line rather than displacing the real title.
	reportsText := ""
	if node.prunedReports > 0 {
		reportsText = fmt.Sprintf("+%d reports", node.prunedReports)
	}

	// Build the block bottom-up so every line is spaced by a real leading and
	// the whole stack stays centred in the node box.
	secondary := make([]string, 0, 2)
	switch {
	case len(titleLines) > 0:
		secondary = append(secondary, titleLines...)
	case hasTitle:
		secondary = append(secondary, titleText)
	}
	if reportsText != "" {
		secondary = append(secondary, reportsText)
	}

	blockH := float64(len(secondary)) * leading
	nameLineSpacing := 0.0
	if wrapName {
		nameLineSpacing = orgLabelLeading(nameFontSize, nameFontSize)
		blockH += nameLineSpacing
	}
	baseY := node.y - blockH/2

	if wrapName {
		b.DrawText(nameLines[0], node.x, baseY, TextAlignCenter, TextBaselineMiddle)
		b.DrawText(nameLines[1], node.x, baseY+nameLineSpacing, TextAlignCenter, TextBaselineMiddle)
	} else {
		b.DrawText(nameText, node.x, baseY, TextAlignCenter, TextBaselineMiddle)
	}
	b.Pop()

	if len(secondary) == 0 {
		return
	}
	b.Push()
	b.SetFontSize(titleFontSize)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(muted)
	y := baseY + nameLineSpacing
	for _, line := range secondary {
		y += leading
		b.DrawText(line, node.x, y, TextAlignCenter, TextBaselineMiddle)
	}
	b.Pop()
}

// orgLabelLeadingFactor is the baseline-to-baseline distance between two label
// lines in a node, as a multiple of the larger of the two font sizes. 1.15 keeps
// the glyph boxes apart with room to spare: two lines of sizes a and b touch
// when the gap falls below (a+b)/2, and max(a,b)*1.15 always exceeds that.
const orgLabelLeadingFactor = 1.15

// orgLabelLeading returns the baseline-to-baseline distance to use between two
// node label lines drawn at the given font sizes.
func orgLabelLeading(upperFontSize, lowerFontSize float64) float64 {
	return math.Max(upperFontSize, lowerFontSize) * orgLabelLeadingFactor
}

// orgFitText tries to fit text within maxWidth at the given fontSize using
// the builder's font metrics. It applies smart abbreviation for names before
// falling back to truncation:
//
//	"Christopher Smith" -> "C. Smith" -> "C.S." -> truncate
func orgFitText(b *SVGBuilder, text string, maxWidth, fontSize float64) string {
	b.Push()
	b.SetFontSize(fontSize)
	defer b.Pop()

	w, _ := b.MeasureText(text)
	if w <= maxWidth {
		return text
	}

	// Try abbreviation strategies for names that contain spaces.
	if idx := strings.LastIndex(text, " "); idx > 0 {
		first := text[:idx]
		last := text[idx+1:]

		// Strategy 1: "F. Last" (first initial + last name)
		if len(last) > 0 {
			abbr1 := string([]rune(first)[:1]) + ". " + last
			if w1, _ := b.MeasureText(abbr1); w1 <= maxWidth {
				return abbr1
			}
		}

		// Strategy 2: "F.L." (initials only)
		if len(last) > 0 {
			abbr2 := string([]rune(first)[:1]) + "." + string([]rune(last)[:1]) + "."
			if w2, _ := b.MeasureText(abbr2); w2 <= maxWidth {
				return abbr2
			}
		}
	}

	// Fallback: let the caller's LabelFitStrategy handle truncation.
	return text
}

// drawNodeBox draws a node's box with the configured corners.
func (oc *OrgChartRenderer) drawNodeBox(rect Rect) {
	if oc.config.CornerRadius > 0 {
		oc.builder.DrawRoundedRect(rect, oc.config.CornerRadius)
		return
	}
	oc.builder.DrawRect(rect)
}

// levelFill is the fill of a node at depth. By default the tonal roles carry
// the hierarchy: solid accent root, accent-tint second level, neutral panel
// below. Authored colours, or a deck accent strategy that rotates accents,
// paint each level solid in its own colour instead.
func (oc *OrgChartRenderer) levelFill(style *StyleGuide, depth int) Color {
	if colors := oc.getColors(style); len(colors) > 0 {
		return colors[depth%len(colors)].Opaque()
	}
	switch depth {
	case 0:
		return diagramAccent(style)
	case 1:
		return diagramContentFill(style)
	default:
		return diagramPanelFill(style)
	}
}

// getColors returns the per-level colours an author or a rotating accent
// strategy asked for, or nil for the default tonal roles (levelFill). Taking
// accent1, accent2, ... per level by default painted the second level red on
// midnight-blue and a foreign blue on abstract, a colour nothing else in a
// primary-accent deck uses (go-slide-creator-libnz).
func (oc *OrgChartRenderer) getColors(style *StyleGuide) []Color {
	if len(oc.config.Colors) > 0 {
		return oc.config.Colors
	}
	if oc.config.LevelAccents {
		if accents := style.Palette.AccentColors(); len(accents) >= 3 {
			return accents
		}
	}
	return nil
}

// orgChartLevelAccents reports whether data asks for one accent per level:
// only under a deck accent_strategy that rotates accents ("rotate",
// "section-keyed"), which the generator passes as data.accent_strategy.
func orgChartLevelAccents(data map[string]any) bool {
	s, _ := data["accent_strategy"].(string)
	return s == "rotate" || s == "section-keyed"
}

// =============================================================================
// Org Chart Diagram Type (for Registry)
// =============================================================================

// OrgChartDiagram implements the Diagram interface for org chart diagrams.
type OrgChartDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for org chart diagrams.
func (d *OrgChartDiagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("org chart diagram requires data. Expected format: {\"root\": {\"name\": \"CEO\", \"children\": [{\"name\": \"CTO\"}, {\"name\": \"CFO\"}]}}")
	}

	// Accept flat "nodes" array with parent references — convert to tree on the fly.
	normalizeOrgChartNodes(req.Data)

	_, hasRoot := req.Data["root"]
	_, hasName := req.Data["name"]
	_, hasTitle := req.Data["title"]
	if !hasRoot && !hasName && !hasTitle {
		return fmt.Errorf("org chart diagram requires 'root' node or flat node with 'name'/'title' in data. Expected: {\"root\": {\"name\": \"CEO\", \"children\": [{\"name\": \"CTO\"}]}} or {\"nodes\": [{\"id\": \"1\", \"name\": \"CEO\"}, {\"id\": \"2\", \"name\": \"CTO\", \"parent\": \"1\"}]}")
	}

	return nil
}

// Render generates an SVG document from the request envelope.
func (d *OrgChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
func (d *OrgChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	// Pre-parse the tree to determine complexity and compute adaptive canvas
	// dimensions. Large/deep trees get a taller canvas so nodes remain readable.
	defaultW, defaultH := 1100.0, 700.0
	data, err := parseOrgChartData(req)
	if err == nil {
		totalNodes := countOrgNodes(&data.Root)
		maxDepth := orgNodeMaxDepth(&data.Root, 0)

		// Scale height: +120pt per tier beyond 2 (was 3), cap at 1400 (was 1200)
		if maxDepth >= 2 && totalNodes >= 8 {
			extraTiers := maxDepth - 1
			defaultH = math.Min(1400, defaultH+float64(extraTiers)*120)
		}

		// Scale width based on breadth: wider trees need more space
		// Count max breadth at any level
		maxBreadth := maxBreadthAtAnyLevel(&data.Root)
		if maxBreadth >= 3 {
			defaultW = math.Min(1800, defaultW+float64(maxBreadth-2)*80)
		}
		if totalNodes >= 12 {
			defaultW = math.Min(1800, math.Max(defaultW, defaultW+float64(totalNodes-10)*20))
		}
	}

	return RenderWithHelperDimensions(req, defaultW, defaultH, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, err := parseOrgChartData(req)
		if err != nil {
			return err
		}

		assumeSlidePlacement(builder, req)
		width, height := builder.Width(), builder.Height()
		config := DefaultOrgChartConfig(width, height)

		// Apply custom options
		if nodeW, ok := req.Data["node_width"].(float64); ok {
			config.NodeWidth = nodeW
		}
		if nodeH, ok := req.Data["node_height"].(float64); ok {
			config.NodeHeight = nodeH
		}
		if hGap, ok := req.Data["horizontal_gap"].(float64); ok {
			config.HorizontalGap = hGap
		}
		if vGap, ok := req.Data["vertical_gap"].(float64); ok {
			config.VerticalGap = vGap
		}
		if radius, ok := req.Data["corner_radius"].(float64); ok {
			config.CornerRadius = radius
		}
		if maxSiblings, ok := req.Data["max_visible_siblings"].(float64); ok {
			config.MaxVisibleSiblings = int(maxSiblings)
		}
		config.LevelAccents = orgChartLevelAccents(req.Data)

		if issues := orgChartNodeIssues(req.Data, &data.Root); len(issues) > 0 {
			builder.AddFinding(Finding{
				Code:     FindingOrgChartNodesInvalid,
				Message:  "org chart: " + strings.Join(issues, "; "),
				Severity: "warning",
				Fix: &FixSuggestion{
					Kind:   FixKindReplaceValue,
					Params: map[string]any{"issues": issues, "diagram_type": "org_chart"},
				},
			})
		}

		chart := NewOrgChartRenderer(builder, config)
		return chart.Draw(data)
	})
}

// countOrgNodes returns the total number of nodes in an OrgNode tree.
func countOrgNodes(node *OrgNode) int {
	count := 1
	for i := range node.Children {
		count += countOrgNodes(&node.Children[i])
	}
	return count
}

// orgNodeMaxDepth returns the maximum depth of an OrgNode tree.
func orgNodeMaxDepth(node *OrgNode, depth int) int {
	max := depth
	for i := range node.Children {
		d := orgNodeMaxDepth(&node.Children[i], depth+1)
		if d > max {
			max = d
		}
	}
	return max
}

// maxBreadthAtAnyLevel returns the maximum number of children any single node has.
func maxBreadthAtAnyLevel(node *OrgNode) int {
	maxB := len(node.Children)
	for i := range node.Children {
		b := maxBreadthAtAnyLevel(&node.Children[i])
		if b > maxB {
			maxB = b
		}
	}
	return maxB
}

// parseOrgChartData parses the request data into OrgChartData.
func parseOrgChartData(req *RequestEnvelope) (OrgChartData, error) {
	data := OrgChartData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	rootRaw, ok := req.Data["root"].(map[string]any)
	if !ok {
		// Accept flat root: if data has "name" or "title" at top level, treat data itself as root
		if _, hasName := req.Data["name"]; hasName {
			rootRaw = req.Data
			ok = true
		} else if _, hasTitle := req.Data["title"]; hasTitle {
			rootRaw = req.Data
			ok = true
		}
	}
	if !ok {
		return data, fmt.Errorf("invalid org chart data: expected 'root' key or flat node with 'name'/'title'")
	}

	root, err := parseOrgNode(rootRaw)
	if err != nil {
		return data, fmt.Errorf("parsing root node: %w", err)
	}
	data.Root = root

	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}

// orgNodeSchema is the data contract of a tree node and its children,
// nestedSchemaDepth levels deep.
func orgNodeSchema(depth int) *DataSchema {
	fields := map[string]*DataSchema{
		"name":  StringDataSchema("Person or unit name"),
		"title": StringDataSchema("Role line under the name"),
		"id":    ToleratedDataSchema("Node id (only read in the flat nodes form)"),
	}
	var children *DataSchema
	if depth > 1 {
		children = orgNodeSchema(depth - 1)
	}
	fields["children"] = ArrayDataSchema("Child nodes", children, 0)
	return ObjectDataSchema("An org chart node", fields, nil)
}

// DataSchema returns the data contract for org chart diagrams: a root tree,
// a flat nodes list with parent ids, or the root node's keys at top level.
func (d *OrgChartDiagram) DataSchema() *DataSchema {
	return diagramDataSchema("Org chart hierarchy", map[string]*DataSchema{
		"root": orgNodeSchema(nestedSchemaDepth),
		"nodes": ArrayDataSchema("Flat nodes linked by parent id", ObjectDataSchema("A flat org node", map[string]*DataSchema{
			"id":     StringDataSchema("Node id"),
			"parent": StringDataSchema("Parent node id"),
			"name":   StringDataSchema("Person or unit name"),
			"title":  StringDataSchema("Role line under the name"),
		}, nil), 0),
		"name":                 StringDataSchema("Flat root form: root node name"),
		"title":                StringDataSchema("Flat root form: root node role"),
		"children":             ArrayDataSchema("Flat root form: root's children", orgNodeSchema(nestedSchemaDepth-1), 0),
		"footnote":             StringDataSchema("Footnote text"),
		"node_width":           NumberDataSchema("Node box width"),
		"node_height":          NumberDataSchema("Node box height"),
		"horizontal_gap":       NumberDataSchema("Gap between siblings"),
		"vertical_gap":         NumberDataSchema("Gap between levels"),
		"corner_radius":        NumberDataSchema("Node corner radius"),
		"max_visible_siblings": NumberDataSchema("Siblings shown before a +N summary node"),
		"accent_strategy":      EnumDataSchema("Deck accent strategy (set by the generator)", "primary", "rotate", "section-keyed"),
		orgNodeIssuesKey:       ToleratedDataSchema("Internal: node issues recorded by normalization"),
	}, nil)
}

// parseOrgNode recursively parses a node map into an OrgNode.
func parseOrgNode(m map[string]any) (OrgNode, error) {
	node := OrgNode{}

	if name, ok := m["name"].(string); ok {
		node.Name = name
	}
	if title, ok := m["title"].(string); ok {
		node.Title = title
	}

	if childrenRaw, ok := toAnySlice(m["children"]); ok {
		for _, cRaw := range childrenRaw {
			cMap, ok := cRaw.(map[string]any)
			if !ok {
				continue
			}
			child, err := parseOrgNode(cMap)
			if err != nil {
				return node, err
			}
			node.Children = append(node.Children, child)
		}
	}

	return node, nil
}

// orgNodeIssuesKey is the private data key under which normalizeOrgChartNodes
// records structural problems in a flat "nodes" array. Validate normalizes the
// payload before any builder exists, so the render pass reads the issues back
// from here and reports them as a FindingOrgChartNodesInvalid finding.
const orgNodeIssuesKey = "_org_node_issues"

// orgFlatNode is one entry of an org chart's flat "nodes" array.
type orgFlatNode struct {
	pos      int // index in the authored nodes array
	id       string
	parentID string
	raw      map[string]any
}

// resolveOrgParents maps each node to the index of its parent (-1 for a root)
// and reports duplicate ids, parents that match no id, and self-parents.
func resolveOrgParents(nodes []orgFlatNode) (parent []int, issues []string) {
	index := make(map[string]int, len(nodes))
	for i, fn := range nodes {
		if fn.id == "" {
			continue
		}
		if j, dup := index[fn.id]; dup {
			issues = append(issues, fmt.Sprintf("nodes[%d].id %q duplicates nodes[%d]; reports naming it attach to nodes[%d]", fn.pos, fn.id, nodes[j].pos, nodes[j].pos))
			continue
		}
		index[fn.id] = i
	}

	parent = make([]int, len(nodes))
	for i, fn := range nodes {
		parent[i] = -1
		if fn.parentID == "" {
			continue
		}
		j, ok := index[fn.parentID]
		switch {
		case !ok:
			issues = append(issues, fmt.Sprintf("nodes[%d].parent %q matches no node id; drawn under the top node", fn.pos, fn.parentID))
		case j == i:
			issues = append(issues, fmt.Sprintf("nodes[%d] (id %q) is its own parent; drawn under the top node", fn.pos, fn.id))
		default:
			parent[i] = j
		}
	}
	return parent, issues
}

// breakOrgParentCycles cuts every node whose ancestor walk returns to itself
// loose (parent -1, drawn under the top node) and reports it.
func breakOrgParentCycles(nodes []orgFlatNode, parent []int) (issues []string) {
	for i := range nodes {
		seen := map[int]bool{i: true}
		for k := parent[i]; k != -1; k = parent[k] {
			if k == i {
				issues = append(issues, fmt.Sprintf("nodes[%d] (id %q) is in a parent cycle; drawn under the top node", nodes[i].pos, nodes[i].id))
				parent[i] = -1
				break
			}
			if seen[k] {
				break // cycle not through i; cut when its member is visited
			}
			seen[k] = true
		}
	}
	return issues
}

// normalizeOrgChartNodes converts a flat "nodes" array (each node has "id" and
// optional "parent") into a nested "root" tree that parseOrgChartData expects.
// If "root" already exists or "nodes" is absent, this is a no-op.
//
// A parent id that matches no node, a duplicate id, a node that is its own
// parent and a parent cycle are all repaired (the node is drawn under the top
// node, or the duplicate is ignored as a parent) and recorded under
// orgNodeIssuesKey. They used to be absorbed silently: an orphan was drawn in
// map-iteration order, and a self-parent recursed forever.
func normalizeOrgChartNodes(data map[string]any) {
	if _, hasRoot := data["root"]; hasRoot {
		return
	}
	nodesRaw, ok := data["nodes"]
	if !ok {
		return
	}
	nodeSlice, ok := toAnySlice(nodesRaw)
	if !ok || len(nodeSlice) == 0 {
		return
	}

	nodes := make([]orgFlatNode, 0, len(nodeSlice))
	for pos, raw := range nodeSlice {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		parentID, _ := m["parent"].(string)
		nodes = append(nodes, orgFlatNode{pos: pos, id: id, parentID: parentID, raw: m})
	}

	parent, issues := resolveOrgParents(nodes)
	issues = append(issues, breakOrgParentCycles(nodes, parent)...)

	// Build clean node maps (name, title, children) without touching the
	// caller's node maps, then link them in authored order.
	tree := make([]map[string]any, len(nodes))
	for i, fn := range nodes {
		node := map[string]any{}
		if name, ok := fn.raw["name"].(string); ok {
			node["name"] = name
		}
		if title, ok := fn.raw["title"].(string); ok {
			node["title"] = title
		}
		tree[i] = node
	}
	var roots []map[string]any
	for i := range nodes {
		if parent[i] == -1 {
			roots = append(roots, tree[i])
			continue
		}
		p := tree[parent[i]]
		children, _ := p["children"].([]any)
		p["children"] = append(children, tree[i])
	}

	if len(issues) > 0 {
		data[orgNodeIssuesKey] = issues
	}
	if len(roots) == 0 {
		return
	}

	// Single root: use directly. Multiple roots: first root adopts the rest as children.
	if len(roots) == 1 {
		data["root"] = roots[0]
	} else {
		primary := roots[0]
		existingChildren, _ := toAnySlice(primary["children"])
		all := make([]any, 0, len(existingChildren)+len(roots)-1)
		all = append(all, existingChildren...)
		for _, r := range roots[1:] {
			all = append(all, r)
		}
		primary["children"] = all
		data["root"] = primary
	}
	delete(data, "nodes")
}

// orgChartNodeIssues returns the structural issues recorded by
// normalizeOrgChartNodes plus one entry per node that has neither a name nor
// a title (an empty box — typically a node written with "label", which the
// org chart does not read).
func orgChartNodeIssues(data map[string]any, root *OrgNode) []string {
	var issues []string
	switch recorded := data[orgNodeIssuesKey].(type) {
	case []string:
		issues = append(issues, recorded...)
	case []any:
		for _, v := range recorded {
			if s, ok := v.(string); ok {
				issues = append(issues, s)
			}
		}
	}
	empty := countEmptyOrgNodes(root)
	if empty > 0 {
		issues = append(issues, fmt.Sprintf("%d node(s) have neither name nor title and draw as empty boxes; set name (and optionally title) — label is not read", empty))
	}
	return issues
}

// countEmptyOrgNodes counts nodes with neither a name nor a title.
func countEmptyOrgNodes(node *OrgNode) int {
	n := 0
	if strings.TrimSpace(node.Name) == "" && strings.TrimSpace(node.Title) == "" {
		n = 1
	}
	for i := range node.Children {
		n += countEmptyOrgNodes(&node.Children[i])
	}
	return n
}
