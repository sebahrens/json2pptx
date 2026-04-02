package pptx

// PresetGeometry identifies an OOXML preset shape geometry (a:prstGeom/@prst).
// These correspond to the ST_ShapeType enumeration in the ECMA-376 specification.
type PresetGeometry string

// Phase 1 preset geometries: layout-essential shapes commonly used in
// presentation templates and slide content.
const (
	GeomRect          PresetGeometry = "rect"
	GeomRoundRect     PresetGeometry = "roundRect"
	GeomRound1Rect    PresetGeometry = "round1Rect"
	GeomRound2SameRect PresetGeometry = "round2SameRect"
	GeomEllipse       PresetGeometry = "ellipse"
	GeomTriangle      PresetGeometry = "triangle"
	GeomDiamond       PresetGeometry = "diamond"
	GeomParallelogram PresetGeometry = "parallelogram"
	GeomTrapezoid     PresetGeometry = "trapezoid"
	GeomHexagon       PresetGeometry = "hexagon"
	GeomOctagon       PresetGeometry = "octagon"
	GeomChevron       PresetGeometry = "chevron"
	GeomHomePlate     PresetGeometry = "homePlate"
	GeomPentagon      PresetGeometry = "pentagon"
	GeomPlus          PresetGeometry = "plus"
	GeomRightArrow    PresetGeometry = "rightArrow"
	GeomLeftArrow     PresetGeometry = "leftArrow"
	GeomUpArrow       PresetGeometry = "upArrow"
	GeomDownArrow     PresetGeometry = "downArrow"
	GeomDonut         PresetGeometry = "donut"
)

// Phase 2 preset geometries: flowchart shapes and additional arrows.
const (
	// Flowchart shapes
	GeomFlowChartProcess            PresetGeometry = "flowChartProcess"
	GeomFlowChartDecision           PresetGeometry = "flowChartDecision"
	GeomFlowChartTerminator         PresetGeometry = "flowChartTerminator"
	GeomFlowChartDocument           PresetGeometry = "flowChartDocument"
	GeomFlowChartMultidocument      PresetGeometry = "flowChartMultidocument"
	GeomFlowChartInputOutput        PresetGeometry = "flowChartInputOutput"
	GeomFlowChartPredefinedProcess  PresetGeometry = "flowChartPredefinedProcess"
	GeomFlowChartInternalStorage    PresetGeometry = "flowChartInternalStorage"
	GeomFlowChartPreparation        PresetGeometry = "flowChartPreparation"
	GeomFlowChartManualInput        PresetGeometry = "flowChartManualInput"
	GeomFlowChartManualOperation    PresetGeometry = "flowChartManualOperation"
	GeomFlowChartConnector          PresetGeometry = "flowChartConnector"
	GeomFlowChartOffpageConnector   PresetGeometry = "flowChartOffpageConnector"
	GeomFlowChartAlternateProcess   PresetGeometry = "flowChartAlternateProcess"

	// Additional arrow shapes
	GeomLeftRightArrow    PresetGeometry = "leftRightArrow"
	GeomUpDownArrow       PresetGeometry = "upDownArrow"
	GeomNotchedRightArrow PresetGeometry = "notchedRightArrow"
	GeomStripedRightArrow PresetGeometry = "stripedRightArrow"
	GeomCurvedRightArrow  PresetGeometry = "curvedRightArrow"
	GeomCurvedLeftArrow   PresetGeometry = "curvedLeftArrow"
	GeomBentArrow         PresetGeometry = "bentArrow"
	GeomBentUpArrow       PresetGeometry = "bentUpArrow"
)

