package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestCLIMCPConfig(t *testing.T) {
	cfg := cliMCPConfig("./templates", "./out")
	if cfg == nil {
		t.Fatal("cliMCPConfig returned nil")
	}
	if cfg.templatesDir != "./templates" {
		t.Errorf("templatesDir = %q, want %q", cfg.templatesDir, "./templates")
	}
	if cfg.outputDir != "./out" {
		t.Errorf("outputDir = %q, want %q", cfg.outputDir, "./out")
	}
	if cfg.cache == nil {
		t.Error("cfg.cache is nil; expected memory cache")
	}
}

func TestMCPNoopRequest(t *testing.T) {
	req := mcpNoopRequest()
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		t.Fatalf("Arguments is %T, want map[string]any", req.Params.Arguments)
	}
	if len(args) != 0 {
		t.Errorf("noop request has %d args, want 0", len(args))
	}
}

func TestMCPRequestWithArgs(t *testing.T) {
	in := map[string]any{"template": "midnight-blue", "count": 3}
	req := mcpRequestWithArgs(in)
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		t.Fatalf("Arguments is %T, want map[string]any", req.Params.Arguments)
	}
	if got := args["template"]; got != "midnight-blue" {
		t.Errorf("args[template] = %v, want midnight-blue", got)
	}
	if got := args["count"]; got != 3 {
		t.Errorf("args[count] = %v, want 3", got)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written. Used to verify printMCPResultJSON formatting without relying on
// implementation details.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.Bytes()
	}()

	fn()
	_ = w.Close()
	os.Stdout = orig
	return string(<-done)
}

func TestPrintMCPResultJSON_Nil(t *testing.T) {
	if err := printMCPResultJSON(nil); err == nil {
		t.Error("expected error on nil result")
	}
}

func TestPrintMCPResultJSON_Empty(t *testing.T) {
	res := &mcpgo.CallToolResult{Content: nil}
	if err := printMCPResultJSON(res); err == nil {
		t.Error("expected error on empty content")
	}
}

func TestPrintMCPResultJSON_PrettyPrintsJSON(t *testing.T) {
	res := &mcpgo.CallToolResult{
		Content: []mcpgo.Content{
			mcpgo.TextContent{Type: "text", Text: `{"a":1,"b":[2,3]}`},
		},
	}
	var err error
	out := captureStdout(t, func() { err = printMCPResultJSON(res) })
	if err != nil {
		t.Fatalf("printMCPResultJSON: %v", err)
	}
	// Pretty output should be parseable and contain a newline (indented form).
	var got any
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%q", jerr, out)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("expected indented output, got %q", out)
	}
}

func TestPrintMCPResultJSON_PassesThroughNonJSON(t *testing.T) {
	res := &mcpgo.CallToolResult{
		Content: []mcpgo.Content{
			mcpgo.TextContent{Type: "text", Text: "plain hello"},
		},
	}
	var err error
	out := captureStdout(t, func() { err = printMCPResultJSON(res) })
	if err != nil {
		t.Fatalf("printMCPResultJSON: %v", err)
	}
	if !strings.Contains(out, "plain hello") {
		t.Errorf("expected pass-through of plain text, got %q", out)
	}
}

func TestPrintMCPResultJSON_IsErrorReturnsError(t *testing.T) {
	res := &mcpgo.CallToolResult{
		IsError: true,
		Content: []mcpgo.Content{
			mcpgo.TextContent{Type: "text", Text: "tool failure"},
		},
	}
	if err := printMCPResultJSON(res); err == nil {
		t.Error("expected error when IsError=true")
	}
}

func TestReadJSONInput_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.json")
	want := `{"hello":"world"}`
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readJSONInput(path)
	if err != nil {
		t.Fatalf("readJSONInput: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadJSONInput_MissingFile(t *testing.T) {
	if _, err := readJSONInput(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadJSONObject_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.json")
	if err := os.WriteFile(path, []byte(`{"k":42}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	obj, err := readJSONObject(path)
	if err != nil {
		t.Fatalf("readJSONObject: %v", err)
	}
	m, ok := obj.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", obj)
	}
	if v, _ := m["k"].(float64); v != 42 {
		t.Errorf("k = %v, want 42", m["k"])
	}
}

func TestReadJSONObject_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readJSONObject(path); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadJSONObject_MissingFile(t *testing.T) {
	if _, err := readJSONObject(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing file")
	}
}
