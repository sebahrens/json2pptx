package svggen

import (
	"testing"
)

// TestChartCapabilities_AuthoringSurfacePopulated verifies that every chart
// capability has AuthoringSurface set. Charts currently all live in svggen, so
// the value must be "svggen" for every entry.
func TestChartCapabilities_AuthoringSurfacePopulated(t *testing.T) {
	caps := ChartCapabilities()
	if len(caps) == 0 {
		t.Fatal("ChartCapabilities returned empty slice")
	}
	for _, c := range caps {
		if c.AuthoringSurface == nil {
			t.Errorf("chart %q has nil AuthoringSurface", c.Type)
			continue
		}
		if *c.AuthoringSurface != "svggen" {
			t.Errorf("chart %q AuthoringSurface = %q, want %q", c.Type, *c.AuthoringSurface, "svggen")
		}
	}
}

// TestChartCapabilities_SvggenTypesResolveInRegistry verifies that every chart
// type advertised as AuthoringSurface=="svggen" can be resolved through the
// default svggen registry, either directly or via an alias. This catches
// silent drift between the capabilities slice (agent-facing truth) and
// builtinDiagrams (implementation truth).
func TestChartCapabilities_SvggenTypesResolveInRegistry(t *testing.T) {
	reg := DefaultRegistry()
	for _, c := range ChartCapabilities() {
		if c.AuthoringSurface == nil || *c.AuthoringSurface != "svggen" {
			continue
		}
		if d := reg.Get(c.Type); d == nil {
			t.Errorf("chart %q advertised as authoring_surface=svggen but is not in DefaultRegistry()", c.Type)
		}
	}
}

// TestDiagramCapabilities_AuthoringSurfacePopulated verifies that every diagram
// capability has AuthoringSurface set to either "svggen" or "native_ooxml".
// Any unmapped or unexpected value is a contract violation.
func TestDiagramCapabilities_AuthoringSurfacePopulated(t *testing.T) {
	caps := DiagramCapabilities()
	if len(caps) == 0 {
		t.Fatal("DiagramCapabilities returned empty slice")
	}
	for _, d := range caps {
		if d.AuthoringSurface == nil {
			t.Errorf("diagram %q has nil AuthoringSurface", d.Type)
			continue
		}
		switch *d.AuthoringSurface {
		case "svggen", "native_ooxml":
			// ok
		default:
			t.Errorf("diagram %q AuthoringSurface = %q, want one of {svggen, native_ooxml}", d.Type, *d.AuthoringSurface)
		}
	}
}

// TestDiagramCapabilities_RegistryAlignment is the truth-up invariant: every
// diagram tagged AuthoringSurface=="svggen" MUST be resolvable through the
// default registry, and every diagram tagged "native_ooxml" MUST NOT be
// resolvable through it. This prevents the capabilities slice from advertising
// unreachable types (e.g., when a renderer migrates from svggen to native
// OOXML and the capability metadata isn't updated).
func TestDiagramCapabilities_RegistryAlignment(t *testing.T) {
	reg := DefaultRegistry()
	for _, d := range DiagramCapabilities() {
		if d.AuthoringSurface == nil {
			continue // covered by TestDiagramCapabilities_AuthoringSurfacePopulated
		}
		got := reg.Get(d.Type)
		switch *d.AuthoringSurface {
		case "svggen":
			if got == nil {
				t.Errorf("diagram %q tagged authoring_surface=svggen but not in DefaultRegistry()", d.Type)
			}
		case "native_ooxml":
			if got != nil {
				t.Errorf("diagram %q tagged authoring_surface=native_ooxml but is registered in svggen DefaultRegistry()", d.Type)
			}
		}
	}
}

// TestDiagramCapabilities_AllTypesMapped guards against silent drift in the
// internal authoring-surface map: every entry in DiagramCapabilities() must
// have an explicit mapping in diagramAuthoringSurface. A new diagram type
// must register its surface choice deliberately, not fall through to a default.
func TestDiagramCapabilities_AllTypesMapped(t *testing.T) {
	for _, d := range DiagramCapabilities() {
		if _, ok := diagramAuthoringSurface[d.Type]; !ok {
			t.Errorf("diagram %q not present in diagramAuthoringSurface map", d.Type)
		}
	}
}

// TestNaturalAspect_KnownDiagrams verifies that the NaturalAspect lookup
// returns the expected W/H ratio for diagrams that pin a fixed canvas via
// RenderWithHelperDimensions. These values are part of the agent-facing fit
// finding contract (diagram_aspect_conflict) — drift would silently change
// preflight behaviour.
func TestNaturalAspect_KnownDiagrams(t *testing.T) {
	cases := map[string]float64{
		"timeline":  2.0,
		"gantt":     1.8,
		"org_chart": 1100.0 / 700.0,
	}
	for name, want := range cases {
		if got := NaturalAspect(name); got != want {
			t.Errorf("NaturalAspect(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestNaturalAspect_AliasResolved verifies that registered aliases resolve
// to the canonical diagram type before lookup.
func TestNaturalAspect_AliasResolved(t *testing.T) {
	want := 1100.0 / 700.0
	for _, alias := range []string{"org", "orgchart"} {
		if got := NaturalAspect(alias); got != want {
			t.Errorf("NaturalAspect(%q) = %v, want %v", alias, got, want)
		}
	}
}

// TestNaturalAspect_ZeroForContainerFitChartsAndUnknown verifies that chart
// types (which fit their container) and unknown types both return 0.
func TestNaturalAspect_ZeroForContainerFitChartsAndUnknown(t *testing.T) {
	for _, name := range []string{"bar_chart", "pie_chart", "scatter_chart", "made_up_type"} {
		if got := NaturalAspect(name); got != 0 {
			t.Errorf("NaturalAspect(%q) = %v, want 0", name, got)
		}
	}
}
