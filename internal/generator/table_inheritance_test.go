package generator

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestTableAppearanceSelectionKeepsCompactTypography(t *testing.T) {
	for _, defined := range []bool{false, true} {
		path := testutil.TableStyleTemplate(t, defined)
		reader, err := template.OpenTemplate(path)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		for _, style := range []types.TableStyle{
			{}, {StyleID: template.TemplateDefaultSentinel}, {UseTableStyle: true},
			{StyleID: testutil.BrandTableStyleID}, {UseTableStyle: true, StyleID: testutil.BrandTableStyleID},
		} {
			t.Run(strings.Join([]string{style.StyleID, boolString(defined), boolString(style.UseTableStyle)}, "/"), func(t *testing.T) {
				table := eightRowTable()
				table.Style = style
				table.Rows[len(table.Rows)-1][0].Content = "Total"
				bounds := types.BoundingBox{Width: 8229600, Height: 2743200}
				result, err := GenerateTableXML(table, TableRenderConfig{Bounds: bounds, Style: style, StyleResolver: reader})
				if err != nil {
					t.Fatal(err)
				}
				if result.ResolvedStyleID != testutil.BrandTableStyleID {
					t.Errorf("resolved %q", result.ResolvedStyleID)
				}
				if len(trRegexp.FindAllString(result.XML, -1)) != 9 {
					t.Fatal("style selection lost rows")
				}
				if !strings.Contains(result.XML, `sz="1200"`) || !strings.Contains(result.XML, `sz="1100"`) || strings.Contains(result.XML, `sz="1800"`) {
					t.Fatal("style selection inflated typography")
				}
				if result.Height > bounds.Height {
					t.Fatalf("height %d exceeds frame %d", result.Height, bounds.Height)
				}
				for _, f := range result.Findings {
					if f.Action == "refuse" {
						t.Errorf("unexpected refusal: %+v", f)
					}
				}
				for _, f := range DetectTablePreflight(TablePreflightInput{Headers: table.Headers, Rows: table.Rows, Style: style, Bounds: bounds}) {
					if f.Action == "refuse" {
						t.Errorf("preflight rejects fitting brand table: %+v", f)
					}
				}
				if defined {
					for _, override := range []string{`<a:ln`, `<a:solidFill`, `<a:noFill`, `b="0"`, `b="1"`} {
						if strings.Contains(result.XML, override) {
							t.Errorf("style masked by %s", override)
						}
					}
				} else if !strings.Contains(result.XML, `<a:lnB w="12700"`) {
					t.Error("undefined style lost fallback")
				}
				if table.Style.StyleID != style.StyleID || table.Style.UseTableStyle != style.UseTableStyle {
					t.Fatal("renderer mutated caller style")
				}
			})
		}
	}
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func TestTemplateTableAppearanceOverridesAreAdditive(t *testing.T) {
	reader, err := template.OpenTemplate(testutil.TableStyleTemplate(t, true))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	striped := true
	unstriped := false
	cases := []struct {
		name         string
		style        types.TableStyle
		want, absent string
	}{
		{"header", types.TableStyle{HeaderBackground: "accent1"}, `<a:schemeClr val="accent1"/>`, `<a:lnL`},
		{"no header", types.TableStyle{HeaderBackground: "none"}, `<a:noFill/>`, `<a:lnL`},
		{"borders", types.TableStyle{Borders: "all"}, `<a:lnL`, `<a:schemeClr val="accent1"/>`},
		{"delta", types.TableStyle{ColumnTypes: []string{"text", "text", "delta"}}, `<a:srgbClr val="007A33"/>`, `b="0"`},
		{"stripe", types.TableStyle{Striped: &striped}, `<a:lumMod val="15000"/>`, `<a:lnL`},
		{"no stripe", types.TableStyle{Striped: &unstriped}, `bandRow="0"`, `<a:solidFill`},
		{"totals", types.TableStyle{TotalsRow: true}, `<a:lnT`, `<a:lnL`},
		{"highlight", types.TableStyle{HighlightColumn: 2}, `<a:lumMod val="20000"/>`, `<a:lnL`},
		{"style only", types.TableStyle{UseTableStyle: true, HeaderBackground: "accent1", Borders: "all"}, `<a:tableStyleId>`, `<a:solidFill`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := eightRowTable()
			table.Style = tc.style
			res, err := GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 8229600}, Style: tc.style, StyleResolver: reader})
			if err != nil {
				t.Fatal(err)
			}
			if (tc.name == "header" || tc.name == "no header" || tc.name == "delta") && (strings.Contains(res.XML, `b="0"`) || strings.Contains(res.XML, `b="1"`)) {
				t.Error("color/fill override replaced template boldness")
			}
			if !strings.Contains(res.XML, tc.want) || strings.Contains(res.XML, tc.absent) {
				t.Fatalf("override not additive: %s", res.XML)
			}
		})
	}
}

func TestTableStyleCannotHideActualOverflow(t *testing.T) {
	reader, err := template.OpenTemplate(testutil.TableStyleTemplate(t, true))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	table := eightRowTable()
	table.Style = types.TableStyle{UseTableStyle: true}
	res, err := GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 8229600, Height: 900000}, Style: table.Style, StyleResolver: reader})
	if err != nil {
		t.Fatal(err)
	}
	if f := findingWithCode(res.Findings, patterns.ErrCodeTableRowsTruncated); f == nil || f.Action != "refuse" {
		t.Fatal("actual row loss was not refused")
	}
}

