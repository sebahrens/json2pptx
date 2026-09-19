package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestTableAltText covers both halves of the contract: an authored alt is used
// verbatim, and without one the description says the table's shape and columns
// rather than nothing at all (go-slide-creator-6e8h).
func TestTableAltText(t *testing.T) {
	tests := []struct {
		name string
		spec *types.TableSpec
		want string
	}{
		{
			name: "nil table",
			spec: nil,
			want: "Table",
		},
		{
			name: "derived from headers and row count",
			spec: &types.TableSpec{
				Headers: []string{"Segment", "FY25", "FY26", "Change"},
				Rows: [][]types.TableCell{
					{{Content: "Enterprise"}, {Content: "$28.4M"}, {Content: "$41.2M"}, {Content: "+45%"}},
					{{Content: "SMB"}, {Content: "$9.6M"}, {Content: "$8.9M"}, {Content: "-7%"}},
				},
			},
			want: "Table, 4 columns by 2 rows. Columns: Segment, FY25, FY26, Change.",
		},
		{
			name: "authored alt wins",
			spec: &types.TableSpec{
				Headers: []string{"Segment"},
				Rows:    [][]types.TableCell{{{Content: "Enterprise"}}},
				Alt:     "Enterprise added $12.8M; SMB gave back $0.7M.",
			},
			want: "Enterprise added $12.8M; SMB gave back $0.7M.",
		},
		{
			name: "single column and row are singular",
			spec: &types.TableSpec{
				Headers: []string{"Segment"},
				Rows:    [][]types.TableCell{{{Content: "Enterprise"}}},
			},
			want: "Table, 1 column by 1 row. Columns: Segment.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TableAltText(tt.spec); got != tt.want {
				t.Errorf("TableAltText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDiagramAltTextForConvertedChart measures the description against the
// payload ChartSpec.ToDiagramSpec actually builds. That conversion runs in Go,
// so `categories` is a []string and `series` a []map[string]any — not the
// []any a JSON decode produces. The first cut of this code asserted []any and
// silently described every flat chart as nothing but its type and title.
func TestDiagramAltTextForConvertedChart(t *testing.T) {
	tests := []struct {
		name  string
		chart *types.ChartSpec //nolint:staticcheck // ChartSpec is the surface being converted
		want  string
	}{
		{
			name: "flat bar data",
			chart: &types.ChartSpec{ //nolint:staticcheck // see above
				Type:      "bar",
				Title:     "Revenue by Quarter ($M)",
				Data:      map[string]any{"Q1": 12.0, "Q2": 14.5, "Q3": 18.0},
				DataOrder: []string{"Q1", "Q2", "Q3"},
			},
			want: "Bar chart, Revenue by Quarter ($M). 3 categories, 1 series, values from 12 to 18.",
		},
		{
			name: "pie data has no series objects",
			chart: &types.ChartSpec{ //nolint:staticcheck // see above
				Type:      "pie",
				Title:     "Market Share",
				Data:      map[string]any{"Us": 35.0, "Them": 65.0},
				DataOrder: []string{"Us", "Them"},
			},
			want: "Pie chart, Market Share. 2 categories, 1 series, values from 35 to 65.",
		},
		{
			name: "funnel data is a labelled point list",
			chart: &types.ChartSpec{ //nolint:staticcheck // see above
				Type:      "funnel",
				Title:     "Pipeline",
				Data:      map[string]any{"Leads": 10000.0, "Closed": 350.0},
				DataOrder: []string{"Leads", "Closed"},
			},
			want: "Funnel chart, Pipeline. 2 categories, values from 350 to 10000.",
		},
		{
			name: "gauge data is one reading against its scale",
			chart: &types.ChartSpec{ //nolint:staticcheck // see above
				Type:  "gauge",
				Title: "NPS Score",
				Data:  map[string]any{"score": 72.0},
			},
			want: "Gauge chart, NPS Score. 72 of 100.",
		},
		{
			name: "alt survives the conversion",
			chart: &types.ChartSpec{ //nolint:staticcheck // see above
				Type:  "bar",
				Title: "Revenue by Quarter ($M)",
				Data:  map[string]any{"Q1": 12.0},
				Alt:   "Revenue rises in every quarter of FY26.",
			},
			want: "Revenue rises in every quarter of FY26.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DiagramAltTextFor(tt.chart.ToDiagramSpec()); got != tt.want {
				t.Errorf("DiagramAltTextFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDiagramAltTextForFrameworkSections checks the section count a framework
// payload gets instead of the bare type name, and that an acronym type reads
// back as the acronym.
func TestDiagramAltTextForFrameworkSections(t *testing.T) {
	got := DiagramAltTextFor(&types.DiagramSpec{
		Type:  "swot",
		Title: "Where we stand",
		Data: map[string]any{
			"strengths":     []any{"Migrated early"},
			"weaknesses":    []any{"Manual reconciliation"},
			"opportunities": []any{"T+1 mandate"},
			"threats":       []any{"Fixed deadline"},
		},
	})
	want := "SWOT diagram, Where we stand. 4 sections."
	if got != want {
		t.Errorf("DiagramAltTextFor() = %q, want %q", got, want)
	}
}

// TestAltTextTruncation keeps a description to something a screen reader can
// announce: an over-long authored alt is cut at altMaxLen with an ellipsis.
func TestAltTextTruncation(t *testing.T) {
	long := strings.Repeat("a", altMaxLen+50)
	got := DiagramAltTextFor(&types.DiagramSpec{Type: "bar_chart", Alt: long})
	if len([]rune(got)) != altMaxLen {
		t.Fatalf("truncated length = %d runes, want %d", len([]rune(got)), altMaxLen)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated description does not end in an ellipsis: %q", got)
	}
}

// TestTableFrameCarriesAltText checks the whole table path: GenerateTableXML
// writes the description onto the frame's cNvPr, and insertTableFrames — which
// rewrites that element to renumber the shape id — keeps it there
// (go-slide-creator-6e8h).
func TestTableFrameCarriesAltText(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Segment", "FY26"},
		Rows: [][]types.TableCell{
			{{Content: "Enterprise"}, {Content: "$41.2M"}},
		},
		Alt: `Enterprise is 64% of FY26 revenue & rising.`,
	}
	result, err := GenerateTableXML(table, TableRenderConfig{
		Bounds: types.BoundingBox{X: 0, Y: 0, Width: 5000000, Height: 1000000},
	})
	if err != nil {
		t.Fatalf("GenerateTableXML: %v", err)
	}
	wantDescr := `descr="Enterprise is 64% of FY26 revenue &amp; rising."`
	if !strings.Contains(result.XML, wantDescr) {
		t.Fatalf("table frame missing %s:\n%s", wantDescr, result.XML)
	}

	slide := []byte(`<p:sld><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="9" name="retained"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`)
	got, err := insertTableFrames(slide, []tableInsert{{graphicFrameXML: result.XML}})
	if err != nil {
		t.Fatalf("insertTableFrames: %v", err)
	}
	if !strings.Contains(string(got), wantDescr) {
		t.Errorf("renumbering the frame dropped the alt text:\n%s", got)
	}
	if !strings.Contains(string(got), `<p:cNvPr id="10" name="Table 1" `) {
		t.Errorf("frame was not renumbered:\n%s", got)
	}
}
