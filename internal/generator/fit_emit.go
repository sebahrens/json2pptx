// Package generator provides PPTX file generation from slide specifications.
package generator

import "github.com/sebahrens/json2pptx/internal/patterns"

// emitFitFinding appends a render-time FitFinding to the context.
// These findings are surfaced in the generation result alongside pre-flight
// findings from the fit-report detectors.
func (ctx *singlePassContext) emitFitFinding(f patterns.FitFinding) {
	ctx.fitFindings = append(ctx.fitFindings, f)
}
