package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

const (
	nativePreflightWidthEMU  = int64(10 * 914400)
	nativePreflightHeightEMU = int64(5.4 * 914400)
)

// NativeDiagramPreflight measures text against the same fixed-font boxes used
// by native OOXML diagram builders. Width and height use authored diagram
// dimensions when present and otherwise approximate the standard content
// placeholder; the cell ratios exactly match the builders.
func NativeDiagramPreflight(spec *types.DiagramSpec, fontName, path string) []patterns.FitFinding {
	if spec == nil || !IsNativeDiagramType(spec) {
		return nil
	}
	width, height := nativePreflightWidthEMU, nativePreflightHeightEMU
	if spec.Width > 0 {
		width = int64(float64(spec.Width) / 72 * 914400)
	}
	if spec.Height > 0 {
		height = int64(float64(spec.Height) / 72 * 914400)
	}
	if fontName == "" {
		fontName = "Arial"
	}
	switch spec.Type {
	case "business_model_canvas":
		return nativeBMCPreflight(spec, fontName, path, width, height)
	case "panel_layout", "stat_cards":
		return nativePanelPreflight(spec, fontName, path, width, height)
	case "value_chain":
		return nativeValueChainPreflight(spec, fontName, path, width, height)
	case "heatmap":
		parsed, err := parseHeatmapData(spec.Data)
		if err != nil || len(parsed.values) == 0 || len(parsed.values[0]) == 0 {
			return nil
		}
		shortened := fitHeatmapLabels(&parsed, types.BoundingBox{Width: width, Height: height})
		if finding := heatmapLabelFinding(len(parsed.values), len(parsed.values[0]), shortened, path); finding != nil {
			return []patterns.FitFinding{*finding}
		}
		return nil
	case "process_flow":
		return nativeProcessFlowPreflight(spec, fontName, path, width, height)
	default:
		return nativeDiagramDensityPreflight(spec, path)
	}
}

func nativeProcessFlowPreflight(spec *types.DiagramSpec, font, path string, width, height int64) []patterns.FitFinding {
	out := nativeDiagramDensityPreflight(spec, path)
	steps, connections, direction := parseProcessFlowDiagramData(spec.Data)
	if len(steps) == 0 {
		return out
	}
	bounds := types.BoundingBox{Width: width, Height: height}
	layout := computeProcessFlowLayout(steps, connections, bounds, direction, font)
	for i, step := range steps {
		box := layout.steps[i]
		if step.stepType == pfDecisionType && step.description == "" {
			// Drawn unwrapped at a size fitted to the diamond.
			if !pfDecisionLabelFits(step.label, font, box.cx, box.cy) {
				need := int64(pfDecisionMinLabelSize) * 127 * 12 / 10 * int64(len(strings.Fields(step.label))) * 2
				out = append(out, nativeTextCollisionFinding(spec.Type, slidepath.Field(path, fmt.Sprintf("data.steps[%d]", i)), step.label, "", need, box.cy))
			}
			continue
		}
		usableW, availableH := pfTextArea(step, font, box.cx, box.cy)
		if usableW < 1 {
			usableW = 1
		}
		if availableH < 1 {
			availableH = 1
		}
		required := pfRequiredTextHeight(step, font, usableW)
		if required > availableH {
			out = append(out, nativeTextCollisionFinding(spec.Type, slidepath.Field(path, fmt.Sprintf("data.steps[%d]", i)), step.label, step.description, required, availableH))
		}
	}

	stepRects := make(map[string]pptx.RectEmu, len(steps))
	stepKinds := make(map[string]processFlowStepType, len(steps))
	for i, step := range steps {
		box := layout.steps[i]
		stepRects[step.id] = pptx.RectEmu{X: box.x, Y: box.y, CX: box.cx, CY: box.cy}
		stepKinds[step.id] = step.stepType
	}
	for i, connection := range connections {
		if connection.label == "" {
			continue
		}
		src, srcOK := stepRects[connection.from]
		tgt, tgtOK := stepRects[connection.to]
		if !srcOK || !tgtOK {
			continue
		}
		labelW, labelH := pfConnLabelSize(connection.label, font)
		label := pfConnLabelBounds(src, tgt, layout.direction, labelW, labelH, stepKinds[connection.from] == pfDecisionType)
		for id, stepRect := range stepRects {
			if pfRectHitsStep(label, stepRect, stepKinds[id]) {
				out = append(out, patterns.FitFinding{
					ValidationError: patterns.ValidationError{
						Pattern: spec.Type,
						Path:    slidepath.Field(path, fmt.Sprintf("data.connections[%d].label", i)),
						Code:    "diagram.text_overlap",
						Message: fmt.Sprintf("process-flow connection label %q overlaps a step box; enlarge the diagram, shorten the flow, or remove the label", connection.label),
						Fix:     &patterns.FixSuggestion{Kind: "reduce_items"},
					},
					Action: "review",
				})
				break
			}
		}
	}
	return out
}

