package generator

import "github.com/sebahrens/json2pptx/internal/patterns"

// EnrichVisualPlacement annotates RecommendVisualResult candidates with placement
// guidance derived from the canonical diagram placement registry and known facts
// about chart/pattern/placeholder render paths. This bridges the gap between the
// category-level recommendation and the authoring path the agent should follow.
func EnrichVisualPlacement(result *patterns.RecommendVisualResult) {
	for i := range result.Candidates {
		c := &result.Candidates[i]
		c.Placement = placementForCandidate(c.Category, c.Name)
	}
}

func placementForCandidate(category patterns.VisualCategory, name string) *patterns.PlacementGuidance {
	switch category {
	case patterns.VisualCategoryDiagram:
		return diagramPlacement(name)
	case patterns.VisualCategoryChart:
		return chartPlacement()
	case patterns.VisualCategoryPattern:
		return patternPlacement()
	case patterns.VisualCategoryPlaceholder:
		return placeholderPlacement()
	case patterns.VisualCategoryShapeGrid:
		return shapeGridPlacement()
	default:
		return nil
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

func patternPlacement() *patterns.PlacementGuidance {
	// Named patterns expand into shape_grid; they are the grid themselves.
	return &patterns.PlacementGuidance{
		PreferredPlacement: "placeholder",
		HostStrategy:       "pattern_expansion",
		GridEmbeddable:     false,
		RenderPipeline:     "native_ooxml",
		ComposableWith:     []string{"chart", "diagram"},
	}
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
