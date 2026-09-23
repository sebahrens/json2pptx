package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

func transparentGridSlide(background *BackgroundInput, fill, textColor string) SlideInput {
	return SlideInput{
		Background: background,
		ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{
			Shape: &ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fill),
				Text:     json.RawMessage(`{"content":"Caption","color":"` + textColor + `","size":12}`),
			},
		}}}}},
	}
}

func TestTransparentGridContrastPreflightUsesEffectiveBackground(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
	}
	tests := []struct {
		name             string
		slide            SlideInput
		layoutBackground string
		wantBackground   string
		wantReplacement  string
	}{
		{"white slide", transparentGridSlide(&BackgroundInput{Color: "lt1"}, `"none"`, "#FFFFFF"), "", "#FFFFFF", "#000000"},
		{"dark slide", transparentGridSlide(&BackgroundInput{Color: "dk1"}, `"none"`, "#000000"), "", "#000000", "#FFFFFF"},
		{"dark layout", transparentGridSlide(nil, `"none"`, "#000000"), "#000000", "#000000", "#FFFFFF"},
		{"photo unknown", transparentGridSlide(&BackgroundInput{Image: "photo.png"}, `"none"`, "#FFFFFF"), "#000000", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			layouts := []types.LayoutMetadata{{ID: "slideLayout1", BackgroundHex: tc.layoutBackground}}
			tc.slide.LayoutID = "slideLayout1"
			in := &PresentationInput{Slides: []SlideInput{tc.slide}}
			findings := contrastPredictions(collectContrastPreflightFindings(in, layouts, theme))
			if tc.wantBackground == "" {
				if len(findings) != 0 {
					t.Errorf("unknown photo background yielded predictions: %+v", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("got %d contrast predictions, want 1: %+v", len(findings), findings)
			}
			if got := findings[0].Fix.Params["background_color"]; got != tc.wantBackground {
				t.Errorf("background = %v, want %s", got, tc.wantBackground)
			}
			if got := findings[0].Fix.Params["predicted_replacement"]; got != tc.wantReplacement {
				t.Errorf("replacement = %v, want %s", got, tc.wantReplacement)
			}
		})
	}
}

func TestAlphaGridContrastPreflightUsesEffectiveBackground(t *testing.T) {
	for _, tc := range []struct {
		name, background, wantFill string
		wantPrediction             bool
	}{
		{"dark", "#000000", "#172848", true},
		{"light", "#FFFFFF", "#97A8C8", false},
		{"unknown photo", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			background := &BackgroundInput{Color: tc.background}
			if tc.name == "unknown photo" {
				background = &BackgroundInput{Image: "photo.png"}
			}
			slide := transparentGridSlide(background, `{"color":"accent1","alpha":50}`, "#000000")
			findings := contrastPredictions(collectContrastPreflightFindings(
				&PresentationInput{Slides: []SlideInput{slide}}, nil, tintTestTheme))
			if !tc.wantPrediction {
				if len(findings) != 0 {
					t.Errorf("unexpected predictions: %+v", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("predictions = %+v, want one", findings)
			}
			if got := findings[0].Fix.Params["background_color"]; got != tc.wantFill {
				t.Errorf("effective fill = %v, want %s", got, tc.wantFill)
			}
			if got := findings[0].Fix.Params["predicted_replacement"]; got != "#FFFFFF" {
				t.Errorf("replacement = %v, want white", got)
			}
		})
	}
}

func TestGridContrastPreflightRespectsSlideOptOut(t *testing.T) {
	off := false
	slide := transparentGridSlide(&BackgroundInput{Color: "#FFFFFF"}, `"#FFFFFF"`, "#FFFFFF")
	slide.ContrastCheck = &off
	in := &PresentationInput{Slides: []SlideInput{slide}}
	if findings := contrastPredictions(collectContrastPreflightFindings(in, nil, s7wmhCmdTheme)); len(findings) != 0 {
		t.Errorf("contrast_check:false predicted a shape-grid swap: %+v", findings)
	}
}

func TestTransparentShapeFillDoesNotTreatUnknownFillAsNoFill(t *testing.T) {
	for _, tc := range []struct {
		fill string
		want bool
	}{
		{`"none"`, true},
		{`{"color":"none"}`, true},
		{`{"gradient":["#FFFFFF","#000000"]}`, false},
		{`{"color":"unknown"}`, false},
	} {
		if got := transparentShapeFill(json.RawMessage(tc.fill)); got != tc.want {
			t.Errorf("fill %s transparent=%t, want %t", tc.fill, got, tc.want)
		}
	}
}

func TestTransparentGridPreflightReResolvesLayoutBackgroundAfterThemeOverride(t *testing.T) {
	slide := transparentGridSlide(nil, `"none"`, "#000000")
	slide.LayoutID = "slideLayout1"
	in := &PresentationInput{Slides: []SlideInput{slide}}
	layouts := []types.LayoutMetadata{{
		ID: "slideLayout1", BackgroundRef: "accent1", BackgroundHex: "#FFFFFF", // pre-override value
	}}
	modifiedTheme := []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
		{Name: "accent1", RGB: "#000000"}, // effective deck theme
	}
	findings := contrastPredictions(collectContrastPreflightFindings(in, layouts, modifiedTheme))
	if len(findings) != 1 {
		t.Fatalf("got %d predictions, want one against overridden accent1", len(findings))
	}
	if got := findings[0].Fix.Params["background_color"]; got != "#000000" {
		t.Errorf("predicted background = %v, want overridden #000000", got)
	}
}

