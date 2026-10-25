package main

import (
	"context"
	"github.com/sebahrens/json2pptx/internal/types"
	"testing"
)

func TestFocalPointRawAuthoringContract(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{Content: []ContentInput{{Type: "image", ImageValue: &ImageInput{Path: "photo.png", FocalPoint: &types.ImageFocalPoint{X: -1, Y: .5}}}}}}}
	if errors := checkNativeImageFitEnums(input.Slides[0].Content, "/slides/0"); len(errors) != 1 || errors[0].Path != "/slides/0/content/0/image_value/focal_point" {
		t.Fatalf("wrong focal diagnostics: %+v", errors)
	}
	definition := buildInputSchema()
	defs := definition["$defs"].(map[string]any)
	focal := defs["ImageFocalPoint"].(map[string]any)
	if focal["additionalProperties"] != false {
		t.Fatal("focal point contract is open")
	}
	props := focal["properties"].(map[string]any)
	for _, key := range []string{"x", "y"} {
		p := props[key].(map[string]any)
		if p["type"] != "number" || p["minimum"] != 0 || p["maximum"] != 1 {
			t.Fatalf("coordinate %s lacks normalized bounds: %+v", key, p)
		}
	}
	for _, name := range []string{"GridImageInput", "ImageInput"} {
		p := defs[name].(map[string]any)["properties"].(map[string]any)["focal_point"].(map[string]any)
		if p["$ref"] != "#/$defs/ImageFocalPoint" {
			t.Fatalf("%s has divergent focal contract: %+v", name, p)
		}
	}
}

func TestInspectRejectsUnknownViewingMode(t *testing.T) {
	mc := cliMCPConfig("./templates", "")
	r, err := mc.handleInspectSlideImages(context.Background(), mcpRequestWithArgs(map[string]any{"slide_images": []any{map[string]any{"index": 0, "png_base64": "AA=="}}, "deck_metadata": map[string]any{"viewing_mode": "typo"}}))
	if err != nil || r == nil || !r.IsError {
		t.Fatalf("unknown viewing mode accepted: %+v %v", r, err)
	}
}