// Phase 3 preset geometries: callouts, stars/banners, and miscellaneous shapes.
const (
	// Callout shapes
	GeomWedgeRectCallout      PresetGeometry = "wedgeRectCallout"
	GeomWedgeRoundRectCallout PresetGeometry = "wedgeRoundRectCallout"
	GeomWedgeEllipseCallout   PresetGeometry = "wedgeEllipseCallout"
	GeomCloudCallout          PresetGeometry = "cloudCallout"
	GeomCloud                 PresetGeometry = "cloud"
	GeomCallout1              PresetGeometry = "callout1"
	GeomCallout2              PresetGeometry = "callout2"
	GeomCallout3              PresetGeometry = "callout3"
	GeomBorderCallout1        PresetGeometry = "borderCallout1"
	GeomBorderCallout2        PresetGeometry = "borderCallout2"
	GeomBorderCallout3        PresetGeometry = "borderCallout3"

	// Stars and banners
	GeomStar4          PresetGeometry = "star4"
	GeomStar5          PresetGeometry = "star5"
	GeomStar6          PresetGeometry = "star6"
	GeomStar7          PresetGeometry = "star7"
	GeomStar8          PresetGeometry = "star8"
	GeomStar10         PresetGeometry = "star10"
	GeomStar12         PresetGeometry = "star12"
	GeomRibbon         PresetGeometry = "ribbon"
	GeomRibbon2        PresetGeometry = "ribbon2"
	GeomIrregularSeal1 PresetGeometry = "irregularSeal1"
	GeomIrregularSeal2 PresetGeometry = "irregularSeal2"

	// Miscellaneous shapes
	GeomHeart         PresetGeometry = "heart"
	GeomLightningBolt PresetGeometry = "lightningBolt"
	GeomSun           PresetGeometry = "sun"
	GeomMoon          PresetGeometry = "moon"
	GeomSmileyFace    PresetGeometry = "smileyFace"
	GeomCube          PresetGeometry = "cube"
	GeomCan           PresetGeometry = "can"
	GeomFoldedCorner  PresetGeometry = "foldedCorner"
	GeomFrame         PresetGeometry = "frame"
	GeomBevel         PresetGeometry = "bevel"
)

// Phase 4 preset geometries: line shapes, dividers, arcs, brackets, and braces.
const (
	GeomLine     PresetGeometry = "line"
	GeomLineInv  PresetGeometry = "lineInv"
	GeomArc      PresetGeometry = "arc"
	GeomBlockArc PresetGeometry = "blockArc"
	GeomChord    PresetGeometry = "chord"

	// Bracket and brace shapes
	GeomLeftBracket  PresetGeometry = "leftBracket"
	GeomRightBracket PresetGeometry = "rightBracket"
	GeomLeftBrace    PresetGeometry = "leftBrace"
	GeomRightBrace   PresetGeometry = "rightBrace"
	GeomBracketPair  PresetGeometry = "bracketPair"
	GeomBracePair    PresetGeometry = "bracePair"

	// Right triangle
	GeomRightTriangle PresetGeometry = "rtTriangle"

	// Snipped-corner rectangle shapes
	GeomSnip1Rect     PresetGeometry = "snip1Rect"
	GeomSnip2SameRect PresetGeometry = "snip2SameRect"
	GeomSnip2DiagRect PresetGeometry = "snip2DiagRect"
	GeomSnipRoundRect PresetGeometry = "snipRoundRect"
)

