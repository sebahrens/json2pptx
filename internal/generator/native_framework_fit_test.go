package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNativeFrameworkSparseContentShrinksAndReports(t *testing.T) {
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 10515600, Height: 5200000}
	cases := []struct {
		kind   string
		panels []nativePanelData
		meta   houseDiagramMeta
	}{
		{"swot", []nativePanelData{{title: "Strengths", body: "- Fast"}, {title: "Weaknesses", body: "- Slow"}, {title: "Opportunities", body: "- New"}, {title: "Threats", body: "- Cost"}}, houseDiagramMeta{}},
		{"pestel", []nativePanelData{{title: "Political", body: "- Policy"}, {title: "Economic", body: "- Growth"}, {title: "Social", body: "- Trust"}, {title: "Technology", body: "- AI"}, {title: "Environment", body: "- Energy"}, {title: "Legal", body: "- Rules"}}, houseDiagramMeta{}},
		{"kpi_dashboard", []nativePanelData{{title: "Revenue", value: "$21M", body: "+17%"}, {title: "Customers", value: "1,250", body: "+8%"}, {title: "NPS", value: "72", body: "+5"}, {title: "Churn", value: "2.1%", body: "-0.3%"}}, houseDiagramMeta{}},
		{"house_diagram", []nativePanelData{{title: "Vision"}, {title: "Growth", body: "- Improve"}, {title: "Quality", body: "- Maintain"}, {title: "People", body: "- Develop"}, {title: "Values"}}, houseDiagramMeta{floors: []houseFloorMeta{{floorType: "parallel", sectionCount: 3}}}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			ctx := &singlePassContext{}
			got := ctx.fitNativeFramework(2, 3, tc.kind, bounds, tc.panels, tc.meta)
			if got.Height >= bounds.Height || got.Height <= 0 {
				t.Fatalf("diagram not content-sized: height=%d, original=%d", got.Height, bounds.Height)
			}
			if got.Width != bounds.Width || got.X != bounds.X || abs((got.Y-bounds.Y)-(bounds.Height-got.Height)/2) > 1 {
				t.Fatalf("diagram not centred within original width: %+v", got)
			}
			if len(ctx.fitFindings) != 1 {
				t.Fatalf("want one sparse finding, got %+v", ctx.fitFindings)
			}
			f := ctx.fitFindings[0]
			if f.Code != patterns.ErrCodeSparseLayout || f.Pattern != tc.kind || f.Path != "/slides/1/content/3" || f.Fix == nil || f.Fix.Kind != "add_detail_or_resize" {
				t.Fatalf("wrong actionable finding: %+v", f)
			}
		})
	}
}

func TestNativeFrameworkDenseContentKeepsBoundsAndNoSparseFinding(t *testing.T) {
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 10515600, Height: 3600000}
	dense := strings.Repeat("- A substantial point about operational priorities and expected outcomes\n", 22)
	cases := []struct {
		kind   string
		panels []nativePanelData
		meta   houseDiagramMeta
	}{
		{"swot", []nativePanelData{{title: "Strengths", body: dense}, {title: "Weaknesses", body: "Short"}, {title: "Opportunities", body: "Short"}, {title: "Threats", body: "Short"}}, houseDiagramMeta{}},
		{"pestel", []nativePanelData{{title: "Political", body: dense}, {title: "Economic", body: "Short"}, {title: "Social", body: "Short"}, {title: "Technology", body: "Short"}, {title: "Environment", body: "Short"}, {title: "Legal", body: "Short"}}, houseDiagramMeta{}},
		{"kpi_dashboard", []nativePanelData{{title: "Revenue", value: strings.Repeat("very long metric ", 80), body: "Increasing"}}, houseDiagramMeta{}},
		{"house_diagram", []nativePanelData{{title: "Vision"}, {title: "Growth", body: dense}, {title: "Values"}}, houseDiagramMeta{floors: []houseFloorMeta{{floorType: "single", sectionCount: 1}}}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			ctx := &singlePassContext{}
			if got := ctx.fitNativeFramework(1, 0, tc.kind, bounds, tc.panels, tc.meta); got != bounds {
				t.Errorf("dense diagram shrank: got %+v, want %+v", got, bounds)
			}
			if len(ctx.fitFindings) != 0 {
				t.Errorf("dense diagram reported sparse: %+v", ctx.fitFindings)
			}
		})
	}
}

