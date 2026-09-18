package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// image-text-split pattern — photo / case-study image beside a text column
// (eyebrow, heading, body, bullets) with an optional row of 1-3 result
// metrics.
// ---------------------------------------------------------------------------
//
// Layout (image_side = left):
//
//   ┌──────────────┐  CASE STUDY
//   │              │  Heading
//   │    image     │  Body paragraph …
//   │  (cover-fit) │  • bullet
//   │              │  • bullet
//   └──────────────┘  ▔▔▔▔▔▔  ▔▔▔▔▔▔  ▔▔▔▔▔▔
//   caption (italic)  $12M    -30%    4 wks   ← result metrics
//
// A real image (path or url) renders as a shape_grid image cell, which the
// generator cover-crops to the frame (no distortion). Without an image the
// pattern draws a dashed wireframe placeholder labelled with image_label.
// The grid is content-sized: its height is the larger of the text column's
// measured height and a ~4:3 image frame, capped at the content area and
// pinned in points so the pattern vertical_align default centres it.

func init() {
	Default().Register(&imageTextSplit{})
}

type imageTextSplit struct{}

// Pattern budgets.
const (
	itsEyebrowMax  = 30
	itsHeadingMax  = 80
	itsBodyMax     = 300
	itsBulletMax   = 140
	itsMaxBullets  = 5
	itsCaptionMax  = 120
	itsLabelMax    = 40
	itsMaxMetrics  = 3
	itsMetricValue = 10
	itsMetricLabel = 40

	itsColGapPt      = 20.0
	itsRowGapPt      = 10.0
	itsImageAspect   = 0.75 // preferred frame height / width (4:3)
	itsMinFillPct    = 60.0 // never shorter than this share of the content height
	itsMetricValuePt = 24.0
	itsMetricLabelPt = 12.0
)

// ImageAssetRef names one image reference inside a pattern's values.
type ImageAssetRef struct {
	Field string                     // JSON path under values, e.g. "image"
	Image *jsonschema.GridImageInput // points into the decoded values
}

// ImageAssetPattern is implemented by patterns whose values reference image
// files or URLs. Hosts resolve those references (relative paths against the
// deck directory, URLs through the fetch cache) before expansion, exactly as
// they do for shape_grid image cells.
type ImageAssetPattern interface {
	ImageAssets(values any) []ImageAssetRef
}

func (p *imageTextSplit) Name() string { return "image-text-split" }
func (p *imageTextSplit) Description() string {
	return "Photo / case-study image beside a text column (eyebrow, heading, body, up to 5 bullets) with an optional row of 1-3 result metrics; cover-cropped image or dashed placeholder"
}
func (p *imageTextSplit) UseWhen() string {
	return "One image (photo, screenshot, site visit, product shot) paired with a short narrative — case study, customer story, location or product spotlight; prefer chart-insights-split when the visual is a data chart, team-bios for several people, agenda-with-images for an agenda with thumbnails"
}
func (p *imageTextSplit) NotWhen() string {
	return "The visual is a chart (use chart-insights-split), there are several people (use team-bios) or several images (use card-grid / agenda-with-images), or the slide is text-only (use exec-summary or card-grid)"
}
func (p *imageTextSplit) Version() int      { return 1 }
func (p *imageTextSplit) CellsHint() string { return "1 image + 1 text column (+ 1-3 metrics)" }
func (p *imageTextSplit) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"evidence", "open"},
		PairsWith:     []string{"kpi-3up", "pull-quote", "table-highlight"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}

func (p *imageTextSplit) SupportsInlineMarkdown() bool { return true }

