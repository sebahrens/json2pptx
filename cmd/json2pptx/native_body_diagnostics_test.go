package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestNativeBodyRuntimeDiagnosticsAcrossCallers(t *testing.T) {
	t.Parallel()
	for _, caller := range []string{"cli", "mcp"} {
		for _, mode := range []string{"", "warn", "off", "strict"} {
			t.Run(caller+"/mode="+mode, func(t *testing.T) {
				bullets := make([]string, 10)
				for i := range bullets {
					bullets[i] = fmt.Sprintf("C1-B%02d Service owner confirms the reporting deadline and documents unresolved handoffs before the monthly close. Café teams review résumé details and regional delivery constraints.", i)
				}
				bullets[1] = "\t" + bullets[1]
				deck := map[string]any{"template": "blue-corporate", "output_filename": "deck.pptx", "slides": []any{map[string]any{"layout_id": "slideLayout2", "content": []any{map[string]any{"placeholder_id": "body", "type": "bullets", "value": bullets}}}}}
				before, err := json.Marshal(deck)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				outputDir := filepath.Join(dir, "output")
				if err := os.Mkdir(outputDir, 0700); err != nil {
					t.Fatal(err)
				}
				destination := filepath.Join(outputDir, "deck.pptx")
				if err := os.WriteFile(destination, []byte("existing-deck"), 0600); err != nil {
					t.Fatal(err)
				}
				if caller == "cli" {
					input, report := filepath.Join(dir, "input.json"), filepath.Join(dir, "result.json")
					if err := os.WriteFile(input, before, 0600); err != nil {
						t.Fatal(err)
					}
					if err := runJSONMode(input, report, testutil.TemplatesDir(), outputDir, "", false, false, "", mode, false, "strict", "", false); err == nil {
						t.Fatal("unreadable native body published")
					}
					data, err := os.ReadFile(report)
					if err != nil {
						t.Fatal(err)
					}
					var result JSONOutput
					if err := json.Unmarshal(data, &result); err != nil {
						t.Fatal(err)
					}
					finding := firstFindingCode(result.FitFindings, patterns.ErrCodeTextBelowReadableMin)
					if result.Success || result.OutputPath != "" || finding == nil || finding.Action != "refuse" || finding.Path != "/slides/0/content/0" || finding.Fix == nil || finding.Fix.Kind != "split_bullets" || finding.Fix.Params["max_items"] != float64(2) || strings.Contains(finding.Message, "shorten") {
						t.Fatalf("CLI lost typed source-preserving readability refusal: %+v", result)
					}
					if after, err := os.ReadFile(input); err != nil || string(after) != string(before) {
						t.Fatal("CLI changed authored input")
					}
				} else {
					mc := &mcpConfig{templatesDir: testutil.TemplatesDir(), outputDir: outputDir, cache: template.NewMemoryCache(24 * time.Hour)}
					params := map[string]any{"output_filename": "deck.pptx", "presentation": deck}
					if mode != "" {
						params["strict_fit"] = mode
					}
					result, err := mc.handleGenerate(context.Background(), makeRequest(params))
					if err != nil {
						t.Fatal(err)
					}
					requireStructuredError(t, result, patterns.ErrCodeTextBelowReadableMin)
					envelope := structuredErrorEnvelope(t, result)
					if len(envelope.Diagnostics) != 1 || envelope.Diagnostics[0].Path != "/slides/0/content/0" || envelope.Diagnostics[0].Fix == nil || envelope.Diagnostics[0].Fix.Kind != string(diagnostics.ActionSplitSlide) || envelope.Diagnostics[0].Fix.Params["kind"] != "split_bullets" || envelope.Diagnostics[0].Fix.Params["max_items"] != float64(2) {
						t.Fatalf("MCP lost readability code/path/repair: %+v", envelope)
					}
				}
				if after, err := json.Marshal(deck); err != nil || string(after) != string(before) {
					t.Fatal("caller changed authored source")
				}
				if data, err := os.ReadFile(destination); err != nil || string(data) != "existing-deck" {
					t.Fatalf("refusal replaced destination: %q %v", data, err)
				}
				if files, err := os.ReadDir(outputDir); err != nil || len(files) != 1 {
					t.Fatalf("refusal leaked output artifacts: %v %v", files, err)
				}
			})
		}
	}
}