func TestNativeFrameworkInvalidBoundsPreserved(t *testing.T) {
	ctx := &singlePassContext{}
	bounds := types.BoundingBox{Width: 0, Height: 100}
	if got := ctx.fitNativeFramework(1, 0, "pestel", bounds, []nativePanelData{{title: "Political"}}, houseDiagramMeta{}); got != bounds {
		t.Fatalf("invalid bounds changed: %+v", got)
	}
	if len(ctx.fitFindings) != 0 {
		t.Fatalf("invalid bounds emitted findings: %+v", ctx.fitFindings)
	}
}

func TestNativeFrameworkEmptyKPIReportsSparse(t *testing.T) {
	ctx := &singlePassContext{}
	bounds := types.BoundingBox{Width: 8000000, Height: 4000000}
	got := ctx.fitNativeFramework(1, 0, "kpi_dashboard", bounds, []nativePanelData{{}}, houseDiagramMeta{})
	if got.Height >= bounds.Height {
		t.Fatalf("empty KPI did not shrink: %+v", got)
	}
	if len(ctx.fitFindings) != 1 || ctx.fitFindings[0].Code != patterns.ErrCodeSparseLayout || ctx.fitFindings[0].OverflowRatio != 0 {
		t.Fatalf("empty KPI did not produce zero-ink sparse finding: %+v", ctx.fitFindings)
	}
}

func TestNativeFrameworkSkewedContentUsesAverageInk(t *testing.T) {
	// One busy quadrant must not mask three near-empty ones. The painted card
	// height follows the busiest quadrant, but sparse detection estimates
	// visible text area across all four equally wide quadrants.
	ctx := &singlePassContext{}
	bounds := types.BoundingBox{Width: 8000000, Height: 4200000}
	panels := []nativePanelData{
		{title: "Strengths", body: strings.Repeat("- Detailed strength supporting the strategy\n", 5)},
		{title: "Weaknesses"}, {title: "Opportunities"}, {title: "Threats"},
	}
	ctx.fitNativeFramework(1, 0, "swot", bounds, panels, houseDiagramMeta{})
	if len(ctx.fitFindings) != 1 || ctx.fitFindings[0].OverflowRatio >= sparseLayoutThreshold {
		t.Fatalf("skewed diagram should report sparse ink, got %+v", ctx.fitFindings)
	}
}

func TestNativeFrameworkPESTELTwoBulletsPreserveBodyFont(t *testing.T) {
	// Regression: sizing from line count alone made two 11pt bullets fit only
	// after PowerPoint autofit reduced them to 8.6pt. Include their 6pt
	// paragraph spacing and indentation in the geometry budget.
	panels := []nativePanelData{
		{title: "Political", body: "- Trade policies\n- Government stability"},
		{title: "Economic", body: "- Interest rates\n- Inflation trends"},
		{title: "Social", body: "- Remote work adoption\n- Demographic shifts"},
		{title: "Technological", body: "- AI advancement\n- Cloud computing"},
		{title: "Environmental", body: "- Carbon regulations\n- Sustainability demand"},
		{title: "Legal", body: "- Data privacy laws\n- IP protection"},
	}
	bounds := types.BoundingBox{Width: 10803850, Height: 3600000}
	ctx := &singlePassContext{}
	fit := ctx.fitNativeFramework(1, 1, "pestel", bounds, panels, houseDiagramMeta{})
	cellW := (fit.Width - 2*pestelGap) / 3
	cellH := (fit.Height - pestelGap) / 2
	headerH := int64(float64(cellH) * pestelHeaderHeightRatio)
	for _, p := range panels {
		headerH = max(headerH, taxonomyTitleHeight(p.title, cellW, pestelBodyInset, pestelHeaderFontSize, ""))
	}
	bodyH := cellH - headerH
	for _, p := range panels {
		width := measureWidthEMU(cellW, pestelBodyInset, pestelBodyInset) - 24*int64(types.EMUPerPoint)
		textH := panelTextHeightEMU(panelBodyParagraphTexts(p.body, pestelBodyFontSize), "", width, pestelBodyFontSize, panelBulletSpaceAfter)
		if bodyH < textH+2*pestelBodyInset {
			t.Errorf("%s body allows %d EMU, needs %d at authored 11pt", p.title, bodyH, textH+2*pestelBodyInset)
		}
	}
}
