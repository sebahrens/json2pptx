package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

func TestDiagramPlacementFor_NativeIntercept(t *testing.T) {
	// porters_five_forces is native-intercepted in placeholder context.
	info := DiagramPlacementFor("porters_five_forces")
	if info == nil {
		t.Fatal("expected placement info for porters_five_forces")
	}
	if info.PlaceholderPipeline != "native_ooxml" {
		t.Errorf("placeholder pipeline = %q, want native_ooxml", info.PlaceholderPipeline)
	}
	if info.GridCellPipeline != "svg" {
		t.Errorf("grid cell pipeline = %q, want svg", info.GridCellPipeline)
	}
	if info.AuthoringSurface != "native_ooxml" {
		t.Errorf("authoring surface = %q, want native_ooxml", info.AuthoringSurface)
	}
}

func TestDiagramPlacementFor_SVGOnly(t *testing.T) {
	// timeline is SVG-only in both contexts.
	info := DiagramPlacementFor("timeline")
	if info == nil {
		t.Fatal("expected placement info for timeline")
	}
	if info.PlaceholderPipeline != "svg" {
		t.Errorf("placeholder pipeline = %q, want svg", info.PlaceholderPipeline)
	}
	if info.GridCellPipeline != "svg" {
		t.Errorf("grid cell pipeline = %q, want svg", info.GridCellPipeline)
	}
	if info.AuthoringSurface != "svggen" {
		t.Errorf("authoring surface = %q, want svggen", info.AuthoringSurface)
	}
}

func TestDiagramPlacementFor_Unknown(t *testing.T) {
	info := DiagramPlacementFor("nonexistent_type")
	if info != nil {
		t.Error("expected nil for unknown diagram type")
	}
}

func TestDiagramPlacementFor_GridCellEmbeddable(t *testing.T) {
	// Verify that a diagram embeddable in shape_grid has non-empty GridCellPipeline.
	info := DiagramPlacementFor("venn")
	if info == nil {
		t.Fatal("expected placement info for venn")
	}
	if info.GridCellPipeline == "" {
		t.Error("expected non-empty grid cell pipeline for venn (shape_grid embeddable)")
	}
}

func TestApplyPlacementMetadata_EnrichesCapabilities(t *testing.T) {
	caps := svggen.DiagramCapabilities()
	enriched := ApplyPlacementMetadata(caps)

	if len(enriched) != len(caps) {
		t.Fatalf("enriched length = %d, want %d", len(enriched), len(caps))
	}

	// Check that every enriched capability has Placements set.
	for _, c := range enriched {
		if len(c.Placements) == 0 {
			t.Errorf("diagram %q has no placements", c.Type)
		}
		if c.GridCellSupport == nil {
			t.Errorf("diagram %q has nil GridCellSupport", c.Type)
		}
		if c.AuthoringSurface == nil {
			t.Errorf("diagram %q has nil AuthoringSurface", c.Type)
		}
	}
}

func TestApplyPlacementMetadata_NativeInterceptHasDualPlacements(t *testing.T) {
	caps := svggen.DiagramCapabilities()
	enriched := ApplyPlacementMetadata(caps)

	// Find business_model_canvas — it should have two placements with different pipelines.
	for _, c := range enriched {
		if c.Type != "business_model_canvas" {
			continue
		}
		if len(c.Placements) != 2 {
			t.Fatalf("bmc placements = %d, want 2", len(c.Placements))
		}
		if c.Placements[0].Context != "placeholder" || c.Placements[0].Pipeline != "native_ooxml" {
			t.Errorf("bmc placeholder placement = %+v, want {placeholder, native_ooxml}", c.Placements[0])
		}
		if c.Placements[1].Context != "shape_grid" || c.Placements[1].Pipeline != "svg" {
			t.Errorf("bmc grid placement = %+v, want {shape_grid, svg}", c.Placements[1])
		}
		return
	}
	t.Error("business_model_canvas not found in enriched capabilities")
}

func TestApplyPlacementMetadata_SVGOnlyHasUniformPipeline(t *testing.T) {
	caps := svggen.DiagramCapabilities()
	enriched := ApplyPlacementMetadata(caps)

	// Find org_chart — SVG-only, both placements should be "svg".
	for _, c := range enriched {
		if c.Type != "org_chart" {
			continue
		}
		for _, p := range c.Placements {
			if p.Pipeline != "svg" {
				t.Errorf("org_chart %s pipeline = %q, want svg", p.Context, p.Pipeline)
			}
		}
		return
	}
	t.Error("org_chart not found in enriched capabilities")
}

func TestApplyPlacementMetadata_PreservesExistingFields(t *testing.T) {
	caps := svggen.DiagramCapabilities()
	enriched := ApplyPlacementMetadata(caps)

	// Verify that original fields are preserved.
	for i, c := range enriched {
		if c.Type != caps[i].Type {
			t.Errorf("type mismatch at index %d: %q != %q", i, c.Type, caps[i].Type)
		}
		if c.Status != caps[i].Status {
			t.Errorf("status mismatch for %q: %q != %q", c.Type, c.Status, caps[i].Status)
		}
	}
}

// TestDiagramPlacementRegistry_CoverageVsCapabilities checks that every diagram type
// in the capabilities registry has a matching placement entry.
func TestDiagramPlacementRegistry_CoverageVsCapabilities(t *testing.T) {
	for _, c := range svggen.DiagramCapabilities() {
		info := DiagramPlacementFor(c.Type)
		if info == nil {
			t.Errorf("diagram type %q is in capabilities but missing from placement registry", c.Type)
		}
	}
}