func nativeRectsOverlap(a, b pptx.RectEmu) bool {
	return a.X < b.X+b.CX && a.X+a.CX > b.X && a.Y < b.Y+b.CY && a.Y+a.CY > b.Y
}

var nativeDiagramTextBudgets = map[string]int{
	"house_diagram":       800,
	"icon_columns":        500,
	"icon_rows":           500,
	"nine_box_talent":     400,
	"pestel":              700,
	"porters_five_forces": 700,
	"process_flow":        400,
	"pyramid":             600,
	"stat_cards":          120,
}

func nativeDiagramDensityPreflight(spec *types.DiagramSpec, path string) []patterns.FitFinding {
	budget, ok := nativeDiagramTextBudgets[spec.Type]
	if !ok {
		return nil
	}
	var labels []string
	collectNativeStrings(spec.Data, &labels)
	total, longest := 0, ""
	for _, label := range labels {
		total += len([]rune(label))
		if len([]rune(label)) > len([]rune(longest)) {
			longest = label
		}
	}
	if total <= budget {
		return nil
	}
	return []patterns.FitFinding{{
		ValidationError: patterns.ValidationError{
			Pattern: spec.Type,
			Path:    slidepath.Field(path, "data"),
			Code:    "diagram.text_overlap",
			Message: fmt.Sprintf("native %s has %d characters across %d labels; its measured shape boxes hold about %d before text overlaps (longest label %q) — shorten labels, reduce items, or enlarge the diagram", spec.Type, total, len(labels), budget, longest),
			Fix:     &patterns.FixSuggestion{Kind: "reduce_items"},
		},
		Action: "review",
	}}
}

func collectNativeStrings(value any, out *[]string) {
	switch v := value.(type) {
	case string:
		*out = append(*out, v)
	case []any:
		for _, item := range v {
			collectNativeStrings(item, out)
		}
	case map[string]any:
		for _, item := range v {
			collectNativeStrings(item, out)
		}
	}
}

func nativeTextCollisionFinding(diagramType, path, title, body string, required, available int64) patterns.FitFinding {
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: diagramType,
			Path:    path,
			Code:    "diagram.text_overlap",
			Message: fmt.Sprintf("native diagram text %q and %q need %.0fpt but their box provides %.0fpt; shorten the copy, reduce items, or enlarge the diagram", title, body, float64(required)/12700, float64(available)/12700),
			Fix:     &patterns.FixSuggestion{Kind: "reduce_items"},
		},
		Action: "review",
	}
}

func measureNativeText(text, font string, size float64, width int64) int64 {
	m, err := textfit.MeasureRun(text, font, size, width, 0)
	if err != nil {
		return 0
	}
	return m.RequiredEMU
}

func nativeBMCPreflight(spec *types.DiagramSpec, font, path string, width, height int64) []patterns.FitFinding {
	sections := parseBMCSections(spec.Data)
	cells := bmcCellRects(bmcPanels(spec), types.BoundingBox{Width: width, Height: height})
	out := bmcIgnoredKeyFindings(spec, path)
	for _, key := range bmcSectionOrder {
		sec := sections[key]
		title := sec.title
		if title == "" {
			title = bmcDefaultTitles[key]
		}
		body := strings.Join(sec.items, " • ")
		cellW, cellH := cells[key].w, cells[key].h
		headerH := cells[key].header
		bodyH := cellH - headerH
		required := measureNativeText(body, font, float64(bmcBodyFontSize)/100, cellW-2*bmcBodyInset)
		if body != "" && required > bodyH {
			out = append(out, nativeTextCollisionFinding(spec.Type, slidepath.Field(path, "data."+string(key)), title, body, required, bodyH))
		}
	}
	return out
}

