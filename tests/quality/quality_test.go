// Package quality implements a minimal eval harness for json2pptx.
//
// It computes mechanical quality metrics from JSON deck fixtures and
// fit-report NDJSON output, producing CSV results for regression tracking.
package quality

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// hexColorPattern matches #RGB or #RRGGBB hex color strings.
var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// brandAllowlist contains hex values that are NOT flagged as non-brand fills.
var brandAllowlist = map[string]bool{
	"#000000": true, "#000": true,
	"#ffffff": true, "#fff": true,
	"#FFFFFF": true, "#FFF": true,
}

// metrics holds the computed quality metrics for a single JSON fixture.
type metrics struct {
	Name             string
	SlideCount       int
	TDRViolations    int     // slides with rows>7 || cols>6 tables
	HexFillCount     int     // non-brand hex fills
	TotalFillCount   int     // total fills (hex + semantic)
	HexFillRatio     float64 // hex / total
	TinyDividerCount int     // row pairs with gap < 3pt
	SmallFontCount   int     // shape text with font_size < 9
	MixedFillSlides  int     // slides mixing hex + semantic fills
	// Fit-report metrics (from --fit-report NDJSON).
	FitOverflowCount int     // cells with fit_overflow
	DensityExceeded  int     // density_exceeded findings
	UnfittableRate   float64 // unfittable / total findings
	ShrinkRate       float64 // shrink actions / total findings
}

// fitFinding is defined in loop_driver.go (shared within the package).

// presentationInput is a minimal parse of the JSON input for metric extraction.
type presentationInput struct {
	Slides []slideInput `json:"slides"`
}

type slideInput struct {
	Content   []contentInput  `json:"content"`
	ShapeGrid *shapeGridInput `json:"shape_grid,omitempty"`
}

