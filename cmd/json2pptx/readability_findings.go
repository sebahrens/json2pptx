package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Shape-grid readability (go-slide-creator-vbic).
//
// shape_grid text is written at its authored size with <a:normAutofit/>;
// when the text exceeds the cell, the renderer shrinks every
// paragraph by the same factor — which is how real decks ended up with 6pt
// KPI deltas and 9pt chevron descriptions. The fit report predicts that
// shrink using the shape emitter's stored scale (including text insets and
// paragraph spacing) and reports the paragraph that lands
// furthest below the deck viewing_mode floor for its text role.

// collectReadabilityFindings returns TEXT_BELOW_READABLE_MIN findings for
// shape_grid cells in the deck.
func collectReadabilityFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []patterns.FitFinding {
	return collectReadabilityFindingsWithTheme(input, layouts, slideWidth, slideHeight, nil)
}

// collectReadabilityFindingsWithTheme is collectReadabilityFindings with the
// template theme the renderer hands nested patterns: their content sizing
// measures text in the theme body font.
func collectReadabilityFindingsWithTheme(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo) []patterns.FitFinding {
	return collectReadability(input, layouts, slideWidth, slideHeight, theme, true)
}

// collectReadability is collectReadabilityFindingsWithTheme; budgets says
// whether to search each shared-cell finding for its field-level repair
// budget, which re-runs this check (without budgets) on a cut copy of the
// slide.
func collectReadability(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo, budgets bool) []patterns.FitFinding {
	authored := input
	mode := tokens.ParseViewingMode(input.ViewingMode)
	rhythm := resolvedValidRhythmGrid(input, layouts, slideWidth, slideHeight)
	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}
	// Expand pattern slides so the check sees the cells generation renders: a
	// slide-level pattern carries slide.Pattern, not ShapeGrid, so no pattern
	// slide was ever evaluated — and "6pt KPI deltas and 9pt chevron
	// descriptions", the motivating examples for this check, are pattern output
	// (go-slide-creator-adur).
	input, fromPattern := expandPatternsForFit(input, slideWidth, slideHeight, theme, layouts...)
	sectionIndices := slideSectionIndices(input.Slides, layouts)

	var findings []patterns.FitFinding
	for si, slide := range input.Slides {
		if slide.ShapeGrid == nil {
			continue
		}
		geom, contentBounds := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, rhythm)
		grid, nested, nestedErr := expandNestedPatternsForReadability(slide.ShapeGrid, slidepath.ShapeGrid(si), nestedExpansionGeometry{
			geom: geom, contentBounds: contentBounds, slideWidth: slideWidth, slideHeight: slideHeight,
			theme: theme, strategy: patterns.AccentStrategy(input.AccentStrategy), slideIdx: si, sectionIdx: sectionIndices[si],
		})
		// A nested pattern that refuses the cell it was given (a kpis region
		// too short for a value over its caption) stops generation; report it
		// here so validate does not approve the slide
		// (go-slide-creator-uj9zq).
		findings = append(findings, patternAreaRefusals(nestedErr, slidepath.ShapeGrid(si))...)
		result := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
		if result == nil {
			continue
		}
		for _, rc := range readabilityGridCells(grid, result, slidepath.ShapeGrid(si), slideWidth, slideHeight, 0) {
			cell := rc.cell
			if cell.Kind != shapegrid.CellKindShape || cell.ShapeSpec == nil || cell.Bounds.CX <= 0 || cell.Bounds.CY <= 0 {
				continue
			}
			paras := parseCellParagraphs(cell.ShapeSpec.Text)
			if len(paras) == 0 {
				continue
			}
			scale, err := writtenCellAutofitScale(cell)
			if err != nil {
				// Generation will report the invalid shape; do not invent a
				// readability measurement for XML that cannot be produced.
				continue
			}
			cellPath := rc.path
			path := slidepath.Join(cellPath, "shape/text")
			roleOverride := patternCellReadabilityRole(slide, cell.RowIdx)
			if f := worstReadability(paras, scale, mode, path, roleOverride); f != nil {
				// Character-budget shaping is needed only for an actual finding,
				// not every readable cell inspected by previews/recommendations.
				maxChars, usable := populatedCellBudget(cell)
				if !usable {
					continue
				}
				// The generic finding suggests reduce_text, which cannot reach a
				// grid cell. Point it at the cell with its measured budget
				// (go-slide-creator-9zof).
				retargetCellReadabilityFix(f, cellPath, maxChars)
				// Generation refuses exactly this outcome: it reads the same
				// stored autofit scale off the same emitted shape and rejects
				// any role-tagged grid paragraph below its floor
				// (generator.reportGridReadability). An advisory here let
				// validate approve a deck render then refused
				// (go-slide-creator-b7qqg.3).
				f.Action = "refuse"
				rerootReadabilityFinding(f, slide, fromPattern[si], fromPattern, nested)
				if budgets {
					// max_chars is the whole cell's budget; name the paragraph
					// whose cut actually clears the floor, with its own budget
					// (go-slide-creator-ifcng).
					fieldRepairBudget(f, fieldRepairScope{
						authored: authored, slideIdx: si, layouts: layouts,
						slideWidth: slideWidth, slideHeight: slideHeight, theme: theme,
					}, paras)
				}
				findings = append(findings, *f)
			}
		}
	}
	return findings
}

