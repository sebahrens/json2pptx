package patterns

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func init() {
	image := visualObject(map[string]*Schema{"path": StringSchema(1000), "url": StringSchema(1000), "alt": StringSchema(300)}, nil...)
	Default().Register(&consultingPattern{name: "image-hero", description: "Cover-cropped image with a black 85% scrim band and measured white headline; top, bottom or left placement", valuesSchema: visualObject(map[string]*Schema{"image": image, "headline": visualString(160), "takeaway": StringSchema(200), "source": StringSchema(200), "eyebrow": StringSchema(60), "band": EnumSchema("top", "bottom", "left")}, "image", "headline"), example: map[string]any{"image": map[string]any{"path": "hero.jpg", "alt": "Project site"}, "headline": "A platform for the next stage", "eyebrow": "Strategy 2027", "band": "bottom"}, check: checkConsultingHero, draw: drawConsultingHero})
}
func checkConsultingHero(v map[string]any) error {
	raw, _ := json.Marshal(v["image"])
	var image jsonschema.GridImageInput
	if err := json.Unmarshal(raw, &image); err != nil {
		return err
	}
	if (image.Path == "") == (image.URL == "") {
		return fmt.Errorf("image-hero requires exactly one image.path or image.url")
	}
	return nil
}
func (p *consultingPattern) ImageAssets(values any) []ImageAssetRef {
	if p.name != "image-hero" {
		return nil
	}
	v, ok := values.(*map[string]any)
	if !ok || v == nil {
		return nil
	}
	raw, _ := json.Marshal((*v)["image"])
	image := &jsonschema.GridImageInput{}
	if json.Unmarshal(raw, image) != nil {
		return nil
	}
	(*v)["image"] = image
	return []ImageAssetRef{{Field: "image", Image: image}}
}
func drawConsultingHero(c *consultingCanvas, v map[string]any) error {
	raw, _ := json.Marshal(v["image"])
	image := &jsonschema.GridImageInput{}
	if err := json.Unmarshal(raw, image); err != nil {
		return err
	}
	image.Fit = "cover"
	c.image = image
	x, y, w, h := 0.0, 0.0, c.w, c.h*.45
	switch visualStr(v, "band") {
	case "left":
		w = c.w * .5
		h = c.h
	case "top":
	default:
		y = c.h - h
	}
	// Black at 85% opacity remains at least 15:1 against white text even
	// over a white image pixel. The contrast proof does not assume image content.
	fill, _ := json.Marshal(jsonschema.ShapeFillInput{Color: "#000000", Alpha: 85})
	if err := c.shape(x, y, w, h, "rect", "none", nil); err != nil {
		return err
	}
	c.layers[len(c.layers)-1].Shape.Fill = fill
	lines := []visualParagraph{}
	if eyebrow := visualStr(v, "eyebrow"); eyebrow != "" {
		lines = append(lines, visualParagraph{Content: eyebrow, Size: scaleBodyPt, Bold: true, Color: "#FFFFFF"})
	}
	lines = append(lines, visualParagraph{Content: visualStr(v, "headline"), Size: scaleDisplayPt, Bold: true, Color: "#FFFFFF"})
	if takeaway := visualStr(v, "takeaway"); takeaway != "" {
		lines = append(lines, visualParagraph{Content: takeaway, Size: scaleSubheadPt, Color: "#FFFFFF"})
	}
	if source := visualStr(v, "source"); source != "" {
		lines = append(lines, visualParagraph{Content: source, Size: scaleCaptionPt, Color: "#FFFFFF"})
	}
	return c.text(x, y, w, h, lines)
}
