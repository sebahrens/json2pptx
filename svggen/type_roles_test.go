package svggen

import "testing"

// TestPresetTypographyOnTypeScale pins the chart typography tables to the
// slide type scale (go-slide-creator-vmdfm): every preset, the compact and
// default typographies, and the ScaleForDimensions caps and floors sit on a
// step; titles never fall below the subhead step, text below the dense-body
// step or labels below the 10pt caption step; and the roles keep their order
// (title > subtitle >= heading, body >= labels).
func TestPresetTypographyOnTypeScale(t *testing.T) {
	tables := map[string]*Typography{
		"DefaultTypography": DefaultTypography(),
		"CompactTypography": CompactTypography(),
	}
	for name, ty := range PresetTypography {
		tables["preset "+name] = ty
	}
	for name, ty := range tables {
		roles := []struct {
			role  string
			pt    float64
			floor float64
		}{
			{"title", ty.SizeTitle, ChartTitleMinPt},
			{"subtitle", ty.SizeSubtitle, ChartTextMinPt},
			{"heading", ty.SizeHeading, ChartTextMinPt},
			{"body", ty.SizeBody, ChartTextMinPt},
			{"small", ty.SizeSmall, ChartLabelMinPt},
			{"caption", ty.SizeCaption, ChartLabelMinPt},
		}
		for _, r := range roles {
			if !OnChartTypeScale(r.pt) {
				t.Errorf("%s: %s %gpt is off the type scale %v", name, r.role, r.pt, ChartTypeScaleStepsPt)
			}
			if r.pt < r.floor {
				t.Errorf("%s: %s %gpt is below its %gpt floor", name, r.role, r.pt, r.floor)
			}
		}
		if ty.SizeTitle <= ty.SizeSubtitle || ty.SizeSubtitle < ty.SizeHeading || ty.SizeBody < ty.SizeSmall {
			t.Errorf("%s: role order broken: title %g, subtitle %g, heading %g, body %g, small %g",
				name, ty.SizeTitle, ty.SizeSubtitle, ty.SizeHeading, ty.SizeBody, ty.SizeSmall)
		}
	}

	for _, c := range []struct {
		role       string
		ref, maxPt float64
	}{
		{"title", ChartTitlePt, ChartTitleMaxPt},
		{"subtitle", ChartSubtitlePt, ChartSubtitleMaxPt},
		{"heading", ChartHeadingPt, ChartHeadingMaxPt},
		{"body", ChartBodyPt, ChartBodyMaxPt},
		{"label", ChartLabelPt, ChartLabelMaxPt},
		{"caption", ChartCaptionPt, ChartCaptionMaxPt},
	} {
		if !OnChartTypeScale(c.maxPt) || c.maxPt <= c.ref {
			t.Errorf("%s cap %gpt must be a type-scale step above its %gpt reference", c.role, c.maxPt, c.ref)
		}
	}

	// A canvas large enough to hit every cap renders every role on a step.
	big := DefaultTypography().ScaleForDimensions(4000, 3000)
	for _, pt := range []float64{big.SizeTitle, big.SizeSubtitle, big.SizeHeading, big.SizeBody, big.SizeSmall, big.SizeCaption} {
		if !OnChartTypeScale(pt) {
			t.Errorf("capped size %gpt is off the type scale", pt)
		}
	}
	// So does a canvas small enough to hit every floor.
	small := DefaultTypography().ScaleForDimensions(100, 75)
	for _, pt := range []float64{small.SizeTitle, small.SizeSubtitle, small.SizeHeading, small.SizeBody, small.SizeSmall, small.SizeCaption} {
		if !OnChartTypeScale(pt) {
			t.Errorf("floored size %gpt is off the type scale", pt)
		}
	}
}
