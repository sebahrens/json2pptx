package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

type layoutReviewText struct {
	path              string
	gridGap           float64
	pattern           string
	figure            bool
	geometry          string
	anchor            string
	frame, ink        pptx.RectEmu
	paragraphs        []geomParagraph
	baseline, widthPt float64
}
type layoutReviewVisual struct {
	path  string
	frame pptx.RectEmu
}
type layoutReviewPane struct {
	path  string
	frame pptx.RectEmu
}

func (a *geomAccumulator) recordLayoutText(cell shapegrid.ResolvedCell, path string, txt geomText, in [4]int64, widthPt float64) {
	if len(txt.paragraphs) == 0 || widthPt <= 0 || txt.rotated() || cell.Layer {
		return
	}
	if cell.AutofitScale > 0 && cell.AutofitScale < 1 {
		txt.paragraphs = append([]geomParagraph(nil), txt.paragraphs...)
		for i := range txt.paragraphs {
			txt.paragraphs[i].sizePt *= cell.AutofitScale
		}
	}
	w, h := a.m.textBlockPt(txt, widthPt)
	frame := cell.Bounds
	y := float64(frame.Y+in[1]) / emuPerPt
	available := float64(frame.CY-in[1]-in[3]) / emuPerPt
	switch txt.vAlign {
	case "top", "t":
	case "bottom", "b":
		y += math.Max(available-h, 0)
	default:
		y += math.Max((available-h)/2, 0)
	}
	x := frame.X + in[0]
	if txt.align == "ctr" || txt.align == "center" || txt.align == "" {
		x += int64(math.Max((widthPt-w)/2, 0) * emuPerPt)
	}
	if txt.align == "right" || txt.align == "r" {
		x += int64(math.Max(widthPt-w, 0) * emuPerPt)
	}
	a.reviewText = append(a.reviewText, layoutReviewText{path: path, gridGap: a.reviewGridGap, pattern: a.reviewPattern, figure: reviewFigureText(cell), geometry: cell.ShapeSpec.Geometry, anchor: txt.vAlign, frame: frame, ink: pptx.RectEmu{X: x, Y: int64(y * emuPerPt), CX: int64(w * emuPerPt), CY: int64(h * emuPerPt)}, paragraphs: txt.paragraphs, baseline: y + txt.paragraphs[0].sizePt*0.8, widthPt: widthPt})
}

func layoutReviewFinding(code, path, action, hint string, params map[string]any) patterns.FitFinding {
	if params == nil {
		params = map[string]any{}
	}
	params["hint"] = hint
	return patterns.FitFinding{ValidationError: patterns.ValidationError{Code: code, Path: path, Message: hint, Fix: &patterns.FixSuggestion{Kind: "review_layout", Params: params}}, Action: action}
}

func (a *geomAccumulator) layoutReviewFindings(safe pptx.RectEmu, patternName string) []patterns.FitFinding {
	out := a.baselineReviewFindings()
	out = append(out, a.paneFillReviewFindings()...)
	if len(a.reviewVisuals) == 1 && safe.CX > 0 && safe.CY > 0 {
		v := a.reviewVisuals[0]
		box := intersectRect(v.frame, safe)
		share := float64(box.CX) * float64(box.CY) / (float64(safe.CX) * float64(safe.CY))
		if share > 0 && share < 0.35 {
			out = append(out, layoutReviewFinding(patterns.ErrCodeExhibitTooSmall, v.path, "review", "The only evidence visual occupies less than 35% of the content area; enlarge its pane or give it a dedicated slide.", map[string]any{"visual_area_share": round1(share*100) / 100, "min_area_share": 0.35}))
		}
	}
	for _, text := range a.reviewText {
		if layoutReviewIsHeading(text) {
			if reason := headingOrphanReason(a.m, text.paragraphs[0], text.widthPt); reason != "" {
				out = append(out, layoutReviewFinding(patterns.ErrCodeTitleOrphan, text.path+"/shape/text", "info", reason+"; rewrite the heading or use a non-breaking space between a number and its unit.", nil))
			}
		}
	}
	out = append(out, a.detachedLabelFindings()...)
	out = append(out, a.gridHierarchyFindings()...)
	return out
}

