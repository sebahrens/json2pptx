package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildJSON2PPTX builds the binary once for test use.
func buildJSON2PPTX(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "json2pptx")
	cmd := exec.Command("go", "build", "-o", bin, "./") //nolint:gosec
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// runBin executes the test binary with the given args.
func runBin(bin string, args ...string) ([]byte, error) {
	return exec.Command(bin, args...).CombinedOutput() //nolint:gosec
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPatternsList(t *testing.T) {
	bin := buildJSON2PPTX(t)

	t.Run("human", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "list")
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		output := string(out)
		if !strings.Contains(output, "kpi-3up") {
			t.Errorf("expected kpi-3up in output, got: %s", output)
		}
		if !strings.Contains(output, "NAME") {
			t.Errorf("expected header row, got: %s", output)
		}
	})

	t.Run("json", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "list", "--json")
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		var entries []skillPatternCompact
		if err := json.Unmarshal(out, &entries); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if len(entries) == 0 {
			t.Fatal("expected at least one pattern")
		}
		found := false
		for _, e := range entries {
			if e.Name == "kpi-3up" {
				found = true
				if e.Cells != "3" {
					t.Errorf("expected cells=3, got %q", e.Cells)
				}
				if e.UseWhen == "" {
					t.Error("expected non-empty use_when")
				}
			}
		}
		if !found {
			t.Error("kpi-3up not found in JSON output")
		}
	})
}

func TestPatternsShow(t *testing.T) {
	bin := buildJSON2PPTX(t)

	t.Run("human", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "show", "kpi-3up")
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		output := string(out)
		if !strings.Contains(output, "Pattern: kpi-3up") {
			t.Errorf("expected pattern header, got: %s", output)
		}
		if !strings.Contains(output, "Schema:") {
			t.Errorf("expected schema section, got: %s", output)
		}
	})

	t.Run("json", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "show", "--json", "kpi-3up")
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		var result skillPatternFull
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if result.Name != "kpi-3up" {
			t.Errorf("expected name=kpi-3up, got %q", result.Name)
		}
		if result.Version != 1 {
			t.Errorf("expected version=1, got %d", result.Version)
		}
		if len(result.Schema) == 0 {
			t.Error("expected non-empty schema")
		}
	})

	t.Run("unknown", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "show", "nonexistent")
		if err == nil {
			t.Fatal("expected non-zero exit for unknown pattern")
		}
		output := string(out)
		if !strings.Contains(output, "unknown pattern") {
			t.Errorf("expected 'unknown pattern' error, got: %s", output)
		}
		if !strings.Contains(output, "json2pptx patterns list") {
			t.Errorf("expected hint about patterns list, got: %s", output)
		}
	})
}

func TestPatternsValidate(t *testing.T) {
	bin := buildJSON2PPTX(t)
	dir := t.TempDir()

	validValues := `{"values":[{"big":"$4.2M","small":"ARR"},{"big":"127%","small":"NRR"},{"big":"12d","small":"Cycle"}]}`
	validFile := writeTestFile(t, dir, "valid.json", validValues)

	invalidValues := `{"values":[{"big":"$4.2M","small":"ARR"}]}`
	invalidFile := writeTestFile(t, dir, "invalid.json", invalidValues)

	t.Run("valid_human", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "validate", "kpi-3up", validFile)
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "valid") {
			t.Errorf("expected 'valid' in output, got: %s", out)
		}
	})

	t.Run("valid_json", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "validate", "--json", "kpi-3up", validFile)
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		var result struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if !result.OK {
			t.Error("expected ok=true")
		}
	})

	t.Run("invalid_exits_nonzero", func(t *testing.T) {
		_, err := runBin(bin, "patterns", "validate", "kpi-3up", invalidFile)
		if err == nil {
			t.Fatal("expected non-zero exit for invalid values")
		}
	})

	t.Run("invalid_json_d10", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "validate", "--json", "kpi-3up", invalidFile)
		if err == nil {
			t.Fatal("expected non-zero exit for invalid values")
		}
		var result struct {
			OK     bool                     `json:"ok"`
			Errors []patternValidationError `json:"errors"`
		}
		// stdout has the JSON, stderr has the error line — parse combined output
		// by finding the JSON object
		jsonStart := strings.Index(string(out), "{")
		jsonEnd := strings.LastIndex(string(out), "}") + 1
		if jsonStart < 0 || jsonEnd <= jsonStart {
			t.Fatalf("no JSON found in output: %s", out)
		}
		if err := json.Unmarshal(out[jsonStart:jsonEnd], &result); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if result.OK {
			t.Error("expected ok=false")
		}
		if len(result.Errors) == 0 {
			t.Error("expected at least one error")
		}
	})
}

func TestPatternsExpand(t *testing.T) {
	bin := buildJSON2PPTX(t)
	dir := t.TempDir()

	values := `[{"big":"$4.2M","small":"ARR"},{"big":"127%","small":"NRR"},{"big":"12d","small":"Cycle"}]`
	valuesFile := writeTestFile(t, dir, "values.json", values)

	t.Run("expand_output", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "expand", "kpi-3up", valuesFile)
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		// Find JSON in output (may have log lines on stderr mixed in)
		jsonStart := strings.Index(string(out), "{")
		jsonEnd := strings.LastIndex(string(out), "}") + 1
		if jsonStart < 0 || jsonEnd <= jsonStart {
			t.Fatalf("no JSON found in output: %s", out)
		}

		var result struct {
			Pattern   string          `json:"pattern"`
			Version   int             `json:"version"`
			ShapeGrid json.RawMessage `json:"shape_grid"`
		}
		if err := json.Unmarshal(out[jsonStart:jsonEnd], &result); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if result.Pattern != "kpi-3up" {
			t.Errorf("expected pattern=kpi-3up, got %q", result.Pattern)
		}
		if result.Version != 1 {
			t.Errorf("expected version=1, got %d", result.Version)
		}
		if len(result.ShapeGrid) == 0 {
			t.Error("expected non-empty shape_grid")
		}
	})

	t.Run("expand_unknown", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "expand", "nonexistent", valuesFile)
		if err == nil {
			t.Fatal("expected non-zero exit for unknown pattern")
		}
		if !strings.Contains(string(out), "unknown pattern") {
			t.Errorf("expected 'unknown pattern' error, got: %s", out)
		}
	})
}
