package slides

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Risk heat-map slides (go-slide-creator-ec74l).
//
// Six named risks on likelihood × impact had no kind: a 2 × 2 put every
// "medium" on an axis line, and the honest 3 × 3 needed the capability-heatmap
// pattern bent to the purpose. The payload here is what a risk author writes —
// the risks, each with a likelihood and an impact — and the compiler places
// them on the risk-heatmap pattern. A payload the pattern refuses (a level off
// the grid, more than 20 risks) becomes bullets that keep every risk and both
// of its ratings.

// riskHeatmapItem is one resolved risk, in the pattern's own field names.
type riskHeatmapItem struct {
	Name       string             `json:"name"`
	Likelihood patterns.RiskLevel `json:"likelihood"`
	Impact     patterns.RiskLevel `json:"impact"`
}

// riskHeatmapValues is the risk-heatmap pattern's values object.
type riskHeatmapValues struct {
	Items            []riskHeatmapItem `json:"items"`
	Size             int               `json:"size,omitempty"`
	LikelihoodLabel  string            `json:"likelihood_label,omitempty"`
	ImpactLabel      string            `json:"impact_label,omitempty"`
	LikelihoodLevels []string          `json:"likelihood_levels,omitempty"`
	ImpactLevels     []string          `json:"impact_levels,omitempty"`
}

// CompileRiskHeatmap compiles a risk heat map onto the risk-heatmap pattern,
// falling back to bullets when the pattern cannot place it.
func CompileRiskHeatmap(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	values := riskHeatmapPayload(in.Body)
	if in.wantsContent() || RiskHeatmapPattern(in.Body) == "" {
		return compileRiskHeatmapFallback(in, values)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal risk-heatmap values: %w", err)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "risk-heatmap", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.items",
		SemanticPath: in.semSlide() + ".items",
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileRiskHeatmapFallback lists each risk with its two ratings, because a
// risk stripped of where it sits on the axes has lost the point of the slide.
func compileRiskHeatmapFallback(in Input, v riskHeatmapValues) (*deckinput.SlideInput, []SourceLink, error) {
	likelihood := strings.ToLower(firstNonEmpty(v.LikelihoodLabel, "likelihood"))
	impact := strings.ToLower(firstNonEmpty(v.ImpactLabel, "impact"))
	bullets := make([]string, 0, len(v.Items))
	for _, it := range v.Items {
		var ratings []string
		if it.Likelihood != "" {
			ratings = append(ratings, likelihood+" "+string(it.Likelihood))
		}
		if it.Impact != "" {
			ratings = append(ratings, impact+" "+string(it.Impact))
		}
		line := it.Name
		if len(ratings) > 0 {
			line += " — " + strings.Join(ratings, ", ")
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	return contentFallback(in, "items", bullets)
}

// riskHeatmapPayload resolves the payload into the pattern's values. A risk is
// an object {name, likelihood, impact}; entries with no name are dropped.
func riskHeatmapPayload(body map[string]any) riskHeatmapValues {
	var v riskHeatmapValues
	if raw, ok := firstList(body, "items"); ok {
		for _, e := range raw {
			m, isMap := e.(map[string]any)
			if !isMap {
				continue
			}
			name := strings.TrimSpace(strField(m, "name"))
			if name == "" {
				continue
			}
			v.Items = append(v.Items, riskHeatmapItem{
				Name:       name,
				Likelihood: patterns.RiskLevel(strings.TrimSpace(strField(m, "likelihood"))),
				Impact:     patterns.RiskLevel(strings.TrimSpace(strField(m, "impact"))),
			})
		}
	}
	if raw := strings.TrimSpace(strField(body, "size")); raw != "" {
		// A size the grid does not have is the pattern's gate to name.
		if n, err := strconv.Atoi(raw); err == nil {
			v.Size = n
		} else {
			v.Size = -1
		}
	}
	v.LikelihoodLabel = strField(body, "likelihood_label")
	v.ImpactLabel = strField(body, "impact_label")
	v.LikelihoodLevels, _ = stringList(body, "likelihood_levels")
	v.ImpactLevels, _ = stringList(body, "impact_levels")
	return v
}

// RiskHeatmapPattern returns "risk-heatmap" when the payload compiles to the
// pattern and "" when it degrades to bullets; explain and validation read it
// so neither promises a visual compile will not emit.
func RiskHeatmapPattern(body map[string]any) string {
	if RiskHeatmapDegradeReason(body) != "" {
		return ""
	}
	return "risk-heatmap"
}

// RiskHeatmapDegradeReason explains why a payload will not render as the
// risk-heatmap pattern, or "" when it will.
func RiskHeatmapDegradeReason(body map[string]any) string {
	v := riskHeatmapPayload(body)
	if len(v.Items) == 0 {
		return "no usable risks"
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return "the risks cannot be encoded as pattern values"
	}
	if err := deckinput.ValidatePattern(&deckinput.PatternInput{Name: "risk-heatmap", Values: encoded}, patterns.Default()); err != nil {
		first := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
		return strings.TrimSpace(strings.TrimPrefix(first, "risk-heatmap:"))
	}
	return ""
}

// UsableRiskHeatmapItemCount returns how many risks survive extraction.
func UsableRiskHeatmapItemCount(body map[string]any) int { return len(riskHeatmapPayload(body).Items) }
