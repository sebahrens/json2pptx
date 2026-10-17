package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/render"
)

// ReadyHandler checks dependencies for the complete HTTP workflow. It leaves
// /health as cheap liveness and never runs a converter as a readiness probe.
func ReadyHandler(templatesDir, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]bool{"templates": false, "output": false, "render_toolchain": false}
		if entries, err := os.ReadDir(templatesDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".pptx") {
					f, err := os.Open(filepath.Join(templatesDir, entry.Name()))
					if err == nil {
						checks["templates"] = f.Close() == nil
						if checks["templates"] {
							break
						}
					}
				}
			}
		}
		if probe, err := os.CreateTemp(outputDir, ".readiness-*"); err == nil {
			_, writeErr := probe.Write([]byte("ready"))
			closeErr := probe.Close()
			removeErr := os.Remove(probe.Name())
			checks["output"] = writeErr == nil && closeErr == nil && removeErr == nil
		}
		available, missing := render.DependencyStatus()
		checks["render_toolchain"] = available
		ready := checks["templates"] && checks["output"] && available
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]any{"ready": ready, "checks": checks, "missing_render_tools": missing})
	}
}
