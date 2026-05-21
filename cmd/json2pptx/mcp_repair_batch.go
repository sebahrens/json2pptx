// mcp_repair_batch.go implements the repair_slides_batch MCP tool — atomic
// multi-slide repair in a single call. Accepts the full deck JSON and an
// ordered list of per-slide fix directives, applies each through the same
// applyRepairFix engine repair_slide uses, then returns the patched deck and
// a fresh deck-wide fit report.
//
// If a single fix fails, the remaining fixes still run; the failed outcome is
// reported with applied:false and a message — the call is "best-effort", not
// transactional. The combined response halves agent round-trip latency for
// the common multi-slide repair plans produced by propose_repairs.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// --- Response types ---

// repairSlidesBatchOutput is the top-level response for repair_slides_batch.
// Shape mirrors repairSlideOutput so agents can reuse the same parser; each
// applied fix carries an explicit slide_index because a single batch can span
// multiple slides.
type repairSlidesBatchOutput struct {
	PatchedDeck  json.RawMessage   `json:"patched_deck"`
	AppliedFixes []batchAppliedFix `json:"applied_fixes"`
	// Findings is the deck-wide post-batch FindingEnvelope. As with
	// repair_slide.findings it is always present so an agent can branch on
	// findings.ok; findings.findings[] is empty when no residual issue remains.
	// Replaces the legacy new_findings []FitFinding array — agents can reuse the
	// same parser across both repair tools. See docs/AGENT_DIAGNOSTICS.md.
	Findings diagnostics.FindingEnvelope `json:"findings"`
}

// batchAppliedFix is the per-fix outcome. Same fields as appliedFix plus the
// slide_index the fix targeted, so the caller can correlate outcomes with the
// directive that produced them.
type batchAppliedFix struct {
	SlideIndex int    `json:"slide_index"`
	Kind       string `json:"kind"`
	Applied    bool   `json:"applied"`
	Message    string `json:"message,omitempty"`

	Code           string                       `json:"code,omitempty"`
	SupportedKinds []string                     `json:"supported_kinds,omitempty"`
	NextToolCall   *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
}

// batchFixInput is one directive in the request fixes[] array. SlideIndex is
// a pointer so we can distinguish "omitted" from "explicit 0".
type batchFixInput struct {
	SlideIndex *int           `json:"slide_index"`
	Kind       string         `json:"kind"`
	Params     map[string]any `json:"params,omitempty"`
}

// --- Tool definition ---

func mcpRepairSlidesBatchTool() mcp.Tool {
	return mcp.NewTool("repair_slides_batch",
		mcp.WithDescription(`Apply targeted fixes to multiple slides in a single call. Same Fix.Kind vocabulary as repair_slide (see that tool's description for the full kind catalogue), but accepts an ordered fixes[] array where each entry carries its own slide_index. Fixes run in order against the same in-memory deck; later fixes observe earlier patches. A single failed fix does NOT abort the batch — its outcome is reported with applied:false and the next directive still runs.

Use this instead of repair_slide when fit-report flags multiple slides and the agent already knows the full repair plan (typically produced by propose_repairs). It halves round-trip latency and gives the engine a single chance to compute the post-batch fit report.

Returns the patched deck JSON, one outcome per directive (including slide_index, kind, applied, and a human-readable message), and a fresh deck-wide fit report after all fixes have been applied.`),
		mcp.WithRawOutputSchema(outputSchemaRepairSlidesBatch),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description(`Full presentation definition. Same schema as generate_presentation / repair_slide.`),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
		mcp.WithArray("fixes",
			mcp.Required(),
			mcp.Description(`Ordered array of {slide_index, kind, params?} directives. slide_index is a 0-based slide offset; kind is one of the repair_slide fix kinds; params follows the same per-kind schema repair_slide accepts.`),
		),
	)
}

// --- Handler ---

