package svggen

import "math"

// Chart text roles (go-slide-creator-vmdfm).
//
// Chart text sits on the same type scale as the slide around it, the scale
// json2pptx publishes in internal/tokens/typography.go: 28pt display, 18pt
// lead, 14pt subhead, 12pt body, 11pt dense body, 10pt caption. A chart title
// therefore never renders a point below the card titles beside it, and axis
// labels match the slide's captions. The root module's
// internal/tokens/chart_scale_test.go pins each role to its slide step.
//
// The reference sizes are for the 800x600pt reference canvas;
// ScaleForDimensions scales them with the canvas and then clamps them to the
// floors below, which are slide steps too.
//
// svggen is a separate module and cannot import the tokens package, so the
// steps are mirrored here; internal/tokens/chart_scale_test.go pins
// ChartTypeScaleStepsPt to tokens' steps one for one.
const (
	ChartStepDisplayPt   = 28.0 // display / slide title
	ChartStepLeadPt      = 18.0 // lead
	ChartStepSubheadPt   = 14.0 // subhead / card title
	ChartStepBodyPt      = 12.0 // body
	ChartStepDenseBodyPt = 11.0 // dense body (the body minimum)
	ChartStepCaptionPt   = 10.0 // caption
)

// ChartTypeScaleStepsPt lists the text steps above, ascending.
var ChartTypeScaleStepsPt = []float64{ChartStepCaptionPt, ChartStepDenseBodyPt, ChartStepBodyPt, ChartStepSubheadPt, ChartStepLeadPt, ChartStepDisplayPt}

const (
	ChartTitlePt    = ChartStepLeadPt    // chart title: the slide lead step
	ChartSubtitlePt = ChartStepSubheadPt // subtitle: the slide subhead step
	ChartHeadingPt  = ChartStepBodyPt    // legend text, section headings: the slide body step
	ChartBodyPt     = ChartStepBodyPt    // pie/donut labels, axis titles: the slide body step
	ChartLabelPt    = ChartStepCaptionPt // axis ticks, value labels: the slide caption step
	ChartCaptionPt  = ChartStepCaptionPt // diagram badges, footnotes: the slide caption step

	// ChartTitleMinPt is the smallest a scaled chart title gets: the slide
	// subhead (card title) step, so a chart title beside a 14pt card title
	// is not a point smaller.
	ChartTitleMinPt = ChartStepSubheadPt
	// ChartTextMinPt floors subtitles, headings and body text: the slide's
	// 11pt dense-body step.
	ChartTextMinPt = ChartStepDenseBodyPt
	// ChartLabelMinPt floors tick, value and caption labels: the slide's
	// 10pt caption step.
	ChartLabelMinPt = ChartStepCaptionPt
)

// Large-canvas caps (ScaleForDimensions). A canvas larger than the 800x600
// reference scales its text up, but each role stops at the next display step
// of the scale above its reference size — 10 -> 12, 12 -> 14, 14 -> 18,
// 18 -> 28 (the 11pt dense-body step is a floor, not a display step, so it is
// skipped). The cap is in canvas units: a large canvas placed on a slide is
// scaled down again, so the cap bounds the role's ratio to the others rather
// than the size a viewer reads.
const (
	ChartTitleMaxPt    = ChartStepDisplayPt
	ChartSubtitleMaxPt = ChartStepLeadPt
	ChartHeadingMaxPt  = ChartStepSubheadPt
	ChartBodyMaxPt     = ChartStepSubheadPt
	ChartLabelMaxPt    = ChartStepBodyPt
	ChartCaptionMaxPt  = ChartStepBodyPt
)

// OnChartTypeScale reports whether pt is one of the text steps.
func OnChartTypeScale(pt float64) bool {
	for _, step := range ChartTypeScaleStepsPt {
		if math.Abs(pt-step) < 1e-9 {
			return true
		}
	}
	return false
}
