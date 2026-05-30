package main

import "github.com/mark3labs/mcp-go/server"

// registerMCPTools registers every json2pptx MCP tool against the supplied
// server. It is the single source of truth for the tool catalog so that
// runMCP (production) and the doctor test (coverage audit) cannot drift —
// adding a tool here automatically gets it tested.
func registerMCPTools(s *server.MCPServer, mc *mcpConfig) {
	s.AddTool(mcpGenerateTool(), mc.handleGenerate)
	s.AddTool(mcpListTemplatesTool(), mc.handleListTemplates)
	s.AddTool(mcpGetDataFormatHintsTool(), handleGetDataFormatHints)
	s.AddTool(mcpGetChartCapabilitiesTool(), handleGetChartCapabilities)
	s.AddTool(mcpGetDiagramCapabilitiesTool(), handleGetDiagramCapabilities)
	s.AddTool(mcpValidateTool(), mc.handleValidate)
	s.AddTool(mcpRecommendPatternTool(), mc.handleRecommendPattern)
	s.AddTool(mcpRecommendVisualTool(), mc.handleRecommendVisual)
	s.AddTool(mcpListPatternsTool(), handleListPatterns)
	s.AddTool(mcpShowPatternTool(), handleShowPattern)
	s.AddTool(mcpValidatePatternTool(), handleValidatePattern)
	s.AddTool(mcpExpandPatternTool(), mc.handleExpandPattern)
	s.AddTool(mcpExpandPatternsTool(), mc.handleExpandPatterns)
	s.AddTool(mcpListIconsTool(), handleListIcons)
	s.AddTool(mcpPreviewIconTool(), mc.handlePreviewIcon)
	s.AddTool(mcpGetShapeCatalogTool(), handleGetShapeCatalog)
	s.AddTool(mcpTableDensityGuideTool(), mc.handleTableDensityGuide)
	s.AddTool(mcpResolveThemeTool(), mc.handleResolveTheme)
	s.AddTool(mcpRenderSlideImageTool(), mc.handleRenderSlideImage)
	s.AddTool(mcpRenderSlideImageFromJSONTool(), mc.handleRenderSlideImageFromJSON)
	s.AddTool(mcpRenderDeckThumbnailsTool(), mc.handleRenderDeckThumbnails)
	s.AddTool(mcpScoreDeckTool(), mc.handleScoreDeck)
	s.AddTool(mcpScoreCandidatesTool(), mc.handleScoreCandidates)
	s.AddTool(mcpInspectSlideImagesTool(), mc.handleInspectSlideImages)
	s.AddTool(mcpPreviewPlanTool(), mc.handlePreviewPlan)
	s.AddTool(mcpPreviewSlideWireframeTool(), mc.handlePreviewSlideWireframe)
	s.AddTool(mcpRepairSlideTool(), mc.handleRepairSlide)
	s.AddTool(mcpRepairSlidesBatchTool(), mc.handleRepairSlidesBatch)
	s.AddTool(mcpProposeRepairsTool(), mc.handleProposeRepairs)
	s.AddTool(mcpAutoRepairTool(), mc.handleAutoRepair)
	s.AddTool(mcpMakeDeckTool(), mc.handleMakeDeck)
	s.AddTool(mcpListTemplateSettingsTool(), mc.handleListTemplateSettings)
	s.AddTool(mcpRegisterTemplateSettingTool(), mc.handleRegisterTemplateSetting)
	s.AddTool(mcpDeleteTemplateSettingTool(), mc.handleDeleteTemplateSetting)
	s.AddTool(mcpAnalyzeDeckRhythmTool(), handleAnalyzeDeckRhythm)
	s.AddTool(mcpPlanDeckTool(), mc.handlePlanDeck)
	s.AddTool(mcpGetCapabilitiesTool(), mc.handleGetCapabilities)
	s.AddTool(mcpGetStartedTool(), handleGetStarted)
	s.AddTool(mcpGetInputSchemaTool(), handleGetInputSchema)
	s.AddTool(mcpReadPresentationTool(), handleReadPresentation)
	s.AddTool(mcpValidateOutputTool(), handleValidateOutput)
	s.AddTool(mcpDescribeFindingTool(), handleDescribeFinding)
	s.AddTool(mcpAuditPaletteTool(), handleAuditPalette)
	s.AddTool(mcpExamineTemplateTool(), mc.handleExamineTemplate)
	s.AddTool(mcpApplyDeckPatchTool(), mc.handleApplyDeckPatch)
	// Semantic compiler tools (compact DeckSpec authoring) — recommended default
	// path for new decks; the raw tools above remain available.
	s.AddTool(mcpValidateDeckSpecTool(), handleValidateDeckSpec)
	s.AddTool(mcpCompileDeckSpecTool(), handleCompileDeckSpec)
	s.AddTool(mcpRenderDeckSpecTool(), mc.handleRenderDeckSpec)
	s.AddTool(mcpExplainDeckSpecTool(), handleExplainDeckSpec)
	s.AddTool(mcpListDeckArchetypesTool(), handleListDeckArchetypes)
	s.AddTool(mcpListSlideKindsTool(), handleListSlideKinds)
}
