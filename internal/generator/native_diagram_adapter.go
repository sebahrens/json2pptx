package generator

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// The bounded native-diagram adapter (go-slide-creator-3grgs).
//
// Every native diagram builder already takes a rectangle and a shape-ID base;
// none of them needs a placeholder. What tied them to body placeholders was
// the dispatch: twelve process*NativeShapes functions each read the
// placeholder's bounds, parsed the spec and registered an insert, and only
// SWOT and five forces had a second, grid-cell copy of that work. This file
// is the one seam both placements now share. layoutNativeDiagram parses a
// spec and lays it out in a rectangle; renderNativeInsert emits the group at a
// shape-ID base; nativeInsertShapeIDs says how many IDs that group consumes.
// The placeholder path feeds it the placeholder's bounds, the shape_grid /
// compose path the cell's — so a native diagram renders the same labels,
// values, fills and fonts wherever it sits.

// nativeDiagramEnv is the template context the native builders read.
type nativeDiagramEnv struct {
	fontName        string
	themeColors     []types.ThemeColor
	semanticAccents map[string]string
}

// nativeDiagramSite locates the authored diagram for the findings layout
// emits: the 0-based slide index and the diagram's JSON pointer.
type nativeDiagramSite struct {
	slideIndex int
	path       string
}

// nativeDiagramLayout is a native diagram laid out in a rectangle.
type nativeDiagramLayout struct {
	// insert carries the parsed panels, the mode flags and the (possibly
	// content-sized) bounds. placeholderIdx, altText and contentPath are the
	// caller's to fill.
	insert panelShapeInsert
	// panelLayout is the panel-family layout mode ("columns", "rows",
	// "stat_cards", "stylish_panels"), or "" for every other type. Panel
	// icons are overlaid per card in this mode's geometry.
	panelLayout string
	// findings and warnings are what laying the diagram out discovered
	// (a sparse framework, shortened heatmap labels, dropped KPI metrics).
	findings []patterns.FitFinding
	warnings []string
}

// nativeLayoutCount summarises the layout for the registration log.
func (l nativeDiagramLayout) logAttrs() []any {
	b := l.insert.bounds
	return []any{
		"type", l.insert.diagramType,
		"panels", len(l.insert.panels),
		"bounds", fmt.Sprintf("%dx%d+%d+%d", b.Width, b.Height, b.X, b.Y),
	}
}

