package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// go-slide-creator-swak. isError is the only protocol-level failure signal, and
// it was absent on every DOMAIN failure of the semantic tools (template not
// found, an unparseable spec) while argument-level failures on the SAME tools
// set it. An agent branching on isError read "a deck that was never written" as
// done and went on to render thumbnails of a path that did not exist.

func specJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func callTool(t *testing.T, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := h(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	return res
}

func payloadFlag(t *testing.T, res *mcp.CallToolResult, key string) (bool, bool) {
	t.Helper()
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatalf("structured content is not an object: %v", err)
	}
	raw, present := top[key]
	if !present {
		return false, false
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, false
	}
	return v, true
}

// TestRenderDeckSpecFailureIsError covers the domain failures the reviewers saw
// on the wire.
func TestRenderDeckSpecFailureIsError(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := semanticTestConfig(t)

	tests := []struct {
		name string
		spec any
	}{
		{
			"template not found",
			map[string]any{
				"meta":   map[string]any{"title": "T", "template": "midnight-blu"},
				"slides": []any{map[string]any{"kind": "title", "title": "x"}},
			},
		},
		{
			"unparseable spec (slides as an object)",
			map[string]any{
				"meta":   map[string]any{"title": "T", "template": "midnight-blue"},
				"slides": map[string]any{"kind": "title"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := callTool(t, mc.handleRenderDeckSpec, map[string]any{"spec": specJSON(t, tt.spec)})
			if !res.IsError {
				t.Error("domain failure returned isError absent — a harness branching on it treats an unwritten deck as done")
			}
			if ok, present := payloadFlag(t, res, "ok"); present && ok {
				t.Error("payload reports ok=true for a failed render")
			}
			// The rich payload is the point of not using a bare error result.
			if res.StructuredContent == nil {
				t.Error("failure lost its structured diagnostics")
			}
		})
	}
}

// TestRenderDeckSpecSuccessIsNotError is the other half: a clean render must
// not be flagged.
func TestRenderDeckSpecSuccessIsNotError(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	mc := semanticTestConfig(t)
	res := callTool(t, mc.handleRenderDeckSpec, map[string]any{"spec": validSemanticSpec})
	if res.IsError {
		t.Error("a successful render was marked isError")
	}
}

// TestValidateDeckSpecVerdictIsNotError pins the deliberate exemption: a tool
// asked to ASSESS reports an invalid deck as a successful call with ok=false.
// Its verdict IS the product, and the repo's contract tests say the same for
// validate_pattern ("validation failures should not be IsError").
func TestValidateDeckSpecVerdictIsNotError(t *testing.T) {
	res := callTool(t, testValidateDeckSpec, map[string]any{"spec": invalidSemanticSpec})
	if res.IsError {
		t.Error("validate_deck_spec marked a verdict as a tool error; assessing tools report ok=false instead")
	}
	if ok, present := payloadFlag(t, res, "ok"); !present || ok {
		t.Error("an invalid spec should still report ok=false in the payload")
	}
}

// TestSemanticToolsIsErrorParity is the parity check the bead asked for: across
// the producing semantic tools, isError and the payload's own ok/success flag
// must never disagree.
func TestSemanticToolsIsErrorParity(t *testing.T) {
	if testing.Short() {
		t.Skip("renders decks")
	}
	mc := semanticTestConfig(t)

	badTemplate := specJSON(t, map[string]any{
		"meta":   map[string]any{"title": "T", "template": "midnight-blu"},
		"slides": []any{map[string]any{"kind": "title", "title": "x"}},
	})
	cases := []struct {
		tool    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args    map[string]any
	}{
		{"render_deck_spec/ok", mc.handleRenderDeckSpec, map[string]any{"spec": validSemanticSpec}},
		{"render_deck_spec/bad-template", mc.handleRenderDeckSpec, map[string]any{"spec": badTemplate}},
		{"compile_deck_spec/ok", handleCompileDeckSpec, map[string]any{"spec": validSemanticSpec}},
		{"compile_deck_spec/invalid", handleCompileDeckSpec, map[string]any{"spec": invalidSemanticSpec}},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			res := callTool(t, c.handler, c.args)
			for _, key := range []string{"ok", "success"} {
				v, present := payloadFlag(t, res, key)
				if !present {
					continue
				}
				if !v && !res.IsError {
					t.Errorf("payload %s=false but isError is absent", key)
				}
				if v && res.IsError {
					t.Errorf("payload %s=true but isError is set", key)
				}
			}
		})
	}
}

// TestBlockingDiagnosticReasons covers the publishability verdict
// (go-slide-creator-swak, closure-audit-1 half): a deck that was WRITTEN can
// still be unfit to ship, and ok:true said nothing about that.
func TestBlockingDiagnosticReasons(t *testing.T) {
	diags := []semanticDiagnostic{
		{Code: "HEADLINE_TOO_LONG", Severity: "info", Action: "review"},
		{Code: "SEMANTIC_DENSITY", Severity: "warning"},
		{Code: "text_overflow", Severity: "error", Action: "refuse", SemanticPath: "slides[1].insights"},
	}
	got := blockingDiagnosticReasons(diags)
	if len(got) != 1 || got[0] != "text_overflow at slides[1].insights" {
		t.Errorf("blocking reasons = %v, want the one refuse finding with its path", got)
	}

	// Repeats collapse to a count rather than repeating the code.
	repeated := []semanticDiagnostic{
		{Code: "text_overflow", Severity: "error", SemanticPath: "slides[1]"},
		{Code: "text_overflow", Severity: "error", SemanticPath: "slides[2]"},
	}
	if got := blockingDiagnosticReasons(repeated); len(got) != 1 || got[0] != "text_overflow (2 slides)" {
		t.Errorf("repeated reasons = %v, want one collapsed entry", got)
	}

	// Nothing blocking, nothing reported.
	if got := blockingDiagnosticReasons(diags[:2]); got != nil {
		t.Errorf("advisory-only diagnostics produced blocking reasons: %v", got)
	}
}

func TestPublishabilityOf(t *testing.T) {
	refuse := []semanticDiagnostic{{Code: "text_overflow", Severity: "error", Action: "refuse"}}

	if ok, reasons := publishabilityOf(nil, nil); !ok || reasons != nil {
		t.Errorf("a clean render should be publishable, got %v %v", ok, reasons)
	}
	if ok, reasons := publishabilityOf(refuse, nil); ok || len(reasons) != 1 {
		t.Errorf("a refuse diagnostic must block: %v %v", ok, reasons)
	}

	// A failed quality gate blocks on its own, with its reasons carried through.
	q := &QualityScore{}
	q.QualityGate = failedGate("1 P1 finding(s) exceeds max_p1_findings 0")
	ok, reasons := publishabilityOf(nil, q)
	if ok {
		t.Error("a deck failing the deterministic quality gate is not publishable")
	}
	if len(reasons) != 1 || reasons[0] != "quality gate: 1 P1 finding(s) exceeds max_p1_findings 0" {
		t.Errorf("gate reasons = %v", reasons)
	}
}

// failedGate builds a failing deterministic quality gate with the given reason.
func failedGate(reason string) *deterministic.QualityGate {
	return &deterministic.QualityGate{Passed: false, Reasons: []string{reason}}
}
