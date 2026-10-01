package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestApplyDiagramAccentStrategy covers the go-slide-creator-libnz plumbing:
// only a rotating deck strategy reaches an org chart's data, an authored
// value wins, other diagrams are untouched, and the authored spec is copied
// rather than mutated.
func TestApplyDiagramAccentStrategy(t *testing.T) {
	newSpec := func(data map[string]any) (*types.DiagramSpec, *types.DiagramSpec, generator.SlideSpec) {
		org := &types.DiagramSpec{Type: "org_chart", Data: data}
		other := &types.DiagramSpec{Type: "timeline", Data: map[string]any{}}
		return org, other, generator.SlideSpec{Content: []generator.ContentItem{{Value: org}, {Value: other}}}
	}

	for _, strategy := range []patterns.AccentStrategy{"", patterns.AccentStrategyPrimary} {
		org, _, spec := newSpec(map[string]any{"root": map[string]any{"name": "CEO"}})
		applyDiagramAccentStrategy(&spec, strategy)
		if got := spec.Content[0].Value.(*types.DiagramSpec); got != org || got.Data["accent_strategy"] != nil {
			t.Errorf("strategy %q: org chart changed: %+v", strategy, got.Data)
		}
	}

	org, other, spec := newSpec(map[string]any{"root": map[string]any{"name": "CEO"}})
	applyDiagramAccentStrategy(&spec, patterns.AccentStrategyRotate)
	got := spec.Content[0].Value.(*types.DiagramSpec)
	if got.Data["accent_strategy"] != "rotate" {
		t.Errorf("rotate strategy not passed to the org chart: %+v", got.Data)
	}
	if _, mutated := org.Data["accent_strategy"]; mutated {
		t.Error("the authored diagram spec was mutated")
	}
	if spec.Content[1].Value.(*types.DiagramSpec) != other || len(other.Data) != 0 {
		t.Error("a non-org diagram was changed")
	}

	_, _, spec = newSpec(map[string]any{"accent_strategy": "primary"})
	applyDiagramAccentStrategy(&spec, patterns.AccentStrategySectionKeyed)
	if spec.Content[0].Value.(*types.DiagramSpec).Data["accent_strategy"] != "primary" {
		t.Error("an authored accent_strategy was overwritten")
	}
}
