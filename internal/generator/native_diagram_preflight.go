package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
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
	default:
		return nativeDiagramDensityPreflight(spec, path)
	}
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
			Path:    path + ".data",
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
	colW := (width - 4*bmcGap) / 5
	topH := int64(float64(height) * bmcTopRowRatio)
	fullH := topH - bmcGap
	halfH := (fullH - bmcGap) / 2
	bottomH := height - topH
	var out []patterns.FitFinding
	for _, key := range bmcSectionOrder {
		sec := sections[key]
		title := sec.title
		if title == "" {
			title = bmcDefaultTitles[key]
		}
		body := strings.Join(sec.items, " • ")
		cellW, cellH := colW, fullH
		switch key {
		case bmcKeyActivities, bmcKeyResources, bmcCustRelations, bmcChannels:
			cellH = halfH
		case bmcCostStructure, bmcRevenueStreams:
			cellW, cellH = (width-bmcGap)/2, bottomH
		}
		headerH := int64(float64(cellH) * bmcHeaderHeightRatio)
		bodyH := cellH - headerH
		required := measureNativeText(body, font, float64(bmcBodyFontSize)/100, cellW-2*bmcBodyInset)
		if body != "" && required > bodyH {
			out = append(out, nativeTextCollisionFinding(spec.Type, path+".data."+string(key), title, body, required, bodyH))
		}
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
					Path:    fmt.Sprintf("%s.data.panels[%d]", path, i),
					Code:    "diagram.text_below_readable_min",
					Message: fmt.Sprintf("stat card value %q and caption %q exceed the measured four-card width; PowerPoint autofit would shrink the caption below the readable floor — shorten the caption/value or use fewer cards", hero, title),
					Fix:     &patterns.FixSuggestion{Kind: "reduce_text"},
				},
				Action: "review",
			})
		}
		required := measureNativeText(title, font, 16, cellW) + measureNativeText(body, font, 14, cellW)
		if required > cellH {
			out = append(out, nativeTextCollisionFinding(spec.Type, fmt.Sprintf("%s.data.panels[%d]", path, i), title, body, required, cellH))
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
			out = append(out, nativeTextCollisionFinding(spec.Type, fmt.Sprintf("%s.data.activities[%d]", path, i), panel.title, panel.body, required, cellH))
		}
	}
	return out
}