func (p *imageTextSplit) ExemplarValues() any {
	return &ImageTextSplitValues{
		ImageLabel: "Photo: distribution centre floor",
		Eyebrow:    "Case study",
		Heading:    "Regional retailer cut stock-outs by a third in two quarters",
		Body:       "A mid-size grocery chain rebuilt replenishment around store-level demand signals instead of weekly averages.",
		Bullets: []string{
			"Daily forecasts for 18,000 SKUs across 140 stores",
			"Automated orders for the top 60% of volume",
			"Store teams freed from manual counts",
		},
		Metrics: []ImageTextSplitMetric{
			{Value: "-34%", Label: "stock-outs"},
			{Value: "$12M", Label: "working capital released"},
			{Value: "6 mo", Label: "to full rollout"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ImageTextSplitMetric is a small result figure under the text column.
type ImageTextSplitMetric struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ImageTextSplitValues holds the image reference and the text column.
type ImageTextSplitValues struct {
	Image      *jsonschema.GridImageInput `json:"image,omitempty"`       // {path | url, alt}
	ImageLabel string                     `json:"image_label,omitempty"` // placeholder label when no image is given
	Caption    string                     `json:"caption,omitempty"`     // italic line under the image
	Eyebrow    string                     `json:"eyebrow,omitempty"`     // small uppercase kicker, e.g. "Case study"
	Heading    string                     `json:"heading,omitempty"`
	Body       string                     `json:"body,omitempty"`
	Bullets    []string                   `json:"bullets,omitempty"`
	Metrics    []ImageTextSplitMetric     `json:"metrics,omitempty"`
}

// ImageTextSplitOverrides are the pattern-level overrides.
type ImageTextSplitOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	ImageSide      string  `json:"image_side,omitempty"`      // left (default) | right
	ImageWidthPct  float64 `json:"image_width_pct,omitempty"` // 30-60, default 45
	HeadingSize    float64 `json:"heading_size,omitempty"`    // default 20
	BodySize       float64 `json:"body_size,omitempty"`       // default 14
}

func (p *imageTextSplit) NewValues() any       { return &ImageTextSplitValues{} }
func (p *imageTextSplit) NewOverrides() any    { return &ImageTextSplitOverrides{} }
func (p *imageTextSplit) NewCellOverride() any { return nil }

// ImageAssets exposes values.image so hosts resolve its path / url.
func (p *imageTextSplit) ImageAssets(values any) []ImageAssetRef {
	v, ok := values.(*ImageTextSplitValues)
	if !ok || v == nil || v.Image == nil {
		return nil
	}
	return []ImageAssetRef{{Field: "image", Image: v.Image}}
}

func (p *imageTextSplit) Schema() *Schema {
	imageSchema := ObjectSchema(map[string]*Schema{
		"path": StringSchema(0).WithDescription("Local .png / .jpg file (relative paths resolve against the deck JSON directory)"),
		"url":  StringSchema(0).WithDescription("HTTPS image URL (downloaded and cached)"),
		"alt":  StringSchema(200).WithDescription("Alt text (defaults to the caption or heading)"),
	}, nil).WithAdditionalProperties(false).WithDescription("The picture; omit to render a dashed placeholder labelled with image_label")

	metric := ObjectSchema(map[string]*Schema{
		"value": StringSchema(itsMetricValue).WithDescription("Short figure, e.g. \"-34%\" or \"$12M\""),
		"label": StringSchema(itsMetricLabel).WithDescription("What the figure measures"),
	}, []string{"value", "label"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"image":       imageSchema,
		"image_label": StringSchema(itsLabelMax).WithDescription("Placeholder label when no image is given, e.g. \"Photo: plant floor\""),
		"caption":     StringSchema(itsCaptionMax).WithDescription("Optional italic caption / source under the image"),
		"eyebrow":     StringSchema(itsEyebrowMax).WithDescription("Small uppercase kicker above the heading, e.g. \"Case study\""),
		"heading":     StringSchema(itsHeadingMax).WithDescription("Headline of the text column"),
		"body":        StringSchema(itsBodyMax).WithDescription("Short paragraph (≤300 chars)"),
		"bullets":     ArraySchema(StringSchema(itsBulletMax), 0, itsMaxBullets).WithDescription("0-5 bullets (≤140 chars each)"),
		"metrics":     ArraySchema(metric, 0, itsMaxMetrics).WithDescription("0-3 result figures shown under the text"),
	}, nil).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":          StringSchema(0).WithDescription("Accent for the eyebrow, metric values and rules (default accent1)").WithDefault("accent1"),
		"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"image_side":      EnumSchema("left", "right").WithDescription("Which side the image sits on (default left)").WithDefault("left"),
		"image_width_pct": NumberSchema(30, 60).WithDescription("Image column width as a percentage of the grid (default 45)").WithDefault(45),
		"heading_size":    NumberSchema(14, 40).WithDescription("Heading font size in points (default 20)"),
		"body_size":       NumberSchema(12, 24).WithDescription("Body / bullet font size in points (default 14)"),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":    valuesSchema,
		"overrides": overridesSchema,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Image beside a text column (eyebrow, heading, body, bullets) with optional result metrics")
}

func (p *imageTextSplit) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*ImageTextSplitValues)
	if !ok || v == nil {
		return fmt.Errorf("image-text-split: values must be *ImageTextSplitValues, got %T", values)
	}
	const name = "image-text-split"
	var errs []error

	if overrides != nil {
		ovr, ok := overrides.(*ImageTextSplitOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("image-text-split: overrides must be *ImageTextSplitOverrides, got %T", overrides))
		} else {
			errs = append(errs, itsValidateOverrides(ovr)...)
		}
	}
	if v.Image != nil {
		if strings.TrimSpace(v.Image.Path) == "" && strings.TrimSpace(v.Image.URL) == "" {
			errs = append(errs, newValidationError(name, "image", ErrCodeRequired,
				"image-text-split: image needs a path or url (omit image to render a placeholder)", ProvideValueFix("image.path")))
		}
		if v.Image.Overlay != nil || v.Image.Text != nil {
			errs = append(errs, errUnknownKey(name, "image", "overlay/text", "path, url, alt"))
		}
	}
	if strings.TrimSpace(v.Body) == "" && len(v.Bullets) == 0 {
		errs = append(errs, newValidationError(name, "body", ErrCodeRequired,
			"image-text-split: provide body text or at least one bullet (hint: an image with no narrative is a plain image slide)", ProvideValueFix("body")))
	}
	for _, f := range []struct {
		path string
		val  string
		max  int
	}{
		{"image_label", v.ImageLabel, itsLabelMax}, {"caption", v.Caption, itsCaptionMax},
		{"eyebrow", v.Eyebrow, itsEyebrowMax}, {"heading", v.Heading, itsHeadingMax}, {"body", v.Body, itsBodyMax},
	} {
		if len(f.val) > f.max {
			errs = append(errs, errMaxLength(name, f.path, f.max, len(f.val)))
		}
	}
	if len(v.Bullets) > itsMaxBullets {
		errs = append(errs, errMaxItems(name, "bullets", itsMaxBullets, len(v.Bullets), "(hint: move detail to a second slide or use card-grid)"))
	}
	for i, b := range v.Bullets {
		path := fmt.Sprintf("bullets[%d]", i)
		if strings.TrimSpace(b) == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(b) > itsBulletMax {
			errs = append(errs, errMaxLength(name, path, itsBulletMax, len(b)))
		}
	}
	errs = append(errs, itsValidateMetrics(v.Metrics)...)
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"image-text-split: cell_overrides are not supported (use overrides)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

func itsValidateOverrides(ovr *ImageTextSplitOverrides) []error {
	const name = "image-text-split"
	var errs []error
	if ovr.ImageSide != "" && ovr.ImageSide != "left" && ovr.ImageSide != "right" {
		errs = append(errs, newValidationError(name, "overrides.image_side", ErrCodeUnknownEnum,
			fmt.Sprintf("image-text-split: overrides.image_side must be left or right, got %q", ovr.ImageSide), UseOneOfFix("overrides.image_side", []string{"left", "right"})))
	}
	if ovr.ImageWidthPct != 0 && (ovr.ImageWidthPct < 30 || ovr.ImageWidthPct > 60) {
		errs = append(errs, errOutOfRange(name, "overrides.image_width_pct", 30, 60, int(ovr.ImageWidthPct)))
	}
	return errs
}

func itsValidateMetrics(metrics []ImageTextSplitMetric) []error {
	const name = "image-text-split"
	var errs []error
	if len(metrics) > itsMaxMetrics {
		errs = append(errs, errMaxItems(name, "metrics", itsMaxMetrics, len(metrics), "(hint: use kpi-Nup for more figures)"))
	}
	for i, m := range metrics {
		vp, lp := fmt.Sprintf("metrics[%d].value", i), fmt.Sprintf("metrics[%d].label", i)
		if strings.TrimSpace(m.Value) == "" {
			errs = append(errs, errRequired(name, vp))
		} else if len(m.Value) > itsMetricValue {
			errs = append(errs, errMaxLength(name, vp, itsMetricValue, len(m.Value)))
		}
		if strings.TrimSpace(m.Label) == "" {
			errs = append(errs, errRequired(name, lp))
		} else if len(m.Label) > itsMetricLabel {
			errs = append(errs, errMaxLength(name, lp, itsMetricLabel, len(m.Label)))
		}
	}
	return errs
}

// itsLayout carries the resolved geometry of one expansion.
type itsLayout struct {
	imgPct, textPct float64
	imgW, textW     float64
	headingSize     float64
	bodySize        float64
	textPt          float64 // text block (eyebrow + heading + body + bullets)
	metricsPt       float64 // metric row (0 = none)
	captionPt       float64 // caption row under the image (0 = none)
	heightPt        float64 // whole grid
}

func (p *imageTextSplit) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*ImageTextSplitValues)
	if !ok {
		return nil, fmt.Errorf("image-text-split: values must be *ImageTextSplitValues, got %T", values)
	}
	ovr := &ImageTextSplitOverrides{}
	if overrides != nil {
		if ovr, ok = overrides.(*ImageTextSplitOverrides); !ok {
			return nil, fmt.Errorf("image-text-split: overrides must be *ImageTextSplitOverrides, got %T", overrides)
		}
	}
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	areaW, areaH := sizingAreaPt(ctx)
	lay := itsMeasure(ctx, v, ovr, areaW, areaH)

	imageCell := itsImageColumn(ctx, v, lay)
	textCell := itsTextColumn(ctx, v, lay, accent)
	cells := []*jsonschema.GridCellInput{imageCell, textCell}
	cols := []float64{lay.imgPct, lay.textPct}
	if ovr.ImageSide == "right" {
		cells = []*jsonschema.GridCellInput{textCell, imageCell}
		cols = []float64{lay.textPct, lay.imgPct}
	}
	colsJSON, _ := json.Marshal(cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  itsColGapPt,
		Rows:    []jsonschema.GridRowInput{{MinHeight: lay.heightPt, MaxHeight: lay.heightPt, Cells: cells}},
	}
	return grid, nil
}