// Phase 5 preset geometries: remaining ST_ShapeType values for full 187-shape coverage.
const (
	// Additional basic shapes
	GeomRound2DiagRect        PresetGeometry = "round2DiagRect"
	GeomNonIsoscelesTrapezoid PresetGeometry = "nonIsoscelesTrapezoid"
	GeomHeptagon              PresetGeometry = "heptagon"
	GeomDecagon               PresetGeometry = "decagon"
	GeomDodecagon             PresetGeometry = "dodecagon"
	GeomCross                 PresetGeometry = "cross"
	GeomHalfFrame             PresetGeometry = "halfFrame"
	GeomCorner                PresetGeometry = "corner"
	GeomDiagStripe            PresetGeometry = "diagStripe"
	GeomPlaque                PresetGeometry = "plaque"
	GeomNoSmoking             PresetGeometry = "noSmoking"
	GeomPie                   PresetGeometry = "pie"
	GeomPieWedge              PresetGeometry = "pieWedge"
	GeomTeardrop              PresetGeometry = "teardrop"
	GeomWave                  PresetGeometry = "wave"
	GeomDoubleWave            PresetGeometry = "doubleWave"
	GeomVerticalScroll        PresetGeometry = "verticalScroll"
	GeomHorizontalScroll      PresetGeometry = "horizontalScroll"

	// Additional stars
	GeomStar16 PresetGeometry = "star16"
	GeomStar24 PresetGeometry = "star24"
	GeomStar32 PresetGeometry = "star32"

	// Additional arrow shapes
	GeomLeftUpArrow              PresetGeometry = "leftUpArrow"
	GeomLeftRightUpArrow         PresetGeometry = "leftRightUpArrow"
	GeomQuadArrow                PresetGeometry = "quadArrow"
	GeomCurvedUpArrow            PresetGeometry = "curvedUpArrow"
	GeomCurvedDownArrow          PresetGeometry = "curvedDownArrow"
	GeomUturnArrow               PresetGeometry = "uturnArrow"
	GeomCircularArrow            PresetGeometry = "circularArrow"
	GeomLeftCircularArrow        PresetGeometry = "leftCircularArrow"
	GeomLeftRightCircularArrow   PresetGeometry = "leftRightCircularArrow"
	GeomSwooshArrow              PresetGeometry = "swooshArrow"

	// Arrow callout shapes
	GeomRightArrowCallout     PresetGeometry = "rightArrowCallout"
	GeomLeftArrowCallout      PresetGeometry = "leftArrowCallout"
	GeomUpArrowCallout        PresetGeometry = "upArrowCallout"
	GeomDownArrowCallout      PresetGeometry = "downArrowCallout"
	GeomLeftRightArrowCallout PresetGeometry = "leftRightArrowCallout"
	GeomUpDownArrowCallout    PresetGeometry = "upDownArrowCallout"
	GeomQuadArrowCallout      PresetGeometry = "quadArrowCallout"

	// Accent callout shapes
	GeomAccentCallout1       PresetGeometry = "accentCallout1"
	GeomAccentCallout2       PresetGeometry = "accentCallout2"
	GeomAccentCallout3       PresetGeometry = "accentCallout3"
	GeomAccentBorderCallout1 PresetGeometry = "accentBorderCallout1"
	GeomAccentBorderCallout2 PresetGeometry = "accentBorderCallout2"
	GeomAccentBorderCallout3 PresetGeometry = "accentBorderCallout3"

	// Equation shapes
	GeomMathPlus     PresetGeometry = "mathPlus"
	GeomMathMinus    PresetGeometry = "mathMinus"
	GeomMathMultiply PresetGeometry = "mathMultiply"
	GeomMathDivide   PresetGeometry = "mathDivide"
	GeomMathEqual    PresetGeometry = "mathEqual"
	GeomMathNotEqual PresetGeometry = "mathNotEqual"

	// Additional flowchart shapes
	GeomFlowChartCollate          PresetGeometry = "flowChartCollate"
	GeomFlowChartSort             PresetGeometry = "flowChartSort"
	GeomFlowChartExtract          PresetGeometry = "flowChartExtract"
	GeomFlowChartMerge            PresetGeometry = "flowChartMerge"
	GeomFlowChartOnlineStorage    PresetGeometry = "flowChartOnlineStorage"
	GeomFlowChartOfflineStorage   PresetGeometry = "flowChartOfflineStorage"
	GeomFlowChartMagneticTape     PresetGeometry = "flowChartMagneticTape"
	GeomFlowChartMagneticDisk     PresetGeometry = "flowChartMagneticDisk"
	GeomFlowChartMagneticDrum     PresetGeometry = "flowChartMagneticDrum"
	GeomFlowChartDisplay          PresetGeometry = "flowChartDisplay"
	GeomFlowChartDelay            PresetGeometry = "flowChartDelay"
	GeomFlowChartPunchedCard      PresetGeometry = "flowChartPunchedCard"
	GeomFlowChartPunchedTape      PresetGeometry = "flowChartPunchedTape"
	GeomFlowChartSummingJunction  PresetGeometry = "flowChartSummingJunction"
	GeomFlowChartOr               PresetGeometry = "flowChartOr"

	// Action button shapes
	GeomActionButtonBlank        PresetGeometry = "actionButtonBlank"
	GeomActionButtonHome         PresetGeometry = "actionButtonHome"
	GeomActionButtonHelp         PresetGeometry = "actionButtonHelp"
	GeomActionButtonInformation  PresetGeometry = "actionButtonInformation"
	GeomActionButtonBackPrevious PresetGeometry = "actionButtonBackPrevious"
	GeomActionButtonForwardNext  PresetGeometry = "actionButtonForwardNext"
	GeomActionButtonBeginning    PresetGeometry = "actionButtonBeginning"
	GeomActionButtonEnd          PresetGeometry = "actionButtonEnd"
	GeomActionButtonReturn       PresetGeometry = "actionButtonReturn"
	GeomActionButtonDocument     PresetGeometry = "actionButtonDocument"
	GeomActionButtonSound        PresetGeometry = "actionButtonSound"
	GeomActionButtonMovie        PresetGeometry = "actionButtonMovie"

	// Chart and tab shapes
	GeomChartX      PresetGeometry = "chartX"
	GeomChartStar   PresetGeometry = "chartStar"
	GeomChartPlus   PresetGeometry = "chartPlus"
	GeomCornerTabs  PresetGeometry = "cornerTabs"
	GeomSquareTabs  PresetGeometry = "squareTabs"
	GeomPlaqueTabs  PresetGeometry = "plaqueTabs"

	// Gears and funnel
	GeomGear6  PresetGeometry = "gear6"
	GeomGear9  PresetGeometry = "gear9"
	GeomFunnel PresetGeometry = "funnel"

)