// rerootReadabilityFinding points a finding measured on an expanded pattern
// (slide-level or nested in a grid cell) at the authored pattern and tags its
// fix with the pattern name.
func rerootReadabilityFinding(f *patterns.FitFinding, slide SlideInput, slideFromPattern bool, fromPattern map[int]bool, nested nestedPatternCells) {
	if path, name, ok := nested.reroot(f.Path); ok {
		f.Path = path
		f.Fix = patternCellFix(f.Fix, name)
		return
	}
	if !slideFromPattern {
		return
	}
	patternName := ""
	if slide.Pattern != nil {
		patternName = slide.Pattern.Name
	}
	f.Path = rerootPatternPath(f.Path, fromPattern)
	f.Fix = patternCellFix(f.Fix, patternName)
}

// nestedPatternCells maps the JSON pointer of each grid cell that authored a
// nested pattern to that pattern's name.
type nestedPatternCells map[string]string

// reroot rewrites a finding path inside an expanded nested pattern
// (<cell>/grid/...) to the authored pattern (<cell>/pattern/...), the way
// rerootPatternPath does for a slide-level pattern, and returns the pattern
// name. The deepest matching cell wins.
func (n nestedPatternCells) reroot(path string) (string, string, bool) {
	best := ""
	for cell := range n {
		if strings.HasPrefix(path, cell+"/grid/") && len(cell) > len(best) {
			best = cell
		}
	}
	if best == "" {
		return path, "", false
	}
	return best + "/pattern" + strings.TrimPrefix(path, best+"/grid"), n[best], true
}

// shapeSources returns sources with every shape that a nested pattern wrote
// pointed at the authored pattern (<cell>/pattern/...) instead of the grid it
// expanded to, and without a text spec: the author wrote the pattern's
// values, not that text, so no cell-level colour repair applies. sources is
// not modified.
func (n nestedPatternCells) shapeSources(sources []gridShapeSource) []gridShapeSource {
	if len(n) == 0 {
		return sources
	}
	out := make([]gridShapeSource, len(sources))
	for i, src := range sources {
		if path, _, ok := n.reroot(src.Path); ok {
			src = gridShapeSource{Path: path}
		}
		out[i] = src
	}
	return out
}

// nestedExpansionGeometry is the expansion context generation hands patterns
// nested in a slide grid's cells.
type nestedExpansionGeometry struct {
	geom                    GridGeometry
	contentBounds           pptx.RectEmu
	slideWidth, slideHeight int64
	theme                   *types.ThemeInfo
	strategy                patterns.AccentStrategy
	slideIdx, sectionIdx    int
}

