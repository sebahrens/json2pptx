// mcp_repair.go implements the repair_slide MCP tool — incremental targeted
// slide edits using the Fix.Kind vocabulary from fit findings. Instead of
// regenerating an entire deck, agents send a single slide index and a list
// of fix directives. The tool patches the deck and returns the result.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
)

// --- Response types ---

// repairSlideOutput is the top-level response for repair_slide.
type repairSlideOutput struct {
	PatchedDeck  json.RawMessage       `json:"patched_deck"`
	AppliedFixes []appliedFix          `json:"applied_fixes"`
	NewFindings  []patterns.FitFinding `json:"new_findings,omitempty"`
}

// appliedFix reports whether a single fix directive was successfully applied.
type appliedFix struct {
	Kind    string `json:"kind"`
	Applied bool   `json:"applied"`
	Message string `json:"message,omitempty"`
}

// repairFixInput is one fix directive from the caller.
type repairFixInput struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params,omitempty"`
}

// --- Tool definition ---

func mcpRepairSlideTool() mcp.Tool {
	return mcp.NewTool("repair_slide",
		mcp.WithDescription(`Apply targeted fixes to a single slide without regenerating the entire deck. Accepts the full deck JSON, a slide index (0-based), and a list of fix directives using the same Fix.Kind vocabulary that fit_report emits.

Returns the patched deck JSON, a report of which fixes were applied, and post-patch fit findings for the modified slide.

All fix kinds accept an optional "path" parameter (JSON Pointer, RFC 6901) to disambiguate which content element to target. When omitted, the fix applies to the first matching element on the slide.

Supported fix kinds (V1):
- reduce_text: Truncate bullets/body text. Params: path (string, optional), max_items (int, for bullets), max_length (int, for text).
- shorten_title: Truncate a title to max_length characters. Params: path (string, optional), max_length (int).
- split_at_row: Split a table across pages using the split_slide envelope. Params: path (string, optional), row (int, rows per page), title_suffix (string, optional), repeat_headers (bool, optional).
- swap_layout: Change the slide's layout_id. Params: layout_id (string, required).
- use_one_of: Replace a field value with a valid option. Params: path (string), value (string).
- replace_color: Replace one color with another in shape_grid fills. Params: from (string, color to find), to (string, replacement color). Also accepts original_color/replacement_color from contrast_autofixed findings.
- use_semantic_color: Replace a hex fill with a semantic scheme color. Params: path (string, JSON Pointer e.g. "/slides/0/shape_grid/rows/0/cells/0/shape/fill"), value (string, scheme name e.g. "accent1").

Unsupported kinds return {applied: false, message: "kind_not_supported"} — agents can fall back to full regeneration.`),
		mcp.WithRawOutputSchema(outputSchemaRepairSlide),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Full presentation definition. Same schema as generate_presentation.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithNumber("slide_index",
			mcp.Description("0-based index of the slide to repair."),
			mcp.Required(),
		),
		mcp.WithObject("fixes",
			mcp.Description(`Array of fix directives: [{"kind":"reduce_text","params":{"max_items":5}}, ...]. Each directive has a "kind" (string) and optional "params" (object).`),
			mcp.Required(),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleRepairSlide(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "presentation is required"), nil
	}

	// Parse the deck.
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "presentation", fmt.Sprintf("invalid JSON: %v", err)), nil
	}
	applyDefaults(&input)

	// Validate required fields.
	if errResult := validateRepairBoundary(&input); errResult != nil {
		return errResult, nil
	}

	// Extract slide_index.
	slideIdx, err := extractSlideIndex(request, len(input.Slides))
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}

	// Extract fixes array.
	fixes, err := extractFixes(request)
	if err != nil {
		return api.MCPSimpleError("INVALID_PARAMETER", err.Error()), nil
	}
	if len(fixes) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "fixes array must contain at least one fix directive"), nil
	}

	// Apply each fix to the target slide.
	var applied []appliedFix
	for _, fix := range fixes {
		result := applyRepairFix(&input, slideIdx, fix)
		applied = append(applied, result)
	}

	// Resolve template for post-patch fit findings.
	var newFindings []patterns.FitFinding
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
	if err == nil {
		defer templateCleanup()
		reader, err := template.OpenTemplate(templatePath)
		if err == nil {
			defer func() { _ = reader.Close() }()
			layouts, err := template.ParseLayouts(reader)
			if err == nil {
				slideWidth, slideHeight := template.ParseSlideDimensions(reader)
				allFindings := collectFitFindings(&input, layouts, slideWidth, slideHeight)
				// Filter to only findings for the repaired slide (and any slides
				// created by split_at_row, which follow the original index).
				newFindings = filterFindingsForSlide(allFindings, slideIdx)
			}
		}
	}

	// Marshal the patched deck.
	patchedJSON, err := json.Marshal(input)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal patched deck: %v", err)), nil
	}

	output := repairSlideOutput{
		PatchedDeck:  patchedJSON,
		AppliedFixes: applied,
		NewFindings:  newFindings,
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Fix application ---

// applyRepairFix applies a single fix directive to the input, returning the result.
func applyRepairFix(input *PresentationInput, slideIdx int, fix repairFixInput) appliedFix {
	switch fix.Kind {
	case "reduce_text":
		return applyReduceText(input, slideIdx, fix.Params)
	case "shorten_title":
		return applyShortenTitle(input, slideIdx, fix.Params)
	case "split_at_row":
		return applySplitAtRow(input, slideIdx, fix.Params)
	case "swap_layout":
		return applySwapLayout(input, slideIdx, fix.Params)
	case "use_one_of":
		return applyUseOneOf(input, slideIdx, fix.Params)
	case "replace_color":
		return applyReplaceColor(input, slideIdx, fix.Params)
	case "use_semantic_color":
		return applyUseSemanticColor(input, slideIdx, fix.Params)
	default:
		return appliedFix{
			Kind:    fix.Kind,
			Applied: false,
			Message: "kind_not_supported",
		}
	}
}

// applyReduceText truncates bullets or body text on a slide.
// When params["path"] is set (e.g. "/slides/0/content/body"), only the matching
// content item is targeted; otherwise all content on the slide is processed.
func applyReduceText(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := &input.Slides[slideIdx]
	maxItems := intParam(params, "max_items", 0)
	maxLength := intParam(params, "max_length", 0)
	targetPath := stringParam(params, "path", "")

	modified := false
	for i := range slide.Content {
		ci := &slide.Content[i]

		// Skip non-matching content when path is specified.
		if targetPath != "" && !contentMatchesPath(slideIdx, i, ci.PlaceholderID, targetPath) {
			continue
		}

		// Truncate bullets.
		if maxItems > 0 && ci.BulletsValue != nil && len(*ci.BulletsValue) > maxItems {
			trimmed := (*ci.BulletsValue)[:maxItems]
			ci.BulletsValue = &trimmed
			modified = true
		}

		// Truncate body_and_bullets bullets.
		if maxItems > 0 && ci.BodyAndBulletsValue != nil && len(ci.BodyAndBulletsValue.Bullets) > maxItems {
			ci.BodyAndBulletsValue.Bullets = ci.BodyAndBulletsValue.Bullets[:maxItems]
			modified = true
		}

		// Truncate bullet_groups.
		if maxItems > 0 && ci.BulletGroupsValue != nil && len(ci.BulletGroupsValue.Groups) > maxItems {
			ci.BulletGroupsValue.Groups = ci.BulletGroupsValue.Groups[:maxItems]
			modified = true
		}

		// Truncate text by max_length.
		if maxLength > 0 && ci.TextValue != nil && len(*ci.TextValue) > maxLength {
			truncated := (*ci.TextValue)[:maxLength]
			ci.TextValue = &truncated
			modified = true
		}
	}

	if modified {
		return appliedFix{Kind: "reduce_text", Applied: true}
	}
	return appliedFix{Kind: "reduce_text", Applied: false, Message: "no text content found to reduce on this slide"}
}

// applyShortenTitle truncates the title placeholder text.
// When params["path"] is set, the specific placeholder is targeted by path.
func applyShortenTitle(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := &input.Slides[slideIdx]
	maxLength := intParam(params, "max_length", 50) // default 50 chars
	targetPath := stringParam(params, "path", "")

	for i := range slide.Content {
		ci := &slide.Content[i]
		if ci.PlaceholderID != "title" {
			continue
		}
		if targetPath != "" && !contentMatchesPath(slideIdx, i, ci.PlaceholderID, targetPath) {
			continue
		}
		if ci.TextValue != nil {
			if len(*ci.TextValue) > maxLength {
				truncated := (*ci.TextValue)[:maxLength]
				ci.TextValue = &truncated
				return appliedFix{Kind: "shorten_title", Applied: true}
			}
			return appliedFix{Kind: "shorten_title", Applied: false, Message: "title already within max_length"}
		}
	}
	return appliedFix{Kind: "shorten_title", Applied: false, Message: "no title placeholder found on this slide"}
}

// applySplitAtRow wraps the target slide in a split_slide envelope, delegating
// to the existing split_slide machinery.
// When params["path"] is set, the specific content-level table is targeted.
func applySplitAtRow(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	slide := input.Slides[slideIdx]
	targetPath := stringParam(params, "path", "")

	// Check that the slide has a table — use path to disambiguate if given.
	tableIdx := -1
	if targetPath != "" {
		for i := range slide.Content {
			if slide.Content[i].Type == "table" && contentMatchesPath(slideIdx, i, slide.Content[i].PlaceholderID, targetPath) {
				tableIdx = i
				break
			}
		}
	} else {
		tableIdx, _ = findTableContent(slide.Content)
	}
	if tableIdx < 0 {
		return appliedFix{Kind: "split_at_row", Applied: false, Message: "slide has no table content to split"}
	}

	groupSize := intParam(params, "row", 0)
	if groupSize <= 0 {
		groupSize = intParam(params, "group_size", 0)
	}
	if groupSize <= 0 {
		return appliedFix{Kind: "split_at_row", Applied: false, Message: "row (rows per page) parameter is required and must be > 0"}
	}

	titleSuffix := stringParam(params, "title_suffix", " ({page}/{total})")
	repeatHeaders := boolParam(params, "repeat_headers", true)

	splitInput := SplitSlideInput{
		Type: "split_slide",
		Base: slide,
		Split: SplitConfig{
			By:            "table.rows",
			GroupSize:     groupSize,
			TitleSuffix:   titleSuffix,
			RepeatHeaders: repeatHeaders,
		},
	}

	expanded, err := expandSplitSlide(splitInput)
	if err != nil {
		return appliedFix{Kind: "split_at_row", Applied: false, Message: fmt.Sprintf("split failed: %v", err)}
	}

	// Replace the original slide with the expanded slides.
	newSlides := make([]SlideInput, 0, len(input.Slides)-1+len(expanded))
	newSlides = append(newSlides, input.Slides[:slideIdx]...)
	newSlides = append(newSlides, expanded...)
	newSlides = append(newSlides, input.Slides[slideIdx+1:]...)
	input.Slides = newSlides

	return appliedFix{
		Kind:    "split_at_row",
		Applied: true,
		Message: fmt.Sprintf("split into %d slides with %d rows each", len(expanded), groupSize),
	}
}

// applySwapLayout changes the slide's layout_id.
func applySwapLayout(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	layoutID := stringParam(params, "layout_id", "")
	if layoutID == "" {
		return appliedFix{Kind: "swap_layout", Applied: false, Message: "layout_id parameter is required"}
	}

	input.Slides[slideIdx].LayoutID = layoutID
	return appliedFix{Kind: "swap_layout", Applied: true}
}

// applyUseOneOf replaces a specific field value on the slide.
func applyUseOneOf(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	value := stringParam(params, "value", "")

	if value == "" {
		return appliedFix{Kind: "use_one_of", Applied: false, Message: "value parameter is required"}
	}

	slide := &input.Slides[slideIdx]

	// Handle common paths.
	switch path {
	case "layout_id":
		slide.LayoutID = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	case "transition":
		slide.Transition = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	case "transition_speed":
		slide.TransitionSpeed = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	case "build":
		slide.Build = value
		return appliedFix{Kind: "use_one_of", Applied: true}
	default:
		// For content-level paths, try to match placeholder_id.type
		for i := range slide.Content {
			ci := &slide.Content[i]
			if path == fmt.Sprintf("content[%d].type", i) || path == ci.PlaceholderID+".type" {
				ci.Type = value
				return appliedFix{Kind: "use_one_of", Applied: true}
			}
		}
		return appliedFix{Kind: "use_one_of", Applied: false, Message: fmt.Sprintf("path %q not recognized for slide-level use_one_of", path)}
	}
}

// applyReplaceColor replaces occurrences of a specific color in shape_grid fills.
// Accepts params from contrast_autofixed findings (original_color/replacement_color)
// or the canonical form (from/to).
func applyReplaceColor(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	from := stringParam(params, "from", "")
	to := stringParam(params, "to", "")

	// Also accept the names emitted by contrast_autofixed findings.
	if from == "" {
		from = stringParam(params, "original_color", "")
	}
	if to == "" {
		to = stringParam(params, "replacement_color", "")
	}

	if from == "" || to == "" {
		return appliedFix{Kind: "replace_color", Applied: false, Message: "from/to (or original_color/replacement_color) parameters are required"}
	}

	slide := &input.Slides[slideIdx]
	if slide.ShapeGrid == nil {
		return appliedFix{Kind: "replace_color", Applied: false, Message: "slide has no shape_grid"}
	}

	modified := replaceColorInShapeGrid(slide.ShapeGrid, from, to)
	if modified {
		return appliedFix{Kind: "replace_color", Applied: true}
	}
	return appliedFix{Kind: "replace_color", Applied: false, Message: fmt.Sprintf("color %q not found in shape_grid fills", from)}
}

// applyUseSemanticColor replaces a hex fill at a specific path with a semantic
// scheme color name (e.g. "accent1", "dk1").
func applyUseSemanticColor(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	path := stringParam(params, "path", "")
	value := stringParam(params, "value", "")

	if value == "" {
		return appliedFix{Kind: "use_semantic_color", Applied: false, Message: "value parameter is required (scheme color name, e.g. accent1)"}
	}

	slide := &input.Slides[slideIdx]
	if slide.ShapeGrid == nil {
		return appliedFix{Kind: "use_semantic_color", Applied: false, Message: "slide has no shape_grid"}
	}

	// If a path is provided, try to resolve it to a specific cell fill.
	if path != "" {
		if setFillAtPath(slide.ShapeGrid, slideIdx, path, value) {
			return appliedFix{Kind: "use_semantic_color", Applied: true}
		}
		return appliedFix{Kind: "use_semantic_color", Applied: false, Message: fmt.Sprintf("path %q not found or not a fill field", path)}
	}

	// Without a path, replace all hex fills on the slide with the semantic color.
	modified := replaceAllHexFills(slide.ShapeGrid, value)
	if modified {
		return appliedFix{Kind: "use_semantic_color", Applied: true}
	}
	return appliedFix{Kind: "use_semantic_color", Applied: false, Message: "no hex fills found in shape_grid"}
}

// replaceColorInShapeGrid walks all cells in a shape grid and replaces fill
// colors matching `from` with `to`.
func replaceColorInShapeGrid(grid *ShapeGridInput, from, to string) bool {
	modified := false
	fromNorm := normalizeColor(from)
	for ri := range grid.Rows {
		for ci := range grid.Rows[ri].Cells {
			cell := grid.Rows[ri].Cells[ci]
			if cell == nil || cell.Shape == nil {
				continue
			}
			if replaceFillColor(cell.Shape, fromNorm, to) {
				modified = true
			}
		}
	}
	return modified
}

// replaceFillColor replaces a fill color on a shape spec if it matches fromNorm.
func replaceFillColor(shape *ShapeSpecInput, fromNorm, to string) bool {
	if len(shape.Fill) == 0 {
		return false
	}

	// Try string form.
	var s string
	if err := json.Unmarshal(shape.Fill, &s); err == nil {
		if normalizeColor(s) == fromNorm {
			newFill, _ := json.Marshal(to)
			shape.Fill = newFill
			return true
		}
		return false
	}

	// Try object form.
	var obj ShapeFillInput
	if err := json.Unmarshal(shape.Fill, &obj); err == nil {
		if normalizeColor(obj.Color) == fromNorm {
			obj.Color = to
			newFill, _ := json.Marshal(obj)
			shape.Fill = newFill
			return true
		}
	}
	return false
}

// setFillAtPath sets the fill color at a specific path like
// "/slides/N/shape_grid/rows/R/cells/C/shape/fill".
func setFillAtPath(grid *ShapeGridInput, slideIdx int, path, value string) bool {
	pathSlideIdx, rowIdx, cellIdx, ok := slidepath.ParseGridCell(path)
	if !ok {
		return false
	}
	// Verify the slide index matches.
	if pathSlideIdx != slideIdx {
		return false
	}
	if rowIdx < 0 || rowIdx >= len(grid.Rows) {
		return false
	}
	if cellIdx < 0 || cellIdx >= len(grid.Rows[rowIdx].Cells) {
		return false
	}
	cell := grid.Rows[rowIdx].Cells[cellIdx]
	if cell == nil || cell.Shape == nil {
		return false
	}
	newFill, _ := json.Marshal(value)
	cell.Shape.Fill = newFill
	return true
}

// replaceAllHexFills replaces all hex fill colors in a shape grid with a semantic color.
func replaceAllHexFills(grid *ShapeGridInput, semanticColor string) bool {
	modified := false
	for ri := range grid.Rows {
		for ci := range grid.Rows[ri].Cells {
			cell := grid.Rows[ri].Cells[ci]
			if cell == nil || cell.Shape == nil || len(cell.Shape.Fill) == 0 {
				continue
			}
			// Check string form.
			var s string
			if err := json.Unmarshal(cell.Shape.Fill, &s); err == nil {
				if isHexColor(s) {
					newFill, _ := json.Marshal(semanticColor)
					cell.Shape.Fill = newFill
					modified = true
				}
				continue
			}
			// Check object form.
			var obj ShapeFillInput
			if err := json.Unmarshal(cell.Shape.Fill, &obj); err == nil {
				if isHexColor(obj.Color) {
					obj.Color = semanticColor
					newFill, _ := json.Marshal(obj)
					cell.Shape.Fill = newFill
					modified = true
				}
			}
		}
	}
	return modified
}

// normalizeColor normalizes a color string for comparison by lowercasing and
// stripping leading "#".
func normalizeColor(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	c = strings.TrimPrefix(c, "#")
	return c
}

// isHexColor reports whether a string looks like a hex color (#RGB, #RRGGBB, or without #).
func isHexColor(s string) bool {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 3 && len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// contentMatchesPath reports whether a content item at the given slide/content
// index matches the provided JSON Pointer path. Matches by placeholder name
// (e.g. "/slides/0/content/body") or by array index (e.g. "/slides/0/content/1").
func contentMatchesPath(slideIdx, contentIdx int, placeholderID, path string) bool {
	// Match by placeholder name.
	if path == slidepath.Content(slideIdx, placeholderID) {
		return true
	}
	// Match by array index.
	if path == slidepath.ContentIndex(slideIdx, contentIdx) {
		return true
	}
	// Also match if path is a sub-path (e.g. "/slides/0/content/body/text").
	byName := slidepath.Content(slideIdx, placeholderID)
	if slidepath.HasPrefix(path, byName) {
		return true
	}
	return slidepath.HasPrefix(path, slidepath.ContentIndex(slideIdx, contentIdx))
}

// --- Helpers ---

// validateRepairBoundary checks required fields for repair_slide.
func validateRepairBoundary(input *PresentationInput) *mcp.CallToolResult {
	var diags []diagnostics.Diagnostic
	if input.Template == "" {
		diags = append(diags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required",
			Severity: diagnostics.SeverityError,
		})
	}
	if len(input.Slides) == 0 {
		diags = append(diags, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity: diagnostics.SeverityError,
		})
	}
	if diagnostics.HasErrors(diags) {
		return api.MCPDiagnosticsError(diags)
	}
	return nil
}