func (mc *mcpConfig) handleRepairSlidesBatch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return argMissing("repair_slides_batch", "presentation", "object", map[string]any{
			"template": "<template-name>",
			"slides":   []any{},
		}, nextCallGetInputSchema()), nil
	}

	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return argInvalidJSON("presentation", fmt.Sprintf("invalid JSON: %v", err), "object", nil, nil), nil
	}
	applyDefaults(&input)

	if errResult := validateRepairBoundary(&input); errResult != nil {
		return errResult, nil
	}

	fixes, err := extractBatchFixes(request, len(input.Slides))
	if err != nil {
		return argInvalidValue("repair_slides_batch", "INVALID_PARAMETER", "fixes", err.Error(), "array", []any{map[string]any{"slide_index": 0, "kind": "reduce_text"}}, nil), nil
	}
	if len(fixes) == 0 {
		return argMissing("repair_slides_batch", "fixes", "array", []any{map[string]any{"slide_index": 0, "kind": "reduce_text", "params": map[string]any{"max_items": 5}}}, nil), nil
	}

	applied := make([]batchAppliedFix, 0, len(fixes))
	for _, f := range fixes {
		res := applyRepairFix(&input, *f.SlideIndex, repairFixInput{Kind: f.Kind, Params: f.Params})
		applied = append(applied, batchAppliedFix{
			SlideIndex:     *f.SlideIndex,
			Kind:           res.Kind,
			Applied:        res.Applied,
			Message:        res.Message,
			Code:           res.Code,
			SupportedKinds: res.SupportedKinds,
			NextToolCall:   res.NextToolCall,
		})
	}

	// Recompute deck-wide fit findings against the patched input so the agent
	// can decide whether another repair pass is needed.
	var newFindings []patterns.FitFinding
	templatePath, templateCleanup, terr := resolveTemplatePath(input.Template, mc.templatesDir)
	if terr == nil {
		defer templateCleanup()
		reader, terr := template.OpenTemplate(templatePath)
		if terr == nil {
			defer func() { _ = reader.Close() }()
			layouts, lerr := template.ParseLayouts(reader)
			if lerr == nil {
				slideWidth, slideHeight := template.ParseSlideDimensions(reader)
				theme := template.ParseTheme(reader)
				newFindings = collectFitFindings(&input, layouts, slideWidth, slideHeight, &theme)
			}
		}
	}

	patchedJSON, err := json.Marshal(input)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal patched deck: %v", err)), nil
	}

	output := repairSlidesBatchOutput{
		PatchedDeck:  patchedJSON,
		AppliedFixes: applied,
		Findings: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "repair_slides_batch",
			Template:    input.Template,
			InputSHA256: diagnostics.ComputeInputSHA256([]byte(jsonStr)),
		}, diagnostics.FromFitFindings(newFindings)),
	}

	mcpResult, err := api.MCPSuccessResult(ctx, output)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// extractBatchFixes reads, parses, and validates the fixes[] array. Each
// directive must carry an explicit slide_index in [0, slideCount). A missing
// or out-of-range index is rejected up-front so the caller knows the batch
// did not start.
func extractBatchFixes(request mcp.CallToolRequest, slideCount int) ([]batchFixInput, error) {
	args := request.GetArguments()
	raw, ok := args["fixes"]
	if !ok {
		return nil, fmt.Errorf("fixes is required")
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("fixes: %w", err)
	}

	var fixes []batchFixInput
	if err := json.Unmarshal(data, &fixes); err != nil {
		return nil, fmt.Errorf("fixes must be an array of {slide_index, kind, params?} objects: %w", err)
	}

	for i, f := range fixes {
		if f.SlideIndex == nil {
			return nil, fmt.Errorf("fixes[%d].slide_index is required", i)
		}
		idx := *f.SlideIndex
		if idx < 0 || idx >= slideCount {
			return nil, fmt.Errorf("fixes[%d].slide_index %d out of range (deck has %d slides, valid range 0-%d)", i, idx, slideCount, slideCount-1)
		}
	}

	return fixes, nil
}
