package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
)

func TestImageFitSurvivesAllInputRoutes(t *testing.T) {
	for _, fit := range []string{"", "cover", "contain", "CONTAIN", "stretch"} {
		for _, route := range []string{"typed", "legacy", "json-content"} {
			t.Run(route+"/"+fit, func(t *testing.T) {
				value, _ := json.Marshal(ImageInput{Path: "source.png", Alt: "Required diagram", Fit: fit})
				var items []generator.ContentItem
				var err error
				if route == "json-content" {
					items, err = convertJSONContent([]JSONContentItem{{PlaceholderID: "image", Type: "image", Value: value}}, 1, "")
				} else {
					input := ContentInput{PlaceholderID: "image", Type: "image", Value: value}
					if route == "typed" {
						input.Value = nil
						input.ImageValue = &ImageInput{Path: "source.png", Alt: "Required diagram", Fit: fit}
					}
					items, err = convertPresentationContent([]ContentInput{input}, 1, "")
				}
				if fit == "stretch" {
					if err == nil || !strings.Contains(err.Error(), "image fit") {
						t.Fatalf("invalid fit accepted: %v", err)
					}
					return
				}
				if err != nil || len(items) != 1 {
					t.Fatalf("conversion failed: %v", err)
				}
				image := items[0].Value.(generator.ImageContent)
				if image.Fit != fit || image.Path != "source.png" || image.Alt != "Required diagram" {
					t.Fatalf("image source or fit lost: %+v", image)
				}
			})
		}
	}
}

func TestImageFitValidationAndSchemaParity(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		content := ContentInput{PlaceholderID: "image", Type: "image", ImageValue: &ImageInput{Path: "source.png", Fit: "stretch"}}
		field := "image_value"
		if legacy {
			content.ImageValue = nil
			content.Value = json.RawMessage(`{"path":"source.png","fit":"stretch"}`)
			field = "value"
		}
		errs := checkInputEnumValues(&PresentationInput{Slides: []SlideInput{{Content: []ContentInput{content}}}})
		if len(errs) != 1 || !strings.HasSuffix(errs[0].Path, "/"+field+"/fit") || errs[0].Code != "UNKNOWN_ENUM" {
			t.Fatalf("wrong fit diagnostic: %+v", errs)
		}
	}
	schema := buildInputSchema()
	defs := schema["$defs"].(map[string]any)
	properties := defs["ImageInput"].(map[string]any)["properties"].(map[string]any)
	fit := properties["fit"].(map[string]any)
	values := fit["enum"].([]string)
	if strings.Join(values, ",") != strings.Join(generator.ValidImageFits(), ",") {
		t.Fatalf("schema/runtime mismatch: %v", fit)
	}
}
