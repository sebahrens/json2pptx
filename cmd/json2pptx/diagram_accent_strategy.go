package main

import (
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// applyDiagramAccentStrategy hands the deck's accent_strategy to the diagrams
// that colour by level. An org chart draws its levels in the primary accent
// and its tints; only a deck that rotates accents ("rotate", "section-keyed")
// gives each level its own accent (go-slide-creator-libnz). An authored
// data.accent_strategy wins, and the authored spec is never mutated: the
// slide gets a copy.
func applyDiagramAccentStrategy(spec *generator.SlideSpec, strategy patterns.AccentStrategy) {
	if strategy != patterns.AccentStrategyRotate && strategy != patterns.AccentStrategySectionKeyed {
		return
	}
	for i := range spec.Content {
		diagram, ok := spec.Content[i].Value.(*types.DiagramSpec)
		if !ok || diagram == nil || diagram.Type != "org_chart" {
			continue
		}
		if _, authored := diagram.Data["accent_strategy"]; authored {
			continue
		}
		clone := *diagram
		clone.Data = make(map[string]any, len(diagram.Data)+1)
		for k, v := range diagram.Data {
			clone.Data[k] = v
		}
		clone.Data["accent_strategy"] = string(strategy)
		spec.Content[i].Value = &clone
	}
}
