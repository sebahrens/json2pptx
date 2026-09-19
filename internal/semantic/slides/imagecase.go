package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Image-case slides (go-slide-creator-q31s).
//
// A customer story or a product screenshot beside the words about it — the case
// study slide every proposal ends with — had no DeckSpec kind, so an agent that
// wanted one dropped to raw_json2pptx. The payload here is the picture, the
// narrative beside it, and the numbers the story is claiming.

const (
	// imageCase* mirror the image-text-split pattern's own budgets.
	imageCaseEyebrowMax     = 30
	imageCaseHeadingMax     = 80
	imageCaseBodyMax        = 300
	imageCaseBulletMax      = 140
	imageCaseMaxBullets     = 5
	imageCaseCaptionMax     = 120
	imageCaseLabelMax       = 40
	imageCaseMaxMetrics     = 3
	imageCaseMetricValueMax = 10
	imageCaseMetricLabelMax = 40
)

// imageCaseImage is the pattern's image reference.
type imageCaseImage struct {
	Path string `json:"path,omitempty"`
	URL  string `json:"url,omitempty"`
	Alt  string `json:"alt,omitempty"`
}

// imageCaseMetric is one result figure beside the story.
type imageCaseMetric struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// imageCaseValues is the image-text-split pattern's values object.
type imageCaseValues struct {
	Image      *imageCaseImage   `json:"image,omitempty"`
	ImageLabel string            `json:"image_label,omitempty"`
	Caption    string            `json:"caption,omitempty"`
	Eyebrow    string            `json:"eyebrow,omitempty"`
	Heading    string            `json:"heading,omitempty"`
	Body       string            `json:"body,omitempty"`
	Bullets    []string          `json:"bullets,omitempty"`
	Metrics    []imageCaseMetric `json:"metrics,omitempty"`
}

// imageCaseOverrides carries which side the picture sits on.
type imageCaseOverrides struct {
	ImageSide string `json:"image_side,omitempty"`
}

// CompileImageCase compiles a case-study payload onto the image-text-split
// pattern, falling back to a content slide when the payload does not fit it.
func CompileImageCase(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	values := imageCaseFrom(in.Body)
	if ImageCaseOverBudget(in.Body) != "" {
		return compileImageCaseFallback(in, values)
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal image-text-split values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "image-text-split", Values: encoded}
	if side := imageCaseSide(in.Body); side != "" {
		overrides, oErr := json.Marshal(imageCaseOverrides{ImageSide: side})
		if oErr != nil {
			return nil, nil, fmt.Errorf("marshal image-text-split overrides: %w", oErr)
		}
		slide.Pattern.Overrides = overrides
	}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values",
		SemanticPath: in.semSlide() + ".body",
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileImageCaseFallback keeps the whole story as text. The picture cannot
// come with it, so the caption or alt text stands in for it rather than the
// slide pretending there was never an image.
func compileImageCaseFallback(in Input, v imageCaseValues) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	links := titleLink(slide, in)

	var paras []string
	if v.Eyebrow != "" {
		paras = append(paras, v.Eyebrow)
	}
	if v.Heading != "" {
		paras = append(paras, v.Heading)
	}
	if v.Body != "" {
		paras = append(paras, v.Body)
	}
	if len(paras) > 0 {
		idx := appendContent(slide, textContent("body", strings.Join(paras, "\n")))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".body",
		})
	}

	bullets := append([]string{}, v.Bullets...)
	for _, m := range v.Metrics {
		bullets = append(bullets, m.Value+" — "+m.Label)
	}
	if caption := firstNonEmpty(v.Caption, imageCaseAlt(v)); caption != "" {
		bullets = append(bullets, "Image: "+caption)
	}
	if len(bullets) > 0 {
		placeholder := "body"
		if len(paras) > 0 {
			placeholder = "body_2"
		}
		idx := appendContent(slide, bulletsContent(placeholder, bullets))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + ".bullets",
		})
	}
	if len(slide.Content) == 0 {
		return CompileFallback(in)
	}
	if len(paras) > 0 && len(bullets) > 0 {
		slide.SlideType = "two-column"
		slide.LayoutID = "two-column"
	}
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// imageCaseAlt returns the image's alt text, for the fallback's stand-in line.
func imageCaseAlt(v imageCaseValues) string {
	if v.Image == nil {
		return v.ImageLabel
	}
	return firstNonEmpty(v.Image.Alt, v.ImageLabel)
}

// imageCaseFrom resolves the payload into the pattern's fields.
func imageCaseFrom(body map[string]any) imageCaseValues {
	v := imageCaseValues{
		ImageLabel: firstNonEmpty(strField(body, "image_label"), strField(body, "placeholder")),
		Caption:    firstNonEmpty(strField(body, "caption"), strField(body, "image_caption")),
		Eyebrow:    firstNonEmpty(strField(body, "eyebrow"), strField(body, "kicker"), strField(body, "label")),
		Heading:    firstNonEmpty(strField(body, "heading"), strField(body, "headline"), strField(body, "subtitle")),
		Body:       firstNonEmpty(strField(body, "body"), strField(body, "text"), strField(body, "story"), strField(body, "description")),
		Bullets:    imageCaseBullets(body),
		Metrics:    imageCaseMetrics(body),
	}
	// No label is invented for a missing picture: the pattern's own dashed
	// placeholder already says what it is, and filling the box with the caption
	// printed the caption twice — once inside the placeholder and once in the
	// italic line beneath it.
	v.Image = imageCaseImageFrom(body)
	return v
}

