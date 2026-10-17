package main

import (
	"github.com/sebahrens/json2pptx/svggen/safeyaml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaybookRenderContracts(t *testing.T) {
	if testing.Short() {
		t.Skip("renders shipped playbooks across template frames")
	}
	base, err := filepath.Abs("../../examples/semantic/playbooks")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"deals-cdd-proposal", "assurance-itgc-readout"} {
		for _, template := range []string{"midnight-blue", "modern-template", "p-style"} {
			t.Run(name+"/"+template, func(t *testing.T) {
				if template == "p-style" {
					if _, err := os.Stat("../../templates/p-style.pptx"); os.IsNotExist(err) {
						t.Skip("optional local p-style template absent")
					}
				}
				raw, err := os.ReadFile(filepath.Join(base, name+".yaml"))
				if err != nil {
					t.Fatal(err)
				}
				var spec map[string]any
				if err := safeyaml.Unmarshal(raw, &spec); err != nil {
					t.Fatal(err)
				}
				spec["meta"].(map[string]any)["template"] = template
				out := renderDeckSpecCall(t, semanticTestConfig(t), map[string]any{"spec": spec, "base_dir": base})
				if !out.Success || out.PptxPath == "" {
					t.Fatalf("render failed: %s %+v", out.Error, out.Diagnostics)
				}
				if out.Quality == nil || out.Quality.StructuralScore < 80 {
					t.Errorf("quality gate: %+v", out.Quality)
				}
				for _, d := range out.Diagnostics {
					for _, code := range []string{"BASELINE_MISALIGN", "EXHIBIT_TOO_SMALL", "diagram_render_failed"} {
						if strings.Contains(d.Code, code) {
							t.Errorf("regression: %+v", d)
						}
					}
				}
			})
		}
	}
}
