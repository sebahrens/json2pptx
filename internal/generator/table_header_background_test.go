package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-87eu0: header_background alone is additive — the table
// keeps the engine-default type (12pt rows, 11pt header), horizontal rules
// only and no zebra; only the header fill and header text colour change.
func TestDefaultTableStyling_HeaderBackgroundIsAdditive(t *testing.T) {
	table := qbrTable()
	table.Style.HeaderBackground = "accent1"
	if !IsEngineDefaultTableStyle(table.Style) {
		t.Fatal("header_background alone must stay in the engine default")
	}
	if got := TableBaseFontSize(table.Style); got != engineDefaultRowFontSize {
		t.Errorf("TableBaseFontSize = %d, want %d", got, engineDefaultRowFontSize)
	}
	res, err := GenerateTableXML(table, TableRenderConfig{
		Bounds: types.BoundingBox{Width: 10000000, Height: 4000000},
		Style:  table.Style,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.XML, `sz="1800"`) || strings.Contains(res.XML, `sz="1980"`) {
		t.Error("header_background must not revert to the legacy 18pt type")
	}
	if strings.Contains(res.XML, `bandRow="1"`) {
		t.Error("header_background must not switch on zebra stripes")
	}
	if strings.Contains(res.XML, `<a:lnL w="12700"`) || strings.Contains(res.XML, `<a:lnR w="12700"`) {
		t.Error("header_background must not draw vertical grid lines")
	}
	rows := trRegexp.FindAllString(res.XML, -1)
	if len(rows) < 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	for i, c := range tcRegexp.FindAllString(rows[0], -1) {
		if !strings.Contains(c, `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:tcPr>`) {
			t.Errorf("header cell %d missing accent1 fill", i)
		}
		if !strings.Contains(c, `sz="1100" b="1"`) {
			t.Errorf("header cell %d not 11pt bold", i)
		}
		if !strings.Contains(c, `<a:schemeClr val="lt1"/>`) {
			t.Errorf("header cell %d text should flip to lt1 on accent1", i)
		}
		// The 1pt rule under the header stays.
		if !strings.Contains(c, `<a:lnB w="12700"`) {
			t.Errorf("header cell %d lost its 1pt rule", i)
		}
	}
	for i, c := range tcRegexp.FindAllString(rows[2], -1) {
		if !strings.Contains(c, `sz="1200"`) {
			t.Errorf("data cell %d not 12pt", i)
		}
		if strings.Contains(c, `<a:solidFill><a:schemeClr val="accent1"><a:lumMod`) {
			t.Errorf("data cell %d carries a zebra stripe", i)
		}
	}
}

// go-slide-creator-87eu0: explicit borders / striped still opt in on top of
// a header_background.
func TestDefaultTableStyling_HeaderBackgroundWithBordersAndStripes(t *testing.T) {
	table := qbrTable()
	table.Style.HeaderBackground = "accent2"
	table.Style.Borders = "all"
	striped := true
	table.Style.Striped = &striped
	res, err := GenerateTableXML(table, TableRenderConfig{
		Bounds: types.BoundingBox{Width: 10000000, Height: 4000000},
		Style:  table.Style,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.XML, `<a:lnL w="12700"`) {
		t.Error("borders: all must draw vertical grid lines")
	}
	if !strings.Contains(res.XML, `bandRow="1"`) {
		t.Error("striped: true must band rows")
	}
	if !strings.Contains(res.XML, `sz="1200"`) || strings.Contains(res.XML, `sz="1800"`) {
		t.Error("explicit borders/striped keep the engine-default type size")
	}
}

// go-slide-creator-87eu0: header text is chosen by contrast with the fill.
func TestHeaderTextColorXML_Contrast(t *testing.T) {
	theme := &types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "000000"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "accent1", RGB: "1F3864"}, {Name: "accent4", RGB: "FFD966"},
		{Name: "lt2", RGB: "E7E6E6"},
	}}
	for _, tc := range []struct{ bg, want string }{
		{"accent1", "lt1"}, {"accent4", "tx1"}, {"lt2", "tx1"},
		{"#0A0A40", "lt1"}, {"#F0F0F0", "tx1"},
	} {
		cfg := TableRenderConfig{Theme: theme, engineDefault: true,
			Style: types.TableStyle{HeaderBackground: tc.bg}}
		got := headerTextColorXML(cfg)
		if !strings.Contains(got, `val="`+tc.want+`"`) {
			t.Errorf("%s: header text = %s, want %s", tc.bg, got, tc.want)
		}
	}
	// Without a theme, dark scheme names still fall back to lt1.
	if got := headerTextColorXML(TableRenderConfig{engineDefault: true,
		Style: types.TableStyle{HeaderBackground: "accent3"}}); !strings.Contains(got, `val="lt1"`) {
		t.Errorf("accent3 without theme = %s, want lt1", got)
	}
}