// itsMeasure resolves column widths, the type scale and the grid height.
func itsMeasure(ctx ExpandContext, v *ImageTextSplitValues, ovr *ImageTextSplitOverrides, areaW, areaH float64) itsLayout {
	lay := itsLayout{imgPct: clampPct(ovr.ImageWidthPct, 45, 30, 60)}
	lay.textPct = 100 - lay.imgPct
	lay.imgW = (areaW - itsColGapPt) * lay.imgPct / 100
	lay.textW = (areaW - itsColGapPt) * lay.textPct / 100
	if len(v.Metrics) > 0 {
		lay.metricsPt = itsMetricValuePt*sizingLineSpacing + 2*itsMetricLabelPt*sizingLineSpacing + 2*sizingInsetTBPt + sizingSafetyPt
	}
	if strings.TrimSpace(v.Caption) != "" {
		lay.captionPt = sizedBlockHeightPt(ctx, []sizedPara{{text: v.Caption, sizePt: 12}}, lay.imgW)
	}

	steps := [][2]float64{{20, 14}, {18, 13}, {16, 12}}
	if ovr.HeadingSize > 0 || ovr.BodySize > 0 {
		steps = [][2]float64{{ResolveSize(ovr.HeadingSize, 20), ResolveSize(ovr.BodySize, 14)}}
	}
	var column float64
	for _, st := range steps {
		lay.headingSize, lay.bodySize = st[0], st[1]
		lay.textPt = sizedBlockHeightPt(ctx, itsTextParas(v, st[0], st[1]), lay.textW)
		column = lay.textPt
		if lay.metricsPt > 0 {
			column += itsRowGapPt + lay.metricsPt
		}
		if column <= areaH {
			break
		}
	}
	imageColumn := lay.imgW * itsImageAspect
	if lay.captionPt > 0 {
		imageColumn += itsRowGapPt + lay.captionPt
	}
	lay.heightPt = math.Min(math.Max(math.Max(column, imageColumn), areaH*itsMinFillPct/100), areaH)
	return lay
}

