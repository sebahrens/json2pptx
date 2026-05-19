package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseContentHintsArg_Inline(t *testing.T) {
	got, err := parseContentHintsArg(`{"item_count":3,"has_metrics":true}`)
	if err != nil {
		t.Fatalf("parseContentHintsArg inline: %v", err)
	}
	if v, ok := got["item_count"].(float64); !ok || v != 3 {
		t.Errorf("item_count: got %v, want 3", got["item_count"])
	}
	if v, ok := got["has_metrics"].(bool); !ok || !v {
		t.Errorf("has_metrics: got %v, want true", got["has_metrics"])
	}
}

func TestParseContentHintsArg_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hints.json")
	if err := os.WriteFile(path, []byte(`{"item_count":4}`), 0o600); err != nil {
		t.Fatalf("write hints file: %v", err)
	}
	got, err := parseContentHintsArg("@" + path)
	if err != nil {
		t.Fatalf("parseContentHintsArg @file: %v", err)
	}
	if v, ok := got["item_count"].(float64); !ok || v != 4 {
		t.Errorf("item_count: got %v, want 4", got["item_count"])
	}
}

func TestParseContentHintsArg_InvalidJSON(t *testing.T) {
	if _, err := parseContentHintsArg(`{not-json}`); err == nil {
		t.Errorf("expected error for invalid JSON")
	}
}

func TestParseContentHintsArg_EmptyAtPath(t *testing.T) {
	if _, err := parseContentHintsArg("@"); err == nil {
		t.Errorf("expected error for empty @ path")
	}
}

func TestSplitCSVNonEmpty(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b,", []string{"a", "b"}},
	}
	for _, tc := range cases {
		if got := splitCSVNonEmpty(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitCSVNonEmpty(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestRecommendPatternCLI_Parity exercises the new ranking-context flags
// end-to-end by invoking the binary, and verifies the CLI response shape
// matches the recommend_pattern MCP tool's JSON output.
func TestRecommendPatternCLI_Parity(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "recommend-pattern", //nolint:gosec
		"--templates-dir", "../../templates",
		"--intent", "compare two options",
		"--content-hints", `{"item_count":2}`,
		"--candidates", "comparison-2col,matrix-2x2",
		"--recent-patterns", "kpi-3up",
		"--prefer-variety",
		"--slide-index", "2",
	)
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("recommend-pattern with parity flags failed: %v\n%s", err, out)
	}

	var resp struct {
		Candidates []struct {
			PatternName string `json:"pattern_name"`
		} `json:"candidates"`
		QueryUnderstoodAs string `json:"query_understood_as"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v\n%s", err, out)
	}

	// candidates mode: every shortlisted name should appear in the response.
	want := map[string]bool{"comparison-2col": false, "matrix-2x2": false}
	for _, c := range resp.Candidates {
		if _, ok := want[c.PatternName]; ok {
			want[c.PatternName] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("expected candidate %q in response, got %+v", name, resp.Candidates)
		}
	}
}

// TestRecommendPatternCLI_HelpDocumentsParity ensures the subcommand's -h
// output advertises the new parity flags so agents discover them.
func TestRecommendPatternCLI_HelpDocumentsParity(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "recommend-pattern", "-h") //nolint:gosec
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	out, _ := cmd.CombinedOutput()
	outStr := string(out)

	mustContain := []string{
		"--content-hints",
		"--recent-patterns",
		"--prefer-variety",
		"--slide-index",
		"--candidates",
		"parity",
	}
	for _, want := range mustContain {
		if !strings.Contains(outStr, want) {
			t.Errorf("recommend-pattern -h missing %q\n--- output ---\n%s", want, outStr)
		}
	}
}
