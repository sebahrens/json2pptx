package main

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerMCPTools registers every json2pptx MCP tool against the supplied
// server. It is the single source of truth for the tool catalog so that
// runMCP (production) and the doctor test (coverage audit) cannot drift —
// adding a tool here automatically gets it tested.
func registerMCPTools(s *server.MCPServer, mc *mcpConfig) {
	// Annotations are derived from each tool's classification rather than left
	// at mcp-go's destructive/open-world defaults (go-slide-creator-ccqn).
	addTool := func(s *server.MCPServer, tool mcp.Tool, handler server.ToolHandlerFunc) {
		annotateTool(&tool)
		s.AddTool(tool, handler)
	}
	addTool(s, mcpGenerateTool(), mc.handleGenerate)
	addTool(s, mcpListTemplatesTool(), mc.handleListTemplates)
	addTool(s, mcpGetDataFormatHintsTool(), handleGetDataFormatHints)
	addTool(s, mcpGetChartCapabilitiesTool(), handleGetChartCapabilities)
	addTool(s, mcpGetDiagramCapabilitiesTool(), handleGetDiagramCapabilities)
	addTool(s, mcpValidateTool(), mc.handleValidate)
	addTool(s, mcpRecommendPatternTool(), mc.handleRecommendPattern)
	addTool(s, mcpRecommendVisualTool(), mc.handleRecommendVisual)
	addTool(s, mcpListPatternsTool(), handleListPatterns)
	addTool(s, mcpShowPatternTool(), handleShowPattern)
	addTool(s, mcpValidatePatternTool(), mc.handleValidatePattern)
	addTool(s, mcpExpandPatternTool(), mc.handleExpandPattern)
	addTool(s, mcpExpandPatternsTool(), mc.handleExpandPatterns)
	addTool(s, mcpListIconsTool(), handleListIcons)
	addTool(s, mcpPreviewIconTool(), mc.handlePreviewIcon)
	addTool(s, mcpGetShapeCatalogTool(), handleGetShapeCatalog)
	addTool(s, mcpTableDensityGuideTool(), mc.handleTableDensityGuide)
	addTool(s, mcpResolveThemeTool(), mc.handleResolveTheme)
	addTool(s, mcpRenderSlideImageTool(), mc.handleRenderSlideImage)
	addTool(s, mcpRenderSlideImageFromJSONTool(), mc.handleRenderSlideImageFromJSON)
	addTool(s, mcpRenderDeckThumbnailsTool(), mc.handleRenderDeckThumbnails)
	addTool(s, mcpPurgeRenderCacheTool(), handlePurgeRenderCache)
	addTool(s, mcpScoreDeckTool(), mc.handleScoreDeck)
	addTool(s, mcpScoreCandidatesTool(), mc.handleScoreCandidates)
	addTool(s, mcpInspectSlideImagesTool(), mc.handleInspectSlideImages)
	addTool(s, mcpPreviewPlanTool(), mc.handlePreviewPlan)
	addTool(s, mcpPreviewSlideWireframeTool(), mc.handlePreviewSlideWireframe)
	addTool(s, mcpRepairSlideTool(), mc.handleRepairSlide)
	addTool(s, mcpRepairSlidesBatchTool(), mc.handleRepairSlidesBatch)
	addTool(s, mcpProposeRepairsTool(), mc.handleProposeRepairs)
	addTool(s, mcpSubmitVisualReviewTool(), handleSubmitVisualReview)
	addTool(s, mcpAutoRepairTool(), mc.handleAutoRepair)
	addTool(s, mcpMakeDeckTool(), mc.handleMakeDeck)
	addTool(s, mcpListTemplateSettingsTool(), mc.handleListTemplateSettings)
	addTool(s, mcpRegisterTemplateSettingTool(), mc.handleRegisterTemplateSetting)
	addTool(s, mcpDeleteTemplateSettingTool(), mc.handleDeleteTemplateSetting)
	addTool(s, mcpAnalyzeDeckRhythmTool(), mc.handleAnalyzeDeckRhythm)
	addTool(s, mcpPlanDeckTool(), mc.handlePlanDeck)
	addTool(s, mcpGetCapabilitiesTool(), mc.handleGetCapabilities)
	addTool(s, mcpGetStartedTool(), mc.handleGetStarted)
	addTool(s, mcpGetInputSchemaTool(), handleGetInputSchema)
	addTool(s, mcpReadPresentationTool(), handleReadPresentation)
	addTool(s, mcpExportDeckTool(), mc.handleExportDeck)
	addTool(s, mcpValidateOutputTool(), handleValidateOutput)
	addTool(s, mcpDescribeFindingTool(), handleDescribeFinding)
	addTool(s, mcpAuditPaletteTool(), handleAuditPalette)
	addTool(s, mcpExamineTemplateTool(), mc.handleExamineTemplate)
	addTool(s, mcpApplyDeckPatchTool(), mc.handleApplyDeckPatch)
	// Semantic compiler tools (compact DeckSpec authoring) — recommended default
	// path for new decks; the raw tools above remain available.
	addTool(s, mcpValidateDeckSpecTool(), mc.handleValidateDeckSpec)
	addTool(s, mcpCompileDeckSpecTool(), handleCompileDeckSpec)
	addTool(s, mcpRenderDeckSpecTool(), mc.handleRenderDeckSpec)
	addTool(s, mcpExplainDeckSpecTool(), mc.handleExplainDeckSpec)
	addTool(s, mcpListDeckArchetypesTool(), handleListDeckArchetypes)
	addTool(s, mcpListSlideKindsTool(), handleListSlideKinds)
}
