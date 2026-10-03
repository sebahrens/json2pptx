package main

import "github.com/sebahrens/json2pptx/internal/patterns"

// Pattern area refusals (go-slide-creator-uj9zq).
//
// A pattern that measures its content against the area it is given can refuse
// it at expansion: a kpi-Nup row in a region too short for a value over its
// caption would be written with the lines shrunk into each other. Generation
// stops on that error; these helpers carry the same refusal to the fit report
// and to render_deck_spec's diagnostics, so validate says what render says and
// both name the cell that holds the pattern.

// patternAreaRefusals returns the fit_overflow refusals err carries as
// blocking fit findings rooted at basePointer (the JSON Pointer of the grid or
// pattern the finding paths are relative to). It returns nil when err is not
// such a refusal.
func patternAreaRefusals(err error, basePointer string) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, ve := range patternValidationFindings(err) {
		if ve.Code != patterns.ErrCodeFitOverflow {
			continue
		}
		f := patterns.FitFinding{ValidationError: *ve, Action: "refuse"}
		f.Path = patternFieldPointer(basePointer, ve.Path)
		out = append(out, f)
	}
	return out
}