// expandNestedPatternsForReadability expands patterns nested in a slide grid's
// cells (a DeckSpec regions slide's stat-hero or timeline) on a copy of the
// grid, at the content rectangle and expansion context generation uses
// (convertSinglePresentationSlide → expandNestedCellPatternsInBounds).
// Generation measures and refuses the text those patterns write; skipping
// them let validate approve a regions slide that render refused
// (go-slide-creator-fn2ka). The source grid is never mutated. When nothing is
// nested, or expansion fails, the source grid is returned with no nested
// cells; the expansion error is returned for the caller to report.
func expandNestedPatternsForReadability(grid *ShapeGridInput, base string, g nestedExpansionGeometry) (*ShapeGridInput, nestedPatternCells, error) {
	if !hasNestedCellPattern(grid) {
		return grid, nil, nil
	}
	cloned := nestedPatternExpansionCopy(grid)
	ctx := slideExpandContext(g.theme, nil, g.geom.Zone,
		patterns.LayoutBounds{X: g.contentBounds.X, Y: g.contentBounds.Y, Width: g.contentBounds.CX, Height: g.contentBounds.CY},
		g.slideWidth, g.slideHeight, g.strategy, g.slideIdx, g.sectionIdx)
	if err := expandNestedCellPatternsInBounds(cloned, ctx, g.contentBounds, patterns.Default(), true); err != nil {
		return grid, nil, err
	}
	nested := nestedPatternCells{}
	collectNestedPatternCells(grid, base, nested, 0)
	return cloned, nested, nil
}

func collectNestedPatternCells(grid *ShapeGridInput, base string, out nestedPatternCells, depth int) {
	if grid == nil || depth > maxGeomNestingDepth {
		return
	}
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			path := fmt.Sprintf("%s/rows/%d/cells/%d", base, ri, ci)
			if len(cell.Pattern) > 0 {
				var p struct {
					Name string `json:"name"`
				}
				_ = json.Unmarshal(cell.Pattern, &p)
				out[path] = p.Name
			}
			collectNestedPatternCells(cell.Grid, path+"/grid", out, depth+1)
		}
	}
}

// readabilityGridCell is one resolved grid cell with its JSON-pointer path.
type readabilityGridCell struct {
	cell shapegrid.ResolvedCell
	path string
}

// readabilityGridCells flattens a resolved grid the way generation does
// (renderNestedSubGrids): nested sub-grids resolve inside their placeholder
// bounds less subGridInsetEMU, and their shape cells are checked too.
// Generation refuses unreadable text in a nested cell (a stacked
// chart-insights-split column, for example), so the fit report must predict
// it there as well (go-slide-creator-bzh34).
func readabilityGridCells(grid *ShapeGridInput, result *shapegrid.ResolveResult, base string, slideWidth, slideHeight int64, depth int) []readabilityGridCell {
	var out []readabilityGridCell
	for _, rc := range result.Cells {
		path := fmt.Sprintf("%s/rows/%d/cells/%d", base, rc.RowIdx, rc.ColIdx)
		if rc.Kind != shapegrid.CellKindSubGrid {
			out = append(out, readabilityGridCell{cell: rc, path: path})
			continue
		}
		src := gridCellAtResolved(grid, rc.RowIdx, rc.ColIdx)
		if src == nil || src.Grid == nil || depth >= maxGeomNestingDepth {
			continue
		}
		inset := pptx.RectEmu{X: rc.Bounds.X + subGridInsetEMU, Y: rc.Bounds.Y + subGridInsetEMU, CX: rc.Bounds.CX - 2*subGridInsetEMU, CY: rc.Bounds.CY - 2*subGridInsetEMU}
		if inset.CX <= 0 || inset.CY <= 0 {
			inset = rc.Bounds
		}
		if sub := resolveGridForStructural(src.Grid, &inset, nil, slideWidth, slideHeight); sub != nil {
			out = append(out, readabilityGridCells(src.Grid, sub, path+"/grid", slideWidth, slideHeight, depth+1)...)
		}
	}
	return out
}

func populatedCellBudget(cell shapegrid.ResolvedCell) (int, bool) {
	d := textcapacity.ForResolvedGrid(&shapegrid.ResolveResult{Cells: []shapegrid.ResolvedCell{cell}})[0]
	return d.MaxChars, d.ActualChars > 0 && d.WidthEMU > 0 && d.HeightEMU > 0
}

// Measure the same shape body generation writes. A density rectangle has
// already subtracted authored padding, so measuring it through textcapacity
// subtracts default padding a second time. Reusing the shape emitter also
// preserves paragraph spacing, overlay insets and the final resolved bounds.
func writtenCellAutofitScale(cell shapegrid.ResolvedCell) (float64, error) {
	data, err := shapegrid.GenerateCellShapeXML(cell)
	if err != nil {
		return 0, err
	}
	var emitted struct {
		TextBody struct {
			BodyProperties struct {
				Autofit struct {
					FontScale int `xml:"fontScale,attr"`
				} `xml:"normAutofit"`
			} `xml:"bodyPr"`
		} `xml:"txBody"`
	}
	if err := xml.Unmarshal(data, &emitted); err != nil {
		return 0, err
	}
	if scale := emitted.TextBody.BodyProperties.Autofit.FontScale; scale > 0 {
		return float64(scale) / 100000, nil
	}
	return 1, nil
}

