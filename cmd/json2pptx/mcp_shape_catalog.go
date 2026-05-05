package main

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// shapeCatalogCategory groups preset geometries by use case.
type shapeCatalogCategory struct {
	Category    string               `json:"category"`
	Description string               `json:"description"`
	Shapes      []shapeCatalogEntry  `json:"shapes"`
}

// shapeCatalogEntry describes a single preset geometry.
type shapeCatalogEntry struct {
	Name       string   `json:"name"`
	AdjustHandles []string `json:"adjust_handles,omitempty"`
}

// shapeCategoryDef defines which geometries belong to each category.
type shapeCategoryDef struct {
	name        string
	description string
	shapes      []pptx.PresetGeometry
}

// shapeCategories defines the grouping of shapes by use case.
var shapeCategories = []shapeCategoryDef{
	{
		name:        "basic",
		description: "Fundamental shapes for general-purpose use: containers, labels, backgrounds",
		shapes: []pptx.PresetGeometry{
			pptx.GeomRect, pptx.GeomRoundRect, pptx.GeomRound1Rect, pptx.GeomRound2SameRect,
			pptx.GeomRound2DiagRect, pptx.GeomSnip1Rect, pptx.GeomSnip2SameRect,
			pptx.GeomSnip2DiagRect, pptx.GeomSnipRoundRect, pptx.GeomEllipse,
			pptx.GeomTriangle, pptx.GeomRightTriangle, pptx.GeomDiamond,
			pptx.GeomParallelogram, pptx.GeomTrapezoid, pptx.GeomNonIsoscelesTrapezoid,
			pptx.GeomHexagon, pptx.GeomHeptagon, pptx.GeomOctagon, pptx.GeomDecagon,
			pptx.GeomDodecagon, pptx.GeomPentagon, pptx.GeomPlus, pptx.GeomCross,
			pptx.GeomDonut, pptx.GeomFrame, pptx.GeomHalfFrame, pptx.GeomCorner,
			pptx.GeomBevel, pptx.GeomPlaque, pptx.GeomDiagStripe,
		},
	},
	{
		name:        "arrow",
		description: "Directional arrows for process flows, timelines, and navigation",
		shapes: []pptx.PresetGeometry{
			pptx.GeomRightArrow, pptx.GeomLeftArrow, pptx.GeomUpArrow, pptx.GeomDownArrow,
			pptx.GeomLeftRightArrow, pptx.GeomUpDownArrow,
			pptx.GeomNotchedRightArrow, pptx.GeomStripedRightArrow,
			pptx.GeomCurvedRightArrow, pptx.GeomCurvedLeftArrow,
			pptx.GeomCurvedUpArrow, pptx.GeomCurvedDownArrow,
			pptx.GeomBentArrow, pptx.GeomBentUpArrow,
			pptx.GeomLeftUpArrow, pptx.GeomLeftRightUpArrow, pptx.GeomQuadArrow,
			pptx.GeomUturnArrow, pptx.GeomCircularArrow,
			pptx.GeomLeftCircularArrow, pptx.GeomLeftRightCircularArrow,
			pptx.GeomSwooshArrow,
		},
	},
	{
		name:        "flow",
		description: "Process flow shapes: steps, decisions, terminators, connectors (chevron/homePlate ideal for horizontal process steps)",
		shapes: []pptx.PresetGeometry{
			pptx.GeomChevron, pptx.GeomHomePlate,
			pptx.GeomFlowChartProcess, pptx.GeomFlowChartAlternateProcess,
			pptx.GeomFlowChartDecision, pptx.GeomFlowChartTerminator,
			pptx.GeomFlowChartDocument, pptx.GeomFlowChartMultidocument,
			pptx.GeomFlowChartInputOutput, pptx.GeomFlowChartPredefinedProcess,
			pptx.GeomFlowChartInternalStorage, pptx.GeomFlowChartPreparation,
			pptx.GeomFlowChartManualInput, pptx.GeomFlowChartManualOperation,
			pptx.GeomFlowChartConnector, pptx.GeomFlowChartOffpageConnector,
			pptx.GeomFlowChartCollate, pptx.GeomFlowChartSort,
			pptx.GeomFlowChartExtract, pptx.GeomFlowChartMerge,
			pptx.GeomFlowChartOnlineStorage, pptx.GeomFlowChartOfflineStorage,
			pptx.GeomFlowChartMagneticTape, pptx.GeomFlowChartMagneticDisk,
			pptx.GeomFlowChartMagneticDrum, pptx.GeomFlowChartDisplay,
			pptx.GeomFlowChartDelay, pptx.GeomFlowChartPunchedCard,
			pptx.GeomFlowChartPunchedTape, pptx.GeomFlowChartSummingJunction,
			pptx.GeomFlowChartOr,
		},
	},
	{
		name:        "callout",
		description: "Speech bubbles, annotations, and pointer shapes for emphasis and labeling",
		shapes: []pptx.PresetGeometry{
			pptx.GeomWedgeRectCallout, pptx.GeomWedgeRoundRectCallout,
			pptx.GeomWedgeEllipseCallout, pptx.GeomCloudCallout, pptx.GeomCloud,
			pptx.GeomCallout1, pptx.GeomCallout2, pptx.GeomCallout3,
			pptx.GeomBorderCallout1, pptx.GeomBorderCallout2, pptx.GeomBorderCallout3,
			pptx.GeomAccentCallout1, pptx.GeomAccentCallout2, pptx.GeomAccentCallout3,
			pptx.GeomAccentBorderCallout1, pptx.GeomAccentBorderCallout2, pptx.GeomAccentBorderCallout3,
			pptx.GeomRightArrowCallout, pptx.GeomLeftArrowCallout,
			pptx.GeomUpArrowCallout, pptx.GeomDownArrowCallout,
			pptx.GeomLeftRightArrowCallout, pptx.GeomUpDownArrowCallout,
			pptx.GeomQuadArrowCallout,
		},
	},
	{
		name:        "star_banner",
		description: "Stars, ribbons, and seals for badges, awards, and decorative emphasis",
		shapes: []pptx.PresetGeometry{
			pptx.GeomStar4, pptx.GeomStar5, pptx.GeomStar6, pptx.GeomStar7,
			pptx.GeomStar8, pptx.GeomStar10, pptx.GeomStar12,
			pptx.GeomStar16, pptx.GeomStar24, pptx.GeomStar32,
			pptx.GeomRibbon, pptx.GeomRibbon2,
			pptx.GeomIrregularSeal1, pptx.GeomIrregularSeal2,
		},
	},
	{
		name:        "line_connector",
		description: "Lines, arcs, brackets, and braces for connecting and grouping elements",
		shapes: []pptx.PresetGeometry{
			pptx.GeomLine, pptx.GeomLineInv, pptx.GeomArc, pptx.GeomBlockArc, pptx.GeomChord,
			pptx.GeomLeftBracket, pptx.GeomRightBracket,
			pptx.GeomLeftBrace, pptx.GeomRightBrace,
			pptx.GeomBracketPair, pptx.GeomBracePair,
		},
	},
	{
		name:        "symbol",
		description: "Pictographic shapes: icons, metaphors, and decorative elements",
		shapes: []pptx.PresetGeometry{
			pptx.GeomHeart, pptx.GeomLightningBolt, pptx.GeomSun, pptx.GeomMoon,
			pptx.GeomSmileyFace, pptx.GeomNoSmoking, pptx.GeomFunnel,
			pptx.GeomGear6, pptx.GeomGear9,
			pptx.GeomCube, pptx.GeomCan, pptx.GeomFoldedCorner,
			pptx.GeomTeardrop, pptx.GeomPie, pptx.GeomPieWedge,
			pptx.GeomWave, pptx.GeomDoubleWave,
			pptx.GeomVerticalScroll, pptx.GeomHorizontalScroll,
		},
	},
	{
		name:        "math",
		description: "Mathematical operator shapes for equations and formulas",
		shapes: []pptx.PresetGeometry{
			pptx.GeomMathPlus, pptx.GeomMathMinus, pptx.GeomMathMultiply,
			pptx.GeomMathDivide, pptx.GeomMathEqual, pptx.GeomMathNotEqual,
		},
	},
	{
		name:        "action_button",
		description: "Interactive navigation buttons (standard presentation action shapes)",
		shapes: []pptx.PresetGeometry{
			pptx.GeomActionButtonBlank, pptx.GeomActionButtonHome,
			pptx.GeomActionButtonHelp, pptx.GeomActionButtonInformation,
			pptx.GeomActionButtonBackPrevious, pptx.GeomActionButtonForwardNext,
			pptx.GeomActionButtonBeginning, pptx.GeomActionButtonEnd,
			pptx.GeomActionButtonReturn, pptx.GeomActionButtonDocument,
			pptx.GeomActionButtonSound, pptx.GeomActionButtonMovie,
		},
	},
	{
		name:        "chart_tab",
		description: "Chart markers and tab shapes for data visualization accents",
		shapes: []pptx.PresetGeometry{
			pptx.GeomChartX, pptx.GeomChartStar, pptx.GeomChartPlus,
			pptx.GeomCornerTabs, pptx.GeomSquareTabs, pptx.GeomPlaqueTabs,
		},
	},
}

