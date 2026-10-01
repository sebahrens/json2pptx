package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func pStyleLikeTheme() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#000000"},
		{Name: "lt2", RGB: "#F2F2F2"},
		{Name: "accent1", RGB: "#FD5108"}, // 3.3:1 on white
		{Name: "accent2", RGB: "#FFB600"}, // ~1.8:1 on white
	}
}

func kpiCell(valueColor, labelColor string) string {
	return `<p:sp><p:spPr><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p><a:r><a:rPr lang="en-US" sz="4000" b="1"><a:solidFill><a:schemeClr val="` + valueColor + `"/></a:solidFill></a:rPr><a:t>$2.4B</a:t></a:r></a:p>` +
		`<a:p><a:r><a:rPr lang="en-US" sz="1100"><a:solidFill><a:schemeClr val="` + labelColor + `"/></a:solidFill></a:rPr><a:t>Revenue</a:t></a:r></a:p>` +
		`</p:txBody></p:sp>`
}

// A 40pt accent value that clears the 3:1 large-text bar keeps the accent;
// the 11pt label in the same body is still held to 4.5:1 (go-slide-creator-tinsz).
func TestShapeGridContrastJudgesBrandRunsAtTheirOwnSize(t *testing.T) {
	theme := pStyleLikeTheme()
	fixed, _ := fixShapeXMLContrast([]byte(kpiCell("accent1", "accent1")), theme, nil)
	out := string(fixed)
	value := out[:strings.Index(out, "Revenue")]
	value = value[:strings.Index(value, "$2.4B")]
	if !strings.Contains(value, `schemeClr val="accent1"`) {
		t.Errorf("40pt accent value lost its accent: %s", out)
	}
	label := out[strings.Index(out, "$2.4B"):]
	if strings.Contains(label, `schemeClr val="accent1"`) {
		t.Errorf("11pt accent label (3.3:1) was not fixed: %s", out)
	}
}

// Sibling KPI cards: the group and per-fill harmonisation passes must not
// re-blacken the large values once the small labels are fixed.
func TestShapeGridGroupKeepsLargeAccentValues(t *testing.T) {
	theme := pStyleLikeTheme()
	shapes := [][]byte{[]byte(kpiCell("accent1", "accent1")), []byte(kpiCell("accent1", "accent1")), []byte(kpiCell("accent1", "accent1"))}
	fixed, _ := enforceShapeGridContrast(shapes, theme, computeWhiteTextSafeHex(theme), 0)
	for i, sh := range fixed {
		out := string(sh)
		value := out[:strings.Index(out, "$2.4B")]
		if !strings.Contains(value, `schemeClr val="accent1"`) {
			t.Errorf("card %d: value lost its accent: %s", i, out)
		}
		if strings.Contains(out[strings.Index(out, "$2.4B"):], `schemeClr val="accent1"`) {
			t.Errorf("card %d: small label kept a failing accent: %s", i, out)
		}
	}
}

// A large brand colour that misses even 3:1 is darkened minimally in its own
// hue on a light fill, not snapped to dk1 / dk2 black.
func TestShapeGridLargeBrandRunDarkensInHue(t *testing.T) {
	theme := pStyleLikeTheme()
	fixed, _ := fixShapeXMLContrast([]byte(kpiCell("accent2", "dk1")), theme, nil)
	out := string(fixed)
	m := srgbClrInFillRegexp.FindStringSubmatch(out[strings.Index(out, "<p:txBody>"):])
	if m == nil {
		t.Fatalf("expected the 40pt accent2 value to be fixed: %s", out)
	}
	if strings.EqualFold(m[2], "000000") {
		t.Fatalf("large brand value snapped to black: %s", out)
	}
	c := svggen.MustParseColor("#" + m[2])
	if r := c.ContrastWith(svggen.MustParseColor("#FFFFFF")); r < svggen.WCAGAALarge || r > 4.0 {
		t.Errorf("replacement #%s ratio %.2f, want a minimal darken to >= 3:1", m[2], r)
	}
}

// Neutral inks keep the body-wide bar: a white header that would clear 3:1
// still flips with its small sibling line so one cell never splits.
func TestShapeGridNeutralInkKeepsBodyBar(t *testing.T) {
	theme := pStyleLikeTheme()
	cell := `<p:sp><p:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p><a:r><a:rPr sz="1400" b="1"><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:rPr><a:t>Head</a:t></a:r></a:p>` +
		`<a:p><a:r><a:rPr sz="1100"><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:rPr><a:t>Body</a:t></a:r></a:p></p:txBody></p:sp>`
	fixed, _ := fixShapeXMLContrast([]byte(cell), theme, nil)
	if strings.Contains(string(fixed), `schemeClr val="lt1"`) {
		t.Errorf("neutral ink split across the cell: %s", fixed)
	}
}
