package generator

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/svggen"
)

// SvggenFindingsToFit converts svggen's render findings into FitFindings rooted
// at path.
//
// The render path collected these for tables and dropped them for diagrams, so
// a chart that told the renderer something at draw time — a thinned axis, a
// clipped label, a legend that could not show every entry — said it into a
// channel nobody read. The dry-render preflight in cmd/json2pptx now shares
// this conversion, so a finding means the same thing whichever side produced it
// (go-slide-creator-p142).
func SvggenFindingsToFit(findings []svggen.Finding, diagramType, path string) []patterns.FitFinding {
	if len(findings) == 0 {
		return nil
	}
	out := make([]patterns.FitFinding, 0, len(findings))
	for _, f := range findings {
		fieldPath := path
		if f.Field != "" {
			fieldPath = path + "." + f.Field
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: diagramType,
				Path:    fieldPath,
				Code:    f.Code,
				Message: f.Message,
				Fix:     svggenFixToFit(f.Fix),
			},
			Action: SvggenSeverityAction(f.Severity),
		})
	}
	return out
}

// svggenFixToFit mirrors an svggen fix suggestion into the patterns shape.
func svggenFixToFit(fs *svggen.FixSuggestion) *patterns.FixSuggestion {
	if fs == nil {
		return nil
	}
	return &patterns.FixSuggestion{Kind: fs.Kind, Params: fs.Params}
}

// SvggenSeverityAction maps svggen's severity ladder onto the FitFinding action
// axis:
//
//	info / warning  → "review"
//	shrink_or_split → "shrink_or_split"
//	refuse          → "refuse"
//
// An unrecognised severity falls back to "info" so an unknown code still
// surfaces without claiming a severity it does not have.
func SvggenSeverityAction(severity string) string {
	switch severity {
	case "refuse":
		return "refuse"
	case "shrink_or_split":
		return "shrink_or_split"
	case "warning":
		return "review"
	case "info":
		return "info"
	}
	return "info"
}
