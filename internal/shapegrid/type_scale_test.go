package shapegrid

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func resolvedScaledShape(t *testing.T, mode, text string, widthPt, heightPt float64) *ShapeSpec {
	t.Helper()
	source := &ShapeSpec{Geometry: "rect", Text: json.RawMessage(text)}
	grid := &Grid{
		Bounds:  pptx.RectEmu{CX: PtToEMU(widthPt), CY: PtToEMU(heightPt)},
		Columns: []float64{100}, TypeScale: mode,
		Rows: []Row{{Cells: []Cell{{Shape: source}}}},
	}
	resolved, err := Resolve(grid, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(source.Text) != text {
		t.Fatalf("Resolve changed authored input: %s", source.Text)
	}
	return resolved.Cells[0].ShapeSpec
}

func firstScaledSize(t *testing.T, spec *ShapeSpec) float64 {
	t.Helper()
	tb, err := ResolveTextInput(spec.Text)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) == 0 || len(tb.Paragraphs[0].Runs) == 0 {
		t.Fatal("resolved text has no run")
	}
	return float64(tb.Paragraphs[0].Runs[0].FontSize) / 100
}

func TestTypeScaleGrowsSparseBodyToRoleCap(t *testing.T) {
	text := `{"content":"• Manual handoffs\n• Five-day close\n• Fragmented reporting\n• Reconciliation delays","size":12}`
	for _, tc := range []struct {
		mode string
		want float64
	}{
		{"compact", 12},
		{"comfortable", 14},
		{"presentation", 18},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			spec := resolvedScaledShape(t, tc.mode, text, 360, 180)
			if got := firstScaledSize(t, spec); got != tc.want {
				t.Errorf("rendered body size = %.2fpt, want %.2fpt", got, tc.want)
			}
			if tc.mode == "presentation" {
				xml, err := GenerateShapeXML(spec, 1, pptx.RectEmu{CX: PtToEMU(360), CY: PtToEMU(180)})
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(xml), `sz="1800"`) {
					t.Error("grown size is missing from rendered OOXML")
				}
			}
		})
	}
}

func TestTypeScaleRespectsCaptionKPIAndDenseCells(t *testing.T) {
	for _, tc := range []struct {
		name     string
		text     string
		widthPt  float64
		heightPt float64
		want     float64
	}{
		{"caption", `{"content":"Revenue growth","size":12}`, 300, 120, 14},
		{"kpi value", `{"content":"$12.4M","size":36,"bold":true}`, 300, 120, 48},
		{"step number stays in its hierarchy", `{"content":"1","size":24,"bold":true}`, 300, 120, 24},
		{"dense body", `{"content":"A long paragraph that cannot fit at its authored size because the cell is narrow and short","size":12}`, 100, 30, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := resolvedScaledShape(t, "presentation", tc.text, tc.widthPt, tc.heightPt)
			if got := firstScaledSize(t, spec); got != tc.want {
				t.Errorf("rendered size = %.2fpt, want %.2fpt", got, tc.want)
			}
		})
	}
}

func TestTypeScalePreservesParagraphStylesAndSource(t *testing.T) {
	text := `{"paragraphs":[{"content":"$12.4M","size":32,"bold":true,"color":"accent1"},{"content":"Annual recurring revenue","size":12,"color":"accent2"}],"inset_left":8}`
	spec := resolvedScaledShape(t, "presentation", text, 400, 170)
	if !strings.Contains(string(spec.Text), `"accent1"`) || !strings.Contains(string(spec.Text), `"accent2"`) {
		t.Fatalf("paragraph styling was dropped: %s", spec.Text)
	}
	if got := firstScaledSize(t, spec); got <= 32 || got > 48 {
		t.Errorf("KPI value size = %.2fpt, want growth no higher than 48pt", got)
	}
	tb, err := ResolveTextInput(spec.Text)
	if err != nil {
		t.Fatal(err)
	}
	if got := float64(tb.Paragraphs[1].Runs[0].FontSize) / 100; got <= 12 || got > 14 {
		t.Errorf("supporting caption size = %.2fpt, want growth no higher than 14pt", got)
	}
}

