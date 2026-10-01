package tokens_test

import (
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/svggen"
)

// TestChartTextRolesOnSlideScale pins svggen's chart text roles to the slide
// type scale (go-slide-creator-vmdfm): a chart title is the slide lead step at
// the reference canvas and never smaller than the subhead (card title) step,
// chart body text is the slide body step, labels and captions the caption
// step. svggen is a separate module and cannot import the tokens, so this
// test is what keeps the two scales one.
func TestChartTextRolesOnSlideScale(t *testing.T) {
	for _, c := range []struct {
		role      string
		chart, pt float64
	}{
		{"title", svggen.ChartTitlePt, tokens.TypeScaleLeadPt},
		{"subtitle", svggen.ChartSubtitlePt, tokens.TypeScaleSubheadPt},
		{"heading", svggen.ChartHeadingPt, tokens.TypeScaleBodyPt},
		{"body", svggen.ChartBodyPt, tokens.TypeScaleBodyPt},
		{"label", svggen.ChartLabelPt, tokens.TypeScaleCaptionPt},
		{"caption", svggen.ChartCaptionPt, tokens.TypeScaleCaptionPt},
		{"title floor", svggen.ChartTitleMinPt, tokens.TypeScaleSubheadPt},
		{"text floor", svggen.ChartTextMinPt, tokens.BodyTextMinPt},
		{"label floor", svggen.ChartLabelMinPt, tokens.TypeScaleCaptionPt},
	} {
		if c.chart != c.pt {
			t.Errorf("chart %s = %gpt, want the slide step %gpt", c.role, c.chart, c.pt)
		}
		if !tokens.OnTypeScaleHPt(int(math.Round(c.chart * 100))) {
			t.Errorf("chart %s %gpt is off the type scale", c.role, c.chart)
		}
	}

	ty := svggen.DefaultTypography()
	for _, c := range []struct {
		role      string
		got, want float64
	}{
		{"SizeTitle", ty.SizeTitle, svggen.ChartTitlePt},
		{"SizeSubtitle", ty.SizeSubtitle, svggen.ChartSubtitlePt},
		{"SizeHeading", ty.SizeHeading, svggen.ChartHeadingPt},
		{"SizeBody", ty.SizeBody, svggen.ChartBodyPt},
		{"SizeSmall", ty.SizeSmall, svggen.ChartLabelPt},
		{"SizeCaption", ty.SizeCaption, svggen.ChartCaptionPt},
	} {
		if c.got != c.want {
			t.Errorf("DefaultTypography().%s = %g, want the chart role %g", c.role, c.got, c.want)
		}
	}

	// A small canvas clamps the title to the subhead step, not below it: the
	// chart title beside a 14pt card title is never 13pt.
	small := svggen.DefaultTypography().ScaleForDimensions(400, 300)
	if small.SizeTitle < tokens.TypeScaleSubheadPt {
		t.Errorf("scaled chart title = %gpt on a small canvas, below the %gpt subhead step", small.SizeTitle, tokens.TypeScaleSubheadPt)
	}
}
