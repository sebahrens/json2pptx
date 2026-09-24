package main

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// chartFindingsAtAuthoredPaths trims each chart/diagram finding to the deepest
// JSON Pointer that exists in the authored deck. svggen may report a derived
// field (for example, an axis added during rendering), while pattern expansion
// creates rows/cells that do not exist in the input at all. Returning the
// nearest authored ancestor gives repair tools a real, replaceable target.
func chartFindingsAtAuthoredPaths(input *PresentationInput, findings []patterns.FitFinding) []patterns.FitFinding {
	if len(findings) == 0 {
		return findings
	}
	data, err := json.Marshal(input)
	if err != nil {
		return chartFindingsAtSlideRoots(findings)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return chartFindingsAtSlideRoots(findings)
	}
	for i := range findings {
		findings[i].Path = deepestAuthoredPointer(document, findings[i].Path)
	}
	return findings
}

func chartFindingsAtSlideRoots(findings []patterns.FitFinding) []patterns.FitFinding {
	for i := range findings {
		if index := slidepath.SlideIndex(findings[i].Path); index >= 0 {
			findings[i].Path = slidepath.Slide(index)
		}
	}
	return findings
}

func deepestAuthoredPointer(document any, pointer string) string {
	if !strings.HasPrefix(pointer, "/slides/") {
		return pointer
	}
	current := document
	path := ""
	for _, token := range strings.Split(pointer[1:], "/") {
		if token == "" {
			break
		}
		next, err := pointerDescend(current, unescapeJSONPointerToken(token))
		if err != nil {
			break
		}
		path += "/" + token
		current = next
	}
	if path == "" {
		return pointer
	}
	return path
}
