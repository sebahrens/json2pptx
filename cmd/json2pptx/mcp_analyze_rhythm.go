package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/rhythm"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// --- Tool definition ---

func mcpAnalyzeDeckRhythmTool() mcp.Tool {
	return mcp.NewTool("analyze_deck_rhythm",
		mcp.WithDescription(`Analyze a presentation's visual rhythm — pattern repetition, density variation, and accent usage across slides.

Use this BEFORE calling generate_presentation to detect monotony and inform pattern choices. Unlike score_deck (which requires a full generation pass), this tool performs lightweight static analysis on the JSON input.

Returns per-slide fingerprints, pattern run detection, a density coefficient of variation, accent balance, and actionable recommendations for breaking repetitive runs.`),
		mcp.WithRawOutputSchema(outputSchemaAnalyzeDeckRhythm),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description("Presentation definition. Same schema as generate_presentation."),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
	)
}

// --- Handler ---

func handleAnalyzeDeckRhythm(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("analyze_deck_rhythm", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil), nil
	}

	if len(input.Slides) == 0 {
		return argMissing("analyze_deck_rhythm", "presentation.slides", "array", []any{map[string]any{"layout_id": "title"}}, nextCallGetInputSchema()), nil
	}

	result := analyzeDeckRhythm(input.Slides)

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Adapter: SlideInput DTOs -> internal/rhythm engine ---

// analyzeDeckRhythm projects the JSON slide inputs into the package-local
// rhythm.Slide model and runs the extracted analysis engine. Both the
// analyze_deck_rhythm MCP tool and score_deck's composition axis go through
// this single adapter so they share one source of truth for composition
// analysis.
func analyzeDeckRhythm(slides []SlideInput) *rhythm.Result {
	rhythmSlides := make([]rhythm.Slide, len(slides))
	for i, s := range slides {
		rhythmSlides[i] = toRhythmSlide(s)
	}
	return rhythm.Analyze(rhythmSlides)
}

// toRhythmSlide projects a single SlideInput into the analyzer's input model.
// It records the structural flags, content kinds, and accent hints the engine
// needs, and pre-builds a resolution-ready shape_grid for density measurement.
func toRhythmSlide(s SlideInput) rhythm.Slide {
	rs := rhythm.Slide{
		SlideType:    s.SlideType,
		HasPattern:   s.Pattern != nil,
		HasShapeGrid: s.ShapeGrid != nil,
		HasCompose:   s.Compose != nil,
	}
	if s.Pattern != nil {
		rs.PatternName = s.Pattern.Name
	}
	if len(s.Content) > 0 {
		rs.ContentKinds = make([]string, len(s.Content))
		for i, c := range s.Content {
			rs.ContentKinds[i] = c.Type
		}
	}
	if s.ShapeGrid != nil {
		for _, row := range s.ShapeGrid.Rows {
			// cellCount counts every slot (matching len(row.Cells)), including
			// nil/empty cells; accent hints only come from filled shape cells.
			rs.CellCount += len(row.Cells)
			for _, cell := range row.Cells {
				if cell != nil && cell.Shape != nil {
					if fill := extractAccentFromFill(cell.Shape.Fill); fill != "" {
						rs.CellAccents = append(rs.CellAccents, fill)
					}
				}
			}
		}
		rs.Grid = buildDensityGrid(s.ShapeGrid)
	}
	return rs
}

// buildDensityGrid converts a shape_grid DTO into a resolution-ready
// shapegrid.Grid for textcapacity density measurement, using default slide
// dimensions. Returns nil when the grid has no rows or its columns cannot be
// resolved, signaling the analyzer to skip the slide for density accounting.
func buildDensityGrid(sg *ShapeGridInput) *shapegrid.Grid {
	if sg == nil || len(sg.Rows) == 0 {
		return nil
	}

	colWidths, err := resolveColumnsDTO(sg.Columns, sg.Rows)
	if err != nil {
		return nil
	}

	colGap := sg.ColGap
	if colGap == 0 {
		colGap = sg.Gap
	}
	rowGap := sg.RowGap
	if rowGap == 0 {
		rowGap = sg.Gap
	}

	rows := convertGridRows(sg.Rows)

	// Use default slide dimensions for bounds resolution.
	bounds := pptx.RectEmu{
		X:  457200,                          // 0.5in default
		Y:  1600200,                         // ~1.26in default
		CX: shapegrid.DefaultSlideWidthEMU - 2*457200,
		CY: shapegrid.DefaultSlideHeightEMU - 1600200 - 457200,
	}
	if sg.Bounds != nil {
		bounds = shapegrid.BoundsFromPercentages(
			sg.Bounds.X, sg.Bounds.Y,
			sg.Bounds.Width, sg.Bounds.Height,
			shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU,
		)
	}

	return &shapegrid.Grid{
		Bounds:  bounds,
		Columns: colWidths,
		Rows:    rows,
		ColGap:  colGap,
		RowGap:  rowGap,
	}
}

// extractAccentFromFill checks if a fill json.RawMessage references an accent color.
// Fill can be a JSON string ("accent1") or object ({"color":"accent1","alpha":80}).
func extractAccentFromFill(fill json.RawMessage) string {
	if len(fill) == 0 {
		return ""
	}

	// Try string form first.
	var s string
	if err := json.Unmarshal(fill, &s); err == nil {
		if isAccentColor(s) {
			return s
		}
		return ""
	}

	// Try object form.
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(fill, &obj); err == nil && isAccentColor(obj.Color) {
		return obj.Color
	}
	return ""
}

// isAccentColor returns true if the color name is a scheme accent.
func isAccentColor(c string) bool {
	switch c {
	case "accent1", "accent2", "accent3", "accent4", "accent5", "accent6":
		return true
	}
	return false
}
