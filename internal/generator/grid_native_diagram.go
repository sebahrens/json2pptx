package generator

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Native diagrams inside a shape_grid cell or compose segment
// (go-slide-creator-ngbnf, go-slide-creator-3grgs).
//
// A native diagram is drawn as OOXML shapes, and until ngbnf only into a body
// placeholder. A grid cell is just a rectangle, which is all the native
// builders ever needed, so the region path runs the same bounded adapter as
// the placeholder path (layoutNativeDiagram / renderNativeInsert) at the
// cell's bounds. Every native type — the ten canonical frameworks and the
// panel aliases — renders in a region with the labels, values, fills, fonts
// and editability it has in a placeholder.
//
// What a region does not get is the placeholder's room. Each region placement
// is therefore laid out at its actual size first: if generation's own
// written-scale readability rule would refuse a shape in it, the placement is
// refused with DIAGRAM_REGION_TOO_SMALL naming the smallest region that does
// render, rather than publishing a canvas shrunk to illegible text.

// NativeDiagramRegion is the rectangle a native diagram is placed in and the
// template context it is drawn with.
type NativeDiagramRegion struct {
	Bounds      types.BoundingBox
	ThemeColors []types.ThemeColor
	FontName    string
	// SlideIndex (0-based) and Path (the authored diagram's JSON pointer)
	// locate the findings the placement emits.
	SlideIndex int
	Path       string
}

// GridNativeDiagram is a native diagram rendered into a region.
type GridNativeDiagram struct {
	// XML is one <p:grpSp> carrying the diagram's shapes.
	XML string
	// Icons are the panel-family icon overlays, positioned in slide EMU.
	Icons []IconInsert
	// Findings and Warnings are what laying the diagram out discovered.
	Findings []patterns.FitFinding
	Warnings []string
}

// IsGridNativeDiagram reports whether a diagram spec renders as native OOXML
// shapes when it sits in a shape_grid cell or compose segment, instead of as
// an svggen picture. It is the same set the placeholder path draws natively:
// both placements dispatch through one adapter.
func IsGridNativeDiagram(spec *types.DiagramSpec) bool {
	return IsNativeDiagramType(spec)
}

// GenerateGridNativeDiagram renders a native diagram as one group at
// region.Bounds. reserve(n) must return the first of n consecutive shape IDs
// the caller has set aside for the group; it is called once, after layout, so
// the reservation matches the shapes actually drawn. alt becomes the group's
// description. A placement whose text generation would refuse as unreadable
// returns a *NativeRegionCapacityError carrying DIAGRAM_REGION_TOO_SMALL.
func GenerateGridNativeDiagram(spec *types.DiagramSpec, region NativeDiagramRegion, alt string, reserve func(n int) uint32) (*GridNativeDiagram, error) {
	if !IsGridNativeDiagram(spec) {
		return nil, fmt.Errorf("diagram type %q has no native renderer", diagramTypeName(spec))
	}
	if region.Bounds.Width <= 0 || region.Bounds.Height <= 0 {
		return nil, fmt.Errorf("%s: empty region bounds", spec.Type)
	}
	if f := NativeDiagramRegionPreflight(spec, region); f != nil {
		return nil, &NativeRegionCapacityError{Finding: *f}
	}
	env := region.env()
	layout, err := layoutNativeDiagram(spec, region.Bounds, env, region.site())
	if err != nil {
		return nil, err
	}
	// Render once to learn the IDs the group spans, then at the reserved base.
	const probeBase = 1
	span := nativeInsertIDSpan(&layout.insert, renderNativeInsert(&layout.insert, probeBase, env), probeBase)
	base := reserve(int(span))
	xml := renderNativeInsert(&layout.insert, base, env)
	if xml == "" {
		return nil, fmt.Errorf("%s: native shape generation failed", spec.Type)
	}
	out := &GridNativeDiagram{
		XML:      pptx.SetGroupDescription(xml, alt),
		Findings: layout.findings,
		Warnings: layout.warnings,
	}
	if layout.panelLayout != "" && panelsHaveIcons(layout.insert.panels) {
		rects := panelIconRects(layout.panelLayout, layout.insert.bounds, layout.insert.panels, env.fontName)
		for i, p := range layout.insert.panels {
			if len(p.iconSVG) == 0 || rects[i].CX <= 0 || rects[i].CY <= 0 {
				continue
			}
			iconAlt := p.iconAlt
			if iconAlt == "" {
				iconAlt = p.title
			}
			out.Icons = append(out.Icons, IconInsert{
				SVGData: p.iconSVG, Alt: iconAlt,
				OffsetX: rects[i].X, OffsetY: rects[i].Y, ExtentCX: rects[i].CX, ExtentCY: rects[i].CY,
			})
		}
	}
	return out, nil
}

// NativeRegionCapacityError is a native diagram placement refused because the
// region is too small for its text to stay readable.
type NativeRegionCapacityError struct {
	Finding patterns.FitFinding
}

func (e *NativeRegionCapacityError) Error() string { return e.Finding.Message }

