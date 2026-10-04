package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Schema legality must not allow the actual generated 2.4–6pt extreme cells
// through the CLI, even when the advisory preflight gate is disabled.
func TestCLIExtremeSchemaPatternsCannotPublishTinyText(t *testing.T) {
	// Read-only inputs; writes go only to t.TempDir() (go-slide-creator-s2s53).
	t.Parallel()
	for _, patternName := range []string{"bmc-canvas", "stylish-panels", "card-grid", "contact-directory", "framework-grid", "text-sidebar"} {
		pat, ok := patterns.Default().Get(patternName)
		if !ok {
			t.Fatalf("missing pattern %s", patternName)
		}
		values, note := schemaMaximumValues(pat)
		if note != "" {
			t.Fatalf("%s: %s", patternName, note)
		}
		// The generic maxima builder intentionally synthesizes non-fetchable
		// URL/path strings. Omit optional photos so this is a text-maxima test,
		// not an unrelated asset resolver failure; keep all people and text.
		if directory, ok := values.(*patterns.ContactDirectoryValues); ok {
			for gi := range directory.Groups {
				for pi := range directory.Groups[gi].People {
					directory.Groups[gi].People[pi].Photo = nil
				}
			}
		}
		for _, name := range schemaMaximaRunTemplateNames(t) {
			t.Run(patternName+"/"+name, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				input, report := filepath.Join(dir, "input.json"), filepath.Join(dir, "result.json")
				outputDir := filepath.Join(dir, "output")
				if err := os.Mkdir(outputDir, 0700); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(outputDir, "deck.pptx")
				if err := os.WriteFile(output, []byte("existing destination"), 0600); err != nil {
					t.Fatal(err)
				}
				deck := map[string]any{"template": name, "output_filename": "deck.pptx", "slides": []any{map[string]any{"layout_id": "blank-title", "pattern": map[string]any{"name": patternName, "values": values}}}}
				before, err := json.Marshal(deck)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(input, before, 0600); err != nil {
					t.Fatal(err)
				}
				if err := runJSONMode(input, report, testutil.TemplatesDir(), outputDir, "", false, false, "", "off", false, "strict", "", false); err == nil {
					t.Fatal("extreme schema pattern published")
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
				if result.Success || result.OutputPath != "" || finding == nil || finding.Action != "refuse" || finding.Fix != nil || !strings.HasPrefix(finding.Path, "/slides/0/pattern") || strings.Contains(finding.Path, "/rows/") || !strings.Contains(fmt.Sprint(finding.Debug["locator"]), "/rendered_shapes/") || strings.Contains(finding.Message, "shorten") {
					t.Fatalf("incorrect readability refusal: %+v", result)
				}
				if after, err := os.ReadFile(input); err != nil || string(after) != string(before) {
					t.Fatal("authored source changed")
				}
				if data, err := os.ReadFile(output); err != nil || string(data) != "existing destination" {
					t.Fatal("existing destination changed")
				}
				if files, err := os.ReadDir(outputDir); err != nil || len(files) != 1 {
					t.Fatal("refusal leaked artifacts")
				}
			})
		}
	}
}
