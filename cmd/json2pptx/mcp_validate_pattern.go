package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// patternWarningFieldRE extracts a pattern-relative cell path from a
// PostExpandWarnings message, such as card-grid's "cells[3].body".
var patternWarningFieldRE = regexp.MustCompile(`\b([a-z][a-z0-9_]*\[\d+\](?:\.[a-z][a-z0-9_]*)+)`)

// handleValidatePattern assesses the same input and template context as
// expand_pattern, but returns diagnostics rather than the expanded grid.
func (mc *mcpConfig) handleValidatePattern(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return argRequired(request, "validate_pattern", "name", "string", "kpi-3up", nextCallListPatterns()), nil
	}
	valuesStr, paramErr := objectParamAsJSON(request, "values")
	if paramErr != nil {
		return paramErr, nil
	}
	if valuesStr == "" {
		return argRequired(request, "validate_pattern", "values", "object", map[string]any{
			"items": []any{map[string]any{"label": "Revenue", "value": "$1.2M"}},
		}, nil), nil
	}

	reg := patterns.Default()
	if _, ok := reg.Get(name); !ok {
		msg := fmt.Sprintf("unknown pattern %q", name)
		fix := &diagnostics.Fix{Kind: "use_one_of"}
		if suggestion, ok := reg.Suggest(name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
			fix = &diagnostics.Fix{Kind: "replace_value", Params: map[string]any{"suggestion": suggestion}}
		}
		return mcpParseErrorWithFix("UNKNOWN_PATTERN", "name", msg, fix), nil
	}

	pi, paramErr := validationPatternInput(request, name, valuesStr)
	if paramErr != nil {
		return paramErr, nil
	}

	templateName, _ := request.RequireString("theme_template")
	expandCtx, boundsSource, err := resolveExpandContext(templateName, mc.templatesDir)
	if err != nil {
		return api.MCPSimpleError("TEMPLATE_NOT_FOUND", fmt.Sprintf("template %q: %v", templateName, err)), nil
	}

	ds := patternValidationDiagnostics(pi, expandCtx, boundsSource, reg)
	inputJSON, _ := json.Marshal(request.GetArguments())
	envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "validate_pattern",
		Template:    templateName,
		InputSHA256: diagnostics.ComputeInputSHA256(inputJSON),
	}, ds)
	mcpResult, err := api.MCPSuccessResult(ctx, envelope)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

func patternValidationDiagnostics(pi *PatternInput, expandCtx patterns.ExpandContext, boundsSource string, reg *patterns.Registry) []diagnostics.Diagnostic {
	result, expandErr := buildPatternExpansionResult(pi, expandCtx, boundsSource, reg)
	var ds []diagnostics.Diagnostic
	if expandErr != nil {
		ds = patternInputDiagnostics(expandErr, "", "")
		if len(ds) == 0 {
			ds = diagnostics.FromJoinedError(expandErr, diagnostics.CodeValidationFailed)
		}
	} else {
		ds = patternExpansionDiagnostics(pi.Name, result)
	}
	return ds
}

func validationPatternInput(request mcp.CallToolRequest, name, valuesStr string) (*PatternInput, *mcp.CallToolResult) {
	pi := &PatternInput{Name: name, Values: json.RawMessage(valuesStr)}
	for _, field := range []string{"overrides", "cell_overrides", "callout", "bounds"} {
		raw, result := objectParamAsJSON(request, field)
		if result != nil {
			return nil, result
		}
		if raw == "" {
			continue
		}
		switch field {
		case "overrides":
			pi.Overrides = json.RawMessage(raw)
		case "cell_overrides":
			if err := json.Unmarshal([]byte(raw), &pi.CellOverrides); err != nil {
				return nil, argInvalidJSON(field, fmt.Sprintf("invalid %s JSON: %v", field, err), "object", nil, nil)
			}
			for key := range pi.CellOverrides {
				if _, err := strconv.Atoi(key); err != nil {
					return nil, argInvalidValue("validate_pattern", "INVALID_KEY", "cell_overrides."+key,
						fmt.Sprintf("cell_overrides key %q is not an integer", key), "integer", 0, nil)
				}
			}
		case "callout":
			var callout patterns.PatternCallout
			if err := json.Unmarshal([]byte(raw), &callout); err != nil {
				return nil, argInvalidJSON(field, fmt.Sprintf("invalid %s JSON: %v", field, err), "object", nil, nil)
			}
			pi.Callout = &callout
		case "bounds":
			var bounds jsonschema.GridBoundsInput
			if err := json.Unmarshal([]byte(raw), &bounds); err != nil {
				return nil, argInvalidJSON(field, fmt.Sprintf("invalid %s JSON: %v", field, err), "object", nil, nil)
			}
			pi.Bounds = &bounds
		}
	}
	if raw, ok := request.GetArguments()["max_height_pct"]; ok && raw != nil {
		if pct, ok := raw.(float64); ok && pct > 0 {
			pi.MaxHeightPct = pct
		}
	}

	return pi, nil
}

