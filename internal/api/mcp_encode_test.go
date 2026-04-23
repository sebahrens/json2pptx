package api

import (
	"strings"
	"testing"
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

	got, err := MarshalMCPResponse(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n  \"name\": \"test\",\n  \"items\": [\n    \"a\",\n    \"b\"\n  ]\n}"
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

	got, err := MarshalMCPResponse(v)
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
			got, err := MarshalMCPResponse(v)
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
	got, err := MarshalMCPResponse(v)
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

	got, err := MarshalMCPResponse(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(got), "\n") {
		t.Errorf("non-'1' env value should produce indented output: %s", got)
	}
}

