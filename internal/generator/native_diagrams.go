package generator

import "github.com/sebahrens/json2pptx/internal/types"

// Native diagram types (go-slide-creator-r87g).
//
// Roughly half the advertised diagram catalogue is rendered as OOXML shapes by
// this package rather than by svggen — a SWOT, a business model canvas, a
// value chain and the panel family are real shapes on the slide, not an
// embedded picture. svggen's registry does not contain them, which is correct;
// what was not correct is that the preflight dry-render asked svggen about them
// anyway, got "unknown diagram type", and reported a REFUSE saying the deck
// would render a grey "Data unavailable" placeholder. It renders perfectly. An
// agent following validate — the gate SKILL.md calls a precondition — would
// have deleted a working diagram.
//
// This list is the seam between the two renderers. It has to stay in step with
// the dispatch in media.go (processDiagramNativeShapes) and with
// isPanelNativeLayout; TestNativeDiagramTypesMatchDispatch asserts that.

// nativeDiagramTypes are the diagram types the generator renders as native
// OOXML shapes. panel_layout is handled separately because whether it is
// native depends on its layout mode.
var nativeDiagramTypes = map[string]bool{
	"business_model_canvas": true,
	"heatmap":               true,
	"house_diagram":         true,
	"icon_columns":          true,
	"icon_rows":             true,
	"kpi_dashboard":         true,
	"nine_box_talent":       true,
	"pestel":                true,
	"porters_five_forces":   true,
	"process_flow":          true,
	"pyramid":               true,
	"stat_cards":            true,
	"swot":                  true,
	"value_chain":           true,
}

// IsNativeDiagramType reports whether a diagram spec is rendered by this
// package's native OOXML shape builders rather than by svggen. Callers that
// reason about svggen's behaviour — the preflight dry-render above all — must
// skip these: svggen has nothing to say about a diagram it never draws.
func IsNativeDiagramType(spec *types.DiagramSpec) bool {
	if spec == nil || spec.Type == "" {
		return false
	}
	if nativeDiagramTypes[spec.Type] {
		return true
	}
	return isPanelNativeLayout(spec)
}

// NativeDiagramTypeNames returns the native types by name, for callers that
// have a type string rather than a spec. panel_layout is included: its
// non-native layout modes are a narrow exception the spec form resolves.
func NativeDiagramTypeNames() []string {
	out := make([]string, 0, len(nativeDiagramTypes)+1)
	for t := range nativeDiagramTypes {
		out = append(out, t)
	}
	return append(out, "panel_layout")
}