// layoutNativeDiagram parses spec and lays it out in bounds with the native
// builders' own sizing rules. It is the single bounded adapter shared by the
// placeholder and the shape_grid / compose dispatch. An error means the spec
// has no native renderer or carries nothing to draw.
func layoutNativeDiagram(spec *types.DiagramSpec, bounds types.BoundingBox, env nativeDiagramEnv, site nativeDiagramSite) (nativeDiagramLayout, error) { //nolint:gocognit,gocyclo // one case per native type
	var out nativeDiagramLayout
	if spec == nil {
		return out, fmt.Errorf("no diagram spec")
	}
	ins := panelShapeInsert{bounds: bounds, diagramType: spec.Type}
	fit := func(kind string, panels []nativePanelData, meta houseDiagramMeta) {
		var f *patterns.FitFinding
		ins.bounds, f = fitNativeFrameworkAt(kind, ins.bounds, panels, meta, env.fontName, site)
		if f != nil {
			out.findings = append(out.findings, *f)
		}
	}

	switch {
	case isPanelNativeLayout(spec):
		panels, err := panelLayoutPanels(spec)
		if err != nil {
			return out, err
		}
		mode := panelLayoutMode(spec)
		ins.panels = panels
		ins.rowsMode = mode == "rows"
		ins.statCardsMode = mode == "stat_cards"
		ins.stylishPanelsMode = mode == "stylish_panels"
		out.panelLayout = mode

	case isSWOTDiagram(spec):
		ins.panels = swotPanels(spec)
		ins.swotMode = true
		ins.taxonomyTints = taxonomyPalette(spec, 4, swotDefaultTint)
		fit("swot", ins.panels, houseDiagramMeta{})

	case isPESTELDiagram(spec):
		// Supports a "segments" array or individual political/economic/...
		// keys (see parsePESTELSegments).
		ins.panels = parsePESTELSegments(spec.Data)
		if len(ins.panels) == 0 {
			return out, fmt.Errorf("pestel: no segments parsed")
		}
		ins.pestelMode = true
		ins.taxonomyTints = taxonomyPalette(spec, len(pestelSegmentColors), uniformTaxonomyTint)
		fit("pestel", ins.panels, houseDiagramMeta{})

	case isNineBoxDiagram(spec):
		ins.panels, _ = nineBoxPanels(spec)
		ins.nineBoxMode = true
		ins.nineBoxTints = nineBoxSemanticTints(env.semanticAccents)

	case isValueChainDiagram(spec):
		panels, meta := parseValueChainData(spec.Data)
		if len(panels) == 0 {
			return out, fmt.Errorf("value_chain: no activities parsed")
		}
		ins.panels = panels
		ins.valueChainMode = true
		ins.valueChainMeta = meta

	case isKPIDashboardDiagram(spec):
		// Accepts a "metrics" or "kpis" key.
		metrics := parseKPIMetrics(spec.Data)
		if len(metrics) == 0 {
			return out, fmt.Errorf("kpi_dashboard: no metrics found")
		}
		// Enforce the documented capacity instead of silently accepting any count.
		if len(metrics) > kpiMaxMetrics {
			dropped := len(metrics) - kpiMaxMetrics
			reason := fmt.Sprintf(
				"kpi_dashboard holds at most %d metrics (declared max_nodes); %d of %d were dropped — split across two slides",
				kpiMaxMetrics, dropped, len(metrics))
			out.findings = append(out.findings, patterns.ContentDropped(
				site.path, fmt.Sprintf("%d kpi_dashboard metrics", dropped), reason))
			out.warnings = append(out.warnings, reason)
			metrics = metrics[:kpiMaxMetrics]
		}
		for _, m := range metrics {
			ins.panels = append(ins.panels, nativePanelData{
				title: m.label,
				value: m.displayValue(),
				body:  buildKPIDeltaText(m.delta, m.trend), // delta with trend arrow prefix
			})
		}
		ins.kpiDashboardMode = true
		fit("kpi_dashboard", ins.panels, houseDiagramMeta{})

	case isPortersFiveForcesDiagram(spec):
		ins.panels = porterPanels(spec)
		if len(ins.panels) == 0 {
			return out, fmt.Errorf("porters_five_forces: no forces parsed")
		}
		ins.portersFiveMode = true

	case isBMCDiagram(spec):
		ins.panels = bmcPanels(spec)
		if ignored := BMCIgnoredKeys(spec.Data); len(ignored) > 0 {
			slog.Warn("native bmc: data keys are not canvas sections and are not drawn",
				"path", site.path, "keys", ignored)
		}
		ins.bmcMode = true
		ins.taxonomyTints = taxonomyPalette(spec, len(bmcSectionOrder), bmcDefaultTint)

	case isProcessFlowDiagram(spec):
		steps, connections, direction := parseProcessFlowDiagramData(spec.Data)
		if len(steps) == 0 {
			return out, fmt.Errorf("process_flow: no steps parsed")
		}
		// Steps and connections ride in the panel list; a connection is
		// marked by a "conn:" prefix in value.
		for _, s := range steps {
			ins.panels = append(ins.panels, nativePanelData{
				title: s.label,
				body:  s.description,
				value: fmt.Sprintf("%s:%s", s.stepType, s.id),
			})
		}
		for _, c := range connections {
			ins.panels = append(ins.panels, nativePanelData{
				title: c.label,
				value: fmt.Sprintf("conn:%s:%s:%s", c.from, c.to, c.style),
			})
		}
		ins.processFlowMode = true
		ins.processFlowMeta = processFlowMeta{
			fontName:        env.fontName,
			stepCount:       len(steps),
			connectionCount: len(connections),
			direction:       direction,
		}

	case isHeatmapDiagram(spec):
		parsed, err := parseHeatmapData(spec.Data)
		if err != nil {
			return out, fmt.Errorf("heatmap: %w", err)
		}
		if len(parsed.values) == 0 || len(parsed.values[0]) == 0 {
			return out, fmt.Errorf("heatmap: empty values grid")
		}
		numRows, numCols := len(parsed.values), len(parsed.values[0])
		// Labels are measured against the boxes they will actually be drawn
		// into before the meta is encoded (go-slide-creator-3rkpt).
		if shortened := fitHeatmapLabels(&parsed, bounds); shortened > 0 {
			if f := heatmapLabelFinding(numRows, numCols, shortened, site.path); f != nil {
				out.findings = append(out.findings, *f)
			}
			slog.Warn("native heatmap shapes: labels shortened to fit",
				"path", site.path, "rows", numRows, "cols", numCols, "labels", shortened)
		}
		// Panel 0 is the metadata; panels 1..N are the cells, row-major,
		// each value encoded as its title.
		ins.panels = append(ins.panels, nativePanelData{title: "__heatmap_meta__", body: encodeHeatmapMeta(parsed)})
		for row := 0; row < numRows; row++ {
			for col := 0; col < numCols; col++ {
				v := 0.0
				if col < len(parsed.values[row]) {
					v = parsed.values[row][col]
				}
				ins.panels = append(ins.panels, nativePanelData{title: formatHeatmapVal(v)})
			}
		}
		ins.heatmapMode = true
		ins.heatmapMeta = heatmapMeta{numRows: numRows, numCols: numCols, colorScale: parsed.colorScale}

	case isPyramidDiagram(spec):
		panels, err := pyramidPanels(spec)
		if err != nil {
			return out, fmt.Errorf("pyramid: %w", err)
		}
		if len(panels) == 0 {
			return out, fmt.Errorf("pyramid: no levels parsed")
		}
		ins.panels = panels
		ins.pyramidMode = true

	case isHouseDiagram(spec):
		panels, meta, err := parseHouseDiagramNativeData(spec.Data)
		if err != nil {
			return out, fmt.Errorf("house_diagram: %w", err)
		}
		if len(panels) == 0 {
			return out, fmt.Errorf("house_diagram: no panels parsed")
		}
		ins.panels = panels
		ins.houseDiagramMode = true
		ins.houseDiagramMeta = meta
		fit("house_diagram", panels, meta)

	default:
		return out, fmt.Errorf("diagram type %q has no native renderer", spec.Type)
	}

	out.insert = ins
	return out, nil
}

