package generator

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

// Check only grid shapes carrying source roles, after contrast correction and
// immediately before their unchanged XML is inserted. This cannot mistake a
// native template footer for body copy or assign roles by font size after shrink.
//
// A finding is located at the written shape (/slides/N/rendered_shapes/ID/…),
// which is what the measurement is of; sources, index-aligned with shapes,
// names the authored element that produced each one, and travels as the
// finding's Source so a surface can report it where the author wrote it.
func (ctx *singlePassContext) reportGridReadability(shapes [][]byte, sources []RawShapeSource, roles map[uint32][]tokens.TextRole, slideIndex int) {
	if len(roles) == 0 || slideIndex < 0 {
		return
	}
	for fi, fragment := range shapes {
		source := rawShapeSourcePath(sources, fi)
		for _, raw := range splitShapeElements(string(fragment)) {
			var shape shapeXML
			if err := xml.Unmarshal([]byte(raw), &shape); err != nil || shape.TextBody == nil {
				continue
			}
			id := shape.NonVisualProperties.ConnectionNonVisual.ID
			paragraphRoles, ok := roles[id]
			if !ok || len(paragraphRoles) != len(shape.TextBody.Paragraphs) {
				continue // do not guess a role when the source/writer contract disagrees
			}
			scale, _ := storedShapeFontScale(raw)
			if scale <= 0 {
				continue
			}
			for pi, paragraph := range shape.TextBody.Paragraphs {
				role := paragraphRoles[pi]
				if role == "" {
					continue
				}
				size := smallestPopulatedParagraphRunSizeHPt(paragraph)
				if size <= 0 {
					continue
				}
				path := fmt.Sprintf("%s/rendered_shapes/%d/paragraphs/%d", slidepath.Slide(slideIndex), id, pi)
				if gridTextFrameHasNoArea(shape) {
					ctx.emitFitFinding(patterns.FitFinding{ValidationError: patterns.ValidationError{
						Path: path, Code: patterns.ErrCodeFitOverflow, Source: source,
						Message: "grid text frame has no usable area after written insets; preserve all source in a readable frame",
					}, Action: "refuse"})
					break
				}
				effective := int(float64(size) * float64(scale) / autofitScaleDenominator)
				if f := NewReadabilityFinding(ReadabilityFindingInput{
					Path: path, Mode: ctx.viewingMode, Role: role, EffectiveHPt: effective,
					Paragraphs: len(shape.TextBody.Paragraphs), MeasurementSource: "generated_grid_role",
					Context: fmt.Sprintf("written %.1fpt run with stored autofit %d%%", float64(size)/100, scale/1000),
				}); f != nil {
					ctx.recordReadabilityEvidence(path, ReadabilityEvidence{
						Role:              string(role),
						ActualPt:          float64(effective) / 100,
						MinPt:             float64(tokens.MinReadableHPt(ctx.viewingMode, role)) / 100,
						ViewingMode:       tokens.ViewingModeInputName(ctx.viewingMode),
						MeasurementSource: "generated_grid_role",
						Text:              paragraphText(paragraph),
					})
					f.Action = "refuse"
					f.Fix = nil
					f.Source = source
					f.Message = strings.TrimSuffix(strings.TrimSuffix(f.Message, "; shorten the text"), "; split the text") + "; preserve all source at a readable size"
					ctx.emitFitFinding(*f)
				}
			}
		}
	}
}

// rawShapeSourcePath is the authored element behind raw shape i, "" when the
// caller did not say.
func rawShapeSourcePath(sources []RawShapeSource, i int) string {
	if i < len(sources) {
		return sources[i].Path
	}
	return ""
}

func gridTextFrameHasNoArea(shape shapeXML) bool {
	transform := shape.ShapeProperties.Transform
	if transform == nil || shape.TextBody == nil {
		return false // no geometric measurement is available
	}
	insets := [4]int64{91440, 45720, 91440, 45720} // OOXML defaults, L/T/R/B
	if p := shape.TextBody.BodyProperties; p != nil {
		for i, value := range []*int64{p.LIns, p.TIns, p.RIns, p.BIns} {
			if value != nil {
				insets[i] = *value
			}
		}
	}
	return transform.Extent.CX <= insets[0]+insets[2] || transform.Extent.CY <= insets[1]+insets[3]
}

// paragraphText joins a paragraph's run text.
func paragraphText(paragraph paragraphXML) string {
	var b strings.Builder
	for _, run := range paragraph.Runs {
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String())
}

func smallestPopulatedParagraphRunSizeHPt(paragraph paragraphXML) int {
	smallest := 0
	for _, run := range paragraph.Runs {
		if strings.TrimSpace(run.Text) == "" || run.RunProperties == nil {
			continue
		}
		size, err := strconv.Atoi(run.RunProperties.FontSize)
		if err == nil && size > 0 && (smallest == 0 || size < smallest) {
			smallest = size
		}
	}
	return smallest
}
