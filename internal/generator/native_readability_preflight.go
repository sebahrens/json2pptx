package generator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Predicting native-diagram readability refusals at validate time
// (go-slide-creator-y72c3).
//
// Generation scans every written shape for a stored autofit shrink that takes
// its text below the 7pt floor no role survives (reportUnreadableAutofit) and
// refuses the deck. validate --fit-report used to measure native diagrams with
// a template-agnostic estimate (NativeDiagramPreflight), so a business model
// canvas that generate refused on eight templates validated clean on all of
// them. This preflight runs the generator's own builders into the placeholder
// the template gives the diagram and the same scan over their XML, so validate
// reports the refusal generation will raise.

// nativeReadabilityShapeIDBase is the first shape id the preflight's group
// uses. Ids never reach a finding: paths name the authored diagram.
const nativeReadabilityShapeIDBase = 10000

// NativeDiagramReadabilityPreflight lays the native diagram out in bounds with
// the generator's own builders and returns the TEXT_BELOW_READABLE_MIN refusal
// generation would raise for it, at path (the authored diagram_value), or nil
// when every shape stays readable. Only refusals are reported: the advisory
// sub-floor findings generation also emits need the rendered deck to judge.
// fontName and themeColors are the template's; bounds is the placeholder the
// diagram replaces.
func NativeDiagramReadabilityPreflight(spec *types.DiagramSpec, bounds types.BoundingBox, fontName string, themeColors []types.ThemeColor, mode tokens.ViewingMode, path string) []patterns.FitFinding {
	if spec == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return nil
	}
	group := nativeDiagramGroupXML(spec, bounds, fontName, themeColors)
	if group == "" {
		return nil
	}
	var first *patterns.FitFinding
	shapes := 0
	for _, f := range unreadableAutofitFindings(group, mode, func(string) string { return path }) {
		if f.Action != "refuse" {
			continue
		}
		shapes++
		if first == nil {
			f := f
			first = &f
		}
	}
	if first == nil {
		return nil
	}
	first.Pattern = spec.Type
	first.Message = fmt.Sprintf("native %s %s; %d of its shapes fall below the floor in this template's placeholder, so generate refuses the deck: shorten or remove items, or give the diagram a larger placeholder",
		spec.Type, first.Message, shapes)
	return []patterns.FitFinding{*first}
}

// nativeShapeSource records which authored diagram a native group's shape
// ids [lo, hi) on one slide were drawn from (go-slide-creator-ygaln).
type nativeShapeSource struct {
	slideIndex  int
	lo, hi      uint32
	path        string
	diagramType string
}

// tagNativeInserts stamps the native groups registered for one diagram
// content item (inserts from index `from` on) with its authored path and type.
func (ctx *singlePassContext) tagNativeInserts(slideNum, from, contentIdx int, item ContentItem) {
	inserts := ctx.panelShapeInserts[slideNum]
	if from >= len(inserts) {
		return
	}
	path := slidepath.ContentField(slideNum-ctx.calculateStartingSlideNum(), contentIdx, "diagram_value")
	diagramType := ""
	if spec, ok := item.Value.(*types.DiagramSpec); ok && spec != nil {
		diagramType = spec.Type
	}
	for i := from; i < len(inserts); i++ {
		inserts[i].contentPath = path
		inserts[i].diagramType = diagramType
	}
}

// authorNativeRefusal points a refusal raised on a rendered native-diagram
// shape (/slides/N/rendered_shapes/ID) at the authored diagram
// (/slides/N/content/J/diagram_value) and names its type. A shape id means
// nothing to the author who has to fix the deck.
func (ctx *singlePassContext) authorNativeRefusal(loss *patterns.ValidationError) {
	const marker = "/rendered_shapes/"
	idx := slidepath.SlideIndex(loss.Path)
	at := strings.Index(loss.Path, marker)
	if idx < 0 || at < 0 {
		return
	}
	rest := loss.Path[at+len(marker):]
	if j := strings.IndexByte(rest, '/'); j >= 0 {
		rest = rest[:j]
	}
	id, err := strconv.ParseUint(rest, 10, 32)
	if err != nil {
		return
	}
	for _, s := range ctx.nativeShapeSources {
		if s.slideIndex == idx && uint32(id) >= s.lo && uint32(id) < s.hi {
			loss.Path = s.path
			if loss.Pattern == "" {
				loss.Pattern = s.diagramType
			}
			return
		}
	}
}

// nativeDiagramGroupXML renders the group generation writes for the native
// diagram types whose text the readability preflight measures, or "" for any
// other type.
func nativeDiagramGroupXML(spec *types.DiagramSpec, bounds types.BoundingBox, fontName string, themeColors []types.ThemeColor) string {
	base := uint32(nativeReadabilityShapeIDBase)
	switch {
	case isBMCDiagram(spec):
		return generateBMCGroupXML(bmcPanels(spec), bounds, base, taxonomyPalette(spec, len(bmcSectionOrder), bmcDefaultTint))
	case isSWOTDiagram(spec):
		panels := swotPanels(spec)
		bounds = fitBoundsToFramework(bounds, nativeFrameworkSize("swot", bounds, panels, houseDiagramMeta{}, fontName))
		return generateSWOTGroupXML(panels, bounds, base, taxonomyPalette(spec, 4, swotDefaultTint))
	case isNineBoxDiagram(spec):
		panels, _ := nineBoxPanels(spec)
		return generateNineBoxGroupXML(panels, bounds, base, nineBoxSemanticTints(nil))
	case isPortersFiveForcesDiagram(spec):
		panels := porterPanels(spec)
		if len(panels) == 0 {
			return ""
		}
		return generatePortersFiveGroupXML(panels, bounds, base, themeColors)
	case isPyramidDiagram(spec):
		panels, err := pyramidPanels(spec)
		if err != nil || len(panels) == 0 {
			return ""
		}
		return generatePyramidGroupXML(panels, bounds, base, fontName)
	}
	return ""
}