// panelLayoutPanels parses a panel-family spec's "panels" list, resolving
// each optional icon into native SVG markup (bundled name, inline svg_data,
// or data URI). Icons are embedded as asvg:svgBlip overlays, never rasterized.
func panelLayoutPanels(spec *types.DiagramSpec) ([]nativePanelData, error) {
	panelsRaw, ok := spec.Data["panels"].([]any)
	if !ok {
		return nil, fmt.Errorf("%s: missing or invalid 'panels' data", spec.Type)
	}
	iconDefaultFill := panelIconDefaultFill(panelLayoutMode(spec))
	var panels []nativePanelData
	for _, item := range panelsRaw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		panel := nativePanelData{}
		if title, ok := m["title"].(string); ok {
			panel.title = title
		}
		if value, ok := m["value"].(string); ok {
			panel.value = value
		}
		if body, ok := m["body"].(string); ok {
			panel.body = body
		}
		if icon, ok := m["icon"]; ok {
			if svg, alt, skip := resolvePanelIcon(icon, iconDefaultFill); len(svg) > 0 {
				panel.iconSVG = svg
				panel.iconAlt = alt
			} else if skip != "" {
				slog.Warn("panel native shapes: icon not embedded", "type", spec.Type, "reason", skip)
			}
		}
		panels = append(panels, panel)
	}
	if len(panels) == 0 {
		return nil, fmt.Errorf("%s: no panels parsed", spec.Type)
	}
	return panels, nil
}

