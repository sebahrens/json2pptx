package svggen

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// chartPaletteTemplatesGlob is the location of bundled templates relative to
// the svggen module.
const chartPaletteTemplatesGlob = "../templates/*.pptx"

// chartPaletteFixture covers one chart type in the per-template palette test.
type chartPaletteFixture struct {
	// chartType is the diagram registry type ID (e.g., "bar_chart").
	chartType string

	// seriesN is the number of series (or slices, for pie) to render. For
	// gauge_chart this is the number of primary fills we expect (1).
	seriesN int

	// buildData returns the chart request payload with the right shape for
	// the chart type and the requested series count.
	buildData func(n int) map[string]any
}

// chartPaletteFixtures lists every chart type covered by the test. The test
// renders one chart per type per template and asserts the rendered series
// colors match the template's resolved DataPalette ordering.
var chartPaletteFixtures = []chartPaletteFixture{
	{chartType: "bar_chart", seriesN: 6, buildData: buildChartPaletteCategoricalData},
	{chartType: "stacked_bar_chart", seriesN: 6, buildData: buildChartPaletteCategoricalData},
	{chartType: "line_chart", seriesN: 6, buildData: buildChartPaletteCategoricalData},
	{chartType: "pie_chart", seriesN: 6, buildData: buildChartPalettePieData},
	{chartType: "gauge_chart", seriesN: 1, buildData: buildChartPaletteGaugeData},
}

// TestChartPalette_PerTemplate asserts that every bundled template's resolved
// DataPalette ordering shows up as chart series colors in rendered SVG output.
//
// For each templates/*.pptx whose metadata declares a data_palette:
//  1. Parse theme1.xml to read accent1..6 hex values.
//  2. Parse ppt/go-slide-creator-metadata.json to read the data_palette
//     scheme names (e.g. ["accent1", "accent3", ...]).
//  3. Resolve the scheme names to hex via the theme.
//  4. For each chart type, render via the diagram registry with a StyleSpec
//     carrying ThemeColors and the resolved DataPalette.
//  5. Extract series colors from the SVG (fill="..." attribute for bar /
//     stacked_bar / pie / gauge, style="stroke:..." for line) and assert
//     that — filtered to the expected palette set — the in-document-order
//     sequence equals DataPalette[0..N-1].
//
// Templates without metadata are skipped, so unstamped designer templates
// (abstract.pptx, modern.pptx, ...) don't break the corpus before they have
// been outfitted with data_palette.
//
// Failure messages name the scheme color so a regression is immediately
// traceable: "template <name> chart <type> series 2: got #XXXXXX expected
// #YYYYYY (accent3)".
func TestChartPalette_PerTemplate(t *testing.T) {
	files, err := filepath.Glob(chartPaletteTemplatesGlob)
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", chartPaletteTemplatesGlob)
	}
	sort.Strings(files)

	for _, f := range files {
		f := f
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			themeColors, palette, paletteNames, err := loadChartPaletteFromTemplate(f)
			if err != nil {
				t.Fatalf("load template %s: %v", name, err)
			}
			if len(palette) == 0 {
				t.Skipf("%s: no data_palette declared in metadata", name)
			}

			for _, fx := range chartPaletteFixtures {
				fx := fx
				t.Run(fx.chartType, func(t *testing.T) {
					n := fx.seriesN
					if n > len(palette) {
						n = len(palette)
					}

					req := &RequestEnvelope{
						Type:  fx.chartType,
						Title: "palette test",
						Data:  fx.buildData(n),
						Output: OutputSpec{
							Width:  800,
							Height: 600,
						},
						Style: StyleSpec{
							ThemeColors: themeColors,
							DataPalette: palette,
						},
					}

					result, err := RenderMultiFormat(req)
					if err != nil {
						t.Fatalf("template %s chart %s: render failed: %v",
							name, fx.chartType, err)
					}
					if result == nil || result.SVG == nil {
						t.Fatalf("template %s chart %s: render returned nil SVG",
							name, fx.chartType)
					}

					svg := string(result.SVG.Content)
					seriesColors := extractChartPaletteSeriesColors(svg, fx.chartType, palette)

					expected := palette[:n]
					expectedNames := paletteNames[:n]

					for i, want := range expected {
						if i >= len(seriesColors) {
							t.Errorf("template %s chart %s series %d: missing fill "+
								"(expected %s [%s])",
								name, fx.chartType, i, want, expectedNames[i])
							continue
						}
						if !strings.EqualFold(seriesColors[i], want) {
							t.Errorf("template %s chart %s series %d: got %s expected %s (%s)",
								name, fx.chartType, i, seriesColors[i], want, expectedNames[i])
						}
					}
				})
			}
		})
	}
}

