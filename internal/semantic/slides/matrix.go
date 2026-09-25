package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Matrix slides (go-slide-creator-ykjh).
//
// Two axes and four quadrants — impact against effort, reach against cost — is
// one of the handful of shapes a strategy deck always reaches for, and the
// DeckSpec had no kind for it. The payload here is what an author writes: the
// two axes, and the four quadrants clockwise from the top left.

const (
	// matrixQuadrantCount is the shape of the thing: four quadrants, no more
	// and no fewer.
	matrixQuadrantCount = 4
	// matrixAxisMax / matrixAxisEndMax / matrixHeaderMax / matrixBodyMax mirror
	// the pattern's own string budgets.
	matrixAxisMax    = 60
	matrixAxisEndMax = 11
	matrixHeaderMax  = 80
	matrixBodyMax    = 200
)

// matrixQuadrant is one resolved quadrant, in the pattern's own field names.
type matrixQuadrant struct {
	Header string `json:"header"`
	Body   string `json:"body,omitempty"`
}

// matrixValues is the matrix-2x2 pattern's values object.
type matrixValues struct {
	XAxisLabel  string         `json:"x_axis_label"`
	YAxisLabel  string         `json:"y_axis_label"`
	XLow        string         `json:"x_low,omitempty"`
	XHigh       string         `json:"x_high,omitempty"`
	YLow        string         `json:"y_low,omitempty"`
	YHigh       string         `json:"y_high,omitempty"`
	TopLeft     matrixQuadrant `json:"top_left"`
	TopRight    matrixQuadrant `json:"top_right"`
	BottomLeft  matrixQuadrant `json:"bottom_left"`
	BottomRight matrixQuadrant `json:"bottom_right"`
}

// matrixPositions names the quadrants in the order an author writes them:
// clockwise from the top left, which is how every 2x2 in every deck is read.
var matrixPositions = []string{"top_left", "top_right", "bottom_right", "bottom_left"}

// matrixPositionLabels are the same positions in prose, for the fallback.
var matrixPositionLabels = map[string]string{
	"top_left":     "Top left",
	"top_right":    "Top right",
	"bottom_right": "Bottom right",
	"bottom_left":  "Bottom left",
}