// extractSlideIndex extracts and validates the slide_index parameter.
func extractSlideIndex(request mcp.CallToolRequest, slideCount int) (int, error) {
	args := request.GetArguments()
	raw, ok := args["slide_index"]
	if !ok {
		return 0, fmt.Errorf("slide_index is required")
	}

	// MCP passes numbers as float64.
	var idx int
	switch v := raw.(type) {
	case float64:
		idx = int(v)
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("slide_index must be an integer, got %v", raw)
		}
		idx = int(i)
	default:
		return 0, fmt.Errorf("slide_index must be a number, got %T", raw)
	}

	if idx < 0 || idx >= slideCount {
		return 0, fmt.Errorf("slide_index %d out of range (deck has %d slides, valid range 0-%d)", idx, slideCount, slideCount-1)
	}
	return idx, nil
}

// extractFixes extracts the fixes array from the request.
func extractFixes(request mcp.CallToolRequest) ([]repairFixInput, error) {
	args := request.GetArguments()
	raw, ok := args["fixes"]
	if !ok {
		return nil, fmt.Errorf("fixes is required")
	}

	// Re-marshal and unmarshal to handle the various shapes MCP might send.
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("fixes: %w", err)
	}

	var fixes []repairFixInput
	if err := json.Unmarshal(data, &fixes); err != nil {
		return nil, fmt.Errorf("fixes must be an array of {kind, params?} objects: %w", err)
	}

	return fixes, nil
}

// filterFindingsForSlide returns findings whose path references the given slide index.
func filterFindingsForSlide(findings []patterns.FitFinding, slideIdx int) []patterns.FitFinding {
	prefix := slidepath.Slide(slideIdx)
	var filtered []patterns.FitFinding
	for _, f := range findings {
		if slidepath.HasPrefix(f.Path, prefix) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// intParam extracts an integer parameter with a default.
func intParam(params map[string]any, key string, defaultVal int) int {
	if params == nil {
		return defaultVal
	}
	raw, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case int:
		return v
	}
	return defaultVal
}

// stringParam extracts a string parameter with a default.
func stringParam(params map[string]any, key string, defaultVal string) string {
	if params == nil {
		return defaultVal
	}
	if v, ok := params[key].(string); ok {
		return v
	}
	return defaultVal
}

// boolParam extracts a boolean parameter with a default.
func boolParam(params map[string]any, key string, defaultVal bool) bool {
	if params == nil {
		return defaultVal
	}
	if v, ok := params[key].(bool); ok {
		return v
	}
	return defaultVal
}