func TestTransparentGridPreflightAppliesLayoutLumModAfterThemeOverride(t *testing.T) {
	slide := transparentGridSlide(nil, `"none"`, "#000000")
	slide.LayoutID = "slideLayout1"
	in := &PresentationInput{Slides: []SlideInput{slide}}
	layouts := []types.LayoutMetadata{{
		ID: "slideLayout1", BackgroundRef: "accent6", BackgroundHex: "#60A2F5",
		BackgroundMods: types.BackgroundColorModifiers{LumMod: 50000, HasLumMod: true},
	}}
	theme := []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"},
		{Name: "accent6", RGB: "#60A2F5"},
	}
	findings := contrastPredictions(collectContrastPreflightFindings(in, layouts, theme))
	if len(findings) != 1 {
		t.Fatalf("tinted-layout predictions = %+v, want one black-text correction", findings)
	}
	if got := findings[0].Fix.Params["background_color"]; got == "#60A2F5" || got == "" {
		t.Errorf("preflight used unmodified layout background: %v", got)
	}
	if got := findings[0].Fix.Params["predicted_replacement"]; got != "#FFFFFF" {
		t.Errorf("predicted replacement = %v, want white", got)
	}
}

func TestRunPresentationTransparentGridContrastMatchesPreflight(t *testing.T) {
	for _, tc := range []struct{ name, background, textColor string }{
		{"light", "#FFFFFF", "#FFFFFF"},
		{"dark", "#000000", "#000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slide := transparentGridSlide(&BackgroundInput{Color: tc.background}, `"none"`, tc.textColor)
			slide.SlideType = "blank"
			input := &PresentationInput{
				Template: "midnight-blue", OutputFilename: "transparent-grid.pptx",
				Slides: []SlideInput{slide},
			}
			applyDefaults(input)
			predictions := contrastPredictions(collectContrastPreflightFindings(input, nil, s7wmhCmdTheme))
			if len(predictions) != 1 {
				t.Fatalf("preflight predictions = %d, want 1", len(predictions))
			}
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			var matched bool
			for _, swap := range result.GenResult.ContrastSwaps {
				if swap.Source != "shape_grid" {
					continue
				}
				if swap.BackgroundColor == tc.background && swap.ReplacedColor == predictions[0].Fix.Params["predicted_replacement"] {
					matched = true
				}
			}
			if !matched {
				t.Errorf("render swaps %+v do not match preflight replacement %v on %s", result.GenResult.ContrastSwaps, predictions[0].Fix.Params["predicted_replacement"], tc.background)
			}
		})
	}
}

func TestRunPresentationAlphaGridContrastMatchesPreflight(t *testing.T) {
	slide := transparentGridSlide(&BackgroundInput{Color: "#000000"}, `{"color":"#2E5090","alpha":50}`, "#000000")
	slide.SlideType = "blank"
	input := &PresentationInput{
		Template: "midnight-blue", OutputFilename: "alpha-grid.pptx",
		Slides: []SlideInput{slide},
	}
	applyDefaults(input)
	predictions := contrastPredictions(collectContrastPreflightFindings(input, nil, tintTestTheme))
	if len(predictions) != 1 {
		t.Fatalf("preflight predictions = %+v, want one", predictions)
	}
	result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, swap := range result.GenResult.ContrastSwaps {
		if swap.Source == "shape_grid" && swap.BackgroundColor == predictions[0].Fix.Params["background_color"] &&
			swap.ReplacedColor == predictions[0].Fix.Params["predicted_replacement"] {
			return
		}
	}
	t.Errorf("render swaps %+v do not match preflight %+v", result.GenResult.ContrastSwaps, predictions[0])
}

func TestRunPresentationGridContrastOptOutHasNoSwap(t *testing.T) {
	off := false
	slide := transparentGridSlide(&BackgroundInput{Color: "#FFFFFF"}, `"none"`, "#FFFFFF")
	slide.SlideType = "blank"
	slide.ContrastCheck = &off
	input := &PresentationInput{
		Template: "midnight-blue", OutputFilename: "transparent-grid-optout.pptx",
		Slides: []SlideInput{slide},
	}
	applyDefaults(input)
	result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "strict",
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, swap := range result.GenResult.ContrastSwaps {
		if swap.Source == "shape_grid" {
			t.Errorf("contrast_check:false still generated grid swap: %+v", swap)
		}
	}
}
