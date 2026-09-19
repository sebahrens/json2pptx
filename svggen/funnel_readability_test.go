package svggen

import (
	"strings"
	"testing"
)

// saasFunnel is the reported case: a real pipeline spanning two orders of
// magnitude (go-slide-creator-6i6j).
func saasFunnel() FunnelData {
	return FunnelData{
		Title: "Pipeline conversion",
		Points: []FunnelDataPoint{
			{Label: "Visitors", Value: 12400},
			{Label: "MQL", Value: 3100},
			{Label: "SQL", Value: 890},
			{Label: "Won", Value: 212},
		},
	}
}

// Under proportional widths the 212 stage was 2px wide: a stick with an
// external leader line, impossible to compare against anything.
func TestFunnelClampedWidthKeepsEveryStageReadable(t *testing.T) {
	const plotW = 800.0
	data := saasFunnel()
	maxValue := data.Points[0].Value

	for _, p := range data.Points {
		w := funnelSegmentWidth(p.Value, maxValue, plotW, FunnelWidthClamped)
		if w < plotW*funnelMinWidthFrac {
			t.Errorf("%s is %.0f of %.0f (%.0f%%), below the %.0f%% floor",
				p.Label, w, plotW, w/plotW*100, funnelMinWidthFrac*100)
		}
	}

	// Ordering survives the clamp: a bigger stage is still wider.
	for i := 1; i < len(data.Points); i++ {
		prev := funnelSegmentWidth(data.Points[i-1].Value, maxValue, plotW, FunnelWidthClamped)
		cur := funnelSegmentWidth(data.Points[i].Value, maxValue, plotW, FunnelWidthClamped)
		if cur >= prev {
			t.Errorf("%s (%.0f) is not narrower than %s (%.0f)", data.Points[i].Label, cur, data.Points[i-1].Label, prev)
		}
	}

	// The other modes still mean what they say.
	if got := funnelSegmentWidth(212, maxValue, plotW, FunnelWidthProportional); got > plotW*0.03 {
		t.Errorf("proportional width = %.1f, want the true proportion", got)
	}
	if got := funnelSegmentWidth(212, maxValue, plotW, FunnelWidthEqual); got != plotW {
		t.Errorf("equal width = %.1f, want the full %.0f", got, plotW)
	}
}

// The conversion between stages is the number a funnel exists to show; the
// chart used to leave the division to the reader.
func TestFunnelShowsStageToStageConversion(t *testing.T) {
	data := saasFunnel()
	if got := funnelConversionLabel(data.Points, 0); got != "" {
		t.Errorf("the first stage has nothing to convert from, got %q", got)
	}
	for i, want := range map[int]string{1: "25% of Visitors", 2: "29% of MQL", 3: "24% of SQL"} {
		if got := funnelConversionLabel(data.Points, i); got != want {
			t.Errorf("stage %d conversion = %q, want %q", i, got, want)
		}
	}

	b := NewSVGBuilder(900, 540)
	chart := NewFunnelChart(b, DefaultFunnelChartConfig(900, 540))
	if err := chart.Draw(data); err != nil {
		t.Fatalf("draw: %v", err)
	}
	raw, err := b.RenderToBytes()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(raw)
	for _, want := range []string{"25% of Visitors", "29% of MQL", "24% of SQL"} {
		if !strings.Contains(svg, want) {
			t.Errorf("the rendered funnel does not carry %q", want)
		}
	}
	// Every stage's own label is inside the chart, including the smallest.
	for _, want := range []string{"Visitors: 12,400", "MQL: 3,100", "SQL: 890", "Won: 212"} {
		if !strings.Contains(svg, want) {
			t.Errorf("the rendered funnel lost the label %q", want)
		}
	}
}

// A funnel is ONE metric falling through its stages. The accent rotation
// painted MQL in the template's alert red and Won in its positive green,
// implying a valence the data does not carry.
func TestFunnelStagesShareOneHue(t *testing.T) {
	b := NewSVGBuilder(900, 540)
	chart := NewFunnelChart(b, DefaultFunnelChartConfig(900, 540))
	colors := chart.getColors(b.StyleGuide(), 4)
	if len(colors) != 4 {
		t.Fatalf("got %d colours, want 4", len(colors))
	}

	accents := b.StyleGuide().Palette.AccentColors()
	for i, c := range colors {
		for j, accent := range accents[1:] {
			if c.Hex() == accent.Hex() {
				t.Errorf("stage %d took accent%d (%s) from the categorical rotation", i, j+2, accent.Hex())
			}
		}
	}
	// The ramp runs light-ward, so the reader sees one quantity shrinking.
	for i := 1; i < len(colors); i++ {
		if colors[i].Luminance() <= colors[i-1].Luminance() {
			t.Errorf("stage %d is not lighter than stage %d", i, i-1)
		}
	}

	// A caller who supplies colours still gets exactly those.
	custom := []Color{MustParseColor("#111111"), MustParseColor("#222222")}
	cfg := DefaultFunnelChartConfig(900, 540)
	cfg.Colors = custom
	own := NewFunnelChart(b, cfg).getColors(b.StyleGuide(), 2)
	if own[0].Hex() != custom[0].Hex() || own[1].Hex() != custom[1].Hex() {
		t.Errorf("authored colours were replaced: %v", own)
	}
}
