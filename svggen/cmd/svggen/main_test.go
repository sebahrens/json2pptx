package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Import root package to auto-register all diagram types.
	_ "github.com/sebahrens/json2pptx/svggen"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: svggen") {
		t.Fatal("help output missing usage line")
	}
}

func TestRunNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, nil, &stdout, &stderr)
	if code != exitRender {
		t.Fatalf("expected exit %d, got %d", exitRender, code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, nil, &stdout, &stderr)
	if code != exitRender {
		t.Fatalf("expected exit %d, got %d", exitRender, code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("expected unknown command error, got: %s", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "svggen") {
		t.Fatal("version output missing 'svggen'")
	}
}

func TestCmdTypes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"types"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "bar_chart") {
		t.Fatal("types output missing bar_chart")
	}
	// Should be sorted
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] < lines[i-1] {
			t.Fatalf("types not sorted: %q before %q", lines[i-1], lines[i])
		}
	}
}

func TestCmdTypesJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"types", "--json"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	var types []string
	if err := json.Unmarshal(stdout.Bytes(), &types); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(types) == 0 {
		t.Fatal("expected at least one type")
	}
}

func TestCmdRenderInlineData(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"render",
		"--type", "bar_chart",
		"--data", `{"series":[{"name":"X","values":[1,2]}],"categories":["A","B"]}`,
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "<svg") {
		t.Fatal("expected SVG output")
	}
}

func TestCmdRenderStdin(t *testing.T) {
	input := `{"type":"bar_chart","data":{"series":[{"name":"X","values":[1,2]}],"categories":["A","B"]}}`
	var stdout, stderr bytes.Buffer
	code := run([]string{"render"}, strings.NewReader(input), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "<svg") {
		t.Fatal("expected SVG output")
	}
}

func TestCmdRenderToFile(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "out.svg")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"render",
		"--type", "bar_chart",
		"--data", `{"series":[{"name":"X","values":[1,2]}],"categories":["A","B"]}`,
		"-o", outFile,
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "<svg") {
		t.Fatal("expected SVG in output file")
	}
}

func TestCmdRenderInputFile(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "input.json")
	outFile := filepath.Join(dir, "out.svg")
	os.WriteFile(inFile, []byte(`{"type":"bar_chart","data":{"series":[{"name":"X","values":[1,2]}],"categories":["A","B"]}}`), 0644)

	var stdout, stderr bytes.Buffer
	code := run([]string{"render", "-i", inFile, "-o", outFile}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	data, _ := os.ReadFile(outFile)
	if !strings.HasPrefix(string(data), "<svg") {
		t.Fatal("expected SVG")
	}
}

func TestCmdRenderMissingType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"render",
		"--data", `{"values":[1]}`,
	}, nil, &stdout, &stderr)
	if code != exitValidation {
		t.Fatalf("expected exit %d, got %d; stderr: %s", exitValidation, code, stderr.String())
	}
}

func TestCmdRenderNoInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"render"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitIO {
		t.Fatalf("expected exit %d, got %d; stderr: %s", exitIO, code, stderr.String())
	}
}

func TestCmdValidateValid(t *testing.T) {
	input := `{"type":"bar_chart","data":{"series":[{"name":"X","values":[1]}],"categories":["A"]}}`
	var stderr bytes.Buffer
	code := run([]string{"validate"}, strings.NewReader(input), nil, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "valid") {
		t.Fatal("expected 'valid' in output")
	}
}

func TestCmdValidateInvalid(t *testing.T) {
	input := `{"type":"bar_chart","data":{}}`
	var stderr bytes.Buffer
	code := run([]string{"validate"}, strings.NewReader(input), nil, &stderr)
	if code != exitValidation {
		t.Fatalf("expected exit %d, got %d; stderr: %s", exitValidation, code, stderr.String())
	}
}

func TestCmdValidateUnknownType(t *testing.T) {
	input := `{"type":"nonexistent","data":{}}`
	var stderr bytes.Buffer
	code := run([]string{"validate"}, strings.NewReader(input), nil, &stderr)
	if code != exitValidation {
		t.Fatalf("expected exit %d, got %d; stderr: %s", exitValidation, code, stderr.String())
	}
}

func TestCmdBatchDir(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	inDir := filepath.Join(dir, "in")
	os.MkdirAll(inDir, 0755)

	os.WriteFile(filepath.Join(inDir, "chart.json"),
		[]byte(`{"type":"bar_chart","data":{"series":[{"name":"X","values":[1]}],"categories":["A"]}}`), 0644)

	var stderr bytes.Buffer
	code := run([]string{"batch", "-i", inDir, "-o", outDir}, nil, nil, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}

	files, _ := os.ReadDir(outDir)
	if len(files) != 1 {
		t.Fatalf("expected 1 output file, got %d", len(files))
	}
}

func TestCmdBatchJSONL(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	inFile := filepath.Join(dir, "batch.jsonl")

	lines := []string{
		`{"type":"bar_chart","data":{"series":[{"name":"X","values":[1]}],"categories":["A"]}}`,
		`{"type":"bar_chart","data":{"series":[{"name":"Y","values":[2]}],"categories":["B"]}}`,
	}
	os.WriteFile(inFile, []byte(strings.Join(lines, "\n")), 0644)

	var stderr bytes.Buffer
	code := run([]string{"batch", "-i", inFile, "-o", outDir}, nil, nil, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}

	files, _ := os.ReadDir(outDir)
	if len(files) != 2 {
		t.Fatalf("expected 2 output files, got %d", len(files))
	}
}

func TestCmdRenderFormatFromExtension(t *testing.T) {
	// Test that format is inferred from output extension
	outFile := filepath.Join(t.TempDir(), "out.svg")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"render",
		"--type", "bar_chart",
		"--data", `{"series":[{"name":"X","values":[1]}],"categories":["A"]}`,
		"-o", outFile,
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}
}

func TestCmdRenderHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"render", "--help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "--format") {
		t.Fatal("render help missing --format")
	}
}

func TestCmdValidateHelp(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"validate", "--help"}, nil, nil, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestCmdBatchHelp(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"batch", "--help"}, nil, nil, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestCmdBatchMissingFlags(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"batch"}, nil, nil, &stderr)
	if code != exitRender {
		t.Fatalf("expected exit %d, got %d", exitRender, code)
	}
}
