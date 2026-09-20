package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Shape-grid readability (go-slide-creator-vbic).
//
// shape_grid text is written at its authored size (floored at 12pt) with
// <a:normAutofit/>; when the text exceeds the cell, the renderer shrinks every
// paragraph by the same factor — which is how real decks ended up with 6pt
// KPI deltas and 9pt chevron descriptions. The fit report predicts that
// shrink by measuring the cell's paragraphs in its text rectangle (the same
// rectangle textcapacity budgets) and reports the paragraph that lands
// furthest below the deck viewing_mode floor for its text role.

// collectReadabilityFindings returns TEXT_BELOW_READABLE_MIN findings for
// shape_grid cells in the deck.
func collectReadabilityFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []patterns.FitFinding {
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
	input, fromPattern := expandPatternsForFit(input, slideWidth, slideHeight, nil, layouts...)

	var findings []patterns.FitFinding
	for si, slide := range input.Slides {
		if slide.ShapeGrid == nil {
			continue
		}
		geom, _ := patternExpansionGeometry(slide, layouts, slideWidth, slideHeight, rhythm)
		result := resolveGridForStructural(slide.ShapeGrid, geom.OverrideBounds, geom.Zone, slideWidth, slideHeight)
		if result == nil {
			continue
		}
		densities := textcapacity.ForResolvedGrid(result)
		for i, cell := range result.Cells {
			if i >= len(densities) || cell.Kind != shapegrid.CellKindShape || cell.ShapeSpec == nil {
				continue
			}
			d := densities[i]
			if d.ActualChars == 0 || d.WidthEMU <= 0 || d.HeightEMU <= 0 {
				continue
			}
			paras := parseCellParagraphs(cell.ShapeSpec.Text)
			if len(paras) == 0 {
				continue
			}
			scale := predictedAutofitScale(paras, d.WidthEMU, d.HeightEMU)
			cellPath := slidepath.GridCell(si, cell.RowIdx, cell.ColIdx)
			path := slidepath.Join(cellPath, "shape/text")
			roleOverride := patternCellReadabilityRole(slide, cell.RowIdx)
			if f := worstReadability(paras, scale, mode, path, roleOverride); f != nil {
				// The generic finding suggests reduce_text, which cannot reach a
				// grid cell. Point it at the cell with its measured budget
				// (go-slide-creator-9zof).
				retargetCellReadabilityFix(f, cellPath, d.MaxChars)
				if fromPattern[si] {
					patternName := ""
					if slide.Pattern != nil {
						patternName = slide.Pattern.Name
					}
					f.Path = rerootPatternPath(f.Path, fromPattern)
					f.Fix = patternCellFix(f.Fix, patternName)
				}
				findings = append(findings, *f)
			}
		}
	}
	return findings
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

// predictedAutofitScale estimates the uniform font scale the renderer's
// shrink-on-overflow (<a:normAutofit/>) applies to fit all paragraphs of a cell
// into its text rectangle. It delegates to textcapacity, which owns the single
// implementation: the fit report's overflow verdict and this readability verdict
// must agree on the size text renders at (go-slide-creator-lmpu).
func predictedAutofitScale(paras []cellParagraph, widthEMU, heightEMU int64) float64 {
	specs := make([]textcapacity.ParagraphSpec, 0, len(paras))
	for _, p := range paras {
		specs = append(specs, textcapacity.ParagraphSpec{Text: p.text, FontPt: p.sizePt})
	}
	scale, _ := textcapacity.AutofitScaleFor(specs, widthEMU, heightEMU)
	return scale
}

// autofitShrinkReportThreshold is the predicted scale at or below which a
// shrink-driven readability finding is trustworthy.
//
// The shrink is a PREDICTION from an estimated cell box, and the estimate runs a
// little tight: on examples/phase-roadmap.json it predicts 12pt shrinking to
// ~10.6pt on cells that render at about 12pt. Reporting inside that error bar
// turned clean pattern decks into gate failures the moment pattern slides
// started being measured (go-slide-creator-adur). A prediction of 0.85 or
// harsher is well outside the error bar; text AUTHORED below the floor is exact
// and always reported.
const autofitShrinkReportThreshold = 0.85

// worstReadability returns the TEXT_BELOW_READABLE_MIN finding for the
// paragraph furthest below its role's floor after the predicted autofit
// scale, or nil when every paragraph stays readable.
func worstReadability(paras []cellParagraph, scale float64, mode tokens.ViewingMode, path string, roleOverride tokens.TextRole) *patterns.FitFinding {
	if scale < 1 && scale > autofitShrinkReportThreshold {
		return nil
	}
	var worst *patterns.FitFinding
	worstRatio := 1.0
	for _, p := range paras {
		role := cellTextRole(p.sizePt, p.bold, len([]rune(p.text)))
		if roleOverride != "" {
			role = roleOverride
		}
		effective := p.sizePt * scale
		minPt := float64(tokens.MinReadableHPt(mode, role)) / 100.0
		ratio := effective / minPt
		if ratio >= worstRatio {
			continue
		}
		ctx := fmt.Sprintf("authored %.0fpt", p.sizePt)
		if scale < 1 {
			ctx = fmt.Sprintf("cell text overflows; autofit shrinks %.0fpt to ~%.1fpt", p.sizePt, effective)
		}
		f := generator.NewReadabilityFinding(generator.ReadabilityFindingInput{
			Path:         path,
			Mode:         mode,
			Role:         role,
			EffectiveHPt: int(math.Round(effective * 100)),
			Paragraphs:   len(paras),
			Context:      ctx,
		})
		if f != nil {
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
// display-size text is a KPI value, bold text a card title, short text a
// caption (KPI labels, deltas, chips), anything else card body copy.
func cellTextRole(fontPt float64, bold bool, chars int) tokens.TextRole {
	switch {
	case fontPt >= 24:
		return tokens.TextRoleKPIValue
	case bold:
		return tokens.TextRoleCardTitle
	case chars <= 40:
		return tokens.TextRoleCaption
	default:
		return tokens.TextRoleCardBody
	}
}