func TestExplicitStyleSelectionAndMissingDefinitionFallback(t *testing.T) {
	reader, err := template.OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, tc := range []struct {
		name, id string
		defined  bool
	}{
		{"explicit built-in is respected", types.DefaultTableStyleID, true},
		{"unknown GUID gets readable fallback", "{00000000-0000-0000-0000-000000000001}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := eightRowTable()
			table.Style = types.TableStyle{StyleID: tc.id}
			res, err := GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 8229600}, Style: table.Style, StyleResolver: reader})
			if err != nil {
				t.Fatal(err)
			}
			if res.ResolvedStyleID != tc.id {
				t.Fatalf("explicit style changed to %s", res.ResolvedStyleID)
			}
			if strings.Contains(res.XML, `<a:lnL`) == tc.defined {
				t.Fatal("definition inheritance/fallback mismatch")
			}
			if !strings.Contains(res.XML, `sz="1200"`) {
				t.Fatal("explicit style inflated typography")
			}
		})
	}
}

func TestExplicitTableFontSizeIsIndependentOfAppearance(t *testing.T) {
	// Check wide-table scaling and row pressure too, including the 10pt floor.
	for _, size := range []int{1000, 1200, 1400, 1800} {
		for _, cols := range []int{3, 5, 6, 7, 10} {
			for _, height := range []int64{1900000, 4000000} {
				var heights []int64
				var fonts []string
				for _, defined := range []bool{false, true} {
					reader, err := template.OpenTemplate(testutil.TableStyleTemplate(t, defined))
					if err != nil {
						t.Fatal(err)
					}
					table := &types.TableSpec{Headers: make([]string, cols), Rows: make([][]types.TableCell, 8)}
					for col := range cols {
						table.Headers[col] = "Column"
					}
					for row := range table.Rows {
						table.Rows[row] = make([]types.TableCell, cols)
						for col := range cols {
							table.Rows[row][col] = types.TableCell{Content: "12", ColSpan: 1, RowSpan: 1}
						}
					}
					res, err := GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 10000000, Height: height}, Style: table.Style, StyleResolver: reader, DefaultSize: size})
					reader.Close()
					if err != nil {
						t.Fatal(err)
					}
					heights = append(heights, res.Height)
					fonts = append(fonts, strings.Join(tableFontSizeRE.FindAllString(res.XML, -1), ","))
				}
				if heights[0] != heights[1] || fonts[0] != fonts[1] {
					t.Errorf("size %d, columns %d, height %d changed geometry/type with appearance: %v", size, cols, height, heights)
				}
			}
		}
	}
}

var tableFontSizeRE = regexp.MustCompile(`sz="[0-9]+"`)

func TestStyleOnlyPreservesTotalsAndDeltaAppearance(t *testing.T) {
	reader, err := template.OpenTemplate(testutil.TableStyleTemplate(t, true))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	table := eightRowTable()
	table.Style = types.TableStyle{UseTableStyle: true, TotalsRow: true, ColumnTypes: []string{"text", "text", "delta"}}
	table.Rows[0][2].Conditional = &types.ConditionalFormat{Fill: "accent1"}
	res, err := GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 8229600}, Style: table.Style, StyleResolver: reader})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.XML, `lastRow="1"`) {
		t.Error("totals style region was not enabled")
	}
	for _, override := range []string{`b="1"`, `b="0"`, `<a:solidFill`, `<a:noFill`, `<a:ln`} {
		if strings.Contains(res.XML, override) {
			t.Errorf("style-only table overridden by %s", override)
		}
	}
}

func TestConditionalFillOnlyOverridesMatchingTemplateCells(t *testing.T) {
	reader, err := template.OpenTemplate(testutil.TableStyleTemplate(t, true))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	table := eightRowTable()
	table.Rows[0][2].Conditional = &types.ConditionalFormat{Rule: types.ConditionalRuleGTE, Threshold: float64(7), Fill: "accent1"}
	table.Rows[1][2].Conditional = &types.ConditionalFormat{Rule: types.ConditionalRuleGTE, Threshold: float64(100), Fill: "accent1"}
	res, err := GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 8229600}, Style: table.Style, StyleResolver: reader})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(res.XML, `<a:solidFill>`) != 1 || strings.Contains(res.XML, `<a:ln`) || strings.Contains(res.XML, `<a:noFill`) {
		t.Error("conditional fill masked other template appearance")
	}
}

func TestTableColumnWidthsDefaultMatchesCompactRenderer(t *testing.T) {
	table := qbrTable()
	want := TableColumnWidths(table.Headers, table.Rows, 8229600, 1200)
	got := TableColumnWidths(table.Headers, table.Rows, 8229600, 0)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("default widths %v differ from 12pt renderer widths %v", got, want)
	}
	internal := calculateColumnWidths(len(table.Headers), 8229600, table.Headers, table.Rows, 0)
	if !reflect.DeepEqual(internal, want) {
		t.Errorf("internal default widths %v differ from renderer %v", internal, want)
	}
}