func (a *geomAccumulator) baselineReviewFindings() []patterns.FitFinding {
	groups := map[string][]layoutReviewText{}
	var keys []string
	for _, text := range a.reviewText {
		cut := strings.LastIndex(text.path, "/cells/")
		if cut < 0 || len(text.paragraphs) == 0 || !layoutReviewSharedBaseline(text) {
			continue
		}
		p := text.paragraphs[0]
		if isIndexMarker(p.text) {
			continue
		}
		key := fmt.Sprintf("%s|%.1f|%t|%s|%s|%.0f", text.path[:cut], p.sizePt, p.bold, layoutReviewRole(text), text.geometry, float64(text.frame.CX)/emuPerPt)
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], text)
	}
	var out []patterns.FitFinding
	for _, key := range keys {
		group := groups[key]
		if len(group) < 2 {
			continue
		}
		low, high := group[0], group[0]
		for _, text := range group[1:] {
			if text.baseline < low.baseline {
				low = text
			}
			if text.baseline > high.baseline {
				high = text
			}
		}
		// Different row spans are different regions, not first-line peers.
		if math.Abs(float64(low.frame.Y-high.frame.Y))/emuPerPt > 2 || math.Abs(float64(low.frame.CY-high.frame.CY))/emuPerPt > 2 || high.baseline-low.baseline <= 2 {
			continue
		}
		f := layoutReviewFinding(patterns.ErrCodeBaselineMisalign, high.path+"/shape/text", "review", "Peer text starts on baselines more than 2pt apart; align the peers' text anchors and top insets.", map[string]any{"cells": []string{low.path, high.path}, "baseline_spread_pt": round1(high.baseline - low.baseline)})
		f.Fix = &patterns.FixSuggestion{Kind: "align_top_insets", Params: f.Fix.Params}
		out = append(out, f)
	}
	return out
}

func (a *geomAccumulator) paneFillReviewFindings() []patterns.FitFinding {
	type fill struct {
		pane  layoutReviewPane
		ratio float64
	}
	groups := map[string][]fill{}
	var keys []string
	for _, pane := range a.reviewPanes {
		area := float64(pane.frame.CX) * float64(pane.frame.CY)
		if area <= 0 {
			continue
		}
		ratio, comparable := a.prosePaneFill(pane)
		if !comparable {
			continue
		}
		cut := strings.LastIndex(pane.path, "/cells/")
		if cut < 0 {
			continue
		}
		key := pane.path[:cut]
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], fill{pane, ratio})
	}
	var out []patterns.FitFinding
	for _, key := range keys {
		group := groups[key]
		if len(group) < 2 {
			continue
		}
		low, high := group[0], group[0]
		for _, x := range group[1:] {
			if x.ratio < low.ratio {
				low = x
			}
			if x.ratio > high.ratio {
				high = x
			}
		}
		if high.ratio-low.ratio > 0.4 {
			out = append(out, layoutReviewFinding(patterns.ErrCodePaneFillImbalance, low.pane.path, "review", "Sibling prose panes differ in measured fill by more than 0.4; rebalance size_pct or distribute the text between the panes.", map[string]any{"cells": []string{low.pane.path, high.pane.path}, "fill_ratios": []float64{low.ratio, high.ratio}}))
		}
	}
	return out
}

func (a *geomAccumulator) detachedLabelFindings() []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, caption := range a.reviewText {
		if !knownFigurePattern(caption.pattern) || len(caption.paragraphs) != 1 || caption.paragraphs[0].bold || caption.paragraphs[0].sizePt > 14 {
			continue
		}
		var parent *layoutReviewText
		for i := range a.reviewText {
			candidate := &a.reviewText[i]
			if reviewGridPath(candidate.path) != reviewGridPath(caption.path) || len(candidate.paragraphs) == 0 || candidate.paragraphs[0].sizePt < 18 || (!candidate.figure && !isKPIValueText(candidate.paragraphs[0].text)) {
				continue
			}
			if candidate.ink.Y+candidate.ink.CY <= caption.ink.Y && math.Abs(float64(candidate.frame.X-caption.frame.X))/emuPerPt < 2 && candidate.frame.CX == caption.frame.CX {
				if parent == nil || candidate.ink.Y > parent.ink.Y {
					parent = candidate
				}
			}
		}
		if parent == nil {
			continue
		}
		gap := float64(caption.ink.Y-parent.ink.Y-parent.ink.CY) / emuPerPt
		limit := 1.5 * caption.gridGap
		if limit <= 0 {
			continue
		}
		for _, next := range a.reviewText {
			if reviewGridPath(next.path) == reviewGridPath(caption.path) && next.ink.Y >= caption.ink.Y+caption.ink.CY && next.frame.X == caption.frame.X && next.frame.CX == caption.frame.CX {
				limit = math.Min(limit, float64(next.ink.Y-caption.ink.Y-caption.ink.CY)/emuPerPt)
			}
		}
		if gap > limit {
			out = append(out, layoutReviewFinding(patterns.ErrCodeDetachedLabel, caption.path, "review", "A caption is detached from its KPI or hero figure; move it closer to its parent.", map[string]any{"parent": parent.path, "gap_pt": gap, "max_gap_pt": limit}))
		}
	}
	return out
}

func (a *geomAccumulator) prosePaneFill(pane layoutReviewPane) (float64, bool) {
	area := float64(pane.frame.CX) * float64(pane.frame.CY)
	if area <= 0 {
		return 0, false
	}
	prefix := pane.path + "/grid/"
	for _, visual := range a.reviewVisuals {
		if strings.HasPrefix(visual.path, prefix) {
			return 0, false
		}
	}
	textArea, anyText := 0.0, false
	for _, text := range a.reviewText {
		if !strings.HasPrefix(text.path, prefix) {
			continue
		}
		for _, p := range text.paragraphs {
			if p.sizePt >= 24 && isKPIValueText(p.text) {
				return 0, false
			}
		}
		anyText = true
		box := intersectRect(text.ink, pane.frame)
		textArea += float64(box.CX) * float64(box.CY)
	}
	return math.Min(textArea/area, 1), anyText
}

