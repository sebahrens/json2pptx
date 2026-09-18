package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/template"
)

// successSchemaBranch returns the success-shaped branch of a tool's output
// schema. Since every schema is wrapped as {$defs?, anyOf: [success, error]}
// (go-slide-creator-vtqo), a test inspecting the success shape must reach into
// anyOf[0] rather than the root.
func successSchemaBranch(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("output schema is not valid JSON: %v", err)
	}
	branches, ok := root["anyOf"].([]any)
	if !ok {
		// Unwrapped schema (a schema withErrorEnvelope could not rewrite).
		return root
	}
	if len(branches) == 0 {
		t.Fatal("anyOf is empty")
	}
	success, ok := branches[0].(map[string]any)
	if !ok {
		t.Fatalf("anyOf[0] is %T, want an object schema", branches[0])
	}
	return success
}

// go-slide-creator-vtqo: MCP requires structuredContent to conform to a
// declared outputSchema, and clients are allowed to validate. Every error result
// carries the diagnostics FindingEnvelope, which matched no success schema, so
// all 11 tested error responses were schema-invalid.
func TestWithErrorEnvelope(t *testing.T) {
	t.Run("admits both the success shape and the error envelope", func(t *testing.T) {
		wrapped := withErrorEnvelope(json.RawMessage(`{"type":"object","properties":{"success":{"type":"boolean"}},"required":["success"]}`))

		var root map[string]any
		if err := json.Unmarshal(wrapped, &root); err != nil {
			t.Fatalf("wrapped schema is not valid JSON: %v", err)
		}
		branches, ok := root["anyOf"].([]any)
		if !ok || len(branches) != 2 {
			t.Fatalf("want two anyOf branches, got %v", root["anyOf"])
		}

		success := branches[0].(map[string]any)
		if success["type"] != "object" {
			t.Errorf("success branch lost its type: %v", success)
		}
		if _, ok := success["properties"].(map[string]any)["success"]; !ok {
			t.Errorf("success branch lost its properties: %v", success["properties"])
		}

		errBranch := branches[1].(map[string]any)
		errProps, _ := errBranch["properties"].(map[string]any)
		for _, want := range []string{"ok", "findings", "summary"} {
			if _, ok := errProps[want]; !ok {
				t.Errorf("error branch missing property %q", want)
			}
		}
	})

	t.Run("hoists $defs so #/$defs refs still resolve", func(t *testing.T) {
		// An anyOf branch is not a schema root, so "#/$defs/thing" inside the
		// success branch would dangle unless $defs moves to the wrapper root.
		wrapped := withErrorEnvelope(json.RawMessage(
			`{"type":"object","properties":{"q":{"$ref":"#/$defs/thing"}},"$defs":{"thing":{"type":"string"}}}`))

		var root map[string]any
		if err := json.Unmarshal(wrapped, &root); err != nil {
			t.Fatalf("not valid JSON: %v", err)
		}
		defs, ok := root["$defs"].(map[string]any)
		if !ok {
			t.Fatalf("$defs was not hoisted to the root: %v", root)
		}
		if _, ok := defs["thing"]; !ok {
			t.Errorf("hoisted $defs lost its entry: %v", defs)
		}
		success := root["anyOf"].([]any)[0].(map[string]any)
		if _, stillThere := success["$defs"]; stillThere {
			t.Error("$defs must move to the root, not be duplicated in the branch")
		}
		if ref := success["properties"].(map[string]any)["q"].(map[string]any)["$ref"]; ref != "#/$defs/thing" {
			t.Errorf("the $ref should be untouched, got %v", ref)
		}
	})

	t.Run("a schema it cannot rewrite is returned unchanged", func(t *testing.T) {
		notAnObject := json.RawMessage(`["not", "a", "schema object"]`)
		if got := withErrorEnvelope(notAnObject); string(got) != string(notAnObject) {
			t.Errorf("got %s, want the input unchanged", got)
		}
	})
}

// Every registered tool that declares an output schema must declare a wrapped
// one, so no error response can be schema-invalid.
func TestAllToolOutputSchemasAdmitTheErrorEnvelope(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	srv := server.NewMCPServer("json2pptx-schema-envelope", "test")
	registerMCPTools(srv, mc)

	tools := srv.ListTools()
	if len(tools) == 0 {
		t.Fatal("registerMCPTools registered zero tools")
	}

	checked := 0
	for name, st := range tools {
		if st == nil || len(st.Tool.RawOutputSchema) == 0 {
			continue // a tool may legitimately declare no output schema
		}
		checked++
		var root map[string]any
		if err := json.Unmarshal(st.Tool.RawOutputSchema, &root); err != nil {
			t.Errorf("%s: output schema is not valid JSON: %v", name, err)
			continue
		}
		branches, ok := root["anyOf"].([]any)
		if !ok || len(branches) != 2 {
			t.Errorf("%s: output schema does not admit the error envelope (no two-branch anyOf) — wrap it with withErrorEnvelope", name)
			continue
		}
		errBranch, ok := branches[1].(map[string]any)
		if !ok {
			t.Errorf("%s: anyOf[1] is not an object schema", name)
			continue
		}
		props, _ := errBranch["properties"].(map[string]any)
		if _, ok := props["findings"]; !ok {
			t.Errorf("%s: anyOf[1] is not the error envelope", name)
		}
	}
	if checked == 0 {
		t.Fatal("no tool declared an output schema — the registry walk is probably wrong")
	}
	t.Logf("checked %d tools with output schemas", checked)
}
