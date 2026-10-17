package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCLIGenerateStrictPlaceholderPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "draft.json")
	deck := `{"template":"midnight-blue","slides":[{"layout_id":"title","content":[{"placeholder_id":"title","type":"text","text_value":"__FILL__"}]}]}`
	if err := os.WriteFile(path, []byte(deck), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"render", "dry-run"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"generate", path, "--placeholder-policy", "strict", "--out", filepath.Join(dir, "draft.pptx")}
			if mode == "dry-run" {
				args = append(args, "--dry-run")
			}
			stdout, stderr, code := cliRun(t, nil, args...)
			if code == 0 || !strings.Contains(stdout, "__FILL__") || !strings.Contains(stdout, "/slides/0/content/0/text_value") {
				t.Fatalf("strict CLI policy did not report the authored unfinished field: code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
		})
	}

	if _, err := os.Stat(filepath.Join(dir, "draft.pptx")); !os.IsNotExist(err) {
		t.Fatalf("strict refusal wrote an artifact: %v", err)
	}
}

func TestCLIGenerateAssetRootAndFindingParity(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	brand, err := os.ReadFile(filepath.Join("..", "..", "templates", "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "brand.pptx"), brand, 0o600); err != nil {
		t.Fatal(err)
	}
	picture := makeSolidPNG(t, 32, 32, 23, 109, 161)
	if err := os.WriteFile(filepath.Join(root, "proof.png"), picture, 0o600); err != nil {
		t.Fatal(err)
	}
	deck := `{"template_path":"brand.pptx","slides":[{"layout_id":"blank","shape_grid":{"columns":1,"rows":[{"cells":[{"image":{"path":"proof.png"}}]}]}}]}`
	path := filepath.Join(dir, "deck.json")
	if err := os.WriteFile(path, []byte(deck), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "cli.pptx")
	stdout, stderr, code := cliRun(t, nil, "generate", path, "--base-dir", root, "--fit-report", "--verbose-fit", "--out", out)
	var cli JSONOutput
	if err := json.Unmarshal([]byte(stdout), &cli); err != nil || code != 0 || !cli.Success || cli.Quality == nil || cli.OutputPath != out {
		t.Fatalf("CLI controls did not produce a full result from the explicit root: code=%d err=%v stdout=%s stderr=%s", code, err, stdout, stderr)
	}
	if media := []byte(readZipText(t, out, "ppt/media/")); !bytes.Contains(media, picture) {
		t.Fatal("generated file does not retain the image resolved from base-dir")
	}
	mc := repairMC(t)
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck), "base_dir": root, "fit_report": true, "verbose_fit": true,
	}))
	if err != nil || result.IsError {
		t.Fatalf("MCP equivalent failed: %v %+v", err, result)
	}
	var mcpOutput JSONOutput
	structuredInto(t, result.StructuredContent, &mcpOutput)
	if !mcpOutput.Success || !reflect.DeepEqual(cli.FitFindings, mcpOutput.FitFindings) {
		t.Fatalf("CLI/MCP finding merge diverged:\nCLI=%+v\nMCP=%+v", cli.FitFindings, mcpOutput.FitFindings)
	}
}
