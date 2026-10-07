package svggen

// Slide-scale type and tonal roles for the structural diagrams (timeline,
// fishbone, org chart, venn).
//
// A diagram drawn into a shape-grid cell is told its physical placement, and
// applyPlacementTypography raises its type model until the smallest role is
// 12pt on the slide. A diagram drawn into a body placeholder is told nothing:
// the generator hands it a canvas of the placeholder's own size in points
// (one user unit is one point on the slide; the SVG export scales the canvas
// to CSS pixels), and the dimension-scaled type model set small labels and
// captions at 8-10 units, so the diagrams read as thumbnails
// (go-slide-creator-mhc3k, go-slide-creator-o8cqh). These diagrams therefore
// assume that scale when no placement is declared.

const (
	// slideCanvasPtPerUnit is the size of one user unit of a canvas drawn
	// without a declared placement: the canvas is the placeholder, in points.
	slideCanvasPtPerUnit = 1.0
	// diagramReadablePt is the smallest text a structural diagram sets on a
	// slide.
	diagramReadablePt = ChartStepBodyPt
)

// assumeSlidePlacement raises the builder's type model so that the smallest
// role is diagramReadablePt at slideCanvasPtPerUnit when the request declares
// no placement of its own. A declared placement was already applied by the
// render helper and is left alone.
func assumeSlidePlacement(b *SVGBuilder, req *RequestEnvelope) {
	if b == nil || req == nil || req.Style.PlacementWidthPt > 0 || req.Style.MinReadablePt > 0 {
		return
	}
	style := req.Style
	style.PlacementWidthPt = b.Width() * slideCanvasPtPerUnit
	style.PlacementHeightPt = b.Height() * slideCanvasPtPerUnit
	style.MinReadablePt = diagramReadablePt
	applyPlacementTypography(b, style, b.Width(), b.Height())
}

// Tonal roles, the pattern engine's system (internal/patterns/tonal_system.go)
// in svggen colours: a panel groups content on the lightest neutral, a shape
// that is the content takes the accent's "Lighter 80%" swatch, the one
// emphasised item the solid accent, and structure (spines, connectors, axis
// bands) a neutral ink. Tints are Color.Tint, the lumMod / lumOff family, so
// both engines tint one accent to one colour.
const (
	tonalPanelShare   = 0.04 // dk1 at 4% over the background
	tonalContentKeep  = 0.20 // accent "Lighter 80%"
	tonalRuleShare    = 0.55 // connectors, bones, leaders
	tonalSpineShare   = 0.80 // the one weighted structural line
	tonalMutedShare   = 0.65 // secondary text
	diagramInkMinimum = 4.5  // WCAG AA for body text
)

// diagramAccent is the template's primary accent.
func diagramAccent(style *StyleGuide) Color {
	return style.Palette.Accent1.Opaque()
}

// diagramPanelFill is the PANEL role: the lightest neutral surface.
func diagramPanelFill(style *StyleGuide) Color {
	return NeutralInk(style.Palette, tonalPanelShare)
}

// diagramContentFill is the CONTENT role: the accent's "Lighter 80%" swatch,
// flattened over the background. On a dark background the tint would be the
// lightest thing on the slide, so the accent is mixed into the background
// instead.
func diagramContentFill(style *StyleGuide) Color {
	accent := diagramAccent(style)
	bg := style.Palette.Background.Opaque()
	if !bg.IsLight() {
		return accent.WithAlpha(0.35).BlendOver(bg)
	}
	return accent.Tint(tonalContentKeep)
}

// diagramInkOn is the text colour for fill, chosen by measurement: the
// palette's text ink or its background colour, whichever contrasts more.
func diagramInkOn(style *StyleGuide, fill Color) Color {
	fill = fill.BlendOver(style.Palette.Background.Opaque())
	ink := style.Palette.TextPrimary.Opaque()
	page := style.Palette.Background.Opaque()
	if page.ContrastWith(fill) > ink.ContrastWith(fill) {
		ink = page
	}
	return EnsureContrast(ink, fill, diagramInkMinimum)
}

// diagramMutedInk is secondary text on the background: the text ink at
// tonalMutedShare, darkened until it still reads as body text.
func diagramMutedInk(style *StyleGuide) Color {
	bg := style.Palette.Background.Opaque()
	return EnsureContrast(NeutralInk(style.Palette, tonalMutedShare), bg, diagramInkMinimum)
}