// =============================================================================
// Template parsing (zip → theme colors + data_palette scheme names)
// =============================================================================

// loadChartPaletteFromTemplate parses a PPTX template and returns:
//
//   - themeInputs:  ThemeColorInput slice (dk1/dk2/lt1/lt2 + accent1..6) for
//     passing to StyleSpec.ThemeColors.
//   - palette:      hex strings resolved from metadata.data_palette via the
//     theme color map, in the order declared by the metadata.
//   - paletteNames: the corresponding scheme color names (e.g. "accent1")
//     in the same order, used to produce diagnostic failure messages.
//
// Returns palette == nil when the template has no metadata file or its
// data_palette is empty — the caller skips in that case.
func loadChartPaletteFromTemplate(pptxPath string) ([]ThemeColorInput, []string, []string, error) {
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = zr.Close() }()

	themeMap, err := readChartPaletteThemeColors(&zr.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	themeInputs := chartPaletteThemeMapToInputs(themeMap)

	paletteNames := readChartPaletteSchemeNames(&zr.Reader)
	if len(paletteNames) == 0 {
		return themeInputs, nil, nil, nil
	}

	resolvedHex := make([]string, 0, len(paletteNames))
	resolvedNames := make([]string, 0, len(paletteNames))
	for _, schemeName := range paletteNames {
		hex, ok := themeMap[schemeName]
		if !ok {
			continue
		}
		resolvedHex = append(resolvedHex, hex)
		resolvedNames = append(resolvedNames, schemeName)
	}

	return themeInputs, resolvedHex, resolvedNames, nil
}

// chartPaletteThemeMapToInputs converts a slot-name → hex map into the ordered
// ThemeColorInput slice consumed by StyleSpec. Only the slots svggen actually
// reads (dk1/dk2/lt1/lt2 + accent1..6) are included.
func chartPaletteThemeMapToInputs(m map[string]string) []ThemeColorInput {
	slots := []string{
		"dk1", "dk2", "lt1", "lt2",
		"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
	}
	out := make([]ThemeColorInput, 0, len(slots))
	for _, name := range slots {
		if hex, ok := m[name]; ok {
			out = append(out, ThemeColorInput{Name: name, RGB: hex})
		}
	}
	return out
}

// readChartPaletteThemeColors loads ppt/theme/theme1.xml and returns a
// slot-name → "#RRGGBB" map (uppercase hex, # prefixed). Errors are returned
// when the theme file is missing or unparseable so the test fails loudly
// rather than silently skipping a misshaped template.
func readChartPaletteThemeColors(zr *zip.Reader) (map[string]string, error) {
	const themePath = "ppt/theme/theme1.xml"
	for _, f := range zr.File {
		if f.Name != themePath {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", themePath, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", themePath, err)
		}
		return parseChartPaletteThemeXML(data)
	}
	return nil, fmt.Errorf("%s not found in template", themePath)
}

// chartPaletteThemeXML mirrors the subset of theme1.xml needed to extract
// named color slots. Each slot wraps either an srgbClr or sysClr child.
type chartPaletteThemeXML struct {
	ThemeElements struct {
		ColorScheme struct {
			Slots []chartPaletteSlotXML `xml:",any"`
		} `xml:"clrScheme"`
	} `xml:"themeElements"`
}

type chartPaletteSlotXML struct {
	XMLName xml.Name
	Srgb    *struct {
		Val string `xml:"val,attr"`
	} `xml:"srgbClr"`
	Sys *struct {
		LastClr string `xml:"lastClr,attr"`
		Val     string `xml:"val,attr"`
	} `xml:"sysClr"`
}

func parseChartPaletteThemeXML(data []byte) (map[string]string, error) {
	var doc chartPaletteThemeXML
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse theme1.xml: %w", err)
	}
	out := make(map[string]string, len(doc.ThemeElements.ColorScheme.Slots))
	for _, s := range doc.ThemeElements.ColorScheme.Slots {
		name := s.XMLName.Local
		if name == "" {
			continue
		}
		var hex string
		switch {
		case s.Srgb != nil && s.Srgb.Val != "":
			hex = s.Srgb.Val
		case s.Sys != nil && s.Sys.LastClr != "":
			hex = s.Sys.LastClr
		}
		if hex == "" {
			continue
		}
		out[name] = "#" + strings.ToUpper(strings.TrimPrefix(hex, "#"))
	}
	return out, nil
}