func mcpGetShapeCatalogTool() mcp.Tool {
	return mcp.NewTool("get_shape_catalog",
		mcp.WithDescription("List all available preset shape geometries grouped by use case (basic, arrow, flow, callout, star_banner, line_connector, symbol, math, action_button, chart_tab). Use to discover shapes beyond the default \"rect\" for shape_grid cells. Chevron and homePlate are ideal for process/timeline steps."),
		mcp.WithRawOutputSchema(outputSchemaGetShapeCatalog),
		mcp.WithString("category",
			mcp.Description("Filter to a single category. Omit for all categories."),
			mcp.Enum("basic", "arrow", "flow", "callout", "star_banner", "line_connector", "symbol", "math", "action_button", "chart_tab"),
		),
		mcp.WithString("search",
			mcp.Description("Substring filter applied to shape names. Case-insensitive. Example: \"arrow\" returns all arrow-related shapes."),
		),
	)
}

func handleGetShapeCatalog(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	categoryFilter, _ := request.RequireString("category")
	search, _ := request.RequireString("search")
	search = strings.ToLower(strings.TrimSpace(search))

	result := make([]shapeCatalogCategory, 0, len(shapeCategories))
	for _, cat := range shapeCategories {
		if categoryFilter != "" && cat.name != categoryFilter {
			continue
		}

		entries := make([]shapeCatalogEntry, 0, len(cat.shapes))
		for _, geom := range cat.shapes {
			name := string(geom)
			if search != "" && !strings.Contains(strings.ToLower(name), search) {
				continue
			}
			entry := shapeCatalogEntry{Name: name}
			if handles := pptx.DefaultAdjustHandles(geom); len(handles) > 0 {
				entry.AdjustHandles = handles
			}
			entries = append(entries, entry)
		}

		if len(entries) > 0 {
			result = append(result, shapeCatalogCategory{
				Category:    cat.name,
				Description: cat.description,
				Shapes:      entries,
			})
		}
	}

	mcpResult, err := api.MCPSuccessResult(context.Background(), result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", "failed to marshal response: "+err.Error()), nil
	}
	return mcpResult, nil
}
