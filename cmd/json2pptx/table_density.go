package main

import (
	"context"
	"fmt"
	"path/filepath"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/template"
)

// mcpTableDensityGuideTool returns the MCP tool definition for table_density_guide.
func mcpTableDensityGuideTool() mcp.Tool {
	return mcp.NewTool("table_density_guide",
		mcp.WithDescription("Get table density and sizing recommendations for shape_grid tables. Returns structured density tiers with font sizes, max rows/columns, and TDR ceiling per tier. Optionally scoped to a specific template (includes its table_styles) or style_id."),
		mcp.WithString("template",
			mcp.Description("Template name to get template-specific recommendations and table_styles (optional)."),
		),
		mcp.WithString("style_id",
			mcp.Description("Table style ID to get density profile for a specific style (optional). Use list_templates to discover available style IDs."),
		),
	)
}

// densityTier describes a single row in the table density reference.
type densityTier struct {
	DataRows    string `json:"data_rows"`
	FontSize    string `json:"font_size"`
	MaxColumns  int    `json:"max_columns"`
	TDRCeiling  int    `json:"tdr_ceiling"`
	Notes       string `json:"notes"`
}

// densityLimits are the hard limits from Rule 20 (TDR).
type densityLimits struct {
	MaxRows      int    `json:"max_rows"`
	MaxColumns   int    `json:"max_columns"`
	MinFontPt    int    `json:"min_font_pt"`
	SplitAdvice  string `json:"split_advice"`
}

// densityGuideResponse is the JSON envelope for table_density_guide.
type densityGuideResponse struct {
	Tiers        []densityTier      `json:"tiers"`
	Limits       densityLimits      `json:"limits"`
	MultilineNote string            `json:"multiline_note"`
	TableStyles  []skillTableStyle  `json:"table_styles,omitempty"`
	Template     string             `json:"template,omitempty"`
}

// buildDensityTiers returns the standard density reference tiers.
func buildDensityTiers() []densityTier {
	return []densityTier{
		{DataRows: "1-4", FontSize: "12-14", MaxColumns: 6, TDRCeiling: 60, Notes: "Default font works; keep spacing generous"},
		{DataRows: "5-7", FontSize: "10-11", MaxColumns: 6, TDRCeiling: 80, Notes: "Explicit font_size required"},
		{DataRows: "8-10", FontSize: "9", MaxColumns: 6, TDRCeiling: 100, Notes: "Use bounds.y: 15, row_gap: 1-2"},
		{DataRows: "11-13", FontSize: "8", MaxColumns: 5, TDRCeiling: 120, Notes: "Tight; consider dropping a column"},
		{DataRows: "14-16", FontSize: "7", MaxColumns: 4, TDRCeiling: 120, Notes: "The last stop before splitting"},
		{DataRows: "17+", FontSize: "-", MaxColumns: 0, TDRCeiling: 0, Notes: "Split across two slides via split_slide"},
	}
}

func (mc *mcpConfig) handleTableDensityGuide(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := densityGuideResponse{
		Tiers: buildDensityTiers(),
		Limits: densityLimits{
			MaxRows:     7,
			MaxColumns:  6,
			MinFontPt:   9,
			SplitAdvice: "Use split_slide with by:table.rows for tables exceeding 17 data rows",
		},
		MultilineNote: "A cell with N text lines at a given font_size needs roughly the same vertical space as N single-line rows. Count each line as a row when sizing.",
	}

	templateName, _ := request.RequireString("template")
	if templateName != "" {
		resp.Template = templateName

		path := filepath.Join(mc.templatesDir, templateName+".pptx")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return mcp.NewToolResultError(templateNotFoundError(templateName, mc.templatesDir)), nil
		}

		reader, err := template.OpenTemplate(path)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("template analysis failed: %v", err)), nil
		}
		defer func() { _ = reader.Close() }()

		entries := reader.TableStyles()
		styles := make([]skillTableStyle, len(entries))
		for i, e := range entries {
			styles[i] = skillTableStyle{ID: e.ID, Name: e.Name}
		}
		resp.TableStyles = styles
	}

	// If style_id is provided, filter to just that style.
	if styleID, err := request.RequireString("style_id"); err == nil && styleID != "" {
		if resp.TableStyles != nil {
			var filtered []skillTableStyle
			for _, s := range resp.TableStyles {
				if s.ID == styleID {
					filtered = append(filtered, s)
				}
			}
			if len(filtered) == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("style_id %q not found in template %q; use list_templates to see available table_styles", styleID, templateName)), nil
			}
			resp.TableStyles = filtered
		} else {
			return mcp.NewToolResultError("style_id requires template parameter"), nil
		}
	}

	responseJSON, err := api.MarshalMCPResponse(ctx, resp)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(responseJSON)), nil
}