// itsTextParas returns the measured paragraphs of the text column.
func itsTextParas(v *ImageTextSplitValues, headingSize, bodySize float64) []sizedPara {
	var paras []sizedPara
	if v.Eyebrow != "" {
		paras = append(paras, sizedPara{text: strings.ToUpper(v.Eyebrow), sizePt: 12, bold: true, spaceAfterPt: 4})
	}
	if v.Heading != "" {
		paras = append(paras, sizedPara{text: v.Heading, sizePt: headingSize, bold: true, spaceAfterPt: 10})
	}
	if v.Body != "" {
		paras = append(paras, sizedPara{text: v.Body, sizePt: bodySize, spaceAfterPt: 8})
	}
	for _, b := range v.Bullets {
		paras = append(paras, sizedPara{text: "• " + b, sizePt: bodySize, spaceAfterPt: 6})
	}
	return paras
}

// itsImageColumn builds the image (or placeholder) cell, with the caption in
// a nested row underneath when present.
func itsImageColumn(ctx ExpandContext, v *ImageTextSplitValues, lay itsLayout) *jsonschema.GridCellInput {
	var picture *jsonschema.GridCellInput
	if v.Image != nil && (v.Image.Path != "" || v.Image.URL != "") {
		img := &jsonschema.GridImageInput{Path: v.Image.Path, URL: v.Image.URL, Alt: v.Image.Alt}
		if strings.TrimSpace(img.Alt) == "" {
			img.Alt = firstNonEmpty(v.Caption, v.Heading, v.ImageLabel, "Image")
		}
		picture = &jsonschema.GridCellInput{Image: img}
	} else {
		label := firstNonEmpty(v.ImageLabel, "Add an image (values.image.path)")
		textJSON, _ := json.Marshal(chartInsightsText{
			Paragraphs: []chartInsightsParagraph{
				{Content: "Image placeholder", Size: 14, Bold: true, Color: inkOnLight(ctx, "dk2", 4.5), Align: "ctr", SpaceAfter: 4},
				{Content: label, Size: 12, Color: "dk1", Align: "ctr"},
			},
			Align:         "ctr",
			VerticalAlign: "ctr",
		})
		picture = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			// A faint tint under the 20% "filled" threshold plus a dashed
			// border: reads as a wireframe slot, not an empty content block.
			Fill: fillTone{Color: "lt2", Alpha: 15}.fillJSON(),
			Line: json.RawMessage(`{"color":"dk2","width":1,"dash":"dash"}`),
			Text: textJSON,
		}}
	}
	if lay.captionPt <= 0 {
		return picture
	}
	capJSON, _ := json.Marshal(chartInsightsText{
		Paragraphs:    []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(v.Caption), Size: 12, Italic: true, Color: "dk1", Align: "l"}},
		Align:         "l",
		VerticalAlign: "t",
	})
	caption := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: capJSON}}
	capPct := pctOf(lay.captionPt, lay.heightPt-itsRowGapPt)
	return &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		RowGap:  itsRowGapPt,
		Rows: []jsonschema.GridRowInput{
			{Height: 100 - capPct, Cells: []*jsonschema.GridCellInput{picture}},
			{Height: capPct, Cells: []*jsonschema.GridCellInput{caption}},
		},
	}}
}