func patternExpansionDiagnostics(name string, result patternExpansionResult) []diagnostics.Diagnostic {
	var ds []diagnostics.Diagnostic
	for _, warning := range result.Warnings {
		match := patternWarningRE.FindStringSubmatch(warning)
		code, message := diagnostics.CodeValidationFailed, warning
		if len(match) == 3 {
			code, message = match[1], match[2]
		}
		path := "/values"
		if field := patternWarningFieldRE.FindStringSubmatch(message); len(field) == 2 {
			path += "/" + dottedPathToPointer(field[1])
		}
		severity := diagnostics.SeverityWarning
		if code == patterns.ErrCodeBodyTooLong || code == patterns.ErrCodeTextExceedsShape || code == patterns.ErrCodeChartPlaceholderEmpty {
			severity = diagnostics.SeverityError
		}
		ds = append(ds, diagnostics.Diagnostic{
			Code: code, Message: message, Path: path, Severity: severity,
			Details: map[string]any{"pattern": name},
		})
	}
	for _, warning := range result.DensityWarnings {
		var fix *diagnostics.Fix
		if warning.Fix != nil {
			fix = &diagnostics.Fix{Kind: warning.Fix.Kind, Params: warning.Fix.Params}
		}
		ds = append(ds, diagnostics.Diagnostic{
			Code: warning.Code, Message: warning.Message, Path: "/values",
			Severity: diagnostics.SeverityError, Fix: fix,
			Details: map[string]any{"pattern": name, "expanded_field": warning.Field},
		})
	}
	for _, warning := range result.CapacityWarnings {
		severity := diagnostics.SeverityInfo
		code := patterns.ErrCodeCellUnderfilled
		switch warning.Status {
		case "overflow":
			code = patterns.ErrCodeFitOverflow
			severity = diagnostics.SeverityError
		case "sparse_layout":
			code = patterns.ErrCodeSparseLayout
		case "density_class_divergence":
			code = patterns.ErrCodePatternUnderfilled
		case "underfilled_ink":
			code = patterns.ErrCodePatternUnderfilled
		}
		var nextToolCall *patterns.ToolCallSuggestion
		if warning.NextToolCall != nil {
			nextToolCall = warning.NextToolCall
		}
		message := fmt.Sprintf("cell %d is %s (%d characters, budget %d)", warning.CellIndex, warning.Status, warning.Actual, warning.Budget)
		details := map[string]any{"pattern": name, "cell_index": warning.CellIndex, "cell_field": warning.Field, "status": warning.Status, "actual_chars": warning.Actual, "budget_chars": warning.Budget}
		if warning.Status == "underfilled_ink" {
			message = fmt.Sprintf("pattern ink fills %d%% of its height (review below %d%%)", warning.Actual, warning.Budget)
			details = map[string]any{"pattern": name, "status": warning.Status, "ink_height_pct": warning.Actual, "threshold_pct": warning.Budget}
		}
		ds = append(ds, diagnostics.Diagnostic{
			Code: code, Message: message,
			Path: "/values", Severity: severity, NextToolCall: nextToolCall,
			Details: details,
		})
	}
	return ds
}
