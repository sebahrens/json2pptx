package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// patternExpansionResult is the per-pattern result emitted by expand_pattern
// (single) and by each element of expand_patterns (batch).
//
// It is the named version of the anonymous struct previously inlined in
// handleExpandPattern; centralising the type lets the batch handler reuse the
// same expansion + analysis pipeline without paying a per-pattern template
// reload.
type patternExpansionResult struct {
	Pattern           string                     `json:"pattern"`
	Version           int                        `json:"version"`
	BoundsSource      string                     `json:"bounds_source"`
	BoundsAssumption  string                     `json:"bounds_assumption"`
	ShapeGrid         *jsonschema.ShapeGridInput `json:"shape_grid"`
	Occupancy         gridOccupancy              `json:"occupancy"`
	CellBudgets       []cellBudgetEntry          `json:"cell_budgets,omitempty"`
	DensityWarnings   []patternValidationError   `json:"density_warnings,omitempty"`
	CapacityWarnings  []cellDensityWarning       `json:"capacity_warnings,omitempty"`
	LayoutSuggestions []layoutSuggestion         `json:"layout_suggestions,omitempty"`
}

// buildPatternExpansionResult expands a single PatternInput against the supplied
// ExpandContext and runs the same density / occupancy / capacity / layout
// analysis as handleExpandPattern. Returns the per-pattern result on success,
// or a *patterns.ValidationError-friendly error.
func buildPatternExpansionResult(
	pi *PatternInput,
	expandCtx patterns.ExpandContext,
	boundsSource string,
	reg *patterns.Registry,
) (patternExpansionResult, error) {
	pat, ok := reg.Get(pi.Name)
	if !ok {
		msg := fmt.Sprintf("unknown pattern %q", pi.Name)
		if suggestion, ok := reg.Suggest(pi.Name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		return patternExpansionResult{}, fmt.Errorf("%s", msg)
	}

	grid, _, err := expandPattern(pi, expandCtx, reg)
	if err != nil {
		return patternExpansionResult{}, err
	}

	densityWarnings := collectGridDensityWarnings(grid)
	attachNextToolCallsToValidationErrors(densityWarnings, pi.Name)

	occupancy := computeGridOccupancy(grid, expandCtx)
	cellBudgets, capacityWarnings := computeCellBudgets(grid, expandCtx)
	attachBoundsHintToCapacityWarnings(capacityWarnings, pi.Name, pi)

	if sparseWarn := sparseLayoutWarning(cellBudgets, pat, pi.Name, pi); sparseWarn != nil {
		capacityWarnings = append(capacityWarnings, *sparseWarn)
	}
	if dcWarn := densityClassWarning(cellBudgets, pat, pi.Name, pi, reg); dcWarn != nil {
		capacityWarnings = append(capacityWarnings, *dcWarn)
	}

	layoutSuggestions := suggestAlternativeLayouts(pat.Name(), cellBudgets, reg)

	boundsAssumption := "full_content_area"
	if grid.Bounds != nil {
		boundsAssumption = "explicit_override"
	}

	return patternExpansionResult{
		Pattern:           pat.Name(),
		Version:           pat.Version(),
		BoundsSource:      boundsSource,
		BoundsAssumption:  boundsAssumption,
		ShapeGrid:         grid,
		Occupancy:         occupancy,
		CellBudgets:       cellBudgets,
		DensityWarnings:   densityWarnings,
		CapacityWarnings:  capacityWarnings,
		LayoutSuggestions: layoutSuggestions,
	}, nil
}

// mcpExpandPatternsTool registers the batch expand_patterns tool: a
// content-aware variant of expand_pattern that expands N patterns under a
// single template load. Closes the recommend-then-N-expansions loop (one
// template parse instead of N).
func mcpExpandPatternsTool() mcp.Tool {
	return mcp.NewTool("expand_patterns",
		mcp.WithDescription("Batch-expand multiple named patterns against the agent's content using a single template load. Returns each pattern's expansion + occupancy + cell_budgets + capacity_warnings + layout_suggestions, computed against the supplied per-pattern values (NOT the pattern's exemplar). Use after recommend_pattern to compare candidates head-to-head with your real content. Patterns missing an entry in `content` fall back to exemplar values; those results are flagged via `used_exemplar=true`. Each candidate that fails validation/expansion is reported via a per-pattern `error` block without aborting the whole batch."),
		mcp.WithRawOutputSchema(outputSchemaExpandPatterns),
		mcp.WithArray("names",
			mcp.Required(),
			mcp.Description("Pattern names to expand (typically the candidates returned by recommend_pattern)."),
		),
		mcp.WithObject("content",
			mcp.Description("Per-pattern content keyed by pattern name. Each entry mirrors expand_pattern parameters: {values, overrides?, cell_overrides?, bounds?, max_height_pct?}. Patterns absent from this map are expanded with their exemplar values (and flagged via used_exemplar=true)."),
		),
		mcp.WithString("theme_template",
			mcp.Description("Template name to use for theme context during expansion. Loaded ONCE for the whole batch. If omitted, a minimal synthesized theme is used for every pattern."),
		),
	)
}

// patternBatchContent is the per-pattern content payload accepted by
// expand_patterns. Mirrors PatternInput minus Name (which is the map key).
type patternBatchContent struct {
	Values        json.RawMessage             `json:"values,omitempty"`
	Overrides     json.RawMessage             `json:"overrides,omitempty"`
	CellOverrides map[string]json.RawMessage  `json:"cell_overrides,omitempty"`
	Bounds        *jsonschema.GridBoundsInput `json:"bounds,omitempty"`
	MaxHeightPct  float64                     `json:"max_height_pct,omitempty"`
}

// batchExpansionEntry is one element of the expand_patterns `results` array.
// Either Error is populated (and the rest may be zero) or the embedded
// patternExpansionResult is fully populated.
type batchExpansionEntry struct {
	Pattern      string                     `json:"pattern"`
	UsedExemplar bool                       `json:"used_exemplar"`
	Error        *patternValidationError    `json:"error,omitempty"`
	Result       *patternExpansionResult    `json:"result,omitempty"`
}

// batchExpansionResponse is the top-level expand_patterns response shape.
type batchExpansionResponse struct {
	BoundsSource string                `json:"bounds_source"`
	Results      []batchExpansionEntry `json:"results"`
}

func (mc *mcpConfig) handleExpandPatterns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:gocognit
	// --- names[] (required) ---
	namesRaw, ok := request.GetArguments()["names"]
	if !ok || namesRaw == nil {
		return api.MCPSimpleError("MISSING_PARAMETER", "names is required"), nil
	}
	namesJSON, err := json.Marshal(namesRaw)
	if err != nil {
		return mcpParseError("INVALID_JSON", "names", fmt.Sprintf("invalid names: %v", err)), nil
	}
	var names []string
	if err := json.Unmarshal(namesJSON, &names); err != nil {
		return mcpParseError("INVALID_JSON", "names", fmt.Sprintf("names must be an array of strings: %v", err)), nil
	}
	if len(names) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "names must contain at least one pattern name"), nil
	}

	// --- content (optional) ---
	contentByName := map[string]patternBatchContent{}
	if contentStr, paramErr := objectParamAsJSON(request, "content"); paramErr != nil {
		return paramErr, nil
	} else if contentStr != "" {
		// Decode as map[string]json.RawMessage first so we can give targeted
		// per-entry parse errors rather than a single opaque one.
		var rawByName map[string]json.RawMessage
		if err := json.Unmarshal([]byte(contentStr), &rawByName); err != nil {
			return mcpParseError("INVALID_JSON", "content", fmt.Sprintf("invalid content JSON: %v", err)), nil
		}
		for k, raw := range rawByName {
			var pc patternBatchContent
			if err := json.Unmarshal(raw, &pc); err != nil {
				return mcpParseError("INVALID_JSON", "content."+k, fmt.Sprintf("invalid content[%q]: %v", k, err)), nil
			}
			contentByName[k] = pc
		}
	}

	// --- resolve ExpandContext ONCE (the whole point of this tool) ---
	templateName, _ := request.RequireString("theme_template")
	expandCtx, boundsSource, err := resolveExpandContext(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", fmt.Sprintf("template %q: %v", templateName, err)), nil
	}

	reg := patterns.Default()

	entries := make([]batchExpansionEntry, 0, len(names))
	for _, name := range names {
		entry := batchExpansionEntry{Pattern: name}

		// Build PatternInput from content (or fall back to exemplar values).
		pi := &PatternInput{Name: name}
		pc, hasContent := contentByName[name]
		if hasContent && len(pc.Values) > 0 {
			pi.Values = pc.Values
			pi.Overrides = pc.Overrides
			pi.CellOverrides = pc.CellOverrides
			pi.Bounds = pc.Bounds
			pi.MaxHeightPct = pc.MaxHeightPct
		} else {
			// Fall back to exemplar values so we can still produce a preview.
			pat, ok := reg.Get(name)
			if !ok {
				msg := fmt.Sprintf("unknown pattern %q", name)
				if suggestion, ok := reg.Suggest(name); ok {
					msg += fmt.Sprintf("; did you mean %q?", suggestion)
				}
				entry.Error = &patternValidationError{
					Field:   "names",
					Code:    "UNKNOWN_PATTERN",
					Message: msg,
				}
				entries = append(entries, entry)
				continue
			}
			ex, ok := pat.(patterns.Exemplar)
			if !ok {
				entry.Error = &patternValidationError{
					Field:   "content." + name + ".values",
					Code:    "MISSING_PARAMETER",
					Message: fmt.Sprintf("pattern %q has no exemplar values; provide content[%q].values", name, name),
				}
				entries = append(entries, entry)
				continue
			}
			exJSON, err := json.Marshal(ex.ExemplarValues())
			if err != nil {
				entry.Error = &patternValidationError{
					Field:   "content." + name + ".values",
					Code:    "INTERNAL",
					Message: fmt.Sprintf("failed to marshal exemplar for %q: %v", name, err),
				}
				entries = append(entries, entry)
				continue
			}
			pi.Values = exJSON
			entry.UsedExemplar = true
		}

		result, err := buildPatternExpansionResult(pi, expandCtx, boundsSource, reg)
		if err != nil {
			// Convert structured validation errors into the same shape used by
			// other pattern tools so agents get a uniform error envelope.
			structured := splitValidationErrors(err)
			attachNextToolCallsToValidationErrors(structured, name)
			if len(structured) > 0 {
				first := structured[0]
				entry.Error = &first
			} else {
				entry.Error = &patternValidationError{
					Field:   "content." + name,
					Message: err.Error(),
				}
			}
			entries = append(entries, entry)
			continue
		}
		entry.Result = &result
		entries = append(entries, entry)
	}

	resp := batchExpansionResponse{
		BoundsSource: boundsSource,
		Results:      entries,
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}