// Resolve the same paragraphs as the writer, retaining blank paragraphs and
// inline run boundaries so roles cannot slide onto a neighbouring paragraph.
func resolvedCellTextRoles(cell shapegrid.ResolvedCell, override tokens.TextRole) []tokens.TextRole {
	if cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0 {
		return nil
	}
	body, err := shapegrid.ResolveTextInput(cell.ShapeSpec.Text)
	if err != nil {
		return nil // shape generation reports invalid text before publication
	}
	roles := make([]tokens.TextRole, len(body.Paragraphs))
	populated := 0
	for _, p := range body.Paragraphs {
		for _, run := range p.Runs {
			if strings.TrimSpace(run.Text) != "" {
				populated++
				break
			}
		}
	}
	for i, p := range body.Paragraphs {
		var text strings.Builder
		bold := true
		size := 0
		for _, run := range p.Runs {
			if strings.TrimSpace(run.Text) == "" {
				continue
			}
			text.WriteString(run.Text)
			bold = bold && run.Bold
			size = max(size, run.FontSize)
		}
		if text.Len() == 0 {
			continue
		}
		roles[i] = cellTextRole(float64(size)/100, bold, text.String(), populated)
		if override != "" {
			roles[i] = override
		}
	}
	return roles
}

// patternCellReadabilityRole applies a pattern's semantic role when font size
// alone is misleading. The pull-quote's 36pt quote row is prose, not a KPI;
// both named and editable expanded patterns carry the source stamp.
func patternCellReadabilityRole(slide SlideInput, row int) tokens.TextRole {
	if row == 0 && slide.ShapeGrid != nil && slide.ShapeGrid.Source == "pattern:pull-quote" {
		return tokens.TextRoleBody
	}
	return ""
}

// cellParagraph is one paragraph of shape_grid cell text at its rendered
// (floored) size.
type cellParagraph struct {
	text   string
	sizePt float64
	bold   bool
}

// renderDefaultCellPt mirrors the shape_grid renderer's default text size
// (shapegrid defaultTextSizeHPt) for text without an authored size.
const renderDefaultCellPt = 14.0

