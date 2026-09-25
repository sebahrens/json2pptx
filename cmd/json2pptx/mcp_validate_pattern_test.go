package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestValidatePatternCatchesExpandContentBudgets(t *testing.T) {
	cells := make([]any, 12)
	for i := range cells {
		cells[i] = map[string]any{"header": fmt.Sprintf("Card %d", i+1), "body": strings.Repeat("x", 105)}
	}
	args := map[string]any{
		"name":           "card-grid",
		"values":         map[string]any{"columns": 4, "rows": 3, "cells": cells},
		"theme_template": "midnight-blue",
	}
	mc := &mcpConfig{templatesDir: "../../templates"}
	validated, err := mc.handleValidatePattern(context.Background(), makeRequest(args))
	if err != nil || validated.IsError {
		t.Fatalf("validate_pattern call failed: err=%v result=%+v", err, validated)
	}
	env := patternValidationEnvelope(t, validated)
	if env.OK || env.Template != "midnight-blue" || env.Subcommand != "validate_pattern" {
		t.Fatalf("wrong validation verdict/context: %+v", env)
	}
	seenPaths := map[string]bool{}
	for _, finding := range env.Findings {
		if finding.Code != "FIT.BODY_TOO_LONG" {
			continue
		}
		path, _ := finding.Evidence["path"].(string)
		seenPaths[path] = true
		if finding.NextToolCall != nil {
			if _, hasSlide := finding.NextToolCall.ArgsTemplate["slide_index"]; hasSlide {
				t.Errorf("pre-slide finding suggests slide_index: %+v", finding.NextToolCall)
			}
		}
	}
	if len(seenPaths) != 12 {
		t.Fatalf("want 12 distinct BODY_TOO_LONG card paths, got %d: %+v", len(seenPaths), seenPaths)
	}
	for i := range cells {
		path := fmt.Sprintf("/values/cells/%d/body", i)
		if !seenPaths[path] {
			t.Errorf("missing budget finding for %s", path)
		}
	}

	expanded, err := mc.handleExpandPattern(context.Background(), makeRequest(args))
	if err != nil || expanded.IsError {
		t.Fatalf("expand_pattern call failed: err=%v result=%+v", err, expanded)
	}
	result, ok := expanded.StructuredContent.(patternExpansionResult)
	if !ok {
		t.Fatalf("expand_pattern returned %T", expanded.StructuredContent)
	}
	if len(result.Warnings) != 12 {
		t.Errorf("expand_pattern warnings = %d, want 12", len(result.Warnings))
	}
}

func TestValidatePatternSuggestsCellsForDroppedCards(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates"}
	result, err := mc.handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name": "card-grid",
		"values": map[string]any{
			"columns": 1,
			"rows":    1,
			"cards":   []any{map[string]any{"header": "Revenue", "body": "Growing"}},
		},
	}))
	if err != nil || result.IsError {
		t.Fatalf("validate_pattern failed: %v, %+v", err, result)
	}
	env := patternValidationEnvelope(t, result)
	if env.OK {
		t.Fatal("dropped cards must not validate")
	}
	for _, finding := range env.Findings {
		if finding.Code != "GRID.PATTERN_UNKNOWN_FIELD" {
			continue
		}
		if path, _ := finding.Evidence["path"].(string); path != "/values/cards" {
			continue
		}
		if finding.Remediation == nil || finding.Remediation.Primary == nil ||
			finding.Remediation.Primary.Params["kind"] != "rename_field" ||
			finding.Remediation.Primary.Params["to"] != "cells" {
			t.Fatalf("cards finding lacks actionable cells rename: %+v", finding)
		}
		return
	}
	t.Fatalf("missing cards finding: %+v", env.Findings)
}

func TestPatternExpansionDiagnosticsCapacityStatus(t *testing.T) {
	tests := []struct {
		status   string
		code     string
		severity diagnostics.Severity
	}{
		{"overflow", patterns.ErrCodeFitOverflow, diagnostics.SeverityError},
		{"underfilled", patterns.ErrCodeCellUnderfilled, diagnostics.SeverityInfo},
		{"sparse_layout", patterns.ErrCodeSparseLayout, diagnostics.SeverityInfo},
		{"density_class_divergence", patterns.ErrCodePatternUnderfilled, diagnostics.SeverityInfo},
		{"underfilled_ink", patterns.ErrCodePatternUnderfilled, diagnostics.SeverityInfo},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			ds := patternExpansionDiagnostics("card-grid", patternExpansionResult{
				CapacityWarnings: []cellDensityWarning{{CellIndex: 2, Field: "body", Status: tt.status, Actual: 120, Budget: 80}},
			})
			if len(ds) != 1 || ds[0].Code != tt.code || ds[0].Severity != tt.severity || ds[0].Path != "/values" {
				t.Fatalf("diagnostics = %+v", ds)
			}
			if _, ok := diagnostics.Describe("FIT." + tt.code); !ok && tt.code == patterns.ErrCodeFitOverflow {
				t.Fatalf("finding %q cannot be described", tt.code)
			}
		})
	}
}

func TestExpandAndValidatePatternReportMeasuredInkUnderfill(t *testing.T) {
	cells := make([]any, 4)
	for i := range cells {
		cells[i] = map[string]any{"header": fmt.Sprintf("Card %d", i+1), "body": "Brief"}
	}
	args := map[string]any{
		"name":           "card-grid",
		"values":         map[string]any{"columns": 2, "rows": 2, "cells": cells},
		"theme_template": "midnight-blue",
	}
	mc := &mcpConfig{templatesDir: "../../templates"}
	expanded, err := mc.handleExpandPattern(context.Background(), makeRequest(args))
	if err != nil || expanded.IsError {
		t.Fatalf("expand_pattern failed: err=%v result=%+v", err, expanded)
	}
	result, ok := expanded.StructuredContent.(patternExpansionResult)
	if !ok {
		t.Fatalf("expand_pattern returned %T", expanded.StructuredContent)
	}
	if result.Occupancy.FilledPct != 100 || result.Occupancy.InkHeightPct <= 0 || result.Occupancy.InkHeightPct >= 40 {
		t.Fatalf("unexpected occupancy: %+v", result.Occupancy)
	}
	underfill := false
	for _, warning := range result.CapacityWarnings {
		underfill = underfill || warning.Status == "underfilled_ink"
	}
	if !underfill {
		t.Fatalf("expand_pattern omitted measured-ink warning: %+v", result.CapacityWarnings)
	}
	validated, err := mc.handleValidatePattern(context.Background(), makeRequest(args))
	if err != nil || validated.IsError {
		t.Fatalf("validate_pattern failed: err=%v result=%+v", err, validated)
	}
	env := patternValidationEnvelope(t, validated)
	for _, finding := range env.Findings {
		if finding.Code == "GRID."+patterns.ErrCodePatternUnderfilled && strings.Contains(finding.Message, "ink fills") {
			if _, ok := finding.Evidence["ink_height_pct"]; !ok {
				t.Fatalf("ink finding missing percentage evidence: %+v", finding)
			}
			if _, ok := finding.Evidence["actual_chars"]; ok {
				t.Fatalf("ink finding mislabeled percentage as characters: %+v", finding)
			}
			return
		}
	}
	t.Fatalf("validate_pattern omitted measured-ink finding: %+v", env.Findings)
}
