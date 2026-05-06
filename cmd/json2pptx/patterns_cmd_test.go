package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
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

	t.Run("multi_error_split_d10", func(t *testing.T) {
		// card-grid with columns=0, rows=0 produces two validation errors;
		// they must appear as separate entries (not collapsed into one).
		multiErrValues := `{"columns":0,"rows":0,"cells":[]}`
		multiErrFile := writeTestFile(t, dir, "multi_err.json", multiErrValues)

		out, err := runBin(bin, "patterns", "validate", "--json", "card-grid", multiErrFile)
		if err == nil {
			t.Fatal("expected non-zero exit for invalid values")
		}
		var result struct {
			OK     bool                     `json:"ok"`
			Errors []patternValidationError `json:"errors"`
		}
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
		if len(result.Errors) < 2 {
			t.Errorf("expected at least 2 separate errors, got %d", len(result.Errors))
		}
		// Verify field extraction — first error should target "columns"
		if len(result.Errors) > 0 && result.Errors[0].Field != "columns" {
			t.Errorf("expected first error field='columns', got %q", result.Errors[0].Field)
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

	t.Run("expand_default_fallback_bounds_source", func(t *testing.T) {
		out, err := runBin(bin, "patterns", "expand", "kpi-3up", valuesFile)
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		var result struct {
			BoundsSource string `json:"bounds_source"`
		}
		jsonStart := strings.Index(string(out), "{")
		jsonEnd := strings.LastIndex(string(out), "}") + 1
		if err := json.Unmarshal(out[jsonStart:jsonEnd], &result); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if result.BoundsSource != "default_fallback" {
			t.Errorf("expected bounds_source=default_fallback, got %q", result.BoundsSource)
		}
	})

	t.Run("expand_template_bounds_source", func(t *testing.T) {
		templatesDir := filepath.Join("..", "..", "templates")
		out, err := runBin(bin, "patterns", "expand", "--templates-dir", templatesDir, "--template", "midnight-blue", "kpi-3up", valuesFile)
		if err != nil {
			t.Fatalf("exit error: %v\n%s", err, out)
		}
		var result struct {
			BoundsSource string `json:"bounds_source"`
		}
		jsonStart := strings.Index(string(out), "{")
		jsonEnd := strings.LastIndex(string(out), "}") + 1
		if err := json.Unmarshal(out[jsonStart:jsonEnd], &result); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		if result.BoundsSource != "template" {
			t.Errorf("expected bounds_source=template, got %q", result.BoundsSource)
		}
	})
}

// TestResolveExpandContext_Unit tests the resolveExpandContext function directly.
func TestResolveExpandContext_Unit(t *testing.T) {
	templatesDir := filepath.Join("..", "..", "templates")

	t.Run("no_template_returns_default_fallback", func(t *testing.T) {
		ctx, source, err := resolveExpandContext("", templatesDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if source != "default_fallback" {
			t.Errorf("expected source=default_fallback, got %q", source)
		}
		if ctx.SlideWidth != 9144000 {
			t.Errorf("expected default slide width 9144000, got %d", ctx.SlideWidth)
		}
		if ctx.LayoutBounds.X != 457200 {
			t.Errorf("expected default X=457200, got %d", ctx.LayoutBounds.X)
		}
	})

	t.Run("with_template_returns_template_source", func(t *testing.T) {
		ctx, source, err := resolveExpandContext("midnight-blue", templatesDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if source != "template" {
			t.Errorf("expected source=template, got %q", source)
		}
		if ctx.SlideWidth <= 0 {
			t.Error("expected positive slide width from template")
		}
		if ctx.LayoutBounds.Width <= 0 {
			t.Error("expected positive layout bounds width from template")
		}
	})

	t.Run("unknown_template_errors", func(t *testing.T) {
		_, _, err := resolveExpandContext("nonexistent-template-xyz", templatesDir)
		if err == nil {
			t.Fatal("expected error for unknown template")
		}
	})
}

// TestExpandCrossTemplate verifies that different templates produce different
// layout bounds. Iterates over all bundled templates and all registered patterns.
func TestExpandCrossTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-template expand test in short mode")
	}

	templatesDir := filepath.Join("..", "..", "templates")
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		t.Fatalf("failed to read templates dir: %v", err)
	}

	var templateNames []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".pptx") {
			templateNames = append(templateNames, strings.TrimSuffix(e.Name(), ".pptx"))
		}
	}
	if len(templateNames) < 2 {
		t.Fatal("need at least 2 templates for cross-template test")
	}

	// Collect bounds per template
	type boundsKey struct {
		X, Y, W, H int64
	}
	boundsByTemplate := make(map[string]boundsKey)

	for _, tmpl := range templateNames {
		ctx, source, err := resolveExpandContext(tmpl, templatesDir)
		if err != nil {
			t.Fatalf("template %q: %v", tmpl, err)
		}
		if source != "template" {
			t.Errorf("template %q: expected source=template, got %q", tmpl, source)
		}
		if ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
			t.Errorf("template %q: layout bounds have non-positive dimensions: %+v", tmpl, ctx.LayoutBounds)
		}
		boundsByTemplate[tmpl] = boundsKey{
			X: ctx.LayoutBounds.X, Y: ctx.LayoutBounds.Y,
			W: ctx.LayoutBounds.Width, H: ctx.LayoutBounds.Height,
		}
		t.Logf("template %q: bounds X=%d Y=%d W=%d H=%d (slide %dx%d)",
			tmpl, ctx.LayoutBounds.X, ctx.LayoutBounds.Y,
			ctx.LayoutBounds.Width, ctx.LayoutBounds.Height,
			ctx.SlideWidth, ctx.SlideHeight)
	}

	// Verify that template-resolved bounds differ from the hardcoded defaults
	// for at least one template (proving template-awareness matters)
	defaultBounds := boundsKey{X: 457200, Y: 457200, W: 8229600, H: 4229100}
	allSameAsDefault := true
	for _, b := range boundsByTemplate {
		if b != defaultBounds {
			allSameAsDefault = false
			break
		}
	}
	if allSameAsDefault {
		t.Error("all templates produced identical bounds to the hardcoded default — template-aware resolution is not working")
	}

	// Verify all patterns expand successfully with each template
	reg := patterns.Default()
	allPatterns := reg.List()

	for _, tmpl := range templateNames {
		ctx, _, err := resolveExpandContext(tmpl, templatesDir)
		if err != nil {
			t.Fatalf("template %q: %v", tmpl, err)
		}

		for _, pat := range allPatterns {
			ex, ok := pat.(patterns.Exemplar)
			if !ok {
				continue // skip patterns without exemplar values
			}
			t.Run(tmpl+"/"+pat.Name(), func(t *testing.T) {
				values := ex.ExemplarValues()
				grid, err := pat.Expand(ctx, values, nil, nil)
				if err != nil {
					t.Errorf("expand failed: %v", err)
				}
				if grid == nil {
					t.Error("expand returned nil grid")
				}
			})
		}
	}
}