func TestTypeScaleSnapsOffScaleSizesDown(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want []float64
	}{
		{"card header 16 to subhead", `{"content":"Quick Wins","size":16,"bold":true}`, []float64{14}},
		{"table header 13 to body", `{"content":"Time to value","size":13,"bold":true}`, []float64{12}},
		{"label 22 to lead", `{"content":"North America","size":22}`, []float64{18}},
		{"name 20 to lead", `{"content":"Jane Smith","size":20}`, []float64{18}},
		{"on-scale untouched", `{"content":"Body","size":12}`, []float64{12}},
		{"sub-12 sizes render at the shape floor", `{"content":"Source: survey","size":8}`, []float64{12}},
		{"display heading untouched", `{"content":"Domestic","size":32}`, []float64{32}},
		{"KPI figure keeps measured size", `{"content":"$4.2M\nrevenue","size":24}`, []float64{24, 24}},
		{"step numeral keeps size", `{"content":"1","size":25,"bold":true}`, []float64{25}},
		{"small figure snaps", `{"content":"2x to 5x","size":15}`, []float64{14}},
		{"paragraphs snap one by one", `{"paragraphs":[{"content":"01","size":20,"bold":true},{"content":"Growth is plateauing","size":17,"color":"accent1"}]}`, []float64{20, 14}},
		// A value its pattern marks "figure" keeps the size it was fitted to
		// from the 16pt KPI floor up, digit or not; unmarked text of the same
		// size still settles onto the scale (go-slide-creator-a5ogo, -3hxu6).
		{"marked figure keeps the 16pt floor", `{"paragraphs":[{"content":"EUR 123.4M","size":16,"bold":true,"figure":true},{"content":"Net revenue retention","size":12}]}`, []float64{16, 12}},
		{"marked figure keeps 17pt", `{"paragraphs":[{"content":"EUR 123.4M","size":17,"bold":true,"figure":true}]}`, []float64{17}},
		{"marked figure without a digit keeps its size", `{"paragraphs":[{"content":"Unlimited","size":23,"bold":true,"figure":true}]}`, []float64{23}},
		{"unmarked 16pt figure snaps", `{"paragraphs":[{"content":"EUR 123.4M","size":16,"bold":true}]}`, []float64{14}},
		{"a mark under the floor is ignored", `{"paragraphs":[{"content":"EUR 123.4M","size":15,"bold":true,"figure":true}]}`, []float64{14}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := resolvedScaledShape(t, "", tc.text, 300, 200)
			tb, err := ResolveTextInput(spec.Text)
			if err != nil {
				t.Fatal(err)
			}
			if len(tb.Paragraphs) != len(tc.want) {
				t.Fatalf("paragraphs = %d, want %d", len(tb.Paragraphs), len(tc.want))
			}
			for i, want := range tc.want {
				if got := float64(tb.Paragraphs[i].Runs[0].FontSize) / 100; got != want {
					t.Errorf("paragraph %d size = %.2fpt, want %.2fpt", i, got, want)
				}
			}
			if strings.Contains(tc.text, "accent1") && !strings.Contains(string(spec.Text), "accent1") {
				t.Errorf("paragraph styling dropped: %s", spec.Text)
			}
		})
	}
}

func TestTypeScaleKeepTextSizesLeavesAuthoredSizes(t *testing.T) {
	text := `{"content":"Quick Wins","size":16}`
	grid := &Grid{
		Bounds:  pptx.RectEmu{CX: PtToEMU(300), CY: PtToEMU(200)},
		Columns: []float64{100}, KeepTextSizes: true,
		Rows: []Row{{Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect", Text: json.RawMessage(text)}}}}},
	}
	resolved, err := Resolve(grid, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := firstScaledSize(t, resolved.Cells[0].ShapeSpec); got != 16 {
		t.Errorf("free-mode authored size = %.2fpt, want 16pt kept", got)
	}
}