// imageCaseImageFrom reads the picture: a path or url string, or an object.
func imageCaseImageFrom(body map[string]any) *imageCaseImage {
	for _, key := range []string{"image", "photo", "screenshot"} {
		switch t := body[key].(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
					return &imageCaseImage{URL: s}
				}
				return &imageCaseImage{Path: s}
			}
		case map[string]any:
			img := &imageCaseImage{
				Path: strField(t, "path"),
				URL:  strField(t, "url"),
				Alt:  strField(t, "alt"),
			}
			if img.Path != "" || img.URL != "" {
				return img
			}
		}
	}
	return nil
}

// imageCaseBullets reads the supporting points.
func imageCaseBullets(body map[string]any) []string {
	raw, ok := firstList(body, "bullets", "points", "highlights")
	if !ok {
		return nil
	}
	var out []string
	for _, e := range raw {
		if s, isString := e.(string); isString {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

// imageCaseMetrics reads the result figures the story claims.
func imageCaseMetrics(body map[string]any) []imageCaseMetric {
	raw, ok := firstList(body, "metrics", "results", "outcomes")
	if !ok {
		return nil
	}
	var out []imageCaseMetric
	for _, e := range raw {
		m, isObject := e.(map[string]any)
		if !isObject {
			continue
		}
		value := firstNonEmpty(strField(m, "value"), strField(m, "number"), strField(m, "stat"))
		label := firstNonEmpty(strField(m, "label"), strField(m, "caption"), strField(m, "name"))
		if value == "" || label == "" {
			// A figure with no words, or words with no figure, is half a claim.
			continue
		}
		out = append(out, imageCaseMetric{Value: value, Label: label})
	}
	return out
}

// imageCaseSide reads which side the picture sits on.
func imageCaseSide(body map[string]any) string {
	side := strings.ToLower(strings.TrimSpace(firstNonEmpty(strField(body, "image_side"), strField(body, "side"))))
	if side == "left" || side == "right" {
		return side
	}
	return ""
}

// ImageCaseOverBudget explains why a case study cannot take the split, or ""
// when it fits.
func ImageCaseOverBudget(body map[string]any) string {
	v := imageCaseFrom(body)
	if v.Body == "" && len(v.Bullets) == 0 {
		// The pattern refuses this too: an image with no narrative is a plain
		// image slide, not a case study.
		if v.Image != nil || v.ImageLabel != "" {
			return "has a picture but nothing said about it; give it a body or at least one bullet"
		}
		return ""
	}
	for _, f := range []struct {
		name  string
		value string
		max   int
	}{
		{"eyebrow", v.Eyebrow, imageCaseEyebrowMax},
		{"heading", v.Heading, imageCaseHeadingMax},
		{"body", v.Body, imageCaseBodyMax},
		{"caption", v.Caption, imageCaseCaptionMax},
		{"image_label", v.ImageLabel, imageCaseLabelMax},
	} {
		if runeLen(f.value) > f.max {
			return fmt.Sprintf("%s is %d characters; the column holds %d", f.name, runeLen(f.value), f.max)
		}
	}
	if len(v.Bullets) > imageCaseMaxBullets {
		return fmt.Sprintf("has %d bullets; the column holds %d", len(v.Bullets), imageCaseMaxBullets)
	}
	for i, b := range v.Bullets {
		if runeLen(b) > imageCaseBulletMax {
			return fmt.Sprintf("bullet %d is %d characters; the column holds %d", i+1, runeLen(b), imageCaseBulletMax)
		}
	}
	if len(v.Metrics) > imageCaseMaxMetrics {
		return fmt.Sprintf("has %d result metrics; the strip holds %d (use kpi_snapshot for more figures)", len(v.Metrics), imageCaseMaxMetrics)
	}
	for i, m := range v.Metrics {
		switch {
		case runeLen(m.Value) > imageCaseMetricValueMax:
			return fmt.Sprintf("metric %d's value is %d characters; the strip holds %d", i+1, runeLen(m.Value), imageCaseMetricValueMax)
		case runeLen(m.Label) > imageCaseMetricLabelMax:
			return fmt.Sprintf("metric %d's label is %d characters; the strip holds %d", i+1, runeLen(m.Label), imageCaseMetricLabelMax)
		}
	}
	return ""
}

// ImageCasePattern returns the pattern a case-study payload compiles to, or ""
// when it degrades to a content slide.
func ImageCasePattern(body map[string]any) string {
	v := imageCaseFrom(body)
	if (v.Body != "" || len(v.Bullets) > 0) && ImageCaseOverBudget(body) == "" {
		return "image-text-split"
	}
	return ""
}

// UsableImageCaseNarrative reports whether the slide says anything about the
// picture, so validation fails fast on a case study with no case.
func UsableImageCaseNarrative(body map[string]any) int {
	v := imageCaseFrom(body)
	if v.Body == "" && len(v.Bullets) == 0 {
		return 0
	}
	return 1
}