// AdjustValue represents a named adjustment handle value for a preset geometry.
// For example, roundRect has an "adj" handle controlling corner radius,
// while rightArrow has "adj1" (head width) and "adj2" (head length).
type AdjustValue struct {
	Name  string // Adjustment handle name (e.g. "adj", "adj1", "adj2")
	Value int64  // Value in 1/50000ths (OOXML adjust coordinate system)
}

// defaultAdjustHandles maps each preset geometry to its default adjustment
// handle names. Shapes not in this map have no adjustment handles.
var defaultAdjustHandles = map[PresetGeometry][]string{
	GeomRoundRect:      {"adj"},
	GeomRound1Rect:     {"adj"},
	GeomRound2SameRect: {"adj1", "adj2"},
	GeomTriangle:       {"adj"},
	GeomParallelogram:  {"adj"},
	GeomTrapezoid:      {"adj"},
	GeomChevron:        {"adj"},
	GeomHomePlate:      {"adj"},
	GeomPlus:           {"adj"},
	GeomRightArrow:     {"adj1", "adj2"},
	GeomLeftArrow:      {"adj1", "adj2"},
	GeomUpArrow:        {"adj1", "adj2"},
	GeomDownArrow:      {"adj1", "adj2"},
	GeomDonut:          {"adj"},

	// Phase 2: flowchart shapes with adjustments
	GeomFlowChartAlternateProcess: {"adj"},

	// Phase 2: additional arrow shapes
	GeomLeftRightArrow:    {"adj1", "adj2"},
	GeomUpDownArrow:       {"adj1", "adj2"},
	GeomNotchedRightArrow: {"adj1", "adj2"},
	GeomStripedRightArrow: {"adj1", "adj2"},
	GeomCurvedRightArrow:  {"adj1", "adj2", "adj3"},
	GeomCurvedLeftArrow:   {"adj1", "adj2", "adj3"},
	GeomBentArrow:         {"adj1", "adj2", "adj3", "adj4"},
	GeomBentUpArrow:       {"adj1", "adj2", "adj3"},

	// Phase 3: callout shapes (pointer direction adjustments)
	GeomWedgeRectCallout:      {"adj1", "adj2"},
	GeomWedgeRoundRectCallout: {"adj1", "adj2"},
	GeomWedgeEllipseCallout:   {"adj1", "adj2"},
	GeomCloudCallout:          {"adj1", "adj2"},
	GeomCallout1:              {"adj1", "adj2", "adj3", "adj4"},
	GeomCallout2:              {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"},
	GeomCallout3:              {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"},
	GeomBorderCallout1:        {"adj1", "adj2", "adj3", "adj4"},
	GeomBorderCallout2:        {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"},
	GeomBorderCallout3:        {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"},

	// Phase 3: stars and banners
	GeomStar4:   {"adj"},
	GeomStar5:   {"adj"},
	GeomStar6:   {"adj"},
	GeomStar7:   {"adj"},
	GeomStar8:   {"adj"},
	GeomStar10:  {"adj"},
	GeomStar12:  {"adj"},
	GeomRibbon:  {"adj1", "adj2"},
	GeomRibbon2: {"adj1", "adj2"},

	// Phase 4: arc shapes
	GeomArc:      {"adj1", "adj2"},
	GeomBlockArc: {"adj1", "adj2", "adj3"},
	GeomChord:    {"adj1", "adj2"},

	// Phase 4: bracket and brace shapes
	GeomLeftBracket:  {"adj"},
	GeomRightBracket: {"adj"},
	GeomLeftBrace:    {"adj1", "adj2"},
	GeomRightBrace:   {"adj1", "adj2"},
	GeomBracketPair:  {"adj"},
	GeomBracePair:    {"adj"},

	// Phase 4: snipped-corner rectangles
	GeomSnip1Rect:     {"adj"},
	GeomSnip2SameRect: {"adj1", "adj2"},
	GeomSnip2DiagRect: {"adj1", "adj2"},
	GeomSnipRoundRect: {"adj1", "adj2"},

	// Phase 3: miscellaneous shapes
	GeomSun:          {"adj"},
	GeomMoon:         {"adj"},
	GeomSmileyFace:   {"adj"},
	GeomCube:         {"adj"},
	GeomCan:          {"adj"},
	GeomFoldedCorner: {"adj"},
	GeomFrame:        {"adj1"},
	GeomBevel:        {"adj"},

	// Phase 5: additional basic shapes
	GeomRound2DiagRect:        {"adj1", "adj2"},
	GeomNonIsoscelesTrapezoid: {"adj1", "adj2"},
	GeomCross:                 {"adj"},
	GeomHalfFrame:             {"adj1", "adj2"},
	GeomCorner:                {"adj1", "adj2"},
	GeomDiagStripe:            {"adj"},
	GeomPlaque:                {"adj"},
	GeomNoSmoking:             {"adj"},
	GeomPie:                   {"adj1", "adj2"},
	GeomTeardrop:              {"adj"},
	GeomWave:                  {"adj1", "adj2"},
	GeomDoubleWave:            {"adj1", "adj2"},
	GeomVerticalScroll:        {"adj"},
	GeomHorizontalScroll:      {"adj"},

	// Phase 5: additional stars
	GeomStar16: {"adj"},
	GeomStar24: {"adj"},
	GeomStar32: {"adj"},

	// Phase 5: additional arrow shapes
	GeomLeftUpArrow:            {"adj1", "adj2", "adj3"},
	GeomLeftRightUpArrow:       {"adj1", "adj2", "adj3"},
	GeomQuadArrow:              {"adj1", "adj2", "adj3"},
	GeomCurvedUpArrow:          {"adj1", "adj2", "adj3"},
	GeomCurvedDownArrow:        {"adj1", "adj2", "adj3"},
	GeomUturnArrow:             {"adj1", "adj2", "adj3", "adj4", "adj5"},
	GeomCircularArrow:          {"adj1", "adj2", "adj3", "adj4", "adj5"},
	GeomLeftCircularArrow:      {"adj1", "adj2", "adj3", "adj4", "adj5"},
	GeomLeftRightCircularArrow: {"adj1", "adj2", "adj3", "adj4", "adj5"},
	GeomSwooshArrow:            {"adj1", "adj2"},

	// Phase 5: arrow callout shapes
	GeomRightArrowCallout:     {"adj1", "adj2", "adj3", "adj4"},
	GeomLeftArrowCallout:      {"adj1", "adj2", "adj3", "adj4"},
	GeomUpArrowCallout:        {"adj1", "adj2", "adj3", "adj4"},
	GeomDownArrowCallout:      {"adj1", "adj2", "adj3", "adj4"},
	GeomLeftRightArrowCallout: {"adj1", "adj2", "adj3", "adj4"},
	GeomUpDownArrowCallout:    {"adj1", "adj2", "adj3", "adj4"},
	GeomQuadArrowCallout:      {"adj1", "adj2", "adj3", "adj4"},

	// Phase 5: accent callout shapes
	GeomAccentCallout1:       {"adj1", "adj2", "adj3", "adj4"},
	GeomAccentCallout2:       {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"},
	GeomAccentCallout3:       {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"},
	GeomAccentBorderCallout1: {"adj1", "adj2", "adj3", "adj4"},
	GeomAccentBorderCallout2: {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6"},
	GeomAccentBorderCallout3: {"adj1", "adj2", "adj3", "adj4", "adj5", "adj6", "adj7", "adj8"},

	// Phase 5: equation shapes
	GeomMathPlus:     {"adj"},
	GeomMathMinus:    {"adj"},
	GeomMathMultiply: {"adj"},
	GeomMathDivide:   {"adj1", "adj2"},
	GeomMathEqual:    {"adj1", "adj2"},
	GeomMathNotEqual: {"adj1", "adj2", "adj3"},

	// Phase 5: gears
	GeomGear6: {"adj1", "adj2"},
	GeomGear9: {"adj1", "adj2"},

}

// DefaultAdjustHandles returns the default adjustment handle names for a
// preset geometry. Returns nil for shapes with no adjustment handles.
func DefaultAdjustHandles(geom PresetGeometry) []string {
	return defaultAdjustHandles[geom]
}

// knownGeometries is the exhaustive list of all defined PresetGeometry constants.
var knownGeometries = []PresetGeometry{
	// Phase 1
	GeomRect, GeomRoundRect, GeomRound1Rect, GeomRound2SameRect,
	GeomEllipse, GeomTriangle, GeomDiamond, GeomParallelogram,
	GeomTrapezoid, GeomHexagon, GeomOctagon, GeomChevron,
	GeomHomePlate, GeomPentagon, GeomPlus,
	GeomRightArrow, GeomLeftArrow, GeomUpArrow, GeomDownArrow, GeomDonut,
	// Flowchart
	GeomFlowChartProcess, GeomFlowChartDecision, GeomFlowChartTerminator,
	GeomFlowChartDocument, GeomFlowChartMultidocument, GeomFlowChartInputOutput,
	GeomFlowChartPredefinedProcess, GeomFlowChartInternalStorage,
	GeomFlowChartPreparation, GeomFlowChartManualInput,
	GeomFlowChartManualOperation, GeomFlowChartConnector,
	GeomFlowChartOffpageConnector, GeomFlowChartAlternateProcess,
	// Arrows
	GeomLeftRightArrow, GeomUpDownArrow, GeomNotchedRightArrow,
	GeomStripedRightArrow, GeomCurvedRightArrow, GeomCurvedLeftArrow,
	GeomBentArrow, GeomBentUpArrow,
	// Callouts
	GeomWedgeRectCallout, GeomWedgeRoundRectCallout, GeomWedgeEllipseCallout,
	GeomCloudCallout, GeomCloud, GeomCallout1, GeomCallout2, GeomCallout3,
	GeomBorderCallout1, GeomBorderCallout2, GeomBorderCallout3,
	// Stars & banners
	GeomStar4, GeomStar5, GeomStar6, GeomStar7, GeomStar8,
	GeomStar10, GeomStar12, GeomRibbon, GeomRibbon2,
	GeomIrregularSeal1, GeomIrregularSeal2,
	// Miscellaneous
	GeomHeart, GeomLightningBolt, GeomSun, GeomMoon, GeomSmileyFace,
	GeomCube, GeomCan, GeomFoldedCorner, GeomFrame, GeomBevel,
	// Lines & arcs
	GeomLine, GeomLineInv, GeomArc, GeomBlockArc, GeomChord,
	// Brackets
	GeomLeftBracket, GeomRightBracket, GeomLeftBrace, GeomRightBrace,
	GeomBracketPair, GeomBracePair,
	// Phase 3
	GeomRightTriangle, GeomSnip1Rect, GeomSnip2SameRect,
	GeomSnip2DiagRect, GeomSnipRoundRect,
	// Phase 4
	GeomRound2DiagRect, GeomNonIsoscelesTrapezoid, GeomHeptagon,
	GeomDecagon, GeomDodecagon, GeomCross, GeomHalfFrame,
	GeomCorner, GeomDiagStripe, GeomPlaque, GeomNoSmoking,
	GeomPie, GeomPieWedge, GeomTeardrop, GeomWave, GeomDoubleWave,
	GeomVerticalScroll, GeomHorizontalScroll,
	// Stars (extended)
	GeomStar16, GeomStar24, GeomStar32,
	// Arrows (extended)
	GeomLeftUpArrow, GeomLeftRightUpArrow, GeomQuadArrow,
	GeomCurvedUpArrow, GeomCurvedDownArrow, GeomUturnArrow,
	GeomCircularArrow, GeomLeftCircularArrow, GeomLeftRightCircularArrow,
	GeomSwooshArrow,
	// Arrow callouts
	GeomRightArrowCallout, GeomLeftArrowCallout, GeomUpArrowCallout,
	GeomDownArrowCallout, GeomLeftRightArrowCallout,
	GeomUpDownArrowCallout, GeomQuadArrowCallout,
	// Accent callouts
	GeomAccentCallout1, GeomAccentCallout2, GeomAccentCallout3,
	GeomAccentBorderCallout1, GeomAccentBorderCallout2, GeomAccentBorderCallout3,
	// Math
	GeomMathPlus, GeomMathMinus, GeomMathMultiply, GeomMathDivide,
	GeomMathEqual, GeomMathNotEqual,
	// Flowchart (extended)
	GeomFlowChartCollate, GeomFlowChartSort, GeomFlowChartExtract,
	GeomFlowChartMerge, GeomFlowChartOnlineStorage, GeomFlowChartOfflineStorage,
	GeomFlowChartMagneticTape, GeomFlowChartMagneticDisk,
	GeomFlowChartMagneticDrum, GeomFlowChartDisplay, GeomFlowChartDelay,
	GeomFlowChartPunchedCard, GeomFlowChartPunchedTape,
	GeomFlowChartSummingJunction, GeomFlowChartOr,
	// Action buttons
	GeomActionButtonBlank, GeomActionButtonHome, GeomActionButtonHelp,
	GeomActionButtonInformation, GeomActionButtonBackPrevious,
	GeomActionButtonForwardNext, GeomActionButtonBeginning,
	GeomActionButtonEnd, GeomActionButtonReturn, GeomActionButtonDocument,
	GeomActionButtonSound, GeomActionButtonMovie,
	// Phase 5
	GeomChartX, GeomChartStar, GeomChartPlus, GeomCornerTabs,
	GeomSquareTabs, GeomPlaqueTabs,
	// Gears
	GeomGear6, GeomGear9, GeomFunnel,
}

// knownGeometrySet is built from knownGeometries for O(1) lookup.
var knownGeometrySet map[PresetGeometry]struct{}

func init() {
	knownGeometrySet = make(map[PresetGeometry]struct{}, len(knownGeometries))
	for _, g := range knownGeometries {
		knownGeometrySet[g] = struct{}{}
	}
}

// KnownGeometries returns the list of all defined PresetGeometry constants.
func KnownGeometries() []PresetGeometry {
	return knownGeometries
}

// IsKnownGeometry returns true if the given string is a valid PresetGeometry.
func IsKnownGeometry(name string) bool {
	_, ok := knownGeometrySet[PresetGeometry(name)]
	return ok
}