// fitNativeFrameworkAt sizes a framework to its measured text while retaining
// the authored width and hanging the result from the region top. Dense
// frameworks keep their original geometry. A sparse finding describes the
// remaining lack of content, rather than suggesting a shape-grid-only repair
// for a diagram.
func fitNativeFrameworkAt(kind string, bounds types.BoundingBox, panels []nativePanelData, meta houseDiagramMeta, fontName string, site nativeDiagramSite) (types.BoundingBox, *patterns.FitFinding) {
	if bounds.Width <= 0 || bounds.Height <= 0 || len(panels) == 0 {
		return bounds, nil
	}
	size := nativeFrameworkSize(kind, bounds, panels, meta, fontName)
	if size.box <= 0 {
		return bounds, nil
	}
	f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex: site.slideIndex, Path: site.path,
		BoundsHeightEMU: bounds.Height, ContentHeightEMU: size.ink, AreaMeasured: size.ink == 0,
	})
	if f != nil {
		f.Pattern = kind
		f.Message = fmt.Sprintf("%s diagram text occupies %.0f%% of its allocated height; add supporting detail or use a smaller diagram region", kind, 100*float64(size.ink)/float64(bounds.Height))
		f.Fix = &patterns.FixSuggestion{Kind: "add_detail_or_resize", Params: map[string]any{
			"diagram_type": kind, "content_height": size.ink, "bounds_height": bounds.Height,
		}}
	}
	return fitBoundsToFramework(bounds, size), f
}

// renderNativeInsert emits ins as one <p:grpSp> whose shape IDs start at base
// and stay below base+nativeInsertShapeIDs(ins). The group carries no
// description; callers apply the alt text.
func renderNativeInsert(ins *panelShapeInsert, base uint32, env nativeDiagramEnv) string {
	switch {
	case ins.swotMode:
		return generateSWOTGroupXML(ins.panels, ins.bounds, base, ins.taxonomyTints)
	case ins.pestelMode:
		return generatePESTELGroupXML(ins.panels, ins.bounds, base, ins.taxonomyTints)
	case ins.valueChainMode:
		return generateValueChainGroupXML(ins.panels, ins.bounds, base, ins.valueChainMeta)
	case ins.nineBoxMode:
		return generateNineBoxGroupXML(ins.panels, ins.bounds, base, ins.nineBoxTints)
	case ins.kpiDashboardMode:
		return generateKPIDashboardGroupXML(ins.panels, ins.bounds, base, env.themeColors)
	case ins.portersFiveMode:
		return generatePortersFiveGroupXML(ins.panels, ins.bounds, base, env.themeColors)
	case ins.bmcMode:
		return generateBMCGroupXML(ins.panels, ins.bounds, base, ins.taxonomyTints)
	case ins.processFlowMode:
		return generateProcessFlowGroupXML(ins.panels, ins.bounds, base, ins.processFlowMeta)
	case ins.heatmapMode:
		return generateHeatmapGroupXML(ins.panels, ins.bounds, base, ins.heatmapMeta, env.themeColors)
	case ins.pyramidMode:
		return generatePyramidGroupXML(ins.panels, ins.bounds, base, env.fontName)
	case ins.houseDiagramMode:
		return generateHouseDiagramGroupXML(ins.panels, ins.bounds, base, ins.houseDiagramMeta)
	case ins.stylishPanelsMode:
		return generateStylishPanelsGroupXML(ins.panels, ins.bounds, base)
	case ins.rowsMode:
		return generatePanelRowsGroupXML(ins.panels, ins.bounds, base)
	case ins.statCardsMode:
		return generateStatCardsGroupXML(ins.panels, ins.bounds, base, env.fontName)
	default:
		return generatePanelGroupXML(ins.panels, ins.bounds, base, env.fontName)
	}
}