// parseCellParagraphs reads a shape_grid text payload (string shorthand,
// object, or paragraphs array) into paragraphs at their rendered sizes.
func parseCellParagraphs(raw json.RawMessage) []cellParagraph {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return splitCellParagraphs(s, renderDefaultCellPt, false)
	}
	var obj struct {
		Content    string  `json:"content"`
		Size       float64 `json:"size"`
		Bold       bool    `json:"bold"`
		Paragraphs []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size"`
			Bold    *bool   `json:"bold"`
		} `json:"paragraphs"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	base := renderDefaultCellPt
	if obj.Size > 0 {
		base = shapegrid.EffectiveTextSizePt(obj.Size)
	}
	if len(obj.Paragraphs) == 0 {
		return splitCellParagraphs(obj.Content, base, obj.Bold)
	}
	var out []cellParagraph
	for _, p := range obj.Paragraphs {
		size := base
		if p.Size > 0 {
			size = shapegrid.EffectiveTextSizePt(p.Size)
		}
		bold := obj.Bold
		if p.Bold != nil {
			bold = *p.Bold
		}
		out = append(out, splitCellParagraphs(p.Content, size, bold)...)
	}
	return out
}

func splitCellParagraphs(content string, sizePt float64, bold bool) []cellParagraph {
	var out []cellParagraph
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, cellParagraph{text: line, sizePt: sizePt, bold: bold})
	}
	return out
}

// worstReadability returns the TEXT_BELOW_READABLE_MIN finding for the
// paragraph furthest below its role's floor after the predicted autofit
// scale, or nil when every paragraph stays readable.
func worstReadability(paras []cellParagraph, scale float64, mode tokens.ViewingMode, path string, roleOverride tokens.TextRole) *patterns.FitFinding {
	var worst *patterns.FitFinding
	worstRatio := 1.0
	populated := populatedCellParagraphs(paras)
	for _, p := range paras {
		role := cellTextRole(p.sizePt, p.bold, p.text, populated)
		if roleOverride != "" {
			role = roleOverride
		}
		// The same integer arithmetic generation uses on the written run size
		// and stored fontScale (generator.reportGridReadability), so a
		// paragraph a hair under its floor is refused by both or neither.
		effectiveHPt := int(math.Round(p.sizePt*100) * math.Round(scale*100000) / 100000)
		effective := float64(effectiveHPt) / 100
		minPt := float64(tokens.MinReadableHPt(mode, role)) / 100.0
		ratio := effective / minPt
		if ratio >= worstRatio {
			continue
		}
		ctx := fmt.Sprintf("authored %.0fpt", p.sizePt)
		measurementSource := "authored"
		if scale < 1 {
			ctx = fmt.Sprintf("cell text overflows; autofit shrinks %.0fpt to ~%.1fpt", p.sizePt, effective)
			measurementSource = "predicted"
		}
		f := generator.NewReadabilityFinding(generator.ReadabilityFindingInput{
			Path:              path,
			Mode:              mode,
			Role:              role,
			EffectiveHPt:      effectiveHPt,
			Paragraphs:        len(paras),
			Context:           ctx,
			MeasurementSource: measurementSource,
		})
		if f != nil {
			// The paragraph text lets a DeckSpec caller name the authored
			// field behind a generated cell (go-slide-creator-b7qqg.3).
			f.Fix.Params["paragraph_text"] = strings.TrimSpace(p.text)
			worst, worstRatio = f, ratio
		}
	}
	return worst
}

// retargetCellReadabilityFix rewrites a readability finding's fix so it names a
// directive that can edit grid-cell text, keeping the readability params that
// explain the verdict.
func retargetCellReadabilityFix(f *patterns.FitFinding, cellPath string, maxChars int) {
	if f == nil || f.Fix == nil || f.Fix.Kind != "reduce_text" {
		return
	}
	params := map[string]any{"cell_path": cellPath}
	for k, v := range f.Fix.Params {
		params[k] = v
	}
	if maxChars > 1 {
		params["max_chars"] = maxChars
	}
	f.Fix = &patterns.FixSuggestion{Kind: "reduce_cell_text", Params: params}
}

// cellTextRole infers the text role of a shape_grid paragraph from its style:
// display-size text stating a figure is a KPI value, a lone index marker an
// axis/step caption, bold text a card title, short text a caption (KPI
// labels, deltas, chips), anything else card body copy. populated counts the
// cell's non-blank paragraphs.
func cellTextRole(fontPt float64, bold bool, text string, populated int) tokens.TextRole {
	switch {
	case populated == 1 && isIndexMarker(strings.TrimSpace(text)):
		return tokens.TextRoleCaption
	case fontPt >= 24 && isKPIValueText(text):
		return tokens.TextRoleKPIValue
	case bold:
		return tokens.TextRoleCardTitle
	case len([]rune(text)) <= 40:
		return tokens.TextRoleCaption
	default:
		return tokens.TextRoleCardBody
	}
}

// isKPIValueText reports whether display-size text states a figure. Size
// alone also matches display words ("Domestic" at 28pt is a heading) and lone
// index markers such as the sovereign-ai-strategy matrix axis numbers, which
// cellTextRole classifies as captions first. A marker accompanied by a caption
// paragraph ("4" over "new markets") is a KPI however short.
func isKPIValueText(text string) bool {
	return strings.ContainsAny(text, "0123456789")
}

// isIndexMarker matches a bare one- or two-digit ordinal: "1", "04", "3.",
// "(2)", "#5".
func isIndexMarker(t string) bool {
	t = strings.TrimPrefix(strings.TrimPrefix(t, "#"), "(")
	t = strings.TrimSuffix(strings.TrimSuffix(t, "."), ")")
	if len(t) == 0 || len(t) > 2 {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// populatedCellParagraphs counts paragraphs carrying visible text.
func populatedCellParagraphs(paras []cellParagraph) int {
	n := 0
	for _, p := range paras {
		if strings.TrimSpace(p.text) != "" {
			n++
		}
	}
	return n
}