// itsTextColumn builds the text cell, stacking the metric row underneath
// (bottom-anchored) when metrics are present.
func itsTextColumn(ctx ExpandContext, v *ImageTextSplitValues, lay itsLayout, accent string) *jsonschema.GridCellInput {
	var paras []chartInsightsParagraph
	for _, sp := range itsTextParas(v, lay.headingSize, lay.bodySize) {
		para := chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(sp.text), Size: sp.sizePt, Bold: sp.bold, Color: "dk1", Align: "l", SpaceAfter: sp.spaceAfterPt}
		switch {
		case v.Eyebrow != "" && len(paras) == 0:
			para.Color = inkOnLight(ctx, accent, 4.5)
		case sp.sizePt == lay.headingSize && sp.bold:
			para.Color = inkOnLight(ctx, "dk2", 4.5)
		}
		paras = append(paras, para)
	}
	if n := len(paras); n > 0 {
		paras[n-1].SpaceAfter = 0
	}
	textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: "l", VerticalAlign: "t"})
	text := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: textJSON}}
	if lay.metricsPt <= 0 {
		return text
	}

	valueInk := inkOnLight(ctx, accent, 3.0)
	metricCells := make([]*jsonschema.GridCellInput, 0, len(v.Metrics))
	for _, m := range v.Metrics {
		mJSON, _ := json.Marshal(chartInsightsText{
			Paragraphs: []chartInsightsParagraph{
				{Content: m.Value, Size: itsMetricValuePt, Bold: true, Color: valueInk, Align: "l"},
				{Content: pptx.ConvertMarkdownEmphasis(m.Label), Size: itsMetricLabelPt, Color: "dk1", Align: "l"},
			},
			Align:         "l",
			VerticalAlign: "t",
		})
		metricCells = append(metricCells, &jsonschema.GridCellInput{
			Shape:     &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: mJSON},
			AccentBar: &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: 2},
		})
	}
	metricsPct := pctOf(lay.metricsPt, lay.heightPt-itsRowGapPt)
	return &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`1`),
		RowGap:  itsRowGapPt,
		Rows: []jsonschema.GridRowInput{
			{Height: 100 - metricsPct, Cells: []*jsonschema.GridCellInput{text}},
			{Height: metricsPct, Cells: []*jsonschema.GridCellInput{{Grid: &jsonschema.ShapeGridInput{
				Columns: json.RawMessage(fmt.Sprintf("%d", len(metricCells))),
				ColGap:  12,
				Rows:    []jsonschema.GridRowInput{{Cells: metricCells}},
			}}}},
		},
	}}
}
