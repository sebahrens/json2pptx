package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestLayoutReviewBaselinesFollowActualTextAnchors(t *testing.T) {
	input := geomSlides(t, `[{"layout_id":"content","shape_grid":{"type_scale":"compact","columns":2,"rows":[{"min_height":100,"max_height":100,"cells":[{"shape":{"text":{"content":"Short label","size":14}}},{"shape":{"text":{"paragraphs":[{"content":"Longer label","size":14},{"content":"A second line","size":14},{"content":"A third line","size":14}]}}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeBaselineMisalign)
	if len(fs) != 1 || fs[0].Fix.Kind != "align_top_insets" || fs[0].Action != "review" {
		t.Fatalf("missing peer baseline finding: %+v", fs)
	}
	for _, cell := range input.Slides[0].ShapeGrid.Rows[0].Cells {
		var text map[string]any
		if err := json.Unmarshal(cell.Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		text["vertical_align"] = "top"
		cell.Shape.Text, _ = json.Marshal(text)
	}
	if fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeBaselineMisalign); len(fs) != 0 {
		t.Fatalf("aligned text still flagged: %+v", fs)
	}
}

func TestLayoutReviewExhibitShareUsesResolvedFrame(t *testing.T) {
	input := geomSlides(t, `[{"layout_id":"content","shape_grid":{"columns":[1,3],"rows":[{"cells":[{"image":{"path":"proof.png"}},{"shape":{"text":"Evidence explanation"}}]}]}}]`)
	fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeExhibitTooSmall)
	if len(fs) != 1 {
		t.Fatalf("small sole exhibit not flagged: %+v", fs)
	}
	input.Slides[0].ShapeGrid.Columns = json.RawMessage(`[3,1]`)
	if fs := findingsByCode(collectGeometryFindings(input, nil, 0, 0, nil), patterns.ErrCodeExhibitTooSmall); len(fs) != 0 {
		t.Fatalf("large exhibit incorrectly flagged: %+v", fs)
	}
}

func TestLayoutReviewProsePaneFillAndDifferentRoles(t *testing.T) {
	frame := func(x int64) pptx.RectEmu { return pptx.RectEmu{X: x, CX: 2000000, CY: 1000000} }
	left, right := frame(0), frame(2100000)
	a := geomAccumulator{reviewPanes: []layoutReviewPane{{path: "/rows/0/cells/0", frame: left}, {path: "/rows/0/cells/1", frame: right}}, reviewText: []layoutReviewText{{path: "/rows/0/cells/0/grid/rows/0/cells/0", ink: pptx.RectEmu{CX: 1900000, CY: 900000}, paragraphs: []geomParagraph{{text: "Dense supporting prose", sizePt: 12}}}, {path: "/rows/0/cells/1/grid/rows/0/cells/0", ink: pptx.RectEmu{X: 2100000, CX: 900000, CY: 120000}, paragraphs: []geomParagraph{{text: "Short prose", sizePt: 12}}}}}
	if fs := a.paneFillReviewFindings(); len(fs) != 1 {
		t.Fatalf("uneven prose panes not flagged: %+v", fs)
	}
	a.reviewText[1].paragraphs = []geomParagraph{{text: "42%", sizePt: 40}}
	if fs := a.paneFillReviewFindings(); len(fs) != 0 {
		t.Fatalf("KPI figure treated as empty prose: %+v", fs)
	}
}

func TestLayoutReviewTitleOrphanAndNumberUnitRepair(t *testing.T) {
	m := newGeomMeasurer(nil)
	p := geomParagraph{text: "Revenue increased 25 percent", sizePt: 18}
	width := m.widthPt("Revenue increased 25", 18, false) + 0.1
	if reason := headingOrphanReason(m, p, width); !strings.Contains(reason, "number") {
		t.Fatalf("number/unit split missed: %q", reason)
	}
	p.text = "Revenue increased 25\u00a0percent"
	if reason := headingOrphanReason(m, p, width); reason != "" {
		t.Fatalf("non-breaking unit repair did not clear: %q", reason)
	}
	p.text = "Revenue increased substantially"
	if reason := headingOrphanReason(m, p, m.widthPt("Revenue increased", 18, false)+0.1); reason == "" {
		t.Fatal("orphan final word not reported")
	}
}

func TestLayoutReviewNativeTypeHierarchy(t *testing.T) {
	text := func(s string) *string { return &s }
	input := &PresentationInput{Slides: []SlideInput{{LayoutID: "content", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: text("Revenue grows")}, {PlaceholderID: "body", Type: "text", TextValue: text("Supporting explanation")}}}}}
	layouts := []types.LayoutMetadata{{ID: "content", Placeholders: []types.PlaceholderInfo{{ID: "title", Type: types.PlaceholderTitle, FontSize: 1600, Bounds: types.BoundingBox{Width: 500 * 12700, Height: 60 * 12700}}, {ID: "body", Type: types.PlaceholderBody, FontSize: 1400, Bounds: types.BoundingBox{Y: 100 * 12700, Width: 500 * 12700, Height: 300 * 12700}}}}}
	if fs := findingsByCode(collectNativeLayoutReviewFindings(input, layouts, 0, 0), patterns.ErrCodeTypeHierarchyFlat); len(fs) != 1 {
		t.Fatalf("flat hierarchy not flagged: %+v", fs)
	}
	layouts[0].Placeholders[0].FontSize = 2800
	if fs := findingsByCode(collectNativeLayoutReviewFindings(input, layouts, 0, 0), patterns.ErrCodeTypeHierarchyFlat); len(fs) != 0 {
		t.Fatalf("clear hierarchy flagged: %+v", fs)
	}
}

func TestLayoutReviewNumberFormatsKeepUnitContextsSeparate(t *testing.T) {
	for _, tt := range []struct {
		name   string
		values []string
		want   bool
	}{
		{"decimal disagreement", []string{"USD 100", "USD 1,2", "USD 1.2"}, true},
		{"grouping disagreement", []string{"EUR 1,234.56", "EUR 1.234,56"}, true},
		{"consistent", []string{"USD 1.2", "USD 2.5"}, false},
		{"different currencies", []string{"EUR 1,2", "USD 1.2"}, false},
		{"different unit scales", []string{"EUR 1,2m", "EUR 1.2bn"}, false},
		{"percent disagreement", []string{"12,5%", "12.5%"}, true},
		{"prose may spell units", []string{"Spend is 23% over plan", "23 percent"}, false},
		{"display unit spelling", []string{"23%", "23 percent"}, true},
		{"prose currency spelling", []string{"Revenue was EUR 1.2m", "€1.2m"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := &PresentationInput{}
			for _, value := range tt.values {
				v := value
				input.Slides = append(input.Slides, SlideInput{Content: []ContentInput{{PlaceholderID: "body", Type: "text", TextValue: &v}}})
			}
			fs := collectNumberFormatFindings(input)
			if (len(fs) > 0) != tt.want {
				t.Fatalf("findings=%+v want=%v", fs, tt.want)
			}
			if len(fs) > 0 && (fs[0].Action != "info" || fs[0].Fix.Params["formatter_hint"] == nil) {
				t.Fatal("formatter advice not actionable")
			}
		})
	}
}

func TestLayoutReviewUsedStatusColorsOnly(t *testing.T) {
	input := geomSlides(t, `[{"pattern":{"name":"table-highlight","values":{"criteria":[{"label":"Risk","scale":"rag"}],"options":[{"name":"A","scores":["red"]},{"name":"B","scores":["amber"]}]},"overrides":{"rag_colors":{"red":"accent1","amber":"accent2"}}}}]`)
	theme := &types.ThemeInfo{Colors: []types.ThemeColor{{Name: "accent1", RGB: "C62828"}, {Name: "accent2", RGB: "C62828"}}}
	fs := collectStatusColorFindings(input, theme)
	if len(fs) != 1 || fs[0].Code != patterns.ErrCodeStatusColorsIndistinct {
		t.Fatalf("identical used status colors not flagged: %+v", fs)
	}
	input.Slides[0].Pattern.Values = json.RawMessage(`{"criteria":[{"label":"Risk","scale":"rag"}],"options":[{"name":"A","scores":["red"]},{"name":"B","scores":["red"]}]}`)
	if fs := collectStatusColorFindings(input, theme); len(fs) != 0 {
		t.Fatalf("unused amber produced a finding: %+v", fs)
	}
}
