package svggen

import (
	"strings"
	"testing"
)

// vennFrame is a real slide frame: the SVG canvas and its placed size.
type vennFrame struct {
	name             string
	w, h             int
	placedW, placedH float64
}

// Frames measured from generated decks: the One Content body placeholder of
// the shipped 16:9 templates (the review's semantic-venn.svg is 1061×384 drawn
// into 796×288 pt) and a half-width column of a two-column layout.
var vennSlideFrames = []vennFrame{
	{"one-content", 1061, 384, 795.75, 288},
	{"two-column", 520, 384, 390, 288},
}

// renderVennInFrame renders data like the generator does (placement floor of
// 10pt report body text) and returns the builder, the chart and its findings.
func renderVennInFrame(t *testing.T, data map[string]any, f vennFrame) (*SVGBuilder, *VennChart) {
	t.Helper()
	req := &RequestEnvelope{
		Type:   "venn",
		Data:   data,
		Output: OutputSpec{Width: f.w, Height: f.h},
		Style:  StyleSpec{PlacementWidthPt: f.placedW, PlacementHeightPt: f.placedH, MinReadablePt: 10},
	}
	var chart *VennChart
	builder, _, err := RenderWithHelper(req, func(b *SVGBuilder, req *RequestEnvelope) error {
		vd, err := parseVennData(req)
		if err != nil {
			return err
		}
		cfg := DefaultVennConfig(b.Width(), b.Height())
		if ov, ok := req.Data["overlap_ratio"].(float64); ok {
			cfg.OverlapRatio = ov
			cfg.FixedOverlap = true
		}
		chart = NewVennChart(b, cfg)
		return chart.Draw(vd)
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return builder, chart
}

// assertCaptionInRegion checks every drawn line of caption lies inside the
// circles named by key and outside the others.
func assertCaptionInRegion(t *testing.T, b *SVGBuilder, vc *VennChart, key, caption string) {
	t.Helper()
	var in, out []circleLayout
	for i, c := range vc.layouts {
		if strings.ContainsRune(key, rune('a'+i)) {
			in = append(in, c)
		} else {
			out = append(out, c)
		}
	}
	region := vennRegionShape{in: in, out: out}
	words := map[string]bool{}
	for _, w := range strings.Fields(caption) {
		words[w] = true
	}
	found := 0
	for _, tb := range b.textBoxes {
		lineWords := strings.Fields(tb.text)
		if len(lineWords) == 0 || !words[lineWords[0]] {
			continue
		}
		found += len(lineWords)
		r := tb.rect
		for _, p := range [][2]float64{{r.X, r.Y}, {r.X + r.W, r.Y}, {r.X, r.Y + r.H}, {r.X + r.W, r.Y + r.H}} {
			if !region.contains(p[0], p[1]) {
				t.Errorf("caption line %q (region %s) crosses the region outline at (%.1f, %.1f); box %+v", tb.text, key, p[0], p[1], r)
				break
			}
		}
	}
	if found < len(strings.Fields(caption)) {
		t.Errorf("caption %q: only %d of %d words drawn", caption, found, len(strings.Fields(caption)))
	}
}

func hasFinding(b *SVGBuilder, code string) bool {
	for _, f := range b.Findings() {
		if f.Code == code {
			return true
		}
	}
	return false
}

// The review's exact reproduction (go-slide-creator-b7qqg.27): a short
// intersection caption wrapped into three lines whose first and last words
// crossed both circle outlines.
func TestVennIntersectionCaptionFitsLens(t *testing.T) {
	data := map[string]any{
		"circles": []any{
			map[string]any{"label": "Customer need", "items": []any{"Fast setup", "Reliable support"}},
			map[string]any{"label": "Product strengths", "items": []any{"Integrations", "Automation"}},
		},
		"intersections": map[string]any{"ab": map[string]any{"label": "Focused value proposition"}},
	}
	for _, f := range vennSlideFrames {
		t.Run(f.name, func(t *testing.T) {
			b, vc := renderVennInFrame(t, data, f)
			assertCaptionInRegion(t, b, vc, "ab", "Focused value proposition")
			if hasFinding(b, FindingDiagramRegionOverflow) {
				t.Errorf("a caption that fits must not report %s", FindingDiagramRegionOverflow)
			}
			if min := b.MinDrawnFontSize(); min+0.05 < b.MinFontSize() {
				t.Errorf("drew text at %.2f, below the placement floor %.2f", min, b.MinFontSize())
			}
		})
	}
}

// Three-circle pairwise and triple captions stay in their own regions.
func TestVennThreeCircleCaptionsFitRegions(t *testing.T) {
	captions := map[string]string{
		"ab":  "Shared roadmap",
		"ac":  "Joint pricing",
		"bc":  "Common data",
		"abc": "Core platform",
	}
	inter := map[string]any{}
	for k, v := range captions {
		inter[k] = map[string]any{"label": v}
	}
	data := map[string]any{
		"circles": []any{
			map[string]any{"label": "Sales"},
			map[string]any{"label": "Product"},
			map[string]any{"label": "Finance"},
		},
		"intersections": inter,
	}
	for _, f := range vennSlideFrames {
		t.Run(f.name, func(t *testing.T) {
			b, vc := renderVennInFrame(t, data, f)
			for k, v := range captions {
				assertCaptionInRegion(t, b, vc, k, v)
			}
			if hasFinding(b, FindingDiagramRegionOverflow) {
				t.Errorf("fitting captions must not report %s: %+v", FindingDiagramRegionOverflow, b.Findings())
			}
		})
	}
}

// When the author pins overlap_ratio and the caption cannot fit, the renderer
// says so and names the overlap that would fit.
func TestVennRegionOverflowIsReported(t *testing.T) {
	data := map[string]any{
		"circles": []any{
			map[string]any{"label": "Customer need"},
			map[string]any{"label": "Product strengths"},
		},
		"overlap_ratio": 0.10,
		"intersections": map[string]any{"ab": map[string]any{"label": "Focused value proposition"}},
	}
	b, _ := renderVennInFrame(t, data, vennSlideFrames[0])
	var got *Finding
	for _, f := range b.Findings() {
		if f.Code == FindingDiagramRegionOverflow {
			got = &f
			break
		}
	}
	if got == nil {
		t.Fatalf("expected %s, got %+v", FindingDiagramRegionOverflow, b.Findings())
	}
	if got.Field != "intersections.ab" || !strings.Contains(got.Message, "Focused value proposition") {
		t.Errorf("finding does not point at the caption: %+v", got)
	}
	if got.Fix == nil || got.Fix.Kind != FixKindShortenLabels {
		t.Fatalf("fix = %+v, want %s", got.Fix, FixKindShortenLabels)
	}
	if ov, ok := got.Fix.Params["overlap_ratio"].(float64); !ok || ov <= 0.10 {
		t.Errorf("fix should suggest a wider overlap_ratio, params = %+v", got.Fix.Params)
	}
}
