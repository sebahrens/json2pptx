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