// nativeInsertShapeIDs is how many consecutive shape IDs renderNativeInsert
// consumes for ins: the group plus its children.
func nativeInsertShapeIDs(ins *panelShapeInsert) uint32 {
	switch {
	case ins.valueChainMode:
		// 1 (group) + N support bars + N primary chevrons + 1 margin (optional)
		n := uint32(ins.valueChainMeta.supportCount + ins.valueChainMeta.primaryCount + 1)
		if ins.valueChainMeta.marginLabel != "" {
			n++
		}
		return n
	case ins.nineBoxMode:
		// 1 (group) + 9×2 (label+body) + 8 (axis shapes max)
		return 27
	case ins.portersFiveMode:
		// 1 (group) + 5 force boxes + 4 connectors
		return 10
	case ins.bmcMode:
		// 1 (group) + 9×2 (header+body per cell) = 19
		return 19
	case ins.processFlowMode:
		// 1 (group) + N steps + M connectors + L labels
		return pfEstimateShapeCount(ins.panels)
	case ins.heatmapMode:
		// 1 (group) + R*C cells + R row labels + C col labels
		m := ins.heatmapMeta
		return uint32(m.numRows*m.numCols + m.numRows + m.numCols + 1)
	case ins.pyramidMode:
		// 1 (group) + N level shapes
		return pyramidEstimateShapeCount(ins.panels)
	case ins.houseDiagramMode:
		// 1 (group) + 1 (roof) + N (floor sections) + 1 (foundation)
		return houseDiagramEstimateShapeCount(ins.panels)
	case ins.stylishPanelsMode:
		// N accents + N bodies + 1 ribbon + N headers + 1 group
		return stylishPanelsEstimateShapeCount(ins.panels)
	case ins.statCardsMode, ins.kpiDashboardMode:
		// 1 (group) + N×1 (single rect per card)
		return uint32(len(ins.panels) + 1)
	default:
		// 1 (group) + N×2 (header + body), which covers SWOT and PESTEL too.
		return uint32(len(ins.panels)*2 + 1)
	}
}

// nativeInsertIDSpan is how many shape IDs from base the rendered group xml
// actually occupies: the larger of the builder's estimate and the highest
// cNvPr id it wrote. A few builders skip or add IDs past their estimate (the
// heatmap's colour scale, the gaps stat cards and rows leave), and a span
// shorter than the IDs drawn lets the next group reuse them.
func nativeInsertIDSpan(ins *panelShapeInsert, xml string, base uint32) uint32 {
	n := nativeInsertShapeIDs(ins)
	for _, m := range renderedShapeIDRE.FindAllStringSubmatch(xml, -1) {
		id, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil || uint32(id) < base {
			continue
		}
		if span := uint32(id) - base + 1; span > n {
			n = span
		}
	}
	return n
}

// processNativeDiagramShapes lays a native diagram out in the placeholder it
// targets and registers the group that replaces that placeholder. The group
// XML itself is generated in finalizePanelGroupXML, once shape IDs are known.
func (ctx *singlePassContext) processNativeDiagramShapes(slideNum, contentIdx int, item ContentItem, shapeIdx int) {
	spec, ok := item.Value.(*types.DiagramSpec)
	if !ok {
		slog.Warn("native diagram shapes: invalid diagram spec", "slide", slideNum)
		return
	}
	if ctx.themeOverride != nil && isPortersFiveForcesDiagram(spec) {
		slog.Warn("porters native shapes: themeOverride is set but scheme color refs won't reflect overrides",
			"slide", slideNum)
	}

	slide := ctx.templateSlideData[slideNum]
	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]
	bounds := getPlaceholderBounds(shape, nil)

	layout, err := layoutNativeDiagram(spec, bounds, ctx.nativeDiagramEnv(), nativeDiagramSite{
		slideIndex: slideNum - 1,
		path:       slidepath.ContentIndex(slideNum-1, contentIdx),
	})
	if err != nil {
		slog.Warn("native diagram shapes: not rendered", "slide", slideNum, "error", err)
		return
	}
	for _, f := range layout.findings {
		ctx.emitFitFinding(f)
	}
	for _, w := range layout.warnings {
		ctx.warnings = append(ctx.warnings, fmt.Sprintf("slide %d: %s", slideNum, w))
	}
	slog.Info("native diagram shapes: registered", append([]any{"slide", slideNum}, layout.logAttrs()...)...)

	ins := layout.insert
	ins.altText = diagramAltText(item)
	ins.placeholderIdx = shapeIdx
	ctx.panelShapeInserts[slideNum] = append(ctx.panelShapeInserts[slideNum], ins)

	// Panel icons are native SVG overlays (asvg:svgBlip) positioned over each
	// card. Registered here (not in the deferred group-XML pass) because the
	// card geometry is derived from the bounds, which are known now.
	if layout.panelLayout != "" {
		ctx.registerPanelIconInserts(slideNum, layout.panelLayout, ins.bounds, ins.panels)
	}
}

func (ctx *singlePassContext) nativeDiagramEnv() nativeDiagramEnv {
	return nativeDiagramEnv{
		fontName:        ctx.themeFontName,
		themeColors:     ctx.themeColors,
		semanticAccents: ctx.semanticAccents,
	}
}
