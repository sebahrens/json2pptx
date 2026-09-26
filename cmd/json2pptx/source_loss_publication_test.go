package main

import (
	"context"
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

func TestHandleGenerateRuntimeSourceLossRetainsStructuredRepair(t *testing.T) {
	bullets := make([]any, 10)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("P%02d %s", i, strings.Repeat("Required ownership and reporting evidence. ", 24))
	}
	for _, kind := range []string{"table", "bullets"} {
		for _, mode := range []string{"", "warn", "off"} {
			t.Run(fmt.Sprintf("%s/mode=%q", kind, mode), func(t *testing.T) {
				dir := t.TempDir()
				output := filepath.Join(dir, "deck.pptx")
				if err := os.WriteFile(output, []byte("existing-deck"), 0600); err != nil {
					t.Fatal(err)
				}
				mc := &mcpConfig{templatesDir: testutil.TemplatesDir(), outputDir: dir, cache: template.NewMemoryCache(24 * time.Hour)}
				rows := make([]any, 80)
				for i := range rows {
					rows[i] = []any{fmt.Sprintf("REQUIRED-R%02d", i), "Owner", "Status"}
				}
				item := map[string]any{"placeholder_id": "body", "type": "table", "table_value": map[string]any{"headers": []any{"Evidence", "Owner", "Status"}, "rows": rows}}
				code := patterns.ErrCodeTableRowsTruncated
				if kind == "bullets" {
					item = map[string]any{"placeholder_id": "body", "type": "bullets", "value": bullets}
					code = patterns.ErrCodeTextTrimmed
				}
				params := map[string]any{"output_filename": "deck.pptx", "presentation": map[string]any{"template": "abstract", "slides": []any{map[string]any{"layout_id": "slideLayout3", "content": []any{item}}}}}
				if mode != "" {
					params["strict_fit"] = mode
				}
				result, err := mc.handleGenerate(context.Background(), makeRequest(params))
				if err != nil {
					t.Fatal(err)
				}
				requireStructuredError(t, result, code)
				envelope := structuredErrorEnvelope(t, result)
				if len(envelope.Diagnostics) != 1 || envelope.Diagnostics[0].Path != "/slides/0/content/0" || envelope.Diagnostics[0].Fix == nil {
					t.Fatalf("source-loss repair/path missing: %+v", envelope.Diagnostics)
				}
				if kind == "table" {
					if envelope.Diagnostics[0].Fix.Kind != string(diagnostics.ActionSplitSlide) {
						t.Fatalf("table refusal missing split action: %+v", envelope.Diagnostics[0].Fix)
					}
					if params := envelope.Diagnostics[0].Fix.Params; params["split_at_row"] != float64(7) || params["visible_rows"] != float64(7) || params["hidden_rows"] != float64(73) {
						t.Fatalf("source-loss repair parameters missing: %+v", params)
					}
				}
				data, err := os.ReadFile(output)
				if err != nil || string(data) != "existing-deck" {
					t.Fatalf("refusal replaced destination: %q %v", data, err)
				}
				files, err := os.ReadDir(dir)
				if err != nil || len(files) != 1 {
					t.Fatalf("refusal leaked artifacts: %v %v", files, err)
				}
			})
		}
	}
}
