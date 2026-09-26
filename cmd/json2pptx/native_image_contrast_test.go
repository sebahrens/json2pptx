package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNativeImageContrastPreflight(t *testing.T) {
	frame := types.BoundingBox{Width: 100, Height: 100}
	layout := types.LayoutMetadata{ID: "slideLayout1", Placeholders: []types.PlaceholderInfo{
		{ID: "Title", Type: types.PlaceholderTitle, Bounds: frame, Index: 0, FontColor: "#FFFFFF"},
		{ID: "Picture", Type: types.PlaceholderImage, Bounds: frame, Index: 10},
	}}
	text := "Visible"
	disabled := false
	for _, tc := range []struct {
		name     string
		content  ContentInput
		image    ContentInput
		contrast *bool
		want     int
	}{
		{"typed local", ContentInput{Type: "text", TextValue: &text}, ContentInput{Type: "image", ImageValue: &ImageInput{Path: "source.png"}}, nil, 1},
		{"legacy local", ContentInput{Type: "text", Value: json.RawMessage(`"Visible"`)}, ContentInput{Type: "image", Value: json.RawMessage(`{"path":"source.png"}`)}, nil, 1},
		{"remote contain", ContentInput{Type: "text", TextValue: &text}, ContentInput{Type: "image", ImageValue: &ImageInput{URL: "https://example.invalid/source.png", Fit: "contain"}}, nil, 1},
		{"opt out", ContentInput{Type: "text", TextValue: &text}, ContentInput{Type: "image", ImageValue: &ImageInput{Path: "source.png"}}, &disabled, 0},
		{"empty text", ContentInput{Type: "text", Value: json.RawMessage(`"  "`)}, ContentInput{Type: "image", ImageValue: &ImageInput{Path: "source.png"}}, nil, 0},
		{"no source", ContentInput{Type: "text", TextValue: &text}, ContentInput{Type: "image", ImageValue: &ImageInput{}}, nil, 0},
		{"invalid fit", ContentInput{Type: "text", TextValue: &text}, ContentInput{Type: "image", ImageValue: &ImageInput{Path: "source.png", Fit: "stretch"}}, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.content.PlaceholderID, tc.image.PlaceholderID = "Title", "Picture"
			input := &PresentationInput{Slides: []SlideInput{{LayoutID: layout.ID, ContrastCheck: tc.contrast, Content: []ContentInput{tc.content, tc.image}}}}
			findings := collectNativeImageContrastFindings(input, []types.LayoutMetadata{layout})
			if len(findings) != tc.want {
				t.Fatalf("findings=%+v, want %d", findings, tc.want)
			}
			if len(findings) > 0 && (findings[0].Code != patterns.ErrCodeTextOverImageUnverified || findings[0].Action != "review") {
				t.Fatalf("must require pixel review, not automatic recoloring: %+v", findings)
			}
		})
	}
}

func TestNativeImageContrastPreflightDoesNotMeasureCanvasBehindImage(t *testing.T) {
	text := "Visible"
	frame := types.BoundingBox{Width: 100, Height: 100}
	layout := types.LayoutMetadata{ID: "slideLayout1", BackgroundHex: "#FFFFFF", Placeholders: []types.PlaceholderInfo{
		{ID: "Title", Index: 0, Type: types.PlaceholderTitle, FontColor: "#FFFFFF", Bounds: frame},
		{ID: "Picture", Index: 10, Type: types.PlaceholderImage, Bounds: frame},
	}}
	input := &PresentationInput{Slides: []SlideInput{{LayoutID: layout.ID, Content: []ContentInput{
		{PlaceholderID: "idx:0", Type: "text", TextValue: &text},
		{PlaceholderID: "idx:10", Type: "image", ImageValue: &ImageInput{Path: "source.png"}},
	}}}}
	if got := collectNativeImageContrastFindings(input, []types.LayoutMetadata{layout}); len(got) != 1 {
		t.Fatalf("index aliases missed image overlap: %+v", got)
	}
	if got := placeholderContrastPairs(input, []types.LayoutMetadata{layout}, nil); len(got) != 0 {
		t.Fatalf("cannot predict canvas contrast for image-dependent text: %+v", got)
	}
	layout.Placeholders[1].Bounds.X = 200
	if got := collectNativeImageContrastFindings(input, []types.LayoutMetadata{layout}); len(got) != 0 {
		t.Fatalf("separate text/image layout flagged: %+v", got)
	}
	if got := placeholderContrastPairs(input, []types.LayoutMetadata{layout}, nil); len(got) != 1 {
		t.Fatalf("separate canvas text should remain measurable: %+v", got)
	}
}

func TestNativeImageTextPopulatedLegacyAndTypedPrecedence(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{`{"type":"text","value":"Visible"}`, true},
		{`{"type":"text","text_value":" ","value":"Visible"}`, false},
		{`{"type":"text","value":123}`, false},
		{`{"type":"bullets","value":[" ","Visible"]}`, true},
		{`{"type":"bullets","bullets_value":[" "]}`, false},
		{`{"type":"body_and_bullets","value":{"body":"Visible","bullets":[]}}`, true},
		{`{"type":"body_and_lead","value":{"lead":"Visible","bullets":[]}}`, true},
		{`{"type":"bullet_groups","value":{"groups":[{"header":"Visible","bullets":[]}]}}`, true},
		{`{"type":"bullet_groups","bullet_groups_value":{"groups":[{"header":" ","bullets":[" "]}]}}`, false},
		{`{"type":"image","image_value":{"path":"photo.png"}}`, false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var content ContentInput
			if err := json.Unmarshal([]byte(tc.input), &content); err != nil {
				t.Fatal(err)
			}
			if got := nativeImageTextPopulated(&content); got != tc.want {
				t.Fatalf("populated=%v, want %v", got, tc.want)
			}
		})
	}
}
