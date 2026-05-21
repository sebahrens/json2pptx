package main

import (
	"context"
	"encoding/json"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// TestHandleAuditPalette_PathTraversal verifies the handler rejects a pptx_path
// with a traversal segment before doing any work.
func TestHandleAuditPalette_PathTraversal(t *testing.T) {
	res, err := handleAuditPalette(context.Background(), makeRequest(map[string]any{
		"pptx_path": "../../../etc/passwd.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for path traversal")
	}
}

// TestHandleAuditPalette_WrongExtension verifies a non-.pptx path is rejected.
func TestHandleAuditPalette_WrongExtension(t *testing.T) {
	res, err := handleAuditPalette(context.Background(), makeRequest(map[string]any{
		"pptx_path": "/tmp/secrets.txt",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for wrong extension")
	}
}

// TestHandleAuditPalette_FileNotFound verifies a missing (but otherwise valid)
// path surfaces FILE_NOT_FOUND rather than attempting a render.
func TestHandleAuditPalette_FileNotFound(t *testing.T) {
	res, err := handleAuditPalette(context.Background(), makeRequest(map[string]any{
		"pptx_path": "/nonexistent/path/to/missing.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for missing file")
	}
}

// TestHandleAuditPalette_BadChromaMin verifies an out-of-range chroma_min is
// rejected with a structured INVALID_PARAMETER error before rendering.
func TestHandleAuditPalette_BadChromaMin(t *testing.T) {
	res, err := handleAuditPalette(context.Background(), makeRequest(map[string]any{
		"pptx_path":  "/tmp/deck.pptx",
		"chroma_min": float64(999),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for chroma_min out of range")
	}
}

// TestDiagnosticsFromAuditReport_ViolationsOnly verifies that only failing
// (pic, shape) pairs become RENDER.palette_drift findings and that passing
// pairs are omitted, so the envelope OK flag mirrors the CLI exit code.
func TestDiagnosticsFromAuditReport_ViolationsOnly(t *testing.T) {
	report := &auditReport{
		PPTX:             "/tmp/x.pptx",
		SlideCount:       1,
		MaxDeltaEAllowed: 5.0,
		Slides: []auditSlide{{
			Index: 2, PicCount: 1, ShapeCount: 2, PairCount: 2,
			Pairs: []auditPair{
				{
					Slide:  2,
					Pic:    auditRegion{Kind: "pic", Name: "Chart", Hex: "2e5090"},
					Shape:  auditRegion{Kind: "shape", Name: "Rect", Hex: "c43f3f", DeclaredHex: "accent2"},
					DeltaE: 42.0, Pass: false,
				},
				{
					Slide:  2,
					Pic:    auditRegion{Kind: "pic", Name: "Chart", Hex: "2e5090"},
					Shape:  auditRegion{Kind: "shape", Name: "Banner", Hex: "2e5091"},
					DeltaE: 0.8, Pass: true,
				},
			},
		}},
	}

	ds := diagnosticsFromAuditReport(report)
	if len(ds) != 1 {
		t.Fatalf("expected 1 diagnostic (only the failing pair), got %d", len(ds))
	}
	d := ds[0]
	if d.Code != "RENDER.palette_drift" {
		t.Errorf("code = %q, want RENDER.palette_drift", d.Code)
	}
	// Path uses the 0-based slide index for whereFromPath extraction.
	if d.Path != "slides[1]" {
		t.Errorf("path = %q, want slides[1] (0-based from 1-based slide 2)", d.Path)
	}
	if d.Details["declared_hex"] != "accent2" {
		t.Errorf("declared_hex evidence = %v, want accent2", d.Details["declared_hex"])
	}
}

// TestAuditPaletteTool_OutputSchemaValid guards that the tool's attached output
// schema is well-formed JSON (structured-output MCP clients reject malformed
// schemas) — the schema-coverage test only checks presence, not validity.
func TestAuditPaletteTool_OutputSchemaValid(t *testing.T) {
	tool := mcpAuditPaletteTool()
	if tool.Name != "audit_palette" {
		t.Fatalf("tool name = %q, want audit_palette", tool.Name)
	}
	if len(tool.RawOutputSchema) == 0 {
		t.Fatal("audit_palette tool has no RawOutputSchema")
	}
	var schema map[string]any
	if err := json.Unmarshal(tool.RawOutputSchema, &schema); err != nil {
		t.Fatalf("output schema is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
}

// compile-time assurance the handler matches the MCP handler signature used by
// registerMCPTools.
var _ func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) = handleAuditPalette