// CompileMatrix compiles a 2x2 payload onto the matrix-2x2 pattern, falling
// back to a quadrant bullet list when the payload does not fit it.
func CompileMatrix(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	values, ok := matrixPatternValues(in.Body)
	if !ok {
		return compileMatrixFallback(in)
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal matrix-2x2 values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "matrix-2x2", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values",
		SemanticPath: in.semSlide() + "." + matrixQuadrantsField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileMatrixFallback names each quadrant's position in prose, because a
// quadrant stripped of where it sits on the axes has lost the point of the
// slide.
func compileMatrixFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	quads := MatrixQuadrants(in.Body)
	x, y := matrixAxes(in.Body)

	// The axes lead, with their ends, because a quadrant list with no axes is
	// four headings and the reader cannot tell why any of them is where it is.
	var bullets []string
	if y != "" {
		bullets = append(bullets, "Vertical: "+matrixAxisLine(y, strField(in.Body, "y_low"), strField(in.Body, "y_high")))
	}
	if x != "" {
		bullets = append(bullets, "Horizontal: "+matrixAxisLine(x, strField(in.Body, "x_low"), strField(in.Body, "x_high")))
	}
	for _, pos := range matrixPositions {
		q, has := quads[pos]
		if !has || q.Header == "" {
			continue
		}
		line := matrixPositionLabels[pos] + " — " + q.Header
		if q.Body != "" {
			line += ": " + q.Body
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	return contentFallback(in, matrixQuadrantsField(in.Body), bullets)
}

// matrixAxisLine renders one axis for the fallback, naming its ends when the
// author gave them.
func matrixAxisLine(label, low, high string) string {
	switch {
	case low != "" && high != "":
		return fmt.Sprintf("%s (%s to %s)", label, low, high)
	case low != "":
		return fmt.Sprintf("%s (from %s)", label, low)
	case high != "":
		return fmt.Sprintf("%s (to %s)", label, high)
	default:
		return label
	}
}

// matrixPatternValues builds the pattern's values, reporting false when the
// payload cannot fill them.
func matrixPatternValues(body map[string]any) (matrixValues, bool) {
	quads := MatrixQuadrants(body)
	if len(quads) != matrixQuadrantCount || MatrixOverBudget(body) != "" {
		return matrixValues{}, false
	}
	x, y := matrixAxes(body)
	return matrixValues{
		XAxisLabel:  x,
		YAxisLabel:  y,
		XLow:        strField(body, "x_low"),
		XHigh:       strField(body, "x_high"),
		YLow:        strField(body, "y_low"),
		YHigh:       strField(body, "y_high"),
		TopLeft:     quads["top_left"],
		TopRight:    quads["top_right"],
		BottomLeft:  quads["bottom_left"],
		BottomRight: quads["bottom_right"],
	}, true
}

// MatrixQuadrants resolves the four quadrants, keyed by position. An author
// writes them either as a list (clockwise from the top left, the reading order
// of every 2x2) or as the named positions. Entries with no header are dropped,
// so a half-filled matrix is reported rather than rendered with blank boxes.
func MatrixQuadrants(body map[string]any) map[string]matrixQuadrant {
	out := map[string]matrixQuadrant{}
	if raw, ok := firstList(body, "quadrants", "cells", "boxes"); ok {
		for i, e := range raw {
			if i >= len(matrixPositions) {
				break
			}
			if q, valid := matrixQuadrantFrom(e); valid {
				out[matrixPositions[i]] = q
			}
		}
		return out
	}
	for _, pos := range matrixPositions {
		v, ok := body[pos]
		if !ok {
			continue
		}
		if q, valid := matrixQuadrantFrom(v); valid {
			out[pos] = q
		}
	}
	return out
}

// matrixQuadrantFrom reads one quadrant: a string (a bare header, or
// "Header: body") or an object.
func matrixQuadrantFrom(v any) (matrixQuadrant, bool) {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return matrixQuadrant{}, false
		}
		if header, body, found := strings.Cut(s, ": "); found {
			return matrixQuadrant{Header: strings.TrimSpace(header), Body: strings.TrimSpace(body)}, true
		}
		return matrixQuadrant{Header: s}, true
	case map[string]any:
		header := firstNonEmpty(strField(t, "header"), strField(t, "title"), strField(t, "label"), strField(t, "name"))
		if header == "" {
			return matrixQuadrant{}, false
		}
		return matrixQuadrant{
			Header: header,
			Body:   firstNonEmpty(strField(t, "body"), strField(t, "description"), strField(t, "detail"), strField(t, "summary")),
		}, true
	}
	return matrixQuadrant{}, false
}

// matrixAxes resolves the two axis labels.
func matrixAxes(body map[string]any) (x, y string) {
	x = firstNonEmpty(strField(body, "x_axis"), strField(body, "x_axis_label"))
	y = firstNonEmpty(strField(body, "y_axis"), strField(body, "y_axis_label"))
	return x, y
}

// matrixQuadrantsField names the payload field the quadrants came from.
func matrixQuadrantsField(body map[string]any) string {
	for _, key := range []string{"quadrants", "cells", "boxes"} {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "quadrants"
}

// MatrixOverBudget explains why a payload cannot take the 2x2, or "" when it
// fits. An empty payload is the required-field gate's business.
func MatrixOverBudget(body map[string]any) string {
	quads := MatrixQuadrants(body)
	if len(quads) == 0 {
		return ""
	}
	if len(quads) != matrixQuadrantCount {
		return fmt.Sprintf("has %d usable quadrants; a 2x2 needs %d, each with a header", len(quads), matrixQuadrantCount)
	}
	x, y := matrixAxes(body)
	switch {
	case x == "" || y == "":
		return "is missing an axis label; a 2x2 whose axes are unnamed says nothing about why a quadrant is where it is"
	case runeLen(x) > matrixAxisMax:
		return fmt.Sprintf("x axis label is %d characters; the axis holds %d", runeLen(x), matrixAxisMax)
	case runeLen(y) > matrixAxisMax:
		return fmt.Sprintf("y axis label is %d characters; the axis holds %d", runeLen(y), matrixAxisMax)
	}
	for _, end := range []struct{ name, value string }{
		{"x_low", strField(body, "x_low")}, {"x_high", strField(body, "x_high")},
		{"y_low", strField(body, "y_low")}, {"y_high", strField(body, "y_high")},
	} {
		if runeLen(end.value) > matrixAxisEndMax {
			return fmt.Sprintf("%s is %d characters; an axis end holds %d", end.name, runeLen(end.value), matrixAxisEndMax)
		}
	}
	for _, pos := range matrixPositions {
		q := quads[pos]
		switch {
		case runeLen(q.Header) > matrixHeaderMax:
			return fmt.Sprintf("the %s header is %d characters; a quadrant holds %d", strings.ToLower(matrixPositionLabels[pos]), runeLen(q.Header), matrixHeaderMax)
		case runeLen(q.Body) > matrixBodyMax:
			return fmt.Sprintf("the %s body is %d characters; a quadrant holds %d", strings.ToLower(matrixPositionLabels[pos]), runeLen(q.Body), matrixBodyMax)
		}
	}
	return ""
}

// MatrixPattern returns the pattern a 2x2 payload compiles to, or "" when it
// degrades to bullets.
func MatrixPattern(body map[string]any) string {
	if _, ok := matrixPatternValues(body); ok {
		return "matrix-2x2"
	}
	return ""
}

// UsableMatrixQuadrantCount returns how many quadrants survive extraction.
func UsableMatrixQuadrantCount(body map[string]any) int { return len(MatrixQuadrants(body)) }