type contentInput struct {
	Type       string          `json:"type"`
	TableValue *tableInput     `json:"table_value,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
}

type tableInput struct {
	Headers []string          `json:"headers"`
	Rows    []json.RawMessage `json:"rows"`
}


type shapeGridInput struct {
	Columns json.RawMessage `json:"columns,omitempty"`
	Gap     float64         `json:"gap,omitempty"`
	RowGap  float64         `json:"row_gap,omitempty"`
	Rows    []gridRowInput  `json:"rows"`
}

type gridRowInput struct {
	Height float64          `json:"height,omitempty"`
	Cells  []*gridCellInput `json:"cells"`
}

type gridCellInput struct {
	Shape *shapeSpecInput `json:"shape,omitempty"`
	Table *tableInput     `json:"table,omitempty"`
}

type shapeSpecInput struct {
	Fill json.RawMessage `json:"fill,omitempty"`
	Text json.RawMessage `json:"text,omitempty"`
}

// TestMechanicalMetrics computes quality metrics for all fixtures and examples.
func TestMechanicalMetrics(t *testing.T) {
	projectRoot := findProjectRoot(t)
	binary := buildBinary(t, projectRoot)

	fixtureDir := filepath.Join(projectRoot, "tests", "quality", "fixtures")
	examplesDir := filepath.Join(projectRoot, "examples")

	var allMetrics []metrics

	// Process quality fixtures.
	fixtures, err := filepath.Glob(filepath.Join(fixtureDir, "*.json"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	for _, f := range fixtures {
		m := computeMetrics(t, f, binary)
		allMetrics = append(allMetrics, m)
	}

	// Process example decks (regression guard).
	examples, err := filepath.Glob(filepath.Join(examplesDir, "*.json"))
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	for _, f := range examples {
		m := computeMetrics(t, f, binary)
		m.Name = "examples/" + filepath.Base(f)
		allMetrics = append(allMetrics, m)
	}

	// Write CSV output.
	csvPath := filepath.Join(projectRoot, "tests", "quality", "results.csv")
	writeCSV(t, csvPath, allMetrics)
	t.Logf("Wrote results to %s", csvPath)

	// Compare against baseline if it exists.
	baselinePath := filepath.Join(projectRoot, "tests", "quality", "baseline.csv")
	if _, err := os.Stat(baselinePath); err == nil {
		compareBaseline(t, baselinePath, allMetrics)
	} else {
		t.Logf("No baseline found at %s — run 'cp results.csv baseline.csv' to create one", baselinePath)
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

func buildBinary(t *testing.T, projectRoot string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "json2pptx")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/json2pptx") //nolint:gosec // test code with controlled args
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build json2pptx: %v\n%s", err, out)
	}
	return binary
}

func computeMetrics(t *testing.T, jsonPath, binary string) metrics {
	t.Helper()
	name := strings.TrimSuffix(filepath.Base(jsonPath), ".json")
	m := metrics{Name: name}

	// Parse the JSON input for mechanical metrics.
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Errorf("read %s: %v", jsonPath, err)
		return m
	}

	var input presentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Errorf("parse %s: %v", jsonPath, err)
		return m
	}

	m.SlideCount = len(input.Slides)

	for si, slide := range input.Slides {
		slideHasHex := false
		slidHasSemantic := false

		// Walk content-level tables.
		for _, content := range slide.Content {
			table := resolveTable(&content)
			if table == nil {
				continue
			}
			numCols := len(table.Headers)
			numRows := len(table.Rows) + 1
			if numRows > 7 || numCols > 6 {
				m.TDRViolations++
			}
		}

		// Walk shape_grid.
		if slide.ShapeGrid != nil {
			grid := slide.ShapeGrid
			gap := effectiveRowGap(grid)

			for ri, row := range grid.Rows {
				for _, cell := range row.Cells {
					if cell == nil {
						continue
					}
					// Check embedded table.
					if cell.Table != nil {
						numCols := len(cell.Table.Headers)
						numRows := len(cell.Table.Rows) + 1
						if numRows > 7 || numCols > 6 {
							m.TDRViolations++
						}
					}
					// Check shape fills and font sizes.
					if cell.Shape != nil {
						color := extractFillColor(cell.Shape.Fill)
						if color != "" {
							m.TotalFillCount++
							if hexColorPattern.MatchString(color) {
								if !brandAllowlist[color] {
									m.HexFillCount++
									slideHasHex = true
								}
							} else {
								slidHasSemantic = true
							}
						}
						fontSize := extractFontSize(cell.Shape.Text)
						if fontSize > 0 && fontSize < 9 {
							m.SmallFontCount++
						}
					}
				}

				// Tiny divider check: gap between consecutive rows.
				if ri > 0 && gap < 3.0 {
					m.TinyDividerCount++
				}
			}
		}

		if slideHasHex && slidHasSemantic {
			m.MixedFillSlides++
		}
		_ = si
	}

	if m.TotalFillCount > 0 {
		m.HexFillRatio = float64(m.HexFillCount) / float64(m.TotalFillCount)
	}

	// Run fit-report via the binary.
	fitReportPath := filepath.Join(t.TempDir(), name+"-fit.ndjson")
	cmd := exec.Command(binary, "validate", "-fit-report", fitReportPath, jsonPath) //nolint:gosec // test code with controlled args
	// Ignore exit code — validate may return non-zero for invalid inputs.
	_ = cmd.Run()

	// Parse fit-report NDJSON.
	if fitData, err := os.ReadFile(fitReportPath); err == nil && len(fitData) > 0 {
		var totalFindings, unfittable, shrink int
		for _, line := range strings.Split(strings.TrimSpace(string(fitData)), "\n") {
			if line == "" {
				continue
			}
			var f fitFinding
			if json.Unmarshal([]byte(line), &f) != nil {
				continue
			}
			totalFindings++
			switch f.Code {
			case "fit_overflow":
				m.FitOverflowCount++
			case "density_exceeded":
				m.DensityExceeded++
			}
			switch f.Action {
			case "unfittable":
				unfittable++
			case "shrink":
				shrink++
			}
		}
		if totalFindings > 0 {
			m.UnfittableRate = float64(unfittable) / float64(totalFindings)
			m.ShrinkRate = float64(shrink) / float64(totalFindings)
		}
	}

	return m
}

func resolveTable(c *contentInput) *tableInput {
	if c.Type != "table" {
		return nil
	}
	if c.TableValue != nil {
		return c.TableValue
	}
	if len(c.Value) > 0 {
		var t tableInput
		if json.Unmarshal(c.Value, &t) == nil && len(t.Headers) > 0 {
			return &t
		}
	}
	return nil
}

func effectiveRowGap(grid *shapeGridInput) float64 {
	if grid.RowGap > 0 {
		return grid.RowGap
	}
	if grid.Gap > 0 {
		return grid.Gap
	}
	return 8.0 // default
}

func extractFillColor(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		Color string `json:"color"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Color
	}
	return ""
}

