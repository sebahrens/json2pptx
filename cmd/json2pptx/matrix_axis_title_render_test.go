package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

var (
	matrixRenderedShapeRE = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
	matrixShapeOffsetRE   = regexp.MustCompile(`<a:off x="(\d+)" y="(\d+)"/>`)
	matrixShapeExtentRE   = regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"/>`)
	matrixFontScaleRE     = regexp.MustCompile(`fontScale="(\d+)"`)
	matrixRunSizeRE       = regexp.MustCompile(`<a:rPr\b[^>]*\bsz="(\d+)"`)
)

// TestMatrixAxisTitleClearsArrow checks final PPTX geometry, not just the
// expanded grid: a shape-grid change once placed the title directly over the
// arrow shaft, and a first attempted split made PowerPoint shrink it to 3pt.
func TestMatrixAxisTitleClearsArrow(t *testing.T) {
	for _, templateName := range testutil.AllTestTemplateNames() {
		t.Run(templateName, func(t *testing.T) {
			input := &PresentationInput{
				Template: templateName,
				Slides: []SlideInput{{SlideType: "content", Pattern: &PatternInput{
					Name: "matrix-2x2", Values: json.RawMessage(`{
						"x_axis_label":"Effort","y_axis_label":"Impact",
						"x_low":"Low effort","x_high":"High effort",
						"y_low":"Low impact","y_high":"Underserved",
						"top_left":{"header":"Quick wins"},"top_right":{"header":"Strategic bets"},
						"bottom_left":{"header":"Maintenance"},"bottom_right":{"header":"Defer"}
					}`),
				}}},
			}
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			var titleShape, arrowShape string
			for _, shape := range matrixRenderedShapeRE.FindAllString(readSlideXML(t, result.OutputPath, "ppt/slides/slide1.xml"), -1) {
				if strings.Contains(shape, "<a:t>Effort</a:t>") {
					titleShape = shape
				}
				if strings.Contains(shape, `<a:prstGeom prst="rightArrow">`) {
					arrowShape = shape
				}
			}
			if titleShape == "" || arrowShape == "" {
				t.Fatalf("matrix title/arrow missing: title=%t arrow=%t", titleShape != "", arrowShape != "")
			}
			if effective := matrixTitleEffectiveHPt(t, titleShape); effective < 1200 {
				t.Errorf("horizontal axis title autofits below 12pt: %.2fpt", float64(effective)/100)
			}
			if strings.Contains(arrowShape, "<a:t>Effort</a:t>") {
				t.Error("horizontal axis title is still inside the arrow shape")
			}
			if edge := matrixShapeRight(t, titleShape); edge > matrixShapeLeft(t, arrowShape) {
				t.Errorf("axis title overlaps arrow: title right=%d, arrow left=%d", edge, matrixShapeLeft(t, arrowShape))
			}
		})
	}
}

func matrixTitleEffectiveHPt(t *testing.T, shape string) int {
	t.Helper()
	run := matrixRunSizeRE.FindStringSubmatch(shape)
	if len(run) != 2 {
		t.Fatalf("axis title has no declared run size: %s", shape)
	}
	size, err := strconv.Atoi(run[1])
	if err != nil {
		t.Fatal(err)
	}
	scale := 100000
	if match := matrixFontScaleRE.FindStringSubmatch(shape); len(match) == 2 {
		scale, err = strconv.Atoi(match[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	return size * scale / 100000
}

func matrixShapeLeft(t *testing.T, shape string) int64 {
	t.Helper()
	m := matrixShapeOffsetRE.FindStringSubmatch(shape)
	if len(m) != 3 {
		t.Fatalf("shape has no x/y offset: %s", shape)
	}
	x, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func matrixShapeRight(t *testing.T, shape string) int64 {
	t.Helper()
	m := matrixShapeExtentRE.FindStringSubmatch(shape)
	if len(m) != 3 {
		t.Fatalf("shape has no width/height: %s", shape)
	}
	w, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return matrixShapeLeft(t, shape) + w
}
