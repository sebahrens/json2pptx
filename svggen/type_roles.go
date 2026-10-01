package svggen

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
const (
	ChartTitlePt    = 18.0 // chart title: the slide lead step
	ChartSubtitlePt = 14.0 // subtitle: the slide subhead step
	ChartHeadingPt  = 12.0 // legend text, section headings: the slide body step
	ChartBodyPt     = 12.0 // pie/donut labels, axis titles: the slide body step
	ChartLabelPt    = 10.0 // axis ticks, value labels: the slide caption step
	ChartCaptionPt  = 10.0 // diagram badges, footnotes: the slide caption step

	// ChartTitleMinPt is the smallest a scaled chart title gets: the slide
	// subhead (card title) step, so a chart title beside a 14pt card title
	// is not a point smaller.
	ChartTitleMinPt = 14.0
	// ChartTextMinPt floors subtitles, headings and body text: the slide's
	// 11pt dense-body step.
	ChartTextMinPt = 11.0
	// ChartLabelMinPt floors tick, value and caption labels: the slide's
	// 10pt caption step.
	ChartLabelMinPt = 10.0
)