func extractFontSize(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	// Try object form {"size": N}.
	var obj struct {
		Size float64 `json:"size"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Size > 0 {
		return obj.Size
	}
	return 0
}

func writeCSV(t *testing.T, path string, all []metrics) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create CSV: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"name", "slide_count", "tdr_violations", "hex_fill_count", "total_fill_count",
		"hex_fill_ratio", "tiny_divider_count", "small_font_count", "mixed_fill_slides",
		"fit_overflow_count", "density_exceeded", "unfittable_rate", "shrink_rate",
	}
	if err := w.Write(header); err != nil {
		t.Fatalf("write CSV header: %v", err)
	}

	for _, m := range all {
		row := []string{
			m.Name,
			strconv.Itoa(m.SlideCount),
			strconv.Itoa(m.TDRViolations),
			strconv.Itoa(m.HexFillCount),
			strconv.Itoa(m.TotalFillCount),
			fmt.Sprintf("%.3f", m.HexFillRatio),
			strconv.Itoa(m.TinyDividerCount),
			strconv.Itoa(m.SmallFontCount),
			strconv.Itoa(m.MixedFillSlides),
			strconv.Itoa(m.FitOverflowCount),
			strconv.Itoa(m.DensityExceeded),
			fmt.Sprintf("%.3f", m.UnfittableRate),
			fmt.Sprintf("%.3f", m.ShrinkRate),
		}
		if err := w.Write(row); err != nil {
			t.Fatalf("write CSV row: %v", err)
		}
	}
}

func compareBaseline(t *testing.T, baselinePath string, current []metrics) {
	t.Helper()
	f, err := os.Open(baselinePath)
	if err != nil {
		t.Logf("Could not open baseline: %v", err)
		return
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Logf("Could not parse baseline CSV: %v", err)
		return
	}
	if len(records) < 2 {
		return
	}

	// Build baseline map by name.
	baselineMap := make(map[string][]string)
	for _, row := range records[1:] {
		if len(row) > 0 {
			baselineMap[row[0]] = row
		}
	}

	// Compare current against baseline — flag regressions.
	for _, m := range current {
		baseline, ok := baselineMap[m.Name]
		if !ok {
			t.Logf("NEW fixture (no baseline): %s", m.Name)
			continue
		}

		// Check key metrics for regression.
		checkRegression(t, m.Name, "tdr_violations", baseline, 2, m.TDRViolations)
		checkRegression(t, m.Name, "small_font_count", baseline, 7, m.SmallFontCount)
		checkRegression(t, m.Name, "mixed_fill_slides", baseline, 8, m.MixedFillSlides)
		checkRegression(t, m.Name, "fit_overflow_count", baseline, 9, m.FitOverflowCount)
		checkRegression(t, m.Name, "density_exceeded", baseline, 10, m.DensityExceeded)
	}
}

func checkRegression(t *testing.T, name, metric string, baseline []string, col, currentVal int) {
	t.Helper()
	if col >= len(baseline) {
		return
	}
	baseVal, err := strconv.Atoi(baseline[col])
	if err != nil {
		return
	}
	if currentVal > baseVal {
		t.Logf("REGRESSION %s/%s: baseline=%d current=%d (delta=+%d)",
			name, metric, baseVal, currentVal, currentVal-baseVal)
	}
}