// bmcIgnoredKeyFindings reports every data key the canvas never reads: its
// text would vanish from the slide with nothing to say so
// (go-slide-creator-b7qqg.17).
func bmcIgnoredKeyFindings(spec *types.DiagramSpec, path string) []patterns.FitFinding {
	ignored := BMCIgnoredKeys(spec.Data)
	if len(ignored) == 0 {
		return nil
	}
	prefix := "data."
	if _, nested := spec.Data["boxes"].(map[string]any); nested {
		prefix = "data.boxes."
	}
	// The documented spelling of the nine keys (capabilities RequiredFields).
	canonical := []string{"key_partners", "key_activities", "key_resources", "value_propositions",
		"customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"}
	out := make([]patterns.FitFinding, 0, len(ignored))
	for _, key := range ignored {
		params := map[string]any{"field": key, "diagram_type": spec.Type, "accepted": canonical}
		msg := fmt.Sprintf("business_model_canvas: data key %q is not a canvas section and is not drawn — its content never reaches the slide", key)
		if match, dist := ClosestMatch(key, bmcAliasKeysSorted(), 4); dist >= 0 {
			params["did_you_mean"] = match
			msg += fmt.Sprintf("; did you mean %q?", match)
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: spec.Type,
				Path:    slidepath.Field(path, prefix+key),
				Code:    "diagram.data_key_ignored",
				Message: msg,
				Fix:     &patterns.FixSuggestion{Kind: "review", Params: params},
			},
			Action: "review",
		})
	}
	return out
}

func nativePanelPreflight(spec *types.DiagramSpec, font, path string, width, height int64) []patterns.FitFinding {
	raw, _ := spec.Data["panels"].([]any)
	if len(raw) == 0 {
		return nil
	}
	cols := len(raw)
	if cols > 4 {
		cols = 4
	}
	rows := (len(raw) + cols - 1) / cols
	cellW, cellH := width/int64(cols), height/int64(rows)
	var out []patterns.FitFinding
	for i, value := range raw {
		panel, _ := value.(map[string]any)
		title, _ := panel["title"].(string)
		body, _ := panel["body"].(string)
		hero, _ := panel["value"].(string)
		if spec.Type == "stat_cards" && (len([]rune(title)) > 24 || len([]rune(hero)) > 14) {
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Pattern: spec.Type,
					Path:    slidepath.Field(path, fmt.Sprintf("data.panels[%d]", i)),
					Code:    "diagram.text_below_readable_min",
					Message: fmt.Sprintf("stat card value %q and caption %q exceed the measured four-card width; PowerPoint autofit would shrink the caption below the readable floor — shorten the caption/value or use fewer cards", hero, title),
					Fix:     &patterns.FixSuggestion{Kind: "reduce_text"},
				},
				Action: "review",
			})
		}
		required := measureNativeText(title, font, 16, cellW) + measureNativeText(body, font, 14, cellW)
		if required > cellH {
			out = append(out, nativeTextCollisionFinding(spec.Type, slidepath.Field(path, fmt.Sprintf("data.panels[%d]", i)), title, body, required, cellH))
		}
	}
	return out
}

func nativeValueChainPreflight(spec *types.DiagramSpec, font, path string, width, height int64) []patterns.FitFinding {
	panels, meta := parseValueChainData(spec.Data)
	count := max(1, meta.supportCount, meta.primaryCount)
	cellW := width / int64(count)
	cellH := height / 2
	var out []patterns.FitFinding
	for i, panel := range panels {
		required := measureNativeText(panel.title, font, 12, cellW) + measureNativeText(panel.body, font, 10, cellW)
		if required > cellH {
			out = append(out, nativeTextCollisionFinding(spec.Type, slidepath.Field(path, fmt.Sprintf("data.activities[%d]", i)), panel.title, panel.body, required, cellH))
		}
	}
	return out
}
