package patterns

import "testing"

// Sizes outside 12–40pt used to validate and be silently clamped
// (go-slide-creator-csclk.110).
func TestTextSizeOverridesOutOfRangeRejected(t *testing.T) {
	fg, _ := Default().Get("framework-grid")
	fgVals := fg.(Exemplar).ExemplarValues()
	for _, ovr := range []*FrameworkGridOverrides{{BodySize: 6}, {TitleSize: 120}} {
		if err := fg.Validate(fgVals, ovr, nil); err == nil {
			t.Errorf("framework-grid overrides body=%v title=%v should be rejected", ovr.BodySize, ovr.TitleSize)
		}
	}
	if err := fg.Validate(fgVals, &FrameworkGridOverrides{BodySize: 14}, nil); err != nil {
		t.Errorf("framework-grid body_size 14 should validate: %v", err)
	}
	hm, _ := Default().Get("capability-heatmap")
	hmVals := hm.(Exemplar).ExemplarValues()
	for _, ovr := range []*CapabilityHeatmapOverrides{{CellSize: 6}, {HeaderSize: 120}} {
		if err := hm.Validate(hmVals, ovr, nil); err == nil {
			t.Errorf("capability-heatmap overrides cell=%v header=%v should be rejected", ovr.CellSize, ovr.HeaderSize)
		}
	}
}
