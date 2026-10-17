package generator

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGraphicsAuditSemanticTableAndHeatmapColors(t *testing.T) {
	theme := &types.ThemeInfo{SemanticAccents: map[string]string{"positive": "accent3", "negative": "accent5"}}
	for _, tc := range []struct{ value, want string }{{"0.5", "accent3"}, {"−0.5", "accent5"}, {"(12%)", "accent5"}, {"+0", ""}, {"no data", ""}} {
		got := deltaTextColor(tc.value, theme)
		if tc.want == "" {
			if got != "" {
				t.Errorf("%s got %s", tc.value, got)
			}
		} else if !strings.Contains(got, `schemeClr val="`+tc.want+`"`) {
			t.Errorf("%s: %s", tc.value, got)
		}
	}
	for _, role := range []string{"positive", "negative"} {
		value := "1"
		if role == "negative" {
			value = "-1"
		}
		got := resolveConditionalFill(&types.ConditionalFormat{Rule: role}, value, theme)
		if !strings.Contains(got, theme.SemanticAccents[role]) {
			t.Fatalf("conditional role lost %s", got)
		}
	}
	tone := heatmapCellFill(7, 0, 10, "red", theme.SemanticAccents)
	if tone.scheme != "accent5" {
		t.Fatalf("heatmap role lost %+v", tone)
	}
	if !strings.Contains(deltaTextColor("0.5"), "007A33") || heatmapCellFill(7, 0, 10, "red").scheme != "#B91C1C" {
		t.Fatal("metadata-free fallbacks changed")
	}
}

func TestGraphicsAuditImageFormatFollowsBytes(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 3, 2)), nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"photo.jfif", "photo.jpe", "photo", "photo.png"} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		ctx := newSinglePassContext("", nil, nil, false, nil)
		media := ctx.allocateMediaSlot(path)
		if !strings.HasSuffix(media, ".jpg") || !ctx.usedExtensions["jpg"] || ctx.allocateMediaSlot(path) != media {
			t.Fatalf("wrong JPEG media registration %s %+v", media, ctx.usedExtensions)
		}
	}
	ctx := newSinglePassContext("", nil, nil, false, nil)
	path := filepath.Join(t.TempDir(), "fake.png")
	if err := os.WriteFile(path, []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := ctx.allocateMediaSlot(path); got != "" || len(ctx.usedExtensions) != 0 {
		t.Fatal("unsupported bytes allocated a media part")
	}
}

func TestGraphicsAuditFallbackUsesNativeText(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	bounds := types.BoundingBox{Width: 4000000, Height: 2000000}
	ctx.insertSVGFallbackImageSVG(1, bounds, 0)
	ctx.insertDiagramPlaceholderSVG(1, bounds, 1, "line_chart")
	ctx.finalizePanelGroupXML()
	if len(ctx.nativeSVGInserts[1]) != 0 || len(ctx.panelShapeInserts[1]) != 2 {
		t.Fatal("fallback depends on SVG support")
	}
	for _, ins := range ctx.panelShapeInserts[1] {
		s := ins.groupXML
		for _, want := range []string{`<p:sp>`, `schemeClr val="lt2"`, `schemeClr val="dk2"`, `typeface="+mn-lt"`} {
			if !strings.Contains(s, want) {
				t.Errorf("fallback missing %s: %s", want, s)
			}
		}
		if strings.Contains(s, "svgBlip") || strings.Contains(s, "srgbClr") {
			t.Fatal("fallback bypassed theme or depends on SVG")
		}
	}
}

func TestGraphicsAuditEscapersStripIllegalRunes(t *testing.T) {
	input := "A\x00\x0b\ufffe\uffff&<\"B"
	for _, tc := range []struct {
		name      string
		escape    func(string) string
		attribute bool
	}{{"notes", xmlEscapeString, false}, {"table", escapeXMLText, false}, {"theme", escapeXMLAttrValue, true}} {
		t.Run(tc.name, func(t *testing.T) {
			text := "<root>" + tc.escape(input) + "</root>"
			if tc.attribute {
				text = `<root value="` + tc.escape(input) + `"/>`
			}
			var decoded struct {
				Value string `xml:"value,attr"`
				Text  string `xml:",chardata"`
			}
			if err := xml.Unmarshal([]byte(text), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Value+decoded.Text != "A&<\"B" {
				t.Fatalf("incorrect sanitized value %+v", decoded)
			}
		})
	}
}

func TestGraphicsAuditFontAwareMeasurement(t *testing.T) {
	heading := strings.Repeat("iii ", 13)
	different := false
	for width := int64(40); width <= 180; width += 5 {
		if NativeColumnHeadingReserve(heading, width*12700, "Arial") != NativeColumnHeadingReserve(heading, width*12700, "Lora") {
			different = true
			break
		}
	}
	if !different {
		t.Fatal("font-specific wrapping never affected the heading reserve")
	}
	rows := [][]types.TableCell{{{Content: "Metric"}, {Content: "1111111111111111111111"}}}
	a, _, okA := financialTableWidths(2, 200*12700, []string{"Measure", "Amount"}, rows, 1200, "Arial")
	b, _, okB := financialTableWidths(2, 200*12700, []string{"Measure", "Amount"}, rows, 1200, "Lora")
	if !okA || !okB || a[1] == b[1] {
		t.Fatalf("financial width ignored template face: %v %v", a, b)
	}
}

func TestGraphicsAuditTableRendererUsesSemanticTheme(t *testing.T) {
	theme := &types.ThemeInfo{BodyFont: "Lora", SemanticAccents: map[string]string{"positive": "accent3", "negative": "accent5"}}
	table := &types.TableSpec{Headers: []string{"Movement"}, Rows: [][]types.TableCell{{{Content: "+0.5"}}, {{Content: "−0.5"}}}, Style: types.TableStyle{ColumnTypes: []string{"delta"}}}
	result, err := PopulateTableInShape(table, types.PlaceholderInfo{FontFamily: "+mn-lt", Bounds: types.BoundingBox{Width: 6000000, Height: 3000000}}, theme, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`schemeClr val="accent3"`, `schemeClr val="accent5"`} {
		if !strings.Contains(result.XML, want) {
			t.Errorf("table theme missing %s", want)
		}
	}
}

func TestGraphicsAuditMasterStylesStayIsolatedByMaster(t *testing.T) {
	files := map[string][]byte{}
	ctx := newSinglePassContext("", nil, nil, false, files)
	ctx.masterXMLCache = map[string][]byte{}
	for _, id := range []string{"1", "2"} {
		files[LayoutRelsPath("slideLayout"+id)] = []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster` + id + `.xml"/></Relationships>`)
		ctx.masterXMLCache["ppt/slideMasters/slideMaster"+id+".xml"] = []byte(`<p:txStyles><p:bodyStyle><a:lvl1pPr><a:defRPr sz="` + id + `400"/></a:lvl1pPr></p:bodyStyle></p:txStyles>`)
	}
	for _, id := range []string{"1", "2", "1"} {
		styles := ctx.masterStylesForLayout("slideLayout" + id)
		if got := masterTextStyleLevel(nil, "bodyStyle", 1, styles); !strings.Contains(got, `sz="`+id+`400"`) {
			t.Fatalf("master %s inherited another master's style: %s", id, got)
		}
	}
}