// readChartPaletteSchemeNames loads ppt/go-slide-creator-metadata.json and
// returns the data_palette scheme name list. Returns nil (without error) when
// the metadata file is absent or the data_palette is empty — that's the
// normal "no metadata" case which the caller treats as a skip.
func readChartPaletteSchemeNames(zr *zip.Reader) []string {
	const metadataPath = "ppt/go-slide-creator-metadata.json"
	for _, f := range zr.File {
		if f.Name != metadataPath {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil
		}
		var meta struct {
			DataPalette []string `json:"data_palette"`
		}
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil
		}
		return meta.DataPalette
	}
	return nil
}

// =============================================================================
// Chart data builders
// =============================================================================

// buildChartPaletteCategoricalData returns chart data with n series, each with
// one value per of 3 categories. Used for bar / stacked_bar / line: every
// series gets a distinct color drawn from the palette, in series order.
func buildChartPaletteCategoricalData(n int) map[string]any {
	categories := []any{"A", "B", "C"}
	series := make([]any, 0, n)
	for i := 0; i < n; i++ {
		series = append(series, map[string]any{
			"name":   fmt.Sprintf("s%d", i),
			"values": []any{float64(10 + i*2), float64(20 + i*2), float64(15 + i*2)},
		})
	}
	return map[string]any{
		"categories": categories,
		"series":     series,
	}
}

// buildChartPalettePieData returns pie data with n slices (one per category).
// The pie chart paints one slice color per category from palette accents in
// order, so this exercises DataPalette[0..n-1] the same way as a multi-series
// bar chart.
func buildChartPalettePieData(n int) map[string]any {
	categories := make([]any, 0, n)
	values := make([]any, 0, n)
	for i := 0; i < n; i++ {
		categories = append(categories, fmt.Sprintf("c%d", i))
		values = append(values, float64(10+i))
	}
	return map[string]any{
		"categories": categories,
		"values":     values,
	}
}

// buildChartPaletteGaugeData returns minimal gauge data. The gauge paints its
// primary value arc with palette accent1 (= DataPalette[0]); the test asserts
// that single fill matches the resolved DataPalette[0].
func buildChartPaletteGaugeData(_ int) map[string]any {
	return map[string]any{
		"value": 73.0,
		"min":   0.0,
		"max":   100.0,
	}
}

// =============================================================================
// SVG color extraction
// =============================================================================

var (
	// chartPaletteAttrFillRegex captures #RRGGBB values inside a fill="..."
	// attribute. svggen emits series fills (bar/pie/gauge) via this form, while
	// text and axis decoration use style="...". Matching only the attribute
	// form keeps the extracted list close to "series colors in draw order".
	chartPaletteAttrFillRegex = regexp.MustCompile(`fill="(#[0-9a-fA-F]{6})"`)

	// chartPaletteStyleStrokeRegex captures #RRGGBB values inside a
	// style="stroke:#...;..." segment. Line charts render their series as path
	// strokes using this form.
	chartPaletteStyleStrokeRegex = regexp.MustCompile(`stroke:(#[0-9a-fA-F]{6})`)
)

// extractChartPaletteSeriesColors returns the in-document-order, deduplicated
// list of series colors rendered into the SVG, filtered to the expected
// palette set so neutral axis / grid / text colors don't contaminate the
// sequence.
//
// chartType controls the extraction strategy: line charts use stroke style
// matching; all other chart types use fill attribute matching.
func extractChartPaletteSeriesColors(svg, chartType string, palette []string) []string {
	var matches []string
	if chartType == "line_chart" {
		matches = chartPaletteExtractMatches(chartPaletteStyleStrokeRegex, svg)
	} else {
		matches = chartPaletteExtractMatches(chartPaletteAttrFillRegex, svg)
	}

	paletteSet := make(map[string]bool, len(palette))
	for _, c := range palette {
		paletteSet[strings.ToUpper(c)] = true
	}

	filtered := make([]string, 0, len(matches))
	for _, c := range matches {
		if !paletteSet[c] {
			continue
		}
		filtered = append(filtered, c)
	}
	return chartPaletteDedupePreservingOrder(filtered)
}

func chartPaletteExtractMatches(re *regexp.Regexp, s string) []string {
	matches := re.FindAllStringSubmatch(s, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, strings.ToUpper(m[1]))
	}
	return out
}

func chartPaletteDedupePreservingOrder(colors []string) []string {
	seen := make(map[string]bool, len(colors))
	out := make([]string, 0, len(colors))
	for _, c := range colors {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}
