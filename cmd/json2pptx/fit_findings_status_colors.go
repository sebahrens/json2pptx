package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func collectStatusColorFindings(input *PresentationInput, theme *types.ThemeInfo) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	b, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	var value any
	if json.Unmarshal(b, &value) != nil {
		return nil
	}
	var out []patterns.FitFinding
	var walk func(any, string)
	walk = func(value any, path string) {
		switch value := value.(type) {
		case []any:
			for i, item := range value {
				walk(item, fmt.Sprintf("%s/%d", path, i))
			}
		case map[string]any:
			if p, ok := value["pattern"].(map[string]any); ok {
				out = append(out, patternStatusColorFindings(p, path+"/pattern", theme)...)
			}
			keys := make([]string, 0, len(value))
			for key := range value {
				if key != "pattern" {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				walk(value[key], path+"/"+key)
			}
		}
	}
	walk(value, "")
	return out
}

type statusReviewPayload struct {
	Criteria []patterns.TableHighlightCriterion `json:"criteria"`
	Options  []patterns.TableHighlightOption    `json:"options"`
	Scale    string                             `json:"scale"`
	Rows     []struct {
		Statuses []string `json:"statuses"`
	} `json:"rows"`
	RAGColors map[string]string `json:"rag_colors"`
}

func patternStatusColorFindings(p map[string]any, path string, theme *types.ThemeInfo) []patterns.FitFinding {
	name, _ := p["name"].(string)
	if name != "table-highlight" && name != "status-board" {
		return nil
	}
	b, err := json.Marshal(p["values"])
	if err != nil {
		return nil
	}
	var payload statusReviewPayload
	if json.Unmarshal(b, &payload) != nil {
		return nil
	}
	used := payload.usedColors(name)
	colors := map[string]string{"red": "#C62828", "amber": "#F2A900", "green": "#2E7D32"}
	palettePath := path + "/overrides/rag_colors"
	overrides := payload.RAGColors
	if name == "status-board" {
		colors = map[string]string{"red": "#B71C1C", "amber": "#946200", "green": "#2E7D32"}
		palettePath = path + "/values/rag_colors"
	} else {
		var o struct {
			Colors map[string]string `json:"rag_colors"`
		}
		raw, _ := json.Marshal(p["overrides"])
		_ = json.Unmarshal(raw, &o)
		overrides = o.Colors
	}
	for key, color := range overrides {
		colors[key] = color
	}
	var themeColors []types.ThemeColor
	if theme != nil {
		themeColors = theme.Colors
	}
	var selected []string
	for _, name := range []string{"red", "amber", "green"} {
		if used[name] {
			selected = append(selected, name)
		}
	}
	for i, left := range selected {
		for _, right := range selected[i+1:] {
			a, e1 := svggen.ParseColor(resolveColorRefToHex(colors[left], themeColors))
			b, e2 := svggen.ParseColor(resolveColorRefToHex(colors[right], themeColors))
			if e1 != nil || e2 != nil {
				continue
			}
			delta := statusColorDistance(a, b)
			if delta < 20 {
				return []patterns.FitFinding{layoutReviewFinding(patterns.ErrCodeStatusColorsIndistinct, palettePath, "review", "Two used status colours have CIE76 deltaE below 20; choose distinct status colours and retain the textual status keys.", map[string]any{"statuses": []string{left, right}, "colors": []string{colors[left], colors[right]}, "delta_e": delta, "minimum_delta_e": 20})}
			}
		}
	}
	return nil
}

func (p statusReviewPayload) usedColors(name string) map[string]bool {
	out := map[string]bool{}
	if name == "status-board" {
		for _, row := range p.Rows {
			for _, status := range row.Statuses {
				out[status] = true
			}
		}
		return out
	}
	for ci, criterion := range p.Criteria {
		scale := criterion.Scale
		if scale == "" {
			scale = p.Scale
		}
		if scale != "rag" {
			continue
		}
		for _, option := range p.Options {
			if ci >= len(option.Scores) {
				continue
			}
			rank, ok := patterns.TableHighlightScoreRank(option.Scores[ci], "rag")
			if !ok {
				continue
			}
			switch rank {
			case 0:
				out["red"] = true
			case 0.5:
				out["amber"] = true
			case 1:
				out["green"] = true
			}
		}
	}
	return out
}

// CIE76 on D65, matching the chart palette's existing perceptual metric.
func statusColorDistance(a, b svggen.Color) float64 {
	lab := func(c svggen.Color) [3]float64 {
		linear := func(v uint8) float64 {
			f := float64(v) / 255
			if f <= 0.04045 {
				return f / 12.92
			}
			return math.Pow((f+0.055)/1.055, 2.4)
		}
		r, g, b := linear(c.R), linear(c.G), linear(c.B)
		f := func(t float64) float64 {
			if t > 0.008856 {
				return math.Cbrt(t)
			}
			return 7.787*t + 16.0/116
		}
		x, y, z := f((r*0.4124+g*0.3576+b*0.1805)/0.95047), f(r*0.2126+g*0.7152+b*0.0722), f((r*0.0193+g*0.1192+b*0.9505)/1.08883)
		return [3]float64{116*y - 16, 500 * (x - y), 200 * (y - z)}
	}
	x, y := lab(a), lab(b)
	return math.Sqrt(math.Pow(x[0]-y[0], 2) + math.Pow(x[1]-y[1], 2) + math.Pow(x[2]-y[2], 2))
}