// nativeRegionMaxGrowth bounds the search for the smallest viable region: a
// diagram that needs more than this multiple of its region in each dimension
// is reported without a minimum size.
const nativeRegionMaxGrowth = 3.0

// NativeDiagramRegionPreflight lays the native diagram out at the region's
// actual size with the generator's own builders and returns the
// DIAGRAM_REGION_TOO_SMALL refusal its placement raises, or nil when every
// shape stays readable. validate and generate both call it, so a region
// generate refuses is an error at validate first.
func NativeDiagramRegionPreflight(spec *types.DiagramSpec, region NativeDiagramRegion) *patterns.FitFinding {
	if !IsGridNativeDiagram(spec) || region.Bounds.Width <= 0 || region.Bounds.Height <= 0 {
		return nil
	}
	shapes, actualPt := nativeSubFloorShapes(spec, region.Bounds, region)
	if shapes == 0 {
		return nil
	}
	b := region.Bounds
	minPt := float64(tokens.FootnoteMinHPt) / 100
	params := map[string]any{
		"diagram_type":       spec.Type,
		"region_width_emu":   b.Width,
		"region_height_emu":  b.Height,
		"shapes_below_floor": shapes,
		"actual_pt":          actualPt,
		"min_pt":             minPt,
		"alternatives": []string{
			"give the diagram a larger shape_grid cell or compose segment (fix.params.min_width_emu / min_height_emu)",
			"place it in a body placeholder as content diagram_value",
			"remove low-priority items",
		},
	}
	f := &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: spec.Type,
			Path:    region.Path,
			Code:    patterns.ErrCodeDiagramRegionTooSmall,
			Fix:     &patterns.FixSuggestion{Kind: "simplify_or_enlarge_diagram", Params: params},
		},
		Action:   "refuse",
		Measured: &patterns.Extent{WidthEMU: b.Width, HeightEMU: b.Height},
	}
	need := fmt.Sprintf("no region up to %.0f× this size renders it readably", nativeRegionMaxGrowth)
	if w, h, ok := nativeMinimumRegion(spec, region); ok {
		params["min_width_emu"], params["min_height_emu"] = w, h
		f.Allowed = &patterns.Extent{WidthEMU: w, HeightEMU: h}
		f.OverflowRatio = float64(w) / float64(b.Width)
		need = fmt.Sprintf("it needs about %.1f×%.1fin", emuToInches(w), emuToInches(h))
	}
	f.Message = fmt.Sprintf("native %s in a %.1f×%.1fin region renders %d shape(s) at %.1fpt, below the %.0fpt floor generation publishes; %s — enlarge the region, place the diagram in a body placeholder, or remove items",
		spec.Type, emuToInches(b.Width), emuToInches(b.Height), shapes, actualPt, minPt, need)
	return f
}

// nativeMinimumRegion grows the region in both dimensions (anchored at its
// origin) until the diagram renders without a sub-floor shape.
func nativeMinimumRegion(spec *types.DiagramSpec, region NativeDiagramRegion) (int64, int64, bool) {
	for step := 11; float64(step)/10 <= nativeRegionMaxGrowth; step++ {
		s := float64(step) / 10
		grown := region.Bounds
		grown.Width = int64(float64(grown.Width) * s)
		grown.Height = int64(float64(grown.Height) * s)
		if n, _ := nativeSubFloorShapes(spec, grown, region); n == 0 {
			return grown.Width, grown.Height, true
		}
	}
	return 0, 0, false
}

var readabilityActualPtRE = regexp.MustCompile(`renders at ([0-9.]+)pt`)

// nativeSubFloorShapes renders spec at bounds and counts the shapes
// generation's written-scale scan refuses, with the smallest size among them.
func nativeSubFloorShapes(spec *types.DiagramSpec, bounds types.BoundingBox, region NativeDiagramRegion) (int, float64) {
	env := region.env()
	layout, err := layoutNativeDiagram(spec, bounds, env, nativeDiagramSite{})
	if err != nil {
		return 0, 0
	}
	group := renderNativeInsert(&layout.insert, nativeReadabilityShapeIDBase, env)
	shapes, smallest := 0, 0.0
	for _, f := range unreadableAutofitFindings(group, tokens.ViewingModePresentation, func(string) string { return "" }) {
		if f.Action != "refuse" {
			continue
		}
		shapes++
		if m := readabilityActualPtRE.FindStringSubmatch(f.Message); m != nil {
			if pt, err := strconv.ParseFloat(m[1], 64); err == nil && (smallest == 0 || pt < smallest) {
				smallest = pt
			}
		}
	}
	return shapes, smallest
}

func (r NativeDiagramRegion) env() nativeDiagramEnv {
	return nativeDiagramEnv{fontName: r.FontName, themeColors: r.ThemeColors}
}

func (r NativeDiagramRegion) site() nativeDiagramSite {
	return nativeDiagramSite{slideIndex: r.SlideIndex, path: r.Path}
}

func emuToInches(emu int64) float64 { return float64(emu) / 914400 }

func diagramTypeName(spec *types.DiagramSpec) string {
	if spec == nil {
		return ""
	}
	return spec.Type
}