func knownFigurePattern(name string) bool {
	return strings.HasPrefix(name, "kpi-") || name == "hero-detail" || name == "stat-hero"
}
func reviewGridPath(path string) string {
	i := strings.LastIndex(path, "/rows/")
	if i < 0 {
		return path
	}
	return path[:i]
}

// Compare measured heading and prose only within their native grid. KPI
// figures and card subheads are not slide titles or prose peers.
func (a *geomAccumulator) gridHierarchyFindings() []patterns.FitFinding {
	type sizes struct {
		heading, body float64
		path          string
	}
	groups := map[string]sizes{}
	for _, text := range a.reviewText {
		if knownFigurePattern(text.pattern) {
			continue
		}
		key := reviewGridPath(text.path)
		g := groups[key]
		for _, p := range text.paragraphs {
			if isKPIValueText(p.text) || isIndexMarker(p.text) {
				continue
			}
			if p.bold && p.sizePt >= 18 {
				if g.heading == 0 || p.sizePt < g.heading {
					g.heading = p.sizePt
					g.path = text.path
				}
			} else if !p.bold && p.sizePt >= 12 && p.sizePt < 24 {
				g.body = math.Max(g.body, p.sizePt)
			}
		}
		groups[key] = g
	}
	var out []patterns.FitFinding
	for _, g := range groups {
		if g.heading > 0 && g.body > 0 && g.heading <= g.body+2 {
			out = append(out, layoutReviewFinding(patterns.ErrCodeTypeHierarchyFlat, g.path, "review", "The measured grid heading is within 2pt of its prose; increase heading contrast or reduce the body size while retaining readability.", map[string]any{"title_pt": g.heading, "body_pt": g.body}))
		}
	}
	return out
}

func reviewFigureText(cell shapegrid.ResolvedCell) bool {
	if cell.ShapeSpec == nil {
		return false
	}
	var obj struct {
		Figure     bool `json:"figure"`
		Paragraphs []struct {
			Figure bool `json:"figure"`
		} `json:"paragraphs"`
	}
	if json.Unmarshal(cell.ShapeSpec.Text, &obj) != nil {
		return false
	}
	if obj.Figure {
		return true
	}
	for _, p := range obj.Paragraphs {
		if p.Figure {
			return true
		}
	}
	return false
}

func layoutReviewRole(text layoutReviewText) string {
	if text.figure {
		return "figure"
	}
	if text.pattern == "text-sidebar" && len(text.paragraphs) == 1 {
		return "message"
	}
	if text.pattern == "state-shift-hub" && len(text.paragraphs) == 1 {
		return "node_label"
	}
	if len(text.paragraphs) > 0 && text.paragraphs[0].bold {
		if len(text.paragraphs) > 1 {
			return "heading_and_body"
		}
		return "label"
	}
	return "prose"
}
func layoutReviewIsHeading(text layoutReviewText) bool {
	if len(text.paragraphs) == 0 || !text.paragraphs[0].bold || text.paragraphs[0].sizePt < 18 {
		return false
	}
	switch layoutReviewRole(text) {
	case "figure", "message", "node_label":
		return false
	default:
		return true
	}
}

// These native cells deliberately center independent blocks against a photo,
// node, or process lane. Their block centers, not text lines, are peers.
func layoutReviewSharedBaseline(text layoutReviewText) bool {
	if text.anchor == "top" || text.anchor == "t" || text.anchor == "bottom" || text.anchor == "b" {
		return true
	}
	switch text.pattern {
	case "contact-directory", "state-shift-hub", "swimlane":
		return false
	default:
		return true
	}
}

// layoutReviewFindingsAtAuthoredPaths retains measured advisories while mapping
// generated pattern geometry to the nearest replaceable authored container.
func layoutReviewFindingsAtAuthoredPaths(input *PresentationInput, findings []patterns.FitFinding) []patterns.FitFinding {
	findings = chartFindingsAtAuthoredPaths(input, findings)
	if len(findings) == 0 {
		return findings
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return findings
	}
	var document any
	if json.Unmarshal(raw, &document) != nil {
		return findings
	}
	for i := range findings {
		fix := findings[i].Fix
		if fix == nil {
			continue
		}
		for _, key := range []string{"cells", "parent"} {
			switch paths := fix.Params[key].(type) {
			case string:
				fix.Params[key] = deepestAuthoredPointer(document, paths)
			case []string:
				mapped := make([]string, len(paths))
				for j, path := range paths {
					mapped[j] = deepestAuthoredPointer(document, path)
				}
				fix.Params[key] = mapped
			}
		}
	}
	return findings
}
