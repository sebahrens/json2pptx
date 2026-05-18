package generator

import "github.com/sebahrens/json2pptx/internal/patterns"

// EnrichVisualPlacement annotates RecommendVisualResult candidates with placement
// guidance derived from the canonical diagram placement registry and known facts
// about chart/pattern/placeholder render paths. This bridges the gap between the
// category-level recommendation and the authoring path the agent should follow.
//
// For named-pattern candidates, ComposableWith is populated from the pattern's
// PatternTaxonomy.ComposesWith via the default pattern registry, exposing the
// per-pattern compose-axis to MCP consumers.
func EnrichVisualPlacement(result *patterns.RecommendVisualResult) {
	EnrichVisualPlacementWithRegistry(result, patterns.Default())
}

// EnrichVisualPlacementWithRegistry is like EnrichVisualPlacement but accepts
// an explicit registry, useful for tests that register synthetic patterns.
func EnrichVisualPlacementWithRegistry(result *patterns.RecommendVisualResult, reg *patterns.Registry) {
	for i := range result.Candidates {
		c := &result.Candidates[i]
		// Compose candidates carry their pair-specific ComposableWith population
		// from the scoring pass — preserve it instead of clobbering with a
		// generic guidance derived from category alone.
		if c.Category == patterns.VisualCategoryCompose && c.Placement != nil {
			continue
		}
		c.Placement = placementForCandidate(reg, c.Category, c.Name)
	}
}

func placementForCandidate(reg *patterns.Registry, category patterns.VisualCategory, name string) *patterns.PlacementGuidance {
	switch category {
	case patterns.VisualCategoryDiagram:
		return diagramPlacement(name)
	case patterns.VisualCategoryChart:
		return chartPlacement()
	case patterns.VisualCategoryPattern:
		return patternPlacement(reg, name)
	case patterns.VisualCategoryPlaceholder:
		return placeholderPlacement()
	case patterns.VisualCategoryShapeGrid:
		return shapeGridPlacement()
	case patterns.VisualCategoryCompose:
		// Fallback only — compose candidates emitted by the scorer carry
		// their pair-specific ComposableWith and bypass this branch.
		return composePlacement()
	default:
		return nil
	}
}

func composePlacement() *patterns.PlacementGuidance {
	// Compose envelopes expand into a single shape_grid (native_ooxml) hosted
	// in a placeholder. Without a pair-specific population this is the safe
	// generic guidance.
	return &patterns.PlacementGuidance{
		PreferredPlacement: "placeholder",
		HostStrategy:       "pattern_expansion",
		GridEmbeddable:     false,
		RenderPipeline:     "native_ooxml",
		ComposableWith:     []string{"named_pattern"},
	}
}

func diagramPlacement(diagramType string) *patterns.PlacementGuidance {
	info := DiagramPlacementFor(diagramType)
	if info == nil {
		// Unknown diagram — assume SVG, grid-embeddable.
		return &patterns.PlacementGuidance{
			PreferredPlacement: "placeholder",
			HostStrategy:       "placeholder_content",
			GridEmbeddable:     true,
			RenderPipeline:     "svg",
			ComposableWith:     []string{"named_pattern"},
		}
	}

	pg := &patterns.PlacementGuidance{
		RenderPipeline: info.PlaceholderPipeline,
		GridEmbeddable: info.GridCellPipeline != "",
	}

	// Native OOXML diagrams render best in placeholders (full fidelity).
	// They can fall back to SVG in grid cells, but placeholder is preferred.
	if info.PlaceholderPipeline == "native_ooxml" {
		pg.PreferredPlacement = "placeholder"
		pg.HostStrategy = "placeholder_content"
		if pg.GridEmbeddable {
			pg.ComposableWith = []string{"named_pattern"}
		}
	} else {
		// SVG diagrams work equally in both contexts.
		pg.PreferredPlacement = "either"
		pg.HostStrategy = "placeholder_content"
		pg.ComposableWith = []string{"named_pattern"}
	}

	return pg
}

func chartPlacement() *patterns.PlacementGuidance {
	// All charts render via SVG, work in both placeholders and grid cells.
	return &patterns.PlacementGuidance{
		PreferredPlacement: "either",
		HostStrategy:       "placeholder_content",
		GridEmbeddable:     true,
		RenderPipeline:     "svg",
		ComposableWith:     []string{"named_pattern"},
	}
}

func patternPlacement(reg *patterns.Registry, name string) *patterns.PlacementGuidance {
	// Named patterns expand into shape_grid; they are the grid themselves.
	pg := &patterns.PlacementGuidance{
		PreferredPlacement: "placeholder",
		HostStrategy:       "pattern_expansion",
		GridEmbeddable:     false,
		RenderPipeline:     "native_ooxml",
	}
	// Project the pattern's ComposesWith taxonomy axis onto ComposableWith so
	// agents see the exact sibling pattern names that can share a compose
	// envelope with this candidate.
	if reg != nil {
		if p, ok := reg.Get(name); ok {
			if cw := p.Taxonomy().ComposesWith; len(cw) > 0 {
				pg.ComposableWith = append(pg.ComposableWith[:0:0], cw...)
			}
		}
	}
	return pg
}

func placeholderPlacement() *patterns.PlacementGuidance {
	// Placeholder layouts are template-driven slide types.
	return &patterns.PlacementGuidance{
		PreferredPlacement: "placeholder",
		HostStrategy:       "standalone_slide",
		GridEmbeddable:     false,
		RenderPipeline:     "template_driven",
	}
}

func shapeGridPlacement() *patterns.PlacementGuidance {
	// Raw shape_grid is a manual layout in a placeholder.
	return &patterns.PlacementGuidance{
		PreferredPlacement: "placeholder",
		HostStrategy:       "grid_cell",
		GridEmbeddable:     false,
		RenderPipeline:     "native_ooxml",
		ComposableWith:     []string{"chart", "diagram"},
	}
}
