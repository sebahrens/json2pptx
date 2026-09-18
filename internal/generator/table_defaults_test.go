package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func qbrTable() *types.TableSpec {
	rows := [][]string{
		{"North America", "24.1", "+1%", "+14%", "112%", "2.8x"},
		{"EMEA", "18.7", "+11%", "+34%", "124%", "3.9x"},
		{"APAC", "6.3", "-3%", "+18%", "109%", "2.6x"},
		{"LATAM", "3.3", "+2%", "+22%", "115%", "3.4x"},
		{"Total", "52.4", "+4%", "+21%", "117%", "3.2x"},
	}
	t := &types.TableSpec{
		Headers: []string{"Region", "Revenue ($M)", "vs Plan", "YoY growth", "NRR", "Pipeline cov."},
		Style:   types.DefaultTableStyle,
	}
	for _, r := range rows {
		var cells []types.TableCell
		for _, c := range r {
			cells = append(cells, types.TableCell{Content: c, ColSpan: 1, RowSpan: 1})
		}
		t.Rows = append(t.Rows, cells)
	}
	return t
}

var trRegexp = regexp.MustCompile(`(?s)<a:tr .*?</a:tr>`)
var tcRegexp = regexp.MustCompile(`(?s)<a:tc>.*?</a:tc>`)

// go-slide-creator-weaq: the QBR table with the default style gets a styled
// header, right-aligned numeric columns and an emphasised Total row.
func TestDefaultTableStyling_QBR(t *testing.T) {
	table := qbrTable()
	res, err := GenerateTableXML(table, TableRenderConfig{
		Bounds: types.BoundingBox{Width: 10000000, Height: 4000000},
		Style:  table.Style,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := trRegexp.FindAllString(res.XML, -1)
	if len(rows) != 6 {
		t.Fatalf("rows = %d, want 6", len(rows))
	}
	header := tcRegexp.FindAllString(rows[0], -1)
	for i, c := range header {
		if !strings.Contains(c, `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:tcPr>`) {
			t.Errorf("header cell %d missing accent1 solidFill", i)
		}
		if !strings.Contains(c, `b="1"`) {
			t.Errorf("header cell %d not bold", i)
		}
	}
	if !strings.Contains(header[1], `algn="r"`) || strings.Contains(header[0], `algn="r"`) {
		t.Error("numeric header should be right-aligned, label header left")
	}
	first := tcRegexp.FindAllString(rows[1], -1)
	if strings.Contains(first[0], `algn="r"`) {
		t.Error("label column should stay left-aligned")
	}
	for i := 1; i < len(first); i++ {
		if !strings.Contains(first[i], `algn="r"`) {
			t.Errorf("numeric cell %d not right-aligned: %s", i, first[i])
		}
	}
	total := tcRegexp.FindAllString(rows[5], -1)
	for i, c := range total {
		if !strings.Contains(c, `b="1"`) {
			t.Errorf("Total row cell %d not bold", i)
		}
		if !strings.Contains(c, `<a:lnT w="12700" cap="flat" cmpd="sng"><a:solidFill><a:schemeClr val="dk1"/>`) {
			t.Errorf("Total row cell %d missing top rule", i)
		}
	}
	if strings.Contains(rows[2], `b="1"`) {
		t.Error("ordinary data row should not be bold")
	}
	// Content-driven rows: not stretched to fill the 4,000,000 EMU placeholder.
	if res.Height >= 4000000 {
		t.Errorf("table height %d should be content-driven, not fill the placeholder", res.Height)
	}
}

func TestDefaultTableStyling_ExplicitChoicesWin(t *testing.T) {
	table := qbrTable()
	table.Style.HeaderBackground = "none"
	table.Style.ColumnTypes = []string{"text", "text", "text", "text", "text", "text"}
	res, err := GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 10000000}, Style: table.Style})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.XML, `<a:schemeClr val="accent1"/></a:solidFill></a:tcPr>`) {
		t.Error("header_background none must not get the default fill")
	}
	if strings.Contains(res.XML, `algn="r"`) {
		t.Error("explicit text column_types must not be right-aligned")
	}

	table = qbrTable()
	table.Style.UseTableStyle = true
	res, err = GenerateTableXML(table, TableRenderConfig{Bounds: types.BoundingBox{Width: 10000000}, Style: table.Style})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.XML, `<a:schemeClr val="lt1"/>`) {
		t.Error("use_table_style must defer header text to the table style")
	}
}

func TestIsNumericCell(t *testing.T) {
	for _, s := range []string{"24.1", "+1%", "-3%", "2.8x", "$1.2M", "(4.5)", "1,234", "€3bn", "12 pp", "−2%"} {
		if !isNumericCell(s) {
			t.Errorf("%q should be numeric", s)
		}
	}
	for _, s := range []string{"North America", "Q3 2025", "n/a text", "High", "v2 launch"} {
		if isNumericCell(s) {
			t.Errorf("%q should not be numeric", s)
		}
	}
}

func TestIsTotalRow(t *testing.T) {
	for _, s := range []string{"Total", "TOTAL", "Grand total", "Sum", "Subtotal", "Total (FY)"} {
		if !isTotalRow([]types.TableCell{{Content: s}}) {
			t.Errorf("%q should be a total row", s)
		}
	}
	for _, s := range []string{"Totally new", "Summary of risks", "EMEA", ""} {
		if isTotalRow([]types.TableCell{{Content: s}}) {
			t.Errorf("%q should not be a total row", s)
		}
	}
}
