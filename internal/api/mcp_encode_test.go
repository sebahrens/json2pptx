package api

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestMarshalMCPResponse_Default(t *testing.T) {
	// Ensure env is unset for default behavior.
	t.Setenv("MCP_COMPACT_RESPONSES", "")

	v := struct {
		Name  string   `json:"name"`
		Items []string `json:"items"`
	}{
		Name:  "test",
		Items: []string{"a", "b"},
	}

	got, err := MarshalMCPResponse(context.Background(), v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Compact is now the DEFAULT: the text block is a machine-read fallback,
	// and one pass over the core tools carried 53,134 B of pure indentation
	// (go-slide-creator-vxre). This assertion used to expect the indented form.
	want := `{"name":"test","items":["a","b"]}`
	if string(got) != want {
		t.Errorf("default mode mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestMarshalMCPResponse_Compact(t *testing.T) {
	t.Setenv("MCP_COMPACT_RESPONSES", "1")

	v := struct {
		Name  string   `json:"name"`
		Items []string `json:"items"`
	}{
		Name:  "test",
		Items: []string{"a", "b"},
	}

	got, err := MarshalMCPResponse(context.Background(), v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{"name":"test","items":["a","b"]}`
	if string(got) != want {
		t.Errorf("compact mode mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestMarshalMCPResponse_EmptySlice(t *testing.T) {
	// Empty slices must serialize as [] not null, in both modes.
	type response struct {
		Items []string `json:"items"`
	}

	for _, compact := range []string{"", "1"} {
		label := "default"
		if compact == "1" {
			label = "compact"
		}
		t.Run(label, func(t *testing.T) {
			t.Setenv("MCP_COMPACT_RESPONSES", compact)

			v := response{Items: []string{}}
			got, err := MarshalMCPResponse(context.Background(), v)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Both modes should include "items":[]
			if !strings.Contains(string(got), `"items":[]`) && !strings.Contains(string(got), `"items": []`) {
				t.Errorf("%s mode: empty slice not serialized as []: %s", label, got)
			}
		})
	}
}

func TestMarshalMCPResponse_OmitemptyScalar(t *testing.T) {
	// Verify that omitempty on scalars works in compact mode.
	type response struct {
		Name  string `json:"name,omitempty"`
		Count int    `json:"count,omitempty"`
	}

	t.Setenv("MCP_COMPACT_RESPONSES", "1")

	v := response{} // zero values
	got, err := MarshalMCPResponse(context.Background(), v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{}`
	if string(got) != want {
		t.Errorf("omitempty scalars not omitted:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestMarshalMCPResponse_EnvNotOne(t *testing.T) {
	// Values other than "1" should use indented mode.
	t.Setenv("MCP_COMPACT_RESPONSES", "true")

	v := struct {
		OK bool `json:"ok"`
	}{OK: true}

	got, err := MarshalMCPResponse(context.Background(), v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The env var no longer selects indentation — compaction is unconditional
	// (go-slide-creator-vxre), so any value of it produces compact output.
	if strings.Contains(string(got), "\n") {
		t.Errorf("output should be compact regardless of the env value: %s", got)
	}
}

func TestMarshalMCPResponse_CapabilityNegotiated(t *testing.T) {
	// Client that negotiated compact_responses gets compact output,
	// even without the env var.
	t.Setenv("MCP_COMPACT_RESPONSES", "")

	// Build a context with a session that has compact_responses capability.
	s := server.NewMCPServer("test", "1.0.0")
	session := server.NewInProcessSession("test-session", nil)
	session.Initialize()
	session.SetClientCapabilities(mcp.ClientCapabilities{
		Experimental: map[string]any{
			"compact_responses": true,
		},
	})
	ctx := s.WithContext(context.Background(), session)

	v := struct {
		Name string `json:"name"`
	}{Name: "hello"}

	got, err := MarshalMCPResponse(ctx, v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{"name":"hello"}`
	if string(got) != want {
		t.Errorf("capability-negotiated compact mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestMarshalMCPResponse_CapabilityNotSet(t *testing.T) {
	// Client that did NOT negotiate compact_responses gets indented output.
	t.Setenv("MCP_COMPACT_RESPONSES", "")

	s := server.NewMCPServer("test", "1.0.0")
	session := server.NewInProcessSession("test-session", nil)
	session.Initialize()
	session.SetClientCapabilities(mcp.ClientCapabilities{})
	ctx := s.WithContext(context.Background(), session)

	v := struct {
		OK bool `json:"ok"`
	}{OK: true}

	got, err := MarshalMCPResponse(ctx, v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Compaction no longer depends on the capability (go-slide-creator-vxre):
	// every client gets compact JSON.
	if strings.Contains(string(got), "\n") {
		t.Errorf("output should be compact even without the capability: %s", got)
	}
}
